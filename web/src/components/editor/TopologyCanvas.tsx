import React, { useMemo, useCallback } from "react"
import { useStore } from "@nanostores/react"
import {
  ReactFlow,
  Background,
  Controls,
  type Node,
  type Edge,
  type NodeTypes,
  type NodeProps,
  Handle,
  Position,
} from "@xyflow/react"
import {
  Database,
  Shield,
  Activity,
  Archive,
  Layers,
} from "lucide-react"
import { Badge } from "@/components/ui/badge"
import {
  $currentTest,
  $selectedNodeId,
  $validationErrors,
  type PostgresNode,
} from "@/stores/editor"

// ── Custom Node ────────────────────────────────────────────────

type DbNodeData = {
  [key: string]: unknown
  name: string
  hardware: { cores: number; memory: number; disk: number }
  role: "MASTER" | "REPLICA"
  hasPgbouncer: boolean
  hasEtcd: boolean
  hasMonitoring: boolean
  hasBackup: boolean
  hasError: boolean
}

function DatabaseNodeComponent({ data, selected }: NodeProps<Node<DbNodeData>>) {
  const isMaster = data.role === "MASTER"
  const borderColor = data.hasError
    ? "border-destructive"
    : selected
      ? "border-primary"
      : "border-border"

  return (
    <div
      className={`border bg-card min-w-[160px] ${borderColor} transition-colors`}
    >
      <Handle type="target" position={Position.Top} className="!bg-primary !w-2 !h-2" />

      <div
        className={`flex items-center gap-1.5 px-2 py-1 border-b ${
          isMaster ? "bg-primary/10 border-primary/20" : "bg-secondary/30 border-border"
        }`}
      >
        <Database className={`h-3 w-3 ${isMaster ? "text-primary" : "text-muted-foreground"}`} />
        <span className="text-[12px] font-medium truncate">{data.name}</span>
      </div>

      <div className="px-2 py-1.5 space-y-1">
        <div className="flex items-center justify-between">
          <Badge variant={isMaster ? "default" : "secondary"} className="text-[10px] px-1 py-0">
            {data.role}
          </Badge>
          <span className="text-[10px] text-muted-foreground font-mono">
            {data.hardware.cores}c/{data.hardware.memory}G/{data.hardware.disk}G
          </span>
        </div>

        <div className="flex items-center gap-1.5 pt-0.5">
          {data.hasPgbouncer && (
            <Layers className="h-3 w-3 text-chart-5" aria-label="PgBouncer" />
          )}
          {data.hasEtcd && (
            <Shield className="h-3 w-3 text-chart-2" aria-label="Etcd" />
          )}
          {data.hasMonitoring && (
            <Activity className="h-3 w-3 text-chart-3" aria-label="Monitoring" />
          )}
          {data.hasBackup && (
            <Archive className="h-3 w-3 text-chart-4" aria-label="Backup" />
          )}
        </div>
      </div>

      <Handle type="source" position={Position.Bottom} className="!bg-primary !w-2 !h-2" />
    </div>
  )
}

const nodeTypes: NodeTypes = {
  databaseNode: DatabaseNodeComponent,
}

// ── Layout ─────────────────────────────────────────────────────

function layoutNodes(nodes: PostgresNode[], errors: string[]): { nodes: Node<DbNodeData>[]; edges: Edge[] } {
  const masters = nodes.filter((n) => n.role === "MASTER")
  const replicas = nodes.filter((n) => n.role === "REPLICA")

  const flowNodes: Node<DbNodeData>[] = []
  const flowEdges: Edge[] = []

  const colWidth = 200
  const rowHeight = 140

  masters.forEach((node, i) => {
    flowNodes.push({
      id: node.name,
      type: "databaseNode",
      position: { x: i * colWidth + 40, y: 40 },
      data: { ...node, hasError: errors.includes(node.name) },
    })
  })

  replicas.forEach((node, i) => {
    flowNodes.push({
      id: node.name,
      type: "databaseNode",
      position: { x: i * colWidth + 40, y: 40 + rowHeight },
      data: { ...node, hasError: errors.includes(node.name) },
    })

    // Connect replicas to the first master
    if (masters.length > 0) {
      flowEdges.push({
        id: `${masters[0].name}-${node.name}`,
        source: masters[0].name,
        target: node.name,
        animated: true,
        style: { stroke: "var(--primary)", strokeWidth: 1.5 },
      })
    }
  })

  return { nodes: flowNodes, edges: flowEdges }
}

// ── Canvas ─────────────────────────────────────────────────────

export function TopologyCanvas() {
  const test = useStore($currentTest)
  const selectedNodeId = useStore($selectedNodeId)
  const validationErrors = useStore($validationErrors)

  const errorNodeNames = useMemo(
    () =>
      validationErrors
        .map((e) => {
          const match = e.fieldPath.match(/nodes\[(\d+)\]/)
          if (match && test?.databaseTemplate) {
            const idx = parseInt(match[1])
            return test.databaseTemplate.nodes[idx]?.name ?? ""
          }
          return ""
        })
        .filter(Boolean),
    [validationErrors, test],
  )

  const { nodes, edges } = useMemo(() => {
    if (!test?.databaseTemplate) return { nodes: [], edges: [] }
    return layoutNodes(test.databaseTemplate.nodes, errorNodeNames)
  }, [test, errorNodeNames])

  const onNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    $selectedNodeId.set(node.id)
  }, [])

  const onPaneClick = useCallback(() => {
    $selectedNodeId.set(null)
  }, [])

  if (!test?.databaseTemplate) {
    return (
      <div className="flex h-full items-center justify-center text-[12px] text-muted-foreground">
        No database template configured
      </div>
    )
  }

  return (
    <ReactFlow
      nodes={nodes.map((n) => ({ ...n, selected: n.id === selectedNodeId }))}
      edges={edges}
      nodeTypes={nodeTypes}
      onNodeClick={onNodeClick}
      onPaneClick={onPaneClick}
      fitView
      fitViewOptions={{ padding: 0.3 }}
      proOptions={{ hideAttribution: true }}
      className="bg-background"
    >
      <Background gap={16} size={1} color="var(--border)" />
      <Controls
        showInteractive={false}
        className="!bg-card !border-border !shadow-none [&>button]:!bg-card [&>button]:!border-border [&>button]:!text-foreground"
      />
    </ReactFlow>
  )
}
