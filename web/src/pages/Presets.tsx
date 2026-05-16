import { useEffect, useState, useCallback } from "react";
import { Link } from "react-router-dom";
import { clients } from "@/api/clients";
import { getTenantId } from "@/api/transport";
import { Database_Kind } from "@/lib/proto/cloud/v1/catalog/database_pb";
import type { DatabasePreset, Database } from "@/lib/proto/cloud/v1/catalog/database_pb";
import { TopologyDiagram } from "@/components/TopologyDiagram";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import {
  Plus,
  Play,
  Copy,
  Trash2,
  Pencil,
  Check,
  AlertCircle,
} from "lucide-react";
import { DB_COLORS } from "@/lib/db-colors";
import { useConfirm } from "@/components/ui/confirm-dialog";

// ─── Local row type ───────────────────────────────────────────────

type DbKindStr = "postgres" | "mysql" | "mariadb" | "picodata" | "ydb" | "ydb-managed" | "cockroach";

const ALL_DB_KINDS: DbKindStr[] = ["postgres", "mysql", "mariadb", "picodata", "ydb", "ydb-managed", "cockroach"];

const KIND_TO_STR: Partial<Record<Database_Kind, DbKindStr>> = {
  [Database_Kind.DATABASE_KIND_POSTGRES]: "postgres",
  [Database_Kind.DATABASE_KIND_MYSQL]: "mysql",
  [Database_Kind.DATABASE_KIND_MARIADB]: "mariadb",
  [Database_Kind.DATABASE_KIND_YDB]: "ydb",
  [Database_Kind.DATABASE_KIND_YDB_MANAGED]: "ydb-managed",
  [Database_Kind.DATABASE_KIND_COCKROACH]: "cockroach",
  [Database_Kind.DATABASE_KIND_PICODATA]: "picodata",
};

// Convert proto Database to legacy topology shapes (for TopologyDiagram).
// TopologyDiagram only needs count fields and boolean feature flags.
function toTopology(db: Database | undefined): Record<string, unknown> | undefined {
  if (!db) return undefined;
  const v = db.variant;
  if (v.case === "postgres" && v.value.shape) {
    const s = v.value.shape;
    return {
      master: { count: 1 },
      replicas: s.replicas > 0 ? [{ count: s.replicas }] : [],
      haproxy: s.haproxyDedicated ? { count: 1 } : undefined,
      etcd: s.patroni && !s.etcdColocated,
      patroni: s.patroni,
      pgbouncer: s.pgbouncerColocated,
      sync_replicas: s.syncReplicas,
    };
  }
  if ((v.case === "mysql" || v.case === "mariadb") && v.value.shape) {
    const s = v.value.shape;
    return {
      primary: { count: 1 },
      replicas: s.replicas > 0 ? [{ count: s.replicas }] : [],
      proxysql: s.proxysqlDedicated ? { count: 1 } : undefined,
      group_replication: s.groupReplication,
      semi_sync: s.semiSync,
    };
  }
  if (v.case === "ydb" && v.value.shape) {
    const s = v.value.shape;
    return {
      storage: { count: s.storageNodes },
      database: s.databaseNodes > 0 ? { count: s.databaseNodes } : undefined,
      haproxy: s.haproxyDedicated ? { count: 1 } : undefined,
    };
  }
  if (v.case === "picodata" && v.value.shape) {
    const s = v.value.shape;
    const tiers = s.tiers?.map((t) => ({
      name: t.name,
      count: t.count,
      can_vote: t.canVote,
    })) ?? [];
    return {
      instances: tiers.length === 0 ? [{ count: s.nodes }] : undefined,
      tiers: tiers.length > 0 ? tiers : undefined,
      haproxy: s.haproxyDedicated ? { count: 1 } : undefined,
      shards: s.shards,
      replication_factor: s.replicationFactor,
    };
  }
  if (v.case === "cockroach" && v.value.shape) {
    return { nodes: { count: v.value.shape.nodes } };
  }
  if (v.case === "ydbManaged" && v.value.shape) {
    const s = v.value.shape;
    return {
      type: s.computeType === 1 ? "dedicated" : "serverless",
      resource_preset_id: s.resourcePresetId,
      storage_groups: s.storageGroups,
      storage_type: s.storageTypeId,
    };
  }
  return undefined;
}

function nodeCount(db: Database | undefined): number {
  if (!db) return 0;
  const v = db.variant;
  if (v.case === "postgres" && v.value.shape) {
    const s = v.value.shape;
    return 1 + s.replicas + (s.haproxyDedicated ? 1 : 0) + (s.patroni && !s.etcdColocated ? 3 : 0);
  }
  if ((v.case === "mysql" || v.case === "mariadb") && v.value.shape) {
    const s = v.value.shape;
    return 1 + s.replicas + (s.proxysqlDedicated ? 1 : 0);
  }
  if (v.case === "ydb" && v.value.shape) {
    const s = v.value.shape;
    return s.storageNodes + s.databaseNodes + (s.haproxyDedicated ? 1 : 0);
  }
  if (v.case === "picodata" && v.value.shape) {
    const s = v.value.shape;
    const fromTiers = s.tiers?.reduce((acc, t) => acc + t.count, 0) ?? 0;
    return (fromTiers || s.nodes) + (s.haproxyDedicated ? 1 : 0);
  }
  if (v.case === "cockroach" && v.value.shape) {
    return v.value.shape.nodes;
  }
  return 1;
}

// ─── Page ─────────────────────────────────────────────────────────

