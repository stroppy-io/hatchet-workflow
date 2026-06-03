// Suite definitions data surface for /t/:slug/suites.
//
// This page is built around cloud.v1.api.SuiteService.ListSuites and the
// SuiteRecord fields it returns:
//   * entity: id/name/description/author/timings/is_favorite
//   * spec: cells/provider/tags/schedule/default_* flags/default_max_parallel
//   * summary: schedule_enabled/cron/next_run_at/last_run_at/last_run_status/
//              run_count/cell_count
//
// Query state mirrors ListSuitesRequest only: EntityFilter fields, providers[],
// schedule_enabled, sort, and page. Favorites are backed by FavoriteService with
// FavoriteKind.SUITE.

import type { RunStatus } from "@/services/dashboard";
import type { DbKind } from "@/services/runs";
import { suiteClient } from "@/services/client";
import { resolveTenantId } from "@/services/tenant";

export type { RunStatus };

/** How a cell's test definition is sourced (oneof SuiteCell.source). */
export type SuiteCellSource = "presetPair" | "testPreset" | "inline";

export type SuiteProviderKind = "" | "docker" | "yandex";

export const SUITE_PROVIDERS: Exclude<SuiteProviderKind, "">[] = [
  "docker",
  "yandex",
];

export type SuiteScheduleFilter = "enabled" | "disabled";

export type SuiteSortField =
  | "name"
  | "created_at"
  | "updated_at"
  | "provider"
  | "schedule_enabled"
  | "next_run_at"
  | "last_run_at"
  | "run_count";

export interface SuiteCellVM {
  /** spec.cells[].id */
  id: string;
  /** spec.cells[].name */
  name: string;
  /** spec.cells[].enabled */
  enabled: boolean;
  /** Which oneof arm of spec.cells[].source is set. */
  source: SuiteCellSource;
  /** spec.cells[].preset_pair (when source === "presetPair") */
  presetPair?: {
    dbPresetId: string;
    workloadPresetId: string;
  };
  /** spec.cells[].test_preset_id (when source === "testPreset") */
  testPresetId?: string;
  /** spec.cells[].inline_test is set (when source === "inline") */
  inlineTest?: boolean;
  /**
   * dbKind is the matrix ROW axis. Display-derived: resolved by the backend
   * from the cell's db_preset_id / test_preset_id / inline_test.database.kind.
   * "" when unresolvable.
   */
  dbKind: DbKind;
  /**
   * workload is the matrix COLUMN axis. Display-derived: resolved from the
   * cell's workload_preset_id / test_preset_id / inline_test.workload.name.
   */
  workload: string;
}

export interface SuiteVM {
  /** entity.id */
  id: string;
  /** entity.name */
  name: string;
  /** entity.description */
  description: string;
  /** entity.author_id */
  authorId: string;
  /** entity.timings.created_at (ISO). */
  createdAt: string;
  /** entity.timings.updated_at (ISO). */
  updatedAt: string;
  /** entity.timings.deleted_at is set. */
  deleted: boolean;
  /** entity.timings.deleted_at (ISO). */
  deletedAt?: string;
  /** entity.is_favorite computed for the requesting account. */
  favorite: boolean;
  /** spec.cells */
  cells: SuiteCellVM[];
  /** spec.provider */
  provider: SuiteProviderKind;
  /** spec.tags.tags */
  tags: string[];
  /** spec.tags.labels */
  labels: Record<string, string>;
  /** spec.schedule.enabled mirrored by summary.schedule_enabled. */
  scheduleEnabled: boolean;
  /** summary.cron / spec.schedule.cron */
  cron: string;
  /** spec.schedule.timezone */
  timezone: string;
  /** summary.next_run_at */
  nextRunAt?: string;
  /** summary.last_run_at */
  lastRunAt?: string;
  /** summary.last_run_status */
  lastRunStatus?: RunStatus;
  /** summary.run_count */
  runCount: number;
  /** summary.cell_count */
  cellCount: number;
  /** spec.default_max_parallel */
  defaultMaxParallel: number;
  /** spec.default_in_tenant_rating */
  defaultInTenantRating?: boolean;
  /** spec.default_in_global_rating */
  defaultInGlobalRating?: boolean;
}

