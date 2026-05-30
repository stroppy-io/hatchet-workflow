package providers

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// This file holds the per-machine OVERRIDE vocabulary — the provider-specific
// params a user may override on an abstract machine. It is NOT the tenant
// provider settings (yandex.go/docker.go); those stay fixed at the tenant. The
// wizard layer composes these fields (narrowed to the tenant's catalog) into the
// override schema it emits per (db × provider × role). The chosen values map to
// topology.Instance.provider_parms at bake.

// YandexOverrideFields returns the Yandex per-machine override params, with
// option sets narrowed to the tenant's catalog (disk classes, zones, platform).
// Empty inputs fall back to the full vocabulary.
func YandexOverrideFields(diskCatalog, zones []string, platform string) []schemapb.FieldDef {
	if len(diskCatalog) == 0 {
		diskCatalog = DiskTypeValues
	}
	disk := utils.StrEnum(FieldDiskType, diskCatalog...).Title("Disk type")
	if !contains(diskCatalog, DiskNetworkSSD) {
		disk = disk.Default(diskCatalog[0])
	} else {
		disk = disk.Default(DiskNetworkSSD)
	}

	fields := []schemapb.FieldDef{disk}

	if len(zones) > 0 {
		fields = append(fields, utils.StrEnum(FieldZone, zones...).Default(zones[0]).Title("Zone"))
	} else {
		fields = append(fields, schemapb.Str(FieldZone).Title("Zone"))
	}

	if platform != "" {
		fields = append(fields, utils.StrEnum(FieldPlatformID, platform).Default(platform).Title("Platform"))
	} else {
		fields = append(fields, utils.StrEnum(FieldPlatformID, PlatformIDValues...).
			Default(PlatformStandardV3).Title("Platform"))
	}

	fields = append(fields, utils.StrEnum(FieldNetworkAccel, NetAccelValues...).
		Default(NetAccelStandard).Title("Network acceleration"))

	return fields
}

// DockerOverrideFields returns the Docker per-machine override params. Docker
// containers have no zone/disk-class/platform, so there is nothing to override.
func DockerOverrideFields() []schemapb.FieldDef { return nil }

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
