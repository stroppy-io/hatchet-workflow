import { Routes, Route, Navigate, useLocation, useNavigate } from "react-router-dom";
import { useEffect } from "react";
import { Layout } from "@/components/Layout";
import { ProtectedRoute } from "@/components/ProtectedRoute";
import { TenantGate } from "@/components/TenantGate";
import { Runs } from "@/pages/Runs";
import { NewRun } from "@/pages/NewRun";
import { RunDetail } from "@/pages/RunDetail";
import { Compare } from "@/pages/Compare";
import { SettingsPage } from "@/pages/Settings";
import { Presets } from "@/pages/Presets";
import { RunPresets } from "@/pages/RunPresets";
import { Suites } from "@/pages/Suites";
import { SuiteDetail } from "@/pages/SuiteDetail";
import { SuiteBuilder } from "@/pages/SuiteBuilder";
import { WorkloadEditor } from "@/pages/WorkloadEditor";
import { Packages } from "@/pages/Packages";
import { PresetDesigner } from "@/pages/PresetDesigner";
import { ServerHealth } from "@/pages/ServerHealth";
import { SharedRun } from "@/pages/SharedRun";

import { Login } from "@/pages/Login";
import { SelectTenant } from "@/pages/SelectTenant";
import { AdminTenants } from "@/pages/AdminTenants";
import { AdminUsers } from "@/pages/AdminUsers";
import { TenantMembers } from "@/pages/TenantMembers";
import { TenantTokens } from "@/pages/TenantTokens";
import { AuthProvider } from "@/contexts/AuthContext";
import { useAuth } from "@/hooks/useAuth";

/**
 * defaultLanding picks where to send an authenticated user landing at "/".
 * Priority: previously-selected tenant in localStorage > sole tenant >
 * select-tenant page > /admin/tenants for root with no tenants.
 */
function defaultLanding(user: ReturnType<typeof useAuth>["user"]): string {
  if (!user) return "/login";
  const stored = localStorage.getItem("stroppy.tenantId");
  if (stored && (user.is_root || user.tenants?.some((t) => t.id === stored))) {
    return `/t/${stored}/runs`;
  }
  const tenants = user.tenants ?? [];
  if (tenants.length === 1) return `/t/${tenants[0].id}/runs`;
  if (tenants.length > 1) return "/select-tenant";
  if (user.is_root) return "/admin/tenants";
  return "/select-tenant";
}

function AppRoutes() {
  const { isAuthenticated, isLoading, user } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();

  // Auto-pick tenant on first load when user lands somewhere that needs one.
  useEffect(() => {
    if (!user || !isAuthenticated) return;
    // Skip if path is already routed properly.
    const p = location.pathname;
    if (
      p === "/select-tenant" ||
      p.startsWith("/admin") ||
      p.startsWith("/t/") ||
      p.startsWith("/share/") ||
      p === "/login"
    ) {
      return;
    }
    // Otherwise resolve a landing.
    navigate(defaultLanding(user), { replace: true });
  }, [user, isAuthenticated, navigate, location.pathname]);

  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center bg-background text-sm text-muted-foreground">
        Loading...
      </div>
    );
  }

  // Share pages are always accessible, regardless of auth.
  if (location.pathname.startsWith("/share/")) {
    return (
      <Routes>
        <Route path="/share/:token" element={<SharedRun />} />
      </Routes>
    );
  }

  if (!isAuthenticated) {
    return (
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route
          path="*"
          element={
            <Navigate
              to={`/login?redirect=${encodeURIComponent(location.pathname)}`}
              replace
            />
          }
        />
      </Routes>
    );
  }

  return (
    <Routes>
      <Route path="/login" element={<Navigate to={defaultLanding(user)} replace />} />
      <Route path="/select-tenant" element={<SelectTenant />} />

      {/* Admin scope — no tenant in path (root operates cross-tenant). */}
      <Route element={<Layout />}>
        <Route element={<ProtectedRoute requireRoot />}>
          <Route path="/admin/tenants" element={<AdminTenants />} />
          <Route path="/admin/users" element={<AdminUsers />} />
          <Route path="/admin/server" element={<ServerHealth />} />
        </Route>
      </Route>

      {/* Tenant-scoped routes: /t/:tenantId/<entity> */}
      <Route path="/t/:tenantId" element={<TenantGate />}>
        <Route element={<Layout />}>
          <Route index element={<Navigate to="runs" replace />} />
          <Route path="runs" element={<Runs />} />
          <Route path="runs/:id" element={<RunDetail />} />
          <Route path="compare" element={<Compare />} />
          <Route path="packages" element={<Packages />} />
          <Route path="presets" element={<Presets />} />
          <Route path="run-presets" element={<RunPresets />} />
          <Route path="suites" element={<Suites />} />
          <Route path="suites/new" element={<SuiteBuilder />} />
          <Route path="suites/:id" element={<SuiteDetail />} />
          <Route path="suites/:id/edit" element={<SuiteBuilder />} />
          <Route path="suites/:id/items/new" element={<WorkloadEditor />} />
          <Route path="suites/:id/items/:itemId/edit" element={<WorkloadEditor />} />
          <Route path="settings" element={<SettingsPage />} />

          <Route element={<ProtectedRoute minRole="operator" />}>
            <Route path="runs/new" element={<NewRun />} />
            <Route path="presets/new" element={<PresetDesigner />} />
            <Route path="presets/:id/edit" element={<PresetDesigner />} />
          </Route>

          <Route element={<ProtectedRoute minRole="owner" />}>
            <Route path="members" element={<TenantMembers />} />
            <Route path="tokens" element={<TenantTokens />} />
          </Route>
        </Route>
      </Route>

      <Route path="/" element={<Navigate to={defaultLanding(user)} replace />} />
      <Route path="*" element={<Navigate to={defaultLanding(user)} replace />} />
    </Routes>
  );
}

export default function App() {
  return (
    <AuthProvider>
      <AppRoutes />
    </AuthProvider>
  );
}
