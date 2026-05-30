package providers

import "github.com/stroppy-io/stroppy-cloud/internal/schemas/types"

// This file is the string vocabulary for the provider schemas (yandex, docker):
// enum value sets + field names. Mirrors the postgres package convention.
// Provider schemas own the deployment-side facts a database is provider-agnostic
// about: credentials, region/network, the disk-type catalog, instance
// platforms/shapes, zones and quota limits.

// =============================================================================
// Enum value sets (string)
// =============================================================================

// Yandex Cloud compute platforms (platform_id).
const (
	PlatformStandardV2 = "standard-v2"
	PlatformStandardV3 = "standard-v3"
	PlatformHighfreqV3 = "highfreq-v3"
)

var PlatformIDValues = []string{PlatformStandardV2, PlatformStandardV3, PlatformHighfreqV3}

// Yandex Cloud disk classes (the disk-type catalog; postgres/machine never
// hardcode these — they live here on the provider).
const (
	DiskNetworkHDD              = "network-hdd"
	DiskNetworkSSD              = "network-ssd"
	DiskNetworkSSDNonReplicated = "network-ssd-nonreplicated"
	DiskNetworkSSDIOM3          = "network-ssd-io-m3"
)

var DiskTypeValues = []string{
	DiskNetworkHDD, DiskNetworkSSD, DiskNetworkSSDNonReplicated, DiskNetworkSSDIOM3,
}

// Network acceleration (per-machine Yandex override).
const (
	NetAccelStandard = "standard"
	NetAccelSoftware = "software_accelerated"
)

var NetAccelValues = []string{NetAccelStandard, NetAccelSoftware}

// Per-machine OVERRIDE field names (singular — the chosen value, vs the plural
// tenant-catalog fields above). These are the params a user may override on an
// abstract machine; they map to topology.Instance.provider_parms.
const (
	FieldDiskType     = types.Field("disk_type")
	FieldZone         = types.Field("zone")
	FieldNetworkAccel = types.Field("network_acceleration")
)

// =============================================================================
// Field names
// =============================================================================

const (
	// yandex
	FieldCloudID                    = types.Field("cloud_id")
	FieldFolderID                   = types.Field("folder_id")
	FieldToken                      = types.Field("token")
	FieldZones                      = types.Field("zones")
	FieldDefaultZone                = types.Field("default_zone")
	FieldPlatformID                 = types.Field("platform_id")
	FieldDiskTypes                  = types.Field("disk_types")
	FieldImageID                    = types.Field("image_id")
	FieldNetworkID                  = types.Field("network_id")
	FieldSubnetCIDR                 = types.Field("subnet_cidr")
	FieldAssignPublicIP             = types.Field("assign_public_ip")
	FieldSoftwareAcceleratedNetwork = types.Field("software_accelerated_network")
	FieldSSHUser                    = types.Field("ssh_user")
	FieldSSHPublicKey               = types.Field("ssh_public_key")

	// docker
	FieldImage      = types.Field("image")
	FieldNetwork    = types.Field("network")
	FieldPrivileged = types.Field("privileged")
	FieldDNS        = types.Field("dns")

	// limits (shared object name; sub-fields differ per provider)
	FieldLimits             = types.Field("limits")
	FieldMaxNodes           = types.Field("max_nodes")
	FieldMaxCoresPerNode    = types.Field("max_cores_per_node")
	FieldMaxMemoryGBPerNode = types.Field("max_memory_gb_per_node")
	FieldMaxDiskGBPerNode   = types.Field("max_disk_gb_per_node")
	FieldMaxContainers      = types.Field("max_containers")
	FieldMaxCores           = types.Field("max_cores")
	FieldMaxMemoryGB        = types.Field("max_memory_gb")
	FieldMaxDiskGB          = types.Field("max_disk_gb")
)
