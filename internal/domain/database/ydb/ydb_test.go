package ydb

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/database/dbtest"
	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/packages"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

func ydbDatabase() *domain.Database {
	return &domain.Database{
		Kind: domain.Database_KIND_YDB,
		Source: &domain.Database_Params{
			Params: &domain.DatabaseParams{
				Engine: &domain.DatabaseParams_Ydb{
					Ydb: &domain.YdbParams{
						StorageNodes:   2,
						DatabaseNodes:  1,
						Haproxy:        1,
						FaultTolerance: domain.YdbParams_FAULT_TOLERANCE_BLOCK_4_2,
					},
				},
			},
		},
	}
}

func TestYdbBuildTopologySpec(t *testing.T) {
	db := ydbDatabase()
	spec, err := (&Database{}).BuildTopologySpec(db.GetParams().GetYdb())
	if err != nil {
		t.Fatalf("build topology spec: %v", err)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("spec invalid: %v", err)
	}
	if got, want := len(spec.GetNodes()), 4; got != want {
		t.Fatalf("nodes = %d, want %d", got, want)
	}
	if dbtest.ComponentsByIDInSpec(spec)["ydb-storage-1"].GetKind() != topology.Component_KIND_DATABASE {
		t.Fatal("ydb-storage-1 is not a database component")
	}
	if !dbtest.HasConnection(spec, "ydb-database-1", "ydb-storage-1", "node-broker") {
		t.Fatal("database-1 does not register against storage-1 over node-broker")
	}
	if !dbtest.HasConnection(spec, "haproxy-1", "ydb-database-1", "grpc") {
		t.Fatal("haproxy-1 does not front database-1 grpc")
	}
}

func TestYdbCombinedTopologyUsesStorageOnly(t *testing.T) {
	spec, err := (&Database{}).BuildTopologySpec(&domain.YdbParams{
		StorageNodes:   1,
		DatabaseNodes:  0,
		FaultTolerance: domain.YdbParams_FAULT_TOLERANCE_NONE,
		DatabasePath:   "/Root/testdb",
	})
	if err != nil {
		t.Fatalf("build topology spec: %v", err)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("spec invalid: %v", err)
	}
	if got, want := len(spec.GetNodes()), 1; got != want {
		t.Fatalf("nodes = %d, want %d", got, want)
	}
	components := dbtest.ComponentsByIDInSpec(spec)
	if components["ydb-storage-1"] == nil {
		t.Fatal("combined topology is missing ydb-storage-1")
	}
	if components["ydb-database-1"] != nil {
		t.Fatal("combined topology should not create a separate ydb-database-1 component")
	}
	if got, want := spec.GetLabels()["combined"], "true"; got != want {
		t.Fatalf("combined label = %q, want %q", got, want)
	}
}

