// TEMPORARY mock — MUST be deleted before real API wiring; do not build on this.
//
// Canned per-engine database presets mirroring the builtin set the backend
// seeds (DatabasePresetService system presets). Each preset carries a believable
// typed DatabaseVM so picking it in the wizard's Database step pre-fills the
// settings pane realistically. Like mock/wizard.ts this is read-only/isolated —
// it only answers listDatabasePresets(engine).

import {
  type DatabasePresetVM,
  type WorkloadPresetVM,
  type PresetProvider,
  type DatabasePresetRow,
  type WorkloadPresetRow,
  type TestPresetRow,
  type PresetListQuery,
  type PresetPage,
  type PresetSortField,
  type PresetKind,
} from "@/services/preset";
import type { DbKind, Protocol } from "@/components/library-table/labels";
import {
  type DatabaseVM,
  type WorkloadVM,
  type EngineKind,
  blankEngineParams,
  defaultPostgresParams,
  defaultMySqlParams,
  defaultPicodataParams,
  defaultYdbParams,
  defaultYdbManagedParams,
  defaultCockroachParams,
  YdbParams_FaultTolerance,
  YdbManagedParams_Type,
  YdbManagedParams_ComputeType,
  Workload_Protocol,
} from "@/services/wizard";

function delay(): Promise<void> {
  return new Promise((r) => setTimeout(r, 120));
}

/** Newest default engine version per engine (matches the wizard's DB_VERSIONS head). */
const DEFAULT_VERSION: Record<EngineKind, string> = {
  postgres: "16",
  mysql: "8.4",
  mariadb: "11.4",
  picodata: "25.3",
  ydb: "25.2",
  ydbManaged: "managed",
  cockroach: "24.2",
  external: "",
};

function preset(
  engine: EngineKind,
  id: string,
  name: string,
  description: string,
  database: DatabaseVM,
): DatabasePresetVM {
  return { id: `sys-${engine}-${id}`, name, description, isSystem: true, database };
}

/**
 * The "Custom · configure from scratch" entry every engine offers. UNLIKE the
 * builtins below (which seed a realistic, immediately-valid DatabaseVM), this
 * starts from a truly BLANK skeleton: the engine kind is set and the typed
 * params are present, but every count/replica is zeroed, no haproxy/patroni/etc,
 * and even the version is left unchosen. The user fills everything from zero;
 * the backend (mirrored by mock/wizard.ts's validate) flags the missing
 * required fields as FieldErrors until they do. It is NOT a system preset
 * (isSystem:false), so the picker renders it distinctly from the builtins.
 */
export function blankPreset(engine: EngineKind): DatabasePresetVM {
  return {
    id: `blank-${engine}`,
    name: "Custom · configure from scratch",
    description: "Empty skeleton — set every parameter yourself; validated as you go.",
    isSystem: false,
    // version "" (not the engine default) keeps it from-scratch — external has no version.
    database: { kind: engine, version: "", params: blankEngineParams(engine) },
  };
}

// --- Per-engine builtin presets ----------------------------------------------

function postgresPresets(): DatabasePresetVM[] {
  const v = DEFAULT_VERSION.postgres;
  return [
    preset("postgres", "single", "Single node", "One primary, no replicas — fast smoke tests.", {
      kind: "postgres",
      version: v,
      params: { kind: "postgres", postgres: { ...defaultPostgresParams(), replicas: 0 } },
    }),
    preset("postgres", "ha", "HA (Patroni + etcd)", "Primary + 2 replicas, Patroni HA over etcd, HAProxy LB.", {
      kind: "postgres",
      version: v,
      params: {
        kind: "postgres",
        postgres: {
          ...defaultPostgresParams(),
          replicas: 2,
          syncReplicas: 1,
          haproxy: 1,
          patroni: true,
          etcd: true,
          pgbouncer: true,
        },
      },
    }),
    preset("postgres", "scale", "Scale-out reads", "Primary + 4 streaming replicas behind 2 HAProxy nodes.", {
      kind: "postgres",
      version: v,
      params: { kind: "postgres", postgres: { ...defaultPostgresParams(), replicas: 4, haproxy: 2, pgbouncer: true } },
    }),
  ];
}

