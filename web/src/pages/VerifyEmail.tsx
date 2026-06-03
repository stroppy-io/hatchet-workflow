import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { Activity, CheckCircle2, XCircle } from "lucide-react";
import { iamClient } from "@/services/client";
import { ConnectError } from "@connectrpc/connect";

type Status = "pending" | "ok" | "error" | "missing";

// Consumes the emailed ?token= and flips Account.email_verified server-side.
// PUBLIC — the token is the credential, the user may not be logged in.
export function VerifyEmail() {
  const [params] = useSearchParams();
  const token = params.get("token") || "";

  const [status, setStatus] = useState<Status>(token ? "pending" : "missing");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    (async () => {
      try {
        await iamClient.verifyEmail({ token });
        if (!cancelled) setStatus("ok");
      } catch (err) {
        if (cancelled) return;
        const msg =
          err instanceof ConnectError
            ? err.rawMessage
            : err instanceof Error
              ? err.message
              : "Verification failed";
        setError(msg);
        setStatus("error");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [token]);

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
          Verify email
        </div>

        {status === "pending" && (
          <p className="mt-4 text-sm text-muted-foreground">Verifying…</p>
        )}

        {status === "missing" && (
          <p className="mt-4 text-sm text-red-400">
            Missing verification token.
          </p>
        )}

        {status === "ok" && (
          <div className="mt-4 flex items-center gap-2 text-sm text-foreground">
            <CheckCircle2 className="h-4 w-4 text-success" />
            Your email has been verified.
          </div>
        )}

        {status === "error" && (
          <div className="mt-4 flex items-center gap-2 text-sm text-red-400">
            <XCircle className="h-4 w-4" />
            {error || "Verification failed."}
          </div>
        )}

        <Link
          to="/login"
          className="mt-6 block text-center text-xs text-primary hover:underline"
        >
          Continue to sign in
        </Link>
      </div>
    </div>
  );
}
