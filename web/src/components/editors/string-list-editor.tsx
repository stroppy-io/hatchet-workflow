import { useState } from "react";
import { Plus, X } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

type Row = { id: number; value: string };

let idCounter = 0;
const nextId = () => ++idCounter;

// Editor for proto repeated string fields (extensions, steps, no_steps). Local
// rows preserve focus; blank rows are dropped from the emitted array. Remount
// (via a `key` prop) to re-seed from a new value.
export function StringListEditor({
  label,
  value,
  onChange,
  placeholder,
}: {
  label?: string;
  value: string[];
  onChange: (value: string[]) => void;
  placeholder?: string;
}) {
  const [rows, setRows] = useState<Row[]>(() => (value ?? []).map((item) => ({ id: nextId(), value: item })));

  function push(next: Row[]) {
    setRows(next);
    onChange(next.map((row) => row.value).filter((item) => item.trim()));
  }

  return (
    <div className="space-y-2">
      {label ? <Label>{label}</Label> : null}
      {rows.length === 0 ? <p className="text-xs text-muted-foreground">None</p> : null}
      {rows.map((row) => (
        <div key={row.id} className="flex items-center gap-2">
          <Input className="font-mono text-xs" value={row.value} placeholder={placeholder} onChange={(event) => push(rows.map((r) => (r.id === row.id ? { ...r, value: event.target.value } : r)))} />
          <Button type="button" variant="ghost" size="icon-sm" onClick={() => push(rows.filter((r) => r.id !== row.id))}>
            <X className="size-3.5" />
          </Button>
        </div>
      ))}
      <Button type="button" variant="outline" size="sm" onClick={() => setRows([...rows, { id: nextId(), value: "" }])}>
        <Plus className="size-3.5" />
        Add
      </Button>
    </div>
  );
}
