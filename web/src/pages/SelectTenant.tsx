import { Building2 } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";

// Organization picker, keyed by slug. Listed when the user has multiple
// tenants and none is active in the URL.
export function SelectTenant() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const tenants = user?.tenants ?? [];

  return (
    <div className="flex h-screen items-center justify-center bg-background p-6">
      <div className="w-full max-w-md">
        <div className="mb-4 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
          Choose organization
        </div>
        <div className="border border-border bg-card">
          {tenants.length === 0 && (
            <div className="p-6 text-sm text-muted-foreground">
              You don't belong to any organization yet.
            </div>
          )}
          {tenants.map((t) => (
            <button
              key={t.id}
              onClick={() => navigate(`/t/${t.slug}`)}
              className="flex w-full items-center gap-3 border-b border-border px-4 py-3 text-left transition-colors last:border-b-0 hover:bg-muted"
            >
              <Building2 className="h-4 w-4 text-muted-foreground" />
              <span className="flex min-w-0 flex-col">
                <span className="truncate font-mono text-sm text-foreground">
                  {t.slug}
                </span>
                <span className="truncate text-xs text-muted-foreground">
                  {t.name} · {t.role}
                </span>
              </span>
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
