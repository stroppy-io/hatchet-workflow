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
//   Infrastructure  .provider + .machine_overrides                  Provider + deployment.MachinePlan[]
//                     (per-node MachinePlan: Docker.Container | Yandex.Vm)
//   Database        .database                                       cloud.v1.domain.Database (typed per engine)
//   Workload        .workload (+ ProbeScript)                       cloud.v1.domain.Workload
//   Review/Render   .render_overrides (FileOverride set)            deployment.RenderOverrideSet
//                     (edit EDITABLE render artifacts → re-render)
//   Finish          FinishTestWizardRequest                         -> TestRun / TestRunRecord / TestPresetRecord
//
// The server fills (on every Patch) topology_spec + infrastructure_plan
// (settings + machines) + render_preview (artifacts + mutability + base_hash);
// the user SEES the infrastructure plan preview, confirms/edits machine_overrides
// and edits the EDITABLE artifacts.
//
// The view-model is intentionally flat (no proto Message instances) so the
// wizard UI stays decoupled from the wire format and the mock stays trivial.
// Provider methods are keyed by tenant SLUG (matching runs.ts); the real
// provider resolves slug -> tenant_id at call time exactly like runs.ts does.

import {
  Database_Kind,
  type Package as DatabasePackage,
} from "@/lib/proto/cloud/v1/domain/database_pb";
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

import { create, toJson } from "@bufbuild/protobuf";
import { testWizardClient } from "@/services/client";
import { resolveTenantId } from "@/services/tenant";
import {
  databaseProtoToVM,
  databaseVMToProto,
  workloadProtoToVM,
  workloadVMToProto,
} from "@/services/domainMappers";
import {
  TestWizardDraftRecordSchema,
  type TestWizardDraftRecord,
} from "@/lib/proto/cloud/v1/models/test_wizard_pb";
import {
  InfrastructurePlanSchema,
  MachinePlanSchema,
} from "@/lib/proto/cloud/v1/deployment/infrastructure_pb";
import { ProviderSettingsSchema } from "@/lib/proto/cloud/v1/deployment/provider_pb";
import { Docker_ContainerSchema } from "@/lib/proto/cloud/v1/deployment/docker_pb";
import { Yandex_VmSchema } from "@/lib/proto/cloud/v1/deployment/yandex_pb";
import {
  RenderOverrideSetSchema,
  FileOverrideSchema,
} from "@/lib/proto/cloud/v1/deployment/render_pb";
import { FileSchema } from "@/lib/proto/cloud/v1/common/file_pb";
import {
  normalizeYandexBootDiskType,
  normalizeYandexDiskGb,
  normalizeYandexInternalIp,
  normalizeYandexNetworkAcceleration,
  type YandexNetworkAcceleration,
} from "@/lib/machine-constraints";

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
  | "orioledb"
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
  orioledb: Database_Kind.ORIOLEDB,
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
  [Database_Kind.ORIOLEDB]: "orioledb",
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
  { kind: "orioledb", label: "OrioleDB", hex: "#E8633A", blurb: "Patched-Postgres storage engine (docker).", typed: true },
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

