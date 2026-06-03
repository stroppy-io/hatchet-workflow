import { useEffect, useRef, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { Activity, XCircle } from "lucide-react";
import { useAuth } from "@/hooks/useAuth";
import { ConnectError } from "@connectrpc/connect";

// sessionStorage key the login page writes the chosen provider id under before
// redirecting to the IdP. The OIDC callback only carries code + state, so the
// provider id (which CompleteSSO keys on) is recovered from here.
export const SSO_PROVIDER_KEY = "stroppy.ssoProviderId";

type Status = "pending" | "error";

// OIDC authorization-code callback target. Reads ?code=&state= from the IdP
// redirect, exchanges them (plus the stashed provider id) for our own TokenPair
// via CompleteSSO, mints the session through AuthContext, then lands on root.
export function SSOCallback() {
  const [params] = useSearchParams();
  const code = params.get("code") || "";
  const state = params.get("state") || "";
  const { completeSSO } = useAuth();
  const navigate = useNavigate();

  const [status, setStatus] = useState<Status>("pending");
  const [error, setError] = useState<string | null>(null);
  const ran = useRef(false);

  useEffect(() => {
    if (ran.current) return;
    ran.current = true;

    const providerId = (() => {
      try {
        return sessionStorage.getItem(SSO_PROVIDER_KEY) || "";
      } catch {
        return "";
      }
    })();

    if (!code || !state) {
      setError("Missing code or state in callback.");
      setStatus("error");
      return;
    }

    (async () => {
      try {
        await completeSSO(providerId, code, state);
        try {
          sessionStorage.removeItem(SSO_PROVIDER_KEY);
        } catch {
          /* storage unavailable */
        }
        navigate("/", { replace: true });
      } catch (err) {
        const msg =
          err instanceof ConnectError
            ? err.rawMessage
            : err instanceof Error
              ? err.message
              : "SSO sign-in failed";
        setError(msg);
        setStatus("error");
      }
    })();
  }, [code, state, completeSSO, navigate]);

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
          Single sign-on
        </div>

        {status === "pending" ? (
          <p className="mt-4 text-sm text-muted-foreground">
            Completing sign-in…
          </p>
        ) : (
          <>
            <div className="mt-4 flex items-center gap-2 text-sm text-red-400">
              <XCircle className="h-4 w-4" />
              {error || "SSO sign-in failed."}
            </div>
            <Link
              to="/login"
              className="mt-6 block text-center text-xs text-primary hover:underline"
            >
              Back to sign in
            </Link>
          </>
        )}
      </div>
    </div>
  );
}
