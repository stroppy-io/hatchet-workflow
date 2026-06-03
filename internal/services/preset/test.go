package preset

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// TestPresetRepo persists TestPresetRecord rows (a reusable database + workload
// combo). Same tenant-scoped contract as the other preset repos: tenant_id pins
// ownership, Get takes the caller account for the computed is_favorite flag,
// List receives the whole request, and persistence methods return
// derrors.ErrNotFound / derrors.ErrConflict.
type TestPresetRepo interface {
	Create(ctx context.Context, preset *models.TestPresetRecord) error
	Get(ctx context.Context, tenantID, id, callerAccountID string) (*models.TestPresetRecord, error)
	List(ctx context.Context, req *api.ListTestPresetsRequest, callerAccountID string) (presets []*models.TestPresetRecord, nextPageToken string, err error)
	Update(ctx context.Context, preset *models.TestPresetRecord) error
}

func (s *TestPresetService) CreateTestPreset(ctx context.Context, req *api.CreateTestPresetRequest) (*api.CreateTestPresetResponse, error) {
	c, err := s.d.caller(ctx)
	if err != nil {
		return nil, err
	}
	preset := req.GetPreset()
	if preset == nil {
		return nil, status.Error(codes.InvalidArgument, "preset is required")
	}
	preset.Entity = s.d.stampNew(preset.GetEntity(), req.GetTenantId(), c.GetAccountId())
	preset.IsSystem = false
	fillTestPresetSummary(preset)
	if err := s.d.Tests.Create(ctx, preset); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.CreateTestPresetResponse{Preset: preset}, nil
}

func (s *TestPresetService) GetTestPreset(ctx context.Context, req *api.GetTestPresetRequest) (*api.GetTestPresetResponse, error) {
	c, err := s.d.caller(ctx)
	if err != nil {
		return nil, err
	}
	preset, err := s.d.Tests.Get(ctx, req.GetTenantId(), req.GetId(), c.GetAccountId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.GetTestPresetResponse{Preset: preset}, nil
}

func (s *TestPresetService) ListTestPresets(ctx context.Context, req *api.ListTestPresetsRequest) (*api.ListTestPresetsResponse, error) {
	c, err := s.d.caller(ctx)
	if err != nil {
		return nil, err
	}
	presets, next, err := s.d.Tests.List(ctx, req, c.GetAccountId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListTestPresetsResponse{Presets: presets, NextPageToken: next}, nil
}

func (s *TestPresetService) UpdateTestPreset(ctx context.Context, req *api.UpdateTestPresetRequest) (*api.UpdateTestPresetResponse, error) {
	c, err := s.d.caller(ctx)
	if err != nil {
		return nil, err
	}
	in := req.GetPreset()
	if in == nil || in.GetEntity().GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "preset.entity.id is required")
	}
	updated, err := doTxRet(ctx, s.d, func(ctx context.Context) (*models.TestPresetRecord, error) {
		existing, err := s.d.Tests.Get(ctx, req.GetTenantId(), in.GetEntity().GetId(), c.GetAccountId())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		if existing.GetIsSystem() {
			return nil, status.Error(codes.FailedPrecondition, "system presets are read-only; clone instead")
		}
		out := &models.TestPresetRecord{
			Entity:   preserveEntity(existing.GetEntity(), in.GetEntity(), s.d),
			Test:     in.GetTest(),
			IsSystem: false,
		}
		fillTestPresetSummary(out)
		if err := s.d.Tests.Update(ctx, out); err != nil {
			return nil, utils.MapErr(err)
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.UpdateTestPresetResponse{Preset: updated}, nil
}

func (s *TestPresetService) DeleteTestPreset(ctx context.Context, req *api.DeleteTestPresetRequest) (*api.DeleteTestPresetResponse, error) {
	c, err := s.d.caller(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.doTx(ctx, func(ctx context.Context) error {
		existing, err := s.d.Tests.Get(ctx, req.GetTenantId(), req.GetId(), c.GetAccountId())
		if err != nil {
			return ignoreNotFound(err)
		}
		if existing.GetIsSystem() {
			return status.Error(codes.FailedPrecondition, "system presets cannot be deleted")
		}
		if existing.GetEntity().GetTimings().GetDeletedAt() != nil {
			return nil
		}
		markEntityDeleted(existing.GetEntity(), s.d.now())
		if err := s.d.Tests.Update(ctx, existing); err != nil {
			return utils.MapErr(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &api.DeleteTestPresetResponse{}, nil
}

func (s *TestPresetService) CloneTestPreset(ctx context.Context, req *api.CloneTestPresetRequest) (*api.CloneTestPresetResponse, error) {
	c, err := s.d.caller(ctx)
	if err != nil {
		return nil, err
	}
	clone, err := doTxRet(ctx, s.d, func(ctx context.Context) (*models.TestPresetRecord, error) {
		src, err := s.d.Tests.Get(ctx, req.GetTenantId(), req.GetId(), c.GetAccountId())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		ent := s.d.stampNew(nil, req.GetTenantId(), c.GetAccountId())
		ent.Name = cloneName(req.GetName(), src.GetEntity().GetName())
		ent.Description = src.GetEntity().GetDescription()
		out := &models.TestPresetRecord{
			Entity:   ent,
			Test:     src.GetTest(),
			IsSystem: false,
		}
		fillTestPresetSummary(out)
		if err := s.d.Tests.Create(ctx, out); err != nil {
			return nil, utils.MapErr(err)
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.CloneTestPresetResponse{Preset: clone}, nil
}
