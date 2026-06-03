// --- Enums / constants ---

export type Provider = "yandex" | "docker";
export type DatabaseKind = "postgres" | "mysql" | "mariadb" | "picodata" | "ydb" | "ydb-managed" | "cockroach" | "testing";

// Protocol is the wire format stroppy uses to talk to a database. Decoupled
// from DatabaseKind because YDB and Picodata speak more than one. See
// internal/domain/types/protocol.go for the source of truth.
export type Protocol =
  | "pg"
  | "mysql"
  | "picodata"
  | "ydb-grpc"
  | "ydb-pgwire"
  | "ydb-grpcs"
  | "cockroach"
  | "noop";

// KindProtocols mirrors the Go-side registry. First entry is the default
// when the user hasn't picked one explicitly. Single-protocol kinds don't
// need a UI picker.
export const KIND_PROTOCOLS: Record<DatabaseKind, Protocol[]> = {
  postgres: ["pg"],
  mysql: ["mysql"],
  mariadb: ["mysql"],
  picodata: ["picodata"],
  ydb: ["ydb-grpc", "ydb-pgwire"],
  "ydb-managed": ["ydb-grpcs"],
  cockroach: ["cockroach"],
  testing: ["noop", "pg"],
};

// SCRIPT_COMPAT keys (kind, protocol) and lists which scripts the wizard
// should offer. Mirrors types.ScriptCompat on the server.
export const SCRIPT_COMPAT: Record<string, string[]> = {
  "postgres:pg":      ["tpcc/procs", "tpcc/tx", "tpcb/procs", "tpcb/tx", "tpch/tx"],
  "mysql:mysql":      ["tpcc/procs", "tpcc/tx", "tpcb/procs", "tpcb/tx", "tpch/tx"],
  "mariadb:mysql":    ["tpcc/procs", "tpcc/tx", "tpcb/procs", "tpcb/tx", "tpch/tx"],
  "picodata:picodata": ["tpcc/tx", "tpcb/tx", "tpch/tx"],
  "ydb:ydb-grpc":     ["tpcc/tx", "tpcb/tx", "tpch/tx"],
  "ydb:ydb-pgwire":   ["tpcc/tx-ydb-pgwire", "tpcb/tx-ydb-pgwire"],
  "ydb-managed:ydb-grpcs": ["tpcc/tx", "tpcb/tx", "tpch/tx"],
  "cockroach:cockroach": ["tpcc/tx", "tpcb/tx", "tpch/tx"],
  "testing:noop":    ["tpcc/procs", "tpcc/tx", "tpcb/procs", "tpcb/tx", "tpch/tx", "tpcc/tx-ydb-pgwire", "tpcb/tx-ydb-pgwire"],
  "testing:pg":      ["tpcc/procs", "tpcc/tx", "tpcb/procs", "tpcb/tx", "tpch/tx", "tpcc/tx-ydb-pgwire", "tpcb/tx-ydb-pgwire"],
};

/** All supported database kinds — single source of truth for UI iterations. */
export const ALL_DB_KINDS: DatabaseKind[] = ["postgres", "mysql", "mariadb", "picodata", "ydb", "ydb-managed", "cockroach", "testing"];

export type Phase =
  | "network"
  | "machines"
  | "install_db"
  | "configure_db"
  | "install_monitor"
  | "configure_monitor"
  | "install_stroppy"
  | "run_stroppy"
  | "install_etcd"
  | "configure_etcd"
  | "install_proxy"
  | "configure_proxy"
  | "install_pgbouncer"
  | "configure_pgbouncer"
  | "teardown";

export type MachineRole =
  | "database"
  | "monitor"
  | "stroppy"
  | "etcd"
  | "proxy"
  | "pgbouncer"
  | "ydb-storage"
  | "ydb-database";

export type NodeStatusValue = "pending" | "running" | "done" | "failed" | "cancelled";

// --- Machine / Topology ---

export interface MachineSpec {
  role: MachineRole;
  count: number;
  cpus: number;
  memory_mb: number;
  disk_gb: number;
  disk_type?: string;
  secondary_disks?: SecondaryDisk[];
  placement?: PlacementSpec;
}

export interface SecondaryDisk {
  device_name: string;
  size_gb: number;
  type?: string;
}

