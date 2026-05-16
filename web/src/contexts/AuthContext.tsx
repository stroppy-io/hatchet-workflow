import {
  createContext,
  useState,
  useCallback,
  useEffect,
  type ReactNode,
} from "react";
import type { AuthUser } from "@/api/types";
import { clients } from "@/api/clients";
import {
  setAccessToken,
  setRefresher,
} from "@/api/transport";
// meAPI still used because UserService.Me returns proto User {id,email,nickname}
// which lacks tenant_id / role / is_root / tenants — REST /auth/me returns the
// enriched AuthUser. TODO: replace once the server exposes a richer Me RPC.
// selectTenantAPI kept for the same reason — no SelectTenant RPC yet.
import {
  meAPI,
  refreshToken as legacyRefreshToken,
  selectTenantAPI,
  SessionExpiredError,
} from "@/api/client";

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
    try {
      // TODO: replace meAPI() with clients.user.me({}) once the server's Me RPC
      // returns tenant_id / role / is_root / tenants in addition to email/nickname.
      const u = await meAPI();
      setUser(u);
    } catch (e) {
      setUser(null);
      setAccessToken(null);
      _refreshToken = null;
      if (e instanceof SessionExpiredError) return;
    }
  }, []);

  // Global handler: catch unhandled SessionExpiredError from any legacy REST call.
  useEffect(() => {
    const handler = (event: PromiseRejectionEvent) => {
      if (event.reason instanceof SessionExpiredError) {
        event.preventDefault();
        setAccessToken(null);
        _refreshToken = null;
        setUser(null);
      }
    };
    window.addEventListener("unhandledrejection", handler);
    return () => window.removeEventListener("unhandledrejection", handler);
  }, []);

  // On mount: try to restore session via cookie-based refresh (legacy REST).
  // After login we'll have _refreshToken in-memory and use ConnectRPC for
  // subsequent refreshes. The httpOnly cookie is required for the bootstrap call.
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

  const selectTenant = useCallback(
    async (tenantId: string) => {
      // TODO: migrate to ConnectRPC once the server exposes a SelectTenant RPC
      // or the Me RPC includes tenant context so we can call setTenantId().
      const r = await selectTenantAPI(tenantId);
      setAccessToken(r.access_token);
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
