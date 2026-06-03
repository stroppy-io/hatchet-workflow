// Auth/session surface for the rebuilt shell.
//
// This is the contract the shell talks to. The real implementation will wire
// these calls to the backend (connectrpc / REST). For now, a single mock gate
// in `main.tsx` can inject canned data so the shell renders without a backend.

export type TenantRole = "viewer" | "operator" | "owner";

/** A tenant the user belongs to, keyed by SLUG for routing (`/t/:slug`). */
export interface SessionTenant {
  id: string;
  slug: string;
  name: string;
  role: TenantRole;
}

/** The authenticated user + their tenant memberships. */
export interface SessionUser {
  id: string;
  username: string;
  /** Optional display name; falls back to username when absent. */
  displayName?: string;
  email?: string;
  /** Platform admin (cross-tenant). Mirrors Account.is_admin. */
  isAdmin: boolean;
  tenants: SessionTenant[];
}

/**
 * AuthProvider abstracts session retrieval + logout so the shell never imports
 * mock or backend specifics directly. The real backend provider replaces the
 * mock one; the shell only ever depends on this interface.
 */
export interface AuthProvider {
  /** Resolve the current session, or null if not authenticated. */
  getSession(): Promise<SessionUser | null>;
  /** Tear down the session (best-effort). */
  logout(): Promise<void>;
}

/**
 * Real backend provider. TODO(real-api): wire to the connect auth client.
 * Throws until implemented so a missing-backend misconfig is loud, not silent.
 */
const realAuthProvider: AuthProvider = {
  async getSession() {
    throw new Error(
      "real AuthProvider not wired yet — run with VITE_MOCK=1 to preview the shell",
    );
  },
  async logout() {
    /* no-op until wired */
  },
};

// --- Provider injection -----------------------------------------------------
//
// The active provider defaults to the real backend. The mock (and ONLY the
// mock) overrides it via setAuthProvider() from the single gate in main.tsx.
// To remove the mock: delete src/mock/ and the gated call in main.tsx.

let active: AuthProvider = realAuthProvider;

export function setAuthProvider(provider: AuthProvider): void {
  active = provider;
}

export function getAuthProvider(): AuthProvider {
  return active;
}
