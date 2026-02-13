import { memo, useMemo, useCallback, useState } from 'react';
import {
  ReactFlow,
  Handle,
  Position,
  Background,
  Panel,
  type NodeProps,
  type Node,
  type Edge,
  type NodeTypes,
  BackgroundVariant,
  useNodesState,
  useEdgesState
} from '@xyflow/react';
import { Database, Zap, Cpu, Shield, Plus, Minus, Share2, Activity, ShieldCheck, HardDrive, Settings2, X } from 'lucide-react';
import { cn } from '../App';
import { create } from '@bufbuild/protobuf';
import { motion } from 'framer-motion';
import {
  Postgres_Settings_Version,
  Postgres_Settings_StorageEngine,
  Postgres_Addons_DcsSchema,
  Postgres_Addons_Dcs_EtcdSchema,
  Postgres_PlacementSchema,
  Postgres_Placement_ColocateSchema,
  Postgres_Placement_Scope,
  Postgres_Placement_DedicatedSchema,
  Postgres_Addons_PoolingSchema,
  Postgres_Addons_Pooling_PgbouncerSchema,
  Postgres_Addons_Pooling_Pgbouncer_PoolMode,
  Postgres_SettingsSchema,
  Postgres_Instance_TemplateSchema,
  Postgres_Cluster_TemplateSchema,
  Postgres_Cluster_Template_TopologySchema,
  Postgres_AddonsSchema
} from '../proto/database/postgres_pb';
import { Database_TemplateSchema } from '../proto/database/database_pb';
import { HardwareSchema } from '../proto/deployment/deployment_pb';
import { Select } from './ui/Select';
import { HardwareInputs } from './ui/HardwareInputs';
import { Input } from './ui/Input';
import { BlueprintCard } from './ui/BlueprintCard';

// --- Custom Node Components ---

const StroppyNode = memo(() => (
  <div className="px-4 py-3 bg-accent text-accent-foreground border-2 border-primary shadow-[4px_4px_0px_0px_var(--primary)] skew-x-[-6deg]">
    <div className="flex items-center gap-2 skew-x-[6deg]">
      <Zap className="w-4 h-4 fill-current" />
      <div className="flex flex-col">
        <span className="text-[10px] font-black uppercase leading-none">LOAD_GENERATOR</span>
        <span className="text-xs font-black italic tracking-tighter">STROPPY_CORE</span>
      </div>
    </div>
    <Handle type="source" position={Position.Right} className="!bg-primary !border-none" />
  </div>
));

const PostgresNode = memo((props: NodeProps) => {
  const data = props.data as any;
  const isMaster = data.type === 'master';
  const hasMonitor = data.monitor;

  return (
    <div className={cn(
      "px-5 py-4 border-2 transition-all group relative min-w-[180px]",
      isMaster
        ? "bg-primary/20 border-primary shadow-[0_0_20px_rgba(var(--primary),0.2)]"
        : "bg-card/80 border-border hover:border-primary/50",
      props.selected && "border-primary ring-2 ring-primary/50 ring-offset-2 ring-offset-background"
    )}>
      {isMaster && (
        <div className="absolute -top-3 left-4 bg-primary text-primary-foreground px-2 py-0.5 text-[8px] font-black uppercase tracking-widest">
          LEADER
        </div>
      )}

      {hasMonitor && (
        <div className="absolute -top-3 right-4 flex gap-1">
          <div className="bg-accent text-accent-foreground px-1.5 py-0.5 text-[7px] font-black border border-primary/50">PROMETHEUS</div>
        </div>
      )}

      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-2 text-foreground">
          <Database className={cn("w-4 h-4", isMaster ? "text-primary" : "text-muted-foreground")} />
          <span className="text-[11px] font-bold font-mono tracking-tighter uppercase">
            {isMaster ? "PG_MASTER" : `PG_REPLICA_${data.index}`}
          </span>
        </div>

        <div className="flex flex-wrap gap-2 pt-1 border-t border-border/30">
          <div className="flex items-center gap-1 text-[8px] font-black text-muted-foreground uppercase opacity-60">
            <Cpu className="w-2.5 h-2.5 text-primary" /> {data.hardware?.cores}C
          </div>
          <div className="flex items-center gap-1 text-[8px] font-black text-muted-foreground uppercase opacity-60">
            <Share2 className="w-2.5 h-2.5 text-primary" /> {data.hardware?.memory}G
          </div>
          <div className="flex items-center gap-1 text-[8px] font-black text-muted-foreground uppercase opacity-60">
            <HardDrive className="w-2.5 h-2.5 text-primary" /> {data.hardware?.disk}G
          </div>
        </div>
      </div>
      <Handle type="target" position={Position.Left} className="!bg-primary !border-none" />
      <Handle type="source" position={Position.Right} className="!bg-primary !border-none" />
    </div>
  );
});

