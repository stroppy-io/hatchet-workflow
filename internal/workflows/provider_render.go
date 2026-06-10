package workflows

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"

	schemapb "github.com/stroppy-io/schemapb/schemapb"
	yandextf "github.com/stroppy-io/stroppy-cloud/deployments/terraform/yandex"
	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	networkinfra "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/networks"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	defaultYandexPlatformID          = "standard-v2"
	defaultYandexBootDiskType        = "network-ssd"
	defaultYandexNetworkAcceleration = "standard"
	reservedNetworkCIDRLabel         = "reserved_network_cidr"
)

func renderDockerInput(req *workflowpb.RenderDockerInputWorkflowRequest) (*deploymentpb.Docker_Input, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	plan := req.GetPlan()
	if plan.GetProvider() != deploymentpb.Provider_PROVIDER_DOCKER {
		return nil, fmt.Errorf("docker input requires docker provider, got %s", plan.GetProvider())
	}

	containers := make(map[string]*deploymentpb.Docker_Container, len(plan.GetMachines()))
	for _, machine := range plan.GetMachines() {
		container := machine.GetDocker()
		if container == nil {
			return nil, fmt.Errorf("machine %q has no docker params", machine.GetNodeId())
		}
		name := dockerResourceName(req.GetRunId(), machine.GetNodeId())
		clone := proto.Clone(container).(*deploymentpb.Docker_Container)
		if err := applyDockerAgentBootstrap(req.GetRunId(), machine.GetNodeId(), clone, req.GetAgentBootstrap()); err != nil {
			return nil, err
		}
		if clone.Labels == nil {
			clone.Labels = map[string]string{}
		}
		clone.Labels["stroppy.cloud/run_id"] = req.GetRunId()
		clone.Labels["stroppy.cloud/node_id"] = machine.GetNodeId()
		clone.Labels["stroppy.cloud/resource_name"] = name
		clone.DependsOn = dockerDependencyNames(req.GetRunId(), clone.GetDependsOn())
		containers[name] = clone
	}

	input := &deploymentpb.Docker_Input{
		Network: &deploymentpb.Docker_Network{
			Name: dockerNetworkName(req.GetRunId()),
		},
		Containers: containers,
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}
	return input, nil
}

func renderTerraformInput(req *workflowpb.RenderTerraformVariablesWorkflowRequest) (*deploymentpb.Terraform_Input, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	plan := req.GetPlan()
	if plan.GetProvider() != deploymentpb.Provider_PROVIDER_YANDEX {
		return nil, fmt.Errorf("terraform input requires yandex provider, got %s", plan.GetProvider())
	}

	settings := plan.GetSettings().GetYandex()
	if settings == nil {
		return nil, fmt.Errorf("yandex settings are required")
	}
	if err := settings.Validate(); err != nil {
		return nil, err
	}

	input, err := yandexInput(req.GetRunId(), plan, settings, req.GetAgentBootstrap())
	if err != nil {
		return nil, err
	}
	tfvars, err := bakedProto("yandex.tfvars", input)
	if err != nil {
		return nil, err
	}

	files, err := yandextf.EmbeddedTfFiles()
	if err != nil {
		return nil, err
	}
	sourceFiles := make([]*deploymentpb.Terraform_Operation_SourceFile, 0, len(files))
	for _, file := range files {
		sourceFiles = append(sourceFiles, &deploymentpb.Terraform_Operation_SourceFile{
			Path:    file.Name(),
			Content: file.Content(),
		})
	}

	action := req.GetAction()
	if action == deploymentpb.Terraform_ACTION_UNSPECIFIED {
		action = deploymentpb.Terraform_ACTION_APPLY
	}
	operation := &deploymentpb.Terraform_Operation{
		Action:                action,
		WorkdirId:             req.GetRunId(),
		VarFileName:           terraform.DefaultVarFileName,
		Files:                 sourceFiles,
		Env:                   yandexEnv(settings),
		Parallelism:           10,
		PreserveExistingState: true,
		DestroyOnApplyError:   action == deploymentpb.Terraform_ACTION_APPLY,
	}

	tfInput := &deploymentpb.Terraform_Input{
		Provider:  deploymentpb.Provider_PROVIDER_YANDEX,
		Operation: operation,
		Tfvars:    tfvars,
	}
	if action == deploymentpb.Terraform_ACTION_DESTROY {
		tfInput.Tfvars = nil
	}
	if err := tfInput.Validate(); err != nil {
		return nil, err
	}
	return tfInput, nil
}

