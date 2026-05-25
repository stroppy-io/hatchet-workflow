import { Navigate } from "react-router-dom";

import { useTenantId } from "@/hooks/use-tenant-id";
import { tenantPath } from "@/lib/routes";

export function TenantHomePage() {
  const tenantId = useTenantId();
  return <Navigate to={tenantPath(tenantId, "/runs")} replace />;
}
