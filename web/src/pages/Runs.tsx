import { useEffect, useState, useRef } from "react";
import { useNavigate } from "react-router-dom";
import { clients } from "@/api/clients";
import { getTenantId } from "@/api/transport";
import { protoTsToISO } from "@/lib/proto-helpers";
import type { TestRun } from "@/lib/proto/cloud/v1/testing/test_run_pb";
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
import { RefreshCw, Play, AlertCircle } from "lucide-react";

function runStatus(r: TestRun): string {
  return r.dagRunId ? "RUNNING" : "PENDING";
}

function formatDate(iso: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "—";
  return d.toLocaleString("en-GB", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

export function Runs() {
  const navigate = useNavigate();
  const [runs, setRuns] = useState<TestRun[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const refreshRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const [refreshInterval, setRefreshInterval] = useState(5);

  const REFRESH_OPTIONS = [
    { label: "Off", value: 0 },
    { label: "3s", value: 3 },
    { label: "5s", value: 5 },
    { label: "10s", value: 10 },
    { label: "30s", value: 30 },
  ];

  async function fetchRuns() {
    const tid = getTenantId();
    if (!tid) return;
    setLoading(true);
    setError(null);
    try {
      const r = await clients.testRun.listTestRuns({ value: tid });
      setRuns(r.testRuns ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load runs");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    fetchRuns();
  }, []);

  useEffect(() => {
    if (refreshRef.current) clearInterval(refreshRef.current);
    if (refreshInterval > 0) {
      refreshRef.current = setInterval(fetchRuns, refreshInterval * 1000);
    }
    return () => {
      if (refreshRef.current) clearInterval(refreshRef.current);
    };
  }, [refreshInterval]);

  const tid = getTenantId();
  if (!tid) return null;

  return (
    <div className="p-5 flex flex-col gap-4 min-h-full">
      {/* Header */}
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <h1 className="text-base font-semibold font-mono tracking-tight">Test Runs</h1>
          <span className="text-[11px] text-zinc-600 font-mono tabular-nums">
            {runs.length} run{runs.length !== 1 ? "s" : ""}
          </span>
        </div>

        <div className="flex items-center gap-2">
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
            <TableRow className="border-zinc-800/80 hover:bg-transparent">
              <TableHead className="text-[11px] font-mono uppercase tracking-wider text-zinc-500 h-9 bg-zinc-900/50">ID</TableHead>
              <TableHead className="text-[11px] font-mono uppercase tracking-wider text-zinc-500 h-9 bg-zinc-900/50">Name</TableHead>
              <TableHead className="text-[11px] font-mono uppercase tracking-wider text-zinc-500 h-9 bg-zinc-900/50">Status</TableHead>
              <TableHead className="text-[11px] font-mono uppercase tracking-wider text-zinc-500 h-9 bg-zinc-900/50">Created</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {runs.length > 0 ? (
              runs.map((r) => {
                const id = r.id?.value ?? "";
                const shortId = id.length > 8 ? id.slice(-8) : id;
                const status = runStatus(r);
                const createdAt = protoTsToISO(r.timestamps?.createdAt);
                return (
                  <TableRow
                    key={id}
                    className="border-zinc-800/50 hover:bg-zinc-900/60 cursor-pointer transition-colors"
                    onClick={() => navigate(`/runs/${id}`)}
                  >
                    <TableCell className="py-2.5">
                      <span className="font-mono text-xs text-primary" title={id}>
                        {shortId}
                      </span>
                    </TableCell>
                    <TableCell className="py-2.5">
                      <span className="text-xs text-zinc-200 truncate block max-w-[18rem]">
                        {r.identity?.name || <span className="text-zinc-700">&mdash;</span>}
                      </span>
                    </TableCell>
                    <TableCell className="py-2.5">
                      <Badge
                        variant={status === "RUNNING" ? "warning" : "secondary"}
                      >
                        {status}
                      </Badge>
                    </TableCell>
                    <TableCell className="py-2.5">
                      <span className="text-xs text-zinc-500 font-mono">
                        {formatDate(createdAt)}
                      </span>
                    </TableCell>
                  </TableRow>
                );
              })
            ) : (
              <TableRow>
                <TableCell colSpan={4} className="h-32 text-center">
                  <span className="text-xs text-zinc-600 font-mono">
                    {loading ? "Loading runs..." : "No runs yet"}
                  </span>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
