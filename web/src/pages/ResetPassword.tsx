import { useState, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { Activity } from "lucide-react";
import { iamClient } from "@/services/client";
import { ConnectError } from "@connectrpc/connect";

// Completes the forgot-password flow: consumes the single-use ?token= from the
// emailed link plus a new password. On success returns the user to /login.
export function ResetPassword() {
  const [params] = useSearchParams();
  const token = params.get("token") || "";
  const navigate = useNavigate();

  const [newPassword, setNewPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (busy) return;
    setError(null);
    setBusy(true);
    try {
      await iamClient.confirmPasswordReset({ token, newPassword });
      navigate("/login", { replace: true });
    } catch (err) {
      const msg =
        err instanceof ConnectError
          ? err.rawMessage
          : err instanceof Error
            ? err.message
            : "Reset failed";
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
          Set new password
        </div>

        {!token ? (
          <>
            <p className="mt-4 text-sm text-red-400">
              Missing or invalid reset token. Request a new reset link.
            </p>
            <Link
              to="/forgot-password"
              className="mt-5 block text-center text-xs text-primary hover:underline"
            >
              Request reset link
            </Link>
          </>
        ) : (
          <form onSubmit={onSubmit}>
            <label className="mt-4 block text-xs font-medium text-muted-foreground">
              New password
              <input
                autoFocus
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                autoComplete="new-password"
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
              disabled={busy || !newPassword}
              className="mt-5 w-full bg-primary px-3 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
            >
              {busy ? "Saving…" : "Set password"}
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
