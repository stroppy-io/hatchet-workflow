// Package dockerprov is the DOCKER provider deployer. Given a baked topology
// it deploys one real container per topology Instance (from the agent image)
// via the Docker SDK, returns their IPs on a dedicated bridge network, and
// tears them down again. This emulates cloud VMs 1:1 for the demo.
//
// It is intentionally agnostic of the benchmark recipe: it only brings up the
// agent containers (each runs a stroppy-agent Temporal worker). The workflow
// then drives the agents via Temporal activities.
package dockerprov

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

const (
	// DefaultAgentImage is the default image deployed for each instance.
	DefaultAgentImage = "stroppy-agent:latest"

	// labelRunID labels every resource we create so teardown can find them.
	labelRunID = "stroppy.run-id"
)

// InstanceDeployment describes one deployed container.
type InstanceDeployment struct {
	// ContainerID is the full docker container ID.
	ContainerID string
	// IP is the container's IPv4 address on the run network.
	IP string
	// Name is the container name (stroppy-<runID>-<instanceID>).
	Name string
}

// Deployed is the result of a Deploy call.
type Deployed struct {
	// Network is the name of the bridge network the instances live on.
	Network string
	// Instances maps a topology instance id to its deployed container.
	Instances map[string]InstanceDeployment
}

// Deployer brings agent containers up and down for a run.
type Deployer struct {
	cli     *client.Client
	image   string
	network string
	// ServerAddr is the ONLY thing agents are told: injected as
	// STROPPY_SERVER_ADDR into every container. The agent reaches Temporal (via
	// the gateway's gRPC proxy), downloads its own binary and fetches artifacts /
	// apt packages all through this single address. Falls back to AGENT_SERVER_ADDR
	// / STROPPY_SERVER_ADDR from the deployer's env when empty.
	ServerAddr string
	// cmd overrides the container command. It is nil in production (the agent
	// image defines its own entrypoint) and is only set by tests so a plain
	// base image (e.g. alpine) stays running long enough to be observed.
	cmd []string
}

// serverAddr resolves the address agents should dial. Explicit ServerAddr wins;
// otherwise the deployer's own env (AGENT_SERVER_ADDR / STROPPY_SERVER_ADDR).
func (d *Deployer) serverAddr() string {
	if d.ServerAddr != "" {
		return d.ServerAddr
	}
	if v := os.Getenv("AGENT_SERVER_ADDR"); v != "" {
		return v
	}
	return os.Getenv("STROPPY_SERVER_ADDR")
}

// New constructs a Deployer talking to the host docker daemon via the
// standard environment (DOCKER_HOST / unix:///var/run/docker.sock). The image
// is the agent image to run for each instance; pass DefaultAgentImage in
// production or an arbitrary image (e.g. "alpine") in tests.
func New(image string) (*Deployer, error) {
	if image == "" {
		image = DefaultAgentImage
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("dockerprov: create docker client: %w", err)
	}

	return &Deployer{cli: cli, image: image}, nil
}

// Close releases the underlying docker client.
func (d *Deployer) Close() error {
	if d.cli == nil {
		return nil
	}

	return d.cli.Close()
}

// networkName returns the dedicated per-run bridge the agent VMs share. Each run
// gets its own isolated network (like a cloud VPC): the agents reach each other
// by IP, and reach the control-plane server through the docker host gateway
// (host.docker.internal) — exactly the address the server told them, never by a
// docker service name. This keeps the docker provider a faithful VM emulation.
func networkName(runID string) string {
	return "stroppy-" + runID
}

// containerName returns the per-instance container name.
func containerName(runID, instanceID string) string {
	return fmt.Sprintf("stroppy-%s-%s", runID, instanceID)
}

// AgentQueue is the Temporal task queue dedicated to ONE agent (one machine).
// Exactly one worker registers on it (that instance's agent), so a session /
// activity placed on this queue can only ever land on that machine — this is how
// the workflow routes each component's steps to the exact machine it belongs to.
// The deployer (injecting AGENT_TASK_QUEUE) and the workflow MUST compute it the
// same way.
func AgentQueue(runID, instanceID string) string {
	return fmt.Sprintf("stroppy-agent-%s-%s", runID, instanceID)
}

