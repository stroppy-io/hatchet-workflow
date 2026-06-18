// Database-preset data surface for the Test Wizard's Database step.
//
// The Database step is a three-pane progressive flow: pick an ENGINE, then a
// PRESET for that engine (a reusable, params-only domain.Database the platform
// or the tenant has seeded), then EDIT the typed settings the preset seeded.
// This module is the contract that second pane talks to — exactly like
// services/wizard.ts, the page depends ONLY on this interface, never on the
// mock or the wire format.
//
// PROTOBUF CORRESPONDENCE — one preset maps to one
// cloud.v1.models.DatabasePresetRecord:
//
//   DatabasePresetVM field   DatabasePresetRecord field
//   ----------------------   ----------------------------------------------
//   id                       entity.id
//   name                     entity.name
//   description              entity.description
//   isSystem                 is_system  (platform-seeded, read-only)
//   database                 database   (cloud.v1.domain.Database, typed)
//
// The real provider issues cloud.v1.api.DatabasePresetService.ListDatabasePresets
// (see src/lib/proto/cloud/v1/api/preset_pb.ts) and maps the proto
// DatabasePresetRecord onto the flat DatabaseVM the wizard step edits. The mock
// gate in main.tsx injects a stateful mock via setPresetProvider().

import { create, toJson } from "@bufbuild/protobuf";
import { timestampFromDate, timestampDate, type Timestamp } from "@bufbuild/protobuf/wkt";
import {
  ENGINE_TO_KIND,
  type DatabaseVM,
  type EngineKind,
  type WorkloadVM,
} from "@/services/wizard";
import type {
  DbKind,
  Protocol,
} from "@/components/library-table/labels";
import {
  DatabasePresetRecordSchema,
  WorkloadPresetRecordSchema,
  TestPresetRecordSchema,
  type DatabasePresetRecord,
  type WorkloadPresetRecord,
  type TestPresetRecord,
} from "@/lib/proto/cloud/v1/models/preset_pb";
import { TestSchema } from "@/lib/proto/cloud/v1/domain/test_pb";
import { EntitySortField } from "@/lib/proto/cloud/v1/common/entity_pb";
import {
  ListDatabasePresetsRequest_Sort_Kind,
  ListWorkloadPresetsRequest_Sort_Kind,
  ListTestPresetsRequest_Sort_Kind,
} from "@/lib/proto/cloud/v1/api/preset_pb";
import { FavoriteKind } from "@/lib/proto/cloud/v1/common/favorite_pb";
import {
  databasePresetClient,
  workloadPresetClient,
  testPresetClient,
  favoriteClient,
} from "@/services/client";
import { resolveTenantId } from "@/services/tenant";
import { dbKindLabelFromJson, dbKindProto, protocolLabelFromJson, protocolProto } from "@/services/enums";
import {
  databaseProtoToVM,
  databaseVMToProto,
  workloadProtoToVM,
  workloadVMToProto,
} from "@/services/domainMappers";

/**
 * A single database preset, flattened for the picker pane. `database` is the
 * fully-typed DatabaseVM the settings pane is pre-filled from (the source of
 * truth that gets Patched), so picking a preset seeds the form realistically.
 */
export interface DatabasePresetVM {
  /** entity.id — stable preset id. */
  id: string;
  /** entity.name — picker title. */
  name: string;
  /** entity.description — short blurb on the picker row. */
  description: string;
  /** is_system — a platform-seeded (read-only) preset vs a tenant one. */
  isSystem: boolean;
  /** the typed domain.Database this preset seeds the settings pane with. */
  database: DatabaseVM;
}

// WORKLOAD-PRESET CORRESPONDENCE — one preset maps to one
// cloud.v1.models.WorkloadPresetRecord:
//
//   WorkloadPresetVM field   WorkloadPresetRecord field
//   ----------------------   ----------------------------------------------
//   id                       entity.id
//   name                     entity.name
//   description              entity.description
//   isSystem                 is_system  (platform-seeded, read-only)
//   workload                 workload   (cloud.v1.domain.Workload, typed)
//
// The real provider issues cloud.v1.api.WorkloadPresetService.ListWorkloadPresets
// and maps the proto WorkloadPresetRecord onto the flat WorkloadVM the wizard's
// Workload step edits.

/**
 * A single workload preset, flattened for the Workload step's preset pane.
 * `workload` is the fully-typed WorkloadVM the parameters pane is seeded from
 * (the source of truth that gets Patched), so picking a preset pre-fills the
 * load realistically — exactly like DatabasePresetVM does for the DB step.
 */
export interface WorkloadPresetVM {
  /** entity.id — stable preset id. */
  id: string;
  /** entity.name — picker title. */
  name: string;
  /** entity.description — short blurb on the picker row. */
  description: string;
  /** is_system — a platform-seeded (read-only) preset vs a tenant one. */
  isSystem: boolean;
  /** the typed domain.Workload this preset seeds the parameters pane with. */
  workload: WorkloadVM;
}

