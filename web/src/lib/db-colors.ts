import { Database_Kind } from "@/lib/proto/cloud/v1/domain/database_pb.ts";
import { Topology_Component_Kind } from "@/lib/proto/cloud/v1/domain/topology_pb.ts";

// Engine brand colors (ported from old_src) for badges and topology nodes.
export const DATABASE_KIND_COLOR: Record<number, string> = {
  [Database_Kind.POSTGRES]: "#5B93C4",
  [Database_Kind.MYSQL]: "#F2A94B",
  [Database_Kind.MARIADB]: "#6E91A8",
  [Database_Kind.YDB]: "#7FB3E8",
  [Database_Kind.COCKROACH]: "#3FAEAE",
  [Database_Kind.PICODATA]: "#F06580",
};

export function databaseColor(kind?: Database_Kind) {
  return (kind !== undefined && DATABASE_KIND_COLOR[kind]) || "#94a3b8";
}

// Topology component colors, by component role.
export const COMPONENT_KIND_COLOR: Record<number, string> = {
  [Topology_Component_Kind.DATABASE]: "#38bdf8",
  [Topology_Component_Kind.AGENT]: "#a78bfa",
  [Topology_Component_Kind.MONITOR]: "#f59e0b",
  [Topology_Component_Kind.STROPPY]: "#22c55e",
  [Topology_Component_Kind.PROXY]: "#ef4444",
  [Topology_Component_Kind.COORDINATOR]: "#14b8a6",
  [Topology_Component_Kind.ADDON]: "#94a3b8",
};

export function componentColor(kind?: Topology_Component_Kind) {
  return (kind !== undefined && COMPONENT_KIND_COLOR[kind]) || "#94a3b8";
}
