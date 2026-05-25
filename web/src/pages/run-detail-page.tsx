import { useEffect, useState } from "react";
import { ArrowLeft, GitCompare, Share2, XCircle } from "lucide-react";
import { useNavigate, useParams } from "react-router-dom";

import { useConfirm } from "@/components/confirm-dialog";
import { LogStream } from "@/components/log-stream";
import { MetricsPanel } from "@/components/metrics-panel";
import { JsonPanel } from "@/components/json-panel";
import { Panel } from "@/components/panel";
import { StateBlock } from "@/components/state-block";
import { StatusBadge } from "@/components/status-badge";
import { Button } from "@/components/ui/button";
import { CopyButton } from "@/components/ui/copy-button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useAction } from "@/hooks/use-action";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { useListQuery } from "@/hooks/use-list-query";
import { useTenantId } from "@/hooks/use-tenant-id";
import { api } from "@/lib/connect";
import { formatId, formatTimestamp } from "@/lib/format";
import { idMessage, tenantIdMessage } from "@/lib/proto";
import { tenantPath } from "@/lib/routes";
import { notifyError, notifySuccess } from "@/lib/toast";
import type { LogLine } from "@/lib/proto/cloud/v1/runtime/logs/logs_pb.ts";

export function RunDetailPage() {
  const tenantId = useTenantId();
  const { runId = "" } = useParams();
  const navigate = useNavigate();
  const confirm = useConfirm();
  const [reloadKey, setReloadKey] = useState(0);

  const result = useListQuery(() => api.run.getTestRun({ tenantId: tenantIdMessage(tenantId), id: idMessage(runId) }), [tenantId, runId, reloadKey]);
  const metrics = useListQuery(() => api.run.getRunMetrics({ tenantId: tenantIdMessage(tenantId), runId: idMessage(runId) }), [tenantId, runId, reloadKey]);
  const run = result.data;

  const [shareUrl, setShareUrl] = useState<string | null>(null);
  const share = useAction(() => api.run.createShareLink({ tenantId: tenantIdMessage(tenantId), runId: idMessage(runId) }));

  async function cancel() {
    const ok = await confirm({ title: "Cancel run?", description: "The run will stop and teardown will run.", danger: true, confirmLabel: "Cancel run" });
    if (!ok) return;
    try {
      await api.run.cancelTestRun({ tenantId: tenantIdMessage(tenantId), id: idMessage(runId) });
      notifySuccess("Run cancelling");
      setReloadKey((key) => key + 1);
    } catch (error) {
      notifyError(error, "Could not cancel run");
    }
  }

  async function doShare() {
    const response = await share.run();
    if (response) setShareUrl(response.url);
  }

  return (
    <section className="flex min-h-0 flex-col gap-5 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon-sm" onClick={() => navigate(tenantPath(tenantId, "/runs"))}>
            <ArrowLeft />
          </Button>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-xl font-semibold tracking-normal">{run?.name || formatId(runId)}</h1>
              {run ? <StatusBadge status={run.status} /> : null}
            </div>
            <p className="mt-1 font-mono text-xs text-muted-foreground">{runId}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => navigate(tenantPath(tenantId, `/compare?a=${runId}`))}>
            <GitCompare />
            Compare
          </Button>
          <Button variant="outline" size="sm" onClick={doShare} disabled={share.loading}>
            <Share2 />
            Share
          </Button>
          <Button variant="outline" size="sm" onClick={cancel}>
            <XCircle />
            Cancel
          </Button>
        </div>
      </div>

      <StateBlock loading={result.loading && !run} error={result.error} empty={!result.loading && !run} emptyMessage="Run not found.">
        {run ? (
          <Tabs defaultValue="overview">
            <TabsList>
              <TabsTrigger value="overview">Overview</TabsTrigger>
              <TabsTrigger value="logs">Logs</TabsTrigger>
              <TabsTrigger value="metrics">Metrics</TabsTrigger>
            </TabsList>

            <TabsContent value="overview" className="space-y-4">
              <Panel title="Summary">
                <dl className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                  <Fact label="Status" value={<StatusBadge status={run.status} />} />
                  <Fact label="DAG" value={<span className="font-mono text-xs">{formatId(run.dag?.value)}</span>} />
                  <Fact label="Suite run" value={<span className="font-mono text-xs">{formatId(run.suiteRunId?.value)}</span>} />
                  <Fact label="Created" value={formatTimestamp(run.entity?.timestamps?.createdAt)} />
                </dl>
                {run.description ? <p className="mt-3 text-sm text-muted-foreground">{run.description}</p> : null}
              </Panel>
              <Panel title="TestPreset snapshot">
                <JsonPanel value={run.testPreset} />
              </Panel>
            </TabsContent>

            <TabsContent value="logs">
              <RunLogs tenantId={tenantId} runId={runId} />
            </TabsContent>

            <TabsContent value="metrics" className="space-y-3">
              <StateBlock loading={metrics.loading} error={metrics.error}>
                <MetricsPanel metrics={metrics.data?.metrics ?? []} />
              </StateBlock>
            </TabsContent>
          </Tabs>
        ) : null}
      </StateBlock>

      <Dialog open={shareUrl !== null} onOpenChange={(open) => !open && setShareUrl(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Share link</DialogTitle>
            <DialogDescription>Read-only, no sign-in required.</DialogDescription>
          </DialogHeader>
          <div className="flex items-center gap-2">
            <code className="block flex-1 select-all break-all rounded-md border bg-background p-3 font-mono text-xs">{shareUrl}</code>
            <CopyButton value={shareUrl ?? ""} toastLabel="Link copied" />
          </div>
        </DialogContent>
      </Dialog>
    </section>
  );
}

function Fact({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div>
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className="mt-0.5 text-sm">{value}</dd>
    </div>
  );
}

function RunLogs({ tenantId, runId }: { tenantId: string; runId: string }) {
  const [lines, setLines] = useState<LogLine[]>([]);
  const [nodeExec, setNodeExec] = useState("");
  const [componentId, setComponentId] = useState("");
  const [live, setLive] = useState(false);
  const [token, setToken] = useState("");
  const dNode = useDebouncedValue(nodeExec);
  const dComponent = useDebouncedValue(componentId);

  const query = useAction((pageToken: string) =>
    api.run.queryRunLogs({
      tenantId: tenantIdMessage(tenantId),
      runId: idMessage(runId),
      nodeExecutionId: dNode || undefined,
      componentId: dComponent || undefined,
      pageSize: 500,
      pageToken,
    }),
  );

  async function load(reset: boolean) {
    const response = await query.run(reset ? "" : token);
    if (!response) return;
    setLines((prev) => (reset ? response.lines : [...prev, ...response.lines]));
    setToken(response.nextToken);
  }

  // Re-query when run or filters change.
  useEffect(() => {
    setLines([]);
    load(true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tenantId, runId, dNode, dComponent]);

  // Live follow via connect-web server-stream.
  useEffect(() => {
    if (!live) return;
    const controller = new AbortController();
    (async () => {
      try {
        for await (const line of api.run.streamTestRunLogs(
          { tenantId: tenantIdMessage(tenantId), id: idMessage(runId), nodeExecutionId: dNode || undefined, componentId: dComponent || undefined },
          { signal: controller.signal },
        )) {
          setLines((prev) => [...prev, line].slice(-5000));
        }
      } catch (error) {
        if (!controller.signal.aborted) notifyError(error, "Log stream error");
      }
    })();
    return () => controller.abort();
  }, [live, tenantId, runId, dNode, dComponent]);

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-end gap-3">
        <div className="space-y-1">
          <Label>Component</Label>
          <Input className="h-8 w-48 font-mono text-xs" value={componentId} placeholder="component id" onChange={(event) => setComponentId(event.target.value)} />
        </div>
        <div className="space-y-1">
          <Label>Stage</Label>
          <Input className="h-8 w-48 font-mono text-xs" value={nodeExec} placeholder="node_execution_id" onChange={(event) => setNodeExec(event.target.value)} />
        </div>
        <div className="flex items-center gap-2 pb-1">
          <Label>Live</Label>
          <Switch checked={live} onCheckedChange={setLive} />
        </div>
        <Button size="sm" variant="outline" onClick={() => load(true)} disabled={query.loading}>
          Refresh
        </Button>
      </div>
      <LogStream lines={lines} />
      {token ? (
        <Button size="sm" variant="outline" onClick={() => load(false)} disabled={query.loading}>
          Load more
        </Button>
      ) : null}
    </div>
  );
}
