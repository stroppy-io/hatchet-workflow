import type { DatabaseKind, MachineSpec, PostgresTopology, MySQLTopology, PicodataTopology, YDBTopology, CockroachTopology } from "@/api/types";
import { DB_COLORS } from "@/lib/db-colors";
import { Database, Server, Cpu, Shield, Layers, Globe } from "lucide-react";

interface TopologyDiagramProps {
  kind: DatabaseKind;
  preset?: string;
  topology?: PostgresTopology | MySQLTopology | PicodataTopology | YDBTopology | CockroachTopology;
}

interface RoleDef {
  label: string;
  count: number;
  color: string;
  icon: typeof Database;
  spec?: string; // "64 vCPU / 128 GB / 50 GB + 558 GB io-m3"
}

// formatSpec turns a MachineSpec into a one-line resource summary used as
// the per-role subline on a topology card. Returns "" for empty/zero specs
// (e.g. preset-name fallback paths) so the renderer can hide the line.
function formatSpec(s: Partial<MachineSpec> | undefined): string {
  if (!s) return "";
  const parts: string[] = [];
  if (s.cpus) parts.push(`${s.cpus} vCPU`);
  if (s.memory_mb) {
    const gb = s.memory_mb / 1024;
    parts.push(gb >= 1 ? `${Number.isInteger(gb) ? gb : gb.toFixed(1)} GB` : `${s.memory_mb} MB`);
  }
  if (s.disk_gb) parts.push(`${s.disk_gb} GB`);
  let line = parts.join(" / ");
  const sd = s.secondary_disks?.[0];
  if (sd?.size_gb) {
    const t = (sd.type || "").replace("network-ssd-io-m3", "io-m3").replace("network-ssd", "ssd");
    line += ` + ${sd.size_gb} GB${t ? " " + t : ""}`;
  }
  return line;
}

// Infra roles use a neutral color across all DB types.
const INFRA_PROXY = "#A0860A";
const INFRA_COORD = "#7C6CC8";

function getRolesFromTopology(kind: DatabaseKind, topology: PostgresTopology | MySQLTopology | PicodataTopology | YDBTopology | CockroachTopology): RoleDef[] {
  const c = DB_COLORS[kind];

  if (kind === "postgres") {
    const t = topology as PostgresTopology;
    const roles: RoleDef[] = [];
    if (t.master) roles.push({ label: "Master", count: t.master.count || 1, color: c.hex, icon: Database, spec: formatSpec(t.master) });
    if (t.replicas?.length) {
      const total = t.replicas.reduce((s, r) => s + r.count, 0);
      if (total > 0) roles.push({ label: "Replica", count: total, color: c.hexSecondary, icon: Database, spec: formatSpec(t.replicas[0]) });
    }
    if (t.haproxy) roles.push({ label: "HAProxy", count: t.haproxy.count || 1, color: INFRA_PROXY, icon: Globe, spec: formatSpec(t.haproxy) });
    if (t.etcd) roles.push({ label: "Etcd", count: 3, color: INFRA_COORD, icon: Layers });
    return roles;
  }

  if (kind === "mysql" || kind === "mariadb") {
    const t = topology as MySQLTopology;
    const roles: RoleDef[] = [];
    if (t.primary) roles.push({ label: "Primary", count: t.primary.count || 1, color: c.hex, icon: Server, spec: formatSpec(t.primary) });
    if (t.replicas?.length) {
      const total = t.replicas.reduce((s, r) => s + r.count, 0);
      if (total > 0) roles.push({ label: "Replica", count: total, color: c.hexSecondary, icon: Server, spec: formatSpec(t.replicas[0]) });
    }
    if (t.proxysql) roles.push({ label: "ProxySQL", count: t.proxysql.count || 1, color: INFRA_PROXY, icon: Globe, spec: formatSpec(t.proxysql) });
    return roles;
  }

  if (kind === "picodata") {
    const t = topology as PicodataTopology;
    const roles: RoleDef[] = [];
    if (t.tiers?.length) {
      // Tiers don't carry per-node specs themselves — they're a logical
      // grouping over the instance pool. Show the instance flavor as the
      // shared spec line so the user still sees CPU/RAM/disk.
      const sharedSpec = t.instances?.length ? formatSpec(t.instances[0]) : "";
      for (const tier of t.tiers) {
        roles.push({
          label: tier.name.charAt(0).toUpperCase() + tier.name.slice(1),
          count: tier.count,
          color: tier.can_vote ? c.hex : c.hexSecondary,
          icon: tier.can_vote ? Cpu : Shield,
          spec: sharedSpec,
        });
      }
    } else if (t.instances?.length) {
      const total = t.instances.reduce((s, i) => s + i.count, 0);
      roles.push({ label: "Instance", count: total, color: c.hex, icon: Cpu, spec: formatSpec(t.instances[0]) });
    }
    if (t.haproxy) roles.push({ label: "HAProxy", count: t.haproxy.count || 1, color: INFRA_PROXY, icon: Globe, spec: formatSpec(t.haproxy) });
    return roles;
  }

  if (kind === "ydb") {
    const t = topology as YDBTopology;
    const roles: RoleDef[] = [];
    roles.push({ label: "Storage", count: t.storage.count || 1, color: c.hex, icon: Shield, spec: formatSpec(t.storage) });
    if (t.database) roles.push({ label: "Database", count: t.database.count || 1, color: c.hexSecondary, icon: Cpu, spec: formatSpec(t.database) });
    if (t.haproxy) roles.push({ label: "HAProxy", count: t.haproxy.count || 1, color: INFRA_PROXY, icon: Globe, spec: formatSpec(t.haproxy) });
    return roles;
  }

  if (kind === "cockroach") {
    const t = topology as unknown as CockroachTopology;
    return [{ label: "Node", count: t.nodes.count || 1, color: c.hex, icon: Database, spec: formatSpec(t.nodes) }];
  }

  return [{ label: "Node", count: 1, color: "#6b7280", icon: Server }];
}

