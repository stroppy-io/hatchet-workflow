//go:build integration

package edge

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	dockerClient "github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/provision"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/settings"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/stroppy"
)

func TestContainerRunnerDockerTargetIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cli := dockerDaemonOrSkip(t, ctx)

	runSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	networkName := "edge-it-" + runSuffix
	thirdOctet := int(time.Now().UnixNano() % 200)
	subnet := fmt.Sprintf("172.28.%d.0/24", thirdOctet)
	serverIP := fmt.Sprintf("172.28.%d.10", thirdOctet)
	clientIP := fmt.Sprintf("172.28.%d.11", thirdOctet)
	workerIP := fmt.Sprintf("10.0.%d.2", thirdOctet)

	_, err := cli.NetworkCreate(ctx, networkName, network.CreateOptions{
		Driver: "bridge",
		IPAM: &network.IPAM{
			Config: []network.IPAMConfig{
				{Subnet: subnet},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create integration docker network: %v", err)
	}
	t.Cleanup(func() {
		_ = cli.NetworkRemove(context.Background(), networkName)
	})

	runner, err := NewContainerRunner(networkName)
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}
	t.Cleanup(func() {
		runner.Cleanup(context.Background())
		_ = runner.Close()
	})

	testRunCtx := &stroppy.TestRunContext{
		RunId: "run-" + runSuffix,
		SelectedTarget: &settings.SelectedTarget{
			Target: &settings.SelectedTarget_DockerSettings{
				DockerSettings: &settings.DockerSettings{
					NetworkName:     networkName,
					EdgeWorkerImage: "unused-in-test",
				},
			},
		},
	}

	serverHostPort := uint32(18080)
	containers := []*provision.Container{
		{
			Id:      "server-id",
			Name:    "server",
			Image:   "busybox:1.36.1",
			Command: []string{"sh", "-c"},
			Args:    []string{"mkdir -p /www && echo pong >/www/index.html && httpd -f -p 18080 -h /www"},
			Ports: []*provision.ContainerPort{
				{
					ContainerPort: 18080,
					HostPort:      &serverHostPort,
				},
			},
			Metadata: map[string]string{
				containerMetadataDockerIPKey: serverIP,
			},
		},
		{
			Id:      "client-id",
			Name:    "client",
			Image:   "busybox:1.36.1",
			Command: []string{"sh", "-c"},
			Args: []string{
				fmt.Sprintf("for i in 1 2 3 4 5; do wget -qO- http://%s:18080 && exit 0; sleep 1; done; exit 1", serverIP),
			},
			Metadata: map[string]string{
				containerMetadataDockerIPKey: clientIP,
			},
		},
	}

	if err := runner.DeployContainersForTarget(ctx, testRunCtx, workerIP, containers); err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	serverOpts := runContainerOptions{
		dockerTarget:   true,
		runID:          testRunCtx.GetRunId(),
		workerInternal: workerIP,
	}
	serverName := containerRuntimeName(containers[0], serverOpts)
	serverID := getTrackedContainerID(t, runner, serverName)

	serverInspect, err := cli.ContainerInspect(ctx, serverID)
	if err != nil {
		t.Fatalf("failed to inspect server container: %v", err)
	}
	networkInfo, ok := serverInspect.NetworkSettings.Networks[networkName]
	if !ok {
		t.Fatalf("server not attached to expected network %q", networkName)
	}
	if networkInfo.IPAddress != serverIP {
		t.Fatalf("unexpected server IP: got=%q want=%q", networkInfo.IPAddress, serverIP)
	}
	if got := serverInspect.Config.Labels[containerLabelRunIDKey]; got != testRunCtx.GetRunId() {
		t.Fatalf("unexpected run_id label: got=%q", got)
	}
	if got := serverInspect.Config.Labels[containerLabelWorkerIPKey]; got != workerIP {
		t.Fatalf("unexpected worker_ip label: got=%q", got)
	}
	if got := serverInspect.Config.Labels[containerLabelManagedByKey]; got != containerLabelManagedByVal {
		t.Fatalf("unexpected managed_by label: got=%q", got)
	}
	if len(serverInspect.HostConfig.PortBindings) != 0 {
		t.Fatalf("expected no host port bindings in docker target mode, got=%v", serverInspect.HostConfig.PortBindings)
	}

	clientName := containerRuntimeName(containers[1], serverOpts)
	clientID := getTrackedContainerID(t, runner, clientName)

	clientLogs, waitErr := waitForContainerExitAndLogs(ctx, cli, clientID, 45*time.Second)
	if waitErr != nil {
		t.Fatalf("client container failed: %v; logs=%s", waitErr, clientLogs)
	}
	if !strings.Contains(clientLogs, "pong") {
		t.Fatalf("expected client logs to contain pong, logs=%s", clientLogs)
	}

	runner.Cleanup(ctx)

	assertContainerRemoved(t, ctx, cli, serverID)
	assertContainerRemoved(t, ctx, cli, clientID)
}

func dockerDaemonOrSkip(t *testing.T, ctx context.Context) *dockerClient.Client {
	t.Helper()

	cli, err := dockerClient.NewClientWithOpts(
		dockerClient.FromEnv,
		dockerClient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		t.Skipf("docker client not available: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })

	if _, err := cli.Ping(ctx); err != nil {
		t.Skipf("docker daemon not available: %v", err)
	}
	return cli
}

func getTrackedContainerID(t *testing.T, runner *ContainerRunner, name string) string {
	t.Helper()
	runner.mu.Lock()
	defer runner.mu.Unlock()

	id, ok := runner.containers[name]
	if !ok || id == "" {
		t.Fatalf("container %q is not tracked", name)
	}
	return id
}

func waitForContainerExitAndLogs(
	ctx context.Context,
	cli *dockerClient.Client,
	containerID string,
	timeout time.Duration,
) (string, error) {
	deadline := time.Now().Add(timeout)

	for {
		inspect, err := cli.ContainerInspect(ctx, containerID)
		if err != nil {
			return "", err
		}
		if inspect.State != nil && inspect.State.Status == "exited" {
			logs, logErr := readContainerLogs(ctx, cli, containerID)
			if logErr != nil {
				return "", logErr
			}
			if inspect.State.ExitCode != 0 {
				return logs, fmt.Errorf("container exited with code %d", inspect.State.ExitCode)
			}
			return logs, nil
		}
		if time.Now().After(deadline) {
			logs, _ := readContainerLogs(ctx, cli, containerID)
			return logs, fmt.Errorf("timeout waiting for container exit")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func readContainerLogs(ctx context.Context, cli *dockerClient.Client, containerID string) (string, error) {
	rc, err := cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return "", err
	}
	defer rc.Close()

	raw, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	trimmed, err := trimDockerLogStream(raw)
	if err != nil {
		return "", err
	}
	return string(trimmed), nil
}

func trimDockerLogStream(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return raw, nil
	}

	out := make([]byte, 0, len(raw))
	for i := 0; i < len(raw); {
		if len(raw)-i < 8 {
			return nil, fmt.Errorf("invalid docker log frame")
		}
		size := int(raw[i+4])<<24 | int(raw[i+5])<<16 | int(raw[i+6])<<8 | int(raw[i+7])
		i += 8
		if size < 0 || len(raw)-i < size {
			return nil, fmt.Errorf("invalid docker log payload size")
		}
		out = append(out, raw[i:i+size]...)
		i += size
	}
	return out, nil
}

func assertContainerRemoved(t *testing.T, ctx context.Context, cli *dockerClient.Client, containerID string) {
	t.Helper()
	_, err := cli.ContainerInspect(ctx, containerID)
	if err == nil {
		t.Fatalf("container %s still exists after cleanup", containerID)
	}
	if !errdefs.IsNotFound(err) {
		t.Fatalf("unexpected inspect error for removed container %s: %v", containerID, err)
	}
}
