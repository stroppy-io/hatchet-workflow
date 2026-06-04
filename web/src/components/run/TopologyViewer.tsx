// TOPOLOGY VIEWER — interactive xyflow graph of the run's topology.
//
// Deterministic, generic layout (no per-topology assumptions):
//   • machines are stacked vertically on the LEFT; their components stack
//     vertically inside each machine box;
//   • the control plane (server) sits ALONE on the right;
//   • intra-machine connections route on the LEFT edge of the cards (a tidy
//     side-rail), cross-machine / server connections route on the RIGHT.
// Cards are custom nodes with explicit left+right source/target Handles so
// edges attach to the correct side. Runtime topology is preferred; the logical
// TopologySpec is a fallback for early snapshots.
import { memo, useMemo } from "react";
import {
  Background,
  BackgroundVariant,
  Controls,
  Handle,
  MarkerType,
  Position,
  ReactFlow,
  type Edge,
  type Node,
  type NodeProps,
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

// Layout geometry.
const CHILD_W = 248;
const CHILD_H = 52;
const CHILD_GAP = 14;
const GROUP_PAD_X = 16;
const GROUP_PAD_TOP = 38;
const GROUP_PAD_BOTTOM = 16;
const RAIL_W = 150; // inner lane (away from server): intra/same-side edges fan into tracks
const RAIL_TRACK = 16; // horizontal spacing between fanned rail tracks
const GROUP_W = CHILD_W + GROUP_PAD_X + RAIL_W; // box hosts the rail inside it
const MACHINE_GAP = 52;
const COL_GAP = 230; // machine column ↔ central server column
const CONTROL_W = 264;
const CONTROL_H = 64;

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
  if (
    edge.kind === "KIND_SUPPORT" &&
    (relation === "agent_control" ||
      relation === "agent_execution" ||
      relation === "placement" ||
      relation === "depends_on" ||
      relation === "agent_action")
  ) {
    return true;
  }
  // Defensive: any agent control-protocol op (write_file/create_dir/call_cmd)
  // is plumbing regardless of how the backend labelled it.
  if (edge.protocol === "PROTOCOL_CONTROL") {
    const ep = edge.endpointName ?? "";
    if (ep.startsWith("write_file") || ep.startsWith("fetch_file") || ep === "create_dir" || ep === "call_cmd" || ep === "agent_action") {
      return true;
    }
  }
  return false;
}

function edgeColor(edge: RuntimeConnectionJson): string {
  const status = edge.status ?? "";
  if (status === "STATUS_FAILED" || status === "STATUS_CANCELLED" || status === "STATUS_CANCELLING") return statusColor(status);
  if (quietSupport(edge)) return "#4d4d4d";
  if (status === "STATUS_RUNNING" || status === "STATUS_DEPLOYMENT" || status === "STATUS_ALLOCATED") return statusColor(status);
  return CONN_COLOR[edge.kind ?? "KIND_UNSPECIFIED"] ?? CONN_COLOR.KIND_UNSPECIFIED;
}

function activeStatus(status?: string): boolean {
  return status === "STATUS_RUNNING" || status === "STATUS_DEPLOYMENT" || status === "STATUS_ALLOCATED";
}

