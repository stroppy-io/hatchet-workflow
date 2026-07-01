// 24-hour date-time picker: a Popover with a shadcn Calendar for the date and
// three padded HH:MM:SS number inputs for the time (guaranteed 24-hour, no
// browser-locale AM/PM). Controlled: value is a Date | undefined.
import { useCallback, useRef, useState } from "react";
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

// ClockFace — a Material-style analog clock for one unit (hours 0–23, or
// minutes/seconds 0–59). Drag the hand (or click a number) to set the value
// from the pointer angle: 12 o'clock = 0, clockwise. Hours render as two
// concentric rings (outer 0–11, inner 12–23); minutes/seconds label every 5.
function ClockFace({ mode, value, onSet }: { mode: "h" | "m" | "s"; value: number; onSet: (n: number) => void }) {
  const ref = useRef<SVGSVGElement>(null);
  const size = 232;
  const cx = size / 2;
  const cy = size / 2;
  const rOuter = 96;
  const rInner = 62; // inner ring for hours 12–23

  // Radius the hand/handle reaches for a given value.
  const radiusFor = (v: number) => (mode === "h" && v >= 12 ? rInner : rOuter);
  const pos = (v: number, r: number) => {
    const a = (v / (mode === "h" ? 12 : 60)) * 2 * Math.PI - Math.PI / 2;
    return { x: cx + r * Math.cos(a), y: cy + r * Math.sin(a) };
  };

  const setFromEvent = useCallback(
    (e: React.PointerEvent<SVGSVGElement>) => {
      const el = ref.current;
      if (!el) return;
      const rect = el.getBoundingClientRect();
      const dx = e.clientX - rect.left - cx;
      const dy = e.clientY - rect.top - cy;
      let a = Math.atan2(dy, dx) + Math.PI / 2;
      if (a < 0) a += 2 * Math.PI;
      if (mode === "h") {
        // 12 sectors; inner vs outer ring picks 12–23 vs 0–11 by cursor radius.
        const dist = Math.hypot(dx, dy);
        const base = Math.round((a / (2 * Math.PI)) * 12) % 12;
        const inner = dist < (rOuter + rInner) / 2;
        onSet(inner ? (base === 0 ? 12 : base + 12) : base);
      } else {
        onSet(Math.round((a / (2 * Math.PI)) * 60) % 60);
      }
    },
    [mode, cx, cy],
  );

  const handle = pos(value, radiusFor(value));
  // Labels: hours -> 0..11 outer + 12..23 inner; min/sec -> 0,5..55 outer.
  const outerLabels = mode === "h" ? Array.from({ length: 12 }, (_, i) => i) : Array.from({ length: 12 }, (_, i) => i * 5);
  const innerLabels = mode === "h" ? Array.from({ length: 12 }, (_, i) => (i === 0 ? 12 : i + 12)) : [];

  const Label = ({ v, r }: { v: number; r: number }) => {
    const p = pos(mode === "h" ? v % 12 : v, r);
    const active = v === value;
    return (
      <text
        x={p.x}
        y={p.y}
        textAnchor="middle"
        dominantBaseline="central"
        className={cn("pointer-events-none select-none font-mono", active ? "fill-primary-foreground" : "fill-foreground")}
        style={{ fontSize: r === rInner ? 11 : 13 }}
      >
        {mode === "h" ? v : pad(v)}
      </text>
    );
  };

  return (
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
      <circle cx={cx} cy={cy} r={rOuter + 16} fill="var(--color-muted)" />
      {/* hand */}
      <line x1={cx} y1={cy} x2={handle.x} y2={handle.y} stroke="var(--color-primary)" strokeWidth={2} />
      <circle cx={cx} cy={cy} r={3} fill="var(--color-primary)" />
      <circle cx={handle.x} cy={handle.y} r={16} fill="var(--color-primary)" />
      {outerLabels.map((v) => (
        <Label key={`o${v}`} v={v} r={rOuter} />
      ))}
      {innerLabels.map((v) => (
        <Label key={`i${v}`} v={v} r={rInner} />
      ))}
    </svg>
  );
}

export function DateTimePicker({
  value,
  onChange,
  placeholder = "Pick date/time",
  className,
  minDate,
  maxDate,
}: {
  value?: Date;
  onChange: (d?: Date) => void;
  placeholder?: string;
  className?: string;
  /** Restrict the calendar to [minDate, maxDate] (e.g. the run window) —
   *  days outside are disabled and month navigation is clamped. */
  minDate?: Date;
  maxDate?: Date;
}) {
  const base = value ?? undefined;
  // Which unit the clock face edits (Material-style HH:MM:SS switcher).
  const [mode, setMode] = useState<"h" | "m" | "s">("h");

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
        <Calendar
          mode="single"
          selected={base}
          onSelect={onDay}
          defaultMonth={base ?? minDate}
          startMonth={minDate}
          endMonth={maxDate}
          disabled={[...(minDate ? [{ before: minDate }] : []), ...(maxDate ? [{ after: maxDate }] : [])]}
          autoFocus
        />
        {/* Material-style HH:MM:SS switcher — click a unit, then use the clock. */}
        <div className="flex items-center justify-center gap-1 border-t border-border px-3 pt-3 font-mono text-2xl tabular-nums">
          {(["h", "m", "s"] as const).map((u, i) => (
            <span key={u} className="flex items-center">
              {i > 0 && <span className="px-1 text-muted-foreground">:</span>}
              <button
                type="button"
                onClick={() => setMode(u)}
                className={cn(
                  "rounded px-2 py-0.5 transition-colors",
                  mode === u ? "bg-primary text-primary-foreground" : "text-foreground hover:bg-muted",
                )}
              >
                {pad(u === "h" ? h : u === "m" ? m : sec)}
              </button>
            </span>
          ))}
        </div>
        {/* Analog clock for the active unit — drag the hand or click a number. */}
        <div className="flex justify-center px-3 py-2">
          <ClockFace mode={mode} value={mode === "h" ? h : mode === "m" ? m : sec} onSet={(n) => setPartN(mode, n)} />
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
