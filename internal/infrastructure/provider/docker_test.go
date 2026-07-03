package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

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
	p := NewDocker(fake)

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

func TestDocker_Destroy_RemovesRunNetwork(t *testing.T) {
	fake := &fakeDockerExec{}
	p := NewDocker(fake)

	err := p.Destroy(context.Background(), dockerRef())
	require.NoError(t, err)
	require.Len(t, fake.removedNetworks, 1)
}
