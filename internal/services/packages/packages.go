// Package packages implements the tenant-scoped PackageService (custom .deb
// catalog). RBAC: list = VIEWER, upload/delete = ADMIN. Bytes are uploaded by the
// client via a presigned PUT URL (G2), not through RPC.
package packages

import (
	"context"
	"fmt"
	"time"

	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/gopherex/xlog"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	uiapi "github.com/stroppy-io/stroppy-cloud/internal/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/authz"
	"github.com/stroppy-io/stroppy-cloud/internal/services/svcutil"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// Uploader issues a presigned PUT URL for an object key (S3, G2). Implemented by
// internal/infrastructure/s3 over the aws-sdk PresignClient.PresignPutObject.
type Uploader interface {
	PresignPut(ctx context.Context, objectKey string, ttl time.Duration) (uploadURL string, err error)
}

// presignTTL is how long an upload URL stays valid.
const presignTTL = 15 * time.Minute

// PackageService implements ui.PackageActions.
type PackageService struct {
	*tracing.Entity
	packages *repository.ProtoRepository[
		models.PackageAlias,
		models.PackageColumnAlias,
		*models.PackageScanner,
		*models.Package,
	]
	uploader Uploader
	authz    *authz.Authz
	txm      tx.Trm
}

var _ uiapi.PackageActions = (*PackageService)(nil)

// NewPackageService builds the service.
func NewPackageService(logger *xlog.Logger, executor exec.DB, txm tx.Trm, az *authz.Authz, uploader Uploader) *PackageService {
	return &PackageService{
		Entity: tracing.NewEntity(logger.AppendName("PackageService")),
		packages: repository.NewProtoRepository(
			repository.NewScannerRepository(models.Packages.Table, executor),
			models.PackageConverter,
		),
		uploader: uploader,
		authz:    az,
		txm:      txm,
	}
}

// ListPackages returns the tenant's packages with cursor pagination
// (newest-first by default), optionally filtered by db kind/version/builtin.
func (s *PackageService) ListPackages(ctx context.Context, req *uipb.ListPackagesRequest) (*uipb.ListPackagesResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListPackages",
		func(ctx context.Context, _ trace.Span) (*uipb.ListPackagesResponse, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			size := svcutil.PageSize(req.GetPage())
			desc := svcutil.CursorDesc(req.GetOrder())
			q := models.Packages.SelectAll().Where(
				models.Packages.TenantId.Eq(req.GetTenantId().GetValue()),
				models.Packages.DeletedAt.IsNull(),
			)
			if req.DbKind != nil {
				q = q.Where(models.Packages.DbKind.Eq(req.GetDbKind()))
			}
			if req.DbVersion != nil {
				q = q.Where(models.Packages.DbVersion.Eq(req.GetDbVersion()))
			}
			if req.IsBuiltin != nil {
				q = q.Where(models.Packages.IsBuiltin.Eq(req.GetIsBuiltin()))
			}
			// search: match the package name (free text).
			if req.GetSearch() != "" {
				q = q.Where(models.Packages.Name.ILike("%" + req.GetSearch() + "%"))
			}
			// tags: Tags is serialized JSON of common.Tags (a TEXT column) — match
			// each requested free tag and key=value label as a substring.
			for _, tag := range req.GetTags().GetTags() {
				q = q.Where(models.Packages.Tags.ILike("%" + tag + "%"))
			}
			for k, v := range req.GetTags().GetLabels() {
				q = q.Where(models.Packages.Tags.ILike("%" + k + "%" + v + "%"))
			}
			if tok := req.GetPage().GetToken(); tok != "" {
				if desc {
					q = q.Where(models.Packages.Id.Lt(tok))
				} else {
					q = q.Where(models.Packages.Id.Gt(tok))
				}
			}
			if desc {
				q = q.OrderByDESC(models.PackageColumnId)
			} else {
				q = q.OrderByASC(models.PackageColumnId)
			}
			rows, err := s.packages.Query(ctx, q.Limit(size+1))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list packages: %v", err)
			}
			items, pageInfo := svcutil.Paginate(rows, size, func(p *models.Package) string {
				return p.GetEntity().GetId().GetValue()
			})
			return &uipb.ListPackagesResponse{Packages: items, PageInfo: pageInfo}, nil
		})
}

// RequestPackageUpload registers a package row and returns a presigned PUT URL
// the client uses to upload the .deb bytes.
func (s *PackageService) RequestPackageUpload(ctx context.Context, req *uipb.RequestPackageUploadRequest) (*uipb.RequestPackageUploadResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "RequestPackageUpload",
		func(ctx context.Context, _ trace.Span) (*uipb.RequestPackageUploadResponse, error) {
			c := svcutil.CallerOf(ctx)
			if err := s.authz.Require(ctx, c, req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			entity := ids.NewEntity()
			objectKey := fmt.Sprintf("packages/%s/%s/%s",
				req.GetTenantId().GetValue(), entity.GetId().GetValue(), req.GetDebFilename())
			pkg := &models.Package{
				Entity:       entity,
				Owned:        &models.Own{OwnerAccountId: c.AccountID, TenantId: req.GetTenantId()},
				Name:         req.GetName(),
				DbKind:       req.GetDbKind(),
				DbVersion:    req.GetDbVersion(),
				DebObjectUri: objectKey,
				IsBuiltin:    false,
			}
			if _, err := tx.DoReadCommittedRet(ctx, s.txm, func(ctx context.Context) (struct{}, error) {
				_, err := s.packages.Execute(ctx,
					models.Packages.Insert().From(pkg.IntoPlain().AllSetters()...))
				return struct{}{}, err
			}); err != nil {
				return nil, status.Errorf(codes.Internal, "insert package: %v", err)
			}
			uploadURL, err := s.uploader.PresignPut(ctx, objectKey, presignTTL)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "presign upload: %v", err)
			}
			return &uipb.RequestPackageUploadResponse{Package: pkg, UploadUrl: uploadURL}, nil
		})
}

// DeletePackage soft-deletes a package.
func (s *PackageService) DeletePackage(ctx context.Context, req *uipb.DeletePackageRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "DeletePackage",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			now := time.Now()
			if _, err := s.packages.Execute(ctx,
				models.Packages.Update().Set(
					models.Packages.DeletedAt.Set(&now),
					models.Packages.UpdatedAt.Set(now),
				).Where(
					models.Packages.Id.Eq(req.GetId().GetValue()),
					models.Packages.TenantId.Eq(req.GetTenantId().GetValue()),
				)); err != nil {
				return nil, status.Errorf(codes.Internal, "delete package: %v", err)
			}
			return &emptypb.Empty{}, nil
		})
}
