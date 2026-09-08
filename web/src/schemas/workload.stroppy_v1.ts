// GENERATED from schemapb schema workload.stroppy@1 — do not edit.
// A stroppy workload: version, protocol, load segments and driver options.

/** variant duration of limit */
export interface WorkloadStroppy1SegmentExecutionLimitDuration {
  kind: "duration";
  /** Duration. Wall-clock length of the segment (k6 --duration). */
  duration: string;
}

/** variant iterations of limit */
export interface WorkloadStroppy1SegmentExecutionLimitIterations {
  kind: "iterations";
  /** Iterations. Total iterations across all VUs (k6 --iterations). */
  count: number | string;
}

/** object execution */
export interface WorkloadStroppy1SegmentExecution {
  /** Virtual users. k6 virtual users driving the segment; sizes the runner machine. */
  vus?: number | string;
  /** Limit. How the segment ends: after a wall-clock duration or after N iterations. */
  limit: WorkloadStroppy1SegmentExecutionLimitDuration | WorkloadStroppy1SegmentExecutionLimitIterations;
  /** Quiet. Suppress the k6 progress bar (k6 --quiet). */
  quiet?: boolean;
  /** Ignore thresholds. Do not fail the segment on threshold breach (k6 --no-thresholds). */
  no_thresholds?: boolean;
  /** Extra k6 args. Raw arguments appended after `--` to the embedded k6 process. */
  extra_args?: Array<string>;
}

/** object params */
export interface WorkloadStroppy1SegmentParams {
  /** Pool size. Driver connection pool size (POOL_SIZE / pool.maxConns). */
  pool_size?: number | string;
  /** Scale factor. Dataset scale (warehouses for TPC-C, SF for TPC-H); drives the disk requirement. */
  scale_factor?: number | string;
  /** Only these steps. Run only the listed stroppy steps (`--steps`); empty = all steps. */
  steps?: Array<string>;
  /** Skip these steps. Skip the listed stroppy steps (`--no-steps`); mutually exclusive with steps. */
  no_steps?: Array<string>;
  /** Insert method. Driver-level write protocol for generated rows (defaultInsertMethod). */
  insert_method?: "native" | "plain_bulk" | "plain_query";
  /** Bulk size. Rows per bulk INSERT statement; only used by plain_bulk. [rows] */
  bulk_size?: number | string;
  /** Environment. Extra script environment (`-e KEY=VALUE`); keys must match ^[A-Z_][A-Z0-9_]*$. */
  env?: Record<string, string>;
}

/** object  */
export interface WorkloadStroppy1SegmentItem {
  /** File name. Name the file gets next to the script on the runner. */
  name: string;
  /** Kind. What the file is: a schema/DDL file, a config, or a data file. */
  kind?: "sql" | "conf" | "data";
  /** Content. Inline file body, at most 1 MiB. */
  content?: string;
  /** Reference. Artifact/object reference to fetch instead of inline content. */
  ref?: string;
}

/** object thresholds */
export interface WorkloadStroppy1SegmentThresholds {
  /** p99 latency. Fail the segment when the p99 request latency exceeds this. [ms] */
  p99_ms?: number;
  /** Error rate. Fail the segment when the error ratio exceeds this (0..1). [ratio] */
  error_rate?: number;
}

/** def segment */
export interface WorkloadStroppy1Segment {
  /** Name. Segment id inside the workload; used as the phase label of the run and as a metric label. */
  name: string;
  /** Script. Stroppy workload script id or path, e.g. tpcc/tx. Restricted to the catalog by the server. */
  script: string;
  /** Execution. Load shape of the segment. */
  execution: WorkloadStroppy1SegmentExecution;
  /** Parameters. Data-generation and driver parameters passed to the script. */
  params?: WorkloadStroppy1SegmentParams;
  /** Files. Extra files (SQL schema, configs, data) shipped with the segment. */
  files?: Array<WorkloadStroppy1SegmentItem>;
  /** Thresholds. Pass/fail thresholds handed to k6; ignored when no_thresholds is set. */
  thresholds?: WorkloadStroppy1SegmentThresholds;
  /** Warm-up. Idle wait before the segment starts, letting caches and replicas settle. */
  warmup?: string;
}

/** variant cockroach of driver_options */
export interface WorkloadStroppy1DriverOptionsCockroach {
  kind: "cockroach";
  /** SSL mode. TLS negotiation mode of the cockroach (pgwire) connection. */
  sslmode?: "disable" | "require" | "verify-full";
}

/** variant mysql of driver_options */
export interface WorkloadStroppy1DriverOptionsMysql {
  kind: "mysql";
  /** TLS. go-sql-driver TLS mode of the MySQL connection. */
  tls?: "false" | "preferred" | "skip-verify" | "true";
  /** Charset. Connection character set. */
  charset?: string;
}

/** variant noop of driver_options */
export interface WorkloadStroppy1DriverOptionsNoop {
  kind: "noop";
}

/** variant pg of driver_options */
export interface WorkloadStroppy1DriverOptionsPg {
  kind: "pg";
  /** SSL mode. TLS negotiation mode of the postgres connection. */
  sslmode?: "disable" | "require" | "verify-full";
  /** Application name. application_name reported to postgres; shows up in pg_stat_activity. */
  application_name?: string;
  /** Statement cache. pgx query execution mode: how prepared statements are cached. */
  statement_cache?: "cache_statement" | "cache_describe" | "describe_exec" | "exec" | "simple_protocol";
}

/** variant picodata of driver_options */
export interface WorkloadStroppy1DriverOptionsPicodata {
  kind: "picodata";
}

/** variant ydb of driver_options */
export interface WorkloadStroppy1DriverOptionsYdb {
  kind: "ydb";
  /** TLS (grpcs). Use the grpcs:// scheme; must agree with the protocol. */
  grpcs?: boolean;
  /** CA certificate. PEM of a private CA, when the endpoint is not signed by a public one. */
  ca_cert?: string;
  /** Auth token. IAM token passed as authToken. */
  token?: string;
}

/** root */
export interface WorkloadStroppy1 {
  /** Stroppy version. Stroppy release to run; must exist in the platform stroppy catalog. */
  stroppy_version: string;
  /** Protocol. Wire protocol stroppy talks to the database with. */
  protocol: "pg" | "mysql" | "picodata" | "ydb_grpc" | "ydb_grpcs" | "cockroach" | "noop";
  /** Segments. Ordered stroppy invocations; each one is a phase of the run. */
  segments: Array<WorkloadStroppy1Segment>;
  /** Driver options. Protocol-specific connection options; the kind must match the protocol. */
  driver_options?: WorkloadStroppy1DriverOptionsCockroach | WorkloadStroppy1DriverOptionsMysql | WorkloadStroppy1DriverOptionsNoop | WorkloadStroppy1DriverOptionsPg | WorkloadStroppy1DriverOptionsPicodata | WorkloadStroppy1DriverOptionsYdb;
}

