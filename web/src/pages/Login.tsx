import { useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { Activity } from "lucide-react";
import { useAuth } from "@/hooks/useAuth";
import { ConnectError } from "@connectrpc/connect";
import { iamClient } from "@/services/client";
import type { SsoButton } from "@/lib/proto/cloud/v1/api/iam_pb";
import { SSO_PROVIDER_KEY } from "@/pages/SSOCallback";
import { getPublicConfig } from "@/services/publicConfig";

// Real sign-in screen. Authenticates via IamService.Login (login = nickname OR
// email + password), then lands on the requested redirect or the root, which
// resolves to the user's first org dashboard.
export function Login() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const redirect = params.get("redirect") || "/";

  const [loginId, setLoginId] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [ssoButtons, setSsoButtons] = useState<SsoButton[]>([]);
  // null = unknown yet; default to allowing the link until the instance says no.
  const [allowRegister, setAllowRegister] = useState(true);

  // ListIdentityProviders is PUBLIC — populate the SSO buttons up front.
  useEffect(() => {
    let cancelled = false;
    iamClient
      .listIdentityProviders({})
      .then((res) => {
        if (!cancelled) setSsoButtons(res.buttons);
      })
      .catch(() => {
        /* no providers configured / endpoint unavailable — hide SSO */
      });
    // Public instance config — hide self-signup when the admin disabled it.
    getPublicConfig()
      .then((cfg) => {
        if (!cancelled) setAllowRegister(cfg.allowSelfRegistration);
      })
      .catch(() => {
        /* config unavailable — leave the link visible, Register gates again */
      });
    return () => {
      cancelled = true;
    };
  }, []);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (busy) return;
    setError(null);
    setBusy(true);
    try {
      await login(loginId.trim(), password);
      navigate(redirect, { replace: true });
    } catch (err) {
      const msg =
        err instanceof ConnectError
          ? err.rawMessage
          : err instanceof Error
            ? err.message
            : "Sign in failed";
      setError(msg);
    } finally {
      setBusy(false);
    }
  }

  // StartSSO returns the IdP authorize URL; stash the provider id (the callback
  // only carries code + state) and send the browser to the IdP.
  async function startSSO(providerId: string) {
    setError(null);
    setBusy(true);
    try {
      const { redirectUrl } = await iamClient.startSSO({ providerId });
      try {
        sessionStorage.setItem(SSO_PROVIDER_KEY, providerId);
      } catch {
        /* storage unavailable */
      }
      window.location.assign(redirectUrl);
    } catch (err) {
      const msg =
        err instanceof ConnectError
          ? err.rawMessage
          : err instanceof Error
            ? err.message
            : "SSO sign-in failed";
      setError(msg);
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
          Sign in
        </div>

        <label className="mt-4 block text-xs font-medium text-muted-foreground">
          Email or nickname
          <input
            autoFocus
            value={loginId}
            onChange={(e) => setLoginId(e.target.value)}
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
            autoComplete="current-password"
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
          disabled={busy || !loginId || !password}
          className="mt-5 w-full bg-primary px-3 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
        >
          {busy ? "Signing in…" : "Sign in"}
        </button>

        <div className="mt-3 flex items-center justify-between text-xs">
          {allowRegister ? (
            <Link to="/register" className="text-primary hover:underline">
              Create account
            </Link>
          ) : (
            <span />
          )}
          <Link
            to="/forgot-password"
            className="text-muted-foreground hover:underline"
          >
            Forgot password?
          </Link>
        </div>

        {ssoButtons.length > 0 && (
          <div className="mt-5 space-y-2 border-t border-border pt-4">
            <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">
              Single sign-on
            </div>
            {ssoButtons.map((button) => (
              <button
                key={button.id}
                type="button"
                disabled={busy}
                onClick={() => void startSSO(button.id)}
                className="w-full border border-border bg-background px-3 py-2 text-sm text-foreground hover:border-primary disabled:opacity-50"
              >
                Sign in with {button.displayName || button.slug}
              </button>
            ))}
          </div>
        )}
      </form>
    </div>
  );
}
