// Package packages tests — service_test.go
// Tests for all 5 RPCs: CreatePackageUpload, CompleteUpload, GetPackage, ListPackages, DeletePackage.
package packages

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	common "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// ------------------------------------------------------------------
// helpers
// ------------------------------------------------------------------

func newSvc(ctrl *gomock.Controller) (
	*PackageService,
	*utils.MockAuthn,
	*MockPackageRepo,
	*MockBlobStore,
	*MockStorageKeys,
	*MockLimits,
	*MockUploadTTL,
) {
	authn := utils.NewMockAuthn(ctrl)
	repo := NewMockPackageRepo(ctrl)
	blobs := NewMockBlobStore(ctrl)
	keys := NewMockStorageKeys(ctrl)
	limits := NewMockLimits(ctrl)
	ttl := NewMockUploadTTL(ctrl)

	svc := NewPackageService(PackageDeps{
		Authn:    authn,
		Packages: repo,
		Blobs:    blobs,
		Keys:     keys,
		Limits:   limits,
		TTL:      ttl,
		Tx:       &utils.MockTrm{},
	})
	return svc, authn, repo, blobs, keys, limits, ttl
}

// makeRecord builds a minimal PackageRecord for use in mock returns.
func makeRecord(tenantID, id, storageKey string, status models.PackageRecord_Status) *models.PackageRecord {
	return &models.PackageRecord{
		Entity: &common.Entity{
			Id:       id,
			TenantId: tenantID,
		},
		StorageUri: storageKey,
		SizeBytes:  100,
		Sha256:     "abc123",
		Status:     status,
	}
}

// ------------------------------------------------------------------
// CreatePackageUpload
// ------------------------------------------------------------------

func TestCreatePackageUpload(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, repo, blobs, keys, limits, ttl := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acct1"}, nil)
		limits.EXPECT().MaxPackageSizeBytes().Return(uint64(1024 * 1024))
		keys.EXPECT().PackageKey("tenant1", gomock.Any()).Return("blobs/tenant1/pkg1")
		ttl.EXPECT().UploadURLTTL().Return(15 * time.Minute)
		blobs.EXPECT().PresignPut(ctx, "blobs/tenant1/pkg1", uint64(500), "deadbeef", 15*time.Minute).
			Return("https://upload.example.com/presign", time.Now().Add(15*time.Minute), nil)
		repo.EXPECT().Create(ctx, gomock.Any()).Return(nil)

		resp, err := svc.CreatePackageUpload(ctx, &api.CreatePackageUploadRequest{
			TenantId:  "tenant1",
			Name:      "mypackage",
			SizeBytes: 500,
			Sha256:    "DEADBEEF",
			Format:    models.PackageRecord_FORMAT_DEB,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.UploadUrl == "" {
			t.Error("expected non-empty UploadUrl")
		}
		if resp.Package == nil {
			t.Error("expected non-nil Package")
		}
		if resp.Package.GetStatus() != models.PackageRecord_STATUS_UPLOADING {
			t.Errorf("expected STATUS_UPLOADING, got %v", resp.Package.GetStatus())
		}
		// sha256 should be lowercased
		if resp.Package.GetSha256() != "deadbeef" {
			t.Errorf("expected lowercase sha256, got %q", resp.Package.GetSha256())
		}
	})

	t.Run("MissingTenantID", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, _, _, _, _, _ := newSvc(ctrl)

		_, err := svc.CreatePackageUpload(ctx, &api.CreatePackageUploadRequest{
			TenantId: "",
			Name:     "pkg",
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _, _, _, _, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))
		_, err := svc.CreatePackageUpload(ctx, &api.CreatePackageUploadRequest{TenantId: "t1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("SizeExceedsLimit", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _, _, _, limits, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acct1"}, nil)
		limits.EXPECT().MaxPackageSizeBytes().Return(uint64(100))

		_, err := svc.CreatePackageUpload(ctx, &api.CreatePackageUploadRequest{
			TenantId:  "tenant1",
			SizeBytes: 200,
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument for size exceeded, got %v", err)
		}
	})

	t.Run("ZeroMaxSizeAllowsAnySize", func(t *testing.T) {
		// max=0 means no limit
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, repo, blobs, keys, limits, ttl := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acct1"}, nil)
		limits.EXPECT().MaxPackageSizeBytes().Return(uint64(0))
		keys.EXPECT().PackageKey("t1", gomock.Any()).Return("k/t1/id1")
		ttl.EXPECT().UploadURLTTL().Return(time.Minute)
		blobs.EXPECT().PresignPut(ctx, "k/t1/id1", uint64(99999999), gomock.Any(), time.Minute).
			Return("https://example.com/upload", time.Now().Add(time.Minute), nil)
		repo.EXPECT().Create(ctx, gomock.Any()).Return(nil)

		_, err := svc.CreatePackageUpload(ctx, &api.CreatePackageUploadRequest{
			TenantId:  "t1",
			SizeBytes: 99999999,
		})
		if err != nil {
			t.Fatalf("expected no error with unlimited size: %v", err)
		}
	})

	t.Run("PresignPutError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _, blobs, keys, limits, ttl := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acct1"}, nil)
		limits.EXPECT().MaxPackageSizeBytes().Return(uint64(0))
		keys.EXPECT().PackageKey("t1", gomock.Any()).Return("k/t1/id1")
		ttl.EXPECT().UploadURLTTL().Return(time.Minute)
		blobs.EXPECT().PresignPut(ctx, "k/t1/id1", uint64(100), gomock.Any(), time.Minute).
			Return("", time.Time{}, errors.New("s3 error"))

		_, err := svc.CreatePackageUpload(ctx, &api.CreatePackageUploadRequest{
			TenantId:  "t1",
			SizeBytes: 100,
		})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("RepoCreateError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, repo, blobs, keys, limits, ttl := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acct1"}, nil)
		limits.EXPECT().MaxPackageSizeBytes().Return(uint64(0))
		keys.EXPECT().PackageKey("t1", gomock.Any()).Return("k/t1/id1")
		ttl.EXPECT().UploadURLTTL().Return(time.Minute)
		blobs.EXPECT().PresignPut(ctx, "k/t1/id1", uint64(100), gomock.Any(), time.Minute).
			Return("https://example.com/upload", time.Now().Add(time.Minute), nil)
		repo.EXPECT().Create(ctx, gomock.Any()).Return(derrors.Conflict("package", "already exists"))

		_, err := svc.CreatePackageUpload(ctx, &api.CreatePackageUploadRequest{
			TenantId:  "t1",
			SizeBytes: 100,
		})
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", err)
		}
	})
}

