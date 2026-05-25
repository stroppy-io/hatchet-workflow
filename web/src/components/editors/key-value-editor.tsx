import { useState } from "react";
import { Plus, X } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

type Row = { id: number; key: string; value: string };

let idCounter = 0;
const nextId = () => ++idCounter;

// Editor for proto map<string,string> fields (parameters, env, extra_params).
// Holds local rows so keys can be typed/renamed without losing focus; pushes a
// rebuilt object up on every change. Empty-key rows are omitted from the result.
// Remount (via a `key` prop) to re-seed from a new value.
export function KeyValueEditor({
  label,
  value,
  onChange,
  keyPlaceholder = "key",
  valuePlaceholder = "value",
}: {
  label?: string;
  value: Record<string, string>;
  onChange: (value: Record<string, string>) => void;
  keyPlaceholder?: string;
  valuePlaceholder?: string;
}) {
  const [rows, setRows] = useState<Row[]>(() =>
    Object.entries(value ?? {}).map(([key, val]) => ({ id: nextId(), key, value: val })),
  );

  function push(next: Row[]) {
    setRows(next);
    const result: Record<string, string> = {};
    for (const row of next) if (row.key.trim()) result[row.key] = row.value;
    onChange(result);
  }

  return (
    <div className="space-y-2">
      {label ? <Label>{label}</Label> : null}
      {rows.length === 0 ? <p className="text-xs text-muted-foreground">None</p> : null}
      {rows.map((row) => (
        <div key={row.id} className="flex items-center gap-2">
          <Input className="font-mono text-xs" value={row.key} placeholder={keyPlaceholder} onChange={(event) => push(rows.map((r) => (r.id === row.id ? { ...r, key: event.target.value } : r)))} />
          <Input className="font-mono text-xs" value={row.value} placeholder={valuePlaceholder} onChange={(event) => push(rows.map((r) => (r.id === row.id ? { ...r, value: event.target.value } : r)))} />
          <Button type="button" variant="ghost" size="icon-sm" onClick={() => push(rows.filter((r) => r.id !== row.id))}>
            <X className="size-3.5" />
          </Button>
        </div>
      ))}
      <Button type="button" variant="outline" size="sm" onClick={() => setRows([...rows, { id: nextId(), key: "", value: "" }])}>
        <Plus className="size-3.5" />
        Add
      </Button>
    </div>
  );
}
