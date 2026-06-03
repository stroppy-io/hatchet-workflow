// Shared domain.Database <-> DatabaseVM and domain.Workload <-> WorkloadVM
// mappers. The wizard / preset surfaces speak the flat, proto-free VMs from
// services/wizard.ts; the wire speaks the typed cloud.v1.domain protos. This is
// the single place that bridges the two, in both directions, field-for-field.
//
// PROTO CORRESPONDENCE:
//   * Database.kind                         <-> DatabaseVM.kind (EngineKind)
//   * Database.source.params (DatabaseParams)<-> self-deploy engines:
//       DatabaseParams.version              <-> DatabaseVM.version
//       DatabaseParams.engine oneof arm     <-> EngineParamsVM (postgres/mysql/
//         (postgres|mysql|mariadb|picodata|     mariadb/picodata/ydb/ydbManaged/
//          ydb|ydbManaged|cockroach)           cockroach)
//   * Database.source.external (External)   <-> external engine (dsn)
//   * Workload.{stroppyVersion,script,sql,protocol,execution,parameters,files}
//                                           <-> WorkloadVM (same fields)

import { create } from "@bufbuild/protobuf";
import {
  type Database,
  DatabaseSchema,
  Database_Kind,
  type DatabaseParams,
  type PostgresParams,
  type MySqlParams,
  type PicodataParams,
  type YdbParams,
  type YdbManagedParams,
  type CockroachParams,
} from "@/lib/proto/cloud/v1/domain/database_pb";
import {
  type Workload,
  WorkloadSchema,
  Workload_Protocol,
} from "@/lib/proto/cloud/v1/domain/workload_pb";
import {
  ENGINE_TO_KIND,
  KIND_TO_ENGINE,
  type DatabaseVM,
  type EngineKind,
  type EngineParamsVM,
  type PostgresParamsVM,
  type MySqlParamsVM,
  type PicodataParamsVM,
  type YdbParamsVM,
  type YdbManagedParamsVM,
  type CockroachParamsVM,
  type ExternalParamsVM,
  type WorkloadVM,
  type WorkloadExecutionVM,
  type WorkloadParametersVM,
  type WorkloadFileVM,
  type K6Limit,
} from "@/services/wizard";

// --- per-engine params: proto -> VM -----------------------------------------

function postgresToVM(p: PostgresParams): PostgresParamsVM {
  return {
    replicas: p.replicas,
    haproxy: p.haproxy,
    pgbouncer: p.pgbouncer,
    patroni: p.patroni,
    etcd: p.etcd,
    syncReplicas: p.syncReplicas,
    masterOptions: { ...p.masterOptions },
    replicaOptions: { ...p.replicaOptions },
    haproxyOptions: { ...p.haproxyOptions },
    pgbouncerOptions: { ...p.pgbouncerOptions },
    patroniOptions: { ...p.patroniOptions },
    etcdOptions: { ...p.etcdOptions },
  };
}

function mysqlToVM(p: MySqlParams): MySqlParamsVM {
  return {
    replicas: p.replicas,
    proxysql: p.proxysql,
    groupReplication: p.groupReplication,
    semiSync: p.semiSync,
    primaryOptions: { ...p.primaryOptions },
    replicaOptions: { ...p.replicaOptions },
    proxysqlOptions: { ...p.proxysqlOptions },
  };
}

function picodataToVM(p: PicodataParams): PicodataParamsVM {
  return {
    instances: p.instances,
    haproxy: p.haproxy,
    replicationFactor: p.replicationFactor,
    shards: p.shards,
    instanceOptions: { ...p.instanceOptions },
    haproxyOptions: { ...p.haproxyOptions },
  };
}

function ydbToVM(p: YdbParams): YdbParamsVM {
  return {
    storageNodes: p.storageNodes,
    databaseNodes: p.databaseNodes,
    haproxy: p.haproxy,
    pdisksPerStorageNode: p.pdisksPerStorageNode,
    faultTolerance: p.faultTolerance,
    defaultDiskType: p.defaultDiskType,
    storageGroups: p.storageGroups,
    autoSizePdisks: p.autoSizePdisks,
    databasePath: p.databasePath,
  };
}

