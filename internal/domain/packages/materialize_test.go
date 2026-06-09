package packages

import (
	"context"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

func TestMaterializeDatabasePackageFromPackageID(t *testing.T) {
	db := postgresDatabase(nil)
	db.PackageId = "pkg-1"
	err := MaterializeDatabasePackage(context.Background(), "tenant-1", db, fakePackageRecords{
		"pkg-1": readyPackageRecord("tenant-1", "pkg-1", "packages_tenant-1_pkg-1"),
	})
	if err != nil {
		t.Fatalf("materialize package: %v", err)
	}
	pkg := db.GetParams().GetPackage()
	if pkg == nil {
		t.Fatal("database params package is nil")
	}
	if got, want := pkg.GetDebFilename(), "${STROPPY_SERVER_ADDR%/}/api/packages/blob/packages_tenant-1_pkg-1"; got != want {
		t.Fatalf("deb filename = %q, want %q", got, want)
	}
	if got, want := pkg.GetPackageRecordId(), "pkg-1"; got != want {
		t.Fatalf("package record id = %q, want %q", got, want)
	}
	if pkg.GetIsBuiltin() {
		t.Fatal("materialized package is builtin")
	}
}

func TestMaterializeDatabasePackageLeavesBuiltinPackageID(t *testing.T) {
	db := postgresDatabase(nil)
	db.PackageId = "builtin/postgres/16"
	if err := MaterializeDatabasePackage(context.Background(), "tenant-1", db, fakePackageRecords{}); err != nil {
		t.Fatalf("materialize package: %v", err)
	}
	if db.GetParams().GetPackage() != nil {
		t.Fatalf("builtin package id should be resolved by engine resolver, got %#v", db.GetParams().GetPackage())
	}
}

type fakePackageRecords map[string]*models.PackageRecord

func (f fakePackageRecords) Get(_ context.Context, _ string, id string) (*models.PackageRecord, error) {
	return f[id], nil
}

func readyPackageRecord(tenantID, id, storageURI string) *models.PackageRecord {
	return &models.PackageRecord{
		Entity: &common.Entity{
			Id:       id,
			TenantId: tenantID,
			Name:     "Custom package",
		},
		TargetDbKind: domain.Database_KIND_POSTGRES,
		Version:      "16",
		StorageUri:   storageURI,
		Status:       models.PackageRecord_STATUS_READY,
	}
}
