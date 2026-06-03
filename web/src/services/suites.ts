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

import { create, toJson } from "@bufbuild/protobuf";
import {
  SuiteRecordSchema,
  type SuiteRecord,
} from "@/lib/proto/cloud/v1/models/suite_pb";
import {
  SuiteCellSchema,
  SuiteCell_PresetPairSchema,
  SuiteSchema,
  type SuiteCell,
} from "@/lib/proto/cloud/v1/domain/suite_pb";
import { TestSchema } from "@/lib/proto/cloud/v1/domain/test_pb";
import { TagsSchema } from "@/lib/proto/cloud/v1/common/tags_pb";
import { EntitySortField } from "@/lib/proto/cloud/v1/common/entity_pb";
import { ListSuitesRequest_Sort_Kind } from "@/lib/proto/cloud/v1/api/suite_pb";
import { FavoriteKind } from "@/lib/proto/cloud/v1/common/favorite_pb";
import { type RunStatus, statusToVM } from "@/services/dashboard";
import type { DbKind } from "@/services/runs";
import { suiteClient, favoriteClient } from "@/services/client";
import { resolveTenantId } from "@/services/tenant";
import {
  dbKindLabelFromJson,
  dbKindProto,
  providerLabelFromJson,
  providerProto,
} from "@/services/enums";

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

/**
 * Editable slice the non-wizard "quick create" submits — the `suite` half of a
 * CreateSuiteRequest. The server owns entity.id / tenant_id / timings; we only
 * author the suite name + provider + the matrix cells + concurrency.
 */
export interface SuiteCreateInput {
  name: string;
  description?: string;
  provider: SuiteProviderKind;
  cells: SuiteCellInput[];
  maxParallel?: number;
}

