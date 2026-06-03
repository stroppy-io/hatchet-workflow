import * as React from "react";
import { cn } from "@/lib/utils";

// Shared section surface for the dashboard. A bordered card with a font-mono
// uppercase header (the existing label style) and optional right-side action,
// giving each block a clear, elevated boundary over the page background.

/** The font-mono uppercase section label used across the dashboard. */
export function SectionLabel({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "text-[10px] font-mono uppercase tracking-wider text-zinc-500",
        className,
      )}
    >
      {children}
    </div>
  );
}

/**
 * Panel — a titled card surface. `bodyClassName` controls inner padding so
 * list-style panels (flush rows) and chart panels (padded) can differ.
 */
export function Panel({
  label,
  action,
  children,
  className,
  bodyClassName,
}: {
  label: React.ReactNode;
  action?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  bodyClassName?: string;
}) {
  return (
    <div
      className={cn(
        "flex flex-col overflow-hidden border border-border bg-card/60 shadow-sm backdrop-blur-[1px]",
        className,
      )}
    >
      <div className="flex items-center justify-between border-b border-border/70 px-4 py-2.5">
        <SectionLabel>{label}</SectionLabel>
        {action}
      </div>
      <div className={cn("flex-1", bodyClassName ?? "p-4")}>{children}</div>
    </div>
  );
}
