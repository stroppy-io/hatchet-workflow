package adapters

import (
	"context"
	"math"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
)

func TestMetricsComparatorHonorsMetricDirection(t *testing.T) {
	metrics := fakeRunMetricsGetter{
		byRunID: map[string]*monitor.RunMetrics{
			"base": {
				RunId: "base",
				Metrics: []*monitor.MetricSummary{
					{Key: "latency", Name: "Latency p99", Avg: 100, Max: 150, Unit: "ms", HigherIsBetter: false, Group: "latency"},
					{Key: "ops", Name: "Operations", Avg: 1000, Max: 1200, Unit: "ops/s", HigherIsBetter: true, Group: "throughput"},
				},
			},
			"slower": {
				RunId: "slower",
				Metrics: []*monitor.MetricSummary{
					{Key: "latency", Name: "Latency p99", Avg: 120, Max: 170, Unit: "ms", HigherIsBetter: false, Group: "latency"},
					{Key: "ops", Name: "Operations", Avg: 1100, Max: 1300, Unit: "ops/s", HigherIsBetter: true, Group: "throughput"},
				},
			},
			"faster": {
				RunId: "faster",
				Metrics: []*monitor.MetricSummary{
					{Key: "latency", Name: "Latency p99", Avg: 80, Max: 110, Unit: "ms", HigherIsBetter: false, Group: "latency"},
					{Key: "ops", Name: "Operations", Avg: 950, Max: 1150, Unit: "ops/s", HigherIsBetter: true, Group: "throughput"},
				},
			},
		},
	}

	cmp, err := NewMetricsComparator(metrics).Compare(context.Background(), []string{"base", "slower", "faster"})
	if err != nil {
		t.Fatalf("compare: %v", err)
	}

	latency := mustMetricRow(t, cmp, "latency")
	if latency.GetHigherIsBetter() {
		t.Fatal("latency must be lower-is-better")
	}
	assertCell(t, latency.GetCells()[1], monitor.Verdict_VERDICT_WORSE, true, true, 120, 20)
	assertCell(t, latency.GetCells()[2], monitor.Verdict_VERDICT_BETTER, true, true, 80, -20)

	ops := mustMetricRow(t, cmp, "ops")
	if !ops.GetHigherIsBetter() {
		t.Fatal("ops must be higher-is-better")
	}
	assertCell(t, ops.GetCells()[1], monitor.Verdict_VERDICT_BETTER, true, true, 1100, 10)
	assertCell(t, ops.GetCells()[2], monitor.Verdict_VERDICT_WORSE, true, true, 950, -5)

	assertSummary(t, cmp.GetSummaries()[0], "slower", 1, 1, 0, 0, 0)
	assertSummary(t, cmp.GetSummaries()[1], "faster", 1, 1, 0, 0, 0)
}

