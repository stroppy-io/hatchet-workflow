package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	infrastructurebuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/infrastructure"
	runbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	workloadbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/workload"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	common "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	suitesvc "github.com/stroppy-io/stroppy-cloud/internal/services/suite"
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
		ensureDatabaseParamsPackage(kind, params)
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
			MasterOptions:    map[string]string{"shared_buffers": "512MB", "max_connections": "200", "work_mem": "64MB"},
			ReplicaOptions:   map[string]string{"shared_buffers": "512MB", "max_connections": "200", "work_mem": "64MB"},
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
			MasterOptions:    map[string]string{"shared_buffers": "1GB", "max_connections": "500", "work_mem": "128MB", "effective_cache_size": "3GB"},
			ReplicaOptions:   map[string]string{"shared_buffers": "1GB", "max_connections": "500", "work_mem": "128MB", "effective_cache_size": "3GB"},
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

func seedWorkloadPresets(ctx context.Context, log *slog.Logger, repo *postgres.WorkloadPresetRepo, tenantID, authorID string) ([]*models.WorkloadPresetRecord, error) {
	existing, _, err := repo.List(ctx, &api.ListWorkloadPresetsRequest{TenantId: tenantID}, authorID)
	if err != nil {
		return nil, err
	}
	byName := workloadPresetsByName(existing)
	presets := builtinWorkloadPresets(tenantID, authorID)
	created := 0
	out := make([]*models.WorkloadPresetRecord, 0, len(presets))
	for _, p := range presets {
		if current := byName[p.GetEntity().GetName()]; current != nil {
			out = append(out, current)
			continue
		}
		if err := repo.Create(ctx, p); err != nil {
			return nil, err
		}
		created++
		out = append(out, p)
	}
	log.Info("first-boot seeding: builtin workload presets ensured",
		slog.String("tenant_id", tenantID), slog.Int("created", created), slog.Int("catalog", len(out)))
	return out, nil
}

func seedTestPresets(ctx context.Context, log *slog.Logger, repo *postgres.TestPresetRepo, tenantID, authorID string, dbPresets []*models.DatabasePresetRecord, workloads []*models.WorkloadPresetRecord) ([]*models.TestPresetRecord, error) {
	existing, _, err := repo.List(ctx, &api.ListTestPresetsRequest{TenantId: tenantID}, authorID)
	if err != nil {
		return nil, err
	}
	byName := testPresetsByName(existing)
	byProtocol := workloadPresetsByProtocol(workloads)

	created := 0
	updated := 0
	out := make([]*models.TestPresetRecord, 0, len(dbPresets))
	for _, dbPreset := range dbPresets {
		name := selfCheckTestPresetName(dbPreset)
		rec, err := buildBuiltinTestPresetRecord(tenantID, authorID, dbPreset, byProtocol)
		if err != nil {
			return nil, err
		}
		if current := byName[name]; current != nil {
			if reconcileBuiltinTestPreset(current, rec) {
				if err := repo.Update(ctx, current); err != nil {
					return nil, err
				}
				updated++
			}
			out = append(out, current)
			continue
		}

		if err := repo.Create(ctx, rec); err != nil {
			return nil, err
		}
		created++
		out = append(out, rec)
	}
	log.Info("first-boot seeding: builtin test presets ensured",
		slog.String("tenant_id", tenantID), slog.Int("created", created), slog.Int("updated", updated), slog.Int("catalog", len(out)))
	return out, nil
}

