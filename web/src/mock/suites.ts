// TEMPORARY mock for SuiteService.ListSuites/GetSuite and row mutations.

import type { DbKind, DeployProvider } from "@/services/runs";
import {
  type RunStatus,
  type SuiteCellInput,
  type SuiteCellVM,
  type SuiteFacets,
  type SuiteProviderKind,
  type SuiteSchedulePatch,
  type SuitesPage,
  type SuitesProvider,
  type SuitesQuery,
  type SuiteVM,
} from "@/services/suites";
import { createSuiteRun } from "@/mock/suiteRuns";
import { mockPresetProvider } from "@/mock/preset";

// Default db kind per cell, used for the db×workload matrix axis when a seed
// does not specify one explicitly.
const DEFAULT_DB: DbKind = "postgres";

function providerToDeploy(p: SuiteProviderKind): DeployProvider {
  return p === "yandex" ? "yandex" : p === "docker" ? "docker" : "";
}

/**
 * Resolve a cell's display axes (dbKind / workload) the way the server would:
 * from the chosen preset(s). For preset_pair we read the db preset row's
 * db_kind and the workload preset row's name; for test_preset_id we read the
 * test preset row's db_kind + name; for inline_test the author supplied them.
 * Returns "" / a placeholder when a preset is missing (unresolvable axis).
 */
async function resolveCell(
  tenantSlug: string,
  cellId: string,
  input: SuiteCellInput,
): Promise<SuiteCellVM> {
  const base = {
    id: cellId,
    name: input.name,
    enabled: input.enabled,
  };
  if (input.source === "presetPair") {
    const dbPresetId = input.dbPresetId ?? "";
    const workloadPresetId = input.workloadPresetId ?? "";
    const [dbPage, wlPage] = await Promise.all([
      mockPresetProvider.listDatabasePresetRows(tenantSlug, { pageSize: 500 }),
      mockPresetProvider.listWorkloadPresetRows(tenantSlug, { pageSize: 500 }),
    ]);
    const db = dbPage.rows.find((r) => r.id === dbPresetId);
    const wl = wlPage.rows.find((r) => r.id === workloadPresetId);
    return {
      ...base,
      source: "presetPair",
      presetPair: { dbPresetId, workloadPresetId },
      dbKind: db?.dbKind ?? "",
      workload: wl?.name ?? workloadPresetId,
    };
  }
  if (input.source === "testPreset") {
    const testPresetId = input.testPresetId ?? "";
    const tpPage = await mockPresetProvider.listTestPresetRows(tenantSlug, {
      pageSize: 500,
    });
    const tp = tpPage.rows.find((r) => r.id === testPresetId);
    return {
      ...base,
      source: "testPreset",
      testPresetId,
      dbKind: tp?.dbKind ?? "",
      workload: tp?.name ?? testPresetId,
    };
  }
  // inline_test — author supplied the axes directly.
  return {
    ...base,
    source: "inline",
    inlineTest: true,
    dbKind: input.inlineDbKind ?? "",
    workload: input.inlineWorkload ?? "inline",
  };
}

function hoursAgo(h: number): string {
  return new Date(Date.now() - h * 3600_000).toISOString();
}

function hoursFromNow(h: number): string {
  return new Date(Date.now() + h * 3600_000).toISOString();
}

/**
 * Explicit matrix cell spec keyed by CATALOG preset ids so cells that share a
 * database / workload preset collapse onto the same matrix row / column (a real
 * database × workload grid, not a diagonal). dbKind / workload are display-only
 * fallbacks; the live mock re-resolves them from the catalog on every mutation.
 */
interface CellSpec {
  dbPresetId: string;
  workloadPresetId: string;
  dbKind: DbKind;
  workload: string;
  enabled?: boolean;
}

// Catalog preset ids the seeds draw from (acme tenant), so the matrix groups.
const PG_SINGLE = "dbp-pg-single";
const PG_HA = "dbp-pg-ha";
const MYSQL = "dbp-mysql-group";
const CRDB = "dbp-crdb-3";
const YDB = "dbp-ydb-b42";
const YDB_MANAGED = "dbp-ydb-managed";
const W_TPCC = "wlp-tpcc";
const W_YCSB_A = "wlp-ycsb-a";
const W_YCSB_C = "wlp-ycsb-c";
const W_PGBENCH = "wlp-pgbench";

