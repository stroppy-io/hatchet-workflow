// Package settings implements the tenant-scoped SettingsService (provider creds
// etc). RBAC: read = ADMIN (creds should be masked), write = OWNER (auth.proto /
// rbac.feature). Items are unique per (tenant_id, part, key).
package settings

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
	"google.golang.org/protobuf/types/known/emptypb"

	uiapi "github.com/stroppy-io/stroppy-cloud/internal/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/authz"
	"github.com/stroppy-io/stroppy-cloud/internal/services/svcutil"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// SettingsService implements ui.SettingsActions.
type SettingsService struct {
	*tracing.Entity
	items *repository.ProtoRepository[
		models.SettingsItemAlias,
		models.SettingsItemColumnAlias,
		*models.SettingsItemScanner,
		*models.SettingsItem,
	]
	authz *authz.Authz
	txm   tx.Trm
}

var _ uiapi.SettingsActions = (*SettingsService)(nil)

// NewSettingsService builds the service.
func NewSettingsService(logger *xlog.Logger, executor exec.DB, txm tx.Trm, az *authz.Authz) *SettingsService {
	return &SettingsService{
		Entity: tracing.NewEntity(logger.AppendName("SettingsService")),
		items: repository.NewProtoRepository(
			repository.NewScannerRepository(models.SettingsItems.Table, executor),
			models.SettingsItemConverter,
		),
		authz: az,
		txm:   txm,
	}
}

// ListSettingsItems returns the tenant's settings.
//
// TODO(settings): secret values are NOT masked yet — read is ADMIN and creds
// should be redacted (which keys are secret = backend rule). Reported.
func (s *SettingsService) ListSettingsItems(ctx context.Context, req *uipb.ListSettingsItemsRequest) (*models.SettingsItem_List, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListSettingsItems",
		func(ctx context.Context, _ trace.Span) (*models.SettingsItem_List, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			items, err := s.tenantItems(ctx, req.GetTenantId().GetValue())
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list settings: %v", err)
			}
			return &models.SettingsItem_List{SettingsItems: items}, nil
		})
}

// GetSettingsItem returns a single settings item by id.
func (s *SettingsService) GetSettingsItem(ctx context.Context, req *uipb.GetSettingsItemRequest) (*models.SettingsItem, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "GetSettingsItem",
		func(ctx context.Context, _ trace.Span) (*models.SettingsItem, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			item, err := s.items.QueryRow(ctx,
				models.SettingsItems.SelectAll().Where(
					models.SettingsItems.Id.Eq(req.GetSettingsItemId().GetValue()),
					models.SettingsItems.TenantId.Eq(req.GetTenantId().GetValue()),
					models.SettingsItems.DeletedAt.IsNull(),
				))
			if err != nil {
				return nil, svcutil.NotFound(err, "settings item")
			}
			return item, nil
		})
}

// SetSettingsItem upserts by (tenant_id, part, key).
func (s *SettingsService) SetSettingsItem(ctx context.Context, req *uipb.SetSettingsItemRequest) (*models.SettingsItem, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "SetSettingsItem",
		func(ctx context.Context, _ trace.Span) (*models.SettingsItem, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_OWNER); err != nil {
				return nil, err
			}
			return tx.DoReadCommittedRet(ctx, s.txm, func(ctx context.Context) (*models.SettingsItem, error) {
				existing, err := s.findByPartKey(ctx, req.GetTenantId().GetValue(), req.GetPart(), req.GetKey())
				if err != nil {
					return nil, status.Errorf(codes.Internal, "lookup settings item: %v", err)
				}
				item := &models.SettingsItem{
					TenantId: req.GetTenantId(),
					Part:     req.GetPart(),
					Key:      req.GetKey(),
					Value:    req.GetValue(),
				}
				if existing != nil {
					item.Id = existing.GetId()
					item.Timestamps = existing.GetTimestamps()
				} else {
					e := ids.NewEntity()
					item.Id = &models.SettingsItemId{Value: e.GetId().GetValue()}
					item.Timestamps = e.GetTimestamps()
				}
				scanner := item.IntoPlain()
				scanner.UpdatedAt = time.Now()
				updated, err := s.items.QueryRow(ctx,
					models.SettingsItems.Insert().From(scanner.AllSetters()...).
						OnConflict(models.SettingsItemColumnId).
						DoUpdate(models.SettingsItemColumnValue, models.SettingsItemColumnUpdatedAt).
						ReturningAll())
				if err != nil {
					return nil, status.Errorf(codes.Internal, "upsert settings item: %v", err)
				}
				return updated, nil
			})
		})
}

// DeleteSettingsItem soft-deletes a settings item.
func (s *SettingsService) DeleteSettingsItem(ctx context.Context, req *uipb.DeleteSettingsItemRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "DeleteSettingsItem",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_OWNER); err != nil {
				return nil, err
			}
			now := time.Now()
			if _, err := s.items.Execute(ctx,
				models.SettingsItems.Update().Set(
					models.SettingsItems.DeletedAt.Set(&now),
					models.SettingsItems.UpdatedAt.Set(now),
				).Where(
					models.SettingsItems.Id.Eq(req.GetSettingsItemId().GetValue()),
					models.SettingsItems.TenantId.Eq(req.GetTenantId().GetValue()),
				)); err != nil {
				return nil, status.Errorf(codes.Internal, "delete settings item: %v", err)
			}
			return &emptypb.Empty{}, nil
		})
}

func (s *SettingsService) tenantItems(ctx context.Context, tenantID string) ([]*models.SettingsItem, error) {
	return s.items.Query(ctx,
		models.SettingsItems.SelectAll().Where(
			models.SettingsItems.TenantId.Eq(tenantID),
			models.SettingsItems.DeletedAt.IsNull(),
		))
}

// findByPartKey resolves the unique (tenant, part, key) row, filtering enums in
// Go (part/key are text columns). Returns nil when absent.
func (s *SettingsService) findByPartKey(ctx context.Context, tenantID string, part models.SettingsItem_Part, key models.SettingsItem_Key) (*models.SettingsItem, error) {
	items, err := s.tenantItems(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		if it.GetPart() == part && it.GetKey() == key {
			return it, nil
		}
	}
	return nil, nil
}
