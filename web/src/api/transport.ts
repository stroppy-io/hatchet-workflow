import { createConnectTransport } from "@connectrpc/connect-web";
import type { Interceptor } from "@connectrpc/connect";

const accessTokenRef: { value: string | null } = { value: null };
const tenantIdRef: { value: string | null } = { value: null };
let refresher: (() => Promise<string | null>) | null = null;

export function setAccessToken(t: string | null) {
  accessTokenRef.value = t;
}
export function getAccessToken() {
  return accessTokenRef.value;
}
export function setTenantId(t: string | null) {
  tenantIdRef.value = t;
}
export function getTenantId() {
  return tenantIdRef.value;
}
export function setRefresher(fn: (() => Promise<string | null>) | null) {
  refresher = fn;
}

const authInterceptor: Interceptor = (next) => async (req) => {
  if (accessTokenRef.value) {
    req.header.set("Authorization", `Bearer ${accessTokenRef.value}`);
  }
  if (tenantIdRef.value) {
    req.header.set("X-Tenant-Id", tenantIdRef.value);
  }
  try {
    return await next(req);
  } catch (e: unknown) {
    const err = e as { code?: string };
    // On Unauthenticated: refresh once + retry
    if (err?.code === "unauthenticated" && refresher) {
      const fresh = await refresher();
      if (fresh) {
        req.header.set("Authorization", `Bearer ${fresh}`);
        return await next(req);
      }
    }
    throw e;
  }
};

export const transport = createConnectTransport({
  baseUrl: "/", // proxied to :8080 in dev, same-origin in prod
  useBinaryFormat: false,
  interceptors: [authInterceptor],
});
