// Test-runs data surface for the rebuilt shell.
//
// This is the contract the Test Runs page talks to. It mirrors the dashboard.ts
// provider pattern: the page depends ONLY on this interface, never on mock or
// backend specifics. The real implementation (TODO below) will call the
// connectrpc `testRunClient.listTestRuns` (cloud.v1.api.TestRunService, see
// src/lib/proto/cloud/v1/api/test_run_pb.ts) and map the proto payload onto the
// flat view-model types here. For now the single mock gate in main.tsx can
// inject canned data via setRunsProvider() so the page renders standalone.
//
// The view-model is intentionally flat (no proto Message instances) so the runs
// UI stays decoupled from the wire format and the mock stays trivial.
//
// SCOPE — the page is built STRICTLY around what cloud.v1.api.ListTestRunsRequest
// can express. We now wire the slice that maps to a visible column / useful
// per-column header filter:
//   * filter.search            -> free-text name/description search (?q=)
//   * statuses[]               -> lifecycle status facet (?status=, repeatable)
//   * db_kinds[]               -> database-kind facet (?db=, repeatable)
//   * stroppy_versions[]       -> stroppy build facet (?sv=, repeatable)
//   * protocols[]              -> workload wire-protocol facet (?proto=, repeatable)
//   * triggers[]               -> how the run was triggered (?trigger=, repeatable)
//   * started_after/before     -> started time window (?sa=, ?sb=, ISO date)
//   * finished_after/before    -> finished time window (?fa=, ?fb=, ISO date)
//   * sort {entity|kind,desc}  -> column ordering (?sort=, ?dir=)
//   * page {size, token}       -> page size + opaque next-page token (?size=, ?page=)
//
// Additional ListTestRunsRequest fields wired into the page (column-header
// popovers + a small inline checkbox row above the table):
//   * filter.author_ids[]      -> Author-column facet (?author=, repeatable)
//   * progress_min / progress_max -> progress window on the Status header (?pmin=, ?pmax=)
//   * duration_min / duration_max -> duration window on the Time header (?dmin=, ?dmax=, seconds)
//   * standalone               -> Standalone checkbox: only non-suite runs (?sa_only=true)
//   * filter.favorites_only    -> Favorites checkbox (?fav=1)
//   * filter.include_deleted   -> Deleted checkbox: include soft-deleted rows (?del=1)
//
// STILL UNWIRED: suite_cell_ids (suite scoping is a separate surface),
// filter.updated_after/before, filter.created_after/before and filter.ids
// (no visible column), providers[] / *_preset_ids[] (no UI surface).

import type { RunStatus } from "@/services/dashboard";
import { testRunClient } from "@/services/client";
import { resolveTenantId } from "@/services/tenant";

export type { RunStatus };

/**
 * Database kind, mapped from cloud.v1.domain.Database.Kind. Lower-cased,
 * UNSPECIFIED collapsed to "" so the column/filter can hide it.
 */
export type DbKind =
  | ""
  | "postgres"
  | "mysql"
  | "mariadb"
  | "ydb"
  | "ydb_managed"
  | "cockroach"
  | "picodata"
  | "external";

/** All selectable db kinds (UNSPECIFIED excluded), for the facet control. */
export const DB_KINDS: Exclude<DbKind, "">[] = [
  "postgres",
  "mysql",
  "mariadb",
  "ydb",
  "ydb_managed",
  "cockroach",
  "picodata",
  "external",
];

/** All selectable statuses for the facet control (UNSPECIFIED excluded). */
export const RUN_STATUSES: RunStatus[] = [
  "pending",
  "running",
  "cancelling",
  "completed",
  "failed",
  "cancelled",
];

/**
 * Workload wire protocol, mapped from cloud.v1.domain.Workload.Protocol.
 * Lower-cased, UNSPECIFIED collapsed to "" so the column/filter can hide it.
 */
