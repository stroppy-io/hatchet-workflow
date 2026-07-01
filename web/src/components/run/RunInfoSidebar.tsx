// RUN INFO sidebar — compact, read-only summary of the run record + overview
// snapshot. Grouped sections (identity / database / workload / infrastructure)
// of label→value mono rows, matching the dense terminal aesthetic of the app.
import { useEffect, useState, type ReactNode } from "react";
import {
  Activity,
  AlertTriangle,
  CheckCircle2,
  Clock,
  Cpu,
  Database,
  FileCode,
  Gauge,
  Hash,
  Layers,
  Network,
  Tag,
  Timer,
  Users,
} from "lucide-react";
import type { OverviewVM, PipelineNodeVM } from "@/services/run_overview";
import type { RunVM } from "@/services/runs";
import {
  fallbackAuthorDisplay,
  resolveAuthorDisplay,
  type AuthorDisplay,
} from "@/lib/author-display";
import { Avatar } from "@/components/Avatar";

function fmtTs(iso?: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime()) || d.getFullYear() < 2000) return "";
  return d.toLocaleString("en-GB", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

function fmtDur(sec?: number): string {
  if (sec === undefined || !Number.isFinite(sec)) return "—";
  if (sec < 60) return `${Math.round(sec)}s`;
  if (sec < 3600) return `${Math.floor(sec / 60)}m ${Math.round(sec % 60)}s`;
  return `${Math.floor(sec / 3600)}h ${Math.floor((sec % 3600) / 60)}m`;
}

const SOURCE_LABEL: Record<string, string> = {
  temporal: "live",
  persisted: "persisted",
  plan: "plan",
  registry: "registry",
  synthetic: "synthetic",
};

function countPipeline(nodes: PipelineNodeVM[]) {
  const acc = { total: 0, running: 0, completed: 0, failed: 0, pending: 0, cancelled: 0 };
  const visit = (list: PipelineNodeVM[]) => {
    for (const node of list) {
      acc.total += 1;
      if (node.status === "running" || node.status === "cancelling") acc.running += 1;
      else if (node.status === "completed") acc.completed += 1;
      else if (node.status === "failed") acc.failed += 1;
      else if (node.status === "cancelled") acc.cancelled += 1;
      else acc.pending += 1;
      if (node.children.length) visit(node.children);
    }
  };
  visit(nodes);
  return acc;
}

function compactCounts(items: Array<[string, number]>): string {
  return items.filter(([, n]) => n > 0).map(([k, n]) => `${k}:${n}`).join(" ");
}

// Live-ticking elapsed time for in-flight runs.
function Elapsed({ startedAt }: { startedAt: string }) {
  const [, force] = useState(0);
  useEffect(() => {
    const start = new Date(startedAt).getTime();
    if (Number.isNaN(start)) return;
    const iv = setInterval(() => force((n) => n + 1), 1000);
    return () => clearInterval(iv);
  }, [startedAt]);
  const sec = Math.max(0, Math.round((Date.now() - new Date(startedAt).getTime()) / 1000));
  return <>{fmtDur(sec)}</>;
}

function Group({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="border-b border-border/60 px-3 py-2.5">
      <div className="mb-1.5 font-mono text-[10px] uppercase tracking-[0.16em] text-muted-foreground">
        {label}
      </div>
      <div className="space-y-1">{children}</div>
    </div>
  );
}

function Row({
  icon: Icon,
  label,
  value,
}: {
  icon?: typeof Cpu;
  label: string;
  value?: ReactNode;
}) {
  if (value === undefined || value === null || value === "" || value === "—") return null;
  return (
    <div className="flex items-start gap-2">
      {Icon && <Icon className="mt-[3px] h-3 w-3 shrink-0 text-muted-foreground/70" />}
      <span className="mt-px w-16 shrink-0 font-mono text-[11px] text-muted-foreground">{label}</span>
      <span className="min-w-0 flex-1 break-all font-mono text-[11px] text-foreground/90">{value}</span>
    </div>
  );
}

export function RunInfoSidebar({ overview, run }: { overview: OverviewVM; run?: RunVM }) {
  const isRunning = overview.status === "running" || overview.status === "cancelling";
  const [ownerDisplay, setOwnerDisplay] = useState<AuthorDisplay | null>(null);
  const pipeline = countPipeline(overview.pipeline);
  const workers = overview.workers.reduce(
    (acc, worker) => {
      acc.total += 1;
      if (worker.presence === "online" || worker.online) acc.online += 1;
      else if (worker.presence === "stale") acc.stale += 1;
      else if (worker.presence === "offline" || worker.presence === "terminated") acc.offline += 1;
      else acc.unknown += 1;
      return acc;
    },
    { total: 0, online: 0, stale: 0, offline: 0, unknown: 0 },
  );
  const errorEvents = overview.timeline.filter((e) => e.severity === "error").length;
  const warningEvents = overview.timeline.filter((e) => e.severity === "warning").length;

  useEffect(() => {
    const authorId = run?.authorId ?? "";
    if (!authorId) {
      setOwnerDisplay(null);
      return;
    }
    let cancelled = false;
    setOwnerDisplay(fallbackAuthorDisplay(authorId));
    void resolveAuthorDisplay(authorId).then((display) => {
      if (!cancelled) setOwnerDisplay(display);
    });
    return () => {
      cancelled = true;
    };
  }, [run?.authorId]);

  return (
    <div className="flex flex-col">
      <Group label="Identity">
        <Row icon={Activity} label="status" value={overview.status} />
        <Row icon={Hash} label="id" value={<span title={overview.runId}>{overview.runId.slice(0, 18)}…</span>} />
        {run?.authorId && (
          <Row
            icon={Users}
            label="owner"
            value={
              <span className="inline-flex min-w-0 items-center gap-1.5" title={ownerDisplay?.title ?? run.authorId}>
                <Avatar name={ownerDisplay?.avatarName ?? run.authorId} size={14} />
                <span className="truncate">{ownerDisplay?.label ?? run.authorId}</span>
              </span>
            }
          />
        )}
        <Row icon={Clock} label="created" value={fmtTs(run?.createdAt)} />
        <Row icon={Clock} label="started" value={fmtTs(overview.startedAt) || "—"} />
        <Row icon={Clock} label="finished" value={fmtTs(overview.finishedAt)} />
        <Row
          icon={Timer}
          label="duration"
          value={
            isRunning && overview.startedAt ? (
              <Elapsed startedAt={overview.startedAt} />
            ) : (
              fmtDur(overview.durationSec)
            )
          }
        />
        {run?.trigger && <Row icon={Tag} label="trigger" value={run.trigger} />}
        {run?.suiteRunId && <Row icon={Layers} label="suite" value={<span title={run.suiteRunId}>{run.suiteRunId.slice(0, 12)}…</span>} />}
      </Group>

      {run && (run.dbKind || run.topologyLabel) && (
        <Group label="Database">
          <Row icon={Database} label="kind" value={run.dbKind || "—"} />
          {run.topologyLabel && <Row icon={Layers} label="topology" value={run.topologyLabel} />}
          {run.dbPresetId && (
            <Row icon={Tag} label="preset" value={run.dbPresetId.slice(0, 8)} />
          )}
          {run.testPresetId && (
            <Row icon={Tag} label="test" value={run.testPresetId.slice(0, 8)} />
          )}
        </Group>
      )}

      {run && (run.workload || run.stroppyVersion || (run.workloadSegments?.length ?? 0) > 0) && (
        <Group label="Workload">
          <Row icon={FileCode} label="name" value={run.workload || "—"} />
          {run.protocol && <Row icon={Network} label="protocol" value={run.protocol} />}
          {run.stroppyVersion && <Row icon={Tag} label="stroppy" value={run.stroppyVersion} />}
          {run.workloadPresetId && (
            <Row icon={Tag} label="preset" value={run.workloadPresetId.slice(0, 8)} />
          )}
          {run.workloadSegments?.map((seg, i) => (
            <Row
              key={i}
              icon={Gauge}
              label={seg.name || `seg ${i + 1}`}
              value={
                <span className="break-words">
                  {[
                    seg.script,
                    seg.vus !== undefined && `vus=${seg.vus}`,
                    seg.duration ? `dur=${seg.duration}` : seg.iterations !== undefined && `iters=${seg.iterations}`,
                    seg.poolSize !== undefined && `pool=${seg.poolSize}`,
                    seg.scaleFactor !== undefined && `scale=${seg.scaleFactor}`,
                  ]
                    .filter(Boolean)
                    .join(" · ")}
                </span>
              }
            />
          ))}
        </Group>
      )}

      {run && (run.provider || run.nodeCount > 0) && (
        <Group label="Infrastructure">
          <Row icon={Cpu} label="provider" value={run.provider || "—"} />
          {run.nodeCount > 0 && <Row icon={Users} label="nodes" value={String(run.nodeCount)} />}
        </Group>
      )}

      <Group label="Runtime">
        <Row
          icon={Users}
          label="agents"
          value={
            workers.total > 0
              ? compactCounts([
                  ["online", workers.online],
                  ["stale", workers.stale],
                  ["offline", workers.offline],
                  ["unknown", workers.unknown],
                ]) || String(workers.total)
              : undefined
          }
        />
        <Row
          icon={CheckCircle2}
          label="stages"
          value={
            pipeline.total > 0
              ? compactCounts([
                  ["run", pipeline.running],
                  ["ok", pipeline.completed],
                  ["fail", pipeline.failed],
                  ["wait", pipeline.pending],
                  ["cancel", pipeline.cancelled],
                ]) || String(pipeline.total)
              : undefined
          }
        />
        <Row icon={Clock} label="events" value={overview.timeline.length ? String(overview.timeline.length) : undefined} />
        <Row
          icon={AlertTriangle}
          label="issues"
          value={
            errorEvents || warningEvents
              ? compactCounts([
                  ["errors", errorEvents],
                  ["warn", warningEvents],
                ])
              : undefined
          }
        />
        <Row icon={Tag} label="source" value={SOURCE_LABEL[overview.source] ?? overview.source} />
        <Row icon={Clock} label="observed" value={fmtTs(overview.observedAt)} />
        <Row icon={AlertTriangle} label="degraded" value={overview.degradedReasons.length ? String(overview.degradedReasons.length) : undefined} />
      </Group>

      <Group label="Progress">
        <div className="flex items-center gap-2">
          <div className="h-1 flex-1 overflow-hidden rounded-full bg-muted">
            <div
              className={`h-full rounded-full transition-all duration-700 ${
                overview.status === "failed"
                  ? "bg-destructive"
                  : overview.status === "completed"
                    ? "bg-success"
                    : overview.status === "cancelled" || overview.status === "cancelling"
                      ? "bg-pending"
                      : "bg-primary"
              }`}
              style={{ width: `${Math.min(100, Math.max(0, overview.progressPct))}%` }}
            />
          </div>
          <span className="shrink-0 font-mono text-[11px] tabular-nums text-muted-foreground">
            {Math.round(overview.progressPct)}%
          </span>
        </div>
      </Group>
    </div>
  );
}
