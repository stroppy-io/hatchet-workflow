// Tenant-dashboard data surface for the rebuilt shell.
//
// This is the contract the Organization Dashboard page talks to. It mirrors the
// auth.ts provider pattern: the page depends ONLY on this interface, never on
// mock or backend specifics. The real implementation (TODO below) will call the
// connectrpc `tenantDashboardClient.getTenantDashboard` (see
// src/lib/proto/cloud/v1/api/tenant_dashboard_pb.ts) and map the proto payload
// onto the view-model types here. For now the single mock gate in main.tsx can
// inject canned data via setDashboardProvider() so the page renders standalone.
//
// The view-model is intentionally flat (no proto Message instances) so the
// dashboard UI stays decoupled from the wire format and the mock stays trivial.

/** Lifecycle status of a run, mapped from cloud.v1.common.Status. */
export type RunStatus =
  | "pending"
  | "running"
  | "cancelling"
  | "completed"
  | "failed"
  | "cancelled";

/** Headline run breakdown by status (the GetTenantDashboard run_counts tile). */
export interface StatusCountsVM {
  total: number;
  pending: number;
  running: number;
  completed: number;
  failed: number;
  cancelled: number;
}

/** A recent test run, flattened from TestRunRecord for the activity list. */
export interface RecentRunVM {
  id: string;
  name: string;
  status: RunStatus;
  /** ISO timestamp the run started/was created. */
  startedAt: string;
  dbKind: string;
  workload: string;
}

/** A scheduled suite + its next planned auto-run (UpcomingSuite). */
export interface UpcomingSuiteVM {
  suiteId: string;
  name: string;
  cron: string;
  /** ISO timestamp of the next planned run. */
  nextRunAt: string;
}

/** A top-benchmark rating entry (RatingEntry), flattened for the leaderboard. */
export interface BenchmarkVM {
  rank: number;
  metricValue: number;
  metricUnit: string;
  dbKind: string;
  workload: string;
  topology: string;
  author: string;
}

/**
 * One day-bucket of recent-run activity for the runs-over-time chart.
 *
 * DERIVED — there is no time-series field in the tenant_dashboard proto. This
 * is computed from `TenantDashboard.recent_runs` by bucketing each run on its
 * real start timestamp (TestRunRecord.summary.started_at, falling back to
 * entity.timings.created_at) and splitting by status (COMPLETED -> succeeded,
 * FAILED/CANCELLED -> failed). `succeeded` + `failed` == `total` per bucket.
 * See `bucketRunsByDay` for the single shared derivation.
 */
export interface RunsTimePoint {
  /** ISO date (YYYY-MM-DD) of the bucket. */
  date: string;
  total: number;
  succeeded: number;
  failed: number;
}

/**
 * The whole landing payload, mapped 1:1 from cloud.v1.api.TenantDashboard:
 *   runCounts     <- run_counts (StatusCounts)
 *   successRate   <- success_rate (float, 0..1)
 *   recentRuns    <- recent_runs (TestRunRecord[])
 *   upcoming      <- upcoming (UpcomingSuite[])
 *   topBenchmarks <- top_benchmarks (RatingEntry[])
 *
 * `runsOverTime` is the ONE derived (non-proto) field: see bucketRunsByDay,
 * computed from recentRuns. It is omitted when recentRuns is empty so the chart
 * hides cleanly.
 */
export interface DashboardVM {
  runCounts: StatusCountsVM;
  /** completed / finished over a recent window, 0..1. */
  successRate: number;
  recentRuns: RecentRunVM[];
  upcoming: UpcomingSuiteVM[];
  topBenchmarks: BenchmarkVM[];

  /** DERIVED from recentRuns (not a proto field); omitted when no recent runs. */
  runsOverTime?: RunsTimePoint[];
}

/**
 * bucketRunsByDay derives the runs-over-time series from recent runs.
 *
 * This is the single source of truth for the derivation (used by both the real
 * provider mapper and the mock). It groups runs by the calendar day of their
 * real start timestamp and counts succeeded ("completed") vs failed ("failed"
 * or "cancelled") per day. Runs still pending/running are counted in `total`
 * only. Returns chronologically-sorted buckets, or undefined when empty.
 */
export function bucketRunsByDay(
  runs: RecentRunVM[],
): RunsTimePoint[] | undefined {
  if (runs.length === 0) return undefined;
  const byDay = new Map<string, RunsTimePoint>();
  for (const r of runs) {
    const date = r.startedAt.slice(0, 10); // ISO -> YYYY-MM-DD
    let bucket = byDay.get(date);
    if (!bucket) {
      bucket = { date, total: 0, succeeded: 0, failed: 0 };
      byDay.set(date, bucket);
    }
    bucket.total += 1;
    if (r.status === "completed") bucket.succeeded += 1;
    else if (r.status === "failed" || r.status === "cancelled")
      bucket.failed += 1;
  }
  const points = [...byDay.values()].sort((a, b) =>
    a.date < b.date ? -1 : a.date > b.date ? 1 : 0,
  );
  return points.length > 0 ? points : undefined;
}

/**
 * DashboardProvider abstracts the tenant-dashboard fetch so the page never
 * imports mock or backend specifics directly.
 */
export interface DashboardProvider {
  /** Fetch the aggregated dashboard for a tenant (by slug). */
  getDashboard(tenantSlug: string): Promise<DashboardVM>;
}

/**
 * Real backend provider. TODO(real-api): build the connect transport + call
 * tenantDashboardClient.getTenantDashboard({ tenantId }) and map the proto
 * TenantDashboard onto DashboardVM. Throws until wired so a missing-backend
 * misconfig is loud, not silent.
 */
const realDashboardProvider: DashboardProvider = {
  async getDashboard() {
    throw new Error(
      "real DashboardProvider not wired yet — run with VITE_MOCK=1 to preview the dashboard",
    );
  },
};

// --- Provider injection -----------------------------------------------------
//
// Defaults to the real backend. The mock (and ONLY the mock) overrides it via
// setDashboardProvider() from the single gate in main.tsx. To remove the mock:
// delete src/mock/ and the gated call in main.tsx.

let active: DashboardProvider = realDashboardProvider;

export function setDashboardProvider(provider: DashboardProvider): void {
  active = provider;
}

export function getDashboardProvider(): DashboardProvider {
  return active;
}
