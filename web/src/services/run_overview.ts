// Run-detail data surface — wraps cloud.v1.api.TestRunOverviewService (overview
// snapshot, metrics, log query, optional log stream). Proto -> flat VM via
// toJson, reusing the shared statusToVM / enum helpers.

import { fromJson, toJson } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { TestRunRecordSchema, type TestRunRecord } from "@/lib/proto/cloud/v1/models/test_run_pb";
import {
  GetTestRunOverviewResponseSchema,
  GetRunMetricsResponseSchema,
  GetLogFacetsResponseSchema,
  QueryLogsResponseSchema,
  TestRunOverviewSnapshotSchema,
  LogScrollDirection,
} from "@/lib/proto/cloud/v1/api/test_run_overview_pb";
import { LogCursorSchema, LogLineSchema, type Source, type Stream } from "@/lib/proto/cloud/v1/monitor/logs_pb";
import type { TopologyJson } from "@/lib/proto/cloud/v1/topology/topology_pb";
import { testRunOverviewClient } from "@/services/client";
import { resolveTenantId } from "@/services/tenant";
import { statusToVM, type RunStatus } from "@/services/dashboard";
import { dbKindLabelFromJson, providerLabelFromJson } from "@/services/enums";
import { testRunRecordToVM, type RunVM } from "@/services/runs";

/** Provenance of a snapshot field/object (monitor.ObservationSource). */
export type ObservationSource =
  | "temporal" | "persisted" | "plan" | "registry" | "synthetic" | "";
/** Registry-based worker liveness (monitor.WorkerPresence). */
export type WorkerPresence =
  | "online" | "stale" | "offline" | "terminated" | "unknown" | "";
/** Timeline emphasis (monitor.EventSeverity). */
export type EventSeverity = "info" | "warning" | "error" | "";
/** Concrete agent action class (monitor.OperationKind). */
export type OperationKind = "create_dir" | "write_file" | "fetch_file" | "call_cmd" | "";
/** Structured pipeline output class (monitor.OutputKind). */
export type OutputKind =
  | "summary" | "render_artifact" | "deployment_plan" | "command_result" | "file" | "directory" | "";

/** Generic, inspectable projection of an AgentStep (monitor.PipelineOperation). */
export interface OperationVM {
  kind: OperationKind;
  stepId: string;
  stepOrder: number;
  target: string;
  summary: string;
  mentions: string[];
  labels: Record<string, string>;
  commandText: string;
  argv: string[];
  filePath: string;
  fileSizeBytes: number;
  contentPreview: string;
  resultAvailable: boolean;
  exitCode: number;
  timedOut: boolean;
  elapsedSec?: number;
  stdoutPreview: string;
  stderrPreview: string;
  resultSummary: string;
}

/** One structured artifact/result produced by a pipeline stage. */
export interface PipelineOutputVM {
  kind: OutputKind;
  id: string;
  name: string;
  summary: string;
  componentId: string;
  machineId: string;
  stepId: string;
  action: string;
  target: string;
  commandText: string;
  contentPreview: string;
  count: number;
  sizeBytes: number;
  labels: Record<string, string>;
}

/** One pipeline stage flattened from monitor.PipelineNode (recursive). */
export interface PipelineNodeVM {
  nodeExecutionId: string;
  name: string;
  status: RunStatus;
  startedAt?: string;
  finishedAt?: string;
  durationSec?: number;
  attempt: number;
  children: PipelineNodeVM[];
  /** Provenance of this node's runtime data. */
  source: ObservationSource;
  /** Stable 1-based sibling display/execution order. */
  order: number;
  /** Parent stage's node execution id (hierarchy link). */
  parentNodeExecutionId: string;
  /** Top-level phase this node belongs to. */
  phase: string;
  /** Topology component this node acts on, when any. */
  componentId: string;
  /** Machine/agent this node targets, when any. */
  machineId: string;
  /** Short machine-readable status explanation. */
  statusReason: string;
  /** User-facing error text when this node failed. */
  errorMessage: string;
  /** Generic action projection (populated for agent step nodes). */
  operation?: OperationVM;
  /** Structured artifacts/results produced by this stage. */
  outputs: PipelineOutputVM[];
}

