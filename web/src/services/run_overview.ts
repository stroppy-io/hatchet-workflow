// Run-detail data surface — wraps cloud.v1.api.TestRunOverviewService (overview
// snapshot, metrics, log query, optional log stream) + a single getTestRun via
// TestRunService for the header record. Proto -> flat VM via toJson, reusing the
// shared statusToVM / enum helpers.

import { fromJson, toJson } from "@bufbuild/protobuf";
import { TestRunRecordSchema } from "@/lib/proto/cloud/v1/models/test_run_pb";
import {
  GetTestRunOverviewResponseSchema,
  GetRunMetricsResponseSchema,
  QueryLogsResponseSchema,
  LogScrollDirection,
} from "@/lib/proto/cloud/v1/api/test_run_overview_pb";
import { LogCursorSchema, LogLineSchema } from "@/lib/proto/cloud/v1/monitor/logs_pb";
import type { TopologyJson } from "@/lib/proto/cloud/v1/topology/topology_pb";
import {
  testRunOverviewClient,
  testRunClient,
} from "@/services/client";
import { resolveTenantId } from "@/services/tenant";
import { statusToVM, type RunStatus } from "@/services/dashboard";
import { dbKindLabelFromJson, providerLabelFromJson } from "@/services/enums";
import { testRunRecordToVM, type RunVM } from "@/services/runs";

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
}

/** One timeline event flattened from monitor.Event. */
export interface TimelineEventVM {
  at: string;
  kind: string;
  message: string;
  nodeExecutionId: string;
  status: RunStatus;
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
  componentId: string;
  machineId: string;
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
  nodeExecutionIds?: string[];
  componentIds?: string[];
  /** "older" pages back in time, "newer" forward; default newer. */
  direction?: "older" | "newer";
  limit?: number;
  /** Opaque anchor cursor from a prior page (LogPageVM.older / .newer). */
  from?: unknown;
}

const num = (v: number | "NaN" | "Infinity" | "-Infinity" | undefined): number =>
  typeof v === "number" ? v : 0;

const durSec = (d: { seconds?: string | number; nanos?: number } | string | undefined): number | undefined => {
  if (d === undefined) return undefined;
  if (typeof d === "string") return parseFloat(d) || 0;
  const s = d.seconds === undefined ? 0 : Number(d.seconds);
  return s + (d.nanos ?? 0) / 1e9;
};

function nodeToVM(n: {
  nodeExecutionId?: string;
  name?: string;
  status?: string;
  startedAt?: string;
  finishedAt?: string;
  duration?: string;
  attempt?: number;
  children?: unknown[];
}): PipelineNodeVM {
  return {
    nodeExecutionId: n.nodeExecutionId ?? "",
    name: n.name ?? "",
    status: statusToVM(n.status),
    startedAt: n.startedAt,
    finishedAt: n.finishedAt,
    durationSec: durSec(n.duration),
    attempt: n.attempt ?? 0,
    children: (n.children ?? []).map((c) => nodeToVM(c as Parameters<typeof nodeToVM>[0])),
  };
}

