// SP-B catalog data surface: instance-wide (LEVEL_INSTANCE) and per-tenant
// (LEVEL_ORG) provider/workflow catalog entries (cloud.v1.catalog.CatalogService).
//
// Mirrors recipe.ts's shape: flat exported functions, VM types only — callers
// never touch proto messages. Every org-scoped function resolves the tenant id
// from the slug (services/tenant.ts) before calling the backend, exactly like
// recipe.ts.
//
// getEntryFiles (below) reads a stored entry's bundle back via
// GetInstanceEntryFiles/GetOrgProviderFiles/GetOrgWorkflowFiles — closing the
// GAP the original spb-frontend-report.md documented (CatalogEntry carries
// only an opaque `source_ref` pointer, never its files). entryFilesCache still
// exists as a fast, no-round-trip path for the one session that just
// created/updated an entry; getEntryFiles is the fallback for every other
// caller (a fresh page load, another session, ...).

import { toJson } from "@bufbuild/protobuf";
import {
  CatalogEntrySchema,
  Kind,
  type CatalogEntry,
  type CatalogEntryJson,
  type KindJson,
  type LevelJson,
  type OriginJson,
} from "@/lib/proto/cloud/v1/catalog/models_pb";
import { Severity, type Diagnostic } from "@/lib/proto/cloud/v1/dsl/service_pb";
import { catalogClient } from "@/services/client";
import { resolveTenantId } from "@/services/tenant";

export type CatalogLevel = LevelJson;
export type CatalogKind = Exclude<KindJson, "KIND_UNSPECIFIED">;
export type CatalogOrigin = OriginJson;

// CatalogKind is the enum's JSON *name* ("KIND_PROVIDER") — what the server
// sends back and what the UI routes on. A request message field, however,
// holds the enum's numeric value, and protobuf-es refuses a string there
// ("cannot encode enum cloud.v1.catalog.Kind to JSON: expected number").
// Translate at the boundary rather than casting the type away.
const KIND_WIRE: Record<CatalogKind, Kind> = {
  KIND_PROVIDER: Kind.PROVIDER,
  KIND_WORKFLOW: Kind.WORKFLOW,
};

/** One cloud.v1.dsl.Diagnostic, severity flattened to a lower-case string. Mirrors recipe.ts's DiagnosticVM. */
export interface CatalogDiagnosticVM {
  severity: "error" | "warning";
  path: string;
  line: number;
  col: number;
  message: string;
  module: string;
}

export interface CatalogSummaryVM {
  /** KIND_PROVIDER: manifest.yaml `provides:` capabilities. */
  provides: string[];
  /** KIND_WORKFLOW: cluster.yaml `provider.use`. */
  providerSlug: string;
  /** KIND_WORKFLOW: cluster.yaml machine group count. */
  machineGroupCount: number;
  /** KIND_WORKFLOW: cluster.yaml service count. */
  serviceCount: number;
  /** Last known Check result — true when the bundle compiled with no errors. */
  compiles: boolean;
}

/** One cloud.v1.catalog.CatalogEntry, flattened. */
export interface CatalogEntryVM {
  id: string;
  tenantId: string;
  name: string;
  description: string;
  authorId: string;
  createdAt?: string;
  updatedAt?: string;
  level: CatalogLevel;
  kind: CatalogKind;
  slug: string;
  version: number;
  origin: CatalogOrigin;
  /** The CatalogEntry.entity.id this entry was linked or forked from (empty for native). */
  sourceEntryId: string;
  summary: CatalogSummaryVM;
}

export interface CreateCatalogEntryInput {
  slug: string;
  name: string;
  description: string;
  files: Record<string, string>;
}

// --- encode/decode helpers (mirrors recipe.ts) -------------------------------

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
  for (const [path, content] of Object.entries(files)) {
    out[path] = decoder.decode(content);
  }
  return out;
}

function severityToVM(s: Severity): "error" | "warning" {
  return s === Severity.ERROR ? "error" : "warning";
}

function diagnosticToVM(d: Diagnostic): CatalogDiagnosticVM {
  return {
    severity: severityToVM(d.severity),
    path: d.path,
    line: d.line,
    col: d.col,
    message: d.message,
    module: d.module,
  };
}

function entryToVM(entry: CatalogEntry): CatalogEntryVM {
  const j = toJson(CatalogEntrySchema, entry) as CatalogEntryJson;
  return {
    id: j.entity?.id ?? "",
    tenantId: j.entity?.tenantId ?? "",
    name: j.entity?.name ?? "",
    description: j.entity?.description ?? "",
    authorId: j.entity?.authorId ?? "",
    createdAt: j.entity?.timings?.createdAt,
    updatedAt: j.entity?.timings?.updatedAt,
    level: j.level ?? "LEVEL_UNSPECIFIED",
    kind: (j.kind ?? "KIND_UNSPECIFIED") as CatalogKind,
    slug: j.slug ?? "",
    version: j.version ?? 0,
    origin: j.origin ?? "ORIGIN_NATIVE",
    sourceEntryId: j.sourceEntryId ?? "",
    summary: {
      provides: j.summary?.provides ?? [],
      providerSlug: j.summary?.providerSlug ?? "",
      machineGroupCount: j.summary?.machineGroupCount ?? 0,
      serviceCount: j.summary?.serviceCount ?? 0,
      compiles: j.summary?.compiles ?? false,
    },
  };
}

