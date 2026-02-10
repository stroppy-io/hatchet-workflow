package docker

import (
	"os"

	"github.com/stroppy-io/hatchet-workflow/internal/core/consts"
)

const (
	DefaultDockerNetworkName     consts.DefaultEnvValue = "stroppy-net"
	DefaultEdgeWorkerDockerImage consts.DefaultEnvValue = "stroppy-edge-worker:latest"

	NetworkNameEnvKey           consts.EnvKey = "DOCKER_NETWORK_NAME"
	EdgeWorkerDockerImageEnvKey consts.EnvKey = "EDGE_WORKER_DOCKER_IMAGE"

	ApiVersion          consts.ConstValue = "docker/v1"
	KindDockerNetwork   consts.ConstValue = "DockerNetwork"
	KindDockerSubnet    consts.ConstValue = "DockerSubnet"
	KindDockerContainer consts.ConstValue = "DockerContainer"
)

type ProviderConfig struct {
	EdgeWorkerImage string
	NetworkName     string
}

func ProviderConfigFromEnv() *ProviderConfig {
	image := os.Getenv(EdgeWorkerDockerImageEnvKey)
	if image == "" {
		image = DefaultEdgeWorkerDockerImage
	}
	network := os.Getenv(NetworkNameEnvKey)
	if network == "" {
		network = DefaultDockerNetworkName
	}
	return &ProviderConfig{
		EdgeWorkerImage: image,
		NetworkName:     network,
	}
}
