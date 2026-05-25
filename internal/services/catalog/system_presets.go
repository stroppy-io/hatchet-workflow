package catalog

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	renderpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/render"
)

// SystemPreset is a named platform-seeded database preset.
type SystemPreset struct {
	Name        string
	Description string
	DB          *domain.DatabasePreset
}

// SystemWorkloadPreset is a named platform-seeded workload preset (KIND_WORKLOAD,
// is_system) — the workload-side analogue of SystemPreset.
type SystemWorkloadPreset struct {
	Name        string
	Description string
	WL          *domain.WorkloadPreset
}

// SystemWorkloadPresets returns the built-in WORKLOAD presets seeded at server
// start: every stroppy benchmark script (tpcc / tpcb / tpch, in both the stored-proc
// and the typescript-transaction flavor where the script has both) at four sizing
// tiers (smoke → small → medium → large). Read-only catalog entries visible to every
// tenant, mirroring SystemDatabasePresets.
//
// Each preset carries the Workload (script + sizing) + a stroppy load machine. The
// protocol is left UNSPECIFIED on purpose — it is DB-specific and gets set when the
// wizard assembles the workload against a chosen DatabasePreset (AssembleFromPresets),
// the same point the stroppy→database FLOW edge is wired. Pure data: no DB, no I/O.
func SystemWorkloadPresets() []SystemWorkloadPreset {
	type scriptDef struct{ key, script, label string }
	scripts := []scriptDef{
		{"tpcc-procs", "tpcc/procs", "TPC-C procs"},
		{"tpcc-tx", "tpcc/tx.ts", "TPC-C tx"},
		{"tpcb-procs", "tpcb/procs", "TPC-B procs"},
		{"tpcb-tx", "tpcb/tx.ts", "TPC-B tx"},
		{"tpch-tx", "tpch/tx.ts", "TPC-H"},
	}
	type sizeDef struct {
		name             string
		scale, tpchScale float64
		vus, pool        uint32
		iterations       uint32 // when duration == ""
		duration         string
	}
	sizes := []sizeDef{
		{"smoke", 1, 0.1, 1, 4, 1, ""},
		{"small", 1, 1, 4, 8, 0, "1m"},
		{"medium", 10, 3, 16, 32, 0, "5m"},
		{"large", 50, 10, 64, 128, 0, "15m"},
	}
	var out []SystemWorkloadPreset
	for _, s := range scripts {
		for _, z := range sizes {
			scale := z.scale
			if strings.HasPrefix(s.key, "tpch") {
				scale = z.tpchScale
			}
			exec := &domain.Workload_Execution{Vus: z.vus, Quiet: true, NoThresholds: true}
			if z.duration != "" {
				exec.Limit = &domain.Workload_Execution_Duration{Duration: z.duration}
			} else {
				exec.Limit = &domain.Workload_Execution_Iterations{Iterations: z.iterations}
			}
			limit := z.duration
			if limit == "" {
				limit = fmt.Sprintf("%d iters", z.iterations)
			}
			out = append(out, SystemWorkloadPreset{
				Name:        fmt.Sprintf("%s — %s", s.label, z.name),
				Description: fmt.Sprintf("%s, %s sizing (scale %g, %d VUs, %s)", s.label, z.name, scale, z.vus, limit),
				WL:          workloadPreset(s.script, scale, z.pool, exec),
			})
		}
	}
	return out
}

// workloadPreset builds a WorkloadPreset: the Workload (script + sizing) plus a single
// stroppy load machine. No stroppy→database FLOW connection (the target component is
// DB-specific — added on assembly); a SUPPORT self-edge satisfies the topology's
// min-one-connection rule and is inert to the dag builder.
func workloadPreset(script string, scale float64, pool uint32, exec *domain.Workload_Execution) *domain.WorkloadPreset {
	return &domain.WorkloadPreset{
		Workload: &domain.Workload{
			StroppyVersion: "v5.1.3",
			Script:         script,
			Protocol:       domain.Workload_PROTOCOL_UNSPECIFIED,
			Parameters:     &domain.Workload_Parameters{PoolSize: pool, ScaleFactor: scale},
			Execution:      exec,
		},
		Topology: &domain.Topology{
			Machines: []*domain.Topology_Machine{
				machine("load1", 2, 4, 20, &domain.Topology_Component{Id: "stroppy", Kind: domain.Topology_Component_KIND_STROPPY, Config: cfg("stroppy")}),
			},
			Connections: []*domain.Topology_Connection{selfConn("stroppy")},
		},
	}
}

