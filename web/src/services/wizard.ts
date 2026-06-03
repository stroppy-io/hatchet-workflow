// Test-wizard data surface for the rebuilt shell.
//
// This is the contract the New Run wizard page talks to. It mirrors the
// runs.ts / dashboard.ts provider pattern: the page depends ONLY on this
// interface, never on mock or backend specifics. The real implementation
// (realWizardProvider below) calls the connectrpc TestWizardService
// (cloud.v1.api.TestWizardService, see
// src/lib/proto/cloud/v1/api/test_wizard_pb.ts) and maps the proto
// TestWizardDraftRecord onto the flat view-model types here. The single mock
// gate in main.tsx injects a stateful mock via setWizardProvider() so the
// wizard is fully clickable standalone under VITE_MOCK=1.
//
// FULL PROTOBUF CORRESPONDENCE — every VM field below maps to a REAL proto
// field; the steps each build one typed sub-message and Patch returns the
// recomputed draft:
//
//   Step            edits (PatchTestWizardRequest field)            proto type
//   ------------    --------------------------------------------    ------------------------------
//   Infrastructure  .provider + .infrastructure_plan                Provider + deployment.InfrastructurePlan
//                     (ProviderSettings + per-node MachinePlan: Docker.Container | Yandex.Vm)
//   Database        .database                                       cloud.v1.domain.Database (typed per engine)
//   Workload        .workload (+ ProbeScript)                       cloud.v1.domain.Workload
//   Review/Render   .render_overrides (FileOverride set)            deployment.RenderOverrideSet
//                     (edit EDITABLE render artifacts → re-render)
//   Finish          FinishTestWizardRequest                         -> TestRun / TestRunRecord / TestPresetRecord
//
// The server fills (on every Patch) topology_spec + infrastructure_plan
// (settings + machines) + render_preview (artifacts + mutability + base_hash);
// the user SEES and EDITS the infrastructure plan and the EDITABLE artifacts.
//
// The view-model is intentionally flat (no proto Message instances) so the
// wizard UI stays decoupled from the wire format and the mock stays trivial.
// Provider methods are keyed by tenant SLUG (matching runs.ts); the real
// provider resolves slug -> tenant_id at call time exactly like runs.ts does.

import { Database_Kind } from "@/lib/proto/cloud/v1/domain/database_pb";
import {
  YdbParams_FaultTolerance,
  YdbParams_DiskType,
  YdbManagedParams_Type,
  YdbManagedParams_ComputeType,
} from "@/lib/proto/cloud/v1/domain/database_pb";
import { Workload_Protocol } from "@/lib/proto/cloud/v1/domain/workload_pb";
import { Provider } from "@/lib/proto/cloud/v1/deployment/provider_pb";
import {
  RenderArtifact_Kind,
  RenderArtifact_Origin,
  RenderArtifact_Mutability,
} from "@/lib/proto/cloud/v1/deployment/render_pb";
import {
  Yandex_Settings_Zone,
  Yandex_Settings_PlatformId,
} from "@/lib/proto/cloud/v1/deployment/yandex_pb";

// Re-export the proto enums the steps drive their selects from, so the page
// imports a single module. Keeping the proto enums (not string unions) makes
// the typed Patch payload trivially constructable by the real provider.
export {
  Database_Kind,
  Provider,
  Workload_Protocol,
  YdbParams_FaultTolerance,
  YdbParams_DiskType,
  YdbManagedParams_Type,
  YdbManagedParams_ComputeType,
  RenderArtifact_Kind,
  RenderArtifact_Origin,
  RenderArtifact_Mutability,
  Yandex_Settings_Zone,
  Yandex_Settings_PlatformId,
};

// --- Engine kinds the wizard can express ------------------------------------
//
// Every engine whose DatabaseParams are typed in the proto is selectable. The
// `params` shape per engine is the typed sub-message DatabaseParams.engine.
export type EngineKind =
  | "postgres"
  | "mysql"
  | "mariadb"
  | "picodata"
  | "ydb"
  | "ydbManaged"
  | "cockroach"
  | "external";

/** EngineKind -> Database_Kind enum (the wire value Patch sends). */
export const ENGINE_TO_KIND: Record<EngineKind, Database_Kind> = {
  postgres: Database_Kind.POSTGRES,
  mysql: Database_Kind.MYSQL,
  mariadb: Database_Kind.MARIADB,
  picodata: Database_Kind.PICODATA,
  ydb: Database_Kind.YDB,
  ydbManaged: Database_Kind.YDB_MANAGED,
  cockroach: Database_Kind.COCKROACH,
  external: Database_Kind.EXTERNAL,
};

