import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  useReactTable,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  flexRender,
  type ColumnDef,
  type ColumnFiltersState,
  type SortingState,
} from "@tanstack/react-table";
import {
  deleteSuite,
  launchSuite,
  listSuites,
  updateSuite,
} from "@/api/client";
import type { Suite } from "@/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { useConfirm } from "@/components/ui/confirm-dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  AlertCircle,
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  CalendarClock,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  Clock,
  Layers,
  Pause,
  Pencil,
  Play,
  Plus,
  RefreshCw,
  Trash2,
  Zap,
} from "lucide-react";

// Suites: top-level list of cron-schedulable benchmark bundles. Style
// mirrors the Runs table — same chrome, mono typography, dark zinc, sharp
// borders — so the section reads as a sibling, not a separate product.

type SuiteStatus = "running" | "scheduled" | "paused" | "manual";

function deriveStatus(s: Suite): SuiteStatus {
  // "running" wins when a recent batch is still alive — the matrix is
  // actively producing runs and the user wants that surfaced first.
  if (s.cron_expr && s.enabled) return "scheduled";
  if (s.cron_expr && !s.enabled) return "paused";
  return "manual";
}

const STATUS_CONFIG: Record<
  SuiteStatus,
  { label: string; variant: "success" | "destructive" | "warning" | "pending" | "secondary" | "default" }
> = {
  running: { label: "Running", variant: "warning" },
  scheduled: { label: "Scheduled", variant: "success" },
  paused: { label: "Paused", variant: "secondary" },
  manual: { label: "Manual", variant: "default" },
};

