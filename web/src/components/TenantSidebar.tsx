import {
  LayoutDashboard,
  List,
  Play,
  GitCompare,
  Boxes,
  Database,
  Gauge,
  FlaskConical,
  Package,
  Activity,
  Trophy,
  Share2,
  Star,
  Terminal,
  type LucideIcon,
} from "lucide-react";
import { NavLink } from "@/lib/router";
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
      { to: "/suites", icon: Boxes, label: "Suites", minLevel: 1 },
      { to: "/runs", icon: List, label: "Test Runs", minLevel: 1 },
      { to: "/favorites", icon: Star, label: "Favorites", minLevel: 1 },
    ],
  },
  {
    label: "Actions",
    items: [
      { to: "/runs/new", icon: Play, label: "New Run", minLevel: 2 },
      { to: "/compare", icon: GitCompare, label: "Compare", minLevel: 1 },
      { to: "/leaderboard", icon: Trophy, label: "Leaderboard", minLevel: 1 },
      { to: "/shares", icon: Share2, label: "Shares", minLevel: 1 },
      { to: "/shell", icon: Terminal, label: "Agent Shell", minLevel: 2 },
    ],
  },
  {
    label: "Library",
    items: [
      { to: "/presets/database", icon: Database, label: "Database Presets", minLevel: 1 },
      { to: "/presets/workload", icon: Gauge, label: "Workload Presets", minLevel: 1 },
      { to: "/presets/test", icon: FlaskConical, label: "Test Presets", minLevel: 1 },
      { to: "/packages", icon: Package, label: "Packages", minLevel: 1 },
    ],
  },
];

export function TenantSidebar() {
  const { user } = useAuth();
  const slug = useTenantSlug();
  const tenant = user?.tenants.find((t) => t.slug === slug);
  const level = user?.isAdmin ? 99 : tenant ? roleLevel[tenant.role] : 0;

  return (
    <aside className="flex w-48 shrink-0 flex-col border-r border-border bg-[#080808]">
      <nav className="flex-1 overflow-y-auto py-2">
        {navGroups.map((group) => {
          const visible = group.items.filter((it) => level >= it.minLevel);
          if (visible.length === 0) return null;
          return (
            <div key={group.label} className="mb-2">
              <div className="px-4 pb-1 pt-2 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
                {group.label}
              </div>
              {visible.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.to === "/"}
                  className={({ isActive }) =>
                    `flex items-center gap-2.5 px-4 py-1.5 text-sm transition-colors ${
                      isActive
                        ? "border-r-2 border-primary bg-muted text-foreground"
                        : "text-muted-foreground hover:bg-muted/50 hover:text-foreground"
                    }`
                  }
                >
                  <item.icon className="h-4 w-4" />
                  {item.label}
                </NavLink>
              ))}
            </div>
          );
        })}
      </nav>
      <div className="border-t border-border px-4 py-3">
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <div className="h-2 w-2 bg-success" />
          <span>Server</span>
        </div>
      </div>
    </aside>
  );
}
