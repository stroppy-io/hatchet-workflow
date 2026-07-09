// Recipe/DSL data surface: persisted DSL recipe bundles (cloud.v1.api.RecipeService)
// and stateless DSL check/schema helpers (cloud.v1.dsl.DslService).
//
// Mirrors the services/runs.ts + services/preset.ts shape: the caller depends
// ONLY on the flat VM types + exported functions below, never on proto
// messages or the wire format. Every function resolves the tenant id from the
// slug (services/tenant.ts) before calling the backend.
//
// RecipeRecord's bundle.files is a map<string, bytes> (path -> Uint8Array) on
// the wire; the VM flattens it to Record<string, string> (UTF-8 text) since
// every bundle file (cluster.yaml, workflow.yaml, components/**, providers/**)
// is textual DSL/YAML source. TextEncoder/TextDecoder do the conversion at the
// edge so the rest of the app never touches Uint8Array.

import { create, fromJson, toJson } from "@bufbuild/protobuf";
import type { BakedJson, FieldErrorJson, Schema } from "@stroppy-io/schemapb";
import { FieldErrorSchema, FilledSchema } from "@stroppy-io/schemapb";
import {
  RecipeRecordSchema,
  type RecipeRecord,
} from "@/lib/proto/cloud/v1/models/recipe_pb";
import { Severity, type Diagnostic } from "@/lib/proto/cloud/v1/dsl/service_pb";
import type { CompiledPlan } from "@/lib/proto/cloud/v1/dsl/compiled_pb";
import { recipeClient, dslClient } from "@/services/client";
import { resolveTenantId } from "@/services/tenant";
import { runToVM, type RunVM } from "@/services/runs";

// Re-exported so callers of RecipeService's run lifecycle (listRuns et al)
// depend only on services/recipe.ts, without also importing services/runs.ts
// directly. RunVM here is the SAME flat view-model runs.ts's runToVM
// produces from models.Run — the runs table (RecipeRuns.tsx) and RunDetail
// both need the full set of denormalized Summary facets (db_kind, provider,
// node_count, progress_pct, started/finished/duration, ...), not just the
// four fields a thin id/name/status/workflowId/startedAt projection carried
// before (see the parity audit in .superpowers/sdd/runs-parity-report.md).
export type { RunVM };

/** One persisted DSL recipe bundle, flattened from cloud.v1.models.RecipeRecord. */
export interface RecipeVM {
  /** entity.id */
  id: string;
  /** entity.name */
  name: string;
  /** version — immutable bundle revision number. */
  version: number;
  /** summary.provider — deployment provider declared by the bundle. */
  provider: string;
  /** summary.machine_group_count */
  machineGroupCount: number;
  /** summary.service_count */
  serviceCount: number;
  /** summary.compiles — last known Check result. */
  compiles: boolean;
  /** bundle.files, decoded path -> UTF-8 text (cluster.yaml, workflow.yaml, ...). */
  files: Record<string, string>;
}

/** One cloud.v1.dsl.Diagnostic, severity flattened to a lower-case string. */
export interface DiagnosticVM {
  severity: "error" | "warning";
  path: string;
  line: number;
  col: number;
  message: string;
  module: string;
}

/** One cloud.v1.dsl.MachineGroup from a Preview plan, flattened for the panel. */
export interface PreviewMachineGroupVM {
  name: string;
  count: number;
  cpu: number;
  ramMb: number;
  /** Sum of the group's disk sizes (GB), per-machine (not multiplied by count). */
  diskGb: number;
}

/** One cloud.v1.dsl.ServiceSpec from a Preview plan, flattened for the panel. */
export interface PreviewServiceVM {
  name: string;
  onGroup: string;
  image: string;
  network: string;
}

/** cloud.v1.dsl.CompiledPlan, flattened to what the RecipeEditor preview panel renders. */
export interface PreviewPlanVM {
  provider: string;
  machineGroups: PreviewMachineGroupVM[];
  services: PreviewServiceVM[];
  /** Sum of machineGroups[].count — total provisioned node count. */
  nodeTotal: number;
}