func yandexInput(runID string, plan *deploymentpb.InfrastructurePlan, settings *deploymentpb.Yandex_Settings, bootstrap *workflowpb.AgentBootstrap) (*deploymentpb.Yandex_Input, error) {
	defaultZone := yandexSettingsZone(settings.GetZone())
	if defaultZone == "" {
		return nil, fmt.Errorf("unsupported yandex zone %s", settings.GetZone())
	}
	platformID := yandexSettingsPlatformID(settings.GetPlatformId())
	if platformID == "" {
		return nil, fmt.Errorf("unsupported yandex platform %s", settings.GetPlatformId())
	}

	vms := make(map[string]*deploymentpb.Yandex_Vm, len(plan.GetMachines()))
	zones := map[string]struct{}{}
	for _, machine := range plan.GetMachines() {
		vm := machine.GetYandex()
		if vm == nil {
			return nil, fmt.Errorf("machine %q has no yandex params", machine.GetNodeId())
		}
		name := yandexResourceName(runID, machine.GetNodeId())
		clone := proto.Clone(vm).(*deploymentpb.Yandex_Vm)
		clone.Zone = defaultZone
		clone.InternalIp = "auto"
		if clone.BootDiskType == "" {
			clone.BootDiskType = defaultYandexBootDiskType
		}
		clone.NetworkAcceleration = defaultYandexNetworkAcceleration
		if settings.GetSoftwareAcceleratedNetwork() {
			clone.NetworkAcceleration = "software_accelerated"
		}
		if clone.UserData == "" {
			userData, err := agentdomain.CloudInit(machine.GetNodeId(), agentBootstrap(bootstrap, machine.GetNodeId(), runID), agentdomain.CloudInitOptions{
				SSHUser:      settings.GetSshUser(),
				SSHPublicKey: settings.GetSshPublicKey(),
			})
			if err != nil {
				return nil, fmt.Errorf("render yandex cloud-init for %q: %w", machine.GetNodeId(), err)
			}
			clone.UserData = userData
		}
		clone.PublicIp = settings.GetAssignPublicIp()
		clone.MemoryGb = normalizeYandexMemoryGB(platformID, clone.GetCores(), clone.GetMemoryGb())
		clone.BootDiskGb = uint64(roundIOM3GB(int(clone.GetBootDiskGb()), clone.GetBootDiskType()))
		for _, disk := range clone.GetSecondaryDisks() {
			disk.SizeGb = uint32(roundIOM3GB(int(disk.GetSizeGb()), disk.GetType()))
		}
		zones[clone.GetZone()] = struct{}{}
		vms[name] = clone
	}

	networkCIDR := plan.GetLabels()[reservedNetworkCIDRLabel]
	if networkCIDR == "" {
		var err error
		networkCIDR, err = networkinfra.SelectRunCIDR(runID, settings.GetSubnetCidr(), nil)
		if err != nil {
			return nil, err
		}
	}

	subnets := map[string]*deploymentpb.Yandex_Subnet{}
	if len(zones) > 1 {
		zoneList := make([]string, 0, len(zones))
		for zone := range zones {
			zoneList = append(zoneList, zone)
		}
		zoneCIDRs, err := networkinfra.ZoneCIDRs(networkCIDR, zoneList)
		if err != nil {
			return nil, err
		}
		for zone, cidr := range zoneCIDRs {
			subnets[zone] = &deploymentpb.Yandex_Subnet{Zone: zone, Cidr: cidr}
		}
	}

	input := &deploymentpb.Yandex_Input{
		Network: &deploymentpb.Yandex_Network{
			Name:      yandexNetworkName(settings.GetNetworkName(), runID),
			NetworkId: settings.GetNetworkId(),
			Cidr:      networkCIDR,
			Zone:      defaultZone,
			Subnets:   subnets,
		},
		Compute: &deploymentpb.Yandex_Compute{
			PlatformId:       platformID,
			ImageId:          settings.GetImageId(),
			SerialPortEnable: true,
			Vms:              vms,
		},
	}

	managed, err := yandexManagedYDBInput(runID, plan, settings)
	if err != nil {
		return nil, err
	}
	input.ManagedYdb = managed

	if err := input.Validate(); err != nil {
		return nil, err
	}
	return input, nil
}