const PgbouncerNode = memo((props: NodeProps) => (
  <div className={cn(
    "px-4 py-2 bg-background border-2 border-accent text-accent shadow-[4px_4px_0px_0px_rgba(var(--accent),0.2)]",
    props.selected && "border-primary ring-2 ring-primary/50 ring-offset-2 ring-offset-background"
  )}>
    <div className="flex items-center gap-2">
      <ShieldCheck className="w-4 h-4" />
      <div className="flex flex-col">
        <span className="text-[8px] font-black uppercase leading-none">TRAFFIC_SHIELD</span>
        <span className="text-[10px] font-black italic tracking-tighter">PGBOUNCER</span>
      </div>
    </div>
    <Handle type="target" position={Position.Left} className="!bg-accent" />
    <Handle type="source" position={Position.Right} className="!bg-accent" />
  </div>
));

const EtcdNode = memo((props: NodeProps) => (
  <div className={cn(
    "p-2 bg-background border border-accent rounded-none rotate-45 group",
    props.selected && "border-2 border-primary ring-2 ring-primary/50 ring-offset-2 ring-offset-background"
  )}>
    <div className="-rotate-45">
      <Shield className="w-3 h-3 text-accent animate-pulse" />
    </div>
    <Handle type="target" position={Position.Top} className="opacity-0" />
    <Handle type="source" position={Position.Bottom} className="opacity-0" />
  </div>
));

const nodeTypes: NodeTypes = {
  stroppy: StroppyNode as any,
  postgres: PostgresNode as any,
  etcd: EtcdNode as any,
  pgbouncer: PgbouncerNode as any,
};

// --- Main Canvas Component ---

interface TopologyCanvasProps {
  databaseRef: any;
  onChange: (update: any) => void;
}

