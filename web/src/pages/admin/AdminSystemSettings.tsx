import { useEffect, useMemo, useState } from "react";
import {
  Globe2,
  LockKeyhole,
  RefreshCw,
  Save,
  ShieldCheck,
  SlidersHorizontal,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Panel } from "@/pages/dashboard/Panel";
import {
  getAdminProvider,
  type AdminPlatformSettings,
} from "@/services/admin";

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

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

export function AdminSystemSettings() {
  const [settings, setSettings] = useState<AdminPlatformSettings | null>(null);
  const [draft, setDraft] = useState<AdminPlatformSettings | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const dirty = useMemo(() => {
    if (!settings || !draft) return false;
    return JSON.stringify(settings) !== JSON.stringify(draft);
  }, [settings, draft]);

  async function load() {
    setError(null);
    try {
      const next = await getAdminProvider().getSystemSettings();
      setSettings(next);
      setDraft(clone(next));
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  useEffect(() => {
    let cancelled = false;
    setError(null);
    getAdminProvider()
      .getSystemSettings()
      .then((next) => {
        if (cancelled) return;
        setSettings(next);
        setDraft(clone(next));
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      });
    return () => {
      cancelled = true;
    };
  }, []);

  async function save() {
    if (!draft) return;
    setError(null);
    setNotice(null);
    try {
      const next = await getAdminProvider().updateSystemSettings(draft);
      setSettings(next);
      setDraft(clone(next));
      setNotice("System settings updated.");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  function update<K extends keyof AdminPlatformSettings>(
    key: K,
    value: AdminPlatformSettings[K],
  ) {
    setDraft((current) => (current ? { ...current, [key]: value } : current));
  }

  return (
    <div className="mx-auto max-w-6xl p-8">
      <div className="mb-1 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
        Admin
      </div>
      <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight text-foreground">
            System settings
          </h1>
          <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
            <span>singleton control-plane configuration</span>
            {dirty && <Badge variant="warning">dirty</Badge>}
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" variant="outline" onClick={() => void load()}>
            <RefreshCw className="h-3.5 w-3.5" />
            Refresh
          </Button>
          <Button size="sm" disabled={!dirty || !draft} onClick={() => void save()}>
            <Save className="h-3.5 w-3.5" />
            Save
          </Button>
        </div>
      </div>

      <div className="mt-5 flex flex-col gap-2">
        <StatusLine value={error} tone="error" />
        <StatusLine value={notice} />
      </div>

      <div className="mt-8 grid grid-cols-1 gap-4 md:grid-cols-3">
        <Panel label="Server">
          <div className="flex items-end justify-between gap-3">
            <div className="min-w-0">
              <div className="truncate text-sm text-foreground">
                {settings?.serverAddr || "derived"}
              </div>
              <div className="mt-1 font-mono text-[10px] text-muted-foreground">
                server_addr
              </div>
            </div>
            <Globe2 className="h-5 w-5 text-muted-foreground" />
          </div>
        </Panel>
        <Panel label="Registration">
          <div className="flex items-end justify-between gap-3">
            <Badge
              variant={settings?.allowSelfRegistration ? "success" : "secondary"}
            >
              {String(!!settings?.allowSelfRegistration)}
            </Badge>
            <LockKeyhole className="h-5 w-5 text-muted-foreground" />
          </div>
        </Panel>
        <Panel label="Tenant creation">
          <div className="flex items-end justify-between gap-3">
            <Badge
              variant={
                settings?.allowMemberTenantCreation ? "success" : "warning"
              }
            >
              {String(!!settings?.allowMemberTenantCreation)}
            </Badge>
            <ShieldCheck className="h-5 w-5 text-muted-foreground" />
          </div>
        </Panel>
      </div>

      <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-[18rem_1fr]">
        <div className="space-y-4">
          <Panel label="SystemSettingsService">
            <div className="flex items-center gap-3">
              <span className="flex h-9 w-9 items-center justify-center border border-border bg-muted/40">
                <SlidersHorizontal className="h-4 w-4 text-muted-foreground" />
              </span>
              <div className="min-w-0">
                <div className="text-sm text-foreground">
                  PlatformSettings
                </div>
                <div className="truncate font-mono text-[11px] text-muted-foreground">
                  GetSystemSettings / UpdateSystemSettings
                </div>
              </div>
            </div>
          </Panel>
        </div>

        <Panel
          label="UpdateSystemSettings"
          action={
            <Button
              size="sm"
              disabled={!dirty || !draft}
              onClick={() => void save()}
            >
              <Save className="h-3.5 w-3.5" />
              Save
            </Button>
          }
        >
          {!draft && !error ? (
            <div className="text-sm text-muted-foreground">Loading...</div>
          ) : draft ? (
            <div className="grid grid-cols-1 gap-4">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="server-addr">server_addr</Label>
                <Input
                  id="server-addr"
                  value={draft.serverAddr}
                  onChange={(e) => update("serverAddr", e.target.value)}
                  placeholder="https://cloud.example.com"
                />
              </div>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div className="flex items-center justify-between border border-border bg-muted/20 px-3 py-2">
                  <div className="min-w-0">
                    <Label htmlFor="allow-self-registration">
                      allow_self_registration
                    </Label>
                    <div className="mt-1 font-mono text-[10px] text-muted-foreground">
                      {String(draft.allowSelfRegistration)}
                    </div>
                  </div>
                  <Switch
                    id="allow-self-registration"
                    checked={draft.allowSelfRegistration}
                    onCheckedChange={(checked) =>
                      update("allowSelfRegistration", checked)
                    }
                  />
                </div>
                <div className="flex items-center justify-between border border-border bg-muted/20 px-3 py-2">
                  <div className="min-w-0">
                    <Label htmlFor="allow-member-tenant-creation">
                      allow_member_tenant_creation
                    </Label>
                    <div className="mt-1 font-mono text-[10px] text-muted-foreground">
                      {String(draft.allowMemberTenantCreation)}
                    </div>
                  </div>
                  <Switch
                    id="allow-member-tenant-creation"
                    checked={draft.allowMemberTenantCreation}
                    onCheckedChange={(checked) =>
                      update("allowMemberTenantCreation", checked)
                    }
                  />
                </div>
              </div>
            </div>
          ) : null}
        </Panel>
      </div>
    </div>
  );
}
