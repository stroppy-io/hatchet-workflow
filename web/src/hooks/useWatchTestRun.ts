import { useEffect, useState } from "react";
import { clients } from "@/api/clients";
import type { TestRunProgress } from "@/lib/proto/cloud/v1/testing/test_run_pb";

export function useWatchTestRun(testRunId: string | null) {
  const [progress, setProgress] = useState<TestRunProgress[]>([]);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    if (!testRunId) return;
    let cancelled = false;
    (async () => {
      try {
        for await (const update of clients.testRun.watchTestRun({
          testRunId: { value: testRunId },
        })) {
          if (cancelled) return;
          setProgress((p) => [...p, update]);
        }
      } catch (e: unknown) {
        if (cancelled) return;
        const err = e as { code?: string };
        // Gracefully degrade on Unimplemented — surface as soft notice
        if (err?.code === "unimplemented") {
          setError(
            new Error("Live run watching not yet implemented on this server")
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

  return { progress, error };
}
