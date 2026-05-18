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

/**
 * doRefresh attempts a silent token refresh. The refresh token is in an
 * HttpOnly cookie set by the server on Login/Refresh — the browser sends it
 * automatically thanks to `credentials: "include"` in the transport. We pass
 * an empty body; the handler reads the cookie.
 */
async function doRefresh(): Promise<string | null> {
  try {
    const r = await clients.auth.refreshTokens({ refreshToken: "" });
    const at = r.tokens?.accessToken ?? null;
    if (at) setAccessToken(at);
    return at;
  } catch {
    setAccessToken(null);
    return null;
  }
}

/** Build AuthUser from ConnectRPC User + tenant list. */
async function fetchUserFromProto(): Promise<AuthUser | null> {
  try {
    const storedTenantId = localStorage.getItem("stroppy.tenantId");
    if (storedTenantId) {
      setTenantId(storedTenantId);
    }

    const [userResp, tenantsResp] = await Promise.all([
      clients.user.me({}),
      clients.tenant
        .listMyTenants({})
        .catch(() => ({
          tenants: [] as import("@/lib/proto/cloud/v1/iam/tenant_pb").Tenant[],
        })),
    ]);

    const tenantsList = (tenantsResp.tenants ?? []).map((t) => ({
      id: t.id?.value ?? "",
      tenant_name: t.identity?.name ?? "",
      role: "viewer" as const,
    }));

    const currentTenantId = storedTenantId || null;
    const currentTenant = currentTenantId
      ? tenantsResp.tenants?.find((t) => t.id?.value === currentTenantId)
      : null;

    let isRoot = false;
    try {
      await clients.admin.listAllTenants({});
      isRoot = true;
    } catch {
      isRoot = false;
    }

    return {
      id: userResp.id?.value ?? "",
      username: userResp.nickname || userResp.email,
      tenant_id: currentTenantId,
      tenant_name: currentTenant?.identity?.name ?? null,
      role: "viewer",
      is_root: isRoot,
      tenants: tenantsList,
    };
  } catch {
    return null;
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Wire the transport refresher so the auth interceptor refreshes tokens
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
    }
  }, []);

  // On mount: silent refresh attempt — if the HttpOnly cookie from a prior
  // session is still valid, this gives us a new access token without UI.
  useEffect(() => {
    (async () => {
      const fresh = await doRefresh();
      if (fresh) {
        await fetchUser();
      }
      setIsLoading(false);
    })();
  }, [fetchUser]);

  const login = useCallback(
    async (email: string, password: string) => {
      const r = await clients.auth.login({ email, password });
      const at = r.tokens?.accessToken ?? "";
      setAccessToken(at);
      // Refresh token rides in HttpOnly cookie — never touched by JS.
      await fetchUser();
    },
    [fetchUser]
  );

  const logout = useCallback(async () => {
    try {
      await clients.auth.logout({ refreshToken: "" });
    } catch {
      // best-effort; cookie still cleared by server
    }
    setAccessToken(null);
    setTenantId(null);
    localStorage.removeItem("stroppy.tenantId");
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
