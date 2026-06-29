package noop

import (
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

// PackageResolver handles the no-DB machine benchmark (KIND_NOOP) in the package
// pipeline. There is no database to install — stroppy runs with its internal
// noop driver — so the resolver deliberately returns nil instead of a package
// (mirrors the managed-YDB resolver). It exists only so the registry does not
// error with "no package resolver for database kind KIND_NOOP".
type PackageResolver struct{}

func (r PackageResolver) SupportsDatabase(database *domain.Database) bool {
	return database != nil && database.GetKind() == domain.Database_KIND_NOOP
}

func (r PackageResolver) ResolveDatabasePackage(database *domain.Database) (*domain.Package, error) {
	return nil, nil
}
