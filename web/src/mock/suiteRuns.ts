// TEMPORARY mock — MUST be deleted before real API wiring; do not build on this.
//
// Stateful suite-runs store keyed by tenant SLUG. Backs
// cloud.v1.api.SuiteRunService (List/Get/Cancel/Delete) for the Suite detail
// page, plus an internal createSuiteRun() the suites mock calls from StartSuite.
//
// Runs that start "running" progress on a wall-clock timer: child runs flip
// pending -> running -> completed, the summary aggregates recompute, and the
// suite run finishes (completed/failed). This makes Run-now genuinely live in
// the standalone preview.

import type { DbKind, DeployProvider } from "@/services/runs";
import {
  type RunStatus,
  type SuiteRunChildVM,
  type SuiteRunsPage,
  type SuiteRunsProvider,
  type SuiteRunsQuery,
  type SuiteRunTrigger,
  type SuiteRunVM,
} from "@/services/suiteRuns";

function nowIso(): string {
  return new Date().toISOString();
}

function isoMinus(min: number): string {
  return new Date(Date.now() - min * 60_000).toISOString();
}

let seq = 0;
function nextId(prefix: string): string {
  seq += 1;
  return `${prefix}-${Date.now().toString(36)}-${seq}`;
}

function recompute(run: SuiteRunVM): void {
  const total = run.children.length;
  let completed = 0;
  let failed = 0;
  let running = 0;
  let pending = 0;
  let cancelling = 0;
  let cancelled = 0;
  for (const c of run.children) {
    switch (c.status) {
      case "completed":
        completed += 1;
        break;
      case "failed":
        failed += 1;
        break;
      case "running":
        running += 1;
        break;
      case "cancelling":
        cancelling += 1;
        break;
      case "cancelled":
        cancelled += 1;
        break;
      default:
        pending += 1;
    }
  }
  run.total = total;
  run.completed = completed;
  run.failed = failed;
  run.running = running + cancelling;
  run.pending = pending;
  const settled = completed + failed + cancelled;
  run.progressPct = total === 0 ? 0 : Math.round((settled / total) * 100);

  const finished = settled === total;
  if (run.status === "cancelling" && running + cancelling + pending === 0) {
    run.status = "cancelled";
  } else if (finished && run.status === "running") {
    run.status = failed > 0 ? "failed" : "completed";
  }
  if (
    (run.status === "completed" ||
      run.status === "failed" ||
      run.status === "cancelled") &&
    !run.finishedAt
  ) {
    run.finishedAt = nowIso();
    if (run.startedAt) {
      run.durationSec = Math.max(
        1,
        Math.round(
          (new Date(run.finishedAt).getTime() -
            new Date(run.startedAt).getTime()) /
            1000,
        ),
      );
    }
  }
}

// --- progress engine --------------------------------------------------------

let timer: ReturnType<typeof setInterval> | null = null;

function tick(): void {
  let anyActive = false;
  for (const list of Object.values(BY_SLUG)) {
    for (const run of list) {
      if (run.status === "running" || run.status === "cancelling") {
        anyActive = true;
        stepRun(run);
      }
    }
  }
  if (!anyActive && timer) {
    clearInterval(timer);
    timer = null;
  }
}

function stepRun(run: SuiteRunVM): void {
  if (run.status === "cancelling") {
    // Cancelling: stop everything not yet settled.
    for (const c of run.children) {
      if (c.status === "running" || c.status === "pending") {
        c.status = "cancelled";
      }
    }
    recompute(run);
    return;
  }
  // Respect max_parallel (0 = unlimited).
  const cap = run.maxParallel > 0 ? run.maxParallel : run.children.length;
  // Settle one running child, then start pending children up to the cap.
  const runningChild = run.children.find((c) => c.status === "running");
  if (runningChild) {
    // ~15% of completions fail, to exercise the failed badges.
    runningChild.status = Math.random() < 0.15 ? "failed" : "completed";
  }
  let active = run.children.filter((c) => c.status === "running").length;
  for (const c of run.children) {
    if (active >= cap) break;
    if (c.status === "pending") {
      c.status = "running";
      active += 1;
    }
  }
  recompute(run);
}

function ensureTimer(): void {
  if (!timer) {
    timer = setInterval(tick, 1500);
  }
}

// --- store ------------------------------------------------------------------

function child(
  cellId: string,
  name: string,
  status: RunStatus,
): SuiteRunChildVM {
  return { suiteCellId: cellId, testRunId: nextId("tr"), name, status };
}

function seedRun(p: {
  id: string;
  suiteId: string;
  suiteName: string;
  provider: DeployProvider;
  dbKinds: DbKind[];
  trigger: SuiteRunTrigger;
  maxParallel: number;
  status: RunStatus;
  startedMinAgo: number;
  finishedMinAgo?: number;
  children: { cell: string; name: string; status: RunStatus }[];
}): SuiteRunVM {
  const run: SuiteRunVM = {
    id: p.id,
    name: p.id,
    suiteId: p.suiteId,
    status: p.status,
    trigger: p.trigger,
    maxParallel: p.maxParallel,
    suiteName: p.suiteName,
    provider: p.provider,
    dbKinds: p.dbKinds,
    total: 0,
    completed: 0,
    failed: 0,
    running: 0,
    pending: 0,
    progressPct: 0,
    startedAt: isoMinus(p.startedMinAgo),
    finishedAt: p.finishedMinAgo !== undefined ? isoMinus(p.finishedMinAgo) : undefined,
    children: p.children.map((c) => child(c.cell, c.name, c.status)),
  };
  if (run.finishedAt && run.startedAt) {
    run.durationSec = Math.max(
      1,
      Math.round(
        (new Date(run.finishedAt).getTime() -
          new Date(run.startedAt).getTime()) /
          1000,
      ),
    );
  }
  recompute(run);
  return run;
}

