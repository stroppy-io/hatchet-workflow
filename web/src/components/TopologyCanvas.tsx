import { memo, useCallback, useMemo, useState, useEffect } from 'react';
import {
  ReactFlow,
  Handle,
  Position,
  Background,
  Panel,
  useNodesState,
  useEdgesState,
  useReactFlow,
  ReactFlowProvider,
  BackgroundVariant,
  type Node,
  type Edge,
  type NodeProps,
  type NodeTypes,
  MiniMap,
  Controls
} from '@xyflow/react';
import { 
  Database, 
  Shield, 
  Activity, 
  Cpu, 
  HardDrive, 
  MousePointer2, 
  Trash2, 
  Plus, 
  Settings2, 
  ChevronRight,
  Monitor,
  Box,
  X,
  Zap
} from 'lucide-react';
import { create } from '@bufbuild/protobuf';
import { motion } from 'framer-motion';
import { cn } from '../App';

import {
  Postgres_PostgresService_Role,
  Postgres_NodeSchema,
  Postgres_PostgresServiceSchema,
  Postgres_EtcdServiceSchema,
  Postgres_PgbouncerServiceSchema,
  Postgres_PgbouncerConfigSchema,
  Postgres_PgbouncerConfig_PoolMode,
  Postgres_Settings_Version,
  Postgres_Settings_StorageEngine,
  Postgres_ClusterSchema,
  Postgres_SettingsSchema,
  type Postgres_Node
} from '../proto/database/postgres_pb';
import { HardwareSchema } from '../proto/deployment/deployment_pb';
import { StroppyCliSchema, StroppyCli_Workload } from '../proto/stroppy/test_pb';
import { Select } from './ui/Select';
import { HardwareInputs } from './ui/HardwareInputs';
import { Input } from './ui/Input';

// --- Types ---

interface TopologyCanvasProps {
  test: any;
  onChange: (update: any) => void;
}

// --- Custom Nodes ---

const LoadNode = memo(({ data, selected }: NodeProps) => {
  const loadData = data as any;
  return (
    <motion.div
      initial={{ scale: 0.9, opacity: 0 }}
      animate={{ scale: 1, opacity: 1 }}
      className={cn(
        "group relative min-w-[200px] bg-accent/90 backdrop-blur-md border-2 transition-all duration-300 skew-x-[-6deg]",
        selected 
          ? "border-primary shadow-[0_0_30px_rgba(var(--primary),0.3)]" 
          : "border-primary/40 shadow-xl"
      )}
    >
      <div className="px-4 py-3 skew-x-[6deg]">
        <div className="flex items-center gap-3 mb-2">
          <Zap className="w-5 h-5 text-primary fill-primary/20" />
          <div className="flex flex-col">
            <span className="text-[10px] font-black uppercase leading-none tracking-tighter text-primary">Load_Generator</span>
            <span className="text-xs font-black italic tracking-tighter uppercase">{loadData.workload === StroppyCli_Workload.TPCC ? 'TPC-C' : 'TPC-B'} Core</span>
          </div>
        </div>
        
        <div className="flex gap-3 text-[8px] font-mono text-accent-foreground/60 uppercase border-t border-primary/20 pt-2 mt-2">
          <div className="flex items-center gap-1"><Cpu className="w-2.5 h-2.5" />{loadData.hardware?.cores}C</div>
          <div className="flex items-center gap-1"><Monitor className="w-2.5 h-2.5" />{loadData.hardware?.memory}G</div>
        </div>
      </div>
      <Handle type="source" position={Position.Right} className="!bg-primary !w-2 !h-2 !border-none" />
    </motion.div>
  );
});

