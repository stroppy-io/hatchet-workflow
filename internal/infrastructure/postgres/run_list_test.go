package postgres

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
)

func TestRunListStripsHeavyColumnsFromRunRecords(t *testing.T) {
	// Guards the SELECT projection: run_records (not test_run_records), and
	// strips runtimeState + compiledPlan (Run's heavy blobs), not
	// deploymentPlan/infrastructureState (TestRunRecord's, which Run doesn't
	// have).
	sb := runListBaseSelect()
	if !strings.Contains(sb, "FROM run_records") {
		t.Fatalf("select %q does not target run_records", sb)
	}
	if !strings.Contains(sb, "- 'runtimeState'") || !strings.Contains(sb, "- 'compiledPlan'") {
		t.Fatalf("select %q does not strip runtimeState/compiledPlan", sb)
	}
	if strings.Contains(sb, "deploymentPlan") || strings.Contains(sb, "infrastructureState") {
		t.Fatalf("select %q references TestRunRecord-only columns", sb)
	}
}

func TestRunListOrderByDropsSuiteAndKeepsSummaryFacets(t *testing.T) {
	// buildOrderBy is shared with test_run_list.go unchanged (Kind switch is
	// identical); this test only guards that RunRepo.List's WHERE-builder no
	// longer emits a jSuiteRunID/jSuiteCellID clause.
	b := &argBuilder{}
	where := runListWhereClauses(b, &api.ListTestRunsRequest{
		TenantId:   "t1",
		SuiteRunId: "suite-1",        // must be ignored
		Standalone: proto.Bool(true), // must be ignored
	})
	for _, w := range where {
		if strings.Contains(w, "suiteRunId") || strings.Contains(w, "suiteCellId") {
			t.Fatalf("RunRepo.List WHERE clause %q still references dropped suite fields", w)
		}
	}
}

func TestRunListWhereClausesTargetsTenantScope(t *testing.T) {
	b := &argBuilder{}
	where := runListWhereClauses(b, &api.ListTestRunsRequest{TenantId: "t1"})
	if len(where) == 0 {
		t.Fatalf("expected at least the tenant-scope clause")
	}
	if !strings.Contains(where[0], "tenant_id") {
		t.Fatalf("first WHERE clause %q does not scope by tenant_id", where[0])
	}
	if len(b.args) == 0 || b.args[0] != "t1" {
		t.Fatalf("args = %v, want first arg to be tenant id", b.args)
	}
}
