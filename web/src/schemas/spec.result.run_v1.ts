// GENERATED from schemapb schema spec.result.run@1 — do not edit.
// RunResult: headline metrics, per-segment outcome, artifacts and summary of one run.

/** map value metrics */
export interface SpecResultRun1MetricsValue {
  /** Value. Headline value of the metric over the window. */
  value: number;
  /** Unit. Unit of the value: tps, ms, ratio… */
  unit?: string;
  /** Minimum. Lowest sample in the window. */
  min?: number;
  /** Maximum. Highest sample in the window. */
  max?: number;
  /** Average. Mean over the window. */
  avg?: number;
}

/** map value metrics */
export interface SpecResultRun1ItemMetricsValue {
  /** Value. Headline value of the metric over the window. */
  value: number;
  /** Unit. Unit of the value: tps, ms, ratio… */
  unit?: string;
  /** Minimum. Lowest sample in the window. */
  min?: number;
  /** Maximum. Highest sample in the window. */
  max?: number;
  /** Average. Mean over the window. */
  avg?: number;
}

/** object  */
export interface SpecResultRun1Item {
  /** Segment. Name of the workload segment. */
  name: string;
  /** Status. How the segment ended. */
  status: "completed" | "failed" | "skipped" | "canceled";
  /** Started. */
  started_at?: string;
  /** Finished. */
  finished_at?: string;
  /** Metrics. Metrics measured over this segment's window only. */
  metrics?: Record<string, SpecResultRun1ItemMetricsValue>;
  /** Error. Failure text; set when the status is failed. */
  error?: string;
}

/** object summary */
export interface SpecResultRun1Summary {
  /** Throughput. Transactions per second over the measured segments. [tps] */
  tps?: number;
  /** Latency p50. [ms] */
  latency_p50_ms?: number;
  /** Latency p95. [ms] */
  latency_p95_ms?: number;
  /** Latency p99. [ms] */
  latency_p99_ms?: number;
  /** Errors. Failed requests across the run. */
  errors?: number | string;
  /** Duration. Wall-clock length of the measured part of the run. */
  duration?: string;
}

/** root */
export interface SpecResultRun1 {
  /** Metrics. Canonical run metrics by key (tps, latency_p99_ms, errors…). */
  metrics?: Record<string, SpecResultRun1MetricsValue>;
  /** Segments. One entry per workload segment, in execution order. */
  segments?: Array<SpecResultRun1Item>;
  /** Artifacts. Graphene artifact references (artifact/<id>): raw stroppy output, the report. */
  artifacts?: Array<string>;
  /** Summary. The headline numbers the run list, rating and compare sort on. */
  summary?: SpecResultRun1Summary;
}