/** One worker flattened from monitor.WorkerInfo. */
export interface WorkerVM {
  id: string;
  kind: string;
  machineId: string;
  host: string;
  online: boolean;
  status: RunStatus;
  currentNodeExecutionId: string;
  /** Registry-based liveness — the canonical online state. */
  presence: WorkerPresence;
  statusReason: string;
  source: ObservationSource;
  registeredAt?: string;
  lastSeenAt?: string;
  heartbeatIntervalSeconds: number;
  agentVersion: string;
}

/** One timeline event flattened from monitor.Event. */
export interface TimelineEventVM {
  at: string;
  kind: string;
  message: string;
  nodeExecutionId: string;
  status: RunStatus;
  severity: EventSeverity;
  source: ObservationSource;
  sequence: number;
  detail: string;
}

/** The Overview snapshot, flattened from api.TestRunOverviewSnapshot. */
export interface OverviewVM {
  runId: string;
  status: RunStatus;
  startedAt?: string;
  finishedAt?: string;
  durationSec?: number;
  progressPct: number;
  pipeline: PipelineNodeVM[];
  workers: WorkerVM[];
  timeline: TimelineEventVM[];
  /** The persisted run record (header), if present in the snapshot. */
  run?: RunVM;
  /** Owning suite run id, when this run is a suite child. */
  suiteRunId: string;
  /** Staged topology envelope (machines / components / connections), if present. */
  topology?: TopologyJson;
  /** Server clock time this snapshot was assembled (ISO). */
  observedAt?: string;
  /** Explanations of missing/stale parts (e.g. persisted-record fallback). */
  degradedReasons: string[];
  /** Primary source of the top-level run status/progress. */
  source: ObservationSource;
}

/** One aggregated metric, flattened from monitor.MetricSummary. */
export interface MetricVM {
  key: string;
  name: string;
  unit: string;
  avg: number;
  min: number;
  max: number;
  last: number;
  higherIsBetter: boolean;
  group: string;
  description: string;
}

/** One log line, flattened from monitor.LogLine. */
export interface LogLineVM {
  observedAt: string;
  lineNo: string;
  nodeExecutionId: string;
  parentNodeExecutionId: string;
  phase: string;
  stageName: string;
  componentId: string;
  machineId: string;
  stepId: string;
  action: string;
  mentions: string[];
  unit: string;
  source: string;
  stream: string;
  line: string;
  /**
   * Stable, cross-client line identity built from the server LogCursor
   * (observedAt + seq). Used for shareable deep-links (#line=) and re-anchoring
   * a page around a specific line — unlike lineNo, it does not depend on when
   * the viewer loaded the logs.
   */
  cursorKey: string;
}

/** A page of logs + the cursors to page either way. */
export interface LogPageVM {
  lines: LogLineVM[];
  /** opaque cursor (raw proto LogCursor) to fetch the page OLDER than these. */
  older?: unknown;
  /** opaque cursor (raw proto LogCursor) to fetch the page NEWER than these. */
  newer?: unknown;
}

/** Structured slice for QueryLogs (maps onto api.LogFilter). */
export interface LogQuery {
  search?: string;
  query?: string;
  nodeExecutionIds?: string[];
  componentIds?: string[];
  nodeIds?: string[];
  machineIds?: string[];
  sources?: Source[];
  streams?: Stream[];
  unit?: string;
  units?: string[];
  phases?: string[];
  parentNodeExecutionIds?: string[];
  stageNames?: string[];
  stepIds?: string[];
  actions?: string[];
  mentions?: string[];
  /** "older" pages back in time, "newer" forward; default newer. */
  direction?: "older" | "newer";
  limit?: number;
  /** Opaque anchor cursor from a prior page (LogPageVM.older / .newer). */
  from?: unknown;
  /** Keep lines at or after this time (LogFilter.start). */
  start?: Date;
  /** Keep lines at or before this time (LogFilter.end). */
  end?: Date;
}

const num = (v: number | "NaN" | "Infinity" | "-Infinity" | undefined): number =>
  typeof v === "number" ? v : 0;

const durSec = (d: { seconds?: string | number; nanos?: number } | string | undefined): number | undefined => {
  if (d === undefined) return undefined;
  if (typeof d === "string") return parseFloat(d) || 0;
  const s = d.seconds === undefined ? 0 : Number(d.seconds);
  return s + (d.nanos ?? 0) / 1e9;
};

