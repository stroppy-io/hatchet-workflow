import { Loader2, Save } from "lucide-react";
import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useAuth } from "@/contexts/auth-context";
import { api } from "@/lib/connect";

export function PlatformSettingsPage() {
  const { account } = useAuth();
  const [serverAddr, setServerAddr] = useState("");
  const [savedServerAddr, setSavedServerAddr] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const canEdit = Boolean(account?.isAdmin);
  const dirty = serverAddr !== savedServerAddr;

  useEffect(() => {
    let cancelled = false;

    setLoading(true);
    setError(null);
    api.platformAdmin
      .getPlatformSettings({})
      .then((settings) => {
        if (cancelled) return;
        setServerAddr(settings.serverAddr);
        setSavedServerAddr(settings.serverAddr);
      })
      .catch((loadError: unknown) => {
        if (!cancelled) setError(loadError instanceof Error ? loadError.message : "Failed to load platform settings");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, []);

  async function save() {
    setSaving(true);
    setError(null);
    try {
      const settings = await api.platformAdmin.setPlatformSettings({ serverAddr });
      setServerAddr(settings.serverAddr);
      setSavedServerAddr(settings.serverAddr);
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "Failed to save platform settings");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="mx-auto max-w-5xl space-y-6 p-6">
      <header>
        <h1 className="text-2xl font-semibold tracking-normal">Platform settings</h1>
        <p className="mt-1 text-sm text-muted-foreground">Global control-plane configuration.</p>
      </header>

      <section className="rounded-md border bg-card">
        <div className="border-b px-5 py-4">
          <h2 className="text-sm font-semibold">Server</h2>
          <p className="mt-1 text-sm text-muted-foreground">Address used by the platform services and generated runtime links.</p>
        </div>
        <div className="space-y-4 p-5">
          {error ? <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</div> : null}
          <div className="grid gap-2 md:grid-cols-[180px_1fr_auto] md:items-center">
            <label className="text-sm font-medium" htmlFor="platform-server-addr">
              Server address
            </label>
            <Input
              disabled={loading || saving || !canEdit}
              id="platform-server-addr"
              onChange={(event) => setServerAddr(event.target.value)}
              placeholder="https://cloud.example.com"
              value={serverAddr}
            />
            <Button disabled={loading || saving || !canEdit || !dirty} onClick={save} type="button">
              {saving ? <Loader2 className="animate-spin" /> : <Save />}
              Save
            </Button>
          </div>
          {!canEdit ? <p className="text-sm text-muted-foreground">Only platform admins can change this value.</p> : null}
        </div>
      </section>
    </div>
  );
}
