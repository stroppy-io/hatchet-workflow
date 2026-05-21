package tenancy

import (
	"context"
	"time"

	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/gopherex/xlog"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	adminapi "github.com/stroppy-io/stroppy-cloud/internal/api/admin"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// TenantAdminService implements platform-level tenant CRUD.
type TenantAdminService struct {
	*tracing.Entity
	tenants *repository.ProtoRepository[
		models.TenantAlias,
		models.TenantColumnAlias,
		*models.TenantScanner,
		*models.Tenant,
	]
	txm tx.Trm
}

var _ adminapi.TenantAdminActions = (*TenantAdminService)(nil)

// NewTenantAdminService builds the service over the given DB executor + tx manager.
func NewTenantAdminService(logger *xlog.Logger, executor exec.DB, txm tx.Trm) *TenantAdminService {
	return &TenantAdminService{
		Entity: tracing.NewEntity(logger.AppendName("TenantAdminService")),
		tenants: repository.NewProtoRepository(
			repository.NewScannerRepository(models.Tenants.Table, executor),
			models.TenantConverter,
		),
		txm: txm,
	}
}

// CreateTenant mints a new tenant with a server-assigned id.
func (s *TenantAdminService) CreateTenant(ctx context.Context, req *adminpb.CreateTenantRequest) (*models.Tenant, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CreateTenant",
		func(ctx context.Context, _ trace.Span) (*models.Tenant, error) {
			tenant := req.GetTenant()
			tenant.Entity = ids.NewEntity()
			return tx.DoReadCommittedRet(ctx, s.txm, func(ctx context.Context) (*models.Tenant, error) {
				if _, err := s.tenants.Execute(ctx,
					models.Tenants.Insert().From(tenant.IntoPlain().AllSetters()...)); err != nil {
					return nil, status.Errorf(codes.Internal, "insert tenant: %v", err)
				}
				return tenant, nil
			})
		})
}

// UpdateTenant updates the mutable tenant fields (owner).
//
// TODO(tenancy): update_mask is NOT yet honored — owner_account_id is written.
// Reported.
func (s *TenantAdminService) UpdateTenant(ctx context.Context, req *adminpb.UpdateTenantRequest) (*models.Tenant, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "UpdateTenant",
		func(ctx context.Context, _ trace.Span) (*models.Tenant, error) {
			tenant := req.GetTenant()
			id := tenant.GetEntity().GetId().GetValue()
			if id == "" {
				return nil, status.Error(codes.InvalidArgument, "tenant id required")
			}
			updated, err := s.tenants.QueryRow(ctx,
				models.Tenants.Update().Set(
					models.Tenants.OwnerAccountId.Set(tenant.GetOwnerAccountId().GetValue()),
					models.Tenants.UpdatedAt.Set(time.Now()),
				).Where(models.Tenants.Id.Eq(id)).ReturningAll(),
			)
			if err != nil {
				return nil, notFound(err, "update tenant")
			}
			return updated, nil
		})
}

// DeleteTenant soft-deletes the tenant and returns the deleted row.
func (s *TenantAdminService) DeleteTenant(ctx context.Context, id *models.TenantId) (*models.Tenant, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "DeleteTenant",
		func(ctx context.Context, _ trace.Span) (*models.Tenant, error) {
			now := time.Now()
			deleted, err := s.tenants.QueryRow(ctx,
				models.Tenants.Update().Set(
					models.Tenants.DeletedAt.Set(&now),
					models.Tenants.UpdatedAt.Set(now),
				).Where(models.Tenants.Id.Eq(id.GetValue())).ReturningAll(),
			)
			if err != nil {
				return nil, notFound(err, "delete tenant")
			}
			return deleted, nil
		})
}