function compactEndpoint(endpoint?: string): string {
  const value = endpoint ?? "";
  if (value === "/insert/0/prometheus/api/v1/write") return "metrics";
  if (value === "/insert/jsonline") return "logs";
  if (value === "/api/binaries") return "binaries";
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

// ── Custom node components (left + right source/target handles) ────────────

interface CardData {
  accent: string;
  status: string;
  title: string;
  subtitle: string;
  hint?: string;
}
interface GroupData {
  title: string;
  host?: string;
  status: string;
}

const portCls = "!h-1.5 !w-1.5 !min-w-0 !border-0 !bg-zinc-600 !opacity-0";

// Two source + two target handles (one pair per side) so edges can choose the
// side that faces their counterpart. Offset vertically so they don't overlap.
function SideHandles() {
  return (
    <>
      <Handle id="tl" type="target" position={Position.Left} className={portCls} style={{ top: "34%" }} />
      <Handle id="sl" type="source" position={Position.Left} className={portCls} style={{ top: "66%" }} />
      <Handle id="tr" type="target" position={Position.Right} className={portCls} style={{ top: "34%" }} />
      <Handle id="sr" type="source" position={Position.Right} className={portCls} style={{ top: "66%" }} />
    </>
  );
}

const CardNode = memo(function CardNode({ data }: NodeProps) {
  const d = data as unknown as CardData;
  return (
    <div className="flex h-full w-full items-center gap-2 px-2.5" title={d.hint}>
      <SideHandles />
      <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: d.status }} />
      <div className="min-w-0 flex-1">
        <div className="truncate font-mono text-[11px] font-semibold" style={{ color: d.accent }}>{d.title}</div>
        <div className="truncate font-mono text-[9px] text-zinc-500">{d.subtitle}</div>
      </div>
    </div>
  );
});

const GroupNode = memo(function GroupNode({ data }: NodeProps) {
  const d = data as unknown as GroupData;
  return (
    <div className="h-full w-full">
      <div className="absolute left-2.5 top-2 flex items-center gap-1.5">
        <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: d.status }} />
        <span className="truncate font-mono text-[10px] uppercase tracking-wide text-zinc-300">{d.title}</span>
        {d.host && <span className="truncate font-mono text-[9px] text-zinc-600">{d.host}</span>}
      </div>
    </div>
  );
});

interface ServerData extends CardData {
  slotsL: number;
  slotsR: number;
}

// Distribute `count` slots around the hemisphere of the server that faces a
// machine column: top corner → side edge → bottom corner. So arrival points use
// the top, side AND bottom of the server, not a single crowded edge.
function hemiSlot(side: "l" | "r", i: number, count: number): { position: Position; style: { left?: string; top?: string } } {
  const topN = Math.min(Math.floor(count / 4), 4);
  const botN = Math.min(Math.floor(count / 4), 4);
  const midN = Math.max(count - topN - botN, 0);
  const edgeBase = side === "l" ? 8 : 56; // % along the top/bottom edge (left vs right half)
  const span = 36;
  if (i < topN) {
    return { position: Position.Top, style: { left: `${edgeBase + ((i + 1) / (topN + 1)) * span}%` } };
  }
  if (i < topN + midN) {
    const j = i - topN;
    return { position: side === "l" ? Position.Left : Position.Right, style: { top: `${((j + 1) / (midN + 1)) * 100}%` } };
  }
  const j = i - topN - midN;
  return { position: Position.Bottom, style: { left: `${edgeBase + ((j + 1) / (botN + 1)) * span}%` } };
}

// Server exposes as many handle slots per facing side as there are edges from
// that side, spread around the hemisphere perimeter — converging edges land on
// distinct points instead of one blob.
const ServerNode = memo(function ServerNode({ data }: NodeProps) {
  const d = data as unknown as ServerData;
  const banks: ReturnType<typeof Handle>[] = [];
  ([["l", d.slotsL] as const, ["r", d.slotsR] as const]).forEach(([side, n]) => {
    const count = Math.max(n, 1);
    for (let i = 0; i < count; i++) {
      const { position, style } = hemiSlot(side, i, count);
      banks.push(<Handle key={`${side}t${i}`} id={`${side}t${i}`} type="target" position={position} className={portCls} style={style} />);
      banks.push(<Handle key={`${side}s${i}`} id={`${side}s${i}`} type="source" position={position} className={portCls} style={style} />);
    }
  });
  return (
    <div className="flex h-full w-full items-center gap-2 px-2.5" title={d.hint}>
      {banks}
      <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: d.status }} />
      <div className="min-w-0 flex-1">
        <div className="truncate font-mono text-[11px] font-semibold" style={{ color: d.accent }}>{d.title}</div>
        <div className="truncate font-mono text-[9px] text-zinc-500">{d.subtitle}</div>
      </div>
    </div>
  );
});

