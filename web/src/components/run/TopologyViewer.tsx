// TOPOLOGY VIEWER — renders the topology envelope as an interactive xyflow
// graph. Runtime topology is preferred: control-plane, machines, agents,
// components, monitors/exporters and runtime connections are rendered directly
// from API data. Logical TopologySpec is only a fallback for early snapshots.
import { useMemo } from "react";
import {
  Background,
  BackgroundVariant,
  Controls,
  MarkerType,
  Position,
  ReactFlow,
  type Edge,
  type Node,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import type {
  RuntimeConnectionJson,
  RuntimeNodeJson,
  TopologyJson,
} from "@/lib/proto/cloud/v1/topology/topology_pb";

const RUNTIME_KIND_COLOR: Record<string, string> = {
  KIND_CONTROL_PLANE: "#f97316",
  KIND_MACHINE: "#64748b",
  KIND_AGENT: "#a1a1aa",
  KIND_COMPONENT: "#3b82f6",
  KIND_MONITOR: "#a78bfa",
  KIND_EXPORTER: "#c084fc",
  KIND_WORKLOAD: "#22c55e",
  KIND_EXTERNAL: "#94a3b8",
  KIND_UNSPECIFIED: "#525252",
};

const SPEC_KIND_COLOR: Record<string, string> = {
  KIND_DATABASE: "#3b82f6",
  KIND_REPLICA: "#60a5fa",
  KIND_PROXY: "#eab308",
  KIND_MONITOR: "#a78bfa",
  KIND_AGENT: "#71717a",
  KIND_WORKLOAD: "#22c55e",
  KIND_COORDINATOR: "#7c6cc8",
  KIND_ADDON: "#94a3b8",
  KIND_EXTERNAL: "#64748b",
  KIND_UNSPECIFIED: "#525252",
};

const CONN_COLOR: Record<string, string> = {
  KIND_FLOW: "#3b82f6",
  KIND_PROXY: "#eab308",
  KIND_REPLICATION: "#60a5fa",
  KIND_COORDINATION: "#7c6cc8",
  KIND_OBSERVATION: "#a78bfa",
  KIND_SUPPORT: "#737373",
  KIND_UNSPECIFIED: "#525252",
};

const STATUS_COLOR: Record<string, string> = {
  STATUS_RUNNING: "#f59e0b",
  STATUS_DEPLOYMENT: "#f59e0b",
  STATUS_ALLOCATED: "#f59e0b",
  STATUS_RETRY_WAIT: "#f59e0b",
  STATUS_COMPLETED: "#22c55e",
  STATUS_DEPLOYED: "#22c55e",
  STATUS_FAILED: "#ef4444",
  STATUS_CANCELLED: "#ef4444",
  STATUS_CANCELLING: "#ef4444",
  STATUS_SKIPPED: "#71717a",
  STATUS_PENDING: "#71717a",
  STATUS_UNSPECIFIED: "#525252",
};

const CHILD_W = 252;
const CHILD_H = 58;
const CHILD_GAP = 16;
const GROUP_W = 284;
const PAD_X = 16;
const PAD_TOP = 54;
const PAD_BOTTOM = 18;
const COL_GAP = 180;
const CONTROL_W = 300;
const CONTROL_H = 68;
const CONTROL_Y = 24;
const MACHINE_Y = 164;

interface Built {
  nodes: Node[];
  edges: Edge[];
}

function shortEnum(v?: string, prefix = ""): string {
  return (v ?? "").replace(prefix, "").replace(/^KIND_/, "").toLowerCase();
}

function statusColor(status?: string): string {
  return STATUS_COLOR[status ?? "STATUS_UNSPECIFIED"] ?? STATUS_COLOR.STATUS_UNSPECIFIED;
}

function runtimeKindColor(kind?: string): string {
  return RUNTIME_KIND_COLOR[kind ?? "KIND_UNSPECIFIED"] ?? RUNTIME_KIND_COLOR.KIND_UNSPECIFIED;
}

function quietSupport(edge: RuntimeConnectionJson): boolean {
  const relation = edge.labels?.relation;
  return (
    edge.kind === "KIND_SUPPORT" &&
    (relation === "agent_control" ||
      relation === "agent_execution" ||
      relation === "placement" ||
      relation === "depends_on" ||
      relation === "agent_action")
  );
}

function edgeColor(edge: RuntimeConnectionJson): string {
  const status = edge.status ?? "";
  if (status === "STATUS_FAILED" || status === "STATUS_CANCELLED" || status === "STATUS_CANCELLING") {
    return statusColor(status);
  }
  if (quietSupport(edge)) {
    return "#575757";
  }
  if (status === "STATUS_RUNNING" || status === "STATUS_DEPLOYMENT" || status === "STATUS_ALLOCATED") {
    return statusColor(status);
  }
  return CONN_COLOR[edge.kind ?? "KIND_UNSPECIFIED"] ?? CONN_COLOR.KIND_UNSPECIFIED;
}

function activeStatus(status?: string): boolean {
  return status === "STATUS_RUNNING" || status === "STATUS_DEPLOYMENT" || status === "STATUS_ALLOCATED";
}

function compactEndpoint(endpoint?: string): string {
  const value = endpoint ?? "";
  if (value === "/insert/0/prometheus/api/v1/write") return "metrics write";
  if (value === "/insert/jsonline") return "logs write";
  if (value === "/api/binaries") return "binary cache";
  if (value.startsWith("write_file:")) return "write file";
  if (value.startsWith("fetch_file:")) return "fetch file";
  return value;
}

function runtimeConnLabel(c: RuntimeConnectionJson): string {
  const proto = shortEnum(c.protocol, "PROTOCOL_");
  const endpoint = compactEndpoint(c.endpointName);
  const parts = [proto, endpoint].filter(Boolean);
  if (c.port) parts.push(String(c.port));
  return parts.join(" ");
}

function visibleRuntimeConnLabel(c: RuntimeConnectionJson): string | undefined {
  if (quietSupport(c)) return undefined;
  if (
    c.endpointName === "call_cmd" ||
    c.endpointName === "create_dir" ||
    c.endpointName === "agent_execution" ||
    c.endpointName === "placement" ||
    c.endpointName === "agent_control"
  ) {
    return undefined;
  }
  return runtimeConnLabel(c) || undefined;
}

function specConnLabel(c: { kind?: string; protocol?: string; port?: number }): string {
  const proto = shortEnum(c.protocol, "PROTOCOL_");
  const parts = [proto].filter(Boolean);
  if (c.port) parts.push(String(c.port));
  return parts.join(":") || shortEnum(c.kind);
}

function nodeTitle(n: RuntimeNodeJson): string {
  return n.label || n.role || n.engine || n.id || "node";
}

function nodeSubtitle(n: RuntimeNodeJson): string {
  const parts = [n.engine, n.role].filter(Boolean);
  if (n.address) parts.push(n.port ? `${n.address}:${n.port}` : n.address);
  if (!parts.length && n.machineId) parts.push(n.machineId);
  return parts.join(" / ") || shortEnum(n.kind);
}

function runtimeNodeCard(n: RuntimeNodeJson, parentId: string | undefined, position: { x: number; y: number }): Node {
  const accent = runtimeKindColor(n.kind);
  const status = statusColor(n.status);
  const detail = nodeSubtitle(n);
  return {
    id: n.id!,
    parentId,
    extent: parentId ? "parent" : undefined,
    position,
    draggable: false,
    sourcePosition: Position.Right,
    targetPosition: Position.Left,
    data: {
      label: (
        <div className="flex h-full w-full items-center gap-2 px-2 py-1 text-left" title={n.statusReason || n.nodeExecutionId}>
          <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: status }} />
          <div className="min-w-0 flex-1">
            <div className="truncate font-mono text-[11px] font-semibold" style={{ color: accent }}>
              {nodeTitle(n)}
            </div>
            <div className="truncate font-mono text-[9px] text-zinc-500">{detail}</div>
          </div>
        </div>
      ),
    },
    style: {
      width: parentId ? CHILD_W : CONTROL_W,
      height: parentId ? CHILD_H : CONTROL_H,
      background: accent + "12",
      border: `1px solid ${accent}66`,
      borderLeft: `3px solid ${status}`,
      borderRadius: 4,
      padding: 0,
    },
  };
}

