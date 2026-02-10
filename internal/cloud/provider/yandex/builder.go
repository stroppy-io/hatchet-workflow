package yandex

import (
	"fmt"
	"os"
	"strings"

	"github.com/samber/lo"
	"github.com/stroppy-io/hatchet-workflow/internal/core/consts"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iancoleman/strcase"
	"github.com/stroppy-io/hatchet-workflow/internal/cloud/cloud-init"
	"github.com/stroppy-io/hatchet-workflow/internal/core/ids"
	"github.com/stroppy-io/hatchet-workflow/internal/core/protoyaml"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
)

type ProviderConfig struct {
	K8sNamespace string `mapstructure:"k8s_namespace" validate:"required"`

	DefaultVmZone       string `mapstructure:"default_vm_zone" validate:"required"`
	DefaultVmPlatformId string `mapstructure:"default_vm_platform_id" validate:"required"`
}

// yamlKeys

const (
	CloudVPCCrossplaneApiVersion     consts.ConstValue = "vpc.yandex-cloud.jet.crossplane.io/v1alpha1"
	CloudComputeCrossplaneApiVersion consts.ConstValue = "compute.yandex-cloud.jet.crossplane.io/v1alpha1"

	ExternalNameAnnotation consts.Str = "crossplane.io/external-name"
	constUserDataKey       consts.Str = "user-data"
	serialPortEnableKey    consts.Str = "serial-port-enable"

	serialPortEnableEnvKey consts.EnvKey = "YANDEX_SERIAL_PORT_ENABLE"
)

const (
	one       consts.Str = "1"
	zero      consts.Str = "0"
	trueValue consts.Str = "true"
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

func (y *CloudBuilder) BuildNetwork(
	template *crossplane.Network_Template,
) (*crossplane.Network, error) {
	networkRef := &crossplane.Ref{
		Name:      template.GetIdentifier().GetName(),
		Namespace: y.Config.K8sNamespace,
	}
	networkDef := &crossplane.ResourceDef{
		ApiVersion: CloudVPCCrossplaneApiVersion,
		Kind:       resourceKindToString(crossplane.YandexCloud_NETWORK),
		Metadata: &crossplane.Metadata{
			Name:      networkRef.GetName(),
			Namespace: networkRef.GetNamespace(),
			Annotations: map[string]string{
				ExternalNameAnnotation: template.GetExternalId(), // Default network ID created outside
			},
		},
		Spec: &crossplane.ResourceDef_Spec{
			DeletionPolicy:    strcase.ToCamel(crossplane.CrossplaneDeletionPolicy_ORPHAN.String()),
			ProviderConfigRef: defaultProviderConfigRef(),
			ForProvider: &crossplane.ResourceDef_Spec_YandexCloudNetwork{
				YandexCloudNetwork: &crossplane.YandexCloud_Network{
					Name: networkRef.GetName(),
				},
			},
		},
	}
	networkYaml, err := y.marshalWithReplaceOneOffs(networkDef)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal network def: %w", err)
	}
	return &crossplane.Network{
		Template: template,
		Resource: &crossplane.Resource{
			Ref:          ids.ExtRefFromResourceDef(networkRef, networkDef),
			ResourceDef:  networkDef,
			CreatedAt:    timestamppb.Now(),
			UpdatedAt:    timestamppb.Now(),
			ResourceYaml: networkYaml,
			Status:       crossplane.Resource_STATUS_CREATING,
			UsingQuotas: []*crossplane.Quota{
				{
					Cloud:   crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX,
					Kind:    crossplane.Quota_KIND_NETWORK,
					Current: 1,
				},
			},
		},
	}, nil
}

func (y *CloudBuilder) BuildSubnet(
	network *crossplane.Network,
	template *crossplane.Subnet_Template,
) (*crossplane.Subnet, error) {
	subnetRef := &crossplane.Ref{
		Name:      template.GetIdentifier().GetName(),
		Namespace: y.Config.K8sNamespace,
	}
	subnetDef := &crossplane.ResourceDef{
		ApiVersion: CloudVPCCrossplaneApiVersion,
		Kind:       resourceKindToString(crossplane.YandexCloud_SUBNET),
		Metadata: &crossplane.Metadata{
			Name:        subnetRef.GetName(),
			Namespace:   subnetRef.GetNamespace(),
			Annotations: map[string]string{
				// we don't use external name for subnet if create in code
				//ExternalNameAnnotation: y.Config.DefaultSubnetId, // Default subnet ID created outside
			},
		},
		Spec: &crossplane.ResourceDef_Spec{
			DeletionPolicy:    strcase.ToCamel(crossplane.CrossplaneDeletionPolicy_DELETE.String()),
			ProviderConfigRef: defaultProviderConfigRef(),
			ForProvider: &crossplane.ResourceDef_Spec_YandexCloudSubnet{
				YandexCloudSubnet: &crossplane.YandexCloud_Subnet{
					Name: subnetRef.GetName(),
					NetworkIdRef: &crossplane.YandexCloud_Subnet_NetworkIdRef{
						Name: network.GetTemplate().GetIdentifier().GetName(),
					},
					V4CidrBlocks: []string{template.GetCidr().GetValue()},
					Zone:         y.Config.DefaultVmZone, // create in same zone as vm
				},
			},
		},
	}
	subnetYaml, err := y.marshalWithReplaceOneOffs(subnetDef)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal subnet def: %w", err)
	}
	return &crossplane.Subnet{
		Template: template,
		Resource: &crossplane.Resource{
			Ref:          ids.ExtRefFromResourceDef(subnetRef, subnetDef),
			ResourceDef:  subnetDef,
			CreatedAt:    timestamppb.Now(),
			UpdatedAt:    timestamppb.Now(),
			ResourceYaml: subnetYaml,
			Status:       crossplane.Resource_STATUS_CREATING,
			UsingQuotas: []*crossplane.Quota{
				{
					Cloud:   crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX,
					Kind:    crossplane.Quota_KIND_SUBNET,
					Current: 1,
				},
			},
		},
	}, nil

}

