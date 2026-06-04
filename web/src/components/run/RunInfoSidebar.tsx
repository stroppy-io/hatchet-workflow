// RUN INFO sidebar — compact, read-only summary of the run record + overview
// snapshot. Grouped sections (identity / database / workload / infrastructure)
// of label→value mono rows, matching the dense terminal aesthetic of the app.
import { useEffect, useState, type ReactNode } from "react";
import {
  Clock,
  Cpu,
  Database,
  FileCode,
  Hash,
  Layers,
  Network,
  Tag,
  Timer,
  Users,
} from "lucide-react";
import type { OverviewVM } from "@/services/run_overview";
import type { RunVM } from "@/services/runs";

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
  return (
    <div className="flex flex-col">
      <Group label="Identity">
        <Row icon={Hash} label="id" value={<span title={overview.runId}>{overview.runId.slice(0, 18)}…</span>} />
        <Row icon={Clock} label="started" value={fmtTs(overview.startedAt) || "—"} />
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
      </Group>

      {run && (run.dbKind || run.topologyLabel) && (
        <Group label="Database">
          <Row icon={Database} label="kind" value={run.dbKind || "—"} />
          {run.topologyLabel && <Row icon={Layers} label="topology" value={run.topologyLabel} />}
          {run.dbPresetId && (
            <Row icon={Tag} label="preset" value={run.dbPresetId.slice(0, 8)} />
          )}
        </Group>
      )}

      {run && (run.workload || run.stroppyVersion) && (
        <Group label="Workload">
          <Row icon={FileCode} label="script" value={run.workload || "—"} />
          {run.protocol && <Row icon={Network} label="protocol" value={run.protocol} />}
          {run.stroppyVersion && <Row icon={Tag} label="stroppy" value={run.stroppyVersion} />}
          {run.workloadPresetId && (
            <Row icon={Tag} label="preset" value={run.workloadPresetId.slice(0, 8)} />
          )}
        </Group>
      )}

      {run && (run.provider || run.nodeCount > 0) && (
        <Group label="Infrastructure">
          <Row icon={Cpu} label="provider" value={run.provider || "—"} />
          {run.nodeCount > 0 && <Row icon={Users} label="nodes" value={String(run.nodeCount)} />}
        </Group>
      )}

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
