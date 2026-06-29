package pgnoop

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/database/dbtest"
	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/packages"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

func pgnoopDatabase() *domain.Database {
	return &domain.Database{
		Kind: domain.Database_KIND_PG_NOOP,
		Source: &domain.Database_Params{
			Params: &domain.DatabaseParams{
				Engine: &domain.DatabaseParams_PgNoop{
					PgNoop: &domain.PgNoopParams{Workers: 8},
				},
			},
		},
	}
}

func TestPgNoopBuildTopologySpec(t *testing.T) {
	spec, err := (&Database{}).BuildTopologySpec(pgnoopDatabase().GetParams().GetPgNoop())
	if err != nil {
		t.Fatalf("build topology spec: %v", err)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("spec invalid: %v", err)
	}
	if got, want := len(spec.GetNodes()), 1; got != want {
		t.Fatalf("nodes = %d, want %d", got, want)
	}
	if got := len(spec.GetConnections()); got != 0 {
		t.Fatalf("connections = %d, want 0 (single node, fronts nothing)", got)
	}
}

func TestPgNoopDeploymentPlan(t *testing.T) {
	db := pgnoopDatabase()
	spec, err := (&Database{}).BuildTopologySpec(db.GetParams().GetPgNoop())
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

	node := dbtest.ComponentsByID(plan)[nodeID]
	service := dbtest.ServiceUnitText(node)
	for _, want := range []string{"ExecStart=/usr/local/bin/pgnoop", "pgnoop.env"} {
		if !strings.Contains(service, want) {
			t.Fatalf("service missing %q:\n%s", want, service)
		}
	}
	install := dbtest.CallCmd(node, "100_install")
	if !strings.Contains(install, "/api/binaries/pgnoop/0.1.2/pg-noop-x86_64-unknown-linux-musl.tar.xz") {
		t.Fatalf("install does not expand pg-noop binary URL: %s", install)
	}
	if !strings.Contains(install, "xz-utils") {
		t.Fatalf("install must ensure xz is present before tar -xJf: %s", install)
	}
	env := dbtest.WriteFileText(node, "030_write_config")
	for _, want := range []string{"PGNOOP_PORT=5432", "PGNOOP_WORKERS=8"} {
		if !strings.Contains(env, want) {
			t.Fatalf("env file missing %q:\n%s", want, env)
		}
	}
}

func TestPgNoopPackageResolver(t *testing.T) {
	pkg, err := PackageResolver{}.ResolveDatabasePackage(pgnoopDatabase())
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if got, want := pkg.GetId(), "builtin/pgnoop/default"; got != want {
		t.Fatalf("package id = %q, want %q", got, want)
	}
	if !strings.Contains(pkg.GetDebFilename(), "${STROPPY_SERVER_ADDR%/}/api/binaries/pgnoop/0.1.2/") {
		t.Fatalf("package download url = %q", pkg.GetDebFilename())
	}
}
