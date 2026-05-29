package suite

// suite_test.go: unit tests for CreateSuite, GetSuite, ListSuites, UpdateSuite,
// DeleteSuite, CloneSuite, SetSuiteSchedule, StartSuite RPCs.

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

// helper builds a SuiteService with all mocks wired.
func newSvc(
	ctrl *gomock.Controller,
	authn *utils.MockAuthn,
	repo *MockSuiteRepo,
	launcher *MockSuiteRunLauncher,
) *SuiteService {
	return NewSuiteService(SuiteDeps{
		Authn:    authn,
		Suites:   repo,
		Launcher: launcher,
		Tx:       &utils.MockTrm{},
	})
}

func stubCaller(authn *utils.MockAuthn, ctx context.Context, accountID string) {
	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: accountID}, nil)
}

func stubCallerErr(authn *utils.MockAuthn, ctx context.Context) {
	authn.EXPECT().Caller(ctx).Return(nil, errors.New("unauthenticated"))
}

// ─────────────────────────── CreateSuite ────────────────────────────────────

func TestCreateSuite(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		stubCaller(authn, ctx, "account-1")
		repo.EXPECT().Create(ctx, gomock.Any()).Return(nil)

		resp, err := svc.CreateSuite(ctx, &api.CreateSuiteRequest{
			TenantId: "tenant-1",
			Suite: &models.SuiteRecord{
				Entity: &common.Entity{Name: "my suite"},
				Spec:   &domain.Suite{},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetSuite().GetEntity().GetTenantId() != "tenant-1" {
			t.Errorf("expected tenant-1, got %s", resp.GetSuite().GetEntity().GetTenantId())
		}
		if resp.GetSuite().GetEntity().GetAuthorId() != "account-1" {
			t.Errorf("expected author account-1, got %s", resp.GetSuite().GetEntity().GetAuthorId())
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		stubCallerErr(authn, ctx)
		_, err := svc.CreateSuite(ctx, &api.CreateSuiteRequest{
			TenantId: "tenant-1",
			Suite:    &models.SuiteRecord{Spec: &domain.Suite{}},
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("NilSuite_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		stubCaller(authn, ctx, "account-1")
		_, err := svc.CreateSuite(ctx, &api.CreateSuiteRequest{TenantId: "tenant-1"})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument for nil suite, got %v", err)
		}
	})

	t.Run("NilSpec_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		stubCaller(authn, ctx, "account-1")
		_, err := svc.CreateSuite(ctx, &api.CreateSuiteRequest{
			TenantId: "tenant-1",
			Suite:    &models.SuiteRecord{Entity: &common.Entity{Name: "s"}},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument for nil spec, got %v", err)
		}
	})

	t.Run("RepoConflict", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		stubCaller(authn, ctx, "account-1")
		repo.EXPECT().Create(ctx, gomock.Any()).Return(derrors.ErrConflict)
		_, err := svc.CreateSuite(ctx, &api.CreateSuiteRequest{
			TenantId: "tenant-1",
			Suite:    &models.SuiteRecord{Entity: &common.Entity{}, Spec: &domain.Suite{}},
		})
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", err)
		}
	})

	t.Run("RepoError_Internal", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		stubCaller(authn, ctx, "account-1")
		repo.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db error"))
		_, err := svc.CreateSuite(ctx, &api.CreateSuiteRequest{
			TenantId: "tenant-1",
			Suite:    &models.SuiteRecord{Entity: &common.Entity{}, Spec: &domain.Suite{}},
		})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("ScheduleMirroredInSummary", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		stubCaller(authn, ctx, "account-1")
		repo.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, rec *models.SuiteRecord) error {
			if rec.GetSummary() == nil {
				t.Error("expected summary to be set")
			}
			return nil
		})
		_, err := svc.CreateSuite(ctx, &api.CreateSuiteRequest{
			TenantId: "tenant-1",
			Suite: &models.SuiteRecord{
				Entity: &common.Entity{Name: "s"},
				Spec: &domain.Suite{
					Schedule: &domain.Schedule{Enabled: true, Cron: "0 * * * *"},
				},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// ─────────────────────────── GetSuite ───────────────────────────────────────

func TestGetSuite(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		rec := &models.SuiteRecord{Entity: &common.Entity{Id: "suite-1", TenantId: "tenant-1"}}
		repo.EXPECT().Get(ctx, "tenant-1", "suite-1").Return(rec, nil)

		resp, err := svc.GetSuite(ctx, &api.GetSuiteRequest{TenantId: "tenant-1", Id: "suite-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetSuite().GetEntity().GetId() != "suite-1" {
			t.Errorf("expected suite-1, got %s", resp.GetSuite().GetEntity().GetId())
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		repo.EXPECT().Get(ctx, "tenant-1", "missing").Return(nil, derrors.ErrNotFound)
		_, err := svc.GetSuite(ctx, &api.GetSuiteRequest{TenantId: "tenant-1", Id: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("RepoError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		repo.EXPECT().Get(ctx, "tenant-1", "suite-1").Return(nil, errors.New("db error"))
		_, err := svc.GetSuite(ctx, &api.GetSuiteRequest{TenantId: "tenant-1", Id: "suite-1"})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})
}

// ─────────────────────────── ListSuites ─────────────────────────────────────

func TestListSuites(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		stubCaller(authn, ctx, "account-1")
		suites := []*models.SuiteRecord{
			{Entity: &common.Entity{Id: "s1"}},
			{Entity: &common.Entity{Id: "s2"}},
		}
		repo.EXPECT().List(ctx, gomock.Any()).Return(suites, "next-token", nil)

		resp, err := svc.ListSuites(ctx, &api.ListSuitesRequest{TenantId: "tenant-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.GetSuites()) != 2 {
			t.Errorf("expected 2 suites, got %d", len(resp.GetSuites()))
		}
		if resp.GetNextPageToken() != "next-token" {
			t.Errorf("expected next-token, got %s", resp.GetNextPageToken())
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		stubCallerErr(authn, ctx)
		_, err := svc.ListSuites(ctx, &api.ListSuitesRequest{TenantId: "tenant-1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("RepoError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		stubCaller(authn, ctx, "account-1")
		repo.EXPECT().List(ctx, gomock.Any()).Return(nil, "", errors.New("db error"))
		_, err := svc.ListSuites(ctx, &api.ListSuitesRequest{TenantId: "tenant-1"})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("ScheduleEnabledFilter", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		stubCaller(authn, ctx, "account-1")
		repo.EXPECT().List(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, q SuiteListQuery) ([]*models.SuiteRecord, string, error) {
			if q.ScheduleEnabled == nil || !*q.ScheduleEnabled {
				t.Error("expected ScheduleEnabled=true in query")
			}
			return nil, "", nil
		})

		enabled := true
		resp, err := svc.ListSuites(ctx, &api.ListSuitesRequest{
			TenantId:        "tenant-1",
			ScheduleEnabled: &enabled,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = resp
	})
}

// ─────────────────────────── UpdateSuite ────────────────────────────────────

func TestUpdateSuite(t *testing.T) {
	ctx := context.Background()

	existingRec := &models.SuiteRecord{
		Entity: &common.Entity{
			Id:       "suite-1",
			TenantId: "tenant-1",
			AuthorId: "original-author",
			Timings:  &common.Timings{},
		},
		Spec: &domain.Suite{Id: "suite-1"},
	}

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		repo.EXPECT().Get(ctx, "tenant-1", "suite-1").Return(existingRec, nil)
		repo.EXPECT().Update(ctx, gomock.Any()).Return(nil)

		resp, err := svc.UpdateSuite(ctx, &api.UpdateSuiteRequest{
			TenantId: "tenant-1",
			Suite: &models.SuiteRecord{
				Entity: &common.Entity{Id: "suite-1", Name: "new name"},
				Spec:   &domain.Suite{},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// immutable fields preserved
		if resp.GetSuite().GetEntity().GetAuthorId() != "original-author" {
			t.Errorf("author should be preserved, got %s", resp.GetSuite().GetEntity().GetAuthorId())
		}
	})

	t.Run("NilSuite_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		_, err := svc.UpdateSuite(ctx, &api.UpdateSuiteRequest{TenantId: "tenant-1"})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("MissingEntityId_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		_, err := svc.UpdateSuite(ctx, &api.UpdateSuiteRequest{
			TenantId: "tenant-1",
			Suite:    &models.SuiteRecord{Entity: &common.Entity{}, Spec: &domain.Suite{}},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument for missing entity.id, got %v", err)
		}
	})

	t.Run("GetNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		repo.EXPECT().Get(ctx, "tenant-1", "missing").Return(nil, derrors.ErrNotFound)
		_, err := svc.UpdateSuite(ctx, &api.UpdateSuiteRequest{
			TenantId: "tenant-1",
			Suite: &models.SuiteRecord{
				Entity: &common.Entity{Id: "missing"},
				Spec:   &domain.Suite{},
			},
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("UpdateRepoError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		repo.EXPECT().Get(ctx, "tenant-1", "suite-1").Return(existingRec, nil)
		repo.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("db error"))

		_, err := svc.UpdateSuite(ctx, &api.UpdateSuiteRequest{
			TenantId: "tenant-1",
			Suite: &models.SuiteRecord{
				Entity: &common.Entity{Id: "suite-1"},
				Spec:   &domain.Suite{},
			},
		})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})
}

// ─────────────────────────── DeleteSuite ────────────────────────────────────

func TestDeleteSuite(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		repo.EXPECT().Delete(ctx, "tenant-1", "suite-1").Return(nil)
		_, err := svc.DeleteSuite(ctx, &api.DeleteSuiteRequest{TenantId: "tenant-1", Id: "suite-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Idempotent_NotFound_IsNoOp", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		repo.EXPECT().Delete(ctx, "tenant-1", "absent").Return(derrors.ErrNotFound)
		_, err := svc.DeleteSuite(ctx, &api.DeleteSuiteRequest{TenantId: "tenant-1", Id: "absent"})
		if err != nil {
			t.Fatalf("expected no error for not-found delete, got %v", err)
		}
	})

	t.Run("RepoError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		repo.EXPECT().Delete(ctx, "tenant-1", "suite-1").Return(errors.New("db error"))
		_, err := svc.DeleteSuite(ctx, &api.DeleteSuiteRequest{TenantId: "tenant-1", Id: "suite-1"})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})
}

// ─────────────────────────── CloneSuite ─────────────────────────────────────

func TestCloneSuite(t *testing.T) {
	ctx := context.Background()

	srcRec := &models.SuiteRecord{
		Entity: &common.Entity{
			Id:       "suite-src",
			TenantId: "tenant-1",
			Name:     "original name",
			AuthorId: "author-1",
			Timings:  &common.Timings{},
		},
		Spec: &domain.Suite{Id: "suite-src"},
	}

	t.Run("Success_DefaultName", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		stubCaller(authn, ctx, "cloner")
		repo.EXPECT().Get(ctx, "tenant-1", "suite-src").Return(srcRec, nil)
		repo.EXPECT().Create(ctx, gomock.Any()).Return(nil)

		resp, err := svc.CloneSuite(ctx, &api.CloneSuiteRequest{
			TenantId: "tenant-1",
			Id:       "suite-src",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetSuite().GetEntity().GetAuthorId() != "cloner" {
			t.Errorf("expected clone author=cloner, got %s", resp.GetSuite().GetEntity().GetAuthorId())
		}
		if resp.GetSuite().GetEntity().GetName() != "original name" {
			t.Errorf("expected original name preserved, got %s", resp.GetSuite().GetEntity().GetName())
		}
		if resp.GetSuite().GetEntity().GetId() == "suite-src" {
			t.Error("clone should have a fresh ID")
		}
	})

	t.Run("Success_CustomName", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		stubCaller(authn, ctx, "cloner")
		repo.EXPECT().Get(ctx, "tenant-1", "suite-src").Return(srcRec, nil)
		repo.EXPECT().Create(ctx, gomock.Any()).Return(nil)

		resp, err := svc.CloneSuite(ctx, &api.CloneSuiteRequest{
			TenantId: "tenant-1",
			Id:       "suite-src",
			Name:     "clone name",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetSuite().GetEntity().GetName() != "clone name" {
			t.Errorf("expected custom name, got %s", resp.GetSuite().GetEntity().GetName())
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		stubCallerErr(authn, ctx)
		_, err := svc.CloneSuite(ctx, &api.CloneSuiteRequest{TenantId: "tenant-1", Id: "suite-src"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("SourceNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		stubCaller(authn, ctx, "cloner")
		repo.EXPECT().Get(ctx, "tenant-1", "missing").Return(nil, derrors.ErrNotFound)

		_, err := svc.CloneSuite(ctx, &api.CloneSuiteRequest{TenantId: "tenant-1", Id: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("CreateConflict", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		stubCaller(authn, ctx, "cloner")
		repo.EXPECT().Get(ctx, "tenant-1", "suite-src").Return(srcRec, nil)
		repo.EXPECT().Create(ctx, gomock.Any()).Return(derrors.ErrConflict)

		_, err := svc.CloneSuite(ctx, &api.CloneSuiteRequest{TenantId: "tenant-1", Id: "suite-src"})
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", err)
		}
	})

	t.Run("SourceHasNilSpec_SpecInitialized", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		// source record with no Spec — clone should initialize one
		noSpecSrc := &models.SuiteRecord{
			Entity: &common.Entity{
				Id:       "suite-nspec",
				TenantId: "tenant-1",
				Name:     "no-spec suite",
				AuthorId: "author-1",
				Timings:  &common.Timings{},
			},
		}
		stubCaller(authn, ctx, "cloner")
		repo.EXPECT().Get(ctx, "tenant-1", "suite-nspec").Return(noSpecSrc, nil)
		repo.EXPECT().Create(ctx, gomock.Any()).Return(nil)

		resp, err := svc.CloneSuite(ctx, &api.CloneSuiteRequest{
			TenantId: "tenant-1",
			Id:       "suite-nspec",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetSuite().GetSpec() == nil {
			t.Error("expected spec initialized in clone")
		}
	})
}

// ─────────────────────────── SetSuiteSchedule ───────────────────────────────

func TestSetSuiteSchedule(t *testing.T) {
	ctx := context.Background()

	existingRec := &models.SuiteRecord{
		Entity: &common.Entity{
			Id:       "suite-1",
			TenantId: "tenant-1",
			Timings:  &common.Timings{},
		},
		Spec: &domain.Suite{Id: "suite-1"},
	}

	t.Run("Success_Enable", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		repo.EXPECT().Get(ctx, "tenant-1", "suite-1").Return(existingRec, nil)
		repo.EXPECT().Update(ctx, gomock.Any()).Return(nil)

		resp, err := svc.SetSuiteSchedule(ctx, &api.SetSuiteScheduleRequest{
			TenantId: "tenant-1",
			Id:       "suite-1",
			Schedule: &domain.Schedule{Enabled: true, Cron: "0 * * * *"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.GetSuite().GetSummary().GetScheduleEnabled() {
			t.Error("expected schedule enabled in summary")
		}
		if resp.GetSuite().GetSummary().GetCron() != "0 * * * *" {
			t.Errorf("expected cron 0 * * * *, got %s", resp.GetSuite().GetSummary().GetCron())
		}
	})

	t.Run("Success_Disable", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		recWithSched := &models.SuiteRecord{
			Entity: &common.Entity{Id: "suite-1", TenantId: "tenant-1", Timings: &common.Timings{}},
			Spec: &domain.Suite{
				Id:       "suite-1",
				Schedule: &domain.Schedule{Enabled: true, Cron: "0 * * * *"},
			},
		}
		repo.EXPECT().Get(ctx, "tenant-1", "suite-1").Return(recWithSched, nil)
		repo.EXPECT().Update(ctx, gomock.Any()).Return(nil)

		resp, err := svc.SetSuiteSchedule(ctx, &api.SetSuiteScheduleRequest{
			TenantId: "tenant-1",
			Id:       "suite-1",
			Schedule: &domain.Schedule{Enabled: false},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetSuite().GetSummary().GetScheduleEnabled() {
			t.Error("expected schedule disabled in summary")
		}
	})

	t.Run("NilSchedule_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		_, err := svc.SetSuiteSchedule(ctx, &api.SetSuiteScheduleRequest{
			TenantId: "tenant-1",
			Id:       "suite-1",
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("SuiteNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		repo.EXPECT().Get(ctx, "tenant-1", "missing").Return(nil, derrors.ErrNotFound)
		_, err := svc.SetSuiteSchedule(ctx, &api.SetSuiteScheduleRequest{
			TenantId: "tenant-1",
			Id:       "missing",
			Schedule: &domain.Schedule{Enabled: true},
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("UpdateError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		repo.EXPECT().Get(ctx, "tenant-1", "suite-1").Return(existingRec, nil)
		repo.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("db error"))

		_, err := svc.SetSuiteSchedule(ctx, &api.SetSuiteScheduleRequest{
			TenantId: "tenant-1",
			Id:       "suite-1",
			Schedule: &domain.Schedule{Enabled: true, Cron: "* * * * *"},
		})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("NilSpec_InitializedOnSet", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		svc := newSvc(ctrl, authn, repo, nil)

		recNoSpec := &models.SuiteRecord{
			Entity: &common.Entity{Id: "suite-2", TenantId: "tenant-1", Timings: &common.Timings{}},
		}
		repo.EXPECT().Get(ctx, "tenant-1", "suite-2").Return(recNoSpec, nil)
		repo.EXPECT().Update(ctx, gomock.Any()).Return(nil)

		_, err := svc.SetSuiteSchedule(ctx, &api.SetSuiteScheduleRequest{
			TenantId: "tenant-1",
			Id:       "suite-2",
			Schedule: &domain.Schedule{Enabled: true, Cron: "* * * * *"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// ─────────────────────────── StartSuite ─────────────────────────────────────

func TestStartSuite(t *testing.T) {
	ctx := context.Background()

	suiteSpec := &domain.Suite{Id: "suite-1"}
	suiteRec := &models.SuiteRecord{
		Entity: &common.Entity{Id: "suite-1", TenantId: "tenant-1"},
		Spec:   suiteSpec,
	}

	t.Run("Success_BySuiteId", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		launcher := NewMockSuiteRunLauncher(ctrl)
		svc := newSvc(ctrl, authn, repo, launcher)

		stubCaller(authn, ctx, "account-1")
		repo.EXPECT().Get(ctx, "tenant-1", "suite-1").Return(suiteRec, nil)
		launcher.EXPECT().Validate(ctx, "tenant-1", gomock.Any()).Return(nil)
		launcher.EXPECT().Launch(ctx, gomock.Any(), gomock.Any()).Return(nil)

		resp, err := svc.StartSuite(ctx, &api.StartSuiteRequest{
			TenantId: "tenant-1",
			Source:   &api.StartSuiteRequest_SuiteId{SuiteId: "suite-1"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetSuiteRun().GetSuiteId() != "suite-1" {
			t.Errorf("expected suite_id=suite-1, got %s", resp.GetSuiteRun().GetSuiteId())
		}
	})

	t.Run("Success_InlineSuite", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		launcher := NewMockSuiteRunLauncher(ctrl)
		svc := newSvc(ctrl, authn, repo, launcher)

		inlineSuite := &domain.Suite{Id: "inline-1"}
		stubCaller(authn, ctx, "account-1")
		launcher.EXPECT().Validate(ctx, "tenant-1", inlineSuite).Return(nil)
		launcher.EXPECT().Launch(ctx, gomock.Any(), gomock.Any()).Return(nil)

		resp, err := svc.StartSuite(ctx, &api.StartSuiteRequest{
			TenantId: "tenant-1",
			Source:   &api.StartSuiteRequest_Suite{Suite: inlineSuite},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// inline suite_id is empty
		if resp.GetSuiteRun().GetSuiteId() != "" {
			t.Errorf("expected empty suite_id for inline, got %s", resp.GetSuiteRun().GetSuiteId())
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		launcher := NewMockSuiteRunLauncher(ctrl)
		svc := newSvc(ctrl, authn, repo, launcher)

		stubCallerErr(authn, ctx)
		_, err := svc.StartSuite(ctx, &api.StartSuiteRequest{
			TenantId: "tenant-1",
			Source:   &api.StartSuiteRequest_SuiteId{SuiteId: "suite-1"},
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("SuiteIdNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		launcher := NewMockSuiteRunLauncher(ctrl)
		svc := newSvc(ctrl, authn, repo, launcher)

		stubCaller(authn, ctx, "account-1")
		repo.EXPECT().Get(ctx, "tenant-1", "missing").Return(nil, derrors.ErrNotFound)

		_, err := svc.StartSuite(ctx, &api.StartSuiteRequest{
			TenantId: "tenant-1",
			Source:   &api.StartSuiteRequest_SuiteId{SuiteId: "missing"},
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("StoredSuiteNoSpec", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		launcher := NewMockSuiteRunLauncher(ctrl)
		svc := newSvc(ctrl, authn, repo, launcher)

		stubCaller(authn, ctx, "account-1")
		noSpecRec := &models.SuiteRecord{
			Entity: &common.Entity{Id: "suite-1", TenantId: "tenant-1"},
		}
		repo.EXPECT().Get(ctx, "tenant-1", "suite-1").Return(noSpecRec, nil)

		_, err := svc.StartSuite(ctx, &api.StartSuiteRequest{
			TenantId: "tenant-1",
			Source:   &api.StartSuiteRequest_SuiteId{SuiteId: "suite-1"},
		})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", err)
		}
	})

	t.Run("NilInlineSuite_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		launcher := NewMockSuiteRunLauncher(ctrl)
		svc := newSvc(ctrl, authn, repo, launcher)

		stubCaller(authn, ctx, "account-1")
		_, err := svc.StartSuite(ctx, &api.StartSuiteRequest{
			TenantId: "tenant-1",
			Source:   &api.StartSuiteRequest_Suite{Suite: nil},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument for nil inline suite, got %v", err)
		}
	})

	t.Run("NoSource_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		launcher := NewMockSuiteRunLauncher(ctrl)
		svc := newSvc(ctrl, authn, repo, launcher)

		stubCaller(authn, ctx, "account-1")
		_, err := svc.StartSuite(ctx, &api.StartSuiteRequest{TenantId: "tenant-1"})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument for missing source, got %v", err)
		}
	})

	t.Run("ValidateFailure", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		launcher := NewMockSuiteRunLauncher(ctrl)
		svc := newSvc(ctrl, authn, repo, launcher)

		stubCaller(authn, ctx, "account-1")
		repo.EXPECT().Get(ctx, "tenant-1", "suite-1").Return(suiteRec, nil)
		launcher.EXPECT().Validate(ctx, "tenant-1", gomock.Any()).Return(
			derrors.FailedPrecondition("NO_CELLS", "no runnable cells"),
		)

		_, err := svc.StartSuite(ctx, &api.StartSuiteRequest{
			TenantId: "tenant-1",
			Source:   &api.StartSuiteRequest_SuiteId{SuiteId: "suite-1"},
		})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", err)
		}
	})

	t.Run("LaunchError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockSuiteRepo(ctrl)
		launcher := NewMockSuiteRunLauncher(ctrl)
		svc := newSvc(ctrl, authn, repo, launcher)

		stubCaller(authn, ctx, "account-1")
		repo.EXPECT().Get(ctx, "tenant-1", "suite-1").Return(suiteRec, nil)
		launcher.EXPECT().Validate(ctx, "tenant-1", gomock.Any()).Return(nil)
		launcher.EXPECT().Launch(ctx, gomock.Any(), gomock.Any()).Return(errors.New("launch failed"))

		_, err := svc.StartSuite(ctx, &api.StartSuiteRequest{
			TenantId: "tenant-1",
			Source:   &api.StartSuiteRequest_SuiteId{SuiteId: "suite-1"},
		})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})
}

// ─────────────────────────── resolveRating ──────────────────────────────────

func TestResolveRating(t *testing.T) {
	t.Run("Default_TenantTrue_GlobalFalse", func(t *testing.T) {
		req := &api.StartSuiteRequest{}
		spec := &domain.Suite{}
		inTenant, inGlobal := resolveRating(req, spec)
		if !inTenant {
			t.Error("default inTenant should be true")
		}
		if inGlobal {
			t.Error("default inGlobal should be false")
		}
	})

	t.Run("RequestOverrides", func(t *testing.T) {
		f := false
		tr := true
		req := &api.StartSuiteRequest{InTenantRating: &f, InGlobalRating: &tr}
		spec := &domain.Suite{}
		inTenant, inGlobal := resolveRating(req, spec)
		if inTenant {
			t.Error("request override inTenant=false not respected")
		}
		if !inGlobal {
			t.Error("request override inGlobal=true not respected")
		}
	})

	t.Run("SpecDefault_UsedWhenRequestAbsent", func(t *testing.T) {
		f := false
		tr := true
		req := &api.StartSuiteRequest{}
		spec := &domain.Suite{DefaultInTenantRating: &f, DefaultInGlobalRating: &tr}
		inTenant, inGlobal := resolveRating(req, spec)
		if inTenant {
			t.Error("spec default inTenant=false not respected")
		}
		if !inGlobal {
			t.Error("spec default inGlobal=true not respected")
		}
	})
}

// ─────────────────────────── scheduleSummary ────────────────────────────────

func TestScheduleSummary(t *testing.T) {
	t.Run("NilSchedule_DisabledSummary", func(t *testing.T) {
		spec := &domain.Suite{}
		s := scheduleSummary(spec, nil)
		if s.GetScheduleEnabled() {
			t.Error("expected disabled")
		}
		if s.GetCron() != "" {
			t.Error("expected empty cron")
		}
	})

	t.Run("EnabledSchedule", func(t *testing.T) {
		spec := &domain.Suite{Schedule: &domain.Schedule{Enabled: true, Cron: "0 1 * * *"}}
		s := scheduleSummary(spec, nil)
		if !s.GetScheduleEnabled() {
			t.Error("expected enabled")
		}
		if s.GetCron() != "0 1 * * *" {
			t.Errorf("expected cron 0 1 * * *, got %s", s.GetCron())
		}
	})

	t.Run("DisabledSchedule_NextRunAtCleared", func(t *testing.T) {
		spec := &domain.Suite{Schedule: &domain.Schedule{Enabled: false, Cron: "0 1 * * *"}}
		s := scheduleSummary(spec, nil)
		if s.GetNextRunAt() != nil {
			t.Error("expected NextRunAt cleared for disabled schedule")
		}
	})

	t.Run("PreservesRunHistoryFromPrev", func(t *testing.T) {
		spec := &domain.Suite{}
		prev := &models.SuiteRecord_Summary{RunCount: 5}
		s := scheduleSummary(spec, prev)
		if s.GetRunCount() != 5 {
			t.Errorf("expected run_count=5, got %d", s.GetRunCount())
		}
	})
}
