// Suite-wizard data surface for /t/:slug/suites/new.
//
// This is the SUITE analogue of services/wizard.ts (the test wizard). It is the
// contract the Suite Wizard page talks to: the page depends ONLY on this
// interface, never on the wire format. The real implementation calls the
// connectrpc SuiteWizardService (cloud.v1.api.SuiteWizardService, see
// src/lib/proto/cloud/v1/api/suite_wizard_pb.ts) and maps the proto
// cloud.v1.models.SuiteWizardDraftRecord onto the flat view-model types here.
//
// A suite is a db x workload MATRIX: each draft holds N cells, one suite-wide
// provider, an optional cron schedule, a max_parallel concurrency and the two
// rating-default flags. Each cell carries a source (preset_pair | test_preset |
// inline_test) which the server RESOLVES into a database + workload + topology +
// infrastructure preview + per-cell readiness/errors.
//
// FULL PROTOBUF CORRESPONDENCE — every VM field maps to a real proto field and
// each edit becomes a typed PatchSuiteWizardRequest sub-message:
//
//   What the user edits                  PatchSuiteWizardRequest field
//   ----------------------------------   --------------------------------------
//   provider                             .provider (deployment.Provider)
//   add/edit/remove a cell               .cells[] (SuiteWizardCellPatch, merged
//                                          by cell_id; empty cell_id = create)
//   replace the whole matrix             .cells[] + .replace_cells = true
//   max_parallel                         .max_parallel
//   schedule (enabled/cron/timezone)     .schedule (domain.Schedule)
//   rating defaults                      .default_in_tenant_rating / _global_
//
// The server fills (on every Patch) per-cell database/workload/topology_spec/
// infrastructure_plan/render_preview/compatible/ready/errors plus draft-level
// errors + ready. FinishSuiteWizard persists a SuiteRecord (and, when start=true,
// launches a SuiteRun) — that is the createSuite path for the wizard.

import { create, toJson } from "@bufbuild/protobuf";
import { Provider } from "@/lib/proto/cloud/v1/deployment/provider_pb";
import {
  SuiteWizardDraftRecordSchema,
  type SuiteWizardDraftRecord,
} from "@/lib/proto/cloud/v1/models/suite_wizard_pb";
import {
  SuiteWizardCellPatchSchema,
  type SuiteWizardCellPatch,
} from "@/lib/proto/cloud/v1/api/suite_wizard_pb";
import {
  SuiteCell_PresetPairSchema,
  ScheduleSchema,
} from "@/lib/proto/cloud/v1/domain/suite_pb";
import { TestSchema } from "@/lib/proto/cloud/v1/domain/test_pb";
import { suiteWizardClient } from "@/services/client";
import { resolveTenantId } from "@/services/tenant";
import { dbKindLabelFromJson, dbKindProto } from "@/services/enums";
import type { DbKind } from "@/services/runs";
import type { SuiteProviderKind, SuiteCellSource, SuiteCellInput } from "@/services/suites";
import {
  machineVMToProto,
  Yandex_Settings_PlatformId,
  Yandex_Settings_Zone,
  type InfrastructurePlanVM,
  type MachineSpecVM,
  type ProviderSettingsVM,
} from "@/services/wizard";
import {
  normalizeYandexBootDiskType,
  normalizeYandexInternalIp,
  normalizeYandexNetworkAcceleration,
} from "@/lib/machine-constraints";

export { Provider };

// --- View models -------------------------------------------------------------

/** One schemapb.FieldError, flattened (mirrors wizard.ts DraftErrorVM). */
export interface SuiteDraftErrorVM {
  field: string;
  message: string;
  severity: "error" | "warning" | "info";
  code: string;
}

/**
 * One SuiteWizardDraftRecord.Cell, flattened. The `spec` half is the editable
 * cell intent (source/enabled/name); the rest is server-derived: the resolved
 * matrix axes (dbKind/workload), the node count from the topology, and the
 * per-cell compatible/ready/errors.
 */
