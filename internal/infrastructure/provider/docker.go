package provider

import (
	"context"
	"encoding/json"
	"fmt"

	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	common "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

// defaultAgentImage is the stroppy-agent container tag the docker builtin
// provider uses when a recipe's provider.params sets no explicit image. It
// matches cmd/cli/serve_cmd.go's AGENT_IMAGE default so the DSL flow and the
// pre-DSL config path agree on the same stock image.
const defaultAgentImage = "stroppy-agent:latest"

// dockerProvider is the builtin Provider implementation: one container per
// requested machine, running the stroppy-agent bootstrap (same env
// convention as the pre-DSL renderDockerInput/agentdomain path), with no
// terraform module involved.
type dockerProvider struct {
	exec dockerExec
}

// NewDocker builds the docker builtin Provider. exec performs the actual
// container lifecycle (see dockerExec for the adaptation gap against the
// real docker.Executor runner).
func NewDocker(exec dockerExec) Provider {
	return &dockerProvider{exec: exec}
}

// dockerParams is the dynamic shape of a docker ProviderRef.ParamsJson: it
// plays the role that terraform's variables.tf/tfvars play for the
// terraform-module provider, since the docker builtin has no module to
// derive a schema from.
type dockerParams struct {
	Image      string            `json:"image"`
	Network    string            `json:"network"`
	ServerAddr string            `json:"server_addr"`
	BinaryURL  string            `json:"binary_url"`
	RunID      string            `json:"run_id"`
	Env        map[string]string `json:"env"`

	// GatewayGroup, when set, names the MachineGroup that is this run's
	// control node: Provision additionally renders a Nomad server+client
	// sidecar container (see nomadGatewayContainer) once for that group, so
	// service-jobs (internal/dsl/nomad.BuildJob) have somewhere to land as
	// sibling docker containers instead of requiring a real Nomad cluster.
	// Empty (the default) renders no Nomad sidecar at all — every existing
	// docker recipe that doesn't set this keeps today's behavior unchanged.
	// A GatewayGroup with Count > 1 still gets exactly one sidecar (rendered
	// once, not per node) — docker recipes are expected to size the gateway
	// group at 1.
	GatewayGroup string `json:"gateway_group"`
	// NomadVersion pins the hashicorp/nomad image tag the gateway sidecar
	// runs. Empty selects agentdomain.DefaultNomadVersion, same as cloud-init.
	NomadVersion string `json:"nomad_version"`
}

func decodeDockerParams(ref *dslpb.ProviderRef) (dockerParams, error) {
	var params dockerParams
	raw := ref.GetParamsJson()
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &params); err != nil {
			return params, fmt.Errorf("decode docker provider params_json: %w", err)
		}
	}
	if params.Image == "" {
		// The docker builtin has no variables.tf to carry a default, so the
		// stock stroppy-agent image is the implicit default when a recipe's
		// provider.params does not pin one — mirrors the pre-DSL
		// renderDockerInput path that also hardcoded this tag.
		params.Image = defaultAgentImage
	}
	if params.ServerAddr == "" {
		return params, fmt.Errorf("docker provider params: server_addr is required")
	}
	if params.RunID == "" {
		return params, fmt.Errorf("docker provider params: run_id is required")
	}
	if params.Network == "" {
		params.Network = dockerNetworkName(params.RunID)
	}
	return params, nil
}

func (p *dockerProvider) Provision(ctx context.Context, ref *dslpb.ProviderRef, groups []*dslpb.MachineGroup) (map[string][]*deploymentpb.MachineState, error) {
	params, err := decodeDockerParams(ref)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]*deploymentpb.MachineState, len(groups))
	for _, group := range groups {
		for idx := uint32(0); idx < group.GetCount(); idx++ {
			nodeID := fmt.Sprintf("%s-%d", group.GetName(), idx)
			containerName := dockerContainerName(params.RunID, nodeID)

			env, err := agentdomain.Env(nodeID, agentdomain.Bootstrap{
				ServerAddr: params.ServerAddr,
				BinaryURL:  params.BinaryURL,
				RunID:      params.RunID,
				ExtraEnv:   params.Env,
			})
			if err != nil {
				return nil, fmt.Errorf("render agent bootstrap env for %q: %w", nodeID, err)
			}

			spec := ContainerSpec{
				Name:    containerName,
				Image:   params.Image,
				Network: params.Network,
				Env:     env,
				Labels: map[string]string{
					"stroppy.cloud/run_id":  params.RunID,
					"stroppy.cloud/node_id": nodeID,
					"group":                 group.GetName(),
				},
			}

			state, err := p.exec.EnsureContainer(ctx, spec)
			if err != nil {
				return nil, fmt.Errorf("ensure container %q: %w", containerName, err)
			}

			result[group.GetName()] = append(result[group.GetName()], &deploymentpb.MachineState{
				NodeId:             nodeID,
				ProviderResourceId: state.ID,
				Status:             common.Status_STATUS_DEPLOYED,
				Endpoints: []*deploymentpb.Endpoint{
					{
						Name:    "private",
						Address: state.InternalIP,
						// disk_device (I2): docker containers have no block
						// device of their own — this per-node agent container
						// sets no Binds/Files (ContainerSpec's Binds/Files
						// exist since the Nomad gateway sidecar below, but
						// Provision never wires arbitrary host disks through
						// them for ordinary machines), so there is no host
						// path to report. The label is still set, explicitly
						// empty, so recipe authors see an intentional "not
						// applicable" rather than a silently missing key; a
						// step that does
						// `mkfs ${{ machine.disks[0].path }}` against a
						// docker topology is a recipe bug, not a provider
						// one, and should target a terraform-backed
						// topology instead.
						Labels: map[string]string{"scope": "private", "disk_device": ""},
					},
				},
				Labels: map[string]string{
					"node_id": nodeID,
					"group":   group.GetName(),
				},
			})
		}

		if params.GatewayGroup != "" && group.GetName() == params.GatewayGroup {
			nomadSpec := nomadGatewayContainer(params.RunID, params.Network, params.NomadVersion)
			if _, err := p.exec.EnsureContainer(ctx, nomadSpec); err != nil {
				return nil, fmt.Errorf("ensure nomad gateway container %q: %w", nomadSpec.Name, err)
			}
		}
	}
	return result, nil
}

