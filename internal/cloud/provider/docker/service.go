package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	dockerClient "github.com/docker/docker/client"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/hatchet-workflow/internal/cloud/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
)

var _ deployment.DeploymentService = (*Service)(nil)

type Service struct {
	client      *dockerClient.Client
	networkName string
}

func NewService(networkName string) (*Service, error) {
	cli, err := dockerClient.NewClientWithOpts(
		dockerClient.FromEnv,
		dockerClient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	return &Service{
		client:      cli,
		networkName: networkName,
	}, nil
}

func (s *Service) CreateDeployment(
	ctx context.Context,
	depl *crossplane.Deployment,
) (*crossplane.Deployment, error) {
	for _, resource := range deployment.GetDeploymentResources(depl) {
		resource.Status = crossplane.Resource_STATUS_CREATING
		resource.CreatedAt = timestamppb.Now()
		resource.UpdatedAt = timestamppb.Now()

		switch resource.GetRef().GetKind() {
		case KindDockerNetwork:
			if err := s.ensureNetwork(ctx, resource); err != nil {
				return nil, fmt.Errorf("failed to create docker network: %w", err)
			}
		case KindDockerSubnet:
			// No-op for Docker subnets — Docker manages networking automatically.
			resource.Status = crossplane.Resource_STATUS_READY
			resource.Ready = true
			resource.Synced = true
			resource.ExternalId = "docker-managed"
		case KindDockerContainer:
			if err := s.createContainer(ctx, resource); err != nil {
				return nil, fmt.Errorf("failed to create docker container: %w", err)
			}
		}
	}
	return depl, nil
}

func (s *Service) ProcessDeploymentStatus(
	ctx context.Context,
	depl *crossplane.Deployment,
) (*crossplane.Deployment, error) {
	for _, resource := range deployment.GetDeploymentResources(depl) {
		switch resource.GetRef().GetKind() {
		case KindDockerNetwork:
			if err := s.checkNetworkStatus(ctx, resource); err != nil {
				return nil, err
			}
		case KindDockerSubnet:
			// Always ready.
			resource.Ready = true
			resource.Synced = true
		case KindDockerContainer:
			if err := s.checkContainerStatus(ctx, resource); err != nil {
				return nil, err
			}
		}
	}
	return depl, nil
}

func (s *Service) DestroyDeployment(
	ctx context.Context,
	depl *crossplane.Deployment,
) error {
	for _, resource := range deployment.GetDeploymentResources(depl) {
		resource.Status = crossplane.Resource_STATUS_DESTROYING

		switch resource.GetRef().GetKind() {
		case KindDockerContainer:
			if err := s.removeContainer(ctx, resource); err != nil {
				return fmt.Errorf("failed to remove docker container %s: %w",
					resource.GetRef().GetRef().GetName(), err)
			}
			resource.Status = crossplane.Resource_STATUS_DESTROYED
		case KindDockerNetwork:
			// Don't remove the shared network — it may be used by infra services.
			resource.Status = crossplane.Resource_STATUS_DESTROYED
		case KindDockerSubnet:
			resource.Status = crossplane.Resource_STATUS_DESTROYED
		}
	}
	return nil
}

func (s *Service) ensureNetwork(ctx context.Context, resource *crossplane.Resource) error {
	// Check if network already exists.
	_, err := s.client.NetworkInspect(ctx, s.networkName, network.InspectOptions{})
	if err == nil {
		resource.Status = crossplane.Resource_STATUS_READY
		resource.Ready = true
		resource.Synced = true
		resource.ExternalId = s.networkName
		resource.UpdatedAt = timestamppb.Now()
		return nil
	}

	// Create the network.
	resp, err := s.client.NetworkCreate(ctx, s.networkName, network.CreateOptions{
		Driver: "bridge",
	})
	if err != nil {
		return fmt.Errorf("failed to create docker network %s: %w", s.networkName, err)
	}
	resource.ExternalId = resp.ID
	resource.UpdatedAt = timestamppb.Now()
	return nil
}

func (s *Service) createContainer(ctx context.Context, resource *crossplane.Resource) error {
	var config DockerContainerConfig
	if err := json.Unmarshal([]byte(resource.GetResourceYaml()), &config); err != nil {
		return fmt.Errorf("failed to unmarshal container config: %w", err)
	}

	envList := make([]string, 0, len(config.Env))
	for k, v := range config.Env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}

	resp, err := s.client.ContainerCreate(
		ctx,
		&container.Config{
			Image: config.ImageName,
			Env:   envList,
		},
		&container.HostConfig{
			RestartPolicy: container.RestartPolicy{
				Name:              container.RestartPolicyOnFailure,
				MaximumRetryCount: 3,
			},
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				config.NetworkName: {},
			},
		},
		nil,
		config.ContainerName,
	)
	if err != nil {
		return fmt.Errorf("failed to create container %s: %w", config.ContainerName, err)
	}

	if err := s.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container %s: %w", config.ContainerName, err)
	}

	resource.ExternalId = resp.ID
	resource.UpdatedAt = timestamppb.Now()
	return nil
}

func (s *Service) checkNetworkStatus(ctx context.Context, resource *crossplane.Resource) error {
	networkID := resource.GetExternalId()
	if networkID == "" {
		networkID = s.networkName
	}
	_, err := s.client.NetworkInspect(ctx, networkID, network.InspectOptions{})
	if err != nil {
		return nil // Network not ready yet.
	}
	resource.Ready = true
	resource.Synced = true
	resource.UpdatedAt = timestamppb.Now()
	return nil
}

func (s *Service) checkContainerStatus(ctx context.Context, resource *crossplane.Resource) error {
	containerID := resource.GetExternalId()
	if containerID == "" {
		return nil // Not created yet.
	}
	info, err := s.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil // Container not found yet.
	}
	if info.State != nil && info.State.Running {
		resource.Ready = true
		resource.Synced = true
		resource.UpdatedAt = timestamppb.Now()
	}
	return nil
}

func (s *Service) removeContainer(ctx context.Context, resource *crossplane.Resource) error {
	containerID := resource.GetExternalId()
	if containerID == "" {
		return nil // Nothing to remove.
	}
	timeout := 10 * time.Second
	_ = s.client.ContainerStop(ctx, containerID, container.StopOptions{
		Timeout: func() *int { t := int(timeout.Seconds()); return &t }(),
	})
	return s.client.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: true,
	})
}