func TestYdbDeploymentPlan(t *testing.T) {
	db := ydbDatabase()
	spec, err := (&Database{}).BuildTopologySpec(db.GetParams().GetYdb())
	if err != nil {
		t.Fatalf("build topology spec: %v", err)
	}

	plan, err := deploymentbuilder.BuildPlan(spec, dbtest.InfrastructureStateForSpec(spec), deploymentbuilder.BuildOptions{
		Database:        db,
		PackageResolver: packages.NewRegistry(PackageResolver{}),
		Renderers:       deploymentbuilder.NewRegistry(DeploymentRenderer{}),
	})
	if err != nil {
		t.Fatalf("build deployment plan: %v", err)
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("plan invalid: %v", err)
	}

	components := dbtest.ComponentsByID(plan)
	storage := components["ydb-storage-1"]
	service := dbtest.ServiceUnitText(storage)
	for _, want := range []string{"/usr/local/bin/ydbd server", "--grpc-port 2135", "--ic-port 19001", "--mon-port 8765", "--node 1"} {
		if !strings.Contains(service, want) {
			t.Fatalf("service missing %q:\n%s", want, service)
		}
	}
	if strings.Contains(service, "is prepared with config") {
		t.Fatalf("service still contains placeholder unit: %s", service)
	}
	config := dbtest.WriteFileText(storage, "030_write_config")
	for _, want := range []string{
		"static_erasure: block-4-2",
		"host_configs:",
		"host_config_id: 1",
		"path: /var/lib/stroppy-cloud/ydb-pdisk.data",
		"type: SSD",
		"domains_config:",
		"name: Root",
		"storage_pool_types:",
		"erasure_species: block-4-2",
		"pdisk_filter:",
		"node: [1]",
		"nto_select: 1",
		"channel_profile_config:",
		"profile_id: 0",
		"storage_pool_kind: ssd",
		"table_service_config:",
		"sql_version: 1",
		"actor_system_config:",
		"use_auto_config: true",
		"node_type: STORAGE",
		"blob_storage_config:",
		"service_set:",
		"vdisk_locations:",
		"pdisk_category: SSD",
		"host: 10.0.0.1",
		"host: 10.0.0.2",
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("config missing %q:\n%s", want, config)
		}
	}
	for _, legacy := range []string{"default_disk_type:"} {
		if strings.Contains(config, legacy) {
			t.Fatalf("config contains unsupported legacy field %q:\n%s", legacy, config)
		}
	}
	if !strings.Contains(service, "ExecStartPre=/bin/mkdir -p '/var/lib/stroppy-cloud'") {
		t.Fatalf("service does not create ydb pdisk parent:\n%s", service)
	}
	if install := dbtest.CallCmd(storage, "100_install"); !strings.Contains(install, "ydbd") {
		t.Fatalf("install does not fetch ydbd: %s", install)
	}
	if install := dbtest.CallCmd(storage, "100_install"); !strings.Contains(install, "\"${STROPPY_SERVER_ADDR%/}/api/binaries/ydbd/24.1.18/ydbd-24.1.18-linux-amd64.tar.gz\"") {
		t.Fatalf("install does not expand server binary URL: %s", install)
	}
	init := dbtest.CallCmd(storage, "240_init_database")
	for _, want := range []string{
		"timeout 10s /usr/local/bin/ydbd -s \"$server\" admin database \"$database\" status",
		"timeout 30s /usr/local/bin/ydbd -s \"$server\" admin blobstorage config init --yaml-file",
		"timeout 30s /usr/local/bin/ydbd -s \"$server\" admin database \"$database\" create \"$pool\"",
		"database='/Root/testdb'",
		"pool='ssd:1'",
		"grpc://127.0.0.1:2135",
	} {
		if !strings.Contains(init, want) {
			t.Fatalf("init command missing %q:\n%s", want, init)
		}
	}

	database := dbtest.ServiceUnitText(components["ydb-database-1"])
	for _, want := range []string{"--grpc-port 2136", "--ic-port 19002", "--mon-port 8766", "--tenant '/Root/testdb'", "--node-broker 'grpc://10.0.0.1:2135'", "--node-broker 'grpc://10.0.0.2:2135'"} {
		if !strings.Contains(database, want) {
			t.Fatalf("database service missing %q:\n%s", want, database)
		}
	}
	if strings.Contains(database, "--node dynamic") {
		t.Fatalf("database service contains unsupported dynamic node flag:\n%s", database)
	}
	databaseHealthcheck := dbtest.CallCmd(components["ydb-database-1"], "230_healthcheck")
	for _, want := range []string{
		"systemctl is-active --quiet \"$service\"",
		"grpc://127.0.0.1:2136",
		"timeout 5s /usr/local/bin/ydbd",
		"admin database \"$database\" status",
		"sleep 20",
	} {
		if !strings.Contains(databaseHealthcheck, want) {
			t.Fatalf("database healthcheck missing %q:\n%s", want, databaseHealthcheck)
		}
	}

	haproxy := dbtest.WriteFileText(components["haproxy-1"], "030_write_config")
	if !strings.Contains(haproxy, "server ydb-1 10.0.0.3:2136 check") {
		t.Fatalf("haproxy backends missing compute node:\n%s", haproxy)
	}
}

func TestYdbCombinedDeploymentStartsDatabaseServiceOnStorageNode(t *testing.T) {
	db := &domain.Database{
		Kind: domain.Database_KIND_YDB,
		Source: &domain.Database_Params{
			Params: &domain.DatabaseParams{
				Engine: &domain.DatabaseParams_Ydb{
					Ydb: &domain.YdbParams{
						StorageNodes:   1,
						DatabasePath:   "/Root/testdb",
						FaultTolerance: domain.YdbParams_FAULT_TOLERANCE_NONE,
					},
				},
			},
		},
	}
	spec, err := (&Database{}).BuildTopologySpec(db.GetParams().GetYdb())
	if err != nil {
		t.Fatalf("build topology spec: %v", err)
	}
	plan, err := deploymentbuilder.BuildPlan(spec, dbtest.InfrastructureStateForSpec(spec), deploymentbuilder.BuildOptions{
		Database:        db,
		PackageResolver: packages.NewRegistry(PackageResolver{}),
		Renderers:       deploymentbuilder.NewRegistry(DeploymentRenderer{}),
	})
	if err != nil {
		t.Fatalf("build deployment plan: %v", err)
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("plan invalid: %v", err)
	}

	storage := dbtest.ComponentsByID(plan)["ydb-storage-1"]
	if storage == nil {
		t.Fatal("plan is missing ydb-storage-1")
	}
	if got, want := len(plan.GetComponents()), 1; got != want {
		t.Fatalf("deployment components = %d, want %d", got, want)
	}
	storageService := dbtest.ServiceUnitText(storage)
	for _, want := range []string{"--grpc-port 2135", "--ic-port 19001", "--mon-port 8765"} {
		if !strings.Contains(storageService, want) {
			t.Fatalf("storage service missing %q:\n%s", want, storageService)
		}
	}
	databaseConfig := dbtest.WriteFileText(storage, "040_write_database_config")
	if !strings.Contains(databaseConfig, "static_erasure: none") {
		t.Fatalf("database config was not rendered:\n%s", databaseConfig)
	}
	databaseService := dbtest.WriteFileText(storage, "050_write_database_service")
	for _, want := range []string{
		"After=network-online.target stroppy-ydb-storage-1.service",
		"--yaml-config '/etc/stroppy-cloud/ydb-storage-1/database.yaml'",
		"--grpc-port 2136",
		"--ic-port 19002",
		"--mon-port 8766",
		"--tenant '/Root/testdb'",
		"--node-broker 'grpc://10.0.0.1:2135'",
	} {
		if !strings.Contains(databaseService, want) {
			t.Fatalf("database service missing %q:\n%s", want, databaseService)
		}
	}
	if cmd := dbtest.CallCmd(storage, "250_enable_start_database"); !strings.Contains(cmd, "stroppy-ydb-storage-1-database") {
		t.Fatalf("combined database service is not enabled:\n%s", cmd)
	}
	if check := dbtest.CallCmd(storage, "260_database_healthcheck"); !strings.Contains(check, "stroppy-ydb-storage-1-database") {
		t.Fatalf("combined database service is not healthchecked:\n%s", check)
	}
	if check := dbtest.CallCmd(storage, "260_database_healthcheck"); !strings.Contains(check, "admin database \"$database\" status") {
		t.Fatalf("combined database healthcheck does not wait for database status:\n%s", check)
	}
}