function runtimeMachineGroup(machine: RuntimeNodeJson, x: number, y: number, children: RuntimeNodeJson[]): Node {
  const status = statusColor(machine.status);
  const groupH = PAD_TOP + Math.max(children.length, 1) * CHILD_H + Math.max(children.length - 1, 0) * CHILD_GAP + PAD_BOTTOM;
  const host = machine.address ? (machine.port ? `${machine.address}:${machine.port}` : machine.address) : machine.machineId;
  return {
    id: machine.id!,
    position: { x, y },
    draggable: false,
    selectable: false,
    sourcePosition: Position.Right,
    targetPosition: Position.Left,
    data: {
      label: (
        <div className="absolute left-2 top-2 flex max-w-[268px] items-center gap-1.5 overflow-hidden">
          <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: status }} />
          <span className="truncate font-mono text-[10px] uppercase text-zinc-300">{machine.machineId || machine.label}</span>
          <span className="truncate font-mono text-[9px] text-zinc-600">{host}</span>
        </div>
      ),
    },
    style: {
      width: GROUP_W,
      height: groupH,
      background: "#0d0d0d",
      border: "1px solid #2a2a2a",
      borderRadius: 4,
    },
  };
}

function runtimeNodeOrder(n: RuntimeNodeJson): number {
  switch (n.kind) {
    case "KIND_AGENT":
      return 10;
    case "KIND_COMPONENT":
    case "KIND_WORKLOAD":
      return 20;
    case "KIND_MONITOR":
      return 30;
    case "KIND_EXPORTER":
      return 40;
    case "KIND_EXTERNAL":
      return 90;
    default:
      return 50;
  }
}

