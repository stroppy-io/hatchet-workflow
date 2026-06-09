// Metrics tab. The snapshot exposes computed metric summaries (avg/min/max/last
// per key), so this mirrors the Compare page's dense metric table: grouped
// rows, searchable metric identity, unit badges, direction hints and
// unit-aware values.
import { Fragment, useMemo, useState } from "react";
import { ArrowDown, ArrowUp, Minus, Search, TrendingDown, TrendingUp } from "lucide-react";
import type { MetricVM } from "@/services/run_overview";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";

const GROUP_ORDER = [
  "throughput",
  "latency",
  "errors",
  "connections",
  "replication",
  "resources",
  "system",
  "stroppy",
  "other",
];

// Fallback bucketing when a metric carries no group, keyed by key prefix.
const CATEGORY: { label: string; match: (k: string) => boolean }[] = [
  { label: "Database", match: (k) => k.startsWith("db_") },
  { label: "System", match: (k) => /^(cpu|memory|mem|disk|net)_/.test(k) },
  { label: "Stroppy", match: (k) => k.startsWith("stroppy_") || k.startsWith("k6_") },
];

function categoryOf(m: MetricVM): string {
  if (m.group) return normalizeGroup(m.group);
  for (const c of CATEGORY) if (c.match(m.key)) return c.label;
  return "Other";
}