const nodeTypes = { card: CardNode, group: GroupNode, server: ServerNode };

interface Built {
  nodes: Node[];
  edges: Edge[];
}

// x-facing handle pick for the simple (single-column) spec fallback.
function pickHandles(srcX: number, tgtX: number, internal: boolean): { sourceHandle: string; targetHandle: string } {
  if (internal) return { sourceHandle: "sl", targetHandle: "tl" };
  if (tgtX >= srcX) return { sourceHandle: "sr", targetHandle: "tl" };
  return { sourceHandle: "sl", targetHandle: "tr" };
}

function makeEdge(
  id: string,
  source: string,
  target: string,
  handles: { sourceHandle: string; targetHandle: string },
  color: string,
  label: string | undefined,
  opts: { quiet?: boolean; animated?: boolean; offset?: number; title?: string; behind?: boolean } = {},
): Edge {
  const quiet = !!opts.quiet;
  return {
    id,
    source,
    target,
    sourceHandle: handles.sourceHandle,
    targetHandle: handles.targetHandle,
    label,
    data: opts.title ? { title: opts.title } : undefined,
    type: "smoothstep",
    pathOptions: { borderRadius: 10, offset: opts.offset ?? 20 },
    animated: !quiet && !!opts.animated,
    markerEnd: { type: MarkerType.ArrowClosed, width: 11, height: 11, color },
    style: {
      stroke: color,
      strokeWidth: quiet ? 0.9 : 1.3,
      opacity: quiet ? 0.3 : 0.92,
      strokeDasharray: quiet ? "4 4" : undefined,
    },
    labelStyle: { fontSize: 9, fontFamily: "JetBrains Mono, monospace", fill: "#c8c8c8" },
    labelBgStyle: { fill: "#0a0a0a", fillOpacity: 0.96 },
    labelBgPadding: [4, 2],
    labelShowBg: Boolean(label),
    zIndex: opts.behind ? 0 : quiet ? 0 : 6,
  } as Edge;
}

