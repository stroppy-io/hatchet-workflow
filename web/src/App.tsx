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

function Loading() {
  return (
    <div className="flex h-screen items-center justify-center bg-background text-sm text-muted-foreground">
      Loading...
    </div>
  );
}

// Where "/" and unknown paths land once authenticated.
function RootRedirect() {
  const { user } = useAuth();
  if (user?.tenant_id) return <Navigate to={`/t/${user.tenant_id}`} replace />;
  if (user?.is_root) return <Navigate to="/admin/tenants" replace />;
  return <Navigate to="/select-tenant" replace />;
}

function AppRoutes() {
  const { isAuthenticated, isLoading, user, selectTenant } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();

  // After login with no tenant in context: auto-select the only tenant, or
  // route to the selector / admin. Tenant-scoped paths (/t/...) are handled by
  // TenantGate, so skip them here.
  useEffect(() => {
    if (!user || !isAuthenticated) return;
    const p = location.pathname;
    if (p === "/select-tenant" || p.startsWith("/admin") || p.startsWith("/t/")) return;
    if (user.tenant_id) return; // tenant already selected

    const tenants = user.tenants || [];
    if (tenants.length === 1) {
      selectTenant(tenants[0].id);
    } else if (tenants.length > 1) {
      navigate("/select-tenant", { replace: true });
    } else if (user.is_root) {
      navigate("/admin/tenants", { replace: true });
    } else {
      navigate("/select-tenant", { replace: true });
    }
  }, [user, isAuthenticated, selectTenant, navigate, location.pathname]);

  if (isLoading) return <Loading />;

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

  // Authenticated but no tenant chosen yet (and not on a route that handles
  // that itself): let the auto-select effect above run before redirecting.
  if (
    !user?.tenant_id &&
    location.pathname !== "/select-tenant" &&
    !location.pathname.startsWith("/admin") &&
    !location.pathname.startsWith("/t/")
  ) {
    return <Loading />;
  }

  return (
    <Routes>
      <Route path="/login" element={<RootRedirect />} />
      <Route path="/select-tenant" element={<SelectTenant />} />

      {/* System scope: root only, tenant-independent. */}
      <Route element={<Layout />}>
        <Route element={<ProtectedRoute requireRoot />}>
          <Route path="/admin/tenants" element={<AdminTenants />} />
          <Route path="/admin/users" element={<AdminUsers />} />
          <Route path="/admin/server" element={<ServerHealth />} />
        </Route>
      </Route>

      {/* Tenant scope: everything lives under /t/:tenantId. */}
      <Route path="/t/:tenantId" element={<TenantGate />}>
        <Route element={<Layout />}>
          {/* Everyone */}
          <Route index element={<Runs />} />
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

          {/* Operator+ */}
          <Route element={<ProtectedRoute minRole="operator" />}>
            <Route path="runs/new" element={<NewRun />} />
            <Route path="presets/new" element={<PresetDesigner />} />
            <Route path="presets/:id/edit" element={<PresetDesigner />} />
          </Route>

          {/* Owner+ */}
          <Route element={<ProtectedRoute minRole="owner" />}>
            <Route path="members" element={<TenantMembers />} />
            <Route path="tokens" element={<TenantTokens />} />
          </Route>
        </Route>
      </Route>

      <Route path="*" element={<RootRedirect />} />
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
