// TEMPORARY mock — MUST be deleted before real API wiring; do not build on this.
//
// A STATEFUL test-wizard backend so the New Run wizard is fully clickable end
// to end under VITE_MOCK=1. Like mock/runs.ts and mock/org.ts this is a mutable
// in-memory store. It mirrors the real server's Patch loop closely enough that
// the whole edit-and-re-render flow is demonstrable:
//
//   Start  → a draft (DOCKER provider by default).
//   Patch  → mutate provider/database/workload/infrastructure_plan/render_overrides,
//            then RE-COMPUTE the server-filled halves:
//              • topology_spec (component + node list)
//              • infrastructure_plan: ProviderSettings (Docker.Settings |
//                Yandex.Settings) + per-node MachinePlan (Docker.Container |
//                Yandex.Vm) — HONORING any incoming infrastructure_plan edits,
//              • render_preview: per-component RenderArtifacts (a few each:
//                e.g. postgresql.conf FILE = EDITABLE with a base_hash, an
//                install COMMAND = READ_ONLY, a vmagent config = EDITABLE) —
//                HONORING render_overrides.files (apply the edited content,
//                flip that artifact's origin to ORIGIN_USER_OVERRIDE),
//            re-validate (FieldError-shaped), flip `ready`.
//   Finish → consume the draft (start → run id, save_as_preset → preset id).
//   Probe  → believable `stroppy probe` metadata from the requested script.

import {
  type ComponentRenderVM,
  type DraftSummaryVM,
  type EngineKind,
  type FileOverrideVM,
  type FinishInput,
  type FinishResultVM,
  type InfrastructurePlanVM,
  type MachineVM,
  type MachineSpecVM,
  type PatchInput,
  type ProbeMetaVM,
  type ProviderSettingsVM,
  type RenderArtifactVM,
  type TopologyComponentVM,
  type TopologyNodeVM,
  type DraftErrorVM,
  type WizardDraftVM,
  type WizardProvider,
  type WorkloadVM,
  type DatabaseVM,
  Provider,
  Workload_Protocol,
  YdbManagedParams_Type,
  YdbParams_FaultTolerance,
  RenderArtifact_Kind,
  RenderArtifact_Origin,
  RenderArtifact_Mutability,
  defaultWorkload,
  defaultProviderSettings,
  defaultYandexVm,
  defaultDockerContainer,
  driverTypeFor,
} from "@/services/wizard";

// --- Stateful store ----------------------------------------------------------
//
// One flat draft store keyed by draft_id (drafts are tenant-agnostic in the
// mock — the slug is only validated as present). Mutated in place by patch /
// finish so the wizard's Patch-loop is genuinely demonstrable.

interface DraftState {
  id: string;
  slug: string;
  name: string;
  provider: Provider;
  database?: DatabaseVM;
  workload?: WorkloadVM;
  /** user-edited provider settings (kept across recomputes). */
  settings?: ProviderSettingsVM;
  /** user-edited per-node machine specs, keyed by node_id. */
  machineSpecs: Map<string, MachineSpecVM>;
  /** active file overrides, keyed by artifact id (RenderOverrideSet.files). */
  overrides: Map<string, FileOverrideVM>;
  testPresetId: string;
  updatedAt: string;
}

const DRAFTS = new Map<string, DraftState>();
let seq = 1;

function delay(): Promise<void> {
  return new Promise((r) => setTimeout(r, 140));
}

function nowISO(): string {
  return new Date().toISOString();
}

function newId(prefix: string): string {
  return `${prefix}-${(seq++).toString(36)}-${Date.now().toString(36)}`;
}

// --- Recompute: database + workload -> topology -> infra plan -> render --------
//
// Believable derivation of the server-side recompute. Node/component counts come
// from the chosen engine params, exactly like the real topology builder.

