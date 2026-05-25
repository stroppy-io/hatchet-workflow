import { create } from "@bufbuild/protobuf";
import { Plus, X } from "lucide-react";

import { NumberField, SelectField, Section, SwitchField, TextField, enumOptions } from "@/components/editors/fields";
import { KeyValueEditor } from "@/components/editors/key-value-editor";
import { StringListEditor } from "@/components/editors/string-list-editor";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import type { Workload, Workload_WorkloadFile } from "@/lib/proto/cloud/v1/domain/workload_pb.ts";
import {
  Workload_ExecutionSchema,
  Workload_ParametersSchema,
  Workload_Protocol,
  Workload_WorkloadFileSchema,
  WorkloadSchema,
} from "@/lib/proto/cloud/v1/domain/workload_pb.ts";

const PROTOCOL_OPTIONS = enumOptions({
  [Workload_Protocol.PG]: "PostgreSQL",
  [Workload_Protocol.MYSQL]: "MySQL",
  [Workload_Protocol.PICODATA]: "Picodata",
  [Workload_Protocol.YDB_GRPC]: "YDB gRPC",
  [Workload_Protocol.YDB_GRPCS]: "YDB gRPCS",
  [Workload_Protocol.COCKROACH]: "Cockroach",
});

const LIMIT_OPTIONS = [
  { label: "Duration", value: "duration" },
  { label: "Iterations", value: "iterations" },
];

export function WorkloadEditor({ value, onChange }: { value: Workload; onChange: (value: Workload) => void }) {
  // Always emit a proper Workload message; create() coerces plain nested patches.
  const patch = (partial: Partial<Workload>) => onChange(create(WorkloadSchema, { ...value, ...partial }));
  // Default to concrete messages so spread-patches keep a defined $typeName
  // (MessageInit rejects spreading an optional message).
  const execution = value.execution ?? create(Workload_ExecutionSchema, {});
  const parameters = value.parameters ?? create(Workload_ParametersSchema, {});
  const limitCase = execution.limit.case ?? "duration";

  function patchFile(index: number, partial: Partial<Workload_WorkloadFile>) {
    patch({ files: value.files.map((file, i) => (i === index ? { ...file, ...partial } : file)) });
  }

  return (
    <div className="space-y-6">
      <Section title="Workload">
        <div className="grid gap-4 sm:grid-cols-2">
          <TextField label="Stroppy version" value={value.stroppyVersion} onChange={(v) => patch({ stroppyVersion: v })} placeholder="latest" />
          <SelectField label="Protocol" value={String(value.protocol)} onChange={(v) => patch({ protocol: Number(v) as Workload_Protocol })} options={PROTOCOL_OPTIONS} />
          <TextField label="Script" value={value.script} onChange={(v) => patch({ script: v })} placeholder="tpcc, ycsb, …" />
          <TextField label="SQL (optional)" value={value.sql} onChange={(v) => patch({ sql: v })} />
        </div>
      </Section>

      <Section title="Execution">
        <div className="grid gap-4 sm:grid-cols-2">
          <NumberField label="Virtual users (VUs)" value={execution?.vus ?? 1} min={1} onChange={(v) => patch({ execution: { ...execution, vus: v } })} />
          <SelectField
            label="Limit by"
            value={limitCase}
            onChange={(v) =>
              patch({
                execution: {
                  ...execution,
                  limit: v === "iterations" ? { case: "iterations", value: 1 } : { case: "duration", value: "" },
                },
              })
            }
            options={LIMIT_OPTIONS}
          />
          {limitCase === "iterations" ? (
            <NumberField
              label="Iterations"
              value={execution?.limit.case === "iterations" ? execution.limit.value : 1}
              min={1}
              onChange={(v) => patch({ execution: { ...execution, limit: { case: "iterations", value: v } } })}
            />
          ) : (
            <TextField
              label="Duration"
              value={execution?.limit.case === "duration" ? execution.limit.value : ""}
              onChange={(v) => patch({ execution: { ...execution, limit: { case: "duration", value: v } } })}
              placeholder="10m, 30s, 1h…"
            />
          )}
        </div>
        <div className="grid gap-2 sm:grid-cols-2">
          <SwitchField label="Quiet" checked={execution?.quiet ?? false} onCheckedChange={(c) => patch({ execution: { ...execution, quiet: c } })} />
          <SwitchField label="No thresholds" checked={execution?.noThresholds ?? false} onCheckedChange={(c) => patch({ execution: { ...execution, noThresholds: c } })} />
        </div>
      </Section>

      <Section title="Parameters">
        <div className="grid gap-4 sm:grid-cols-3">
          <NumberField label="Pool size" value={parameters?.poolSize ?? 0} min={0} onChange={(v) => patch({ parameters: { ...parameters, poolSize: v } })} />
          <NumberField label="Scale factor" value={parameters?.scaleFactor ?? 1} min={0} onChange={(v) => patch({ parameters: { ...parameters, scaleFactor: v } })} />
          <TextField label="Default insert method" value={parameters?.defaultInsertMethod ?? ""} onChange={(v) => patch({ parameters: { ...parameters, defaultInsertMethod: v } })} />
        </div>
        <KeyValueEditor
          label="Environment"
          value={parameters?.env ?? {}}
          onChange={(env) => patch({ parameters: { ...parameters, env } })}
          keyPlaceholder="ENV_VAR"
        />
        <div className="grid gap-4 sm:grid-cols-2">
          <StringListEditor label="Steps" value={parameters?.steps ?? []} onChange={(steps) => patch({ parameters: { ...parameters, steps } })} placeholder="step_name" />
          <StringListEditor label="Skip steps" value={parameters?.noSteps ?? []} onChange={(noSteps) => patch({ parameters: { ...parameters, noSteps } })} placeholder="step_name" />
        </div>
      </Section>

      <Section title="Files">
        <div className="space-y-3">
          {value.files.length === 0 ? <p className="text-xs text-muted-foreground">No files</p> : null}
          {value.files.map((file, index) => (
            <div key={index} className="space-y-2 rounded-md border p-3">
              <div className="flex items-center gap-2">
                <Input className="font-mono text-xs" value={file.name} placeholder="filename" onChange={(event) => patchFile(index, { name: event.target.value })} />
                <Input className="w-40 font-mono text-xs" value={file.kind} placeholder="kind" onChange={(event) => patchFile(index, { kind: event.target.value })} />
                <Button type="button" variant="ghost" size="icon-sm" onClick={() => patch({ files: value.files.filter((_, i) => i !== index) })}>
                  <X className="size-3.5" />
                </Button>
              </div>
              <div className="space-y-1.5">
                <Label>Content</Label>
                <Textarea className="font-mono text-xs" rows={5} value={file.content} onChange={(event) => patchFile(index, { content: event.target.value })} />
              </div>
            </div>
          ))}
          <Button type="button" variant="outline" size="sm" onClick={() => patch({ files: [...value.files, create(Workload_WorkloadFileSchema, {})] })}>
            <Plus className="size-3.5" />
            Add file
          </Button>
        </div>
      </Section>
    </div>
  );
}