func normalizeYandexMemoryGB(platformID string, cores uint32, memoryGB uint64) uint64 {
	if memoryGB == 0 || cores == 0 {
		return memoryGB
	}
	switch platformID {
	case "standard-v3":
		step := uint64(cores)
		if memoryGB < step {
			return step
		}
		if rem := memoryGB % step; rem != 0 {
			return memoryGB + step - rem
		}
	}
	return memoryGB
}

func planWithReservedNetworkCIDR(plan *deploymentpb.InfrastructurePlan, cidr string) *deploymentpb.InfrastructurePlan {
	if plan == nil {
		return nil
	}
	clone := proto.Clone(plan).(*deploymentpb.InfrastructurePlan)
	if clone.Labels == nil {
		clone.Labels = map[string]string{}
	}
	clone.Labels[reservedNetworkCIDRLabel] = cidr
	return clone
}

// managedYDBInputLabel is the plan-label key carrying the managed-YDB provider
// input (deployment.Yandex_ManagedYdb as protojson), emitted by the managed-YDB
// database builder and propagated onto the InfrastructurePlan labels. Mirrors
// ydbmanaged.LabelManagedInput.
const managedYDBInputLabel = "managed_ydb_input"

// yandexManagedYDBInput reconstructs the managed-YDB provider input from the
// plan label (engine topology → managed_ydb tfvars) and fills the account-scoped
// fields (name/folder/location/service account) from provider settings. Returns
// nil when the plan has no managed-YDB database.
func yandexManagedYDBInput(runID string, plan *deploymentpb.InfrastructurePlan, settings *deploymentpb.Yandex_Settings) (*deploymentpb.Yandex_ManagedYdb, error) {
	encoded := plan.GetLabels()[managedYDBInputLabel]
	if encoded == "" {
		return nil, nil
	}

	managed := &deploymentpb.Yandex_ManagedYdb{}
	if err := protojson.Unmarshal([]byte(encoded), managed); err != nil {
		return nil, fmt.Errorf("decode managed ydb input: %w", err)
	}

	managed.Name = yandexManagedYDBName(runID)
	managed.FolderId = settings.GetFolderId()
	managed.LocationId = yandexManagedYDBLocation(settings.GetZone())
	if managed.GetServiceAccountName() == "" {
		managed.ServiceAccountName = sanitizeProviderName("stroppy-" + runID)
	}
	return managed, nil
}

// yandexManagedYDBName derives a folder-unique, RFC1035-ish managed YDB name.
func yandexManagedYDBName(runID string) string {
	name := sanitizeProviderName("stroppy-" + runID)
	if len(name) > 63 {
		name = strings.TrimRight(name[:63], "-")
	}
	return name
}

// yandexManagedYDBLocation maps the per-VM zone to the zone-less YDB location id
// ("ru-central1-b" → "ru-central1") that Yandex Managed YDB expects.
func yandexManagedYDBLocation(zone deploymentpb.Yandex_Settings_Zone) string {
	z := yandexSettingsZone(zone)
	if z == "" {
		return ""
	}
	if i := strings.LastIndex(z, "-"); i > 0 && len(z)-i <= 3 {
		return z[:i]
	}
	return z
}