function ydbManagedToVM(p: YdbManagedParams): YdbManagedParamsVM {
  return {
    type: p.type,
    computeType: p.computeType,
    resourcePresetId: p.resourcePresetId,
    nodeCount: p.nodeCount,
    storageGroups: p.storageGroups,
    storageType: p.storageType,
    throttlingRcus: p.throttlingRcus,
  };
}

function cockroachToVM(p: CockroachParams): CockroachParamsVM {
  return {
    nodes: p.nodes,
    options: { ...p.options },
  };
}

// --- per-engine params: VM -> proto init -------------------------------------

function postgresToProto(vm: PostgresParamsVM): Partial<PostgresParams> {
  return {
    replicas: vm.replicas,
    haproxy: vm.haproxy,
    pgbouncer: vm.pgbouncer,
    patroni: vm.patroni,
    etcd: vm.etcd,
    syncReplicas: vm.syncReplicas,
    masterOptions: { ...vm.masterOptions },
    replicaOptions: { ...vm.replicaOptions },
    haproxyOptions: { ...vm.haproxyOptions },
    pgbouncerOptions: { ...vm.pgbouncerOptions },
    patroniOptions: { ...vm.patroniOptions },
    etcdOptions: { ...vm.etcdOptions },
  };
}

function mysqlToProto(vm: MySqlParamsVM): Partial<MySqlParams> {
  return {
    replicas: vm.replicas,
    proxysql: vm.proxysql,
    groupReplication: vm.groupReplication,
    semiSync: vm.semiSync,
    primaryOptions: { ...vm.primaryOptions },
    replicaOptions: { ...vm.replicaOptions },
    proxysqlOptions: { ...vm.proxysqlOptions },
  };
}

function picodataToProto(vm: PicodataParamsVM): Partial<PicodataParams> {
  return {
    instances: vm.instances,
    haproxy: vm.haproxy,
    replicationFactor: vm.replicationFactor,
    shards: vm.shards,
    instanceOptions: { ...vm.instanceOptions },
    haproxyOptions: { ...vm.haproxyOptions },
  };
}

function ydbToProto(vm: YdbParamsVM): Partial<YdbParams> {
  return {
    storageNodes: vm.storageNodes,
    databaseNodes: vm.databaseNodes,
    haproxy: vm.haproxy,
    pdisksPerStorageNode: vm.pdisksPerStorageNode,
    faultTolerance: vm.faultTolerance,
    defaultDiskType: vm.defaultDiskType,
    storageGroups: vm.storageGroups,
    autoSizePdisks: vm.autoSizePdisks,
    databasePath: vm.databasePath,
  };
}

function ydbManagedToProto(vm: YdbManagedParamsVM): Partial<YdbManagedParams> {
  return {
    type: vm.type,
    computeType: vm.computeType,
    resourcePresetId: vm.resourcePresetId,
    nodeCount: vm.nodeCount,
    storageGroups: vm.storageGroups,
    storageType: vm.storageType,
    throttlingRcus: vm.throttlingRcus,
  };
}

function cockroachToProto(vm: CockroachParamsVM): Partial<CockroachParams> {
  return {
    nodes: vm.nodes,
    options: { ...vm.options },
  };
}

// --- Database: proto -> VM ---------------------------------------------------

/** Map a typed domain.Database onto the flat DatabaseVM the wizard step edits. */
export function databaseProtoToVM(db: Database | undefined): DatabaseVM {
  const kind = db ? (KIND_TO_ENGINE[db.kind] ?? "postgres") : "postgres";

  // external engine: source.external carries dsn; no DatabaseParams/version.
  if (kind === "external") {
    const ext: ExternalParamsVM = {
      dsn: db?.source.case === "external" ? db.source.value.dsn : "",
    };
    return { kind: "external", version: "", params: { kind: "external", external: ext } };
  }

  const params = db?.source.case === "params" ? db.source.value : undefined;
  const version = params?.version ?? "";
  return { kind, version, params: engineParamsProtoToVM(kind, params) };
}

