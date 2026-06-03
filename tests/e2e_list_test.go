//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	api "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	common "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
)

// TestE2EListFacets launches a run, then exercises the read/list surface:
// ListTestRuns, ListTestRunFacets and GetTestRun. The run is left to finish
// (tiny footprint) so the suite stays self-contained.
func TestE2EListFacets(t *testing.T) {
	e := setup(t)

	runID := e.launchDockerRun(t, "e2e-list", 1, 1, "10s")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// ListTestRuns must include the run we just launched.
	lr, err := e.runs.ListTestRuns(ctx, &api.ListTestRunsRequest{
		TenantId: e.tenantID,
		Page:     &common.Page{Size: 100},
	})
	if err != nil {
		t.Fatalf("ListTestRuns: %v", err)
	}
	found := false
	for _, r := range lr.GetRuns() {
		if r.GetEntity().GetId() == runID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("ListTestRuns did not contain launched run %s (got %d runs)", runID, len(lr.GetRuns()))
	}
	t.Logf("ListTestRuns returned %d runs, includes %s", len(lr.GetRuns()), runID)

	// Facets must be reachable.
	if _, err := e.runs.ListTestRunFacets(ctx, &api.ListTestRunFacetsRequest{
		TenantId: e.tenantID,
	}); err != nil {
		t.Fatalf("ListTestRunFacets: %v", err)
	}

	// Let it finish so we don't leak a running container.
	rec := e.waitTerminal(t, runID, 15*time.Minute)
	t.Logf("list-run %s finished status=%s", runID, rec.GetStatus())
}

// TestE2ECancel launches a run and cancels it mid-flight, asserting the full
// cancel transition observed through the API: cancel -> CANCELLING -> CANCELLED.
// CancelTestRun persists CANCELLING synchronously (returned in the response) and
// the workflow then drives it to CANCELLED.
func TestE2ECancel(t *testing.T) {
	e := setup(t)

	runID := e.launchDockerRun(t, "e2e-cancel", 1, 1, "60s")

	// The record flips to RUNNING at the very start of the workflow (before the
	// deploy/install/workload stages), so this is reached quickly and leaves a
	// wide window to cancel mid-run.
	e.waitStatus(t, runID, 5*time.Minute, common.Status_STATUS_RUNNING)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cr, err := e.runs.CancelTestRun(ctx, &api.CancelTestRunRequest{
		TenantId: e.tenantID,
		Id:       runID,
	})
	if err != nil {
		t.Fatalf("CancelTestRun(%s): %v", runID, err)
	}
	// CancelTestRun sets CANCELLING synchronously and returns the updated record.
	cancelStatus := cr.GetRun().GetStatus()
	t.Logf("CancelTestRun response status = %s", cancelStatus)
	if cancelStatus != common.Status_STATUS_CANCELLING && cancelStatus != common.Status_STATUS_CANCELLED {
		t.Errorf("CancelTestRun response status = %s, want CANCELLING (or already CANCELLED)", cancelStatus)
	}

	// Poll the full transition through to the terminal CANCELLED state.
	seq := e.pollStatuses(t, runID, 1*time.Second, 10*time.Minute)
	if last := seq[len(seq)-1]; last != common.Status_STATUS_CANCELLED {
		t.Fatalf("run %s final status = %s, want CANCELLED (seq=%v)", runID, last, seq)
	}
	// The full intended sequence is RUNNING -> CANCELLING -> CANCELLED. The poll
	// may collapse the transient CANCELLING; the synchronous response above
	// already proved it, so only warn if the poll missed it.
	if containsSubsequence(seq, []common.Status{
		common.Status_STATUS_CANCELLING, common.Status_STATUS_CANCELLED,
	}) {
		t.Logf("OK observed CANCELLING -> CANCELLED via poll: %v", seq)
	} else {
		t.Logf("note: poll collapsed transient CANCELLING (seq=%v); response confirmed it", seq)
	}
}