export interface PlacementSpec {
  strategy?: "single" | "round-robin";
  zones?: string[];
}

export interface PostgresTopology {
  master: MachineSpec;
  replicas?: MachineSpec[];
  haproxy?: MachineSpec;
  pgbouncer: boolean;
  patroni: boolean;
  etcd: boolean;
  sync_replicas: number;
  master_options?: Record<string, string>;
  replica_options?: Record<string, string>;
  haproxy_options?: Record<string, string>;
  pgbouncer_options?: Record<string, string>;
  patroni_options?: Record<string, string>;
  etcd_options?: Record<string, string>;
}

export interface MySQLTopology {
  primary: MachineSpec;
  replicas?: MachineSpec[];
  proxysql?: MachineSpec;
  group_replication: boolean;
  semi_sync: boolean;
  primary_options?: Record<string, string>;
  replica_options?: Record<string, string>;
  proxysql_options?: Record<string, string>;
}

export interface PicodataTier {
  name: string;
  replication_factor: number;
  can_vote: boolean;
  count: number;
}

export interface PicodataTopology {
  instances: MachineSpec[];
  haproxy?: MachineSpec;
  replication_factor: number;
  shards: number;
  tiers?: PicodataTier[];
  instance_options?: Record<string, string>;
  haproxy_options?: Record<string, string>;
}

export interface YDBTopology {
  storage: MachineSpec;
  database?: MachineSpec;
  haproxy?: MachineSpec;
  fault_tolerance: string;
  failure_domain_type?: string;
  default_disk_type?: string;
  storage_groups?: number;
  auto_size_pdisks?: boolean;
  database_path: string;
  storage_options?: Record<string, string>;
  database_options?: Record<string, string>;
  haproxy_options?: Record<string, string>;
}

// Yandex Cloud Managed YDB topology. Mirrors types.YDBManagedTopology in
// internal/domain/types/run.go. The patched stroppy ydb driver pulls SA
// token + CA from the YC metadata service, so the only extra surface the
// user sees is `client` (the runner VM, which the terraform module attaches
// the stroppy SA to).
export type YDBManagedComputeType = "oltp" | "olap";

export interface YDBManagedAutoScale {
  min_size: number;
  max_size: number;
  cpu_utilization_percent?: number;  // default 70
}

export interface YDBManagedTopology {
  type: "serverless" | "dedicated";
  compute_type?: YDBManagedComputeType; // dedicated only — UI filter, not sent to terraform
  resource_preset_id?: string;       // dedicated only
  node_count?: number;               // dedicated, fixed_scale only (default 1)
  auto_scale?: YDBManagedAutoScale;  // dedicated, replaces node_count when set
  storage_groups?: number;           // dedicated only
  storage_type?: string;             // dedicated only
  throttling_rcus?: number;          // serverless only
  client: MachineSpec;
  database_path?: string;            // populated by terraform output at run time
  endpoint?: string;                 // populated by terraform output at run time
  terraform_output?: YDBManagedTerraformOutput;
}

// YDBManagedResourcePreset mirrors types.YDBManagedResourcePreset in Go.
// The dedicated-cluster picker — Cores + MemoryGB drive the label badges,
// ComputeType filters the list against the OLTP/OLAP toggle.
export interface YDBManagedResourcePreset {
  id: string;
  label: string;
  cores: number;
  memory_gb: number;
  compute_type: YDBManagedComputeType;
}

// Catalog mirrors types.YDBManagedResourcePresets in Go. Kept here so the
// UI doesn't need a backend round-trip just to render the picker; resync
// when YC publishes new presets.
export const YDB_MANAGED_RESOURCE_PRESETS: YDBManagedResourcePreset[] = [
  { id: "small-m8",      label: "Small M8",      cores: 4,  memory_gb: 8,   compute_type: "oltp" },
  { id: "small",         label: "Small",         cores: 4,  memory_gb: 16,  compute_type: "oltp" },
  { id: "medium",        label: "Medium",        cores: 8,  memory_gb: 32,  compute_type: "oltp" },
  { id: "medium-m64",    label: "Medium M64",    cores: 8,  memory_gb: 64,  compute_type: "oltp" },
  { id: "medium-m96",    label: "Medium M96",    cores: 8,  memory_gb: 96,  compute_type: "oltp" },
  { id: "large",         label: "Large",         cores: 12, memory_gb: 48,  compute_type: "oltp" },
  { id: "xlarge",        label: "XLarge",        cores: 16, memory_gb: 64,  compute_type: "oltp" },
  { id: "oltp-c16-m128", label: "OLTP C16 M128", cores: 16, memory_gb: 128, compute_type: "oltp" },
  { id: "olap-medium",   label: "OLAP Medium",   cores: 8,  memory_gb: 32,  compute_type: "olap" },
  { id: "olap-large",    label: "OLAP Large",    cores: 12, memory_gb: 48,  compute_type: "olap" },
];

