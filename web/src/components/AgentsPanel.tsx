import { useEffect, useState, useCallback } from "react";
import { clients } from "@/api/clients";
import { useTenantId } from "@/hooks/useTenantPath";
import {
  AgentStatus,
  type Agent as ProtoAgent,
} from "@/lib/proto/cloud/v1/agent/agent_pb";
import { MachineRole } from "@/lib/proto/cloud/v1/catalog/deployment_pb";

interface AgentInfo {
  machine_id: string;
  role: string;
  host?: string;
  internal_host?: string;
  agent_port?: number;
  registered: boolean;
  healthy: boolean;
  health_error?: string;
  last_seen_at?: string;
}

const ROLE_NAME: Record<MachineRole, string> = {
  [MachineRole.UNSPECIFIED]: "unknown",
  [MachineRole.DATABASE]: "database",
  [MachineRole.MONITOR]: "monitor",
  [MachineRole.STROPPY]: "stroppy",
  [MachineRole.PROXY]: "proxy",
  [MachineRole.PGBOUNCER]: "pgbouncer",
  [MachineRole.ETCD]: "etcd",
  [MachineRole.YDB_STORAGE]: "ydb-storage",
  [MachineRole.YDB_DATABASE]: "ydb-database",
};

function agentToInfo(a: ProtoAgent): AgentInfo {
  const registered = a.status !== AgentStatus.UNSPECIFIED;
  const healthy = a.status === AgentStatus.HEALTHY;
  return {
    machine_id: a.machineId,
    role: ROLE_NAME[a.role] ?? "unknown",
    host: a.publicIp,
    internal_host: a.internalIp,
    registered,
    healthy,
    health_error: a.lastError || undefined,
  };
}

async function getRunAgents(runID: string, tenantID: string): Promise<AgentInfo[]> {
  // No per-run AgentAdminService RPC — fall back to tenant-scoped list filtered
  // client-side by dag_run_id. The TestRun.dag_run_id is the engine DagRun
  // that provisioned the agents, which is what Agent.dag_run_id carries.
  const tr = await clients.testRun.getTestRun({ value: runID });
  const dagRunId = tr.testRun?.dagRunId?.value ?? "";
  if (!dagRunId) return [];
  const resp = await clients.agentAdmin.listAgents({
    tenantId: { value: tenantID },
  });
  return (resp.agents ?? [])
    .filter((a) => a.dagRunId === dagRunId)
    .map(agentToInfo);
}
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from "@/components/ui/table";
import { RefreshCw, Cpu, AlertCircle, CheckCircle2, Circle, XCircle } from "lucide-react";

const ROLE_STYLE: Record<string, string> = {
  database: "border-emerald-500/40 text-emerald-400 bg-emerald-500/5",
  monitor: "border-blue-500/40 text-blue-400 bg-blue-500/5",
  stroppy: "border-amber-500/40 text-amber-400 bg-amber-500/5",
  proxy: "border-purple-500/40 text-purple-400 bg-purple-500/5",
  pgbouncer: "border-purple-500/40 text-purple-400 bg-purple-500/5",
  etcd: "border-cyan-500/40 text-cyan-400 bg-cyan-500/5",
  "ydb-storage": "border-emerald-500/40 text-emerald-400 bg-emerald-500/5",
  "ydb-database": "border-emerald-500/40 text-emerald-400 bg-emerald-500/5",
};

export function AgentsPanel({ runID }: { runID?: string }) {
  const tenantId = useTenantId();
  const [rows, setRows] = useState<AgentInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!runID || !tenantId) return;
    try {
      setRows(await getRunAgents(runID, tenantId));
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load agents");
    } finally {
      setLoading(false);
    }
  }, [runID, tenantId]);

  useEffect(() => {
    load();
    // Refresh every 5s — agents come and go fast during install/configure;
    // a stale view confuses the operator hunting for a stuck phase.
    const t = setInterval(load, 5000);
    return () => clearInterval(t);
  }, [load]);

  if (!runID) return <div className="p-4 text-xs text-zinc-500">No run id</div>;

  return (
    <div className="p-4 space-y-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Cpu className="h-4 w-4 text-primary" />
          <h2 className="text-sm font-semibold">Agents</h2>
          <span className="text-[10px] font-mono text-zinc-600">
            {rows.length} target{rows.length === 1 ? "" : "s"}
          </span>
        </div>
        <button
          type="button"
          onClick={load}
          className="text-zinc-500 hover:text-zinc-200 transition-colors"
          title="Refresh"
        >
          <RefreshCw className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
        </button>
      </div>

      {error && (
        <div className="flex items-center gap-2 text-xs p-2 border border-destructive/30 text-destructive font-mono">
          <AlertCircle className="h-3.5 w-3.5" />
          {error}
        </div>
      )}

      {rows.length === 0 ? (
        <div className="border border-dashed border-zinc-800 p-6 text-center text-xs text-zinc-500">
          No agents yet. They appear as soon as the machines phase finishes.
        </div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">Status</TableHead>
              <TableHead>Machine</TableHead>
              <TableHead>Role</TableHead>
              <TableHead>Host</TableHead>
              <TableHead>Port</TableHead>
              <TableHead>Last issue</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((a) => {
              const roleCls = ROLE_STYLE[a.role] || "border-zinc-700 text-zinc-400 bg-zinc-900/40";
              return (
                <TableRow key={a.machine_id}>
                  <TableCell>
                    <StatusDot registered={a.registered} healthy={a.healthy} />
                  </TableCell>
                  <TableCell>
                    <span className="font-mono text-[11px] text-zinc-200" title={a.machine_id}>
                      {a.machine_id.length > 32 ? a.machine_id.slice(-28) : a.machine_id}
                    </span>
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline" className={`text-[10px] ${roleCls}`}>
                      {a.role}
                    </Badge>
                  </TableCell>
                  <TableCell className="font-mono text-[10px] text-zinc-400">
                    {a.host || a.internal_host || "—"}
                  </TableCell>
                  <TableCell className="font-mono text-[10px] text-zinc-400">
                    {a.agent_port || "—"}
                  </TableCell>
                  <TableCell className="font-mono text-[10px] text-zinc-500 max-w-md truncate" title={a.health_error}>
                    {a.health_error || (a.healthy ? "" : a.registered ? "no health probe" : "not yet registered")}
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      )}
    </div>
  );
}

function StatusDot({ registered, healthy }: { registered: boolean; healthy: boolean }) {
  if (healthy) {
    return <CheckCircle2 className="h-3.5 w-3.5 text-emerald-400" />;
  }
  if (registered) {
    return <Circle className="h-3.5 w-3.5 text-amber-400" />;
  }
  return <XCircle className="h-3.5 w-3.5 text-zinc-600" />;
}
