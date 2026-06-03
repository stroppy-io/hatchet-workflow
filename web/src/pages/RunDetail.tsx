import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";
import { Activity, ArrowLeft, GitCompare, Radio, RefreshCw, Share2, Terminal } from "lucide-react";

import { Link, useParams, useTenantSlug } from "@/lib/router";
import { useBreadcrumbLabel } from "@/lib/breadcrumbs";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import {
  getRunOverview,
  getRunMetrics,
  queryLogs,
  streamLogs,
  type OverviewVM,
  type PipelineNodeVM,
  type MetricVM,
  type LogLineVM,
} from "@/services/run_overview";
import type { RunStatus } from "@/services/dashboard";

const STATUS_VARIANT: Record<RunStatus, "default" | "success" | "destructive" | "warning" | "pending"> = {
  pending: "pending",
  running: "default",
  cancelling: "warning",
  completed: "success",
  failed: "destructive",
  cancelled: "warning",
};

export function RunDetail() {
  const tenantSlug = useTenantSlug() ?? "";
  const { id = "" } = useParams<{ id: string }>();

  const [overview, setOverview] = useState<OverviewVM | null>(null);
  const [metrics, setMetrics] = useState<MetricVM[]>([]);
  const [logs, setLogs] = useState<LogLineVM[]>([]);
  const [logSearch, setLogSearch] = useState("");
  const [loading, setLoading] = useState(false);
  const [logLoading, setLogLoading] = useState(false);
  const [liveTail, setLiveTail] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // The active stream controller; aborted on toggle-off / filter change / unmount.
  const tailAbort = useRef<AbortController | null>(null);

  useBreadcrumbLabel(
    "id",
    overview?.run?.name || (id ? `Run ${id.slice(0, 8)}` : undefined),
  );

  const load = useCallback(async () => {
    if (!tenantSlug || !id) return;
    setLoading(true);
    setError(null);
    try {
      const [ov, ms] = await Promise.all([
        getRunOverview(tenantSlug, id),
        getRunMetrics(tenantSlug, id).catch(() => [] as MetricVM[]),
      ]);
      setOverview(ov);
      setMetrics(ms);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [tenantSlug, id]);

  const loadLogs = useCallback(async () => {
    if (!tenantSlug || !id) return;
    setLogLoading(true);
    try {
      const page = await queryLogs(tenantSlug, id, {
        search: logSearch.trim() || undefined,
        direction: "newer",
        limit: 200,
      });
      setLogs(page.lines);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLogLoading(false);
    }
  }, [tenantSlug, id, logSearch]);

  useEffect(() => {
    void load();
  }, [load]);
  useEffect(() => {
    void loadLogs();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tenantSlug, id]);

  // Live tail: when enabled, open StreamLogs and append each incoming line to
  // the same log view. Re-runs (and re-subscribes) when the run, tenant, or the
  // committed search filter changes. The cleanup aborts the controller, which
  // also covers React StrictMode's double-invoke (the first run's controller is
  // aborted before the second opens) and unmount.
  useEffect(() => {
    if (!liveTail || !tenantSlug || !id) return;
    const controller = new AbortController();
    tailAbort.current = controller;
    void streamLogs(
      tenantSlug,
      id,
      { search: logSearch.trim() || undefined },
      controller.signal,
      (line) => {
        if (controller.signal.aborted) return;
        setLogs((prev) => [...prev, line]);
      },
    ).catch((err) => {
      if (controller.signal.aborted) return;
      setError(err instanceof Error ? err.message : String(err));
    });
    return () => {
      controller.abort();
      if (tailAbort.current === controller) tailAbort.current = null;
    };
  }, [liveTail, tenantSlug, id, logSearch]);

  const flatNodes = useMemo(
    () => (overview ? flatten(overview.pipeline) : []),
    [overview],
  );

  const run = overview?.run;

  return (
    <div className="min-h-full bg-background text-foreground">
      <div className="mx-auto flex max-w-[1600px] flex-col gap-4 p-4 md:p-6">
        <div className="flex flex-col gap-3 border-b border-border pb-4 md:flex-row md:items-center md:justify-between">
          <div className="min-w-0">
            <Link to="/runs" className="mb-1 inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground">
              <ArrowLeft className="h-3 w-3" /> Runs
            </Link>
            <div className="flex items-center gap-2 text-lg font-semibold">
              <Activity className="h-5 w-5 text-primary" />
              <h1 className="truncate">{run?.name || `Run ${id.slice(0, 8)}`}</h1>
              {overview && (
                <Badge variant={STATUS_VARIANT[overview.status]}>{overview.status}</Badge>
              )}
            </div>
            <div className="mt-1 font-mono text-[11px] text-muted-foreground">{id}</div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Link to={`/shares?targetId=${id}`}>
              <Button variant="outline" size="sm"><Share2 className="h-4 w-4" /> Share</Button>
            </Link>
            <Link to={`/compare?runIds=${id}`}>
              <Button variant="outline" size="sm"><GitCompare className="h-4 w-4" /> Compare</Button>
            </Link>
            <Link to={`/shell?runId=${id}`}>
              <Button variant="outline" size="sm"><Terminal className="h-4 w-4" /> Shell</Button>
            </Link>
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

        {/* header summary */}
        <div className="grid grid-cols-2 gap-2 md:grid-cols-4 lg:grid-cols-6">
          <Stat label="Progress" value={`${overview?.progressPct ?? 0}%`} />
          <Stat label="Duration" value={fmtDur(overview?.durationSec)} />
          <Stat label="DB" value={run?.dbKind || "—"} />
          <Stat label="Workload" value={run?.workload || "—"} />
          <Stat label="Stroppy" value={run?.stroppyVersion || "—"} />
          <Stat label="Provider" value={run?.provider || "—"} />
        </div>

        {/* pipeline phases */}
        <Section title="Phases" count={flatNodes.length}>
          <div className="overflow-hidden border border-border">
            <table className="w-full border-collapse text-sm">
              <thead className="bg-muted/60 text-xs uppercase text-muted-foreground">
                <tr className="border-b border-border">
                  <Th>Stage</Th>
                  <Th>Status</Th>
                  <Th>Attempt</Th>
                  <Th>Started</Th>
                  <Th>Duration</Th>
                </tr>
              </thead>
              <tbody>
                {flatNodes.map((n) => (
                  <tr key={n.nodeExecutionId || n.name} className="border-b border-border/70 hover:bg-muted/30">
                    <Td style={{ paddingLeft: 12 + n.depth * 16 }}>
                      <span className="font-mono text-xs">{n.name || "—"}</span>
                    </Td>
                    <Td><Badge variant={STATUS_VARIANT[n.status]}>{n.status}</Badge></Td>
                    <Td className="tabular-nums">{n.attempt > 1 ? `×${n.attempt}` : "—"}</Td>
                    <Td>{fmtTime(n.startedAt)}</Td>
                    <Td className="tabular-nums">{fmtDur(n.durationSec)}</Td>
                  </tr>
                ))}
                {flatNodes.length === 0 && <Empty cols={5} text={loading ? "Loading..." : "No pipeline stages."} />}
              </tbody>
            </table>
          </div>
        </Section>

        {/* workers + timeline side by side */}
        <div className="grid gap-4 lg:grid-cols-2">
          <Section title="Workers" count={overview?.workers.length ?? 0}>
            <div className="overflow-hidden border border-border">
              <table className="w-full border-collapse text-sm">
                <thead className="bg-muted/60 text-xs uppercase text-muted-foreground">
                  <tr className="border-b border-border">
                    <Th>Worker</Th><Th>Kind</Th><Th>Host</Th><Th>Online</Th>
                  </tr>
                </thead>
                <tbody>
                  {(overview?.workers ?? []).map((w) => (
                    <tr key={w.id} className="border-b border-border/70 hover:bg-muted/30">
                      <Td><span className="font-mono text-xs">{w.id || "—"}</span></Td>
                      <Td>{w.kind || "—"}</Td>
                      <Td className="font-mono text-xs">{w.host || w.machineId || "—"}</Td>
                      <Td>{w.online ? <Badge variant="success">up</Badge> : <Badge variant="warning">down</Badge>}</Td>
                    </tr>
                  ))}
                  {(overview?.workers.length ?? 0) === 0 && <Empty cols={4} text="No workers." />}
                </tbody>
              </table>
            </div>
          </Section>

          <Section title="Timeline" count={overview?.timeline.length ?? 0}>
            <div className="max-h-[320px] overflow-y-auto border border-border">
              <ul className="divide-y divide-border/70 text-sm">
                {(overview?.timeline ?? []).map((e, i) => (
                  <li key={i} className="flex items-start gap-2 px-3 py-2">
                    <span className="shrink-0 font-mono text-[11px] text-muted-foreground">{fmtTime(e.at)}</span>
                    <span className="shrink-0 text-[11px] uppercase text-primary">{e.kind}</span>
                    <span className="min-w-0 break-words">{e.message}</span>
                  </li>
                ))}
                {(overview?.timeline.length ?? 0) === 0 && (
                  <li className="px-3 py-6 text-center text-muted-foreground">No timeline events.</li>
                )}
              </ul>
            </div>
          </Section>
        </div>

        {/* metrics */}
        <Section title="Metrics" count={metrics.length}>
          <div className="overflow-hidden border border-border">
            <table className="w-full border-collapse text-sm">
              <thead className="bg-muted/60 text-xs uppercase text-muted-foreground">
                <tr className="border-b border-border">
                  <Th>Metric</Th><Th className="text-right">Avg</Th><Th className="text-right">Min</Th>
                  <Th className="text-right">Max</Th><Th className="text-right">Last</Th><Th>Group</Th>
                </tr>
              </thead>
              <tbody>
                {metrics.map((m) => (
                  <tr key={m.key} className="border-b border-border/70 hover:bg-muted/30">
                    <Td>
                      <div className="font-mono text-xs">{m.name}</div>
                      <div className="text-[11px] text-muted-foreground">{m.unit}</div>
                    </Td>
                    <Td className="text-right tabular-nums">{fmtNum(m.avg)}</Td>
                    <Td className="text-right tabular-nums">{fmtNum(m.min)}</Td>
                    <Td className="text-right tabular-nums">{fmtNum(m.max)}</Td>
                    <Td className="text-right tabular-nums">{fmtNum(m.last)}</Td>
                    <Td className="text-[11px] text-muted-foreground">{m.group || "—"}</Td>
                  </tr>
                ))}
                {metrics.length === 0 && <Empty cols={6} text="No metrics yet." />}
              </tbody>
            </table>
          </div>
        </Section>

        {/* logs */}
        <Section title="Logs" count={logs.length}>
          <div className="mb-2 flex items-center gap-2">
            <Input
              value={logSearch}
              onChange={(e) => setLogSearch(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && void loadLogs()}
              placeholder="Search log lines..."
              className="h-9 max-w-md"
            />
            <Button variant="outline" size="sm" onClick={() => void loadLogs()} disabled={logLoading || liveTail}>
              <RefreshCw className={cn("h-4 w-4", logLoading && "animate-spin")} /> Query
            </Button>
            <Button
              variant={liveTail ? "default" : "outline"}
              size="sm"
              onClick={() => setLiveTail((v) => !v)}
            >
              <Radio className={cn("h-4 w-4", liveTail && "animate-pulse")} />{" "}
              {liveTail ? "Live tail on" : "Live tail"}
            </Button>
          </div>
          <pre className="max-h-[420px] overflow-auto border border-border bg-black/40 p-3 font-mono text-[11px] leading-relaxed">
            {logs.length === 0
              ? logLoading
                ? "Loading logs..."
                : "No log lines."
              : logs.map((l, i) => (
                  <div key={`${l.lineNo}-${i}`} className="whitespace-pre-wrap">
                    <span className="text-muted-foreground">{l.lineNo.padStart(6)} </span>
                    <span className={cn(l.stream === "STREAM_STDERR" && "text-destructive")}>{l.line}</span>
                  </div>
                ))}
          </pre>
        </Section>
      </div>
    </div>
  );
}

type FlatNode = PipelineNodeVM & { depth: number };
function flatten(nodes: PipelineNodeVM[], depth = 0, out: FlatNode[] = []): FlatNode[] {
  for (const n of nodes) {
    out.push({ ...n, depth });
    if (n.children.length) flatten(n.children, depth + 1, out);
  }
  return out;
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="border border-border bg-muted/20 px-3 py-2">
      <div className="text-[11px] uppercase text-muted-foreground">{label}</div>
      <div className="mt-1 truncate text-base font-semibold tabular-nums">{value}</div>
    </div>
  );
}

function Section({ title, count, children }: { title: string; count: number; children: ReactNode }) {
  return (
    <div>
      <h2 className="mb-2 text-sm font-semibold">
        {title} <span className="text-xs text-muted-foreground">({count})</span>
      </h2>
      {children}
    </div>
  );
}

function Th({ children, className }: { children: ReactNode; className?: string }) {
  return <th className={cn("px-3 py-2 text-left font-medium", className)}>{children}</th>;
}
function Td({ children, className, style }: { children: ReactNode; className?: string; style?: React.CSSProperties }) {
  return <td className={cn("px-3 py-2 align-middle", className)} style={style}>{children}</td>;
}
function Empty({ cols, text }: { cols: number; text: string }) {
  return (
    <tr>
      <td colSpan={cols} className="px-4 py-8 text-center text-sm text-muted-foreground">{text}</td>
    </tr>
  );
}

function fmtDur(sec?: number): string {
  if (sec === undefined || !Number.isFinite(sec)) return "—";
  if (sec < 60) return `${sec.toFixed(0)}s`;
  const m = Math.floor(sec / 60);
  const s = Math.floor(sec % 60);
  return `${m}m ${s}s`;
}
function fmtTime(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return new Intl.DateTimeFormat(undefined, { timeStyle: "medium" }).format(d);
}
function fmtNum(v: number): string {
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(v);
}