func TestYdbCombinedDeploymentSplitsMachineBudgetBetweenDaemons(t *testing.T) {
	db := &domain.Database{
		Kind: domain.Database_KIND_YDB,
		Source: &domain.Database_Params{
			Params: &domain.DatabaseParams{
				Engine: &domain.DatabaseParams_Ydb{
					Ydb: &domain.YdbParams{
						StorageNodes:   1,
						DatabasePath:   "/Root/testdb",
						FaultTolerance: domain.YdbParams_FAULT_TOLERANCE_NONE,
					},
				},
			},
		},
	}
	spec, err := (&Database{}).BuildTopologySpec(db.GetParams().GetYdb())
	if err != nil {
		t.Fatalf("build topology spec: %v", err)
	}
	state := dbtest.InfrastructureStateForSpec(spec)
	state.GetMachines()[0].AllocatedQuotas = []*deploymentpb.Quota_Allocation{
		quotaAllocation("compute.instanceCores.count", "cores", 8),
		quotaAllocation("compute.instanceMemory.size", "GiB", 16),
	}

	plan, err := deploymentbuilder.BuildPlan(spec, state, deploymentbuilder.BuildOptions{
		Database:        db,
		PackageResolver: packages.NewRegistry(PackageResolver{}),
		Renderers:       deploymentbuilder.NewRegistry(DeploymentRenderer{}),
	})
	if err != nil {
		t.Fatalf("build deployment plan: %v", err)
	}

	storage := dbtest.ComponentsByID(plan)["ydb-storage-1"]
	storageConfig := dbtest.WriteFileText(storage, "030_write_config")
	databaseConfig := dbtest.WriteFileText(storage, "040_write_database_config")
	for name, config := range map[string]string{"storage": storageConfig, "database": databaseConfig} {
		for _, want := range []string{
			"cpu_count: 4",
			"memory_controller_config:",
			"hard_limit_bytes: 7301234688",
		} {
			if !strings.Contains(config, want) {
				t.Fatalf("%s config missing %q:\n%s", name, want, config)
			}
		}
	}
}

func quotaAllocation(name, units string, used uint64) *deploymentpb.Quota_Allocation {
	return &deploymentpb.Quota_Allocation{
		Info: &deploymentpb.Quota_Info{
			Provider: deploymentpb.Provider_PROVIDER_YANDEX,
			Name:     name,
			Units:    units,
		},
		Used: used,
	}
}

func TestYdbInstallDownloadsUploadedPackageBlobWithAgentBearer(t *testing.T) {
	commands := ydbInstallCommands(&topology.Component{Role: ydbRoleStorage}, &domain.Package{
		DebFilename:     "${STROPPY_SERVER_ADDR%/}/api/packages/blob/packages_tenant_ydb",
		PackageRecordId: "pkg",
	})
	install := strings.Join(commands, "\n")
	for _, want := range []string{
		`curl -fsSL -H "Authorization: Bearer ${STROPPY_AGENT_TOKEN:?}" "${STROPPY_SERVER_ADDR%/}/api/packages/blob/packages_tenant_ydb" -o '/tmp/ydbd.tgz'`,
		"tar -xzf /tmp/ydbd.tgz",
	} {
		if !strings.Contains(install, want) {
			t.Fatalf("install script missing %q:\n%s", want, install)
		}
	}
}

func TestYdbPackageResolver(t *testing.T) {
	pkg, err := PackageResolver{}.ResolveDatabasePackage(ydbDatabase())
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if got, want := pkg.GetId(), "builtin/ydb/default"; got != want {
		t.Fatalf("package id = %q, want %q", got, want)
	}
	if !strings.Contains(pkg.GetDebFilename(), "${STROPPY_SERVER_ADDR%/}/api/binaries/ydbd/24.1.18/") {
		t.Fatalf("package download url = %q", pkg.GetDebFilename())
	}
}
