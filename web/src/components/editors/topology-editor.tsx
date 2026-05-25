import { useMemo } from "react";
import { create, toJsonString } from "@bufbuild/protobuf";
import { Background, Controls, ReactFlow, type Edge, type Node } from "@xyflow/react";
import { Plus, X } from "lucide-react";
import "@xyflow/react/dist/style.css";

import { NumberField, SelectField, Section, TextField, enumAuto } from "@/components/editors/fields";
import { JsonEditor } from "@/components/ui/json-editor";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { componentColor } from "@/lib/db-colors";
import { ConfigSchema } from "@/lib/proto/cloud/v1/runtime/render/config_pb.ts";
import {
  Topology_ComponentSchema,
  Topology_Component_Kind,
  Topology_ConnectionSchema,
  Topology_Connection_Kind,
  Topology_MachineSchema,
  TopologySchema,
  type Topology,
  type Topology_Component,
  type Topology_Connection,
  type Topology_Machine,
} from "@/lib/proto/cloud/v1/domain/topology_pb.ts";
import { Net_Mode, Net_Protocol } from "@/lib/proto/cloud/v1/runtime/system/net_pb.ts";

const COMPONENT_KINDS = enumAuto(Topology_Component_Kind as unknown as Record<string, number>);
const CONNECTION_KINDS = enumAuto(Topology_Connection_Kind as unknown as Record<string, number>);
const NET_PROTOCOLS = enumAuto(Net_Protocol as unknown as Record<string, number>);
const NET_MODES = enumAuto(Net_Mode as unknown as Record<string, number>);

function parseDiskCsv(text: string): bigint[] {
  return text
    .split(",")
    .map((part) => part.trim())
    .filter(Boolean)
    .map((part) => {
      try {
        return BigInt(Math.max(0, Math.trunc(Number(part))));
      } catch {
        return 0n;
      }
    });
}

export function TopologyEditor({ value, onChange }: { value: Topology; onChange: (value: Topology) => void }) {
  const patch = (partial: Partial<Topology>) => onChange(create(TopologySchema, { ...value, ...partial }));

  return (
    <div className="space-y-6">
      <TopologyGraph topology={value} />

      <Section title="Machines">
        <div className="space-y-3">
          {value.machines.length === 0 ? <p className="text-xs text-muted-foreground">No machines</p> : null}
          {value.machines.map((machine, index) => (
            <MachineCard
              key={index}
              value={machine}
              onChange={(next) => patch({ machines: value.machines.map((m, i) => (i === index ? next : m)) })}
              onRemove={() => patch({ machines: value.machines.filter((_, i) => i !== index) })}
            />
          ))}
          <Button type="button" variant="outline" size="sm" onClick={() => patch({ machines: [...value.machines, create(Topology_MachineSchema, { cores: 1, memoryGb: 1n, diskGb: 10n })] })}>
            <Plus className="size-3.5" />
            Add machine
          </Button>
        </div>
      </Section>

      <Section title="Connections">
        <div className="space-y-3">
          {value.connections.length === 0 ? <p className="text-xs text-muted-foreground">No connections</p> : null}
          {value.connections.map((connection, index) => (
            <ConnectionRow
              key={index}
              value={connection}
              onChange={(next) => patch({ connections: value.connections.map((c, i) => (i === index ? next : c)) })}
              onRemove={() => patch({ connections: value.connections.filter((_, i) => i !== index) })}
            />
          ))}
          <Button type="button" variant="outline" size="sm" onClick={() => patch({ connections: [...value.connections, create(Topology_ConnectionSchema, {})] })}>
            <Plus className="size-3.5" />
            Add connection
          </Button>
        </div>
      </Section>

      <Section title="External components">
        <div className="space-y-3">
          {value.externalComponents.length === 0 ? <p className="text-xs text-muted-foreground">None</p> : null}
          {value.externalComponents.map((component, index) => (
            <ComponentEditor
              key={index}
              value={component}
              onChange={(next) => patch({ externalComponents: value.externalComponents.map((c, i) => (i === index ? next : c)) })}
              onRemove={() => patch({ externalComponents: value.externalComponents.filter((_, i) => i !== index) })}
            />
          ))}
          <Button type="button" variant="outline" size="sm" onClick={() => patch({ externalComponents: [...value.externalComponents, create(Topology_ComponentSchema, {})] })}>
            <Plus className="size-3.5" />
            Add external component
          </Button>
        </div>
      </Section>
    </div>
  );
}