function buildRuntime(topo: TopologyJson): Built {
  const runtimeNodes = topo.runtimeNodes ?? [];
  const runtimeConnections = topo.runtimeConnections ?? [];
  const nodeById = new Map(runtimeNodes.filter((n) => n.id).map((n) => [n.id!, n]));

  const control = runtimeNodes.filter((n) => n.kind === "KIND_CONTROL_PLANE" && n.id);
  const machines = runtimeNodes.filter((n) => n.kind === "KIND_MACHINE" && n.id);
  const machineIds = new Set(machines.map((n) => n.id));
  const childByMachine = new Map<string, RuntimeNodeJson[]>();
  const placed = new Set<string>();

  for (const machine of machines) {
    const children = runtimeNodes
      .filter((n) => n.id && n.id !== machine.id && n.machineId === machine.machineId)
      .sort((a, b) => runtimeNodeOrder(a) - runtimeNodeOrder(b) || (a.id ?? "").localeCompare(b.id ?? ""));
    childByMachine.set(machine.id!, children);
    for (const child of children) placed.add(child.id!);
  }

  const nodes: Node[] = [];
  let x = 0;
  let controlY = CONTROL_Y;
  for (const n of control) {
    nodes.push(runtimeNodeCard(n, undefined, { x, y: controlY }));
    placed.add(n.id!);
    controlY += CONTROL_H + 24;
  }
  if (control.length) x += CONTROL_W + COL_GAP;

  for (const machine of machines) {
    const children = childByMachine.get(machine.id!) ?? [];
    nodes.push(runtimeMachineGroup(machine, x, MACHINE_Y, children));
    placed.add(machine.id!);
    let y = PAD_TOP;
    for (const child of children) {
      nodes.push(runtimeNodeCard(child, machine.id, { x: PAD_X, y }));
      y += CHILD_H + CHILD_GAP;
    }
    x += GROUP_W + COL_GAP;
  }

  const orphans = runtimeNodes
    .filter((n) => n.id && !placed.has(n.id) && !machineIds.has(n.id))
    .sort((a, b) => runtimeNodeOrder(a) - runtimeNodeOrder(b) || (a.id ?? "").localeCompare(b.id ?? ""));
  let orphanY = MACHINE_Y;
  for (const n of orphans) {
    nodes.push(runtimeNodeCard(n, undefined, { x, y: orphanY }));
    orphanY += CONTROL_H + 24;
  }

  const knownIds = new Set(nodes.map((n) => n.id));
  const edges: Edge[] = [];
  for (const c of runtimeConnections) {
    const source = c.fromNodeId ?? "";
    const target = c.toNodeId ?? "";
    if (!source || !target || !knownIds.has(source) || !knownIds.has(target)) continue;
    const color = edgeColor(c);
    const sourceNode = nodeById.get(source);
    const targetNode = nodeById.get(target);
    const label = visibleRuntimeConnLabel(c);
    const quiet = quietSupport(c);
    const title = [
      sourceNode?.label || source,
      "->",
      targetNode?.label || target,
      c.endpointName,
      c.status,
      c.phase,
      c.nodeExecutionId,
    ].filter(Boolean).join(" ");
    edges.push({
      id: c.id ?? `${source}->${target}`,
      source,
      target,
      label,
      type: quiet ? "step" : "smoothstep",
      animated: !quiet && (activeStatus(c.status) || c.mode === "MODE_STREAM"),
      markerEnd: { type: MarkerType.ArrowClosed, width: 12, height: 12, color },
      style: {
        stroke: color,
        strokeWidth: quiet ? 0.9 : activeStatus(c.status) ? 1.8 : 1.15,
        opacity: quiet ? 0.42 : 0.92,
        strokeDasharray: quiet ? "5 5" : undefined,
      },
      labelStyle: { fontSize: 9, fontFamily: "JetBrains Mono, monospace", fill: "#b8b8b8" },
      labelBgStyle: { fill: "#0a0a0a", fillOpacity: 0.94 },
      labelBgPadding: [4, 2],
      labelShowBg: Boolean(label),
      zIndex: quiet ? 0 : 5,
      data: { title },
    });
  }

  return { nodes, edges };
}

