package ydbmanaged

import (
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

// PackageResolver resolves the (virtual) package for a managed YDB database.
// Managed YDB has no installable artifact — the provider runs the database —
// so the resolver returns a builtin marker package. It exists so the package
// registry has a handler for the KIND_YDB_MANAGED kind.
type PackageResolver struct{}

func (r PackageResolver) SupportsDatabase(database *domain.Database) bool {
	return database != nil && database.GetKind() == domain.Database_KIND_YDB_MANAGED
}

func (r PackageResolver) ResolveDatabasePackage(database *domain.Database) (*domain.Package, error) {
	version := database.GetParams().GetVersion()
	if version == "" {
		version = "managed"
	}
	packageID := database.GetPackageId()
	if packageID == "" {
		packageID = "builtin/ydb-managed/" + version
	}
	return &domain.Package{
		Id:        packageID,
		Name:      "Yandex Managed YDB",
		DbKind:    domain.Database_KIND_YDB_MANAGED,
		DbVersion: version,
		IsBuiltin: true,
	}, nil
}
