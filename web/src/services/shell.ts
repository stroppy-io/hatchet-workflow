// Agent shell data surface — wraps cloud.v1.api.AgentShellService.OpenShell, a
// BIDIRECTIONAL stream of ShellClientFrame <-> ShellServerFrame.
//
// connect-web drives bidi streams with an async-iterable REQUEST: we push client
// frames into a queue exposed as an AsyncIterable, hand it to openShell(), and
// iterate the server frames it returns. The first client frame MUST be a
// ShellStart (api.ShellStart). Server frames carry stdout/stderr bytes or a
// final ShellExit.
//
// RUNTIME CAVEAT: connect-web bidi streaming requires an HTTP/2 transport that
// supports full-duplex (the connect protocol over fetch streams). The wiring,
// message construction (ShellStart / ShellClientFrame / ShellServerFrame) and
// the openShell() call are exercised here and type-check; whether the duplex
// actually flows depends on the deployed transport/proxy.

import { create } from "@bufbuild/protobuf";
import {
  ShellClientFrameSchema,
  type ShellClientFrame,
} from "@/lib/proto/cloud/v1/api/agent_shell_pb";
import { agentShellClient } from "@/services/client";

/** A decoded server frame for the terminal view. */
export interface ShellOutput {
  kind: "stdout" | "stderr" | "exit";
  /** decoded text for stdout/stderr. */
  text?: string;
  /** exit code for the "exit" frame. */
  code?: number;
  /** optional error message for the "exit" frame. */
  error?: string;
}

/** Parameters for the opening ShellStart frame. */
export interface ShellStartParams {
  tenantId: string;
  machineId: string;
  runId?: string;
  componentId?: string;
  cols?: number;
  rows?: number;
  shell?: string;
}

/**
 * A live shell session handle. `send` pushes stdin, `resize` resizes the pty,
 * `close` ends the session, and `output` is the async stream of server frames.
 */
export interface ShellSession {
  send(data: string): void;
  resize(cols: number, rows: number): void;
  close(): void;
  output: AsyncIterable<ShellOutput>;
}

// A simple async queue: producers push frames, the iterator yields them in order
// and blocks (awaits) when empty until the next push or end().
class FrameQueue<T> implements AsyncIterable<T> {
  private items: T[] = [];
  private resolvers: Array<(r: IteratorResult<T>) => void> = [];
  private done = false;

  push(item: T): void {
    if (this.done) return;
    const r = this.resolvers.shift();
    if (r) r({ value: item, done: false });
    else this.items.push(item);
  }

  end(): void {
    this.done = true;
    let r: ((r: IteratorResult<T>) => void) | undefined;
    while ((r = this.resolvers.shift())) r({ value: undefined as never, done: true });
  }

  [Symbol.asyncIterator](): AsyncIterator<T> {
    return {
      next: (): Promise<IteratorResult<T>> => {
        const item = this.items.shift();
        if (item !== undefined) return Promise.resolve({ value: item, done: false });
        if (this.done) return Promise.resolve({ value: undefined as never, done: true });
        return new Promise((resolve) => this.resolvers.push(resolve));
      },
    };
  }
}

const enc = new TextEncoder();
const dec = new TextDecoder();

export function openShell(params: ShellStartParams): ShellSession {
  const clientFrames = new FrameQueue<ShellClientFrame>();

  // First frame MUST be the ShellStart.
  clientFrames.push(
    create(ShellClientFrameSchema, {
      frame: {
        case: "start",
        value: {
          tenantId: params.tenantId,
          machineId: params.machineId,
          runId: params.runId ?? "",
          componentId: params.componentId ?? "",
          cols: params.cols ?? 80,
          rows: params.rows ?? 24,
          shell: params.shell ?? "",
        },
      },
    }),
  );

  const serverStream = agentShellClient.openShell(clientFrames);

  async function* outputs(): AsyncGenerator<ShellOutput> {
    for await (const frame of serverStream) {
      switch (frame.frame.case) {
        case "stdout":
          yield { kind: "stdout", text: dec.decode(frame.frame.value) };
          break;
        case "stderr":
          yield { kind: "stderr", text: dec.decode(frame.frame.value) };
          break;
        case "exit":
          yield {
            kind: "exit",
            code: frame.frame.value.code,
            error: frame.frame.value.error,
          };
          clientFrames.end();
          return;
        default:
          break;
      }
    }
  }

  return {
    send(data: string) {
      clientFrames.push(
        create(ShellClientFrameSchema, {
          frame: { case: "stdin", value: enc.encode(data) },
        }),
      );
    },
    resize(cols: number, rows: number) {
      clientFrames.push(
        create(ShellClientFrameSchema, {
          frame: { case: "resize", value: { cols, rows } },
        }),
      );
    },
    close() {
      clientFrames.push(
        create(ShellClientFrameSchema, {
          frame: { case: "close", value: {} },
        }),
      );
      clientFrames.end();
    },
    output: outputs(),
  };
}
