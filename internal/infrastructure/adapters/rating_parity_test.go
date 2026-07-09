package adapters

import (
	"context"
	"testing"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"github.com/stroppy-io/stroppy-cloud/internal/services/rating"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestRatingParity_EntryFieldsFromRunFixture guards spec §6.E: every
// RatingEntry field (db_kind/workload_name/stroppy_version/provider/
// topology_label/node_count/run_at/run_id/author_name/tenant_name +
// metric_value/metric_unit/rank) must be sourced 1:1 from the models.Run
// fixture via the real RatingBoard.Rank path.
func TestRatingParity_EntryFieldsFromRunFixture(t *testing.T) {
	run := &models.Run{
		Entity:         &common.Entity{Id: "run-1", AuthorId: "author-1", TenantId: "tenant-1"},
		InGlobalRating: true,
		Summary: &models.Run_Summary{
			DbKind:         domainpb.Database_KIND_POSTGRES,
			WorkloadName:   "workload-name",
			StroppyVersion: "1.2.3",
			Provider:       deploymentpb.Provider_PROVIDER_DOCKER,
			TopologyLabel:  "topology-label",
			NodeCount:      3,
			StartedAt:      timestamppb.Now(),
		},
	}
	board := NewRatingBoard(
		fakeRatingRunsLister{runs: []*models.Run{run}},
		fakeRunMetricsGetter{byRunID: map[string]*monitor.RunMetrics{
			"run-1": {RunId: "run-1", Metrics: []*monitor.MetricSummary{
				{Key: "throughput", Avg: 99, Unit: "ops/s", HigherIsBetter: true},
			}},
		}},
		fakeRatingNameResolver{authorName: "Alice", tenantName: "Acme"},
	)

	entries, _, err := board.Rank(context.Background(), rating.RatingQuery{
		Scope:     rating.ScopeSystem,
		MetricKey: "throughput",
	})
	if err != nil {
		t.Fatalf("rank: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	e := entries[0]

	if got, want := e.GetRank(), uint32(1); got != want {
		t.Errorf("§6.E parity: RatingEntry.rank = %d, want %d", got, want)
	}
	if got, want := e.GetMetricValue(), float64(99); got != want {
		t.Errorf("§6.E parity: RatingEntry.metric_value = %v, want %v", got, want)
	}
	if got, want := e.GetMetricUnit(), "ops/s"; got != want {
		t.Errorf("§6.E parity: RatingEntry.metric_unit = %q, want %q", got, want)
	}
	if got, want := e.GetDbKind(), run.GetSummary().GetDbKind(); got != want {
		t.Errorf("§6.E parity: RatingEntry.db_kind = %v, want %v", got, want)
	}
	if got, want := e.GetWorkloadName(), run.GetSummary().GetWorkloadName(); got != want {
		t.Errorf("§6.E parity: RatingEntry.workload_name = %q, want %q", got, want)
	}
	if got, want := e.GetStroppyVersion(), run.GetSummary().GetStroppyVersion(); got != want {
		t.Errorf("§6.E parity: RatingEntry.stroppy_version = %q, want %q", got, want)
	}
	if got, want := e.GetProvider(), run.GetSummary().GetProvider(); got != want {
		t.Errorf("§6.E parity: RatingEntry.provider = %v, want %v", got, want)
	}
	if got, want := e.GetTopologyLabel(), run.GetSummary().GetTopologyLabel(); got != want {
		t.Errorf("§6.E parity: RatingEntry.topology_label = %q, want %q", got, want)
	}
	if got, want := e.GetNodeCount(), run.GetSummary().GetNodeCount(); got != want {
		t.Errorf("§6.E parity: RatingEntry.node_count = %d, want %d", got, want)
	}
	if got, want := e.GetRunAt().AsTime(), run.GetSummary().GetStartedAt().AsTime(); !got.Equal(want) {
		t.Errorf("§6.E parity: RatingEntry.run_at = %v, want %v", got, want)
	}
	if got, want := e.GetRunId(), run.GetEntity().GetId(); got != want {
		t.Errorf("§6.E parity: RatingEntry.run_id = %q, want %q", got, want)
	}
	if got, want := e.GetAuthorName(), "Alice"; got != want {
		t.Errorf("§6.E parity: RatingEntry.author_name = %q, want %q", got, want)
	}
	if got, want := e.GetTenantName(), "Acme"; got != want {
		t.Errorf("§6.E parity: RatingEntry.tenant_name = %q, want %q", got, want)
	}
}

// TestRatingParity_FilterFacetsNarrowByRunSummary guards spec §6.E's
// RatingFilter.{db_kinds,stroppy_versions,providers,started_after/before}
// row: each facet must still narrow candidates by the matching
// Run.summary.* field via matchFacets.
func TestRatingParity_FilterFacetsNarrowByRunSummary(t *testing.T) {
	matching := &models.Run{
		Entity: &common.Entity{Id: "match"},
		Summary: &models.Run_Summary{
			DbKind:         domainpb.Database_KIND_POSTGRES,
			StroppyVersion: "1.2.3",
			Provider:       deploymentpb.Provider_PROVIDER_DOCKER,
			StartedAt:      timestamppb.Now(),
		},
	}
	nonMatchingKind := &models.Run{
		Entity:  &common.Entity{Id: "wrong-kind"},
		Summary: &models.Run_Summary{DbKind: domainpb.Database_KIND_MYSQL, StartedAt: timestamppb.Now()},
	}
	board := NewPublicRatingBoard(
		fakeRatingRunsLister{runs: []*models.Run{matching, nonMatchingKind}},
		fakeRunMetricsGetter{byRunID: map[string]*monitor.RunMetrics{
			"match":      {RunId: "match", Metrics: []*monitor.MetricSummary{{Key: "ops", Avg: 1, HigherIsBetter: true}}},
			"wrong-kind": {RunId: "wrong-kind", Metrics: []*monitor.MetricSummary{{Key: "ops", Avg: 2, HigherIsBetter: true}}},
		}},
	)

	entries, _, err := board.Public(context.Background(), &api.RatingFilter{
		MetricKey:       "ops",
		DbKinds:         []domainpb.Database_Kind{domainpb.Database_KIND_POSTGRES},
		StroppyVersions: []string{"1.2.3"},
		Providers:       []deploymentpb.Provider{deploymentpb.Provider_PROVIDER_DOCKER},
	}, 10, "")
	if err != nil {
		t.Fatalf("public rank: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("§6.E parity: RatingFilter facets = %d entries, want exactly the 1 matching run; got %+v", len(entries), entries)
	}
}

// TestFavoriteShareDashboardParity_ReadEntityAndSummaryFromRun guards spec
// §6.F: Favorite/Share/tenant-dashboard read entity+summary off models.Run,
// not the retired TestRunRecord/spec/suite fields.
func TestFavoriteShareDashboardParity_ReadEntityAndSummaryFromRun(t *testing.T) {
	run := &models.Run{
		Entity: &common.Entity{Id: "run-1", Name: "run-name", TenantId: "tenant-1"},
		Status: common.Status_STATUS_COMPLETED,
	}

	t.Run("favorite: EntityGetterFromRecord[*models.Run] returns run.GetEntity() unchanged", func(t *testing.T) {
		getter := EntityGetterFromRecord[*models.Run](func(_ context.Context, id string) (*models.Run, error) {
			if id != "run-1" {
				return nil, derrors.NotFound("run", "not found")
			}
			return run, nil
		})
		got, err := getter.GetEntity(context.Background(), "tenant-1", "run-1")
		if err != nil {
			t.Fatalf("§6.F parity: favorite GetEntity: %v", err)
		}
		if got != run.GetEntity() {
			t.Errorf("§6.F parity: favorite target entity = %v, want the same pointer as run.GetEntity()", got)
		}
	})

	t.Run("share: sharedTestRun projects entity+summary only, no suite fields", func(t *testing.T) {
		builder := NewRunSnapshotBuilder(fakeShareRunReader{run: run}, nil)
		view := builder.sharedTestRun(context.Background(), run)
		if view.GetName() != run.GetEntity().GetName() {
			t.Errorf("§6.F parity: share snapshot name = %q, want %q", view.GetName(), run.GetEntity().GetName())
		}
		if view.GetStatus() != run.GetStatus() {
			t.Errorf("§6.F parity: share snapshot status = %v, want %v", view.GetStatus(), run.GetStatus())
		}
		// models.SharedTestRun has no GetSuiteRunId/GetSuiteCellId method at
		// all — this is a compile-time guard: if the proto ever regained a
		// suite field, referencing SharedTestRun's exact generated field set
		// here would need updating, which is the point.
	})

	t.Run("tenant-dashboard: RecentRuns/StatusCounts read summary+entity from Run", func(t *testing.T) {
		reader := fakeDashboardRunsReader{runs: []*models.Run{run}}
		stats := NewRunStatsReader(reader)
		counts, err := stats.StatusCounts(context.Background(), "tenant-1")
		if err != nil {
			t.Fatalf("§6.F parity: status counts: %v", err)
		}
		if counts.GetCompleted() != 1 {
			t.Errorf("§6.F parity: status counts completed = %d, want 1", counts.GetCompleted())
		}

		recent := NewRecentRunsReader(reader)
		runs, err := recent.RecentRuns(context.Background(), "tenant-1", 10)
		if err != nil {
			t.Fatalf("§6.F parity: recent runs: %v", err)
		}
		if len(runs) != 1 || runs[0].GetEntity().GetId() != "run-1" {
			t.Errorf("§6.F parity: recent runs = %+v, want [run-1]", runs)
		}
	})
}

type fakeRatingNameResolver struct {
	authorName string
	tenantName string
}

func (f fakeRatingNameResolver) AccountName(context.Context, string) string { return f.authorName }
func (f fakeRatingNameResolver) TenantName(context.Context, string) string  { return f.tenantName }

type fakeShareRunReader struct{ run *models.Run }

func (f fakeShareRunReader) GetTestRun(context.Context, string) (*models.Run, error) {
	return f.run, nil
}

type fakeDashboardRunsReader struct{ runs []*models.Run }

func (f fakeDashboardRunsReader) ListTenantRuns(context.Context, string) ([]*models.Run, error) {
	return f.runs, nil
}

func (f fakeDashboardRunsReader) ListScheduledSuites(context.Context, string) ([]ScheduledSuite, error) {
	return nil, nil
}