export interface YDBManagedTerraformOutput {
  id: string;
  name: string;
  type: "serverless" | "dedicated";
  folder_id: string;
  location_id: string;
  database_path: string;
  ydb_api_endpoint: string;
  ydb_full_endpoint: string;
  document_api_endpoint: string;
  tls_enabled: boolean;
  status: string;
  created_at: string;
  labels?: Record<string, string>;
  resource_preset_id?: string;
  network_id?: string;
  subnet_ids?: string[];
  storage_groups?: number;
  storage_type_id?: string;
  throttling_rcu_limit?: number;
}

export interface CockroachTopology {
  nodes: MachineSpec;
  options?: Record<string, string>;
}

export interface PgNoopConfig {
  version?: string;
  port?: number;
  workers?: number;
}

export interface TestingTopology {
  mode: "noop-driver" | "pg-noop";
  database?: MachineSpec;
  pg_noop?: PgNoopConfig;
}

export interface DatabaseConfig {
  kind: DatabaseKind;
  version: string;
  postgres?: PostgresTopology;
  mysql?: MySQLTopology;
  mariadb?: MySQLTopology;
  picodata?: PicodataTopology;
  ydb?: YDBTopology;
  ydb_managed?: YDBManagedTopology;
  cockroach?: CockroachTopology;
  testing?: TestingTopology;
  rendered_config_overrides?: Record<string, string>;
}

export interface MonitorConfig {
  metrics_endpoint?: string;
  logs_endpoint?: string;
}

export interface StroppyConfig {
  version: string;
  protocol?: Protocol;          // unset → default for the database kind (see KIND_PROTOCOLS)
  script: string;               // e.g. "tpcc/procs", "tpcc/tx", "tpcb/procs", "tpcb/tx"
  sql?: string;                  // optional second stroppy positional arg / RunConfig.sql
  duration: string;
  k6_mode?: "duration" | "iterations";
  iterations?: number;
  quiet?: boolean;
  no_thresholds?: boolean;
  vus: number;
  pool_size?: number;
  scale_factor?: number;
  default_insert_method?: string;
  env?: Record<string, string>;  // script-specific env overrides from probe metadata
  files?: WorkloadFile[];        // run-scoped files materialized beside stroppy-config.json
  steps?: string[];             // step allowlist
  no_steps?: string[];          // step blocklist
  machine?: MachineSpec;        // stroppy runner machine spec
  config_override_json?: string; // raw stroppy protojson override
  // Deprecated — kept for backward compat with existing runs.
  workload?: string;
  vus_scale?: number;
  workers?: number;
}

export interface WorkloadFile {
  name: string;
  kind?: "sql" | string;
  content: string;
}

export interface ProbeRequest {
  script: string;
  version?: string;
  sql?: string;
  driver_type?: string;
  pool_size?: number;
  scale_factor?: number;
  env?: Record<string, string>;
  files?: WorkloadFile[];
  include_human?: boolean;
}

export interface EnvDeclaration {
  names: string[];
  default?: string;
  description: string;
}

export interface ProbeResponse {
  env_declarations?: EnvDeclaration[];
  steps?: string[];
  sql_sections?: { name: string; queries?: { name: string }[] }[];
  driver_setups?: { index: number; defaults: Record<string, unknown> }[];
  human?: string;
  human_error?: string;
}

export interface NetworkConfig {
  cidr: string;
  zone?: string;
}

