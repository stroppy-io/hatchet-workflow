// Package metrics holds the per-DB PromQL query catalogue and the aggregation
// (collector) that turns a run's raw VictoriaMetrics series into MetricSummary
// values. Run isolation is by the stroppy_run_id label, which is globally
// unique, so all queries are tenant-agnostic.
package metrics

import (
	"fmt"
	"strings"
)

// RunLabel is the label injected by monitoring (vmagent external_labels) to
// identify a run. It is globally unique across all tenants, which is why the
// read path can safely query a single shared tenant space (accountID 0).
const RunLabel = "stroppy_run_id"

// MetricDef defines a named PromQL query template.
// Templates use %s for the run_id label filter and %p for the metric prefix.
type MetricDef struct {
	Name           string // human-readable name
	Key            string // stable key for comparison
	Query          string // PromQL template
	Unit           string // e.g. "ops/s", "ms", "bytes", "%"
	HigherIsBetter bool   // comparison direction surfaced to the UI
	Group          string // optional UI grouping label
}

func runFilter(runID string) string {
	return fmt.Sprintf(`%s="%s"`, RunLabel, runID)
}

// StroppyMetricPrefix converts a run ID to the PromQL-safe prefix injected into
// stroppy/k6 OTEL metric names. OTEL instrument names must start with a letter.
func StroppyMetricPrefix(runID string) string {
	return "stroppy_" + strings.ReplaceAll(runID, "-", "_")
}

// MetricsForDB returns metrics with DB-specific queries based on database kind.
// Recognised kinds: postgres, mysql, ydb, picodata, cockroach. Unknown kinds
// fall back to postgres. System + stroppy metrics are always appended.
func MetricsForDB(dbKind string) []MetricDef {
	var dbMetrics []MetricDef
	switch strings.ToLower(dbKind) {
	case "mysql":
		dbMetrics = mysqlMetrics()
	case "picodata":
		dbMetrics = picodataMetrics()
	case "ydb", "ydb_managed":
		dbMetrics = ydbMetrics()
	case "cockroach", "cockroachdb":
		dbMetrics = cockroachMetrics()
	default: // postgres
		dbMetrics = postgresMetrics()
	}
	return append(dbMetrics, systemMetrics()...)
}

// DefaultMetrics returns postgres metrics (backward compat).
func DefaultMetrics() []MetricDef {
	return MetricsForDB("postgres")
}

func postgresMetrics() []MetricDef {
	return []MetricDef{
		{
			Name:           "DB Transactions Per Second",
			Key:            "db_tps",
			Query:          `sum(rate(pg_stat_database_xact_commit{%s}[5m]) + rate(pg_stat_database_xact_rollback{%s}[5m]))`,
			Unit:           "txn/s",
			HigherIsBetter: true,
			Group:          "throughput",
		},
		{
			Name:           "DB Rows Fetched Per Second",
			Key:            "db_qps",
			Query:          `sum(rate(pg_stat_database_tup_fetched{%s}[5m]))`,
			Unit:           "rows/s",
			HigherIsBetter: true,
			Group:          "throughput",
		},
		{
			Name:  "DB Active Connections",
			Key:   "db_connections",
			Query: `sum(pg_stat_activity_count{%s})`,
			Unit:  "",
			Group: "connections",
		},
		{
			Name:  "DB Replication Lag",
			Key:   "db_repl_lag",
			Query: `max(pg_replication_lag_seconds{%s})`,
			Unit:  "s",
			Group: "replication",
		},
	}
}

func mysqlMetrics() []MetricDef {
	return []MetricDef{
		{
			Name:           "DB Transactions Per Second",
			Key:            "db_tps",
			Query:          `sum(rate(mysql_global_status_commands_total{command=~"commit|rollback",%s}[5m]))`,
			Unit:           "txn/s",
			HigherIsBetter: true,
			Group:          "throughput",
		},
		{
			Name:           "DB Queries Per Second",
			Key:            "db_qps",
			Query:          `sum(rate(mysql_global_status_queries{%s}[5m]))`,
			Unit:           "q/s",
			HigherIsBetter: true,
			Group:          "throughput",
		},
		{
			Name:  "DB Active Connections",
			Key:   "db_connections",
			Query: `sum(mysql_global_status_threads_connected{%s})`,
			Unit:  "",
			Group: "connections",
		},
		{
			Name:  "DB Replication Lag",
			Key:   "db_repl_lag",
			Query: `max(mysql_slave_status_seconds_behind_master{%s})`,
			Unit:  "s",
			Group: "replication",
		},
	}
}

