// Package webhook implements the tenant-scoped WebhookService. RBAC: all ADMIN
// (rbac.feature). The secret is write-only (stored, never returned).
package webhook

import (
	"context"
	"fmt"
	"time"

	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/gopherex/xlog"
	"github.com/yaroher/ratel/pkg/dml/set"
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

// ListWebhooks returns the tenant's webhooks with cursor pagination
// (newest-first by default).
func (s *WebhookService) ListWebhooks(ctx context.Context, req *uipb.ListWebhooksRequest) (*uipb.ListWebhooksResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListWebhooks",
		func(ctx context.Context, _ trace.Span) (*uipb.ListWebhooksResponse, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			size := svcutil.PageSize(req.GetPage())
			desc := svcutil.CursorDesc(req.GetOrder())
			q := models.Webhooks.SelectAll().Where(
				models.Webhooks.TenantId.Eq(req.GetTenantId().GetValue()),
				models.Webhooks.DeletedAt.IsNull(),
			)
			// search: no Name column on webhooks — match the endpoint URL.
			if req.Search != nil && req.GetSearch() != "" {
				q = q.Where(models.Webhooks.Url.ILike("%" + req.GetSearch() + "%"))
			}
			// enabled: tri-state (*bool) — filter only when set.
			if req.Enabled != nil {
				q = q.Where(models.Webhooks.Enabled.Eq(req.GetEnabled()))
			}
			// events: Events is a TEXT[] of enum String() values — match webhooks
			// subscribed to any of the requested events (array overlap, &&).
			if evs := req.GetEvents(); len(evs) > 0 {
				vals := make([]string, len(evs))
				for i, e := range evs {
					vals[i] = e.String()
				}
				q = q.Where(models.Webhooks.Events.ARRAYOverlapRaw("?", vals))
			}
			// tags: Tags is serialized JSON of common.Tags (a TEXT column) — match
			// each requested free tag and key=value label as a substring.
			for _, tag := range req.GetTags().GetTags() {
				q = q.Where(models.Webhooks.Tags.ILike("%" + tag + "%"))
			}
			for k, v := range req.GetTags().GetLabels() {
				q = q.Where(models.Webhooks.Tags.ILike("%" + k + "%" + v + "%"))
			}
			if tok := req.GetPage().GetToken(); tok != "" {
				if desc {
					q = q.Where(models.Webhooks.Id.Lt(tok))
				} else {
					q = q.Where(models.Webhooks.Id.Gt(tok))
				}
			}
			if desc {
				q = q.OrderByDESC(models.WebhookColumnId)
			} else {
				q = q.OrderByASC(models.WebhookColumnId)
			}
			rows, err := s.hooks.Query(ctx, q.Limit(size+1))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list webhooks: %v", err)
			}
			items, pageInfo := svcutil.Paginate(rows, size, func(w *models.Webhook) string {
				return w.GetEntity().GetId().GetValue()
			})
			return &uipb.ListWebhooksResponse{Webhooks: items, PageInfo: pageInfo}, nil
		})
}

// webhookUpdatableColumns maps proto field paths (UpdateWebhookRequest.webhook)
// to the mutable webhook columns a mask may select. id/created_at and the
// tenancy/owner invariants are never writable here; secret is rotated only via
// the request's dedicated secret field (write-only, never on the proto).
var webhookUpdatableColumns = map[string]models.WebhookColumnAlias{
	"url":     models.WebhookColumnUrl,
	"events":  models.WebhookColumnEvents,
	"enabled": models.WebhookColumnEnabled,
	"tags":    models.WebhookColumnTags,
}

// webhookMutableColumns is the back-compat full-replace set written when the
// update_mask is empty/nil.
var webhookMutableColumns = []models.WebhookColumnAlias{
	models.WebhookColumnUrl,
	models.WebhookColumnEvents,
	models.WebhookColumnEnabled,
	models.WebhookColumnTags,
}

