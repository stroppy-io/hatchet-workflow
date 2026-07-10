// RecipeEditor — cloud.v1.api.RecipeService's editor.
//
// EDIT (isEdit === true, /recipes/:id): renders the embedded code-server IDE
// for the recipe's own git repo (SP-C: "рецепт = тоже репозиторий" — recipes
// get the same one-repo-per-item model providers/workflows already have, see
// internal/services/recipe.recipeBundleIdentity's doc). A row's file bytes
// live in that repo now (source_ref, resolved server-side — see
// internal/services/recipe/recipe.go's resolveBundle), so the IDE opens the
// SAME physical repo GetRecipe would resolve, and edits made there are the
// recipe's real content going forward. There is no in-page Save anymore for
// an existing recipe: the IDE (code-server's LSP, backed by DslService.Check)
// is the one editing+diagnostics surface, mirroring CatalogEntryEditor.tsx's
// EDIT branch exactly (see ide.ts's recipeIdeUrl for the scope-string
// construction and @/components/ide/EmbeddedIde for the shared
// ticket->iframe handshake).
//
// CREATE (isEdit === false, /recipes/new): a brand-new recipe has no repo
// yet, so there is nothing for an IDE to open until the first Create call
// makes one. This path keeps the original file-tabs + DslEditor +
// check-as-you-type form, then navigates straight into the (now-real)
// recipe's edit route, landing in the IDE — exactly CatalogEntryEditor.tsx's
// CREATE branch, generalized from a slug to a name (a recipe has no slug
// field; RecipeRecord's identity is entity.name, mirrored 1:1 into the repo
// path via recipeBundleIdentity).
//
// VERSIONING: creating "again" under the same name is still how a new
// version is authored server-side (nextVersion = latest-for-name + 1) — but
// now that editing goes through the IDE (a real git repo, real commits),
// there is no more separate "Save new version" action from this page: once a
// recipe exists, its repo IS the editable surface, and its version history
// lives in that repo's own commits, mirroring what the catalog redesign
// already did for providers/workflows.
//
// RUN NAVIGATION TARGET: Run navigates to the generated launch form at
// `/recipes/:id/launch` (LaunchForm.tsx), which fetches the composed
// schemapb form schema (RecipeService.LaunchFormSchema) and calls startRun
// itself once the form is submitted. LaunchForm then navigates to
// `/runs/${runId}` — router.tsx's tenant-prefixing turns this into
// `/t/:slug/runs/${runId}`, landing on the existing RunDetail page.

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  AlertCircle,
  AlertTriangle,
  ArrowLeft,
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
import { DslEditor } from "@/components/ui/dsl-editor";
import { EmbeddedIde } from "@/components/ide/EmbeddedIde";
import { checkBundle, createRecipe, getRecipe, type DiagnosticVM, type RecipeVM } from "@/services/recipe";
import { recipeIdeUrl } from "@/services/ide";

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

export function RecipeEditor() {
  const slug = useTenantSlug() ?? "";
  const { id } = useParams();
  const navigate = useNavigate();
  const isEdit = !!id;

  useBreadcrumbLabel("id", isEdit ? id : undefined);

  if (isEdit && id) {
    return <RecipeIdeView slug={slug} id={id} navigate={navigate} />;
  }
  return <RecipeCreateForm slug={slug} navigate={navigate} />;
}

// --- EDIT: the embedded IDE -------------------------------------------------

interface RecipeIdeViewProps {
  slug: string;
  id: string;
  navigate: (path: string, opts?: { replace?: boolean }) => void;
}