function buildRuntime(topo: TopologyJson): Built {
  const runtimeNodes = (topo.runtimeNodes ?? []).filter((n) => n.id);
  const runtimeConnections = topo.runtimeConnections ?? [];

  const machines = runtimeNodes.filter((n) => n.kind === "KIND_MACHINE");
  const machineIds = new Set(machines.map((n) => n.id!));
  const controls = runtimeNodes.filter((n) => n.kind === "KIND_CONTROL_PLANE");
  const controlIds = new Set(controls.map((n) => n.id!));

  const childrenOf = new Map<string, RuntimeNodeJson[]>();
  const machineByMid = new Map<string, RuntimeNodeJson>();
  for (const m of machines) if (m.machineId) machineByMid.set(m.machineId, m);
  for (const m of machines) childrenOf.set(m.id!, []);
  const orphanNodes: RuntimeNodeJson[] = [];
  // Order so monitoring sources sit above their sinks (exporters above vmagent,
  // vector last) → internal edges mostly point downward, short spans.
  const order = (n: RuntimeNodeJson) =>
    n.kind === "KIND_AGENT" ? 0
      : n.kind === "KIND_WORKLOAD" || n.kind === "KIND_COMPONENT" ? 1
      : n.kind === "KIND_EXPORTER" ? 2
      : n.kind === "KIND_MONITOR" ? 3
      : 4;
  for (const n of runtimeNodes) {
    if (machineIds.has(n.id!) || controlIds.has(n.id!)) continue;
    const parent = n.machineId ? machineByMid.get(n.machineId) : undefined;
    if (parent && parent.id !== n.id) childrenOf.get(parent.id!)!.push(n);
    else orphanNodes.push(n);
  }
  for (const arr of childrenOf.values())
    arr.sort((a, b) => order(a) - order(b) || (a.id ?? "").localeCompare(b.id ?? ""));

  // Group machines by cross-machine connectivity (union-find over edges whose
  // endpoints live on different machines). A connected group (e.g. an
  // etcd+patroni+replica DB cluster) is kept together on ONE side so its
  // cross-machine edges stay short; groups are then balanced across the two
  // sides of the central server.
  type Side = "L" | "R";
  const machineOfComp = new Map<string, string>(); // component id → machine id
  for (const m of machines) for (const k of childrenOf.get(m.id!) ?? []) machineOfComp.set(k.id!, m.id!);
  const uf = new Map<string, string>();
  const find = (x: string): string => {
    let r = x;
    while (uf.get(r) && uf.get(r) !== r) r = uf.get(r)!;
    uf.set(x, r);
    return r;
  };
  const union = (a: string, b: string) => {
    uf.set(find(a), find(b));
  };
  for (const m of machines) uf.set(m.id!, m.id!);
  for (const c of runtimeConnections) {
    if (quietSupport(c)) continue;
    const ma = machineOfComp.get(c.fromNodeId ?? "");
    const mb = machineOfComp.get(c.toNodeId ?? "");
    if (ma && mb && ma !== mb) union(ma, mb);
  }
  const groups = new Map<string, string[]>();
  for (const m of machines) {
    const r = find(m.id!);
    (groups.get(r) ?? groups.set(r, []).get(r)!).push(m.id!);
  }
  // Assign each group to the currently-lighter side (balance machine counts);
  // machines of a group stay contiguous in that side's column.
  const sideOfMachine = new Map<string, Side>();
  const orderL: string[] = [];
  const orderR: string[] = [];
  const ordered = [...groups.values()].sort((a, b) => b.length - a.length);
  let loadL = 0;
  let loadR = 0;
  for (const grp of ordered) {
    const side: Side = loadL <= loadR ? "L" : "R";
    if (side === "L") loadL += grp.length;
    else loadR += grp.length;
    for (const mid of grp) {
      sideOfMachine.set(mid, side);
      (side === "L" ? orderL : orderR).push(mid);
    }
  }
  const machineById = new Map(machines.map((m) => [m.id!, m]));
  const sideOfNode = new Map<string, Side>(); // node id → its column side

  const machineH = (n: number) => GROUP_PAD_TOP + Math.max(n, 1) * CHILD_H + Math.max(n - 1, 0) * CHILD_GAP + GROUP_PAD_BOTTOM;

  const nodes: Node[] = [];
  // Column x positions: left boxes | gap | server | gap | right boxes.
  const xLeft = 0;
  const xServer = GROUP_W + COL_GAP;
  const xRight = xServer + CONTROL_W + COL_GAP;

  const place = (side: Side, startX: number, mids: string[]) => {
    let y = 0;
    for (const mid of mids) {
      const m = machineById.get(mid);
      if (!m) continue;
      const kids = childrenOf.get(m.id!) ?? [];
      const h = machineH(kids.length);
      const host = m.address ? (m.port ? `${m.address}:${m.port}` : m.address) : m.machineId;
      nodes.push({
        id: m.id!,
        type: "group",
        position: { x: startX, y },
        draggable: false,
        selectable: false,
        data: { title: m.machineId || m.label || m.id || "machine", host, status: statusColor(m.status) } satisfies GroupData as unknown as Record<string, unknown>,
        style: { width: GROUP_W, height: h, background: "#0d0d0d", border: "1px solid #2a2a2a", borderRadius: 6 },
      });
      sideOfNode.set(m.id!, side);
      // Cards sit toward the server; the RAIL_W lane (away from server, inside
      // the box) hosts the intra-machine edges.
      const childX = side === "L" ? RAIL_W : GROUP_PAD_X;
      kids.forEach((k, i) => {
        nodes.push(cardNode(k, { x: childX, y: GROUP_PAD_TOP + i * (CHILD_H + CHILD_GAP) }, { w: CHILD_W, h: CHILD_H }, m.id!));
        sideOfNode.set(k.id!, side);
      });
      y += h + MACHINE_GAP;
    }
    return Math.max(y - MACHINE_GAP, 0);
  };

  const hL = place("L", xLeft, orderL);
  const hR = place("R", xRight, orderR);
  const totalH = Math.max(hL, hR, CONTROL_H);

  // Count server-bound edges per side so the server can be made tall enough to
  // spread its arrival points out (not bunched on a few pixels).
  let nL = 0;
  let nR = 0;
  for (const c of runtimeConnections) {
    if (quietSupport(c)) continue;
    const a = c.fromNodeId ?? "";
    const b = c.toNodeId ?? "";
    const aS = controlIds.has(a);
    const bS = controlIds.has(b);
    if (aS === bS) continue;
    const m = machineOfComp.get(aS ? b : a);
    if ((m ? sideOfMachine.get(m) : "R") === "R") nR++;
    else nL++;
  }
  const serverH = Math.max(CONTROL_H, Math.min(Math.max(nL, nR) * 24 + 20, Math.max(totalH, 160)));

  // Server(s) — centered between the two machine columns; "server" node type
  // renders a bank of handles spread over its full height so converging edges
  // land on distinct points.
  controls.forEach((c, i) => {
    const y = totalH / 2 - serverH / 2 + i * (serverH + 24);
    const node = cardNode(c, { x: xServer, y }, { w: CONTROL_W, h: serverH });
    node.type = "server";
    (node.data as Record<string, unknown>).slotsL = nL;
    (node.data as Record<string, unknown>).slotsR = nR;
    nodes.push(node);
  });

  // Orphans / externals — stacked under the right column.
  let orphanY = hR + MACHINE_GAP;
  for (const n of orphanNodes) {
    nodes.push(cardNode(n, { x: xRight, y: orphanY }, { w: CHILD_W, h: CHILD_H }));
    sideOfNode.set(n.id!, "R");
    orphanY += CHILD_H + CHILD_GAP;
  }

  // ── edges ──
  const known = new Set(nodes.map((n) => n.id));

  // server-facing handle suffix for a node, given its partner's side.
  const facing = (id: string, partnerSide: Side): "l" | "r" => {
    const s = sideOfNode.get(id);
    if (s === "L") return "r"; // left column faces right (toward center server)
    if (s === "R") return "l"; // right column faces left
    return partnerSide === "L" ? "l" : "r"; // server: face the partner
  };
  const railSuffix = (id: string): "l" | "r" => (sideOfNode.get(id) === "R" ? "r" : "l");

  // Server has banks of handles per side; hand each edge touching it a fresh
  // slot so they fan out instead of converging on one point.
  const serverSlot = new Map<string, number>();
  const endHandle = (id: string, isSource: boolean, partnerSide: Side): string => {
    if (controlIds.has(id)) {
      const side = partnerSide === "L" ? "l" : "r";
      const count = Math.max(side === "l" ? nL : nR, 1);
      const i = serverSlot.get(side) ?? 0;
      serverSlot.set(side, i + 1);
      return `${side}${isSource ? "s" : "t"}${i % count}`;
    }
    return `${isSource ? "s" : "t"}${facing(id, partnerSide)}`;
  };

  const seenLabel = new Set<string>();
  const railTrack = new Map<string, number>(); // side → next fan track for rail edges
  const edges: Edge[] = [];
  for (const c of runtimeConnections) {
    const source = c.fromNodeId ?? "";
    const target = c.toNodeId ?? "";
    if (!source || !target || source === target || !known.has(source) || !known.has(target)) continue;
    const sSrc = sideOfNode.get(source);
    const sTgt = sideOfNode.get(target);
    // INTRA-machine edges route on the inner left rail. Everything else
    // (cross-machine and server traffic) faces the server side / center.
    const mSrc = machineOfComp.get(source);
    const mTgt = machineOfComp.get(target);
    const internal = !!mSrc && mSrc === mTgt;
    const crossSameSide = !!mSrc && !!mTgt && mSrc !== mTgt && !!sSrc && sSrc === sTgt;
    const quiet = quietSupport(c);

    let handles: { sourceHandle: string; targetHandle: string };
    let offset: number;
    let behind = false;
    let label: string | undefined;
    if (internal) {
      const r = railSuffix(source);
      handles = { sourceHandle: `s${r}`, targetHandle: `t${r}` };
      // Fan each rail edge onto its own track so they don't pile into one line.
      if (quiet) {
        offset = 10;
      } else {
        const t = railTrack.get(sSrc!) ?? 0;
        railTrack.set(sSrc!, t + 1);
        offset = 22 + t * RAIL_TRACK;
      }
      label = quiet ? undefined : visibleRuntimeConnLabel(c);
    } else if (crossSameSide) {
      // Cross-machine, same column → a back-bus. Cards sit RAIL_W in from the
      // box edge, so an offset > RAIL_W pushes the stub out PAST the left edge
      // into a gutter to the LEFT of the boxes; low z keeps it behind them.
      const r = railSuffix(source);
      handles = { sourceHandle: `s${r}`, targetHandle: `t${r}` };
      offset = RAIL_W + 56;
      behind = true;
      label = visibleRuntimeConnLabel(c);
    } else {
      handles = {
        sourceHandle: endHandle(source, true, (sTgt ?? "L") as Side),
        targetHandle: endHandle(target, false, (sSrc ?? "L") as Side),
      };
      offset = 26;
      label = visibleRuntimeConnLabel(c);
    }

    if (label) {
      const k = `${source}|${target}|${label}`;
      if (seenLabel.has(k)) label = undefined;
      else seenLabel.add(k);
    }
    edges.push(
      makeEdge(
        c.id ?? `${source}->${target}-${edges.length}`,
        source,
        target,
        handles,
        edgeColor(c),
        label,
        { quiet, animated: activeStatus(c.status) || c.mode === "MODE_STREAM", offset, behind, title: runtimeConnLabel(c) },
      ),
    );
  }

  return { nodes, edges };
}

