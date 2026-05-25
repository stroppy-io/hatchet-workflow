import { useMemo, useState } from "react";
import { Plus } from "lucide-react";

import { useConfirm } from "@/components/confirm-dialog";
import { EntityDataTable, type EntityColumnDef } from "@/components/data-table/entity-data-table";
import { DataBadge, FilterSelect } from "@/components/data-table/table-controls";
import { FormDialog } from "@/components/form-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { useAction } from "@/hooks/use-action";
import { useListQuery } from "@/hooks/use-list-query";
import { useTableState } from "@/hooks/use-table-state";
import { useTenantId } from "@/hooks/use-tenant-id";
import { api } from "@/lib/connect";
import { formatBool, formatTimestamp } from "@/lib/format";
import { idMessage, tenantIdMessage } from "@/lib/proto";
import { notifyError, notifySuccess } from "@/lib/toast";
import { ListAccountsRequest_SortField } from "@/lib/proto/cloud/v1/api/admin/account_pb.ts";
import type { Account } from "@/lib/proto/cloud/v1/models/account_pb.ts";

export function PlatformAccountsPage() {
  const tenantId = useTenantId();
  const confirm = useConfirm();
  const [isAdminFilter, setIsAdminFilter] = useState("all");
  const table = useTableState(ListAccountsRequest_SortField.CREATED_AT, [tenantId, isAdminFilter]);

  const result = useListQuery(
    () =>
      api.accountAdmin.listAccounts({
        tenantId: tenantIdMessage(tenantId),
        search: table.debouncedSearch || undefined,
        isAdmin: isAdminFilter === "all" ? undefined : isAdminFilter === "true",
        sortField: table.sortField,
        order: table.order,
        page: table.pagination.page,
      }),
    [tenantId, isAdminFilter, table.debouncedSearch, table.sortField, table.order, table.pagination.page, table.reloadKey],
  );

  // Create / edit dialog.
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<Account | null>(null);
  const [email, setEmail] = useState("");
  const [nickname, setNickname] = useState("");
  const [admin, setAdmin] = useState(false);
  const [password, setPassword] = useState("");

  const save = useAction(async () => {
    if (editTarget) {
      return api.accountAdmin.updateAccount({
        account: { entity: { id: idMessage(editTarget.entity?.id?.value) }, email, nickname, isAdmin: admin },
        updateMask: { paths: ["nickname", "is_admin"] },
      });
    }
    return api.accountAdmin.createAccount({
      account: { email: email.trim(), nickname: nickname.trim(), isAdmin: admin },
      password,
    });
  });

  // Reset-password dialog.
  const [pwTarget, setPwTarget] = useState<Account | null>(null);
  const [newPassword, setNewPassword] = useState("");
  const resetPw = useAction(() =>
    api.accountAdmin.updatePassword({ accountId: idMessage(pwTarget?.entity?.id?.value), newPassword }),
  );

  function openCreate() {
    setEditTarget(null);
    setEmail("");
    setNickname("");
    setAdmin(false);
    setPassword("");
    save.reset();
    setDialogOpen(true);
  }

  function openEdit(target: Account) {
    setEditTarget(target);
    setEmail(target.email);
    setNickname(target.nickname);
    setAdmin(target.isAdmin);
    setPassword("");
    save.reset();
    setDialogOpen(true);
  }

  async function submit() {
    const response = await save.run();
    if (!response) return;
    setDialogOpen(false);
    notifySuccess(editTarget ? "Account updated" : "Account created");
    table.reload();
  }

  function openResetPassword(target: Account) {
    setPwTarget(target);
    setNewPassword("");
    resetPw.reset();
  }

  async function submitResetPassword() {
    const response = await resetPw.run();
    if (!response) return;
    setPwTarget(null);
    notifySuccess("Password reset");
  }

  async function remove(target: Account) {
    const ok = await confirm({ title: `Delete account "${target.email}"?`, danger: true });
    if (!ok) return;
    try {
      await api.accountAdmin.deleteAccount(idMessage(target.entity?.id?.value));
      notifySuccess("Account deleted");
      table.reload();
    } catch (error) {
      notifyError(error, "Could not delete account");
    }
  }

  const columns = useMemo<EntityColumnDef<Account>[]>(
    () => [
      { accessorKey: "email", id: "email", header: "Email", sortField: ListAccountsRequest_SortField.EMAIL },
      { accessorKey: "nickname", id: "nickname", header: "Nickname", sortField: ListAccountsRequest_SortField.NICKNAME },
      { id: "admin", header: "Platform admin", cell: ({ row }) => <DataBadge>{formatBool(row.original.isAdmin)}</DataBadge> },
      { id: "created", header: "Created", sortField: ListAccountsRequest_SortField.CREATED_AT, cell: ({ row }) => formatTimestamp(row.original.entity?.timestamps?.createdAt) },
    ],
    [],
  );

  return (
    <>
      <EntityDataTable
        actions={[
          { label: "Edit", onSelect: openEdit },
          { label: "Reset password", onSelect: openResetPassword },
          { label: "Delete", onSelect: remove },
        ]}
        columns={columns}
        data={result.data?.accounts ?? []}
        error={result.error}
        filters={<FilterSelect onChange={setIsAdminFilter} options={[{ label: "All accounts", value: "all" }, { label: "Admins", value: "true" }, { label: "Regular", value: "false" }]} value={isAdminFilter} />}
        getRowId={(row) => row.entity?.id?.value ?? row.email}
        headerActions={
          <Button size="sm" onClick={openCreate}>
            <Plus />
            New account
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
        searchPlaceholder="Search accounts"
        title="Platform Accounts"
      />

      <FormDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        title={editTarget ? "Edit account" : "New account"}
        submitLabel={editTarget ? "Save" : "Create"}
        onSubmit={submit}
        loading={save.loading}
        error={save.error}
        submitDisabled={editTarget ? !nickname.trim() : !email.trim() || !nickname.trim() || password.length < 8}
      >
        <div className="space-y-2">
          <Label htmlFor="acc-email">Email</Label>
          <Input id="acc-email" type="email" value={email} onChange={(event) => setEmail(event.target.value)} disabled={Boolean(editTarget)} autoFocus={!editTarget} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="acc-nickname">Nickname</Label>
          <Input id="acc-nickname" value={nickname} onChange={(event) => setNickname(event.target.value)} />
        </div>
        {!editTarget ? (
          <div className="space-y-2">
            <Label htmlFor="acc-password">Password (min 8 chars)</Label>
            <Input id="acc-password" type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="new-password" />
          </div>
        ) : null}
        <div className="flex items-center justify-between">
          <Label htmlFor="acc-admin">Platform admin</Label>
          <Switch id="acc-admin" checked={admin} onCheckedChange={setAdmin} />
        </div>
      </FormDialog>

      <FormDialog
        open={pwTarget !== null}
        onOpenChange={(open) => !open && setPwTarget(null)}
        title="Reset password"
        description={pwTarget?.email}
        submitLabel="Reset"
        onSubmit={submitResetPassword}
        loading={resetPw.loading}
        error={resetPw.error}
        submitDisabled={newPassword.length < 1}
      >
        <div className="space-y-2">
          <Label htmlFor="acc-newpw">New password</Label>
          <Input id="acc-newpw" type="password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} autoComplete="new-password" autoFocus />
        </div>
      </FormDialog>
    </>
  );
}
