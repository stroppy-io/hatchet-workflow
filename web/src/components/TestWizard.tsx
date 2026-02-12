import { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { create } from '@bufbuild/protobuf'
import {
    TestSchema, StroppyCli_Workload, StroppyCliSchema
} from '../proto/stroppy/test_pb'
import type { Test } from '../proto/stroppy/test_pb'
import { HardwareSchema } from '../proto/deployment/deployment_pb'
import {
    Database_TemplateSchema,
} from '../proto/database/database_pb'
import {
    Postgres_Instance_TemplateSchema,
    Postgres_Cluster_TemplateSchema,
    Postgres_Cluster_Template_TopologySchema,
    Postgres_SettingsSchema,
    Postgres_Settings_Version,
    Postgres_Settings_StorageEngine,
    Postgres_AddonsSchema,
    Postgres_Addons_DcsSchema,
    Postgres_Addons_Dcs_EtcdSchema,
    Postgres_PlacementSchema,
    Postgres_Placement_ColocateSchema,
    Postgres_Placement_Scope,
    Postgres_Placement_DedicatedSchema,
    Postgres_Addons_PoolingSchema,
    Postgres_Addons_Pooling_PgbouncerSchema,
    Postgres_Addons_Pooling_Pgbouncer_PoolMode
} from '../proto/database/postgres_pb'

import { Stepper } from './ui/Stepper'
import { Input } from './ui/Input'
import { Select } from './ui/Select'
import { HardwareInputs } from './ui/HardwareInputs'
import { BlueprintCard } from './ui/BlueprintCard'
import { TopologyCanvas } from './TopologyCanvas'
import { ShieldCheck, Zap, Layers, Shield, Activity, Database, Server, Check } from 'lucide-react'
import { cn } from '../App'

interface TestWizardProps {
    test: Test
    onChange: (updates: Partial<Test>) => void
}

const STEPS = ["Context", "Client", "Database", "Verify"]

export const TestWizard = ({ test, onChange }: TestWizardProps) => {
    const [currentStep, setCurrentStep] = useState(0)

    const updateTest = (updates: any) => {
        onChange(updates)
    }

    return (
        <div className="flex flex-col h-full">
            <div className="p-8 border-b border-border bg-card/10">
                <Stepper steps={STEPS} currentStep={currentStep} onStepClick={setCurrentStep} />
            </div>

            <div className="flex-1 overflow-hidden relative">
                <AnimatePresence mode='wait'>
                    {currentStep === 0 && (
                        <motion.div key="step0" initial={{ opacity: 0, x: 20 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: -20 }} className="p-12 max-w-3xl mx-auto space-y-8">
                            <div className="flex items-center gap-3 mb-6 border-b border-primary/20 pb-4">
                                <ShieldCheck className="w-6 h-6 text-primary" />
                                <div>
                                    <h2 className="text-2xl font-black uppercase italic tracking-tighter">Test Context</h2>
                                    <p className="text-[10px] font-mono text-muted-foreground uppercase tracking-widest">Define the identity and scope of the test scenario.</p>
                                </div>
                            </div>

                            <div className="grid grid-cols-1 gap-6">
                                <Input label="Name" fieldName="name" value={test.name} onChange={(e) => updateTest({ name: e.target.value })} className="text-lg py-6" autoFocus />
                                <div className="space-y-1">
                                    <div className="flex justify-between items-baseline px-1">
                                        <label className="text-[9px] font-black uppercase tracking-widest text-muted-foreground">Description</label>
                                        <span className="text-[9px] font-mono text-primary/50">description</span>
                                    </div>
                                    <textarea
                                        value={test.description || ""}
                                        onChange={(e) => updateTest({ description: e.target.value })}
                                        className="w-full bg-background border border-input p-4 text-xs font-mono h-32 outline-none focus:border-primary transition-all resize-none"
                                        placeholder="Describe the test scenario, objectives, and expected outcomes..."
                                    />
                                </div>
                            </div>
                        </motion.div>
                    )}

                    {currentStep === 1 && (
                        <motion.div key="step1" initial={{ opacity: 0, x: 20 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: -20 }} className="p-12 max-w-4xl mx-auto space-y-8">
                            <div className="flex items-center gap-3 mb-6 border-b border-primary/20 pb-4">
                                <Zap className="w-6 h-6 text-primary" />
                                <div>
                                    <h2 className="text-2xl font-black uppercase italic tracking-tighter">Stroppy Client</h2>
                                    <p className="text-[10px] font-mono text-muted-foreground uppercase tracking-widest">Configure the workload generator and its resources.</p>
                                </div>
                            </div>

                            <div className="grid grid-cols-2 gap-12">
                                <div className="space-y-6">
                                    <h3 className="text-sm font-black uppercase tracking-widest border-l-4 border-primary pl-4">Workload Config</h3>
                                    <div className="space-y-4">
                                        <Input label="Version" fieldName="active_test.stroppy_cli.version" value={test.stroppyCli?.version || ""} onChange={(e) => updateTest({ stroppyCli: create(StroppyCliSchema, { ...test.stroppyCli!, version: e.target.value }) })} />
                                        <Select label="Workload" fieldName="active_test.stroppy_cli.workload" value={test.stroppyCli?.workload} onChange={(v) => updateTest({ stroppyCli: create(StroppyCliSchema, { ...test.stroppyCli!, workload: v }) })} options={[{ label: "TPC-C", value: StroppyCli_Workload.TPCC }, { label: "TPC-B", value: StroppyCli_Workload.TPCB }]} />
                                    </div>
                                </div>
                                <div className="space-y-6">
                                    <h3 className="text-sm font-black uppercase tracking-widest border-l-4 border-accent pl-4">Resource Allocation</h3>
                                    <HardwareInputs label="Client Resources" fieldName="stroppy_hardware" hardware={test.stroppyHardware} onChange={(h) => updateTest({ stroppyHardware: h })} />
                                </div>
                            </div>
                        </motion.div>
                    )}

                    {currentStep === 2 && (
                        <motion.div key="step2" initial={{ opacity: 0, x: 20 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: -20 }} className="flex h-full">
                            {/* Left Config Panel */}
                            <div className="w-1/3 border-r border-border p-8 overflow-y-auto space-y-8 bg-card/10">
                                <div className="flex items-center gap-3 mb-6 border-b border-primary/20 pb-4">
                                    <Database className="w-6 h-6 text-primary" />
                                    <div>
                                        <h2 className="text-xl font-black uppercase italic tracking-tighter">Database</h2>
                                        <p className="text-[9px] font-mono text-muted-foreground uppercase tracking-widest">Configuration & Topology</p>
                                    </div>
                                </div>

                                <div className="flex gap-2 mb-6">
                                    <button onClick={() => updateTest({ databaseRef: { case: "connectionString", value: "" } })} className={cn("flex-1 py-2 text-[10px] font-black uppercase border-2 transition-all", test.databaseRef.case === 'connectionString' ? "bg-primary border-primary text-white" : "border-border text-muted-foreground hover:border-primary")}>External</button>
                                    <button onClick={() => updateTest({ databaseRef: { case: "databaseTemplate", value: create(Database_TemplateSchema, { template: { case: "postgresInstance", value: create(Postgres_Instance_TemplateSchema, { settings: create(Postgres_SettingsSchema, { version: Postgres_Settings_Version.VERSION_17, storageEngine: Postgres_Settings_StorageEngine.HEAP }), hardware: create(HardwareSchema, { cores: 4, memory: 8, disk: 100 }) }) } }) } })} className={cn("flex-1 py-2 text-[10px] font-black uppercase border-2 transition-all", test.databaseRef.case === 'databaseTemplate' ? "bg-primary border-primary text-white" : "border-border text-muted-foreground hover:border-primary")}>Forge Managed</button>
                                </div>

                                {test.databaseRef.case === 'databaseTemplate' && (
                                    <div className="space-y-8">
                                        <div className="space-y-4">
                                            <Select label="Template Type" fieldName="template.case" value={test.databaseRef.value.template.case} onChange={(v) => {
                                                if (v === 'postgresInstance') updateTest({ databaseRef: { case: "databaseTemplate", value: create(Database_TemplateSchema, { template: { case: "postgresInstance", value: create(Postgres_Instance_TemplateSchema, { settings: create(Postgres_SettingsSchema, { version: Postgres_Settings_Version.VERSION_17, storageEngine: Postgres_Settings_StorageEngine.HEAP }), hardware: create(HardwareSchema, { cores: 4, memory: 8, disk: 100 }) }) } }) } });
                                                else updateTest({ databaseRef: { case: "databaseTemplate", value: create(Database_TemplateSchema, { template: { case: "postgresCluster", value: create(Postgres_Cluster_TemplateSchema, { topology: create(Postgres_Cluster_Template_TopologySchema, { replicasCount: 2, monitor: true, settings: create(Postgres_SettingsSchema, { version: Postgres_Settings_Version.VERSION_17, storageEngine: Postgres_Settings_StorageEngine.HEAP }), masterHardware: create(HardwareSchema, { cores: 4, memory: 8, disk: 100 }), replicaHardware: create(HardwareSchema, { cores: 2, memory: 4, disk: 50 }) }), addons: create(Postgres_AddonsSchema, {}) }) } }) } });
                                            }} options={[{ label: "Single Instance", value: "postgresInstance" }, { label: "HA Cluster (Patroni)", value: "postgresCluster" }]} />

                                            <div className="grid grid-cols-2 gap-2">
                                                <Select label="Version" fieldName="settings.version" value={test.databaseRef.value.template.case === 'postgresInstance' ? (test.databaseRef.value.template.value as any).settings?.version : (test.databaseRef.value.template.value as any).topology?.settings?.version} onChange={(v) => {
                                                    const ref = JSON.parse(JSON.stringify(test.databaseRef.value));
                                                    if (ref.template.case === 'postgresInstance') ref.template.value.settings.version = v;
                                                    else ref.template.value.topology.settings.version = v;
                                                    updateTest({ databaseRef: { case: "databaseTemplate", value: ref } });
                                                }} options={[{ label: "v17", value: Postgres_Settings_Version.VERSION_17 }, { label: "v16", value: Postgres_Settings_Version.VERSION_16 }]} />

                                                <Select label="Engine" fieldName="settings.storage_engine" value={test.databaseRef.value.template.case === 'postgresInstance' ? (test.databaseRef.value.template.value as any).settings?.storageEngine : (test.databaseRef.value.template.value as any).topology?.settings?.storageEngine} onChange={(v) => {
                                                    const ref = JSON.parse(JSON.stringify(test.databaseRef.value));
                                                    if (ref.template.case === 'postgresInstance') ref.template.value.settings.storageEngine = v;
                                                    else ref.template.value.topology.settings.storageEngine = v;
                                                    updateTest({ databaseRef: { case: "databaseTemplate", value: ref } });
                                                }} options={[{ label: "Heap", value: Postgres_Settings_StorageEngine.HEAP }, { label: "OrioleDB", value: Postgres_Settings_StorageEngine.ORIOLEDB }]} />
                                            </div>
                                            <HardwareInputs label={test.databaseRef.value.template.case === 'postgresInstance' ? "Instance Hardware" : "Leader Hardware"} fieldName="hardware" hardware={test.databaseRef.value.template.case === 'postgresInstance' ? (test.databaseRef.value.template.value as any).hardware : (test.databaseRef.value.template.value as any).topology.masterHardware} onChange={(h) => {
                                                const ref = JSON.parse(JSON.stringify(test.databaseRef.value));
                                                if (ref.template.case === 'postgresInstance') ref.template.value.hardware = h;
                                                else ref.template.value.topology.masterHardware = h;
                                                updateTest({ databaseRef: { case: "databaseTemplate", value: ref } });
                                            }} />
                                        </div>

                                        {test.databaseRef.value.template.case === 'postgresCluster' && (
                                            <div className="space-y-4 pt-4 border-t border-border">
                                                <div className="flex items-center gap-2 mb-2"><Layers className="w-4 h-4 text-accent" /><h4 className="text-[10px] font-black uppercase tracking-widest text-foreground font-mono">cluster.addons</h4></div>
                                                <BlueprintCard
                                                    title="DCS (ETCD)" fieldName="addons.dcs" subtitle="Distributed Config Store" icon={Shield}
                                                    active={!!test.databaseRef.value.template.value.addons?.dcs?.etcd}
                                                    onClick={() => {
                                                        const ref = JSON.parse(JSON.stringify(test.databaseRef.value));
                                                        if (!ref.template.value.addons.dcs) ref.template.value.addons.dcs = create(Postgres_Addons_DcsSchema, { etcd: create(Postgres_Addons_Dcs_EtcdSchema, { size: 3, monitor: true, placement: create(Postgres_PlacementSchema, { mode: { case: "colocate", value: create(Postgres_Placement_ColocateSchema, { scope: Postgres_Placement_Scope.ALL_NODES }) } }) }) });
                                                        else delete ref.template.value.addons.dcs;
                                                        updateTest({ databaseRef: { case: "databaseTemplate", value: ref } });
                                                    }}
                                                >
                                                    <div className="grid grid-cols-2 gap-2">
                                                        <Input label="Size" type="number" className="h-6 text-[10px]" value={test.databaseRef.value.template.value.addons?.dcs?.etcd?.size || 0} onChange={(e) => {
                                                            const ref = JSON.parse(JSON.stringify(test.databaseRef.value)); ref.template.value.addons.dcs.etcd.size = parseInt(e.target.value); updateTest({ databaseRef: { case: "databaseTemplate", value: ref } });
                                                        }} />
                                                        <Select label="Placement" className="h-6 text-[10px]" value={test.databaseRef.value.template.value.addons?.dcs?.etcd?.placement?.mode?.case} onChange={(v) => {
                                                            const ref = JSON.parse(JSON.stringify(test.databaseRef.value));
                                                            if (v === 'dedicated') ref.template.value.addons.dcs.etcd.placement = create(Postgres_PlacementSchema, { mode: { case: "dedicated", value: create(Postgres_Placement_DedicatedSchema, { instancesCount: ref.template.value.addons.dcs.etcd.size, hardware: create(HardwareSchema, { cores: 1, memory: 2, disk: 10 }) }) } });
                                                            else ref.template.value.addons.dcs.etcd.placement = create(Postgres_PlacementSchema, { mode: { case: "colocate", value: create(Postgres_Placement_ColocateSchema, { scope: Postgres_Placement_Scope.ALL_NODES }) } });
                                                            updateTest({ databaseRef: { case: "databaseTemplate", value: ref } });
                                                        }} options={[{ label: "Colocated", value: "colocate" }, { label: "Dedicated", value: "dedicated" }]} />
                                                    </div>
                                                </BlueprintCard>

                                                <BlueprintCard
                                                    title="Pooling" fieldName="addons.pooling" subtitle="PgBouncer" icon={Activity}
                                                    active={!!test.databaseRef.value.template.value.addons?.pooling?.pgbouncer}
                                                    onClick={() => {
                                                        const ref = JSON.parse(JSON.stringify(test.databaseRef.value));
                                                        if (!ref.template.value.addons.pooling) ref.template.value.addons.pooling = create(Postgres_Addons_PoolingSchema, { pgbouncer: create(Postgres_Addons_Pooling_PgbouncerSchema, { enabled: true, poolSize: 20, poolMode: Postgres_Addons_Pooling_Pgbouncer_PoolMode.TRANSACTION, monitor: true, placement: create(Postgres_PlacementSchema, { mode: { case: "colocate", value: create(Postgres_Placement_ColocateSchema, { scope: Postgres_Placement_Scope.MASTER }) } }) }) });
                                                        else delete ref.template.value.addons.pooling;
                                                        updateTest({ databaseRef: { case: "databaseTemplate", value: ref } });
                                                    }}
                                                >
                                                    <div className="grid grid-cols-2 gap-2">
                                                        <Select label="Mode" className="h-6 text-[10px]" value={Postgres_Addons_Pooling_Pgbouncer_PoolMode.TRANSACTION} onChange={() => { }} options={[{ label: "Transaction", value: 1 }, { label: "Session", value: 2 }]} />
                                                        <Input label="Size" className="h-6 text-[10px]" type="number" value={20} onChange={() => { }} />
                                                    </div>
                                                </BlueprintCard>
                                            </div>
                                        )}
                                    </div>
                                )}
                            </div>

                            {/* Right Preview Panel */}
                            <div className="flex-1 bg-black/20 relative">
                                <div className="absolute top-6 right-6 z-10">
                                    <div className="bg-card/80 backdrop-blur-md border border-border px-4 py-2 rounded-full flex items-center gap-2">
                                        <Server className="w-4 h-4 text-green-500 animate-pulse" />
                                        <span className="text-[10px] font-black uppercase tracking-widest text-muted-foreground">Live Topology Preview</span>
                                    </div>
                                </div>
                                <TopologyCanvas databaseRef={test.databaseRef} onChange={(newRef) => updateTest({ databaseRef: newRef })} />
                            </div>
                        </motion.div>
                    )}

                    {currentStep === 3 && (
                        <motion.div key="step3" initial={{ opacity: 0, x: 20 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: -20 }} className="p-12 max-w-2xl mx-auto text-center space-y-8">
                            <div className="w-24 h-24 bg-primary/20 rounded-full flex items-center justify-center mx-auto mb-8 border-4 border-primary shadow-[0_0_50px_rgba(var(--primary),0.4)]">
                                <Check className="w-12 h-12 text-primary" />
                            </div>
                            <h2 className="text-3xl font-black uppercase italic tracking-tighter">Configuration Ready</h2>
                            <p className="text-muted-foreground font-mono">You can now review the source YAML or execute this test scenario.</p>

                            <div className="p-6 bg-card border border-border text-left space-y-2 font-mono text-xs">
                                <div className="flex justify-between"><span className="text-muted-foreground">Test Name:</span> <span className="text-primary font-bold">{test.name}</span></div>
                                <div className="flex justify-between"><span className="text-muted-foreground">Client:</span> <span>Stroppy {test.stroppyCli?.version} ({test.stroppyCli?.workload === 1 ? 'TPC-C' : 'TPC-B'})</span></div>
                                <div className="flex justify-between"><span className="text-muted-foreground">Database:</span> <span>
                                    {test.databaseRef.case === 'connectionString' ? 'External' :
                                        test.databaseRef.value?.template?.case === 'postgresInstance' ? 'Single Instance' : 'HA Cluster'}
                                </span></div>
                            </div>
                        </motion.div>
                    )}

                </AnimatePresence>
            </div>

            {/* Footer Navigation */}
            <div className="p-6 border-t border-border bg-card/20 flex justify-between items-center">
                <button
                    onClick={() => setCurrentStep(prev => Math.max(0, prev - 1))}
                    disabled={currentStep === 0}
                    className="px-6 py-2 text-xs font-black uppercase tracking-widest text-muted-foreground disabled:opacity-30 hover:text-foreground transition-colors"
                >
                    Back
                </button>

                <button
                    onClick={() => setCurrentStep(prev => Math.min(STEPS.length - 1, prev + 1))}
                    disabled={currentStep === STEPS.length - 1}
                    className="px-8 py-3 bg-primary text-primary-foreground text-xs font-black uppercase tracking-widest disabled:opacity-30 disabled:grayscale hover:translate-x-1 transition-all"
                >
                    {currentStep === STEPS.length - 1 ? "Finish" : "Next Step"}
                </button>
            </div>
        </div>
    )
}
