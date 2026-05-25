// Tenant-aware routing wrapper.
//
// The active tenant lives in the URL as `/t/<tenant_id>/...`. To keep every
// link and navigation tenant-scoped without rewriting each call site, this
// module re-exports react-router-dom and overrides Link / NavLink / Navigate /
// useNavigate so absolute app paths are automatically prefixed with the current
// tenant. Pages import these symbols from "@/lib/router" instead of
// "react-router-dom".
import { useCallback } from "react";
import {
  Link as RRLink,
  NavLink as RRNavLink,
  Navigate as RRNavigate,
  useNavigate as useRRNavigate,
  useParams,
  type LinkProps,
  type NavLinkProps,
  type NavigateProps,
  type To,
  type NavigateOptions,
} from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";

// Re-export everything else (useParams, useLocation, Outlet, Routes, ...).
export * from "react-router-dom";

// Top-level routes that are NOT tenant-scoped and must never be prefixed.
const NON_TENANT_PREFIXES = ["/login", "/select-tenant", "/admin", "/share", "/t/"];

function isNonTenant(path: string): boolean {
  return NON_TENANT_PREFIXES.some(
    (p) =>
      path === p ||
      path === p.replace(/\/$/, "") ||
      path.startsWith(p) ||
      path.startsWith(p.replace(/\/$/, "") + "/") ||
      path.startsWith(p.replace(/\/$/, "") + "?")
  );
}

function prefixPath(path: string, tenantId?: string): string {
  if (!tenantId) return path;
  if (!path.startsWith("/")) return path; // relative, leave untouched
  if (isNonTenant(path)) return path;
  // "/" -> "/t/<id>", "/runs?x=1" -> "/t/<id>/runs?x=1"
  return `/t/${tenantId}${path === "/" ? "" : path}`;
}

function prefixTo(to: To, tenantId?: string): To {
  if (typeof to === "string") return prefixPath(to, tenantId);
  if (to && typeof to === "object" && typeof to.pathname === "string") {
    return { ...to, pathname: prefixPath(to.pathname, tenantId) };
  }
  return to;
}

/** Current tenant id: URL param when inside `/t/:tenantId`, else the auth context. */
export function useTenantId(): string | undefined {
  const params = useParams();
  const { user } = useAuth();
  return params.tenantId ?? user?.tenant_id ?? undefined;
}

export function Link({ to, ...rest }: LinkProps) {
  const tenantId = useTenantId();
  return <RRLink to={prefixTo(to, tenantId)} {...rest} />;
}

export function NavLink({ to, ...rest }: NavLinkProps) {
  const tenantId = useTenantId();
  return <RRNavLink to={prefixTo(to, tenantId)} {...rest} />;
}

export function Navigate({ to, ...rest }: NavigateProps) {
  const tenantId = useTenantId();
  return <RRNavigate to={prefixTo(to, tenantId)} {...rest} />;
}

export function useNavigate() {
  const navigate = useRRNavigate();
  const tenantId = useTenantId();
  return useCallback(
    (to: To | number, options?: NavigateOptions) => {
      if (typeof to === "number") return navigate(to);
      return navigate(prefixTo(to, tenantId), options);
    },
    [navigate, tenantId]
  );
}
