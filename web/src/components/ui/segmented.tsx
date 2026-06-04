// SegmentedControl — a segmented switch with an indicator that slides/flows
// between options (cubic-bezier ease). Equal-width segments; the indicator is
// positioned by index so it animates across any N options.
//   - "boxed" (default): bordered pill, used for compact toggles.
//   - "tabs": borderless, full-height segments + full-height indicator, used as
//     a flush tab strip.
import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export interface SegmentOption<T extends string> {
  value: T;
  label: string;
  icon?: ReactNode;
}

interface SegmentedControlProps<T extends string> {
  options: SegmentOption<T>[];
  value: T;
  onChange: (value: T) => void;
  className?: string;
  /** Tailwind width per segment (default w-24). */
  segmentClassName?: string;
  variant?: "boxed" | "tabs";
}

export function SegmentedControl<T extends string>({
  options,
  value,
  onChange,
  className,
  segmentClassName = "w-24",
  variant = "boxed",
}: SegmentedControlProps<T>) {
  const n = options.length;
  const idx = Math.max(0, options.findIndex((o) => o.value === value));
  const tabs = variant === "tabs";

  // Indicator geometry: boxed has 2px padding all round (p-0.5), tabs has none.
  const pad = tabs ? "0px" : "2px";
  const inset = tabs ? "inset-0" : "inset-y-0.5";

  return (
    <div
      className={cn(
        "relative inline-flex bg-background",
        tabs ? "h-full" : "border border-border p-0.5",
        className,
      )}
    >
      <span
        aria-hidden
        className={cn(
          "absolute rounded-[2px] bg-primary/15 ring-1 ring-primary/40 transition-all duration-300 ease-[cubic-bezier(0.22,1,0.36,1)]",
          inset,
        )}
        style={{
          width: `calc((100% - 2 * ${pad}) / ${n})`,
          left: `calc(${pad} + ${idx} * (100% - 2 * ${pad}) / ${n})`,
        }}
      />
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          onClick={() => onChange(o.value)}
          className={cn(
            "relative z-10 flex items-center justify-center gap-1.5 font-mono text-[10px] uppercase tracking-wider transition-colors duration-200",
            tabs ? "py-2" : "py-1",
            segmentClassName,
            value === o.value ? "text-primary" : "text-muted-foreground hover:text-foreground",
          )}
        >
          {o.icon}
          {o.label}
        </button>
      ))}
    </div>
  );
}