func yandexEnv(settings *deploymentpb.Yandex_Settings) map[string]string {
	return map[string]string{
		"YC_TOKEN":     settings.GetToken(),
		"YC_CLOUD_ID":  settings.GetCloudId(),
		"YC_FOLDER_ID": settings.GetFolderId(),
		"YC_ZONE":      yandexSettingsZone(settings.GetZone()),
	}
}

func applyDockerAgentBootstrap(runID, nodeID string, container *deploymentpb.Docker_Container, bootstrap *workflowpb.AgentBootstrap) error {
	boot := agentBootstrap(bootstrap, nodeID, runID)
	env, err := agentdomain.Env(nodeID, boot)
	if err != nil {
		return fmt.Errorf("render docker agent env for %q: %w", nodeID, err)
	}
	mergedEnv := mergeStringMap(container.GetEnv(), env)
	container.Env = mergedEnv
	container.Files = upsertDockerFile(container.GetFiles(), &deploymentpb.Docker_File{
		Path:    agentdomain.DockerEnvFilePath,
		Content: []byte(agentdomain.EnvFileFromMap(mergedEnv)),
		Mode:    0644,
	})
	if aptProxyConfig := agentdomain.AptProxyConfig(boot.ServerAddr); aptProxyConfig != "" {
		container.Files = upsertDockerFile(container.GetFiles(), &deploymentpb.Docker_File{
			Path:    agentdomain.DockerAptProxyFilePath,
			Content: []byte(aptProxyConfig),
			Mode:    0644,
		})
	}
	return nil
}

func agentBootstrap(input *workflowpb.AgentBootstrap, nodeID, runID string) agentdomain.Bootstrap {
	if input == nil {
		return agentdomain.Bootstrap{RunID: runID}
	}
	return agentdomain.Bootstrap{
		ServerAddr:        input.GetServerAddr(),
		BinaryURL:         input.GetBinaryUrl(),
		TemporalNamespace: input.GetTemporalNamespace(),
		RunID:             runID,
		ExtraEnv:          input.GetExtraEnv(),
		AgentToken:        input.GetAgentTokens()[nodeID],
		AgentTaskQueue:    input.GetAgentTaskQueues()[nodeID],
	}
}

func mergeStringMap(base, override map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(override))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range override {
		out[key] = value
	}
	return out
}

func upsertDockerFile(files []*deploymentpb.Docker_File, file *deploymentpb.Docker_File) []*deploymentpb.Docker_File {
	out := make([]*deploymentpb.Docker_File, 0, len(files)+1)
	replaced := false
	for _, existing := range files {
		if existing.GetPath() == file.GetPath() {
			out = append(out, file)
			replaced = true
			continue
		}
		out = append(out, existing)
	}
	if !replaced {
		out = append(out, file)
	}
	return out
}

func terraformYandexOutput(output *deploymentpb.Terraform_Output) (*deploymentpb.Yandex_Output, error) {
	if output.GetOutputs() == nil {
		return nil, fmt.Errorf("terraform outputs are missing")
	}
	data, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(output.GetOutputs().GetValues())
	if err != nil {
		return nil, err
	}
	var yandexOutput deploymentpb.Yandex_Output
	// Tolerate terraform outputs that are not (yet) modelled in the proto
	// (e.g. stroppy_service_account_id); strict decoding would fail the run
	// AFTER the VMs are applied, leaking infrastructure.
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &yandexOutput); err != nil {
		return nil, fmt.Errorf("decode yandex terraform outputs: %w", err)
	}
	return &yandexOutput, nil
}

func bakedProto(name string, msg proto.Message) (*schemapb.Baked, error) {
	data, err := protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}.Marshal(msg)
	if err != nil {
		return nil, err
	}
	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, err
	}
	return bakedValues(name, values)
}