/** Reverse map for rendering a recomputed draft back into form state. */
export const KIND_TO_ENGINE: Partial<Record<Database_Kind, EngineKind>> = {
  [Database_Kind.POSTGRES]: "postgres",
  [Database_Kind.MYSQL]: "mysql",
  [Database_Kind.MARIADB]: "mariadb",
  [Database_Kind.PICODATA]: "picodata",
  [Database_Kind.YDB]: "ydb",
  [Database_Kind.YDB_MANAGED]: "ydbManaged",
  [Database_Kind.COCKROACH]: "cockroach",
  [Database_Kind.EXTERNAL]: "external",
};

/** Display metadata per engine — the catalog the Database step renders. */
export interface EngineMeta {
  kind: EngineKind;
  label: string;
  /** brand hex used for the accent in the dark UI. */
  hex: string;
  /** short blurb on the picker card. */
  blurb: string;
  /** which engines have fully-typed params (vs sparse). */
  typed: boolean;
}

export const ENGINES: EngineMeta[] = [
  { kind: "postgres", label: "PostgreSQL", hex: "#5B93C4", blurb: "Streaming replicas, Patroni HA, HAProxy, PgBouncer.", typed: true },
  { kind: "mysql", label: "MySQL", hex: "#F2A94B", blurb: "Group replication or semi-sync, ProxySQL.", typed: true },
  { kind: "mariadb", label: "MariaDB", hex: "#6E91A8", blurb: "MySQL-compatible topology (group/semi-sync, ProxySQL).", typed: true },
  { kind: "picodata", label: "Picodata", hex: "#F06580", blurb: "Sharded in-memory cluster, replication factor, HAProxy.", typed: true },
  { kind: "ydb", label: "YDB", hex: "#7FB3E8", blurb: "Self-deployed storage + database nodes, erasure modes.", typed: true },
  { kind: "ydbManaged", label: "YDB Managed", hex: "#FF6666", blurb: "Cloud-managed serverless or dedicated YDB.", typed: true },
  { kind: "cockroach", label: "CockroachDB", hex: "#3FAEAE", blurb: "Homogeneous node cluster with cluster settings.", typed: true },
  { kind: "external", label: "External", hex: "#A1A1AA", blurb: "Connect to an existing DSN — no deploy/teardown.", typed: true },
];

// --- Typed per-engine params (mirror DatabaseParams.engine sub-messages) -----
//
// Each interface is the exact field set of the corresponding proto params
// message. NO invented fields. Option maps (postgresql.conf, my.cnf, ...) are
// carried as plain records so the config-editor can edit them.

export interface PostgresParamsVM {
  replicas: number;
  haproxy: number;
  pgbouncer: boolean;
  patroni: boolean;
  etcd: boolean;
  syncReplicas: number;
  masterOptions: Record<string, string>;
  replicaOptions: Record<string, string>;
  haproxyOptions: Record<string, string>;
  pgbouncerOptions: Record<string, string>;
  patroniOptions: Record<string, string>;
  etcdOptions: Record<string, string>;
}

/** Shared by mysql + mariadb (proto: MySqlParams is reused for both). */
export interface MySqlParamsVM {
  replicas: number;
  proxysql: number;
  groupReplication: boolean;
  semiSync: boolean;
  primaryOptions: Record<string, string>;
  replicaOptions: Record<string, string>;
  proxysqlOptions: Record<string, string>;
}

export interface PicodataParamsVM {
  instances: number;
  haproxy: number;
  replicationFactor: number;
  shards: number;
  instanceOptions: Record<string, string>;
  haproxyOptions: Record<string, string>;
}

export interface YdbParamsVM {
  storageNodes: number;
  databaseNodes: number;
  haproxy: number;
  pdisksPerStorageNode: number;
  faultTolerance: YdbParams_FaultTolerance;
  defaultDiskType: YdbParams_DiskType;
  storageGroups: number;
  autoSizePdisks: boolean;
  databasePath: string;
}

export interface YdbManagedParamsVM {
  type: YdbManagedParams_Type;
  computeType: YdbManagedParams_ComputeType;
  resourcePresetId: string;
  nodeCount: number;
  storageGroups: number;
  storageType: string;
  throttlingRcus: number;
}

export interface CockroachParamsVM {
  nodes: number;
  options: Record<string, string>;
}

export interface ExternalParamsVM {
  dsn: string;
}

/**
 * EngineParamsVM is a discriminated union over the engine kind, carrying the
 * exact typed params for the selected engine. It is the heart of the Database
 * step: the step edits this, the provider maps it onto domain.Database, Patch
 * recomputes topology from it.
 */
