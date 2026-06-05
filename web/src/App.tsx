import { Routes, Route, Navigate } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { AppLayout } from "@/components/AppLayout";
import { PlainLayout } from "@/components/PlainLayout";
import { RequireAdmin, RequireAuth, RequireTenant } from "@/components/guards";
import { BreadcrumbProvider } from "@/lib/breadcrumbs";
import { Login } from "@/pages/Login";
import { Register } from "@/pages/Register";
import { ForgotPassword } from "@/pages/ForgotPassword";
import { ResetPassword } from "@/pages/ResetPassword";
import { VerifyEmail } from "@/pages/VerifyEmail";
import { SSOCallback } from "@/pages/SSOCallback";
import { SelectTenant } from "@/pages/SelectTenant";
import { Profile } from "@/pages/Profile";
import { Dashboard } from "@/pages/Dashboard";
import { Runs } from "@/pages/Runs";
import { Suites } from "@/pages/Suites";
import { SuiteDetail } from "@/pages/SuiteDetail";
import { Quotas } from "@/pages/Quotas";
import { NewRun } from "@/pages/NewRun";
import { SuiteWizard } from "@/pages/SuiteWizard";
import { RunDetail } from "@/pages/RunDetail";
import { Compare } from "@/pages/Compare";
import { SharedRun } from "@/pages/SharedRun";
import { Landing } from "@/pages/Landing";
import { DatabasePresets } from "@/pages/library/DatabasePresets";
import { DatabasePresetForm } from "@/pages/library/DatabasePresetForm";
import { DatabasePresetDetail } from "@/pages/library/DatabasePresetDetail";
import { WorkloadPresets } from "@/pages/library/WorkloadPresets";
import { WorkloadPresetForm } from "@/pages/library/WorkloadPresetForm";
import { WorkloadPresetDetail } from "@/pages/library/WorkloadPresetDetail";
import { TestPresets } from "@/pages/library/TestPresets";
import { TestPresetForm } from "@/pages/library/TestPresetForm";
import { TestPresetDetail } from "@/pages/library/TestPresetDetail";
import { Packages } from "@/pages/library/Packages";
import { PackageUploadForm } from "@/pages/library/PackageUploadForm";
import { PackageDetail } from "@/pages/library/PackageDetail";
import { AdminAccounts } from "@/pages/admin/AdminAccounts";
import { AdminSystemSettings } from "@/pages/admin/AdminSystemSettings";
import { AdminIdentityProviders } from "@/pages/admin/AdminIdentityProviders";
import { AdminRegistrationRequests } from "@/pages/admin/AdminRegistrationRequests";
import { Orgs } from "@/pages/orgs/Orgs";
import { OrgDetail } from "@/pages/orgs/OrgDetail";

function Loading() {
  return (
    <div className="flex h-screen items-center justify-center bg-background text-sm text-muted-foreground">
      Loading...
    </div>
  );
}

// Where "/" and unknown paths land (logo/brand click too): unauthenticated ->
// login; otherwise the user's first org dashboard (by slug), else the
// Organizations area, else login. This matches Variant D's default-landing rule.
function RootRedirect() {
  const { user } = useAuth();
  if (!user) return <Navigate to="/login" replace />;
  const first = user.tenants[0];
  if (first) return <Navigate to={`/t/${first.slug}`} replace />;
  return <Navigate to="/orgs" replace />;
}

// "/" — public landing for visitors; authenticated users skip straight to their
// first org dashboard (RootRedirect handles the signed-in routing).
function RootEntry() {
  const { user } = useAuth();
  if (!user) return <Landing />;
  return <RootRedirect />;
}

