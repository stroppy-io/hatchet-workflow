// CatalogEntryEditor — multi-file bundle editor for cloud.v1.catalog.CatalogService,
// shared between the instance scope (/admin/catalog/...) and the org scope
// (/t/:slug/catalog/...). Mirrors RecipeEditor.tsx's file-tabs + DslEditor +
// diagnostics-panel layout (Preview/Run are dropped — a catalog entry is a
// reusable building block, not something you launch directly).
//
// CREATE: seeds a kind-appropriate template (manifest.yaml for a provider,
// cluster.yaml+workflow.yaml for a workflow) and posts via createInstanceEntry/
// createOrgEntry.
//
// EDIT: CatalogEntry carries no file contents of its own (only an opaque
// source_ref — see catalog/models.proto's doc for why). cachedEntryFiles
// pre-fills the editor for free when this browser session itself just
// created/updated this entry; otherwise getInstanceEntryFiles/getOrgEntryFiles
// (GetInstanceEntryFiles/GetOrgProviderFiles/GetOrgWorkflowFiles) fetch the
// stored bundle back from the server. Saving always calls
// updateInstanceEntry/updateOrgEntry with the full files map (bytes, not a
// diff) — for a LINKED org entry this is exactly the fork-on-edit path: the
// server implicitly forks (new id, origin -> FORKED) rather than mutating the
// linked row (see catalog.ts's updateOrgEntry doc). The UI's only job is
// warning the user BEFORE they save that this will happen.

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  AlertCircle,
  AlertTriangle,
  ArrowLeft,
  FileCode2,
  GitFork,
  Link2,
  Loader2,
  Plus,
  Save,
  X,
} from "lucide-react";
import { useNavigate, useParams, useTenantSlug } from "@/lib/router";
import { useBreadcrumbLabel } from "@/lib/breadcrumbs";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { DslEditor } from "@/components/ui/dsl-editor";
import {
  cachedEntryFiles,
  checkCatalogBundle,
  checkInstanceBundle,
  createInstanceEntry,
  createOrgEntry,
  getInstanceEntry,
  getInstanceEntryFiles,
  getOrgEntry,
  getOrgEntryFiles,
  updateInstanceEntry,
  updateOrgEntry,
  type CatalogDiagnosticVM,
  type CatalogEntryVM,
  type CatalogKind,
} from "@/services/catalog";

const PROVIDER_TEMPLATE: Record<string, string> = {
  "manifest.yaml": `# edit me — manifest.yaml declares what this provider makes available.
name: my-provider
provides: [machines]
`,
};

const WORKFLOW_TEMPLATE: Record<string, string> = {
  "cluster.yaml": `# edit me — cluster.yaml declares the deployment topology.
version: 1
provider:
  use: docker
machines:
  db:
    count: 1
    resources:
      cpu: 2
      ram: 4g
services: {}
`,
  "workflow.yaml": `# edit me — workflow.yaml declares the test-run jobs.
jobs: {}
`,
};

function templateFor(kind: CatalogKind): Record<string, string> {
  return kind === "KIND_PROVIDER" ? { ...PROVIDER_TEMPLATE } : { ...WORKFLOW_TEMPLATE };
}

function firstPath(files: Record<string, string>): string {
  return Object.keys(files).sort()[0] ?? "";
}

interface CatalogEntryEditorProps {
  /** "instance" -> /admin/catalog/...; "org" -> /t/:slug/catalog/... */
  scope: "instance" | "org";
}

