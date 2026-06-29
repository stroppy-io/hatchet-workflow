package noop

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/packages"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

// The no-DB benchmark must resolve through the package registry WITHOUT error
// (it has params but no installable artifact) — otherwise the wizard preview
// fails with "no package resolver for database kind KIND_NOOP".
func TestNoopResolvesToNoPackage(t *testing.T) {
	db := &domain.Database{
		Kind: domain.Database_KIND_NOOP,
		Source: &domain.Database_Params{
			Params: &domain.DatabaseParams{
				Engine: &domain.DatabaseParams_Noop{Noop: &domain.NoopParams{}},
			},
		},
	}

	registry := packages.NewRegistry(PackageResolver{})
	pkg, err := registry.ResolveDatabasePackage(db)
	if err != nil {
		t.Fatalf("resolve noop package: %v", err)
	}
	if pkg != nil {
		t.Fatalf("noop must have no package, got %+v", pkg)
	}
}