export type EngineParamsVM =
  | { kind: "postgres"; postgres: PostgresParamsVM }
  | { kind: "mysql"; mysql: MySqlParamsVM }
  | { kind: "mariadb"; mariadb: MySqlParamsVM }
  | { kind: "picodata"; picodata: PicodataParamsVM }
  | { kind: "ydb"; ydb: YdbParamsVM }
  | { kind: "ydbManaged"; ydbManaged: YdbManagedParamsVM }
  | { kind: "cockroach"; cockroach: CockroachParamsVM }
  | { kind: "external"; external: ExternalParamsVM };

/** The Database step's editable state: kind + version + typed engine params. */
export interface DatabaseVM {
  kind: EngineKind;
  /** DatabaseParams.version (engine version label); unused for external. */
  version: string;
  params: EngineParamsVM;
}

// --- Workload (mirror domain.Workload) ---------------------------------------

export type K6Limit =
  | { case: "duration"; duration: string }
  | { case: "iterations"; iterations: number };

export interface WorkloadExecutionVM {
  vus: number;
  limit: K6Limit;
  quiet: boolean;
  noThresholds: boolean;
}

export interface WorkloadParametersVM {
  poolSize: number;
  scaleFactor: number;
  defaultInsertMethod: string;
  env: Record<string, string>;
  /** Workload.Parameters.steps — phase allowlist (XOR with noSteps). */
  steps: string[];
  /** Workload.Parameters.no_steps — phase blocklist. */
  noSteps: string[];
}

export interface WorkloadFileVM {
  name: string;
  kind: string;
  content: string;
}

export interface WorkloadVM {
  stroppyVersion: string;
  script: string;
  sql: string;
  protocol: Workload_Protocol;
  execution: WorkloadExecutionVM;
  parameters: WorkloadParametersVM;
  files: WorkloadFileVM[];
}

// --- Server-derived preview (mirror topology + infrastructure + render) -------

/** One topology.Component, flattened (id/kind/engine/role + node placement). */
export interface TopologyComponentVM {
  id: string;
  /** Component.Kind name, lower-cased without the KIND_ prefix (e.g. "database"). */
  kind: string;
  engine: string;
  role: string;
  /** node id this component is colocated on (from Node.component_ids). */
  nodeId: string;
}

/** One topology.Node, flattened. */
export interface TopologyNodeVM {
  id: string;
  componentIds: string[];
  labels: Record<string, string>;
}

// --- Infrastructure plan (mirror deployment.InfrastructurePlan) ---------------
//
// The SERVER fills infrastructure_plan on every Patch: the provider-level
// settings (ProviderSettings oneof) + a per-node MachinePlan (Docker.Container
// or Yandex.Vm). The wizard SHOWS and EDITS these and feeds the edited
// InfrastructurePlan back into the next Patch.

/** deployment.Docker.Settings — empty top-level (network/containers are derived). */
export interface DockerSettingsVM {
  /** Docker.Settings.Network.name (the bridge network the run shares). */
  networkName: string;
}

/** deployment.Yandex.Settings — the editable provider-level Yandex config. */
export interface YandexSettingsVM {
  cloudId: string;
  folderId: string;
  /** Yandex.Settings.Zone enum (default zone for all VMs). */
  zone: number;
  networkName: string;
  subnetCidr: string;
  /** Yandex.Settings.PlatformId enum (default CPU platform for all VMs). */
  platformId: number;
  imageId: string;
  assignPublicIp: boolean;
  sshUser: string;
}

/** deployment.ProviderSettings oneof, flattened to the active variant. */
export type ProviderSettingsVM =
  | { case: "docker"; docker: DockerSettingsVM }
  | { case: "yandex"; yandex: YandexSettingsVM }
  | { case: undefined };

/** deployment.Docker.Container — the per-machine spec on a Docker run. */
export interface DockerContainerVM {
  image: string;
  /** Docker.Resources.cpu_cores (double). */
  cpuCores: number;
  /** Docker.Resources.memory_mb (uint64). */
  memoryMb: number;
}

/** deployment.Yandex.Vm — the per-machine spec on a Yandex run. */
export interface YandexVmVM {
  /** Yandex.Vm.cores (uint32). */
  cores: number;
  /** Yandex.Vm.memory_gb (uint64). */
  memoryGb: number;
  /** Yandex.Vm.boot_disk_gb (uint64). */
  bootDiskGb: number;
  /** Yandex.Vm.boot_disk_type (e.g. network-ssd, network-ssd-io-m3). */
  bootDiskType: string;
  /** Yandex.Vm.zone (per-VM override of the settings zone). */
  zone: string;
  publicIp: boolean;
}

