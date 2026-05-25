import { useMemo } from "react";
import { Background, Controls, ReactFlow, type Edge, type Node } from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import { Section } from "@/components/editors/fields";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { componentColor } from "@/lib/db-colors";
import {
  Topology_Component_Kind,
  Topology_Connection_Kind,
  type Topology,
} from "@/lib/proto/cloud/v1/domain/topology_pb.ts";
import { Net_Mode, Net_Protocol } from "@/lib/proto/cloud/v1/runtime/system/net_pb.ts";

// Topology is a RENDER, not user-authored: it is derived from the Database +
// Workload settings. This is a read-only structured view (graph + tables).
export function TopologyView({ value }: { value: Topology }) {
  return (
    <div className="space-y-4">
      <TopologyGraph topology={value} />

      <Section title="Machines">
        <div className="overflow-hidden rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Machine</TableHead>
                <TableHead>Cores</TableHead>
                <TableHead>Memory</TableHead>
                <TableHead>Disk</TableHead>
                <TableHead>Components</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {value.machines.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="h-16 text-center text-muted-foreground">No machines</TableCell>
                </TableRow>
              ) : (
                value.machines.map((machine, index) => (
                  <TableRow key={machine.id || index}>
                    <TableCell className="font-mono text-xs">{machine.id}</TableCell>
                    <TableCell className="text-xs">{machine.cores}</TableCell>
                    <TableCell className="text-xs">{Number(machine.memoryGb)} GB</TableCell>
                    <TableCell className="text-xs">{Number(machine.diskGb)} GB{machine.dataDisksGb.length ? ` +${machine.dataDisksGb.length}` : ""}</TableCell>
                    <TableCell className="text-xs">{machine.components.map((c) => Topology_Component_Kind[c.kind] ?? "?").join(", ") || "—"}</TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </Section>

      {value.connections.length > 0 ? (
        <Section title="Connections">
          <div className="overflow-hidden rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>From</TableHead>
                  <TableHead>To</TableHead>
                  <TableHead>Protocol</TableHead>
                  <TableHead>Mode</TableHead>
                  <TableHead>Kind</TableHead>
                  <TableHead>Port</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {value.connections.map((connection, index) => (
                  <TableRow key={index}>
                    <TableCell className="font-mono text-xs">{connection.from}</TableCell>
                    <TableCell className="font-mono text-xs">{connection.to}</TableCell>
                    <TableCell className="text-xs">{Net_Protocol[connection.protocol] ?? "—"}</TableCell>
                    <TableCell className="text-xs">{Net_Mode[connection.mode] ?? "—"}</TableCell>
                    <TableCell className="text-xs">{Topology_Connection_Kind[connection.kind] ?? "—"}</TableCell>
                    <TableCell className="text-xs">{connection.port || "—"}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </Section>
      ) : null}

      {value.externalComponents.length > 0 ? (
        <Section title="External components">
          <div className="flex flex-wrap gap-2">
            {value.externalComponents.map((component, index) => (
              <span key={index} className="rounded-full border px-2 py-0.5 text-xs" style={{ borderColor: componentColor(component.kind) }}>
                {component.id} · {Topology_Component_Kind[component.kind] ?? "?"}
              </span>
            ))}
          </div>
        </Section>
      ) : null}
    </div>
  );
}

function TopologyGraph({ topology }: { topology: Topology }) {
  const nodes = useMemo<Node[]>(() => {
    const machineNodes: Node[] = topology.machines.map((machine, index) => ({
      id: machine.id || `machine-${index}`,
      position: { x: index * 240, y: 0 },
      data: { label: `${machine.id || `machine-${index}`} · ${machine.cores}c / ${Number(machine.memoryGb)}GB` },
      style: { border: "1px solid #27272a", background: "#111113", color: "#f4f4f5", borderRadius: 6, fontSize: 11, width: 200 },
    }));
    const externalNodes: Node[] = topology.externalComponents.map((component, index) => ({
      id: component.id || `external-${index}`,
      position: { x: index * 240, y: 180 },
      data: { label: `${component.id || `external-${index}`} (external)` },
      style: { border: `1px solid ${componentColor(component.kind)}`, background: "#111113", color: "#f4f4f5", borderRadius: 6, fontSize: 11, width: 200 },
    }));
    return [...machineNodes, ...externalNodes];
  }, [topology]);

  const edges = useMemo<Edge[]>(
    () =>
      topology.connections.map((connection, index) => ({
        id: `connection-${index}`,
        source: connection.from,
        target: connection.to,
        animated: true,
        style: { stroke: "#3f3f46" },
      })),
    [topology],
  );

  if (nodes.length === 0) {
    return <div className="flex h-40 items-center justify-center rounded-md border text-sm text-muted-foreground">No topology rendered yet</div>;
  }

  return (
    <div className="h-72 overflow-hidden rounded-md border">
      <ReactFlow nodes={nodes} edges={edges} fitView nodesDraggable={false} nodesConnectable={false} elementsSelectable={false} proOptions={{ hideAttribution: true }}>
        <Background color="#27272a" gap={16} />
        <Controls showInteractive={false} />
      </ReactFlow>
    </div>
  );
}
