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

// WebhookActions is the dependency the WebhookService is built on: tenant-scoped
// webhook subscriptions (ADMIN). internal/services implements it.
type WebhookActions interface {
	CreateWebhook(ctx context.Context, req *uipb.CreateWebhookRequest) (*models.Webhook, error)
	ListWebhooks(ctx context.Context, req *uipb.ListWebhooksRequest) (*models.Webhook_List, error)
	UpdateWebhook(ctx context.Context, req *uipb.UpdateWebhookRequest) (*models.Webhook, error)
	DeleteWebhook(ctx context.Context, req *uipb.DeleteWebhookRequest) (*emptypb.Empty, error)
	TestWebhook(ctx context.Context, req *uipb.TestWebhookRequest) (*emptypb.Empty, error)
}

// WebhookService is the gRPC handler for cloud.v1.api.ui.WebhookService. Pure
// transport: trace the call and delegate to svc.
type WebhookService struct {
	uipb.UnimplementedWebhookServiceServer
	*tracing.Entity
	svc WebhookActions
}

var _ uipb.WebhookServiceServer = (*WebhookService)(nil)

func NewWebhookService(logger *xlog.Logger, svc WebhookActions) *WebhookService {
	return &WebhookService{
		Entity: tracing.NewEntity(logger.AppendName("WebhookService")),
		svc:    svc,
	}
}

func (s *WebhookService) CreateWebhook(ctx context.Context, req *uipb.CreateWebhookRequest) (*models.Webhook, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CreateWebhook",
		func(ctx context.Context, _ trace.Span) (*models.Webhook, error) {
			return s.svc.CreateWebhook(ctx, req)
		})
}

func (s *WebhookService) ListWebhooks(ctx context.Context, req *uipb.ListWebhooksRequest) (*models.Webhook_List, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListWebhooks",
		func(ctx context.Context, _ trace.Span) (*models.Webhook_List, error) {
			return s.svc.ListWebhooks(ctx, req)
		})
}

func (s *WebhookService) UpdateWebhook(ctx context.Context, req *uipb.UpdateWebhookRequest) (*models.Webhook, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "UpdateWebhook",
		func(ctx context.Context, _ trace.Span) (*models.Webhook, error) {
			return s.svc.UpdateWebhook(ctx, req)
		})
}

func (s *WebhookService) DeleteWebhook(ctx context.Context, req *uipb.DeleteWebhookRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "DeleteWebhook",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			return s.svc.DeleteWebhook(ctx, req)
		})
}

func (s *WebhookService) TestWebhook(ctx context.Context, req *uipb.TestWebhookRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "TestWebhook",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			return s.svc.TestWebhook(ctx, req)
		})
}
