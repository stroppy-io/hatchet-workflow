// Metrics tab. The new snapshot exposes computed metric summaries (avg/min/max/
// last per key) — no raw series — so this renders a dense, grouped, formatted
// table (an upgrade on the legacy flat list): metrics are bucketed by their
// reported group (falling back to a key-derived category), values are
// unit-aware, and a trend glyph encodes higher/lower-is-better against avg.
import { useMemo } from "react";
import { Minus, TrendingDown, TrendingUp } from "lucide-react";
import type { MetricVM } from "@/services/run_overview";
import { cn } from "@/lib/utils";

// Fallback bucketing when a metric carries no group, keyed by key prefix.
const CATEGORY: { label: string; match: (k: string) => boolean }[] = [
  { label: "Database", match: (k) => k.startsWith("db_") },
  { label: "System", match: (k) => /^(cpu|memory|mem|disk|net)_/.test(k) },
  { label: "Stroppy", match: (k) => k.startsWith("stroppy_") || k.startsWith("k6_") },
];

function categoryOf(m: MetricVM): string {
  if (m.group) return m.group;
  for (const c of CATEGORY) if (c.match(m.key)) return c.label;
  return "Other";
}

function fmt(value: number, unit: string): string {
  if (!Number.isFinite(value) || value === 0) return "—";
  if (unit === "%" || unit === "s" || unit === "ms") return value.toFixed(2);
  if (unit === "ops/s" || unit === "errors/s" || unit === "req/s") return value.toFixed(1);
  if (unit === "bytes/s" || unit === "B/s") {
    if (value >= 1e9) return `${(value / 1e9).toFixed(1)} GB/s`;
    if (value >= 1e6) return `${(value / 1e6).toFixed(1)} MB/s`;
    if (value >= 1e3) return `${(value / 1e3).toFixed(1)} KB/s`;
    return `${value.toFixed(0)} B/s`;
  }
  if (value >= 1e6) return `${(value / 1e6).toFixed(2)}M`;
  if (value >= 1e3) return `${(value / 1e3).toFixed(2)}K`;
  return value.toFixed(2);
}

// Trend of `last` vs `avg`, coloured by whether higher is better.
function Trend({ m }: { m: MetricVM }) {
  if (!Number.isFinite(m.last) || m.last === 0 || !Number.isFinite(m.avg) || m.avg === 0)
    return <Minus className="h-3 w-3 text-muted-foreground/50" />;
  const up = m.last >= m.avg;
  const good = up === m.higherIsBetter;
  const Icon = up ? TrendingUp : TrendingDown;
  return <Icon className={cn("h-3 w-3", good ? "text-success" : "text-warning")} />;
}

export function MetricsPanel({ metrics }: { metrics: MetricVM[] }) {
  const groups = useMemo(() => {
    const by = new Map<string, MetricVM[]>();
    for (const m of metrics) {
      const g = categoryOf(m);
      const arr = by.get(g) ?? [];
      arr.push(m);
      by.set(g, arr);
    }
    return Array.from(by.entries()).sort((a, b) => a[0].localeCompare(b[0]));
  }, [metrics]);

  if (metrics.length === 0) {
    return (
      <div className="border border-border px-4 py-10 text-center font-mono text-sm text-muted-foreground">
        No metrics yet.
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {groups.map(([group, rows]) => (
        <div key={group} className="overflow-hidden border border-border">
          <div className="border-b border-border bg-muted/40 px-3 py-1.5 font-mono text-[11px] uppercase tracking-[0.14em] text-foreground/80">
            {group} <span className="text-muted-foreground">({rows.length})</span>
          </div>
          <table className="w-full border-collapse text-sm">
            <thead className="bg-muted/30 text-[11px] uppercase text-muted-foreground">
              <tr className="border-b border-border">
                <th className="px-3 py-1.5 text-left font-medium">Metric</th>
                <th className="w-8" />
                <th className="px-3 py-1.5 text-right font-medium">Last</th>
                <th className="px-3 py-1.5 text-right font-medium">Avg</th>
                <th className="px-3 py-1.5 text-right font-medium">Min</th>
                <th className="px-3 py-1.5 text-right font-medium">Max</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((m) => (
                <tr key={m.key} className="border-b border-border/60 last:border-0 hover:bg-muted/20">
                  <td className="px-3 py-1.5">
                    <div className="font-mono text-xs text-foreground/90" title={m.description}>
                      {m.name}
                    </div>
                    {m.unit && <div className="font-mono text-[10px] text-muted-foreground">{m.unit}</div>}
                  </td>
                  <td className="text-center align-middle">
                    <span className="inline-flex"><Trend m={m} /></span>
                  </td>
                  <td className="px-3 py-1.5 text-right font-mono text-xs tabular-nums text-foreground/90">
                    {fmt(m.last, m.unit)}
                  </td>
                  <td className="px-3 py-1.5 text-right font-mono text-xs tabular-nums text-muted-foreground">
                    {fmt(m.avg, m.unit)}
                  </td>
                  <td className="px-3 py-1.5 text-right font-mono text-xs tabular-nums text-muted-foreground">
                    {fmt(m.min, m.unit)}
                  </td>
                  <td className="px-3 py-1.5 text-right font-mono text-xs tabular-nums text-muted-foreground">
                    {fmt(m.max, m.unit)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ))}
    </div>
  );
}
