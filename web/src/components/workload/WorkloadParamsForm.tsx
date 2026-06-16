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
import { ChevronDown, RotateCcw } from "lucide-react";
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
import { NumField, ToggleRow, FieldErrors, errorsFor } from "@/components/database/DatabaseParamsForm";
import {
  Workload_Protocol,
  type WorkloadVM,
  type DraftErrorVM,
  type ProbeMetaVM,
} from "@/services/wizard";

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
  if (!w.script.trim()) err("workload.script", "A workload needs a script.", "error", "required");
  if (w.execution.vus < 1) err("workload.execution.vus", "At least 1 virtual user.", "error", "min");
  if (w.execution.limit.case === "duration" && !w.execution.limit.duration.trim())
    err("workload.execution.duration", "Set a run duration.", "error", "required");
  if (w.execution.limit.case === "iterations" && w.execution.limit.iterations < 1)
    err("workload.execution.iterations", "At least 1 iteration.", "error", "min");
  if (w.parameters.steps.length > 0 && w.parameters.noSteps.length > 0)
    err("workload.parameters.steps", "steps and no_steps are mutually exclusive.", "error", "exclusive");
  return errs;
}

/** The typed-workload editor — script/sql/protocol + k6 execution + data params. */
export function WorkloadParamsForm({
  w,
  apply,
  disabled,
  advancedInitiallyOpen = disabled ?? false,
  probe = null,
}: {
  w: WorkloadVM;
  apply: (w: WorkloadVM) => void;
  disabled?: boolean;
  advancedInitiallyOpen?: boolean;
  /**
   * Live `stroppy probe` metadata for the current script. When present, its
   * declared phases and env variables are surfaced as first-class controls
   * (chips + described fields) right here in the parameters form — the probe
   * pane is then only a status/output disclosure. Absent on the preset
   * authoring pages, which fall back to the free-text editors.
   */
  probe?: ProbeMetaVM | null;
}) {
  const limit = w.execution.limit;
  const declaredEnv = probe?.env ?? [];
  const declaredNames = useMemo(() => new Set(declaredEnv.map((d) => d.name)), [declaredEnv]);
  const probeSteps = probe?.steps ?? [];

  const setEnv = (name: string, value: string) =>
    apply({ ...w, parameters: { ...w.parameters, env: setEnvKey(w.parameters.env, name, value) } });

  // Toggle a phase in the steps allowlist (XOR with noSteps — selecting any
  // phase clears the blocklist, mirroring the backend's mutual exclusion).
  const togglePhase = (phase: string) => {
    const has = w.parameters.steps.includes(phase);
    const steps = has ? w.parameters.steps.filter((s) => s !== phase) : [...w.parameters.steps, phase];
    apply({ ...w, parameters: { ...w.parameters, steps, noSteps: [] } });
  };
  return (
    <div className={disabled ? "pointer-events-none select-none opacity-90" : undefined}>
      {/* @container: the field grids below break on the PANE's width, not the
          viewport — so they stack to one column when this form shares a narrow
          row with the Version/Preset panes, instead of cramming 2-3 columns. */}
      <div className="@container space-y-5">
        <div className="grid grid-cols-1 gap-3 @lg:grid-cols-2 @3xl:grid-cols-3">
          <div>
            <Label>Script</Label>
            <Input
              className="mt-1 font-mono text-xs"
              value={w.script}
              onChange={(e) => apply({ ...w, script: e.target.value })}
              placeholder="tpcc/tx"
            />
          </div>
          <div>
            <Label>SQL arg (optional)</Label>
            <Input
              className="mt-1 font-mono text-xs"
              value={w.sql}
              onChange={(e) => apply({ ...w, sql: e.target.value })}
              placeholder="queries.sql"
            />
          </div>
          <div>
            <Label>Protocol</Label>
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
          </div>
        </div>

        <div className="border border-zinc-800/60 bg-[#0a0a0a] p-4">
          <div className="mb-3 text-[10px] font-mono uppercase tracking-wider text-zinc-600">k6 execution</div>
          <div className="grid grid-cols-1 gap-3 @lg:grid-cols-2 @3xl:grid-cols-3">
            <NumField
              label="Virtual users"
              value={w.execution.vus}
              onChange={(n) => apply({ ...w, execution: { ...w.execution, vus: n } })}
              min={1}
            />
            <div>
              <Label>Limit by</Label>
              <Select
                value={limit.case}
                onValueChange={(v) =>
                  apply({
                    ...w,
                    execution: {
                      ...w.execution,
                      limit:
                        v === "duration"
                          ? { case: "duration", duration: "5m" }
                          : { case: "iterations", iterations: 10000 },
                    },
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
                  onChange={(e) =>
                    apply({
                      ...w,
                      execution: { ...w.execution, limit: { case: "duration", duration: e.target.value } },
                    })
                  }
                  placeholder="10m"
                />
              </div>
            ) : (
              <NumField
                label="Iterations"
                value={limit.iterations}
                min={1}
                onChange={(n) =>
                  apply({
                    ...w,
                    execution: { ...w.execution, limit: { case: "iterations", iterations: n } },
                  })
                }
              />
            )}
          </div>
          <div className="mt-3 grid grid-cols-1 gap-2 @lg:grid-cols-2">
            <ToggleRow
              label="Quiet"
              hint="k6 -q"
              checked={w.execution.quiet}
              onChange={(b) => apply({ ...w, execution: { ...w.execution, quiet: b } })}
            />
            <ToggleRow
              label="No thresholds"
              hint="k6 --no-thresholds"
              checked={w.execution.noThresholds}
              onChange={(b) => apply({ ...w, execution: { ...w.execution, noThresholds: b } })}
            />
          </div>
        </div>

        <div className="border border-zinc-800/60 bg-[#0a0a0a] p-4">
          <div className="mb-3 text-[10px] font-mono uppercase tracking-wider text-zinc-600">data parameters</div>
          <div className="grid grid-cols-1 gap-3 @lg:grid-cols-2 @3xl:grid-cols-3">
            <NumField
              label="Pool size"
              value={w.parameters.poolSize}
              onChange={(n) => apply({ ...w, parameters: { ...w.parameters, poolSize: n } })}
            />
            <div>
              <Label>Scale factor</Label>
              <Input
                type="number"
                step="0.01"
                className="mt-1"
                value={String(w.parameters.scaleFactor)}
                onChange={(e) => {
                  const n = Number.parseFloat(e.target.value);
                  apply({ ...w, parameters: { ...w.parameters, scaleFactor: Number.isNaN(n) ? 0 : n } });
                }}
              />
            </div>
            <div>
              <Label>Insert method</Label>
              <Input
                className="mt-1"
                value={w.parameters.defaultInsertMethod}
                onChange={(e) => apply({ ...w, parameters: { ...w.parameters, defaultInsertMethod: e.target.value } })}
                placeholder="native"
              />
            </div>
          </div>
          {probeSteps.length > 0 && (
            <PhaseChips steps={probeSteps} selected={w.parameters.steps} onToggle={togglePhase} />
          )}

          {declaredEnv.length > 0 && (
            <ProbeEnvFields decls={declaredEnv} env={w.parameters.env} onSet={setEnv} />
          )}

          <AdvancedRuntimeParameters initiallyOpen={advancedInitiallyOpen}>
            <EnvMapEditor
              label={declaredEnv.length > 0 ? "Additional environment variables" : "Environment variables"}
              env={w.parameters.env}
              declaredNames={declaredNames}
              onChange={(env) => apply({ ...w, parameters: { ...w.parameters, env } })}
            />
            {probeSteps.length === 0 && (
              <StepsEditor
                steps={w.parameters.steps}
                onChange={(steps) => apply({ ...w, parameters: { ...w.parameters, steps } })}
              />
            )}
          </AdvancedRuntimeParameters>
        </div>
      </div>
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
          const set = (env[d.name] ?? "") !== "";
          return (
            <div key={d.name} className="flex items-center gap-2">
              <div className="w-44 shrink-0">
                <div className="font-mono text-[11px] text-zinc-300">
                  {d.name}
                  {d.required && <span className="ml-1 text-red-400">*</span>}
                </div>
                {d.description && (
                  <div className="truncate text-[10px] text-zinc-600" title={d.description}>
                    {d.description}
                  </div>
                )}
              </div>
              <Input
                className="h-8 flex-1 font-mono text-xs"
                placeholder={d.default || "<unset>"}
                value={env[d.name] ?? ""}
                onChange={(e) => onSet(d.name, e.target.value)}
              />
              <button
                type="button"
                onClick={() => onSet(d.name, "")}
                disabled={!set}
                title="Reset to default"
                className="shrink-0 p-1 text-zinc-600 transition-colors hover:text-zinc-300 disabled:opacity-30"
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
