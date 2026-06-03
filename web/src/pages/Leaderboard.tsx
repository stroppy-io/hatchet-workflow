import { useCallback, useEffect, useState } from "react";
import type { ReactNode } from "react";
import { RefreshCw, Trophy } from "lucide-react";

import { useTenantSlug } from "@/lib/router";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import {
  getTenantRating,
  getSystemRating,
  getPublicRating,
  type RatingEntryVM,
} from "@/services/rating";

type Scope = "tenant" | "system" | "public";

const SCOPES: { value: Scope; label: string }[] = [
  { value: "tenant", label: "This tenant" },
  { value: "system", label: "System-wide" },
  { value: "public", label: "Public" },
];

export function Leaderboard() {
  const tenantSlug = useTenantSlug() ?? "";
  const [scope, setScope] = useState<Scope>("tenant");
  const [metricKey, setMetricKey] = useState("ops_per_sec");
  const [rows, setRows] = useState<RatingEntryVM[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!tenantSlug && scope === "tenant") return;
    setLoading(true);
    setError(null);
    try {
      const query = { metricKey: metricKey.trim(), limit: 100 };
      const page =
        scope === "tenant"
          ? await getTenantRating(tenantSlug, query)
          : scope === "system"
            ? await getSystemRating(query)
            : await getPublicRating(query);
      setRows(page.entries);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [tenantSlug, scope, metricKey]);

  useEffect(() => {
    void load();
  }, [load]);

  const showAuthor = scope !== "public";
  const showTenant = scope === "system";

  return (
    <div className="min-h-full bg-background text-foreground">
      <div className="mx-auto flex max-w-[1600px] flex-col gap-4 p-4 md:p-6">
        <div className="flex flex-col gap-3 border-b border-border pb-4 md:flex-row md:items-center md:justify-between">
          <div>
            <div className="flex items-center gap-2 text-lg font-semibold">
              <Trophy className="h-5 w-5 text-primary" />
              <h1>Leaderboard</h1>
            </div>
            <div className="mt-1 text-xs text-muted-foreground">
              {rows.length} ranked benchmarks · metric {metricKey || "—"}
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <div className="flex border border-border">
              {SCOPES.map((s) => (
                <button
                  key={s.value}
                  onClick={() => setScope(s.value)}
                  className={cn(
                    "px-3 py-1.5 text-sm",
                    scope === s.value ? "bg-primary/15 text-primary" : "text-muted-foreground hover:bg-muted/40",
                  )}
                >
                  {s.label}
                </button>
              ))}
            </div>
            <Input
              value={metricKey}
              onChange={(e) => setMetricKey(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && void load()}
              placeholder="metric key"
              className="h-9 w-[180px]"
            />
            <Button variant="outline" size="sm" onClick={() => void load()} disabled={loading}>
              <RefreshCw className={cn("h-4 w-4", loading && "animate-spin")} /> Refresh
            </Button>
          </div>
        </div>

        {error && (
          <div className="border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </div>
        )}

        <div className="overflow-x-auto border border-border">
          <table className="min-w-[1000px] w-full border-collapse text-sm">
            <thead className="bg-muted/60 text-xs uppercase text-muted-foreground">
              <tr className="border-b border-border">
                <Th className="text-right">#</Th>
                <Th className="text-right">Value</Th>
                <Th>DB</Th>
                <Th>Workload</Th>
                <Th>Topology</Th>
                <Th className="text-right">Nodes</Th>
                <Th>Stroppy</Th>
                <Th>Provider</Th>
                {showAuthor && <Th>Author</Th>}
                {showTenant && <Th>Tenant</Th>}
                <Th>Run at</Th>
              </tr>
            </thead>
            <tbody>
              {rows.map((e, i) => (
                <tr key={`${e.rank}-${e.runId || i}`} className="border-b border-border/70 hover:bg-muted/30">
                  <Td className="text-right font-semibold tabular-nums">{e.rank}</Td>
                  <Td className="text-right tabular-nums">
                    {fmtNum(e.metricValue)} <span className="text-[11px] text-muted-foreground">{e.metricUnit}</span>
                  </Td>
                  <Td>{e.dbKind || "—"}</Td>
                  <Td>{e.workloadName || "—"}</Td>
                  <Td>{e.topologyLabel || "—"}</Td>
                  <Td className="text-right tabular-nums">{e.nodeCount}</Td>
                  <Td className="font-mono text-xs">{e.stroppyVersion || "—"}</Td>
                  <Td>{e.provider || "—"}</Td>
                  {showAuthor && <Td>{e.authorName || "—"}</Td>}
                  {showTenant && <Td>{e.tenantName || "—"}</Td>}
                  <Td>{fmtTime(e.runAt)}</Td>
                </tr>
              ))}
              {rows.length === 0 && (
                <tr>
                  <td colSpan={12} className="px-4 py-10 text-center text-sm text-muted-foreground">
                    {loading ? "Loading..." : "No ranked benchmarks for this metric."}
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

function Th({ children, className }: { children: ReactNode; className?: string }) {
  return <th className={cn("px-3 py-2 text-left font-medium", className)}>{children}</th>;
}
function Td({ children, className }: { children: ReactNode; className?: string }) {
  return <td className={cn("px-3 py-2 align-middle", className)}>{children}</td>;
}
function fmtNum(v: number): string {
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(v);
}
function fmtTime(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return new Intl.DateTimeFormat(undefined, { dateStyle: "short", timeStyle: "short" }).format(d);
}