/** How many DB nodes the chosen engine params imply, by role. */
type NodeGroup = { role: string; engine: string; count: number; kind: string };
function engineNodes(db: DatabaseVM): NodeGroup[] {
  const e = db.params;
  switch (e.kind) {
    case "postgres": {
      const p = e.postgres;
      const out: NodeGroup[] = [
        { role: "master", engine: "postgres", count: 1, kind: "database" },
      ];
      if (p.replicas > 0) out.push({ role: "replica", engine: "postgres", count: p.replicas, kind: "replica" });
      if (p.haproxy > 0) out.push({ role: "haproxy", engine: "haproxy", count: p.haproxy, kind: "proxy" });
      if (p.patroni && p.etcd) out.push({ role: "etcd", engine: "etcd", count: Math.min(3, 1 + p.replicas), kind: "coordinator" });
      return out;
    }
    case "mysql":
    case "mariadb": {
      const p = e.kind === "mysql" ? e.mysql : e.mariadb;
      const out: NodeGroup[] = [{ role: "primary", engine: e.kind, count: 1, kind: "database" }];
      if (p.replicas > 0) out.push({ role: "replica", engine: e.kind, count: p.replicas, kind: "replica" });
      if (p.proxysql > 0) out.push({ role: "proxysql", engine: "proxysql", count: p.proxysql, kind: "proxy" });
      return out;
    }
    case "picodata": {
      const p = e.picodata;
      const out: NodeGroup[] = [{ role: "instance", engine: "picodata", count: Math.max(1, p.instances), kind: "database" }];
      if (p.haproxy > 0) out.push({ role: "haproxy", engine: "haproxy", count: p.haproxy, kind: "proxy" });
      return out;
    }
    case "ydb": {
      const p = e.ydb;
      const out: NodeGroup[] = [{ role: "storage", engine: "ydb", count: Math.max(1, p.storageNodes), kind: "database" }];
      if (p.databaseNodes > 0) out.push({ role: "database", engine: "ydb", count: p.databaseNodes, kind: "database" });
      if (p.haproxy > 0) out.push({ role: "haproxy", engine: "haproxy", count: p.haproxy, kind: "proxy" });
      return out;
    }
    case "ydbManaged":
      // Managed YDB has no self-deployed nodes — only the external endpoint.
      return [{ role: "managed", engine: "ydb", count: 0, kind: "external" }];
    case "cockroach": {
      const p = e.cockroach;
      return [{ role: "node", engine: "cockroach", count: Math.max(1, p.nodes), kind: "database" }];
    }
    case "external":
      return [{ role: "external", engine: "external", count: 0, kind: "external" }];
  }
}

interface Derived {
  components: TopologyComponentVM[];
  nodes: TopologyNodeVM[];
  infrastructurePlan: InfrastructurePlanVM;
  renderComponents: ComponentRenderVM[];
  artifacts: RenderArtifactVM[];
}

/** node placement plan: an ordered list of {compId, nodeId, role, engine, kind}. */
interface Placement {
  compId: string;
  nodeId: string;
  role: string;
  engine: string;
  kind: string;
}

function placeNodes(state: DraftState): Placement[] {
  if (!state.database) return [];
  const out: Placement[] = [];
  for (const grp of engineNodes(state.database)) {
    for (let i = 0; i < grp.count; i++) {
      const suffix = grp.count > 1 ? `-${i + 1}` : "";
      const compId = `${grp.engine}-${grp.role}${suffix}`;
      out.push({ compId, nodeId: `node-${compId}`, role: grp.role, engine: grp.engine, kind: grp.kind });
    }
  }
  if (state.workload) {
    out.push({ compId: "stroppy-agent", nodeId: "node-stroppy-agent", role: "agent", engine: "stroppy", kind: "workload" });
  }
  out.push({ compId: "monitoring", nodeId: "node-monitoring", role: "metrics", engine: "victoriametrics", kind: "monitor" });
  return out;
}

/** Build a MachinePlan spec for a node, honoring any user edit. */
function machineSpecFor(state: DraftState, p: Placement): MachineSpecVM {
  const edited = state.machineSpecs.get(p.nodeId);
  if (edited && edited.case !== undefined) {
    // Only keep the edit if it matches the active provider variant.
    if (state.provider === Provider.YANDEX && edited.case === "yandex") return edited;
    if (state.provider === Provider.DOCKER && edited.case === "docker") return edited;
  }
  return state.provider === Provider.YANDEX
    ? { case: "yandex", yandex: defaultYandexVm(p.role) }
    : { case: "docker", docker: defaultDockerContainer(p.role, p.engine) };
}

