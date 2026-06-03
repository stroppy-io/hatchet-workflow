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

import { useEffect, useMemo, useRef, useState } from "react";
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
} from "@/services/wizard";

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
}: {
  w: WorkloadVM;
  apply: (w: WorkloadVM) => void;
  disabled?: boolean;
}) {
  const limit = w.execution.limit;
  return (
    <div className={disabled ? "pointer-events-none select-none opacity-90" : undefined}>
      <div className="space-y-5">
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-3">
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
          <div className="grid grid-cols-3 gap-3">
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
          <div className="mt-3 grid grid-cols-2 gap-2">
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
          <div className="grid grid-cols-3 gap-3">
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
          <EnvMapEditor
            env={w.parameters.env}
            onChange={(env) => apply({ ...w, parameters: { ...w.parameters, env } })}
          />
          <StepsEditor
            steps={w.parameters.steps}
            onChange={(steps) => apply({ ...w, parameters: { ...w.parameters, steps } })}
          />
        </div>
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
    <div className="mt-3">
      <Label>Phase steps (allowlist)</Label>
      <Input
        className="mt-1 font-mono text-xs"
        value={steps.join(", ")}
        placeholder="create_schema, load_data, workload"
        onChange={(e) =>
          onChange(
            e.target.value
              .split(",")
              .map((s) => s.trim())
              .filter(Boolean),
          )
        }
      />
      <p className="mt-1 text-[11px] text-zinc-600">
        Empty runs every phase. A list restricts the run to those phases (Workload.Parameters.steps).
      </p>
    </div>
  );
}

/** Free-form env map editor (KEY=value lines), backed by the dark ConfigEditor. */
function EnvMapEditor({
  env,
  onChange,
}: {
  env: Record<string, string>;
  onChange: (m: Record<string, string>) => void;
}) {
  const text = useMemo(
    () =>
      Object.entries(env)
        .map(([k, v]) => `${k}=${v}`)
        .join("\n"),
    [env],
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
    <div className="mt-3">
      <Label>Environment (KEY=value)</Label>
      <div className="mt-1">
        <ConfigEditor
          filename=".env"
          value={draftText}
          height="clamp(8rem, 18vh, 16rem)"
          onChange={(next) => {
            setDraftText(next);
            const map: Record<string, string> = {};
            for (const line of next.split("\n")) {
              const t = line.trim();
              if (!t || t.startsWith("#")) continue;
              const idx = t.indexOf("=");
              if (idx < 0) continue;
              map[t.slice(0, idx).trim()] = t.slice(idx + 1).trim();
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
