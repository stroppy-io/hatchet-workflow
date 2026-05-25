import { Minus, TrendingDown, TrendingUp } from "lucide-react";

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";
import { Verdict, type Comparison, type MetricCell } from "@/lib/proto/cloud/v1/runtime/metrics/metrics_pb.ts";

function num(value: number) {
  if (!Number.isFinite(value)) return "—";
  return Math.abs(value) >= 100 ? value.toFixed(0) : value.toFixed(2);
}

function shortId(id: string) {
  return id.length > 10 ? `${id.slice(0, 8)}…` : id;
}

function VerdictIcon({ verdict }: { verdict: Verdict }) {
  if (verdict === Verdict.BETTER) return <TrendingUp className="size-3.5 text-success" />;
  if (verdict === Verdict.WORSE) return <TrendingDown className="size-3.5 text-destructive" />;
  return <Minus className="size-3.5 text-muted-foreground" />;
}

// Per-cell delta vs baseline, coloured by the (direction-aware) verdict.
function CellDiff({ cell }: { cell: MetricCell }) {
  if (!Number.isFinite(cell.diffAvgPct)) return null;
  const cls =
    cell.verdict === Verdict.BETTER ? "text-success" : cell.verdict === Verdict.WORSE ? "text-destructive" : "text-muted-foreground";
  return (
    <span className={cn("inline-flex items-center gap-1 font-mono text-[11px]", cls)}>
      {cell.diffAvgPct > 0 ? "+" : ""}
      {cell.diffAvgPct.toFixed(1)}%
      <VerdictIcon verdict={cell.verdict} />
    </span>
  );
}

// MetricsDiff renders an N-way comparison matrix: rows are metrics, columns are
// runs (run_ids[0] = baseline). Cells aligned 1:1 with comparison.runIds.
export function MetricsDiff({ comparison, labels }: { comparison: Comparison; labels?: Record<string, string> }) {
  const runIds = comparison.runIds;
  const label = (id: string) => labels?.[id] || shortId(id);
  const summaryByRun = new Map(comparison.summaries.map((s) => [s.runId, s]));

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-3 text-sm">
        {runIds.slice(1).map((id) => {
          const s = summaryByRun.get(id);
          return (
            <div key={id} className="flex items-center gap-3 rounded-md border px-3 py-1.5">
              <span className="font-medium">{label(id)}</span>
              <span className="text-success">{s?.better ?? 0} better</span>
              <span className="text-destructive">{s?.worse ?? 0} worse</span>
              <span className="text-muted-foreground">{s?.same ?? 0} same</span>
            </div>
          );
        })}
      </div>
      <div className="overflow-auto rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Metric</TableHead>
              {runIds.map((id, i) => (
                <TableHead key={id || i} className="font-mono">
                  {label(id)}
                  {i === 0 ? <span className="ml-1 text-[10px] uppercase text-muted-foreground">baseline</span> : null}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {comparison.metrics.map((row) => (
              <TableRow key={row.key || row.name}>
                <TableCell className="font-medium">
                  {row.name}
                  {row.unit ? <span className="ml-1 text-xs text-muted-foreground">{row.unit}</span> : null}
                </TableCell>
                {row.cells.map((cell, i) => (
                  <TableCell key={cell.runId || i}>
                    <div className="flex flex-col gap-0.5">
                      <span className="font-mono text-xs">{num(cell.avg)}</span>
                      {i > 0 ? <CellDiff cell={cell} /> : null}
                    </div>
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
