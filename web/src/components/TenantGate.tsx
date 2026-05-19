import { useEffect } from "react";
import { Navigate, Outlet, useParams } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { setTenantId as setTransportTenantId } from "@/api/transport";

/**
 * TenantGate validates the :tenantId URL segment against the authenticated
 * user's tenant list and syncs the active tenant into AuthContext + transport
 * header (X-Tenant-Id). URL is the single source of truth — switching tenants
 * is just a navigation to a different /t/<id>/... path.
 *
 * Root users may visit any tenant; non-root must belong.
 */
export function TenantGate() {
  const { tenantId } = useParams<{ tenantId: string }>();
  const { user, selectTenant } = useAuth();

  const validForUser = !!user && !!tenantId && (
    user.is_root || (user.tenants ?? []).some((t) => t.id === tenantId)
  );

  useEffect(() => {
    if (!tenantId || !user) return;
    if (!validForUser) return;
    if (user.tenant_id !== tenantId) {
      // Silent sync — does NOT navigate, just updates context + header.
      void selectTenant(tenantId);
    } else {
      // Ensure transport header is set on first mount / hard reload.
      setTransportTenantId(tenantId);
    }
  }, [tenantId, user, validForUser, selectTenant]);

  if (!user) return null; // outer auth guard will handle
  if (!tenantId || !validForUser) {
    return <Navigate to="/select-tenant" replace />;
  }
  return <Outlet />;
}
