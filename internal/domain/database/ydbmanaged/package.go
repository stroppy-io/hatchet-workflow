package ydbmanaged

import (
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

// PackageResolver handles managed YDB in the package pipeline. Managed YDB has
// no installable artifact — the provider runs the database — so the resolver
// deliberately returns nil instead of a package.
type PackageResolver struct{}

func (r PackageResolver) SupportsDatabase(database *domain.Database) bool {
	return database != nil && database.GetKind() == domain.Database_KIND_YDB_MANAGED
}

func (r PackageResolver) ResolveDatabasePackage(database *domain.Database) (*domain.Package, error) {
	return nil, nil
}
