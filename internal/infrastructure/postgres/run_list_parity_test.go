package postgres

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
)

// TestRunListParity_AllSummaryFacetsAndSortKindsPreserved guards spec §6.A:
// every Summary facet RunVM/ListTestRunFacetsResponse renders must still map
// to the expected jsonb path 1:1 on run_records (RunRepo.List's WHERE/ORDER
// builder), for every ListTestRunsRequest_Sort_Kind and every filter facet
// honored by runListWhereClauses/buildOrderBy, and suite_run_id/
// suite_cell_id/standalone must produce NO WHERE clause (dropped per Task
// 2 Step 4/5, spec §6.A "dropped" row).
func TestRunListParity_AllSummaryFacetsAndSortKindsPreserved(t *testing.T) {
	t.Run("sort kinds map 1:1 onto Run.summary jsonb paths", func(t *testing.T) {
		cases := []struct {
			kind     api.ListTestRunsRequest_Sort_Kind
			wantPath string
		}{
			{api.ListTestRunsRequest_Sort_KIND_STATUS, jStatus},
			{api.ListTestRunsRequest_Sort_KIND_DB_KIND, jDBKind},
			{api.ListTestRunsRequest_Sort_KIND_WORKLOAD, jWorkloadName},
			{api.ListTestRunsRequest_Sort_KIND_PROVIDER, jProvider},
			{api.ListTestRunsRequest_Sort_KIND_PROGRESS, jProgressPct},
			{api.ListTestRunsRequest_Sort_KIND_DURATION, jDurationSecs},
			{api.ListTestRunsRequest_Sort_KIND_STARTED_AT, jStartedAtTs},
			{api.ListTestRunsRequest_Sort_KIND_FINISHED_AT, jFinishedAtTs},
			{api.ListTestRunsRequest_Sort_KIND_NODE_COUNT, jNodeCount},
			{api.ListTestRunsRequest_Sort_KIND_PROTOCOL, jProtocol},
			{api.ListTestRunsRequest_Sort_KIND_TEST_PRESET, jTestPresetID},
			{api.ListTestRunsRequest_Sort_KIND_TRIGGER, jTrigger},
		}
		for _, c := range cases {
			sort := &api.ListTestRunsRequest_Sort{
				By:   &api.ListTestRunsRequest_Sort_Kind_{Kind: c.kind},
				Desc: true,
			}
			orderBy := buildOrderBy(sort)
			if !strings.Contains(orderBy, c.wantPath) {
				t.Errorf("§6.A parity: sort kind %v produced ORDER BY %q, want it to reference jsonb path %q", c.kind, orderBy, c.wantPath)
			}
		}
	})

	t.Run("filter facets map 1:1 onto Run.summary/entity jsonb paths", func(t *testing.T) {
		b := &argBuilder{}
		query := &api.ListTestRunsRequest{
			TenantId: "t1",
			Filter: &commonpb.EntityFilter{
				AuthorIds: []string{"author-1"},
			},
			StroppyVersions:   []string{"1.2.3"},
			DbPresetIds:       []string{"preset-db"},
			WorkloadPresetIds: []string{"preset-workload"},
			TestPresetIds:     []string{"preset-test"},
		}
		where := runListWhereClauses(b, query)
		joined := strings.Join(where, " AND ")

		for _, want := range []struct {
			label string
			path  string
		}{
			{"author_ids", jAuthorID},
			{"stroppy_versions", jStroppyVersion},
			{"db_preset_ids", jDBPresetID},
			{"workload_preset_ids", jWorkloadPreset},
			{"test_preset_ids", jTestPresetID},
		} {
			if !strings.Contains(joined, want.path) {
				t.Errorf("§6.A parity: filter facet %s did not produce a WHERE clause referencing %q; got clauses: %v", want.label, want.path, where)
			}
		}
	})

	t.Run("suite_run_id/suite_cell_id/standalone are silently dropped", func(t *testing.T) {
		b := &argBuilder{}
		where := runListWhereClauses(b, &api.ListTestRunsRequest{
			TenantId:     "t1",
			SuiteRunId:   "suite-1",
			SuiteCellIds: []string{"cell-1", "cell-2"},
			Standalone:   proto.Bool(true),
		})
		joined := strings.Join(where, " AND ")
		if strings.Contains(joined, "suiteRunId") || strings.Contains(joined, "suiteCellId") || strings.Contains(joined, "standalone") {
			t.Errorf("§6.A parity: dropped fields suite_run_id/suite_cell_id/standalone must NOT produce a WHERE clause; got: %v", where)
		}
		// Only the tenant-scope + soft-delete clauses should be present.
		if len(where) != 2 {
			t.Errorf("§6.A parity: expected exactly tenant-scope + soft-delete clauses (no suite clause), got %d clauses: %v", len(where), where)
		}
	})
}
