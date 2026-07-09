// Shared, typed per-engine database parameter editor.
//
// This is the single source of truth for editing a cloud.v1.domain.Database
// (engine Kind + version + the typed per-engine Params). It is consumed by BOTH
// the New Run wizard's Database step (src/pages/NewRun.tsx) AND the Database
// Preset authoring form (src/pages/library/DatabasePresetForm.tsx), so the two
// surfaces stay byte-for-byte consistent with the proto's params messages.
//
// It also exports the small primitives the editor needs (ToggleRow /
// ConfigField / FieldErrors; the scrubbable NumField now lives in
// @/components/ui/num-field), the per-engine version catalog (DB_VERSIONS), the
// derived-topology summary (engineNodes / topologySummary — mirrors the server
// topology builder), and a from-the-wire database validator (validateDatabase —
// mirrors cloud.v1.domain.Database.Validate). Keeping these together means the
// preset form can offer the same validation + topology preview the wizard does
// without depending on the wizard page.

import { useEffect, useMemo, useRef, useState } from "react";
import {
  AlertCircle,
  AlertTriangle,
  ChevronDown,
  Info,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { NumField } from "@/components/ui/num-field";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import { ConfigEditor } from "@/components/ui/config-editor";
import {
  YdbParams_FaultTolerance,
  YdbParams_DiskType,
  YdbManagedParams_Type,
  YdbManagedParams_ComputeType,
  type DatabaseVM,
  type EngineKind,
  type PostgresParamsVM,
  type MySqlParamsVM,
  type PicodataParamsVM,
  type YdbParamsVM,
  type YdbManagedParamsVM,
  type CockroachParamsVM,
  type OrioledbParamsVM,
  type PgNoopParamsVM,
  type DraftErrorVM,
} from "@/services/wizard";

// ─── Engine version catalog (DatabaseParams.version options per engine) ────────

export const DB_VERSIONS: Record<EngineKind, string[]> = {
  postgres: ["17", "16", "15"],
  mysql: ["8.4", "8.0"],
  mariadb: ["11.4", "10.11"],
  picodata: ["25.3"],
  ydb: ["25.2", "24.4"],
  ydbManaged: ["managed"],
  cockroach: ["24.2", "23.2"],
  orioledb: ["pg18", "pg17", "pg16"],
  noop: [],
  pgnoop: ["0.1.2"],
  external: [],
};

// ─── Small form primitives (shared with the wizard) ────────────────────────────

export function ToggleRow({
  label,
  hint,
  checked,
  onChange,
}: {
  label: string;
  hint?: string;
  checked: boolean;
  onChange: (b: boolean) => void;
}) {
  return (
    <div className="flex min-w-0 items-center justify-between gap-3 border border-zinc-800/60 bg-[#0a0a0a] px-3 py-2.5">
      <div className="min-w-0">
        <div className="truncate text-sm text-foreground">{label}</div>
        {hint && <div className="text-[11px] leading-snug text-zinc-600">{hint}</div>}
      </div>
      <Switch className="shrink-0" checked={checked} onCheckedChange={onChange} />
    </div>
  );
}

function severityTone(s: DraftErrorVM["severity"]): string {
  return s === "error" ? "text-red-400" : s === "warning" ? "text-amber-400" : "text-zinc-400";
}

function SeverityIcon({ severity, className }: { severity: DraftErrorVM["severity"]; className?: string }) {
  if (severity === "error") return <AlertCircle className={className} />;
  if (severity === "warning") return <AlertTriangle className={className} />;
  return <Info className={className} />;
}

/**
 * Renders the FieldError-shaped DraftErrors for a section as a FIXED-HEIGHT,
 * expandable row. Validation re-runs on (debounced) every edit, so a row that
 * grew/shrank with the error count — or vanished entirely when valid — made the
 * form jump as the user typed. Instead we always reserve one line: when valid
 * the line is an empty spacer; when not, it's a one-line summary (worst-severity
 * message + an "+N more" count) that expands on demand to the full list.
 */
export function FieldErrors({ errs }: { errs: DraftErrorVM[] }) {
  const [open, setOpen] = useState(false);
  if (errs.length === 0) return <div className="mt-2 h-6" aria-hidden />;

  // Surface the worst severity first (error > warning > info).
  const rank = (s: DraftErrorVM["severity"]) => (s === "error" ? 0 : s === "warning" ? 1 : 2);
  const sorted = [...errs].sort((a, b) => rank(a.severity) - rank(b.severity));
  const top = sorted[0];

  return (
    <div className="mt-2">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className={`flex h-6 w-full items-center gap-1.5 text-left text-[11px] ${severityTone(top.severity)}`}
      >
        <SeverityIcon severity={top.severity} className="h-3 w-3 shrink-0" />
        <span className="truncate">{top.message}</span>
        {errs.length > 1 && (
          <span className="shrink-0 text-zinc-600">+{errs.length - 1} more</span>
        )}
        <ChevronDown className={`ml-auto h-3 w-3 shrink-0 text-zinc-600 transition-transform ${open ? "rotate-180" : ""}`} />
      </button>
      {open && (
        <div className="mt-1 space-y-1 border-l border-zinc-800/70 pl-2">
          {sorted.map((e, i) => (
            <div key={i} className={`flex items-start gap-1.5 text-[11px] ${severityTone(e.severity)}`}>
              <SeverityIcon severity={e.severity} className="mt-px h-3 w-3 shrink-0" />
              <span>
                <span className="font-mono text-[10px] text-zinc-600">{e.field}</span> · {e.message}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

/** filter a flat DraftError list down to one section (e.g. "database"). */
export function errorsFor(errs: DraftErrorVM[], prefix: string): DraftErrorVM[] {
  return errs.filter((e) => e.field === prefix || e.field.startsWith(prefix + "."));
}

// A key=value config field backed by the dark ConfigEditor.
export function ConfigField({
  filename,
  label,
  description = "Optional key/value options for this engine.",
  initiallyOpen = false,
  value,
  onChange,
}: {
  filename: string;
  label: string;
  description?: string;
  initiallyOpen?: boolean;
  value: Record<string, string>;
  onChange: (m: Record<string, string>) => void;
}) {
  const [open, setOpen] = useState(initiallyOpen);
  const entryCount = Object.keys(value).length;
  const text = useMemo(
    () =>
      Object.entries(value)
        .map(([k, v]) => `${k} = ${v}`)
        .join("\n"),
    [value],
  );
  const [draftText, setDraftText] = useState(text);
  const lastApplied = useRef(text);
  useEffect(() => {
    if (text !== lastApplied.current) {
      setDraftText(text);
      lastApplied.current = text;
    }
  }, [text]);

  return (
    <div className="overflow-hidden border border-zinc-800/70 bg-[#070707]">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-start gap-3 px-3 py-2.5 text-left transition-colors hover:bg-zinc-900/40"
      >
        <ChevronDown className={`mt-0.5 h-3.5 w-3.5 shrink-0 text-zinc-500 transition-transform ${open ? "rotate-180" : ""}`} />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <span className="text-sm font-medium text-foreground">{label}</span>
            <span className="font-mono text-[10px] uppercase tracking-wide text-zinc-600">
              {entryCount} option{entryCount === 1 ? "" : "s"}
            </span>
          </div>
          <p className="mt-0.5 text-[11px] leading-snug text-zinc-600">{description}</p>
        </div>
      </button>
      {open && (
        <div className="border-t border-zinc-800/70 p-3">
        <ConfigEditor
          filename={filename}
          value={draftText}
          height="clamp(10rem, 28vh, 22rem)"
          onChange={(next) => {
            setDraftText(next);
            const map: Record<string, string> = {};
            for (const line of next.split("\n")) {
              const t = line.trim();
              if (!t || t.startsWith("#") || t.startsWith("[")) continue;
              const idx = t.indexOf("=");
              if (idx < 0) continue;
              map[t.slice(0, idx).trim()] = t.slice(idx + 1).trim();
            }
            lastApplied.current = next;
            onChange(map);
          }}
        />
      </div>
      )}
    </div>
  );
}

// ─── Engine version select (shown for self-deploy engines) ─────────────────────

export function EngineVersionSelect({
  db,
  apply,
}: {
  db: DatabaseVM;
  apply: (d: DatabaseVM) => void;
}) {
  if (db.kind === "external" || db.kind === "ydbManaged" || db.kind === "noop") return null;
  return (
    <div className="max-w-xs">
      <Label>Engine version</Label>
      <Select value={db.version} onValueChange={(v) => apply({ ...db, version: v })}>
        <SelectTrigger className="mt-1">
          <SelectValue placeholder="Pick a version" />
        </SelectTrigger>
        <SelectContent>
          {DB_VERSIONS[db.kind].map((v) => (
            <SelectItem key={v} value={v}>
              {v}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

// ─── The typed per-engine parameter editor ─────────────────────────────────────

export function EngineParamsForm({
  db,
  apply,
  advancedInitiallyOpen = false,
}: {
  db: DatabaseVM;
  apply: (d: DatabaseVM) => void;
  advancedInitiallyOpen?: boolean;
}) {
  const e = db.params;
  switch (e.kind) {
    case "postgres": {
      const p = e.postgres;
      const set = (patch: Partial<PostgresParamsVM>) =>
        apply({ ...db, params: { kind: "postgres", postgres: { ...p, ...patch } } });
      return (
        <div className="space-y-4">
          <div className="grid grid-cols-1 gap-3 @lg:grid-cols-2 @3xl:grid-cols-3">
            <NumField label="Replicas" value={p.replicas} onChange={(n) => set({ replicas: n })} hint="streaming standbys" />
            <NumField label="Sync replicas" value={p.syncReplicas} onChange={(n) => set({ syncReplicas: n })} hint="synchronous standbys" />
            <NumField label="HAProxy nodes" value={p.haproxy} onChange={(n) => set({ haproxy: n })} hint="dedicated LB" />
          </div>
          <div className="grid grid-cols-1 gap-2 @lg:grid-cols-2 @3xl:grid-cols-3">
            <ToggleRow label="PgBouncer" hint="colocated pooler" checked={p.pgbouncer} onChange={(b) => set({ pgbouncer: b })} />
            <ToggleRow label="Patroni HA" hint="needs etcd" checked={p.patroni} onChange={(b) => set({ patroni: b })} />
            <ToggleRow label="etcd" hint="DCS for Patroni" checked={p.etcd} onChange={(b) => set({ etcd: b })} />
          </div>
          <ConfigField
            filename="postgresql.conf"
            label="Advanced PostgreSQL options"
            description="Master-node postgresql.conf key/value options."
            initiallyOpen={advancedInitiallyOpen}
            value={p.masterOptions}
            onChange={(m) => set({ masterOptions: m })}
          />
        </div>
      );
    }
    case "mysql":
    case "mariadb": {
      const p = e.kind === "mysql" ? e.mysql : e.mariadb;
      const set = (patch: Partial<MySqlParamsVM>) =>
        apply({ ...db, params: e.kind === "mysql" ? { kind: "mysql", mysql: { ...p, ...patch } } : { kind: "mariadb", mariadb: { ...p, ...patch } } });
      return (
        <div className="space-y-4">
          <div className="grid grid-cols-1 gap-3 @lg:grid-cols-2">
            <NumField label="Replicas" value={p.replicas} onChange={(n) => set({ replicas: n })} />
            <NumField label="ProxySQL nodes" value={p.proxysql} onChange={(n) => set({ proxysql: n })} hint="dedicated proxy" />
          </div>
          <div className="grid grid-cols-1 gap-2 @lg:grid-cols-2">
            <ToggleRow label="Group replication" checked={p.groupReplication} onChange={(b) => set({ groupReplication: b })} />
            <ToggleRow label="Semi-sync" hint="when group repl off" checked={p.semiSync} onChange={(b) => set({ semiSync: b })} />
          </div>
          <ConfigField
            filename="my.cnf"
            label="Advanced MySQL/MariaDB options"
            description="Primary-node my.cnf key/value options."
            initiallyOpen={advancedInitiallyOpen}
            value={p.primaryOptions}
            onChange={(m) => set({ primaryOptions: m })}
          />
        </div>
      );
    }
    case "picodata": {
      const p = e.picodata;
      const set = (patch: Partial<PicodataParamsVM>) =>
        apply({ ...db, params: { kind: "picodata", picodata: { ...p, ...patch } } });
      return (
        <div className="space-y-4">
          <div className="grid grid-cols-1 gap-3 @lg:grid-cols-2 @3xl:grid-cols-4">
            <NumField label="Instances" value={p.instances} onChange={(n) => set({ instances: n })} min={1} />
            <NumField label="Replication" value={p.replicationFactor} onChange={(n) => set({ replicationFactor: n })} hint="factor" />
            <NumField label="Shards" value={p.shards} onChange={(n) => set({ shards: n })} />
            <NumField label="HAProxy" value={p.haproxy} onChange={(n) => set({ haproxy: n })} />
          </div>
          <ConfigField
            filename="picodata.yaml"
            label="Advanced Picodata options"
            description="Instance-level YAML options rendered into the cluster config."
            initiallyOpen={advancedInitiallyOpen}
            value={p.instanceOptions}
            onChange={(m) => set({ instanceOptions: m })}
          />
        </div>
      );
    }
    case "ydb": {
      const p = e.ydb;
      const set = (patch: Partial<YdbParamsVM>) =>
        apply({ ...db, params: { kind: "ydb", ydb: { ...p, ...patch } } });
      return (
        <div className="space-y-4">
          <div className="grid grid-cols-1 gap-3 @lg:grid-cols-2 @3xl:grid-cols-3">
            <NumField label="Storage nodes" value={p.storageNodes} onChange={(n) => set({ storageNodes: n })} min={1} />
            <NumField label="Database nodes" value={p.databaseNodes} onChange={(n) => set({ databaseNodes: n })} hint="0 = combined" />
            <NumField label="HAProxy" value={p.haproxy} onChange={(n) => set({ haproxy: n })} />
          </div>
          <div className="grid grid-cols-1 gap-3 @lg:grid-cols-2 @3xl:grid-cols-3">
            <NumField label="pdisks / node" value={p.pdisksPerStorageNode} onChange={(n) => set({ pdisksPerStorageNode: n })} />
            <NumField label="Storage groups" value={p.storageGroups} onChange={(n) => set({ storageGroups: n })} />
            <div>
              <Label>Fault tolerance</Label>
              <Select value={String(p.faultTolerance)} onValueChange={(v) => set({ faultTolerance: Number(v) as YdbParams_FaultTolerance })}>
                <SelectTrigger className="mt-1">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={String(YdbParams_FaultTolerance.NONE)}>none</SelectItem>
                  <SelectItem value={String(YdbParams_FaultTolerance.BLOCK_4_2)}>block-4-2</SelectItem>
                  <SelectItem value={String(YdbParams_FaultTolerance.MIRROR_3_DC)}>mirror-3-dc</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="grid grid-cols-1 gap-3 @lg:grid-cols-2">
            <div>
              <Label>Disk type</Label>
              <Select value={String(p.defaultDiskType)} onValueChange={(v) => set({ defaultDiskType: Number(v) as YdbParams_DiskType })}>
                <SelectTrigger className="mt-1">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={String(YdbParams_DiskType.SSD)}>SSD</SelectItem>
                  <SelectItem value={String(YdbParams_DiskType.NVME)}>NVMe</SelectItem>
                  <SelectItem value={String(YdbParams_DiskType.ROT)}>ROT</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label>Database path</Label>
              <Input className="mt-1" value={p.databasePath} onChange={(ev) => set({ databasePath: ev.target.value })} />
            </div>
          </div>
          <ToggleRow label="Auto-size pdisks" hint="dry-run resize from workload" checked={p.autoSizePdisks} onChange={(b) => set({ autoSizePdisks: b })} />
        </div>
      );
    }
    case "ydbManaged": {
      const p = e.ydbManaged;
      const set = (patch: Partial<YdbManagedParamsVM>) =>
        apply({ ...db, params: { kind: "ydbManaged", ydbManaged: { ...p, ...patch } } });
      const dedicated = p.type === YdbManagedParams_Type.DEDICATED;
      return (
        <div className="space-y-4">
          <div className="grid grid-cols-1 gap-3 @lg:grid-cols-2">
            <div>
              <Label>Flavor</Label>
              <Select value={String(p.type)} onValueChange={(v) => set({ type: Number(v) as YdbManagedParams_Type })}>
                <SelectTrigger className="mt-1">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={String(YdbManagedParams_Type.SERVERLESS)}>Serverless</SelectItem>
                  <SelectItem value={String(YdbManagedParams_Type.DEDICATED)}>Dedicated</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label>Compute class</Label>
              <Select value={String(p.computeType)} onValueChange={(v) => set({ computeType: Number(v) as YdbManagedParams_ComputeType })}>
                <SelectTrigger className="mt-1">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={String(YdbManagedParams_ComputeType.OLTP)}>OLTP</SelectItem>
                  <SelectItem value={String(YdbManagedParams_ComputeType.OLAP)}>OLAP</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          {dedicated ? (
            <div className="grid grid-cols-1 gap-3 @lg:grid-cols-2 @3xl:grid-cols-3">
              <div>
                <Label>Resource preset</Label>
                <Input className="mt-1" placeholder="medium" value={p.resourcePresetId} onChange={(ev) => set({ resourcePresetId: ev.target.value })} />
              </div>
              <NumField label="Node count" value={p.nodeCount} onChange={(n) => set({ nodeCount: n })} min={1} />
              <NumField label="Storage groups" value={p.storageGroups} onChange={(n) => set({ storageGroups: n })} />
            </div>
          ) : (
            <NumField label="Throttling RCUs" value={p.throttlingRcus} onChange={(n) => set({ throttlingRcus: n })} hint="0 = provider default" />
          )}
          <p className="text-[11px] text-zinc-600">
            Managed YDB has no self-deployed nodes — only the stroppy client runs in the topology.
          </p>
        </div>
      );
    }
    case "cockroach": {
      const p = e.cockroach;
      const set = (patch: Partial<CockroachParamsVM>) =>
        apply({ ...db, params: { kind: "cockroach", cockroach: { ...p, ...patch } } });
      return (
        <div className="space-y-4">
          <div className="max-w-xs">
            <NumField label="Nodes" value={p.nodes} onChange={(n) => set({ nodes: n })} min={1} hint="homogeneous cluster" />
          </div>
          <ConfigField
            filename="cluster.settings"
            label="Advanced CockroachDB options"
            description="Cluster settings as key/value pairs; flag entries are supported."
            initiallyOpen={advancedInitiallyOpen}
            value={p.options}
            onChange={(m) => set({ options: m })}
          />
        </div>
      );
    }
    case "orioledb": {
      const p = e.orioledb;
      const set = (patch: Partial<OrioledbParamsVM>) =>
        apply({ ...db, params: { kind: "orioledb", orioledb: { ...p, ...patch } } });
      return (
        <div className="space-y-4">
          <div className="grid grid-cols-1 gap-3 @lg:grid-cols-2">
            <div>
              <Label>Version</Label>
              <Select value={db.version} onValueChange={(v) => apply({ ...db, version: v })}>
                <SelectTrigger className="mt-1">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {DB_VERSIONS[db.kind].map((v) => (
                    <SelectItem key={v} value={v}>{v}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label>initdb locale</Label>
              <Input
                className="mt-1"
                placeholder="C"
                value={p.initdbLocale}
                onChange={(ev) => set({ initdbLocale: ev.target.value })}
              />
            </div>
          </div>
          <div className="grid grid-cols-1 gap-3 @lg:grid-cols-3">
            <NumField label="shared_buffers (MiB)" value={p.sharedBuffersMb} onChange={(n) => set({ sharedBuffersMb: n })} hint="0 = image default" />
            <NumField label="Replicas" value={p.replicas} onChange={(n) => set({ replicas: n })} hint="streaming standbys" />
            <NumField label="HAProxy nodes" value={p.haproxy} onChange={(n) => set({ haproxy: n })} hint="0/1 — write/read split" />
          </div>
          <ConfigField
            filename="postgresql.conf"
            label="Advanced PostgreSQL options (master)"
            description="postgresql.conf key/value options for the master container."
            initiallyOpen={advancedInitiallyOpen}
            value={p.postgresOptions}
            onChange={(m) => set({ postgresOptions: m })}
          />
          {p.replicas > 0 && (
            <ConfigField
              filename="postgresql.conf"
              label="Advanced replica options"
              description="postgresql.conf key/value options applied to replica containers."
              value={p.replicaOptions}
              onChange={(m) => set({ replicaOptions: m })}
            />
          )}
          {p.haproxy > 0 && (
            <ConfigField
              filename="haproxy.cfg"
              label="Advanced HAProxy options"
              description="haproxy.cfg tunables (e.g. maxconn)."
              value={p.haproxyOptions}
              onChange={(m) => set({ haproxyOptions: m })}
            />
          )}
        </div>
      );
    }
    case "noop": {
      return (
        <p className="text-[11px] text-zinc-600">
          No tunable parameters — the no-DB benchmark measures raw stroppy generation rate.
        </p>
      );
    }
    case "pgnoop": {
      const p = e.pgnoop;
      const set = (patch: Partial<PgNoopParamsVM>) =>
        apply({ ...db, params: { kind: "pgnoop", pgnoop: { ...p, ...patch } } });
      return (
        <div className="space-y-4">
          <div className="max-w-xs">
            <NumField label="Worker threads" value={p.workers} onChange={(n) => set({ workers: n })} hint="0 = one per CPU" />
          </div>
        </div>
      );
    }
    case "external": {
      const p = e.external;
      return (
        <div className="space-y-3">
          <div>
            <Label>DSN</Label>
            <Input
              className="mt-1 font-mono text-xs"
              placeholder="postgres://user:pass@host:5432/db"
              value={p.dsn}
              onChange={(ev) => apply({ ...db, params: { kind: "external", external: { dsn: ev.target.value } } })}
            />
            <p className="mt-1 text-[11px] text-zinc-600">The workflow connects to this endpoint and skips deploy/teardown.</p>
          </div>
        </div>
      );
    }
  }
}

// ─── Derived topology summary (mirrors the server topology builder) ────────────
//
// engineNodes maps the chosen engine params to the node groups they imply, by
// role. It is the SAME mapping the wizard mock's topology builder uses, so the
// preset detail/edit pages can preview the topology a preset would deploy. Pure
// (no React), so any surface can render it.

export interface TopologyGroup {
  role: string;
  engine: string;
  count: number;
  /** topology component kind (database | replica | proxy | coordinator | external). */
  kind: string;
}

export function engineNodes(db: DatabaseVM): TopologyGroup[] {
  const e = db.params;
  switch (e.kind) {
    case "postgres": {
      const p = e.postgres;
      const out: TopologyGroup[] = [
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
      const out: TopologyGroup[] = [{ role: "primary", engine: e.kind, count: 1, kind: "database" }];
      if (p.replicas > 0) out.push({ role: "replica", engine: e.kind, count: p.replicas, kind: "replica" });
      if (p.proxysql > 0) out.push({ role: "proxysql", engine: "proxysql", count: p.proxysql, kind: "proxy" });
      return out;
    }
    case "picodata": {
      const p = e.picodata;
      const out: TopologyGroup[] = [{ role: "instance", engine: "picodata", count: Math.max(1, p.instances), kind: "database" }];
      if (p.haproxy > 0) out.push({ role: "haproxy", engine: "haproxy", count: p.haproxy, kind: "proxy" });
      return out;
    }
    case "ydb": {
      const p = e.ydb;
      const out: TopologyGroup[] = [{ role: "storage", engine: "ydb", count: Math.max(1, p.storageNodes), kind: "database" }];
      if (p.databaseNodes > 0) out.push({ role: "database", engine: "ydb", count: p.databaseNodes, kind: "database" });
      if (p.haproxy > 0) out.push({ role: "haproxy", engine: "haproxy", count: p.haproxy, kind: "proxy" });
      return out;
    }
    case "ydbManaged":
      return [{ role: "managed", engine: "ydb", count: 0, kind: "external" }];
    case "cockroach": {
      const p = e.cockroach;
      return [{ role: "node", engine: "cockroach", count: Math.max(1, p.nodes), kind: "database" }];
    }
    case "orioledb": {
      const p = e.orioledb;
      const out: TopologyGroup[] = [{ role: "master", engine: "orioledb", count: 1, kind: "database" }];
      if (p.replicas > 0) out.push({ role: "replica", engine: "orioledb", count: p.replicas, kind: "replica" });
      if (p.haproxy > 0) out.push({ role: "haproxy", engine: "haproxy", count: p.haproxy, kind: "proxy" });
      return out;
    }
    case "noop":
      return [];
    case "pgnoop":
      return [{ role: "node", engine: "pgnoop", count: 1, kind: "database" }];
    case "external":
      return [{ role: "external", engine: "external", count: 0, kind: "external" }];
  }
}

// ─── Database validation (mirrors cloud.v1.domain.Database.Validate) ───────────
//
// Returns FieldError-shaped DraftErrors with dotted `field` paths under the
// "database." prefix, exactly matching the wizard mock's database half of
// validate(). The preset form surfaces these as the backend would. Keep in sync
// with internal/proto/cloud/v1/domain/database.proto's validate constraints.

export function validateDatabase(db: DatabaseVM | null): DraftErrorVM[] {
  const errs: DraftErrorVM[] = [];
  const err = (
    field: string,
    message: string,
    severity: DraftErrorVM["severity"] = "error",
    code = "invalid",
  ) => errs.push({ field, message, severity, code });

  if (!db) {
    err("database", "Configure the database.");
    return errs;
  }
  const e = db.params;

  if (!db.kind) {
    err("database.kind", "Choose a database engine.", "error", "required");
  }

  if (e.kind === "external") {
    if (!e.external.dsn.trim()) err("database.external.dsn", "An external database needs a DSN.", "error", "required");
  } else if (e.kind === "ydbManaged") {
    const p = e.ydbManaged;
    if (p.type === YdbManagedParams_Type.UNSPECIFIED)
      err("database.params.ydb_managed.type", "Choose serverless or dedicated.", "error", "required");
    if (p.type === YdbManagedParams_Type.DEDICATED && p.nodeCount < 1)
      err("database.params.ydb_managed.node_count", "Dedicated YDB needs at least 1 node.", "error", "min");
  } else if (e.kind === "postgres") {
    const p = e.postgres;
    if (p.patroni && !p.etcd)
      err("database.params.postgres.etcd", "Patroni HA needs etcd.", "warning", "needs_etcd");
    if (p.syncReplicas > p.replicas)
      err("database.params.postgres.sync_replicas", "Sync replicas cannot exceed total replicas.", "error", "range");
  } else if (e.kind === "mysql" || e.kind === "mariadb") {
    const p = e.kind === "mysql" ? e.mysql : e.mariadb;
    if (p.semiSync && p.replicas < 1)
      err("database.params.mysql.replicas", "Semi-sync needs at least 1 replica.", "warning", "noop");
    if (p.groupReplication && p.replicas < 2)
      err("database.params.mysql.replicas", "Group replication needs at least 2 replicas.", "warning", "noop");
  } else if (e.kind === "ydb") {
    if (e.ydb.storageNodes < 1) err("database.params.ydb.storage_nodes", "At least 1 storage node.", "error", "min");
    if (e.ydb.faultTolerance === YdbParams_FaultTolerance.MIRROR_3_DC) {
      if (e.ydb.storageNodes < 3) err("database.params.ydb.storage_nodes", "mirror-3-dc needs at least 3 storage nodes.", "error", "range");
      if (e.ydb.pdisksPerStorageNode < 3) err("database.params.ydb.pdisks_per_storage_node", "mirror-3-dc needs at least 3 pdisks per node.", "error", "range");
    }
  } else if (e.kind === "cockroach") {
    if (e.cockroach.nodes < 1) err("database.params.cockroach.nodes", "At least 1 node.", "error", "min");
  } else if (e.kind === "picodata") {
    if (e.picodata.instances < 1) err("database.params.picodata.instances", "At least 1 instance.", "error", "min");
    if (e.picodata.replicationFactor > Math.max(1, e.picodata.instances))
      err("database.params.picodata.replication_factor", "Replication factor cannot exceed the instance count.", "error", "range");
  }

  if (e.kind !== "external" && e.kind !== "ydbManaged" && e.kind !== "noop") {
    if (db.version.length > 128)
      err("database.params.version", "Engine version is too long (max 128 chars).", "error", "max_len");
    else if (!db.version.trim())
      err("database.params.version", "Pick an engine version.", "warning", "version_default");
  }

  return errs;
}
