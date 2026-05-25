import { useCallback, useEffect, useState } from "react";
import { ArrowLeft, GitCompare, Share2, XCircle } from "lucide-react";
import { useNavigate, useParams } from "react-router-dom";

import { AutoRefresh } from "@/components/auto-refresh";
import { useConfirm } from "@/components/confirm-dialog";
import { DagGraph } from "@/components/dag-graph";
import { LogStream } from "@/components/log-stream";
import { MetricsPanel } from "@/components/metrics-panel";
import { RunOverview } from "@/components/run-overview";
import { StateBlock } from "@/components/state-block";
import { StatusBadge } from "@/components/status-badge";
import { Button } from "@/components/ui/button";
import { CopyButton } from "@/components/ui/copy-button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
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
import { cn } from "@/lib/utils";
import type { Agent } from "@/lib/proto/cloud/v1/models/agent_pb.ts";
import type { Topology_Machine } from "@/lib/proto/cloud/v1/domain/topology_pb.ts";
import { AgentStatus } from "@/lib/proto/cloud/v1/runtime/agent/agent_pb.ts";

export function RunDetailPage() {
  const tenantId = useTenantId();
  const { runId = "" } = useParams();
  const navigate = useNavigate();
  const confirm = useConfirm();
  const [reloadKey, setReloadKey] = useState(0);
  const [activeTab, setActiveTab] = useState("overview");
  // Node execution id to focus when jumping from the pipeline into the logs tab.
  const [logFocusNode, setLogFocusNode] = useState("");

  const result = useListQuery(() => api.run.getTestRun({ tenantId: tenantIdMessage(tenantId), id: idMessage(runId) }), [tenantId, runId, reloadKey]);
  const metrics = useListQuery(() => api.run.getRunMetrics({ tenantId: tenantIdMessage(tenantId), runId: idMessage(runId) }), [tenantId, runId, reloadKey]);
  const dagResult = useListQuery(() => api.run.getTestRunDag({ tenantId: tenantIdMessage(tenantId), id: idMessage(runId) }), [tenantId, runId, reloadKey]);
  const agentsResult = useListQuery(() => api.run.listAgents({ tenantId: tenantIdMessage(tenantId), page: { size: 200 } }), [tenantId, reloadKey]);
  const run = result.data;

  const reload = useCallback(() => setReloadKey((key) => key + 1), []);

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
          <AutoRefresh onRefresh={reload} />
          <Button variant="outline" size="sm" onClick={() => navigate(tenantPath(tenantId, `/compare?runs=${runId}`))}>
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
          <Tabs value={activeTab} onValueChange={setActiveTab}>
            <TabsList>
              <TabsTrigger value="overview">Overview</TabsTrigger>
              <TabsTrigger value="graph">Graph</TabsTrigger>
              <TabsTrigger value="infra">Infrastructure</TabsTrigger>
              <TabsTrigger value="logs">Logs</TabsTrigger>
              <TabsTrigger value="metrics">Metrics</TabsTrigger>
            </TabsList>

            <TabsContent value="overview">
              {run.description ? <p className="mb-3 max-w-3xl whitespace-pre-wrap text-sm text-muted-foreground">{run.description}</p> : null}
              <RunOverview
                run={run}
                dag={dagResult.data ?? undefined}
                dagLoading={dagResult.loading}
                dagError={dagResult.error}
                onViewLogs={(node) => {
                  setLogFocusNode(node);
                  setActiveTab("logs");
                }}
              />
            </TabsContent>

            <TabsContent value="graph">
              <StateBlock loading={dagResult.loading} error={dagResult.error} empty={!dagResult.loading && !dagResult.data} emptyMessage="No DAG for this run.">
                {dagResult.data ? <DagGraph dag={dagResult.data} /> : null}
              </StateBlock>
            </TabsContent>

            <TabsContent value="infra">
              <RunInfrastructure
                machines={run.testPreset?.topology?.machines ?? []}
                agents={agentsResult.data?.agents ?? []}
                loading={agentsResult.loading}
                error={agentsResult.error}
              />
            </TabsContent>

            <TabsContent value="logs">
              <RunLogs tenantId={tenantId} runId={runId} focusNode={logFocusNode} />
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

function RunLogs({ tenantId, runId, focusNode }: { tenantId: string; runId: string; focusNode?: string }) {
  const [lines, setLines] = useState<LogLine[]>([]);
  const [nodeExec, setNodeExec] = useState("");
  const [componentId, setComponentId] = useState("");
  const [live, setLive] = useState(false);
  const [token, setToken] = useState("");
  const dNode = useDebouncedValue(nodeExec);
  const dComponent = useDebouncedValue(componentId);

  // Seed the stage filter when the user jumps here from a pipeline step.
  useEffect(() => {
    if (focusNode) setNodeExec(focusNode);
  }, [focusNode]);

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

  async function copyLink() {
    try {
      const response = await api.run.buildLogLink({
        tenantId: tenantIdMessage(tenantId),
        runId: idMessage(runId),
        nodeExecutionId: dNode || undefined,
        componentId: dComponent || undefined,
      });
      await navigator.clipboard.writeText(response.url);
      notifySuccess("Log link copied");
    } catch (error) {
      notifyError(error, "Could not build log link");
    }
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
        <Button size="sm" variant="outline" onClick={copyLink}>
          Copy link
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

const AGENT_STATUS: Record<number, { label: string; cls: string }> = {
  [AgentStatus.REGISTERED]: { label: "Registered", cls: "bg-muted text-muted-foreground" },
  [AgentStatus.READY]: { label: "Ready", cls: "bg-success/15 text-success" },
  [AgentStatus.BUSY]: { label: "Busy", cls: "bg-primary/15 text-primary" },
  [AgentStatus.OFFLINE]: { label: "Offline", cls: "bg-muted text-muted-foreground" },
  [AgentStatus.FAILED]: { label: "Failed", cls: "bg-destructive/15 text-destructive" },
};

function AgentBadge({ agent }: { agent?: Agent }) {
  if (!agent) return <span className="text-xs text-muted-foreground">no agent</span>;
  const status = AGENT_STATUS[agent.status] ?? { label: "—", cls: "bg-muted text-muted-foreground" };
  return <span className={cn("inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium", status.cls)}>{status.label}</span>;
}

// Run infrastructure: the topology's machines with their live agent overlaid
// (machine == agent, matched by machine_id).
function RunInfrastructure({ machines, agents, loading, error }: { machines: Topology_Machine[]; agents: Agent[]; loading: boolean; error: string | null }) {
  const byMachine = new Map<string, Agent>();
  for (const agent of agents) byMachine.set(agent.machineId, agent);

  return (
    <StateBlock loading={loading} error={error} empty={machines.length === 0} emptyMessage="No topology machines.">
      <div className="overflow-hidden rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Machine</TableHead>
              <TableHead>Resources</TableHead>
              <TableHead>Components</TableHead>
              <TableHead>Agent</TableHead>
              <TableHead>Last seen</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {machines.map((machine) => {
              const agent = byMachine.get(machine.id);
              return (
                <TableRow key={machine.id}>
                  <TableCell className="font-mono text-xs">{machine.id}</TableCell>
                  <TableCell className="text-xs">{machine.cores}c / {Number(machine.memoryGb)}GB</TableCell>
                  <TableCell className="text-xs">{machine.components.length}</TableCell>
                  <TableCell><AgentBadge agent={agent} /></TableCell>
                  <TableCell>{formatTimestamp(agent?.lastSeenAt)}</TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>
    </StateBlock>
  );
}