function mysqlFamilyPresets(engine: "mysql" | "mariadb"): DatabasePresetVM[] {
  const v = DEFAULT_VERSION[engine];
  const wrap = (p: ReturnType<typeof defaultMySqlParams>): DatabaseVM =>
    engine === "mysql"
      ? { kind: "mysql", version: v, params: { kind: "mysql", mysql: p } }
      : { kind: "mariadb", version: v, params: { kind: "mariadb", mariadb: p } };
  return [
    preset(engine, "single", "Single node", "One primary, no replicas.", wrap({ ...defaultMySqlParams(), replicas: 0 })),
    preset(engine, "replica", "Primary + replica", "Async primary/replica with semi-sync.", wrap({ ...defaultMySqlParams(), replicas: 1, semiSync: true })),
    preset(engine, "group", "Group replication", "3-node group replication behind 1 ProxySQL.", wrap({ ...defaultMySqlParams(), replicas: 2, proxysql: 1, groupReplication: true })),
  ];
}

function picodataPresets(): DatabasePresetVM[] {
  const v = DEFAULT_VERSION.picodata;
  const wrap = (p: ReturnType<typeof defaultPicodataParams>): DatabaseVM => ({
    kind: "picodata",
    version: v,
    params: { kind: "picodata", picodata: p },
  });
  return [
    preset("picodata", "single", "Single instance", "One instance, replication factor 1.", wrap({ ...defaultPicodataParams(), instances: 1, replicationFactor: 1 })),
    preset("picodata", "cluster", "3-node cluster", "3 instances, replication factor 2, 1 HAProxy.", wrap({ ...defaultPicodataParams(), instances: 3, replicationFactor: 2, haproxy: 1 })),
    preset("picodata", "scale", "Sharded scale-out", "6 instances, 3 shards, replication factor 2.", wrap({ ...defaultPicodataParams(), instances: 6, shards: 3, replicationFactor: 2, haproxy: 1 })),
  ];
}

function ydbPresets(): DatabasePresetVM[] {
  const v = DEFAULT_VERSION.ydb;
  const wrap = (p: ReturnType<typeof defaultYdbParams>): DatabaseVM => ({
    kind: "ydb",
    version: v,
    params: { kind: "ydb", ydb: p },
  });
  return [
    preset("ydb", "single", "Single (no fault tolerance)", "1 storage node, combined database role — dev only.", wrap({ ...defaultYdbParams(), storageNodes: 1, faultTolerance: YdbParams_FaultTolerance.NONE })),
    preset("ydb", "block42", "block-4-2 erasure", "8 storage nodes, block-4-2, separate database nodes.", wrap({ ...defaultYdbParams(), storageNodes: 8, databaseNodes: 2, faultTolerance: YdbParams_FaultTolerance.BLOCK_4_2, storageGroups: 2 })),
    preset("ydb", "mirror3dc", "mirror-3-dc", "9 storage nodes across 3 DCs, mirror-3-dc, HAProxy.", wrap({ ...defaultYdbParams(), storageNodes: 9, databaseNodes: 3, haproxy: 1, faultTolerance: YdbParams_FaultTolerance.MIRROR_3_DC, storageGroups: 3 })),
  ];
}

function ydbManagedPresets(): DatabasePresetVM[] {
  const wrap = (p: ReturnType<typeof defaultYdbManagedParams>): DatabaseVM => ({
    kind: "ydbManaged",
    version: DEFAULT_VERSION.ydbManaged,
    params: { kind: "ydbManaged", ydbManaged: p },
  });
  return [
    preset("ydbManaged", "serverless", "Serverless", "Cloud serverless YDB — pay per request unit.", wrap({ ...defaultYdbManagedParams(), type: YdbManagedParams_Type.SERVERLESS, computeType: YdbManagedParams_ComputeType.OLTP })),
    preset("ydbManaged", "dedicated", "Dedicated", "Dedicated 3-node managed cluster, medium preset.", wrap({ ...defaultYdbManagedParams(), type: YdbManagedParams_Type.DEDICATED, computeType: YdbManagedParams_ComputeType.OLTP, resourcePresetId: "medium", nodeCount: 3, storageGroups: 2 })),
  ];
}

function cockroachPresets(): DatabasePresetVM[] {
  const v = DEFAULT_VERSION.cockroach;
  const wrap = (p: ReturnType<typeof defaultCockroachParams>): DatabaseVM => ({
    kind: "cockroach",
    version: v,
    params: { kind: "cockroach", cockroach: p },
  });
  return [
    preset("cockroach", "single", "Single node", "One node — dev only, no resilience.", wrap({ ...defaultCockroachParams(), nodes: 1 })),
    preset("cockroach", "cluster3", "3-node cluster", "3 homogeneous nodes — survives one failure.", wrap({ ...defaultCockroachParams(), nodes: 3 })),
    preset("cockroach", "cluster6", "6-node cluster", "6 homogeneous nodes — multi-region scale.", wrap({ ...defaultCockroachParams(), nodes: 6 })),
  ];
}