// =====================================================================
// Database-preset CRUD surface (the create / edit / detail authoring pages).
//
// One DatabasePresetDetail is the full cloud.v1.models.DatabasePresetRecord the
// detail page renders read-only; DatabasePresetInput is the editable slice the
// create/edit form submits (it becomes the `preset` of a Create/Update request —
// the server assigns entity.id / tenant_id / timings / author / is_system).
//
//   DatabasePresetDetail field   DatabasePresetRecord field
//   --------------------------   ----------------------------------------------
//   id / name / description      entity{.id,.name,.description}
//   tags                         entity.labels (or tags)
//   authorId                     entity.author_id
//   isSystem                     is_system  (platform-seeded, read-only)
//   createdAt / updatedAt        entity.timings{.created_at,.updated_at}
//   database                     database   (cloud.v1.domain.Database, typed)
// =====================================================================

/** Full database-preset record for the detail page (read-only projection). */
export interface DatabasePresetDetail {
  id: string;
  name: string;
  description: string;
  /** entity.labels — free-form key/value tags. */
  tags: Record<string, string>;
  authorId: string;
  isSystem: boolean;
  createdAt: string;
  updatedAt: string;
  /** the typed domain.Database this preset carries. */
  database: DatabaseVM;
}

/**
 * Editable slice the create/edit form submits — the `preset` half of a
 * CreateDatabasePreset / UpdateDatabasePreset request. The server owns id /
 * tenant_id / author / timings / is_system, so they are NOT in the input.
 */
export interface DatabasePresetInput {
  name: string;
  description: string;
  tags: Record<string, string>;
  database: DatabaseVM;
}

// =====================================================================
// Workload-preset CRUD surface (the create / edit / detail authoring pages).
//
// One WorkloadPresetDetail is the full cloud.v1.models.WorkloadPresetRecord the
// detail page renders read-only; WorkloadPresetInput is the editable slice the
// create/edit form submits (it becomes the `preset` of a Create/Update request —
// the server assigns entity.id / tenant_id / timings / author / is_system).
//
//   WorkloadPresetDetail field   WorkloadPresetRecord field
//   --------------------------   ----------------------------------------------
//   id / name / description      entity{.id,.name,.description}
//   tags                         entity.labels (or tags)
//   authorId                     entity.author_id
//   isSystem                     is_system  (platform-seeded, read-only)
//   createdAt / updatedAt        entity.timings{.created_at,.updated_at}
//   workload                     workload   (cloud.v1.domain.Workload, typed)
// =====================================================================

/** Full workload-preset record for the detail page (read-only projection). */
export interface WorkloadPresetDetail {
  id: string;
  name: string;
  description: string;
  /** entity.labels — free-form key/value tags. */
  tags: Record<string, string>;
  authorId: string;
  isSystem: boolean;
  createdAt: string;
  updatedAt: string;
  /** the typed domain.Workload this preset carries. */
  workload: WorkloadVM;
}

/**
 * Editable slice the create/edit form submits — the `preset` half of a
 * CreateWorkloadPreset / UpdateWorkloadPreset request. The server owns id /
 * tenant_id / author / timings / is_system, so they are NOT in the input.
 */
export interface WorkloadPresetInput {
  name: string;
  description: string;
  tags: Record<string, string>;
  workload: WorkloadVM;
}

// =====================================================================
// Test-preset CRUD surface (the create / edit / detail authoring pages).
//
// One TestPresetDetail is the full cloud.v1.models.TestPresetRecord the detail
// page renders read-only; TestPresetInput is the editable slice the create/edit
// form submits (it becomes the `preset` of a Create/Update request — the server
// assigns entity.id / tenant_id / timings / author / is_system). A Test is the
// COMBINED cloud.v1.domain.Test = a database (domain.Database) + a workload
// (domain.Workload) + tags.
//
//   TestPresetDetail field   TestPresetRecord field
//   --------------------     ----------------------------------------------
//   id / name / description  entity{.id,.name,.description}
//   tags                     entity.labels (or tags)
//   authorId                 entity.author_id
//   isSystem                 is_system  (platform-seeded, read-only)
//   createdAt / updatedAt    entity.timings{.created_at,.updated_at}
//   database                 test.database  (cloud.v1.domain.Database, typed)
//   workload                 test.workload  (cloud.v1.domain.Workload, typed)
// =====================================================================