function fmt(value: number, unit: string): string {
  if (!Number.isFinite(value) || value === 0) return "—";
  if (unit === "bytes") return formatBytes(value);
  if (unit === "bytes/s" || unit === "B/s") return `${formatBytes(value)}/s`;
  if (unit === "%") return `${trimFixed(value, 2)}%`;
  if (unit === "s" || unit === "ms") return `${trimFixed(value, 2)} ${unit}`;
  if (unit === "ops/s" || unit === "txn/s" || unit === "q/s" || unit === "req/s" || unit === "iter/s" || unit === "err/s") {
    return `${trimFixed(value, 1)} ${unit}`;
  }
  const formatted = value >= 1e6 || value <= -1e6
    ? `${trimFixed(value / 1e6, 2)}M`
    : value >= 1e3 || value <= -1e3
      ? `${trimFixed(value / 1e3, 2)}K`
      : trimFixed(value, 2);
  return unit ? `${formatted} ${unit}` : formatted;
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
  const [query, setQuery] = useState("");
  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return metrics;
    return metrics.filter((m) =>
      [m.name, m.key, m.unit, m.group, m.description].some((value) => value.toLowerCase().includes(q)),
    );
  }, [metrics, query]);

  const groups = useMemo(() => {
    const by = new Map<string, MetricVM[]>();
    for (const m of visible) {
      const g = categoryOf(m);
      const arr = by.get(g) ?? [];
      arr.push(m);
      by.set(g, arr);
    }
    return Array.from(by.entries()).sort(
      ([left], [right]) => groupRank(left) - groupRank(right) || groupLabel(left).localeCompare(groupLabel(right)),
    );
  }, [visible]);

  if (metrics.length === 0) {
    return (
      <div className="border border-border px-4 py-10 text-center font-mono text-sm text-muted-foreground">
        No metrics yet.
      </div>
    );
  }

  return (
    <div className="overflow-hidden border border-border bg-background">
      <div className="flex flex-col gap-3 border-b border-border bg-muted/20 px-3 py-3 md:flex-row md:items-center md:justify-between">
        <div className="flex items-center gap-2">
          <h2 className="text-sm font-semibold">Metrics</h2>
          <span className="font-mono text-[11px] text-muted-foreground">
            {visible.length} of {metrics.length}
          </span>
        </div>
        <div className="relative w-full md:max-w-sm">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Filter metrics"
            className="h-8 pl-8 text-xs"
          />
        </div>
      </div>

      {visible.length === 0 ? (
        <div className="px-4 py-10 text-center text-sm text-muted-foreground">No metrics match the filter.</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[860px] border-collapse text-sm">
            <thead className="bg-muted/50 text-xs text-muted-foreground">
              <tr className="border-b border-border">
                <th className="sticky left-0 z-20 w-[340px] min-w-[340px] border-r border-border bg-muted/50 px-3 py-2 text-left font-medium">
                  Metric
                </th>
                <th className="w-12 px-3 py-2 text-center font-medium">Trend</th>
                <th className="px-3 py-2 text-right font-medium">Last</th>
                <th className="px-3 py-2 text-right font-medium">Avg</th>
                <th className="px-3 py-2 text-right font-medium">Min</th>
                <th className="px-3 py-2 text-right font-medium">Max</th>
              </tr>
            </thead>
            <tbody>
              {groups.map(([group, rows]) => (
                <Fragment key={group}>
                  <tr className="border-b border-border bg-muted/30">
                    <td colSpan={6} className="px-3 py-1.5 text-[11px] font-medium uppercase text-muted-foreground">
                      {groupLabel(group)} <span className="font-mono">({rows.length})</span>
                    </td>
                  </tr>
                  {rows.map((m) => (
                    <tr key={m.key} className="border-b border-border/70 last:border-b-0 hover:bg-muted/20">
                      <td className="sticky left-0 z-10 w-[340px] min-w-[340px] border-r border-border bg-background px-3 py-2">
                        <MetricIdentity metric={m} />
                      </td>
                      <td className="px-3 py-2 text-center align-middle">
                        <span className="inline-flex"><Trend m={m} /></span>
                      </td>
                      <ValueCell value={m.last} unit={m.unit} strong />
                      <ValueCell value={m.avg} unit={m.unit} />
                      <ValueCell value={m.min} unit={m.unit} />
                      <ValueCell value={m.max} unit={m.unit} />
                    </tr>
                  ))}
                </Fragment>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function MetricIdentity({ metric }: { metric: MetricVM }) {
  return (
    <div className="flex min-w-0 items-start justify-between gap-3">
      <div className="min-w-0">
        <div className="truncate text-sm font-medium text-foreground" title={metric.description || metric.name}>
          {metric.name}
        </div>
        <div className="mt-1 flex flex-wrap items-center gap-1.5">
          <Badge variant="outline" className="max-w-[190px] truncate font-mono text-[10px]">
            {metric.key}
          </Badge>
          {metric.unit && <Badge variant="secondary">{metric.unit}</Badge>}
        </div>
      </div>
      <DirectionBadge higherIsBetter={metric.higherIsBetter} />
    </div>
  );
}

function DirectionBadge({ higherIsBetter }: { higherIsBetter: boolean }) {
  const Icon = higherIsBetter ? ArrowUp : ArrowDown;
  return (
    <Badge variant="secondary" className="shrink-0 gap-1 whitespace-nowrap">
      <Icon className={cn("h-3 w-3", higherIsBetter ? "text-success" : "text-primary")} />
      {higherIsBetter ? "higher is better" : "lower is better"}
    </Badge>
  );
}

function ValueCell({ value, unit, strong = false }: { value: number; unit: string; strong?: boolean }) {
  return (
    <td
      className={cn(
        "px-3 py-2 text-right font-mono text-xs tabular-nums",
        strong ? "text-foreground" : "text-muted-foreground",
      )}
    >
      {fmt(value, unit)}
    </td>
  );
}

function normalizeGroup(group: string): string {
  return group.trim().toLowerCase() || "other";
}

function groupRank(group: string): number {
  const index = GROUP_ORDER.indexOf(group);
  return index === -1 ? GROUP_ORDER.length : index;
}

function groupLabel(group: string): string {
  const labels: Record<string, string> = {
    throughput: "Throughput",
    latency: "Latency",
    errors: "Errors",
    connections: "Connections",
    replication: "Replication",
    resources: "Resources",
    system: "System",
    stroppy: "Stroppy",
    other: "Other",
  };
  return labels[group] ?? group;
}

function formatBytes(value: number): string {
  const abs = Math.abs(value);
  if (abs >= 1e12) return `${trimFixed(value / 1e12, 2)} TB`;
  if (abs >= 1e9) return `${trimFixed(value / 1e9, 2)} GB`;
  if (abs >= 1e6) return `${trimFixed(value / 1e6, 2)} MB`;
  if (abs >= 1e3) return `${trimFixed(value / 1e3, 2)} KB`;
  return `${trimFixed(value, 0)} B`;
}

function trimFixed(value: number, digits: number): string {
  return value.toFixed(digits).replace(/\.0+$/, "").replace(/(\.\d*?)0+$/, "$1");
}
