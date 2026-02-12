package edge

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	dockerClient "github.com/docker/docker/client"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/provision"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/stroppy"
)

const (
	bridgeDriver = "bridge"

	containerMetadataDockerIPKey      = "docker.network.ipv4"
	containerMetadataPlacementNodeKey = "docker.placement.node"
	containerMetadataLogicalNameKey   = "docker.logical_name"

	containerLabelManagedByKey = "managed_by"
	containerLabelManagedByVal = "hatchet-edge"
	containerLabelRunIDKey     = "run_id"
	containerLabelWorkerIPKey  = "worker_ip"
	containerLabelLogicalKey   = "logical_name"
)

type ContainerRunner struct {
	client      *dockerClient.Client
	networkName string
	networkID   string
	mu          sync.Mutex
	containers  map[string]string
}

type startedContainer struct {
	name string
	id   string
}

type runContainerOptions struct {
	dockerTarget   bool
	runID          string
	workerInternal string
	publishPorts   bool
}

func NewContainerRunner(networkName string) (*ContainerRunner, error) {
	cli, err := dockerClient.NewClientWithOpts(
		dockerClient.FromEnv,
		dockerClient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &ContainerRunner{
		client:      cli,
		networkName: networkName,
		containers:  make(map[string]string),
	}, nil
}

func (r *ContainerRunner) DeployContainersForTarget(
	ctx context.Context,
	testRunCtx *stroppy.TestRunContext,
	workerInternalIP string,
	containers []*provision.Container,
) error {
	opts := runContainerOptions{publishPorts: true}
	if dockerSettings := testRunCtx.GetSelectedTarget().GetDockerSettings(); dockerSettings != nil {
		if err := r.setNetworkName(dockerSettings.GetNetworkName()); err != nil {
			return err
		}
		opts = runContainerOptions{
			dockerTarget:   true,
			runID:          testRunCtx.GetRunId(),
			workerInternal: workerInternalIP,
			publishPorts:   false,
		}
	}
	return r.deployContainers(ctx, containers, opts)
}

func (r *ContainerRunner) deployContainers(
	ctx context.Context,
	containers []*provision.Container,
	opts runContainerOptions,
) error {
	started := make([]startedContainer, 0, len(containers))

	for _, c := range containers {
		if c == nil {
			return fmt.Errorf("container spec is nil")
		}

		startedContainer, err := r.runContainer(ctx, c, opts)
		if err != nil {
			r.rollbackBatch(ctx, started)
			return fmt.Errorf("failed to run container %q: %w", c.GetName(), err)
		}

		r.mu.Lock()
		r.containers[startedContainer.name] = startedContainer.id
		r.mu.Unlock()
		started = append(started, startedContainer)
	}

	return nil
}

func (r *ContainerRunner) Cleanup(ctx context.Context) {
	r.mu.Lock()
	tracked := make(map[string]string, len(r.containers))
	for name, id := range r.containers {
		tracked[name] = id
	}
	r.mu.Unlock()

	for name, id := range tracked {
		if err := r.stopContainer(ctx, id); err != nil {
			log.Printf("failed to cleanup container %s (%s): %v", name, id, err)
			continue
		}

		r.mu.Lock()
		delete(r.containers, name)
		r.mu.Unlock()
	}
}

func (r *ContainerRunner) Close() error {
	return r.client.Close()
}

func (r *ContainerRunner) runContainer(
	ctx context.Context,
	c *provision.Container,
	opts runContainerOptions,
) (startedContainer, error) {
	if err := r.ensureNetwork(ctx); err != nil {
		return startedContainer{}, err
	}

	if err := r.pullImage(ctx, c.GetImage()); err != nil {
		return startedContainer{}, fmt.Errorf("failed to pull image %q: %w", c.GetImage(), err)
	}

	containerCfg := toContainerConfig(c, opts)
	hostCfg, err := toHostConfig(c, opts)
	if err != nil {
		return startedContainer{}, fmt.Errorf("failed to map host config for container %q: %w", c.GetName(), err)
	}
	containerName := containerRuntimeName(c, opts)
	networkCfg := toNetworkConfig(r.getNetworkName(), r.getNetworkID(), c, opts)

	resp, err := r.client.ContainerCreate(
		ctx,
		containerCfg,
		hostCfg,
		networkCfg,
		nil,
		containerName,
	)
	if err != nil {
		return startedContainer{}, fmt.Errorf("failed to create container %q: %w", containerName, err)
	}

	if err := r.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = r.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return startedContainer{}, fmt.Errorf("failed to start container %q: %w", containerName, err)
	}

	return startedContainer{name: containerName, id: resp.ID}, nil
}

