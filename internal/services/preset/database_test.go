package preset

// database_test.go: tests for DatabasePresetService (database.go):
// CreateDatabasePreset, GetDatabasePreset, ListDatabasePresets,
// UpdateDatabasePreset, DeleteDatabasePreset, CloneDatabasePreset.

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
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

func newDatabaseSvc(t *testing.T, ctrl *gomock.Controller) (*DatabasePresetService, *utils.MockAuthn, *MockDatabasePresetRepo) {
	t.Helper()
	authn := utils.NewMockAuthn(ctrl)
	dbs := NewMockDatabasePresetRepo(ctrl)
	svc := NewDatabasePresetService(Deps{
		Authn:     authn,
		Databases: dbs,
		Tx:        &utils.MockTrm{},
	})
	return svc, authn, dbs
}

func makeClaims(accountID string) *iampb.AccessClaims {
	return &iampb.AccessClaims{AccountId: accountID}
}

func makeDBRecord(id, tenantID string, isSystem bool) *models.DatabasePresetRecord {
	return &models.DatabasePresetRecord{
		Entity: &common.Entity{
			Id:       id,
			TenantId: tenantID,
			Name:     "test-preset",
		},
		IsSystem: isSystem,
	}
}

// -------- CreateDatabasePreset --------

func TestCreateDatabasePreset(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, dbs := newDatabaseSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		dbs.EXPECT().Create(ctx, gomock.Any()).Return(nil)

		resp, err := svc.CreateDatabasePreset(ctx, &api.CreateDatabasePresetRequest{
			TenantId: "t1",
			Preset:   &models.DatabasePresetRecord{},
		})
		if err != nil {
			t.Fatalf("CreateDatabasePreset: %v", err)
		}
		if resp.GetPreset() == nil {
			t.Fatal("expected preset in response")
		}
		// server must stamp author_id and override is_system=false
		if resp.GetPreset().GetEntity().GetAuthorId() != "acc1" {
			t.Errorf("expected author_id=acc1, got %s", resp.GetPreset().GetEntity().GetAuthorId())
		}
		if resp.GetPreset().GetIsSystem() {
			t.Error("expected is_system=false (server override)")
		}
	})

	t.Run("NilPreset_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newDatabaseSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)

		_, err := svc.CreateDatabasePreset(ctx, &api.CreateDatabasePresetRequest{TenantId: "t1"})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newDatabaseSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))

		_, err := svc.CreateDatabasePreset(ctx, &api.CreateDatabasePresetRequest{
			TenantId: "t1",
			Preset:   &models.DatabasePresetRecord{},
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("RepoError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, dbs := newDatabaseSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		dbs.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db error"))

		_, err := svc.CreateDatabasePreset(ctx, &api.CreateDatabasePresetRequest{
			TenantId: "t1",
			Preset:   &models.DatabasePresetRecord{},
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("Conflict", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, dbs := newDatabaseSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		dbs.EXPECT().Create(ctx, gomock.Any()).Return(derrors.ErrConflict)

		_, err := svc.CreateDatabasePreset(ctx, &api.CreateDatabasePresetRequest{
			TenantId: "t1",
			Preset:   &models.DatabasePresetRecord{},
		})
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", err)
		}
	})
}

// -------- GetDatabasePreset --------

