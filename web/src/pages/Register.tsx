import { useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Activity } from "lucide-react";
import { useAuth } from "@/hooks/useAuth";
import { ConnectError } from "@connectrpc/connect";

// Public self-signup. Register auto-logs-in (RegisterResponse carries a
// TokenPair), so on success we land on the root which resolves to the user's
// first org dashboard. The email starts UNVERIFIED — surfaced later on /profile.
export function Register() {
  const { register } = useAuth();
  const navigate = useNavigate();

  const [email, setEmail] = useState("");
  const [nickname, setNickname] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (busy) return;
    setError(null);
    setBusy(true);
    try {
      await register({
        email: email.trim(),
        nickname: nickname.trim(),
        password,
      });
      // Auto-logged-in: go to root. AuthContext already holds the session.
      navigate("/", { replace: true });
    } catch (err) {
      const msg =
        err instanceof ConnectError
          ? err.rawMessage
          : err instanceof Error
            ? err.message
            : "Sign up failed";
      setError(msg);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex h-screen items-center justify-center bg-background">
      <form
        onSubmit={onSubmit}
        className="w-full max-w-sm border border-border bg-card p-8"
      >
        <div className="mb-6 flex items-center gap-2">
          <Activity className="h-5 w-5 text-primary" />
          <span className="text-base font-semibold tracking-tight">
            stroppy-cloud
          </span>
        </div>
        <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">
          Create account
        </div>

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

        <label className="mt-3 block text-xs font-medium text-muted-foreground">
          Nickname
          <input
            value={nickname}
            onChange={(e) => setNickname(e.target.value)}
            autoComplete="username"
            className="mt-1 w-full border border-border bg-background px-3 py-2 text-sm text-foreground outline-none focus:border-primary"
          />
        </label>

        <label className="mt-3 block text-xs font-medium text-muted-foreground">
          Password
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
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
          disabled={busy || !email || !nickname || !password}
          className="mt-5 w-full bg-primary px-3 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
        >
          {busy ? "Creating…" : "Create account"}
        </button>

        <div className="mt-4 text-center text-xs text-muted-foreground">
          Already have an account?{" "}
          <Link to="/login" className="text-primary hover:underline">
            Sign in
          </Link>
        </div>
      </form>
    </div>
  );
}