/** A small but real matrix: 2 db rows × N workload columns. */
function makeCells(count: number, prefix: string, db: DbKind): SuiteCellVM[] {
  const dbPresets = [PG_SINGLE, PG_HA];
  const wlPresets = [W_TPCC, W_YCSB_A, W_YCSB_C, W_PGBENCH];
  const wlNames = ["tpcc", "ycsb-a", "ycsb-c", "pgbench"];
  return Array.from({ length: count }, (_, index) => {
    const dbIdx = index % dbPresets.length;
    const wlIdx = Math.floor(index / dbPresets.length) % wlPresets.length;
    return {
      id: `${prefix}-cell-${index + 1}`,
      name: "",
      enabled: true,
      source: "presetPair" as const,
      presetPair: {
        dbPresetId: dbPresets[dbIdx],
        workloadPresetId: wlPresets[wlIdx],
      },
      dbKind: db,
      workload: wlNames[wlIdx],
    };
  });
}

function suite(p: {
  id: string;
  name: string;
  description?: string;
  author: string;
  provider: SuiteProviderKind;
  cells: number;
  cellNames?: string[];
  /** Explicit db×workload matrix cells (overrides cells/cellNames). */
  cellSpecs?: CellSpec[];
  /** Default db kind for makeCells / cellNames-derived cells. */
  db?: DbKind;
  runs: number;
  schedule?: boolean;
  cron?: string;
  timezone?: string;
  nextInHours?: number;
  lastHoursAgo?: number;
  lastStatus?: RunStatus;
  maxParallel?: number;
  tenantRating?: boolean;
  globalRating?: boolean;
  tags?: string[];
  labels?: Record<string, string>;
  favorite?: boolean;
  deleted?: boolean;
}): SuiteVM {
  const createdAt = hoursAgo((p.lastHoursAgo ?? 24) + 12);
  const updatedAt = hoursAgo(Math.max((p.lastHoursAgo ?? 4) - 0.2, 0.1));
  const db = p.db ?? DEFAULT_DB;
  let cells: SuiteCellVM[];
  if (p.cellSpecs) {
    cells = p.cellSpecs.map((spec, index) => ({
      id: `${p.id}-cell-${index + 1}`,
      name: "",
      enabled: spec.enabled ?? true,
      source: "presetPair" as const,
      presetPair: {
        dbPresetId: spec.dbPresetId,
        workloadPresetId: spec.workloadPresetId,
      },
      dbKind: spec.dbKind,
      workload: spec.workload,
    }));
  } else if (p.cellNames) {
    // Legacy name list — spread over a 2-row matrix of catalog presets so the
    // grid still groups (db row × workload column) rather than a diagonal.
    const dbPresets = [PG_SINGLE, PG_HA];
    const wlPresets = [W_TPCC, W_YCSB_A, W_YCSB_C, W_PGBENCH];
    const wlNames = ["tpcc", "ycsb-a", "ycsb-c", "pgbench"];
    cells = p.cellNames.map((_name, index) => {
      const dbIdx = index % dbPresets.length;
      const wlIdx = Math.floor(index / dbPresets.length) % wlPresets.length;
      return {
        id: `${p.id}-cell-${index + 1}`,
        name: "",
        enabled: true,
        source: "presetPair" as const,
        presetPair: {
          dbPresetId: dbPresets[dbIdx],
          workloadPresetId: wlPresets[wlIdx],
        },
        dbKind: db,
        workload: wlNames[wlIdx],
      };
    });
  } else {
    cells = makeCells(p.cells, p.name, db);
  }
  return {
    id: p.id,
    name: p.name,
    description: p.description ?? "",
    authorId: p.author,
    createdAt,
    updatedAt,
    deleted: p.deleted ?? false,
    deletedAt: p.deleted ? hoursAgo(p.lastHoursAgo ?? 240) : undefined,
    favorite: p.favorite ?? false,
    cells,
    provider: p.provider,
    tags: p.tags ?? [],
    labels: p.labels ?? {},
    scheduleEnabled: p.schedule ?? false,
    cron: p.cron ?? "",
    timezone: p.timezone ?? "UTC",
    nextRunAt: p.schedule && p.nextInHours !== undefined ? hoursFromNow(p.nextInHours) : undefined,
    lastRunAt: p.lastHoursAgo !== undefined ? hoursAgo(p.lastHoursAgo) : undefined,
    lastRunStatus: p.lastStatus,
    runCount: p.runs,
    cellCount: cells.filter((c) => c.enabled).length,
    defaultMaxParallel: p.maxParallel ?? 0,
    defaultInTenantRating: p.tenantRating,
    defaultInGlobalRating: p.globalRating,
  };
}

