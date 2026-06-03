// Tenant-aware routing wrapper.
//
// The active tenant lives in the URL as `/t/<slug>/...`. To keep links and
// navigation tenant-scoped without rewriting each call site, this module
// re-exports react-router-dom and overrides Link / NavLink / Navigate /
// useNavigate so absolute app paths are automatically prefixed with the
// current tenant slug. Components import these symbols from "@/lib/router"
// instead of "react-router-dom".
//
// NAVIGATION FOUNDATION (browser Back must always work):
//   * Every page change goes through react-router — <Link>/<NavLink>/navigate()
//     — so it produces a real history entry. Never swap pages via component
//     state, window.history, or location.href; Back can't reverse those.
//   * Use replace ONLY for genuine redirects (auth bounce, post-login landing,
//     guard fallbacks). User-initiated navigation (menu items, switches, links,
//     breadcrumbs) is always a push.
//   * Do NOT swallow navigation with preventDefault on links.
//   * MODAL / DRAWER "pages": back them with a route (e.g. /t/:slug/runs/new or
//     a `?dialog=...` search param) so that opening pushes history and Back
//     closes the overlay. Drive open/close from the URL (useSearchParams or a
//     nested route), not local useState. A transient, non-addressable popover
//     (confirm dialog, dropdown) may stay state-only since it isn't a "page".
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

// Re-export everything else (useParams, useLocation, Outlet, Routes, ...).
export * from "react-router-dom";

// Top-level routes that are NOT tenant-scoped and must never be prefixed.
const NON_TENANT_PREFIXES = ["/login", "/select-tenant", "/admin", "/profile", "/orgs", "/share", "/t/"];

function isNonTenant(path: string): boolean {
  return NON_TENANT_PREFIXES.some(
    (p) =>
      path === p ||
      path === p.replace(/\/$/, "") ||
      path.startsWith(p) ||
      path.startsWith(p.replace(/\/$/, "") + "/") ||
      path.startsWith(p.replace(/\/$/, "") + "?"),
  );
}

function prefixPath(path: string, slug?: string): string {
  if (!slug) return path;
  if (!path.startsWith("/")) return path; // relative, leave untouched
  if (isNonTenant(path)) return path;
  // "/" -> "/t/<slug>", "/runs?x=1" -> "/t/<slug>/runs?x=1"
  return `/t/${slug}${path === "/" ? "" : path}`;
}

function prefixTo(to: To, slug?: string): To {
  if (typeof to === "string") return prefixPath(to, slug);
  if (to && typeof to === "object" && typeof to.pathname === "string") {
    return { ...to, pathname: prefixPath(to.pathname, slug) };
  }
  return to;
}

/** Current tenant slug from the URL (`/t/:slug`). */
export function useTenantSlug(): string | undefined {
  const params = useParams();
  return params.slug;
}

export function Link({ to, ...rest }: LinkProps) {
  const slug = useTenantSlug();
  return <RRLink to={prefixTo(to, slug)} {...rest} />;
}

export function NavLink({ to, ...rest }: NavLinkProps) {
  const slug = useTenantSlug();
  return <RRNavLink to={prefixTo(to, slug)} {...rest} />;
}

export function Navigate({ to, ...rest }: NavigateProps) {
  const slug = useTenantSlug();
  return <RRNavigate to={prefixTo(to, slug)} {...rest} />;
}

export function useNavigate() {
  const navigate = useRRNavigate();
  const slug = useTenantSlug();
  return useCallback(
    (to: To | number, options?: NavigateOptions) => {
      if (typeof to === "number") return navigate(to);
      return navigate(prefixTo(to, slug), options);
    },
    [navigate, slug],
  );
}
