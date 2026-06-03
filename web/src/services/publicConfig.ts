// Public, pre-login instance config — the unauthenticated subset of the
// platform settings. SystemSettingsService.GetPublicConfig is `public: true`,
// so the sign-in / sign-up screens can read it before any token exists and hide
// closed affordances up front instead of only failing on submit.

import { systemSettingsClient } from "@/services/client";

export interface PublicConfig {
  allowSelfRegistration: boolean;
  allowMemberTenantCreation: boolean;
}

export async function getPublicConfig(): Promise<PublicConfig> {
  const res = await systemSettingsClient.getPublicConfig({});
  return {
    allowSelfRegistration: res.allowSelfRegistration,
    allowMemberTenantCreation: res.allowMemberTenantCreation,
  };
}