export default function App() {
  const { isLoading } = useAuth();

  if (isLoading) return <Loading />;

  return (
    <BreadcrumbProvider>
      <Routes>
        {/* Public landing at root (unauthenticated). */}
        <Route path="/" element={<RootEntry />} />
        {/* Landing is always reachable here, even when signed in (root
            redirects authenticated users straight to their tenant). */}
        <Route path="/landing" element={<Landing />} />

        <Route path="/login" element={<Login />} />

        {/* Public auth flows (outside RequireAuth, next to /login). */}
        <Route path="/register" element={<Register />} />
        <Route path="/forgot-password" element={<ForgotPassword />} />
        <Route path="/reset-password" element={<ResetPassword />} />
        <Route path="/verify-email" element={<VerifyEmail />} />
        <Route path="/sso/callback" element={<SSOCallback />} />

        {/* Public, unauthenticated share view (outside RequireAuth). */}
        <Route path="/shared/:token" element={<SharedRun />} />

        <Route element={<RequireAuth />}>
          <Route path="/select-tenant" element={<SelectTenant />} />

          {/* Outside-tenant: header-only shell. */}
          <Route element={<PlainLayout />}>
            <Route path="/profile" element={<Profile />} />
            {/* Organizations area — org management lives here, reached from the
                account menu. Single-org sub-nav sections are role-gated inside. */}
            <Route path="/orgs" element={<Orgs />} />
            <Route path="/orgs/:slug" element={<OrgDetail />} />
            <Route element={<RequireAdmin />}>
              <Route
                path="/admin"
                element={<Navigate to="/admin/accounts" replace />}
              />
              <Route path="/admin/accounts" element={<AdminAccounts />} />
              <Route
                path="/admin/registration-requests"
                element={<AdminRegistrationRequests />}
              />
              <Route path="/admin/system" element={<AdminSystemSettings />} />
              <Route
                path="/admin/identity-providers"
                element={<AdminIdentityProviders />}
              />
              <Route
                path="/admin/users"
                element={<Navigate to="/admin/accounts" replace />}
              />
              <Route
                path="/admin/server"
                element={<Navigate to="/admin/system" replace />}
              />
              <Route
                path="/admin/tenants"
                element={<Navigate to="/orgs" replace />}
              />
            </Route>
          </Route>

          {/* Tenant scope: header + sidebar shell, routed by slug. */}
          <Route path="/t/:slug" element={<RequireTenant />}>
            <Route element={<AppLayout />}>
              <Route index element={<Dashboard />} />
              <Route path="runs" element={<Runs />} />
              <Route path="runs/new" element={<NewRun />} />
              <Route path="runs/:id" element={<RunDetail />} />
              <Route path="compare" element={<Compare />} />
              <Route path="suites" element={<Suites />} />
              <Route path="suites/new" element={<SuiteWizard />} />
              <Route path="suites/:id/edit" element={<SuiteWizard />} />
              <Route path="suites/:id" element={<SuiteDetail />} />
              <Route path="quotas" element={<Quotas />} />
              {/* Library — 3 preset tables + packages. */}
              <Route
                path="presets"
                element={<Navigate to="database" replace />}
              />
              <Route path="presets/database" element={<DatabasePresets />} />
              <Route path="presets/database/new" element={<DatabasePresetForm />} />
              <Route path="presets/database/:id" element={<DatabasePresetDetail />} />
              <Route path="presets/database/:id/edit" element={<DatabasePresetForm />} />
              <Route path="presets/workload" element={<WorkloadPresets />} />
              <Route path="presets/workload/new" element={<WorkloadPresetForm />} />
              <Route path="presets/workload/:id" element={<WorkloadPresetDetail />} />
              <Route path="presets/workload/:id/edit" element={<WorkloadPresetForm />} />
              <Route path="presets/test" element={<TestPresets />} />
              <Route path="presets/test/:id" element={<TestPresetDetail />} />
              <Route path="presets/test/:id/edit" element={<TestPresetForm />} />
              <Route path="packages" element={<Packages />} />
              <Route path="packages/new" element={<PackageUploadForm />} />
              <Route path="packages/:id" element={<PackageDetail />} />
            </Route>
          </Route>
        </Route>

        <Route path="*" element={<RootRedirect />} />
      </Routes>
    </BreadcrumbProvider>
  );
}
