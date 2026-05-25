import { Minus, TrendingDown, TrendingUp } from "lucide-react";

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";
import { MetricDiff_Verdict, type Comparison } from "@/lib/proto/cloud/v1/runtime/metrics/metrics_pb.ts";

function num(value: number, unit: string) {
  if (!Number.isFinite(value)) return "—";
  const rounded = Math.abs(value) >= 100 ? value.toFixed(0) : value.toFixed(2);
  return unit ? `${rounded} ${unit}` : rounded;
}

function Verdict({ verdict }: { verdict: MetricDiff_Verdict }) {
  if (verdict === MetricDiff_Verdict.BETTER) return <TrendingUp className="size-4 text-success" />;
  if (verdict === MetricDiff_Verdict.WORSE) return <TrendingDown className="size-4 text-destructive" />;
  return <Minus className="size-4 text-muted-foreground" />;
}

export function MetricsDiff({ comparison }: { comparison: Comparison }) {
  const summary = comparison.summary;
  return (
    <div className="space-y-4">
      {summary ? (
        <div className="flex gap-4 text-sm">
          <span className="text-success">{summary.better} better</span>
          <span className="text-destructive">{summary.worse} worse</span>
          <span className="text-muted-foreground">{summary.same} unchanged</span>
        </div>
      ) : null}
      <div className="overflow-hidden rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Metric</TableHead>
              <TableHead>Run A</TableHead>
              <TableHead>Run B</TableHead>
              <TableHead>Δ avg</TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {comparison.metrics.map((metric) => (
              <TableRow key={metric.key || metric.name}>
                <TableCell className="font-medium">{metric.name}</TableCell>
                <TableCell className="font-mono text-xs">{num(metric.avgA, metric.unit)}</TableCell>
                <TableCell className="font-mono text-xs">{num(metric.avgB, metric.unit)}</TableCell>
                <TableCell className={cn("font-mono text-xs", metric.diffAvgPct > 0 ? "text-success" : metric.diffAvgPct < 0 ? "text-destructive" : "")}>
                  {Number.isFinite(metric.diffAvgPct) ? `${metric.diffAvgPct > 0 ? "+" : ""}${metric.diffAvgPct.toFixed(1)}%` : "—"}
                </TableCell>
                <TableCell>
                  <Verdict verdict={metric.verdict} />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
