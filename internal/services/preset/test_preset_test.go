package preset

// test_preset_test.go: tests for TestPresetService (test.go):
// CreateTestPreset, GetTestPreset, ListTestPresets,
// UpdateTestPreset, DeleteTestPreset, CloneTestPreset.

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

func newTestSvc(t *testing.T, ctrl *gomock.Controller) (*TestPresetService, *utils.MockAuthn, *MockTestPresetRepo) {
	t.Helper()
	authn := utils.NewMockAuthn(ctrl)
	tests := NewMockTestPresetRepo(ctrl)
	svc := NewTestPresetService(Deps{
		Authn: authn,
		Tests: tests,
		Tx:    &utils.MockTrm{},
	})
	return svc, authn, tests
}

func makeTestRecord(id, tenantID string, isSystem bool) *models.TestPresetRecord {
	return &models.TestPresetRecord{
		Entity: &common.Entity{
			Id:       id,
			TenantId: tenantID,
			Name:     "tp-preset",
		},
		IsSystem: isSystem,
	}
}

// -------- CreateTestPreset --------

func TestCreateTestPreset(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tests := newTestSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		tests.EXPECT().Create(ctx, gomock.Any()).Return(nil)

		resp, err := svc.CreateTestPreset(ctx, &api.CreateTestPresetRequest{
			TenantId: "t1",
			Preset:   &models.TestPresetRecord{},
		})
		if err != nil {
			t.Fatalf("CreateTestPreset: %v", err)
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
		svc, authn, _ := newTestSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)

		_, err := svc.CreateTestPreset(ctx, &api.CreateTestPresetRequest{TenantId: "t1"})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newTestSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))

		_, err := svc.CreateTestPreset(ctx, &api.CreateTestPresetRequest{
			TenantId: "t1",
			Preset:   &models.TestPresetRecord{},
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("Conflict", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tests := newTestSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		tests.EXPECT().Create(ctx, gomock.Any()).Return(derrors.ErrConflict)

		_, err := svc.CreateTestPreset(ctx, &api.CreateTestPresetRequest{
			TenantId: "t1",
			Preset:   &models.TestPresetRecord{},
		})
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", err)
		}
	})
}

// -------- GetTestPreset --------

