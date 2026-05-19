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
  type SortingState,
} from "@tanstack/react-table";
import { clients } from "@/api/clients";
import { useTenantId, useTenantPath } from "@/hooks/useTenantPath";
import type { TestSuite } from "@/lib/proto/cloud/v1/testing/test_suite_pb";
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
import {
  AlertCircle,
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  Copy,
  Layers,
  Pencil,
  Play,
  Plus,
  RefreshCw,
  Trash2,
  Zap,
} from "lucide-react";

const PAGE_SIZES = [10, 25, 50, 100];
const REFRESH_OPTIONS = [
  { label: "Off", value: 0 },
  { label: "5s", value: 5 },
  { label: "15s", value: 15 },
  { label: "30s", value: 30 },
];

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

function makeColumns(
  onLaunch: (s: TestSuite) => void,
  onEdit: (s: TestSuite) => void,
  onClone: (s: TestSuite) => void,
  onDelete: (s: TestSuite) => void,
  busyID: string | null,
): ColumnDef<TestSuite>[] {
  return [
    {
      accessorKey: "id",
      header: ({ column }) => <SortableHeader column={column} label="ID" />,
      cell: ({ row }) => {
        const id = row.original.id?.value ?? "";
        const short = id.length > 8 ? id.slice(-8) : id;
        return (
          <span className="font-mono text-xs text-primary" title={id}>
            {short}
          </span>
        );
      },
      size: 96,
    },
    {
      id: "name",
      accessorFn: (s) => s.identity?.name ?? "",
      header: ({ column }) => <SortableHeader column={column} label="Name" />,
      cell: ({ row }) => {
        const s = row.original;
        return (
          <div className="min-w-0 max-w-[22rem]">
            <span className="text-xs text-zinc-200 truncate block">{s.identity?.name}</span>
            {s.identity?.description && (
              <span className="text-[10px] text-zinc-600 font-mono truncate block">
                {s.identity.description}
              </span>
            )}
          </div>
        );
      },
    },
    {
      id: "targets",
      header: "Targets",
      cell: ({ row }) => {
        const n = row.original.matrix?.databases?.length ?? 0;
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
    {
      id: "workloads",
      header: "Workloads",
      cell: ({ row }) => {
        const n = row.original.matrix?.workloads?.length ?? 0;
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
    {
      id: "policy",
      header: "Policy",
      cell: ({ row }) => {
        const mode = row.original.policy?.mode;
        return (
          <Badge variant="secondary" className="text-[10px]">
            {mode === 1 ? "sequential" : mode === 2 ? "parallel" : "—"}
          </Badge>
        );
      },
      enableSorting: false,
      size: 110,
    },
    {
      id: "actions",
      header: "",
      cell: ({ row }) => {
        const s = row.original;
        const id = s.id?.value ?? "";
        const cantLaunch = busyID === id ||
          (s.matrix?.databases?.length ?? 0) === 0 ||
          (s.matrix?.workloads?.length ?? 0) === 0;
        return (
          <div className="flex items-center gap-0.5">
            <Button
              size="sm"
              variant="ghost"
              className="h-7 w-7 p-0 text-emerald-600 hover:text-emerald-400 cursor-pointer disabled:text-zinc-700"
              title={cantLaunch ? "Add at least one DB and one workload first" : "Launch matrix now"}
              disabled={cantLaunch}
              onClick={(e) => { e.preventDefault(); e.stopPropagation(); onLaunch(s); }}
            >
              <Play className="h-3.5 w-3.5" />
            </Button>
            <Button
              size="sm"
              variant="ghost"
              className="h-7 w-7 p-0 text-zinc-500 hover:text-primary cursor-pointer"
              title="Edit suite"
              onClick={(e) => { e.preventDefault(); e.stopPropagation(); onEdit(s); }}
            >
              <Pencil className="h-3.5 w-3.5" />
            </Button>
            <Button
              size="sm"
              variant="ghost"
              className="h-7 w-7 p-0 text-zinc-500 hover:text-primary cursor-pointer"
              title="Clone suite"
              disabled={busyID === id}
              onClick={(e) => { e.preventDefault(); e.stopPropagation(); onClone(s); }}
            >
              <Copy className="h-3.5 w-3.5" />
            </Button>
            <Button
              size="sm"
              variant="ghost"
              className="h-7 w-7 p-0 text-zinc-600 hover:text-destructive cursor-pointer"
              title="Delete suite"
              disabled={busyID === id}
              onClick={(e) => { e.preventDefault(); e.stopPropagation(); onDelete(s); }}
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
  const tid = useTenantId();
  const tPath = useTenantPath();

  const [suites, setSuites] = useState<TestSuite[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busyID, setBusyID] = useState<string | null>(null);

  const [refreshInterval, setRefreshInterval] = useState(15);
  const refreshRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const [sorting, setSorting] = useState<SortingState>([{ id: "name", desc: false }]);

  async function fetchSuites() {
    setLoading(true);
    setError(null);
    try {
      const resp = await clients.suite.listTestSuites(
        tid ? { value: tid } : {}
      );
      setSuites(resp.testSuites ?? []);
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
    return () => { if (refreshRef.current) clearInterval(refreshRef.current); };
  }, [refreshInterval]);

  async function handleLaunch(s: TestSuite) {
    const id = s.id?.value ?? "";
    setBusyID(id);
    try {
      const r = await clients.suiteRun.launchTestSuite({ value: id });
      await fetchSuites();
      navigate(tPath(`runs?suite=${id}&batch=${r.id?.value ?? ""}`));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Launch failed");
    } finally {
      setBusyID(null);
    }
  }

  async function handleClone(s: TestSuite) {
    const id = s.id?.value ?? "";
    setBusyID(id);
    try {
      const r = await clients.suite.cloneTestSuite({ value: id });
      await fetchSuites();
      navigate(tPath(`suites/${r.id?.value}/edit`));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Clone failed");
    } finally {
      setBusyID(null);
    }
  }

  async function handleDelete(s: TestSuite) {
    const ok = await confirm({
      title: `Delete suite "${s.identity?.name}"?`,
      description: "This removes the suite and its items. Past runs stay visible.",
      danger: true,
    });
    if (!ok) return;
    const id = s.id?.value ?? "";
    setBusyID(id);
    try {
      await clients.suite.deleteTestSuite({ value: id });
      await fetchSuites();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete suite");
    } finally {
      setBusyID(null);
    }
  }

  // eslint-disable-next-line react-hooks/exhaustive-deps
  const columns = useMemo(
    () => makeColumns(
      handleLaunch,
      (s) => navigate(tPath(`suites/${s.id?.value}/edit`)),
      handleClone,
      handleDelete,
      busyID,
    ),
    [busyID],
  );

  const table = useReactTable({
    data: suites,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getRowId: (row) => row.id?.value ?? "",
    initialState: { pagination: { pageSize: 25 } },
  });

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
            onClick={() => navigate(tPath("suites/new"))}
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
              <RefreshCw className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
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
                    if (target.closest("button, a, input")) return;
                    navigate(tPath(`suites/${row.original.id?.value}`));
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
                      : "No suites yet — create one to bundle (db × workload) runs."}
                  </span>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      {/* Pagination */}
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

export default Suites;
