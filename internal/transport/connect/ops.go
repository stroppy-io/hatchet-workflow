package connect

import (
	"context"

	"connectrpc.com/connect"

	opssvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/ops"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
)

// ─── WebhookHandler ──────────────────────────────────────────────────────────

// WebhookHandler implements opsconnect.WebhookServiceHandler.
type WebhookHandler struct {
	svc *opssvc.WebhookService
}

// NewWebhookHandler constructs a WebhookHandler.
func NewWebhookHandler(svc *opssvc.WebhookService) *WebhookHandler {
	return &WebhookHandler{svc: svc}
}

func (h *WebhookHandler) CreateWebhook(ctx context.Context, req *connect.Request[opspb.CreateWebhookRequest]) (*connect.Response[opspb.Webhook], error) {
	tenantID := &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	userID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	w, err := h.svc.CreateWebhook(ctx, tenantID, userID, req.Msg.GetWebhook())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(w), nil
}

func (h *WebhookHandler) UpdateWebhook(ctx context.Context, req *connect.Request[opspb.UpdateWebhookRequest]) (*connect.Response[opspb.Webhook], error) {
	w, err := h.svc.UpdateWebhook(ctx, req.Msg.GetWebhook())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(w), nil
}

func (h *WebhookHandler) DeleteWebhook(ctx context.Context, req *connect.Request[opspb.WebhookId]) (*connect.Response[opspb.Webhook], error) {
	deleted, err := h.svc.DeleteWebhook(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(deleted), nil
}

func (h *WebhookHandler) GetWebhook(ctx context.Context, req *connect.Request[opspb.WebhookId]) (*connect.Response[opspb.Webhook], error) {
	w, err := h.svc.GetWebhook(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(w), nil
}

func (h *WebhookHandler) ListWebhooks(ctx context.Context, req *connect.Request[iampb.TenantId]) (*connect.Response[opspb.Webhook_List], error) {
	list, err := h.svc.ListWebhooks(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&opspb.Webhook_List{Webhooks: list}), nil
}

func (h *WebhookHandler) TestWebhook(ctx context.Context, req *connect.Request[opspb.TestWebhookRequest]) (*connect.Response[opspb.TestWebhookResponse], error) {
	resp, err := h.svc.TestWebhook(ctx, req.Msg.GetId(), req.Msg.GetEvent())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// ─── QuotaHandler ────────────────────────────────────────────────────────────

// QuotaHandler implements opsconnect.QuotaServiceHandler.
type QuotaHandler struct {
	svc *opssvc.QuotaService
}

// NewQuotaHandler constructs a QuotaHandler.
func NewQuotaHandler(svc *opssvc.QuotaService) *QuotaHandler {
	return &QuotaHandler{svc: svc}
}

func (h *QuotaHandler) GetQuotas(ctx context.Context, req *connect.Request[opspb.GetQuotasRequest]) (*connect.Response[opspb.QuotaList], error) {
	list, err := h.svc.GetQuotas(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(list), nil
}

func (h *QuotaHandler) RefreshQuotas(ctx context.Context, req *connect.Request[opspb.RefreshQuotasRequest]) (*connect.Response[opspb.QuotaList], error) {
	list, err := h.svc.RefreshQuotas(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(list), nil
}

// ─── BinaryCacheHandler ──────────────────────────────────────────────────────

// BinaryCacheHandler implements agentconnect.BinaryCacheServiceHandler.
type BinaryCacheHandler struct {
	svc *opssvc.BinaryCacheService
}

// NewBinaryCacheHandler constructs a BinaryCacheHandler.
func NewBinaryCacheHandler(svc *opssvc.BinaryCacheService) *BinaryCacheHandler {
	return &BinaryCacheHandler{svc: svc}
}

func (h *BinaryCacheHandler) Resolve(ctx context.Context, req *connect.Request[agentpb.ResolveArtifactRequest]) (*connect.Response[agentpb.ResolveArtifactResponse], error) {
	resp, err := h.svc.Resolve(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