export interface SuiteCellVM {
  /** spec.id — stable within the suite; "" for a not-yet-persisted local cell. */
  id: string;
  /** spec.name — display label; "" lets the server derive one. */
  name: string;
  /** spec.enabled */
  enabled: boolean;
  /** which arm of spec.source is set. */
  source: SuiteCellSource;
  /** preset_pair arm. */
  dbPresetId: string;
  workloadPresetId: string;
  /** test_preset_id arm. */
  testPresetId: string;
  /** inline_test arm — the authored matrix db axis (inline_test.database.kind). */
  inlineDbKind: DbKind;
  /**
   * matrix ROW axis — resolved by the server from the cell's database
   * (cells[].database.kind). "" until resolvable.
   */
  dbKind: DbKind;
  /**
   * matrix COLUMN axis — a short label resolved from the cell's workload
   * (script). "" until resolvable.
   */
  workload: string;
  /** server-derived: db/workload compatibility passed. */
  compatible: boolean;
  /** server-derived: this enabled cell can be baked into a TestRun. */
  ready: boolean;
  /** number of nodes in the cell's derived topology (machine count). */
  nodeCount: number;
  /** server-derived provider machine preview for this cell. */
  infrastructurePlan: InfrastructurePlanVM;
  /** number of explicit machine overrides currently stored on spec. */
  machineOverrideCount: number;
  /** per-cell validation/capacity/render errors. */
  errors: SuiteDraftErrorVM[];
}

/**
 * SuiteWizardDraftVM is the flat projection of a SuiteWizardDraftRecord the
 * Suite Wizard page renders. provider/cells/schedule/max_parallel/rating flags
 * are the user-edited halves; errors/ready (+ each cell's derived halves) are
 * the server-recomputed halves.
 */
export interface SuiteWizardDraftVM {
  id: string;
  name: string;
  /** spec.provider mirrored to a deployment.Provider enum. */
  provider: Provider;
  cells: SuiteCellVM[];
  /** max_parallel (0 = unlimited). */
  maxParallel: number;
  /** schedule.enabled */
  scheduleEnabled: boolean;
  /** schedule.cron */
  cron: string;
  /** schedule.timezone */
  timezone: string;
  /** default_in_tenant_rating (optional). */
  defaultInTenantRating?: boolean;
  /** default_in_global_rating (optional). */
  defaultInGlobalRating?: boolean;
  /** draft-level validation errors, recomputed each Patch. */
  errors: SuiteDraftErrorVM[];
  /** true when the suite validates and FinishSuiteWizard is allowed. */
  ready: boolean;
  /** suite_id when seeded from an existing suite. */
  suiteId: string;
  /** entity.timings.updated_at (ISO) — drives the resume list ordering. */
  updatedAt: string;
}

/** A compact draft row for the resume ("continue where you left off") list. */
export interface SuiteDraftSummaryVM {
  id: string;
  name: string;
  cellCount: number;
  provider: SuiteProviderKind;
  ready: boolean;
  updatedAt: string;
}

/** A single cell edit (maps to one SuiteWizardCellPatch). */
export interface SuiteCellPatchInput {
  /** "" creates a new cell; an id merges into / removes that cell. */
  cellId: string;
  /** delete the selected cell (other fields ignored). */
  remove?: boolean;
  enabled?: boolean;
  name?: string;
  /** the cell's source intent (reuses the Suites SuiteCellInput shape). */
  cell?: SuiteCellInput;
  /** explicit provider machine settings for this cell. */
  machineOverrides?: InfrastructurePlanVM["machines"];
  /** Provider-level values that own Yandex placement/network toggles for this cell. */
  machineOverrideSettings?: ProviderSettingsVM;
}

