// CatalogEntryEditor — multi-file bundle editor for cloud.v1.catalog.CatalogService,
// shared between the instance scope (/admin/catalog/...) and the org scope
// (/t/:slug/catalog/...).
//
// EDIT (isEdit === true): renders the embedded code-server IDE for the
// entry's own git repo (spec SP-C: "каждый провайдер или воркфлоу это
// отдельный репозиторий полностью"). CatalogEntry carries no file contents
// of its own — only an opaque source_ref pointing at that repo (see
// catalog/models.proto's doc) — so the IDE opens the SAME physical repo
// GetInstanceEntryFiles/GetOrgProviderFiles/GetOrgWorkflowFiles would read,
// and edits made there are the entry's real content going forward. There is
// no in-page Save anymore: the IDE (via code-server's LSP, backed by
// DslService.Check) is the one editing+diagnostics surface — a parallel
// validator here would drift from the compiler exactly the way the launch
// form did across nine "green" tasks. See ide.ts's instanceIdeUrl/orgIdeUrl
// for the scope-string construction (singular kind — the historical
// plural-tab-name 404 bug) and the ticket->iframe handshake in EmbeddedIde
// below.
//
// CREATE (isEdit === false): a brand-new entry has no repo yet, so there is
// nothing for an IDE to open until the first Create call makes one. This
// path keeps the original file-tabs + DslEditor + check-as-you-type form,
// then navigates straight into the (now-real) entry's edit route, landing
// in the IDE.
//
// ORIGIN_LINKED (org scope only): a linked row has no files of its own yet
// (org repos start empty — the spec's read-through model; see
// internal/services/catalog.GitBundleStore's package doc) until it is
// forked, so there is no repo for the IDE to open. There is no separate
// fork RPC (see services/catalog.ts's updateOrgEntry doc) — forking happens
// by calling UpdateOrgProvider/UpdateOrgWorkflow with the entry's own
// unmodified files, which the server turns into a new FORKED row. Rather
// than doing that invisibly, LinkedGuard shows an explicit "this will fork"
// screen the user must click through — preserving the same warning
// OrgCatalog.tsx's Edit button title already gives before you ever get
// here.

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
  checkCatalogBundle,
  checkInstanceBundle,
  createInstanceEntry,
  createOrgEntry,
  getInstanceEntry,
  getOrgEntry,
  getOrgEntryFiles,
  updateOrgEntry,
  type CatalogDiagnosticVM,
  type CatalogEntryVM,
  type CatalogKind,
} from "@/services/catalog";
import { instanceIdeUrl, mintIdeTicket, orgIdeUrl, type IdeEntryTab } from "@/services/ide";

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

// --- EmbeddedIde: the ticket -> iframe handshake --------------------------
//
// Sequence: mintIdeTicket(targetUrl) authenticates with the SPA's Bearer
// token (a fetch, which CAN carry it) and returns a single-use ticket-
// bearing URL; that URL is set as the iframe's src. The iframe's navigation
// carries no Authorization header (browser navigations never do — that is
// exactly why the ticket exists), but it IS same-origin with the SPA (the
// url is always a relative "/ide/..." path — see ide.ts's IdeTicket doc),
// so it is not a cross-site request. The gateway exchanges the ticket for
// an httpOnly cookie scoped to Path=/ide/<scope> with SameSite=Lax (see
// internal/ide/ticket.go's TicketExchanger) and 302s the iframe to the
// clean URL; the browser then attaches that cookie to every subsequent
// request the iframe makes into /ide/<scope>/... (assets, the code-server
// websocket). SameSite=Lax only restricts CROSS-SITE requests — a
// same-origin iframe subresource/navigation is not cross-site, so Lax does
// not block it here (Lax would only matter if the IDE were embedded from a
// different origin, which this never is).
//
// The FIRST open of a scope can take ~15s (EnsureRunning blocks the
// gateway's HTTP response until the code-server container is up — see
// internal/ide/backend.go), so the response to the iframe's navigation
// itself is slow, not just slow-to-render: the iframe's `load` event simply
// does not fire until the container is ready. The overlay below stays up
// until `load` fires and swaps its message after a few seconds so a slow
// first start reads as "working", not "hung".
function classifyIdeError(message: string): "disabled" | "forbidden" | "other" {
  if (message.includes("404")) return "disabled";
  if (message.includes("403")) return "forbidden";
  return "other";
}

