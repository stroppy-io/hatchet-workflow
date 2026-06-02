package preset

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// WorkloadPresetRepo persists WorkloadPresetRecord rows. Same tenant-scoped
// contract as DatabasePresetRepo: tenant_id pins ownership, Get takes the caller
// account for the computed is_favorite flag, List receives the whole request,
// and persistence methods return derrors.ErrNotFound / derrors.ErrConflict.
type WorkloadPresetRepo interface {
	Create(ctx context.Context, preset *models.WorkloadPresetRecord) error
	Get(ctx context.Context, tenantID, id, callerAccountID string) (*models.WorkloadPresetRecord, error)
	List(ctx context.Context, req *api.ListWorkloadPresetsRequest, callerAccountID string) (presets []*models.WorkloadPresetRecord, nextPageToken string, err error)
	Update(ctx context.Context, preset *models.WorkloadPresetRecord) error
	Delete(ctx context.Context, tenantID, id string) error
}

func (s *WorkloadPresetService) CreateWorkloadPreset(ctx context.Context, req *api.CreateWorkloadPresetRequest) (*api.CreateWorkloadPresetResponse, error) {
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
	if err := s.d.Workloads.Create(ctx, preset); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.CreateWorkloadPresetResponse{Preset: preset}, nil
}

func (s *WorkloadPresetService) GetWorkloadPreset(ctx context.Context, req *api.GetWorkloadPresetRequest) (*api.GetWorkloadPresetResponse, error) {
	c, err := s.d.caller(ctx)
	if err != nil {
		return nil, err
	}
	preset, err := s.d.Workloads.Get(ctx, req.GetTenantId(), req.GetId(), c.GetAccountId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.GetWorkloadPresetResponse{Preset: preset}, nil
}

func (s *WorkloadPresetService) ListWorkloadPresets(ctx context.Context, req *api.ListWorkloadPresetsRequest) (*api.ListWorkloadPresetsResponse, error) {
	c, err := s.d.caller(ctx)
	if err != nil {
		return nil, err
	}
	presets, next, err := s.d.Workloads.List(ctx, req, c.GetAccountId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListWorkloadPresetsResponse{Presets: presets, NextPageToken: next}, nil
}

func (s *WorkloadPresetService) UpdateWorkloadPreset(ctx context.Context, req *api.UpdateWorkloadPresetRequest) (*api.UpdateWorkloadPresetResponse, error) {
	c, err := s.d.caller(ctx)
	if err != nil {
		return nil, err
	}
	in := req.GetPreset()
	if in == nil || in.GetEntity().GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "preset.entity.id is required")
	}
	updated, err := doTxRet(ctx, s.d, func(ctx context.Context) (*models.WorkloadPresetRecord, error) {
		existing, err := s.d.Workloads.Get(ctx, req.GetTenantId(), in.GetEntity().GetId(), c.GetAccountId())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		if existing.GetIsSystem() {
			return nil, status.Error(codes.FailedPrecondition, "system presets are read-only; clone instead")
		}
		out := &models.WorkloadPresetRecord{
			Entity:   preserveEntity(existing.GetEntity(), in.GetEntity(), s.d),
			Workload: in.GetWorkload(),
			IsSystem: false,
		}
		if err := s.d.Workloads.Update(ctx, out); err != nil {
			return nil, utils.MapErr(err)
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.UpdateWorkloadPresetResponse{Preset: updated}, nil
}

func (s *WorkloadPresetService) DeleteWorkloadPreset(ctx context.Context, req *api.DeleteWorkloadPresetRequest) (*api.DeleteWorkloadPresetResponse, error) {
	c, err := s.d.caller(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.d.doTx(ctx, func(ctx context.Context) error {
		existing, err := s.d.Workloads.Get(ctx, req.GetTenantId(), req.GetId(), c.GetAccountId())
		if err != nil {
			return ignoreNotFound(err)
		}
		if existing.GetIsSystem() {
			return status.Error(codes.FailedPrecondition, "system presets cannot be deleted")
		}
		return ignoreNotFound(s.d.Workloads.Delete(ctx, req.GetTenantId(), req.GetId()))
	}); err != nil {
		return nil, err
	}
	return &api.DeleteWorkloadPresetResponse{}, nil
}

func (s *WorkloadPresetService) CloneWorkloadPreset(ctx context.Context, req *api.CloneWorkloadPresetRequest) (*api.CloneWorkloadPresetResponse, error) {
	c, err := s.d.caller(ctx)
	if err != nil {
		return nil, err
	}
	clone, err := doTxRet(ctx, s.d, func(ctx context.Context) (*models.WorkloadPresetRecord, error) {
		src, err := s.d.Workloads.Get(ctx, req.GetTenantId(), req.GetId(), c.GetAccountId())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		ent := s.d.stampNew(nil, req.GetTenantId(), c.GetAccountId())
		ent.Name = cloneName(req.GetName(), src.GetEntity().GetName())
		ent.Description = src.GetEntity().GetDescription()
		out := &models.WorkloadPresetRecord{
			Entity:   ent,
			Workload: src.GetWorkload(),
			IsSystem: false,
		}
		if err := s.d.Workloads.Create(ctx, out); err != nil {
			return nil, utils.MapErr(err)
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.CloneWorkloadPresetResponse{Preset: clone}, nil
}
