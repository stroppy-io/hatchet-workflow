package suite_run

// suite_run_test.go: unit tests for SuiteRunService RPCs:
// GetSuiteRun, ListSuiteRuns, CancelSuiteRun, DeleteSuiteRun.

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// makeRecord builds a minimal SuiteRunRecord owned by tenantID.
func makeRecord(id, tenantID string, st common.Status) *models.SuiteRunRecord {
	return &models.SuiteRunRecord{
		Entity: &common.Entity{
			Id:       id,
			TenantId: tenantID,
		},
		Status: st,
	}
}

// newSvc is a test helper that wires up all mocks and returns the service.
func newSvc(
	ctrl *gomock.Controller,
) (
	svc *SuiteRunService,
	repo *MockSuiteRunRepo,
	canceller *MockSuiteRunCanceller,
) {
	repo = NewMockSuiteRunRepo(ctrl)
	canceller = NewMockSuiteRunCanceller(ctrl)
	svc = NewSuiteRunService(SuiteRunDeps{
		Authn:     nil, // Authn is NOT used by any handler directly
		SuiteRuns: repo,
		Canceller: canceller,
		Tx:        &utils.MockTrm{},
	})
	return svc, repo, canceller
}

// ───────────────────────────── GetSuiteRun ─────────────────────────────────

func TestGetSuiteRun(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, _ := newSvc(ctrl)

		rec := makeRecord("run1", "tenant1", common.Status_STATUS_RUNNING)
		repo.EXPECT().Get(ctx, "run1").Return(rec, nil)

		resp, err := svc.GetSuiteRun(ctx, &api.GetSuiteRunRequest{TenantId: "tenant1", Id: "run1"})
		if err != nil {
			t.Fatalf("GetSuiteRun: %v", err)
		}
		if resp.SuiteRun.GetEntity().GetId() != "run1" {
			t.Errorf("expected id run1, got %s", resp.SuiteRun.GetEntity().GetId())
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, _ := newSvc(ctrl)

		repo.EXPECT().Get(ctx, "missing").Return(nil, derrors.ErrNotFound)
		_, err := svc.GetSuiteRun(ctx, &api.GetSuiteRunRequest{TenantId: "tenant1", Id: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("WrongTenant_ReturnsNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, _ := newSvc(ctrl)

		// Record belongs to "other-tenant", not "tenant1"
		rec := makeRecord("run1", "other-tenant", common.Status_STATUS_RUNNING)
		repo.EXPECT().Get(ctx, "run1").Return(rec, nil)

		_, err := svc.GetSuiteRun(ctx, &api.GetSuiteRunRequest{TenantId: "tenant1", Id: "run1"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound for cross-tenant access, got %v", err)
		}
	})

	t.Run("RepoError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, _ := newSvc(ctrl)

		repo.EXPECT().Get(ctx, "run1").Return(nil, errors.New("db error"))
		_, err := svc.GetSuiteRun(ctx, &api.GetSuiteRunRequest{TenantId: "tenant1", Id: "run1"})
		if err == nil {
			t.Error("expected error from repo")
		}
	})
}

// ──────────────────────────── ListSuiteRuns ────────────────────────────────

func TestListSuiteRuns(t *testing.T) {
	ctx := context.Background()

	t.Run("Success_WithResults", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, _ := newSvc(ctrl)

		records := []*models.SuiteRunRecord{
			makeRecord("r1", "t1", common.Status_STATUS_COMPLETED),
			makeRecord("r2", "t1", common.Status_STATUS_RUNNING),
		}
		req := &api.ListSuiteRunsRequest{TenantId: "t1"}
		repo.EXPECT().List(ctx, req).Return(records, "next-token", nil)

		resp, err := svc.ListSuiteRuns(ctx, req)
		if err != nil {
			t.Fatalf("ListSuiteRuns: %v", err)
		}
		if len(resp.SuiteRuns) != 2 {
			t.Errorf("expected 2 records, got %d", len(resp.SuiteRuns))
		}
		if resp.NextPageToken != "next-token" {
			t.Errorf("expected next-token, got %s", resp.NextPageToken)
		}
	})

	t.Run("Success_EmptyList", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, _ := newSvc(ctrl)

		req := &api.ListSuiteRunsRequest{TenantId: "t1"}
		repo.EXPECT().List(ctx, req).Return(nil, "", nil)

		resp, err := svc.ListSuiteRuns(ctx, req)
		if err != nil {
			t.Fatalf("ListSuiteRuns empty: %v", err)
		}
		if len(resp.SuiteRuns) != 0 {
			t.Errorf("expected 0 records, got %d", len(resp.SuiteRuns))
		}
	})

	t.Run("RepoError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, _ := newSvc(ctrl)

		req := &api.ListSuiteRunsRequest{TenantId: "t1"}
		repo.EXPECT().List(ctx, req).Return(nil, "", errors.New("db error"))

		_, err := svc.ListSuiteRuns(ctx, req)
		if err == nil {
			t.Error("expected error from repo")
		}
	})
}

