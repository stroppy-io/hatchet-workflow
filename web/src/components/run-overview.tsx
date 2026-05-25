import { useMemo, useState } from "react";
import { Check, ChevronRight, Circle, Clock, Cpu, CornerDownRight, FileCode, HardDrive, Layers, Loader2, MemoryStick, Repeat, Server, Timer, Users, X, Zap } from "lucide-react";

import { PayloadView, summarizeAny } from "@/components/run-ops";
import { CopyButton } from "@/components/ui/copy-button";
import { componentColor, databaseColor } from "@/lib/db-colors";
import { formatDuration, formatTimestamp, statusLabel } from "@/lib/format";
import { componentKindLabel, databaseKindLabel, topologySummary, workloadProtocolLabel } from "@/lib/preset-summary";
import { cn } from "@/lib/utils";
import type { TestRun } from "@/lib/proto/cloud/v1/models/testing_pb.ts";
import type { Dag, Dag_Node } from "@/lib/proto/cloud/v1/runtime/primitive/dag_pb.ts";
import { Dag_Node_TaskState_ExecutionLocus } from "@/lib/proto/cloud/v1/runtime/primitive/dag_pb.ts";
import { Status } from "@/lib/proto/cloud/v1/runtime/primitive/status_pb.ts";

// Per-status palette mapped onto the app's semantic colors (success/primary/
// warning/destructive/pending). One source of truth for dots + text + tints.
type StatusStyle = { text: string; dot: string; tint: string };
const STATUS_STYLE: Record<number, StatusStyle> = {
  [Status.COMPLETED]: { text: "text-success", dot: "bg-success", tint: "bg-success/[0.04]" },
  [Status.RUNNING]: { text: "text-primary", dot: "bg-primary", tint: "bg-primary/[0.05]" },
  [Status.RETRY_WAIT]: { text: "text-warning", dot: "bg-warning", tint: "bg-warning/[0.05]" },
  [Status.CANCELLING]: { text: "text-warning", dot: "bg-warning", tint: "bg-warning/[0.05]" },
  [Status.FAILED]: { text: "text-destructive", dot: "bg-destructive", tint: "bg-destructive/[0.05]" },
  [Status.CANCELLED]: { text: "text-muted-foreground", dot: "bg-pending", tint: "bg-muted/40" },
  [Status.SKIPPED]: { text: "text-muted-foreground", dot: "bg-pending", tint: "bg-muted/30" },
  [Status.PENDING]: { text: "text-muted-foreground", dot: "bg-pending/60", tint: "bg-transparent" },
};

function styleOf(status?: Status): StatusStyle {
  return (status !== undefined && STATUS_STYLE[status]) || STATUS_STYLE[Status.PENDING];
}

function StepDot({ status }: { status: Status }) {
  const base = "flex size-4 shrink-0 items-center justify-center rounded-full";
  if (status === Status.COMPLETED) return <span className={cn(base, "bg-success")}><Check className="size-2.5 text-background" strokeWidth={3.5} /></span>;
  if (status === Status.FAILED) return <span className={cn(base, "bg-destructive")}><X className="size-2.5 text-background" strokeWidth={3.5} /></span>;
  if (status === Status.CANCELLED || status === Status.SKIPPED) return <span className={cn(base, "bg-pending")}><X className="size-2.5 text-background" strokeWidth={3.5} /></span>;
  if (status === Status.RUNNING) return <span className={cn(base, "animate-pulse bg-primary")}><Loader2 className="size-2.5 animate-spin text-background" strokeWidth={3.5} /></span>;
  if (status === Status.RETRY_WAIT || status === Status.CANCELLING) return <span className={cn(base, "animate-pulse bg-warning")}><Loader2 className="size-2.5 animate-spin text-background" strokeWidth={3.5} /></span>;
  return <span className={cn(base, "border border-border bg-muted")}><Circle className="size-1 text-muted-foreground" fill="currentColor" /></span>;
}

