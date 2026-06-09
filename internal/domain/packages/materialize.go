package packages

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

type PackageRecordGetter interface {
	Get(ctx context.Context, tenantID, id string) (*models.PackageRecord, error)
}

func MaterializeTestRunPackages(ctx context.Context, tenantID string, run *domain.TestRun, records PackageRecordGetter) error {
	if run == nil {
		return nil
	}
	return MaterializeDatabasePackage(ctx, tenantID, run.GetDatabase(), records)
}

func MaterializeDatabasePackage(ctx context.Context, tenantID string, database *domain.Database, records PackageRecordGetter) error {
	if database == nil || database.GetParams() == nil {
		return nil
	}
	params := database.GetParams()
	if pkg := params.GetPackage(); pkg != nil && !pkg.GetIsBuiltin() {
		if pkg.GetDebFilename() != "" {
			return nil
		}
		if pkg.GetPackageRecordId() == "" {
			return nil
		}
		return materializePackageRecord(ctx, tenantID, database, pkg.GetPackageRecordId(), records)
	}

	packageID := strings.TrimSpace(database.GetPackageId())
	if packageID == "" || strings.HasPrefix(packageID, "builtin/") {
		return nil
	}
	return materializePackageRecord(ctx, tenantID, database, packageID, records)
}

func materializePackageRecord(ctx context.Context, tenantID string, database *domain.Database, packageID string, records PackageRecordGetter) error {
	if records == nil {
		return fmt.Errorf("package %q requires package record resolver", packageID)
	}
	record, err := records.Get(ctx, tenantID, packageID)
	if err != nil {
		return fmt.Errorf("get package %q: %w", packageID, err)
	}
	if record == nil {
		return fmt.Errorf("package %q not found", packageID)
	}
	if record.GetStatus() != models.PackageRecord_STATUS_READY {
		return fmt.Errorf("package %q is not ready", packageID)
	}
	if record.GetTargetDbKind() != domain.Database_KIND_UNSPECIFIED && record.GetTargetDbKind() != database.GetKind() {
		return fmt.Errorf("package %q targets %s, not %s", packageID, record.GetTargetDbKind(), database.GetKind())
	}
	if record.GetStorageUri() == "" {
		return fmt.Errorf("package %q has empty storage uri", packageID)
	}

	version := record.GetVersion()
	if version == "" {
		version = database.GetParams().GetVersion()
	}
	database.GetParams().Package = &domain.Package{
		Id:              record.GetEntity().GetId(),
		Name:            record.GetEntity().GetName(),
		DbKind:          database.GetKind(),
		DbVersion:       version,
		IsBuiltin:       false,
		DebFilename:     PackageBlobURL(record.GetStorageUri()),
		PackageRecordId: record.GetEntity().GetId(),
	}
	return nil
}

func PackageBlobURL(storageURI string) string {
	return "${STROPPY_SERVER_ADDR%/}/api/packages/blob/" + url.PathEscape(storageURI)
}
