import { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { create } from '@bufbuild/protobuf'
import {
    StroppyCli_Workload, StroppyCliSchema
} from '../proto/stroppy/test_pb'
import type { Test } from '../proto/stroppy/test_pb'
import {
    Postgres_Settings_Version,
    Postgres_Settings_StorageEngine,
    Postgres_Addons_Pooling_Pgbouncer_PoolMode
} from '../proto/database/postgres_pb'

import { Stepper } from './ui/Stepper'
import { Input } from './ui/Input'
import { Select } from './ui/Select'
import { HardwareInputs } from './ui/HardwareInputs'
import { TopologyCanvas } from './TopologyCanvas'
import { ShieldCheck, Zap, Check } from 'lucide-react'

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
                            {/* Full Screen Interactive Canvas */}
                            <div className="flex-1 bg-black/20 relative">
                                <TopologyCanvas databaseRef={test.databaseRef} onChange={(updates) => updateTest(updates)} />
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