/** Full test-preset record for the detail page (read-only projection). */
export interface TestPresetDetail {
  id: string;
  name: string;
  description: string;
  /** entity.labels — free-form key/value tags. */
  tags: Record<string, string>;
  authorId: string;
  isSystem: boolean;
  createdAt: string;
  updatedAt: string;
  /** the typed domain.Database half of the test. */
  database: DatabaseVM;
  /** the typed domain.Workload half of the test. */
  workload: WorkloadVM;
}

/**
 * Editable slice the create/edit form submits — the `preset` half of a
 * CreateTestPreset / UpdateTestPreset request. The server owns id / tenant_id /
 * author / timings / is_system, so they are NOT in the input.
 */
export interface TestPresetInput {
  name: string;
  description: string;
  tags: Record<string, string>;
  database: DatabaseVM;
  workload: WorkloadVM;
}

// =====================================================================
// Library list/table surface (the Database / Workload / Test preset pages).
//
// The wizard panes above consume the WHOLE catalog for one engine. The Library
// PAGES instead present a paged, filterable, sortable TABLE per preset kind,
// oriented to the matching List RPC. They use their OWN flat row VMs (below),
// mapped from the record's denormalized Summary + Entity, so the table stays
// decoupled from the wizard's typed DatabaseVM / WorkloadVM.
//
// PROTO CORRESPONDENCE (rows) — every field maps to a real record field:
//   * id/name/author/created/updated  -> entity{.id,.name,.author_id,.timings}
//   * isSystem                         -> is_system
//   * dbKind/version/external          -> DatabasePresetRecord.Summary
//   * protocol/stroppyVersion/script   -> WorkloadPresetRecord.Summary
//   * dbKind/protocol/stroppyVersion   -> TestPresetRecord.Summary
// =====================================================================

/** Sort target for a preset list — common Entity column or table Kind. */
export type PresetSortField =
  | "name"
  | "created_at"
  | "updated_at"
  | "author_id"
  | "db_kind" // database + test presets
  | "is_system" // database presets
  | "protocol" // workload + test presets
  | "stroppy_version"; // workload + test presets

/** Shared list query slice across the three preset List RPCs. */
export interface PresetListQuery {
  /** filter.search — free text over name + description. */
  search?: string;
  /** filter.favorites_only — only rows the caller has favorited (EntityFilter). */
  favoritesOnly?: boolean;
  /** db_kinds[] — engine facet (database + test presets). */
  dbKinds?: Exclude<DbKind, "">[];
  /** protocols[] — wire-protocol facet (workload + test presets). */
  protocols?: Exclude<Protocol, "">[];
  /** stroppy_versions[] — version facet (workload + test presets). */
  stroppyVersions?: string[];
  /** is_system — optional bool facet (unset = all). */
  isSystem?: boolean;
  /** filter.created_after / created_before — created window (ISO). */
  createdAfter?: string;
  createdBefore?: string;
  /** filter.updated_after / updated_before — updated window (ISO). */
  updatedAfter?: string;
  updatedBefore?: string;
  /** sort target + direction. */
  sort?: PresetSortField;
  desc?: boolean;
  /** page.size / page.token. */
  pageSize?: number;
  pageToken?: string;
}

/** A page of preset rows + the cursor for the next page. */
export interface PresetPage<T> {
  rows: T[];
  nextPageToken: string;
}

/** Row VM for the Database Presets table — DatabasePresetRecord. */
export interface DatabasePresetRow {
  id: string;
  name: string;
  description: string;
  authorId: string;
  isSystem: boolean;
  /** entity.is_favorite — the caller's computed favorite flag. */
  isFavorite: boolean;
  createdAt: string;
  updatedAt: string;
  /** Summary.db_kind, lower-cased. */
  dbKind: DbKind;
  /** Summary.version (self-deploy engine version), "" when unset. */
  version: string;
  /** Summary.external — targets an already-running database. */
  external: boolean;
}

/** Row VM for the Workload Presets table — WorkloadPresetRecord. */
export interface WorkloadPresetRow {
  id: string;
  name: string;
  description: string;
  authorId: string;
  isSystem: boolean;
  /** entity.is_favorite — the caller's computed favorite flag. */
  isFavorite: boolean;
  createdAt: string;
  updatedAt: string;
  /** Summary.protocol, lower-cased. */
  protocol: Protocol;
  /** Summary.stroppy_version, "" when unset. */
  stroppyVersion: string;
  /** Summary.script label, "" when unset. */
  script: string;
}

/** Row VM for the Test Presets table — TestPresetRecord (db + workload combo). */
export interface TestPresetRow {
  id: string;
  name: string;
  description: string;
  authorId: string;
  isSystem: boolean;
  /** entity.is_favorite — the caller's computed favorite flag. */
  isFavorite: boolean;
  createdAt: string;
  updatedAt: string;
  /** Summary.db_kind, lower-cased. */
  dbKind: DbKind;
  /** Summary.protocol, lower-cased. */
  protocol: Protocol;
  /** Summary.stroppy_version, "" when unset. */
  stroppyVersion: string;
}