/** deployment.MachinePlan oneof, flattened to the active provider variant. */
export type MachineSpecVM =
  | { case: "docker"; docker: DockerContainerVM }
  | { case: "yandex"; yandex: YandexVmVM }
  | { case: undefined };

/**
 * One deployment.MachinePlan: a node_id + the provider-specific spec the user
 * edits (Docker.Container / Yandex.Vm). role/engine are denormalized from the
 * colocated topology component so the form can label/size by role.
 */
export interface MachineVM {
  nodeId: string;
  /** denormalized from the colocated component (label only). */
  role: string;
  engine: string;
  spec: MachineSpecVM;
}

/** deployment.InfrastructurePlan — provider + settings + per-node machines. */
export interface InfrastructurePlanVM {
  provider: Provider;
  settings: ProviderSettingsVM;
  machines: MachineVM[];
}

/**
 * One deployment.RenderArtifact, flattened for the preview list. Carries the
 * full mutability/origin/lock_reason/base_hash so the Review step can render
 * editable configs (Mutability=EDITABLE) via the ConfigEditor and feed edits
 * back as a FileOverride keyed by id + base_hash.
 */
export interface RenderArtifactVM {
  id: string;
  componentId: string;
  /** RenderArtifact.Kind enum. */
  kind: RenderArtifact_Kind;
  /** RenderArtifact.Origin enum (RENDERED_DEFAULT | USER_OVERRIDE | ...). */
  origin: RenderArtifact_Origin;
  /** RenderArtifact.Mutability enum (READ_ONLY | EDITABLE | RUNTIME_ONLY). */
  mutability: RenderArtifact_Mutability;
  /** why a non-editable artifact is locked (RenderArtifact.lock_reason). */
  lockReason: string;
  /** path (FILE/DIRECTORY) or display title (COMMAND/RUNTIME_VALUE). */
  title: string;
  /** rendered content for files/commands; empty for runtime placeholders. */
  content: string;
  /** RenderArtifact.base_hash — the hash a FileOverride must echo back. */
  baseHash: string;
}

/** deployment.ComponentRender — groups artifacts under one component/node. */
export interface ComponentRenderVM {
  componentId: string;
  nodeId: string;
  artifactIds: string[];
}

/**
 * A user-authored config override (deployment.FileOverride). Keyed by the
 * artifact's id + base_hash; carries the edited file content. Patch ships the
 * full set (RenderOverrideSet.files) so the server re-renders and flips that
 * artifact's origin to ORIGIN_USER_OVERRIDE.
 */
export interface FileOverrideVM {
  artifactId: string;
  componentId: string;
  baseHash: string;
  /** the edited file path (common.File.Info.path). */
  path: string;
  /** the edited file body (common.File.text). */
  content: string;
}

/** One schemapb.FieldError, flattened. severity: "error" | "warning" | "info". */
export interface DraftErrorVM {
  field: string;
  message: string;
  severity: "error" | "warning" | "info";
  code: string;
}

/**
 * WizardDraftVM is the flat projection of cloud.v1.models.TestWizardDraftRecord
 * the wizard renders. provider/database/workload are the user-edited halves;
 * topology/machines/artifacts/errors/ready are the server-recomputed halves.
 */
export interface WizardDraftVM {
  id: string;
  name: string;
  provider: Provider;
  /** present once the Database step has been visited; undefined on a fresh draft. */
  database?: DatabaseVM;
  /** present once the Workload step has been visited. */
  workload?: WorkloadVM;
  /** server-derived topology component list (recomputed on each Patch). */
  topologyComponents: TopologyComponentVM[];
  /** server-derived topology nodes. */
  topologyNodes: TopologyNodeVM[];
  /**
   * server-filled deployment.InfrastructurePlan: provider settings + per-node
   * machine specs. The Infrastructure step SHOWS + EDITS this and Patches the
   * edited plan back (PatchTestWizard.infrastructure_plan).
   */
  infrastructurePlan: InfrastructurePlanVM;
  /** deployment.RenderPreview.components — artifact grouping per component. */
  renderComponents: ComponentRenderVM[];
  /**
   * deployment.RenderPreview.artifacts — per-component render output. EDITABLE
   * ones feed the ConfigEditor → FileOverride; others are shown locked.
   */
  artifacts: RenderArtifactVM[];
  /** authoritative validation/capacity errors, recomputed on each Patch. */
  errors: DraftErrorVM[];
  /** true when the whole form validates and FinishTestWizard is allowed. */
  ready: boolean;
  /** the test preset this draft was seeded from, if any. */
  testPresetId: string;
  /** entity.timings.updated_at (ISO) — drives the resume list ordering. */
  updatedAt: string;
}

