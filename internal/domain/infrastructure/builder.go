package infrastructure

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

const (
	DefaultDockerImage = "stroppy-agent:latest"

	defaultCPUCores = 2
	defaultMemoryMB = 4096
	defaultDiskGB   = 40
)

type MachineSizing struct {
	CPUCores uint32
	MemoryMB uint64
	DiskGB   uint64
}

type DockerOptions struct {
	Image         string
	FallbackImage string
	NetworkName   string
	DNS           []string
	Env           map[string]string
	Privileged    bool
	CgroupnsMode  string
	RestartPolicy deployment.Docker_RestartPolicy
	PublishPorts  bool
	HostIP        string
}

type YandexOptions struct {
	BootDiskType        string
	Zone                string
	InternalIP          string
	PublicIP            bool
	UserData            string
	NetworkAcceleration string
	SecondaryDisks      []*deployment.Yandex_Disk
}

type BuildOptions struct {
	Settings *deployment.ProviderSettings
	Labels   map[string]string
	Tags     *common.Tags

	DefaultSizing MachineSizing
	MachineSizing map[string]MachineSizing

	Docker DockerOptions
	Yandex YandexOptions
}

func BuildPlan(spec *topologypb.TopologySpec, provider deployment.Provider, options BuildOptions) (*deployment.InfrastructurePlan, error) {
	idx, err := topology.NewIndex(spec)
	if err != nil {
		return nil, err
	}
	if provider == deployment.Provider_PROVIDER_UNSPECIFIED {
		return nil, errors.New("provider is required")
	}

	plan := &deployment.InfrastructurePlan{
		Provider: provider,
		Settings: providerSettings(provider, options.Settings),
		Machines: make([]*deployment.MachinePlan, 0, len(spec.GetNodes())),
		Labels:   mergeLabels(spec.GetLabels(), options.Labels, map[string]string{"stage": "infrastructure_plan"}),
		Tags:     tagsOrDefault(options.Tags, "infrastructure", providerLabel(provider)),
	}

	for _, node := range idx.Spec().GetNodes() {
		machine, err := buildMachine(node, provider, options)
		if err != nil {
			return nil, err
		}
		plan.Machines = append(plan.Machines, machine)
	}

	if err := plan.Validate(); err != nil {
		return nil, err
	}
	return plan, nil
}

func buildMachine(node *topologypb.Node, provider deployment.Provider, options BuildOptions) (*deployment.MachinePlan, error) {
	sizing := sizingFor(node.GetId(), options)
	labels := mergeLabels(node.GetLabels(), map[string]string{
		"node_id": node.GetId(),
	})

	machine := &deployment.MachinePlan{
		NodeId:        node.GetId(),
		QuotaRequests: quotaRequests(provider, sizing, 0),
		Labels:        labels,
		Tags:          node.GetTags(),
	}

	switch provider {
	case deployment.Provider_PROVIDER_DOCKER:
		container := dockerContainer(node, sizing, options.Docker)
		machine.ProviderParams = &deployment.MachinePlan_Docker{Docker: container}
		machine.QuotaRequests = quotaRequests(provider, sizing, len(container.GetPorts()))
	case deployment.Provider_PROVIDER_YANDEX:
		machine.ProviderParams = &deployment.MachinePlan_Yandex{Yandex: yandexVM(sizing, options.Yandex)}
		machine.QuotaRequests = quotaRequests(provider, sizing, 0)
	default:
		return nil, fmt.Errorf("provider %s is not supported", provider)
	}

	return machine, nil
}

func dockerContainer(node *topologypb.Node, sizing MachineSizing, options DockerOptions) *deployment.Docker_Container {
	image := options.Image
	if image == "" {
		image = DefaultDockerImage
	}
	restartPolicy := options.RestartPolicy
	if restartPolicy == deployment.Docker_RESTART_POLICY_UNSPECIFIED {
		restartPolicy = deployment.Docker_RESTART_POLICY_UNLESS_STOPPED
	}
	cgroupnsMode := options.CgroupnsMode
	if cgroupnsMode == "" {
		cgroupnsMode = "host"
	}

	container := &deployment.Docker_Container{
		Image:         image,
		FallbackImage: options.FallbackImage,
		Hostname:      node.GetId(),
		Cmd:           []string{"/sbin/init"},
		Env:           copyStringMap(options.Env),
		Labels: mergeLabels(node.GetLabels(), map[string]string{
			"stroppy.cloud/node_id": node.GetId(),
		}),
		Privileged:    true,
		CgroupnsMode:  cgroupnsMode,
		Tmpfs:         map[string]string{"/run": "rw,nosuid,nodev,mode=755", "/tmp": "rw,nosuid,nodev"},
		RestartPolicy: restartPolicy,
		Resources: &deployment.Docker_Resources{
			CpuCores: float64(sizing.CPUCores),
			MemoryMb: sizing.MemoryMB,
		},
	}
	if !options.Privileged {
		container.Privileged = true
	}
	if options.PublishPorts {
		container.Ports = defaultNodePorts(options.HostIP)
	}

	return container
}