func TestMetricsComparatorMarksMissingAndUncomparableMetrics(t *testing.T) {
	metrics := fakeRunMetricsGetter{
		byRunID: map[string]*monitor.RunMetrics{
			"base": {
				RunId: "base",
				Metrics: []*monitor.MetricSummary{
					{Key: "errors", Name: "Errors", Avg: 0, Max: 0, Unit: "err/s", HigherIsBetter: false, Group: "errors"},
					{Key: "base_only", Name: "Base-only metric", Avg: 10, Max: 12, HigherIsBetter: true},
				},
			},
			"candidate": {
				RunId: "candidate",
				Metrics: []*monitor.MetricSummary{
					{Key: "errors", Name: "Errors", Avg: 2, Max: 3, Unit: "err/s", HigherIsBetter: false, Group: "errors"},
					{Key: "candidate_only", Name: "Candidate-only metric", Avg: 5, Max: 7, HigherIsBetter: true},
				},
			},
		},
	}

	cmp, err := NewMetricsComparator(metrics).Compare(context.Background(), []string{"base", "candidate"})
	if err != nil {
		t.Fatalf("compare: %v", err)
	}

	errorsRow := mustMetricRow(t, cmp, "errors")
	errorsCell := errorsRow.GetCells()[1]
	assertCell(t, errorsCell, monitor.Verdict_VERDICT_WORSE, true, false, 2, 0)
	if errorsCell.GetDiffMaxPctDefined() {
		t.Fatal("max diff should be undefined when baseline max is zero and candidate max is non-zero")
	}

	baseOnly := mustMetricRow(t, cmp, "base_only")
	missingCell := baseOnly.GetCells()[1]
	if missingCell.GetPresent() {
		t.Fatal("candidate should be marked absent for base_only metric")
	}
	if got := missingCell.GetVerdict(); got != monitor.Verdict_VERDICT_UNSPECIFIED {
		t.Fatalf("missing verdict = %s, want unspecified", got)
	}

	candidateOnly := mustMetricRow(t, cmp, "candidate_only")
	if candidateOnly.GetCells()[0].GetPresent() {
		t.Fatal("baseline should be marked absent for candidate_only metric")
	}
	extraCell := candidateOnly.GetCells()[1]
	if !extraCell.GetPresent() {
		t.Fatal("candidate should be marked present for candidate_only metric")
	}
	if got := extraCell.GetVerdict(); got != monitor.Verdict_VERDICT_UNSPECIFIED {
		t.Fatalf("not comparable verdict = %s, want unspecified", got)
	}

	assertSummary(t, cmp.GetSummaries()[0], "candidate", 0, 1, 0, 1, 1)
}

func mustMetricRow(t *testing.T, cmp *monitor.Comparison, key string) *monitor.MetricRow {
	t.Helper()
	for _, row := range cmp.GetMetrics() {
		if row.GetKey() == key {
			return row
		}
	}
	t.Fatalf("metric row %q not found", key)
	return nil
}

func assertCell(
	t *testing.T,
	cell *monitor.MetricCell,
	wantVerdict monitor.Verdict,
	wantPresent bool,
	wantDiffDefined bool,
	wantAvg float64,
	wantDiffAvgPct float64,
) {
	t.Helper()
	if got := cell.GetPresent(); got != wantPresent {
		t.Fatalf("present = %t, want %t", got, wantPresent)
	}
	if got := cell.GetVerdict(); got != wantVerdict {
		t.Fatalf("verdict = %s, want %s", got, wantVerdict)
	}
	if got := cell.GetDiffAvgPctDefined(); got != wantDiffDefined {
		t.Fatalf("diff avg defined = %t, want %t", got, wantDiffDefined)
	}
	if math.Abs(cell.GetAvg()-wantAvg) > 0.0001 {
		t.Fatalf("avg = %f, want %f", cell.GetAvg(), wantAvg)
	}
	if math.Abs(cell.GetDiffAvgPct()-wantDiffAvgPct) > 0.0001 {
		t.Fatalf("diff avg pct = %f, want %f", cell.GetDiffAvgPct(), wantDiffAvgPct)
	}
}

func assertSummary(
	t *testing.T,
	summary *monitor.Comparison_RunSummary,
	runID string,
	better uint32,
	worse uint32,
	same uint32,
	missing uint32,
	notComparable uint32,
) {
	t.Helper()
	if summary.GetRunId() != runID ||
		summary.GetBetter() != better ||
		summary.GetWorse() != worse ||
		summary.GetSame() != same ||
		summary.GetMissing() != missing ||
		summary.GetNotComparable() != notComparable {
		t.Fatalf(
			"summary = {run:%s better:%d worse:%d same:%d missing:%d not_comparable:%d}, want {run:%s better:%d worse:%d same:%d missing:%d not_comparable:%d}",
			summary.GetRunId(),
			summary.GetBetter(),
			summary.GetWorse(),
			summary.GetSame(),
			summary.GetMissing(),
			summary.GetNotComparable(),
			runID,
			better,
			worse,
			same,
			missing,
			notComparable,
		)
	}
}
