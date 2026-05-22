import type { SortingState } from "@tanstack/react-table";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { EntityDataTable, type EntityColumnDef } from "@/components/data-table/entity-data-table";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useCursorPagination } from "@/hooks/use-cursor-pagination";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { useListQuery } from "@/hooks/use-list-query";
import { useTenantId } from "@/hooks/use-tenant-id";
import { api } from "@/lib/connect";
import {
  formatBool,
  formatId,
  formatTimestamp,
  roleLabel,
  webhookEventLabel,
} from "@/lib/format";
import type { Account } from "@/lib/proto/cloud/v1/models/account_pb.ts";
import { SortOrder } from "@/lib/proto/cloud/v1/models/common_pb.ts";
import type { Package } from "@/lib/proto/cloud/v1/models/package_pb.ts";
import type { ApiToken } from "@/lib/proto/cloud/v1/models/apitoken_pb.ts";
import type { SuiteRun, TestRun } from "@/lib/proto/cloud/v1/models/testing_pb.ts";
import type { Tenant } from "@/lib/proto/cloud/v1/models/tenant_pb.ts";
import { TenantMember_Role } from "@/lib/proto/cloud/v1/models/tenant_pb.ts";
import type { Webhook } from "@/lib/proto/cloud/v1/models/webhook_pb.ts";
import { Webhook_Event } from "@/lib/proto/cloud/v1/models/webhook_pb.ts";
import { ListAccountsRequest_SortField } from "@/lib/proto/cloud/v1/api/admin/account_pb.ts";
import { ListTenantsRequest_SortField } from "@/lib/proto/cloud/v1/api/admin/tenant_pb.ts";
import { ListApiTokensRequest_SortField } from "@/lib/proto/cloud/v1/api/ui/apitoken_pb.ts";
import { ListPackagesRequest_SortField } from "@/lib/proto/cloud/v1/api/ui/package_pb.ts";
import { ListTestRunsRequest_SortField } from "@/lib/proto/cloud/v1/api/ui/run_pb.ts";
import { ListSuiteRunsRequest_SortField } from "@/lib/proto/cloud/v1/api/ui/suite_pb.ts";
import { ListTenantMembersRequest_SortField, type TenantMemberRow } from "@/lib/proto/cloud/v1/api/ui/tenant_pb.ts";
import { ListWebhooksRequest_SortField } from "@/lib/proto/cloud/v1/api/ui/webhook_pb.ts";
import { tenantPath } from "@/lib/routes";

type ListState = {
  order: SortOrder;
  search: string;
  sortField: number;
};