interface EmbeddedIdeProps {
  targetUrl: string;
  entryLabel: string;
}

function EmbeddedIde({ targetUrl, entryLabel }: EmbeddedIdeProps) {
  const [ticketUrl, setTicketUrl] = useState<string | null>(null);
  const [minting, setMinting] = useState(true);
  const [mintError, setMintError] = useState<{ kind: "disabled" | "forbidden" | "other"; message: string } | null>(
    null,
  );
  const [iframeLoaded, setIframeLoaded] = useState(false);
  const [slowStart, setSlowStart] = useState(false);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setMinting(true);
    setMintError(null);
    setTicketUrl(null);
    setIframeLoaded(false);
    setSlowStart(false);
    mintIdeTicket(targetUrl)
      .then((res) => {
        if (!cancelled) setTicketUrl(res.url);
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        const message = e instanceof Error ? e.message : String(e);
        setMintError({ kind: classifyIdeError(message), message });
      })
      .finally(() => !cancelled && setMinting(false));
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [targetUrl, attempt]);

  useEffect(() => {
    if (!ticketUrl || iframeLoaded) return undefined;
    const t = setTimeout(() => setSlowStart(true), 4000);
    return () => clearTimeout(t);
  }, [ticketUrl, iframeLoaded]);

  if (minting) {
    return (
      <div className="flex h-full items-center justify-center gap-2 text-sm text-zinc-500">
        <Loader2 className="h-4 w-4 animate-spin" /> Preparing your IDE session…
      </div>
    );
  }

  if (mintError) {
    const heading =
      mintError.kind === "disabled"
        ? "The embedded IDE is not enabled on this server."
        : mintError.kind === "forbidden"
          ? "You are not authorized to open this entry in the IDE."
          : "Could not open the IDE.";
    const hint =
      mintError.kind === "disabled"
        ? "Ask an administrator to enable IDE_MANAGER_ENABLED and configure the Gitea backend."
        : mintError.kind === "forbidden"
          ? "This requires an update grant on this resource."
          : mintError.message;
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 p-8 text-center">
        <div className="flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
          <AlertCircle className="h-4 w-4 shrink-0" /> {heading}
        </div>
        <p className="max-w-md text-xs text-zinc-500">{hint}</p>
        <Button size="sm" variant="outline" onClick={() => setAttempt((n) => n + 1)}>
          Retry
        </Button>
      </div>
    );
  }

  return (
    <div className="relative h-full w-full">
      <iframe
        key={ticketUrl}
        src={ticketUrl ?? undefined}
        title={`${entryLabel} — IDE`}
        onLoad={() => setIframeLoaded(true)}
        className="h-full w-full border-0"
        sandbox="allow-scripts allow-same-origin allow-popups allow-forms allow-downloads allow-modals"
        data-testid="ide-iframe"
      />
      {!iframeLoaded && (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-background/90">
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
          <span className="font-mono text-xs text-muted-foreground">
            {slowStart ? "Starting your IDE session — first open can take up to ~15s…" : "Opening IDE…"}
          </span>
        </div>
      )}
    </div>
  );
}

// --- LinkedGuard: fork-then-open for ORIGIN_LINKED org entries ------------

interface LinkedGuardProps {
  kindLabel: string;
  entrySlug: string;
  forking: boolean;
  forkError: string | null;
  onFork: () => void;
}

