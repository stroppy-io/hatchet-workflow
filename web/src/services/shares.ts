// Shares data surface — wraps cloud.v1.api.ShareService (create/get/list/revoke/
// delete/setExpiry) + the public cloud.v1.api.PublicShareService.GetSharedRun.
// Proto -> flat VM via toJson, reusing statusToVM + enum helpers.

import { toJson } from "@bufbuild/protobuf";
import {
  CreateShareResponseSchema,
  ListSharesResponseSchema,
  GetShareResponseSchema,
} from "@/lib/proto/cloud/v1/api/share_pb";
import { GetSharedRunResponseSchema } from "@/lib/proto/cloud/v1/api/public_share_pb";
import {
  ShareRecordSchema,
  ShareRecord_Target_Kind,
  type ShareRecord,
} from "@/lib/proto/cloud/v1/models/share_pb";
import { shareClient, publicShareClient } from "@/services/client";
import { resolveTenantId } from "@/services/tenant";
import { statusToVM, type RunStatus } from "@/services/dashboard";
import { dbKindLabelFromJson, providerLabelFromJson } from "@/services/enums";
import type { MetricVM } from "@/services/run_overview";

/** A share record, flattened from models.ShareRecord. */
export interface ShareVM {
  id: string;
  name: string;
  token: string;
  targetKind: "test_run" | "suite_run" | "";
  targetId: string;
  expiresAt?: string;
  revoked: boolean;
  createdAt?: string;
  /** captured_at of the snapshot, if any. */
  capturedAt?: string;
  /** the target run's display name from the snapshot, when present. */
  snapshotName: string;
}

function targetKind(s: string | undefined): ShareVM["targetKind"] {
  return s === "KIND_TEST_RUN"
    ? "test_run"
    : s === "KIND_SUITE_RUN"
      ? "suite_run"
      : "";
}

function shareToVM(rec: ShareRecord): ShareVM {
  const j = toJson(ShareRecordSchema, rec) as {
    entity?: { id?: string; name?: string; timings?: { createdAt?: string } };
    target?: { kind?: string; id?: string };
    token?: string;
    expiresAt?: string;
    revoked?: boolean;
    snapshot?: {
      capturedAt?: string;
      testRun?: { name?: string };
      suiteRun?: { name?: string };
    };
  };
  return {
    id: j.entity?.id ?? "",
    name: j.entity?.name ?? "",
    token: j.token ?? "",
    targetKind: targetKind(j.target?.kind),
    targetId: j.target?.id ?? "",
    expiresAt: j.expiresAt,
    revoked: j.revoked ?? false,
    createdAt: j.entity?.timings?.createdAt,
    capturedAt: j.snapshot?.capturedAt,
    snapshotName: j.snapshot?.testRun?.name ?? j.snapshot?.suiteRun?.name ?? "",
  };
}

export async function listShares(
  tenantSlug: string,
  targetId = "",
): Promise<ShareVM[]> {
  const tenantId = await resolveTenantId(tenantSlug);
  const resp = await shareClient.listShares({ tenantId, targetId });
  // touch the response schema so toJson coverage records the wrapper too.
  toJson(ListSharesResponseSchema, resp);
  return resp.shares.map(shareToVM);
}

export async function getShare(
  tenantSlug: string,
  id: string,
): Promise<ShareVM | undefined> {
  const tenantId = await resolveTenantId(tenantSlug);
  const resp = await shareClient.getShare({ tenantId, id });
  toJson(GetShareResponseSchema, resp);
  return resp.share ? shareToVM(resp.share) : undefined;
}

export async function createShare(
  tenantSlug: string,
  targetId: string,
  opts: { kind?: "test_run" | "suite_run"; ttlSec?: number } = {},
): Promise<ShareVM | undefined> {
  const tenantId = await resolveTenantId(tenantSlug);
  const resp = await shareClient.createShare({
    tenantId,
    target: {
      kind:
        opts.kind === "suite_run"
          ? ShareRecord_Target_Kind.SUITE_RUN
          : ShareRecord_Target_Kind.TEST_RUN,
      id: targetId,
    },
    ttl:
      opts.ttlSec === undefined
        ? undefined
        : { seconds: BigInt(Math.floor(opts.ttlSec)), nanos: 0 },
  });
  toJson(CreateShareResponseSchema, resp);
  return resp.share ? shareToVM(resp.share) : undefined;
}

