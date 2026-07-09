// Per-kind field components for the schemapb launch-form renderer (SP-D §3
// D2). Each component is a thin, controlled wrapper around an existing
// web/src/components/ui primitive — no validation logic lives here, that is
// entirely owned by the schemapb WASM engine via useSchemaForm's `register`
// (see LaunchFormRenderer.tsx / SchemaFormBody.tsx). Components only decide
// how to *display* a field's current value/error and how to translate a UI
// event back into the raw value schemapb expects (bigint for int64, number
// for double/enum, boolean for bool, string otherwise).
import { Input } from "@/components/ui/input";
import { NumField } from "@/components/ui/num-field";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Label } from "@/components/ui/label";
import type { FieldProps } from "@stroppy-io/schemapb-react";
import type { FieldErrorJson } from "@stroppy-io/schemapb";

export function FieldError({ error }: { error?: FieldErrorJson }) {
  if (!error) return null;
  return (
    <p role="alert" className="text-[11px] text-destructive">
      {error.message || error.code}
    </p>
  );
}

export function StringField({ label, field }: { label: string; field: FieldProps }) {
  return (
    <div className="flex flex-col gap-1">
      <Label htmlFor={field.name}>{label}</Label>
      <Input
        id={field.name}
        value={typeof field.value === "string" ? field.value : (field.value ?? "") === null ? "" : String(field.value ?? "")}
        onChange={(e) => field.onChange(e.target.value)}
      />
      <FieldError error={field.error} />
    </div>
  );
}

export function Int64Field({ label, field }: { label: string; field: FieldProps }) {
  // useSchemaForm JSON.stringify's the raw `values` map when calling into the
  // WASM bridge, so int64 values must stay plain JS numbers here (never
  // bigint — JSON.stringify throws on BigInt). schemapb's own int64 JSON
  // convention is number|numeric-string, both of which round-trip fine.
  const raw = field.value;
  const num = typeof raw === "bigint" ? Number(raw) : Number(raw ?? 0);
  return (
    <div className="flex flex-col gap-1">
      <NumField
        id={field.name}
        label={label}
        value={Number.isFinite(num) ? num : 0}
        onChange={(v) => field.onChange(Math.trunc(v))}
      />
      <FieldError error={field.error} />
    </div>
  );
}

export function DoubleField({ label, field }: { label: string; field: FieldProps }) {
  const num = Number(field.value ?? 0);
  return (
    <div className="flex flex-col gap-1">
      <NumField
        id={field.name}
        label={label}
        float
        step={0.01}
        value={Number.isFinite(num) ? num : 0}
        onChange={(v) => field.onChange(v)}
      />
      <FieldError error={field.error} />
    </div>
  );
}

export function BoolField({ label, field }: { label: string; field: FieldProps }) {
  return (
    <div className="flex items-center gap-2">
      <Switch
        id={field.name}
        checked={!!field.value}
        onCheckedChange={(v) => field.onChange(v)}
      />
      <Label htmlFor={field.name}>{label}</Label>
      <FieldError error={field.error} />
    </div>
  );
}

export function EnumField({
  label,
  field,
  values,
}: {
  label: string;
  field: FieldProps;
  values: Record<number, string>;
}) {
  const current = field.value === undefined || field.value === null ? "" : String(field.value);
  return (
    <div className="flex flex-col gap-1">
      <Label htmlFor={field.name}>{label}</Label>
      <Select value={current} onValueChange={(v) => field.onChange(Number(v))}>
        <SelectTrigger id={field.name}>
          <SelectValue placeholder={label} />
        </SelectTrigger>
        <SelectContent>
          {Object.entries(values).map(([n, name]) => (
            <SelectItem key={n} value={n}>
              {name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <FieldError error={field.error} />
    </div>
  );
}
