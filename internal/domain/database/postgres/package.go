package postgres

import "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"

type PackageResolver struct{}

func (r PackageResolver) SupportsDatabase(database *domain.Database) bool {
	return database != nil && database.GetKind() == domain.Database_KIND_POSTGRES && database.GetParams() != nil
}

func (r PackageResolver) ResolveDatabasePackage(database *domain.Database) (*domain.Package, error) {
	params := database.GetParams()
	version := params.GetVersion()
	if version == "" {
		version = "default"
	}

	packageID := database.GetPackageId()
	if packageID == "" {
		packageID = "builtin/postgres/" + version
	}

	aptPackages := []string{"postgresql", "postgresql-contrib"}
	if version != "default" {
		aptPackages = []string{"postgresql-" + version, "postgresql-contrib-" + version}
	}

	return &domain.Package{
		Id:          packageID,
		Name:        "PostgreSQL " + version,
		DbKind:      domain.Database_KIND_POSTGRES,
		DbVersion:   version,
		IsBuiltin:   true,
		AptPackages: aptPackages,
		PreInstall:  []string{"apt-get update"},
	}, nil
}