// Package is a first-class entity stored in the DB.
export interface Package {
  id: string;
  name: string;
  description: string;
  db_kind: string;
  db_version: string;
  is_builtin: boolean;
  apt_packages: string[];
  pre_install: string[];
  custom_repo?: string;
  custom_repo_key?: string;
  deb_filename?: string;
  has_deb?: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface ExternalDBConfig {
  endpoint: string;
  database?: string;
  username?: string;
  password?: string;
  ssl_mode?: string;
}

export interface RunConfig {
  id: string;
  name?: string;
  description?: string;
  provider: Provider;
  network: NetworkConfig;
  machines: MachineSpec[];
  database: DatabaseConfig;
  monitor: MonitorConfig;
  stroppy: StroppyConfig;
  preset_id?: string;
  package_id?: string;
  platform_id?: string;
  machine_override?: MachineSpec;
  run_preset_id?: string;
  suite_id?: string;
  external_db?: ExternalDBConfig;
}

// --- DAG / Snapshot ---

export interface NodeStatus {
  id: string;
  status: NodeStatusValue;
  error?: string;
}

export interface SnapshotTarget {
  id: string;
  host: string;
  internal_host: string;
  role: string;
}

export interface Snapshot {
  graph: string; // JSON-encoded graph
  nodes: NodeStatus[];
  started_at?: string;
  finished_at?: string;
  state?: {
    provider?: string;
    run_config?: Record<string, unknown> | string; // object or JSON string
    targets?: SnapshotTarget[];
    effective_configs?: Record<string, Record<string, string>>;
    [key: string]: unknown;
  };
}

// --- Run Summary ---

export interface RunSummary {
  id: string;
  nodes: NodeStatus[];
  total: number;
  done: number;
  failed: number;
  pending: number;
  started_at?: string;
  finished_at?: string;
  db_kind?: string;
  provider?: string;
  script?: string;
  duration?: string;
  vus?: number;
  db_version?: string;
  node_count?: number;
  preset_id?: string;
  cancelled?: boolean;
  name?: string;
  description?: string;
  suite_id?: string;
  run_preset_id?: string;
}

// --- Run Presets ---

export interface RunPreset {
  id: string;
  name: string;
  description: string;
  db_kind: string;
  config: RunConfig;
  created_at: string;
  updated_at: string;
}

// --- Suites (v3: matrix of db_presets × workloads) ---
//
// A Suite is the cartesian product of `db_preset_ids` (database axis,
// references presets) and `items` (workload axis, each item is a
// StroppyConfig). One launch produces N*M job_runs — one per matrix cell.
// Shared infrastructure (provider, network, stroppy machine) lives on the
// Suite itself.

export interface SuiteItem {
  id: string;
  suite_id: string;
  position: number;
  name: string;
  workload: StroppyConfig;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface SuitePolicy {
  mode: "sequential" | "parallel";
  max_parallel: number;
  on_step_fail: "continue" | "stop";
  step_timeout_min: number;
}

export const DEFAULT_SUITE_POLICY: SuitePolicy = {
  mode: "sequential",
  max_parallel: 1,
  on_step_fail: "continue",
  step_timeout_min: 0,
};

export interface ComparisonConfig {
  strategy?: "" | "previous" | "first" | "fixed";
  baseline_batch_id?: string;
  threshold_pct?: number;
}

export interface SuiteRunBrief {
  run_id: string;
  suite_item_id?: string;
  db_preset_id?: string;
  position: number;
  state: string;
}

export interface SuiteBatchSummary {
  batch_id: string;
  trigger?: string;
  fire_at?: string;
  created_at: string;
  total: number;
  finished: number;
  failed: number;
  cancelled: number;
  running: number;
  queued: number;
  runs: SuiteRunBrief[];
}

export interface Suite {
  id: string;
  name: string;
  description: string;
  policy: SuitePolicy;
  comparison: ComparisonConfig;
  // Matrix axes.
  db_preset_ids: string[];
  // Items are returned inline by GET /suites/:id; managed via items endpoints.
  items?: SuiteItem[];
  // ItemCount is the workload-axis size — populated on both list and detail
  // responses so the suites table can render the "Workloads" column without
  // a per-row fetch.
  item_count?: number;
  // Shared infrastructure for every (preset × workload) cell.
  provider: Provider;
  platform_id?: string;
  network: NetworkConfig;
  stroppy_machine: Partial<MachineSpec>;
  // Scheduling.
  cron_expr?: string;
  timezone: string;
  enabled: boolean;
  next_fire_at?: string;
  last_fire_at?: string;
  last_batch_id?: string;
  concurrent_policy: "forbid" | "allow";
  catchup_mode: "skip" | "once";
  retention_runs: number;
  created_at: string;
  updated_at: string;
  batches?: SuiteBatchSummary[];
}

export interface SuiteCrossMember {
  run_id: string;
  state: string;
  db_preset_id?: string;
  preset_name?: string;
  suite_item_id?: string;
  item_name?: string;
}

export interface SuiteCrossGroup {
  group_key: string;
  group_label: string;
  members: SuiteCrossMember[];
}

export interface SuiteCrossResponse {
  suite_id: string;
  batch_id: string;
  pivot: "item" | "preset";
  groups: SuiteCrossGroup[];
}

// --- Presets ---

// Legacy format (kept for backward compatibility with TopologyDiagram).
export interface PresetsResponse {
  postgres: Record<string, PostgresTopology>;
  mysql: Record<string, MySQLTopology>;
  picodata: Record<string, PicodataTopology>;
}

// Per-tenant preset stored in the DB.
export interface Preset {
  id: string;
  name: string;
  description: string;
  db_kind: DatabaseKind;
  is_builtin: boolean;
  topology: PostgresTopology | MySQLTopology | PicodataTopology | YDBTopology | YDBManagedTopology | CockroachTopology | TestingTopology;
  created_at?: string;
}

// --- Settings ---

export interface YandexCloudSettings {
  token: string;
  cloud_id: string;
  folder_id: string;
  zone: string;
  network_id: string;
  network_name: string;
  subnet_cidr: string;
  platform_id: string;
  image_id: string;
  assign_public_ip: boolean;
  software_accelerated_network: boolean;
  ssh_user: string;
  ssh_public_key: string;
}

export interface CloudSettings {
  yandex: YandexCloudSettings;
  server_addr: string;
  binary_url: string;
}

export interface GrafanaSettings {
  url: string;
  embed_enabled: boolean;
  dashboards: Record<string, string>;
}

export interface TenantQuotas {
  allowed_db_kinds?: string[];
  allowed_providers?: string[];
  max_nodes?: number;
  max_cpus_per_node?: number;
  max_memory_mb_per_node?: number;
  max_disk_gb_per_node?: number;
  max_concurrent_runs?: number;
}

export interface ServerSettings {
  cloud: CloudSettings;
  webhooks?: Record<string, unknown>;
  quotas: TenantQuotas;
}

// --- Metrics / Compare ---

export interface MetricValue {
  name: string;
  value: number;
  unit: string;
}

export interface ComparisonRow {
  key: string;
  name: string;
  unit: string;
  avg_a: number;
  avg_b: number;
  max_a: number;
  max_b: number;
  diff_avg_pct: number;
  diff_max_pct: number;
  verdict: "better" | "worse" | "same";
}

export interface ComparisonResponse {
  run_a: string;
  run_b: string;
  start: string;
  end: string;
  metrics: ComparisonRow[];
  summary: { better: number; worse: number; same: number };
  config_a?: RunConfig;
  config_b?: RunConfig;
}

// --- Auth / Multi-tenancy ---

export interface AuthUser {
  id: string;
  username: string;
  tenant_id: string | null;
  tenant_name: string | null;
  role: "viewer" | "operator" | "owner";
  is_root: boolean;
  tenants: { id: string; tenant_name: string; role: string }[];
}

export interface Tenant {
  id: string;
  name: string;
  created_at: string;
}

export interface TenantMember {
  tenant_id: string;
  user_id: string;
  username: string;
  role: string;
  created_at: string;
}

export interface TenantAPIToken {
  id: string;
  tenant_id: string;
  name: string;
  role: string;
  created_by: string;
  expires_at: string | null;
  created_at: string;
}

// --- WebSocket messages ---

export interface WSMessage {
  type: "log" | "report" | "agent_log";
  run_id?: string;
  node_id?: string;
  payload: unknown;
}

export interface LogLine {
  run_id: string;
  phase: string;
  machine_id: string;
  line: string;
  ts: string;
}