function LinkedGuard({ kindLabel, entrySlug, forking, forkError, onFork }: LinkedGuardProps) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-4 p-8 text-center">
      <div className="flex max-w-md items-center gap-2 border border-blue-900/50 bg-blue-950/20 px-3 py-2 text-xs text-blue-300">
        <Link2 className="h-3.5 w-3.5 shrink-0" />
        &quot;{entrySlug}&quot; is a <strong>live link</strong> to the instance catalog — it has no files of its own
        for the IDE to open yet.
      </div>
      <p className="max-w-md text-xs text-zinc-500">
        Opening this {kindLabel} in the IDE forks it into an independent copy owned by this org first. The live link
        and the instance source are never mutated.
      </p>
      {forkError && (
        <div className="flex max-w-md items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
          <AlertCircle className="h-4 w-4 shrink-0" /> {forkError}
        </div>
      )}
      <Button size="sm" disabled={forking} onClick={onFork}>
        {forking ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <GitFork className="h-3.5 w-3.5" />}
        {forking ? "Forking…" : "Fork & open in IDE"}
      </Button>
    </div>
  );
}

// --- CatalogEntryEditor ----------------------------------------------------

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
  const tab: IdeEntryTab = kindParam === "workflows" ? "workflows" : "providers";
  const kindLabel = kind === "KIND_PROVIDER" ? "provider" : "workflow";
  const basePath = scope === "instance" ? "/admin/catalog" : "/catalog";

  const [entry, setEntry] = useState<CatalogEntryVM | null>(null);
  const [loading, setLoading] = useState(isEdit);
  const [loadError, setLoadError] = useState<string | null>(null);

  useBreadcrumbLabel("id", isEdit ? entry?.name || entry?.slug || id : undefined);

  useEffect(() => {
    if (!isEdit || !id) return undefined;
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
      .then((e) => {
        if (!cancelled) setEntry(e);
      })
      .catch((e) => !cancelled && setLoadError(e instanceof Error ? e.message : String(e)))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [isEdit, id, scope, slug, kind]);

  // --- fork-then-open (ORIGIN_LINKED org entries only) ---------------------

  const [forking, setForking] = useState(false);
  const [forkError, setForkError] = useState<string | null>(null);

  const onForkAndOpen = useCallback(async () => {
    if (!entry || scope !== "org" || !slug) return;
    setForking(true);
    setForkError(null);
    try {
      const bundleFiles = await getOrgEntryFiles(slug, kind, entry.id);
      const forked = await updateOrgEntry(slug, kind, entry.id, bundleFiles);
      navigate(`${basePath}/${kindParam}/${forked.id}/edit`, { replace: true });
    } catch (e) {
      setForkError(e instanceof Error ? e.message : String(e));
    } finally {
      setForking(false);
    }
  }, [entry, scope, slug, kind, navigate, basePath, kindParam]);

  // --- CREATE-only state (a brand-new entry has no repo for the IDE yet) --

  const [entrySlug, setEntrySlug] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [files, setFiles] = useState<Record<string, string>>(() => (isEdit ? {} : templateFor(kind)));
  const [activePath, setActivePath] = useState(() => (isEdit ? "" : firstPath(templateFor(kind))));
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [diagnostics, setDiagnostics] = useState<CatalogDiagnosticVM[]>([]);
  const [checking, setChecking] = useState(false);
  const [newFileName, setNewFileName] = useState("");

  // Bundle-wide diagnostics: debounced re-check on any edit. CREATE only —
  // once an entry exists, the IDE's own LSP (DslService.Check under the
  // hood) is the one diagnostics surface; a parallel checker here would
  // drift from the compiler.
  useEffect(() => {
    if (isEdit) return undefined;
    if (Object.keys(files).length === 0) return undefined;
    if (scope === "org" && !slug) return undefined;
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
  }, [isEdit, files, slug, kind, scope]);

  const slugMissing = !entrySlug.trim();
  const nameMissing = !name.trim();
  const canCreate = !creating && !nameMissing && !slugMissing && Object.keys(files).length > 0;

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
    setCreating(true);
    setCreateError(null);
    try {
      const input = { slug: entrySlug.trim(), name: name.trim(), description, files };
      const saved =
        scope === "instance" ? await createInstanceEntry(kind, input) : await createOrgEntry(slug, kind, input);
      navigate(`${basePath}/${kindParam}/${saved.id}/edit`);
    } catch (e) {
      setCreateError(e instanceof Error ? e.message : String(e));
    } finally {
      setCreating(false);
    }
  }, [scope, kind, slug, entrySlug, name, description, files, navigate, basePath, kindParam]);

  const paths = useMemo(() => Object.keys(files).sort(), [files]);
  const errorCount = diagnostics.filter((d) => d.severity === "error").length;
  const warningCount = diagnostics.filter((d) => d.severity === "warning").length;

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

  // --- EDIT: the embedded IDE ---------------------------------------------

  if (isEdit && entry) {
    const isLinked = scope === "org" && entry.origin === "ORIGIN_LINKED";
    const isForked = entry.origin === "ORIGIN_FORKED";
    const ideTarget = scope === "instance" ? instanceIdeUrl(tab, entry.slug) : orgIdeUrl(slug, tab, entry.slug);

    return (
      <div className="flex h-full flex-col">
        <div className="flex items-center justify-between border-b border-zinc-800/80 px-5 py-3">
          <div className="flex min-w-0 items-center gap-3">
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
                Edit {kindLabel}
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
              <div className="mt-0.5 truncate text-base font-semibold tracking-tight text-foreground">
                {entry.name || entry.slug}
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Badge variant="outline">v{entry.version}</Badge>
            <Button variant="outline" size="sm" onClick={() => navigate(`${basePath}?tab=${kindParam}`)}>
              Close
            </Button>
          </div>
        </div>

        <div className="min-h-0 flex-1">
          {isLinked ? (
            <LinkedGuard
              kindLabel={kindLabel}
              entrySlug={entry.slug}
              forking={forking}
              forkError={forkError}
              onFork={() => void onForkAndOpen()}
            />
          ) : (
            <EmbeddedIde targetUrl={ideTarget} entryLabel={entry.name || entry.slug} />
          )}
        </div>
      </div>
    );
  }

  // --- CREATE: file-tabs + DslEditor form ---------------------------------

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between border-b border-zinc-800/80 px-5 py-3">
        <div className="flex min-w-0 items-center gap-3">
          <button
            type="button"
            onClick={() => navigate(`${basePath}?tab=${kindParam}`)}
            className="flex h-7 w-7 shrink-0 items-center justify-center border border-zinc-800 text-zinc-500 transition-colors hover:border-zinc-700 hover:text-zinc-300"
            aria-label="Back"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>
          <div className="min-w-0">
            <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">New {kindLabel}</div>
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
          <Button size="sm" disabled={!canCreate} onClick={() => void onCreate()}>
            {creating ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Save className="h-3.5 w-3.5" />}
            Create {kindLabel}
          </Button>
        </div>
      </div>

      <div className="mx-5 mt-4 flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <span className="w-24 shrink-0 text-[10px] font-mono uppercase tracking-wider text-zinc-600">Slug</span>
          <Input
            className="h-7 max-w-xs font-mono text-xs"
            value={entrySlug}
            placeholder="e.g. yandex"
            onChange={(e) => setEntrySlug(e.target.value)}
          />
        </div>
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

      {nameMissing && (
        <div className="mx-5 mt-3 flex items-center gap-2 border border-amber-900/50 bg-amber-950/20 px-3 py-2 text-xs text-amber-400">
          <AlertTriangle className="h-3.5 w-3.5 shrink-0" />
          A {kindLabel} needs a name before it can be created.
        </div>
      )}
      {createError && (
        <div className="mx-5 mt-3 flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
          <AlertCircle className="h-4 w-4 shrink-0" /> {createError}
        </div>
      )}

      <div className="min-h-0 flex-1 overflow-hidden px-5 py-5">
        <div className="grid h-full min-h-0 grid-cols-[14rem_minmax(0,1fr)_20rem] gap-4">
          {/* Left: file list */}
          <Card className="flex min-h-0 flex-col">
            <CardContent className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto p-2">
              <div className="px-1 text-[10px] font-mono uppercase tracking-wider text-zinc-600">Bundle files</div>
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
              <span className="truncate font-mono text-[11px] text-zinc-500">{activePath || "no file selected"}</span>
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