/** DslService.Preview's response, flattened: plan is null when the bundle failed to compile. */
export interface PreviewVM {
  plan: PreviewPlanVM | null;
  diagnostics: DiagnosticVM[];
}

// --- encode/decode helpers ---------------------------------------------------

const encoder = new TextEncoder();
const decoder = new TextDecoder();

function encodeFiles(files: Record<string, string>): Record<string, Uint8Array> {
  const out: Record<string, Uint8Array> = {};
  for (const [path, content] of Object.entries(files)) {
    out[path] = encoder.encode(content);
  }
  return out;
}

function decodeFiles(files: Record<string, Uint8Array>): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [path, bytes] of Object.entries(files)) {
    out[path] = decoder.decode(bytes);
  }
  return out;
}

/** Severity enum -> lower-case string; UNSPECIFIED collapses to "warning". */
function severityToVM(s: Severity): "error" | "warning" {
  return s === Severity.ERROR ? "error" : "warning";
}

function diagnosticToVM(d: Diagnostic): DiagnosticVM {
  return {
    severity: severityToVM(d.severity),
    path: d.path,
    line: d.line,
    col: d.col,
    message: d.message,
    module: d.module,
  };
}

// bundle.files is a raw bytes map — read the fields directly off the message
// (no toJson, which would base64-encode the bytes and complicate decoding).
function recipeRecordToVM(rec: RecipeRecord): RecipeVM {
  const summary = rec.summary;
  return {
    id: rec.entity?.id ?? "",
    name: rec.entity?.name ?? "",
    version: rec.version ?? 0,
    provider: summary?.provider ?? "",
    machineGroupCount: summary?.machineGroupCount ?? 0,
    serviceCount: summary?.serviceCount ?? 0,
    compiles: summary?.compiles ?? false,
    files: decodeFiles(rec.bundle?.files ?? {}),
  };
}

// --- RecipeService: CRUD + check-mode ---------------------------------------

export async function listRecipes(tenantSlug: string): Promise<RecipeVM[]> {
  const tenantId = await resolveTenantId(tenantSlug);
  const { recipes } = await recipeClient.listRecipes({ tenantId });
  return recipes.map(recipeRecordToVM);
}

export async function getRecipe(tenantSlug: string, id: string): Promise<RecipeVM> {
  const tenantId = await resolveTenantId(tenantSlug);
  const { recipe } = await recipeClient.getRecipe({ tenantId, id });
  if (!recipe) throw new Error(`recipe ${id} was not found`);
  return recipeRecordToVM(recipe);
}

export async function createRecipe(
  tenantSlug: string,
  input: { name: string; files: Record<string, string> },
): Promise<RecipeVM> {
  const tenantId = await resolveTenantId(tenantSlug);
  const { recipe } = await recipeClient.createRecipe({
    tenantId,
    recipe: create(RecipeRecordSchema, {
      entity: { name: input.name },
      bundle: { files: encodeFiles(input.files) },
    }),
  });
  if (!recipe) throw new Error("createRecipe returned no recipe");
  return recipeRecordToVM(recipe);
}

export async function deleteRecipe(tenantSlug: string, id: string): Promise<void> {
  const tenantId = await resolveTenantId(tenantSlug);
  await recipeClient.deleteRecipe({ tenantId, id });
}

/** CheckRecipe — check-mode compile of the STORED bundle (no local files). */
export async function checkStoredRecipe(
  tenantSlug: string,
  id: string,
): Promise<DiagnosticVM[]> {
  const tenantId = await resolveTenantId(tenantSlug);
  const { diagnostics } = await recipeClient.checkRecipe({ tenantId, id });
  return diagnostics.map(diagnosticToVM);
}

