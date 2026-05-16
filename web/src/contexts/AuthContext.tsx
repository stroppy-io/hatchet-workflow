import {
  createContext,
  useState,
  useCallback,
  useEffect,
  type ReactNode,
} from "react";
import { clients } from "@/api/clients";
import {
  setAccessToken,
  setTenantId,
  setRefresher,
} from "@/api/transport";

export interface AuthUser {
  id: string;
  username: string;
  tenant_id: string | null;
  tenant_name: string | null;
  role: "viewer" | "operator" | "owner";
  is_root: boolean;
  tenants: { id: string; tenant_name: string; role: string }[];
}

/** Bootstrap session using the httpOnly cookie (REST legacy endpoint). */
async function legacyRefreshToken(): Promise<{ access_token: string }> {
  const res = await fetch("/api/v1/auth/refresh", {
    method: "POST",
    credentials: "include",
  });
  if (!res.ok) throw new Error("refresh failed");
  return res.json();
}

// In-memory refresh token (never put in localStorage).
let _refreshToken: string | null = null;

export interface AuthContextValue {
  user: AuthUser | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
  selectTenant: (tenantId: string) => Promise<void>;
}

export const AuthContext = createContext<AuthContextValue>({
  user: null,
  isAuthenticated: false,
  isLoading: true,
  login: async () => {},
  logout: async () => {},
  refresh: async () => {},
  selectTenant: async () => {},
});

/** Attempt a ConnectRPC token refresh; return new access token or null. */
async function doRefresh(): Promise<string | null> {
  if (!_refreshToken) return null;
  try {
    const r = await clients.auth.refreshTokens({ refreshToken: _refreshToken });
    const at = r.tokens?.accessToken ?? null;
    const rt = r.tokens?.refreshToken;
    if (rt) _refreshToken = rt;
    if (at) setAccessToken(at);
    return at;
  } catch {
    _refreshToken = null;
    setAccessToken(null);
    return null;
  }
}

/** Build AuthUser from ConnectRPC User + tenant list. */
async function fetchUserFromProto(): Promise<AuthUser | null> {
  try {
    // Restore tenant from localStorage if present.
    const storedTenantId = localStorage.getItem("stroppy.tenantId");
    if (storedTenantId) {
      setTenantId(storedTenantId);
    }

    const [userResp, tenantsResp] = await Promise.all([
      clients.user.me({}),
      clients.tenant.listMyTenants({}).catch(() => ({ tenants: [] as import("@/lib/proto/cloud/v1/iam/tenant_pb").Tenant[] })),
    ]);

    const tenantsList = (tenantsResp.tenants ?? []).map((t) => ({
      id: t.id?.value ?? "",
      tenant_name: t.identity?.name ?? "",
      role: "viewer" as const,
    }));

    // Determine current tenant context.
    const currentTenantId = storedTenantId || null;
    const currentTenant = currentTenantId
      ? tenantsResp.tenants?.find((t) => t.id?.value === currentTenantId)
      : null;

    // Probe root status: only admins can call listAllTenants.
    let isRoot = false;
    try {
      await clients.admin.listAllTenants({});
      isRoot = true;
    } catch {
      isRoot = false;
    }

    const authUser: AuthUser = {
      id: userResp.id?.value ?? "",
      username: userResp.nickname || userResp.email,
      tenant_id: currentTenantId,
      tenant_name: currentTenant?.identity?.name ?? null,
      role: "viewer",
      is_root: isRoot,
      tenants: tenantsList,
    };
    return authUser;
  } catch {
    return null;
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Wire the transport refresher so the auth interceptor can refresh tokens
  // automatically on Unauthenticated errors from any ConnectRPC call.
  useEffect(() => {
    setRefresher(doRefresh);
    return () => setRefresher(null);
  }, []);

  const fetchUser = useCallback(async () => {
    const u = await fetchUserFromProto();
    if (u) {
      setUser(u);
    } else {
      setUser(null);
      setAccessToken(null);
      _refreshToken = null;
    }
  }, []);

  // On mount: try to restore session via cookie-based refresh.
  // The httpOnly cookie is required for the bootstrap call.
  useEffect(() => {
    (async () => {
      try {
        const r = await legacyRefreshToken();
        setAccessToken(r.access_token);
        await fetchUser();
      } catch {
        setAccessToken(null);
        _refreshToken = null;
        setUser(null);
      } finally {
        setIsLoading(false);
      }
    })();
  }, [fetchUser]);

  const login = useCallback(
    async (email: string, password: string) => {
      const r = await clients.auth.login({ email, password });
      const at = r.tokens?.accessToken ?? "";
      const rt = r.tokens?.refreshToken ?? "";
      setAccessToken(at);
      _refreshToken = rt;
      await fetchUser();
    },
    [fetchUser]
  );

  const logout = useCallback(async () => {
    try {
      if (_refreshToken) {
        await clients.auth.logout({ refreshToken: _refreshToken });
      }
    } catch {
      // best-effort
    }
    setAccessToken(null);
    setTenantId(null);
    localStorage.removeItem("stroppy.tenantId");
    _refreshToken = null;
    setUser(null);
  }, []);

  const refresh = useCallback(async () => {
    const fresh = await doRefresh();
    if (fresh) {
      await fetchUser();
    } else {
      setUser(null);
    }
  }, [fetchUser]);

  // selectTenant is purely client-side: store id in localStorage + transport header.
  const selectTenant = useCallback(
    async (tenantId: string) => {
      localStorage.setItem("stroppy.tenantId", tenantId);
      setTenantId(tenantId);
      await fetchUser();
    },
    [fetchUser]
  );

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated: !!user,
        isLoading,
        login,
        logout,
        refresh,
        selectTenant,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}
