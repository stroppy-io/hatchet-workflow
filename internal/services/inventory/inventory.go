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
// our network mirror against it.
//
// TODO(inventory): no implementation exists — needs a Yandex Cloud client +
// reconcile logic (D19/D20). Reported.
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

// ListNetworkAllocations returns the tenant's subnet allocations (our mirror).
//
// TODO(inventory): page_token pagination is ignored — returns all rows. Reported.
func (s *CloudInventoryService) ListNetworkAllocations(ctx context.Context, req *uipb.ListNetworkAllocationsRequest) (*models.NetworkAllocation_List, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListNetworkAllocations",
		func(ctx context.Context, _ trace.Span) (*models.NetworkAllocation_List, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			allocs, err := s.allocations.Query(ctx,
				models.NetworkAllocations.SelectAll().Where(
					models.NetworkAllocations.TenantId.Eq(req.GetTenantId().GetValue()),
					models.NetworkAllocations.DeletedAt.IsNull(),
				))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list network allocations: %v", err)
			}
			return &models.NetworkAllocation_List{NetworkAllocations: allocs}, nil
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
