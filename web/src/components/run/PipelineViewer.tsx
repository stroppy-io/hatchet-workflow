// PIPELINE VIEWER — groups the flat monitor pipeline (PipelineNodeVM tree) into
// the logical phase bands (Infrastructure / Database / … / Teardown) and renders
// each as a collapsible band with per-step status. Unknown phases fall into an
// "Other" band so nothing is dropped. Data-only port of the legacy DAG view onto
// the connect overview snapshot — no effective-config side panel (not carried by
// the new snapshot).
import { useMemo, useState } from "react";
import {
  BarChart3,
  Check,
  ChevronDown,
  Circle,
  Copy,
  Database,
  Download,
  FileText,
  FolderPlus,
  Loader2,
  Play,
  ScrollText,
  Server,
  Terminal,
  Trash2,
  X,
  Zap,
} from "lucide-react";
import type { OperationVM, PipelineNodeVM, PipelineOutputVM } from "@/services/run_overview";
import type { RunStatus } from "@/services/dashboard";
import { cn } from "@/lib/utils";

// Icon heuristic by node name — the connect pipeline is a free-form tree
// (Infrastructure / Render Deployment Plan / Postgres Master / Workload …),
// not the old fixed phase ids, so we pick an icon from keywords.
function iconFor(name: string): typeof Zap {
  const s = name.toLowerCase();
  if (/(infra|network|machine|provision)/.test(s)) return Server;
  if (/(teardown|cleanup|destroy|undeploy)/.test(s)) return Trash2;
  if (/(monitor|metric|observ)/.test(s)) return BarChart3;
  if (/(workload|stroppy|benchmark|runner|run\b)/.test(s)) return Play;
  if (/(postgres|mysql|ydb|picodata|etcd|database|\bdb\b|master|replica|proxy|pgbouncer)/.test(s))
    return Database;
  if (/(deploy|render|execute|plan)/.test(s)) return Zap;
  return Circle;
}

// Operation-kind glyph — lets the UI show what an agent step actually does.
function opIcon(kind: OperationVM["kind"]): typeof Zap {
  switch (kind) {
    case "create_dir": return FolderPlus;
    case "write_file": return FileText;
    case "fetch_file": return Download;
    case "call_cmd": return Terminal;
    default: return Circle;
  }
}

type FlatNode = PipelineNodeVM & { depth: number };

// Flatten the node tree depth-first, ordering siblings by their stable `order`.
function flatten(nodes: PipelineNodeVM[], depth = 0, out: FlatNode[] = []): FlatNode[] {
  const sorted = [...nodes].sort((a, b) => (a.order || 0) - (b.order || 0));
  for (const n of sorted) {
    out.push({ ...n, depth });
    if (n.children.length) flatten(n.children, depth + 1, out);
  }
  return out;
}

function fmtBytes(n: number): string {
  if (!n) return "";
  if (n >= 1 << 20) return `${(n / (1 << 20)).toFixed(1)} MiB`;
  if (n >= 1 << 10) return `${(n / (1 << 10)).toFixed(1)} KiB`;
  return `${n} B`;
}