func TestGetTestPreset(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tests := newTestSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		tests.EXPECT().Get(ctx, "t1", "tp1", "acc1").Return(makeTestRecord("tp1", "t1", false), nil)

		resp, err := svc.GetTestPreset(ctx, &api.GetTestPresetRequest{TenantId: "t1", Id: "tp1"})
		if err != nil {
			t.Fatalf("GetTestPreset: %v", err)
		}
		if resp.GetPreset().GetEntity().GetId() != "tp1" {
			t.Errorf("expected tp1, got %s", resp.GetPreset().GetEntity().GetId())
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tests := newTestSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		tests.EXPECT().Get(ctx, "t1", "missing", "acc1").Return(nil, derrors.ErrNotFound)

		_, err := svc.GetTestPreset(ctx, &api.GetTestPresetRequest{TenantId: "t1", Id: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newTestSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))

		_, err := svc.GetTestPreset(ctx, &api.GetTestPresetRequest{TenantId: "t1", Id: "tp1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})
}

// -------- ListTestPresets --------

func TestListTestPresets(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tests := newTestSvc(t, ctrl)

		req := &api.ListTestPresetsRequest{TenantId: "t1"}
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		tests.EXPECT().List(ctx, req, "acc1").Return(
			[]*models.TestPresetRecord{makeTestRecord("tp1", "t1", false)},
			"next-page",
			nil,
		)

		resp, err := svc.ListTestPresets(ctx, req)
		if err != nil {
			t.Fatalf("ListTestPresets: %v", err)
		}
		if len(resp.GetPresets()) != 1 {
			t.Errorf("expected 1 preset, got %d", len(resp.GetPresets()))
		}
		if resp.GetNextPageToken() != "next-page" {
			t.Errorf("expected next-page, got %s", resp.GetNextPageToken())
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newTestSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))

		_, err := svc.ListTestPresets(ctx, &api.ListTestPresetsRequest{TenantId: "t1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("RepoError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tests := newTestSvc(t, ctrl)

		req := &api.ListTestPresetsRequest{TenantId: "t1"}
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		tests.EXPECT().List(ctx, req, "acc1").Return(nil, "", errors.New("db error"))

		_, err := svc.ListTestPresets(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

// -------- UpdateTestPreset --------

func TestUpdateTestPreset(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tests := newTestSvc(t, ctrl)

		existing := makeTestRecord("tp1", "t1", false)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		tests.EXPECT().Get(ctx, "t1", "tp1", "acc1").Return(existing, nil)
		tests.EXPECT().Update(ctx, gomock.Any()).Return(nil)

		resp, err := svc.UpdateTestPreset(ctx, &api.UpdateTestPresetRequest{
			TenantId: "t1",
			Preset: &models.TestPresetRecord{
				Entity: &common.Entity{Id: "tp1", Name: "updated"},
			},
		})
		if err != nil {
			t.Fatalf("UpdateTestPreset: %v", err)
		}
		if resp.GetPreset() == nil {
			t.Fatal("expected preset in response")
		}
	})

	t.Run("NilPreset_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newTestSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)

		_, err := svc.UpdateTestPreset(ctx, &api.UpdateTestPresetRequest{TenantId: "t1"})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("EmptyID_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newTestSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)

		_, err := svc.UpdateTestPreset(ctx, &api.UpdateTestPresetRequest{
			TenantId: "t1",
			Preset:   &models.TestPresetRecord{Entity: &common.Entity{Id: ""}},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("SystemPreset_FailedPrecondition", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tests := newTestSvc(t, ctrl)

		existing := makeTestRecord("tp1", "t1", true)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		tests.EXPECT().Get(ctx, "t1", "tp1", "acc1").Return(existing, nil)

		_, err := svc.UpdateTestPreset(ctx, &api.UpdateTestPresetRequest{
			TenantId: "t1",
			Preset:   &models.TestPresetRecord{Entity: &common.Entity{Id: "tp1"}},
		})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", err)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tests := newTestSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		tests.EXPECT().Get(ctx, "t1", "tp1", "acc1").Return(nil, derrors.ErrNotFound)

		_, err := svc.UpdateTestPreset(ctx, &api.UpdateTestPresetRequest{
			TenantId: "t1",
			Preset:   &models.TestPresetRecord{Entity: &common.Entity{Id: "tp1"}},
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newTestSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))

		_, err := svc.UpdateTestPreset(ctx, &api.UpdateTestPresetRequest{
			TenantId: "t1",
			Preset:   &models.TestPresetRecord{Entity: &common.Entity{Id: "tp1"}},
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("UpdateRepoError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tests := newTestSvc(t, ctrl)

		existing := makeTestRecord("tp1", "t1", false)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		tests.EXPECT().Get(ctx, "t1", "tp1", "acc1").Return(existing, nil)
		tests.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("db error"))

		_, err := svc.UpdateTestPreset(ctx, &api.UpdateTestPresetRequest{
			TenantId: "t1",
			Preset:   &models.TestPresetRecord{Entity: &common.Entity{Id: "tp1"}},
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

// -------- DeleteTestPreset --------

func TestDeleteTestPreset(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tests := newTestSvc(t, ctrl)

		existing := makeTestRecord("tp1", "t1", false)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		tests.EXPECT().Get(ctx, "t1", "tp1", "acc1").Return(existing, nil)
		tests.EXPECT().Delete(ctx, "t1", "tp1").Return(nil)

		_, err := svc.DeleteTestPreset(ctx, &api.DeleteTestPresetRequest{TenantId: "t1", Id: "tp1"})
		if err != nil {
			t.Fatalf("DeleteTestPreset: %v", err)
		}
	})

	t.Run("NotFound_Idempotent", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tests := newTestSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		tests.EXPECT().Get(ctx, "t1", "missing", "acc1").Return(nil, derrors.ErrNotFound)

		_, err := svc.DeleteTestPreset(ctx, &api.DeleteTestPresetRequest{TenantId: "t1", Id: "missing"})
		if err != nil {
			t.Fatalf("expected no error for idempotent delete, got %v", err)
		}
	})

	t.Run("SystemPreset_FailedPrecondition", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tests := newTestSvc(t, ctrl)

		existing := makeTestRecord("tp1", "t1", true)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		tests.EXPECT().Get(ctx, "t1", "tp1", "acc1").Return(existing, nil)

		_, err := svc.DeleteTestPreset(ctx, &api.DeleteTestPresetRequest{TenantId: "t1", Id: "tp1"})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newTestSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))

		_, err := svc.DeleteTestPreset(ctx, &api.DeleteTestPresetRequest{TenantId: "t1", Id: "tp1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("GetError_NonNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tests := newTestSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		tests.EXPECT().Get(ctx, "t1", "tp1", "acc1").Return(nil, errors.New("db error"))

		_, err := svc.DeleteTestPreset(ctx, &api.DeleteTestPresetRequest{TenantId: "t1", Id: "tp1"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

// -------- CloneTestPreset --------

func TestCloneTestPreset(t *testing.T) {
	ctx := context.Background()

	t.Run("Success_DefaultName", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tests := newTestSvc(t, ctrl)

		src := makeTestRecord("src1", "t1", false)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		tests.EXPECT().Get(ctx, "t1", "src1", "acc1").Return(src, nil)
		tests.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, r *models.TestPresetRecord) error {
			if r.GetEntity().GetName() != "tp-preset (copy)" {
				t.Errorf("expected 'tp-preset (copy)', got %s", r.GetEntity().GetName())
			}
			if r.GetIsSystem() {
				t.Error("clone must not be system")
			}
			return nil
		})

		resp, err := svc.CloneTestPreset(ctx, &api.CloneTestPresetRequest{TenantId: "t1", Id: "src1"})
		if err != nil {
			t.Fatalf("CloneTestPreset: %v", err)
		}
		if resp.GetPreset() == nil {
			t.Fatal("expected preset in response")
		}
	})

	t.Run("Success_OverrideName", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tests := newTestSvc(t, ctrl)

		src := makeTestRecord("src1", "t1", false)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		tests.EXPECT().Get(ctx, "t1", "src1", "acc1").Return(src, nil)
		tests.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, r *models.TestPresetRecord) error {
			if r.GetEntity().GetName() != "override-name" {
				t.Errorf("expected override-name, got %s", r.GetEntity().GetName())
			}
			return nil
		})

		_, err := svc.CloneTestPreset(ctx, &api.CloneTestPresetRequest{
			TenantId: "t1",
			Id:       "src1",
			Name:     "override-name",
		})
		if err != nil {
			t.Fatalf("CloneTestPreset with override: %v", err)
		}
	})

	t.Run("SourceNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tests := newTestSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		tests.EXPECT().Get(ctx, "t1", "missing", "acc1").Return(nil, derrors.ErrNotFound)

		_, err := svc.CloneTestPreset(ctx, &api.CloneTestPresetRequest{TenantId: "t1", Id: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newTestSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))

		_, err := svc.CloneTestPreset(ctx, &api.CloneTestPresetRequest{TenantId: "t1", Id: "src1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("CreateError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, tests := newTestSvc(t, ctrl)

		src := makeTestRecord("src1", "t1", false)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		tests.EXPECT().Get(ctx, "t1", "src1", "acc1").Return(src, nil)
		tests.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db error"))

		_, err := svc.CloneTestPreset(ctx, &api.CloneTestPresetRequest{TenantId: "t1", Id: "src1"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
