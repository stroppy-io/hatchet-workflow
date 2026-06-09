package packages

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

func TestRegistryUsesEngineResolver(t *testing.T) {
	pkg, err := NewRegistry(fakePackageResolver{}).ResolveDatabasePackage(postgresDatabase(nil))
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if got, want := pkg.GetId(), "fake-package"; got != want {
		t.Fatalf("package id = %q, want %q", got, want)
	}
}

func TestRegistryPrefersEmbeddedPackage(t *testing.T) {
	embedded := &domain.Package{
		Id:        "custom-package",
		DbKind:    domain.Database_KIND_POSTGRES,
		DbVersion: "custom",
	}

	pkg, err := NewRegistry(fakePackageResolver{}).ResolveDatabasePackage(postgresDatabase(embedded))
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if pkg != embedded {
		t.Fatal("resolver did not return embedded package")
	}
}

func TestRegistryRebuildsEmbeddedBuiltinPackage(t *testing.T) {
	embedded := &domain.Package{
		Id:          "builtin/postgres/16",
		DbKind:      domain.Database_KIND_POSTGRES,
		DbVersion:   "16",
		IsBuiltin:   true,
		AptPackages: []string{"stale-postgres-package"},
	}

	pkg, err := NewRegistry(fakePackageResolver{}).ResolveDatabasePackage(postgresDatabase(embedded))
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if pkg == embedded {
		t.Fatal("resolver returned stale embedded builtin package")
	}
	if got, want := pkg.GetId(), "fake-package"; got != want {
		t.Fatalf("package id = %q, want %q", got, want)
	}
}

func TestRegistryRejectsMissingResolver(t *testing.T) {
	_, err := NewRegistry().ResolveDatabasePackage(postgresDatabase(nil))
	if err == nil {
		t.Fatal("expected error")
	}
}

type fakePackageResolver struct{}

func (r fakePackageResolver) SupportsDatabase(database *domain.Database) bool {
	return database.GetKind() == domain.Database_KIND_POSTGRES
}

func (r fakePackageResolver) ResolveDatabasePackage(database *domain.Database) (*domain.Package, error) {
	return &domain.Package{
		Id:        "fake-package",
		DbKind:    database.GetKind(),
		DbVersion: database.GetParams().GetVersion(),
	}, nil
}

func postgresDatabase(pkg *domain.Package) *domain.Database {
	return &domain.Database{
		Kind: domain.Database_KIND_POSTGRES,
		Source: &domain.Database_Params{
			Params: &domain.DatabaseParams{
				Version: "16",
				Package: pkg,
				Engine: &domain.DatabaseParams_Postgres{
					Postgres: &domain.PostgresParams{},
				},
			},
		},
	}
}