export async function getRunOverview(
  tenantSlug: string,
  runId: string,
): Promise<OverviewVM> {
  const tenantId = await resolveTenantId(tenantSlug);
  const resp = await testRunOverviewClient.getTestRunOverview({ tenantId, runId });
  const j = toJson(GetTestRunOverviewResponseSchema, resp) as {
    snapshot?: {
      run?: unknown;
      topology?: TopologyJson;
      suiteRun?: { entity?: { id?: string } };
      overview?: {
        runId?: string;
        status?: string;
        startedAt?: string;
        finishedAt?: string;
        duration?: string;
        progressPct?: number;
        pipeline?: { roots?: unknown[] };
        workers?: Array<{
          id?: string;
          kind?: string;
          machineId?: string;
          host?: string;
          online?: boolean;
          status?: string;
          currentNodeExecutionId?: string;
        }>;
        timeline?: Array<{
          at?: string;
          kind?: string;
          message?: string;
          nodeExecutionId?: string;
          status?: string;
        }>;
      };
    };
  };
  const snap = j.snapshot ?? {};
  const ov = snap.overview ?? {};
  const run = resp.snapshot?.run
    ? testRunRecordToVM(resp.snapshot.run)
    : undefined;
  return {
    runId: ov.runId ?? runId,
    status: statusToVM(ov.status),
    startedAt: ov.startedAt,
    finishedAt: ov.finishedAt,
    durationSec: durSec(ov.duration),
    progressPct: ov.progressPct ?? 0,
    pipeline: (ov.pipeline?.roots ?? []).map((n) => nodeToVM(n as Parameters<typeof nodeToVM>[0])),
    workers: (ov.workers ?? []).map((w) => ({
      id: w.id ?? "",
      kind: w.kind ?? "",
      machineId: w.machineId ?? "",
      host: w.host ?? "",
      online: w.online ?? false,
      status: statusToVM(w.status),
      currentNodeExecutionId: w.currentNodeExecutionId ?? "",
    })),
    timeline: (ov.timeline ?? []).map((e) => ({
      at: e.at ?? "",
      kind: e.kind ?? "",
      message: e.message ?? "",
      nodeExecutionId: e.nodeExecutionId ?? "",
      status: statusToVM(e.status),
    })),
    run,
    suiteRunId: snap.suiteRun?.entity?.id ?? "",
    topology: snap.topology,
  };
}

export async function getRunMetrics(
  tenantSlug: string,
  runId: string,
): Promise<MetricVM[]> {
  const tenantId = await resolveTenantId(tenantSlug);
  const resp = await testRunOverviewClient.getRunMetrics({ tenantId, runId });
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

export async function queryLogs(
  tenantSlug: string,
  runId: string,
  query: LogQuery = {},
): Promise<LogPageVM> {
  const tenantId = await resolveTenantId(tenantSlug);
  const resp = await testRunOverviewClient.queryLogs({
    tenantId,
    runId,
    filter: {
      search: query.search ?? "",
      nodeExecutionIds: query.nodeExecutionIds ?? [],
      componentIds: query.componentIds ?? [],
    },
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
  componentId?: string;
  machineId?: string;
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
    componentId: l.componentId ?? "",
    machineId: l.machineId ?? "",
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
          nodeExecutionIds: query.nodeExecutionIds ?? [],
          componentIds: query.componentIds ?? [],
        },
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
 * Stream a few overview frames (server stream). Calls the callback per frame
 * until the signal aborts or the stream ends. Optional / best-effort.
 */
export async function streamRunOverview(
  tenantSlug: string,
  runId: string,
  onFrame: (vm: OverviewVM) => void,
  signal?: AbortSignal,
): Promise<void> {
  const tenantId = await resolveTenantId(tenantSlug);
  for await (const snap of testRunOverviewClient.streamTestRunOverview(
    { tenantId, runId },
    { signal },
  )) {
    const ov = snap.overview;
    if (!ov) continue;
    onFrame({
      runId: ov.runId || runId,
      status: statusToVM(statusJson(ov.status)),
      progressPct: ov.progressPct,
      pipeline: [],
      workers: [],
      timeline: [],
      suiteRunId: "",
    });
  }
}

// status comes back as a numeric proto enum on the live message; convert to the
// JSON-string form statusToVM expects. We only need a coarse mapping here.
function statusJson(s: number): string {
  switch (s) {
    case 1:
      return "STATUS_PENDING";
    case 2:
      return "STATUS_RUNNING";
    case 7:
      return "STATUS_CANCELLING";
    case 8:
      return "STATUS_COMPLETED";
    case 9:
      return "STATUS_FAILED";
    case 10:
      return "STATUS_CANCELLED";
    default:
      return "STATUS_UNSPECIFIED";
  }
}

/** Fetch just the run record header (TestRunService.GetTestRun -> RunVM). */
export async function getRunRecord(
  tenantSlug: string,
  runId: string,
): Promise<RunVM | undefined> {
  const tenantId = await resolveTenantId(tenantSlug);
  const resp = await testRunClient.getTestRun({ tenantId, id: runId });
  return resp.run ? testRunRecordToVM(resp.run) : undefined;
}

// Re-exported for callers that want the raw record schema/enums.
export { TestRunRecordSchema, dbKindLabelFromJson, providerLabelFromJson };
