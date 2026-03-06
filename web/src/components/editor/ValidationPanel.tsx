import { useStore } from "@nanostores/react"
import { AlertCircle, AlertTriangle, CheckCircle } from "lucide-react"
import { $validationErrors, $selectedNodeId } from "@/stores/editor"

export function ValidationPanel() {
  const errors = useStore($validationErrors)

  if (errors.length === 0) {
    return (
      <div className="flex items-center gap-2 p-3 text-[12px] text-chart-2">
        <CheckCircle className="h-3.5 w-3.5" />
        <span>No validation errors</span>
      </div>
    )
  }

  return (
    <div className="h-full overflow-auto">
      {errors.map((err, i) => {
        const isError = err.severity === "error"
        const Icon = isError ? AlertCircle : AlertTriangle

        return (
          <button
            key={i}
            type="button"
            className="flex items-start gap-2 w-full px-3 py-1.5 text-left hover:bg-accent/50 transition-colors"
            onClick={() => {
              const match = err.fieldPath.match(/nodes\[(\d+)\]/)
              if (match) {
                // Could set selected node here if we had the node name
                $selectedNodeId.set(null)
              }
            }}
          >
            <Icon
              className={`h-3.5 w-3.5 mt-0.5 shrink-0 ${
                isError ? "text-destructive" : "text-chart-3"
              }`}
            />
            <div className="min-w-0">
              {err.fieldPath && (
                <span className="text-[11px] font-mono text-muted-foreground">
                  {err.fieldPath}
                  {" "}
                </span>
              )}
              <span className="text-[12px]">{err.message}</span>
            </div>
          </button>
        )
      })}
    </div>
  )
}
