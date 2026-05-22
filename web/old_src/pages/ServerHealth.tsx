import { useEffect, useState, useMemo } from "react";
import { getGrafanaSettings } from "@/api/client";
import type { GrafanaSettings } from "@/api/types";
import { AlertCircle } from "lucide-react";

export function ServerHealth() {
  const [grafana, setGrafana] = useState<GrafanaSettings | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getGrafanaSettings()
      .then(setGrafana)
      .catch((err) => setError(err instanceof Error ? err.message : "Failed to load Grafana settings"));
  }, []);

  const iframeSrc = useMemo(() => {
    if (!grafana?.url) return null;
    return `${grafana.url}/d/stroppy-server?kiosk&theme=dark&refresh=10s`;
  }, [grafana]);

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

      <div className="flex-1 min-h-0">
        {grafana?.embed_enabled && iframeSrc ? (
          <iframe
            src={iframeSrc}
            className="w-full h-full border-0"
            title="Server Health Dashboard"
            sandbox="allow-scripts allow-same-origin allow-popups allow-forms"
          />
        ) : (
          <div className="p-6 text-sm text-muted-foreground">
            {grafana && !grafana.embed_enabled
              ? "Grafana embed is disabled. Enable GF_SECURITY_ALLOW_EMBEDDING=true."
              : !error
                ? "Loading..."
                : "Cannot display dashboard."}
          </div>
        )}
      </div>
    </div>
  );
}
