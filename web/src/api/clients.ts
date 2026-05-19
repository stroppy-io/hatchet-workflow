import { createClient } from "@connectrpc/connect";
import { transport } from "./transport";

import { AuthService } from "@/lib/proto/cloud/v1/iam/auth_pb";
import { UserService } from "@/lib/proto/cloud/v1/iam/user_pb";
import { TenantService } from "@/lib/proto/cloud/v1/iam/tenant_pb";
// Note: member service is named TenantMemberService in the proto
import { TenantMemberService } from "@/lib/proto/cloud/v1/iam/member_pb";
import { ApiTokenService } from "@/lib/proto/cloud/v1/iam/api_token_pb";
import { AgentAdminService } from "@/lib/proto/cloud/v1/agent/service_pb";
import { TestRunService } from "@/lib/proto/cloud/v1/testing/test_run_pb";
import { TestRunTemplateService } from "@/lib/proto/cloud/v1/testing/test_run_template_pb";
import { TestSuiteService } from "@/lib/proto/cloud/v1/testing/test_suite_pb";
import { TestSuiteRunService } from "@/lib/proto/cloud/v1/testing/test_suite_run_pb";
import { SharedTestRunService } from "@/lib/proto/cloud/v1/testing/shared_test_run_pb";
import { SharedSuiteRunService } from "@/lib/proto/cloud/v1/testing/shared_suite_run_pb";
import { ComparisonService } from "@/lib/proto/cloud/v1/testing/comparison_pb";
import { DatabasePresetService } from "@/lib/proto/cloud/v1/catalog/database_pb";
import { WorkloadPresetService } from "@/lib/proto/cloud/v1/catalog/workload_pb";
import { PackageService } from "@/lib/proto/cloud/v1/catalog/package_pb";
import { SettingsService } from "@/lib/proto/cloud/v1/catalog/settings_pb";
import { StroppyService } from "@/lib/proto/cloud/v1/stroppy/stroppy_pb";
import { WebhookService } from "@/lib/proto/cloud/v1/ops/webhook_pb";
import { QuotaService } from "@/lib/proto/cloud/v1/ops/quota_pb";
import { AdminService } from "@/lib/proto/cloud/v1/admin/admin_pb";
import { BinaryCacheAdminService } from "@/lib/proto/cloud/v1/admin/binary_cache_pb";

export const clients = {
  auth: createClient(AuthService, transport),
  user: createClient(UserService, transport),
  tenant: createClient(TenantService, transport),
  // member service is TenantMemberService in proto
  member: createClient(TenantMemberService, transport),
  apiToken: createClient(ApiTokenService, transport),
  agentAdmin: createClient(AgentAdminService, transport),
  testRun: createClient(TestRunService, transport),
  template: createClient(TestRunTemplateService, transport),
  suite: createClient(TestSuiteService, transport),
  suiteRun: createClient(TestSuiteRunService, transport),
  sharedRun: createClient(SharedTestRunService, transport),
  sharedSuite: createClient(SharedSuiteRunService, transport),
  comparison: createClient(ComparisonService, transport),
  databasePreset: createClient(DatabasePresetService, transport),
  workloadPreset: createClient(WorkloadPresetService, transport),
  package: createClient(PackageService, transport),
  settings: createClient(SettingsService, transport),
  stroppy: createClient(StroppyService, transport),
  webhook: createClient(WebhookService, transport),
  quota: createClient(QuotaService, transport),
  admin: createClient(AdminService, transport),
  binaryCacheAdmin: createClient(BinaryCacheAdminService, transport),
};
