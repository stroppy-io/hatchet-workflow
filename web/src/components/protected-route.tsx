import { Navigate, Outlet, useLocation, useParams } from "react-router-dom";

import { useAuth } from "@/contexts/auth-context";

export function ProtectedRoute() {
  const { account, loading, tenants } = useAuth();
  const location = useLocation();
  const { tenantId } = useParams<{ tenantId: string }>();

  if (loading) {
    return <div className="grid min-h-screen place-items-center text-sm text-muted-foreground">Loading session...</div>;
  }

  if (!account) {
    return <Navigate to="/login" replace state={{ next: location.pathname }} />;
  }

  if (tenantId && !tenants.some((tenant) => tenant.entity?.id?.value === tenantId)) {
    const firstTenant = tenants[0]?.entity?.id?.value;
    return <Navigate to={firstTenant ? `/t/${firstTenant}/runs` : "/login"} replace />;
  }

  return <Outlet />;
}
