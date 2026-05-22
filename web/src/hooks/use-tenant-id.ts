import { useParams } from "react-router-dom";

export function useTenantId() {
  const { tenantId } = useParams<{ tenantId: string }>();
  return tenantId ?? "";
}