func bakedValues(name string, values map[string]any) (*schemapb.Baked, error) {
	st, err := structpb.NewStruct(values)
	if err != nil {
		return nil, err
	}
	return &schemapb.Baked{
		Schema: &schemapb.Schema{
			Id: &schemapb.SchemaIdentity{
				Namespace: "stroppy-cloud",
				Name:      name,
				Version:   "v1",
			},
		},
		Values: st,
	}, nil
}

func yandexSettingsZone(zone deploymentpb.Yandex_Settings_Zone) string {
	switch zone {
	case deploymentpb.Yandex_Settings_ZONE_RU_CENTRAL1_A:
		return "ru-central1-a"
	case deploymentpb.Yandex_Settings_ZONE_RU_CENTRAL1_B:
		return "ru-central1-b"
	case deploymentpb.Yandex_Settings_ZONE_RU_CENTRAL1_D:
		return "ru-central1-d"
	default:
		return ""
	}
}

func yandexSettingsPlatformID(platform deploymentpb.Yandex_Settings_PlatformId) string {
	switch platform {
	case deploymentpb.Yandex_Settings_PLATFORM_ID_UNSPECIFIED:
		return defaultYandexPlatformID
	case deploymentpb.Yandex_Settings_PLATFORM_ID_STANDARD_V1:
		return "standard-v1"
	case deploymentpb.Yandex_Settings_PLATFORM_ID_STANDARD_V2:
		return "standard-v2"
	case deploymentpb.Yandex_Settings_PLATFORM_ID_STANDARD_V3:
		return "standard-v3"
	case deploymentpb.Yandex_Settings_PLATFORM_ID_HIGHFREQ_V3:
		return "highfreq-v3"
	default:
		return ""
	}
}

func roundIOM3GB(sizeGB int, diskType string) int {
	if sizeGB <= 0 {
		return sizeGB
	}
	if diskType != "network-ssd-io-m3" {
		return sizeGB
	}
	return int(math.Ceil(float64(sizeGB)/93.0)) * 93
}

func dockerNetworkName(runID string) string {
	return sanitizeProviderName("stroppy-" + runID)
}

func dockerResourceName(runID, nodeID string) string {
	return sanitizeProviderName("stroppy-" + runID + "-" + nodeID)
}

func yandexNetworkName(base, runID string) string {
	if base == "" {
		base = "stroppy"
	}
	return sanitizeProviderName(base + "-" + runID)
}

func yandexResourceName(runID, nodeID string) string {
	return sanitizeProviderName("stroppy-" + runID + "-" + nodeID)
}

func dockerDependencyNames(runID string, deps []string) []string {
	out := make([]string, 0, len(deps))
	for _, dep := range deps {
		out = append(out, dockerResourceName(runID, dep))
	}
	return out
}

var providerNameInvalid = regexp.MustCompile(`[^a-z0-9-]+`)

func sanitizeProviderName(name string) string {
	name = strings.ToLower(name)
	name = providerNameInvalid.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		name = "stroppy"
	}
	if name[0] < 'a' || name[0] > 'z' {
		name = "s-" + name
	}
	if len(name) > 63 {
		// Truncating to 63 chars naively drops the trailing node index
		// (e.g. "stroppy-<uuid>-picodata-instance-1" is 64 chars, so the "-1"
		// is cut and all instances collapse to the same name -> Terraform's
		// for_each VM map keeps only ONE VM, the other instances' agents never
		// come online and the deploy hangs on EnsureAgentOnline). Append a short
		// deterministic hash of the full name to a trimmed prefix to keep names
		// unique and stable across applies.
		sum := sha1.Sum([]byte(name))
		suffix := "-" + hex.EncodeToString(sum[:])[:8]
		name = strings.TrimRight(name[:63-len(suffix)], "-") + suffix
	}
	return name
}
