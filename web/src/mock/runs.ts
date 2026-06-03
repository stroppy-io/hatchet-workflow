// TEMPORARY mock — MUST be deleted before real API wiring; do not build on this.
//
// Canned test-run lists so the Test Runs page renders without a backend. Keyed
// by tenant SLUG so switching orgs shows different data. Every RunVM field maps
// 1:1 to cloud.v1.models.TestRunRecord (entity + status + summary).
//
// The mock honours the slice of cloud.v1.api.ListTestRunsRequest the page wires
// — filter.search, statuses[], db_kinds[], sort{entity|kind,desc}, page{size,
// token} — by applying those facets in-memory and emitting an opaque numeric
// page token (just the next offset, base64-free for simplicity). This makes the
// filters + paging genuinely exercisable in the standalone preview.

import {
  type DbKind,
  type DeployProvider,
  type Protocol,
  type RunStatus,
  type RunFacets,
  type RunsPage,
  type RunsProvider,
  type RunsQuery,
  type RunTrigger,
  type RunVM,
} from "@/services/runs";

// Default protocol per db kind so the mock rows stay coherent without listing
// it on every row (overridable via the factory's `protocol`).
const PROTOCOL_FOR_DB: Record<Exclude<DbKind, "">, Protocol> = {
  postgres: "pg",
  cockroach: "cockroach",
  mysql: "mysql",
  mariadb: "mysql",
  picodata: "picodata",
  ydb: "ydb_grpc",
  ydb_managed: "ydb_grpcs",
  external: "",
};

// Default deployment provider per db kind so rows carry a coherent provider
// without listing it everywhere (managed/external lean cloud; rest are local).
const PROVIDER_FOR_DB: Record<Exclude<DbKind, "">, DeployProvider> = {
  postgres: "docker",
  cockroach: "docker",
  mysql: "docker",
  mariadb: "docker",
  picodata: "docker",
  ydb: "yandex",
  ydb_managed: "yandex",
  external: "",
};

function hoursAgo(h: number): string {
  return new Date(Date.now() - h * 3600_000).toISOString();
}

// A compact factory so the canned rows stay readable.
function run(p: {
  id: string;
  name: string;
  author: string;
  status: RunStatus;
  db: DbKind;
  workload: string;
  stroppy: string;
  topology: string;
  nodes: number;
  progress: number;
  startedHoursAgo: number;
  durationSec?: number;
  suiteRunId?: string;
  protocol?: Protocol;
  trigger?: RunTrigger;
  provider?: DeployProvider;
  dbPreset?: string;
  workloadPreset?: string;
  testPreset?: string;
  favorite?: boolean;
  deleted?: boolean;
}): RunVM {
  const startedAt = hoursAgo(p.startedHoursAgo);
  const finishedAt =
    p.durationSec !== undefined &&
    (p.status === "completed" ||
      p.status === "failed" ||
      p.status === "cancelled")
      ? new Date(
          new Date(startedAt).getTime() + p.durationSec * 1000,
        ).toISOString()
      : undefined;
  return {
    id: p.id,
    name: p.name,
    authorId: p.author,
    // created slightly before started so the created_at sort differs subtly.
    createdAt: hoursAgo(p.startedHoursAgo + 0.05),
    status: p.status,
    dbKind: p.db,
    workload: p.workload,
    stroppyVersion: p.stroppy,
    protocol: p.protocol ?? (p.db ? PROTOCOL_FOR_DB[p.db] : ""),
    trigger: p.trigger ?? "manual",
    topologyLabel: p.topology,
    nodeCount: p.nodes,
    progressPct: p.progress,
    startedAt: p.status === "pending" ? undefined : startedAt,
    finishedAt,
    durationSec: p.durationSec,
    suiteRunId: p.suiteRunId ?? "",
    provider: p.provider ?? (p.db ? PROVIDER_FOR_DB[p.db] : ""),
    // Derive coherent, repeatable preset ids from db / workload so the preset
    // facets have a small distinct set to pick from in the preview.
    dbPresetId: p.dbPreset ?? (p.db ? `dbp-${p.db}` : ""),
    workloadPresetId: p.workloadPreset ?? (p.workload ? `wlp-${p.workload}` : ""),
    testPresetId: p.testPreset ?? "",
    favorite: p.favorite ?? false,
    deleted: p.deleted ?? false,
  };
}

