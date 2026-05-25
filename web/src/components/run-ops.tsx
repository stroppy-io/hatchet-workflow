import { useMemo, useState } from "react";
import type { ReactElement } from "react";
import type { Any } from "@bufbuild/protobuf/wkt";
import { ChevronRight, FileText, FolderPlus, Info, Terminal } from "lucide-react";
import type { LucideIcon } from "lucide-react";

import { JsonPanel } from "@/components/json-panel";
import { decodeAny } from "@/lib/proto-registry";
import { cn } from "@/lib/utils";
import type { Operation, Operation_Result } from "@/lib/proto/cloud/v1/runtime/ops/operation_pb.ts";
import type { File as SysFile } from "@/lib/proto/cloud/v1/runtime/system/file_pb.ts";
import type { Cmd_Spec, Cmd_Result } from "@/lib/proto/cloud/v1/runtime/system/cmd_pb.ts";
import type { Command, Report } from "@/lib/proto/cloud/v1/runtime/agent/agent_pb.ts";
import { CommandStatus } from "@/lib/proto/cloud/v1/runtime/agent/agent_pb.ts";

const OP_TYPE = "cloud.v1.runtime.ops.Operation";
const FILE_TYPE = "cloud.v1.runtime.system.File";
const COMMAND_TYPE = "cloud.v1.runtime.agent.Command";
const REPORT_TYPE = "cloud.v1.runtime.agent.Report";

const decoder = new TextDecoder();
const bytesToText = (b?: Uint8Array) => (b && b.length ? decoder.decode(b) : "");

// ── file type recognition ────────────────────────────────────────
function basename(path: string): string {
  const clean = path.replace(/\/+$/, "");
  return clean.slice(clean.lastIndexOf("/") + 1) || clean;
}

function fileType(path: string): string {
  const name = basename(path).toLowerCase();
  if (name === "pg_hba.conf") return "pg_hba";
  if (name.endsWith(".conf")) return "conf";
  if (name.endsWith(".cnf")) return "cnf";
  if (name.endsWith(".yaml") || name.endsWith(".yml")) return "yaml";
  if (name.endsWith(".ini")) return "ini";
  if (name.endsWith(".toml")) return "toml";
  if (name.endsWith(".json")) return "json";
  if (name.endsWith(".sql")) return "sql";
  if (name.endsWith(".sh")) return "sh";
  if (name.endsWith(".service")) return "systemd";
  if (name.endsWith(".env") || name === "environment") return "env";
  if (name.endsWith(".js")) return "js";
  return "file";
}

// ── one-line summary (shown on the step row, no expand needed) ────
export type OpSummary = { icon: LucideIcon; text: string };

function operationSummary(op?: Operation): OpSummary | null {
  const v = op?.operation;
  switch (v?.case) {
    case "writeFile":
      return { icon: FileText, text: `write ${basename(v.value.info?.path ?? "")}` };
    case "appendFile":
      return { icon: FileText, text: `append ${basename(v.value.info?.path ?? "")}` };
    case "makeDir":
    case "makeTempDir":
      return { icon: FolderPlus, text: `mkdir ${(v.value as { path?: string }).path ?? ""}`.trim() };
    case "readOsInfo":
      return { icon: Info, text: "read os info" };
    case "runCmd":
      return { icon: Terminal, text: cmdOneLine(v.value) };
    default:
      return null;
  }
}

function cmdOneLine(spec: Cmd_Spec): string {
  const c = spec.command;
  if (c.case === "argv") return c.value.args.join(" ");
  if (c.case === "script") return c.value.text.split("\n").find((l) => l.trim() && !l.trim().startsWith("#")) ?? "script";
  return "command";
}

// summarizeAny returns a compact one-liner for an op/file/command payload, or
// null when not recognised (caller falls back to the generic view).
export function summarizeAny(any?: Any): OpSummary | null {
  const decoded = decodeAny(any);
  if (!decoded?.message) return null;
  switch (decoded.typeName) {
    case OP_TYPE:
      return operationSummary(decoded.message as unknown as Operation);
    case COMMAND_TYPE:
      return operationSummary((decoded.message as unknown as Command).operation);
    case FILE_TYPE:
      return { icon: FileText, text: `file ${basename((decoded.message as unknown as SysFile).info?.path ?? "")}` };
    default:
      return null;
  }
}

