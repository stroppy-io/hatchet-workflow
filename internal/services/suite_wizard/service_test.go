package suite_wizard

// service_test.go: unit tests for all 6 RPCs of SuiteWizardService.
//
// Mocks are generated in mocks_test.go; MockTrm + FakeClock live in
// mock_tx_test.go. We do NOT modify those files.

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
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// newSvc builds a SuiteWizardService wired up with gomock mocks.
func newSvc(
	authn *utils.MockAuthn,
	drafts *MockDraftRepo,
	suites *MockSuiteReader,
	engine *MockWizardEngine,
) *SuiteWizardService {
	return NewSuiteWizardService(SuiteWizardDeps{
		Authn:  authn,
		Drafts: drafts,
		Suites: suites,
		Engine: engine,
		Tx:     &utils.MockTrm{},
	})
}

// fakeClaims returns a minimal AccessClaims for the given account.
func fakeClaims(accountID string) *iampb.AccessClaims {
	return &iampb.AccessClaims{AccountId: accountID}
}

// fakeDraft returns a minimal SuiteWizardDraftRecord.
func fakeDraft(id, tenantID string) *models.SuiteWizardDraftRecord {
	return &models.SuiteWizardDraftRecord{
		Entity: &common.Entity{
			Id:       id,
			TenantId: tenantID,
			Timings:  &common.Timings{},
		},
		Ready: false,
	}
}

// ===== StartSuiteWizard =====