// --- session-local file cache (see the GAP note in the file doc) -----------
//
// Populated only by THIS session's own create/update calls (the caller
// already holds the files in memory at that point — no server round trip
// invents anything). Never populated from a list/get read, so a genuinely
// unknown bundle stays genuinely unknown rather than silently showing stale
// or wrong content.

const entryFilesCache = new Map<string, Record<string, string>>();

/** Best-effort: the files last known client-side for this entry id, if this
 * session created or updated it. Returns undefined otherwise — callers MUST
 * treat that as "content unavailable", not as an empty bundle. */
export function cachedEntryFiles(entryId: string): Record<string, string> | undefined {
  return entryFilesCache.get(entryId);
}

// --- Instance-level (LEVEL_INSTANCE, admin_only) ----------------------------

export async function listInstanceEntries(kind: CatalogKind): Promise<CatalogEntryVM[]> {
  const { entries } = await catalogClient.listInstanceEntries({ kind: KIND_WIRE[kind] });
  return entries.map(entryToVM);
}

export async function getInstanceEntry(id: string): Promise<CatalogEntryVM> {
  const { entry } = await catalogClient.getInstanceEntry({ id });
  if (!entry) throw new Error(`catalog entry ${id} was not found`);
  return entryToVM(entry);
}

export async function createInstanceEntry(
  kind: CatalogKind,
  input: CreateCatalogEntryInput,
): Promise<CatalogEntryVM> {
  const { entry } = await catalogClient.createInstanceEntry({
    kind: KIND_WIRE[kind],
    slug: input.slug,
    name: input.name,
    description: input.description,
    files: encodeFiles(input.files),
  });
  if (!entry) throw new Error("createInstanceEntry returned no entry");
  const vm = entryToVM(entry);
  entryFilesCache.set(vm.id, input.files);
  return vm;
}

export async function updateInstanceEntry(
  id: string,
  files: Record<string, string>,
): Promise<CatalogEntryVM> {
  const { entry } = await catalogClient.updateInstanceEntry({ id, files: encodeFiles(files) });
  if (!entry) throw new Error("updateInstanceEntry returned no entry");
  const vm = entryToVM(entry);
  entryFilesCache.set(vm.id, files);
  return vm;
}

export async function deleteInstanceEntry(id: string): Promise<void> {
  await catalogClient.deleteInstanceEntry({ id });
}

/** GetInstanceEntryFiles — reads a LEVEL_INSTANCE entry's stored bundle back. */
export async function getInstanceEntryFiles(id: string): Promise<Record<string, string>> {
  const { files } = await catalogClient.getInstanceEntryFiles({ id });
  return decodeFiles(files);
}

// --- Org-level (LEVEL_ORG), split per kind -----------------------------------

export async function listOrgEntries(
  tenantSlug: string,
  kind: CatalogKind,
): Promise<CatalogEntryVM[]> {
  const tenantId = await resolveTenantId(tenantSlug);
  const { entries } =
    kind === "KIND_PROVIDER"
      ? await catalogClient.listOrgProviders({ tenantId })
      : await catalogClient.listOrgWorkflows({ tenantId });
  return entries.map(entryToVM);
}

export async function getOrgEntry(
  tenantSlug: string,
  kind: CatalogKind,
  id: string,
): Promise<CatalogEntryVM> {
  const tenantId = await resolveTenantId(tenantSlug);
  const { entry } =
    kind === "KIND_PROVIDER"
      ? await catalogClient.getOrgProvider({ tenantId, id })
      : await catalogClient.getOrgWorkflow({ tenantId, id });
  if (!entry) throw new Error(`catalog entry ${id} was not found`);
  return entryToVM(entry);
}

export async function createOrgEntry(
  tenantSlug: string,
  kind: CatalogKind,
  input: CreateCatalogEntryInput,
): Promise<CatalogEntryVM> {
  const tenantId = await resolveTenantId(tenantSlug);
  const req = {
    tenantId,
    slug: input.slug,
    name: input.name,
    description: input.description,
    files: encodeFiles(input.files),
  };
  const { entry } =
    kind === "KIND_PROVIDER"
      ? await catalogClient.createOrgProvider(req)
      : await catalogClient.createOrgWorkflow(req);
  if (!entry) throw new Error("createOrgEntry returned no entry");
  const vm = entryToVM(entry);
  entryFilesCache.set(vm.id, input.files);
  return vm;
}

