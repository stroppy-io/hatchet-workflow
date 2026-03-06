import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"
import type { TestRunStatus, WorkflowNodeStatus } from "@/stores/runs"

type StatusType = TestRunStatus | WorkflowNodeStatus

const statusConfig: Record<string, { label: string; className: string }> = {
  TEST_RUN_STATUS_PENDING: {
    label: "Pending",
    className: "bg-secondary text-secondary-foreground border-secondary",
  },
  TEST_RUN_STATUS_RUNNING: {
    label: "Running",
    className: "bg-primary/15 text-primary border-primary/30 animate-pulse",
  },
  TEST_RUN_STATUS_COMPLETED: {
    label: "Completed",
    className: "bg-chart-2/15 text-chart-2 border-chart-2/30",
  },
  TEST_RUN_STATUS_FAILED: {
    label: "Failed",
    className: "bg-destructive/15 text-destructive border-destructive/30",
  },
  TEST_RUN_STATUS_CANCELLED: {
    label: "Cancelled",
    className: "bg-muted text-muted-foreground border-muted",
  },
  WORKFLOW_NODE_STATUS_PENDING: {
    label: "Pending",
    className: "bg-secondary text-secondary-foreground border-secondary",
  },
  WORKFLOW_NODE_STATUS_RUNNING: {
    label: "Running",
    className: "bg-primary/15 text-primary border-primary/30 animate-pulse",
  },
  WORKFLOW_NODE_STATUS_COMPLETED: {
    label: "Completed",
    className: "bg-chart-2/15 text-chart-2 border-chart-2/30",
  },
  WORKFLOW_NODE_STATUS_FAILED: {
    label: "Failed",
    className: "bg-destructive/15 text-destructive border-destructive/30",
  },
  WORKFLOW_NODE_STATUS_CANCELLED: {
    label: "Cancelled",
    className: "bg-muted text-muted-foreground border-muted",
  },
}

interface RunStatusBadgeProps {
  status: StatusType
  className?: string
}

export function RunStatusBadge({ status, className }: RunStatusBadgeProps) {
  const config = statusConfig[status] ?? {
    label: status,
    className: "bg-secondary text-secondary-foreground",
  }

  return (
    <Badge variant="outline" className={cn(config.className, className)}>
      {config.label}
    </Badge>
  )
}
