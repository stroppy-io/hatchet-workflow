package test_wizard

// service_test.go: unit tests for all 6 TestWizardService RPCs:
// StartTestWizard, GetTestWizardDraft, ListTestWizardDrafts,
// PatchTestWizard, DeleteTestWizardDraft, FinishTestWizard.

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	schemapb "github.com/stroppy-io/schemapb/schemapb"
	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// boolPtr is a helper to get *bool in FinishTestWizardRequest.
func boolPtr(b bool) *bool { return &b }

// newSvc is a helper that builds a TestWizardService with the provided deps.
func newSvc(deps TestWizardDeps) *TestWizardService {
	return NewTestWizardService(deps)
}

// baseDeps returns a minimal deps wiring for tests that don't need all fields.
func baseDeps(ctrl *gomock.Controller) (
	authn *utils.MockAuthn,
	drafts *MockDraftRepo,
	presets *MockPresetReader,
	engine *MockWizardEngine,
	runs *MockTestRunStarter,
	saver *MockPresetSaver,
	svc *TestWizardService,
) {
	authn = utils.NewMockAuthn(ctrl)
	drafts = NewMockDraftRepo(ctrl)
	presets = NewMockPresetReader(ctrl)
	engine = NewMockWizardEngine(ctrl)
	runs = NewMockTestRunStarter(ctrl)
	saver = NewMockPresetSaver(ctrl)
	svc = newSvc(TestWizardDeps{
		Authn:   authn,
		Drafts:  drafts,
		Presets: presets,
		Engine:  engine,
		Runs:    runs,
		Saver:   saver,
		Tx:      &utils.MockTrm{},
	})
	return
}

// ─── StartTestWizard ──────────────────────────────────────────────────────────

