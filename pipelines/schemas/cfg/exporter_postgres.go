package cfg

import (
	"strings"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// pgExporterCollector is one --collector.<name> flag of postgres_exporter.
type pgExporterCollector struct {
	name string
	// def is the upstream default: "on" for registerCollector(..., defaultEnabled, ...),
	// "off" for defaultDisabled.
	def string
	// product overrides the upstream default; empty means keep it.
	product string
	desc    string
}

// pgExporterCollectors mirrors the registerCollector calls of
// prometheus-community/postgres_exporter v0.20.1 (collector/pg_*.go): the
// collector name is the file's `<x>Subsystem` constant and the default is its
// defaultEnabled / defaultDisabled argument.
//
// doc: https://github.com/prometheus-community/postgres_exporter/tree/v0.20.1/collector
//
//nolint:gochecknoglobals // a static, immutable table of upstream facts
var pgExporterCollectors = []pgExporterCollector{
	{name: "database", def: "on", desc: "Per-database size and connection counts."},
	{name: "database_wraparound", def: "off", desc: "Transaction-id and multixact age per database."},
	{name: "locks", def: "on", desc: "Lock counts per database and lock mode."},
	{name: "long_running_transactions", def: "off", desc: "Age and count of long-running transactions."},
	{name: "postmaster", def: "off", desc: "Postmaster start time."},
	{name: "process_idle", def: "off", desc: "Histogram of idle backend times per application."},
	{name: "replication", def: "on", desc: "Replication lag and is-replica flag."},
	{name: "replication_slots", def: "on", desc: "Per-slot WAL retention and activity."},
	{name: "roles", def: "on", desc: "Per-role connection limits."},
	{name: "settings", def: "on", desc: "The server's pg_settings as metrics."},
	{name: "stat_activity", def: "on", desc: "Backend counts by state, from pg_stat_activity."},
	{name: "stat_activity_autovacuum", def: "off", desc: "Age of running autovacuum workers."},
	{name: "stat_archiver", def: "on", desc: "WAL archiver successes and failures."},
	{name: "stat_bgwriter", def: "on", desc: "Background writer and checkpoint counters."},
	{name: "stat_checkpointer", def: "off", desc: "pg_stat_checkpointer, the PostgreSQL 17+ split of the bgwriter view."},
	{name: "stat_database", def: "on", desc: "pg_stat_database: transactions, tuples, conflicts, deadlocks."},
	{name: "stat_progress_vacuum", def: "on", desc: "Progress of running VACUUMs."},
	{name: "stat_replication", def: "on", desc: "Per-walsender byte positions and lag."},
	{
		name: "stat_statements", def: "off", product: "on",
		desc: "Per-statement time and call counts. Upstream default off; on in stroppy because the query dashboards read it (it needs pg_stat_statements preloaded).",
	},
	{name: "stat_user_tables", def: "on", desc: "Per-table scan, tuple and vacuum counters."},
	{name: "stat_wal_receiver", def: "off", desc: "Standby-side WAL receiver state."},
	{name: "statio_user_indexes", def: "off", desc: "Per-index block read/hit counters."},
	{name: "statio_user_tables", def: "on", desc: "Per-table block read/hit counters."},
	{name: "wal", def: "on", desc: "WAL segment count and size."},
	{name: "xlog_location", def: "off", desc: "Current WAL LSN as a number."},
	{name: "buffercache_summary", def: "off", desc: "pg_buffercache_summary; needs the pg_buffercache extension."},
}

// ExporterPostgres1 is cfg.exporter.postgres@1 — the flags and environment of
// prometheus-community/postgres_exporter.
//
// Flag names, defaults and deprecations verified against
// https://github.com/prometheus-community/postgres_exporter (v0.20.1,
// cmd/postgres_exporter/main.go and collector/). The rendered "conf" is a
// systemd EnvironmentFile: the DATA_SOURCE_* and PG_EXPORTER_* variables plus
// one POSTGRES_EXPORTER_OPTS line holding the command-line flags.
//
//nolint:funlen // one flat key table
func ExporterPostgres1() *schemapb.Schema {
	fields := []schemapb.FieldDef{
		// doc: main.go — DATA_SOURCE_NAME
		schemapb.Str("data_source_name").Title("DATA_SOURCE_NAME").Group("Cluster").
			Desc("libpq connection string or URI the exporter scrapes, e.g. postgresql://exporter@10.0.0.11:5432/postgres?sslmode=disable. Filled by the server from the topology; it carries a password, so it is a secret.").
			MinLen(1).MaxLen(2048).Secret().Nullable(),

		// doc: main.go — web.listen-address, kingpinflag default :9187
		schemapb.Int64("web_listen_port").Title("Listen port").Group("Web").
			Desc("Port the exporter serves /metrics on; rendered as --web.listen-address=:<port>.").
			Gte(1).Lte(65535).Default(9187),
		// doc: main.go — web.telemetry-path, default /metrics
		schemapb.Str("web_telemetry_path").Title("Telemetry path").Group("Web").
			Desc("HTTP path the metrics are exposed under.").
			Pattern(`^/.*$`).Default("/metrics"),

		// doc: promslog flag.AddFlags — log.level, default info
		schemapb.Choice("log_level").Title("Log level").Group("Logging").
			Desc("Exporter log verbosity.").
			Opt(schemapb.StrV("debug"), "debug").Opt(schemapb.StrV("info"), "info").
			Opt(schemapb.StrV("warn"), "warn").Opt(schemapb.StrV("error"), "error").
			Default(schemapb.StrV("info")),
		// doc: promslog flag.AddFlags — log.format, default logfmt
		schemapb.Choice("log_format").Title("Log format").Group("Logging").
			Desc("Exporter log encoding.").
			Opt(schemapb.StrV("logfmt"), "logfmt").Opt(schemapb.StrV("json"), "json").
			Default(schemapb.StrV("logfmt")),

		// doc: main.go — disable-default-metrics, default false
		onOff("disable_default_metrics", "off").Title("Disable default metrics").Group("Metrics").
			Desc("Drop every built-in metric and export only the custom queries."),
		// doc: main.go — metric-prefix, default pg
		schemapb.Str("metric_prefix").Title("Metric prefix").Group("Metrics").
			Desc("Prefix of every exported metric name.").
			Pattern(`^[a-z_][a-z0-9_]*$`).Default("pg"),
		// doc: main.go — collection-timeout, default 1m
		schemapb.Str("collection_timeout").Title("Collection timeout").Group("Metrics").
			Desc("Timeout for one scrape, as a Go duration; a slow database is reported as a failed scrape instead of stalling Prometheus.").
			Pattern(`^[0-9]+(ms|s|m|h)$`).Default("1m"),

		// doc: main.go — auto-discover-databases (DEPRECATED), default false
		onOff("auto_discover_databases", "off").Title("Auto-discover databases").Group("Discovery").
			Desc("Scrape every database on the server, not just the one in DATA_SOURCE_NAME. Deprecated upstream.").
			Deprecated(),
		// doc: main.go — exclude-databases (DEPRECATED), default ""
		schemapb.Str("exclude_databases").Title("Exclude databases").Group("Discovery").
			Desc("Comma-separated databases to skip when auto-discovery is on. Deprecated upstream.").
			MaxLen(1024).Default("template0,template1").Deprecated(),
		// doc: main.go — include-databases (DEPRECATED), default ""
		schemapb.Str("include_databases").Title("Include databases").Group("Discovery").
			Desc("Comma-separated databases to restrict auto-discovery to. Deprecated upstream.").
			MaxLen(1024).Default("").Deprecated(),
	}

	var flagExprs []string

	for _, c := range pgExporterCollectors {
		def := c.def
		desc := c.desc

		if c.product != "" {
			def = c.product
		}

		fields = append(fields,
			onOff(schemapb.FieldName("collector_"+c.name), def).
				Title("collector."+c.name).Group("Collectors").
				Desc(desc+" Upstream default: "+c.def+"."))

		flagExprs = append(flagExprs,
			`(root.collector_`+c.name+` == "on" ? "--collector.`+c.name+`" : "--no-collector.`+c.name+`")`)
	}

	fields = append(fields,
		schemapb.Computed("collector_flags", "["+strings.Join(flagExprs, ",\n")+`].join(" ")`).
			Title("Collector flags").Group("Collectors").
			Desc("Every collector rendered as --collector.<name> or --no-collector.<name>, in table order.").
			Result(schemapb.ResultString),

		schemapb.Computed("opts", `"--web.listen-address=:" + string(root.web_listen_port)`+
			` + " --web.telemetry-path=" + root.web_telemetry_path`+
			` + " --log.level=" + root.log_level`+
			` + " --log.format=" + root.log_format`+
			` + " --metric-prefix=" + root.metric_prefix`+
			` + " --collection-timeout=" + root.collection_timeout`+
			` + (root.disable_default_metrics == "on" ? " --disable-default-metrics" : "")`+
			` + (root.auto_discover_databases == "on" ? " --auto-discover-databases" : "")`+
			` + (root.exclude_databases != "" ? " --exclude-databases=" + root.exclude_databases : "")`+
			` + (root.include_databases != "" ? " --include-databases=" + root.include_databases : "")`+
			` + " " + root.collector_flags`).
			Title("Command line").Group("Collectors").
			Desc("The full postgres_exporter argument list.").
			Result(schemapb.ResultString),
	)

	return schemapb.NewSchema(ids.Cfg("exporter.postgres", 1)).
		Descr("postgres_exporter flags and environment (prometheus-community/postgres_exporter v0.20.x).").
		Strict().Coerce().
		Fields(fields...).
		Rules(
			schemapb.Rule(`root.auto_discover_databases == "on" || root.include_databases == ""`,
				"include_databases only applies when auto_discover_databases is on").
				ID("include-needs-discovery"),
			schemapb.Rule(`root.collector_stat_statements == "off" || root.disable_default_metrics == "off"`,
				"the stat_statements collector produces nothing with default metrics disabled").
				ID("stat-statements-needs-defaults"),
		).
		Template("conf", `# postgres_exporter environment — generated by stroppy.
{{#values.data_source_name}}DATA_SOURCE_NAME={{{.}}}
{{/values.data_source_name}}PG_EXPORTER_WEB_TELEMETRY_PATH={{{values.web_telemetry_path}}}
PG_EXPORTER_METRIC_PREFIX={{{values.metric_prefix}}}
PG_EXPORTER_COLLECTION_TIMEOUT={{{values.collection_timeout}}}
POSTGRES_EXPORTER_OPTS={{{values.opts}}}
`).
		MustBuild()
}