/** What a Patch carries — the typed sub-message(s) a step changed. */
export interface SuitePatchInput {
  provider?: Provider;
  /** cell edits merged by cell_id (or the full matrix when replaceCells=true). */
  cells?: SuiteCellPatchInput[];
  replaceCells?: boolean;
  maxParallel?: number;
  schedule?: { enabled: boolean; cron: string; timezone: string };
  defaultInTenantRating?: boolean;
  defaultInGlobalRating?: boolean;
}

/** FinishSuiteWizard inputs (persist + optionally launch + rating overrides). */
export interface SuiteFinishInput {
  start: boolean;
  suiteName: string;
  inTenantRating?: boolean;
  inGlobalRating?: boolean;
}

/** FinishSuiteWizard result, flattened. */
export interface SuiteFinishResultVM {
  /** id of the persisted suite. */
  suiteId: string;
  /** id of the launched suite run when start=true; "" otherwise. */
  suiteRunId: string;
}

/**
 * SuiteWizardProvider abstracts the StartSuiteWizard / Patch-loop / Finish flow
 * so the page never imports the wire format. Each method maps to one
 * SuiteWizardService RPC; methods are keyed by tenant SLUG (matching wizard.ts).
 */
export interface SuiteWizardProvider {
  /** StartSuiteWizard -> a fresh draft (optionally seeded from a suite). */
  start(tenantSlug: string, name: string, suiteId?: string): Promise<SuiteWizardDraftVM>;
  /** GetSuiteWizardDraft -> a single draft by id (resume / Back). */
  get(tenantSlug: string, draftId: string): Promise<SuiteWizardDraftVM>;
  /** ListSuiteWizardDrafts -> the caller's recent drafts (resume list). */
  list(tenantSlug: string): Promise<SuiteDraftSummaryVM[]>;
  /** PatchSuiteWizard -> the recomputed draft. */
  patch(tenantSlug: string, draftId: string, input: SuitePatchInput): Promise<SuiteWizardDraftVM>;
  /** DeleteSuiteWizardDraft -> drop a draft. */
  remove(tenantSlug: string, draftId: string): Promise<void>;
  /** FinishSuiteWizard -> the persisted suite, optionally launched. */
  finish(tenantSlug: string, draftId: string, input: SuiteFinishInput): Promise<SuiteFinishResultVM>;
}

// --- proto -> VM --------------------------------------------------------------

function providerFromJson(s: string | undefined): Provider {
  return s === "PROVIDER_YANDEX"
    ? Provider.YANDEX
    : s === "PROVIDER_DOCKER"
      ? Provider.DOCKER
      : Provider.UNSPECIFIED;
}

function providerLabel(p: Provider): SuiteProviderKind {
  return p === Provider.DOCKER ? "docker" : p === Provider.YANDEX ? "yandex" : "";
}

function errorSeverity(s: string | undefined): SuiteDraftErrorVM["severity"] {
  switch (s) {
    case "WARNING":
      return "warning";
    case "SEVERITY_UNSPECIFIED":
      return "info";
    default:
      return "error";
  }
}

function mapErrors(
  errs: { field?: string; message?: string; severity?: string; code?: string }[] | undefined,
): SuiteDraftErrorVM[] {
  return (errs ?? []).map((e) => ({
    field: e.field ?? "",
    message: e.message ?? "",
    severity: errorSeverity(e.severity),
    code: e.code ?? "",
  }));
}

// A short, human label for the matrix COLUMN axis derived from the resolved
// workload (domain.Workload carries no display name — use the script path).
function workloadLabel(w: { script?: string } | undefined): string {
  const s = w?.script ?? "";
  return s ? s.split("/")[0] : "";
}

