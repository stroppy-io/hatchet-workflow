package catalog

import (
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	renderpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/render"
)

// SystemPreset is a named platform-seeded database preset.
type SystemPreset struct {
	Name        string
	Description string
	DB          *domain.DatabasePreset
}

// SystemDatabasePresets returns the built-in DATABASE presets seeded at server
// start. They are read-only catalog entries visible to every tenant.
//
// The structures here mirror the new-structure topologies already proven by the
// integration + dag tests (test/integration/*_pipeline_test.go,
// internal/domain/dag/compile_test.go, authoring_test.go): one Topology_Machine
// per host, each carrying typed Topology_Components (DATABASE / COORDINATOR /
// PROXY / STROPPY) wired by Topology_Connections whose From/To reference those
// component ids. Every Database carries the Config/Target/Options trio the proto
// validator requires; HA postgres sets Replication.Mode = MODE_PATRONI.
//
// Pure data: no DB, no I/O.
func SystemDatabasePresets() []SystemPreset {
	return []SystemPreset{
		// ── PostgreSQL ───────────────────────────────────────────────────────
		{
			Name:        "PostgreSQL Single",
			Description: "Single PostgreSQL instance",
			DB:          pgSingle(),
		},
		{
			Name:        "PostgreSQL HA",
			Description: "PostgreSQL with Patroni, etcd coordinator and a proxy (synchronous replication)",
			DB:          pgHA(),
		},
		{
			Name:        "PostgreSQL Scale",
			Description: "PostgreSQL with 4 replicas, 2 proxies, Patroni and etcd",
			DB:          pgScale(),
		},
		// ── MySQL ────────────────────────────────────────────────────────────
		{
			Name:        "MySQL Single",
			Description: "Single MySQL instance",
			DB:          mysqlSingle(),
		},
		{
			Name:        "MySQL Replica",
			Description: "MySQL primary/replica with GTID-based asynchronous replication",
			DB:          mysqlReplica(),
		},
		{
			Name:        "MySQL Group",
			Description: "MySQL Group Replication topology with ProxySQL",
			DB:          mysqlGroup(),
		},
		// ── MariaDB (mysql-shaped, KIND_MARIADB) ─────────────────────────────
		{
			Name:        "MariaDB Single",
			Description: "Single MariaDB instance",
			DB:          mariadbSingle(),
		},
		{
			Name:        "MariaDB Replica",
			Description: "MariaDB primary/replica with GTID-based asynchronous replication",
			DB:          mariadbReplica(),
		},
		{
			Name:        "MariaDB Group",
			Description: "MariaDB Group Replication-shaped topology with ProxySQL",
			DB:          mariadbGroup(),
		},
		// ── Picodata ─────────────────────────────────────────────────────────
		{
			Name:        "Picodata Single",
			Description: "Single Picodata instance",
			DB:          picodataSingle(),
		},
		{
			Name:        "Picodata Cluster",
			Description: "Picodata 3-instance cluster (Raft coordination)",
			DB:          picodataCluster(),
		},
		{
			Name:        "Picodata Scale",
			Description: "Picodata 6-instance multi-tier deployment",
			DB:          picodataScale(),
		},
		// ── YDB ──────────────────────────────────────────────────────────────
		{
			Name:        "YDB Single",
			Description: "YDB single universal node",
			DB:          ydbSingle(),
		},
		{
			Name:        "YDB Cluster",
			Description: "YDB 3-node storage cluster (mirror-3-dc fault tolerance)",
			DB:          ydbCluster(),
		},
		// ── CockroachDB ──────────────────────────────────────────────────────
		{
			Name:        "CockroachDB Single",
			Description: "Single CockroachDB node — dev / smoke runs",
			DB:          cockroachSingle(),
		},
		{
			Name:        "CockroachDB Cluster",
			Description: "3-node CockroachDB cluster",
			DB:          cockroachCluster(),
		},
		{
			Name:        "CockroachDB Scale",
			Description: "6-node CockroachDB cluster",
			DB:          cockroachScale(),
		},
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

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

// dbComponent / coordinatorComponent / proxyComponent build a typed topology
// component with its required render config.
func dbComponent(id string) *domain.Topology_Component {
	return &domain.Topology_Component{Id: id, Kind: domain.Topology_Component_KIND_DATABASE, Config: cfg(id)}
}

func coordinatorComponent(id string) *domain.Topology_Component {
	return &domain.Topology_Component{Id: id, Kind: domain.Topology_Component_KIND_COORDINATOR, Config: cfg(id)}
}

func proxyComponent(id string) *domain.Topology_Component {
	return &domain.Topology_Component{Id: id, Kind: domain.Topology_Component_KIND_PROXY, Config: cfg(id)}
}

// machine builds a sized host carrying the given components.
func machine(id string, cores uint32, memGb, diskGb uint64, comps ...*domain.Topology_Component) *domain.Topology_Machine {
	return &domain.Topology_Machine{
		Id: id, Cores: cores, MemoryGb: memGb, DiskGb: diskGb,
		Components: comps,
	}
}

// selfConn satisfies the topology min-1-connection rule for single-node presets
// without affecting dag compilation: the dag builder only reads REPLICATION /
// COORDINATION connections, so a SUPPORT self-edge is inert.
func selfConn(id string) *domain.Topology_Connection {
	return &domain.Topology_Connection{From: id, To: id, Kind: domain.Topology_Connection_KIND_SUPPORT}
}

// ── PostgreSQL ─────────────────────────────────────────────────────────────

func pgSingle() *domain.DatabasePreset {
	return &domain.DatabasePreset{
		Database: &domain.Database{
			Kind:    domain.Database_KIND_POSTGRES,
			Version: "16",
			Config:  cfg("postgres-single"),
			Target:  selfHostedTarget(),
			Options: &domain.Database_Options{Options: &domain.Database_Options_Postgres_{Postgres: &domain.Database_Options_Postgres{
				Replication: &domain.Database_Options_Postgres_Replication{
					Mode: domain.Database_Options_Postgres_Replication_MODE_SINGLE,
				},
			}}},
		},
		Topology: &domain.Topology{
			Machines: []*domain.Topology_Machine{
				machine("m-db1", 4, 16, 100, dbComponent("pg")),
			},
			Connections: []*domain.Topology_Connection{selfConn("pg")},
		},
	}
}

// pgHA mirrors compile_test.go TestGraphCompilesHATopology / ha_pipeline_test.go:
// etcd COORDINATOR + 2 patroni DATABASE nodes + a PROXY, wired by
// COORDINATION / REPLICATION / PROXY connections.
func pgHA() *domain.DatabasePreset {
	return pgPatroni("postgres-ha", 2, 1, 4, 16, 200, 1)
}

func pgScale() *domain.DatabasePreset {
	return pgPatroni("postgres-scale", 4, 2, 8, 16, 200, 2)
}

func pgPatroni(configID string, replicas, proxies int, dbCores uint32, dbMemGb, dbDiskGb uint64, syncReplicas uint32) *domain.DatabasePreset {
	machines := []*domain.Topology_Machine{machine("m-etcd", 2, 4, 50, coordinatorComponent("etcd1"))}
	conns := []*domain.Topology_Connection{}
	dbIDs := make([]string, 0, replicas+1)
	for i := 0; i <= replicas; i++ {
		id := "db" + string(rune('1'+i))
		dbIDs = append(dbIDs, id)
		machines = append(machines, machine("m-"+id, dbCores, dbMemGb, dbDiskGb, dbComponent(id)))
		conns = append(conns, &domain.Topology_Connection{From: id, To: "etcd1", Kind: domain.Topology_Connection_KIND_COORDINATION})
		if i > 0 {
			conns = append(conns, &domain.Topology_Connection{From: dbIDs[0], To: id, Kind: domain.Topology_Connection_KIND_REPLICATION})
		}
	}
	for i := 0; i < proxies; i++ {
		id := "px" + string(rune('1'+i))
		machines = append(machines, machine("m-"+id, 2, 4, 50, proxyComponent(id)))
		conns = append(conns, &domain.Topology_Connection{From: id, To: dbIDs[0], Kind: domain.Topology_Connection_KIND_PROXY})
	}
	return &domain.DatabasePreset{
		Database: &domain.Database{
			Kind:    domain.Database_KIND_POSTGRES,
			Version: "16",
			Config:  cfg(configID),
			Target:  selfHostedTarget(),
			Options: &domain.Database_Options{Options: &domain.Database_Options_Postgres_{Postgres: &domain.Database_Options_Postgres{
				Replication: &domain.Database_Options_Postgres_Replication{
					Mode:         domain.Database_Options_Postgres_Replication_MODE_PATRONI,
					Replicas:     uint32(replicas),
					SyncReplicas: syncReplicas,
				},
			}}},
		},
		Topology: &domain.Topology{Machines: machines, Connections: conns},
	}
}

// ── MySQL / MariaDB ─────────────────────────────────────────────────────────

func mysqlSingle() *domain.DatabasePreset {
	return &domain.DatabasePreset{
		Database: &domain.Database{
			Kind:    domain.Database_KIND_MYSQL,
			Version: "8.4",
			Config:  cfg("mysql-single"),
			Target:  selfHostedTarget(),
			Options: &domain.Database_Options{Options: &domain.Database_Options_Mysql_{Mysql: &domain.Database_Options_Mysql{
				Replication: &domain.Database_Options_Mysql_Replication{
					Mode: domain.Database_Options_Mysql_Replication_MODE_SINGLE,
				},
			}}},
		},
		Topology: &domain.Topology{
			Machines: []*domain.Topology_Machine{
				machine("m-db1", 4, 16, 100, dbComponent("db1")),
			},
			Connections: []*domain.Topology_Connection{selfConn("db1")},
		},
	}
}

// mysqlReplica mirrors systemd_pipeline_test.go mysqlReplicationPipeline:
// 2 DATABASE nodes wired by a REPLICATION connection (primary -> replica).
func mysqlReplica() *domain.DatabasePreset {
	return mysqlMulti(domain.Database_KIND_MYSQL, "mysql-replica", "8.4", domain.Database_Options_Mysql_Replication_MODE_ASYNC, 2, 1, 4, 8, 100)
}

func mysqlGroup() *domain.DatabasePreset {
	return mysqlMulti(domain.Database_KIND_MYSQL, "mysql-group", "8.4", domain.Database_Options_Mysql_Replication_MODE_GROUP_REPLICATION, 2, 2, 8, 16, 200)
}

func mariadbReplica() *domain.DatabasePreset {
	return mysqlMulti(domain.Database_KIND_MARIADB, "mariadb-replica", "11.4", domain.Database_Options_Mysql_Replication_MODE_ASYNC, 2, 1, 4, 8, 100)
}

func mariadbGroup() *domain.DatabasePreset {
	return mysqlMulti(domain.Database_KIND_MARIADB, "mariadb-group", "11.4", domain.Database_Options_Mysql_Replication_MODE_GROUP_REPLICATION, 2, 2, 8, 16, 200)
}

func mysqlMulti(kind domain.Database_Kind, configID, version string, mode domain.Database_Options_Mysql_Replication_Mode, replicas, proxies int, dbCores uint32, dbMemGb, dbDiskGb uint64) *domain.DatabasePreset {
	machines := []*domain.Topology_Machine{machine("m-db1", dbCores, dbMemGb, dbDiskGb, dbComponent("db1"))}
	conns := []*domain.Topology_Connection{}
	for i := 0; i < replicas; i++ {
		id := "db" + string(rune('2'+i))
		machines = append(machines, machine("m-"+id, dbCores, dbMemGb, dbDiskGb, dbComponent(id)))
		conns = append(conns, &domain.Topology_Connection{From: "db1", To: id, Kind: domain.Topology_Connection_KIND_REPLICATION})
	}
	for i := 0; i < proxies; i++ {
		id := "px" + string(rune('1'+i))
		machines = append(machines, machine("m-"+id, 2, 4, 50, proxyComponent(id)))
		conns = append(conns, &domain.Topology_Connection{From: id, To: "db1", Kind: domain.Topology_Connection_KIND_PROXY})
	}
	return &domain.DatabasePreset{
		Database: &domain.Database{
			Kind:    kind,
			Version: version,
			Config:  cfg(configID),
			Target:  selfHostedTarget(),
			Options: &domain.Database_Options{Options: &domain.Database_Options_Mysql_{Mysql: &domain.Database_Options_Mysql{
				Replication: &domain.Database_Options_Mysql_Replication{
					Mode:     mode,
					Replicas: uint32(replicas),
				},
				Access: &domain.Database_Options_Mysql_Access{Proxysql: proxies > 0},
			}}},
		},
		Topology: &domain.Topology{Machines: machines, Connections: conns},
	}
}

// mariadbSingle reuses the MySQL-shaped options under KIND_MARIADB — the agent's
// my.cnf writer produces a config mariadb-server reads unmodified.
func mariadbSingle() *domain.DatabasePreset {
	return &domain.DatabasePreset{
		Database: &domain.Database{
			Kind:    domain.Database_KIND_MARIADB,
			Version: "11.4",
			Config:  cfg("mariadb-single"),
			Target:  selfHostedTarget(),
			Options: &domain.Database_Options{Options: &domain.Database_Options_Mysql_{Mysql: &domain.Database_Options_Mysql{
				Replication: &domain.Database_Options_Mysql_Replication{
					Mode: domain.Database_Options_Mysql_Replication_MODE_SINGLE,
				},
			}}},
		},
		Topology: &domain.Topology{
			Machines: []*domain.Topology_Machine{
				machine("m-db1", 4, 16, 100, dbComponent("db1")),
			},
			Connections: []*domain.Topology_Connection{selfConn("db1")},
		},
	}
}

// ── Picodata ─────────────────────────────────────────────────────────────────

func picodataSingle() *domain.DatabasePreset {
	return &domain.DatabasePreset{
		Database: &domain.Database{
			Kind:    domain.Database_KIND_PICODATA,
			Version: "25.3",
			Config:  cfg("picodata-single"),
			Target:  selfHostedTarget(),
			Options: &domain.Database_Options{Options: &domain.Database_Options_Picodata_{Picodata: &domain.Database_Options_Picodata{
				Shards: 1,
			}}},
		},
		Topology: &domain.Topology{
			Machines: []*domain.Topology_Machine{
				machine("m-pd1", 4, 8, 100, dbComponent("pd1")),
			},
			Connections: []*domain.Topology_Connection{selfConn("pd1")},
		},
	}
}

// picodataCluster mirrors ha_pipeline_test.go TestPicodataClusterFullPipeline:
// 2 DATABASE instances wired by a COORDINATION connection (Raft peers).
func picodataCluster() *domain.DatabasePreset {
	return picodataMulti("picodata-cluster", 3, 1, 4, 8, 100, 3)
}

func picodataScale() *domain.DatabasePreset {
	return picodataMulti("picodata-scale", 6, 2, 8, 16, 200, 6)
}

func picodataMulti(configID string, instances, proxies int, dbCores uint32, dbMemGb, dbDiskGb uint64, shards uint32) *domain.DatabasePreset {
	machines := make([]*domain.Topology_Machine, 0, instances+proxies)
	conns := make([]*domain.Topology_Connection, 0, instances+proxies)
	for i := 0; i < instances; i++ {
		id := "pd" + string(rune('1'+i))
		machines = append(machines, machine("m-"+id, dbCores, dbMemGb, dbDiskGb, dbComponent(id)))
		if i > 0 {
			conns = append(conns, &domain.Topology_Connection{From: id, To: "pd1", Kind: domain.Topology_Connection_KIND_COORDINATION})
		}
	}
	for i := 0; i < proxies; i++ {
		id := "px" + string(rune('1'+i))
		machines = append(machines, machine("m-"+id, 2, 4, 50, proxyComponent(id)))
		conns = append(conns, &domain.Topology_Connection{From: id, To: "pd1", Kind: domain.Topology_Connection_KIND_PROXY})
	}
	return &domain.DatabasePreset{
		Database: &domain.Database{
			Kind:    domain.Database_KIND_PICODATA,
			Version: "25.3",
			Config:  cfg(configID),
			Target:  selfHostedTarget(),
			Options: &domain.Database_Options{Options: &domain.Database_Options_Picodata_{Picodata: &domain.Database_Options_Picodata{
				Shards: shards,
			}}},
		},
		Topology: &domain.Topology{Machines: machines, Connections: conns},
	}
}

// ── YDB ────────────────────────────────────────────────────────────────────

// ydbCluster mirrors ha_pipeline_test.go TestYDBClusterFullPipeline: 3 storage
// DATABASE nodes, peers wired by COORDINATION connections to the first node.
func ydbSingle() *domain.DatabasePreset {
	return &domain.DatabasePreset{
		Database: &domain.Database{
			Kind:    domain.Database_KIND_YDB,
			Version: "24.2",
			Config:  cfg("ydb-single"),
			Target:  selfHostedTarget(),
			Options: &domain.Database_Options{Options: &domain.Database_Options_Ydb_{Ydb: &domain.Database_Options_Ydb{
				Rollout: &domain.Database_Options_Ydb_SelfHosted_{SelfHosted: &domain.Database_Options_Ydb_SelfHosted{
					StorageNodes:   1,
					StorageGroups:  1,
					FaultTolerance: domain.Database_Options_Ydb_SelfHosted_FAULT_TOLERANCE_NONE,
					FailureDomain:  domain.Database_Options_Ydb_SelfHosted_FAILURE_DOMAIN_DISK,
				}},
				DatabasePath: "/Root/testdb",
			}}},
		},
		Topology: &domain.Topology{
			Machines:    []*domain.Topology_Machine{machine("m-ydb1", 8, 32, 200, dbComponent("ydb1"))},
			Connections: []*domain.Topology_Connection{selfConn("ydb1")},
		},
	}
}

func ydbCluster() *domain.DatabasePreset {
	machines := make([]*domain.Topology_Machine, 0, 3)
	conns := make([]*domain.Topology_Connection, 0, 2)
	ids := []string{"ydb1", "ydb2", "ydb3"}
	for i, id := range ids {
		machines = append(machines, machine("m-"+id, 8, 32, 200, dbComponent(id)))
		if i > 0 {
			conns = append(conns, &domain.Topology_Connection{
				From: id, To: ids[0], Kind: domain.Topology_Connection_KIND_COORDINATION,
			})
		}
	}
	return &domain.DatabasePreset{
		Database: &domain.Database{
			Kind:    domain.Database_KIND_YDB,
			Version: "24.2",
			Config:  cfg("ydb-cluster"),
			Target:  selfHostedTarget(),
			Options: &domain.Database_Options{Options: &domain.Database_Options_Ydb_{Ydb: &domain.Database_Options_Ydb{
				Rollout: &domain.Database_Options_Ydb_SelfHosted_{SelfHosted: &domain.Database_Options_Ydb_SelfHosted{
					StorageNodes:   3,
					StorageGroups:  8,
					FaultTolerance: domain.Database_Options_Ydb_SelfHosted_FAULT_TOLERANCE_MIRROR_3_DC,
					FailureDomain:  domain.Database_Options_Ydb_SelfHosted_FAILURE_DOMAIN_DISK,
				}},
				DatabasePath: "/Root/testdb",
			}}},
		},
		Topology: &domain.Topology{Machines: machines, Connections: conns},
	}
}

// ── CockroachDB ──────────────────────────────────────────────────────────────

func cockroachSingle() *domain.DatabasePreset {
	return &domain.DatabasePreset{
		Database: &domain.Database{
			Kind:    domain.Database_KIND_COCKROACH,
			Version: "24.2",
			Config:  cfg("cockroach-single"),
			Target:  selfHostedTarget(),
			Options: &domain.Database_Options{Options: &domain.Database_Options_Cockroach_{Cockroach: &domain.Database_Options_Cockroach{
				Nodes:        1,
				ZoneReplicas: 1,
			}}},
		},
		Topology: &domain.Topology{
			Machines: []*domain.Topology_Machine{
				machine("m-crdb1", 4, 16, 100, dbComponent("crdb1")),
			},
			Connections: []*domain.Topology_Connection{selfConn("crdb1")},
		},
	}
}

// cockroachCluster is a 3-node cockroach cluster; the gossip mesh is modelled as
// COORDINATION connections from each follower to the first node.
func cockroachCluster() *domain.DatabasePreset {
	return cockroachMulti("cockroach-cluster", 3, 4, 16, 100)
}

func cockroachScale() *domain.DatabasePreset {
	return cockroachMulti("cockroach-scale", 6, 8, 16, 200)
}

func cockroachMulti(configID string, nodes int, cores uint32, memGb, diskGb uint64) *domain.DatabasePreset {
	machines := make([]*domain.Topology_Machine, 0, nodes)
	conns := make([]*domain.Topology_Connection, 0, nodes-1)
	for i := 0; i < nodes; i++ {
		id := "crdb" + string(rune('1'+i))
		machines = append(machines, machine("m-"+id, cores, memGb, diskGb, dbComponent(id)))
		if i > 0 {
			conns = append(conns, &domain.Topology_Connection{
				From: id, To: "crdb1", Kind: domain.Topology_Connection_KIND_COORDINATION,
			})
		}
	}
	return &domain.DatabasePreset{
		Database: &domain.Database{
			Kind:    domain.Database_KIND_COCKROACH,
			Version: "24.2",
			Config:  cfg(configID),
			Target:  selfHostedTarget(),
			Options: &domain.Database_Options{Options: &domain.Database_Options_Cockroach_{Cockroach: &domain.Database_Options_Cockroach{
				Nodes:        uint32(nodes),
				ZoneReplicas: uint32(nodes),
			}}},
		},
		Topology: &domain.Topology{Machines: machines, Connections: conns},
	}
}
