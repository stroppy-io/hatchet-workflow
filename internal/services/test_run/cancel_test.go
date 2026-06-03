package test_run

import (
	"context"
	"errors"
	"testing"

	trm "github.com/avito-tech/go-transaction-manager/trm"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

func TestCancelTestRunFinalizesWhenWorkflowIsNotFound(t *testing.T) {
	repo := &fakeTestRunRepo{
		run: &models.TestRunRecord{
			Entity: &common.Entity{Id: "run-1", TenantId: "tenant-1"},
			Status: common.Status_STATUS_RUNNING,
		},
	}
	svc := NewTestRunService(TestRunDeps{
		Runs:      repo,
		Workflows: fakeMissingWorkflows{},
		Tx:        noopTrm{},
	})

	resp, err := svc.CancelTestRun(context.Background(), &api.CancelTestRunRequest{
		TenantId: "tenant-1",
		Id:       "run-1",
	})
	if err != nil {
		t.Fatalf("cancel test run: %v", err)
	}

	if got := resp.GetRun().GetStatus(); got != common.Status_STATUS_CANCELLED {
		t.Fatalf("status = %s, want %s", got, common.Status_STATUS_CANCELLED)
	}
	if got := repo.run.GetStatus(); got != common.Status_STATUS_CANCELLED {
		t.Fatalf("persisted status = %s, want %s", got, common.Status_STATUS_CANCELLED)
	}
	if repo.run.GetSummary().GetFinishedAt() == nil {
		t.Fatal("finished_at was not set")
	}
	if repo.updates != 2 {
		t.Fatalf("updates = %d, want 2", repo.updates)
	}
}

func TestStartTestRunMarksRecordFailedWhenWorkflowLaunchFails(t *testing.T) {
	repo := &fakeTestRunRepo{}
	svc := NewTestRunService(TestRunDeps{
		Authn:      fakeAuthn{},
		Runs:       repo,
		Summarizer: fakeSummarizer{},
		Workflows:  fakeLaunchFailWorkflows{},
		Tx:         noopTrm{},
	})

	_, err := svc.StartTestRun(context.Background(), &api.StartTestRunRequest{
		TenantId: "tenant-1",
		Source: &api.StartTestRunRequest_Run{
			Run: &domain.TestRun{
				Database: &domain.Database{Kind: domain.Database_KIND_POSTGRES},
				Workload: &domain.Workload{
					Script: "tpcc/tx",
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected launch error")
	}
	if repo.run == nil {
		t.Fatal("run was not persisted")
	}
	if got := repo.run.GetStatus(); got != common.Status_STATUS_FAILED {
		t.Fatalf("persisted status = %s, want %s", got, common.Status_STATUS_FAILED)
	}
	if repo.run.GetSummary().GetFinishedAt() == nil {
		t.Fatal("finished_at was not set")
	}
}

func TestDeleteTestRunSoftDeletesTerminalRun(t *testing.T) {
	repo := &fakeTestRunRepo{
		run: &models.TestRunRecord{
			Entity: &common.Entity{
				Id:       "run-1",
				TenantId: "tenant-1",
				Timings:  &common.Timings{},
			},
			Status: common.Status_STATUS_COMPLETED,
		},
	}
	svc := NewTestRunService(TestRunDeps{
		Runs:      repo,
		Workflows: fakeMissingWorkflows{},
		Tx:        noopTrm{},
	})

	_, err := svc.DeleteTestRun(context.Background(), &api.DeleteTestRunRequest{
		TenantId: "tenant-1",
		Id:       "run-1",
	})
	if err != nil {
		t.Fatalf("delete test run: %v", err)
	}
	if got := repo.run.GetStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("status = %s, want %s", got, common.Status_STATUS_COMPLETED)
	}
	if repo.run.GetEntity().GetTimings().GetDeletedAt() == nil {
		t.Fatal("deleted_at was not set")
	}
	if repo.updates != 1 {
		t.Fatalf("updates = %d, want 1", repo.updates)
	}
}

func TestDeleteTestRunFinalizesAndSoftDeletesWhenWorkflowIsNotFound(t *testing.T) {
	repo := &fakeTestRunRepo{
		run: &models.TestRunRecord{
			Entity: &common.Entity{
				Id:       "run-1",
				TenantId: "tenant-1",
				Timings:  &common.Timings{},
			},
			Status: common.Status_STATUS_RUNNING,
		},
	}
	svc := NewTestRunService(TestRunDeps{
		Runs:      repo,
		Workflows: fakeMissingWorkflows{},
		Tx:        noopTrm{},
	})

	_, err := svc.DeleteTestRun(context.Background(), &api.DeleteTestRunRequest{
		TenantId: "tenant-1",
		Id:       "run-1",
	})
	if err != nil {
		t.Fatalf("delete test run: %v", err)
	}
	if got := repo.run.GetStatus(); got != common.Status_STATUS_CANCELLED {
		t.Fatalf("status = %s, want %s", got, common.Status_STATUS_CANCELLED)
	}
	if repo.run.GetEntity().GetTimings().GetDeletedAt() == nil {
		t.Fatal("deleted_at was not set")
	}
	if repo.run.GetSummary().GetFinishedAt() == nil {
		t.Fatal("finished_at was not set")
	}
	if repo.updates != 1 {
		t.Fatalf("updates = %d, want 1", repo.updates)
	}
}

type noopTrm struct{}

func (noopTrm) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func (noopTrm) DoWithSettings(ctx context.Context, _ trm.Settings, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type fakeTestRunRepo struct {
	run     *models.TestRunRecord
	updates int
}

func (r *fakeTestRunRepo) Create(_ context.Context, run *models.TestRunRecord) error {
	r.run = run
	return nil
}

func (r *fakeTestRunRepo) Get(_ context.Context, tenantID, id string) (*models.TestRunRecord, error) {
	if r.run.GetEntity().GetTenantId() != tenantID || r.run.GetEntity().GetId() != id {
		return nil, derrors.ErrNotFound
	}
	return r.run, nil
}

func (r *fakeTestRunRepo) List(context.Context, *api.ListTestRunsRequest, string) ([]*models.TestRunRecord, string, error) {
	return nil, "", nil
}

func (r *fakeTestRunRepo) ListFacets(context.Context, *api.ListTestRunFacetsRequest, string) (*api.ListTestRunFacetsResponse, error) {
	return &api.ListTestRunFacetsResponse{}, nil
}

func (r *fakeTestRunRepo) Update(_ context.Context, run *models.TestRunRecord) error {
	r.updates++
	r.run = run
	return nil
}

func (r *fakeTestRunRepo) Delete(context.Context, string, string) error {
	return nil
}

type fakeMissingWorkflows struct{}

func (fakeMissingWorkflows) LaunchTest(context.Context, *models.TestRunRecord) error {
	return nil
}

func (fakeMissingWorkflows) CancelTest(context.Context, string) error {
	return derrors.ErrNotFound
}

type fakeLaunchFailWorkflows struct{}

func (fakeLaunchFailWorkflows) LaunchTest(context.Context, *models.TestRunRecord) error {
	return errors.New("temporal unavailable")
}

func (fakeLaunchFailWorkflows) CancelTest(context.Context, string) error {
	return nil
}

type fakeSummarizer struct{}

func (fakeSummarizer) Summarize(*domain.TestRun) *models.TestRunRecord_Summary {
	return &models.TestRunRecord_Summary{}
}

type fakeAuthn struct{}

func (fakeAuthn) Caller(context.Context) (*iam.AccessClaims, error) {
	return &iam.AccessClaims{AccountId: "account-1"}, nil
}
