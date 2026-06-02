package picodata

import "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"

type PackageResolver struct{}

func (r PackageResolver) SupportsDatabase(database *domain.Database) bool {
	return database != nil && database.GetKind() == domain.Database_KIND_PICODATA && database.GetParams() != nil
}

func (r PackageResolver) ResolveDatabasePackage(database *domain.Database) (*domain.Package, error) {
	version := database.GetParams().GetVersion()
	if version == "" {
		version = "default"
	}

	packageID := database.GetPackageId()
	if packageID == "" {
		packageID = "builtin/picodata/" + version
	}

	aptPackages := []string{"picodata"}
	if version != "default" {
		aptPackages = []string{"picodata=" + version}
	}

	return &domain.Package{
		Id:          packageID,
		Name:        "Picodata " + version,
		DbKind:      domain.Database_KIND_PICODATA,
		DbVersion:   version,
		IsBuiltin:   true,
		AptPackages: aptPackages,
		PreInstall:  []string{"apt-get update"},
	}, nil
}
