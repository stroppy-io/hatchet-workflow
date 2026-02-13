package yandex

import (
	"github.com/stroppy-io/hatchet-workflow/internal/core/consts"
	"github.com/stroppy-io/hatchet-workflow/internal/core/defaults"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/settings"
)

const (
	DefaultPlatformId consts.DefaultValue = "standard-v2"
)

type networking struct {
	Name       string `json:"name"`
	ExternalId string `json:"external_id"`
	Cidr       string `json:"cidr"`
}

type vm struct {
	Cores       int    `json:"cores"`
	Memory      int    `json:"memory"`
	DiskSize    int    `json:"disk_size"`
	HasPublicIp bool   `json:"has_public_ip"`
	UserData    string `json:"user_data"`
	InternalIp  string `json:"internal_ip"`
}

type compute struct {
	PlatformId       string        `json:"platform_id"`
	ImageId          string        `json:"image_id"`
	SerialPortEnable bool          `json:"serial_port_enable"`
	Vms              map[string]vm `json:"vms"`
}

type Variables struct {
	Networking networking `json:"networking"`
	Compute    compute    `json:"compute"`
}

func VariablesFromTemplate(
	settings *settings.Settings,
	depl *deployment.Deployment_Template,
) Variables {
	vars := Variables{
		Networking: networking{
			Name:       settings.GetYandexCloud().GetNetworkSettings().GetName(),
			ExternalId: settings.GetYandexCloud().GetNetworkSettings().GetExternalId(),
			Cidr:       depl.GetNetwork().GetCidr().GetValue(),
		},
		Compute: compute{
			PlatformId: defaults.StringOrDefault(
				settings.GetYandexCloud().GetVmSettings().GetPlatformId(),
				DefaultPlatformId,
			),
			ImageId:          settings.GetYandexCloud().GetVmSettings().GetBaseImageId(),
			SerialPortEnable: settings.GetYandexCloud().GetVmSettings().GetEnablePublicIps(),
		},
	}
	for _, vmTemplate := range depl.GetVmTemplates() {
		vars.Compute.Vms[vmTemplate.GetIdentifier().GetName()] = vm{
			Cores:       int(vmTemplate.GetHardware().GetCores()),
			Memory:      int(vmTemplate.GetHardware().GetMemory()),
			DiskSize:    int(vmTemplate.GetHardware().GetDisk()),
			HasPublicIp: vmTemplate.GetHasPublicIp(),
			UserData:    vmTemplate.GetCloudInit().GetContent(),
			InternalIp:  vmTemplate.GetInternalIp().GetValue(),
		}
	}
	return vars
}
