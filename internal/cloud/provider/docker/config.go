package docker

import "os"

const (
	DefaultDockerNetworkName     = "stroppy-net"
	DefaultEdgeWorkerDockerImage = "stroppy-edge-worker:latest"

	DockerNetworkNameEnvKey     = "DOCKER_NETWORK_NAME"
	EdgeWorkerDockerImageEnvKey = "EDGE_WORKER_DOCKER_IMAGE"

	DockerApiVersion = "docker/v1"

	KindDockerNetwork   = "DockerNetwork"
	KindDockerSubnet    = "DockerSubnet"
	KindDockerContainer = "DockerContainer"
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
	network := os.Getenv(DockerNetworkNameEnvKey)
	if network == "" {
		network = DefaultDockerNetworkName
	}
	return &ProviderConfig{
		EdgeWorkerImage: image,
		NetworkName:     network,
	}
}
