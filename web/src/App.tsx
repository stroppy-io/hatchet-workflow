import { Routes, Route, Navigate } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { AppLayout } from "@/components/AppLayout";
import { PlainLayout } from "@/components/PlainLayout";
import { RequireAdmin, RequireAuth, RequireTenant } from "@/components/guards";
import { BreadcrumbProvider } from "@/lib/breadcrumbs";
import { Login } from "@/pages/Login";
import { SelectTenant } from "@/pages/SelectTenant";
import { Profile } from "@/pages/Profile";
import { Dashboard } from "@/pages/Dashboard";
import { Runs } from "@/pages/Runs";
import { Suites } from "@/pages/Suites";
import { SuiteDetail } from "@/pages/SuiteDetail";
import { Quotas } from "@/pages/Quotas";
import { NewRun } from "@/pages/NewRun";
import { Placeholder } from "@/pages/Placeholder";
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

// Tenant-scoped placeholder pages — WORK ENTITIES ONLY. Real pages are rebuilt
// in later tasks. The index route (/t/:slug) is the Organization Dashboard,
// declared separately. Org settings/members/roles live under /orgs/:slug, NOT
// here. Account API tokens live under /profile.
const tenantPages: { path: string; title: string; crumbParam?: string }[] = [
  // `runs` (the index list) and `runs/new` (the wizard) are real pages now —
  // declared explicitly below. `suites` is also real now.
  { path: "runs/:id", title: "Run Detail", crumbParam: "id" },
  { path: "compare", title: "Compare" },
];

export default function App() {
  const { isLoading } = useAuth();

  if (isLoading) return <Loading />;

  return (
    <BreadcrumbProvider>
      <Routes>
        <Route path="/login" element={<Login />} />

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
              <Route path="/admin/system" element={<AdminSystemSettings />} />
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
              <Route path="suites" element={<Suites />} />
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
              <Route path="presets/test/new" element={<TestPresetForm />} />
              <Route path="presets/test/:id" element={<TestPresetDetail />} />
              <Route path="presets/test/:id/edit" element={<TestPresetForm />} />
              <Route path="packages" element={<Packages />} />
              <Route path="packages/new" element={<PackageUploadForm />} />
              <Route path="packages/:id" element={<PackageDetail />} />
              {tenantPages.map((p) => (
                <Route
                  key={p.path}
                  path={p.path}
                  element={
                    <Placeholder title={p.title} crumbParam={p.crumbParam} />
                  }
                />
              ))}
            </Route>
          </Route>
        </Route>

        <Route path="*" element={<RootRedirect />} />
      </Routes>
    </BreadcrumbProvider>
  );
}
