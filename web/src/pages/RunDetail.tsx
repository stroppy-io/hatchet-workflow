import { useEffect, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { clients } from "@/api/clients";
import { protoTsToISO } from "@/lib/proto-helpers";
import { useWatchTestRun } from "@/hooks/useWatchTestRun";
import { useStreamTestRunLogs } from "@/hooks/useStreamTestRunLogs";
import type { TestRun, TestRunProgress } from "@/lib/proto/cloud/v1/testing/test_run_pb";
import { NodeRunStatus } from "@/lib/proto/cloud/v1/system/dag_pb";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { AlertCircle, RefreshCw, StopCircle } from "lucide-react";

function formatDate(iso: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "—";
  return d.toLocaleString("en-GB", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

export function RunDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const [testRun, setTestRun] = useState<TestRun | null>(null);
  const [progress, setProgress] = useState<TestRunProgress | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [cancelling, setCancelling] = useState(false);
  const [cancelMsg, setCancelMsg] = useState<string | null>(null);

  const { progress: watchProgress, error: watchError } = useWatchTestRun(id ?? null);
  const { lines, error: logsError } = useStreamTestRunLogs(id ?? null);

  async function load() {
    if (!id) return;
    setLoading(true);
    setError(null);
    try {
      const r = await clients.testRun.getTestRun({ value: id });
      setTestRun(r.testRun ?? null);
      if (r.progress) setProgress(r.progress);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load run");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, [id]);

  // Update progress from watch stream
  useEffect(() => {
    if (watchProgress.length > 0) {
      setProgress(watchProgress[watchProgress.length - 1]);
    }
  }, [watchProgress]);

  async function handleCancel() {
    if (!id || cancelling) return;
    setCancelling(true);
    setCancelMsg(null);
    try {
      await clients.testRun.cancelTestRun({ value: id });
      await load();
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed to cancel";
      setCancelMsg(msg);
    } finally {
      setCancelling(false);
    }
  }

  if (loading && !testRun) {
    return (
      <div className="p-6 text-sm text-muted-foreground">Loading run...</div>
    );
  }

  if (error && !testRun) {
    return (
      <div className="p-6 flex items-center gap-2 text-sm text-destructive">
        <AlertCircle className="h-4 w-4" />
        {error}
      </div>
    );
  }

  const runId = testRun?.id?.value ?? id ?? "";
  const name = testRun?.identity?.name;
  const isRunning = !!testRun?.dagRunId;
  const createdAt = protoTsToISO(testRun?.timestamps?.createdAt);

  // Database summary
  const dbVariant = testRun?.database?.databaseVariant;
  let dbSummary = "—";
  if (dbVariant?.case === "databasePresetId") {
    dbSummary = `preset:${dbVariant.value.value}`;
  } else if (dbVariant?.case === "database") {
    dbSummary = dbVariant.value.variant?.case ?? "inline";
  }

  // Workload summary
  const wlVariant = testRun?.workload?.workloadVariant;
  let wlSummary = "—";
  if (wlVariant?.case === "workloadPresetId") {
    wlSummary = `preset:${wlVariant.value.value}`;
  } else if (wlVariant?.case === "workload") {
    const s = wlVariant.value.shape;
    wlSummary = s ? `${s.script} / ${s.protocol}` : "inline";
  }

  // Last 200 log lines
  const logLines = lines.slice(-200);

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="min-w-0">
          {name ? (
            <>
              <h1 className="text-lg font-semibold leading-tight">{name}</h1>
              <p className="text-[11px] font-mono text-zinc-500 leading-tight">{runId}</p>
            </>
          ) : (
            <h1 className="text-lg font-semibold font-mono">{runId}</h1>
          )}
          <p className="text-xs text-zinc-500">Created: {formatDate(createdAt)}</p>
        </div>

        <div className="flex items-center gap-2">
          <Badge variant={isRunning ? "warning" : "secondary"}>
            {isRunning ? "RUNNING" : "PENDING"}
          </Badge>
          <Button variant="outline" size="sm" onClick={load}>
            <RefreshCw className="h-3.5 w-3.5" />
            Refresh
          </Button>
          {isRunning && (
            <Button
              variant="outline"
              size="sm"
              onClick={handleCancel}
              disabled={cancelling}
              className="border-amber-800 text-amber-400 hover:bg-amber-500/10"
            >
              <StopCircle className="h-3.5 w-3.5" />
              {cancelling ? "Cancelling..." : "Cancel"}
            </Button>
          )}
        </div>
      </div>

      {cancelMsg && (
        <div className="flex items-center gap-2 text-sm p-3 border border-amber-500/30 text-amber-400">
          <AlertCircle className="h-4 w-4" />
          {cancelMsg}
        </div>
      )}

      {/* Details */}
      <div className="grid grid-cols-2 gap-4">
        <div className="border border-zinc-800 bg-zinc-900/30 p-4 space-y-2">
          <div className="text-[10px] font-mono text-zinc-600 uppercase tracking-wider">Database</div>
          <div className="text-sm font-mono text-zinc-300">{dbSummary}</div>
        </div>
        <div className="border border-zinc-800 bg-zinc-900/30 p-4 space-y-2">
          <div className="text-[10px] font-mono text-zinc-600 uppercase tracking-wider">Workload</div>
          <div className="text-sm font-mono text-zinc-300">{wlSummary}</div>
        </div>
        {testRun?.dagRunId && (
          <div className="border border-zinc-800 bg-zinc-900/30 p-4 space-y-2 col-span-2">
            <div className="text-[10px] font-mono text-zinc-600 uppercase tracking-wider">DAG Run ID</div>
            <div className="text-sm font-mono text-zinc-300">{testRun.dagRunId.value}</div>
          </div>
        )}
      </div>

      {/* Progress (from WatchTestRun) */}
      {watchError && (
        <div className="text-xs text-zinc-500 border border-zinc-800 p-3 font-mono">
          Watch: {watchError.message}
        </div>
      )}

      {progress && progress.steps.length > 0 && (
        <div className="border border-zinc-800 bg-zinc-900/30 p-4 space-y-2">
          <div className="text-[10px] font-mono text-zinc-600 uppercase tracking-wider mb-3">
            Progress — {progress.status}
          </div>
          <div className="space-y-1">
            {progress.steps.map((step) => (
              <div
                key={step.id}
                className="flex items-center gap-3 text-xs font-mono"
              >
                <span
                  className={`w-16 shrink-0 ${
                    step.status === NodeRunStatus.SUCCEEDED
                      ? "text-emerald-400"
                      : step.status === NodeRunStatus.FAILED
                        ? "text-red-400"
                        : step.status === NodeRunStatus.RUNNING
                          ? "text-amber-400"
                          : "text-zinc-600"
                  }`}
                >
                  {NodeRunStatus[step.status] ?? String(step.status)}
                </span>
                <span className="text-zinc-400 truncate flex-1">{step.name || step.id}</span>
                {step.error && (
                  <span className="text-red-400 truncate max-w-xs" title={step.error}>
                    {step.error}
                  </span>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Logs */}
      <div className="border border-zinc-800 bg-zinc-900/30">
        <div className="flex items-center justify-between px-4 py-2 border-b border-zinc-800">
          <div className="text-[10px] font-mono text-zinc-600 uppercase tracking-wider">
            Logs {logsError ? `(${logsError.message})` : `— ${logLines.length} lines`}
          </div>
        </div>
        {logLines.length > 0 ? (
          <pre className="p-4 text-[11px] font-mono leading-relaxed text-zinc-400 overflow-auto max-h-96">
            {logLines.map((l, i) => (
              <div key={i}>{l.line}</div>
            ))}
          </pre>
        ) : (
          <div className="p-4 text-xs text-zinc-600 font-mono">
            {logsError
              ? logsError.message
              : "No log lines yet — logs appear as the run executes."}
          </div>
        )}
      </div>

      {/* Back */}
      <div>
        <button
          onClick={() => navigate("/runs")}
          className="text-xs text-zinc-500 hover:text-zinc-300 font-mono underline underline-offset-2"
        >
          ← Back to runs
        </button>
      </div>
    </div>
  );
}