// SystemDatabasePresets returns the built-in DATABASE presets seeded at server start.
// They are read-only catalog entries visible to every tenant.
//
// Each preset is INTENT + HARDWARE, lowered to a Topology by dag.CompileTopology:
// the Database carries the structural intent (engine, replication / coordination
// variant, node counts, fault-tolerance) and the dag.Sizing carries the per-role
// machine specs (the Database deliberately holds no sizing). The compiler is the only
// place that reads Database.Options to derive structure; the resulting Topology is the
// IR every downstream layer (render / recipe / deployment) consumes.
//
// Pure data: no DB, no I/O.
func SystemDatabasePresets() []SystemPreset {
	return []SystemPreset{
		// ── PostgreSQL ───────────────────────────────────────────────────────
		{
			Name:        "PostgreSQL Single",
			Description: "Single PostgreSQL instance",
			DB:          compiled(pgSingle()),
		},
		{
			Name:        "PostgreSQL HA",
			Description: "PostgreSQL with Patroni, etcd coordinator and a proxy (synchronous replication)",
			DB:          compiled(pgHA()),
		},
		{
			Name:        "PostgreSQL Scale",
			Description: "PostgreSQL with 4 replicas, 2 proxies, Patroni and etcd",
			DB:          compiled(pgScale()),
		},
		// ── MySQL ────────────────────────────────────────────────────────────
		{
			Name:        "MySQL Single",
			Description: "Single MySQL instance",
			DB:          compiled(mysqlSingle()),
		},
		{
			Name:        "MySQL Replica",
			Description: "MySQL primary/replica with GTID-based asynchronous replication",
			DB:          compiled(mysqlReplica()),
		},
		{
			Name:        "MySQL Group",
			Description: "MySQL Group Replication topology with ProxySQL",
			DB:          compiled(mysqlGroup()),
		},
		// ── MariaDB (mysql-shaped, KIND_MARIADB) ─────────────────────────────
		{
			Name:        "MariaDB Single",
			Description: "Single MariaDB instance",
			DB:          compiled(mariadbSingle()),
		},
		{
			Name:        "MariaDB Replica",
			Description: "MariaDB primary/replica with GTID-based asynchronous replication",
			DB:          compiled(mariadbReplica()),
		},
		{
			Name:        "MariaDB Group",
			Description: "MariaDB Group Replication-shaped topology with ProxySQL",
			DB:          compiled(mariadbGroup()),
		},
		// ── Picodata ─────────────────────────────────────────────────────────
		{
			Name:        "Picodata Single",
			Description: "Single Picodata instance",
			DB:          compiled(picodataSingle()),
		},
		{
			Name:        "Picodata Cluster",
			Description: "Picodata 3-instance cluster (Raft coordination)",
			DB:          compiled(picodataCluster()),
		},
		{
			Name:        "Picodata Scale",
			Description: "Picodata 6-instance multi-tier deployment",
			DB:          compiled(picodataScale()),
		},
		// ── YDB ──────────────────────────────────────────────────────────────
		{
			Name:        "YDB Single",
			Description: "YDB single universal node",
			DB:          compiled(ydbSingle()),
		},
		{
			Name:        "YDB mirror3dc-3x32",
			Description: "Target perf topology: mirror-3-dc, 3×32 vCPU compute, 3×16 vCPU storage, 9 io-m3 pdisks",
			DB:          compiled(ydbMirror3DC("ydb-mirror3dc-3x32", 3, flavor(32, 64, 50))),
		},
		{
			Name:        "YDB mirror3dc-3x64",
			Description: "Target perf topology: mirror-3-dc, 3×64 vCPU compute, 3×16 vCPU storage, 9 io-m3 pdisks",
			DB:          compiled(ydbMirror3DC("ydb-mirror3dc-3x64", 3, flavor(64, 128, 50))),
		},
		{
			Name:        "YDB mirror3dc-9x32",
			Description: "Target perf topology: mirror-3-dc, 9×32 vCPU compute, 3×16 vCPU storage, 9 io-m3 pdisks",
			DB:          compiled(ydbMirror3DC("ydb-mirror3dc-9x32", 9, flavor(32, 64, 50))),
		},
		// ── CockroachDB ──────────────────────────────────────────────────────
		{
			Name:        "CockroachDB Single",
			Description: "Single CockroachDB node — dev / smoke runs",
			DB:          compiled(cockroachSingle()),
		},
		{
			Name:        "CockroachDB Cluster",
			Description: "3-node CockroachDB cluster",
			DB:          compiled(cockroachCluster()),
		},
		{
			Name:        "CockroachDB Scale",
			Description: "6-node CockroachDB cluster",
			DB:          compiled(cockroachScale()),
		},
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// compiled lowers a (Database intent, Sizing) pair into a DatabasePreset via
// dag.CompileTopology. A compile error here is a programmer bug (static, known-good
// presets), caught by TestCompileTopology_MatchesHandAuthored — so it panics.
func compiled(db *domain.Database, size *dag.Sizing) *domain.DatabasePreset {
	topo, err := dag.CompileTopology(db, size)
	if err != nil {
		panic(fmt.Sprintf("catalog: compile system preset topology (%s): %v", db.GetConfig().GetId(), err))
	}
	return &domain.DatabasePreset{Database: db, Topology: topo}
}

// flavor is shorthand for a boot-disk-only machine spec (no secondary data disks).
func flavor(cores uint32, memGb, diskGb uint64) dag.Flavor {
	return dag.Flavor{Cores: cores, MemGb: memGb, DiskGb: diskGb}
}

// cfg builds a non-empty render.Config with the given id. The proto validator
// requires render.Config.Id to be 1..128 runes, so every Database and Component
// config carries an explicit id.
func cfg(id string) *renderpb.Config { return &renderpb.Config{Id: id} }

// selfHostedTarget is the standard provisioned (cloud-creates-it) target.
func selfHostedTarget() *domain.Database_Target {
	return &domain.Database_Target{
		Target: &domain.Database_Target_SelfHosted_{SelfHosted: &domain.Database_Target_SelfHosted{}},
	}
}

// machine builds a sized host carrying the given components (workload topology only;
// database topologies are produced by dag.CompileTopology).
func machine(id string, cores uint32, memGb, diskGb uint64, comps ...*domain.Topology_Component) *domain.Topology_Machine {
	return &domain.Topology_Machine{
		Id: id, Cores: cores, MemoryGb: memGb, DiskGb: diskGb,
		Components: comps,
	}
}

// selfConn satisfies the topology min-1-connection rule for single-component
// topologies without affecting dag compilation (a SUPPORT self-edge is inert).
func selfConn(id string) *domain.Topology_Connection {
	return &domain.Topology_Connection{From: id, To: id, Kind: domain.Topology_Connection_KIND_SUPPORT}
}

// ── PostgreSQL ─────────────────────────────────────────────────────────────

func pgSingle() (*domain.Database, *dag.Sizing) {
	return pgDatabase("postgres-single", domain.Database_Options_Postgres_Replication_MODE_SINGLE, 0, 0),
		&dag.Sizing{Database: flavor(4, 16, 100)}
}

func pgHA() (*domain.Database, *dag.Sizing) {
	return pgDatabase("postgres-ha", domain.Database_Options_Postgres_Replication_MODE_PATRONI, 2, 1),
		&dag.Sizing{Database: flavor(4, 16, 200), Coordinator: flavor(2, 4, 50), Proxy: flavor(2, 4, 50), Proxies: 1}
}

func pgScale() (*domain.Database, *dag.Sizing) {
	return pgDatabase("postgres-scale", domain.Database_Options_Postgres_Replication_MODE_PATRONI, 4, 2),
		&dag.Sizing{Database: flavor(8, 16, 200), Coordinator: flavor(2, 4, 50), Proxy: flavor(2, 4, 50), Proxies: 2}
}

func pgDatabase(configID string, mode domain.Database_Options_Postgres_Replication_Mode, replicas, syncReplicas uint32) *domain.Database {
	return &domain.Database{
		Kind:    domain.Database_KIND_POSTGRES,
		Version: "16",
		Config:  cfg(configID),
		Target:  selfHostedTarget(),
		Options: &domain.Database_Options{Options: &domain.Database_Options_Postgres_{Postgres: &domain.Database_Options_Postgres{
			Replication: &domain.Database_Options_Postgres_Replication{Mode: mode, Replicas: replicas, SyncReplicas: syncReplicas},
		}}},
	}
}

// ── MySQL / MariaDB ─────────────────────────────────────────────────────────

func mysqlSingle() (*domain.Database, *dag.Sizing) {
	return mysqlDatabase(domain.Database_KIND_MYSQL, "mysql-single", "8.4", domain.Database_Options_Mysql_Replication_MODE_SINGLE, 0, false),
		&dag.Sizing{Database: flavor(4, 16, 100)}
}

func mysqlReplica() (*domain.Database, *dag.Sizing) {
	return mysqlDatabase(domain.Database_KIND_MYSQL, "mysql-replica", "8.4", domain.Database_Options_Mysql_Replication_MODE_ASYNC, 2, true),
		&dag.Sizing{Database: flavor(4, 8, 100), Proxy: flavor(2, 4, 50), Proxies: 1}
}

func mysqlGroup() (*domain.Database, *dag.Sizing) {
	return mysqlDatabase(domain.Database_KIND_MYSQL, "mysql-group", "8.4", domain.Database_Options_Mysql_Replication_MODE_GROUP_REPLICATION, 2, true),
		&dag.Sizing{Database: flavor(8, 16, 200), Proxy: flavor(2, 4, 50), Proxies: 2}
}

func mariadbSingle() (*domain.Database, *dag.Sizing) {
	return mysqlDatabase(domain.Database_KIND_MARIADB, "mariadb-single", "11.4", domain.Database_Options_Mysql_Replication_MODE_SINGLE, 0, false),
		&dag.Sizing{Database: flavor(4, 16, 100)}
}

func mariadbReplica() (*domain.Database, *dag.Sizing) {
	return mysqlDatabase(domain.Database_KIND_MARIADB, "mariadb-replica", "11.4", domain.Database_Options_Mysql_Replication_MODE_ASYNC, 2, true),
		&dag.Sizing{Database: flavor(4, 8, 100), Proxy: flavor(2, 4, 50), Proxies: 1}
}

func mariadbGroup() (*domain.Database, *dag.Sizing) {
	return mysqlDatabase(domain.Database_KIND_MARIADB, "mariadb-group", "11.4", domain.Database_Options_Mysql_Replication_MODE_GROUP_REPLICATION, 2, true),
		&dag.Sizing{Database: flavor(8, 16, 200), Proxy: flavor(2, 4, 50), Proxies: 2}
}

func mysqlDatabase(kind domain.Database_Kind, configID, version string, mode domain.Database_Options_Mysql_Replication_Mode, replicas uint32, proxysql bool) *domain.Database {
	mysql := &domain.Database_Options_Mysql{
		Replication: &domain.Database_Options_Mysql_Replication{Mode: mode, Replicas: replicas},
	}
	if proxysql {
		mysql.Access = &domain.Database_Options_Mysql_Access{Proxysql: true}
	}
	return &domain.Database{
		Kind:    kind,
		Version: version,
		Config:  cfg(configID),
		Target:  selfHostedTarget(),
		Options: &domain.Database_Options{Options: &domain.Database_Options_Mysql_{Mysql: mysql}},
	}
}

// ── Picodata ─────────────────────────────────────────────────────────────────

func picodataSingle() (*domain.Database, *dag.Sizing) {
	return picodataDatabase("picodata-single", 1), &dag.Sizing{Database: flavor(4, 8, 100), Instances: 1}
}

func picodataCluster() (*domain.Database, *dag.Sizing) {
	return picodataDatabase("picodata-cluster", 3),
		&dag.Sizing{Database: flavor(4, 8, 100), Proxy: flavor(2, 4, 50), Proxies: 1, Instances: 3}
}

func picodataScale() (*domain.Database, *dag.Sizing) {
	return picodataDatabase("picodata-scale", 6),
		&dag.Sizing{Database: flavor(8, 16, 200), Proxy: flavor(2, 4, 50), Proxies: 2, Instances: 6}
}

func picodataDatabase(configID string, shards uint32) *domain.Database {
	return &domain.Database{
		Kind:    domain.Database_KIND_PICODATA,
		Version: "25.3",
		Config:  cfg(configID),
		Target:  selfHostedTarget(),
		Options: &domain.Database_Options{Options: &domain.Database_Options_Picodata_{Picodata: &domain.Database_Options_Picodata{Shards: shards}}},
	}
}

// ── YDB ────────────────────────────────────────────────────────────────────

func ydbSingle() (*domain.Database, *dag.Sizing) {
	return ydbDatabase("ydb-single", 1, 0, 1, domain.Database_Options_Ydb_SelfHosted_FAULT_TOLERANCE_NONE),
		&dag.Sizing{Database: flavor(8, 32, 200)}
}

// ydbMirror3DC is the canonical YDB mirror-3-dc deployment: a dedicated STORAGE tier
// of 3 nodes, each with 3 × 930 GB io-m3 pdisks (= 9 fail domains across 3 realms, what
// mirror-3-dc needs), plus a COMPUTE tier of computeNodes dynamic database nodes. The
// "<N>x<C>" suffix in the preset name is computeNodes × compute vCPU.
func ydbMirror3DC(configID string, computeNodes uint32, compute dag.Flavor) (*domain.Database, *dag.Sizing) {
	return ydbDatabase(configID, 3, computeNodes, 8, domain.Database_Options_Ydb_SelfHosted_FAULT_TOLERANCE_MIRROR_3_DC),
		&dag.Sizing{
			Storage: dag.Flavor{Cores: 16, MemGb: 32, DiskGb: 50, DataDisksGb: []uint64{930, 930, 930}},
			Compute: compute,
		}
}

func ydbDatabase(configID string, storageNodes, databaseNodes, storageGroups uint32, ft domain.Database_Options_Ydb_SelfHosted_FaultTolerance) *domain.Database {
	return &domain.Database{
		Kind:    domain.Database_KIND_YDB,
		Version: "24.2",
		Config:  cfg(configID),
		Target:  selfHostedTarget(),
		Options: &domain.Database_Options{Options: &domain.Database_Options_Ydb_{Ydb: &domain.Database_Options_Ydb{
			Rollout: &domain.Database_Options_Ydb_SelfHosted_{SelfHosted: &domain.Database_Options_Ydb_SelfHosted{
				StorageNodes:   storageNodes,
				DatabaseNodes:  databaseNodes,
				StorageGroups:  storageGroups,
				FaultTolerance: ft,
				FailureDomain:  domain.Database_Options_Ydb_SelfHosted_FAILURE_DOMAIN_DISK,
			}},
			DatabasePath: "/Root/testdb",
		}}},
	}
}

// ── CockroachDB ──────────────────────────────────────────────────────────────

func cockroachSingle() (*domain.Database, *dag.Sizing) {
	return cockroachDatabase("cockroach-single", 1), &dag.Sizing{Database: flavor(4, 16, 100)}
}

func cockroachCluster() (*domain.Database, *dag.Sizing) {
	return cockroachDatabase("cockroach-cluster", 3), &dag.Sizing{Database: flavor(4, 16, 100)}
}

func cockroachScale() (*domain.Database, *dag.Sizing) {
	return cockroachDatabase("cockroach-scale", 6), &dag.Sizing{Database: flavor(8, 16, 200)}
}

func cockroachDatabase(configID string, nodes uint32) *domain.Database {
	return &domain.Database{
		Kind:    domain.Database_KIND_COCKROACH,
		Version: "24.2",
		Config:  cfg(configID),
		Target:  selfHostedTarget(),
		Options: &domain.Database_Options{Options: &domain.Database_Options_Cockroach_{Cockroach: &domain.Database_Options_Cockroach{
			Nodes:        nodes,
			ZoneReplicas: nodes,
		}}},
	}
}
