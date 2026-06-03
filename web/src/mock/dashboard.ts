// TEMPORARY mock — MUST be deleted before real API wiring; do not build on this.
//
// Canned tenant-dashboard payloads so the Organization Dashboard renders
// without a backend. Keyed by tenant SLUG so switching orgs shows different
// data. Every field maps 1:1 to cloud.v1.api.TenantDashboard (run_counts,
// success_rate, recent_runs, upcoming, top_benchmarks). The runs-over-time
// series is NOT canned: it is derived from recentRuns by bucketRunsByDay, the
// same shared derivation the real provider mapper will use.

import {
  bucketRunsByDay,
  type DashboardProvider,
  type DashboardVM,
  type RecentRunVM,
  type UpcomingSuiteVM,
  type BenchmarkVM,
  type StatusCountsVM,
} from "@/services/dashboard";

function hoursAgo(h: number): string {
  return new Date(Date.now() - h * 3600_000).toISOString();
}
function hoursAhead(h: number): string {
  return new Date(Date.now() + h * 3600_000).toISOString();
}

/**
 * Assemble a DashboardVM from the real proto-backed fields, deriving the
 * runs-over-time series from recentRuns (mirrors the real mapper).
 */
function makeVM(parts: {
  runCounts: StatusCountsVM;
  successRate: number;
  recentRuns: RecentRunVM[];
  upcoming: UpcomingSuiteVM[];
  topBenchmarks: BenchmarkVM[];
}): DashboardVM {
  return {
    ...parts,
    runsOverTime: bucketRunsByDay(parts.recentRuns),
  };
}

// recentRuns carry real ISO start timestamps spread across several days so the
// derived runs-over-time series has multiple buckets.
const ACME: DashboardVM = makeVM({
  runCounts: {
    total: 248,
    pending: 3,
    running: 2,
    completed: 221,
    failed: 18,
    cancelled: 4,
  },
  successRate: 0.924,
  recentRuns: [
    { id: "r-1042", name: "tpcc-scale-32", status: "running", startedAt: hoursAgo(0.3), dbKind: "postgres", workload: "tpcc" },
    { id: "r-1041", name: "ycsb-a-baseline", status: "completed", startedAt: hoursAgo(2), dbKind: "postgres", workload: "ycsb" },
    { id: "r-1040", name: "tpcc-scale-16", status: "completed", startedAt: hoursAgo(20), dbKind: "postgres", workload: "tpcc" },
    { id: "r-1039", name: "pgbench-rw-mix", status: "failed", startedAt: hoursAgo(28), dbKind: "postgres", workload: "pgbench" },
    { id: "r-1038", name: "ycsb-c-readonly", status: "completed", startedAt: hoursAgo(34), dbKind: "postgres", workload: "ycsb" },
    { id: "r-1037", name: "tpcc-scale-8", status: "cancelled", startedAt: hoursAgo(50), dbKind: "postgres", workload: "tpcc" },
    { id: "r-1036", name: "ycsb-b-mixed", status: "completed", startedAt: hoursAgo(58), dbKind: "postgres", workload: "ycsb" },
    { id: "r-1035", name: "tpcc-scale-4", status: "completed", startedAt: hoursAgo(76), dbKind: "postgres", workload: "tpcc" },
  ],
  upcoming: [
    { suiteId: "s-nightly", name: "Nightly regression", cron: "0 2 * * *", nextRunAt: hoursAhead(7) },
    { suiteId: "s-weekly", name: "Weekly scale sweep", cron: "0 3 * * 1", nextRunAt: hoursAhead(54) },
  ],
  topBenchmarks: [
    { rank: 1, metricValue: 184_320, metricUnit: "tpmC", dbKind: "postgres", workload: "tpcc", topology: "3-node HA", author: "Ada Lovelace" },
    { rank: 2, metricValue: 162_004, metricUnit: "tpmC", dbKind: "postgres", workload: "tpcc", topology: "single", author: "Grace Hopper" },
    { rank: 3, metricValue: 98_500, metricUnit: "ops/s", dbKind: "postgres", workload: "ycsb", topology: "3-node HA", author: "Ada Lovelace" },
  ],
});

const GLOBEX: DashboardVM = makeVM({
  runCounts: {
    total: 57,
    pending: 0,
    running: 1,
    completed: 49,
    failed: 6,
    cancelled: 1,
  },
  successRate: 0.891,
  recentRuns: [
    { id: "g-310", name: "ycsb-b-readheavy", status: "running", startedAt: hoursAgo(0.5), dbKind: "postgres", workload: "ycsb" },
    { id: "g-309", name: "tpcc-smoke", status: "completed", startedAt: hoursAgo(6), dbKind: "postgres", workload: "tpcc" },
    { id: "g-308", name: "pgbench-ro", status: "failed", startedAt: hoursAgo(26), dbKind: "postgres", workload: "pgbench" },
    { id: "g-307", name: "ycsb-a", status: "completed", startedAt: hoursAgo(30), dbKind: "postgres", workload: "ycsb" },
    { id: "g-306", name: "tpcc-baseline", status: "completed", startedAt: hoursAgo(52), dbKind: "postgres", workload: "tpcc" },
  ],
  upcoming: [
    { suiteId: "g-nightly", name: "Smoke suite", cron: "30 1 * * *", nextRunAt: hoursAhead(11) },
  ],
  topBenchmarks: [
    { rank: 1, metricValue: 71_200, metricUnit: "ops/s", dbKind: "postgres", workload: "ycsb", topology: "single", author: "Globex CI" },
    { rank: 2, metricValue: 64_900, metricUnit: "tpmC", dbKind: "postgres", workload: "tpcc", topology: "single", author: "Globex CI" },
  ],
});

const EMPTY: DashboardVM = makeVM({
  runCounts: { total: 0, pending: 0, running: 0, completed: 0, failed: 0, cancelled: 0 },
  successRate: 0,
  recentRuns: [],
  upcoming: [],
  topBenchmarks: [],
});

const BY_SLUG: Record<string, DashboardVM> = {
  acme: ACME,
  globex: GLOBEX,
  // initech intentionally falls through to EMPTY to show the empty state.
};

export const mockDashboardProvider: DashboardProvider = {
  async getDashboard(tenantSlug: string): Promise<DashboardVM> {
    // Simulate a little network latency so loading states are visible.
    await new Promise((r) => setTimeout(r, 150));
    return BY_SLUG[tenantSlug] ?? EMPTY;
  },
};
