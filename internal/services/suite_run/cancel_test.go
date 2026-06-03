package suite_run

import (
	"context"
	"testing"

	trm "github.com/avito-tech/go-transaction-manager/trm"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

func TestCancelSuiteRunFinalizesParentAndChildrenWhenWorkflowIsNotFound(t *testing.T) {
	suites := &fakeSuiteRunRepo{
		run: &models.SuiteRunRecord{
			Entity: &common.Entity{Id: "suite-run-1", TenantId: "tenant-1"},
			Status: common.Status_STATUS_RUNNING,
			Children: []*models.SuiteRunRecord_ChildRun{
				{TestRunId: "run-1", Status: common.Status_STATUS_RUNNING},
			},
		},
	}
	runs := &fakeSuiteChildRunRepo{
		runs: map[string]*models.TestRunRecord{
			"run-1": {
				Entity: &common.Entity{Id: "run-1", TenantId: "tenant-1"},
				Status: common.Status_STATUS_RUNNING,
			},
		},
	}
	svc := NewSuiteRunService(SuiteRunDeps{
		SuiteRuns: suites,
		Runs:      runs,
		Canceller: fakeMissingSuiteCanceller{},
		Tx:        noopTrm{},
	})

	resp, err := svc.CancelSuiteRun(context.Background(), &api.CancelSuiteRunRequest{
		TenantId: "tenant-1",
		Id:       "suite-run-1",
	})
	if err != nil {
		t.Fatalf("cancel suite run: %v", err)
	}

	if got := resp.GetSuiteRun().GetStatus(); got != common.Status_STATUS_CANCELLED {
		t.Fatalf("suite status = %s, want %s", got, common.Status_STATUS_CANCELLED)
	}
	if got := resp.GetSuiteRun().GetChildren()[0].GetStatus(); got != common.Status_STATUS_CANCELLED {
		t.Fatalf("suite child status = %s, want %s", got, common.Status_STATUS_CANCELLED)
	}
	if got := runs.runs["run-1"].GetStatus(); got != common.Status_STATUS_CANCELLED {
		t.Fatalf("child run status = %s, want %s", got, common.Status_STATUS_CANCELLED)
	}
	if resp.GetSuiteRun().GetSummary().GetFinishedAt() == nil {
		t.Fatal("suite finished_at was not set")
	}
	if runs.runs["run-1"].GetSummary().GetFinishedAt() == nil {
		t.Fatal("child run finished_at was not set")
	}
	if got := resp.GetSuiteRun().GetSummary().GetProgressPct(); got != 100 {
		t.Fatalf("suite progress = %d, want 100", got)
	}
	if suites.updates != 2 {
		t.Fatalf("suite updates = %d, want 2", suites.updates)
	}
	if runs.updates != 2 {
		t.Fatalf("child run updates = %d, want 2", runs.updates)
	}
}

func TestDeleteSuiteRunSoftDeletesTerminalRun(t *testing.T) {
	suites := &fakeSuiteRunRepo{
		run: &models.SuiteRunRecord{
			Entity: &common.Entity{Id: "suite-run-1", TenantId: "tenant-1", Timings: &common.Timings{}},
			Status: common.Status_STATUS_COMPLETED,
		},
	}
	svc := NewSuiteRunService(SuiteRunDeps{
		SuiteRuns: suites,
		Canceller: fakeSuiteCanceller{},
		Tx:        noopTrm{},
	})

	_, err := svc.DeleteSuiteRun(context.Background(), &api.DeleteSuiteRunRequest{
		TenantId: "tenant-1",
		Id:       "suite-run-1",
	})
	if err != nil {
		t.Fatalf("delete suite run: %v", err)
	}

	if suites.run.GetEntity().GetTimings().GetDeletedAt() == nil {
		t.Fatal("deleted_at was not set")
	}
	if got := suites.run.GetStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("suite status = %s, want %s", got, common.Status_STATUS_COMPLETED)
	}
	if suites.updates != 1 {
		t.Fatalf("suite updates = %d, want 1", suites.updates)
	}
}

