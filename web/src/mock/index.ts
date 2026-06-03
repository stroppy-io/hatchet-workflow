// TEMPORARY mock — MUST be deleted before real API wiring; do not build on this.
//
// Single entry point for all mock wiring. main.tsx calls installMock() behind
// the VITE_MOCK flag. Removing the mock = delete src/mock/ and the one gated
// call in main.tsx; nothing else references this directory.

import { setAuthProvider } from "@/services/auth";
import { setAccountProvider } from "@/services/account";
import { setAdminProvider } from "@/services/admin";
import { setDashboardProvider } from "@/services/dashboard";
import { setOrgProvider } from "@/services/org";
import { setRunsProvider } from "@/services/runs";
import { setSuitesProvider } from "@/services/suites";
import { setWizardProvider } from "@/services/wizard";
import { setPresetProvider } from "@/services/preset";
import { setPackagesProvider } from "@/services/packages";
import { setStroppyProvider } from "@/services/stroppy";
import { mockAuthProvider } from "@/mock/auth";
import { mockAccountProvider } from "@/mock/account";
import { mockAdminProvider } from "@/mock/admin";
import { mockDashboardProvider } from "@/mock/dashboard";
import { mockOrgProvider } from "@/mock/org";
import { mockRunsProvider } from "@/mock/runs";
import { mockSuitesProvider } from "@/mock/suites";
import { mockWizardProvider } from "@/mock/wizard";
import { mockPresetProvider } from "@/mock/preset";
import { mockPackagesProvider } from "@/mock/packages";
import { mockStroppyProvider } from "@/mock/stroppy";

export function installMock(): void {
  setAuthProvider(mockAuthProvider);
  setAccountProvider(mockAccountProvider);
  setAdminProvider(mockAdminProvider);
  setDashboardProvider(mockDashboardProvider);
  setOrgProvider(mockOrgProvider);
  setRunsProvider(mockRunsProvider);
  setSuitesProvider(mockSuitesProvider);
  setWizardProvider(mockWizardProvider);
  setPresetProvider(mockPresetProvider);
  setPackagesProvider(mockPackagesProvider);
  setStroppyProvider(mockStroppyProvider);
}
