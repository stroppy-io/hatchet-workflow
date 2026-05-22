package tenancy

import (
	"context"
	"time"

	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/gopherex/xlog"
	"github.com/yaroher/ratel/pkg/dml/set"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"github.com/yaroher/ratel/pkg/types"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	adminapi "github.com/stroppy-io/stroppy-cloud/internal/api/admin"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/svcutil"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// maskedSetters builds the column setters for a partial UPDATE that honors a
// proto FieldMask. getSetter is the scanner's GetSetter (binds a column to the
// scanner's already-converted field value); paths are mask.GetPaths(); pathToCol
// maps proto field paths to columns; full is the default column set written when
// the mask is empty/nil (back-compat full-replace). Unknown paths are ignored;
// updated_at is added by the caller.
func maskedSetters[C interface {
	types.ColumnAlias
	comparable
}](
	getSetter func(C) func() set.ValueSetter[C],
	paths []string,
	pathToCol map[string]C,
	full []C,
) []set.ValueSetter[C] {
	cols := full
	if len(paths) > 0 {
		cols = cols[:0:0]
		seen := make(map[C]struct{}, len(paths))
		for _, p := range paths {
			col, ok := pathToCol[p]
			if !ok {
				continue
			}
			if _, dup := seen[col]; dup {
				continue
			}
			seen[col] = struct{}{}
			cols = append(cols, col)
		}
	}
	setters := make([]set.ValueSetter[C], 0, len(cols))
	for _, col := range cols {
		setters = append(setters, getSetter(col)())
	}
	return setters
}

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

// ListTenants returns the platform's tenants (cross-tenant; gated by the
// is_admin interceptor) with cursor pagination (newest-first by default),
// optionally filtered by owner account.
func (s *TenantAdminService) ListTenants(ctx context.Context, req *adminpb.ListTenantsRequest) (*adminpb.ListTenantsResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListTenants",
		func(ctx context.Context, _ trace.Span) (*adminpb.ListTenantsResponse, error) {
			size := svcutil.PageSize(req.GetPage())
			desc := svcutil.CursorDesc(req.GetOrder())
			q := models.Tenants.SelectAll().Where(
				models.Tenants.DeletedAt.IsNull(),
			)
			if req.OwnerAccountId != nil {
				q = q.Where(models.Tenants.OwnerAccountId.Eq(req.GetOwnerAccountId().GetValue()))
			}
			// search: match the tenant name (NullText column → raw ILIKE).
			if req.GetSearch() != "" {
				q = q.Where(models.Tenants.Name.Raw("ILIKE", "?", "%"+req.GetSearch()+"%"))
			}
			// tags: Tags is serialized JSON of common.Tags (a TEXT column) — match
			// each requested free tag and key=value label as a substring.
			for _, tag := range req.GetTags().GetTags() {
				q = q.Where(models.Tenants.Tags.ILike("%" + tag + "%"))
			}
			for k, v := range req.GetTags().GetLabels() {
				q = q.Where(models.Tenants.Tags.ILike("%" + k + "%" + v + "%"))
			}
			if tok := req.GetPage().GetToken(); tok != "" {
				if desc {
					q = q.Where(models.Tenants.Id.Lt(tok))
				} else {
					q = q.Where(models.Tenants.Id.Gt(tok))
				}
			}
			if desc {
				q = q.OrderByDESC(models.TenantColumnId)
			} else {
				q = q.OrderByASC(models.TenantColumnId)
			}
			rows, err := s.tenants.Query(ctx, q.Limit(size+1))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list tenants: %v", err)
			}
			items, pageInfo := svcutil.Paginate(rows, size, func(t *models.Tenant) string {
				return t.GetEntity().GetId().GetValue()
			})
			return &adminpb.ListTenantsResponse{Tenants: items, PageInfo: pageInfo}, nil
		})
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

// tenantUpdatableColumns maps proto field paths (UpdateTenantRequest.tenant) to
// the mutable tenant columns the mask may select. id/created_at and tenancy
// invariants are never writable here.
var tenantUpdatableColumns = map[string]models.TenantColumnAlias{
	"owner_account_id": models.TenantColumnOwnerAccountId,
	"ownerAccountId":   models.TenantColumnOwnerAccountId,
	"name":             models.TenantColumnName,
	"tags":             models.TenantColumnTags,
}

// UpdateTenant updates the mutable tenant fields. When req.update_mask names
// paths, only those columns are written; an empty/nil mask is full-replace of
// the mutable fields (back-compat). updated_at is always bumped.
func (s *TenantAdminService) UpdateTenant(ctx context.Context, req *adminpb.UpdateTenantRequest) (*models.Tenant, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "UpdateTenant",
		func(ctx context.Context, _ trace.Span) (*models.Tenant, error) {
			tenant := req.GetTenant()
			id := tenant.GetEntity().GetId().GetValue()
			if id == "" {
				return nil, status.Error(codes.InvalidArgument, "tenant id required")
			}
			scanner := tenant.IntoPlain()
			setters := maskedSetters(
				scanner.GetSetter,
				req.GetUpdateMask().GetPaths(),
				tenantUpdatableColumns,
				// nil/empty mask = original full-replace set (back-compat):
				// owner_account_id only. name/tags are mask-only.
				[]models.TenantColumnAlias{
					models.TenantColumnOwnerAccountId,
				},
			)
			setters = append(setters, models.Tenants.UpdatedAt.Set(time.Now()))
			updated, err := s.tenants.QueryRow(ctx,
				models.Tenants.Update().Set(setters...).
					Where(models.Tenants.Id.Eq(id)).ReturningAll(),
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