export function CatalogEntryEditor({ scope }: CatalogEntryEditorProps) {
  const slug = useTenantSlug() ?? "";
  const { kind: kindParam, id } = useParams();
  const navigate = useNavigate();
  const isEdit = !!id;
  const kind: CatalogKind = kindParam === "workflows" ? "KIND_WORKFLOW" : "KIND_PROVIDER";
  const kindLabel = kind === "KIND_PROVIDER" ? "provider" : "workflow";
  const basePath = scope === "instance" ? "/admin/catalog" : "/catalog";

  const [entry, setEntry] = useState<CatalogEntryVM | null>(null);
  const [entrySlug, setEntrySlug] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [files, setFiles] = useState<Record<string, string>>(() =>
    isEdit ? {} : templateFor(kind),
  );
  const [activePath, setActivePath] = useState(() =>
    isEdit ? "" : firstPath(templateFor(kind)),
  );

  const [loading, setLoading] = useState(isEdit);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const [diagnostics, setDiagnostics] = useState<CatalogDiagnosticVM[]>([]);
  const [checking, setChecking] = useState(false);

  const [newFileName, setNewFileName] = useState("");

  useBreadcrumbLabel("id", isEdit ? name || id : undefined);

  useEffect(() => {
    if (!isEdit || !id) return;
    let cancelled = false;
    setLoading(true);
    setLoadError(null);
    const load =
      scope === "instance"
        ? getInstanceEntry(id)
        : slug
          ? getOrgEntry(slug, kind, id)
          : Promise.reject(new Error("no tenant"));
    load
      .then(async (e) => {
        if (cancelled) return;
        setEntry(e);
        setEntrySlug(e.slug);
        setName(e.name);
        setDescription(e.description);
        // cachedEntryFiles is a fast, no-round-trip path for the one session
        // that just created/updated this entry; every other load fetches the
        // stored bundle back from the server (GetInstanceEntryFiles/
        // GetOrgProviderFiles/GetOrgWorkflowFiles — see services/catalog.ts).
        const cached = cachedEntryFiles(e.id);
        const loadedFiles =
          cached ??
          (scope === "instance" ? await getInstanceEntryFiles(e.id) : await getOrgEntryFiles(slug, kind, e.id));
        if (cancelled) return;
        setFiles(loadedFiles);
        setActivePath(firstPath(loadedFiles));
      })
      .catch((e) => !cancelled && setLoadError(e instanceof Error ? e.message : String(e)))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [isEdit, id, scope, slug, kind]);

  // Bundle-wide diagnostics: debounced re-check on any edit, mirrors
  // RecipeEditor. Org scope uses CheckCatalogProvider/Workflow (tenant-gated
  // RBAC); instance scope uses their admin_only CheckInstanceProvider/
  // Workflow counterpart (see services/catalog.ts's doc).
  useEffect(() => {
    if (loading || Object.keys(files).length === 0) return;
    if (scope === "org" && !slug) return;
    let cancelled = false;
    setChecking(true);
    const t = setTimeout(() => {
      const check = scope === "instance" ? checkInstanceBundle(kind, files) : checkCatalogBundle(slug, kind, files);
      check
        .then((diags) => !cancelled && setDiagnostics(diags))
        .catch((e) => {
          if (cancelled) return;
          console.warn("CatalogEntryEditor: check bundle failed", e);
          setDiagnostics([]);
        })
        .finally(() => !cancelled && setChecking(false));
    }, 500);
    return () => {
      cancelled = true;
      clearTimeout(t);
    };
  }, [files, loading, slug, kind, scope]);

  const slugMissing = !entrySlug.trim();
  const nameMissing = !name.trim();
  const canSave = !saving && !nameMissing && (isEdit || !slugMissing) && Object.keys(files).length > 0;

  const updateFileContent = useCallback((path: string, value: string) => {
    setFiles((cur) => ({ ...cur, [path]: value }));
  }, []);

  const addFile = useCallback(() => {
    const path = newFileName.trim();
    if (!path || files[path] !== undefined) return;
    setFiles((cur) => ({ ...cur, [path]: "# new file\n" }));
    setActivePath(path);
    setNewFileName("");
  }, [newFileName, files]);

  const removeFile = useCallback(
    (path: string) => {
      const remaining = Object.keys(files).filter((p) => p !== path);
      if (remaining.length === 0) return; // never remove the last file
      setFiles((cur) => {
        const next = { ...cur };
        delete next[path];
        return next;
      });
      if (activePath === path) setActivePath(remaining.sort()[0]);
    },
    [files, activePath],
  );

  const onSave = useCallback(async () => {
    setSaving(true);
    setSaveError(null);
    try {
      if (isEdit && id) {
        const saved =
          scope === "instance"
            ? await updateInstanceEntry(id, files)
            : await updateOrgEntry(slug, kind, id, files);
        navigate(`${basePath}/${kindParam}/${saved.id}/edit`);
      } else {
        const saved =
          scope === "instance"
            ? await createInstanceEntry(kind, {
                slug: entrySlug.trim(),
                name: name.trim(),
                description,
                files,
              })
            : await createOrgEntry(slug, kind, {
                slug: entrySlug.trim(),
                name: name.trim(),
                description,
                files,
              });
        navigate(`${basePath}/${kindParam}/${saved.id}/edit`);
      }
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  }, [isEdit, id, scope, slug, kind, files, entrySlug, name, description, navigate, basePath, kindParam]);

  const paths = useMemo(() => Object.keys(files).sort(), [files]);
  const errorCount = diagnostics.filter((d) => d.severity === "error").length;
  const warningCount = diagnostics.filter((d) => d.severity === "warning").length;

  const isLinked = entry?.origin === "ORIGIN_LINKED";
  const isForked = entry?.origin === "ORIGIN_FORKED";

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-zinc-500">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" /> Loading {kindLabel}…
      </div>
    );
  }
  if (loadError) {
    return (
      <div className="p-5">
        <div className="flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
          <AlertCircle className="h-4 w-4" /> {loadError}
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-zinc-800/80 px-5 py-3">
        <div className="flex items-center gap-3 min-w-0">
          <button
            type="button"
            onClick={() => navigate(`${basePath}?tab=${kindParam}`)}
            className="flex h-7 w-7 shrink-0 items-center justify-center border border-zinc-800 text-zinc-500 transition-colors hover:border-zinc-700 hover:text-zinc-300"
            aria-label="Back"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>
          <div className="min-w-0">
            <div className="flex items-center gap-1.5 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
              {isEdit ? `Edit ${kindLabel}` : `New ${kindLabel}`}
              {isLinked && (
                <span className="flex items-center gap-1 text-blue-400">
                  <Link2 className="h-3 w-3" /> linked
                </span>
              )}
              {isForked && (
                <span className="flex items-center gap-1 text-amber-400">
                  <GitFork className="h-3 w-3" /> forked
                </span>
              )}
            </div>
            <Input
              className="mt-0.5 h-7 max-w-xs border-none bg-transparent px-0 text-base font-semibold tracking-tight shadow-none focus-visible:ring-0"
              value={name}
              placeholder="Display name"
              onChange={(e) => setName(e.target.value)}
            />
          </div>
        </div>
        <div className="flex items-center gap-2">
          {errorCount > 0 && <Badge variant="destructive">{errorCount} error{errorCount === 1 ? "" : "s"}</Badge>}
          {warningCount > 0 && <Badge variant="warning">{warningCount} warning{warningCount === 1 ? "" : "s"}</Badge>}
          <Button variant="outline" size="sm" onClick={() => navigate(`${basePath}?tab=${kindParam}`)}>
            Cancel
          </Button>
          <Button size="sm" disabled={!canSave} onClick={() => void onSave()}>
            {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Save className="h-3.5 w-3.5" />}
            {isEdit ? "Save new version" : `Create ${kindLabel}`}
          </Button>
        </div>
      </div>

      <div className="mx-5 mt-4 flex flex-col gap-2">
        {!isEdit && (
          <div className="flex items-center gap-2">
            <span className="w-24 shrink-0 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
              Slug
            </span>
            <Input
              className="h-7 max-w-xs font-mono text-xs"
              value={entrySlug}
              placeholder="e.g. yandex"
              onChange={(e) => setEntrySlug(e.target.value)}
            />
          </div>
        )}
        <div className="flex items-start gap-2">
          <span className="mt-1 w-24 shrink-0 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
            Description
          </span>
          <textarea
            className="min-h-14 w-full max-w-xl resize-y border border-zinc-800 bg-transparent px-2 py-1.5 text-xs text-zinc-200 outline-none focus-visible:border-zinc-600"
            value={description}
            placeholder="What this bundle is for…"
            onChange={(e) => setDescription(e.target.value)}
          />
        </div>
      </div>

      {isLinked && (
        <div className="mx-5 mt-3 flex items-center gap-2 border border-blue-900/50 bg-blue-950/20 px-3 py-2 text-xs text-blue-300">
          <Link2 className="h-3.5 w-3.5 shrink-0" />
          This entry is a <strong>live link</strong> to an instance catalog entry — it
          always tracks the linked source. Saving any edit here will{" "}
          <strong>fork</strong> it into an independent copy owned by this org;
          the live link stays untouched and unaffected.
        </div>
      )}
      {nameMissing && (
        <div className="mx-5 mt-3 flex items-center gap-2 border border-amber-900/50 bg-amber-950/20 px-3 py-2 text-xs text-amber-400">
          <AlertTriangle className="h-3.5 w-3.5 shrink-0" />
          A {kindLabel} needs a name before it can be saved.
        </div>
      )}
      {saveError && (
        <div className="mx-5 mt-3 flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
          <AlertCircle className="h-4 w-4 shrink-0" /> {saveError}
        </div>
      )}

      <div className="min-h-0 flex-1 overflow-hidden px-5 py-5">
          <div className="grid h-full min-h-0 grid-cols-[14rem_minmax(0,1fr)_20rem] gap-4">
            {/* Left: file list */}
            <Card className="flex min-h-0 flex-col">
              <CardContent className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto p-2">
                <div className="px-1 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
                  Bundle files
                </div>
                {paths.map((path) => (
                  <div
                    key={path}
                    className={`group flex items-center gap-1.5 border px-2 py-1.5 text-xs transition-colors ${
                      path === activePath
                        ? "border-primary/50 bg-primary/[0.07] text-primary"
                        : "border-transparent text-zinc-400 hover:border-zinc-800 hover:bg-zinc-900/50"
                    }`}
                  >
                    <button
                      type="button"
                      onClick={() => setActivePath(path)}
                      className="flex min-w-0 flex-1 items-center gap-1.5 text-left"
                    >
                      <FileCode2 className="h-3.5 w-3.5 shrink-0" />
                      <span className="truncate font-mono">{path}</span>
                    </button>
                    {paths.length > 1 && (
                      <button
                        type="button"
                        onClick={() => removeFile(path)}
                        className="shrink-0 text-zinc-600 opacity-0 transition-colors hover:text-red-400 group-hover:opacity-100"
                        aria-label={`Remove ${path}`}
                      >
                        <X className="h-3 w-3" />
                      </button>
                    )}
                  </div>
                ))}
                <div className="mt-1 flex items-center gap-1 px-1">
                  <Input
                    className="h-7 font-mono text-[11px]"
                    placeholder="components/foo.yaml"
                    value={newFileName}
                    onChange={(e) => setNewFileName(e.target.value)}
                    onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), addFile())}
                  />
                  <Button
                    type="button"
                    variant="outline"
                    size="icon"
                    className="h-7 w-7 shrink-0"
                    disabled={!newFileName.trim() || files[newFileName.trim()] !== undefined}
                    onClick={addFile}
                    aria-label="Add file"
                  >
                    <Plus className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </CardContent>
            </Card>

            {/* Middle: DslEditor for the active file */}
            <div className="flex min-h-0 flex-col">
              <div className="mb-1.5 flex items-center justify-between px-0.5">
                <span className="truncate font-mono text-[11px] text-zinc-500">
                  {activePath || "no file selected"}
                </span>
                {checking && (
                  <span className="flex items-center gap-1 text-[10px] text-zinc-600">
                    <Loader2 className="h-3 w-3 animate-spin" /> checking…
                  </span>
                )}
              </div>
              <div className="min-h-0 flex-1 overflow-auto">
                {activePath ? (
                  <DslEditor
                    path={activePath}
                    value={files[activePath] ?? ""}
                    onChange={(v) => updateFileContent(activePath, v)}
                    files={files}
                  />
                ) : (
                  <div className="flex h-full items-center justify-center text-xs text-zinc-600">
                    Add a file to start editing.
                  </div>
                )}
              </div>
            </div>

            {/* Right: diagnostics panel */}
            <Card className="flex min-h-0 flex-col overflow-hidden">
              <div className="flex h-8 shrink-0 items-center border-b border-zinc-800/80 px-2 text-[10px] font-mono uppercase tracking-wider text-zinc-500">
                Diagnostics{diagnostics.length > 0 && ` (${diagnostics.length})`}
              </div>
              <div className="flex min-h-0 flex-1 flex-col gap-1.5 overflow-y-auto p-2">
                {diagnostics.length === 0 ? (
                  <div className="px-1 py-2 text-xs text-zinc-600">
                    {checking ? "Checking…" : "No issues found."}
                  </div>
                ) : (
                  diagnostics.map((d, i) => (
                    <button
                      key={`${d.path}:${d.line}:${d.col}:${i}`}
                      type="button"
                      onClick={() => files[d.path] !== undefined && setActivePath(d.path)}
                      className="flex items-start gap-1.5 border border-zinc-800 bg-[#0a0a0a] px-2 py-1.5 text-left text-xs transition-colors hover:border-zinc-700"
                    >
                      {d.severity === "error" ? (
                        <AlertCircle className="mt-0.5 h-3.5 w-3.5 shrink-0 text-red-400" />
                      ) : (
                        <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0 text-amber-400" />
                      )}
                      <span className="min-w-0 flex-1">
                        <span className="block truncate font-mono text-[10px] text-zinc-500">
                          {d.path}:{d.line}:{d.col}
                          {d.module && ` · ${d.module}`}
                        </span>
                        <span className="block text-zinc-300">{d.message}</span>
                      </span>
                    </button>
                  ))
                )}
              </div>
            </Card>
          </div>
      </div>
    </div>
  );
}
