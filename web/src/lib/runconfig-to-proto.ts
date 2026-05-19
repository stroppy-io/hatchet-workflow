// Adapter: wizard runtime types → proto messages.
//
// The wizard's RunConfig / topology types were modelled around the old REST
// JSON shape. The backend now takes typed proto messages (Database, Workload,
// TestRun). This module converts between the two so the wizard's existing
// state shape doesn't have to change while still feeding well-typed protos
// to the ConnectRPC handlers (createTestRun, createDatabasePreset,
// createWorkloadPreset, dryRunTestRun, probeStroppyConfig).

import { placeholderId } from "@/lib/proto-helpers";
import type {
  Database,
  Database_Postgres,
  Database_Mysql,
  Database_Picodata,
  Database_Picodata_Tier,
  Database_Ydb,
  Database_YdbManaged,
  Database_External,
} from "@/lib/proto/cloud/v1/catalog/database_pb";
import {
  Database_Kind,
  Database_Ydb_Shape_FaultTolerance,
  Database_Ydb_Shape_FailureDomainType,
  Database_YdbManaged_Shape_Kind,
  Database_YdbManaged_Shape_ComputeType,
  Database_YdbManaged_Shape_ResourcePresetId,
  Database_YdbManaged_Shape_StorageTypeId,
} from "@/lib/proto/cloud/v1/catalog/database_pb";
import type {
  Workload,
  Workload_Shape,
  Workload_Tuning,
  Workload_Load,
} from "@/lib/proto/cloud/v1/catalog/workload_pb";
import {
  Workload_Protocol,
  Workload_Script,
} from "@/lib/proto/cloud/v1/catalog/workload_pb";
import type { ConfigFile } from "@/lib/proto/cloud/v1/common/config_file_pb";
import type { TestRun } from "@/lib/proto/cloud/v1/testing/test_run_pb";
import type {
  PostgresTopology,
  MySQLTopology,
  PicodataTopology,
  YDBTopology,
  YDBManagedTopology,
} from "@/pages/PresetDesigner";
import type {
  DatabaseKind,
  Protocol,
  WorkloadFile,
} from "@/components/WorkloadForm";

// ─── Helpers ─────────────────────────────────────────────────

export function databaseKindToProto(kind: DatabaseKind): Database_Kind {
  switch (kind) {
    case "postgres": return Database_Kind.DATABASE_KIND_POSTGRES;
    case "mysql": return Database_Kind.DATABASE_KIND_MYSQL;
    case "mariadb": return Database_Kind.DATABASE_KIND_MARIADB;
    case "ydb": return Database_Kind.DATABASE_KIND_YDB;
    case "ydb-managed": return Database_Kind.DATABASE_KIND_YDB_MANAGED;
    case "cockroach": return Database_Kind.DATABASE_KIND_COCKROACH;
    case "picodata": return Database_Kind.DATABASE_KIND_PICODATA;
  }
}

export function protocolToProto(p: Protocol): Workload_Protocol {
  switch (p) {
    case "pg":
    case "cockroach": return Workload_Protocol.PG;
    case "mysql": return Workload_Protocol.MYSQL;
    case "picodata": return Workload_Protocol.PICODATA;
    case "ydb-grpc":
    case "ydb-grpcs": return Workload_Protocol.YDB_GRPCS;
    case "ydb-pgwire": return Workload_Protocol.YDB_PGWIRE;
  }
  return Workload_Protocol.UNSPECIFIED;
}

// Wizard's script string ("tpcc/procs", "tpcc/tx", ...) → proto enum.
export function scriptToProto(s: string): Workload_Script {
  switch (s) {
    case "tpcc/procs": return Workload_Script.TPCC_PROCS;
    case "tpcc/tx": return Workload_Script.TPCC_TX;
    case "tpcb/procs": return Workload_Script.TPCB_PROCS;
    case "tpcb/tx": return Workload_Script.TPCB_TX;
    case "tpch/tx": return Workload_Script.TPCH_TX;
    case "tpcc/tx-ydb-pgwire": return Workload_Script.TPCC_TX_YDB_PGWIRE;
    case "tpcb/tx-ydb-pgwire": return Workload_Script.TPCB_TX_YDB_PGWIRE;
  }
  return Workload_Script.UNSPECIFIED;
}

