import { useState } from "react";
import { useSearchParams } from "react-router-dom";

import { MetricsDiff } from "@/components/metrics-diff";
import { Panel } from "@/components/panel";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAction } from "@/hooks/use-action";
import { useTenantId } from "@/hooks/use-tenant-id";
import { api } from "@/lib/connect";
import { idMessage, tenantIdMessage } from "@/lib/proto";
import type { Comparison } from "@/lib/proto/cloud/v1/runtime/metrics/metrics_pb.ts";

export function ComparePage() {
  const tenantId = useTenantId();
  const [params] = useSearchParams();
  const [runA, setRunA] = useState(params.get("a") ?? "");
  const [runB, setRunB] = useState(params.get("b") ?? "");
  const [threshold, setThreshold] = useState(5);
  const [comparison, setComparison] = useState<Comparison | null>(null);

  const compare = useAction(() =>
    api.run.compareRuns({ tenantId: tenantIdMessage(tenantId), runA: idMessage(runA), runB: idMessage(runB), threshold }),
  );

  async function go() {
    const response = await compare.run();
    if (response) setComparison(response);
  }

  return (
    <section className="flex min-h-0 flex-col gap-5 p-6">
      <div>
        <h1 className="text-xl font-semibold tracking-normal">Compare runs</h1>
        <p className="mt-1 text-sm text-muted-foreground">Metric-by-metric diff of two test runs.</p>
      </div>

      <Panel title="Runs">
        <div className="flex flex-wrap items-end gap-3">
          <div className="flex-1 space-y-1.5">
            <Label>Run A</Label>
            <Input className="font-mono text-xs" value={runA} placeholder="test run id" onChange={(event) => setRunA(event.target.value)} />
          </div>
          <div className="flex-1 space-y-1.5">
            <Label>Run B</Label>
            <Input className="font-mono text-xs" value={runB} placeholder="test run id" onChange={(event) => setRunB(event.target.value)} />
          </div>
          <div className="w-32 space-y-1.5">
            <Label>Threshold %</Label>
            <Input type="number" min={0} max={100} value={threshold} onChange={(event) => setThreshold(Number(event.target.value) || 0)} />
          </div>
          <Button onClick={go} disabled={!runA.trim() || !runB.trim() || compare.loading}>
            {compare.loading ? "Comparing…" : "Compare"}
          </Button>
        </div>
        {compare.error ? <p className="mt-2 text-sm text-destructive">{compare.error}</p> : null}
      </Panel>

      {comparison ? <MetricsDiff comparison={comparison} /> : null}
    </section>
  );
}
