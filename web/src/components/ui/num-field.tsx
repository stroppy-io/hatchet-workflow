// Numeric form primitives with "scrubby-slider" drag behavior: grab the handle
// and drag left/right to change a number relative to its current value,
// Premiere/Blender style. Uses the Pointer Lock API so the cursor hides and you
// can drag past the screen edge indefinitely; falls back to plain pointer deltas
// (touch / no lock, where it stays bounded by the screen). The faster you drag,
// the larger the increment; hold Shift for fine (¼-speed), Alt/Ctrl for coarse
// (×10).
//
// Exports:
//   NumField    — a labeled numeric field whose label is the drag handle.
//   ScrubHandle — a standalone drag grip to sit next to a free-text input.

import { useCallback, useEffect, useRef, useState } from "react";
import { GripVertical } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";

function useScrub({
  value,
  onChange,
  min,
  max,
  step = 1,
  float = false,
}: {
  value: number;
  onChange: (n: number) => void;
  min?: number;
  max?: number;
  step?: number;
  /** Allow fractional values (typed precisely; scrubbed in whole `step`s). */
  float?: boolean;
}) {
  const [scrubbing, setScrubbing] = useState(false);
  // Read the latest value inside the long-lived drag listeners without
  // re-binding them every render.
  const valueRef = useRef(value);
  valueRef.current = value;
  // Latest active-drag teardown, so unmounting mid-scrub doesn't leak the
  // window listeners or leave body styles / the pointer lock stuck.
  const cleanupRef = useRef<(() => void) | null>(null);
  useEffect(() => () => cleanupRef.current?.(), []);

  const onPointerDown = useCallback(
    (e: React.PointerEvent<HTMLElement>) => {
      if (e.button !== 0) return;
      e.preventDefault();
      const handle = e.currentTarget;
      const isMouse = e.pointerType === "mouse";
      const downX = e.clientX;
      let lastX = e.clientX;
      let started = false; // engaged only after a few px of travel (so a plain click never locks)
      let locked = false;
      let acc = 0; // fractional-step accumulator so slow drags still tick by `step`

      // Round to the step's precision (so a 0.1 step doesn't drift into
      // 0.30000000000000004 territory). Float fields scrub in whole `step`s but
      // keep any fractional offset the user typed, so round generously there.
      const dot = String(step).indexOf(".");
      const stepDecimals = dot < 0 ? 0 : String(step).length - dot - 1;
      const decimals = float ? Math.max(stepDecimals, 6) : stepDecimals;
      const pow = Math.pow(10, decimals);
      const clamp = (n: number) => {
        let r = Math.round(n * pow) / pow;
        if (min !== undefined) r = Math.max(min, r);
        if (max !== undefined) r = Math.min(max, r);
        return r;
      };
      const apply = (dx: number, shift: boolean, coarse: boolean) => {
        const fine = shift ? 0.25 : 1;
        const big = coarse ? 10 : 1;
        const accel = 1 + Math.min(Math.abs(dx) * 0.08, 5); // speed → bigger jumps
        acc += (dx / 5) * fine * big * accel; // ~5px of drag per step, before accel
        const steps = Math.trunc(acc); // whole steps to apply this move
        if (steps === 0) return;
        acc -= steps;
        const next = clamp(valueRef.current + steps * step);
        if (next !== valueRef.current) {
          valueRef.current = next;
          onChange(next);
        }
      };

      const onLockChange = () => {
        locked = document.pointerLockElement === handle;
      };
      const onMove = (ev: PointerEvent) => {
        if (!started) {
          if (Math.abs(ev.clientX - downX) < 3) return;
          started = true;
          if (isMouse) handle.requestPointerLock?.();
          lastX = ev.clientX;
          return;
        }
        // Under pointer lock the cursor is frozen, so use the relative
        // movementX; otherwise diff against the last client position.
        const dx = locked ? ev.movementX : ev.clientX - lastX;
        lastX = ev.clientX;
        apply(dx, ev.shiftKey, ev.altKey || ev.ctrlKey || ev.metaKey);
      };
      const end = () => {
        window.removeEventListener("pointermove", onMove);
        window.removeEventListener("pointerup", end);
        window.removeEventListener("pointercancel", end);
        document.removeEventListener("pointerlockchange", onLockChange);
        if (document.pointerLockElement === handle) document.exitPointerLock();
        document.body.style.cursor = "";
        document.body.style.userSelect = "";
        cleanupRef.current = null;
        setScrubbing(false);
      };

      cleanupRef.current = end;
      setScrubbing(true);
      document.body.style.cursor = "ew-resize";
      document.body.style.userSelect = "none";
      document.addEventListener("pointerlockchange", onLockChange);
      window.addEventListener("pointermove", onMove);
      window.addEventListener("pointerup", end);
      window.addEventListener("pointercancel", end);
    },
    [min, max, step, float, onChange],
  );

  return { scrubbing, onPointerDown };
}