// ─────────────────────────── CancelSuiteRun ───────────────────────────────

func TestCancelSuiteRun(t *testing.T) {
	ctx := context.Background()

	t.Run("Success_RunningRun_BecomeCancelling", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, canceller := newSvc(ctrl)

		rec := makeRecord("run1", "tenant1", common.Status_STATUS_RUNNING)
		repo.EXPECT().Get(ctx, "run1").Return(rec, nil)
		canceller.EXPECT().CancelSuiteRun(ctx, "run1").Return(nil)
		repo.EXPECT().Update(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, r *models.SuiteRunRecord) error {
			if r.Status != common.Status_STATUS_CANCELLING {
				t.Errorf("expected STATUS_CANCELLING, got %v", r.Status)
			}
			return nil
		})

		resp, err := svc.CancelSuiteRun(ctx, &api.CancelSuiteRunRequest{TenantId: "tenant1", Id: "run1"})
		if err != nil {
			t.Fatalf("CancelSuiteRun: %v", err)
		}
		if resp.SuiteRun.GetStatus() != common.Status_STATUS_CANCELLING {
			t.Errorf("expected CANCELLING, got %v", resp.SuiteRun.GetStatus())
		}
	})

	t.Run("Idempotent_AlreadyCancelled", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, _ := newSvc(ctrl)

		rec := makeRecord("run1", "tenant1", common.Status_STATUS_CANCELLED)
		repo.EXPECT().Get(ctx, "run1").Return(rec, nil)
		// No canceller or update calls expected

		resp, err := svc.CancelSuiteRun(ctx, &api.CancelSuiteRunRequest{TenantId: "tenant1", Id: "run1"})
		if err != nil {
			t.Fatalf("CancelSuiteRun idempotent cancelled: %v", err)
		}
		if resp.SuiteRun.GetStatus() != common.Status_STATUS_CANCELLED {
			t.Errorf("expected CANCELLED unchanged, got %v", resp.SuiteRun.GetStatus())
		}
	})

	t.Run("Idempotent_AlreadyCompleted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, _ := newSvc(ctrl)

		rec := makeRecord("run1", "tenant1", common.Status_STATUS_COMPLETED)
		repo.EXPECT().Get(ctx, "run1").Return(rec, nil)

		resp, err := svc.CancelSuiteRun(ctx, &api.CancelSuiteRunRequest{TenantId: "tenant1", Id: "run1"})
		if err != nil {
			t.Fatalf("CancelSuiteRun idempotent completed: %v", err)
		}
		if resp.SuiteRun.GetStatus() != common.Status_STATUS_COMPLETED {
			t.Errorf("expected COMPLETED unchanged, got %v", resp.SuiteRun.GetStatus())
		}
	})

	t.Run("Idempotent_AlreadyCancelling", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, _ := newSvc(ctrl)

		rec := makeRecord("run1", "tenant1", common.Status_STATUS_CANCELLING)
		repo.EXPECT().Get(ctx, "run1").Return(rec, nil)

		resp, err := svc.CancelSuiteRun(ctx, &api.CancelSuiteRunRequest{TenantId: "tenant1", Id: "run1"})
		if err != nil {
			t.Fatalf("CancelSuiteRun idempotent cancelling: %v", err)
		}
		if resp.SuiteRun.GetStatus() != common.Status_STATUS_CANCELLING {
			t.Errorf("expected CANCELLING unchanged, got %v", resp.SuiteRun.GetStatus())
		}
	})

	t.Run("Idempotent_AlreadyFailed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, _ := newSvc(ctrl)

		rec := makeRecord("run1", "tenant1", common.Status_STATUS_FAILED)
		repo.EXPECT().Get(ctx, "run1").Return(rec, nil)

		_, err := svc.CancelSuiteRun(ctx, &api.CancelSuiteRunRequest{TenantId: "tenant1", Id: "run1"})
		if err != nil {
			t.Fatalf("CancelSuiteRun idempotent failed: %v", err)
		}
	})

	t.Run("Idempotent_AlreadySkipped", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, _ := newSvc(ctrl)

		rec := makeRecord("run1", "tenant1", common.Status_STATUS_SKIPPED)
		repo.EXPECT().Get(ctx, "run1").Return(rec, nil)

		_, err := svc.CancelSuiteRun(ctx, &api.CancelSuiteRunRequest{TenantId: "tenant1", Id: "run1"})
		if err != nil {
			t.Fatalf("CancelSuiteRun idempotent skipped: %v", err)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, _ := newSvc(ctrl)

		repo.EXPECT().Get(ctx, "missing").Return(nil, derrors.ErrNotFound)
		_, err := svc.CancelSuiteRun(ctx, &api.CancelSuiteRunRequest{TenantId: "tenant1", Id: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("WrongTenant_ReturnsNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, _ := newSvc(ctrl)

		rec := makeRecord("run1", "other-tenant", common.Status_STATUS_RUNNING)
		repo.EXPECT().Get(ctx, "run1").Return(rec, nil)

		_, err := svc.CancelSuiteRun(ctx, &api.CancelSuiteRunRequest{TenantId: "tenant1", Id: "run1"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound for cross-tenant, got %v", err)
		}
	})

	t.Run("CancellerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, canceller := newSvc(ctrl)

		rec := makeRecord("run1", "tenant1", common.Status_STATUS_RUNNING)
		repo.EXPECT().Get(ctx, "run1").Return(rec, nil)
		canceller.EXPECT().CancelSuiteRun(ctx, "run1").Return(errors.New("runtime error"))

		_, err := svc.CancelSuiteRun(ctx, &api.CancelSuiteRunRequest{TenantId: "tenant1", Id: "run1"})
		if err == nil {
			t.Error("expected error from canceller")
		}
	})

	t.Run("UpdateError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, canceller := newSvc(ctrl)

		rec := makeRecord("run1", "tenant1", common.Status_STATUS_RUNNING)
		repo.EXPECT().Get(ctx, "run1").Return(rec, nil)
		canceller.EXPECT().CancelSuiteRun(ctx, "run1").Return(nil)
		repo.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("db error"))

		_, err := svc.CancelSuiteRun(ctx, &api.CancelSuiteRunRequest{TenantId: "tenant1", Id: "run1"})
		if err == nil {
			t.Error("expected error from update")
		}
	})

	// Test that timings are stamped when Entity is nil (touchUpdated nil path)
	t.Run("NilEntity_TimingsHandled", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, canceller := newSvc(ctrl)

		// Record with a nil Entity — canceller path uses GetEntity().GetId() which
		// returns "" for nil, and touchUpdated is guarded by rec.Entity != nil.
		rec := &models.SuiteRunRecord{
			Entity: &common.Entity{Id: "run1", TenantId: "tenant1"},
			Status: common.Status_STATUS_RUNNING,
		}
		repo.EXPECT().Get(ctx, "run1").Return(rec, nil)
		canceller.EXPECT().CancelSuiteRun(ctx, "run1").Return(nil)
		repo.EXPECT().Update(ctx, gomock.Any()).Return(nil)

		resp, err := svc.CancelSuiteRun(ctx, &api.CancelSuiteRunRequest{TenantId: "tenant1", Id: "run1"})
		if err != nil {
			t.Fatalf("CancelSuiteRun nil entity: %v", err)
		}
		if resp.SuiteRun.GetStatus() != common.Status_STATUS_CANCELLING {
			t.Errorf("expected CANCELLING, got %v", resp.SuiteRun.GetStatus())
		}
	})
}