export type Protocol =
  | ""
  | "pg"
  | "mysql"
  | "picodata"
  | "ydb_grpc"
  | "ydb_grpcs"
  | "cockroach";

/** All selectable protocols (UNSPECIFIED excluded), for the facet control. */
export const PROTOCOLS: Exclude<Protocol, "">[] = [
  "pg",
  "mysql",
  "picodata",
  "ydb_grpc",
  "ydb_grpcs",
  "cockroach",
];

/**
 * How the run was triggered, mapped from cloud.v1.common.Trigger. Lower-cased,
 * UNSPECIFIED collapsed to "" so the column/filter can hide it.
 */
export type RunTrigger = "" | "manual" | "cron" | "api";

/** All selectable triggers (UNSPECIFIED excluded), for the facet control. */
export const TRIGGERS: Exclude<RunTrigger, "">[] = ["manual", "cron", "api"];

/**
 * Deployment provider, mapped from cloud.v1.deployment.Provider. Lower-cased,
 * UNSPECIFIED collapsed to "" so the record field can hide it. Still carried on
 * RunVM (summary.provider) though no longer exposed as a filter facet.
 */
export type DeployProvider = "" | "docker" | "yandex";

/**
 * Sortable columns. Each maps to a real ListTestRunsRequest.Sort target:
 *   name        -> Sort.entity = ENTITY_SORT_FIELD_NAME
 *   created_at  -> Sort.entity = ENTITY_SORT_FIELD_CREATED_AT
 *   status      -> Sort.kind   = KIND_STATUS
 *   db_kind     -> Sort.kind   = KIND_DB_KIND
 *   workload    -> Sort.kind   = KIND_WORKLOAD
 *   trigger     -> Sort.kind   = KIND_TRIGGER
 *   duration    -> Sort.kind   = KIND_DURATION
 *   started_at  -> Sort.kind   = KIND_STARTED_AT
 *   finished_at -> Sort.kind   = KIND_FINISHED_AT
 */
export type SortField =
  | "name"
  | "created_at"
  | "status"
  | "db_kind"
  | "workload"
  | "trigger"
  | "duration"
  | "started_at"
  | "finished_at";

/**
 * A test run, flattened from cloud.v1.models.TestRunRecord. Every field maps to
 * a REAL record field — id/name/author/createdAt from entity; status/trigger
 * from the record; the rest from the denormalized summary projection. Optional
 * fields are absent when the proto field is unset (e.g. finishedAt while
 * running).
 */
export interface RunVM {
  /** entity.id */
  id: string;
  /** entity.name */
  name: string;
  /** entity.author_id (display id; real provider may resolve to a name later). */
  authorId: string;
  /** entity.timings.created_at (ISO). */
  createdAt: string;
  /** record.status, mapped to the dashboard RunStatus union. */
  status: RunStatus;
  /** summary.db_kind, lower-cased ("" when unspecified). */
  dbKind: DbKind;
  /** summary.workload_name. */
  workload: string;
  /** summary.stroppy_version ("" when unset). */
  stroppyVersion: string;
  /** summary.workload_protocol, lower-cased ("" when unspecified). */
  protocol: Protocol;
  /** record.trigger, lower-cased ("" when unspecified). */
  trigger: RunTrigger;
  /** summary.topology_label, e.g. "PG HA x3" ("" when unset). */
  topologyLabel: string;
  /** summary.node_count (0 when unset). */
  nodeCount: number;
  /** summary.progress_pct, 0..100. */
  progressPct: number;
  /** summary.started_at (ISO); absent until the run starts. */
  startedAt?: string;
  /** summary.finished_at (ISO); absent while running. */
  finishedAt?: string;
  /** summary.duration in seconds (derived from the proto Duration); absent when unset. */
  durationSec?: number;
  /** record.suite_run_id ("" for standalone runs). */
  suiteRunId: string;
  /** summary.provider (deployment backend), lower-cased ("" when unspecified). */
  provider: DeployProvider;
  /** summary.db_preset_id ("" when the run did not use a saved db preset). */
  dbPresetId: string;
  /** summary.workload_preset_id ("" when not from a saved workload preset). */
  workloadPresetId: string;
  /** summary.test_preset_id ("" when not from a saved complete test preset). */
  testPresetId: string;
  /** whether the REQUESTING caller has favorited this run (drives favorites_only). */
  favorite: boolean;
  /** entity.timings.deleted_at is set (soft-deleted); drives include_deleted. */
  deleted: boolean;
}