const obsSource = (s?: string): ObservationSource =>
  (({
    OBSERVATION_SOURCE_TEMPORAL_RUN_STATE: "temporal",
    OBSERVATION_SOURCE_PERSISTED_RECORD: "persisted",
    OBSERVATION_SOURCE_DEPLOYMENT_PLAN: "plan",
    OBSERVATION_SOURCE_AGENT_REGISTRY: "registry",
    OBSERVATION_SOURCE_SYNTHETIC: "synthetic",
  } as Record<string, ObservationSource>)[s ?? ""] ?? "");

const workerPresence = (s?: string): WorkerPresence =>
  (({
    WORKER_PRESENCE_ONLINE: "online",
    WORKER_PRESENCE_STALE: "stale",
    WORKER_PRESENCE_OFFLINE: "offline",
    WORKER_PRESENCE_TERMINATED: "terminated",
    WORKER_PRESENCE_UNKNOWN: "unknown",
  } as Record<string, WorkerPresence>)[s ?? ""] ?? "");

const eventSeverity = (s?: string): EventSeverity =>
  (({
    EVENT_SEVERITY_INFO: "info",
    EVENT_SEVERITY_WARNING: "warning",
    EVENT_SEVERITY_ERROR: "error",
  } as Record<string, EventSeverity>)[s ?? ""] ?? "");

const operationKind = (s?: string): OperationKind =>
  (({
    OPERATION_KIND_CREATE_DIR: "create_dir",
    OPERATION_KIND_WRITE_FILE: "write_file",
    OPERATION_KIND_FETCH_FILE: "fetch_file",
    OPERATION_KIND_CALL_CMD: "call_cmd",
  } as Record<string, OperationKind>)[s ?? ""] ?? "");

const outputKind = (s?: string): OutputKind =>
  (({
    OUTPUT_KIND_SUMMARY: "summary",
    OUTPUT_KIND_RENDER_ARTIFACT: "render_artifact",
    OUTPUT_KIND_DEPLOYMENT_PLAN: "deployment_plan",
    OUTPUT_KIND_COMMAND_RESULT: "command_result",
    OUTPUT_KIND_FILE: "file",
    OUTPUT_KIND_DIRECTORY: "directory",
  } as Record<string, OutputKind>)[s ?? ""] ?? "");

function operationToVM(o?: {
  kind?: string;
  stepId?: string;
  stepOrder?: number;
  target?: string;
  summary?: string;
  mentions?: string[];
  labels?: Record<string, string>;
  commandText?: string;
  argv?: string[];
  filePath?: string;
  fileSizeBytes?: string;
  contentPreview?: string;
  resultAvailable?: boolean;
  exitCode?: number;
  timedOut?: boolean;
  elapsed?: { seconds?: string | number; nanos?: number } | string;
  stdoutPreview?: string;
  stderrPreview?: string;
  resultSummary?: string;
}): OperationVM | undefined {
  if (!o) return undefined;
  return {
    kind: operationKind(o.kind),
    stepId: o.stepId ?? "",
    stepOrder: o.stepOrder ?? 0,
    target: o.target ?? "",
    summary: o.summary ?? "",
    mentions: o.mentions ?? [],
    labels: o.labels ?? {},
    commandText: o.commandText ?? "",
    argv: o.argv ?? [],
    filePath: o.filePath ?? "",
    fileSizeBytes: o.fileSizeBytes ? Number(o.fileSizeBytes) : 0,
    contentPreview: o.contentPreview ?? "",
    resultAvailable: o.resultAvailable ?? false,
    exitCode: o.exitCode ?? 0,
    timedOut: o.timedOut ?? false,
    elapsedSec: durSec(o.elapsed),
    stdoutPreview: o.stdoutPreview ?? "",
    stderrPreview: o.stderrPreview ?? "",
    resultSummary: o.resultSummary ?? "",
  };
}

function outputToVM(o: {
  kind?: string;
  id?: string;
  name?: string;
  summary?: string;
  componentId?: string;
  machineId?: string;
  stepId?: string;
  action?: string;
  target?: string;
  commandText?: string;
  contentPreview?: string;
  count?: number;
  sizeBytes?: string;
  labels?: Record<string, string>;
}): PipelineOutputVM {
  return {
    kind: outputKind(o.kind),
    id: o.id ?? "",
    name: o.name ?? "",
    summary: o.summary ?? "",
    componentId: o.componentId ?? "",
    machineId: o.machineId ?? "",
    stepId: o.stepId ?? "",
    action: o.action ?? "",
    target: o.target ?? "",
    commandText: o.commandText ?? "",
    contentPreview: o.contentPreview ?? "",
    count: o.count ?? 0,
    sizeBytes: o.sizeBytes ? Number(o.sizeBytes) : 0,
    labels: o.labels ?? {},
  };
}

