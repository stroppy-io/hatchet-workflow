import { NavLink, Outlet, useNavigate } from "react-router-dom";

import { useAuth } from "@/contexts/auth-context";
import { useTenantId } from "@/hooks/use-tenant-id";
import { canAccess, roleLabel, ROLE_ADMIN, ROLE_OWNER, ROLE_VIEWER, type TenantRole } from "@/lib/rbac";
import { tenantPath } from "@/lib/routes";

type NavItem = {
  label: string;
  minRole: TenantRole;
  path: string;
  platformOnly?: boolean;
};

const navSections: { label: string; items: NavItem[] }[] = [
  {
    label: "Workspace",
    items: [
      { label: "Wizard", path: "/wizard", minRole: ROLE_VIEWER },
      { label: "Runs", path: "/runs", minRole: ROLE_VIEWER },
      { label: "Suites", path: "/suites", minRole: ROLE_VIEWER },
      { label: "Suite runs", path: "/suite-runs", minRole: ROLE_VIEWER },
    ],
  },
  {
    label: "Presets",
    items: [
      { label: "Database", path: "/presets/database", minRole: ROLE_VIEWER },
      { label: "Workload", path: "/presets/workload", minRole: ROLE_VIEWER },
      { label: "Test", path: "/presets/test", minRole: ROLE_VIEWER },
    ],
  },
  {
    label: "Tenant",
    items: [
      { label: "Packages", path: "/packages", minRole: ROLE_ADMIN },
      { label: "Webhooks", path: "/webhooks", minRole: ROLE_ADMIN },
      { label: "Settings", path: "/settings", minRole: ROLE_OWNER },
      { label: "Inventory", path: "/inventory", minRole: ROLE_OWNER },
      { label: "API Tokens", path: "/tokens", minRole: ROLE_OWNER },
      { label: "Members", path: "/members", minRole: ROLE_OWNER },
    ],
  },
  {
    label: "Platform",
    items: [
      { label: "Settings", path: "/platform/settings", minRole: ROLE_OWNER, platformOnly: true },
      { label: "Accounts", path: "/platform/accounts", minRole: ROLE_OWNER, platformOnly: true },
      { label: "Tenants", path: "/platform/tenants", minRole: ROLE_OWNER, platformOnly: true },
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
    <div className="flex min-h-screen bg-background text-foreground">
      <aside className="flex w-64 shrink-0 flex-col border-r bg-[#070708]">
        <div className="border-b px-4 py-4">
          <NavLink to={tenantPath(tenantId, "/wizard")} className="text-sm font-semibold">
            Stroppy Cloud
          </NavLink>
          <div className="mt-4">
            <label className="mb-1 block text-[11px] text-muted-foreground">Tenant</label>
            <select
              className="w-full border bg-background px-2 py-2 text-xs"
              value={tenantId}
              onChange={(event) => navigate(tenantPath(event.target.value, "/wizard"))}
            >
              {tenants.map((tenant) => {
                const id = tenant.entity?.id?.value ?? "";
                return (
                  <option key={id} value={id}>
                    {id}
                  </option>
                );
              })}
            </select>
          </div>
          <div className="mt-3 flex items-center justify-between text-[11px]">
            <span className="text-muted-foreground">Role</span>
            <span className="font-mono text-primary">{roleLabel(role)}</span>
          </div>
        </div>

        <nav className="flex-1 space-y-4 overflow-y-auto p-3">
          {visibleSections.map((section) => (
            <div key={section.label}>
              <div className="mb-1 px-3 text-[11px] font-medium uppercase tracking-normal text-muted-foreground">{section.label}</div>
              <div className="space-y-1">
                {section.items.map((item) => (
                <NavLink
                  key={item.path}
                  to={tenantPath(tenantId, item.path)}
                  className={({ isActive }) =>
                    [
                      "block px-3 py-2 text-sm",
                      isActive ? "bg-muted text-foreground" : "text-muted-foreground hover:bg-muted/40 hover:text-foreground",
                    ].join(" ")
                  }
                >
                  {item.label}
                </NavLink>
                ))}
              </div>
            </div>
          ))}
        </nav>

        <div className="border-t p-4">
          <div className="truncate text-sm">{account?.nickname || account?.email}</div>
          <div className="truncate text-xs text-muted-foreground">{account?.email}</div>
          <button className="mt-3 w-full border px-3 py-2 text-sm text-muted-foreground hover:text-foreground" type="button" onClick={logout}>
            Log out
          </button>
        </div>
      </aside>

      <main className="min-w-0 flex-1">
        <Outlet />
      </main>
    </div>
  );
}
