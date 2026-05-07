import { useMemo, useState, useEffect, useRef } from "react";
import type { NodeStatus, NodeStatusValue, RunConfig, MachineSpec, DatabaseKind } from "@/api/types";
import { TopologyDiagram } from "@/components/TopologyDiagram";
import { listPresets } from "@/api/client";
import {
  Check,
  X,
  Loader2,
  Circle,
  Server,
  Database,
  Zap,
  BarChart3,
  Play,
  Trash2,
  Copy,
  Cpu,
  HardDrive,
  MemoryStick,
  Clock,
  Users,
  Network,
  Container,
  Tag,
  Timer,
  FileCode,
  ChevronDown,
  Repeat,
  VolumeX,
  AlertOctagon,
  Wand2,
  FileText,
} from "lucide-react";

// ─── Phase groups ────────────────────────────────────────────────

const phaseGroups: { label: string; icon: typeof Zap; phases: string[] }[] = [
  { label: "Infrastructure", icon: Server, phases: ["network", "machines"] },
  {
    label: "Database",
    icon: Database,
    phases: [
      "install_etcd", "configure_etcd",
      "install_db", "configure_db",
      "install_pgbouncer", "configure_pgbouncer",
    ],
  },
  { label: "Proxy", icon: Zap, phases: ["install_proxy", "configure_proxy"] },
  { label: "Monitoring", icon: BarChart3, phases: ["install_monitor", "configure_monitor"] },
  { label: "Benchmark", icon: Play, phases: ["install_stroppy", "run_stroppy"] },
  { label: "Teardown", icon: Trash2, phases: ["teardown"] },
];

function groupStatus(
  phases: string[],
  nodeMap: Map<string, NodeStatus>,
): NodeStatusValue {
  const statuses = phases.map((p) => nodeMap.get(p)?.status).filter(Boolean) as NodeStatusValue[];
  if (statuses.length === 0) return "pending";
  if (statuses.some((s) => s === "failed")) return "failed";
  if (statuses.some((s) => s === "cancelled")) return "cancelled";
  if (statuses.every((s) => s === "done")) return "done";
  if (statuses.some((s) => s === "running" || s === "done")) return "running";
  return "pending";
}

function isStepActive(phaseId: string, allPhases: string[], nodeMap: Map<string, NodeStatus>): boolean {
  const idx = allPhases.indexOf(phaseId);
  if (nodeMap.get(phaseId)?.status !== "pending") return false;
  for (let i = 0; i < idx; i++) {
    const prev = nodeMap.get(allPhases[i]);
    if (prev && prev.status !== "done") return false;
  }
  return true;
}

function humanPhase(id: string): string {
  return id.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

// ─── Status colors ───────────────────────────────────────────────

const statusStyles = {
  done: { border: "border-emerald-500/25", bg: "bg-emerald-500/[0.03]", text: "text-emerald-400", connector: "bg-emerald-500/30" },
  failed: { border: "border-red-500/25", bg: "bg-red-500/[0.03]", text: "text-red-400", connector: "bg-red-500/30" },
  cancelled: { border: "border-zinc-600/25", bg: "bg-zinc-500/[0.03]", text: "text-zinc-400", connector: "bg-zinc-600/30" },
  running: { border: "border-amber-500/25", bg: "bg-amber-500/[0.03]", text: "text-amber-400", connector: "bg-amber-500/30" },
  pending: { border: "border-zinc-800/60", bg: "bg-transparent", text: "text-zinc-600", connector: "bg-zinc-800" },
} as const;

// ─── Tiny components ─────────────────────────────────────────────

function StepDot({ status }: { status: NodeStatusValue; active?: boolean; cancelled?: boolean }) {
  if (status === "done")
    return <div className="w-3.5 h-3.5 rounded-full bg-emerald-500 flex items-center justify-center shrink-0"><Check className="w-2 h-2 text-white" strokeWidth={3} /></div>;
  if (status === "cancelled")
    return <div className="w-3.5 h-3.5 rounded-full bg-zinc-500 flex items-center justify-center shrink-0"><X className="w-2 h-2 text-white" strokeWidth={3} /></div>;
  if (status === "failed")
    return <div className="w-3.5 h-3.5 rounded-full bg-red-500 flex items-center justify-center shrink-0"><X className="w-2 h-2 text-white" strokeWidth={3} /></div>;
  if (status === "running")
    return <div className="w-3.5 h-3.5 rounded-full bg-blue-500 flex items-center justify-center shrink-0 animate-pulse"><Loader2 className="w-2 h-2 text-white animate-spin" strokeWidth={3} /></div>;
  return <div className="w-3.5 h-3.5 rounded-full border border-zinc-700 bg-zinc-900 flex items-center justify-center shrink-0"><Circle className="w-1 h-1 text-zinc-600" fill="currentColor" /></div>;
}

function CopyButton({ text }: { text: string }) {
  return (
    <button
      type="button"
      onClick={() => navigator.clipboard.writeText(text)}
      className="p-0.5 text-zinc-600 hover:text-zinc-400 transition-colors"
      title="Copy error"
    >
      <Copy className="w-3 h-3" />
    </button>
  );
}

// ─── Config panel ────────────────────────────────────────────────

function ConfigLine({ label, value, icon: Icon }: { label: string; value: React.ReactNode; icon?: typeof Cpu }) {
  const [copied, setCopied] = useState(false);
  if (!value) return null;
  const text = typeof value === "string" || typeof value === "number" ? String(value) : "";
  const onCopy = async () => {
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1200);
    } catch { /* ignore */ }
  };
  return (
    <div className="group flex items-start gap-2 py-[3px]">
      {Icon && <Icon className="w-3 h-3 text-zinc-600 shrink-0 mt-[3px]" />}
      <span className="text-[11px] text-zinc-500 shrink-0 w-16 mt-[1px]">{label}</span>
      <span className="text-xs text-zinc-300 font-mono break-all flex-1 min-w-0">{value}</span>
      {text && (
        <button
          onClick={onCopy}
          title={copied ? "copied" : "copy"}
          className="opacity-0 group-hover:opacity-100 transition-opacity shrink-0 text-zinc-600 hover:text-zinc-300 p-0.5"
        >
          {copied ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
        </button>
      )}
    </div>
  );
}