/** A compact draft row for the resume ("continue where you left off") list. */
export interface DraftSummaryVM {
  id: string;
  name: string;
  /** engine kind if chosen, else "". */
  engine: EngineKind | "";
  ready: boolean;
  updatedAt: string;
}

/**
 * ProbeMetaVM is the believable subset of stroppy's `probe -o json` the
 * Workload step drives its form from (ProbeScriptResponse.metadata, a Struct).
 * The real provider passes the full Struct through; the step reads these keys.
 */
export interface ProbeMetaVM {
  /** phases the script exposes (-> Parameters.steps / no_steps pickers). */
  steps: string[];
  /** env declarations the script reads (-> Parameters.env editor). */
  env: { name: string; description: string; default: string; required: boolean }[];
  /** sql section names the script declares. */
  sqlSections: string[];
  /** driver default pool size the probe reports. */
  poolSize: number;
  /** the driver type the probe ran against. */
  driverType: string;
  /** the human-readable render (`probe -o human`), when requested. */
  human: string;
}

/** What the Patch payload carries — the typed sub-message(s) a step changed. */
export interface PatchInput {
  provider?: Provider;
  database?: DatabaseVM;
  workload?: WorkloadVM;
  /**
   * The edited deployment.InfrastructurePlan (provider settings + machine
   * specs). Set by the Infrastructure step → PatchTestWizard.infrastructure_plan;
   * the server recomputes the rest from it.
   */
  infrastructurePlan?: InfrastructurePlanVM;
  /**
   * The full deployment.RenderOverrideSet (FileOverride list) — set by the
   * Review step when a config is edited → PatchTestWizard.render_overrides.
   * Ship the WHOLE set each Patch (dropping a FileOverride resets that
   * artifact to its rendered default).
   */
  renderOverrides?: FileOverrideVM[];
}

/** FinishTestWizard inputs (launch and/or save-as-preset + rating flags). */
export interface FinishInput {
  start: boolean;
  saveAsPreset: boolean;
  presetName: string;
  inTenantRating?: boolean;
  inGlobalRating?: boolean;
}

/** FinishTestWizard result, flattened. */
export interface FinishResultVM {
  /** id of the launched run when start=true; "" otherwise. */
  runId: string;
  /** id of the saved preset when save_as_preset=true; "" otherwise. */
  presetId: string;
  /** the baked run's name (TestRun.id / entity name) for the success toast. */
  testRunName: string;
}

/**
 * WizardProvider abstracts the full StartTestWizard / Patch-loop / Finish flow
 * so the page never imports mock or backend specifics. Each method maps to one
 * TestWizardService RPC.
 */
export interface WizardProvider {
  /** StartTestWizard -> a fresh draft (optionally seeded from a test preset). */
  start(tenantSlug: string, name: string, testPresetId?: string): Promise<WizardDraftVM>;
  /** GetTestWizardDraft -> a single draft by id (for resume / Back). */
  get(tenantSlug: string, draftId: string): Promise<WizardDraftVM>;
  /** ListTestWizardDrafts -> the caller's recent drafts (resume list). */
  list(tenantSlug: string): Promise<DraftSummaryVM[]>;
  /** PatchTestWizard -> the recomputed draft (preview/validation/ready). */
  patch(tenantSlug: string, draftId: string, input: PatchInput): Promise<WizardDraftVM>;
  /** DeleteTestWizardDraft -> drop a draft. */
  remove(tenantSlug: string, draftId: string): Promise<void>;
  /** FinishTestWizard -> baked run, optionally launched and/or saved. */
  finish(tenantSlug: string, draftId: string, input: FinishInput): Promise<FinishResultVM>;
  /** ProbeScript -> stroppy script metadata to drive the Workload form. */
  probe(
    tenantSlug: string,
    input: { version: string; script: string; sql: string; driverType: string; poolSize: number; scaleFactor: number; includeHuman: boolean },
  ): Promise<ProbeMetaVM>;
}

// --- Real backend provider ---------------------------------------------------
//
// TODO(real-api): wire the connect transport (testWizardClient) and map proto
// <-> VM. Each method documents the exact RPC it will issue. Throws until wired
// so a missing-backend misconfig is loud, not a silent fake-success — the same
// convention as runs.ts / dashboard.ts.

const NOT_WIRED =
  "real WizardProvider not wired yet — run with VITE_MOCK=1 to preview the test wizard";

