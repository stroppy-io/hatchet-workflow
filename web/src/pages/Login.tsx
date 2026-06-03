import { Activity } from "lucide-react";

// Placeholder login screen. The real auth flow is wired in a later task; the
// mock provider auto-authenticates, so this is only shown when no session
// resolves (e.g. real backend, not yet implemented).
export function Login() {
  return (
    <div className="flex h-screen items-center justify-center bg-background">
      <div className="w-full max-w-sm border border-border bg-card p-8">
        <div className="mb-6 flex items-center gap-2">
          <Activity className="h-5 w-5 text-primary" />
          <span className="text-base font-semibold tracking-tight">
            stroppy-cloud
          </span>
        </div>
        <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">
          Sign in
        </div>
        <p className="mt-2 text-sm text-muted-foreground">
          Authentication is wired in a later task. Run with{" "}
          <code className="font-mono text-foreground">VITE_MOCK=1</code> to
          preview the shell.
        </p>
      </div>
    </div>
  );
}
