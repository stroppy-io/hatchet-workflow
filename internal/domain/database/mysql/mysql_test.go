package mysql

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/database/dbtest"
	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/packages"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

func mysqlDatabase() *domain.Database {
	return &domain.Database{
		Kind: domain.Database_KIND_MYSQL,
		Source: &domain.Database_Params{
			Params: &domain.DatabaseParams{
				Engine: &domain.DatabaseParams_Mysql{
					Mysql: &domain.MySqlParams{
						Replicas: 1,
						Proxysql: 1,
						SemiSync: true,
						PrimaryOptions: map[string]string{
							"max_connections": "500",
						},
					},
				},
			},
		},
	}
}

func TestMysqlBuildTopologySpec(t *testing.T) {
	db := mysqlDatabase()
	spec, err := (&Database{}).BuildTopologySpec(db.GetParams().GetMysql())
	if err != nil {
		t.Fatalf("build topology spec: %v", err)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("spec invalid: %v", err)
	}
	if got, want := len(spec.GetNodes()), 3; got != want {
		t.Fatalf("nodes = %d, want %d", got, want)
	}
	if dbtest.ComponentsByIDInSpec(spec)["mysql-replica-1"].GetKind() != topology.Component_KIND_REPLICA {
		t.Fatal("mysql-replica-1 is not a replica component")
	}
	if !dbtest.HasConnection(spec, "mysql-replica-1", "mysql-primary", "mysql") {
		t.Fatal("replica-1 does not replicate from primary")
	}
	if !dbtest.HasConnection(spec, "proxysql-1", "mysql-primary", "mysql") {
		t.Fatal("proxysql-1 does not front primary")
	}
}

func TestMysqlDeploymentPlan(t *testing.T) {
	db := mysqlDatabase()
	spec, err := (&Database{}).BuildTopologySpec(db.GetParams().GetMysql())
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
	primary := components["mysql-primary"]
	service := dbtest.ServiceUnitText(primary)
	for _, want := range []string{"/usr/sbin/mysqld", "initialize-insecure", "--defaults-file='/etc/mysql/stroppy-cloud/mysql-primary.cnf'"} {
		if !strings.Contains(service, want) {
			t.Fatalf("service missing %q:\n%s", want, service)
		}
	}
	for _, want := range []string{
		"ExecStartPre=/bin/mkdir -p '/var/lib/mysql'",
		"find '/var/lib/mysql' -mindepth 1 -maxdepth 1 -exec rm -rf {} +",
	} {
		if !strings.Contains(service, want) {
			t.Fatalf("service missing datadir bootstrap guard %q:\n%s", want, service)
		}
	}
	if strings.Contains(service, "/var/lib/stroppy-cloud/mysql-primary") {
		t.Fatalf("service uses apparmor-hostile custom mysql datadir:\n%s", service)
	}
	if strings.Contains(service, "is prepared with config") {
		t.Fatalf("service still contains placeholder unit: %s", service)
	}
	config := dbtest.WriteFileText(primary, "030_write_config")
	for _, want := range []string{"[mysqld]", "datadir = /var/lib/mysql", "server_id = 1", "max_connections = 500", "rpl_semi_sync_master_enabled = 1"} {
		if !strings.Contains(config, want) {
			t.Fatalf("config missing %q:\n%s", want, config)
		}
	}

	replica := components["mysql-replica-1"]
	replSQL := dbtest.WriteFileText(replica, "040_write_replication_sql")
	if !strings.Contains(replSQL, "SOURCE_HOST='10.0.0.1', SOURCE_PORT=3306") {
		t.Fatalf("replication.sql does not target primary:\n%s", replSQL)
	}
	if !strings.Contains(dbtest.ServiceUnitText(replica), "replication.sql") {
		t.Fatalf("replica service does not apply replication.sql")
	}

	proxysql := dbtest.WriteFileText(components["proxysql-1"], "030_write_config")
	for _, want := range []string{
		`{ address="10.0.0.1" , port=3306 , hostgroup=0 }`,
		`{ address="10.0.0.2" , port=3306 , hostgroup=1 }`,
	} {
		if !strings.Contains(proxysql, want) {
			t.Fatalf("proxysql mysql_servers missing %q:\n%s", want, proxysql)
		}
	}
}

func TestMysqlPackageResolver(t *testing.T) {
	pkg, err := PackageResolver{}.ResolveDatabasePackage(mysqlDatabase())
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if got, want := pkg.GetId(), "builtin/mysql/default"; got != want {
		t.Fatalf("package id = %q, want %q", got, want)
	}
}

func TestMysqlPackageResolverConfiguresVersionedRepo(t *testing.T) {
	db := mysqlDatabase()
	db.GetParams().Version = "8.4"

	pkg, err := PackageResolver{}.ResolveDatabasePackage(db)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if got, want := pkg.GetAptPackages()[0], "mysql-server"; got != want {
		t.Fatalf("apt package = %q, want %q", got, want)
	}
	preInstall := strings.Join(pkg.GetPreInstall(), "\n")
	for _, want := range []string{
		"http://repo.mysql.com/apt/ubuntu/",
		"mysql-8.4-lts",
		"RPM-GPG-KEY-mysql-2025",
		"apt-get update",
	} {
		if !strings.Contains(preInstall, want) {
			t.Fatalf("preinstall missing %q:\n%s", want, preInstall)
		}
	}
	if strings.Contains(preInstall, "mysql-server-8.4") {
		t.Fatalf("preinstall contains versioned apt package name:\n%s", preInstall)
	}
}

func TestMysqlPackageResolverSupportsMariaDB(t *testing.T) {
	db := mysqlDatabase()
	db.Kind = domain.Database_KIND_MARIADB
	db.GetParams().Engine = &domain.DatabaseParams_Mariadb{Mariadb: db.GetParams().GetMysql()}

	pkg, err := PackageResolver{}.ResolveDatabasePackage(db)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if got, want := pkg.GetId(), "builtin/mariadb/default"; got != want {
		t.Fatalf("package id = %q, want %q", got, want)
	}
	if got, want := pkg.GetDbKind(), domain.Database_KIND_MARIADB; got != want {
		t.Fatalf("db kind = %s, want %s", got, want)
	}
	if got, want := pkg.GetAptPackages()[0], "mariadb-server"; got != want {
		t.Fatalf("apt package = %q, want %q", got, want)
	}
}

func TestMysqlPackageResolverConfiguresMariaDBVersionedRepo(t *testing.T) {
	db := mysqlDatabase()
	db.Kind = domain.Database_KIND_MARIADB
	db.GetParams().Version = "10.11"
	db.GetParams().Engine = &domain.DatabaseParams_Mariadb{Mariadb: db.GetParams().GetMysql()}

	pkg, err := PackageResolver{}.ResolveDatabasePackage(db)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if got, want := pkg.GetAptPackages()[0], "mariadb-server"; got != want {
		t.Fatalf("apt package = %q, want %q", got, want)
	}
	preInstall := strings.Join(pkg.GetPreInstall(), "\n")
	for _, want := range []string{
		"https://r.mariadb.com/downloads/mariadb_repo_setup",
		"--mariadb-server-version=10.11",
		"apt-get update",
	} {
		if !strings.Contains(preInstall, want) {
			t.Fatalf("preinstall missing %q:\n%s", want, preInstall)
		}
	}
	if strings.Contains(preInstall, "mariadb-server-10.11") {
		t.Fatalf("preinstall contains versioned apt package name:\n%s", preInstall)
	}
}