const ACME: SuiteVM[] = [
  suite({ id: "suite-nightly", name: "suite-nightly", description: "Nightly Postgres baseline checks", author: "ada", provider: "docker", cells: 6, cellNames: ["pg-ha-tpcc", "pg-ha-ycsb", "pg-single-tpcc", "pg-single-ycsb", "mysql-ycsb", "crdb-tpcc"], runs: 182, schedule: true, cron: "0 2 * * *", timezone: "UTC", nextInHours: 7, lastHoursAgo: 20, lastStatus: "completed", maxParallel: 2, tenantRating: true, globalRating: false, tags: ["nightly", "baseline"], labels: { env: "prod" }, favorite: true }),
  suite({ id: "suite-weekly", name: "suite-weekly", description: "Long weekly mixed workload suite", author: "grace", provider: "docker", cells: 9, runs: 41, schedule: true, cron: "0 3 * * 1", timezone: "UTC", nextInHours: 74, lastHoursAgo: 150, lastStatus: "failed", maxParallel: 3, tags: ["weekly", "long"] }),
  suite({ id: "suite-ydb-smoke", name: "ydb-smoke", description: "Managed YDB quick compatibility check", author: "ada", provider: "yandex", cells: 4, cellSpecs: [{ dbPresetId: YDB, dbKind: "ydb", workloadPresetId: W_YCSB_C, workload: "ycsb-c" }, { dbPresetId: YDB, dbKind: "ydb", workloadPresetId: W_YCSB_A, workload: "ycsb-a" }, { dbPresetId: YDB_MANAGED, dbKind: "ydb_managed", workloadPresetId: W_YCSB_C, workload: "ycsb-c" }, { dbPresetId: YDB_MANAGED, dbKind: "ydb_managed", workloadPresetId: W_TPCC, workload: "tpcc" }], runs: 57, schedule: true, cron: "*/30 * * * *", timezone: "Europe/Moscow", nextInHours: 0.4, lastHoursAgo: 0.6, lastStatus: "running", maxParallel: 1, tenantRating: true, globalRating: false, tags: ["smoke", "ydb"], labels: { provider: "yc" }, favorite: true }),
  suite({ id: "suite-crdb-regression", name: "crdb-regression", description: "CockroachDB + MySQL regression bundle", author: "max", provider: "docker", cells: 4, cellSpecs: [{ dbPresetId: CRDB, dbKind: "cockroach", workloadPresetId: W_TPCC, workload: "tpcc" }, { dbPresetId: CRDB, dbKind: "cockroach", workloadPresetId: W_YCSB_A, workload: "ycsb-a" }, { dbPresetId: MYSQL, dbKind: "mysql", workloadPresetId: W_TPCC, workload: "tpcc" }, { dbPresetId: MYSQL, dbKind: "mysql", workloadPresetId: W_YCSB_A, workload: "ycsb-a", enabled: false }], runs: 23, schedule: false, lastHoursAgo: 50, lastStatus: "cancelled", maxParallel: 0, tags: ["regression"] }),
  suite({ id: "suite-cancel-demo", name: "cancel-demo", description: "Cancellation path validation", author: "grace", provider: "docker", cells: 3, runs: 12, schedule: false, lastHoursAgo: 0.2, lastStatus: "cancelling", maxParallel: 1, tags: ["ops"] }),
  suite({ id: "suite-picodata-lab", name: "picodata-lab", description: "Picodata exploratory presets", author: "max", provider: "docker", cells: 7, runs: 9, schedule: false, lastHoursAgo: 96, lastStatus: "completed", maxParallel: 2, labels: { track: "lab" } }),
  suite({ id: "suite-empty-draft", name: "draft-api-suite", description: "API-created draft", author: "lee", provider: "docker", cells: 2, runs: 0, schedule: false, maxParallel: 0 }),
  suite({ id: "suite-archived", name: "archived-old-weekly", description: "Soft-deleted old weekly suite", author: "max", provider: "docker", cells: 4, runs: 66, schedule: false, lastHoursAgo: 240, lastStatus: "completed", tags: ["archived"], deleted: true }),
];

