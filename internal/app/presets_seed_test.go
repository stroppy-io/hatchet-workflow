package app

import (
	"strings"
	"testing"

	runbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"google.golang.org/protobuf/proto"
)

func TestBuiltinDatabasePresetsCarryBuiltinPackage(t *testing.T) {
	presets := builtinDatabasePresets("tenant-1", "author-1")
	if len(presets) == 0 {
		t.Fatal("builtin database preset catalog is empty")
	}

	kinds := map[domain.Database_Kind]bool{}
	for _, preset := range presets {
		db := preset.GetDatabase()
		kinds[db.GetKind()] = true
		pkg := db.GetParams().GetPackage()
		if db.GetKind() == domain.Database_KIND_YDB_MANAGED {
			if pkg != nil {
				t.Fatalf("%q managed YDB preset has package %+v", preset.GetEntity().GetName(), pkg)
			}
			continue
		}
		if pkg == nil {
			t.Fatalf("%q has no builtin package", preset.GetEntity().GetName())
		}
		if !pkg.GetIsBuiltin() {
			t.Fatalf("%q package is not builtin", preset.GetEntity().GetName())
		}
		if pkg.GetDbKind() != db.GetKind() {
			t.Fatalf("%q package db kind = %s, want %s", preset.GetEntity().GetName(), pkg.GetDbKind(), db.GetKind())
		}
		if pkg.GetId() == "" {
			t.Fatalf("%q package id is empty", preset.GetEntity().GetName())
		}
	}

	for _, kind := range []domain.Database_Kind{
		domain.Database_KIND_POSTGRES,
		domain.Database_KIND_MYSQL,
		domain.Database_KIND_MARIADB,
		domain.Database_KIND_PICODATA,
		domain.Database_KIND_YDB,
		domain.Database_KIND_YDB_MANAGED,
		domain.Database_KIND_COCKROACH,
	} {
		if !kinds[kind] {
			t.Fatalf("builtin database catalog is missing kind %s", kind)
		}
	}
}

func TestReconcileBuiltinDatabasePresetAddsMissingPackage(t *testing.T) {
	preset := firstDatabasePresetExcept(t, builtinDatabasePresets("tenant-1", "author-1"), domain.Database_KIND_YDB_MANAGED)
	preset.GetDatabase().GetParams().Package = nil

	if !reconcileBuiltinDatabasePreset(preset, nil) {
		t.Fatal("reconcileBuiltinDatabasePreset reported no change for missing package")
	}
	pkg := preset.GetDatabase().GetParams().GetPackage()
	if pkg == nil {
		t.Fatal("reconcileBuiltinDatabasePreset did not restore package")
	}
	if !pkg.GetIsBuiltin() {
		t.Fatalf("restored package is not builtin: %+v", pkg)
	}
	if reconcileBuiltinDatabasePreset(preset, nil) {
		t.Fatal("reconcileBuiltinDatabasePreset changed an already reconciled preset")
	}
}

func TestReconcileBuiltinDatabasePresetRemovesManagedYdbPackage(t *testing.T) {
	preset := firstDatabasePresetFor(t, builtinDatabasePresets("tenant-1", "author-1"), domain.Database_KIND_YDB_MANAGED)
	preset.GetDatabase().GetParams().Package = &domain.Package{
		Id:        "builtin/ydb-managed/managed",
		Name:      "Yandex Managed YDB",
		DbKind:    domain.Database_KIND_YDB_MANAGED,
		DbVersion: "managed",
		IsBuiltin: true,
	}

	if !reconcileBuiltinDatabasePreset(preset, nil) {
		t.Fatal("reconcileBuiltinDatabasePreset reported no change for managed YDB package")
	}
	if pkg := preset.GetDatabase().GetParams().GetPackage(); pkg != nil {
		t.Fatalf("managed YDB package was not removed: %+v", pkg)
	}
	if reconcileBuiltinDatabasePreset(preset, nil) {
		t.Fatal("reconcileBuiltinDatabasePreset changed an already reconciled managed YDB preset")
	}
}

func TestBuiltinPostgresPresetsUseConcreteMemoryValues(t *testing.T) {
	for _, preset := range builtinDatabasePresets("tenant-1", "author-1") {
		if preset.GetDatabase().GetKind() != domain.Database_KIND_POSTGRES {
			continue
		}
		params := preset.GetDatabase().GetParams().GetPostgres()
		for group, options := range map[string]map[string]string{
			"master":  params.GetMasterOptions(),
			"replica": params.GetReplicaOptions(),
		} {
			for key, value := range options {
				if strings.Contains(value, "%") {
					t.Fatalf("%q %s option %s uses percentage value %q; postgresql.conf requires concrete units", preset.GetEntity().GetName(), group, key, value)
				}
			}
		}
	}
}

