package ui

import (
	"context"

	"github.com/gopherex/xlog"
	"go.opentelemetry.io/otel/trace"

	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// CloudInventoryActions is the dependency the CloudInventoryService is built on:
// tenant-scoped cloud quota/network inventory (OWNER). Cloud is the source of
// truth; Reconcile syncs allocations. internal/services implements it.
type CloudInventoryActions interface {
	FetchQuotas(ctx context.Context, req *uipb.FetchQuotasRequest) (*deployment.QuotaInventory, error)
	ListNetworkAllocations(ctx context.Context, req *uipb.ListNetworkAllocationsRequest) (*uipb.ListNetworkAllocationsResponse, error)
	Reconcile(ctx context.Context, req *uipb.ReconcileRequest) (*uipb.ReconcileResponse, error)
}

// CloudInventoryService is the gRPC handler for cloud.v1.api.ui.CloudInventoryService.
// Pure transport: trace the call and delegate to svc.
type CloudInventoryService struct {
	uipb.UnimplementedCloudInventoryServiceServer
	*tracing.Entity
	svc CloudInventoryActions
}

var _ uipb.CloudInventoryServiceServer = (*CloudInventoryService)(nil)

func NewCloudInventoryService(logger *xlog.Logger, svc CloudInventoryActions) *CloudInventoryService {
	return &CloudInventoryService{
		Entity: tracing.NewEntity(logger.AppendName("CloudInventoryService")),
		svc:    svc,
	}
}

func (s *CloudInventoryService) FetchQuotas(ctx context.Context, req *uipb.FetchQuotasRequest) (*deployment.QuotaInventory, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "FetchQuotas",
		func(ctx context.Context, _ trace.Span) (*deployment.QuotaInventory, error) {
			return s.svc.FetchQuotas(ctx, req)
		})
}

func (s *CloudInventoryService) ListNetworkAllocations(ctx context.Context, req *uipb.ListNetworkAllocationsRequest) (*uipb.ListNetworkAllocationsResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListNetworkAllocations",
		func(ctx context.Context, _ trace.Span) (*uipb.ListNetworkAllocationsResponse, error) {
			return s.svc.ListNetworkAllocations(ctx, req)
		})
}

func (s *CloudInventoryService) Reconcile(ctx context.Context, req *uipb.ReconcileRequest) (*uipb.ReconcileResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "Reconcile",
		func(ctx context.Context, _ trace.Span) (*uipb.ReconcileResponse, error) {
			return s.svc.Reconcile(ctx, req)
		})
}
