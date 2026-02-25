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
  type NodeTypes
} from '@xyflow/react';
import { 
  Database, 
  Shield, 
  Activity, 
  Cpu, 
  HardDrive, 
  Trash2, 
  Plus, 
  Settings2, 
  ChevronRight,
  Monitor,
  Box,
  X,
  Zap,
  Layers
} from 'lucide-react';
import { create } from '@bufbuild/protobuf';
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
import { HardwareInputs } from './ui/HardwareInputs';
import { Select } from './ui/Select';
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
    <div
      className={cn(
        "min-w-[200px] bg-[#2d2d2d] border transition-all duration-300 rounded shadow-xl",
        selected 
          ? "border-primary ring-2 ring-primary/20 shadow-primary/10" 
          : "border-[#454545]"
      )}
    >
      <div className="px-3 py-2 border-b border-[#454545] flex items-center justify-between bg-primary/5">
        <div className="flex items-center gap-2">
          <Zap className={cn("w-3.5 h-3.5 transition-colors", selected ? "text-primary" : "text-[#858585]")} />
          <span className="text-[10px] font-bold uppercase tracking-tight text-[#e1e1e1]">Load Generator</span>
        </div>
        <div className="px-1.5 py-0.5 bg-primary/10 border border-primary/20 text-primary text-[8px] font-black rounded-sm uppercase">
          {loadData.workload === StroppyCli_Workload.TPCC ? 'TPC-C' : 'TPC-B'}
        </div>
      </div>
      
      <div className="p-3 space-y-2">
        <div className="flex gap-3 text-[9px] font-mono text-[#858585] uppercase">
          <div className="flex items-center gap-1.5"><Cpu className="w-3 h-3 text-primary/40" />{loadData.hardware?.cores}C</div>
          <div className="flex items-center gap-1.5"><Monitor className="w-3 h-3 text-primary/40" />{loadData.hardware?.memory}G</div>
        </div>
      </div>
      <Handle type="source" position={Position.Right} className="!bg-primary !w-2 !h-2 !border-none !rounded-full shadow-[0_0_8px_rgba(var(--primary),0.5)]" />
    </div>
  );
});

