package test_run

// test_run_test.go: tests for all 6 RPCs in test_run.go:
// StartTestRun, GetTestRun, ListTestRuns, CancelTestRun, DeleteTestRun, ExtractToPreset.

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// ---- test helpers ----

func newSvc(ctrl *gomock.Controller) (*TestRunService,
	*utils.MockAuthn, *MockTestRunRepo, *MockPresetRepo, *MockSummarizer, *MockWorkflows) {

	authn := utils.NewMockAuthn(ctrl)
	runs := NewMockTestRunRepo(ctrl)
	presets := NewMockPresetRepo(ctrl)
	sum := NewMockSummarizer(ctrl)
	wf := NewMockWorkflows(ctrl)

	svc := NewTestRunService(TestRunDeps{
		Authn:      authn,
		Runs:       runs,
		Presets:    presets,
		Summarizer: sum,
		Workflows:  wf,
		Tx:         &utils.MockTrm{},
	})
	return svc, authn, runs, presets, sum, wf
}

func stubClaims(authn *utils.MockAuthn, ctx context.Context, accountID string) {
	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: accountID}, nil)
}

func stubCallerErr(authn *utils.MockAuthn, ctx context.Context) {
	authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))
}

func makeTestRun() *domain.TestRun {
	return &domain.TestRun{
		Id:       "spec-id",
		Database: &domain.Database{},
		Workload: &domain.Workload{},
	}
}

func makeRecord(tenantID, runID string, st commonpb.Status) *models.TestRunRecord {
	return &models.TestRunRecord{
		Entity: &commonpb.Entity{
			Id:       runID,
			TenantId: tenantID,
		},
		Spec:   makeTestRun(),
		Status: st,
	}
}

// =====================================================================
// StartTestRun
// =====================================================================