func TestBuiltinPicodataPresetsInstallRepository(t *testing.T) {
	preset := databasePresetByName(t, builtinDatabasePresets("tenant-1", "author-1"), "Picodata single")
	pkg := preset.GetDatabase().GetParams().GetPackage()
	if pkg == nil {
		t.Fatal("Picodata single has no builtin package")
	}
	preInstall := strings.Join(pkg.GetPreInstall(), "\n")
	for _, want := range []string{
		"https://download.picodata.io/tarantool-picodata/picodata.gpg.key",
		"http://download.picodata.io/tarantool-picodata/%s/",
		"/etc/apt/sources.list.d/picodata.list",
	} {
		if !strings.Contains(preInstall, want) {
			t.Fatalf("Picodata preinstall missing %q:\n%s", want, preInstall)
		}
	}
}

func TestBuiltinVersionedMySQLPackagesConfigureRepository(t *testing.T) {
	pkg := builtinDatabasePackage(domain.Database_KIND_MYSQL, "8.4")
	if pkg == nil {
		t.Fatal("versioned MySQL package is nil")
	}
	if got, want := pkg.GetAptPackages()[0], "mysql-server"; got != want {
		t.Fatalf("apt package = %q, want %q", got, want)
	}
	preInstall := strings.Join(pkg.GetPreInstall(), "\n")
	for _, want := range []string{
		"http://repo.mysql.com/apt/ubuntu/",
		"mysql-8.4-lts",
		"RPM-GPG-KEY-mysql-2025",
	} {
		if !strings.Contains(preInstall, want) {
			t.Fatalf("MySQL preinstall missing %q:\n%s", want, preInstall)
		}
	}
}

func TestBuiltinVersionedMariaDBPackagesConfigureRepository(t *testing.T) {
	pkg := builtinDatabasePackage(domain.Database_KIND_MARIADB, "11.4")
	if pkg == nil {
		t.Fatal("versioned MariaDB package is nil")
	}
	if got, want := pkg.GetAptPackages()[0], "mariadb-server"; got != want {
		t.Fatalf("apt package = %q, want %q", got, want)
	}
	preInstall := strings.Join(pkg.GetPreInstall(), "\n")
	for _, want := range []string{
		"https://r.mariadb.com/downloads/mariadb_repo_setup",
		"--mariadb-server-version=11.4",
	} {
		if !strings.Contains(preInstall, want) {
			t.Fatalf("MariaDB preinstall missing %q:\n%s", want, preInstall)
		}
	}
}

func TestBuiltinYdbSingleUsesCombinedNode(t *testing.T) {
	preset := databasePresetByName(t, builtinDatabasePresets("tenant-1", "author-1"), "YDB single")
	params := preset.GetDatabase().GetParams().GetYdb()
	if got, want := params.GetStorageNodes(), uint32(1); got != want {
		t.Fatalf("storage nodes = %d, want %d", got, want)
	}
	if got, want := params.GetDatabaseNodes(), uint32(0); got != want {
		t.Fatalf("database nodes = %d, want %d", got, want)
	}
	if got, want := params.GetDatabasePath(), "/Root/testdb"; got != want {
		t.Fatalf("database path = %q, want %q", got, want)
	}
}

func TestReconcileBuiltinTestPresetUpdatesEmbeddedDatabase(t *testing.T) {
	workloads := workloadPresetsByProtocol(builtinWorkloadPresets("tenant-1", "author-1"))
	canonicalDB := databasePresetByName(t, builtinDatabasePresets("tenant-1", "author-1"), "PostgreSQL ha")
	staleDB := proto.Clone(canonicalDB).(*models.DatabasePresetRecord)
	staleDB.GetDatabase().GetParams().GetPostgres().MasterOptions["shared_buffers"] = "25%"
	staleDB.GetDatabase().GetParams().GetPostgres().ReplicaOptions["shared_buffers"] = "25%"

	current, err := buildBuiltinTestPresetRecord("tenant-1", "author-1", staleDB, workloads)
	if err != nil {
		t.Fatalf("build stale test preset: %v", err)
	}
	canonical, err := buildBuiltinTestPresetRecord("tenant-1", "author-1", canonicalDB, workloads)
	if err != nil {
		t.Fatalf("build canonical test preset: %v", err)
	}

	if !reconcileBuiltinTestPreset(current, canonical) {
		t.Fatal("reconcileBuiltinTestPreset reported no change for stale embedded database")
	}
	params := current.GetTest().GetDatabase().GetParams().GetPostgres()
	if got, want := params.GetMasterOptions()["shared_buffers"], "512MB"; got != want {
		t.Fatalf("master shared_buffers = %q, want %q", got, want)
	}
	if got, want := params.GetReplicaOptions()["shared_buffers"], "512MB"; got != want {
		t.Fatalf("replica shared_buffers = %q, want %q", got, want)
	}
}

