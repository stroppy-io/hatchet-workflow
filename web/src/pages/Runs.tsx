import { useEffect, useState, useMemo, useRef } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
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
import { listRuns, deleteRun, cancelRun, getRunStatus, listPresets, getSuite, listSuites } from "@/api/client";
import type { RunSummary, RunConfig } from "@/api/types";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useConfirm } from "@/components/ui/confirm-dialog";
import {
  RefreshCw,
  Trash2,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  ArrowUpDown,
  ArrowUp,
  ArrowDown,
  GitCompare,
  X,
  Play,
  AlertCircle,
  StopCircle,
  RotateCcw,
  Boxes,
} from "lucide-react";

// --- Helpers ---

type RunStatus = "queued" | "done" | "failed" | "running" | "pending" | "cancelled" | "cancelling";

function deriveStatus(r: RunSummary, cancellingIds?: Set<string>): RunStatus {
  if (cancellingIds?.has(r.id)) {
    // Still cancelling — check if teardown finished yet.
    if (r.cancelled || (r.failed > 0 && r.done === r.total - r.failed - r.pending)) return "cancelled";
    return "cancelling";
  }
  if (r.cancelled) return "cancelled";
  if (r.failed > 0) return "failed";
  if (r.done === r.total && r.total > 0) return "done";
  if (r.done > 0) return "running";
  // Queued runs have no snapshot yet — listRuns synthesises a row with
  // total=0, done=0, pending=1 so we can tell them apart from "ran but
  // produced no nodes".
  if (r.total === 0 && r.pending > 0) return "queued";
  return "pending";
}

const STATUS_CONFIG: Record<RunStatus, { label: string; variant: "success" | "destructive" | "warning" | "pending" | "secondary" }> = {
  queued: { label: "Queued", variant: "secondary" },
  done: { label: "Done", variant: "success" },
  failed: { label: "Failed", variant: "destructive" },
  cancelling: { label: "Cancelling", variant: "warning" },
  running: { label: "Running", variant: "warning" },
  pending: { label: "Pending", variant: "pending" },
  cancelled: { label: "Cancelled", variant: "secondary" },
};