// Varied status / db / workload / timestamps so every wired facet, sort and the
// pagination are demonstrable.
const ACME: RunVM[] = [
  run({ id: "r-1042", name: "tpcc-scale-32", author: "ada", status: "running", db: "postgres", workload: "tpcc", stroppy: "5.1.2", topology: "PG HA x3", nodes: 3, progress: 64, startedHoursAgo: 0.3, trigger: "manual", favorite: true, testPreset: "tp-nightly-pg" }),
  run({ id: "r-1043", name: "tpcc-cancel-request", author: "grace", status: "cancelling", db: "postgres", workload: "tpcc", stroppy: "5.1.2", topology: "PG HA x3", nodes: 3, progress: 73, startedHoursAgo: 0.15, trigger: "api" }),
  run({ id: "r-1041", name: "ycsb-a-baseline", author: "ada", status: "completed", db: "postgres", workload: "ycsb", stroppy: "5.1.2", topology: "PG single", nodes: 1, progress: 100, startedHoursAgo: 2, durationSec: 1840, trigger: "api", favorite: true }),
  run({ id: "r-1040", name: "tpcc-scale-16", author: "grace", status: "completed", db: "postgres", workload: "tpcc", stroppy: "5.1.1", topology: "PG HA x3", nodes: 3, progress: 100, startedHoursAgo: 20, durationSec: 5260, trigger: "cron", suiteRunId: "suite-nightly" }),
  run({ id: "r-1039", name: "pgbench-rw-mix", author: "grace", status: "failed", db: "postgres", workload: "pgbench", stroppy: "5.1.1", topology: "PG single", nodes: 1, progress: 42, startedHoursAgo: 28, durationSec: 612, trigger: "manual" }),
  run({ id: "r-1038", name: "ydb-ycsb-c", author: "ada", status: "completed", db: "ydb", workload: "ycsb", stroppy: "5.1.2", topology: "YDB x3", nodes: 3, progress: 100, startedHoursAgo: 34, durationSec: 2400, trigger: "manual" }),
  run({ id: "r-1037", name: "cockroach-tpcc", author: "grace", status: "cancelled", db: "cockroach", workload: "tpcc", stroppy: "5.1.0", topology: "CRDB x3", nodes: 3, progress: 18, startedHoursAgo: 50, durationSec: 305, trigger: "api" }),
  run({ id: "r-1036", name: "mysql-ycsb-b", author: "max", status: "completed", db: "mysql", workload: "ycsb", stroppy: "5.1.1", topology: "MySQL single", nodes: 1, progress: 100, startedHoursAgo: 58, durationSec: 3120, trigger: "manual" }),
  run({ id: "r-1035", name: "tpcc-scale-8", author: "ada", status: "completed", db: "postgres", workload: "tpcc", stroppy: "5.1.0", topology: "PG HA x3", nodes: 3, progress: 100, startedHoursAgo: 76, durationSec: 4810, trigger: "cron", suiteRunId: "suite-nightly" }),
  run({ id: "r-1034", name: "picodata-smoke", author: "max", status: "pending", db: "picodata", workload: "ycsb", stroppy: "5.1.2", topology: "Picodata x3", nodes: 3, progress: 0, startedHoursAgo: 0.05, suiteRunId: "suite-nightly", trigger: "cron" }),
  run({ id: "r-1033", name: "mariadb-pgbench", author: "grace", status: "failed", db: "mariadb", workload: "pgbench", stroppy: "5.0.9", topology: "MariaDB single", nodes: 1, progress: 88, startedHoursAgo: 96, durationSec: 1502, trigger: "manual" }),
  run({ id: "r-1032", name: "ydb-managed-ycsb", author: "ada", status: "completed", db: "ydb_managed", workload: "ycsb", stroppy: "5.1.1", topology: "YDB managed", nodes: 0, progress: 100, startedHoursAgo: 110, durationSec: 2680, trigger: "api" }),
  run({ id: "r-1031", name: "external-baseline", author: "max", status: "completed", db: "external", workload: "tpcc", stroppy: "5.1.0", topology: "External", nodes: 0, progress: 100, startedHoursAgo: 130, durationSec: 6200, trigger: "manual" }),
  run({ id: "r-1030", name: "tpcc-scale-64", author: "ada", status: "running", db: "postgres", workload: "tpcc", stroppy: "5.1.2", topology: "PG HA x5", nodes: 5, progress: 31, startedHoursAgo: 0.1, trigger: "manual" }),
  run({ id: "r-1029", name: "ycsb-d-readlatest", author: "grace", status: "completed", db: "postgres", workload: "ycsb", stroppy: "5.1.2", topology: "PG single", nodes: 1, progress: 100, startedHoursAgo: 150, durationSec: 1990, trigger: "cron", suiteRunId: "suite-weekly" }),
  run({ id: "r-1028", name: "cockroach-ycsb-e", author: "max", status: "running", db: "cockroach", workload: "ycsb", stroppy: "5.1.2", topology: "CRDB x5", nodes: 5, progress: 12, startedHoursAgo: 0.2, trigger: "api" }),
  run({ id: "r-1027", name: "picodata-tpcc", author: "grace", status: "completed", db: "picodata", workload: "tpcc", stroppy: "5.1.1", topology: "Picodata x3", nodes: 3, progress: 100, startedHoursAgo: 168, durationSec: 5400, trigger: "manual" }),
  run({ id: "r-1026", name: "mysql-pgbench-ro", author: "lee", status: "failed", db: "mysql", workload: "pgbench", stroppy: "5.0.9", topology: "MySQL HA x3", nodes: 3, progress: 7, startedHoursAgo: 200, durationSec: 60, trigger: "cron", suiteRunId: "suite-weekly" }),
  // A soft-deleted row: hidden unless include_deleted is on.
  run({ id: "r-1025", name: "tpcc-archived", author: "max", status: "completed", db: "postgres", workload: "tpcc", stroppy: "5.0.9", topology: "PG single", nodes: 1, progress: 100, startedHoursAgo: 240, durationSec: 900, trigger: "manual", deleted: true }),
];