/**
 * RunsQuery is the page's view of ListTestRunsRequest — only the fields we wire.
 * Empty collections / unset optionals = "facet not applied", matching the proto
 * semantics. tenant_id is supplied separately (by slug) at call time.
 */
export interface RunsQuery {
  /** filter.search — free text over name + description. */
  search?: string;
  /** statuses[] — lifecycle facet (AND-empty = not applied). */
  statuses?: RunStatus[];
  /** db_kinds[] — database-kind facet. */
  dbKinds?: Exclude<DbKind, "">[];
  /** stroppy_versions[] — stroppy build facet. */
  stroppyVersions?: string[];
  /** protocols[] — workload wire-protocol facet. */
  protocols?: Exclude<Protocol, "">[];
  /** triggers[] — run-trigger facet. */
  triggers?: Exclude<RunTrigger, "">[];
  /** started_after — keep runs started at or after this ISO time. */
  startedAfter?: string;
  /** started_before — keep runs started at or before this ISO time. */
  startedBefore?: string;
  /** finished_after — keep runs finished at or after this ISO time. */
  finishedAfter?: string;
  /** finished_before — keep runs finished at or before this ISO time. */
  finishedBefore?: string;
  /** filter.author_ids[] — restrict to runs authored by these account ids. */
  authorIds?: string[];
  /** progress_min — lower bound of the progress_pct window (0..100). */
  progressMin?: number;
  /** progress_max — upper bound of the progress_pct window (0..100). */
  progressMax?: number;
  /** duration_min — lower bound of the duration window, in SECONDS. */
  durationMinSec?: number;
  /** duration_max — upper bound of the duration window, in SECONDS. */
  durationMaxSec?: number;
  /**
   * standalone — Standalone checkbox. undefined = all runs; true = only
   * standalone runs (no suite_run_id).
   */
  standalone?: boolean;
  /** suite_run_id — scope to a single suite run's child test runs (deep-link
   * from a suite's runs table). Maps to ListTestRunsRequest.suite_run_id. */
  suiteRunId?: string;
  /** filter.favorites_only — only runs the caller has favorited. */
  favoritesOnly?: boolean;
  /** filter.include_deleted — include soft-deleted runs. */
  includeDeleted?: boolean;
  /** sort target + direction. */
  sort?: SortField;
  /** sort descending when true. */
  desc?: boolean;
  /** page.size — rows per page (0/undefined = server default). */
  pageSize?: number;
  /** page.token — opaque next-page cursor from a prior response. */
  pageToken?: string;
}

/** A page of runs + the cursor for the next page (empty when at the end). */
export interface RunsPage {
  runs: RunVM[];
  nextPageToken: string;
}

/**
 * RunFacets is the set of distinct id-keyed values present in a tenant's runs,
 * used to populate the Author column-header pick-list so it's usable without a
 * separate catalog. A real provider would source these from a faceting RPC; the
 * mock derives them from its rows.
 */
export interface RunFacets {
  authorIds: string[];
}

