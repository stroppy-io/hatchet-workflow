import { useMemo } from "react";
import { Background, Controls, ReactFlow, type Edge, type Node } from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import { statusLabel } from "@/lib/format";
import { Status } from "@/lib/proto/cloud/v1/runtime/primitive/status_pb.ts";
import type { Dag } from "@/lib/proto/cloud/v1/runtime/primitive/dag_pb.ts";

const STATUS_COLOR: Record<number, string> = {
  [Status.PENDING]: "#52525b",
  [Status.RUNNING]: "#38bdf8",
  [Status.RETRY_WAIT]: "#f59e0b",
  [Status.COMPLETED]: "#22c55e",
  [Status.FAILED]: "#ef4444",
  [Status.CANCELLING]: "#f59e0b",
  [Status.CANCELLED]: "#71717a",
  [Status.SKIPPED]: "#3f3f46",
};

// Read-only DAG graph for a run's execution Dag. Layers nodes left-to-right by
// longest-path depth from the edges; node border color encodes status.
export function DagGraph({ dag, height = 520 }: { dag: Dag; height?: number }) {
  const { nodes, edges } = useMemo(() => {
    const depth = new Map<string, number>();
    dag.nodes.forEach((node) => depth.set(node.id, 0));
    // Fixpoint over edges (small dags): to.depth = max(from.depth + 1).
    for (let i = 0; i < dag.nodes.length; i += 1) {
      for (const edge of dag.edges) {
        const next = (depth.get(edge.source) ?? 0) + 1;
        if (next > (depth.get(edge.target) ?? 0)) depth.set(edge.target, next);
      }
    }
    const rowPerDepth = new Map<number, number>();
    const flowNodes: Node[] = dag.nodes.map((node) => {
      const d = depth.get(node.id) ?? 0;
      const row = rowPerDepth.get(d) ?? 0;
      rowPerDepth.set(d, row + 1);
      const color = STATUS_COLOR[node.status] ?? "#52525b";
      return {
        id: node.id,
        position: { x: d * 250, y: row * 88 },
        data: { label: `${node.executionId || node.id}\n${statusLabel(node.status)}` },
        style: { border: `1px solid ${color}`, background: "#111113", color: "#f4f4f5", borderRadius: 6, fontSize: 11, width: 210, whiteSpace: "pre-wrap" as const },
      };
    });
    const flowEdges: Edge[] = dag.edges.map((edge, index) => ({
      id: edge.id || `edge-${index}`,
      source: edge.source,
      target: edge.target,
      style: { stroke: "#3f3f46" },
    }));
    return { nodes: flowNodes, edges: flowEdges };
  }, [dag]);

  if (nodes.length === 0) {
    return <div className="flex items-center justify-center rounded-md border text-sm text-muted-foreground" style={{ height }}>Empty DAG</div>;
  }

  return (
    <div className="overflow-hidden rounded-md border" style={{ height }}>
      <ReactFlow nodes={nodes} edges={edges} fitView nodesDraggable={false} nodesConnectable={false} elementsSelectable={false} proOptions={{ hideAttribution: true }}>
        <Background color="#27272a" gap={16} />
        <Controls showInteractive={false} />
      </ReactFlow>
    </div>
  );
}
