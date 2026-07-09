// RecipeEditor — multi-file bundle editor for cloud.v1.api.RecipeService.
//
// Serves BOTH create (/recipes/new) and edit (/recipes/:id): with no :id the
// page seeds a minimal cluster.yaml/workflow.yaml template; with an :id it
// loads the persisted bundle via getRecipe. Mirrors DatabasePresetForm.tsx's
// header (back button + name + Save/Cancel) and page chrome, but the body is
// a file-list + DslEditor + bundle-wide diagnostics layout instead of a form.
//
// VERSIONING: RecipeService has no update RPC (see recipe.ts / recipe.proto —
// CreateRecipeRequest carries no id). Every Save calls createRecipe(slug,
// {name, files}), which the backend treats as a brand-new row: a fresh
// entity.id, and version = (latest existing version for that `name`) + 1 (see
// internal/services/recipe/recipe.go nextVersion). So "editing" a recipe is
// really "author a new version under the same name" — Save always produces a
// NEW id, and the page navigates to that new id's route afterward (matching
// DatabasePresetForm's post-save navigate-to-the-saved-entity pattern). Run is
// gated on the CURRENTLY LOADED saved version having no unsaved edits, since
// starting a run against `id` runs exactly that stored bundle — running while
// dirty would silently launch a stale version.
//
// RUN NAVIGATION TARGET: Run no longer calls startRun directly — it navigates
// to the generated launch form at `/recipes/:id/launch` (LaunchForm.tsx),
// which fetches the composed schemapb form schema (RecipeService.
// LaunchFormSchema) and calls startRun itself once the form is submitted
// (BakeForm seals a Baked snapshot even when the form composes to zero
// required fields). LaunchForm then navigates to `/runs/${runId}` —
// router.tsx's tenant-prefixing turns this into `/t/:slug/runs/${runId}`,
// landing on the existing RunDetail page.

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  AlertCircle,
  AlertTriangle,
  ArrowLeft,
  Eye,
  FileCode2,
  Loader2,
  Play,
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useConfirm } from "@/components/ui/confirm-dialog";
import { DslEditor } from "@/components/ui/dsl-editor";
import {
  checkBundle,
  createRecipe,
  getRecipe,
  previewBundle,
  type DiagnosticVM,
  type PreviewPlanVM,
} from "@/services/recipe";

