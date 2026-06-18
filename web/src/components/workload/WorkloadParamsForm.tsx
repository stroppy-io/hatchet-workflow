// Shared typed-workload editor — the cloud.v1.domain.Workload form fields the
// Test Wizard's Workload step (NewRun.tsx pane 3) and the Workload Preset
// authoring pages (WorkloadPresetForm / WorkloadPresetDetail) both render.
//
// It mirrors components/database/DatabaseParamsForm.tsx: a single source of
// truth for the workload fields (script / sql / protocol, the k6 Execution
// vus + duration|iterations + quiet/noThresholds, and the data Parameters
// pool/scale/insert/env/steps), PLUS the domain.Workload.Validate mirror
// (validateWorkload) so every surface validates identically to the backend.
//
// The editor is a pure controlled component: it takes a WorkloadVM + an apply
// callback and emits the next WorkloadVM. stroppy_version is owned elsewhere
// (the wizard's Version pane / the preset form's Identity section), so it is
// NOT edited here.

import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { AlertCircle, Check, ChevronDown, ChevronUp, Loader2, Plus, RotateCcw, Trash2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import { ConfigEditor } from "@/components/ui/config-editor";
import { ToggleRow, FieldErrors, errorsFor } from "@/components/database/DatabaseParamsForm";
import { NumField, ScrubHandle } from "@/components/ui/num-field";
import {
  Workload_Protocol,
  driverTypeFor,
  defaultProtocolFor,
  getWizardProvider,
  defaultSegment,
  type EngineKind,
  type WorkloadVM,
  type WorkloadSegmentVM,
  type DraftErrorVM,
  type ProbeMetaVM,
} from "@/services/wizard";

/**
 * Optional probe wiring. When present (the wizard's Workload step), each segment
 * editor runs its own debounced `stroppy probe` for its script and surfaces the
 * declared phases/env as first-class controls. Absent on the preset authoring
 * pages, which fall back to free-text editors.
 */
export interface SegmentProbeContext {
  slug: string;
  engine: EngineKind;
  /** The run's shared stroppy version (from the wizard Version pane). */
  version: string;
}

/** Env vars already surfaced by dedicated controls — hidden from the generic env list. */
const COVERED_ENV = new Set(["POOL_SIZE", "SCALE_FACTOR", "WAREHOUSES", "STROPPY_STEPS", "STROPPY_NO_STEPS"]);

/** Driver-level insert methods (mirrors stroppy run.proto default_insert_method). */
const INSERT_METHODS = ["native", "plain_bulk", "plain_query"] as const;

/** Numbers (incl. negatives/decimals) vs free text like "max"/"false". */
const NUMERIC_RE = /^-?\d+(\.\d+)?$/;

/** Set or clear a single env key (empty value clears it, falling back to the script default). */
function setEnvKey(env: Record<string, string>, name: string, value: string): Record<string, string> {
  const next = { ...env };
  if (value === "") delete next[name];
  else next[name] = value;
  return next;
}

/** The wire protocols the Protocol picker offers (mirrors NewRun's PROTOCOLS). */
export const PROTOCOLS: { v: Workload_Protocol; label: string }[] = [
  { v: Workload_Protocol.PG, label: "PostgreSQL wire (pg)" },
  { v: Workload_Protocol.MYSQL, label: "MySQL" },
  { v: Workload_Protocol.PICODATA, label: "Picodata" },
  { v: Workload_Protocol.YDB_GRPC, label: "YDB gRPC" },
  { v: Workload_Protocol.YDB_GRPCS, label: "YDB gRPC (TLS)" },
  { v: Workload_Protocol.COCKROACH, label: "CockroachDB" },
];

/**
 * Mirror cloud.v1.domain.Workload.Validate (the same rules the wizard's
 * mock/wizard.ts validate() applies to the workload slice), surfaced as the
 * shared DraftError shape so the preset form can block save on errors exactly
 * like the wizard blocks readiness.
 */
export function validateWorkload(w: WorkloadVM | null): DraftErrorVM[] {
  const errs: DraftErrorVM[] = [];
  const err = (
    field: string,
    message: string,
    severity: DraftErrorVM["severity"] = "error",
    code = "invalid",
  ) => errs.push({ field, message, severity, code });

  if (!w) {
    err("workload", "Configure the workload.");
    return errs;
  }
  if (w.segments.length === 0) {
    err("workload.segments", "Add at least one segment.", "error", "required");
    return errs;
  }
  w.segments.forEach((seg, i) => {
    const f = (suffix: string) => `workload.segments[${i}].${suffix}`;
    if (!seg.name.trim()) err(f("name"), "Segment needs a name.", "error", "required");
    if (!seg.script.trim()) err(f("script"), "A segment needs a script.", "error", "required");
    if (seg.execution.vus < 1) err(f("execution.vus"), "At least 1 virtual user.", "error", "min");
    if (seg.execution.limit.case === "duration" && !seg.execution.limit.duration.trim())
      err(f("execution.duration"), "Set a run duration.", "error", "required");
    if (seg.execution.limit.case === "iterations" && seg.execution.limit.iterations < 1)
      err(f("execution.iterations"), "At least 1 iteration.", "error", "min");
    if (seg.parameters.steps.length > 0 && seg.parameters.noSteps.length > 0)
      err(f("parameters.steps"), "steps and no_steps are mutually exclusive.", "error", "exclusive");
  });
  return errs;
}

/**
 * The typed-workload editor: a shared protocol picker plus an accordion of
 * segment cards. Each segment is a self-contained stroppy invocation (script +
 * k6 execution + data params); they run sequentially on the same DB. The common
 * shape is a bootstrap segment followed by the measured workload.
 */
export function WorkloadParamsForm({
  w,
  apply,
  disabled,
  advancedInitiallyOpen = disabled ?? false,
  engine,
  probeContext = null,
}: {
  w: WorkloadVM;
  apply: (w: WorkloadVM) => void;
  disabled?: boolean;
  advancedInitiallyOpen?: boolean;
  /**
   * The selected database engine, when authored inside a run (NewRun). A builtin
   * engine speaks exactly one protocol, so the picker is locked to it — the
   * protocol decides the connection port/db (CockroachDB → 26257/defaultdb, not
   * PG's 5432). Absent on the engine-agnostic preset authoring pages, where the
   * picker stays free; "external" also keeps it free (no canonical protocol).
   */
  engine?: EngineKind;
  /**
   * When present, each segment editor runs its own live `stroppy probe` for its
   * script and surfaces the declared phases/env as first-class controls. Absent
   * on the preset authoring pages, which fall back to the free-text editors.
   */
  probeContext?: SegmentProbeContext | null;
}) {
  const protocolLocked = !!engine && engine !== "external";
  const setSegment = (index: number, seg: WorkloadSegmentVM) =>
    apply({ ...w, segments: w.segments.map((s, i) => (i === index ? seg : s)) });
  const addSegment = () =>
    apply({ ...w, segments: [...w.segments, defaultSegment(`segment ${w.segments.length + 1}`)] });
  const removeSegment = (index: number) => {
    if (w.segments.length <= 1) return; // always keep at least one segment
    apply({ ...w, segments: w.segments.filter((_, i) => i !== index) });
  };
  const moveSegment = (index: number, dir: -1 | 1) => {
    const target = index + dir;
    if (target < 0 || target >= w.segments.length) return;
    const next = [...w.segments];
    [next[index], next[target]] = [next[target], next[index]];
    apply({ ...w, segments: next });
  };

  return (
    <div className={disabled ? "pointer-events-none select-none opacity-90" : undefined}>
      <div className="@container space-y-4">
        <div className="grid grid-cols-1 gap-3 @lg:grid-cols-2">
          <div>
            <Label>Protocol</Label>
            <p className="mb-1 mt-0.5 text-[11px] text-zinc-600">
              {protocolLocked
                ? "Determined by the selected database engine."
                : "Shared across segments — they all target the one database."}
            </p>
            {protocolLocked ? (
              <div className="mt-1 flex h-9 items-center border border-zinc-800 bg-zinc-900/40 px-3 text-[13px] text-zinc-300">
                {PROTOCOLS.find((p) => p.v === defaultProtocolFor(engine!))?.label ??
                  Workload_Protocol[w.protocol]}
              </div>
            ) : (
              <Select
                value={String(w.protocol)}
                onValueChange={(v) => apply({ ...w, protocol: Number(v) as Workload_Protocol })}
              >
                <SelectTrigger className="mt-1">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {PROTOCOLS.map((p) => (
                    <SelectItem key={p.v} value={String(p.v)}>
                      {p.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>
        </div>

        <div className="space-y-3">
          {w.segments.map((seg, i) => (
            <SegmentEditor
              key={i}
              index={i}
              total={w.segments.length}
              segment={seg}
              onChange={(s) => setSegment(i, s)}
              onRemove={() => removeSegment(i)}
              onMove={(dir) => moveSegment(i, dir)}
              advancedInitiallyOpen={advancedInitiallyOpen}
              probeContext={probeContext}
            />
          ))}
        </div>

        {!disabled && (
          <button
            type="button"
            onClick={addSegment}
            className="flex w-full items-center justify-center gap-2 border border-dashed border-zinc-800 py-2.5 text-[12px] text-zinc-500 transition-colors hover:border-zinc-700 hover:text-zinc-300"
          >
            <Plus className="h-3.5 w-3.5" /> Add segment
          </button>
        )}
      </div>
    </div>
  );
}

/** Concise one-line summary of a segment for the collapsed accordion header. */
function segmentSummary(seg: WorkloadSegmentVM): string {
  const limit =
    seg.execution.limit.case === "duration"
      ? seg.execution.limit.duration || "—"
      : `${seg.execution.limit.iterations} iters`;
  const phases = seg.parameters.steps.length > 0 ? ` · ${seg.parameters.steps.join("+")}` : "";
  return `${seg.script || "no script"} · ${seg.execution.vus} vus · ${limit}${phases}`;
}

/**
 * One collapsible segment card: an editable name + the script/execution/data
 * fields, with its own optional probe (declared phases/env + status disclosure).
 */
function SegmentEditor({
  index,
  total,
  segment: seg,
  onChange,
  onRemove,
  onMove,
  advancedInitiallyOpen,
  probeContext,
}: {
  index: number;
  total: number;
  segment: WorkloadSegmentVM;
  onChange: (seg: WorkloadSegmentVM) => void;
  onRemove: () => void;
  onMove: (dir: -1 | 1) => void;
  advancedInitiallyOpen: boolean;
  probeContext: SegmentProbeContext | null;
}) {
  const [open, setOpen] = useState(total <= 2);
  const limit = seg.execution.limit;

  // --- Per-segment probe: fires once script + version are present, debounced,
  // re-firing when the script/sql/version/scale/pool change.
  const [probe, setProbe] = useState<ProbeMetaVM | null>(null);
  const [probing, setProbing] = useState(false);
  const [probeErr, setProbeErr] = useState<string | null>(null);
  const version = probeContext?.version ?? "";
  const canProbe = !!probeContext && !!seg.script.trim() && !!version.trim();
  const probeKey = `${version}|${seg.script}|${seg.sql}|${probeContext?.engine}|${seg.parameters.poolSize}|${seg.parameters.scaleFactor}`;
  useEffect(() => {
    if (!probeContext || !canProbe) {
      setProbe(null);
      setProbeErr(null);
      return;
    }
    let cancelled = false;
    setProbing(true);
    setProbeErr(null);
    const timer = window.setTimeout(() => {
      getWizardProvider()
        .probe(probeContext.slug, {
          version,
          script: seg.script.trim(),
          sql: seg.sql,
          driverType: driverTypeFor(probeContext.engine),
          poolSize: seg.parameters.poolSize,
          scaleFactor: seg.parameters.scaleFactor,
          includeHuman: true,
        })
        .then((meta) => {
          if (cancelled) return;
          setProbe(meta);
          setProbeErr(null);
        })
        .catch((e) => {
          if (cancelled) return;
          setProbe(null);
          setProbeErr(e instanceof Error ? e.message : String(e));
        })
        .finally(() => {
          if (!cancelled) setProbing(false);
        });
    }, 350);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
    // probeKey captures every dependency; slug/engine are stable per step.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [probeKey, canProbe]);

  const declaredEnv = useMemo(
    () => (probe?.env ?? []).filter((d) => !d.names.some((n) => COVERED_ENV.has(n))),
    [probe],
  );
  const declaredNames = useMemo(() => new Set(declaredEnv.flatMap((d) => d.names)), [declaredEnv]);
  const probeSteps = probe?.steps ?? [];

  const setExec = (patch: Partial<WorkloadSegmentVM["execution"]>) =>
    onChange({ ...seg, execution: { ...seg.execution, ...patch } });
  const setParams = (patch: Partial<WorkloadSegmentVM["parameters"]>) =>
    onChange({ ...seg, parameters: { ...seg.parameters, ...patch } });
  const setEnv = (name: string, value: string) =>
    setParams({ env: setEnvKey(seg.parameters.env, name, value) });
  const togglePhase = (phase: string) => {
    const has = seg.parameters.steps.includes(phase);
    const steps = has ? seg.parameters.steps.filter((s) => s !== phase) : [...seg.parameters.steps, phase];
    setParams({ steps, noSteps: [] });
  };

  // Phases are script-specific. When the probe reports a new set (e.g. after the
  // script changes), drop any selected steps/no_steps the new script no longer
  // exposes — otherwise stale phases from the previous script are sent and
  // stroppy rejects them with "unknown steps".
  useEffect(() => {
    if (probeSteps.length === 0) return; // no probe result yet — leave as-is
    const valid = new Set(probeSteps);
    const steps = seg.parameters.steps.filter((s) => valid.has(s));
    const noSteps = seg.parameters.noSteps.filter((s) => valid.has(s));
    if (steps.length !== seg.parameters.steps.length || noSteps.length !== seg.parameters.noSteps.length) {
      setParams({ steps, noSteps });
    }
    // Re-run only when the available phase set changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [probeSteps.join(" ")]);

  return (
    <div className="border border-zinc-800/70 bg-[#0a0a0a]">
      <div className="flex items-center gap-2 px-3 py-2">
        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          className="text-zinc-500 transition-colors hover:text-zinc-300"
          aria-expanded={open}
        >
          <ChevronDown className={`h-4 w-4 transition-transform ${open ? "rotate-180" : ""}`} />
        </button>
        <Input
          className="h-7 w-40 font-mono text-xs"
          value={seg.name}
          onChange={(e) => onChange({ ...seg, name: e.target.value })}
          placeholder={`segment ${index + 1}`}
        />
        {!open && (
          <span className="truncate font-mono text-[11px] text-zinc-600">{segmentSummary(seg)}</span>
        )}
        <div className="ml-auto flex items-center gap-0.5">
          <button
            type="button"
            onClick={() => onMove(-1)}
            disabled={index === 0}
            title="Move up"
            className="p-1 text-zinc-600 transition-colors hover:text-zinc-300 disabled:opacity-25"
          >
            <ChevronUp className="h-3.5 w-3.5" />
          </button>
          <button
            type="button"
            onClick={() => onMove(1)}
            disabled={index === total - 1}
            title="Move down"
            className="p-1 text-zinc-600 transition-colors hover:text-zinc-300 disabled:opacity-25"
          >
            <ChevronDown className="h-3.5 w-3.5" />
          </button>
          <button
            type="button"
            onClick={onRemove}
            disabled={total <= 1}
            title="Remove segment"
            className="p-1 text-zinc-600 transition-colors hover:text-red-400 disabled:opacity-25"
          >
            <Trash2 className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>

      {open && (
        <div className="@container space-y-5 border-t border-zinc-800/70 p-4">
          <div className="grid grid-cols-1 gap-3 @lg:grid-cols-2">
            <div>
              <Label>Script</Label>
              <Input
                className="mt-1 font-mono text-xs"
                value={seg.script}
                onChange={(e) => onChange({ ...seg, script: e.target.value })}
                placeholder="tpcc/tx"
              />
            </div>
            <div>
              <Label>SQL arg (optional)</Label>
              <Input
                className="mt-1 font-mono text-xs"
                value={seg.sql}
                onChange={(e) => onChange({ ...seg, sql: e.target.value })}
                placeholder="queries.sql"
              />
            </div>
          </div>

          <div className="border border-zinc-800/60 bg-[#070707] p-4">
            <div className="mb-3 text-[10px] font-mono uppercase tracking-wider text-zinc-600">k6 execution</div>
            <div className="grid grid-cols-1 gap-3 @lg:grid-cols-2 @3xl:grid-cols-3">
              <NumField
                label="Virtual users"
                value={seg.execution.vus}
                onChange={(n) => setExec({ vus: n })}
                min={1}
              />
              <div>
                <Label>Limit by</Label>
                <Select
                  value={limit.case}
                  onValueChange={(v) =>
                    setExec({
                      limit:
                        v === "duration"
                          ? { case: "duration", duration: "5m" }
                          : { case: "iterations", iterations: 10000 },
                    })
                  }
                >
                  <SelectTrigger className="mt-1">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="duration">Duration</SelectItem>
                    <SelectItem value="iterations">Iterations</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              {limit.case === "duration" ? (
                <div>
                  <Label>Duration</Label>
                  <Input
                    className="mt-1"
                    value={limit.duration}
                    onChange={(e) => setExec({ limit: { case: "duration", duration: e.target.value } })}
                    placeholder="10m"
                  />
                </div>
              ) : (
                <NumField
                  label="Iterations"
                  value={limit.iterations}
                  min={1}
                  onChange={(n) => setExec({ limit: { case: "iterations", iterations: n } })}
                />
              )}
            </div>
            <div className="mt-3 grid grid-cols-1 gap-2 @lg:grid-cols-2">
              <ToggleRow
                label="Quiet"
                hint="k6 -q"
                checked={seg.execution.quiet}
                onChange={(b) => setExec({ quiet: b })}
              />
              <ToggleRow
                label="No thresholds"
                hint="k6 --no-thresholds"
                checked={seg.execution.noThresholds}
                onChange={(b) => setExec({ noThresholds: b })}
              />
            </div>
          </div>

          <div className="border border-zinc-800/60 bg-[#070707] p-4">
            <div className="mb-3 text-[10px] font-mono uppercase tracking-wider text-zinc-600">data parameters</div>
            <div className="grid grid-cols-1 gap-3 @lg:grid-cols-2 @3xl:grid-cols-3">
              <NumField
                label="Pool size"
                value={seg.parameters.poolSize}
                onChange={(n) => setParams({ poolSize: n })}
              />
              <NumField
                label="Scale factor"
                value={seg.parameters.scaleFactor}
                float
                min={0.01}
                onChange={(n) => setParams({ scaleFactor: n })}
                hint="warehouses / branches / scale — fractional for smoke tests"
              />
              <div>
                <Label>Insert method</Label>
                <Select
                  value={seg.parameters.defaultInsertMethod || "native"}
                  onValueChange={(v) => setParams({ defaultInsertMethod: v })}
                >
                  <SelectTrigger className="mt-1">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {INSERT_METHODS.map((m) => (
                      <SelectItem key={m} value={m}>
                        {m}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              {seg.parameters.defaultInsertMethod === "plain_bulk" && (
                <NumField
                  label="Batch size"
                  value={seg.parameters.bulkSize}
                  min={0}
                  max={1000000}
                  onChange={(n) => setParams({ bulkSize: n })}
                  hint="rows per bulk INSERT (0 = stroppy default 2500)"
                />
              )}
            </div>
            {probeSteps.length > 0 && (
              <PhaseChips steps={probeSteps} selected={seg.parameters.steps} onToggle={togglePhase} />
            )}

            {declaredEnv.length > 0 && (
              <ProbeEnvFields decls={declaredEnv} env={seg.parameters.env} onSet={setEnv} />
            )}

            <AdvancedRuntimeParameters initiallyOpen={advancedInitiallyOpen}>
              <EnvMapEditor
                label={declaredEnv.length > 0 ? "Additional environment variables" : "Environment variables"}
                env={seg.parameters.env}
                declaredNames={declaredNames}
                onChange={(env) => setParams({ env })}
              />
              {probeSteps.length === 0 && (
                <StepsEditor steps={seg.parameters.steps} onChange={(steps) => setParams({ steps })} />
              )}
            </AdvancedRuntimeParameters>
          </div>

          {probeContext && (
            <ProbeDisclosure probe={probe} probing={probing} probeErr={probeErr} />
          )}
        </div>
      )}
    </div>
  );
}

/**
 * Collapsed status/output disclosure for a segment's `stroppy probe` result:
 * spinner while probing, error, or an OK summary that expands to the driver/pool
 * details + the `probe -o human` render.
 */
function ProbeDisclosure({
  probe,
  probing,
  probeErr,
}: {
  probe: ProbeMetaVM | null;
  probing: boolean;
  probeErr: string | null;
}) {
  const [open, setOpen] = useState(false);
  const hasDetail = !!probe;
  if (!probing && !probeErr && !probe) return null;
  return (
    <div className="overflow-hidden border border-zinc-800/70 bg-[#070707]">
      <button
        type="button"
        onClick={() => hasDetail && setOpen((v) => !v)}
        aria-expanded={open}
        className={`flex w-full items-center gap-2 px-3 py-2 text-left text-[11px] ${
          hasDetail ? "transition-colors hover:bg-zinc-900/40" : "cursor-default"
        }`}
      >
        {probing ? (
          <span className="flex items-center gap-2 text-zinc-500">
            <Loader2 className="h-3.5 w-3.5 animate-spin" /> Probing workload…
          </span>
        ) : probeErr ? (
          <span className="flex min-w-0 items-center gap-2 text-red-400">
            <AlertCircle className="h-3.5 w-3.5 shrink-0" />
            <span className="truncate">{probeErr}</span>
          </span>
        ) : probe ? (
          <span className="flex items-center gap-2 text-emerald-400">
            <Check className="h-3.5 w-3.5" /> Probe OK — {probe.steps.length} phase
            {probe.steps.length === 1 ? "" : "s"}, {probe.env.length} env
          </span>
        ) : null}
        {hasDetail && (
          <ChevronDown
            className={`ml-auto h-3.5 w-3.5 shrink-0 text-zinc-500 transition-transform ${open ? "rotate-180" : ""}`}
          />
        )}
      </button>

      {open && probe && (
        <div className="space-y-4 border-t border-zinc-800/70 p-3">
          <div className="flex flex-wrap gap-x-6 gap-y-1 text-[11px] text-zinc-500">
            <span>driver: <span className="font-mono text-zinc-300">{probe.driverType}</span></span>
            <span>pool: <span className="font-mono text-zinc-300">{probe.poolSize}</span></span>
            {probe.sqlSections.length > 0 && (
              <span>sql: <span className="font-mono text-zinc-300">{probe.sqlSections.join(", ")}</span></span>
            )}
          </div>
          {probe.human && (
            <div>
              <Label>probe -o human</Label>
              <pre className="mt-1.5 max-h-56 overflow-auto border border-zinc-800/60 bg-black/40 p-2 font-mono text-[10px] leading-relaxed text-zinc-400">
                {probe.human}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function AdvancedRuntimeParameters({
  children,
  initiallyOpen,
}: {
  children: ReactNode;
  initiallyOpen: boolean;
}) {
  const [open, setOpen] = useState(initiallyOpen);
  return (
    <div className="mt-4 overflow-hidden border border-zinc-800/70 bg-[#070707]">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-start gap-3 px-3 py-2.5 text-left transition-colors hover:bg-zinc-900/40"
      >
        <ChevronDown className={`mt-0.5 h-3.5 w-3.5 shrink-0 text-zinc-500 transition-transform ${open ? "rotate-180" : ""}`} />
        <div className="min-w-0">
          <div className="text-sm font-medium text-foreground">Advanced runtime parameters</div>
          <p className="mt-0.5 text-[11px] leading-snug text-zinc-600">
            Extra environment variables and manual phase selection for special runs.
          </p>
        </div>
      </button>
      {open && <div className="border-t border-zinc-800/70 p-3">{children}</div>}
    </div>
  );
}

/**
 * Phase allowlist as toggle chips, populated from the probe's declared phases.
 * Empty selection runs every phase; selecting any subset narrows the run.
 */
function PhaseChips({
  steps,
  selected,
  onToggle,
}: {
  steps: string[];
  selected: string[];
  onToggle: (phase: string) => void;
}) {
  return (
    <div className="mt-4">
      <Label>Phases</Label>
      <p className="mb-1.5 mt-0.5 text-[11px] text-zinc-600">
        From the probe — none selected runs every phase.
      </p>
      <div className="flex flex-wrap gap-1.5">
        {steps.map((s) => {
          const on = selected.includes(s);
          return (
            <button
              key={s}
              type="button"
              onClick={() => onToggle(s)}
              className={`px-2.5 py-1 font-mono text-[11px] transition-all ${
                on
                  ? "border border-primary/40 bg-primary/[0.08] text-primary"
                  : "border border-zinc-800 text-zinc-500 hover:text-zinc-300"
              }`}
            >
              {s}
            </button>
          );
        })}
      </div>
    </div>
  );
}

/**
 * Described env fields, one per probe-declared variable: shows the name,
 * description, required marker, and the script default as placeholder. Editing
 * sets `parameters.env[name]`; clearing it falls back to the default.
 */
function ProbeEnvFields({
  decls,
  env,
  onSet,
}: {
  decls: ProbeMetaVM["env"];
  env: Record<string, string>;
  onSet: (name: string, value: string) => void;
}) {
  return (
    <div className="mt-4">
      <Label>Environment</Label>
      <p className="mb-2 mt-0.5 text-[11px] text-zinc-600">
        Declared by the script — blank uses the default.
      </p>
      <div className="space-y-2">
        {decls.map((d) => {
          const raw = env[d.name] ?? "";
          const set = raw !== "";
          // The value is a string (env vars can be "max"/"false"), but when the
          // script's default is numeric we offer a scrub grip — active only while
          // the current value (or the default it falls back to) is itself numeric.
          const effective = (raw !== "" ? raw : d.default).trim();
          const numericDefault = NUMERIC_RE.test(d.default.trim());
          const numericNow = NUMERIC_RE.test(effective);
          const fractional = effective.includes(".") || d.default.includes(".");
          return (
            <div key={d.name} className="flex items-start gap-3">
              {/* Name + description fill the row width; the description gets two
                  lines before clamping, so most read in full without a hover. */}
              <div className="min-w-0 flex-1">
                <div
                  className="font-mono text-[11px] text-zinc-300"
                  title={d.names.length > 1 ? d.names.join(", ") : undefined}
                >
                  {d.name}
                  {d.required && <span className="ml-1 text-red-400">*</span>}
                </div>
                {d.description && (
                  <div className="mt-0.5 line-clamp-2 text-[10px] leading-snug text-zinc-600">
                    {d.description}
                  </div>
                )}
              </div>
              {numericDefault && (
                <ScrubHandle
                  value={numericNow ? Number(effective) : 0}
                  disabled={!numericNow}
                  step={fractional ? 0.1 : 1}
                  onChange={(n) => onSet(d.name, String(n))}
                />
              )}
              <Input
                className="h-7 w-32 shrink-0 font-mono text-xs"
                placeholder={d.default || "<unset>"}
                value={raw}
                onChange={(e) => onSet(d.name, e.target.value)}
              />
              <button
                type="button"
                onClick={() => onSet(d.name, "")}
                disabled={!set}
                title="Reset to default"
                className="mt-0.5 shrink-0 p-1 text-zinc-600 transition-colors hover:text-zinc-300 disabled:opacity-30"
              >
                <RotateCcw className="h-3.5 w-3.5" />
              </button>
            </div>
          );
        })}
      </div>
    </div>
  );
}

/** Phase steps allowlist editor (comma/newline list). XOR with no_steps. */
function StepsEditor({
  steps,
  onChange,
}: {
  steps: string[];
  onChange: (steps: string[]) => void;
}) {
  return (
    <div>
      <Label>Run only selected phases</Label>
      <Input
        className="mt-1 font-mono text-xs"
        value={steps.join(", ")}
        placeholder="create_schema, load_data, workload"
        onChange={(e) =>
          onChange(
            e.target.value
              .split(/[,\n]/)
              .map((s) => s.trim())
              .filter(Boolean),
          )
        }
      />
      <p className="mt-1 text-[11px] text-zinc-600">
        Leave empty to run every phase.
      </p>
    </div>
  );
}

/**
 * Free-form env map editor (KEY=value lines), backed by the dark ConfigEditor.
 * When `declaredNames` is given, the editor only shows/owns the env keys NOT
 * declared by the probe (those have their own described fields above) — so
 * round-tripping the text never drops the probe-driven values.
 */
function EnvMapEditor({
  env,
  onChange,
  label = "Environment variables",
  declaredNames,
}: {
  env: Record<string, string>;
  onChange: (m: Record<string, string>) => void;
  label?: string;
  declaredNames?: Set<string>;
}) {
  const isExtra = (k: string) => !declaredNames || !declaredNames.has(k);
  const text = useMemo(
    () =>
      Object.entries(env)
        .filter(([k]) => isExtra(k))
        .map(([k, v]) => `${k}=${v}`)
        .join("\n"),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [env, declaredNames],
  );
  const [draftText, setDraftText] = useState(text);
  const lastApplied = useRef(text);
  useEffect(() => {
    if (text !== lastApplied.current) {
      setDraftText(text);
      lastApplied.current = text;
    }
  }, [text, setDraftText]);

  return (
    <div className="mb-3">
      <Label>{label}</Label>
      <div className="mt-1">
        <ConfigEditor
          filename=".env"
          value={draftText}
          height="clamp(8rem, 18vh, 16rem)"
          onChange={(next) => {
            setDraftText(next);
            // Preserve probe-declared keys (edited via their own fields); only
            // the undeclared "extra" keys are parsed back out of this editor.
            const map: Record<string, string> = {};
            for (const [k, v] of Object.entries(env)) {
              if (!isExtra(k)) map[k] = v;
            }
            for (const line of next.split("\n")) {
              const t = line.trim();
              if (!t || t.startsWith("#")) continue;
              const idx = t.indexOf("=");
              if (idx < 0) continue;
              const key = t.slice(0, idx).trim();
              if (!isExtra(key)) continue;
              map[key] = t.slice(idx + 1).trim();
            }
            lastApplied.current = next;
            onChange(map);
          }}
        />
      </div>
    </div>
  );
}

export { FieldErrors, errorsFor };
