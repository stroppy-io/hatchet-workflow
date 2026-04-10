package api

import (
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "stroppy_http_requests_total", Help: "Total HTTP requests"},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "stroppy_http_request_duration_seconds", Help: "HTTP request duration", Buckets: prometheus.DefBuckets},
		[]string{"method", "path"},
	)
	activeRuns = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "stroppy_active_runs", Help: "Currently running DAG executions"},
	)
	wsConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "stroppy_ws_connections", Help: "Active WebSocket connections"},
	)
	agentCount = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "stroppy_agents_connected", Help: "Connected agents"},
	)
	goroutines = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{Name: "stroppy_goroutines", Help: "Number of goroutines"},
		func() float64 { return float64(runtime.NumGoroutine()) },
	)
	memAlloc = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{Name: "stroppy_memory_alloc_bytes", Help: "Allocated memory in bytes"},
		func() float64 {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			return float64(m.Alloc)
		},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration, activeRuns, wsConnections, agentCount, goroutines, memAlloc)
}

// metricsMiddleware records HTTP request metrics.
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip metrics/health/ws paths to reduce cardinality.
		path := r.URL.Path
		if path == "/metrics" || path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		// Normalize paths to reduce cardinality.
		if len(path) > 30 {
			path = path[:30]
		}
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		duration := time.Since(start).Seconds()
		httpRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(sw.status)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
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

func metricsHandler() http.Handler {
	return promhttp.Handler()
}
