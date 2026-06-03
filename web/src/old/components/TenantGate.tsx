import { useEffect, useRef, useState } from "react";
import { Navigate, Outlet, useParams } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";

// Gate for the `/t/:tenantId/*` subtree. The URL tenant is the source of truth:
// when it differs from the tenant baked into the current JWT, re-issue a token
// scoped to the URL tenant via selectTenant. Renders the subtree only once the
// active tenant matches the URL.
export function TenantGate() {
  const { tenantId } = useParams<{ tenantId: string }>();
  const { user, selectTenant } = useAuth();
  const [switching, setSwitching] = useState(false);
  const attempted = useRef<string | null>(null);

  const hasAccess =
    !!user &&
    (user.is_root || (user.tenants || []).some((t) => t.id === tenantId));

  useEffect(() => {
    if (!tenantId || !user || !hasAccess) return;
    if (user.tenant_id === tenantId) {
      attempted.current = null;
      return;
    }
    if (attempted.current === tenantId) return; // already tried, avoid loop
    attempted.current = tenantId;
    setSwitching(true);
    selectTenant(tenantId).finally(() => setSwitching(false));
  }, [tenantId, user, hasAccess, selectTenant]);

  if (!user) return null;
  if (!hasAccess) return <Navigate to="/select-tenant" replace />;
  if (user.tenant_id !== tenantId || switching) {
    return (
      <div className="flex h-screen items-center justify-center bg-background text-sm text-muted-foreground">
        Switching tenant...
      </div>
    );
  }
  return <Outlet />;
}
