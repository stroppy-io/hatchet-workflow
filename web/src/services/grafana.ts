// Grafana embed surface. The control plane does NOT expose a Grafana settings
// RPC; instead the gateway reverse-proxies the embedded Grafana under the same
// origin at /grafana (see internal/gateway/gateway.go). Dashboards are
// provisioned with stable UIDs (deployments/grafana/dashboards/*.json), so we
// map dashboard kind -> UID here and build per-run kiosk embed URLs directly.

/**
 * Base the gateway proxies to the embedded Grafana. Same-origin "/grafana" by
 * default (matches the backend GRAFANA_URL); override with VITE_GRAFANA_URL when
 * Grafana is served elsewhere. NOTE: requires the gateway (not the bare API
 * server) to be answering /grafana/* — otherwise the SPA shell is returned and
 * Grafana reports "failed to load its application files".
 */
export const GRAFANA_BASE =
  (import.meta.env.VITE_GRAFANA_URL as string | undefined)?.replace(/\/$/, "") || "/grafana";

/** Provisioned dashboard UIDs (deployments/grafana/dashboards/*.json). */
export const DASHBOARD_UID: Record<string, string> = {
  workload: "stroppy-metrics-v1",
  system: "stroppy-system",
  postgres: "stroppy-postgres",
  mysql: "stroppy-mysql",
  // MariaDB reuses the MySQL dashboard: same mysqld_exporter, same metric names.
  mariadb: "stroppy-mysql",
  cockroach: "stroppy-cockroach",
  ydb: "stroppy-ydb",
  picodata: "stroppy-picodata",
};

/** Human label per dashboard key. */
export const DASHBOARD_LABEL: Record<string, string> = {
  workload: "Workload",
  system: "System",
  postgres: "PostgreSQL",
  mysql: "MySQL",
  mariadb: "MariaDB",
  cockroach: "CockroachDB",
  ydb: "YDB",
  picodata: "Picodata",
};

/** Per-run metric series prefix (matches the agent's metric namespacing). */
export function runPrefix(runId: string): string {
  return `stroppy_${runId.replace(/-/g, "_")}_`;
}

/**
 * The dashboards to surface for a run: always Workload + System, plus the
 * database-specific one when the run's db kind has a provisioned dashboard.
 */
export function dashboardsForRun(dbKind?: string): string[] {
  const out = ["workload", "system"];
  if (dbKind && DASHBOARD_UID[dbKind]) out.push(dbKind);
  return out;
}

interface EmbedOpts {
  runId: string;
  dbKind?: string;
  /** machine id for the system dashboard's `node` template var ("" = All). */
  machine?: string;
  startedAt?: string;
  finishedAt?: string;
}

function validTs(ts?: string): boolean {
  return !!ts && ts !== "0001-01-01T00:00:00Z" && new Date(ts).getFullYear() >= 2000;
}

/**
 * Build the kiosk embed URL for a dashboard. Scoping differs by dashboard:
 *   - workload → var-prefix (run-wide metric namespace)
 *   - system   → var-run_id + var-node (per-machine)
 *   - db kinds → var-run_id
 * The time window is locked to [started-30s, finished+60s]; while running it
 * tracks now with a 5s refresh.
 */
export function buildEmbedUrl(name: string, opts: EmbedOpts): string {
  const uid = DASHBOARD_UID[name] ?? name;
  // Assembled as explicit pairs (not URLSearchParams) so `kiosk` can be emitted
  // BARE — Grafana only enters kiosk mode (hide nav/chrome, show just the
  // dashboard) for `&kiosk`; `kiosk=` with a value renders the full UI.
  const pairs: string[] = [];
  const add = (k: string, v: string) => pairs.push(`${k}=${encodeURIComponent(v)}`);

  if (name === "workload") {
    add("var-prefix", runPrefix(opts.runId));
  } else {
    add("var-run_id", opts.runId);
    if (name === "system") add("var-node", opts.machine || "All");
  }

  if (validTs(opts.startedAt)) {
    const from = new Date(opts.startedAt as string).getTime() - 30_000;
    if (validTs(opts.finishedAt)) {
      add("from", String(from));
      add("to", String(new Date(opts.finishedAt as string).getTime() + 60_000));
    } else {
      add("from", String(from));
      add("to", "now");
      add("refresh", "5s");
    }
  }

  add("theme", "dark");
  pairs.push("kiosk"); // bare flag — full kiosk mode
  return `${GRAFANA_BASE}/d/${uid}?${pairs.join("&")}`;
}
