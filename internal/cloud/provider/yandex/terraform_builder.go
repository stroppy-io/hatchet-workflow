package yandex

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/samber/lo"
	"github.com/stroppy-io/hatchet-workflow/internal/cloud/cloud-init"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type TerraformBuilder struct {
	Config *ProviderConfig
}

func NewTerraformBuilder(config *ProviderConfig) *TerraformBuilder {
	return &TerraformBuilder{Config: config}
}

type NetworkVar struct {
	Name string `json:"name"`
}

type SubnetVar struct {
	Name        string `json:"name"`
	Zone        string `json:"zone"`
	NetworkName string `json:"network_name"`
	Cidr        string `json:"cidr"`
}

type VmVar struct {
	Name             string `json:"name"`
	PlatformID       string `json:"platform_id"`
	Zone             string `json:"zone"`
	Cores            uint32 `json:"cores"`
	Memory           uint32 `json:"memory"`
	ImageID          string `json:"image_id"`
	SubnetName       string `json:"subnet_name"`
	Nat              bool   `json:"nat"`
	InternalIP       string `json:"internal_ip"`
	UserData         string `json:"user_data"`
	SerialPortEnable string `json:"serial_port_enable"`
}

func (y *TerraformBuilder) BuildNetwork(
	template *crossplane.Network_Template,
) (*crossplane.Network, error) {
	v := NetworkVar{
		Name: template.GetIdentifier().GetName(),
	}
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal network var: %w", err)
	}

	return &crossplane.Network{
		Template: template,
		Resource: &crossplane.Resource{
			Ref: &crossplane.ExtRef{
				ApiVersion: "yandex-cloud/yandex",
				Kind:       "yandex_vpc_network",
				Name:       template.GetIdentifier().GetName(),
			},
			CreatedAt:    timestamppb.Now(),
			UpdatedAt:    timestamppb.Now(),
			ResourceYaml: string(jsonBytes),
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

func (y *TerraformBuilder) BuildSubnet(
	network *crossplane.Network,
	template *crossplane.Subnet_Template,
) (*crossplane.Subnet, error) {
	v := SubnetVar{
		Name:        template.GetIdentifier().GetName(),
		Zone:        y.Config.DefaultVmZone,
		NetworkName: network.GetTemplate().GetIdentifier().GetName(),
		Cidr:        template.GetCidr().GetValue(),
	}
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal subnet var: %w", err)
	}

	return &crossplane.Subnet{
		Template: template,
		Resource: &crossplane.Resource{
			Ref: &crossplane.ExtRef{
				ApiVersion: "yandex-cloud/yandex",
				Kind:       "yandex_vpc_subnet",
				Name:       template.GetIdentifier().GetName(),
			},
			CreatedAt:    timestamppb.Now(),
			UpdatedAt:    timestamppb.Now(),
			ResourceYaml: string(jsonBytes),
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

func (y *TerraformBuilder) BuildVm(
	_ *crossplane.Network,
	subnet *crossplane.Subnet,
	template *crossplane.Vm_Template,
) (*crossplane.Vm, error) {
	machineScriptBytes, err := cloud_init.GenerateCloudInit(template.GetCloudInit())
	if err != nil {
		return nil, fmt.Errorf("failed to generate cloud-init: %w", err)
	}

	serialPortEnable := lo.Ternary(os.Getenv(string(serialPortEnableEnvKey)) == string(trueValue), string(one), string(zero))

	v := VmVar{
		Name:             template.GetIdentifier().GetName(),
		PlatformID:       y.Config.DefaultVmPlatformId,
		Zone:             y.Config.DefaultVmZone,
		Cores:            template.GetHardware().GetCores(),
		Memory:           template.GetHardware().GetMemory(),
		ImageID:          template.GetBaseImageId(),
		SubnetName:       subnet.GetTemplate().GetIdentifier().GetName(),
		Nat:              template.GetPublicIp(),
		InternalIP:       template.GetInternalIp().GetValue(),
		UserData:         string(machineScriptBytes),
		SerialPortEnable: serialPortEnable,
	}

	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal vm var: %w", err)
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
			Ref: &crossplane.ExtRef{
				ApiVersion: "yandex-cloud/yandex",
				Kind:       "yandex_compute_instance",
				Name:       template.GetIdentifier().GetName(),
			},
			CreatedAt:    timestamppb.Now(),
			UpdatedAt:    timestamppb.Now(),
			ResourceYaml: string(jsonBytes),
			Status:       crossplane.Resource_STATUS_CREATING,
			UsingQuotas:  quotas,
		},
	}, nil
}