function deriveTopology(state: DraftState): Derived {
  const components: TopologyComponentVM[] = [];
  const nodes: TopologyNodeVM[] = [];
  const machines: MachineVM[] = [];
  const renderComponents: ComponentRenderVM[] = [];
  const artifacts: RenderArtifactVM[] = [];

  const places = placeNodes(state);
  if (!state.database || places.length === 0) {
    return {
      components,
      nodes,
      infrastructurePlan: {
        provider: state.provider,
        settings: state.settings ?? defaultProviderSettings(state.provider),
        machines,
      },
      renderComponents,
      artifacts,
    };
  }

  for (const p of places) {
    components.push({ id: p.compId, kind: p.kind, engine: p.engine, role: p.role, nodeId: p.nodeId });
    nodes.push({ id: p.nodeId, componentIds: [p.compId], labels: { role: p.role, engine: p.engine } });
    machines.push({ nodeId: p.nodeId, role: p.role, engine: p.engine, spec: machineSpecFor(state, p) });

    // Render artifacts per component.
    const ids: string[] = [];
    const push = (a: RenderArtifactVM) => {
      artifacts.push(applyOverride(state, a));
      ids.push(a.id);
    };

    if (p.role === "master" || p.role === "primary" || p.role === "instance" || p.role === "storage" || p.role === "node") {
      // Editable engine config (FILE).
      push(makeFile(p.compId, configFileFor(state.database.kind), configContentFor(state.database.kind)));
      // Read-only install command.
      push(makeCmd(p.compId, `${p.engine} install`, `apt-get install -y ${p.engine}-${state.database.version}`, "system-managed install step"));
    } else if (p.kind === "proxy") {
      push(makeFile(p.compId, proxyFileFor(p.engine), proxyContentFor(p.engine)));
    }
    // Every node ships a vmagent scrape config (EDITABLE) for monitoring.
    if (p.kind === "database" || p.kind === "replica" || p.kind === "workload") {
      push(makeFile(p.compId, "vmagent.yaml", vmagentConfig(p.compId)));
    }

    if (p.compId === "stroppy-agent" && state.workload) {
      push(makeFile(p.compId, "stroppy-config.json", stroppyConfigPreview(state.workload)));
      push(makeCmd(p.compId, "stroppy run", stroppyCmdPreview(state.workload), "generated from the workload spec"));
      // A runtime-only placeholder (resolved at execution).
      artifacts.push(
        applyOverride(state, {
          id: "stroppy-agent/db-endpoint",
          componentId: "stroppy-agent",
          kind: RenderArtifact_Kind.RUNTIME_VALUE,
          origin: RenderArtifact_Origin.RUNTIME,
          mutability: RenderArtifact_Mutability.RUNTIME_ONLY,
          lockReason: "resolved from infrastructure at runtime",
          title: "DB endpoint",
          content: "",
          baseHash: "",
        }),
      );
      ids.push("stroppy-agent/db-endpoint");
    }
    if (p.compId === "monitoring") {
      push(makeFile(p.compId, "victoriametrics.yaml", vmConfig()));
    }

    renderComponents.push({ componentId: p.compId, nodeId: p.nodeId, artifactIds: ids });
  }

  return {
    components,
    nodes,
    infrastructurePlan: {
      provider: state.provider,
      settings: state.settings ?? defaultProviderSettings(state.provider),
      machines,
    },
    renderComponents,
    artifacts,
  };
}

// --- Artifact factories ------------------------------------------------------

/** A stable base_hash for an artifact's rendered-default content. */
function hashOf(s: string): string {
  let h = 2166136261;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return "sha256:" + (h >>> 0).toString(16).padStart(8, "0");
}

function makeFile(compId: string, path: string, content: string): RenderArtifactVM {
  return {
    id: `${compId}/${path}`,
    componentId: compId,
    kind: RenderArtifact_Kind.FILE,
    origin: RenderArtifact_Origin.RENDERED_DEFAULT,
    mutability: RenderArtifact_Mutability.EDITABLE,
    lockReason: "",
    title: path,
    content,
    baseHash: hashOf(content),
  };
}

function makeCmd(compId: string, title: string, content: string, lock: string): RenderArtifactVM {
  return {
    id: `${compId}/${title.replace(/\s+/g, "-")}`,
    componentId: compId,
    kind: RenderArtifact_Kind.COMMAND,
    origin: RenderArtifact_Origin.SYSTEM,
    mutability: RenderArtifact_Mutability.READ_ONLY,
    lockReason: lock,
    title,
    content,
    baseHash: hashOf(content),
  };
}