func ydbMetrics() []MetricDef {
	// Metric names use the `<counter-group>_` prefix added by the vmagent
	// metric_relabel_config that mirrors the official ydb-platform Helm chart.
	return []MetricDef{
		{
			Name:           "DB gRPC Requests/s",
			Key:            "db_qps",
			Query:          `sum(rate(ydb_api_grpc_request_count{%s}[5m]))`,
			Unit:           "req/s",
			HigherIsBetter: true,
			Group:          "throughput",
		},
		{
			Name:  "DB gRPC Errors/s",
			Key:   "db_errors",
			Query: `sum(rate(ydb_api_grpc_response_count{status!="SUCCESS",%s}[5m]))`,
			Unit:  "err/s",
			Group: "errors",
		},
		{
			Name:           "DB Active Sessions",
			Key:            "db_sessions",
			Query:          `sum(kqp_SessionActors_Active{%s})`,
			Unit:           "",
			HigherIsBetter: true,
			Group:          "connections",
		},
		{
			Name:  "DB Query Latency p99",
			Key:   "db_latency_p99",
			Query: `histogram_quantile(0.99, sum by (le) (rate(ydb_table_query_execution_latency_milliseconds_bucket{%s}[5m])))`,
			Unit:  "ms",
			Group: "latency",
		},
		{
			Name:  "DB CPU Used (cores %)",
			Key:   "db_cpu",
			Query: `sum(ydb_resources_cpu_used_core_percents{%s})`,
			Unit:  "%",
			Group: "resources",
		},
		{
			Name:  "DB Storage Used",
			Key:   "db_storage",
			Query: `sum(ydb_resources_storage_used_bytes{%s})`,
			Unit:  "bytes",
			Group: "resources",
		},
	}
}

func picodataMetrics() []MetricDef {
	return []MetricDef{
		{
			Name:           "DB SQL Requests/s",
			Key:            "db_qps",
			Query:          `sum(rate(pico_sql_query_total{%s}[5m]))`,
			Unit:           "q/s",
			HigherIsBetter: true,
			Group:          "throughput",
		},
		{
			Name:  "DB SQL Errors/s",
			Key:   "db_errors",
			Query: `sum(rate(pico_sql_query_errors_total{%s}[5m]))`,
			Unit:  "err/s",
			Group: "errors",
		},
		{
			Name:  "DB Raft Commit Index",
			Key:   "db_raft_commit",
			Query: `max(pico_raft_commit_index{%s})`,
			Unit:  "",
			Group: "replication",
		},
		{
			// Picodata exposes no table-count metric; tables are tarantool
			// spaces, so count the per-space gauge instead.
			Name:  "DB Spaces Count",
			Key:   "db_tables",
			Query: `count(tnt_space_len{%s})`,
			Unit:  "",
			Group: "resources",
		},
	}
}

func cockroachMetrics() []MetricDef {
	// CockroachDB exposes Prometheus metrics on each node's /_status/vars.
	return []MetricDef{
		{
			Name:           "DB Queries Per Second",
			Key:            "db_qps",
			Query:          `sum(rate(sql_query_count{%s}[5m]))`,
			Unit:           "q/s",
			HigherIsBetter: true,
			Group:          "throughput",
		},
		{
			Name:           "DB Transactions Per Second",
			Key:            "db_tps",
			Query:          `sum(rate(sql_txn_commit_count{%s}[5m]) + rate(sql_txn_abort_count{%s}[5m]))`,
			Unit:           "txn/s",
			HigherIsBetter: true,
			Group:          "throughput",
		},
		{
			Name:  "DB SQL Errors/s",
			Key:   "db_errors",
			Query: `sum(rate(sql_failure_count{%s}[5m]))`,
			Unit:  "err/s",
			Group: "errors",
		},
		{
			Name:  "DB Open SQL Connections",
			Key:   "db_connections",
			Query: `sum(sql_conns{%s})`,
			Unit:  "",
			Group: "connections",
		},
		{
			Name:  "DB Service Latency p99",
			Key:   "db_latency_p99",
			Query: `histogram_quantile(0.99, sum by (le) (rate(sql_service_latency_bucket{%s}[5m]))) / 1e6`,
			Unit:  "ms",
			Group: "latency",
		},
		{
			Name:  "DB Live Bytes",
			Key:   "db_storage",
			Query: `sum(livebytes{%s})`,
			Unit:  "bytes",
			Group: "resources",
		},
	}
}