// The JSON projection of a SuiteWizardDraftRecord; we pluck only what the VM
// exposes. The matrix axes come from each cell's RESOLVED database/workload.
type DraftJson = {
  entity?: { id?: string; name?: string; timings?: { updatedAt?: string } };
  provider?: string;
  maxParallel?: number;
  schedule?: { enabled?: boolean; cron?: string; timezone?: string };
  defaultInTenantRating?: boolean;
  defaultInGlobalRating?: boolean;
  suiteId?: string;
  ready?: boolean;
  errors?: { field?: string; message?: string; severity?: string; code?: string }[];
  cells?: {
    spec?: {
      id?: string;
      name?: string;
      enabled?: boolean;
      presetPair?: { dbPresetId?: string; workloadPresetId?: string };
      testPresetId?: string;
      inlineTest?: { database?: { kind?: string } };
      machineOverrides?: unknown[];
    };
    database?: { kind?: string };
    workload?: { script?: string };
    topologySpec?: TopologyJson;
    infrastructurePlan?: InfrastructurePlanJson;
    compatible?: boolean;
    ready?: boolean;
    errors?: { field?: string; message?: string; severity?: string; code?: string }[];
  }[];
};

type TopologyJson = {
  components?: { id?: string; kind?: string; engine?: string; role?: string }[];
  nodes?: { id?: string; componentIds?: string[]; labels?: Record<string, string> }[];
};

type InfrastructurePlanJson = {
  provider?: string;
  settings?: {
    docker?: Record<string, never>;
    yandex?: {
      cloudId?: string;
      folderId?: string;
      zone?: number;
      networkName?: string;
      subnetCidr?: string;
      platformId?: number;
      imageId?: string;
      assignPublicIp?: boolean;
      softwareAcceleratedNetwork?: boolean;
      sshUser?: string;
    };
  };
  machines?: {
    nodeId?: string;
    docker?: { image?: string; resources?: { cpuCores?: number; memoryMb?: string } };
      yandex?: {
        cores?: number;
        memoryGb?: string;
        bootDiskGb?: string;
        bootDiskType?: string;
        zone?: string;
        internalIp?: string;
        publicIp?: boolean;
        networkAcceleration?: string;
      };
  }[];
};

function infrastructurePlanToVM(
  ip: InfrastructurePlanJson | undefined,
  topology: TopologyJson | undefined,
  fallbackProvider: Provider,
): InfrastructurePlanVM {
  const topologyComponents = (topology?.components ?? []).map((c) => {
    const id = c.id ?? "";
    const node = (topology?.nodes ?? []).find((n) => (n.componentIds ?? []).includes(id));
    return {
      id,
      engine: c.engine ?? "",
      role: c.role ?? "",
      nodeId: node?.id ?? "",
    };
  });
  const nodeRoleEngine = (nodeId: string): { role: string; engine: string } => {
    const node = (topology?.nodes ?? []).find((n) => n.id === nodeId);
    for (const cid of node?.componentIds ?? []) {
      const comp = topologyComponents.find((x) => x.id === cid);
      if (comp) return { role: comp.role, engine: comp.engine };
    }
    return { role: "", engine: "" };
  };

  let settings: ProviderSettingsVM = { case: undefined };
  if (ip?.settings?.yandex) {
    const y = ip.settings.yandex;
    settings = {
      case: "yandex",
      yandex: {
        cloudId: y.cloudId ?? "",
        folderId: y.folderId ?? "",
        zone: y.zone ?? Yandex_Settings_Zone.UNSPECIFIED,
        networkName: y.networkName ?? "",
        subnetCidr: y.subnetCidr ?? "",
        platformId: y.platformId ?? Yandex_Settings_PlatformId.UNSPECIFIED,
        imageId: y.imageId ?? "",
        assignPublicIp: y.assignPublicIp ?? false,
        softwareAcceleratedNetwork: y.softwareAcceleratedNetwork ?? false,
        sshUser: y.sshUser ?? "",
      },
    };
  } else if (ip?.settings?.docker) {
    settings = { case: "docker", docker: { networkName: "" } };
  }

  const machines = (ip?.machines ?? []).map((m) => {
    const nodeId = m.nodeId ?? "";
    const { role, engine } = nodeRoleEngine(nodeId);
    let spec: MachineSpecVM = { case: undefined };
    if (m.yandex) {
      spec = {
        case: "yandex",
        yandex: {
          cores: m.yandex.cores ?? 0,
          memoryGb: Number(m.yandex.memoryGb ?? 0),
          bootDiskGb: Number(m.yandex.bootDiskGb ?? 0),
          bootDiskType: normalizeYandexBootDiskType(m.yandex.bootDiskType),
          zone: m.yandex.zone ?? "",
          internalIp: normalizeYandexInternalIp(m.yandex.internalIp),
          publicIp: m.yandex.publicIp ?? false,
          networkAcceleration: normalizeYandexNetworkAcceleration(m.yandex.networkAcceleration),
        },
      };
    } else if (m.docker) {
      spec = {
        case: "docker",
        docker: {
          image: m.docker.image ?? "",
          cpuCores: m.docker.resources?.cpuCores ?? 0,
          memoryMb: Number(m.docker.resources?.memoryMb ?? 0),
        },
      };
    }
    return { nodeId, role, engine, spec };
  });

  return {
    provider: providerFromJson(ip?.provider) || fallbackProvider,
    settings,
    machines,
  };
}

