package planner

import "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"

// DeploymentParams carries the resolved, provider-specific values the planner
// needs to render terraform vars. Closed-set values are typed deployment.Yandex
// enums (translated to terraform strings only at marshal, via the yandex module's
// mappers); free identifiers stay strings. Pure data — the planner stays I/O-free.
// The caller (RunService) builds it from tenant provider settings (SettingsService),
// the subnet network allocation (D20), and the agent cloud-init bootstrap (D18).
type DeploymentParams struct {
	Platform            deployment.Yandex_PlatformId
	Zone                deployment.Yandex_Zone
	BootDiskType        deployment.Yandex_DiskType
	NetworkAcceleration deployment.Yandex_NetworkAcceleration

	ImageID        string
	NetworkID      string
	NetworkName    string
	NetworkCIDR    string
	AssignPublicIP bool

	// Per-machine values (machine id -> value).
	//
	// TODO(planner): MachineInternalIP comes from the subnet network allocation
	// (D20); MachineUserData is the agent cloud-init JWT (D18). Empty until wired.
	MachineInternalIP map[string]string
	MachineUserData   map[string]string
}

// bootDiskType returns the disk type, defaulting to network-ssd when unset.
func (p *DeploymentParams) bootDiskType() deployment.Yandex_DiskType {
	if p == nil || p.BootDiskType == deployment.Yandex_DISK_TYPE_UNSPECIFIED {
		return deployment.Yandex_DISK_TYPE_NETWORK_SSD
	}
	return p.BootDiskType
}

// networkAcceleration returns the acceleration mode, defaulting to standard.
func (p *DeploymentParams) networkAcceleration() deployment.Yandex_NetworkAcceleration {
	if p == nil || p.NetworkAcceleration == deployment.Yandex_NETWORK_ACCELERATION_UNSPECIFIED {
		return deployment.Yandex_NETWORK_ACCELERATION_STANDARD
	}
	return p.NetworkAcceleration
}

func (p *DeploymentParams) internalIP(machineID string) string {
	if p == nil {
		return ""
	}
	return p.MachineInternalIP[machineID]
}

func (p *DeploymentParams) userData(machineID string) string {
	if p == nil {
		return ""
	}
	return p.MachineUserData[machineID]
}