/**
 * The three favoritable + clonable preset tables. Each maps to its own
 * Service (Database/Workload/Test PresetService) and its own FAVORITE_KIND_*:
 *   "database" -> DatabasePresetService + FAVORITE_KIND_DATABASE_PRESET
 *   "workload" -> WorkloadPresetService + FAVORITE_KIND_WORKLOAD_PRESET
 *   "test"     -> TestPresetService     + FAVORITE_KIND_TEST_PRESET
 */
export type PresetKind = "database" | "workload" | "test";

/**
 * PresetAction enumerates the per-row Actions (⋯) menu items, each mapping to a
 * real preset Service RPC or an in-app route — no speculative operations:
 *   * "view"      -> route to the preset (no RPC; always available).
 *   * "use"       -> route to /runs/new?preset=:id (seed the wizard; always).
 *   * "edit"      -> Update<Kind>Preset (NOT allowed for system presets).
 *   * "duplicate" -> Clone<Kind>Preset (always available — clones any preset).
 *   * "delete"    -> Delete<Kind>Preset (NOT allowed for system presets;
 *                    operator+ role required).
 */
export type PresetAction = "view" | "use" | "edit" | "duplicate" | "delete";

/**
 * PresetProvider abstracts the DatabasePresetService list call so the Database
 * step never imports mock or backend specifics. Keyed by tenant SLUG to match
 * runs.ts / wizard.ts; the real provider resolves slug -> tenant_id at call
 * time. Listing is scoped to one engine — the pane only shows presets for the
 * engine the user already chose.
 */
export interface PresetProvider {
  /** ListDatabasePresets filtered to one engine -> the presets for that engine. */
  listDatabasePresets(tenantSlug: string, engine: EngineKind): Promise<DatabasePresetVM[]>;
  /**
   * ListWorkloadPresets -> the reusable stroppy workload presets the Workload
   * step seeds its parameters from. Workload presets are provider/engine
   * agnostic (a script + k6 execution + data params), so unlike databases they
   * are NOT scoped by engine — the whole catalog is offered.
   */
  listWorkloadPresets(tenantSlug: string): Promise<WorkloadPresetVM[]>;

  // --- Database-preset CRUD (create / edit / detail authoring pages). ------
  /** GetDatabasePreset -> the full record the detail/edit pages render. */
  getDatabasePreset(tenantSlug: string, id: string): Promise<DatabasePresetDetail>;
  /**
   * CreateDatabasePreset(tenant_id, preset) -> the new preset's id. The server
   * assigns entity.id / tenant_id / timings / author and forces is_system=false.
   */
  createDatabasePreset(tenantSlug: string, input: DatabasePresetInput): Promise<string>;
  /**
   * UpdateDatabasePreset(tenant_id, preset) -> wholesale replace. preset.entity.id
   * selects the row; system presets cannot be edited (gated in the UI).
   */
  updateDatabasePreset(
    tenantSlug: string,
    id: string,
    input: DatabasePresetInput,
  ): Promise<void>;

  // --- Workload-preset CRUD (create / edit / detail authoring pages). ------
  /** GetWorkloadPreset -> the full record the detail/edit pages render. */
  getWorkloadPreset(tenantSlug: string, id: string): Promise<WorkloadPresetDetail>;
  /**
   * CreateWorkloadPreset(tenant_id, preset) -> the new preset's id. The server
   * assigns entity.id / tenant_id / timings / author and forces is_system=false.
   */
  createWorkloadPreset(tenantSlug: string, input: WorkloadPresetInput): Promise<string>;
  /**
   * UpdateWorkloadPreset(tenant_id, preset) -> wholesale replace. preset.entity.id
   * selects the row; system presets cannot be edited (gated in the UI).
   */
  updateWorkloadPreset(
    tenantSlug: string,
    id: string,
    input: WorkloadPresetInput,
  ): Promise<void>;

  // --- Test-preset CRUD (create / edit / detail authoring pages). ---------
  /** GetTestPreset -> the full record (db + workload) the detail/edit pages render. */
  getTestPreset(tenantSlug: string, id: string): Promise<TestPresetDetail>;
  /**
   * CreateTestPreset(tenant_id, preset) -> the new preset's id. The server
   * assigns entity.id / tenant_id / timings / author and forces is_system=false.
   */
  createTestPreset(tenantSlug: string, input: TestPresetInput): Promise<string>;
  /**
   * UpdateTestPreset(tenant_id, preset) -> wholesale replace. preset.entity.id
   * selects the row; system presets cannot be edited (gated in the UI).
   */
  updateTestPreset(
    tenantSlug: string,
    id: string,
    input: TestPresetInput,
  ): Promise<void>;

