package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

// fakeDockerExec is a test double for dockerExec: it records every
// EnsureContainer call and returns a deterministic container id/ip per spec
// name, and records RemoveContainers calls by network name.
type fakeDockerExec struct {
	ensured []ContainerSpec

	removedNetworks []string
	removeErr       error
}

func (f *fakeDockerExec) EnsureContainer(_ context.Context, spec ContainerSpec) (ContainerState, error) {
	f.ensured = append(f.ensured, spec)
	return ContainerState{
		ID:         "container-" + spec.Name,
		InternalIP: "172.20.0." + itoa(len(f.ensured)),
	}, nil
}

func (f *fakeDockerExec) RemoveContainers(_ context.Context, networkName string) error {
	f.removedNetworks = append(f.removedNetworks, networkName)
	return f.removeErr
}

func runnerGroup() *dslpb.MachineGroup {
	return &dslpb.MachineGroup{Name: "runner", Count: 2, Cpu: 4, RamMb: 8 * 1024}
}

func dockerRef() *dslpb.ProviderRef {
	return &dslpb.ProviderRef{
		Name: "docker",
		ParamsJson: `{"image":"stroppy-agent:latest","server_addr":"http://gateway:8080",` +
			`"binary_url":"http://gateway:8080/agent/binary","run_id":"run-1"}`,
	}
}

func TestDocker_Provision_OneContainerPerMachineWithBootstrapEnv(t *testing.T) {
	fake := &fakeDockerExec{}
	p := NewDocker(fake, nil)

	groups := []*dslpb.MachineGroup{runnerGroup()}
	result, err := p.Provision(context.Background(), dockerRef(), groups)
	require.NoError(t, err)

	require.Len(t, fake.ensured, 2, "one container per machine")

	byName := map[string]ContainerSpec{}
	for _, spec := range fake.ensured {
		byName[spec.Name] = spec
		require.Equal(t, "stroppy-agent:latest", spec.Image)
		require.NotEmpty(t, spec.Network)
		require.Equal(t, "http://gateway:8080", spec.Env["STROPPY_SERVER_ADDR"])
		require.Equal(t, "http://gateway:8080/agent/binary", spec.Env["STROPPY_AGENT_BINARY_URL"])
		require.Equal(t, "run-1", spec.Env["STROPPY_RUN_ID"])
		require.Equal(t, "runner", spec.Labels["group"])

		// The agent image runs systemd as PID 1: it must be privileged, share
		// the host cgroup ns, get a writable /run tmpfs + the host cgroup fs,
		// and receive its env as a FILE (systemd services do not inherit docker
		// env) at agentdomain.DockerEnvFilePath.
		require.True(t, spec.Privileged, "systemd agent container must be privileged")
		require.Equal(t, "host", spec.CgroupnsMode)
		require.Contains(t, spec.Tmpfs, "/run")
		require.Contains(t, spec.Binds, "/sys/fs/cgroup:/sys/fs/cgroup:rw")
		var envFile *ContainerFile
		for i := range spec.Files {
			if spec.Files[i].Path == agentdomain.DockerEnvFilePath {
				envFile = &spec.Files[i]
			}
		}
		require.NotNil(t, envFile, "agent env must be delivered as %s", agentdomain.DockerEnvFilePath)
		require.Contains(t, string(envFile.Content), "STROPPY_SERVER_ADDR=http://gateway:8080")
	}
	require.Len(t, byName, 2, "container names must be unique per machine")

	require.Contains(t, result, "runner")
	require.Len(t, result["runner"], 2)
	for _, m := range result["runner"] {
		require.NotEmpty(t, m.GetProviderResourceId())
		found := false
		for _, ep := range m.GetEndpoints() {
			if ep.GetName() == "private" {
				found = true
				require.NotEmpty(t, ep.GetAddress())
			}
		}
		require.True(t, found, "machine state must carry a private endpoint with the container ip")
	}
}

