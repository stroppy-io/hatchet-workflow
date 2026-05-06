package run

import "github.com/stroppy-io/stroppy-cloud/internal/domain/types"

// Cost mirrors postgres.JobCost — defining it here lets the run package
// estimate without importing infrastructure (avoids the dependency cycle).
// Conversion is structural at the call site.
type Cost struct {
	CPUs        int
	MemoryMB    int
	DiskGB      int
	VMCount     int
	RunsRunning int
}

// EstimateRunCost computes the in-flight resource footprint of a single
// run by summing across every machine the topology will provision plus the
// stroppy runner. Identical to what FillMachinesFromTopology + the
// terraform apply will request from the provider, so a tenant whose limits
// total to N CPUs can run exactly N CPUs worth of in-flight load.
//
// External-DB runs only ever provision the stroppy runner.
func EstimateRunCost(cfg types.RunConfig) Cost {
	work := cfg
	if cfg.ExternalDB == nil {
		FillMachinesFromTopology(&work)
	} else if cfg.Stroppy.Machine != nil {
		// FillMachinesFromTopology already handles ExternalDB by emitting
		// only the stroppy runner; no infra path needed here.
		FillMachinesFromTopology(&work)
	}

	c := Cost{RunsRunning: 1}
	for _, m := range work.Machines {
		count := m.Count
		if count <= 0 {
			count = 1
		}
		c.CPUs += m.CPUs * count
		c.MemoryMB += m.MemoryMB * count
		c.DiskGB += m.DiskGB * count
		// Secondary disks are also paid resources on Yandex.
		for _, sd := range m.SecondaryDisks {
			c.DiskGB += sd.SizeGB * count
		}
		c.VMCount += count
	}

	// Stroppy runner: included implicitly when machines list already
	// contains a "stroppy" role (FillMachinesFromTopology adds it for
	// yandex). On docker the runner is inline so we don't double-count.
	if cfg.Provider == types.ProviderYandex && cfg.Stroppy.Machine != nil {
		stroppyAlreadyIncluded := false
		for _, m := range work.Machines {
			if m.Role == types.RoleStroppy {
				stroppyAlreadyIncluded = true
				break
			}
		}
		if !stroppyAlreadyIncluded {
			sm := cfg.Stroppy.Machine
			cnt := sm.Count
			if cnt <= 0 {
				cnt = 1
			}
			c.CPUs += sm.CPUs * cnt
			c.MemoryMB += sm.MemoryMB * cnt
			c.DiskGB += sm.DiskGB * cnt
			c.VMCount += cnt
		}
	}

	return c
}
