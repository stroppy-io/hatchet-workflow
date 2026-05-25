import { useMemo, useState } from "react";
import { Plus } from "lucide-react";

import { AccountPicker } from "@/components/account-picker";
import { useConfirm } from "@/components/confirm-dialog";
import { EntityDataTable, type EntityColumnDef } from "@/components/data-table/entity-data-table";
import { FormDialog } from "@/components/form-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAction } from "@/hooks/use-action";
import { useListQuery } from "@/hooks/use-list-query";
import { useTableState } from "@/hooks/use-table-state";
import { useTenantId } from "@/hooks/use-tenant-id";
import { api } from "@/lib/connect";
import { formatId, formatTimestamp } from "@/lib/format";
import { idMessage, tenantIdMessage } from "@/lib/proto";
import { notifyError, notifySuccess } from "@/lib/toast";
import { ListTenantsRequest_SortField } from "@/lib/proto/cloud/v1/api/admin/tenant_pb.ts";
import type { Tenant } from "@/lib/proto/cloud/v1/models/tenant_pb.ts";

export function PlatformTenantsPage() {
  const tenantId = useTenantId();
  const confirm = useConfirm();
  const table = useTableState(ListTenantsRequest_SortField.CREATED_AT, [tenantId]);

  const result = useListQuery(
    () =>
      api.tenantAdmin.listTenants({
        tenantId: tenantIdMessage(tenantId),
        search: table.debouncedSearch || undefined,
        sortField: table.sortField,
        order: table.order,
        page: table.pagination.page,
      }),
    [tenantId, table.debouncedSearch, table.sortField, table.order, table.pagination.page, table.reloadKey],
  );

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<Tenant | null>(null);
  const [name, setName] = useState("");
  const [ownerId, setOwnerId] = useState("");
  const [ownerLabel, setOwnerLabel] = useState("");

  const save = useAction(async () => {
    if (editTarget) {
      return api.tenantAdmin.updateTenant({
        tenant: { entity: { id: idMessage(editTarget.entity?.id?.value) }, name },
        updateMask: { paths: ["name"] },
      });
    }
    return api.tenantAdmin.createTenant({
      tenant: { name, ownerAccountId: idMessage(ownerId.trim()) },
    });
  });

  function openCreate() {
    setEditTarget(null);
    setName("");
    setOwnerId("");
    setOwnerLabel("");
    save.reset();
    setDialogOpen(true);
  }

  function openEdit(target: Tenant) {
    setEditTarget(target);
    setName(target.name ?? "");
    setOwnerId(target.ownerAccountId?.value ?? "");
    setOwnerLabel("");
    save.reset();
    setDialogOpen(true);
  }

  async function submit() {
    const response = await save.run();
    if (!response) return;
    setDialogOpen(false);
    notifySuccess(editTarget ? "Tenant updated" : "Tenant created");
    table.reload();
  }

  async function remove(target: Tenant) {
    const ok = await confirm({
      title: "Delete tenant?",
      description: `${target.name || target.entity?.id?.value}. This removes the workspace and its data.`,
      danger: true,
    });
    if (!ok) return;
    try {
      await api.tenantAdmin.deleteTenant(idMessage(target.entity?.id?.value));
      notifySuccess("Tenant deleted");
      table.reload();
    } catch (error) {
      notifyError(error, "Could not delete tenant");
    }
  }

  const columns = useMemo<EntityColumnDef<Tenant>[]>(
    () => [
      { id: "id", header: "Tenant", sortField: ListTenantsRequest_SortField.NAME, cell: ({ row }) => <span className="font-mono text-xs">{row.original.name || row.original.entity?.id?.value || "—"}</span> },
      { id: "owner", header: "Owner", cell: ({ row }) => <span className="font-mono text-xs">{formatId(row.original.ownerAccountId?.value)}</span> },
      { id: "members", header: "Members", cell: ({ row }) => row.original.members.length },
      { id: "created", header: "Created", sortField: ListTenantsRequest_SortField.CREATED_AT, cell: ({ row }) => formatTimestamp(row.original.entity?.timestamps?.createdAt) },
    ],
    [],
  );

  return (
    <>
      <EntityDataTable
        actions={[
          { label: "Edit", onSelect: openEdit },
          { label: "Delete", onSelect: remove },
        ]}
        columns={columns}
        data={result.data?.tenants ?? []}
        error={result.error}
        getRowId={(row) => row.entity?.id?.value ?? crypto.randomUUID()}
        headerActions={
          <Button size="sm" onClick={openCreate}>
            <Plus />
            New tenant
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
        searchPlaceholder="Search tenants"
        title="Platform Tenants"
      />

      <FormDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        title={editTarget ? "Edit tenant" : "New tenant"}
        submitLabel={editTarget ? "Save" : "Create"}
        onSubmit={submit}
        loading={save.loading}
        error={save.error}
        submitDisabled={editTarget ? false : !ownerId.trim()}
      >
        <div className="space-y-2">
          <Label htmlFor="tenant-name">Name</Label>
          <Input id="tenant-name" value={name} onChange={(event) => setName(event.target.value)} placeholder="acme-prod" autoFocus />
        </div>
        {!editTarget ? (
          <div className="space-y-2">
            <Label>Owner</Label>
            <AccountPicker
              tenantId={tenantId}
              value={ownerId}
              label={ownerLabel}
              onChange={(id, label) => {
                setOwnerId(id);
                setOwnerLabel(label);
              }}
            />
          </div>
        ) : null}
      </FormDialog>
    </>
  );
}