function RecipeIdeView({ slug, id, navigate }: RecipeIdeViewProps) {
  const [recipe, setRecipe] = useState<RecipeVM | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  useBreadcrumbLabel("id", recipe?.name || id);

  useEffect(() => {
    if (!slug) return undefined;
    let cancelled = false;
    setLoading(true);
    setLoadError(null);
    getRecipe(slug, id)
      .then((r) => {
        if (!cancelled) setRecipe(r);
      })
      .catch((e) => !cancelled && setLoadError(e instanceof Error ? e.message : String(e)))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [slug, id]);

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-zinc-500">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" /> Loading recipe…
      </div>
    );
  }
  if (loadError || !recipe) {
    return (
      <div className="p-5">
        <div className="flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
          <AlertCircle className="h-4 w-4" /> {loadError ?? "recipe not found"}
        </div>
      </div>
    );
  }

  const ideTarget = recipeIdeUrl(slug, recipe.name);

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between border-b border-zinc-800/80 px-5 py-3">
        <div className="flex min-w-0 items-center gap-3">
          <button
            type="button"
            onClick={() => navigate("/recipes")}
            className="flex h-7 w-7 shrink-0 items-center justify-center border border-zinc-800 text-zinc-500 transition-colors hover:border-zinc-700 hover:text-zinc-300"
            aria-label="Back"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>
          <div className="min-w-0">
            <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">Edit recipe</div>
            <div className="mt-0.5 truncate text-base font-semibold tracking-tight text-foreground">
              {recipe.name}
            </div>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant="outline">v{recipe.version}</Badge>
          <Button
            variant="outline"
            size="sm"
            onClick={() => navigate(`/recipes/${id}/launch`)}
          >
            <Play className="h-3.5 w-3.5" />
            Run
          </Button>
          <Button variant="outline" size="sm" onClick={() => navigate("/recipes")}>
            Close
          </Button>
        </div>
      </div>
      <div className="min-h-0 flex-1">
        <EmbeddedIde targetUrl={ideTarget} entryLabel={recipe.name} />
      </div>
    </div>
  );
}

// --- CREATE: file-tabs + DslEditor form -------------------------------------

interface RecipeCreateFormProps {
  slug: string;
  navigate: (path: string, opts?: { replace?: boolean }) => void;
}

function RecipeCreateForm({ slug, navigate }: RecipeCreateFormProps) {
  const [name, setName] = useState("");
  const [files, setFiles] = useState<Record<string, string>>(() => ({ ...NEW_RECIPE_TEMPLATE }));
  const [activePath, setActivePath] = useState(() => firstPath(NEW_RECIPE_TEMPLATE));

  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  const [diagnostics, setDiagnostics] = useState<DiagnosticVM[]>([]);
  const [checking, setChecking] = useState(false);

  const [newFileName, setNewFileName] = useState("");

  // Bundle-wide diagnostics: debounced re-check on any edit. CREATE only —
  // once a recipe exists, the IDE's own LSP (DslService.Check under the
  // hood) is the one diagnostics surface; a parallel checker here would
  // drift from the compiler.
  useEffect(() => {
    if (Object.keys(files).length === 0) return undefined;
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
  }, [files, slug]);

  const nameMissing = !name.trim();
  const canCreate = !creating && !nameMissing && Object.keys(files).length > 0;

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

  const onCreate = useCallback(async () => {
    if (!slug) return;
    setCreating(true);
    setCreateError(null);
    try {
      const saved = await createRecipe(slug, { name: name.trim(), files });
      navigate(`/recipes/${saved.id}`);
    } catch (e) {
      setCreateError(e instanceof Error ? e.message : String(e));
    } finally {
      setCreating(false);
    }
  }, [slug, name, files, navigate]);

  const paths = useMemo(() => Object.keys(files).sort(), [files]);
  const errorCount = diagnostics.filter((d) => d.severity === "error").length;
  const warningCount = diagnostics.filter((d) => d.severity === "warning").length;

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
            <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">New recipe</div>
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
          <Button variant="outline" size="sm" onClick={() => navigate("/recipes")}>
            Cancel
          </Button>
          <Button size="sm" disabled={!canCreate} onClick={() => void onCreate()}>
            {creating ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Save className="h-3.5 w-3.5" />}
            Create recipe
          </Button>
        </div>
      </div>

      {nameMissing && (
        <div className="mx-5 mt-4 flex items-center gap-2 border border-amber-900/50 bg-amber-950/20 px-3 py-2 text-xs text-amber-400">
          <AlertTriangle className="h-3.5 w-3.5 shrink-0" />
          A recipe needs a name before it can be created.
        </div>
      )}
      {createError && (
        <div className="mx-5 mt-4 flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
          <AlertCircle className="h-4 w-4 shrink-0" /> {createError}
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
                <div className="px-1 py-2 text-xs text-zinc-600">{checking ? "Checking…" : "No issues found."}</div>
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
