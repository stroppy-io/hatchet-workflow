import { useEffect, useState, useMemo } from "react";
import { getGrafanaSettings } from "@/api/client";
import type { GrafanaSettings } from "@/api/types";
import { Card, CardContent } from "@/components/ui/card";
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
    // Use hardcoded UID matching the provisioned dashboard.
    return `${grafana.url}/d/stroppy-server?kiosk&theme=dark&refresh=10s`;
  }, [grafana]);

  return (
    <div className="p-6 space-y-6">
      <div>
        <h1 className="text-lg font-semibold font-mono">Server Health</h1>
        <p className="text-sm text-muted-foreground">Self-monitoring metrics for the stroppy-cloud server</p>
      </div>

      {error && (
        <div className="flex items-center gap-2 text-sm p-3 border border-destructive/30 text-destructive">
          <AlertCircle className="h-4 w-4" />
          {error}
        </div>
      )}

      <Card>
        <CardContent className="p-0">
          {grafana?.embed_enabled && iframeSrc ? (
            <iframe
              src={iframeSrc}
              className="w-full border-0 h-[calc(100vh-14rem)]"
              title="Server Health Dashboard"
              sandbox="allow-scripts allow-same-origin allow-popups allow-forms"
            />
          ) : (
            <div className="p-6 text-sm text-muted-foreground">
              {grafana && !grafana.embed_enabled
                ? "Grafana embed is disabled. Enable it in server settings to view the dashboard."
                : !error
                  ? "Loading Grafana settings..."
                  : "Cannot display dashboard."}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
