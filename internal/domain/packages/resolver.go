package packages

import (
	"errors"
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

type Resolver interface {
	ResolveDatabasePackage(database *domain.Database) (*domain.Package, error)
}

type EngineResolver interface {
	SupportsDatabase(database *domain.Database) bool
	ResolveDatabasePackage(database *domain.Database) (*domain.Package, error)
}

type Registry struct {
	resolvers []EngineResolver
}

func NewRegistry(resolvers ...EngineResolver) Registry {
	return Registry{resolvers: append([]EngineResolver(nil), resolvers...)}
}

func (r Registry) ResolveDatabasePackage(database *domain.Database) (*domain.Package, error) {
	if database == nil {
		return nil, errors.New("database is required")
	}

	params := database.GetParams()
	if params == nil {
		return nil, nil
	}
	if pkg := params.GetPackage(); pkg != nil && !pkg.GetIsBuiltin() {
		return params.GetPackage(), nil
	}

	for _, resolver := range r.resolvers {
		if resolver.SupportsDatabase(database) {
			return resolver.ResolveDatabasePackage(database)
		}
	}

	return nil, fmt.Errorf("no package resolver for database kind %s", database.GetKind())
}
