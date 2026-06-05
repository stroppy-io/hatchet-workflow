package app

import (
	"testing"

	runbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
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

	if !reconcileBuiltinDatabasePreset(preset) {
		t.Fatal("reconcileBuiltinDatabasePreset reported no change for missing package")
	}
	pkg := preset.GetDatabase().GetParams().GetPackage()
	if pkg == nil {
		t.Fatal("reconcileBuiltinDatabasePreset did not restore package")
	}
	if !pkg.GetIsBuiltin() {
		t.Fatalf("restored package is not builtin: %+v", pkg)
	}
	if reconcileBuiltinDatabasePreset(preset) {
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

	if !reconcileBuiltinDatabasePreset(preset) {
		t.Fatal("reconcileBuiltinDatabasePreset reported no change for managed YDB package")
	}
	if pkg := preset.GetDatabase().GetParams().GetPackage(); pkg != nil {
		t.Fatalf("managed YDB package was not removed: %+v", pkg)
	}
	if reconcileBuiltinDatabasePreset(preset) {
		t.Fatal("reconcileBuiltinDatabasePreset changed an already reconciled managed YDB preset")
	}
}

func TestBuiltinSelfCheckMatrixBuildsEverySeededTopology(t *testing.T) {
	workloads := workloadPresetsByProtocol(builtinWorkloadPresets("tenant-1", "author-1"))

	for _, preset := range builtinDatabasePresets("tenant-1", "author-1") {
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
			ID:       "seed-preview-" + preset.GetEntity().GetId(),
			Database: db,
			Workload: workload.GetWorkload(),
			Provider: deploymentpb.Provider_PROVIDER_YANDEX,
		})
		if err != nil {
			t.Fatalf("%q does not build as self-check suite cell: %v", preset.GetEntity().GetName(), err)
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
	}
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