// ydb-smoke runs (matches suite-ydb-smoke seeded in mock/suites.ts).
const YDB_CELLS = [
  { cell: "suite-ydb-smoke-cell-1", name: "ydb / read-heavy" },
  { cell: "suite-ydb-smoke-cell-2", name: "ydb / write-heavy" },
  { cell: "suite-ydb-smoke-cell-3", name: "ydb_managed / read-heavy" },
  { cell: "suite-ydb-smoke-cell-4", name: "ydb_managed / mixed-oltp" },
];

const ACME_RUNS: SuiteRunVM[] = [
  seedRun({
    id: "sr-ydb-live",
    suiteId: "suite-ydb-smoke",
    suiteName: "ydb-smoke",
    provider: "yandex",
    dbKinds: ["ydb", "ydb_managed"],
    trigger: "cron",
    maxParallel: 1,
    status: "running",
    startedMinAgo: 4,
    children: [
      { ...YDB_CELLS[0], status: "completed" },
      { ...YDB_CELLS[1], status: "running" },
      { ...YDB_CELLS[2], status: "pending" },
      { ...YDB_CELLS[3], status: "pending" },
    ],
  }),
  seedRun({
    id: "sr-ydb-prev",
    suiteId: "suite-ydb-smoke",
    suiteName: "ydb-smoke",
    provider: "yandex",
    dbKinds: ["ydb", "ydb_managed"],
    trigger: "cron",
    maxParallel: 1,
    status: "completed",
    startedMinAgo: 38,
    finishedMinAgo: 34,
    children: [
      { ...YDB_CELLS[0], status: "completed" },
      { ...YDB_CELLS[1], status: "completed" },
      { ...YDB_CELLS[2], status: "completed" },
      { ...YDB_CELLS[3], status: "completed" },
    ],
  }),
  seedRun({
    id: "sr-ydb-fail",
    suiteId: "suite-ydb-smoke",
    suiteName: "ydb-smoke",
    provider: "yandex",
    dbKinds: ["ydb", "ydb_managed"],
    trigger: "manual",
    maxParallel: 1,
    status: "failed",
    startedMinAgo: 95,
    finishedMinAgo: 90,
    children: [
      { ...YDB_CELLS[0], status: "completed" },
      { ...YDB_CELLS[1], status: "failed" },
      { ...YDB_CELLS[2], status: "completed" },
      { ...YDB_CELLS[3], status: "cancelled" },
    ],
  }),
];

const BY_SLUG: Record<string, SuiteRunVM[]> = {
  acme: ACME_RUNS,
};

function rows(slug: string): SuiteRunVM[] {
  let list = BY_SLUG[slug];
  if (!list) {
    list = [];
    BY_SLUG[slug] = list;
  }
  return list;
}

function delay(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 120));
}

/**
 * Internal: called by the suites mock's startSuite. Creates a fresh running
 * suite run from the suite's enabled cells and kicks the progress engine.
 */
export function createSuiteRun(
  slug: string,
  p: {
    suiteId: string;
    suiteName: string;
    provider: DeployProvider;
    dbKinds: DbKind[];
    maxParallel: number;
    cells: { id: string; name: string }[];
  },
): SuiteRunVM {
  const id = nextId("sr");
  const run = seedRun({
    id,
    suiteId: p.suiteId,
    suiteName: p.suiteName,
    provider: p.provider,
    dbKinds: p.dbKinds,
    trigger: "manual",
    maxParallel: p.maxParallel,
    status: "running",
    startedMinAgo: 0,
    children: p.cells.map((c) => ({
      cell: c.id,
      name: c.name,
      status: "pending" as RunStatus,
    })),
  });
  // Kick the first child(ren) immediately so the run shows progress.
  stepRun(run);
  rows(slug).unshift(run);
  ensureTimer();
  return run;
}

// Start the progress engine if any seeded run is already live.
ensureTimer();

const DEFAULT_SIZE = 25;

export const mockSuiteRunsProvider: SuiteRunsProvider = {
  async listSuiteRuns(
    slug: string,
    query: SuiteRunsQuery,
  ): Promise<SuiteRunsPage> {
    await delay();
    const all = rows(slug)
      .filter((r) => r.suiteId === query.suiteId)
      .sort((a, b) => (b.startedAt ?? "").localeCompare(a.startedAt ?? ""));
    const size = query.pageSize && query.pageSize > 0 ? query.pageSize : DEFAULT_SIZE;
    const offset = query.pageToken ? Number.parseInt(query.pageToken, 10) || 0 : 0;
    const slice = all.slice(offset, offset + size);
    const nextOffset = offset + size;
    return {
      runs: slice.map((r) => ({ ...r, children: [...r.children] })),
      nextPageToken: nextOffset < all.length ? String(nextOffset) : "",
    };
  },

  async getSuiteRun(slug: string, id: string): Promise<SuiteRunVM | null> {
    await delay();
    const r = rows(slug).find((x) => x.id === id);
    return r ? { ...r, children: [...r.children] } : null;
  },

  async cancelSuiteRun(slug: string, id: string): Promise<void> {
    await delay();
    const r = rows(slug).find((x) => x.id === id);
    if (!r) return;
    if (r.status === "running" || r.status === "pending") {
      r.status = "cancelling";
      ensureTimer();
    }
  },

  async deleteSuiteRun(slug: string, id: string): Promise<void> {
    await delay();
    const list = rows(slug);
    const i = list.findIndex((x) => x.id === id);
    if (i >= 0) list.splice(i, 1);
  },
};