/** Apply a stored FileOverride to a rendered artifact (origin → USER_OVERRIDE). */
function applyOverride(state: DraftState, a: RenderArtifactVM): RenderArtifactVM {
  const ov = state.overrides.get(a.id);
  // Only EDITABLE artifacts can carry an override; a stale base_hash (the
  // rendered default changed underneath) drops the override.
  if (!ov || a.mutability !== RenderArtifact_Mutability.EDITABLE) return a;
  if (ov.baseHash !== a.baseHash) {
    state.overrides.delete(a.id);
    return a;
  }
  return { ...a, content: ov.content, origin: RenderArtifact_Origin.USER_OVERRIDE };
}

// --- Render content templates ------------------------------------------------

function configFileFor(kind: EngineKind): string {
  switch (kind) {
    case "postgres":
      return "postgresql.conf";
    case "mysql":
    case "mariadb":
      return "my.cnf";
    case "picodata":
      return "picodata.yaml";
    case "ydb":
      return "ydb.yaml";
    case "cockroach":
      return "cockroach.flags";
    default:
      return "config.conf";
  }
}

function configContentFor(kind: EngineKind): string {
  switch (kind) {
    case "postgres":
      return "shared_buffers = 2GB\nmax_connections = 200\nwal_level = replica\ncheckpoint_timeout = 15min\n";
    case "mysql":
    case "mariadb":
      return "[mysqld]\ninnodb_buffer_pool_size = 2G\nmax_connections = 500\ninnodb_flush_log_at_trx_commit = 1\n";
    case "picodata":
      return "instance:\n  memtx:\n    memory: 2G\n  vinyl:\n    memory: 512M\n";
    case "ydb":
      return "actor_system_config:\n  scheduler:\n    resolution: 64\n  executor:\n    - name: System\n      threads: 2\n";
    case "cockroach":
      return "--cache=2GiB\n--max-sql-memory=1GiB\n";
    default:
      return "# engine config\n";
  }
}

function proxyFileFor(engine: string): string {
  return engine === "proxysql" ? "proxysql.cnf" : "haproxy.cfg";
}

function proxyContentFor(engine: string): string {
  if (engine === "proxysql") {
    return "mysql_variables=\n{\n  threads=4\n  max_connections=2048\n}\n";
  }
  return "global\n  maxconn 4096\ndefaults\n  timeout connect 5s\n  timeout client 30s\n  timeout server 30s\n";
}

function vmagentConfig(compId: string): string {
  return `scrape_configs:\n  - job_name: ${compId}\n    scrape_interval: 5s\n    static_configs:\n      - targets: ["localhost:9100"]\n`;
}

function vmConfig(): string {
  return "retentionPeriod: 1\nsearch.maxQueryDuration: 60s\npromscrape.config: /etc/vmagent/vmagent.yaml\n";
}

function stroppyConfigPreview(w: WorkloadVM): string {
  const limit = w.execution.limit.case === "duration"
    ? { duration: w.execution.limit.duration }
    : { iterations: w.execution.limit.iterations };
  return JSON.stringify(
    {
      stroppy: {
        version: w.stroppyVersion || "<default>",
        script: w.script,
        sql: w.sql || undefined,
        protocol: Workload_Protocol[w.protocol].toLowerCase(),
        execution: { vus: w.execution.vus, ...limit },
        parameters: {
          pool_size: w.parameters.poolSize,
          scale_factor: w.parameters.scaleFactor,
          env: w.parameters.env,
          steps: w.parameters.steps.length ? w.parameters.steps : undefined,
          no_steps: w.parameters.noSteps.length ? w.parameters.noSteps : undefined,
        },
      },
    },
    null,
    2,
  );
}

function stroppyCmdPreview(w: WorkloadVM): string {
  const parts = ["stroppy", "run", w.script];
  if (w.sql) parts.push(w.sql);
  parts.push(`--vus ${w.execution.vus}`);
  if (w.execution.limit.case === "duration") parts.push(`--duration ${w.execution.limit.duration}`);
  else parts.push(`--iterations ${w.execution.limit.iterations}`);
  return parts.join(" ");
}

