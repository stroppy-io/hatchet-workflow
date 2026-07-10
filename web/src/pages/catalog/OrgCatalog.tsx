// OrgCatalog — the per-tenant (LEVEL_ORG) provider/workflow catalog:
// natively authored entries, live links to instance entries, and forks
// (diverged copies of a link or of another org entry). Reached at
// /t/:slug/catalog.
//
// RBAC: reuses org.ts's hasOrgPermission against RESOURCE_PROVIDER /
// RESOURCE_WORKFLOW (the same fine-grained permission catalog OrgDetail.tsx
// gates its own actions with) rather than inventing a parallel mechanism.

import { useCallback, useEffect, useMemo, useState } from "react";
import { Boxes, GitFork, Link2, Pencil, Plus, RefreshCw, Sparkles, Trash2, Workflow } from "lucide-react";
import { Link, useNavigate, useSearchParams, useTenantSlug } from "@/lib/router";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useConfirm } from "@/components/ui/confirm-dialog";
import { Panel } from "@/pages/dashboard/Panel";
import { getOrgProvider, hasOrgPermission, type OrgRole } from "@/services/org";
import {
  deleteOrgEntry,
  linkInstanceEntry,
  listInstanceEntries,
  listOrgEntries,
  type CatalogEntryVM,
  type CatalogKind,
} from "@/services/catalog";

function StatusLine({ value, tone = "default" }: { value: string | null; tone?: "default" | "error" }) {
  if (!value) return null;
  return (
    <div
      className={
        tone === "error"
          ? "border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive"
          : "border border-border bg-muted/30 px-3 py-2 text-sm text-muted-foreground"
      }
    >
      {value}
    </div>
  );
}

function fmtDate(iso?: string): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

function OriginBadge({ origin }: { origin: CatalogEntryVM["origin"] }) {
  if (origin === "ORIGIN_LINKED") {
    return (
      <Badge variant="outline" className="gap-1 border-blue-800 text-blue-300">
        <Link2 className="h-3 w-3" /> Linked
      </Badge>
    );
  }
  if (origin === "ORIGIN_FORKED") {
    return (
      <Badge variant="outline" className="gap-1 border-amber-800 text-amber-300">
        <GitFork className="h-3 w-3" /> Forked
      </Badge>
    );
  }
  return (
    <Badge variant="outline" className="gap-1 border-zinc-700 text-zinc-400">
      <Sparkles className="h-3 w-3" /> Native
    </Badge>
  );
}

const KIND_PARAM: Record<"providers" | "workflows", CatalogKind> = {
  providers: "KIND_PROVIDER",
  workflows: "KIND_WORKFLOW",
};
const RESOURCE_FOR: Record<CatalogKind, "RESOURCE_PROVIDER" | "RESOURCE_WORKFLOW"> = {
  KIND_PROVIDER: "RESOURCE_PROVIDER",
  KIND_WORKFLOW: "RESOURCE_WORKFLOW",
};

