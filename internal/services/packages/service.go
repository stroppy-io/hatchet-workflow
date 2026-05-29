// Package packages implements PackageService — the tenant package registry for
// custom DB builds (.deb / binary). The package dir/name is "packages" because
// "package" is a Go keyword.
package packages

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/pgtx/pkg/tx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	common "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	===== Dependency interfaces (constructor-injected) =====

	The service owns no storage or blob plumbing of its own: every side effect goes
	through one of these. Implementations live elsewhere and are wired in via
	PackageDeps. Persistence methods return derrors.ErrNotFound / derrors.ErrConflict
	so the handler can translate them into status codes via utils.MapErr.
*/

// PackageRepo persists PackageRecord metadata (never the blob itself). Every
// method is tenant-scoped: the storage layer keys rows by (tenant_id, id) so a
// package is tenant-private. Get/Delete return derrors.ErrNotFound for an
// unknown (tenant, id) pair.
type PackageRepo interface {
	Create(ctx context.Context, pkg *models.PackageRecord) error
	Get(ctx context.Context, tenantID, id string) (*models.PackageRecord, error)
	List(ctx context.Context, q PackageQuery) (packages []*models.PackageRecord, nextPageToken string, err error)
	Update(ctx context.Context, pkg *models.PackageRecord) error
	Delete(ctx context.Context, tenantID, id string) error
}

// PackageQuery is the resolved, tenant-scoped list query handed to the repo. The
// handler builds it so the repo never has to reach back into the request: tenant
// scoping is server-enforced (TenantID), never client-supplied.
type PackageQuery struct {
	TenantID  string
	Filter    *common.EntityFilter
	Formats   []models.PackageRecord_Format
	DbKinds   []domain.Database_Kind
	Sort      *common.EntitySort
	PageSize  uint32
	PageToken string
}

// BlobStore brokers the object-storage side of the presigned-upload flow. None
// of these touch the DB; they speak to S3/MinIO.
type BlobStore interface {
	// PresignPut returns a presigned PUT url for the given storage key plus the
	// moment that url stops working. expectedSize/sha256 may be folded into the
	// signature (e.g. Content-Length / x-amz-checksum) so the object cannot be
	// substituted.
	PresignPut(ctx context.Context, key string, expectedSize uint64, sha256 string, ttl time.Duration) (url string, expiresAt time.Time, err error)
	// Stat reports the uploaded object's size + content hash for verification on
	// CompleteUpload. Returns derrors.ErrNotFound if the client never PUT the blob.
	Stat(ctx context.Context, key string) (sizeBytes uint64, sha256 string, err error)
	// Delete removes the blob; idempotent (not-found is fine).
	Delete(ctx context.Context, key string) error
}

// StorageKeys mints the object-storage key for a package blob. Keeping it behind
// a port lets the layout (prefix per tenant, etc.) change without touching the
// handler.
type StorageKeys interface {
	PackageKey(tenantID, packageID string) string
}

// Limits supplies the configurable size ceiling enforced before a presigned url
// is minted (a package executes on the host, so an unbounded blob is a hazard).
type Limits interface {
	MaxPackageSizeBytes() uint64
}

// UploadTTL supplies the lifetime of a freshly minted presigned PUT url.
type UploadTTL interface {
	UploadURLTTL() time.Duration
}

// PackageDeps bundles every dependency for the constructor.
type PackageDeps struct {
	Authn    utils.Authn
	Packages PackageRepo
	Blobs    BlobStore
	Keys     StorageKeys
	Limits   Limits
	TTL      UploadTTL
	Tx       tx.Trm
}

type PackageService struct {
	*api.UnimplementedPackageServiceServer
	tx.Trm
	d PackageDeps
}

var _ api.PackageServiceServer = (*PackageService)(nil)

func NewPackageService(deps PackageDeps) *PackageService {
	return &PackageService{d: deps}
}

/*
	===== helpers =====
*/

