import { useEffect, useState } from "react";
import {
  Loader2,
  Clock,
  CalendarClock,
  Trophy,
} from "lucide-react";
import { Link, useTenantSlug } from "@/lib/router";
import { useAuth } from "@/hooks/useAuth";
import { Badge } from "@/components/ui/badge";
import {
  getDashboardProvider,
  type DashboardVM,
  type RunStatus,
} from "@/services/dashboard";
import { Panel, SectionLabel } from "@/pages/dashboard/Panel";
import { CountUp, Reveal } from "@/pages/dashboard/motion";
import { RunsOverTimeChart } from "@/pages/dashboard/charts";

// Organization Dashboard — the tenant landing page (/t/:slug index).
// Calls the TenantDashboardService surface (via the provider, mock for now)
// and lays the payload out in the existing dark card design, with charts,
// staggered entrance motion, and stat count-ups.

const STATUS_BADGE: Record<
  RunStatus,
  "default" | "success" | "destructive" | "secondary" | "pending" | "warning"
> = {
  pending: "pending",
  running: "default",
  cancelling: "warning",
  completed: "success",
  failed: "destructive",
  cancelled: "secondary",
};

function relTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const abs = Math.abs(diff);
  const m = Math.round(abs / 60_000);
  const h = Math.round(abs / 3_600_000);
  const d = Math.round(abs / 86_400_000);
  const s = d >= 1 ? `${d}d` : h >= 1 ? `${h}h` : `${Math.max(m, 1)}m`;
  return diff >= 0 ? `${s} ago` : `in ${s}`;
}

function StatCell({
  label,
  value,
  tone = "text-foreground",
}: {
  label: string;
  value: React.ReactNode;
  tone?: string;
}) {
  return (
    <div className="flex flex-col gap-1 border border-border/70 bg-background/40 px-3 py-2.5">
      <span className="text-[10px] font-mono uppercase tracking-wider text-zinc-500">
        {label}
      </span>
      <span className={`text-lg font-semibold tabular-nums ${tone}`}>
        {value}
      </span>
    </div>
  );
}

/**
 * MergedNumbers — the single combined stat block. Total is prominent, with the
 * status breakdown + success rate as a tidy inline grid inside the same panel.
 * All fields come from runCounts + successRate (real proto, no derivation).
 */
function MergedNumbers({ data }: { data: DashboardVM }) {
  const { runCounts, successRate } = data;
  return (
    <Panel
      label="Run Totals"
      className="h-full hover-lift"
      bodyClassName="flex flex-col gap-5 p-5"
    >
      <div className="flex items-baseline gap-3">
        <span className="text-5xl font-semibold tabular-nums text-foreground">
          <CountUp value={runCounts.total} />
        </span>
        <span className="text-[10px] font-mono uppercase tracking-wider text-zinc-500">
          total runs
        </span>
      </div>
      <div className="grid grid-cols-2 gap-2.5 sm:grid-cols-3">
        <StatCell
          label="Succeeded"
          tone="text-success"
          value={<CountUp value={runCounts.completed} />}
        />
        <StatCell
          label="Running"
          tone="text-primary"
          value={<CountUp value={runCounts.running} />}
        />
        <StatCell
          label="Failed"
          tone="text-destructive"
          value={<CountUp value={runCounts.failed} />}
        />
        <StatCell
          label="Cancelled"
          tone="text-muted-foreground"
          value={<CountUp value={runCounts.cancelled} />}
        />
        <StatCell
          label="Pending"
          tone="text-muted-foreground"
          value={<CountUp value={runCounts.pending} />}
        />
        <StatCell
          label="Success Rate"
          tone="text-success"
          value={
            <CountUp value={successRate * 100} decimals={1} suffix="%" />
          }
        />
      </div>
    </Panel>
  );
}

