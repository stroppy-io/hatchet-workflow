// Shared Library table chrome — the reusable module behind all four Library
// pages (Database / Workload / Test presets + Packages). It mirrors the Test
// Runs table (src/pages/Runs.tsx) one-to-one: a single internal-scroll region
// with a sticky <thead>, per-column header controls = `[⇅ sort] Label [⛃ filter]`
// with a CONTROLLED-open filter popover, tri-state column sort, a page-size
// selector and opaque page-token prev/next paging — all on the same dark tokens.
//
// The four pages differ only in their ROWS, COLUMNS and the slice of their List
// RPC they wire, so the table itself is generic over a row type <T>: a page
// hands it a column list (each column declares its own sort field + an optional
// filter popover body) and a data page. Filter/sort/paging STATE lives in the
// URL (useLibraryTableState below) exactly like Runs, so Back works and views
// are shareable. Visual + behavioural parity with Runs is the requirement.

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
  type RowData,
} from "@tanstack/react-table";
import {
  AlertCircle,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  ChevronsUpDown,
  Filter,
  Loader2,
  X,
} from "lucide-react";
import { useSearchParams } from "@/lib/router";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";

// Per-column metadata: an optional className applied to both <th> and <td> so a
// column can carry its own width / padding, matching the Runs convention.
declare module "@tanstack/react-table" {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    className?: string;
    /** Cells that should not trigger row navigation (utility columns). */
    disableRowNavigation?: boolean;
  }
}

export const PAGE_SIZES = [15, 25, 50, 100, 150];
export const DEFAULT_PAGE_SIZE = 25;

// --- header building blocks (extracted verbatim-in-spirit from Runs). -------

const HEADER_BTN =
  "flex h-6 w-6 items-center justify-center rounded transition-colors cursor-pointer shrink-0";
const HEADER_ICON = "h-3.5 w-3.5";

function SortButton({
  active,
  desc,
  onSort,
}: {
  active: boolean;
  desc: boolean;
  onSort: () => void;
}) {
  const Icon = !active ? ChevronsUpDown : desc ? ChevronDown : ChevronUp;
  return (
    <button
      type="button"
      onClick={onSort}
      className={cn(
        HEADER_BTN,
        active ? "text-primary" : "text-zinc-600 hover:text-zinc-400",
      )}
      title={
        !active
          ? "Sort"
          : desc
            ? "Sorted descending — click for neutral"
            : "Sorted ascending — click for descending"
      }
      aria-label="Toggle sort"
    >
      <Icon className={HEADER_ICON} />
    </button>
  );
}

/**
 * ColumnHeader — the three-slot leaf header `[⇅ sort] Label [⛃ filter]`. The
 * sort + filter buttons share one hit-area/icon size so they align column to
 * column; missing slots reserve their width. The filter popover open-state is
 * LIFTED to the table (`openColumnId`) so a re-render / refetch can't reset it
 * and the input keeps focus while typing — identical to Runs.
 */
export function ColumnHeader({
  label,
  sortField,
  sort,
  desc,
  onSort,
  filterActive,
  children,
  columnId,
  openColumnId,
  onOpenChange,
}: {
  label: string;
  sortField?: string;
  sort?: string;
  desc: boolean;
  onSort: (field: string) => void;
  filterActive?: boolean;
  children?: ReactNode;
  columnId?: string;
  openColumnId?: string | null;
  onOpenChange?: (columnId: string, open: boolean) => void;
}) {
  const isActiveSort = sortField !== undefined && sort === sortField;
  const isOpen = !!columnId && openColumnId === columnId;
  return (
    <div className="flex items-center justify-start gap-1.5">
      {sortField ? (
        <SortButton
          active={isActiveSort}
          desc={desc}
          onSort={() => onSort(sortField)}
        />
      ) : (
        <span className="h-6 w-6 shrink-0" aria-hidden />
      )}

      <span className="whitespace-nowrap leading-none">{label}</span>

      {children && columnId ? (
        <Popover open={isOpen} onOpenChange={(o) => onOpenChange?.(columnId, o)}>
          <PopoverTrigger asChild>
            <button
              type="button"
              className={cn(
                HEADER_BTN,
                filterActive
                  ? "text-primary"
                  : "text-zinc-600 hover:text-zinc-400",
              )}
              title={filterActive ? `${label} filter active` : `Filter ${label}`}
            >
              <Filter
                className={HEADER_ICON}
                fill={filterActive ? "currentColor" : "none"}
              />
            </button>
          </PopoverTrigger>
          <PopoverContent
            align="center"
            className="w-60 p-0 bg-zinc-950 border-zinc-800"
          >
            {children}
          </PopoverContent>
        </Popover>
      ) : (
        <span className="h-6 w-6 shrink-0" aria-hidden />
      )}
    </div>
  );
}

