package run

import (
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbconfig"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// BuildRenderedConfigs returns the per-component config files that the agent
// will write on each database node, rendered from the resolved RunConfig.
//
// Keys are stable identifiers shared with DatabaseConfig.RenderedConfigOverrides
// (e.g. "postgresql.conf:master"). The SPA seeds its textareas from this map
// and ships back any user-edited entries under the same keys.
//
// User-supplied overrides win: when a key is present in
// cfg.Database.RenderedConfigOverrides we surface that value verbatim so the
// preview shows what's actually going to land on the box.
func BuildRenderedConfigs(cfg *types.RunConfig) map[string]string {
	out := map[string]string{}
	db := cfg.Database
	overrides := db.RenderedConfigOverrides

	put := func(key, body string) {
		if v, ok := overrides[key]; ok {
			out[key] = v
			return
		}
		out[key] = body
	}

	switch db.Kind {
	case types.DatabasePostgres:
		if db.Postgres == nil {
			return out
		}
		put("postgresql.conf:master", dbconfig.RenderPostgresConf(dbconfig.RenderPostgresConfOpts{
			Version:       db.Version,
			Role:          "master",
			Options:       db.Postgres.MasterOptions,
			Patroni:       db.Postgres.Patroni,
			TotalMemoryMB: db.Postgres.Master.MemoryMB,
		}))
		if len(db.Postgres.Replicas) > 0 {
			r := db.Postgres.Replicas[0]
			put("postgresql.conf:replica", dbconfig.RenderPostgresConf(dbconfig.RenderPostgresConfOpts{
				Version:       db.Version,
				Role:          "replica",
				Options:       db.Postgres.ReplicaOptions,
				Patroni:       db.Postgres.Patroni,
				TotalMemoryMB: r.MemoryMB,
			}))
		}
		put("pg_hba.conf", dbconfig.PostgresPgHbaConf())

		if db.Postgres.Patroni {
			put("patroni.yml", dbconfig.RenderPatroniConf(dbconfig.RenderPatroniConfOpts{
				PGVersion: db.Version,
				SyncMode:  db.Postgres.SyncReplicas > 0,
				SyncCount: db.Postgres.SyncReplicas,
				PGOptions: db.Postgres.MasterOptions,
			}))
		}
		if db.Postgres.PgBouncer {
			put("pgbouncer.ini", dbconfig.RenderPgBouncerConf(dbconfig.RenderPgBouncerConfOpts{}))
		}
		if db.Postgres.HAProxy != nil {
			healthCheck := "tcp"
			patroniPort := 0
			if db.Postgres.Patroni {
				healthCheck = "patroni"
				patroniPort = 8008
			}
			put("haproxy.cfg", dbconfig.RenderHAProxyConf(dbconfig.RenderHAProxyConfOpts{
				WritePort:   5000,
				ReadPort:    5001,
				HealthCheck: healthCheck,
				PatroniPort: patroniPort,
				// Backends are zero-length here; the real list isn't known
				// until provisioning. The user can still see and edit
				// global/defaults/frontend sections; the backend lines come
				// in once the run is live and submission re-enters dry-run.
			}))
		}

	case types.DatabaseMySQL:
		if db.MySQL == nil {
			return out
		}
		put("my.cnf:primary", dbconfig.RenderMySQLConf(dbconfig.RenderMySQLConfOpts{
			Version:       db.Version,
			Role:          "primary",
			SemiSync:      db.MySQL.SemiSync,
			GroupRepl:     db.MySQL.GroupRepl,
			Options:       db.MySQL.PrimaryOptions,
			TotalMemoryMB: db.MySQL.Primary.MemoryMB,
		}))
		if len(db.MySQL.Replicas) > 0 {
			r := db.MySQL.Replicas[0]
			put("my.cnf:replica", dbconfig.RenderMySQLConf(dbconfig.RenderMySQLConfOpts{
				Version:       db.Version,
				Role:          "replica",
				SemiSync:      db.MySQL.SemiSync,
				GroupRepl:     db.MySQL.GroupRepl,
				Options:       db.MySQL.ReplicaOptions,
				TotalMemoryMB: r.MemoryMB,
			}))
		}
		if db.MySQL.ProxySQL != nil {
			// Backend count = primary + (count of each replica spec). The
			// preview placeholder list has to match what task_proxy.go ships
			// at run time — primary first, replicas after.
			backendCount := db.MySQL.Primary.Count
			for _, r := range db.MySQL.Replicas {
				backendCount += r.Count
			}
			put("proxysql.cnf", dbconfig.RenderProxySQLConf(dbconfig.RenderProxySQLConfOpts{
				BackendCount: backendCount,
			}))
		}

	case types.DatabasePicodata:
		if db.Picodata == nil {
			return out
		}
		spec := types.MachineSpec{}
		if len(db.Picodata.Instances) > 0 {
			spec = db.Picodata.Instances[0]
		}
		put("picodata.yaml", dbconfig.RenderPicodataConf(dbconfig.RenderPicodataConfOpts{
			Replication:   db.Picodata.Replication,
			Options:       db.Picodata.InstanceOptions,
			TotalMemoryMB: spec.MemoryMB,
		}))
		if db.Picodata.HAProxy != nil {
			put("haproxy.cfg", dbconfig.RenderHAProxyConf(dbconfig.RenderHAProxyConfOpts{
				WritePort:   4327,
				ReadPort:    4328,
				HealthCheck: "tcp",
			}))
		}

	case types.DatabaseYDB:
		if db.YDB == nil {
			return out
		}
		// When the storage spec attaches a secondary disk, the agent will
		// point YDB's pdisk at the raw device — render the preview to match.
		ydbBlockDevice := ""
		for _, d := range db.YDB.Storage.SecondaryDisks {
			if d.DeviceName != "" {
				ydbBlockDevice = "/dev/disk/by-id/virtio-" + d.DeviceName
				break
			}
		}
		// In combined mode (Database == nil) both ydbd-storage and
		// ydbd-database run on the same box. Each daemon takes 85% of its
		// configured MemoryMB as its hard limit, so passing the full
		// machine memory to both means a combined hard limit of 1.7×
		// physical RAM and a near-certain OOM. Halve the budget so the
		// two together fit; the agent does the same on the live path.
		storageMem := db.YDB.Storage.MemoryMB
		combined := db.YDB.Database == nil
		if combined && storageMem > 0 {
			storageMem /= 2
		}
		put("ydb.yaml:storage", dbconfig.RenderYDBStorageConf(dbconfig.RenderYDBConfOpts{
			HostCount:       db.YDB.Storage.Count,
			DiskPath:        "/ydb_data",
			BlockDevicePath: ydbBlockDevice,
			CPUs:            db.YDB.Storage.CPUs,
			MemoryMB:        storageMem,
			FaultTolerance:  db.YDB.FaultTolerance,
		}))
		// Database (dynamic) node config: separate file the agent writes to
		// /opt/ydb/cfg/database.yaml. Same cluster topology as the storage
		// yaml but actor-system / memory hints come from the database spec
		// when the topology is split, otherwise mirror the (halved) storage spec.
		dbCPUs := db.YDB.Storage.CPUs
		dbMem := storageMem
		if db.YDB.Database != nil {
			dbCPUs = db.YDB.Database.CPUs
			dbMem = db.YDB.Database.MemoryMB
		}
		put("ydb.yaml:database", dbconfig.RenderYDBDatabaseConf(dbconfig.RenderYDBDatabaseConfOpts{
			HostCount:       db.YDB.Storage.Count,
			DiskPath:        "/ydb_data",
			BlockDevicePath: ydbBlockDevice,
			CPUs:            dbCPUs,
			MemoryMB:        dbMem,
			FaultTolerance:  db.YDB.FaultTolerance,
		}))
		if db.YDB.HAProxy != nil {
			put("haproxy.cfg", dbconfig.RenderHAProxyConf(dbconfig.RenderHAProxyConfOpts{
				WritePort:   2136,
				ReadPort:    2137,
				HealthCheck: "tcp",
			}))
		}
	}

	return out
}
