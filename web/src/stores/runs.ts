import { atom } from "nanostores"
import { testApi, streamWorkflowGraph } from "./api"

// Types

export type TestRunStatus =
  | "TEST_RUN_STATUS_PENDING"
  | "TEST_RUN_STATUS_RUNNING"
  | "TEST_RUN_STATUS_COMPLETED"
  | "TEST_RUN_STATUS_FAILED"
  | "TEST_RUN_STATUS_CANCELLED"

export type WorkflowNodeStatus =
  | "WORKFLOW_NODE_STATUS_PENDING"
  | "WORKFLOW_NODE_STATUS_RUNNING"
  | "WORKFLOW_NODE_STATUS_COMPLETED"
  | "WORKFLOW_NODE_STATUS_FAILED"
  | "WORKFLOW_NODE_STATUS_CANCELLED"

export type WorkflowStatus =
  | "WORKFLOW_STATUS_PENDING"
  | "WORKFLOW_STATUS_RUNNING"
  | "WORKFLOW_STATUS_COMPLETED"
  | "WORKFLOW_STATUS_FAILED"
  | "WORKFLOW_STATUS_CANCELLED"

export interface TestRunSummary {
  runId: string
  status: TestRunStatus
  createdAt: string
  completedAt: string
  testSuiteName: string
}

export interface WorkflowNode {
  id: string
  name: string
  status: WorkflowNodeStatus
  startedAt?: string
  completedAt?: string
  error?: string
}

export interface WorkflowEdge {
  fromNodeId: string
  toNodeId: string
}

export interface WorkflowGraph {
  nodes: WorkflowNode[]
  edges: WorkflowEdge[]
  status: WorkflowStatus
}

// Stores

export const $runs = atom<TestRunSummary[]>([])
export const $runsLoading = atom(false)
export const $nextPageToken = atom("")
export const $currentRun = atom<TestRunSummary | null>(null)
export const $workflowGraph = atom<WorkflowGraph | null>(null)

// Streaming abort controller
let streamAbort: AbortController | null = null

// Actions

export async function loadRuns(pageSize?: number, pageToken?: string) {
  $runsLoading.set(true)
  try {
    const resp = await testApi.listTestRuns(pageSize, pageToken)
    const runs = (resp.runs ?? []) as TestRunSummary[]
    if (pageToken) {
      $runs.set([...$runs.get(), ...runs])
    } else {
      $runs.set(runs)
    }
    $nextPageToken.set(resp.nextPageToken ?? "")
  } catch (err) {
    console.error("Failed to load runs:", err)
  } finally {
    $runsLoading.set(false)
  }
}

export async function startStreamingGraph(runId: string) {
  stopStreaming()
  streamAbort = new AbortController()
  const signal = streamAbort.signal

  try {
    for await (const msg of streamWorkflowGraph(runId)) {
      if (signal.aborted) break
      $workflowGraph.set(msg as WorkflowGraph)
    }
  } catch (err) {
    if (!signal.aborted) {
      console.error("Stream error:", err)
    }
  }
}

export function stopStreaming() {
  if (streamAbort) {
    streamAbort.abort()
    streamAbort = null
  }
}
