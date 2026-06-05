package infrastructure

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"google.golang.org/protobuf/proto"

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
	// MachineOverrides are compatible user edits to provider-specific machine
	// intent. They are matched by node_id and provider, then overlaid on the
	// generated machine plan after topology-derived defaults are rebuilt.
	MachineOverrides []*deployment.MachinePlan

	Docker DockerOptions
	Yandex YandexOptions
}

func BuildPlan(spec *topologypb.TopologySpec, provider deployment.Provider, options BuildOptions) (*deployment.InfrastructurePlan, error) {
	options = withMachineSizingOverrides(provider, options)
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

	applyMachineOverrides(plan, provider, options.MachineOverrides)

	if err := plan.Validate(); err != nil {
		return nil, err
	}
	return plan, nil
}

// MachineSizingOverrides extracts the resource sizing encoded in a provider
// plan. It is used by wizard/start paths to carry user machine edits back into
// BuildOptions before a fresh topology-derived plan is generated.
func MachineSizingOverrides(plan *deployment.InfrastructurePlan) map[string]MachineSizing {
	if plan == nil {
		return nil
	}
	return MachineSizingOverridesForProvider(plan.GetProvider(), plan.GetMachines())
}

// BuildOptionsFromPlanOverrides turns an edited InfrastructurePlan back into the
// BuildOptions fields needed to rebuild a fresh compatible plan.
func BuildOptionsFromPlanOverrides(plan *deployment.InfrastructurePlan) BuildOptions {
	if plan == nil {
		return BuildOptions{}
	}
	return BuildOptionsFromMachineOverrides(plan.GetProvider(), plan.GetMachines())
}

// BuildOptionsFromMachineOverrides turns per-node MachinePlan overrides into
// provider-scoped BuildOptions.
func BuildOptionsFromMachineOverrides(provider deployment.Provider, machines []*deployment.MachinePlan) BuildOptions {
	return BuildOptions{
		MachineSizing:    MachineSizingOverridesForProvider(provider, machines),
		MachineOverrides: machines,
	}
}