function externalPresets(): DatabasePresetVM[] {
  return [
    preset("external", "dsn", "Existing endpoint", "Connect to an existing database via DSN — no deploy.", {
      kind: "external",
      version: "",
      params: { kind: "external", external: { dsn: "" } },
    }),
  ];
}

const BUILTINS: Record<EngineKind, () => DatabasePresetVM[]> = {
  postgres: postgresPresets,
  mysql: () => mysqlFamilyPresets("mysql"),
  mariadb: () => mysqlFamilyPresets("mariadb"),
  picodata: picodataPresets,
  ydb: ydbPresets,
  ydbManaged: ydbManagedPresets,
  cockroach: cockroachPresets,
  external: externalPresets,
};

// --- Workload presets ---------------------------------------------------------
//
// Reusable stroppy workload configs the Workload step seeds its Parameters pane
// from — mirroring the database presets above but for cloud.v1.domain.Workload.
// Each builtin carries a believable typed WorkloadVM (script, protocol, k6
// execution vus+limit, pool/scale/insert, phase steps, env) so picking it
// pre-fills the form realistically. Workloads are provider/engine agnostic, so
// the catalog is flat (not per-engine). stroppyVersion is left "" — the Version
// pane already chose it; seeding a preset must not clobber that choice (the
// page merges the preset's workload onto the picked version).

function wPreset(
  id: string,
  name: string,
  description: string,
  workload: WorkloadVM,
): WorkloadPresetVM {
  return { id: `sys-workload-${id}`, name, description, isSystem: true, workload };
}

/** Builtin workload base: realistic k6 execution + data params, no version. */
function workload(over: {
  script: string;
  sql?: string;
  protocol: Workload_Protocol;
  vus: number;
  duration?: string;
  iterations?: number;
  poolSize: number;
  scaleFactor?: number;
  defaultInsertMethod?: string;
  steps?: string[];
  env?: Record<string, string>;
}): WorkloadVM {
  return {
    stroppyVersion: "",
    script: over.script,
    sql: over.sql ?? "",
    protocol: over.protocol,
    execution: {
      vus: over.vus,
      limit:
        over.iterations !== undefined
          ? { case: "iterations", iterations: over.iterations }
          : { case: "duration", duration: over.duration ?? "10m" },
      quiet: false,
      noThresholds: false,
    },
    parameters: {
      poolSize: over.poolSize,
      scaleFactor: over.scaleFactor ?? 1,
      defaultInsertMethod: over.defaultInsertMethod ?? "",
      env: over.env ?? {},
      steps: over.steps ?? [],
      noSteps: [],
    },
    files: [],
  };
}

/**
 * The "Custom · configure from scratch" workload entry. UNLIKE the builtins
 * (which seed a realistic, immediately-valid WorkloadVM) this starts BLANK: no
 * script, minimal k6 execution, zeroed data params. The user fills it from
 * scratch; the backend (mirrored by mock/wizard.ts's validate) flags the
 * missing required fields (workload.script, …) as FieldErrors until they do.
 * Not a system preset (isSystem:false) so the picker renders it distinctly.
 */
export function blankWorkloadPreset(): WorkloadPresetVM {
  return {
    id: "blank-workload",
    name: "Custom · configure from scratch",
    description: "Empty load — set the script and every parameter yourself; validated as you go.",
    isSystem: false,
    workload: {
      stroppyVersion: "",
      script: "",
      sql: "",
      protocol: Workload_Protocol.UNSPECIFIED,
      execution: {
        vus: 1,
        limit: { case: "duration", duration: "" },
        quiet: false,
        noThresholds: false,
      },
      parameters: {
        poolSize: 0,
        scaleFactor: 0,
        defaultInsertMethod: "",
        env: {},
        steps: [],
        noSteps: [],
      },
      files: [],
    },
  };
}