const GLOBEX: RunVM[] = [
  run({ id: "g-310", name: "ycsb-b-readheavy", author: "ci", status: "running", db: "postgres", workload: "ycsb", stroppy: "5.1.2", topology: "PG single", nodes: 1, progress: 55, startedHoursAgo: 0.5, trigger: "api" }),
  run({ id: "g-309", name: "tpcc-smoke", author: "ci", status: "completed", db: "postgres", workload: "tpcc", stroppy: "5.1.2", topology: "PG single", nodes: 1, progress: 100, startedHoursAgo: 6, durationSec: 720, trigger: "cron", suiteRunId: "suite-smoke" }),
  run({ id: "g-308", name: "pgbench-ro", author: "ci", status: "failed", db: "postgres", workload: "pgbench", stroppy: "5.1.1", topology: "PG single", nodes: 1, progress: 12, startedHoursAgo: 26, durationSec: 95, trigger: "api" }),
  run({ id: "g-307", name: "mysql-ycsb-a", author: "lee", status: "completed", db: "mysql", workload: "ycsb", stroppy: "5.1.1", topology: "MySQL single", nodes: 1, progress: 100, startedHoursAgo: 30, durationSec: 1610, trigger: "manual" }),
  run({ id: "g-306", name: "cockroach-baseline", author: "lee", status: "completed", db: "cockroach", workload: "tpcc", stroppy: "5.1.0", topology: "CRDB x3", nodes: 3, progress: 100, startedHoursAgo: 52, durationSec: 4400, trigger: "manual" }),
  run({ id: "g-305", name: "ydb-tpcc", author: "ci", status: "cancelled", db: "ydb", workload: "tpcc", stroppy: "5.1.0", topology: "YDB x3", nodes: 3, progress: 22, startedHoursAgo: 70, durationSec: 410, trigger: "cron", suiteRunId: "suite-smoke" }),
];