// --- DslService: stateless check / schema (no persistence) -----------------
//
// tenantSlug is OPTIONAL on Check/Preview (unlike every RecipeService
// function above, which requires one): DslService.Check/Preview fall back to
// resolving provider.use against the bundle's own providers/<name>/
// subtree when no tenant_id is sent (see CheckRequest.tenant_id's proto
// doc) — the SP-B org-catalog provider resolver only engages when a caller
// supplies one. Passing it through here lets provider.use also resolve
// catalog-managed providers by slug from the live editor.

/** DslService.Check — check-mode compile of an in-memory (not-yet-saved) bundle. */
export async function checkBundle(
  files: Record<string, string>,
  tenantSlug?: string,
): Promise<DiagnosticVM[]> {
  const tenantId = tenantSlug ? await resolveTenantId(tenantSlug) : undefined;
  const { diagnostics } = await dslClient.check({ files: encodeFiles(files), tenantId });
  return diagnostics.map(diagnosticToVM);
}

/**
 * RecipeService.LaunchFormSchema — the composed launch-form schemapb.Schema
 * for an already-stored recipe bundle (workflow.yaml's declared inputs at
 * the top level, plus the resolved provider's params nested under
 * "provider", absent for docker/no-provider bundles). Unlike composedSchema
 * (DslService, operates on in-memory files for the editor's own use), this
 * reads the STORED bundle by id — the source LaunchForm.tsx renders from.
 */
export async function fetchLaunchFormSchema(tenantSlug: string, recipeId: string): Promise<Schema> {
  const tenantId = await resolveTenantId(tenantSlug);
  const { schema } = await recipeClient.launchFormSchema({ tenantId, recipeId });
  if (!schema) throw new Error("launchFormSchema returned no schema");
  return schema;
}

/** DslService.ComposedSchema — schemapb.Schema (protojson): the bundle's launch-form schema (workflow inputs + provider params), used to render/validate the launch form. */
export async function composedSchema(files: Record<string, string>): Promise<string> {
  const { schemaJson } = await dslClient.composedSchema({ files: encodeFiles(files) });
  return schemaJson;
}

// plan is a bare Message with no toJson usage here (its ramMb/sizeGb are
// uint64 -> bigint on the wire): reading fields directly off the message and
// narrowing to number keeps the VM JSON-serializable and avoids the
// precision-preserving-but-awkward bigint type leaking into the UI layer.
// Machine sizes (RAM in MB, disk in GB) never approach Number.MAX_SAFE_INTEGER
// in practice, so the narrowing is safe.
function planToVM(plan: CompiledPlan | undefined): PreviewPlanVM | null {
  if (!plan) return null;
  return {
    provider: plan.provider?.name ?? "",
    machineGroups: plan.machineGroups.map((g) => ({
      name: g.name,
      count: g.count,
      cpu: g.cpu,
      ramMb: Number(g.ramMb),
      diskGb: g.disks.reduce((sum, d) => sum + Number(d.sizeGb), 0),
    })),
    services: plan.services.map((s) => ({
      name: s.name,
      onGroup: s.onGroup,
      image: s.image,
      network: s.network,
    })),
    nodeTotal: plan.machineGroups.reduce((sum, g) => sum + g.count, 0),
  };
}

/**
 * DslService.Preview — compiles an in-memory (not-yet-saved) bundle and
 * returns the resolved CompiledPlan (machine groups, services) for the
 * RecipeEditor's "what will be provisioned" panel, plus diagnostics. Unlike
 * checkBundle (auto-run on every edit, debounced), this is called on-demand
 * from a "Preview" button — a full compile is heavier than Check's own
 * diagnostics pass, and the diagnostics panel already covers the
 * edit-as-you-type linting need.
 */
export async function previewBundle(
  files: Record<string, string>,
  tenantSlug?: string,
): Promise<PreviewVM> {
  const tenantId = tenantSlug ? await resolveTenantId(tenantSlug) : undefined;
  const { plan, diagnostics } = await dslClient.preview({ files: encodeFiles(files), tenantId });
  return {
    plan: planToVM(plan),
    diagnostics: diagnostics.map(diagnosticToVM),
  };
}