/**
 * RunAction enumerates the per-row operations exposed by the table's row
 * Actions menu. Every member maps to a REAL cloud.v1.api.TestRunService RPC or
 * an existing in-app route — no speculative operations:
 *   * "view"      -> in-app route /runs/:id (no RPC; always available).
 *   * "share"     -> copy an in-app /runs/:id link (no RPC; always available).
 *   * "cancel"    -> TestRunService.CancelTestRun  (running / pending only).
 *   * "rerun"     -> TestRunService.StartTestRun with source = { testRunId }
 *                    (terminal states only: completed / failed / cancelled).
 *   * "extract"   -> TestRunService.ExtractToPreset (completed only — needs a
 *                    finished spec to promote into a preset).
 *   * "delete"    -> TestRunService.DeleteTestRun  (anything NOT running).
 * Favorite toggling is intentionally NOT a RunAction menu item: it maps to the
 * cross-resource FavoriteService AddFavorite/RemoveFavorite pair.
 */
export type RunAction =
  | "view"
  | "share"
  | "cancel"
  | "rerun"
  | "extract"
  | "delete";

/**
 * actionsForStatus encodes the status-gating allow-rules. It returns, for a
 * given run lifecycle status, the set of actions whose backing RPC/route is
 * valid in that state. The Actions menu renders every action but DISABLES the
 * ones absent from this set (disabled-with-reason is friendlier than hiding):
 *
 *   status      | view | share | cancel | rerun | extract | delete
 *   ------------+------+-------+--------+-------+---------+-------
 *   pending     |  ✓   |   ✓   |   ✓    |       |         |   ✓
 *   running     |  ✓   |   ✓   |   ✓    |       |         |
 *   cancelling  |  ✓   |   ✓   |        |       |         |
 *   completed   |  ✓   |   ✓   |        |   ✓   |    ✓    |   ✓
 *   failed      |  ✓   |   ✓   |        |   ✓   |         |   ✓
 *   cancelled   |  ✓   |   ✓   |        |   ✓   |         |   ✓
 *
 *   * Cancel   — only while the run can still accept a cancel request
 *                (running / pending).
 *   * Rerun    — only from a terminal state (completed / failed / cancelled).
 *   * Extract  — only when completed (a successful spec worth promoting).
 *   * Delete   — anything except running (idempotent server-side).
 *   * View     — always (pure navigation).
 *   * Share    — always (copies the in-app run link).
 */
export function actionsForStatus(status: RunStatus): Set<RunAction> {
  const set = new Set<RunAction>(["view", "share"]);
  const canCancel = status === "running" || status === "pending";
  const inFlight = canCancel || status === "cancelling";
  const terminal =
    status === "completed" || status === "failed" || status === "cancelled";
  if (canCancel) set.add("cancel");
  if (terminal) set.add("rerun");
  if (status === "completed") set.add("extract");
  if (!inFlight || status === "pending") set.add("delete"); // delete: NOT running
  return set;
}

/**
 * RunsProvider abstracts the list fetch + per-row mutations so the page never
 * imports mock or backend specifics directly. The mutation methods map to
 * concrete backend RPCs; the page calls them through this interface so the mock
 * can demonstrate them and the real provider can wire the RPC.
 */
export interface RunsProvider {
  /** List a page of runs for a tenant (by slug), applying the query facets. */
  listRuns(tenantSlug: string, query: RunsQuery): Promise<RunsPage>;
  /** Distinct id-facet values (authors / presets) for the toolbar pick-lists. */
  listFacets(tenantSlug: string): Promise<RunFacets>;
  /** Cancel a run -> CancelTestRun. Valid while running / pending. */
  cancelRun(tenantSlug: string, runId: string): Promise<void>;
  /** Re-run an existing run's spec as a new run -> StartTestRun (testRunId). */
  rerunRun(tenantSlug: string, runId: string): Promise<void>;
  /** Promote a run's db+workload into a reusable preset -> ExtractToPreset. */
  extractToPreset(tenantSlug: string, runId: string): Promise<void>;
  /** Delete a run -> DeleteTestRun. Valid for any non-running run. */
  deleteRun(tenantSlug: string, runId: string): Promise<void>;
  /** Toggle caller's favorite relation -> FavoriteService Add/RemoveFavorite. */
  setFavorite(
    tenantSlug: string,
    runId: string,
    favorite: boolean,
  ): Promise<void>;
}

