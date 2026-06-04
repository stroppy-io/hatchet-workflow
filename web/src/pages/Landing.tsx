import { useEffect, useMemo, useState } from "react";
import type { CSSProperties, ReactNode } from "react";
import { Link } from "react-router-dom";
import { Activity, ArrowRight, Cpu, GitBranch, Layers, RefreshCw } from "lucide-react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { DB_COLORS } from "@/lib/db-colors";
import type { DatabaseKind } from "@/services/types";
import { getPublicRating, type RatingEntryVM } from "@/services/rating";
import { getPublicConfig } from "@/services/publicConfig";

// Public marketing surface served at "/" for unauthenticated visitors. It
// reuses the app's design tokens (Tailwind v4 @theme), components, and the
// dash-reveal / hover-lift motion classes so it reads as the same product, not
// a bolted-on page. The proof block pulls the real public leaderboard
// (PublicRatingService) — no mock data.

// Real, ranked metric keys (see internal/domain/metrics/queries.go). Each is
// higher-is-better except p99 latency, where the board itself flips direction.
const METRICS: { key: string; label: string; blurb: string }[] = [
  { key: "db_tps", label: "Throughput", blurb: "transactions per second" },
  { key: "db_qps", label: "Queries", blurb: "queries per second" },
  { key: "db_latency_p99", label: "Latency p99", blurb: "lower is faster" },
];

// dbKindLabelFromJson emits underscore labels ("ydb_managed"); DB_COLORS keys
// use the hyphen form ("ydb-managed"). Normalize before lookup.
function colorFor(dbKind: string) {
  const key = dbKind.replace(/_/g, "-") as DatabaseKind;
  return DB_COLORS[key];
}

const ENGINES: { kind: DatabaseKind; label: string }[] = [
  { kind: "postgres", label: "PostgreSQL" },
  { kind: "mysql", label: "MySQL" },
  { kind: "mariadb", label: "MariaDB" },
  { kind: "ydb", label: "YDB" },
  { kind: "ydb-managed", label: "YDB Managed" },
  { kind: "cockroach", label: "CockroachDB" },
  { kind: "picodata", label: "Picodata" },
];

