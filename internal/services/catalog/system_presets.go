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
		// ── MariaDB (mysql-shaped, KIND_MARIADB) ─────────────────────────────
		{
			Name:        "MariaDB Single",
			Description: "Single MariaDB instance",
			DB:          mariadbSingle(),
		},
		// ── Picodata ─────────────────────────────────────────────────────────
		{
			Name:        "Picodata Single",
			Description: "Single Picodata instance",
			DB:          picodataSingle(),
		},
		{
			Name:        "Picodata Cluster",
			Description: "Picodata 2-instance cluster (Raft coordination)",
			DB:          picodataCluster(),
		},
		// ── YDB ──────────────────────────────────────────────────────────────
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
	return &domain.DatabasePreset{
		Database: &domain.Database{
			Kind:    domain.Database_KIND_POSTGRES,
			Version: "16",
			Config:  cfg("postgres-ha"),
			Target:  selfHostedTarget(),
			Options: &domain.Database_Options{Options: &domain.Database_Options_Postgres_{Postgres: &domain.Database_Options_Postgres{
				Replication: &domain.Database_Options_Postgres_Replication{
					Mode:         domain.Database_Options_Postgres_Replication_MODE_PATRONI,
					Replicas:     1,
					SyncReplicas: 1,
				},
			}}},
		},
		Topology: &domain.Topology{
			Machines: []*domain.Topology_Machine{
				machine("m-etcd", 2, 4, 50, coordinatorComponent("etcd1")),
				machine("m-db1", 4, 16, 200, dbComponent("db1")),
				machine("m-db2", 4, 16, 200, dbComponent("db2")),
				machine("m-px", 2, 4, 50, proxyComponent("px")),
			},
			Connections: []*domain.Topology_Connection{
				{From: "db1", To: "etcd1", Kind: domain.Topology_Connection_KIND_COORDINATION},
				{From: "db2", To: "etcd1", Kind: domain.Topology_Connection_KIND_COORDINATION},
				{From: "db1", To: "db2", Kind: domain.Topology_Connection_KIND_REPLICATION},
				{From: "px", To: "db1", Kind: domain.Topology_Connection_KIND_PROXY},
			},
		},
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
	return &domain.DatabasePreset{
		Database: &domain.Database{
			Kind:    domain.Database_KIND_MYSQL,
			Version: "8.4",
			Config:  cfg("mysql-replica"),
			Target:  selfHostedTarget(),
			Options: &domain.Database_Options{Options: &domain.Database_Options_Mysql_{Mysql: &domain.Database_Options_Mysql{
				Replication: &domain.Database_Options_Mysql_Replication{
					Mode:     domain.Database_Options_Mysql_Replication_MODE_ASYNC,
					Replicas: 1,
				},
			}}},
		},
		Topology: &domain.Topology{
			Machines: []*domain.Topology_Machine{
				machine("m-db1", 4, 8, 100, dbComponent("db1")),
				machine("m-db2", 4, 8, 100, dbComponent("db2")),
			},
			Connections: []*domain.Topology_Connection{
				{From: "db1", To: "db2", Kind: domain.Topology_Connection_KIND_REPLICATION},
			},
		},
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
	return &domain.DatabasePreset{
		Database: &domain.Database{
			Kind:    domain.Database_KIND_PICODATA,
			Version: "25.3",
			Config:  cfg("picodata-cluster"),
			Target:  selfHostedTarget(),
			Options: &domain.Database_Options{Options: &domain.Database_Options_Picodata_{Picodata: &domain.Database_Options_Picodata{
				Shards: 3,
			}}},
		},
		Topology: &domain.Topology{
			Machines: []*domain.Topology_Machine{
				machine("m-pd1", 4, 8, 100, dbComponent("pd1")),
				machine("m-pd2", 4, 8, 100, dbComponent("pd2")),
			},
			Connections: []*domain.Topology_Connection{
				{From: "pd1", To: "pd2", Kind: domain.Topology_Connection_KIND_COORDINATION},
			},
		},
	}
}

// ── YDB ────────────────────────────────────────────────────────────────────

// ydbCluster mirrors ha_pipeline_test.go TestYDBClusterFullPipeline: 3 storage
// DATABASE nodes, peers wired by COORDINATION connections to the first node.
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
			Version: "24.1",
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
	machines := make([]*domain.Topology_Machine, 0, 3)
	conns := make([]*domain.Topology_Connection, 0, 2)
	ids := []string{"crdb1", "crdb2", "crdb3"}
	for i, id := range ids {
		machines = append(machines, machine("m-"+id, 4, 16, 100, dbComponent(id)))
		if i > 0 {
			conns = append(conns, &domain.Topology_Connection{
				From: id, To: ids[0], Kind: domain.Topology_Connection_KIND_COORDINATION,
			})
		}
	}
	return &domain.DatabasePreset{
		Database: &domain.Database{
			Kind:    domain.Database_KIND_COCKROACH,
			Version: "24.1",
			Config:  cfg("cockroach-cluster"),
			Target:  selfHostedTarget(),
			Options: &domain.Database_Options{Options: &domain.Database_Options_Cockroach_{Cockroach: &domain.Database_Options_Cockroach{
				Nodes:        3,
				ZoneReplicas: 3,
			}}},
		},
		Topology: &domain.Topology{Machines: machines, Connections: conns},
	}
}