/**
 * updateOrgEntry — the single write path for editing an org entry, REGARDLESS
 * of its current origin: NATIVE/FORKED rows get a new version in place;
 * editing a LINKED row implicitly forks it server-side (origin becomes
 * FORKED, a new entity id/version is minted, the original LINKED row and its
 * upstream instance source are left untouched — see
 * internal/services/catalog/service.go's updateOrgEntry doc and
 * fork_test.go's TestUpdateOrgProvider_ForksLinkedRow). There is NO separate
 * client-callable fork RPC: CatalogService exposes no ForkEntry method (it
 * exists only as an internal Go helper used by tests) — the UI's job is only
 * to warn the caller BEFORE they submit that editing a linked entry will
 * diverge it, never to call a fork step of its own.
 */
export async function updateOrgEntry(
  tenantSlug: string,
  kind: CatalogKind,
  id: string,
  files: Record<string, string>,
): Promise<CatalogEntryVM> {
  const tenantId = await resolveTenantId(tenantSlug);
  const req = { tenantId, id, files: encodeFiles(files) };
  const { entry } =
    kind === "KIND_PROVIDER"
      ? await catalogClient.updateOrgProvider(req)
      : await catalogClient.updateOrgWorkflow(req);
  if (!entry) throw new Error("updateOrgEntry returned no entry");
  const vm = entryToVM(entry);
  entryFilesCache.set(vm.id, files);
  return vm;
}

export async function deleteOrgEntry(
  tenantSlug: string,
  kind: CatalogKind,
  id: string,
): Promise<void> {
  const tenantId = await resolveTenantId(tenantSlug);
  if (kind === "KIND_PROVIDER") {
    await catalogClient.deleteOrgProvider({ tenantId, id });
  } else {
    await catalogClient.deleteOrgWorkflow({ tenantId, id });
  }
}

/** GetOrgProviderFiles/GetOrgWorkflowFiles — reads a LEVEL_ORG entry's stored
 * bundle back (org counterpart of getInstanceEntryFiles above). */
export async function getOrgEntryFiles(
  tenantSlug: string,
  kind: CatalogKind,
  id: string,
): Promise<Record<string, string>> {
  const tenantId = await resolveTenantId(tenantSlug);
  const { files } =
    kind === "KIND_PROVIDER"
      ? await catalogClient.getOrgProviderFiles({ tenantId, id })
      : await catalogClient.getOrgWorkflowFiles({ tenantId, id });
  return decodeFiles(files);
}

/** LinkInstanceProvider/LinkInstanceWorkflow — creates a new LEVEL_ORG row of
 * origin LINKED referencing the instance entry, sharing its content
 * (no bundle copy). Changes to the instance entry do NOT retroactively
 * propagate into already-linked org rows (each is its own immutable version;
 * a new instance version requires re-linking to pick it up) — see
 * service.go's linkInstanceEntry doc. */
export async function linkInstanceEntry(
  tenantSlug: string,
  kind: CatalogKind,
  instanceEntryId: string,
): Promise<CatalogEntryVM> {
  const tenantId = await resolveTenantId(tenantSlug);
  const { entry } =
    kind === "KIND_PROVIDER"
      ? await catalogClient.linkInstanceProvider({ tenantId, instanceEntryId })
      : await catalogClient.linkInstanceWorkflow({ tenantId, instanceEntryId });
  if (!entry) throw new Error("linkInstanceEntry returned no entry");
  return entryToVM(entry);
}

/** CheckCatalogProvider/CheckCatalogWorkflow — stateless check-mode compile of
 * an in-memory (not-yet-saved) bundle, never persisted. tenant_id is required
 * by the RPC purely for all_of/tenant_field RBAC resolution (see
 * service.proto's file doc), even though nothing is written. */
export async function checkCatalogBundle(
  tenantSlug: string,
  kind: CatalogKind,
  files: Record<string, string>,
): Promise<CatalogDiagnosticVM[]> {
  const tenantId = await resolveTenantId(tenantSlug);
  const req = { files: encodeFiles(files), tenantId };
  const { diagnostics } =
    kind === "KIND_PROVIDER"
      ? await catalogClient.checkCatalogProvider(req)
      : await catalogClient.checkCatalogWorkflow(req);
  return diagnostics.map(diagnosticToVM);
}

/** CheckInstanceProvider/CheckInstanceWorkflow — CheckCatalogProvider/
 * CheckCatalogWorkflow's admin_only, tenant-less LEVEL_INSTANCE counterpart:
 * live check-as-you-type for the instance-scope catalog editor, which has no
 * tenant to resolve all_of RBAC against (see service.proto's file doc for why
 * these are split from the tenant-gated pair). */
export async function checkInstanceBundle(
  kind: CatalogKind,
  files: Record<string, string>,
): Promise<CatalogDiagnosticVM[]> {
  const req = { files: encodeFiles(files) };
  const { diagnostics } =
    kind === "KIND_PROVIDER"
      ? await catalogClient.checkInstanceProvider(req)
      : await catalogClient.checkInstanceWorkflow(req);
  return diagnostics.map(diagnosticToVM);
}
