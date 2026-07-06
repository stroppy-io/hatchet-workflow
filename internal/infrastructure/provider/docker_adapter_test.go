package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
)

// fakeDockerRunner is a test double for dockerRunner: it records the
// *Docker_Input each Up/Down call was built with and returns canned results.
type fakeDockerRunner struct {
	upInput *deploymentpb.Docker_Input
	upOut   *deploymentpb.Docker_Output
	upErr   error

	downInput *deploymentpb.Docker_Input
	downOut   *deploymentpb.Docker_Output
	downErr   error

	// downFn, if set, runs while Down is "in flight" (before it returns),
	// letting tests simulate a concurrent tracker mutating adapter state
	// between RemoveContainers' snapshot-read and its post-Down clear.
	downFn func()
}

func (f *fakeDockerRunner) Up(_ context.Context, input *deploymentpb.Docker_Input) (*deploymentpb.Docker_Output, error) {
	f.upInput = input
	return f.upOut, f.upErr
}

func (f *fakeDockerRunner) Down(_ context.Context, input *deploymentpb.Docker_Input) (*deploymentpb.Docker_Output, error) {
	f.downInput = input
	if f.downFn != nil {
		f.downFn()
	}
	return f.downOut, f.downErr
}

func TestDockerExecutorExec_EnsureContainer_BuildsInputAndReturnsState(t *testing.T) {
	fake := &fakeDockerRunner{
		upOut: &deploymentpb.Docker_Output{
			Containers: map[string]*deploymentpb.Docker_ContainerOutput{
				"stroppy-run1-node-0": {Id: "container-id-1", InternalIp: "10.0.0.5"},
			},
		},
	}
	adapter := NewDockerExecutorExec(fake)

	spec := ContainerSpec{
		Name:    "stroppy-run1-node-0",
		Image:   "stroppy/agent:latest",
		Network: "stroppy-run1",
		Env:     map[string]string{"RUN_ID": "run1"},
		Labels:  map[string]string{"stroppy.cloud/run_id": "run1"},
	}

	state, err := adapter.EnsureContainer(context.Background(), spec)
	require.NoError(t, err)
	require.Equal(t, ContainerState{ID: "container-id-1", InternalIP: "10.0.0.5"}, state)

	require.NotNil(t, fake.upInput)
	require.Equal(t, "stroppy-run1", fake.upInput.GetNetwork().GetName())
	require.Len(t, fake.upInput.GetContainers(), 1)
	container := fake.upInput.GetContainers()["stroppy-run1-node-0"]
	require.NotNil(t, container)
	require.Equal(t, "stroppy/agent:latest", container.GetImage())
	require.Equal(t, map[string]string{"RUN_ID": "run1"}, container.GetEnv())
	require.Equal(t, map[string]string{"stroppy.cloud/run_id": "run1"}, container.GetLabels())
}

func TestDockerExecutorExec_EnsureContainer_MapsPrivilegedBindsCmdFiles(t *testing.T) {
	fake := &fakeDockerRunner{
		upOut: &deploymentpb.Docker_Output{
			Containers: map[string]*deploymentpb.Docker_ContainerOutput{
				"stroppy-run1-nomad": {Id: "container-id-nomad", InternalIp: "10.0.0.9"},
			},
		},
	}
	adapter := NewDockerExecutorExec(fake)

	spec := ContainerSpec{
		Name:       "stroppy-run1-nomad",
		Image:      "hashicorp/nomad:1.9.3",
		Network:    "stroppy-run1",
		Privileged: true,
		Binds:      []string{"/var/run/docker.sock:/var/run/docker.sock"},
		Cmd:        []string{"agent", "-config=/etc/nomad.d"},
		Files: []ContainerFile{
			{Path: "/etc/nomad.d/server.hcl", Content: []byte("datacenter = \"dc1\"\n"), Mode: 0o644},
		},
	}

	_, err := adapter.EnsureContainer(context.Background(), spec)
	require.NoError(t, err)

	container := fake.upInput.GetContainers()["stroppy-run1-nomad"]
	require.NotNil(t, container)
	require.True(t, container.GetPrivileged())
	require.Equal(t, []string{"/var/run/docker.sock:/var/run/docker.sock"}, container.GetBinds())
	require.Equal(t, []string{"agent", "-config=/etc/nomad.d"}, container.GetCmd())
	require.Len(t, container.GetFiles(), 1)
	require.Equal(t, "/etc/nomad.d/server.hcl", container.GetFiles()[0].GetPath())
	require.Equal(t, []byte("datacenter = \"dc1\"\n"), container.GetFiles()[0].GetContent())
	require.EqualValues(t, 0o644, container.GetFiles()[0].GetMode())
}

func TestDockerExecutorExec_EnsureContainer_NoFilesSet_NilNotEmptySlice(t *testing.T) {
	fake := &fakeDockerRunner{
		upOut: &deploymentpb.Docker_Output{
			Containers: map[string]*deploymentpb.Docker_ContainerOutput{
				"node-0": {Id: "id-0"},
			},
		},
	}
	adapter := NewDockerExecutorExec(fake)

	_, err := adapter.EnsureContainer(context.Background(), ContainerSpec{Name: "node-0", Network: "net"})
	require.NoError(t, err)

	container := fake.upInput.GetContainers()["node-0"]
	require.NotNil(t, container)
	require.Empty(t, container.GetFiles())
	require.False(t, container.GetPrivileged())
	require.Empty(t, container.GetBinds())
	require.Empty(t, container.GetCmd())
}

