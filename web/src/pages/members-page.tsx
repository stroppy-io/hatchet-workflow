import { useMemo, useState } from "react";
import { Plus } from "lucide-react";

import { AccountPicker } from "@/components/account-picker";
import { useConfirm } from "@/components/confirm-dialog";
import { EntityDataTable, type EntityColumnDef } from "@/components/data-table/entity-data-table";
import { DataBadge, FilterSelect } from "@/components/data-table/table-controls";
import { FormDialog } from "@/components/form-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useAuth } from "@/contexts/auth-context";
import { useAction } from "@/hooks/use-action";
import { useListQuery } from "@/hooks/use-list-query";
import { useTableState } from "@/hooks/use-table-state";
import { useTenantId } from "@/hooks/use-tenant-id";
import { api } from "@/lib/connect";
import { formatTimestamp, roleLabel } from "@/lib/format";
import { idMessage, tenantIdMessage } from "@/lib/proto";
import { notifyError, notifySuccess } from "@/lib/toast";
import { ListTenantMembersRequest_SortField, type TenantMemberRow } from "@/lib/proto/cloud/v1/api/ui/tenant_pb.ts";
import { TenantMember_Role } from "@/lib/proto/cloud/v1/models/tenant_pb.ts";

const ROLE_OPTIONS = [
  { label: "Viewer", value: String(TenantMember_Role.VIEWER) },
  { label: "Admin", value: String(TenantMember_Role.ADMIN) },
  { label: "Owner", value: String(TenantMember_Role.OWNER) },
];