const GLOBEX: SuiteVM[] = [
  suite({ id: "globex-smoke", name: "globex-smoke", description: "Small CI smoke suite", author: "ci", provider: "docker", cells: 3, runs: 73, schedule: true, cron: "*/15 * * * *", timezone: "UTC", nextInHours: 0.2, lastHoursAgo: 0.4, lastStatus: "completed", maxParallel: 1, tags: ["ci"], favorite: true }),
  suite({ id: "globex-weekly", name: "globex-weekly", description: "Weekly capacity run", author: "lee", provider: "docker", cells: 8, runs: 18, schedule: true, cron: "0 4 * * 0", timezone: "UTC", nextInHours: 30, lastHoursAgo: 52, lastStatus: "running", maxParallel: 2, labels: { team: "capacity" } }),
  suite({ id: "globex-ydb", name: "globex-ydb", description: "YDB managed suite", author: "ci", provider: "yandex", cells: 5, runs: 11, schedule: false, lastHoursAgo: 80, lastStatus: "failed", maxParallel: 1, tags: ["ydb"] }),
];

const BY_SLUG: Record<string, SuiteVM[]> = {
  acme: ACME,
  globex: GLOBEX,
};

function rows(slug: string): SuiteVM[] {
  let list = BY_SLUG[slug];
  if (!list) {
    list = [];
    BY_SLUG[slug] = list;
  }
  return list;
}

function delay(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 150));
}

function distinct(values: string[]): string[] {
  return [...new Set(values.filter(Boolean))].sort();
}

function distinctDb(values: DbKind[]): DbKind[] {
  return [...new Set(values.filter((v): v is Exclude<DbKind, ""> => v !== ""))];
}

function matchesQuery(s: SuiteVM, q: SuitesQuery): boolean {
  if (q.search) {
    const needle = q.search.toLowerCase();
    const hay = `${s.name} ${s.id} ${s.description}`.toLowerCase();
    if (!hay.includes(needle)) return false;
  }
  if (q.authorIds && q.authorIds.length > 0 && !q.authorIds.includes(s.authorId))
    return false;
  if (q.providers && q.providers.length > 0) {
    if (s.provider === "" || !q.providers.includes(s.provider)) return false;
  }
  if (q.schedule === "enabled" && !s.scheduleEnabled) return false;
  if (q.schedule === "disabled" && s.scheduleEnabled) return false;
  if (q.favoritesOnly && !s.favorite) return false;
  if (!q.includeDeleted && s.deleted) return false;
  if (q.createdAfter && s.createdAt < q.createdAfter) return false;
  if (q.createdBefore && s.createdAt > q.createdBefore) return false;
  if (q.updatedAfter && s.updatedAt < q.updatedAfter) return false;
  if (q.updatedBefore && s.updatedAt > q.updatedBefore) return false;
  return true;
}

function ts(iso?: string): string {
  return iso ?? "";
}

function compare(a: SuiteVM, b: SuiteVM, q: SuitesQuery): number {
  const dir = q.desc ? -1 : 1;
  switch (q.sort) {
    case "name":
      return dir * a.name.localeCompare(b.name);
    case "created_at":
      return dir * a.createdAt.localeCompare(b.createdAt);
    case "updated_at":
      return dir * a.updatedAt.localeCompare(b.updatedAt);
    case "provider":
      return dir * a.provider.localeCompare(b.provider);
    case "schedule_enabled":
      return dir * (Number(a.scheduleEnabled) - Number(b.scheduleEnabled));
    case "next_run_at":
      return dir * ts(a.nextRunAt).localeCompare(ts(b.nextRunAt));
    case "run_count":
      return dir * (a.runCount - b.runCount);
    case "last_run_at":
    default:
      return dir * ts(a.lastRunAt).localeCompare(ts(b.lastRunAt));
  }
}

const DEFAULT_SIZE = 25;