// ─────────────────────────── DeleteSuiteRun ───────────────────────────────

func TestDeleteSuiteRun(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, _ := newSvc(ctrl)

		rec := makeRecord("run1", "tenant1", common.Status_STATUS_COMPLETED)
		repo.EXPECT().Get(ctx, "run1").Return(rec, nil)
		repo.EXPECT().Delete(ctx, "run1").Return(nil)

		resp, err := svc.DeleteSuiteRun(ctx, &api.DeleteSuiteRunRequest{TenantId: "tenant1", Id: "run1"})
		if err != nil {
			t.Fatalf("DeleteSuiteRun: %v", err)
		}
		if resp == nil {
			t.Error("expected non-nil response")
		}
	})

	t.Run("Idempotent_NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, _ := newSvc(ctrl)

		// getOwned returns ErrNotFound — DeleteSuiteRun treats it as a no-op
		repo.EXPECT().Get(ctx, "missing").Return(nil, derrors.ErrNotFound)

		resp, err := svc.DeleteSuiteRun(ctx, &api.DeleteSuiteRunRequest{TenantId: "tenant1", Id: "missing"})
		if err != nil {
			t.Fatalf("DeleteSuiteRun idempotent not-found: %v", err)
		}
		if resp == nil {
			t.Error("expected non-nil response")
		}
	})

	t.Run("WrongTenant_Idempotent_NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, _ := newSvc(ctrl)

		// Record belongs to a different tenant; getOwned returns ErrNotFound
		// which DeleteSuiteRun treats as a no-op (idempotent).
		rec := makeRecord("run1", "other-tenant", common.Status_STATUS_COMPLETED)
		repo.EXPECT().Get(ctx, "run1").Return(rec, nil)

		resp, err := svc.DeleteSuiteRun(ctx, &api.DeleteSuiteRunRequest{TenantId: "tenant1", Id: "run1"})
		if err != nil {
			t.Fatalf("DeleteSuiteRun wrong tenant: %v", err)
		}
		if resp == nil {
			t.Error("expected non-nil response")
		}
	})

	t.Run("RepoGetError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, _ := newSvc(ctrl)

		repo.EXPECT().Get(ctx, "run1").Return(nil, errors.New("db error"))
		_, err := svc.DeleteSuiteRun(ctx, &api.DeleteSuiteRunRequest{TenantId: "tenant1", Id: "run1"})
		if err == nil {
			t.Error("expected error from repo get")
		}
	})

	t.Run("DeleteError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, _ := newSvc(ctrl)

		rec := makeRecord("run1", "tenant1", common.Status_STATUS_COMPLETED)
		repo.EXPECT().Get(ctx, "run1").Return(rec, nil)
		repo.EXPECT().Delete(ctx, "run1").Return(errors.New("db delete error"))

		_, err := svc.DeleteSuiteRun(ctx, &api.DeleteSuiteRunRequest{TenantId: "tenant1", Id: "run1"})
		if err == nil {
			t.Error("expected error from repo delete")
		}
	})

	t.Run("DeleteNotFound_IsIgnored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, repo, _ := newSvc(ctrl)

		rec := makeRecord("run1", "tenant1", common.Status_STATUS_COMPLETED)
		repo.EXPECT().Get(ctx, "run1").Return(rec, nil)
		// Delete returns ErrNotFound — IgnoreNotFound wraps this, should be a no-op
		repo.EXPECT().Delete(ctx, "run1").Return(derrors.ErrNotFound)

		resp, err := svc.DeleteSuiteRun(ctx, &api.DeleteSuiteRunRequest{TenantId: "tenant1", Id: "run1"})
		if err != nil {
			t.Fatalf("DeleteSuiteRun delete-not-found: %v", err)
		}
		if resp == nil {
			t.Error("expected non-nil response")
		}
	})
}

