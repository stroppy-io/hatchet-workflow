package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	dockerClient "github.com/docker/docker/client"
	"github.com/stroppy-io/hatchet-workflow/internal/core/consts"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/settings"
)

const (
	SockMount    consts.ConstValue = "/var/run/docker.sock"
	bridgeDriver consts.ConstValue = "bridge"
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
	netResp, err := s.ensureNetwork(ctx, s.settings.GetDocker().GetNetworkName())
	if err != nil {
		return nil, err
	}
	ret := &deployment.Deployment{
		Template: depl,
		Target:   deployment.Target_TARGET_DOCKER,
	}
	for _, vm := range depl.GetVmTemplates() {
		resp, err := s.client.ContainerCreate(
			ctx,
			&container.Config{
				Image:        s.settings.GetDocker().GetEdgeWorkerImage(),
				Env:          mapEnvToList(vm.GetCloudInit().GetEnv()),
				AttachStderr: true,
				AttachStdout: true,
			},
			&container.HostConfig{
				Mounts: []mount.Mount{
					{
						Type:     mount.TypeBind,
						Source:   SockMount,
						Target:   SockMount,
						ReadOnly: true, // Set to true if you only need read access
					},
				},
				RestartPolicy: container.RestartPolicy{
					Name:              container.RestartPolicyOnFailure,
					MaximumRetryCount: 3,
				},
			},
			&network.NetworkingConfig{
				EndpointsConfig: map[string]*network.EndpointSettings{
					s.settings.GetDocker().GetNetworkName(): {
						NetworkID: netResp.ID,
					},
				},
			},
			nil,
			vm.GetIdentifier().GetName(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create container %s: %w", vm.GetIdentifier().GetName(), err)
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
		err := s.client.ContainerRemove(ctx, vm.GetAssignedInternalIp().GetValue(), container.RemoveOptions{})
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
