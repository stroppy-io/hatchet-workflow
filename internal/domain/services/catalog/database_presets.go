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

func (s *Service) CreateDatabasePreset(ctx context.Context, tenantID *iampb.TenantId, createdBy *iampb.UserId, preset *catalogpb.DatabasePreset) (*catalogpb.DatabasePreset, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CreateDatabasePreset",
		func(ctx context.Context, _ trace.Span) (*catalogpb.DatabasePreset, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*catalogpb.DatabasePreset, error) {
					now := timestamppb.Now()
					preset.Id = &catalogpb.DatabasePresetId{Value: ids.New()}
					preset.TenantId = tenantID
					preset.CreatedBy = createdBy
					preset.Timestamps = &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now}
					scanner := preset.IntoPlain()
					if scanner.Label == nil {
						scanner.Label = []string{}
					}
					if _, err := s.dbPresetRepo.Execute(ctx,
						catalogpb.DatabasePresets.Insert().From(scanner.AllSetters()...),
					); err != nil {
						return nil, err
					}
					return preset, nil
				})
		})
}

func (s *Service) GetDatabasePreset(ctx context.Context, id *catalogpb.DatabasePresetId) (*catalogpb.DatabasePreset, error) {
	p, err := s.dbPresetRepo.QueryRow(ctx,
		catalogpb.DatabasePresets.SelectAll().Where(
			catalogpb.DatabasePresets.Id.Eq(id.GetValue()),
			catalogpb.DatabasePresets.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("database_preset", id.GetValue()))
		}
		return nil, err
	}
	return p, nil
}

func (s *Service) ListDatabasePresets(ctx context.Context, tenantID *iampb.TenantId) ([]*catalogpb.DatabasePreset, error) {
	return s.dbPresetRepo.Query(ctx,
		catalogpb.DatabasePresets.SelectAll().Where(
			catalogpb.DatabasePresets.TenantId.Eq(tenantID.GetValue()),
			catalogpb.DatabasePresets.DeletedAt.IsNull(),
		),
	)
}

func (s *Service) DeleteDatabasePreset(ctx context.Context, id *catalogpb.DatabasePresetId) error {
	now := time.Now()
	n, err := s.dbPresetRepo.Execute(ctx,
		catalogpb.DatabasePresets.Update().
			Set(catalogpb.DatabasePresets.DeletedAt.Set(&now)).
			Where(
				catalogpb.DatabasePresets.Id.Eq(id.GetValue()),
				catalogpb.DatabasePresets.DeletedAt.IsNull(),
			),
	)
	if err != nil {
		return err
	}
	if n == 0 {
		return domainerr.NotFound(domainerr.ResourceInfo("database_preset", id.GetValue()))
	}
	return nil
}

func (s *Service) CloneDatabasePreset(ctx context.Context, id *catalogpb.DatabasePresetId, callerID *iampb.UserId) (*catalogpb.DatabasePreset, error) {
	original, err := s.GetDatabasePreset(ctx, id)
	if err != nil {
		return nil, err
	}
	cloned := &catalogpb.DatabasePreset{
		TenantId: original.GetTenantId(),
		Identity: original.GetIdentity(),
		Database: original.GetDatabase(),
	}
	return s.CreateDatabasePreset(ctx, original.GetTenantId(), callerID, cloned)
}