func buildBuiltinTestPresetRecord(tenantID, authorID string, dbPreset *models.DatabasePresetRecord, byProtocol map[domain.Workload_Protocol]*models.WorkloadPresetRecord) (*models.TestPresetRecord, error) {
	protocol := workloadProtocolForDatabase(dbPreset.GetDatabase().GetKind())
	workload := byProtocol[protocol]
	if workload == nil {
		return nil, fmt.Errorf("missing builtin workload preset for protocol %s", protocol)
	}

	rec := &models.TestPresetRecord{
		Entity:   newSeedEntity(tenantID, authorID, selfCheckTestPresetName(dbPreset), "Minimal self-check for "+dbPreset.GetEntity().GetName()),
		IsSystem: true,
		Test: &domain.Test{
			Database: cloneDatabaseWithBuiltinPackage(dbPreset.GetDatabase()),
			Workload: proto.Clone(workload.GetWorkload()).(*domain.Workload),
		},
	}
	rec.Summary = &models.TestPresetRecord_Summary{
		DbKind:         rec.GetTest().GetDatabase().GetKind(),
		Protocol:       rec.GetTest().GetWorkload().GetProtocol(),
		StroppyVersion: rec.GetTest().GetWorkload().GetStroppyVersion(),
	}
	if err := rec.GetTest().Validate(); err != nil {
		return nil, fmt.Errorf("builtin test preset %q is invalid: %w", rec.GetEntity().GetName(), err)
	}
	return rec, nil
}

func reconcileBuiltinTestPreset(current, canonical *models.TestPresetRecord) bool {
	if current == nil || canonical == nil {
		return false
	}
	changed := !proto.Equal(current.GetTest(), canonical.GetTest()) ||
		!proto.Equal(current.GetSummary(), canonical.GetSummary()) ||
		current.GetIsSystem() != canonical.GetIsSystem() ||
		current.GetEntity().GetDescription() != canonical.GetEntity().GetDescription()
	if !changed {
		return false
	}
	current.Test = proto.Clone(canonical.GetTest()).(*domain.Test)
	current.Summary = proto.Clone(canonical.GetSummary()).(*models.TestPresetRecord_Summary)
	current.IsSystem = canonical.GetIsSystem()
	if entity := current.GetEntity(); entity != nil {
		entity.Description = canonical.GetEntity().GetDescription()
		entity.IsFavorite = false
		if timings := entity.GetTimings(); timings != nil {
			timings.UpdatedAt = timestamppb.Now()
		}
	}
	return true
}

const (
	builtinSelfCheckSuiteName       = "Builtin self-check suite"
	builtinDockerSelfCheckSuiteName = "Builtin Docker self-check suite"

	dockerSelfCheckCPUCores = 1
	dockerSelfCheckMemoryMB = 2048
	dockerSelfCheckDiskGB   = 8

	dockerSelfCheckMaxCPUCores = 12
	dockerSelfCheckMaxMemoryMB = 96 * 1024
	dockerSelfCheckMaxDiskGB   = 96
)

type builtinSuiteSeed struct {
	name        string
	description string
	provider    deploymentpb.Provider
	include     func(*models.TestPresetRecord) bool
}

func builtinSuiteSeeds() []builtinSuiteSeed {
	return []builtinSuiteSeed{
		{
			name:        builtinSelfCheckSuiteName,
			description: "Minimal self-check across every seeded database topology",
			provider:    deploymentpb.Provider_PROVIDER_YANDEX,
			include:     includeSuiteTest,
		},
		{
			name:        builtinDockerSelfCheckSuiteName,
			description: "Docker-only self-check across every Docker-compatible seeded database topology",
			provider:    deploymentpb.Provider_PROVIDER_DOCKER,
			include:     includeDockerSuiteTest,
		},
	}
}

