package run

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// ComputeEffectiveConfigs derives effective config maps from a resolved RunConfig.
// Used by dry-run to show what configs will be applied before the run starts.
func ComputeEffectiveConfigs(cfg *types.RunConfig) map[string]map[string]string {
	out := make(map[string]map[string]string)

	db := cfg.Database

	switch db.Kind {
	case types.DatabaseYDB:
		if db.YDB != nil {
			st := db.YDB.Storage
			diskGB := st.DiskGB
			if diskGB <= 0 {
				diskGB = 80
			}
			memMB := st.MemoryMB
			if memMB <= 0 {
				memMB = 4096
			}
			cpus := st.CPUs
			if cpus <= 0 {
				cpus = 2
			}
			if cfg.MachineOverride != nil {
				if cfg.MachineOverride.DiskGB > 0 {
					diskGB = cfg.MachineOverride.DiskGB
				}
				if cfg.MachineOverride.MemoryMB > 0 {
					memMB = cfg.MachineOverride.MemoryMB
				}
				if cfg.MachineOverride.CPUs > 0 {
					cpus = cfg.MachineOverride.CPUs
				}
			}
			pdiskGB := diskGB - 2
			if pdiskGB < 10 {
				pdiskGB = 10
			}
			hardMB := memMB * 85 / 100
			ft := db.YDB.FaultTolerance
			if ft == "" {
				ft = "none"
			}
			nodes := fmt.Sprintf("%d storage", st.Count)
			if db.YDB.Database != nil {
				nodes += fmt.Sprintf(" + %d compute", db.YDB.Database.Count)
			} else {
				nodes += " (combined)"
			}

			out["database"] = map[string]string{
				"kind":      "ydb",
				"nodes":     nodes,
				"per_node":  fmt.Sprintf("%d vCPU / %d MB / %d GB", cpus, memMB, diskGB),
				"pdisk_gb":  fmt.Sprintf("%d", pdiskGB),
				"mem_limit": fmt.Sprintf("%d MB", hardMB),
				"cpu_count": fmt.Sprintf("%d", cpus),
				"erasure":   ft,
				"db_path":   db.YDB.DatabasePath,
			}
		}

	case types.DatabasePostgres:
		if db.Postgres != nil {
			m := db.Postgres.Master
			ec := map[string]string{
				"kind":    "postgres",
				"version": string(db.Version),
				"master":  fmt.Sprintf("%d× %d vCPU / %d MB / %d GB", m.Count, m.CPUs, m.MemoryMB, m.DiskGB),
			}
			if len(db.Postgres.Replicas) > 0 {
				r := db.Postgres.Replicas[0]
				ec["replicas"] = fmt.Sprintf("%d× %d vCPU / %d MB", r.Count, r.CPUs, r.MemoryMB)
			}
			if db.Postgres.Patroni {
				ec["ha"] = "patroni + etcd"
			}
			if db.Postgres.PgBouncer {
				ec["pooler"] = "pgbouncer"
			}
			for k, v := range db.Postgres.MasterOptions {
				ec[k] = v
			}
			out["database"] = ec
		}

	case types.DatabaseMySQL, types.DatabaseMariaDB:
		t := db.MySQL
		kindLabel := "mysql"
		if db.Kind == types.DatabaseMariaDB {
			t = db.MariaDB
			kindLabel = "mariadb"
		}
		if t != nil {
			p := t.Primary
			ec := map[string]string{
				"kind":    kindLabel,
				"version": string(db.Version),
				"primary": fmt.Sprintf("%d× %d vCPU / %d MB / %d GB", p.Count, p.CPUs, p.MemoryMB, p.DiskGB),
			}
			if len(t.Replicas) > 0 {
				r := t.Replicas[0]
				ec["replicas"] = fmt.Sprintf("%d× %d vCPU / %d MB", r.Count, r.CPUs, r.MemoryMB)
			}
			if t.GroupRepl {
				ec["replication"] = "group"
			} else if t.SemiSync {
				ec["replication"] = "semi-sync"
			}
			for k, v := range t.PrimaryOptions {
				ec[k] = v
			}
			out["database"] = ec
		}

	case types.DatabasePicodata:
		if db.Picodata != nil {
			ec := map[string]string{
				"kind":      "picodata",
				"instances": fmt.Sprintf("%d", len(db.Picodata.Instances)),
				"shards":    fmt.Sprintf("%d", db.Picodata.Shards),
				"rf":        fmt.Sprintf("%d", db.Picodata.Replication),
			}
			if len(db.Picodata.Instances) > 0 {
				inst := db.Picodata.Instances[0]
				ec["per_node"] = fmt.Sprintf("%d vCPU / %d MB / %d GB", inst.CPUs, inst.MemoryMB, inst.DiskGB)
			}
			for k, v := range db.Picodata.InstanceOptions {
				ec[k] = v
			}
			out["database"] = ec
		}

	case types.DatabaseCockroach:
		if db.Cockroach != nil {
			n := db.Cockroach.Nodes
			ec := map[string]string{
				"kind":    "cockroach",
				"version": string(db.Version),
				"nodes":   fmt.Sprintf("%d× %d vCPU / %d MB / %d GB", n.Count, n.CPUs, n.MemoryMB, n.DiskGB),
			}
			for k, v := range db.Cockroach.Options {
				ec[k] = v
			}
			out["database"] = ec
		}
	}

	// Benchmark
	s := cfg.Stroppy
	bench := map[string]string{}
	if s.Script != "" {
		bench["script"] = s.Script
	}
	if s.SQL != "" {
		bench["sql"] = s.SQL
	}
	if s.Version != "" {
		bench["stroppy"] = "v" + s.Version
	}
	k6Mode := s.K6Mode
	if k6Mode == "" {
		k6Mode = "duration"
	}
	bench["k6 mode"] = k6Mode
	if k6Mode == "duration" && s.Duration != "" {
		bench["duration"] = s.Duration
	}
	if k6Mode == "iterations" && s.Iterations > 0 {
		bench["iterations"] = fmt.Sprintf("%d", s.Iterations)
	}
	quiet := true
	if s.Quiet != nil {
		quiet = *s.Quiet
	}
	bench["quiet"] = fmt.Sprintf("%t", quiet)
	if s.NoThresholds {
		bench["thresholds"] = "disabled"
	}
	if s.VUs > 0 {
		bench["VUs"] = fmt.Sprintf("%d", s.VUs)
	}
	if s.PoolSize > 0 {
		bench["pool"] = fmt.Sprintf("%d", s.PoolSize)
	}
	if s.ScaleFactor > 0 {
		bench["scale"] = fmt.Sprintf("%d", s.ScaleFactor)
	}
	if s.DefaultInsertMethod != "" {
		bench["insert"] = s.DefaultInsertMethod
	}
	if s.Machine != nil {
		bench["runner"] = fmt.Sprintf("%d vCPU / %d MB", s.Machine.CPUs, s.Machine.MemoryMB)
	}
	if len(bench) > 0 {
		out["benchmark"] = bench
	}

	// Infrastructure
	infra := map[string]string{
		"provider": string(cfg.Provider),
	}
	if cfg.PlatformID != "" {
		infra["platform"] = cfg.PlatformID
	}
	for _, m := range cfg.Machines {
		infra[string(m.Role)] = fmt.Sprintf("%d× %d vCPU / %d MB / %d GB", m.Count, m.CPUs, m.MemoryMB, m.DiskGB)
	}
	out["infrastructure"] = infra

	return out
}
