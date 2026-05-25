import { useMemo } from "react";

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type { MetricSummary } from "@/lib/proto/cloud/v1/runtime/metrics/metrics_pb.ts";

function formatValue(value: number, unit: string) {
  if (!Number.isFinite(value)) return "—";
  const rounded = Math.abs(value) >= 100 ? value.toFixed(0) : value.toFixed(2);
  return unit ? `${rounded} ${unit}` : rounded;
}

// Self-describing metric summaries (name/unit/group drive rendering; no metric
// enums). Grouped by `group`. Reused by run detail + public share.
export function MetricsPanel({ metrics }: { metrics: MetricSummary[] }) {
  const groups = useMemo(() => {
    const map = new Map<string, MetricSummary[]>();
    for (const metric of metrics) {
      const key = metric.group || "General";
      map.set(key, [...(map.get(key) ?? []), metric]);
    }
    return Array.from(map.entries()).sort(([a], [b]) => a.localeCompare(b));
  }, [metrics]);

  if (metrics.length === 0) {
    return <p className="p-4 text-sm text-muted-foreground">No metrics yet.</p>;
  }

  return (
    <div className="space-y-6">
      {groups.map(([group, rows]) => (
        <div key={group} className="space-y-2">
          <div className="flex items-center gap-3">
            <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{group}</h3>
            <div className="h-px flex-1 bg-border" />
          </div>
          <div className="overflow-hidden rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Metric</TableHead>
                  <TableHead>Avg</TableHead>
                  <TableHead>Min</TableHead>
                  <TableHead>Max</TableHead>
                  <TableHead>Last</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((metric) => (
                  <TableRow key={metric.key || metric.name}>
                    <TableCell>
                      <div className="font-medium">{metric.name}</div>
                      {metric.description ? <div className="text-xs text-muted-foreground">{metric.description}</div> : null}
                    </TableCell>
                    <TableCell className="font-mono text-xs">{formatValue(metric.avg, metric.unit)}</TableCell>
                    <TableCell className="font-mono text-xs">{formatValue(metric.min, metric.unit)}</TableCell>
                    <TableCell className="font-mono text-xs">{formatValue(metric.max, metric.unit)}</TableCell>
                    <TableCell className="font-mono text-xs">{formatValue(metric.last, metric.unit)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </div>
      ))}
    </div>
  );
}
