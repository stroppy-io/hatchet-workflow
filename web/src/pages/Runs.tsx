import { useEffect, useCallback } from "react"
import { useNavigate } from "react-router"
import { useStore } from "@nanostores/react"
import { RefreshCw, ArrowRight, Loader2 } from "lucide-react"
import { motion } from "framer-motion"
import { Button } from "@/components/ui/button"
import { RunStatusBadge } from "@/components/runs/RunStatusBadge"
import { $runs, $runsLoading, $nextPageToken, loadRuns } from "@/stores/runs"
import type { TestRunSummary } from "@/stores/runs"

function relativeTime(iso: string): string {
  if (!iso) return "--"
  const diff = Date.now() - new Date(iso).getTime()
  const secs = Math.floor(diff / 1000)
  if (secs < 60) return `${secs}s ago`
  const mins = Math.floor(secs / 60)
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h ago`
  const days = Math.floor(hrs / 24)
  return `${days}d ago`
}

function formatDuration(start: string, end: string): string {
  if (!start || !end) return "--"
  const diff = new Date(end).getTime() - new Date(start).getTime()
  if (diff < 0) return "--"
  const secs = Math.floor(diff / 1000)
  if (secs < 60) return `${secs}s`
  const mins = Math.floor(secs / 60)
  const remSecs = secs % 60
  if (mins < 60) return `${mins}m ${remSecs}s`
  const hrs = Math.floor(mins / 60)
  const remMins = mins % 60
  return `${hrs}h ${remMins}m`
}

export function RunsPage() {
  const runs = useStore($runs)
  const loading = useStore($runsLoading)
  const nextToken = useStore($nextPageToken)
  const navigate = useNavigate()

  const refresh = useCallback(() => {
    loadRuns(20)
  }, [])

  useEffect(() => {
    refresh()
    const interval = setInterval(() => {
      if (document.hasFocus()) refresh()
    }, 10000)
    return () => clearInterval(interval)
  }, [refresh])

  return (
    <div className="flex flex-col h-full">
      {/* Header bar */}
      <div className="flex items-center justify-between border-b bg-secondary/30 px-3 py-1.5">
        <span className="text-[14px] font-semibold">Test Runs</span>
        <Button
          variant="ghost"
          size="sm"
          className="h-6 text-[11px] gap-1"
          onClick={refresh}
          disabled={loading}
        >
          <RefreshCw className={`h-3 w-3 ${loading ? "animate-spin" : ""}`} />
          Refresh
        </Button>
      </div>

      {/* Table */}
      <div className="flex-1 overflow-auto">
        {loading && runs.length === 0 ? (
          <div className="flex items-center justify-center h-32">
            <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
          </div>
        ) : runs.length === 0 ? (
          <div className="flex items-center justify-center h-32">
            <p className="text-[13px] text-muted-foreground">No test runs yet</p>
          </div>
        ) : (
          <>
            <table className="w-full text-[13px]">
              <thead>
                <tr className="border-b bg-secondary/20 text-[11px] uppercase tracking-wide text-muted-foreground">
                  <th className="px-3 py-1.5 text-left font-medium">Status</th>
                  <th className="px-3 py-1.5 text-left font-medium">Test Suite</th>
                  <th className="px-3 py-1.5 text-left font-medium">Started</th>
                  <th className="px-3 py-1.5 text-left font-medium">Duration</th>
                  <th className="px-3 py-1.5 text-right font-medium w-16"></th>
                </tr>
              </thead>
              <tbody>
                {runs.map((run: TestRunSummary, i: number) => (
                  <motion.tr
                    key={run.runId}
                    initial={{ opacity: 0, y: 4 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: i * 0.02 }}
                    className="border-b hover:bg-accent/50 cursor-pointer transition-colors"
                    onClick={() => navigate(`/runs/${run.runId}`)}
                  >
                    <td className="px-3 py-1.5">
                      <RunStatusBadge status={run.status} />
                    </td>
                    <td className="px-3 py-1.5 font-medium">{run.testSuiteName || run.runId}</td>
                    <td className="px-3 py-1.5 text-[12px] text-muted-foreground">
                      {relativeTime(run.createdAt)}
                    </td>
                    <td className="px-3 py-1.5 text-[12px] font-mono text-muted-foreground">
                      {formatDuration(run.createdAt, run.completedAt)}
                    </td>
                    <td className="px-3 py-1.5 text-right">
                      <ArrowRight className="h-3.5 w-3.5 text-muted-foreground inline-block" />
                    </td>
                  </motion.tr>
                ))}
              </tbody>
            </table>

            {nextToken && (
              <div className="flex justify-center py-3">
                <Button
                  variant="ghost"
                  size="sm"
                  className="text-[12px]"
                  onClick={() => loadRuns(20, nextToken)}
                  disabled={loading}
                >
                  {loading ? <Loader2 className="h-3 w-3 animate-spin mr-1" /> : null}
                  Load more
                </Button>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}