func TestBuiltinSelfCheckMatrixBuildsSeededTopologies(t *testing.T) {
	workloads := workloadPresetsByProtocol(builtinWorkloadPresets("tenant-1", "author-1"))

	matrices := []struct {
		name     string
		provider deploymentpb.Provider
		include  func(*models.DatabasePresetRecord) bool
	}{
		{
			name:     "yandex",
			provider: deploymentpb.Provider_PROVIDER_YANDEX,
			include:  func(*models.DatabasePresetRecord) bool { return true },
		},
		{
			name:     "docker",
			provider: deploymentpb.Provider_PROVIDER_DOCKER,
			include: func(preset *models.DatabasePresetRecord) bool {
				return preset.GetDatabase().GetKind() != domain.Database_KIND_YDB_MANAGED
			},
		},
	}

	for _, matrix := range matrices {
		t.Run(matrix.name, func(t *testing.T) {
			built := 0
			for _, preset := range builtinDatabasePresets("tenant-1", "author-1") {
				if !matrix.include(preset) {
					continue
				}
				protocol := workloadProtocolForDatabase(preset.GetDatabase().GetKind())
				workload := workloads[protocol]
				if workload == nil {
					t.Fatalf("%q has no workload for protocol %s", preset.GetEntity().GetName(), protocol)
				}

				db := cloneDatabaseWithBuiltinPackage(preset.GetDatabase())
				pkg := db.GetParams().GetPackage()
				if db.GetKind() == domain.Database_KIND_YDB_MANAGED {
					if pkg != nil {
						t.Fatalf("%q managed YDB self-check database has package %+v", preset.GetEntity().GetName(), pkg)
					}
				} else if pkg == nil {
					t.Fatalf("%q self-check database has no builtin package", preset.GetEntity().GetName())
				} else if !pkg.GetIsBuiltin() {
					t.Fatalf("%q self-check database package is not builtin", preset.GetEntity().GetName())
				}

				run, err := runbuilder.BuildTestRun(runbuilder.BuildOptions{
					ID:             "seed-preview-" + preset.GetEntity().GetId(),
					Database:       db,
					Workload:       workload.GetWorkload(),
					Provider:       matrix.provider,
					Infrastructure: builtinSuiteInfrastructureOptions(matrix.provider),
				})
				if err != nil {
					t.Fatalf("%q does not build as %s self-check suite cell: %v", preset.GetEntity().GetName(), matrix.name, err)
				}
				if len(run.GetInfrastructurePlan().GetMachines()) == 0 {
					t.Fatalf("%q built no runnable machines", preset.GetEntity().GetName())
				}
				runPkg := run.GetDatabase().GetParams().GetPackage()
				if db.GetKind() == domain.Database_KIND_YDB_MANAGED {
					if runPkg != nil {
						t.Fatalf("%q managed YDB run has package %+v", preset.GetEntity().GetName(), runPkg)
					}
					continue
				}
				if runPkg == nil {
					t.Fatalf("%q run lost builtin package", preset.GetEntity().GetName())
				}
				if got := runPkg.GetId(); got != pkg.GetId() {
					t.Fatalf("%q run lost builtin package: got %q, want %q", preset.GetEntity().GetName(), got, pkg.GetId())
				}
				built++
			}
			if built == 0 {
				t.Fatal("self-check matrix built no cells")
			}
		})
	}
}