func TestStartSuiteWizard(t *testing.T) {
	ctx := context.Background()

	t.Run("Success_NoSeed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		authn.EXPECT().Caller(ctx).Return(fakeClaims("acc1"), nil)
		engine.EXPECT().InitialForm(ctx, "t1", "my-suite", (*domain.Suite)(nil)).Return(nil, nil)
		engine.EXPECT().Compute(ctx, "t1", nil).Return(nil, nil, nil, true, nil)
		drafts.EXPECT().Create(ctx, gomock.Any()).Return(nil)

		resp, err := svc.StartSuiteWizard(ctx, &api.StartSuiteWizardRequest{
			TenantId: "t1",
			Name:     "my-suite",
		})
		if err != nil {
			t.Fatalf("StartSuiteWizard: %v", err)
		}
		if resp.Draft == nil {
			t.Fatal("expected draft in response")
		}
	})

	t.Run("Success_WithSeed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		suite := &domain.Suite{}
		authn.EXPECT().Caller(ctx).Return(fakeClaims("acc1"), nil)
		suites.EXPECT().Get(ctx, "t1", "suite-id").Return(suite, nil)
		engine.EXPECT().InitialForm(ctx, "t1", "seeded", suite).Return(nil, nil)
		engine.EXPECT().Compute(ctx, "t1", nil).Return(nil, nil, nil, false, nil)
		drafts.EXPECT().Create(ctx, gomock.Any()).Return(nil)

		resp, err := svc.StartSuiteWizard(ctx, &api.StartSuiteWizardRequest{
			TenantId: "t1",
			Name:     "seeded",
			SuiteId:  "suite-id",
		})
		if err != nil {
			t.Fatalf("StartSuiteWizard with seed: %v", err)
		}
		if resp.Draft == nil {
			t.Fatal("expected draft in response")
		}
	})

	t.Run("CallerError_Unauthenticated", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no claims"))

		_, err := svc.StartSuiteWizard(ctx, &api.StartSuiteWizardRequest{TenantId: "t1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("SuiteReader_NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		authn.EXPECT().Caller(ctx).Return(fakeClaims("acc1"), nil)
		suites.EXPECT().Get(ctx, "t1", "no-such-suite").Return(nil, derrors.ErrNotFound)

		_, err := svc.StartSuiteWizard(ctx, &api.StartSuiteWizardRequest{
			TenantId: "t1",
			SuiteId:  "no-such-suite",
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("SuiteReader_InternalError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		authn.EXPECT().Caller(ctx).Return(fakeClaims("acc1"), nil)
		suites.EXPECT().Get(ctx, "t1", "sid").Return(nil, errors.New("db down"))

		_, err := svc.StartSuiteWizard(ctx, &api.StartSuiteWizardRequest{
			TenantId: "t1",
			SuiteId:  "sid",
		})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("InitialForm_Error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		authn.EXPECT().Caller(ctx).Return(fakeClaims("acc1"), nil)
		engine.EXPECT().InitialForm(ctx, "t1", "", (*domain.Suite)(nil)).Return(nil, errors.New("engine err"))

		_, err := svc.StartSuiteWizard(ctx, &api.StartSuiteWizardRequest{TenantId: "t1"})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("Compute_Error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		authn.EXPECT().Caller(ctx).Return(fakeClaims("acc1"), nil)
		engine.EXPECT().InitialForm(ctx, "t1", "", (*domain.Suite)(nil)).Return(nil, nil)
		engine.EXPECT().Compute(ctx, "t1", nil).Return(nil, nil, nil, false, errors.New("compute err"))

		_, err := svc.StartSuiteWizard(ctx, &api.StartSuiteWizardRequest{TenantId: "t1"})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("DraftCreate_Conflict", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		authn.EXPECT().Caller(ctx).Return(fakeClaims("acc1"), nil)
		engine.EXPECT().InitialForm(ctx, "t1", "", (*domain.Suite)(nil)).Return(nil, nil)
		engine.EXPECT().Compute(ctx, "t1", nil).Return(nil, nil, nil, false, nil)
		drafts.EXPECT().Create(ctx, gomock.Any()).Return(derrors.ErrConflict)

		_, err := svc.StartSuiteWizard(ctx, &api.StartSuiteWizardRequest{TenantId: "t1"})
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", err)
		}
	})

	t.Run("DraftCreate_InternalError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		authn.EXPECT().Caller(ctx).Return(fakeClaims("acc1"), nil)
		engine.EXPECT().InitialForm(ctx, "t1", "", (*domain.Suite)(nil)).Return(nil, nil)
		engine.EXPECT().Compute(ctx, "t1", nil).Return(nil, nil, nil, false, nil)
		drafts.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db err"))

		_, err := svc.StartSuiteWizard(ctx, &api.StartSuiteWizardRequest{TenantId: "t1"})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})
}

// ===== GetSuiteWizardDraft =====

func TestGetSuiteWizardDraft(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		draft := fakeDraft("d1", "t1")
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(draft, nil)

		resp, err := svc.GetSuiteWizardDraft(ctx, &api.GetSuiteWizardDraftRequest{
			TenantId: "t1",
			DraftId:  "d1",
		})
		if err != nil {
			t.Fatalf("GetSuiteWizardDraft: %v", err)
		}
		if resp.Draft == nil {
			t.Fatal("expected draft")
		}
		if resp.Draft.Entity.Id != "d1" {
			t.Errorf("expected draft id d1, got %s", resp.Draft.Entity.Id)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		drafts.EXPECT().Get(ctx, "t1", "missing").Return(nil, derrors.ErrNotFound)

		_, err := svc.GetSuiteWizardDraft(ctx, &api.GetSuiteWizardDraftRequest{
			TenantId: "t1",
			DraftId:  "missing",
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("InternalError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		drafts.EXPECT().Get(ctx, "t1", "d1").Return(nil, errors.New("db err"))

		_, err := svc.GetSuiteWizardDraft(ctx, &api.GetSuiteWizardDraftRequest{
			TenantId: "t1",
			DraftId:  "d1",
		})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})
}

// ===== ListSuiteWizardDrafts =====

func TestListSuiteWizardDrafts(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		list := []*models.SuiteWizardDraftRecord{
			fakeDraft("d1", "t1"),
			fakeDraft("d2", "t1"),
		}
		drafts.EXPECT().List(ctx, "t1", nil, nil, nil).Return(list, "next-token", nil)

		resp, err := svc.ListSuiteWizardDrafts(ctx, &api.ListSuiteWizardDraftsRequest{
			TenantId: "t1",
		})
		if err != nil {
			t.Fatalf("ListSuiteWizardDrafts: %v", err)
		}
		if len(resp.Drafts) != 2 {
			t.Errorf("expected 2 drafts, got %d", len(resp.Drafts))
		}
		if resp.NextPageToken != "next-token" {
			t.Errorf("expected next-token, got %s", resp.NextPageToken)
		}
	})

	t.Run("EmptyList", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		drafts.EXPECT().List(ctx, "t1", nil, nil, nil).Return(nil, "", nil)

		resp, err := svc.ListSuiteWizardDrafts(ctx, &api.ListSuiteWizardDraftsRequest{TenantId: "t1"})
		if err != nil {
			t.Fatalf("ListSuiteWizardDrafts empty: %v", err)
		}
		if len(resp.Drafts) != 0 {
			t.Errorf("expected 0 drafts, got %d", len(resp.Drafts))
		}
	})

	t.Run("InternalError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		drafts.EXPECT().List(ctx, "t1", nil, nil, nil).Return(nil, "", errors.New("db err"))

		_, err := svc.ListSuiteWizardDrafts(ctx, &api.ListSuiteWizardDraftsRequest{TenantId: "t1"})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})
}

// ===== PatchSuiteWizard =====

func TestPatchSuiteWizard(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		draft := fakeDraft("d1", "t1")
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(draft, nil)
		engine.EXPECT().Compute(ctx, "t1", nil).Return(nil, nil, nil, true, nil)
		drafts.EXPECT().Update(ctx, gomock.Any()).Return(nil)

		resp, err := svc.PatchSuiteWizard(ctx, &api.PatchSuiteWizardRequest{
			TenantId: "t1",
			DraftId:  "d1",
		})
		if err != nil {
			t.Fatalf("PatchSuiteWizard: %v", err)
		}
		if resp.Draft == nil {
			t.Fatal("expected draft")
		}
		if !resp.Draft.Ready {
			t.Error("expected draft to be ready")
		}
	})

	t.Run("DraftNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		drafts.EXPECT().Get(ctx, "t1", "missing").Return(nil, derrors.ErrNotFound)

		_, err := svc.PatchSuiteWizard(ctx, &api.PatchSuiteWizardRequest{
			TenantId: "t1",
			DraftId:  "missing",
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("Compute_Error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		draft := fakeDraft("d1", "t1")
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(draft, nil)
		engine.EXPECT().Compute(ctx, "t1", nil).Return(nil, nil, nil, false, errors.New("compute fail"))

		_, err := svc.PatchSuiteWizard(ctx, &api.PatchSuiteWizardRequest{
			TenantId: "t1",
			DraftId:  "d1",
		})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("Update_Error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		draft := fakeDraft("d1", "t1")
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(draft, nil)
		engine.EXPECT().Compute(ctx, "t1", nil).Return(nil, nil, nil, false, nil)
		drafts.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("db err"))

		_, err := svc.PatchSuiteWizard(ctx, &api.PatchSuiteWizardRequest{
			TenantId: "t1",
			DraftId:  "d1",
		})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("Idempotent_SameFormConverges", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		draft := fakeDraft("d1", "t1")
		// Two identical calls should both succeed.
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(draft, nil).Times(2)
		engine.EXPECT().Compute(ctx, "t1", nil).Return(nil, nil, nil, true, nil).Times(2)
		drafts.EXPECT().Update(ctx, gomock.Any()).Return(nil).Times(2)

		for i := 0; i < 2; i++ {
			_, err := svc.PatchSuiteWizard(ctx, &api.PatchSuiteWizardRequest{
				TenantId: "t1",
				DraftId:  "d1",
			})
			if err != nil {
				t.Fatalf("PatchSuiteWizard call %d: %v", i, err)
			}
		}
	})
}

// ===== DeleteSuiteWizardDraft =====

func TestDeleteSuiteWizardDraft(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		drafts.EXPECT().Delete(ctx, "t1", "d1").Return(nil)

		_, err := svc.DeleteSuiteWizardDraft(ctx, &api.DeleteSuiteWizardDraftRequest{
			TenantId: "t1",
			DraftId:  "d1",
		})
		if err != nil {
			t.Fatalf("DeleteSuiteWizardDraft: %v", err)
		}
	})

	t.Run("NotFound_IsIdempotent", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		// Per proto: "deleting an absent draft is a no-op".
		drafts.EXPECT().Delete(ctx, "t1", "absent").Return(derrors.ErrNotFound)

		_, err := svc.DeleteSuiteWizardDraft(ctx, &api.DeleteSuiteWizardDraftRequest{
			TenantId: "t1",
			DraftId:  "absent",
		})
		if err != nil {
			t.Fatalf("DeleteSuiteWizardDraft idempotent: %v", err)
		}
	})

	t.Run("InternalError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		drafts.EXPECT().Delete(ctx, "t1", "d1").Return(errors.New("db err"))

		_, err := svc.DeleteSuiteWizardDraft(ctx, &api.DeleteSuiteWizardDraftRequest{
			TenantId: "t1",
			DraftId:  "d1",
		})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})
}

