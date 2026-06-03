import { createClient, type Interceptor } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { IamService } from "@/lib/proto/cloud/v1/api/iam_pb";
import { QuotaService } from "@/lib/proto/cloud/v1/api/quota_pb";
import { StroppyService } from "@/lib/proto/cloud/v1/api/stroppy_pb";
import { SuiteService } from "@/lib/proto/cloud/v1/api/suite_pb";
import { SuiteRunService } from "@/lib/proto/cloud/v1/api/suite_run_pb";
import { TestRunService } from "@/lib/proto/cloud/v1/api/test_run_pb";
import { TestRunOverviewService } from "@/lib/proto/cloud/v1/api/test_run_overview_pb";
import { TenantSettingsService } from "@/lib/proto/cloud/v1/api/tenant_settings_pb";
import { TenantDashboardService } from "@/lib/proto/cloud/v1/api/tenant_dashboard_pb";
import { SystemSettingsService } from "@/lib/proto/cloud/v1/api/system_settings_pb";
import { PackageService } from "@/lib/proto/cloud/v1/api/package_pb";
import { FavoriteService } from "@/lib/proto/cloud/v1/api/favorite_pb";
import {
  DatabasePresetService,
  WorkloadPresetService,
  TestPresetService,
} from "@/lib/proto/cloud/v1/api/preset_pb";
import { TestWizardService } from "@/lib/proto/cloud/v1/api/test_wizard_pb";
import { SuiteWizardService } from "@/lib/proto/cloud/v1/api/suite_wizard_pb";
import { CompareService } from "@/lib/proto/cloud/v1/api/compare_pb";
import { ShareService } from "@/lib/proto/cloud/v1/api/share_pb";
import { PublicShareService } from "@/lib/proto/cloud/v1/api/public_share_pb";
import { RatingService } from "@/lib/proto/cloud/v1/api/rating_pb";
import { PublicRatingService } from "@/lib/proto/cloud/v1/api/public_rating_pb";
import { AgentShellService } from "@/lib/proto/cloud/v1/api/agent_shell_pb";

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

export const transport = createConnectTransport({
  baseUrl,
  interceptors: [authInterceptor],
});

export const iamClient = createClient(IamService, transport);
export const testRunClient = createClient(TestRunService, transport);
export const testRunOverviewClient = createClient(TestRunOverviewService, transport);
export const suiteClient = createClient(SuiteService, transport);
export const suiteRunClient = createClient(SuiteRunService, transport);
export const stroppyClient = createClient(StroppyService, transport);
export const quotaClient = createClient(QuotaService, transport);
export const tenantSettingsClient = createClient(TenantSettingsService, transport);
export const tenantDashboardClient = createClient(TenantDashboardService, transport);
export const systemSettingsClient = createClient(SystemSettingsService, transport);
export const packageClient = createClient(PackageService, transport);
export const favoriteClient = createClient(FavoriteService, transport);
export const databasePresetClient = createClient(DatabasePresetService, transport);
export const workloadPresetClient = createClient(WorkloadPresetService, transport);
export const testPresetClient = createClient(TestPresetService, transport);
export const testWizardClient = createClient(TestWizardService, transport);
export const suiteWizardClient = createClient(SuiteWizardService, transport);
export const compareClient = createClient(CompareService, transport);
export const shareClient = createClient(ShareService, transport);
export const publicShareClient = createClient(PublicShareService, transport);
export const ratingClient = createClient(RatingService, transport);
export const publicRatingClient = createClient(PublicRatingService, transport);
export const agentShellClient = createClient(AgentShellService, transport);
