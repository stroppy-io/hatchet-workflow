import { useEffect, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { clients } from "@/api/clients";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";

interface TenantOption {
  id: string;
  name: string;
}

export function TenantSwitcher() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [tenants, setTenants] = useState<TenantOption[]>([]);

  // Preserve the entity path after the tenant segment when switching. e.g.
  // /t/A/runs/X → /t/B/runs (drop the entity id so we don't 404 cross-tenant).
  function switchTo(id: string) {
    const m = location.pathname.match(/^\/t\/[^/]+(\/[^/]+)?/);
    const entitySegment = m && m[1] ? m[1].split("/")[1] : "runs";
    const safeEntity = ["runs", "suites", "presets", "run-presets", "packages", "compare", "settings", "members", "tokens"].includes(entitySegment) ? entitySegment : "runs";
    navigate(`/t/${id}/${safeEntity}`);
  }

  useEffect(() => {
    if (user?.is_root) {
      clients.admin.listAllTenants({})
        .then((resp) =>
          setTenants(
            (resp.tenants ?? []).map((t) => ({
              id: t.id?.value ?? "",
              name: t.identity?.name ?? "",
            }))
          )
        )
        .catch(() => {});
    }
  }, [user?.is_root]);

  if (!user?.is_root || tenants.length === 0) return null;

  return (
    <Select
      value={user.tenant_id ?? undefined}
      onValueChange={(v) => switchTo(v)}
    >
      <SelectTrigger className="h-7 text-xs border-border">
        <SelectValue placeholder="Switch tenant" />
      </SelectTrigger>
      <SelectContent>
        {tenants.map((t) => (
          <SelectItem key={t.id} value={t.id} className="text-xs">
            {t.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
