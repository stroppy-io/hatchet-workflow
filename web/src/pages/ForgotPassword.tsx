import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { Activity } from "lucide-react";
import { iamClient } from "@/services/client";
import { ConnectError } from "@connectrpc/connect";

// Public forgot-password entry point. RequestPasswordReset always succeeds
// regardless of whether the email exists (no account-existence leak), so the
// confirmation copy is deliberately neutral.
export function ForgotPassword() {
  const [email, setEmail] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [sent, setSent] = useState(false);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (busy) return;
    setError(null);
    setBusy(true);
    try {
      await iamClient.requestPasswordReset({ email: email.trim() });
      setSent(true);
    } catch (err) {
      const msg =
        err instanceof ConnectError
          ? err.rawMessage
          : err instanceof Error
            ? err.message
            : "Request failed";
      setError(msg);
    } finally {
      setBusy(false);
    }
  }

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
          Reset password
        </div>

        {sent ? (
          <>
            <p className="mt-4 text-sm text-muted-foreground">
              If an account exists for that address, a password reset link has
              been sent. Check your inbox.
            </p>
            <Link
              to="/login"
              className="mt-5 block text-center text-xs text-primary hover:underline"
            >
              Back to sign in
            </Link>
          </>
        ) : (
          <form onSubmit={onSubmit}>
            <label className="mt-4 block text-xs font-medium text-muted-foreground">
              Email
              <input
                autoFocus
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                autoComplete="email"
                className="mt-1 w-full border border-border bg-background px-3 py-2 text-sm text-foreground outline-none focus:border-primary"
              />
            </label>

            {error && (
              <p className="mt-3 text-xs text-red-400" role="alert">
                {error}
              </p>
            )}

            <button
              type="submit"
              disabled={busy || !email}
              className="mt-5 w-full bg-primary px-3 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
            >
              {busy ? "Sending…" : "Send reset link"}
            </button>

            <Link
              to="/login"
              className="mt-4 block text-center text-xs text-muted-foreground hover:underline"
            >
              Back to sign in
            </Link>
          </form>
        )}
      </div>
    </div>
  );
}
