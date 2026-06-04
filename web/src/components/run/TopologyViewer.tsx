// TOPOLOGY VIEWER — renders the staged topology envelope as an interactive
// xyflow graph. Each logical machine (topology Node) is a parent group box that
// contains its colocated components (Component); external components sit in a
// trailing column; connections (Connection) are drawn as edges, coloured and
// labelled by their kind/protocol/port. Pure read-only view (no drag/connect).
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
import type { TopologyJson } from "@/lib/proto/cloud/v1/topology/topology_pb";

// Component-kind → accent colour (hex). Mirrors the run-status / db palette.
const KIND_COLOR: Record<string, string> = {
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
  KIND_SUPPORT: "#525252",
  KIND_UNSPECIFIED: "#525252",
};

const COMP_W = 168;
const COMP_H = 46;
const COMP_GAP = 12;
const PAD_X = 14;
const PAD_TOP = 34; // room for the machine label
const PAD_BOTTOM = 14;
const COL_GAP = 90;

function shortKind(k?: string): string {
  return (k ?? "").replace(/^KIND_/, "").toLowerCase();
}
function connLabel(c: { kind?: string; protocol?: string; port?: number }): string {
  const proto = (c.protocol ?? "").replace(/^PROTOCOL_/, "").toLowerCase();
  const parts = [proto].filter(Boolean);
  if (c.port) parts.push(String(c.port));
  return parts.join(":") || shortKind(c.kind);
}

interface Built {
  nodes: Node[];
  edges: Edge[];
}

function build(topo: TopologyJson | undefined): Built {
  const spec = topo?.spec;
  if (!spec) return { nodes: [], edges: [] };

  const machines = spec.nodes ?? [];
  const components = spec.components ?? [];
  const external = spec.externalComponents ?? [];
  const connections = spec.connections ?? [];

  const compById = new Map<string, (typeof components)[number]>();
  for (const c of [...components, ...external]) if (c.id) compById.set(c.id, c);

  // compId -> the xyflow node id that represents it (child node id == comp id).
  const placed = new Set<string>();
  const nodes: Node[] = [];

  const machineColW = COMP_W + PAD_X * 2;
  let colX = 0;

  const compChild = (c: (typeof components)[number], parentId: string, y: number): Node => {
    const color = KIND_COLOR[c.kind ?? "KIND_UNSPECIFIED"] ?? KIND_COLOR.KIND_UNSPECIFIED;
    const title = c.role || shortKind(c.kind);
    const sub = c.engine || shortKind(c.kind);
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
              {title}
            </span>
            <span className="truncate font-mono text-[9px] text-zinc-500">{sub}</span>
          </div>
        ),
      },
      style: {
        width: COMP_W,
        height: COMP_H,
        background: color + "14",
        border: `1px solid ${color}55`,
        borderRadius: 3,
        padding: 0,
      },
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
    };
  };

  // Machine group boxes with their colocated components.
  for (const m of machines) {
    const own = (m.componentIds ?? [])
      .map((id) => compById.get(id))
      .filter((c): c is (typeof components)[number] => !!c);
    const inner = Math.max(own.length, 1);
    const groupH = PAD_TOP + inner * COMP_H + (inner - 1) * COMP_GAP + PAD_BOTTOM;
    const role = m.labels?.role || m.labels?.kind || "machine";
    nodes.push({
      id: `m:${m.id}`,
      position: { x: colX, y: 0 },
      draggable: false,
      selectable: false,
      data: {
        label: (
          <div className="absolute left-2 top-1.5 flex items-center gap-1.5">
            <span className="font-mono text-[10px] uppercase tracking-wider text-zinc-300">
              {m.id}
            </span>
            <span className="font-mono text-[9px] text-zinc-600">{role}</span>
          </div>
        ),
      },
      style: {
        width: machineColW,
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
      y += COMP_H + COMP_GAP;
    }
    colX += machineColW + COL_GAP;
  }

  // Components/external not colocated on any machine → standalone column.
  const orphans = [...components, ...external].filter((c) => c.id && !placed.has(c.id));
  if (orphans.length) {
    let y = 0;
    for (const c of orphans) {
      const color = KIND_COLOR[c.kind ?? "KIND_UNSPECIFIED"] ?? KIND_COLOR.KIND_UNSPECIFIED;
      nodes.push({
        id: c.id!,
        position: { x: colX, y },
        draggable: false,
        data: {
          label: (
            <div className="flex w-full flex-col items-start gap-0.5 px-2 py-1 text-left">
              <span className="truncate font-mono text-[11px] font-semibold" style={{ color }}>
                {c.role || shortKind(c.kind)}
              </span>
              <span className="truncate font-mono text-[9px] text-zinc-500">
                {c.engine || shortKind(c.kind)}
              </span>
            </div>
          ),
        },
        style: {
          width: COMP_W,
          height: COMP_H,
          background: color + "14",
          border: `1px solid ${color}55`,
          borderRadius: 3,
          padding: 0,
        },
        sourcePosition: Position.Right,
        targetPosition: Position.Left,
      });
      y += COMP_H + COMP_GAP;
    }
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
      label: connLabel(c),
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
        fitViewOptions={{ padding: 0.2 }}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        panOnDrag
        zoomOnScroll
        minZoom={0.3}
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
