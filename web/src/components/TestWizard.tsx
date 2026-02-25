import { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
    StroppyCli_Workload, StroppyCliSchema
} from '../proto/stroppy/test_pb'
import type { Test } from '../proto/stroppy/test_pb'
import { create } from '@bufbuild/protobuf'

import { Input } from './ui/Input'
import { Select } from './ui/Select'
import { HardwareInputs } from './ui/HardwareInputs'
import { TopologyCanvas } from './TopologyCanvas'
import { ShieldCheck, Zap, Layers, Info } from 'lucide-react'
import { cn } from '../App'

interface TestWizardProps {
    test: Test
    onChange: (updates: Partial<Test>) => void
}

const TABS = [
    { id: 'database', label: 'Topology Designer', icon: Layers },
    { id: 'context', label: 'Identity', icon: Info },
    { id: 'client', label: 'Client Config', icon: Zap },
]

export const TestWizard = ({ test, onChange }: TestWizardProps) => {
    const [activeTab, setActiveTab] = useState('database')

    const updateTest = (updates: any) => {
        onChange(updates)
    }

    return (
        <div className="flex flex-col h-full bg-background/50">
            {/* Contextual Tabs */}
            <div className="h-12 border-b border-border bg-card/20 flex items-center px-4 justify-between shrink-0">
                <div className="flex h-full">
                    {TABS.map((tab) => (
                        <button
                            key={tab.id}
                            onClick={() => setActiveTab(tab.id)}
                            className={cn(
                                "px-6 flex items-center gap-2 text-[10px] font-black uppercase tracking-widest transition-all border-b-2 h-full",
                                activeTab === tab.id 
                                    ? "border-primary text-primary bg-primary/5" 
                                    : "border-transparent text-muted-foreground hover:text-foreground hover:bg-white/5"
                            )}
                        >
                            <tab.icon className="w-3.5 h-3.5" />
                            {tab.label}
                        </button>
                    ))}
                </div>
                
                <div className="flex items-center gap-4 text-[10px] font-mono text-muted-foreground uppercase pr-4">
                    <span>Target: <span className="text-primary font-bold">Postgres_HA</span></span>
                    <span className="w-px h-4 bg-border" />
                    <span>State: <span className="text-green-500 font-bold">Valid</span></span>
                </div>
            </div>

            <div className="flex-1 relative overflow-hidden">
                <AnimatePresence mode='wait'>
                    {activeTab === 'context' && (
                        <motion.div 
                            key="context" 
                            initial={{ opacity: 0, y: 10 }} 
                            animate={{ opacity: 1, y: 0 }} 
                            exit={{ opacity: 0, y: -10 }} 
                            className="absolute inset-0 p-12 max-w-3xl mx-auto space-y-12 overflow-y-auto custom-scrollbar"
                        >
                            <div className="space-y-2">
                                <div className="flex items-center gap-3 text-primary">
                                    <ShieldCheck className="w-6 h-6" />
                                    <h2 className="text-2xl font-black uppercase italic tracking-tighter">Test Identity</h2>
                                </div>
                                <p className="text-[10px] font-mono text-muted-foreground uppercase tracking-widest">Define the unique signature of this test scenario.</p>
                            </div>

                            <div className="grid grid-cols-1 gap-8">
                                <Input label="System Name" fieldName="name" value={test.name} onChange={(e) => updateTest({ name: e.target.value })} className="text-xl py-8 font-black tracking-tighter" />
                                <div className="space-y-2">
                                    <label className="text-[9px] font-black uppercase tracking-widest text-muted-foreground px-1">Detailed Description</label>
                                    <textarea
                                        value={test.description || ""}
                                        onChange={(e) => updateTest({ description: e.target.value })}
                                        className="w-full bg-background border border-border p-6 text-xs font-mono h-48 outline-none focus:border-primary transition-all resize-none shadow-inner"
                                        placeholder="Enter scenario objectives, expected metrics, and architecture notes..."
                                    />
                                </div>
                            </div>
                        </motion.div>
                    )}

                    {activeTab === 'client' && (
                        <motion.div 
                            key="client" 
                            initial={{ opacity: 0, y: 10 }} 
                            animate={{ opacity: 1, y: 0 }} 
                            exit={{ opacity: 0, y: -10 }} 
                            className="absolute inset-0 p-12 max-w-4xl mx-auto space-y-12 overflow-y-auto custom-scrollbar"
                        >
                            <div className="space-y-2">
                                <div className="flex items-center gap-3 text-primary">
                                    <Zap className="w-6 h-6" />
                                    <h2 className="text-2xl font-black uppercase italic tracking-tighter">Stroppy Core Config</h2>
                                </div>
                                <p className="text-[10px] font-mono text-muted-foreground uppercase tracking-widest">Configure the high-performance workload generator.</p>
                            </div>

                            <div className="grid grid-cols-2 gap-16">
                                <div className="space-y-8">
                                    <h3 className="text-[10px] font-black uppercase tracking-[0.3em] text-primary/60 flex items-center gap-2">
                                        <div className="w-4 h-0.5 bg-primary" /> Logic Engine
                                    </h3>
                                    <div className="space-y-6">
                                        <Input label="Engine Version" value={test.stroppyCli?.version || ""} onChange={(e) => updateTest({ stroppyCli: create(StroppyCliSchema, { ...test.stroppyCli!, version: e.target.value }) })} />
                                        <Select label="Workload Model" value={test.stroppyCli?.workload} onChange={(v) => updateTest({ stroppyCli: create(StroppyCliSchema, { ...test.stroppyCli!, workload: v }) })} options={[{ label: "TPC-C (Complex Transactions)", value: StroppyCli_Workload.TPCC }, { label: "TPC-B (Simple Account Balance)", value: StroppyCli_Workload.TPCB }]} />
                                    </div>
                                </div>
                                <div className="space-y-8">
                                    <h3 className="text-[10px] font-black uppercase tracking-[0.3em] text-accent/60 flex items-center gap-2">
                                        <div className="w-4 h-0.5 bg-accent" /> Compute Resources
                                    </h3>
                                    <HardwareInputs label="Generator Resources" hardware={test.stroppyHardware} onChange={(h) => updateTest({ stroppyHardware: h })} />
                                </div>
                            </div>
                        </motion.div>
                    )}

                    {activeTab === 'database' && (
                        <motion.div 
                            key="database" 
                            initial={{ opacity: 0 }} 
                            animate={{ opacity: 1 }} 
                            exit={{ opacity: 0 }} 
                            className="absolute inset-0"
                        >
                            <TopologyCanvas test={test} onChange={(updates) => updateTest(updates)} />
                        </motion.div>
                    )}
                </AnimatePresence>
            </div>
        </div>
    )
}