function ydbFaultToleranceToProto(s: string): Database_Ydb_Shape_FaultTolerance {
  switch (s) {
    case "none": return Database_Ydb_Shape_FaultTolerance.YDB_FAULT_TOLERANCE_NONE;
    case "block-4-2": return Database_Ydb_Shape_FaultTolerance.YDB_FAULT_TOLERANCE_BLOCK_4_2;
    case "mirror-3-dc": return Database_Ydb_Shape_FaultTolerance.YDB_FAULT_TOLERANCE_MIRROR_3_DC;
  }
  return Database_Ydb_Shape_FaultTolerance.YDB_FAULT_TOLERANCE_UNSPECIFIED;
}

function ydbFailureDomainToProto(s: string | undefined): Database_Ydb_Shape_FailureDomainType {
  switch (s) {
    case "disk": return Database_Ydb_Shape_FailureDomainType.YDB_FAILURE_DOMAIN_TYPE_DISK;
    case "body": return Database_Ydb_Shape_FailureDomainType.YDB_FAILURE_DOMAIN_TYPE_BODY;
    case "rack": return Database_Ydb_Shape_FailureDomainType.YDB_FAILURE_DOMAIN_TYPE_RACK;
    case "datacenter": return Database_Ydb_Shape_FailureDomainType.YDB_FAILURE_DOMAIN_TYPE_DATACENTER;
  }
  return Database_Ydb_Shape_FailureDomainType.YDB_FAILURE_DOMAIN_TYPE_UNSPECIFIED;
}

function ydbManagedKindToProto(s: string): Database_YdbManaged_Shape_Kind {
  if (s === "serverless") return Database_YdbManaged_Shape_Kind.YDB_MANAGED_KIND_SERVERLESS;
  if (s === "dedicated") return Database_YdbManaged_Shape_Kind.YDB_MANAGED_KIND_DEDICATED;
  return Database_YdbManaged_Shape_Kind.YDB_MANAGED_KIND_UNSPECIFIED;
}

function ydbManagedComputeToProto(s: string | undefined): Database_YdbManaged_Shape_ComputeType {
  if (s === "oltp") return Database_YdbManaged_Shape_ComputeType.YDB_MANAGED_COMPUTE_TYPE_OLTP;
  if (s === "olap") return Database_YdbManaged_Shape_ComputeType.YDB_MANAGED_COMPUTE_TYPE_OLAP;
  return Database_YdbManaged_Shape_ComputeType.YDB_MANAGED_COMPUTE_TYPE_UNSPECIFIED;
}

function ydbManagedResourceToProto(s: string | undefined): Database_YdbManaged_Shape_ResourcePresetId {
  switch (s) {
    case "small": return Database_YdbManaged_Shape_ResourcePresetId.YDB_MANAGED_RESOURCE_PRESET_ID_SMALL;
    case "medium": return Database_YdbManaged_Shape_ResourcePresetId.YDB_MANAGED_RESOURCE_PRESET_ID_MEDIUM;
    case "large": return Database_YdbManaged_Shape_ResourcePresetId.YDB_MANAGED_RESOURCE_PRESET_ID_LARGE;
    case "xlarge": return Database_YdbManaged_Shape_ResourcePresetId.YDB_MANAGED_RESOURCE_PRESET_ID_XLARGE;
    case "olap-medium": return Database_YdbManaged_Shape_ResourcePresetId.YDB_MANAGED_RESOURCE_PRESET_ID_OLAP_MEDIUM;
    case "olap-large": return Database_YdbManaged_Shape_ResourcePresetId.YDB_MANAGED_RESOURCE_PRESET_ID_OLAP_LARGE;
  }
  return Database_YdbManaged_Shape_ResourcePresetId.YDB_MANAGED_RESOURCE_PRESET_ID_UNSPECIFIED;
}