func systemMetrics() []MetricDef {
	return []MetricDef{

		// --- System ---
		// Filtered by stroppy_run_id label (set by vmagent external_labels).
		{
			Name:  "CPU Usage",
			Key:   "cpu_usage",
			Query: `clamp_min(100 - (avg(rate(node_cpu_seconds_total{mode="idle",%s}[5m])) * 100), 0)`,
			Unit:  "%",
			Group: "system",
		},
		{
			Name:  "Memory Usage",
			Key:   "memory_usage",
			Query: `avg((1 - node_memory_MemAvailable_bytes{%s} / node_memory_MemTotal_bytes) * 100)`,
			Unit:  "%",
			Group: "system",
		},
		{
			Name:  "Disk IO Read",
			Key:   "disk_read",
			Query: `sum(rate(node_disk_read_bytes_total{%s}[5m]))`,
			Unit:  "bytes/s",
			Group: "system",
		},
		{
			Name:  "Disk IO Write",
			Key:   "disk_write",
			Query: `sum(rate(node_disk_written_bytes_total{%s}[5m]))`,
			Unit:  "bytes/s",
			Group: "system",
		},
		{
			Name:  "Network Received",
			Key:   "net_rx",
			Query: `sum(rate(node_network_receive_bytes_total{%s}[5m]))`,
			Unit:  "bytes/s",
			Group: "system",
		},
		{
			Name:  "Network Transmitted",
			Key:   "net_tx",
			Query: `sum(rate(node_network_transmit_bytes_total{%s}[5m]))`,
			Unit:  "bytes/s",
			Group: "system",
		},

		// --- Stroppy (K6 OTEL metrics, prefixed with stroppy_<runID>_) ---
		{
			Name:           "Stroppy Active VUs",
			Key:            "stroppy_vus",
			Query:          `sum(%p_vus)`,
			Unit:           "",
			HigherIsBetter: true,
			Group:          "stroppy",
		},
		{
			Name:           "Stroppy Iterations/s",
			Key:            "stroppy_ops",
			Query:          `sum(rate(%p_iterations[30s]))`,
			Unit:           "iter/s",
			HigherIsBetter: true,
			Group:          "stroppy",
		},
		{
			Name:  "Stroppy Iteration Duration p99",
			Key:   "stroppy_iter_p99",
			Query: `histogram_quantile(0.99, sum by (le) (rate(%p_iteration_duration_bucket[30s])))`,
			Unit:  "ms",
			Group: "stroppy",
		},
		{
			Name:           "Stroppy Query Rate",
			Key:            "stroppy_query_rate",
			Query:          `sum(rate(%p_run_query_count[30s]))`,
			Unit:           "q/s",
			HigherIsBetter: true,
			Group:          "stroppy",
		},
		{
			Name:  "Stroppy Query Duration p99",
			Key:   "stroppy_latency_p99",
			Query: `histogram_quantile(0.99, sum by (le) (rate(%p_run_query_duration_bucket[30s])))`,
			Unit:  "ms",
			Group: "stroppy",
		},
		{
			Name:  "Stroppy Error Count",
			Key:   "stroppy_errors",
			Query: `sum(%p_run_query_error_rate)`,
			Unit:  "",
			Group: "stroppy",
		},
	}
}

// RenderQuery fills the run_id filter (%s) and metric prefix (%p) into a
// MetricDef query.
func RenderQuery(def MetricDef, runID string) string {
	q := def.Query
	q = strings.ReplaceAll(q, "%s", runFilter(runID))
	q = strings.ReplaceAll(q, "%p", StroppyMetricPrefix(runID))
	return q
}
