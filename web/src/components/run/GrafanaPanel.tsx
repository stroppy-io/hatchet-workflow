// Grafana embed tab. Mirrors the legacy DashboardSelector behaviour against the
// new stack: dashboards are addressed by provisioned UID (services/grafana.ts)
// and proxied same-origin at /grafana. Each (dashboard, machine) pair gets its
// own iframe, kept mounted but hidden so switching is a CSS toggle — never a
// Grafana reload. Machine pills (system dashboard only) come from the run's
// workers.
import { useEffect, useRef, useState } from "react";
import { ExternalLink, Loader2 } from "lucide-react";
import { SegmentedControl } from "@/components/ui/segmented";
import {
  DASHBOARD_LABEL,
  buildEmbedUrl,
  dashboardsForRun,
} from "@/services/grafana";
import { ensureGrafanaSession, type WorkerVM } from "@/services/run_overview";
import { cn } from "@/lib/utils";

interface GrafanaPanelProps {
  tenantSlug: string;
  runId: string;
  dbKind?: string;
  startedAt?: string;
  finishedAt?: string;
  workers: WorkerVM[];
}

// Stable machine id for the system dashboard's `node` var.
function machineId(w: WorkerVM): string {
  return w.machineId || w.host || w.id;
}

export function GrafanaPanel({ tenantSlug, runId, dbKind, startedAt, finishedAt, workers }: GrafanaPanelProps) {
  const dashboards = dashboardsForRun(dbKind);
  const [selected, setSelected] = useState(dashboards[0] ?? "workload");
  const [machine, setMachine] = useState<string>(""); // "" = All

  // The embed queries the gateway's run-scoped datasource, which needs a scope
  // cookie. Mint it BEFORE any iframe mounts, or the first dashboard load races
  // the cookie and every panel 404s. Iframes stay unmounted until it is set.
  const [sessionReady, setSessionReady] = useState(false);
  useEffect(() => {
    let alive = true;
    setSessionReady(false);
    ensureGrafanaSession(tenantSlug, runId)
      .catch(() => {}) // best-effort: still show the dashboards, panels error if unscoped
      .finally(() => alive && setSessionReady(true));
    return () => {
      alive = false;
    };
  }, [tenantSlug, runId]);

  const machineActive = selected === "system";
  const targets = workers
    .map((w) => ({ id: machineId(w), role: w.kind }))
    .filter((t) => t.id);

  // One iframe URL per (dashboard, machine) visited. Non-system dashboards
  // collapse onto a single iframe (machine slot blank).
  const srcsRef = useRef<Map<string, string>>(new Map());
  const [, tick] = useState(0);
  // Keys whose iframe has fired `load` — used to drop the per-pane loader.
  const [loaded, setLoaded] = useState<Set<string>>(new Set());

  const keyFor = (name: string, m: string) => `${name}|${name === "system" ? m : ""}`;
  const activeKey = keyFor(selected, machine);

  // Lazily mount ONLY the dashboard the user is actually looking at; once
  // visited its iframe stays cached (hidden) so re-selecting it is instant.
  // (Pre-warming the whole dashboards×machines product booted a full Grafana
  // app per iframe → hundreds of requests on page open — never do that.)
  useEffect(() => {
    if (!sessionReady) return; // don't race the scope cookie
    const m = srcsRef.current;
    if (!m.has(activeKey)) {
      m.set(activeKey, buildEmbedUrl(selected, { runId, dbKind, machine, startedAt, finishedAt }));
      tick((n) => n + 1);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeKey, runId, sessionReady]);

  // Rebuild every URL when the run's time window changes (run finishes) — the
  // locked from..to is part of the URL.
  useEffect(() => {
    const m = srcsRef.current;
    if (m.size === 0) return;
    for (const k of Array.from(m.keys())) {
      const [name, mc] = k.split("|");
      m.set(k, buildEmbedUrl(name, { runId, dbKind, machine: mc, startedAt, finishedAt }));
    }
    setLoaded(new Set()); // URLs changed → iframes reload → re-arm loaders
    tick((n) => n + 1);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [startedAt, finishedAt]);

  return (
    <div className="flex h-full flex-col">
      {/* Dashboard switcher — same flush tab style as the run view tabs. */}
      <div className="flex h-9 items-center p-1 gap-2">
        <SegmentedControl
          variant="tabs"
          value={selected}
          onChange={setSelected}
          segmentClassName="w-[88px]"
          options={dashboards.map((name) => ({ value: name, label: DASHBOARD_LABEL[name] ?? name }))}
        />
        <div className="flex-1" />
        <a
          href={buildEmbedUrl(selected, { runId, dbKind, machine, startedAt, finishedAt })}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center gap-1 border border-border px-2 py-1 font-mono text-[11px] text-muted-foreground transition-colors hover:border-foreground/40 hover:text-foreground"
          title="Open this dashboard in a new tab"
        >
          <ExternalLink className="h-3 w-3" /> Open
        </a>
      </div>

      {/* Machine pills — system dashboard only */}
      {machineActive && targets.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5 border-b border-border bg-background px-2 py-2">
          <span className="mr-1 font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
            Machines
          </span>
          <button
            type="button"
            onClick={() => setMachine("")}
            className={cn(
              "border px-1.5 py-0.5 font-mono text-[10px] transition-colors",
              machine === ""
                ? "border-primary bg-primary/10 text-primary"
                : "border-border text-muted-foreground hover:text-foreground",
            )}
            title="All machines (auto)"
          >
            all
          </button>
          {targets.map((t) => {
            const short = t.id.length > 24 ? `…${t.id.slice(-24)}` : t.id;
            return (
              <button
                key={t.id}
                type="button"
                onClick={() => setMachine(t.id)}
                title={`${t.role}: ${t.id}`}
                className={cn(
                  "inline-flex items-center gap-1.5 border px-1.5 py-0.5 font-mono text-[10px] transition-colors",
                  machine === t.id
                    ? "border-primary bg-primary/10 text-primary"
                    : "border-border text-foreground/80 hover:border-foreground/40",
                )}
              >
                {t.role && (
                  <span className="border border-border px-1 py-px text-[9px] uppercase text-muted-foreground">
                    {t.role}
                  </span>
                )}
                <span className="max-w-[16rem] truncate">{short}</span>
              </button>
            );
          })}
        </div>
      )}

      {/* Iframes — every (dashboard, machine) pair is mounted and pre-loading;
          only the active pair is visible. A loader covers it until it loads. */}
      <div className="relative w-full flex-1" style={{ minHeight: "calc(100vh - 16rem)" }}>
        {Array.from(srcsRef.current.entries()).map(([k, src]) => (
          <iframe
            key={k}
            src={src}
            title={k}
            onLoad={() => setLoaded((prev) => (prev.has(k) ? prev : new Set(prev).add(k)))}
            className={cn("absolute inset-0 h-full w-full border-0", k === activeKey ? "block" : "hidden")}
            sandbox="allow-scripts allow-same-origin allow-popups allow-forms"
          />
        ))}
        {!loaded.has(activeKey) && (
          <div className="absolute inset-0 flex items-center justify-center gap-2 bg-background/70">
            <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
            <span className="font-mono text-xs text-muted-foreground">Loading dashboard…</span>
          </div>
        )}
      </div>
    </div>
  );
}
