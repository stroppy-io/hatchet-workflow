package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
)

type Executor struct {
	cli    *client.Client
	stderr io.Writer
	// attachNetwork, when set, is an additional docker network every agent
	// container is connected to so it can reach the control-plane gateway
	// (`server:8080`) for the agent binary, apt cache and monitoring. The
	// per-run network only links the run's own nodes to each other.
	attachNetwork string
}

type ExecutorOption func(*Executor)

func WithStderr(w io.Writer) ExecutorOption {
	return func(e *Executor) {
		e.stderr = w
	}
}

func NewExecutor(opts ...ExecutorOption) (*Executor, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	e := &Executor{cli: cli, stderr: os.Stderr, attachNetwork: os.Getenv("AGENT_ATTACH_NETWORK")}
	for _, opt := range opts {
		opt(e)
	}
	if e.stderr == nil {
		e.stderr = io.Discard
	}
	return e, nil
}

func (e *Executor) Close() error {
	return e.cli.Close()
}

func (e *Executor) Pull(ctx context.Context, input *deployment.Docker_Input) (*deployment.Docker_Output, error) {
	if input == nil {
		return nil, fmt.Errorf("docker input is required")
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}
	for _, name := range sortedContainerNames(input.GetContainers()) {
		spec := input.GetContainers()[name]
		if _, err := e.pullImage(ctx, spec.GetImage(), spec.GetFallbackImage()); err != nil {
			return nil, fmt.Errorf("pull image for %s: %w", name, err)
		}
	}
	return &deployment.Docker_Output{}, nil
}

func (e *Executor) Up(ctx context.Context, input *deployment.Docker_Input) (*deployment.Docker_Output, error) {
	if input == nil {
		return nil, fmt.Errorf("docker input is required")
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}

	networkName := input.GetNetwork().GetName()
	networkID := ""
	if networkName != "" {
		id, err := e.ensureNetwork(ctx, networkName)
		if err != nil {
			return nil, err
		}
		networkID = id
	}

	output := &deployment.Docker_Output{
		Containers: make(map[string]*deployment.Docker_ContainerOutput, len(input.GetContainers())),
		NetworkId:  networkID,
	}

	for _, name := range sortedByDependencies(input.GetContainers()) {
		spec := input.GetContainers()[name]
		resolvedImage, err := e.pullImage(ctx, spec.GetImage(), spec.GetFallbackImage())
		if err != nil {
			return nil, fmt.Errorf("pull image for %s: %w", name, err)
		}
		if err := e.removeContainer(ctx, name); err != nil {
			return nil, err
		}

		cfg := containerConfig(resolvedImage, spec)
		hostCfg, err := hostConfig(input.GetNetwork(), spec)
		if err != nil {
			return nil, fmt.Errorf("docker host config %s: %w", name, err)
		}
		netCfg := networkingConfig(networkName)

		resp, err := e.cli.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, name)
		if err != nil {
			return nil, fmt.Errorf("docker create %s: %w", name, err)
		}
		// Connect the agent to the control-plane network (before start, so its
		// embedded DNS resolves `server` at boot) in addition to the per-run
		// network. Without this the agent cannot fetch its binary from the
		// gateway and never comes online.
		if e.attachNetwork != "" && e.attachNetwork != networkName {
			if err := e.cli.NetworkConnect(ctx, e.attachNetwork, resp.ID, nil); err != nil {
				return nil, fmt.Errorf("docker connect %s to %s: %w", name, e.attachNetwork, err)
			}
		}
		for _, file := range spec.GetFiles() {
			if err := e.copyFileToContainer(ctx, resp.ID, file); err != nil {
				return nil, fmt.Errorf("docker copy file to %s: %w", name, err)
			}
		}
		if err := e.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
			return nil, fmt.Errorf("docker start %s: %w", name, err)
		}
		inspect, err := e.cli.ContainerInspect(ctx, resp.ID)
		if err != nil {
			return nil, fmt.Errorf("docker inspect %s: %w", name, err)
		}
		output.Containers[name] = containerOutput(name, networkName, inspect)
	}

	return output, nil
}

func (e *Executor) Down(ctx context.Context, input *deployment.Docker_Input) (*deployment.Docker_Output, error) {
	if input == nil {
		return nil, fmt.Errorf("docker input is required")
	}
	for _, name := range sortedContainerNames(input.GetContainers()) {
		_ = e.removeContainer(ctx, name)
	}
	networkName := input.GetNetwork().GetName()
	if networkName != "" {
		_ = e.cli.NetworkRemove(ctx, networkName)
	}
	return &deployment.Docker_Output{}, nil
}