function MachineCard({ value, onChange, onRemove }: { value: Topology_Machine; onChange: (value: Topology_Machine) => void; onRemove: () => void }) {
  const patch = (partial: Partial<Topology_Machine>) => onChange(create(Topology_MachineSchema, { ...value, ...partial }));
  return (
    <div className="space-y-3 rounded-md border p-4">
      <div className="flex items-center gap-2">
        <Input className="flex-1 font-mono text-xs" value={value.id} placeholder="machine id" onChange={(event) => patch({ id: event.target.value })} />
        <Button type="button" variant="ghost" size="icon-sm" onClick={onRemove}>
          <X className="size-3.5" />
        </Button>
      </div>
      <div className="grid gap-3 sm:grid-cols-4">
        <NumberField label="Cores" value={value.cores} min={1} onChange={(v) => patch({ cores: v })} />
        <NumberField label="Memory (GB)" value={Number(value.memoryGb)} min={0} onChange={(v) => patch({ memoryGb: BigInt(Math.max(0, Math.trunc(v))) })} />
        <NumberField label="Disk (GB)" value={Number(value.diskGb)} min={0} onChange={(v) => patch({ diskGb: BigInt(Math.max(0, Math.trunc(v))) })} />
        <TextField label="Data disks (GB, csv)" value={value.dataDisksGb.map(String).join(",")} onChange={(v) => patch({ dataDisksGb: parseDiskCsv(v) })} placeholder="100,100" />
      </div>
      <div className="space-y-2">
        <Label>Components</Label>
        {value.components.map((component, index) => (
          <ComponentEditor
            key={index}
            value={component}
            onChange={(next) => patch({ components: value.components.map((c, i) => (i === index ? next : c)) })}
            onRemove={() => patch({ components: value.components.filter((_, i) => i !== index) })}
          />
        ))}
        <Button type="button" variant="outline" size="sm" onClick={() => patch({ components: [...value.components, create(Topology_ComponentSchema, {})] })}>
          <Plus className="size-3.5" />
          Add component
        </Button>
      </div>
    </div>
  );
}

function ComponentEditor({ value, onChange, onRemove }: { value: Topology_Component; onChange: (value: Topology_Component) => void; onRemove: () => void }) {
  const patch = (partial: Partial<Topology_Component>) => onChange(create(Topology_ComponentSchema, { ...value, ...partial }));
  return (
    <div className="space-y-2 rounded-md border p-3">
      <div className="flex items-center gap-2">
        <Input className="flex-1 font-mono text-xs" value={value.id} placeholder="component id" onChange={(event) => patch({ id: event.target.value })} />
        <div className="w-44">
          <SelectField label="" value={String(value.kind)} onChange={(v) => patch({ kind: Number(v) as Topology_Component_Kind })} options={COMPONENT_KINDS} />
        </div>
        <Button type="button" variant="ghost" size="icon-sm" onClick={onRemove}>
          <X className="size-3.5" />
        </Button>
      </div>
      <details>
        <summary className="cursor-pointer text-xs text-muted-foreground">Rendered config (read-only)</summary>
        <div className="mt-2">
          <JsonEditor readOnly minHeight="140px" value={toJsonString(ConfigSchema, value.config ?? create(ConfigSchema, {}), { prettySpaces: 2 })} />
        </div>
      </details>
    </div>
  );
}

function ConnectionRow({ value, onChange, onRemove }: { value: Topology_Connection; onChange: (value: Topology_Connection) => void; onRemove: () => void }) {
  const patch = (partial: Partial<Topology_Connection>) => onChange(create(Topology_ConnectionSchema, { ...value, ...partial }));
  return (
    <div className="grid grid-cols-2 items-end gap-2 rounded-md border p-3 sm:grid-cols-6">
      <TextField label="From" value={value.from} onChange={(v) => patch({ from: v })} />
      <TextField label="To" value={value.to} onChange={(v) => patch({ to: v })} />
      <SelectField label="Protocol" value={String(value.protocol)} onChange={(v) => patch({ protocol: Number(v) as Net_Protocol })} options={NET_PROTOCOLS} />
      <SelectField label="Mode" value={String(value.mode)} onChange={(v) => patch({ mode: Number(v) as Net_Mode })} options={NET_MODES} />
      <SelectField label="Kind" value={String(value.kind)} onChange={(v) => patch({ kind: Number(v) as Topology_Connection_Kind })} options={CONNECTION_KINDS} />
      <div className="flex items-end gap-2">
        <NumberField label="Port" value={value.port} min={0} max={65535} onChange={(v) => patch({ port: v })} />
        <Button type="button" variant="ghost" size="icon-sm" onClick={onRemove}>
          <X className="size-3.5" />
        </Button>
      </div>
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
    return <div className="flex h-40 items-center justify-center rounded-md border text-sm text-muted-foreground">Add machines to see the topology graph</div>;
  }

  return (
    <div className="h-72 overflow-hidden rounded-md border">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        fitView
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        proOptions={{ hideAttribution: true }}
      >
        <Background color="#27272a" gap={16} />
        <Controls showInteractive={false} />
      </ReactFlow>
    </div>
  );
}
