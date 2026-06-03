// Shared chrome for the four Library pages (Database / Workload / Test presets
// + Packages). A title, a route-driven tab strip (so Back works and each tab is
// addressable per the router foundation), and a slot for the page's table. The
// "3 PRESET tabs + PACKAGES" sit on the same dark tokens as the Test Runs page.

import type { ReactNode } from "react";
import { NavLink } from "@/lib/router";
import { cn } from "@/lib/utils";

const TABS: { to: string; label: string }[] = [
  { to: "/presets/database", label: "Database Presets" },
  { to: "/presets/workload", label: "Workload Presets" },
  { to: "/presets/test", label: "Test Presets" },
  { to: "/packages", label: "Packages" },
];

export function LibraryShell({
  shownCount,
  children,
}: {
  /** "N shown" count for the active table; omitted while loading. */
  shownCount?: number;
  children: ReactNode;
}) {
  return (
    <div className="p-5 flex flex-col gap-4 h-full min-h-0">
      <div className="flex items-center gap-3">
        <h1 className="text-base font-semibold font-mono tracking-tight">
          Library
        </h1>
        {shownCount !== undefined && (
          <span className="text-[11px] text-zinc-600 font-mono tabular-nums">
            {shownCount} shown
          </span>
        )}
      </div>

      {/* Route-driven tab strip. */}
      <div className="flex items-center gap-1 border-b border-zinc-800/80">
        {TABS.map((tab) => (
          <NavLink
            key={tab.to}
            to={tab.to}
            className={({ isActive }) =>
              cn(
                "px-3 py-1.5 -mb-px text-xs font-mono border-b-2 transition-colors",
                isActive
                  ? "border-primary text-foreground"
                  : "border-transparent text-zinc-500 hover:text-zinc-300",
              )
            }
          >
            {tab.label}
          </NavLink>
        ))}
      </div>

      <div className="flex-1 min-h-0">{children}</div>
    </div>
  );
}