function formatRel(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  if (d.getFullYear() < 2000) return "—";
  const sameDay = d.toDateString() === new Date().toDateString();
  if (sameDay) {
    return d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
  }
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

// FilterChip mirrors the chip used in Runs.tsx — small mono pill with a
// subtle primary glow when active.
function FilterChip({ label, active, onClick }: { label: string; active: boolean; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className={`px-2.5 py-1 text-[11px] font-mono border transition-colors cursor-pointer ${
        active
          ? "border-primary/60 text-primary bg-primary/8"
          : "border-zinc-800 text-zinc-500 hover:text-zinc-300 hover:border-zinc-700"
      }`}
    >
      {label}
    </button>
  );
}

function SortableHeader({
  column,
  label,
}: {
  column: { getIsSorted: () => false | "asc" | "desc"; toggleSorting: () => void };
  label: string;
}) {
  const sorted = column.getIsSorted();
  return (
    <button
      className="flex items-center gap-1 hover:text-foreground transition-colors cursor-pointer"
      onClick={() => column.toggleSorting()}
    >
      {label}
      {sorted === "asc" ? (
        <ArrowUp className="h-3 w-3" />
      ) : sorted === "desc" ? (
        <ArrowDown className="h-3 w-3" />
      ) : (
        <ArrowUpDown className="h-3 w-3 opacity-40" />
      )}
    </button>
  );
}

const PAGE_SIZES = [10, 25, 50, 100];
const REFRESH_OPTIONS = [
  { label: "Off", value: 0 },
  { label: "5s", value: 5 },
  { label: "15s", value: 15 },
  { label: "30s", value: 30 },
];

function makeColumns(
  onLaunch: (s: Suite) => void,
  onToggle: (s: Suite, enabled: boolean) => void,
  onEdit: (s: Suite) => void,
  onDelete: (s: Suite) => void,
  busyID: string | null,
): ColumnDef<Suite>[] {
  return [
    // ID — last 8 chars, font-mono primary; matches Runs ID column.
    {
      accessorKey: "id",
      header: ({ column }) => <SortableHeader column={column} label="ID" />,
      cell: ({ row }) => {
        const s = row.original;
        const short = s.id.length > 8 ? s.id.slice(-8) : s.id;
        return (
          <span className="font-mono text-xs text-primary" title={s.id}>
            {short}
          </span>
        );
      },
      size: 96,
    },
    // Name + description preview.
    {
      accessorKey: "name",
      header: ({ column }) => <SortableHeader column={column} label="Name" />,
      cell: ({ row }) => {
        const s = row.original;
        const titleAttr = s.description ? `${s.name}\n\n${s.description}` : s.name;
        return (
          <div className="min-w-0 max-w-[22rem]">
            <span className="text-xs text-zinc-200 truncate block" title={titleAttr}>
              {s.name}
            </span>
            {s.description && (
              <span className="text-[10px] text-zinc-600 font-mono truncate block">
                {s.description}
              </span>
            )}
          </div>
        );
      },
    },
    // Status — composite of cron + enabled + last batch.
    {
      id: "status",
      accessorFn: (s) => deriveStatus(s),
      header: ({ column }) => <SortableHeader column={column} label="Status" />,
      cell: ({ row }) => {
        const status = deriveStatus(row.original);
        const cfg = STATUS_CONFIG[status];
        return <Badge variant={cfg.variant}>{cfg.label}</Badge>;
      },
      filterFn: (row, _id, value) => {
        if (!value || value === "all") return true;
        return deriveStatus(row.original) === value;
      },
    },
    // DB targets count.
    {
      id: "targets",
      header: "Targets",
      cell: ({ row }) => {
        const n = row.original.db_preset_ids?.length ?? 0;
        return (
          <span className="font-mono text-xs text-zinc-400 inline-flex items-center gap-1">
            <Layers className="w-3 h-3 text-zinc-600" />
            {n} db
          </span>
        );
      },
      enableSorting: false,
      size: 110,
    },
    // Workloads count.
    {
      id: "workloads",
      header: "Workloads",
      cell: ({ row }) => {
        const n = row.original.items?.length ?? 0;
        return (
          <span className="font-mono text-xs text-zinc-400 inline-flex items-center gap-1">
            <Zap className="w-3 h-3 text-zinc-600" />
            {n}
          </span>
        );
      },
      enableSorting: false,
      size: 110,
    },
    // Schedule (cron expression + tz).
    {
      id: "schedule",
      header: "Schedule",
      cell: ({ row }) => {
        const s = row.original;
        if (!s.cron_expr) {
          return <span className="font-mono text-xs text-zinc-600">{"—"}</span>;
        }
        return (
          <span className="font-mono text-xs text-zinc-400 inline-flex items-center gap-1">
            <Clock className="w-3 h-3 text-zinc-600" />
            <span className="text-zinc-300">{s.cron_expr}</span>
            <span className="text-zinc-700">{s.timezone}</span>
          </span>
        );
      },
      enableSorting: false,
    },
    // Last batch — short id + relative time.
    {
      id: "last_batch",
      accessorFn: (s) => s.last_fire_at || "",
      header: ({ column }) => <SortableHeader column={column} label="Last batch" />,
      cell: ({ row }) => {
        const s = row.original;
        if (!s.last_fire_at) return <span className="font-mono text-xs text-zinc-600">{"—"}</span>;
        return (
          <div className="flex flex-col">
            <span className="text-xs text-zinc-400 font-mono">{formatRel(s.last_fire_at)}</span>
            {s.last_batch_id && (
              <span className="text-[10px] text-zinc-700 font-mono tabular-nums">
                {s.last_batch_id.slice(0, 8)}
              </span>
            )}
          </div>
        );
      },
      sortingFn: (a, b) => {
        const av = a.original.last_fire_at ? new Date(a.original.last_fire_at).getTime() : 0;
        const bv = b.original.last_fire_at ? new Date(b.original.last_fire_at).getTime() : 0;
        return av - bv;
      },
    },
    // Next fire — only for scheduled+enabled rows.
    {
      id: "next_fire",
      accessorFn: (s) => s.next_fire_at || "",
      header: ({ column }) => <SortableHeader column={column} label="Next fire" />,
      cell: ({ row }) => {
        const s = row.original;
        if (!s.enabled || !s.next_fire_at) {
          return <span className="font-mono text-xs text-zinc-600">{"—"}</span>;
        }
        return (
          <span className="font-mono text-xs text-zinc-400 inline-flex items-center gap-1">
            <CalendarClock className="w-3 h-3 text-zinc-600" />
            {formatRel(s.next_fire_at)}
          </span>
        );
      },
      sortingFn: (a, b) => {
        const av = a.original.next_fire_at ? new Date(a.original.next_fire_at).getTime() : Infinity;
        const bv = b.original.next_fire_at ? new Date(b.original.next_fire_at).getTime() : Infinity;
        return av - bv;
      },
    },
    // Enabled — Switch column. Click stops row navigation.
    {
      id: "enabled",
      header: "On",
      cell: ({ row }) => {
        const s = row.original;
        return (
          <span data-row-stop className="inline-flex items-center" onClick={(e) => e.stopPropagation()}>
            <Switch
              checked={s.enabled}
              disabled={busyID === s.id}
              onCheckedChange={(v: boolean) => onToggle(s, v)}
            />
          </span>
        );
      },
      enableSorting: false,
      size: 56,
    },
    // Actions
    {
      id: "actions",
      header: "",
      cell: ({ row }) => {
        const s = row.original;
        const cantLaunch = busyID === s.id || (s.items?.length ?? 0) === 0 || (s.db_preset_ids?.length ?? 0) === 0;
        return (
          <div className="flex items-center gap-0.5">
            <Button
              size="sm"
              variant="ghost"
              className="h-7 w-7 p-0 text-emerald-600 hover:text-emerald-400 cursor-pointer disabled:text-zinc-700"
              title={cantLaunch ? "Add at least one DB target and one workload first" : "Launch matrix now"}
              disabled={cantLaunch}
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                onLaunch(s);
              }}
            >
              <Play className="h-3.5 w-3.5" />
            </Button>
            <Button
              size="sm"
              variant="ghost"
              className="h-7 w-7 p-0 text-zinc-500 hover:text-primary cursor-pointer"
              title="Edit suite"
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                onEdit(s);
              }}
            >
              <Pencil className="h-3.5 w-3.5" />
            </Button>
            <Button
              size="sm"
              variant="ghost"
              className="h-7 w-7 p-0 text-zinc-600 hover:text-destructive cursor-pointer"
              title="Delete suite"
              disabled={busyID === s.id}
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                onDelete(s);
              }}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          </div>
        );
      },
      enableSorting: false,
      size: 110,
    },
  ];
}

