// Package dockerprov is the local-Docker DockerProvisioner (dag.Deps seam): it turns
// a deployment.Docker.Input (network + container specs) into running containers and
// reports their internal IPs in Docker.Output. Same docker SDK approach as the old
// agent DockerDeployer, but driven by the proto spec (the container Env — including
// STROPPY_SERVER_ADDR / STROPPY_MACHINE_ID / STROPPY_AGENT_TOKEN — is set upstream by
// the deploy step). Used by real Docker-provider runs AND the docker integration test.
package dockerprov

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
)

// Provisioner implements dag.DockerProvisioner over a docker daemon (FromEnv).
type Provisioner struct {
	cli *client.Client
}

func New() (*Provisioner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &Provisioner{cli: cli}, nil
}

func (p *Provisioner) Close() error { return p.cli.Close() }

// Apply creates the network (if named) and every container in d.Input, then fills
// d.Output with each container's id/name/internal IP and the network id.
func (p *Provisioner) Apply(ctx context.Context, d *deployment.Docker) (*deployment.Docker, error) {
	in := d.GetInput()
	netName := in.GetNetwork().GetName()
	netID, err := p.ensureNetwork(ctx, netName)
	if err != nil {
		return nil, err
	}

	out := &deployment.Docker_Output{
		Containers: make(map[string]*deployment.Docker_ContainerOutput, len(in.GetContainers())),
		NetworkId:  netID,
	}
	for id, spec := range in.GetContainers() {
		name := "stroppy-" + id
		_ = p.cli.ContainerRemove(ctx, name, container.RemoveOptions{Force: true}) // best-effort idempotency

		cfg := &container.Config{
			Image:      spec.GetImage(),
			Hostname:   name,
			Env:        envSlice(spec.GetEnv()),
			Entrypoint: spec.GetEntrypoint(),
			Cmd:        spec.GetCmd(),
			Labels:     spec.GetLabels(),
		}
		hostCfg := &container.HostConfig{
			Privileged:   spec.GetPrivileged(),
			CgroupnsMode: container.CgroupnsMode(spec.GetCgroupnsMode()),
			Tmpfs:        spec.GetTmpfs(),
			Binds:        spec.GetBinds(),
			DNS:          in.GetNetwork().GetDns(),
			// Let the in-container agent reach the control plane on the host
			// (host.docker.internal) even on a custom bridge network.
			ExtraHosts: []string{"host.docker.internal:host-gateway"},
		}
		var netCfg *network.NetworkingConfig
		if netName != "" {
			netCfg = &network.NetworkingConfig{EndpointsConfig: map[string]*network.EndpointSettings{netName: {}}}
		}

		resp, err := p.cli.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, name)
		if err != nil && spec.GetFallbackImage() != "" {
			cfg.Image = spec.GetFallbackImage()
			resp, err = p.cli.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, name)
		}
		if err != nil {
			return nil, fmt.Errorf("docker create %s: %w", name, err)
		}
		// Inject files (e.g. /etc/stroppy-agent.env) BEFORE start so the agent's
		// systemd unit reads them on boot.
		if err := p.copyFiles(ctx, resp.ID, spec.GetFiles()); err != nil {
			return nil, fmt.Errorf("docker copy files %s: %w", name, err)
		}
		if err := p.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
			return nil, fmt.Errorf("docker start %s: %w", name, err)
		}
		ip, err := p.containerIP(ctx, resp.ID, netName)
		if err != nil {
			return nil, err
		}
		out.Containers[id] = &deployment.Docker_ContainerOutput{
			Id:         resp.ID,
			Name:       name,
			InternalIp: ip,
			Status:     "running",
		}
	}
	d.Output = out
	return d, nil
}

// Destroy force-removes the deployment's containers (and the network if owned).
func (p *Provisioner) Destroy(ctx context.Context, d *deployment.Docker) error {
	for _, c := range d.GetOutput().GetContainers() {
		_ = p.cli.ContainerRemove(ctx, c.GetId(), container.RemoveOptions{Force: true})
	}
	if id := d.GetOutput().GetNetworkId(); id != "" {
		_ = p.cli.NetworkRemove(ctx, id)
	}
	return nil
}

// ensureNetwork creates the named network if it does not already exist; returns its
// id. Empty name => containers join the default bridge (no managed network).
func (p *Provisioner) ensureNetwork(ctx context.Context, name string) (string, error) {
	if name == "" {
		return "", nil
	}
	if res, err := p.cli.NetworkInspect(ctx, name, network.InspectOptions{}); err == nil {
		return res.ID, nil
	}
	res, err := p.cli.NetworkCreate(ctx, name, network.CreateOptions{Driver: "bridge"})
	if err != nil {
		return "", fmt.Errorf("docker network %s: %w", name, err)
	}
	return res.ID, nil
}

func (p *Provisioner) containerIP(ctx context.Context, id, netName string) (string, error) {
	info, err := p.cli.ContainerInspect(ctx, id)
	if err != nil {
		return "", fmt.Errorf("docker inspect %s: %w", id, err)
	}
	nets := info.NetworkSettings.Networks
	if netName != "" {
		if ep, ok := nets[netName]; ok {
			return ep.IPAddress, nil
		}
	}
	for _, ep := range nets { // default bridge or first attached network
		if ep.IPAddress != "" {
			return ep.IPAddress, nil
		}
	}
	return "", nil
}

// copyFiles writes spec files into the container as a tar stream rooted at "/".
func (p *Provisioner) copyFiles(ctx context.Context, id string, files []*deployment.Docker_File) error {
	if len(files) == 0 {
		return nil
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, f := range files {
		mode := f.GetMode()
		if mode == 0 {
			mode = 0o644
		}
		// tar entries are relative to the copy root "/".
		name := f.GetPath()
		for len(name) > 0 && name[0] == '/' {
			name = name[1:]
		}
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: int64(mode), Size: int64(len(f.GetContent()))}); err != nil {
			return err
		}
		if _, err := tw.Write(f.GetContent()); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return p.cli.CopyToContainer(ctx, id, "/", &buf, container.CopyToContainerOptions{})
}

func envSlice(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}