function formatTimestamp(ts?: string): string {
  if (!ts) return "\u2014";
  const d = new Date(ts);
  if (isNaN(d.getTime())) return "\u2014";
  // Zero-value Go time
  if (d.getFullYear() < 2000) return "\u2014";
  return d.toLocaleString("en-GB", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

function durationBetween(start?: string, end?: string): string {
  if (!start) return "\u2014";
  const s = new Date(start);
  if (isNaN(s.getTime()) || s.getFullYear() < 2000) return "\u2014";
  const e = end ? new Date(end) : new Date();
  if (isNaN(e.getTime()) || e.getFullYear() < 2000) return "\u2014";
  const sec = Math.max(0, Math.round((e.getTime() - s.getTime()) / 1000));
  if (sec < 60) return `${sec}s`;
  if (sec < 3600) return `${Math.floor(sec / 60)}m ${sec % 60}s`;
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  return `${h}h ${m}m`;
}

// --- Filter button component ---

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

// --- Column definitions ---

function makeColumns(onDelete: (id: string) => void, onCancel: (id: string) => void, onRerun: (id: string) => void, cancellingIds: Set<string>, presetNames: Map<string, string>): ColumnDef<RunSummary>[] {
  return [
    // Checkbox
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
    // ID (truncated to last 8 chars). Whole row is clickable; this column
    // just renders the short id text — the navigation is handled by the
    // TableRow onClick. Title attr keeps the full id discoverable on hover.
    {
      accessorKey: "id",
      header: ({ column }) => (
        <SortableHeader column={column} label="ID" />
      ),
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
    // Name (separate column — sortable, falls back to em dash when unset).
    {
      accessorKey: "name",
      header: ({ column }) => (
        <SortableHeader column={column} label="Name" />
      ),
      cell: ({ row }) => {
        const r = row.original;
        if (!r.name) {
          return <span className="text-xs text-zinc-700">—</span>;
        }
        const titleAttr = r.description ? `${r.name}\n\n${r.description}` : r.name;
        return (
          <span className="text-xs text-zinc-200 truncate block max-w-[18rem]" title={titleAttr}>
            {r.name}
          </span>
        );
      },
    },
    // Status
    {
      id: "status",
      accessorFn: (row) => deriveStatus(row, cancellingIds),
      header: ({ column }) => (
        <SortableHeader column={column} label="Status" />
      ),
      cell: ({ row }) => {
        const status = deriveStatus(row.original, cancellingIds);
        const cfg = STATUS_CONFIG[status];
        return <Badge variant={cfg.variant}>{cfg.label}</Badge>;
      },
      filterFn: (row, _id, value) => {
        if (!value || value === "all") return true;
        return deriveStatus(row.original, cancellingIds) === value;
      },
    },
    // Database (kind + version + node count)
    {
      accessorKey: "db_kind",
      header: ({ column }) => (
        <SortableHeader column={column} label="Database" />
      ),
      cell: ({ row }) => {
        const r = row.original;
        if (!r.db_kind) return <span className="font-mono text-xs text-zinc-600">{"\u2014"}</span>;
        const parts = [r.db_kind];
        if (r.db_version) parts.push(r.db_version);
        const label = parts.join(" ");
        const nodes = r.node_count ? `${r.node_count} node${r.node_count > 1 ? "s" : ""}` : null;
        return (
          <span className="font-mono text-xs text-zinc-400">
            {label}{nodes && <span className="text-zinc-600"> &middot; {nodes}</span>}
          </span>
        );
      },
      filterFn: (row, _id, value) => {
        if (!value || value === "all") return true;
        return row.original.db_kind === value;
      },
    },
    // Preset (resolved from preset_id)
    {
      accessorKey: "preset_id",
      header: ({ column }) => (
        <SortableHeader column={column} label="Preset" />
      ),
      cell: ({ row }) => {
        const id = row.original.preset_id;
        if (!id) return <span className="font-mono text-xs text-zinc-600">{"—"}</span>;
        const name = presetNames.get(id);
        return (
          <span className="font-mono text-xs text-zinc-400" title={id}>
            {name ?? id.slice(0, 8)}
          </span>
        );
      },
      enableSorting: false,
    },
    // Workload (script + duration + VUs)
    {
      id: "workload",
      header: "Workload",
      cell: ({ row }) => {
        const r = row.original;
        if (!r.script) return <span className="font-mono text-xs text-zinc-600">{"\u2014"}</span>;
        const parts = [r.script];
        if (r.duration) parts.push(r.duration);
        if (r.vus) parts.push(`${r.vus} VUs`);
        return (
          <span className="font-mono text-xs text-zinc-400">
            {parts.join(" \u00b7 ")}
          </span>
        );
      },
      enableSorting: false,
    },
    // Provider
    {
      accessorKey: "provider",
      header: ({ column }) => (
        <SortableHeader column={column} label="Provider" />
      ),
      cell: ({ row }) => (
        <span className="font-mono text-xs text-zinc-400">
          {row.original.provider || "\u2014"}
        </span>
      ),
      filterFn: (row, _id, value) => {
        if (!value || value === "all") return true;
        return row.original.provider === value;
      },
    },
    // Progress (amber bar for cancelled)
    {
      id: "progress",
      header: "Progress",
      cell: ({ row }) => {
        const r = row.original;
        const pct = r.total > 0 ? (r.done / r.total) * 100 : 0;
        const barColor = r.cancelled
          ? "bg-zinc-500"
          : r.failed > 0
            ? "bg-destructive"
            : "bg-emerald-500";
        return (
          <div className="flex items-center gap-2 min-w-[120px]">
            <div className="flex-1 bg-zinc-900 h-1.5 overflow-hidden">
              <div
                className={`h-full transition-all duration-500 ${barColor}`}
                style={{ width: `${pct}%` }}
              />
            </div>
            <span className="text-[10px] font-mono text-zinc-500 tabular-nums w-12 text-right">
              {r.done}/{r.total}
            </span>
          </div>
        );
      },
      enableSorting: false,
    },
    // Duration
    {
      id: "duration",
      header: "Duration",
      cell: ({ row }) => (
        <span className="text-xs text-zinc-500 font-mono tabular-nums">
          {durationBetween(row.original.started_at, row.original.finished_at)}
        </span>
      ),
      enableSorting: false,
    },
    // Actions: cancel (for running) + delete
    {
      id: "actions",
      header: "",
      cell: ({ row }) => {
        const status = deriveStatus(row.original, cancellingIds);
        const isFinished = status === "done" || status === "failed" || status === "cancelled";
        return (
          <div className="flex items-center gap-0.5">
            {status === "running" && (
              <Button
                size="sm"
                variant="ghost"
                className="h-7 w-7 p-0 text-amber-600 hover:text-amber-400 cursor-pointer"
                title="Cancel run"
                onClick={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  onCancel(row.original.id);
                }}
              >
                <StopCircle className="h-3.5 w-3.5" />
              </Button>
            )}
            {isFinished && (
              <Button
                size="sm"
                variant="ghost"
                className="h-7 w-7 p-0 text-zinc-500 hover:text-primary cursor-pointer"
                title="Rerun with same config"
                onClick={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  onRerun(row.original.id);
                }}
              >
                <RotateCcw className="h-3.5 w-3.5" />
              </Button>
            )}
            <Button
              size="sm"
              variant="ghost"
              className="h-7 w-7 p-0 text-zinc-600 hover:text-destructive cursor-pointer"
              title="Delete run"
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                onDelete(row.original.id);
              }}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          </div>
        );
      },
      enableSorting: false,
      size: 100,
    },
  ];
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