function ydbManagedStorageToProto(s: string | undefined): Database_YdbManaged_Shape_StorageTypeId {
  if (s === "ssd") return Database_YdbManaged_Shape_StorageTypeId.YDB_MANAGED_STORAGE_TYPE_ID_SSD;
  return Database_YdbManaged_Shape_StorageTypeId.YDB_MANAGED_STORAGE_TYPE_ID_UNSPECIFIED;
}

// ─── Per-engine variant constructors ─────────────────────────────────────

function toPostgres(t: PostgresTopology): Database_Postgres {
  const replicaCount = (t.replicas ?? []).reduce((s, r) => s + (r.count || 0), 0);
  return {
    $typeName: "cloud.v1.catalog.Database.Postgres",
    shape: {
      $typeName: "cloud.v1.catalog.Database.Postgres.Shape",
      replicas: replicaCount,
      syncReplicas: t.sync_replicas ?? 0,
      patroni: !!t.patroni,
      etcdColocated: !!t.etcd,
      pgbouncerColocated: !!t.pgbouncer,
      haproxyDedicated: !!t.haproxy,
    },
  } as Database_Postgres;
}

function toMysql(t: MySQLTopology): Database_Mysql {
  const replicaCount = (t.replicas ?? []).reduce((s, r) => s + (r.count || 0), 0);
  return {
    $typeName: "cloud.v1.catalog.Database.Mysql",
    shape: {
      $typeName: "cloud.v1.catalog.Database.Mysql.Shape",
      replicas: replicaCount,
      groupReplication: !!t.group_replication,
      semiSync: !!t.semi_sync,
      proxysqlDedicated: !!t.proxysql,
    },
  } as Database_Mysql;
}

function toPicodata(t: PicodataTopology): Database_Picodata {
  const totalNodes = (t.instances ?? []).reduce((s, i) => s + (i.count || 0), 0);
  const tiers: Database_Picodata_Tier[] = (t.tiers ?? []).map((tier) => ({
    $typeName: "cloud.v1.catalog.Database.Picodata.Tier",
    name: tier.name,
    replicationFactor: tier.replication_factor,
    count: tier.count,
    canVote: tier.can_vote,
  })) as Database_Picodata_Tier[];
  return {
    $typeName: "cloud.v1.catalog.Database.Picodata",
    shape: {
      $typeName: "cloud.v1.catalog.Database.Picodata.Shape",
      nodes: totalNodes,
      replicationFactor: t.replication_factor,
      shards: t.shards,
      haproxyDedicated: !!t.haproxy,
      tiers,
    },
  } as Database_Picodata;
}

function toYdb(t: YDBTopology): Database_Ydb {
  return {
    $typeName: "cloud.v1.catalog.Database.Ydb",
    shape: {
      $typeName: "cloud.v1.catalog.Database.Ydb.Shape",
      storageNodes: t.storage?.count ?? 1,
      databaseNodes: t.database?.count ?? 0,
      haproxyDedicated: !!t.haproxy,
      faultTolerance: ydbFaultToleranceToProto(t.fault_tolerance),
      storageGroups: t.storage_groups ?? 0,
      autoSizePdisks: !!t.auto_size_pdisks,
      failureDomainType: ydbFailureDomainToProto(t.failure_domain_type),
      databasePath: t.database_path ?? "",
    },
  } as Database_Ydb;
}

function toYdbManaged(t: YDBManagedTopology): Database_YdbManaged {
  return {
    $typeName: "cloud.v1.catalog.Database.YdbManaged",
    shape: {
      $typeName: "cloud.v1.catalog.Database.YdbManaged.Shape",
      kind: ydbManagedKindToProto(t.type),
      computeType: ydbManagedComputeToProto(t.compute_type),
      nodeCount: t.node_count ?? 0,
      storageGroups: t.storage_groups ?? 0,
      resourcePresetId: ydbManagedResourceToProto(t.resource_preset_id),
      storageTypeId: ydbManagedStorageToProto(t.storage_type),
      throttlingRcus: t.throttling_rcus ?? 0,
      autoScale: t.auto_scale
        ? {
            $typeName: "cloud.v1.catalog.Database.YdbManaged.Shape.AutoScale",
            minSize: t.auto_scale.min_size,
            maxSize: t.auto_scale.max_size,
            cpuUtilizationPercent: t.auto_scale.cpu_utilization_percent ?? 0,
          }
        : undefined,
    },
  } as Database_YdbManaged;
}

