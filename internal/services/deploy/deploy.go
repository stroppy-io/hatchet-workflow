// Package deploy resolves a tenant's full deployment params for the dag compiler:
// provider settings + per-machine network allocation (D20) + per-machine
// cloud-init with an agent JWT (D18). It composes provider.Resolver (settings),
// netalloc (IPs), and cloudinit (user-data).
package deploy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gopherex/xlog"
	"github.com/yaroher/ratel/pkg/exec"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	domainauth "github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/cloudinit"
	dagdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/netalloc"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/system"
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

// Resolve produces the run-scoped deployment.Deployment for a TestPreset in a tenant:
// the provider-shaped infra (dag.BuildDeployment) enriched with per-machine network
// allocation (D20) and the agent bootstrap (D18). For Docker the container Env carries
// STROPPY_SERVER_ADDR / STROPPY_MACHINE_ID / STROPPY_AGENT_TOKEN (+ systemd mounts);
// for Yandex the per-instance cloud-init user-data. It is baked into the dag at submit
// time — the prepareDeployment node passes it through to the deploy/teardown handlers.
func (r *Resolver) Resolve(ctx context.Context, tenantID string, preset *domain.TestPreset) (*deployment.Deployment, error) {
	topo := preset.GetTopology()
	prov := preset.GetDeployment().GetProvider()
	serverAddr := r.serverAddr(ctx)
	binaryURL := serverAddr + agentBinaryPath

	// signToken mints a per-machine agent JWT (the agent's machine principal, D18).
	signToken := func(mid string) (string, error) {
		token, err := r.signer.SignAgent(ids.New(), tenantID, mid, r.cfg.AgentJWTTTL())
		if err != nil {
			return "", status.Errorf(codes.Internal, "sign agent token: %v", err)
		}
		return token, nil
	}

	switch prov {
	case deployment.Provider_PROVIDER_DOCKER:
		dep := dagdomain.BuildDeployment(preset, &system.Network{}, dagdomain.QuotaForTopology(topo, prov))
		containers := dep.GetDocker().GetInput().GetContainers()
		for _, m := range topo.GetMachines() {
			token, err := signToken(m.GetId())
			if err != nil {
				return nil, err
			}
			c := containers[m.GetId()]
			if c == nil {
				continue
			}
			// STROPPY_SERVER_ADDR (http URL) is used by the agent unit's ExecStartPre to
			// fetch the binary; STROPPY_AGENT_SERVER is the grpc dial target (host:port,
			// no scheme); STROPPY_AGENT_TENANT scopes the agent's lease/report.
			agentServer := strings.TrimPrefix(strings.TrimPrefix(serverAddr, "https://"), "http://")
			c.Env = map[string]string{
				"STROPPY_SERVER_ADDR":  serverAddr,
				"STROPPY_AGENT_SERVER": agentServer,
				"STROPPY_AGENT_TENANT": tenantID,
				"STROPPY_MACHINE_ID":   m.GetId(),
				"STROPPY_AGENT_TOKEN":  token,
			}
			// systemd services do NOT inherit PID-1 (container) env, so the agent unit
			// reads /etc/stroppy-agent.env (its EnvironmentFile). Write it here.
			c.Files = append(c.GetFiles(), &deployment.Docker_File{
				Path: "/etc/stroppy-agent.env",
				Content: []byte(fmt.Sprintf(
					"STROPPY_SERVER_ADDR=%s\nSTROPPY_AGENT_SERVER=%s\nSTROPPY_AGENT_TENANT=%s\nSTROPPY_MACHINE_ID=%s\nSTROPPY_AGENT_TOKEN=%s\n",
					serverAddr, agentServer, tenantID, m.GetId(), token)),
				Mode: 0o644,
			})
			// systemd-in-docker needs a writable /run + the host cgroup mount.
			c.Tmpfs = map[string]string{"/run": "exec,mode=755", "/run/lock": ""}
			c.Binds = append(c.GetBinds(), "/sys/fs/cgroup:/sys/fs/cgroup:rw")
		}
		return dep, nil

	case deployment.Provider_PROVIDER_YANDEX:
		network, compute, vmDefaults, err := r.provider.Resolve(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		ips, err := r.allocIPs(ctx, tenantID, network.GetCidr(), topo)
		if err != nil {
			return nil, err
		}
		vms := make(map[string]*deployment.Yandex_Vm, len(topo.GetMachines()))
		for _, m := range topo.GetMachines() {
			token, err := signToken(m.GetId())
			if err != nil {
				return nil, err
			}
			ud, err := cloudinit.Render(cloudinit.Params{
				BinaryURL: binaryURL, ServerAddr: serverAddr, MachineID: m.GetId(), AgentToken: token,
			})
			if err != nil {
				return nil, status.Errorf(codes.Internal, "render cloud-init: %v", err)
			}
			vm, _ := proto.Clone(vmDefaults).(*deployment.Yandex_Vm)
			vm.Cores = m.GetCores()
			vm.MemoryGb = m.GetMemoryGb()
			vm.BootDiskGb = m.GetDiskGb()
			vm.InternalIp = ips[m.GetId()]
			vm.UserData = ud
			for i, gb := range m.GetDataDisksGb() {
				vm.SecondaryDisks = append(vm.SecondaryDisks, &deployment.Yandex_Disk{
					DeviceName: fmt.Sprintf("data-%d", i), SizeGb: uint32(gb), Type: vmDefaults.GetBootDiskType(),
				})
			}
			vms[m.GetId()] = vm
		}
		compute.Vms = vms
		return &deployment.Deployment{
			Provider: prov,
			Deployment: &deployment.Deployment_Yandex{Yandex: &deployment.Yandex{
				Input: &deployment.Yandex_Input{Network: network, Compute: compute},
			}},
		}, nil

	default:
		return nil, status.Errorf(codes.InvalidArgument, "unsupported provider %s", prov)
	}
}

// allocIPs assigns per-machine private IPs from the subnet, avoiding IPs the provider
// reports as used (never assume the subnet is empty, D20), and reserves them.
func (r *Resolver) allocIPs(ctx context.Context, tenantID, cidr string, topo *domain.Topology) (map[string]string, error) {
	if cidr == "" || len(topo.GetMachines()) == 0 {
		return map[string]string{}, nil
	}
	machineIDs := make([]string, 0, len(topo.GetMachines()))
	for _, m := range topo.GetMachines() {
		machineIDs = append(machineIDs, m.GetId())
	}
	used, err := r.inventory.UsedIPs(ctx, tenantID, cidr)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "network inventory: %v", err)
	}
	ips, err := netalloc.AssignIPs(cidr, machineIDs, used)
	if err != nil {
		return nil, status.Errorf(codes.ResourceExhausted, "allocate ips: %v", err)
	}
	assigned := make([]string, 0, len(ips))
	for _, ip := range ips {
		assigned = append(assigned, ip)
	}
	if err := r.inventory.Reserve(ctx, tenantID, assigned); err != nil {
		return nil, status.Errorf(codes.Internal, "reserve ips: %v", err)
	}
	return ips, nil
}