// ContainersByNetwork lists the names of every container currently attached
// to networkName, per Docker's own network-membership tracking — used as
// RemoveContainers' fallback (F5) when the in-process byNet map has nothing
// for this network (e.g. after a control-plane restart), since Docker's
// network membership survives this process' own restarts even though the
// in-memory tracker does not.
func (e *Executor) ContainersByNetwork(ctx context.Context, networkName string) ([]string, error) {
	containers, err := e.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("network", networkName)),
	})
	if err != nil {
		return nil, fmt.Errorf("list containers on network %q: %w", networkName, err)
	}
	names := make([]string, 0, len(containers))
	for _, c := range containers {
		for _, name := range c.Names {
			names = append(names, strings.TrimPrefix(name, "/")) // docker prefixes names with "/"
		}
	}
	return names, nil
}

func (e *Executor) ensureNetwork(ctx context.Context, name string) (string, error) {
	nets, err := e.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("list docker networks: %w", err)
	}
	for _, net := range nets {
		if net.Name == name {
			return net.ID, nil
		}
	}
	resp, err := e.cli.NetworkCreate(ctx, name, network.CreateOptions{
		Driver: "bridge",
		Labels: map[string]string{"stroppy.cloud/managed": "true"},
	})
	if err != nil {
		return "", fmt.Errorf("create docker network %s: %w", name, err)
	}
	return resp.ID, nil
}

func (e *Executor) removeContainer(ctx context.Context, name string) error {
	if err := e.cli.ContainerRemove(ctx, name, container.RemoveOptions{Force: true}); err != nil && !client.IsErrNotFound(err) {
		return fmt.Errorf("docker remove %s: %w", name, err)
	}
	return nil
}

func (e *Executor) pullImage(ctx context.Context, primary, fallback string) (string, error) {
	if primary == "" {
		primary = fallback
	}
	if primary == "" {
		return "", fmt.Errorf("image is required")
	}
	if err := e.pullIfMissing(ctx, primary); err == nil {
		return primary, nil
	}
	if fallback == "" || fallback == primary {
		return "", e.pullIfMissing(ctx, primary)
	}
	if err := e.pullIfMissing(ctx, fallback); err != nil {
		return "", err
	}
	return fallback, nil
}

func (e *Executor) pullIfMissing(ctx context.Context, ref string) error {
	if _, _, err := e.cli.ImageInspectWithRaw(ctx, ref); err == nil {
		return nil
	}
	reader, err := e.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("docker pull %s: %w", ref, err)
	}
	defer reader.Close()
	_, _ = io.Copy(e.stderr, reader)
	return nil
}

func (e *Executor) copyFileToContainer(ctx context.Context, containerID string, file *deployment.Docker_File) error {
	if file == nil {
		return nil
	}
	path := file.GetPath()
	content := file.GetContent()
	mode := int64(file.GetMode())
	if mode == 0 {
		mode = 0o644
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Name: strings.TrimPrefix(path, "/"), Mode: mode, Size: int64(len(content))}); err != nil {
		return err
	}
	if _, err := tw.Write(content); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return e.cli.CopyToContainer(ctx, containerID, "/", &buf, container.CopyToContainerOptions{})
}

func containerConfig(imageRef string, spec *deployment.Docker_Container) *container.Config {
	return &container.Config{
		Image:        imageRef,
		Hostname:     spec.GetHostname(),
		Entrypoint:   spec.GetEntrypoint(),
		Cmd:          spec.GetCmd(),
		Env:          envList(spec.GetEnv()),
		Labels:       spec.GetLabels(),
		ExposedPorts: exposedPorts(spec.GetPorts()),
		Healthcheck:  healthcheck(spec.GetHealthcheck()),
	}
}

func hostConfig(net *deployment.Docker_Network, spec *deployment.Docker_Container) (*container.HostConfig, error) {
	cfg := &container.HostConfig{
		Binds:         append([]string(nil), spec.GetBinds()...),
		DNS:           append([]string(nil), net.GetDns()...),
		PortBindings:  portBindings(spec.GetPorts()),
		Privileged:    spec.GetPrivileged(),
		Tmpfs:         spec.GetTmpfs(),
		CgroupnsMode:  container.CgroupnsMode(spec.GetCgroupnsMode()),
		RestartPolicy: restartPolicy(spec.GetRestartPolicy()),
		Mounts:        volumeMounts(spec.GetVolumes()),
	}
	if resources := spec.GetResources(); resources != nil {
		cfg.Resources.Memory = int64(resources.GetMemoryMb() * 1024 * 1024)
		cfg.Resources.NanoCPUs = int64(resources.GetCpuCores() * 1_000_000_000)
		if resources.GetPidsLimit() > 0 {
			limit := int64(resources.GetPidsLimit())
			cfg.Resources.PidsLimit = &limit
		}
	}
	return cfg, nil
}

