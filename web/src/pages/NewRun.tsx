import { useState, useEffect, useMemo, useRef, useCallback } from "react";
import { useSearchParams, useNavigate } from "react-router-dom";
import { clients } from "@/api/clients";
import { useTenantId, useTenantPath } from "@/hooks/useTenantPath";
import { placeholderId } from "@/lib/proto-helpers";
import {
  WorkloadForm,
  type K6Mode,
  type DatabaseKind,
  type Protocol,
  type Provider,
  type WorkloadFile,
  type ProbeResponse,
  type TenantQuotas,
  KIND_PROTOCOLS,
  SCRIPT_COMPAT,
  probeDriverType,
} from "@/components/WorkloadForm";
import { InfrastructureForm, PROVIDER_META } from "@/components/InfrastructureForm";
import { generateRunID } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import { TopologyDiagram } from "@/components/TopologyDiagram";
import {
  PostgresForm,
  MySQLForm,
  PicodataForm,
  YDBForm,
  YDBManagedForm,
  defaultPostgres,
  defaultMySQL,
  defaultPicodata,
  defaultYDB,
  defaultYDBManaged,
  type PostgresTopology,
  type MySQLTopology,
  type PicodataTopology,
  type YDBTopology,
  type YDBManagedTopology,
} from "@/pages/PresetDesigner";
import { JsonEditor } from "@/components/ui/json-editor";
import {
  Check,
  AlertCircle,
  Database,
  Server,
  Cpu,
  Cloud,
  Copy,
  Rocket,
  ChevronRight,
  ChevronLeft,
  ChevronDown,
  Loader2,
  Pencil,
  RotateCcw,
  FlaskConical,
  Layers,
} from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";

import { DB_COLORS } from "@/lib/db-colors";
import { platformLimits } from "@/components/ui/sliders";

import type { DatabasePreset } from "@/lib/proto/cloud/v1/catalog/database_pb";
import { Database_Kind } from "@/lib/proto/cloud/v1/catalog/database_pb";
import type { Package as PbPackage } from "@/lib/proto/cloud/v1/catalog/package_pb";
import { ConnectError } from "@connectrpc/connect";
import type { Dag } from "@/lib/proto/cloud/v1/system/dag_pb";
import {
  toDatabaseProto,
  toExternalDatabaseProto,
  toWorkloadProto,
  toTestRunProto,
  type AnyTopology,
  type ExternalDBInput,
  type WorkloadCfgInput,
} from "@/lib/runconfig-to-proto";

// --- Constants ---

const ALL_DB_KINDS: DatabaseKind[] = ["postgres", "mysql", "mariadb", "picodata", "ydb", "ydb-managed", "cockroach"];
const DB_KINDS = ALL_DB_KINDS;
const PROVIDERS: Provider[] = ["docker", "yandex"];

const DB_VERSIONS: Record<DatabaseKind, string[]> = {
  postgres: ["17", "16", "15"],
  mysql: ["8.4", "8.0"],
  mariadb: ["11.4", "10.11"],
  picodata: ["25.3"],
  ydb: ["25.2", "25.1", "24.4", "24.3"],
  "ydb-managed": ["managed"],
  cockroach: ["24.2", "24.1", "23.2"],
};

const DB_META: Record<DatabaseKind, { icon: typeof Database; label: string }> = {
  postgres:  { icon: Database, label: "PostgreSQL" },
  mysql:     { icon: Server,   label: "MySQL" },
  mariadb:   { icon: Server,   label: "MariaDB" },
  picodata:  { icon: Cpu,      label: "Picodata" },
  ydb:       { icon: Database, label: "YDB" },
  "ydb-managed": { icon: Cloud, label: "YDB Managed" },
  cockroach: { icon: Database, label: "CockroachDB" },
};

// Map proto Database.Kind enum → frontend DatabaseKind string.
function protoKindToDatabaseKind(k: Database_Kind | undefined): DatabaseKind | null {
  switch (k) {
    case Database_Kind.DATABASE_KIND_POSTGRES: return "postgres";
    case Database_Kind.DATABASE_KIND_MYSQL: return "mysql";
    case Database_Kind.DATABASE_KIND_MARIADB: return "mariadb";
    case Database_Kind.DATABASE_KIND_YDB: return "ydb";
    case Database_Kind.DATABASE_KIND_YDB_MANAGED: return "ydb-managed";
    case Database_Kind.DATABASE_KIND_COCKROACH: return "cockroach";
    case Database_Kind.DATABASE_KIND_PICODATA: return "picodata";
    default: return null;
  }
}

// Internal shim — the wizard's UI was built around the old REST `Preset`
// shape (id, name, db_kind, is_builtin, topology). We surface that shape
// over the proto DatabasePreset so the existing JSX doesn't need to change.
// Topology is reconstructed best-effort from the proto Database.variant; for
// preview purposes (TopologyDiagram thumbnails) we only need the high-signal
// node counts, so partial coverage is fine.
interface UiPreset {
  id: string;
  name: string;
  description: string;
  db_kind: DatabaseKind;
  is_builtin: boolean;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  topology: any;
  proto: DatabasePreset;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function protoPresetToUi(p: DatabasePreset): UiPreset | null {
  const kind = protoKindToDatabaseKind(p.database?.kind);
  if (!kind) return null;
  const variant = p.database?.variant;
  // Best-effort topology reconstruction from proto Shape (just enough for
  // the TopologyDiagram preview tile + the inline editor's initial state).
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let topology: any = {};
  if (variant?.case === "postgres") {
    const s = variant.value.shape;
    topology = {
      master: { role: "database", count: 1, cpus: 2, memory_mb: 4096, disk_gb: 50 },
      replicas: s?.replicas
        ? [{ role: "database", count: s.replicas, cpus: 2, memory_mb: 4096, disk_gb: 50 }]
        : [],
      pgbouncer: !!s?.pgbouncerColocated,
      patroni: !!s?.patroni,
      etcd: !!s?.etcdColocated,
      sync_replicas: s?.syncReplicas ?? 0,
      ...(s?.haproxyDedicated ? { haproxy: { role: "proxy", count: 1, cpus: 2, memory_mb: 2048, disk_gb: 20 } } : {}),
    } as PostgresTopology;
  } else if (variant?.case === "mysql" || variant?.case === "mariadb") {
    const s = variant.value.shape;
    topology = {
      primary: { role: "database", count: 1, cpus: 2, memory_mb: 4096, disk_gb: 50 },
      replicas: s?.replicas
        ? [{ role: "database", count: s.replicas, cpus: 2, memory_mb: 4096, disk_gb: 50 }]
        : [],
      group_replication: !!s?.groupReplication,
      semi_sync: !!s?.semiSync,
      ...(s?.proxysqlDedicated ? { proxysql: { role: "proxy", count: 1, cpus: 2, memory_mb: 2048, disk_gb: 20 } } : {}),
    } as MySQLTopology;
  } else if (variant?.case === "picodata") {
    const s = variant.value.shape;
    topology = {
      instances: [{ role: "database", count: s?.nodes ?? 1, cpus: 2, memory_mb: 4096, disk_gb: 50 }],
      replication_factor: s?.replicationFactor ?? 1,
      shards: s?.shards ?? 0,
      tiers: s?.tiers?.map((t) => ({
        name: t.name, replication_factor: t.replicationFactor, can_vote: t.canVote, count: t.count,
      })) ?? [],
      ...(s?.haproxyDedicated ? { haproxy: { role: "proxy", count: 1, cpus: 2, memory_mb: 2048, disk_gb: 20 } } : {}),
    } as PicodataTopology;
  } else if (variant?.case === "ydb") {
    const s = variant.value.shape;
    topology = {
      storage: { role: "ydb-storage", count: s?.storageNodes ?? 1, cpus: 2, memory_mb: 4096, disk_gb: 50 },
      ...(s?.databaseNodes ? { database: { role: "ydb-database", count: s.databaseNodes, cpus: 2, memory_mb: 4096, disk_gb: 50 } } : {}),
      fault_tolerance: String(s?.faultTolerance ?? ""),
      database_path: s?.databasePath ?? "",
      storage_groups: s?.storageGroups ?? 0,
      auto_size_pdisks: !!s?.autoSizePdisks,
      ...(s?.haproxyDedicated ? { haproxy: { role: "proxy", count: 1, cpus: 2, memory_mb: 2048, disk_gb: 20 } } : {}),
    } as YDBTopology;
  }
  return {
    id: p.id?.value ?? "",
    name: p.identity?.name ?? "(unnamed)",
    description: p.identity?.description ?? "",
    db_kind: kind,
    is_builtin: !p.tenantId, // builtins have no tenant binding
    topology,
    proto: p,
  };
}

interface UiPackage {
  id: string;
  name: string;
  has_deb: boolean;
  db_kind: DatabaseKind | null;
}

function protoPackageToUi(p: PbPackage): UiPackage {
  return {
    id: p.id?.value ?? "",
    name: p.identity?.name ?? "(unnamed)",
    has_deb: p.source?.source.case === "debBlob",
    db_kind: protoKindToDatabaseKind(p.dbKind),
  };
}

// RunConfig is the wizard's internal state shape — preserved from main so
// the UI code below doesn't need to change. It is rendered as JSON in the
// summary panel; on launch we extract the bits the proto TestRun cares
// about (preset id + tenant scope), see toTestRunPayload().
interface RunConfig {
  id: string;
  name?: string;
  description?: string;
  provider: Provider;
  network: { cidr: string };
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  machines: any[];
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  database: any;
  monitor: Record<string, never>;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  stroppy: any;
  preset_id?: string;
  package_id?: string;
  platform_id?: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  external_db?: any;
}

function redactRunConfigForDisplay(cfg: RunConfig): RunConfig {
  if (!cfg.stroppy.files?.length) return cfg;
  return {
    ...cfg,
    stroppy: {
      ...cfg.stroppy,
      files: cfg.stroppy.files.map((f: WorkloadFile) => ({
        ...f,
        content: `<${f.content.length} chars redacted>`,
      })),
    },
  };
}

const STEPS = [
  { key: "infra", label: "Infrastructure" },
  { key: "database", label: "Database" },
  { key: "stroppy", label: "Workload" },
  { key: "review", label: "Review & Launch" },
];

// --- Main ---

export function NewRun() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const tid = useTenantId();
  const tPath = useTenantPath();