export interface SuitesQuery {
  /** filter.search */
  search?: string;
  /** filter.author_ids[] */
  authorIds?: string[];
  /** providers[] */
  providers?: Exclude<SuiteProviderKind, "">[];
  /** schedule_enabled optional bool, encoded as enabled/disabled in URL. */
  schedule?: SuiteScheduleFilter;
  /** filter.favorites_only */
  favoritesOnly?: boolean;
  /** filter.include_deleted */
  includeDeleted?: boolean;
  /** filter.created_after/before */
  createdAfter?: string;
  createdBefore?: string;
  /** filter.updated_after/before */
  updatedAfter?: string;
  updatedBefore?: string;
  sort?: SuiteSortField;
  desc?: boolean;
  pageSize?: number;
  pageToken?: string;
}

export interface SuitesPage {
  suites: SuiteVM[];
  nextPageToken: string;
}

export interface SuiteFacets {
  authorIds: string[];
}

export type SuiteAction =
  | "view"
  | "start"
  | "clone"
  | "enableSchedule"
  | "disableSchedule"
  | "delete";

export function actionsForSuite(suite: SuiteVM): Set<SuiteAction> {
  if (suite.deleted) return new Set<SuiteAction>(["view", "delete"]);
  const set = new Set<SuiteAction>(["view", "start", "clone", "delete"]);
  set.add(suite.scheduleEnabled ? "disableSchedule" : "enableSchedule");
  return set;
}

/** Schedule patch for SetSuiteSchedule (domain.Schedule fields). */
export interface SuiteSchedulePatch {
  enabled: boolean;
  /** spec.schedule.cron */
  cron: string;
  /** spec.schedule.timezone */
  timezone: string;
}

/**
 * A new/edited cell's authored intent, mapped onto domain.SuiteCell. The
 * `source` arm decides which oneof field is set:
 *   "presetPair" -> spec.cells[].preset_pair {db_preset_id, workload_preset_id}
 *   "testPreset" -> spec.cells[].test_preset_id
 *   "inline"     -> spec.cells[].inline_test (a domain.Test)
 * The matrix axes (dbKind / workload) are server-derived from the chosen
 * presets; the mock resolves them from the preset catalog. For an inline cell
 * the author supplies them directly (inline_test.database.kind / workload.name).
 */
export interface SuiteCellInput {
  /** spec.cells[].name — display label; empty lets the server derive one. */
  name: string;
  /** spec.cells[].enabled */
  enabled: boolean;
  source: SuiteCellSource;
  /** preset_pair arm. */
  dbPresetId?: string;
  workloadPresetId?: string;
  /** test_preset_id arm. */
  testPresetId?: string;
  /**
   * inline arm. The proto carries a full domain.Test (database + workload); the
   * UI authors only the matrix axes here (inline_test.database.kind +
   * inline_test.workload.name) and leaves the rest server-defaulted.
   */
  inlineDbKind?: DbKind;
  inlineWorkload?: string;
}

