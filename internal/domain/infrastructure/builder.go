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

	// labelManaged marks a node/spec as a provider-managed database. Such a
	// node gets no self-hosted VM; the provider runs the database. Mirrors
	// ydbmanaged.LabelManaged (kept as a local string to avoid coupling the
	// infrastructure builder to a specific database engine package).
	labelManaged = "managed"
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
		// Provider-managed databases (e.g. Yandex Managed YDB) carry no
		// self-hosted VM: the cloud provider runs the database. Skip VM/
		// container allocation for such nodes — the managed resource is
		// requested via the provider-specific managed input instead (carried
		// on the plan labels, see managedInputLabel below).
		if isManagedNode(node) {
			continue
		}
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

// isManagedNode reports whether the topology node represents a provider-managed
// database (e.g. Yandex Managed YDB) that must not allocate a self-hosted VM.
func isManagedNode(node *topologypb.Node) bool {
	return node.GetLabels()[labelManaged] == "true"
}

func buildMachine(node *topologypb.Node, provider deployment.Provider, options BuildOptions) (*deployment.MachinePlan, error) {
	sizing := sizingFor(node.GetId(), options)
	labels := mergeLabels(node.GetLabels(), map[string]string{
		"node_id": node.GetId(),
	})

	machine := &deployment.MachinePlan{
		NodeId: node.GetId(),
		Labels: labels,
		Tags:   node.GetTags(),
	}

	switch provider {
	case deployment.Provider_PROVIDER_DOCKER:
		container := dockerContainer(node, sizing, options.Docker)
		machine.ProviderParams = &deployment.MachinePlan_Docker{Docker: container}
		machine.QuotaRequests = dockerQuotaRequests(sizing, len(container.GetPorts()))
	case deployment.Provider_PROVIDER_YANDEX:
		vm := yandexVM(sizing, options.Yandex)
		machine.ProviderParams = &deployment.MachinePlan_Yandex{Yandex: vm}
		machine.QuotaRequests = yandexQuotaRequests(vm)
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

func dockerQuotaRequests(sizing MachineSizing, publishedPorts int) []*deployment.Quota_Request {
	requests := []*deployment.Quota_Request{
		quotaRequest(deployment.Provider_PROVIDER_DOCKER, "host.containers.count", "count", 1),
		quotaRequest(deployment.Provider_PROVIDER_DOCKER, "host.cpuCores", "cores", uint64(sizing.CPUCores)),
		quotaRequest(deployment.Provider_PROVIDER_DOCKER, "host.memory.size", "MiB", sizing.MemoryMB),
		quotaRequest(deployment.Provider_PROVIDER_DOCKER, "host.disk.size", "GiB", sizing.DiskGB),
	}
	if publishedPorts > 0 {
		requests = append(requests, quotaRequest(deployment.Provider_PROVIDER_DOCKER, "host.ports.count", "count", uint64(publishedPorts)))
	}
	return requests
}

func yandexQuotaRequests(vm *deployment.Yandex_Vm) []*deployment.Quota_Request {
	if vm == nil {
		return nil
	}
	requests := []*deployment.Quota_Request{
		quotaRequest(deployment.Provider_PROVIDER_YANDEX, "compute.instances.count", "count", 1),
		quotaRequest(deployment.Provider_PROVIDER_YANDEX, "compute.instanceCores.count", "cores", uint64(vm.GetCores())),
		quotaRequest(deployment.Provider_PROVIDER_YANDEX, "compute.instanceMemory.size", "GiB", vm.GetMemoryGb()),
		quotaRequest(deployment.Provider_PROVIDER_YANDEX, yandexDiskQuotaName(vm.GetBootDiskType()), "GiB", vm.GetBootDiskGb()),
	}
	for _, disk := range vm.GetSecondaryDisks() {
		if disk.GetSizeGb() == 0 {
			continue
		}
		requests = append(requests, quotaRequest(deployment.Provider_PROVIDER_YANDEX, yandexDiskQuotaName(disk.GetType()), "GiB", uint64(disk.GetSizeGb())))
	}
	return requests
}

func yandexDiskQuotaName(diskType string) string {
	if strings.Contains(strings.ToLower(diskType), "hdd") {
		return "compute.hddDisks.size"
	}
	return "compute.ssdDisks.size"
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
