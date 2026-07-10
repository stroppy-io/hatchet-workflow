// AdminCatalog — instance-wide (LEVEL_INSTANCE) provider/workflow catalog
// entries, admin_only (see CatalogService's file doc). Every LEVEL_INSTANCE
// row is ORIGIN_NATIVE (there is nothing to link/fork at this level — see
// service.go's UpdateInstanceEntry doc), so this page is CRUD only, no
// lineage badges. These are the rows every tenant can later Link into their
// own org catalog (OrgCatalog.tsx).

import { useCallback, useEffect, useMemo, useState } from "react";
import { Boxes, Code2, Pencil, Plus, RefreshCw, Trash2, Workflow } from "lucide-react";
import { Link, useNavigate, useSearchParams } from "@/lib/router";
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
import { useConfirm } from "@/components/ui/confirm-dialog";
import { Panel } from "@/pages/dashboard/Panel";
import {
  deleteInstanceEntry,
  listInstanceEntries,
  type CatalogEntryVM,
  type CatalogKind,
} from "@/services/catalog";
import { openInIde } from "@/services/ide";

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

const KIND_PARAM: Record<"providers" | "workflows", CatalogKind> = {
  providers: "KIND_PROVIDER",
  workflows: "KIND_WORKFLOW",
};

// instanceIdeUrl builds the gateway /ide/instance/* URL for an entry's
// on-disk location in the instance repo (spec §3 C1's fixed layout:
// providers/<slug>/... or workflows/<slug>/...), opened as a code-server
// "folder" deep link so the IDE lands on the entry's own files rather than
// the whole repo root. Requires the instance-admin IdeAuthorizer grant
// (internal/ide.Authorizer.CanAuthor) — a non-admin's click 403s at the
// gateway, same as every other LEVEL_INSTANCE write path.
function instanceIdeUrl(tab: "providers" | "workflows", slug: string): string {
  // The scope segment is EntryKindProvider/EntryKindWorkflow — singular,
  // exactly what internal/ide.ParseScope requires — never the plural tab
  // name (that stays plural only in the `folder` deep link below, which
  // mirrors the repo's own on-disk providers/<slug> | workflows/<slug>
  // layout, a completely different string).
  const entryKind = tab === "providers" ? "provider" : "workflow";
  const folder = encodeURIComponent(`/home/coder/project/${tab}/${slug}`);
  return `/ide/instance/${entryKind}/${slug}?folder=${folder}`;
}

export function AdminCatalog() {
  const navigate = useNavigate();
  const confirm = useConfirm();
  const [params, setParams] = useSearchParams();
  const tabParam = params.get("tab") === "workflows" ? "workflows" : "providers";

  const [entries, setEntries] = useState<CatalogEntryVM[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const kind = KIND_PARAM[tabParam];

  const load = useCallback(async () => {
    setError(null);
    try {
      const list = await listInstanceEntries(kind);
      setEntries(list.sort((a, b) => a.slug.localeCompare(b.slug) || b.version - a.version));
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setEntries([]);
    }
  }, [kind]);

  useEffect(() => {
    void load();
  }, [load]);

  const stats = useMemo(() => {
    const list = entries ?? [];
    return { total: list.length, compiling: list.filter((e) => e.summary.compiles).length };
  }, [entries]);

  async function onOpenIde(entry: CatalogEntryVM) {
    setError(null);
    try {
      await openInIde(instanceIdeUrl(tabParam, entry.slug));
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function onDelete(entry: CatalogEntryVM) {
    const ok = await confirm({
      title: `Delete ${entry.name || entry.slug}?`,
      description: `Slug "${entry.slug}" v${entry.version} will be permanently removed from the instance catalog. Any org that has LINKED it keeps its already-linked copy.`,
      danger: true,
      confirmLabel: "Delete",
    });
    if (!ok) return;
    setError(null);
    setNotice(null);
    try {
      await deleteInstanceEntry(entry.id);
      setNotice(`${entry.name || entry.slug} deleted.`);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  return (
    <div className="mx-auto max-w-6xl p-8">
      <div className="mb-1 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
        Admin
      </div>
      <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight text-foreground">
            Catalog
          </h1>
          <p className="mt-1 text-xs text-muted-foreground">
            Instance-wide providers and workflows, visible to every tenant.
            Tenants Link these into their own org catalog to reuse them.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" variant="outline" onClick={() => void load()}>
            <RefreshCw className="h-3.5 w-3.5" />
            Refresh
          </Button>
          <Button asChild size="sm">
            <Link to={`/admin/catalog/${tabParam}/new`}>
              <Plus className="h-3.5 w-3.5" />
              New {tabParam === "providers" ? "provider" : "workflow"}
            </Link>
          </Button>
        </div>
      </div>

      <div className="mt-5 flex flex-col gap-2">
        <StatusLine value={error} tone="error" />
        <StatusLine value={notice} />
      </div>

      <div className="mt-6">
        <Tabs
          value={tabParam}
          onValueChange={(v) => setParams({ tab: v }, { replace: true })}
        >
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
        <Panel label="Last check: compiling clean">
          <div className="text-3xl font-semibold text-foreground">{stats.compiling}</div>
        </Panel>
      </div>

      <Panel label={tabParam === "providers" ? "Providers" : "Workflows"} className="mt-4">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Slug</TableHead>
              <TableHead>Version</TableHead>
              <TableHead>Compiles</TableHead>
              <TableHead>Updated</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {entries === null ? (
              <TableRow>
                <TableCell colSpan={6} className="py-6 text-center text-xs text-muted-foreground">
                  Loading…
                </TableCell>
              </TableRow>
            ) : entries.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="py-6 text-center text-xs text-muted-foreground">
                  No {tabParam} yet.
                </TableCell>
              </TableRow>
            ) : (
              entries.map((entry) => (
                <TableRow
                  key={entry.id}
                  className="cursor-pointer"
                  onClick={() => navigate(`/admin/catalog/${tabParam}/${entry.id}/edit`)}
                >
                  <TableCell className="font-medium">{entry.name || entry.slug}</TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">{entry.slug}</TableCell>
                  <TableCell className="font-mono text-xs">v{entry.version}</TableCell>
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
                        aria-label="Open in IDE"
                        title="Open in IDE"
                        onClick={() => void onOpenIde(entry)}
                      >
                        <Code2 className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        onClick={() => navigate(`/admin/catalog/${tabParam}/${entry.id}/edit`)}
                        aria-label="Edit"
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
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
    </div>
  );
}
