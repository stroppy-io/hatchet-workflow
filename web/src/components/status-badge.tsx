import { statusLabel } from "@/lib/format";
import { cn } from "@/lib/utils";
import { Status } from "@/lib/proto/cloud/v1/runtime/primitive/status_pb.ts";

const STATUS_CLASS: Record<number, string> = {
  [Status.PENDING]: "bg-muted text-muted-foreground",
  [Status.RUNNING]: "bg-primary/15 text-primary",
  [Status.RETRY_WAIT]: "bg-warning/15 text-warning",
  [Status.COMPLETED]: "bg-success/15 text-success",
  [Status.FAILED]: "bg-destructive/15 text-destructive",
  [Status.SKIPPED]: "bg-muted text-muted-foreground",
  [Status.CANCELLING]: "bg-warning/15 text-warning",
  [Status.CANCELLED]: "bg-muted text-muted-foreground",
};

export function StatusBadge({ status }: { status?: Status }) {
  const cls = (status !== undefined && STATUS_CLASS[status]) || "bg-muted text-muted-foreground";
  return <span className={cn("inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium", cls)}>{statusLabel(status)}</span>;
}
