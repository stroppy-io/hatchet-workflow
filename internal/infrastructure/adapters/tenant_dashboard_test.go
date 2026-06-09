package adapters

import (
	"context"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
)

func TestDashboardRatingReaderFallsBackToAvailableThroughputMetric(t *testing.T) {
	runs := fakeRatingRunsLister{
		runs: []*models.TestRunRecord{
			{
				Entity: &common.Entity{Id: "run-a", TenantId: "tenant-a", Timings: &common.Timings{}},
				Summary: &models.TestRunRecord_Summary{
					WorkloadName: "workload",
				},
			},
		},
	}
	metrics := fakeRunMetricsGetter{
		byRunID: map[string]*monitor.RunMetrics{
			"run-a": {
				RunId: "run-a",
				Metrics: []*monitor.MetricSummary{
					{Key: "db_qps", Avg: 42, Unit: "q/s", HigherIsBetter: true},
				},
			},
		},
	}
	board := NewRatingBoard(runs, metrics, nil)
	reader := NewDashboardRatingReader(board, "db_tps")

	entries, err := reader.TopBenchmarks(context.Background(), "tenant-a", 10)
	if err != nil {
		t.Fatalf("top benchmarks: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if got := entries[0].GetMetricValue(); got != 42 {
		t.Fatalf("metric value = %v, want 42", got)
	}
	if got := entries[0].GetMetricUnit(); got != "q/s" {
		t.Fatalf("metric unit = %q, want q/s", got)
	}
}

type fakeRatingRunsLister struct {
	runs []*models.TestRunRecord
}

func (l fakeRatingRunsLister) ListRatingRuns(context.Context, RatingRunsScope, string) ([]*models.TestRunRecord, error) {
	return l.runs, nil
}
