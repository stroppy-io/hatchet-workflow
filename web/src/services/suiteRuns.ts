// Suite-runs data surface for the Suite detail page.
//
// Mirrors the runs.ts / suites.ts provider pattern: the page depends ONLY on
// this interface. The real implementation wires the connectrpc
// `suiteRunClient` (cloud.v1.api.SuiteRunService, see
// src/lib/proto/cloud/v1/api/suite_run_pb.ts) and maps cloud.v1.models.
// SuiteRunRecord onto the flat view-model here.
//
// Scope — built around the slice of cloud.v1.api.ListSuiteRunsRequest the page
// uses: suite_id (scope to one suite) + page{size,token}. Cancel/Delete map to
// SuiteRunService.CancelSuiteRun / DeleteSuiteRun.

import type { RunStatus } from "@/services/dashboard";
import type { DbKind, DeployProvider } from "@/services/runs";
import { suiteRunClient } from "@/services/client";
import { resolveTenantId } from "@/services/tenant";

export type { RunStatus };

/** How the suite run was triggered (cloud.v1.common.Trigger). */
export type SuiteRunTrigger = "" | "manual" | "cron" | "api";

/**
 * One child run of a suite run (cloud.v1.models.SuiteRunRecord.ChildRun). Each
 * child is a first-class TestRunRecord reachable at /runs/:testRunId.
 */
export interface SuiteRunChildVM {
  /** children[].suite_cell_id -> domain.SuiteCell.id */
  suiteCellId: string;
  /** children[].test_run_id (entity id of the child TestRunRecord). */
  testRunId: string;
  /** children[].name */
  name: string;
  /** children[].status */
  status: RunStatus;
}

/**
 * A suite run, flattened from cloud.v1.models.SuiteRunRecord (entity + status +
 * trigger + max_parallel + children + summary).
 */
export interface SuiteRunVM {
  /** entity.id */
  id: string;
  /** entity.name */
  name: string;
  /** suite_id */
  suiteId: string;
  /** record.status */
  status: RunStatus;
  /** record.trigger, lower-cased ("" when unspecified). */
  trigger: SuiteRunTrigger;
  /** record.max_parallel (0 = unlimited). */
  maxParallel: number;
  /** summary.suite_name */
  suiteName: string;
  /** summary.provider, lower-cased ("" when unspecified). */
  provider: DeployProvider;
  /** summary.db_kinds, lower-cased. */
  dbKinds: DbKind[];
  /** summary.total */
  total: number;
  /** summary.completed */
  completed: number;
  /** summary.failed */
  failed: number;
  /** summary.running */
  running: number;
  /** summary.pending */
  pending: number;
  /** summary.progress_pct, 0..100. */
  progressPct: number;
  /** summary.started_at (ISO); absent until started. */
  startedAt?: string;
  /** summary.finished_at (ISO); absent while running. */
  finishedAt?: string;
  /** summary.duration in seconds (from proto Duration); absent when unset. */
  durationSec?: number;
  /** record.children */
  children: SuiteRunChildVM[];
}

export interface SuiteRunsQuery {
  /** suite_id scope (required for the detail page). */
  suiteId: string;
  /** page.size */
  pageSize?: number;
  /** page.token */
  pageToken?: string;
}

export interface SuiteRunsPage {
  runs: SuiteRunVM[];
  nextPageToken: string;
}

export function isSuiteRunActive(status: RunStatus): boolean {
  return status === "running" || status === "pending" || status === "cancelling";
}

export interface SuiteRunsProvider {
  /** ListSuiteRuns scoped to one suite. */
  listSuiteRuns(
    tenantSlug: string,
    query: SuiteRunsQuery,
  ): Promise<SuiteRunsPage>;
  /** GetSuiteRun — single run by id; null when not found. */
  getSuiteRun(tenantSlug: string, id: string): Promise<SuiteRunVM | null>;
  /** CancelSuiteRun — flips a running/pending run to cancelling/cancelled. */
  cancelSuiteRun(tenantSlug: string, id: string): Promise<void>;
  /** DeleteSuiteRun — soft-deletes a (finished) suite run. */
  deleteSuiteRun(tenantSlug: string, id: string): Promise<void>;
}

const NOT_WIRED =
  "real SuiteRunsProvider not wired yet - run with VITE_MOCK=1 to preview suite runs";

const realSuiteRunsProvider: SuiteRunsProvider = {
  async listSuiteRuns() {
    // await suiteRunClient.listSuiteRuns({
    //   tenantId, suiteId, page: { size, token },
    // });
    throw new Error(NOT_WIRED);
  },
  async getSuiteRun() {
    // await suiteRunClient.getSuiteRun({ tenantId, id });
    throw new Error(NOT_WIRED);
  },
  async cancelSuiteRun() {
    // await suiteRunClient.cancelSuiteRun({ tenantId, id });
    throw new Error(NOT_WIRED);
  },
  async deleteSuiteRun() {
    // await suiteRunClient.deleteSuiteRun({ tenantId, id });
    throw new Error(NOT_WIRED);
  },
};

// Touch the imports so the real wiring stays a thin client/tenant wrapper and
// lint does not flag them as unused until the methods above are implemented.
void suiteRunClient;
void resolveTenantId;

let active: SuiteRunsProvider = realSuiteRunsProvider;

export function setSuiteRunsProvider(provider: SuiteRunsProvider): void {
  active = provider;
}

export function getSuiteRunsProvider(): SuiteRunsProvider {
  return active;
}
