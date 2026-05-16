import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Activity, Building2 } from "lucide-react";
import { clients } from "@/api/clients";
import { setTenantId } from "@/api/transport";
import { protoTsToISO } from "@/lib/proto-helpers";
import type { Tenant } from "@/lib/proto/cloud/v1/iam/tenant_pb";

export function SelectTenant() {
  const navigate = useNavigate();
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    clients.tenant
      .listMyTenants({})
      .then((r) => setTenants(r.tenants ?? []))
      .catch((err) =>
        setError(err instanceof Error ? err.message : "Failed to load tenants")
      )
      .finally(() => setLoading(false));
  }, []);

  function handleSelect(t: Tenant) {
    const id = t.id!.value;
    setTenantId(id);
    localStorage.setItem("stroppy.tenantId", id);
    navigate("/");
  }

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <div className="text-sm text-muted-foreground">Loading tenants...</div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="w-full max-w-md space-y-6 rounded-lg border border-border bg-card p-8">
        <div className="flex flex-col items-center gap-2">
          <Activity className="h-8 w-8 text-primary" />
          <h1 className="text-lg font-semibold tracking-tight">
            Select Tenant
          </h1>
          <p className="text-sm text-muted-foreground">
            Choose a workspace to continue
          </p>
        </div>

        {error && (
          <div className="text-sm text-destructive border border-destructive/30 p-3 rounded">
            {error}
          </div>
        )}

        {tenants.length === 0 && !error ? (
          <div className="text-center text-sm text-muted-foreground py-8">
            No tenants assigned. Contact admin.
          </div>
        ) : (
          <div className="space-y-2">
            {tenants.map((t) => (
              <button
                key={t.id?.value}
                onClick={() => handleSelect(t)}
                className="w-full flex items-center gap-3 rounded-md border border-border p-4 text-left transition-colors hover:bg-muted/50"
              >
                <Building2 className="h-5 w-5 text-muted-foreground shrink-0" />
                <div className="flex-1 min-w-0">
                  <div className="text-sm font-medium truncate">
                    {t.identity?.name ?? t.id?.value}
                  </div>
                  <div className="text-xs text-muted-foreground font-mono">
                    {t.id?.value}
                  </div>
                  {t.timestamps?.createdAt && (
                    <div className="text-xs text-muted-foreground">
                      Created {new Date(protoTsToISO(t.timestamps.createdAt)).toLocaleDateString()}
                    </div>
                  )}
                </div>
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
