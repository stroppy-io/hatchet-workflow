package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus collectors mirror the metric names used on `main` so existing
// dashboards keep working.
var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "stroppy_http_requests_total", Help: "Total HTTP requests"},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "stroppy_http_request_duration_seconds",
			Help:    "HTTP request duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
	ActiveRuns = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "stroppy_active_runs", Help: "Currently running DAG executions"},
	)
	AgentsConnected = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "stroppy_agents_connected", Help: "Connected agents"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration, ActiveRuns, AgentsConnected)
}

// MetricsHandler returns the /metrics scrape endpoint.
func MetricsHandler() http.Handler { return promhttp.Handler() }

// MetricsHTTP wraps an http.Handler to record request counters + duration.
// Skips /metrics, /healthz, and any /ws/ upgrade path so it doesn't double-count
// scrape traffic or break websocket hijack.
func MetricsHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/metrics" || path == "/healthz" || path == "/health" ||
			strings.HasPrefix(path, "/ws/") {
			next.ServeHTTP(w, r)
			return
		}
		// Cardinality cap.
		if len(path) > 64 {
			path = path[:64]
		}
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		dur := time.Since(start).Seconds()
		httpRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(sw.status)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, path).Observe(dur)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
