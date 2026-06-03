//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"

	api "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	common "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
)

// TestE2EDelete launches a run, lets it complete, deletes it, and asserts the
// delete is observable through the API: GetTestRun reports NotFound and the run
// is excluded from ListTestRuns (soft delete).
func TestE2EDelete(t *testing.T) {
	e := setup(t)

	runID := e.launchDockerRun(t, "e2e-delete", 1, 1, "10s")
	rec := e.waitTerminal(t, runID, 15*time.Minute)
	if rec.GetStatus() != common.Status_STATUS_COMPLETED {
		t.Fatalf("run %s = %s, want COMPLETED before delete", runID, rec.GetStatus())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := e.runs.DeleteTestRun(ctx, &api.DeleteTestRunRequest{
		TenantId: e.tenantID,
		Id:       runID,
	}); err != nil {
		t.Fatalf("DeleteTestRun(%s): %v", runID, err)
	}

	// GetTestRun must now report NotFound.
	if _, err := e.runs.GetTestRun(ctx, &api.GetTestRunRequest{
		TenantId: e.tenantID,
		Id:       runID,
	}); err == nil {
		t.Fatal("GetTestRun after delete: expected error, got nil")
	} else if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("GetTestRun after delete: code = %s, want NotFound (%v)", connect.CodeOf(err), err)
	}

	// ListTestRuns must exclude the deleted run.
	lr, err := e.runs.ListTestRuns(ctx, &api.ListTestRunsRequest{
		TenantId: e.tenantID,
		Page:     &common.Page{Size: 200},
	})
	if err != nil {
		t.Fatalf("ListTestRuns: %v", err)
	}
	for _, r := range lr.GetRuns() {
		if r.GetEntity().GetId() == runID {
			t.Errorf("deleted run %s still present in ListTestRuns", runID)
		}
	}
	t.Logf("run %s deleted: GetTestRun=NotFound, excluded from list", runID)
}