// Deploy ensures the run network exists and runs one privileged container per
// topology instance, then returns each container's IP on that network. serverAddr
// is the single address injected into every agent VM (the admin-set instance
// address, or a derived docker-host address); empty falls back to the deployer's
// own env.
func (d *Deployer) Deploy(ctx context.Context, topo *topology.Topology, runID, serverAddr string) (*Deployed, error) {
	if topo == nil {
		return nil, errors.New("dockerprov: nil topology")
	}

	if runID == "" {
		return nil, errors.New("dockerprov: empty runID")
	}

	if serverAddr == "" {
		serverAddr = d.serverAddr()
	}

	netName := networkName(runID)
	d.network = netName

	if err := d.ensureNetwork(ctx, netName, runID); err != nil {
		return nil, err
	}

	out := &Deployed{
		Network:   netName,
		Instances: make(map[string]InstanceDeployment),
	}

	for _, inst := range topo.GetInstances() {
		dep, err := d.deployInstance(ctx, inst, netName, runID, serverAddr)
		if err != nil {
			// Best-effort cleanup of whatever we already created so we do not
			// leak containers/networks on a partial failure.
			_ = d.Teardown(context.WithoutCancel(ctx), runID)

			return nil, err
		}

		out.Instances[inst.GetId()] = dep
	}

	return out, nil
}

// ensureNetwork creates the bridge network if it does not already exist.
func (d *Deployer) ensureNetwork(ctx context.Context, netName, runID string) error {
	nets, err := d.cli.NetworkList(ctx, network.ListOptions{
		Filters: filters.NewArgs(filters.Arg("name", netName)),
	})
	if err != nil {
		return fmt.Errorf("dockerprov: list networks: %w", err)
	}

	for _, n := range nets {
		// NetworkList does a substring match, so confirm the exact name.
		if n.Name == netName {
			return nil
		}
	}

	_, err = d.cli.NetworkCreate(ctx, netName, network.CreateOptions{
		Driver: "bridge",
		Labels: map[string]string{labelRunID: runID},
	})
	if err != nil && !cerrdefs.IsConflict(err) {
		return fmt.Errorf("dockerprov: create network %q: %w", netName, err)
	}

	return nil
}

