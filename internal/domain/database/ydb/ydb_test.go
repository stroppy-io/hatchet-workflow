package ydb

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/database/dbtest"
	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/packages"
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
	if !dbtest.HasConnection(spec, "ydb-database-1", "ydb-storage-1", "grpc") {
		t.Fatal("database-1 does not register against storage-1 over grpc")
	}
	if !dbtest.HasConnection(spec, "haproxy-1", "ydb-database-1", "grpc") {
		t.Fatal("haproxy-1 does not front database-1 grpc")
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
	for _, want := range []string{"/usr/local/bin/ydbd server", "--node static"} {
		if !strings.Contains(service, want) {
			t.Fatalf("service missing %q:\n%s", want, service)
		}
	}
	if strings.Contains(service, "is prepared with config") {
		t.Fatalf("service still contains placeholder unit: %s", service)
	}
	config := dbtest.WriteFileText(storage, "030_write_config")
	for _, want := range []string{"node_type: STORAGE", "static_erasure: block-4-2", "grpc_config", "host: 10.0.0.1", "host: 10.0.0.2"} {
		if !strings.Contains(config, want) {
			t.Fatalf("config missing %q:\n%s", want, config)
		}
	}
	if install := dbtest.CallCmd(storage, "100_install"); !strings.Contains(install, "ydbd") {
		t.Fatalf("install does not fetch ydbd: %s", install)
	}

	database := dbtest.ServiceUnitText(components["ydb-database-1"])
	for _, want := range []string{"--node dynamic", "--node-broker '10.0.0.1:2136'", "--node-broker '10.0.0.2:2136'"} {
		if !strings.Contains(database, want) {
			t.Fatalf("database service missing %q:\n%s", want, database)
		}
	}

	haproxy := dbtest.WriteFileText(components["haproxy-1"], "030_write_config")
	if !strings.Contains(haproxy, "server ydb-1 10.0.0.3:2136 check") {
		t.Fatalf("haproxy backends missing compute node:\n%s", haproxy)
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
	if !strings.Contains(pkg.GetDebFilename(), "binaries.ydb.tech") {
		t.Fatalf("package download url = %q", pkg.GetDebFilename())
	}
}
