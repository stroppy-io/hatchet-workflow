import { memo, useMemo, useCallback } from 'react';
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
  BackgroundVariant
} from '@xyflow/react';
import { Database, Zap, Cpu, Shield, Plus, Minus, Share2, Activity, ShieldCheck, HardDrive } from 'lucide-react';
import { cn } from '../App';

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
        : "bg-card/80 border-border hover:border-primary/50"
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

const PgbouncerNode = memo(() => (
  <div className="px-4 py-2 bg-background border-2 border-accent text-accent shadow-[4px_4px_0px_0px_rgba(var(--accent),0.2)]">
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

const EtcdNode = memo(() => (
  <div className="p-2 bg-background border border-accent rounded-none rotate-45 group">
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
  const { nodes, edges } = useMemo(() => {
    const nodes: Node[] = [];
    const edges: Edge[] = [];

    // 1. Add Stroppy Node
    nodes.push({
      id: 'stroppy',
      type: 'stroppy',
      position: { x: 0, y: 150 },
      data: {}
    });

    if (databaseRef.case === 'connectionString') {
      nodes.push({
        id: 'external-db',
        type: 'postgres',
        position: { x: 400, y: 150 },
        data: { type: 'master', index: 0, hardware: { cores: '?', memory: '?', disk: '?' } }
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
          data: {}
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
          data: { type: 'master', index: 0, hardware: instance.hardware, monitor: true }
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
          data: { type: 'master', index: 0, hardware: topology.masterHardware, monitor: topology.monitor }
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
            data: { type: 'replica', index: i + 1, hardware: topology.replicaHardware, monitor: topology.monitor }
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
                data: {}
              });
              edges.push({ id: `etcd-to-m-${i}`, source: `etcd-${i}`, target: 'master', style: { opacity: 0.2, strokeDasharray: '5,5', stroke: 'var(--accent)' } });
            }
          } else {
            // Visualize as indicator on PG nodes
            nodes.push({ id: 'etcd-colocate-tag', type: 'etcd', position: { x: 430, y: 130 }, data: {}, dragHandle: '.nothing' });
          }
        }
      }
    }

    return { nodes, edges };
  }, [databaseRef]);

  const onAddReplica = useCallback(() => {
    if (databaseRef.case !== 'databaseTemplate' || databaseRef.value.template.case !== 'postgresCluster') return;
    const ref = JSON.parse(JSON.stringify(databaseRef.value));
    ref.template.value.topology.replicasCount = (ref.template.value.topology.replicasCount || 0) + 1;
    onChange({ case: 'databaseTemplate', value: ref });
  }, [databaseRef, onChange]);

  const onRemoveReplica = useCallback(() => {
    if (databaseRef.case !== 'databaseTemplate' || databaseRef.value.template.case !== 'postgresCluster') return;
    const ref = JSON.parse(JSON.stringify(databaseRef.value));
    ref.template.value.topology.replicasCount = Math.max(0, (ref.template.value.topology.replicasCount || 0) - 1);
    onChange({ case: 'databaseTemplate', value: ref });
  }, [databaseRef, onChange]);

  return (
    <div className="w-full h-full bg-black/40 border-2 border-primary/20 relative group overflow-hidden">
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_center,rgba(var(--primary),0.05),transparent)] pointer-events-none" />
      
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        fitView
        proOptions={{ hideAttribution: true }}
        minZoom={0.2}
        maxZoom={2}
      >
        <Background color="#222" gap={20} variant={BackgroundVariant.Lines} />
        
        <Panel position="top-right" className="flex flex-col gap-2">
          {databaseRef.case === 'databaseTemplate' && databaseRef.value.template.case === 'postgresCluster' && (
            <div className="bg-card/90 border-2 border-primary p-1 flex flex-col gap-1 shadow-[4px_4px_0px_0px_var(--primary)] backdrop-blur-md">
              <button onClick={onAddReplica} className="p-2 hover:bg-primary hover:text-primary-foreground transition-all">
                <Plus className="w-4 h-4" />
              </button>
              <div className="h-[1px] bg-primary/20 mx-2" />
              <button onClick={onRemoveReplica} className="p-2 hover:bg-primary hover:text-primary-foreground transition-all">
                <Minus className="w-4 h-4" />
              </button>
            </div>
          )}
        </Panel>

        <Panel position="bottom-left" className="bg-background/80 border border-primary/20 p-2 backdrop-blur-sm">
          <div className="text-[10px] font-black uppercase italic text-primary tracking-[0.2em] select-none flex items-center gap-2">
            <Activity className="w-3 h-3 animate-pulse" /> HOLOGRAPHIC_ARCHITECT_v4.0
          </div>
        </Panel>
      </ReactFlow>
    </div>
  );
};