  // --- Library table surface (paged + filtered + sorted). -----------------
  /** ListDatabasePresets oriented to the Database Presets table. */
  listDatabasePresetRows(
    tenantSlug: string,
    query: PresetListQuery,
  ): Promise<PresetPage<DatabasePresetRow>>;
  /** ListWorkloadPresets oriented to the Workload Presets table. */
  listWorkloadPresetRows(
    tenantSlug: string,
    query: PresetListQuery,
  ): Promise<PresetPage<WorkloadPresetRow>>;
  /** ListTestPresets oriented to the Test Presets table. */
  listTestPresetRows(
    tenantSlug: string,
    query: PresetListQuery,
  ): Promise<PresetPage<TestPresetRow>>;

  // --- Per-row mutations (Actions menu + favorite star). ------------------
  /**
   * Toggle the caller's favorite relation on a preset row -> FavoriteService
   * AddFavorite / RemoveFavorite with the kind-specific FAVORITE_KIND_* and the
   * row id as target_id.
   */
  setPresetFavorite(
    tenantSlug: string,
    kind: PresetKind,
    id: string,
    favorite: boolean,
  ): Promise<void>;
  /**
   * Duplicate a preset -> Clone<Kind>Preset (tenant_id, id, name). Returns the
   * new preset id. Works for system AND tenant presets.
   */
  clonePreset(
    tenantSlug: string,
    kind: PresetKind,
    id: string,
    name: string,
  ): Promise<string>;
  /**
   * Delete a tenant preset -> Delete<Kind>Preset. System presets cannot be
   * deleted (the page disables the action); operator+ role required.
   */
  deletePreset(
    tenantSlug: string,
    kind: PresetKind,
    id: string,
  ): Promise<void>;
}

// --- Real backend provider ---------------------------------------------------
//
// Wires the three preset Services + FavoriteService and maps proto <-> the flat
// VMs via domainMappers. Every method resolves the tenant slug -> tenant_id
// first, exactly like packages.ts / suites.ts.

// PresetKind -> the FavoriteService kind for that preset table.
const PRESET_FAVORITE_KIND: Record<PresetKind, FavoriteKind> = {
  database: FavoriteKind.DATABASE_PRESET,
  workload: FavoriteKind.WORKLOAD_PRESET,
  test: FavoriteKind.TEST_PRESET,
};

// A proto Timestamp -> ISO string ("" when unset).
function timingsIso(ts: Timestamp | undefined): string {
  return ts ? timestampDate(ts).toISOString() : "";
}

// Map the shared EntityFilter slice of a PresetListQuery onto the wire filter.
function entityFilter(query: PresetListQuery) {
  return {
    search: query.search,
    favoritesOnly: query.favoritesOnly,
    createdAfter: query.createdAfter
      ? timestampFromDate(new Date(query.createdAfter))
      : undefined,
    createdBefore: query.createdBefore
      ? timestampFromDate(new Date(query.createdBefore))
      : undefined,
    updatedAfter: query.updatedAfter
      ? timestampFromDate(new Date(query.updatedAfter))
      : undefined,
    updatedBefore: query.updatedBefore
      ? timestampFromDate(new Date(query.updatedBefore))
      : undefined,
  };
}

// Common Entity sort column for a PresetSortField, or UNSPECIFIED when the
// field is a kind-specific (table) column.
function entitySortField(f?: PresetSortField): EntitySortField {
  switch (f) {
    case "name":
      return EntitySortField.NAME;
    case "created_at":
      return EntitySortField.CREATED_AT;
    case "updated_at":
      return EntitySortField.UPDATED_AT;
    case "author_id":
      return EntitySortField.AUTHOR_ID;
    default:
      return EntitySortField.UNSPECIFIED;
  }
}

// Sort builders per table — the `by` oneof is entity(column) XOR kind(column).
function dbPresetSort(query: PresetListQuery) {
  const desc = query.desc ?? false;
  switch (query.sort) {
    case "db_kind":
      return { by: { case: "kind" as const, value: ListDatabasePresetsRequest_Sort_Kind.DB_KIND }, desc };
    case "is_system":
      return { by: { case: "kind" as const, value: ListDatabasePresetsRequest_Sort_Kind.IS_SYSTEM }, desc };
    default:
      return { by: { case: "entity" as const, value: entitySortField(query.sort) }, desc };
  }
}
function workloadPresetSort(query: PresetListQuery) {
  const desc = query.desc ?? false;
  switch (query.sort) {
    case "protocol":
      return { by: { case: "kind" as const, value: ListWorkloadPresetsRequest_Sort_Kind.PROTOCOL }, desc };
    case "stroppy_version":
      return { by: { case: "kind" as const, value: ListWorkloadPresetsRequest_Sort_Kind.STROPPY_VERSION }, desc };
    case "is_system":
      return { by: { case: "kind" as const, value: ListWorkloadPresetsRequest_Sort_Kind.IS_SYSTEM }, desc };
    default:
      return { by: { case: "entity" as const, value: entitySortField(query.sort) }, desc };
  }
}
function testPresetSort(query: PresetListQuery) {
  const desc = query.desc ?? false;
  switch (query.sort) {
    case "db_kind":
      return { by: { case: "kind" as const, value: ListTestPresetsRequest_Sort_Kind.DB_KIND }, desc };
    case "protocol":
      return { by: { case: "kind" as const, value: ListTestPresetsRequest_Sort_Kind.PROTOCOL }, desc };
    case "stroppy_version":
      return { by: { case: "kind" as const, value: ListTestPresetsRequest_Sort_Kind.STROPPY_VERSION }, desc };
    case "is_system":
      return { by: { case: "kind" as const, value: ListTestPresetsRequest_Sort_Kind.IS_SYSTEM }, desc };
    default:
      return { by: { case: "entity" as const, value: entitySortField(query.sort) }, desc };
  }
}

