package adapters

import (
	"context"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
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

type fakeRunMetricsGetter struct {
	byRunID map[string]*monitor.RunMetrics
}

func (g fakeRunMetricsGetter) Get(_ context.Context, runID string) (*monitor.RunMetrics, error) {
	return g.byRunID[runID], nil
}
