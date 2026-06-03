import { createClient, type Interceptor } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { IamService } from "@/lib/proto/cloud/v1/api/iam_pb";
import { QuotaService } from "@/lib/proto/cloud/v1/api/quota_pb";
import { StroppyService } from "@/lib/proto/cloud/v1/api/stroppy_pb";
import { SuiteService } from "@/lib/proto/cloud/v1/api/suite_pb";
import { TestRunService } from "@/lib/proto/cloud/v1/api/test_run_pb";

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
export const suiteClient = createClient(SuiteService, transport);
export const stroppyClient = createClient(StroppyService, transport);
export const quotaClient = createClient(QuotaService, transport);
