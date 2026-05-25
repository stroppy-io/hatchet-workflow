import { useState } from "react";
import { Plus, X } from "lucide-react";
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

// Seed the run id list from the URL: ?runs=id1,id2,... (new N-way link from the
// Runs table) or the legacy ?a=&b= pair. Always keep at least two rows.
function initialRuns(params: URLSearchParams): string[] {
  const fromRuns = params.get("runs");
  if (fromRuns) {
    const ids = fromRuns.split(",").map((s) => s.trim()).filter(Boolean);
    if (ids.length >= 2) return ids;
    if (ids.length === 1) return [ids[0], ""];
  }
  return [params.get("a") ?? "", params.get("b") ?? ""];
}

export function ComparePage() {
  const tenantId = useTenantId();
  const [params] = useSearchParams();
  const [runs, setRuns] = useState<string[]>(() => initialRuns(params));
  const [threshold, setThreshold] = useState(5);
  const [comparison, setComparison] = useState<Comparison | null>(null);

  const validRuns = runs.map((r) => r.trim()).filter(Boolean);
  const canCompare = validRuns.length >= 2;

  const compare = useAction(() =>
    api.run.compareRuns({
      tenantId: tenantIdMessage(tenantId),
      runIds: validRuns.map((id) => idMessage(id)),
      threshold,
    }),
  );

  async function go() {
    const response = await compare.run();
    if (response) setComparison(response);
  }

  const setRunAt = (index: number, value: string) => setRuns((cur) => cur.map((run, i) => (i === index ? value : run)));
  const addRun = () => setRuns((cur) => [...cur, ""]);
  const removeRun = (index: number) => setRuns((cur) => (cur.length <= 2 ? cur : cur.filter((_, i) => i !== index)));

  return (
    <section className="flex min-h-0 flex-col gap-5 p-6">
      <div>
        <h1 className="text-xl font-semibold tracking-normal">Compare runs</h1>
        <p className="mt-1 text-sm text-muted-foreground">Metric-by-metric diff of two or more runs against a baseline (the first run).</p>
      </div>

      <Panel title="Runs">
        <div className="space-y-3">
          {runs.map((run, index) => (
            <div key={index} className="flex items-end gap-2">
              <div className="flex-1 space-y-1.5">
                <Label>{index === 0 ? "Baseline" : `Run ${index + 1}`}</Label>
                <Input
                  className="font-mono text-xs"
                  value={run}
                  placeholder="test run id"
                  onChange={(event) => setRunAt(index, event.target.value)}
                />
              </div>
              <Button
                size="icon-sm"
                variant="ghost"
                className="mb-0.5"
                disabled={runs.length <= 2}
                onClick={() => removeRun(index)}
                aria-label="Remove run"
              >
                <X />
              </Button>
            </div>
          ))}
          <div className="flex flex-wrap items-end justify-between gap-3">
            <Button size="sm" variant="outline" onClick={addRun}>
              <Plus />
              Add run
            </Button>
            <div className="flex items-end gap-3">
              <div className="w-32 space-y-1.5">
                <Label>Threshold %</Label>
                <Input type="number" min={0} max={100} value={threshold} onChange={(event) => setThreshold(Number(event.target.value) || 0)} />
              </div>
              <Button onClick={go} disabled={!canCompare || compare.loading}>
                {compare.loading ? "Comparing…" : `Compare ${validRuns.length || ""}`}
              </Button>
            </div>
          </div>
        </div>
        {compare.error ? <p className="mt-2 text-sm text-destructive">{compare.error}</p> : null}
      </Panel>

      {comparison ? <MetricsDiff comparison={comparison} /> : null}
    </section>
  );
}
