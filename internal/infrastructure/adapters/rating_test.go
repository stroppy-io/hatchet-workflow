package adapters

import (
	"context"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"github.com/stroppy-io/stroppy-cloud/internal/services/rating"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestRankRunsSkipsSoftDeletedRuns(t *testing.T) {
	deletedAt := timestamppb.Now()
	runs := []*models.TestRunRecord{
		{
			Entity:  &common.Entity{Id: "deleted", Timings: &common.Timings{DeletedAt: deletedAt}},
			Summary: &models.TestRunRecord_Summary{},
		},
		{
			Entity:  &common.Entity{Id: "active", Timings: &common.Timings{}},
			Summary: &models.TestRunRecord_Summary{},
		},
	}
	metrics := fakeRunMetricsGetter{
		byRunID: map[string]*monitor.RunMetrics{
			"deleted": {RunId: "deleted", Metrics: []*monitor.MetricSummary{{Key: "ops", Avg: 100, HigherIsBetter: true}}},
			"active":  {RunId: "active", Metrics: []*monitor.MetricSummary{{Key: "ops", Avg: 10, HigherIsBetter: true}}},
		},
	}

	ranked, found, err := rankRuns(context.Background(), metrics, runs, ratingFacets{metricKey: "ops"})
	if err != nil {
		t.Fatalf("rank runs: %v", err)
	}
	if !found {
		t.Fatal("metric was not found")
	}
	if len(ranked) != 1 {
		t.Fatalf("ranked runs = %d, want 1", len(ranked))
	}
	if got := ranked[0].run.GetEntity().GetId(); got != "active" {
		t.Fatalf("ranked run = %s, want active", got)
	}
}

func TestRatingBoardFallsBackFromDbTpsToDbQps(t *testing.T) {
	board := NewRatingBoard(
		fakeRatingRunsLister{
			runs: []*models.TestRunRecord{{
				Entity:  &common.Entity{Id: "run-a", Timings: &common.Timings{}},
				Summary: &models.TestRunRecord_Summary{},
			}},
		},
		fakeRunMetricsGetter{
			byRunID: map[string]*monitor.RunMetrics{
				"run-a": {RunId: "run-a", Metrics: []*monitor.MetricSummary{
					{Key: "db_qps", Avg: 120, Unit: "q/s", HigherIsBetter: true},
				}},
			},
		},
		nil,
	)

	entries, _, err := board.Rank(context.Background(), ratingQuery("db_tps"))
	if err != nil {
		t.Fatalf("rank: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if got := entries[0].GetMetricValue(); got != 120 {
		t.Fatalf("metric value = %v, want 120", got)
	}
}

func TestPublicRatingBoardAllowsUnsetTimeWindow(t *testing.T) {
	board := NewPublicRatingBoard(
		fakeRatingRunsLister{
			runs: []*models.TestRunRecord{{
				Entity:  &common.Entity{Id: "run-a", Timings: &common.Timings{}},
				Summary: &models.TestRunRecord_Summary{},
			}},
		},
		fakeRunMetricsGetter{
			byRunID: map[string]*monitor.RunMetrics{
				"run-a": {RunId: "run-a", Metrics: []*monitor.MetricSummary{
					{Key: "db_tps", Avg: 80, Unit: "txn/s", HigherIsBetter: true},
				}},
			},
		},
	)

	entries, _, err := board.Public(context.Background(), &api.RatingFilter{MetricKey: "db_tps"}, 10, "")
	if err != nil {
		t.Fatalf("public rating: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
}

func ratingQuery(metricKey string) rating.RatingQuery {
	return rating.RatingQuery{MetricKey: metricKey}
}

type fakeRunMetricsGetter struct {
	byRunID map[string]*monitor.RunMetrics
}

func (g fakeRunMetricsGetter) Get(_ context.Context, runID string) (*monitor.RunMetrics, error) {
	return g.byRunID[runID], nil
}