func seedSuites(ctx context.Context, log *slog.Logger, repo *postgres.SuiteRepo, tenantID, authorID string, tests []*models.TestPresetRecord) error {
	existing, _, err := repo.List(ctx, suitesvc.SuiteListQuery{TenantID: tenantID, CallerID: authorID})
	if err != nil {
		return err
	}
	byName := suiteRecordsByName(existing)
	created := 0
	updated := 0
	for _, seed := range builtinSuiteSeeds() {
		if current := byName[seed.name]; current != nil {
			rec, err := buildBuiltinSuiteRecord(seed, tenantID, authorID, tests)
			if err != nil {
				return err
			}
			preserveBuiltinSuiteIdentity(rec, current)
			if err := repo.Update(ctx, rec); err != nil {
				return err
			}
			updated++
			log.Info("first-boot seeding: builtin suite updated",
				slog.String("tenant_id", tenantID), slog.String("suite_id", rec.GetEntity().GetId()), slog.String("suite", seed.name), slog.Int("cells", len(rec.GetSpec().GetCells())))
			continue
		}

		rec, err := buildBuiltinSuiteRecord(seed, tenantID, authorID, tests)
		if err != nil {
			return err
		}
		if err := repo.Create(ctx, rec); err != nil {
			return err
		}
		created++
		log.Info("first-boot seeding: builtin suite created",
			slog.String("tenant_id", tenantID), slog.String("suite_id", rec.GetEntity().GetId()), slog.String("suite", seed.name), slog.Int("cells", len(rec.GetSpec().GetCells())))
	}
	log.Info("first-boot seeding: builtin suites ensured",
		slog.String("tenant_id", tenantID), slog.Int("created", created), slog.Int("updated", updated), slog.Int("catalog", len(builtinSuiteSeeds())))
	return nil
}

func buildBuiltinSuiteRecord(seed builtinSuiteSeed, tenantID, authorID string, tests []*models.TestPresetRecord) (*models.SuiteRecord, error) {
	if seed.include == nil {
		seed.include = includeSuiteTest
	}

	cells := make([]*domain.SuiteCell, 0, len(tests))
	for _, test := range tests {
		if !seed.include(test) {
			continue
		}
		run, err := runbuilder.BuildTestRun(runbuilder.BuildOptions{
			ID:             uuid.NewString(),
			Database:       test.GetTest().GetDatabase(),
			Workload:       test.GetTest().GetWorkload(),
			Provider:       seed.provider,
			Infrastructure: builtinSuiteInfrastructureOptions(seed.provider),
		})
		if err != nil {
			return nil, fmt.Errorf("build builtin suite cell %q: %w", test.GetEntity().GetName(), err)
		}
		if seed.provider == deploymentpb.Provider_PROVIDER_DOCKER && !dockerSelfCheckRunFits(run) {
			continue
		}
		cells = append(cells, &domain.SuiteCell{
			Id:      uuid.NewString(),
			Name:    test.GetEntity().GetName(),
			Enabled: true,
			Source: &domain.SuiteCell_TestPresetId{
				TestPresetId: test.GetEntity().GetId(),
			},
			MachineOverrides: cloneMachinePlans(run.GetInfrastructurePlan().GetMachines()),
		})
	}
	if len(cells) == 0 {
		return nil, fmt.Errorf("builtin suite %q has no cells", seed.name)
	}

	id := uuid.NewString()
	rec := &models.SuiteRecord{
		Entity: newSeedEntity(tenantID, authorID, seed.name, seed.description),
		Spec: &domain.Suite{
			Id:                 id,
			Cells:              cells,
			Provider:           seed.provider,
			DefaultMaxParallel: 1,
		},
		Summary: &models.SuiteRecord_Summary{
			CellCount: uint32(len(cells)),
		},
	}
	rec.Entity.Id = id
	if err := rec.GetSpec().Validate(); err != nil {
		return nil, fmt.Errorf("builtin suite is invalid: %w", err)
	}
	return rec, nil
}

func builtinSuiteInfrastructureOptions(provider deploymentpb.Provider) infrastructurebuilder.BuildOptions {
	if provider != deploymentpb.Provider_PROVIDER_DOCKER {
		return infrastructurebuilder.BuildOptions{}
	}
	sizing := infrastructurebuilder.MachineSizing{
		CPUCores: dockerSelfCheckCPUCores,
		MemoryMB: dockerSelfCheckMemoryMB,
		DiskGB:   dockerSelfCheckDiskGB,
	}
	return infrastructurebuilder.BuildOptions{
		DefaultSizing: sizing,
		MachineSizing: map[string]infrastructurebuilder.MachineSizing{
			workloadbuilder.RunnerNodeID: sizing,
		},
	}
}

