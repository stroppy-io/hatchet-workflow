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
import { suiteClient } from "@/services/client";
import { resolveTenantId } from "@/services/tenant";

export type { RunStatus };

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
  /** spec.cells[].preset_pair */
  presetPair?: {
    dbPresetId: string;
    workloadPresetId: string;
  };
  /** spec.cells[].test_preset_id */
  testPresetId?: string;
  /** spec.cells[].inline_test is set */
  inlineTest?: boolean;
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

export interface SuitesProvider {
  listSuites(tenantSlug: string, query: SuitesQuery): Promise<SuitesPage>;
  listFacets(tenantSlug: string): Promise<SuiteFacets>;
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
