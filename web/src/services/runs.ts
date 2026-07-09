// Test-run view-model surface, shared by RunDetail (via run_overview.ts) and
// the recipe-run views. The old test-run listing/mutation RPC surface
// (list/cancel/rerun/extract/delete/favorite via a RunsProvider) was removed
// here — runs are now driven through RecipeService (see services/recipe.ts).
// What remains is the flat view-model (RunVM), the mapper from
// cloud.v1.models.Run (runToVM — models/test_run.proto only), and the
// row-action gating used by RunDetail (RunAction/actionsForStatus).

import { toJson } from "@bufbuild/protobuf";
import { RunSchema, type Run } from "@/lib/proto/cloud/v1/models/test_run_pb";
import type { BakedJson } from "@/lib/proto/schemapb/schema_pb";
import type { CompiledPlanJson } from "@/lib/proto/cloud/v1/dsl/compiled_pb";
import { type RunStatus, statusToVM } from "@/services/dashboard";
import {
  dbKindLabelFromJson,
  providerLabelFromJson,
  protocolLabelFromJson,
  triggerLabelFromJson,
} from "@/services/enums";

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
  | "orioledb"
  | "external"
  | "noop"
  | "pgnoop";

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
  | "cockroach"
  | "noop";

/**
 * How the run was triggered, mapped from cloud.v1.common.Trigger. Lower-cased,
 * UNSPECIFIED collapsed to "" so the column/filter can hide it.
 */
export type RunTrigger = "" | "manual" | "cron" | "api";

/**
 * Deployment provider, mapped from cloud.v1.deployment.Provider. Lower-cased,
 * UNSPECIFIED collapsed to "" so the record field can hide it. Still carried on
 * RunVM (summary.provider) though no longer exposed as a filter facet.
 */
export type DeployProvider = "" | "docker" | "yandex";

/**
 * A test run, flattened from cloud.v1.models.Run. Every field maps to a REAL
 * record field — id/name/author/createdAt from entity; status/trigger from
 * the record; the rest from the denormalized summary projection. Optional
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
  /**
   * record.workflow_id — the originating models.RecipeRecord.entity.id that
   * RecipeService.StartRun launched this run from. Always set (every live run
   * is a recipe run — see cloud.v1.models.Run's doc). Lets RunDetail's Rerun
   * action re-launch through RecipeService.StartRun(workflowId).
   */
  workflowId: string;
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
  /**
   * baked — the sealed launch-form snapshot (schemapb.Baked: schema + frozen
   * values), when the run was launched through SP-D's generated-form path.
   * nil until SP-D exists (see cloud.v1.models.Run.baked's doc). Rendered on
   * the run Config tab as the primary "resolved parameters" view.
   */
  baked?: BakedJson;
  /**
   * compiledPlan — the compiled DSL plan (machine groups, services, jobs)
   * RunRecipeWorkflow actually executed. Rendered on the run Config tab
   * alongside `baked` — this is what today's `RunConfigTab.tsx` had NO
   * equivalent for (recipe runs never carried a `domain.TestRun` spec), so
   * this is new, not preserved, data (spec §6.C "fixed" delta).
   */
  compiledPlan?: CompiledPlanJson;
}

/**
 * RunAction enumerates the per-row operations exposed by the table's row
 * Actions menu. Every member maps to a REAL RPC or an existing in-app route —
 * no speculative operations:
 *   * "view"      -> in-app route /runs/:id (no RPC; always available).
 *   * "share"     -> copy an in-app /runs/:id link (no RPC; always available).
 *   * "cancel"    -> RecipeService.CancelRun  (running / pending only).
 *   * "rerun"     -> RecipeService.StartRun on the run's originating recipe
 *                    (terminal states only: completed / failed / cancelled).
 *   * "delete"    -> RecipeService.DeleteRun  (anything NOT running).
 * Favorite toggling is intentionally NOT a RunAction menu item: it maps to the
 * cross-resource FavoriteService AddFavorite/RemoveFavorite pair.
 *
 * A "clone"/"New from run" action was removed here: it pointed at
 * /runs/new?from=:id, a route that was never wired to any handler (dead —
 * re-running a recipe run is already covered by "rerun", which relaunches the
 * run's originating recipe bundle via RecipeService.StartRun).
 */
export type RunAction =
  | "view"
  | "share"
  | "cancel"
  | "rerun"
  | "delete";

/**
 * actionsForStatus encodes the status-gating allow-rules. It returns, for a
 * given run lifecycle status, the set of actions whose backing RPC/route is
 * valid in that state. The Actions menu renders every action but DISABLES the
 * ones absent from this set (disabled-with-reason is friendlier than hiding):
 *
 *   status      | view | share | cancel | rerun | delete
 *   ------------+------+-------+--------+-------+-------
 *   pending     |  ✓   |   ✓   |   ✓    |       |   ✓
 *   running     |  ✓   |   ✓   |   ✓    |       |
 *   cancelling  |  ✓   |   ✓   |        |       |
 *   completed   |  ✓   |   ✓   |        |   ✓   |   ✓
 *   failed      |  ✓   |   ✓   |        |   ✓   |   ✓
 *   cancelled   |  ✓   |   ✓   |        |   ✓   |   ✓
 *
 *   * Cancel   — only while the run can still accept a cancel request
 *                (running / pending).
 *   * Rerun    — only from a terminal state (completed / failed / cancelled).
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
  if (!inFlight || status === "pending") set.add("delete"); // delete: NOT running
  return set;
}

// One Run -> flat RunVM (string enums + ISO timestamps via toJson).
export function runToVM(rec: Run): RunVM {
  const j = toJson(RunSchema, rec) as {
    entity?: {
      id?: string;
      name?: string;
      authorId?: string;
      isFavorite?: boolean;
      timings?: { createdAt?: string; deletedAt?: string };
    };
    status?: string;
    trigger?: string;
    workflowId?: string;
    baked?: BakedJson;
    compiledPlan?: CompiledPlanJson;
    summary?: {
      dbKind?: string;
      workloadName?: string;
      stroppyVersion?: string;
      workloadProtocol?: string;
      topologyLabel?: string;
      nodeCount?: number;
      provider?: string;
      progressPct?: number;
      startedAt?: string;
      finishedAt?: string;
      duration?: string;
      dbPresetId?: string;
      workloadPresetId?: string;
      testPresetId?: string;
    };
  };
  const e = j.entity ?? {};
  const s = j.summary ?? {};
  return {
    id: e.id ?? "",
    name: e.name ?? "",
    authorId: e.authorId ?? "",
    createdAt: e.timings?.createdAt ?? "",
    status: statusToVM(j.status),
    dbKind: dbKindLabelFromJson(s.dbKind),
    workload: s.workloadName ?? "",
    stroppyVersion: s.stroppyVersion ?? "",
    protocol: protocolLabelFromJson(s.workloadProtocol),
    trigger: triggerLabelFromJson(j.trigger),
    topologyLabel: s.topologyLabel ?? "",
    nodeCount: s.nodeCount ?? 0,
    progressPct: s.progressPct ?? 0,
    startedAt: s.startedAt,
    finishedAt: s.finishedAt,
    durationSec: s.duration ? parseFloat(s.duration) : undefined,
    workflowId: j.workflowId ?? "",
    provider: providerLabelFromJson(s.provider),
    dbPresetId: s.dbPresetId ?? "",
    workloadPresetId: s.workloadPresetId ?? "",
    testPresetId: s.testPresetId ?? "",
    favorite: e.isFavorite ?? false,
    deleted: !!e.timings?.deletedAt,
    baked: j.baked,
    compiledPlan: j.compiledPlan,
  };
}
