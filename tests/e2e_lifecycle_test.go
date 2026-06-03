//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	api "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	common "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
)

// TestE2ELifecycle verifies the full run status progression observed via the API
// (PENDING -> RUNNING -> COMPLETED) AND that the overview subscription stream
// (StreamTestRunOverview) emits the same progression and closes at the terminal
// state.
func TestE2ELifecycle(t *testing.T) {
	e := setup(t)

	rec := e.launchDockerRunRec(t, "e2e-lifecycle", 1, 1, "10s")
	runID := rec.GetEntity().GetId()

	// The launch response status is deterministically PENDING (the record is
	// created PENDING and the workflow flips it to RUNNING asynchronously).
	if rec.GetStatus() != common.Status_STATUS_PENDING {
		t.Errorf("launch response status = %s, want PENDING", rec.GetStatus())
	}

	// Subscribe to the overview stream concurrently; collect distinct statuses.
	streamCh := make(chan []common.Status, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 16*time.Minute)
		defer cancel()
		var seen []common.Status
		stream, err := e.overview.StreamTestRunOverview(ctx, &api.StreamTestRunOverviewRequest{
			TenantId: e.tenantID,
			RunId:    runID,
		})
		if err != nil {
			t.Errorf("StreamTestRunOverview: %v", err)
			streamCh <- seen
			return
		}
		for stream.Receive() {
			st := stream.Msg().GetRun().GetStatus()
			if len(seen) == 0 || seen[len(seen)-1] != st {
				seen = append(seen, st)
				t.Logf("stream %s -> %s", runID, st)
			}
		}
		if err := stream.Err(); err != nil {
			t.Logf("overview stream ended with: %v", err)
		}
		streamCh <- seen
	}()

	// Poll-based capture. Prepend the PENDING we deterministically saw at launch
	// (it is transient and a poll may miss it).
	polled := append([]common.Status{common.Status_STATUS_PENDING},
		e.pollStatuses(t, runID, 2*time.Second, 15*time.Minute)...)

	wantSeq := []common.Status{
		common.Status_STATUS_PENDING,
		common.Status_STATUS_RUNNING,
		common.Status_STATUS_COMPLETED,
	}
	if !containsSubsequence(polled, wantSeq) {
		t.Errorf("API status sequence %v does not contain %v", polled, wantSeq)
	}

	// The stream must have observed RUNNING -> COMPLETED and then closed.
	select {
	case seen := <-streamCh:
		if len(seen) == 0 {
			t.Fatal("overview stream produced no snapshots")
		}
		t.Logf("stream statuses: %v", seen)
		if !containsSubsequence(seen, []common.Status{
			common.Status_STATUS_RUNNING, common.Status_STATUS_COMPLETED,
		}) {
			t.Errorf("stream sequence %v does not contain RUNNING -> COMPLETED", seen)
		}
		if last := seen[len(seen)-1]; last != common.Status_STATUS_COMPLETED {
			t.Errorf("stream final status = %s, want COMPLETED", last)
		}
	case <-time.After(45 * time.Second):
		t.Fatal("overview stream did not close after the run completed")
	}
}
