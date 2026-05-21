// Package catalog implements the tenant-scoped Preset catalog (ui PresetService).
// RBAC: list/read = VIEWER, create/update/delete/clone = ADMIN (rbac.feature).
package catalog

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

// PresetService implements ui.PresetActions.
type PresetService struct {
	*tracing.Entity
	presets *repository.ProtoRepository[
		models.PresetAlias,
		models.PresetColumnAlias,
		*models.PresetScanner,
		*models.Preset,
	]
	authz *authz.Authz
	txm   tx.Trm
}

var _ uiapi.PresetActions = (*PresetService)(nil)

// NewPresetService builds the service.
func NewPresetService(logger *xlog.Logger, executor exec.DB, txm tx.Trm, az *authz.Authz) *PresetService {
	return &PresetService{
		Entity: tracing.NewEntity(logger.AppendName("PresetService")),
		presets: repository.NewProtoRepository(
			repository.NewScannerRepository(models.Presets.Table, executor),
			models.PresetConverter,
		),
		authz: az,
		txm:   txm,
	}
}

// ListPresets returns the tenant's presets of the requested kind.
//
// TODO(catalog): kind is filtered in Go (enum<->text column mapping not asserted);
// push to SQL WHERE once confirmed. Reported.
func (s *PresetService) ListPresets(ctx context.Context, req *uipb.ListPresetRequest) (*models.Preset_List, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListPresets",
		func(ctx context.Context, _ trace.Span) (*models.Preset_List, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			all, err := s.presets.Query(ctx,
				models.Presets.SelectAll().Where(
					models.Presets.TenantId.Eq(req.GetTenantId().GetValue()),
					models.Presets.DeletedAt.IsNull(),
				))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list presets: %v", err)
			}
			out := &models.Preset_List{Presets: make([]*models.Preset, 0, len(all))}
			for _, p := range all {
				if p.GetKind() == req.GetKind() {
					out.Presets = append(out.Presets, p)
				}
			}
			return out, nil
		})
}

// CreatePreset stores a new preset owned by the caller in the preset's tenant.
func (s *PresetService) CreatePreset(ctx context.Context, preset *models.Preset) (*models.Preset, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CreatePreset",
		func(ctx context.Context, _ trace.Span) (*models.Preset, error) {
			c := svcutil.CallerOf(ctx)
			tenantID := preset.GetOwned().GetTenantId()
			if tenantID.GetValue() == "" {
				return nil, status.Error(codes.InvalidArgument, "preset tenant_id required")
			}
			if err := s.authz.Require(ctx, c, tenantID, models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			preset.Entity = ids.NewEntity()
			preset.Owned = &models.Own{OwnerAccountId: c.AccountID, TenantId: tenantID}
			return tx.DoReadCommittedRet(ctx, s.txm, func(ctx context.Context) (*models.Preset, error) {
				scanner := preset.IntoPlain()
				nilEmptyPresetJSONB(scanner)
				if _, err := s.presets.Execute(ctx,
					models.Presets.Insert().From(scanner.AllSetters()...)); err != nil {
					return nil, status.Errorf(codes.Internal, "insert preset: %v", err)
				}
				return preset, nil
			})
		})
}

// UpdatePreset replaces a preset's mutable fields (id + created_at preserved).
//
// TODO(catalog): update_mask is not honored — full replace of mutable fields. Reported.
func (s *PresetService) UpdatePreset(ctx context.Context, preset *models.Preset) (*models.Preset, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "UpdatePreset",
		func(ctx context.Context, _ trace.Span) (*models.Preset, error) {
			c := svcutil.CallerOf(ctx)
			tenantID := preset.GetOwned().GetTenantId()
			if err := s.authz.Require(ctx, c, tenantID, models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			id := preset.GetEntity().GetId().GetValue()
			existing, err := s.presets.QueryRow(ctx,
				models.Presets.SelectAll().Where(
					models.Presets.Id.Eq(id),
					models.Presets.TenantId.Eq(tenantID.GetValue()),
					models.Presets.DeletedAt.IsNull(),
				))
			if err != nil {
				return nil, svcutil.NotFound(err, "preset")
			}
			// Preserve identity + creation time; bump updated_at.
			preset.Entity.Id = existing.GetEntity().GetId()
			preset.Entity.Timestamps = existing.GetEntity().GetTimestamps()
			scanner := preset.IntoPlain()
			nilEmptyPresetJSONB(scanner)
			scanner.UpdatedAt = time.Now()
			updated, err := s.presets.QueryRow(ctx,
				models.Presets.Update().Set(scanner.AllSetters()...).
					Where(models.Presets.Id.Eq(id)).ReturningAll())
			if err != nil {
				return nil, status.Errorf(codes.Internal, "update preset: %v", err)
			}
			return updated, nil
		})
}

// DeletePreset soft-deletes a preset.
func (s *PresetService) DeletePreset(ctx context.Context, req *uipb.DeletePresetRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "DeletePreset",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			now := time.Now()
			if _, err := s.presets.Execute(ctx,
				models.Presets.Update().Set(
					models.Presets.DeletedAt.Set(&now),
					models.Presets.UpdatedAt.Set(now),
				).Where(
					models.Presets.Id.Eq(req.GetId().GetValue()),
					models.Presets.TenantId.Eq(req.GetTenantId().GetValue()),
				)); err != nil {
				return nil, status.Errorf(codes.Internal, "delete preset: %v", err)
			}
			return &emptypb.Empty{}, nil
		})
}

// ClonePreset duplicates a preset under a new id.
func (s *PresetService) ClonePreset(ctx context.Context, req *uipb.ClonePresetRequest) (*models.Preset, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ClonePreset",
		func(ctx context.Context, _ trace.Span) (*models.Preset, error) {
			c := svcutil.CallerOf(ctx)
			if err := s.authz.Require(ctx, c, req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			src, err := s.presets.QueryRow(ctx,
				models.Presets.SelectAll().Where(
					models.Presets.Id.Eq(req.GetId().GetValue()),
					models.Presets.TenantId.Eq(req.GetTenantId().GetValue()),
					models.Presets.DeletedAt.IsNull(),
				))
			if err != nil {
				return nil, svcutil.NotFound(err, "preset")
			}
			src.Entity = ids.NewEntity()
			src.Owned = &models.Own{OwnerAccountId: c.AccountID, TenantId: req.GetTenantId()}
			return tx.DoReadCommittedRet(ctx, s.txm, func(ctx context.Context) (*models.Preset, error) {
				scanner := src.IntoPlain()
				nilEmptyPresetJSONB(scanner)
				if _, err := s.presets.Execute(ctx,
					models.Presets.Insert().From(scanner.AllSetters()...)); err != nil {
					return nil, status.Errorf(codes.Internal, "clone preset: %v", err)
				}
				return src, nil
			})
		})
}

// nilEmptyPresetJSONB normalizes the preset's nullable JSONB columns: the generated
// IntoPlain writes []byte{} (not nil) for absent oneof bodies, which Postgres
// rejects as invalid jsonb ("", 22P02). Bind nil so they store as NULL.
func nilEmptyPresetJSONB(s *models.PresetScanner) {
	if len(s.PresetWorkloadPreset) == 0 {
		s.PresetWorkloadPreset = nil
	}
	if len(s.PresetDatabasePreset) == 0 {
		s.PresetDatabasePreset = nil
	}
	if len(s.PresetTestPreset) == 0 {
		s.PresetTestPreset = nil
	}
}