function engineParamsProtoToVM(
  kind: Exclude<EngineKind, "external">,
  params: DatabaseParams | undefined,
): EngineParamsVM {
  const engine = params?.engine;
  switch (kind) {
    case "postgres":
      return {
        kind: "postgres",
        postgres: postgresToVM(
          engine?.case === "postgres" ? engine.value : create_PostgresEmpty(),
        ),
      };
    case "mysql":
      return {
        kind: "mysql",
        mysql: mysqlToVM(engine?.case === "mysql" ? engine.value : create_MySqlEmpty()),
      };
    case "mariadb":
      return {
        kind: "mariadb",
        mariadb: mysqlToVM(engine?.case === "mariadb" ? engine.value : create_MySqlEmpty()),
      };
    case "picodata":
      return {
        kind: "picodata",
        picodata: picodataToVM(
          engine?.case === "picodata" ? engine.value : create_PicodataEmpty(),
        ),
      };
    case "ydb":
      return {
        kind: "ydb",
        ydb: ydbToVM(engine?.case === "ydb" ? engine.value : create_YdbEmpty()),
      };
    case "ydbManaged":
      return {
        kind: "ydbManaged",
        ydbManaged: ydbManagedToVM(
          engine?.case === "ydbManaged" ? engine.value : create_YdbManagedEmpty(),
        ),
      };
    case "cockroach":
      return {
        kind: "cockroach",
        cockroach: cockroachToVM(
          engine?.case === "cockroach" ? engine.value : create_CockroachEmpty(),
        ),
      };
  }
}