function workloadPresets(): WorkloadPresetVM[] {
  return [
    wPreset(
      "tpcc",
      "TPC-C",
      "OLTP order-entry mix (5 transaction types) — the standard contention benchmark.",
      workload({
        script: "tpcc/tx",
        protocol: Workload_Protocol.PG,
        vus: 32,
        duration: "15m",
        poolSize: 32,
        scaleFactor: 10,
        defaultInsertMethod: "copy",
        steps: ["create_schema", "load_data", "warmup", "workload"],
        env: { WAREHOUSES: "10", TERMINALS: "10" },
      }),
    ),
    wPreset(
      "ycsb-a",
      "YCSB-A (50/50)",
      "Update-heavy key-value mix — 50% reads, 50% writes (Zipfian).",
      workload({
        script: "ycsb/a",
        protocol: Workload_Protocol.PG,
        vus: 64,
        duration: "10m",
        poolSize: 64,
        scaleFactor: 1,
        steps: ["create_schema", "load_data", "workload"],
        env: { RECORD_COUNT: "1000000", REQUEST_DISTRIBUTION: "zipfian" },
      }),
    ),
    wPreset(
      "ycsb-b",
      "YCSB-B (95/5)",
      "Read-mostly key-value mix — 95% reads, 5% writes.",
      workload({
        script: "ycsb/b",
        protocol: Workload_Protocol.PG,
        vus: 64,
        duration: "10m",
        poolSize: 64,
        scaleFactor: 1,
        steps: ["create_schema", "load_data", "workload"],
        env: { RECORD_COUNT: "1000000", REQUEST_DISTRIBUTION: "zipfian" },
      }),
    ),
    wPreset(
      "ycsb-c",
      "YCSB-C (read-only)",
      "100% reads — point-lookup throughput ceiling.",
      workload({
        script: "ycsb/c",
        protocol: Workload_Protocol.PG,
        vus: 96,
        duration: "10m",
        poolSize: 96,
        scaleFactor: 1,
        steps: ["create_schema", "load_data", "workload"],
        env: { RECORD_COUNT: "1000000" },
      }),
    ),
    wPreset(
      "pgbench-rw",
      "pgbench rw-mix",
      "TPC-B-like read/write mix (pgbench default) — quick baseline.",
      workload({
        script: "pgbench/tpcb",
        protocol: Workload_Protocol.PG,
        vus: 16,
        duration: "5m",
        poolSize: 16,
        scaleFactor: 100,
        defaultInsertMethod: "copy",
        steps: ["create_schema", "load_data", "workload"],
        env: { SCALE: "100" },
      }),
    ),
    wPreset(
      "tpch",
      "TPC-H",
      "Analytical decision-support — 22 long-running OLAP queries.",
      workload({
        script: "tpch/queries",
        sql: "tpch.sql",
        protocol: Workload_Protocol.PG,
        vus: 4,
        iterations: 22,
        poolSize: 4,
        scaleFactor: 1,
        steps: ["create_schema", "load_data", "workload"],
        env: { SCALE_FACTOR: "1" },
      }),
    ),
  ];
}

// =====================================================================
// Library table rows — stateful per-tenant stores (like mock/runs.ts).
//
// Believable database/workload/test preset rows per tenant slug so the Library
// pages' filters, sort and paging are genuinely demonstrable. Each row maps
// 1:1 to the matching record's Entity + Summary. Kept separate from the wizard
// catalogs above (those serve the wizard's typed VMs; these serve flat rows).
// =====================================================================

function daysAgo(d: number): string {
  return new Date(Date.now() - d * 86_400_000).toISOString();
}

function dbRow(p: {
  id: string;
  name: string;
  description: string;
  author: string;
  isSystem: boolean;
  dbKind: DbKind;
  version: string;
  external?: boolean;
  favorite?: boolean;
  createdDaysAgo: number;
  updatedDaysAgo: number;
}): DatabasePresetRow {
  return {
    id: p.id,
    name: p.name,
    description: p.description,
    authorId: p.author,
    isSystem: p.isSystem,
    isFavorite: p.favorite ?? false,
    dbKind: p.dbKind,
    version: p.version,
    external: p.external ?? false,
    createdAt: daysAgo(p.createdDaysAgo),
    updatedAt: daysAgo(p.updatedDaysAgo),
  };
}

function wlRow(p: {
  id: string;
  name: string;
  description: string;
  author: string;
  isSystem: boolean;
  protocol: Protocol;
  stroppyVersion: string;
  script: string;
  favorite?: boolean;
  createdDaysAgo: number;
  updatedDaysAgo: number;
}): WorkloadPresetRow {
  return {
    id: p.id,
    name: p.name,
    description: p.description,
    authorId: p.author,
    isSystem: p.isSystem,
    isFavorite: p.favorite ?? false,
    protocol: p.protocol,
    stroppyVersion: p.stroppyVersion,
    script: p.script,
    createdAt: daysAgo(p.createdDaysAgo),
    updatedAt: daysAgo(p.updatedDaysAgo),
  };
}