// UpdateWebhook updates a webhook's mutable fields (id + created_at preserved).
// When the request's update_mask names paths, only those columns are written;
// an empty/nil mask is full-replace of the mutable fields (back-compat). The
// optional secret rotates the signing secret regardless of the mask (empty
// leaves it unchanged). UpdateWebhookRequest has no update_mask field on the
// wire today, so the mask is always empty and this is full-replace; the mask
// plumbing is in place for when the request gains one. updated_at is always bumped.
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
			setters := maskedWebhookSetters(scanner, nil)
			setters = append(setters, models.Webhooks.UpdatedAt.Set(scanner.UpdatedAt))
			if sec := req.GetSecret(); sec != "" {
				scanner.Secret = sec
				setters = append(setters, scanner.GetSetter(models.WebhookColumnSecret)())
			}
			updated, err := s.hooks.QueryRow(ctx,
				models.Webhooks.Update().Set(setters...).
					Where(models.Webhooks.Id.Eq(id)).ReturningAll())
			if err != nil {
				return nil, status.Errorf(codes.Internal, "update webhook: %v", err)
			}
			return updated, nil
		})
}

// maskedWebhookSetters builds the column setters for a partial webhook UPDATE
// honoring a proto FieldMask path list. paths from the mask select the columns;
// an empty/nil mask falls back to the full mutable set (back-compat). Unknown
// paths are ignored; updated_at and secret are handled by the caller.
func maskedWebhookSetters(scanner *models.WebhookScanner, paths []string) []set.ValueSetter[models.WebhookColumnAlias] {
	cols := webhookMutableColumns
	if len(paths) > 0 {
		cols = nil
		seen := make(map[models.WebhookColumnAlias]struct{}, len(paths))
		for _, p := range paths {
			col, ok := webhookUpdatableColumns[p]
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
	setters := make([]set.ValueSetter[models.WebhookColumnAlias], 0, len(cols))
	for _, col := range cols {
		setters = append(setters, scanner.GetSetter(col)())
	}
	return setters
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

// DeliverRunEvent fires every enabled webhook of the tenant subscribed to event
// with the given run/suite payload. Best-effort: a delivery failure is logged but
// does not abort the others (this is called off the run lifecycle, not a request).
func (s *WebhookService) DeliverRunEvent(ctx context.Context, tenantID string, event models.Webhook_Event, entityID, name string) error {
	hooks, err := s.hooks.Query(ctx, models.Webhooks.SelectAll().Where(
		models.Webhooks.TenantId.Eq(tenantID),
		models.Webhooks.DeletedAt.IsNull(),
	))
	if err != nil {
		return fmt.Errorf("list webhooks: %w", err)
	}
	payload := []byte(fmt.Sprintf(`{"event":%q,"id":%q,"name":%q}`, eventName(event), entityID, name))
	delivered := 0
	for _, hook := range hooks {
		if !hook.GetEnabled() || !subscribed(hook, event) {
			continue
		}
		secret, serr := s.hooks.Scanner().QueryRow(ctx,
			models.Webhooks.Select(models.WebhookColumnSecret).Where(models.Webhooks.Id.Eq(hook.GetEntity().GetId().GetValue())))
		sec := ""
		if serr == nil {
			sec = secret.Secret
		}
		if derr := s.sender.Send(ctx, hook.GetUrl(), sec, payload); derr != nil {
			s.Logger().Warn("webhook delivery failed", xlog.String("url", hook.GetUrl()), xlog.Error("error", derr))
			continue
		}
		delivered++
	}
	s.Logger().Debug("delivered run event", xlog.String("event", eventName(event)), xlog.Int("hooks", delivered))
	return nil
}

func subscribed(hook *models.Webhook, event models.Webhook_Event) bool {
	for _, e := range hook.GetEvents() {
		if e == event {
			return true
		}
	}
	return false
}

func eventName(e models.Webhook_Event) string {
	switch e {
	case models.Webhook_EVENT_RUN_COMPLETED:
		return "run.completed"
	case models.Webhook_EVENT_RUN_FAILED:
		return "run.failed"
	case models.Webhook_EVENT_SUITE_COMPLETED:
		return "suite.completed"
	case models.Webhook_EVENT_SUITE_FAILED:
		return "suite.failed"
	default:
		return "unspecified"
	}
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
