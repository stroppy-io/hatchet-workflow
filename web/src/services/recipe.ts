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

import { create, toJson } from "@bufbuild/protobuf";
import {
  RecipeRecordSchema,
  type RecipeRecord,
} from "@/lib/proto/cloud/v1/models/recipe_pb";
import {
  TestRunRecordSchema,
  type TestRunRecord,
} from "@/lib/proto/cloud/v1/models/test_run_pb";
import { Severity, type Diagnostic } from "@/lib/proto/cloud/v1/dsl/service_pb";
import { recipeClient, dslClient } from "@/services/client";
import { resolveTenantId } from "@/services/tenant";
import { statusToVM, type RunStatus } from "@/services/dashboard";

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

/** One run launched from a recipe bundle, flattened from models.TestRunRecord. */
export interface RunVM {
  /** entity.id */
  id: string;
  /** entity.name */
  name: string;
  /** record.status, mapped to the dashboard RunStatus union. */
  status: RunStatus;
  /** record.recipe_id — the originating recipe bundle. */
  recipeId: string;
  /** summary.started_at (ISO); absent until the run starts. */
  startedAt?: string;
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

// TestRunRecord.status/entity are proto enum/message fields — toJson gives us
// the JSON-string enum form that statusToVM (services/dashboard.ts) expects.
function runRecordToVM(rec: TestRunRecord): RunVM {
  const j = toJson(TestRunRecordSchema, rec) as {
    entity?: { id?: string; name?: string };
    status?: string;
    recipeId?: string;
    summary?: { startedAt?: string };
  };
  return {
    id: j.entity?.id ?? "",
    name: j.entity?.name ?? "",
    status: statusToVM(j.status),
    recipeId: j.recipeId ?? "",
    startedAt: j.summary?.startedAt,
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

// --- DslService: stateless check / schema (no tenant, no persistence) ------

/** DslService.Check — check-mode compile of an in-memory (not-yet-saved) bundle. */
export async function checkBundle(
  files: Record<string, string>,
): Promise<DiagnosticVM[]> {
  const { diagnostics } = await dslClient.check({ files: encodeFiles(files) });
  return diagnostics.map(diagnosticToVM);
}

/** DslService.ComposedSchema — dynamic JSON Schema for the bundle's editor. */
export async function composedSchema(files: Record<string, string>): Promise<string> {
  const { schemaJson } = await dslClient.composedSchema({ files: encodeFiles(files) });
  return schemaJson;
}

// --- RecipeService: run lifecycle -------------------------------------------

export async function startRun(tenantSlug: string, recipeId: string): Promise<string> {
  const tenantId = await resolveTenantId(tenantSlug);
  const { run } = await recipeClient.startRun({ tenantId, recipeId });
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
    out.push(...runs.map(runRecordToVM));
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
