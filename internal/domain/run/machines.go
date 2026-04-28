package run

import "github.com/stroppy-io/stroppy-cloud/internal/domain/types"

// applyOverride returns the override's value if set and positive, otherwise the original.
func applyOverride(orig int, override *types.MachineSpec, field func(*types.MachineSpec) int) int {
	if override != nil {
		if v := field(override); v > 0 {
			return v
		}
	}
	return orig
}

// BakeMachineOverrideIntoTopology folds cfg.MachineOverride into the database
// topology's per-component MachineSpec fields and clears MachineOverride.
// After this runs, the topology alone reflects the final database-node sizing,
// which lets the dry-run preview (and the review-step textarea) show the
// effective values without any hidden override left behind.
func BakeMachineOverrideIntoTopology(cfg *types.RunConfig) {
	ov := cfg.MachineOverride
	if ov == nil {
		return
	}
	apply := func(m *types.MachineSpec) {
		if ov.CPUs > 0 {
			m.CPUs = ov.CPUs
		}
		if ov.MemoryMB > 0 {
			m.MemoryMB = ov.MemoryMB
		}
		if ov.DiskGB > 0 {
			m.DiskGB = ov.DiskGB
		}
		if ov.DiskType != "" {
			m.DiskType = ov.DiskType
		}
	}
	db := &cfg.Database
	switch db.Kind {
	case types.DatabasePostgres:
		if db.Postgres != nil {
			apply(&db.Postgres.Master)
			for i := range db.Postgres.Replicas {
				apply(&db.Postgres.Replicas[i])
			}
		}
	case types.DatabaseMySQL:
		if db.MySQL != nil {
			apply(&db.MySQL.Primary)
			for i := range db.MySQL.Replicas {
				apply(&db.MySQL.Replicas[i])
			}
		}
	case types.DatabasePicodata:
		if db.Picodata != nil {
			for i := range db.Picodata.Instances {
				apply(&db.Picodata.Instances[i])
			}
		}
	case types.DatabaseYDB:
		if db.YDB != nil {
			apply(&db.YDB.Storage)
			if db.YDB.Database != nil {
				apply(db.YDB.Database)
			}
		}
	}
	cfg.MachineOverride = nil
}

// fillMachinesFromTopology populates cfg.Machines from the database topology
// when the caller (e.g. SPA) did not specify machines explicitly.
// If MachineOverride is set, its CPU/memory/disk values override the preset's
// database node specs (non-database roles like HAProxy are not affected).
func FillMachinesFromTopology(cfg *types.RunConfig) {
	if len(cfg.Machines) > 0 {
		return // user specified machines explicitly
	}

	ov := cfg.MachineOverride
	ovCPU := func(orig int) int { return applyOverride(orig, ov, func(m *types.MachineSpec) int { return m.CPUs }) }
	ovMem := func(orig int) int { return applyOverride(orig, ov, func(m *types.MachineSpec) int { return m.MemoryMB }) }
	ovDisk := func(orig int) int { return applyOverride(orig, ov, func(m *types.MachineSpec) int { return m.DiskGB }) }

	db := cfg.Database
	switch db.Kind {
	case types.DatabasePostgres:
		if db.Postgres != nil {
			dbCount := db.Postgres.Master.Count
			for _, r := range db.Postgres.Replicas {
				dbCount += r.Count
			}
			cfg.Machines = append(cfg.Machines, types.MachineSpec{
				Role: types.RoleDatabase, Count: dbCount,
				CPUs: ovCPU(db.Postgres.Master.CPUs), MemoryMB: ovMem(db.Postgres.Master.MemoryMB), DiskGB: ovDisk(db.Postgres.Master.DiskGB),
				DiskType: db.Postgres.Master.DiskType, SecondaryDisks: db.Postgres.Master.SecondaryDisks,
			})
			if db.Postgres.HAProxy != nil {
				cfg.Machines = append(cfg.Machines, *db.Postgres.HAProxy)
			}
		}
	case types.DatabaseMySQL:
		if db.MySQL != nil {
			dbCount := db.MySQL.Primary.Count
			for _, r := range db.MySQL.Replicas {
				dbCount += r.Count
			}
			cfg.Machines = append(cfg.Machines, types.MachineSpec{
				Role: types.RoleDatabase, Count: dbCount,
				CPUs: ovCPU(db.MySQL.Primary.CPUs), MemoryMB: ovMem(db.MySQL.Primary.MemoryMB), DiskGB: ovDisk(db.MySQL.Primary.DiskGB),
				DiskType: db.MySQL.Primary.DiskType, SecondaryDisks: db.MySQL.Primary.SecondaryDisks,
			})
			if db.MySQL.ProxySQL != nil {
				cfg.Machines = append(cfg.Machines, *db.MySQL.ProxySQL)
			}
		}
	case types.DatabasePicodata:
		if db.Picodata != nil {
			for _, inst := range db.Picodata.Instances {
				cfg.Machines = append(cfg.Machines, types.MachineSpec{
					Role: types.RoleDatabase, Count: inst.Count,
					CPUs: ovCPU(inst.CPUs), MemoryMB: ovMem(inst.MemoryMB), DiskGB: ovDisk(inst.DiskGB),
					DiskType: inst.DiskType, SecondaryDisks: inst.SecondaryDisks,
				})
			}
			if db.Picodata.HAProxy != nil {
				cfg.Machines = append(cfg.Machines, *db.Picodata.HAProxy)
			}
		}
	case types.DatabaseYDB:
		if db.YDB != nil {
			count := db.YDB.Storage.Count
			cpus := db.YDB.Storage.CPUs
			mem := db.YDB.Storage.MemoryMB
			disk := db.YDB.Storage.DiskGB
			if db.YDB.Database != nil {
				if db.YDB.Database.Count > count {
					count = db.YDB.Database.Count
				}
				if db.YDB.Database.CPUs > cpus {
					cpus = db.YDB.Database.CPUs
				}
				if db.YDB.Database.MemoryMB > mem {
					mem = db.YDB.Database.MemoryMB
				}
				if db.YDB.Database.DiskGB > disk {
					disk = db.YDB.Database.DiskGB
				}
			}
			cfg.Machines = append(cfg.Machines, types.MachineSpec{
				Role: types.RoleDatabase, Count: count,
				CPUs: ovCPU(cpus), MemoryMB: ovMem(mem), DiskGB: ovDisk(disk),
				DiskType: db.YDB.Storage.DiskType, SecondaryDisks: db.YDB.Storage.SecondaryDisks,
			})
			if db.YDB.HAProxy != nil {
				cfg.Machines = append(cfg.Machines, *db.YDB.HAProxy)
			}
		}
	}

	// Add stroppy runner — use custom spec if provided, otherwise default.
	stroppySpec := types.MachineSpec{Role: types.RoleStroppy, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 20}
	if cfg.Stroppy.Machine != nil {
		stroppySpec = *cfg.Stroppy.Machine
		stroppySpec.Role = types.RoleStroppy
		stroppySpec.Count = 1
	}
	cfg.Machines = append(cfg.Machines, stroppySpec)
}