// --- Recompute: validation ---------------------------------------------------
//
// AUTHORITATIVE VALIDATION IS SERVER-SIDE: the real flow is
// PatchTestWizard → Compute → BuildTestRun → database.Validate() (protovalidate
// rules on cloud.v1.domain.Database + the per-engine params messages) plus the
// topology builder's "params.GetX() != nil" guards. This mock MIRRORS those same
// rules so a from-scratch Custom config is rejected exactly as the backend would
// reject it — emitting FieldError-shaped DraftErrors with dotted `field` paths —
// and `ready` only flips true once the config validates. Keep this in sync with
// internal/proto/cloud/v1/domain/database.proto's validate constraints.

function validate(state: DraftState): DraftErrorVM[] {
  const errs: DraftErrorVM[] = [];
  const err = (field: string, message: string, severity: DraftErrorVM["severity"] = "error", code = "invalid") =>
    errs.push({ field, message, severity, code });

  if (!state.name.trim()) err("name", "Give the run a name.");

  if (state.provider === Provider.UNSPECIFIED) {
    err("provider", "Pick a deployment provider.");
  } else if (state.provider === Provider.YANDEX) {
    const s = state.settings;
    if (s?.case === "yandex") {
      if (!s.yandex.cloudId.trim()) err("infrastructure_plan.settings.yandex.cloud_id", "Yandex needs a cloud id.", "warning", "missing_cloud");
      if (!s.yandex.folderId.trim()) err("infrastructure_plan.settings.yandex.folder_id", "Yandex needs a folder id.", "warning", "missing_folder");
    }
  }

  // Database validation — mirrors cloud.v1.domain.Database.Validate() (the proto
  // validate rules) + the topology builder's required-params guards. A blank
  // "Custom" skeleton trips these until the user fills the required bits.
  if (!state.database) {
    err("database", "Configure the database under test.");
  } else {
    const db = state.database;
    const e = db.params;

    // Database.kind is a required enum (validate: defined_only + not_in:0). A
    // missing/unknown engine is invalid regardless of params.
    if (!db.kind) {
      err("database.kind", "Choose a database engine.", "error", "required");
    }

    if (e.kind === "external") {
      // Database.External.dsn — string min_len:1.
      if (!e.external.dsn.trim()) err("database.external.dsn", "An external database needs a DSN.", "error", "required");
    } else if (e.kind === "ydbManaged") {
      const p = e.ydbManaged;
      // YdbManagedParams.type — required enum (not_in:0).
      if (p.type === YdbManagedParams_Type.UNSPECIFIED)
        err("database.params.ydb_managed.type", "Choose serverless or dedicated.", "error", "required");
      // Dedicated needs at least one node (node_count gte:1 when fixed-scale).
      if (p.type === YdbManagedParams_Type.DEDICATED && p.nodeCount < 1)
        err("database.params.ydb_managed.node_count", "Dedicated YDB needs at least 1 node.", "error", "min");
    } else if (e.kind === "postgres") {
      const p = e.postgres;
      // No proto floor on master count (master is always 1), but the topology
      // rules still apply: Patroni needs etcd, and sync standbys can't exceed
      // the replica pool.
      if (p.patroni && !p.etcd)
        err("database.params.postgres.etcd", "Patroni HA needs etcd.", "warning", "needs_etcd");
      if (p.syncReplicas > p.replicas)
        err("database.params.postgres.sync_replicas", "Sync replicas cannot exceed total replicas.", "error", "range");
    } else if (e.kind === "mysql" || e.kind === "mariadb") {
      const p = e.kind === "mysql" ? e.mysql : e.mariadb;
      // No params have a proto floor (primary is always 1), but a semi-sync /
      // group-replication config with no replicas is a no-op — flag it.
      if (p.semiSync && p.replicas < 1)
        err("database.params.mysql.replicas", "Semi-sync needs at least 1 replica.", "warning", "noop");
      if (p.groupReplication && p.replicas < 2)
        err("database.params.mysql.replicas", "Group replication needs at least 2 replicas.", "warning", "noop");
    } else if (e.kind === "ydb") {
      // YdbParams.storage_nodes — uint32 gte:1.
      if (e.ydb.storageNodes < 1) err("database.params.ydb.storage_nodes", "At least 1 storage node.", "error", "min");
      // mirror-3-dc needs >=3 storage nodes + >=3 pdisks each.
      if (e.ydb.faultTolerance === YdbParams_FaultTolerance.MIRROR_3_DC) {
        if (e.ydb.storageNodes < 3) err("database.params.ydb.storage_nodes", "mirror-3-dc needs at least 3 storage nodes.", "error", "range");
        if (e.ydb.pdisksPerStorageNode < 3) err("database.params.ydb.pdisks_per_storage_node", "mirror-3-dc needs at least 3 pdisks per node.", "error", "range");
      }
    } else if (e.kind === "cockroach") {
      // CockroachParams.nodes — uint32 gte:1.
      if (e.cockroach.nodes < 1) err("database.params.cockroach.nodes", "At least 1 node.", "error", "min");
    } else if (e.kind === "picodata") {
      // PicodataParams.instances — uint32 gte:1.
      if (e.picodata.instances < 1) err("database.params.picodata.instances", "At least 1 instance.", "error", "min");
      if (e.picodata.replicationFactor > Math.max(1, e.picodata.instances))
        err("database.params.picodata.replication_factor", "Replication factor cannot exceed the instance count.", "error", "range");
    }

    // DatabaseParams.version — max_len:128; a missing version falls back to the
    // engine default server-side, so it's a soft warning (not for managed/external).
    if (e.kind !== "external" && e.kind !== "ydbManaged") {
      if (db.version.length > 128)
        err("database.params.version", "Engine version is too long (max 128 chars).", "error", "max_len");
      else if (!db.version.trim())
        err("database.params.version", "Pick an engine version.", "warning", "version_default");
    }
  }

  if (!state.workload) {
    err("workload", "Configure the workload.");
  } else {
    const w = state.workload;
    if (!w.script.trim()) err("workload.script", "A workload needs a script.");
    if (w.execution.vus < 1) err("workload.execution.vus", "At least 1 virtual user.");
    if (w.execution.limit.case === "duration" && !w.execution.limit.duration.trim())
      err("workload.execution.duration", "Set a run duration.");
    if (w.execution.limit.case === "iterations" && w.execution.limit.iterations < 1)
      err("workload.execution.iterations", "At least 1 iteration.");
    if (w.parameters.steps.length > 0 && w.parameters.noSteps.length > 0)
      err("workload.parameters.steps", "steps and no_steps are mutually exclusive.");
  }

  return errs;
}

