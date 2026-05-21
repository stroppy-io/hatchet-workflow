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

// metricDef is one summary metric: a PromQL expression (with a %s run-id slot)
// aggregated over the run window.
//
// TODO(metrics): this is a placeholder catalog. The real metric catalog is
// backend data (self-describing MetricDef, H46) and the expressions/labels must
// match what stroppy actually exports. min/max/avg need per-window aggregation
// (the run TimeRange is not plumbed yet).
type metricDef struct {
	key, name, unit, group string
	expr                   string // %s -> run id
	higherIsBetter         bool
}

var catalog = []metricDef{
	{key: "throughput", name: "Throughput", unit: "ops/s", group: "workload", expr: `avg_over_time(stroppy_throughput{run_id=%q}[1h])`, higherIsBetter: true},
	{key: "latency_p99", name: "Latency p99", unit: "ms", group: "workload", expr: `avg_over_time(stroppy_latency_p99{run_id=%q}[1h])`, higherIsBetter: false},
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
		v, ok, err := c.vm.InstantValue(ctx, fmt.Sprintf(d.expr, runID))
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		// TODO(metrics): avg/min/max collapsed to the single queried value until
		// the run window + per-aggregation queries are wired.
		out.Metrics = append(out.Metrics, &metricspb.MetricSummary{
			Key: d.key, Name: d.name, Unit: d.unit, Group: d.group,
			HigherIsBetter: d.higherIsBetter,
			Avg:            v, Min: v, Max: v, Last: v,
		})
	}
	return out, nil
}

// CompareRuns diffs two runs' summaries.
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
	cmp := &metricspb.Comparison{RunA: runA, RunB: runB, Metrics: make([]*metricspb.MetricDiff, 0, len(a.GetMetrics()))}
	for _, ma := range a.GetMetrics() {
		mb := bByKey[ma.GetKey()]
		cmp.Metrics = append(cmp.Metrics, &metricspb.MetricDiff{
			Key: ma.GetKey(), Name: ma.GetName(), Unit: ma.GetUnit(),
			AvgA: ma.GetAvg(), AvgB: mb.GetAvg(),
			MaxA: ma.GetMax(), MaxB: mb.GetMax(),
			DiffAvgPct: pct(ma.GetAvg(), mb.GetAvg()),
			DiffMaxPct: pct(ma.GetMax(), mb.GetMax()),
		})
	}
	return cmp, nil
}

func pct(a, b float64) float64 {
	if a == 0 {
		return 0
	}
	return (b - a) / a * 100
}