function humanize(value: string): string {
  return value.replace(/[_-]/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

// One labelled section in the left config column.
function Section({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="border-b px-3 py-2.5">
      <div className="mb-1.5 text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">{label}</div>
      {children}
    </div>
  );
}

function Line({ icon: Icon, label, value }: { icon?: typeof Cpu; label: string; value: React.ReactNode }) {
  if (value === null || value === undefined || value === "") return null;
  return (
    <div className="flex items-baseline gap-2 py-[3px] text-xs">
      {Icon ? <Icon className="size-3 shrink-0 translate-y-0.5 text-muted-foreground/60" /> : null}
      <span className="w-16 shrink-0 text-[11px] text-muted-foreground">{label}</span>
      <span className="min-w-0 flex-1 break-all font-mono text-foreground/90">{value}</span>
    </div>
  );
}

// Left column — run identity, status roll-up, and the TestPreset config (db /
// workload / topology / machines), all from proto. Mirrors old_src config panel.
function ConfigColumn({ run, dag }: { run: TestRun; dag?: Dag }) {
  const preset = run.testPreset;
  const database = preset?.database;
  const workload = preset?.workload;
  const machines = preset?.topology?.machines ?? [];

  const nodes = dag?.nodes ?? [];
  const counts = {
    done: nodes.filter((n) => n.status === Status.COMPLETED).length,
    running: nodes.filter((n) => n.status === Status.RUNNING || n.status === Status.RETRY_WAIT || n.status === Status.CANCELLING).length,
    failed: nodes.filter((n) => n.status === Status.FAILED).length,
    cancelled: nodes.filter((n) => n.status === Status.CANCELLED || n.status === Status.SKIPPED).length,
    pending: nodes.filter((n) => n.status === Status.PENDING).length,
  };
  const rollup = styleOf(run.status);

  const limit = workload?.execution?.limit;

  return (
    <div className="w-80 shrink-0 overflow-y-auto border-r">
      {/* Status roll-up */}
      <Section label="Status">
        <div className={cn("flex items-center gap-2 text-sm font-semibold", rollup.text)}>
          <span className={cn("inline-block size-2 rounded-full", rollup.dot, (run.status === Status.RUNNING || run.status === Status.CANCELLING) && "animate-pulse")} />
          {statusLabel(run.status)}
        </div>
        <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] font-mono text-muted-foreground">
          {counts.done > 0 ? <span className="flex items-center gap-1"><span className="inline-block size-1.5 rounded-full bg-success" />{counts.done}</span> : null}
          {counts.running > 0 ? <span className="flex items-center gap-1"><span className="inline-block size-1.5 animate-pulse rounded-full bg-primary" />{counts.running}</span> : null}
          {counts.failed > 0 ? <span className="flex items-center gap-1"><span className="inline-block size-1.5 rounded-full bg-destructive" />{counts.failed}</span> : null}
          {counts.cancelled > 0 ? <span className="flex items-center gap-1"><span className="inline-block size-1.5 rounded-full bg-pending" />{counts.cancelled}</span> : null}
          {counts.pending > 0 ? <span className="flex items-center gap-1"><span className="inline-block size-1.5 rounded-full bg-pending/60" />{counts.pending}</span> : null}
        </div>
      </Section>

      {/* Identity + timing */}
      <Section label="Run">
        <div className="flex items-center gap-1">
          <span className="min-w-0 flex-1 truncate font-mono text-[11px] text-muted-foreground" title={run.entity?.id?.value}>{run.entity?.id?.value}</span>
          <CopyButton value={run.entity?.id?.value ?? ""} />
        </div>
        <div className="mt-1 space-y-px">
          <Line icon={Clock} label="created" value={formatTimestamp(run.entity?.timestamps?.createdAt)} />
          <Line icon={Timer} label="duration" value={dag?.execution?.startedAt ? formatDuration(dag.execution.startedAt, dag.execution.finishedAt) : "—"} />
        </div>
      </Section>

      {/* Database */}
      {database ? (
        <Section label="Database">
          <div className="flex items-baseline gap-2">
            <span className="font-mono text-sm font-semibold" style={{ color: databaseColor(database.kind) }}>{databaseKindLabel(database.kind)}</span>
            {database.version ? <span className="font-mono text-xs text-muted-foreground">v{database.version}</span> : null}
          </div>
          <div className="mt-0.5 text-[11px] text-muted-foreground">{topologySummary(preset?.topology)}</div>
        </Section>
      ) : null}

      {/* Workload */}
      {workload ? (
        <Section label="Workload">
          <Line icon={FileCode} label="script" value={workload.script} />
          {workload.stroppyVersion ? <Line icon={Zap} label="stroppy" value={`v${workload.stroppyVersion}`} /> : null}
          <Line icon={Server} label="protocol" value={workloadProtocolLabel(workload.protocol)} />
          {limit?.case === "duration" ? <Line icon={Clock} label="duration" value={limit.value} /> : null}
          {limit?.case === "iterations" ? <Line icon={Repeat} label="iterations" value={String(limit.value)} /> : null}
          {workload.execution?.vus ? <Line icon={Users} label="VUs" value={String(workload.execution.vus)} /> : null}
          {workload.parameters?.poolSize ? <Line icon={Server} label="pool" value={String(workload.parameters.poolSize)} /> : null}
          {workload.parameters?.scaleFactor ? <Line label="scale" value={String(workload.parameters.scaleFactor)} /> : null}
        </Section>
      ) : null}

      {/* Machines */}
      {machines.length > 0 ? (
        <Section label={`Machines · ${machines.length}`}>
          <div className="space-y-2">
            {machines.map((machine) => (
              <div key={machine.id} className="space-y-1">
                <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5 font-mono text-[11px] text-muted-foreground">
                  <span className="truncate text-foreground/80" title={machine.id}>{machine.id}</span>
                  <Cpu className="size-3 text-muted-foreground/60" />
                  <span>{machine.cores}</span>
                  <MemoryStick className="size-3 text-muted-foreground/60" />
                  <span>{Number(machine.memoryGb)}G</span>
                  <HardDrive className="size-3 text-muted-foreground/60" />
                  <span>{Number(machine.diskGb)}G</span>
                </div>
                {machine.components.length > 0 ? (
                  <div className="flex flex-wrap gap-1">
                    {machine.components.map((component) => (
                      <span
                        key={component.id}
                        className="inline-flex items-center gap-1 rounded-sm border px-1.5 py-px text-[10px] font-mono"
                        style={{ borderColor: `${componentColor(component.kind)}55`, color: componentColor(component.kind) }}
                      >
                        {componentKindLabel(component.kind)}
                      </span>
                    ))}
                  </div>
                ) : null}
              </div>
            ))}
          </div>
        </Section>
      ) : null}
    </div>
  );
}

// Right column — the run's DAG as a vertical pipeline: a progress bar plus a
// connected list of nodes with per-node status, duration, and failure, each
// linking into the filtered log view.
function Pipeline({ dag, loading, error, onViewLogs }: { dag?: Dag; loading?: boolean; error?: string | null; onViewLogs?: (nodeExecutionId: string) => void }) {
  const nodes = dag?.nodes ?? [];

  if (nodes.length === 0) {
    return (
      <div className="flex h-full items-center justify-center px-6 text-center text-xs">
        {loading ? (
          <span className="flex items-center gap-2 text-muted-foreground">
            <Loader2 className="size-4 animate-spin" />
            Loading run graph…
          </span>
        ) : error ? (
          <span className="max-w-md whitespace-pre-wrap text-destructive">{error}</span>
        ) : (
          <span className="text-muted-foreground">No run graph yet — the DAG is compiled once the run is scheduled.</span>
        )}
      </div>
    );
  }

  // Skip is a normal terminal resolution — count it as done so a fully-resolved
  // run reads 6/6, not 5/6. Bar fills when nothing is still active.
  const total = nodes.length;
  const completed = nodes.filter((n) => n.status === Status.COMPLETED).length;
  const skipped = nodes.filter((n) => n.status === Status.SKIPPED).length;
  const failed = nodes.filter((n) => n.status === Status.FAILED).length;
  const cancelled = nodes.filter((n) => n.status === Status.CANCELLED).length;
  const done = completed + skipped;
  const active = total - done - failed - cancelled;
  const barColor = failed > 0 ? "bg-destructive" : cancelled > 0 ? "bg-pending" : active === 0 ? "bg-success" : "bg-primary";

  return (
    <div className="flex h-full min-w-0 flex-1 flex-col overflow-hidden">
      {/* Progress */}
      <div className="flex shrink-0 items-center gap-3 border-b px-4 py-2.5">
        <div className="h-1 flex-1 overflow-hidden rounded-full bg-muted">
          <div className={cn("h-full rounded-full transition-all duration-700 ease-out", barColor)} style={{ width: `${total ? (done / total) * 100 : 0}%` }} />
        </div>
        <span className="flex shrink-0 items-center gap-2 font-mono text-[11px] tabular-nums text-muted-foreground">
          <span>{done}/{total}</span>
          {skipped > 0 ? <span className="text-pending">{skipped} skipped</span> : null}
          {failed > 0 ? <span className="text-destructive">{failed} failed</span> : null}
          {cancelled > 0 ? <span className="text-pending">{cancelled} cancelled</span> : null}
        </span>
      </div>

      {/* Steps */}
      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3">
        {nodes.map((node, index) => (
          <NodeRow key={node.id || node.executionId || index} node={node} depth={0} last={index === nodes.length - 1} index={index} onViewLogs={onViewLogs} />
        ))}
      </div>
    </div>
  );
}

const LOCUS_LABEL: Record<number, string> = {
  [Dag_Node_TaskState_ExecutionLocus.SERVER]: "server",
  [Dag_Node_TaskState_ExecutionLocus.AGENT]: "agent",
};

const ULID_RE = /^[0-9A-HJKMNP-TV-Z]{26}$/i;

// Readable step name: the executionId/id is a dotted address with ULID + index
// segments ("install_and_run.01KSE….0.load1.monitor_vmagent_install"). Take the
// leaf, drop ULID/index tokens, humanize → "Monitor Vmagent Install". A node
// metadata label (if present) always wins.
function cleanStepName(node: Dag_Node): string {
  const label = node.metadata?.name || node.metadata?.label;
  if (label) return label;
  const raw = node.executionId || node.id || "";
  const leaf = raw.split(".").pop() ?? raw;
  const tokens = leaf.split(/[_\s]+/).filter((t) => t && !ULID_RE.test(t) && !/^\d+$/.test(t));
  return humanize((tokens.join(" ") || leaf).trim());
}

function NodeRow({ node, depth, last, index, onViewLogs }: { node: Dag_Node; depth: number; last: boolean; index: number; onViewLogs?: (nodeExecutionId: string) => void }) {
  const style = styleOf(node.status);
  const execution = node.execution;
  const failure = execution?.failure;
  const failures = execution?.failures ?? [];
  const duration = execution?.startedAt ? formatDuration(execution.startedAt, execution.finishedAt) : "";
  const name = cleanStepName(node);

  const variant = node.variant;
  const task = variant.case === "taskState" ? variant.value : undefined;
  const subDag = variant.case === "subDag" ? variant.value : undefined;
  const dagRef = variant.case === "dagRef" ? variant.value : undefined;
  const subNodes = subDag?.nodes ?? [];

  // Inline one-line op summary (write file / run cmd / …) so the gist is readable
  // without expanding.
  const summary = useMemo(() => (task ? summarizeAny(task.input) : null), [task]);

  // Failures repeat the same message many times (retries / interrupts) — collapse
  // identical messages into one row with a ×N count.
  const groupedFailures = useMemo(() => {
    const source = failures.length ? failures : failure ? [failure] : [];
    const groups: { message: string; count: number }[] = [];
    for (const f of source) {
      const message = f.message || "";
      const existing = groups.find((g) => g.message === message);
      if (existing) existing.count += 1;
      else groups.push({ message, count: 1 });
    }
    return groups;
  }, [failures, failure]);

  const expandable = Boolean(task || dagRef || subDag || failures.length || failure?.message);
  // Structure (sub-dags) is open by default so the tree is visible without a
  // click per level; task details default closed but auto-open on failure.
  const [open, setOpen] = useState(Boolean(subDag) || Boolean(failure?.message));

  return (
    <div className="animate-rise" style={{ animationDelay: `${Math.min(index * 24, 300)}ms` }}>
      <div
        className={cn("flex items-center gap-2 rounded-sm px-1.5 py-1.5", style.tint, expandable && "cursor-pointer hover:bg-muted/30")}
        onClick={expandable ? () => setOpen((v) => !v) : undefined}
      >
        {expandable ? (
          <ChevronRight className={cn("size-3 shrink-0 text-muted-foreground transition-transform", open && "rotate-90")} />
        ) : (
          <span className="w-3 shrink-0" />
        )}
        <StepDot status={node.status} />
        <span className={cn("shrink-0 truncate text-xs font-medium", node.status === Status.PENDING ? "text-muted-foreground" : "text-foreground/90")} title={name}>
          {name}
        </span>
        {/* Inline op summary (readable without expanding) */}
        {summary ? (
          <span className="flex min-w-0 flex-1 items-center gap-1 truncate font-mono text-[10px] text-muted-foreground" title={summary.text}>
            <summary.icon className="size-3 shrink-0 text-muted-foreground/60" />
            <span className="truncate">{summary.text}</span>
          </span>
        ) : (
          <span className="min-w-0 flex-1" />
        )}
        {/* Variant tag */}
        {task ? (
          <span className="shrink-0 rounded-sm border px-1 py-px text-[9px] font-mono uppercase tracking-wide text-muted-foreground">{LOCUS_LABEL[task.locus] ?? "task"}</span>
        ) : subDag ? (
          <span className="inline-flex shrink-0 items-center gap-1 rounded-sm border border-primary/30 px-1 py-px text-[9px] font-mono uppercase tracking-wide text-primary"><Layers className="size-2.5" />subdag·{subNodes.length}</span>
        ) : dagRef ? (
          <span className="inline-flex shrink-0 items-center gap-1 rounded-sm border border-primary/30 px-1 py-px text-[9px] font-mono uppercase tracking-wide text-primary"><CornerDownRight className="size-2.5" />dagref</span>
        ) : null}
        {duration ? <span className="shrink-0 font-mono text-[10px] tabular-nums text-muted-foreground">{duration}</span> : null}
        <span className={cn("shrink-0 font-mono text-[10px] uppercase tracking-wide", style.text)}>{statusLabel(node.status)}</span>
      </div>

      {/* Expanded content */}
      {open && expandable ? (
        <div className="ml-6 mb-1.5 mt-1 space-y-2 border-l border-border/60 pl-3">
          {/* Task: handler + decoded in/out (file/command cards, else JSON) */}
          {task ? (
            <>
              <div className="flex items-center gap-1.5 text-[11px] font-mono">
                <span className="text-muted-foreground">handler</span>
                <span className="text-foreground/80">{task.handlerName || "—"}</span>
                <span className="ml-1 rounded-sm border px-1 py-px text-[9px] uppercase tracking-wide text-muted-foreground">{LOCUS_LABEL[task.locus] ?? "task"}</span>
              </div>
              {task.input ? (
                <div className="space-y-1">
                  <div className="text-[10px] font-mono uppercase tracking-wider text-muted-foreground/60">input</div>
                  <PayloadView any={task.input} />
                </div>
              ) : null}
              {task.output ? (
                <div className="space-y-1">
                  <div className="text-[10px] font-mono uppercase tracking-wider text-muted-foreground/60">output</div>
                  <PayloadView any={task.output} />
                </div>
              ) : null}
            </>
          ) : null}

          {dagRef ? (
            <div className="flex items-center gap-1 text-[11px] font-mono text-muted-foreground">
              <span>dag</span>
              <span className="truncate text-foreground/80" title={dagRef.dagId}>{dagRef.dagId}</span>
              <CopyButton value={dagRef.dagId} />
            </div>
          ) : null}

          {/* Failures — identical messages collapsed with a repeat count. */}
          {groupedFailures.map((group, gi) => (
            <div
              key={gi}
              className={cn(
                "flex items-start gap-2 whitespace-pre-wrap break-all rounded-sm border p-1.5 text-[11px] font-mono leading-relaxed",
                node.status === Status.CANCELLED ? "border-border bg-muted/40 text-muted-foreground" : "border-destructive/30 bg-destructive/5 text-destructive/90",
              )}
            >
              <span className="min-w-0 flex-1">{group.message}</span>
              {group.count > 1 ? <span className="shrink-0 rounded-sm bg-foreground/10 px-1 py-px text-[10px] tabular-nums">×{group.count}</span> : null}
            </div>
          ))}

          {onViewLogs && (task || failure?.message) ? (
            <button type="button" onClick={() => onViewLogs(node.executionId || node.id)} className="text-[10px] font-mono text-primary hover:underline">
              View in logs →
            </button>
          ) : null}

          {/* Nested sub-dag nodes (rendered inline; no per-level click) */}
          {subDag ? (
            subNodes.length > 0 ? (
              <div className="space-y-0">
                {subNodes.map((child, ci) => (
                  <NodeRow key={child.id || child.executionId || ci} node={child} depth={depth + 1} last={ci === subNodes.length - 1} index={ci} onViewLogs={onViewLogs} />
                ))}
              </div>
            ) : (
              <div className="text-[10px] font-mono text-muted-foreground/60">empty sub-dag</div>
            )
          ) : null}
        </div>
      ) : null}

      {/* Connector between top-level steps */}
      {!last && depth === 0 ? (
        <div className="flex pl-[20px]">
          <div className={cn("h-2 w-px", node.status === Status.COMPLETED ? "bg-success/30" : node.status === Status.RUNNING ? "bg-primary/30" : "bg-border")} />
        </div>
      ) : null}
    </div>
  );
}

export function RunOverview({
  run,
  dag,
  dagLoading,
  dagError,
  onViewLogs,
}: {
  run: TestRun;
  dag?: Dag;
  dagLoading?: boolean;
  dagError?: string | null;
  onViewLogs?: (nodeExecutionId: string) => void;
}) {
  return (
    <div className="flex h-[calc(100vh-15rem)] min-h-0 overflow-hidden rounded-md border bg-card">
      <ConfigColumn run={run} dag={dag} />
      <Pipeline dag={dag} loading={dagLoading} error={dagError} onViewLogs={onViewLogs} />
    </div>
  );
}
