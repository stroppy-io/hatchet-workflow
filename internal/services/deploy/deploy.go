// Package deploy resolves a tenant's full deployment params for the planner:
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

	domainauth "github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/cloudinit"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/netalloc"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/planner"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
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

// Config supplies cloud-init + agent-token settings.
type Config interface {
	ServerAddr() string
	AgentBinaryURL() string
	JWTSecret() []byte
	AgentJWTTTL() time.Duration
}

// Resolver builds DeploymentParams from settings + network allocation + cloud-init.
type Resolver struct {
	*tracing.Entity
	provider  *provider.Resolver
	inventory NetworkInventory
	signer    *domainauth.Signer
	cfg       Config
}

// New builds a Resolver.
func New(logger *xlog.Logger, executor exec.DB, inventory NetworkInventory, cfg Config) *Resolver {
	return &Resolver{
		Entity:    tracing.NewEntity(logger.AppendName("DeployResolver")),
		provider:  provider.New(logger, executor),
		inventory: inventory,
		signer:    domainauth.NewSigner(cfg.JWTSecret()),
		cfg:       cfg,
	}
}

// Resolve produces full deployment params for the topology in the given tenant.
func (r *Resolver) Resolve(ctx context.Context, tenantID string, topo *domain.Topology) (*planner.DeploymentParams, error) {
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

	// Cloud-init: per-machine agent JWT + bootstrap user-data (D18).
	for _, mid := range machineIDs {
		token, err := r.signer.SignAgent(ids.New(), tenantID, mid, r.cfg.AgentJWTTTL())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "sign agent token: %v", err)
		}
		ud, err := cloudinit.Render(cloudinit.Params{
			BinaryURL:  r.cfg.AgentBinaryURL(),
			ServerAddr: r.cfg.ServerAddr(),
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
