package metrics

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/victoria"
)

// TimeRange defines the observation window for a run.
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// MetricSummary holds aggregated values for a single metric over a run.
type MetricSummary struct {
	Key  string  `json:"key"`
	Name string  `json:"name"`
	Unit string  `json:"unit"`
	Avg  float64 `json:"avg"`
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
	Last float64 `json:"last"`
}

// RunMetrics is the full metrics snapshot for a single run.
type RunMetrics struct {
	RunID   string          `json:"run_id"`
	Range   TimeRange       `json:"range"`
	Metrics []MetricSummary `json:"metrics"`
}

// Collector fetches and aggregates metrics for runs.
type Collector struct {
	client *victoria.Client
	defs   []MetricDef
}

// NewCollector creates a metrics collector.
func NewCollector(client *victoria.Client) *Collector {
	return &Collector{
		client: client,
		defs:   DefaultMetrics(),
	}
}

// NewCollectorForDB creates a metrics collector with DB-specific queries.
func NewCollectorForDB(client *victoria.Client, dbKind string) *Collector {
	return &Collector{
		client: client,
		defs:   MetricsForDB(dbKind),
	}
}

// Collect fetches all defined metrics for a run within the given time range.
func (c *Collector) Collect(ctx context.Context, runID string, tr TimeRange) (*RunMetrics, error) {
	step := inferStep(tr)
	result := &RunMetrics{
		RunID:   runID,
		Range:   tr,
		Metrics: make([]MetricSummary, 0, len(c.defs)),
	}

	for _, def := range c.defs {
		query := RenderQuery(def, runID)
		qr, err := c.client.QueryRange(ctx, query, tr.Start, tr.End, step)
		if err != nil {
			return nil, fmt.Errorf("metrics: query %q: %w", def.Key, err)
		}

		summary := summarize(def, qr)
		result.Metrics = append(result.Metrics, summary)
	}

	return result, nil
}

func summarize(def MetricDef, qr *victoria.QueryResult) MetricSummary {
	s := MetricSummary{
		Key:  def.Key,
		Name: def.Name,
		Unit: def.Unit,
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

	// Sort to compute percentiles and filter outliers.
	sort.Float64s(vals)

	// Use p5-p95 range to exclude counter reset spikes.
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
	s.Last = vals[len(vals)-1]

	return s
}

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
