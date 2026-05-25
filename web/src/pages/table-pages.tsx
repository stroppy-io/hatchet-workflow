import { useMemo } from "react";
import { useNavigate } from "react-router-dom";

import { EntityDataTable, type EntityColumnDef } from "@/components/data-table/entity-data-table";
import { useListQuery } from "@/hooks/use-list-query";
import { useTableState } from "@/hooks/use-table-state";
import { useTenantId } from "@/hooks/use-tenant-id";
import { api } from "@/lib/connect";
import { formatId, formatTimestamp } from "@/lib/format";
import { tenantIdMessage } from "@/lib/proto";
import { tenantPath } from "@/lib/routes";
import type { SuiteRun, TestRun } from "@/lib/proto/cloud/v1/models/testing_pb.ts";
import { ListTestRunsRequest_SortField } from "@/lib/proto/cloud/v1/api/ui/run_pb.ts";
import { ListSuiteRunsRequest_SortField } from "@/lib/proto/cloud/v1/api/ui/suite_pb.ts";

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
