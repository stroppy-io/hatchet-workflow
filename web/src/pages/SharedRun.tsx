// Public shared-run page: a benchmark result handed to someone with no account.
//
// It reads as a lab report, in the order a stranger actually needs it: what the
// run WAS (the signature line), the conditions it ran under (configuration),
// the numbers it produced (metrics), and finally the charts to dig into. The
// configuration therefore sits ABOVE the metrics — it is what makes them mean
// anything.
import { useEffect, useMemo, useState } from "react";
import { useParams } from "react-router-dom";
import { BarChart3, Check, Copy, Database, Gauge, LineChart } from "lucide-react";

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

function fmtTime(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return new Intl.DateTimeFormat(undefined, { dateStyle: "short", timeStyle: "short" }).format(d);
}

function fmtElapsed(startedAt?: string, finishedAt?: string): string {
  if (!startedAt || !finishedAt) return "—";
  const ms = new Date(finishedAt).getTime() - new Date(startedAt).getTime();
  if (!Number.isFinite(ms) || ms <= 0) return "—";
  const sec = Math.round(ms / 1000);
  if (sec < 60) return `${sec}s`;
  if (sec < 3600) return `${Math.floor(sec / 60)}m ${sec % 60}s`;
  return `${Math.floor(sec / 3600)}h ${Math.floor((sec % 3600) / 60)}m`;
}

/** The knobs of one segment, in the order someone reads them: what, how hard, how written. */
function segmentFacts(s: SharedSegmentVM): string[] {
  return [
    s.script,
    s.vus !== undefined && `vus=${s.vus}`,
    s.duration ? `dur=${s.duration}` : s.iterations !== undefined && `iters=${s.iterations}`,
    s.poolSize !== undefined && `pool=${s.poolSize}`,
    s.scaleFactor !== undefined && `scale=${s.scaleFactor}`,
    s.insertMethod && `insert=${s.insertMethod}`,
    s.bulkSize ? `bulk=${s.bulkSize}` : false,
    s.quiet && "quiet",
    s.noThresholds && "no-thresholds",
  ].filter(Boolean) as string[];
}

/**
 * The run signature: one line that identifies this benchmark well enough to
 * paste into an issue and compare against another. Engine and sizing, then the
 * measured segment — the bootstrap segment is setup, not the measurement, so
 * the last segment is the one that produced the numbers.
 */
function runSignature(run: SharedRunVM): string {
  const engine = [run.dbKind, run.database?.version].filter(Boolean).join(" ");
  const sizing = (run.database?.settings ?? []).map((s) => `${s.key}=${s.value}`).join(" ");
  const measured = run.workloadSegments[run.workloadSegments.length - 1];
  const load = measured ? segmentFacts(measured).join(" ") : "";
  return [engine, run.nodeCount ? `nodes=${run.nodeCount}` : "", sizing, load]
    .filter(Boolean)
    .join(" · ");
}

/** The signature, copyable. Copying it is the point — it is how runs get compared. */
function Signature({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  if (!text) return null;

  const copy = () => {
    void navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    });
  };

  return (
    <div className="group flex items-start gap-2 border-l-2 border-primary/60 bg-muted/20 py-2 pl-3 pr-2">
      <code className="min-w-0 flex-1 break-words font-mono text-[12px] leading-relaxed text-foreground">
        {text}
      </code>
      <button
        type="button"
        onClick={copy}
        aria-label="Copy run signature"
        className="shrink-0 rounded p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
      >
        {copied ? <Check className="h-3.5 w-3.5 text-success" /> : <Copy className="h-3.5 w-3.5" />}
      </button>
    </div>
  );
}

/** Provenance, not parameters: where and when it ran. One line, no card grid. */
function FactStrip({ run }: { run: SharedRunVM }) {
  const facts: Array<[string, string]> = [
    ["provider", run.provider || "—"],
    ["topology", run.topologyLabel || "—"],
    ["stroppy", run.stroppyVersion || "—"],
    ["started", fmtTime(run.startedAt)],
    ["elapsed", fmtElapsed(run.startedAt, run.finishedAt)],
  ];
  if (run.progressPct < 100) facts.push(["progress", `${run.progressPct}%`]);

  return (
    <dl className="flex flex-wrap items-baseline gap-x-6 gap-y-2 font-mono text-[11px]">
      {facts.map(([label, value]) => (
        <div key={label} className="flex items-baseline gap-1.5">
          <dt className="uppercase tracking-wider text-muted-foreground">{label}</dt>
          <dd className="text-foreground">{value}</dd>
        </div>
      ))}
    </dl>
  );
}

/**
 * The launch knobs behind the numbers. Everything here is a typed field the
 * server explicitly allowlisted — the run's env, inline SQL, workload files,
 * extra CLI args and free-form engine option maps are never projected into a
 * share.
 */