function cardNode(
  n: RuntimeNodeJson,
  pos: { x: number; y: number },
  size: { w: number; h: number },
  parentId?: string,
): Node {
  const accent = runtimeKindColor(n.kind);
  const status = statusColor(n.status);
  return {
    id: n.id!,
    type: "card",
    parentId,
    extent: parentId ? "parent" : undefined,
    position: pos,
    draggable: false,
    data: {
      accent,
      status,
      title: nodeTitle(n),
      subtitle: nodeSubtitle(n),
      hint: n.statusReason || n.nodeExecutionId || undefined,
    } satisfies CardData as unknown as Record<string, unknown>,
    style: {
      width: size.w,
      height: size.h,
      background: accent + "12",
      border: `1px solid ${accent}55`,
      borderLeft: `3px solid ${status}`,
      borderRadius: 4,
    },
  };
}

// ── Spec fallback (logical topology) — same stacked layout ─────────────────

function buildSpec(topo: TopologyJson | undefined): Built {
  const spec = topo?.spec;
  if (!spec) return { nodes: [], edges: [] };

  const machines = spec.nodes ?? [];
  const components = spec.components ?? [];
  const external = spec.externalComponents ?? [];
  const connections = spec.connections ?? [];
  const compById = new Map<string, (typeof components)[number]>();
  for (const c of [...components, ...external]) if (c.id) compById.set(c.id, c);

  const machineOfComp = new Map<string, string>();
  for (const m of machines) for (const cid of m.componentIds ?? []) if (m.id) machineOfComp.set(cid, m.id);

  const absX = new Map<string, number>();
  const nodes: Node[] = [];
  let cursorY = 0;
  const machineH = (n: number) => GROUP_PAD_TOP + Math.max(n, 1) * CHILD_H + Math.max(n - 1, 0) * CHILD_GAP + GROUP_PAD_BOTTOM;

  for (const m of machines) {
    if (!m.id) continue;
    const own = (m.componentIds ?? []).map((id) => compById.get(id)).filter((c): c is (typeof components)[number] => !!c);
    const h = machineH(own.length);
    const gid = `m:${m.id}`;
    nodes.push({
      id: gid,
      type: "group",
      position: { x: 0, y: cursorY },
      draggable: false,
      selectable: false,
      data: { title: m.id, status: "#71717a" } satisfies GroupData as unknown as Record<string, unknown>,
      style: { width: GROUP_W, height: h, background: "#0d0d0d", border: "1px solid #2a2a2a", borderRadius: 6 },
    });
    absX.set(gid, 0);
    own.forEach((c, i) => {
      const color = SPEC_KIND_COLOR[c.kind ?? "KIND_UNSPECIFIED"] ?? SPEC_KIND_COLOR.KIND_UNSPECIFIED;
      nodes.push(specCard(c.id!, color, c.role || shortEnum(c.kind), c.engine || shortEnum(c.kind), { x: GROUP_PAD_X, y: GROUP_PAD_TOP + i * (CHILD_H + CHILD_GAP) }, gid));
      absX.set(c.id!, GROUP_PAD_X);
    });
    cursorY += h + MACHINE_GAP;
  }

  const leftHeight = Math.max(cursorY - MACHINE_GAP, CHILD_H);
  const orphans = [...components, ...external].filter((c) => c.id && !machineOfComp.has(c.id));
  let oy = leftHeight / 2;
  for (const c of orphans) {
    const color = SPEC_KIND_COLOR[c.kind ?? "KIND_UNSPECIFIED"] ?? SPEC_KIND_COLOR.KIND_UNSPECIFIED;
    nodes.push(specCard(c.id!, color, c.role || shortEnum(c.kind), c.engine || shortEnum(c.kind), { x: GROUP_W + COL_GAP, y: oy }));
    absX.set(c.id!, GROUP_W + COL_GAP);
    oy += CHILD_H + CHILD_GAP;
  }

  const known = new Set(nodes.map((n) => n.id));
  const edges: Edge[] = connections
    .filter((c) => known.has(c.fromComponentId ?? "") && known.has(c.toComponentId ?? ""))
    .map((c, i) => {
      const from = c.fromComponentId!;
      const to = c.toComponentId!;
      const internal = !!machineOfComp.get(from) && machineOfComp.get(from) === machineOfComp.get(to);
      return makeEdge(
        `c-${i}`,
        from,
        to,
        pickHandles(absX.get(from) ?? 0, absX.get(to) ?? 0, internal),
        CONN_COLOR[c.kind ?? "KIND_UNSPECIFIED"] ?? CONN_COLOR.KIND_UNSPECIFIED,
        specConnLabel(c),
        { animated: c.kind === "KIND_FLOW" || c.kind === "KIND_REPLICATION" },
      );
    });

  return { nodes, edges };
}

function specCard(id: string, color: string, title: string, subtitle: string, pos: { x: number; y: number }, parentId?: string): Node {
  return {
    id,
    type: "card",
    parentId,
    extent: parentId ? "parent" : undefined,
    position: pos,
    draggable: false,
    data: { accent: color, status: "#71717a", title, subtitle } satisfies CardData as unknown as Record<string, unknown>,
    style: { width: CHILD_W, height: CHILD_H, background: color + "12", border: `1px solid ${color}55`, borderRadius: 4 },
  };
}

function build(topo: TopologyJson | undefined): Built {
  if ((topo?.runtimeNodes?.length ?? 0) > 0) return buildRuntime(topo!);
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
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.16 }}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        panOnDrag
        zoomOnScroll
        minZoom={0.2}
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