  // Rerun config from sessionStorage (set by RunDetail's Rerun button). The
  // shape mirrors what we used to serialise on the old REST path; we read
  // optional fields defensively since the new TestRun proto path may set
  // them slightly differently.
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const rerunConfig = useMemo<any | null>(() => {
    try {
      const raw = sessionStorage.getItem("rerun_config");
      if (raw) {
        sessionStorage.removeItem("rerun_config");
        return JSON.parse(raw);
      }
    } catch { /* ignore */ }
    return null;
  }, []);

  const rc = rerunConfig;
  const rcS = rc?.stroppy;
  const rcSM = rc?.stroppy?.machine;

  const [step, setStep] = useState(0);

  const [runName, setRunName] = useState<string>(rc?.name || "");
  const [runDescription, setRunDescription] = useState<string>(rc?.description || "");

  const [externalEnabled, setExternalEnabled] = useState<boolean>(!!rc?.external_db);
  const [externalEndpoint, setExternalEndpoint] = useState<string>(rc?.external_db?.endpoint || "");
  const [externalDatabase, setExternalDatabase] = useState<string>(rc?.external_db?.database || "");
  const [externalUsername, setExternalUsername] = useState<string>(rc?.external_db?.username || "");
  const [externalPassword, setExternalPassword] = useState<string>(rc?.external_db?.password || "");
  const [externalSSLMode, setExternalSSLMode] = useState<string>(rc?.external_db?.ssl_mode || "");

  const initialKind = (rc?.database?.kind as DatabaseKind) || (searchParams.get("kind") as DatabaseKind) || "postgres";
  const [allPresets, setAllPresets] = useState<UiPreset[]>([]);
  const [kind, setKind] = useState<DatabaseKind>(initialKind);
  const [selectedPresetId, setSelectedPresetId] = useState<string>(rc?.preset_id || searchParams.get("preset_id") || "");
  const [topologyEdits, setTopologyEdits] = useState<Record<string, unknown>>({});
  const [provider, setProvider] = useState<Provider>(rc?.provider || "docker");
  const [platformId, setPlatformId] = useState<string>(rc?.platform_id || "standard-v3");
  const [version, setVersion] = useState<string>(rc?.database?.version || DB_VERSIONS[kind][0]);
  const [protocol, setProtocol] = useState<Protocol>(
    (rcS?.protocol as Protocol) || KIND_PROTOCOLS[initialKind][0],
  );
  const [script, setScript] = useState<string>(rcS?.script || rcS?.workload || "tpcc/procs");
  const [sql, setSql] = useState<string>(rcS?.sql || "");
  const [stroppyEnv, setStroppyEnv] = useState<Record<string, string>>({ STROPPY_ERROR_MODE: "", ...(rcS?.env || {}) });
  const [workloadFiles, setWorkloadFiles] = useState<WorkloadFile[]>(rcS?.files || []);
  const [duration, setDuration] = useState<string>(rcS?.duration || "5m");
  const [k6Mode, setK6Mode] = useState<K6Mode>((rcS?.k6_mode as K6Mode) || (rcS?.iterations ? "iterations" : "duration"));
  const [iterations, setIterations] = useState<number>(rcS?.iterations || 1);
  const [quiet, setQuiet] = useState<boolean>(rcS?.quiet ?? true);
  const [noThresholds, setNoThresholds] = useState<boolean>(rcS?.no_thresholds || false);
  const [defaultInsertMethod, setDefaultInsertMethod] = useState<string>(rcS?.default_insert_method || "native");
  const [vus, setVus] = useState<number>(rcS?.vus || rcS?.vus_scale || 100);
  const [poolSize, setPoolSize] = useState<number>(rcS?.pool_size || 150);
  const [scaleFactor, setScaleFactor] = useState<number>(rcS?.scale_factor || 500);
  const [stroppyVersion, setStroppyVersion] = useState<string>(rcS?.version || "");
  const [stroppyVersions, setStroppyVersions] = useState<string[]>(rcS?.version ? [rcS.version] : []);
  const [versionsLoading, setVersionsLoading] = useState<boolean>(false);
  const versionsLoaded = useRef(false);
  const [probeData, setProbeData] = useState<ProbeResponse | null>(null);
  const [workloadProbeOK, setWorkloadProbeOK] = useState<boolean>(false);
  const [availableSteps, setAvailableSteps] = useState<string[]>([]);
  const [selectedSteps, setSelectedSteps] = useState<string[]>(rcS?.steps || []);
  const [noSteps, setNoSteps] = useState<string[]>(rcS?.no_steps || []);
  const [stroppyCpus, setStroppyCpus] = useState<number>(rcSM?.cpus || 32);
  const [stroppyMemory, setStroppyMemory] = useState<number>(rcSM?.memory_mb || 65536);
  const [stroppyDisk, setStroppyDisk] = useState<number>(rcSM?.disk_gb || 100);
  const [stroppyDiskType, setStroppyDiskType] = useState<string>(rcSM?.disk_type || "network-ssd");
  const [packageId, setPackageId] = useState<string>(rc?.package_id || "");
  const [availablePackages, setAvailablePackages] = useState<UiPackage[]>([]);
  // Tenant quotas — surfaced from the QuotaService (ops/quota_pb). Resource
  // IDs are free-form strings; we decode the subset the wizard understands.
  // The Settings proto only carries Yandex Cloud knobs (PART_YANDEX_CLOUD),
  // so quota gating lives in QuotaService, not SettingsService.
  const [quotas, setQuotas] = useState<TenantQuotas>({});
  useEffect(() => {
    if (!tid) return;
    clients.quota.getQuotas({ tenantId: { value: tid }, sources: [], resourceIdPrefix: "" })
      .then((resp) => {
        const next: TenantQuotas = {};
        for (const q of resp.quotas ?? []) {
          const limit = Number(q.limit);
          switch (q.resourceId) {
            case "runs.concurrent":
              if (limit > 0) next.max_concurrent_runs = limit;
              break;
            case "nodes.max":
              if (limit > 0) next.max_nodes = limit;
              break;
            case "node.cpus.max":
              if (limit > 0) next.max_cpus_per_node = limit;
              break;
            case "node.memory_mb.max":
              if (limit > 0) next.max_memory_mb_per_node = limit;
              break;
            case "node.disk_gb.max":
              if (limit > 0) next.max_disk_gb_per_node = limit;
              break;
            case "db.kinds.allowed":
              // scope holds CSV of allowed kinds, e.g. "postgres,mysql".
              next.allowed_db_kinds = (q.scope || "").split(",").map((s) => s.trim()).filter(Boolean);
              break;
            case "providers.allowed":
              next.allowed_providers = (q.scope || "").split(",").map((s) => s.trim()).filter(Boolean);
              break;
            default: break;
          }
        }
        setQuotas(next);
      })
      .catch(() => setQuotas({}));
  }, [tid]);

  const allowedKinds = useMemo(() =>
    quotas.allowed_db_kinds?.length ? DB_KINDS.filter((k) => quotas.allowed_db_kinds!.includes(k)) : DB_KINDS,
    [quotas]);
  const allowedProviders = useMemo(() =>
    quotas.allowed_providers?.length ? PROVIDERS.filter((p) => quotas.allowed_providers!.includes(p)) : PROVIDERS,
    [quotas]);

  const [submitting, setSubmitting] = useState<boolean>(false);
  // DryRun + previewStroppyConfig results. dryRunResult holds the DAG
  // returned by TestRunService.DryRunTestRun (graph.nodes feeds the
  // execution-plan tree on step 3). stroppyConfigDraft is seeded from
  // StroppyService.PreviewStroppyConfig when the user reaches step 2/3.
  interface DryRunNodeUi { id: string; type: string; deps?: string[] }
  const [dryRunResult, setDryRunResult] = useState<{ nodes: DryRunNodeUi[]; effective_config?: Record<string, Record<string, string>> } | null>(null);
  const [dryRunLoading, setDryRunLoading] = useState<boolean>(false);
  const [resolvedConfig, setResolvedConfig] = useState<RunConfig | null>(null);
  const [dbConfigDraft, setDbConfigDraft] = useState<string | null>(null);
  const [stroppyConfigDraft, setStroppyConfigDraft] = useState<string | null>(rcS?.config_override_json || null);
  const [stroppyConfigPristine, setStroppyConfigPristine] = useState<string | null>(null);
  const [stroppyConfigUserEdited, setStroppyConfigUserEdited] = useState<boolean>(!!rcS?.config_override_json);
  const [renderedConfigsPristine] = useState<Record<string, string>>({});
  const [renderedConfigDrafts, setRenderedConfigDrafts] = useState<Record<string, string>>({});
  const [validationResult, setValidationResult] = useState<{ ok: boolean; message: string } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState<boolean>(false);

  // Load presets (proto).
  const refreshPresets = useCallback(() => {
    if (!tid) return;
    clients.databasePreset.listDatabasePresets({ value: tid })
      .then((resp) => {
        const ui: UiPreset[] = [];
        for (const p of resp.databasePresets ?? []) {
          const u = protoPresetToUi(p);
          if (u) ui.push(u);
        }
        setAllPresets(ui);
      })
      .catch(() => {});
  }, [tid]);

