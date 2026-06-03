// TEMPORARY mock — MUST be deleted before real API wiring; do not build on this.
//
// Canned suggested stroppy release tags so the Workload step's version pane is
// fully clickable under VITE_MOCK=1. Like mock/preset.ts this is read-only /
// isolated — it only answers listStroppyVersions(). The commit-hash free input
// in the UI is independent of this list.

import { type StroppyProvider } from "@/services/stroppy";

function delay(): Promise<void> {
  return new Promise((r) => setTimeout(r, 120));
}

const SUGGESTED = ["5.1.2", "5.1.1", "5.1.0", "5.0.0rc3", "nightly"];

export const mockStroppyProvider: StroppyProvider = {
  async listStroppyVersions(tenantSlug) {
    await delay();
    void tenantSlug;
    return [...SUGGESTED];
  },
};
