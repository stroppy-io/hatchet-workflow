package metrics

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gopherex/xlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/victoria"
	metricspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/metrics"
)

// vectorResponse renders a VictoriaMetrics /api/v1/query instant-vector envelope
// with a single canned scalar value.
func vectorResponse(value float64) string {
	return fmt.Sprintf(
		`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1700000000,%q]}]}}`,
		fmt.Sprintf("%g", value),
	)
}

// emptyVectorResponse renders a successful response with no series (metric absent).
const emptyVectorResponse = `{"status":"success","data":{"resultType":"vector","result":[]}}`

// cannedServer serves /api/v1/query, dispatching on (metric key, run id) -> value.
// The metric key is identified by looking for the catalog's series-name fragment
// in the PromQL query string; the run id is taken from the run_id="..." label.
// A run/key pair absent from values yields an empty (no-series) response.
func cannedServer(t *testing.T, values map[string]map[string]float64) *httptest.Server {
	t.Helper()
	// Map a substring that is unique to each catalog metric's expr -> metric key.
	// These mirror the series names declared in catalog.
	fragments := []struct {
		frag, key string
	}{
		{"stroppy_ops_total", "throughput"}, // also referenced by error_rate; checked after errors below
		{"stroppy_latency_p50_ms", "latency_p50"},
		{"stroppy_latency_p95_ms", "latency_p95"},
		{"stroppy_latency_p99_ms", "latency_p99"},
		{"stroppy_errors_total", "errors"},
		{"stroppy_active_connections", "active_connections"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/query", r.URL.Path)
		query := r.URL.Query().Get("query")
		require.NotEmpty(t, query, "query param must be present")

		runID := extractRunID(query)

		key := classify(query, fragments)
		require.NotEmpty(t, key, "could not classify query: %s", query)

		w.Header().Set("Content-Type", "application/json")
		if byRun, ok := values[key]; ok {
			if v, ok := byRun[runID]; ok {
				_, _ = w.Write([]byte(vectorResponse(v)))
				return
			}
		}
		_, _ = w.Write([]byte(emptyVectorResponse))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// classify returns the catalog metric key for a query, disambiguating error_rate
// (which references both errors_total and ops_total) from plain throughput/errors.
func classify(query string, fragments []struct{ frag, key string }) string {
	// error_rate uses both errors_total and ops_total and a "* 100" factor.
	if strings.Contains(query, "stroppy_errors_total") && strings.Contains(query, "stroppy_ops_total") {
		return "error_rate"
	}
	for _, f := range fragments {
		if strings.Contains(query, f.frag) {
			return f.key
		}
	}
	return ""
}

// extractRunID pulls the run id out of a run_id="..." label in the PromQL string.
func extractRunID(query string) string {
	const marker = `run_id="`
	i := strings.Index(query, marker)
	if i < 0 {
		return ""
	}
	rest := query[i+len(marker):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		return ""
	}
	return rest[:j]
}

func newTestClient(t *testing.T, values map[string]map[string]float64) *Client {
	t.Helper()
	srv := cannedServer(t, values)
	vm := victoria.NewClient(srv.URL, "")
	return New(xlog.Default(), vm)
}

func TestGetRunMetrics_ReturnsCatalogSummaries(t *testing.T) {
	const run = "run-A"
	values := map[string]map[string]float64{
		"throughput":         {run: 1500},
		"latency_p50":        {run: 5.5},
		"latency_p95":        {run: 12.0},
		"latency_p99":        {run: 30.0},
		"errors":             {run: 4},
		"error_rate":         {run: 0.5},
		"active_connections": {run: 16},
	}
	c := newTestClient(t, values)

	rm, err := c.GetRunMetrics(context.Background(), run)
	require.NoError(t, err)
	require.Equal(t, run, rm.GetRunId())
	// All catalog metrics have canned values, so all should be present.
	require.Len(t, rm.GetMetrics(), len(catalog))

	byKey := map[string]*metricspb.MetricSummary{}
	for _, m := range rm.GetMetrics() {
		byKey[m.GetKey()] = m
	}

	// Self-describing metadata flows from the catalog onto every summary.
	for _, d := range catalog {
		m, ok := byKey[d.key]
		require.Truef(t, ok, "metric %q missing from result", d.key)
		assert.Equal(t, d.name, m.GetName(), "name for %s", d.key)
		assert.Equal(t, d.unit, m.GetUnit(), "unit for %s", d.key)
		assert.Equal(t, d.group, m.GetGroup(), "group for %s", d.key)
		assert.Equal(t, d.description, m.GetDescription(), "description for %s", d.key)
		assert.Equal(t, d.higherIsBetter, m.GetHigherIsBetter(), "higherIsBetter for %s", d.key)
	}

	// Values from the canned responses, mirrored across avg/min/max/last.
	tp := byKey["throughput"]
	assert.Equal(t, 1500.0, tp.GetAvg())
	assert.Equal(t, 1500.0, tp.GetMin())
	assert.Equal(t, 1500.0, tp.GetMax())
	assert.Equal(t, 1500.0, tp.GetLast())
	assert.True(t, tp.GetHigherIsBetter())

	assert.Equal(t, 30.0, byKey["latency_p99"].GetLast())
	assert.False(t, byKey["latency_p99"].GetHigherIsBetter())
}

func TestGetRunMetrics_SkipsAbsentMetrics(t *testing.T) {
	const run = "sparse"
	// Only throughput and errors have data; the rest return empty vectors.
	values := map[string]map[string]float64{
		"throughput": {run: 900},
		"errors":     {run: 2},
	}
	c := newTestClient(t, values)

	rm, err := c.GetRunMetrics(context.Background(), run)
	require.NoError(t, err)
	require.Len(t, rm.GetMetrics(), 2)

	got := map[string]bool{}
	for _, m := range rm.GetMetrics() {
		got[m.GetKey()] = true
	}
	assert.True(t, got["throughput"])
	assert.True(t, got["errors"])
	assert.False(t, got["latency_p99"])
}

func TestCompareRuns_VerdictsAndSummary(t *testing.T) {
	const runA, runB = "base", "candidate"
	values := map[string]map[string]float64{
		// throughput up 20% -> higher_is_better -> BETTER.
		"throughput": {runA: 1000, runB: 1200},
		// p99 up 10% -> lower_is_better -> WORSE.
		"latency_p99": {runA: 20, runB: 22},
		// errors down 50% -> lower_is_better -> BETTER.
		"errors": {runA: 10, runB: 5},
		// active_connections unchanged -> SAME.
		"active_connections": {runA: 8, runB: 8},
		// p50 / p95 / error_rate present so the catalog is fully covered.
		"latency_p50": {runA: 5, runB: 5},   // SAME
		"latency_p95": {runA: 10, runB: 12}, // up 20% lower-is-better -> WORSE
		"error_rate":  {runA: 1, runB: 0.5}, // down 50% lower-is-better -> BETTER
	}
	c := newTestClient(t, values)

	cmp, err := c.CompareRuns(context.Background(), runA, runB)
	require.NoError(t, err)
	require.Equal(t, runA, cmp.GetRunA())
	require.Equal(t, runB, cmp.GetRunB())
	require.Len(t, cmp.GetMetrics(), len(catalog))

	byKey := map[string]*metricspb.MetricDiff{}
	for _, d := range cmp.GetMetrics() {
		byKey[d.GetKey()] = d
	}

	// Throughput: +20%, higher is better => BETTER.
	tp := byKey["throughput"]
	assert.InDelta(t, 20.0, tp.GetDiffAvgPct(), 1e-9)
	assert.Equal(t, metricspb.MetricDiff_VERDICT_BETTER, tp.GetVerdict())

	// Latency p99: +10%, lower is better => WORSE.
	p99 := byKey["latency_p99"]
	assert.InDelta(t, 10.0, p99.GetDiffAvgPct(), 1e-9)
	assert.Equal(t, metricspb.MetricDiff_VERDICT_WORSE, p99.GetVerdict())

	// Errors: -50%, lower is better => BETTER.
	errs := byKey["errors"]
	assert.InDelta(t, -50.0, errs.GetDiffAvgPct(), 1e-9)
	assert.Equal(t, metricspb.MetricDiff_VERDICT_BETTER, errs.GetVerdict())

	// Active connections: unchanged => SAME.
	ac := byKey["active_connections"]
	assert.InDelta(t, 0.0, ac.GetDiffAvgPct(), 1e-9)
	assert.Equal(t, metricspb.MetricDiff_VERDICT_SAME, ac.GetVerdict())

	// Roll-up summary: better = throughput, errors, error_rate (3);
	// worse = p99, p95 (2); same = active_connections, p50 (2).
	require.NotNil(t, cmp.GetSummary())
	assert.Equal(t, uint32(3), cmp.GetSummary().GetBetter())
	assert.Equal(t, uint32(2), cmp.GetSummary().GetWorse())
	assert.Equal(t, uint32(2), cmp.GetSummary().GetSame())
}

func TestVerdict(t *testing.T) {
	cases := []struct {
		name           string
		higherIsBetter bool
		diffPct        float64
		want           metricspb.MetricDiff_Verdict
	}{
		{"higher-better up is improvement", true, 15, metricspb.MetricDiff_VERDICT_BETTER},
		{"higher-better down is regression", true, -15, metricspb.MetricDiff_VERDICT_WORSE},
		{"lower-better up is regression", false, 15, metricspb.MetricDiff_VERDICT_WORSE},
		{"lower-better down is improvement", false, -15, metricspb.MetricDiff_VERDICT_BETTER},
		{"higher-better unchanged is same", true, 0, metricspb.MetricDiff_VERDICT_SAME},
		{"lower-better unchanged is same", false, 0, metricspb.MetricDiff_VERDICT_SAME},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, verdict(tc.higherIsBetter, tc.diffPct))
		})
	}
}

func TestMetricDefExpr_FillsAllRunIDSlots(t *testing.T) {
	// error_rate references the run id twice; both must be filled and no %!q
	// formatting verb may leak.
	var errorRate metricDef
	for _, d := range catalog {
		if d.key == "error_rate" {
			errorRate = d
		}
	}
	require.NotEmpty(t, errorRate.key, "error_rate must exist in catalog")

	got := errorRate.expr("xyz")
	assert.Equal(t, 2, strings.Count(got, `run_id="xyz"`))
	assert.NotContains(t, got, "%q")
	assert.NotContains(t, got, "%!")
}