function mapCell(c: NonNullable<DraftJson["cells"]>[number], provider: Provider): SuiteCellVM {
  const spec = c.spec ?? {};
  const source: SuiteCellSource = spec.presetPair
    ? "presetPair"
    : spec.testPresetId !== undefined
      ? "testPreset"
      : "inline";
  // The matrix ROW axis comes from the resolved database (server-side), falling
  // back to the inline authored kind before the server resolves anything.
  const dbKind =
    dbKindLabelFromJson(c.database?.kind) ||
    dbKindLabelFromJson(spec.inlineTest?.database?.kind);
  return {
    id: spec.id ?? "",
    name: spec.name ?? "",
    enabled: spec.enabled ?? false,
    source,
    dbPresetId: spec.presetPair?.dbPresetId ?? "",
    workloadPresetId: spec.presetPair?.workloadPresetId ?? "",
    testPresetId: spec.testPresetId ?? "",
    inlineDbKind: dbKindLabelFromJson(spec.inlineTest?.database?.kind),
    dbKind,
    workload: workloadLabel(c.workload),
    compatible: c.compatible ?? false,
    ready: c.ready ?? false,
    nodeCount: c.topologySpec?.nodes?.length ?? 0,
    infrastructurePlan: infrastructurePlanToVM(c.infrastructurePlan, c.topologySpec, provider),
    machineOverrideCount: spec.machineOverrides?.length ?? 0,
    errors: mapErrors(c.errors),
  };
}

function draftToVM(draft: SuiteWizardDraftRecord | undefined): SuiteWizardDraftVM {
  const j = (draft ? toJson(SuiteWizardDraftRecordSchema, draft) : {}) as DraftJson;
  return {
    id: j.entity?.id ?? "",
    name: j.entity?.name ?? "",
    provider: providerFromJson(j.provider),
    cells: (j.cells ?? []).map((c) => mapCell(c, providerFromJson(j.provider))),
    maxParallel: j.maxParallel ?? 0,
    scheduleEnabled: j.schedule?.enabled ?? false,
    cron: j.schedule?.cron ?? "",
    timezone: j.schedule?.timezone ?? "",
    defaultInTenantRating: j.defaultInTenantRating,
    defaultInGlobalRating: j.defaultInGlobalRating,
    errors: mapErrors(j.errors),
    ready: j.ready ?? false,
    suiteId: j.suiteId ?? "",
    updatedAt: j.entity?.timings?.updatedAt ?? "",
  };
}

function draftToSummaryVM(draft: SuiteWizardDraftRecord): SuiteDraftSummaryVM {
  const vm = draftToVM(draft);
  return {
    id: vm.id,
    name: vm.name,
    cellCount: vm.cells.length,
    provider: providerLabel(vm.provider),
    ready: vm.ready,
    updatedAt: vm.updatedAt,
  };
}

// --- VM -> proto --------------------------------------------------------------