  useEffect(() => { refreshPresets(); }, [refreshPresets]);

  // Load stroppy versions (proto). Pin the rerun version when present and
  // not in the list so the selector keeps showing it.
  useEffect(() => {
    versionsLoaded.current = true;
    setVersionsLoading(true);
    clients.stroppy.listStroppyVersions({})
      .then((r) => {
        const versions = (r.versions ?? []).map((v) => v.tag);
        if (versions.length === 0) return;
        setStroppyVersions((prev) => {
          const pinned = rcS?.version && !versions.includes(rcS.version) ? [rcS.version] : prev.filter((v) => !versions.includes(v));
          return [...pinned, ...versions];
        });
        if (!rcS?.version) setStroppyVersion(versions[0]);
      })
      .catch(() => {})
      .finally(() => setVersionsLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    // Reconcile dependent state when kind / allPresets change.
    const matching = allPresets.filter((p) => p.db_kind === kind);
    if (matching.length > 0 && !matching.find((p) => p.id === selectedPresetId)) {
      setSelectedPresetId(matching[0].id);
    }
    if (!DB_VERSIONS[kind].includes(version)) {
      setVersion(DB_VERSIONS[kind][0]);
    }
    if (!KIND_PROTOCOLS[kind].includes(protocol)) {
      setProtocol(KIND_PROTOCOLS[kind][0]);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [kind, allPresets]);

  useEffect(() => {
    const compat = SCRIPT_COMPAT[`${kind}:${protocol}`] || [];
    const known = Object.values(SCRIPT_COMPAT).some((scripts) => scripts.includes(script));
    if (known && compat.length > 0 && !compat.includes(script)) {
      setScript(compat[0]);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [kind, protocol]);

  // Load packages filtered by db_kind. The proto request only accepts kind
  // (no version filter) — frontend version filtering on UiPackage if needed.
  useEffect(() => {
    if (!tid) return;
    const protoKind = (() => {
      switch (kind) {
        case "postgres": return Database_Kind.DATABASE_KIND_POSTGRES;
        case "mysql": return Database_Kind.DATABASE_KIND_MYSQL;
        case "mariadb": return Database_Kind.DATABASE_KIND_MARIADB;
        case "ydb": return Database_Kind.DATABASE_KIND_YDB;
        case "ydb-managed": return Database_Kind.DATABASE_KIND_YDB_MANAGED;
        case "cockroach": return Database_Kind.DATABASE_KIND_COCKROACH;
        case "picodata": return Database_Kind.DATABASE_KIND_PICODATA;
      }
    })();
    clients.package.listPackages({ tenantId: { value: tid }, dbKind: protoKind })
      .then((resp) => setAvailablePackages((resp.packages ?? []).map(protoPackageToUi)))
      .catch(() => setAvailablePackages([]));
  }, [tid, kind, version]);

  const presetsForKind = useMemo(
    () => allPresets.filter((p) => p.db_kind === kind),
    [allPresets, kind]
  );

  const selectedPreset = useMemo(
    () => presetsForKind.find((p) => p.id === selectedPresetId),
    [presetsForKind, selectedPresetId]
  );

  const runIDRef = useRef(generateRunID());

  const config = useMemo((): RunConfig => {
    const id = runIDRef.current;
    const cfg: RunConfig = {
      id, provider,
      network: { cidr: "10.0.0.0/24" },
      machines: [],
      database: { kind, version },
      monitor: {},
      stroppy: {
        version: stroppyVersion,
        protocol,
        script,
        ...(sql.trim() ? { sql: sql.trim() } : {}),
        duration,
        k6_mode: k6Mode,
        ...(k6Mode === "iterations" ? { iterations } : {}),
        quiet,
        no_thresholds: noThresholds,
        vus,
        pool_size: poolSize,
        scale_factor: scaleFactor,
        default_insert_method: defaultInsertMethod,
        ...(Object.keys(stroppyEnv).length > 0 ? { env: stroppyEnv } : {}),
        ...(workloadFiles.length > 0 ? { files: workloadFiles } : {}),
        ...(selectedSteps.length > 0 ? { steps: selectedSteps } : {}),
        ...(noSteps.length > 0 ? { no_steps: noSteps } : {}),
        ...(provider === "yandex" ? { machine: { role: "stroppy" as const, count: 1, cpus: stroppyCpus, memory_mb: stroppyMemory, disk_gb: stroppyDisk, disk_type: stroppyDiskType } } : {}),
      },
    };
    const edited = topologyEdits[kind] as Record<string, unknown> | undefined;
    if (!externalEnabled && edited) {
      const dbAny = cfg.database as unknown as Record<string, unknown>;
      const subKey = kind === "ydb-managed" ? "ydb_managed" : kind;
      dbAny[subKey] = edited;
    } else if (!externalEnabled && selectedPresetId) {
      cfg.preset_id = selectedPresetId;
    }
    if (packageId) cfg.package_id = packageId;
    if (provider === "yandex") cfg.platform_id = platformId;
    if (runName.trim()) cfg.name = runName.trim();
    if (runDescription.trim()) cfg.description = runDescription.trim();
    if (externalEnabled && externalEndpoint.trim()) {
      cfg.external_db = {
        endpoint: externalEndpoint.trim(),
        ...(externalDatabase.trim() ? { database: externalDatabase.trim() } : {}),
        ...(externalUsername.trim() ? { username: externalUsername.trim() } : {}),
        ...(externalPassword ? { password: externalPassword } : {}),
        ...(externalSSLMode.trim() ? { ssl_mode: externalSSLMode.trim() } : {}),
      };
    }
    return cfg;
  }, [kind, protocol, selectedPresetId, provider, platformId, version, script, sql, duration, k6Mode, iterations, quiet, noThresholds, vus, poolSize, scaleFactor, defaultInsertMethod, packageId, stroppyEnv, workloadFiles, selectedSteps, noSteps, stroppyCpus, stroppyMemory, stroppyDisk, stroppyDiskType, stroppyVersion, runName, runDescription, externalEnabled, externalEndpoint, externalDatabase, externalUsername, externalPassword, externalSSLMode, topologyEdits]);

  const configJSON = useMemo(() => JSON.stringify(redactRunConfigForDisplay(config), null, 2), [config]);

  // ─── Proto builders (memoised on wizard state) ─────────────────────

  const buildWorkloadCfg = useCallback((): WorkloadCfgInput => ({
    protocol,
    dbKind: kind,
    script,
    sql: sql.trim() || undefined,
    duration,
    k6Mode,
    iterations,
    quiet,
    noThresholds,
    vus,
    poolSize,
    scaleFactor,
    defaultInsertMethod,
    env: stroppyEnv,
    steps: selectedSteps,
    noSteps,
    files: workloadFiles,
    configOverrideJson: stroppyConfigUserEdited && stroppyConfigDraft ? stroppyConfigDraft : undefined,
  }), [protocol, kind, script, sql, duration, k6Mode, iterations, quiet, noThresholds, vus, poolSize, scaleFactor, defaultInsertMethod, stroppyEnv, selectedSteps, noSteps, workloadFiles, stroppyConfigUserEdited, stroppyConfigDraft]);

  const buildExternalInput = useCallback((): ExternalDBInput => ({
    endpoint: externalEndpoint.trim(),
    database: externalDatabase.trim() || undefined,
    username: externalUsername.trim() || undefined,
    password: externalPassword || undefined,
    ssl_mode: externalSSLMode.trim() || undefined,
  }), [externalEndpoint, externalDatabase, externalUsername, externalPassword, externalSSLMode]);

  // Resolve the database proto / preset id pair the wizard wants to submit.
  // Returns null when the user picked a preset (use presetId path).
  const resolveDatabaseChoice = useCallback((): { inlineDatabase?: ReturnType<typeof toDatabaseProto>; databasePresetId?: string } => {
    if (externalEnabled) {
      return { inlineDatabase: toExternalDatabaseProto(kind, buildExternalInput()) };
    }
    const edited = topologyEdits[kind] as AnyTopology | undefined;
    if (edited) {
      return { inlineDatabase: toDatabaseProto(kind, edited) };
    }
    if (selectedPresetId) {
      return { databasePresetId: selectedPresetId };
    }
    return {};
  }, [externalEnabled, buildExternalInput, kind, topologyEdits, selectedPresetId]);

  // ─── previewStroppyConfig — populates the JSON draft on step 2/3 ───

  const previewSeq = useRef(0);
  useEffect(() => {
    if (step < 2) return; // only when reaching workload / review.
    if (stroppyConfigUserEdited) return; // don't overwrite user edits.
    const seq = ++previewSeq.current;
    clients.stroppy.previewStroppyConfig({
      tenantId: { value: tid! },
      stroppyVersion: stroppyVersion || "",
      script,
      sql: sql.trim(),
      driverType: probeDriverType(protocol, kind),
      poolSize,
      scaleFactor,
      env: stroppyEnv,
      files: workloadFiles.map((f) => ({ name: f.name, content: f.content })),
      configOverrideJson: "",
    }).then((resp) => {
      if (seq !== previewSeq.current) return;
      const json = resp.stroppyConfigJson || "";
      setStroppyConfigDraft(json);
      setStroppyConfigPristine(json);
    }).catch(() => { /* preview is best-effort */ });
  }, [step, stroppyConfigUserEdited, tid, stroppyVersion, script, sql, protocol, kind, poolSize, scaleFactor, stroppyEnv, workloadFiles]);

  // ─── DryRun — populates dryRunResult + validationResult on step 3 ───

  const runDryRun = useCallback(async () => {
    if (!tid) return;
    const dbChoice = resolveDatabaseChoice();
    if (!dbChoice.inlineDatabase && !dbChoice.databasePresetId) {
      setValidationResult({ ok: false, message: "Pick a database preset, edit a topology, or enable BYOD before validating." });
      return;
    }
    const testRun = toTestRunProto({
      tenantID: tid,
      name: runName.trim(),
      description: runDescription.trim(),
      databasePresetId: dbChoice.databasePresetId,
      inlineDatabase: dbChoice.inlineDatabase,
      inlineWorkload: toWorkloadProto(buildWorkloadCfg()),
    });
    setDryRunLoading(true);
    setValidationResult(null);
    try {
      const dag: Dag = await clients.testRun.dryRunTestRun(testRun);
      const nodes = (dag.graph?.nodes ?? []).map((n) => ({ id: n.id, type: n.type, deps: n.deps ?? [] }));
      setDryRunResult({ nodes });
      setResolvedConfig(config);
      setValidationResult({ ok: true, message: "Configuration is valid." });
    } catch (err) {
      const msg = err instanceof ConnectError
        ? `${err.rawMessage || err.message} (${err.code})`
        : err instanceof Error ? err.message : "DryRun failed";
      setValidationResult({ ok: false, message: msg });
      setDryRunResult(null);
    } finally {
      setDryRunLoading(false);
    }
  }, [tid, resolveDatabaseChoice, runName, runDescription, buildWorkloadCfg, config]);

  // Auto-run dry-run when entering step 3, debounced on the key inputs.
  useEffect(() => {
    if (step !== 3) return;
    void runDryRun();
  }, [step, runDryRun]);

  // ─── Save-as-Topology-Preset — builds a real Database proto. ───

  const handleSaveAsTopologyPreset = useCallback(async (name: string, description: string) => {
    if (!tid) throw new Error("No tenant");
    const edited = topologyEdits[kind] as AnyTopology | undefined;
    const presetSrc = edited ?? (selectedPreset?.topology as AnyTopology | undefined);
    if (!presetSrc) throw new Error("No topology to save — edit the current topology or pick a base preset first.");
    const database = toDatabaseProto(kind, presetSrc);
    await clients.databasePreset.createDatabasePreset({
      preset: {
        id: placeholderId(),
        tenantId: { value: tid },
        identity: { name: name.trim(), description: description.trim(), label: [] },
        database,
      } as never,
    });
    refreshPresets();
  }, [tid, topologyEdits, kind, selectedPreset, refreshPresets]);

  // ─── Save-as-Run-Preset — builds inline Workload proto + reuses DB. ───

  const handleSaveAsPreset = useCallback(async (name: string, description: string, alsoLaunch: boolean) => {
    if (!tid) throw new Error("No tenant");
    const dbChoice = resolveDatabaseChoice();
    if (!dbChoice.inlineDatabase && !dbChoice.databasePresetId) {
      throw new Error("Pick a database preset, edit a topology, or enable BYOD before saving the run template.");
    }
    // We always save an inline workload — captures all wizard state and
    // avoids the legacy "must have a workload preset first" requirement.
    const workload = toWorkloadProto(buildWorkloadCfg());
    await clients.template.createTestRunTemplate({
      template: {
        id: placeholderId(),
        tenantId: { value: tid },
        identity: { name: name.trim(), description: description.trim(), label: [] },
        database: dbChoice.databasePresetId
          ? { databaseVariant: { case: "databasePresetId", value: { value: dbChoice.databasePresetId } } }
          : { databaseVariant: { case: "database", value: dbChoice.inlineDatabase! } },
        workload: {
          workloadVariant: { case: "workload", value: workload },
        },
      } as never,
    });
    if (alsoLaunch) {
      void handleSubmit();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tid, resolveDatabaseChoice, buildWorkloadCfg]);

  // ─── handleSubmit — launches the run via CreateTestRun + LaunchTestRun ───

  const handleSubmit = useCallback(async () => {
    if (!tid) { setError("No tenant"); return; }
    if (config.stroppy.k6_mode !== "iterations" && !config.stroppy.duration.trim()) { setError("Duration is required"); return; }
    if (config.stroppy.k6_mode === "iterations" && (config.stroppy.iterations || 0) < 1) { setError("Iterations must be at least 1"); return; }
    const dbChoice = resolveDatabaseChoice();
    if (!dbChoice.inlineDatabase && !dbChoice.databasePresetId) {
      setError(externalEnabled
        ? "Bring-your-own-database requires an endpoint."
        : "Pick a database preset before launching.");
      return;
    }
    const testRun = toTestRunProto({
      tenantID: tid,
      name: runName.trim(),
      description: runDescription.trim(),
      databasePresetId: dbChoice.databasePresetId,
      inlineDatabase: dbChoice.inlineDatabase,
      inlineWorkload: toWorkloadProto(buildWorkloadCfg()),
    });
    setSubmitting(true); setError(null);
    try {
      const created = await clients.testRun.createTestRun({ testRun });
      const createdId = created.id?.value;
      if (!createdId) throw new Error("No run ID returned from createTestRun");
      await clients.testRun.launchTestRun({ value: createdId });
      navigate(tPath(`runs/${createdId}`));
    } catch (err) {
      const msg = err instanceof ConnectError
        ? `${err.rawMessage || err.message} (${err.code})`
        : err instanceof Error ? err.message : "Failed to start run";
      setError(msg);
      setSubmitting(false);
    }
  }, [tid, config, navigate, tPath, resolveDatabaseChoice, externalEnabled, runName, runDescription, buildWorkloadCfg]);

  function handleCopy() {
    navigator.clipboard.writeText(configJSON);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }

  if (!tid) {
    return (
      <div className="p-6 text-sm text-muted-foreground">
        No tenant selected. Please select a tenant first.
      </div>
    );
  }

  const dbMeta = DB_META[kind];
  const dbColor = DB_COLORS[kind];
  const DbIcon = dbMeta.icon;
  const ProvIcon = PROVIDER_META[provider].icon;

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Step indicator */}
      <div className="shrink-0 border-b border-zinc-800 bg-[#070707] px-5 py-2.5">
        <div className="flex items-center gap-1">
          {STEPS.map((s, i) => (
            <div key={s.key} className="flex items-center gap-1">
              {i > 0 && <ChevronRight className="w-3 h-3 text-zinc-700" />}
              <button
                type="button"
                onClick={() => setStep(i)}
                className={`px-3 py-1 text-xs font-mono transition-all cursor-pointer ${
                  i === step
                    ? "text-primary border border-primary/30 bg-primary/[0.06]"
                    : i < step
                      ? "text-zinc-400 hover:text-zinc-300"
                      : "text-zinc-600"
                }`}
              >
                <span className="text-zinc-600 mr-1.5">{i + 1}.</span>
                {s.label}
              </button>
            </div>
          ))}
        </div>
      </div>

      {/* Main: step content left | sidebar right */}
      <div className="flex-1 min-h-0 flex overflow-hidden">
        {/* Left — step content */}
        <div className="flex-1 min-w-0 overflow-y-auto p-5">
          {step === 0 && (
            <InfrastructureForm
              provider={provider} setProvider={setProvider}
              platformId={platformId} setPlatformId={setPlatformId}
              providers={allowedProviders}
              name={runName} setName={setRunName}
              description={runDescription} setDescription={setRunDescription}
            />
          )}
          {step === 1 && (
            <StepDatabase
              kind={kind} setKind={(nextKind) => {
                setKind(nextKind);
                if (!KIND_PROTOCOLS[nextKind].includes(protocol)) {
                  setProtocol(KIND_PROTOCOLS[nextKind][0]);
                }
              }}
              protocol={protocol} setProtocol={setProtocol}
              version={version} setVersion={setVersion}
              packageId={packageId} setPackageId={setPackageId}
              availablePackages={availablePackages}
              presetsForKind={presetsForKind}
              selectedPresetId={selectedPresetId} setSelectedPresetId={setSelectedPresetId}
              dbMeta={dbMeta} dbColor={dbColor}
              allowedKinds={allowedKinds}
              externalEnabled={externalEnabled} setExternalEnabled={setExternalEnabled}
              externalEndpoint={externalEndpoint} setExternalEndpoint={setExternalEndpoint}
              externalDatabase={externalDatabase} setExternalDatabase={setExternalDatabase}
              externalUsername={externalUsername} setExternalUsername={setExternalUsername}
              externalPassword={externalPassword} setExternalPassword={setExternalPassword}
              externalSSLMode={externalSSLMode} setExternalSSLMode={setExternalSSLMode}
              onSaveAsTopologyPreset={handleSaveAsTopologyPreset}
              defaultPresetName={runName}
              defaultPresetDescription={runDescription}
              topologyEdits={topologyEdits}
              setTopologyEdits={setTopologyEdits}
              onPresetCreated={(newID) => { setSelectedPresetId(newID); refreshPresets(); }}
              tPath={tPath}
            />
          )}
          {step === 2 && (
            <WorkloadForm
              script={script} setScript={setScript}
              sql={sql} setSql={setSql}
              stroppyEnv={stroppyEnv} setStroppyEnv={setStroppyEnv}
              workloadFiles={workloadFiles} setWorkloadFiles={setWorkloadFiles}
              duration={duration} setDuration={setDuration}
              k6Mode={k6Mode} setK6Mode={setK6Mode}
              iterations={iterations} setIterations={setIterations}
              quiet={quiet} setQuiet={setQuiet}
              noThresholds={noThresholds} setNoThresholds={setNoThresholds}
              defaultInsertMethod={defaultInsertMethod} setDefaultInsertMethod={setDefaultInsertMethod}
              scaleFactor={scaleFactor} setScaleFactor={setScaleFactor}
              vus={vus} setVus={setVus}
              poolSize={poolSize} setPoolSize={setPoolSize}
              dbKind={kind}
              protocol={protocol}
              provider={provider}
              platformId={platformId}
              quotas={quotas}
              availableSteps={availableSteps} setAvailableSteps={setAvailableSteps}
              selectedSteps={selectedSteps} setSelectedSteps={setSelectedSteps}
              noSteps={noSteps} setNoSteps={setNoSteps}
              probeData={probeData} setProbeData={setProbeData}
              setWorkloadProbeOK={setWorkloadProbeOK}
              stroppyCpus={stroppyCpus} setStroppyCpus={setStroppyCpus}
              stroppyMemory={stroppyMemory} setStroppyMemory={setStroppyMemory}
              stroppyDisk={stroppyDisk} setStroppyDisk={setStroppyDisk}
              stroppyDiskType={stroppyDiskType} setStroppyDiskType={setStroppyDiskType}
              stroppyVersion={stroppyVersion} setStroppyVersion={setStroppyVersion}
              stroppyVersions={stroppyVersions} setStroppyVersions={setStroppyVersions}
              versionsLoading={versionsLoading} setVersionsLoading={setVersionsLoading}
              versionsLoaded={versionsLoaded}
              stroppyConfigDraft={stroppyConfigDraft}
              setStroppyConfigDraft={setStroppyConfigDraft}
              stroppyConfigPristine={stroppyConfigPristine}
              setStroppyConfigPristine={setStroppyConfigPristine}
              stroppyConfigUserEdited={stroppyConfigUserEdited}
              setStroppyConfigUserEdited={setStroppyConfigUserEdited}
            />
          )}
          {step === 3 && (
            <StepReview
              dryRunResult={dryRunResult}
              dryRunLoading={dryRunLoading}
              validationResult={validationResult}
              error={error}
              submitting={submitting}
              onSubmit={handleSubmit}
              onEdit={(group, key, value) => {
                const n = parseInt(value);
                if (group === "benchmark") {
                  if (key === "VUs" && !isNaN(n)) setVus(n);
                  else if (key === "duration") setDuration(value);
                  else if (key === "pool" && !isNaN(n)) setPoolSize(n);
                  else if (key === "scale" && !isNaN(n)) setScaleFactor(n);
                } else if (group === "infrastructure") {
                  if (key === "platform") setPlatformId(value);
                }
              }}
              dbConfigDraft={dbConfigDraft}
              setDbConfigDraft={setDbConfigDraft}
              stroppyConfigDraft={stroppyConfigDraft}
              setStroppyConfigDraft={(v) => {
                setStroppyConfigDraft(v);
                setStroppyConfigUserEdited(stroppyConfigPristine === null ? stroppyConfigUserEdited : v !== stroppyConfigPristine);
              }}
              script={script}
              scaleFactor={scaleFactor}
              stroppyVersion={stroppyVersion}
              setStroppyVersion={setStroppyVersion}
              renderedConfigDrafts={renderedConfigDrafts}
              setRenderedConfigDrafts={setRenderedConfigDrafts}
              renderedConfigsPristine={renderedConfigsPristine}
              onSaveAsPreset={handleSaveAsPreset}
              defaultPresetName={runName}
              defaultPresetDescription={runDescription}
              hasResolvedConfig={resolvedConfig !== null}
            />
          )}

          {/* Navigation */}
          {step < 3 && (
            <div className="flex items-center justify-between mt-6 pt-4 border-t border-zinc-800/50">
              <Button variant="outline" size="sm" onClick={() => setStep(Math.max(0, step - 1))} disabled={step === 0}>
                <ChevronLeft className="h-3 w-3" /> Back
              </Button>
              <Button size="sm" onClick={() => setStep(step + 1)} className="gap-1.5" disabled={step === 2 && !workloadProbeOK}>
                Next <ChevronRight className="h-3 w-3" />
              </Button>
            </div>
          )}
        </div>

        {/* Right sidebar */}
        <div className="w-80 shrink-0 flex flex-col bg-[#050505] border-l border-zinc-800/50 overflow-hidden">
          {/* Summary */}
          <div className="shrink-0 px-4 py-3 border-b border-zinc-800/50 space-y-2">
            <div className="text-[10px] font-mono text-zinc-600 uppercase tracking-wider">Setup Summary</div>
            <div className="space-y-1.5">
              <SummaryRow icon={ProvIcon} label="Provider" value={PROVIDER_META[provider].label} />
              {provider === "yandex" && <SummaryRow label="Platform" value={platformLimits(platformId).label} />}
              <SummaryRow icon={DbIcon} label="Database" value={`${dbMeta.label} ${version}`} color={dbColor.text} />
              {selectedPreset && (
                <SummaryRow label="Topology" value={selectedPreset.name} />
              )}
              <SummaryRow label="Script" value={script} />
              {sql && <SummaryRow label="SQL" value={sql} />}
              <SummaryRow label={k6Mode === "iterations" ? "Iterations" : "Duration"} value={k6Mode === "iterations" ? String(iterations) : duration} />
              <SummaryRow label="VUs" value={String(vus)} />
              <SummaryRow label="Pool" value={String(poolSize)} />
              {scaleFactor > 1 && <SummaryRow label="Scale" value={String(scaleFactor)} />}
            </div>
          </div>

          {/* Config JSON */}
          <div className="shrink-0 flex items-center justify-between px-4 py-2 border-b border-zinc-800/50">
            <div className="flex items-center gap-2">
              <span className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">Config</span>
              <span className="text-[9px] font-mono text-zinc-700 tabular-nums">{runIDRef.current}</span>
            </div>
            <button type="button" onClick={handleCopy}
              className="p-1 text-zinc-600 hover:text-zinc-300 transition-colors cursor-pointer" title="Copy">
              {copied ? <Check className="h-3 w-3 text-emerald-400" /> : <Copy className="h-3 w-3" />}
            </button>
          </div>
          <pre className="flex-1 p-3 text-[10px] font-mono leading-[1.5] text-zinc-500 overflow-auto selection:bg-primary/20">
            {configJSON}
          </pre>
        </div>
      </div>
    </div>
  );
}

// ─── Summary Row ─────────────────────────────────────────────────

function SummaryRow({ icon: Icon, label, value, color }: {
  icon?: typeof Database;
  label: string;
  value: string;
  color?: string;
}) {
  return (
    <div className="flex items-center gap-2 text-[11px] font-mono">
      {Icon && <Icon className={`w-3 h-3 shrink-0 ${color || "text-zinc-600"}`} />}
      {!Icon && <span className="w-3" />}
      <span className="text-zinc-600">{label}</span>
      <span className={`ml-auto ${color || "text-zinc-400"}`}>{value}</span>
    </div>
  );
}

// ─── Step 2: Database ────────────────────────────────────────────

function StepDatabase({
  kind, setKind,
  protocol, setProtocol,
  version, setVersion,
  packageId, setPackageId,
  availablePackages,
  presetsForKind,
  selectedPresetId, setSelectedPresetId,
  dbMeta, dbColor,
  allowedKinds,
  externalEnabled, setExternalEnabled,
  externalEndpoint, setExternalEndpoint,
  externalDatabase, setExternalDatabase,
  externalUsername, setExternalUsername,
  externalPassword, setExternalPassword,
  externalSSLMode, setExternalSSLMode,
  onSaveAsTopologyPreset,
  defaultPresetName,
  defaultPresetDescription,
  topologyEdits,
  setTopologyEdits,
  onPresetCreated,
  tPath,
}: {
  kind: DatabaseKind; setKind: (k: DatabaseKind) => void;
  protocol: Protocol; setProtocol: (p: Protocol) => void;
  allowedKinds: DatabaseKind[];
  version: string; setVersion: (v: string) => void;
  packageId: string; setPackageId: (v: string) => void;
  availablePackages: UiPackage[];
  presetsForKind: UiPreset[];
  selectedPresetId: string; setSelectedPresetId: (v: string) => void;
  dbMeta: { icon: typeof Database; label: string };
  dbColor: { hex: string; text: string; accent: string };
  externalEnabled: boolean; setExternalEnabled: (v: boolean) => void;
  externalEndpoint: string; setExternalEndpoint: (v: string) => void;
  externalDatabase: string; setExternalDatabase: (v: string) => void;
  externalUsername: string; setExternalUsername: (v: string) => void;
  externalPassword: string; setExternalPassword: (v: string) => void;
  externalSSLMode: string; setExternalSSLMode: (v: string) => void;
  onSaveAsTopologyPreset: (name: string, description: string) => Promise<void>;
  defaultPresetName: string;
  defaultPresetDescription: string;
  topologyEdits: Record<string, unknown>;
  setTopologyEdits: React.Dispatch<React.SetStateAction<Record<string, unknown>>>;
  onPresetCreated: (id: string) => void;
  tPath: (p: string) => string;
}) {
  const protocolsForKind = KIND_PROTOCOLS[kind];
  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-sm font-semibold mb-1">Database</h2>
        <p className="text-xs text-zinc-500">Choose the database engine, version, and topology preset.</p>
      </div>

      {/* BYO toggle */}
      <div className="border border-zinc-800/60 bg-[#070707] p-3 space-y-3">
        <label className="flex items-center gap-2 cursor-pointer select-none">
          <input
            type="checkbox"
            checked={externalEnabled}
            onChange={(e) => setExternalEnabled(e.target.checked)}
            className="accent-primary"
          />
          <span className="text-xs font-mono text-zinc-300">
            Bring your own database
          </span>
          <span className="text-[10px] font-mono text-zinc-600 ml-1">
            — skip infra; stroppy points at an existing endpoint
          </span>
        </label>
        {externalEnabled && (
          <div className="grid grid-cols-2 gap-3 pt-1">
            <div className="space-y-1.5 col-span-2">
              <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Endpoint</Label>
              <input
                value={externalEndpoint}
                onChange={(e) => setExternalEndpoint(e.target.value)}
                placeholder="host:port"
                className="w-full bg-[#0a0a0a] border border-zinc-800 px-2 py-1.5 text-xs font-mono text-zinc-200 focus:outline-none focus:border-primary/60"
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Database</Label>
              <input
                value={externalDatabase}
                onChange={(e) => setExternalDatabase(e.target.value)}
                placeholder="stroppy"
                className="w-full bg-[#0a0a0a] border border-zinc-800 px-2 py-1.5 text-xs font-mono text-zinc-200 focus:outline-none focus:border-primary/60"
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">SSL Mode</Label>
              <input
                value={externalSSLMode}
                onChange={(e) => setExternalSSLMode(e.target.value)}
                placeholder="disable / require / ..."
                className="w-full bg-[#0a0a0a] border border-zinc-800 px-2 py-1.5 text-xs font-mono text-zinc-200 focus:outline-none focus:border-primary/60"
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Username</Label>
              <input
                value={externalUsername}
                onChange={(e) => setExternalUsername(e.target.value)}
                placeholder="stroppy"
                className="w-full bg-[#0a0a0a] border border-zinc-800 px-2 py-1.5 text-xs font-mono text-zinc-200 focus:outline-none focus:border-primary/60"
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Password</Label>
              <input
                type="password"
                value={externalPassword}
                onChange={(e) => setExternalPassword(e.target.value)}
                className="w-full bg-[#0a0a0a] border border-zinc-800 px-2 py-1.5 text-xs font-mono text-zinc-200 focus:outline-none focus:border-primary/60"
              />
            </div>
          </div>
        )}
      </div>

      {/* DB Kind */}
      <div className="grid grid-cols-3 gap-2">
        {allowedKinds.map((k) => {
          const meta = DB_META[k];
          const kColor = DB_COLORS[k];
          const Icon = meta.icon;
          const active = kind === k;
          return (
            <button type="button" key={k} onClick={() => setKind(k)}
              className={`flex items-center gap-2.5 border p-2.5 transition-all cursor-pointer ${
                active ? `${kColor.accent}` : "border-zinc-800/60 hover:bg-zinc-900/50 hover:border-zinc-700"
              }`}
            >
              <Icon className={`h-4 w-4 ${active ? kColor.text : "text-zinc-600"}`} />
              <span className={`text-sm font-mono font-medium ${active ? kColor.text : "text-zinc-500"}`}>{meta.label}</span>
            </button>
          );
        })}
      </div>

      {/* Protocol toggle */}
      {protocolsForKind.length > 1 && (
        <div className="space-y-1.5">
          <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Protocol</Label>
          <div className="inline-flex border border-zinc-800 overflow-hidden text-[11px] font-mono">
            {protocolsForKind.map((p) => {
              const active = protocol === p;
              return (
                <button
                  key={p}
                  type="button"
                  onClick={() => setProtocol(p)}
                  className={`px-3 py-1 transition-colors cursor-pointer ${
                    active ? `${dbColor.text} ${dbColor.accent}` : "text-zinc-500 hover:text-zinc-300 hover:bg-zinc-900"
                  }`}
                >
                  {p}
                </button>
              );
            })}
          </div>
          <p className="text-[10px] font-mono text-zinc-600">
            Wire format stroppy uses to talk to {dbMeta.label}. Different protocols expose different SQL feature ceilings — the script list on step 2 narrows accordingly.
          </p>
        </div>
      )}

      {/* Version + Package */}
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Version</Label>
          <Select
            value={DB_VERSIONS[kind].includes(version) ? version : "__custom__"}
            onValueChange={(v) => { if (v !== "__custom__") setVersion(v); }}
          >
            <SelectTrigger className="h-8 font-mono text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              {DB_VERSIONS[kind].map((v) => (
                <SelectItem key={v} value={v}>{v}</SelectItem>
              ))}
              <SelectItem value="__custom__">Custom...</SelectItem>
            </SelectContent>
          </Select>
          {!DB_VERSIONS[kind].includes(version) && (
            <input
              value={version}
              onChange={(e) => setVersion(e.target.value)}
              placeholder="Enter version"
              className="h-7 w-full px-2 font-mono text-xs bg-transparent border border-zinc-800 text-zinc-300 outline-none focus:border-zinc-600"
            />
          )}
        </div>
        {kind !== "ydb" && (
          <div className="space-y-1.5">
            <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Package</Label>
            <Select value={packageId || "__default__"} onValueChange={(v) => setPackageId(v === "__default__" ? "" : v)}>
              <SelectTrigger className="h-8 font-mono text-xs"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="__default__">Default</SelectItem>
                {availablePackages.map((p) => (
                  <SelectItem key={p.id} value={p.id}>
                    {p.name}{p.has_deb ? " [.deb]" : ""}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <a href={tPath("packages")} className="text-[9px] font-mono text-zinc-500 hover:text-zinc-300">manage packages</a>
          </div>
        )}
      </div>

      {/* Topology Preset */}
      <div>
        <div className="flex items-center justify-between mb-3">
          <span className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Topology Preset</span>
          <a href={tPath("presets")} className="text-[9px] font-mono text-zinc-500 hover:text-zinc-300">manage presets</a>
        </div>
        <div className="grid grid-cols-3 gap-3">
          {presetsForKind.map((p) => {
            const active = selectedPresetId === p.id;
            return (
              <button type="button" key={p.id} onClick={() => setSelectedPresetId(p.id)}
                className={`border p-3 text-left transition-all cursor-pointer ${
                  active ? `${dbColor.accent}` : "border-zinc-800/60 hover:bg-zinc-900/50 hover:border-zinc-700"
                }`}
              >
                <div className="flex items-center justify-between mb-2">
                  <span className={`text-xs font-mono font-semibold uppercase tracking-wider ${active ? dbColor.text : "text-zinc-400"}`}>
                    {p.name}
                  </span>
                  <div className="flex items-center gap-1">
                    {!p.is_builtin && <span className="text-[8px] text-zinc-600 font-mono">custom</span>}
                    {active && <div className="w-1.5 h-1.5 rounded-full" style={{ backgroundColor: dbColor.hex }} />}
                  </div>
                </div>
                <TopologyDiagram kind={kind} topology={p.topology} />
              </button>
            );
          })}
        </div>
      </div>

      {/* Inline topology editor */}
      {!externalEnabled && (
        <TopologyEditPanel
          kind={kind}
          presetsForKind={presetsForKind}
          selectedPresetId={selectedPresetId}
          topologyEdits={topologyEdits}
          setTopologyEdits={setTopologyEdits}
          defaultPresetName={defaultPresetName}
          defaultPresetDescription={defaultPresetDescription}
          onSaveAsTopologyPreset={onSaveAsTopologyPreset}
          onPresetCreated={onPresetCreated}
        />
      )}
    </div>
  );
}

function TopologyEditPanel({
  kind,
  presetsForKind,
  selectedPresetId,
  topologyEdits,
  setTopologyEdits,
  defaultPresetName,
  defaultPresetDescription,
  onSaveAsTopologyPreset,
  onPresetCreated,
}: {
  kind: DatabaseKind;
  presetsForKind: UiPreset[];
  selectedPresetId: string;
  topologyEdits: Record<string, unknown>;
  setTopologyEdits: React.Dispatch<React.SetStateAction<Record<string, unknown>>>;
  defaultPresetName: string;
  defaultPresetDescription: string;
  onSaveAsTopologyPreset: (name: string, description: string) => Promise<void>;
  onPresetCreated: (id: string) => void;
}) {
  const selectedPreset = presetsForKind.find((p) => p.id === selectedPresetId);
  const [editing, setEditing] = useState(false);
  const [pg, setPg] = useState<PostgresTopology>(defaultPostgres());
  const [my, setMy] = useState<MySQLTopology>(defaultMySQL());
  const [pico, setPico] = useState<PicodataTopology>(defaultPicodata());
  const [ydb, setYdb] = useState<YDBTopology>(defaultYDB());
  const [ydbm, setYdbm] = useState<YDBManagedTopology>(defaultYDBManaged());

  const editorSupported = kind === "postgres" || kind === "mysql" || kind === "mariadb" || kind === "picodata" || kind === "ydb" || kind === "ydb-managed";
  const hasEdits = !!topologyEdits[kind];

  function loadFromPreset() {
    if (!selectedPreset) return;
    const t = selectedPreset.topology as unknown;
    if (kind === "postgres") setPg(t as PostgresTopology);
    else if (kind === "mysql" || kind === "mariadb") setMy(t as MySQLTopology);
    else if (kind === "picodata") setPico(t as PicodataTopology);
    else if (kind === "ydb") setYdb(t as YDBTopology);
    else if (kind === "ydb-managed") setYdbm(t as YDBManagedTopology);
  }

  useEffect(() => {
    loadFromPreset();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedPresetId, kind]);

  function pushEditsToParent(t: unknown) {
    setTopologyEdits((prev) => ({ ...prev, [kind]: t }));
  }

  function discardEdits() {
    setTopologyEdits((prev) => {
      const next = { ...prev };
      delete next[kind];
      return next;
    });
    loadFromPreset();
  }

  function renderForm() {
    if (kind === "postgres") {
      return <PostgresForm topology={pg} onChange={(t) => { setPg(t); pushEditsToParent(t); }} disabled={false} />;
    }
    if (kind === "mysql" || kind === "mariadb") {
      return <MySQLForm topology={my} onChange={(t) => { setMy(t); pushEditsToParent(t); }} disabled={false} />;
    }
    if (kind === "picodata") {
      return <PicodataForm topology={pico} onChange={(t) => { setPico(t); pushEditsToParent(t); }} disabled={false} />;
    }
    if (kind === "ydb") {
      return <YDBForm topology={ydb} onChange={(t) => { setYdb(t); pushEditsToParent(t); }} disabled={false} />;
    }
    if (kind === "ydb-managed") {
      return <YDBManagedForm topology={ydbm} onChange={(t) => { setYdbm(t); pushEditsToParent(t); }} disabled={false} />;
    }
    return null;
  }

  return (
    <div className="border-t border-zinc-800/50 pt-4 space-y-3">
      <div className="flex items-center gap-2">
        {editorSupported && selectedPreset && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => setEditing((v) => !v)}
            className="gap-1.5"
          >
            <Pencil className="h-3.5 w-3.5" />
            {editing ? "Hide editor" : "Edit topology"}
          </Button>
        )}
        {hasEdits && (
          <>
            <span className="text-[10px] font-mono text-amber-400/80 border border-amber-500/30 px-2 py-0.5">
              modified
            </span>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={discardEdits}
              className="gap-1.5 text-zinc-500 hover:text-zinc-200"
            >
              <RotateCcw className="h-3 w-3" />
              Discard edits
            </Button>
          </>
        )}
        <div className="flex-1" />
        <SaveAsPresetButton
          variant="topology"
          defaultName={defaultPresetName}
          defaultDescription={defaultPresetDescription}
          onSave={async (name, description) => {
            await onSaveAsTopologyPreset(name, description);
            setTopologyEdits((prev) => {
              const next = { ...prev };
              delete next[kind];
              return next;
            });
            onPresetCreated("");
          }}
          disabled={false}
        />
      </div>

      {editing && editorSupported && (
        <div className="border border-zinc-800/60 bg-[#070707] p-4">
          {renderForm()}
        </div>
      )}
      {editing && !editorSupported && (
        <div className="border border-dashed border-zinc-800 p-4 text-xs text-zinc-500">
          Inline editor isn't available for this database kind yet — edit the
          topology directly in the JSON viewer on the Review step.
        </div>
      )}
    </div>
  );
}

// ─── Step 4: Review & Launch ─────────────────────────────────────

// Phase grouping — mirrors DAG dependency structure.
const PHASE_GROUPS: { label: string; icon: typeof Database; phases: string[] }[] = [
  { label: "Infrastructure", icon: Server, phases: ["network", "machines"] },
  { label: "Database", icon: Database, phases: ["install_etcd", "configure_etcd", "install_patroni", "configure_patroni", "install_db", "configure_db", "install_pgbouncer", "configure_pgbouncer"] },
  { label: "Proxy", icon: Server, phases: ["install_proxy", "configure_proxy"] },
  { label: "Monitoring", icon: Server, phases: ["install_monitor", "configure_monitor"] },
  { label: "Benchmark", icon: Rocket, phases: ["install_stroppy", "run_stroppy"] },
  { label: "Teardown", icon: Server, phases: ["teardown"] },
];

function humanPhase(id: string): string {
  return id.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

const EDITABLE_KEYS: Record<string, Set<string>> = {
  benchmark: new Set(["VUs", "duration", "pool", "scale"]),
  infrastructure: new Set(["platform"]),
};

function EditableCfgRow({ k, v, groupKey, onEdit }: {
  k: string; v: string; groupKey: string;
  onEdit?: (group: string, key: string, value: string) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(v);
  const editable = onEdit && EDITABLE_KEYS[groupKey]?.has(k);

  const commit = () => {
    setEditing(false);
    if (draft !== v && onEdit) onEdit(groupKey, k, draft);
  };

  if (editing) {
    return (
      <div className="flex gap-2 text-[10px] font-mono leading-relaxed">
        <span className="text-zinc-600 shrink-0 w-20 text-right">{k}</span>
        <input
          autoFocus
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onBlur={commit}
          onKeyDown={(e) => { if (e.key === "Enter") commit(); if (e.key === "Escape") { setDraft(v); setEditing(false); } }}
          className="flex-1 bg-zinc-800 text-zinc-200 px-1 py-0 border border-zinc-600 outline-none text-[10px] font-mono"
        />
      </div>
    );
  }

  return (
    <div className="flex gap-2 text-[10px] font-mono leading-relaxed group/row">
      <span className="text-zinc-600 shrink-0 w-20 text-right">{k}</span>
      <span className="text-zinc-400 truncate flex-1" title={v}>{v}</span>
      {editable && (
        <button type="button" onClick={() => { setDraft(v); setEditing(true); }}
          className="opacity-0 group-hover/row:opacity-100 text-zinc-600 hover:text-zinc-400 transition-opacity shrink-0">
          <Pencil className="w-2.5 h-2.5" />
        </button>
      )}
    </div>
  );
}

interface DryRunNode {
  id: string;
  type: string;
  deps?: string[];
}

function StepReview({
  dryRunResult,
  dryRunLoading,
  validationResult,
  error,
  submitting,
  onSubmit,
  onEdit,
  dbConfigDraft,
  setDbConfigDraft,
  stroppyConfigDraft,
  setStroppyConfigDraft,
  stroppyVersion,
  setStroppyVersion,
  renderedConfigDrafts,
  setRenderedConfigDrafts,
  renderedConfigsPristine,
  onSaveAsPreset,
  defaultPresetName,
  defaultPresetDescription,
  hasResolvedConfig,
}: {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  dryRunResult: any;
  dryRunLoading: boolean;
  validationResult: { ok: boolean; message: string } | null;
  error: string | null;
  submitting: boolean;
  onSubmit: () => void;
  onEdit?: (group: string, key: string, value: string) => void;
  dbConfigDraft: string | null;
  setDbConfigDraft: (v: string) => void;
  stroppyConfigDraft: string | null;
  setStroppyConfigDraft: (v: string) => void;
  script: string;
  scaleFactor: number;
  stroppyVersion: string;
  setStroppyVersion: (v: string) => void;
  renderedConfigDrafts: Record<string, string>;
  setRenderedConfigDrafts: React.Dispatch<React.SetStateAction<Record<string, string>>>;
  renderedConfigsPristine: Record<string, string>;
  onSaveAsPreset: (name: string, description: string, alsoLaunch: boolean) => Promise<void>;
  defaultPresetName: string;
  defaultPresetDescription: string;
  hasResolvedConfig: boolean;
}) {
  // Launch-ready when the dry-run succeeded (or there's no validation
  // result yet but no surface error). Failed validation blocks launch.
  const canLaunch = !dryRunLoading && !error && (validationResult === null || validationResult.ok);
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());
  const toggle = (label: string) => setExpanded((prev) => {
    const next = new Set(prev);
    next.has(label) ? next.delete(label) : next.add(label);
    return next;
  });

  const dagNodes: DryRunNode[] = dryRunResult?.nodes || [];
  const nodeIds = new Set(dagNodes.map((n: DryRunNode) => n.id));

  const activeGroups = PHASE_GROUPS
    .map((g) => ({ ...g, phases: g.phases.filter((p) => nodeIds.has(p)) }))
    .filter((g) => g.phases.length > 0);

  const totalPhases = dagNodes.length;
  const reviewConfigs: Record<string, Record<string, string>> = dryRunResult?.effective_config || {};

  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-sm font-semibold mb-1">Review & Launch</h2>
        <p className="text-xs text-zinc-500">
          {dryRunLoading ? "Validating configuration and building execution plan..." : "Review the execution plan and launch the run."}
        </p>
      </div>

      {!dryRunLoading && !validationResult && activeGroups.length === 0 && (
        <div className="flex items-center gap-2 text-xs p-3 border border-zinc-800/50 font-mono text-zinc-500">
          <AlertCircle className="h-3 w-3" />
          Execution plan will appear here once dry-run completes.
        </div>
      )}
      {dryRunLoading && (
        <div className="flex items-center gap-2 text-xs p-3 border border-zinc-800/50 font-mono text-zinc-500">
          <Loader2 className="h-3 w-3 animate-spin" />
          Preparing execution plan...
        </div>
      )}
      {validationResult && !dryRunLoading && (
        <div className={`flex items-center gap-2 text-xs p-3 border font-mono ${
          validationResult.ok ? "border-emerald-500/30 text-emerald-400" : "border-red-500/30 text-red-400"
        }`}>
          {validationResult.ok ? <Check className="h-3 w-3" /> : <AlertCircle className="h-3 w-3" />}
          {validationResult.message}
          {validationResult.ok && totalPhases > 0 && (
            <span className="ml-auto text-zinc-600">{totalPhases} phases</span>
          )}
        </div>
      )}
      {error && (
        <div className="flex items-center gap-2 text-xs p-3 border border-red-500/30 text-red-400 font-mono">
          <AlertCircle className="h-3 w-3" />
          {error}
        </div>
      )}

      {activeGroups.length > 0 && (
        <div className="select-none w-full">
          {activeGroups.map((group, gi) => {
            const isLast = gi === activeGroups.length - 1;
            const GroupIcon = group.icon;
            const isExpanded = expanded.has(group.label);
            const groupKey = group.label.toLowerCase();
            const cfgEntries = reviewConfigs[groupKey];

            return (
              <div key={group.label} className="relative">
                {gi > 0 && (
                  <div className="flex justify-start pl-[11px]">
                    <div className="w-px h-2.5 bg-zinc-800" />
                  </div>
                )}

                <div className="border border-zinc-800/80 bg-zinc-900/30">
                  <button
                    type="button"
                    onClick={() => toggle(group.label)}
                    className="flex items-center gap-2 px-3 py-2 w-full text-left cursor-pointer hover:bg-white/[0.02] transition-colors"
                  >
                    <div className="w-6 h-6 rounded-full bg-zinc-900 border border-zinc-700 flex items-center justify-center shrink-0">
                      <GroupIcon className="w-3 h-3 text-zinc-500" />
                    </div>
                    <span className="text-[11px] font-semibold text-zinc-300 font-mono flex-1">{group.label}</span>
                    <span className="text-[10px] text-zinc-600 font-mono tabular-nums">{group.phases.length}</span>
                    <ChevronDown className={`w-3 h-3 text-zinc-600 transition-transform duration-200 ${isExpanded ? "rotate-180" : ""}`} />
                  </button>

                  {isExpanded && (
                    <div className="border-t border-zinc-800/50 flex flex-col md:flex-row md:divide-x md:divide-zinc-800/30">
                      <div className="md:w-72 md:shrink-0">
                        {group.phases.map((phaseId, pi) => {
                          const node = dagNodes.find((n: DryRunNode) => n.id === phaseId);
                          const isLastStep = pi === group.phases.length - 1;
                          const deps = node?.deps?.filter((d: string) => nodeIds.has(d)) || [];

                          return (
                            <div key={phaseId}>
                              <div className="flex items-center gap-2 px-3 py-1.5 relative">
                                {!isLastStep && (
                                  <div className="absolute left-[17px] top-[22px] w-px h-[calc(100%-10px)] bg-zinc-800" />
                                )}
                                <div className="w-[14px] h-[14px] rounded-full border border-zinc-700 bg-zinc-900 flex items-center justify-center shrink-0">
                                  <div className="w-1 h-1 rounded-full bg-zinc-600" />
                                </div>
                                <span className="text-[11px] font-mono text-zinc-400 flex-1">{humanPhase(phaseId)}</span>
                                {deps.length > 0 && (
                                  <span className="text-[9px] text-zinc-700 font-mono shrink-0">← {deps.map(humanPhase).join(", ")}</span>
                                )}
                              </div>
                              {!isLastStep && <div className="mx-3 border-b border-zinc-800/20" />}
                            </div>
                          );
                        })}
                      </div>

                      {groupKey === "database" && dbConfigDraft !== null && hasResolvedConfig ? (
                        <div className="px-3 py-1.5 bg-zinc-900/50 flex-1 min-w-0 border-t md:border-t-0 border-zinc-800/20">
                          <div className="flex items-center justify-between mb-1">
                            <span className="text-[9px] font-mono text-zinc-600 uppercase">Database Config (editable JSON — overrides field-level settings)</span>
                          </div>
                          <JsonEditor
                            value={dbConfigDraft}
                            onChange={setDbConfigDraft}
                            height="18rem"
                          />
                        </div>
                      ) : groupKey === "benchmark" && stroppyConfigDraft !== null ? (
                        <div className="px-3 py-1.5 bg-zinc-900/50 flex-1 min-w-0 border-t md:border-t-0 border-zinc-800/20">
                          <div className="flex items-center gap-2 mb-2">
                            <span className="text-[10px] font-mono text-zinc-500 shrink-0">Stroppy Version</span>
                            <input
                              type="text"
                              value={stroppyVersion}
                              onChange={(e) => setStroppyVersion(e.target.value)}
                              spellCheck={false}
                              placeholder="5.0.0rc3"
                              className="flex-1 bg-[#0a0a0a] text-[11px] font-mono text-zinc-300 border border-zinc-800 px-2 py-1 outline-none focus:border-zinc-600"
                            />
                          </div>
                          <div className="flex items-center justify-between mb-1">
                            <span className="text-[9px] font-mono text-zinc-600 uppercase">Stroppy Config (editable protojson — overrides field-level settings)</span>
                          </div>
                          <JsonEditor
                            value={stroppyConfigDraft}
                            onChange={setStroppyConfigDraft}
                            height="18rem"
                          />
                          {cfgEntries && Object.keys(cfgEntries).length > 0 && (
                            <div className="mt-2 space-y-0.5">
                              <span className="text-[9px] font-mono text-zinc-600 uppercase">Summary</span>
                              {Object.entries(cfgEntries).map(([k, v]) => (
                                <EditableCfgRow key={k} k={k} v={v} groupKey={groupKey} onEdit={onEdit} />
                              ))}
                            </div>
                          )}
                        </div>
                      ) : cfgEntries && Object.keys(cfgEntries).length > 0 ? (
                        <div className="px-3 py-1.5 bg-zinc-900/50 space-y-0.5 flex-1 min-w-0 border-t md:border-t-0 border-zinc-800/20">
                          {Object.entries(cfgEntries).map(([k, v]) => (
                            <EditableCfgRow key={k} k={k} v={v} groupKey={groupKey} onEdit={onEdit} />
                          ))}
                        </div>
                      ) : null}
                    </div>
                  )}
                </div>

                {!isLast && (
                  <div className="flex justify-start pl-[11px]">
                    <div className="w-px h-2 bg-zinc-800" />
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* Save as Run Preset + Launch */}
      <div className="flex flex-col sm:flex-row gap-2">
        <SaveAsPresetButton
          variant="run"
          defaultName={defaultPresetName}
          defaultDescription={defaultPresetDescription}
          onSave={onSaveAsPreset}
          disabled={!canLaunch}
          allowLaunchToggle
          fullWidth
        />
        <Button
          size="lg"
          onClick={onSubmit}
          disabled={!canLaunch || submitting}
          className="flex-1 w-full gap-2 h-12 text-base"
        >
          <Rocket className="h-5 w-5" />
          {submitting ? "Launching..." : "Launch Run"}
        </Button>
      </div>

      {/* renderedConfigDrafts is reserved for per-component config editing
          surfaced from the dry-run DAG. The current UI exposes db/stroppy
          drafts inline (see groupKey === "database" / "benchmark" above);
          deep per-phase editing is layered on as additional rows fed by
          dryRunResult.effective_config in a follow-up. */}
      {void setRenderedConfigDrafts}
      {void renderedConfigDrafts}
      {void renderedConfigsPristine}
    </div>
  );
}

function SaveAsPresetButton({ variant, defaultName, defaultDescription, onSave, disabled, allowLaunchToggle, fullWidth }: {
  variant: "run" | "topology";
  defaultName: string;
  defaultDescription: string;
  onSave: (name: string, description: string, alsoLaunch: boolean) => Promise<void>;
  disabled?: boolean;
  allowLaunchToggle?: boolean;
  fullWidth?: boolean;
}) {
  const isRun = variant === "run";
  const buttonLabel = isRun ? "Save as Run Preset" : "Save as Topology Preset";
  const dialogTitle = isRun ? "Save as Run Preset" : "Save as Topology Preset";
  const Icon = isRun ? FlaskConical : Layers;
  const dialogDescription = isRun
    ? "Persists the current workload + infrastructure config as a reusable template. The run-specific name & description (if any) are stripped before saving."
    : "Persists the current database topology subtree (machine specs, replicas, options, proxy/etcd toggles) as a new entry in the topology presets catalog. Workload & stroppy settings are not saved.";
  const placeholder = isRun ? "e.g. tpcc-pg17-100vu-baseline" : "e.g. PostgreSQL HA-tuned (32GB shared_buffers)";
  const [open, setOpen] = useState(false);
  const [name, setName] = useState(defaultName);
  const [description, setDescription] = useState(defaultDescription);
  const [alsoLaunch, setAlsoLaunch] = useState(false);
  const [saving, setSaving] = useState(false);
  const [done, setDone] = useState(false);
  const [err, setErr] = useState("");

  useEffect(() => {
    if (open) {
      setName(defaultName);
      setDescription(defaultDescription);
      setAlsoLaunch(false);
      setDone(false);
      setErr("");
    }
  }, [open, defaultName, defaultDescription]);

  async function save() {
    if (!name.trim()) return;
    setSaving(true);
    setErr("");
    try {
      await onSave(name, description, alsoLaunch);
      setDone(true);
      window.setTimeout(() => setOpen(false), 700);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "Failed");
    } finally {
      setSaving(false);
    }
  }

  return (
    <>
      <Button
        type="button"
        variant="outline"
        size="lg"
        disabled={disabled}
        onClick={() => setOpen(true)}
        className={`h-12 gap-2 text-base ${fullWidth ? "flex-1 w-full" : ""}`}
      >
        <Icon className="h-4 w-4" />
        {buttonLabel}
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{dialogTitle}</DialogTitle>
            <DialogDescription>{dialogDescription}</DialogDescription>
          </DialogHeader>
          <div className="space-y-3 pt-2">
            <div className="space-y-1.5">
              <label className="text-xs text-zinc-500 font-mono">Name</label>
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                autoFocus
                placeholder={placeholder}
                className="w-full bg-[#0a0a0a] border border-zinc-800 px-2 py-1.5 text-sm font-mono text-zinc-200 focus:outline-none focus:border-primary/60"
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-xs text-zinc-500 font-mono">Description</label>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={3}
                placeholder="optional"
                className="w-full bg-[#0a0a0a] border border-zinc-800 px-2 py-1.5 text-xs font-mono text-zinc-300 focus:outline-none focus:border-primary/60 resize-none"
              />
            </div>
            {allowLaunchToggle && (
              <label className="flex items-center gap-2 cursor-pointer select-none pt-1">
                <input
                  type="checkbox"
                  checked={alsoLaunch}
                  onChange={(e) => setAlsoLaunch(e.target.checked)}
                  className="accent-primary"
                />
                <span className="text-xs font-mono text-zinc-300">
                  Launch run after saving
                </span>
                <Rocket className="h-3 w-3 text-zinc-600 ml-auto" />
              </label>
            )}
            {err && <div className="text-xs text-destructive font-mono">{err}</div>}
            <Button onClick={save} disabled={saving || done || !name.trim()}>
              {done
                ? alsoLaunch ? "Saved + launching..." : "Saved!"
                : saving
                  ? alsoLaunch ? "Saving + launching..." : "Saving..."
                  : alsoLaunch ? "Save + Launch" : "Save"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
