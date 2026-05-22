import { useState, type FormEvent } from "react";
import { Navigate, useLocation, useNavigate } from "react-router-dom";

import { useAuth } from "@/contexts/auth-context";

type LocationState = {
  next?: string;
};

export function LoginPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const { account, login, tenants } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  if (account) {
    const next = (location.state as LocationState | null)?.next;
    const fallbackTenant = tenants[0]?.entity?.id?.value ?? "default";
    return <Navigate to={next || `/t/${fallbackTenant}/wizard`} replace />;
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setSubmitting(true);
    try {
      await login(email, password);
      const next = (location.state as LocationState | null)?.next;
      navigate(next || "/t/default/wizard", { replace: true });
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Login failed");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="grid min-h-screen place-items-center bg-background p-6 text-foreground">
      <form className="w-full max-w-sm border bg-card p-6" onSubmit={submit}>
        <h1 className="text-lg font-semibold">Sign in</h1>
        <p className="mt-2 text-sm text-muted-foreground">Use your account credentials to open tenant-scoped workspaces.</p>

        <label className="mt-6 block text-xs text-muted-foreground" htmlFor="email">
          Email or username
        </label>
        <input
          id="email"
          className="mt-1 w-full border bg-background px-3 py-2 text-sm"
          value={email}
          autoComplete="username"
          onChange={(event) => setEmail(event.target.value)}
        />

        <label className="mt-4 block text-xs text-muted-foreground" htmlFor="password">
          Password
        </label>
        <input
          id="password"
          className="mt-1 w-full border bg-background px-3 py-2 text-sm"
          type="password"
          value={password}
          autoComplete="current-password"
          onChange={(event) => setPassword(event.target.value)}
        />

        {error ? <div className="mt-4 border border-danger/50 bg-danger/10 px-3 py-2 text-sm text-danger">{error}</div> : null}

        <button
          className="mt-5 w-full bg-primary px-3 py-2 text-sm font-medium text-primary-foreground disabled:opacity-60"
          disabled={submitting || !email || !password}
          type="submit"
        >
          {submitting ? "Signing in..." : "Sign in"}
        </button>
      </form>
    </div>
  );
}
