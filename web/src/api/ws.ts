import { clients } from "./clients";

export interface WSMessage {
  type: "log" | "report" | "agent_log";
  run_id?: string;
  node_id?: string;
  payload: unknown;
}

export interface LogLine {
  run_id: string;
  phase: string;
  machine_id: string;
  line: string;
  ts: string;
}

export type WSMessageHandler = (msg: WSMessage) => void;
export type LogHandler = (line: LogLine) => void;

/**
 * WSConnection — legacy facade around ConnectRPC's server-streaming
 * `TestRunService.StreamTestRunLogs`. The old WebSocket transport at
 * `/ws/logs` is gone; this class preserves the constructor +
 * `connect/onMessage/disconnect` surface used by LogStream.tsx and
 * pipes each LogLine through as a synthetic "agent_log" WSMessage.
 *
 * Live logs only: history is fetched by LogStream via the same Connect
 * RPC with follow=false / since=<bound>.
 */
export class WSConnection {
  private runID?: string;
  private handlers: WSMessageHandler[] = [];
  private abort: AbortController | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private shouldReconnect = true;

  constructor(runID?: string) {
    this.runID = runID;
  }

  connect(): void {
    if (this.abort) return;
    if (!this.runID) return;
    this.shouldReconnect = true;
    this.start();
  }

  private start(): void {
    const ac = new AbortController();
    this.abort = ac;
    (async () => {
      try {
        for await (const line of clients.testRun.streamTestRunLogs(
          {
            testRunId: { value: this.runID! },
            stepId: "",
            follow: true,
          },
          { signal: ac.signal },
        )) {
          if (ac.signal.aborted) return;
          // Convert proto LogLine → legacy WSMessage shape consumed by
          // LogStream.tsx. ts is a proto Timestamp; coerce to ISO for
          // downstream code that expects a string.
          const tsMs = line.ts
            ? Number(line.ts.seconds) * 1000 +
              Math.floor((line.ts.nanos ?? 0) / 1_000_000)
            : Date.now();
          const msg: WSMessage = {
            type: "agent_log",
            run_id: this.runID,
            payload: {
              command_id: line.commandId,
              machine_id: line.machineId,
              action: "",
              line: line.line,
              stream: line.stream,
              ts: new Date(tsMs).toISOString(),
            },
          };
          this.handlers.forEach((h) => h(msg));
        }
      } catch {
        // Stream closed or errored — fall through to reconnect.
      }
      if (this.shouldReconnect && this.abort === ac) {
        this.abort = null;
        this.reconnectTimer = setTimeout(() => this.start(), 2000);
      }
    })();
  }

  onMessage(handler: WSMessageHandler): () => void {
    this.handlers.push(handler);
    return () => {
      this.handlers = this.handlers.filter((h) => h !== handler);
    };
  }

  disconnect(): void {
    this.shouldReconnect = false;
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.abort?.abort();
    this.abort = null;
  }
}
