package adapters

import (
	"context"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"github.com/stroppy-io/stroppy-cloud/internal/services/compare"
)

// sameThresholdPct is the relative diff (percent) under which a metric is judged
// unchanged ("same") rather than better/worse. A small dead-band keeps run-to-run
// noise from being reported as a regression/improvement.
const sameThresholdPct = 1.0

// RunRecordGetter is the minimal consumer interface the compare adapters need to
// load a stored run by id. The gormstore TestRuns repo (Get(ctx, id)
// (*models.TestRunRecord, error)) satisfies it directly. Get returns
// derrors.ErrNotFound for an unknown id.
type RunRecordGetter interface {
	Get(ctx context.Context, id string) (*models.TestRunRecord, error)
}

// RunMetricsGetter is the minimal consumer interface the comparator needs to read
// a run's persisted, aggregated metric summaries. The gormstore metrics reader
// (Get(ctx, runID) (*monitor.RunMetrics, error)) satisfies it. Get returns
// derrors.ErrNotFound when a run has no metrics.
type RunMetricsGetter interface {
	Get(ctx context.Context, runID string) (*monitor.RunMetrics, error)
}

// TestRunReader implements compare.TestRunReader by delegating to a RunRecordGetter.
type TestRunReader struct{ runs RunRecordGetter }

var _ compare.TestRunReader = (*TestRunReader)(nil)

// NewTestRunReader builds the compare.TestRunReader over the injected run getter.
func NewTestRunReader(runs RunRecordGetter) *TestRunReader { return &TestRunReader{runs: runs} }

// Get loads a run by id.
func (r *TestRunReader) Get(ctx context.Context, id string) (*models.TestRunRecord, error) {
	return r.runs.Get(ctx, id)
}

// MetricsComparator implements compare.MetricsComparator: it reads each run's
// aggregated metric summaries and computes the aligned, per-metric cross-run diff
// (monitor.Comparison) against the baseline (runIDs[0]).
type MetricsComparator struct{ metrics RunMetricsGetter }

var _ compare.MetricsComparator = (*MetricsComparator)(nil)

// NewMetricsComparator builds the comparator over the injected metrics getter.
func NewMetricsComparator(metrics RunMetricsGetter) *MetricsComparator {
	return &MetricsComparator{metrics: metrics}
}

// Compare builds the N-way comparison. It loads every run's RunMetrics, unions the
// metric keys (baseline order first, then any extra keys from other runs in first-
// seen order), and for each key emits a MetricRow with one MetricCell per run.
// diff_*_pct and the verdict are computed relative to the baseline cell, honoring
// the metric's higher_is_better direction. derrors.ErrNotFound when a run has no
// metrics to compare.
func (c *MetricsComparator) Compare(ctx context.Context, runIDs []string) (*monitor.Comparison, error) {
	if len(runIDs) < 2 {
		return nil, derrors.Invalid("run_ids", "at least two runs are required to compare")
	}

	// Load metrics per run, indexed by metric key for aligned lookup.
	perRun := make([]map[string]*monitor.MetricSummary, len(runIDs))
	var orderedKeys []string
	seenKey := make(map[string]struct{})
	var rng *monitor.TimeRange

	for i, id := range runIDs {
		rm, err := c.metrics.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if rng == nil {
			rng = rm.GetRange()
		}
		byKey := make(map[string]*monitor.MetricSummary, len(rm.GetMetrics()))
		for _, m := range rm.GetMetrics() {
			byKey[m.GetKey()] = m
			if _, ok := seenKey[m.GetKey()]; !ok {
				seenKey[m.GetKey()] = struct{}{}
				orderedKeys = append(orderedKeys, m.GetKey())
			}
		}
		perRun[i] = byKey
	}

	cmp := &monitor.Comparison{RunIds: runIDs, Range: rng}

	// Per-run roll-up counters (aligned with runIDs[1:]).
	summaries := make([]*monitor.Comparison_RunSummary, len(runIDs)-1)
	for i := 1; i < len(runIDs); i++ {
		summaries[i-1] = &monitor.Comparison_RunSummary{RunId: runIDs[i]}
	}

	for _, key := range orderedKeys {
		baseline := perRun[0][key]
		// Carry display metadata from whichever run first defines the key.
		meta := baseline
		if meta == nil {
			for i := 1; i < len(perRun); i++ {
				if m := perRun[i][key]; m != nil {
					meta = m
					break
				}
			}
		}
		row := &monitor.MetricRow{
			Key:            key,
			Name:           meta.GetName(),
			Unit:           meta.GetUnit(),
			HigherIsBetter: meta.GetHigherIsBetter(),
			Group:          meta.GetGroup(),
		}
		baseAvg := baseline.GetAvg()
		baseMax := baseline.GetMax()
		higherIsBetter := meta.GetHigherIsBetter()

		for i, id := range runIDs {
			m := perRun[i][key]
			cell := &monitor.MetricCell{RunId: id, Avg: m.GetAvg(), Max: m.GetMax()}
			if i == 0 {
				cell.Verdict = monitor.Verdict_VERDICT_SAME
				row.Cells = append(row.Cells, cell)
				continue
			}
			cell.DiffAvgPct = relDiffPct(m.GetAvg(), baseAvg)
			cell.DiffMaxPct = relDiffPct(m.GetMax(), baseMax)
			cell.Verdict = verdict(cell.DiffAvgPct, higherIsBetter)
			row.Cells = append(row.Cells, cell)

			switch cell.Verdict {
			case monitor.Verdict_VERDICT_BETTER:
				summaries[i-1].Better++
			case monitor.Verdict_VERDICT_WORSE:
				summaries[i-1].Worse++
			default:
				summaries[i-1].Same++
			}
		}
		cmp.Metrics = append(cmp.Metrics, row)
	}

	cmp.Summaries = summaries
	return cmp, nil
}

// relDiffPct is (value - baseline) / baseline * 100. When the baseline is zero it
// reports 0 (no defined relative change) to avoid divide-by-zero noise.
func relDiffPct(value, baseline float64) float64 {
	if baseline == 0 {
		return 0
	}
	return (value - baseline) / baseline * 100
}

// verdict classifies a cell's avg diff against the baseline within the dead-band,
// honoring the metric's higher_is_better direction.
func verdict(diffAvgPct float64, higherIsBetter bool) monitor.Verdict {
	if diffAvgPct <= sameThresholdPct && diffAvgPct >= -sameThresholdPct {
		return monitor.Verdict_VERDICT_SAME
	}
	higher := diffAvgPct > 0
	if higher == higherIsBetter {
		return monitor.Verdict_VERDICT_BETTER
	}
	return monitor.Verdict_VERDICT_WORSE
}