const realWizardProvider: WizardProvider = {
  async start() {
    // const tenantId = await resolveTenantId(tenantSlug);
    // const { draft } = await testWizardClient.startTestWizard({
    //   tenantId, name, testPresetId: testPresetId ?? "",
    // });  // cloud.v1.api.TestWizardService.StartTestWizard
    // return draftToVM(draft);
    throw new Error(NOT_WIRED);
  },
  async get() {
    // const { draft } = await testWizardClient.getTestWizardDraft({ tenantId, draftId });
    throw new Error(NOT_WIRED);
  },
  async list() {
    // const { drafts } = await testWizardClient.listTestWizardDrafts({
    //   tenantId,
    //   filter: { authorIds: [meId] },                 // "my drafts"
    //   sort: { /* updated_at desc */ },
    //   page: { size: 20 },
    // });
    // return drafts.map(draftToSummaryVM);
    throw new Error(NOT_WIRED);
  },
  async patch() {
    // Build the typed sub-messages with create(...Schema, {...}) from PatchInput:
    //   provider: input.provider,                                   // Provider enum
    //   database: databaseVMToProto(input.database),                // domain.Database
    //   workload: workloadVMToProto(input.workload),                // domain.Workload
    //   infrastructurePlan: infraPlanVMToProto(input.infrastructurePlan),
    //       // deployment.InfrastructurePlan: ProviderSettings (Docker.Settings |
    //       // Yandex.Settings) + repeated MachinePlan (Docker.Container | Yandex.Vm)
    //   renderOverrides: create(RenderOverrideSetSchema, {
    //     files: input.renderOverrides?.map((o) => create(FileOverrideSchema, {
    //       artifactId: o.artifactId, componentId: o.componentId,
    //       baseHash: o.baseHash,                          // echo the artifact base_hash
    //       file: create(FileSchema, {
    //         info: create(File_InfoSchema, { path: o.path }),
    //         content: { case: "text", value: o.content },
    //       }),
    //     })) ?? [],
    //   }),                                                // deployment.RenderOverrideSet
    // const { draft } = await testWizardClient.patchTestWizard({
    //   tenantId, draftId, provider, database, workload,
    //   infrastructurePlan, renderOverrides,
    // });  // server re-derives topology_spec/infrastructure_plan/render_preview + ready;
    //      // overridden artifacts come back with origin = ORIGIN_USER_OVERRIDE.
    // return draftToVM(draft);
    throw new Error(NOT_WIRED);
  },
  async remove() {
    // await testWizardClient.deleteTestWizardDraft({ tenantId, draftId });
    throw new Error(NOT_WIRED);
  },
  async finish() {
    // const resp = await testWizardClient.finishTestWizard({
    //   tenantId, draftId,
    //   start: input.start,
    //   saveAsPreset: input.saveAsPreset,
    //   presetName: input.presetName,
    //   inTenantRating: input.inTenantRating,          // optional bool
    //   inGlobalRating: input.inGlobalRating,          // optional bool
    // });
    // return { runId: resp.run?.entity?.id ?? "", presetId: resp.preset?.entity?.id ?? "",
    //          testRunName: resp.testRun?.id ?? "" };
    throw new Error(NOT_WIRED);
  },
  async probe() {
    // const resp = await testWizardClient.probeScript({
    //   version: input.version, script: input.script, sql: input.sql,
    //   driverType: input.driverType, poolSize: input.poolSize,
    //   scaleFactor: input.scaleFactor, includeHuman: input.includeHuman,
    // });
    // return probeMetaFromStruct(resp.metadata, resp.human);
    throw new Error(NOT_WIRED);
  },
};

// --- Provider injection ------------------------------------------------------
//
// Defaults to the real backend. The mock (and ONLY the mock) overrides it via
// setWizardProvider() from the single gate in main.tsx. To remove the mock:
// delete src/mock/ and the gated call in main.tsx.

let active: WizardProvider = realWizardProvider;

export function setWizardProvider(provider: WizardProvider): void {
  active = provider;
}

export function getWizardProvider(): WizardProvider {
  return active;
}

// --- Sensible defaults shared by the page + mock -----------------------------

export function defaultPostgresParams(): PostgresParamsVM {
  return {
    replicas: 2,
    haproxy: 0,
    pgbouncer: false,
    patroni: false,
    etcd: false,
    syncReplicas: 0,
    masterOptions: {},
    replicaOptions: {},
    haproxyOptions: {},
    pgbouncerOptions: {},
    patroniOptions: {},
    etcdOptions: {},
  };
}

export function defaultMySqlParams(): MySqlParamsVM {
  return {
    replicas: 2,
    proxysql: 0,
    groupReplication: false,
    semiSync: false,
    primaryOptions: {},
    replicaOptions: {},
    proxysqlOptions: {},
  };
}

