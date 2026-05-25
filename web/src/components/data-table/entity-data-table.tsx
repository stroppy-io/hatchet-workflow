import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
  type VisibilityState,
} from "@tanstack/react-table";
import { ArrowDown, ArrowUp, ChevronsUpDown, Columns3, ListFilter, MoreHorizontal, Search } from "lucide-react";
import { useMemo, useState, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
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

// ColumnFilter renders an enum/value filter inside the column header. By
// convention options[0] is the "no filter" choice (active state highlights when
// value differs from it).
export type ColumnFilter = {
  options: { label: string; value: string }[];
  value: string;
  onChange: (value: string) => void;
};

export type EntityColumnDef<TData, TValue = unknown> = ColumnDef<TData, TValue> & {
  sortField?: number;
  filter?: ColumnFilter;
};

// pageWindow returns the page indices to render (with "…" gaps): always the
// first/last page and a window around the current one, so large page counts stay
// compact.
function pageWindow(current: number, count: number): (number | "ellipsis")[] {
  const shown = new Set<number>([0, count - 1]);
  for (let i = current - 1; i <= current + 1; i++) {
    if (i >= 0 && i < count) shown.add(i);
  }
  const sorted = [...shown].sort((a, b) => a - b);
  const out: (number | "ellipsis")[] = [];
  let prev = -1;
  for (const page of sorted) {
    if (prev >= 0 && page - prev > 1) out.push("ellipsis");
    out.push(page);
    prev = page;
  }
  return out;
}

function HeaderFilter({ filter }: { filter: ColumnFilter }) {
  const active = filter.value !== filter.options[0]?.value;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button aria-label="Filter column" size="icon-sm" variant="ghost" className={cn("size-6", active && "text-primary")}>
          <ListFilter className="size-3.5" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start">
        <DropdownMenuRadioGroup value={filter.value} onValueChange={filter.onChange}>
          {filter.options.map((option) => (
            <DropdownMenuRadioItem key={option.value} value={option.value}>
              {option.label}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

type EntityDataTableProps<TData> = {
  actions?: RowAction<TData>[];
  columns: EntityColumnDef<TData>[];
  data: TData[];
  emptyMessage?: string;
  error?: string | null;
  filters?: ReactNode;
  headerActions?: ReactNode;
  onRowClick?: (row: TData) => void;
  selectionActions?: (rows: TData[]) => ReactNode;
  getRowId?: (row: TData, index: number) => string;
  loading?: boolean;
  onNextPage?: () => void;
  onPreviousPage?: () => void;
  onGoToPage?: (index: number) => void;
  onSearchChange?: (value: string) => void;
  onSortChange?: (field: number, order: SortOrder) => void;
  onPageSizeChange?: (size: number) => void;
  pageIndex?: number;
  pageCount?: number;
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
  onRowClick,
  selectionActions,
  getRowId,
  loading = false,
  onNextPage,
  onPreviousPage,
  onGoToPage,
  onSearchChange,
  onSortChange,
  onPageSizeChange,
  pageIndex = 0,
  pageCount = 0,
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

  const selectedRows = table.getFilteredSelectedRowModel().rows.map((r) => r.original);

  return (
    <section className="flex h-full min-h-0 flex-col gap-4 p-6">
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
          {selectedCount > 0 ? selectionActions?.(selectedRows) : null}
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

      <div className="min-h-0 flex-1 overflow-auto rounded-md border">
        <Table containerClassName="overflow-visible">
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => {
                  const columnDef = header.column.columnDef as EntityColumnDef<TData>;
                  const sortField = columnDef.sortField;
                  return (
                    <TableHead key={header.id} className={cn("sticky top-0 z-10 bg-card", header.id.startsWith("__") && "w-10")}>
                      {header.isPlaceholder ? null : (
                        <div className="flex items-center gap-1">
                          {sortField !== undefined ? (
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
                          {columnDef.filter ? <HeaderFilter filter={columnDef.filter} /> : null}
                        </div>
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
                <TableRow
                  key={row.id}
                  data-state={row.getIsSelected() && "selected"}
                  className={onRowClick ? "cursor-pointer" : undefined}
                  onClick={onRowClick ? () => onRowClick(row.original) : undefined}
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell
                      key={cell.id}
                      onClick={cell.column.id.startsWith("__") ? (event) => event.stopPropagation() : undefined}
                    >
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
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
        <Select onValueChange={(value) => onPageSizeChange?.(Number(value))} value={String(pageSize)}>
          <SelectTrigger className="h-8 w-28">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {[10, 15, 20, 25, 50, 100].map((size) => (
              <SelectItem key={size} value={String(size)}>
                {size} / page
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <div className="flex max-w-full items-center gap-1 overflow-x-auto">
          <Button disabled={!onPreviousPage || pageIndex === 0 || loading} onClick={onPreviousPage} size="sm" variant="outline">
            Previous
          </Button>
          {onGoToPage && pageCount > 1 ? (
            pageWindow(pageIndex, pageCount).map((entry, i) =>
              entry === "ellipsis" ? (
                <span key={`gap-${i}`} className="px-1 text-sm text-muted-foreground">
                  …
                </span>
              ) : (
                <Button
                  key={entry}
                  size="icon-sm"
                  variant={entry === pageIndex ? "default" : "outline"}
                  disabled={loading}
                  onClick={() => onGoToPage(entry)}
                >
                  {entry + 1}
                </Button>
              ),
            )
          ) : (
            <span className="px-2 text-sm text-muted-foreground">Page {pageIndex + 1}</span>
          )}
          <Button disabled={!onNextPage || !pageInfo?.hasMore || loading} onClick={onNextPage} size="sm" variant="outline">
            Next
          </Button>
        </div>
      </div>
    </section>
  );
}