function getRolesFromPresetName(kind: DatabaseKind, preset: string): RoleDef[] {
  const c = DB_COLORS[kind];

  if (kind === "postgres") {
    switch (preset) {
      case "single":
        return [{ label: "Master", count: 1, color: c.hex, icon: Database }];
      case "ha":
        return [
          { label: "Master", count: 1, color: c.hex, icon: Database },
          { label: "Replica", count: 2, color: c.hexSecondary, icon: Database },
          { label: "HAProxy", count: 1, color: INFRA_PROXY, icon: Globe },
          { label: "Etcd", count: 3, color: INFRA_COORD, icon: Layers },
        ];
      case "scale":
        return [
          { label: "Master", count: 1, color: c.hex, icon: Database },
          { label: "Replica", count: 4, color: c.hexSecondary, icon: Database },
          { label: "HAProxy", count: 2, color: INFRA_PROXY, icon: Globe },
          { label: "Etcd", count: 3, color: INFRA_COORD, icon: Layers },
        ];
    }
  }

  if (kind === "mysql" || kind === "mariadb") {
    switch (preset) {
      case "single":
        return [{ label: "Primary", count: 1, color: c.hex, icon: Server }];
      case "replica":
        return [
          { label: "Primary", count: 1, color: c.hex, icon: Server },
          { label: "Replica", count: 2, color: c.hexSecondary, icon: Server },
          { label: "ProxySQL", count: 1, color: INFRA_PROXY, icon: Globe },
        ];
      case "group":
        return [
          { label: "Primary", count: 1, color: c.hex, icon: Server },
          { label: "Replica", count: 2, color: c.hexSecondary, icon: Server },
          { label: "ProxySQL", count: 2, color: INFRA_PROXY, icon: Globe },
        ];
    }
  }

  if (kind === "picodata") {
    switch (preset) {
      case "single":
        return [{ label: "Instance", count: 1, color: c.hex, icon: Cpu }];
      case "cluster":
        return [
          { label: "Instance", count: 3, color: c.hex, icon: Cpu },
          { label: "HAProxy", count: 1, color: INFRA_PROXY, icon: Globe },
        ];
      case "scale":
        return [
          { label: "Compute", count: 3, color: c.hex, icon: Cpu },
          { label: "Storage", count: 3, color: c.hexSecondary, icon: Shield },
          { label: "HAProxy", count: 2, color: INFRA_PROXY, icon: Globe },
        ];
    }
  }

  if (kind === "ydb") {
    switch (preset) {
      case "single":
        return [{ label: "Storage", count: 1, color: c.hex, icon: Shield }];
      case "cluster":
        return [
          { label: "Storage", count: 3, color: c.hex, icon: Shield },
          { label: "Database", count: 3, color: c.hexSecondary, icon: Cpu },
        ];
      case "scale":
        return [
          { label: "Storage", count: 3, color: c.hex, icon: Shield },
          { label: "Database", count: 6, color: c.hexSecondary, icon: Cpu },
        ];
    }
  }

  if (kind === "cockroach") {
    switch (preset) {
      case "single":
        return [{ label: "Node", count: 1, color: c.hex, icon: Database }];
      case "cluster-3":
        return [{ label: "Node", count: 3, color: c.hex, icon: Database }];
      case "cluster-6":
        return [{ label: "Node", count: 6, color: c.hex, icon: Database }];
    }
  }

  return [{ label: "Node", count: 1, color: "#6b7280", icon: Server }];
}

export function TopologyDiagram({ kind, preset, topology }: TopologyDiagramProps) {
  const roles = topology
    ? getRolesFromTopology(kind, topology)
    : preset
      ? getRolesFromPresetName(kind, preset)
      : [{ label: "Node", count: 1, color: "#6b7280", icon: Server }];

  const totalNodes = roles.reduce((s, r) => s + r.count, 0);

  return (
    <div className="space-y-1.5">
      {roles.map((role, i) => {
        const Icon = role.icon;
        return (
          <div key={i} className="space-y-0.5">
            <div className="flex items-center gap-2">
              <Icon className="h-3 w-3 shrink-0" style={{ color: role.color }} />
              <span className="text-[11px] font-mono text-zinc-400 flex-1 truncate">
                {role.label}
              </span>
              {role.count > 1 && (
                <span
                  className="text-[10px] font-mono tabular-nums px-1.5 py-px border"
                  style={{ borderColor: role.color + "40", color: role.color }}
                >
                  ×{role.count}
                </span>
              )}
            </div>
            {role.spec && (
              <div className="pl-5 text-[9px] font-mono text-zinc-600 truncate" title={role.spec}>
                {role.spec}
              </div>
            )}
          </div>
        );
      })}
      <div className="flex items-center gap-1.5 pt-1 border-t border-zinc-800/40">
        <span className="text-[10px] font-mono text-zinc-600 tabular-nums">
          {totalNodes} node{totalNodes !== 1 ? "s" : ""}
        </span>
        <div className="flex gap-0.5 ml-auto">
          {roles.map((role, ri) =>
            Array.from({ length: Math.min(role.count, 6) }).map((_, j) => (
              <div
                key={`${ri}-${j}`}
                className="w-1.5 h-1.5 rounded-full"
                style={{ backgroundColor: role.color }}
              />
            ))
          )}
        </div>
      </div>
    </div>
  );
}