// MachineSizingOverridesForProvider extracts sizing from a list of machine
// overrides, ignoring machines that do not match the active provider.
func MachineSizingOverridesForProvider(provider deployment.Provider, machines []*deployment.MachinePlan) map[string]MachineSizing {
	out := make(map[string]MachineSizing)
	for _, machine := range machines {
		if machine.GetNodeId() == "" {
			continue
		}
		sizing, ok := machineSizingForProvider(provider, machine)
		if !ok {
			continue
		}
		out[machine.GetNodeId()] = sizing
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func withMachineSizingOverrides(provider deployment.Provider, options BuildOptions) BuildOptions {
	fromOverrides := MachineSizingOverridesForProvider(provider, options.MachineOverrides)
	if len(fromOverrides) == 0 {
		return options
	}
	merged := make(map[string]MachineSizing, len(fromOverrides)+len(options.MachineSizing))
	for nodeID, sizing := range fromOverrides {
		merged[nodeID] = sizing
	}
	for nodeID, sizing := range options.MachineSizing {
		merged[nodeID] = sizing
	}
	options.MachineSizing = merged
	return options
}

func applyMachineOverrides(plan *deployment.InfrastructurePlan, provider deployment.Provider, overrides []*deployment.MachinePlan) {
	if plan == nil || len(overrides) == 0 {
		return
	}
	byNode := make(map[string]*deployment.MachinePlan, len(overrides))
	for _, override := range overrides {
		if override.GetNodeId() == "" {
			continue
		}
		byNode[override.GetNodeId()] = override
	}
	for _, machine := range plan.GetMachines() {
		override := byNode[machine.GetNodeId()]
		if override == nil {
			continue
		}
		switch provider {
		case deployment.Provider_PROVIDER_DOCKER:
			applyDockerOverride(machine, override)
		case deployment.Provider_PROVIDER_YANDEX:
			applyYandexOverride(machine, override)
		}
	}
}

func applyDockerOverride(machine, override *deployment.MachinePlan) {
	src := override.GetDocker()
	if src == nil {
		return
	}
	// Start from the generated container so runtime-critical fields the user
	// never edits (privileged, init cmd, tmpfs, cgroupns, restart policy,
	// hostname) survive; overlay only the user-editable fields from the
	// override. A partial override must not strip the agent container's
	// requirements.
	base := machine.GetDocker()
	if base == nil {
		base = &deployment.Docker_Container{}
	}
	merged := proto.Clone(base).(*deployment.Docker_Container)
	mergeDockerContainer(merged, proto.Clone(src).(*deployment.Docker_Container))
	machine.ProviderParams = &deployment.MachinePlan_Docker{Docker: merged}

	diskGB := dockerDiskGB(override)
	if diskGB == 0 {
		diskGB = dockerDiskGB(machine)
	}
	sizing, _ := machineSizingForDocker(machine.GetNodeId(), merged, diskGB)
	machine.QuotaRequests = dockerQuotaRequests(sizing, len(merged.GetPorts()))
}

// mergeDockerContainer overlays user-editable fields from src onto base,
// leaving base's runtime-critical fields intact when src leaves a field unset.
func mergeDockerContainer(base, src *deployment.Docker_Container) {
	if img := src.GetImage(); img != "" {
		base.Image = img
	}
	if fb := src.GetFallbackImage(); fb != "" {
		base.FallbackImage = fb
	}
	if res := src.GetResources(); res != nil {
		if base.Resources == nil {
			base.Resources = &deployment.Docker_Resources{}
		}
		if cpu := res.GetCpuCores(); cpu > 0 {
			base.Resources.CpuCores = cpu
		}
		if mem := res.GetMemoryMb(); mem > 0 {
			base.Resources.MemoryMb = mem
		}
	}
	if ports := src.GetPorts(); len(ports) > 0 {
		base.Ports = ports
	}
	for key, value := range src.GetEnv() {
		if base.Env == nil {
			base.Env = make(map[string]string, len(src.GetEnv()))
		}
		base.Env[key] = value
	}
	for key, value := range src.GetLabels() {
		if base.Labels == nil {
			base.Labels = make(map[string]string, len(src.GetLabels()))
		}
		base.Labels[key] = value
	}
}

func applyYandexOverride(machine, override *deployment.MachinePlan) {
	src := override.GetYandex()
	if src == nil {
		return
	}
	base := machine.GetYandex()
	if base == nil {
		base = &deployment.Yandex_Vm{}
	}
	merged := proto.Clone(base).(*deployment.Yandex_Vm)
	mergeYandexVM(merged, proto.Clone(src).(*deployment.Yandex_Vm))
	machine.ProviderParams = &deployment.MachinePlan_Yandex{Yandex: merged}
	machine.QuotaRequests = yandexQuotaRequests(merged)
}

// mergeYandexVM overlays user-editable VM sizing/storage fields from src onto
// base. Placement/networking fields are provider-owned: zone and public IP come
// from tenant/provider settings, internal IP is auto, and acceleration is
// derived from provider settings at render time.
func mergeYandexVM(base, src *deployment.Yandex_Vm) {
	if v := src.GetCores(); v != 0 {
		base.Cores = v
	}
	if v := src.GetMemoryGb(); v != 0 {
		base.MemoryGb = v
	}
	if v := src.GetBootDiskGb(); v != 0 {
		base.BootDiskGb = v
	}
	if v := src.GetBootDiskType(); v != "" {
		base.BootDiskType = v
	}
	if v := src.GetUserData(); v != "" {
		base.UserData = v
	}
	if disks := src.GetSecondaryDisks(); len(disks) > 0 {
		base.SecondaryDisks = disks
	}
}

func machineSizingForProvider(provider deployment.Provider, machine *deployment.MachinePlan) (MachineSizing, bool) {
	if machine == nil {
		return MachineSizing{}, false
	}
	switch provider {
	case deployment.Provider_PROVIDER_DOCKER:
		return machineSizingForDocker(machine.GetNodeId(), machine.GetDocker(), dockerDiskGB(machine))
	case deployment.Provider_PROVIDER_YANDEX:
		return machineSizingForYandex(machine.GetNodeId(), machine.GetYandex())
	default:
		return MachineSizing{}, false
	}
}

func machineSizingForDocker(nodeID string, container *deployment.Docker_Container, diskGB uint64) (MachineSizing, bool) {
	if nodeID == "" || container == nil {
		return MachineSizing{}, false
	}
	var sizing MachineSizing
	if resources := container.GetResources(); resources != nil {
		sizing.CPUCores = cpuCoresFromFloat(resources.GetCpuCores())
		sizing.MemoryMB = resources.GetMemoryMb()
	}
	sizing.DiskGB = diskGB
	return sizing, sizing.CPUCores != 0 || sizing.MemoryMB != 0 || sizing.DiskGB != 0
}

func machineSizingForYandex(nodeID string, vm *deployment.Yandex_Vm) (MachineSizing, bool) {
	if nodeID == "" || vm == nil {
		return MachineSizing{}, false
	}
	sizing := MachineSizing{
		CPUCores: vm.GetCores(),
		MemoryMB: vm.GetMemoryGb() * 1024,
		DiskGB:   vm.GetBootDiskGb(),
	}
	return sizing, sizing.CPUCores != 0 || sizing.MemoryMB != 0 || sizing.DiskGB != 0
}

func cpuCoresFromFloat(value float64) uint32 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if value > float64(math.MaxUint32) {
		return math.MaxUint32
	}
	return uint32(math.Ceil(value))
}

func dockerDiskGB(machine *deployment.MachinePlan) uint64 {
	for _, req := range machine.GetQuotaRequests() {
		info := req.GetInfo()
		if info.GetProvider() == deployment.Provider_PROVIDER_DOCKER && info.GetName() == "host.disk.size" {
			return req.GetRequest()
		}
	}
	return 0
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
		Privileged:   true,
		CgroupnsMode: cgroupnsMode,
		// The agent image declares VOLUME /sys/fs/cgroup; without an explicit
		// bind, Docker shadows it with an empty anonymous volume and systemd
		// (PID 1, /sbin/init) fails to boot -> container exits 255 immediately.
		// Bind the host cgroupfs so systemd comes up (works on cgroup v1 and v2).
		Binds:         []string{"/sys/fs/cgroup:/sys/fs/cgroup:rw"},
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