// fakeTokenIssuer records the (tenant, run, machine, queue) tuples it was
// asked to mint tokens for and returns a deterministic sentinel token.
type fakeTokenIssuer struct {
	calls []struct{ tenant, run, machine, queue string }
}

func (f *fakeTokenIssuer) IssueAgentToken(tenantID, runID, machineID, taskQueue string) (string, error) {
	f.calls = append(f.calls, struct{ tenant, run, machine, queue string }{tenantID, runID, machineID, taskQueue})
	return "token-" + machineID, nil
}

// TestDocker_Provision_IssuesAgentTokenPerNode locks the token-injection path:
// when an issuer is wired, every agent container is rendered with a valid
// STROPPY_AGENT_TOKEN and an AGENT_TASK_QUEUE equal to the deterministic
// TaskQueue(nodeID) the token was minted for (so RunRecipeWorkflow's queue
// routing agrees). Without this the agent refuses to start.
func TestDocker_Provision_IssuesAgentTokenPerNode(t *testing.T) {
	fake := &fakeDockerExec{}
	issuer := &fakeTokenIssuer{}
	p := NewDocker(fake, issuer)

	ref := &dslpb.ProviderRef{
		Name: "docker",
		ParamsJson: `{"image":"stroppy-agent:latest","server_addr":"http://gateway:8080",` +
			`"run_id":"run-1","tenant_id":"tenant-1"}`,
	}
	_, err := p.Provision(context.Background(), ref, []*dslpb.MachineGroup{runnerGroup()})
	require.NoError(t, err)

	require.Len(t, fake.ensured, 2)
	for _, spec := range fake.ensured {
		nodeID := spec.Labels["stroppy.cloud/node_id"]
		require.Equal(t, "token-"+nodeID, spec.Env["STROPPY_AGENT_TOKEN"])
		require.Equal(t, agentdomain.TaskQueue(nodeID), spec.Env["AGENT_TASK_QUEUE"])
	}
	require.Len(t, issuer.calls, 2)
	for _, c := range issuer.calls {
		require.Equal(t, "tenant-1", c.tenant)
		require.Equal(t, "run-1", c.run)
		require.Equal(t, agentdomain.TaskQueue(c.machine), c.queue)
	}
}

// TestDocker_Provision_StampsEmptyDiskDeviceLabel is the I2 provider-side
// lock for the docker builtin provider: containers have no block device
// (ContainerSpec has no volume/bind plumbing today), so disk_device must
// still be present on the private endpoint — explicitly empty, documenting
// the "not applicable" convention rather than silently omitting the key.
func TestDocker_Provision_StampsEmptyDiskDeviceLabel(t *testing.T) {
	fake := &fakeDockerExec{}
	p := NewDocker(fake, nil)

	groups := []*dslpb.MachineGroup{runnerGroup()}
	result, err := p.Provision(context.Background(), dockerRef(), groups)
	require.NoError(t, err)

	require.Contains(t, result, "runner")
	require.NotEmpty(t, result["runner"])
	for _, m := range result["runner"] {
		found := false
		for _, ep := range m.GetEndpoints() {
			if ep.GetName() == "private" {
				found = true
				value, ok := ep.GetLabels()["disk_device"]
				require.True(t, ok, "disk_device label must be present (documented as intentionally empty)")
				require.Empty(t, value)
			}
		}
		require.True(t, found, "machine state must carry a private endpoint")
	}
}

// gatewayGroup is a single-node control-plane group: docker recipes are
// expected to size the gateway group at 1 (see dockerParams.GatewayGroup
// doc), so the Nomad sidecar renders exactly once for it.
func gatewayGroup() *dslpb.MachineGroup {
	return &dslpb.MachineGroup{Name: "gateway", Count: 1, Cpu: 2, RamMb: 4 * 1024}
}

