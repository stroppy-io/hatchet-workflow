import { useEffect, useState } from "react";
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
  const { user, selectTenant } = useAuth();
  const [tenants, setTenants] = useState<TenantOption[]>([]);

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
      onValueChange={(v) => selectTenant(v)}
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