const SmartNode = memo(({ data, selected }: NodeProps) => {
  const node = data as unknown as Postgres_Node;
  const isMaster = node.postgres?.role === Postgres_PostgresService_Role.MASTER;
  
  return (
    <motion.div
      initial={{ scale: 0.9, opacity: 0 }}
      animate={{ scale: 1, opacity: 1 }}
      className={cn(
        "group relative min-w-[220px] bg-card/90 backdrop-blur-md border-2 transition-all duration-300",
        selected 
          ? "border-primary shadow-[0_0_30px_rgba(var(--primary),0.2)]" 
          : "border-border hover:border-primary/40 shadow-xl",
        isMaster && "ring-1 ring-primary/30"
      )}
    >
      {/* Header */}
      <div className={cn(
        "px-4 py-2 border-b flex items-center justify-between",
        isMaster ? "bg-primary/10 border-primary/20" : "bg-background/50 border-border"
      )}>
        <div className="flex items-center gap-2">
          <Box className={cn("w-3 h-3", isMaster ? "text-primary" : "text-muted-foreground")} />
          <span className="text-[10px] font-black uppercase tracking-tighter truncate max-w-[120px]">
            {node.name || "UNNAMED_NODE"}
          </span>
        </div>
        {isMaster && (
          <span className="bg-primary text-primary-foreground text-[7px] font-black px-1.5 py-0.5 uppercase tracking-widest">
            Leader
          </span>
        )}
      </div>

      {/* Services Grid */}
      <div className="p-4 space-y-3">
        {/* Hardware Mini-view */}
        <div className="flex gap-3 text-[8px] font-mono text-muted-foreground uppercase border-b border-border/10 pb-2">
          <div className="flex items-center gap-1"><Cpu className="w-2.5 h-2.5" />{node.hardware?.cores}C</div>
          <div className="flex items-center gap-1"><Monitor className="w-2.5 h-2.5" />{node.hardware?.memory}G</div>
          <div className="flex items-center gap-1"><HardDrive className="w-2.5 h-2.5" />{node.hardware?.disk}G</div>
        </div>

        {/* Active Services List */}
        <div className="space-y-1.5">
          {node.postgres && (
            <div className="flex items-center gap-2 text-[9px] font-bold uppercase text-primary">
              <Database className="w-3 h-3" /> Postgres {node.postgres.role === Postgres_PostgresService_Role.MASTER ? 'Leader' : 'Replica'}
            </div>
          )}
          {node.etcd && (
            <div className="flex items-center gap-2 text-[9px] font-bold uppercase text-accent">
              <Shield className="w-3 h-3" /> Etcd Consensus
            </div>
          )}
          {node.pgbouncer && (
            <div className="flex items-center gap-2 text-[9px] font-bold uppercase text-accent">
              <Activity className="w-3 h-3" /> Connection Pool
            </div>
          )}
          {!node.postgres && !node.etcd && !node.pgbouncer && (
            <div className="text-[9px] italic text-muted-foreground py-2 text-center border border-dashed border-border">
              No active services
            </div>
          )}
        </div>
      </div>

      <Handle type="target" position={Position.Left} className="!bg-primary/50 !w-2 !h-2 !border-none" />
      <Handle type="source" position={Position.Right} className="!bg-primary/50 !w-2 !h-2 !border-none" />
    </motion.div>
  );
});

const nodeTypes: NodeTypes = {
  smart: SmartNode,
  load: LoadNode,
};

// --- Sidebar Components ---

const Blueprint = ({ icon: Icon, title, type, description }: any) => {
  const onDragStart = (event: React.DragEvent) => {
    event.dataTransfer.setData('application/reactflow', type);
    event.dataTransfer.effectAllowed = 'move';
  };

  return (
    <div
      draggable
      onDragStart={onDragStart}
      className="p-4 bg-background border-2 border-border hover:border-primary transition-all cursor-grab active:cursor-grabbing group"
    >
      <div className="flex items-center gap-3 mb-2">
        <div className="p-2 bg-primary/5 group-hover:bg-primary/10 transition-colors">
          <Icon className="w-5 h-5 text-primary" />
        </div>
        <div className="text-[11px] font-black uppercase tracking-tight">{title}</div>
      </div>
      <p className="text-[8px] text-muted-foreground uppercase leading-relaxed font-mono">{description}</p>
    </div>
  );
};

// --- Main Designer Component ---

