package ui

import (
	"context"

	"github.com/gopherex/xlog"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// SettingsActions is the dependency the SettingsService is built on: per-tenant
// settings items (provider creds etc; read=ADMIN masked, write=OWNER).
// internal/services implements it.
type SettingsActions interface {
	ListSettingsItems(ctx context.Context, req *uipb.ListSettingsItemsRequest) (*uipb.ListSettingsItemsResponse, error)
	GetSettingsItem(ctx context.Context, req *uipb.GetSettingsItemRequest) (*models.SettingsItem, error)
	SetSettingsItem(ctx context.Context, req *uipb.SetSettingsItemRequest) (*models.SettingsItem, error)
	DeleteSettingsItem(ctx context.Context, req *uipb.DeleteSettingsItemRequest) (*emptypb.Empty, error)
}

// SettingsService is the gRPC handler for cloud.v1.api.ui.SettingsService. Pure
// transport: trace the call and delegate to svc.
type SettingsService struct {
	uipb.UnimplementedSettingsServiceServer
	*tracing.Entity
	svc SettingsActions
}

var _ uipb.SettingsServiceServer = (*SettingsService)(nil)

func NewSettingsService(logger *xlog.Logger, svc SettingsActions) *SettingsService {
	return &SettingsService{
		Entity: tracing.NewEntity(logger.AppendName("SettingsService")),
		svc:    svc,
	}
}

func (s *SettingsService) ListSettingsItems(ctx context.Context, req *uipb.ListSettingsItemsRequest) (*uipb.ListSettingsItemsResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListSettingsItems",
		func(ctx context.Context, _ trace.Span) (*uipb.ListSettingsItemsResponse, error) {
			return s.svc.ListSettingsItems(ctx, req)
		})
}

func (s *SettingsService) GetSettingsItem(ctx context.Context, req *uipb.GetSettingsItemRequest) (*models.SettingsItem, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "GetSettingsItem",
		func(ctx context.Context, _ trace.Span) (*models.SettingsItem, error) {
			return s.svc.GetSettingsItem(ctx, req)
		})
}

func (s *SettingsService) SetSettingsItem(ctx context.Context, req *uipb.SetSettingsItemRequest) (*models.SettingsItem, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "SetSettingsItem",
		func(ctx context.Context, _ trace.Span) (*models.SettingsItem, error) {
			return s.svc.SetSettingsItem(ctx, req)
		})
}

func (s *SettingsService) DeleteSettingsItem(ctx context.Context, req *uipb.DeleteSettingsItemRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "DeleteSettingsItem",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			return s.svc.DeleteSettingsItem(ctx, req)
		})
}