function tpRow(p: {
  id: string;
  name: string;
  description: string;
  author: string;
  isSystem: boolean;
  dbKind: DbKind;
  protocol: Protocol;
  stroppyVersion: string;
  favorite?: boolean;
  createdDaysAgo: number;
  updatedDaysAgo: number;
}): TestPresetRow {
  return {
    id: p.id,
    name: p.name,
    description: p.description,
    authorId: p.author,
    isSystem: p.isSystem,
    isFavorite: p.favorite ?? false,
    dbKind: p.dbKind,
    protocol: p.protocol,
    stroppyVersion: p.stroppyVersion,
    createdAt: daysAgo(p.createdDaysAgo),
    updatedAt: daysAgo(p.updatedDaysAgo),
  };
}

const ACME_DB: DatabasePresetRow[] = [
  dbRow({ id: "dbp-pg-single", name: "PG single", description: "One primary, no replicas.", author: "system", isSystem: true, dbKind: "postgres", version: "16", createdDaysAgo: 120, updatedDaysAgo: 120 }),
  dbRow({ id: "dbp-pg-ha", name: "PG HA x3", description: "Patroni HA, 2 replicas, HAProxy.", author: "system", isSystem: true, dbKind: "postgres", version: "16", favorite: true, createdDaysAgo: 120, updatedDaysAgo: 30 }),
  dbRow({ id: "dbp-pg-scale", name: "PG scale-out", description: "4 streaming replicas.", author: "ada", isSystem: false, dbKind: "postgres", version: "16", favorite: true, createdDaysAgo: 18, updatedDaysAgo: 2 }),
  dbRow({ id: "dbp-mysql-group", name: "MySQL group", description: "3-node group replication.", author: "system", isSystem: true, dbKind: "mysql", version: "8.4", createdDaysAgo: 110, updatedDaysAgo: 110 }),
  dbRow({ id: "dbp-maria-replica", name: "MariaDB replica", description: "Primary + replica, semi-sync.", author: "grace", isSystem: false, dbKind: "mariadb", version: "11.4", createdDaysAgo: 40, updatedDaysAgo: 5 }),
  dbRow({ id: "dbp-ydb-b42", name: "YDB block-4-2", description: "8 storage nodes, block-4-2.", author: "system", isSystem: true, dbKind: "ydb", version: "25.2", createdDaysAgo: 90, updatedDaysAgo: 90 }),
  dbRow({ id: "dbp-ydb-managed", name: "YDB managed dedicated", description: "Dedicated 3-node managed cluster.", author: "ada", isSystem: false, dbKind: "ydb_managed", version: "managed", createdDaysAgo: 25, updatedDaysAgo: 1 }),
  dbRow({ id: "dbp-crdb-3", name: "CRDB 3-node", description: "Survives one failure.", author: "system", isSystem: true, dbKind: "cockroach", version: "24.2", createdDaysAgo: 80, updatedDaysAgo: 80 }),
  dbRow({ id: "dbp-picodata", name: "Picodata cluster", description: "3 instances, RF 2.", author: "max", isSystem: false, dbKind: "picodata", version: "25.3", createdDaysAgo: 12, updatedDaysAgo: 3 }),
  dbRow({ id: "dbp-external", name: "Prod read-replica", description: "Existing endpoint via DSN.", author: "grace", isSystem: false, dbKind: "external", version: "", external: true, createdDaysAgo: 7, updatedDaysAgo: 7 }),
];