const TopologyDesigner = ({ test, onChange }: TopologyCanvasProps) => {
  const { screenToFlowPosition } = useReactFlow();
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const databaseRef = test.databaseRef;

  // 1. Sync React Flow state with databaseRef and test
  const elements = useMemo(() => {
    const nodes: Node[] = [];
    const edges: Edge[] = [];

    // Add Load Generator if present
    if (test.stroppyCli) {
      nodes.push({
        id: 'stroppy-node',
        type: 'load',
        position: { x: 50, y: 250 },
        data: { 
          workload: test.stroppyCli.workload,
          hardware: test.stroppyHardware,
          version: test.stroppyCli.version
        }
      });
    }

    if (databaseRef.case === 'databaseTemplate' && databaseRef.value.template.case === 'postgresCluster') {
      const cluster = databaseRef.value.template.value;
      
      // Master Discovery
      const masterNodeIdx = cluster.nodes.findIndex((n: any) => n.postgres?.role === Postgres_PostgresService_Role.MASTER);
      const masterId = masterNodeIdx !== -1 ? `node-${masterNodeIdx}` : null;

      // Link Load Generator to Master or first Pgbouncer
      const firstPgbouncerIdx = cluster.nodes.findIndex((n: any) => n.pgbouncer);
      const entryId = firstPgbouncerIdx !== -1 ? `node-${firstPgbouncerIdx}` : masterId;

      if (test.stroppyCli && entryId) {
        edges.push({
          id: 'edge-load-to-entry',
          source: 'stroppy-node',
          target: entryId,
          animated: true,
          style: { stroke: 'var(--accent)', strokeWidth: 3 }
        });
      }

      cluster.nodes.forEach((n: any, i: number) => {
        const nodeId = `node-${i}`;
        nodes.push({
          id: nodeId,
          type: 'smart',
          position: { x: 400 + (i % 3) * 300, y: 100 + Math.floor(i / 3) * 250 },
          data: { ...n }
        });

        if (i !== masterNodeIdx && masterId && n.postgres) {
          edges.push({
            id: `edge-${masterId}-${nodeId}`,
            source: masterId,
            target: nodeId,
            animated: true,
            style: { stroke: 'var(--primary)', strokeWidth: 2, opacity: 0.4 }
          });
        }
      });
    }

    return { nodes, edges };
  }, [databaseRef, test]);

  const [nodes, setNodes, onNodesChange] = useNodesState(elements.nodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(elements.edges);

  useEffect(() => {
    setNodes((nds) => {
      return elements.nodes.map((newNo: Node) => {
        const existingNode = nds.find((n) => n.id === newNo.id);
        return existingNode ? { ...newNo, position: existingNode.position } : newNo;
      });
    });
    setEdges(elements.edges);
  }, [elements, setNodes, setEdges]);

  const onDrop = useCallback(
    (event: React.DragEvent) => {
      event.preventDefault();
      const type = event.dataTransfer.getData('application/reactflow');
      if (!type) return;

      if (type === 'load') {
        onChange({
          stroppyCli: create(StroppyCliSchema, { version: "v1.0.0", workload: StroppyCli_Workload.TPCC }),
          stroppyHardware: create(HardwareSchema, { cores: 2, memory: 4, disk: 20 })
        });
        return;
      }

      const position = screenToFlowPosition({ x: event.clientX, y: event.clientY });

      if (databaseRef.case === 'databaseTemplate' && databaseRef.value.template.case === 'postgresCluster') {
        const ref = JSON.parse(JSON.stringify(databaseRef.value));
        const nodeCount = ref.template.value.nodes.length;

        let newNode = create(Postgres_NodeSchema, {
          hardware: create(HardwareSchema, { cores: 2, memory: 4, disk: 50 })
        });

        if (type === 'postgres') {
          newNode.name = `pg-node-${nodeCount + 1}`;
          newNode.postgres = create(Postgres_PostgresServiceSchema, { role: Postgres_PostgresService_Role.REPLICA });
        } else if (type === 'etcd') {
          newNode.name = `etcd-node-${nodeCount + 1}`;
          newNode.etcd = create(Postgres_EtcdServiceSchema, { monitor: false });
        } else if (type === 'pgbouncer') {
          newNode.name = `pgb-node-${nodeCount + 1}`;
          newNode.pgbouncer = create(Postgres_PgbouncerServiceSchema, { 
            config: create(Postgres_PgbouncerConfigSchema, { poolMode: Postgres_PgbouncerConfig_PoolMode.TRANSACTION, poolSize: 20 }),
            monitor: false,
          });
        }

        ref.template.value.nodes.push(newNode);
        onChange({ databaseRef: { case: "databaseTemplate", value: ref } });
        
        // Use position to place node precisely where it was dropped
        setNodes((nds) => nds.concat({
          id: `node-${nodeCount}`,
          type: 'smart',
          position,
          data: newNode as any
        }));
      }
    },
    [screenToFlowPosition, databaseRef, onChange]
  );

  const updateNodeData = (nodeIdx: number, updates: any) => {
    const ref = JSON.parse(JSON.stringify(databaseRef.value));
    ref.template.value.nodes[nodeIdx] = { ...ref.template.value.nodes[nodeIdx], ...updates };
    onChange({ databaseRef: { case: "databaseTemplate", value: ref } });
  };

  const deleteNode = (nodeId: string) => {
    if (nodeId === 'stroppy-node') {
      onChange({ stroppyCli: undefined, stroppyHardware: undefined });
      setSelectedNodeId(null);
      return;
    }
    const idx = parseInt(nodeId.replace('node-', ''));
    const ref = JSON.parse(JSON.stringify(databaseRef.value));
    ref.template.value.nodes.splice(idx, 1);
    setSelectedNodeId(null);
    onChange({ databaseRef: { case: "databaseTemplate", value: ref } });
  };

  const renderProperties = () => {
    if (!selectedNodeId) return null;

    if (selectedNodeId === 'stroppy-node' && test.stroppyCli) {
      return (
        <Panel position="top-right" className="m-4">
          <motion.div initial={{ x: 300, opacity: 0 }} animate={{ x: 0, opacity: 1 }} className="w-96 bg-card/95 backdrop-blur-2xl border-2 border-primary shadow-2xl overflow-hidden flex flex-col">
            <div className="p-4 bg-primary/10 border-b border-primary/20 flex items-center justify-between">
              <div className="flex items-center gap-2"><Zap className="w-4 h-4 text-primary" /><h3 className="text-xs font-black uppercase tracking-widest text-primary">Load Generator Config</h3></div>
              <button onClick={() => setSelectedNodeId(null)}><X className="w-4 h-4 hover:text-primary transition-colors" /></button>
            </div>
            <div className="p-6 space-y-6">
               <Input label="Version" value={test.stroppyCli.version} onChange={(e) => onChange({ stroppyCli: { ...test.stroppyCli, version: e.target.value } })} />
               <Select label="Workload" value={test.stroppyCli.workload} onChange={(v) => onChange({ stroppyCli: { ...test.stroppyCli, workload: v } })} options={[{ label: "TPC-C", value: StroppyCli_Workload.TPCC }, { label: "TPC-B", value: StroppyCli_Workload.TPCB }]} />
               <HardwareInputs label="Client Resources" hardware={test.stroppyHardware} onChange={(h) => onChange({ stroppyHardware: h })} />
               <button onClick={() => deleteNode('stroppy-node')} className="w-full py-4 border-2 border-destructive text-destructive text-[10px] font-black uppercase hover:bg-destructive hover:text-white transition-all flex items-center justify-center gap-2 mt-4"><Trash2 className="w-4 h-4" /> Remove Generator</button>
            </div>
          </motion.div>
        </Panel>
      );
    }

    const idx = parseInt(selectedNodeId.replace('node-', ''));
    const node = databaseRef.value.template.value.nodes[idx];
    if (!node) return null;

    return (
      <Panel position="top-right" className="m-4">
        <motion.div
          initial={{ x: 300, opacity: 0 }}
          animate={{ x: 0, opacity: 1 }}
          className="w-96 bg-card/95 backdrop-blur-2xl border-2 border-primary shadow-2xl overflow-hidden flex flex-col max-h-[80vh]"
        >
          <div className="p-4 bg-primary/10 border-b border-primary/20 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Settings2 className="w-4 h-4 text-primary" />
              <h3 className="text-xs font-black uppercase tracking-widest text-primary">Node Configuration</h3>
            </div>
            <button onClick={() => setSelectedNodeId(null)}><X className="w-4 h-4 hover:text-primary transition-colors" /></button>
          </div>

          <div className="p-6 space-y-8 overflow-y-auto custom-scrollbar">
            {/* Identity */}
            <div className="space-y-4">
              <Input label="System ID" value={node.name} onChange={(e) => updateNodeData(idx, { name: e.target.value })} />
              <HardwareInputs label="Node Resources" hardware={node.hardware} onChange={(h) => updateNodeData(idx, { hardware: h })} />
            </div>

            {/* Service Modules */}
            <div className="space-y-4 pt-4 border-t border-border">
              <h4 className="text-[10px] font-black uppercase tracking-[0.3em] text-muted-foreground flex items-center gap-2">
                <ChevronRight className="w-3 h-3 text-primary" /> Module Matrix
              </h4>

              <div className="space-y-3">
                {/* Postgres Module */}
                <div className={cn("p-4 border-2 transition-all", node.postgres ? "bg-primary/5 border-primary/50" : "bg-background border-border opacity-50")}>
                  <div className="flex items-center justify-between mb-3">
                    <div className="flex items-center gap-2">
                      <Database className="w-4 h-4" />
                      <span className="text-[11px] font-black uppercase">PostgreSQL</span>
                    </div>
                    <input 
                      type="checkbox" 
                      checked={!!node.postgres} 
                      onChange={(e) => {
                        const postgres = e.target.checked ? create(Postgres_PostgresServiceSchema, { role: Postgres_PostgresService_Role.REPLICA }) : undefined;
                        updateNodeData(idx, { postgres });
                      }} 
                    />
                  </div>
                  {node.postgres && (
                    <Select 
                      label="Service Role" 
                      value={node.postgres.role} 
                      onChange={(v) => {
                        const p = JSON.parse(JSON.stringify(node.postgres));
                        p.role = v;
                        updateNodeData(idx, { postgres: p });
                      }}
                      options={[
                        { label: "Cluster Leader", value: Postgres_PostgresService_Role.MASTER },
                        { label: "Active Replica", value: Postgres_PostgresService_Role.REPLICA }
                      ]}
                    />
                  )}
                </div>

                {/* Etcd Module */}
                <div className={cn("p-4 border-2 transition-all", node.etcd ? "bg-accent/5 border-accent/50" : "bg-background border-border opacity-50")}>
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Shield className="w-4 h-4" />
                      <span className="text-[11px] font-black uppercase">Etcd Consensus</span>
                    </div>
                    <input 
                      type="checkbox" 
                      checked={!!node.etcd} 
                      onChange={(e) => {
                        const etcd = e.target.checked ? create(Postgres_EtcdServiceSchema, { monitor: false }) : undefined;
                        updateNodeData(idx, { etcd });
                      }} 
                    />
                  </div>
                </div>

                {/* Pgbouncer Module */}
                <div className={cn("p-4 border-2 transition-all", node.pgbouncer ? "bg-accent/5 border-accent/50" : "bg-background border-border opacity-50")}>
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Activity className="w-4 h-4" />
                      <span className="text-[11px] font-black uppercase">PgBouncer Pool</span>
                    </div>
                    <input 
                      type="checkbox" 
                      checked={!!node.pgbouncer} 
                      onChange={(e) => {
                        const pgbouncer = e.target.checked ? create(Postgres_PgbouncerServiceSchema, { 
                          config: create(Postgres_PgbouncerConfigSchema, { poolMode: Postgres_PgbouncerConfig_PoolMode.TRANSACTION, poolSize: 20 }),
                          monitor: false,
                        }) : undefined;
                        updateNodeData(idx, { pgbouncer });
                      }} 
                    />
                  </div>
                </div>
              </div>
            </div>

            <button 
              onClick={() => deleteNode(selectedNodeId!)}
              className="w-full py-4 border-2 border-destructive text-destructive text-[10px] font-black uppercase hover:bg-destructive hover:text-white transition-all flex items-center justify-center gap-2 mt-8"
            >
              <Trash2 className="w-4 h-4" /> Decommission Node
            </button>
          </div>
        </motion.div>
      </Panel>
    );
  };

  return (
    <div className="w-full h-full flex bg-background overflow-hidden relative">
      {/* Sidebar - Designer Tools */}
      <div className="w-72 border-r-2 border-border bg-card/20 backdrop-blur-xl z-20 flex flex-col">
        <div className="p-6 border-b border-border bg-background/50">
          <h2 className="text-xs font-black uppercase tracking-[0.4em] text-primary">Architect Tools</h2>
          <p className="text-[9px] text-muted-foreground uppercase mt-1">v4.8 Modular Designer</p>
        </div>
        
        <div className="p-6 space-y-6 overflow-y-auto">
          <div className="space-y-3">
            <h3 className="text-[9px] font-black text-muted-foreground uppercase tracking-widest">Base Blueprints</h3>
            <Blueprint 
              icon={Zap} 
              type="load" 
              title="Load Generator" 
              description="Stroppy workload generator for performance testing." 
            />
            <Blueprint 
              icon={Database} 
              type="postgres" 
              title="Database Node" 
              description="PostgreSQL instance with customizable role and resources." 
            />
            <Blueprint 
              icon={Shield} 
              type="etcd" 
              title="Consensus Node" 
              description="Distributed key-value store for cluster high availability." 
            />
            <Blueprint 
              icon={Activity} 
              type="pgbouncer" 
              title="Proxy Node" 
              description="Lightweight connection pooler for PostgreSQL." 
            />
          </div>

          <div className="pt-6 border-t border-border space-y-4">
            <h3 className="text-[9px] font-black text-muted-foreground uppercase tracking-widest">Global State</h3>
            <button 
              onClick={() => {
                if (confirm("Reset current topology?")) {
                  const ref = JSON.parse(JSON.stringify(databaseRef.value));
                  ref.template.value.nodes = [];
                  onChange({ databaseRef: { case: "databaseTemplate", value: ref }, stroppyCli: undefined, stroppyHardware: undefined });
                }
              }}
              className="w-full p-3 border-2 border-border hover:border-destructive hover:text-destructive transition-all text-[9px] font-black uppercase flex items-center justify-center gap-2"
            >
              <Trash2 className="w-3 h-3" /> Clear Blueprint
            </button>
          </div>
        </div>

        <div className="mt-auto p-6 bg-primary/5 border-t border-primary/20">
          <div className="flex items-center gap-2 text-[9px] font-black uppercase text-primary animate-pulse">
            <MousePointer2 className="w-3 h-3" /> Design Mode Active
          </div>
        </div>
      </div>

      {/* Main Designer Canvas */}
      <div className="flex-1 relative">
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          nodeTypes={nodeTypes}
          onNodeClick={(_, n) => setSelectedNodeId(n.id)}
          onPaneClick={() => setSelectedNodeId(null)}
          onDrop={onDrop}
          onDragOver={(e) => { e.preventDefault(); e.dataTransfer.dropEffect = 'move'; }}
          fitView
          minZoom={0.1}
          maxZoom={2}
          snapToGrid={true}
          snapGrid={[20, 20]}
        >
          <Background variant={BackgroundVariant.Lines} gap={20} color="#333" />
          
          {renderProperties()}
          
          <Panel position="bottom-right">
             <div className="p-2 bg-background/80 border border-border text-[8px] font-mono text-muted-foreground uppercase">
                Nodes: {nodes.length} | Edges: {edges.length}
             </div>
          </Panel>

          <Controls className="!bg-card !border-2 !border-primary !shadow-none [&_button]:!border-border" />
          <MiniMap 
            className="!bg-card !border-2 !border-primary !m-4" 
            nodeColor={(n) => {
              const nodeData = n.data as unknown as Postgres_Node;
              if (n.type === 'load') return 'var(--accent)';
              return nodeData?.postgres?.role === Postgres_PostgresService_Role.MASTER ? 'var(--primary)' : '#333';
            }}
            maskColor="rgba(0,0,0,0.5)"
          />
        </ReactFlow>
      </div>
    </div>
  );
};

export const TopologyCanvas = (props: TopologyCanvasProps) => {
  const isCluster = props.test.databaseRef.case === 'databaseTemplate' && props.test.databaseRef.value.template.case === 'postgresCluster';

  if (!isCluster) {
    return (
      <div className="w-full h-full flex flex-col items-center justify-center bg-background p-12 text-center space-y-6">
        <div className="p-8 border-2 border-dashed border-border group hover:border-primary transition-all cursor-pointer" onClick={() => {
           props.onChange({
            databaseRef: {
              case: "databaseTemplate", value: create(Postgres_ClusterSchema, {
                defaults: create(Postgres_SettingsSchema, { version: Postgres_Settings_Version.VERSION_17, storageEngine: Postgres_Settings_StorageEngine.HEAP }),
                nodes: []
              })
            }
          });
        }}>
          <Plus className="w-12 h-12 text-muted-foreground group-hover:text-primary transition-colors mx-auto mb-4" />
          <h3 className="text-xl font-black uppercase italic tracking-tighter">Initialize HA Cluster</h3>
          <p className="text-xs text-muted-foreground uppercase font-mono max-w-xs mx-auto">Click to switch from external connection to local high-availability cluster design.</p>
        </div>
      </div>
    );
  }

  return (
    <ReactFlowProvider>
      <TopologyDesigner {...props} />
    </ReactFlowProvider>
  );
};
