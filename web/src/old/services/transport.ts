// Connect transport + auth wiring.
//
// The backend no longer serves the old REST /api/v1 surface. Every service is
// a connectrpc service mounted on the SAME origin under
// /cloud.v1.api.<Service>/<Method>. This module builds a single shared
// Connect transport and the typed clients the app talks to.
//
// Auth model (unchanged from the old REST client):
//   - The access token lives in module memory only (never localStorage).
//   - It is attached as `Authorization: Bearer <token>` by an interceptor.
//   - The refresh token also lives in memory; on a 401 (Code.Unauthenticated)
//     we run a single in-flight Refresh RPC, swap in the new pair and retry the
//     original call once. A failed refresh clears the session and surfaces a
//     SessionExpiredError so the AuthContext can redirect to login.

import {
  createClient,
  Code,
  ConnectError,
  type Interceptor,
  type Transport,
} from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { IamService } from "@/lib/proto/cloud/v1/api/iam_pb";
import { SystemSettingsService } from "@/lib/proto/cloud/v1/api/system_settings_pb";
import { TenantSettingsService } from "@/lib/proto/cloud/v1/api/tenant_settings_pb";
import { TenantDashboardService } from "@/lib/proto/cloud/v1/api/tenant_dashboard_pb";
import {
  DatabasePresetService,
  WorkloadPresetService,
  TestPresetService,
} from "@/lib/proto/cloud/v1/api/preset_pb";
import { PackageService } from "@/lib/proto/cloud/v1/api/package_pb";
import { TestWizardService } from "@/lib/proto/cloud/v1/api/test_wizard_pb";
import { TestRunService } from "@/lib/proto/cloud/v1/api/test_run_pb";
import { TestRunOverviewService } from "@/lib/proto/cloud/v1/api/test_run_overview_pb";
import { SuiteService } from "@/lib/proto/cloud/v1/api/suite_pb";
import { SuiteRunService } from "@/lib/proto/cloud/v1/api/suite_run_pb";
import { SuiteWizardService } from "@/lib/proto/cloud/v1/api/suite_wizard_pb";
import { ShareService } from "@/lib/proto/cloud/v1/api/share_pb";
import { PublicShareService } from "@/lib/proto/cloud/v1/api/public_share_pb";
import { RatingService } from "@/lib/proto/cloud/v1/api/rating_pb";
import { PublicRatingService } from "@/lib/proto/cloud/v1/api/public_rating_pb";
import { FavoriteService } from "@/lib/proto/cloud/v1/api/favorite_pb";
import { CompareService } from "@/lib/proto/cloud/v1/api/compare_pb";

// Thrown when auth refresh fails — UI should redirect to login.
export class SessionExpiredError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "SessionExpiredError";
  }
}

// ---- In-memory token store ----

let _accessToken: string | null = null;
let _refreshToken: string | null = null;

export function setAccessToken(token: string | null) {
  _accessToken = token;
}

export function getAccessToken(): string | null {
  return _accessToken;
}

export function setRefreshToken(token: string | null) {
  _refreshToken = token;
}

export function getRefreshToken(): string | null {
  return _refreshToken;
}

// Set both tokens at once (e.g. from a TokenPair returned by login/refresh).
export function setTokens(access: string | null, refresh: string | null) {
  _accessToken = access;
  _refreshToken = refresh;
}

export function clearTokens() {
  _accessToken = null;
  _refreshToken = null;
}

// ---- Refresh coordination ----
//
// A bare transport with NO auth interceptor — used only by the refresh RPC so
// a refresh can never recurse into another refresh.
const refreshTransport: Transport = createConnectTransport({
  baseUrl: "/",
});
const refreshClient = createClient(IamService, refreshTransport);

let _refreshPromise: Promise<void> | null = null;

// runRefresh exchanges the stored refresh token for a fresh TokenPair exactly
// once even under concurrent callers. Resolves when tokens are swapped in;
// rejects (and clears the session) when there is no token or the server denies.
export function runRefresh(): Promise<void> {
  if (_refreshPromise) return _refreshPromise;
  _refreshPromise = (async () => {
    if (!_refreshToken) {
      throw new SessionExpiredError("no refresh token");
    }
    try {
      const resp = await refreshClient.refresh({ refreshToken: _refreshToken });
      const tokens = resp.tokens;
      if (!tokens?.accessToken) {
        throw new SessionExpiredError("refresh returned no token");
      }
      _accessToken = tokens.accessToken;
      // refresh tokens are single-use + rotated: persist the new one.
      _refreshToken = tokens.refreshToken || _refreshToken;
    } catch (e) {
      clearTokens();
      if (e instanceof SessionExpiredError) throw e;
      throw new SessionExpiredError("session expired");
    }
  })();
  try {
    return _refreshPromise;
  } finally {
    // Reset so the next 401 starts a new refresh cycle.
    _refreshPromise = null;
  }
}

// ---- Auth interceptor ----
//
// Attaches the bearer token and transparently refreshes once on 401. Connect's
// interceptor signature wraps the whole call, so we can re-issue it after a
// successful refresh — matching the old REST client's single silent retry.
const authInterceptor: Interceptor = (next) => async (req) => {
  if (_accessToken) {
    req.header.set("Authorization", `Bearer ${_accessToken}`);
  }
  try {
    return await next(req);
  } catch (e) {
    const isAuth =
      e instanceof ConnectError && e.code === Code.Unauthenticated;
    if (!isAuth || !_accessToken) throw e;
    // One silent refresh + retry, mirroring the old fetch client.
    await runRefresh();
    req.header.set("Authorization", `Bearer ${_accessToken}`);
    return await next(req);
  }
};

// Single shared transport for every authenticated client. Same origin, HTTP/2
// cleartext is handled by the browser; connect-web negotiates the Connect
// protocol over standard fetch.
export const transport: Transport = createConnectTransport({
  baseUrl: "/",
  interceptors: [authInterceptor],
});

// ---- Typed clients ----

export const iamClient = createClient(IamService, transport);
export const systemSettingsClient = createClient(SystemSettingsService, transport);
export const tenantSettingsClient = createClient(TenantSettingsService, transport);
export const tenantDashboardClient = createClient(TenantDashboardService, transport);
export const databasePresetClient = createClient(DatabasePresetService, transport);
export const workloadPresetClient = createClient(WorkloadPresetService, transport);
export const testPresetClient = createClient(TestPresetService, transport);
export const packageClient = createClient(PackageService, transport);
export const testWizardClient = createClient(TestWizardService, transport);
export const testRunClient = createClient(TestRunService, transport);
export const testRunOverviewClient = createClient(TestRunOverviewService, transport);
export const suiteClient = createClient(SuiteService, transport);
export const suiteRunClient = createClient(SuiteRunService, transport);
export const suiteWizardClient = createClient(SuiteWizardService, transport);
export const shareClient = createClient(ShareService, transport);
export const publicShareClient = createClient(PublicShareService, transport);
export const ratingClient = createClient(RatingService, transport);
export const publicRatingClient = createClient(PublicRatingService, transport);
export const favoriteClient = createClient(FavoriteService, transport);
export const compareClient = createClient(CompareService, transport);