func (p *dockerProvider) Destroy(ctx context.Context, ref *dslpb.ProviderRef) error {
	params, err := decodeDockerParams(ref)
	if err != nil {
		return err
	}
	if err := p.exec.RemoveContainers(ctx, params.Network); err != nil {
		return fmt.Errorf("remove docker containers for network %q: %w", params.Network, err)
	}
	return nil
}

func dockerNetworkName(runID string) string {
	return "stroppy-" + runID
}

func dockerContainerName(runID, nodeID string) string {
	return "stroppy-" + runID + "-" + nodeID
}

// nomadContainerName names the run's Nomad gateway sidecar container,
// distinct from dockerContainerName's per-node agent containers so the two
// never collide.
func nomadContainerName(runID string) string {
	return "stroppy-" + runID + "-nomad"
}

// nomadGatewayContainer builds the ContainerSpec for the docker provider's
// dev-only Nomad sidecar: ONE single-node Nomad server+client (docker driver,
// docker.sock mounted in) so nomad.BuildJob-produced service jobs
// (internal/dsl/nomad/jobspec.go) can schedule as sibling docker containers
// on the gateway node, without standing up a real multi-node Nomad cluster.
//
// Shape decisions (see task-2-report.md for the full writeup):
//   - Image is "hashicorp/nomad:<version>" (official image); its default CMD
//     is ["help"] (see hashicorp/nomad's Dockerfile), so Cmd overrides it to
//     "agent -config=<NomadConfigDir>" — the entrypoint just execs
//     "nomad $@", it does not read any NOMAD_LOCAL_CONFIG-style env var, so
//     the config MUST land on disk before the container starts.
//   - Config is agentdomain.NomadServerHCL() (the same single-node
//     server+client HCL cloud-init renders for NomadRoleServer), delivered
//     via ContainerSpec.Files at agentdomain.NomadServerHCLPath — Executor
//     copies files into the container before ContainerStart (see
//     docker_adapter.go/executor.go), so the file is present when Nomad's
//     entrypoint runs.
//   - Privileged + Binds mount /var/run/docker.sock so Nomad's docker task
//     driver can talk to the same docker daemon this provider itself uses.
//   - Network is the run's shared bridge network (params.Network), the same
//     one every agent container joins — NOT host networking. The brief's
//     "network host (or the run network)" alternative is resolved to "the
//     run network" because internal/infrastructure/docker.Executor's
//     hostConfig has no per-container NetworkMode override today (network
//     mode is derived once, for the whole Docker_Input, from
//     Docker_Network.Name — see executor.go); adding a per-container host-
//     network override would be a docker.Executor/proto change, out of
//     scope for this task (flagged in task-2-report.md).
//
// KNOWN GAP (documented, not fixed here — a 1D concern): NomadServerHCL
// renders NomadRoleServer, which stamps NO meta.stroppy_node_id (only
// NomadRoleClient does, see bootstrap.go's renderNomadHCL). nomad.BuildJob
// pins every task group with a `${meta.stroppy_node_id} == NodeID`
// constraint (jobspec.go's nodeIDMetaAttr), so on this single-node docker
// Nomad, no task group's constraint will ever match — placement fails for
// any node id BuildJob is given. Resolving this needs either (a) the docker
// service-job path dropping/relaxing that per-node constraint, or (b)
// teaching this single node to stamp every requested NodeID's meta — both
// are internal/dsl/nomad (jobspec.go) or run/recipe-orchestration changes,
// out of scope here per the task's scope guard. This function only renders
// the sidecar container; the constraint reconciliation is left to the 1D
// docker-scheduling integration task.
func nomadGatewayContainer(runID, network, version string) ContainerSpec {
	if version == "" {
		version = agentdomain.DefaultNomadVersion
	}
	return ContainerSpec{
		Name:       nomadContainerName(runID),
		Image:      fmt.Sprintf("hashicorp/nomad:%s", version),
		Network:    network,
		Privileged: true,
		Binds:      []string{"/var/run/docker.sock:/var/run/docker.sock"},
		Cmd:        []string{"agent", "-config=" + agentdomain.NomadConfigDir},
		Files: []ContainerFile{
			{
				Path:    agentdomain.NomadServerHCLPath,
				Content: []byte(agentdomain.NomadServerHCL()),
				Mode:    0o644,
			},
		},
		Labels: map[string]string{
			"stroppy.cloud/run_id": runID,
			"stroppy.cloud/role":   "nomad-gateway",
		},
	}
}
