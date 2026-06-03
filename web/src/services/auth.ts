// Auth/session surface for the rebuilt shell.
//
// The shell depends ONLY on the AuthProvider interface below; the real
// implementation talks to IamService over connect.

import { iamClient, getAccessToken } from "@/services/client";
import { Action } from "@/lib/proto/cloud/v1/iam/permission_pb";
import type { Tenant } from "@/lib/proto/cloud/v1/iam/tenant_pb";
import { applyTokens, clearTokens, getRefreshToken } from "@/services/tokens";

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
 * AuthProvider abstracts the session lifecycle so the shell never imports
 * backend specifics directly. The real backend provider talks to IamService.
 */
export interface RegisterInput {
  email: string;
  nickname: string;
  password: string;
}

export interface AuthProvider {
  /** Authenticate by login (nickname or email) + password; resolves the new session. */
  login(login: string, password: string): Promise<SessionUser>;
  /**
   * PUBLIC self-signup. Register auto-logs-in the new account by returning a
   * TokenPair, so a successful signup needs no follow-up Login. Resolves the new
   * session (mirrors login).
   */
  register(input: RegisterInput): Promise<SessionUser>;
  /**
   * Finish an OIDC authorization-code flow: exchange the IdP callback's
   * (code, state) for our own TokenPair and resolve the session (mirrors login).
   */
  completeSSO(
    providerId: string,
    code: string,
    state: string,
  ): Promise<SessionUser>;
  /** Resolve the current session, or null if not authenticated. */
  getSession(): Promise<SessionUser | null>;
  /** Tear down the session (best-effort). */
  logout(): Promise<void>;
}

// Map a tenant's effective permissions onto the coarse UI role used for gating
// write affordances. Ownership is free (Tenant.owner_account_id); otherwise a
// MANAGE grant => owner, any write verb => operator, read-only => viewer.
async function resolveRole(
  tenant: Tenant,
  accountId: string,
): Promise<TenantRole> {
  if (tenant.ownerAccountId && tenant.ownerAccountId === accountId) {
    return "owner";
  }
  try {
    const { permissions } = await iamClient.getMyPermissions({
      tenantId: tenant.id,
    });
    if (permissions.some((p) => p.action === Action.MANAGE)) return "owner";
    const write = [Action.CREATE, Action.UPDATE, Action.DELETE];
    if (permissions.some((p) => write.includes(p.action))) return "operator";
    return "viewer";
  } catch {
    return "viewer";
  }
}

// Build the full session from the bearer token: who am I, which tenants, what
// can I do in each. Throws if the token is missing/expired (the caller decides
// whether to refresh).
async function buildSession(): Promise<SessionUser> {
  const [{ account }, { tenants }] = await Promise.all([
    iamClient.getMyAccount({}),
    iamClient.listMyTenants({}),
  ]);
  if (!account) throw new Error("getMyAccount returned no account");

  const sessionTenants: SessionTenant[] = await Promise.all(
    tenants.map(async (t) => ({
      id: t.id,
      slug: t.slug,
      name: t.name,
      role: await resolveRole(t, account.id),
    })),
  );

  return {
    id: account.id,
    username: account.nickname,
    displayName: account.nickname || account.email,
    email: account.email,
    isAdmin: account.isAdmin,
    tenants: sessionTenants,
  };
}

// Exchange the persisted refresh token for a fresh pair; returns false when no
// usable refresh token exists or the exchange fails (credentials cleared).
async function tryRefresh(): Promise<boolean> {
  const refreshToken = getRefreshToken();
  if (!refreshToken) return false;
  try {
    const { tokens } = await iamClient.refresh({ refreshToken });
    applyTokens(tokens);
    return true;
  } catch {
    clearTokens();
    return false;
  }
}

const realAuthProvider: AuthProvider = {
  async login(login, password) {
    const { tokens } = await iamClient.login({ login, password });
    applyTokens(tokens);
    return buildSession();
  },

  async register({ email, nickname, password }) {
    const { tokens } = await iamClient.register({ email, nickname, password });
    applyTokens(tokens);
    return buildSession();
  },

  async completeSSO(providerId, code, state) {
    const { tokens } = await iamClient.completeSSO({ providerId, code, state });
    applyTokens(tokens);
    return buildSession();
  },

  async getSession() {
    // No access token in memory (fresh load) — mint one from the refresh token.
    if (!getAccessToken() && !(await tryRefresh())) return null;
    try {
      return await buildSession();
    } catch {
      // Access token likely expired mid-session: rotate once and retry.
      if (await tryRefresh()) {
        try {
          return await buildSession();
        } catch {
          clearTokens();
          return null;
        }
      }
      return null;
    }
  },

  async logout() {
    const refreshToken = getRefreshToken();
    if (refreshToken) {
      try {
        await iamClient.logout({ refreshToken });
      } catch {
        /* best-effort server-side revoke */
      }
    }
    clearTokens();
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
