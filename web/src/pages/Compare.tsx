import { useCallback, useEffect, useMemo, useState } from "react";
import type { FormEvent, ReactNode, TdHTMLAttributes, ThHTMLAttributes } from "react";
import {
  ArrowDown,
  ArrowUp,
  GitCompare,
  RefreshCw,
  Search,
  TriangleAlert,
} from "lucide-react";

import { useSearchParams, useTenantSlug } from "@/lib/router";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import {
  compareRuns,
  type CompareCellVM,
  type CompareColumnVM,
  type CompareMetricVM,
  type CompareSummaryVM,
  type CompareVM,
} from "@/services/compare";

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

export function Compare() {
  const tenantSlug = useTenantSlug() ?? "";
  const [searchParams, setSearchParams] = useSearchParams();
  const initial = (searchParams.get("runIds") ?? "").trim();

  const [idsText, setIdsText] = useState(initial);
  const [metricSearch, setMetricSearch] = useState("");
  const [view, setView] = useState<CompareVM | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(
    async (raw: string) => {
      const runIds = parseRunIds(raw);
      if (!tenantSlug || runIds.length === 0) {
        setView(null);
        return;
      }
      if (runIds.length < 2) {
        setView(null);
        setError("At least two run IDs are required.");
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

  const submit = (event?: FormEvent) => {
    event?.preventDefault();
    const runIds = parseRunIds(idsText);
    setSearchParams(runIds.length > 0 ? { runIds: runIds.join(",") } : {});
    void load(runIds.join(","));
  };

  const cols = view?.columns ?? [];
  const baseline = cols[0];
  const summaryByRun = useMemo(() => {
    const byRun = new Map<string, CompareSummaryVM>();
    for (const summary of view?.summaries ?? []) {
      byRun.set(summary.runId, summary);
    }
    return byRun;
  }, [view?.summaries]);

  const visibleMetrics = useMemo(() => {
    const query = normalizeSearch(metricSearch);
    const metrics = view?.metrics ?? [];
    if (!query) return metrics;
    return metrics.filter((metric) =>
      includesQuery([metric.name, metric.key, metric.unit, metric.group, metric.description], query),
    );
  }, [metricSearch, view?.metrics]);

  const metricGroups = useMemo(() => groupMetrics(visibleMetrics), [visibleMetrics]);

  return (
    <main className="min-h-full bg-background text-foreground">
      <div className="mx-auto flex max-w-[1600px] flex-col gap-4 p-4 md:p-6">
        <header className="flex flex-col gap-3 border-b border-border pb-4 xl:flex-row xl:items-end xl:justify-between">
          <div className="min-w-0">
            <div className="flex items-center gap-2 text-lg font-semibold">
              <GitCompare className="h-5 w-5 text-primary" />
              <h1>Compare runs</h1>
            </div>
            <div className="mt-1 text-xs text-muted-foreground">
              {view
                ? `${view.metrics.length} metrics · baseline ${runLabel(baseline)}`
                : "Baseline is the first run in the list."}
            </div>
          </div>

          <form className="flex w-full flex-col gap-2 sm:flex-row xl:max-w-3xl" onSubmit={submit}>
            <Input
              value={idsText}
              onChange={(event) => setIdsText(event.target.value)}
              placeholder="run-a, run-b, run-c"
              className="h-9 min-w-0 flex-1 font-mono text-xs"
            />
            <Button variant="outline" size="sm" type="submit" disabled={loading}>
              <RefreshCw className={cn("h-4 w-4", loading && "animate-spin")} />
              Compare
            </Button>
          </form>
        </header>

        {error && (
          <div className="flex items-start gap-2 border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {cols.length > 0 && (
          <>
            <section className="grid gap-2 md:grid-cols-2 xl:grid-cols-4">
              <BaselineTile column={baseline} />
              {cols.slice(1).map((column) => (
                <SummaryTile key={column.runId} column={column} summary={summaryByRun.get(column.runId)} />
              ))}
            </section>

            <section className="overflow-hidden border border-border bg-background">
              <SectionTitle
                title="Run config"
                meta={`${cols.length} runs`}
              />
              <div className="overflow-x-auto">
                <table className="w-full min-w-[900px] border-collapse text-sm">
                  <thead className="bg-muted/50 text-xs text-muted-foreground">
                    <tr className="border-b border-border">
                      <Th className="w-[220px]">Config</Th>
                      {cols.map((column, index) => (
                        <Th key={column.runId} className="min-w-[180px] text-right">
                          <ColumnHead column={column} baseline={index === 0} />
                        </Th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    <ConfigRow label="Status" cells={cols.map((column) => column.status)} />
                    <ConfigRow label="Database" cells={cols.map((column) => column.dbKind || "-")} />
                    <ConfigRow label="DB preset" cells={cols.map((column) => column.dbName || "-")} />
                    <ConfigRow label="Workload" cells={cols.map((column) => column.workloadName || "-")} />
                    <ConfigRow label="Stroppy" cells={cols.map((column) => column.stroppyVersion || "-")} />
                    <ConfigRow label="Provider" cells={cols.map((column) => column.provider || "-")} />
                    <ConfigRow label="Topology" cells={cols.map((column) => column.topologyLabel || "-")} />
                    <ConfigRow label="Nodes" cells={cols.map((column) => formatInteger(column.nodeCount))} />
                    <ConfigRow label="Duration" cells={cols.map((column) => fmtDur(column.durationSec))} />
                  </tbody>
                </table>
              </div>
            </section>
          </>
        )}

        {view && (
          <section className="overflow-hidden border border-border bg-background">
            <div className="flex flex-col gap-3 border-b border-border bg-muted/20 px-3 py-3 md:flex-row md:items-center md:justify-between">
              <SectionTitle
                title="Metrics"
                meta={`${visibleMetrics.length} of ${view.metrics.length}`}
                flush
              />
              <div className="relative w-full md:max-w-sm">
                <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={metricSearch}
                  onChange={(event) => setMetricSearch(event.target.value)}
                  placeholder="Filter metrics"
                  className="h-8 pl-8 text-xs"
                />
              </div>
            </div>

            {view.metrics.length === 0 ? (
              <EmptyState>No metrics returned for these runs.</EmptyState>
            ) : visibleMetrics.length === 0 ? (
              <EmptyState>No metrics match the filter.</EmptyState>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full min-w-[980px] border-collapse text-sm">
                  <thead className="bg-muted/50 text-xs text-muted-foreground">
                    <tr className="border-b border-border">
                      <Th className="sticky left-0 z-20 w-[340px] min-w-[340px] border-r border-border bg-muted/50">
                        Metric
                      </Th>
                      {cols.map((column, index) => (
                        <Th key={column.runId} className="min-w-[220px] text-right">
                          <ColumnHead column={column} baseline={index === 0} />
                        </Th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {metricGroups.map((group) => (
                      <MetricGroupRows key={group.key} group={group} columns={cols} />
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>
        )}

        {!loading && !view && !error && (
          <div className="border border-dashed border-border px-4 py-10 text-center text-sm text-muted-foreground">
            Select two or more runs to compare.
          </div>
        )}
      </div>
    </main>
  );
}

function BaselineTile({ column }: { column?: CompareColumnVM }) {
  return (
    <div className="border border-border bg-muted/20 px-3 py-2">
      <div className="text-[11px] font-medium uppercase text-muted-foreground">Baseline</div>
      <div className="mt-1 truncate text-sm font-semibold">{runLabel(column)}</div>
      <div className="mt-1 font-mono text-[11px] text-muted-foreground">{shortId(column?.runId)}</div>
    </div>
  );
}

function SummaryTile({ column, summary }: { column: CompareColumnVM; summary?: CompareSummaryVM }) {
  return (
    <div className="border border-border bg-background px-3 py-2">
      <div className="flex min-w-0 items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold">{runLabel(column)}</div>
          <div className="font-mono text-[11px] text-muted-foreground">{shortId(column.runId)}</div>
        </div>
        <Badge variant={summary && summary.worse > 0 ? "destructive" : "secondary"}>
          {summary ? `${summary.better}/${summary.worse}` : "-"}
        </Badge>
      </div>
      <div className="mt-2 grid grid-cols-5 gap-1 text-center text-[11px]">
        <SummaryCount label="Better" value={summary?.better ?? 0} tone="text-success" />
        <SummaryCount label="Worse" value={summary?.worse ?? 0} tone="text-destructive" />
        <SummaryCount label="Same" value={summary?.same ?? 0} tone="text-muted-foreground" />
        <SummaryCount label="Missing" value={summary?.missing ?? 0} tone="text-warning" />
        <SummaryCount label="N/A" value={summary?.notComparable ?? 0} tone="text-muted-foreground" />
      </div>
    </div>
  );
}

function SummaryCount({ label, value, tone }: { label: string; value: number; tone: string }) {
  return (
    <div className="min-w-0">
      <div className={cn("font-mono text-sm font-semibold tabular-nums", tone)}>{formatInteger(value)}</div>
      <div className="truncate text-muted-foreground">{label}</div>
    </div>
  );
}

function ColumnHead({ column, baseline }: { column: CompareColumnVM; baseline?: boolean }) {
  return (
    <div className="flex flex-col items-end gap-1">
      <div className="max-w-[180px] truncate text-foreground" title={column.name || column.runId}>
        {runLabel(column)}
      </div>
      <div className="flex items-center gap-1 font-mono text-[11px] text-muted-foreground">
        <span>{shortId(column.runId)}</span>
        {baseline && <Badge variant="outline">baseline</Badge>}
      </div>
    </div>
  );
}

function ConfigRow({ label, cells }: { label: string; cells: string[] }) {
  return (
    <tr className="border-b border-border/70 last:border-b-0 hover:bg-muted/20">
      <Td className="text-xs font-medium text-muted-foreground">{label}</Td>
      {cells.map((cell, index) => (
        <Td key={index} className="max-w-[260px] truncate text-right" title={cell}>
          {cell}
        </Td>
      ))}
    </tr>
  );
}

interface MetricGroup {
  key: string;
  label: string;
  rows: CompareMetricVM[];
}

function MetricGroupRows({ group, columns }: { group: MetricGroup; columns: CompareColumnVM[] }) {
  return (
    <>
      <tr className="border-b border-border bg-muted/30">
        <td colSpan={columns.length + 1} className="px-3 py-1.5 text-[11px] font-medium uppercase text-muted-foreground">
          {group.label} <span className="font-mono">({group.rows.length})</span>
        </td>
      </tr>
      {group.rows.map((metric) => (
        <tr key={metric.key} className="border-b border-border/70 last:border-b-0 hover:bg-muted/20">
          <Td className="sticky left-0 z-10 w-[340px] min-w-[340px] border-r border-border bg-background">
            <MetricIdentity metric={metric} />
          </Td>
          {columns.map((column, index) => {
            const cell = metric.cells[index];
            return (
              <Td key={column.runId} className={cn("min-w-[220px] text-right tabular-nums", cellTone(cell, index))}>
                <MetricCellView cell={cell} metric={metric} baseline={index === 0} />
              </Td>
            );
          })}
        </tr>
      ))}
    </>
  );
}

function MetricIdentity({ metric }: { metric: CompareMetricVM }) {
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

function MetricCellView({
  cell,
  metric,
  baseline,
}: {
  cell?: CompareCellVM;
  metric: CompareMetricVM;
  baseline: boolean;
}) {
  if (!cell || !cell.present) {
    return (
      <div className="flex flex-col items-end gap-1 text-muted-foreground">
        <Badge variant="outline">missing</Badge>
        <span className="text-[11px]">No sample</span>
      </div>
    );
  }

  if (baseline) {
    return (
      <div className="flex flex-col items-end gap-1">
        <span className="font-mono text-sm text-foreground">{formatMetricValue(cell.avg, metric.unit)}</span>
        <span className="font-mono text-[11px] text-muted-foreground">max {formatMetricValue(cell.max, metric.unit)}</span>
        <Badge variant="outline">baseline</Badge>
      </div>
    );
  }

  if (cell.verdict === "VERDICT_UNSPECIFIED") {
    return (
      <div className="flex flex-col items-end gap-1">
        <span className="font-mono text-sm text-foreground">{formatMetricValue(cell.avg, metric.unit)}</span>
        <span className="font-mono text-[11px] text-muted-foreground">max {formatMetricValue(cell.max, metric.unit)}</span>
        <Badge variant="warning">not comparable</Badge>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <span className="font-mono text-sm text-foreground">{formatMetricValue(cell.avg, metric.unit)}</span>
      <span className="font-mono text-[11px] text-muted-foreground">max {formatMetricValue(cell.max, metric.unit)}</span>
      <Badge variant={verdictVariant(cell.verdict)} className="gap-1">
        {verdictLabel(cell.verdict)}
        <span className="font-mono">{cell.diffAvgPctDefined ? fmtPct(cell.diffAvgPct) : "delta n/a"}</span>
      </Badge>
    </div>
  );
}

function SectionTitle({ title, meta, flush = false }: { title: string; meta?: string; flush?: boolean }) {
  return (
    <div className={cn("flex items-center gap-2", !flush && "border-b border-border bg-muted/20 px-3 py-2")}>
      <h2 className="text-sm font-semibold">{title}</h2>
      {meta && <span className="font-mono text-[11px] text-muted-foreground">{meta}</span>}
    </div>
  );
}

function EmptyState({ children }: { children: ReactNode }) {
  return <div className="px-4 py-10 text-center text-sm text-muted-foreground">{children}</div>;
}

function Th({ children, className, ...props }: ThHTMLAttributes<HTMLTableCellElement>) {
  return <th className={cn("px-3 py-2 text-left font-medium", className)} {...props}>{children}</th>;
}

function Td({ children, className, ...props }: TdHTMLAttributes<HTMLTableCellElement>) {
  return <td className={cn("px-3 py-2 align-middle", className)} {...props}>{children}</td>;
}

function parseRunIds(raw: string): string[] {
  return raw
    .split(/[\s,;]+/)
    .map((value) => value.trim())
    .filter(Boolean);
}

function groupMetrics(metrics: CompareMetricVM[]): MetricGroup[] {
  const byGroup = new Map<string, CompareMetricVM[]>();
  for (const metric of metrics) {
    const key = normalizeGroup(metric.group);
    const rows = byGroup.get(key) ?? [];
    rows.push(metric);
    byGroup.set(key, rows);
  }
  return Array.from(byGroup.entries())
    .sort(([left], [right]) => groupRank(left) - groupRank(right) || left.localeCompare(right))
    .map(([key, rows]) => ({ key, label: groupLabel(key), rows }));
}

function normalizeGroup(group: string): string {
  return normalizeSearch(group) || "other";
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

function cellTone(cell: CompareCellVM | undefined, index: number): string {
  if (index === 0) return "bg-muted/10";
  if (!cell?.present) return "bg-muted/20";
  if (cell.verdict === "VERDICT_BETTER") return "bg-success/5";
  if (cell.verdict === "VERDICT_WORSE") return "bg-destructive/5";
  if (cell.verdict === "VERDICT_UNSPECIFIED") return "bg-warning/5";
  return "";
}

function verdictVariant(v: string): "success" | "destructive" | "secondary" {
  if (v === "VERDICT_BETTER") return "success";
  if (v === "VERDICT_WORSE") return "destructive";
  return "secondary";
}

function verdictLabel(v: string): string {
  if (v === "VERDICT_BETTER") return "better";
  if (v === "VERDICT_WORSE") return "worse";
  if (v === "VERDICT_SAME") return "same";
  return "n/a";
}

function runLabel(column?: CompareColumnVM): string {
  if (!column) return "-";
  return column.name || shortId(column.runId);
}

function shortId(id?: string): string {
  if (!id) return "-";
  return id.length <= 12 ? id : id.slice(0, 8);
}

function normalizeSearch(value: string): string {
  return value.trim().toLowerCase();
}

function includesQuery(values: Array<string | undefined>, query: string): boolean {
  return values.some((value) => (value ?? "").toLowerCase().includes(query));
}

function fmtDur(sec?: number): string {
  if (sec === undefined || !Number.isFinite(sec)) return "-";
  if (sec < 60) return `${sec.toFixed(0)}s`;
  return `${Math.floor(sec / 60)}m ${Math.floor(sec % 60)}s`;
}

function formatInteger(value: number): string {
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(value);
}

function formatMetricValue(value: number, unit: string): string {
  if (!Number.isFinite(value)) return "-";
  if (unit === "bytes") return formatBytes(value);
  if (unit === "bytes/s" || unit === "B/s") return `${formatBytes(value)}/s`;
  if (unit === "%") return `${value.toFixed(2)}%`;
  if (unit === "s" || unit === "ms") return `${trimFixed(value, 2)} ${unit}`;
  if (unit === "ops/s" || unit === "txn/s" || unit === "q/s" || unit === "req/s" || unit === "iter/s") {
    return `${trimFixed(value, 1)} ${unit}`;
  }
  const formatted = value >= 1e6 || value <= -1e6
    ? `${trimFixed(value / 1e6, 2)}M`
    : value >= 1e3 || value <= -1e3
      ? `${trimFixed(value / 1e3, 2)}K`
      : trimFixed(value, 2);
  return unit ? `${formatted} ${unit}` : formatted;
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

function fmtPct(value: number): string {
  const sign = value > 0 ? "+" : "";
  return `${sign}${value.toFixed(1)}%`;
}