export function defaultPicodataParams(): PicodataParamsVM {
  return {
    instances: 3,
    haproxy: 0,
    replicationFactor: 2,
    shards: 0,
    instanceOptions: {},
    haproxyOptions: {},
  };
}

export function defaultYdbParams(): YdbParamsVM {
  return {
    storageNodes: 3,
    databaseNodes: 0,
    haproxy: 0,
    pdisksPerStorageNode: 1,
    faultTolerance: YdbParams_FaultTolerance.NONE,
    defaultDiskType: YdbParams_DiskType.SSD,
    storageGroups: 1,
    autoSizePdisks: false,
    databasePath: "/Root/testdb",
  };
}

export function defaultYdbManagedParams(): YdbManagedParamsVM {
  return {
    type: YdbManagedParams_Type.SERVERLESS,
    computeType: YdbManagedParams_ComputeType.OLTP,
    resourcePresetId: "",
    nodeCount: 1,
    storageGroups: 0,
    storageType: "",
    throttlingRcus: 0,
  };
}

export function defaultCockroachParams(): CockroachParamsVM {
  return { nodes: 3, options: {} };
}

export function defaultEngineParams(kind: EngineKind): EngineParamsVM {
  switch (kind) {
    case "postgres":
      return { kind, postgres: defaultPostgresParams() };
    case "mysql":
      return { kind, mysql: defaultMySqlParams() };
    case "mariadb":
      return { kind, mariadb: defaultMySqlParams() };
    case "picodata":
      return { kind, picodata: defaultPicodataParams() };
    case "ydb":
      return { kind, ydb: defaultYdbParams() };
    case "ydbManaged":
      return { kind, ydbManaged: defaultYdbManagedParams() };
    case "cockroach":
      return { kind, cockroach: defaultCockroachParams() };
    case "external":
      return { kind, external: { dsn: "" } };
  }
}

// --- Blank (from-scratch / Custom) skeletons ---------------------------------
//
// Unlike the rich defaults above (which seed a realistic, immediately-valid
// topology), the BLANK skeletons are what the "Custom · configure from scratch"
// preset starts from: the engine kind is set and the typed params object is
// PRESENT (so the form renders + edits), but every count/replica is zeroed and
// every optional component is off. The user fills everything from zero.
//
// These are INTENTIONALLY minimal — several of these zeros are below the proto
// validate floors (e.g. PicodataParams.instances >= 1, YdbParams.storage_nodes
// >= 1, CockroachParams.nodes >= 1, YdbManagedParams.type not UNSPECIFIED), so a
// fresh Custom config surfaces real FieldErrors (mirroring the server's
// database.Validate) until the user supplies the required bits. See the
// validate() in mock/wizard.ts which mirrors those same proto rules.

export function blankPostgresParams(): PostgresParamsVM {
  return {
    replicas: 0,
    haproxy: 0,
    pgbouncer: false,
    patroni: false,
    etcd: false,
    syncReplicas: 0,
    masterOptions: {},
    replicaOptions: {},
    haproxyOptions: {},
    pgbouncerOptions: {},
    patroniOptions: {},
    etcdOptions: {},
  };
}

export function blankMySqlParams(): MySqlParamsVM {
  return {
    replicas: 0,
    proxysql: 0,
    groupReplication: false,
    semiSync: false,
    primaryOptions: {},
    replicaOptions: {},
    proxysqlOptions: {},
  };
}

export function blankPicodataParams(): PicodataParamsVM {
  // instances 0 is below the proto floor (gte:1) → forces the user to set it.
  return {
    instances: 0,
    haproxy: 0,
    replicationFactor: 0,
    shards: 0,
    instanceOptions: {},
    haproxyOptions: {},
  };
}

export function blankYdbParams(): YdbParamsVM {
  // storage_nodes 0 is below the proto floor (gte:1) → forces the user to set it.
  return {
    storageNodes: 0,
    databaseNodes: 0,
    haproxy: 0,
    pdisksPerStorageNode: 0,
    faultTolerance: YdbParams_FaultTolerance.UNSPECIFIED,
    defaultDiskType: YdbParams_DiskType.UNSPECIFIED,
    storageGroups: 0,
    autoSizePdisks: false,
    databasePath: "",
  };
}

export function blankYdbManagedParams(): YdbManagedParamsVM {
  // type UNSPECIFIED is rejected by the proto (not_in:0) → forces a choice.
  return {
    type: YdbManagedParams_Type.UNSPECIFIED,
    computeType: YdbManagedParams_ComputeType.UNSPECIFIED,
    resourcePresetId: "",
    nodeCount: 0,
    storageGroups: 0,
    storageType: "",
    throttlingRcus: 0,
  };
}