const ACME_WL: WorkloadPresetRow[] = [
  wlRow({ id: "wlp-tpcc", name: "TPC-C", description: "OLTP order-entry mix.", author: "system", isSystem: true, protocol: "pg", stroppyVersion: "5.1.2", script: "tpcc/tx", favorite: true, createdDaysAgo: 120, updatedDaysAgo: 10 }),
  wlRow({ id: "wlp-ycsb-a", name: "YCSB-A (50/50)", description: "Update-heavy KV mix.", author: "system", isSystem: true, protocol: "pg", stroppyVersion: "5.1.2", script: "ycsb/a", createdDaysAgo: 118, updatedDaysAgo: 60 }),
  wlRow({ id: "wlp-ycsb-c", name: "YCSB-C (read-only)", description: "100% reads.", author: "system", isSystem: true, protocol: "pg", stroppyVersion: "5.1.1", script: "ycsb/c", createdDaysAgo: 118, updatedDaysAgo: 60 }),
  wlRow({ id: "wlp-pgbench", name: "pgbench rw-mix", description: "TPC-B-like quick baseline.", author: "ada", isSystem: false, protocol: "pg", stroppyVersion: "5.1.2", script: "pgbench/tpcb", createdDaysAgo: 22, updatedDaysAgo: 4 }),
  wlRow({ id: "wlp-tpch", name: "TPC-H", description: "22 OLAP queries.", author: "grace", isSystem: false, protocol: "pg", stroppyVersion: "5.1.0", script: "tpch/queries", createdDaysAgo: 30, updatedDaysAgo: 8 }),
  wlRow({ id: "wlp-ydb-ycsb", name: "YDB YCSB-B", description: "Read-mostly over YDB gRPC.", author: "max", isSystem: false, protocol: "ydb_grpc", stroppyVersion: "5.1.2", script: "ycsb/b", createdDaysAgo: 14, updatedDaysAgo: 2 }),
  wlRow({ id: "wlp-crdb-tpcc", name: "CRDB TPC-C", description: "TPC-C over CockroachDB.", author: "system", isSystem: true, protocol: "cockroach", stroppyVersion: "5.1.1", script: "tpcc/tx", createdDaysAgo: 70, updatedDaysAgo: 70 }),
];

const ACME_TP: TestPresetRow[] = [
  tpRow({ id: "tp-nightly-pg", name: "Nightly PG TPC-C", description: "PG HA + TPC-C combo.", author: "system", isSystem: true, dbKind: "postgres", protocol: "pg", stroppyVersion: "5.1.2", favorite: true, createdDaysAgo: 100, updatedDaysAgo: 9 }),
  tpRow({ id: "tp-pg-ycsb", name: "PG YCSB baseline", description: "PG single + YCSB-A.", author: "ada", isSystem: false, dbKind: "postgres", protocol: "pg", stroppyVersion: "5.1.2", createdDaysAgo: 20, updatedDaysAgo: 3 }),
  tpRow({ id: "tp-ydb-ycsb", name: "YDB YCSB", description: "YDB block-4-2 + YCSB-B.", author: "max", isSystem: false, dbKind: "ydb", protocol: "ydb_grpc", stroppyVersion: "5.1.2", createdDaysAgo: 16, updatedDaysAgo: 1 }),
  tpRow({ id: "tp-crdb-tpcc", name: "CRDB TPC-C", description: "CRDB 3-node + TPC-C.", author: "system", isSystem: true, dbKind: "cockroach", protocol: "cockroach", stroppyVersion: "5.1.1", createdDaysAgo: 65, updatedDaysAgo: 65 }),
  tpRow({ id: "tp-mysql-tpch", name: "MySQL TPC-H", description: "MySQL group + TPC-H.", author: "grace", isSystem: false, dbKind: "mysql", protocol: "mysql", stroppyVersion: "5.1.0", createdDaysAgo: 33, updatedDaysAgo: 7 }),
];

const GLOBEX_DB: DatabasePresetRow[] = [
  dbRow({ id: "g-dbp-pg-single", name: "PG single", description: "One primary.", author: "system", isSystem: true, dbKind: "postgres", version: "16", createdDaysAgo: 60, updatedDaysAgo: 60 }),
  dbRow({ id: "g-dbp-pg-ci", name: "PG CI", description: "Single node for CI smoke.", author: "ci", isSystem: false, dbKind: "postgres", version: "16", createdDaysAgo: 9, updatedDaysAgo: 1 }),
  dbRow({ id: "g-dbp-mysql", name: "MySQL single", description: "One primary.", author: "system", isSystem: true, dbKind: "mysql", version: "8.4", createdDaysAgo: 60, updatedDaysAgo: 60 }),
];
const GLOBEX_WL: WorkloadPresetRow[] = [
  wlRow({ id: "g-wlp-tpcc", name: "TPC-C", description: "OLTP order-entry mix.", author: "system", isSystem: true, protocol: "pg", stroppyVersion: "5.1.2", script: "tpcc/tx", createdDaysAgo: 60, updatedDaysAgo: 6 }),
  wlRow({ id: "g-wlp-smoke", name: "Smoke", description: "Tiny CI smoke load.", author: "ci", isSystem: false, protocol: "pg", stroppyVersion: "5.1.2", script: "ycsb/a", createdDaysAgo: 4, updatedDaysAgo: 1 }),
];
const GLOBEX_TP: TestPresetRow[] = [
  tpRow({ id: "g-tp-smoke", name: "CI smoke", description: "PG CI + Smoke.", author: "ci", isSystem: false, dbKind: "postgres", protocol: "pg", stroppyVersion: "5.1.2", createdDaysAgo: 4, updatedDaysAgo: 1 }),
];

