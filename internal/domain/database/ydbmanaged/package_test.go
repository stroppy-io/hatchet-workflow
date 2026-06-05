package ydbmanaged

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

func TestPackageResolverReturnsNoPackage(t *testing.T) {
	db := &domain.Database{
		Kind: domain.Database_KIND_YDB_MANAGED,
		Source: &domain.Database_Params{
			Params: &domain.DatabaseParams{
				Version: "managed",
				Engine: &domain.DatabaseParams_YdbManaged{
					YdbManaged: &domain.YdbManagedParams{},
				},
			},
		},
	}

	resolver := PackageResolver{}
	if !resolver.SupportsDatabase(db) {
		t.Fatal("resolver does not support managed YDB")
	}
	pkg, err := resolver.ResolveDatabasePackage(db)
	if err != nil {
		t.Fatalf("ResolveDatabasePackage returned error: %v", err)
	}
	if pkg != nil {
		t.Fatalf("managed YDB package = %+v, want nil", pkg)
	}
}