export function blankCockroachParams(): CockroachParamsVM {
  // nodes 0 is below the proto floor (gte:1) → forces the user to set it.
  return { nodes: 0, options: {} };
}

/**
 * Minimal from-scratch params for the Custom preset. Mirrors defaultEngineParams
 * shape-for-shape but zeroed/minimal so the user configures everything and the
 * backend (mirrored by the mock's validate) flags the still-missing required
 * fields until they do.
 */
export function blankEngineParams(kind: EngineKind): EngineParamsVM {
  switch (kind) {
    case "postgres":
      return { kind, postgres: blankPostgresParams() };
    case "mysql":
      return { kind, mysql: blankMySqlParams() };
    case "mariadb":
      return { kind, mariadb: blankMySqlParams() };
    case "picodata":
      return { kind, picodata: blankPicodataParams() };
    case "ydb":
      return { kind, ydb: blankYdbParams() };
    case "ydbManaged":
      return { kind, ydbManaged: blankYdbManagedParams() };
    case "cockroach":
      return { kind, cockroach: blankCockroachParams() };
    case "external":
      return { kind, external: { dsn: "" } };
  }
}

/** Default protocol the Workload step seeds when an engine is picked. */
export function defaultProtocolFor(kind: EngineKind): Workload_Protocol {
  switch (kind) {
    case "postgres":
      return Workload_Protocol.PG;
    case "mysql":
    case "mariadb":
      return Workload_Protocol.MYSQL;
    case "picodata":
      return Workload_Protocol.PICODATA;
    case "ydb":
    case "ydbManaged":
      return Workload_Protocol.YDB_GRPC;
    case "cockroach":
      return Workload_Protocol.COCKROACH;
    case "external":
      return Workload_Protocol.UNSPECIFIED;
  }
}

/** stroppy driver_type string for ProbeScript / config, per engine. */
export function driverTypeFor(kind: EngineKind): string {
  switch (kind) {
    case "postgres":
      return "postgres";
    case "mysql":
      return "mysql";
    case "mariadb":
      return "mariadb";
    case "picodata":
      return "picodata";
    case "ydb":
    case "ydbManaged":
      return "ydb";
    case "cockroach":
      return "cockroach";
    case "external":
      return "postgres";
  }
}

// --- Infrastructure-plan defaults (provider settings + machine specs) ---------

export function defaultDockerSettings(): DockerSettingsVM {
  return { networkName: "stroppy-net" };
}

export function defaultYandexSettings(): YandexSettingsVM {
  return {
    cloudId: "",
    folderId: "",
    zone: Yandex_Settings_Zone.RU_CENTRAL1_A,
    networkName: "stroppy-net",
    subnetCidr: "10.0.0.0/24",
    platformId: Yandex_Settings_PlatformId.STANDARD_V3,
    imageId: "",
    assignPublicIp: true,
    sshUser: "stroppy",
  };
}

export function defaultProviderSettings(provider: Provider): ProviderSettingsVM {
  return provider === Provider.YANDEX
    ? { case: "yandex", yandex: defaultYandexSettings() }
    : { case: "docker", docker: defaultDockerSettings() };
}

/** A believable Yandex.Vm spec for a given role (DB roles get more). */
export function defaultYandexVm(role: string): YandexVmVM {
  const heavy = role === "master" || role === "primary" || role === "instance" || role === "storage" || role === "node" || role === "replica";
  return {
    cores: heavy ? 8 : 4,
    memoryGb: heavy ? 32 : 16,
    bootDiskGb: heavy ? 200 : 50,
    bootDiskType: "network-ssd",
    zone: "",
    publicIp: false,
  };
}

/** A believable Docker.Container spec for a given role. */
export function defaultDockerContainer(role: string, engine: string): DockerContainerVM {
  const heavy = role === "master" || role === "primary" || role === "instance" || role === "storage" || role === "node" || role === "replica";
  return {
    image: engine ? `${engine}:latest` : "",
    cpuCores: heavy ? 4 : 2,
    memoryMb: heavy ? 8192 : 2048,
  };
}

export function defaultWorkload(kind: EngineKind): WorkloadVM {
  return {
    stroppyVersion: "",
    script: "tpcc/tx",
    sql: "",
    protocol: defaultProtocolFor(kind),
    execution: {
      vus: 16,
      limit: { case: "duration", duration: "5m" },
      quiet: false,
      noThresholds: false,
    },
    parameters: {
      poolSize: 16,
      scaleFactor: 1,
      defaultInsertMethod: "",
      env: {},
      steps: [],
      noSteps: [],
    },
    files: [],
  };
}