export function Landing() {
  const [metricKey, setMetricKey] = useState(METRICS[0].key);
  const [rows, setRows] = useState<RatingEntryVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // Self-signup gate: hide the "Get access" CTAs when the instance disabled
  // open registration. Default visible until the public config says otherwise.
  const [allowRegister, setAllowRegister] = useState(true);

  useEffect(() => {
    let cancelled = false;
    getPublicConfig()
      .then((cfg) => {
        if (!cancelled) setAllowRegister(cfg.allowSelfRegistration);
      })
      .catch(() => {
        /* config unavailable — leave CTAs visible, Register still gates */
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    getPublicRating({ metricKey, limit: 10 })
      .then((page) => {
        if (!cancelled) setRows(page.entries);
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [metricKey]);

  const activeMetric = useMemo(
    () => METRICS.find((m) => m.key === metricKey) ?? METRICS[0],
    [metricKey],
  );

  return (
    <div className="min-h-[100dvh] bg-background text-foreground">
      {/* Nav — single line, brand left, auth right. */}
      <header className="sticky top-0 z-40 border-b border-border bg-background/85 backdrop-blur">
        <nav className="mx-auto flex h-14 max-w-[1200px] items-center justify-between px-4 md:px-6">
          <div className="flex items-center gap-2">
            <Activity className="h-5 w-5 text-primary" />
            <span className="text-base font-semibold tracking-tight">stroppy-cloud</span>
          </div>
          <div className="flex items-center gap-2">
            <Button asChild variant="ghost" size="sm">
              <Link to="/login">Sign in</Link>
            </Button>
            <Button asChild size="sm">
              <Link to="/register">
                {allowRegister ? "Get access" : "Request access"}
              </Link>
            </Button>
          </div>
        </nav>
      </header>

      {/* Hero A — value prop + primary CTA. */}
      <section className="mx-auto flex min-h-[calc(100dvh-3.5rem)] max-w-[1200px] flex-col justify-center px-4 py-20 md:px-6">
        <div className="max-w-3xl">
          <div
            className="reveal-item font-mono text-[11px] uppercase tracking-[0.22em] text-muted-foreground"
            style={{ "--reveal-delay": "0ms" } as CSSProperties}
          >
            Distributed database benchmarking
          </div>
          <h1
            className="reveal-item mt-5 text-4xl font-semibold leading-[1.05] tracking-tight md:text-6xl"
            style={{ "--reveal-delay": "60ms" } as CSSProperties}
          >
            Benchmark every database
            <br />
            on the same yardstick.
          </h1>
          <p
            className="reveal-item mt-6 max-w-xl text-base leading-relaxed text-muted-foreground md:text-lg"
            style={{ "--reveal-delay": "120ms" } as CSSProperties}
          >
            Provision a cluster, run a reproducible stroppy workload, and compare TPC results
            across engines. Every number below is a real public run.
          </p>
          <div
            className="reveal-item mt-9 flex flex-wrap items-center gap-3"
            style={{ "--reveal-delay": "180ms" } as CSSProperties}
          >
            {allowRegister ? (
              <>
                <Button asChild size="lg">
                  <Link to="/register">
                    Get access <ArrowRight className="h-4 w-4" />
                  </Link>
                </Button>
                <Button asChild variant="outline" size="lg">
                  <Link to="/login">Sign in</Link>
                </Button>
              </>
            ) : (
              <>
                <Button asChild size="lg">
                  <Link to="/register">
                    Request access <ArrowRight className="h-4 w-4" />
                  </Link>
                </Button>
                <Button asChild variant="outline" size="lg">
                  <Link to="/login">Sign in</Link>
                </Button>
              </>
            )}
          </div>

          <div
            className="reveal-item mt-16 grid max-w-2xl grid-cols-1 gap-px overflow-hidden border border-border sm:grid-cols-3"
            style={{ "--reveal-delay": "240ms" } as CSSProperties}
          >
            <Capability icon={<Layers className="h-4 w-4" />} title="One run, many engines">
              TPC-C, TPC-B and TPC-H across seven engines.
            </Capability>
            <Capability icon={<Cpu className="h-4 w-4" />} title="Real topologies">
              Replicas, proxies and managed clusters, provisioned for you.
            </Capability>
            <Capability icon={<GitBranch className="h-4 w-4" />} title="Reproducible">
              Pinned stroppy version, captured config, shareable results.
            </Capability>
          </div>
        </div>
      </section>

      {/* Supported engines — color variation, brand recognition. */}
      <section className="border-t border-border">
        <div className="mx-auto flex max-w-[1200px] flex-wrap items-center gap-2 px-4 py-6 md:px-6">
          <span className="mr-2 font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
            Engines
          </span>
          {ENGINES.map((e) => {
            const c = colorFor(e.kind);
            return (
              <span
                key={e.kind}
                className={cn("border px-2.5 py-1 text-xs font-medium", c?.accent)}
                style={c ? { color: c.hexLight } : undefined}
              >
                {e.label}
              </span>
            );
          })}
        </div>
      </section>

      {/* Hero B — live public leaderboard, the proof. */}
      <section className="border-t border-border">
        <div className="mx-auto max-w-[1200px] px-4 py-16 md:px-6">
          <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
            <div>
              <h2 className="text-2xl font-semibold tracking-tight md:text-3xl">Public results</h2>
              <p className="mt-2 max-w-md text-sm text-muted-foreground">
                Top ranked runs by {activeMetric.label.toLowerCase()} ({activeMetric.blurb}). No
                account needed to read the board.
              </p>
            </div>
            <div className="flex border border-border">
              {METRICS.map((m) => (
                <button
                  key={m.key}
                  onClick={() => setMetricKey(m.key)}
                  className={cn(
                    "px-3 py-1.5 text-sm transition-colors",
                    metricKey === m.key
                      ? "bg-primary/15 text-primary"
                      : "text-muted-foreground hover:bg-muted/40",
                  )}
                >
                  {m.label}
                </button>
              ))}
            </div>
          </div>

          {error && (
            <div className="mt-6 border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {error}
            </div>
          )}

          <div className="mt-6 overflow-x-auto border border-border">
            <table className="w-full min-w-[820px] border-collapse text-sm">
              <thead className="bg-muted/60 text-xs uppercase text-muted-foreground">
                <tr className="border-b border-border">
                  <Th className="text-right">#</Th>
                  <Th>Database</Th>
                  <Th>Workload</Th>
                  <Th className="text-right">{activeMetric.label}</Th>
                  <Th>Topology</Th>
                  <Th className="text-right">Nodes</Th>
                  <Th>Provider</Th>
                </tr>
              </thead>
              <tbody>
                {loading &&
                  rows.length === 0 &&
                  Array.from({ length: 6 }).map((_, i) => (
                    <tr key={`sk-${i}`} className="border-b border-border/70">
                      {Array.from({ length: 7 }).map((__, j) => (
                        <td key={j} className="px-3 py-3">
                          <div className="h-3 w-full max-w-[120px] animate-pulse bg-muted" />
                        </td>
                      ))}
                    </tr>
                  ))}

                {!loading &&
                  rows.map((e, i) => {
                    const c = colorFor(e.dbKind);
                    return (
                      <tr
                        key={`${e.rank}-${i}`}
                        className="border-b border-border/70 hover:bg-muted/30"
                      >
                        <Td className="text-right font-semibold tabular-nums">{e.rank}</Td>
                        <Td>
                          <span
                            className={cn(
                              "inline-block border px-2 py-0.5 text-xs font-medium",
                              c?.accent,
                            )}
                            style={c ? { color: c.hexLight } : undefined}
                          >
                            {e.dbKind || "unknown"}
                          </span>
                        </Td>
                        <Td>{e.workloadName || "—"}</Td>
                        <Td className="text-right font-mono tabular-nums">
                          {fmtNum(e.metricValue)}{" "}
                          <span className="text-[11px] text-muted-foreground">{e.metricUnit}</span>
                        </Td>
                        <Td className="text-muted-foreground">{e.topologyLabel || "—"}</Td>
                        <Td className="text-right tabular-nums">{e.nodeCount || "—"}</Td>
                        <Td className="text-muted-foreground">{e.provider || "—"}</Td>
                      </tr>
                    );
                  })}

                {!loading && rows.length === 0 && !error && (
                  <tr>
                    <td colSpan={7} className="px-4 py-12 text-center text-sm text-muted-foreground">
                      No public runs ranked for this metric yet.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          <div className="mt-3 flex items-center gap-1.5 text-xs text-muted-foreground">
            <RefreshCw className={cn("h-3 w-3", loading && "animate-spin")} />
            Live from the public rating service.
          </div>
        </div>
      </section>

      {/* Footer — thin. */}
      <footer className="border-t border-border">
        <div className="mx-auto flex max-w-[1200px] flex-col gap-3 px-4 py-8 text-sm text-muted-foreground md:flex-row md:items-center md:justify-between md:px-6">
          <div className="flex items-center gap-2">
            <Activity className="h-4 w-4 text-primary" />
            <span className="font-medium text-foreground">stroppy-cloud</span>
          </div>
          <div className="flex items-center gap-5">
            <Link to="/login" className="hover:text-foreground">
              Sign in
            </Link>
            <Link to="/register" className="hover:text-foreground">
              {allowRegister ? "Get access" : "Request access"}
            </Link>
          </div>
        </div>
      </footer>
    </div>
  );
}

function Capability({
  icon,
  title,
  children,
}: {
  icon: ReactNode;
  title: string;
  children: ReactNode;
}) {
  return (
    <div className="hover-lift bg-card p-4">
      <div className="flex items-center gap-2 text-primary">{icon}</div>
      <div className="mt-3 text-sm font-medium text-foreground">{title}</div>
      <p className="mt-1 text-xs leading-relaxed text-muted-foreground">{children}</p>
    </div>
  );
}

function Th({ children, className }: { children: ReactNode; className?: string }) {
  return <th className={cn("px-3 py-2 text-left font-medium", className)}>{children}</th>;
}
function Td({ children, className }: { children: ReactNode; className?: string }) {
  return <td className={cn("px-3 py-2 align-middle", className)}>{children}</td>;
}
function fmtNum(v: number): string {
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(v);
}