function buildSpec(topo: TopologyJson | undefined): Built {
  const spec = topo?.spec;
  if (!spec) return { nodes: [], edges: [] };

  const machines = spec.nodes ?? [];
  const components = spec.components ?? [];
  const external = spec.externalComponents ?? [];
  const connections = spec.connections ?? [];

  const compById = new Map<string, (typeof components)[number]>();
  for (const c of [...components, ...external]) if (c.id) compById.set(c.id, c);

  const placed = new Set<string>();
  const nodes: Node[] = [];
  let colX = 0;

  const compChild = (c: (typeof components)[number], parentId: string, y: number): Node => {
    const color = SPEC_KIND_COLOR[c.kind ?? "KIND_UNSPECIFIED"] ?? SPEC_KIND_COLOR.KIND_UNSPECIFIED;
    return {
      id: c.id!,
      parentId,
      extent: "parent",
      position: { x: PAD_X, y },
      draggable: false,
      data: {
        label: (
          <div className="flex w-full flex-col items-start gap-0.5 px-2 py-1 text-left">
            <span className="truncate font-mono text-[11px] font-semibold" style={{ color }}>
              {c.role || shortEnum(c.kind)}
            </span>
            <span className="truncate font-mono text-[9px] text-zinc-500">{c.engine || shortEnum(c.kind)}</span>
          </div>
        ),
      },
      style: {
        width: CHILD_W,
        height: CHILD_H,
        background: color + "14",
        border: `1px solid ${color}55`,
        borderRadius: 3,
        padding: 0,
      },
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
    };
  };

  for (const m of machines) {
    const own = (m.componentIds ?? [])
      .map((id) => compById.get(id))
      .filter((c): c is (typeof components)[number] => !!c);
    const groupH = PAD_TOP + Math.max(own.length, 1) * CHILD_H + Math.max(own.length - 1, 0) * CHILD_GAP + PAD_BOTTOM;
    const role = m.labels?.role || m.labels?.kind || "machine";
    nodes.push({
      id: `m:${m.id}`,
      position: { x: colX, y: 0 },
      draggable: false,
      selectable: false,
      data: {
        label: (
          <div className="absolute left-2 top-1.5 flex items-center gap-1.5">
            <span className="font-mono text-[10px] uppercase text-zinc-300">{m.id}</span>
            <span className="font-mono text-[9px] text-zinc-600">{role}</span>
          </div>
        ),
      },
      style: {
        width: GROUP_W,
        height: groupH,
        background: "#0d0d0d",
        border: "1px solid #2a2a2a",
        borderRadius: 4,
      },
    });
    let y = PAD_TOP;
    for (const c of own) {
      nodes.push(compChild(c, `m:${m.id}`, y));
      placed.add(c.id!);
      y += CHILD_H + CHILD_GAP;
    }
    colX += GROUP_W + COL_GAP;
  }

  const orphans = [...components, ...external].filter((c) => c.id && !placed.has(c.id));
  let y = 0;
  for (const c of orphans) {
    const color = SPEC_KIND_COLOR[c.kind ?? "KIND_UNSPECIFIED"] ?? SPEC_KIND_COLOR.KIND_UNSPECIFIED;
    nodes.push({
      id: c.id!,
      position: { x: colX, y },
      draggable: false,
      data: {
        label: (
          <div className="flex w-full flex-col items-start gap-0.5 px-2 py-1 text-left">
            <span className="truncate font-mono text-[11px] font-semibold" style={{ color }}>
              {c.role || shortEnum(c.kind)}
            </span>
            <span className="truncate font-mono text-[9px] text-zinc-500">{c.engine || shortEnum(c.kind)}</span>
          </div>
        ),
      },
      style: {
        width: CHILD_W,
        height: CHILD_H,
        background: color + "14",
        border: `1px solid ${color}55`,
        borderRadius: 3,
        padding: 0,
      },
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
    });
    y += CHILD_H + CHILD_GAP;
  }

  const knownIds = new Set(nodes.map((n) => n.id));
  const edges: Edge[] = [];
  connections.forEach((c, i) => {
    const from = c.fromComponentId ?? "";
    const to = c.toComponentId ?? "";
    if (!knownIds.has(from) || !knownIds.has(to)) return;
    const color = CONN_COLOR[c.kind ?? "KIND_UNSPECIFIED"] ?? CONN_COLOR.KIND_UNSPECIFIED;
    edges.push({
      id: `c-${i}`,
      source: from,
      target: to,
      label: specConnLabel(c),
      type: "smoothstep",
      animated: c.kind === "KIND_FLOW" || c.kind === "KIND_REPLICATION",
      markerEnd: { type: MarkerType.ArrowClosed, width: 12, height: 12, color },
      style: { stroke: color, strokeWidth: 1.2 },
      labelStyle: { fontSize: 9, fontFamily: "JetBrains Mono, monospace", fill: "#a3a3a3" },
      labelBgStyle: { fill: "#0a0a0a", fillOpacity: 0.92 },
      labelBgPadding: [4, 2],
    });
  });

  return { nodes, edges };
}

function build(topo: TopologyJson | undefined): Built {
  if ((topo?.runtimeNodes?.length ?? 0) > 0) {
    return buildRuntime(topo!);
  }
  return buildSpec(topo);
}

export function TopologyViewer({ topology }: { topology?: TopologyJson }) {
  const { nodes, edges } = useMemo(() => build(topology), [topology]);

  if (nodes.length === 0) {
    return (
      <div className="flex h-full items-center justify-center py-12 font-mono text-xs text-muted-foreground">
        Topology not available for this run
      </div>
    );
  }

  return (
    <div className="h-full w-full">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        fitView
        fitViewOptions={{ padding: 0.18 }}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        panOnDrag
        zoomOnScroll
        minZoom={0.25}
        maxZoom={2}
        proOptions={{ hideAttribution: true }}
        colorMode="dark"
      >
        <Background variant={BackgroundVariant.Dots} gap={20} size={1} color="#1a1a1a" />
        <Controls showInteractive={false} className="!border-border !bg-card" />
      </ReactFlow>
    </div>
  );
}
