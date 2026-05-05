import { useEffect, useState, useCallback, useRef, useMemo } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { getRunStatus, getGrafanaSettings, deleteRun, cancelRun, createShareLink, getRunRenderedConfigs, createRunPreset } from "@/api/client";
import { ALL_DB_KINDS, type Snapshot, type NodeStatus, type GrafanaSettings, type RunConfig } from "@/api/types";
import { RunOverview } from "@/components/RunOverview";
import { LogStream } from "@/components/LogStream";
import { MetricsPanel } from "@/components/MetricsPanel";
import { TopologyFlow } from "@/components/TopologyFlow";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { useConfirm } from "@/components/ui/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { RefreshCw, AlertCircle, Trash2, StopCircle, Share2, Check, RotateCcw, FlaskConical } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";

// DB-specific dashboards — only show the one matching the run's database kind.
const DB_DASHBOARDS = ALL_DB_KINDS;

// Role styling — keeps the machine bar legible by color-coding what each
// machine actually does without forcing the user to read role text.
const ROLE_STYLE: Record<string, { label: string; cls: string }> = {
  database: { label: "DB", cls: "border-emerald-500/40 text-emerald-400 bg-emerald-500/5" },
  monitor: { label: "MON", cls: "border-blue-500/40 text-blue-400 bg-blue-500/5" },
  stroppy: { label: "RUNNER", cls: "border-amber-500/40 text-amber-400 bg-amber-500/5" },
  proxy: { label: "PROXY", cls: "border-purple-500/40 text-purple-400 bg-purple-500/5" },
  pgbouncer: { label: "BOUNCER", cls: "border-purple-500/40 text-purple-400 bg-purple-500/5" },
  etcd: { label: "ETCD", cls: "border-cyan-500/40 text-cyan-400 bg-cyan-500/5" },
  "ydb-storage": { label: "YDB-STG", cls: "border-emerald-500/40 text-emerald-400 bg-emerald-500/5" },
  "ydb-database": { label: "YDB-DB", cls: "border-emerald-500/40 text-emerald-400 bg-emerald-500/5" },
};

function roleStyle(role: string) {
  return ROLE_STYLE[role] || { label: role.toUpperCase(), cls: "border-zinc-700 text-zinc-400 bg-zinc-900/40" };
}