func dockerSelfCheckRunFits(run *domain.TestRun) bool {
	var cpu uint64
	var memory uint64
	var disk uint64
	for _, machine := range run.GetInfrastructurePlan().GetMachines() {
		if machine.GetDocker() == nil {
			return false
		}
		for _, req := range machine.GetQuotaRequests() {
			info := req.GetInfo()
			if info.GetProvider() != deploymentpb.Provider_PROVIDER_DOCKER {
				continue
			}
			switch info.GetName() {
			case "host.cpuCores":
				cpu += req.GetRequest()
			case "host.memory.size":
				memory += req.GetRequest()
			case "host.disk.size":
				disk += req.GetRequest()
			}
		}
	}
	return cpu <= dockerSelfCheckMaxCPUCores &&
		memory <= dockerSelfCheckMaxMemoryMB &&
		disk <= dockerSelfCheckMaxDiskGB
}

func preserveBuiltinSuiteIdentity(next, current *models.SuiteRecord) {
	if next == nil || current == nil {
		return
	}
	if next.Entity != nil && current.GetEntity() != nil {
		next.Entity.Id = current.GetEntity().GetId()
		next.Entity.TenantId = current.GetEntity().GetTenantId()
		next.Entity.AuthorId = current.GetEntity().GetAuthorId()
		if current.GetEntity().GetTimings() != nil {
			next.Entity.Timings = proto.Clone(current.GetEntity().GetTimings()).(*common.Timings)
		} else {
			next.Entity.Timings = &common.Timings{}
		}
		next.Entity.Timings.UpdatedAt = timestamppb.Now()
	}
	if next.Spec != nil && current.GetSpec() != nil {
		next.Spec.Id = current.GetSpec().GetId()
		if next.Spec.Id == "" {
			next.Spec.Id = current.GetEntity().GetId()
		}
	}
	if current.GetSummary() != nil {
		cellCount := next.GetSummary().GetCellCount()
		next.Summary = proto.Clone(current.GetSummary()).(*models.SuiteRecord_Summary)
		next.Summary.CellCount = cellCount
	}
}

func includeSuiteTest(*models.TestPresetRecord) bool {
	return true
}

func includeDockerSuiteTest(test *models.TestPresetRecord) bool {
	return test.GetTest().GetDatabase().GetKind() != domain.Database_KIND_YDB_MANAGED
}

func builtinWorkloadPresets(tenantID, authorID string) []*models.WorkloadPresetRecord {
	add := func(name, description string, protocol domain.Workload_Protocol) *models.WorkloadPresetRecord {
		workload := minimalSelfCheckWorkload(protocol)
		return &models.WorkloadPresetRecord{
			Entity:   newSeedEntity(tenantID, authorID, name, description),
			IsSystem: true,
			Workload: workload,
			Summary: &models.WorkloadPresetRecord_Summary{
				Protocol:       workload.GetProtocol(),
				StroppyVersion: workload.GetStroppyVersion(),
				Script:         workload.GetScript(),
			},
		}
	}
	return []*models.WorkloadPresetRecord{
		add("Self-check PG", "Minimal PostgreSQL-compatible workload", domain.Workload_PROTOCOL_PG),
		add("Self-check MySQL", "Minimal MySQL-compatible workload", domain.Workload_PROTOCOL_MYSQL),
		add("Self-check Picodata", "Minimal Picodata workload", domain.Workload_PROTOCOL_PICODATA),
		add("Self-check YDB gRPC", "Minimal self-hosted YDB workload", domain.Workload_PROTOCOL_YDB_GRPC),
		add("Self-check YDB gRPCS", "Minimal managed YDB workload", domain.Workload_PROTOCOL_YDB_GRPCS),
		add("Self-check CockroachDB", "Minimal CockroachDB workload", domain.Workload_PROTOCOL_COCKROACH),
	}
}