// ------------------------------------------------------------------
// CompleteUpload
// ------------------------------------------------------------------

func TestCompleteUpload(t *testing.T) {
	ctx := context.Background()

	t.Run("Success_Verified", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, blobs, _, _, _ := newSvc(ctrl)

		pkg := makeRecord("t1", "id1", "blobs/t1/id1", models.PackageRecord_STATUS_UPLOADING)
		pkg.SizeBytes = 100
		pkg.Sha256 = "abc123"

		repo.EXPECT().Get(ctx, "t1", "id1").Return(pkg, nil)
		blobs.EXPECT().Stat(ctx, "blobs/t1/id1").Return(uint64(100), "ABC123", nil)
		repo.EXPECT().Update(ctx, gomock.Any()).Return(nil)

		resp, err := svc.CompleteUpload(ctx, &api.CompleteUploadRequest{
			TenantId: "t1",
			Id:       "id1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Package.GetStatus() != models.PackageRecord_STATUS_READY {
			t.Errorf("expected STATUS_READY, got %v", resp.Package.GetStatus())
		}
	})

	t.Run("Idempotent_AlreadyReady", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, _, _, _, _ := newSvc(ctrl)

		pkg := makeRecord("t1", "id1", "blobs/t1/id1", models.PackageRecord_STATUS_READY)
		repo.EXPECT().Get(ctx, "t1", "id1").Return(pkg, nil)

		resp, err := svc.CompleteUpload(ctx, &api.CompleteUploadRequest{
			TenantId: "t1",
			Id:       "id1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Package.GetStatus() != models.PackageRecord_STATUS_READY {
			t.Errorf("expected STATUS_READY (idempotent), got %v", resp.Package.GetStatus())
		}
	})

	t.Run("MissingTenantID", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, _, _, _, _, _ := newSvc(ctrl)

		_, err := svc.CompleteUpload(ctx, &api.CompleteUploadRequest{TenantId: ""})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("PackageNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, _, _, _, _ := newSvc(ctrl)

		repo.EXPECT().Get(ctx, "t1", "missing").Return(nil, derrors.ErrNotFound)

		_, err := svc.CompleteUpload(ctx, &api.CompleteUploadRequest{
			TenantId: "t1",
			Id:       "missing",
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("BlobNotUploaded_FailedPrecondition", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, blobs, _, _, _ := newSvc(ctrl)

		pkg := makeRecord("t1", "id1", "blobs/t1/id1", models.PackageRecord_STATUS_UPLOADING)
		repo.EXPECT().Get(ctx, "t1", "id1").Return(pkg, nil)
		blobs.EXPECT().Stat(ctx, "blobs/t1/id1").Return(uint64(0), "", derrors.ErrNotFound)

		_, err := svc.CompleteUpload(ctx, &api.CompleteUploadRequest{TenantId: "t1", Id: "id1"})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition (blob not found), got %v", err)
		}
	})

	t.Run("StatError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, blobs, _, _, _ := newSvc(ctrl)

		pkg := makeRecord("t1", "id1", "blobs/t1/id1", models.PackageRecord_STATUS_UPLOADING)
		repo.EXPECT().Get(ctx, "t1", "id1").Return(pkg, nil)
		blobs.EXPECT().Stat(ctx, "blobs/t1/id1").Return(uint64(0), "", errors.New("stat failed"))

		_, err := svc.CompleteUpload(ctx, &api.CompleteUploadRequest{TenantId: "t1", Id: "id1"})
		if err == nil {
			t.Error("expected error from Stat")
		}
	})

	t.Run("VerificationFailed_SizeMismatch", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, blobs, _, _, _ := newSvc(ctrl)

		pkg := makeRecord("t1", "id1", "blobs/t1/id1", models.PackageRecord_STATUS_UPLOADING)
		pkg.SizeBytes = 100
		pkg.Sha256 = "abc123"

		repo.EXPECT().Get(ctx, "t1", "id1").Return(pkg, nil)
		// Stat returns different size
		blobs.EXPECT().Stat(ctx, "blobs/t1/id1").Return(uint64(200), "abc123", nil)
		repo.EXPECT().Update(ctx, gomock.Any()).Return(nil)

		_, err := svc.CompleteUpload(ctx, &api.CompleteUploadRequest{TenantId: "t1", Id: "id1"})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition (size mismatch), got %v", err)
		}
	})

	t.Run("VerificationFailed_Sha256Mismatch", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, blobs, _, _, _ := newSvc(ctrl)

		pkg := makeRecord("t1", "id1", "blobs/t1/id1", models.PackageRecord_STATUS_UPLOADING)
		pkg.SizeBytes = 100
		pkg.Sha256 = "abc123"

		repo.EXPECT().Get(ctx, "t1", "id1").Return(pkg, nil)
		// Stat returns correct size but different sha
		blobs.EXPECT().Stat(ctx, "blobs/t1/id1").Return(uint64(100), "wronghash", nil)
		repo.EXPECT().Update(ctx, gomock.Any()).Return(nil)

		_, err := svc.CompleteUpload(ctx, &api.CompleteUploadRequest{TenantId: "t1", Id: "id1"})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition (sha mismatch), got %v", err)
		}
	})

	t.Run("UpdateError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, blobs, _, _, _ := newSvc(ctrl)

		pkg := makeRecord("t1", "id1", "blobs/t1/id1", models.PackageRecord_STATUS_UPLOADING)
		pkg.SizeBytes = 100
		pkg.Sha256 = "abc123"

		repo.EXPECT().Get(ctx, "t1", "id1").Return(pkg, nil)
		blobs.EXPECT().Stat(ctx, "blobs/t1/id1").Return(uint64(100), "ABC123", nil)
		repo.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("db write error"))

		_, err := svc.CompleteUpload(ctx, &api.CompleteUploadRequest{TenantId: "t1", Id: "id1"})
		if err == nil {
			t.Error("expected error from Update")
		}
	})
}

