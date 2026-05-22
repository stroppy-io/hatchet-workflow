import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

import { api, setAccessToken } from "@/lib/connect";
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

  const refresh = useCallback(async () => {
    const response = await api.auth.refreshTokens({ refreshToken: "" });
    setAccessToken(response.tokens?.accessToken || null);
    await loadIdentity();
  }, [loadIdentity]);

  useEffect(() => {
    refresh()
      .catch(() => {
        setAccessToken(null);
        setAccount(null);
        setTenants([]);
      })
      .finally(() => setLoading(false));
  }, [refresh]);

  const login = useCallback(
    async (email: string, password: string) => {
      const response = await api.auth.login({ email, password });
      setAccessToken(response.tokens?.accessToken || null);
      await loadIdentity();
    },
    [loadIdentity],
  );

  const logout = useCallback(async () => {
    try {
      await api.auth.logout({ refreshToken: "" });
    } finally {
      setAccessToken(null);
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
