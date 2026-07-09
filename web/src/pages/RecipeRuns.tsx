// RecipeRuns — the list page for RecipeService.ListRuns: runs launched from
// (or alongside) recipe bundles for the current tenant. This is the "Test
// Runs" table's DSL-era rebuild: a client-side sortable/filterable table
// (tanstack/react-table, same as ref's Runs.tsx) over services/runs.ts's
// RunVM — the FULL flattened cloud.v1.models.Run projection (db kind,
// provider, node count, progress, duration, ...), not the old four-field
// id/name/status/startedAt-only projection. See
// .superpowers/sdd/runs-parity-report.md for the field-by-field parity audit
// against ref's Runs.tsx.
//
// Row -> RunVM (services/runs.ts, re-exported by services/recipe.ts).
// Name links to RunDetail (/runs/:id) which renders the FULL overview via
// testRunOverviewClient — unrelated to this list's data source. Actions:
// Cancel/Rerun/Delete gated by services/runs.ts's actionsForStatus (the same
// gating RunDetail's action bar uses), each behind useConfirm.
//
// listRuns(slug) with no recipeId returns EVERY run for the tenant (recipe.ts
// RecipeService.ListRuns falls back to the tenant's full run list when
// recipe_id is unset — see recipe.ts's doc comment); the whole (paginated,
// looped) list is fetched once and sorted/filtered/paged client-side here,
// exactly like ref's Runs.tsx did — ListRunsRequest has no sort/filter
// params of its own to wire a server-driven table to.
//
// DROPPED vs ref (see parity report for the full table + reasoning):
//   * "Preset" column (preset_id -> catalog preset name) — presets were
//     replaced by recipe bundles; the equivalent here is the Recipe column
//     (workflowId -> recipe name).
//   * Workload "script"/"VUs"/configured-duration sub-labels — no DSL-era
//     equivalent facet is denormalized onto Run.Summary (workload shape now
//     lives in the recipe bundle's compiled plan, not a per-run knob).

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  useReactTable,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  flexRender,
  type ColumnDef,
  type SortingState,
  type ColumnFiltersState,
  type RowSelectionState,
} from "@tanstack/react-table";
import {
  AlertCircle,
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  GitCompare,
  Loader2,
  RefreshCw,
  Repeat,
  Square,
  Trash2,
  X,
} from "lucide-react";
import { Link, useNavigate, useTenantSlug } from "@/lib/router";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useConfirm } from "@/components/ui/confirm-dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { cancelRun, deleteRun, listRecipes, listRuns, type RecipeVM } from "@/services/recipe";
import { actionsForStatus, type RunVM } from "@/services/runs";
import type { RunStatus } from "@/services/dashboard";
import { DB_COLOR, DB_LABEL } from "@/components/library-table/labels";

const STATUS_VARIANT: Record<RunStatus, "default" | "secondary" | "success" | "destructive" | "warning" | "pending"> = {
  pending: "pending",
  running: "warning",
  cancelling: "warning",
  completed: "success",
  failed: "destructive",
  cancelled: "secondary",
};

function fmtStarted(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(d);
}

