import { useCallback, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { Activity, RefreshCw } from "lucide-react";
import type { Timestamp } from "@bufbuild/protobuf/wkt";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { useTenantSlug } from "@/lib/router";
import { getQuotaProvider, type QuotaProviderFilter } from "@/services/quotas";
import type { QuotaReservationView, QuotaView } from "@/lib/proto/cloud/v1/api/quota_pb";
import { Provider as ProviderEnum } from "@/lib/proto/cloud/v1/deployment/provider_pb";
import { Quota_ReservationStatus } from "@/lib/proto/cloud/v1/deployment/quota_pb";

type ProviderFilter = QuotaProviderFilter;

const PROVIDERS: { value: ProviderFilter; label: string }[] = [
  { value: "all", label: "All providers" },
  { value: "yandex", label: "Yandex Cloud" },
  { value: "docker", label: "Docker" },
];

export function Quotas() {
  const tenantSlug = useTenantSlug() ?? "";
  const [provider, setProvider] = useState<ProviderFilter>("all");
  const [rows, setRows] = useState<QuotaView[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [runId, setRunId] = useState("");
  const [runRows, setRunRows] = useState<QuotaReservationView[]>([]);
  const [runLoading, setRunLoading] = useState(false);
  const [runError, setRunError] = useState<string | null>(null);

  const load = useCallback(
    async (force = false) => {
      if (!tenantSlug) return;
      setLoading(true);
      setError(null);
      try {
        const data = await getQuotaProvider().listQuotas(tenantSlug, {
          provider,
          refresh: force ? "force" : "refreshIfStale",
        });
        setRows(data);
      } catch (err) {
        setError(err instanceof Error ? err.message : String(err));
      } finally {
        setLoading(false);
      }
    },
    [provider, tenantSlug],
  );

  useEffect(() => {
    void load(false);
  }, [load]);

  const stats = useMemo(() => {
    return rows.reduce(
      (acc, row) => {
        acc.limit += finite(row.limit);
        acc.used += finite(row.providerUsed);
        acc.reserved += finite(row.reserved);
        acc.available += finite(row.availableForRuns);
        if (row.stale) acc.stale += 1;
        return acc;
      },
      { limit: 0, used: 0, reserved: 0, available: 0, stale: 0 },
    );
  }, [rows]);

  const loadRunUsage = useCallback(async () => {
    if (!tenantSlug || !runId.trim()) return;
    setRunLoading(true);
    setRunError(null);
    try {
      const data = await getQuotaProvider().getRunQuotaUsage(tenantSlug, runId.trim());
      setRunRows(data);
    } catch (err) {
      setRunError(err instanceof Error ? err.message : String(err));
    } finally {
      setRunLoading(false);
    }
  }, [runId, tenantSlug]);

  return (
    <div className="min-h-full bg-background text-foreground">
      <div className="mx-auto flex max-w-[1600px] flex-col gap-4 p-4 md:p-6">
        <div className="flex flex-col gap-3 border-b border-border pb-4 md:flex-row md:items-center md:justify-between">
          <div className="min-w-0">
            <div className="flex items-center gap-2 text-lg font-semibold">
              <Activity className="h-5 w-5 text-primary" />
              <h1>Quotas</h1>
            </div>
            <div className="mt-1 text-xs text-muted-foreground">
              {rows.length} rows · {stats.stale} stale
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Select value={provider} onValueChange={(value) => setProvider(value as ProviderFilter)}>
              <SelectTrigger className="h-9 w-[180px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PROVIDERS.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button variant="outline" size="sm" onClick={() => void load(true)} disabled={loading}>
              <RefreshCw className={cn("h-4 w-4", loading && "animate-spin")} />
              Refresh
            </Button>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
          <Stat label="Limit" value={fmt(stats.limit)} />
          <Stat label="Provider used" value={fmt(stats.used)} />
          <Stat label="Reserved" value={fmt(stats.reserved)} />
          <Stat label="Available" value={fmt(stats.available)} />
        </div>

        {error && (
          <div className="border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </div>
        )}

        <div className="overflow-hidden border border-border">
          <div className="overflow-x-auto">
            <table className="min-w-[1120px] w-full border-collapse text-sm">
              <thead className="bg-muted/60 text-xs uppercase text-muted-foreground">
                <tr className="border-b border-border">
                  <Th>Provider</Th>
                  <Th>Service</Th>
                  <Th>Quota</Th>
                  <Th className="text-right">Used</Th>
                  <Th className="text-right">Limit</Th>
                  <Th className="text-right">Reserved</Th>
                  <Th className="text-right">Available</Th>
                  <Th>Observed</Th>
                  <Th>Stale after</Th>
                </tr>
              </thead>
              <tbody>
                {rows.map((row) => (
                  <tr
                    key={`${row.info?.provider ?? 0}:${row.resourceId}:${row.info?.name ?? ""}`}
                    className="border-b border-border/70 hover:bg-muted/30"
                  >
                    <Td>{providerLabel(row.info?.provider)}</Td>
                    <Td>{row.service || "—"}</Td>
                    <Td>
                      <div className="font-mono text-xs text-foreground">{row.info?.name || "—"}</div>
                      <div className="mt-0.5 text-[11px] text-muted-foreground">
                        {row.resourceType} · {row.resourceId}
                      </div>
                    </Td>
                    <Td className="text-right tabular-nums">{fmt(row.providerUsed)} {row.info?.units}</Td>
                    <Td className="text-right tabular-nums">{fmt(row.limit)} {row.info?.units}</Td>
                    <Td className="text-right tabular-nums">{fmt(row.reserved)} {row.info?.units}</Td>
                    <Td className="text-right tabular-nums">{fmt(row.availableForRuns)} {row.info?.units}</Td>
                    <Td>{formatTimestamp(row.observedAt)}</Td>
                    <Td>
                      <span className={cn(row.stale && "text-warning")}>
                        {formatTimestamp(row.staleAfter)}
                      </span>
                    </Td>
                  </tr>
                ))}
                {!loading && rows.length === 0 && (
                  <tr>
                    <td colSpan={9} className="px-4 py-10 text-center text-sm text-muted-foreground">
                      No quota snapshots.
                    </td>
                  </tr>
                )}
                {loading && rows.length === 0 && (
                  <tr>
                    <td colSpan={9} className="px-4 py-10 text-center text-sm text-muted-foreground">
                      Loading quotas...
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>

        <div className="mt-2 border-t border-border pt-4">
          <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
            <div>
              <h2 className="text-base font-semibold">Run quota usage</h2>
              <div className="mt-1 text-xs text-muted-foreground">{runRows.length} ledger rows</div>
            </div>
            <div className="flex w-full flex-col gap-2 sm:w-auto sm:flex-row">
              <Input
                value={runId}
                onChange={(event) => setRunId(event.target.value)}
                placeholder="Run ID"
                className="sm:w-[320px]"
              />
              <Button size="sm" variant="outline" onClick={() => void loadRunUsage()} disabled={runLoading || !runId.trim()}>
                <RefreshCw className={cn("h-4 w-4", runLoading && "animate-spin")} />
                Load
              </Button>
            </div>
          </div>

          {runError && (
            <div className="mt-3 border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {runError}
            </div>
          )}

          <div className="mt-3 overflow-hidden border border-border">
            <div className="overflow-x-auto">
              <table className="min-w-[860px] w-full border-collapse text-sm">
                <thead className="bg-muted/60 text-xs uppercase text-muted-foreground">
                  <tr className="border-b border-border">
                    <Th>Node</Th>
                    <Th>Quota</Th>
                    <Th>Status</Th>
                    <Th className="text-right">Amount</Th>
                    <Th>Created</Th>
                    <Th>Expires</Th>
                  </tr>
                </thead>
                <tbody>
                  {runRows.map((row) => (
                    <tr key={row.id} className="border-b border-border/70 hover:bg-muted/30">
                      <Td>{row.nodeId}</Td>
                      <Td>
                        <div className="font-mono text-xs">{row.info?.name || "—"}</div>
                        <div className="mt-0.5 text-[11px] text-muted-foreground">
                          {row.resourceType} · {row.resourceId}
                        </div>
                      </Td>
                      <Td>{reservationStatus(row.status)}</Td>
                      <Td className="text-right tabular-nums">{amount(row.amount)} {row.info?.units}</Td>
                      <Td>{formatTimestamp(row.createdAt)}</Td>
                      <Td>{formatTimestamp(row.expiresAt)}</Td>
                    </tr>
                  ))}
                  {!runLoading && runRows.length === 0 && (
                    <tr>
                      <td colSpan={6} className="px-4 py-8 text-center text-sm text-muted-foreground">
                        No run quota rows.
                      </td>
                    </tr>
                  )}
                  {runLoading && runRows.length === 0 && (
                    <tr>
                      <td colSpan={6} className="px-4 py-8 text-center text-sm text-muted-foreground">
                        Loading run quotas...
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="border border-border bg-muted/20 px-3 py-2">
      <div className="text-[11px] uppercase text-muted-foreground">{label}</div>
      <div className="mt-1 text-lg font-semibold tabular-nums">{value}</div>
    </div>
  );
}

function Th({ children, className }: { children: ReactNode; className?: string }) {
	return <th className={cn("px-3 py-2 text-left font-medium", className)}>{children}</th>;
}

function Td({ children, className }: { children: ReactNode; className?: string }) {
	return <td className={cn("px-3 py-2 align-middle", className)}>{children}</td>;
}

function finite(value: number): number {
  return Number.isFinite(value) ? value : 0;
}

function fmt(value: number): string {
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 }).format(finite(value));
}

function formatTimestamp(ts?: Timestamp): string {
  if (!ts) return "—";
  const millis = Number(ts.seconds) * 1000 + Math.floor(ts.nanos / 1_000_000);
  if (!Number.isFinite(millis) || millis <= 0) return "—";
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "short",
    timeStyle: "short",
  }).format(new Date(millis));
}

function providerLabel(provider?: ProviderEnum): string {
  switch (provider) {
    case ProviderEnum.DOCKER:
      return "Docker";
    case ProviderEnum.YANDEX:
      return "Yandex";
    default:
      return "—";
  }
}

function amount(value: bigint): string {
  return new Intl.NumberFormat().format(Number(value));
}

function reservationStatus(status: Quota_ReservationStatus): string {
  switch (status) {
    case Quota_ReservationStatus.RESERVED:
      return "Reserved";
    case Quota_ReservationStatus.ALLOCATED:
      return "Allocated";
    case Quota_ReservationStatus.RELEASED:
      return "Released";
    case Quota_ReservationStatus.EXPIRED:
      return "Expired";
    case Quota_ReservationStatus.FAILED:
      return "Failed";
    default:
      return "—";
  }
}
