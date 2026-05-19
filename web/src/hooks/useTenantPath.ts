import { useParams } from "react-router-dom";
import { useCallback } from "react";

/**
 * tenantPath builds an absolute path under /t/<tenantId>. The input is the
 * tenant-relative path (with or without a leading slash). Returns "/" if the
 * tenant id is empty — caller should not invoke this without a tenant.
 */
export function tenantPath(tenantId: string | null | undefined, p: string): string {
  if (!tenantId) return "/";
  const clean = p.startsWith("/") ? p.slice(1) : p;
  if (clean === "" || clean === "/") return `/t/${tenantId}`;
  return `/t/${tenantId}/${clean}`;
}

/** useTenantId returns the tenantId param from the URL, or null when outside tenant scope. */
export function useTenantId(): string | null {
  const { tenantId } = useParams<{ tenantId: string }>();
  return tenantId ?? null;
}

/**
 * useTenantPath returns a memoized builder that prefixes the tenant id from
 * the URL. Use inside components rendered under the /t/:tenantId/* tree.
 */
export function useTenantPath(): (p: string) => string {
  const tid = useTenantId();
  return useCallback((p: string) => tenantPath(tid, p), [tid]);
}
