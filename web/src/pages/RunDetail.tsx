import { useEffect, useState, useCallback } from "react"
import { useParams, useNavigate } from "react-router"
import { useStore } from "@nanostores/react"
import { ArrowLeft, Loader2 } from "lucide-react"
import { AnimatePresence } from "framer-motion"
import { Button } from "@/components/ui/button"
import { RunStatusBadge } from "@/components/runs/RunStatusBadge"
import { DagGraph } from "@/components/runs/DagGraph"
import { StepDetail } from "@/components/runs/StepDetail"
import {
  $workflowGraph,
  startStreamingGraph,
  stopStreaming,
} from "@/stores/runs"

export function RunDetailPage() {
  const { runId } = useParams<{ runId: string }>()
  const navigate = useNavigate()
  const graph = useStore($workflowGraph)
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)

  useEffect(() => {
    if (runId) {
      startStreamingGraph(runId)
    }
    return () => {
      stopStreaming()
    }
  }, [runId])

  const handleNodeClick = useCallback((nodeId: string) => {
    setSelectedNodeId((prev) => (prev === nodeId ? null : nodeId))
  }, [])

  const selectedNode = graph?.nodes.find((n) => n.id === selectedNodeId) ?? null

  const completedCount = graph?.nodes.filter(
    (n) => n.status === "WORKFLOW_NODE_STATUS_COMPLETED"
  ).length ?? 0
  const totalCount = graph?.nodes.length ?? 0
  const progressPct = totalCount > 0 ? (completedCount / totalCount) * 100 : 0

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center gap-2 border-b bg-secondary/30 px-3 py-1.5">
        <Button
          variant="ghost"
          size="icon"
          className="h-6 w-6"
          onClick={() => navigate("/runs")}
        >
          <ArrowLeft className="h-3.5 w-3.5" />
        </Button>
        <span className="text-[12px] font-mono text-muted-foreground truncate">
          {runId}
        </span>
        {graph && (
          <RunStatusBadge
            status={
              graph.status.replace("WORKFLOW_STATUS_", "TEST_RUN_STATUS_") as
                | "TEST_RUN_STATUS_PENDING"
                | "TEST_RUN_STATUS_RUNNING"
                | "TEST_RUN_STATUS_COMPLETED"
                | "TEST_RUN_STATUS_FAILED"
                | "TEST_RUN_STATUS_CANCELLED"
            }
          />
        )}
      </div>

      {/* Progress bar */}
      <div className="h-1 bg-secondary">
        <div
          className="h-full bg-chart-2 transition-all duration-500"
          style={{ width: `${progressPct}%` }}
        />
      </div>

      {/* Main content */}
      {!graph ? (
        <div className="flex-1 flex items-center justify-center">
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        </div>
      ) : (
        <div className="flex flex-1 overflow-hidden">
          {/* DAG */}
          <div className="flex-1">
            <DagGraph
              graph={graph}
              onNodeClick={handleNodeClick}
              selectedNodeId={selectedNodeId}
            />
          </div>

          {/* Step detail panel */}
          <AnimatePresence>
            {selectedNode && (
              <StepDetail
                key={selectedNode.id}
                node={selectedNode}
                onClose={() => setSelectedNodeId(null)}
              />
            )}
          </AnimatePresence>
        </div>
      )}
    </div>
  )
}
