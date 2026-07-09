import {
  createClient,
  Code,
  ConnectError,
  type Interceptor,
} from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { IamService } from "@/lib/proto/cloud/v1/api/iam_pb";
import { QuotaService } from "@/lib/proto/cloud/v1/api/quota_pb";
import { StroppyService } from "@/lib/proto/cloud/v1/api/stroppy_pb";
import { TestRunOverviewService } from "@/lib/proto/cloud/v1/api/test_run_overview_pb";
import { TenantSettingsService } from "@/lib/proto/cloud/v1/api/tenant_settings_pb";
import { TenantDashboardService } from "@/lib/proto/cloud/v1/api/tenant_dashboard_pb";
import { SystemSettingsService } from "@/lib/proto/cloud/v1/api/system_settings_pb";
import { PackageService } from "@/lib/proto/cloud/v1/api/package_pb";
import { FavoriteService } from "@/lib/proto/cloud/v1/api/favorite_pb";
import { CompareService } from "@/lib/proto/cloud/v1/api/compare_pb";
import { ShareService } from "@/lib/proto/cloud/v1/api/share_pb";
import { PublicShareService } from "@/lib/proto/cloud/v1/api/public_share_pb";
import { RatingService } from "@/lib/proto/cloud/v1/api/rating_pb";
import { PublicRatingService } from "@/lib/proto/cloud/v1/api/public_rating_pb";
import { AgentShellService } from "@/lib/proto/cloud/v1/api/agent_shell_pb";
import { RecipeService } from "@/lib/proto/cloud/v1/api/recipe_pb";
import { DslService } from "@/lib/proto/cloud/v1/dsl/service_pb";
import { CatalogService } from "@/lib/proto/cloud/v1/catalog/service_pb";

let accessToken: string | null = null;

export function setAccessToken(token: string | null): void {
  accessToken = token;
}

export function getAccessToken(): string | null {
  return accessToken;
}

const authInterceptor: Interceptor = (next) => async (req) => {
  if (accessToken) {
    req.header.set("Authorization", `Bearer ${accessToken}`);
  }
  return next(req);
};

const baseUrl = (import.meta.env.VITE_API_BASE_URL as string | undefined) || "/";

// Refresh-token storage (localStorage). Read directly here to avoid an import
// cycle with services/tokens.ts; the key MUST match REFRESH_KEY there.
const REFRESH_KEY = "stroppy.refreshToken";

function readRefreshToken(): string | null {
  try {
    return localStorage.getItem(REFRESH_KEY);
  } catch {
    return null;
  }
}

function writeRefreshToken(token: string | null): void {
  try {
    if (token) localStorage.setItem(REFRESH_KEY, token);
    else localStorage.removeItem(REFRESH_KEY);
  } catch {
    /* storage unavailable (private mode) — in-memory access token still works */
  }
}

// Bare transport for the Refresh call itself — NO auth/retry wrapping, so a
// failed refresh can never recurse back into the retry interceptor below.
const refreshTransport = createConnectTransport({ baseUrl });
const refreshClient = createClient(IamService, refreshTransport);

// Single-flight: many concurrent calls that all 401 collapse onto ONE Refresh,
// then each replays. Prevents a refresh-token rotation stampede (the token is
// single-use — parallel refreshes would invalidate each other).
let refreshInFlight: Promise<boolean> | null = null;

function refreshOnce(): Promise<boolean> {
  if (!refreshInFlight) {
    refreshInFlight = (async () => {
      const rt = readRefreshToken();
      if (!rt) return false;
      try {
        const { tokens } = await refreshClient.refresh({ refreshToken: rt });
        if (tokens?.accessToken) setAccessToken(tokens.accessToken);
        if (tokens?.refreshToken) writeRefreshToken(tokens.refreshToken);
        return true;
      } catch {
        // Refresh token expired/revoked — only now must the user re-login.
        setAccessToken(null);
        writeRefreshToken(null);
        return false;
      }
    })().finally(() => {
      refreshInFlight = null;
    });
  }
  return refreshInFlight;
}

// On Unauthenticated, rotate the access token once and replay the request. Sits
// BEFORE authInterceptor so the replay re-runs auth and attaches the FRESH
// bearer. This keeps the user signed in for the entire refresh-token lifetime
// with no reload — access-token expiry becomes invisible.
const refreshRetryInterceptor: Interceptor = (next) => async (req) => {
  try {
    return await next(req);
  } catch (err) {
    const unauthenticated =
      err instanceof ConnectError && err.code === Code.Unauthenticated;
    // Skip streams and the case where we have nothing to refresh with.
    if (!unauthenticated || req.stream || !readRefreshToken()) throw err;
    if (!(await refreshOnce())) throw err;
    return next(req);
  }
};

export const transport = createConnectTransport({
  baseUrl,
  // Order matters: refresh-retry wraps auth, so a replay re-attaches the token.
  interceptors: [refreshRetryInterceptor, authInterceptor],
});

export const iamClient = createClient(IamService, transport);
export const testRunOverviewClient = createClient(TestRunOverviewService, transport);
export const stroppyClient = createClient(StroppyService, transport);
export const quotaClient = createClient(QuotaService, transport);
export const tenantSettingsClient = createClient(TenantSettingsService, transport);
export const tenantDashboardClient = createClient(TenantDashboardService, transport);
export const systemSettingsClient = createClient(SystemSettingsService, transport);
export const packageClient = createClient(PackageService, transport);
export const favoriteClient = createClient(FavoriteService, transport);
export const compareClient = createClient(CompareService, transport);
export const shareClient = createClient(ShareService, transport);
export const publicShareClient = createClient(PublicShareService, transport);
export const ratingClient = createClient(RatingService, transport);
export const publicRatingClient = createClient(PublicRatingService, transport);
export const agentShellClient = createClient(AgentShellService, transport);
export const recipeClient = createClient(RecipeService, transport);
export const dslClient = createClient(DslService, transport);
export const catalogClient = createClient(CatalogService, transport);
