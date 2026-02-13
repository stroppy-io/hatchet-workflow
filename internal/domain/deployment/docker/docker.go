package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	dockerClient "github.com/docker/docker/client"
	"github.com/samber/lo"
	"github.com/stroppy-io/hatchet-workflow/internal/core/consts"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/settings"
)

const (
	SockMount              consts.ConstValue = "/var/run/docker.sock"
	ConfigMount            consts.ConstValue = "/root/.docker/config.json"
	ConfigMountDir         consts.ConstValue = "/root/.docker"
	bridgeDriver           consts.ConstValue = "bridge"
	dockerConfigFileName   consts.ConstValue = "config.json"
	dockerConfigDirEnvName consts.ConstValue = "DOCKER_CONFIG"
)

type Service struct {
	client   *dockerClient.Client
	settings *settings.Settings
}

func NewService(settings *settings.Settings) (*Service, error) {
	cli, err := dockerClient.NewClientWithOpts(
		dockerClient.FromEnv,
		dockerClient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	return &Service{
		client:   cli,
		settings: settings,
	}, nil
}

func mapEnvToList(ma map[string]string) []string {
	var out []string
	for k, v := range ma {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}

func (s *Service) CreateDeployment(
	ctx context.Context,
	depl *deployment.Deployment_Template,
) (*deployment.Deployment, error) {
	mounts := []mount.Mount{
		{
			Type:     mount.TypeBind,
			Source:   SockMount,
			Target:   SockMount,
			ReadOnly: true,
		},
	}
	if dockerConfigPath, ok := resolveDockerConfigPath(); ok {
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   dockerConfigPath,
			Target:   ConfigMount,
			ReadOnly: true,
		})
	}

	ret := &deployment.Deployment{
		Template: depl,
	}
	for _, vm := range depl.GetVmTemplates() {
		env := mapEnvToList(vm.GetCloudInit().GetEnv())
		if len(mounts) > 1 && !containsEnvKey(env, string(dockerConfigDirEnvName)) {
			env = append(env, fmt.Sprintf("%s=%s", dockerConfigDirEnvName, ConfigMountDir))
		}

		resp, err := s.client.ContainerCreate(
			ctx,
			&container.Config{
				Image:        s.settings.GetDocker().GetEdgeWorkerImage(),
				Env:          env,
				AttachStderr: true,
				AttachStdout: true,
			},
			&container.HostConfig{
				NetworkMode: network.NetworkHost,
				Mounts:      mounts,
				RestartPolicy: container.RestartPolicy{
					Name:              container.RestartPolicyOnFailure,
					MaximumRetryCount: 3,
				},
			},
			nil,
			nil,
			vm.GetIdentifier().GetName(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create container %s: %w", vm.GetIdentifier().GetName(), err)
		}
		err = s.client.ContainerStart(ctx, resp.ID, container.StartOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to start container %s: %w", vm.GetIdentifier().GetName(), err)
		}
		ret.Vms = append(ret.Vms, &deployment.Vm{
			Template: vm,
			AssignedInternalIp: &deployment.Ip{
				Value: resp.ID,
			},
		})
	}
	return ret, nil
}

func (s *Service) DestroyDeployment(
	ctx context.Context,
	depl *deployment.Deployment,
) error {
	for _, vm := range depl.GetVms() {
		err := s.client.ContainerStop(ctx, vm.GetAssignedInternalIp().GetValue(), container.StopOptions{
			Timeout: lo.ToPtr(-1),
		})
		if err != nil {
			return fmt.Errorf("failed to stop container %s: %w", vm.GetAssignedInternalIp().GetValue(), err)
		}
		err = s.client.ContainerRemove(ctx, vm.GetAssignedInternalIp().GetValue(), container.RemoveOptions{})
		if err != nil {
			return fmt.Errorf("failed to remove container %s: %w", vm.GetAssignedInternalIp().GetValue(), err)
		}
	}
	return nil
}

func (s *Service) ensureNetwork(ctx context.Context, networkName string) (network.CreateResponse, error) {
	// Check if network already exists.
	inspect, err := s.client.NetworkInspect(ctx, networkName, network.InspectOptions{})
	if err == nil {
		return network.CreateResponse{
			ID: inspect.ID,
		}, nil
	}
	// Create the network.
	resp, err := s.client.NetworkCreate(ctx, networkName, network.CreateOptions{
		Driver: bridgeDriver,
	})
	if err != nil {
		return network.CreateResponse{}, fmt.Errorf("failed to create docker network %s: %w", networkName, err)
	}
	return resp, nil
}

func (s *Service) removeNetwork(ctx context.Context, networkName string) error {
	return s.client.NetworkRemove(ctx, networkName)
}

func resolveDockerConfigPath() (string, bool) {
	cfgDir := os.Getenv(dockerConfigDirEnvName)
	if cfgDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		cfgDir = filepath.Join(homeDir, ".docker")
	}

	cfgPath := filepath.Join(cfgDir, dockerConfigFileName)
	if _, err := os.Stat(cfgPath); err != nil {
		return "", false
	}
	return cfgPath, true
}

func containsEnvKey(env []string, key string) bool {
	prefix := key + "="
	for _, item := range env {
		if len(item) >= len(prefix) && item[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
