package packages

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

func TestListPackagesIncludesBuiltinCatalog(t *testing.T) {
	repo := &capturingPackageRepo{}
	svc := NewPackageService(PackageDeps{Packages: repo})

	resp, err := svc.ListPackages(context.Background(), &api.ListPackagesRequest{
		TenantId: "tenant-1",
	})
	if err != nil {
		t.Fatalf("ListPackages returned error: %v", err)
	}
	if len(resp.GetPackages()) != len(builtinPackageSpecs) {
		t.Fatalf("got %d packages, want %d", len(resp.GetPackages()), len(builtinPackageSpecs))
	}
	if len(repo.lastQuery.Builtin) != len(builtinPackageSpecs) {
		t.Fatalf("repo received %d builtin packages, want %d", len(repo.lastQuery.Builtin), len(builtinPackageSpecs))
	}

	for _, pkg := range resp.GetPackages() {
		if !pkg.GetIsBuiltin() {
			t.Fatalf("package %q is not marked built-in", pkg.GetEntity().GetId())
		}
		if got := pkg.GetEntity().GetTenantId(); got != "tenant-1" {
			t.Fatalf("package %q tenant = %q, want tenant-1", pkg.GetEntity().GetId(), got)
		}
		if got := pkg.GetStatus(); got != models.PackageRecord_STATUS_READY {
			t.Fatalf("package %q status = %v, want READY", pkg.GetEntity().GetId(), got)
		}
		if pkg.GetSha256() != "" {
			t.Fatalf("package %q sha256 = %q, want empty", pkg.GetEntity().GetId(), pkg.GetSha256())
		}
		if pkg.GetStorageUri() != "" {
			t.Fatalf("package %q has storage uri %q", pkg.GetEntity().GetId(), pkg.GetStorageUri())
		}
	}
}

func TestBuiltinPackageCatalogCoversDatabaseKinds(t *testing.T) {
	want := map[domain.Database_Kind]bool{
		domain.Database_KIND_POSTGRES:  false,
		domain.Database_KIND_MYSQL:     false,
		domain.Database_KIND_MARIADB:   false,
		domain.Database_KIND_PICODATA:  false,
		domain.Database_KIND_YDB:       false,
		domain.Database_KIND_COCKROACH: false,
	}

	for _, pkg := range builtinPackageRecords("tenant-1") {
		if _, ok := want[pkg.GetTargetDbKind()]; ok {
			want[pkg.GetTargetDbKind()] = true
		}
	}

	for kind, seen := range want {
		if !seen {
			t.Fatalf("missing built-in package for database kind %v", kind)
		}
	}
	for _, pkg := range builtinPackageRecords("tenant-1") {
		if pkg.GetTargetDbKind() == domain.Database_KIND_YDB_MANAGED {
			t.Fatal("managed YDB must not have a built-in install package")
		}
	}
}

func TestDeletePackageRejectsBuiltinPackages(t *testing.T) {
	svc := NewPackageService(PackageDeps{})

	_, err := svc.DeletePackage(context.Background(), &api.DeletePackageRequest{
		TenantId: "tenant-1",
		Id:       "builtin/postgres/default",
	})
	if got := status.Code(err); got != codes.FailedPrecondition {
		t.Fatalf("DeletePackage status = %v, want FailedPrecondition; err=%v", got, err)
	}
}

func TestCompleteUploadRejectsBuiltinPackages(t *testing.T) {
	svc := NewPackageService(PackageDeps{})

	_, err := svc.CompleteUpload(context.Background(), &api.CompleteUploadRequest{
		TenantId: "tenant-1",
		Id:       "builtin/postgres/default",
	})
	if got := status.Code(err); got != codes.FailedPrecondition {
		t.Fatalf("CompleteUpload status = %v, want FailedPrecondition; err=%v", got, err)
	}
}

type capturingPackageRepo struct {
	lastQuery PackageQuery
}

func (r *capturingPackageRepo) Create(context.Context, *models.PackageRecord) error {
	return nil
}

func (r *capturingPackageRepo) Get(context.Context, string, string) (*models.PackageRecord, error) {
	return nil, nil
}

func (r *capturingPackageRepo) List(_ context.Context, q PackageQuery) ([]*models.PackageRecord, string, error) {
	r.lastQuery = q
	return q.Builtin, "", nil
}

func (r *capturingPackageRepo) Update(context.Context, *models.PackageRecord) error {
	return nil
}

func (r *capturingPackageRepo) Delete(context.Context, string, string) error {
	return nil
}
