// Compare data surface — wraps cloud.v1.api.CompareService.CompareRuns. Maps the
// CompareView (per-run config columns + monitor.Comparison metric rows) onto a
// flat VM the Compare page renders as a table.

import { toJson } from "@bufbuild/protobuf";
import { CompareRunsResponseSchema } from "@/lib/proto/cloud/v1/api/compare_pb";
import { compareClient } from "@/services/client";
import { resolveTenantId } from "@/services/tenant";
import { statusToVM, type RunStatus } from "@/services/dashboard";
import {
  dbKindLabelFromJson,
  providerLabelFromJson,
  type DeployProvider,
} from "@/services/enums";

const num = (v: number | "NaN" | "Infinity" | "-Infinity" | undefined): number =>
  typeof v === "number" ? v : 0;

const durSec = (d: { seconds?: string | number; nanos?: number } | undefined): number | undefined => {
  if (!d) return undefined;
  const s = d.seconds === undefined ? 0 : Number(d.seconds);
  return s + (d.nanos ?? 0) / 1e9;
};

/** One run's config column, flattened from api.RunColumn. */
export interface CompareColumnVM {
  runId: string;
  name: string;
  status: RunStatus;
  dbKind: string;
  dbName: string;
  workloadName: string;
  stroppyVersion: string;
  provider: DeployProvider;
  topologyLabel: string;
  nodeCount: number;
  startedAt?: string;
  durationSec?: number;
}

/** One run's value for a metric, flattened from monitor.MetricCell. */
export interface CompareCellVM {
  runId: string;
  avg: number;
  max: number;
  diffAvgPct: number;
  diffMaxPct: number;
  verdict: string;
}

/** One metric row across all compared runs, flattened from monitor.MetricRow. */
export interface CompareMetricVM {
  key: string;
  name: string;
  unit: string;
  higherIsBetter: boolean;
  group: string;
  cells: CompareCellVM[];
}

/** The full comparison view. */
export interface CompareVM {
  runIds: string[];
  columns: CompareColumnVM[];
  metrics: CompareMetricVM[];
}

export async function compareRuns(
  tenantSlug: string,
  runIds: string[],
): Promise<CompareVM> {
  const tenantId = await resolveTenantId(tenantSlug);
  const resp = await compareClient.compareRuns({ tenantId, runIds });
  const j = toJson(CompareRunsResponseSchema, resp) as {
    view?: {
      columns?: Array<{
        runId?: string;
        name?: string;
        status?: string;
        dbKind?: string;
        dbName?: string;
        workloadName?: string;
        stroppyVersion?: string;
        provider?: string;
        topologyLabel?: string;
        nodeCount?: number;
        startedAt?: string;
        duration?: { seconds?: string | number; nanos?: number };
      }>;
      metrics?: {
        runIds?: string[];
        metrics?: Array<{
          key?: string;
          name?: string;
          unit?: string;
          higherIsBetter?: boolean;
          group?: string;
          cells?: Array<{
            runId?: string;
            avg?: number | "NaN" | "Infinity" | "-Infinity";
            max?: number | "NaN" | "Infinity" | "-Infinity";
            diffAvgPct?: number | "NaN" | "Infinity" | "-Infinity";
            diffMaxPct?: number | "NaN" | "Infinity" | "-Infinity";
            verdict?: string;
          }>;
        }>;
      };
    };
  };
  const view = j.view ?? {};
  return {
    runIds: view.metrics?.runIds ?? runIds,
    columns: (view.columns ?? []).map((c) => ({
      runId: c.runId ?? "",
      name: c.name ?? "",
      status: statusToVM(c.status),
      dbKind: dbKindLabelFromJson(c.dbKind),
      dbName: c.dbName ?? "",
      workloadName: c.workloadName ?? "",
      stroppyVersion: c.stroppyVersion ?? "",
      provider: providerLabelFromJson(c.provider),
      topologyLabel: c.topologyLabel ?? "",
      nodeCount: c.nodeCount ?? 0,
      startedAt: c.startedAt,
      durationSec: durSec(c.duration),
    })),
    metrics: (view.metrics?.metrics ?? []).map((m) => ({
      key: m.key ?? "",
      name: m.name ?? m.key ?? "",
      unit: m.unit ?? "",
      higherIsBetter: m.higherIsBetter ?? false,
      group: m.group ?? "",
      cells: (m.cells ?? []).map((cell) => ({
        runId: cell.runId ?? "",
        avg: num(cell.avg),
        max: num(cell.max),
        diffAvgPct: num(cell.diffAvgPct),
        diffMaxPct: num(cell.diffMaxPct),
        verdict: cell.verdict ?? "",
      })),
    })),
  };
}