interface PipelineNodeJson {
  nodeExecutionId?: string;
  name?: string;
  status?: string;
  startedAt?: string;
  finishedAt?: string;
  duration?: string;
  attempt?: number;
  children?: unknown[];
  source?: string;
  order?: number;
  parentNodeExecutionId?: string;
  phase?: string;
  componentId?: string;
  machineId?: string;
  statusReason?: string;
  errorMessage?: string;
  operation?: Parameters<typeof operationToVM>[0];
  outputs?: unknown[];
}

function nodeToVM(n: PipelineNodeJson): PipelineNodeVM {
  return {
    nodeExecutionId: n.nodeExecutionId ?? "",
    name: n.name ?? "",
    status: statusToVM(n.status),
    startedAt: n.startedAt,
    finishedAt: n.finishedAt,
    durationSec: durSec(n.duration),
    attempt: n.attempt ?? 0,
    children: (n.children ?? []).map((c) => nodeToVM(c as PipelineNodeJson)),
    source: obsSource(n.source),
    order: n.order ?? 0,
    parentNodeExecutionId: n.parentNodeExecutionId ?? "",
    phase: n.phase ?? "",
    componentId: n.componentId ?? "",
    machineId: n.machineId ?? "",
    statusReason: n.statusReason ?? "",
    errorMessage: n.errorMessage ?? "",
    operation: operationToVM(n.operation),
    outputs: (n.outputs ?? []).map((o) => outputToVM(o as Parameters<typeof outputToVM>[0])),
  };
}

/** Shape of api.TestRunOverviewSnapshot after toJson (the fields we read). */
interface SnapshotJson {
  topology?: TopologyJson;
  suiteRun?: { entity?: { id?: string } };
  overview?: {
    runId?: string;
    status?: string;
    startedAt?: string;
    finishedAt?: string;
    duration?: string;
    progressPct?: number;
    observedAt?: string;
    degradedReasons?: string[];
    source?: string;
    pipeline?: { roots?: unknown[] };
    workers?: Array<{
      id?: string;
      kind?: string;
      machineId?: string;
      host?: string;
      online?: boolean;
      status?: string;
      currentNodeExecutionId?: string;
      presence?: string;
      statusReason?: string;
      source?: string;
      registeredAt?: string;
      lastSeenAt?: string;
      heartbeatIntervalSeconds?: number;
      agentVersion?: string;
    }>;
    timeline?: Array<{
      at?: string;
      kind?: string;
      message?: string;
      nodeExecutionId?: string;
      status?: string;
      severity?: string;
      source?: string;
      sequence?: string;
      detail?: string;
    }>;
  };
}

// Map a snapshot (JSON for overview/topology/suite + raw run record proto for
// the header) onto OverviewVM. Shared by the one-shot fetch and the live stream.
function snapshotToVM(snap: SnapshotJson, runRecord: TestRunRecord | undefined, runId: string): OverviewVM {
  const ov = snap.overview ?? {};
  return {
    runId: ov.runId ?? runId,
    status: statusToVM(ov.status),
    startedAt: ov.startedAt,
    finishedAt: ov.finishedAt,
    durationSec: durSec(ov.duration),
    progressPct: ov.progressPct ?? 0,
    pipeline: (ov.pipeline?.roots ?? []).map((n) => nodeToVM(n as PipelineNodeJson)),
    workers: (ov.workers ?? []).map((w) => ({
      id: w.id ?? "",
      kind: w.kind ?? "",
      machineId: w.machineId ?? "",
      host: w.host ?? "",
      online: w.online ?? false,
      status: statusToVM(w.status),
      currentNodeExecutionId: w.currentNodeExecutionId ?? "",
      presence: workerPresence(w.presence),
      statusReason: w.statusReason ?? "",
      source: obsSource(w.source),
      registeredAt: w.registeredAt,
      lastSeenAt: w.lastSeenAt,
      heartbeatIntervalSeconds: w.heartbeatIntervalSeconds ?? 0,
      agentVersion: w.agentVersion ?? "",
    })),
    timeline: (ov.timeline ?? []).map((e) => ({
      at: e.at ?? "",
      kind: e.kind ?? "",
      message: e.message ?? "",
      nodeExecutionId: e.nodeExecutionId ?? "",
      status: statusToVM(e.status),
      severity: eventSeverity(e.severity),
      source: obsSource(e.source),
      sequence: e.sequence ? Number(e.sequence) : 0,
      detail: e.detail ?? "",
    })),
    run: runRecord ? testRunRecordToVM(runRecord) : undefined,
    suiteRunId: snap.suiteRun?.entity?.id ?? "",
    topology: snap.topology,
    observedAt: ov.observedAt,
    degradedReasons: ov.degradedReasons ?? [],
    source: obsSource(ov.source),
  };
}

