import { getAccessToken } from "./transport";

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

export class WSConnection {
  private ws: WebSocket | null = null;
  private baseUrl: string;
  private handlers: WSMessageHandler[] = [];
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private shouldReconnect = true;

  constructor(runID?: string) {
    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    const base = `${proto}//${window.location.host}`;
    this.baseUrl = runID ? `${base}/ws/logs/${runID}` : `${base}/ws/logs`;
  }

  connect(): void {
    if (this.ws?.readyState === WebSocket.OPEN) return;

    // Pass JWT via query param — browsers can't set headers on WebSocket.
    const token = getAccessToken();
    const url = token ? `${this.baseUrl}?token=${encodeURIComponent(token)}` : this.baseUrl;
    this.ws = new WebSocket(url);

    this.ws.onmessage = (event) => {
      try {
        const msg: WSMessage = JSON.parse(event.data);
        this.handlers.forEach((h) => h(msg));
      } catch {
        // ignore parse errors
      }
    };

    this.ws.onclose = () => {
      if (this.shouldReconnect) {
        this.reconnectTimer = setTimeout(() => this.connect(), 2000);
      }
    };

    this.ws.onerror = () => {
      this.ws?.close();
    };
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
    this.ws?.close();
    this.ws = null;
  }
}
