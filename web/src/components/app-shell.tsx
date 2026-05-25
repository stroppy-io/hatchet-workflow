import { Suspense } from "react";
import {
  Activity,
  Boxes,
  Building2,
  Database,
  FlaskConical,
  GitCompare,
  KeyRound,
  Layers,
  List,
  Loader2,
  Package,
  Server,
  Settings,
  Settings2,
  ShieldCheck,
  Users,
  Webhook,
  type LucideIcon,
} from "lucide-react";
import { NavLink, Outlet, useNavigate } from "react-router-dom";

import { useAuth } from "@/contexts/auth-context";
import { useTenantId } from "@/hooks/use-tenant-id";
import { canAccess, roleLabel, ROLE_ADMIN, ROLE_OWNER, ROLE_VIEWER, type TenantRole } from "@/lib/rbac";
import { tenantPath } from "@/lib/routes";

type NavItem = {
  label: string;
  icon: LucideIcon;
  minRole: TenantRole;
  path: string;
  platformOnly?: boolean;
};

const navSections: { label: string; items: NavItem[] }[] = [
  {
    label: "Workspace",
    items: [
      { label: "Runs", icon: List, path: "/runs", minRole: ROLE_VIEWER },
      { label: "Compare", icon: GitCompare, path: "/compare", minRole: ROLE_VIEWER },
      { label: "Suites", icon: Boxes, path: "/suites", minRole: ROLE_VIEWER },
      { label: "Suite runs", icon: Activity, path: "/suite-runs", minRole: ROLE_VIEWER },
    ],
  },
  {
    label: "Presets",
    items: [
      { label: "Database", icon: Database, path: "/presets/database", minRole: ROLE_VIEWER },
      { label: "Workload", icon: FlaskConical, path: "/presets/workload", minRole: ROLE_VIEWER },
      { label: "Test", icon: Layers, path: "/presets/test", minRole: ROLE_VIEWER },
    ],
  },
  {
    label: "Tenant",
    items: [
      { label: "Packages", icon: Package, path: "/packages", minRole: ROLE_ADMIN },
      { label: "Webhooks", icon: Webhook, path: "/webhooks", minRole: ROLE_ADMIN },
      { label: "Settings", icon: Settings, path: "/settings", minRole: ROLE_OWNER },
      { label: "Inventory", icon: Server, path: "/inventory", minRole: ROLE_OWNER },
      { label: "API Tokens", icon: KeyRound, path: "/tokens", minRole: ROLE_OWNER },
      { label: "Members", icon: Users, path: "/members", minRole: ROLE_OWNER },
    ],
  },
  {
    label: "Platform",
    items: [
      { label: "Settings", icon: Settings2, path: "/platform/settings", minRole: ROLE_OWNER, platformOnly: true },
      { label: "Accounts", icon: ShieldCheck, path: "/platform/accounts", minRole: ROLE_OWNER, platformOnly: true },
      { label: "Tenants", icon: Building2, path: "/platform/tenants", minRole: ROLE_OWNER, platformOnly: true },
    ],
  },
];

export function AppShell() {
  const tenantId = useTenantId();
  const navigate = useNavigate();
  const { account, tenants, logout, roleForTenant } = useAuth();
  const role = roleForTenant(tenantId);
  const visibleSections = navSections
    .map((section) => ({
      ...section,
      items: section.items.filter((item) => (item.platformOnly ? account?.isAdmin : canAccess(role, item.minRole as TenantRole))),
    }))
    .filter((section) => section.items.length > 0);

  return (
    <div className="flex h-screen overflow-hidden bg-background text-foreground">
      <aside className="flex w-56 shrink-0 flex-col border-r bg-[#080808]">
        <div className="border-b px-3 py-3">
          <NavLink to={tenantPath(tenantId, "/runs")} className="flex items-center gap-2 text-sm font-semibold">
            <Activity className="size-4 text-primary" />
            Stroppy Cloud
          </NavLink>
          <div className="mt-2.5">
            <label className="mb-1 block text-[11px] text-muted-foreground">Tenant</label>
            <select
              className="w-full rounded-md border bg-background px-2 py-1.5 text-xs"
              value={tenantId}
              onChange={(event) => navigate(tenantPath(event.target.value, "/runs"))}
            >
              {tenants.map((tenant) => {
                const id = tenant.entity?.id?.value ?? "";
                return (
                  <option key={id} value={id}>
                    {tenant.name || id}
                  </option>
                );
              })}
            </select>
          </div>
          <div className="mt-2 flex items-center justify-between text-[11px]">
            <span className="text-muted-foreground">Role</span>
            <span className="font-mono text-primary">{roleLabel(role)}</span>
          </div>
        </div>

        <nav className="flex-1 space-y-3 overflow-y-auto p-2">
          {visibleSections.map((section) => (
            <div key={section.label}>
              <div className="mb-0.5 px-2 text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">{section.label}</div>
              <div className="space-y-0.5">
                {section.items.map((item) => (
                <NavLink
                  key={item.path}
                  to={tenantPath(tenantId, item.path)}
                  className={({ isActive }) =>
                    [
                      "flex items-center gap-2 rounded-md px-2 py-1.5 text-sm",
                      isActive ? "bg-muted text-foreground" : "text-muted-foreground hover:bg-muted/40 hover:text-foreground",
                    ].join(" ")
                  }
                >
                  <item.icon className="size-4 shrink-0" />
                  {item.label}
                </NavLink>
                ))}
              </div>
            </div>
          ))}
        </nav>

        <div className="border-t p-3">
          <div className="truncate text-xs">{account?.nickname || account?.email}</div>
          <div className="truncate text-[11px] text-muted-foreground">{account?.email}</div>
          <button className="mt-2 w-full rounded-md border px-2 py-1.5 text-xs text-muted-foreground hover:text-foreground" type="button" onClick={logout}>
            Log out
          </button>
        </div>
      </aside>

      <main className="min-w-0 flex-1 overflow-y-auto">
        <Suspense
          fallback={
            <div className="flex items-center justify-center gap-2 p-12 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" />
              Loading…
            </div>
          }
        >
          <Outlet />
        </Suspense>
      </main>
    </div>
  );
}