function toVM(state: DraftState): WizardDraftVM {
  const derived = deriveTopology(state);
  const errors = validate(state);
  const ready = errors.every((e) => e.severity !== "error");
  return {
    id: state.id,
    name: state.name,
    provider: state.provider,
    database: state.database,
    workload: state.workload,
    topologyComponents: derived.components,
    topologyNodes: derived.nodes,
    infrastructurePlan: derived.infrastructurePlan,
    renderComponents: derived.renderComponents,
    artifacts: derived.artifacts,
    errors,
    ready,
    testPresetId: state.testPresetId,
    updatedAt: state.updatedAt,
  };
}

function toSummary(state: DraftState): DraftSummaryVM {
  return {
    id: state.id,
    name: state.name,
    engine: state.database?.kind ?? "",
    ready: validate(state).every((e) => e.severity !== "error"),
    updatedAt: state.updatedAt,
  };
}

function getDraft(slug: string, draftId: string): DraftState {
  void slug;
  const s = DRAFTS.get(draftId);
  if (!s) throw new Error(`wizard draft not found: ${draftId}`);
  return s;
}

// --- Probe -------------------------------------------------------------------

const SCRIPT_SHAPES: Record<string, { steps: string[]; env: ProbeMetaVM["env"]; sql: string[] }> = {
  tpcc: {
    steps: ["create_schema", "load_data", "warmup", "workload"],
    env: [
      { name: "WAREHOUSES", description: "Number of TPC-C warehouses", default: "10", required: true },
      { name: "TERMINALS", description: "Terminals per warehouse", default: "10", required: false },
    ],
    sql: ["schema.sql", "transactions.sql"],
  },
  tpch: {
    steps: ["create_schema", "load_data", "queries"],
    env: [{ name: "SCALE_FACTOR", description: "TPC-H scale factor", default: "1", required: true }],
    sql: ["queries.sql"],
  },
  tpcds: {
    steps: ["create_schema", "load_data", "queries"],
    env: [{ name: "SCALE_FACTOR", description: "TPC-DS scale factor", default: "1", required: true }],
    sql: ["queries.sql"],
  },
};

