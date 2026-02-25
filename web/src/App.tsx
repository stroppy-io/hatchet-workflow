import { useState, useMemo } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
  Plus, Play, Trash2, Activity, Settings2, Terminal, Database, ChevronRight, FileCode, Check, Copy, Users, Layers, PlayCircle
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
import { SettingsSchema } from './proto/settings/settings_pb'
import { Workflows_StroppyTestSuite_InputSchema } from './proto/workflows/workflows_pb'
import yaml from 'js-yaml'
import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

import { TestWizard } from './components/TestWizard'
import { SettingsEditor } from './components/SettingsEditor'

type Section = 'editor' | 'execution' | 'users' | 'settings'

export default function App() {
  const [activeSection, setActiveSection] = useState<Section>('editor')
  const [suiteInput, setSuiteInput] = useState(() => create(Workflows_StroppyTestSuite_InputSchema, {
    suite: create(TestSuiteSchema, {
      tests: [create(TestSchema, {
        name: "test_scenario_primary",
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
    settings: {
      hatchetConnection: { host: "localhost", port: 8080, token: "forge-dev-token" },
      docker: { networkName: "forge-net", edgeWorkerImage: "stroppy-edge:latest" },
      preferredTarget: 0
    },
    target: 0
  }))

  const [activeTestIndex, setActiveTestIndex] = useState(0)
  const [showYaml, setShowYaml] = useState(false)
  const [isExecuting, setIsExecuting] = useState(false)
  const [executionStep, setExecutionStep] = useState(0)
  
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
    <div className="h-screen w-screen bg-[#0d0d0d] text-[#cccccc] font-sans dark flex flex-row overflow-hidden selection:bg-primary/30">
      
      {/* 1. IDE Activity Bar (Vertical Navigation) */}
      <nav className="w-12 border-r border-[#2b2b2b] bg-[#181818] flex flex-col items-center py-4 gap-4 shrink-0 z-[60]">
        <div className="w-8 h-8 bg-primary/10 border border-primary/20 rounded-sm flex items-center justify-center mb-4 group cursor-pointer shadow-lg shadow-primary/5">
          <Terminal className="w-4 h-4 text-primary group-hover:scale-110 transition-transform" />
        </div>

        <button 
          onClick={() => setActiveSection('editor')}
          className={cn(
            "p-2 transition-colors relative group",
            activeSection === 'editor' ? "text-primary" : "text-[#858585] hover:text-[#cccccc]"
          )}
          title="Test Editor"
        >
          <Layers className="w-5 h-5" />
          {activeSection === 'editor' && <div className="absolute left-0 top-1/4 bottom-1/4 w-0.5 bg-primary rounded-full shadow-[0_0_8px_var(--primary)]" />}
        </button>

        <button 
          onClick={() => setActiveSection('execution')}
          className={cn(
            "p-2 transition-colors relative group",
            activeSection === 'execution' ? "text-primary" : "text-[#858585] hover:text-[#cccccc]"
          )}
          title="Suite Execution"
        >
          <PlayCircle className="w-5 h-5" />
          {activeSection === 'execution' && <div className="absolute left-0 top-1/4 bottom-1/4 w-0.5 bg-primary rounded-full" />}
        </button>

        <button 
          onClick={() => setActiveSection('users')}
          className={cn(
            "p-2 transition-colors relative group",
            activeSection === 'users' ? "text-primary" : "text-[#858585] hover:text-[#cccccc]"
          )}
          title="User Management"
        >
          <Users className="w-5 h-5" />
          {activeSection === 'users' && <div className="absolute left-0 top-1/4 bottom-1/4 w-0.5 bg-primary rounded-full" />}
        </button>

        <div className="mt-auto">
          <button 
            onClick={() => setActiveSection('settings')}
            className={cn(
              "p-2 transition-colors relative group",
              activeSection === 'settings' ? "text-primary" : "text-[#858585] hover:text-[#cccccc]"
            )}
            title="System Settings"
          >
            <Settings2 className="w-5 h-5" />
            {activeSection === 'settings' && <div className="absolute left-0 top-1/4 bottom-1/4 w-0.5 bg-primary rounded-full" />}
          </button>
        </div>
      </nav>

      {/* Main Container */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* IDE Header (Horizontal) */}
        <header className="h-10 border-b border-[#2b2b2b] bg-[#1e1e1e] flex items-center px-4 justify-between shrink-0 z-50 shadow-sm">
          <div className="flex items-center gap-4">
            <span className="text-[10px] font-bold text-[#858585] uppercase tracking-widest flex items-center gap-2">
              <ChevronRight className="w-3 h-3" />
              STROPPY_FORGE <span className="opacity-40">/</span> {activeSection.toUpperCase()}
            </span>
          </div>

          <div className="flex items-center gap-3">
            <button 
              onClick={() => setShowYaml(!showYaml)} 
              className="h-6 px-2.5 text-[9px] font-bold uppercase tracking-wider bg-[#333333] hover:bg-[#404040] border border-[#454545] rounded-sm transition-all flex items-center gap-2 shadow-sm"
            >
              <FileCode className="w-3 h-3" /> Manifest
            </button>
            <button 
              onClick={handleExecute} 
              className="h-6 px-3 text-[9px] font-black uppercase tracking-widest bg-primary text-primary-foreground hover:brightness-110 rounded-sm transition-all flex items-center gap-2 shadow-[0_0_15px_rgba(var(--primary),0.1)]"
            >
              <Play className="w-2.5 h-2.5 fill-current" /> Run
            </button>
          </div>
        </header>

        <div className="flex-1 flex overflow-hidden">
          {activeSection === 'editor' && (
            <>
              {/* Sidebar: Project Explorer */}
              <aside className="w-56 border-r border-[#2b2b2b] bg-[#181818] flex flex-col shrink-0">
                <div className="h-8 px-4 border-b border-[#2b2b2b] flex justify-between items-center bg-[#1e1e1e]/50">
                  <span className="text-[9px] font-bold uppercase tracking-wider text-[#858585]">
                    Explorer
                  </span>
                  <Plus 
                    className="w-3 h-3 cursor-pointer text-[#858585] hover:text-primary transition-colors" 
                    onClick={() => {
                      const n = create(TestSchema, { name: `scenario_${suite.tests.length + 1}`, stroppyCli: create(StroppyCliSchema, { version: "v1.0.0" }), stroppyHardware: create(HardwareSchema, { cores: 1, memory: 2, disk: 10 }), databaseRef: { case: "connectionString", value: "" } })
                      setSuiteInput({ ...suiteInput, suite: { ...suite, tests: [...suite.tests, n] } }); 
                      setActiveTestIndex(suite.tests.length);
                    }} 
                  />
                </div>
                
                <div className="flex-1 overflow-y-auto py-2 custom-scrollbar">
                  <div className="px-2">
                    <div className="flex items-center gap-1 text-[9px] font-bold text-[#858585]/60 px-2 py-1 uppercase tracking-widest">
                      <ChevronRight className="w-3 h-3" />
                      <span>TestSuite</span>
                    </div>
                    <div className="pl-2 space-y-0.5 mt-1">
                      {suite.tests.map((t, i) => (
                        <div 
                          key={i} 
                          onClick={() => setActiveTestIndex(i)} 
                          className={cn(
                            "flex items-center gap-3 px-3 py-1.5 cursor-pointer rounded-sm transition-all text-[11px] font-medium",
                            activeTestIndex === i 
                              ? "bg-[#37373d] text-primary" 
                              : "text-[#858585] hover:bg-[#2a2d2e] hover:text-[#cccccc]"
                          )}
                        >
                          <Database className={cn("w-3.5 h-3.5 shrink-0 transition-colors", activeTestIndex === i ? "text-primary" : "text-[#454545]")} />
                          <span className="truncate tracking-tight">{t.name}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                </div>

                <div className="p-3 border-t border-[#2b2b2b] bg-[#1e1e1e]">
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-[8px] font-bold text-[#858585] uppercase">Live</span>
                    <div className="w-1.5 h-1.5 rounded-full bg-green-500 shadow-[0_0_5px_rgba(34,197,94,0.4)] animate-pulse" />
                  </div>
                  <div className="text-[9px] font-mono text-[#858585] bg-[#0d0d0d] p-2 rounded border border-[#2b2b2b]">
                    $ forge --connected
                  </div>
                </div>
              </aside>

              {/* Editor Workspace */}
              <main className="flex-1 relative bg-[#1e1e1e] flex flex-col overflow-hidden">
                <TestWizard test={activeTest} onChange={(updates) => updateActiveTest(updates)} />
              </main>
            </>
          )}

          {activeSection === 'execution' && (
            <main className="flex-1 bg-[#1e1e1e] p-8 overflow-y-auto custom-scrollbar">
              <div className="max-w-4xl space-y-8">
                <div className="space-y-1">
                  <h2 className="text-xl font-bold text-[#e1e1e1] uppercase tracking-tight">Suite Execution History</h2>
                  <div className="h-0.5 w-12 bg-primary/40 rounded-full" />
                </div>
                <div className="p-12 border border-dashed border-[#333] rounded-lg text-center bg-[#181818]/50">
                  <PlayCircle className="w-12 h-12 text-[#333] mx-auto mb-4" />
                  <p className="text-xs text-[#858585] uppercase font-mono">No active execution streams detected.</p>
                </div>
              </div>
            </main>
          )}

          {activeSection === 'users' && (
            <main className="flex-1 bg-[#1e1e1e] p-8 overflow-y-auto custom-scrollbar">
              <div className="max-w-4xl space-y-8">
                <div className="space-y-1">
                  <h2 className="text-xl font-bold text-[#e1e1e1] uppercase tracking-tight">Team Management</h2>
                  <div className="h-0.5 w-12 bg-primary/40 rounded-full" />
                </div>
                <div className="grid grid-cols-3 gap-4">
                  <div className="p-4 bg-[#252526] border border-[#2b2b2b] rounded flex items-center gap-4 shadow-sm">
                    <div className="w-10 h-10 rounded-full bg-primary/10 flex items-center justify-center text-primary font-bold">A</div>
                    <div className="flex flex-col">
                      <span className="text-xs font-bold text-[#e1e1e1]">Admin User</span>
                      <span className="text-[10px] text-[#858585]">Superuser</span>
                    </div>
                  </div>
                </div>
              </div>
            </main>
          )}

          {activeSection === 'settings' && (
            <SettingsEditor 
              settings={suiteInput.settings!} 
              onChange={(updates) => {
                const next = create(SettingsSchema, suiteInput.settings);
                Object.assign(next, updates);
                setSuiteInput({ ...suiteInput, settings: next });
              }} 
            />
          )}
        </div>
      </div>

      {/* Manifest Editor & Runner overlays remain the same... */}
      <AnimatePresence>
        {showYaml && (
          <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="fixed inset-0 bg-black/70 backdrop-blur-md z-[100] flex items-center justify-center p-12" onClick={() => setShowYaml(false)}>
            <motion.div initial={{ scale: 0.98, y: 10 }} animate={{ scale: 1, y: 0 }} exit={{ scale: 0.98, y: 10 }} onClick={(e) => e.stopPropagation()} className="bg-[#1e1e1e] border border-[#454545] w-full max-w-5xl h-full flex flex-col shadow-[0_20px_50px_rgba(0,0,0,0.5)] overflow-hidden rounded">
              <div className="h-11 flex items-center justify-between px-4 border-b border-[#2b2b2b] bg-[#252526]">
                <div className="flex items-center gap-2">
                  <FileCode className="w-4 h-4 text-primary" />
                  <span className="text-[11px] font-bold uppercase tracking-widest text-[#cccccc]">manifest.yaml</span>
                </div>
                <button onClick={() => setShowYaml(false)} className="p-1.5 hover:bg-destructive/10 rounded transition-all text-[#858585] hover:text-destructive">
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
              <div className="flex-1 p-8 overflow-auto font-mono text-[12px] leading-relaxed bg-[#0d0d0d] text-[#d4d4d4] custom-scrollbar selection:bg-primary/20">
                <pre className="whitespace-pre-wrap">{yamlOutput}</pre>
              </div>
              <div className="p-3 border-t border-[#2b2b2b] flex justify-end gap-3 bg-[#252526]">
                <button onClick={() => navigator.clipboard.writeText(yamlOutput)} className="h-8 px-5 bg-primary text-primary-foreground text-[10px] font-bold uppercase tracking-widest rounded-sm hover:brightness-110 transition-all flex items-center gap-2 shadow-lg shadow-primary/10">
                  <Copy className="w-3.5 h-3.5" /> Copy to Clipboard
                </button>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>

      <AnimatePresence>
        {isExecuting && (
          <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="fixed inset-0 bg-black/80 backdrop-blur-xl z-[110] flex items-center justify-center p-8">
            <motion.div initial={{ scale: 0.95, opacity: 0 }} animate={{ scale: 1, opacity: 1 }} exit={{ scale: 0.95, opacity: 0 }} className="w-full max-w-md bg-[#1e1e1e] border border-primary/20 p-8 rounded shadow-[0_0_50px_rgba(var(--primary),0.1)] flex flex-col space-y-10">
              <div className="space-y-3">
                <div className="w-12 h-12 bg-primary/10 rounded-full flex items-center justify-center border border-primary/20 shadow-[0_0_20px_rgba(var(--primary),0.1)]">
                  <Activity className="w-6 h-6 text-primary animate-pulse" />
                </div>
                <div className="space-y-1">
                  <h2 className="text-lg font-bold text-[#e1e1e1] uppercase tracking-tight">Deploying Suite</h2>
                  <p className="text-[10px] font-mono text-[#858585] uppercase tracking-widest">{activeTest.name}</p>
                </div>
              </div>

              <div className="space-y-5">
                <div className="w-full bg-[#2b2b2b] h-1 rounded-full overflow-hidden shadow-inner">
                  <motion.div className="h-full bg-primary shadow-[0_0_10px_rgba(var(--primary),0.5)]" initial={{ width: "0%" }} animate={{ width: `${(executionStep + 1) / executionSteps.length * 100}%` }} />
                </div>
                <p className="text-[10px] font-black text-primary text-center uppercase tracking-[0.2em]">{executionSteps[executionStep]}</p>
              </div>

              <div className="flex flex-col gap-2 h-32 overflow-hidden border-t border-[#2b2b2b] pt-5">
                 {executionSteps.slice(0, executionStep).map((step, i) => (
                   <div key={i} className="text-[10px] font-mono text-[#858585]/60 flex items-center gap-3">
                     <Check className="w-3.5 h-3.5 text-green-500" /> <span className="uppercase tracking-tighter">{step}</span>
                   </div>
                 ))}
              </div>

              {executionStep === executionSteps.length - 1 && (
                <button onClick={() => setIsExecuting(false)} className="w-full h-11 bg-primary text-primary-foreground text-[11px] font-bold uppercase tracking-widest rounded-sm hover:brightness-110 transition-all shadow-lg shadow-primary/10">Return to Forge</button>
              )}
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}
