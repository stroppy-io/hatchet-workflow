import { atom, computed } from "nanostores"
import { testApi, topologyApi } from "./api"

// ── Types ──────────────────────────────────────────────────────

export type WorkloadType = "TPCC" | "TPCB"
export type DatabaseType = "postgres-instance" | "postgres-cluster" | "picodata-instance" | "picodata-cluster"
export type PostgresVersion = "VERSION_17" | "VERSION_16" | "VERSION_18"
export type StorageEngine = "HEAP" | "ORIOLEDB"
export type NodeRole = "MASTER" | "REPLICA"

export interface Hardware {
  cores: number
  memory: number
  disk: number
}

export interface StroppyCli {
  version: string
  binaryPath: string
  workdir: string
  workload: WorkloadType
  scaleFactor: number
  duration: string
  stroppyEnv: Record<string, string>
}

export interface PostgresNode {
  name: string
  hardware: Hardware
  role: NodeRole
  hasPgbouncer: boolean
  hasEtcd: boolean
  hasMonitoring: boolean
  hasBackup: boolean
}

export interface PostgresTemplate {
  type: "postgres-instance" | "postgres-cluster"
  version: PostgresVersion
  storageEngine: StorageEngine
  nodes: PostgresNode[]
}

export type DatabaseTemplate = PostgresTemplate

export interface Test {
  name: string
  description: string
  stroppyCli: StroppyCli
  stroppyHardware: Hardware
  databaseTemplate: DatabaseTemplate | null
}

export interface TestSuite {
  tests: Test[]
}

export interface ValidationError {
  fieldPath: string
  severity: string
  message: string
}

// ── Helpers ────────────────────────────────────────────────────

let nodeCounter = 1

function createDefaultNode(role: NodeRole): PostgresNode {
  const id = nodeCounter++
  return {
    name: role === "MASTER" ? `pg-master-${id}` : `pg-replica-${id}`,
    hardware: { cores: 4, memory: 8, disk: 100 },
    role,
    hasPgbouncer: false,
    hasEtcd: role === "MASTER",
    hasMonitoring: false,
    hasBackup: false,
  }
}

export function createDefaultTest(index: number = 1): Test {
  nodeCounter = 1
  return {
    name: `test-${index}`,
    description: "",
    stroppyCli: {
      version: "latest",
      binaryPath: "/usr/local/bin/stroppy",
      workdir: "/tmp/stroppy",
      workload: "TPCC",
      scaleFactor: 10,
      duration: "5m",
      stroppyEnv: {},
    },
    stroppyHardware: { cores: 2, memory: 4, disk: 50 },
    databaseTemplate: {
      type: "postgres-instance",
      version: "VERSION_17",
      storageEngine: "HEAP",
      nodes: [createDefaultNode("MASTER")],
    },
  }
}

// ── Atoms ──────────────────────────────────────────────────────

export const $testSuite = atom<TestSuite>({ tests: [createDefaultTest(1)] })
export const $currentTestIndex = atom<number>(0)
export const $selectedNodeId = atom<string | null>(null)
export const $validationErrors = atom<ValidationError[]>([])
export const $editorDirty = atom<boolean>(false)
export const $bottomPanelOpen = atom<boolean>(true)
export const $dbType = atom<DatabaseType>("postgres-instance")

export const $currentTest = computed(
  [$testSuite, $currentTestIndex],
  (suite, index) => suite.tests[index] ?? null,
)

// ── Actions ────────────────────────────────────────────────────

export function addTest() {
  const suite = $testSuite.get()
  const newTest = createDefaultTest(suite.tests.length + 1)
  $testSuite.set({ tests: [...suite.tests, newTest] })
  $currentTestIndex.set(suite.tests.length)
  $editorDirty.set(true)
}

export function removeTest(index: number) {
  const suite = $testSuite.get()
  if (suite.tests.length <= 1) return
  const tests = suite.tests.filter((_, i) => i !== index)
  $testSuite.set({ tests })
  if ($currentTestIndex.get() >= tests.length) {
    $currentTestIndex.set(tests.length - 1)
  }
  $editorDirty.set(true)
}

export function updateTest(index: number, patch: Partial<Test>) {
  const suite = $testSuite.get()
  const tests = suite.tests.map((t, i) => (i === index ? { ...t, ...patch } : t))
  $testSuite.set({ tests })
  $editorDirty.set(true)
}

export function addNode(role: NodeRole) {
  const index = $currentTestIndex.get()
  const suite = $testSuite.get()
  const test = suite.tests[index]
  if (!test?.databaseTemplate) return

  const node = createDefaultNode(role)
  const template = {
    ...test.databaseTemplate,
    nodes: [...test.databaseTemplate.nodes, node],
  }
  updateTest(index, { databaseTemplate: template })
}

export function removeNode(nodeName: string) {
  const index = $currentTestIndex.get()
  const suite = $testSuite.get()
  const test = suite.tests[index]
  if (!test?.databaseTemplate) return

  const nodes = test.databaseTemplate.nodes.filter((n) => n.name !== nodeName)
  updateTest(index, {
    databaseTemplate: { ...test.databaseTemplate, nodes },
  })
  if ($selectedNodeId.get() === nodeName) {
    $selectedNodeId.set(null)
  }
}

export function updateNode(nodeName: string, patch: Partial<PostgresNode>) {
  const index = $currentTestIndex.get()
  const suite = $testSuite.get()
  const test = suite.tests[index]
  if (!test?.databaseTemplate) return

  const nodes = test.databaseTemplate.nodes.map((n) =>
    n.name === nodeName ? { ...n, ...patch } : n,
  )
  updateTest(index, {
    databaseTemplate: { ...test.databaseTemplate, nodes },
  })
}

export function setDatabaseType(type: DatabaseType) {
  $dbType.set(type)
  const index = $currentTestIndex.get()
  const suite = $testSuite.get()
  const test = suite.tests[index]
  if (!test) return

  if (type.startsWith("postgres")) {
    const template: PostgresTemplate = {
      type: type as "postgres-instance" | "postgres-cluster",
      version: test.databaseTemplate?.version ?? "VERSION_17",
      storageEngine: test.databaseTemplate?.storageEngine ?? "HEAP",
      nodes: test.databaseTemplate?.nodes ?? [createDefaultNode("MASTER")],
    }
    updateTest(index, { databaseTemplate: template })
  }
}

export async function validateTopology() {
  try {
    const suite = $testSuite.get()
    const result = await topologyApi.validateTopology(suite)
    $validationErrors.set(result.errors ?? [])
  } catch {
    $validationErrors.set([
      { fieldPath: "", severity: "error", message: "Validation request failed" },
    ])
  }
}

export async function runTestSuite(): Promise<string | null> {
  try {
    const suite = $testSuite.get()
    const result = await testApi.runTestSuite(suite)
    return result.runId
  } catch {
    return null
  }
}