export const mockSuitesProvider: SuitesProvider = {
  async listFacets(tenantSlug: string): Promise<SuiteFacets> {
    await delay();
    const all = rows(tenantSlug);
    return { authorIds: distinct(all.map((s) => s.authorId)) };
  },

  async listSuites(tenantSlug: string, query: SuitesQuery): Promise<SuitesPage> {
    await delay();
    const filtered = rows(tenantSlug)
      .filter((s) => matchesQuery(s, query))
      .sort((a, b) =>
        compare(a, b, query.sort ? query : { ...query, sort: "updated_at", desc: true }),
      );
    const size = query.pageSize && query.pageSize > 0 ? query.pageSize : DEFAULT_SIZE;
    const offset = query.pageToken ? Number.parseInt(query.pageToken, 10) || 0 : 0;
    const slice = filtered.slice(offset, offset + size);
    const nextOffset = offset + size;
    return {
      suites: slice,
      nextPageToken: nextOffset < filtered.length ? String(nextOffset) : "",
    };
  },

  async getSuite(tenantSlug: string, suiteId: string): Promise<SuiteVM | null> {
    await delay();
    const s = rows(tenantSlug).find((x) => x.id === suiteId);
    if (!s) return null;
    // Deep-ish copy so the page never mutates the store directly.
    return { ...s, cells: s.cells.map((c) => ({ ...c })), tags: [...s.tags] };
  },

  async startSuite(tenantSlug: string, suiteId: string): Promise<{ suiteRunId: string }> {
    await delay();
    const s = rows(tenantSlug).find((x) => x.id === suiteId);
    if (!s || s.deleted) throw new Error("Suite is deleted");
    const run = createSuiteRun(tenantSlug, {
      suiteId: s.id,
      suiteName: s.name,
      provider: providerToDeploy(s.provider),
      dbKinds: distinctDb(s.cells.filter((c) => c.enabled).map((c) => c.dbKind)),
      maxParallel: s.defaultMaxParallel,
      cells: s.cells
        .filter((c) => c.enabled)
        .map((c) => ({ id: c.id, name: c.name })),
    });
    s.runCount += 1;
    s.lastRunAt = new Date().toISOString();
    s.lastRunStatus = "running";
    s.updatedAt = s.lastRunAt;
    return { suiteRunId: run.id };
  },

  async cloneSuite(
    tenantSlug: string,
    suiteId: string,
    name?: string,
  ): Promise<{ suiteId: string }> {
    await delay();
    const list = rows(tenantSlug);
    const src = list.find((x) => x.id === suiteId);
    if (!src) return { suiteId };
    const id = `${src.id}-copy-${Date.now().toString(36)}`;
    const now = new Date().toISOString();
    list.unshift({
      ...src,
      id,
      name: name || `${src.name}-copy`,
      description: `Clone of ${src.name}`,
      createdAt: now,
      updatedAt: now,
      cells: src.cells.map((cell, index) => ({
        ...cell,
        id: `${id}-cell-${index + 1}`,
      })),
      tags: [...src.tags],
      favorite: false,
      deleted: false,
      deletedAt: undefined,
      runCount: 0,
      lastRunAt: undefined,
      lastRunStatus: undefined,
    });
    return { suiteId: id };
  },

  async setScheduleEnabled(
    tenantSlug: string,
    suiteId: string,
    enabled: boolean,
  ): Promise<void> {
    await delay();
    const s = rows(tenantSlug).find((x) => x.id === suiteId);
    if (!s || s.deleted) throw new Error("Suite is deleted");
    s.scheduleEnabled = enabled;
    s.cron = s.cron || "0 2 * * *";
    s.nextRunAt = enabled ? hoursFromNow(12) : undefined;
    s.updatedAt = new Date().toISOString();
  },

  async setSchedule(
    tenantSlug: string,
    suiteId: string,
    schedule: SuiteSchedulePatch,
  ): Promise<void> {
    await delay();
    const s = rows(tenantSlug).find((x) => x.id === suiteId);
    if (!s || s.deleted) throw new Error("Suite is deleted");
    s.scheduleEnabled = schedule.enabled;
    s.cron = schedule.cron;
    s.timezone = schedule.timezone;
    s.nextRunAt = schedule.enabled ? hoursFromNow(12) : undefined;
    s.updatedAt = new Date().toISOString();
  },

  async setRatingFlags(
    tenantSlug: string,
    suiteId: string,
    flags: { inTenant?: boolean; inGlobal?: boolean },
  ): Promise<void> {
    await delay();
    const s = rows(tenantSlug).find((x) => x.id === suiteId);
    if (!s || s.deleted) throw new Error("Suite is deleted");
    if (flags.inTenant !== undefined) s.defaultInTenantRating = flags.inTenant;
    if (flags.inGlobal !== undefined) s.defaultInGlobalRating = flags.inGlobal;
    s.updatedAt = new Date().toISOString();
  },

  async setMaxParallel(
    tenantSlug: string,
    suiteId: string,
    maxParallel: number,
  ): Promise<void> {
    await delay();
    const s = rows(tenantSlug).find((x) => x.id === suiteId);
    if (!s || s.deleted) throw new Error("Suite is deleted");
    s.defaultMaxParallel = Math.max(0, Math.floor(maxParallel));
    s.updatedAt = new Date().toISOString();
  },

  async setCellEnabled(
    tenantSlug: string,
    suiteId: string,
    cellId: string,
    enabled: boolean,
  ): Promise<void> {
    await delay();
    const s = rows(tenantSlug).find((x) => x.id === suiteId);
    if (!s || s.deleted) throw new Error("Suite is deleted");
    const cell = s.cells.find((c) => c.id === cellId);
    if (cell) cell.enabled = enabled;
    s.cellCount = s.cells.filter((c) => c.enabled).length;
    s.updatedAt = new Date().toISOString();
  },

  async setDescription(
    tenantSlug: string,
    suiteId: string,
    description: string,
  ): Promise<void> {
    await delay();
    const s = rows(tenantSlug).find((x) => x.id === suiteId);
    if (!s || s.deleted) throw new Error("Suite is deleted");
    s.description = description;
    s.updatedAt = new Date().toISOString();
  },

  async setProvider(
    tenantSlug: string,
    suiteId: string,
    provider: SuiteProviderKind,
  ): Promise<void> {
    await delay();
    const s = rows(tenantSlug).find((x) => x.id === suiteId);
    if (!s || s.deleted) throw new Error("Suite is deleted");
    s.provider = provider;
    s.updatedAt = new Date().toISOString();
  },

  async setTags(
    tenantSlug: string,
    suiteId: string,
    tags: string[],
  ): Promise<void> {
    await delay();
    const s = rows(tenantSlug).find((x) => x.id === suiteId);
    if (!s || s.deleted) throw new Error("Suite is deleted");
    s.tags = [...new Set(tags.map((t) => t.trim()).filter(Boolean))];
    s.updatedAt = new Date().toISOString();
  },

  async addCell(
    tenantSlug: string,
    suiteId: string,
    cell: SuiteCellInput,
  ): Promise<{ cellId: string }> {
    await delay();
    const s = rows(tenantSlug).find((x) => x.id === suiteId);
    if (!s || s.deleted) throw new Error("Suite is deleted");
    const cellId = `${s.id}-cell-${Date.now().toString(36)}`;
    const resolved = await resolveCell(tenantSlug, cellId, cell);
    s.cells.push(resolved);
    s.cellCount = s.cells.filter((c) => c.enabled).length;
    s.updatedAt = new Date().toISOString();
    return { cellId };
  },

  async removeCell(
    tenantSlug: string,
    suiteId: string,
    cellId: string,
  ): Promise<void> {
    await delay();
    const s = rows(tenantSlug).find((x) => x.id === suiteId);
    if (!s || s.deleted) throw new Error("Suite is deleted");
    s.cells = s.cells.filter((c) => c.id !== cellId);
    s.cellCount = s.cells.filter((c) => c.enabled).length;
    s.updatedAt = new Date().toISOString();
  },

  async updateCell(
    tenantSlug: string,
    suiteId: string,
    cellId: string,
    cell: SuiteCellInput,
  ): Promise<void> {
    await delay();
    const s = rows(tenantSlug).find((x) => x.id === suiteId);
    if (!s || s.deleted) throw new Error("Suite is deleted");
    const index = s.cells.findIndex((c) => c.id === cellId);
    if (index < 0) return;
    s.cells[index] = await resolveCell(tenantSlug, cellId, cell);
    s.cellCount = s.cells.filter((c) => c.enabled).length;
    s.updatedAt = new Date().toISOString();
  },

  async deleteSuite(tenantSlug: string, suiteId: string): Promise<void> {
    await delay();
    const list = rows(tenantSlug);
    const s = list.find((x) => x.id === suiteId);
    if (!s) return;
    if (s.deleted) return;
    const now = new Date().toISOString();
    s.deleted = true;
    s.deletedAt = now;
    s.updatedAt = now;
  },

  async setFavorite(
    tenantSlug: string,
    suiteId: string,
    favorite: boolean,
  ): Promise<void> {
    await delay();
    const s = rows(tenantSlug).find((x) => x.id === suiteId);
    if (s) s.favorite = favorite;
  },
};