func dockerRefWithGateway() *dslpb.ProviderRef {
	return &dslpb.ProviderRef{
		Name: "docker",
		ParamsJson: `{"image":"stroppy-agent:latest","server_addr":"http://gateway:8080",` +
			`"binary_url":"http://gateway:8080/agent/binary","run_id":"run-1",` +
			`"gateway_group":"gateway"}`,
	}
}

func TestDocker_Provision_GatewayGroup_RendersNomadSidecarContainer(t *testing.T) {
	fake := &fakeDockerExec{}
	p := NewDocker(fake, nil)

	groups := []*dslpb.MachineGroup{gatewayGroup(), runnerGroup()}
	result, err := p.Provision(context.Background(), dockerRefWithGateway(), groups)
	require.NoError(t, err)

	// 1 gateway agent container + 1 nomad sidecar + 2 runner agent containers.
	require.Len(t, fake.ensured, 4)

	var nomadSpec *ContainerSpec
	for i := range fake.ensured {
		if fake.ensured[i].Labels["stroppy.cloud/role"] == "nomad-gateway" {
			nomadSpec = &fake.ensured[i]
		}
	}
	require.NotNil(t, nomadSpec, "nomad gateway sidecar must be rendered")

	require.Equal(t, "hashicorp/nomad:"+agentdomain.DefaultNomadVersion, nomadSpec.Image)
	require.True(t, nomadSpec.Privileged, "nomad sidecar must run privileged for the docker task driver")
	require.Contains(t, nomadSpec.Binds, "/var/run/docker.sock:/var/run/docker.sock")
	require.Contains(t, nomadSpec.Binds, "/sys/fs/cgroup:/sys/fs/cgroup:rw")
	require.Equal(t, "host", nomadSpec.CgroupnsMode, "nomad client needs the host cgroup ns on cgroup v2")
	require.Equal(t, []string{"agent", "-config=" + agentdomain.NomadConfigDir}, nomadSpec.Cmd)
	require.NotEmpty(t, nomadSpec.Network)

	require.Len(t, nomadSpec.Files, 1)
	configFile := nomadSpec.Files[0]
	require.Equal(t, agentdomain.NomadServerHCLPath, configFile.Path)
	// The gateway group here is "gateway", so its first node is "gateway-0";
	// the sidecar stamps that as meta.stroppy_node_id so service jobs targeting
	// the gateway node place (nomad.BuildJob's per-node constraint).
	require.Equal(t, agentdomain.NomadGatewayHCL("gateway-0"), string(configFile.Content))
	// The rendered server config must actually configure a Nomad server +
	// docker driver and stamp the gateway node meta, not just be non-empty.
	require.Contains(t, string(configFile.Content), "server {")
	require.Contains(t, string(configFile.Content), `plugin "docker"`)
	require.Contains(t, string(configFile.Content), `stroppy_node_id = "gateway-0"`)

	// The nomad sidecar is infrastructure the docker provider adds
	// transparently; it must not show up as a requested machine's state.
	require.Contains(t, result, "gateway")
	require.Len(t, result["gateway"], 1)
	require.Contains(t, result, "runner")
	require.Len(t, result["runner"], 2)
}

func TestDocker_Provision_NoGatewayGroupSet_NoNomadSidecar(t *testing.T) {
	fake := &fakeDockerExec{}
	p := NewDocker(fake, nil)

	groups := []*dslpb.MachineGroup{runnerGroup()}
	_, err := p.Provision(context.Background(), dockerRef(), groups)
	require.NoError(t, err)

	for _, spec := range fake.ensured {
		require.NotEqual(t, "nomad-gateway", spec.Labels["stroppy.cloud/role"])
	}
	require.Len(t, fake.ensured, 2, "no extra sidecar container when gateway_group is unset")
}

func TestDocker_Destroy_RemovesRunNetwork(t *testing.T) {
	fake := &fakeDockerExec{}
	p := NewDocker(fake, nil)

	err := p.Destroy(context.Background(), dockerRef())
	require.NoError(t, err)
	require.Len(t, fake.removedNetworks, 1)
}
