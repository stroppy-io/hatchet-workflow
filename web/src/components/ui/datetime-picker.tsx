// 24-hour date-time picker: a Popover with a shadcn Calendar for the date and
// three padded HH:MM:SS number inputs for the time (guaranteed 24-hour, no
// browser-locale AM/PM). Controlled: value is a Date | undefined.
import { CalendarClock, X } from "lucide-react";
import { format } from "date-fns";

import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

function pad(n: number): string {
  return n.toString().padStart(2, "0");
}
function clamp(n: number, max: number): number {
  if (Number.isNaN(n)) return 0;
  return Math.min(max, Math.max(0, n));
}

export function DateTimePicker({
  value,
  onChange,
  placeholder = "Pick date/time",
  className,
}: {
  value?: Date;
  onChange: (d?: Date) => void;
  placeholder?: string;
  className?: string;
}) {
  const base = value ?? undefined;

  // Merge a newly-picked calendar day into the current time (or 00:00:00).
  const onDay = (day?: Date) => {
    if (!day) {
      onChange(undefined);
      return;
    }
    const d = new Date(day);
    if (value) {
      d.setHours(value.getHours(), value.getMinutes(), value.getSeconds(), 0);
    } else {
      d.setHours(0, 0, 0, 0);
    }
    onChange(d);
  };

  const setPart = (part: "h" | "m" | "s", raw: string) => {
    const d = value ? new Date(value) : new Date();
    if (!value) d.setSeconds(0, 0);
    const n = clamp(parseInt(raw || "0", 10), part === "h" ? 23 : 59);
    if (part === "h") d.setHours(n);
    if (part === "m") d.setMinutes(n);
    if (part === "s") d.setSeconds(n);
    onChange(d);
  };

  const num = "w-9 bg-transparent text-center font-mono text-sm outline-none tabular-nums";

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          className={cn("justify-start gap-1.5 font-mono text-[11px] font-normal", !value && "text-muted-foreground", className)}
        >
          <CalendarClock className="h-3.5 w-3.5 shrink-0" />
          {value ? format(value, "yyyy-MM-dd HH:mm:ss") : placeholder}
          {value && (
            <X
              className="ml-1 h-3 w-3 shrink-0 text-muted-foreground hover:text-foreground"
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                onChange(undefined);
              }}
            />
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-auto p-0" align="start">
        <Calendar mode="single" selected={base} onSelect={onDay} defaultMonth={base} autoFocus />
        <div className="flex items-center justify-center gap-1 border-t border-border px-3 py-2">
          <span className="mr-1 text-[11px] text-muted-foreground">24h</span>
          <input
            className={num}
            inputMode="numeric"
            value={value ? pad(value.getHours()) : ""}
            placeholder="HH"
            onChange={(e) => setPart("h", e.target.value)}
          />
          <span className="text-muted-foreground">:</span>
          <input
            className={num}
            inputMode="numeric"
            value={value ? pad(value.getMinutes()) : ""}
            placeholder="MM"
            onChange={(e) => setPart("m", e.target.value)}
          />
          <span className="text-muted-foreground">:</span>
          <input
            className={num}
            inputMode="numeric"
            value={value ? pad(value.getSeconds()) : ""}
            placeholder="SS"
            onChange={(e) => setPart("s", e.target.value)}
          />
        </div>
      </PopoverContent>
    </Popover>
  );
}
