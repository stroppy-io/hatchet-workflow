import { useState, useMemo } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
  Plus, Play, ScrollText, Wand2, Trash2
} from 'lucide-react'
import { create, toJson } from '@bufbuild/protobuf'
import {
  TestSuiteSchema,
  TestSchema,
  StroppyCli_Workload,
  StroppyCliSchema
} from './proto/stroppy/test_pb'
import { HardwareSchema } from './proto/deployment/deployment_pb'
import { Database_TemplateSchema } from './proto/database/database_pb'
import {
  Postgres_Cluster_TemplateSchema,
  Postgres_Cluster_Template_TopologySchema,
  Postgres_SettingsSchema,
  Postgres_Settings_Version,
  Postgres_Settings_StorageEngine,
  Postgres_AddonsSchema
} from './proto/database/postgres_pb'
import yaml from 'js-yaml'
import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

import { Input } from './components/ui/Input'
import { TestWizard } from './components/TestWizard'

// --- Main App ---

export default function App() {
  const [suite, setSuite] = useState(() => create(TestSuiteSchema, {
    tests: [create(TestSchema, {
      name: "NEON_CLUSTER_01",
      stroppyCli: create(StroppyCliSchema, { version: "v1.0.0", workload: StroppyCli_Workload.TPCC }),
      stroppyHardware: create(HardwareSchema, { cores: 2, memory: 4, disk: 20 }),
      databaseRef: {
        case: "databaseTemplate",
        value: create(Database_TemplateSchema, {
          template: {
            case: "postgresCluster",
            value: create(Postgres_Cluster_TemplateSchema, {
              topology: create(Postgres_Cluster_Template_TopologySchema, {
                replicasCount: 2, monitor: true, settings: create(Postgres_SettingsSchema, { version: Postgres_Settings_Version.VERSION_17, storageEngine: Postgres_Settings_StorageEngine.HEAP }),
                masterHardware: create(HardwareSchema, { cores: 4, memory: 8, disk: 100 }),
                replicaHardware: create(HardwareSchema, { cores: 2, memory: 4, disk: 50 })
              }),
              addons: create(Postgres_AddonsSchema, {})
            })
          }
        })
      }
    })],
    selectedTarget: { target: { case: "dockerSettings", value: { networkName: "forge-net", edgeWorkerImage: "stroppy-edge:latest" } } }
  }))

  const [activeTestIndex, setActiveTestIndex] = useState(0)
  const [showYaml, setShowYaml] = useState(false)
  const activeTest = suite.tests[activeTestIndex]

  const updateActiveTest = (updates: any) => {
    const newTests = [...suite.tests]
    newTests[activeTestIndex] = { ...activeTest, ...updates }
    setSuite({ ...suite, tests: newTests })
  }

  const yamlOutput = useMemo(() => yaml.dump(toJson(TestSuiteSchema, suite)), [suite])

  return (
    <div className="min-h-screen bg-background text-foreground font-sans dark flex flex-col selection:bg-primary selection:text-primary-foreground">
      <div className="fixed inset-0 bg-[linear-gradient(to_right,#80808012_1px,transparent_1px),linear-gradient(to_bottom,#80808012_1px,transparent_1px)] bg-[size:40px_40px] pointer-events-none" />

      <nav className="border-b-2 border-primary bg-card/80 backdrop-blur-xl sticky top-0 z-50 h-20 flex items-center px-8 justify-between">
        <div className="flex items-center gap-4">
          <div className="p-3 bg-primary text-primary-foreground skew-x-[-12deg] shadow-[4px_4px_0px_0px_var(--accent)]"><Wand2 className="w-8 h-8 skew-x-[12deg]" /></div>
          <div>
            <h1 className="text-3xl font-black italic tracking-tighter uppercase leading-none">Stroppy <span className="text-primary">Forge</span></h1>
            <p className="text-[10px] font-black text-muted-foreground uppercase tracking-[0.4em]">Neural Cluster Architect // System.v4</p>
          </div>
        </div>
        <div className="flex gap-4">
          <button onClick={() => setShowYaml(!showYaml)} className="px-6 py-2 text-xs font-black uppercase italic border-2 border-primary bg-secondary text-secondary-foreground hover:bg-primary hover:text-primary-foreground transition-all flex items-center gap-2"><ScrollText className="w-4 h-4" /> Source YAML</button>
          <button className="px-8 py-2 text-xs font-black uppercase italic bg-accent text-accent-foreground shadow-[4px_4px_0px_0px_var(--primary)] hover:translate-x-1 hover:translate-y-1 hover:shadow-none transition-all flex items-center gap-2"><Play className="w-4 h-4" /> Execute</button>
        </div>
      </nav>

      <div className="flex-1 max-w-[1600px] w-full mx-auto grid grid-cols-12 relative z-10 overflow-hidden">
        <aside className="col-span-2 border-r-2 border-border bg-card/20 p-6 space-y-8 overflow-y-auto">
          <div>
            <div className="flex justify-between items-center mb-4 text-[10px] font-black text-primary uppercase tracking-[0.3em]">
              <span><span className="animate-pulse">●</span> Tests</span>
              <Plus className="w-4 h-4 cursor-pointer hover:text-white" onClick={() => {
                const n = create(TestSchema, { name: `NODE_0${suite.tests.length + 1}`, stroppyCli: create(StroppyCliSchema, { version: "v1.0.0" }), stroppyHardware: create(HardwareSchema, { cores: 1, memory: 2, disk: 10 }), databaseRef: { case: "connectionString", value: "" } })
                setSuite({ ...suite, tests: [...suite.tests, n] }); setActiveTestIndex(suite.tests.length);
              }} />
            </div>
            <div className="space-y-2 overflow-y-auto max-h-[40vh] custom-scrollbar">
              {suite.tests.map((t, i) => (
                <div key={i} onClick={() => setActiveTestIndex(i)} className={cn("p-3 border-l-4 cursor-pointer font-mono text-[11px] transition-all flex justify-between group", activeTestIndex === i ? "bg-primary/10 border-primary text-primary" : "border-transparent text-muted-foreground hover:bg-card hover:text-foreground")}>
                  <span className="truncate pr-2">{activeTestIndex === i ? "> " : ""}{t.name}</span>
                </div>
              ))}
            </div>
          </div>
          <div className="pt-8 border-t border-border space-y-4">
            <h2 className="text-[10px] font-black text-accent uppercase tracking-[0.3em] font-mono">suite.selected_target</h2>
            <div className="space-y-4 p-4 bg-background border-t-4 border-accent">
              <Input label="Network Name" fieldName="network_name" value={suite.selectedTarget?.target.case === 'dockerSettings' ? suite.selectedTarget.target.value.networkName : ""} onChange={(e) => {
                const target = JSON.parse(JSON.stringify(suite.selectedTarget)); target.target.value.networkName = e.target.value; setSuite({ ...suite, selectedTarget: target });
              }} />
            </div>
          </div>
        </aside>

        <main className="col-span-10 overflow-hidden bg-card/5">
          <TestWizard test={activeTest} onChange={(updates) => updateActiveTest(updates)} />
        </main>
      </div>

      <AnimatePresence>
        {showYaml && (
          <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="fixed inset-0 bg-background/95 backdrop-blur-xl z-[100] flex items-center justify-center p-8" onClick={() => setShowYaml(false)}>
            <motion.div initial={{ scale: 0.95, opacity: 0 }} animate={{ scale: 1, opacity: 1 }} exit={{ scale: 0.95, opacity: 0 }} onClick={(e) => e.stopPropagation()} className="bg-card border-2 border-primary w-full max-w-4xl h-full max-h-[85vh] flex flex-col shadow-[0_0_50px_rgba(var(--primary),0.3)] overflow-hidden rounded-none">
              <div className="flex items-center justify-between p-6 border-b-2 border-primary bg-primary/5">
                <div className="flex items-center gap-3"><ScrollText className="w-6 h-6 text-primary" /><h2 className="text-2xl font-black uppercase italic tracking-tighter">Source YAML Stream</h2></div>
                <Trash2 className="w-6 h-6 cursor-pointer text-muted-foreground hover:text-white" onClick={() => setShowYaml(false)} />
              </div>
              <div className="flex-1 p-8 overflow-auto font-mono text-sm leading-relaxed custom-scrollbar"><pre className="whitespace-pre-wrap">{yamlOutput}</pre></div>
              <div className="p-6 border-t-2 border-primary bg-primary/5 flex justify-end gap-4"><button onClick={() => navigator.clipboard.writeText(yamlOutput)} className="px-8 py-3 bg-accent text-accent-foreground font-black uppercase italic shadow-[4px_4px_0px_0px_var(--primary)] transition-all active:scale-95">Capture Stream</button></div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}