const BY_SLUG: Record<string, RunVM[]> = {
  acme: ACME,
  globex: GLOBEX,
  // initech intentionally falls through to [] to show the empty state.
};

// --- In-memory query application (mirrors the wired ListTestRunsRequest slice).

function matchesQuery(r: RunVM, q: RunsQuery): boolean {
  if (q.search) {
    if (!r.name.toLowerCase().includes(q.search.toLowerCase())) return false;
  }
  if (q.statuses && q.statuses.length > 0) {
    if (!q.statuses.includes(r.status)) return false;
  }
  if (q.dbKinds && q.dbKinds.length > 0) {
    if (r.dbKind === "" || !q.dbKinds.includes(r.dbKind)) return false;
  }
  if (q.stroppyVersions && q.stroppyVersions.length > 0) {
    if (!q.stroppyVersions.includes(r.stroppyVersion)) return false;
  }
  if (q.protocols && q.protocols.length > 0) {
    if (r.protocol === "" || !q.protocols.includes(r.protocol)) return false;
  }
  if (q.triggers && q.triggers.length > 0) {
    if (r.trigger === "" || !q.triggers.includes(r.trigger)) return false;
  }
  if (q.authorIds && q.authorIds.length > 0) {
    if (!r.authorId || !q.authorIds.includes(r.authorId)) return false;
  }
  // standalone: undefined = all runs; true = only non-suite (no suite_run_id).
  if (q.standalone === true && r.suiteRunId !== "") return false;
  // favorites_only: keep only rows the caller has favorited.
  if (q.favoritesOnly && !r.favorite) return false;
  // include_deleted: soft-deleted rows are hidden unless explicitly included.
  if (!q.includeDeleted && r.deleted) return false;
  // progress window (0..100).
  if (q.progressMin !== undefined && r.progressPct < q.progressMin) return false;
  if (q.progressMax !== undefined && r.progressPct > q.progressMax) return false;
  // duration window (seconds); rows without a duration are excluded once bounded.
  if (q.durationMinSec !== undefined && (r.durationSec ?? -1) < q.durationMinSec)
    return false;
  if (q.durationMaxSec !== undefined && (r.durationSec ?? Infinity) > q.durationMaxSec)
    return false;
  // Time windows compare ISO strings lexically (ISO sorts chronologically).
  if (q.startedAfter && (!r.startedAt || r.startedAt < q.startedAfter)) return false;
  if (q.startedBefore && (!r.startedAt || r.startedAt > q.startedBefore)) return false;
  if (q.finishedAfter && (!r.finishedAt || r.finishedAt < q.finishedAfter)) return false;
  if (q.finishedBefore && (!r.finishedAt || r.finishedAt > q.finishedBefore)) return false;
  return true;
}

function compare(a: RunVM, b: RunVM, q: RunsQuery): number {
  const dir = q.desc ? -1 : 1;
  switch (q.sort) {
    case "name":
      return dir * a.name.localeCompare(b.name);
    case "status":
      return dir * a.status.localeCompare(b.status);
    case "db_kind":
      return dir * a.dbKind.localeCompare(b.dbKind);
    case "workload":
      return dir * a.workload.localeCompare(b.workload);
    case "trigger":
      return dir * a.trigger.localeCompare(b.trigger);
    case "duration":
      return dir * ((a.durationSec ?? 0) - (b.durationSec ?? 0));
    case "finished_at":
      return dir * ts(a.finishedAt).localeCompare(ts(b.finishedAt));
    case "created_at":
      return dir * a.createdAt.localeCompare(b.createdAt);
    case "started_at":
    default:
      return dir * ts(a.startedAt).localeCompare(ts(b.startedAt));
  }
}

function ts(iso?: string): string {
  return iso ?? "";
}

const DEFAULT_SIZE = 10;