// A minimal, intentionally-not-guaranteed-to-compile starting point for a
// brand-new recipe. It gives the user two real files to edit rather than a
// blank bundle; the diagnostics panel + DslEditor will flag whatever's still
// missing (e.g. an empty `services: {}` block).
const NEW_RECIPE_TEMPLATE: Record<string, string> = {
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

function firstPath(files: Record<string, string>): string {
  return Object.keys(files).sort()[0] ?? "";
}

/** Cheap dirty-check: compares the current name+files against a JSON snapshot
 * taken at load / after the last successful save. Bundles are small text
 * files, so stringify-compare is fine — no need for a diff. */
function snapshotOf(name: string, files: Record<string, string>): string {
  return JSON.stringify({ name, files });
}

export function RecipeEditor() {
  const slug = useTenantSlug() ?? "";
  const { id } = useParams();
  const navigate = useNavigate();
  const confirm = useConfirm();
  const isEdit = !!id;

  const [name, setName] = useState("");
  const [files, setFiles] = useState<Record<string, string>>(() =>
    isEdit ? {} : { ...NEW_RECIPE_TEMPLATE },
  );
  const [activePath, setActivePath] = useState(() => (isEdit ? "" : firstPath(NEW_RECIPE_TEMPLATE)));

  const [loading, setLoading] = useState(isEdit);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const [diagnostics, setDiagnostics] = useState<DiagnosticVM[]>([]);
  const [checking, setChecking] = useState(false);

  // Compile-plan preview: on-demand (a "Preview" button), NOT debounced —
  // Check already re-runs on every edit for the diagnostics panel above; a
  // full compile-to-plan on every keystroke would be redundant work for a
  // panel the user only consults right before Run.
  const [previewPlan, setPreviewPlan] = useState<PreviewPlanVM | null>(null);
  const [previewDiagnostics, setPreviewDiagnostics] = useState<DiagnosticVM[]>([]);
  const [previewing, setPreviewing] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [previewRequested, setPreviewRequested] = useState(false);
  const previewSnapshotRef = useRef<string | null>(null);

  const [newFileName, setNewFileName] = useState("");

  // Snapshot of the last-saved (or last-loaded) state, used to gate Run on
  // "no unsaved edits since the currently-loaded id was stored".
  const savedSnapshotRef = useRef<string | null>(null);

  useBreadcrumbLabel("id", isEdit ? name || id : undefined);

  // Load the persisted bundle in edit mode.
  useEffect(() => {
    if (!isEdit || !slug || !id) return;
    let cancelled = false;
    setLoading(true);
    setLoadError(null);
    getRecipe(slug, id)
      .then((recipe) => {
        if (cancelled) return;
        setName(recipe.name);
        setFiles(recipe.files);
        setActivePath(firstPath(recipe.files));
        savedSnapshotRef.current = snapshotOf(recipe.name, recipe.files);
      })
      .catch((e) => !cancelled && setLoadError(e instanceof Error ? e.message : String(e)))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [isEdit, slug, id]);

  // Bundle-wide diagnostics panel: debounced ~500ms re-check on any file
  // edit. Separate from DslEditor's own per-file linter (400ms), which only
  // ever shows the diagnostics belonging to the currently-open file inline.
  useEffect(() => {
    if (loading || Object.keys(files).length === 0) return;
    let cancelled = false;
    setChecking(true);
    const t = setTimeout(() => {
      checkBundle(files, slug)
        .then((diags) => !cancelled && setDiagnostics(diags))
        .catch((e) => {
          if (cancelled) return;
          console.warn("RecipeEditor: checkBundle failed", e);
          setDiagnostics([]);
        })
        .finally(() => !cancelled && setChecking(false));
    }, 500);
    return () => {
      cancelled = true;
      clearTimeout(t);
    };
  }, [files, loading, slug]);

  const dirty = useMemo(() => {
    if (savedSnapshotRef.current === null) return true; // never saved yet
    return snapshotOf(name, files) !== savedSnapshotRef.current;
  }, [name, files]);

  // Has the bundle changed since the last Preview run? Compared against
  // savedSnapshotRef's sibling (previewSnapshotRef), so an edit after
  // previewing flags the shown plan as stale without re-running the compile.
  const previewStale = useMemo(() => {
    if (!previewRequested || previewSnapshotRef.current === null) return false;
    return snapshotOf(name, files) !== previewSnapshotRef.current;
  }, [name, files, previewRequested]);

  const onPreview = useCallback(async () => {
    setPreviewing(true);
    setPreviewError(null);
    setPreviewRequested(true);
    try {
      const { plan, diagnostics: previewDiags } = await previewBundle(files, slug);
      setPreviewPlan(plan);
      setPreviewDiagnostics(previewDiags);
      previewSnapshotRef.current = snapshotOf(name, files);
    } catch (e) {
      setPreviewError(e instanceof Error ? e.message : String(e));
      setPreviewPlan(null);
    } finally {
      setPreviewing(false);
    }
  }, [files, name, slug]);

  const nameMissing = !name.trim();
  const canSave = !saving && !nameMissing && Object.keys(files).length > 0;
  const canRun = !!id && !dirty;

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
    async (path: string) => {
      const remaining = Object.keys(files).filter((p) => p !== path);
      if (remaining.length === 0) return; // never remove the last file
      const ok = await confirm({
        title: "Remove file?",
        description: `"${path}" will be removed from the bundle.`,
        danger: true,
        confirmLabel: "Remove",
      });
      if (!ok) return;
      setFiles((cur) => {
        const next = { ...cur };
        delete next[path];
        return next;
      });
      if (activePath === path) setActivePath(remaining.sort()[0]);
    },
    [files, activePath, confirm],
  );

  const onSave = useCallback(async () => {
    if (!slug) return;
    setSaving(true);
    setSaveError(null);
    try {
      const saved = await createRecipe(slug, { name: name.trim(), files });
      savedSnapshotRef.current = snapshotOf(saved.name, saved.files);
      // TODO: post-save reflash is minor: could update local state from saved instead of forcing reload.
      navigate(`/recipes/${saved.id}`);
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  }, [slug, name, files, navigate]);

  const onRun = useCallback(() => {
    if (!slug || !id) return;
    navigate(`/recipes/${id}/launch`);
  }, [slug, id, navigate]);

  const paths = useMemo(() => Object.keys(files).sort(), [files]);
  const errorCount = diagnostics.filter((d) => d.severity === "error").length;
  const warningCount = diagnostics.filter((d) => d.severity === "warning").length;

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-zinc-500">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" /> Loading recipe…
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
            onClick={() => navigate("/recipes")}
            className="flex h-7 w-7 shrink-0 items-center justify-center border border-zinc-800 text-zinc-500 transition-colors hover:border-zinc-700 hover:text-zinc-300"
            aria-label="Back"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>
          <div className="min-w-0">
            <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">
              {isEdit ? `Edit recipe${dirty ? " — unsaved changes" : ""}` : "New recipe"}
            </div>
            <Input
              className="mt-0.5 h-7 max-w-xs border-none bg-transparent px-0 text-base font-semibold tracking-tight shadow-none focus-visible:ring-0"
              value={name}
              placeholder="e.g. PG 3-node smoke"
              onChange={(e) => setName(e.target.value)}
            />
          </div>
        </div>
        <div className="flex items-center gap-2">
          {errorCount > 0 && <Badge variant="destructive">{errorCount} error{errorCount === 1 ? "" : "s"}</Badge>}
          {warningCount > 0 && <Badge variant="warning">{warningCount} warning{warningCount === 1 ? "" : "s"}</Badge>}
          <Button
            variant="outline"
            size="sm"
            onClick={() => navigate("/recipes")}
          >
            Cancel
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={!canRun}
            title={!id ? "Save the recipe before running it" : dirty ? "Save changes before running" : undefined}
            onClick={onRun}
          >
            <Play className="h-3.5 w-3.5" />
            Run
          </Button>
          <Button size="sm" disabled={!canSave} onClick={() => void onSave()}>
            {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Save className="h-3.5 w-3.5" />}
            {isEdit ? "Save new version" : "Create recipe"}
          </Button>
        </div>
      </div>

      {nameMissing && (
        <div className="mx-5 mt-4 flex items-center gap-2 border border-amber-900/50 bg-amber-950/20 px-3 py-2 text-xs text-amber-400">
          <AlertTriangle className="h-3.5 w-3.5 shrink-0" />
          A recipe needs a name before it can be saved.
        </div>
      )}
      {saveError && (
        <div className="mx-5 mt-4 flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
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
                      onClick={() => void removeFile(path)}
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

          {/* Right: diagnostics + compile-plan preview panel */}
          <Card className="flex min-h-0 flex-col overflow-hidden">
            <Tabs defaultValue="diagnostics" className="flex min-h-0 flex-1 flex-col">
              <TabsList className="h-8 shrink-0 justify-start gap-1 border-b border-zinc-800/80 bg-transparent p-1">
                <TabsTrigger
                  value="diagnostics"
                  className="h-6 px-2 text-[10px] font-mono uppercase tracking-wider data-[state=active]:bg-zinc-900 data-[state=active]:text-zinc-200"
                >
                  Diagnostics{diagnostics.length > 0 && ` (${diagnostics.length})`}
                </TabsTrigger>
                <TabsTrigger
                  value="preview"
                  className="h-6 px-2 text-[10px] font-mono uppercase tracking-wider data-[state=active]:bg-zinc-900 data-[state=active]:text-zinc-200"
                >
                  Preview
                </TabsTrigger>
              </TabsList>

              <TabsContent
                value="diagnostics"
                className="mt-0 flex min-h-0 flex-1 flex-col gap-1.5 overflow-y-auto p-2"
              >
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
              </TabsContent>

              <TabsContent
                value="preview"
                className="mt-0 flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto p-2"
              >
                <div className="flex items-center gap-2 px-1">
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    className="h-7 flex-1"
                    disabled={previewing || Object.keys(files).length === 0}
                    onClick={() => void onPreview()}
                  >
                    {previewing ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <Eye className="h-3.5 w-3.5" />
                    )}
                    {previewRequested ? "Refresh preview" : "Preview plan"}
                  </Button>
                  {previewStale && !previewing && (
                    <Badge variant="warning" className="shrink-0">stale</Badge>
                  )}
                </div>

                {previewError && (
                  <div className="mx-1 flex items-center gap-1.5 border border-red-900/50 bg-red-950/30 px-2 py-1.5 text-xs text-red-400">
                    <AlertCircle className="h-3.5 w-3.5 shrink-0" /> {previewError}
                  </div>
                )}

                {!previewRequested && !previewError && (
                  <div className="px-1 py-2 text-xs text-zinc-600">
                    Compile the bundle to see what will be provisioned before running it.
                  </div>
                )}

                {previewRequested && !previewing && !previewError && !previewPlan && (
                  <div className="px-1 py-2 text-xs text-zinc-600">
                    {previewDiagnostics.some((d) => d.severity === "error")
                      ? "The bundle has compile errors — fix them (see Diagnostics) to preview a plan."
                      : "No plan available."}
                  </div>
                )}

                {previewPlan && (
                  <div className="flex flex-col gap-3 px-1">
                    <div className="flex items-center justify-between text-xs">
                      <span className="text-zinc-500">Provider</span>
                      <span className="font-mono text-zinc-200">{previewPlan.provider || "—"}</span>
                    </div>
                    <div className="flex items-center justify-between text-xs">
                      <span className="text-zinc-500">Nodes</span>
                      <span className="font-mono text-zinc-200">
                        {previewPlan.nodeTotal} across {previewPlan.machineGroups.length} group
                        {previewPlan.machineGroups.length === 1 ? "" : "s"}
                      </span>
                    </div>

                    <div>
                      <div className="mb-1 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
                        Machine groups
                      </div>
                      <div className="flex flex-col gap-1">
                        {previewPlan.machineGroups.map((g) => (
                          <div
                            key={g.name}
                            className="border border-zinc-800 bg-[#0a0a0a] px-2 py-1.5 text-xs"
                          >
                            <div className="flex items-center justify-between">
                              <span className="font-mono text-zinc-200">{g.name}</span>
                              <span className="text-zinc-500">×{g.count}</span>
                            </div>
                            <div className="text-[10px] text-zinc-500">
                              {g.cpu} vCPU · {(g.ramMb / 1024).toFixed(1)} GB RAM
                              {g.diskGb > 0 && ` · ${g.diskGb} GB disk`}
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>

                    <div>
                      <div className="mb-1 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
                        Services
                      </div>
                      <div className="flex flex-col gap-1">
                        {previewPlan.services.map((s) => (
                          <div
                            key={s.name}
                            className="border border-zinc-800 bg-[#0a0a0a] px-2 py-1.5 text-xs"
                          >
                            <div className="flex items-center justify-between">
                              <span className="font-mono text-zinc-200">{s.name}</span>
                              <span className="text-zinc-500">on {s.onGroup}</span>
                            </div>
                            <div className="truncate text-[10px] text-zinc-500">{s.image}</div>
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>
                )}
              </TabsContent>
            </Tabs>
          </Card>
        </div>
      </div>
    </div>
  );
}
