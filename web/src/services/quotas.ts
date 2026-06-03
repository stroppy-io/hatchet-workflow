import {
  QuotaRefreshPolicy,
  type QuotaReservationView,
  type QuotaView,
} from "@/lib/proto/cloud/v1/api/quota_pb";
import { Provider as ProviderEnum } from "@/lib/proto/cloud/v1/deployment/provider_pb";
import { quotaClient } from "@/services/client";
import { resolveTenantId } from "@/services/tenant";
import type { Provider as AppProvider } from "@/services/types";

export type QuotaProviderFilter = AppProvider | "all";
export type QuotaRefresh = "cache" | "refreshIfStale" | "force";

export interface ListQuotasOptions {
  provider?: QuotaProviderFilter;
  refresh?: QuotaRefresh;
}

export interface QuotaProvider {
  listQuotas(tenantSlug: string, options?: ListQuotasOptions): Promise<QuotaView[]>;
  refreshQuotas(tenantSlug: string, provider: AppProvider): Promise<QuotaView[]>;
  getRunQuotaUsage(tenantSlug: string, runId: string): Promise<QuotaReservationView[]>;
}

const realQuotaProvider: QuotaProvider = {
  async listQuotas(tenantSlug, options = {}) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { quotas } = await quotaClient.listQuotas({
      tenantId,
      provider: protoProvider(options.provider),
      refreshPolicy: protoRefresh(options.refresh),
    });
    return quotas;
  },

  async refreshQuotas(tenantSlug, provider) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { quotas } = await quotaClient.refreshQuotas({
      tenantId,
      provider: protoRequiredProvider(provider),
    });
    return quotas;
  },

  async getRunQuotaUsage(tenantSlug, runId) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { reservations } = await quotaClient.getRunQuotaUsage({ tenantId, runId });
    return reservations;
  },
};

let active: QuotaProvider = realQuotaProvider;

export function setQuotaProvider(provider: QuotaProvider): void {
  active = provider;
}

export function getQuotaProvider(): QuotaProvider {
  return active;
}

function protoProvider(provider?: QuotaProviderFilter): ProviderEnum {
  if (!provider || provider === "all") {
    return ProviderEnum.UNSPECIFIED;
  }
  return protoRequiredProvider(provider);
}

function protoRequiredProvider(provider: AppProvider): ProviderEnum {
  switch (provider) {
    case "docker":
      return ProviderEnum.DOCKER;
    case "yandex":
      return ProviderEnum.YANDEX;
    default:
      return ProviderEnum.UNSPECIFIED;
  }
}

function protoRefresh(refresh?: QuotaRefresh): QuotaRefreshPolicy {
  switch (refresh) {
    case "cache":
      return QuotaRefreshPolicy.CACHE_ONLY;
    case "force":
      return QuotaRefreshPolicy.FORCE_REFRESH;
    case "refreshIfStale":
    default:
      return QuotaRefreshPolicy.REFRESH_IF_STALE;
  }
}
