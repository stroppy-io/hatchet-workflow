// Package webhook implements the tenant-scoped WebhookService. RBAC: all ADMIN
// (rbac.feature). The secret is write-only (stored, never returned).
package webhook

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

// Sender delivers a webhook payload to a URL, signing it with the secret.
// Implemented by internal/infrastructure/webhooksender.
type Sender interface {
	Send(ctx context.Context, url, secret string, payload []byte) error
}

// WebhookService implements ui.WebhookActions.
type WebhookService struct {
	*tracing.Entity
	hooks *repository.ProtoRepository[
		models.WebhookAlias,
		models.WebhookColumnAlias,
		*models.WebhookScanner,
		*models.Webhook,
	]
	sender Sender
	authz  *authz.Authz
	txm    tx.Trm
}

var _ uiapi.WebhookActions = (*WebhookService)(nil)

// NewWebhookService builds the service.
func NewWebhookService(logger *xlog.Logger, executor exec.DB, txm tx.Trm, az *authz.Authz, sender Sender) *WebhookService {
	return &WebhookService{
		Entity: tracing.NewEntity(logger.AppendName("WebhookService")),
		hooks: repository.NewProtoRepository(
			repository.NewScannerRepository(models.Webhooks.Table, executor),
			models.WebhookConverter,
		),
		sender: sender,
		authz:  az,
		txm:    txm,
	}
}

// CreateWebhook registers a webhook owned by the caller; the optional secret is
// stored write-only.
func (s *WebhookService) CreateWebhook(ctx context.Context, req *uipb.CreateWebhookRequest) (*models.Webhook, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CreateWebhook",
		func(ctx context.Context, _ trace.Span) (*models.Webhook, error) {
			c := svcutil.CallerOf(ctx)
			if err := s.authz.Require(ctx, c, req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			hook := req.GetWebhook()
			hook.Entity = ids.NewEntity()
			hook.Owned = &models.Own{OwnerAccountId: c.AccountID, TenantId: req.GetTenantId()}
			scanner := hook.IntoPlain()
			if sec := req.GetSecret(); sec != "" {
				scanner.Secret = sec
			}
			return tx.DoReadCommittedRet(ctx, s.txm, func(ctx context.Context) (*models.Webhook, error) {
				if _, err := s.hooks.Scanner().Execute(ctx,
					models.Webhooks.Insert().From(scanner.AllSetters()...)); err != nil {
					return nil, status.Errorf(codes.Internal, "insert webhook: %v", err)
				}
				return hook, nil
			})
		})
}

// ListWebhooks returns the tenant's webhooks.
func (s *WebhookService) ListWebhooks(ctx context.Context, req *uipb.ListWebhooksRequest) (*models.Webhook_List, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListWebhooks",
		func(ctx context.Context, _ trace.Span) (*models.Webhook_List, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			hooks, err := s.hooks.Query(ctx,
				models.Webhooks.SelectAll().Where(
					models.Webhooks.TenantId.Eq(req.GetTenantId().GetValue()),
					models.Webhooks.DeletedAt.IsNull(),
				))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list webhooks: %v", err)
			}
			return &models.Webhook_List{Webhooks: hooks}, nil
		})
}

// UpdateWebhook replaces a webhook's mutable fields (id + created_at preserved).
//
// TODO(webhook): update_mask not honored — full replace. Reported.
func (s *WebhookService) UpdateWebhook(ctx context.Context, req *uipb.UpdateWebhookRequest) (*models.Webhook, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "UpdateWebhook",
		func(ctx context.Context, _ trace.Span) (*models.Webhook, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			hook := req.GetWebhook()
			id := hook.GetEntity().GetId().GetValue()
			existing, err := s.hooks.QueryRow(ctx,
				models.Webhooks.SelectAll().Where(
					models.Webhooks.Id.Eq(id),
					models.Webhooks.TenantId.Eq(req.GetTenantId().GetValue()),
					models.Webhooks.DeletedAt.IsNull(),
				))
			if err != nil {
				return nil, svcutil.NotFound(err, "webhook")
			}
			hook.Entity.Id = existing.GetEntity().GetId()
			hook.Entity.Timestamps = existing.GetEntity().GetTimestamps()
			hook.Owned = existing.GetOwned()
			scanner := hook.IntoPlain()
			scanner.UpdatedAt = time.Now()
			if sec := req.GetSecret(); sec != "" {
				scanner.Secret = sec
			}
			updated, err := s.hooks.QueryRow(ctx,
				models.Webhooks.Update().Set(scanner.AllSetters()...).
					Where(models.Webhooks.Id.Eq(id)).ReturningAll())
			if err != nil {
				return nil, status.Errorf(codes.Internal, "update webhook: %v", err)
			}
			return updated, nil
		})
}

// DeleteWebhook soft-deletes a webhook.
func (s *WebhookService) DeleteWebhook(ctx context.Context, req *uipb.DeleteWebhookRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "DeleteWebhook",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			now := time.Now()
			if _, err := s.hooks.Execute(ctx,
				models.Webhooks.Update().Set(
					models.Webhooks.DeletedAt.Set(&now),
					models.Webhooks.UpdatedAt.Set(now),
				).Where(
					models.Webhooks.Id.Eq(req.GetId().GetValue()),
					models.Webhooks.TenantId.Eq(req.GetTenantId().GetValue()),
				)); err != nil {
				return nil, status.Errorf(codes.Internal, "delete webhook: %v", err)
			}
			return &emptypb.Empty{}, nil
		})
}

// TestWebhook delivers a test payload to the webhook, signed with its secret.
func (s *WebhookService) TestWebhook(ctx context.Context, req *uipb.TestWebhookRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "TestWebhook",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			hook, err := s.hooks.QueryRow(ctx, models.Webhooks.SelectAll().Where(
				models.Webhooks.Id.Eq(req.GetId().GetValue()),
				models.Webhooks.TenantId.Eq(req.GetTenantId().GetValue()),
				models.Webhooks.DeletedAt.IsNull(),
			))
			if err != nil {
				return nil, svcutil.NotFound(err, "webhook")
			}
			// secret is write-only (not on the proto) — read it from the scanner.
			secret, err := s.hooks.Scanner().QueryRow(ctx, models.Webhooks.Select(models.WebhookColumnSecret).Where(
				models.Webhooks.Id.Eq(req.GetId().GetValue()),
			))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "load secret: %v", err)
			}
			payload := []byte(fmt.Sprintf(`{"event":"test","webhook_id":%q}`, req.GetId().GetValue()))
			if err := s.sender.Send(ctx, hook.GetUrl(), secret.Secret, payload); err != nil {
				return nil, status.Errorf(codes.Unavailable, "deliver test webhook: %v", err)
			}
			return &emptypb.Empty{}, nil
		})
}