const DB_BY_SLUG: Record<string, DatabasePresetRow[]> = { acme: ACME_DB, globex: GLOBEX_DB };
const WL_BY_SLUG: Record<string, WorkloadPresetRow[]> = { acme: ACME_WL, globex: GLOBEX_WL };
const TP_BY_SLUG: Record<string, TestPresetRow[]> = { acme: ACME_TP, globex: GLOBEX_TP };

function store<T>(map: Record<string, T[]>, slug: string): T[] {
  let list = map[slug];
  if (!list) {
    list = [];
    map[slug] = list;
  }
  return list;
}

// Shared row shape the in-memory filter/sort operate over (the common columns;
// kind-specific facets are matched per-table by the caller's predicate).
interface CommonRow {
  id: string;
  name: string;
  description: string;
  authorId: string;
  isSystem: boolean;
  isFavorite: boolean;
  createdAt: string;
  updatedAt: string;
}

function matchesCommon(r: CommonRow, q: PresetListQuery): boolean {
  if (q.search) {
    const s = q.search.toLowerCase();
    if (
      !r.name.toLowerCase().includes(s) &&
      !r.description.toLowerCase().includes(s)
    )
      return false;
  }
  // filter.favorites_only — only rows the caller has favorited.
  if (q.favoritesOnly && !r.isFavorite) return false;
  if (q.isSystem !== undefined && r.isSystem !== q.isSystem) return false;
  if (q.createdAfter && r.createdAt < q.createdAfter) return false;
  if (q.createdBefore && r.createdAt > q.createdBefore) return false;
  if (q.updatedAfter && r.updatedAt < q.updatedAfter) return false;
  if (q.updatedBefore && r.updatedAt > q.updatedBefore) return false;
  return true;
}

function comparePreset(
  a: CommonRow,
  b: CommonRow,
  sort: PresetSortField | undefined,
  desc: boolean | undefined,
): number {
  const dir = desc ? -1 : 1;
  const get = (r: CommonRow, key: string): string => {
    const v = (r as unknown as Record<string, unknown>)[key];
    return typeof v === "string" ? v : v === true ? "1" : v === false ? "0" : "";
  };
  switch (sort) {
    case "name":
      return dir * a.name.localeCompare(b.name);
    case "author_id":
      return dir * a.authorId.localeCompare(b.authorId);
    case "created_at":
      return dir * a.createdAt.localeCompare(b.createdAt);
    case "is_system":
      return dir * get(a, "isSystem").localeCompare(get(b, "isSystem"));
    case "db_kind":
      return dir * get(a, "dbKind").localeCompare(get(b, "dbKind"));
    case "protocol":
      return dir * get(a, "protocol").localeCompare(get(b, "protocol"));
    case "stroppy_version":
      return dir * get(a, "stroppyVersion").localeCompare(get(b, "stroppyVersion"));
    case "updated_at":
    default:
      return dir * a.updatedAt.localeCompare(b.updatedAt);
  }
}

const DEFAULT_LIST_SIZE = 10;

function paginate<T extends CommonRow>(
  all: T[],
  q: PresetListQuery,
  extraMatch: (r: T) => boolean,
): PresetPage<T> {
  const filtered = all
    .filter((r) => matchesCommon(r, q) && extraMatch(r))
    // default ordering = updated_at desc, matching the page default sort.
    .sort((a, b) =>
      comparePreset(a, b, q.sort ?? "updated_at", q.sort ? q.desc : true),
    );
  const size = q.pageSize && q.pageSize > 0 ? q.pageSize : DEFAULT_LIST_SIZE;
  const offset = q.pageToken ? Number.parseInt(q.pageToken, 10) || 0 : 0;
  const slice = filtered.slice(offset, offset + size);
  const nextOffset = offset + size;
  const nextPageToken = nextOffset < filtered.length ? String(nextOffset) : "";
  return { rows: slice, nextPageToken };
}

