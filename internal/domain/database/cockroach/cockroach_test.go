package cockroach

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/database/dbtest"
	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/packages"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

func cockroachDatabase() *domain.Database {
	return &domain.Database{
		Kind: domain.Database_KIND_COCKROACH,
		Source: &domain.Database_Params{
			Params: &domain.DatabaseParams{
				Engine: &domain.DatabaseParams_Cockroach{
					Cockroach: &domain.CockroachParams{
						Nodes: 2,
						Options: map[string]string{
							"flag:cache":                     "1GB",
							"kv.range_split.by_load_enabled": "true",
						},
					},
				},
			},
		},
	}
}

func TestCockroachBuildTopologySpec(t *testing.T) {
	db := cockroachDatabase()
	spec, err := (&Database{}).BuildTopologySpec(db.GetParams().GetCockroach())
	if err != nil {
		t.Fatalf("build topology spec: %v", err)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("spec invalid: %v", err)
	}
	if got, want := len(spec.GetNodes()), 2; got != want {
		t.Fatalf("nodes = %d, want %d", got, want)
	}
	if !dbtest.HasConnection(spec, "cockroach-node-2", "cockroach-node-1", "rpc") {
		t.Fatal("node-2 does not join node-1 over rpc")
	}
}

func TestCockroachDeploymentPlan(t *testing.T) {
	db := cockroachDatabase()
	spec, err := (&Database{}).BuildTopologySpec(db.GetParams().GetCockroach())
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

	node := dbtest.ComponentsByID(plan)["cockroach-node-1"]
	service := dbtest.ServiceUnitText(node)
	for _, want := range []string{
		"cockroach start ",
		"--advertise-addr='10.0.0.1:26257'",
		"--join='10.0.0.1:26257,10.0.0.2:26257'",
		"cockroach init --insecure --host='10.0.0.1:26257'",
	} {
		if !strings.Contains(service, want) {
			t.Fatalf("service missing %q:\n%s", want, service)
		}
	}
	if strings.Contains(service, "is prepared with config") {
		t.Fatalf("service still contains placeholder unit: %s", service)
	}
	// node-2 joins but is not the init node.
	node2 := dbtest.ServiceUnitText(dbtest.ComponentsByID(plan)["cockroach-node-2"])
	if strings.Contains(node2, "cockroach init") {
		t.Fatalf("non-first node must not run cockroach init:\n%s", node2)
	}
	flags := dbtest.WriteFileText(node, "030_write_config")
	if !strings.Contains(flags, "--cache=1GB") {
		t.Fatalf("flags missing startup flag: %s", flags)
	}
	settings := dbtest.WriteFileText(node, "040_write_cluster_settings")
	if !strings.Contains(settings, "SET CLUSTER SETTING kv.range_split.by_load_enabled = 'true';") {
		t.Fatalf("cluster settings missing: %s", settings)
	}
}

func TestCockroachPackageResolver(t *testing.T) {
	pkg, err := PackageResolver{}.ResolveDatabasePackage(cockroachDatabase())
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if got, want := pkg.GetId(), "builtin/cockroach/default"; got != want {
		t.Fatalf("package id = %q, want %q", got, want)
	}
	if !strings.Contains(pkg.GetDebFilename(), "${STROPPY_SERVER_ADDR%/}/api/binaries/cockroach/23.2.5/") {
		t.Fatalf("package download url = %q", pkg.GetDebFilename())
	}
}
