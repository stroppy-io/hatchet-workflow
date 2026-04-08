package agent

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

const (
	// DockerBaseImage is the base image used for agent containers.
	// Pre-built with vmagent + node_exporter to avoid slow GitHub downloads at runtime.
	// Build with: docker build -f deployments/docker/agent.Dockerfile -t stroppy-agent:latest .
	// Falls back to ubuntu:22.04 if custom image not found.
	DockerBaseImage = "stroppy-agent:latest"
)

// DeployResult holds the result of deploying an agent container.
type DeployResult struct {
	ContainerID   string
	ContainerName string
	MappedPort    int // host-mapped port for the agent
}

// DockerDeployer emulates cloud VMs using Docker containers.
type DockerDeployer struct {
	cli         *client.Client
	networkName string
}

// NewDockerDeployer creates a deployer backed by the local Docker daemon.
func NewDockerDeployer(networkName string) (*DockerDeployer, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("agent: docker client: %w", err)
	}
	return &DockerDeployer{cli: cli, networkName: networkName}, nil
}

// Deploy creates and starts a container running the agent in poll mode.
// The agent downloads its binary from the server, then polls for commands.
// No inbound ports are needed — all communication is agent→server.
func (d *DockerDeployer) Deploy(ctx context.Context, machineID string, serverAddr string, agentToken string, _ int) (DeployResult, error) {
	// Use pre-built agent image (systemd-enabled) if available, otherwise fall back to bare ubuntu.
	baseImage := DockerBaseImage
	if err := d.pullIfMissing(ctx, baseImage); err != nil {
		baseImage = "ubuntu:22.04"
		if err := d.pullIfMissing(ctx, baseImage); err != nil {
			return DeployResult{}, err
		}
	}

	useSystemd := baseImage == DockerBaseImage // systemd image

	var cfg *container.Config
	if useSystemd {
		// Systemd container: PID 1 = /lib/systemd/systemd.
		// Use ExecStartPre in the systemd unit to download agent binary before service starts.
		// This way systemd is PID 1 from the start (required for service management).
		cfg = &container.Config{
			Image: baseImage,
			Env: []string{
				fmt.Sprintf("STROPPY_SERVER_ADDR=%s", serverAddr),
				fmt.Sprintf("STROPPY_MACHINE_ID=%s", machineID),
				fmt.Sprintf("STROPPY_AGENT_TOKEN=%s", agentToken),
			},
		}
	} else {
		// Fallback: bare ubuntu, no systemd.
		cfg = &container.Config{
			Image: baseImage,
			Entrypoint: []string{"sh", "-c",
				fmt.Sprintf(
					"set -ex && "+
						"apt-get update && apt-get install -y curl && "+
						"curl -fL --retry 5 --retry-delay 2 %s/agent/binary -o %s && "+
						"chmod +x %s && "+
						"exec %s agent",
					serverAddr, RemoteBinPath, RemoteBinPath, RemoteBinPath),
			},
			Env: []string{
				fmt.Sprintf("STROPPY_SERVER_ADDR=%s", serverAddr),
				fmt.Sprintf("STROPPY_MACHINE_ID=%s", machineID),
				fmt.Sprintf("STROPPY_AGENT_TOKEN=%s", agentToken),
			},
		}
	}

	hostCfg := &container.HostConfig{
		DNS: []string{"8.8.8.8", "1.1.1.1"},
	}

	// Systemd requires privileged mode, cgroup mount, and tmpfs.
	if useSystemd {
		hostCfg.Privileged = true
		hostCfg.Tmpfs = map[string]string{
			"/run":      "exec,mode=755",
			"/run/lock": "",
		}
		hostCfg.CgroupnsMode = "host"
		hostCfg.Binds = append(hostCfg.Binds, "/sys/fs/cgroup:/sys/fs/cgroup:rw")
	}

	var netCfg *network.NetworkingConfig
	if d.networkName != "" {
		netCfg = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				d.networkName: {},
			},
		}
	}

	name := fmt.Sprintf("stroppy-agent-%s", machineID)
	d.cli.ContainerRemove(ctx, name, container.RemoveOptions{Force: true})

	resp, err := d.cli.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, name)
	if err != nil {
		return DeployResult{}, fmt.Errorf("agent: docker create %s: %w", name, err)
	}

	// For systemd containers: write env file before starting so the agent service picks it up.
	if useSystemd {
		envContent := fmt.Sprintf("STROPPY_SERVER_ADDR=%s\nSTROPPY_MACHINE_ID=%s\nSTROPPY_AGENT_TOKEN=%s\n",
			serverAddr, machineID, agentToken)
		if err := d.copyFileToContainer(ctx, resp.ID, "/etc/stroppy-agent.env", envContent); err != nil {
			return DeployResult{}, fmt.Errorf("agent: write env file %s: %w", name, err)
		}
	}

	if err := d.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return DeployResult{}, fmt.Errorf("agent: docker start %s: %w", name, err)
	}

	return DeployResult{
		ContainerID:   resp.ID,
		ContainerName: name,
	}, nil
}

// Stop removes the agent container (force).
func (d *DockerDeployer) Stop(ctx context.Context, containerID string) error {
	return d.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
}

// StopGraceful stops the container with a timeout before removing it.
func (d *DockerDeployer) StopGraceful(ctx context.Context, containerID string, timeoutSec int) error {
	// Best-effort stop: container may already be stopped or removed; ignore error.
	_ = d.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeoutSec})
	return d.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{})
}

func (d *DockerDeployer) pullIfMissing(ctx context.Context, img string) error {
	_, err := d.cli.ImageInspect(ctx, img)
	if err == nil {
		return nil
	}
	reader, err := d.cli.ImagePull(ctx, img, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("agent: docker pull %s: %w", img, err)
	}
	defer reader.Close()
	_, _ = io.Copy(os.Stderr, reader)
	return nil
}

// copyFileToContainer creates a single file inside a stopped container via tar archive.
func (d *DockerDeployer) copyFileToContainer(ctx context.Context, containerID, filePath, content string) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name: filePath,
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		return err
	}
	tw.Close()
	return d.cli.CopyToContainer(ctx, containerID, "/", &buf, container.CopyToContainerOptions{})
}

// Close releases the Docker client.
func (d *DockerDeployer) Close() error {
	return d.cli.Close()
}
