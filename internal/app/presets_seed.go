package app

import (
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	common "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

// builtinDatabasePresets returns the catalog of system database presets that a
// fresh control plane ships, mirroring the set the legacy `main` build seeded
// (internal/domain/types BuiltinPresets). The names/descriptions match main 1:1
// so the catalog is recognizable; the topology intent is translated into THIS
// branch's typed, provider-agnostic domain.Database model — the legacy
// per-component machine specs (CPUs/RAM/disk) live in the provider/topology
// layer now and are intentionally NOT part of a preset, so only the logical
// shape (kind + replica/proxy counts, HA toggles, engine options) is carried
// over here.
//
// Every record is stamped IsSystem=true with a fresh UUID, the default tenant,
// the seeded admin as author, and now() timings.
func builtinDatabasePresets(tenantID, authorID string) []*models.DatabasePresetRecord {
	var out []*models.DatabasePresetRecord

	add := func(name, description string, kind domain.Database_Kind, params *domain.DatabaseParams) {
		out = append(out, &models.DatabasePresetRecord{
			Entity: &common.Entity{
				Id:          uuid.NewString(),
				TenantId:    tenantID,
				Name:        name,
				Description: description,
				AuthorId:    authorID,
				Timings: &common.Timings{
					CreatedAt: timestamppb.Now(),
					UpdatedAt: timestamppb.Now(),
				},
			},
			IsSystem: true,
			Database: &domain.Database{
				Kind: kind,
				Source: &domain.Database_Params{
					Params: params,
				},
			},
		})
	}
	pg := func(p *domain.PostgresParams) *domain.DatabaseParams {
		return &domain.DatabaseParams{Engine: &domain.DatabaseParams_Postgres{Postgres: p}}
	}
	my := func(p *domain.MySqlParams) *domain.DatabaseParams {
		return &domain.DatabaseParams{Engine: &domain.DatabaseParams_Mysql{Mysql: p}}
	}
	maria := func(p *domain.MySqlParams) *domain.DatabaseParams {
		return &domain.DatabaseParams{Engine: &domain.DatabaseParams_Mariadb{Mariadb: p}}
	}
	pico := func(p *domain.PicodataParams) *domain.DatabaseParams {
		return &domain.DatabaseParams{Engine: &domain.DatabaseParams_Picodata{Picodata: p}}
	}
	ydb := func(p *domain.YdbParams) *domain.DatabaseParams {
		return &domain.DatabaseParams{Engine: &domain.DatabaseParams_Ydb{Ydb: p}}
	}
	ydbm := func(p *domain.YdbManagedParams) *domain.DatabaseParams {
		return &domain.DatabaseParams{Engine: &domain.DatabaseParams_YdbManaged{YdbManaged: p}}
	}
	crdb := func(p *domain.CockroachParams) *domain.DatabaseParams {
		return &domain.DatabaseParams{Engine: &domain.DatabaseParams_Cockroach{Cockroach: p}}
	}

	// --- PostgreSQL: single / ha / scale ---
	add("PostgreSQL single", "Single PostgreSQL instance",
		domain.Database_KIND_POSTGRES, pg(&domain.PostgresParams{}))
	add("PostgreSQL ha", "PostgreSQL with Patroni, HAProxy, PgBouncer, synchronous replication",
		domain.Database_KIND_POSTGRES, pg(&domain.PostgresParams{
			Replicas:         2,
			Haproxy:          1,
			Pgbouncer:        true,
			Patroni:          true,
			Etcd:             true,
			SyncReplicas:     1,
			MasterOptions:    map[string]string{"shared_buffers": "25%", "max_connections": "200", "work_mem": "64MB"},
			ReplicaOptions:   map[string]string{"shared_buffers": "25%", "max_connections": "200", "work_mem": "64MB"},
			PgbouncerOptions: map[string]string{"pool_mode": "transaction", "max_client_conn": "1000", "default_pool_size": "25"},
			PatroniOptions:   map[string]string{"ttl": "30", "loop_wait": "10", "retry_timeout": "10"},
		}))
	add("PostgreSQL scale", "PostgreSQL with 4 replicas, 2 HAProxy nodes, full HA stack",
		domain.Database_KIND_POSTGRES, pg(&domain.PostgresParams{
			Replicas:         4,
			Haproxy:          2,
			Pgbouncer:        true,
			Patroni:          true,
			Etcd:             true,
			SyncReplicas:     2,
			MasterOptions:    map[string]string{"shared_buffers": "25%", "max_connections": "500", "work_mem": "128MB", "effective_cache_size": "75%"},
			ReplicaOptions:   map[string]string{"shared_buffers": "25%", "max_connections": "500", "work_mem": "128MB", "effective_cache_size": "75%"},
			PgbouncerOptions: map[string]string{"pool_mode": "transaction", "max_client_conn": "2000", "default_pool_size": "50"},
			PatroniOptions:   map[string]string{"ttl": "30", "loop_wait": "10", "retry_timeout": "10"},
		}))

	// --- MySQL: single / replica / group ---
	mysqlSingle := func() *domain.MySqlParams { return &domain.MySqlParams{} }
	mysqlReplica := func() *domain.MySqlParams {
		return &domain.MySqlParams{
			Replicas:        2,
			Proxysql:        1,
			SemiSync:        true,
			PrimaryOptions:  map[string]string{"innodb_buffer_pool_size": "25%", "max_connections": "200"},
			ReplicaOptions:  map[string]string{"innodb_buffer_pool_size": "25%", "max_connections": "200"},
			ProxysqlOptions: map[string]string{"threads": "4", "max_connections": "2048"},
		}
	}
	mysqlGroup := func() *domain.MySqlParams {
		return &domain.MySqlParams{
			Replicas:         2,
			Proxysql:         2,
			GroupReplication: true,
			PrimaryOptions:   map[string]string{"innodb_buffer_pool_size": "25%", "max_connections": "500"},
			ReplicaOptions:   map[string]string{"innodb_buffer_pool_size": "25%", "max_connections": "500"},
			ProxysqlOptions:  map[string]string{"threads": "4", "max_connections": "2048"},
		}
	}
	add("MySQL single", "Single MySQL instance",
		domain.Database_KIND_MYSQL, my(mysqlSingle()))
	add("MySQL replica", "MySQL with semi-synchronous replication and ProxySQL",
		domain.Database_KIND_MYSQL, my(mysqlReplica()))
	add("MySQL group", "MySQL with Group Replication and ProxySQL",
		domain.Database_KIND_MYSQL, my(mysqlGroup()))

	// --- MariaDB: reuses the MySQL set/shape (DbKind switches the install package) ---
	add("MariaDB single", "Single MySQL instance",
		domain.Database_KIND_MARIADB, maria(mysqlSingle()))
	add("MariaDB replica", "MySQL with semi-synchronous replication and ProxySQL",
		domain.Database_KIND_MARIADB, maria(mysqlReplica()))
	add("MariaDB group", "MySQL with Group Replication and ProxySQL",
		domain.Database_KIND_MARIADB, maria(mysqlGroup()))

	// --- Picodata: single / cluster / scale ---
	add("Picodata single", "Single Picodata instance",
		domain.Database_KIND_PICODATA, pico(&domain.PicodataParams{
			Instances:         1,
			ReplicationFactor: 1,
			Shards:            1,
		}))
	add("Picodata cluster", "Picodata with 3 instances, 3 shards, HAProxy",
		domain.Database_KIND_PICODATA, pico(&domain.PicodataParams{
			Instances:         3,
			Haproxy:           1,
			ReplicationFactor: 2,
			Shards:            3,
		}))
	add("Picodata scale", "Picodata with 6 instances, multi-tier deployment",
		domain.Database_KIND_PICODATA, pico(&domain.PicodataParams{
			Instances:         6,
			Haproxy:           2,
			ReplicationFactor: 3,
			Shards:            6,
			Tiers: []*domain.PicodataTier{
				{Name: "compute", ReplicationFactor: 1, CanVote: true, Count: 3},
				{Name: "storage", ReplicationFactor: 2, CanVote: false, Count: 3},
			},
		}))

	// --- YDB (self-deployed): single + the three mirror-3-dc target topologies.
	// Legacy node flavors (64 vCPU storage etc.) are provider/topology concerns
	// and not representable on the preset; we carry node counts, fault tolerance,
	// failure domain, disk type, storage groups, pdisk count and db path. ---
	add("YDB single", "1 universal node (storage + compute on one box)",
		domain.Database_KIND_YDB, ydb(&domain.YdbParams{
			StorageNodes:   1,
			FaultTolerance: domain.YdbParams_FAULT_TOLERANCE_NONE,
			DatabasePath:   "/Root/testdb",
			AutoSizePdisks: true,
		}))
	ydbMirror := func(databaseNodes uint32) *domain.YdbParams {
		return &domain.YdbParams{
			StorageNodes:         3,
			DatabaseNodes:        databaseNodes,
			PdisksPerStorageNode: 3,
			FaultTolerance:       domain.YdbParams_FAULT_TOLERANCE_MIRROR_3_DC,
			FailureDomainType:    domain.YdbParams_FAILURE_DOMAIN_DISK,
			DefaultDiskType:      domain.YdbParams_DISK_TYPE_SSD,
			StorageGroups:        8,
			DatabasePath:         "/Root/testdb",
		}
	}
	add("YDB mirror3dc-3x32", "Target perf topology: mirror-3-dc, 3×32 vCPU compute, 3×16 vCPU storage, 9 io-m3 pdisks",
		domain.Database_KIND_YDB, ydb(ydbMirror(3)))
	add("YDB mirror3dc-9x32", "Target perf topology: mirror-3-dc, 9×32 vCPU compute, 3×16 vCPU storage, 9 io-m3 pdisks",
		domain.Database_KIND_YDB, ydb(ydbMirror(9)))
	add("YDB mirror3dc-3x64", "Target perf topology: mirror-3-dc, 3×64 vCPU compute, 3×16 vCPU storage, 9 io-m3 pdisks",
		domain.Database_KIND_YDB, ydb(ydbMirror(3)))

	// --- YDB Managed (Yandex Cloud): serverless / dedicated ---
	add("YDB Managed serverless", "Yandex Cloud Managed YDB — serverless (pay-per-request, grpcs only)",
		domain.Database_KIND_YDB_MANAGED, ydbm(&domain.YdbManagedParams{
			Type: domain.YdbManagedParams_TYPE_SERVERLESS,
		}))
	add("YDB Managed dedicated", "Yandex Cloud Managed YDB — dedicated medium (1 storage group, ssd)",
		domain.Database_KIND_YDB_MANAGED, ydbm(&domain.YdbManagedParams{
			Type:             domain.YdbManagedParams_TYPE_DEDICATED,
			ComputeType:      domain.YdbManagedParams_COMPUTE_TYPE_OLTP,
			ResourcePresetId: "medium",
			NodeCount:        1,
			StorageGroups:    1,
			StorageType:      "ssd",
		}))

	// --- CockroachDB: single / cluster-3 / cluster-6 (homogeneous node count) ---
	add("CockroachDB single", "Single CockroachDB node — dev / smoke runs",
		domain.Database_KIND_COCKROACH, crdb(&domain.CockroachParams{Nodes: 1}))
	add("CockroachDB cluster-3", "3-node CockroachDB cluster",
		domain.Database_KIND_COCKROACH, crdb(&domain.CockroachParams{Nodes: 3}))
	add("CockroachDB cluster-6", "6-node CockroachDB cluster (more parallel ranges)",
		domain.Database_KIND_COCKROACH, crdb(&domain.CockroachParams{Nodes: 6}))

	return out
}