export interface OrioledbParamsVM {
  image: string;
  postgresOptions: Record<string, string>;
  initdbLocale: string;
  sharedBuffersMb: number;
  replicas: number;
  haproxy: number;
  replicaOptions: Record<string, string>;
  haproxyOptions: Record<string, string>;
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
  | { kind: "orioledb"; orioledb: OrioledbParamsVM }
  | { kind: "external"; external: ExternalParamsVM };

export type DatabasePackageVM = DatabasePackage;

/** The Database step's editable state: kind + version + typed engine params. */
export interface DatabaseVM {
  kind: EngineKind;
  /** DatabaseParams.version (engine version label); unused for external. */
  version: string;
  /** Database.package_id, when a stored custom/builtin package id was selected. */
  packageId?: string;
  /** DatabaseParams.package install recipe. Hidden in UI, preserved across preset -> wizard round trips. */
  installPackage?: DatabasePackageVM;
  params: EngineParamsVM;
}

// --- Workload (mirror domain.Workload) ---------------------------------------

export type K6Limit =
  | { case: "none" }
  | { case: "duration"; duration: string }
  | { case: "iterations"; iterations: number };

export interface WorkloadExecutionVM {
  /** k6 --vus; null => omit the flag (let stroppy env/config own it). */
  vus: number | null;
  /** k6 --duration|--iterations; "none" => omit the limit flag. */
  limit: K6Limit;
  /** k6 -q. */
  quiet: boolean;
  /** k6 --no-thresholds. */
  noThresholds: boolean;
  /** Free-form raw "k6 run" argv tokens, appended last, e.g. ["--max-duration", "1h"]. */
  extraArgs: string[];
}

export interface WorkloadParametersVM {
  poolSize: number;
  scaleFactor: number;
  defaultInsertMethod: string;
  /** Workload.Parameters.bulk_size — rows per bulk INSERT (plain_bulk only). 0 = default. */
  bulkSize: number;
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

/**
 * One workload segment (mirror domain.Workload.Segment): a self-contained
 * stroppy invocation — script + k6 execution + data parameters + run-scoped
 * files. Segments run sequentially on the same DB; stroppy_version and protocol
 * live on the parent WorkloadVM since they are shared across the run.
 */
export interface WorkloadSegmentVM {
  name: string;
  script: string;
  sql: string;
  execution: WorkloadExecutionVM;
  parameters: WorkloadParametersVM;
  files: WorkloadFileVM[];
}

export interface WorkloadVM {
  stroppyVersion: string;
  protocol: Workload_Protocol;
  /** Ordered segments — at least one; the first is typically the measured workload. */
  segments: WorkloadSegmentVM[];
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
// settings preview (ProviderSettings oneof) + a per-node MachinePlan
// (Docker.Container or Yandex.Vm). The wizard SHOWS this preview and sends
// explicit machine_overrides when the user confirms or edits machines.

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
  softwareAcceleratedNetwork: boolean;
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
  /** Runtime echo only. Placement comes from tenant/provider settings. */
  zone: string;
  /** Runtime echo only. Machine overrides always serialize this as "auto". */
  internalIp: string;
  /** Runtime echo only. Public IP comes from tenant/provider settings. */
  publicIp: boolean;
  /** Runtime echo only. Acceleration comes from tenant/provider settings. */
  networkAcceleration: YandexNetworkAcceleration;
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
   * machine specs. The Infrastructure step SHOWS this preview and Patches
   * explicit machine_overrides when the user confirms or edits it.
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
  env: { name: string; names: string[]; description: string; default: string; required: boolean }[];
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
   * Compatibility path for older clients that patched the edited preview.
   */
  infrastructurePlan?: InfrastructurePlanVM;
  /**
   * Explicit provider machine settings confirmed or edited by the user.
   * Set by the Infrastructure step → PatchTestWizard.machine_overrides.
   */
  machineOverrides?: InfrastructurePlanVM["machines"];
  /** Provider-level values that own Yandex placement/network toggles. */
  machineOverrideSettings?: ProviderSettingsVM;
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
  /**
   * StartTestWizard -> a fresh draft. Optionally seeded from a test preset, or —
   * via opts.sourceRunId ("New from run") — from an existing run's baked spec
   * (a fully editable clone). sourceRunId wins when both are given.
   */
  start(
    tenantSlug: string,
    name: string,
    opts?: { testPresetId?: string; sourceRunId?: string },
  ): Promise<WizardDraftVM>;
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
  /**
   * ProbeCatalog -> the runnable script ids ("<preset>/<script>") the chosen
   * stroppy binary embeds, for the Script field's dropdown. Rejects on binaries
   * that predate the catalog probe (stroppy < 5.4.0); callers fall back to free
   * text.
   */
  catalog(version: string): Promise<string[]>;
}

// --- Real backend provider ---------------------------------------------------
//
// Wires the connect transport (testWizardClient) and maps the proto
// TestWizardDraftRecord <-> the flat VMs. Database/Workload reuse domainMappers;
// topology/infrastructure/render are flattened via toJson and rebuilt from the
// edited VMs on Patch. Every method resolves the tenant slug -> tenant_id first,
// exactly like preset.ts / runs.ts.

// A topology.Component.Kind JSON-string enum -> the lower-cased label the VM
// declares (no KIND_ prefix). Unknown -> "".
function componentKindLabel(s: string | undefined): string {
  return s ? s.replace(/^KIND_/, "").toLowerCase() : "";
}

// A RenderArtifact JSON-string enum -> its numeric proto enum. The toJson()
// projection emits the JSON-string forms; map them back to the enums the VM
// declares so the Review step can drive its mutability/origin gating.
function artifactKind(s: string | undefined): RenderArtifact_Kind {
  switch (s) {
    case "KIND_FILE":
      return RenderArtifact_Kind.FILE;
    case "KIND_COMMAND":
      return RenderArtifact_Kind.COMMAND;
    case "KIND_DIRECTORY":
      return RenderArtifact_Kind.DIRECTORY;
    case "KIND_RUNTIME_VALUE":
      return RenderArtifact_Kind.RUNTIME_VALUE;
    default:
      return RenderArtifact_Kind.UNSPECIFIED;
  }
}
function artifactOrigin(s: string | undefined): RenderArtifact_Origin {
  switch (s) {
    case "ORIGIN_SYSTEM":
      return RenderArtifact_Origin.SYSTEM;
    case "ORIGIN_RENDERED_DEFAULT":
      return RenderArtifact_Origin.RENDERED_DEFAULT;
    case "ORIGIN_USER_OVERRIDE":
      return RenderArtifact_Origin.USER_OVERRIDE;
    case "ORIGIN_RUNTIME":
      return RenderArtifact_Origin.RUNTIME;
    default:
      return RenderArtifact_Origin.UNSPECIFIED;
  }
}
function artifactMutability(s: string | undefined): RenderArtifact_Mutability {
  switch (s) {
    case "MUTABILITY_READ_ONLY":
      return RenderArtifact_Mutability.READ_ONLY;
    case "MUTABILITY_EDITABLE":
      return RenderArtifact_Mutability.EDITABLE;
    case "MUTABILITY_RUNTIME_ONLY":
      return RenderArtifact_Mutability.RUNTIME_ONLY;
    default:
      return RenderArtifact_Mutability.UNSPECIFIED;
  }
}

// A FieldError.severity JSON-string enum -> the VM severity union.
function errorSeverity(s: string | undefined): DraftErrorVM["severity"] {
  switch (s) {
    case "WARNING":
      return "warning";
    case "SEVERITY_UNSPECIFIED":
      return "info";
    default:
      // ERROR (or unknown) is the blocking default.
      return "error";
  }
}

// A deployment.Provider JSON-string enum -> the numeric Provider enum.
function providerFromJson(s: string | undefined): Provider {
  return s === "PROVIDER_YANDEX"
    ? Provider.YANDEX
    : s === "PROVIDER_DOCKER"
      ? Provider.DOCKER
      : Provider.UNSPECIFIED;
}

// The JSON projection of a TestWizardDraftRecord; we only pluck the pieces the
// VM exposes (database/workload are mapped from the typed message instead).
type DraftJson = {
  entity?: { id?: string; name?: string; timings?: { updatedAt?: string } };
  provider?: string;
  topologySpec?: {
    nodes?: { id?: string; componentIds?: string[]; labels?: { [k: string]: string } }[];
    components?: { id?: string; kind?: string; engine?: string; role?: string }[];
  };
  infrastructurePlan?: {
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
  renderPreview?: {
    components?: { componentId?: string; nodeId?: string; artifactIds?: string[] }[];
    artifacts?: {
      id?: string;
      componentId?: string;
      kind?: string;
      origin?: string;
      mutability?: string;
      lockReason?: string;
      baseHash?: string;
      file?: { info?: { path?: string }; text?: string };
      cmd?: { name?: string };
      dir?: { path?: string };
      runtimeValue?: string;
    }[];
  };
  errors?: { field?: string; message?: string; severity?: string; code?: string }[];
  ready?: boolean;
  testPresetId?: string;
};

// Map a typed TestWizardDraftRecord onto the flat WizardDraftVM the wizard
// renders. database/workload use domainMappers; the server-derived halves
// (topology/infra/render/errors) are plucked from the JSON projection.
function draftToVM(draft: TestWizardDraftRecord | undefined): WizardDraftVM {
  const j = (draft ? toJson(TestWizardDraftRecordSchema, draft) : {}) as DraftJson;

  const provider = providerFromJson(j.provider);

  const topologyComponents: TopologyComponentVM[] = (j.topologySpec?.components ?? []).map(
    (c) => {
      const id = c.id ?? "";
      // node placement: the node whose component_ids include this component.
      const node = (j.topologySpec?.nodes ?? []).find((n) =>
        (n.componentIds ?? []).includes(id),
      );
      return {
        id,
        kind: componentKindLabel(c.kind),
        engine: c.engine ?? "",
        role: c.role ?? "",
        nodeId: node?.id ?? "",
      };
    },
  );

  const topologyNodes: TopologyNodeVM[] = (j.topologySpec?.nodes ?? []).map((n) => ({
    id: n.id ?? "",
    componentIds: [...(n.componentIds ?? [])],
    labels: { ...(n.labels ?? {}) },
  }));

  // role/engine per node, denormalized from the colocated component (label only).
  const componentById = new Map(topologyComponents.map((c) => [c.id, c]));
  const nodeRoleEngine = (nodeId: string): { role: string; engine: string } => {
    const node = topologyNodes.find((n) => n.id === nodeId);
    for (const cid of node?.componentIds ?? []) {
      const comp = componentById.get(cid);
      if (comp) return { role: comp.role, engine: comp.engine };
    }
    return { role: "", engine: "" };
  };

  const ip = j.infrastructurePlan;
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
    // Docker.Settings is empty in the proto; networkName has no wire home and
    // is left empty (server-derived/runtime-only).
    settings = { case: "docker", docker: { networkName: "" } };
  }

  const machines: MachineVM[] = (ip?.machines ?? []).map((m) => {
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

  const infrastructurePlan: InfrastructurePlanVM = {
    provider: providerFromJson(ip?.provider) || provider,
    settings,
    machines,
  };

  const renderComponents: ComponentRenderVM[] = (j.renderPreview?.components ?? []).map(
    (c) => ({
      componentId: c.componentId ?? "",
      nodeId: c.nodeId ?? "",
      artifactIds: [...(c.artifactIds ?? [])],
    }),
  );

  const artifacts: RenderArtifactVM[] = (j.renderPreview?.artifacts ?? []).map((a) => {
    // title: file/dir path or command/runtime title; content: the editable body.
    let title = "";
    let content = "";
    if (a.file) {
      title = a.file.info?.path ?? "";
      content = a.file.text ?? "";
    } else if (a.dir) {
      title = a.dir.path ?? "";
    } else if (a.cmd) {
      title = a.cmd.name ?? "";
    } else if (a.runtimeValue !== undefined) {
      content = a.runtimeValue;
    }
    return {
      id: a.id ?? "",
      componentId: a.componentId ?? "",
      kind: artifactKind(a.kind),
      origin: artifactOrigin(a.origin),
      mutability: artifactMutability(a.mutability),
      lockReason: a.lockReason ?? "",
      title,
      content,
      baseHash: a.baseHash ?? "",
    };
  });

  const errors: DraftErrorVM[] = (j.errors ?? []).map((e) => ({
    field: e.field ?? "",
    message: e.message ?? "",
    severity: errorSeverity(e.severity),
    code: e.code ?? "",
  }));

  // database/workload only present once their step has been visited.
  const database = draft?.database ? databaseProtoToVM(draft.database) : undefined;
  const workload = draft?.workload ? workloadProtoToVM(draft.workload) : undefined;

  return {
    id: j.entity?.id ?? "",
    name: j.entity?.name ?? "",
    provider,
    database,
    workload,
    topologyComponents,
    topologyNodes,
    infrastructurePlan,
    renderComponents,
    artifacts,
    errors,
    ready: j.ready ?? false,
    testPresetId: j.testPresetId ?? "",
    updatedAt: j.entity?.timings?.updatedAt ?? "",
  };
}

// A draft summary row for the resume list.
function draftToSummaryVM(draft: TestWizardDraftRecord): DraftSummaryVM {
  const vm = draftToVM(draft);
  return {
    id: vm.id,
    name: vm.name,
    engine: vm.database?.kind ?? "",
    ready: vm.ready,
    updatedAt: vm.updatedAt,
  };
}

function yandexNetworkAccelerationFromSettings(settings: ProviderSettingsVM | undefined): YandexNetworkAcceleration {
  return settings?.case === "yandex" && settings.yandex.softwareAcceleratedNetwork
    ? "software_accelerated"
    : "standard";
}

// Build one deployment.MachinePlan from an edited MachineVM.
export function machineVMToProto(m: MachineVM, settings?: ProviderSettingsVM) {
  if (m.spec.case === "yandex") {
    const diskType = normalizeYandexBootDiskType(m.spec.yandex.bootDiskType);
    const yandexSettings = settings?.case === "yandex" ? settings.yandex : undefined;
    return create(MachinePlanSchema, {
      nodeId: m.nodeId,
      providerParams: {
        case: "yandex",
        value: create(Yandex_VmSchema, {
          cores: Math.max(1, Math.trunc(m.spec.yandex.cores)),
          memoryGb: BigInt(Math.max(1, Math.trunc(m.spec.yandex.memoryGb))),
          bootDiskGb: BigInt(normalizeYandexDiskGb(diskType, m.spec.yandex.bootDiskGb)),
          bootDiskType: diskType,
          zone: "",
          internalIp: "auto",
          publicIp: yandexSettings?.assignPublicIp ?? false,
          networkAcceleration: yandexNetworkAccelerationFromSettings(settings),
        }),
      },
    });
  }
  if (m.spec.case === "docker") {
    return create(MachinePlanSchema, {
      nodeId: m.nodeId,
      providerParams: {
        case: "docker",
        value: create(Docker_ContainerSchema, {
          image: m.spec.docker.image,
          resources: {
            cpuCores: m.spec.docker.cpuCores,
            memoryMb: BigInt(Math.trunc(m.spec.docker.memoryMb)),
          },
        }),
      },
    });
  }
  return create(MachinePlanSchema, { nodeId: m.nodeId });
}

// Build a deployment.InfrastructurePlan from the edited InfrastructurePlanVM.
function infraPlanVMToProto(vm: InfrastructurePlanVM) {
  const settings =
    vm.settings.case === "yandex"
      ? create(ProviderSettingsSchema, {
          settings: {
            case: "yandex",
            value: {
              cloudId: vm.settings.yandex.cloudId,
              folderId: vm.settings.yandex.folderId,
              zone: vm.settings.yandex.zone,
              networkName: vm.settings.yandex.networkName,
              subnetCidr: vm.settings.yandex.subnetCidr,
              platformId: vm.settings.yandex.platformId,
              imageId: vm.settings.yandex.imageId,
              assignPublicIp: vm.settings.yandex.assignPublicIp,
              softwareAcceleratedNetwork: vm.settings.yandex.softwareAcceleratedNetwork,
              sshUser: vm.settings.yandex.sshUser,
            },
          },
        })
      : vm.settings.case === "docker"
        ? create(ProviderSettingsSchema, { settings: { case: "docker", value: {} } })
        : undefined;

  return create(InfrastructurePlanSchema, {
    provider: vm.provider,
    settings,
    machines: vm.machines.map((machine) => machineVMToProto(machine, vm.settings)),
  });
}

const realWizardProvider: WizardProvider = {
  async start(tenantSlug, name, opts) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { draft } = await testWizardClient.startTestWizard({
      tenantId,
      name,
      testPresetId: opts?.testPresetId ?? "",
      sourceRunId: opts?.sourceRunId ?? "",
    });
    return draftToVM(draft);
  },

  async get(tenantSlug, draftId) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { draft } = await testWizardClient.getTestWizardDraft({ tenantId, draftId });
    return draftToVM(draft);
  },

  async list(tenantSlug) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { drafts } = await testWizardClient.listTestWizardDrafts({
      tenantId,
      page: { size: 20 },
    });
    return drafts.map(draftToSummaryVM);
  },