export async function getRunOverview(
  tenantSlug: string,
  runId: string,
): Promise<OverviewVM> {
  const tenantId = await resolveTenantId(tenantSlug);
  const resp = await testRunOverviewClient.getTestRunOverview({ tenantId, runId });
  const j = toJson(GetTestRunOverviewResponseSchema, resp) as { snapshot?: SnapshotJson };
  return snapshotToVM(j.snapshot ?? {}, resp.snapshot?.run, runId);
}

/** Optional [startedAt, finishedAt] ISO window to scope the metrics aggregation to. */
export interface MetricsWindow {
  startedAt?: string;
  finishedAt?: string;
}

export async function getRunMetrics(
  tenantSlug: string,
  runId: string,
  window?: MetricsWindow,
): Promise<MetricVM[]> {
  const tenantId = await resolveTenantId(tenantSlug);
  const win =
    window?.startedAt && window?.finishedAt
      ? {
          start: timestampFromDate(new Date(window.startedAt)),
          end: timestampFromDate(new Date(window.finishedAt)),
        }
      : undefined;
  const resp = await testRunOverviewClient.getRunMetrics({ tenantId, runId, window: win });
  const j = toJson(GetRunMetricsResponseSchema, resp) as {
    metrics?: {
      metrics?: Array<{
        key?: string;
        name?: string;
        unit?: string;
        avg?: number | "NaN" | "Infinity" | "-Infinity";
        min?: number | "NaN" | "Infinity" | "-Infinity";
        max?: number | "NaN" | "Infinity" | "-Infinity";
        last?: number | "NaN" | "Infinity" | "-Infinity";
        higherIsBetter?: boolean;
        group?: string;
        description?: string;
      }>;
    };
  };
  return (j.metrics?.metrics ?? []).map((m) => ({
    key: m.key ?? "",
    name: m.name ?? m.key ?? "",
    unit: m.unit ?? "",
    avg: num(m.avg),
    min: num(m.min),
    max: num(m.max),
    last: num(m.last),
    higherIsBetter: m.higherIsBetter ?? false,
    group: m.group ?? "",
    description: m.description ?? "",
  }));
}

/** A workload segment's name + its [started, finished] window, for scoping. */
export interface SegmentWindowVM {
  name: string;
  startedAt?: string;
  finishedAt?: string;
}

/**
 * The per-segment run steps under the workload stage, in execution order — the
 * leaf "workload"-phase pipeline nodes, each with its own time window. Used to
 * scope Grafana / metrics to a single segment (e.g. the measured workload,
 * excluding bootstrap). Empty until the workload stage starts.
 */
export function workloadSegments(overview: OverviewVM | null | undefined): SegmentWindowVM[] {
  const out: SegmentWindowVM[] = [];
  const walk = (nodes: PipelineNodeVM[]) => {
    for (const n of nodes) {
      if (n.phase === "workload" && n.children.length === 0 && n.startedAt) {
        out.push({ name: n.name, startedAt: n.startedAt, finishedAt: n.finishedAt });
      }
      if (n.children.length) walk(n.children);
    }
  };
  walk(overview?.pipeline ?? []);
  return out;
}

/** Map a LogQuery slice onto the wire api.LogFilter shape (shared by query /
 *  stream / facets so all three narrow identically). */
function toLogFilter(query: LogQuery) {
  return {
    search: query.search ?? "",
    query: query.query ?? "",
    nodeExecutionIds: query.nodeExecutionIds ?? [],
    componentIds: query.componentIds ?? [],
    nodeIds: query.nodeIds ?? [],
    machineIds: query.machineIds ?? [],
    sources: query.sources ?? [],
    streams: query.streams ?? [],
    unit: query.unit ?? "",
    units: query.units ?? [],
    phases: query.phases ?? [],
    parentNodeExecutionIds: query.parentNodeExecutionIds ?? [],
    stageNames: query.stageNames ?? [],
    stepIds: query.stepIds ?? [],
    actions: query.actions ?? [],
    mentions: query.mentions ?? [],
    start: query.start ? timestampFromDate(query.start) : undefined,
    end: query.end ? timestampFromDate(query.end) : undefined,
  };
}

