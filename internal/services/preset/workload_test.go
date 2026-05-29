package preset

// workload_test.go: tests for WorkloadPresetService (workload.go):
// CreateWorkloadPreset, GetWorkloadPreset, ListWorkloadPresets,
// UpdateWorkloadPreset, DeleteWorkloadPreset, CloneWorkloadPreset.

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

func newWorkloadSvc(t *testing.T, ctrl *gomock.Controller) (*WorkloadPresetService, *utils.MockAuthn, *MockWorkloadPresetRepo) {
	t.Helper()
	authn := utils.NewMockAuthn(ctrl)
	wls := NewMockWorkloadPresetRepo(ctrl)
	svc := NewWorkloadPresetService(Deps{
		Authn:     authn,
		Workloads: wls,
		Tx:        &utils.MockTrm{},
	})
	return svc, authn, wls
}

func makeWLRecord(id, tenantID string, isSystem bool) *models.WorkloadPresetRecord {
	return &models.WorkloadPresetRecord{
		Entity: &common.Entity{
			Id:       id,
			TenantId: tenantID,
			Name:     "wl-preset",
		},
		IsSystem: isSystem,
	}
}

// -------- CreateWorkloadPreset --------

func TestCreateWorkloadPreset(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, wls := newWorkloadSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		wls.EXPECT().Create(ctx, gomock.Any()).Return(nil)

		resp, err := svc.CreateWorkloadPreset(ctx, &api.CreateWorkloadPresetRequest{
			TenantId: "t1",
			Preset:   &models.WorkloadPresetRecord{},
		})
		if err != nil {
			t.Fatalf("CreateWorkloadPreset: %v", err)
		}
		if resp.GetPreset().GetIsSystem() {
			t.Error("expected is_system=false")
		}
		if resp.GetPreset().GetEntity().GetAuthorId() != "acc1" {
			t.Errorf("expected author_id=acc1, got %s", resp.GetPreset().GetEntity().GetAuthorId())
		}
	})

	t.Run("NilPreset_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newWorkloadSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)

		_, err := svc.CreateWorkloadPreset(ctx, &api.CreateWorkloadPresetRequest{TenantId: "t1"})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newWorkloadSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))

		_, err := svc.CreateWorkloadPreset(ctx, &api.CreateWorkloadPresetRequest{
			TenantId: "t1",
			Preset:   &models.WorkloadPresetRecord{},
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("Conflict", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, wls := newWorkloadSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		wls.EXPECT().Create(ctx, gomock.Any()).Return(derrors.ErrConflict)

		_, err := svc.CreateWorkloadPreset(ctx, &api.CreateWorkloadPresetRequest{
			TenantId: "t1",
			Preset:   &models.WorkloadPresetRecord{},
		})
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", err)
		}
	})
}

// -------- GetWorkloadPreset --------