func TestBuiltinDockerSelfCheckSuiteUsesDockerCompatibleTests(t *testing.T) {
	tests := builtinSelfCheckTestRecords(t)
	kindByTestID := make(map[string]domain.Database_Kind, len(tests))
	for _, test := range tests {
		kind := test.GetTest().GetDatabase().GetKind()
		kindByTestID[test.GetEntity().GetId()] = kind
	}
	expectedCells := dockerCompatibleSelfCheckTests(t, tests)

	rec, err := buildBuiltinSuiteRecord(builtinSuiteSeed{
		name:        builtinDockerSelfCheckSuiteName,
		description: "Docker self-check",
		provider:    deploymentpb.Provider_PROVIDER_DOCKER,
		include:     includeDockerSuiteTest,
	}, "tenant-1", "author-1", tests)
	if err != nil {
		t.Fatalf("build docker builtin suite: %v", err)
	}
	if got := rec.GetSpec().GetProvider(); got != deploymentpb.Provider_PROVIDER_DOCKER {
		t.Fatalf("docker suite provider = %s, want %s", got, deploymentpb.Provider_PROVIDER_DOCKER)
	}
	if got, want := len(rec.GetSpec().GetCells()), expectedCells; got != want {
		t.Fatalf("docker suite cells = %d, want %d", got, want)
	}
	for _, cell := range rec.GetSpec().GetCells() {
		testID := cell.GetTestPresetId()
		if kindByTestID[testID] == domain.Database_KIND_YDB_MANAGED {
			t.Fatalf("docker suite contains managed YDB test %s", testID)
		}
		if len(cell.GetMachineOverrides()) == 0 {
			t.Fatalf("docker suite cell %q has no machine overrides", cell.GetName())
		}
		for _, machine := range cell.GetMachineOverrides() {
			if machine.GetDocker() == nil {
				t.Fatalf("docker suite cell %q has non-docker machine override for node %q", cell.GetName(), machine.GetNodeId())
			}
		}
		if got, limit := dockerDiskQuotaTotal(cell.GetMachineOverrides()), uint64(96); got > limit {
			t.Fatalf("docker suite cell %q disk quota = %d GiB, want <= %d GiB", cell.GetName(), got, limit)
		}
	}
}

func dockerCompatibleSelfCheckTests(t *testing.T, tests []*models.TestPresetRecord) int {
	t.Helper()

	var count int
	for _, test := range tests {
		if !includeDockerSuiteTest(test) {
			continue
		}
		run, err := runbuilder.BuildTestRun(runbuilder.BuildOptions{
			ID:             "docker-fit-" + test.GetEntity().GetId(),
			Database:       test.GetTest().GetDatabase(),
			Workload:       test.GetTest().GetWorkload(),
			Provider:       deploymentpb.Provider_PROVIDER_DOCKER,
			Infrastructure: builtinSuiteInfrastructureOptions(deploymentpb.Provider_PROVIDER_DOCKER),
		})
		if err != nil {
			t.Fatalf("build docker fit run for %q: %v", test.GetEntity().GetName(), err)
		}
		if dockerSelfCheckRunFits(run) {
			count++
		}
	}
	return count
}

func dockerDiskQuotaTotal(machines []*deploymentpb.MachinePlan) uint64 {
	var total uint64
	for _, machine := range machines {
		for _, req := range machine.GetQuotaRequests() {
			info := req.GetInfo()
			if info.GetProvider() == deploymentpb.Provider_PROVIDER_DOCKER && info.GetName() == "host.disk.size" {
				total += req.GetRequest()
			}
		}
	}
	return total
}

func builtinSelfCheckTestRecords(t *testing.T) []*models.TestPresetRecord {
	t.Helper()
	workloads := workloadPresetsByProtocol(builtinWorkloadPresets("tenant-1", "author-1"))
	out := make([]*models.TestPresetRecord, 0)
	for _, dbPreset := range builtinDatabasePresets("tenant-1", "author-1") {
		protocol := workloadProtocolForDatabase(dbPreset.GetDatabase().GetKind())
		workload := workloads[protocol]
		if workload == nil {
			t.Fatalf("%q has no workload for protocol %s", dbPreset.GetEntity().GetName(), protocol)
		}
		out = append(out, &models.TestPresetRecord{
			Entity:   newSeedEntity("tenant-1", "author-1", selfCheckTestPresetName(dbPreset), "test"),
			IsSystem: true,
			Test: &domain.Test{
				Database: cloneDatabaseWithBuiltinPackage(dbPreset.GetDatabase()),
				Workload: workload.GetWorkload(),
			},
		})
	}
	return out
}

func firstDatabasePresetFor(t *testing.T, presets []*models.DatabasePresetRecord, kind domain.Database_Kind) *models.DatabasePresetRecord {
	t.Helper()
	for _, preset := range presets {
		if preset.GetDatabase().GetKind() == kind {
			return preset
		}
	}
	t.Fatalf("database preset catalog has no kind %s", kind)
	return nil
}

func databasePresetByName(t *testing.T, presets []*models.DatabasePresetRecord, name string) *models.DatabasePresetRecord {
	t.Helper()
	for _, preset := range presets {
		if preset.GetEntity().GetName() == name {
			return preset
		}
	}
	t.Fatalf("database preset catalog has no preset %q", name)
	return nil
}

func firstDatabasePresetExcept(t *testing.T, presets []*models.DatabasePresetRecord, excluded domain.Database_Kind) *models.DatabasePresetRecord {
	t.Helper()
	for _, preset := range presets {
		if preset.GetDatabase().GetKind() != excluded {
			return preset
		}
	}
	t.Fatalf("database preset catalog has no preset except %s", excluded)
	return nil
}
