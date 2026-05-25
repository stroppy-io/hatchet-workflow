import { useMemo, useState } from "react";
import { Plus } from "lucide-react";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";

import { useConfirm } from "@/components/confirm-dialog";
import { EntityDataTable, type EntityColumnDef } from "@/components/data-table/entity-data-table";
import { DataBadge, FilterSelect } from "@/components/data-table/table-controls";
import { FormDialog } from "@/components/form-dialog";
import { Button } from "@/components/ui/button";
import { CopyButton } from "@/components/ui/copy-button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useAction } from "@/hooks/use-action";
import { useListQuery } from "@/hooks/use-list-query";
import { useTableState } from "@/hooks/use-table-state";
import { useTenantId } from "@/hooks/use-tenant-id";
import { api } from "@/lib/connect";
import { formatTimestamp, roleLabel } from "@/lib/format";
import { idMessage, tenantIdMessage } from "@/lib/proto";
import { notifyError, notifySuccess } from "@/lib/toast";
import { ListApiTokensRequest_SortField } from "@/lib/proto/cloud/v1/api/ui/apitoken_pb.ts";
import type { ApiToken } from "@/lib/proto/cloud/v1/models/apitoken_pb.ts";
import { TenantMember_Role } from "@/lib/proto/cloud/v1/models/tenant_pb.ts";

// Roles a token can authenticate as — proto caps this at <= ADMIN (no OWNER).
const TOKEN_ROLE_OPTIONS = [
  { label: "Viewer", value: String(TenantMember_Role.VIEWER) },
  { label: "Admin", value: String(TenantMember_Role.ADMIN) },
];

export function TokensPage() {
  const tenantId = useTenantId();
  const confirm = useConfirm();
  const [role, setRole] = useState("all");
  const table = useTableState(ListApiTokensRequest_SortField.CREATED_AT, [tenantId, role]);

  const result = useListQuery(
    () =>
      api.apiToken.listApiTokens({
        tenantId: tenantIdMessage(tenantId),
        search: table.debouncedSearch || undefined,
        role: role === "all" ? undefined : (Number(role) as TenantMember_Role),
        sortField: table.sortField,
        order: table.order,
        page: table.pagination.page,
      }),
    [tenantId, role, table.debouncedSearch, table.sortField, table.order, table.pagination.page, table.reloadKey],
  );

  // Create dialog.
  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState("");
  const [newRole, setNewRole] = useState(String(TenantMember_Role.VIEWER));
  const [expires, setExpires] = useState("");
  // Plaintext secret shown once after creation.
  const [secret, setSecret] = useState<string | null>(null);
  const createAction = useAction(api.apiToken.createApiToken);

  function openCreate() {
    setName("");
    setNewRole(String(TenantMember_Role.VIEWER));
    setExpires("");
    createAction.reset();
    setCreateOpen(true);
  }

  async function submitCreate() {
    const response = await createAction.run({
      tenantId: tenantIdMessage(tenantId),
      name: name.trim(),
      role: Number(newRole) as TenantMember_Role,
      expiresAt: expires ? timestampFromDate(new Date(expires)) : undefined,
    });
    if (!response) return; // failure shown inline via createAction.error
    setCreateOpen(false);
    setSecret(response.secret);
    notifySuccess("Token created");
    table.reload();
  }

  async function revoke(token: ApiToken) {
    const ok = await confirm({
      title: `Revoke "${token.name}"?`,
      description: "Any client using this token loses access immediately.",
      danger: true,
      confirmLabel: "Revoke",
    });
    if (!ok) return;
    try {
      await api.apiToken.revokeApiToken({
        tenantId: tenantIdMessage(tenantId),
        id: idMessage(token.entity?.id?.value),
      });
      notifySuccess("Token revoked");
      table.reload();
    } catch (error) {
      notifyError(error, "Could not revoke token");
    }
  }

  const columns = useMemo<EntityColumnDef<ApiToken>[]>(
    () => [
      { accessorKey: "name", id: "name", header: "Name", sortField: ListApiTokensRequest_SortField.NAME },
      { id: "role", header: "Role", cell: ({ row }) => <DataBadge>{roleLabel(row.original.role)}</DataBadge> },
      { id: "expires", header: "Expires", sortField: ListApiTokensRequest_SortField.EXPIRES_AT, cell: ({ row }) => formatTimestamp(row.original.expiresAt) },
      { id: "created", header: "Created", sortField: ListApiTokensRequest_SortField.CREATED_AT, cell: ({ row }) => formatTimestamp(row.original.entity?.timestamps?.createdAt) },
    ],
    [],
  );

  return (
    <>
      <EntityDataTable
        actions={[{ label: "Revoke", onSelect: revoke }]}
        columns={columns}
        data={result.data?.apiTokens ?? []}
        error={result.error}
        filters={<FilterSelect onChange={setRole} options={[{ label: "All roles", value: "all" }, ...TOKEN_ROLE_OPTIONS]} value={role} />}
        getRowId={(row) => row.entity?.id?.value ?? row.name}
        headerActions={
          <Button size="sm" onClick={openCreate}>
            <Plus />
            New token
          </Button>
        }
        loading={result.loading}
        onNextPage={() => table.pagination.next(result.data?.pageInfo)}
        onPageSizeChange={table.pagination.setPageSize}
        onPreviousPage={table.pagination.previous}
        onSearchChange={table.setSearch}
        onSortChange={table.setSort}
        pageIndex={table.pagination.index}
        pageInfo={result.data?.pageInfo}
        pageSize={table.pagination.pageSize}
        search={table.search}
        searchPlaceholder="Search tokens"
        title="API Tokens"
      />

      <FormDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        title="Create API token"
        description="CI/SDK credential. The secret is shown once."
        submitLabel="Create"
        onSubmit={submitCreate}
        loading={createAction.loading}
        error={createAction.error}
        submitDisabled={!name.trim()}
      >
        <div className="space-y-2">
          <Label htmlFor="token-name">Name</Label>
          <Input id="token-name" value={name} onChange={(event) => setName(event.target.value)} placeholder="ci-pipeline" autoFocus />
        </div>
        <div className="space-y-2">
          <Label>Role</Label>
          <Select value={newRole} onValueChange={setNewRole}>
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {TOKEN_ROLE_OPTIONS.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label htmlFor="token-expires">Expires at (optional)</Label>
          <Input id="token-expires" type="datetime-local" value={expires} onChange={(event) => setExpires(event.target.value)} />
        </div>
      </FormDialog>

      <Dialog open={secret !== null} onOpenChange={(open) => !open && setSecret(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Token created</DialogTitle>
            <DialogDescription>Copy this token now — it will not be shown again.</DialogDescription>
          </DialogHeader>
          <div className="flex items-center gap-2">
            <code className="block flex-1 select-all break-all rounded-md border bg-background p-3 font-mono text-xs">
              {secret}
            </code>
            <CopyButton value={secret ?? ""} toastLabel="Token copied" />
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