// Zero-valued proto params used when the oneof arm is absent (mismatched kind).
function create_PostgresEmpty(): PostgresParams {
  return {
    $typeName: "cloud.v1.domain.PostgresParams",
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
function create_MySqlEmpty(): MySqlParams {
  return {
    $typeName: "cloud.v1.domain.MySqlParams",
    replicas: 0,
    proxysql: 0,
    groupReplication: false,
    semiSync: false,
    primaryOptions: {},
    replicaOptions: {},
    proxysqlOptions: {},
  };
}
function create_PicodataEmpty(): PicodataParams {
  return {
    $typeName: "cloud.v1.domain.PicodataParams",
    instances: 0,
    haproxy: 0,
    replicationFactor: 0,
    shards: 0,
    tiers: [],
    instanceOptions: {},
    haproxyOptions: {},
  };
}
function create_YdbEmpty(): YdbParams {
  return {
    $typeName: "cloud.v1.domain.YdbParams",
    storageNodes: 0,
    databaseNodes: 0,
    haproxy: 0,
    pdisksPerStorageNode: 0,
    faultTolerance: 0,
    failureDomainType: 0,
    defaultDiskType: 0,
    storageGroups: 0,
    autoSizePdisks: false,
    databasePath: "",
    storageOptions: {},
    databaseOptions: {},
    haproxyOptions: {},
  };
}
function create_YdbManagedEmpty(): YdbManagedParams {
  return {
    $typeName: "cloud.v1.domain.YdbManagedParams",
    type: 0,
    computeType: 0,
    resourcePresetId: "",
    nodeCount: 0,
    storageGroups: 0,
    storageType: "",
    throttlingRcus: 0,
  };
}
function create_CockroachEmpty(): CockroachParams {
  return {
    $typeName: "cloud.v1.domain.CockroachParams",
    nodes: 0,
    options: {},
  };
}

// --- Database: VM -> proto ---------------------------------------------------

/** Build a typed domain.Database from the flat DatabaseVM. */
export function databaseVMToProto(vm: DatabaseVM): Database {
  if (vm.kind === "external") {
    const dsn = vm.params.kind === "external" ? vm.params.external.dsn : "";
    return create(DatabaseSchema, {
      kind: Database_Kind.EXTERNAL,
      source: { case: "external", value: { dsn } },
    });
  }

  return create(DatabaseSchema, {
    kind: ENGINE_TO_KIND[vm.kind],
    source: {
      case: "params",
      value: { version: vm.version, engine: engineParamsVMToProto(vm.params) },
    },
  });
}

function engineParamsVMToProto(p: EngineParamsVM): DatabaseParams["engine"] {
  switch (p.kind) {
    case "postgres":
      return { case: "postgres", value: postgresToProto(p.postgres) as PostgresParams };
    case "mysql":
      return { case: "mysql", value: mysqlToProto(p.mysql) as MySqlParams };
    case "mariadb":
      return { case: "mariadb", value: mysqlToProto(p.mariadb) as MySqlParams };
    case "picodata":
      return { case: "picodata", value: picodataToProto(p.picodata) as PicodataParams };
    case "ydb":
      return { case: "ydb", value: ydbToProto(p.ydb) as YdbParams };
    case "ydbManaged":
      return { case: "ydbManaged", value: ydbManagedToProto(p.ydbManaged) as YdbManagedParams };
    case "cockroach":
      return { case: "cockroach", value: cockroachToProto(p.cockroach) as CockroachParams };
    case "external":
      // external never reaches here (handled in databaseVMToProto), but keep
      // the switch exhaustive.
      return { case: undefined };
  }
}

// --- Workload: proto -> VM ---------------------------------------------------

/** Map a typed domain.Workload onto the flat WorkloadVM the wizard step edits. */
export function workloadProtoToVM(w: Workload | undefined): WorkloadVM {
  const exec = w?.execution;
  let limit: K6Limit;
  if (exec?.limit.case === "iterations") {
    limit = { case: "iterations", iterations: exec.limit.value };
  } else {
    limit = { case: "duration", duration: exec?.limit.case === "duration" ? exec.limit.value : "" };
  }
  const execution: WorkloadExecutionVM = {
    vus: exec?.vus ?? 0,
    limit,
    quiet: exec?.quiet ?? false,
    noThresholds: exec?.noThresholds ?? false,
  };

  const p = w?.parameters;
  const parameters: WorkloadParametersVM = {
    poolSize: p?.poolSize ?? 0,
    scaleFactor: p?.scaleFactor ?? 0,
    defaultInsertMethod: p?.defaultInsertMethod ?? "",
    env: { ...(p?.env ?? {}) },
    steps: [...(p?.steps ?? [])],
    noSteps: [...(p?.noSteps ?? [])],
  };

  const files: WorkloadFileVM[] = (w?.files ?? []).map((f) => ({
    name: f.name,
    kind: f.kind,
    content: f.content,
  }));

  return {
    stroppyVersion: w?.stroppyVersion ?? "",
    script: w?.script ?? "",
    sql: w?.sql ?? "",
    protocol: w?.protocol ?? Workload_Protocol.UNSPECIFIED,
    execution,
    parameters,
    files,
  };
}

// --- Workload: VM -> proto ---------------------------------------------------

/** Build a typed domain.Workload from the flat WorkloadVM. */
export function workloadVMToProto(vm: WorkloadVM): Workload {
  const limit =
    vm.execution.limit.case === "iterations"
      ? ({ case: "iterations", value: vm.execution.limit.iterations } as const)
      : ({ case: "duration", value: vm.execution.limit.duration } as const);

  return create(WorkloadSchema, {
    stroppyVersion: vm.stroppyVersion,
    script: vm.script,
    sql: vm.sql,
    protocol: vm.protocol,
    execution: {
      vus: vm.execution.vus,
      limit,
      quiet: vm.execution.quiet,
      noThresholds: vm.execution.noThresholds,
    },
    parameters: {
      poolSize: vm.parameters.poolSize,
      scaleFactor: vm.parameters.scaleFactor,
      defaultInsertMethod: vm.parameters.defaultInsertMethod,
      env: { ...vm.parameters.env },
      steps: [...vm.parameters.steps],
      noSteps: [...vm.parameters.noSteps],
    },
    files: vm.files.map((f) => ({ name: f.name, kind: f.kind, content: f.content })),
  });
}