function fmtDuration(sec?: number): string {
  if (sec === undefined || !Number.isFinite(sec)) return "—";
  const s = Math.max(0, Math.round(sec));
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m ${s % 60}s`;
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  return `${h}h ${m}m`;
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
      type="button"
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

function FilterChip({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
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

const PAGE_SIZES = [10, 25, 50, 100];

function makeColumns(opts: {
  onDelete: (run: RunVM) => void;
  onCancel: (run: RunVM) => void;
  onRerun: (run: RunVM) => void;
  busyId: string | null;
  recipeNames: Map<string, string>;
}): ColumnDef<RunVM>[] {
  const { onDelete, onCancel, onRerun, busyId, recipeNames } = opts;
  return [
    {
      id: "select",
      header: ({ table }) => (
        <input
          type="checkbox"
          className="accent-primary w-3.5 h-3.5 cursor-pointer"
          checked={table.getIsAllPageRowsSelected()}
          onChange={table.getToggleAllPageRowsSelectedHandler()}
        />
      ),
      cell: ({ row }) => (
        <input
          type="checkbox"
          className="accent-primary w-3.5 h-3.5 cursor-pointer"
          checked={row.getIsSelected()}
          onChange={row.getToggleSelectedHandler()}
        />
      ),
      enableSorting: false,
      size: 36,
    },
    {
      accessorKey: "id",
      header: ({ column }) => <SortableHeader column={column} label="ID" />,
      cell: ({ row }) => {
        const r = row.original;
        const short = r.id.length > 8 ? r.id.slice(-8) : r.id;
        return (
          <span className="font-mono text-xs text-primary" title={r.id}>
            {short}
          </span>
        );
      },
    },
    {
      accessorKey: "name",
      header: ({ column }) => <SortableHeader column={column} label="Name" />,
      cell: ({ row }) => {
        const r = row.original;
        if (!r.name) return <span className="text-xs text-zinc-700">—</span>;
        return (
          <span className="text-xs text-zinc-200 truncate block max-w-[16rem]" title={r.name}>
            {r.name}
          </span>
        );
      },
    },
    {
      accessorKey: "status",
      header: ({ column }) => <SortableHeader column={column} label="Status" />,
      cell: ({ row }) => {
        const status = row.original.status;
        return <Badge variant={STATUS_VARIANT[status]}>{status}</Badge>;
      },
      filterFn: (row, _id, value) => {
        if (!value || value === "all") return true;
        return row.original.status === value;
      },
    },
    {
      accessorKey: "dbKind",
      header: ({ column }) => <SortableHeader column={column} label="Database" />,
      cell: ({ row }) => {
        const r = row.original;
        if (!r.dbKind) return <span className="font-mono text-xs text-zinc-600">—</span>;
        const color = DB_COLOR[r.dbKind];
        const nodes = r.nodeCount ? `${r.nodeCount} node${r.nodeCount > 1 ? "s" : ""}` : null;
        return (
          <span className="font-mono text-xs" style={{ color }}>
            {DB_LABEL[r.dbKind]}
            {r.dbVersion && <span className="text-zinc-500"> {r.dbVersion}</span>}
            {nodes && <span className="text-zinc-600"> · {nodes}</span>}
          </span>
        );
      },
      filterFn: (row, _id, value) => {
        if (!value || value === "all") return true;
        return row.original.dbKind === value;
      },
    },
    {
      // Recipe replaces the old preset_id->name column: recipes are the
      // DSL-era successor to presets (see file doc comment).
      accessorKey: "workflowId",
      header: "Recipe",
      cell: ({ row }) => {
        const id = row.original.workflowId;
        if (!id) return <span className="font-mono text-xs text-zinc-600">—</span>;
        const name = recipeNames.get(id);
        return (
          <Link
            to={`/recipes/${id}`}
            onClick={(e) => e.stopPropagation()}
            className="font-mono text-xs text-zinc-400 hover:text-primary hover:underline underline-offset-2"
            title={id}
          >
            {name ?? id.slice(0, 8)}
          </Link>
        );
      },
      enableSorting: false,
    },
    {
      id: "workload",
      header: "Workload",
      cell: ({ row }) => {
        const r = row.original;
        if (!r.workload) return <span className="font-mono text-xs text-zinc-600">—</span>;
        return <span className="font-mono text-xs text-zinc-400">{r.workload}</span>;
      },
      enableSorting: false,
    },
    {
      accessorKey: "provider",
      header: ({ column }) => <SortableHeader column={column} label="Provider" />,
      cell: ({ row }) => (
        <span className="font-mono text-xs text-zinc-400">{row.original.provider || "—"}</span>
      ),
      filterFn: (row, _id, value) => {
        if (!value || value === "all") return true;
        return row.original.provider === value;
      },
    },
    {
      id: "progress",
      header: "Progress",
      cell: ({ row }) => {
        const r = row.original;
        const pct = Math.max(0, Math.min(100, r.progressPct));
        const barColor =
          r.status === "cancelled"
            ? "bg-zinc-500"
            : r.status === "failed"
              ? "bg-destructive"
              : "bg-emerald-500";
        return (
          <div className="flex items-center gap-2 min-w-[100px]">
            <div className="flex-1 bg-zinc-900 h-1.5 overflow-hidden">
              <div className={`h-full transition-all duration-500 ${barColor}`} style={{ width: `${pct}%` }} />
            </div>
            <span className="text-[10px] font-mono text-zinc-500 tabular-nums w-9 text-right">{pct}%</span>
          </div>
        );
      },
      enableSorting: false,
    },
    {
      id: "duration",
      header: "Duration",
      cell: ({ row }) => (
        <span className="text-xs text-zinc-500 font-mono tabular-nums">
          {fmtDuration(row.original.durationSec)}
        </span>
      ),
      enableSorting: false,
    },
    {
      accessorKey: "startedAt",
      header: ({ column }) => <SortableHeader column={column} label="Started" />,
      cell: ({ row }) => (
        <span className="font-mono text-xs text-zinc-500" title={row.original.startedAt}>
          {fmtStarted(row.original.startedAt)}
        </span>
      ),
    },
    {
      id: "actions",
      header: "",
      cell: ({ row }) => {
        const r = row.original;
        const allowed = actionsForStatus(r.status);
        const isBusy = busyId === r.id;
        return (
          <div className="flex items-center gap-0.5">
            {allowed.has("cancel") && (
              <Button
                size="sm"
                variant="ghost"
                className="h-7 w-7 p-0 text-amber-600 hover:text-amber-400 cursor-pointer"
                title="Cancel run"
                disabled={isBusy}
                onClick={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  onCancel(r);
                }}
              >
                {isBusy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Square className="h-3.5 w-3.5" />}
              </Button>
            )}
            {allowed.has("rerun") && (
              <Button
                size="sm"
                variant="ghost"
                className="h-7 w-7 p-0 text-zinc-500 hover:text-primary cursor-pointer"
                title="Rerun this recipe"
                disabled={isBusy}
                onClick={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  onRerun(r);
                }}
              >
                <Repeat className="h-3.5 w-3.5" />
              </Button>
            )}
            {allowed.has("delete") && (
              <Button
                size="sm"
                variant="ghost"
                className="h-7 w-7 p-0 text-zinc-600 hover:text-destructive cursor-pointer"
                title="Delete run"
                disabled={isBusy}
                onClick={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  onDelete(r);
                }}
              >
                {isBusy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Trash2 className="h-3.5 w-3.5" />}
              </Button>
            )}
          </div>
        );
      },
      enableSorting: false,
      size: 110,
    },
  ];
}

export function RecipeRuns() {
  const slug = useTenantSlug() ?? "";
  const navigate = useNavigate();
  const confirm = useConfirm();

  const [runs, setRuns] = useState<RunVM[]>([]);
  const [recipeNames, setRecipeNames] = useState<Map<string, string>>(new Map());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

  const [sorting, setSorting] = useState<SortingState>([{ id: "startedAt", desc: true }]);
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([]);
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({});

  const fetchRuns = useCallback(async () => {
    if (!slug) return;
    setLoading(true);
    setError(null);
    try {
      const rows = await listRuns(slug);
      setRuns(rows);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load runs");
      setRuns([]);
    } finally {
      setLoading(false);
    }
  }, [slug]);

  useEffect(() => {
    void fetchRuns();
  }, [fetchRuns]);

  // Recipe id -> name lookup for the Recipe column (mirrors ref's preset
  // lookup); best-effort, falls back to the id's short form on failure.
  useEffect(() => {
    if (!slug) return;
    let cancelled = false;
    listRecipes(slug)
      .then((recipes: RecipeVM[]) => {
        if (cancelled) return;
        const m = new Map<string, string>();
        for (const r of recipes) m.set(r.id, r.name);
        setRecipeNames(m);
      })
      .catch(() => {
        /* recipe list optional — column falls back to id slice */
      });
    return () => {
      cancelled = true;
    };
  }, [slug]);

  const handleCancel = useCallback(
    async (run: RunVM) => {
      if (!slug) return;
      const ok = await confirm({
        title: "Cancel run?",
        description: "The run will be stopped. Already-provisioned resources are torn down during teardown.",
        danger: true,
        confirmLabel: "Cancel run",
      });
      if (!ok) return;
      setBusyId(run.id);
      setError(null);
      try {
        await cancelRun(slug, run.id);
        await fetchRuns();
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to cancel run");
      } finally {
        setBusyId(null);
      }
    },
    [slug, confirm, fetchRuns],
  );

  const handleDelete = useCallback(
    async (run: RunVM) => {
      if (!slug) return;
      const ok = await confirm({
        title: "Delete run?",
        description: `"${run.name || run.id}" will be permanently removed. This cannot be undone.`,
        danger: true,
        confirmLabel: "Delete",
      });
      if (!ok) return;
      setBusyId(run.id);
      setError(null);
      try {
        await deleteRun(slug, run.id);
        await fetchRuns();
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to delete run");
      } finally {
        setBusyId(null);
      }
    },
    [slug, confirm, fetchRuns],
  );

  // Rerun navigates to the originating recipe's launch form, prefilled from
  // this run's own stored Baked snapshot — same contract as RunDetail's
  // Rerun button (see RunDetail.tsx's onRerun doc comment / commit 82d6a8b0).
  const handleRerun = useCallback(
    (run: RunVM) => {
      if (!run.workflowId) return;
      navigate(`/recipes/${run.workflowId}/launch?from=${encodeURIComponent(run.id)}`);
    },
    [navigate],
  );

  const columns = useMemo(
    () => makeColumns({ onDelete: handleDelete, onCancel: handleCancel, onRerun: handleRerun, busyId, recipeNames }),
    [handleDelete, handleCancel, handleRerun, busyId, recipeNames],
  );

  const filterValues = useMemo(() => {
    const statuses = new Set<string>();
    const dbKinds = new Set<string>();
    const providers = new Set<string>();
    for (const r of runs) {
      statuses.add(r.status);
      if (r.dbKind) dbKinds.add(r.dbKind);
      if (r.provider) providers.add(r.provider);
    }
    return {
      statuses: Array.from(statuses).sort(),
      dbKinds: Array.from(dbKinds).sort(),
      providers: Array.from(providers).sort(),
    };
  }, [runs]);

  const activeStatus = columnFilters.find((f) => f.id === "status")?.value as string | undefined;
  const activeDbKind = columnFilters.find((f) => f.id === "dbKind")?.value as string | undefined;
  const activeProvider = columnFilters.find((f) => f.id === "provider")?.value as string | undefined;

  function setFilter(id: string, value: string | undefined) {
    setColumnFilters((prev) => {
      const without = prev.filter((f) => f.id !== id);
      if (!value || value === "all") return without;
      return [...without, { id, value }];
    });
  }

  const table = useReactTable({
    data: runs,
    columns,
    state: { sorting, columnFilters, rowSelection },
    onSortingChange: setSorting,
    onColumnFiltersChange: setColumnFilters,
    onRowSelectionChange: setRowSelection,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getRowId: (row) => row.id,
    initialState: { pagination: { pageSize: 25 } },
  });

  const selectedRunIds = Object.keys(rowSelection).filter((k) => rowSelection[k]);
  const canCompare = selectedRunIds.length >= 2;

  function handleCompare() {
    if (!canCompare) return;
    navigate(`/compare?runIds=${selectedRunIds.map(encodeURIComponent).join(",")}`);
  }

  const hasActiveFilters = !!(activeStatus || activeDbKind || activeProvider);

  return (
    <div className="p-5 flex flex-col gap-4 h-full min-h-0">
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <h1 className="text-base font-semibold font-mono tracking-tight">Runs</h1>
          <span className="text-[11px] text-zinc-600 font-mono tabular-nums">
            {table.getFilteredRowModel().rows.length} of {runs.length}
          </span>
        </div>

        <div className="flex items-center gap-2">
          {selectedRunIds.length > 0 && (
            <div className="flex items-center gap-2 mr-2">
              <span className="text-[11px] text-zinc-500 font-mono">{selectedRunIds.length} selected</span>
              {canCompare && (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={handleCompare}
                  className="border-primary/40 text-primary hover:bg-primary/10"
                >
                  <GitCompare className="h-3.5 w-3.5" />
                  Compare
                </Button>
              )}
              <Button size="sm" variant="ghost" onClick={() => setRowSelection({})} className="h-7 w-7 p-0 text-zinc-500">
                <X className="h-3.5 w-3.5" />
              </Button>
            </div>
          )}
          <Button size="sm" variant="outline" onClick={() => void fetchRuns()} disabled={loading}>
            <RefreshCw className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
            Refresh
          </Button>
        </div>
      </div>

      {error && (
        <div className="flex items-center gap-2 text-xs p-2.5 border border-destructive/30 text-destructive font-mono">
          <AlertCircle className="h-3.5 w-3.5 shrink-0" />
          {error}
        </div>
      )}

      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        {filterValues.statuses.length > 1 && (
          <div className="flex items-center gap-1.5">
            <span className="text-[10px] text-zinc-600 font-mono uppercase tracking-wider">Status</span>
            <FilterChip label="all" active={!activeStatus} onClick={() => setFilter("status", undefined)} />
            {filterValues.statuses.map((s) => (
              <FilterChip
                key={s}
                label={s}
                active={activeStatus === s}
                onClick={() => setFilter("status", activeStatus === s ? undefined : s)}
              />
            ))}
          </div>
        )}

        {filterValues.dbKinds.length > 1 && (
          <div className="flex items-center gap-1.5">
            <span className="text-[10px] text-zinc-600 font-mono uppercase tracking-wider">DB</span>
            <FilterChip label="all" active={!activeDbKind} onClick={() => setFilter("dbKind", undefined)} />
            {filterValues.dbKinds.map((k) => (
              <FilterChip
                key={k}
                label={k}
                active={activeDbKind === k}
                onClick={() => setFilter("dbKind", activeDbKind === k ? undefined : k)}
              />
            ))}
          </div>
        )}

        {filterValues.providers.length > 1 && (
          <div className="flex items-center gap-1.5">
            <span className="text-[10px] text-zinc-600 font-mono uppercase tracking-wider">Provider</span>
            <FilterChip label="all" active={!activeProvider} onClick={() => setFilter("provider", undefined)} />
            {filterValues.providers.map((p) => (
              <FilterChip
                key={p}
                label={p}
                active={activeProvider === p}
                onClick={() => setFilter("provider", activeProvider === p ? undefined : p)}
              />
            ))}
          </div>
        )}

        {hasActiveFilters && (
          <button
            type="button"
            onClick={() => setColumnFilters([])}
            className="text-[10px] text-zinc-500 hover:text-zinc-300 font-mono underline underline-offset-2 cursor-pointer"
          >
            clear filters
          </button>
        )}
      </div>

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
                    {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
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
                  data-state={row.getIsSelected() ? "selected" : undefined}
                  className="border-zinc-800/50 hover:bg-zinc-900/60 data-[state=selected]:bg-primary/[0.04] cursor-pointer transition-colors"
                  onClick={(e) => {
                    const target = e.target as HTMLElement;
                    if (target.closest("button, a, input, [data-row-stop]")) return;
                    navigate(`/runs/${row.original.id}`);
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
                    {loading ? "Loading runs..." : runs.length === 0 ? "No runs yet — start one from a recipe" : "No runs matching filters"}
                  </span>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      {runs.length > 0 && (
        <div className="flex items-center justify-between mt-auto">
          <div className="flex items-center gap-2">
            <span className="text-[11px] text-zinc-600 font-mono">Rows</span>
            <div className="flex gap-0.5">
              {PAGE_SIZES.map((size) => (
                <button
                  type="button"
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
            <Button
              variant="ghost"
              size="sm"
              className="h-7 w-7 p-0"
              onClick={() => table.setPageIndex(table.getPageCount() - 1)}
              disabled={!table.getCanNextPage()}
            >
              <ChevronsRight className="h-3.5 w-3.5" />
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