// deployInstance creates, starts and inspects one container for an instance.
func (d *Deployer) deployInstance(
	ctx context.Context,
	inst *topology.Topology_Instance,
	netName, runID, serverAddr string,
) (InstanceDeployment, error) {
	name := containerName(runID, inst.GetId())

	// The agent VM is told ONLY the server address (full URL). Temporal (via the
	// gateway gRPC proxy), the agent binary, artifacts and apt are all reached
	// through it. Passed both as a container env and (for the systemd image) as
	// the EnvironmentFile the stroppy-agent.service reads.
	// Each agent listens on its OWN task queue so the workflow can route this
	// machine's component steps to exactly this agent (deterministic placement).
	taskQueue := AgentQueue(runID, inst.GetId())
	env := []string{
		"STROPPY_SERVER_ADDR=" + serverAddr,
		"STROPPY_MACHINE_ID=" + inst.GetId(),
		"AGENT_TASK_QUEUE=" + taskQueue,
	}

	var resources container.Resources
	if mi := inst.GetMachineInfo(); mi != nil {
		if cores := mi.GetCores(); cores > 0 {
			resources.NanoCPUs = int64(cores) * 1e9
		}

		if memGB := mi.GetMemoryGb(); memGB > 0 {
			resources.Memory = int64(memGB) << 30
		}
	}

	// Run the container as a real systemd machine (PID 1 = systemd) so it emulates
	// a cloud VM 1:1: privileged, host cgroup namespace, the cgroup fs mounted rw,
	// and tmpfs for /run. Public DNS like a VM. host.docker.internal maps to the
	// docker host gateway so the agent reaches the server at the address it was
	// given (per-run bridges have no host.docker.internal by default).
	hostCfg := &container.HostConfig{
		Privileged:   true,
		NetworkMode:  container.NetworkMode(netName),
		Resources:    resources,
		CgroupnsMode: container.CgroupnsModeHost,
		Tmpfs:        map[string]string{"/run": "exec,mode=755", "/run/lock": ""},
		Binds:        []string{"/sys/fs/cgroup:/sys/fs/cgroup:rw"},
		DNS:          []string{"8.8.8.8", "1.1.1.1"},
		ExtraHosts:   []string{"host.docker.internal:host-gateway"},
	}

	netCfg := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			netName: {},
		},
	}

	created, err := d.cli.ContainerCreate(
		ctx,
		&container.Config{
			Image:    d.image,
			Cmd:      d.cmd,
			Env:      env,
			Hostname: name,
			Labels:   map[string]string{labelRunID: runID},
		},
		hostCfg,
		netCfg,
		nil,
		name,
	)
	if err != nil {
		return InstanceDeployment{}, fmt.Errorf("dockerprov: create container %q: %w", name, err)
	}

	// Drop the agent's cloud-init-equivalent files BEFORE start so systemd picks
	// them up on boot: the env file (server address) + the apt proxy config that
	// routes apt through the server's single address.
	envFile := fmt.Sprintf("STROPPY_SERVER_ADDR=%s\nSTROPPY_MACHINE_ID=%s\nAGENT_TASK_QUEUE=%s\n", serverAddr, inst.GetId(), taskQueue)
	if err := d.copyFileToContainer(ctx, created.ID, "/etc/stroppy-agent.env", envFile); err != nil {
		return InstanceDeployment{}, fmt.Errorf("dockerprov: write agent env %q: %w", name, err)
	}
	aptConf := fmt.Sprintf("Acquire::http::Proxy \"%s\";\nAcquire::https::Proxy \"%s\";\n", serverAddr, serverAddr)
	if err := d.copyFileToContainer(ctx, created.ID, "/etc/apt/apt.conf.d/00stroppy-proxy", aptConf); err != nil {
		return InstanceDeployment{}, fmt.Errorf("dockerprov: write apt proxy %q: %w", name, err)
	}

	if err := d.cli.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return InstanceDeployment{}, fmt.Errorf("dockerprov: start container %q: %w", name, err)
	}

	ip, err := d.containerIP(ctx, created.ID, netName)
	if err != nil {
		return InstanceDeployment{}, err
	}

	return InstanceDeployment{
		ContainerID: created.ID,
		IP:          ip,
		Name:        name,
	}, nil
}

// containerIP inspects a container and returns its IPv4 address on netName.
func (d *Deployer) containerIP(ctx context.Context, containerID, netName string) (string, error) {
	info, err := d.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("dockerprov: inspect container %q: %w", containerID, err)
	}

	if info.NetworkSettings != nil {
		if ep, ok := info.NetworkSettings.Networks[netName]; ok && ep != nil && ep.IPAddress != "" {
			return ep.IPAddress, nil
		}
	}

	return "", fmt.Errorf("dockerprov: container %q has no IP on network %q", containerID, netName)
}

// Teardown stops and removes every container created for runID and removes the
// run network. It is idempotent: not-found errors are ignored.
func (d *Deployer) Teardown(ctx context.Context, runID string) error {
	if runID == "" {
		return errors.New("dockerprov: empty runID")
	}

	// Debug hook: keep the agent containers + network alive after a run so their
	// logs can be inspected. Never set in production.
	if os.Getenv("STROPPY_KEEP_AGENTS") == "1" {
		return nil
	}

	netName := networkName(runID)

	containers, err := d.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", labelRunID+"="+runID)),
	})
	if err != nil {
		return fmt.Errorf("dockerprov: list containers: %w", err)
	}

	var errs []error

	for _, c := range containers {
		if err := d.cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		}); err != nil && !cerrdefs.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("dockerprov: remove container %q: %w", c.ID, err))
		}
	}

	// Remove the per-run bridge.
	if err := d.cli.NetworkRemove(ctx, netName); err != nil && !cerrdefs.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("dockerprov: remove network %q: %w", netName, err))
	}

	return errors.Join(errs...)
}

// copyFileToContainer writes content to filePath inside the container (used to
// drop the agent env file + apt proxy config before start, cloud-init style).
func (d *Deployer) copyFileToContainer(ctx context.Context, containerID, filePath, content string) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{Name: filePath, Mode: 0o644, Size: int64(len(content))}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return d.cli.CopyToContainer(ctx, containerID, "/", &buf, container.CopyToContainerOptions{})
}
