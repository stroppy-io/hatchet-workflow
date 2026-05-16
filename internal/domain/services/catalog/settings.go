package catalog

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// ListSettings returns all settings items for the given tenant.
func (s *Service) ListSettings(ctx context.Context, tenantID *iampb.TenantId) ([]*catalogpb.SettingsItem, error) {
	return s.settingsRepo.Query(ctx,
		catalogpb.SettingsItems.SelectAll().Where(
			catalogpb.SettingsItems.TenantId.Eq(tenantID.GetValue()),
		),
	)
}

// SetSetting upserts a settings item by (tenant_id, part, key).
// If a row already exists it updates value+updated_at and returns the existing ID.
// If no row exists it inserts a new row with a fresh ULID.
func (s *Service) SetSetting(
	ctx context.Context,
	tenantID *iampb.TenantId,
	part catalogpb.SettingsItem_Part,
	key catalogpb.SettingsItem_Key,
	value *catalogpb.SettingsItem_Value,
) (*catalogpb.SettingsItem, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "SetSetting",
		func(ctx context.Context, _ trace.Span) (*catalogpb.SettingsItem, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*catalogpb.SettingsItem, error) {
					// Try to find existing row.
					existing, err := s.settingsRepo.QueryRow(ctx,
						catalogpb.SettingsItems.SelectAll().Where(
							catalogpb.SettingsItems.TenantId.Eq(tenantID.GetValue()),
							catalogpb.SettingsItems.Part.Eq(part.String()),
							catalogpb.SettingsItems.Key.Eq(key.String()),
						),
					)
					if err != nil && !errors.Is(err, pgx.ErrNoRows) {
						return nil, err
					}

					now := time.Now()

					if existing != nil {
						// UPDATE: value + updated_at only.
						item := existing
						item.Value = value
						item.Timestamps.UpdatedAt = timestamppb.New(now)
						scanner := item.IntoPlain()
						if _, err := s.settingsRepo.Execute(ctx,
							catalogpb.SettingsItems.Update().
								Set(
									scanner.GetSetter(catalogpb.SettingsItemColumnValue)(),
									scanner.GetSetter(catalogpb.SettingsItemColumnUpdatedAt)(),
								).
								Where(
									catalogpb.SettingsItems.TenantId.Eq(tenantID.GetValue()),
									catalogpb.SettingsItems.Part.Eq(part.String()),
									catalogpb.SettingsItems.Key.Eq(key.String()),
								),
						); err != nil {
							return nil, err
						}
						return item, nil
					}

					// INSERT: new row with fresh ULID.
					item := &catalogpb.SettingsItem{
						Id:       &catalogpb.SettingsItemId{Value: ids.New()},
						TenantId: tenantID,
						Part:     part,
						Key:      key,
						Value:    value,
						Timestamps: &commonpb.Timestamps{
							CreatedAt: timestamppb.New(now),
							UpdatedAt: timestamppb.New(now),
						},
					}
					scanner := item.IntoPlain()
					if _, err := s.settingsRepo.Execute(ctx,
						catalogpb.SettingsItems.Insert().From(scanner.AllSetters()...),
					); err != nil {
						return nil, err
					}
					return item, nil
				},
			)
		},
	)
}

// GetSetting retrieves a settings item by (tenant_id, part, key).
// Returns domainerr.NotFound if no row matches.
func (s *Service) GetSetting(
	ctx context.Context,
	tenantID *iampb.TenantId,
	part catalogpb.SettingsItem_Part,
	key catalogpb.SettingsItem_Key,
) (*catalogpb.SettingsItem, error) {
	item, err := s.settingsRepo.QueryRow(ctx,
		catalogpb.SettingsItems.SelectAll().Where(
			catalogpb.SettingsItems.TenantId.Eq(tenantID.GetValue()),
			catalogpb.SettingsItems.Part.Eq(part.String()),
			catalogpb.SettingsItems.Key.Eq(key.String()),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("settings_item", part.String()+"/"+key.String()))
		}
		return nil, err
	}
	return item, nil
}
