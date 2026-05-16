import { useEffect, useState } from "react";
import { AlertCircle } from "lucide-react";

interface HealthStatus {
  status: string;
}

export function ServerHealth() {
  const [health, setHealth] = useState<HealthStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [grafanaUrl, setGrafanaUrl] = useState<string | null>(null);

  useEffect(() => {
    fetch("/health")
      .then((r) => {
        if (!r.ok) throw new Error(`${r.status}`);
        return r.json() as Promise<HealthStatus>;
      })
      .then(setHealth)
      .catch((err) => setError(err instanceof Error ? err.message : "Failed to reach /health"));
  }, []);

  useEffect(() => {
    // Try to get grafana URL from settings (best-effort).
    fetch("/api/v1/grafana")
      .then((r) => r.ok ? r.json() : null)
      .then((data) => {
        if (data?.url && data?.embed_enabled) {
          setGrafanaUrl(`${data.url}/d/stroppy-server?kiosk&theme=dark&refresh=10s`);
        }
      })
      .catch(() => {});
  }, []);

  return (
    <div className="flex flex-col h-full">
      <div className="px-6 py-4 shrink-0">
        <h1 className="text-lg font-semibold font-mono">Server Health</h1>
        <p className="text-sm text-muted-foreground">Self-monitoring metrics for the stroppy-cloud server</p>
      </div>

      {error && (
        <div className="mx-6 flex items-center gap-2 text-sm p-3 border border-destructive/30 text-destructive">
          <AlertCircle className="h-4 w-4" />
          {error}
        </div>
      )}

      {health && !grafanaUrl && (
        <div className="mx-6 p-4 border border-zinc-800 bg-zinc-900/30">
          <div className="flex items-center gap-2">
            <div className={`w-2.5 h-2.5 rounded-full ${health.status === "ok" ? "bg-emerald-500" : "bg-red-500"}`} />
            <span className="text-sm font-mono text-zinc-200">
              Status: <span className={health.status === "ok" ? "text-emerald-400" : "text-red-400"}>{health.status}</span>
            </span>
          </div>
        </div>
      )}

      <div className="flex-1 min-h-0">
        {grafanaUrl ? (
          <iframe
            src={grafanaUrl}
            className="w-full h-full border-0"
            title="Server Health Dashboard"
            sandbox="allow-scripts allow-same-origin allow-popups allow-forms"
          />
        ) : !error && !health ? (
          <div className="p-6 text-sm text-muted-foreground">Loading...</div>
        ) : null}
      </div>
    </div>
  );
}
