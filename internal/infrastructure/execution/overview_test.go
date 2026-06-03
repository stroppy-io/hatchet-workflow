package execution

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestOverviewGetFallsBackToPersistedTerminalRecord(t *testing.T) {
	started := timestamppb.New(time.Unix(10, 0))
	finished := timestamppb.New(time.Unix(70, 0))
	reader := &OverviewReader{
		tc: fakeRunStateQuerier{err: errors.New("workflow closed")},
		store: fakeSnapshotStore{
			run: &models.TestRunRecord{
				Entity:     &common.Entity{Id: "run-1"},
				Status:     common.Status_STATUS_COMPLETED,
				SuiteRunId: "suite-run-1",
				Summary: &models.TestRunRecord_Summary{
					StartedAt:   started,
					FinishedAt:  finished,
					Duration:    durationpb.New(time.Minute),
					ProgressPct: 100,
				},
			},
			suite: &models.SuiteRunRecord{
				Entity: &common.Entity{Id: "suite-run-1"},
				Status: common.Status_STATUS_COMPLETED,
			},
		},
	}

	snap, err := reader.Get(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("get overview: %v", err)
	}

	if got := snap.GetOverview().GetStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("overview status = %s, want %s", got, common.Status_STATUS_COMPLETED)
	}
	if got := snap.GetOverview().GetProgressPct(); got != 100 {
		t.Fatalf("overview progress = %d, want 100", got)
	}
	if got := snap.GetOverview().GetStartedAt(); got != started {
		t.Fatalf("overview started_at = %v, want persisted value", got)
	}
	if got := snap.GetRun().GetStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("run status = %s, want %s", got, common.Status_STATUS_COMPLETED)
	}
	if got := snap.GetSuiteRun().GetStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("suite run status = %s, want %s", got, common.Status_STATUS_COMPLETED)
	}
}

func TestOverviewStreamClosesAfterPersistedTerminalSnapshot(t *testing.T) {
	reader := &OverviewReader{
		tc: fakeRunStateQuerier{err: errors.New("workflow closed")},
		store: fakeSnapshotStore{
			run: &models.TestRunRecord{
				Entity: &common.Entity{Id: "run-1"},
				Status: common.Status_STATUS_COMPLETED,
				Summary: &models.TestRunRecord_Summary{
					ProgressPct: 100,
				},
			},
		},
	}

	ch, err := reader.Stream(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("stream overview: %v", err)
	}

	select {
	case snap, ok := <-ch:
		if !ok {
			t.Fatal("stream closed before emitting snapshot")
		}
		if got := snap.GetOverview().GetStatus(); got != common.Status_STATUS_COMPLETED {
			t.Fatalf("overview status = %s, want %s", got, common.Status_STATUS_COMPLETED)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for stream snapshot")
	}

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("stream stayed open after terminal snapshot")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for stream close")
	}
}

type fakeRunStateQuerier struct {
	state *workflowpb.RunState
	err   error
}

func (f fakeRunStateQuerier) GetRunState(context.Context, string, string) (*workflowpb.RunState, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.state, nil
}

type fakeSnapshotStore struct {
	run   *models.TestRunRecord
	suite *models.SuiteRunRecord
}

func (f fakeSnapshotStore) RunRecord(context.Context, string) (*models.TestRunRecord, error) {
	return f.run, nil
}

func (f fakeSnapshotStore) SuiteRun(context.Context, string) (*models.SuiteRunRecord, error) {
	return f.suite, nil
}