// ===== FinishSuiteWizard =====

func TestFinishSuiteWizard(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		draft := fakeDraft("d1", "t1")
		run := &domain.SuiteRun{}
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(draft, nil)
		engine.EXPECT().Compute(ctx, "t1", draft.GetForm()).Return(nil, nil, nil, true, nil)
		engine.EXPECT().Bake(ctx, gomock.Any()).Return(run, nil)

		resp, err := svc.FinishSuiteWizard(ctx, &api.FinishSuiteWizardRequest{
			TenantId: "t1",
			DraftId:  "d1",
		})
		if err != nil {
			t.Fatalf("FinishSuiteWizard: %v", err)
		}
		if resp.SuiteRun == nil {
			t.Fatal("expected SuiteRun in response")
		}
	})

	t.Run("DraftNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		drafts.EXPECT().Get(ctx, "t1", "missing").Return(nil, derrors.ErrNotFound)

		_, err := svc.FinishSuiteWizard(ctx, &api.FinishSuiteWizardRequest{
			TenantId: "t1",
			DraftId:  "missing",
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("NotReady_FailedPrecondition", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		draft := fakeDraft("d1", "t1")
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(draft, nil)
		// Compute returns ready=false => FinishSuiteWizard must reject.
		engine.EXPECT().Compute(ctx, "t1", draft.GetForm()).Return(nil, nil, nil, false, nil)

		_, err := svc.FinishSuiteWizard(ctx, &api.FinishSuiteWizardRequest{
			TenantId: "t1",
			DraftId:  "d1",
		})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", err)
		}
	})

	t.Run("Compute_Error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		draft := fakeDraft("d1", "t1")
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(draft, nil)
		engine.EXPECT().Compute(ctx, "t1", draft.GetForm()).Return(nil, nil, nil, false, errors.New("compute err"))

		_, err := svc.FinishSuiteWizard(ctx, &api.FinishSuiteWizardRequest{
			TenantId: "t1",
			DraftId:  "d1",
		})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("Bake_Error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		draft := fakeDraft("d1", "t1")
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(draft, nil)
		engine.EXPECT().Compute(ctx, "t1", draft.GetForm()).Return(nil, nil, nil, true, nil)
		engine.EXPECT().Bake(ctx, gomock.Any()).Return(nil, errors.New("bake err"))

		_, err := svc.FinishSuiteWizard(ctx, &api.FinishSuiteWizardRequest{
			TenantId: "t1",
			DraftId:  "d1",
		})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("Bake_DomainError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		drafts := NewMockDraftRepo(ctrl)
		suites := NewMockSuiteReader(ctrl)
		engine := NewMockWizardEngine(ctrl)
		svc := newSvc(authn, drafts, suites, engine)

		draft := fakeDraft("d1", "t1")
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(draft, nil)
		engine.EXPECT().Compute(ctx, "t1", draft.GetForm()).Return(nil, nil, nil, true, nil)
		engine.EXPECT().Bake(ctx, gomock.Any()).Return(nil, derrors.FailedPrecondition("BAKE_FAIL", "cannot bake"))

		_, err := svc.FinishSuiteWizard(ctx, &api.FinishSuiteWizardRequest{
			TenantId: "t1",
			DraftId:  "d1",
		})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", err)
		}
	})
}
