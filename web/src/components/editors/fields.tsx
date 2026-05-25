import type { ReactNode } from "react";

import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";

export type Option = { label: string; value: string };

// Build a Select option list from a proto enum + a label map, dropping the
// UNSPECIFIED (0) value. Keeps editors proto-truthful (enum is the source).
export function enumOptions(labels: Record<number, string>): Option[] {
  return Object.entries(labels)
    .filter(([value]) => Number(value) !== 0)
    .map(([value, label]) => ({ value, label }));
}

// Derive options straight from a generated TS enum (name keys → humanized
// labels), dropping UNSPECIFIED (0). For large enums where hand-written labels
// add no value (e.g. YDB resource presets).
export function enumAuto(enumObject: Record<string, string | number>): Option[] {
  return Object.entries(enumObject)
    .filter(([, value]) => typeof value === "number" && value !== 0)
    .map(([key, value]) => ({
      value: String(value),
      label: key
        .toLowerCase()
        .replace(/_/g, " ")
        .replace(/^\w/, (c) => c.toUpperCase()),
    }));
}

export function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <div className="space-y-1.5">
      <Label>{label}</Label>
      {children}
      {hint ? <p className="text-xs text-muted-foreground">{hint}</p> : null}
    </div>
  );
}

export function TextField({
  label,
  value,
  onChange,
  placeholder,
  hint,
  type = "text",
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  hint?: string;
  type?: string;
}) {
  return (
    <Field label={label} hint={hint}>
      <Input type={type} value={value} placeholder={placeholder} onChange={(event) => onChange(event.target.value)} />
    </Field>
  );
}

export function NumberField({
  label,
  value,
  onChange,
  min,
  max,
  hint,
}: {
  label: string;
  value: number;
  onChange: (value: number) => void;
  min?: number;
  max?: number;
  hint?: string;
}) {
  return (
    <Field label={label} hint={hint}>
      <Input
        type="number"
        value={Number.isFinite(value) ? value : 0}
        min={min}
        max={max}
        onChange={(event) => onChange(event.target.value === "" ? 0 : Number(event.target.value))}
      />
    </Field>
  );
}

export function SelectField({
  label,
  value,
  onChange,
  options,
  hint,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: Option[];
  hint?: string;
}) {
  return (
    <Field label={label} hint={hint}>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger className="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </Field>
  );
}

export function SwitchField({
  label,
  checked,
  onCheckedChange,
}: {
  label: string;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
}) {
  return (
    <div className="flex items-center justify-between py-1">
      <Label>{label}</Label>
      <Switch checked={checked} onCheckedChange={onCheckedChange} />
    </div>
  );
}

// Section groups a labelled block of fields with a thin divider header.
export function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="space-y-3">
      <div className="flex items-center gap-3">
        <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{title}</h3>
        <div className="h-px flex-1 bg-border" />
      </div>
      {children}
    </div>
  );
}