// --- Page size options ---
const PAGE_SIZES = [10, 25, 50, 100];

// --- Main component ---

export function Runs() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  // Suite + batch filtering — populated from URL so links from the Suites
  // page deep-link straight to "all runs from suite X" or "batch Y of suite X".
  const suiteFilter = searchParams.get("suite") || "";
  const batchFilter = searchParams.get("batch") || "";
  const [runs, setRuns] = useState<RunSummary[]>([]);
  const [presetNames, setPresetNames] = useState<Map<string, string>>(new Map());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [cancellingIds, setCancellingIds] = useState<Set<string>>(new Set());
  const confirm = useConfirm();

  // Load presets once for id→name lookup in the table.
  useEffect(() => {
    listPresets()
      .then((ps) => {
        const m = new Map<string, string>();
        for (const p of ps ?? []) m.set(p.id, p.name);
        setPresetNames(m);
      })
      .catch(() => {/* preset list optional — column falls back to id slice */});
  }, []);

  // Suite name + batch run_id allowlist, populated when ?suite= / ?batch=
  // query params are present. Lets the active-filter chip show a real name
  // and lets the batch filter narrow rows to exactly the runs of that batch.
  const [suiteName, setSuiteName] = useState<string>("");
  const [batchRunIds, setBatchRunIds] = useState<Set<string> | null>(null);
  useEffect(() => {
    if (!suiteFilter) {
      setSuiteName(""); setBatchRunIds(null);
      return;
    }
    let cancelled = false;
    if (batchFilter) {
      // Resolve the batch's runs through getSuite; the runs[] field is
      // populated by the API for individual suite GETs (not list).
      getSuite(suiteFilter)
        .then((s) => {
          if (cancelled) return;
          setSuiteName(s.name);
          const ids = new Set<string>();
          for (const r of s.runs ?? []) {
            if (r.batch_id === batchFilter) ids.add(r.run_id);
          }
          setBatchRunIds(ids);
        })
        .catch(() => { if (!cancelled) setBatchRunIds(new Set()); });
    } else {
      // Just need a name for the chip — list call is cheap.
      listSuites()
        .then((all) => {
          if (cancelled) return;
          const found = all.find((s) => s.id === suiteFilter);
          setSuiteName(found?.name || "");
          setBatchRunIds(null);
        })
        .catch(() => { /* leave name empty */ });
    }
    return () => { cancelled = true; };
  }, [suiteFilter, batchFilter]);

  // Auto-refresh
  const REFRESH_OPTIONS = [
    { label: "Off", value: 0 },
    { label: "3s", value: 3 },
    { label: "5s", value: 5 },
    { label: "10s", value: 10 },
    { label: "30s", value: 30 },
  ];
  const [refreshInterval, setRefreshInterval] = useState(5); // default 5s
  const refreshRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Table state
  const [sorting, setSorting] = useState<SortingState>([{ id: "started_at", desc: true }]);
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([]);
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({});


  // Derive unique filter values
  const filterValues = useMemo(() => {
    const statuses = new Set<string>();
    const dbKinds = new Set<string>();
    const providers = new Set<string>();
    for (const r of runs) {
      statuses.add(deriveStatus(r, cancellingIds));
      if (r.db_kind) dbKinds.add(r.db_kind);
      if (r.provider) providers.add(r.provider);
    }
    return {
      statuses: Array.from(statuses).sort(),
      dbKinds: Array.from(dbKinds).sort(),
      providers: Array.from(providers).sort(),
    };
  }, [runs]);

  // Active filter helpers
  const activeStatus = columnFilters.find((f) => f.id === "status")?.value as string | undefined;
  const activeDbKind = columnFilters.find((f) => f.id === "db_kind")?.value as string | undefined;
  const activeProvider = columnFilters.find((f) => f.id === "provider")?.value as string | undefined;

  function setFilter(id: string, value: string | undefined) {
    setColumnFilters((prev) => {
      const without = prev.filter((f) => f.id !== id);
      if (!value || value === "all") return without;
      return [...without, { id, value }];
    });
  }

  async function fetchRuns() {
    setLoading(true);
    setError(null);
    try {
      const result = await listRuns();
      setRuns(result ?? []);
      // Clear cancellingIds for runs that reached cancelled state.
      setCancellingIds((prev) => {
        if (prev.size === 0) return prev;
        const next = new Set(prev);
        for (const r of result ?? []) {
          if (next.has(r.id) && r.cancelled) next.delete(r.id);
        }
        return next.size === prev.size ? prev : next;
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load runs");
    } finally {
      setLoading(false);
    }
  }

  async function handleDelete(runID: string) {
    if (!(await confirm({ title: `Delete run "${runID}"?`, description: "This will also remove its Docker resources. Cannot be undone.", danger: true }))) return;
    try {
      await deleteRun(runID);
      await fetchRuns();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete run");
    }
  }

  async function handleCancel(runID: string) {
    setCancellingIds((prev) => new Set(prev).add(runID));
    try {
      await cancelRun(runID);
      // Don't fetchRuns immediately — let auto-refresh pick up the transition.
      // This ensures "Cancelling" is visible before it becomes "Cancelled".
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to cancel run");
      setCancellingIds((prev) => { const n = new Set(prev); n.delete(runID); return n; });
    }
  }

  // Rerun: fetch the run snapshot to extract its RunConfig, drop it into
  // sessionStorage under the same key NewRun reads on mount, then navigate
  // to /runs/new. Same shape as the per-run-detail Rerun button.
  async function handleRerun(runID: string) {
    try {
      const snap = await getRunStatus(runID);
      const rc = snap?.state?.run_config;
      let cfg: RunConfig | null = null;
      if (typeof rc === "string") {
        cfg = JSON.parse(rc) as RunConfig;
      } else if (rc && typeof rc === "object") {
        cfg = rc as unknown as RunConfig;
      }
      if (!cfg) {
        setError(`Run ${runID} has no saved config to rerun`);
        return;
      }
      sessionStorage.setItem("rerun_config", JSON.stringify(cfg));
      navigate(`/runs/new?kind=${cfg.database?.kind || "postgres"}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load run for rerun");
    }
  }

  // Initial fetch + auto-refresh interval
  useEffect(() => {
    fetchRuns();
  }, []);

  useEffect(() => {
    if (refreshRef.current) clearInterval(refreshRef.current);
    if (refreshInterval > 0) {
      refreshRef.current = setInterval(() => {
        fetchRuns();
      }, refreshInterval * 1000);
    }
    return () => {
      if (refreshRef.current) clearInterval(refreshRef.current);
    };
  }, [refreshInterval]);

  // eslint-disable-next-line react-hooks/exhaustive-deps
  const columns = useMemo(() => makeColumns(handleDelete, handleCancel, handleRerun, cancellingIds, presetNames), [cancellingIds, presetNames]);

  // Apply the URL-driven suite/batch filters before handing rows to the
  // table — so per-column filters (status, db, provider) compose with the
  // suite scope rather than fighting it.
  const scopedRuns = useMemo(() => {
    let out = runs;
    if (suiteFilter) out = out.filter((r) => r.suite_id === suiteFilter);
    if (batchRunIds) out = out.filter((r) => batchRunIds!.has(r.id));
    return out;
  }, [runs, suiteFilter, batchRunIds]);

  const table = useReactTable({
    data: scopedRuns,
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
    initialState: {
      pagination: { pageSize: 25 },
    },
  });

  // Selected runs for comparison
  const selectedRunIds = Object.keys(rowSelection).filter((k) => rowSelection[k]);
  const canCompare = selectedRunIds.length === 2;

  function handleCompare() {
    if (!canCompare) return;
    const [a, b] = selectedRunIds;
    navigate(`/compare?a=${encodeURIComponent(a)}&b=${encodeURIComponent(b)}`);
  }

  const hasActiveFilters = activeStatus || activeDbKind || activeProvider;

  return (
    <div className="p-5 flex flex-col gap-4 min-h-full">
      {/* Active suite filter chip — clicking the X clears both query
          params at once so a stuck batch filter doesn't outlive its suite. */}
      {suiteFilter && (
        <div className="flex items-center gap-2 px-2.5 py-1.5 border border-primary/30 bg-primary/[0.04] text-[11px] font-mono">
          <Boxes className="h-3.5 w-3.5 text-primary" />
          <span className="text-zinc-400">Suite:</span>
          <Link to={`/suites/${suiteFilter}`} className="text-primary hover:underline">
            {suiteName || suiteFilter.slice(0, 8)}
          </Link>
          {batchFilter && (
            <>
              <span className="text-zinc-600">·</span>
              <span className="text-zinc-400">batch</span>
              <span className="text-primary">{batchFilter.slice(0, 8)}</span>
            </>
          )}
          <button
            onClick={() => setSearchParams(new URLSearchParams())}
            className="ml-2 text-zinc-500 hover:text-destructive"
            title="Clear suite filter"
          >
            <X className="h-3 w-3" />
          </button>
        </div>
      )}

      {/* Header bar */}
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <h1 className="text-base font-semibold font-mono tracking-tight">Test Runs</h1>
          <span className="text-[11px] text-zinc-600 font-mono tabular-nums">
            {table.getFilteredRowModel().rows.length} of {runs.length}
          </span>
        </div>

        <div className="flex items-center gap-2">
          {/* Compare button */}
          {selectedRunIds.length > 0 && (
            <div className="flex items-center gap-2 mr-2">
              <span className="text-[11px] text-zinc-500 font-mono">
                {selectedRunIds.length} selected
              </span>
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
              <Button
                size="sm"
                variant="ghost"
                onClick={() => setRowSelection({})}
                className="h-7 w-7 p-0 text-zinc-500"
              >
                <X className="h-3.5 w-3.5" />
              </Button>
            </div>
          )}

          <button
            type="button"
            onClick={() => navigate("/runs/new")}
            className="flex items-center gap-1.5 px-2.5 py-1 text-[11px] font-mono border border-zinc-800 text-zinc-400 hover:text-zinc-200 hover:border-zinc-700 transition-colors cursor-pointer"
          >
            <Play className="h-3 w-3" />
            New Run
          </button>

          {/* Auto-refresh selector */}
          <div className="flex items-center gap-0.5 border border-zinc-800 px-1 py-0.5">
            <button
              type="button"
              onClick={fetchRuns}
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

      {/* Filters */}
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        {/* Status filter */}
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

        {/* DB Kind filter */}
        {filterValues.dbKinds.length > 1 && (
          <div className="flex items-center gap-1.5">
            <span className="text-[10px] text-zinc-600 font-mono uppercase tracking-wider">DB</span>
            <FilterChip label="all" active={!activeDbKind} onClick={() => setFilter("db_kind", undefined)} />
            {filterValues.dbKinds.map((k) => (
              <FilterChip
                key={k}
                label={k}
                active={activeDbKind === k}
                onClick={() => setFilter("db_kind", activeDbKind === k ? undefined : k)}
              />
            ))}
          </div>
        )}

        {/* Provider filter */}
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
                  data-state={row.getIsSelected() ? "selected" : undefined}
                  className="border-zinc-800/50 hover:bg-zinc-900/60 data-[state=selected]:bg-primary/[0.04] cursor-pointer transition-colors"
                  onClick={(e) => {
                    // Ignore clicks originating in interactive cells (checkbox,
                    // action buttons, links). Lets row click be the dominant
                    // gesture without trapping the existing controls.
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
                    {loading
                      ? "Loading runs..."
                      : runs.length === 0
                        ? "No runs yet"
                        : "No runs matching filters"}
                  </span>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      {/* Pagination */}
      {runs.length > 0 && (
        <div className="flex items-center justify-between mt-auto">
          {/* Page size selector */}
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

          {/* Page navigation */}
          <div className="flex items-center gap-1">
            <span className="text-[11px] text-zinc-600 font-mono tabular-nums mr-2">
              {table.getState().pagination.pageIndex * table.getState().pagination.pageSize + 1}
              {"\u2013"}
              {Math.min(
                (table.getState().pagination.pageIndex + 1) * table.getState().pagination.pageSize,
                table.getFilteredRowModel().rows.length
              )}
              {" of "}
              {table.getFilteredRowModel().rows.length}
            </span>
            <Button
              variant="ghost"
              size="sm"
              className="h-7 w-7 p-0"
              onClick={() => table.setPageIndex(0)}
              disabled={!table.getCanPreviousPage()}
            >
              <ChevronsLeft className="h-3.5 w-3.5" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="h-7 w-7 p-0"
              onClick={() => table.previousPage()}
              disabled={!table.getCanPreviousPage()}
            >
              <ChevronLeft className="h-3.5 w-3.5" />
            </Button>
            <span className="text-[11px] text-zinc-500 font-mono tabular-nums px-2">
              {table.getState().pagination.pageIndex + 1}/{table.getPageCount()}
            </span>
            <Button
              variant="ghost"
              size="sm"
              className="h-7 w-7 p-0"
              onClick={() => table.nextPage()}
              disabled={!table.getCanNextPage()}
            >
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