export function Suites() {
  const navigate = useNavigate();
  const confirm = useConfirm();

  const [suites, setSuites] = useState<Suite[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busyID, setBusyID] = useState<string | null>(null);

  // Auto-refresh, mirrors Runs.
  const [refreshInterval, setRefreshInterval] = useState(15);
  const refreshRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const [sorting, setSorting] = useState<SortingState>([{ id: "name", desc: false }]);
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([]);

  async function fetchSuites() {
    setLoading(true);
    setError(null);
    try {
      const result = await listSuites();
      setSuites(result ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load suites");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { fetchSuites(); }, []);

  useEffect(() => {
    if (refreshRef.current) clearInterval(refreshRef.current);
    if (refreshInterval > 0) {
      refreshRef.current = setInterval(fetchSuites, refreshInterval * 1000);
    }
    return () => {
      if (refreshRef.current) clearInterval(refreshRef.current);
    };
  }, [refreshInterval]);

  async function handleToggle(s: Suite, enabled: boolean) {
    setBusyID(s.id);
    try {
      await updateSuite(s.id, { enabled });
      await fetchSuites();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to toggle suite");
    } finally {
      setBusyID(null);
    }
  }

  async function handleLaunch(s: Suite) {
    setBusyID(s.id);
    try {
      const r = await launchSuite(s.id);
      // Inline transient feedback — full toast system isn't installed yet.
      setError(null);
      await fetchSuites();
      navigate(`/runs?suite=${s.id}&batch=${r.batch_id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Launch failed");
    } finally {
      setBusyID(null);
    }
  }

  async function handleEdit(s: Suite) {
    navigate(`/suites/${s.id}/edit`);
  }

  async function handleDelete(s: Suite) {
    const ok = await confirm({
      title: `Delete suite "${s.name}"?`,
      description: "This removes the suite and its scheduled items. Past batches stay visible under runs.",
      danger: true,
    });
    if (!ok) return;
    setBusyID(s.id);
    try {
      await deleteSuite(s.id);
      await fetchSuites();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete suite");
    } finally {
      setBusyID(null);
    }
  }

  // eslint-disable-next-line react-hooks/exhaustive-deps
  const columns = useMemo(
    () => makeColumns(handleLaunch, handleToggle, handleEdit, handleDelete, busyID),
    [busyID],
  );

  const table = useReactTable({
    data: suites,
    columns,
    state: { sorting, columnFilters },
    onSortingChange: setSorting,
    onColumnFiltersChange: setColumnFilters,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getRowId: (row) => row.id,
    initialState: { pagination: { pageSize: 25 } },
  });

  const filterValues = useMemo(() => {
    const statuses = new Set<string>();
    for (const s of suites) statuses.add(deriveStatus(s));
    return { statuses: Array.from(statuses).sort() };
  }, [suites]);

  const activeStatus = columnFilters.find((f) => f.id === "status")?.value as string | undefined;
  function setStatusFilter(value: string | undefined) {
    setColumnFilters((prev) => {
      const without = prev.filter((f) => f.id !== "status");
      if (!value || value === "all") return without;
      return [...without, { id: "status", value }];
    });
  }

  return (
    <div className="p-5 flex flex-col gap-4 min-h-full">
      {/* Header bar */}
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <h1 className="text-base font-semibold font-mono tracking-tight">Suites</h1>
          <span className="text-[11px] text-zinc-600 font-mono tabular-nums">
            {table.getFilteredRowModel().rows.length} of {suites.length}
          </span>
        </div>

        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => navigate("/suites/new")}
            className="flex items-center gap-1.5 px-2.5 py-1 text-[11px] font-mono border border-zinc-800 text-zinc-400 hover:text-zinc-200 hover:border-zinc-700 transition-colors cursor-pointer"
          >
            <Plus className="h-3 w-3" />
            New Suite
          </button>

          <div className="flex items-center gap-0.5 border border-zinc-800 px-1 py-0.5">
            <button
              type="button"
              onClick={fetchSuites}
              disabled={loading}
              className="p-1 text-zinc-500 hover:text-zinc-300 transition-colors cursor-pointer disabled:opacity-50"
              title="Refresh now"
            >
              <RefreshCw className={`h-3.5 w-3.5 ${loading || refreshInterval > 0 ? "animate-spin" : ""}`} />
            </button>
            {REFRESH_OPTIONS.map((opt) => (
              <button
                type="button"
                key={opt.value}
                onClick={() => setRefreshInterval(opt.value)}
                className={`px-1.5 py-0.5 text-[10px] font-mono transition-colors cursor-pointer ${
                  refreshInterval === opt.value
                    ? "text-primary bg-primary/10"
                    : "text-zinc-600 hover:text-zinc-400"
                }`}
              >
                {opt.label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {error && (
        <div className="flex items-center gap-2 text-xs p-2.5 border border-destructive/30 text-destructive font-mono">
          <AlertCircle className="h-3.5 w-3.5 shrink-0" />
          {error}
        </div>
      )}

      {/* Filters — single dimension (status). Add more here when needed. */}
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        {filterValues.statuses.length > 1 && (
          <div className="flex items-center gap-1.5">
            <span className="text-[10px] text-zinc-600 font-mono uppercase tracking-wider">Status</span>
            <FilterChip label="all" active={!activeStatus} onClick={() => setStatusFilter(undefined)} />
            {filterValues.statuses.map((s) => (
              <FilterChip
                key={s}
                label={s}
                active={activeStatus === s}
                onClick={() => setStatusFilter(activeStatus === s ? undefined : s)}
              />
            ))}
          </div>
        )}
        {activeStatus && (
          <button
            onClick={() => setColumnFilters([])}
            className="text-[10px] text-zinc-500 hover:text-zinc-300 font-mono underline underline-offset-2 cursor-pointer"
          >
            clear filters
          </button>
        )}
      </div>

      {/* Table */}
      <div className="border border-zinc-800/80 bg-[#080808] flex-1 min-h-0 overflow-auto">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id} className="border-zinc-800/80 hover:bg-transparent">
                {headerGroup.headers.map((header) => (
                  <TableHead
                    key={header.id}
                    className="text-[11px] font-mono uppercase tracking-wider text-zinc-500 h-9 bg-zinc-900/50"
                    style={header.column.getSize() !== 150 ? { width: header.column.getSize() } : undefined}
                  >
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  className="border-zinc-800/50 hover:bg-zinc-900/60 cursor-pointer transition-colors"
                  onClick={(e) => {
                    const target = e.target as HTMLElement;
                    if (target.closest("button, a, input, [data-row-stop]")) return;
                    navigate(`/suites/${row.original.id}`);
                  }}
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id} className="py-2.5">
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={columns.length} className="h-32 text-center">
                  <span className="text-xs text-zinc-600 font-mono">
                    {loading
                      ? "Loading suites..."
                      : suites.length === 0
                        ? "No suites yet — create one to bundle (db_preset × workload) runs."
                        : "No suites matching filters"}
                  </span>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      {/* Pagination — duplicates the Runs footer pattern. */}
      {suites.length > 0 && (
        <div className="flex items-center justify-between mt-auto">
          <div className="flex items-center gap-2">
            <span className="text-[11px] text-zinc-600 font-mono">Rows</span>
            <div className="flex gap-0.5">
              {PAGE_SIZES.map((size) => (
                <button
                  key={size}
                  onClick={() => table.setPageSize(size)}
                  className={`px-2 py-0.5 text-[11px] font-mono border transition-colors cursor-pointer ${
                    table.getState().pagination.pageSize === size
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
            <span className="text-[11px] text-zinc-600 font-mono tabular-nums mr-2">
              {table.getState().pagination.pageIndex * table.getState().pagination.pageSize + 1}
              {"–"}
              {Math.min(
                (table.getState().pagination.pageIndex + 1) * table.getState().pagination.pageSize,
                table.getFilteredRowModel().rows.length,
              )}
              {" of "}
              {table.getFilteredRowModel().rows.length}
            </span>
            <Button variant="ghost" size="sm" className="h-7 w-7 p-0" onClick={() => table.setPageIndex(0)} disabled={!table.getCanPreviousPage()}>
              <ChevronsLeft className="h-3.5 w-3.5" />
            </Button>
            <Button variant="ghost" size="sm" className="h-7 w-7 p-0" onClick={() => table.previousPage()} disabled={!table.getCanPreviousPage()}>
              <ChevronLeft className="h-3.5 w-3.5" />
            </Button>
            <span className="text-[11px] text-zinc-500 font-mono tabular-nums px-2">
              {table.getState().pagination.pageIndex + 1}/{table.getPageCount()}
            </span>
            <Button variant="ghost" size="sm" className="h-7 w-7 p-0" onClick={() => table.nextPage()} disabled={!table.getCanNextPage()}>
              <ChevronRight className="h-3.5 w-3.5" />
            </Button>
            <Button variant="ghost" size="sm" className="h-7 w-7 p-0" onClick={() => table.setPageIndex(table.getPageCount() - 1)} disabled={!table.getCanNextPage()}>
              <ChevronsRight className="h-3.5 w-3.5" />
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

// Pause icon imported but unused after status refactor — re-export to keep
// tree-shaking happy if a future "Pause" action lands here.
export { Pause };

export default Suites;
