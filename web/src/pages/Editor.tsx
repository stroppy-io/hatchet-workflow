import { useCallback, useState } from "react"
import { useStore } from "@nanostores/react"
import { useNavigate } from "react-router"
import { motion, AnimatePresence } from "framer-motion"
import {
  Play,
  Plus,
  X,
  ChevronDown,
  ChevronUp,
  Database,
  CheckCircle,
  AlertCircle,
  FileCode,
  ShieldCheck,
} from "lucide-react"
import { ReactFlowProvider } from "@xyflow/react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs"
import { TestConfigPanel } from "@/components/editor/TestConfigPanel"
import { TopologyCanvas } from "@/components/editor/TopologyCanvas"
import { NodeProperties } from "@/components/editor/NodeProperties"
import { YamlPreview } from "@/components/editor/YamlPreview"
import { ValidationPanel } from "@/components/editor/ValidationPanel"
import {
  $testSuite,
  $currentTestIndex,
  $selectedNodeId,
  $validationErrors,
  $editorDirty,
  $bottomPanelOpen,
  $dbType,
  addTest,
  removeTest,
  addNode,
  setDatabaseType,
  validateTopology,
  runTestSuite,
  type DatabaseType,
} from "@/stores/editor"

// ── Toolbar ────────────────────────────────────────────────────

function EditorToolbar() {
  const dbType = useStore($dbType)
  const errors = useStore($validationErrors)
  const dirty = useStore($editorDirty)
  const navigate = useNavigate()
  const [running, setRunning] = useState(false)

  const handleRun = useCallback(async () => {
    setRunning(true)
    await validateTopology()
    const runId = await runTestSuite()
    setRunning(false)
    if (runId) {
      navigate(`/runs/${runId}`)
    }
  }, [navigate])

  return (
    <div className="flex items-center gap-2 border-b border-border bg-secondary/20 px-3 py-1.5">
      <select
        value={dbType}
        onChange={(e) => setDatabaseType(e.target.value as DatabaseType)}
        className="h-7 border border-input bg-background px-2 text-[12px] focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
      >
        <option value="postgres-instance">Postgres Instance</option>
        <option value="postgres-cluster">Postgres Cluster</option>
        <option value="picodata-instance">Picodata Instance</option>
        <option value="picodata-cluster">Picodata Cluster</option>
      </select>

      <Button variant="outline" size="sm" className="text-[12px] h-7" onClick={addTest}>
        <Plus className="h-3 w-3" />
        Add Test
      </Button>

      <div className="flex items-center gap-1.5">
        <Button
          variant="outline"
          size="sm"
          className="text-[12px] h-7"
          onClick={() => addNode("MASTER")}
        >
          <Database className="h-3 w-3 text-primary" />
          Master
        </Button>
        <Button
          variant="outline"
          size="sm"
          className="text-[12px] h-7"
          onClick={() => addNode("REPLICA")}
        >
          <Database className="h-3 w-3 text-muted-foreground" />
          Replica
        </Button>
      </div>

      <div className="ml-auto flex items-center gap-2">
        {errors.length > 0 && (
          <Badge variant="destructive" className="text-[10px]">
            <AlertCircle className="h-3 w-3 mr-1" />
            {errors.length} error{errors.length > 1 ? "s" : ""}
          </Badge>
        )}
        {errors.length === 0 && dirty && (
          <Badge variant="secondary" className="text-[10px]">Modified</Badge>
        )}
        <Button
          variant="outline"
          size="sm"
          className="text-[12px] h-7"
          onClick={() => validateTopology()}
        >
          <ShieldCheck className="h-3 w-3" />
          Validate
        </Button>
        <Button size="sm" className="text-[12px] h-7" onClick={handleRun} disabled={running}>
          <Play className="h-3 w-3" />
          {running ? "Running..." : "Run"}
        </Button>
      </div>
    </div>
  )
}

// ── Test Tabs ──────────────────────────────────────────────────

