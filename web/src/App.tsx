import { lazy, Suspense } from "react";
import { Loader2 } from "lucide-react";
import { Navigate, Route, Routes } from "react-router-dom";

import { AppShell } from "@/components/app-shell";
import { ProtectedRoute } from "@/components/protected-route";
import { InventoryPage } from "@/pages/inventory-page";
import { LoginPage } from "@/pages/login-page";
import { NotFoundPage } from "@/pages/not-found-page";
import { PlatformSettingsPage } from "@/pages/platform-settings-page";
import { DatabasePresetsPage, TestPresetsPage, WorkloadPresetsPage } from "@/pages/preset-card-pages";
import { PlaceholderPage } from "@/pages/placeholder-page";
import { SettingsPage } from "@/pages/settings-page";
import { SuitesPage } from "@/pages/suite-card-page";
import { RunsPage, SuiteRunsPage } from "@/pages/table-pages";
import { MembersPage } from "@/pages/members-page";
import { PackagesPage } from "@/pages/packages-page";
import { PlatformAccountsPage } from "@/pages/platform-accounts-page";
import { PlatformTenantsPage } from "@/pages/platform-tenants-page";
import { TokensPage } from "@/pages/tokens-page";
import { WebhooksPage } from "@/pages/webhooks-page";
import { TenantHomePage } from "@/pages/tenant-home-page";

// Heavy pages (CodeMirror editors, xyflow graph, log viewer) are code-split so
// they don't bloat the initial bundle.
const WizardPage = lazy(() => import("@/pages/wizard-page").then((m) => ({ default: m.WizardPage })));
const PresetDatabasePage = lazy(() => import("@/pages/preset-database-page").then((m) => ({ default: m.PresetDatabasePage })));
const PresetWorkloadPage = lazy(() => import("@/pages/preset-workload-page").then((m) => ({ default: m.PresetWorkloadPage })));
const PresetTestPage = lazy(() => import("@/pages/preset-test-page").then((m) => ({ default: m.PresetTestPage })));
const RunDetailPage = lazy(() => import("@/pages/run-detail-page").then((m) => ({ default: m.RunDetailPage })));
const ComparePage = lazy(() => import("@/pages/compare-page").then((m) => ({ default: m.ComparePage })));
const SharedRunPage = lazy(() => import("@/pages/shared-run-page").then((m) => ({ default: m.SharedRunPage })));
const SuiteDetailPage = lazy(() => import("@/pages/suite-detail-page").then((m) => ({ default: m.SuiteDetailPage })));
const SuiteRunDetailPage = lazy(() => import("@/pages/suite-run-detail-page").then((m) => ({ default: m.SuiteRunDetailPage })));

const DEFAULT_TENANT_ID = "default";

function PageFallback() {
  return (
    <div className="flex items-center justify-center gap-2 p-12 text-sm text-muted-foreground">
      <Loader2 className="size-4 animate-spin" />
      Loading…
    </div>
  );
}

export function App() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to={`/t/${DEFAULT_TENANT_ID}/wizard`} replace />} />
      <Route path="/login" element={<LoginPage />} />
      <Route
        path="/share/:token"
        element={
          <Suspense fallback={<PageFallback />}>
            <SharedRunPage />
          </Suspense>
        }
      />
      <Route element={<ProtectedRoute />}>
        <Route path="/t/:tenantId" element={<AppShell />}>
          <Route index element={<TenantHomePage />} />
          <Route path="wizard" element={<WizardPage />} />
          <Route path="runs" element={<RunsPage />} />
          <Route path="runs/:runId" element={<RunDetailPage />} />
          <Route path="compare" element={<ComparePage />} />
          <Route path="presets" element={<Navigate to="database" replace />} />
          <Route path="presets/database" element={<DatabasePresetsPage />} />
          <Route path="presets/database/:presetId" element={<PresetDatabasePage />} />
          <Route path="presets/workload" element={<WorkloadPresetsPage />} />
          <Route path="presets/workload/:presetId" element={<PresetWorkloadPage />} />
          <Route path="presets/test" element={<TestPresetsPage />} />
          <Route path="presets/test/:presetId" element={<PresetTestPage />} />
          <Route path="suites" element={<SuitesPage />} />
          <Route path="suites/:suiteId" element={<SuiteDetailPage />} />
          <Route path="suite-runs" element={<SuiteRunsPage />} />
          <Route path="suite-runs/:suiteRunId" element={<SuiteRunDetailPage />} />
          <Route path="packages" element={<PackagesPage />} />
          <Route path="webhooks" element={<WebhooksPage />} />
          <Route path="settings" element={<SettingsPage />} />
          <Route path="inventory" element={<InventoryPage />} />
          <Route path="tokens" element={<TokensPage />} />
          <Route path="members" element={<MembersPage />} />
          <Route path="platform" element={<PlaceholderPage title="Platform Admin" />} />
          <Route path="platform/settings" element={<PlatformSettingsPage />} />
          <Route path="platform/accounts" element={<PlatformAccountsPage />} />
          <Route path="platform/tenants" element={<PlatformTenantsPage />} />
        </Route>
      </Route>
      <Route path="*" element={<NotFoundPage />} />
    </Routes>
  );
}