// Mutable per-slug store so the row Actions (cancel / rerun / delete) are
// genuinely demonstrable in the preview: cancel flips a row's status, delete
// removes it, rerun prepends a fresh running row. Seeded from the canned lists
// above; never touches them so a remount re-seeds cleanly only on full reload.
function rows(slug: string): RunVM[] {
  let list = BY_SLUG[slug];
  if (!list) {
    list = [];
    BY_SLUG[slug] = list;
  }
  return list;
}

// A short delay matching listRuns so action handlers exercise the same
// loading/refresh path the real RPC round-trip would.
function delay(): Promise<void> {
  return new Promise((r) => setTimeout(r, 150));
}

// Distinct facet values present in a slug's data, so the page can offer
// pick-lists for the id-keyed facets (authors / presets) instead of free text.
// A real provider would surface these from a faceting RPC; here we derive them
// from the canned rows (including soft-deleted ones so the option survives when
// "show deleted" is on). Sorted for a stable menu order.
function distinct(values: string[]): string[] {
  return [...new Set(values.filter(Boolean))].sort();
}

export const mockRunsProvider: RunsProvider = {
  async listFacets(tenantSlug: string): Promise<RunFacets> {
    await delay();
    const all = rows(tenantSlug);
    return {
      authorIds: distinct(all.map((r) => r.authorId)),
    };
  },

  async listRuns(tenantSlug: string, query: RunsQuery): Promise<RunsPage> {
    // Simulate a little network latency so loading states are visible.
    await delay();

    const all = rows(tenantSlug);
    const filtered = all
      .filter((r) => matchesQuery(r, query))
      // default ordering = started_at desc, matching the page's default sort.
      .sort((a, b) =>
        compare(a, b, query.sort ? query : { ...query, sort: "started_at", desc: true }),
      );

    const size = query.pageSize && query.pageSize > 0 ? query.pageSize : DEFAULT_SIZE;
    const offset = query.pageToken ? Number.parseInt(query.pageToken, 10) || 0 : 0;
    const slice = filtered.slice(offset, offset + size);
    const nextOffset = offset + size;
    const nextPageToken = nextOffset < filtered.length ? String(nextOffset) : "";

    return { runs: slice, nextPageToken };
  },

  // CancelTestRun (mock): request cancellation, leaving the run in-flight until
  // the backend workflow would later report CANCELLED.
  async cancelRun(tenantSlug: string, runId: string): Promise<void> {
    await delay();
    const r = rows(tenantSlug).find((x) => x.id === runId);
    if (r) {
      r.status = "cancelling";
      r.finishedAt = undefined;
      r.durationSec = undefined;
    }
  },

  // StartTestRun via testRunId (mock): prepend a fresh, just-started run that
  // re-uses the source run's target spec, so the new row shows up at the top.
  async rerunRun(tenantSlug: string, runId: string): Promise<void> {
    await delay();
    const list = rows(tenantSlug);
    const src = list.find((x) => x.id === runId);
    if (!src) return;
    const now = new Date().toISOString();
    list.unshift({
      ...src,
      id: `${src.id}-rerun-${Date.now().toString(36)}`,
      status: "running",
      trigger: "manual",
      progressPct: 0,
      createdAt: now,
      startedAt: now,
      finishedAt: undefined,
      durationSec: undefined,
    });
  },

  // ExtractToPreset (mock): no list mutation — the preset lands in a different
  // surface. Resolve so the optimistic "preset saved" feedback can show.
  async extractToPreset(_tenantSlug: string, _runId: string): Promise<void> {
    await delay();
  },

  // DeleteTestRun (mock): drop the row from the slug's list.
  async deleteRun(tenantSlug: string, runId: string): Promise<void> {
    await delay();
    const list = rows(tenantSlug);
    const i = list.findIndex((x) => x.id === runId);
    if (i >= 0) list.splice(i, 1);
  },

  // FavoriteService AddFavorite / RemoveFavorite (mock): update the computed
  // per-caller favorite flag exposed on Entity.is_favorite.
  async setFavorite(
    tenantSlug: string,
    runId: string,
    favorite: boolean,
  ): Promise<void> {
    await delay();
    const r = rows(tenantSlug).find((x) => x.id === runId);
    if (r) r.favorite = favorite;
  },
};
