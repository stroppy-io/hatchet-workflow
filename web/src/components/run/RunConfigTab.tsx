// Run CONFIG (Overview) tab — the full, read-only effective launch spec of a
// run: every workload segment's stroppy parameters (insert method, bulk size,
// pool/scale, env, steps, execution flags, inline SQL/files) and the complete
// database spec including free-form "advanced" option maps (postgresql.conf,
// haproxy/pgbouncer/patroni/etcd, per-engine passthroughs). Rendered so a run is
// fully reproducible / auditable without "New from run". The workload section is
// curated; the database + anything else is rendered generically from the decoded
// spec so new engines / fields show up without a UI change.
import { useState } from "react";
import { ChevronRight, Code2, Database, FileCode, Gauge, Settings2 } from "lucide-react";
import type { RunVM, WorkloadSegmentVM } from "@/services/runs";
import { cn } from "@/lib/utils";

function humanizeKey(k: string): string {
  return k
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2") // camelCase
    .replace(/[_-]+/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase())
    .trim();
}

function isPlainObject(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

// True when every value is a primitive — render as a compact key→value table.
function isFlatMap(o: Record<string, unknown>): boolean {
  return Object.values(o).every((v) => v === null || typeof v !== "object");
}

function isEmptyValue(v: unknown): boolean {
  if (v === null || v === undefined || v === "") return true;
  if (Array.isArray(v)) return v.length === 0;
  if (isPlainObject(v)) return Object.keys(v).length === 0;
  return false;
}

function Prim({ value }: { value: unknown }) {
  return <span className="break-all font-mono text-[11px] text-foreground">{String(value)}</span>;
}

// Compact label→value row (mono, dense terminal aesthetic).
function KV({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex gap-2 py-0.5 text-[11px]">
      <span className="w-40 shrink-0 truncate font-mono text-muted-foreground" title={label}>
        {label}
      </span>
      <span className="min-w-0 flex-1">{children}</span>
    </div>
  );
}

// Generic recursive renderer for an arbitrary decoded-proto value. Skips empty
// values so a spec with unset optionals stays clean.
function ConfigValue({ value }: { value: unknown }): React.ReactElement | null {
  if (isEmptyValue(value)) return null;

  if (Array.isArray(value)) {
    // Array of primitives → inline; array of objects → stacked cards.
    if (value.every((v) => v === null || typeof v !== "object")) {
      return <Prim value={value.join(", ")} />;
    }
    return (
      <div className="flex flex-col gap-2">
        {value.map((v, i) => (
          <div key={i} className="rounded border border-border/60 p-2">
            <ConfigValue value={v} />
          </div>
        ))}
      </div>
    );
  }

  if (isPlainObject(value)) {
    const entries = Object.entries(value).filter(([, v]) => !isEmptyValue(v));
    if (entries.length === 0) return null;
    return (
      <div className="flex flex-col">
        {entries.map(([k, v]) => (
          <KV key={k} label={humanizeKey(k)}>
            {isPlainObject(v) || Array.isArray(v) ? <ConfigValue value={v} /> : <Prim value={v} />}
          </KV>
        ))}
      </div>
    );
  }

  return <Prim value={value} />;
}

function Card({ icon, title, children }: { icon: React.ReactNode; title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-border bg-card/40 p-4">
      <div className="mb-3 flex items-center gap-2 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
        {icon}
        {title}
      </div>
      {children}
    </div>
  );
}

// Advanced-option maps (postgresql.conf etc.) rendered as their own key→value
// tables, so a long options blob reads like a config file rather than a blob.
function OptionsTable({ opts }: { opts: Record<string, unknown> }) {
  const entries = Object.entries(opts).filter(([, v]) => !isEmptyValue(v));
  if (entries.length === 0) return null;
  return (
    <div className="rounded border border-border/60 bg-black/20 p-2 font-mono text-[11px]">
      {entries.map(([k, v]) => (
        <div key={k} className="flex gap-2 py-px">
          <span className="shrink-0 text-primary/80">{k}</span>
          <span className="text-muted-foreground">=</span>
          <span className="break-all text-foreground">{String(v)}</span>
        </div>
      ))}
    </div>
  );
}

function SegmentCard({ seg, index }: { seg: WorkloadSegmentVM; index: number }) {
  const exec = [
    seg.vus !== undefined && `vus=${seg.vus}`,
    seg.duration ? `duration=${seg.duration}` : seg.iterations !== undefined && `iterations=${seg.iterations}`,
    seg.quiet && "quiet",
    seg.noThresholds && "no-thresholds",
  ].filter(Boolean);
  const bulk = seg.insertMethod === "plain_bulk" || (seg.bulkSize ?? 0) > 0;
  return (
    <div className="rounded border border-border/60 p-3">
      <div className="mb-2 flex items-center gap-2">
        <Gauge className="h-3.5 w-3.5 text-primary" />
        <span className="font-mono text-xs font-semibold text-foreground">{seg.name || `segment ${index + 1}`}</span>
      </div>
      {seg.script && <KV label="Script"><Prim value={seg.script} /></KV>}
      {exec.length > 0 && <KV label="Execution"><Prim value={exec.join(" · ")} /></KV>}
      <KV label="Insert method"><Prim value={seg.insertMethod || "native"} /></KV>
      {bulk && <KV label="Bulk size"><Prim value={seg.bulkSize ?? "(default)"} /></KV>}
      {seg.poolSize !== undefined && <KV label="Pool size"><Prim value={seg.poolSize} /></KV>}
      {seg.scaleFactor !== undefined && <KV label="Scale factor"><Prim value={seg.scaleFactor} /></KV>}
      {seg.noSteps && <KV label="Steps"><Prim value="(all)" /></KV>}
      {(seg.steps?.length ?? 0) > 0 && <KV label="Steps"><Prim value={seg.steps!.join(", ")} /></KV>}
      {(seg.extraArgs?.length ?? 0) > 0 && <KV label="Extra args"><Prim value={seg.extraArgs!.join(" ")} /></KV>}
      {seg.env && Object.keys(seg.env).length > 0 && (
        <KV label="Env"><OptionsTable opts={seg.env} /></KV>
      )}
      {(seg.files?.length ?? 0) > 0 && <KV label="Files"><Prim value={seg.files!.join(", ")} /></KV>}
      {seg.sql && (
        <KV label="SQL">
          <pre className="max-h-40 overflow-auto rounded border border-border/60 bg-black/30 p-2 font-mono text-[11px] text-foreground">{seg.sql}</pre>
        </KV>
      )}
    </div>
  );
}

// Pull the engine params object out of spec.database (proto oneof:
// database.source.params.<engine> or database.params.<engine>). Returns the
// engine name + its params object, or null.
function extractDbParams(db: Record<string, unknown>): { engine: string; params: Record<string, unknown> } | null {
  const paramsHolder =
    (isPlainObject(db.source) && isPlainObject((db.source as Record<string, unknown>).params)
      ? ((db.source as Record<string, unknown>).params as Record<string, unknown>)
      : undefined) ?? (isPlainObject(db.params) ? (db.params as Record<string, unknown>) : undefined);
  if (!paramsHolder) return null;
  for (const [engine, v] of Object.entries(paramsHolder)) {
    if (isPlainObject(v)) return { engine, params: v };
  }
  return null;
}

export function RunConfigTab({ run }: { run: RunVM }) {
  const [rawOpen, setRawOpen] = useState(false);

  if (!run.spec || Object.keys(run.spec).length === 0) {
    return (
      <div className="p-6 text-sm text-muted-foreground">
        No spec available for this run (list summary only — open the run detail to load the full spec).
      </div>
    );
  }

  const db = isPlainObject(run.spec.database) ? (run.spec.database as Record<string, unknown>) : undefined;
  const dbParams = db ? extractDbParams(db) : null;
  // Split the engine params into "advanced option maps" (*_options / *Options)
  // and plain scalar settings, so the config-file-like blobs render distinctly.
  const optionMaps: Array<[string, Record<string, unknown>]> = [];
  const scalarParams: Record<string, unknown> = {};
  if (dbParams) {
    for (const [k, v] of Object.entries(dbParams.params)) {
      if (isEmptyValue(v)) continue;
      if (isPlainObject(v) && isFlatMap(v) && /options$/i.test(k)) optionMaps.push([k, v]);
      else scalarParams[k] = v;
    }
  }

  return (
    <div className="flex flex-col gap-4 p-1">
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        {/* Workload */}
        <Card icon={<FileCode className="h-3.5 w-3.5" />} title="Workload">
          <div className="mb-3 flex flex-col">
            {run.workload && <KV label="Name"><Prim value={run.workload} /></KV>}
            {run.protocol && <KV label="Protocol"><Prim value={run.protocol} /></KV>}
            {run.stroppyVersion && <KV label="Stroppy"><Prim value={run.stroppyVersion} /></KV>}
            {run.workloadPresetId && <KV label="Preset"><Prim value={run.workloadPresetId} /></KV>}
          </div>
          <div className="flex flex-col gap-2">
            {run.workloadSegments.length === 0 && <span className="text-[11px] text-muted-foreground">No segments.</span>}
            {run.workloadSegments.map((seg, i) => (
              <SegmentCard key={i} seg={seg} index={i} />
            ))}
          </div>
        </Card>

        {/* Database */}
        <Card icon={<Database className="h-3.5 w-3.5" />} title="Database">
          <div className="mb-3 flex flex-col">
            <KV label="Kind"><Prim value={run.dbKind || dbParams?.engine || "—"} /></KV>
            {run.topologyLabel && <KV label="Topology"><Prim value={run.topologyLabel} /></KV>}
            {run.dbPresetId && <KV label="Preset"><Prim value={run.dbPresetId} /></KV>}
            {run.provider && <KV label="Provider"><Prim value={run.provider} /></KV>}
            {run.nodeCount > 0 && <KV label="Nodes"><Prim value={run.nodeCount} /></KV>}
          </div>
          {Object.keys(scalarParams).length > 0 && (
            <div className="mb-3 flex flex-col">
              <ConfigValue value={scalarParams} />
            </div>
          )}
          {optionMaps.length > 0 && (
            <div className="flex flex-col gap-2">
              {optionMaps.map(([k, v]) => (
                <div key={k}>
                  <div className="mb-1 flex items-center gap-1.5 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                    <Settings2 className="h-3 w-3" />
                    {humanizeKey(k)}
                  </div>
                  <OptionsTable opts={v} />
                </div>
              ))}
            </div>
          )}
          {!dbParams && (
            <span className="text-[11px] text-muted-foreground">No detailed database params in spec.</span>
          )}
        </Card>
      </div>

      {/* Full raw spec — the ultimate "everything we know" fallback. */}
      <div className="rounded-lg border border-border bg-card/40">
        <button
          type="button"
          onClick={() => setRawOpen((v) => !v)}
          className="flex w-full items-center gap-2 p-3 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground hover:text-foreground"
        >
          <ChevronRight className={cn("h-3.5 w-3.5 transition-transform", rawOpen && "rotate-90")} />
          <Code2 className="h-3.5 w-3.5" />
          Full spec (raw)
        </button>
        {rawOpen && (
          <pre className="max-h-[32rem] overflow-auto border-t border-border bg-black/30 p-3 font-mono text-[11px] leading-relaxed text-foreground">
            {JSON.stringify(run.spec, null, 2)}
          </pre>
        )}
      </div>
    </div>
  );
}