func (r *ContainerRunner) ensureNetwork(ctx context.Context) error {
	r.mu.Lock()
	if r.networkID != "" {
		r.mu.Unlock()
		return nil
	}
	networkName := r.networkName
	r.mu.Unlock()

	inspect, err := r.client.NetworkInspect(ctx, networkName, network.InspectOptions{})
	if err == nil {
		r.setNetworkID(inspect.ID)
		return nil
	}

	resp, createErr := r.client.NetworkCreate(ctx, networkName, network.CreateOptions{Driver: bridgeDriver})
	if createErr != nil {
		inspect, inspectErr := r.client.NetworkInspect(ctx, networkName, network.InspectOptions{})
		if inspectErr == nil {
			r.setNetworkID(inspect.ID)
			return nil
		}
		return fmt.Errorf("failed to create docker network %s: %w", networkName, createErr)
	}

	r.setNetworkID(resp.ID)
	return nil
}

func (r *ContainerRunner) pullImage(ctx context.Context, imageName string) error {
	reader, err := r.client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	_, err = io.Copy(io.Discard, reader)
	return err
}

func (r *ContainerRunner) stopContainer(ctx context.Context, containerID string) error {
	timeoutSeconds := 10
	stopErr := r.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeoutSeconds})
	removeErr := r.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})

	if stopErr != nil && removeErr != nil {
		return fmt.Errorf(
			"failed to stop container %s: %w; failed to remove container %s: %v",
			containerID,
			stopErr,
			containerID,
			removeErr,
		)
	}
	if stopErr != nil {
		return fmt.Errorf("failed to stop container %s: %w", containerID, stopErr)
	}
	if removeErr != nil {
		return fmt.Errorf("failed to remove container %s: %w", containerID, removeErr)
	}
	return nil
}

func (r *ContainerRunner) rollbackBatch(ctx context.Context, started []startedContainer) {
	for i := len(started) - 1; i >= 0; i-- {
		name := started[i].name
		containerID := started[i].id

		if err := r.stopContainer(ctx, containerID); err != nil {
			log.Printf("failed to rollback container %s (%s): %v", name, containerID, err)
			continue
		}

		r.mu.Lock()
		delete(r.containers, name)
		r.mu.Unlock()
	}
}

func (r *ContainerRunner) getNetworkID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.networkID
}

func (r *ContainerRunner) getNetworkName() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.networkName
}

func (r *ContainerRunner) setNetworkID(networkID string) {
	r.mu.Lock()
	r.networkID = networkID
	r.mu.Unlock()
}

func (r *ContainerRunner) setNetworkName(networkName string) error {
	if networkName == "" {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.networkID != "" && r.networkName != networkName {
		return fmt.Errorf("docker network is already initialized as %q", r.networkName)
	}
	r.networkName = networkName
	return nil
}

func containerRuntimeName(c *provision.Container, opts runContainerOptions) string {
	base := containerLogicalName(c)
	if !opts.dockerTarget {
		return base
	}

	parts := []string{sanitizeDockerNamePart(opts.runID), sanitizeDockerNamePart(opts.workerInternal), sanitizeDockerNamePart(base)}
	return strings.Trim(strings.Join(parts, "-"), "-")
}

func containerLogicalName(c *provision.Container) string {
	if c.GetName() != "" {
		return c.GetName()
	}
	if c.GetId() != "" {
		return c.GetId()
	}
	return "container"
}

func sanitizeDockerNamePart(s string) string {
	if s == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z':
			b.WriteByte(ch)
		case ch >= 'A' && ch <= 'Z':
			b.WriteByte(ch + ('a' - 'A'))
		case ch >= '0' && ch <= '9':
			b.WriteByte(ch)
		case ch == '.' || ch == '-' || ch == '_':
			b.WriteByte(ch)
		default:
			b.WriteByte('-')
		}
	}

	out := strings.Trim(b.String(), "-._")
	if out == "" {
		return "c"
	}
	return out
}