export const TopologyCanvas = ({ databaseRef, onChange }: TopologyCanvasProps) => {
  const [selectedElement, setSelectedElement] = useState<string | null>(null);

  const { initialNodes, initialEdges } = useMemo(() => {
    const nodes: Node[] = [];
    const edges: Edge[] = [];

    // 1. Add Stroppy Node
    nodes.push({
      id: 'stroppy',
      type: 'stroppy',
      position: { x: 0, y: 150 },
      data: { label: 'Stroppy Client' }
    });

    if (databaseRef.case === 'connectionString') {
      nodes.push({
        id: 'external-db',
        type: 'postgres',
        position: { x: 400, y: 150 },
        data: { type: 'master', index: 0, hardware: { cores: '?', memory: '?', disk: '?' }, label: 'External DB' }
      });
      edges.push({ id: 's-to-db', source: 'stroppy', target: 'external-db', animated: true });
    } else {
      const template = databaseRef.value.template;
      const isCluster = template.case === 'postgresCluster';

      let entryTargetId = 'master';

      // PgBouncer Layer
      const hasPgbouncer = isCluster && template.value.addons?.pooling?.pgbouncer?.enabled;
      if (hasPgbouncer) {
        nodes.push({
          id: 'pgbouncer',
          type: 'pgbouncer',
          position: { x: 250, y: 150 },
          data: { label: 'PgBouncer' }
        });
        edges.push({ id: 's-to-pgb', source: 'stroppy', target: 'pgbouncer', animated: true });
        entryTargetId = 'pgbouncer';
      }

      if (template.case === 'postgresInstance') {
        const instance = template.value;
        nodes.push({
          id: 'master',
          type: 'postgres',
          position: { x: 450, y: 150 },
          data: { type: 'master', index: 0, hardware: instance.hardware, monitor: true, label: 'Single Instance' }
        });
        edges.push({ id: 'entry-to-m', source: hasPgbouncer ? 'pgbouncer' : 'stroppy', target: 'master', animated: true });
      } else {
        const cluster = template.value;
        const topology = cluster.topology;

        // Master
        nodes.push({
          id: 'master',
          type: 'postgres',
          position: { x: 450, y: 150 },
          data: { type: 'master', index: 0, hardware: topology.masterHardware, monitor: topology.monitor, label: 'Postgres Leader' }
        });
        edges.push({ id: 'entry-to-m', source: entryTargetId === 'pgbouncer' ? 'pgbouncer' : 'stroppy', target: 'master', animated: true });

        // Replicas
        const count = topology.replicasCount || 0;
        for (let i = 0; i < count; i++) {
          const yPos = 150 + (i - (count - 1) / 2) * 120;
          nodes.push({
            id: `replica-${i}`,
            type: 'postgres',
            position: { x: 750, y: yPos },
            data: { type: 'replica', index: i + 1, hardware: topology.replicaHardware, monitor: topology.monitor, label: `Postgres Replica ${i + 1}` }
          });
          edges.push({
            id: `m-to-r-${i}`,
            source: 'master',
            target: `replica-${i}`,
            animated: true,
            style: { stroke: 'var(--primary)', strokeWidth: 2 }
          });
        }

        // ETCD DCS Addon
        if (cluster.addons?.dcs?.etcd) {
          const etcd = cluster.addons.dcs.etcd;
          const etcdCount = etcd.size || 3;
          const isDedicated = etcd.placement?.mode?.case === 'dedicated';

          if (isDedicated) {
            for (let i = 0; i < etcdCount; i++) {
              const yPos = 150 + (i - (etcdCount - 1) / 2) * 60;
              nodes.push({
                id: `etcd-${i}`,
                type: 'etcd',
                position: { x: 450, y: yPos - 150 }, // Orbit above master
                data: { label: 'ETCD Node' }
              });
              edges.push({ id: `etcd-to-m-${i}`, source: `etcd-${i}`, target: 'master', style: { opacity: 0.2, strokeDasharray: '5,5', stroke: 'var(--accent)' } });
            }
          } else {
            // Colocated marker
            nodes.push({
              id: 'etcd-colocate',
              type: 'etcd',
              position: { x: 430, y: 130 },
              data: { label: 'Colocated ETCD' },
              draggable: false
            });
          }
        }
      }
    }

    return { initialNodes: nodes, initialEdges: edges };
  }, [databaseRef]);

  const [nodes, setNodes, onNodesChange] = useNodesState(initialNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges);

  // Sync nodes when databaseRef changes (basic sync)
  useMemo(() => {
    setNodes(initialNodes);
    setEdges(initialEdges);
  }, [initialNodes, initialEdges, setNodes, setEdges]);

  const onNodeClick = useCallback((_: any, node: Node) => {
    setSelectedElement(node.id);
  }, []);

  const onPaneClick = useCallback(() => {
    setSelectedElement(null);
  }, []);

  const updateTest = (updates: any) => {
    onChange(updates);
  };

  const renderConfigPanel = () => {
    if (!selectedElement) return null;

    let content = null;
    let title = "Configuration";

    if (selectedElement === 'master' || selectedElement.startsWith('replica-')) {
      const isMaster = selectedElement === 'master';
      title = isMaster ? "Postgres Leader" : "Postgres Replica";

      if (databaseRef.case === 'databaseTemplate') {
        const ref = databaseRef.value;
        const isInstance = ref.template.case === 'postgresInstance';

        content = (
          <div className="space-y-4">
            <Select label="Template Type" fieldName="template.case" value={ref.template.case} onChange={(v) => {
              if (v === 'postgresInstance') updateTest({ databaseRef: { case: "databaseTemplate", value: create(Database_TemplateSchema, { template: { case: "postgresInstance", value: create(Postgres_Instance_TemplateSchema, { settings: create(Postgres_SettingsSchema, { version: Postgres_Settings_Version.VERSION_17, storageEngine: Postgres_Settings_StorageEngine.HEAP }), hardware: create(HardwareSchema, { cores: 4, memory: 8, disk: 100 }) }) } }) } });
              else updateTest({ databaseRef: { case: "databaseTemplate", value: create(Database_TemplateSchema, { template: { case: "postgresCluster", value: create(Postgres_Cluster_TemplateSchema, { topology: create(Postgres_Cluster_Template_TopologySchema, { replicasCount: 2, monitor: true, settings: create(Postgres_SettingsSchema, { version: Postgres_Settings_Version.VERSION_17, storageEngine: Postgres_Settings_StorageEngine.HEAP }), masterHardware: create(HardwareSchema, { cores: 4, memory: 8, disk: 100 }), replicaHardware: create(HardwareSchema, { cores: 2, memory: 4, disk: 50 }) }), addons: create(Postgres_AddonsSchema, {}) }) } }) } });
            }} options={[{ label: "Single Instance", value: "postgresInstance" }, { label: "HA Cluster (Patroni)", value: "postgresCluster" }]} />

            <div className="grid grid-cols-2 gap-2">
              <Select label="Version" value={isInstance ? (ref.template.value as any).settings?.version : (ref.template.value as any).topology?.settings?.version} onChange={(v) => {
                const newRef = JSON.parse(JSON.stringify(ref));
                if (newRef.template.case === 'postgresInstance') newRef.template.value.settings.version = v;
                else newRef.template.value.topology.settings.version = v;
                updateTest({ databaseRef: { case: "databaseTemplate", value: newRef } });
              }} options={[{ label: "v17", value: Postgres_Settings_Version.VERSION_17 }, { label: "v16", value: Postgres_Settings_Version.VERSION_16 }]} />

              <Select label="Engine" value={isInstance ? (ref.template.value as any).settings?.storageEngine : (ref.template.value as any).topology?.settings?.storageEngine} onChange={(v) => {
                const newRef = JSON.parse(JSON.stringify(ref));
                if (newRef.template.case === 'postgresInstance') newRef.template.value.settings.storageEngine = v;
                else newRef.template.value.topology.settings.storageEngine = v;
                updateTest({ databaseRef: { case: "databaseTemplate", value: newRef } });
              }} options={[{ label: "Heap", value: Postgres_Settings_StorageEngine.HEAP }, { label: "OrioleDB", value: Postgres_Settings_StorageEngine.ORIOLEDB }]} />
            </div>

            <HardwareInputs
              label="Resources"
              hardware={isMaster ? (isInstance ? (ref.template.value as any).hardware : (ref.template.value as any).topology.masterHardware) : (ref.template.value as any).topology.replicaHardware}
              onChange={(h) => {
                const newRef = JSON.parse(JSON.stringify(ref));
                if (isInstance) newRef.template.value.hardware = h;
                else if (isMaster) newRef.template.value.topology.masterHardware = h;
                else newRef.template.value.topology.replicaHardware = h;
                updateTest({ databaseRef: { case: "databaseTemplate", value: newRef } });
              }}
            />

            {!isInstance && isMaster && (
              <div className="pt-4 border-t border-border flex items-center justify-between">
                <span className="text-[10px] font-black uppercase tracking-widest text-muted-foreground">Replicas: {(ref.template.value as any).topology.replicasCount}</span>
                <div className="flex gap-2">
                  <button onClick={() => {
                    const newRef = JSON.parse(JSON.stringify(ref));
                    newRef.template.value.topology.replicasCount = Math.max(0, newRef.template.value.topology.replicasCount - 1);
                    updateTest({ databaseRef: { case: "databaseTemplate", value: newRef } });
                  }} className="p-1 border border-border hover:border-primary transition-all"><Minus className="w-3 h-3" /></button>
                  <button onClick={() => {
                    const newRef = JSON.parse(JSON.stringify(ref));
                    newRef.template.value.topology.replicasCount += 1;
                    updateTest({ databaseRef: { case: "databaseTemplate", value: newRef } });
                  }} className="p-1 border border-border hover:border-primary transition-all"><Plus className="w-3 h-3" /></button>
                </div>
              </div>
            )}
          </div>
        );
      }
    } else if (selectedElement === 'pgbouncer' || selectedElement === 'etcd' || selectedElement.startsWith('etcd-') || selectedElement === 'etcd-colocate') {
      const isPgbouncer = selectedElement === 'pgbouncer';
      title = isPgbouncer ? "Traffic Pooling" : "DCS (ETCD)";

      if (databaseRef.case === 'databaseTemplate' && databaseRef.value.template.case === 'postgresCluster') {
        const ref = databaseRef.value;
        if (isPgbouncer) {
          content = (
            <div className="space-y-4">
              <BlueprintCard
                title="PgBouncer" subtitle="Active Multiplexer" icon={Activity} active={true}
                onClick={() => {
                  const newRef = JSON.parse(JSON.stringify(ref));
                  delete newRef.template.value.addons.pooling;
                  setSelectedElement(null);
                  updateTest({ databaseRef: { case: "databaseTemplate", value: newRef } });
                }}
              >
                <div className="grid grid-cols-2 gap-2">
                  <Select label="Mode" className="h-6 text-[10px]" value={Postgres_Addons_Pooling_Pgbouncer_PoolMode.TRANSACTION} onChange={() => { }} options={[{ label: "Transaction", value: 1 }, { label: "Session", value: 2 }]} />
                  <Input label="Size" className="h-6 text-[10px]" type="number" value={20} onChange={() => { }} />
                </div>
              </BlueprintCard>
            </div>
          );
        } else {
          content = (
            <div className="space-y-4">
              <BlueprintCard
                title="ETCD" subtitle="Distributed Config" icon={Shield} active={true}
                onClick={() => {
                  const newRef = JSON.parse(JSON.stringify(ref));
                  delete newRef.template.value.addons.dcs;
                  setSelectedElement(null);
                  updateTest({ databaseRef: { case: "databaseTemplate", value: newRef } });
                }}
              >
                <div className="grid grid-cols-2 gap-2">
                  <Input label="Size" type="number" className="h-6 text-[10px]" value={ref.template.value.addons?.dcs?.etcd?.size || 0} onChange={(e) => {
                    const newRef = JSON.parse(JSON.stringify(ref)); newRef.template.value.addons.dcs.etcd.size = parseInt(e.target.value); updateTest({ databaseRef: { case: "databaseTemplate", value: newRef } });
                  }} />
                  <Select label="Placement" className="h-6 text-[10px]" value={ref.template.value.addons?.dcs?.etcd?.placement?.mode?.case} onChange={(v) => {
                    const newRef = JSON.parse(JSON.stringify(ref));
                    if (v === 'dedicated') newRef.template.value.addons.dcs.etcd.placement = create(Postgres_PlacementSchema, { mode: { case: "dedicated", value: create(Postgres_Placement_DedicatedSchema, { instancesCount: newRef.template.value.addons.dcs.etcd.size, hardware: create(HardwareSchema, { cores: 1, memory: 2, disk: 10 }) }) } });
                    else newRef.template.value.addons.dcs.etcd.placement = create(Postgres_PlacementSchema, { mode: { case: "colocate", value: create(Postgres_Placement_ColocateSchema, { scope: Postgres_Placement_Scope.ALL_NODES }) } });
                    updateTest({ databaseRef: { case: "databaseTemplate", value: newRef } });
                  }} options={[{ label: "Colocated", value: "colocate" }, { label: "Dedicated", value: "dedicated" }]} />
                </div>
              </BlueprintCard>
            </div>
          );
        }
      }
    } else if (selectedElement === 'stroppy') {
      title = "Simulation Engine";
      content = (
        <div className="space-y-2 text-[10px] font-mono text-muted-foreground uppercase leading-relaxed">
          <p>The Stroppy core engine generates synthetic workload and monitors performance metrics in real-time.</p>
          <p className="pt-2 text-primary border-t border-border/20">Configure client resources in the previous step.</p>
        </div>
      );
    }

    return (
      <Panel position="top-left" className="m-8">
        <motion.div
          initial={{ opacity: 0, x: -50 }}
          animate={{ opacity: 1, x: 0 }}
          className="w-80 bg-card/95 backdrop-blur-xl border-2 border-primary shadow-[10px_10px_0px_0px_rgba(var(--primary),0.2)]"
        >
          <div className="p-4 border-b border-primary/20 flex items-center justify-between bg-primary/10">
            <div className="flex items-center gap-2">
              <Settings2 className="w-4 h-4 text-primary" />
              <h3 className="text-xs font-black uppercase tracking-widest">{title}</h3>
            </div>
            <button onClick={() => setSelectedElement(null)} className="p-1 hover:bg-primary/20 transition-colors">
              <X className="w-4 h-4" />
            </button>
          </div>
          <div className="p-6">
            {content}
          </div>
        </motion.div>
      </Panel>
    );
  };

  return (
    <div className="w-full h-full bg-black/40 border-2 border-primary/20 relative group overflow-hidden">
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_center,rgba(var(--primary),0.05),transparent)] pointer-events-none" />

      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        nodeTypes={nodeTypes}
        onNodeClick={onNodeClick}
        onPaneClick={onPaneClick}
        fitView
        proOptions={{ hideAttribution: true }}
        minZoom={0.2}
        maxZoom={2}
      >
        <Background color="#222" gap={20} variant={BackgroundVariant.Lines} />

        {renderConfigPanel()}

        <Panel position="top-right" className="flex flex-col gap-2">
          <div className="bg-card/90 border-2 border-primary p-2 flex flex-col gap-2 shadow-[4px_4px_0px_0px_var(--primary)] backdrop-blur-md">
            <h4 className="text-[8px] font-black uppercase tracking-widest text-center border-b border-primary/20 pb-1">Quick Addons</h4>
            <div className="grid grid-cols-2 gap-1">
              <button
                disabled={databaseRef.case !== 'databaseTemplate' || databaseRef.value.template.case !== 'postgresCluster' || !!databaseRef.value.template.value.addons?.dcs}
                onClick={() => {
                  const ref = JSON.parse(JSON.stringify(databaseRef.value));
                  if (!ref.template.value.addons) ref.template.value.addons = create(Postgres_AddonsSchema, {});
                  ref.template.value.addons.dcs = create(Postgres_Addons_DcsSchema, { etcd: create(Postgres_Addons_Dcs_EtcdSchema, { size: 3, monitor: true, placement: create(Postgres_PlacementSchema, { mode: { case: "colocate", value: create(Postgres_Placement_ColocateSchema, { scope: Postgres_Placement_Scope.ALL_NODES }) } }) }) });
                  updateTest({ databaseRef: { case: "databaseTemplate", value: ref } });
                  setSelectedElement('etcd-colocate');
                }}
                className="p-2 border border-border hover:border-primary disabled:opacity-20 transition-all flex flex-col items-center gap-1"
              >
                <Shield className="w-3 h-3" />
                <span className="text-[7px] font-black uppercase">DCS</span>
              </button>
              <button
                disabled={databaseRef.case !== 'databaseTemplate' || databaseRef.value.template.case !== 'postgresCluster' || !!databaseRef.value.template.value.addons?.pooling}
                onClick={() => {
                  const ref = JSON.parse(JSON.stringify(databaseRef.value));
                  if (!ref.template.value.addons) ref.template.value.addons = create(Postgres_AddonsSchema, {});
                  ref.template.value.addons.pooling = create(Postgres_Addons_PoolingSchema, { pgbouncer: create(Postgres_Addons_Pooling_PgbouncerSchema, { enabled: true, poolSize: 20, poolMode: Postgres_Addons_Pooling_Pgbouncer_PoolMode.TRANSACTION, monitor: true, placement: create(Postgres_PlacementSchema, { mode: { case: "colocate", value: create(Postgres_Placement_ColocateSchema, { scope: Postgres_Placement_Scope.MASTER }) } }) }) });
                  updateTest({ databaseRef: { case: "databaseTemplate", value: ref } });
                  setSelectedElement('pgbouncer');
                }}
                className="p-2 border border-border hover:border-primary disabled:opacity-20 transition-all flex flex-col items-center gap-1"
              >
                <Activity className="w-3 h-3" />
                <span className="text-[7px] font-black uppercase">POOL</span>
              </button>
            </div>
          </div>
        </Panel>

        <Panel position="bottom-left" className="bg-background/80 border border-primary/20 p-2 backdrop-blur-sm">
          <div className="text-[10px] font-black uppercase italic text-primary tracking-[0.2em] select-none flex items-center gap-2">
            <Activity className="w-3 h-3 animate-pulse" /> HOLOGRAPHIC_ARCHITECT_v4.5
          </div>
        </Panel>
      </ReactFlow>
    </div>
  );
};