export async function revokeShare(tenantSlug: string, id: string): Promise<void> {
  const tenantId = await resolveTenantId(tenantSlug);
  await shareClient.revokeShare({ tenantId, id });
}

export async function deleteShare(tenantSlug: string, id: string): Promise<void> {
  const tenantId = await resolveTenantId(tenantSlug);
  await shareClient.deleteShare({ tenantId, id });
}

export async function setShareExpiry(
  tenantSlug: string,
  id: string,
  ttlSec: number,
): Promise<void> {
  const tenantId = await resolveTenantId(tenantSlug);
  await shareClient.setShareExpiry({
    tenantId,
    id,
    ttl: { seconds: BigInt(Math.floor(ttlSec)), nanos: 0 },
  });
}

// --- public side ------------------------------------------------------------

/** The public read-only snapshot for a share token. */
export interface SharedRunVM {
  capturedAt?: string;
  kind: "test_run" | "suite_run" | "";
  name: string;
  status: RunStatus;
  dbKind: string;
  dbName: string;
  workloadName: string;
  stroppyVersion: string;
  provider: string;
  topologyLabel: string;
  nodeCount: number;
  startedAt?: string;
  finishedAt?: string;
  progressPct: number;
  metrics: MetricVM[];
  /** Safe launch knobs, so a shared result is reproducible. Absent on old
   *  snapshots captured before they were projected. */
  workloadSegments: SharedSegmentVM[];
  database?: SharedDatabaseVM;
  /** Per-VM hardware the run ran on (typed sizing only, no cloud creds). */
  machines: SharedMachineVM[];
}

/** One provisioned VM's hardware. */
export interface SharedMachineVM {
  nodeId: string;
  cores?: number;
  memoryGb?: number;
  bootDiskGb?: number;
  bootDiskType: string;
  platform: string;
  zone: string;
  secondaryDisks: Array<{ sizeGb?: number; type: string }>;
}

/** One workload segment's public launch knobs (no env / sql / extra args). */
export interface SharedSegmentVM {
  name: string;
  script: string;
  vus?: number;
  duration?: string;
  iterations?: number;
  poolSize?: number;
  scaleFactor?: number;
  insertMethod?: string;
  bulkSize?: number;
  steps: string[];
  noSteps: string[];
  quiet: boolean;
  noThresholds: boolean;
}

/** Database sizing + typed tuning (no free-form engine option maps). */
export interface SharedDatabaseVM {
  version: string;
  settings: Array<{ key: string; value: string }>;
}

const num = (v: number | "NaN" | "Infinity" | "-Infinity" | undefined): number =>
  typeof v === "number" ? v : 0;

type RawMetric = {
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
};

function metricToVM(m: RawMetric): MetricVM {
  return {
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
  };
}

/**
 * What the share page needs to embed Grafana: the run the token exposes and the
 * window to pin the dashboards to.
 */
export interface SharedSessionVM {
  runId: string;
  /** unix millis; 0/absent when unknown */
  from?: number;
  /** unix millis; absent while the run is still going */
  to?: number;
}

/**
 * Exchange the share token for a scoped session.
 *
 * The gateway answers with an HttpOnly `stroppy_share` cookie. Grafana's public
 * organisation forwards that cookie to its only datasource — the gateway's
 * /public/metrics proxy — which pins every query to this one run. The panels are
 * therefore live and interactive without the viewer being able to reach any
 * other run's series: the token, not the dashboard URL, decides what is visible.
 *
 * A revoked, expired or unknown token 404s exactly like a nonexistent one.
 */
export async function startSharedSession(token: string): Promise<SharedSessionVM> {
  const resp = await fetch(`/public/share/${encodeURIComponent(token)}/session`, {
    credentials: "include",
  });
  if (!resp.ok) {
    throw new Error("share session unavailable");
  }
  return (await resp.json()) as SharedSessionVM;
}

