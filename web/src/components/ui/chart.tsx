import * as React from "react";
import { ResponsiveContainer } from "recharts";
import type {
  NameType,
  Payload,
  ValueType,
} from "recharts/types/component/DefaultTooltipContent";
import { cn } from "@/lib/utils";

// Lean chart primitive themed to the existing dark @theme tokens (index.css).
// Intentionally NOT the stock shadcn chart.tsx: that one wires Tailwind-v3
// `hsl(var(--chart-N))` tokens this project doesn't define. Here series colors
// are passed explicitly via the project's own CSS variables (--color-primary,
// --color-success, --color-destructive, --color-muted-foreground, …).

/** Per-series display metadata: a human label and a CSS color value. */
export interface ChartSeries {
  label: string;
  /** Any CSS color — pass `var(--color-primary)` etc. to stay on-token. */
  color: string;
}

export type ChartConfig = Record<string, ChartSeries>;

const ChartConfigContext = React.createContext<ChartConfig>({});

function useChartConfig(): ChartConfig {
  return React.useContext(ChartConfigContext);
}

/**
 * Responsive, fixed-height chart frame. Exposes the series config to the
 * themed tooltip via context and injects `--chart-<key>` CSS vars so children
 * (recharts shapes) can reference `var(--chart-<key>)` for fills/strokes.
 */
export function ChartContainer({
  config,
  className,
  children,
}: {
  config: ChartConfig;
  className?: string;
  children: React.ReactElement;
}) {
  const cssVars = React.useMemo(() => {
    const vars: Record<string, string> = {};
    for (const [key, series] of Object.entries(config)) {
      vars[`--chart-${key}`] = series.color;
    }
    return vars as React.CSSProperties;
  }, [config]);

  return (
    <ChartConfigContext.Provider value={config}>
      <div
        data-slot="chart"
        style={cssVars}
        className={cn(
          "w-full text-muted-foreground [&_.recharts-cartesian-axis-tick_text]:fill-current [&_.recharts-cartesian-grid_line]:stroke-border/60 [&_.recharts-curve.recharts-tooltip-cursor]:stroke-border [&_.recharts-radial-bar-background-sector]:fill-muted [&_.recharts-rectangle.recharts-tooltip-cursor]:fill-muted/40 [&_.recharts-sector]:outline-none [&_.recharts-surface]:outline-none",
          className,
        )}
      >
        <ResponsiveContainer width="100%" height="100%">
          {children}
        </ResponsiveContainer>
      </div>
    </ChartConfigContext.Provider>
  );
}

interface ChartTooltipContentProps {
  active?: boolean;
  payload?: ReadonlyArray<Payload<ValueType, NameType>>;
  label?: React.ReactNode;
  /** Optional formatter for the displayed value. */
  valueFormatter?: (value: number, key: string) => string;
  /** Optional formatter for the header label. */
  labelFormatter?: (label: React.ReactNode) => React.ReactNode;
  /** Hide the header label row entirely (useful for single-category charts). */
  hideLabel?: boolean;
}

/**
 * Themed tooltip body. Pass as `<ChartTooltip content={<ChartTooltipContent />} />`
 * (recharts injects `active`/`payload`/`label`). Reads labels/colors from the
 * ChartContainer config keyed by `dataKey`.
 */
export function ChartTooltipContent({
  active,
  payload,
  label,
  valueFormatter,
  labelFormatter,
  hideLabel,
}: ChartTooltipContentProps) {
  const config = useChartConfig();

  if (!active || !payload || payload.length === 0) return null;

  return (
    <div className="min-w-[8rem] border border-border bg-card/95 px-2.5 py-2 text-xs shadow-lg backdrop-blur-sm">
      {!hideLabel && label != null && (
        <div className="mb-1.5 font-mono text-[10px] uppercase tracking-wider text-zinc-500">
          {labelFormatter ? labelFormatter(label) : label}
        </div>
      )}
      <div className="flex flex-col gap-1">
        {payload.map((item, i) => {
          const key = String(item.dataKey ?? item.name ?? i);
          const series = config[key];
          const color =
            series?.color ??
            (typeof item.color === "string" ? item.color : "currentColor");
          const raw =
            typeof item.value === "number" ? item.value : Number(item.value);
          const display =
            valueFormatter && Number.isFinite(raw)
              ? valueFormatter(raw, key)
              : Number.isFinite(raw)
                ? raw.toLocaleString()
                : String(item.value ?? "");
          return (
            <div
              key={key}
              className="flex items-center justify-between gap-3 leading-none"
            >
              <span className="flex items-center gap-1.5 text-muted-foreground">
                <span
                  aria-hidden
                  className="h-2 w-2 shrink-0 rounded-[1px]"
                  style={{ backgroundColor: color }}
                />
                {series?.label ?? key}
              </span>
              <span className="font-mono tabular-nums text-foreground">
                {display}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

/**
 * Compact legend. Reads from the nearest ChartContainer config by default, or
 * accepts an explicit `config` when rendered outside the container.
 */
export function ChartLegend({
  className,
  config: configProp,
}: {
  className?: string;
  config?: ChartConfig;
}) {
  const ctxConfig = useChartConfig();
  const config = configProp ?? ctxConfig;
  const entries = Object.entries(config);
  if (entries.length === 0) return null;
  return (
    <div
      className={cn(
        "flex flex-wrap items-center justify-center gap-x-4 gap-y-1 pt-1",
        className,
      )}
    >
      {entries.map(([key, series]) => (
        <span
          key={key}
          className="flex items-center gap-1.5 text-[11px] text-muted-foreground"
        >
          <span
            aria-hidden
            className="h-2 w-2 shrink-0 rounded-[1px]"
            style={{ backgroundColor: series.color }}
          />
          {series.label}
        </span>
      ))}
    </div>
  );
}