function DashboardSelector({ grafana, runID, dbKind, startedAt, finishedAt, targets }: {
  grafana: GrafanaSettings; runID: string; dbKind?: string; startedAt?: string; finishedAt?: string;
  targets?: { id: string; role: string }[];
}) {
  const dashboards = grafana.dashboards || {};

  const visibleDashboards = Object.keys(dashboards).filter(name => {
    if (name === "compare" || name === "overview") return false;
    if ((DB_DASHBOARDS as string[]).includes(name)) return name === dbKind;
    return true;
  });

  const [selectedDashboard, setSelectedDashboard] = useState(visibleDashboards[0] || "stroppy");
  // Selected machine for system/db dashboards. Empty string = let Grafana
  // auto-pick the first (default behaviour for the stroppy/k6 dashboard
  // which doesn't use the `node` template var).
  const [selectedMachine, setSelectedMachine] = useState<string>("");

  const uid = dashboards[selectedDashboard] || "";

  // Surface only this run's machines in the bar — the dashboard template
  // var query already filters by stroppy_run_id, but explicit pills here
  // map machine_id → role so the user sees what they're switching between.
  const runTargets = (targets || []).filter((t) => t.role !== "");
  // System-style dashboards (everything except the workload metrics one)
  // honor the `node` template var. For the stroppy dashboard the bar is
  // shown in read-only mode — clicking a machine has no effect there.
  const machineSelectActive = selectedDashboard !== "stroppy";

  // Build iframe src — memoize to avoid reloading iframe on every poll.
  // While run is in progress, use "now" as end time (Grafana auto-refreshes internally).
  // Once finished, lock the time range.
  const iframeSrc = useMemo(() => {
    let timeParams = "";
    if (startedAt && startedAt !== "0001-01-01T00:00:00Z") {
      const from = new Date(startedAt).getTime() - 30000;
      if (finishedAt && finishedAt !== "0001-01-01T00:00:00Z") {
        const to = new Date(finishedAt).getTime() + 60000;
        timeParams = `&from=${from}&to=${to}`;
      } else {
        // Run in progress — use relative "now" so Grafana auto-refreshes.
        timeParams = `&from=${from}&to=now&refresh=5s`;
      }
    }
    // var-run_id for system/db dashboards (vmagent external_labels), var-prefix for stroppy dashboard (K6 metric prefix = runID_)
    const prefix = runID.replace(/-/g, "_") + "_";
    const nodeParam = machineSelectActive && selectedMachine ? `&var-node=${encodeURIComponent(selectedMachine)}` : "";
    return `${grafana.url}/d/${uid}?var-run_id=${runID}&var-prefix=${prefix}${nodeParam}${timeParams}&kiosk&theme=dark`;
    // Only recalculate when dashboard, machine, startedAt, or finishedAt changes — NOT on every poll.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [grafana.url, uid, runID, startedAt, finishedAt, selectedMachine, machineSelectActive]);

  return (
    <>
      <div className="flex gap-2 p-3 border-b border-border">
        {visibleDashboards.map(name => (
          <button
            type="button"
            key={name}
            onClick={() => setSelectedDashboard(name)}
            className={`px-3 py-1 text-xs font-mono border ${
              selectedDashboard === name
                ? "border-primary text-primary bg-primary/5"
                : "border-zinc-800 text-zinc-500 hover:text-zinc-300"
            }`}
          >
            {name}
          </button>
        ))}
      </div>
      {runTargets.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5 px-3 py-2 border-b border-border bg-[#070707]">
          <span className="text-[10px] font-mono uppercase tracking-wider text-zinc-600 mr-1">
            Machines
          </span>
          {machineSelectActive && (
            <button
              type="button"
              onClick={() => setSelectedMachine("")}
              className={`text-[10px] font-mono px-1.5 py-0.5 border transition-colors ${
                selectedMachine === ""
                  ? "border-primary text-primary bg-primary/10"
                  : "border-zinc-800 text-zinc-500 hover:text-zinc-300"
              }`}
              title="Let Grafana auto-pick the first machine"
            >
              auto
            </button>
          )}
          {runTargets.map((t) => {
            const rs = roleStyle(t.role);
            const active = machineSelectActive && selectedMachine === t.id;
            const short = t.id.length > 24 ? t.id.slice(-24) : t.id;
            return (
              <button
                type="button"
                key={t.id}
                onClick={machineSelectActive ? () => setSelectedMachine(t.id) : undefined}
                disabled={!machineSelectActive}
                title={machineSelectActive ? `Show ${t.role} metrics for ${t.id}` : `${t.role} machine — workload dashboard is run-wide, no per-machine view`}
                className={`inline-flex items-center gap-1.5 px-1.5 py-0.5 text-[10px] font-mono border transition-colors ${
                  active
                    ? "border-primary bg-primary/10 text-primary"
                    : "border-zinc-800 hover:border-zinc-600 text-zinc-300"
                } ${machineSelectActive ? "cursor-pointer" : "cursor-default opacity-80"}`}
              >
                <span className={`px-1 py-px text-[9px] border ${rs.cls}`}>
                  {rs.label}
                </span>
                <span className="truncate max-w-[16rem]">{short}</span>
              </button>
            );
          })}
        </div>
      )}
      <iframe
        src={iframeSrc}
        className="w-full border-0 h-[calc(100vh-14rem)]"
        title="Run Metrics Dashboard"
        sandbox="allow-scripts allow-same-origin allow-popups allow-forms"
      />
    </>
  );
}

export function RunDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [shareUrl, setShareUrl] = useState<string | null>(null);
  const [sharing, setSharing] = useState(false);
  // Convert-to-preset dialog state. Captures the run's config as a reusable
  // RunPreset; identity (id/name/desc) is stripped server-side.
  const [presetOpen, setPresetOpen] = useState(false);
  const [presetName, setPresetName] = useState("");
  const [presetDescription, setPresetDescription] = useState("");
  const [presetSaving, setPresetSaving] = useState(false);
  const [presetSaved, setPresetSaved] = useState(false);
  const [presetErr, setPresetErr] = useState("");
  const [logFocusPhase, setLogFocusPhase] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState(() => {
    const params = new URLSearchParams(window.location.search);
    if (params.get("tab")) return params.get("tab")!;
    if (window.location.hash.startsWith("#L")) return "logs";
    return "overview";
  });
  const [, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [grafana, setGrafana] = useState<GrafanaSettings | null>(null);
  const [renderedConfigs, setRenderedConfigs] = useState<Record<string, string>>({});
  const confirm = useConfirm();
  const snapshotRef = useRef(snapshot);
  snapshotRef.current = snapshot;

  const fetchStatus = useCallback(async () => {
    if (!id) return;
    try {
      const snap = await getRunStatus(id);
      setSnapshot(snap);
      setError(null);
    } catch (err) {
      // Run may not be ready yet (DAG still building) — don't show error immediately.
      if (!snapshotRef.current) {
        setError(null); // suppress error, keep showing loading
      } else {
        setError(err instanceof Error ? err.message : "Failed to fetch status");
      }
    } finally {
      setLoading(false);
    }
  }, [id]);

  const finishedRef = useRef(false);

  useEffect(() => {
    fetchStatus();
    const interval = setInterval(() => {
      if (finishedRef.current) {
        clearInterval(interval);
        return;
      }
      fetchStatus();
    }, 3000);
    return () => clearInterval(interval);
  }, [fetchStatus]);

  // Track whether run is finished to stop polling.
  useEffect(() => {
    if (snapshot && snapshot.nodes.length > 0) {
      const hasPending = snapshot.nodes.some(n => n.status === "pending");
      finishedRef.current = !hasPending;
    }
  }, [snapshot]);

  useEffect(() => {
    getGrafanaSettings()
      .then(setGrafana)
      .catch(() => setGrafana(null));
  }, []);

  // Fetch the rendered config files (postgresql.conf, ydb.yaml, …) once the
  // run snapshot lands. Server derives them from the run's saved RunConfig,
  // so they're available the moment a run is created — no waiting on the
  // agent to push effective_configs back.
  useEffect(() => {
    if (!id || !snapshot?.state?.run_config) return;
    let cancelled = false;
    getRunRenderedConfigs(id)
      .then((m) => { if (!cancelled) setRenderedConfigs(m || {}); })
      .catch(() => { if (!cancelled) setRenderedConfigs({}); });
    return () => { cancelled = true; };
  }, [id, snapshot?.state?.run_config]);

  const [cancelling, setCancelling] = useState(false);

  async function handleCancel() {
    if (!id || cancelling) return;
    setCancelling(true);
    try {
      await cancelRun(id);
      // Don't reset cancelling — polling will show the run finishing via teardown.
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to cancel run");
      setCancelling(false);
    }
  }

  async function handleDelete() {
    if (!id) return;
    if (!(await confirm({ title: `Delete run "${id}"?`, description: "This will also remove its Docker resources. Cannot be undone.", danger: true }))) return;
    setDeleting(true);
    try {
      await deleteRun(id);
      navigate("/runs");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete run");
      setDeleting(false);
    }
  }

  const nodes: NodeStatus[] = snapshot?.nodes || [];
  const hasFailed = nodes.some((n) => n.status === "failed");
  const hasCancelled = nodes.some((n) => n.status === "cancelled");
  const hasRunning = nodes.some((n) => n.status === "running");
  const hasPending = nodes.some((n) => n.status === "pending");
  const inTeardown = nodes.some((n) => n.id === "teardown" && (n.status === "running" || n.status === "done"));

  // Run is finished when no nodes are pending or running.
  const isFinished = snapshot
    ? snapshot.nodes.length > 0 && !hasPending && !hasRunning
    : false;

  // Cancelled = has cancelled nodes (proper FSM state from backend).
  const isCancelled = hasCancelled;

  const runConfig = useMemo<RunConfig | null>(() => {
    const rc = snapshot?.state?.run_config;
    if (!rc) return null;
    try {
      return typeof rc === "string" ? JSON.parse(rc) : (rc as unknown as RunConfig);
    } catch {
      return null;
    }
  }, [snapshot]);

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="min-w-0">
            {runConfig?.name ? (
              <>
                <h1 className="text-lg font-semibold leading-tight">{runConfig.name}</h1>
                <p className="text-[11px] font-mono text-zinc-500 leading-tight">{id}</p>
              </>
            ) : (
              <>
                <h1 className="text-lg font-semibold font-mono">{id}</h1>
                <p className="text-sm text-muted-foreground">Run detail view</p>
              </>
            )}
            {runConfig?.description && (
              <p className="text-xs text-zinc-400 mt-1 max-w-2xl whitespace-pre-wrap">
                {runConfig.description}
              </p>
            )}
          </div>
          {/* Run status badge */}
          {snapshot && (
            isCancelled ? (
              <Badge className="bg-zinc-500/20 text-zinc-400 border-zinc-500/30">Cancelled</Badge>
            ) : cancelling ? (
              <Badge className="bg-amber-500/20 text-amber-400 border-amber-500/30 animate-pulse">Cancelling...</Badge>
            ) : !isFinished ? (
              <Badge className="bg-blue-500/20 text-blue-400 border-blue-500/30 animate-pulse">Running</Badge>
            ) : hasFailed ? (
              <Badge variant="destructive">Failed</Badge>
            ) : (
              <Badge variant="success">Completed</Badge>
            )
          )}
        </div>
        <div className="flex items-center gap-3">
          <Button variant="outline" size="sm" onClick={fetchStatus}>
            <RefreshCw className="h-3.5 w-3.5" />
            Refresh
          </Button>

          {!isFinished && snapshot && !inTeardown && (
            <Button
              variant="outline"
              size="sm"
              onClick={handleCancel}
              disabled={cancelling}
              className="border-amber-800 text-amber-400 hover:bg-amber-500/10"
            >
              <StopCircle className="h-3.5 w-3.5" />
              {cancelling ? "Cancelling..." : "Cancel Run"}
            </Button>
          )}

          {runConfig && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setPresetName(runConfig.name || "");
                setPresetDescription(runConfig.description || "");
                setPresetSaved(false);
                setPresetErr("");
                setPresetOpen(true);
              }}
            >
              <FlaskConical className="h-3.5 w-3.5" />
              Save as Preset
            </Button>
          )}

          {isFinished && (
            <>
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  if (runConfig) {
                    sessionStorage.setItem("rerun_config", JSON.stringify(runConfig));
                  }
                  navigate(`/runs/new?kind=${runConfig?.database?.kind || "postgres"}`);
                }}
              >
                <RotateCcw className="h-3.5 w-3.5" />
                Rerun
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={sharing}
                className="border-primary/40 text-primary hover:bg-primary/10"
                onClick={async () => {
                  if (shareUrl) {
                    navigator.clipboard.writeText(window.location.origin + shareUrl);
                    return;
                  }
                  setSharing(true);
                  try {
                    const res = await createShareLink(id!);
                    setShareUrl(res.url);
                    navigator.clipboard.writeText(window.location.origin + res.url);
                  } catch (err) {
                    setError(err instanceof Error ? err.message : "Failed to create share link");
                  } finally {
                    setSharing(false);
                  }
                }}
              >
                {shareUrl ? <Check className="h-3.5 w-3.5" /> : <Share2 className="h-3.5 w-3.5" />}
                {sharing ? "Sharing..." : shareUrl ? "Copied!" : "Share"}
              </Button>
              {shareUrl && (
                <a
                  href={shareUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-[10px] font-mono text-primary hover:underline truncate max-w-xs"
                  onClick={(e) => { e.preventDefault(); navigator.clipboard.writeText(window.location.origin + shareUrl); }}
                  title="Click to copy"
                >
                  {window.location.origin}{shareUrl}
                </a>
              )}
              <Button
                variant="destructive"
                size="sm"
                onClick={handleDelete}
                disabled={deleting}
              >
                <Trash2 className="h-3.5 w-3.5" />
                {deleting ? "Deleting..." : "Delete"}
              </Button>
            </>
          )}
        </div>
      </div>

      <Dialog open={presetOpen} onOpenChange={setPresetOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Save run as preset</DialogTitle>
            <DialogDescription>
              Captures this run's full config as a reusable Run Preset.
              Identity (run id, name, description) is stripped — only workload + infra knobs are stored.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3 pt-2">
            <div className="space-y-1.5">
              <label className="text-xs text-zinc-500 font-mono">Name</label>
              <input
                value={presetName}
                onChange={(e) => setPresetName(e.target.value)}
                autoFocus
                placeholder="e.g. tpcc-pg17-100vu-baseline"
                className="w-full bg-[#0a0a0a] border border-zinc-800 px-2 py-1.5 text-sm font-mono text-zinc-200 focus:outline-none focus:border-primary/60"
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-xs text-zinc-500 font-mono">Description</label>
              <textarea
                value={presetDescription}
                onChange={(e) => setPresetDescription(e.target.value)}
                rows={3}
                placeholder="optional"
                className="w-full bg-[#0a0a0a] border border-zinc-800 px-2 py-1.5 text-xs font-mono text-zinc-300 focus:outline-none focus:border-primary/60 resize-none"
              />
            </div>
            {presetErr && <div className="text-xs text-destructive font-mono">{presetErr}</div>}
            <div className="flex items-center gap-2">
              <Button
                onClick={async () => {
                  if (!runConfig || !presetName.trim()) return;
                  setPresetSaving(true); setPresetErr("");
                  try {
                    await createRunPreset({
                      name: presetName.trim(),
                      description: presetDescription.trim(),
                      db_kind: (runConfig.database?.kind || "postgres") as "postgres",
                      config: runConfig,
                    });
                    setPresetSaved(true);
                    window.setTimeout(() => setPresetOpen(false), 700);
                  } catch (e) {
                    setPresetErr(e instanceof Error ? e.message : "Failed");
                  } finally {
                    setPresetSaving(false);
                  }
                }}
                disabled={presetSaving || presetSaved || !presetName.trim()}
              >
                {presetSaved ? "Saved!" : presetSaving ? "Saving..." : "Save"}
              </Button>
              {presetSaved && (
                <Button variant="outline" size="sm" onClick={() => navigate("/run-presets")}>
                  Open Run Presets
                </Button>
              )}
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {!snapshot && !error && (
        <div className="text-sm text-muted-foreground">Loading run status...</div>
      )}

      {error && (
        <div className="flex items-center gap-2 text-sm p-3 border border-destructive/30 text-destructive">
          <AlertCircle className="h-4 w-4" />
          {error}
        </div>
      )}

      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="topology">Topology</TabsTrigger>
          <div className="w-px h-4 bg-zinc-800 mx-1" />
          <TabsTrigger value="logs">Logs</TabsTrigger>
          <TabsTrigger value="metrics">Metrics</TabsTrigger>
          {grafana?.embed_enabled && (
            <TabsTrigger value="grafana">Grafana</TabsTrigger>
          )}
        </TabsList>

        <TabsContent value="overview">
          <Card className="h-[calc(100vh-11rem)]">
            <CardContent className="p-0 h-full">
              <RunOverview
                nodes={nodes}
                snapshot={snapshot}
                runStatus={isCancelled ? "cancelled" : cancelling ? "cancelling" : !isFinished ? "running" : hasFailed ? "failed" : "completed"}
                onViewLogs={(phase) => { setLogFocusPhase(phase); setActiveTab("logs"); }}
                renderedConfigs={renderedConfigs}
              />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="topology">
          <Card className="h-[calc(100vh-11rem)]">
            <CardContent className="p-0 h-full">
              <TopologyFlow config={runConfig} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="logs" forceMount className="data-[state=inactive]:hidden">
          <Card className="h-[calc(100vh-11rem)]">
            <CardContent className="p-0 h-full relative">
              <LogStream runID={id} snapshot={snapshot} focusPhase={logFocusPhase} />
            </CardContent>
          </Card>
        </TabsContent>

        {/* Metrics tab — direct API data */}
        <TabsContent value="metrics">
          {id ? (
            <MetricsPanel runID={id} startedAt={snapshot?.started_at} finishedAt={snapshot?.finished_at} />
          ) : (
            <p className="text-sm text-muted-foreground p-4">No run ID</p>
          )}
        </TabsContent>

        {/* Grafana embed tab. forceMount keeps the iframe in the DOM even
            when another tab is active — Radix sets `hidden` on inactive
            content (display:none) so the iframe stays loaded but invisible.
            Without this every tab switch reloads the dashboard, which takes
            5+ seconds and burns Grafana resources. */}
        {grafana?.embed_enabled && (
          <TabsContent value="grafana" forceMount className="data-[state=inactive]:hidden">
            <Card>
              <CardContent className="p-0">
                <DashboardSelector
                  grafana={grafana}
                  runID={id || ""}
                  dbKind={(() => { try { const rc = snapshot?.state?.run_config; if (!rc) return undefined; const cfg = typeof rc === "string" ? JSON.parse(rc) : rc; return cfg?.database?.kind; } catch { return undefined; } })()}
                  startedAt={snapshot?.started_at}
                  finishedAt={snapshot?.finished_at}
                  targets={snapshot?.state?.targets}
                />
              </CardContent>
            </Card>
          </TabsContent>
        )}
      </Tabs>
    </div>
  );
}
