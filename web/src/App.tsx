import { lazy, Suspense } from "react";
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
import { Quotas } from "@/pages/Quotas";
import { Recipes } from "@/pages/Recipes";
import { RecipeEditor } from "@/pages/RecipeEditor";
import { RecipeRuns } from "@/pages/RecipeRuns";
import { RunDetail } from "@/pages/RunDetail";
import { Compare } from "@/pages/Compare";
import { SharedRun } from "@/pages/SharedRun";
import { Landing } from "@/pages/Landing";
import { Packages } from "@/pages/library/Packages";
import { PackageUploadForm } from "@/pages/library/PackageUploadForm";
import { PackageDetail } from "@/pages/library/PackageDetail";
import { AdminAccounts } from "@/pages/admin/AdminAccounts";
import { AdminSystemSettings } from "@/pages/admin/AdminSystemSettings";
import { AdminIdentityProviders } from "@/pages/admin/AdminIdentityProviders";
import { AdminRegistrationRequests } from "@/pages/admin/AdminRegistrationRequests";
import { AdminCatalog } from "@/pages/admin/AdminCatalog";
import { Orgs } from "@/pages/orgs/Orgs";
import { OrgDetail } from "@/pages/orgs/OrgDetail";
import { OrgCatalog } from "@/pages/catalog/OrgCatalog";
import { CatalogEntryEditor } from "@/pages/catalog/CatalogEntryEditor";

// LaunchForm pulls in LaunchFormRenderer -> (lazily) SchemaFormBody, which
// statically imports @stroppy-io/schemapb-react/@stroppy-io/schemapb — the
// ~22MB schemapb.wasm engine. lazy() here keeps LaunchForm's own module
// (and therefore that whole dependency chain) out of the main entry chunk;
// it's fetched as its own async chunk only when this route is visited. Do
// NOT change this to a static top-level import.
const LaunchForm = lazy(() => import("@/pages/LaunchForm"));

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
              <Route path="/admin/catalog" element={<AdminCatalog />} />
              <Route
                path="/admin/catalog/:kind/new"
                element={<CatalogEntryEditor scope="instance" />}
              />
              <Route
                path="/admin/catalog/:kind/:id/edit"
                element={<CatalogEntryEditor scope="instance" />}
              />
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
              <Route path="recipes" element={<Recipes />} />
              <Route path="recipes/new" element={<RecipeEditor />} />
              <Route path="recipes/runs" element={<RecipeRuns />} />
              <Route path="recipes/:id" element={<RecipeEditor />} />
              <Route
                path="recipes/:id/launch"
                element={
                  <Suspense fallback={<Loading />}>
                    <LaunchForm />
                  </Suspense>
                }
              />
              <Route path="runs/:id" element={<RunDetail />} />
              <Route path="compare" element={<Compare />} />
              <Route path="quotas" element={<Quotas />} />
              <Route path="packages" element={<Packages />} />
              <Route path="packages/new" element={<PackageUploadForm />} />
              <Route path="packages/:id" element={<PackageDetail />} />
              <Route path="catalog" element={<OrgCatalog />} />
              <Route
                path="catalog/:kind/new"
                element={<CatalogEntryEditor scope="org" />}
              />
              <Route
                path="catalog/:kind/:id/edit"
                element={<CatalogEntryEditor scope="org" />}
              />
            </Route>
          </Route>
        </Route>

        <Route path="*" element={<RootRedirect />} />
      </Routes>
    </BreadcrumbProvider>
  );
}
