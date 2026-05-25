import { useMemo, useState } from "react";
import { Plus } from "lucide-react";
import { useNavigate } from "react-router-dom";

import { AutoRefresh } from "@/components/auto-refresh";
import { EntityDataTable, type EntityColumnDef } from "@/components/data-table/entity-data-table";
import { StatusBadge } from "@/components/status-badge";
import { Button } from "@/components/ui/button";
import { useListQuery } from "@/hooks/use-list-query";
import { useTableState } from "@/hooks/use-table-state";
import { useTenantId } from "@/hooks/use-tenant-id";
import { api } from "@/lib/connect";
import { databaseColor } from "@/lib/db-colors";
import { formatDuration, formatId, formatTimestamp, formatTimestampShort } from "@/lib/format";
import { databaseKindLabel, topologySummary } from "@/lib/preset-summary";
import { tenantIdMessage } from "@/lib/proto";
import { tenantPath } from "@/lib/routes";
import { cn } from "@/lib/utils";
import { Status } from "@/lib/proto/cloud/v1/runtime/primitive/status_pb.ts";
import type { Account } from "@/lib/proto/cloud/v1/models/account_pb.ts";
import type { Workload_Execution } from "@/lib/proto/cloud/v1/domain/workload_pb.ts";
import type { SuiteRun, TestRun } from "@/lib/proto/cloud/v1/models/testing_pb.ts";
import { ListTestRunsRequest_SortField } from "@/lib/proto/cloud/v1/api/ui/run_pb.ts";
import type { RunTiming } from "@/lib/proto/cloud/v1/api/ui/run_pb.ts";
import { ListSuiteRunsRequest_SortField } from "@/lib/proto/cloud/v1/api/ui/suite_pb.ts";