export function MembersPage() {
  const tenantId = useTenantId();
  const { account } = useAuth();
  const isAdmin = Boolean(account?.isAdmin);
  const confirm = useConfirm();
  const [role, setRole] = useState("all");
  const table = useTableState(ListTenantMembersRequest_SortField.CREATED_AT, [tenantId, role]);

  const result = useListQuery(
    () =>
      api.tenant.listTenantMembers({
        tenantId: tenantIdMessage(tenantId),
        search: table.debouncedSearch || undefined,
        role: role === "all" ? undefined : (Number(role) as TenantMember_Role),
        sortField: table.sortField,
        order: table.order,
        page: table.pagination.page,
      }),
    [tenantId, role, table.debouncedSearch, table.sortField, table.order, table.pagination.page, table.reloadKey],
  );

  const [addOpen, setAddOpen] = useState(false);
  const [accountId, setAccountId] = useState("");
  const [accountLabel, setAccountLabel] = useState("");
  const [newRole, setNewRole] = useState(String(TenantMember_Role.VIEWER));

  const add = useAction(() =>
    api.tenant.addMemberToTenant({
      tenantId: tenantIdMessage(tenantId),
      accountId: idMessage(accountId.trim()),
      role: Number(newRole) as TenantMember_Role,
    }),
  );

  // Email lookup (non-admin owners resolve an account id by exact email).
  const [email, setEmail] = useState("");
  const lookup = useAction(() => api.tenant.lookupAccountByEmail({ tenantId: tenantIdMessage(tenantId), email: email.trim() }));

  async function doLookup() {
    const account = await lookup.run();
    if (!account) return;
    setAccountId(account.entity?.id?.value ?? "");
    setAccountLabel(account.email);
  }

  // Change-role dialog.
  const [roleTarget, setRoleTarget] = useState<TenantMemberRow | null>(null);
  const [editRole, setEditRole] = useState(String(TenantMember_Role.VIEWER));
  const updateRole = useAction(() =>
    api.tenant.updateMemberRole({
      tenantId: tenantIdMessage(tenantId),
      accountId: idMessage(roleTarget?.member?.accountId?.value ?? roleTarget?.account?.entity?.id?.value ?? ""),
      role: Number(editRole) as TenantMember_Role,
    }),
  );

  function openRole(row: TenantMemberRow) {
    setRoleTarget(row);
    setEditRole(String(row.member?.role ?? TenantMember_Role.VIEWER));
    updateRole.reset();
  }

  async function submitRole() {
    const response = await updateRole.run();
    if (!response) return;
    setRoleTarget(null);
    notifySuccess("Role updated");
    table.reload();
  }

  function openAdd() {
    setAccountId("");
    setAccountLabel("");
    setEmail("");
    setNewRole(String(TenantMember_Role.VIEWER));
    add.reset();
    lookup.reset();
    setAddOpen(true);
  }

  async function submitAdd() {
    const response = await add.run();
    if (!response) return;
    setAddOpen(false);
    notifySuccess("Member added");
    table.reload();
  }

  async function remove(row: TenantMemberRow) {
    const memberAccountId = row.member?.accountId?.value ?? row.account?.entity?.id?.value ?? "";
    const ok = await confirm({
      title: "Remove member?",
      description: row.account?.email ?? memberAccountId,
      danger: true,
      confirmLabel: "Remove",
    });
    if (!ok) return;
    try {
      await api.tenant.removeMemberFromTenant({ tenantId: tenantIdMessage(tenantId), accountId: idMessage(memberAccountId) });
      notifySuccess("Member removed");
      table.reload();
    } catch (error) {
      notifyError(error, "Could not remove member");
    }
  }

  const columns = useMemo<EntityColumnDef<TenantMemberRow>[]>(
    () => [
      { id: "email", header: "Email", cell: ({ row }) => row.original.account?.email ?? "—" },
      { id: "nickname", header: "Nickname", cell: ({ row }) => row.original.account?.nickname ?? "—" },
      { id: "role", header: "Role", sortField: ListTenantMembersRequest_SortField.ROLE, cell: ({ row }) => <DataBadge>{roleLabel(row.original.member?.role ?? TenantMember_Role.UNSPECIFIED)}</DataBadge> },
      { id: "created", header: "Created", sortField: ListTenantMembersRequest_SortField.CREATED_AT, cell: ({ row }) => formatTimestamp(row.original.member?.entity?.timestamps?.createdAt) },
    ],
    [],
  );

  return (
    <>
      <EntityDataTable
        actions={[
          { label: "Change role", onSelect: openRole },
          { label: "Remove", onSelect: remove },
        ]}
        columns={columns}
        data={result.data?.members ?? []}
        error={result.error}
        filters={<FilterSelect onChange={setRole} options={[{ label: "All roles", value: "all" }, ...ROLE_OPTIONS]} value={role} />}
        getRowId={(row) => row.member?.entity?.id?.value ?? row.account?.entity?.id?.value ?? crypto.randomUUID()}
        headerActions={
          <Button size="sm" onClick={openAdd}>
            <Plus />
            Add member
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
        searchPlaceholder="Search members"
        title="Members"
      />

      <FormDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        title="Add member"
        submitLabel="Add"
        onSubmit={submitAdd}
        loading={add.loading}
        error={add.error}
        submitDisabled={!accountId.trim()}
      >
        <div className="space-y-2">
          <Label>Account</Label>
          {isAdmin ? (
            <AccountPicker
              tenantId={tenantId}
              value={accountId}
              label={accountLabel}
              onChange={(id, label) => {
                setAccountId(id);
                setAccountLabel(label);
              }}
            />
          ) : (
            <div className="space-y-2">
              <div className="flex gap-2">
                <Input type="email" value={email} onChange={(event) => setEmail(event.target.value)} placeholder="member@email" autoFocus />
                <Button type="button" variant="outline" onClick={doLookup} disabled={!email.trim() || lookup.loading}>
                  Find
                </Button>
              </div>
              {accountLabel ? <p className="text-xs text-muted-foreground">Found: {accountLabel}</p> : null}
              {lookup.error ? <p className="text-xs text-destructive">{lookup.error}</p> : null}
            </div>
          )}
        </div>
        <div className="space-y-2">
          <Label>Role</Label>
          <Select value={newRole} onValueChange={setNewRole}>
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {ROLE_OPTIONS.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </FormDialog>

      <FormDialog
        open={roleTarget !== null}
        onOpenChange={(open) => !open && setRoleTarget(null)}
        title="Change role"
        description={roleTarget?.account?.email}
        submitLabel="Save"
        onSubmit={submitRole}
        loading={updateRole.loading}
        error={updateRole.error}
      >
        <div className="space-y-2">
          <Label>Role</Label>
          <Select value={editRole} onValueChange={setEditRole}>
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {ROLE_OPTIONS.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </FormDialog>
    </>
  );
}
