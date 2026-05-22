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
import {
  MembersPage,
  PackagesPage,
  PlatformAccountsPage,
  PlatformTenantsPage,
  RunsPage,
  SuiteRunsPage,
  TokensPage,
  WebhooksPage,
} from "@/pages/table-pages";
import { TenantHomePage } from "@/pages/tenant-home-page";
import { WizardPage } from "@/pages/wizard-page";

const DEFAULT_TENANT_ID = "default";

export function App() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to={`/t/${DEFAULT_TENANT_ID}/wizard`} replace />} />
      <Route path="/login" element={<LoginPage />} />
      <Route element={<ProtectedRoute />}>
        <Route path="/t/:tenantId" element={<AppShell />}>
          <Route index element={<TenantHomePage />} />
          <Route path="wizard" element={<WizardPage />} />
          <Route path="runs" element={<RunsPage />} />
          <Route path="runs/:runId" element={<PlaceholderPage title="Run details" />} />
          <Route path="presets" element={<Navigate to="database" replace />} />
          <Route path="presets/database" element={<DatabasePresetsPage />} />
          <Route path="presets/database/:presetId" element={<PlaceholderPage title="Database preset" />} />
          <Route path="presets/workload" element={<WorkloadPresetsPage />} />
          <Route path="presets/workload/:presetId" element={<PlaceholderPage title="Workload preset" />} />
          <Route path="presets/test" element={<TestPresetsPage />} />
          <Route path="presets/test/:presetId" element={<PlaceholderPage title="Test preset" />} />
          <Route path="suites" element={<SuitesPage />} />
          <Route path="suites/:suiteId" element={<PlaceholderPage title="Suite" />} />
          <Route path="suite-runs" element={<SuiteRunsPage />} />
          <Route path="suite-runs/:suiteRunId" element={<PlaceholderPage title="Suite run" />} />
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
