import { useParams } from "react-router-dom";

import { MetricsPanel } from "@/components/metrics-panel";
import { JsonPanel } from "@/components/json-panel";
import { Panel } from "@/components/panel";
import { StateBlock } from "@/components/state-block";
import { StatusBadge } from "@/components/status-badge";
import { useListQuery } from "@/hooks/use-list-query";
import { api } from "@/lib/connect";
import { formatId } from "@/lib/format";

// Public, no-auth, no-tenant page (GetSharedRun is token-addressed). Rendered
// outside ProtectedRoute / AppShell.
export function SharedRunPage() {
  const { token = "" } = useParams();
  const result = useListQuery(() => api.run.getSharedRun({ token }), [token]);
  const run = result.data?.run;
  const metrics = result.data?.metrics;

  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="border-b bg-[#070708] px-6 py-4">
        <div className="mx-auto flex max-w-5xl items-center gap-3">
          <span className="text-sm font-semibold">Stroppy Cloud</span>
          <span className="text-xs text-muted-foreground">shared run (read-only)</span>
        </div>
      </header>
      <main className="mx-auto max-w-5xl space-y-5 p-6">
        <StateBlock loading={result.loading} error={result.error} empty={!result.loading && !run} emptyMessage="Shared run not found or expired.">
          {run ? (
            <>
              <div className="flex items-center gap-3">
                <h1 className="text-xl font-semibold">{run.name || formatId(run.entity?.id?.value)}</h1>
                <StatusBadge status={run.status} />
              </div>
              {run.description ? <p className="text-sm text-muted-foreground">{run.description}</p> : null}
              <Panel title="Metrics">
                <MetricsPanel metrics={metrics?.metrics ?? []} />
              </Panel>
              <Panel title="TestPreset snapshot">
                <JsonPanel value={run.testPreset} />
              </Panel>
            </>
          ) : null}
        </StateBlock>
      </main>
    </div>
  );
}
