import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
  type VisibilityState,
} from "@tanstack/react-table";
import { ArrowDown, ArrowUp, ChevronsUpDown, Columns3, MoreHorizontal, Search } from "lucide-react";
import { useMemo, useState, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type { PageInfo, SortOrder } from "@/lib/proto/cloud/v1/models/common_pb.ts";
import { SortOrder as SortOrderEnum } from "@/lib/proto/cloud/v1/models/common_pb.ts";
import { cn } from "@/lib/utils";

export type RowAction<TData> = {
  label: string;
  onSelect: (row: TData) => void;
};

export type EntityColumnDef<TData, TValue = unknown> = ColumnDef<TData, TValue> & {
  sortField?: number;
};

type EntityDataTableProps<TData> = {
  actions?: RowAction<TData>[];
  columns: EntityColumnDef<TData>[];
  data: TData[];
  emptyMessage?: string;
  error?: string | null;
  filters?: ReactNode;
  headerActions?: ReactNode;
  getRowId?: (row: TData, index: number) => string;
  loading?: boolean;
  onNextPage?: () => void;
  onPreviousPage?: () => void;
  onSearchChange?: (value: string) => void;
  onSortChange?: (field: number, order: SortOrder) => void;
  onPageSizeChange?: (size: number) => void;
  pageIndex?: number;
  pageInfo?: PageInfo;
  pageSize?: number;
  search?: string;
  searchPlaceholder?: string;
  title: string;
};

export function EntityDataTable<TData>({
  actions,
  columns,
  data,
  emptyMessage = "No rows",
  error,
  filters,
  headerActions,
  getRowId,
  loading = false,
  onNextPage,
  onPreviousPage,
  onSearchChange,
  onSortChange,
  onPageSizeChange,
  pageIndex = 0,
  pageInfo,
  pageSize = 25,
  search = "",
  searchPlaceholder = "Search",
  title,
}: EntityDataTableProps<TData>) {
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({});
  const [rowSelection, setRowSelection] = useState({});
  const [sorting, setSorting] = useState<SortingState>([]);

  const tableColumns = useMemo<ColumnDef<TData>[]>(
    () => [
      {
        id: "__select",
        enableHiding: false,
        enableSorting: false,
        header: ({ table }) => (
          <Checkbox
            aria-label="Select all rows"
            checked={table.getIsAllPageRowsSelected() || (table.getIsSomePageRowsSelected() && "indeterminate")}
            onCheckedChange={(value) => table.toggleAllPageRowsSelected(Boolean(value))}
          />
        ),
        cell: ({ row }) => (
          <Checkbox
            aria-label="Select row"
            checked={row.getIsSelected()}
            onCheckedChange={(value) => row.toggleSelected(Boolean(value))}
          />
        ),
      },
      ...columns,
      ...(actions?.length
        ? [
            {
              id: "__actions",
              enableHiding: false,
              enableSorting: false,
              cell: ({ row }) => (
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button aria-label="Open row actions" size="icon-sm" variant="ghost">
                      <MoreHorizontal />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    {actions.map((action) => (
                      <DropdownMenuItem key={action.label} onSelect={() => action.onSelect(row.original)}>
                        {action.label}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
              ),
            } satisfies ColumnDef<TData>,
          ]
        : []),
    ],
    [actions, columns],
  );

  const table = useReactTable({
    columns: tableColumns,
    data,
    enableRowSelection: true,
    getCoreRowModel: getCoreRowModel(),
    getRowId,
    manualPagination: true,
    manualSorting: true,
    onColumnVisibilityChange: setColumnVisibility,
    onRowSelectionChange: setRowSelection,
    onSortingChange: (updater) => {
      const next = typeof updater === "function" ? updater(sorting) : updater;
      setSorting(next);
      const sort = next[0];
      const column = sort ? tableColumns.find((candidate) => candidate.id === sort.id) : undefined;
      const sortField = (column as EntityColumnDef<TData> | undefined)?.sortField;
      if (sortField !== undefined && onSortChange) {
        onSortChange(sortField, sort.desc ? SortOrderEnum.DESC : SortOrderEnum.ASC);
      }
    },
    state: {
      columnVisibility,
      rowSelection,
      sorting,
    },
  });

  const selectedCount = table.getFilteredSelectedRowModel().rows.length;
  const total = pageInfo?.total !== undefined ? Number(pageInfo.total) : undefined;

  return (
    <section className="flex min-h-0 flex-col gap-4 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-normal">{title}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {loading ? "Loading" : `${data.length} rows`}
            {total !== undefined ? ` of ${total}` : ""}
            {selectedCount > 0 ? `, ${selectedCount} selected` : ""}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {headerActions}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button size="sm" variant="outline">
                <Columns3 />
                Columns
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              {table
                .getAllColumns()
                .filter((column) => column.getCanHide())
                .map((column) => (
                  <DropdownMenuCheckboxItem
                    key={column.id}
                    checked={column.getIsVisible()}
                    onCheckedChange={(value) => column.toggleVisibility(Boolean(value))}
                  >
                    {column.id}
                  </DropdownMenuCheckboxItem>
                ))}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <div className="relative min-w-72 flex-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="pl-9"
            onChange={(event) => onSearchChange?.(event.target.value)}
            placeholder={searchPlaceholder}
            value={search}
          />
        </div>
        {filters}
      </div>

      {error ? <div className="border border-danger/40 bg-danger/10 px-3 py-2 text-sm text-danger">{error}</div> : null}

      <div className="overflow-hidden rounded-md border">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => {
                  const sortField = (header.column.columnDef as EntityColumnDef<TData>).sortField;
                  return (
                    <TableHead key={header.id} className={cn(header.id.startsWith("__") && "w-10")}>
                      {header.isPlaceholder ? null : sortField !== undefined ? (
                        <button
                          className="inline-flex items-center gap-1 text-left"
                          onClick={header.column.getToggleSortingHandler()}
                          type="button"
                        >
                          {flexRender(header.column.columnDef.header, header.getContext())}
                          {header.column.getIsSorted() === "asc" ? (
                            <ArrowUp className="size-3" />
                          ) : header.column.getIsSorted() === "desc" ? (
                            <ArrowDown className="size-3" />
                          ) : (
                            <ChevronsUpDown className="size-3 text-muted-foreground" />
                          )}
                        </button>
                      ) : (
                        flexRender(header.column.columnDef.header, header.getContext())
                      )}
                    </TableHead>
                  );
                })}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow key={row.id} data-state={row.getIsSelected() && "selected"}>
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell className="h-24 text-center text-muted-foreground" colSpan={tableColumns.length}>
                  {loading ? "Loading..." : emptyMessage}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="text-sm text-muted-foreground">Page {pageIndex + 1}</div>
        <div className="flex items-center gap-2">
          <Select onValueChange={(value) => onPageSizeChange?.(Number(value))} value={String(pageSize)}>
            <SelectTrigger className="h-8 w-24">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {[10, 25, 50, 100].map((size) => (
                <SelectItem key={size} value={String(size)}>
                  {size}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button disabled={!onPreviousPage || pageIndex === 0 || loading} onClick={onPreviousPage} size="sm" variant="outline">
            Previous
          </Button>
          <Button disabled={!onNextPage || !pageInfo?.hasMore || loading} onClick={onNextPage} size="sm" variant="outline">
            Next
          </Button>
        </div>
      </div>
    </section>
  );
}
