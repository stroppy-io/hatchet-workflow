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
	if database.GetKind() == domain.Database_KIND_MARIADB {
		family = "mariadb"
		name = "MariaDB "
		dbKind = domain.Database_KIND_MARIADB
		aptPackages = []string{"mariadb-server"}
	}
	if version != "default" {
		aptPackages = []string{family + "-server-" + version}
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
		PreInstall:  []string{"apt-get update"},
	}, nil
}