function TestTabs() {
  const suite = useStore($testSuite)
  const currentIndex = useStore($currentTestIndex)

  return (
    <div className="flex items-center border-b border-border bg-secondary/30 overflow-x-auto">
      {suite.tests.map((test, i) => (
        <button
          key={i}
          type="button"
          className={`flex items-center gap-1.5 px-3 py-1.5 text-[12px] border-r border-border transition-colors shrink-0 ${
            i === currentIndex
              ? "bg-card text-foreground border-b-2 border-b-primary -mb-px"
              : "text-muted-foreground hover:text-foreground hover:bg-accent/30"
          }`}
          onClick={() => $currentTestIndex.set(i)}
        >
          <span className="truncate max-w-[100px]">{test.name}</span>
          {suite.tests.length > 1 && (
            <button
              type="button"
              className="ml-1 p-0.5 hover:text-destructive"
              onClick={(e) => {
                e.stopPropagation()
                removeTest(i)
              }}
            >
              <X className="h-3 w-3" />
            </button>
          )}
        </button>
      ))}
    </div>
  )
}

// ── Bottom Panel ───────────────────────────────────────────────

function BottomPanel() {
  const isOpen = useStore($bottomPanelOpen)

  return (
    <div className="border-t border-border bg-card flex flex-col shrink-0">
      <button
        type="button"
        className="flex items-center gap-1.5 px-3 py-1 text-[11px] text-muted-foreground hover:text-foreground bg-secondary/30 border-b border-border"
        onClick={() => $bottomPanelOpen.set(!isOpen)}
      >
        {isOpen ? <ChevronDown className="h-3 w-3" /> : <ChevronUp className="h-3 w-3" />}
        Tool Window
      </button>

      <AnimatePresence>
        {isOpen && (
          <motion.div
            initial={{ height: 0 }}
            animate={{ height: 192 }}
            exit={{ height: 0 }}
            transition={{ duration: 0.15 }}
            className="overflow-hidden"
          >
            <Tabs defaultValue="yaml" className="flex flex-col h-48">
              <TabsList>
                <TabsTrigger value="yaml" className="gap-1 text-[11px]">
                  <FileCode className="h-3 w-3" />
                  YAML
                </TabsTrigger>
                <TabsTrigger value="validation" className="gap-1 text-[11px]">
                  <CheckCircle className="h-3 w-3" />
                  Validation
                </TabsTrigger>
              </TabsList>
              <TabsContent value="yaml" className="flex-1 min-h-0 overflow-hidden">
                <YamlPreview />
              </TabsContent>
              <TabsContent value="validation" className="flex-1 min-h-0 overflow-hidden">
                <ValidationPanel />
              </TabsContent>
            </Tabs>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}

// ── Main Layout ────────────────────────────────────────────────

export function EditorPage() {
  const selectedNodeId = useStore($selectedNodeId)

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.2 }}
      className="flex flex-col h-full"
    >
      <EditorToolbar />
      <TestTabs />

      <div className="flex flex-1 min-h-0">
        {/* Left: Config Panel */}
        <div className="w-64 shrink-0 border-r border-border bg-card overflow-hidden">
          <TestConfigPanel />
        </div>

        {/* Center: Topology Canvas */}
        <div className="flex-1 min-w-0">
          <ReactFlowProvider>
            <TopologyCanvas />
          </ReactFlowProvider>
        </div>

        {/* Right: Node Properties */}
        <AnimatePresence>
          {selectedNodeId && (
            <motion.div
              initial={{ width: 0, opacity: 0 }}
              animate={{ width: 256, opacity: 1 }}
              exit={{ width: 0, opacity: 0 }}
              transition={{ duration: 0.15 }}
              className="shrink-0 overflow-hidden"
            >
              <NodeProperties />
            </motion.div>
          )}
        </AnimatePresence>
      </div>

      <BottomPanel />
    </motion.div>
  )
}
