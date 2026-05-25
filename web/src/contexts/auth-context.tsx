import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { api, setAccessToken, setUnauthorizedHandler } from "@/lib/connect";
import type { Account } from "@/lib/proto/cloud/v1/models/account_pb.ts";
import type { Tenant } from "@/lib/proto/cloud/v1/models/tenant_pb.ts";
import { TenantMember_Role } from "@/lib/proto/cloud/v1/models/tenant_pb.ts";

type AuthState = {
  account: Account | null;
  tenants: Tenant[];
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
  roleForTenant: (tenantId: string) => TenantMember_Role;
};

const AuthContext = createContext<AuthState | null>(null);

// Refresh token persistence. The backend reads the refresh token from the request
// BODY (the documented HttpOnly-cookie path is not implemented server-side), so
// the web client behaves like the CLI/SDK: it keeps the opaque refresh token and
// sends it explicitly. Survives full page reloads (access token is memory-only).
// XSS note: localStorage is script-readable; an HttpOnly cookie set by the server
// would be the stronger long-term design.
const REFRESH_KEY = "stroppy.refresh_token";

function getStoredRefresh(): string {
  try {
    return localStorage.getItem(REFRESH_KEY) ?? "";
  } catch {
    return "";
  }
}

function setStoredRefresh(token: string | null) {
  try {
    if (token) localStorage.setItem(REFRESH_KEY, token);
    else localStorage.removeItem(REFRESH_KEY);
  } catch {
    /* storage unavailable (private mode); session stays memory-only */
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [account, setAccount] = useState<Account | null>(null);
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [loading, setLoading] = useState(true);

  const loadIdentity = useCallback(async () => {
    const [nextAccount, tenantList] = await Promise.all([
      api.auth.me({}),
      api.tenant.listMyTenants({}),
    ]);
    setAccount(nextAccount);
    setTenants(tenantList.tenants);
  }, []);

  // Single-flight token refresh. Concurrent 401s (e.g. me + listMyTenants on
  // mount) and StrictMode's double-invoke would otherwise fire several
  // refreshTokens calls; with single-use refresh tokens all but one fail and the
  // session dies ("login keeps dropping"). One in-flight call dedupes them all.
  // It refreshes the TOKEN ONLY (never loadIdentity) so it can't recurse through
  // the interceptor. Carries x-retry so the refresh request itself isn't retried.
  // Sends the stored refresh token in the body and persists the ROTATED token the
  // server returns (refresh tokens are single-use), so the next refresh succeeds.
  const refreshingRef = useRef<Promise<boolean> | null>(null);
  const doRefresh = useCallback((): Promise<boolean> => {
    if (!refreshingRef.current) {
      refreshingRef.current = (async () => {
        const stored = getStoredRefresh();
        if (!stored) {
          // No persisted session (fresh browser / after logout): nothing to restore.
          setAccessToken(null);
          setAccount(null);
          setTenants([]);
          return false;
        }
        try {
          const response = await api.auth.refreshTokens({ refreshToken: stored }, { headers: { "x-retry": "1" } });
          const token = response.tokens?.accessToken ?? null;
          setAccessToken(token);
          setStoredRefresh(response.tokens?.refreshToken ?? null);
          if (token) return true;
          setAccount(null);
          setTenants([]);
          return false;
        } catch {
          setAccessToken(null);
          setStoredRefresh(null);
          setAccount(null);
          setTenants([]);
          return false;
        } finally {
          refreshingRef.current = null;
        }
      })();
    }
    return refreshingRef.current;
  }, []);

  const refresh = useCallback(async () => {
    if (await doRefresh()) await loadIdentity();
  }, [doRefresh, loadIdentity]);

  // On mount: restore session from the stored refresh token, then load identity.
  useEffect(() => {
    (async () => {
      const ok = await doRefresh();
      if (ok) {
        await loadIdentity().catch(() => undefined);
      }
      setLoading(false);
    })();
  }, [doRefresh, loadIdentity]);

  // The connect 401 interceptor refreshes once (single-flight) and retries.
  useEffect(() => {
    setUnauthorizedHandler(doRefresh);
    return () => setUnauthorizedHandler(null);
  }, [doRefresh]);

  const login = useCallback(
    async (email: string, password: string) => {
      const response = await api.auth.login({ email, password });
      setAccessToken(response.tokens?.accessToken || null);
      setStoredRefresh(response.tokens?.refreshToken ?? null);
      await loadIdentity();
    },
    [loadIdentity],
  );

  const logout = useCallback(async () => {
    try {
      // Send the token so the server revokes the session in Valkey (logout only
      // revokes when given the token; empty would leave it alive until TTL).
      await api.auth.logout({ refreshToken: getStoredRefresh() });
    } finally {
      setAccessToken(null);
      setStoredRefresh(null);
      setAccount(null);
      setTenants([]);
    }
  }, []);

  const roleForTenant = useCallback(
    (tenantId: string) => {
      const accountId = account?.entity?.id?.value;
      const tenant = tenants.find((candidate) => candidate.entity?.id?.value === tenantId);
      if (!tenant || !accountId) return TenantMember_Role.UNSPECIFIED;
      if (tenant.ownerAccountId?.value === accountId) return TenantMember_Role.OWNER;
      return tenant.members.find((member) => member.accountId?.value === accountId)?.role ?? TenantMember_Role.UNSPECIFIED;
    },
    [account, tenants],
  );

  const value = useMemo<AuthState>(
    () => ({ account, tenants, loading, login, logout, refresh, roleForTenant }),
    [account, tenants, loading, login, logout, refresh, roleForTenant],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) {
    throw new Error("useAuth must be used inside AuthProvider");
  }
  return value;
}