func TestDeleteSuiteRunFinalizesAndSoftDeletesWhenWorkflowIsNotFound(t *testing.T) {
	suites := &fakeSuiteRunRepo{
		run: &models.SuiteRunRecord{
			Entity: &common.Entity{Id: "suite-run-1", TenantId: "tenant-1", Timings: &common.Timings{}},
			Status: common.Status_STATUS_RUNNING,
			Children: []*models.SuiteRunRecord_ChildRun{
				{TestRunId: "run-1", Status: common.Status_STATUS_RUNNING},
			},
		},
	}
	runs := &fakeSuiteChildRunRepo{
		runs: map[string]*models.TestRunRecord{
			"run-1": {
				Entity: &common.Entity{Id: "run-1", TenantId: "tenant-1"},
				Status: common.Status_STATUS_RUNNING,
			},
		},
	}
	svc := NewSuiteRunService(SuiteRunDeps{
		SuiteRuns: suites,
		Runs:      runs,
		Canceller: fakeMissingSuiteCanceller{},
		Tx:        noopTrm{},
	})

	_, err := svc.DeleteSuiteRun(context.Background(), &api.DeleteSuiteRunRequest{
		TenantId: "tenant-1",
		Id:       "suite-run-1",
	})
	if err != nil {
		t.Fatalf("delete suite run: %v", err)
	}

	if got := suites.run.GetStatus(); got != common.Status_STATUS_CANCELLED {
		t.Fatalf("suite status = %s, want %s", got, common.Status_STATUS_CANCELLED)
	}
	if suites.run.GetEntity().GetTimings().GetDeletedAt() == nil {
		t.Fatal("deleted_at was not set")
	}
	if got := runs.runs["run-1"].GetStatus(); got != common.Status_STATUS_CANCELLED {
		t.Fatalf("child run status = %s, want %s", got, common.Status_STATUS_CANCELLED)
	}
	if suites.updates != 2 {
		t.Fatalf("suite updates = %d, want 2", suites.updates)
	}
	if runs.updates != 2 {
		t.Fatalf("child run updates = %d, want 2", runs.updates)
	}
}

type noopTrm struct{}

func (noopTrm) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func (noopTrm) DoWithSettings(ctx context.Context, _ trm.Settings, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type fakeSuiteRunRepo struct {
	run     *models.SuiteRunRecord
	updates int
}

func (r *fakeSuiteRunRepo) Get(_ context.Context, id string) (*models.SuiteRunRecord, error) {
	if r.run.GetEntity().GetId() != id {
		return nil, derrors.ErrNotFound
	}
	return r.run, nil
}

func (*fakeSuiteRunRepo) List(context.Context, *api.ListSuiteRunsRequest, string) ([]*models.SuiteRunRecord, string, error) {
	return nil, "", nil
}

func (r *fakeSuiteRunRepo) Update(_ context.Context, run *models.SuiteRunRecord) error {
	r.updates++
	r.run = run
	return nil
}

type fakeSuiteChildRunRepo struct {
	runs    map[string]*models.TestRunRecord
	updates int
}

func (r *fakeSuiteChildRunRepo) Get(_ context.Context, tenantID, id string) (*models.TestRunRecord, error) {
	run := r.runs[id]
	if run == nil || run.GetEntity().GetTenantId() != tenantID {
		return nil, derrors.ErrNotFound
	}
	return run, nil
}

func (r *fakeSuiteChildRunRepo) Update(_ context.Context, run *models.TestRunRecord) error {
	r.updates++
	r.runs[run.GetEntity().GetId()] = run
	return nil
}

type fakeMissingSuiteCanceller struct{}

func (fakeMissingSuiteCanceller) CancelSuiteRun(context.Context, string) error {
	return derrors.ErrNotFound
}

type fakeSuiteCanceller struct{}

func (fakeSuiteCanceller) CancelSuiteRun(context.Context, string) error {
	return nil
}