/** One distinct value of a log filter dimension + its hit count over the run. */
export interface LogFacetValueVM {
  value: string;
  count: number;
}
/** Server-side facets keyed by wire field name (component_id, machine_id, …). */
export type LogFacetsVM = Record<string, LogFacetValueVM[]>;

/**
 * Distinct values + counts of every log filter dimension across the WHOLE run
 * (narrowed by the same filter), so the dropdowns don't depend on which page of
 * logs is loaded. `query` cross-narrows exactly like queryLogs.
 */
export async function getLogFacets(
  tenantSlug: string,
  runId: string,
  query: LogQuery = {},
): Promise<LogFacetsVM> {
  const tenantId = await resolveTenantId(tenantSlug);
  const resp = await testRunOverviewClient.getLogFacets({
    tenantId,
    runId,
    filter: toLogFilter(query),
  });
  const j = toJson(GetLogFacetsResponseSchema, resp) as {
    fields?: Array<{ field?: string; values?: Array<{ value?: string; count?: string | number }> }>;
  };
  const out: LogFacetsVM = {};
  for (const f of j.fields ?? []) {
    if (!f.field) continue;
    out[f.field] = (f.values ?? [])
      .filter((v) => !!v.value)
      .map((v) => ({ value: v.value as string, count: Number(v.count ?? 0) }));
  }
  return out;
}

export async function queryLogs(
  tenantSlug: string,
  runId: string,
  query: LogQuery = {},
): Promise<LogPageVM> {
  const tenantId = await resolveTenantId(tenantSlug);
  const resp = await testRunOverviewClient.queryLogs({
    tenantId,
    runId,
    filter: toLogFilter(query),
    direction:
      query.direction === "older"
        ? LogScrollDirection.OLDER
        : LogScrollDirection.NEWER,
    limit: query.limit ?? 200,
    // Anchor: a raw LogCursor returned by a prior page (load-older / load-newer).
    from: query.from as never,
  });
  const j = toJson(QueryLogsResponseSchema, resp) as {
    lines?: Array<LogLineJson>;
    older?: unknown;
    newer?: unknown;
  };
  return {
    lines: (j.lines ?? []).map(logLineToVM),
    // Raw proto cursors so the caller can feed them straight back as `from`.
    older: resp.older,
    newer: resp.newer,
  };
}

/** Shape of a monitor.LogLine after toJson (camelCase, optional everything). */
type LogLineJson = {
  observedAt?: string;
  lineNo?: string;
  nodeExecutionId?: string;
  parentNodeExecutionId?: string;
  phase?: string;
  stageName?: string;
  componentId?: string;
  machineId?: string;
  stepId?: string;
  action?: string;
  mentions?: string[];
  unit?: string;
  source?: string;
  stream?: string;
  line?: string;
  cursor?: { observedAt?: string; seq?: string };
};

function cursorKeyOf(c?: { observedAt?: string; seq?: string }): string {
  if (!c || (!c.observedAt && !c.seq)) return "";
  return `${c.observedAt ?? ""}@${c.seq ?? ""}`;
}

function logLineToVM(l: LogLineJson): LogLineVM {
  return {
    observedAt: l.observedAt ?? "",
    lineNo: l.lineNo ?? "",
    nodeExecutionId: l.nodeExecutionId ?? "",
    parentNodeExecutionId: l.parentNodeExecutionId ?? "",
    phase: l.phase ?? "",
    stageName: l.stageName ?? "",
    componentId: l.componentId ?? "",
    machineId: l.machineId ?? "",
    stepId: l.stepId ?? "",
    action: l.action ?? "",
    mentions: l.mentions ?? [],
    unit: l.unit ?? "",
    source: l.source ?? "",
    stream: l.stream ?? "",
    line: l.line ?? "",
    cursorKey: cursorKeyOf(l.cursor),
  };
}

/**
 * Rebuild a raw LogCursor from a cursorKey ("<observedAt>@<seq>") so a shared
 * #line= anchor can be passed back to QueryLogs as `from` to re-fetch the page
 * around that exact line. Returns undefined for an empty/garbled key.
 */
