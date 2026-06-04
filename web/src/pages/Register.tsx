import { useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Activity } from "lucide-react";
import { useAuth } from "@/hooks/useAuth";
import { ConnectError } from "@connectrpc/connect";
import { getPublicConfig } from "@/services/publicConfig";
import { submitRegistrationRequest } from "@/services/registration";

// Public self-signup. Register auto-logs-in (RegisterResponse carries a
// TokenPair), so on success we land on the root which resolves to the user's
// first org dashboard. The email starts UNVERIFIED — surfaced later on /profile.
//
// When self-registration is DISABLED, the form swaps to an access-request form:
// the visitor leaves an email + optional note that platform admins triage from
// the admin console (SubmitRegistrationRequest).
export function Register() {
  const { register } = useAuth();
  const navigate = useNavigate();

  const [email, setEmail] = useState("");
  const [nickname, setNickname] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  // null = still loading the instance config; true/false once known.
  const [allowed, setAllowed] = useState<boolean | null>(null);

  // Access-request (closed-signup) state.
  const [reqMessage, setReqMessage] = useState("");
  const [reqSent, setReqSent] = useState(false);

  useEffect(() => {
    let cancelled = false;
    getPublicConfig()
      .then((cfg) => {
        if (!cancelled) setAllowed(cfg.allowSelfRegistration);
      })
      // Config unreachable — let the form through; the server still gates Register.
      .catch(() => {
        if (!cancelled) setAllowed(true);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (busy || allowed === false) return;
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
      setError(toMessage(err, "Sign up failed"));
    } finally {
      setBusy(false);
    }
  }

  async function onRequestSubmit(e: FormEvent) {
    e.preventDefault();
    if (busy) return;
    setError(null);
    setBusy(true);
    try {
      await submitRegistrationRequest(email.trim(), reqMessage.trim());
      setReqSent(true);
    } catch (err) {
      setError(toMessage(err, "Could not submit request"));
    } finally {
      setBusy(false);
    }
  }

  const closed = allowed === false;

  return (
    <div className="flex h-screen items-center justify-center bg-background">
      <form
        onSubmit={closed ? onRequestSubmit : onSubmit}
        className="w-full max-w-sm border border-border bg-card p-8"
      >
        <div className="mb-6 flex items-center gap-2">
          <Activity className="h-5 w-5 text-primary" />
          <span className="text-base font-semibold tracking-tight">
            stroppy-cloud
          </span>
        </div>
        <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">
          {closed ? "Request access" : "Create account"}
        </div>

        {/* --- Closed signup: request-access form --- */}
        {closed && reqSent && (
          <div className="mt-4 border border-border bg-background p-3 text-xs text-muted-foreground">
            Thanks — your request was received. A platform administrator will
            review it and set up your account.
            <div className="mt-3">
              <Link to="/login" className="text-primary hover:underline">
                Back to sign in
              </Link>
            </div>
          </div>
        )}

        {closed && !reqSent && (
          <>
            <p className="mt-4 text-xs text-muted-foreground">
              Open registration is disabled on this instance. Leave your email
              and a short note — an administrator will set up your account.
            </p>

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
              Message <span className="text-zinc-600">(optional)</span>
              <textarea
                value={reqMessage}
                onChange={(e) => setReqMessage(e.target.value)}
                rows={3}
                className="mt-1 w-full resize-none border border-border bg-background px-3 py-2 text-sm text-foreground outline-none focus:border-primary"
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
              {busy ? "Sending…" : "Request access"}
            </button>

            <div className="mt-4 text-center text-xs text-muted-foreground">
              Already have an account?{" "}
              <Link to="/login" className="text-primary hover:underline">
                Sign in
              </Link>
            </div>
          </>
        )}

        {/* --- Open signup: standard self-registration --- */}
        {!closed && (
          <fieldset disabled={allowed === null} className="contents">
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
          </fieldset>
        )}
      </form>
    </div>
  );
}

function toMessage(err: unknown, fallback: string): string {
  return err instanceof ConnectError
    ? err.rawMessage
    : err instanceof Error
      ? err.message
      : fallback;
}