func TestGetDatabasePreset(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, dbs := newDatabaseSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		dbs.EXPECT().Get(ctx, "t1", "id1", "acc1").Return(makeDBRecord("id1", "t1", false), nil)

		resp, err := svc.GetDatabasePreset(ctx, &api.GetDatabasePresetRequest{TenantId: "t1", Id: "id1"})
		if err != nil {
			t.Fatalf("GetDatabasePreset: %v", err)
		}
		if resp.GetPreset().GetEntity().GetId() != "id1" {
			t.Errorf("expected id1, got %s", resp.GetPreset().GetEntity().GetId())
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, dbs := newDatabaseSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		dbs.EXPECT().Get(ctx, "t1", "missing", "acc1").Return(nil, derrors.ErrNotFound)

		_, err := svc.GetDatabasePreset(ctx, &api.GetDatabasePresetRequest{TenantId: "t1", Id: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newDatabaseSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))

		_, err := svc.GetDatabasePreset(ctx, &api.GetDatabasePresetRequest{TenantId: "t1", Id: "id1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})
}

// -------- ListDatabasePresets --------

func TestListDatabasePresets(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, dbs := newDatabaseSvc(t, ctrl)

		req := &api.ListDatabasePresetsRequest{TenantId: "t1"}
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		dbs.EXPECT().List(ctx, req, "acc1").Return(
			[]*models.DatabasePresetRecord{makeDBRecord("id1", "t1", false)},
			"next-token",
			nil,
		)

		resp, err := svc.ListDatabasePresets(ctx, req)
		if err != nil {
			t.Fatalf("ListDatabasePresets: %v", err)
		}
		if len(resp.GetPresets()) != 1 {
			t.Errorf("expected 1 preset, got %d", len(resp.GetPresets()))
		}
		if resp.GetNextPageToken() != "next-token" {
			t.Errorf("expected next-token, got %s", resp.GetNextPageToken())
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newDatabaseSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))

		_, err := svc.ListDatabasePresets(ctx, &api.ListDatabasePresetsRequest{TenantId: "t1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("RepoError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, dbs := newDatabaseSvc(t, ctrl)

		req := &api.ListDatabasePresetsRequest{TenantId: "t1"}
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		dbs.EXPECT().List(ctx, req, "acc1").Return(nil, "", errors.New("db error"))

		_, err := svc.ListDatabasePresets(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

// -------- UpdateDatabasePreset --------

func TestUpdateDatabasePreset(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, dbs := newDatabaseSvc(t, ctrl)

		existing := makeDBRecord("id1", "t1", false)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		dbs.EXPECT().Get(ctx, "t1", "id1", "acc1").Return(existing, nil)
		dbs.EXPECT().Update(ctx, gomock.Any()).Return(nil)

		resp, err := svc.UpdateDatabasePreset(ctx, &api.UpdateDatabasePresetRequest{
			TenantId: "t1",
			Preset: &models.DatabasePresetRecord{
				Entity: &common.Entity{Id: "id1", Name: "updated-name"},
			},
		})
		if err != nil {
			t.Fatalf("UpdateDatabasePreset: %v", err)
		}
		if resp.GetPreset() == nil {
			t.Fatal("expected preset in response")
		}
	})

	t.Run("NilPreset_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newDatabaseSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)

		_, err := svc.UpdateDatabasePreset(ctx, &api.UpdateDatabasePresetRequest{TenantId: "t1"})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("EmptyID_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newDatabaseSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)

		_, err := svc.UpdateDatabasePreset(ctx, &api.UpdateDatabasePresetRequest{
			TenantId: "t1",
			Preset:   &models.DatabasePresetRecord{Entity: &common.Entity{Id: ""}},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("SystemPreset_FailedPrecondition", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, dbs := newDatabaseSvc(t, ctrl)

		existing := makeDBRecord("id1", "t1", true) // is_system=true
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		dbs.EXPECT().Get(ctx, "t1", "id1", "acc1").Return(existing, nil)

		_, err := svc.UpdateDatabasePreset(ctx, &api.UpdateDatabasePresetRequest{
			TenantId: "t1",
			Preset:   &models.DatabasePresetRecord{Entity: &common.Entity{Id: "id1"}},
		})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", err)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, dbs := newDatabaseSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		dbs.EXPECT().Get(ctx, "t1", "id1", "acc1").Return(nil, derrors.ErrNotFound)

		_, err := svc.UpdateDatabasePreset(ctx, &api.UpdateDatabasePresetRequest{
			TenantId: "t1",
			Preset:   &models.DatabasePresetRecord{Entity: &common.Entity{Id: "id1"}},
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newDatabaseSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))

		_, err := svc.UpdateDatabasePreset(ctx, &api.UpdateDatabasePresetRequest{
			TenantId: "t1",
			Preset:   &models.DatabasePresetRecord{Entity: &common.Entity{Id: "id1"}},
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("UpdateRepoError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, dbs := newDatabaseSvc(t, ctrl)

		existing := makeDBRecord("id1", "t1", false)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		dbs.EXPECT().Get(ctx, "t1", "id1", "acc1").Return(existing, nil)
		dbs.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("db error"))

		_, err := svc.UpdateDatabasePreset(ctx, &api.UpdateDatabasePresetRequest{
			TenantId: "t1",
			Preset:   &models.DatabasePresetRecord{Entity: &common.Entity{Id: "id1"}},
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

// -------- DeleteDatabasePreset --------

func TestDeleteDatabasePreset(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, dbs := newDatabaseSvc(t, ctrl)

		existing := makeDBRecord("id1", "t1", false)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		dbs.EXPECT().Get(ctx, "t1", "id1", "acc1").Return(existing, nil)
		dbs.EXPECT().Delete(ctx, "t1", "id1").Return(nil)

		_, err := svc.DeleteDatabasePreset(ctx, &api.DeleteDatabasePresetRequest{TenantId: "t1", Id: "id1"})
		if err != nil {
			t.Fatalf("DeleteDatabasePreset: %v", err)
		}
	})

	t.Run("NotFound_Idempotent", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, dbs := newDatabaseSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		dbs.EXPECT().Get(ctx, "t1", "missing", "acc1").Return(nil, derrors.ErrNotFound)

		_, err := svc.DeleteDatabasePreset(ctx, &api.DeleteDatabasePresetRequest{TenantId: "t1", Id: "missing"})
		if err != nil {
			t.Fatalf("expected no error for idempotent delete, got %v", err)
		}
	})

	t.Run("SystemPreset_FailedPrecondition", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, dbs := newDatabaseSvc(t, ctrl)

		existing := makeDBRecord("id1", "t1", true)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		dbs.EXPECT().Get(ctx, "t1", "id1", "acc1").Return(existing, nil)

		_, err := svc.DeleteDatabasePreset(ctx, &api.DeleteDatabasePresetRequest{TenantId: "t1", Id: "id1"})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newDatabaseSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))

		_, err := svc.DeleteDatabasePreset(ctx, &api.DeleteDatabasePresetRequest{TenantId: "t1", Id: "id1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("GetError_NonNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, dbs := newDatabaseSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		dbs.EXPECT().Get(ctx, "t1", "id1", "acc1").Return(nil, errors.New("db error"))

		_, err := svc.DeleteDatabasePreset(ctx, &api.DeleteDatabasePresetRequest{TenantId: "t1", Id: "id1"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("DeleteAfterGetNotFound_Idempotent", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, dbs := newDatabaseSvc(t, ctrl)

		existing := makeDBRecord("id1", "t1", false)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		dbs.EXPECT().Get(ctx, "t1", "id1", "acc1").Return(existing, nil)
		// repo.Delete returns ErrNotFound (race: deleted between Get and Delete)
		dbs.EXPECT().Delete(ctx, "t1", "id1").Return(derrors.ErrNotFound)

		_, err := svc.DeleteDatabasePreset(ctx, &api.DeleteDatabasePresetRequest{TenantId: "t1", Id: "id1"})
		if err != nil {
			t.Fatalf("expected idempotent delete, got %v", err)
		}
	})
}

// -------- CloneDatabasePreset --------

func TestCloneDatabasePreset(t *testing.T) {
	ctx := context.Background()

	t.Run("Success_DefaultName", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, dbs := newDatabaseSvc(t, ctrl)

		src := makeDBRecord("src1", "t1", false)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		dbs.EXPECT().Get(ctx, "t1", "src1", "acc1").Return(src, nil)
		dbs.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, r *models.DatabasePresetRecord) error {
			if r.GetEntity().GetName() != "test-preset (copy)" {
				t.Errorf("expected 'test-preset (copy)', got %s", r.GetEntity().GetName())
			}
			if r.GetIsSystem() {
				t.Error("clone must not be system")
			}
			return nil
		})

		resp, err := svc.CloneDatabasePreset(ctx, &api.CloneDatabasePresetRequest{TenantId: "t1", Id: "src1"})
		if err != nil {
			t.Fatalf("CloneDatabasePreset: %v", err)
		}
		if resp.GetPreset() == nil {
			t.Fatal("expected preset in response")
		}
	})

	t.Run("Success_OverrideName", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, dbs := newDatabaseSvc(t, ctrl)

		src := makeDBRecord("src1", "t1", false)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		dbs.EXPECT().Get(ctx, "t1", "src1", "acc1").Return(src, nil)
		dbs.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, r *models.DatabasePresetRecord) error {
			if r.GetEntity().GetName() != "custom-clone" {
				t.Errorf("expected custom-clone, got %s", r.GetEntity().GetName())
			}
			return nil
		})

		_, err := svc.CloneDatabasePreset(ctx, &api.CloneDatabasePresetRequest{
			TenantId: "t1",
			Id:       "src1",
			Name:     "custom-clone",
		})
		if err != nil {
			t.Fatalf("CloneDatabasePreset with override name: %v", err)
		}
	})

	t.Run("SourceNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, dbs := newDatabaseSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		dbs.EXPECT().Get(ctx, "t1", "missing", "acc1").Return(nil, derrors.ErrNotFound)

		_, err := svc.CloneDatabasePreset(ctx, &api.CloneDatabasePresetRequest{TenantId: "t1", Id: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _ := newDatabaseSvc(t, ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))

		_, err := svc.CloneDatabasePreset(ctx, &api.CloneDatabasePresetRequest{TenantId: "t1", Id: "src1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("CreateError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, dbs := newDatabaseSvc(t, ctrl)

		src := makeDBRecord("src1", "t1", false)
		authn.EXPECT().Caller(ctx).Return(makeClaims("acc1"), nil)
		dbs.EXPECT().Get(ctx, "t1", "src1", "acc1").Return(src, nil)
		dbs.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db error"))

		_, err := svc.CloneDatabasePreset(ctx, &api.CloneDatabasePresetRequest{TenantId: "t1", Id: "src1"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
