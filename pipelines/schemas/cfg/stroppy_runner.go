package cfg

import (
	"time"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// StroppyRunner is cfg.stroppy.runner@1 — the process environment of the
// stroppy load generator on the runner machine. Rendered as KEY=VALUE lines
// (an env-file / docker --env-file).
//
// Everything here is checked against the stroppy checkout itself, which is
// the authority for its own knobs:
//   - env var names and the k6/OTEL bridge:
//     stroppy/internal/runner/script_runner.go (K6_OTEL_*, K6_LOG_FORMAT)
//   - LOG_LEVEL and the config-file/driver precedence:
//     stroppy/cmd/stroppy/commands/help/topic_config_file.go
//   - the driver JSON put into STROPPY_DRIVER_N, its option keys and value
//     domains: stroppy/cmd/stroppy/commands/help/topic_drivers.go
//
// The driver block is a discriminated union on driver type, because the TLS /
// auth options only exist for the wire protocols that have them.
func StroppyRunner() *schemapb.Schema {
	return schemapb.NewSchema(ids.Cfg("stroppy.runner", 1)).
		Descr("Environment of the stroppy runner process: logging, OTLP export, pool and driver options.").
		Strict().Coerce().
		Fields(
			// doc: topic_config_file.go — "LOG_LEVEL=debug stroppy run ..."
			schemapb.Choice("log_level").Title("Log level").Group("Logging").
				Desc("LOG_LEVEL. debug traces how every parameter was resolved; it also turns on k6 --verbose.").
				Opt(schemapb.StrV("debug"), "debug").
				Opt(schemapb.StrV("info"), "info").
				Opt(schemapb.StrV("warn"), "warn").
				Opt(schemapb.StrV("error"), "error").
				Default(schemapb.StrV("info")),
			// doc: script_runner.go — LOG_MODE_PRODUCTION => K6_LOG_FORMAT=json
			schemapb.Choice("log_format").Title("Log format").Group("Logging").
				Desc("K6_LOG_FORMAT of the embedded k6 process. json is what the agent ships to the log store.").
				Opt(schemapb.StrV("json"), "json").
				Opt(schemapb.StrV("text"), "text").
				Default(schemapb.StrV("json")),

			// doc: script_runner.go:415-439
			schemapb.Choice("otlp_protocol").Title("OTLP protocol").Group("Telemetry").
				Desc("K6_OTEL_EXPORTER_PROTOCOL.").
				Opt(schemapb.StrV("grpc"), "gRPC").
				Opt(schemapb.StrV("http/protobuf"), "HTTP/protobuf").
				Default(schemapb.StrV("grpc")),
			schemapb.Str("otlp_endpoint").Title("OTLP endpoint").Group("Cluster").
				Desc("K6_OTEL_GRPC_EXPORTER_ENDPOINT / K6_OTEL_HTTP_EXPORTER_ENDPOINT. Filled by the server: the agent's OTLP receiver on this machine.").
				MaxLen(256).Nullable().Examples(schemapb.StrV("127.0.0.1:4317")),
			schemapb.Str("otlp_http_url_path").Title("OTLP HTTP path").Group("Telemetry").
				Desc("K6_OTEL_HTTP_EXPORTER_URL_PATH; only read for the http/protobuf protocol.").
				When(`root.otlp_protocol == "http/protobuf"`).
				Pattern(`^/.*$`).Default("/v1/metrics"),
			schemapb.Bool("otlp_insecure").Title("OTLP insecure").Group("Telemetry").
				Desc("K6_OTEL_GRPC_EXPORTER_INSECURE / K6_OTEL_HTTP_EXPORTER_INSECURE. The endpoint is a local agent, so plaintext is the norm.").
				Default(true),
			schemapb.Str("otlp_service_name").Title("OTLP service name").Group("Telemetry").
				Desc("K6_OTEL_SERVICE_NAME.").
				MinLen(1).MaxLen(64).Default("stroppy"),
			schemapb.Str("otlp_metric_prefix").Title("OTLP metric prefix").Group("Telemetry").
				Desc("K6_OTEL_METRIC_PREFIX prepended to every k6 metric name.").
				Pattern(`^[a-z][a-z0-9_]*$`).Default("k6_"),
			// Upstream carries per-export key/values as OTLP headers; there is
			// no separate label env var, so run/tenant labels ride here.
			schemapb.MapOf("otlp_headers", schemapb.Str("value").MaxLen(256)).
				Title("OTLP headers").Group("Cluster").
				Desc("K6_OTEL_HEADERS, comma-joined k=v. The server fills the run/tenant identity here (there is no separate label env var upstream).").
				MaxEntries(16).
				Rules(schemapb.Rule(
					`this.all(k, k.matches("^[A-Za-z0-9_-]+$"))`,
					"header names are [A-Za-z0-9_-]").ID("header-name-shape")),
			// TODO verify with stroppy: xk6-output-opentelemetry exposes an
			// export interval, but no stroppy code path sets it today.
			schemapb.Duration("metrics_flush_interval").Title("Metrics flush interval").Group("Telemetry").
				Desc("K6_OTEL_EXPORT_INTERVAL: how often k6 pushes metric batches to the collector.").
				Gte(time.Second).Lte(5*time.Minute).Default(10*time.Second),

			schemapb.Int64("threads").Title("GOMAXPROCS").Group("Process").
				Desc("GOMAXPROCS of the stroppy/k6 process; 0 leaves the Go default (one per CPU) and emits no variable.").
				Gte(0).Lte(4096).Default(0),

			// doc: topic_drivers.go — pool.* keys of the driver JSON
			schemapb.Int64("pool_max_conns").Title("Pool max connections").Group("Pool").
				Desc("driver pool.maxConns. Keep it at or above the k6 VU count or VUs queue on the pool instead of the database.").
				Gte(1).Lte(100000).Default(100),
			schemapb.Int64("pool_min_conns").Title("Pool min connections").Group("Pool").
				Desc("driver pool.minConns: connections kept warm; equal to max removes connect latency from the measurement.").
				Gte(0).Lte(100000).Default(100),
			schemapb.Duration("pool_conn_lifetime").Title("Connection lifetime").Group("Pool").
				Desc("driver pool.maxConnLifetime.").
				Gte(time.Minute).Lte(24*time.Hour).Default(time.Hour),
			schemapb.Duration("pool_conn_idle_time").Title("Connection idle time").Group("Pool").
				Desc("driver pool.maxConnIdleTime.").
				Gte(time.Second).Lte(24*time.Hour).Default(30*time.Minute),

			// doc: topic_drivers.go — insertProgress.*, errorMode, bulkSize
			schemapb.Duration("insert_progress_interval").Title("Insert progress interval").Group("Timeouts").
				Desc("driver insertProgress.interval: how often the load phase reports row progress (upstream default 10s).").
				Gte(time.Second).Lte(time.Hour).Default(30*time.Second),
			schemapb.Duration("insert_progress_stall_after").Title("Stall warning after").Group("Timeouts").
				Desc("driver insertProgress.stallAfter: warn when no rows moved for this long (upstream default 60s).").
				Gte(time.Second).Lte(time.Hour).Default(2*time.Minute),
			schemapb.Choice("insert_progress_mode").Title("Insert progress mode").Group("Timeouts").
				Desc("driver insertProgress.mode.").
				Opt(schemapb.StrV("both"), "log + metrics").
				Opt(schemapb.StrV("log"), "log").
				Opt(schemapb.StrV("metrics"), "metrics").
				Opt(schemapb.StrV("off"), "off").
				Default(schemapb.StrV("both")),
			schemapb.Choice("error_mode").Title("Error mode").Group("Driver").
				Desc("driver errorMode: what a failing statement does to the run.").
				Opt(schemapb.StrV("silent"), "silent").
				Opt(schemapb.StrV("log"), "log").
				Opt(schemapb.StrV("throw"), "throw").
				Opt(schemapb.StrV("fail"), "fail").
				Opt(schemapb.StrV("abort"), "abort").
				Default(schemapb.StrV("log")),
			schemapb.Int64("bulk_size").Title("Bulk size").Group("Driver").
				Desc("driver bulkSize: rows per bulk INSERT during load_data (upstream default 2500).").
				Gte(1).Lte(1000000).Default(2500),
			schemapb.Choice("default_insert_method").Title("Insert method").Group("Driver").
				Desc("driver defaultInsertMethod; the fastest method differs per driver (native for pg/ydb, plain_bulk for mysql/picodata).").
				Opt(schemapb.StrV("native"), "native").
				Opt(schemapb.StrV("plain_bulk"), "plain_bulk").
				Opt(schemapb.StrV("plain_query"), "plain_query").
				Default(schemapb.StrV("native")),
			schemapb.Str("driver_url").Title("Driver URL").Group("Cluster").
				Desc("driver url. Filled by the server from topology (the proxy or primary endpoint of the deployed database).").
				MaxLen(1024).Nullable().
				Examples(schemapb.StrV("postgres://stroppy:stroppy@10.0.0.10:5432/bench")),

			// Protocol-specific options: TLS/auth only exist where the wire
			// protocol has them (topic_drivers.go, "TLS / Authentication").
			schemapb.OneOf("driver", "driverType").
				Variant("postgres",
					schemapb.Choice("ssl_mode").Title("sslmode").Group("Driver").
						Desc("sslmode of the libpq URL; the driver URL carries it as a query parameter.").
						Opt(schemapb.StrV("disable"), "disable").
						Opt(schemapb.StrV("require"), "require").
						Opt(schemapb.StrV("verify-ca"), "verify-ca").
						Opt(schemapb.StrV("verify-full"), "verify-full").
						Default(schemapb.StrV("disable")),
				).
				Variant("picodata",
					schemapb.Choice("ssl_mode").Title("sslmode").Group("Driver").
						Desc("Picodata speaks the PostgreSQL wire protocol on its pg port, so sslmode applies verbatim.").
						Opt(schemapb.StrV("disable"), "disable").
						Opt(schemapb.StrV("require"), "require").
						Default(schemapb.StrV("disable")),
				).
				Variant("mysql",
					schemapb.Choice("tls").Title("tls").Group("Driver").
						Desc("go-sql-driver/mysql tls parameter of the DSN.").
						Opt(schemapb.StrV("false"), "false").
						Opt(schemapb.StrV("preferred"), "preferred").
						Opt(schemapb.StrV("skip-verify"), "skip-verify").
						Opt(schemapb.StrV("true"), "true").
						Default(schemapb.StrV("false")),
				).
				Variant("ydb",
					schemapb.Bool("grpcs").Title("Use grpcs").Group("Driver").
						Desc("TLS is switched on by the URL scheme (grpcs://); this only records the intent for the URL the server builds.").
						Default(false),
					schemapb.Str("ca_cert_file").Title("CA certificate").Group("Driver").
						Desc("driver caCertFile: PEM path, needed only for a private CA.").
						MaxLen(256).Default(""),
					schemapb.Str("auth_user").Title("Auth user").Group("Driver").
						Desc("driver authUser for static credentials.").MaxLen(128).Default(""),
					schemapb.Str("auth_password").Title("Auth password").Group("Driver").
						Desc("driver authPassword for static credentials.").MaxLen(256).Secret().Default(""),
					schemapb.Str("auth_token").Title("Auth token").Group("Driver").
						Desc("driver authToken (IAM token) for managed YDB.").MaxLen(4096).Secret().Default(""),
					schemapb.Bool("tls_insecure_skip_verify").Title("Skip TLS verify").Group("Driver").
						Desc("driver tlsInsecureSkipVerify — testing only.").Default(false),
				).
				Variant("noop").
				Title("Driver").Group("Driver").
				Desc("Which stroppy driver the workload talks through; the protocol decides which TLS/auth options exist.").
				Required(),

			// STROPPY_DRIVER_0 is a JSON document, and Mustache has one level
			// of context: the document is assembled here and printed as one
			// value.
			schemapb.Computed("driver_json",
				`'{"driverType":"' + root.driver.driverType + '"' +
				 (("driver_url" in root) ? ',"url":"' + root.driver_url + '"' : "") +
				 ',"defaultInsertMethod":"' + root.default_insert_method + '"' +
				 ',"errorMode":"' + root.error_mode + '"' +
				 ',"bulkSize":' + string(root.bulk_size) +
				 ',"insertProgress":{"enabled":' + (root.insert_progress_mode != "off" ? "true" : "false") +
				 ',"interval":"' + string(root.insert_progress_interval) +
				 '","stallAfter":"' + string(root.insert_progress_stall_after) +
				 '","mode":"' + root.insert_progress_mode + '"}' +
				 ',"pool":{"maxConns":' + string(root.pool_max_conns) +
				 ',"minConns":' + string(root.pool_min_conns) +
				 ',"maxConnLifetime":"' + string(root.pool_conn_lifetime) +
				 '","maxConnIdleTime":"' + string(root.pool_conn_idle_time) + '"}' +
				 (("ca_cert_file" in root.driver) && root.driver.ca_cert_file != ""
					? ',"caCertFile":"' + root.driver.ca_cert_file + '"' : "") +
				 (("auth_user" in root.driver) && root.driver.auth_user != ""
					? ',"authUser":"' + root.driver.auth_user + '"' : "") +
				 (("auth_password" in root.driver) && root.driver.auth_password != ""
					? ',"authPassword":"' + root.driver.auth_password + '"' : "") +
				 (("auth_token" in root.driver) && root.driver.auth_token != ""
					? ',"authToken":"' + root.driver.auth_token + '"' : "") +
				 (("tls_insecure_skip_verify" in root.driver) && root.driver.tls_insecure_skip_verify
					? ',"tlsInsecureSkipVerify":true' : "") +
				 "}"`).
				Result(schemapb.ResultString).Group("Driver").
				Title("Rendered STROPPY_DRIVER_0").Desc("Derived: the driver JSON stroppy reads from STROPPY_DRIVER_0."),
			schemapb.Computed("otlp_endpoint_lines",
				`!("otlp_endpoint" in root) ? "" :
				 (root.otlp_protocol == "grpc"
					? "K6_OTEL_GRPC_EXPORTER_ENDPOINT=" + root.otlp_endpoint + "\n" +
					  "K6_OTEL_GRPC_EXPORTER_INSECURE=" + (root.otlp_insecure ? "true" : "false")
					: "K6_OTEL_HTTP_EXPORTER_ENDPOINT=" + root.otlp_endpoint + "\n" +
					  "K6_OTEL_HTTP_EXPORTER_INSECURE=" + (root.otlp_insecure ? "true" : "false") + "\n" +
					  "K6_OTEL_HTTP_EXPORTER_URL_PATH=" + root.otlp_http_url_path)`).
				Result(schemapb.ResultString).Group("Cluster").Title("Rendered OTLP endpoint lines"),
			schemapb.Computed("otlp_headers_line",
				`!("otlp_headers" in root) || size(root.otlp_headers) == 0 ? "" :
				 "K6_OTEL_HEADERS=" + root.otlp_headers.map(k, k + "=" + string(root.otlp_headers[k])).join(",")`).
				Result(schemapb.ResultString).Group("Cluster").Title("Rendered K6_OTEL_HEADERS"),
			schemapb.Computed("gomaxprocs_line",
				`root.threads == 0 ? "" : "GOMAXPROCS=" + string(root.threads)`).
				Result(schemapb.ResultString).Group("Process").Title("Rendered GOMAXPROCS"),
		).
		Rules(
			schemapb.Rule("int(root.pool_min_conns) <= int(root.pool_max_conns)",
				"pool.minConns must be <= pool.maxConns").ID("pool-min-le-max"),
			schemapb.Rule("root.insert_progress_interval <= root.insert_progress_stall_after",
				"the stall warning must be no earlier than one progress report").ID("progress-interval-le-stall"),
		).
		Template("conf", `# managed by stroppy-cloud — cfg.stroppy.runner@1
LOG_LEVEL={{{values.log_level}}}
K6_LOG_FORMAT={{{values.log_format}}}
K6_OTEL_SERVICE_NAME={{{values.otlp_service_name}}}
K6_OTEL_METRIC_PREFIX={{{values.otlp_metric_prefix}}}
K6_OTEL_EXPORTER_PROTOCOL={{{values.otlp_protocol}}}
K6_OTEL_EXPORT_INTERVAL={{{values.metrics_flush_interval}}}
{{#values.otlp_endpoint_lines}}{{{values.otlp_endpoint_lines}}}
{{/values.otlp_endpoint_lines}}{{#values.otlp_headers_line}}{{{values.otlp_headers_line}}}
{{/values.otlp_headers_line}}{{#values.gomaxprocs_line}}{{{values.gomaxprocs_line}}}
{{/values.gomaxprocs_line}}STROPPY_DRIVER_0={{{values.driver_json}}}
`).
		MustBuild()
}
