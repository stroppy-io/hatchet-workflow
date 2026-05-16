package catalog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	s3pkg "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/s3"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	errorspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/errors"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// PackageStorage abstracts S3-backed binary storage for packages.
type PackageStorage interface {
	PutObject(ctx context.Context, key string, body io.Reader, contentType string) (string, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	Delete(ctx context.Context, key string) error
}

// s3PackageStorage adapts *s3pkg.Client to PackageStorage.
type s3PackageStorage struct {
	cli *s3pkg.Client
}

// NewS3PackageStorage wraps an s3 Client as a PackageStorage.
func NewS3PackageStorage(cli *s3pkg.Client) PackageStorage {
	return &s3PackageStorage{cli: cli}
}

func (s *s3PackageStorage) PutObject(ctx context.Context, key string, body io.Reader, contentType string) (string, error) {
	return s.cli.PutObject(ctx, key, body, contentType)
}

func (s *s3PackageStorage) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return s.cli.PresignGet(ctx, key, ttl)
}

func (s *s3PackageStorage) Delete(ctx context.Context, key string) error {
	return s.cli.Delete(ctx, key)
}

// SetPackageStorage injects a PackageStorage into the service.
func (s *Service) SetPackageStorage(p PackageStorage) {
	s.pkgStorage = p
}

// CreatePackage inserts a new package row.
// If pkg.IsBuiltin is false the package is scoped to tenantID; builtin packages have tenant_id = NULL.
func (s *Service) CreatePackage(ctx context.Context, tenantID *iampb.TenantId, createdBy *iampb.UserId, pkg *catalogpb.Package) (*catalogpb.Package, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CreatePackage",
		func(ctx context.Context, _ trace.Span) (*catalogpb.Package, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*catalogpb.Package, error) {
					now := timestamppb.Now()
					pkg.Id = &catalogpb.PackageId{Value: ids.New()}
					if !pkg.GetIsBuiltin() {
						pkg.TenantId = tenantID
					}
					pkg.CreatedBy = createdBy
					pkg.Timestamps = &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now}
					scanner := pkg.IntoPlain()
					if scanner.Label == nil {
						scanner.Label = []string{}
					}
					if _, err := s.packageRepo.Execute(ctx,
						catalogpb.Packages.Insert().From(scanner.AllSetters()...),
					); err != nil {
						return nil, err
					}
					return pkg, nil
				})
		})
}

// GetPackage returns a package by ID.
func (s *Service) GetPackage(ctx context.Context, id *catalogpb.PackageId) (*catalogpb.Package, error) {
	p, err := s.packageRepo.QueryRow(ctx,
		catalogpb.Packages.SelectAll().Where(
			catalogpb.Packages.Id.Eq(id.GetValue()),
			catalogpb.Packages.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("package", id.GetValue()))
		}
		return nil, err
	}
	return p, nil
}

// ListPackages returns packages visible to the given tenant:
// those with tenant_id IS NULL (builtins) or tenant_id = tenantID.
// An optional dbKind filter narrows the result further.
func (s *Service) ListPackages(ctx context.Context, tenantID *iampb.TenantId, dbKind *catalogpb.Database_Kind) ([]*catalogpb.Package, error) {
	tid := tenantID.GetValue()
	tenantClause := catalogpb.Packages.TenantId.Or(
		catalogpb.Packages.TenantId.IsNull(),
		catalogpb.Packages.TenantId.Eq(&tid),
	)
	notDeleted := catalogpb.Packages.DeletedAt.IsNull()

	if dbKind != nil {
		return s.packageRepo.Query(ctx,
			catalogpb.Packages.SelectAll().Where(
				tenantClause,
				notDeleted,
				catalogpb.Packages.DbKind.Eq(dbKind.String()),
			),
		)
	}
	return s.packageRepo.Query(ctx,
		catalogpb.Packages.SelectAll().Where(
			tenantClause,
			notDeleted,
		),
	)
}

// DeletePackage soft-deletes a package by ID.
func (s *Service) DeletePackage(ctx context.Context, id *catalogpb.PackageId) error {
	now := time.Now()
	n, err := s.packageRepo.Execute(ctx,
		catalogpb.Packages.Update().
			Set(catalogpb.Packages.DeletedAt.Set(&now)).
			Where(
				catalogpb.Packages.Id.Eq(id.GetValue()),
				catalogpb.Packages.DeletedAt.IsNull(),
			),
	)
	if err != nil {
		return err
	}
	if n == 0 {
		return domainerr.NotFound(domainerr.ResourceInfo("package", id.GetValue()))
	}
	return nil
}

// ClonePackage creates a copy of an existing package under the same tenant.
func (s *Service) ClonePackage(ctx context.Context, id *catalogpb.PackageId, callerID *iampb.UserId) (*catalogpb.Package, error) {
	original, err := s.GetPackage(ctx, id)
	if err != nil {
		return nil, err
	}
	cloned := &catalogpb.Package{
		TenantId:  original.GetTenantId(),
		Identity:  original.GetIdentity(),
		DbKind:    original.GetDbKind(),
		DbVersion: original.GetDbVersion(),
		IsBuiltin: original.GetIsBuiltin(),
		Source:    original.GetSource(),
	}
	return s.CreatePackage(ctx, original.GetTenantId(), callerID, cloned)
}

func safeFilename(name string) (string, error) {
	base := filepath.Base(name)
	if base == "" || base == "." || base == ".." || base != name {
		return "", domainerr.InvalidArgument(domainerr.FieldViolation("filename", "must be a plain file name, no path separators"))
	}
	return base, nil
}

// UploadPackageBinary streams a binary to S3 and returns the storage key.
// Key format: packages/<tenantID>/<packageULID>/<filename>.
func (s *Service) UploadPackageBinary(ctx context.Context, pkgID *catalogpb.PackageId, tenantID *iampb.TenantId, filename string, body io.Reader, contentType string) (string, error) {
	if s.pkgStorage == nil {
		return "", domainerr.E(errorspb.Code_CODE_INTERNAL).WithCause(fmt.Errorf("package storage not configured"))
	}
	safe, err := safeFilename(filename)
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf("packages/%s/%s/%s", tenantID.GetValue(), pkgID.GetValue(), safe)
	if _, err := s.pkgStorage.PutObject(ctx, key, body, contentType); err != nil {
		return "", err
	}
	return key, nil
}

// PresignPackageDownload returns a presigned GET URL valid for 10 minutes.
func (s *Service) PresignPackageDownload(ctx context.Context, pkgID *catalogpb.PackageId, tenantID *iampb.TenantId, filename string) (string, error) {
	if s.pkgStorage == nil {
		return "", domainerr.E(errorspb.Code_CODE_INTERNAL).WithCause(fmt.Errorf("package storage not configured"))
	}
	safe, err := safeFilename(filename)
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf("packages/%s/%s/%s", tenantID.GetValue(), pkgID.GetValue(), safe)
	return s.pkgStorage.PresignGet(ctx, key, 10*time.Minute)
}
