import { Bar, BarChart, CartesianGrid, Tooltip, XAxis, YAxis } from "recharts";
import {
  ChartContainer,
  ChartLegend,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import type { DashboardVM } from "@/services/dashboard";

// The one Organization Dashboard chart. Series colors are project CSS tokens
// (no new palette). Self-guarding: renders nothing if its series is absent so a
// series-less provider degrades gracefully.

// On-token series colors (resolve at paint to the @theme values in index.css).
const C = {
  success: "var(--color-success)",
  destructive: "var(--color-destructive)",
} as const;

const AXIS_TICK = { fontSize: 10, fontFamily: "var(--font-mono)" } as const;

function shortDay(iso: string): string {
  // iso is YYYY-MM-DD; show MM-DD.
  return iso.slice(5);
}

/**
 * Recent runs over time — DERIVED from DashboardVM.recentRuns (proto
 * recent_runs), bucketed by day and split succeeded vs failed. Rendered as a
 * STACKED bar chart (succeeded + failed per day) so sparse day-buckets read
 * clearly instead of overlapping area blobs. Not a proto time-series; see
 * services/dashboard.ts bucketRunsByDay.
 */
export function RunsOverTimeChart({ data }: { data: DashboardVM }) {
  const series = data.runsOverTime;
  if (!series || series.length === 0) return null;

  const config: ChartConfig = {
    succeeded: { label: "Succeeded", color: C.success },
    failed: { label: "Failed", color: C.destructive },
  };

  return (
    <div className="flex h-full flex-col">
      <ChartContainer config={config} className="h-[200px] flex-1">
        <BarChart data={series} margin={{ left: -16, right: 4, top: 4 }}>
          <CartesianGrid vertical={false} strokeDasharray="2 4" />
          <XAxis
            dataKey="date"
            tickLine={false}
            axisLine={false}
            tickMargin={8}
            tick={AXIS_TICK}
            tickFormatter={shortDay}
            minTickGap={12}
          />
          <YAxis
            tickLine={false}
            axisLine={false}
            width={36}
            tick={AXIS_TICK}
            allowDecimals={false}
          />
          <Tooltip
            cursor={{ fill: "var(--color-muted)", opacity: 0.4 }}
            content={
              <ChartTooltipContent labelFormatter={(l) => shortDay(String(l))} />
            }
          />
          <Bar
            dataKey="succeeded"
            stackId="runs"
            fill={C.success}
            radius={[0, 0, 2, 2]}
            maxBarSize={40}
            isAnimationActive
            animationDuration={650}
          />
          <Bar
            dataKey="failed"
            stackId="runs"
            fill={C.destructive}
            radius={[2, 2, 0, 0]}
            maxBarSize={40}
            isAnimationActive
            animationDuration={650}
          />
        </BarChart>
      </ChartContainer>
      <ChartLegend config={config} className="justify-start pt-3" />
    </div>
  );
}