function probeShape(script: string) {
  const head = script.split("/")[0].toLowerCase();
  return (
    SCRIPT_SHAPES[head] ?? {
      steps: ["create_schema", "load_data", "workload"],
      env: [{ name: "ROWS", description: "Rows to generate", default: "100000", required: false }],
      sql: [],
    }
  );
}

// --- Provider implementation -------------------------------------------------

export const mockWizardProvider: WizardProvider = {
  async start(tenantSlug, name, testPresetId) {
    await delay();
    const id = newId("draft");
    const state: DraftState = {
      id,
      slug: tenantSlug,
      name: name.trim(),
      provider: Provider.DOCKER,
      machineSpecs: new Map(),
      overrides: new Map(),
      testPresetId: testPresetId ?? "",
      updatedAt: nowISO(),
    };
    DRAFTS.set(id, state);
    return toVM(state);
  },

  async get(tenantSlug, draftId) {
    await delay();
    return toVM(getDraft(tenantSlug, draftId));
  },

  async list(tenantSlug) {
    await delay();
    return [...DRAFTS.values()]
      .filter((s) => s.slug === tenantSlug)
      .sort((a, b) => b.updatedAt.localeCompare(a.updatedAt))
      .map(toSummary);
  },

  async patch(tenantSlug, draftId, input: PatchInput) {
    await delay();
    const state = getDraft(tenantSlug, draftId);

    if (input.provider !== undefined && input.provider !== state.provider) {
      state.provider = input.provider;
      // Provider switch invalidates the old settings + machine variants.
      state.settings = undefined;
      state.machineSpecs.clear();
    }
    if (input.database !== undefined) {
      state.database = input.database;
      if (!state.workload) state.workload = defaultWorkload(input.database.kind);
    }
    if (input.workload !== undefined) state.workload = input.workload;

    // HONOR infrastructure_plan edits: per-node machine specs only. Provider
    // settings (cloud/folder/zone/network/image/…) are TENANT provider defaults
    // resolved server-side — the wizard never edits them, so any incoming
    // `settings` is ignored and the server-derived defaults are kept.
    if (input.infrastructurePlan !== undefined) {
      const plan = input.infrastructurePlan;
      if (plan.provider !== state.provider) {
        state.provider = plan.provider;
        state.machineSpecs.clear();
      }
      for (const m of plan.machines) state.machineSpecs.set(m.nodeId, m.spec);
    }

    // HONOR render_overrides: ship the WHOLE set each Patch.
    if (input.renderOverrides !== undefined) {
      state.overrides = new Map(input.renderOverrides.map((o) => [o.artifactId, o]));
    }

    state.updatedAt = nowISO();
    return toVM(state);
  },

  async remove(tenantSlug, draftId) {
    await delay();
    void tenantSlug;
    DRAFTS.delete(draftId);
  },

  async finish(tenantSlug, draftId, input: FinishInput): Promise<FinishResultVM> {
    await delay();
    const state = getDraft(tenantSlug, draftId);
    const errs = validate(state);
    if (errs.some((e) => e.severity === "error")) {
      throw new Error("Draft is not ready — resolve the remaining errors first.");
    }
    const result: FinishResultVM = {
      runId: input.start ? newId("run") : "",
      presetId: input.saveAsPreset ? newId("preset") : "",
      testRunName: state.name || "Untitled run",
    };
    DRAFTS.delete(draftId);
    return result;
  },

  async probe(tenantSlug, input): Promise<ProbeMetaVM> {
    await delay();
    void tenantSlug;
    const shape = probeShape(input.script);
    return {
      steps: shape.steps,
      env: shape.env,
      sqlSections: shape.sql,
      poolSize: input.poolSize > 0 ? input.poolSize : 16,
      driverType: input.driverType || driverTypeFor("postgres"),
      human: input.includeHuman
        ? [
            `script:   ${input.script}`,
            `driver:   ${input.driverType}`,
            `pool:     ${input.poolSize > 0 ? input.poolSize : 16}`,
            `scale:    ${input.scaleFactor}`,
            `steps:    ${shape.steps.join(", ")}`,
            `env:      ${shape.env.map((e) => e.name).join(", ")}`,
          ].join("\n")
        : "",
    };
  },
};
