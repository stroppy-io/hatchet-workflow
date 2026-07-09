import { useEffect, useState } from "react";
import {
  LayoutDashboard,
  List,
  GitCompare,
  FileCode,
  Package,
  Library,
  Activity,
  PanelLeftClose,
  PanelLeftOpen,
  Pin,
  PinOff,
  type LucideIcon,
} from "lucide-react";
import { NavLink, useLocation } from "@/lib/router";
import { useAuth } from "@/hooks/useAuth";
import { useTenantSlug } from "@/lib/router";
import { roleLevel } from "@/lib/roles";

interface NavItem {
  to: string;
  icon: LucideIcon;
  label: string;
  minLevel: number; // 1=viewer, 2=operator, 3=owner
}

interface NavGroup {
  label: string;
  items: NavItem[];
}

// Per-tenant navigation — WORK ENTITIES ONLY. Org settings/members/tokens live
// in the Organizations area (/orgs/:slug), reached from the account menu, and
// are intentionally NOT in this sidebar. Platform-admin lives in the account
// menu. This is purely the tenant work scope (Overview / Tests / Library).
//
// Role-gating: authoring actions (New Run, preset authoring) require operator+
// (minLevel 2); read views are visible to viewers (minLevel 1).
const navGroups: NavGroup[] = [
  {
    label: "Overview",
    items: [
      { to: "/", icon: LayoutDashboard, label: "Dashboard", minLevel: 1 },
      { to: "/quotas", icon: Activity, label: "Quotas", minLevel: 1 },
    ],
  },
  {
    label: "Tests",
    items: [
      { to: "/recipes", icon: FileCode, label: "Recipes", minLevel: 1 },
      { to: "/recipes/runs", icon: List, label: "Runs", minLevel: 1 },
    ],
  },
  {
    label: "Actions",
    items: [{ to: "/compare", icon: GitCompare, label: "Compare", minLevel: 1 }],
  },
  {
    label: "Library",
    items: [
      { to: "/packages", icon: Package, label: "Packages", minLevel: 1 },
      { to: "/catalog", icon: Library, label: "Catalog", minLevel: 1 },
    ],
  },
];

const COLLAPSE_KEY = "tenant-sidebar-collapsed";
const HOVER_OPEN_KEY = "tenant-sidebar-hover-open";

