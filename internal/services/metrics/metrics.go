// Package metrics implements run.MetricsClient over VictoriaMetrics: it turns a
// run id into the additive RunMetrics/Comparison summaries the UI shows (E19, the
// summaries are additive to the direct-to-VM Grafana dashboards, H56).
package metrics

import (
	"context"
	"fmt"

	"github.com/gopherex/xlog"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/victoria"
	metricspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/metrics"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// metricDef is one self-describing entry of the run-metrics catalog (H46): the
// MetricSummary metadata (key/name/unit/group/description/higher_is_better) plus
// the PromQL expression template used to fetch its value. It is the in-process
// rendering of the "self-describing MetricDef" surface — every field that lands
// on the wire MetricSummary is declared here, so the UI never has to guess a
// metric's unit or comparison direction.
//
// exprAgg holds a single aggregating PromQL expression with a %q run-id slot. It
// is queried once and the scalar result is currently mirrored into avg/min/max/
// last on the MetricSummary (see GetRunMetrics). This collapse is correct only
// because every catalog expr is itself an aggregate over the run window; when a
// real per-aggregation query set (and the run TimeRange) is plumbed, swap exprAgg
// for distinct avg/min/max expressions here without touching the consumers.
type metricDef struct {
	key            string
	name           string
	unit           string
	group          string
	description    string
	higherIsBetter bool
	exprAgg        string // single aggregating PromQL expr; %q -> run id
}

// catalog is the self-describing stroppy run-metrics catalog (H46). It is the one
// editable surface that couples the cloud summaries to the metric/label NAMES
// stroppy actually exports.
//
// BEST-EFFORT NAMES: the series names (e.g. `stroppy_ops_total`) and the
// `run_id` label are a best-effort guess at the stroppy export schema. They MUST
// be reconciled against what the stroppy workload exporter really emits — keep
// every name change confined to this var. The aggregation window (`[1h]`) is a
// stand-in until the run TimeRange is plumbed through the query layer.
var catalog = []metricDef{
	{
		key:            "throughput",
		name:           "Throughput",
		unit:           "ops/s",
		group:          "throughput",
		description:    "Completed operations per second across the run window. Higher is faster.",
		higherIsBetter: true,
		exprAgg:        `avg_over_time(rate(stroppy_ops_total{run_id=%q}[1m])[1h:1m])`,
	},
	{
		key:            "latency_p50",
		name:           "Latency p50",
		unit:           "ms",
		group:          "latency",
		description:    "Median (50th percentile) operation latency over the run window. Lower is better.",
		higherIsBetter: false,
		exprAgg:        `avg_over_time(stroppy_latency_p50_ms{run_id=%q}[1h])`,
	},
	{
		key:            "latency_p95",
		name:           "Latency p95",
		unit:           "ms",
		group:          "latency",
		description:    "95th-percentile operation latency over the run window. Lower is better.",
		higherIsBetter: false,
		exprAgg:        `avg_over_time(stroppy_latency_p95_ms{run_id=%q}[1h])`,
	},
	{
		key:            "latency_p99",
		name:           "Latency p99",
		unit:           "ms",
		group:          "latency",
		description:    "99th-percentile (tail) operation latency over the run window. Lower is better.",
		higherIsBetter: false,
		exprAgg:        `avg_over_time(stroppy_latency_p99_ms{run_id=%q}[1h])`,
	},
	{
		key:            "errors",
		name:           "Errors",
		unit:           "count",
		group:          "errors",
		description:    "Total failed operations observed during the run. Lower is better.",
		higherIsBetter: false,
		exprAgg:        `sum(increase(stroppy_errors_total{run_id=%q}[1h]))`,
	},
	{
		key:            "error_rate",
		name:           "Error rate",
		unit:           "%",
		group:          "errors",
		description:    "Share of operations that failed during the run, in percent. Lower is better.",
		higherIsBetter: false,
		exprAgg:        `avg_over_time((sum(rate(stroppy_errors_total{run_id=%q}[1m])) / sum(rate(stroppy_ops_total{run_id=%q}[1m])) * 100)[1h:1m])`,
	},
	{
		key:            "active_connections",
		name:           "Active connections",
		unit:           "count",
		group:          "resources",
		description:    "Average number of in-use database connections over the run window.",
		higherIsBetter: false,
		exprAgg:        `avg_over_time(stroppy_active_connections{run_id=%q}[1h])`,
	},
}

// expr renders the metric's PromQL expression, filling every %q slot with the run
// id (some exprs reference the run id more than once, e.g. error_rate).
func (d metricDef) expr(runID string) string {
	n := 0
	for i := 0; i+1 < len(d.exprAgg); i++ {
		if d.exprAgg[i] == '%' && d.exprAgg[i+1] == 'q' {
			n++
		}
	}
	args := make([]any, n)
	for i := range args {
		args[i] = runID
	}
	return fmt.Sprintf(d.exprAgg, args...)
}

// summary builds the static (metadata) part of a MetricSummary from the def,
// leaving the numeric fields to the caller.
func (d metricDef) summary() *metricspb.MetricSummary {
	return &metricspb.MetricSummary{
		Key:            d.key,
		Name:           d.name,
		Unit:           d.unit,
		Group:          d.group,
		Description:    d.description,
		HigherIsBetter: d.higherIsBetter,
	}
}

// Client implements run.MetricsClient.
type Client struct {
	*tracing.Entity
	vm *victoria.Client
}

// New builds a metrics client over a VictoriaMetrics client.
func New(logger *xlog.Logger, vm *victoria.Client) *Client {
	return &Client{Entity: tracing.NewEntity(logger.AppendName("MetricsClient")), vm: vm}
}

// GetRunMetrics queries the summary set for a run.
func (c *Client) GetRunMetrics(ctx context.Context, runID string) (*metricspb.RunMetrics, error) {
	out := &metricspb.RunMetrics{RunId: runID, Metrics: make([]*metricspb.MetricSummary, 0, len(catalog))}
	for _, d := range catalog {
		v, ok, err := c.vm.InstantValue(ctx, d.expr(runID))
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		// avg/min/max/last are collapsed to the single aggregated value because
		// every catalog expr is itself an aggregate over the run window. See the
		// metricDef doc-comment: split exprAgg into per-aggregation queries here
		// when the run TimeRange is wired.
		s := d.summary()
		s.Avg, s.Min, s.Max, s.Last = v, v, v, v
		out.Metrics = append(out.Metrics, s)
	}
	return out, nil
}

// CompareRuns diffs two runs' summaries, computing per-metric percentage deltas
// and a better/worse/same verdict from each metric's higher_is_better direction.
func (c *Client) CompareRuns(ctx context.Context, runA, runB string) (*metricspb.Comparison, error) {
	a, err := c.GetRunMetrics(ctx, runA)
	if err != nil {
		return nil, err
	}
	b, err := c.GetRunMetrics(ctx, runB)
	if err != nil {
		return nil, err
	}
	bByKey := make(map[string]*metricspb.MetricSummary, len(b.GetMetrics()))
	for _, m := range b.GetMetrics() {
		bByKey[m.GetKey()] = m
	}
	cmp := &metricspb.Comparison{
		RunA:    runA,
		RunB:    runB,
		Metrics: make([]*metricspb.MetricDiff, 0, len(a.GetMetrics())),
		Summary: &metricspb.Comparison_Summary{},
	}
	for _, ma := range a.GetMetrics() {
		mb := bByKey[ma.GetKey()]
		diffAvg := pct(ma.GetAvg(), mb.GetAvg())
		verdict := verdict(ma.GetHigherIsBetter(), diffAvg)
		cmp.Metrics = append(cmp.Metrics, &metricspb.MetricDiff{
			Key:        ma.GetKey(),
			Name:       ma.GetName(),
			Unit:       ma.GetUnit(),
			AvgA:       ma.GetAvg(),
			AvgB:       mb.GetAvg(),
			MaxA:       ma.GetMax(),
			MaxB:       mb.GetMax(),
			DiffAvgPct: diffAvg,
			DiffMaxPct: pct(ma.GetMax(), mb.GetMax()),
			Verdict:    verdict,
		})
		switch verdict {
		case metricspb.MetricDiff_VERDICT_BETTER:
			cmp.Summary.Better++
		case metricspb.MetricDiff_VERDICT_WORSE:
			cmp.Summary.Worse++
		default:
			cmp.Summary.Same++
		}
	}
	return cmp, nil
}

// pct returns the percentage change from a to b: (b - a) / a * 100. Positive
// means b is higher than a. Returns 0 when the baseline a is 0 (undefined).
func pct(a, b float64) float64 {
	if a == 0 {
		return 0
	}
	return (b - a) / a * 100
}

// verdict maps a percentage delta (run B relative to run A) to a better/worse/same
// verdict, honouring the metric's comparison direction. For a higher-is-better
// metric an increase is an improvement; for a lower-is-better metric a decrease is
// an improvement. A zero delta is VERDICT_SAME.
func verdict(higherIsBetter bool, diffPct float64) metricspb.MetricDiff_Verdict {
	switch {
	case diffPct == 0:
		return metricspb.MetricDiff_VERDICT_SAME
	case diffPct > 0:
		// B is higher than A.
		if higherIsBetter {
			return metricspb.MetricDiff_VERDICT_BETTER
		}
		return metricspb.MetricDiff_VERDICT_WORSE
	default:
		// B is lower than A.
		if higherIsBetter {
			return metricspb.MetricDiff_VERDICT_WORSE
		}
		return metricspb.MetricDiff_VERDICT_BETTER
	}
}
