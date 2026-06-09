package picodata

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/database/dbtest"
	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/packages"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

func picodataDatabase() *domain.Database {
	return &domain.Database{
		Kind: domain.Database_KIND_PICODATA,
		Source: &domain.Database_Params{
			Params: &domain.DatabaseParams{
				Version: "25.3",
				Engine: &domain.DatabaseParams_Picodata{
					Picodata: &domain.PicodataParams{
						Instances:         2,
						Haproxy:           1,
						ReplicationFactor: 2,
						InstanceOptions:   map[string]string{"memtx_memory": "2147483648"},
					},
				},
			},
		},
	}
}

func TestPicodataBuildTopologySpec(t *testing.T) {
	db := picodataDatabase()
	spec, err := (&Database{}).BuildTopologySpec(db.GetParams().GetPicodata())
	if err != nil {
		t.Fatalf("build topology spec: %v", err)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("spec invalid: %v", err)
	}
	if got, want := len(spec.GetNodes()), 3; got != want {
		t.Fatalf("nodes = %d, want %d", got, want)
	}

	components := dbtest.ComponentsByIDInSpec(spec)
	if components["picodata-instance-1"].GetKind() != topology.Component_KIND_DATABASE {
		t.Fatalf("instance-1 is not a database component")
	}
	if components["haproxy-1"].GetKind() != topology.Component_KIND_PROXY {
		t.Fatalf("haproxy-1 is not a proxy component")
	}
	if !dbtest.HasConnection(spec, "picodata-instance-2", "picodata-instance-1", "iproto") {
		t.Fatal("instance-2 does not join instance-1 over iproto")
	}
	if !dbtest.HasConnection(spec, "haproxy-1", "picodata-instance-1", "pgproto") {
		t.Fatal("haproxy-1 does not front instance-1 pgproto")
	}
}

func TestPicodataDeploymentPlan(t *testing.T) {
	db := picodataDatabase()
	spec, err := (&Database{}).BuildTopologySpec(db.GetParams().GetPicodata())
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
	instance := components["picodata-instance-1"]
	installScripts := strings.Join([]string{
		dbtest.CallCmd(instance, "100_install"),
		dbtest.CallCmd(instance, "110_install"),
		dbtest.CallCmd(instance, "120_install"),
		dbtest.CallCmd(instance, "130_install"),
		dbtest.CallCmd(instance, "140_install"),
	}, "\n")
	for _, want := range []string{
		"https://download.picodata.io/tarantool-picodata/picodata.gpg.key",
		"http://download.picodata.io/tarantool-picodata/%s/",
		"/etc/apt/sources.list.d/picodata.list",
		"DEBIAN_FRONTEND=noninteractive apt-get install -y picodata",
	} {
		if !strings.Contains(installScripts, want) {
			t.Fatalf("instance-1 install scripts missing %q:\n%s", want, installScripts)
		}
	}

	service := dbtest.ServiceUnitText(instance)
	for _, want := range []string{"/usr/bin/picodata run --config", "Environment=PICODATA_ADMIN_PASSWORD=T0psecret", "Restart=always"} {
		if !strings.Contains(service, want) {
			t.Fatalf("instance-1 service missing %q:\n%s", want, service)
		}
	}
	for _, unwanted := range []string{"--advertise", "--peer", "--data-dir"} {
		if strings.Contains(service, unwanted) {
			t.Fatalf("service still contains CLI runtime option %q:\n%s", unwanted, service)
		}
	}
	if strings.Contains(service, "is prepared with config") {
		t.Fatalf("service still contains placeholder unit: %s", service)
	}
	config := dbtest.WriteFileText(instance, "030_write_config")
	for _, want := range []string{
		"replication_factor: 2",
		"instance_dir: '/var/lib/stroppy-cloud/picodata-instance-1'",
		"name: 'picodata_instance_1'",
		"peer:\n    - '10.0.0.1:3301'",
		"iproto:\n    enabled: true\n    listen: '0.0.0.0:3301'\n    advertise: '10.0.0.1:3301'",
		"pgproto:\n    enabled: true\n    listen: '0.0.0.0:5432'\n    advertise: '10.0.0.1:5432'",
		"memtx_memory: 2147483648",
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("config missing %q:\n%s", want, config)
		}
	}

	config2 := dbtest.WriteFileText(components["picodata-instance-2"], "030_write_config")
	if !strings.Contains(config2, "peer:\n    - '10.0.0.1:3301'") {
		t.Fatalf("instance-2 does not join bootstrap:\n%s", config2)
	}
	service2 := dbtest.ServiceUnitText(components["picodata-instance-2"])
	if strings.Contains(service2, "PICODATA_ADMIN_PASSWORD") {
		t.Fatalf("non-bootstrap instance should not set bootstrap admin password:\n%s", service2)
	}

	haproxy := dbtest.WriteFileText(components["haproxy-1"], "030_write_config")
	for _, want := range []string{"server instance-1 10.0.0.1:5432 check", "server instance-2 10.0.0.2:5432 check"} {
		if !strings.Contains(haproxy, want) {
			t.Fatalf("haproxy backends missing %q:\n%s", want, haproxy)
		}
	}
}

func TestPicodataPackageResolver(t *testing.T) {
	pkg, err := PackageResolver{}.ResolveDatabasePackage(picodataDatabase())
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if got, want := pkg.GetId(), "builtin/picodata/25.3"; got != want {
		t.Fatalf("package id = %q, want %q", got, want)
	}
	preInstall := strings.Join(pkg.GetPreInstall(), "\n")
	for _, want := range []string{
		"https://download.picodata.io/tarantool-picodata/picodata.gpg.key",
		"http://download.picodata.io/tarantool-picodata/%s/",
		"/etc/apt/sources.list.d/picodata.list",
	} {
		if !strings.Contains(preInstall, want) {
			t.Fatalf("package preinstall missing %q:\n%s", want, preInstall)
		}
	}
}