func (s *PackageService) caller(ctx context.Context) (*iam.AccessClaims, error) {
	c, err := s.d.Authn.Caller(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return c, nil
}

func (s *PackageService) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// doTx runs fn in a top-level serializable transaction, retrying on transient
// serialization failures/deadlocks (DefaultRetryPolicy). fn is re-run from
// scratch on retry, so it MUST be safe to repeat: DB-only side effects and never
// synchronous external IO.
func (s *PackageService) doTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.DoSerializable(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// doTxRet is doTx for a transaction that returns a value. Same retry semantics:
// fn must be safe to re-run.
func doTxRet[T any](ctx context.Context, s *PackageService, fn func(ctx context.Context) (T, error)) (T, error) {
	return tx.DoSerializableRet(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// requireTenant fences every handler: a tenant_id is mandatory (the row is
// tenant-private). The interceptor has already enforced the RESOURCE_PACKAGE
// permission; here we only validate the active tenant scope.
func requireTenant(tenantID string) error {
	if strings.TrimSpace(tenantID) == "" {
		return status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	return nil
}

// getOwned fetches a record and asserts it belongs to the requested tenant. A
// row keyed under a different tenant is reported as NotFound so a caller cannot
// probe another tenant's id space.
func (s *PackageService) getOwned(ctx context.Context, tenantID, id string) (*models.PackageRecord, error) {
	pkg, err := s.d.Packages.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if pkg.GetEntity().GetTenantId() != tenantID {
		return nil, derrors.NotFound("package", "package not found in tenant")
	}
	return pkg, nil
}

/*
	===== Handlers =====
*/

// CreatePackageUpload mints a STATUS_UPLOADING record + a presigned PUT url. Not
// idempotent. The declared size is bounded before any url is signed, and the
// storage key is server-assigned (never client-set).
func (s *PackageService) CreatePackageUpload(ctx context.Context, req *api.CreatePackageUploadRequest) (*api.CreatePackageUploadResponse, error) {
	if err := requireTenant(req.GetTenantId()); err != nil {
		return nil, err
	}
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	if max := s.d.Limits.MaxPackageSizeBytes(); max > 0 && req.GetSizeBytes() > max {
		return nil, status.Errorf(codes.InvalidArgument, "size_bytes exceeds limit of %d bytes", max)
	}

	id := uuid.NewString()
	key := s.d.Keys.PackageKey(req.GetTenantId(), id)
	pkg := &models.PackageRecord{
		Entity: &common.Entity{
			Id:       id,
			TenantId: req.GetTenantId(),
			Name:     req.GetName(),
			AuthorId: c.GetAccountId(),
			Timings: &common.Timings{
				CreatedAt: s.now(),
				UpdatedAt: s.now(),
			},
		},
		Format:       req.GetFormat(),
		Version:      req.GetVersion(),
		TargetDbKind: req.GetTargetDbKind(),
		Os:           req.GetOs(),
		Arch:         req.GetArch(),
		StorageUri:   key,
		SizeBytes:    req.GetSizeBytes(),
		Sha256:       strings.ToLower(req.GetSha256()),
		Status:       models.PackageRecord_STATUS_UPLOADING,
	}

	url, expiresAt, err := s.d.Blobs.PresignPut(ctx, key, req.GetSizeBytes(), pkg.Sha256, s.d.TTL.UploadURLTTL())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if err := s.d.Packages.Create(ctx, pkg); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.CreatePackageUploadResponse{
		Package:            pkg,
		UploadUrl:          url,
		UploadUrlExpiresAt: timestamppb.New(expiresAt),
	}, nil
}

// CompleteUpload finalizes after the client PUT the blob: verifies the uploaded
// object's size + sha256 against the declared values and flips the record to
// READY (or FAILED). Idempotent — re-completing a READY record is a no-op and a
// FAILED record can be re-verified.
func (s *PackageService) CompleteUpload(ctx context.Context, req *api.CompleteUploadRequest) (*api.CompleteUploadResponse, error) {
	if err := requireTenant(req.GetTenantId()); err != nil {
		return nil, err
	}
	pkg, err := doTxRet(ctx, s, func(ctx context.Context) (*models.PackageRecord, error) {
		pkg, err := s.getOwned(ctx, req.GetTenantId(), req.GetId())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		// Idempotency: an already-verified record is returned unchanged.
		if pkg.GetStatus() == models.PackageRecord_STATUS_READY {
			return pkg, nil
		}

		actualSize, actualSha, statErr := s.d.Blobs.Stat(ctx, pkg.GetStorageUri())
		switch {
		case errors.Is(statErr, derrors.ErrNotFound):
			// Blob never landed: cannot complete yet.
			return nil, status.Error(codes.FailedPrecondition, "uploaded object not found; PUT the blob before completing")
		case statErr != nil:
			return nil, utils.MapErr(statErr)
		}

		verified := actualSize == pkg.GetSizeBytes() &&
			strings.EqualFold(normalizeSha(actualSha), pkg.GetSha256())
		if verified {
			pkg.Status = models.PackageRecord_STATUS_READY
		} else {
			pkg.Status = models.PackageRecord_STATUS_FAILED
		}
		touch(pkg, s.now())
		if err := s.d.Packages.Update(ctx, pkg); err != nil {
			return nil, utils.MapErr(err)
		}
		if !verified {
			return nil, status.Errorf(codes.FailedPrecondition,
				"upload verification failed: declared size=%d sha256=%s, got size=%d sha256=%s",
				pkg.GetSizeBytes(), pkg.GetSha256(), actualSize, normalizeSha(actualSha))
		}
		return pkg, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.CompleteUploadResponse{Package: pkg}, nil
}

// GetPackage reads one tenant-private record. NO_SIDE_EFFECTS.
func (s *PackageService) GetPackage(ctx context.Context, req *api.GetPackageRequest) (*api.GetPackageResponse, error) {
	if err := requireTenant(req.GetTenantId()); err != nil {
		return nil, err
	}
	pkg, err := s.getOwned(ctx, req.GetTenantId(), req.GetId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.GetPackageResponse{Package: pkg}, nil
}

// ListPackages lists the tenant's records with optional Entity filter, format/
// db-kind facets, sort and pagination. NO_SIDE_EFFECTS. Tenant scoping is taken
// from the request tenant_id + interceptor, never from the client filter.
func (s *PackageService) ListPackages(ctx context.Context, req *api.ListPackagesRequest) (*api.ListPackagesResponse, error) {
	if err := requireTenant(req.GetTenantId()); err != nil {
		return nil, err
	}
	q := PackageQuery{
		TenantID:  req.GetTenantId(),
		Filter:    req.GetFilter(),
		Formats:   req.GetFormats(),
		DbKinds:   req.GetDbKinds(),
		Sort:      req.GetSort(),
		PageSize:  req.GetPage().GetSize(),
		PageToken: req.GetPage().GetToken(),
	}
	pkgs, next, err := s.d.Packages.List(ctx, q)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListPackagesResponse{Packages: pkgs, NextPageToken: next}, nil
}

// DeletePackage removes the record and its blob. Idempotent: deleting an absent
// package is a no-op. The metadata row and the blob are removed together; the
// blob delete is best-effort within the same flow (not-found tolerated).
func (s *PackageService) DeletePackage(ctx context.Context, req *api.DeletePackageRequest) (*api.DeletePackageResponse, error) {
	if err := requireTenant(req.GetTenantId()); err != nil {
		return nil, err
	}
	if err := s.doTx(ctx, func(ctx context.Context) error {
		pkg, err := s.getOwned(ctx, req.GetTenantId(), req.GetId())
		if errors.Is(err, derrors.ErrNotFound) {
			return nil // idempotent no-op
		}
		if err != nil {
			return utils.MapErr(err)
		}
		if err := s.d.Blobs.Delete(ctx, pkg.GetStorageUri()); err != nil && !errors.Is(err, derrors.ErrNotFound) {
			return utils.MapErr(err)
		}
		return utils.MapErr(derrors.IgnoreNotFound(s.d.Packages.Delete(ctx, req.GetTenantId(), req.GetId())))
	}); err != nil {
		return nil, err
	}
	return &api.DeletePackageResponse{}, nil
}

/*
	===== small helpers =====
*/

// touch bumps the record's updated_at, allocating the Entity/Timings chain if a
// repo handed back a sparse row.
func touch(pkg *models.PackageRecord, ts *timestamppb.Timestamp) {
	if pkg.Entity == nil {
		pkg.Entity = &common.Entity{}
	}
	if pkg.Entity.Timings == nil {
		pkg.Entity.Timings = &common.Timings{}
	}
	pkg.Entity.Timings.UpdatedAt = ts
}

// normalizeSha lowercases/trims a hex digest for case-insensitive comparison.
func normalizeSha(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
