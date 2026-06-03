import { useEffect, useState } from "react";
import {
  CheckCircle2,
  KeyRound,
  Pencil,
  Plus,
  RefreshCw,
  ShieldCheck,
  Trash2,
  XCircle,
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
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useConfirm } from "@/components/ui/confirm-dialog";
import { Panel } from "@/pages/dashboard/Panel";
import {
  getAdminProvider,
  type AdminIdentityProvider,
  type CreateIdentityProviderInput,
  type UpdateIdentityProviderInput,
} from "@/services/admin";

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

/** Split a comma/space/newline-separated list into a trimmed, deduped array. */
function parseList(raw: string): string[] {
  return [...new Set(raw.split(/[\s,]+/).map((s) => s.trim()).filter(Boolean))];
}

interface FormState {
  slug: string;
  displayName: string;
  issuer: string;
  clientId: string;
  clientSecret: string;
  scopes: string;
  allowedDomains: string;
  autoProvision: boolean;
  disabled: boolean;
}

const EMPTY_FORM: FormState = {
  slug: "",
  displayName: "",
  issuer: "https://",
  clientId: "",
  clientSecret: "",
  scopes: "openid email profile",
  allowedDomains: "",
  autoProvision: false,
  disabled: false,
};

export function AdminIdentityProviders() {
  const confirm = useConfirm();
  const [providers, setProviders] = useState<AdminIdentityProvider[] | null>(
    null,
  );
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [mode, setMode] = useState<"create" | "update">("create");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState<FormState>(EMPTY_FORM);

  async function load() {
    setError(null);
    try {
      setProviders(await getAdminProvider().listIdentityProviders());
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  useEffect(() => {
    let cancelled = false;
    getAdminProvider()
      .listIdentityProviders()
      .then((list) => {
        if (!cancelled) setProviders(list);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      });
    return () => {
      cancelled = true;
    };
  }, []);

  function set<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  function openCreate() {
    setMode("create");
    setEditingId(null);
    setForm(EMPTY_FORM);
    setDialogOpen(true);
  }

  // Edit fetches the single full record via GetIdentityProvider so the form is
  // populated from the authoritative row, not the (already loaded) list copy.
  async function openEdit(id: string) {
    setError(null);
    try {
      const p = await getAdminProvider().getIdentityProvider(id);
      setMode("update");
      setEditingId(p.id);
      setForm({
        slug: p.slug,
        displayName: p.displayName,
        issuer: p.issuer,
        clientId: p.clientId,
        clientSecret: "",
        scopes: p.scopes.join(" "),
        allowedDomains: p.allowedDomains.join(" "),
        autoProvision: p.autoProvision,
        disabled: p.disabled,
      });
      setDialogOpen(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function save() {
    setError(null);
    setNotice(null);
    try {
      if (mode === "create") {
        const input: CreateIdentityProviderInput = {
          slug: form.slug,
          displayName: form.displayName,
          issuer: form.issuer,
          clientId: form.clientId,
          clientSecret: form.clientSecret,
          scopes: parseList(form.scopes),
          allowedDomains: parseList(form.allowedDomains),
          autoProvision: form.autoProvision,
        };
        await getAdminProvider().createIdentityProvider(input);
        setNotice("Identity provider created.");
      } else if (editingId) {
        const input: UpdateIdentityProviderInput = {
          id: editingId,
          displayName: form.displayName,
          issuer: form.issuer,
          clientId: form.clientId,
          clientSecret: form.clientSecret || undefined,
          scopes: parseList(form.scopes),
          allowedDomains: parseList(form.allowedDomains),
          autoProvision: form.autoProvision,
          disabled: form.disabled,
        };
        await getAdminProvider().updateIdentityProvider(input);
        setNotice("Identity provider updated.");
      }
      setDialogOpen(false);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function remove(p: AdminIdentityProvider) {
    const ok = await confirm({
      title: "Delete identity provider?",
      description: `${p.displayName} / ${p.slug}`,
      confirmLabel: "Delete",
      danger: true,
    });
    if (!ok) return;
    setError(null);
    setNotice(null);
    try {
      await getAdminProvider().deleteIdentityProvider(p.id);
      setNotice("Identity provider deleted.");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  const createValid =
    !!form.slug.trim() &&
    !!form.displayName.trim() &&
    /^https:\/\/.+/.test(form.issuer) &&
    !!form.clientId.trim() &&
    (mode === "update" || !!form.clientSecret.trim());

  return (
    <div className="mx-auto max-w-6xl p-8">
      <div className="mb-1 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
        Admin
      </div>
      <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight text-foreground">
            Identity providers
          </h1>
          <div className="mt-1 text-xs text-muted-foreground">
            OIDC single sign-on providers (IamService)
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" variant="outline" onClick={() => void load()}>
            <RefreshCw className="h-3.5 w-3.5" />
            Refresh
          </Button>
          <Button size="sm" onClick={openCreate}>
            <Plus className="h-3.5 w-3.5" />
            New provider
          </Button>
        </div>
      </div>

      <div className="mt-5 flex flex-col gap-2">
        <StatusLine value={error} tone="error" />
        <StatusLine value={notice} />
      </div>

      <div className="mt-8">
        <Panel
          label="ListIdentityProviders / GetIdentityProvider"
          bodyClassName=""
        >
          {providers === null ? (
            <div className="p-4 text-sm text-muted-foreground">Loading...</div>
          ) : providers.length === 0 ? (
            <div className="p-4 text-sm text-muted-foreground">
              No identity providers configured. The public buttons list returns
              only ENABLED providers, so disabled ones are not shown here.
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Provider</TableHead>
                  <TableHead>Issuer</TableHead>
                  <TableHead>Scopes / domains</TableHead>
                  <TableHead>State</TableHead>
                  <TableHead className="w-[96px] text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {providers.map((p) => (
                  <TableRow key={p.id}>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <ShieldCheck className="h-4 w-4 text-muted-foreground" />
                        <span className="text-sm text-foreground">
                          {p.displayName}
                        </span>
                        <Badge variant="outline" className="font-mono">
                          {p.slug}
                        </Badge>
                      </div>
                      <div className="mt-1 font-mono text-[10px] text-muted-foreground">
                        client_id {p.clientId}
                      </div>
                      <div className="font-mono text-[10px] text-muted-foreground">
                        created {fmtDate(p.createdAt)}
                      </div>
                    </TableCell>
                    <TableCell>
                      <span className="font-mono text-[11px] text-foreground">
                        {p.issuer}
                      </span>
                    </TableCell>
                    <TableCell>
                      <div className="flex max-w-xs flex-wrap gap-1">
                        {p.scopes.map((s) => (
                          <Badge key={`s-${s}`} variant="secondary">
                            {s}
                          </Badge>
                        ))}
                        {p.allowedDomains.map((d) => (
                          <Badge key={`d-${d}`} variant="outline">
                            @{d}
                          </Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-col gap-1">
                        <Badge variant={p.disabled ? "secondary" : "success"}>
                          {p.disabled ? "disabled" : "enabled"}
                        </Badge>
                        <span className="flex items-center gap-1 text-[11px] text-muted-foreground">
                          {p.autoProvision ? (
                            <CheckCircle2 className="h-3 w-3 text-success" />
                          ) : (
                            <XCircle className="h-3 w-3 text-muted-foreground" />
                          )}
                          auto_provision
                        </span>
                      </div>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="inline-flex gap-1">
                        <Button
                          size="icon"
                          variant="ghost"
                          className="h-7 w-7 text-muted-foreground"
                          onClick={() => void openEdit(p.id)}
                        >
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                        <Button
                          size="icon"
                          variant="ghost"
                          className="h-7 w-7 text-muted-foreground hover:text-destructive"
                          onClick={() => void remove(p)}
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </Panel>
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {mode === "create"
                ? "CreateIdentityProvider"
                : "UpdateIdentityProvider"}
            </DialogTitle>
            <DialogDescription>
              {mode === "create"
                ? "CreateIdentityProviderRequest"
                : "UpdateIdentityProviderRequest (slug is immutable)"}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="idp-slug">slug</Label>
                <Input
                  id="idp-slug"
                  value={form.slug}
                  disabled={mode === "update"}
                  onChange={(e) => set("slug", e.target.value)}
                  className="font-mono"
                  placeholder="google"
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="idp-display">display_name</Label>
                <Input
                  id="idp-display"
                  value={form.displayName}
                  onChange={(e) => set("displayName", e.target.value)}
                  placeholder="Google"
                />
              </div>
              <div className="flex flex-col gap-1.5 sm:col-span-2">
                <Label htmlFor="idp-issuer">issuer</Label>
                <Input
                  id="idp-issuer"
                  value={form.issuer}
                  onChange={(e) => set("issuer", e.target.value)}
                  className="font-mono"
                  placeholder="https://accounts.google.com"
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="idp-client-id">client_id</Label>
                <Input
                  id="idp-client-id"
                  value={form.clientId}
                  onChange={(e) => set("clientId", e.target.value)}
                  className="font-mono"
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="idp-secret">
                  client_secret{" "}
                  {mode === "update" && (
                    <span className="text-muted-foreground">
                      (blank = unchanged)
                    </span>
                  )}
                </Label>
                <div className="flex items-center gap-2">
                  <KeyRound className="h-4 w-4 shrink-0 text-muted-foreground" />
                  <Input
                    id="idp-secret"
                    type="password"
                    value={form.clientSecret}
                    onChange={(e) => set("clientSecret", e.target.value)}
                    className="font-mono"
                  />
                </div>
              </div>
              <div className="flex flex-col gap-1.5 sm:col-span-2">
                <Label htmlFor="idp-scopes">scopes (space/comma separated)</Label>
                <Input
                  id="idp-scopes"
                  value={form.scopes}
                  onChange={(e) => set("scopes", e.target.value)}
                  className="font-mono"
                  placeholder="openid email profile"
                />
              </div>
              <div className="flex flex-col gap-1.5 sm:col-span-2">
                <Label htmlFor="idp-domains">
                  allowed_domains (space/comma separated)
                </Label>
                <Input
                  id="idp-domains"
                  value={form.allowedDomains}
                  onChange={(e) => set("allowedDomains", e.target.value)}
                  className="font-mono"
                  placeholder="acme.com"
                />
              </div>
              <div className="flex min-w-0 flex-col gap-1.5">
                <Label>auto_provision</Label>
                <div className="flex h-9 items-center gap-2 border border-input px-3">
                  <Switch
                    checked={form.autoProvision}
                    onCheckedChange={(c) => set("autoProvision", c)}
                  />
                  <span className="text-sm text-muted-foreground">
                    {String(form.autoProvision)}
                  </span>
                </div>
              </div>
              {mode === "update" && (
                <div className="flex min-w-0 flex-col gap-1.5">
                  <Label>disabled</Label>
                  <div className="flex h-9 items-center gap-2 border border-input px-3">
                    <Switch
                      checked={form.disabled}
                      onCheckedChange={(c) => set("disabled", c)}
                    />
                    <span className="text-sm text-muted-foreground">
                      {String(form.disabled)}
                    </span>
                  </div>
                </div>
              )}
            </div>
            {form.autoProvision && parseList(form.allowedDomains).length === 0 && (
              <div className="border border-warning/40 bg-warning/10 px-3 py-2 text-xs text-warning">
                auto_provision requires a non-empty allowed_domains; the server
                rejects this combination.
              </div>
            )}
            <div className="flex justify-end gap-2">
              <Button
                size="sm"
                variant="outline"
                onClick={() => setDialogOpen(false)}
              >
                Cancel
              </Button>
              <Button size="sm" disabled={!createValid} onClick={() => void save()}>
                Save
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
