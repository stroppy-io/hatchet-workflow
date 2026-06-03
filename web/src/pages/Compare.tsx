import { useCallback, useEffect, useState } from "react";
import type { ReactNode } from "react";
import { GitCompare, RefreshCw } from "lucide-react";

import { useSearchParams, useTenantSlug } from "@/lib/router";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { compareRuns, type CompareVM } from "@/services/compare";

export function Compare() {
  const tenantSlug = useTenantSlug() ?? "";
  const [searchParams, setSearchParams] = useSearchParams();
  const initial = (searchParams.get("runIds") ?? "").trim();

  const [idsText, setIdsText] = useState(initial);
  const [view, setView] = useState<CompareVM | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(
    async (raw: string) => {
      const runIds = raw.split(",").map((s) => s.trim()).filter(Boolean);
      if (!tenantSlug || runIds.length === 0) {
        setView(null);
        return;
      }
      setLoading(true);
      setError(null);
      try {
        setView(await compareRuns(tenantSlug, runIds));
      } catch (err) {
        setError(err instanceof Error ? err.message : String(err));
      } finally {
        setLoading(false);
      }
    },
    [tenantSlug],
  );

  useEffect(() => {
    if (initial) void load(initial);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initial, tenantSlug]);

  const submit = () => {
    setSearchParams(idsText.trim() ? { runIds: idsText.trim() } : {});
    void load(idsText);
  };

  const cols = view?.columns ?? [];

  return (
    <div className="min-h-full bg-background text-foreground">
      <div className="mx-auto flex max-w-[1600px] flex-col gap-4 p-4 md:p-6">
        <div className="flex flex-col gap-3 border-b border-border pb-4">
          <div className="flex items-center gap-2 text-lg font-semibold">
            <GitCompare className="h-5 w-5 text-primary" />
            <h1>Compare runs</h1>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Input
              value={idsText}
              onChange={(e) => setIdsText(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && submit()}
              placeholder="Comma-separated run IDs (first is baseline)"
              className="h-9 max-w-xl"
            />
            <Button variant="outline" size="sm" onClick={submit} disabled={loading}>
              <RefreshCw className={cn("h-4 w-4", loading && "animate-spin")} /> Compare
            </Button>
          </div>
        </div>

        {error && (
          <div className="border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </div>
        )}

        {cols.length > 0 && (
          <div className="overflow-x-auto border border-border">
            <table className="w-full border-collapse text-sm">
              <thead className="bg-muted/60 text-xs uppercase text-muted-foreground">
                <tr className="border-b border-border">
                  <Th>Config</Th>
                  {cols.map((c, i) => (
                    <Th key={c.runId} className="text-right">
                      {c.name || c.runId.slice(0, 8)}
                      {i === 0 && <span className="ml-1 text-[10px] text-primary">(baseline)</span>}
                    </Th>
                  ))}
                </tr>
              </thead>
              <tbody>
                <ConfigRow label="Status" cells={cols.map((c) => c.status)} />
                <ConfigRow label="DB" cells={cols.map((c) => c.dbKind || "—")} />
                <ConfigRow label="DB name" cells={cols.map((c) => c.dbName || "—")} />
                <ConfigRow label="Workload" cells={cols.map((c) => c.workloadName || "—")} />
                <ConfigRow label="Stroppy" cells={cols.map((c) => c.stroppyVersion || "—")} />
                <ConfigRow label="Provider" cells={cols.map((c) => c.provider || "—")} />
                <ConfigRow label="Topology" cells={cols.map((c) => c.topologyLabel || "—")} />
                <ConfigRow label="Nodes" cells={cols.map((c) => String(c.nodeCount))} />
                <ConfigRow label="Duration" cells={cols.map((c) => fmtDur(c.durationSec))} />
              </tbody>
            </table>
          </div>
        )}

        {(view?.metrics.length ?? 0) > 0 && (
          <div>
            <h2 className="mb-2 text-sm font-semibold">
              Metrics <span className="text-xs text-muted-foreground">({view!.metrics.length})</span>
            </h2>
            <div className="overflow-x-auto border border-border">
              <table className="w-full border-collapse text-sm">
                <thead className="bg-muted/60 text-xs uppercase text-muted-foreground">
                  <tr className="border-b border-border">
                    <Th>Metric</Th>
                    {cols.map((c) => (
                      <Th key={c.runId} className="text-right">{c.name || c.runId.slice(0, 8)}</Th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {view!.metrics.map((m) => (
                    <tr key={m.key} className="border-b border-border/70 hover:bg-muted/30">
                      <Td>
                        <div className="font-mono text-xs">{m.name}</div>
                        <div className="text-[11px] text-muted-foreground">
                          {m.unit} · {m.higherIsBetter ? "higher better" : "lower better"}
                        </div>
                      </Td>
                      {m.cells.map((cell, i) => (
                        <Td key={cell.runId || i} className="text-right tabular-nums">
                          <div>{fmtNum(cell.avg)}</div>
                          {i > 0 && cell.verdict && (
                            <div className="text-[11px]">
                              <Badge variant={verdictVariant(cell.verdict)}>
                                {fmtPct(cell.diffAvgPct)}
                              </Badge>
                            </div>
                          )}
                        </Td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {!loading && !view && !error && (
          <div className="px-4 py-10 text-center text-sm text-muted-foreground">
            Enter two or more run IDs to compare.
          </div>
        )}
      </div>
    </div>
  );
}

function ConfigRow({ label, cells }: { label: string; cells: string[] }) {
  return (
    <tr className="border-b border-border/70 hover:bg-muted/30">
      <Td className="text-xs font-medium text-muted-foreground">{label}</Td>
      {cells.map((c, i) => (
        <Td key={i} className="text-right">{c}</Td>
      ))}
    </tr>
  );
}

function verdictVariant(v: string): "success" | "destructive" | "secondary" {
  if (v === "VERDICT_BETTER") return "success";
  if (v === "VERDICT_WORSE") return "destructive";
  return "secondary";
}

function Th({ children, className }: { children: ReactNode; className?: string }) {
  return <th className={cn("px-3 py-2 text-left font-medium", className)}>{children}</th>;
}
function Td({ children, className }: { children: ReactNode; className?: string }) {
  return <td className={cn("px-3 py-2 align-middle", className)}>{children}</td>;
}
function fmtDur(sec?: number): string {
  if (sec === undefined || !Number.isFinite(sec)) return "—";
  if (sec < 60) return `${sec.toFixed(0)}s`;
  return `${Math.floor(sec / 60)}m ${Math.floor(sec % 60)}s`;
}
function fmtNum(v: number): string {
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(v);
}
function fmtPct(v: number): string {
  const sign = v > 0 ? "+" : "";
  return `${sign}${v.toFixed(1)}%`;
}
