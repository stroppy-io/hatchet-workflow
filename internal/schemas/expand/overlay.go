package expand

import "github.com/stroppy-io/stroppy-cloud/internal/schemas/providers"

// ApplyProvider fills each VM's provider overlay (disk class, zone, platform,
// network acceleration) from the chosen provider's settings. It does NOT change
// shapes — only the provider-specific attributes. Unknown/docker providers are
// a no-op (docker has no zones/platform/disk classes).
func ApplyProvider(providerKind string, prov map[string]any, vms []VM) []VM {
	switch providerKind {
	case ProviderYandex:
		return applyYandex(prov, vms)
	default:
		return vms
	}
}

// Provider kind tokens (match the provider schema names).
const (
	ProviderYandex = "yandex"
	ProviderDocker = "docker"
)

func applyYandex(prov map[string]any, vms []VM) []VM {
	platform := mapStr(prov, providers.FieldPlatformID, providers.PlatformStandardV3)
	zones := mapStrSlice(prov, providers.FieldZones)
	disks := mapStrSlice(prov, providers.FieldDiskTypes)

	// default disk class: prefer network-ssd if in the catalog, else first entry.
	disk := providers.DiskNetworkSSD
	if len(disks) > 0 && !contains(disks, disk) {
		disk = disks[0]
	}

	accel := "standard"
	if mapBool(prov, providers.FieldSoftwareAcceleratedNetwork, false) {
		accel = "software_accelerated"
	}

	out := make([]VM, len(vms))
	for i, vm := range vms {
		vm.Provider.Platform = platform
		vm.Provider.DiskType = disk
		vm.Provider.NetworkAccel = accel
		if len(zones) > 0 {
			vm.Provider.Zone = zones[i%len(zones)] // round-robin spread across AZs
		}
		out[i] = vm
	}
	return out
}
