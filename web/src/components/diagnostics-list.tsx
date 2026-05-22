import type { Diagnostic } from "@/lib/proto/cloud/v1/api/ui/authoring_pb.ts";
import { Severity } from "@/lib/proto/cloud/v1/api/ui/authoring_pb.ts";

type DiagnosticsListProps = {
  diagnostics: Diagnostic[];
};

export function DiagnosticsList({ diagnostics }: DiagnosticsListProps) {
  if (diagnostics.length === 0) {
    return <div className="text-sm text-muted-foreground">No diagnostics.</div>;
  }

  return (
    <div className="space-y-2">
      {diagnostics.map((diagnostic, index) => (
        <div key={`${diagnostic.code}-${diagnostic.fieldPath}-${index}`} className="border p-3">
          <div className="flex items-center justify-between gap-3">
            <span className={["text-xs font-semibold", severityClass(diagnostic.severity)].join(" ")}>
              {severityLabel(diagnostic.severity)}
            </span>
            <span className="font-mono text-[11px] text-muted-foreground">{diagnostic.fieldPath || "preset"}</span>
          </div>
          <p className="mt-2 text-sm">{diagnostic.message}</p>
        </div>
      ))}
    </div>
  );
}

function severityLabel(severity: Severity) {
  switch (severity) {
    case Severity.ERROR:
      return "ERROR";
    case Severity.WARNING:
      return "WARNING";
    case Severity.INFO:
      return "INFO";
    default:
      return "UNKNOWN";
  }
}

function severityClass(severity: Severity) {
  switch (severity) {
    case Severity.ERROR:
      return "text-danger";
    case Severity.WARNING:
      return "text-warning";
    case Severity.INFO:
      return "text-primary";
    default:
      return "text-muted-foreground";
  }
}
