// Stroppy-version data surface for the Test Wizard's Workload step (pane 1).
//
// The Workload step is a three-pane progressive flow: pick a stroppy VERSION
// (a suggested release tag OR a commit hash), then edit the workload
// PARAMETERS, then the PROBE results. This module is the contract that first
// pane talks to — exactly like services/preset.ts / services/wizard.ts the
// page depends ONLY on this interface, never on the mock or the wire format.
//
// The chosen value sets domain.Workload.stroppy_version: a release tag is sent
// verbatim (e.g. "5.1.2"); a commit is sent as "commit:<sha7>" (matching the
// old NewRun's StroppyConfig.version convention + the backend's binary
// resolver). Pane 1 only PRODUCES the string; the wizard's existing
// patch({ workload }) ships it.
//
// The real provider calls cloud.v1.api.StroppyService.ListStroppyVersions. The
// mock gate in main.tsx injects a stateful mock via setStroppyProvider().

import { stroppyClient } from "@/services/client";
import { resolveTenantId } from "@/services/tenant";

/**
 * StroppyProvider abstracts the suggested-version listing so the Workload
 * step's version pane never imports mock or backend specifics. Keyed by tenant
 * SLUG to match runs.ts / wizard.ts; the real provider resolves slug ->
 * tenant_id at call time. The commit-hash free input is always available in
 * the UI regardless of this list.
 */
export interface StroppyProvider {
  /** ListStroppyVersions -> suggested release tags (newest first). */
  listStroppyVersions(tenantSlug: string): Promise<string[]>;
}

/** Format a commit sha as the wire value domain.Workload.stroppy_version wants. */
export function commitVersion(sha: string): string {
  const clean = sha.trim().toLowerCase().slice(0, 40).replace(/[^0-9a-f]/g, "");
  return clean ? `commit:${clean.slice(0, 7)}` : "";
}

/** True when a stroppy_version string is a commit pin (vs a release tag). */
export function isCommitVersion(version: string): boolean {
  return version.startsWith("commit:");
}

/** Extract the short sha from a commit:<sha> version (empty for release tags). */
export function commitSha(version: string): string {
  return isCommitVersion(version) ? version.slice("commit:".length) : "";
}

// --- Real backend provider ---------------------------------------------------

const realStroppyProvider: StroppyProvider = {
  async listStroppyVersions(tenantSlug) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { versions } = await stroppyClient.listStroppyVersions({ tenantId });
    return versions;
  },
};

// --- Provider injection ------------------------------------------------------
//
// Defaults to the real backend. The mock (and ONLY the mock) overrides it via
// setStroppyProvider() from the single gate in main.tsx.

let active: StroppyProvider = realStroppyProvider;

export function setStroppyProvider(provider: StroppyProvider): void {
  active = provider;
}

export function getStroppyProvider(): StroppyProvider {
  return active;
}
