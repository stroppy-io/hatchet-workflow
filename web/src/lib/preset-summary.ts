import type { DatabasePreset } from "@/lib/proto/cloud/v1/domain/database_pb.ts";
import { Database_Kind } from "@/lib/proto/cloud/v1/domain/database_pb.ts";
import type { TestPreset } from "@/lib/proto/cloud/v1/domain/test_pb.ts";
import type { Topology } from "@/lib/proto/cloud/v1/domain/topology_pb.ts";
import { Topology_Component_Kind } from "@/lib/proto/cloud/v1/domain/topology_pb.ts";
import type { WorkloadPreset } from "@/lib/proto/cloud/v1/domain/workload_pb.ts";
import { Workload_Protocol } from "@/lib/proto/cloud/v1/domain/workload_pb.ts";
import type { Preset } from "@/lib/proto/cloud/v1/models/preset_pb.ts";

export function presetTitle(preset: Preset) {
  return (
    preset.name ||
    preset.tags?.labels?.title ||
    preset.tags?.labels?.name ||
    preset.tags?.labels?.display_name ||
    preset.entity?.id?.value ||
    "Untitled preset"
  );
}

export function presetDescription(preset: Preset) {
  return preset.tags?.labels?.description || preset.tags?.labels?.summary || "";
}

export function databaseKindLabel(kind?: Database_Kind) {
  switch (kind) {
    case Database_Kind.POSTGRES:
      return "Postgres";
    case Database_Kind.MYSQL:
      return "MySQL";
    case Database_Kind.MARIADB:
      return "MariaDB";
    case Database_Kind.YDB:
      return "YDB";
    case Database_Kind.COCKROACH:
      return "Cockroach";
    case Database_Kind.PICODATA:
      return "Picodata";
    default:
      return "Other database";
  }
}

export function workloadProtocolLabel(protocol?: Workload_Protocol) {
  switch (protocol) {
    case Workload_Protocol.PG:
      return "PostgreSQL";
    case Workload_Protocol.MYSQL:
      return "MySQL";
    case Workload_Protocol.PICODATA:
      return "Picodata";
    case Workload_Protocol.YDB_GRPC:
      return "YDB gRPC";
    case Workload_Protocol.YDB_GRPCS:
      return "YDB gRPCS";
    case Workload_Protocol.COCKROACH:
      return "Cockroach";
    default:
      return "Default protocol";
  }
}

export function topologySummary(topology?: Topology) {
  if (!topology) return "No topology";
  const machines = topology.machines.length;
  const components = topology.machines.reduce((count, machine) => count + machine.components.length, 0) + topology.externalComponents.length;
  const cores = topology.machines.reduce((sum, machine) => sum + machine.cores, 0);
  const memory = topology.machines.reduce((sum, machine) => sum + Number(machine.memoryGb || 0n), 0);
  const dataDisks = topology.machines.reduce((sum, machine) => sum + machine.dataDisksGb.length, 0);
  return `${machines} machines, ${components} components, ${cores} cores, ${memory} GiB RAM${dataDisks ? `, ${dataDisks} data disks` : ""}`;
}

export function topologyComponentSummary(topology?: Topology) {
  if (!topology) return [];
  const counts = new Map<Topology_Component_Kind, number>();
  for (const machine of topology.machines) {
    for (const component of machine.components) {
      counts.set(component.kind, (counts.get(component.kind) ?? 0) + 1);
    }
  }
  for (const component of topology.externalComponents) {
    counts.set(component.kind, (counts.get(component.kind) ?? 0) + 1);
  }
  return Array.from(counts.entries())
    .filter(([kind]) => kind !== Topology_Component_Kind.UNSPECIFIED)
    .map(([kind, count]) => `${componentKindLabel(kind)} x${count}`);
}

export function databasePresetSummary(preset?: DatabasePreset) {
  const database = preset?.database;
  return {
    group: databaseKindLabel(database?.kind),
    primary: database?.version ? `${databaseKindLabel(database.kind)} ${database.version}` : databaseKindLabel(database?.kind),
    target: database?.target?.target.case === "external" ? "External target" : "Self-hosted target",
    topology: topologySummary(preset?.topology),
    components: topologyComponentSummary(preset?.topology),
  };
}

export function workloadPresetSummary(preset?: WorkloadPreset) {
  const workload = preset?.workload;
  const script = workload?.script || "Unspecified script";
  return {
    group: scriptGroup(script),
    primary: script,
    protocol: workloadProtocolLabel(workload?.protocol),
    execution: workload?.execution
      ? `${workload.execution.vus} VUs, ${workload.execution.limit.case ? workload.execution.limit.value : "open limit"}`
      : "No execution profile",
    topology: topologySummary(preset?.topology),
  };
}

export function testPresetSummary(preset?: TestPreset) {
  return {
    group: preset?.database ? databaseKindLabel(preset.database.kind) : "Other tests",
    database: preset?.database?.version ? `${databaseKindLabel(preset.database.kind)} ${preset.database.version}` : databaseKindLabel(preset?.database?.kind),
    workload: preset?.workload?.script || "Unspecified workload",
    protocol: workloadProtocolLabel(preset?.workload?.protocol),
    topology: topologySummary(preset?.topology),
    deployment: preset?.deployment ? `${preset.deployment.specs.length} deployment specs` : "No deployment intent",
  };
}

export function componentKindLabel(kind: Topology_Component_Kind) {
  switch (kind) {
    case Topology_Component_Kind.AGENT:
      return "agent";
    case Topology_Component_Kind.DATABASE:
      return "database";
    case Topology_Component_Kind.MONITOR:
      return "monitor";
    case Topology_Component_Kind.STROPPY:
      return "stroppy";
    case Topology_Component_Kind.PROXY:
      return "proxy";
    case Topology_Component_Kind.COORDINATOR:
      return "coordinator";
    case Topology_Component_Kind.ADDON:
      return "addon";
    default:
      return "component";
  }
}

function scriptGroup(script: string) {
  const normalized = script.toLowerCase();
  if (normalized.includes("tpcc")) return "TPC-C";
  if (normalized.includes("tpcds")) return "TPC-DS";
  if (normalized.includes("ycsb")) return "YCSB";
  if (normalized.includes("sql")) return "SQL";
  if (normalized.includes("probe")) return "Probe";
  return "Custom scripts";
}