export interface SuitesProvider {
  listSuites(tenantSlug: string, query: SuitesQuery): Promise<SuitesPage>;
  listFacets(tenantSlug: string): Promise<SuiteFacets>;
  /** GetSuite — single SuiteRecord by id. Returns null when not found. */
  getSuite(tenantSlug: string, suiteId: string): Promise<SuiteVM | null>;
  /**
   * CreateSuite — persist a brand-new SuiteRecord directly (non-wizard "quick
   * create"). The server assigns entity.id / tenant_id / timings; we only ship
   * the editable spec (name + provider + cells + max_parallel). Returns the id.
   */
  createSuite(tenantSlug: string, input: SuiteCreateInput): Promise<{ suiteId: string }>;
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

// --- proto -> VM ------------------------------------------------------------

// Build the SuiteCell.source oneof from the authored cell intent.
function cellInputToSource(cell: SuiteCellInput): SuiteCell["source"] {
  if (cell.source === "presetPair") {
    return {
      case: "presetPair",
      value: create(SuiteCell_PresetPairSchema, {
        dbPresetId: cell.dbPresetId ?? "",
        workloadPresetId: cell.workloadPresetId ?? "",
      }),
    };
  }
  if (cell.source === "testPreset") {
    return { case: "testPresetId", value: cell.testPresetId ?? "" };
  }
  // inline: the matrix db axis lives in inline_test.database.kind; the workload
  // axis is server-derived (domain.Workload carries no display name field).
  return {
    case: "inlineTest",
    value: create(TestSchema, {
      database: { kind: dbKindProto(cell.inlineDbKind ?? "") },
    }),
  };
}

function mapCell(c: {
  id?: string;
  name?: string;
  enabled?: boolean;
  presetPair?: { dbPresetId?: string; workloadPresetId?: string };
  testPresetId?: string;
  inlineTest?: { database?: { kind?: string }; workload?: { name?: string } };
}): SuiteCellVM {
  const source: SuiteCellSource = c.presetPair
    ? "presetPair"
    : c.testPresetId !== undefined
      ? "testPreset"
      : "inline";
  return {
    id: c.id ?? "",
    name: c.name ?? "",
    enabled: c.enabled ?? false,
    source,
    presetPair: c.presetPair
      ? {
          dbPresetId: c.presetPair.dbPresetId ?? "",
          workloadPresetId: c.presetPair.workloadPresetId ?? "",
        }
      : undefined,
    testPresetId: c.testPresetId,
    inlineTest: c.inlineTest ? true : undefined,
    dbKind:
      source === "inline" ? dbKindLabelFromJson(c.inlineTest?.database?.kind) : "",
    workload: source === "inline" ? (c.inlineTest?.workload?.name ?? "") : "",
  };
}

function mapSuiteRecord(rec: SuiteRecord): SuiteVM {
  const j = toJson(SuiteRecordSchema, rec) as {
    entity?: {
      id?: string;
      name?: string;
      description?: string;
      authorId?: string;
      isFavorite?: boolean;
      timings?: { createdAt?: string; updatedAt?: string; deletedAt?: string };
    };
    spec?: {
      cells?: Parameters<typeof mapCell>[0][];
      provider?: string;
      tags?: { tags?: string[]; labels?: Record<string, string> };
      schedule?: { enabled?: boolean; cron?: string; timezone?: string };
      defaultInTenantRating?: boolean;
      defaultInGlobalRating?: boolean;
      defaultMaxParallel?: number;
    };
    summary?: {
      scheduleEnabled?: boolean;
      cron?: string;
      nextRunAt?: string;
      lastRunAt?: string;
      lastRunStatus?: string;
      runCount?: number;
      cellCount?: number;
    };
  };
  const e = j.entity ?? {};
  const spec = j.spec ?? {};
  const sum = j.summary ?? {};
  return {
    id: e.id ?? "",
    name: e.name ?? "",
    description: e.description ?? "",
    authorId: e.authorId ?? "",
    createdAt: e.timings?.createdAt ?? "",
    updatedAt: e.timings?.updatedAt ?? "",
    deleted: !!e.timings?.deletedAt,
    deletedAt: e.timings?.deletedAt,
    favorite: e.isFavorite ?? false,
    cells: (spec.cells ?? []).map(mapCell),
    provider: providerLabelFromJson(spec.provider) as SuiteProviderKind,
    tags: spec.tags?.tags ?? [],
    labels: spec.tags?.labels ?? {},
    scheduleEnabled: sum.scheduleEnabled ?? spec.schedule?.enabled ?? false,
    cron: sum.cron ?? spec.schedule?.cron ?? "",
    timezone: spec.schedule?.timezone ?? "",
    nextRunAt: sum.nextRunAt,
    lastRunAt: sum.lastRunAt,
    lastRunStatus: sum.lastRunStatus ? statusToVM(sum.lastRunStatus) : undefined,
    runCount: sum.runCount ?? 0,
    cellCount: sum.cellCount ?? 0,
    defaultMaxParallel: spec.defaultMaxParallel ?? 0,
    defaultInTenantRating: spec.defaultInTenantRating,
    defaultInGlobalRating: spec.defaultInGlobalRating,
  };
}

function suiteSort(query: SuitesQuery): {
  entity: EntitySortField;
  kind: ListSuitesRequest_Sort_Kind;
  desc: boolean;
} {
  const desc = query.desc ?? false;
  switch (query.sort) {
    case "name":
      return { entity: EntitySortField.NAME, kind: ListSuitesRequest_Sort_Kind.UNSPECIFIED, desc };
    case "created_at":
      return { entity: EntitySortField.CREATED_AT, kind: ListSuitesRequest_Sort_Kind.UNSPECIFIED, desc };
    case "updated_at":
      return { entity: EntitySortField.UPDATED_AT, kind: ListSuitesRequest_Sort_Kind.UNSPECIFIED, desc };
    case "provider":
      return { entity: EntitySortField.UNSPECIFIED, kind: ListSuitesRequest_Sort_Kind.PROVIDER, desc };
    case "schedule_enabled":
      return { entity: EntitySortField.UNSPECIFIED, kind: ListSuitesRequest_Sort_Kind.SCHEDULE_ENABLED, desc };
    case "next_run_at":
      return { entity: EntitySortField.UNSPECIFIED, kind: ListSuitesRequest_Sort_Kind.NEXT_RUN_AT, desc };
    case "last_run_at":
      return { entity: EntitySortField.UNSPECIFIED, kind: ListSuitesRequest_Sort_Kind.LAST_RUN_AT, desc };
    case "run_count":
      return { entity: EntitySortField.UNSPECIFIED, kind: ListSuitesRequest_Sort_Kind.RUN_COUNT, desc };
    default:
      return { entity: EntitySortField.UNSPECIFIED, kind: ListSuitesRequest_Sort_Kind.UNSPECIFIED, desc };
  }
}

// Read-modify-write helper for the UpdateSuite-backed cell/field mutations.
async function patchSuite(
  tenantId: string,
  suiteId: string,
  mutate: (rec: SuiteRecord) => void,
): Promise<SuiteRecord> {
  const { suite } = await suiteClient.getSuite({ tenantId, id: suiteId });
  if (!suite) throw new Error(`suite ${suiteId} not found`);
  mutate(suite);
  const { suite: updated } = await suiteClient.updateSuite({ tenantId, suite });
  return updated ?? suite;
}

const realSuitesProvider: SuitesProvider = {
  async listSuites(tenantSlug, query) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { suites, nextPageToken } = await suiteClient.listSuites({
      tenantId,
      filter: {
        search: query.search,
        authorIds: query.authorIds ?? [],
        favoritesOnly: query.favoritesOnly,
        includeDeleted: query.includeDeleted ?? false,
      },
      providers: query.providers?.map(providerProto),
      scheduleEnabled:
        query.schedule === undefined ? undefined : query.schedule === "enabled",
      sort: suiteSort(query),
      page: { size: query.pageSize ?? 0, token: query.pageToken ?? "" },
    });
    return { suites: suites.map(mapSuiteRecord), nextPageToken };
  },