// --- RecipeService: run lifecycle -------------------------------------------

/**
 * StartRunFieldError carries StartRunResponse.field_errors — thrown instead
 * of a bare Error so LaunchForm.tsx can render each failure against its own
 * field (mirroring LaunchFormRenderer's onInvalid contract for the local
 * WASM Bake) rather than a single opaque message. This is StartRun's
 * *server-side* re-Bake failing (the trust boundary: the browser-side Bake
 * only seals the payload for UX, it is re-validated server-side against the
 * same composed form schema before a run is minted) — distinct from a local
 * onInvalid, which never reaches the network at all.
 */
export class StartRunFieldError extends Error {
  readonly fieldErrors: FieldErrorJson[];

  constructor(fieldErrors: FieldErrorJson[]) {
    super(fieldErrors.map((e) => `${e.field}: ${e.message}`).join("; ") || "launch form failed validation");
    this.name = "StartRunFieldError";
    this.fieldErrors = fieldErrors;
  }
}

/**
 * RecipeService.StartRun — launches the stored recipe bundle. filled is the
 * launch form's baked/sealed submission (LaunchFormRenderer's onSubmit
 * payload, BakedJson-shaped); StartRun re-Bakes it server-side against the
 * same composed form schema (schemapb single source of truth — no
 * hand-rolled validation here) before minting a run. A server-side bake
 * failure comes back as a normal (non-throwing-at-the-RPC-level) response
 * with field_errors populated and no run — surfaced here as a
 * StartRunFieldError so the caller can render it per-field.
 */
export async function startRun(tenantSlug: string, recipeId: string, filled?: BakedJson): Promise<string> {
  const tenantId = await resolveTenantId(tenantSlug);
  const { run, fieldErrors } = await recipeClient.startRun({
    tenantId,
    recipeId,
    filled: filled ? fromJson(FilledSchema, { values: filled.values ?? {} }) : undefined,
  });
  if (fieldErrors.length > 0) {
    throw new StartRunFieldError(fieldErrors.map((e) => toJson(FieldErrorSchema, e) as FieldErrorJson));
  }
  if (!run?.entity?.id) throw new Error("startRun returned no run");
  return run.entity.id;
}

/**
 * Lists runs launched from recipe bundles, optionally narrowed to one recipe.
 *
 * When recipeId is set, RecipeService.ListRuns applies the filter IN-PROCESS
 * over an already-paginated tenant page (see ListRunsResponse.next_page_token
 * doc), so a single page can come back empty while next_page_token is still
 * non-empty. We loop pages until next_page_token is empty, capped at
 * MAX_PAGES to bound worst-case latency for a pathological (huge,
 * mostly-unrelated) tenant run history.
 */
const MAX_LIST_RUNS_PAGES = 20;

export async function listRuns(
  tenantSlug: string,
  recipeId?: string,
): Promise<RunVM[]> {
  const tenantId = await resolveTenantId(tenantSlug);
  const out: RunVM[] = [];
  let token = "";
  for (let page = 0; page < MAX_LIST_RUNS_PAGES; page++) {
    const { runs, nextPageToken } = await recipeClient.listRuns({
      tenantId,
      recipeId: recipeId ?? "",
      page: { size: 0, token },
    });
    out.push(...runs.map(runToVM));
    if (!nextPageToken) break;
    token = nextPageToken;
  }
  return out;
}

export async function cancelRun(tenantSlug: string, runId: string): Promise<void> {
  const tenantId = await resolveTenantId(tenantSlug);
  await recipeClient.cancelRun({ tenantId, runId });
}

export async function deleteRun(tenantSlug: string, runId: string): Promise<void> {
  const tenantId = await resolveTenantId(tenantSlug);
  await recipeClient.deleteRun({ tenantId, runId });
}
