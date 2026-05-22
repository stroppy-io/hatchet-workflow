import type { ComparisonRow } from "@/api/types";
import { ArrowUp, ArrowDown, Minus } from "lucide-react";

interface MetricsDiffProps {
  rows: ComparisonRow[];
  runA?: string;
  runB?: string;
}

function formatValue(val: number, unit: string): string {
  if (unit === "%" || unit === "percent") return `${val.toFixed(1)}%`;
  if (unit === "ms") {
    if (val >= 1000) return `${(val / 1000).toFixed(2)}s`;
    return `${val.toFixed(1)}ms`;
  }
  if (unit === "bytes/s" || unit === "B/s") {
    if (val >= 1e9) return `${(val / 1e9).toFixed(1)} GB/s`;
    if (val >= 1e6) return `${(val / 1e6).toFixed(1)} MB/s`;
    if (val >= 1e3) return `${(val / 1e3).toFixed(1)} KB/s`;
    return `${val.toFixed(0)} B/s`;
  }
  if (Math.abs(val) >= 1e6) return `${(val / 1e6).toFixed(1)}M`;
  if (Math.abs(val) >= 1e3) return `${(val / 1e3).toFixed(1)}K`;
  if (Number.isInteger(val)) return val.toString();
  return val.toFixed(2);
}

function VerdictIcon({ verdict }: { verdict: string }) {
  if (verdict === "better") return <ArrowUp className="h-3 w-3 text-emerald-400" />;
  if (verdict === "worse") return <ArrowDown className="h-3 w-3 text-red-400" />;
  return <Minus className="h-3 w-3 text-zinc-600" />;
}

export function MetricsDiff({ rows, runA, runB }: MetricsDiffProps) {
  if (rows.length === 0) {
    return (
      <div className="text-sm text-muted-foreground p-4">
        No comparison data available.
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b text-left text-[10px] font-mono uppercase tracking-wider text-zinc-500">
            <th className="py-2.5 px-3 font-medium">Metric</th>
            <th className="py-2.5 px-3 font-medium text-right">
              {runA ? <span className="text-cyan-400">{runA}</span> : "Run A"} avg
            </th>
            <th className="py-2.5 px-3 font-medium text-right">
              {runB ? <span className="text-amber-400">{runB}</span> : "Run B"} avg
            </th>
            <th className="py-2.5 px-3 font-medium text-right">Diff %</th>
            <th className="py-2.5 px-3 font-medium w-20"></th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr
              key={row.key}
              className={`border-b border-zinc-800/40 hover:bg-zinc-900/30 ${
                row.verdict === "better"
                  ? "bg-emerald-500/[0.02]"
                  : row.verdict === "worse"
                    ? "bg-red-500/[0.02]"
                    : ""
              }`}
            >
              <td className="py-2 px-3">
                <div className="flex flex-col">
                  <span className="font-mono text-xs text-zinc-300">{row.name}</span>
                  <span className="font-mono text-[10px] text-zinc-600">{row.key}</span>
                </div>
              </td>
              <td className="py-2 px-3 text-right font-mono text-xs text-zinc-300 tabular-nums">
                {formatValue(row.avg_a, row.unit)}
              </td>
              <td className="py-2 px-3 text-right font-mono text-xs text-zinc-300 tabular-nums">
                {formatValue(row.avg_b, row.unit)}
              </td>
              <td className="py-2 px-3 text-right font-mono text-xs tabular-nums">
                <span
                  className={
                    row.verdict === "better"
                      ? "text-emerald-400"
                      : row.verdict === "worse"
                        ? "text-red-400"
                        : "text-zinc-600"
                  }
                >
                  {row.diff_avg_pct > 0 ? "+" : ""}
                  {row.diff_avg_pct.toFixed(1)}%
                </span>
              </td>
              <td className="py-2 px-3">
                <div className="flex items-center gap-1.5">
                  <VerdictIcon verdict={row.verdict} />
                  <span
                    className={`text-[10px] font-mono ${
                      row.verdict === "better"
                        ? "text-emerald-400"
                        : row.verdict === "worse"
                          ? "text-red-400"
                          : "text-zinc-600"
                    }`}
                  >
                    {row.verdict}
                  </span>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