func networkingConfig(networkName string) *network.NetworkingConfig {
	if networkName == "" {
		return nil
	}
	return &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			networkName: {},
		},
	}
}

func containerOutput(name, networkName string, inspect container.InspectResponse) *deployment.Docker_ContainerOutput {
	out := &deployment.Docker_ContainerOutput{
		Id:          inspect.ID,
		Name:        name,
		Status:      inspect.State.Status,
		StartedAt:   inspect.State.StartedAt,
		MappedPorts: make(map[uint32]uint32),
	}
	if inspect.NetworkSettings != nil {
		if networkName != "" {
			if ep, ok := inspect.NetworkSettings.Networks[networkName]; ok {
				out.InternalIp = ep.IPAddress
			}
		}
		if out.InternalIp == "" {
			for _, ep := range inspect.NetworkSettings.Networks {
				out.InternalIp = ep.IPAddress
				break
			}
		}
		for port, bindings := range inspect.NetworkSettings.Ports {
			if len(bindings) == 0 {
				continue
			}
			hostPort, _ := strconv.ParseUint(bindings[0].HostPort, 10, 32)
			out.MappedPorts[uint32(port.Int())] = uint32(hostPort)
		}
	}
	return out
}

func envList(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+env[key])
	}
	return out
}

func exposedPorts(bindings []*deployment.Docker_PortBinding) nat.PortSet {
	ports := make(nat.PortSet, len(bindings))
	for _, binding := range bindings {
		port := dockerPort(binding)
		ports[port] = struct{}{}
	}
	return ports
}

func portBindings(bindings []*deployment.Docker_PortBinding) nat.PortMap {
	ports := make(nat.PortMap, len(bindings))
	for _, binding := range bindings {
		port := dockerPort(binding)
		hostPort := ""
		if binding.GetHostPort() > 0 {
			hostPort = strconv.Itoa(int(binding.GetHostPort()))
		}
		ports[port] = append(ports[port], nat.PortBinding{
			HostIP:   binding.GetHostIp(),
			HostPort: hostPort,
		})
	}
	return ports
}

func dockerPort(binding *deployment.Docker_PortBinding) nat.Port {
	proto := "tcp"
	if binding.GetProtocol() == deployment.Docker_PROTOCOL_UDP {
		proto = "udp"
	}
	return nat.Port(fmt.Sprintf("%d/%s", binding.GetContainerPort(), proto))
}

func restartPolicy(policy deployment.Docker_RestartPolicy) container.RestartPolicy {
	switch policy {
	case deployment.Docker_RESTART_POLICY_NO:
		return container.RestartPolicy{Name: "no"}
	case deployment.Docker_RESTART_POLICY_ON_FAILURE:
		return container.RestartPolicy{Name: "on-failure"}
	case deployment.Docker_RESTART_POLICY_ALWAYS:
		return container.RestartPolicy{Name: "always"}
	case deployment.Docker_RESTART_POLICY_UNLESS_STOPPED:
		return container.RestartPolicy{Name: "unless-stopped"}
	default:
		return container.RestartPolicy{}
	}
}

func volumeMounts(volumes []*deployment.Docker_VolumeMount) []mount.Mount {
	mounts := make([]mount.Mount, 0, len(volumes))
	for _, volume := range volumes {
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeVolume,
			Source:   volume.GetName(),
			Target:   volume.GetTarget(),
			ReadOnly: volume.GetReadOnly(),
		})
	}
	return mounts
}

func healthcheck(health *deployment.Docker_Healthcheck) *container.HealthConfig {
	if health == nil {
		return nil
	}
	return &container.HealthConfig{
		Test:        append([]string(nil), health.GetTest()...),
		Interval:    time.Duration(health.GetIntervalSeconds()) * time.Second,
		Timeout:     time.Duration(health.GetTimeoutSeconds()) * time.Second,
		Retries:     int(health.GetRetries()),
		StartPeriod: time.Duration(health.GetStartPeriodSeconds()) * time.Second,
	}
}

func sortedContainerNames(containers map[string]*deployment.Docker_Container) []string {
	names := make([]string, 0, len(containers))
	for name := range containers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedByDependencies(containers map[string]*deployment.Docker_Container) []string {
	names := sortedContainerNames(containers)
	seen := make(map[string]bool, len(names))
	var ordered []string
	var visit func(string)
	visit = func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		for _, dep := range containers[name].GetDependsOn() {
			if _, ok := containers[dep]; ok {
				visit(dep)
			}
		}
		ordered = append(ordered, name)
	}
	for _, name := range names {
		visit(name)
	}
	return ordered
}