// ─── BYOD ────────────────────────────────────────────────────────────

export interface ExternalDBInput {
  endpoint: string;
  database?: string;
  username?: string;
  password?: string;
  ssl_mode?: string;
}

function toExternal(e: ExternalDBInput): Database_External {
  return {
    $typeName: "cloud.v1.catalog.Database.External",
    endpoint: e.endpoint,
    database: e.database ?? "",
    username: e.username ?? "",
    password: e.password ?? "",
    sslMode: e.ssl_mode ?? "",
    extraParams: {},
  } as Database_External;
}

// ─── toDatabaseProto ─────────────────────────────────────────────────

// Topology union covering every supported engine. Callers narrow per-kind.
export type AnyTopology =
  | PostgresTopology
  | MySQLTopology
  | PicodataTopology
  | YDBTopology
  | YDBManagedTopology;

export function toDatabaseProto(kind: DatabaseKind, topology: AnyTopology): Database {
  const k = databaseKindToProto(kind);
  switch (kind) {
    case "postgres":
      return {
        $typeName: "cloud.v1.catalog.Database",
        kind: k,
        variant: { case: "postgres", value: toPostgres(topology as PostgresTopology) },
      } as Database;
    case "mysql":
      return {
        $typeName: "cloud.v1.catalog.Database",
        kind: k,
        variant: { case: "mysql", value: toMysql(topology as MySQLTopology) },
      } as Database;
    case "mariadb":
      return {
        $typeName: "cloud.v1.catalog.Database",
        kind: k,
        variant: { case: "mariadb", value: toMysql(topology as MySQLTopology) },
      } as Database;
    case "picodata":
      return {
        $typeName: "cloud.v1.catalog.Database",
        kind: k,
        variant: { case: "picodata", value: toPicodata(topology as PicodataTopology) },
      } as Database;
    case "ydb":
      return {
        $typeName: "cloud.v1.catalog.Database",
        kind: k,
        variant: { case: "ydb", value: toYdb(topology as YDBTopology) },
      } as Database;
    case "ydb-managed":
      return {
        $typeName: "cloud.v1.catalog.Database",
        kind: k,
        variant: { case: "ydbManaged", value: toYdbManaged(topology as YDBManagedTopology) },
      } as Database;
    case "cockroach":
      // CockroachDB shape doesn't have a UI editor; fall back to a 1-node default.
      return {
        $typeName: "cloud.v1.catalog.Database",
        kind: k,
        variant: {
          case: "cockroach",
          value: {
            $typeName: "cloud.v1.catalog.Database.Cockroach",
            shape: { $typeName: "cloud.v1.catalog.Database.Cockroach.Shape", nodes: 1 },
          },
        },
      } as Database;
  }
}

export function toExternalDatabaseProto(kind: DatabaseKind, ext: ExternalDBInput): Database {
  return {
    $typeName: "cloud.v1.catalog.Database",
    kind: databaseKindToProto(kind),
    variant: { case: "external", value: toExternal(ext) },
  } as Database;
}

// ─── toWorkloadProto ─────────────────────────────────────────────────

export interface WorkloadCfgInput {
  protocol: Protocol;
  dbKind: DatabaseKind;
  script: string;
  sql?: string;
  duration: string;
  k6Mode: "duration" | "iterations";
  iterations?: number;
  quiet: boolean;
  noThresholds: boolean;
  vus: number;
  poolSize: number;
  scaleFactor: number;
  defaultInsertMethod: string;
  env?: Record<string, string>;
  steps?: string[];
  noSteps?: string[];
  files?: WorkloadFile[];
  configOverrideJson?: string;
}

