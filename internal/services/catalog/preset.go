// Package catalog implements the tenant-scoped Preset catalog (ui PresetService).
// RBAC: list/read = VIEWER, create/update/delete/clone = ADMIN (rbac.feature).
package catalog

import (
	"context"
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

// ListPresets returns the tenant's presets, optionally filtered by kind(s),
// with cursor pagination (newest-first by default).
func (s *PresetService) ListPresets(ctx context.Context, req *uipb.ListPresetRequest) (*uipb.ListPresetsResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListPresets",
		func(ctx context.Context, _ trace.Span) (*uipb.ListPresetsResponse, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			size := svcutil.PageSize(req.GetPage())
			desc := svcutil.CursorDesc(req.GetOrder())
			// The tenant's own presets PLUS the platform's system presets (is_system),
			// a shared read-only catalog visible to every tenant.
			q := models.Presets.SelectAll().Where(
				models.Presets.Or(
					models.Presets.TenantId.Eq(req.GetTenantId().GetValue()),
					models.Presets.IsSystem.Eq(true),
				),
				models.Presets.DeletedAt.IsNull(),
			)
			if kinds := req.GetKinds(); len(kinds) > 0 {
				// Kind is stored as the enum String() value (preset_plain converter).
				ks := make([]string, 0, len(kinds))
				for _, k := range kinds {
					ks = append(ks, k.String())
				}
				q = q.Where(models.Presets.Kind.In(ks...))
			}
			// search: no Name column on presets — match the case identifier
			// (the human-visible label) case-insensitively.
			if req.GetSearch() != "" {
				q = q.Where(models.Presets.PresetCase.ILike("%" + req.GetSearch() + "%"))
			}
			// tags: Tags is serialized JSON of common.Tags (a TEXT column) — match
			// each requested free tag and key=value label as a substring.
			for _, tag := range req.GetTags().GetTags() {
				q = q.Where(models.Presets.Tags.ILike("%" + tag + "%"))
			}
			for k, v := range req.GetTags().GetLabels() {
				q = q.Where(models.Presets.Tags.ILike("%" + k + "%" + v + "%"))
			}
			if tok := req.GetPage().GetToken(); tok != "" {
				if desc {
					q = q.Where(models.Presets.Id.Lt(tok))
				} else {
					q = q.Where(models.Presets.Id.Gt(tok))
				}
			}
			if desc {
				q = q.OrderByDESC(models.PresetColumnId)
			} else {
				q = q.OrderByASC(models.PresetColumnId)
			}
			rows, err := s.presets.Query(ctx, q.Limit(size+1))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list presets: %v", err)
			}
			items, pageInfo := svcutil.Paginate(rows, size, func(p *models.Preset) string {
				return p.GetEntity().GetId().GetValue()
			})
			return &uipb.ListPresetsResponse{Presets: items, PageInfo: pageInfo}, nil
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

// presetUpdatableColumns maps proto field paths (models.Preset) to the mutable
// preset columns a mask may select. id/created_at and the tenancy/owner
// invariants are never writable here.
var presetUpdatableColumns = map[string]models.PresetColumnAlias{
	"tags":            models.PresetColumnTags,
	"kind":            models.PresetColumnKind,
	"preset_case":     models.PresetColumnPresetCase,
	"presetCase":      models.PresetColumnPresetCase,
	"workload_preset": models.PresetColumnPresetWorkloadPreset,
	"workloadPreset":  models.PresetColumnPresetWorkloadPreset,
	"database_preset": models.PresetColumnPresetDatabasePreset,
	"databasePreset":  models.PresetColumnPresetDatabasePreset,
	"test_preset":     models.PresetColumnPresetTestPreset,
	"testPreset":      models.PresetColumnPresetTestPreset,
}

// presetMutableColumns is the back-compat full-replace set written when the
// update_mask is empty/nil.
var presetMutableColumns = []models.PresetColumnAlias{
	models.PresetColumnTags,
	models.PresetColumnKind,
	models.PresetColumnPresetCase,
	models.PresetColumnPresetWorkloadPreset,
	models.PresetColumnPresetDatabasePreset,
	models.PresetColumnPresetTestPreset,
}

// UpdatePreset updates a preset's mutable fields (id + created_at preserved).
// When the request's update_mask names paths, only those columns are written;
// an empty/nil mask is full-replace of the mutable fields (back-compat). The
// PresetService RPC takes a bare models.Preset (no mask field on the wire), so
// today the mask is always empty and this is full-replace; the mask plumbing is
// in place for when the request gains an update_mask. updated_at is always bumped.
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
			if existing.GetIsSystem() {
				return nil, status.Error(codes.PermissionDenied, "system preset is read-only")
			}
			// Preserve identity + creation time; bump updated_at.
			preset.Entity.Id = existing.GetEntity().GetId()
			preset.Entity.Timestamps = existing.GetEntity().GetTimestamps()
			scanner := preset.IntoPlain()
			nilEmptyPresetJSONB(scanner)
			scanner.UpdatedAt = time.Now()
			setters := maskedPresetSetters(scanner, nil)
			setters = append(setters, models.Presets.UpdatedAt.Set(scanner.UpdatedAt))
			updated, err := s.presets.QueryRow(ctx,
				models.Presets.Update().Set(setters...).
					Where(models.Presets.Id.Eq(id)).ReturningAll())
			if err != nil {
				return nil, status.Errorf(codes.Internal, "update preset: %v", err)
			}
			return updated, nil
		})
}

// maskedPresetSetters builds the column setters for a partial preset UPDATE
// honoring a proto FieldMask path list. paths from the mask select the columns;
// an empty/nil mask falls back to the full mutable set (back-compat). Unknown
// paths are ignored; updated_at is appended by the caller.
func maskedPresetSetters(scanner *models.PresetScanner, paths []string) []set.ValueSetter[models.PresetColumnAlias] {
	cols := presetMutableColumns
	if len(paths) > 0 {
		cols = nil
		seen := make(map[models.PresetColumnAlias]struct{}, len(paths))
		for _, p := range paths {
			col, ok := presetUpdatableColumns[p]
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
	setters := make([]set.ValueSetter[models.PresetColumnAlias], 0, len(cols))
	for _, col := range cols {
		setters = append(setters, scanner.GetSetter(col)())
	}
	return setters
}

// DeletePreset soft-deletes a preset.
func (s *PresetService) DeletePreset(ctx context.Context, req *uipb.DeletePresetRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "DeletePreset",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			if err := s.authz.Require(ctx, svcutil.CallerOf(ctx), req.GetTenantId(), models.TenantMember_ROLE_ADMIN); err != nil {
				return nil, err
			}
			existing, err := s.presets.QueryRow(ctx, models.Presets.SelectAll().Where(
				models.Presets.Id.Eq(req.GetId().GetValue()),
				models.Presets.TenantId.Eq(req.GetTenantId().GetValue()),
				models.Presets.DeletedAt.IsNull(),
			))
			if err != nil {
				return nil, svcutil.NotFound(err, "preset")
			}
			if existing.GetIsSystem() {
				return nil, status.Error(codes.PermissionDenied, "system preset is read-only")
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