export function Presets() {
  const [presets, setPresets] = useState<DatabasePreset[]>([]);
  const [loading, setLoading] = useState(true);
  const [filterKind, setFilterKind] = useState<string>("");
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);
  const confirm = useConfirm();

  const load = useCallback(async () => {
    try {
      const tid = getTenantId();
      const resp = await clients.databasePreset.listDatabasePresets(
        tid ? { value: tid } : {}
      );
      setPresets(resp.databasePresets ?? []);
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed to load" });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  async function handleDelete(id: string) {
    if (!(await confirm({ title: "Delete this preset?", description: "This action cannot be undone.", danger: true }))) return;
    try {
      await clients.databasePreset.deleteDatabasePreset({ value: id });
      setMessage({ type: "success", text: "Deleted" });
      load();
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed" });
    }
  }

  async function handleClone(id: string, name: string) {
    try {
      const r = await clients.databasePreset.cloneDatabasePreset({ value: id });
      setMessage({ type: "success", text: `Cloned as "${r.identity?.name ?? name}"` });
      load();
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed" });
    }
  }

  // Group by db kind
  const grouped: Record<string, DatabasePreset[]> = {};
  for (const k of ALL_DB_KINDS) grouped[k] = [];
  for (const p of presets) {
    const k = KIND_TO_STR[p.database?.kind ?? 0];
    if (k && (!filterKind || filterKind === k)) {
      (grouped[k] ??= []).push(p);
    }
  }

  const kindsToShow = filterKind ? [filterKind as DbKindStr] : ALL_DB_KINDS;

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Topology Presets</h1>
          <p className="text-sm text-muted-foreground">
            Manage database topology templates for runs
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Select value={filterKind || "__all__"} onValueChange={(v) => setFilterKind(v === "__all__" ? "" : v)}>
            <SelectTrigger className="h-8 w-36 font-mono text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">All databases</SelectItem>
              <SelectItem value="postgres">PostgreSQL</SelectItem>
              <SelectItem value="mysql">MySQL</SelectItem>
              <SelectItem value="picodata">Picodata</SelectItem>
              <SelectItem value="ydb">YDB</SelectItem>
            </SelectContent>
          </Select>
          <Link to="/presets/new">
            <Button size="sm">
              <Plus className="h-3.5 w-3.5" /> New Preset
            </Button>
          </Link>
        </div>
      </div>

      {message && (
        <div className={`flex items-center gap-2 text-xs p-2 border font-mono ${
          message.type === "success" ? "border-success/30 text-success" : "border-destructive/30 text-destructive"
        }`}>
          {message.type === "success" ? <Check className="h-3 w-3" /> : <AlertCircle className="h-3 w-3" />}
          {message.text}
        </div>
      )}

      {loading ? (
        <p className="text-sm text-muted-foreground">Loading presets...</p>
      ) : (
        kindsToShow.map((kind) => {
          const items = grouped[kind];
          if (!items?.length) return null;
          return (
            <div key={kind}>
              <h2 className={`text-sm font-semibold uppercase tracking-wider mb-3 ${DB_COLORS[kind]?.text ?? ""}`}>
                {kind}
              </h2>
              <div className="grid grid-cols-3 gap-4">
                {items.map((p) => (
                  <PresetCard
                    key={p.id?.value}
                    preset={p}
                    dbKind={KIND_TO_STR[p.database?.kind ?? 0] ?? "postgres"}
                    onClone={() => handleClone(p.id?.value ?? "", p.identity?.name ?? "")}
                    onDelete={() => handleDelete(p.id?.value ?? "")}
                  />
                ))}
              </div>
            </div>
          );
        })
      )}
    </div>
  );
}

// ─── Preset Card ─────────────────────────────────────────────────

function PresetCard({
  preset,
  dbKind,
  onClone,
  onDelete,
}: {
  preset: DatabasePreset;
  dbKind: DbKindStr;
  onClone: () => void;
  onDelete: () => void;
}) {
  const topology = toTopology(preset.database);
  const nodes = nodeCount(preset.database);
  const isBuiltin = !preset.tenantId;

  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 min-w-0">
            <CardTitle className="text-sm truncate">{preset.identity?.name}</CardTitle>
            {isBuiltin && <Badge variant="secondary" className="text-[8px] shrink-0">builtin</Badge>}
          </div>
          <Badge variant="secondary">
            {nodes} node{nodes !== 1 ? "s" : ""}
          </Badge>
        </div>
        {preset.identity?.description && (
          <p className="text-[10px] text-zinc-500 font-mono truncate">{preset.identity.description}</p>
        )}
      </CardHeader>
      <CardContent className="space-y-3">
        <TopologyDiagram kind={dbKind} topology={topology as never} />
        <div className="flex items-center gap-1 pt-1">
          <Link to={`/runs/new?preset_id=${preset.id?.value}`} className="flex-1">
            <Button size="sm" variant="outline" className="w-full">
              <Play className="h-3 w-3" />
              Start Run
            </Button>
          </Link>
          <Link to={`/presets/${preset.id?.value}/edit`} className="p-1.5 text-zinc-600 hover:text-zinc-300" title="Edit">
            <Pencil className="w-3 h-3" />
          </Link>
          <button onClick={onClone} className="p-1.5 text-zinc-600 hover:text-zinc-300" title="Clone">
            <Copy className="w-3 h-3" />
          </button>
          {!isBuiltin && (
            <button onClick={onDelete} className="p-1.5 text-zinc-600 hover:text-red-400" title="Delete">
              <Trash2 className="w-3 h-3" />
            </button>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