export async function getSharedRun(token: string): Promise<SharedRunVM | undefined> {
  const resp = await publicShareClient.getSharedRun({ token });
  const j = toJson(GetSharedRunResponseSchema, resp) as {
    snapshot?: {
      capturedAt?: string;
      testRun?: {
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
        finishedAt?: string;
        progressPct?: number;
        metrics?: { metrics?: RawMetric[] };
        workloadSegments?: Array<{
          name?: string;
          script?: string;
          vus?: number;
          duration?: string;
          iterations?: number;
          poolSize?: number;
          scaleFactor?: number;
          insertMethod?: string;
          bulkSize?: number;
          steps?: string[];
          noSteps?: string[];
          quiet?: boolean;
          noThresholds?: boolean;
        }>;
        database?: { version?: string; settings?: Array<{ key?: string; value?: string }> };
        machines?: Array<{
          nodeId?: string;
          cores?: number;
          memoryGb?: string | number;
          bootDiskGb?: string | number;
          bootDiskType?: string;
          platform?: string;
          zone?: string;
          secondaryDisks?: Array<{ sizeGb?: number; type?: string }>;
        }>;
      };
      suiteRun?: { name?: string };
    };
  };
  const snap = j.snapshot;
  if (!snap) return undefined;
  const tr = snap.testRun;
  if (tr) {
    return {
      capturedAt: snap.capturedAt,
      kind: "test_run",
      name: tr.name ?? "",
      status: statusToVM(tr.status),
      dbKind: dbKindLabelFromJson(tr.dbKind),
      dbName: tr.dbName ?? "",
      workloadName: tr.workloadName ?? "",
      stroppyVersion: tr.stroppyVersion ?? "",
      provider: providerLabelFromJson(tr.provider),
      topologyLabel: tr.topologyLabel ?? "",
      nodeCount: tr.nodeCount ?? 0,
      startedAt: tr.startedAt,
      finishedAt: tr.finishedAt,
      progressPct: tr.progressPct ?? 0,
      metrics: (tr.metrics?.metrics ?? []).map(metricToVM),
      workloadSegments: (tr.workloadSegments ?? []).map((s) => ({
        name: s.name ?? "",
        script: s.script ?? "",
        vus: s.vus,
        duration: s.duration,
        iterations: s.iterations,
        poolSize: s.poolSize,
        scaleFactor: s.scaleFactor,
        insertMethod: s.insertMethod,
        bulkSize: s.bulkSize,
        steps: s.steps ?? [],
        noSteps: s.noSteps ?? [],
        quiet: s.quiet ?? false,
        noThresholds: s.noThresholds ?? false,
      })),
      database: tr.database
        ? {
            version: tr.database.version ?? "",
            settings: (tr.database.settings ?? [])
              .filter((s) => !!s.key)
              .map((s) => ({ key: s.key as string, value: s.value ?? "" })),
          }
        : undefined,
      machines: (tr.machines ?? []).map((m) => ({
        nodeId: m.nodeId ?? "",
        cores: m.cores,
        memoryGb: m.memoryGb !== undefined ? Number(m.memoryGb) : undefined,
        bootDiskGb: m.bootDiskGb !== undefined ? Number(m.bootDiskGb) : undefined,
        bootDiskType: m.bootDiskType ?? "",
        platform: m.platform ?? "",
        zone: m.zone ?? "",
        secondaryDisks: (m.secondaryDisks ?? []).map((d) => ({ sizeGb: d.sizeGb, type: d.type ?? "" })),
      })),
    };
  }
  return {
    capturedAt: snap.capturedAt,
    kind: snap.suiteRun ? "suite_run" : "",
    name: snap.suiteRun?.name ?? "",
    status: "pending",
    dbKind: "",
    dbName: "",
    workloadName: "",
    stroppyVersion: "",
    provider: "",
    topologyLabel: "",
    nodeCount: 0,
    progressPct: 0,
    metrics: [],
    workloadSegments: [],
    machines: [],
  };
}
