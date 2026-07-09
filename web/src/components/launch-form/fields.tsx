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

// schemapb bounds may be bigint (Int64) or number|"NaN"|"Infinity"|"-Infinity"
// (Double). Convert to the finite JS number NumField's min/max accept, or
// `undefined` when the schema declares no bound — NumField itself defaults
// `min` to 0, which would wrongly clamp a field with a legitimately negative
// range, so callers below must pass `min={undefined}` explicitly rather than
// omitting the prop.
function toBoundNumber(v: unknown): number | undefined {
  if (v === undefined || v === null) return undefined;
  const n = typeof v === "bigint" ? Number(v) : Number(v);
  return Number.isFinite(n) ? n : undefined;
}

export function Int64Field({
  label,
  field,
  gt,
  gte,
  lt,
  lte,
}: {
  label: string;
  field: FieldProps;
  gt?: bigint;
  gte?: bigint;
  lt?: bigint;
  lte?: bigint;
}) {
  // useSchemaForm JSON.stringify's the raw `values` map when calling into the
  // WASM bridge, so int64 values must stay plain JS numbers here (never
  // bigint — JSON.stringify throws on BigInt). schemapb's own int64 JSON
  // convention is number|numeric-string, both of which round-trip fine.
  const raw = field.value;
  const num = typeof raw === "bigint" ? Number(raw) : Number(raw ?? 0);
  // Inclusive bound takes precedence when both are set; NumField only
  // supports a single inclusive min/max, so an exclusive-only bound is
  // widened by one (schemapb still enforces the real, exact constraint).
  const min = toBoundNumber(gte) ?? (gt !== undefined ? toBoundNumber(gt)! + 1 : undefined);
  const max = toBoundNumber(lte) ?? (lt !== undefined ? toBoundNumber(lt)! - 1 : undefined);
  return (
    <div className="flex flex-col gap-1">
      <NumField
        id={field.name}
        label={label}
        min={min}
        max={max}
        value={Number.isFinite(num) ? num : 0}
        onChange={(v) => field.onChange(Math.trunc(v))}
      />
      <FieldError error={field.error} />
    </div>
  );
}

export function DoubleField({
  label,
  field,
  gt,
  gte,
  lt,
  lte,
}: {
  label: string;
  field: FieldProps;
  gt?: number | "NaN" | "Infinity" | "-Infinity";
  gte?: number | "NaN" | "Infinity" | "-Infinity";
  lt?: number | "NaN" | "Infinity" | "-Infinity";
  lte?: number | "NaN" | "Infinity" | "-Infinity";
}) {
  const num = Number(field.value ?? 0);
  const min = toBoundNumber(gte) ?? (toBoundNumber(gt) !== undefined ? toBoundNumber(gt)! : undefined);
  const max = toBoundNumber(lte) ?? (toBoundNumber(lt) !== undefined ? toBoundNumber(lt)! : undefined);
  return (
    <div className="flex flex-col gap-1">
      <NumField
        id={field.name}
        label={label}
        float
        step={0.01}
        min={min}
        max={max}
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
  options,
}: {
  label: string;
  field: FieldProps;
  values: Record<number, string>;
  /** Allowed enum values for the current form state — from the engine's
   *  `form.enumOptions(path)`, which honors dynamic `options_expr`
   *  narrowing. Must be used instead of the static `values` map's keys,
   *  which does not reflect options_expr and would let the UI offer
   *  choices the engine will reject. */
  options: number[];
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
          {options.map((n) => (
            <SelectItem key={n} value={String(n)}>
              {values[n] ?? String(n)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <FieldError error={field.error} />
    </div>
  );
}