export function logCursorFromKey(key: string): unknown {
  const at = key.indexOf("@");
  if (at < 0) return undefined;
  const observedAt = key.slice(0, at);
  const seq = key.slice(at + 1);
  if (!observedAt && !seq) return undefined;
  const obj: Record<string, string> = { seq: seq || "0" };
  if (observedAt) obj.observedAt = observedAt;
  return fromJson(LogCursorSchema, obj);
}

/**
 * Open a live log tail (TestRunOverviewService.StreamLogs, server stream).
 * Each frame is ONE monitor.LogLine; the helper maps it to a LogLineVM and
 * invokes onFrame per line. The loop ends when the stream completes or the
 * AbortSignal fires; the caller owns the AbortController and aborts on
 * toggle-off / filter change / unmount. An AbortError (from signal abort) is
 * swallowed so cancelling the tail is not surfaced as an error.
 */
export async function streamLogs(
  tenantSlug: string,
  runId: string,
  query: LogQuery,
  signal: AbortSignal,
  onFrame: (line: LogLineVM) => void,
): Promise<void> {
  const tenantId = await resolveTenantId(tenantSlug);
  try {
    for await (const frame of testRunOverviewClient.streamLogs(
      {
        tenantId,
        runId,
        filter: {
          search: query.search ?? "",
          query: query.query ?? "",
          nodeExecutionIds: query.nodeExecutionIds ?? [],
          componentIds: query.componentIds ?? [],
          nodeIds: query.nodeIds ?? [],
          machineIds: query.machineIds ?? [],
          sources: query.sources ?? [],
          streams: query.streams ?? [],
          unit: query.unit ?? "",
          units: query.units ?? [],
          phases: query.phases ?? [],
          parentNodeExecutionIds: query.parentNodeExecutionIds ?? [],
          stageNames: query.stageNames ?? [],
          stepIds: query.stepIds ?? [],
          actions: query.actions ?? [],
          mentions: query.mentions ?? [],
          start: query.start ? timestampFromDate(query.start) : undefined,
          end: query.end ? timestampFromDate(query.end) : undefined,
        },
        from: query.from as never,
      },
      { signal },
    )) {
      if (signal.aborted) break;
      onFrame(logLineToVM(toJson(LogLineSchema, frame) as LogLineJson));
    }
  } catch (err) {
    // Aborting the stream (signal.abort) rejects with an AbortError-like; that
    // is an expected, caller-initiated stop, not a failure.
    if (signal.aborted) return;
    if (err instanceof Error && err.name === "AbortError") return;
    throw err;
  }
}

/**
 * Resolve a log deep-link ref to a concrete filter + anchor. Thin pass-through
 * exercising ResolveLogRef; returns the proto response untouched.
 */
export async function resolveLogRef(
  tenantSlug: string,
  runId: string,
  ref: { nodeExecutionId?: string; componentId?: string } = {},
) {
  const tenantId = await resolveTenantId(tenantSlug);
  return testRunOverviewClient.resolveLogRef({
    tenantId,
    ref: {
      runId,
      nodeExecutionId: ref.nodeExecutionId ?? "",
      componentId: ref.componentId ?? "",
    },
  });
}

/**
 * Live overview stream (TestRunOverviewService.StreamTestRunOverview, server
 * stream). Each frame is a FULL TestRunOverviewSnapshot — mapped exactly like
 * the one-shot fetch so the UI updates in place. The loop ends when the stream
 * completes or the AbortSignal fires; the caller owns the controller. An
 * AbortError from signal abort is swallowed (caller-initiated stop).
 */
export async function streamRunOverview(
  tenantSlug: string,
  runId: string,
  onFrame: (vm: OverviewVM) => void,
  signal?: AbortSignal,
): Promise<void> {
  const tenantId = await resolveTenantId(tenantSlug);
  try {
    for await (const snap of testRunOverviewClient.streamTestRunOverview(
      { tenantId, runId },
      { signal },
    )) {
      if (signal?.aborted) break;
      const j = toJson(TestRunOverviewSnapshotSchema, snap) as SnapshotJson;
      onFrame(snapshotToVM(j, snap.run, runId));
    }
  } catch (err) {
    if (signal?.aborted) return;
    if (err instanceof Error && err.name === "AbortError") return;
    throw err;
  }
}

// Re-exported for callers that want the raw record schema/enums.
export { TestRunRecordSchema, dbKindLabelFromJson, providerLabelFromJson };
