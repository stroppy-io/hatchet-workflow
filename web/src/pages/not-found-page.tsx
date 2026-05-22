import { Link } from "react-router-dom";

import { tenantPath } from "@/lib/routes";

export function NotFoundPage() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-6 text-foreground">
      <div className="w-full max-w-md border bg-card p-6">
        <h1 className="text-lg font-semibold">Page not found</h1>
        <p className="mt-2 text-sm text-muted-foreground">Routes are tenant-scoped. Open a tenant workspace first.</p>
        <Link className="mt-5 inline-flex bg-primary px-3 py-2 text-sm text-primary-foreground" to={tenantPath("default", "/wizard")}>
          Open default tenant
        </Link>
      </div>
    </div>
  );
}
