// Package deploy resolves a tenant's full deployment params for the dag compiler:
// provider settings + per-machine network allocation (D20) + per-machine
// cloud-init with an agent JWT (D18). It composes provider.Resolver (settings),
// netalloc (IPs), and cloudinit (user-data).
package deploy

import (
	"context"
	"time"

	"github.com/gopherex/xlog"
	"github.com/yaroher/ratel/pkg/exec"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	domainauth "github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/cloudinit"
	dagdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/netalloc"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/provider"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// NetworkInventory reports the IPs already in use in a tenant's subnet so the
// allocator does not collide with existing VMs (the subnet is never assumed
// empty, H33), and reserves the IPs it hands out. Implemented by
// internal/services/netinventory (persisted mirror); a live-VPC reconcile layer
// (cloud as source of truth) can wrap it later.
type NetworkInventory interface {
	UsedIPs(ctx context.Context, tenantID, subnetCIDR string) ([]string, error)
	Reserve(ctx context.Context, tenantID string, ips []string) error
}

// Config supplies cloud-init + agent-token settings. ServerAddr is only the
// FALLBACK (e.g. the docker-host address for local runs); the authoritative public
// address is the global PlatformSettings.server_addr (root-admin set).
type Config interface {
	ServerAddr() string
	JWTSecret() []byte
	AgentJWTTTL() time.Duration
}

// PlatformReader reads the global (singleton) control-plane settings — the single
// public server_addr handed to every agent. Implemented by services/platform.
type PlatformReader interface {
	GetPlatformSettings(ctx context.Context, _ *emptypb.Empty) (*models.PlatformSettings, error)
}

// agentBinaryPath is the server endpoint that serves the agent binary; the cloud-init
// download URL is derived as <server_addr><agentBinaryPath>.
const agentBinaryPath = "/agent/binary"

// Resolver builds DeploymentParams from settings + network allocation + cloud-init.
type Resolver struct {
	*tracing.Entity
	provider  *provider.Resolver
	inventory NetworkInventory
	platform  PlatformReader
	signer    *domainauth.Signer
	cfg       Config
}

// New builds a Resolver. platform may be nil (then only cfg.ServerAddr() is used).
func New(logger *xlog.Logger, executor exec.DB, inventory NetworkInventory, platform PlatformReader, cfg Config) *Resolver {
	return &Resolver{
		Entity:    tracing.NewEntity(logger.AppendName("DeployResolver")),
		provider:  provider.New(logger, executor),
		inventory: inventory,
		platform:  platform,
		signer:    domainauth.NewSigner(cfg.JWTSecret()),
		cfg:       cfg,
	}
}

// serverAddr resolves the public control-plane address handed to agents: the global
// PlatformSettings.server_addr if set, else the configured fallback (docker-host
// address for local runs).
func (r *Resolver) serverAddr(ctx context.Context) string {
	if r.platform != nil {
		if ps, err := r.platform.GetPlatformSettings(ctx, &emptypb.Empty{}); err == nil && ps.GetServerAddr() != "" {
			return ps.GetServerAddr()
		}
	}
	return r.cfg.ServerAddr()
}

// Resolve produces full deployment params for the topology in the given tenant.
func (r *Resolver) Resolve(ctx context.Context, tenantID string, topo *domain.Topology) (*dagdomain.DeploymentParams, error) {
	params, err := r.provider.Resolve(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	machineIDs := make([]string, 0, len(topo.GetMachines()))
	for _, m := range topo.GetMachines() {
		machineIDs = append(machineIDs, m.GetId())
	}

	// Network allocation: assign per-machine private IPs from the subnet, avoiding
	// IPs the provider reports as used (never assume the subnet is empty).
	if params.NetworkCIDR != "" && len(machineIDs) > 0 {
		used, err := r.inventory.UsedIPs(ctx, tenantID, params.NetworkCIDR)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "network inventory: %v", err)
		}
		ips, err := netalloc.AssignIPs(params.NetworkCIDR, machineIDs, used)
		if err != nil {
			return nil, status.Errorf(codes.ResourceExhausted, "allocate ips: %v", err)
		}
		params.MachineInternalIP = ips
		// Reserve the assigned IPs so concurrent/later runs observe them as used.
		assigned := make([]string, 0, len(ips))
		for _, ip := range ips {
			assigned = append(assigned, ip)
		}
		if err := r.inventory.Reserve(ctx, tenantID, assigned); err != nil {
			return nil, status.Errorf(codes.Internal, "reserve ips: %v", err)
		}
	}

	// Cloud-init: per-machine agent JWT + bootstrap user-data (D18). The single
	// public server_addr (PlatformSettings, root-admin) is handed to every agent;
	// the binary download URL is DERIVED from it (server serves the agent binary).
	serverAddr := r.serverAddr(ctx)
	binaryURL := serverAddr + agentBinaryPath
	for _, mid := range machineIDs {
		token, err := r.signer.SignAgent(ids.New(), tenantID, mid, r.cfg.AgentJWTTTL())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "sign agent token: %v", err)
		}
		ud, err := cloudinit.Render(cloudinit.Params{
			BinaryURL:  binaryURL,
			ServerAddr: serverAddr,
			MachineID:  mid,
			AgentToken: token,
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "render cloud-init: %v", err)
		}
		params.MachineUserData[mid] = ud
	}
	return params, nil
}
