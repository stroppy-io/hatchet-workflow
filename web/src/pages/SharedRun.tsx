import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { Activity, LineChart } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { MetricsPanel } from "@/components/run/MetricsPanel";
import { getSharedRun, type SharedRunVM } from "@/services/shares";

export function SharedRun() {
  const { token = "" } = useParams<{ token: string }>();
  const [run, setRun] = useState<SharedRunVM | null>(null);
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

  return (
    <div className="min-h-screen bg-background text-foreground">
      <div className="mx-auto flex max-w-6xl flex-col gap-4 p-6">
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
