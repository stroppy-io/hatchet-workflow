package metrics

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/victoria"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
)

// TimeRange defines the observation window for a run.
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// Collector fetches and aggregates metrics for runs against VictoriaMetrics.
type Collector struct {
	client *victoria.Client
	defs   []MetricDef
}

// NewCollector creates a metrics collector with the default (postgres) query set.
func NewCollector(client *victoria.Client) *Collector {
	return &Collector{client: client, defs: DefaultMetrics()}
}

// NewCollectorForDB creates a metrics collector with DB-specific queries.
func NewCollectorForDB(client *victoria.Client, dbKind string) *Collector {
	return &Collector{client: client, defs: MetricsForDB(dbKind)}
}

// Collect fetches all defined metrics for a run within the given time range and
// returns the aggregated monitor.RunMetrics proto.
func (c *Collector) Collect(ctx context.Context, runID string, tr TimeRange) (*monitor.RunMetrics, error) {
	step := inferStep(tr)
	result := &monitor.RunMetrics{
		RunId: runID,
		Range: &monitor.TimeRange{
			Start: timestamppb.New(tr.Start),
			End:   timestamppb.New(tr.End),
		},
		Metrics: make([]*monitor.MetricSummary, 0, len(c.defs)),
	}

	for _, def := range c.defs {
		query := RenderQuery(def, runID)
		qr, err := c.client.QueryRange(ctx, query, tr.Start, tr.End, step)
		if err != nil {
			return nil, fmt.Errorf("metrics: query %q: %w", def.Key, err)
		}
		result.Metrics = append(result.Metrics, summarize(def, qr))
	}

	return result, nil
}

// summarize aggregates a query_range result into a MetricSummary using a p5–p95
// trim to exclude counter-reset spikes.
func summarize(def MetricDef, qr *victoria.QueryResult) *monitor.MetricSummary {
	s := &monitor.MetricSummary{
		Key:            def.Key,
		Name:           def.Name,
		Unit:           def.Unit,
		HigherIsBetter: def.HigherIsBetter,
		Group:          def.Group,
	}

	var vals []float64
	for _, series := range qr.Data.Result {
		for _, pair := range series.Values {
			if v, err := strconv.ParseFloat(pair.Val(), 64); err == nil && !math.IsInf(v, 0) && !math.IsNaN(v) {
				vals = append(vals, v)
			}
		}
	}

	if len(vals) == 0 {
		return s
	}

	// last value is the most recent sample (pre-trim, pre-sort).
	last := vals[len(vals)-1]

	// Sort to compute percentiles and filter outliers.
	sort.Float64s(vals)

	// Use p5–p95 range to exclude counter reset spikes.
	lo := len(vals) * 5 / 100
	hi := len(vals) - lo
	if hi <= lo {
		lo = 0
		hi = len(vals)
	}
	trimmed := vals[lo:hi]
	if len(trimmed) == 0 {
		trimmed = vals
	}

	s.Min = trimmed[0]
	s.Max = trimmed[len(trimmed)-1]
	var sum float64
	for _, v := range trimmed {
		sum += v
	}
	s.Avg = sum / float64(len(trimmed))
	s.Last = last

	return s
}

// inferStep picks a query step proportional to the window width.
func inferStep(tr TimeRange) time.Duration {
	d := tr.End.Sub(tr.Start)
	switch {
	case d <= 15*time.Minute:
		return 5 * time.Second
	case d <= time.Hour:
		return 15 * time.Second
	case d <= 6*time.Hour:
		return time.Minute
	default:
		return 5 * time.Minute
	}
}
