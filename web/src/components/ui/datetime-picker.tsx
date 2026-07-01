// 24-hour date-time picker: a Popover with a shadcn Calendar for the date and
// three padded HH:MM:SS number inputs for the time (guaranteed 24-hour, no
// browser-locale AM/PM). Controlled: value is a Date | undefined.
import { useCallback, useRef } from "react";
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

// Dial — a circular scrub slider for one time unit. Drag anywhere on the ring
// (or click) to set the value from the pointer angle: 12 o'clock = 0, clockwise
// increases to `max`. Absolute mapping (no carry into the neighbouring unit).
function Dial({ value, max, label, onChange }: { value: number; max: number; label: string; onChange: (n: number) => void }) {
  const ref = useRef<SVGSVGElement>(null);
  const size = 52;
  const stroke = 4;
  const cx = size / 2;
  const cy = size / 2;
  const r = (size - stroke * 2) / 2;
  const C = 2 * Math.PI * r;
  const steps = max + 1;
  const frac = value / steps;
  const ang = frac * 2 * Math.PI - Math.PI / 2; // 0 at top, clockwise
  const hx = cx + r * Math.cos(ang);
  const hy = cy + r * Math.sin(ang);

  const setFromEvent = useCallback(
    (e: React.PointerEvent<SVGSVGElement>) => {
      const el = ref.current;
      if (!el) return;
      const rect = el.getBoundingClientRect();
      const x = e.clientX - rect.left - cx;
      const y = e.clientY - rect.top - cy;
      let a = Math.atan2(y, x) + Math.PI / 2; // rotate so 0 = top
      if (a < 0) a += 2 * Math.PI;
      onChange(Math.round((a / (2 * Math.PI)) * steps) % steps);
    },
    [onChange, steps, cx, cy],
  );

  return (
    <div className="flex flex-col items-center gap-1">
      <svg
        ref={ref}
        width={size}
        height={size}
        className="cursor-pointer touch-none select-none"
        onPointerDown={(e) => {
          e.preventDefault();
          (e.target as Element).setPointerCapture?.(e.pointerId);
          setFromEvent(e);
        }}
        onPointerMove={(e) => {
          if (e.buttons === 1) setFromEvent(e);
        }}
      >
        <circle cx={cx} cy={cy} r={r} fill="none" stroke="var(--color-border)" strokeWidth={stroke} />
        <circle
          cx={cx}
          cy={cy}
          r={r}
          fill="none"
          stroke="var(--color-primary)"
          strokeWidth={stroke}
          strokeLinecap="round"
          strokeDasharray={C}
          strokeDashoffset={C * (1 - frac)}
          transform={`rotate(-90 ${cx} ${cy})`}
        />
        <circle cx={hx} cy={hy} r={stroke + 1.5} fill="var(--color-primary)" />
        <text x={cx} y={cy} textAnchor="middle" dominantBaseline="central" className="fill-foreground font-mono" style={{ fontSize: 13 }}>
          {pad(value)}
        </text>
      </svg>
      <span className="text-[10px] uppercase tracking-wider text-muted-foreground">{label}</span>
    </div>
  );
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

  // Set one time part to an exact number (shared by the dials and the inputs).
  const setPartN = (part: "h" | "m" | "s", n: number) => {
    const d = value ? new Date(value) : new Date();
    if (!value) d.setSeconds(0, 0);
    if (part === "h") d.setHours(n);
    if (part === "m") d.setMinutes(n);
    if (part === "s") d.setSeconds(n);
    onChange(d);
  };
  const setPart = (part: "h" | "m" | "s", raw: string) =>
    setPartN(part, clamp(parseInt(raw || "0", 10), part === "h" ? 23 : 59));

  const h = value ? value.getHours() : 0;
  const m = value ? value.getMinutes() : 0;
  const sec = value ? value.getSeconds() : 0;

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
        {/* Circular scrub dials — drag with the mouse to set each unit. */}
        <div className="flex items-center justify-center gap-4 border-t border-border px-3 py-3">
          <Dial value={h} max={23} label="hour" onChange={(n) => setPartN("h", n)} />
          <Dial value={m} max={59} label="min" onChange={(n) => setPartN("m", n)} />
          <Dial value={sec} max={59} label="sec" onChange={(n) => setPartN("s", n)} />
        </div>
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