// ------------------------------------------------------------------
// GetPackage
// ------------------------------------------------------------------

func TestGetPackage(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, _, _, _, _ := newSvc(ctrl)

		pkg := makeRecord("t1", "id1", "blobs/t1/id1", models.PackageRecord_STATUS_READY)
		repo.EXPECT().Get(ctx, "t1", "id1").Return(pkg, nil)

		resp, err := svc.GetPackage(ctx, &api.GetPackageRequest{TenantId: "t1", Id: "id1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Package.GetEntity().GetId() != "id1" {
			t.Errorf("expected id1, got %s", resp.Package.GetEntity().GetId())
		}
	})

	t.Run("MissingTenantID", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, _, _, _, _, _ := newSvc(ctrl)

		_, err := svc.GetPackage(ctx, &api.GetPackageRequest{TenantId: ""})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, _, _, _, _ := newSvc(ctrl)

		repo.EXPECT().Get(ctx, "t1", "missing").Return(nil, derrors.ErrNotFound)

		_, err := svc.GetPackage(ctx, &api.GetPackageRequest{TenantId: "t1", Id: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("WrongTenant_NotFound", func(t *testing.T) {
		// getOwned checks that Entity.TenantId matches the requested tenantID
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, _, _, _, _ := newSvc(ctrl)

		// record belongs to "other-tenant"
		pkg := makeRecord("other-tenant", "id1", "blobs/other/id1", models.PackageRecord_STATUS_READY)
		repo.EXPECT().Get(ctx, "t1", "id1").Return(pkg, nil)

		_, err := svc.GetPackage(ctx, &api.GetPackageRequest{TenantId: "t1", Id: "id1"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound for cross-tenant probe, got %v", err)
		}
	})

	t.Run("RepoError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, _, _, _, _ := newSvc(ctrl)

		repo.EXPECT().Get(ctx, "t1", "id1").Return(nil, errors.New("db down"))

		_, err := svc.GetPackage(ctx, &api.GetPackageRequest{TenantId: "t1", Id: "id1"})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})
}

// ------------------------------------------------------------------
// ListPackages
// ------------------------------------------------------------------

func TestListPackages(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, _, _, _, _ := newSvc(ctrl)

		pkgs := []*models.PackageRecord{
			makeRecord("t1", "id1", "blobs/t1/id1", models.PackageRecord_STATUS_READY),
			makeRecord("t1", "id2", "blobs/t1/id2", models.PackageRecord_STATUS_UPLOADING),
		}
		repo.EXPECT().List(ctx, gomock.Any()).Return(pkgs, "nexttoken", nil)

		resp, err := svc.ListPackages(ctx, &api.ListPackagesRequest{TenantId: "t1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Packages) != 2 {
			t.Errorf("expected 2 packages, got %d", len(resp.Packages))
		}
		if resp.NextPageToken != "nexttoken" {
			t.Errorf("expected nexttoken, got %q", resp.NextPageToken)
		}
	})

	t.Run("EmptyList", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, _, _, _, _ := newSvc(ctrl)

		repo.EXPECT().List(ctx, gomock.Any()).Return([]*models.PackageRecord{}, "", nil)

		resp, err := svc.ListPackages(ctx, &api.ListPackagesRequest{TenantId: "t1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Packages) != 0 {
			t.Errorf("expected 0 packages, got %d", len(resp.Packages))
		}
	})

	t.Run("MissingTenantID", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, _, _, _, _, _ := newSvc(ctrl)

		_, err := svc.ListPackages(ctx, &api.ListPackagesRequest{TenantId: "  "})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("RepoError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, _, _, _, _ := newSvc(ctrl)

		repo.EXPECT().List(ctx, gomock.Any()).Return(nil, "", errors.New("db failure"))

		_, err := svc.ListPackages(ctx, &api.ListPackagesRequest{TenantId: "t1"})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("QueryPropagation", func(t *testing.T) {
		// Verify TenantID is always server-enforced in the query
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, _, _, _, _ := newSvc(ctrl)

		repo.EXPECT().List(ctx, gomock.AssignableToTypeOf(PackageQuery{})).
			DoAndReturn(func(_ context.Context, q PackageQuery) ([]*models.PackageRecord, string, error) {
				if q.TenantID != "myTenant" {
					return nil, "", errors.New("wrong tenant in query")
				}
				return nil, "", nil
			})

		_, err := svc.ListPackages(ctx, &api.ListPackagesRequest{TenantId: "myTenant"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// ------------------------------------------------------------------
// DeletePackage
// ------------------------------------------------------------------

func TestDeletePackage(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, blobs, _, _, _ := newSvc(ctrl)

		pkg := makeRecord("t1", "id1", "blobs/t1/id1", models.PackageRecord_STATUS_READY)
		repo.EXPECT().Get(ctx, "t1", "id1").Return(pkg, nil)
		blobs.EXPECT().Delete(ctx, "blobs/t1/id1").Return(nil)
		repo.EXPECT().Delete(ctx, "t1", "id1").Return(nil)

		_, err := svc.DeletePackage(ctx, &api.DeletePackageRequest{TenantId: "t1", Id: "id1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Idempotent_NotFound", func(t *testing.T) {
		// deleting an absent package is a no-op
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, _, _, _, _ := newSvc(ctrl)

		repo.EXPECT().Get(ctx, "t1", "missing").Return(nil, derrors.ErrNotFound)

		_, err := svc.DeletePackage(ctx, &api.DeletePackageRequest{TenantId: "t1", Id: "missing"})
		if err != nil {
			t.Fatalf("expected no error for idempotent delete: %v", err)
		}
	})

	t.Run("MissingTenantID", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, _, _, _, _, _ := newSvc(ctrl)

		_, err := svc.DeletePackage(ctx, &api.DeletePackageRequest{TenantId: ""})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("BlobDeleteNotFoundIgnored", func(t *testing.T) {
		// blob delete returning not-found should be tolerated
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, blobs, _, _, _ := newSvc(ctrl)

		pkg := makeRecord("t1", "id1", "blobs/t1/id1", models.PackageRecord_STATUS_READY)
		repo.EXPECT().Get(ctx, "t1", "id1").Return(pkg, nil)
		blobs.EXPECT().Delete(ctx, "blobs/t1/id1").Return(derrors.ErrNotFound)
		repo.EXPECT().Delete(ctx, "t1", "id1").Return(nil)

		_, err := svc.DeletePackage(ctx, &api.DeletePackageRequest{TenantId: "t1", Id: "id1"})
		if err != nil {
			t.Fatalf("expected no error when blob delete returns not-found: %v", err)
		}
	})

	t.Run("BlobDeleteError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, blobs, _, _, _ := newSvc(ctrl)

		pkg := makeRecord("t1", "id1", "blobs/t1/id1", models.PackageRecord_STATUS_READY)
		repo.EXPECT().Get(ctx, "t1", "id1").Return(pkg, nil)
		blobs.EXPECT().Delete(ctx, "blobs/t1/id1").Return(errors.New("s3 error"))

		_, err := svc.DeletePackage(ctx, &api.DeletePackageRequest{TenantId: "t1", Id: "id1"})
		if err == nil {
			t.Error("expected error from Blobs.Delete")
		}
	})

	t.Run("RepoDeleteError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, blobs, _, _, _ := newSvc(ctrl)

		pkg := makeRecord("t1", "id1", "blobs/t1/id1", models.PackageRecord_STATUS_READY)
		repo.EXPECT().Get(ctx, "t1", "id1").Return(pkg, nil)
		blobs.EXPECT().Delete(ctx, "blobs/t1/id1").Return(nil)
		repo.EXPECT().Delete(ctx, "t1", "id1").Return(errors.New("db delete error"))

		_, err := svc.DeletePackage(ctx, &api.DeletePackageRequest{TenantId: "t1", Id: "id1"})
		if err == nil {
			t.Error("expected error from Packages.Delete")
		}
	})

	t.Run("GetError_NotNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, _, _, _, _ := newSvc(ctrl)

		repo.EXPECT().Get(ctx, "t1", "id1").Return(nil, errors.New("db unavailable"))

		_, err := svc.DeletePackage(ctx, &api.DeletePackageRequest{TenantId: "t1", Id: "id1"})
		if err == nil {
			t.Error("expected error from Get non-NotFound")
		}
	})

	t.Run("RepoDeleteNotFoundIgnored", func(t *testing.T) {
		// IgnoreNotFound in deletion path: if Packages.Delete returns ErrNotFound, no error
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, repo, blobs, _, _, _ := newSvc(ctrl)

		pkg := makeRecord("t1", "id1", "blobs/t1/id1", models.PackageRecord_STATUS_READY)
		repo.EXPECT().Get(ctx, "t1", "id1").Return(pkg, nil)
		blobs.EXPECT().Delete(ctx, "blobs/t1/id1").Return(nil)
		repo.EXPECT().Delete(ctx, "t1", "id1").Return(derrors.ErrNotFound)

		_, err := svc.DeletePackage(ctx, &api.DeletePackageRequest{TenantId: "t1", Id: "id1"})
		if err != nil {
			t.Fatalf("expected no error when repo delete returns not-found: %v", err)
		}
	})
}