func TestGetWorkloadPreset(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, wls := newWorkloadSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		wls.EXPECT().Get(ctx, "t1", "wl1", "acc1").Return(makeWLRecord("wl1", "t1", false), nil)

		resp, err := svc.GetWorkloadPreset(ctx, &api.GetWorkloadPresetRequest{TenantId: "t1", Id: "wl1"})
		if err != nil {
			t.Fatalf("GetWorkloadPreset: %v", err)
		}
		if resp.GetPreset().GetEntity().GetId() != "wl1" {
			t.Errorf("expected wl1, got %s", resp.GetPreset().GetEntity().GetId())
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, wls := newWorkloadSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		wls.EXPECT().Get(ctx, "t1", "missing", "acc1").Return(nil, derrors.ErrNotFound)

		_, err := svc.GetWorkloadPreset(ctx, &api.GetWorkloadPresetRequest{TenantId: "t1", Id: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newWorkloadSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))

		_, err := svc.GetWorkloadPreset(ctx, &api.GetWorkloadPresetRequest{TenantId: "t1", Id: "wl1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})
}

// -------- ListWorkloadPresets --------

func TestListWorkloadPresets(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, wls := newWorkloadSvc(t, ctrl)

		req := &api.ListWorkloadPresetsRequest{TenantId: "t1"}
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		wls.EXPECT().List(ctx, req, "acc1").Return(
			[]*models.WorkloadPresetRecord{makeWLRecord("wl1", "t1", false)},
			"",
			nil,
		)

		resp, err := svc.ListWorkloadPresets(ctx, req)
		if err != nil {
			t.Fatalf("ListWorkloadPresets: %v", err)
		}
		if len(resp.GetPresets()) != 1 {
			t.Errorf("expected 1 preset, got %d", len(resp.GetPresets()))
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newWorkloadSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))

		_, err := svc.ListWorkloadPresets(ctx, &api.ListWorkloadPresetsRequest{TenantId: "t1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("RepoError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, wls := newWorkloadSvc(t, ctrl)

		req := &api.ListWorkloadPresetsRequest{TenantId: "t1"}
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		wls.EXPECT().List(ctx, req, "acc1").Return(nil, "", errors.New("db error"))

		_, err := svc.ListWorkloadPresets(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

// -------- UpdateWorkloadPreset --------

func TestUpdateWorkloadPreset(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, wls := newWorkloadSvc(t, ctrl)

		existing := makeWLRecord("wl1", "t1", false)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		wls.EXPECT().Get(ctx, "t1", "wl1", "acc1").Return(existing, nil)
		wls.EXPECT().Update(ctx, gomock.Any()).Return(nil)

		resp, err := svc.UpdateWorkloadPreset(ctx, &api.UpdateWorkloadPresetRequest{
			TenantId: "t1",
			Preset: &models.WorkloadPresetRecord{
				Entity: &common.Entity{Id: "wl1", Name: "updated"},
			},
		})
		if err != nil {
			t.Fatalf("UpdateWorkloadPreset: %v", err)
		}
		if resp.GetPreset() == nil {
			t.Fatal("expected preset in response")
		}
	})

	t.Run("NilPreset_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newWorkloadSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)

		_, err := svc.UpdateWorkloadPreset(ctx, &api.UpdateWorkloadPresetRequest{TenantId: "t1"})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("SystemPreset_FailedPrecondition", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, wls := newWorkloadSvc(t, ctrl)

		existing := makeWLRecord("wl1", "t1", true)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		wls.EXPECT().Get(ctx, "t1", "wl1", "acc1").Return(existing, nil)

		_, err := svc.UpdateWorkloadPreset(ctx, &api.UpdateWorkloadPresetRequest{
			TenantId: "t1",
			Preset:   &models.WorkloadPresetRecord{Entity: &common.Entity{Id: "wl1"}},
		})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", err)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, wls := newWorkloadSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		wls.EXPECT().Get(ctx, "t1", "wl1", "acc1").Return(nil, derrors.ErrNotFound)

		_, err := svc.UpdateWorkloadPreset(ctx, &api.UpdateWorkloadPresetRequest{
			TenantId: "t1",
			Preset:   &models.WorkloadPresetRecord{Entity: &common.Entity{Id: "wl1"}},
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newWorkloadSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))

		_, err := svc.UpdateWorkloadPreset(ctx, &api.UpdateWorkloadPresetRequest{
			TenantId: "t1",
			Preset:   &models.WorkloadPresetRecord{Entity: &common.Entity{Id: "wl1"}},
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})
}

// -------- DeleteWorkloadPreset --------

func TestDeleteWorkloadPreset(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, wls := newWorkloadSvc(t, ctrl)

		existing := makeWLRecord("wl1", "t1", false)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		wls.EXPECT().Get(ctx, "t1", "wl1", "acc1").Return(existing, nil)
		wls.EXPECT().Delete(ctx, "t1", "wl1").Return(nil)

		_, err := svc.DeleteWorkloadPreset(ctx, &api.DeleteWorkloadPresetRequest{TenantId: "t1", Id: "wl1"})
		if err != nil {
			t.Fatalf("DeleteWorkloadPreset: %v", err)
		}
	})

	t.Run("NotFound_Idempotent", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, wls := newWorkloadSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		wls.EXPECT().Get(ctx, "t1", "missing", "acc1").Return(nil, derrors.ErrNotFound)

		_, err := svc.DeleteWorkloadPreset(ctx, &api.DeleteWorkloadPresetRequest{TenantId: "t1", Id: "missing"})
		if err != nil {
			t.Fatalf("expected no error for idempotent delete, got %v", err)
		}
	})

	t.Run("SystemPreset_FailedPrecondition", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, wls := newWorkloadSvc(t, ctrl)

		existing := makeWLRecord("wl1", "t1", true)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		wls.EXPECT().Get(ctx, "t1", "wl1", "acc1").Return(existing, nil)

		_, err := svc.DeleteWorkloadPreset(ctx, &api.DeleteWorkloadPresetRequest{TenantId: "t1", Id: "wl1"})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newWorkloadSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))

		_, err := svc.DeleteWorkloadPreset(ctx, &api.DeleteWorkloadPresetRequest{TenantId: "t1", Id: "wl1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})
}

// -------- CloneWorkloadPreset --------

func TestCloneWorkloadPreset(t *testing.T) {
	ctx := context.Background()

	t.Run("Success_DefaultName", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, wls := newWorkloadSvc(t, ctrl)

		src := makeWLRecord("src1", "t1", false)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		wls.EXPECT().Get(ctx, "t1", "src1", "acc1").Return(src, nil)
		wls.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, r *models.WorkloadPresetRecord) error {
			if r.GetEntity().GetName() != "wl-preset (copy)" {
				t.Errorf("expected 'wl-preset (copy)', got %s", r.GetEntity().GetName())
			}
			return nil
		})

		resp, err := svc.CloneWorkloadPreset(ctx, &api.CloneWorkloadPresetRequest{TenantId: "t1", Id: "src1"})
		if err != nil {
			t.Fatalf("CloneWorkloadPreset: %v", err)
		}
		if resp.GetPreset() == nil {
			t.Fatal("expected preset in response")
		}
	})

	t.Run("Success_OverrideName", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, wls := newWorkloadSvc(t, ctrl)

		src := makeWLRecord("src1", "t1", false)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		wls.EXPECT().Get(ctx, "t1", "src1", "acc1").Return(src, nil)
		wls.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, r *models.WorkloadPresetRecord) error {
			if r.GetEntity().GetName() != "my-clone" {
				t.Errorf("expected my-clone, got %s", r.GetEntity().GetName())
			}
			return nil
		})

		_, err := svc.CloneWorkloadPreset(ctx, &api.CloneWorkloadPresetRequest{
			TenantId: "t1",
			Id:       "src1",
			Name:     "my-clone",
		})
		if err != nil {
			t.Fatalf("CloneWorkloadPreset with override: %v", err)
		}
	})

	t.Run("SourceNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, wls := newWorkloadSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		wls.EXPECT().Get(ctx, "t1", "missing", "acc1").Return(nil, derrors.ErrNotFound)

		_, err := svc.CloneWorkloadPreset(ctx, &api.CloneWorkloadPresetRequest{TenantId: "t1", Id: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newWorkloadSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))

		_, err := svc.CloneWorkloadPreset(ctx, &api.CloneWorkloadPresetRequest{TenantId: "t1", Id: "src1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})
}