func TestDockerExecutorExec_EnsureContainer_MissingOutputEntry_Errors(t *testing.T) {
	fake := &fakeDockerRunner{
		upOut: &deploymentpb.Docker_Output{Containers: map[string]*deploymentpb.Docker_ContainerOutput{}},
	}
	adapter := NewDockerExecutorExec(fake)

	_, err := adapter.EnsureContainer(context.Background(), ContainerSpec{Name: "missing", Network: "net"})
	require.Error(t, err)
}

func TestDockerExecutorExec_EnsureContainer_NoIPYet_ReturnsEmptyIPNotError(t *testing.T) {
	// A container can come up before it has an IP assigned on its network;
	// that is not treated as an adapter error (documented in EnsureContainer).
	fake := &fakeDockerRunner{
		upOut: &deploymentpb.Docker_Output{
			Containers: map[string]*deploymentpb.Docker_ContainerOutput{
				"node-0": {Id: "container-id-2", InternalIp: ""},
			},
		},
	}
	adapter := NewDockerExecutorExec(fake)

	state, err := adapter.EnsureContainer(context.Background(), ContainerSpec{Name: "node-0", Network: "net"})
	require.NoError(t, err)
	require.Equal(t, ContainerState{ID: "container-id-2", InternalIP: ""}, state)
}

func TestDockerExecutorExec_EnsureContainer_PropagatesRunnerError(t *testing.T) {
	fake := &fakeDockerRunner{upErr: assertErr}
	adapter := NewDockerExecutorExec(fake)

	_, err := adapter.EnsureContainer(context.Background(), ContainerSpec{Name: "node-0", Network: "net"})
	require.ErrorIs(t, err, assertErr)
}

func TestDockerExecutorExec_RemoveContainers_DownsAllTrackedNamesForNetwork(t *testing.T) {
	fake := &fakeDockerRunner{
		upOut: &deploymentpb.Docker_Output{
			Containers: map[string]*deploymentpb.Docker_ContainerOutput{
				"node-0": {Id: "id-0", InternalIp: "10.0.0.1"},
			},
		},
	}
	adapter := NewDockerExecutorExec(fake)

	_, err := adapter.EnsureContainer(context.Background(), ContainerSpec{Name: "node-0", Network: "stroppy-run1"})
	require.NoError(t, err)
	fake.upOut = &deploymentpb.Docker_Output{
		Containers: map[string]*deploymentpb.Docker_ContainerOutput{
			"node-1": {Id: "id-1", InternalIp: "10.0.0.2"},
		},
	}
	_, err = adapter.EnsureContainer(context.Background(), ContainerSpec{Name: "node-1", Network: "stroppy-run1"})
	require.NoError(t, err)

	err = adapter.RemoveContainers(context.Background(), "stroppy-run1")
	require.NoError(t, err)

	require.NotNil(t, fake.downInput)
	require.Equal(t, "stroppy-run1", fake.downInput.GetNetwork().GetName())
	require.Len(t, fake.downInput.GetContainers(), 2)
	require.Contains(t, fake.downInput.GetContainers(), "node-0")
	require.Contains(t, fake.downInput.GetContainers(), "node-1")

	// A second RemoveContainers for the same (now untracked) network calls
	// Down again with an empty container set — Down is idempotent (it only
	// removes named containers then the network), and tracking is cleared so
	// nothing stale is repeated.
	err = adapter.RemoveContainers(context.Background(), "stroppy-run1")
	require.NoError(t, err)
	require.Empty(t, fake.downInput.GetContainers())
	require.Equal(t, "stroppy-run1", fake.downInput.GetNetwork().GetName())
}

func TestRemoveContainersPreservesConcurrentlyTrackedContainer(t *testing.T) {
	fake := &fakeDockerRunner{
		upOut: &deploymentpb.Docker_Output{
			Containers: map[string]*deploymentpb.Docker_ContainerOutput{
				"c1": {Id: "id-c1"},
			},
		},
	}
	adapter := NewDockerExecutorExec(fake)

	_, err := adapter.EnsureContainer(context.Background(), ContainerSpec{Name: "c1", Network: "n"})
	require.NoError(t, err)
	fake.upOut = &deploymentpb.Docker_Output{
		Containers: map[string]*deploymentpb.Docker_ContainerOutput{
			"c2": {Id: "id-c2"},
		},
	}
	_, err = adapter.EnsureContainer(context.Background(), ContainerSpec{Name: "c2", Network: "n"})
	require.NoError(t, err)

	// Simulate a concurrent EnsureContainer for the same network racing with
	// RemoveContainers: it tracks c3 while Down (for c1+c2) is in flight,
	// i.e. after RemoveContainers has already snapshotted the names to
	// remove but before it clears byNet["n"].
	fake.downFn = func() {
		adapter.track("n", "c3")
	}

	err = adapter.RemoveContainers(context.Background(), "n")
	require.NoError(t, err)

	// Down must only have been asked to remove what it snapshotted.
	require.Len(t, fake.downInput.GetContainers(), 2)
	require.Contains(t, fake.downInput.GetContainers(), "c1")
	require.Contains(t, fake.downInput.GetContainers(), "c2")
	require.NotContains(t, fake.downInput.GetContainers(), "c3")

	// c3, tracked concurrently, must survive the clear: it was never in the
	// removed set, so it must still be tracked for a future RemoveContainers.
	adapter.mu.Lock()
	remaining := adapter.byNet["n"]
	adapter.mu.Unlock()
	require.Equal(t, []string{"c3"}, remaining)
}

func TestDockerExecutorExec_RemoveContainers_PropagatesRunnerError(t *testing.T) {
	fake := &fakeDockerRunner{downErr: assertErr}
	adapter := NewDockerExecutorExec(fake)

	err := adapter.RemoveContainers(context.Background(), "net")
	require.ErrorIs(t, err, assertErr)
}

var _ dockerExec = (*dockerExecutorExec)(nil)
