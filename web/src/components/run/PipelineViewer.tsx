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
  Database,
  Loader2,
  Play,
  ScrollText,
  Server,
  Trash2,
  X,
  Zap,
} from "lucide-react";
import type { PipelineNodeVM } from "@/services/run_overview";
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

type FlatNode = PipelineNodeVM & { depth: number };

function flatten(nodes: PipelineNodeVM[], depth = 0, out: FlatNode[] = []): FlatNode[] {
  for (const n of nodes) {
    out.push({ ...n, depth });
    if (n.children.length) flatten(n.children, depth + 1, out);
  }
  return out;
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
        label: humanize(root.name) || "—",
        icon: iconFor(root.name),
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
          const hasSteps = band.steps.length > 0;
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
                  {band.steps.map((n) => (
                    <div
                      key={n.nodeExecutionId || n.name}
                      className={cn(
                        "group/step flex items-center gap-2 rounded-sm px-0.5 py-1",
                        bucket(n.status) === "running" && "bg-primary/[0.05]",
                      )}
                      style={{ paddingLeft: 2 + n.depth * 14 }}
                    >
                      {onOpenLogs && n.nodeExecutionId && (
                        <button
                          type="button"
                          title="View in Logs"
                          onClick={() => onOpenLogs(n.nodeExecutionId)}
                          className="shrink-0 text-muted-foreground opacity-0 transition-opacity hover:text-primary group-hover/step:opacity-100"
                        >
                          <ScrollText className="h-3 w-3" />
                        </button>
                      )}
                      <StepDot status={n.status} />
                      <span className="flex-1 truncate font-mono text-[11px] leading-tight text-foreground/80">
                        {humanize(n.name) || "—"}
                      </span>
                      {n.attempt > 1 && (
                        <span className="font-mono text-[10px] text-warning">×{n.attempt}</span>
                      )}
                      {fmtDur(n.durationSec) && (
                        <span className="font-mono text-[10px] tabular-nums text-muted-foreground">
                          {fmtDur(n.durationSec)}
                        </span>
                      )}
                    </div>
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