function SharedConfig({ run }: { run: SharedRunVM }) {
  const segments = run.workloadSegments;
  const db = run.database;
  if (segments.length === 0 && !db) return null;

  return (
    <section className="grid gap-px overflow-hidden border border-border bg-border md:grid-cols-2">
      {segments.length > 0 && (
        <div className="bg-background">
          <SectionHead icon={<Gauge className="h-3.5 w-3.5" />} title="Workload" />
          <div className="flex flex-col gap-2 p-3">
            {segments.map((s, i) => (
              <div key={i} className="flex flex-col gap-1">
                <div className="font-mono text-xs text-foreground">{s.name || `segment ${i + 1}`}</div>
                <div className="flex flex-wrap gap-1">
                  {segmentFacts(s).map((f) => (
                    <span key={f} className="rounded-sm bg-muted px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground">
                      {f}
                    </span>
                  ))}
                </div>
                {s.steps.length > 0 && (
                  <div className="font-mono text-[11px] text-muted-foreground/70">steps: {s.steps.join(" → ")}</div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {db && (
        <div className="bg-background">
          <SectionHead icon={<Database className="h-3.5 w-3.5" />} title={`Database${db.version ? ` · ${db.version}` : ""}`} />
          <div className="p-3 font-mono text-[11px]">
            {db.settings.length === 0 ? (
              <span className="text-muted-foreground">Defaults, unchanged.</span>
            ) : (
              db.settings.map((s) => (
                <div key={s.key} className="flex gap-2 py-px">
                  <span className="w-48 shrink-0 text-muted-foreground">{s.key}</span>
                  <span className="break-all text-foreground">{s.value}</span>
                </div>
              ))
            )}
          </div>
        </div>
      )}
    </section>
  );
}

function SectionHead({ icon, title, extra }: { icon: React.ReactNode; title: string; extra?: React.ReactNode }) {
  return (
    <div className="flex items-center gap-2 border-b border-border bg-muted/20 px-3 py-2 text-primary">
      {icon}
      <h2 className="text-[11px] font-semibold uppercase tracking-wider text-foreground">{title}</h2>
      {extra}
    </div>
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
      <SectionHead
        icon={<BarChart3 className="h-3.5 w-3.5" />}
        title="Dashboards"
        extra={
          <div className="ml-auto flex gap-1">
            {dashboards.map((name) => (
              <button
                key={name}
                type="button"
                onClick={() => setActive(name)}
                className={cn(
                  "rounded px-2 py-1 font-mono text-[11px] uppercase tracking-wider transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring",
                  name === active
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                {DASHBOARD_LABEL[name] ?? name}
              </button>
            ))}
          </div>
        }
      />
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

  const signature = useMemo(() => (run ? runSignature(run) : ""), [run]);
  const isTestRun = run?.kind === "test_run";

  return (
    <div className="min-h-screen bg-background text-foreground">
      {/* Full-bleed: the embedded dashboards are the point of this page, and a
          centred column wastes the horizontal room their panels need. */}
      <div className="flex w-full flex-col gap-6 px-6 py-6">
        {loading && <div className="text-sm text-muted-foreground">Loading the shared run…</div>}

        {error && (
          <div className="border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            This share is unavailable: {error}
          </div>
        )}

        {!loading && !error && !run && (
          <div className="text-sm text-muted-foreground">
            This link no longer works. The share was revoked, expired, or never existed.
          </div>
        )}

        {run && (
          <div className="flex flex-col gap-6">
            <header className="flex flex-col gap-3 border-b border-border pb-5">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="flex min-w-0 items-center gap-3">
                  <h1 className="truncate text-xl font-semibold tracking-tight">{run.name || "Unnamed run"}</h1>
                  <Badge>{run.status}</Badge>
                  {run.dbKind && (
                    <span className="font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
                      {run.dbKind}
                      {run.workloadName ? ` · ${run.workloadName}` : ""}
                    </span>
                  )}
                </div>
                {run.capturedAt && (
                  <div className="font-mono text-[11px] text-muted-foreground">
                    Snapshot taken {fmtTime(run.capturedAt)}
                  </div>
                )}
              </div>

              <FactStrip run={run} />
              {isTestRun && <Signature text={signature} />}
            </header>

            {isTestRun && <SharedConfig run={run} />}

            <section className="overflow-hidden border border-border bg-background">
              <SectionHead
                icon={<LineChart className="h-3.5 w-3.5" />}
                title="Metrics"
                extra={<span className="font-mono text-[11px] text-muted-foreground">{run.metrics.length}</span>}
              />
              <div className="p-3">
                <MetricsPanel metrics={run.metrics} />
              </div>
            </section>

            {session && isTestRun && <SharedGrafana session={session} dbKind={run.dbKind} />}
          </div>
        )}
      </div>
    </div>
  );
}