func minimalSelfCheckWorkload(protocol domain.Workload_Protocol) *domain.Workload {
	return &domain.Workload{
		Script:   "tpcc/tx",
		Protocol: protocol,
		Execution: &domain.Workload_Execution{
			Vus: 1,
			Limit: &domain.Workload_Execution_Duration{
				Duration: "30s",
			},
			Quiet:        true,
			NoThresholds: true,
		},
		Parameters: &domain.Workload_Parameters{
			PoolSize:    1,
			ScaleFactor: 1,
		},
	}
}

func newSeedEntity(tenantID, authorID, name, description string) *common.Entity {
	now := timestamppb.Now()
	return &common.Entity{
		Id:          uuid.NewString(),
		TenantId:    tenantID,
		Name:        name,
		Description: description,
		AuthorId:    authorID,
		Timings: &common.Timings{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}

func ensureDatabaseParamsPackage(kind domain.Database_Kind, params *domain.DatabaseParams) {
	if params == nil {
		return
	}
	if kind == domain.Database_KIND_YDB_MANAGED {
		params.Package = nil
		return
	}
	if params.GetPackage() != nil {
		return
	}
	params.Package = builtinDatabasePackage(kind, params.GetVersion())
}

func builtinDatabasePackage(kind domain.Database_Kind, version string) *domain.Package {
	if kind == domain.Database_KIND_UNSPECIFIED ||
		kind == domain.Database_KIND_YDB_MANAGED ||
		kind == domain.Database_KIND_EXTERNAL {
		return nil
	}
	if version == "" {
		version = "default"
	}
	pkg := &domain.Package{
		DbKind:    kind,
		DbVersion: version,
		IsBuiltin: true,
	}
	switch kind {
	case domain.Database_KIND_POSTGRES:
		pkg.Id = "builtin/postgres/" + version
		pkg.Name = "PostgreSQL " + version
		pkg.AptPackages = []string{"postgresql", "postgresql-contrib"}
		pkg.PreInstall = []string{"apt-get update"}
		if version != "default" {
			pkg.AptPackages = []string{"postgresql-" + version, "postgresql-contrib-" + version}
			pkg.PreInstall = seedPGDGPreInstall()
		}
	case domain.Database_KIND_MYSQL:
		pkg.Id = "builtin/mysql/" + version
		pkg.Name = "MySQL " + version
		pkg.AptPackages = []string{"mysql-server"}
		pkg.PreInstall = []string{"apt-get update"}
		if version != "default" {
			pkg.AptPackages = []string{"mysql-server-" + version}
		}
	case domain.Database_KIND_MARIADB:
		pkg.Id = "builtin/mariadb/" + version
		pkg.Name = "MariaDB " + version
		pkg.AptPackages = []string{"mariadb-server"}
		pkg.PreInstall = []string{"apt-get update"}
		if version != "default" {
			pkg.AptPackages = []string{"mariadb-server-" + version}
		}
	case domain.Database_KIND_PICODATA:
		pkg.Id = "builtin/picodata/" + version
		pkg.Name = "Picodata " + version
		pkg.AptPackages = []string{"picodata"}
		pkg.PreInstall = []string{"apt-get update"}
		if version != "default" {
			pkg.AptPackages = []string{"picodata=" + version}
		}
	case domain.Database_KIND_YDB:
		pkg.Id = "builtin/ydb/" + version
		pkg.Name = "YDB " + version
		pkg.DebFilename = "https://binaries.ydb.tech/release/24.1.18/ydbd-24.1.18-linux-amd64.tar.gz"
		if version != "default" {
			pkg.DebFilename = fmt.Sprintf("https://binaries.ydb.tech/release/%s/ydbd-%s-linux-amd64.tar.gz", version, version)
		}
	case domain.Database_KIND_COCKROACH:
		pkg.Id = "builtin/cockroach/" + version
		pkg.Name = "CockroachDB " + version
		pkg.DebFilename = "https://binaries.cockroachdb.com/cockroach-v23.2.5.linux-amd64.tgz"
		if version != "default" {
			pkg.DebFilename = fmt.Sprintf("https://binaries.cockroachdb.com/cockroach-v%s.linux-amd64.tgz", version)
		}
	}
	return pkg
}

func seedPGDGPreInstall() []string {
	const keyring = "/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc"
	return []string{
		"install -d /usr/share/postgresql-common/pgdg",
		"curl -fsSL -o " + keyring + " https://www.postgresql.org/media/keys/ACCC4CF8.asc",
		`sh -c 'echo "deb [signed-by=` + keyring + `] http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'`,
		"apt-get update",
	}
}

func cloneDatabaseWithBuiltinPackage(input *domain.Database) *domain.Database {
	if input == nil {
		return nil
	}
	db := proto.Clone(input).(*domain.Database)
	ensureDatabaseParamsPackage(db.GetKind(), db.GetParams())
	return db
}

func cloneMachinePlans(input []*deploymentpb.MachinePlan) []*deploymentpb.MachinePlan {
	if len(input) == 0 {
		return nil
	}
	out := make([]*deploymentpb.MachinePlan, 0, len(input))
	for _, machine := range input {
		if machine == nil {
			continue
		}
		out = append(out, proto.Clone(machine).(*deploymentpb.MachinePlan))
	}
	return out
}

func workloadProtocolForDatabase(kind domain.Database_Kind) domain.Workload_Protocol {
	switch kind {
	case domain.Database_KIND_MYSQL, domain.Database_KIND_MARIADB:
		return domain.Workload_PROTOCOL_MYSQL
	case domain.Database_KIND_PICODATA:
		return domain.Workload_PROTOCOL_PICODATA
	case domain.Database_KIND_YDB:
		return domain.Workload_PROTOCOL_YDB_GRPC
	case domain.Database_KIND_YDB_MANAGED:
		return domain.Workload_PROTOCOL_YDB_GRPCS
	case domain.Database_KIND_COCKROACH:
		return domain.Workload_PROTOCOL_COCKROACH
	default:
		return domain.Workload_PROTOCOL_PG
	}
}

func selfCheckTestPresetName(dbPreset *models.DatabasePresetRecord) string {
	return "Self-check / " + dbPreset.GetEntity().GetName()
}

func databasePresetsByName(input []*models.DatabasePresetRecord) map[string]*models.DatabasePresetRecord {
	out := make(map[string]*models.DatabasePresetRecord, len(input))
	for _, p := range input {
		if p.GetEntity().GetName() != "" {
			out[p.GetEntity().GetName()] = p
		}
	}
	return out
}

func workloadPresetsByName(input []*models.WorkloadPresetRecord) map[string]*models.WorkloadPresetRecord {
	out := make(map[string]*models.WorkloadPresetRecord, len(input))
	for _, p := range input {
		if p.GetEntity().GetName() != "" {
			out[p.GetEntity().GetName()] = p
		}
	}
	return out
}

func workloadPresetsByProtocol(input []*models.WorkloadPresetRecord) map[domain.Workload_Protocol]*models.WorkloadPresetRecord {
	out := make(map[domain.Workload_Protocol]*models.WorkloadPresetRecord, len(input))
	for _, p := range input {
		if p.GetWorkload().GetProtocol() != domain.Workload_PROTOCOL_UNSPECIFIED {
			out[p.GetWorkload().GetProtocol()] = p
		}
	}
	return out
}

func testPresetsByName(input []*models.TestPresetRecord) map[string]*models.TestPresetRecord {
	out := make(map[string]*models.TestPresetRecord, len(input))
	for _, p := range input {
		if p.GetEntity().GetName() != "" {
			out[p.GetEntity().GetName()] = p
		}
	}
	return out
}

func suiteRecordsByName(input []*models.SuiteRecord) map[string]*models.SuiteRecord {
	out := make(map[string]*models.SuiteRecord, len(input))
	for _, p := range input {
		if p.GetEntity().GetName() != "" {
			out[p.GetEntity().GetName()] = p
		}
	}
	return out
}
