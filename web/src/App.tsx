import { useState, useMemo } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
  Plus, Play, ScrollText, Wand2, Trash2, Activity, Settings2
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
  Postgres_ClusterSchema,
  Postgres_SettingsSchema,
  Postgres_Settings_Version,
  Postgres_Settings_StorageEngine,
} from './proto/database/postgres_pb'
import { Workflows_StroppyTestSuite_InputSchema } from './proto/workflows/workflows_pb'
import yaml from 'js-yaml'
import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

import { TestWizard } from './components/TestWizard'

// --- Main App ---

export default function App() {
  const [suiteInput, setSuiteInput] = useState(() => create(Workflows_StroppyTestSuite_InputSchema, {
    suite: create(TestSuiteSchema, {
      tests: [create(TestSchema, {
        name: "NEON_CLUSTER_01",
        stroppyCli: create(StroppyCliSchema, { version: "v1.0.0", workload: StroppyCli_Workload.TPCC }),
        stroppyHardware: create(HardwareSchema, { cores: 2, memory: 4, disk: 20 }),
        databaseRef: {
          case: "databaseTemplate",
          value: create(Database_TemplateSchema, {
            template: {
              case: "postgresCluster",
              value: create(Postgres_ClusterSchema, {
                defaults: create(Postgres_SettingsSchema, { version: Postgres_Settings_Version.VERSION_17, storageEngine: Postgres_Settings_StorageEngine.HEAP }),
                nodes: []
              })
            }
          })
        }
      })]
    }),
    target: 0
  }))

  const [activeTestIndex, setActiveTestIndex] = useState(0)
  const [showYaml, setShowYaml] = useState(false)
  const [isExecuting, setIsExecuting] = useState(false)
  const [executionStep, setExecutionStep] = useState(0)
  const [viewMode, setViewMode] = useState<'design' | 'settings'>('design')
  
  const suite = suiteInput.suite!
  const activeTest = suite.tests[activeTestIndex]

  const executionSteps = [
    "Validating cluster topology...",
    "Acquiring network resources...",
    "Planning VM placement...",
    "Provisioning infrastructure...",
    "Bootstrapping Postgres nodes...",
    "Configuring replication...",
    "Setting up HA (Patroni/Etcd)...",
    "Deploying PgBouncer...",
    "Cluster ready!"
  ]

  const handleExecute = () => {
    setIsExecuting(true)
    setExecutionStep(0)
    const interval = setInterval(() => {
      setExecutionStep(prev => {
        if (prev >= executionSteps.length - 1) {
          clearInterval(interval)
          return prev
        }
        return prev + 1
      })
    }, 1500)
  }

  const updateActiveTest = (updates: any) => {
    const newTests = [...suite.tests]
    newTests[activeTestIndex] = { ...activeTest, ...updates }
    setSuiteInput({ ...suiteInput, suite: { ...suite, tests: newTests } })
  }

  const yamlOutput = useMemo(() => yaml.dump(toJson(Workflows_StroppyTestSuite_InputSchema, suiteInput)), [suiteInput])

  return (
    <div className="h-screen w-screen bg-background text-foreground font-sans dark flex flex-col overflow-hidden selection:bg-primary selection:text-primary-foreground">
      {/* Dynamic Grid Background */}
      <div className="fixed inset-0 bg-[linear-gradient(to_right,#80808008_1px,transparent_1px),linear-gradient(to_bottom,#80808008_1px,transparent_1px)] bg-[size:40px_40px] pointer-events-none" />

      {/* Unified Header */}
      <header className="h-16 border-b border-primary/20 bg-card/40 backdrop-blur-xl flex items-center px-6 justify-between shrink-0 z-50">
        <div className="flex items-center gap-6">
          <div className="flex items-center gap-3 group cursor-pointer">
            <div className="p-2 bg-primary text-primary-foreground skew-x-[-12deg] shadow-[3px_3px_0px_0px_var(--accent)] transition-transform group-hover:scale-110">
              <Wand2 className="w-5 h-5 skew-x-[12deg]" />
            </div>
            <div>
              <h1 className="text-xl font-black italic tracking-tighter uppercase leading-none">Stroppy <span className="text-primary">Forge</span></h1>
              <p className="text-[8px] font-bold text-muted-foreground uppercase tracking-[0.3em]">Project: Suite_v1.0</p>
            </div>
          </div>

          <nav className="flex gap-1 bg-background/50 p-1 border border-border">
            <button 
              onClick={() => setViewMode('design')}
              className={cn(
                "px-4 py-1.5 text-[10px] font-black uppercase tracking-widest transition-all",
                viewMode === 'design' ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:text-foreground"
              )}
            >
              Design
            </button>
            <button 
              onClick={() => setViewMode('settings')}
              className={cn(
                "px-4 py-1.5 text-[10px] font-black uppercase tracking-widest transition-all",
                viewMode === 'settings' ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:text-foreground"
              )}
            >
              Suite Settings
            </button>
          </nav>
        </div>

        <div className="flex gap-3">
          <button onClick={() => setShowYaml(!showYaml)} className="px-5 py-2 text-[10px] font-black uppercase italic border border-primary/30 hover:border-primary transition-all flex items-center gap-2">
            <ScrollText className="w-3.5 h-3.5" /> Source YAML
          </button>
          <button onClick={handleExecute} className="px-6 py-2 text-[10px] font-black uppercase italic bg-accent text-accent-foreground shadow-[4px_4px_0px_0px_var(--primary)] hover:translate-x-0.5 hover:translate-y-0.5 hover:shadow-none transition-all flex items-center gap-2">
            <Play className="w-3.5 h-3.5" /> Launch Suite
          </button>
        </div>
      </header>

      {/* Main Integrated Workspace */}
      <div className="flex-1 flex overflow-hidden relative z-10">
        {/* Project Navigator (Sidebar) */}
        <aside className="w-64 border-r border-border bg-card/10 flex flex-col shrink-0">
          <div className="p-4 border-b border-border flex justify-between items-center bg-background/30">
            <span className="text-[9px] font-black uppercase tracking-widest text-primary/60 flex items-center gap-2">
              <Activity className="w-3 h-3 animate-pulse" /> Test Navigator
            </span>
            <Plus 
              className="w-4 h-4 cursor-pointer hover:text-primary transition-colors" 
              onClick={() => {
                const n = create(TestSchema, { name: `TEST_0${suite.tests.length + 1}`, stroppyCli: create(StroppyCliSchema, { version: "v1.0.0" }), stroppyHardware: create(HardwareSchema, { cores: 1, memory: 2, disk: 10 }), databaseRef: { case: "connectionString", value: "" } })
                setSuiteInput({ ...suiteInput, suite: { ...suite, tests: [...suite.tests, n] } }); 
                setActiveTestIndex(suite.tests.length);
              }} 
            />
          </div>
          
          <div className="flex-1 overflow-y-auto p-3 space-y-1 custom-scrollbar">
            {suite.tests.map((t, i) => (
              <div 
                key={i} 
                onClick={() => setActiveTestIndex(i)} 
                className={cn(
                  "p-3 cursor-pointer group transition-all border-l-2 relative overflow-hidden",
                  activeTestIndex === i 
                    ? "bg-primary/10 border-primary text-primary" 
                    : "border-transparent text-muted-foreground hover:bg-white/5 hover:text-foreground"
                )}
              >
                <div className="flex flex-col gap-1">
                  <span className="text-[10px] font-mono uppercase font-black truncate">{t.name}</span>
                  <span className="text-[8px] opacity-50 font-mono truncate">{t.stroppyCli?.workload === 1 ? 'TPC-C' : 'TPC-B'} | {t.stroppyHardware?.cores}C / {t.stroppyHardware?.memory}G</span>
                </div>
                {activeTestIndex === i && (
                  <motion.div layoutId="active-indicator" className="absolute right-0 top-0 bottom-0 w-1 bg-primary" />
                )}
              </div>
            ))}
          </div>

          <div className="p-4 border-t border-border bg-background/30 mt-auto">
            <div className="text-[8px] font-mono text-muted-foreground uppercase mb-2">Build Environment</div>
            <div className="p-2 border border-border bg-background flex items-center justify-between">
               <span className="text-[10px] font-black uppercase text-accent">Docker_Local</span>
               <div className="w-2 h-2 rounded-full bg-green-500 animate-pulse" />
            </div>
          </div>
        </aside>

        {/* Fullscreen Designer Area */}
        <main className="flex-1 relative flex flex-col bg-background overflow-hidden">
          {viewMode === 'design' ? (
            <div className="flex-1 flex flex-col">
              <TestWizard test={activeTest} onChange={(updates) => updateActiveTest(updates)} />
            </div>
          ) : (
            <div className="p-12 max-w-4xl mx-auto space-y-12 w-full overflow-y-auto custom-scrollbar">
               <section className="space-y-6">
                  <h2 className="text-2xl font-black italic uppercase tracking-tighter border-b-2 border-primary pb-2 flex items-center gap-3">
                    <Settings2 className="w-6 h-6 text-primary" /> Suite Configuration
                  </h2>
                  <div className="grid grid-cols-2 gap-8">
                    <div className="space-y-4">
                      <label className="text-[10px] font-black uppercase tracking-widest text-muted-foreground">Network Strategy</label>
                      <div className="p-4 border-2 border-primary/20 bg-card flex justify-between items-center">
                        <span className="text-xs font-mono font-bold uppercase">forge-internal-net</span>
                        <span className="text-[8px] bg-primary/10 text-primary px-2 py-1 font-black">AUTO_GENERATED</span>
                      </div>
                    </div>
                    <div className="space-y-4">
                      <label className="text-[10px] font-black uppercase tracking-widest text-muted-foreground">Suite Concurrency</label>
                      <div className="p-4 border-2 border-primary/20 bg-card flex justify-between items-center">
                        <span className="text-xs font-mono font-bold uppercase underline decoration-primary decoration-2 underline-offset-4">Sequential_Execution</span>
                      </div>
                    </div>
                  </div>
               </section>
            </div>
          )}
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

      <AnimatePresence>
        {isExecuting && (
          <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="fixed inset-0 bg-background/95 backdrop-blur-xl z-[110] flex items-center justify-center p-8">
            <motion.div initial={{ scale: 0.9, opacity: 0 }} animate={{ scale: 1, opacity: 1 }} exit={{ scale: 0.9, opacity: 0 }} className="bg-card border-2 border-accent w-full max-w-2xl p-12 flex flex-col shadow-[0_0_100px_rgba(var(--accent),0.2)] text-center space-y-8">
              <div className="relative w-32 h-32 mx-auto">
                <div className="absolute inset-0 rounded-full border-4 border-accent/20 border-t-accent animate-spin" />
                <div className="absolute inset-4 rounded-full border-4 border-primary/20 border-b-primary animate-[spin_2s_linear_infinite_reverse]" />
                <div className="absolute inset-0 flex items-center justify-center">
                  <Activity className="w-12 h-12 text-accent animate-pulse" />
                </div>
              </div>
              
              <div className="space-y-2">
                <h2 className="text-3xl font-black uppercase italic tracking-tighter">Executing Deployment</h2>
                <p className="text-muted-foreground font-mono text-xs uppercase tracking-widest">{activeTest.name} // Cluster Build Phase</p>
              </div>

              <div className="w-full bg-background border border-border h-4 relative overflow-hidden">
                <motion.div 
                  className="absolute inset-y-0 left-0 bg-accent" 
                  initial={{ width: "0%" }}
                  animate={{ width: `${(executionStep + 1) / executionSteps.length * 100}%` }}
                />
              </div>

              <div className="space-y-4">
                <p className="text-xl font-black italic uppercase text-primary animate-pulse">{executionSteps[executionStep]}</p>
                <div className="flex flex-col gap-1 items-center max-h-32 overflow-hidden opacity-40 grayscale">
                   {executionSteps.slice(0, executionStep).map((step, i) => (
                     <div key={i} className="text-[10px] font-mono line-through">{step}</div>
                   ))}
                </div>
              </div>

              {executionStep === executionSteps.length - 1 && (
                <button 
                  onClick={() => setIsExecuting(false)}
                  className="mt-8 px-12 py-4 bg-primary text-primary-foreground font-black uppercase italic shadow-[6px_6px_0px_0px_var(--accent)] hover:translate-x-1 hover:translate-y-1 hover:shadow-none transition-all"
                >
                  Return to Forge
                </button>
              )}
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}
