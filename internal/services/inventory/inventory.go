// Package inventory implements the tenant-scoped CloudInventoryService. RBAC:
// FetchQuotas/ListNetworkAllocations = ADMIN, Reconcile = OWNER. The cloud is the
// source of truth for quotas/network; ListNetworkAllocations reads our mirror.
package inventory

import (
	"context"

	"github.com/gopherex/xlog"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	uiapi "github.com/stroppy-io/stroppy-cloud/internal/api/ui"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/authz"
	"github.com/stroppy-io/stroppy-cloud/internal/services/svcutil"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// CloudQuota talks to the cloud provider for live quota inventory and reconciles
// our network mirror against it. Implemented by services/yandexcloud over the real
// Yandex Cloud SDK (quota manager + compute instance listing) — FetchQuotas and
// Reconcile are live (D19/D20).
type CloudQuota interface {
	FetchQuotas(ctx context.Context, tenantID string, live bool) (*deployment.QuotaInventory, error)
	Reconcile(ctx context.Context, tenantID string) (*uipb.ReconcileResponse, error)
}

// CloudInventoryService implements ui.CloudInventoryActions.
type CloudInventoryService struct {
	*tracing.Entity
	allocations *repository.ProtoRepository[
		models.NetworkAllocationAlias,
		models.NetworkAllocationColumnAlias,
		*models.NetworkAllocationScanner,
		*models.NetworkAllocation,
	]
	cloud CloudQuota
	authz *authz.Authz
}

var _ uiapi.CloudInventoryActions = (*CloudInventoryService)(nil)

// NewCloudInventoryService builds the service.
func NewCloudInventoryService(logger *xlog.Logger, executor exec.DB, az *authz.Authz, cloud CloudQuota) *CloudInventoryService {
	return &CloudInventoryService{
		Entity: tracing.NewEntity(logger.AppendName("CloudInventoryService")),
		allocations: repository.NewProtoRepository(
			repository.NewScannerRepository(models.NetworkAllocations.Table, executor),
			models.NetworkAllocationConverter,
		),
		cloud: cloud,
		authz: az,
	}
}

// FetchQuotas returns the tenant's cloud quota inventory.
func (s *CloudInventoryService) FetchQuotas(ctx context.Context, req *uipb.FetchQuotasRequest) (*deployment.QuotaInventory, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "FetchQuotas",
		func(ctx context.Context, _ trace.Span) (*deployment.QuotaInventory, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			return s.cloud.FetchQuotas(ctx, req.GetTenantId().GetValue(), req.GetLive())
		})
}

// ListNetworkAllocations returns the tenant's subnet allocations (our mirror)
// with cursor pagination (newest-first by default), optionally filtered by
// provider / zone.
func (s *CloudInventoryService) ListNetworkAllocations(ctx context.Context, req *uipb.ListNetworkAllocationsRequest) (*uipb.ListNetworkAllocationsResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListNetworkAllocations",
		func(ctx context.Context, _ trace.Span) (*uipb.ListNetworkAllocationsResponse, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			size := svcutil.PageSize(req.GetPage())
			desc := svcutil.CursorDesc(req.GetOrder())
			q := models.NetworkAllocations.SelectAll().Where(
				models.NetworkAllocations.TenantId.Eq(req.GetTenantId().GetValue()),
				models.NetworkAllocations.DeletedAt.IsNull(),
			)
			if req.GetProvider() != deployment.Provider_PROVIDER_UNSPECIFIED {
				// Provider is stored as the enum String() value (network_plain converter).
				q = q.Where(models.NetworkAllocations.Provider.Eq(req.GetProvider().String()))
			}
			if req.Zone != nil {
				q = q.Where(models.NetworkAllocations.Zone.Eq(req.GetZone()))
			}
			// search: no Name column on network_allocations — match the CIDR (the
			// human-visible identifier) case-insensitively.
			if s := req.GetSearch(); s != "" {
				q = q.Where(models.NetworkAllocations.Cidr.ILike("%" + s + "%"))
			}
			// leased: a row is leased iff it has a (future) lease expiry recorded.
			if req.GetLeased() {
				q = q.Where(models.NetworkAllocations.LeaseExpiresAt.IsNotNull())
			}
			// tags: Tags is serialized JSON of common.Tags (a TEXT column) — match
			// each requested free tag and key=value label as a substring.
			for _, tag := range req.GetTags().GetTags() {
				q = q.Where(models.NetworkAllocations.Tags.ILike("%" + tag + "%"))
			}
			for k, v := range req.GetTags().GetLabels() {
				q = q.Where(models.NetworkAllocations.Tags.ILike("%" + k + "%" + v + "%"))
			}
			if tok := req.GetPage().GetToken(); tok != "" {
				if desc {
					q = q.Where(models.NetworkAllocations.Id.Lt(tok))
				} else {
					q = q.Where(models.NetworkAllocations.Id.Gt(tok))
				}
			}
			if desc {
				q = q.OrderByDESC(models.NetworkAllocationColumnId)
			} else {
				q = q.OrderByASC(models.NetworkAllocationColumnId)
			}
			rows, err := s.allocations.Query(ctx, q.Limit(size+1))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list network allocations: %v", err)
			}
			items, pageInfo := svcutil.Paginate(rows, size, func(n *models.NetworkAllocation) string {
				return n.GetId()
			})
			return &uipb.ListNetworkAllocationsResponse{NetworkAllocations: items, PageInfo: pageInfo}, nil
		})
}

// Reconcile syncs the tenant's network/quota mirror against the cloud.
func (s *CloudInventoryService) Reconcile(ctx context.Context, req *uipb.ReconcileRequest) (*uipb.ReconcileResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "Reconcile",
		func(ctx context.Context, _ trace.Span) (*uipb.ReconcileResponse, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_OWNER); err != nil {
				return nil, err
			}
			return s.cloud.Reconcile(ctx, req.GetTenantId().GetValue())
		})
}
