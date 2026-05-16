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

func (s *Service) CreateWorkloadPreset(ctx context.Context, tenantID *iampb.TenantId, createdBy *iampb.UserId, preset *catalogpb.WorkloadPreset) (*catalogpb.WorkloadPreset, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CreateWorkloadPreset",
		func(ctx context.Context, _ trace.Span) (*catalogpb.WorkloadPreset, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*catalogpb.WorkloadPreset, error) {
					now := timestamppb.Now()
					preset.Id = &catalogpb.WorkloadPresetId{Value: ids.New()}
					preset.TenantId = tenantID
					preset.CreatedBy = createdBy
					preset.Timestamps = &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now}
					scanner := preset.IntoPlain()
					if scanner.Label == nil {
						scanner.Label = []string{}
					}
					if _, err := s.workloadPresetRepo.Execute(ctx,
						catalogpb.WorkloadPresets.Insert().From(scanner.AllSetters()...),
					); err != nil {
						return nil, err
					}
					return preset, nil
				})
		})
}

func (s *Service) GetWorkloadPreset(ctx context.Context, id *catalogpb.WorkloadPresetId) (*catalogpb.WorkloadPreset, error) {
	p, err := s.workloadPresetRepo.QueryRow(ctx,
		catalogpb.WorkloadPresets.SelectAll().Where(
			catalogpb.WorkloadPresets.Id.Eq(id.GetValue()),
			catalogpb.WorkloadPresets.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("workload_preset", id.GetValue()))
		}
		return nil, err
	}
	return p, nil
}

func (s *Service) ListWorkloadPresets(ctx context.Context, tenantID *iampb.TenantId) ([]*catalogpb.WorkloadPreset, error) {
	return s.workloadPresetRepo.Query(ctx,
		catalogpb.WorkloadPresets.SelectAll().Where(
			catalogpb.WorkloadPresets.TenantId.Eq(tenantID.GetValue()),
			catalogpb.WorkloadPresets.DeletedAt.IsNull(),
		),
	)
}

func (s *Service) DeleteWorkloadPreset(ctx context.Context, id *catalogpb.WorkloadPresetId) error {
	now := time.Now()
	n, err := s.workloadPresetRepo.Execute(ctx,
		catalogpb.WorkloadPresets.Update().
			Set(catalogpb.WorkloadPresets.DeletedAt.Set(&now)).
			Where(
				catalogpb.WorkloadPresets.Id.Eq(id.GetValue()),
				catalogpb.WorkloadPresets.DeletedAt.IsNull(),
			),
	)
	if err != nil {
		return err
	}
	if n == 0 {
		return domainerr.NotFound(domainerr.ResourceInfo("workload_preset", id.GetValue()))
	}
	return nil
}

func (s *Service) CloneWorkloadPreset(ctx context.Context, id *catalogpb.WorkloadPresetId, callerID *iampb.UserId) (*catalogpb.WorkloadPreset, error) {
	original, err := s.GetWorkloadPreset(ctx, id)
	if err != nil {
		return nil, err
	}
	cloned := &catalogpb.WorkloadPreset{
		TenantId: original.GetTenantId(),
		Identity: original.GetIdentity(),
		Workload: original.GetWorkload(),
	}
	return s.CreateWorkloadPreset(ctx, original.GetTenantId(), callerID, cloned)
}

// UpdateWorkloadPreset patches the mutable fields of an existing workload preset.
func (s *Service) UpdateWorkloadPreset(ctx context.Context, preset *catalogpb.WorkloadPreset) (*catalogpb.WorkloadPreset, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "UpdateWorkloadPreset",
		func(ctx context.Context, _ trace.Span) (*catalogpb.WorkloadPreset, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*catalogpb.WorkloadPreset, error) {
					existing, err := s.GetWorkloadPreset(ctx, preset.GetId())
					if err != nil {
						return nil, err
					}
					if preset.GetIdentity() != nil {
						existing.Identity = preset.GetIdentity()
					}
					if preset.GetWorkload() != nil {
						existing.Workload = preset.GetWorkload()
					}
					existing.Timestamps.UpdatedAt = timestamppb.Now()
					scanner := existing.IntoPlain()
					if _, err := s.workloadPresetRepo.Execute(ctx,
						catalogpb.WorkloadPresets.Update().
							Set(
								scanner.GetSetter(catalogpb.WorkloadPresetColumnName)(),
								scanner.GetSetter(catalogpb.WorkloadPresetColumnDescription)(),
								scanner.GetSetter(catalogpb.WorkloadPresetColumnLabel)(),
								scanner.GetSetter(catalogpb.WorkloadPresetColumnWorkload)(),
								scanner.GetSetter(catalogpb.WorkloadPresetColumnUpdatedAt)(),
							).
							Where(
								catalogpb.WorkloadPresets.Id.Eq(existing.GetId().GetValue()),
							),
					); err != nil {
						return nil, err
					}
					return existing, nil
				})
		})
}

// DeleteWorkloadPresetAndReturn soft-deletes a workload preset and returns the pre-delete record.
func (s *Service) DeleteWorkloadPresetAndReturn(ctx context.Context, id *catalogpb.WorkloadPresetId) (*catalogpb.WorkloadPreset, error) {
	existing, err := s.GetWorkloadPreset(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.DeleteWorkloadPreset(ctx, id); err != nil {
		return nil, err
	}
	return existing, nil
}
