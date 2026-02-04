package yandex

import (
	"fmt"
	"net"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/stroppy-io/hatchet-workflow/internal/cloud/cloud-init"
	"github.com/stroppy-io/hatchet-workflow/internal/core/defaults"
	ids "github.com/stroppy-io/hatchet-workflow/internal/core/ids"
	"github.com/stroppy-io/hatchet-workflow/internal/core/protoyaml"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type ProviderConfig struct {
	K8sNamespace       string `mapstructure:"k8s_namespace" validate:"required"`
	DefaultNetworkName string `mapstructure:"default_network_name" validate:"required"`
	DefaultNetworkId   string `mapstructure:"default_network_id" validate:"required"`

	DefaultVmZone       string `mapstructure:"default_vm_zone" validate:"required"`
	DefaultVmPlatformId string `mapstructure:"default_vm_platform_id" validate:"required"`
}

const (
	CloudVPCCrossplaneApiVersion     = "vpc.yandex-cloud.jet.crossplane.io/v1alpha1"
	CloudComputeCrossplaneApiVersion = "compute.yandex-cloud.jet.crossplane.io/v1alpha1"
)

// yamlKeys

const (
	ExternalNameAnnotation = "crossplane.io/external-name"
)

// dfaultValues
const (
	defaultNetworkName = "stroppy-crossplane-net"
	constUserDataKey   = "user-data"
)

type CloudBuilder struct {
	Config *ProviderConfig
}

func NewCloudBuilder(config *ProviderConfig) *CloudBuilder {
	return &CloudBuilder{Config: config}
}

func defaultProviderConfigRef() map[string]string {
	return map[string]string{
		"name": "default",
	}
}

func resourceKindToString(kind crossplane.YandexCloud_ResourceKind) string {
	return strcase.ToCamel(kind.String())
}

func resourceKindFromString(kind string) crossplane.YandexCloud_ResourceKind {
	knd, ok := crossplane.YandexCloud_ResourceKind_value[strings.ToUpper(kind)]
	if !ok {
		panic(fmt.Sprintf(".resourceKindFromString unknown yandex cloud resource kind: %s", kind))
	}
	return crossplane.YandexCloud_ResourceKind(knd)
}

func (y *CloudBuilder) newNetworkDef(networkIdRef *crossplane.Ref) *crossplane.ResourceDef {
	return &crossplane.ResourceDef{
		ApiVersion: CloudVPCCrossplaneApiVersion,
		Kind:       resourceKindToString(crossplane.YandexCloud_NETWORK),
		Metadata: &crossplane.Metadata{
			Name:      networkIdRef.GetName(),
			Namespace: networkIdRef.GetNamespace(),
			Annotations: map[string]string{
				ExternalNameAnnotation: y.Config.DefaultNetworkId, // Default network ID created outside
			},
		},
		Spec: &crossplane.ResourceDef_Spec{
			DeletionPolicy:    strcase.ToCamel(crossplane.CrossplaneDeletionPolicy_ORPHAN.String()),
			ProviderConfigRef: defaultProviderConfigRef(),
			ForProvider: &crossplane.ResourceDef_Spec_YandexCloudNetwork{
				YandexCloudNetwork: &crossplane.YandexCloud_Network{
					Name: networkIdRef.GetName(),
				},
			},
		},
	}
}

func (y *CloudBuilder) newSubnetDef(
	networkIdRef *crossplane.Ref,
	subnetIdRef *crossplane.Ref,
	usingCidr *net.IPNet,
) *crossplane.ResourceDef {
	return &crossplane.ResourceDef{
		ApiVersion: CloudVPCCrossplaneApiVersion,
		Kind:       resourceKindToString(crossplane.YandexCloud_SUBNET),
		Metadata: &crossplane.Metadata{
			Name:        subnetIdRef.GetName(),
			Namespace:   subnetIdRef.GetNamespace(),
			Annotations: map[string]string{
				// we don't use external name for subnet if creat in code
				//ExternalNameAnnotation: y.Config.DefaultSubnetId, // Default subnet ID created outside
			},
		},
		Spec: &crossplane.ResourceDef_Spec{
			DeletionPolicy:    strcase.ToCamel(crossplane.CrossplaneDeletionPolicy_DELETE.String()),
			ProviderConfigRef: defaultProviderConfigRef(),
			ForProvider: &crossplane.ResourceDef_Spec_YandexCloudSubnet{
				YandexCloudSubnet: &crossplane.YandexCloud_Subnet{
					Name: subnetIdRef.GetName(),
					NetworkIdRef: &crossplane.YandexCloud_Subnet_NetworkIdRef{
						Name: networkIdRef.GetName(),
					},
					V4CidrBlocks: []string{usingCidr.String()},
					Zone:         y.Config.DefaultVmZone, // create in same zone as vm
				},
			},
		},
	}
}

var ErrEmptyInternalIp = fmt.Errorf("internal ip is empty in deployment")

func (y *CloudBuilder) newVmDef(
	ref *crossplane.Ref,
	subnetIdRef *crossplane.Ref,
	connectCredsRef *crossplane.Ref,
	vm *crossplane.Deployment_Vm,
) (*crossplane.ResourceDef, error) {
	if vm.GetNetworkParams().GetInternalIp().GetValue() == "" {
		return nil, ErrEmptyInternalIp
	}

	machineScriptBytes, err := cloud_init.GenerateCloudInit(vm.GetCloudInit())
	if err != nil {
		return nil, fmt.Errorf("failed to generate cloud-init: %w", err)
	}

	if vm.GetMachineInfo().GetBaseImageId() == "" {
		return nil, fmt.Errorf("vm image id is empty")
	}

	metadata := make(map[string]string)
	metadata[constUserDataKey] = string(machineScriptBytes)

	return &crossplane.ResourceDef{
		ApiVersion: CloudComputeCrossplaneApiVersion,
		Kind:       resourceKindToString(crossplane.YandexCloud_INSTANCE),
		Metadata: &crossplane.Metadata{
			Name:      ref.GetName(),
			Namespace: ref.GetNamespace(),
		},
		Spec: &crossplane.ResourceDef_Spec{
			DeletionPolicy:             strcase.ToCamel(crossplane.CrossplaneDeletionPolicy_DELETE.String()),
			WriteConnectionSecretToRef: connectCredsRef,
			ProviderConfigRef:          defaultProviderConfigRef(),
			ForProvider: &crossplane.ResourceDef_Spec_YandexCloudVm{
				YandexCloudVm: &crossplane.YandexCloud_Vm{
					Name:       ref.GetName(),
					PlatformId: y.Config.DefaultVmPlatformId,
					Zone:       y.Config.DefaultVmZone,
					Resources: []*crossplane.YandexCloud_Vm_Resources{
						{
							Cores:  vm.GetMachineInfo().GetCores(),
							Memory: vm.GetMachineInfo().GetMemory(),
						},
					},
					// yaml format shit in this block
					BootDisk: []*crossplane.YandexCloud_Vm_Disk{
						{
							InitializeParams: []*crossplane.YandexCloud_Vm_Disk_InitializeParams{
								{
									ImageId: vm.GetMachineInfo().GetBaseImageId(),
								},
							},
						},
					},
					NetworkInterface: []*crossplane.YandexCloud_Vm_NetworkInterface{
						{
							SubnetIdRef: &crossplane.OnlyNameRef{
								Name: subnetIdRef.GetName(),
							},
							Nat:       vm.GetNetworkParams().GetPublicIp(),
							IpAddress: vm.GetNetworkParams().GetInternalIp().GetValue(),
						},
					},
					Metadata: metadata,
				},
			},
		},
	}, nil
}

func (y *CloudBuilder) marshalWithReplaceOneOffs(def *crossplane.ResourceDef) (string, error) {
	yaml, err := protoyaml.Marshal(def)
	if err != nil {
		return "", err
	}
	replacedSymbol := ""
	switch def.GetSpec().GetForProvider().(type) {
	case *crossplane.ResourceDef_Spec_YandexCloudVm:
		replacedSymbol = "yandexCloudVm"
	case *crossplane.ResourceDef_Spec_YandexCloudNetwork:
		replacedSymbol = "yandexCloudNetwork"
	case *crossplane.ResourceDef_Spec_YandexCloudSubnet:
		replacedSymbol = "yandexCloudSubnet"
	}
	return strings.ReplaceAll(string(yaml), replacedSymbol, "forProvider"), nil
}

func (y *CloudBuilder) BuildVmCloudDetails(
	id string,
	network *crossplane.Network,
	vm *crossplane.Deployment_Vm,
) (*crossplane.Deployment_CloudDetails, error) {
	err := vm.Validate()
	if err != nil {
		return nil, fmt.Errorf("failed to validate vm: %w", err)
	}
	quotas := make([]*crossplane.Quota, 0)
	addQuota := func(kind crossplane.Quota_Kind) {
		quotas = append(quotas, &crossplane.Quota{
			Cloud:   crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX,
			Kind:    kind,
			Current: 1,
		})
	}

	// __WARNING__
	// Here we use vmId to generate unique names for vm
	// requestId used to generate unique names for subnet only
	// if caller of this function wants, they can set requestId to some other subnet (if they call twice)
	subnetName := strings.ToLower(fmt.Sprintf(
		"%s-subnet-%s",
		network.GetName(),
		network.GetId(),
	))
	// we use deployment id to generate unique names for vm (one deployment = one vm)
	machineName := fmt.Sprintf("cloud-vm-%s", id)
	// __WARNING__

	saveSecretTo := &crossplane.Ref{
		Name:      fmt.Sprintf("%s-access-secret", machineName),
		Namespace: y.Config.K8sNamespace,
	}
	networkRef := &crossplane.Ref{
		Name:      defaults.StringOrDefault(y.Config.DefaultNetworkName, defaultNetworkName),
		Namespace: y.Config.K8sNamespace,
	}
	subnetRef := &crossplane.Ref{
		Name:      subnetName,
		Namespace: y.Config.K8sNamespace,
	}
	networkDef := y.newNetworkDef(networkRef)
	//addQuota(crossplane.Quota_KIND_NETWORK) // now we use one network for all vms
	_, networkCidr, err := net.ParseCIDR(network.GetCidrWithIps().GetCidr().GetValue())
	if err != nil {
		return nil, fmt.Errorf("failed to parse network cidr: %w", err)
	}
	subnetDef := y.newSubnetDef(networkRef, subnetRef, networkCidr)
	addQuota(crossplane.Quota_KIND_SUBNET)
	vmRef := &crossplane.Ref{
		Name:      machineName,
		Namespace: y.Config.K8sNamespace,
	}
	vmDef, err := y.newVmDef(vmRef, subnetRef, saveSecretTo, vm)
	if err != nil {
		return nil, err
	}
	addQuota(crossplane.Quota_KIND_VM)
	if vm.GetNetworkParams().GetPublicIp() {
		addQuota(crossplane.Quota_KIND_PUBLIC_IP_ADDRESS)
	}

	vmYaml, err := y.marshalWithReplaceOneOffs(vmDef)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal vm def: %w", err)
	}
	subnetYaml, err := y.marshalWithReplaceOneOffs(subnetDef)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal subnet def: %w", err)
	}
	networkYaml, err := y.marshalWithReplaceOneOffs(networkDef)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal network def: %w", err)
	}

	return &crossplane.Deployment_CloudDetails{
		UsingQuotas: quotas,
		Resources: []*crossplane.Resource{
			{
				Ref:          ids.ExtRefFromResourceDef(networkRef, networkDef),
				ResourceDef:  networkDef,
				CreatedAt:    timestamppb.Now(),
				UpdatedAt:    timestamppb.Now(),
				ResourceYaml: networkYaml,
				Status:       crossplane.Resource_STATUS_CREATING,
			},
			{
				Ref:          ids.ExtRefFromResourceDef(subnetRef, subnetDef),
				ResourceDef:  subnetDef,
				CreatedAt:    timestamppb.Now(),
				UpdatedAt:    timestamppb.Now(),
				ResourceYaml: subnetYaml,
				Status:       crossplane.Resource_STATUS_CREATING,
			},
			{
				Ref:          ids.ExtRefFromResourceDef(vmRef, vmDef),
				ResourceDef:  vmDef,
				CreatedAt:    timestamppb.Now(),
				UpdatedAt:    timestamppb.Now(),
				ResourceYaml: vmYaml,
				Status:       crossplane.Resource_STATUS_CREATING,
			},
		},
	}, nil
}