export function Dashboard() {
  const slug = useTenantSlug();
  const { user } = useAuth();
  const tenant = user?.tenants.find((t) => t.slug === slug);

  const [data, setData] = useState<DashboardVM | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!slug) return;
    let cancelled = false;
    setLoading(true);
    setError(null);
    getDashboardProvider()
      .getDashboard(slug)
      .then((d) => {
        if (!cancelled) setData(d);
      })
      .catch((e: unknown) => {
        if (!cancelled)
          setError(e instanceof Error ? e.message : "failed to load dashboard");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [slug]);

  // The runs-over-time chart is derived from recentRuns; hide it when there are
  // no recent runs to bucket.
  const hasRunsSeries = !!data?.runsOverTime?.length;

  return (
    <div className="p-8">
      <div className="mb-7">
        <SectionLabel className="mb-2">Organization</SectionLabel>
        <h1 className="text-xl font-semibold tracking-tight text-foreground">
          {tenant?.name ?? slug}
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Benchmark activity overview for{" "}
          <span className="font-mono text-xs">{slug}</span>.
        </p>
      </div>

      {loading && (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" />
          Loading dashboard…
        </div>
      )}

      {!loading && error && (
        <div className="border border-border bg-card/60 p-6 text-sm text-destructive">
          {error}
        </div>
      )}

      {!loading && !error && data && (
        <div className="space-y-7">
          {/* ROW 1 — merged numbers block | runs-over-time chart */}
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <Reveal index={0} className="h-full">
              <MergedNumbers data={data} />
            </Reveal>
            <Reveal index={1} className="h-full">
              <Panel
                label="Runs Over Time"
                className="h-full hover-lift"
              >
                {hasRunsSeries ? (
                  <RunsOverTimeChart data={data} />
                ) : (
                  <div className="flex h-[200px] items-center justify-center text-sm text-muted-foreground">
                    No recent runs to chart.
                  </div>
                )}
              </Panel>
            </Reveal>
          </div>

          {/* ROW 2 — Top Benchmarks (featured, internal scroll) */}
          <Reveal index={2}>
            <Panel
              label={
                <span className="flex items-center gap-1.5">
                  <Trophy className="h-3 w-3 text-warning" />
                  Top Benchmarks
                </span>
              }
              bodyClassName="max-h-[22rem] overflow-y-auto"
              className="border-warning/30 shadow-md ring-1 ring-warning/10"
            >
              {data.topBenchmarks.length === 0 ? (
                <div className="p-6 text-sm text-muted-foreground">
                  No ranked benchmarks yet.
                </div>
              ) : (
                data.topBenchmarks.map((b) => (
                  <div
                    key={b.rank}
                    className="flex items-center justify-between border-b border-border/70 px-4 py-3.5 last:border-b-0"
                  >
                    <div className="flex items-center gap-3">
                      <span className="font-mono text-sm font-semibold tabular-nums text-warning">
                        #{b.rank}
                      </span>
                      <div className="flex flex-col">
                        <span className="text-sm text-foreground">
                          {b.workload} · {b.dbKind}
                        </span>
                        <span className="mt-0.5 font-mono text-[10px] text-muted-foreground">
                          {b.topology} · {b.author}
                        </span>
                      </div>
                    </div>
                    <span className="font-mono text-sm tabular-nums text-foreground">
                      {b.metricValue.toLocaleString()}{" "}
                      <span className="text-muted-foreground">
                        {b.metricUnit}
                      </span>
                    </span>
                  </div>
                ))
              )}
            </Panel>
          </Reveal>

          {/* ROW 3 — Recent Runs | Scheduled */}
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <Reveal index={3} className="h-full">
              <Panel label="Recent Runs" bodyClassName="" className="h-full">
                {data.recentRuns.length === 0 ? (
                  <div className="p-6 text-sm text-muted-foreground">
                    No runs yet.
                  </div>
                ) : (
                  data.recentRuns.map((r) => (
                    <Link
                      key={r.id}
                      to={`/runs/${r.id}`}
                      className="flex items-center justify-between border-b border-border/70 px-4 py-3 transition-colors last:border-b-0 hover:bg-muted/50"
                    >
                      <div className="flex min-w-0 flex-col">
                        <span className="truncate text-sm text-foreground">
                          {r.name}
                        </span>
                        <span className="mt-0.5 font-mono text-[10px] text-muted-foreground">
                          {r.dbKind} · {r.workload} · {relTime(r.startedAt)}
                        </span>
                      </div>
                      <Badge variant={STATUS_BADGE[r.status]}>{r.status}</Badge>
                    </Link>
                  ))
                )}
              </Panel>
            </Reveal>

            <Reveal index={4} className="h-full">
              <Panel label="Scheduled" bodyClassName="" className="h-full">
                {data.upcoming.length === 0 ? (
                  <div className="p-6 text-sm text-muted-foreground">
                    Nothing scheduled.
                  </div>
                ) : (
                  data.upcoming.map((s) => (
                    <div
                      key={s.suiteId}
                      className="flex items-start gap-2.5 border-b border-border/70 px-4 py-3 last:border-b-0"
                    >
                      <CalendarClock className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                      <div className="flex min-w-0 flex-col">
                        <span className="truncate text-sm text-foreground">
                          {s.name}
                        </span>
                        <span className="mt-0.5 flex items-center gap-1.5 font-mono text-[10px] text-muted-foreground">
                          <Clock className="h-3 w-3" />
                          {s.cron} · next {relTime(s.nextRunAt)}
                        </span>
                      </div>
                    </div>
                  ))
                )}
              </Panel>
            </Reveal>
          </div>
        </div>
      )}
    </div>
  );
}
