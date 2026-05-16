import { useEffect, useState } from "react";
import { clients } from "@/api/clients";
import type { LogLine } from "@/lib/proto/cloud/v1/agent/protocol_pb";

export function useStreamTestRunLogs(testRunId: string | null) {
  const [lines, setLines] = useState<LogLine[]>([]);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    if (!testRunId) return;
    let cancelled = false;
    (async () => {
      try {
        for await (const line of clients.testRun.streamTestRunLogs({
          testRunId: { value: testRunId },
          stepId: "",
        })) {
          if (cancelled) return;
          setLines((p) => [...p, line]);
        }
      } catch (e: unknown) {
        if (cancelled) return;
        const err = e as { code?: string };
        // Gracefully degrade on Unimplemented — surface as soft notice
        if (err?.code === "unimplemented") {
          setError(
            new Error("Live log streaming not yet implemented on this server")
          );
        } else {
          setError(e instanceof Error ? e : new Error(String(e)));
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [testRunId]);

  return { lines, error };
}