// ── full views ───────────────────────────────────────────────────
function FileCard({ file, op }: { file: SysFile; op: string }) {
  const path = file.info?.path ?? "";
  const src = file.source;
  let text: string | null = null;
  let note: string | null = null;
  if (src.case === "content") {
    const c = src.value.content;
    if (c.case === "text") text = c.value;
    else if (c.case === "bytes") note = `binary · ${c.value.length} bytes`;
  } else if (src.case === "ref") {
    note = `ref → ${src.value.uri}${src.value.sizeBytes ? ` · ${Number(src.value.sizeBytes)} bytes` : ""}`;
  }
  const mode = file.info?.mode ? file.info.mode.toString(8).padStart(4, "0") : "";

  return (
    <div className="overflow-hidden rounded-md border bg-[#050506]">
      <div className="flex flex-wrap items-center gap-2 border-b bg-card px-2.5 py-1.5">
        <FileText className="size-3.5 text-primary" />
        <span className="font-mono text-xs font-semibold text-foreground">{basename(path)}</span>
        <span className="rounded-sm border px-1 py-px font-mono text-[9px] uppercase tracking-wide text-muted-foreground">{fileType(path)}</span>
        <span className="rounded-sm bg-primary/10 px-1 py-px font-mono text-[9px] uppercase tracking-wide text-primary">{op}</span>
        <span className="ml-auto truncate font-mono text-[10px] text-muted-foreground" title={path}>{path}</span>
        {mode ? <span className="font-mono text-[10px] text-muted-foreground/70">{mode}</span> : null}
      </div>
      {text !== null ? (
        <pre className="max-h-[26rem] overflow-auto px-3 py-2 font-mono text-[11px] leading-5 text-zinc-200">{text || "(empty file)"}</pre>
      ) : (
        <div className="px-3 py-2 font-mono text-[11px] text-muted-foreground">{note ?? "(no content)"}</div>
      )}
    </div>
  );
}

function CmdBlock({ spec }: { spec: Cmd_Spec }) {
  const c = spec.command;
  const argv = c.case === "argv" ? c.value.args.join(" ") : "";
  const script = c.case === "script" ? c.value.text : "";
  return (
    <div className="overflow-hidden rounded-md border bg-[#050506]">
      <div className="flex items-center gap-2 border-b bg-card px-2.5 py-1.5">
        <Terminal className="size-3.5 text-success" />
        <span className="font-mono text-[10px] uppercase tracking-wide text-muted-foreground">{c.case === "script" ? `shell${c.value.shell ? ` · ${c.value.shell}` : ""}` : "exec"}</span>
        {spec.cwd ? <span className="ml-auto truncate font-mono text-[10px] text-muted-foreground" title={spec.cwd}>cwd {spec.cwd}</span> : null}
      </div>
      <pre className="max-h-[26rem] overflow-auto px-3 py-2 font-mono text-[11px] leading-5 text-zinc-200">
        <span className="select-none text-success">$ </span>
        {argv || script}
      </pre>
    </div>
  );
}

function StreamBlock({ label, body, tone }: { label: string; body: string; tone: string }) {
  if (!body) return null;
  return (
    <div className="overflow-hidden rounded-md border bg-[#050506]">
      <div className={cn("border-b bg-card px-2.5 py-1 font-mono text-[10px] uppercase tracking-wide", tone)}>{label}</div>
      <pre className="max-h-[20rem] overflow-auto px-3 py-2 font-mono text-[11px] leading-5 text-zinc-200">{body}</pre>
    </div>
  );
}

function CmdResultBlock({ result }: { result: Cmd_Result }) {
  const ok = result.exitCode === 0 && !result.timedOut;
  return (
    <div className="space-y-1.5">
      <div className="flex items-center gap-2 font-mono text-[11px]">
        <span className={cn("rounded-sm px-1.5 py-px", ok ? "bg-success/15 text-success" : "bg-destructive/15 text-destructive")}>exit {result.exitCode}</span>
        {result.timedOut ? <span className="rounded-sm bg-warning/15 px-1.5 py-px text-warning">timed out</span> : null}
      </div>
      <StreamBlock label="stdout" body={bytesToText(result.stdout)} tone="text-muted-foreground" />
      <StreamBlock label="stderr" body={bytesToText(result.stderr)} tone="text-destructive" />
    </div>
  );
}