  async patch(tenantSlug, draftId, input) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { draft } = await testWizardClient.patchTestWizard({
      tenantId,
      draftId,
      provider: input.provider ?? Provider.UNSPECIFIED,
      database: input.database ? databaseVMToProto(input.database) : undefined,
      workload: input.workload ? workloadVMToProto(input.workload) : undefined,
      infrastructurePlan: input.infrastructurePlan
        ? infraPlanVMToProto(input.infrastructurePlan)
        : undefined,
      machineOverrides: input.machineOverrides?.map((machine) =>
        machineVMToProto(machine, input.machineOverrideSettings),
      ),
      renderOverrides: input.renderOverrides
        ? create(RenderOverrideSetSchema, {
            files: input.renderOverrides.map((o) =>
              create(FileOverrideSchema, {
                artifactId: o.artifactId,
                componentId: o.componentId,
                baseHash: o.baseHash,
                file: create(FileSchema, {
                  info: { path: o.path },
                  content: { case: "text", value: o.content },
                }),
              }),
            ),
          })
        : undefined,
    });
    return draftToVM(draft);
  },

  async remove(tenantSlug, draftId) {
    const tenantId = await resolveTenantId(tenantSlug);
    await testWizardClient.deleteTestWizardDraft({ tenantId, draftId });
  },

