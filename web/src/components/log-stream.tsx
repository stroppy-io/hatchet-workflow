import { VList } from "virtua";

import { formatTimestamp } from "@/lib/format";
import { cn } from "@/lib/utils";
import { Stream, type LogLine } from "@/lib/proto/cloud/v1/runtime/logs/logs_pb.ts";

// Virtualized log viewer (handles large buffers). Reused by run detail + share.
export function LogStream({ lines, height = 480 }: { lines: LogLine[]; height?: number }) {
  if (lines.length === 0) {
    return <div className="flex items-center justify-center rounded-md border p-8 text-sm text-muted-foreground" style={{ height }}>No log lines</div>;
  }
  return (
    <div className="overflow-hidden rounded-md border bg-[#050505]">
      <VList style={{ height }}>
        {lines.map((line, index) => {
          const source = line.componentId || line.machineId || "";
          return (
            <div key={index} className="flex gap-3 px-3 py-0.5 font-mono text-xs leading-relaxed hover:bg-muted/30">
              <span className="shrink-0 text-muted-foreground/70">{formatTimestamp(line.observedAt)}</span>
              {source ? <span className="shrink-0 text-primary/80">{source}</span> : null}
              <span className={cn("whitespace-pre-wrap break-all", line.stream === Stream.STDERR ? "text-destructive" : "text-foreground/90")}>{line.line}</span>
            </div>
          );
        })}
      </VList>
    </div>
  );
}
