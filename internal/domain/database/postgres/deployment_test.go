package postgres

import (
	"strings"
	"testing"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/packages"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

func TestPostgresDeploymentRendererRendersPrioritiesDependenciesAndSteps(t *testing.T) {
	db := postgresDeploymentDatabase()
	spec, err := (&Database{}).BuildTopologySpec(db.GetParams().GetPostgres())
	if err != nil {
		t.Fatalf("build topology spec: %v", err)
	}

	plan, err := deploymentbuilder.BuildPlan(spec, postgresInfrastructureStateForSpec(spec), deploymentbuilder.BuildOptions{
		Database:        db,
		PackageResolver: packages.NewRegistry(PackageResolver{}),
		Renderers:       deploymentbuilder.NewRegistry(DeploymentRenderer{}),
	})
	if err != nil {
		t.Fatalf("build deployment plan: %v", err)
	}

	if err := plan.Validate(); err != nil {
		t.Fatalf("plan is invalid: %v", err)
	}

	components := deploymentComponentsByID(plan)
	if got, want := components["postgres-master-etcd"].GetGlobalPriority(), uint32(10); got != want {
		t.Fatalf("etcd global priority = %d, want %d", got, want)
	}
	if got, want := components["postgres-master"].GetGlobalPriority(), uint32(20); got != want {
		t.Fatalf("master global priority = %d, want %d", got, want)
	}
	if !deploymentHasDependency(components["postgres-master-pgbouncer"], "postgres-master") {
		t.Fatal("pgbouncer does not depend on postgres-master")
	}
	if !deploymentHasDependency(components["haproxy-1"], "postgres-master-pgbouncer") {
		t.Fatal("haproxy does not depend on postgres-master-pgbouncer")
	}

	master := components["postgres-master"]
	context := findDeploymentWriteFileText(t, master, "020_write_context")
	if !strings.Contains(context, "ENDPOINT_PRIVATE_ADDRESS='10.0.0.1'") {
		t.Fatalf("context does not contain private endpoint: %s", context)
	}
	config := findDeploymentWriteFileText(t, master, "030_write_config")
	if !strings.Contains(config, "max_connections = 200") {
		t.Fatalf("config does not contain rendered postgres options: %s", config)
	}

	install := findDeploymentCallCmdContaining(t, master, "postgresql-16")
	if !strings.Contains(install, "postgresql-16") {
		t.Fatalf("install command does not use resolved package: %s", install)
	}
	hba := findDeploymentWriteFileText(t, master, "040_write_hba")
	if !pgHBARuleExists(hba, "host", "all", "all", "0.0.0.0/0", "scram-sha-256") {
		t.Fatalf("pg_hba does not require scram for client access: %s", hba)
	}
	if !pgHBARuleExists(hba, "host", "replication", "replicator", "10.0.0.0/8", "scram-sha-256") {
		t.Fatalf("pg_hba does not contain scram replication access: %s", hba)
	}
	if strings.Contains(hba, "0.0.0.0/0 trust") {
		t.Fatalf("pg_hba must not trust replication from anywhere: %s", hba)
	}
	service := findDeploymentWriteFileText(t, master, "200_write_service")
	for _, want := range []string{
		"ExecStartPre=/bin/install -d -m 0755 '/var/lib/stroppy-cloud' '/run/stroppy-cloud'",
		"/usr/sbin/runuser -u postgres",
		"initdb",
		"postgres",
		"-c config_file='/etc/stroppy-cloud/postgres-master/postgresql.conf'",
		"-c hba_file='/etc/stroppy-cloud/postgres-master/pg_hba.conf'",
	} {
		if !strings.Contains(service, want) {
			t.Fatalf("service does not contain %q:\n%s", want, service)
		}
	}
	if strings.Contains(service, "is prepared with config") {
		t.Fatalf("service still contains placeholder unit: %s", service)
	}
	healthcheck := findDeploymentCallCmd(t, master, "230_healthcheck")
	if !strings.Contains(healthcheck, "pg_isready -h 127.0.0.1 -p 5432") {
		t.Fatalf("healthcheck is not postgres readiness check: %s", healthcheck)
	}
}

func TestPostgresDeploymentWiresClusterPeers(t *testing.T) {
	db := postgresDeploymentDatabase()
	spec, err := (&Database{}).BuildTopologySpec(db.GetParams().GetPostgres())
	if err != nil {
		t.Fatalf("build topology spec: %v", err)
	}

	plan, err := deploymentbuilder.BuildPlan(spec, postgresInfrastructureStateForSpec(spec), deploymentbuilder.BuildOptions{
		Database:        db,
		PackageResolver: packages.NewRegistry(PackageResolver{}),
		Renderers:       deploymentbuilder.NewRegistry(DeploymentRenderer{}),
	})
	if err != nil {
		t.Fatalf("build deployment plan: %v", err)
	}

	components := deploymentComponentsByID(plan)

	replica := findDeploymentWriteFileText(t, components["postgres-replica-1"], "200_write_service")
	for _, want := range []string{"pg_basebackup", "-h 10.0.0.1 -p 5432", "-U 'replicator'", "-R", "PGPASSFILE="} {
		if !strings.Contains(replica, want) {
			t.Fatalf("replica unit missing %q:\n%s", want, replica)
		}
	}
	pgpass := findDeploymentWriteFile(t, components["postgres-replica-1"], "045_write_pgpass")
	if got := pgpass.GetInfo().GetMode(); got != 0600 {
		t.Fatalf(".pgpass mode = %o, want 0600", got)
	}
	if !strings.Contains(pgpass.GetText(), "replicator:stroppy_replication") {
		t.Fatalf(".pgpass missing replication credential: %s", pgpass.GetText())
	}
	master := findDeploymentWriteFileText(t, components["postgres-master"], "200_write_service")
	if !strings.Contains(master, "replication-setup.sql") {
		t.Fatalf("master unit does not provision the replication role:\n%s", master)
	}
	if strings.Contains(master, "replication-setup.sql' || true") {
		t.Fatalf("master unit ignores setup failure:\n%s", master)
	}
	if !strings.Contains(master, "pg_isready -U postgres -h 127.0.0.1 -p 5432") {
		t.Fatalf("master unit readiness check does not use the postgres role:\n%s", master)
	}
	if !strings.Contains(master, "psql -v ON_ERROR_STOP=1 -U postgres -h 127.0.0.1 -p 5432 < '/etc/stroppy-cloud/postgres-master/replication-setup.sql'") {
		t.Fatalf("master unit does not stop on setup SQL errors:\n%s", master)
	}
	if strings.Contains(master, "psql -v ON_ERROR_STOP=1 -h 127.0.0.1 -p 5432 -f") {
		t.Fatalf("master unit runs psql -f on a root-owned setup file:\n%s", master)
	}
	setupFile := findDeploymentWriteFile(t, components["postgres-master"], "050_write_replication_setup")
	if got := setupFile.GetInfo().GetMode(); got != 0640 {
		t.Fatalf("replication setup mode = %o, want 0640", got)
	}
	setup := setupFile.GetText()
	for _, want := range []string{
		"EXECUTE format('CREATE ROLE %I WITH REPLICATION LOGIN PASSWORD %L', 'replicator', 'stroppy_replication')",
		"END;\n$$;",
		"SET password_encryption = 'scram-sha-256'",
		"ALTER ROLE postgres WITH LOGIN PASSWORD 'stroppy_postgres'",
	} {
		if !strings.Contains(setup, want) {
			t.Fatalf("setup SQL missing %q:\n%s", want, setup)
		}
	}

	haproxy := findDeploymentWriteFileText(t, components["haproxy-1"], "030_write_config")
	for _, want := range []string{"server pg-1 10.0.0.1:6432 check", "server pg-2 10.0.0.2:6432 check"} {
		if !strings.Contains(haproxy, want) {
			t.Fatalf("haproxy backends missing %q:\n%s", want, haproxy)
		}
	}

	etcd := findDeploymentWriteFileText(t, components["postgres-master-etcd"], "200_write_service")
	for _, want := range []string{
		"ExecStartPre=/bin/install -d -m 0755 '/var/lib/stroppy-cloud'",
		"--initial-cluster",
		"postgres-master-etcd=http://10.0.0.1:2380",
		"postgres-replica-1-etcd=http://10.0.0.2:2380",
	} {
		if !strings.Contains(etcd, want) {
			t.Fatalf("etcd unit missing %q:\n%s", want, etcd)
		}
	}
}

func TestPostgresDeploymentRendererBuildsRenderPreview(t *testing.T) {
	db := postgresDeploymentDatabase()
	spec, err := (&Database{}).BuildTopologySpec(db.GetParams().GetPostgres())
	if err != nil {
		t.Fatalf("build topology spec: %v", err)
	}

	preview, err := deploymentbuilder.BuildPreview(spec, deploymentbuilder.PreviewOptions{
		Database:        db,
		PackageResolver: packages.NewRegistry(PackageResolver{}),
		Renderers:       deploymentbuilder.NewRegistry(DeploymentRenderer{}),
	})
	if err != nil {
		t.Fatalf("build render preview: %v", err)
	}

	if err := preview.Validate(); err != nil {
		t.Fatalf("preview is invalid: %v", err)
	}

	config := findRenderArtifact(t, preview, "postgres-master/postgresql.conf")
	if got, want := config.GetOrigin(), deploymentpb.RenderArtifact_ORIGIN_RENDERED_DEFAULT; got != want {
		t.Fatalf("config origin = %s, want %s", got, want)
	}
	if got, want := config.GetMutability(), deploymentpb.RenderArtifact_MUTABILITY_EDITABLE; got != want {
		t.Fatalf("config mutability = %s, want %s", got, want)
	}
	if !strings.Contains(config.GetFile().GetText(), "max_connections = 200") {
		t.Fatalf("config preview does not contain rendered postgres options: %s", config.GetFile().GetText())
	}

	runtime := findRenderArtifact(t, preview, "postgres-master/runtime/private-address")
	if got, want := runtime.GetMutability(), deploymentpb.RenderArtifact_MUTABILITY_RUNTIME_ONLY; got != want {
		t.Fatalf("runtime mutability = %s, want %s", got, want)
	}
	if got := runtime.GetRuntimeValue(); got != "machine.postgres-master.endpoint.private.address" {
		t.Fatalf("runtime value = %q", got)
	}

	install := findRenderCommandArtifactContaining(t, preview, "postgres-master/install/", "postgresql-16")
	if got, want := install.GetMutability(), deploymentpb.RenderArtifact_MUTABILITY_READ_ONLY; got != want {
		t.Fatalf("install mutability = %s, want %s", got, want)
	}
	if !strings.Contains(install.GetCmd().GetSpec().GetScript().GetText(), "postgresql-16") {
		t.Fatalf("install preview does not use resolved package: %s", install.GetCmd().GetSpec().GetScript().GetText())
	}
	service := findRenderArtifact(t, preview, "postgres-master/systemd.service")
	if strings.Contains(service.GetFile().GetText(), "is prepared with config") {
		t.Fatalf("service preview still contains placeholder unit: %s", service.GetFile().GetText())
	}
	if !strings.Contains(service.GetFile().GetText(), "/usr/sbin/runuser -u postgres") {
		t.Fatalf("service preview does not run postgres: %s", service.GetFile().GetText())
	}
}

func TestPostgresDeploymentRendererAppliesRenderOverrides(t *testing.T) {
	db := postgresDeploymentDatabase()
	spec, err := (&Database{}).BuildTopologySpec(db.GetParams().GetPostgres())
	if err != nil {
		t.Fatalf("build topology spec: %v", err)
	}

	master := componentsByID(spec)["postgres-master"]
	overrides := &deploymentpb.RenderOverrideSet{
		Files: []*deploymentpb.FileOverride{
			{
				ArtifactId:  "postgres-master/postgresql.conf",
				ComponentId: "postgres-master",
				BaseHash:    deploymentbuilder.FileHash(postgresDefaultConfigFile(master, db, postgresWiring{})),
				File: &common.File{
					Info: &common.File_Info{
						Path:          "/etc/stroppy-cloud/postgres-master/postgresql.conf",
						Mode:          0644,
						CreateParents: true,
					},
					Content: &common.File_Text{Text: "shared_buffers = 1GB\n"},
				},
			},
		},
	}

	preview, err := deploymentbuilder.BuildPreview(spec, deploymentbuilder.PreviewOptions{
		Database:        db,
		PackageResolver: packages.NewRegistry(PackageResolver{}),
		Renderers:       deploymentbuilder.NewRegistry(DeploymentRenderer{}),
		RenderOverrides: overrides,
	})
	if err != nil {
		t.Fatalf("build render preview: %v", err)
	}
	config := findRenderArtifact(t, preview, "postgres-master/postgresql.conf")
	if got, want := config.GetOrigin(), deploymentpb.RenderArtifact_ORIGIN_USER_OVERRIDE; got != want {
		t.Fatalf("config origin = %s, want %s", got, want)
	}
	if got := config.GetFile().GetText(); got != "shared_buffers = 1GB\n" {
		t.Fatalf("config preview text = %q", got)
	}

	plan, err := deploymentbuilder.BuildPlan(spec, postgresInfrastructureStateForSpec(spec), deploymentbuilder.BuildOptions{
		Database:        db,
		PackageResolver: packages.NewRegistry(PackageResolver{}),
		Renderers:       deploymentbuilder.NewRegistry(DeploymentRenderer{}),
		RenderOverrides: overrides,
	})
	if err != nil {
		t.Fatalf("build deployment plan: %v", err)
	}
	deployment := deploymentComponentsByID(plan)["postgres-master"]
	if got := findDeploymentWriteFileText(t, deployment, "030_write_config"); got != "shared_buffers = 1GB\n" {
		t.Fatalf("deployment config text = %q", got)
	}
}

func TestPostgresPackageResolverResolvesVersion(t *testing.T) {
	pkg, err := PackageResolver{}.ResolveDatabasePackage(postgresDeploymentDatabase())
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if got, want := pkg.GetId(), "builtin/postgres/16"; got != want {
		t.Fatalf("package id = %q, want %q", got, want)
	}
	if got, want := pkg.GetAptPackages()[0], "postgresql-16"; got != want {
		t.Fatalf("first apt package = %q, want %q", got, want)
	}
}

func postgresDeploymentDatabase() *domain.Database {
	return &domain.Database{
		Kind: domain.Database_KIND_POSTGRES,
		Source: &domain.Database_Params{
			Params: &domain.DatabaseParams{
				Version: "16",
				Engine: &domain.DatabaseParams_Postgres{
					Postgres: &domain.PostgresParams{
						Replicas:  1,
						Haproxy:   1,
						Pgbouncer: true,
						Patroni:   true,
						MasterOptions: map[string]string{
							"max_connections": "200",
						},
					},
				},
			},
		},
	}
}

func postgresInfrastructureStateForSpec(spec *topologypb.TopologySpec) *deploymentpb.InfrastructureState {
	state := &deploymentpb.InfrastructureState{
		Provider: deploymentpb.Provider_PROVIDER_DOCKER,
		Machines: make([]*deploymentpb.MachineState, 0, len(spec.GetNodes())),
		Tags:     &common.Tags{Tags: []string{"test"}},
	}

	for i, node := range spec.GetNodes() {
		privatePort := uint32(22)
		state.Machines = append(state.Machines, &deploymentpb.MachineState{
			NodeId:             node.GetId(),
			ProviderResourceId: "container-" + node.GetId(),
			Status:             common.Status_STATUS_DEPLOYED,
			Endpoints: []*deploymentpb.Endpoint{
				{
					Name:    "private",
					Address: "10.0.0." + string(rune('1'+i)),
					Port:    &privatePort,
				},
			},
		})
	}

	return state
}

func deploymentComponentsByID(plan *deploymentpb.DeploymentPlan) map[string]*deploymentpb.ComponentDeployment {
	components := make(map[string]*deploymentpb.ComponentDeployment, len(plan.GetComponents()))
	for _, component := range plan.GetComponents() {
		components[component.GetComponentId()] = component
	}
	return components
}

func deploymentHasDependency(component *deploymentpb.ComponentDeployment, dependency string) bool {
	for _, candidate := range component.GetDependsOnComponentIds() {
		if candidate == dependency {
			return true
		}
	}
	return false
}

func pgHBARuleExists(hba string, fields ...string) bool {
	for _, line := range strings.Split(hba, "\n") {
		if strings.Join(strings.Fields(line), " ") == strings.Join(fields, " ") {
			return true
		}
	}
	return false
}

func findDeploymentWriteFileText(t *testing.T, component *deploymentpb.ComponentDeployment, stepID string) string {
	t.Helper()

	for _, step := range component.GetSteps() {
		if step.GetId() == stepID {
			return step.GetWriteFile().GetText()
		}
	}
	t.Fatalf("step %q is missing", stepID)
	return ""
}

func findDeploymentWriteFile(t *testing.T, component *deploymentpb.ComponentDeployment, stepID string) *common.File {
	t.Helper()

	for _, step := range component.GetSteps() {
		if step.GetId() == stepID {
			return step.GetWriteFile()
		}
	}
	t.Fatalf("step %q is missing", stepID)
	return nil
}

func findDeploymentCallCmd(t *testing.T, component *deploymentpb.ComponentDeployment, stepID string) string {
	t.Helper()

	for _, step := range component.GetSteps() {
		if step.GetId() == stepID {
			return step.GetCallCmd().GetSpec().GetScript().GetText()
		}
	}
	t.Fatalf("step %q is missing", stepID)
	return ""
}

func findDeploymentCallCmdContaining(t *testing.T, component *deploymentpb.ComponentDeployment, needle string) string {
	t.Helper()

	for _, step := range component.GetSteps() {
		text := step.GetCallCmd().GetSpec().GetScript().GetText()
		if strings.Contains(text, needle) {
			return text
		}
	}
	t.Fatalf("call command containing %q is missing", needle)
	return ""
}

func findRenderArtifact(t *testing.T, preview *deploymentpb.RenderPreview, artifactID string) *deploymentpb.RenderArtifact {
	t.Helper()

	for _, artifact := range preview.GetArtifacts() {
		if artifact.GetId() == artifactID {
			return artifact
		}
	}
	t.Fatalf("artifact %q is missing", artifactID)
	return nil
}

func findRenderCommandArtifactContaining(t *testing.T, preview *deploymentpb.RenderPreview, idPrefix, needle string) *deploymentpb.RenderArtifact {
	t.Helper()

	for _, artifact := range preview.GetArtifacts() {
		if strings.HasPrefix(artifact.GetId(), idPrefix) &&
			strings.Contains(artifact.GetCmd().GetSpec().GetScript().GetText(), needle) {
			return artifact
		}
	}
	t.Fatalf("command artifact with prefix %q containing %q is missing", idPrefix, needle)
	return nil
}