function humanize(name: string): string {
  return name.replace(/[_-]+/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

function fmtDur(sec?: number): string {
  if (sec === undefined || !Number.isFinite(sec) || sec <= 0) return "";
  if (sec < 60) return `${Math.round(sec)}s`;
  return `${Math.floor(sec / 60)}m ${Math.round(sec % 60)}s`;
}

// Collapse the lifecycle union into the four rendering buckets.
function bucket(s: RunStatus): "done" | "running" | "failed" | "cancelled" | "pending" {
  if (s === "completed") return "done";
  if (s === "running") return "running";
  if (s === "failed") return "failed";
  if (s === "cancelled" || s === "cancelling") return "cancelled";
  return "pending";
}

const BAND_STYLE = {
  done: { border: "border-success/25", bg: "bg-success/[0.03]", text: "text-success" },
  running: { border: "border-primary/30", bg: "bg-primary/[0.04]", text: "text-primary" },
  failed: { border: "border-destructive/25", bg: "bg-destructive/[0.03]", text: "text-destructive" },
  cancelled: { border: "border-border", bg: "bg-pending/[0.03]", text: "text-pending" },
  pending: { border: "border-border/60", bg: "bg-transparent", text: "text-muted-foreground/60" },
} as const;

function StepDot({ status }: { status: RunStatus }) {
  const b = bucket(status);
  if (b === "done")
    return (
      <span className="flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full bg-success">
        <Check className="h-2 w-2 text-background" strokeWidth={3} />
      </span>
    );
  if (b === "failed")
    return (
      <span className="flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full bg-destructive">
        <X className="h-2 w-2 text-background" strokeWidth={3} />
      </span>
    );
  if (b === "cancelled")
    return (
      <span className="flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full bg-pending">
        <X className="h-2 w-2 text-background" strokeWidth={3} />
      </span>
    );
  if (b === "running")
    return (
      <span className="flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full bg-primary">
        <Loader2 className="h-2 w-2 animate-spin text-background" strokeWidth={3} />
      </span>
    );
  return (
    <span className="flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full border border-border bg-muted">
      <Circle className="h-1 w-1 text-muted-foreground" fill="currentColor" />
    </span>
  );
}

function groupBucket(nodes: { status: RunStatus }[]): keyof typeof BAND_STYLE {
  const bs = nodes.map((n) => bucket(n.status));
  if (bs.length === 0) return "pending";
  if (bs.some((b) => b === "failed")) return "failed";
  if (bs.some((b) => b === "cancelled")) return "cancelled";
  if (bs.every((b) => b === "done")) return "done";
  if (bs.some((b) => b === "running" || b === "done")) return "running";
  return "pending";
}

export function PipelineViewer({
  pipeline,
  onOpenLogs,
}: {
  pipeline: PipelineNodeVM[];
  onOpenLogs?: (nodeExecutionId: string) => void;
}) {
  const flat = useMemo(() => flatten(pipeline), [pipeline]);

  // One band per top-level root; its subtree (flattened, depth-relative to the
  // root's children) is the band's steps.
  const bands = useMemo(
    () =>
      pipeline.map((root) => ({
        id: root.nodeExecutionId || root.name,
        label: humanize(root.phase || root.name) || "—",
        icon: iconFor(root.phase || root.name),
        root,
        steps: flatten(root.children),
      })),
    [pipeline],
  );

  // Auto-expand bands that contain an active (running/failed) node; collapse
  // the rest. The set is seeded once per distinct band signature.
  const autoOpen = useMemo(() => {
    const s = new Set<string>();
    for (const band of bands) {
      const subtree = [band.root, ...band.steps];
      if (subtree.some((n) => bucket(n.status) === "running" || bucket(n.status) === "failed")) {
        s.add(band.id);
      }
    }
    return s;
  }, [bands]);
  const [overrides, setOverrides] = useState<Map<string, boolean>>(() => new Map());
  const isOpenBand = (id: string) => overrides.get(id) ?? autoOpen.has(id);

  // Per-step expansion (operation / error detail).
  const [openSteps, setOpenSteps] = useState<Set<string>>(() => new Set());
  const toggleStep = (id: string) =>
    setOpenSteps((prev) => {
      const next = new Set(prev);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });

  const total = flat.length;
  const done = flat.filter((n) => bucket(n.status) === "done").length;
  const failed = flat.filter((n) => bucket(n.status) === "failed").length;

  const toggle = (id: string) =>
    setOverrides((prev) => {
      const next = new Map(prev);
      next.set(id, !isOpenBand(id));
      return next;
    });

  if (total === 0) {
    return (
      <div className="flex h-full items-center justify-center gap-2 py-12">
        <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
        <span className="font-mono text-xs text-muted-foreground">Waiting for pipeline steps…</span>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      {/* Progress */}
      <div className="flex shrink-0 items-center gap-3 border-b border-border/60 px-3 py-2">
        <div className="h-1 flex-1 overflow-hidden rounded-full bg-muted">
          <div
            className={cn(
              "h-full rounded-full transition-all duration-700",
              failed > 0 ? "bg-destructive" : done === total ? "bg-success" : "bg-primary",
            )}
            style={{ width: `${total > 0 ? (done / total) * 100 : 0}%` }}
          />
        </div>
        <span className="shrink-0 font-mono text-[11px] tabular-nums text-muted-foreground">
          {done}/{total}
          {failed > 0 && <span className="ml-1 text-destructive">{failed} err</span>}
        </span>
      </div>

      {/* Bands */}
      <div className="min-h-0 flex-1 space-y-1.5 overflow-y-auto p-3">
        {bands.map((band) => {
          const subtree = [band.root, ...band.steps];
          const b = groupBucket(subtree);
          const style = BAND_STYLE[b];
          const Icon = band.icon;
          const isOpen = isOpenBand(band.id);
          const hasSteps = band.steps.length > 0 || band.root.outputs.length > 0;
          const groupDone = subtree.filter((n) => bucket(n.status) === "done").length;
          return (
            <div key={band.id} className={cn("border transition-colors", style.border, style.bg)}>
              <button
                type="button"
                onClick={() => hasSteps && toggle(band.id)}
                className={cn(
                  "group/band flex w-full items-center gap-2 px-2.5 py-1.5 text-left transition-colors",
                  hasSteps ? "hover:bg-foreground/[0.02]" : "cursor-default",
                )}
              >
                {onOpenLogs && band.root.nodeExecutionId && (
                  <span
                    role="button"
                    tabIndex={0}
                    title="View in Logs"
                    onClick={(e) => {
                      e.stopPropagation();
                      onOpenLogs(band.root.nodeExecutionId);
                    }}
                    className="shrink-0 text-muted-foreground opacity-0 transition-opacity hover:text-primary group-hover/band:opacity-100"
                  >
                    <ScrollText className="h-3 w-3" />
                  </span>
                )}
                <StepDot status={band.root.status} />
                <Icon className={cn("h-3.5 w-3.5 shrink-0", style.text)} />
                <span className="flex-1 truncate font-mono text-xs font-semibold text-foreground/90">
                  {band.label}
                </span>
                {fmtDur(band.root.durationSec) && (
                  <span className="font-mono text-[10px] tabular-nums text-muted-foreground">
                    {fmtDur(band.root.durationSec)}
                  </span>
                )}
                {hasSteps && (
                  <>
                    <span className="font-mono text-[10px] tabular-nums text-muted-foreground">
                      {groupDone}/{subtree.length}
                    </span>
                    <ChevronDown
                      className={cn(
                        "h-3 w-3 text-muted-foreground transition-transform duration-200",
                        isOpen && "rotate-180",
                      )}
                    />
                  </>
                )}
              </button>
              {isOpen && hasSteps && (
                <div className="space-y-px border-t border-border/40 px-2.5 py-1">
                  {band.root.outputs.length > 0 && (
                    <div className="mb-1 border-b border-border/30 pb-1">
                      <OutputsDetail outputs={band.root.outputs} />
                    </div>
                  )}
                  {band.steps.map((n) => (
                    <StepRow
                      key={n.nodeExecutionId || n.name}
                      node={n}
                      expanded={openSteps.has(n.nodeExecutionId || n.name)}
                      onToggle={() => toggleStep(n.nodeExecutionId || n.name)}
                      onOpenLogs={onOpenLogs}
                    />
                  ))}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

// ── Step row ────────────────────────────────────────────────────────────────

function StepRow({
  node,
  expanded,
  onToggle,
  onOpenLogs,
}: {
  node: FlatNode;
  expanded: boolean;
  onToggle: () => void;
  onOpenLogs?: (nodeExecutionId: string) => void;
}) {
  const op = node.operation;
  const hasDetail = !!op || !!node.errorMessage || node.outputs.length > 0;
  const label = op?.summary || humanize(node.name) || "—";
  const OpIcon = op ? opIcon(op.kind) : null;
  const isErr = bucket(node.status) === "failed";

  return (
    <div style={{ paddingLeft: 2 + node.depth * 14 }}>
      <div
        className={cn(
          "group/step flex items-center gap-2 rounded-sm px-0.5 py-1",
          bucket(node.status) === "running" && "bg-primary/[0.05]",
          hasDetail && "cursor-pointer hover:bg-foreground/[0.03]",
        )}
        onClick={hasDetail ? onToggle : undefined}
      >
        {onOpenLogs && node.nodeExecutionId && (
          <button
            type="button"
            title="View in Logs"
            onClick={(e) => {
              e.stopPropagation();
              onOpenLogs(node.nodeExecutionId);
            }}
            className="shrink-0 text-muted-foreground opacity-0 transition-opacity hover:text-primary group-hover/step:opacity-100"
          >
            <ScrollText className="h-3 w-3" />
          </button>
        )}
        <StepDot status={node.status} />
        {OpIcon && <OpIcon className="h-3 w-3 shrink-0 text-muted-foreground" />}
        <span
          className={cn(
            "min-w-0 flex-1 truncate font-mono text-[11px] leading-tight",
            isErr ? "text-destructive" : "text-foreground/80",
          )}
          title={op?.target || label}
        >
          {label}
        </span>
        {op?.mentions?.slice(0, 3).map((m) => (
          <span key={m} className="hidden shrink-0 rounded-sm border border-border px-1 font-mono text-[9px] text-muted-foreground md:inline">
            {m}
          </span>
        ))}
        {node.machineId && (
          <span className="hidden shrink-0 font-mono text-[9px] text-muted-foreground lg:inline" title={`machine: ${node.machineId}`}>
            {node.machineId.length > 14 ? `…${node.machineId.slice(-14)}` : node.machineId}
          </span>
        )}
        {node.attempt > 1 && <span className="shrink-0 font-mono text-[10px] text-warning">×{node.attempt}</span>}
        {fmtDur(node.durationSec) && (
          <span className="shrink-0 font-mono text-[10px] tabular-nums text-muted-foreground">{fmtDur(node.durationSec)}</span>
        )}
        {hasDetail && (
          <ChevronDown className={cn("h-3 w-3 shrink-0 text-muted-foreground transition-transform", expanded && "rotate-180")} />
        )}
      </div>

      {expanded && hasDetail && (
        <div className="mb-1 ml-5 space-y-1.5 border-l border-border/60 pl-3 pt-1">
          {node.statusReason && (
            <div className="font-mono text-[10px] text-muted-foreground">{node.statusReason}</div>
          )}
          {node.errorMessage && (
            <div className="space-y-1">
              <pre className="whitespace-pre-wrap break-all border border-destructive/30 bg-destructive/5 p-1.5 font-mono text-[10px] leading-relaxed text-destructive/90">
                {node.errorMessage}
              </pre>
              <CopyBtn text={node.errorMessage} label="copy error" />
            </div>
          )}
          {op && <OperationDetail op={op} />}
          {node.outputs.length > 0 && <OutputsDetail outputs={node.outputs} />}
        </div>
      )}
    </div>
  );
}

function OperationDetail({ op }: { op: OperationVM }) {
  return (
    <div className="space-y-1.5">
      {(op.stepId || op.stepOrder > 0) && (
        <div className="flex gap-2 font-mono text-[10px] text-muted-foreground">
          {op.stepId && <span>step: {op.stepId}</span>}
          {op.stepOrder > 0 && <span>#{op.stepOrder}</span>}
        </div>
      )}

      {/* CALL_CMD */}
      {op.kind === "call_cmd" && (op.commandText || op.argv.length > 0) && (
        <div className="space-y-1">
          <pre className="max-h-48 overflow-auto whitespace-pre-wrap break-all border border-border bg-black/40 p-1.5 font-mono text-[10px] leading-relaxed text-foreground/85">
            {op.commandText || op.argv.join(" ")}
          </pre>
          <CopyBtn text={op.commandText || op.argv.join(" ")} label="copy command" />
        </div>
      )}

      {op.resultAvailable && (
        <div className="space-y-1">
          <div className="flex flex-wrap items-center gap-2 font-mono text-[10px] text-muted-foreground">
            <span className={op.exitCode === 0 && !op.timedOut ? "text-success" : "text-destructive"}>
              exit {op.exitCode}
            </span>
            {op.timedOut && <span className="text-warning">timed out</span>}
            {fmtDur(op.elapsedSec) && <span>{fmtDur(op.elapsedSec)}</span>}
            {op.resultSummary && <span>{op.resultSummary}</span>}
          </div>
          {(op.stdoutPreview || op.stderrPreview) && (
            <pre className="max-h-48 overflow-auto whitespace-pre-wrap break-all border border-border bg-black/40 p-1.5 font-mono text-[10px] leading-relaxed text-foreground/75">
              {op.stdoutPreview}
              {op.stdoutPreview && op.stderrPreview ? "\n" : ""}
              {op.stderrPreview ? `stderr:\n${op.stderrPreview}` : ""}
            </pre>
          )}
        </div>
      )}

      {/* WRITE_FILE / FETCH_FILE */}
      {(op.kind === "write_file" || op.kind === "fetch_file") && (
        <div className="space-y-1">
          <div className="flex flex-wrap items-center gap-2 font-mono text-[10px] text-muted-foreground">
            {op.filePath && <span className="break-all text-foreground/80">{op.filePath}</span>}
            {fmtBytes(op.fileSizeBytes) && <span>· {fmtBytes(op.fileSizeBytes)}</span>}
          </div>
          {op.contentPreview && (
            <pre className="max-h-48 overflow-auto whitespace-pre-wrap break-all border border-border bg-black/40 p-1.5 font-mono text-[10px] leading-relaxed text-foreground/75">
              {op.contentPreview}
            </pre>
          )}
        </div>
      )}

      {/* CREATE_DIR */}
      {op.kind === "create_dir" && op.target && (
        <div className="font-mono text-[10px] text-foreground/80">{op.target}</div>
      )}

      {op.mentions.length > 0 && (
        <div className="flex flex-wrap gap-1">
          {op.mentions.map((m) => (
            <span key={m} className="rounded-sm border border-border px-1 font-mono text-[9px] text-muted-foreground">{m}</span>
          ))}
        </div>
      )}
    </div>
  );
}

function OutputsDetail({ outputs }: { outputs: PipelineOutputVM[] }) {
  if (outputs.length === 0) return null;
  return (
    <div className="space-y-1">
      {outputs.slice(0, 120).map((out) => (
        <div key={out.id || `${out.kind}-${out.name}-${out.target}`} className="space-y-1 rounded-sm border border-border/60 bg-black/20 p-1.5">
          <div className="flex flex-wrap items-center gap-2 font-mono text-[10px]">
            <span className="text-muted-foreground">{out.kind || "output"}</span>
            <span className="text-foreground/85">{out.name || out.id || out.target || "output"}</span>
            {out.componentId && <span className="text-muted-foreground">{out.componentId}</span>}
            {out.action && <span className="text-muted-foreground">{out.action}</span>}
            {out.count > 0 && <span className="text-muted-foreground">count {out.count}</span>}
            {fmtBytes(out.sizeBytes) && <span className="text-muted-foreground">{fmtBytes(out.sizeBytes)}</span>}
          </div>
          {out.summary && <div className="font-mono text-[10px] text-foreground/75">{out.summary}</div>}
          {out.target && <div className="break-all font-mono text-[10px] text-muted-foreground">{out.target}</div>}
          {(out.commandText || out.contentPreview) && (
            <pre className="max-h-32 overflow-auto whitespace-pre-wrap break-all border border-border bg-black/40 p-1.5 font-mono text-[10px] leading-relaxed text-foreground/70">
              {out.commandText || out.contentPreview}
            </pre>
          )}
        </div>
      ))}
      {outputs.length > 120 && (
        <div className="font-mono text-[10px] text-muted-foreground">+{outputs.length - 120} more outputs</div>
      )}
    </div>
  );
}

function CopyBtn({ text, label }: { text: string; label: string }) {
  const [done, setDone] = useState(false);
  return (
    <button
      type="button"
      onClick={(e) => {
        e.stopPropagation();
        void navigator.clipboard.writeText(text).then(() => {
          setDone(true);
          window.setTimeout(() => setDone(false), 1200);
        });
      }}
      className="inline-flex items-center gap-1 font-mono text-[9px] text-muted-foreground hover:text-foreground"
    >
      {done ? <Check className="h-2.5 w-2.5 text-success" /> : <Copy className="h-2.5 w-2.5" />}
      {done ? "copied" : label}
    </button>
  );
}