// Shared row store keyed by (kind, slug) so the Actions menu + favorite star can
// mutate the same in-memory lists the list calls read back.
type AnyRow = DatabasePresetRow | WorkloadPresetRow | TestPresetRow;

function storeFor(kind: PresetKind, slug: string): AnyRow[] {
  switch (kind) {
    case "database":
      return store(DB_BY_SLUG, slug);
    case "workload":
      return store(WL_BY_SLUG, slug);
    case "test":
      return store(TP_BY_SLUG, slug);
  }
}

let cloneSeq = 0;

// --- Provider implementation -------------------------------------------------

export const mockPresetProvider: PresetProvider = {
  async listDatabasePresets(tenantSlug, engine): Promise<DatabasePresetVM[]> {
    await delay();
    void tenantSlug;
    // Builtins (realistic, pre-seeded) first, then the "Custom · configure from
    // scratch" entry that starts from a blank skeleton the user fills entirely.
    return [...BUILTINS[engine](), blankPreset(engine)];
  },

  async listWorkloadPresets(tenantSlug): Promise<WorkloadPresetVM[]> {
    await delay();
    void tenantSlug;
    // Builtins first, then the "Custom · configure from scratch" blank entry.
    return [...workloadPresets(), blankWorkloadPreset()];
  },

  async listDatabasePresetRows(tenantSlug, query): Promise<PresetPage<DatabasePresetRow>> {
    await delay();
    return paginate(store(DB_BY_SLUG, tenantSlug), query, (r) => {
      if (query.dbKinds && query.dbKinds.length > 0) {
        if (r.dbKind === "" || !query.dbKinds.includes(r.dbKind)) return false;
      }
      return true;
    });
  },

  async listWorkloadPresetRows(tenantSlug, query): Promise<PresetPage<WorkloadPresetRow>> {
    await delay();
    return paginate(store(WL_BY_SLUG, tenantSlug), query, (r) => {
      if (query.protocols && query.protocols.length > 0) {
        if (r.protocol === "" || !query.protocols.includes(r.protocol)) return false;
      }
      if (query.stroppyVersions && query.stroppyVersions.length > 0) {
        if (!query.stroppyVersions.includes(r.stroppyVersion)) return false;
      }
      return true;
    });
  },

  async listTestPresetRows(tenantSlug, query): Promise<PresetPage<TestPresetRow>> {
    await delay();
    return paginate(store(TP_BY_SLUG, tenantSlug), query, (r) => {
      if (query.dbKinds && query.dbKinds.length > 0) {
        if (r.dbKind === "" || !query.dbKinds.includes(r.dbKind)) return false;
      }
      if (query.protocols && query.protocols.length > 0) {
        if (r.protocol === "" || !query.protocols.includes(r.protocol)) return false;
      }
      if (query.stroppyVersions && query.stroppyVersions.length > 0) {
        if (!query.stroppyVersions.includes(r.stroppyVersion)) return false;
      }
      return true;
    });
  },

  // FavoriteService Add/RemoveFavorite (mock): flip the row's is_favorite flag.
  async setPresetFavorite(tenantSlug, kind, id, favorite): Promise<void> {
    await delay();
    const row = storeFor(kind, tenantSlug).find((r) => r.id === id);
    if (row) row.isFavorite = favorite;
  },

  // Clone<Kind>Preset (mock): insert a non-system copy at the front, named after
  // the request, and return its new id.
  async clonePreset(tenantSlug, kind, id, name): Promise<string> {
    await delay();
    const list = storeFor(kind, tenantSlug);
    const src = list.find((r) => r.id === id);
    if (!src) throw new Error("preset not found");
    cloneSeq += 1;
    const newId = `${id}-copy-${cloneSeq}`;
    const now = daysAgo(0);
    const copy: AnyRow = {
      ...src,
      id: newId,
      name: name || `${src.name} (copy)`,
      isSystem: false,
      isFavorite: false,
      authorId: "you",
      createdAt: now,
      updatedAt: now,
    };
    list.unshift(copy);
    return newId;
  },

  // Delete<Kind>Preset (mock): drop the row from its tenant store.
  async deletePreset(tenantSlug, kind, id): Promise<void> {
    await delay();
    const list = storeFor(kind, tenantSlug);
    const idx = list.findIndex((r) => r.id === id);
    if (idx >= 0) list.splice(idx, 1);
  },
};