  async listFacets(tenantSlug) {
    const tenantId = await resolveTenantId(tenantSlug);
    const resp = await suiteClient.listSuiteFacets({ tenantId });
    return { authorIds: resp.authorIds };
  },

  async getSuite(tenantSlug, suiteId) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { suite } = await suiteClient.getSuite({ tenantId, id: suiteId });
    return suite ? mapSuiteRecord(suite) : null;
  },

  async createSuite(tenantSlug, input) {
    const tenantId = await resolveTenantId(tenantSlug);
    const suite = create(SuiteRecordSchema, {
      entity: { name: input.name, description: input.description ?? "" },
      spec: create(SuiteSchema, {
        provider: providerProto(input.provider),
        defaultMaxParallel: input.maxParallel ?? 0,
        cells: input.cells.map((cell) =>
          create(SuiteCellSchema, {
            id: "",
            name: cell.name,
            enabled: cell.enabled,
            source: cellInputToSource(cell),
          }),
        ),
      }),
    });
    const { suite: created } = await suiteClient.createSuite({ tenantId, suite });
    return { suiteId: created?.entity?.id ?? "" };
  },

  async startSuite(tenantSlug, suiteId) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { suiteRun } = await suiteClient.startSuite({
      tenantId,
      source: { case: "suiteId", value: suiteId },
    });
    return { suiteRunId: suiteRun?.entity?.id ?? "" };
  },

  async cloneSuite(tenantSlug, suiteId, name) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { suite } = await suiteClient.cloneSuite({
      tenantId,
      id: suiteId,
      name: name ?? "",
    });
    return { suiteId: suite?.entity?.id ?? "" };
  },

  async setScheduleEnabled(tenantSlug, suiteId, enabled) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { suite } = await suiteClient.getSuite({ tenantId, id: suiteId });
    const sch = suite?.spec?.schedule;
    await suiteClient.setSuiteSchedule({
      tenantId,
      id: suiteId,
      schedule: {
        enabled,
        cron: sch?.cron ?? "",
        timezone: sch?.timezone ?? "",
      },
    });
  },

  async setSchedule(tenantSlug, suiteId, schedule) {
    const tenantId = await resolveTenantId(tenantSlug);
    await suiteClient.setSuiteSchedule({
      tenantId,
      id: suiteId,
      schedule: {
        enabled: schedule.enabled,
        cron: schedule.cron,
        timezone: schedule.timezone,
      },
    });
  },

  async setRatingFlags(tenantSlug, suiteId, flags) {
    const tenantId = await resolveTenantId(tenantSlug);
    await patchSuite(tenantId, suiteId, (rec) => {
      if (!rec.spec) return;
      if (flags.inTenant !== undefined) rec.spec.defaultInTenantRating = flags.inTenant;
      if (flags.inGlobal !== undefined) rec.spec.defaultInGlobalRating = flags.inGlobal;
    });
  },

  async setMaxParallel(tenantSlug, suiteId, maxParallel) {
    const tenantId = await resolveTenantId(tenantSlug);
    await patchSuite(tenantId, suiteId, (rec) => {
      if (rec.spec) rec.spec.defaultMaxParallel = maxParallel;
    });
  },

  async setCellEnabled(tenantSlug, suiteId, cellId, enabled) {
    const tenantId = await resolveTenantId(tenantSlug);
    await patchSuite(tenantId, suiteId, (rec) => {
      const cell = rec.spec?.cells.find((c) => c.id === cellId);
      if (cell) cell.enabled = enabled;
    });
  },

  async setDescription(tenantSlug, suiteId, description) {
    const tenantId = await resolveTenantId(tenantSlug);
    await patchSuite(tenantId, suiteId, (rec) => {
      if (rec.entity) rec.entity.description = description;
    });
  },

  async setProvider(tenantSlug, suiteId, provider) {
    const tenantId = await resolveTenantId(tenantSlug);
    await patchSuite(tenantId, suiteId, (rec) => {
      if (rec.spec) rec.spec.provider = providerProto(provider);
    });
  },

  async setTags(tenantSlug, suiteId, tags) {
    const tenantId = await resolveTenantId(tenantSlug);
    await patchSuite(tenantId, suiteId, (rec) => {
      if (rec.spec) {
        rec.spec.tags = create(TagsSchema, {
          tags,
          labels: rec.spec.tags?.labels ?? {},
        });
      }
    });
  },

  async addCell(tenantSlug, suiteId, cell) {
    const tenantId = await resolveTenantId(tenantSlug);
    const updated = await patchSuite(tenantId, suiteId, (rec) => {
      rec.spec?.cells.push(
        create(SuiteCellSchema, {
          id: "",
          name: cell.name,
          enabled: cell.enabled,
          source: cellInputToSource(cell),
        }),
      );
    });
    const cells = updated.spec?.cells ?? [];
    return { cellId: cells.length ? cells[cells.length - 1].id : "" };
  },

  async removeCell(tenantSlug, suiteId, cellId) {
    const tenantId = await resolveTenantId(tenantSlug);
    await patchSuite(tenantId, suiteId, (rec) => {
      if (rec.spec) rec.spec.cells = rec.spec.cells.filter((c) => c.id !== cellId);
    });
  },

  async updateCell(tenantSlug, suiteId, cellId, cell) {
    const tenantId = await resolveTenantId(tenantSlug);
    await patchSuite(tenantId, suiteId, (rec) => {
      const c = rec.spec?.cells.find((x) => x.id === cellId);
      if (c) {
        c.name = cell.name;
        c.enabled = cell.enabled;
        c.source = cellInputToSource(cell);
      }
    });
  },

  async deleteSuite(tenantSlug, suiteId) {
    const tenantId = await resolveTenantId(tenantSlug);
    await suiteClient.deleteSuite({ tenantId, id: suiteId });
  },

  async setFavorite(tenantSlug, suiteId, favorite) {
    const tenantId = await resolveTenantId(tenantSlug);
    if (favorite) {
      await favoriteClient.addFavorite({
        tenantId,
        kind: FavoriteKind.SUITE,
        targetId: suiteId,
      });
    } else {
      await favoriteClient.removeFavorite({
        tenantId,
        kind: FavoriteKind.SUITE,
        targetId: suiteId,
      });
    }
  },
};

let active: SuitesProvider = realSuitesProvider;

export function setSuitesProvider(provider: SuitesProvider): void {
  active = provider;
}

export function getSuitesProvider(): SuitesProvider {
  return active;
}