function MachineRow({ spec }: { spec: MachineSpec }) {
  const diskLabel = spec.disk_type === "network-ssd-io-m3" ? "io-m3" : spec.disk_type === "network-ssd" ? "ssd" : "";
  return (
    <div className="flex items-center gap-1.5 text-[11px] font-mono text-zinc-400 flex-wrap">
      <span className="text-zinc-500 w-16 shrink-0">{spec.role}</span>
      <span>{spec.count}x</span>
      <span className="text-zinc-700">·</span>
      <Cpu className="w-3 h-3 text-zinc-600" /><span>{spec.cpus}</span>
      <MemoryStick className="w-3 h-3 text-zinc-600" /><span>{spec.memory_mb >= 1024 ? `${(spec.memory_mb/1024).toFixed(0)}G` : `${spec.memory_mb}M`}</span>
      <HardDrive className="w-3 h-3 text-zinc-600" /><span>{spec.disk_gb}G</span>
      {diskLabel && <span className="text-zinc-600 text-[9px]">{diskLabel}</span>}
    </div>
  );
}

function ElapsedTimer({ startedAt }: { startedAt: string }) {
  const [elapsed, setElapsed] = useState("");
  useEffect(() => {
    const start = new Date(startedAt).getTime();
    if (isNaN(start) || start < 946684800000) return; // invalid or year < 2000
    const tick = () => {
      const sec = Math.max(0, Math.round((Date.now() - start) / 1000));
      if (sec < 60) setElapsed(`${sec}s`);
      else if (sec < 3600) setElapsed(`${Math.floor(sec / 60)}m ${sec % 60}s`);
      else { const h = Math.floor(sec / 3600); setElapsed(`${h}h ${Math.floor((sec % 3600) / 60)}m`); }
    };
    tick();
    const iv = setInterval(tick, 1000);
    return () => clearInterval(iv);
  }, [startedAt]);
  return <>{elapsed}</>;
}

function formatTs(ts?: string): string {
  if (!ts) return "";
  const d = new Date(ts);
  if (isNaN(d.getTime()) || d.getFullYear() < 2000) return "";
  return d.toLocaleString("en-GB", { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false });
}