function renderOperation(op?: Operation): ReactElement | null {
  const v = op?.operation;
  if (v?.case === "writeFile") return <FileCard file={v.value} op="write" />;
  if (v?.case === "appendFile") return <FileCard file={v.value} op="append" />;
  if (v?.case === "runCmd") return <CmdBlock spec={v.value} />;
  if (v?.case === "makeDir" || v?.case === "makeTempDir") {
    return (
      <div className="flex items-center gap-2 rounded-md border bg-[#050506] px-2.5 py-1.5 font-mono text-[11px] text-muted-foreground">
        <FolderPlus className="size-3.5 text-muted-foreground/70" />
        mkdir {(v.value as { path?: string }).path ?? "(temp)"}
      </div>
    );
  }
  if (v?.case === "readOsInfo") {
    return <div className="rounded-md border bg-[#050506] px-2.5 py-1.5 font-mono text-[11px] text-muted-foreground">read os info</div>;
  }
  return null;
}

const REPORT_STATUS: Record<number, { label: string; cls: string }> = {
  [CommandStatus.RUNNING]: { label: "running", cls: "bg-primary/15 text-primary" },
  [CommandStatus.COMPLETED]: { label: "completed", cls: "bg-success/15 text-success" },
  [CommandStatus.FAILED]: { label: "failed", cls: "bg-destructive/15 text-destructive" },
  [CommandStatus.CANCELLED]: { label: "cancelled", cls: "bg-muted text-muted-foreground" },
};

function renderResult(result?: Operation_Result): ReactElement | null {
  const r = result?.result;
  if (r?.case === "runCmd") return <CmdResultBlock result={r.value} />;
  if (r?.case === "makeTempDir") {
    return <div className="rounded-md border bg-[#050506] px-2.5 py-1.5 font-mono text-[11px] text-muted-foreground">tmp dir → {(r.value as { path?: string }).path}</div>;
  }
  return null;
}

function ReportView({ report }: { report: Report }) {
  const status = REPORT_STATUS[report.status] ?? { label: "—", cls: "bg-muted text-muted-foreground" };
  return (
    <div className="space-y-1.5">
      <span className={cn("inline-block rounded-sm px-1.5 py-px font-mono text-[10px] uppercase tracking-wide", status.cls)}>{status.label}</span>
      {report.error ? (
        <div className="whitespace-pre-wrap break-all rounded-sm border border-destructive/30 bg-destructive/5 p-1.5 font-mono text-[11px] leading-relaxed text-destructive/90">{report.error}</div>
      ) : null}
      {renderResult(report.result)}
    </div>
  );
}

// renderTyped turns a recognised decoded message into readable UI, or null.
function renderTyped(typeName: string, message: unknown): ReactElement | null {
  switch (typeName) {
    case COMMAND_TYPE:
      return renderOperation((message as Command).operation);
    case REPORT_TYPE:
      return <ReportView report={message as Report} />;
    case OP_TYPE:
      return renderOperation(message as Operation);
    case FILE_TYPE:
      return <FileCard file={message as SysFile} op="file" />;
    default:
      return null;
  }
}

// PayloadView renders a decoded Any: agent commands/operations become file or
// command cards (readable), reports become status + output, and everything else
// falls back to collapsible JSON.
export function PayloadView({ any }: { any?: Any }) {
  const decoded = useMemo(() => decodeAny(any), [any]);
  const [open, setOpen] = useState(false);

  if (!decoded) return null;

  if (decoded.message) {
    const view = renderTyped(decoded.typeName, decoded.message);
    if (view) return view;
  }

  const short = decoded.typeName.slice(decoded.typeName.lastIndexOf(".") + 1) || decoded.typeName;
  const canExpand = decoded.json !== null;
  return (
    <div>
      <button
        type="button"
        disabled={!canExpand}
        onClick={() => setOpen((o) => !o)}
        className={cn("flex w-full items-center gap-1.5 text-left text-[11px] font-mono", canExpand && "hover:text-foreground")}
      >
        <span className="text-foreground/80" title={decoded.typeName}>{short}</span>
        <span className="text-muted-foreground/60">· {decoded.bytes} B</span>
        {canExpand ? <ChevronRight className={cn("size-3 text-muted-foreground/60 transition-transform", open && "rotate-90")} /> : <span className="text-[9px] text-muted-foreground/40">opaque</span>}
      </button>
      {open && canExpand ? (
        <div className="mt-1 overflow-hidden rounded-md border">
          <JsonPanel value={decoded.json} />
        </div>
      ) : null}
    </div>
  );
}