/**
 * Real backend provider. TODO(real-api): build the connect transport + call
 * testRunClient.listTestRuns({ tenantId, filter: { search }, statuses, dbKinds,
 * sort: { by: {...}, desc }, page: { size, token } }) and map the proto
 * TestRunRecord[] onto RunVM[] (+ next_page_token). Throws until wired so a
 * missing-backend misconfig is loud, not silent.
 */
const NOT_WIRED =
  "real RunsProvider not wired yet — run with VITE_MOCK=1 to preview the runs table";

const realRunsProvider: RunsProvider = {
  async listRuns() {
    // await testRunClient.listTestRuns({
    //   tenantId,
    //   filter: {
    //     search: query.search,
    //     authorIds: query.authorIds,                  // filter.author_ids
    //     favoritesOnly: query.favoritesOnly,           // filter.favorites_only
    //     includeDeleted: query.includeDeleted ?? false,// filter.include_deleted
    //   },
    //   statuses, dbKinds, protocols, triggers, stroppyVersions,
    //   standalone: query.standalone,                            // optional bool standalone
    //   progressMin: query.progressMin, progressMax: query.progressMax,
    //   durationMin: durationFrom(query.durationMinSec),         // google.protobuf.Duration
    //   durationMax: durationFrom(query.durationMaxSec),
    //   startedAfter, startedBefore, finishedAfter, finishedBefore,
    //   // sort.kind: trigger -> KIND_TRIGGER (status -> KIND_STATUS, etc.)
    //   sort: { ... , desc }, page: { size, token },
    // });
    throw new Error(NOT_WIRED);
  },
  async listFacets(tenantSlug) {
    const tenantId = await resolveTenantId(tenantSlug);
    const resp = await testRunClient.listTestRunFacets({ tenantId });
    return { authorIds: resp.authorIds };
  },
  // The mutations below follow the SAME convention as listRuns: each documents
  // the exact connectrpc call it will make once the transport is wired, then
  // throws so a missing-backend misconfig is loud, not a silent fake-success.
  async cancelRun() {
    // await testRunClient.cancelTestRun({ tenantId, id: runId });
    // (cloud.v1.api.TestRunService.CancelTestRun — resolve tenantId from slug.)
    throw new Error(NOT_WIRED);
  },
  async rerunRun() {
    // await testRunClient.startTestRun({
    //   tenantId, source: { case: "testRunId", value: runId },
    // });  // cloud.v1.api.TestRunService.StartTestRun
    throw new Error(NOT_WIRED);
  },
  async extractToPreset() {
    // await testRunClient.extractToPreset({ tenantId, id: runId, name: "" });
    // (cloud.v1.api.TestRunService.ExtractToPreset — empty name => server derives.)
    throw new Error(NOT_WIRED);
  },
  async deleteRun() {
    // await testRunClient.deleteTestRun({ tenantId, id: runId });
    // (cloud.v1.api.TestRunService.DeleteTestRun — idempotent.)
    throw new Error(NOT_WIRED);
  },
  async setFavorite() {
    // if (favorite) {
    //   await favoriteClient.addFavorite({
    //     tenantId, kind: FavoriteKind.TEST_RUN, targetId: runId,
    //   });
    // } else {
    //   await favoriteClient.removeFavorite({
    //     tenantId, kind: FavoriteKind.TEST_RUN, targetId: runId,
    //   });
    // }
    // (cloud.v1.api.FavoriteService — resolve tenantId from slug.)
    throw new Error(NOT_WIRED);
  },
};

// --- Provider injection -----------------------------------------------------
//
// Defaults to the real backend. The mock (and ONLY the mock) overrides it via
// setRunsProvider() from the single gate in main.tsx. To remove the mock:
// delete src/mock/ and the gated call in main.tsx.

let active: RunsProvider = realRunsProvider;

export function setRunsProvider(provider: RunsProvider): void {
  active = provider;
}

export function getRunsProvider(): RunsProvider {
  return active;
}
