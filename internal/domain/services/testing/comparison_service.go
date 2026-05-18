package testing

import (
	"context"
	"fmt"
	"math"

	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

// ComparisonService computes per-metric diffs between two TestRuns.
type ComparisonService struct {
	metrics       MetricsPort
	testRunLister TestRunSuiteLister
}

// NewComparisonService constructs a ComparisonService.
func NewComparisonService(metrics MetricsPort) *ComparisonService {
	return &ComparisonService{metrics: metrics}
}

// CompareRuns fetches metrics for runs a and b, computes abs and pct deltas,
// and assigns a verdict for each metric name.
//
// When metricNames is empty the union of names available in both runs is used.
func (s *ComparisonService) CompareRuns(
	ctx context.Context,
	tenantID *iampb.TenantId,
	aID *testingpb.TestRunId,
	bID *testingpb.TestRunId,
	metricNames []string,
) (*testingpb.CompareRunsResponse, error) {
	// Query both runs.  MetricsPort.Query accepts a query string; passing the
	// metric name as the query string is the convention used elsewhere.
	aMetrics, err := s.queryAll(ctx, tenantID, aID, metricNames)
	if err != nil {
		return nil, fmt.Errorf("CompareRuns: query metrics for a (%s): %w", aID.GetValue(), err)
	}
	bMetrics, err := s.queryAll(ctx, tenantID, bID, metricNames)
	if err != nil {
		return nil, fmt.Errorf("CompareRuns: query metrics for b (%s): %w", bID.GetValue(), err)
	}

	// Build average maps.
	aAvgs := avgByName(aMetrics)
	bAvgs := avgByName(bMetrics)

	// Compute diffs for all names present in at least one side.
	names := unionKeys(aAvgs, bAvgs)
	if len(metricNames) > 0 {
		names = metricNames
	}

	diffs := make([]*testingpb.MetricDiff, 0, len(names))
	for _, name := range names {
		av := aAvgs[name]
		bv := bAvgs[name]

		absDelta := bv - av
		var pctDelta float64
		if av != 0 {
			pctDelta = (bv - av) / math.Abs(av) * 100
		}

		verdict := verdictFor(name, absDelta)

		diffs = append(diffs, &testingpb.MetricDiff{
			MetricName: name,
			AValue:     av,
			BValue:     bv,
			AbsDelta:   absDelta,
			PctDelta:   pctDelta,
			Verdict:    verdict,
		})
	}

	return &testingpb.CompareRunsResponse{
		A:     aID,
		B:     bID,
		Diffs: diffs,
	}, nil
}

// queryAll fetches MetricSeriesList for each requested metric name (or a
// single wildcard query when names is empty).
func (s *ComparisonService) queryAll(
	ctx context.Context,
	tenantID *iampb.TenantId,
	id *testingpb.TestRunId,
	names []string,
) ([]*testingpb.MetricSeries, error) {
	if len(names) == 0 {
		// Empty query: fetch all metrics for the run.
		list, err := s.metrics.Query(ctx, tenantID, id, "")
		if err != nil {
			return nil, err
		}
		return list.GetSeries(), nil
	}

	var all []*testingpb.MetricSeries
	for _, n := range names {
		list, err := s.metrics.Query(ctx, tenantID, id, n)
		if err != nil {
			return nil, fmt.Errorf("metric %q: %w", n, err)
		}
		all = append(all, list.GetSeries()...)
	}
	return all, nil
}

// avgByName returns a map[metricName]average across all points in all series
// with that name.
func avgByName(series []*testingpb.MetricSeries) map[string]float64 {
	sums := map[string]float64{}
	counts := map[string]int{}
	for _, s := range series {
		name := s.GetName()
		for _, pt := range s.GetPoints() {
			sums[name] += pt.GetValue()
			counts[name]++
		}
	}
	avgs := make(map[string]float64, len(sums))
	for name, sum := range sums {
		if counts[name] > 0 {
			avgs[name] = sum / float64(counts[name])
		}
	}
	return avgs
}

// unionKeys returns the union of keys from two maps in deterministic order.
func unionKeys(a, b map[string]float64) []string {
	seen := map[string]bool{}
	var out []string
	for k := range a {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for k := range b {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}

// CrossCompareBatch returns per-cell metrics + diffs vs a baseline run for
// every TestRun within a TestSuiteRun. When baselineRunID is nil the first
// child is used; pairwise matrix beyond that is the caller's job.
//
// Note: this needs a way to list child TestRuns of a TestSuiteRun. Until the
// TestSuiteRun.children index is exposed via a port, we accept the suite-run
// id and rely on the TestRun listing by suite_run_id ancestor (added below).
func (s *ComparisonService) CrossCompareBatch(
	ctx context.Context,
	tenantID *iampb.TenantId,
	suiteRunID *testingpb.TestSuiteRunId,
	baselineRunID *testingpb.TestRunId,
) (*testingpb.CrossCompareBatchResponse, error) {
	if s.testRunLister == nil {
		return nil, fmt.Errorf("CrossCompareBatch: TestRunByteSuite lister not wired")
	}
	children, err := s.testRunLister.ListTestRunsBySuiteRun(ctx, tenantID, suiteRunID)
	if err != nil {
		return nil, err
	}
	if len(children) == 0 {
		return &testingpb.CrossCompareBatchResponse{}, nil
	}
	if baselineRunID == nil || baselineRunID.GetValue() == "" {
		baselineRunID = children[0].GetId()
	}

	out := &testingpb.CrossCompareBatchResponse{}
	for _, child := range children {
		runMetrics, err := s.metrics.Query(ctx, tenantID, child.GetId(), "")
		if err != nil {
			return nil, fmt.Errorf("CrossCompareBatch: cell %s: %w", child.GetId().GetValue(), err)
		}
		cell := &testingpb.CrossCompareBatchResponse_CellMetrics{
			TestRunId: child.GetId(),
			Metrics:   runMetrics,
		}
		if child.GetId().GetValue() != baselineRunID.GetValue() {
			diff, err := s.CompareRuns(ctx, tenantID, baselineRunID, child.GetId(), nil)
			if err == nil {
				cell.DiffsVsBaseline = diff.GetDiffs()
			}
		}
		out.Cells = append(out.Cells, cell)
	}
	return out, nil
}

// TestRunSuiteLister is the minimum port needed to enumerate children of a
// TestSuiteRun. Implemented by TestSuiteRunService.
type TestRunSuiteLister interface {
	ListTestRunsBySuiteRun(ctx context.Context, tenantID *iampb.TenantId, suiteRunID *testingpb.TestSuiteRunId) ([]*testingpb.TestRun, error)
}

// WithTestRunLister wires the lister required by CrossCompareBatch.
func (s *ComparisonService) WithTestRunLister(l TestRunSuiteLister) *ComparisonService {
	s.testRunLister = l
	return s
}

// verdictFor assigns a verdict based on delta sign. For metrics where a higher
// value is better (e.g. throughput), positive delta is IMPROVED. A more
// sophisticated implementation would use per-metric direction config.
func verdictFor(_ string, absDelta float64) testingpb.MetricDiff_Verdict {
	const threshold = 0.01 // treat deltas < 1% of a unit as neutral
	switch {
	case absDelta > threshold:
		return testingpb.MetricDiff_VERDICT_IMPROVED
	case absDelta < -threshold:
		return testingpb.MetricDiff_VERDICT_REGRESSED
	default:
		return testingpb.MetricDiff_VERDICT_NEUTRAL
	}
}
