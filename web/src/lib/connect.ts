import { createClient, type Interceptor } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { AccountAdminService } from "@/lib/proto/cloud/v1/api/admin/account_pb.ts";
import { PlatformAdminService } from "@/lib/proto/cloud/v1/api/admin/platform_pb.ts";
import { TenantAdminService } from "@/lib/proto/cloud/v1/api/admin/tenant_pb.ts";
import { ApiTokenService } from "@/lib/proto/cloud/v1/api/ui/apitoken_pb.ts";
import { AuthoringService } from "@/lib/proto/cloud/v1/api/ui/authoring_pb.ts";
import { AuthService } from "@/lib/proto/cloud/v1/api/ui/auth_pb.ts";
import { CloudInventoryService } from "@/lib/proto/cloud/v1/api/ui/inventory_pb.ts";
import { PackageService } from "@/lib/proto/cloud/v1/api/ui/package_pb.ts";
import { PresetService } from "@/lib/proto/cloud/v1/api/ui/preset_pb.ts";
import { RunService } from "@/lib/proto/cloud/v1/api/ui/run_pb.ts";
import { SettingsService } from "@/lib/proto/cloud/v1/api/ui/settings_pb.ts";
import { SuiteService } from "@/lib/proto/cloud/v1/api/ui/suite_pb.ts";
import { TenantService } from "@/lib/proto/cloud/v1/api/ui/tenant_pb.ts";
import { WebhookService } from "@/lib/proto/cloud/v1/api/ui/webhook_pb.ts";

let accessToken: string | null = null;

export function setAccessToken(token: string | null) {
  accessToken = token;
}

const authInterceptor: Interceptor = (next) => async (request) => {
  if (accessToken) {
    request.header.set("Authorization", `Bearer ${accessToken}`);
  }
  return next(request);
};

const transport = createConnectTransport({
  baseUrl: "",
  interceptors: [authInterceptor],
});

export const api = {
  accountAdmin: createClient(AccountAdminService, transport),
  apiToken: createClient(ApiTokenService, transport),
  authoring: createClient(AuthoringService, transport),
  auth: createClient(AuthService, transport),
  inventory: createClient(CloudInventoryService, transport),
  package: createClient(PackageService, transport),
  platformAdmin: createClient(PlatformAdminService, transport),
  preset: createClient(PresetService, transport),
  run: createClient(RunService, transport),
  settings: createClient(SettingsService, transport),
  suite: createClient(SuiteService, transport),
  tenant: createClient(TenantService, transport),
  tenantAdmin: createClient(TenantAdminService, transport),
  webhook: createClient(WebhookService, transport),
};
