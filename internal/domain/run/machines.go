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
	case types.DatabaseMySQL, types.DatabaseMariaDB:
		t := db.MySQL
		if db.Kind == types.DatabaseMariaDB {
			t = db.MariaDB
		}
		if t != nil {
			apply(&t.Primary)
			for i := range t.Replicas {
				apply(&t.Replicas[i])
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
	case types.DatabaseCockroach:
		if db.Cockroach != nil {
			apply(&db.Cockroach.Nodes)
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
	ovMem := func(orig int) int {
		return applyOverride(orig, ov, func(m *types.MachineSpec) int { return m.MemoryMB })
	}
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
	case types.DatabaseMySQL, types.DatabaseMariaDB:
		// MariaDB shares MySQL's topology shape; pick whichever pointer is set.
		t := db.MySQL
		if db.Kind == types.DatabaseMariaDB {
			t = db.MariaDB
		}
		if t != nil {
			dbCount := t.Primary.Count
			for _, r := range t.Replicas {
				dbCount += r.Count
			}
			cfg.Machines = append(cfg.Machines, types.MachineSpec{
				Role: types.RoleDatabase, Count: dbCount,
				CPUs: ovCPU(t.Primary.CPUs), MemoryMB: ovMem(t.Primary.MemoryMB), DiskGB: ovDisk(t.Primary.DiskGB),
				DiskType: t.Primary.DiskType, SecondaryDisks: t.Primary.SecondaryDisks,
			})
			if t.ProxySQL != nil {
				cfg.Machines = append(cfg.Machines, *t.ProxySQL)
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
			// In split mode emit the dynamic (compute) nodes first so the
			// downstream "first dbTarget" plumbing — used to set the SQL
			// endpoint stroppy connects to — picks a compute node, not a
			// storage one. In combined mode (Database == nil) only storage
			// nodes exist; the database daemon runs co-located on them.
			if db.YDB.Database != nil {
				d := *db.YDB.Database
				cfg.Machines = append(cfg.Machines, types.MachineSpec{
					Role: types.RoleYDBDatabase, Count: d.Count,
					CPUs: ovCPU(d.CPUs), MemoryMB: ovMem(d.MemoryMB), DiskGB: ovDisk(d.DiskGB),
					DiskType: d.DiskType, SecondaryDisks: d.SecondaryDisks,
				})
			}
			s := db.YDB.Storage
			cfg.Machines = append(cfg.Machines, types.MachineSpec{
				Role: types.RoleYDBStorage, Count: s.Count,
				CPUs: ovCPU(s.CPUs), MemoryMB: ovMem(s.MemoryMB), DiskGB: ovDisk(s.DiskGB),
				DiskType: s.DiskType, SecondaryDisks: s.SecondaryDisks,
			})
			if db.YDB.HAProxy != nil {
				cfg.Machines = append(cfg.Machines, *db.YDB.HAProxy)
			}
		}
	case types.DatabaseCockroach:
		if db.Cockroach != nil {
			n := db.Cockroach.Nodes
			cfg.Machines = append(cfg.Machines, types.MachineSpec{
				Role: types.RoleDatabase, Count: n.Count,
				CPUs: ovCPU(n.CPUs), MemoryMB: ovMem(n.MemoryMB), DiskGB: ovDisk(n.DiskGB),
				DiskType: n.DiskType, SecondaryDisks: n.SecondaryDisks,
			})
		}
	case types.DatabaseYDBManaged:
		// Managed YDB has no DB-side machines — YC manages the database
		// itself. The runner VM is added below as the stroppy machine, but
		// we override its spec from the topology so the user can size the
		// client (the typical knob for managed loads is "how big a client
		// to drive load from").
		if db.YDBManaged != nil && db.YDBManaged.Client.CPUs > 0 && cfg.Stroppy.Machine == nil {
			c := db.YDBManaged.Client
			cfg.Stroppy.Machine = &types.MachineSpec{
				Role: types.RoleStroppy, Count: 1,
				CPUs: ovCPU(c.CPUs), MemoryMB: ovMem(c.MemoryMB), DiskGB: ovDisk(c.DiskGB),
				DiskType: c.DiskType,
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