export interface SuitesProvider {
  listSuites(tenantSlug: string, query: SuitesQuery): Promise<SuitesPage>;
  listFacets(tenantSlug: string): Promise<SuiteFacets>;
  /** GetSuite — single SuiteRecord by id. Returns null when not found. */
  getSuite(tenantSlug: string, suiteId: string): Promise<SuiteVM | null>;
  startSuite(tenantSlug: string, suiteId: string): Promise<{ suiteRunId: string }>;
  cloneSuite(
    tenantSlug: string,
    suiteId: string,
    name?: string,
  ): Promise<{ suiteId: string }>;
  setScheduleEnabled(
    tenantSlug: string,
    suiteId: string,
    enabled: boolean,
  ): Promise<void>;
  /** SetSuiteSchedule — full schedule (enabled + cron + timezone). */
  setSchedule(
    tenantSlug: string,
    suiteId: string,
    schedule: SuiteSchedulePatch,
  ): Promise<void>;
  /** UpdateSuite — patch spec.default_in_tenant_rating / default_in_global_rating. */
  setRatingFlags(
    tenantSlug: string,
    suiteId: string,
    flags: { inTenant?: boolean; inGlobal?: boolean },
  ): Promise<void>;
  /** UpdateSuite — patch spec.default_max_parallel. */
  setMaxParallel(
    tenantSlug: string,
    suiteId: string,
    maxParallel: number,
  ): Promise<void>;
  /** UpdateSuite — toggle a single cell's enabled flag (spec.cells[].enabled). */
  setCellEnabled(
    tenantSlug: string,
    suiteId: string,
    cellId: string,
    enabled: boolean,
  ): Promise<void>;
  /** UpdateSuite — patch entity.description. */
  setDescription(
    tenantSlug: string,
    suiteId: string,
    description: string,
  ): Promise<void>;
  /** UpdateSuite — patch spec.provider (deployment.Provider). */
  setProvider(
    tenantSlug: string,
    suiteId: string,
    provider: SuiteProviderKind,
  ): Promise<void>;
  /** UpdateSuite — replace spec.tags.tags. */
  setTags(tenantSlug: string, suiteId: string, tags: string[]): Promise<void>;
  /** UpdateSuite — append a cell to spec.cells. Returns the new cell id. */
  addCell(
    tenantSlug: string,
    suiteId: string,
    cell: SuiteCellInput,
  ): Promise<{ cellId: string }>;
  /** UpdateSuite — drop a cell from spec.cells. */
  removeCell(
    tenantSlug: string,
    suiteId: string,
    cellId: string,
  ): Promise<void>;
  /** UpdateSuite — replace a cell's source + per-cell fields in spec.cells. */
  updateCell(
    tenantSlug: string,
    suiteId: string,
    cellId: string,
    cell: SuiteCellInput,
  ): Promise<void>;
  deleteSuite(tenantSlug: string, suiteId: string): Promise<void>;
  setFavorite(
    tenantSlug: string,
    suiteId: string,
    favorite: boolean,
  ): Promise<void>;
}

const NOT_WIRED =
  "real SuitesProvider not wired yet - run with VITE_MOCK=1 to preview the suites table";