function useTableState(defaultSortField: number, extraResetDeps: unknown[] = []) {
  const [state, setState] = useState<ListState>({
    order: SortOrder.DESC,
    search: "",
    sortField: defaultSortField,
  });
  const debouncedSearch = useDebouncedValue(state.search);
  const pagination = useCursorPagination();

  const setSearch = useCallback(
    (search: string) => {
      setState((current) => ({ ...current, search }));
      pagination.reset();
    },
    [pagination],
  );

  const setSort = useCallback(
    (sortField: number, order: SortOrder) => {
      setState((current) => ({ ...current, sortField, order }));
      pagination.reset();
    },
    [pagination],
  );

  useEffect(() => {
    pagination.reset();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, extraResetDeps);

  return { ...state, debouncedSearch, pagination, setSearch, setSort };
}

function tenantIdMessage(value: string) {
  return { value };
}

function FilterSelect({
  onChange,
  options,
  value,
}: {
  onChange: (value: string) => void;
  options: { label: string; value: string }[];
  value: string;
}) {
  return (
    <Select onValueChange={onChange} value={value}>
      <SelectTrigger className="h-9 w-44">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {options.map((option) => (
          <SelectItem key={option.value} value={option.value}>
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

function DataBadge({ children }: { children: string }) {
  return <Badge variant="secondary">{children}</Badge>;
}

export function RunsPage() {
  const tenantId = useTenantId();
  const navigate = useNavigate();
  const table = useTableState(ListTestRunsRequest_SortField.CREATED_AT, [tenantId]);
  const result = useListQuery(
    () =>
      api.run.listTestRuns({
        tenantId: tenantIdMessage(tenantId),
        search: table.debouncedSearch || undefined,
        sortField: table.sortField,
        order: table.order,
        page: table.pagination.page,
      }),
    [tenantId, table.debouncedSearch, table.sortField, table.order, table.pagination.page],
  );

  const columns = useMemo<EntityColumnDef<TestRun>[]>(
    () => [
      {
        accessorKey: "name",
        id: "name",
        header: "Name",
        sortField: ListTestRunsRequest_SortField.NAME,
        cell: ({ row }) => row.original.name || formatId(row.original.entity?.id?.value),
      },
      {
        id: "dag",
        header: "DAG",
        cell: ({ row }) => <span className="font-mono text-xs">{formatId(row.original.dag?.value)}</span>,
      },
      {
        id: "suite",
        header: "Suite run",
        cell: ({ row }) => <span className="font-mono text-xs">{formatId(row.original.suiteRunId?.value)}</span>,
      },
      {
        id: "created",
        header: "Created",
        sortField: ListTestRunsRequest_SortField.CREATED_AT,
        cell: ({ row }) => formatTimestamp(row.original.entity?.timestamps?.createdAt),
      },
    ],
    [],
  );

  return (
    <EntityDataTable
      actions={[{ label: "Open", onSelect: (row) => navigate(tenantPath(tenantId, `/runs/${row.entity?.id?.value ?? ""}`)) }]}
      columns={columns}
      data={result.data?.testRuns ?? []}
      error={result.error}
      getRowId={(row) => row.entity?.id?.value ?? crypto.randomUUID()}
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
      searchPlaceholder="Search runs"
      title="Runs"
    />
  );
}

export function SuiteRunsPage() {
  const tenantId = useTenantId();
  const table = useTableState(ListSuiteRunsRequest_SortField.CREATED_AT, [tenantId]);
  const result = useListQuery(
    () =>
      api.suite.listSuiteRuns({
        tenantId: tenantIdMessage(tenantId),
        sortField: table.sortField,
        order: table.order,
        page: table.pagination.page,
      }),
    [tenantId, table.sortField, table.order, table.pagination.page],
  );

  const columns = useMemo<EntityColumnDef<SuiteRun>[]>(
    () => [
      { id: "suite", header: "Suite", cell: ({ row }) => <span className="font-mono text-xs">{formatId(row.original.suiteId?.value)}</span> },
      { id: "dag", header: "DAG", cell: ({ row }) => <span className="font-mono text-xs">{formatId(row.original.dag?.value)}</span> },
      { id: "tests", header: "Test runs", cell: ({ row }) => row.original.testRuns.length },
      { id: "created", header: "Created", sortField: ListSuiteRunsRequest_SortField.CREATED_AT, cell: ({ row }) => formatTimestamp(row.original.entity?.timestamps?.createdAt) },
    ],
    [],
  );

  return (
    <EntityDataTable
      columns={columns}
      data={result.data?.suiteRuns ?? []}
      error={result.error}
      getRowId={(row) => row.entity?.id?.value ?? crypto.randomUUID()}
      loading={result.loading}
      onNextPage={() => table.pagination.next(result.data?.pageInfo)}
      onPageSizeChange={table.pagination.setPageSize}
      onPreviousPage={table.pagination.previous}
      onSortChange={table.setSort}
      pageIndex={table.pagination.index}
      pageInfo={result.data?.pageInfo}
      pageSize={table.pagination.pageSize}
      title="Suite runs"
    />
  );
}

export function PackagesPage() {
  const tenantId = useTenantId();
  const [builtin, setBuiltin] = useState("all");
  const table = useTableState(ListPackagesRequest_SortField.CREATED_AT, [tenantId, builtin]);
  const result = useListQuery(
    () =>
      api.package.listPackages({
        tenantId: tenantIdMessage(tenantId),
        search: table.debouncedSearch || undefined,
        isBuiltin: builtin === "all" ? undefined : builtin === "true",
        sortField: table.sortField,
        order: table.order,
        page: table.pagination.page,
      }),
    [tenantId, builtin, table.debouncedSearch, table.sortField, table.order, table.pagination.page],
  );

  const columns = useMemo<EntityColumnDef<Package>[]>(
    () => [
      { accessorKey: "name", id: "name", header: "Name", sortField: ListPackagesRequest_SortField.NAME },
      { accessorKey: "dbKind", id: "dbKind", header: "DB kind", sortField: ListPackagesRequest_SortField.DB_KIND },
      { accessorKey: "dbVersion", id: "dbVersion", header: "Version" },
      { id: "origin", header: "Origin", cell: ({ row }) => <DataBadge>{row.original.isBuiltin ? "Builtin" : "Custom"}</DataBadge> },
      { id: "checksum", header: "Checksum", cell: ({ row }) => <span className="font-mono text-xs">{formatId(row.original.checksum)}</span> },
    ],
    [],
  );

  return (
    <EntityDataTable
      columns={columns}
      data={result.data?.packages ?? []}
      error={result.error}
      filters={<FilterSelect onChange={setBuiltin} options={[{ label: "All packages", value: "all" }, { label: "Builtin", value: "true" }, { label: "Custom", value: "false" }]} value={builtin} />}
      getRowId={(row) => row.entity?.id?.value ?? row.name}
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
      searchPlaceholder="Search packages"
      title="Packages"
    />
  );
}

export function WebhooksPage() {
  const tenantId = useTenantId();
  const [enabled, setEnabled] = useState("all");
  const table = useTableState(ListWebhooksRequest_SortField.CREATED_AT, [tenantId, enabled]);
  const result = useListQuery(
    () =>
      api.webhook.listWebhooks({
        tenantId: tenantIdMessage(tenantId),
        search: table.debouncedSearch || undefined,
        enabled: enabled === "all" ? undefined : enabled === "true",
        sortField: table.sortField,
        order: table.order,
        page: table.pagination.page,
      }),
    [tenantId, enabled, table.debouncedSearch, table.sortField, table.order, table.pagination.page],
  );

  const columns = useMemo<EntityColumnDef<Webhook>[]>(
    () => [
      { accessorKey: "url", id: "url", header: "URL", sortField: ListWebhooksRequest_SortField.URL },
      { id: "enabled", header: "Enabled", cell: ({ row }) => <DataBadge>{formatBool(row.original.enabled)}</DataBadge> },
      { id: "events", header: "Events", cell: ({ row }) => row.original.events.map(webhookEventLabel).join(", ") || "—" },
      { id: "created", header: "Created", sortField: ListWebhooksRequest_SortField.CREATED_AT, cell: ({ row }) => formatTimestamp(row.original.entity?.timestamps?.createdAt) },
    ],
    [],
  );

  return (
    <EntityDataTable
      columns={columns}
      data={result.data?.webhooks ?? []}
      error={result.error}
      filters={<FilterSelect onChange={setEnabled} options={[{ label: "All states", value: "all" }, { label: "Enabled", value: "true" }, { label: "Disabled", value: "false" }]} value={enabled} />}
      getRowId={(row) => row.entity?.id?.value ?? row.url}
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
      searchPlaceholder="Search webhooks"
      title="Webhooks"
    />
  );
}

export function TokensPage() {
  const tenantId = useTenantId();
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
    [tenantId, role, table.debouncedSearch, table.sortField, table.order, table.pagination.page],
  );

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
    <EntityDataTable
      columns={columns}
      data={result.data?.apiTokens ?? []}
      error={result.error}
      filters={<FilterSelect onChange={setRole} options={[{ label: "All roles", value: "all" }, { label: "Viewer", value: String(TenantMember_Role.VIEWER) }, { label: "Admin", value: String(TenantMember_Role.ADMIN) }]} value={role} />}
      getRowId={(row) => row.entity?.id?.value ?? row.name}
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
  );
}

export function MembersPage() {
  const tenantId = useTenantId();
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
    [tenantId, role, table.debouncedSearch, table.sortField, table.order, table.pagination.page],
  );

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
    <EntityDataTable
      columns={columns}
      data={result.data?.members ?? []}
      error={result.error}
      filters={<FilterSelect onChange={setRole} options={[{ label: "All roles", value: "all" }, { label: "Viewer", value: String(TenantMember_Role.VIEWER) }, { label: "Admin", value: String(TenantMember_Role.ADMIN) }, { label: "Owner", value: String(TenantMember_Role.OWNER) }]} value={role} />}
      getRowId={(row) => row.member?.entity?.id?.value ?? row.account?.entity?.id?.value ?? crypto.randomUUID()}
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
  );
}

export function PlatformAccountsPage() {
  const tenantId = useTenantId();
  const [isAdmin, setIsAdmin] = useState("all");
  const table = useTableState(ListAccountsRequest_SortField.CREATED_AT, [tenantId, isAdmin]);
  const result = useListQuery(
    () =>
      api.accountAdmin.listAccounts({
        tenantId: tenantIdMessage(tenantId),
        search: table.debouncedSearch || undefined,
        isAdmin: isAdmin === "all" ? undefined : isAdmin === "true",
        sortField: table.sortField,
        order: table.order,
        page: table.pagination.page,
      }),
    [tenantId, isAdmin, table.debouncedSearch, table.sortField, table.order, table.pagination.page],
  );

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
    <EntityDataTable
      columns={columns}
      data={result.data?.accounts ?? []}
      error={result.error}
      filters={<FilterSelect onChange={setIsAdmin} options={[{ label: "All accounts", value: "all" }, { label: "Admins", value: "true" }, { label: "Regular", value: "false" }]} value={isAdmin} />}
      getRowId={(row) => row.entity?.id?.value ?? row.email}
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
  );
}

export function PlatformTenantsPage() {
  const tenantId = useTenantId();
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
    [tenantId, table.debouncedSearch, table.sortField, table.order, table.pagination.page],
  );

  const columns = useMemo<EntityColumnDef<Tenant>[]>(
    () => [
      { id: "id", header: "Tenant", sortField: ListTenantsRequest_SortField.NAME, cell: ({ row }) => <span className="font-mono text-xs">{row.original.entity?.id?.value ?? "—"}</span> },
      { id: "owner", header: "Owner", cell: ({ row }) => <span className="font-mono text-xs">{formatId(row.original.ownerAccountId?.value)}</span> },
      { id: "members", header: "Members", cell: ({ row }) => row.original.members.length },
      { id: "created", header: "Created", sortField: ListTenantsRequest_SortField.CREATED_AT, cell: ({ row }) => formatTimestamp(row.original.entity?.timestamps?.createdAt) },
    ],
    [],
  );

  return (
    <EntityDataTable
      columns={columns}
      data={result.data?.tenants ?? []}
      error={result.error}
      getRowId={(row) => row.entity?.id?.value ?? crypto.randomUUID()}
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
  );
}
