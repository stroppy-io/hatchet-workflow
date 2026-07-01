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

function sameDay(a?: Date, b?: Date): boolean {
  return !!a && !!b && a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
}

// Inclusive list of calendar days in [min, max] (capped for safety).
function dayList(min: Date, max: Date): Date[] {
  const out: Date[] = [];
  const end = new Date(max.getFullYear(), max.getMonth(), max.getDate());
  const d = new Date(min.getFullYear(), min.getMonth(), min.getDate());
  while (d <= end && out.length < 93) {
    out.push(new Date(d));
    d.setDate(d.getDate() + 1);
  }
  return out;
}

// ClockFace — a Material-style analog clock for one unit (hours 0–23, or
// minutes/seconds 0–59). Drag the hand (or click a number) to set the value
// from the pointer angle: 12 o'clock = 0, clockwise. Hours render as two
// concentric rings (outer 0–11, inner 12–23); minutes/seconds label every 5.
function ClockFace({ mode, value, onSet, onCommit }: { mode: "h" | "m" | "s"; value: number; onSet: (n: number) => void; onCommit?: () => void }) {
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

  // Hand as a rotation (0° = 12 o'clock, clockwise) so switching units / snapping
  // animates smoothly via a CSS transition instead of jumping.
  const unitMax = mode === "h" ? 12 : 60;
  const deg = ((value % unitMax) / unitMax) * 360;
  const handRadius = radiusFor(value);
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
      onPointerUp={() => onCommit?.()}
      onPointerCancel={() => onCommit?.()}
    >
      <circle cx={cx} cy={cy} r={rOuter + 16} fill="var(--color-muted)" />
      {/* hand — rotated group so angle + radius transition smoothly */}
      <g
        style={{
          transform: `rotate(${deg}deg)`,
          transformOrigin: `${cx}px ${cy}px`,
          transformBox: "view-box",
          transition: "transform 160ms cubic-bezier(0.22, 1, 0.36, 1)",
        }}
      >
        <line x1={cx} y1={cy} x2={cx} y2={cy - handRadius} stroke="var(--color-primary)" strokeWidth={2} style={{ transition: "y2 160ms cubic-bezier(0.22,1,0.36,1)" }} />
        <circle cx={cx} cy={cy - handRadius} r={16} fill="var(--color-primary)" style={{ transition: "cy 160ms cubic-bezier(0.22,1,0.36,1)" }} />
      </g>
      <circle cx={cx} cy={cy} r={3} fill="var(--color-primary)" />
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
  onOpenChange,
}: {
  value?: Date;
  onChange: (d?: Date) => void;
  placeholder?: string;
  className?: string;
  /** Restrict the calendar to [minDate, maxDate] (e.g. the run window) —
   *  days outside are disabled and month navigation is clamped. */
  minDate?: Date;
  maxDate?: Date;
  /** Notified when the popover opens/closes (e.g. so the parent can pause a
   *  live stream that would otherwise jank the clock animation). */
  onOpenChange?: (open: boolean) => void;
}) {
  const base = value ?? undefined;
  // Which unit the clock face edits (Material-style HH:MM:SS switcher).
  const [mode, setMode] = useState<"h" | "m" | "s">("h");

  // While scrubbing the clock (or typing), edits land in a local draft so the
  // face spins smoothly; we push ONE onChange (→ URL → log refetch) on release
  // — dragging the dial must not fire a query per pointer-move tick.
  const [draft, setDraft] = useState<Date | undefined>(undefined);
  const eff = draft ?? value; // value shown/edited (draft wins while active)

  const commit = () => {
    if (draft) {
      onChange(draft);
      setDraft(undefined);
    }
  };

  // Merge a newly-picked calendar day into the current time (or 00:00:00).
  // A day pick is a single action → commit straight away.
  const onDay = (day?: Date) => {
    setDraft(undefined);
    if (!day) {
      onChange(undefined);
      return;
    }
    const d = new Date(day);
    if (eff) {
      d.setHours(eff.getHours(), eff.getMinutes(), eff.getSeconds(), 0);
    } else {
      d.setHours(0, 0, 0, 0);
    }
    onChange(d);
  };

  // Set one time part to an exact number — updates the draft only (commit on
  // pointer-up / input blur). Accumulates on top of the current draft/value.
  const setPartN = (part: "h" | "m" | "s", n: number) => {
    const d = eff ? new Date(eff) : new Date();
    if (!eff) d.setSeconds(0, 0);
    if (part === "h") d.setHours(n);
    if (part === "m") d.setMinutes(n);
    if (part === "s") d.setSeconds(n);
    setDraft(d);
  };
  const h = eff ? eff.getHours() : 0;
  const m = eff ? eff.getMinutes() : 0;
  const sec = eff ? eff.getSeconds() : 0;

  // With a run window, render ONLY the test's days as chips (no month grid);
  // fall back to the full calendar only if the span is implausibly large.
  const days = minDate && maxDate ? dayList(minDate, maxDate) : null;
  const dayChips = days && days.length <= 62 ? days : null;

  // Big editable time segment (inline, not a nested component — a nested one
  // would remount each render and drop input focus mid-typing).
  const segCls = (unit: "h" | "m" | "s") =>
    cn(
      "w-14 rounded px-1 py-0.5 text-center font-mono text-2xl tabular-nums outline-none transition-colors",
      mode === unit ? "bg-primary text-primary-foreground" : "text-foreground hover:bg-muted",
    );
  const onSeg = (unit: "h" | "m" | "s") => (e: React.ChangeEvent<HTMLInputElement>) =>
    setPartN(unit, clamp(parseInt(e.target.value.replace(/\D/g, "") || "0", 10), unit === "h" ? 23 : 59));

  const onSegKey = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") commit();
  };

  return (
    <Popover
      onOpenChange={(open) => {
        if (!open) commit();
        onOpenChange?.(open);
      }}
    >
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
        {dayChips ? (
          // Only the days the test ran — compact chips, no month grid.
          <div className="flex max-w-[264px] flex-wrap justify-center gap-1 p-3">
            {dayChips.map((d) => (
              <button
                key={d.toISOString()}
                type="button"
                onClick={() => onDay(d)}
                className={cn(
                  "rounded px-2.5 py-1 font-mono text-xs transition-colors",
                  sameDay(d, value) ? "bg-primary text-primary-foreground" : "text-foreground hover:bg-muted",
                )}
              >
                {format(d, "EEE d MMM")}
              </button>
            ))}
          </div>
        ) : (
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
        )}
        {/* HH:MM:SS — type digits or click a unit, then use the clock. */}
        <div className="flex items-center justify-center gap-1 border-t border-border px-3 pt-3">
          <input value={pad(h)} inputMode="numeric" maxLength={2} onFocus={() => setMode("h")} onChange={onSeg("h")} onBlur={commit} onKeyDown={onSegKey} className={segCls("h")} />
          <span className="text-2xl text-muted-foreground">:</span>
          <input value={pad(m)} inputMode="numeric" maxLength={2} onFocus={() => setMode("m")} onChange={onSeg("m")} onBlur={commit} onKeyDown={onSegKey} className={segCls("m")} />
          <span className="text-2xl text-muted-foreground">:</span>
          <input value={pad(sec)} inputMode="numeric" maxLength={2} onFocus={() => setMode("s")} onChange={onSeg("s")} onBlur={commit} onKeyDown={onSegKey} className={segCls("s")} />
        </div>
        {/* Analog clock for the active unit — drag the hand or click a number. */}
        <div className="flex justify-center px-3 pb-3">
          <ClockFace mode={mode} value={mode === "h" ? h : mode === "m" ? m : sec} onSet={(n) => setPartN(mode, n)} onCommit={commit} />
        </div>
      </PopoverContent>
    </Popover>
  );
}