// --- record -> row VM (entity + denormalized Summary, timestamps as ISO) -----

function dbPresetRecordToRow(rec: DatabasePresetRecord): DatabasePresetRow {
  const j = toJson(DatabasePresetRecordSchema, rec) as {
    entity?: {
      id?: string;
      name?: string;
      description?: string;
      authorId?: string;
      isFavorite?: boolean;
      timings?: { createdAt?: string; updatedAt?: string };
    };
    isSystem?: boolean;
    summary?: { dbKind?: string; version?: string; external?: boolean };
  };
  const e = j.entity ?? {};
  const s = j.summary ?? {};
  return {
    id: e.id ?? "",
    name: e.name ?? "",
    description: e.description ?? "",
    authorId: e.authorId ?? "",
    isSystem: j.isSystem ?? false,
    isFavorite: e.isFavorite ?? false,
    createdAt: e.timings?.createdAt ?? "",
    updatedAt: e.timings?.updatedAt ?? "",
    dbKind: dbKindLabelFromJson(s.dbKind),
    version: s.version ?? "",
    external: s.external ?? false,
  };
}

function workloadPresetRecordToRow(rec: WorkloadPresetRecord): WorkloadPresetRow {
  const j = toJson(WorkloadPresetRecordSchema, rec) as {
    entity?: {
      id?: string;
      name?: string;
      description?: string;
      authorId?: string;
      isFavorite?: boolean;
      timings?: { createdAt?: string; updatedAt?: string };
    };
    isSystem?: boolean;
    summary?: { protocol?: string; stroppyVersion?: string; script?: string };
  };
  const e = j.entity ?? {};
  const s = j.summary ?? {};
  return {
    id: e.id ?? "",
    name: e.name ?? "",
    description: e.description ?? "",
    authorId: e.authorId ?? "",
    isSystem: j.isSystem ?? false,
    isFavorite: e.isFavorite ?? false,
    createdAt: e.timings?.createdAt ?? "",
    updatedAt: e.timings?.updatedAt ?? "",
    protocol: protocolLabelFromJson(s.protocol),
    stroppyVersion: s.stroppyVersion ?? "",
    script: s.script ?? "",
  };
}

function testPresetRecordToRow(rec: TestPresetRecord): TestPresetRow {
  const j = toJson(TestPresetRecordSchema, rec) as {
    entity?: {
      id?: string;
      name?: string;
      description?: string;
      authorId?: string;
      isFavorite?: boolean;
      timings?: { createdAt?: string; updatedAt?: string };
    };
    isSystem?: boolean;
    summary?: { dbKind?: string; protocol?: string; stroppyVersion?: string };
  };
  const e = j.entity ?? {};
  const s = j.summary ?? {};
  return {
    id: e.id ?? "",
    name: e.name ?? "",
    description: e.description ?? "",
    authorId: e.authorId ?? "",
    isSystem: j.isSystem ?? false,
    isFavorite: e.isFavorite ?? false,
    createdAt: e.timings?.createdAt ?? "",
    updatedAt: e.timings?.updatedAt ?? "",
    dbKind: dbKindLabelFromJson(s.dbKind),
    protocol: protocolLabelFromJson(s.protocol),
    stroppyVersion: s.stroppyVersion ?? "",
  };
}

