package preset

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// DatabasePresetRepo persists DatabasePresetRecord rows. Every method is
// tenant-scoped: the tenant_id pins the row's owning tenant and rows never cross
// tenants. Get takes the caller account so the persistence layer can fill the
// computed per-caller is_favorite flag. List receives the whole request (its
// tenant, entity filter, tag map, kind/source filters, system flag, sort and
// page) and returns the matching rows plus the next page token. Get returns
// derrors.ErrNotFound for an absent or other-tenant row; Create returns
// derrors.ErrConflict on a duplicate id.
type DatabasePresetRepo interface {
	Create(ctx context.Context, preset *models.DatabasePresetRecord) error
	Get(ctx context.Context, tenantID, id, callerAccountID string) (*models.DatabasePresetRecord, error)
	List(ctx context.Context, req *api.ListDatabasePresetsRequest, callerAccountID string) (presets []*models.DatabasePresetRecord, nextPageToken string, err error)
	Update(ctx context.Context, preset *models.DatabasePresetRecord) error
	Delete(ctx context.Context, tenantID, id string) error
}

func (s *DatabasePresetService) CreateDatabasePreset(ctx context.Context, req *api.CreateDatabasePresetRequest) (*api.CreateDatabasePresetResponse, error) {
	c, err := s.d.caller(ctx)
	if err != nil {
		return nil, err
	}
	preset := req.GetPreset()
	if preset == nil {
		return nil, status.Error(codes.InvalidArgument, "preset is required")
	}
	// Server owns id/tenant_id/author_id/timings and is_system; the client cannot
	// mint a system preset.
	preset.Entity = s.d.stampNew(preset.GetEntity(), req.GetTenantId(), c.GetAccountId())
	preset.IsSystem = false
	if err := s.d.Databases.Create(ctx, preset); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.CreateDatabasePresetResponse{Preset: preset}, nil
}

func (s *DatabasePresetService) GetDatabasePreset(ctx context.Context, req *api.GetDatabasePresetRequest) (*api.GetDatabasePresetResponse, error) {
	c, err := s.d.caller(ctx)
	if err != nil {
		return nil, err
	}
	preset, err := s.d.Databases.Get(ctx, req.GetTenantId(), req.GetId(), c.GetAccountId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.GetDatabasePresetResponse{Preset: preset}, nil
}

func (s *DatabasePresetService) ListDatabasePresets(ctx context.Context, req *api.ListDatabasePresetsRequest) (*api.ListDatabasePresetsResponse, error) {
	c, err := s.d.caller(ctx)
	if err != nil {
		return nil, err
	}
	presets, next, err := s.d.Databases.List(ctx, req, c.GetAccountId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListDatabasePresetsResponse{Presets: presets, NextPageToken: next}, nil
}

func (s *DatabasePresetService) UpdateDatabasePreset(ctx context.Context, req *api.UpdateDatabasePresetRequest) (*api.UpdateDatabasePresetResponse, error) {
	c, err := s.d.caller(ctx)
	if err != nil {
		return nil, err
	}
	in := req.GetPreset()
	if in == nil || in.GetEntity().GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "preset.entity.id is required")
	}
	updated, err := doTxRet(ctx, s.d, func(ctx context.Context) (*models.DatabasePresetRecord, error) {
		// Verify the target row exists and belongs to the tenant before writing.
		existing, err := s.d.Databases.Get(ctx, req.GetTenantId(), in.GetEntity().GetId(), c.GetAccountId())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		if existing.GetIsSystem() {
			return nil, status.Error(codes.FailedPrecondition, "system presets are read-only; clone instead")
		}
		// Wholesale replace of the editable payload; server-owned envelope fields are
		// preserved so a converging retry stays idempotent.
		out := &models.DatabasePresetRecord{
			Entity:   preserveEntity(existing.GetEntity(), in.GetEntity(), s.d),
			Database: in.GetDatabase(),
			IsSystem: false,
		}
		if err := s.d.Databases.Update(ctx, out); err != nil {
			return nil, utils.MapErr(err)
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.UpdateDatabasePresetResponse{Preset: updated}, nil
}

func (s *DatabasePresetService) DeleteDatabasePreset(ctx context.Context, req *api.DeleteDatabasePresetRequest) (*api.DeleteDatabasePresetResponse, error) {
	c, err := s.d.caller(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.doTx(ctx, func(ctx context.Context) error {
		existing, err := s.d.Databases.Get(ctx, req.GetTenantId(), req.GetId(), c.GetAccountId())
		if err != nil {
			// Deleting an absent preset is a no-op (idempotent).
			return ignoreNotFound(err)
		}
		if existing.GetIsSystem() {
			return status.Error(codes.FailedPrecondition, "system presets cannot be deleted")
		}
		return ignoreNotFound(s.d.Databases.Delete(ctx, req.GetTenantId(), req.GetId()))
	}); err != nil {
		return nil, err
	}
	return &api.DeleteDatabasePresetResponse{}, nil
}

func (s *DatabasePresetService) CloneDatabasePreset(ctx context.Context, req *api.CloneDatabasePresetRequest) (*api.CloneDatabasePresetResponse, error) {
	c, err := s.d.caller(ctx)
	if err != nil {
		return nil, err
	}
	clone, err := doTxRet(ctx, s.d, func(ctx context.Context) (*models.DatabasePresetRecord, error) {
		src, err := s.d.Databases.Get(ctx, req.GetTenantId(), req.GetId(), c.GetAccountId())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		ent := s.d.stampNew(nil, req.GetTenantId(), c.GetAccountId())
		ent.Name = cloneName(req.GetName(), src.GetEntity().GetName())
		ent.Description = src.GetEntity().GetDescription()
		out := &models.DatabasePresetRecord{
			Entity:   ent,
			Database: src.GetDatabase(),
			IsSystem: false,
		}
		if err := s.d.Databases.Create(ctx, out); err != nil {
			return nil, utils.MapErr(err)
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.CloneDatabasePresetResponse{Preset: clone}, nil
}