const SmartNode = memo(({ data, selected }: NodeProps) => {
  const node = data as unknown as Postgres_Node;
  const isMaster = node.postgres?.role === Postgres_PostgresService_Role.MASTER;
  
  return (
    <div
      className={cn(
        "min-w-[220px] bg-[#252526] border transition-all duration-300 rounded shadow-2xl",
        selected 
          ? "border-primary ring-2 ring-primary/20 shadow-primary/10 scale-[1.02]" 
          : "border-[#454545] hover:border-[#606060]"
      )}
    >
      {/* Header */}
      <div className={cn(
        "px-3 py-2 border-b flex items-center justify-between transition-colors",
        isMaster ? "bg-primary/[0.07] border-primary/20" : "bg-[#2d2d2d] border-[#454545]"
      )}>
        <div className="flex items-center gap-2">
          <Box className={cn("w-3.5 h-3.5 transition-colors", isMaster ? "text-primary" : "text-[#858585]")} />
          <span className="text-[10px] font-bold uppercase tracking-tight text-[#cccccc] truncate max-w-[120px]">
            {node.name || "node_unnamed"}
          </span>
        </div>
        {isMaster && (
          <div className="flex items-center gap-1">
            <div className="w-1 h-1 rounded-full bg-primary animate-pulse" />
            <span className="text-primary text-[8px] font-black uppercase tracking-widest">Leader</span>
          </div>
        )}
      </div>

      {/* Services Grid */}
      <div className="p-3 space-y-3">
        <div className="flex gap-3 text-[9px] font-mono text-[#858585] uppercase border-b border-[#333] pb-2">
          <div className="flex items-center gap-1"><Cpu className="w-3 h-3 text-primary/30" />{node.hardware?.cores}C</div>
          <div className="flex items-center gap-1"><Monitor className="w-3 h-3 text-primary/30" />{node.hardware?.memory}G</div>
          <div className="flex items-center gap-1"><HardDrive className="w-3 h-3 text-primary/30" />{node.hardware?.disk}G</div>
        </div>

        <div className="space-y-2">
          {node.postgres && (
            <div className="flex items-center justify-between p-1.5 rounded-sm bg-primary/5 border border-primary/10">
              <div className="flex items-center gap-2 text-[10px] font-bold text-[#e1e1e1] uppercase tracking-tighter">
                <Database className="w-3 h-3 text-primary" /> Postgres
              </div>
              <span className="text-[8px] font-bold text-primary/60 uppercase">{node.postgres.role === Postgres_PostgresService_Role.MASTER ? 'M' : 'R'}</span>
            </div>
          )}
          {node.etcd && (
            <div className="flex items-center gap-2 text-[10px] font-bold text-[#e1e1e1] uppercase tracking-tighter p-1.5 rounded-sm bg-[#007acc]/5 border border-[#007acc]/10">
              <Shield className="w-3 h-3 text-[#007acc]" /> Etcd Cluster
            </div>
          )}
          {node.pgbouncer && (
            <div className="flex items-center gap-2 text-[10px] font-bold text-[#e1e1e1] uppercase tracking-tighter p-1.5 rounded-sm bg-[#4ec9b0]/5 border border-[#4ec9b0]/10">
              <Activity className="w-3 h-3 text-[#4ec9b0]" /> Connection Pool
            </div>
          )}
        </div>
      </div>

      <Handle type="target" position={Position.Left} className="!bg-[#555] !w-1.5 !h-1.5 !border-none !rounded-none" />
      <Handle type="source" position={Position.Right} className="!bg-[#555] !w-1.5 !h-1.5 !border-none !rounded-none" />
    </div>
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
      className="p-3 bg-[#252526] border border-[#333333] hover:border-primary/40 hover:bg-[#2d2d2d] hover:shadow-lg transition-all cursor-grab active:cursor-grabbing group rounded shadow-sm"
    >
      <div className="flex items-center gap-2.5 mb-1.5">
        <div className="p-1.5 bg-[#1e1e1e] rounded group-hover:bg-primary/10 transition-colors">
          <Icon className="w-4 h-4 text-[#858585] group-hover:text-primary transition-colors" />
        </div>
        <div className="text-[11px] font-bold uppercase tracking-tight text-[#cccccc] group-hover:text-[#e1e1e1]">{title}</div>
      </div>
      <p className="text-[9px] text-[#606060] leading-tight font-medium uppercase tracking-tighter">{description}</p>
    </div>
  );
};

// --- Main Designer Component ---

const TopologyDesigner = ({ test, onChange }: TopologyCanvasProps) => {
  const { screenToFlowPosition, fitView } = useReactFlow();
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const databaseRef = test.databaseRef;

  const elements = useMemo(() => {
    const nodes: Node[] = [];
    const edges: Edge[] = [];

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
      const masterNodeIdx = cluster.nodes.findIndex((n: any) => n.postgres?.role === Postgres_PostgresService_Role.MASTER);
      const masterId = masterNodeIdx !== -1 ? `node-${masterNodeIdx}` : null;
      const firstPgbouncerIdx = cluster.nodes.findIndex((n: any) => n.pgbouncer);
      const entryId = firstPgbouncerIdx !== -1 ? `node-${firstPgbouncerIdx}` : masterId;

      if (test.stroppyCli && entryId) {
        edges.push({
          id: 'edge-load-to-entry',
          source: 'stroppy-node',
          target: entryId,
          animated: true,
          style: { stroke: 'rgba(var(--primary), 0.4)', strokeWidth: 2 }
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
            style: { stroke: '#454545', strokeWidth: 1.5, opacity: 0.6 }
          });
        }
      });
    }

    return { nodes, edges };
  }, [databaseRef, test]);

  const [nodes, setNodes, onNodesChange] = useNodesState(elements.nodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(elements.edges);

  const deleteNode = useCallback((nodeId: string) => {
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
  }, [databaseRef, onChange]);

  // 1. Hotkeys support
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
        return;
      }

      if (e.key === 'f') {
        e.preventDefault();
        fitView({ duration: 400 });
      }

      if (e.key === 'C' && e.shiftKey) {
        e.preventDefault();
        if (confirm("Reset current topology blueprint?")) {
          const ref = JSON.parse(JSON.stringify(databaseRef.value));
          ref.template.value.nodes = [];
          onChange({ databaseRef: { case: "databaseTemplate", value: ref }, stroppyCli: undefined, stroppyHardware: undefined });
        }
      }

      if (e.key === 'Escape') {
        setSelectedNodeId(null);
        setNodes(nds => nds.map(n => ({ ...n, selected: false })));
      }

      if (e.key === 'Delete' || e.key === 'Backspace') {
        const selectedNodes = nodes.filter(n => n.selected);
        if (selectedNodes.length > 0) {
          e.preventDefault();
          selectedNodes.forEach(node => deleteNode(node.id));
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [fitView, databaseRef, onChange, setNodes, nodes, deleteNode]);

  useEffect(() => {
    setNodes((nds) => {
      return elements.nodes.map((newNo: Node) => {
        const existingNode = nds.find((n) => n.id === newNo.id);
        return existingNode ? { ...newNo, position: existingNode.position } : newNo;
      });
    });
    setEdges(elements.edges);
  }, [elements, setNodes, setEdges]);

  const updateNodeData = (nodeIdx: number, updates: any) => {
    const ref = JSON.parse(JSON.stringify(databaseRef.value));
    ref.template.value.nodes[nodeIdx] = { ...ref.template.value.nodes[nodeIdx], ...updates };
    onChange({ databaseRef: { case: "databaseTemplate", value: ref } });
  };

  const onNodesDelete = useCallback((deleted: Node[]) => {
    deleted.forEach(node => {
      deleteNode(node.id);
    });
  }, [deleteNode]);

  const onDrop = useCallback(
    (event: React.DragEvent) => {
      event.preventDefault();
      const type = event.dataTransfer.getData('application/reactflow');
      if (!type) return;

      const position = screenToFlowPosition({ x: event.clientX, y: event.clientY });
      
      if (type === 'load') {
        onChange({
          stroppyCli: create(StroppyCliSchema, { version: "v1.0.0", workload: StroppyCli_Workload.TPCC }),
          stroppyHardware: create(HardwareSchema, { cores: 2, memory: 4, disk: 20 })
        });
        return;
      }

      if (databaseRef.case === 'databaseTemplate' && databaseRef.value.template.case === 'postgresCluster') {
        const ref = JSON.parse(JSON.stringify(databaseRef.value));
        const nodeCount = ref.template.value.nodes.length;

        let newNode = create(Postgres_NodeSchema, {
          hardware: create(HardwareSchema, { cores: 2, memory: 4, disk: 50 })
        });

        if (type === 'postgres') {
          newNode.name = `pg_node_${nodeCount + 1}`;
          newNode.postgres = create(Postgres_PostgresServiceSchema, { role: Postgres_PostgresService_Role.REPLICA });
        } else if (type === 'etcd') {
          newNode.name = `etcd_node_${nodeCount + 1}`;
          newNode.etcd = create(Postgres_EtcdServiceSchema, { monitor: false });
        } else if (type === 'pgbouncer') {
          newNode.name = `pgb_node_${nodeCount + 1}`;
          newNode.pgbouncer = create(Postgres_PgbouncerServiceSchema, { 
            config: create(Postgres_PgbouncerConfigSchema, { poolMode: Postgres_PgbouncerConfig_PoolMode.TRANSACTION, poolSize: 20 }),
            monitor: false,
          });
        }

        ref.template.value.nodes.push(newNode);
        onChange({ databaseRef: { case: "databaseTemplate", value: ref } });
        
        setNodes((nds) => nds.concat({
          id: `node-${nodeCount}`,
          type: 'smart',
          position,
          data: newNode as any
        }));
      }
    },
    [screenToFlowPosition, databaseRef, onChange, setNodes]
  );

  const renderProperties = () => {
    if (!selectedNodeId) return null;

    let title = "Node Configuration";
    let icon = <Settings2 className="w-4 h-4 text-primary" />;
    let content = null;

    if (selectedNodeId === 'stroppy-node' && test.stroppyCli) {
      title = "Load Generator";
      icon = <Zap className="w-4 h-4 text-primary" />;
      content = (
        <div className="space-y-6">
          <Input label="Engine Version" value={test.stroppyCli.version} onChange={(e) => onChange({ stroppyCli: { ...test.stroppyCli, version: e.target.value } })} />
          <Select label="Workload Model" value={test.stroppyCli.workload} onChange={(v) => onChange({ stroppyCli: { ...test.stroppyCli, workload: v } })} options={[{ label: "TPC-C", value: StroppyCli_Workload.TPCC }, { label: "TPC-B", value: StroppyCli_Workload.TPCB }]} />
          <HardwareInputs label="Compute Resources" hardware={test.stroppyHardware} onChange={(h) => onChange({ stroppyHardware: h })} />
          <button onClick={() => deleteNode('stroppy-node')} className="w-full h-10 bg-destructive/5 hover:bg-destructive hover:text-white text-destructive text-[10px] font-bold uppercase border border-destructive/20 transition-all rounded flex items-center justify-center gap-2 mt-6 shadow-sm"><Trash2 className="w-4 h-4" /> Decommission Generator</button>
        </div>
      );
    } else {
      const idx = parseInt(selectedNodeId.replace('node-', ''));
      const node = databaseRef.value.template.value.nodes[idx];
      if (!node) return null;
      content = (
        <div className="space-y-8">
          <div className="space-y-4">
            <Input label="Identity ID" value={node.name} onChange={(e) => updateNodeData(idx, { name: e.target.value })} />
            <HardwareInputs label="Node Resources" hardware={node.hardware} onChange={(h) => updateNodeData(idx, { hardware: h })} />
          </div>

          <div className="space-y-4 pt-6 border-t border-[#333]">
            <h4 className="text-[10px] font-black uppercase tracking-widest text-[#858585] flex items-center gap-2">
              <ChevronRight className="w-3 h-3 text-primary" /> Module Matrix
            </h4>

            <div className="space-y-3">
              {/* Postgres Module */}
              <div className={cn("p-4 border rounded shadow-sm transition-all", node.postgres ? "bg-primary/5 border-primary/30 shadow-primary/5" : "bg-[#1e1e1e] border-[#333] opacity-60")}>
                <div className="flex items-center justify-between mb-3">
                  <div className="flex items-center gap-2.5">
                    <Database className={cn("w-4 h-4", node.postgres ? "text-primary" : "text-[#454545]")} />
                    <span className="text-[11px] font-bold uppercase text-[#e1e1e1]">PostgreSQL</span>
                  </div>
                  <input type="checkbox" checked={!!node.postgres} onChange={(e) => updateNodeData(idx, { postgres: e.target.checked ? create(Postgres_PostgresServiceSchema, { role: Postgres_PostgresService_Role.REPLICA }) : undefined })} className="accent-primary" />
                </div>
                {node.postgres && (
                  <Select label="Role" value={node.postgres.role} onChange={(v) => {
                    const p = JSON.parse(JSON.stringify(node.postgres)); p.role = v; updateNodeData(idx, { postgres: p });
                  }} options={[{ label: "Master / Leader", value: Postgres_PostgresService_Role.MASTER }, { label: "Active Replica", value: Postgres_PostgresService_Role.REPLICA }]} />
                )}
              </div>

              {/* Etcd Module */}
              <div className={cn("p-4 border rounded shadow-sm transition-all", node.etcd ? "bg-[#007acc]/10 border-[#007acc]/30" : "bg-[#1e1e1e] border-[#333] opacity-60")}>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2.5">
                    <Shield className={cn("w-4 h-4", node.etcd ? "text-[#007acc]" : "text-[#454545]")} />
                    <span className="text-[11px] font-bold uppercase text-[#e1e1e1]">Etcd Consensus</span>
                  </div>
                  <input type="checkbox" checked={!!node.etcd} onChange={(e) => updateNodeData(idx, { etcd: e.target.checked ? create(Postgres_EtcdServiceSchema, { monitor: false }) : undefined })} className="accent-[#007acc]" />
                </div>
              </div>

              {/* Pgbouncer Module */}
              <div className={cn("p-4 border rounded shadow-sm transition-all", node.pgbouncer ? "bg-[#4ec9b0]/10 border-[#4ec9b0]/30" : "bg-[#1e1e1e] border-[#333] opacity-60")}>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2.5">
                    <Activity className={cn("w-4 h-4", node.pgbouncer ? "text-[#4ec9b0]" : "text-[#454545]")} />
                    <span className="text-[11px] font-bold uppercase text-[#e1e1e1]">PgBouncer Pool</span>
                  </div>
                  <input type="checkbox" checked={!!node.pgbouncer} onChange={(e) => updateNodeData(idx, { pgbouncer: e.target.checked ? create(Postgres_PgbouncerServiceSchema, { config: create(Postgres_PgbouncerConfigSchema, { poolMode: Postgres_PgbouncerConfig_PoolMode.TRANSACTION, poolSize: 20 }), monitor: false }) : undefined })} className="accent-[#4ec9b0]" />
                </div>
              </div>
            </div>
          </div>

          <button onClick={() => deleteNode(selectedNodeId)} className="w-full h-10 bg-destructive/5 hover:bg-destructive hover:text-white text-destructive text-[10px] font-bold uppercase border border-destructive/20 transition-all rounded flex items-center justify-center gap-2 mt-6 shadow-sm"><Trash2 className="w-4 h-4" /> Decommission Node</button>
        </div>
      );
    }

    return (
      <Panel position="top-right" className="m-0 h-full">
        <div className="w-80 h-full bg-[#252526] border-l border-[#333] shadow-[-10px_0_30px_rgba(0,0,0,0.3)] flex flex-col">
          <div className="h-10 px-4 border-b border-[#333] bg-[#2d2d2d] flex items-center justify-between shadow-sm">
            <div className="flex items-center gap-2.5">
              {icon}
              <h3 className="text-[10px] font-black uppercase tracking-widest text-[#cccccc]">{title}</h3>
            </div>
            <button onClick={() => setSelectedNodeId(null)} className="p-1 hover:bg-[#37373d] rounded transition-colors text-[#858585] hover:text-[#cccccc]"><X className="w-4 h-4" /></button>
          </div>
          <div className="flex-1 p-6 overflow-y-auto custom-scrollbar bg-[#1e1e1e]/30 shadow-inner">
            {content}
          </div>
        </div>
      </Panel>
    );
  };

  return (
    <div className="w-full h-full flex bg-[#1e1e1e] overflow-hidden relative shadow-inner">
      {/* Sidebar - Designer Tools */}
      <div className="w-60 border-r border-[#2b2b2b] bg-[#181818] z-20 flex flex-col shrink-0 shadow-2xl">
        <div className="h-9 px-4 border-b border-[#2b2b2b] flex items-center bg-[#1e1e1e]/50">
          <span className="text-[10px] font-bold uppercase tracking-[0.2em] text-[#858585] flex items-center gap-2">
            <Layers className="w-3.5 h-3.5 text-primary/60" /> Blueprints
          </span>
        </div>
        
        <div className="p-3 space-y-3 overflow-y-auto custom-scrollbar flex-1 shadow-inner">
          <Blueprint icon={Zap} type="load" title="Load Generator" description="Workload generation engine." />
          <Blueprint icon={Database} type="postgres" title="Database Node" description="PostgreSQL instance module." />
          <Blueprint icon={Shield} type="etcd" title="Consensus Node" description="HA consensus store module." />
          <Blueprint icon={Activity} type="pgbouncer" title="Proxy Node" description="Connection pooler module." />

          <div className="pt-4 border-t border-[#333] space-y-2">
            <button 
              onClick={() => { if (confirm("Reset current topology blueprint?")) {
                const ref = JSON.parse(JSON.stringify(databaseRef.value)); ref.template.value.nodes = [];
                onChange({ databaseRef: { case: "databaseTemplate", value: ref }, stroppyCli: undefined, stroppyHardware: undefined });
              }}}
              className="w-full h-8 border border-[#454545] hover:bg-destructive/10 hover:border-destructive/30 hover:text-destructive text-[#858585] text-[9px] font-bold uppercase transition-all flex items-center justify-center gap-2 rounded shadow-sm"
            >
              <Trash2 className="w-3 h-3" /> Reset Canvas
            </button>
          </div>
        </div>

        <div className="p-3 bg-[#1e1e1e] border-t border-[#2b2b2b] shadow-2xl">
          <div className="flex items-center gap-2.5 text-[9px] font-bold uppercase text-primary/60 tracking-wider">
            <div className="w-1.5 h-1.5 rounded-full bg-primary/40 animate-pulse shadow-[0_0_5px_rgba(var(--primary),0.3)]" />
            Designer Engine Active
          </div>
        </div>
      </div>

      {/* Main Designer Canvas */}
      <div className="flex-1 relative bg-[#1e1e1e] shadow-inner">
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onNodesDelete={onNodesDelete}
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
          <Background variant={BackgroundVariant.Lines} gap={20} color="#252526" />
          
          {renderProperties()}
          
          <Panel position="bottom-left" className="m-4">
             <div className="px-3 py-1.5 bg-[#252526]/90 border border-[#454545] rounded shadow-xl text-[9px] font-mono text-[#858585] uppercase flex items-center gap-4 backdrop-blur-sm">
                <div className="flex items-center gap-1.5">
                  <span className="w-1.5 h-1.5 rounded-full bg-[#858585]/30" />
                  <span>Nodes: <span className="text-[#cccccc]">{nodes.length}</span></span>
                </div>
                <div className="flex items-center gap-1.5">
                  <span className="w-1.5 h-1.5 rounded-full bg-[#858585]/30" />
                  <span>Edges: <span className="text-[#cccccc]">{edges.length}</span></span>
                </div>
                <div className="flex items-center gap-4 ml-2 border-l border-[#454545] pl-4">
                  <span className="opacity-40">[F] FIT</span>
                  <span className="opacity-40">[ESC] DESELECT</span>
                  <span className="opacity-40">[DEL] DELETE</span>
                  <span className="opacity-40">[SHIFT+C] CLEAR</span>
                </div>
             </div>
          </Panel>
        </ReactFlow>
      </div>
    </div>
  );
};