/** Multi-select checklist body (db kinds, protocols, formats, statuses, …). */
export function ChecklistFilter({
  options,
  selected,
  onChange,
}: {
  options: { value: string; label: string; color?: string }[];
  selected: Set<string>;
  onChange: (next: Set<string>) => void;
}) {
  const allSelected = selected.size === 0;
  const toggle = (value: string) => {
    const next = new Set(selected);
    if (next.has(value)) next.delete(value);
    else next.add(value);
    onChange(next);
  };
  return (
    <div>
      <button
        type="button"
        onClick={() => onChange(new Set())}
        className={cn(
          "w-full flex items-center gap-2 px-3 py-1.5 text-[11px] font-mono hover:bg-zinc-900 transition-colors border-b border-zinc-800 cursor-pointer",
          allSelected && "text-primary",
        )}
      >
        <div
          className={cn(
            "h-3.5 w-3.5 rounded-sm border border-zinc-700 flex items-center justify-center shrink-0",
            allSelected && "bg-primary border-primary",
          )}
        >
          {allSelected && (
            <Check className="h-2.5 w-2.5 text-primary-foreground" />
          )}
        </div>
        All
      </button>
      <div className="max-h-60 overflow-y-auto py-1">
        {options.map((opt) => {
          const isSelected = selected.has(opt.value);
          return (
            <button
              key={opt.value}
              type="button"
              onClick={() => toggle(opt.value)}
              className="w-full flex items-center gap-2 px-3 py-1 text-[11px] font-mono hover:bg-zinc-900 transition-colors cursor-pointer"
            >
              <div
                className={cn(
                  "h-3.5 w-3.5 rounded-sm border border-zinc-700 flex items-center justify-center shrink-0",
                  isSelected && "bg-primary border-primary",
                )}
              >
                {isSelected && (
                  <Check className="h-2.5 w-2.5 text-primary-foreground" />
                )}
              </div>
              {opt.color && (
                <span
                  className="h-2 w-2 rounded-full shrink-0"
                  style={{ backgroundColor: opt.color }}
                />
              )}
              <span className="truncate text-zinc-300" title={opt.label}>
                {opt.label}
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

/** Free-text filter body (Name search). Debounces into the URL, keeps focus. */
export function TextFilter({
  value,
  onChange,
  placeholder,
}: {
  value: string;
  onChange: (next: string) => void;
  placeholder: string;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [local, setLocal] = useState(value);
  useEffect(() => setLocal(value), [value]);
  useEffect(() => {
    if (local === value) return;
    const id = setTimeout(() => onChange(local), 300);
    return () => clearTimeout(id);
  }, [local, value, onChange]);
  useEffect(() => {
    const el = inputRef.current;
    if (!el) return;
    if (document.activeElement !== el) {
      el.focus();
      const end = el.value.length;
      el.setSelectionRange(end, end);
    }
  });
  return (
    <div className="p-2">
      <Input
        ref={inputRef}
        autoFocus
        value={local}
        onChange={(e) => setLocal(e.target.value)}
        placeholder={placeholder}
        className="h-8 w-full text-xs font-mono border-zinc-800 bg-transparent focus-visible:ring-primary/40"
      />
    </div>
  );
}

// ISO datetime <-> <input type="date"> value (YYYY-MM-DD).
function isoToDateInput(iso?: string): string {
  return iso ? iso.slice(0, 10) : "";
}
function dateInputToIso(value: string, endOfDay: boolean): string | null {
  if (!value) return null;
  return endOfDay ? `${value}T23:59:59.999Z` : `${value}T00:00:00.000Z`;
}

/** Date-range filter body (created / updated windows). */
export function DateRangeFilter({
  after,
  before,
  onChange,
}: {
  after?: string;
  before?: string;
  onChange: (after: string | null, before: string | null) => void;
}) {
  return (
    <div className="p-3 flex flex-col gap-2">
      <label className="flex flex-col gap-1">
        <span className="text-[9px] font-mono uppercase tracking-wider text-zinc-500">
          From
        </span>
        <input
          type="date"
          value={isoToDateInput(after)}
          onChange={(e) =>
            onChange(dateInputToIso(e.target.value, false), before ?? null)
          }
          className="h-8 px-2 text-xs font-mono border border-zinc-800 bg-transparent text-zinc-300 focus:outline-none focus:ring-1 focus:ring-primary/40 [color-scheme:dark]"
        />
      </label>
      <label className="flex flex-col gap-1">
        <span className="text-[9px] font-mono uppercase tracking-wider text-zinc-500">
          To
        </span>
        <input
          type="date"
          value={isoToDateInput(before)}
          onChange={(e) =>
            onChange(after ?? null, dateInputToIso(e.target.value, true))
          }
          className="h-8 px-2 text-xs font-mono border border-zinc-800 bg-transparent text-zinc-300 focus:outline-none focus:ring-1 focus:ring-primary/40 [color-scheme:dark]"
        />
      </label>
      {(after || before) && (
        <button
          type="button"
          onClick={() => onChange(null, null)}
          className="flex items-center gap-1 text-[10px] text-zinc-500 hover:text-zinc-300 font-mono underline underline-offset-2 cursor-pointer self-start"
        >
          <X className="h-3 w-3" /> clear
        </button>
      )}
    </div>
  );
}

/** Small square toggle for the inline boolean filters above the table. */
export function ToolbarTriToggle({
  value,
  onChange,
  labels,
}: {
  // undefined = all; true / false = the two states.
  value: boolean | undefined;
  onChange: (next: boolean | undefined) => void;
  // [allLabel, trueLabel, falseLabel]
  labels: [string, string, string];
}) {
  const cycle = () => {
    if (value === undefined) onChange(true);
    else if (value === true) onChange(false);
    else onChange(undefined);
  };
  const active = value !== undefined;
  const label = value === undefined ? labels[0] : value ? labels[1] : labels[2];
  return (
    <button
      type="button"
      onClick={cycle}
      className={cn(
        "flex h-5 items-center gap-1.5 px-1.5 text-[10px] font-mono border transition-colors cursor-pointer",
        active
          ? "border-primary/40 text-primary bg-primary/5"
          : "border-zinc-800 text-zinc-500 hover:text-zinc-300",
      )}
      title="Cycle: all → on → off"
      aria-pressed={active}
    >
      {label}
    </button>
  );
}

// --- shared URL <-> query state hook. ---------------------------------------
//
// Encodes the slice every Library list shares: ?q= (search), ?sort=&dir=
// (tri-state sort), ?size=&page= (page size + opaque token). Per-table facets
// (?db=, ?proto=, ?fmt=, ?is_system=, …) are read/written by each page through
// the returned getCsv / patch helpers — the hook stays facet-agnostic.

export interface LibraryTableState {
  search?: string;
  sort?: string;
  desc: boolean;
  pageSize: number;
  pageToken?: string;
}

export function useLibraryTableState(sortableFields: readonly string[]) {
  const [searchParams, setSearchParams] = useSearchParams();

  const tokenStackRef = useRef<string[]>([]);

  const state = useMemo<LibraryTableState>(() => {
    const sortRaw = searchParams.get("sort");
    const sort =
      sortRaw && sortableFields.includes(sortRaw) ? sortRaw : undefined;
    const desc = searchParams.get("dir")
      ? searchParams.get("dir") === "desc"
      : true;
    const size = Number.parseInt(searchParams.get("size") ?? "", 10);
    const pageSize = PAGE_SIZES.includes(size) ? size : DEFAULT_PAGE_SIZE;
    return {
      search: searchParams.get("q") ?? undefined,
      sort,
      desc,
      pageSize,
      pageToken: searchParams.get("page") ?? undefined,
    };
  }, [searchParams, sortableFields]);

  const patch = useCallback(
    (next: Record<string, string | null>, resetPage = true) => {
      const params = new URLSearchParams(searchParams);
      for (const [k, v] of Object.entries(next)) {
        if (v === null || v === "") params.delete(k);
        else params.set(k, v);
      }
      if (resetPage) {
        params.delete("page");
        tokenStackRef.current = [];
      }
      setSearchParams(params);
    },
    [searchParams, setSearchParams],
  );

  // CSV facet read/write helpers, validated against an allowed value set.
  const getCsv = useCallback(
    (key: string, allowed?: readonly string[]): string[] => {
      const raw = (searchParams.get(key) ?? "")
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean);
      return allowed ? raw.filter((v) => allowed.includes(v)) : raw;
    },
    [searchParams],
  );
  const setCsv = useCallback(
    (key: string, selected: Set<string>) => {
      patch({ [key]: [...selected].join(",") || null });
    },
    [patch],
  );

  // Tri-state optional-bool param (?key=true|false|absent).
  const getTriBool = useCallback(
    (key: string): boolean | undefined => {
      const v = searchParams.get(key);
      if (v === "true") return true;
      if (v === "false") return false;
      return undefined;
    },
    [searchParams],
  );

  // Tri-state sort cycle: neutral → asc → desc → neutral.
  const toggleSort = useCallback(
    (field: string) => {
      const active = state.sort === field;
      if (!active) patch({ sort: field, dir: "asc" });
      else if (!state.desc) patch({ sort: field, dir: "desc" });
      else patch({ sort: null, dir: null });
    },
    [patch, state.sort, state.desc],
  );

  const setPageSize = useCallback(
    (size: number) => {
      patch({ size: size === DEFAULT_PAGE_SIZE ? null : String(size) });
    },
    [patch],
  );

  const pageIndex = tokenStackRef.current.length;

  const goNext = useCallback(
    (nextPageToken: string) => {
      if (!nextPageToken) return;
      tokenStackRef.current = [
        ...tokenStackRef.current,
        state.pageToken ?? "",
      ];
      patch({ page: nextPageToken }, false);
    },
    [patch, state.pageToken],
  );
  const goPrev = useCallback(() => {
    const stack = tokenStackRef.current;
    if (stack.length === 0) return;
    const prev = stack[stack.length - 1];
    tokenStackRef.current = stack.slice(0, -1);
    patch({ page: prev || null }, false);
  }, [patch]);

  return {
    state,
    patch,
    getCsv,
    setCsv,
    getTriBool,
    toggleSort,
    setPageSize,
    pageIndex,
    goNext,
    goPrev,
  };
}

// --- the table shell. --------------------------------------------------------

export interface LibraryTableProps<T> {
  columns: ColumnDef<T>[];
  rows: T[];
  getRowId: (row: T) => string;
  columnWidths?: Record<string, string>;
  loading: boolean;
  error: string | null;
  emptyLabel: string;
  emptyFilteredLabel: string;
  hasActiveFilters: boolean;
  onRowClick?: (row: T) => void;
  // Filter popover open-state, lifted here so refetch can't reset it.
  openColumnId: string | null;
  // Paging.
  pageIndex: number;
  canPrev: boolean;
  canNext: boolean;
  onPrev: () => void;
  onNext: () => void;
  pageSize: number;
  onPageSize: (size: number) => void;
}

export function LibraryTable<T>({
  columns,
  rows,
  getRowId,
  columnWidths,
  loading,
  error,
  emptyLabel,
  emptyFilteredLabel,
  hasActiveFilters,
  onRowClick,
  pageIndex,
  canPrev,
  canNext,
  onPrev,
  onNext,
  pageSize,
  onPageSize,
}: LibraryTableProps<T>) {
  const table = useReactTable({
    data: rows,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getRowId,
    manualSorting: true,
    manualFiltering: true,
    manualPagination: true,
    enableSortingRemoval: true,
  });

  return (
    <div className="flex flex-col gap-4 h-full min-h-0">
      {error && (
        <div className="flex items-center gap-2 text-xs p-2.5 border border-destructive/30 text-destructive font-mono">
          <AlertCircle className="h-3.5 w-3.5 shrink-0" />
          {error}
        </div>
      )}

      {/* Internal-scroll table region with a sticky header, identical chrome to
          the Test Runs table: border-separate + border-spacing-0 so the sticky
          <th> keeps its divider, an opaque header bg so rows never show through,
          and a proportional colgroup with table-auto. */}
      <div className="border border-zinc-800/80 bg-[#080808] flex-1 min-h-0 overflow-auto">
        <table className="w-full table-auto caption-bottom text-sm border-separate border-spacing-0">
          {columnWidths && (
            <colgroup>
              {table.getVisibleLeafColumns().map((column) => (
                <col key={column.id} style={{ width: columnWidths[column.id] }} />
              ))}
            </colgroup>
          )}
          <thead>
            {table.getHeaderGroups().map((hg) => (
              <tr key={hg.id}>
                {hg.headers.map((header) => (
                  <th
                    key={header.id}
                    colSpan={header.colSpan}
                    className={cn(
                      "text-left align-middle font-medium px-3",
                      "font-mono uppercase tracking-wider text-zinc-500",
                      "sticky top-0 z-20 bg-zinc-900",
                      "border-b border-zinc-800 h-9 text-[11px]",
                      header.column.columnDef.meta?.className,
                    )}
                  >
                    {header.isPlaceholder
                      ? null
                      : flexRender(
                          header.column.columnDef.header,
                          header.getContext(),
                        )}
                  </th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody>
            {table.getRowModel().rows.length ? (
              table.getRowModel().rows.map((row) => (
                <tr
                  key={row.id}
                  className={cn(
                    "group/lib-row transition-colors [&>td]:border-b [&>td]:border-zinc-800/50",
                    onRowClick && "cursor-pointer",
                  )}
                  onClick={
                    onRowClick ? () => onRowClick(row.original) : undefined
                  }
                >
                  {row.getVisibleCells().map((cell) => {
                    const meta = cell.column.columnDef.meta;
                    return (
                      <td
                        key={cell.id}
                        onClick={
                          meta?.disableRowNavigation
                            ? (e) => e.stopPropagation()
                            : undefined
                        }
                        className={cn(
                          "px-3 py-2.5 align-middle text-left bg-zinc-950/40",
                          "transition-colors group-hover/lib-row:bg-zinc-800/60 group-hover/lib-row:border-zinc-700/80",
                          meta?.disableRowNavigation && "cursor-default",
                          meta?.className,
                        )}
                      >
                        {flexRender(
                          cell.column.columnDef.cell,
                          cell.getContext(),
                        )}
                      </td>
                    );
                  })}
                </tr>
              ))
            ) : (
              <tr>
                <td
                  colSpan={table.getVisibleLeafColumns().length}
                  className="h-32 text-center px-3 align-middle"
                >
                  <span className="inline-flex items-center gap-2 text-xs text-zinc-600 font-mono">
                    {loading ? (
                      <>
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        Loading…
                      </>
                    ) : hasActiveFilters ? (
                      emptyFilteredLabel
                    ) : (
                      emptyLabel
                    )}
                  </span>
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination — page-size selector + opaque-token prev/next, like Runs. */}
      <div className="flex items-center justify-between mt-auto">
        <div className="flex items-center gap-2">
          <span className="text-[11px] text-zinc-600 font-mono">Rows</span>
          <div className="flex gap-0.5">
            {PAGE_SIZES.map((size) => (
              <button
                key={size}
                type="button"
                onClick={() => onPageSize(size)}
                className={`px-2 py-0.5 text-[11px] font-mono border transition-colors cursor-pointer ${
                  pageSize === size
                    ? "border-zinc-600 text-zinc-300 bg-zinc-800"
                    : "border-transparent text-zinc-600 hover:text-zinc-400"
                }`}
              >
                {size}
              </button>
            ))}
          </div>
        </div>

        <div className="flex items-center gap-1">
          <span className="text-[11px] text-zinc-600 font-mono tabular-nums px-2">
            Page {pageIndex + 1}
          </span>
          <Button
            variant="ghost"
            size="sm"
            className="h-7 w-7 p-0"
            onClick={onPrev}
            disabled={!canPrev || loading}
          >
            <ChevronLeft className="h-3.5 w-3.5" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="h-7 w-7 p-0"
            onClick={onNext}
            disabled={!canNext || loading}
          >
            <ChevronRight className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>
    </div>
  );
}