function filesToConfigFiles(files: WorkloadFile[] | undefined): ConfigFile[] {
  if (!files?.length) return [];
  return files.map((f) => ({
    $typeName: "cloud.v1.common.ConfigFile",
    path: f.name,
    inline: f.content,
    mode: 0o644,
    owner: "",
  } as ConfigFile));
}

export function toWorkloadProto(w: WorkloadCfgInput): Workload {
  const load: Workload_Load = {
    $typeName: "cloud.v1.catalog.Workload.Load",
    vus: w.vus,
    mode:
      w.k6Mode === "iterations"
        ? { case: "iterations", value: w.iterations ?? 1 }
        : { case: "duration", value: w.duration || "5m" },
    quiet: w.quiet,
    noThresholds: w.noThresholds,
  } as Workload_Load;

  const shape: Workload_Shape = {
    $typeName: "cloud.v1.catalog.Workload.Shape",
    script: scriptToProto(w.script),
    protocol: protocolToProto(w.protocol),
    sql: w.sql ?? "",
    load,
  } as Workload_Shape;

  const tuning: Workload_Tuning = {
    $typeName: "cloud.v1.catalog.Workload.Tuning",
    poolSize: w.poolSize,
    scaleFactor: w.scaleFactor,
    defaultInsertMethod: w.defaultInsertMethod,
    env: w.env ?? {},
    steps: w.steps ?? [],
    noSteps: w.noSteps ?? [],
    files: filesToConfigFiles(w.files),
    configOverrideJson: w.configOverrideJson ?? "",
  } as Workload_Tuning;

  return {
    $typeName: "cloud.v1.catalog.Workload",
    shape,
    tuning,
  } as Workload;
}

// ─── toTestRunProto ─────────────────────────────────────────────────

export interface BuildTestRunInput {
  tenantID: string;
  name: string;
  description: string;
  // Database resolution: choose ONE of presetId / inlineDatabase / external.
  databasePresetId?: string;
  inlineDatabase?: Database;
  // Workload resolution: choose ONE of presetId / inlineWorkload.
  workloadPresetId?: string;
  inlineWorkload?: Workload;
}

export function toTestRunProto(in_: BuildTestRunInput): TestRun {
  let databaseVariant: NonNullable<TestRun["database"]>["databaseVariant"];
  if (in_.inlineDatabase) {
    databaseVariant = { case: "database", value: in_.inlineDatabase };
  } else if (in_.databasePresetId) {
    databaseVariant = { case: "databasePresetId", value: { $typeName: "cloud.v1.catalog.DatabasePresetId", value: in_.databasePresetId } as never };
  } else {
    databaseVariant = { case: undefined };
  }
  let workloadVariant: NonNullable<TestRun["workload"]>["workloadVariant"];
  if (in_.inlineWorkload) {
    workloadVariant = { case: "workload", value: in_.inlineWorkload };
  } else if (in_.workloadPresetId) {
    workloadVariant = { case: "workloadPresetId", value: { $typeName: "cloud.v1.catalog.WorkloadPresetId", value: in_.workloadPresetId } as never };
  } else {
    workloadVariant = { case: undefined };
  }
  return {
    $typeName: "cloud.v1.testing.TestRun",
    id: { $typeName: "cloud.v1.testing.TestRunId", ...placeholderId() } as never,
    tenantId: { $typeName: "cloud.v1.iam.TenantId", value: in_.tenantID } as never,
    identity: {
      $typeName: "cloud.v1.common.Identity",
      name: in_.name,
      description: in_.description,
      label: [],
    } as never,
    database: {
      $typeName: "cloud.v1.catalog.DatabaseOrPreset",
      databaseVariant,
    } as never,
    workload: {
      $typeName: "cloud.v1.catalog.WorkloadOrPreset",
      workloadVariant,
    } as never,
  } as TestRun;
}
