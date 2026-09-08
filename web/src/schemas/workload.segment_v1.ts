// GENERATED from schemapb schema workload.segment@1 — do not edit.
// One stroppy load segment: script, execution limits, generator parameters and files.

/** variant duration of limit */
export interface WorkloadSegment1ExecutionLimitDuration {
  kind: "duration";
  /** Duration. Wall-clock length of the segment (k6 --duration). */
  duration: string;
}

/** variant iterations of limit */
export interface WorkloadSegment1ExecutionLimitIterations {
  kind: "iterations";
  /** Iterations. Total iterations across all VUs (k6 --iterations). */
  count: number | string;
}

/** object execution */
export interface WorkloadSegment1Execution {
  /** Virtual users. k6 virtual users driving the segment; sizes the runner machine. */
  vus?: number | string;
  /** Limit. How the segment ends: after a wall-clock duration or after N iterations. */
  limit: WorkloadSegment1ExecutionLimitDuration | WorkloadSegment1ExecutionLimitIterations;
  /** Quiet. Suppress the k6 progress bar (k6 --quiet). */
  quiet?: boolean;
  /** Ignore thresholds. Do not fail the segment on threshold breach (k6 --no-thresholds). */
  no_thresholds?: boolean;
  /** Extra k6 args. Raw arguments appended after `--` to the embedded k6 process. */
  extra_args?: Array<string>;
}

/** object params */
export interface WorkloadSegment1Params {
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
export interface WorkloadSegment1Item {
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
export interface WorkloadSegment1Thresholds {
  /** p99 latency. Fail the segment when the p99 request latency exceeds this. [ms] */
  p99_ms?: number;
  /** Error rate. Fail the segment when the error ratio exceeds this (0..1). [ratio] */
  error_rate?: number;
}

/** root */
export interface WorkloadSegment1 {
  /** Name. Segment id inside the workload; used as the phase label of the run and as a metric label. */
  name: string;
  /** Script. Stroppy workload script id or path, e.g. tpcc/tx. Restricted to the catalog by the server. */
  script: string;
  /** Execution. Load shape of the segment. */
  execution: WorkloadSegment1Execution;
  /** Parameters. Data-generation and driver parameters passed to the script. */
  params?: WorkloadSegment1Params;
  /** Files. Extra files (SQL schema, configs, data) shipped with the segment. */
  files?: Array<WorkloadSegment1Item>;
  /** Thresholds. Pass/fail thresholds handed to k6; ignored when no_thresholds is set. */
  thresholds?: WorkloadSegment1Thresholds;
  /** Warm-up. Idle wait before the segment starts, letting caches and replicas settle. */
  warmup?: string;
}

