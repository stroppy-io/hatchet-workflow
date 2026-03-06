import { useMemo, useCallback } from "react"
import {
  ReactFlow,
  Handle,
  Position,
  type Node,
  type Edge,
  type NodeProps,
} from "@xyflow/react"
import {
  CheckCircle2,
  Circle,
  Loader2,
  XCircle,
  Ban,
  Clock,
} from "lucide-react"
import { cn } from "@/lib/utils"
import type { WorkflowGraph, WorkflowNodeStatus } from "@/stores/runs"

const stepDisplayNames: Record<string, string> = {
  "validate-input": "Validate Input",
  "acquire-network": "Acquire Network",
  "plan-placement-intent": "Plan Placement",
  "build-placement": "Build Placement",
  "deploy-plan": "Deploy Infrastructure",
  "wait-workers-in-hatchet": "Wait for Workers",
  "run-database-containers": "Start Database",
  "run-stroppy-containers": "Start Stroppy",
  "install-stroppy": "Install Stroppy",
  "run-stroppy-test": "Run Test",
  "destroy-plan": "Cleanup",
}

const statusStyles: Record<string, string> = {
  WORKFLOW_NODE_STATUS_PENDING: "border-border bg-secondary text-muted-foreground",
  WORKFLOW_NODE_STATUS_RUNNING: "border-primary bg-primary/10 text-primary",
  WORKFLOW_NODE_STATUS_COMPLETED: "border-chart-2 bg-chart-2/10 text-chart-2",
  WORKFLOW_NODE_STATUS_FAILED: "border-destructive bg-destructive/10 text-destructive",
  WORKFLOW_NODE_STATUS_CANCELLED: "border-border bg-muted text-muted-foreground",
}

function StatusIcon({ status }: { status: WorkflowNodeStatus }) {
  switch (status) {
    case "WORKFLOW_NODE_STATUS_RUNNING":
      return <Loader2 className="h-3.5 w-3.5 animate-spin" />
    case "WORKFLOW_NODE_STATUS_COMPLETED":
      return <CheckCircle2 className="h-3.5 w-3.5" />
    case "WORKFLOW_NODE_STATUS_FAILED":
      return <XCircle className="h-3.5 w-3.5" />
    case "WORKFLOW_NODE_STATUS_CANCELLED":
      return <Ban className="h-3.5 w-3.5" />
    default:
      return <Circle className="h-3.5 w-3.5" />
  }
}

function formatElapsed(startedAt?: string, completedAt?: string): string {
  if (!startedAt) return ""
  const start = new Date(startedAt).getTime()
  const end = completedAt ? new Date(completedAt).getTime() : Date.now()
  const secs = Math.max(0, Math.floor((end - start) / 1000))
  if (secs < 60) return `${secs}s`
  const mins = Math.floor(secs / 60)
  const remSecs = secs % 60
  return `${mins}m ${remSecs}s`
}

type StepNodeData = {
  label: string
  nodeStatus: WorkflowNodeStatus
  startedAt?: string
  completedAt?: string
  hasError: boolean
  selected: boolean
}

function WorkflowStepNode({ data }: NodeProps<Node<StepNodeData>>) {
  const style = statusStyles[data.nodeStatus] ?? statusStyles.WORKFLOW_NODE_STATUS_PENDING
  const elapsed = formatElapsed(data.startedAt, data.completedAt)

  return (
    <>
      <Handle type="target" position={Position.Top} className="!bg-border !w-1.5 !h-1.5" />
      <div
        className={cn(
          "flex items-center gap-2 border px-3 py-2 min-w-[180px] cursor-pointer transition-shadow",
          style,
          data.selected && "ring-1 ring-ring shadow-md"
        )}
      >
        <StatusIcon status={data.nodeStatus} />
        <div className="flex flex-col">
          <span className="text-[12px] font-medium leading-tight">{data.label}</span>
          {elapsed && (
            <span className="text-[10px] opacity-70 flex items-center gap-0.5 mt-0.5">
              <Clock className="h-2.5 w-2.5" />
              {elapsed}
            </span>
          )}
        </div>
        {data.hasError && <XCircle className="h-3 w-3 text-destructive ml-auto" />}
      </div>
      <Handle type="source" position={Position.Bottom} className="!bg-border !w-1.5 !h-1.5" />
    </>
  )
}

const nodeTypes = { workflowStep: WorkflowStepNode }

interface DagGraphProps {
  graph: WorkflowGraph
  onNodeClick: (nodeId: string) => void
  selectedNodeId: string | null
}

export function DagGraph({ graph, onNodeClick, selectedNodeId }: DagGraphProps) {
  const nodes: Node<StepNodeData>[] = useMemo(() => {
    return graph.nodes.map((n, i) => ({
      id: n.id,
      type: "workflowStep",
      position: { x: 0, y: i * 80 },
      data: {
        label: stepDisplayNames[n.name] ?? n.name,
        nodeStatus: n.status,
        startedAt: n.startedAt,
        completedAt: n.completedAt,
        hasError: !!n.error,
        selected: n.id === selectedNodeId,
      },
    }))
  }, [graph.nodes, selectedNodeId])

  const edges: Edge[] = useMemo(() => {
    return graph.edges.map((e) => ({
      id: `${e.fromNodeId}-${e.toNodeId}`,
      source: e.fromNodeId,
      target: e.toNodeId,
      style: { stroke: "var(--border)" },
    }))
  }, [graph.edges])

  const handleNodeClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      onNodeClick(node.id)
    },
    [onNodeClick]
  )

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      nodeTypes={nodeTypes}
      onNodeClick={handleNodeClick}
      fitView
      fitViewOptions={{ padding: 0.3 }}
      proOptions={{ hideAttribution: true }}
      nodesDraggable={false}
      nodesConnectable={false}
      elementsSelectable={false}
      panOnScroll
      minZoom={0.5}
      maxZoom={1.5}
    />
  )
}
