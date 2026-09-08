package cfg

import (
	"strings"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// cfg.exporter.mysqld@1 — prometheus/mysqld_exporter 0.15/0.16 command line.
//
// doc: https://github.com/prometheus/mysqld_exporter/blob/main/README.md
// The collector toggles keep the historic `collect.` prefix (NOT `collector.`)
// and every one of them has a `--no-collect.<name>` counterpart, which is how
// a toggle that is on by default gets turned off.

// mysqldCollector is one --collect.<flag> toggle: an on/off choice whose
// rendered flag is built by mysqldFlagExpr.
type mysqldCollector struct {
	field string // schema field name
	flag  string // collect.<flag> suffix
	on    bool   // upstream default
	desc  string
}

//nolint:gochecknoglobals // static table of the exporter's collectors
var mysqldCollectors = []mysqldCollector{
	{"global_status", "global_status", true, "SHOW GLOBAL STATUS — the core counter set."},
	{"global_variables", "global_variables", true, "SHOW GLOBAL VARIABLES — server configuration as metrics."},
	{"slave_status", "slave_status", true, "SHOW REPLICA/SLAVE STATUS — replication lag and errors."},
	{"info_schema_innodb_metrics", "info_schema.innodb_metrics", false, "information_schema.innodb_metrics — the detailed InnoDB counters."},
	{"info_schema_innodb_cmp", "info_schema.innodb_cmp", true, "information_schema.innodb_cmp — compression statistics."},
	{"info_schema_innodb_cmpmem", "info_schema.innodb_cmpmem", true, "information_schema.innodb_cmpmem — compressed buffer pool statistics."},
	{"info_schema_processlist", "info_schema.processlist", false, "information_schema.processlist — per-state connection counts; costly on busy servers."},
	{"info_schema_tables", "info_schema.tables", true, "information_schema.tables — per-table size and row counts."},
	{"info_schema_query_response_time", "info_schema.query_response_time", true, "Query response time histogram (Percona/MariaDB only)."},
	{"perf_schema_eventswaits", "perf_schema.eventswaits", false, "performance_schema wait events summary."},
	{"perf_schema_eventsstatements", "perf_schema.eventsstatements", false, "performance_schema statement digests — the per-query latency source."},
	{"perf_schema_tableiowaits", "perf_schema.tableiowaits", false, "performance_schema table I/O waits."},
	{"perf_schema_indexiowaits", "perf_schema.indexiowaits", false, "performance_schema index I/O waits."},
	{"perf_schema_file_events", "perf_schema.file_events", false, "performance_schema file I/O events."},
	{"binlog_size", "binlog_size", false, "Total size of the binary logs on disk."},
	{"engine_innodb_status", "engine_innodb_status", false, "SHOW ENGINE INNODB STATUS — parsed into metrics."},
	{"auto_increment_columns", "auto_increment.columns", false, "Auto-increment column headroom per table."},
	{"heartbeat", "heartbeat", false, "pt-heartbeat table based replication delay."},
}

// ExporterMysqld is cfg.exporter.mysqld@1.
//
//nolint:funlen // one flat schema definition
func ExporterMysqld() *schemapb.Schema {
	fields := []schemapb.FieldDef{
		// doc: README — --mysqld.address (default localhost:3306)
		schemapb.Str("mysqld_address").Title("MySQL address").Group("Target").
			Desc("host:port of the mysqld to scrape; filled by the server from topology.").
			MaxLen(255).Nullable(),
		// doc: README — --mysqld.username
		schemapb.Str("mysqld_username").Title("MySQL user").Group("Target").
			Desc("Account used for scraping; needs PROCESS, REPLICATION CLIENT and SELECT.").
			MinLen(1).MaxLen(64).Default("exporter"),
		// doc: README — MYSQLD_EXPORTER_PASSWORD / the client section of --config.my-cnf
		schemapb.Str("mysqld_password").Title("MySQL password").Group("Target").
			Desc("Password for that account; filled by the server. Never rendered into the flags file — it is passed as MYSQLD_EXPORTER_PASSWORD or written into config.my-cnf.").
			Secret().MaxLen(255).Nullable(),
		// doc: README — --config.my-cnf (default ~/.my.cnf)
		schemapb.Str("config_my_cnf").Title("Credentials file").Group("Target").
			Desc("my.cnf whose [client] section holds the credentials; its options override the flags.").
			MinLen(1).MaxLen(255).Default("/etc/mysqld_exporter/.my.cnf"),

		// doc: README — --web.listen-address
		schemapb.Int64("listen_port").Title("Listen port").Group("Exporter").
			Desc("Port of the /metrics endpoint (--web.listen-address).").
			Gte(1).Lte(65535).Default(9104),
		// doc: README — --log.level
		schemapb.Choice("log_level").Title("Log level").Group("Exporter").
			Desc("Exporter log verbosity.").
			Opt(schemapb.StrV("debug"), "debug").
			Opt(schemapb.StrV("info"), "info").
			Opt(schemapb.StrV("warn"), "warn").
			Opt(schemapb.StrV("error"), "error").
			Default(schemapb.StrV("info")),
		// doc: README — --exporter.max_open_conns (connection pool, default 3)
		schemapb.Int64("max_open_connections").Title("Max open connections").Group("Exporter").
			Desc("Size of the exporter's connection pool to mysqld.").
			Gte(1).Lte(64).Default(3),
		// doc: README — --exporter.query_timeout (seconds, 0 = disabled)
		schemapb.Int64("query_timeout").Title("Query timeout").Group("Exporter").
			Desc("Per-scraper query timeout; 0 disables it.").
			Unit("s").Gte(0).Lte(600).Default(0),
	}

	for _, c := range mysqldCollectors {
		def := "OFF"
		if c.on {
			def = "ON"
		}

		fields = append(fields,
			myOnOff(schemapb.FieldName(c.field)).
				Title("collect."+c.flag).Group("Collectors").
				Desc(c.desc+" Upstream default "+def+".").
				Default(schemapb.StrV(def)),
		)
	}

	fields = append(fields,
		customField().MaxEntries(64),
		schemapb.Computed("collector_flags", mysqldFlagExpr()).
			Result(schemapb.ResultString).Group("Rendered").
			Title("Collector flags").
			Desc("The collect.* toggles rendered as --collect.x / --no-collect.x, one per line."),
		schemapb.Computed("custom_rendered",
			`("custom" in root) ? root.custom.map(k, "--" + k + "=" + string(root.custom[k])).join("\n") : ""`).
			Result(schemapb.ResultString).Group("Custom").
			Title("Rendered extra flags").
			Desc("The `custom` map joined into --key=value flags."),
		schemapb.Computed("target_flags",
			`(("mysqld_address" in root) ? "--mysqld.address=" + string(root.mysqld_address) + "\n" : "")`).
			Result(schemapb.ResultString).Group("Rendered").
			Title("Target flags").
			Desc("--mysqld.address, emitted only once topology filled it in."),
	)

	return schemapb.NewSchema(ids.Cfg("exporter.mysqld", 1)).
		Descr("prometheus/mysqld_exporter 0.15/0.16 command line, rendered as one flag per line.").
		Strict().Coerce().
		Fields(fields...).
		Rules(customKeyRule()).
		Template("conf", `# mysqld_exporter — generated by stroppy, do not edit by hand
{{{values.target_flags}}}--mysqld.username={{{values.mysqld_username}}}
--config.my-cnf={{{values.config_my_cnf}}}
--web.listen-address=:{{{values.listen_port}}}
--log.level={{{values.log_level}}}
--exporter.max_open_conns={{{values.max_open_connections}}}
--exporter.query_timeout={{{values.query_timeout}}}
{{{values.collector_flags}}}
{{{values.custom_rendered}}}
`).
		MustBuild()
}

// mysqldFlagExpr builds the CEL that turns every collector choice into its
// --collect./--no-collect. flag.
func mysqldFlagExpr() string {
	parts := make([]string, 0, len(mysqldCollectors))

	for _, c := range mysqldCollectors {
		parts = append(parts, `((`+quoteCEL(c.field)+` in root) ? (root[`+quoteCEL(c.field)+`] == "ON" ? "--collect.`+c.flag+`\n" : "--no-collect.`+c.flag+`\n") : "")`)
	}

	return strings.Join(parts, " +\n")
}

// quoteCEL quotes a bare identifier as a CEL string literal.
func quoteCEL(s string) string { return `"` + s + `"` }
