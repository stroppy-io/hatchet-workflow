import { useCallback, useEffect, useRef, useState } from "react";
import { Play, Square, Terminal } from "lucide-react";

import { useSearchParams, useTenantSlug } from "@/lib/router";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { resolveTenantId } from "@/services/tenant";
import { openShell, type ShellSession } from "@/services/shell";

// Minimal agent terminal. Opens a bidi OpenShell stream: a ShellStart frame
// targets a machine, stdin frames carry typed commands, and ShellServerFrame
// stdout/stderr bytes are appended to the output pane until a ShellExit.
export function Shell() {
  const tenantSlug = useTenantSlug() ?? "";
  const [searchParams] = useSearchParams();
  const runId = searchParams.get("runId") ?? "";

  const [machineId, setMachineId] = useState("");
  const [input, setInput] = useState("");
  const [output, setOutput] = useState("");
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const sessionRef = useRef<ShellSession | null>(null);
  const preRef = useRef<HTMLPreElement>(null);

  const append = useCallback((text: string) => {
    setOutput((prev) => prev + text);
  }, []);

  useEffect(() => {
    if (preRef.current) preRef.current.scrollTop = preRef.current.scrollHeight;
  }, [output]);

  const start = useCallback(async () => {
    if (!tenantSlug || !machineId.trim()) return;
    setError(null);
    try {
      const tenantId = await resolveTenantId(tenantSlug);
      const session = openShell({
        tenantId,
        machineId: machineId.trim(),
        runId,
        cols: 120,
        rows: 30,
      });
      sessionRef.current = session;
      setConnected(true);
      append(`\n[connected to ${machineId.trim()}]\n`);
      // consume server frames
      void (async () => {
        try {
          for await (const frame of session.output) {
            if (frame.kind === "stdout" || frame.kind === "stderr") {
              append(frame.text ?? "");
            } else if (frame.kind === "exit") {
              append(`\n[exit code=${frame.code ?? 0}${frame.error ? ` error=${frame.error}` : ""}]\n`);
              setConnected(false);
            }
          }
        } catch (err) {
          setError(err instanceof Error ? err.message : String(err));
          setConnected(false);
        }
      })();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setConnected(false);
    }
  }, [tenantSlug, machineId, runId, append]);

  const stop = useCallback(() => {
    sessionRef.current?.close();
    sessionRef.current = null;
    setConnected(false);
  }, []);

  useEffect(() => () => sessionRef.current?.close(), []);

  const sendInput = useCallback(() => {
    const session = sessionRef.current;
    if (!session) return;
    session.send(input + "\n");
    append(`$ ${input}\n`);
    setInput("");
  }, [input, append]);

  return (
    <div className="min-h-full bg-background text-foreground">
      <div className="mx-auto flex max-w-[1200px] flex-col gap-4 p-4 md:p-6">
        <div className="flex flex-col gap-3 border-b border-border pb-4">
          <div className="flex items-center gap-2 text-lg font-semibold">
            <Terminal className="h-5 w-5 text-primary" />
            <h1>Agent shell</h1>
          </div>
          <div className="flex flex-wrap items-end gap-2">
            <div className="flex flex-col gap-1">
              <label className="text-[11px] uppercase text-muted-foreground">Machine ID</label>
              <Input
                value={machineId}
                onChange={(e) => setMachineId(e.target.value)}
                placeholder="target machine id"
                className="h-9 w-[280px]"
                disabled={connected}
              />
            </div>
            {runId && (
              <div className="pb-2 text-[11px] text-muted-foreground">run {runId.slice(0, 8)}</div>
            )}
            {!connected ? (
              <Button size="sm" onClick={() => void start()} disabled={!machineId.trim()}>
                <Play className="h-4 w-4" /> Open shell
              </Button>
            ) : (
              <Button size="sm" variant="outline" onClick={stop}>
                <Square className="h-4 w-4" /> Close
              </Button>
            )}
          </div>
        </div>

        {error && (
          <div className="border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </div>
        )}

        <pre
          ref={preRef}
          className="h-[420px] overflow-auto border border-border bg-black/60 p-3 font-mono text-[12px] leading-relaxed text-green-300"
        >
          {output || "[not connected]"}
        </pre>

        <div className="flex items-center gap-2">
          <Input
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && sendInput()}
            placeholder={connected ? "type a command and press Enter" : "open a shell first"}
            className="h-9 font-mono"
            disabled={!connected}
          />
          <Button size="sm" variant="outline" onClick={sendInput} disabled={!connected}>
            Send
          </Button>
        </div>
      </div>
    </div>
  );
}
