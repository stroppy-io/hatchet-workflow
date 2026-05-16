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
	metrics MetricsPort
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
