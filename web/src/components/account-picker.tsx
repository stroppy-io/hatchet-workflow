import { useState } from "react";
import { Check, ChevronsUpDown } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { useListQuery } from "@/hooks/use-list-query";
import { api } from "@/lib/connect";
import { tenantIdMessage } from "@/lib/proto";
import { cn } from "@/lib/utils";

type AccountPickerProps = {
  tenantId: string;
  value: string;
  label?: string;
  onChange: (accountId: string, label: string) => void;
  placeholder?: string;
};

// Platform-admin account search (AccountAdminService.ListAccounts). Used where an
// AccountId must be picked by a human — tenant member add, tenant owner. Only
// usable by is_admin callers; non-admin surfaces fall back to a raw id input.
export function AccountPicker({ tenantId, value, label, onChange, placeholder = "Search account…" }: AccountPickerProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const search = useDebouncedValue(query);

  const result = useListQuery(
    () =>
      api.accountAdmin.listAccounts({
        tenantId: tenantIdMessage(tenantId),
        search: search || undefined,
        page: { size: 8 },
      }),
    [tenantId, search, open],
  );

  const accounts = result.data?.accounts ?? [];

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button type="button" variant="outline" role="combobox" className="w-full justify-between font-normal">
          <span className={cn("truncate", !value && "text-muted-foreground")}>{value ? label || value : placeholder}</span>
          <ChevronsUpDown className="size-4 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[var(--radix-popover-trigger-width)] p-0" align="start">
        <div className="border-b p-2">
          <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Email or nickname" autoFocus />
        </div>
        <div className="max-h-64 overflow-y-auto p-1">
          {result.loading ? (
            <div className="p-3 text-sm text-muted-foreground">Searching…</div>
          ) : result.error ? (
            <div className="p-3 text-sm text-destructive">{result.error}</div>
          ) : accounts.length === 0 ? (
            <div className="p-3 text-sm text-muted-foreground">No accounts</div>
          ) : (
            accounts.map((account) => {
              const id = account.entity?.id?.value ?? "";
              const display = account.email || account.nickname || id;
              return (
                <button
                  key={id}
                  type="button"
                  className="flex w-full items-center justify-between gap-2 rounded-sm px-2 py-1.5 text-left text-sm hover:bg-accent hover:text-accent-foreground"
                  onClick={() => {
                    onChange(id, display);
                    setOpen(false);
                  }}
                >
                  <span className="truncate">{display}</span>
                  {value === id ? <Check className="size-4 shrink-0" /> : null}
                </button>
              );
            })
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}
