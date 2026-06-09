package mysql

import "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"

type PackageResolver struct{}

func (r PackageResolver) SupportsDatabase(database *domain.Database) bool {
	return database != nil &&
		(database.GetKind() == domain.Database_KIND_MYSQL || database.GetKind() == domain.Database_KIND_MARIADB) &&
		database.GetParams() != nil
}

func (r PackageResolver) ResolveDatabasePackage(database *domain.Database) (*domain.Package, error) {
	version := database.GetParams().GetVersion()
	if version == "" {
		version = "default"
	}

	family := "mysql"
	name := "MySQL "
	dbKind := domain.Database_KIND_MYSQL
	aptPackages := []string{"mysql-server"}
	preInstall := []string{"apt-get update"}
	if database.GetKind() == domain.Database_KIND_MARIADB {
		family = "mariadb"
		name = "MariaDB "
		dbKind = domain.Database_KIND_MARIADB
		aptPackages = []string{"mariadb-server"}
		if version != "default" {
			preInstall = mariadbPreInstall(version)
		}
	} else if version != "default" {
		preInstall = mysqlPreInstall(version)
	}

	packageID := database.GetPackageId()
	if packageID == "" {
		packageID = "builtin/" + family + "/" + version
	}

	return &domain.Package{
		Id:          packageID,
		Name:        name + version,
		DbKind:      dbKind,
		DbVersion:   version,
		IsBuiltin:   true,
		AptPackages: aptPackages,
		PreInstall:  preInstall,
	}, nil
}

func mysqlPreInstall(version string) []string {
	repoComponent := "mysql-" + version
	if version == "8.4" {
		repoComponent = "mysql-8.4-lts"
	}
	return []string{
		"DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates curl gpg lsb-release",
		"install -d /etc/apt/keyrings",
		`sh -ec 'key=/tmp/mysql-repo.gpg.key
curl -fsSL https://repo.mysql.com/RPM-GPG-KEY-mysql-2025 -o "$key" || curl -fsSL https://repo.mysql.com/RPM-GPG-KEY-mysql-2023 -o "$key"
gpg --dearmor -o /etc/apt/keyrings/mysql.gpg "$key"
chmod 644 /etc/apt/keyrings/mysql.gpg'`,
		`sh -c 'echo "deb [signed-by=/etc/apt/keyrings/mysql.gpg] http://repo.mysql.com/apt/ubuntu/ $(lsb_release -cs) ` + repoComponent + `" > /etc/apt/sources.list.d/mysql.list'`,
		"apt-get update",
	}
}

func mariadbPreInstall(version string) []string {
	return []string{
		"DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates curl",
		"curl -fsSL https://r.mariadb.com/downloads/mariadb_repo_setup -o /tmp/mariadb_repo_setup",
		"bash /tmp/mariadb_repo_setup --mariadb-server-version=" + version,
		"apt-get update",
	}
}
