// GENERATED from schemapb schema cfg.stroppy.runner@1 — do not edit.
// Environment of the stroppy runner process: logging, OTLP export, pool and driver options.

/** variant mysql of driver */
export interface CfgStroppyRunner1DriverMysql {
  driverType: "mysql";
  /** tls. go-sql-driver/mysql tls parameter of the DSN. */
  tls?: "false" | "preferred" | "skip-verify" | "true";
}

/** variant noop of driver */
export interface CfgStroppyRunner1DriverNoop {
  driverType: "noop";
}

/** variant picodata of driver */
export interface CfgStroppyRunner1DriverPicodata {
  driverType: "picodata";
  /** sslmode. Picodata speaks the PostgreSQL wire protocol on its pg port, so sslmode applies verbatim. */
  ssl_mode?: "disable" | "require";
}

/** variant postgres of driver */
export interface CfgStroppyRunner1DriverPostgres {
  driverType: "postgres";
  /** sslmode. sslmode of the libpq URL; the driver URL carries it as a query parameter. */
  ssl_mode?: "disable" | "require" | "verify-ca" | "verify-full";
}

/** variant ydb of driver */
export interface CfgStroppyRunner1DriverYdb {
  driverType: "ydb";
  /** Use grpcs. TLS is switched on by the URL scheme (grpcs://); this only records the intent for the URL the server builds. */
  grpcs?: boolean;
  /** CA certificate. driver caCertFile: PEM path, needed only for a private CA. */
  ca_cert_file?: string;
  /** Auth user. driver authUser for static credentials. */
  auth_user?: string;
  /** Auth password. driver authPassword for static credentials. */
  auth_password?: string;
  /** Auth token. driver authToken (IAM token) for managed YDB. */
  auth_token?: string;
  /** Skip TLS verify. driver tlsInsecureSkipVerify — testing only. */
  tls_insecure_skip_verify?: boolean;
}

/** root */
export interface CfgStroppyRunner1 {
  /** Log level. LOG_LEVEL. debug traces how every parameter was resolved; it also turns on k6 --verbose. */
  log_level?: "debug" | "info" | "warn" | "error";
  /** Log format. K6_LOG_FORMAT of the embedded k6 process. json is what the agent ships to the log store. */
  log_format?: "json" | "text";
  /** OTLP protocol. K6_OTEL_EXPORTER_PROTOCOL. */
  otlp_protocol?: "grpc" | "http/protobuf";
  /** OTLP endpoint. K6_OTEL_GRPC_EXPORTER_ENDPOINT / K6_OTEL_HTTP_EXPORTER_ENDPOINT. Filled by the server: the agent's OTLP receiver on this machine. */
  otlp_endpoint?: string | null;
  /** OTLP HTTP path. K6_OTEL_HTTP_EXPORTER_URL_PATH; only read for the http/protobuf protocol. */
  otlp_http_url_path?: string;
  /** OTLP insecure. K6_OTEL_GRPC_EXPORTER_INSECURE / K6_OTEL_HTTP_EXPORTER_INSECURE. The endpoint is a local agent, so plaintext is the norm. */
  otlp_insecure?: boolean;
  /** OTLP service name. K6_OTEL_SERVICE_NAME. */
  otlp_service_name?: string;
  /** OTLP metric prefix. K6_OTEL_METRIC_PREFIX prepended to every k6 metric name. */
  otlp_metric_prefix?: string;
  /** OTLP headers. K6_OTEL_HEADERS, comma-joined k=v. The server fills the run/tenant identity here (there is no separate label env var upstream). */
  otlp_headers?: Record<string, string>;
  /** Metrics flush interval. K6_OTEL_EXPORT_INTERVAL: how often k6 pushes metric batches to the collector. */
  metrics_flush_interval?: string;
  /** GOMAXPROCS. GOMAXPROCS of the stroppy/k6 process; 0 leaves the Go default (one per CPU) and emits no variable. */
  threads?: number | string;
  /** Pool max connections. driver pool.maxConns. Keep it at or above the k6 VU count or VUs queue on the pool instead of the database. */
  pool_max_conns?: number | string;
  /** Pool min connections. driver pool.minConns: connections kept warm; equal to max removes connect latency from the measurement. */
  pool_min_conns?: number | string;
  /** Connection lifetime. driver pool.maxConnLifetime. */
  pool_conn_lifetime?: string;
  /** Connection idle time. driver pool.maxConnIdleTime. */
  pool_conn_idle_time?: string;
  /** Insert progress interval. driver insertProgress.interval: how often the load phase reports row progress (upstream default 10s). */
  insert_progress_interval?: string;
  /** Stall warning after. driver insertProgress.stallAfter: warn when no rows moved for this long (upstream default 60s). */
  insert_progress_stall_after?: string;
  /** Insert progress mode. driver insertProgress.mode. */
  insert_progress_mode?: "both" | "log" | "metrics" | "off";
  /** Error mode. driver errorMode: what a failing statement does to the run. */
  error_mode?: "silent" | "log" | "throw" | "fail" | "abort";
  /** Bulk size. driver bulkSize: rows per bulk INSERT during load_data (upstream default 2500). */
  bulk_size?: number | string;
  /** Insert method. driver defaultInsertMethod; the fastest method differs per driver (native for pg/ydb, plain_bulk for mysql/picodata). */
  default_insert_method?: "native" | "plain_bulk" | "plain_query";
  /** Driver URL. driver url. Filled by the server from topology (the proxy or primary endpoint of the deployed database). */
  driver_url?: string | null;
  /** Driver. Which stroppy driver the workload talks through; the protocol decides which TLS/auth options exist. */
  driver: CfgStroppyRunner1DriverMysql | CfgStroppyRunner1DriverNoop | CfgStroppyRunner1DriverPicodata | CfgStroppyRunner1DriverPostgres | CfgStroppyRunner1DriverYdb;
  /** Rendered STROPPY_DRIVER_0. Derived: the driver JSON stroppy reads from STROPPY_DRIVER_0. */
  readonly driver_json?: string;
  /** Rendered OTLP endpoint lines. */
  readonly otlp_endpoint_lines?: string;
  /** Rendered K6_OTEL_HEADERS. */
  readonly otlp_headers_line?: string;
  /** Rendered GOMAXPROCS. */
  readonly gomaxprocs_line?: string;
}

