import { X, AlertTriangle } from "lucide-react"
import { motion } from "framer-motion"
import { Button } from "@/components/ui/button"
import { RunStatusBadge } from "./RunStatusBadge"
import type { WorkflowNode } from "@/stores/runs"

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

function formatTimestamp(ts?: string): string {
  if (!ts) return "--"
  const d = new Date(ts)
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  })
}

function formatDuration(startedAt?: string, completedAt?: string): string {
  if (!startedAt) return "--"
  const start = new Date(startedAt).getTime()
  const end = completedAt ? new Date(completedAt).getTime() : Date.now()
  const diff = Math.max(0, end - start)
  const secs = Math.floor(diff / 1000)
  if (secs < 60) return `${secs}s`
  const mins = Math.floor(secs / 60)
  const remSecs = secs % 60
  if (mins < 60) return `${mins}m ${remSecs}s`
  const hrs = Math.floor(mins / 60)
  const remMins = mins % 60
  return `${hrs}h ${remMins}m`
}

interface StepDetailProps {
  node: WorkflowNode
  onClose: () => void
}

export function StepDetail({ node, onClose }: StepDetailProps) {
  const displayName = stepDisplayNames[node.name] ?? node.name

  return (
    <motion.div
      initial={{ x: 20, opacity: 0 }}
      animate={{ x: 0, opacity: 1 }}
      exit={{ x: 20, opacity: 0 }}
      transition={{ duration: 0.15 }}
      className="w-72 shrink-0 border-l bg-card overflow-y-auto"
    >
      <div className="flex items-center justify-between border-b px-3 py-2">
        <span className="text-[14px] font-semibold truncate">{displayName}</span>
        <Button variant="ghost" size="icon" className="h-6 w-6" onClick={onClose}>
          <X className="h-3.5 w-3.5" />
        </Button>
      </div>

      <div className="p-3 space-y-3">
        <div>
          <span className="text-[11px] text-muted-foreground uppercase tracking-wide">Status</span>
          <div className="mt-1">
            <RunStatusBadge status={node.status} />
          </div>
        </div>

        <div>
          <span className="text-[11px] text-muted-foreground uppercase tracking-wide">Started</span>
          <p className="text-[12px] mt-0.5">{formatTimestamp(node.startedAt)}</p>
        </div>

        <div>
          <span className="text-[11px] text-muted-foreground uppercase tracking-wide">Completed</span>
          <p className="text-[12px] mt-0.5">{formatTimestamp(node.completedAt)}</p>
        </div>

        <div>
          <span className="text-[11px] text-muted-foreground uppercase tracking-wide">Duration</span>
          <p className="text-[12px] font-mono mt-0.5">
            {formatDuration(node.startedAt, node.completedAt)}
          </p>
        </div>

        {node.error && (
          <div className="bg-destructive/10 border border-destructive/30 p-2">
            <div className="flex items-center gap-1.5 mb-1">
              <AlertTriangle className="h-3.5 w-3.5 text-destructive" />
              <span className="text-[11px] font-medium text-destructive">Error</span>
            </div>
            <p className="text-[11px] text-destructive/90 font-mono break-all">
              {node.error}
            </p>
          </div>
        )}
      </div>
    </motion.div>
  )
}