func (y *CloudBuilder) BuildVm(
	_ *crossplane.Network,
	subnet *crossplane.Subnet,
	template *crossplane.Vm_Template,
) (*crossplane.Vm, error) {
	machineScriptBytes, err := cloud_init.GenerateCloudInit(template.GetCloudInit())
	if err != nil {
		return nil, fmt.Errorf("failed to generate cloud-init: %w", err)
	}
	metadata := map[string]string{
		serialPortEnableKey: lo.Ternary(
			os.Getenv(serialPortEnableEnvKey) == trueValue, one, zero,
		),
		constUserDataKey: string(machineScriptBytes),
	}
	subnetRef := &crossplane.Ref{
		Name:      subnet.GetTemplate().GetIdentifier().GetName(),
		Namespace: y.Config.K8sNamespace,
	}
	vmRef := &crossplane.Ref{
		Name:      template.GetIdentifier().GetName(),
		Namespace: y.Config.K8sNamespace,
	}
	saveSecretTo := &crossplane.Ref{
		Name:      fmt.Sprintf("%s-access-secret", vmRef.GetName()),
		Namespace: y.Config.K8sNamespace,
	}
	vmResourceDef := &crossplane.ResourceDef{
		ApiVersion: CloudComputeCrossplaneApiVersion,
		Kind:       resourceKindToString(crossplane.YandexCloud_INSTANCE),
		Metadata: &crossplane.Metadata{
			Name:      vmRef.GetName(),
			Namespace: vmRef.GetNamespace(),
		},
		Spec: &crossplane.ResourceDef_Spec{
			DeletionPolicy:             strcase.ToCamel(crossplane.CrossplaneDeletionPolicy_DELETE.String()),
			WriteConnectionSecretToRef: saveSecretTo,
			ProviderConfigRef:          defaultProviderConfigRef(),
			ForProvider: &crossplane.ResourceDef_Spec_YandexCloudVm{
				YandexCloudVm: &crossplane.YandexCloud_Vm{
					Name:       vmRef.GetName(),
					PlatformId: y.Config.DefaultVmPlatformId,
					Zone:       y.Config.DefaultVmZone,
					Resources: []*crossplane.YandexCloud_Vm_Resources{
						{
							Cores:  template.GetHardware().GetCores(),
							Memory: template.GetHardware().GetMemory(),
						},
					},
					// yaml format shit in this block
					BootDisk: []*crossplane.YandexCloud_Vm_Disk{
						{
							InitializeParams: []*crossplane.YandexCloud_Vm_Disk_InitializeParams{
								{
									ImageId: template.GetBaseImageId(),
								},
							},
						},
					},
					NetworkInterface: []*crossplane.YandexCloud_Vm_NetworkInterface{
						{
							SubnetIdRef: &crossplane.OnlyNameRef{
								Name: subnetRef.GetName(),
							},
							Nat:       template.GetPublicIp(),
							IpAddress: template.GetInternalIp().GetValue(),
						},
					},
					Metadata: metadata,
				},
			},
		},
	}
	vmYaml, err := y.marshalWithReplaceOneOffs(vmResourceDef)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal vm resource def: %w", err)
	}
	quotas := []*crossplane.Quota{
		{
			Cloud:   crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX,
			Kind:    crossplane.Quota_KIND_VM,
			Current: 1,
		},
	}
	if template.GetPublicIp() {
		quotas = append(quotas, &crossplane.Quota{
			Cloud:   crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX,
			Kind:    crossplane.Quota_KIND_PUBLIC_IP_ADDRESS,
			Current: 1,
		})
	}
	return &crossplane.Vm{
		Template: template,
		Resource: &crossplane.Resource{
			Ref:          ids.ExtRefFromResourceDef(vmRef, vmResourceDef),
			ResourceDef:  vmResourceDef,
			CreatedAt:    timestamppb.Now(),
			UpdatedAt:    timestamppb.Now(),
			ResourceYaml: vmYaml,
			Status:       crossplane.Resource_STATUS_CREATING,
			UsingQuotas:  quotas,
		},
	}, nil
}
