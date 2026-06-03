import { Navigate, Outlet, useParams, useLocation } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";

// Authenticated-only gate. Bounces to /login preserving the target.
export function RequireAuth() {
  const { user, isLoading } = useAuth();
  const location = useLocation();
  if (isLoading) return null;
  if (!user) {
    return (
      <Navigate
        to={`/login?redirect=${encodeURIComponent(location.pathname)}`}
        replace
      />
    );
  }
  return <Outlet />;
}

// Platform-admin gate for /admin/*.
export function RequireAdmin() {
  const { user } = useAuth();
  if (!user) return <Navigate to="/login" replace />;
  if (!user.isAdmin) return <Navigate to="/" replace />;
  return <Outlet />;
}

// Tenant-scope gate: the `/t/:slug` slug must be one the user belongs to
// (or the user is a platform admin). The slug is the source of truth for the active org.
export function RequireTenant() {
  const { user } = useAuth();
  const { slug } = useParams<{ slug: string }>();
  if (!user) return <Navigate to="/login" replace />;
  const member = user.tenants.some((t) => t.slug === slug);
  if (!member && !user.isAdmin) return <Navigate to="/select-tenant" replace />;
  return <Outlet />;
}