export const TopologyCanvas = (props: TopologyCanvasProps) => {
  const isCluster = props.test.databaseRef.case === 'databaseTemplate' && props.test.databaseRef.value.template.case === 'postgresCluster';

  if (!isCluster) {
    return (
      <div className="w-full h-full flex flex-col items-center justify-center bg-[#1e1e1e] p-12 text-center space-y-8 shadow-inner">
        <div 
          className="p-12 border border-[#333] bg-[#252526] hover:border-primary/30 hover:shadow-[0_0_40px_rgba(var(--primary),0.05)] transition-all cursor-pointer rounded shadow-2xl group relative overflow-hidden" 
          onClick={() => {
            props.onChange({
              databaseRef: {
                case: "databaseTemplate", value: create(Postgres_ClusterSchema, {
                  defaults: create(Postgres_SettingsSchema, { version: Postgres_Settings_Version.VERSION_17, storageEngine: Postgres_Settings_StorageEngine.HEAP }),
                  nodes: []
                })
              }
            });
          }}
        >
          <div className="absolute inset-0 bg-primary/[0.02] group-hover:bg-primary/[0.04] transition-colors" />
          <div className="relative z-10">
            <Plus className="w-12 h-12 text-[#454545] group-hover:text-primary transition-all group-hover:scale-110 mx-auto mb-6" />
            <h3 className="text-xl font-black uppercase tracking-widest text-[#e1e1e1]">Initialize Scenario</h3>
            <p className="text-[10px] text-[#858585] uppercase font-bold tracking-widest max-w-[280px] mx-auto mt-3 opacity-60">Architect high-availability database cluster resources for performance profiling.</p>
          </div>
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
