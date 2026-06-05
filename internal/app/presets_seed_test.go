package app

import (
	"testing"

	runbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
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
	preset := builtinDatabasePresets("tenant-1", "author-1")[0]
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
		if pkg == nil {
			t.Fatalf("%q self-check database has no builtin package", preset.GetEntity().GetName())
		}
		if !pkg.GetIsBuiltin() {
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
		if got := run.GetDatabase().GetParams().GetPackage().GetId(); got != pkg.GetId() {
			t.Fatalf("%q run lost builtin package: got %q, want %q", preset.GetEntity().GetName(), got, pkg.GetId())
		}
	}
}