func TestStartTestWizard(t *testing.T) {
	ctx := context.Background()

	t.Run("Success_NullPreset", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, drafts, _, engine, _, _, svc := baseDeps(ctrl)

		req := &api.StartTestWizardRequest{TenantId: "t1", Name: "mytest"}
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		form := &schemapb.Filled{}
		engine.EXPECT().InitialForm(ctx, "t1", "mytest", (*models.TestPresetRecord)(nil)).Return(form, nil)
		engine.EXPECT().Compute(ctx, "t1", form).Return(form, nil, nil, true, nil)
		drafts.EXPECT().Create(ctx, gomock.Any()).Return(nil)

		resp, err := svc.StartTestWizard(ctx, req)
		if err != nil {
			t.Fatalf("StartTestWizard: %v", err)
		}
		if resp.Draft == nil {
			t.Error("expected non-nil Draft in response")
		}
		if resp.Draft.Ready != true {
			t.Error("expected draft.Ready=true")
		}
	})

	t.Run("Success_WithPreset", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, drafts, presets, engine, _, _, svc := baseDeps(ctrl)

		req := &api.StartTestWizardRequest{TenantId: "t1", Name: "seeded", TestPresetId: "p1"}
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		preset := &models.TestPresetRecord{}
		presets.EXPECT().Get(ctx, "t1", "p1").Return(preset, nil)
		form := &schemapb.Filled{}
		engine.EXPECT().InitialForm(ctx, "t1", "seeded", preset).Return(form, nil)
		engine.EXPECT().Compute(ctx, "t1", form).Return(form, nil, nil, false, nil)
		drafts.EXPECT().Create(ctx, gomock.Any()).Return(nil)

		resp, err := svc.StartTestWizard(ctx, req)
		if err != nil {
			t.Fatalf("StartTestWizard with preset: %v", err)
		}
		if resp.Draft.TestPresetId != "p1" {
			t.Errorf("expected TestPresetId=p1, got %s", resp.Draft.TestPresetId)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, _, _, _, _, _, svc := baseDeps(ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no claims"))
		_, err := svc.StartTestWizard(ctx, &api.StartTestWizardRequest{TenantId: "t1"})
		if err == nil {
			t.Error("expected error")
		}
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", status.Code(err))
		}
	})

	t.Run("PresetNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, _, presets, _, _, _, svc := baseDeps(ctrl)

		req := &api.StartTestWizardRequest{TenantId: "t1", TestPresetId: "missing"}
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		presets.EXPECT().Get(ctx, "t1", "missing").Return(nil, derrors.ErrNotFound)

		_, err := svc.StartTestWizard(ctx, req)
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", status.Code(err))
		}
	})

	t.Run("PresetGetError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, _, presets, _, _, _, svc := baseDeps(ctrl)

		req := &api.StartTestWizardRequest{TenantId: "t1", TestPresetId: "p1"}
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		presets.EXPECT().Get(ctx, "t1", "p1").Return(nil, errors.New("db err"))

		_, err := svc.StartTestWizard(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("InitialFormError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, _, _, engine, _, _, svc := baseDeps(ctrl)

		req := &api.StartTestWizardRequest{TenantId: "t1", Name: "x"}
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		engine.EXPECT().InitialForm(ctx, "t1", "x", (*models.TestPresetRecord)(nil)).Return(nil, errors.New("engine err"))

		_, err := svc.StartTestWizard(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ComputeError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, _, _, engine, _, _, svc := baseDeps(ctrl)

		req := &api.StartTestWizardRequest{TenantId: "t1", Name: "x"}
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		form := &schemapb.Filled{}
		engine.EXPECT().InitialForm(ctx, "t1", "x", (*models.TestPresetRecord)(nil)).Return(form, nil)
		engine.EXPECT().Compute(ctx, "t1", form).Return(nil, nil, nil, false, errors.New("compute err"))

		_, err := svc.StartTestWizard(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("DraftCreateError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, drafts, _, engine, _, _, svc := baseDeps(ctrl)

		req := &api.StartTestWizardRequest{TenantId: "t1", Name: "x"}
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		form := &schemapb.Filled{}
		engine.EXPECT().InitialForm(ctx, "t1", "x", (*models.TestPresetRecord)(nil)).Return(form, nil)
		engine.EXPECT().Compute(ctx, "t1", form).Return(form, nil, nil, true, nil)
		drafts.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db err"))

		_, err := svc.StartTestWizard(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("DraftCreateConflict", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, drafts, _, engine, _, _, svc := baseDeps(ctrl)

		req := &api.StartTestWizardRequest{TenantId: "t1", Name: "x"}
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		form := &schemapb.Filled{}
		engine.EXPECT().InitialForm(ctx, "t1", "x", (*models.TestPresetRecord)(nil)).Return(form, nil)
		engine.EXPECT().Compute(ctx, "t1", form).Return(form, nil, nil, true, nil)
		drafts.EXPECT().Create(ctx, gomock.Any()).Return(derrors.ErrConflict)

		_, err := svc.StartTestWizard(ctx, req)
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", status.Code(err))
		}
	})
}

// ─── GetTestWizardDraft ───────────────────────────────────────────────────────

func TestGetTestWizardDraft(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		_, drafts, _, _, _, _, svc := baseDeps(ctrl)

		draftRecord := &models.TestWizardDraftRecord{
			Entity: &common.Entity{Id: "d1", TenantId: "t1"},
		}
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(draftRecord, nil)

		resp, err := svc.GetTestWizardDraft(ctx, &api.GetTestWizardDraftRequest{TenantId: "t1", DraftId: "d1"})
		if err != nil {
			t.Fatalf("GetTestWizardDraft: %v", err)
		}
		if resp.Draft.Entity.Id != "d1" {
			t.Errorf("expected draft id d1, got %s", resp.Draft.Entity.Id)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		_, drafts, _, _, _, _, svc := baseDeps(ctrl)

		drafts.EXPECT().Get(ctx, "t1", "missing").Return(nil, derrors.ErrNotFound)

		_, err := svc.GetTestWizardDraft(ctx, &api.GetTestWizardDraftRequest{TenantId: "t1", DraftId: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", status.Code(err))
		}
	})

	t.Run("RepoError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		_, drafts, _, _, _, _, svc := baseDeps(ctrl)

		drafts.EXPECT().Get(ctx, "t1", "d1").Return(nil, errors.New("db error"))

		_, err := svc.GetTestWizardDraft(ctx, &api.GetTestWizardDraftRequest{TenantId: "t1", DraftId: "d1"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// ─── ListTestWizardDrafts ─────────────────────────────────────────────────────

func TestListTestWizardDrafts(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		_, drafts, _, _, _, _, svc := baseDeps(ctrl)

		draftList := []*models.TestWizardDraftRecord{
			{Entity: &common.Entity{Id: "d1"}},
			{Entity: &common.Entity{Id: "d2"}},
		}
		drafts.EXPECT().List(ctx, "t1", nil, nil, nil).Return(draftList, "nextTok", nil)

		resp, err := svc.ListTestWizardDrafts(ctx, &api.ListTestWizardDraftsRequest{TenantId: "t1"})
		if err != nil {
			t.Fatalf("ListTestWizardDrafts: %v", err)
		}
		if len(resp.Drafts) != 2 {
			t.Errorf("expected 2 drafts, got %d", len(resp.Drafts))
		}
		if resp.NextPageToken != "nextTok" {
			t.Errorf("expected nextTok, got %s", resp.NextPageToken)
		}
	})

	t.Run("EmptyList", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		_, drafts, _, _, _, _, svc := baseDeps(ctrl)

		drafts.EXPECT().List(ctx, "t1", nil, nil, nil).Return(nil, "", nil)

		resp, err := svc.ListTestWizardDrafts(ctx, &api.ListTestWizardDraftsRequest{TenantId: "t1"})
		if err != nil {
			t.Fatalf("ListTestWizardDrafts empty: %v", err)
		}
		if len(resp.Drafts) != 0 {
			t.Errorf("expected 0 drafts, got %d", len(resp.Drafts))
		}
	})

	t.Run("RepoError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		_, drafts, _, _, _, _, svc := baseDeps(ctrl)

		drafts.EXPECT().List(ctx, "t1", nil, nil, nil).Return(nil, "", errors.New("db err"))

		_, err := svc.ListTestWizardDrafts(ctx, &api.ListTestWizardDraftsRequest{TenantId: "t1"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// ─── PatchTestWizard ──────────────────────────────────────────────────────────

func TestPatchTestWizard(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		_, drafts, _, engine, _, _, svc := baseDeps(ctrl)

		draftRecord := &models.TestWizardDraftRecord{
			Entity: &common.Entity{Id: "d1", TenantId: "t1"},
		}
		newForm := &schemapb.Filled{}
		req := &api.PatchTestWizardRequest{TenantId: "t1", DraftId: "d1", Form: newForm}

		drafts.EXPECT().Get(ctx, "t1", "d1").Return(draftRecord, nil)
		engine.EXPECT().Compute(ctx, "t1", newForm).Return(newForm, nil, nil, true, nil)
		drafts.EXPECT().Update(ctx, gomock.Any()).Return(nil)

		resp, err := svc.PatchTestWizard(ctx, req)
		if err != nil {
			t.Fatalf("PatchTestWizard: %v", err)
		}
		if !resp.Draft.Ready {
			t.Error("expected draft.Ready=true after patch")
		}
	})

	t.Run("DraftNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		_, drafts, _, _, _, _, svc := baseDeps(ctrl)

		drafts.EXPECT().Get(ctx, "t1", "missing").Return(nil, derrors.ErrNotFound)

		_, err := svc.PatchTestWizard(ctx, &api.PatchTestWizardRequest{TenantId: "t1", DraftId: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", status.Code(err))
		}
	})

	t.Run("ComputeError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		_, drafts, _, engine, _, _, svc := baseDeps(ctrl)

		draftRecord := &models.TestWizardDraftRecord{
			Entity: &common.Entity{Id: "d1", TenantId: "t1"},
		}
		newForm := &schemapb.Filled{}
		req := &api.PatchTestWizardRequest{TenantId: "t1", DraftId: "d1", Form: newForm}

		drafts.EXPECT().Get(ctx, "t1", "d1").Return(draftRecord, nil)
		engine.EXPECT().Compute(ctx, "t1", newForm).Return(nil, nil, nil, false, errors.New("engine err"))

		_, err := svc.PatchTestWizard(ctx, req)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("UpdateError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		_, drafts, _, engine, _, _, svc := baseDeps(ctrl)

		draftRecord := &models.TestWizardDraftRecord{
			Entity: &common.Entity{Id: "d1", TenantId: "t1"},
		}
		newForm := &schemapb.Filled{}
		req := &api.PatchTestWizardRequest{TenantId: "t1", DraftId: "d1", Form: newForm}

		drafts.EXPECT().Get(ctx, "t1", "d1").Return(draftRecord, nil)
		engine.EXPECT().Compute(ctx, "t1", newForm).Return(newForm, nil, nil, true, nil)
		drafts.EXPECT().Update(ctx, gomock.Any()).Return(derrors.ErrNotFound)

		_, err := svc.PatchTestWizard(ctx, req)
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound from Update, got %v", status.Code(err))
		}
	})

	t.Run("WithValidationErrors_NotReady", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		_, drafts, _, engine, _, _, svc := baseDeps(ctrl)

		draftRecord := &models.TestWizardDraftRecord{
			Entity: &common.Entity{Id: "d1", TenantId: "t1"},
		}
		newForm := &schemapb.Filled{}
		fieldErrs := []*schemapb.FieldError{{Field: "db.host", Message: "required"}}
		req := &api.PatchTestWizardRequest{TenantId: "t1", DraftId: "d1", Form: newForm}

		drafts.EXPECT().Get(ctx, "t1", "d1").Return(draftRecord, nil)
		engine.EXPECT().Compute(ctx, "t1", newForm).Return(newForm, nil, fieldErrs, false, nil)
		drafts.EXPECT().Update(ctx, gomock.Any()).Return(nil)

		resp, err := svc.PatchTestWizard(ctx, req)
		if err != nil {
			t.Fatalf("PatchTestWizard with field errors: %v", err)
		}
		if resp.Draft.Ready {
			t.Error("expected draft.Ready=false when there are field errors")
		}
		if len(resp.Draft.Errors) == 0 {
			t.Error("expected field errors to be set")
		}
	})
}

// ─── DeleteTestWizardDraft ────────────────────────────────────────────────────

func TestDeleteTestWizardDraft(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		_, drafts, _, _, _, _, svc := baseDeps(ctrl)

		drafts.EXPECT().Delete(ctx, "t1", "d1").Return(nil)

		_, err := svc.DeleteTestWizardDraft(ctx, &api.DeleteTestWizardDraftRequest{TenantId: "t1", DraftId: "d1"})
		if err != nil {
			t.Fatalf("DeleteTestWizardDraft: %v", err)
		}
	})

	t.Run("NotFound_IsIdempotent", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		_, drafts, _, _, _, _, svc := baseDeps(ctrl)

		// Not-found is swallowed per the documented idempotency
		drafts.EXPECT().Delete(ctx, "t1", "absent").Return(derrors.ErrNotFound)

		_, err := svc.DeleteTestWizardDraft(ctx, &api.DeleteTestWizardDraftRequest{TenantId: "t1", DraftId: "absent"})
		if err != nil {
			t.Fatalf("DeleteTestWizardDraft idempotent: %v", err)
		}
	})

	t.Run("RepoError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		_, drafts, _, _, _, _, svc := baseDeps(ctrl)

		drafts.EXPECT().Delete(ctx, "t1", "d1").Return(errors.New("db err"))

		_, err := svc.DeleteTestWizardDraft(ctx, &api.DeleteTestWizardDraftRequest{TenantId: "t1", DraftId: "d1"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// ─── FinishTestWizard ─────────────────────────────────────────────────────────

func TestFinishTestWizard(t *testing.T) {
	ctx := context.Background()

	// Build a basic ready draft record for reuse across subtests.
	readyDraft := func() *models.TestWizardDraftRecord {
		return &models.TestWizardDraftRecord{
			Entity: &common.Entity{Id: "d1", TenantId: "t1", Name: "myrun"},
			Form:   &schemapb.Filled{},
			Ready:  true,
		}
	}
	bakedRun := &domain.TestRun{}

	t.Run("Success_JustBake", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, drafts, _, engine, _, _, svc := baseDeps(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(readyDraft(), nil)
		engine.EXPECT().Compute(ctx, "t1", gomock.Any()).Return(&schemapb.Filled{}, nil, nil, true, nil)
		engine.EXPECT().Bake(ctx, gomock.Any()).Return(bakedRun, nil)
		drafts.EXPECT().Delete(ctx, "t1", "d1").Return(nil)

		resp, err := svc.FinishTestWizard(ctx, &api.FinishTestWizardRequest{TenantId: "t1", DraftId: "d1"})
		if err != nil {
			t.Fatalf("FinishTestWizard: %v", err)
		}
		if resp.TestRun == nil {
			t.Error("expected TestRun in response")
		}
		if resp.Run != nil {
			t.Error("expected Run=nil when Start=false")
		}
		if resp.Preset != nil {
			t.Error("expected Preset=nil when SaveAsPreset=false")
		}
	})

	t.Run("Success_WithStart", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, drafts, _, engine, runs, _, svc := baseDeps(ctrl)

		runRecord := &models.TestRunRecord{}
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(readyDraft(), nil)
		engine.EXPECT().Compute(ctx, "t1", gomock.Any()).Return(&schemapb.Filled{}, nil, nil, true, nil)
		engine.EXPECT().Bake(ctx, gomock.Any()).Return(bakedRun, nil)
		// Default rating: inTenant=true, inGlobal=false (unset fields)
		runs.EXPECT().Start(ctx, "t1", bakedRun, common.Trigger_TRIGGER_MANUAL, true, false).Return(runRecord, nil)
		drafts.EXPECT().Delete(ctx, "t1", "d1").Return(nil)

		resp, err := svc.FinishTestWizard(ctx, &api.FinishTestWizardRequest{
			TenantId: "t1", DraftId: "d1", Start: true,
		})
		if err != nil {
			t.Fatalf("FinishTestWizard with start: %v", err)
		}
		if resp.Run == nil {
			t.Error("expected Run in response")
		}
	})

	t.Run("Success_WithSaveAsPreset", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, drafts, _, engine, _, saver, svc := baseDeps(ctrl)

		presetRecord := &models.TestPresetRecord{}
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(readyDraft(), nil)
		engine.EXPECT().Compute(ctx, "t1", gomock.Any()).Return(&schemapb.Filled{}, nil, nil, true, nil)
		engine.EXPECT().Bake(ctx, gomock.Any()).Return(bakedRun, nil)
		// SaveAsPreset=true, PresetName="" -> uses draft.Entity.Name
		saver.EXPECT().Save(ctx, "t1", "acc1", "myrun", bakedRun).Return(presetRecord, nil)
		drafts.EXPECT().Delete(ctx, "t1", "d1").Return(nil)

		resp, err := svc.FinishTestWizard(ctx, &api.FinishTestWizardRequest{
			TenantId: "t1", DraftId: "d1", SaveAsPreset: true,
		})
		if err != nil {
			t.Fatalf("FinishTestWizard with preset: %v", err)
		}
		if resp.Preset == nil {
			t.Error("expected Preset in response")
		}
	})

	t.Run("Success_WithSaveAsPreset_ExplicitName", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, drafts, _, engine, _, saver, svc := baseDeps(ctrl)

		presetRecord := &models.TestPresetRecord{}
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(readyDraft(), nil)
		engine.EXPECT().Compute(ctx, "t1", gomock.Any()).Return(&schemapb.Filled{}, nil, nil, true, nil)
		engine.EXPECT().Bake(ctx, gomock.Any()).Return(bakedRun, nil)
		saver.EXPECT().Save(ctx, "t1", "acc1", "explicit-name", bakedRun).Return(presetRecord, nil)
		drafts.EXPECT().Delete(ctx, "t1", "d1").Return(nil)

		resp, err := svc.FinishTestWizard(ctx, &api.FinishTestWizardRequest{
			TenantId: "t1", DraftId: "d1", SaveAsPreset: true, PresetName: "explicit-name",
		})
		if err != nil {
			t.Fatalf("FinishTestWizard with explicit preset name: %v", err)
		}
		if resp.Preset == nil {
			t.Error("expected Preset in response")
		}
	})

	t.Run("Success_StartAndSaveAsPreset", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, drafts, _, engine, runs, saver, svc := baseDeps(ctrl)

		presetRecord := &models.TestPresetRecord{}
		runRecord := &models.TestRunRecord{}
		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(readyDraft(), nil)
		engine.EXPECT().Compute(ctx, "t1", gomock.Any()).Return(&schemapb.Filled{}, nil, nil, true, nil)
		engine.EXPECT().Bake(ctx, gomock.Any()).Return(bakedRun, nil)
		saver.EXPECT().Save(ctx, "t1", "acc1", "myrun", bakedRun).Return(presetRecord, nil)
		runs.EXPECT().Start(ctx, "t1", bakedRun, common.Trigger_TRIGGER_MANUAL, true, false).Return(runRecord, nil)
		drafts.EXPECT().Delete(ctx, "t1", "d1").Return(nil)

		resp, err := svc.FinishTestWizard(ctx, &api.FinishTestWizardRequest{
			TenantId: "t1", DraftId: "d1", SaveAsPreset: true, Start: true,
		})
		if err != nil {
			t.Fatalf("FinishTestWizard start+preset: %v", err)
		}
		if resp.Preset == nil || resp.Run == nil {
			t.Error("expected both Preset and Run in response")
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, _, _, _, _, _, svc := baseDeps(ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no claims"))
		_, err := svc.FinishTestWizard(ctx, &api.FinishTestWizardRequest{TenantId: "t1", DraftId: "d1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", status.Code(err))
		}
	})

	t.Run("DraftNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, drafts, _, _, _, _, svc := baseDeps(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		drafts.EXPECT().Get(ctx, "t1", "missing").Return(nil, derrors.ErrNotFound)

		_, err := svc.FinishTestWizard(ctx, &api.FinishTestWizardRequest{TenantId: "t1", DraftId: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", status.Code(err))
		}
	})

	t.Run("NotReady_FailedPrecondition", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, drafts, _, engine, _, _, svc := baseDeps(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(readyDraft(), nil)
		// Engine re-validates and finds it not ready
		engine.EXPECT().Compute(ctx, "t1", gomock.Any()).Return(&schemapb.Filled{}, nil,
			[]*schemapb.FieldError{{Field: "f", Message: "required"}}, false, nil)

		_, err := svc.FinishTestWizard(ctx, &api.FinishTestWizardRequest{TenantId: "t1", DraftId: "d1"})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", status.Code(err))
		}
	})

	t.Run("ComputeError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, drafts, _, engine, _, _, svc := baseDeps(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(readyDraft(), nil)
		engine.EXPECT().Compute(ctx, "t1", gomock.Any()).Return(nil, nil, nil, false, errors.New("engine err"))

		_, err := svc.FinishTestWizard(ctx, &api.FinishTestWizardRequest{TenantId: "t1", DraftId: "d1"})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("BakeError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, drafts, _, engine, _, _, svc := baseDeps(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(readyDraft(), nil)
		engine.EXPECT().Compute(ctx, "t1", gomock.Any()).Return(&schemapb.Filled{}, nil, nil, true, nil)
		engine.EXPECT().Bake(ctx, gomock.Any()).Return(nil, errors.New("bake err"))

		_, err := svc.FinishTestWizard(ctx, &api.FinishTestWizardRequest{TenantId: "t1", DraftId: "d1"})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("SavePresetError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, drafts, _, engine, _, saver, svc := baseDeps(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(readyDraft(), nil)
		engine.EXPECT().Compute(ctx, "t1", gomock.Any()).Return(&schemapb.Filled{}, nil, nil, true, nil)
		engine.EXPECT().Bake(ctx, gomock.Any()).Return(bakedRun, nil)
		saver.EXPECT().Save(ctx, "t1", "acc1", "myrun", bakedRun).Return(nil, errors.New("save err"))

		_, err := svc.FinishTestWizard(ctx, &api.FinishTestWizardRequest{
			TenantId: "t1", DraftId: "d1", SaveAsPreset: true,
		})
		if err == nil {
			t.Error("expected error from save preset")
		}
	})

	t.Run("StartRunError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, drafts, _, engine, runs, _, svc := baseDeps(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(readyDraft(), nil)
		engine.EXPECT().Compute(ctx, "t1", gomock.Any()).Return(&schemapb.Filled{}, nil, nil, true, nil)
		engine.EXPECT().Bake(ctx, gomock.Any()).Return(bakedRun, nil)
		runs.EXPECT().Start(ctx, "t1", bakedRun, common.Trigger_TRIGGER_MANUAL, true, false).Return(nil, errors.New("start err"))

		_, err := svc.FinishTestWizard(ctx, &api.FinishTestWizardRequest{
			TenantId: "t1", DraftId: "d1", Start: true,
		})
		if err == nil {
			t.Error("expected error from start run")
		}
	})

	t.Run("DeleteDraftError_NonNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, drafts, _, engine, _, _, svc := baseDeps(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(readyDraft(), nil)
		engine.EXPECT().Compute(ctx, "t1", gomock.Any()).Return(&schemapb.Filled{}, nil, nil, true, nil)
		engine.EXPECT().Bake(ctx, gomock.Any()).Return(bakedRun, nil)
		drafts.EXPECT().Delete(ctx, "t1", "d1").Return(errors.New("db err"))

		_, err := svc.FinishTestWizard(ctx, &api.FinishTestWizardRequest{TenantId: "t1", DraftId: "d1"})
		if err == nil {
			t.Error("expected error from draft delete")
		}
	})

	t.Run("DeleteDraftNotFound_IsIdempotent", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn, drafts, _, engine, _, _, svc := baseDeps(ctrl)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		drafts.EXPECT().Get(ctx, "t1", "d1").Return(readyDraft(), nil)
		engine.EXPECT().Compute(ctx, "t1", gomock.Any()).Return(&schemapb.Filled{}, nil, nil, true, nil)
		engine.EXPECT().Bake(ctx, gomock.Any()).Return(bakedRun, nil)
		// Not-found during draft cleanup is idempotent (swallowed by IgnoreNotFound)
		drafts.EXPECT().Delete(ctx, "t1", "d1").Return(derrors.ErrNotFound)

		resp, err := svc.FinishTestWizard(ctx, &api.FinishTestWizardRequest{TenantId: "t1", DraftId: "d1"})
		if err != nil {
			t.Fatalf("expected no error when draft already absent: %v", err)
		}
		if resp.TestRun == nil {
			t.Error("expected TestRun in response")
		}
	})
}

// ─── resolveRating ────────────────────────────────────────────────────────────

func TestResolveRating(t *testing.T) {
	t.Run("Defaults_UnsetFields", func(t *testing.T) {
		req := &api.FinishTestWizardRequest{}
		inTenant, inGlobal := resolveRating(req)
		if !inTenant {
			t.Error("expected inTenant=true by default")
		}
		if inGlobal {
			t.Error("expected inGlobal=false by default")
		}
	})

	t.Run("ExplicitFalse_InTenant", func(t *testing.T) {
		req := &api.FinishTestWizardRequest{InTenantRating: boolPtr(false)}
		inTenant, inGlobal := resolveRating(req)
		if inTenant {
			t.Error("expected inTenant=false")
		}
		if inGlobal {
			t.Error("expected inGlobal=false")
		}
	})

	t.Run("ExplicitTrue_InGlobal", func(t *testing.T) {
		req := &api.FinishTestWizardRequest{InGlobalRating: boolPtr(true)}
		inTenant, inGlobal := resolveRating(req)
		if !inTenant {
			t.Error("expected inTenant=true (default)")
		}
		if !inGlobal {
			t.Error("expected inGlobal=true")
		}
	})

	t.Run("BothExplicit", func(t *testing.T) {
		req := &api.FinishTestWizardRequest{InTenantRating: boolPtr(false), InGlobalRating: boolPtr(true)}
		inTenant, inGlobal := resolveRating(req)
		if inTenant {
			t.Error("expected inTenant=false")
		}
		if !inGlobal {
			t.Error("expected inGlobal=true")
		}
	})
}

// ─── touch helper ─────────────────────────────────────────────────────────────

func TestTouch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	_, _, _, _, _, _, svc := baseDeps(ctrl)

	t.Run("NilEntity_NoPanic", func(t *testing.T) {
		draft := &models.TestWizardDraftRecord{}
		// Should not panic when Entity is nil
		svc.touch(draft)
	})

	t.Run("NilTimings_Allocated", func(t *testing.T) {
		draft := &models.TestWizardDraftRecord{
			Entity: &common.Entity{},
		}
		svc.touch(draft)
		if draft.Entity.Timings == nil {
			t.Error("expected Timings to be allocated")
		}
		if draft.Entity.Timings.UpdatedAt == nil {
			t.Error("expected UpdatedAt to be set")
		}
	})

	t.Run("ExistingTimings_Updated", func(t *testing.T) {
		draft := &models.TestWizardDraftRecord{
			Entity: &common.Entity{
				Timings: &common.Timings{},
			},
		}
		svc.touch(draft)
		if draft.Entity.Timings.UpdatedAt == nil {
			t.Error("expected UpdatedAt to be set")
		}
	})
}
