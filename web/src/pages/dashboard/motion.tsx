import * as React from "react";
import { cn } from "@/lib/utils";

// Lightweight, dependency-free motion helpers for the dashboard. No animation
// library: a CSS keyframe (see index.css `.reveal-item`) drives the staggered
// entrance, and a rAF loop drives the stat count-up. Both respect
// prefers-reduced-motion.

function usePrefersReducedMotion(): boolean {
  const [reduced, setReduced] = React.useState(false);
  React.useEffect(() => {
    const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
    setReduced(mq.matches);
    const onChange = (e: MediaQueryListEvent) => setReduced(e.matches);
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);
  return reduced;
}

/**
 * Reveal — staggered fade/slide-up entrance. `index` drives the delay so a row
 * of children cascades in. Falls back to no-op under reduced motion.
 */
export function Reveal({
  index = 0,
  step = 60,
  className,
  children,
  as: Tag = "div",
}: {
  index?: number;
  step?: number;
  className?: string;
  children: React.ReactNode;
  as?: "div" | "section";
}) {
  return (
    <Tag
      className={cn("reveal-item", className)}
      style={{ "--reveal-delay": `${index * step}ms` } as React.CSSProperties}
    >
      {children}
    </Tag>
  );
}

/**
 * Animated number that counts up from 0 to `value` on mount. Honors
 * fractional values via `decimals`. Reduced motion jumps straight to the value.
 */
export function CountUp({
  value,
  decimals = 0,
  duration = 900,
  suffix = "",
  prefix = "",
  format,
}: {
  value: number;
  decimals?: number;
  duration?: number;
  suffix?: string;
  prefix?: string;
  /** Custom formatter for the integer-ish part; overrides decimals/locale. */
  format?: (n: number) => string;
}) {
  const reduced = usePrefersReducedMotion();
  const [display, setDisplay] = React.useState(reduced ? value : 0);

  React.useEffect(() => {
    if (reduced || duration <= 0) {
      setDisplay(value);
      return;
    }
    let raf = 0;
    const start = performance.now();
    const from = 0;
    const tick = (now: number) => {
      const t = Math.min(1, (now - start) / duration);
      // easeOutExpo — quick settle, professional feel.
      const eased = t === 1 ? 1 : 1 - Math.pow(2, -10 * t);
      setDisplay(from + (value - from) * eased);
      if (t < 1) raf = requestAnimationFrame(tick);
      else setDisplay(value);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [value, duration, reduced]);

  const text = format
    ? format(display)
    : display.toLocaleString(undefined, {
        minimumFractionDigits: decimals,
        maximumFractionDigits: decimals,
      });

  return (
    <span className="tabular-nums">
      {prefix}
      {text}
      {suffix}
    </span>
  );
}