func yandexVM(sizing MachineSizing, options YandexOptions) *deployment.Yandex_Vm {
	bootDiskType := options.BootDiskType
	if bootDiskType == "" {
		bootDiskType = "network-ssd"
	}
	internalIP := options.InternalIP
	if internalIP == "" {
		internalIP = "auto"
	}
	networkAcceleration := options.NetworkAcceleration
	if networkAcceleration == "" {
		networkAcceleration = "standard"
	}

	memoryGB := uint64(math.Ceil(float64(sizing.MemoryMB) / 1024))
	if memoryGB == 0 {
		memoryGB = 1
	}

	return &deployment.Yandex_Vm{
		Cores:               sizing.CPUCores,
		MemoryGb:            memoryGB,
		BootDiskGb:          sizing.DiskGB,
		BootDiskType:        bootDiskType,
		Zone:                options.Zone,
		InternalIp:          internalIP,
		PublicIp:            options.PublicIP,
		UserData:            options.UserData,
		NetworkAcceleration: networkAcceleration,
		SecondaryDisks:      options.SecondaryDisks,
	}
}

func providerSettings(provider deployment.Provider, settings *deployment.ProviderSettings) *deployment.ProviderSettings {
	if settings != nil {
		return settings
	}
	if provider == deployment.Provider_PROVIDER_DOCKER {
		return &deployment.ProviderSettings{
			Settings: &deployment.ProviderSettings_Docker{Docker: &deployment.Docker_Settings{}},
		}
	}
	return nil
}

func sizingFor(nodeID string, options BuildOptions) MachineSizing {
	if sizing, ok := options.MachineSizing[nodeID]; ok {
		return normalizeSizing(sizing)
	}
	return normalizeSizing(options.DefaultSizing)
}

func normalizeSizing(sizing MachineSizing) MachineSizing {
	if sizing.CPUCores == 0 {
		sizing.CPUCores = defaultCPUCores
	}
	if sizing.MemoryMB == 0 {
		sizing.MemoryMB = defaultMemoryMB
	}
	if sizing.DiskGB == 0 {
		sizing.DiskGB = defaultDiskGB
	}
	return sizing
}

func quotaRequests(provider deployment.Provider, sizing MachineSizing, publishedPorts int) []*deployment.Quota_Request {
	switch provider {
	case deployment.Provider_PROVIDER_DOCKER:
		requests := []*deployment.Quota_Request{
			quotaRequest(provider, "host.containers.count", "count", 1),
			quotaRequest(provider, "host.cpuCores", "cores", uint64(sizing.CPUCores)),
			quotaRequest(provider, "host.memory.size", "MiB", sizing.MemoryMB),
			quotaRequest(provider, "host.disk.size", "GiB", sizing.DiskGB),
		}
		if publishedPorts > 0 {
			requests = append(requests, quotaRequest(provider, "host.ports.count", "count", uint64(publishedPorts)))
		}
		return requests
	case deployment.Provider_PROVIDER_YANDEX:
		return []*deployment.Quota_Request{
			quotaRequest(provider, "compute.instances.count", "count", 1),
			quotaRequest(provider, "compute.cores.count", "cores", uint64(sizing.CPUCores)),
			quotaRequest(provider, "compute.memory.size", "GiB", uint64(math.Ceil(float64(sizing.MemoryMB)/1024))),
			quotaRequest(provider, "compute.disks.size", "GiB", sizing.DiskGB),
		}
	default:
		return nil
	}
}

func quotaRequest(provider deployment.Provider, name, units string, request uint64) *deployment.Quota_Request {
	return &deployment.Quota_Request{
		Info: &deployment.Quota_Info{
			Provider: provider,
			Name:     name,
			Units:    units,
		},
		Request: request,
	}
}

func defaultNodePorts(hostIP string) []*deployment.Docker_PortBinding {
	return []*deployment.Docker_PortBinding{
		{ContainerPort: 22, HostIp: hostIP, Protocol: deployment.Docker_PROTOCOL_TCP},
	}
}

func copyStringMap(input map[string]string) map[string]string {
	output := make(map[string]string, len(input)+2)
	for key, value := range input {
		output[key] = value
	}
	return output
}

func mergeLabels(maps ...map[string]string) map[string]string {
	total := 0
	for _, labels := range maps {
		total += len(labels)
	}
	if total == 0 {
		return nil
	}

	merged := make(map[string]string, total)
	for _, labels := range maps {
		for key, value := range labels {
			merged[key] = value
		}
	}
	return merged
}

func tagsOrDefault(input *common.Tags, values ...string) *common.Tags {
	if input != nil {
		return input
	}
	return &common.Tags{Tags: values}
}

func providerLabel(provider deployment.Provider) string {
	if provider == deployment.Provider_PROVIDER_UNSPECIFIED {
		return "unspecified"
	}
	return strings.TrimPrefix(provider.String(), "PROVIDER_")
}
