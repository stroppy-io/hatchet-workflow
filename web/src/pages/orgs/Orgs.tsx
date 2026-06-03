import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  Building2,
  ChevronRight,
  Plus,
  RefreshCw,
  ShieldCheck,
  Tag,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Panel } from "@/pages/dashboard/Panel";
import {
  getOrgProvider,
  roleListLabel,
  type OrgRole,
  type OrgSummaryVM,
} from "@/services/org";

function roleBadgeVariant(roles: OrgRole[]): "default" | "success" | "secondary" {
  const names = roles.map((role) => role.name.toLowerCase());
  if (names.some((name) => name === "owner" || name === "admin")) return "success";
  if (names.some((name) => name === "operator")) return "default";
  return "secondary";
}

function fmtDate(iso?: string): string {
  if (!iso) return "Not set";
  return new Date(iso).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

function StatusLine({
  value,
  tone = "default",
}: {
  value: string | null;
  tone?: "default" | "error";
}) {
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

export function Orgs() {
  const navigate = useNavigate();
  const [orgs, setOrgs] = useState<OrgSummaryVM[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState("");
  const [createSlug, setCreateSlug] = useState("");

  const roleCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const org of orgs ?? []) {
      const key = roleListLabel(org.currentRoles);
      counts.set(key, (counts.get(key) ?? 0) + 1);
    }
    return Array.from(counts.entries());
  }, [orgs]);

  async function load() {
    setError(null);
    try {
      const list = await getOrgProvider().listOrgs();
      setOrgs(list);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  useEffect(() => {
    let cancelled = false;
    setError(null);
    getOrgProvider()
      .listOrgs()
      .then((list) => {
        if (!cancelled) setOrgs(list);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      });
    return () => {
      cancelled = true;
    };
  }, []);

  async function createTenant() {
    setError(null);
    setNotice(null);
    try {
      const detail = await getOrgProvider().createTenant({
        name: createName,
        slug: createSlug,
      });
      setCreateOpen(false);
      setCreateName("");
      setCreateSlug("");
      setNotice("Tenant created.");
      await load();
      navigate(`/orgs/${detail.tenant.slug}`);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  return (
    <div className="mx-auto max-w-6xl p-8">
      <div className="mb-1 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
        Account
      </div>
      <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight text-foreground">
            Organizations
          </h1>
          <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
            ListMyTenants with tenant routing data and resolved current roles.
            CreateTenant opens a new workspace and seeds owner membership.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" variant="outline" onClick={() => void load()}>
            <RefreshCw className="h-3.5 w-3.5" />
            Refresh
          </Button>
          <Button
            size="sm"
            onClick={() => {
              setCreateName("");
              setCreateSlug("");
              setCreateOpen(true);
            }}
          >
            <Plus className="h-3.5 w-3.5" />
            Create
          </Button>
        </div>
      </div>

      <div className="mt-5 flex flex-col gap-2">
        <StatusLine value={error} tone="error" />
        <StatusLine value={notice} />
      </div>

      <div className="mt-8 grid grid-cols-1 gap-4 md:grid-cols-3">
        <Panel label="Tenants">
          <div className="flex items-end justify-between gap-3">
            <div className="text-3xl font-semibold text-foreground">
              {orgs?.length ?? 0}
            </div>
            <Building2 className="h-5 w-5 text-muted-foreground" />
          </div>
          <div className="mt-2 text-xs text-muted-foreground">
            Visible through ListMyTenants.
          </div>
        </Panel>
        <Panel label="Roles">
          <div className="space-y-2">
            {roleCounts.length === 0 ? (
              <div className="text-sm text-muted-foreground">No roles.</div>
            ) : (
              roleCounts.map(([role, count]) => (
                <div key={role} className="flex items-center justify-between gap-2">
                  <span className="truncate text-sm text-muted-foreground">
                    {role}
                  </span>
                  <Badge variant="secondary">{count}</Badge>
                </div>
              ))
            )}
          </div>
        </Panel>
        <Panel label="Routing">
          <div className="space-y-2 text-xs text-muted-foreground">
            <div>Tenant pages resolve by slug.</div>
            <div>Storage and authorization use tenant id.</div>
          </div>
        </Panel>
      </div>

      <div className="mt-6">
        <Panel label="ListMyTenants" bodyClassName="">
          {!orgs && !error && (
            <div className="p-6 text-sm text-muted-foreground">Loading...</div>
          )}
          {orgs && orgs.length === 0 && !error && (
            <div className="p-6 text-sm text-muted-foreground">
              You do not belong to any organization.
            </div>
          )}
          {orgs && orgs.length > 0 && (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Tenant</TableHead>
                  <TableHead>Current roles</TableHead>
                  <TableHead>Tags</TableHead>
                  <TableHead>Audit</TableHead>
                  <TableHead className="w-[72px] text-right">Open</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {orgs.map((org) => (
                  <TableRow key={org.tenant.id}>
                    <TableCell>
                      <div className="flex min-w-0 items-center gap-3">
                        <span className="flex h-8 w-8 shrink-0 items-center justify-center border border-border bg-muted/40">
                          <Building2 className="h-4 w-4 text-muted-foreground" />
                        </span>
                        <div className="min-w-0">
                          <div className="truncate text-sm text-foreground">
                            {org.tenant.name}
                          </div>
                          <div className="truncate font-mono text-[11px] text-muted-foreground">
                            slug {org.tenant.slug}
                          </div>
                          <div className="truncate font-mono text-[10px] text-muted-foreground">
                            id {org.tenant.id}
                          </div>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={roleBadgeVariant(org.currentRoles)}>
                        <ShieldCheck className="mr-1 h-3 w-3" />
                        {roleListLabel(org.currentRoles)}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex max-w-sm flex-wrap gap-1">
                        {(org.tenant.tags?.tags ?? []).length === 0 ? (
                          <span className="text-xs text-muted-foreground">
                            no tags
                          </span>
                        ) : (
                          (org.tenant.tags?.tags ?? []).map((tag) => (
                            <Badge key={tag} variant="secondary">
                              <Tag className="mr-1 h-3 w-3" />
                              {tag}
                            </Badge>
                          ))
                        )}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="text-[11px] text-muted-foreground">
                        created {fmtDate(org.tenant.createdAt)}
                      </div>
                      <div className="text-[11px] text-muted-foreground">
                        updated {fmtDate(org.tenant.updatedAt)}
                      </div>
                    </TableCell>
                    <TableCell className="text-right">
                      <Button asChild size="icon" variant="ghost" className="h-8 w-8">
                        <Link to={`/orgs/${org.tenant.slug}`}>
                          <ChevronRight className="h-4 w-4" />
                        </Link>
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </Panel>
      </div>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>CreateTenant</DialogTitle>
            <DialogDescription>CreateTenantRequest</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="tenant-name">name</Label>
                <Input
                  id="tenant-name"
                  value={createName}
                  onChange={(e) => setCreateName(e.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="tenant-slug">slug</Label>
                <Input
                  id="tenant-slug"
                  value={createSlug}
                  onChange={(e) => setCreateSlug(e.target.value)}
                  className="font-mono"
                />
              </div>
            </div>
            <div className="flex justify-end gap-2">
              <Button size="sm" variant="outline" onClick={() => setCreateOpen(false)}>
                Cancel
              </Button>
              <Button
                size="sm"
                disabled={!createName.trim() || !createSlug.trim()}
                onClick={() => void createTenant()}
              >
                Create
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