const realSuitesProvider: SuitesProvider = {
  async listSuites() {
    // await suiteClient.listSuites({
    //   tenantId,
    //   filter: {
    //     search, authorIds, favoritesOnly, includeDeleted,
    //     createdAfter, createdBefore, updatedAfter, updatedBefore,
    //   },
    //   providers,
    //   scheduleEnabled,
    //   sort: { by: { entity | kind }, desc },
    //   page: { size, token },
    // });
    throw new Error(NOT_WIRED);
  },
  async listFacets(tenantSlug) {
    const tenantId = await resolveTenantId(tenantSlug);
    const resp = await suiteClient.listSuiteFacets({ tenantId });
    return { authorIds: resp.authorIds };
  },
  async getSuite() {
    // const resp = await suiteClient.getSuite({ tenantId, id: suiteId });
    // return mapSuiteRecord(resp.suite);
    throw new Error(NOT_WIRED);
  },
  async startSuite() {
    // await suiteClient.startSuite({ tenantId, suiteId });
    throw new Error(NOT_WIRED);
  },
  async cloneSuite() {
    // await suiteClient.cloneSuite({ tenantId, id: suiteId, name });
    throw new Error(NOT_WIRED);
  },
  async setScheduleEnabled() {
    // await suiteClient.setSuiteSchedule({ tenantId, id: suiteId, schedule });
    throw new Error(NOT_WIRED);
  },
  async setSchedule() {
    // await suiteClient.setSuiteSchedule({
    //   tenantId, id: suiteId, schedule: { enabled, cron, timezone },
    // });
    throw new Error(NOT_WIRED);
  },
  async setRatingFlags() {
    // const cur = await suiteClient.getSuite({ tenantId, id: suiteId });
    // cur.suite.spec.defaultInTenantRating = flags.inTenant;
    // cur.suite.spec.defaultInGlobalRating = flags.inGlobal;
    // await suiteClient.updateSuite({ tenantId, suite: cur.suite });
    throw new Error(NOT_WIRED);
  },
  async setMaxParallel() {
    // const cur = await suiteClient.getSuite({ tenantId, id: suiteId });
    // cur.suite.spec.defaultMaxParallel = maxParallel;
    // await suiteClient.updateSuite({ tenantId, suite: cur.suite });
    throw new Error(NOT_WIRED);
  },
  async setCellEnabled() {
    // const cur = await suiteClient.getSuite({ tenantId, id: suiteId });
    // const cell = cur.suite.spec.cells.find((c) => c.id === cellId);
    // if (cell) cell.enabled = enabled;
    // await suiteClient.updateSuite({ tenantId, suite: cur.suite });
    throw new Error(NOT_WIRED);
  },
  async setDescription() {
    // const cur = await suiteClient.getSuite({ tenantId, id: suiteId });
    // cur.suite.entity.description = description;
    // await suiteClient.updateSuite({ tenantId, suite: cur.suite });
    throw new Error(NOT_WIRED);
  },
  async setProvider() {
    // const cur = await suiteClient.getSuite({ tenantId, id: suiteId });
    // cur.suite.spec.provider = PROVIDER_TO_PROTO[provider];  // deployment.Provider
    // await suiteClient.updateSuite({ tenantId, suite: cur.suite });
    throw new Error(NOT_WIRED);
  },
  async setTags() {
    // const cur = await suiteClient.getSuite({ tenantId, id: suiteId });
    // cur.suite.spec.tags = create(TagsSchema, { tags });   // spec.tags.tags
    // await suiteClient.updateSuite({ tenantId, suite: cur.suite });
    throw new Error(NOT_WIRED);
  },
  async addCell() {
    // const cur = await suiteClient.getSuite({ tenantId, id: suiteId });
    // cur.suite.spec.cells.push(create(SuiteCellSchema, {
    //   id: "", name: cell.name, enabled: cell.enabled,
    //   source: cellInputToSource(cell),   // preset_pair / test_preset_id / inline_test
    // }));  // server assigns the cell id
    // const { suite } = await suiteClient.updateSuite({ tenantId, suite: cur.suite });
    // return { cellId: suite.spec.cells.at(-1)?.id ?? "" };
    throw new Error(NOT_WIRED);
  },
  async removeCell() {
    // const cur = await suiteClient.getSuite({ tenantId, id: suiteId });
    // cur.suite.spec.cells = cur.suite.spec.cells.filter((c) => c.id !== cellId);
    // await suiteClient.updateSuite({ tenantId, suite: cur.suite });
    throw new Error(NOT_WIRED);
  },
  async updateCell() {
    // const cur = await suiteClient.getSuite({ tenantId, id: suiteId });
    // const c = cur.suite.spec.cells.find((x) => x.id === cellId);
    // if (c) { c.name = cell.name; c.enabled = cell.enabled;
    //          c.source = cellInputToSource(cell); }
    // await suiteClient.updateSuite({ tenantId, suite: cur.suite });
    throw new Error(NOT_WIRED);
  },
  async deleteSuite() {
    // await suiteClient.deleteSuite({ tenantId, id: suiteId });
    throw new Error(NOT_WIRED);
  },
  async setFavorite() {
    // if (favorite) {
    //   await favoriteClient.addFavorite({
    //     tenantId, kind: FavoriteKind.SUITE, targetId: suiteId,
    //   });
    // } else {
    //   await favoriteClient.removeFavorite({
    //     tenantId, kind: FavoriteKind.SUITE, targetId: suiteId,
    //   });
    // }
    throw new Error(NOT_WIRED);
  },
};

let active: SuitesProvider = realSuitesProvider;

export function setSuitesProvider(provider: SuitesProvider): void {
  active = provider;
}

export function getSuitesProvider(): SuitesProvider {
  return active;
}