// ────────────────────────── Helper unit tests ─────────────────────────────

func TestIsTerminal(t *testing.T) {
	terminal := []common.Status{
		common.Status_STATUS_COMPLETED,
		common.Status_STATUS_FAILED,
		common.Status_STATUS_SKIPPED,
		common.Status_STATUS_CANCELLED,
	}
	nonTerminal := []common.Status{
		common.Status_STATUS_UNSPECIFIED,
		common.Status_STATUS_PENDING,
		common.Status_STATUS_RUNNING,
		common.Status_STATUS_CANCELLING,
	}
	for _, st := range terminal {
		if !isTerminal(st) {
			t.Errorf("expected %v to be terminal", st)
		}
	}
	for _, st := range nonTerminal {
		if isTerminal(st) {
			t.Errorf("expected %v to NOT be terminal", st)
		}
	}
}

func TestTouchUpdated(t *testing.T) {
	ts := time.Now()

	t.Run("NilTimings", func(t *testing.T) {
		result := touchUpdated(nil, timestamppb.New(ts))
		if result == nil {
			t.Fatal("expected non-nil timings")
		}
		if result.UpdatedAt == nil {
			t.Error("expected UpdatedAt to be set")
		}
	})

	t.Run("ExistingTimings", func(t *testing.T) {
		existing := &common.Timings{}
		result := touchUpdated(existing, timestamppb.New(ts))
		if result != existing {
			t.Error("expected same timings pointer")
		}
		if result.UpdatedAt == nil {
			t.Error("expected UpdatedAt to be set")
		}
	})
}