func TestStartTestRun(t *testing.T) {
	ctx := context.Background()

	t.Run("Success_WithRun", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, runs, _, sum, wf := newSvc(ctrl)

		stubClaims(authn, ctx, "author1")
		sum.EXPECT().Summarize(gomock.Any()).Return(nil)
		runs.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		wf.EXPECT().LaunchTest(ctx, gomock.Any()).Return(nil)

		resp, err := svc.StartTestRun(ctx, &api.StartTestRunRequest{
			TenantId: "t1",
			Source:   &api.StartTestRunRequest_Run{Run: makeTestRun()},
		})
		if err != nil {
			t.Fatalf("StartTestRun: %v", err)
		}
		if resp.Run == nil {
			t.Error("expected Run in response")
		}
		if resp.Run.GetStatus() != commonpb.Status_STATUS_PENDING {
			t.Errorf("expected PENDING, got %v", resp.Run.GetStatus())
		}
		if resp.Run.GetTrigger() != commonpb.Trigger_TRIGGER_API {
			t.Errorf("expected TRIGGER_API, got %v", resp.Run.GetTrigger())
		}
	})

	t.Run("Success_RerunExistingId", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, runs, _, sum, wf := newSvc(ctrl)

		existing := makeRecord("t1", "run-old", commonpb.Status_STATUS_COMPLETED)
		stubClaims(authn, ctx, "author1")
		runs.EXPECT().Get(ctx, "t1", "run-old").Return(existing, nil)
		sum.EXPECT().Summarize(gomock.Any()).Return(nil)
		runs.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		wf.EXPECT().LaunchTest(ctx, gomock.Any()).Return(nil)

		resp, err := svc.StartTestRun(ctx, &api.StartTestRunRequest{
			TenantId: "t1",
			Source:   &api.StartTestRunRequest_TestRunId{TestRunId: "run-old"},
		})
		if err != nil {
			t.Fatalf("re-run: %v", err)
		}
		if resp.Run.GetTrigger() != commonpb.Trigger_TRIGGER_MANUAL {
			t.Errorf("expected TRIGGER_MANUAL for re-run, got %v", resp.Run.GetTrigger())
		}
	})

	t.Run("MissingTenantId", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _, _, _, _ := newSvc(ctrl)
		stubClaims(authn, ctx, "a1")

		_, err := svc.StartTestRun(ctx, &api.StartTestRunRequest{
			Source: &api.StartTestRunRequest_Run{Run: makeTestRun()},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("NoSourceProvided", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _, _, _, _ := newSvc(ctrl)
		stubClaims(authn, ctx, "a1")

		_, err := svc.StartTestRun(ctx, &api.StartTestRunRequest{TenantId: "t1"})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument for missing source, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _, _, _, _ := newSvc(ctrl)
		stubCallerErr(authn, ctx)

		_, err := svc.StartTestRun(ctx, &api.StartTestRunRequest{
			TenantId: "t1",
			Source:   &api.StartTestRunRequest_Run{Run: makeTestRun()},
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("RerunSourceNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, runs, _, _, _ := newSvc(ctrl)
		stubClaims(authn, ctx, "a1")
		runs.EXPECT().Get(ctx, "t1", "missing").Return(nil, derrors.ErrNotFound)

		_, err := svc.StartTestRun(ctx, &api.StartTestRunRequest{
			TenantId: "t1",
			Source:   &api.StartTestRunRequest_TestRunId{TestRunId: "missing"},
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("RerunSourceHasNoSpec", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, runs, _, _, _ := newSvc(ctrl)
		stubClaims(authn, ctx, "a1")

		recNoSpec := &models.TestRunRecord{
			Entity: &commonpb.Entity{Id: "r1", TenantId: "t1"},
			Status: commonpb.Status_STATUS_COMPLETED,
		}
		runs.EXPECT().Get(ctx, "t1", "r1").Return(recNoSpec, nil)

		_, err := svc.StartTestRun(ctx, &api.StartTestRunRequest{
			TenantId: "t1",
			Source:   &api.StartTestRunRequest_TestRunId{TestRunId: "r1"},
		})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", err)
		}
	})

	t.Run("CreateError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, runs, _, sum, _ := newSvc(ctrl)
		stubClaims(authn, ctx, "a1")
		sum.EXPECT().Summarize(gomock.Any()).Return(nil)
		runs.EXPECT().Create(ctx, gomock.Any()).Return(errors.New("db err"))

		_, err := svc.StartTestRun(ctx, &api.StartTestRunRequest{
			TenantId: "t1",
			Source:   &api.StartTestRunRequest_Run{Run: makeTestRun()},
		})
		if err == nil {
			t.Error("expected error from Create")
		}
	})

	t.Run("LaunchWorkflowError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, runs, _, sum, wf := newSvc(ctrl)
		stubClaims(authn, ctx, "a1")
		sum.EXPECT().Summarize(gomock.Any()).Return(nil)
		runs.EXPECT().Create(ctx, gomock.Any()).Return(nil)
		wf.EXPECT().LaunchTest(ctx, gomock.Any()).Return(errors.New("workflow error"))

		_, err := svc.StartTestRun(ctx, &api.StartTestRunRequest{
			TenantId: "t1",
			Source:   &api.StartTestRunRequest_Run{Run: makeTestRun()},
		})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal for workflow launch failure, got %v", err)
		}
	})
}

// =====================================================================
// GetTestRun
// =====================================================================

func TestGetTestRun(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, runs, _, _, _ := newSvc(ctrl)

		rec := makeRecord("t1", "r1", commonpb.Status_STATUS_RUNNING)
		runs.EXPECT().Get(ctx, "t1", "r1").Return(rec, nil)

		resp, err := svc.GetTestRun(ctx, &api.GetTestRunRequest{TenantId: "t1", Id: "r1"})
		if err != nil {
			t.Fatalf("GetTestRun: %v", err)
		}
		if resp.Run.GetEntity().GetId() != "r1" {
			t.Errorf("expected id r1, got %s", resp.Run.GetEntity().GetId())
		}
	})

	t.Run("MissingTenantId", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, _, _, _, _ := newSvc(ctrl)

		_, err := svc.GetTestRun(ctx, &api.GetTestRunRequest{Id: "r1"})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, runs, _, _, _ := newSvc(ctrl)
		runs.EXPECT().Get(ctx, "t1", "nope").Return(nil, derrors.ErrNotFound)

		_, err := svc.GetTestRun(ctx, &api.GetTestRunRequest{TenantId: "t1", Id: "nope"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("InternalError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, runs, _, _, _ := newSvc(ctrl)
		runs.EXPECT().Get(ctx, "t1", "r1").Return(nil, errors.New("db gone"))

		_, err := svc.GetTestRun(ctx, &api.GetTestRunRequest{TenantId: "t1", Id: "r1"})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})
}

// =====================================================================
// ListTestRuns
// =====================================================================

func TestListTestRuns(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, runs, _, _, _ := newSvc(ctrl)

		recs := []*models.TestRunRecord{
			makeRecord("t1", "r1", commonpb.Status_STATUS_COMPLETED),
			makeRecord("t1", "r2", commonpb.Status_STATUS_RUNNING),
		}
		runs.EXPECT().List(ctx, gomock.Any()).Return(recs, "next-token", nil)

		resp, err := svc.ListTestRuns(ctx, &api.ListTestRunsRequest{TenantId: "t1"})
		if err != nil {
			t.Fatalf("ListTestRuns: %v", err)
		}
		if len(resp.Runs) != 2 {
			t.Errorf("expected 2 runs, got %d", len(resp.Runs))
		}
		if resp.NextPageToken != "next-token" {
			t.Errorf("expected next-token, got %s", resp.NextPageToken)
		}
	})

	t.Run("MissingTenantId", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, _, _, _, _ := newSvc(ctrl)

		_, err := svc.ListTestRuns(ctx, &api.ListTestRunsRequest{})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("ListError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, runs, _, _, _ := newSvc(ctrl)
		runs.EXPECT().List(ctx, gomock.Any()).Return(nil, "", errors.New("db err"))

		_, err := svc.ListTestRuns(ctx, &api.ListTestRunsRequest{TenantId: "t1"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// =====================================================================
// CancelTestRun
// =====================================================================

func TestCancelTestRun(t *testing.T) {
	ctx := context.Background()

	t.Run("Success_CancelsRunningRun", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, runs, _, _, wf := newSvc(ctrl)

		rec := makeRecord("t1", "r1", commonpb.Status_STATUS_RUNNING)
		runs.EXPECT().Get(ctx, "t1", "r1").Return(rec, nil)
		runs.EXPECT().Update(ctx, gomock.Any()).Return(nil)
		wf.EXPECT().CancelTest(ctx, "r1").Return(nil)

		resp, err := svc.CancelTestRun(ctx, &api.CancelTestRunRequest{TenantId: "t1", Id: "r1"})
		if err != nil {
			t.Fatalf("CancelTestRun: %v", err)
		}
		if resp.Run.GetStatus() != commonpb.Status_STATUS_CANCELLING {
			t.Errorf("expected CANCELLING, got %v", resp.Run.GetStatus())
		}
	})

	t.Run("Idempotent_AlreadyTerminal", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, runs, _, _, _ := newSvc(ctrl)

		for _, st := range []commonpb.Status{
			commonpb.Status_STATUS_COMPLETED,
			commonpb.Status_STATUS_FAILED,
			commonpb.Status_STATUS_CANCELLED,
			commonpb.Status_STATUS_SKIPPED,
		} {
			t.Run(st.String(), func(t *testing.T) {
				rec := makeRecord("t1", "r1", st)
				runs.EXPECT().Get(ctx, "t1", "r1").Return(rec, nil)

				resp, err := svc.CancelTestRun(ctx, &api.CancelTestRunRequest{TenantId: "t1", Id: "r1"})
				if err != nil {
					t.Fatalf("CancelTestRun terminal: %v", err)
				}
				if resp.Run.GetStatus() != st {
					t.Errorf("expected %v unchanged, got %v", st, resp.Run.GetStatus())
				}
			})
		}
	})

	t.Run("Idempotent_AlreadyCancelling", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, runs, _, _, wf := newSvc(ctrl)

		rec := makeRecord("t1", "r1", commonpb.Status_STATUS_CANCELLING)
		runs.EXPECT().Get(ctx, "t1", "r1").Return(rec, nil)
		// workflow signal when already CANCELLING:
		wf.EXPECT().CancelTest(ctx, "r1").Return(nil)

		resp, err := svc.CancelTestRun(ctx, &api.CancelTestRunRequest{TenantId: "t1", Id: "r1"})
		if err != nil {
			t.Fatalf("CancelTestRun already cancelling: %v", err)
		}
		if resp.Run.GetStatus() != commonpb.Status_STATUS_CANCELLING {
			t.Errorf("expected CANCELLING, got %v", resp.Run.GetStatus())
		}
	})

	t.Run("MissingTenantId", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, _, _, _, _ := newSvc(ctrl)

		_, err := svc.CancelTestRun(ctx, &api.CancelTestRunRequest{Id: "r1"})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, runs, _, _, _ := newSvc(ctrl)
		runs.EXPECT().Get(ctx, "t1", "nope").Return(nil, derrors.ErrNotFound)

		_, err := svc.CancelTestRun(ctx, &api.CancelTestRunRequest{TenantId: "t1", Id: "nope"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("UpdateError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, runs, _, _, _ := newSvc(ctrl)

		rec := makeRecord("t1", "r1", commonpb.Status_STATUS_RUNNING)
		runs.EXPECT().Get(ctx, "t1", "r1").Return(rec, nil)
		runs.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("db err"))

		_, err := svc.CancelTestRun(ctx, &api.CancelTestRunRequest{TenantId: "t1", Id: "r1"})
		if err == nil {
			t.Error("expected error from Update")
		}
	})

	t.Run("WorkflowCancelError_Propagates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, runs, _, _, wf := newSvc(ctrl)

		rec := makeRecord("t1", "r1", commonpb.Status_STATUS_RUNNING)
		runs.EXPECT().Get(ctx, "t1", "r1").Return(rec, nil)
		runs.EXPECT().Update(ctx, gomock.Any()).Return(nil)
		wf.EXPECT().CancelTest(ctx, "r1").Return(errors.New("workflow gone"))

		_, err := svc.CancelTestRun(ctx, &api.CancelTestRunRequest{TenantId: "t1", Id: "r1"})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("WorkflowCancelNotFound_IsNoOp", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, runs, _, _, wf := newSvc(ctrl)

		rec := makeRecord("t1", "r1", commonpb.Status_STATUS_RUNNING)
		runs.EXPECT().Get(ctx, "t1", "r1").Return(rec, nil)
		runs.EXPECT().Update(ctx, gomock.Any()).Return(nil)
		// Returning ErrNotFound from CancelTest is a no-op (workflow not running).
		wf.EXPECT().CancelTest(ctx, "r1").Return(derrors.ErrNotFound)

		_, err := svc.CancelTestRun(ctx, &api.CancelTestRunRequest{TenantId: "t1", Id: "r1"})
		if err != nil {
			t.Fatalf("expected no error when workflow not found, got %v", err)
		}
	})
}

// =====================================================================
// DeleteTestRun
// =====================================================================

func TestDeleteTestRun(t *testing.T) {
	ctx := context.Background()

	t.Run("Success_TerminalRun", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, runs, _, _, _ := newSvc(ctrl)

		rec := makeRecord("t1", "r1", commonpb.Status_STATUS_COMPLETED)
		runs.EXPECT().Get(ctx, "t1", "r1").Return(rec, nil)
		runs.EXPECT().Delete(ctx, "t1", "r1").Return(nil)

		_, err := svc.DeleteTestRun(ctx, &api.DeleteTestRunRequest{TenantId: "t1", Id: "r1"})
		if err != nil {
			t.Fatalf("DeleteTestRun: %v", err)
		}
	})

	t.Run("Success_ActiveRun_CancelsFirst", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, runs, _, _, wf := newSvc(ctrl)

		rec := makeRecord("t1", "r1", commonpb.Status_STATUS_RUNNING)
		runs.EXPECT().Get(ctx, "t1", "r1").Return(rec, nil)
		wf.EXPECT().CancelTest(ctx, "r1").Return(nil)
		runs.EXPECT().Delete(ctx, "t1", "r1").Return(nil)

		_, err := svc.DeleteTestRun(ctx, &api.DeleteTestRunRequest{TenantId: "t1", Id: "r1"})
		if err != nil {
			t.Fatalf("DeleteTestRun active: %v", err)
		}
	})

	t.Run("Idempotent_NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, runs, _, _, _ := newSvc(ctrl)
		runs.EXPECT().Get(ctx, "t1", "nope").Return(nil, derrors.ErrNotFound)

		_, err := svc.DeleteTestRun(ctx, &api.DeleteTestRunRequest{TenantId: "t1", Id: "nope"})
		if err != nil {
			t.Fatalf("expected no error for not found, got %v", err)
		}
	})

	t.Run("MissingTenantId", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, _, _, _, _ := newSvc(ctrl)

		_, err := svc.DeleteTestRun(ctx, &api.DeleteTestRunRequest{Id: "r1"})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("GetError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, runs, _, _, _ := newSvc(ctrl)
		runs.EXPECT().Get(ctx, "t1", "r1").Return(nil, errors.New("db gone"))

		_, err := svc.DeleteTestRun(ctx, &api.DeleteTestRunRequest{TenantId: "t1", Id: "r1"})
		if err == nil {
			t.Error("expected error from Get")
		}
	})

	t.Run("CancelWorkflowError_Propagates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, runs, _, _, wf := newSvc(ctrl)

		rec := makeRecord("t1", "r1", commonpb.Status_STATUS_RUNNING)
		runs.EXPECT().Get(ctx, "t1", "r1").Return(rec, nil)
		wf.EXPECT().CancelTest(ctx, "r1").Return(errors.New("workflow gone"))

		_, err := svc.DeleteTestRun(ctx, &api.DeleteTestRunRequest{TenantId: "t1", Id: "r1"})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("CancelWorkflow_NotFound_IsNoOp", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, runs, _, _, wf := newSvc(ctrl)

		rec := makeRecord("t1", "r1", commonpb.Status_STATUS_RUNNING)
		runs.EXPECT().Get(ctx, "t1", "r1").Return(rec, nil)
		wf.EXPECT().CancelTest(ctx, "r1").Return(derrors.ErrNotFound)
		runs.EXPECT().Delete(ctx, "t1", "r1").Return(nil)

		_, err := svc.DeleteTestRun(ctx, &api.DeleteTestRunRequest{TenantId: "t1", Id: "r1"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("DeleteError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, runs, _, _, _ := newSvc(ctrl)

		rec := makeRecord("t1", "r1", commonpb.Status_STATUS_COMPLETED)
		runs.EXPECT().Get(ctx, "t1", "r1").Return(rec, nil)
		runs.EXPECT().Delete(ctx, "t1", "r1").Return(errors.New("db err"))

		_, err := svc.DeleteTestRun(ctx, &api.DeleteTestRunRequest{TenantId: "t1", Id: "r1"})
		if err == nil {
			t.Error("expected error from Delete")
		}
	})

	// Delete returning ErrNotFound inside doTx should be swallowed (IgnoreNotFound).
	t.Run("DeleteNotFound_IsNoOp", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, _, runs, _, _, _ := newSvc(ctrl)

		rec := makeRecord("t1", "r1", commonpb.Status_STATUS_COMPLETED)
		runs.EXPECT().Get(ctx, "t1", "r1").Return(rec, nil)
		runs.EXPECT().Delete(ctx, "t1", "r1").Return(derrors.ErrNotFound)

		_, err := svc.DeleteTestRun(ctx, &api.DeleteTestRunRequest{TenantId: "t1", Id: "r1"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})
}

// =====================================================================
// ExtractToPreset
// =====================================================================

func TestExtractToPreset(t *testing.T) {
	ctx := context.Background()

	t.Run("Success_DerivesName", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, runs, presets, _, _ := newSvc(ctrl)

		rec := makeRecord("t1", "r1", commonpb.Status_STATUS_COMPLETED)
		stubClaims(authn, ctx, "author1")
		runs.EXPECT().Get(ctx, "t1", "r1").Return(rec, nil)
		presets.EXPECT().CreateTestPreset(ctx, gomock.Any()).Return(nil)

		resp, err := svc.ExtractToPreset(ctx, &api.ExtractToPresetRequest{TenantId: "t1", Id: "r1"})
		if err != nil {
			t.Fatalf("ExtractToPreset: %v", err)
		}
		if resp.Preset == nil {
			t.Error("expected Preset in response")
		}
		if resp.Preset.GetEntity().GetTenantId() != "t1" {
			t.Errorf("expected tenant t1, got %s", resp.Preset.GetEntity().GetTenantId())
		}
	})

	t.Run("Success_CustomName", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, runs, presets, _, _ := newSvc(ctrl)

		rec := makeRecord("t1", "r1", commonpb.Status_STATUS_COMPLETED)
		stubClaims(authn, ctx, "author1")
		runs.EXPECT().Get(ctx, "t1", "r1").Return(rec, nil)
		presets.EXPECT().CreateTestPreset(ctx, gomock.Any()).DoAndReturn(
			func(_ context.Context, p *models.TestPresetRecord) error {
				if p.GetEntity().GetName() != "my preset" {
					t.Errorf("expected name 'my preset', got %s", p.GetEntity().GetName())
				}
				return nil
			})

		_, err := svc.ExtractToPreset(ctx, &api.ExtractToPresetRequest{
			TenantId: "t1",
			Id:       "r1",
			Name:     "my preset",
		})
		if err != nil {
			t.Fatalf("ExtractToPreset custom name: %v", err)
		}
	})

	t.Run("MissingTenantId", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _, _, _, _ := newSvc(ctrl)
		stubClaims(authn, ctx, "a1")

		_, err := svc.ExtractToPreset(ctx, &api.ExtractToPresetRequest{Id: "r1"})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _, _, _, _ := newSvc(ctrl)
		stubCallerErr(authn, ctx)

		_, err := svc.ExtractToPreset(ctx, &api.ExtractToPresetRequest{TenantId: "t1", Id: "r1"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("RunNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, runs, _, _, _ := newSvc(ctrl)
		stubClaims(authn, ctx, "a1")
		runs.EXPECT().Get(ctx, "t1", "nope").Return(nil, derrors.ErrNotFound)

		_, err := svc.ExtractToPreset(ctx, &api.ExtractToPresetRequest{TenantId: "t1", Id: "nope"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("RunHasNoSpec", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, runs, _, _, _ := newSvc(ctrl)
		stubClaims(authn, ctx, "a1")

		recNoSpec := &models.TestRunRecord{
			Entity: &commonpb.Entity{Id: "r1", TenantId: "t1"},
			Status: commonpb.Status_STATUS_COMPLETED,
		}
		runs.EXPECT().Get(ctx, "t1", "r1").Return(recNoSpec, nil)

		_, err := svc.ExtractToPreset(ctx, &api.ExtractToPresetRequest{TenantId: "t1", Id: "r1"})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition, got %v", err)
		}
	})

	t.Run("RunSpecMissingDatabase", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, runs, _, _, _ := newSvc(ctrl)
		stubClaims(authn, ctx, "a1")

		rec := &models.TestRunRecord{
			Entity: &commonpb.Entity{Id: "r1", TenantId: "t1"},
			Spec: &domain.TestRun{
				Id:       "r1",
				Workload: &domain.Workload{}, // no Database
			},
			Status: commonpb.Status_STATUS_COMPLETED,
		}
		runs.EXPECT().Get(ctx, "t1", "r1").Return(rec, nil)

		_, err := svc.ExtractToPreset(ctx, &api.ExtractToPresetRequest{TenantId: "t1", Id: "r1"})
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition for missing db, got %v", err)
		}
	})

	t.Run("CreatePresetError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, runs, presets, _, _ := newSvc(ctrl)

		rec := makeRecord("t1", "r1", commonpb.Status_STATUS_COMPLETED)
		stubClaims(authn, ctx, "a1")
		runs.EXPECT().Get(ctx, "t1", "r1").Return(rec, nil)
		presets.EXPECT().CreateTestPreset(ctx, gomock.Any()).Return(errors.New("db err"))

		_, err := svc.ExtractToPreset(ctx, &api.ExtractToPresetRequest{TenantId: "t1", Id: "r1"})
		if err == nil {
			t.Error("expected error from CreateTestPreset")
		}
	})
}