export function OrgCatalog() {
  const slug = useTenantSlug() ?? "";
  const navigate = useNavigate();
  const confirm = useConfirm();
  const [params, setParams] = useSearchParams();
  const tabParam = params.get("tab") === "workflows" ? "workflows" : "providers";
  const kind = KIND_PARAM[tabParam];
  const resource = RESOURCE_FOR[kind];

  const [entries, setEntries] = useState<CatalogEntryVM[] | null>(null);
  const [roles, setRoles] = useState<OrgRole[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const [linkOpen, setLinkOpen] = useState(false);
  const [linkCandidates, setLinkCandidates] = useState<CatalogEntryVM[] | null>(null);
  const [linkError, setLinkError] = useState<string | null>(null);
  const [linking, setLinking] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!slug) return;
    setError(null);
    try {
      const list = await listOrgEntries(slug, kind);
      setEntries(list.sort((a, b) => a.slug.localeCompare(b.slug) || b.version - a.version));
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setEntries([]);
    }
  }, [slug, kind]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (!slug) return;
    let cancelled = false;
    getOrgProvider()
      .getOrg(slug)
      .then((detail) => !cancelled && setRoles(detail.currentRoles))
      .catch(() => {
        /* role fetch is best-effort; every action stays gated off (disabled) */
      });
    return () => {
      cancelled = true;
    };
  }, [slug]);

  const canCreate = hasOrgPermission(roles, resource, "ACTION_CREATE");
  const canUpdate = hasOrgPermission(roles, resource, "ACTION_UPDATE");
  const canDelete = hasOrgPermission(roles, resource, "ACTION_DELETE");

  const stats = useMemo(() => {
    const list = entries ?? [];
    return {
      total: list.length,
      linked: list.filter((e) => e.origin === "ORIGIN_LINKED").length,
      forked: list.filter((e) => e.origin === "ORIGIN_FORKED").length,
    };
  }, [entries]);

  async function onDelete(entry: CatalogEntryVM) {
    const ok = await confirm({
      title: `Delete ${entry.name || entry.slug}?`,
      description:
        entry.origin === "ORIGIN_LINKED"
          ? `This removes this org's LIVE LINK to "${entry.slug}" — the instance catalog entry itself is unaffected.`
          : `"${entry.slug}" v${entry.version} will be permanently removed from this org's catalog.`,
      danger: true,
      confirmLabel: "Delete",
    });
    if (!ok) return;
    setError(null);
    setNotice(null);
    try {
      await deleteOrgEntry(slug, kind, entry.id);
      setNotice(`${entry.name || entry.slug} deleted.`);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function openLink() {
    setLinkOpen(true);
    setLinkError(null);
    setLinkCandidates(null);
    try {
      const [instanceEntries, orgEntries] = await Promise.all([
        listInstanceEntries(kind),
        listOrgEntries(slug, kind),
      ]);
      const linkedSourceIds = new Set(
        orgEntries.filter((e) => e.origin === "ORIGIN_LINKED" || e.sourceEntryId).map((e) => e.sourceEntryId),
      );
      setLinkCandidates(instanceEntries.filter((e) => !linkedSourceIds.has(e.id)));
    } catch (e) {
      setLinkError(e instanceof Error ? e.message : String(e));
      setLinkCandidates([]);
    }
  }

  async function onLink(instanceEntryId: string) {
    setLinking(instanceEntryId);
    setLinkError(null);
    try {
      await linkInstanceEntry(slug, kind, instanceEntryId);
      setLinkOpen(false);
      setNotice("Linked into this org's catalog.");
      await load();
    } catch (e) {
      setLinkError(e instanceof Error ? e.message : String(e));
    } finally {
      setLinking(null);
    }
  }

  return (
    <div className="mx-auto max-w-6xl p-8">
      <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight text-foreground">
            Catalog
          </h1>
          <p className="mt-1 text-xs text-muted-foreground">
            Providers and workflows this org can build recipes from — native,
            linked from the instance catalog, or forked from a link.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" variant="outline" onClick={() => void load()}>
            <RefreshCw className="h-3.5 w-3.5" />
            Refresh
          </Button>
          {canCreate && (
            <Button size="sm" variant="outline" onClick={() => void openLink()}>
              <Link2 className="h-3.5 w-3.5" />
              Link from instance catalog
            </Button>
          )}
          {canCreate && (
            <Button asChild size="sm">
              <Link to={`/catalog/${tabParam}/new`}>
                <Plus className="h-3.5 w-3.5" />
                New {tabParam === "providers" ? "provider" : "workflow"}
              </Link>
            </Button>
          )}
        </div>
      </div>

      <div className="mt-5 flex flex-col gap-2">
        <StatusLine value={error} tone="error" />
        <StatusLine value={notice} />
      </div>

      <div className="mt-6">
        <Tabs value={tabParam} onValueChange={(v) => setParams({ tab: v }, { replace: true })}>
          <TabsList>
            <TabsTrigger value="providers">
              <Boxes className="h-3.5 w-3.5" /> Providers
            </TabsTrigger>
            <TabsTrigger value="workflows">
              <Workflow className="h-3.5 w-3.5" /> Workflows
            </TabsTrigger>
          </TabsList>
        </Tabs>
      </div>

      <div className="mt-4 grid grid-cols-1 gap-4 md:grid-cols-3">
        <Panel label="Entries">
          <div className="text-3xl font-semibold text-foreground">{stats.total}</div>
        </Panel>
        <Panel label="Linked">
          <div className="text-3xl font-semibold text-foreground">{stats.linked}</div>
        </Panel>
        <Panel label="Forked">
          <div className="text-3xl font-semibold text-foreground">{stats.forked}</div>
        </Panel>
      </div>

      <Panel label={tabParam === "providers" ? "Providers" : "Workflows"} className="mt-4">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Slug</TableHead>
              <TableHead>Version</TableHead>
              <TableHead>Origin</TableHead>
              <TableHead>Compiles</TableHead>
              <TableHead>Updated</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {entries === null ? (
              <TableRow>
                <TableCell colSpan={7} className="py-6 text-center text-xs text-muted-foreground">
                  Loading…
                </TableCell>
              </TableRow>
            ) : entries.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className="py-6 text-center text-xs text-muted-foreground">
                  No {tabParam} yet. Link one from the instance catalog or author a new one.
                </TableCell>
              </TableRow>
            ) : (
              entries.map((entry) => (
                <TableRow
                  key={entry.id}
                  className="cursor-pointer"
                  onClick={() => navigate(`/catalog/${tabParam}/${entry.id}/edit`)}
                >
                  <TableCell className="font-medium">{entry.name || entry.slug}</TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">{entry.slug}</TableCell>
                  <TableCell className="font-mono text-xs">v{entry.version}</TableCell>
                  <TableCell>
                    <OriginBadge origin={entry.origin} />
                  </TableCell>
                  <TableCell>
                    <Badge variant={entry.summary.compiles ? "success" : "destructive"}>
                      {entry.summary.compiles ? "ok" : "errors"}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">{fmtDate(entry.updatedAt)}</TableCell>
                  <TableCell className="text-right" onClick={(e) => e.stopPropagation()}>
                    <div className="flex justify-end gap-1">
                      <Button
                        size="icon"
                        variant="ghost"
                        disabled={!canUpdate}
                        title={
                          !canUpdate
                            ? "Requires update permission on this resource"
                            : entry.origin === "ORIGIN_LINKED"
                              ? "Editing a linked entry forks it into an independent copy"
                              : "Open in the embedded IDE"
                        }
                        onClick={() => navigate(`/catalog/${tabParam}/${entry.id}/edit`)}
                        aria-label="Edit"
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        disabled={!canDelete}
                        title={!canDelete ? "Requires delete permission on this resource" : "Delete"}
                        onClick={() => void onDelete(entry)}
                        aria-label="Delete"
                        className="text-destructive hover:text-destructive"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </Panel>

      <Dialog open={linkOpen} onOpenChange={setLinkOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>Link from the instance catalog</DialogTitle>
            <DialogDescription>
              Creates a live link: this org's copy always tracks the instance
              entry as-is. Editing it later forks it into an independent copy
              owned by this org — the link and the instance source are never
              mutated.
            </DialogDescription>
          </DialogHeader>
          <StatusLine value={linkError} tone="error" />
          <div className="flex max-h-80 flex-col gap-2 overflow-y-auto">
            {linkCandidates === null ? (
              <div className="py-4 text-center text-xs text-muted-foreground">Loading…</div>
            ) : linkCandidates.length === 0 ? (
              <div className="py-4 text-center text-xs text-muted-foreground">
                Nothing left to link — every instance {tabParam === "providers" ? "provider" : "workflow"} is
                already linked into this org.
              </div>
            ) : (
              linkCandidates.map((c) => (
                <div
                  key={c.id}
                  className="flex items-center justify-between border border-border px-3 py-2"
                >
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium text-foreground">{c.name || c.slug}</div>
                    <div className="truncate font-mono text-[11px] text-muted-foreground">
                      {c.slug} · v{c.version}
                    </div>
                  </div>
                  <Button
                    size="sm"
                    disabled={linking === c.id}
                    onClick={() => void onLink(c.id)}
                  >
                    <Link2 className="h-3.5 w-3.5" />
                    {linking === c.id ? "Linking…" : "Link"}
                  </Button>
                </div>
              ))
            )}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
