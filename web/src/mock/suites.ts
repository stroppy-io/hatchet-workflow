// TEMPORARY mock for SuiteService.ListSuites and row mutations.

import {
  type RunStatus,
  type SuiteCellVM,
  type SuiteFacets,
  type SuiteProviderKind,
  type SuitesPage,
  type SuitesProvider,
  type SuitesQuery,
  type SuiteVM,
} from "@/services/suites";

function hoursAgo(h: number): string {
  return new Date(Date.now() - h * 3600_000).toISOString();
}

function hoursFromNow(h: number): string {
  return new Date(Date.now() + h * 3600_000).toISOString();
}

function makeCells(count: number, prefix: string): SuiteCellVM[] {
  return Array.from({ length: count }, (_, index) => ({
    id: `${prefix}-cell-${index + 1}`,
    name: `${prefix}-${index + 1}`,
    enabled: true,
    presetPair: {
      dbPresetId: `${prefix}-db-${index + 1}`,
      workloadPresetId: `${prefix}-workload-${index + 1}`,
    },
  }));
}

function suite(p: {
  id: string;
  name: string;
  description?: string;
  author: string;
  provider: SuiteProviderKind;
  cells: number;
  cellNames?: string[];
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
  const cells: SuiteCellVM[] = p.cellNames
    ? p.cellNames.map((name, index) => ({
        id: `${p.id}-cell-${index + 1}`,
        name,
        enabled: true,
        presetPair: {
          dbPresetId: `${p.name}-db-${index + 1}`,
          workloadPresetId: `${p.name}-workload-${index + 1}`,
        },
      }))
    : makeCells(p.cells, p.name);
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
    cellCount: p.cells,
    defaultMaxParallel: p.maxParallel ?? 0,
    defaultInTenantRating: p.tenantRating,
    defaultInGlobalRating: p.globalRating,
  };
}

const ACME: SuiteVM[] = [
  suite({ id: "suite-nightly", name: "suite-nightly", description: "Nightly Postgres baseline checks", author: "ada", provider: "docker", cells: 6, cellNames: ["pg-ha-tpcc", "pg-ha-ycsb", "pg-single-tpcc", "pg-single-ycsb", "mysql-ycsb", "crdb-tpcc"], runs: 182, schedule: true, cron: "0 2 * * *", timezone: "UTC", nextInHours: 7, lastHoursAgo: 20, lastStatus: "completed", maxParallel: 2, tenantRating: true, globalRating: false, tags: ["nightly", "baseline"], labels: { env: "prod" }, favorite: true }),
  suite({ id: "suite-weekly", name: "suite-weekly", description: "Long weekly mixed workload suite", author: "grace", provider: "docker", cells: 9, runs: 41, schedule: true, cron: "0 3 * * 1", timezone: "UTC", nextInHours: 74, lastHoursAgo: 150, lastStatus: "failed", maxParallel: 3, tags: ["weekly", "long"] }),
  suite({ id: "suite-ydb-smoke", name: "ydb-smoke", description: "Managed YDB quick compatibility check", author: "ada", provider: "yandex", cells: 4, runs: 57, schedule: true, cron: "*/30 * * * *", timezone: "Europe/Moscow", nextInHours: 0.4, lastHoursAgo: 0.6, lastStatus: "running", maxParallel: 1, labels: { provider: "yc" }, favorite: true }),
  suite({ id: "suite-crdb-regression", name: "crdb-regression", description: "CockroachDB regression bundle", author: "max", provider: "docker", cells: 5, runs: 23, schedule: false, lastHoursAgo: 50, lastStatus: "cancelled", maxParallel: 0, tags: ["regression"] }),
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

  async startSuite(tenantSlug: string, suiteId: string): Promise<{ suiteRunId: string }> {
    await delay();
    const s = rows(tenantSlug).find((x) => x.id === suiteId);
    if (!s || s.deleted) throw new Error("Suite is deleted");
    s.runCount += 1;
    s.lastRunAt = new Date().toISOString();
    s.lastRunStatus = "running";
    s.updatedAt = s.lastRunAt;
    return { suiteRunId: `sr-${Date.now().toString(36)}` };
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
