import { useCallback, useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import {
  Activity,
  ArrowLeft,
  BarChart3,
  GitCompare,
  LineChart,
  Network,
  RefreshCw,
  Repeat,
  Save,
  ScrollText,
  Share2,
  Square,
  Terminal,
  Trash2,
  Workflow,
} from "lucide-react";

import { Link, useNavigate, useParams, useSearchParams, useTenantSlug } from "@/lib/router";
import { useBreadcrumbLabel } from "@/lib/breadcrumbs";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { SegmentedControl } from "@/components/ui/segmented";
import { useConfirm } from "@/components/ui/confirm-dialog";
import { cn } from "@/lib/utils";
import { RunInfoSidebar } from "@/components/run/RunInfoSidebar";
import { PipelineViewer } from "@/components/run/PipelineViewer";
import { TopologyViewer } from "@/components/run/TopologyViewer";
import { LogsPanel } from "@/components/run/LogsPanel";
import { MetricsPanel } from "@/components/run/MetricsPanel";
import { GrafanaPanel } from "@/components/run/GrafanaPanel";
import {
  getRunOverview,
  getRunMetrics,
  streamRunOverview,
  type OverviewVM,
  type MetricVM,
  type WorkerPresence,
  type EventSeverity,
} from "@/services/run_overview";
import { actionsForStatus, getRunsProvider } from "@/services/runs";
import type { RunStatus } from "@/services/dashboard";

const STATUS_VARIANT: Record<RunStatus, "default" | "success" | "destructive" | "warning" | "pending"> = {
  pending: "pending",
  running: "default",
  cancelling: "warning",
  completed: "success",
  failed: "destructive",
  cancelled: "warning",
};

const STATUS_DOT: Record<RunStatus, string> = {
  pending: "bg-pending",
  running: "bg-primary animate-pulse",
  cancelling: "bg-warning animate-pulse",
  completed: "bg-success",
  failed: "bg-destructive",
  cancelled: "bg-pending",
};

const SEVERITY_DOT: Record<EventSeverity, string> = {
  info: "bg-primary",
  warning: "bg-warning",
  error: "bg-destructive",
  "": "bg-muted-foreground",
};
const SEVERITY_TEXT: Record<EventSeverity, string> = {
  info: "text-primary",
  warning: "text-warning",
  error: "text-destructive",
  "": "text-muted-foreground",
};

const PRESENCE: Record<WorkerPresence, { label: string; variant: "success" | "warning" | "destructive" | "pending" }> = {
  online: { label: "online", variant: "success" },
  stale: { label: "stale", variant: "warning" },
  offline: { label: "offline", variant: "destructive" },
  terminated: { label: "terminated", variant: "pending" },
  unknown: { label: "unknown", variant: "pending" },
  "": { label: "unknown", variant: "pending" },
};

const SOURCE_LABEL: Record<string, string> = {
  temporal: "live",
  persisted: "persisted",
  plan: "plan",
  registry: "registry",
  synthetic: "synthetic",
};

function PresenceBadge({ presence, online }: { presence: WorkerPresence; online: boolean }) {
  const p = PRESENCE[presence] ?? (online ? PRESENCE.online : PRESENCE.unknown);
  return <Badge variant={p.variant}>{p.label}</Badge>;
}

function relTime(iso?: string): string {
  if (!iso) return "—";
  const t = new Date(iso).getTime();
  if (Number.isNaN(t) || new Date(iso).getFullYear() < 2000) return "—";
  const s = Math.max(0, Math.round((Date.now() - t) / 1000));
  if (s < 60) return `${s}s ago`;
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  return `${Math.floor(s / 86400)}d ago`;
}

type View = "pipeline" | "topology" | "logs" | "metrics" | "grafana" | "agents";

const VIEW_OPTIONS = [
  { value: "pipeline" as const, label: "Pipeline", icon: <Workflow className="h-3 w-3" /> },
  { value: "topology" as const, label: "Topology", icon: <Network className="h-3 w-3" /> },
  { value: "logs" as const, label: "Logs", icon: <ScrollText className="h-3 w-3" /> },
  { value: "metrics" as const, label: "Metrics", icon: <LineChart className="h-3 w-3" /> },
  { value: "grafana" as const, label: "Grafana", icon: <BarChart3 className="h-3 w-3" /> },
  { value: "agents" as const, label: "Agents", icon: <Activity className="h-3 w-3" /> },
];

export function RunDetail() {
  const tenantSlug = useTenantSlug() ?? "";
  const navigate = useNavigate();
  const confirm = useConfirm();
  const { id = "" } = useParams<{ id: string }>();

  const [overview, setOverview] = useState<OverviewVM | null>(null);
  const [metrics, setMetrics] = useState<MetricVM[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  // Active view + log filters live in the URL query so any view is a shareable
  // link (e.g. ?view=logs&steps=<id>&q=error). The pipeline jump and back
  // button all flow through it.
  const [sp, setSp] = useSearchParams();
  const view = (sp.get("view") as View | null) ?? "pipeline";
  const setView = useCallback(
    (v: string) =>
      setSp(
        (prev) => {
          const next = new URLSearchParams(prev);
          next.set("view", v);
          // A copied line anchor is transient — drop it when switching views.
          next.delete("line");
          return next;
        },
        { replace: true },
      ),
    [setSp],
  );

  // Jump into Logs pre-filtered to a pipeline node (replaces other log filters).
  const openLogsForNode = useCallback(
    (nodeExecutionId: string) =>
      setSp(
        (prev) => {
          const next = new URLSearchParams(prev);
          next.set("view", "logs");
          next.set("steps", nodeExecutionId);
          next.delete("comp");
          next.delete("mach");
          next.delete("unit");
          next.delete("q");
          next.delete("line");
          return next;
        },
        { replace: false },
      ),
    [setSp],
  );

  useBreadcrumbLabel("id", overview?.run?.name || (id ? `Run ${id.slice(0, 8)}` : undefined));

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

  useEffect(() => {
    void load();
  }, [load]);

  const run = overview?.run;
  const status = overview?.status;
  const allowed = status ? actionsForStatus(status) : new Set<string>();
  const terminal = status === "completed" || status === "failed" || status === "cancelled";

  // Live updates while the run is in flight: subscribe to the overview stream
  // (full snapshot per frame); fall back to polling if streaming is
  // unavailable. Stops once the run reaches a terminal state.
  useEffect(() => {
    if (!tenantSlug || !id || !status || terminal) return;
    const ctrl = new AbortController();
    let pollIv: number | undefined;
    const startPoll = () => {
      if (pollIv !== undefined) return;
      pollIv = window.setInterval(() => {
        getRunOverview(tenantSlug, id).then(setOverview).catch(() => {});
      }, 5000);
    };
    void streamRunOverview(tenantSlug, id, setOverview, ctrl.signal).catch(() => {
      if (!ctrl.signal.aborted) startPoll();
    });
    return () => {
      ctrl.abort();
      if (pollIv !== undefined) clearInterval(pollIv);
    };
  }, [tenantSlug, id, status, terminal]);

  // Refresh metrics periodically while the run is in flight (the stream carries
  // overview only).
  useEffect(() => {
    if (!tenantSlug || !id || terminal) return;
    const iv = window.setInterval(() => {
      getRunMetrics(tenantSlug, id).then(setMetrics).catch(() => {});
    }, 12000);
    return () => clearInterval(iv);
  }, [tenantSlug, id, terminal]);

  const flash = useCallback((msg: string) => {
    setNotice(msg);
    setError(null);
    window.setTimeout(() => setNotice(null), 2500);
  }, []);

  // Action dispatch — each maps to a real TestRunService RPC via the runs
  // provider (the same surface the runs table uses). Gated by actionsForStatus.
  const onShare = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(window.location.href);
      flash("Link copied");
    } catch {
      navigate(`/shares?targetId=${id}`);
    }
  }, [id, flash, navigate]);

  const onRerun = useCallback(async () => {
    setBusy(true);
    try {
      await getRunsProvider().rerunRun(tenantSlug, id);
      flash("Run re-launched");
      navigate("/runs");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to rerun");
    } finally {
      setBusy(false);
    }
  }, [tenantSlug, id, flash, navigate]);

  const onSavePreset = useCallback(async () => {
    setBusy(true);
    try {
      await getRunsProvider().extractToPreset(tenantSlug, id);
      flash("Saved as preset");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save preset");
    } finally {
      setBusy(false);
    }
  }, [tenantSlug, id, flash]);

  const onCancel = useCallback(async () => {
    const ok = await confirm({
      title: "Cancel run?",
      description: "The run will be stopped. Already-provisioned resources are torn down during teardown.",
      danger: true,
      confirmLabel: "Cancel run",
    });
    if (!ok) return;
    setBusy(true);
    try {
      await getRunsProvider().cancelRun(tenantSlug, id);
      flash("Cancel requested");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to cancel");
    } finally {
      setBusy(false);
    }
  }, [tenantSlug, id, confirm, flash, load]);

  const onDelete = useCallback(async () => {
    const ok = await confirm({
      title: "Delete run?",
      description: `“${run?.name || id}” will be permanently removed. This cannot be undone.`,
      danger: true,
      confirmLabel: "Delete",
    });
    if (!ok) return;
    setBusy(true);
    try {
      await getRunsProvider().deleteRun(tenantSlug, id);
      navigate("/runs");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete");
      setBusy(false);
    }
  }, [tenantSlug, id, run?.name, confirm, navigate]);

  // Whether the active view manages its own height (fills) vs scrolls.
  const scrolls = view === "metrics" || view === "agents";

  // Resizable RUN INFO sidebar — width persisted so the choice sticks.
  const SIDEBAR_KEY = "stroppy.runDetail.sidebarWidth";
  const [sidebarW, setSidebarW] = useState<number>(() => {
    const s = parseInt(localStorage.getItem(SIDEBAR_KEY) ?? "", 10);
    return Number.isFinite(s) && s >= 220 && s <= 560 ? s : 300;
  });
  useEffect(() => {
    localStorage.setItem(SIDEBAR_KEY, String(sidebarW));
  }, [sidebarW]);
  const dragRef = useRef<{ x: number; w: number } | null>(null);
  const onDividerDown = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      dragRef.current = { x: e.clientX, w: sidebarW };
      const move = (ev: MouseEvent) => {
        const d = dragRef.current;
        if (!d) return;
        setSidebarW(Math.min(560, Math.max(220, d.w + ev.clientX - d.x)));
      };
      const up = () => {
        dragRef.current = null;
        window.removeEventListener("mousemove", move);
        window.removeEventListener("mouseup", up);
      };
      window.addEventListener("mousemove", move);
      window.addEventListener("mouseup", up);
    },
    [sidebarW],
  );

  return (
    <div className="flex h-full flex-col bg-background p-2 text-foreground">
      {/* Top bar: name + status (left), run actions (right) */}
      <div className="flex shrink-0 flex-col gap-3 border-b border-border pb-2 lg:flex-row lg:items-center lg:justify-between">
        <div className="min-w-0">
          <Link to="/runs" className="mb-1 inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground">
            <ArrowLeft className="h-3 w-3" /> Runs
          </Link>
          <div className="flex items-center gap-2.5">
            {status && <span className={cn("h-2 w-2 shrink-0 rounded-full", STATUS_DOT[status])} />}
            <h1 className="truncate text-lg font-semibold">{run?.name || `Run ${id.slice(0, 8)}`}</h1>
            {status && <Badge variant={STATUS_VARIANT[status]}>{status}</Badge>}
          </div>
          <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-0.5 font-mono text-[11px] text-muted-foreground">
            <span>{id}</span>
            {overview?.source && overview.source !== "temporal" && (
              <span className="rounded-sm border border-border px-1 text-[10px] uppercase tracking-wide" title="data source">
                {SOURCE_LABEL[overview.source] ?? overview.source}
              </span>
            )}
            {overview?.observedAt && <span title={overview.observedAt}>· as of {relTime(overview.observedAt)}</span>}
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => void load()} disabled={loading}>
            <RefreshCw className={cn("h-4 w-4", loading && "animate-spin")} /> Refresh
          </Button>
          {allowed.has("rerun") && (
            <Button variant="outline" size="sm" onClick={() => void onRerun()} disabled={busy}>
              <Repeat className="h-4 w-4" /> Rerun
            </Button>
          )}
          {allowed.has("extract") && (
            <Button variant="outline" size="sm" onClick={() => void onSavePreset()} disabled={busy}>
              <Save className="h-4 w-4" /> Save preset
            </Button>
          )}
          <Button variant="outline" size="sm" onClick={() => void onShare()}>
            <Share2 className="h-4 w-4" /> Share
          </Button>
          <Link to={`/compare?runIds=${id}`}>
            <Button variant="outline" size="sm">
              <GitCompare className="h-4 w-4" /> Compare
            </Button>
          </Link>
          <Link to={`/shell?runId=${id}`}>
            <Button variant="outline" size="sm">
              <Terminal className="h-4 w-4" /> Shell
            </Button>
          </Link>
          {allowed.has("cancel") && (
            <Button variant="outline" size="sm" onClick={() => void onCancel()} disabled={busy}>
              <Square className="h-4 w-4 text-warning" /> Cancel
            </Button>
          )}
          {allowed.has("delete") && (
            <Button variant="outline" size="sm" onClick={() => void onDelete()} disabled={busy}>
              <Trash2 className="h-4 w-4 text-destructive" /> Delete
            </Button>
          )}
        </div>
      </div>

      {error && (
        <div className="shrink-0 border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</div>
      )}
      {notice && (
        <div className="shrink-0 border border-success/40 bg-success/10 px-3 py-2 text-sm text-success">{notice}</div>
      )}
      {overview && overview.degradedReasons.length > 0 && (
        <div className="shrink-0 border border-warning/40 bg-warning/10 px-3 py-1.5 font-mono text-[11px] text-warning">
          Degraded snapshot: {overview.degradedReasons.join("; ")}
        </div>
      )}

      {/* Body: resizable RUN INFO sidebar + main panel with a single view
          switcher. The only border is the draggable divider between them. */}
      <div className="flex min-h-0 flex-1">
        <div style={{ width: sidebarW }} className="min-h-0 shrink-0 overflow-auto">
          {overview ? (
            <RunInfoSidebar overview={overview} run={run} />
          ) : (
            <div className="px-3 py-8 text-center font-mono text-xs text-muted-foreground">
              {loading ? "Loading…" : "No data"}
            </div>
          )}
        </div>

        {/* Draggable divider — the single border; resize is persisted. */}
        <div
          onMouseDown={onDividerDown}
          title="Drag to resize"
          className="group/divider relative w-2 shrink-0 cursor-col-resize"
        >
          <span className="absolute inset-y-0 left-1/2 w-px -translate-x-1/2 bg-border transition-colors group-hover/divider:bg-primary/60 group-active/divider:bg-primary" />
        </div>

        <div className="flex h-full min-w-0 min-h-0 flex-1 pl-1 flex-col">
          <div className="flex h-12 shrink-0 items-stretch p-2">
            <SegmentedControl
              variant="tabs"
              value={view}
              onChange={setView}
              options={VIEW_OPTIONS}
              segmentClassName="w-[88px]"
            />
          </div>
          <div className="relative min-h-0 flex-1 overflow-hidden">
            {/* Grafana is mounted as soon as the run loads (hidden until
                selected) so its dashboards pre-load in the background. */}
            {overview && (
              <div className={cn("absolute inset-0 overflow-hidden", view === "grafana" ? "block" : "hidden")}>
                <GrafanaPanel
                  runId={id}
                  dbKind={run?.dbKind}
                  startedAt={overview.startedAt}
                  finishedAt={overview.finishedAt}
                  workers={overview.workers}
                />
              </div>
            )}
            <div className={cn("h-full", scrolls ? "overflow-auto p-3" : "overflow-hidden", view === "grafana" && "hidden")}>
            {view === "pipeline" && <PipelineViewer pipeline={overview?.pipeline ?? []} onOpenLogs={openLogsForNode} />}
            {view === "topology" && <TopologyViewer topology={overview?.topology} />}
            {view === "logs" && <LogsPanel tenantSlug={tenantSlug} runId={id} pipeline={overview?.pipeline ?? []} />}
            {view === "metrics" && <MetricsPanel metrics={metrics} />}
            {view === "agents" && (
              <div className="grid gap-3 lg:grid-cols-2">
                <div className="overflow-hidden border border-border">
                  <table className="w-full border-collapse text-sm">
                    <thead className="bg-muted/60 text-xs uppercase text-muted-foreground">
                      <tr className="border-b border-border">
                        <Th>Worker</Th><Th>Kind</Th><Th>Presence</Th><Th>Last seen</Th><Th>Version</Th>
                      </tr>
                    </thead>
                    <tbody>
                      {(overview?.workers ?? []).map((w) => (
                        <tr key={w.id} className="border-b border-border/70 hover:bg-muted/30">
                          <Td>
                            <div className="font-mono text-xs">{w.id || "—"}</div>
                            <div className="font-mono text-[10px] text-muted-foreground">{w.host || w.machineId || ""}</div>
                          </Td>
                          <Td>{w.kind || "—"}</Td>
                          <Td title={w.statusReason || undefined}>
                            <PresenceBadge presence={w.presence} online={w.online} />
                          </Td>
                          <Td className="font-mono text-[11px] text-muted-foreground" title={w.lastSeenAt}>
                            {relTime(w.lastSeenAt)}
                          </Td>
                          <Td className="font-mono text-[11px] text-muted-foreground">{w.agentVersion || "—"}</Td>
                        </tr>
                      ))}
                      {(overview?.workers.length ?? 0) === 0 && <Empty cols={5} text="No workers." />}
                    </tbody>
                  </table>
                </div>
                <div className="max-h-[60vh] overflow-y-auto border border-border">
                  <ul className="divide-y divide-border/70 text-sm">
                    {[...(overview?.timeline ?? [])]
                      .sort((a, b) => a.sequence - b.sequence)
                      .map((e, i) => (
                        <li key={i} className="flex items-start gap-2 px-3 py-2">
                          <span className={cn("mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full", SEVERITY_DOT[e.severity] ?? "bg-muted-foreground")} />
                          <span className="shrink-0 font-mono text-[11px] text-muted-foreground">{fmtTime(e.at)}</span>
                          <span className={cn("shrink-0 text-[11px] uppercase", SEVERITY_TEXT[e.severity] ?? "text-primary")}>{e.kind}</span>
                          <div className="min-w-0">
                            <div className="break-words">{e.message}</div>
                            {e.detail && <div className="break-words font-mono text-[10px] text-muted-foreground">{e.detail}</div>}
                          </div>
                        </li>
                      ))}
                    {(overview?.timeline.length ?? 0) === 0 && (
                      <li className="px-3 py-6 text-center text-muted-foreground">No events.</li>
                    )}
                  </ul>
                </div>
              </div>
            )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function Th({ children, className }: { children: ReactNode; className?: string }) {
  return <th className={cn("px-3 py-2 text-left font-medium", className)}>{children}</th>;
}
function Td({ children, className, style, title }: { children: ReactNode; className?: string; style?: React.CSSProperties; title?: string }) {
  return <td className={cn("px-3 py-2 align-middle", className)} style={style} title={title}>{children}</td>;
}
function Empty({ cols, text }: { cols: number; text: string }) {
  return (
    <tr>
      <td colSpan={cols} className="px-4 py-8 text-center text-sm text-muted-foreground">{text}</td>
    </tr>
  );
}
function fmtTime(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return new Intl.DateTimeFormat(undefined, { timeStyle: "medium" }).format(d);
}