  async finish(tenantSlug, draftId, input) {
    const tenantId = await resolveTenantId(tenantSlug);
    const resp = await testWizardClient.finishTestWizard({
      tenantId,
      draftId,
      start: input.start,
      saveAsPreset: input.saveAsPreset,
      presetName: input.presetName,
      inTenantRating: input.inTenantRating,
      inGlobalRating: input.inGlobalRating,
    });
    return {
      runId: resp.run?.entity?.id ?? "",
      presetId: resp.preset?.entity?.id ?? "",
      testRunName: resp.testRun?.id ?? "",
    };
  },

  async probe(tenantSlug, input) {
    const resp = await testWizardClient.probeScript({
      version: input.version,
      script: input.script,
      sql: input.sql,
      driverType: input.driverType,
      poolSize: input.poolSize,
      scaleFactor: input.scaleFactor,
      includeHuman: input.includeHuman,
    });
    return probeMetaFromStruct(resp.metadata, resp.human);
  },

  async catalog(version) {
    const resp = await testWizardClient.probeCatalog({ version });
    return resp.scripts;
  },
};

// Map stroppy's `probe -o json` Struct (loosely typed) onto the believable
// subset the Workload step reads. Unknown shapes degrade to empty defaults.
function probeMetaFromStruct(
  metadata: Record<string, unknown> | undefined,
  human: string,
): ProbeMetaVM {
  const m = (metadata ?? {}) as Record<string, unknown>;
  const asStringArray = (v: unknown): string[] =>
    Array.isArray(v) ? v.filter((x): x is string => typeof x === "string") : [];
  const asNumber = (v: unknown): number => (typeof v === "number" ? v : 0);
  const asString = (v: unknown): string => (typeof v === "string" ? v : "");

  // stroppy's `probe -o json` emits `env_declarations`, each declaring one or
  // more aliases under `names` (e.g. ["SCALE_FACTOR","WAREHOUSES"]) plus an
  // optional default + description. (Older/mock shapes used `env` with a
  // singular `name`; accept both.) We surface the first name as the primary key
  // and keep the full alias list so the form can de-dupe against its dedicated
  // controls.
  const envRaw = Array.isArray(m.env_declarations)
    ? m.env_declarations
    : Array.isArray(m.env)
      ? m.env
      : [];
  const env = envRaw
    .map((e) => {
      const o = (e ?? {}) as Record<string, unknown>;
      const names = asStringArray(o.names);
      const name = names[0] || asString(o.name);
      return {
        name,
        names: names.length > 0 ? names : name ? [name] : [],
        description: asString(o.description),
        default: asString(o.default),
        required: o.required === true,
      };
    })
    .filter((e) => e.name !== "");

  return {
    steps: asStringArray(m.steps),
    env,
    sqlSections: asStringArray(m.sqlSections ?? m.sql_sections),
    poolSize: asNumber(m.poolSize ?? m.pool_size),
    driverType: asString(m.driverType ?? m.driver_type),
    human,
  };
}

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
    case "orioledb":
      return { kind, orioledb: { image: "", postgresOptions: {}, initdbLocale: "C", sharedBuffersMb: 256, replicas: 0, haproxy: 0, replicaOptions: {}, haproxyOptions: {} } };
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
    case "orioledb":
      return { kind, orioledb: { image: "", postgresOptions: {}, initdbLocale: "C", sharedBuffersMb: 0, replicas: 0, haproxy: 0, replicaOptions: {}, haproxyOptions: {} } };
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
    case "orioledb":
      return Workload_Protocol.PG;
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
      // MariaDB speaks the MySQL wire protocol; stroppy has no mariadb driver.
      return "mysql";
    case "picodata":
      return "picodata";
    case "ydb":
    case "ydbManaged":
      return "ydb";
    case "cockroach":
      // CockroachDB speaks pg-wire; stroppy has no cockroach driver (the
      // distinct PROTOCOL_COCKROACH only changes port/url, not the driver).
      return "postgres";
    case "orioledb":
      // OrioleDB is a patched-Postgres storage engine; uses the pg wire protocol.
      return "postgres";
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
    subnetCidr: "10.0.0.0/8",
    platformId: Yandex_Settings_PlatformId.STANDARD_V3,
    imageId: "",
    assignPublicIp: true,
    softwareAcceleratedNetwork: false,
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
    internalIp: "auto",
    publicIp: false,
    networkAcceleration: "standard",
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

/** A fresh workload segment with sensible k6/data defaults. */
export function defaultSegment(name = "workload"): WorkloadSegmentVM {
  return {
    name,
    script: "tpcc/tx",
    sql: "",
    execution: {
      vus: 16,
      limit: { case: "duration", duration: "5m" },
      quiet: false,
      noThresholds: false,
      extraArgs: [],
    },
    parameters: {
      poolSize: 16,
      scaleFactor: 1,
      defaultInsertMethod: "",
      bulkSize: 0,
      env: {},
      steps: [],
      noSteps: [],
    },
    files: [],
  };
}

export function defaultWorkload(kind: EngineKind): WorkloadVM {
  return {
    stroppyVersion: "",
    protocol: defaultProtocolFor(kind),
    segments: [defaultSegment()],
  };
}
