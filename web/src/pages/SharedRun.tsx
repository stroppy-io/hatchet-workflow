import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { Activity, BarChart3, Gauge, LineChart, Settings2 } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { MetricsPanel } from "@/components/run/MetricsPanel";
import { cn } from "@/lib/utils";
import { buildEmbedUrl, DASHBOARD_LABEL, dashboardsForRun } from "@/services/grafana";
import {
  getSharedRun,
  startSharedSession,
  type SharedRunVM,
  type SharedSegmentVM,
  type SharedSessionVM,
} from "@/services/shares";

const isoOrUndefined = (millis?: number): string | undefined =>
  millis ? new Date(millis).toISOString() : undefined;

/**
 * The launch knobs behind the numbers: what ran, how hard, against what shape of
 * database. Everything here is a typed field the server explicitly allowlisted —
 * the run's env, inline SQL, workload files, extra CLI args and free-form engine
 * option maps are never projected into a share.
 */
function SharedConfig({ run }: { run: SharedRunVM }) {
  const segments = run.workloadSegments;
  const db = run.database;
  if (segments.length === 0 && !db) return null;

  const segFacts = (s: SharedSegmentVM) =>
    [
      s.script,
      s.vus !== undefined && `vus=${s.vus}`,
      s.duration ? `dur=${s.duration}` : s.iterations !== undefined && `iters=${s.iterations}`,
      s.insertMethod && `insert=${s.insertMethod}`,
      s.bulkSize ? `bulk=${s.bulkSize}` : false,
      s.poolSize !== undefined && `pool=${s.poolSize}`,
      s.scaleFactor !== undefined && `scale=${s.scaleFactor}`,
      s.steps.length > 0 && `steps=${s.steps.join(",")}`,
      s.noSteps.length > 0 && `no-steps=${s.noSteps.join(",")}`,
      s.quiet && "quiet",
      s.noThresholds && "no-thresholds",
    ]
      .filter(Boolean)
      .join(" · ");

  return (
    <section className="overflow-hidden border border-border bg-background">
      <div className="flex items-center gap-2 border-b border-border bg-muted/20 px-3 py-2">
        <Settings2 className="h-4 w-4 text-primary" />
        <h3 className="text-sm font-semibold">Configuration</h3>
      </div>
      <div className="grid gap-3 p-3 md:grid-cols-2">
        {segments.length > 0 && (
          <div className="flex flex-col gap-2">
            <div className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">Workload</div>
            {segments.map((s, i) => (
              <div key={i} className="rounded border border-border/60 p-2">
                <div className="flex items-center gap-1.5">
                  <Gauge className="h-3.5 w-3.5 text-primary" />
                  <span className="font-mono text-xs font-semibold">{s.name || `segment ${i + 1}`}</span>
                </div>
                <div className="mt-1 break-words font-mono text-[11px] text-muted-foreground">{segFacts(s)}</div>
              </div>
            ))}
          </div>
        )}

        {db && (
          <div className="flex flex-col gap-2">
            <div className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">Database</div>
            <div className="rounded border border-border/60 bg-black/20 p-2 font-mono text-[11px]">
              {db.version && (
                <div className="flex gap-2 py-px">
                  <span className="shrink-0 text-primary/80">version</span>
                  <span className="text-muted-foreground">=</span>
                  <span className="text-foreground">{db.version}</span>
                </div>
              )}
              {db.settings.map((s) => (
                <div key={s.key} className="flex gap-2 py-px">
                  <span className="shrink-0 text-primary/80">{s.key}</span>
                  <span className="text-muted-foreground">=</span>
                  <span className="break-all text-foreground">{s.value}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </section>
  );
}

/**
 * Live Grafana for a share link.
 *
 * The iframes are ordinary dashboards, but the viewer is anonymous and lands in
 * Grafana's `public` organisation, whose only datasource is the gateway's
 * share-scoped proxy. That proxy reads the HttpOnly cookie minted by
 * startSharedSession and pins every query to this run — so editing `var-run_id`
 * (or any dashboard variable) in the iframe URL changes nothing about what the
 * viewer can read.
 */
function SharedGrafana({ session, dbKind }: { session: SharedSessionVM; dbKind: string }) {
  const dashboards = dashboardsForRun(dbKind);
  const [active, setActive] = useState(dashboards[0] ?? "workload");

  const src = buildEmbedUrl(active, {
    runId: session.runId,
    dbKind,
    startedAt: isoOrUndefined(session.from),
    finishedAt: isoOrUndefined(session.to),
  });

  return (
    <section className="overflow-hidden border border-border bg-background">
      <div className="flex items-center gap-2 border-b border-border bg-muted/20 px-3 py-2">
        <BarChart3 className="h-4 w-4 text-primary" />
        <h3 className="text-sm font-semibold">Dashboards</h3>
        <div className="ml-auto flex gap-1">
          {dashboards.map((name) => (
            <button
              key={name}
              type="button"
              onClick={() => setActive(name)}
              className={cn(
                "rounded px-2 py-1 font-mono text-[11px] uppercase tracking-wider transition-colors",
                name === active
                  ? "bg-primary text-primary-foreground"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              {DASHBOARD_LABEL[name] ?? name}
            </button>
          ))}
        </div>
      </div>
      {/* Nearly the whole viewport: a dashboard is many panels tall, so a short
          frame turns into a tiny window scrolled by Grafana's own scrollbar. */}
      <iframe
        key={active}
        src={src}
        title={`${active} dashboard`}
        className="h-[calc(100vh-7rem)] min-h-[640px] w-full border-0"
      />
    </section>
  );
}

export function SharedRun() {
  const { token = "" } = useParams<{ token: string }>();
  const [run, setRun] = useState<SharedRunVM | null>(null);
  const [session, setSession] = useState<SharedSessionVM | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    getSharedRun(token)
      .then((r) => alive && setRun(r ?? null))
      .catch((err) => alive && setError(err instanceof Error ? err.message : String(err)))
      .finally(() => alive && setLoading(false));
    return () => {
      alive = false;
    };
  }, [token]);

  // The snapshot renders without it, so a share whose dashboards are unavailable
  // (monitoring down, suite-run share) degrades to the frozen numbers instead of
  // failing the page.
  useEffect(() => {
    let alive = true;
    startSharedSession(token)
      .then((s) => alive && setSession(s))
      .catch(() => alive && setSession(null));
    return () => {
      alive = false;
    };
  }, [token]);

  return (
    <div className="min-h-screen bg-background text-foreground">
      {/* Full-bleed: the embedded dashboards are the point of this page, and a
          centred column wastes the horizontal room their panels need. */}
      <div className="flex w-full flex-col gap-4 px-6 py-6">
        <div className="flex items-center gap-2 border-b border-border pb-4 text-lg font-semibold">
          <Activity className="h-5 w-5 text-primary" />
          <h1>Shared run</h1>
        </div>

        {loading && <div className="text-sm text-muted-foreground">Loading...</div>}

        {error && (
          <div className="border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            This share is unavailable: {error}
          </div>
        )}

        {!loading && !error && !run && (
          <div className="text-sm text-muted-foreground">Share not found, revoked, or expired.</div>
        )}

        {run && (
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-1 border-b border-border pb-3 md:flex-row md:items-end md:justify-between">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <h2 className="truncate text-base font-semibold">{run.name || "(unnamed run)"}</h2>
                  <Badge>{run.status}</Badge>
                </div>
                <div className="mt-1 text-xs text-muted-foreground">
                  {run.kind === "test_run" ? "Public test run snapshot" : "Public suite snapshot"}
                </div>
              </div>
              {run.capturedAt && (
                <div className="font-mono text-[11px] text-muted-foreground">
                  Captured {fmtTime(run.capturedAt)}
                </div>
              )}
            </div>
            <div className="grid grid-cols-2 gap-2 md:grid-cols-3">
              <Field label="Database" value={run.dbKind || "—"} />
              <Field label="DB name" value={run.dbName || "—"} />
              <Field label="Workload" value={run.workloadName || "—"} />
              <Field label="Stroppy" value={run.stroppyVersion || "—"} />
              <Field label="Provider" value={run.provider || "—"} />
              <Field label="Topology" value={run.topologyLabel || "—"} />
              <Field label="Nodes" value={String(run.nodeCount)} />
              <Field label="Progress" value={`${run.progressPct}%`} />
              <Field label="Started" value={fmtTime(run.startedAt)} />
              <Field label="Finished" value={fmtTime(run.finishedAt)} />
            </div>

            <section className="overflow-hidden border border-border bg-background">
              <div className="flex items-center gap-2 border-b border-border bg-muted/20 px-3 py-2">
                <LineChart className="h-4 w-4 text-primary" />
                <h3 className="text-sm font-semibold">Metrics</h3>
                <span className="font-mono text-[11px] text-muted-foreground">
                  {run.metrics.length}
                </span>
              </div>
              <div className="p-3">
                <MetricsPanel metrics={run.metrics} />
              </div>
            </section>

            {run.kind === "test_run" && <SharedConfig run={run} />}

            {session && run.kind === "test_run" && (
              <SharedGrafana session={session} dbKind={run.dbKind} />
            )}
          </div>
        )}
      </div>
    </div>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div className="border border-border bg-muted/20 px-3 py-2">
      <div className="text-[11px] uppercase text-muted-foreground">{label}</div>
      <div className="mt-1 truncate text-sm font-medium">{value}</div>
    </div>
  );
}
function fmtTime(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return new Intl.DateTimeFormat(undefined, { dateStyle: "short", timeStyle: "short" }).format(d);
}
