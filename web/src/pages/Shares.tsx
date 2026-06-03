import { useCallback, useEffect, useState } from "react";
import type { ReactNode } from "react";
import { Ban, Clock, Copy, Plus, RefreshCw, Share2, Trash2 } from "lucide-react";

import { useSearchParams, useTenantSlug } from "@/lib/router";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import {
  listShares,
  createShare,
  revokeShare,
  deleteShare,
  setShareExpiry,
  type ShareVM,
} from "@/services/shares";

const TTL_OPTIONS = [
  { label: "1 hour", sec: 3600 },
  { label: "1 day", sec: 86400 },
  { label: "1 week", sec: 604800 },
  { label: "30 days", sec: 2592000 },
];

export function Shares() {
  const tenantSlug = useTenantSlug() ?? "";
  const [searchParams] = useSearchParams();
  const targetFilter = searchParams.get("targetId") ?? "";

  const [rows, setRows] = useState<ShareVM[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [newTarget, setNewTarget] = useState(targetFilter);
  const [newTtl, setNewTtl] = useState(604800);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    if (!tenantSlug) return;
    setLoading(true);
    setError(null);
    try {
      setRows(await listShares(tenantSlug, targetFilter));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [tenantSlug, targetFilter]);

  useEffect(() => {
    void load();
  }, [load]);

  const run = useCallback(
    async (fn: () => Promise<unknown>) => {
      setBusy(true);
      setError(null);
      try {
        await fn();
        await load();
      } catch (err) {
        setError(err instanceof Error ? err.message : String(err));
      } finally {
        setBusy(false);
      }
    },
    [load],
  );

  const onCreate = () =>
    void run(() => createShare(tenantSlug, newTarget.trim(), { ttlSec: newTtl }));

  const publicUrl = (token: string) =>
    `${window.location.origin}/shared/${token}`;

  return (
    <div className="min-h-full bg-background text-foreground">
      <div className="mx-auto flex max-w-[1600px] flex-col gap-4 p-4 md:p-6">
        <div className="flex flex-col gap-3 border-b border-border pb-4 md:flex-row md:items-center md:justify-between">
          <div>
            <div className="flex items-center gap-2 text-lg font-semibold">
              <Share2 className="h-5 w-5 text-primary" />
              <h1>Shares</h1>
            </div>
            <div className="mt-1 text-xs text-muted-foreground">
              {rows.length} shares{targetFilter && ` for run ${targetFilter.slice(0, 8)}`}
            </div>
          </div>
          <Button variant="outline" size="sm" onClick={() => void load()} disabled={loading}>
            <RefreshCw className={cn("h-4 w-4", loading && "animate-spin")} /> Refresh
          </Button>
        </div>

        {/* create */}
        <div className="flex flex-wrap items-end gap-2 border border-border bg-muted/20 p-3">
          <div className="flex flex-col gap-1">
            <label className="text-[11px] uppercase text-muted-foreground">Target run ID</label>
            <Input
              value={newTarget}
              onChange={(e) => setNewTarget(e.target.value)}
              placeholder="run id"
              className="h-9 w-[320px]"
            />
          </div>
          <div className="flex flex-col gap-1">
            <label className="text-[11px] uppercase text-muted-foreground">TTL</label>
            <select
              value={newTtl}
              onChange={(e) => setNewTtl(Number(e.target.value))}
              className="h-9 border border-border bg-background px-2 text-sm"
            >
              {TTL_OPTIONS.map((o) => (
                <option key={o.sec} value={o.sec}>{o.label}</option>
              ))}
            </select>
          </div>
          <Button size="sm" onClick={onCreate} disabled={busy || !newTarget.trim()}>
            <Plus className="h-4 w-4" /> Create share
          </Button>
        </div>

        {error && (
          <div className="border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </div>
        )}

        <div className="overflow-x-auto border border-border">
          <table className="min-w-[980px] w-full border-collapse text-sm">
            <thead className="bg-muted/60 text-xs uppercase text-muted-foreground">
              <tr className="border-b border-border">
                <Th>Target</Th><Th>Token / Link</Th><Th>State</Th><Th>Expires</Th><Th>Captured</Th><Th>Actions</Th>
              </tr>
            </thead>
            <tbody>
              {rows.map((s) => (
                <tr key={s.id} className="border-b border-border/70 hover:bg-muted/30">
                  <Td>
                    <div className="font-mono text-xs">{s.snapshotName || s.targetId.slice(0, 12)}</div>
                    <div className="text-[11px] text-muted-foreground">{s.targetKind || "—"}</div>
                  </Td>
                  <Td>
                    <button
                      className="inline-flex items-center gap-1 font-mono text-xs text-primary hover:underline"
                      onClick={() => void navigator.clipboard?.writeText(publicUrl(s.token))}
                    >
                      <Copy className="h-3 w-3" /> {s.token.slice(0, 16)}…
                    </button>
                  </Td>
                  <Td>
                    {s.revoked ? <Badge variant="destructive">revoked</Badge> : <Badge variant="success">active</Badge>}
                  </Td>
                  <Td>{fmtTime(s.expiresAt)}</Td>
                  <Td>{fmtTime(s.capturedAt)}</Td>
                  <Td>
                    <div className="flex items-center gap-1">
                      <IconBtn title="Extend 1 week" disabled={busy} onClick={() => void run(() => setShareExpiry(tenantSlug, s.id, 604800))}>
                        <Clock className="h-4 w-4" />
                      </IconBtn>
                      <IconBtn title="Revoke" disabled={busy || s.revoked} onClick={() => void run(() => revokeShare(tenantSlug, s.id))}>
                        <Ban className="h-4 w-4" />
                      </IconBtn>
                      <IconBtn title="Delete" disabled={busy} onClick={() => void run(() => deleteShare(tenantSlug, s.id))}>
                        <Trash2 className="h-4 w-4" />
                      </IconBtn>
                    </div>
                  </Td>
                </tr>
              ))}
              {rows.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-10 text-center text-sm text-muted-foreground">
                    {loading ? "Loading..." : "No shares."}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

function IconBtn({ children, title, onClick, disabled }: { children: ReactNode; title: string; onClick: () => void; disabled?: boolean }) {
  return (
    <button
      title={title}
      onClick={onClick}
      disabled={disabled}
      className="border border-border p-1.5 text-muted-foreground hover:bg-muted/50 hover:text-foreground disabled:opacity-40"
    >
      {children}
    </button>
  );
}
function Th({ children }: { children: ReactNode }) {
  return <th className="px-3 py-2 text-left font-medium">{children}</th>;
}
function Td({ children, className }: { children: ReactNode; className?: string }) {
  return <td className={cn("px-3 py-2 align-middle", className)}>{children}</td>;
}
function fmtTime(iso?: string): string {
  if (!iso) return "never";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return new Intl.DateTimeFormat(undefined, { dateStyle: "short", timeStyle: "short" }).format(d);
}