export function RunsPage() {
  const tenantId = useTenantId();
  const navigate = useNavigate();
  const [statusFilter, setStatusFilter] = useState("all");
  const table = useTableState(ListTestRunsRequest_SortField.CREATED_AT, [tenantId, statusFilter], "offset");
  const result = useListQuery(
    () =>
      api.run.listTestRuns({
        tenantId: tenantIdMessage(tenantId),
        search: table.debouncedSearch || undefined,
        status: statusFilter === "all" ? undefined : (Number(statusFilter) as Status),
        sortField: table.sortField,
        order: table.order,
        page: table.pagination.page,
      }),
    [tenantId, statusFilter, table.debouncedSearch, table.sortField, table.order, table.pagination.page, table.reloadKey],
  );

  const ownersById = useMemo(() => {
    const map = new Map<string, Account>();
    for (const owner of result.data?.owners ?? []) map.set(owner.entity?.id?.value ?? "", owner);
    return map;
  }, [result.data?.owners]);

  const timingsById = useMemo(() => {
    const map = new Map<string, RunTiming>();
    for (const timing of result.data?.timings ?? []) map.set(timing.runId?.value ?? "", timing);
    return map;
  }, [result.data?.timings]);

  // Total pages from the server count (offset pagination) — page buttons show from
  // the first load and any page is reachable.
  const pageCount = result.data?.pageInfo?.total != null
    ? Math.ceil(Number(result.data.pageInfo.total) / table.pagination.pageSize)
    : 0;

  const columns = useMemo<EntityColumnDef<TestRun>[]>(
    () => [
      {
        accessorKey: "name",
        id: "name",
        header: "Name",
        sortField: ListTestRunsRequest_SortField.NAME,
        cell: ({ row }) => {
          const id = row.original.entity?.id?.value ?? "";
          const name = row.original.name;
          return (
            <span className="text-primary">
              {name || formatId(id)}
              {name ? <span className="ml-2 font-mono text-[11px] text-muted-foreground">{formatId(id)}</span> : null}
            </span>
          );
        },
      },
      {
        id: "status",
        header: "Status",
        filter: { options: STATUS_FILTER_OPTIONS, value: statusFilter, onChange: setStatusFilter },
        cell: ({ row }) => <StatusBadge status={row.original.status} />,
      },
      {
        id: "progress",
        header: "Progress",
        cell: ({ row }) => <RunProgress status={row.original.status} />,
      },
      {
        id: "database",
        header: "Database",
        cell: ({ row }) => {
          const db = row.original.testPreset?.database;
          if (!db) return <span className="text-muted-foreground">—</span>;
          return (
            <span style={{ color: databaseColor(db.kind) }}>
              {databaseKindLabel(db.kind)}
              {db.version ? ` ${db.version}` : ""}
            </span>
          );
        },
      },
      {
        id: "workload",
        header: "Workload",
        cell: ({ row }) => {
          const wl = row.original.testPreset?.workload;
          if (!wl) return <span className="text-muted-foreground">—</span>;
          return (
            <span>
              {wl.script || "—"}
              {workloadLimit(wl.execution) ? (
                <span className="ml-2 font-mono text-[11px] text-muted-foreground">{workloadLimit(wl.execution)}</span>
              ) : null}
            </span>
          );
        },
      },
      {
        id: "topology",
        header: "Topology",
        cell: ({ row }) => <span className="text-xs text-muted-foreground">{topologySummary(row.original.testPreset?.topology)}</span>,
      },
      {
        id: "author",
        header: "Author",
        cell: ({ row }) => {
          const id = row.original.owned?.ownerAccountId?.value ?? "";
          const owner = ownersById.get(id);
          return <span className="text-xs text-muted-foreground">{owner?.email || owner?.nickname || formatId(id)}</span>;
        },
      },
      {
        id: "duration",
        header: "Duration",
        cell: ({ row }) => {
          const timing = timingsById.get(row.original.entity?.id?.value ?? "");
          return (
            <span className="text-xs text-muted-foreground">
              {timing ? formatDuration(timing.startedAt, timing.finishedAt) : "—"}
            </span>
          );
        },
      },
      {
        id: "created",
        header: "Created",
        sortField: ListTestRunsRequest_SortField.CREATED_AT,
        cell: ({ row }) => <span className="text-xs text-muted-foreground">{formatTimestampShort(row.original.entity?.timestamps?.createdAt)}</span>,
      },
    ],
    [ownersById, statusFilter, timingsById],
  );

  return (
    <EntityDataTable
      columns={columns}
      data={result.data?.testRuns ?? []}
      error={result.error}
      getRowId={(row) => row.entity?.id?.value ?? crypto.randomUUID()}
      filters={<AutoRefresh onRefresh={table.reload} />}
      onRowClick={(row) => navigate(tenantPath(tenantId, `/runs/${row.entity?.id?.value ?? ""}`))}
      selectionActions={(rows) =>
        rows.length >= 2 ? (
          <Button
            size="sm"
            variant="outline"
            onClick={() => navigate(tenantPath(tenantId, `/compare?runs=${rows.map((r) => r.entity?.id?.value ?? "").join(",")}`))}
          >
            Compare {rows.length}
          </Button>
        ) : (
          <span className="text-xs text-muted-foreground">select 2+ to compare</span>
        )
      }
      headerActions={
        <Button size="sm" onClick={() => navigate(tenantPath(tenantId, "/wizard"))}>
          <Plus />
          New test
        </Button>
      }
      loading={result.loading}
      onNextPage={() => table.pagination.next(result.data?.pageInfo)}
      onGoToPage={table.pagination.goTo}
      pageCount={pageCount}
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

const STATUS_PROGRESS: Record<number, { pct: number; cls: string }> = {
  [Status.PENDING]: { pct: 6, cls: "bg-muted-foreground/40" },
  [Status.RUNNING]: { pct: 60, cls: "bg-primary" },
  [Status.RETRY_WAIT]: { pct: 50, cls: "bg-warning" },
  [Status.CANCELLING]: { pct: 80, cls: "bg-warning" },
  [Status.COMPLETED]: { pct: 100, cls: "bg-success" },
  [Status.FAILED]: { pct: 100, cls: "bg-destructive" },
  [Status.CANCELLED]: { pct: 100, cls: "bg-muted-foreground/50" },
  [Status.SKIPPED]: { pct: 100, cls: "bg-muted-foreground/40" },
};

function RunProgress({ status }: { status: Status }) {
  const p = STATUS_PROGRESS[status] ?? { pct: 0, cls: "bg-muted-foreground/40" };
  return (
    <div className="h-1.5 w-24 overflow-hidden rounded-full bg-muted">
      <div className={cn("h-full transition-all", p.cls)} style={{ width: `${p.pct}%` }} />
    </div>
  );
}

const STATUS_FILTER_OPTIONS = [
  { label: "All statuses", value: "all" },
  { label: "Pending", value: String(Status.PENDING) },
  { label: "Running", value: String(Status.RUNNING) },
  { label: "Completed", value: String(Status.COMPLETED) },
  { label: "Failed", value: String(Status.FAILED) },
  { label: "Cancelled", value: String(Status.CANCELLED) },
];

// Workload run limit (k6 --duration | --iterations) for the Runs table row.
function workloadLimit(execution?: Workload_Execution): string {
  const limit = execution?.limit;
  if (limit?.case === "duration") return limit.value;
  if (limit?.case === "iterations") return `${limit.value} iter`;
  return "";
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
