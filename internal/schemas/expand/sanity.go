package expand

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/databases/ydb"
	"github.com/stroppy-io/stroppy-cloud/internal/schemas/providers"
)

// Sanity runs the bake-time capacity checks that need BOTH the derived VMs and
// the chosen provider (the few genuinely cross-cutting checks that can't live in
// a single schema). Returns one error per violation; empty = OK.
func Sanity(kind string, db map[string]any, providerKind string, prov map[string]any, vms []VM) []error {
	var errs []error
	switch providerKind {
	case ProviderYandex:
		errs = append(errs, yandexCapacity(prov, vms)...)
	case ProviderDocker:
		errs = append(errs, dockerCapacity(prov, vms)...)
	}
	errs = append(errs, mirror3dcZones(kind, db, providerKind, prov)...)
	return errs
}

// yandexCapacity: each VM must fit the provider's per-node limits, and the node
// count must be within the node quota. A limit of 0 means "unlimited".
func yandexCapacity(prov map[string]any, vms []VM) []error {
	lim := mapObj(prov, providers.FieldLimits)
	maxCores := mapInt(lim, providers.FieldMaxCoresPerNode, 0)
	maxMem := mapInt(lim, providers.FieldMaxMemoryGBPerNode, 0)
	maxDisk := mapInt(lim, providers.FieldMaxDiskGBPerNode, 0)
	maxNodes := mapInt(lim, providers.FieldMaxNodes, 0)

	var errs []error
	for _, vm := range vms {
		if maxCores > 0 && vm.Shape.Cores > maxCores {
			errs = append(errs, fmt.Errorf("vm %q: %d cores exceeds max_cores_per_node %d", vm.Name, vm.Shape.Cores, maxCores))
		}
		if maxMem > 0 && vm.Shape.MemoryGB > maxMem {
			errs = append(errs, fmt.Errorf("vm %q: %dGB memory exceeds max_memory_gb_per_node %d", vm.Name, vm.Shape.MemoryGB, maxMem))
		}
		if maxDisk > 0 && vm.Shape.DiskGB > maxDisk {
			errs = append(errs, fmt.Errorf("vm %q: %dGB disk exceeds max_disk_gb_per_node %d", vm.Name, vm.Shape.DiskGB, maxDisk))
		}
	}
	if maxNodes > 0 && len(vms) > maxNodes {
		errs = append(errs, fmt.Errorf("%d nodes exceeds max_nodes %d", len(vms), maxNodes))
	}
	return errs
}

// dockerCapacity: the SUM of all containers must fit the host limits.
func dockerCapacity(prov map[string]any, vms []VM) []error {
	lim := mapObj(prov, providers.FieldLimits)
	maxCores := mapInt(lim, providers.FieldMaxCores, 0)
	maxMem := mapInt(lim, providers.FieldMaxMemoryGB, 0)
	maxDisk := mapInt(lim, providers.FieldMaxDiskGB, 0)
	maxCt := mapInt(lim, providers.FieldMaxContainers, 0)

	var sumCores, sumMem, sumDisk int
	for _, vm := range vms {
		sumCores += vm.Shape.Cores
		sumMem += vm.Shape.MemoryGB
		sumDisk += vm.Shape.DiskGB
	}
	var errs []error
	if maxCores > 0 && sumCores > maxCores {
		errs = append(errs, fmt.Errorf("total %d cores exceeds host max_cores %d", sumCores, maxCores))
	}
	if maxMem > 0 && sumMem > maxMem {
		errs = append(errs, fmt.Errorf("total %dGB memory exceeds host max_memory_gb %d", sumMem, maxMem))
	}
	if maxDisk > 0 && sumDisk > maxDisk {
		errs = append(errs, fmt.Errorf("total %dGB disk exceeds host max_disk_gb %d", sumDisk, maxDisk))
	}
	if maxCt > 0 && len(vms) > maxCt {
		errs = append(errs, fmt.Errorf("%d containers exceeds max_containers %d", len(vms), maxCt))
	}
	return errs
}

// mirror3dcZones: a YDB mirror-3-dc cluster spreads storage across 3 data
// centers, so it requires the yandex provider with >= 3 available zones.
func mirror3dcZones(kind string, db map[string]any, providerKind string, prov map[string]any) []error {
	if kind != "ydb" {
		return nil
	}
	ft := mapStr(mapObj(db, ydb.FieldStorage), ydb.FieldFaultTolerance, "")
	if ft != ydb.FaultToleranceMirror3DC {
		return nil
	}
	if providerKind != ProviderYandex {
		return []error{fmt.Errorf("mirror-3-dc requires the yandex provider (got %q)", providerKind)}
	}
	if z := mapStrSlice(prov, providers.FieldZones); len(z) < 3 {
		return []error{fmt.Errorf("mirror-3-dc requires >= 3 zones, provider offers %d", len(z))}
	}
	return nil
}