export function TenantSidebar() {
  const { user } = useAuth();
  const slug = useTenantSlug();
  const { pathname } = useLocation();
  const tenant = user?.tenants.find((t) => t.slug === slug);
  const level = user?.isAdmin ? 99 : tenant ? roleLevel[tenant.role] : 0;
  const tenantPath = tenantRelativePath(pathname, slug);

  // collapsed = persistent rail mode (toggled by button). hovered = transient
  // expand-on-hover while collapsed (Supabase-style flyout, doesn't push content).
  const [collapsed, setCollapsed] = useState(() => {
    try {
      return localStorage.getItem(COLLAPSE_KEY) === "1";
    } catch {
      return false;
    }
  });
  // hoverOpen = whether collapsed sidebar expands on hover (default on).
  const [hoverOpen, setHoverOpen] = useState(() => {
    try {
      return localStorage.getItem(HOVER_OPEN_KEY) !== "0";
    } catch {
      return true;
    }
  });
  const [hovered, setHovered] = useState(false);
  useEffect(() => {
    try {
      localStorage.setItem(COLLAPSE_KEY, collapsed ? "1" : "0");
    } catch {
      /* ignore */
    }
  }, [collapsed]);
  useEffect(() => {
    try {
      localStorage.setItem(HOVER_OPEN_KEY, hoverOpen ? "1" : "0");
    } catch {
      /* ignore */
    }
  }, [hoverOpen]);

  const expanded = !collapsed || (hovered && hoverOpen);
  const flyout = collapsed && hovered && hoverOpen; // overlaying main content

  return (
    // Spacer reserves the rail/full width in flow; the <aside> is absolutely
    // positioned so the hover flyout overlays main instead of reflowing it.
    <div
      className={`relative shrink-0 transition-[width] duration-150 ${collapsed ? "w-14" : "w-48"}`}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      <aside
        className={`absolute inset-y-0 left-0 z-30 flex flex-col border-r border-border bg-[#080808] transition-[width] duration-150 ${
          expanded ? "w-48" : "w-14"
        } ${flyout ? "shadow-xl shadow-black/50" : ""}`}
      >
        <nav className="flex-1 overflow-y-auto overflow-x-hidden py-2">
          {navGroups.map((group) => {
            const visible = group.items.filter((it) => level >= it.minLevel);
            if (visible.length === 0) return null;
            return (
              <div key={group.label} className="mb-2">
                {expanded ? (
                  <div className="px-4 pb-1 pt-2 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
                    {group.label}
                  </div>
                ) : (
                  <div className="mx-3 mb-1 mt-2 border-t border-border/60" />
                )}
                {visible.map((item) => (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    end={item.to === "/"}
                    title={expanded ? undefined : item.label}
                    className={({ isActive }) =>
                      `flex items-center gap-2.5 py-1.5 text-sm transition-colors ${
                        expanded ? "px-4" : "justify-center px-0"
                      } ${
                        navItemActive(item.to, tenantPath, isActive)
                          ? "border-r-2 border-primary bg-muted text-foreground"
                          : "text-muted-foreground hover:bg-muted/50 hover:text-foreground"
                      }`
                    }
                  >
                    <item.icon className="h-4 w-4 shrink-0" />
                    {expanded && <span className="truncate">{item.label}</span>}
                  </NavLink>
                ))}
              </div>
            );
          })}
        </nav>
        <div className={`flex items-center gap-1 border-t border-border py-3 ${expanded ? "justify-between px-2" : "flex-col justify-center px-0"}`}>
          {expanded && (
            <div className="flex items-center gap-2 px-2 text-xs text-muted-foreground">
              <div className="h-2 w-2 shrink-0 bg-success" />
              <span>Server</span>
            </div>
          )}
          <div className={`flex items-center ${expanded ? "gap-1" : "flex-col gap-1"}`}>
            <button
              type="button"
              onClick={() => setHoverOpen((h) => !h)}
              title={hoverOpen ? "Disable open on hover" : "Enable open on hover"}
              className={`rounded p-1 transition-colors hover:bg-muted/50 ${
                hoverOpen ? "text-foreground" : "text-muted-foreground"
              }`}
            >
              {hoverOpen ? <Pin className="h-4 w-4" /> : <PinOff className="h-4 w-4" />}
            </button>
            <button
              type="button"
              onClick={() => setCollapsed((c) => !c)}
              title={collapsed ? "Pin sidebar open" : "Collapse sidebar"}
              className="rounded p-1 text-muted-foreground transition-colors hover:bg-muted/50 hover:text-foreground"
            >
              {collapsed ? (
                <PanelLeftOpen className="h-4 w-4" />
              ) : (
                <PanelLeftClose className="h-4 w-4" />
              )}
            </button>
          </div>
        </div>
      </aside>
    </div>
  );
}

function tenantRelativePath(pathname: string, slug?: string): string {
  if (!slug) return pathname || "/";
  const prefix = `/t/${slug}`;
  if (pathname === prefix) return "/";
  if (pathname.startsWith(prefix + "/")) return pathname.slice(prefix.length);
  return pathname || "/";
}

function navItemActive(to: string, tenantPath: string, routerActive: boolean): boolean {
  if (to === "/") return tenantPath === "/";
  // "/recipes" covers the list plus the editor (new/:id), but not the runs
  // sub-route (which has its own nav item below).
  if (to === "/recipes") {
    return (
      tenantPath === "/recipes" ||
      (tenantPath.startsWith("/recipes/") && !tenantPath.startsWith("/recipes/runs"))
    );
  }
  // "/recipes/runs" also covers run detail pages ("/runs/:id"), which are
  // reached from the Runs list.
  if (to === "/recipes/runs") {
    return tenantPath === "/recipes/runs" || tenantPath.startsWith("/runs/");
  }
  return routerActive;
}