function ConfigPanel({ config, startedAt, finishedAt, isRunning }: {
  config: RunConfig | null;
  startedAt?: string;
  finishedAt?: string;
  isRunning?: boolean;
}) {
  // Resolve preset_id → name. Single fetch per panel mount; preset list is small.
  const [presetName, setPresetName] = useState<string | undefined>(undefined);
  useEffect(() => {
    const id = config?.preset_id;
    if (!id) {
      setPresetName(undefined);
      return;
    }
    let cancelled = false;
    listPresets()
      .then((ps) => {
        if (cancelled) return;
        const found = (ps ?? []).find((p) => p.id === id);
        setPresetName(found?.name);
      })
      .catch(() => {/* ignore — column will fall back to id */});
    return () => { cancelled = true; };
  }, [config?.preset_id]);

  if (!config) {
    return (
      <div className="flex items-center justify-center h-full text-zinc-600 text-xs font-mono">
        No config data
      </div>
    );
  }

  const db = config.database;
  const topology = db.postgres || db.mysql || db.picodata || db.ydb || db.ydb_managed;
  const s = config.stroppy;
  const script = s.script || s.workload || "";
  const vus = s.vus || s.vus_scale || 0;

  // Topology summary
  let topoLabel = "";
  if (db.postgres) {
    const parts: string[] = ["master"];
    if (db.postgres.replicas?.length) parts.push(`${db.postgres.replicas.length}r`);
    if (db.postgres.patroni) parts.push("patroni");
    if (db.postgres.pgbouncer) parts.push("pgb");
    if (db.postgres.etcd) parts.push("etcd");
    topoLabel = parts.join(" + ");
  } else if (db.mysql) {
    const parts: string[] = ["primary"];
    if (db.mysql.replicas?.length) parts.push(`${db.mysql.replicas.length}r`);
    if (db.mysql.group_replication) parts.push("gr");
    if (db.mysql.proxysql) parts.push("proxysql");
    topoLabel = parts.join(" + ");
  } else if (db.picodata) {
    topoLabel = `${db.picodata.shards}sh rf=${db.picodata.replication_factor}`;
  } else if (db.ydb) {
    const parts: string[] = [`${db.ydb.storage.count} storage`];
    if (db.ydb.database) parts.push(`${db.ydb.database.count} database`);
    else parts.push("combined");
    if (db.ydb.haproxy) parts.push("haproxy");
    topoLabel = parts.join(" + ");
  } else if (db.ydb_managed) {
    const m = db.ydb_managed;
    const parts: string[] = [`managed (${m.type})`];
    if (m.type === "dedicated" && m.resource_preset_id) parts.push(m.resource_preset_id);
    topoLabel = parts.join(" + ");
  }

  return (
    <div className="flex flex-col h-full">
      {/* Run ID + timing */}
      <div className="px-3 py-2 border-b border-zinc-800/50">
        <div className="text-[10px] font-mono text-zinc-600 truncate" title={config.id}>{config.id}</div>
        <div className="flex items-center gap-2 mt-1 text-[10px] font-mono text-zinc-500">
          {startedAt && formatTs(startedAt) && (
            <span className="flex items-center gap-1">
              <Clock className="w-3 h-3 text-zinc-600" />
              {formatTs(startedAt)}
            </span>
          )}
        </div>
        {startedAt && (
          <div className="flex items-center gap-1 mt-0.5 text-[10px] font-mono text-zinc-500">
            <Timer className="w-3 h-3 text-zinc-600" />
            {isRunning ? (
              <ElapsedTimer startedAt={startedAt} />
            ) : finishedAt && formatTs(finishedAt) ? (
              (() => {
                const s = new Date(startedAt).getTime();
                const e = new Date(finishedAt).getTime();
                const sec = Math.max(0, Math.round((e - s) / 1000));
                if (sec < 60) return `${sec}s`;
                if (sec < 3600) return `${Math.floor(sec / 60)}m ${sec % 60}s`;
                return `${Math.floor(sec / 3600)}h ${Math.floor((sec % 3600) / 60)}m`;
              })()
            ) : "—"}
          </div>
        )}
      </div>

      {/* Database section */}
      <div className="px-3 py-2 border-b border-zinc-800/50">
        <div className="text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5">Database</div>
        <div className="flex items-baseline gap-2">
          <span className="text-sm font-semibold font-mono text-zinc-100">{db.kind}</span>
          <span className="text-xs font-mono text-zinc-500">v{db.version}</span>
        </div>
        {topoLabel && (
          <div className="text-[11px] font-mono text-zinc-500 mt-0.5">{topoLabel}</div>
        )}
        {config.preset_id && (
          <div className="text-[11px] font-mono text-zinc-500 mt-0.5" title={config.preset_id}>
            preset: {presetName ?? config.preset_id.slice(0, 8)}
          </div>
        )}
      </div>

      {/* Managed YDB params (only when YC-managed) */}
      {db.ydb_managed && (
        <div className="px-3 py-2 border-b border-zinc-800/50">
          <div className="text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5">Managed YDB</div>
          <div className="space-y-0">
            <ConfigLine label="type" value={db.ydb_managed.type} icon={Tag} />
            {db.ydb_managed.type === "dedicated" && db.ydb_managed.compute_type && (
              <ConfigLine label="workload" value={db.ydb_managed.compute_type.toUpperCase()} icon={Zap} />
            )}
            {db.ydb_managed.type === "dedicated" && db.ydb_managed.resource_preset_id && (
              <ConfigLine label="preset" value={db.ydb_managed.resource_preset_id} icon={Cpu} />
            )}
            {db.ydb_managed.type === "dedicated" && db.ydb_managed.auto_scale && (
              <ConfigLine
                label="scale"
                value={`auto ${db.ydb_managed.auto_scale.min_size}-${db.ydb_managed.auto_scale.max_size} @ ${db.ydb_managed.auto_scale.cpu_utilization_percent ?? 70}% CPU`}
                icon={Users}
              />
            )}
            {db.ydb_managed.type === "dedicated" && !db.ydb_managed.auto_scale && db.ydb_managed.node_count != null && (
              <ConfigLine label="nodes" value={String(db.ydb_managed.node_count)} icon={Users} />
            )}
            {db.ydb_managed.type === "dedicated" && db.ydb_managed.storage_groups != null && (
              <ConfigLine label="storage groups" value={String(db.ydb_managed.storage_groups)} icon={HardDrive} />
            )}
            {db.ydb_managed.type === "dedicated" && db.ydb_managed.storage_type && (
              <ConfigLine label="storage type" value={db.ydb_managed.storage_type} icon={HardDrive} />
            )}
            {db.ydb_managed.type === "serverless" && db.ydb_managed.throttling_rcus != null && (
              <ConfigLine label="throttling" value={`${db.ydb_managed.throttling_rcus} RCU`} icon={Zap} />
            )}
            {db.ydb_managed.endpoint && (
              <ConfigLine label="endpoint" value={db.ydb_managed.endpoint} icon={Network} />
            )}
            {db.ydb_managed.database_path && (
              <ConfigLine label="db path" value={db.ydb_managed.database_path} icon={Database} />
            )}
          </div>
          {/* Terraform snapshot — what actually landed in the cloud. */}
          {db.ydb_managed.terraform_output && (
            <div className="mt-2 pt-2 border-t border-zinc-800/50 space-y-0">
              <div className="text-[10px] text-zinc-600 uppercase tracking-wider mb-1">Terraform</div>
              <ConfigLine label="db id" value={db.ydb_managed.terraform_output.id} icon={Tag} />
              <ConfigLine label="name" value={db.ydb_managed.terraform_output.name} icon={Tag} />
              <ConfigLine label="status" value={db.ydb_managed.terraform_output.status} icon={Zap} />
              <ConfigLine label="folder" value={db.ydb_managed.terraform_output.folder_id} icon={Server} />
              <ConfigLine label="location" value={db.ydb_managed.terraform_output.location_id} icon={Server} />
              {db.ydb_managed.terraform_output.ydb_full_endpoint && (
                <ConfigLine label="full endpoint" value={db.ydb_managed.terraform_output.ydb_full_endpoint} icon={Network} />
              )}
              {db.ydb_managed.terraform_output.document_api_endpoint && (
                <ConfigLine label="doc api" value={db.ydb_managed.terraform_output.document_api_endpoint} icon={Network} />
              )}
              <ConfigLine label="tls" value={db.ydb_managed.terraform_output.tls_enabled ? "yes" : "no"} icon={Tag} />
              {db.ydb_managed.terraform_output.created_at && (
                <ConfigLine label="created" value={db.ydb_managed.terraform_output.created_at} icon={Clock} />
              )}
              {db.ydb_managed.terraform_output.network_id && (
                <ConfigLine label="network" value={db.ydb_managed.terraform_output.network_id} icon={Network} />
              )}
              {db.ydb_managed.terraform_output.subnet_ids && db.ydb_managed.terraform_output.subnet_ids.length > 0 && (
                <ConfigLine label="subnets" value={db.ydb_managed.terraform_output.subnet_ids.join(", ")} icon={Network} />
              )}
            </div>
          )}
        </div>
      )}

      {/* Topology card */}
      {topology && (
        <div className="px-3 py-2 border-b border-zinc-800/50">
          <div className="text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5">Topology</div>
          <TopologyDiagram kind={db.kind as DatabaseKind} topology={topology} />
        </div>
      )}

      {/* Workload section */}
      <div className="px-3 py-2 border-b border-zinc-800/50">
        <div className="text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5">Workload</div>
        <div className="space-y-0">
          <ConfigLine label="script" value={script} icon={FileCode} />
          {s.version && <ConfigLine label="stroppy" value={`v${s.version}`} icon={Tag} />}
          {s.sql && <ConfigLine label="sql" value={s.sql} icon={FileCode} />}
          {/* k6 driving knob: duration vs iterations. Show whichever the
              user picked; the unselected one is omitted to avoid clutter. */}
          {s.k6_mode === "iterations" ? (
            <ConfigLine label="iterations" value={String(s.iterations ?? 0)} icon={Repeat} />
          ) : (
            <ConfigLine label="duration" value={s.duration} icon={Clock} />
          )}
          {vus > 0 && <ConfigLine label="VUs" value={String(vus)} icon={Users} />}
          {(s.pool_size ?? 0) > 0 && <ConfigLine label="pool" value={String(s.pool_size)} icon={Network} />}
          {(s.scale_factor ?? 0) > 0 && <ConfigLine label="scale" value={String(s.scale_factor)} />}
          {s.default_insert_method && s.default_insert_method !== "native" && (
            <ConfigLine label="insert" value={s.default_insert_method} icon={Wand2} />
          )}
          {s.quiet === false && <ConfigLine label="quiet" value="off" icon={VolumeX} />}
          {s.no_thresholds && <ConfigLine label="thresholds" value="off" icon={AlertOctagon} />}
          {s.env && Object.keys(s.env).length > 0 && (
            <div className="text-[10px] font-mono text-zinc-600 mt-1">
              env: {Object.entries(s.env).filter(([, v]) => v !== "").map(([k, v]) => `${k}=${v}`).join(", ") || "(none)"}
            </div>
          )}
          {s.files && s.files.length > 0 && (
            <div className="flex items-start gap-1.5 mt-1 text-[10px] font-mono text-zinc-600">
              <FileText className="w-3 h-3 mt-0.5 shrink-0" />
              <span className="truncate" title={s.files.map((f) => f.name).join(", ")}>
                {s.files.length} file{s.files.length === 1 ? "" : "s"}: {s.files.map((f) => f.name).join(", ")}
              </span>
            </div>
          )}
          {s.steps && s.steps.length > 0 && (
            <div className="text-[10px] font-mono text-zinc-600 mt-1">
              steps: {s.steps.join(", ")}
            </div>
          )}
          {s.no_steps && s.no_steps.length > 0 && (
            <div className="text-[10px] font-mono text-zinc-600 mt-0.5">
              skip: {s.no_steps.join(", ")}
            </div>
          )}
        </div>
      </div>

      {/* Infra section */}
      <div className="px-3 py-2 border-b border-zinc-800/50">
        <div className="text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5">Infrastructure</div>
        <ConfigLine label="provider" value={config.provider} icon={Container} />
        {config.platform_id && <ConfigLine label="platform" value={config.platform_id} />}
        <ConfigLine label="network" value={config.network?.cidr} icon={Network} />
        {config.network?.zone && <ConfigLine label="zone" value={config.network.zone} />}
      </div>

      {/* Machines */}
      {config.machines && config.machines.length > 0 && (
        <div className="px-3 py-2 border-b border-zinc-800/50">
          <div className="text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5">Machines</div>
          <div className="space-y-1">
            {config.machines.map((m, i) => (
              <MachineRow key={`${m.role}-${i}`} spec={m} />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

// ─── Effective config per group ──────────────────────────────────



function CfgRow({ k, v }: { k: string; v?: string | null }) {
  if (!v) return null;
  return (
    <div className="flex gap-2 text-[10px] font-mono leading-relaxed">
      <span className="text-zinc-600 shrink-0 w-20 text-right">{k}</span>
      <span className="text-zinc-400 truncate" title={v}>{v}</span>
    </div>
  );
}

// groupOwnsRenderedKey decides which accordion group should display a given
// rendered-configs key. Keys are shaped as "<file>" or "<file>:<role>" — see
// internal/domain/run/rendered_configs.go.
function groupOwnsRenderedKey(groupKey: string, renderedKey: string): boolean {
  switch (groupKey) {
    case "database":
      return renderedKey.startsWith("postgresql.conf") ||
        renderedKey === "pg_hba.conf" ||
        renderedKey.startsWith("my.cnf") ||
        renderedKey === "picodata.yaml" ||
        renderedKey.startsWith("ydb.yaml") ||
        renderedKey === "patroni.yml" ||
        renderedKey === "pgbouncer.ini";
    case "proxy":
      return renderedKey === "haproxy.cfg" || renderedKey === "proxysql.cnf";
    default:
      return false;
  }
}

// RenderedConfigBlock is a collapsible read-only view of a generated config
// file. Closed by default so a long postgresql.conf doesn't dominate the
// accordion; the user clicks the header to expand.
function RenderedConfigBlock({ name, body }: { name: string; body: string }) {
  const [open, setOpen] = useState(false);
  const lineCount = useMemo(() => (body ? body.split("\n").length : 0), [body]);
  return (
    <div className="border border-zinc-800/60 rounded">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="w-full flex items-center gap-2 px-2 py-1 text-[10px] font-mono text-zinc-300 hover:bg-zinc-900/50"
      >
        <span className="text-zinc-500">{open ? "▾" : "▸"}</span>
        <span className="flex-1 text-left">{name}</span>
        <span className="text-zinc-600 tabular-nums">{lineCount} lines</span>
      </button>
      {open && (
        <pre className="px-2 py-1.5 text-[10px] font-mono text-zinc-300 bg-[#0a0a0a] border-t border-zinc-800/40 overflow-x-auto whitespace-pre">{body}</pre>
      )}
    </div>
  );
}

// ─── DAG pipeline (vertical) ─────────────────────────────────────

// deriveGroupConfig backfills a synthetic config row map for a group when the
// snapshot's effective_configs map is missing or empty for that group. The
// stroppy benchmark group has its config in cfg.stroppy on every run, so we
// can always render it — older runs that finished before the agent reported
// effective_configs would otherwise show "config not available". Other groups
// get a small derived view from RunConfig too, since the snapshot always
// stores it. Real effective_configs (when present) take precedence: this is
// only the fallback.
function deriveGroupConfig(groupKey: string, cfg: RunConfig | null): Record<string, string> | null {
  if (!cfg) return null;
  const out: Record<string, string> = {};
  const put = (k: string, v: unknown) => {
    if (v === undefined || v === null) return;
    if (typeof v === "string" && v === "") return;
    if (typeof v === "number" && v === 0) return;
    out[k] = String(v);
  };
  switch (groupKey) {
    case "benchmark": {
      const s = cfg.stroppy || {};
      put("version", s.version);
      put("script", s.script || s.workload);
      put("sql", s.sql);
      put("protocol", s.protocol);
      put("duration", s.duration);
      put("k6_mode", s.k6_mode);
      put("iterations", s.iterations);
      put("vus", s.vus ?? s.vus_scale ?? s.workers);
      put("pool_size", s.pool_size);
      put("scale_factor", s.scale_factor);
      put("default_insert_method", s.default_insert_method);
      if (s.no_thresholds) put("no_thresholds", "true");
      if (s.quiet === false) put("quiet", "false");
      if (s.steps?.length) put("steps", s.steps.join(", "));
      if (s.no_steps?.length) put("no_steps", s.no_steps.join(", "));
      break;
    }
    case "infrastructure": {
      put("provider", cfg.provider);
      put("platform_id", cfg.platform_id);
      put("network.cidr", cfg.network?.cidr);
      put("network.zone", cfg.network?.zone);
      break;
    }
    case "database": {
      put("kind", cfg.database?.kind);
      put("version", cfg.database?.version);
      put("preset_id", cfg.preset_id);
      if (cfg.external_db?.endpoint) {
        put("external_db.endpoint", cfg.external_db.endpoint);
        put("external_db.database", cfg.external_db.database);
      }
      break;
    }
    case "monitoring": {
      put("metrics_endpoint", cfg.monitor?.metrics_endpoint);
      put("logs_endpoint", cfg.monitor?.logs_endpoint);
      break;
    }
  }
  return Object.keys(out).length > 0 ? out : null;
}

function DagPipeline({ nodes, cancelled, effectiveConfigs, renderedConfigs, runConfig, onViewLogs }: { nodes: NodeStatus[]; cancelled?: boolean; effectiveConfigs?: Record<string, Record<string, string>>; renderedConfigs?: Record<string, string>; runConfig?: RunConfig | null; onViewLogs?: (phase: string) => void }) {
  const allPhaseIds = useMemo(() => phaseGroups.flatMap((g) => g.phases), []);
  // Groups are collapsed by default. Persist user expansions to
  // localStorage so the choice sticks across reloads of the same run.
  const expandKey = "stroppy.runOverview.expandedGroups";
  const [expanded, setExpanded] = useState<Set<string>>(() => {
    try {
      const raw = localStorage.getItem(expandKey);
      if (raw) return new Set(JSON.parse(raw) as string[]);
    } catch { /* ignore */ }
    return new Set();
  });

  const toggle = (label: string) => setExpanded((prev) => {
    const next = new Set(prev);
    next.has(label) ? next.delete(label) : next.add(label);
    try {
      localStorage.setItem(expandKey, JSON.stringify(Array.from(next)));
    } catch { /* ignore */ }
    return next;
  });

  if (nodes.length === 0) {
    return (
      <div className="flex items-center justify-center h-full gap-2">
        <Loader2 className="w-4 h-4 text-zinc-600 animate-spin" />
        <span className="text-zinc-600 text-xs font-mono">Waiting...</span>
      </div>
    );
  }

  const nodeMap = new Map(nodes.map((n) => [n.id, n]));
  const activeGroups = phaseGroups
    .map((g) => ({ ...g, phases: g.phases.filter((p) => nodeMap.has(p)) }))
    .filter((g) => g.phases.length > 0);

  const totalPhases = activeGroups.reduce((s, g) => s + g.phases.length, 0);
  const donePhases = nodes.filter((n) => n.status === "done").length;
  const failedPhases = nodes.filter((n) => n.status === "failed").length;

  return (
    <div className="h-full flex flex-col overflow-hidden">
      {/* Progress bar */}
      <div className="flex items-center gap-3 px-3 py-2 border-b border-zinc-800/50 shrink-0">
        <div className="flex-1 h-1 bg-zinc-800/80 rounded-full overflow-hidden">
          <div
            className={`h-full transition-all duration-700 ease-out rounded-full ${
              failedPhases > 0 ? (cancelled ? "bg-zinc-500" : "bg-red-500") : donePhases === totalPhases ? "bg-emerald-500" : "bg-amber-500"
            }`}
            style={{ width: `${totalPhases > 0 ? (donePhases / totalPhases) * 100 : 0}%` }}
          />
        </div>
        <span className="text-[11px] font-mono text-zinc-500 tabular-nums shrink-0">
          {donePhases}/{totalPhases}
          {failedPhases > 0 && <span className={`ml-1 ${cancelled ? "text-zinc-500" : "text-red-500"}`}>{cancelled ? `${failedPhases} cancelled` : `${failedPhases} err`}</span>}
        </span>
      </div>

      {/* Scrollable pipeline */}
      <div className="flex-1 min-h-0 overflow-y-auto px-3 py-2">
        {activeGroups.map((group, gi) => {
          const rawGStatus = groupStatus(group.phases, nodeMap);
          const gStatus = cancelled && rawGStatus === "failed" ? "cancelled" : rawGStatus;
          const colors = statusStyles[gStatus as keyof typeof statusStyles] || statusStyles.pending;
          const doneInGroup = group.phases.filter((p) => nodeMap.get(p)?.status === "done").length;
          const GroupIcon = group.icon;
          const isLast = gi === activeGroups.length - 1;
          const isExpanded = expanded.has(group.label);

          return (
            <div key={group.label}>
              {/* Connector between groups */}
              {gi > 0 && (
                <div className="flex justify-start pl-[7px]">
                  <div className={`w-px h-2 ${colors.connector} transition-colors duration-500`} />
                </div>
              )}

              {/* Group */}
              <div className={`border ${colors.border} ${colors.bg} transition-all duration-300`}>
                {/* Group header — clickable */}
                <button
                  type="button"
                  onClick={() => toggle(group.label)}
                  className="flex items-center gap-2 px-2.5 py-1.5 w-full text-left cursor-pointer hover:bg-white/[0.02] transition-colors"
                >
                  <GroupIcon className={`w-3.5 h-3.5 ${colors.text} shrink-0`} />
                  <span className="text-xs font-semibold font-mono text-zinc-200 flex-1 truncate">
                    {group.label}
                  </span>
                  <span className="text-[10px] font-mono text-zinc-600 tabular-nums">
                    {doneInGroup}/{group.phases.length}
                  </span>
                  <ChevronDown className={`w-3 h-3 text-zinc-600 transition-transform duration-200 ${isExpanded ? "rotate-180" : ""}`} />
                </button>

                {/* Expanded content: steps + config laid out side-by-side
                    so the wide right pane gets used. On narrow viewports
                    the flex wraps to a column automatically. */}
                {isExpanded && (
                  <div className="border-t border-zinc-800/30 flex flex-col md:flex-row md:divide-x md:divide-zinc-800/30">
                    {/* Steps */}
                    <div className="px-2.5 py-1 space-y-px md:w-72 md:shrink-0">
                      {group.phases.map((phaseId) => {
                        const node = nodeMap.get(phaseId);
                        if (!node) return null;
                        const active = isStepActive(phaseId, allPhaseIds, nodeMap);

                        return (
                          <div key={phaseId}>
                            <div className={`flex items-center gap-2 py-0.5 px-0.5 rounded-sm ${node.status === "running" ? "bg-blue-500/[0.06]" : active ? "bg-amber-500/[0.06]" : ""}`}>
                              <StepDot status={node.status} />
                              <span
                                className={`text-[11px] font-mono leading-tight truncate ${
                                  node.status === "done" ? "text-zinc-400"
                                    : node.status === "running" ? "text-blue-300"
                                    : node.status === "failed" ? "text-red-400"
                                    : node.status === "cancelled" ? "text-zinc-400"
                                    : active ? "text-amber-300" : "text-zinc-600"
                                }`}
                                title={humanPhase(phaseId)}
                              >
                                {humanPhase(phaseId)}
                              </span>
                            </div>

                            {/* Error — full height, copyable, with log link */}
                            {(node.status === "failed" || node.status === "cancelled") && node.error && (
                              <div className="mt-0.5 mb-1 ml-5">
                                <div className={`p-1.5 text-[11px] font-mono leading-relaxed select-text whitespace-pre-wrap break-all ${
                                  node.status === "cancelled"
                                    ? "bg-zinc-500/5 border border-zinc-500/20 text-zinc-400/80"
                                    : "bg-red-500/5 border border-red-500/20 text-red-400/80"
                                }`}>
                                  {node.error}
                                </div>
                                <div className="flex items-center gap-2 mt-1">
                                  <CopyButton text={node.error} />
                                  {onViewLogs && (
                                    <button
                                      type="button"
                                      onClick={() => onViewLogs(phaseId)}
                                      className="text-[9px] font-mono text-primary hover:underline cursor-pointer"
                                    >
                                      View in Logs
                                    </button>
                                  )}
                                </div>
                              </div>
                            )}
                          </div>
                        );
                      })}
                    </div>

                    {/* Effective config + rendered config files for this group.
                        Falls back to a synthetic config derived from RunConfig
                        when the snapshot has no effective_configs entry — old
                        runs and runs whose agent didn't report still get the
                        benchmark/database/infra summary. */}
                    {(() => {
                      const groupKey = group.label.toLowerCase();
                      const stored = effectiveConfigs?.[groupKey] || deriveGroupConfig(groupKey, runConfig ?? null);
                      const renderedKeys = renderedConfigs
                        ? Object.keys(renderedConfigs).filter((k) => groupOwnsRenderedKey(groupKey, k))
                        : [];
                      const hasAnything = stored || renderedKeys.length > 0;
                      if (!hasAnything) {
                        return (
                          <div className="px-2.5 py-1.5 bg-zinc-900/30 flex-1 min-w-0 border-t md:border-t-0 border-zinc-800/20">
                            <span className="text-[10px] font-mono text-zinc-700">config not available for this run</span>
                          </div>
                        );
                      }
                      return (
                        <div className="px-2.5 py-1.5 bg-zinc-900/30 space-y-2 flex-1 min-w-0 border-t md:border-t-0 border-zinc-800/20">
                          {stored && (
                            <div className="space-y-0.5">
                              {Object.entries(stored).map(([k, v]) => <CfgRow key={k} k={k} v={v} />)}
                            </div>
                          )}
                          {renderedKeys.map((key) => (
                            <RenderedConfigBlock key={key} name={key} body={renderedConfigs![key]} />
                          ))}
                        </div>
                      );
                    })()}
                  </div>
                )}
              </div>

              {/* Bottom connector */}
              {!isLast && (
                <div className="flex justify-start pl-[7px]">
                  <div className={`w-px h-1.5 ${gStatus === "done" ? "bg-emerald-500/30" : gStatus === "running" ? "bg-amber-500/30" : gStatus === "cancelled" ? "bg-zinc-600/30" : "bg-zinc-800"} transition-colors duration-500`} />
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

// ─── Main export ─────────────────────────────────────────────────

type RunStatusValue = "running" | "cancelling" | "cancelled" | "failed" | "completed" | "pending";

interface RunOverviewProps {
  nodes: NodeStatus[];
  runStatus?: RunStatusValue;
  onViewLogs?: (phase: string) => void;
  snapshot?: {
    started_at?: string;
    finished_at?: string;
    state?: {
      provider?: string;
      run_config?: Record<string, unknown> | string;
      effective_configs?: Record<string, Record<string, string>>;
    };
  } | null;
  // Rendered config files (postgresql.conf, ydb.yaml, …) keyed like
  // BuildRenderedConfigs ships them: "postgresql.conf:master", "my.cnf:primary",
  // "ydb.yaml:storage", "pgbouncer.ini", "haproxy.cfg", etc. Shown read-only
  // in the database accordion so the user can verify what landed on the box
  // even mid-run, before the agent has reported back its effective_configs.
  renderedConfigs?: Record<string, string>;
}

export function RunOverview({ nodes, snapshot, runStatus, onViewLogs, renderedConfigs }: RunOverviewProps) {
  const config = useMemo<RunConfig | null>(() => {
    const rc = snapshot?.state?.run_config;
    if (!rc) return null;
    try {
      return typeof rc === "string" ? JSON.parse(rc) : (rc as unknown as RunConfig);
    } catch {
      return null;
    }
  }, [snapshot]);

  // Resizable config panel — width persisted in localStorage so the user's
  // preference survives page reloads. Range 200..640px.
  const [configWidth, setConfigWidth] = useState<number>(() => {
    const stored = parseInt(localStorage.getItem("stroppy.runOverview.configWidth") ?? "");
    return Number.isFinite(stored) && stored >= 200 && stored <= 640 ? stored : 240;
  });
  useEffect(() => {
    localStorage.setItem("stroppy.runOverview.configWidth", String(configWidth));
  }, [configWidth]);
  const dragStateRef = useRef<{ startX: number; startW: number } | null>(null);
  const onDragStart = (e: React.MouseEvent) => {
    e.preventDefault();
    dragStateRef.current = { startX: e.clientX, startW: configWidth };
    const onMove = (ev: MouseEvent) => {
      const ds = dragStateRef.current;
      if (!ds) return;
      const next = Math.min(640, Math.max(200, ds.startW + (ev.clientX - ds.startX)));
      setConfigWidth(next);
    };
    const onUp = () => {
      dragStateRef.current = null;
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    };
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
  };

  return (
    <div className="h-full flex overflow-hidden">
      {/* Left — Config panel (resizable) */}
      <div
        className="shrink-0 border-r border-zinc-800/50 overflow-auto"
        style={{ width: configWidth }}
      >
        {/* Run status + phase counters */}
        {runStatus && (
          <div className="px-3 py-2 border-b border-zinc-800/50 space-y-1">
            <div className={`text-xs font-mono font-semibold flex items-center gap-1.5 ${
              runStatus === "running" ? "text-blue-400" :
              runStatus === "cancelling" ? "text-amber-400" :
              runStatus === "cancelled" ? "text-zinc-400" :
              runStatus === "failed" ? "text-red-400" :
              runStatus === "completed" ? "text-emerald-400" :
              "text-zinc-500"
            }`}>
              <span className={`inline-block w-2 h-2 rounded-full ${
                runStatus === "running" ? "bg-blue-400 animate-pulse" :
                runStatus === "cancelling" ? "bg-amber-400 animate-pulse" :
                runStatus === "cancelled" ? "bg-zinc-500" :
                runStatus === "failed" ? "bg-red-500" :
                runStatus === "completed" ? "bg-emerald-500" :
                "bg-zinc-600"
              }`} />
              {runStatus === "running" ? "Running" :
               runStatus === "cancelling" ? "Cancelling..." :
               runStatus === "cancelled" ? "Cancelled" :
               runStatus === "failed" ? "Failed" :
               runStatus === "completed" ? "Completed" :
               "Pending"}
            </div>
            <div className="flex items-center gap-2.5 text-[10px] font-mono text-zinc-500">
              {(() => {
                const done = nodes.filter(n => n.status === "done").length;
                const running = nodes.filter(n => n.status === "running").length;
                const failed = nodes.filter(n => n.status === "failed").length;
                const cancelled = nodes.filter(n => n.status === "cancelled").length;
                const pending = nodes.filter(n => n.status === "pending").length;
                return (
                  <>
                    {done > 0 && <span className="flex items-center gap-1"><span className="w-1.5 h-1.5 rounded-full bg-emerald-500 inline-block" />{done}</span>}
                    {running > 0 && <span className="flex items-center gap-1"><span className="w-1.5 h-1.5 rounded-full bg-blue-400 animate-pulse inline-block" />{running}</span>}
                    {failed > 0 && <span className="flex items-center gap-1"><span className="w-1.5 h-1.5 rounded-full bg-red-500 inline-block" />{failed}</span>}
                    {cancelled > 0 && <span className="flex items-center gap-1"><span className="w-1.5 h-1.5 rounded-full bg-zinc-500 inline-block" />{cancelled}</span>}
                    {pending > 0 && <span className="flex items-center gap-1"><span className="w-1.5 h-1.5 rounded-full bg-zinc-700 inline-block" />{pending}</span>}
                  </>
                );
              })()}
            </div>
          </div>
        )}
        <ConfigPanel
          config={config}
          startedAt={snapshot?.started_at}
          finishedAt={snapshot?.finished_at}
          isRunning={runStatus === "running" || runStatus === "cancelling"}
        />
      </div>

      {/* Drag handle — resize config panel horizontally. */}
      <div
        onMouseDown={onDragStart}
        title="Drag to resize"
        className="w-1 shrink-0 cursor-col-resize bg-transparent hover:bg-zinc-700/40 active:bg-zinc-600/60 transition-colors"
      />

      {/* Right — DAG pipeline */}
      <div className="flex-1 min-w-0">
        <DagPipeline nodes={nodes} cancelled={runStatus === "cancelled"} effectiveConfigs={snapshot?.state?.effective_configs} renderedConfigs={renderedConfigs} runConfig={config} onViewLogs={onViewLogs} />
      </div>
    </div>
  );
}