// Build the SuiteCell.source oneof from the authored cell intent (mirrors
// suites.ts cellInputToSource but emits the SuiteWizardCellPatch source arm).
function cellSource(cell: SuiteCellInput): SuiteWizardCellPatch["source"] {
  if (cell.source === "presetPair") {
    return {
      case: "presetPair",
      value: create(SuiteCell_PresetPairSchema, {
        dbPresetId: cell.dbPresetId ?? "",
        workloadPresetId: cell.workloadPresetId ?? "",
      }),
    };
  }
  if (cell.source === "testPreset") {
    return { case: "testPresetId", value: cell.testPresetId ?? "" };
  }
  return {
    case: "inlineTest",
    value: create(TestSchema, {
      database: { kind: dbKindProto(cell.inlineDbKind ?? "") },
    }),
  };
}

function cellPatch(input: SuiteCellPatchInput): SuiteWizardCellPatch {
  return create(SuiteWizardCellPatchSchema, {
    cellId: input.cellId,
    remove: input.remove ?? false,
    enabled: input.enabled,
    name: input.name,
    source: input.cell ? cellSource(input.cell) : { case: undefined },
    machineOverrides: input.machineOverrides?.map((machine) =>
      machineVMToProto(machine, input.machineOverrideSettings),
    ),
  });
}

// --- Real backend provider ----------------------------------------------------

const realSuiteWizardProvider: SuiteWizardProvider = {
  async start(tenantSlug, name, suiteId) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { draft } = await suiteWizardClient.startSuiteWizard({
      tenantId,
      name,
      suiteId: suiteId ?? "",
    });
    return draftToVM(draft);
  },

  async get(tenantSlug, draftId) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { draft } = await suiteWizardClient.getSuiteWizardDraft({ tenantId, draftId });
    return draftToVM(draft);
  },

  async list(tenantSlug) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { drafts } = await suiteWizardClient.listSuiteWizardDrafts({
      tenantId,
      page: { size: 20 },
    });
    return drafts.map(draftToSummaryVM);
  },

  async patch(tenantSlug, draftId, input) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { draft } = await suiteWizardClient.patchSuiteWizard({
      tenantId,
      draftId,
      provider: input.provider ?? Provider.UNSPECIFIED,
      cells: (input.cells ?? []).map(cellPatch),
      replaceCells: input.replaceCells ?? false,
      maxParallel: input.maxParallel,
      schedule: input.schedule
        ? create(ScheduleSchema, {
            enabled: input.schedule.enabled,
            cron: input.schedule.cron,
            timezone: input.schedule.timezone,
          })
        : undefined,
      defaultInTenantRating: input.defaultInTenantRating,
      defaultInGlobalRating: input.defaultInGlobalRating,
    });
    return draftToVM(draft);
  },

  async remove(tenantSlug, draftId) {
    const tenantId = await resolveTenantId(tenantSlug);
    await suiteWizardClient.deleteSuiteWizardDraft({ tenantId, draftId });
  },

  async finish(tenantSlug, draftId, input) {
    const tenantId = await resolveTenantId(tenantSlug);
    const resp = await suiteWizardClient.finishSuiteWizard({
      tenantId,
      draftId,
      start: input.start,
      suiteName: input.suiteName,
      inTenantRating: input.inTenantRating,
      inGlobalRating: input.inGlobalRating,
    });
    return {
      suiteId: resp.suite?.entity?.id ?? "",
      suiteRunId: resp.suiteRun?.entity?.id ?? "",
    };
  },
};

// --- Provider injection -------------------------------------------------------

let active: SuiteWizardProvider = realSuiteWizardProvider;

export function setSuiteWizardProvider(provider: SuiteWizardProvider): void {
  active = provider;
}

export function getSuiteWizardProvider(): SuiteWizardProvider {
  return active;
}

// Re-export the provider label converter for the page's provider header.
export { providerLabel as suiteProviderLabel };