export function NumField({
  label,
  value,
  onChange,
  min = 0,
  max,
  step = 1,
  float = false,
  hint,
  disabled = false,
  id,
  inputClassName,
}: {
  label: string;
  value: number;
  onChange: (n: number) => void;
  min?: number;
  max?: number;
  step?: number;
  /** Accept fractional input (typed precisely; scrubbed in whole `step`s). */
  float?: boolean;
  hint?: string;
  disabled?: boolean;
  /** Wires the label's htmlFor to the input for a11y when provided. */
  id?: string;
  /** Extra classes for the inner <Input> (e.g. font-mono / sizing). */
  inputClassName?: string;
}) {
  const { scrubbing, onPointerDown } = useScrub({ value, onChange, min, max, step, float });
  const isFloat = float || !Number.isInteger(step);
  return (
    <div className="min-w-0">
      {/* The label itself is the drag handle (with an inline grip), kept as the
          same inline <Label> as non-scrub fields so it lines up vertically.
          Click-to-type still works on the input; only a real drag scrubs. */}
      <Label
        htmlFor={id}
        onPointerDown={disabled ? undefined : onPointerDown}
        title={disabled ? undefined : "Drag to adjust (Shift = fine, Alt = coarse)"}
        className={cn(
          "touch-none select-none",
          disabled
            ? "cursor-default opacity-50"
            : cn("cursor-ew-resize", scrubbing ? "text-primary" : "hover:text-foreground"),
        )}
      >
        <GripVertical className="mr-1 inline-block h-3 w-3 -translate-y-px align-middle opacity-50" />
        {label}
      </Label>
      <Input
        id={id}
        type="number"
        className={cn("mt-1", inputClassName)}
        min={min}
        max={max}
        step={isFloat ? "any" : step}
        disabled={disabled}
        value={String(value)}
        onChange={(e) => {
          const n = isFloat ? Number.parseFloat(e.target.value) : Number.parseInt(e.target.value, 10);
          onChange(Number.isNaN(n) ? 0 : n);
        }}
      />
      {hint && <p className="mt-1 text-[11px] text-muted-foreground">{hint}</p>}
    </div>
  );
}

// A standalone scrub grip — the same drag-to-change behavior as NumField, but
// detached from a label so it can sit next to a free-text input (e.g. a probe
// env field whose value is usually numeric but may be a word like "max"). When
// `disabled` (the current value isn't a number) it dims and ignores drags.
export function ScrubHandle({
  value,
  onChange,
  min,
  max,
  step = 1,
  disabled = false,
}: {
  value: number;
  onChange: (n: number) => void;
  min?: number;
  max?: number;
  step?: number;
  disabled?: boolean;
}) {
  const { scrubbing, onPointerDown } = useScrub({ value, onChange, min, max, step });
  return (
    <button
      type="button"
      tabIndex={-1}
      onPointerDown={disabled ? undefined : onPointerDown}
      title={disabled ? "Enter a number to drag" : "Drag to adjust (Shift = fine, Alt = coarse)"}
      className={cn(
        "flex h-7 shrink-0 touch-none items-center",
        disabled ? "cursor-default opacity-25" : "cursor-ew-resize",
        scrubbing ? "text-primary" : "text-muted-foreground hover:text-foreground",
      )}
    >
      <GripVertical className="h-3.5 w-3.5" />
    </button>
  );
}
