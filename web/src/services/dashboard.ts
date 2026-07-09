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

/** A recent test run, flattened from Run for the activity list. */
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
 * real start timestamp (Run.summary.started_at, falling back to
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
 *   recentRuns    <- recent_runs (Run[])
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

import { toJson } from "@bufbuild/protobuf";
import { RunSchema, type Run } from "@/lib/proto/cloud/v1/models/test_run_pb";
import type {
  StatusCounts,
  UpcomingSuite,
} from "@/lib/proto/cloud/v1/api/tenant_dashboard_pb";
import type { RatingEntry } from "@/lib/proto/cloud/v1/api/rating_pb";
import { tenantDashboardClient } from "@/services/client";
import { resolveTenantId } from "@/services/tenant";

/** Map the wire Status enum (JSON string form) onto the flat RunStatus union. */
export function statusToVM(s: string | undefined): RunStatus {
  switch (s) {
    case "STATUS_PENDING":
    case "STATUS_UNSPECIFIED":
      return "pending";
    case "STATUS_RUNNING":
    case "STATUS_ALLOCATED":
    case "STATUS_DEPLOYMENT":
    case "STATUS_RETRY_WAIT":
      return "running";
    case "STATUS_CANCELLING":
      return "cancelling";
    case "STATUS_COMPLETED":
    case "STATUS_DEPLOYED":
      return "completed";
    case "STATUS_FAILED":
      return "failed";
    case "STATUS_CANCELLED":
    case "STATUS_SKIPPED":
      return "cancelled";
    default:
      return "pending";
  }
}

/** Flatten a Run onto the dashboard's RecentRunVM. */
export function recentRunFromRecord(rec: Run): RecentRunVM {
  const j = toJson(RunSchema, rec) as {
    entity?: { id?: string; name?: string; timings?: { createdAt?: string } };
    status?: string;
    summary?: { dbKind?: string; workloadName?: string; startedAt?: string };
  };
  return {
    id: j.entity?.id ?? "",
    name: j.entity?.name ?? "",
    status: statusToVM(j.status),
    startedAt: j.summary?.startedAt ?? j.entity?.timings?.createdAt ?? "",
    dbKind: j.summary?.dbKind ?? "",
    workload: j.summary?.workloadName ?? "",
  };
}

function countsFromProto(c: StatusCounts | undefined): StatusCountsVM {
  return {
    total: c?.total ?? 0,
    pending: c?.pending ?? 0,
    running: c?.running ?? 0,
    completed: c?.completed ?? 0,
    failed: c?.failed ?? 0,
    cancelled: c?.cancelled ?? 0,
  };
}

function upcomingFromProto(u: UpcomingSuite): UpcomingSuiteVM {
  return {
    suiteId: u.suiteId,
    name: u.name,
    cron: u.cron,
    nextRunAt: u.nextRunAt
      ? new Date(Number(u.nextRunAt.seconds) * 1000).toISOString()
      : "",
  };
}

function benchmarkFromProto(r: RatingEntry): BenchmarkVM {
  return {
    rank: r.rank,
    metricValue: r.metricValue,
    metricUnit: r.metricUnit,
    dbKind: String(r.dbKind),
    workload: r.workloadName,
    topology: r.topologyLabel,
    author: r.authorName,
  };
}

const realDashboardProvider: DashboardProvider = {
  async getDashboard(tenantSlug) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { dashboard } = await tenantDashboardClient.getTenantDashboard({
      tenantId,
    });
    const recentRuns = (dashboard?.recentRuns ?? []).map(recentRunFromRecord);
    return {
      runCounts: countsFromProto(dashboard?.runCounts),
      successRate: dashboard?.successRate ?? 0,
      recentRuns,
      upcoming: (dashboard?.upcoming ?? []).map(upcomingFromProto),
      topBenchmarks: (dashboard?.topBenchmarks ?? []).map(benchmarkFromProto),
      runsOverTime: bucketRunsByDay(recentRuns),
    };
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
