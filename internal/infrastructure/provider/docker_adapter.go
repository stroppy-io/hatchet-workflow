package provider

import (
	"context"
	"fmt"
	"sync"

	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
)

// dockerRunner is the minimal docker.Executor surface dockerExecutorExec
// needs: batch Up/Down over a *Docker_Input. *docker.Executor satisfies it
// as-is (its Up already pulls the image, with fallback, before creating the
// container, so the adapter never needs to call Executor.Pull separately);
// tests inject a fake so the built Docker_Input can be observed without a
// docker daemon.
type dockerRunner interface {
	Up(ctx context.Context, input *deploymentpb.Docker_Input) (*deploymentpb.Docker_Output, error)
	Down(ctx context.Context, input *deploymentpb.Docker_Input) (*deploymentpb.Docker_Output, error)
}

// dockerExecutorExec adapts a dockerRunner (batch Up/Down over Docker_Input)
// to the per-machine dockerExec interface dockerProvider depends on.
// EnsureContainer builds a single-entry Docker_Input and calls Up;
// RemoveContainers builds a Docker_Input carrying every container name
// EnsureContainer has tracked for that network and calls Down. Tracking is
// required because Destroy (see provider.go's dockerExec doc) receives only
// the network name, not the machine list.
type dockerExecutorExec struct {
	exec dockerRunner

	mu    sync.Mutex
	byNet map[string][]string // networkName -> container names, for RemoveContainers
}

// NewDockerExecutorExec builds a dockerExec backed by exec.
func NewDockerExecutorExec(exec dockerRunner) *dockerExecutorExec {
	return &dockerExecutorExec{exec: exec, byNet: make(map[string][]string)}
}

// EnsureContainer creates/replaces and starts a single container by calling
// Up with a 1-entry Docker_Input, then reads that container's runtime state
// back out of the Docker_Output.
//
// If the output has no entry at all for spec.Name, that means Up returned
// successfully without reporting the container it was asked to create — an
// inconsistent result the adapter cannot recover a ContainerState from, so
// this is an error. A present entry with an empty InternalIp, however, is
// not an error: containerOutput (docker/executor.go) only fills InternalIp
// once the container has network settings, which is not guaranteed to be
// immediate, so an empty IP is returned as-is for the caller to handle.
func (a *dockerExecutorExec) EnsureContainer(ctx context.Context, spec ContainerSpec) (ContainerState, error) {
	input := &deploymentpb.Docker_Input{
		Network: &deploymentpb.Docker_Network{Name: spec.Network},
		Containers: map[string]*deploymentpb.Docker_Container{
			spec.Name: {
				Image:  spec.Image,
				Env:    spec.Env,
				Labels: spec.Labels,
			},
		},
	}

	output, err := a.exec.Up(ctx, input)
	if err != nil {
		return ContainerState{}, fmt.Errorf("docker up %q: %w", spec.Name, err)
	}

	out, ok := output.GetContainers()[spec.Name]
	if !ok {
		return ContainerState{}, fmt.Errorf("docker up %q: no container output returned", spec.Name)
	}

	a.track(spec.Network, spec.Name)

	return ContainerState{ID: out.GetId(), InternalIP: out.GetInternalIp()}, nil
}

// RemoveContainers removes every container EnsureContainer has tracked for
// networkName (plus the network itself) by calling Down, then clears the
// tracking for that network.
func (a *dockerExecutorExec) RemoveContainers(ctx context.Context, networkName string) error {
	a.mu.Lock()
	names := a.byNet[networkName]
	a.mu.Unlock()

	containers := make(map[string]*deploymentpb.Docker_Container, len(names))
	for _, name := range names {
		containers[name] = &deploymentpb.Docker_Container{}
	}

	input := &deploymentpb.Docker_Input{
		Network:    &deploymentpb.Docker_Network{Name: networkName},
		Containers: containers,
	}

	if _, err := a.exec.Down(ctx, input); err != nil {
		return fmt.Errorf("docker down %q: %w", networkName, err)
	}

	a.mu.Lock()
	delete(a.byNet, networkName)
	a.mu.Unlock()

	return nil
}

func (a *dockerExecutorExec) track(networkName, containerName string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, name := range a.byNet[networkName] {
		if name == containerName {
			return
		}
	}
	a.byNet[networkName] = append(a.byNet[networkName], containerName)
}

var _ dockerExec = (*dockerExecutorExec)(nil)