const realPresetProvider: PresetProvider = {
  async listDatabasePresets(tenantSlug, engine) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { presets } = await databasePresetClient.listDatabasePresets({
      tenantId,
      dbKinds: [ENGINE_TO_KIND[engine]],
      page: { size: 50 },
    });
    return presets.map((p) => ({
      id: p.entity?.id ?? "",
      name: p.entity?.name ?? "",
      description: p.entity?.description ?? "",
      isSystem: p.isSystem,
      database: databaseProtoToVM(p.database),
    }));
  },

  async listWorkloadPresets(tenantSlug) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { presets } = await workloadPresetClient.listWorkloadPresets({
      tenantId,
      page: { size: 50 },
    });
    return presets.map((p) => ({
      id: p.entity?.id ?? "",
      name: p.entity?.name ?? "",
      description: p.entity?.description ?? "",
      isSystem: p.isSystem,
      workload: workloadProtoToVM(p.workload),
    }));
  },

  // --- Database-preset CRUD. ---------------------------------------------
  async getDatabasePreset(tenantSlug, id) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { preset } = await databasePresetClient.getDatabasePreset({ tenantId, id });
    return {
      id: preset?.entity?.id ?? "",
      name: preset?.entity?.name ?? "",
      description: preset?.entity?.description ?? "",
      tags: preset?.database?.tags?.labels ?? {},
      authorId: preset?.entity?.authorId ?? "",
      isSystem: preset?.isSystem ?? false,
      createdAt: timingsIso(preset?.entity?.timings?.createdAt),
      updatedAt: timingsIso(preset?.entity?.timings?.updatedAt),
      database: databaseProtoToVM(preset?.database),
    };
  },
  async createDatabasePreset(tenantSlug, input) {
    const tenantId = await resolveTenantId(tenantSlug);
    const database = databaseVMToProto(input.database);
    database.tags = { $typeName: "cloud.v1.common.Tags", tags: [], labels: { ...input.tags } };
    const { preset } = await databasePresetClient.createDatabasePreset({
      tenantId,
      preset: create(DatabasePresetRecordSchema, {
        entity: { name: input.name, description: input.description },
        database,
      }),
    });
    return preset?.entity?.id ?? "";
  },
  async updateDatabasePreset(tenantSlug, id, input) {
    const tenantId = await resolveTenantId(tenantSlug);
    const database = databaseVMToProto(input.database);
    database.tags = { $typeName: "cloud.v1.common.Tags", tags: [], labels: { ...input.tags } };
    await databasePresetClient.updateDatabasePreset({
      tenantId,
      preset: create(DatabasePresetRecordSchema, {
        entity: { id, name: input.name, description: input.description },
        database,
      }),
    });
  },

  // --- Workload-preset CRUD. ---------------------------------------------
  async getWorkloadPreset(tenantSlug, id) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { preset } = await workloadPresetClient.getWorkloadPreset({ tenantId, id });
    return {
      id: preset?.entity?.id ?? "",
      name: preset?.entity?.name ?? "",
      description: preset?.entity?.description ?? "",
      tags: preset?.workload?.tags?.labels ?? {},
      authorId: preset?.entity?.authorId ?? "",
      isSystem: preset?.isSystem ?? false,
      createdAt: timingsIso(preset?.entity?.timings?.createdAt),
      updatedAt: timingsIso(preset?.entity?.timings?.updatedAt),
      workload: workloadProtoToVM(preset?.workload),
    };
  },
  async createWorkloadPreset(tenantSlug, input) {
    const tenantId = await resolveTenantId(tenantSlug);
    const workload = workloadVMToProto(input.workload);
    workload.tags = { $typeName: "cloud.v1.common.Tags", tags: [], labels: { ...input.tags } };
    const { preset } = await workloadPresetClient.createWorkloadPreset({
      tenantId,
      preset: create(WorkloadPresetRecordSchema, {
        entity: { name: input.name, description: input.description },
        workload,
      }),
    });
    return preset?.entity?.id ?? "";
  },
  async updateWorkloadPreset(tenantSlug, id, input) {
    const tenantId = await resolveTenantId(tenantSlug);
    const workload = workloadVMToProto(input.workload);
    workload.tags = { $typeName: "cloud.v1.common.Tags", tags: [], labels: { ...input.tags } };
    await workloadPresetClient.updateWorkloadPreset({
      tenantId,
      preset: create(WorkloadPresetRecordSchema, {
        entity: { id, name: input.name, description: input.description },
        workload,
      }),
    });
  },

  // --- Test-preset CRUD. -------------------------------------------------
  async getTestPreset(tenantSlug, id) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { preset } = await testPresetClient.getTestPreset({ tenantId, id });
    return {
      id: preset?.entity?.id ?? "",
      name: preset?.entity?.name ?? "",
      description: preset?.entity?.description ?? "",
      tags: preset?.test?.tags?.labels ?? {},
      authorId: preset?.entity?.authorId ?? "",
      isSystem: preset?.isSystem ?? false,
      createdAt: timingsIso(preset?.entity?.timings?.createdAt),
      updatedAt: timingsIso(preset?.entity?.timings?.updatedAt),
      database: databaseProtoToVM(preset?.test?.database),
      workload: workloadProtoToVM(preset?.test?.workload),
    };
  },
  async createTestPreset(tenantSlug, input) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { preset } = await testPresetClient.createTestPreset({
      tenantId,
      preset: create(TestPresetRecordSchema, {
        entity: { name: input.name, description: input.description },
        test: create(TestSchema, {
          database: databaseVMToProto(input.database),
          workload: workloadVMToProto(input.workload),
          tags: { tags: [], labels: { ...input.tags } },
        }),
      }),
    });
    return preset?.entity?.id ?? "";
  },
  async updateTestPreset(tenantSlug, id, input) {
    const tenantId = await resolveTenantId(tenantSlug);
    await testPresetClient.updateTestPreset({
      tenantId,
      preset: create(TestPresetRecordSchema, {
        entity: { id, name: input.name, description: input.description },
        test: create(TestSchema, {
          database: databaseVMToProto(input.database),
          workload: workloadVMToProto(input.workload),
          tags: { tags: [], labels: { ...input.tags } },
        }),
      }),
    });
  },

  // --- Library table surface. --------------------------------------------
  async listDatabasePresetRows(tenantSlug, query) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { presets, nextPageToken } = await databasePresetClient.listDatabasePresets({
      tenantId,
      filter: entityFilter(query),
      dbKinds: query.dbKinds?.map(dbKindProto) ?? [],
      isSystem: query.isSystem,
      sort: dbPresetSort(query),
      page: { size: query.pageSize ?? 0, token: query.pageToken ?? "" },
    });
    return { rows: presets.map(dbPresetRecordToRow), nextPageToken };
  },
  async listWorkloadPresetRows(tenantSlug, query) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { presets, nextPageToken } = await workloadPresetClient.listWorkloadPresets({
      tenantId,
      filter: entityFilter(query),
      protocols: query.protocols?.map(protocolProto) ?? [],
      stroppyVersions: query.stroppyVersions ?? [],
      isSystem: query.isSystem,
      sort: workloadPresetSort(query),
      page: { size: query.pageSize ?? 0, token: query.pageToken ?? "" },
    });
    return { rows: presets.map(workloadPresetRecordToRow), nextPageToken };
  },
  async listTestPresetRows(tenantSlug, query) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { presets, nextPageToken } = await testPresetClient.listTestPresets({
      tenantId,
      filter: entityFilter(query),
      dbKinds: query.dbKinds?.map(dbKindProto) ?? [],
      protocols: query.protocols?.map(protocolProto) ?? [],
      stroppyVersions: query.stroppyVersions ?? [],
      isSystem: query.isSystem,
      sort: testPresetSort(query),
      page: { size: query.pageSize ?? 0, token: query.pageToken ?? "" },
    });
    return { rows: presets.map(testPresetRecordToRow), nextPageToken };
  },

  // --- Per-row mutations. ------------------------------------------------
  async setPresetFavorite(tenantSlug, kind, id, favorite) {
    const tenantId = await resolveTenantId(tenantSlug);
    const kindEnum = PRESET_FAVORITE_KIND[kind];
    if (favorite) {
      await favoriteClient.addFavorite({ tenantId, kind: kindEnum, targetId: id });
    } else {
      await favoriteClient.removeFavorite({ tenantId, kind: kindEnum, targetId: id });
    }
  },
  async clonePreset(tenantSlug, kind, id, name) {
    const tenantId = await resolveTenantId(tenantSlug);
    switch (kind) {
      case "database": {
        const { preset } = await databasePresetClient.cloneDatabasePreset({ tenantId, id, name });
        return preset?.entity?.id ?? "";
      }
      case "workload": {
        const { preset } = await workloadPresetClient.cloneWorkloadPreset({ tenantId, id, name });
        return preset?.entity?.id ?? "";
      }
      case "test": {
        const { preset } = await testPresetClient.cloneTestPreset({ tenantId, id, name });
        return preset?.entity?.id ?? "";
      }
    }
  },
  async deletePreset(tenantSlug, kind, id) {
    const tenantId = await resolveTenantId(tenantSlug);
    switch (kind) {
      case "database":
        await databasePresetClient.deleteDatabasePreset({ tenantId, id });
        return;
      case "workload":
        await workloadPresetClient.deleteWorkloadPreset({ tenantId, id });
        return;
      case "test":
        await testPresetClient.deleteTestPreset({ tenantId, id });
        return;
    }
  },
};

// --- Provider injection ------------------------------------------------------
//
// Defaults to the real backend. The mock (and ONLY the mock) overrides it via
// setPresetProvider() from the single gate in main.tsx.

let active: PresetProvider = realPresetProvider;

export function setPresetProvider(provider: PresetProvider): void {
  active = provider;
}

export function getPresetProvider(): PresetProvider {
  return active;
}
