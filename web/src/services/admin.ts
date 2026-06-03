// Platform-admin API surface for the rebuilt shell.
//
// This wraps only admin-capable proto surfaces that exist today:
// - IamService account administration
// - SystemSettingsService platform settings
// There is intentionally no "server health" or list-all-tenants contract here
// until those RPCs exist in proto.

import { toJson, fromJson, type JsonValue } from "@bufbuild/protobuf";
import {
  PlatformSettingsSchema,
  type PlatformSettings,
  type PlatformSettingsJson,
} from "@/lib/proto/cloud/v1/api/admin_pb";
import {
  CreateAccountRequestSchema,
  CreateIdentityProviderRequestSchema,
  UpdateIdentityProviderRequestSchema,
  type CreateAccountRequestJson,
  type CreateIdentityProviderRequestJson,
  type ListAccountsRequestJson,
  type ResetPasswordRequestJson,
  type UpdateAccountRequestJson,
  type UpdateIdentityProviderRequestJson,
} from "@/lib/proto/cloud/v1/api/iam_pb";
import {
  AccountSchema,
  type Account,
  type AccountJson,
} from "@/lib/proto/cloud/v1/iam/account_pb";
import {
  IdentityProviderSchema,
  type IdentityProvider,
  type IdentityProviderJson,
} from "@/lib/proto/cloud/v1/iam/sso_pb";
import { iamClient, systemSettingsClient } from "@/services/client";

export type AdminAccount = Required<
  Pick<AccountJson, "id" | "email" | "emailVerified" | "nickname" | "isAdmin">
> &
  Pick<AccountJson, "createdAt" | "updatedAt">;

export type AdminPlatformSettings = Required<
  Pick<
    PlatformSettingsJson,
    "serverAddr" | "allowSelfRegistration" | "allowMemberTenantCreation"
  >
>;

export type ListAccountsInput = Pick<
  ListAccountsRequestJson,
  "pageSize" | "pageToken"
>;

export type CreateAdminAccountInput = Required<
  Pick<CreateAccountRequestJson, "email" | "nickname">
> &
  Pick<CreateAccountRequestJson, "password" | "isAdmin" | "link">;

export type UpdateAdminAccountInput = Required<
  Pick<UpdateAccountRequestJson, "id">
> &
  Pick<UpdateAccountRequestJson, "email" | "nickname">;

export type ResetAdminPasswordInput = Required<
  Pick<ResetPasswordRequestJson, "accountId" | "newPassword">
>;

export interface AdminAccountsPage {
  accounts: AdminAccount[];
  nextPageToken: string;
}

/**
 * AdminIdentityProvider is the flat VM for one OIDC provider. client_secret is
 * write-only in the proto (never returned on IdentityProvider), so it lives
 * only on the create/update inputs below, never on this read VM.
 *
 * NOTE: the proto only exposes a slim public list RPC (ListIdentityProviders ->
 * SsoButton, id/slug/displayName only) — there is no admin "list full records"
 * RPC. The admin list therefore fetches the buttons and hydrates each into a
 * full record via GetIdentityProvider(id).
 */
export type AdminIdentityProvider = Required<
  Pick<
    IdentityProviderJson,
    | "id"
    | "slug"
    | "displayName"
    | "issuer"
    | "clientId"
    | "autoProvision"
    | "disabled"
  >
> & {
  scopes: string[];
  allowedDomains: string[];
} & Pick<IdentityProviderJson, "createdAt" | "updatedAt">;

export interface CreateIdentityProviderInput {
  slug: string;
  displayName: string;
  issuer: string;
  clientId: string;
  clientSecret: string;
  scopes: string[];
  allowedDomains: string[];
  autoProvision: boolean;
}

export interface UpdateIdentityProviderInput {
  id: string;
  displayName?: string;
  issuer?: string;
  clientId?: string;
  /** present + non-empty ROTATES the stored secret; omit to leave unchanged. */
  clientSecret?: string;
  scopes: string[];
  allowedDomains: string[];
  autoProvision?: boolean;
  disabled?: boolean;
}

export interface AdminProvider {
  /** ListAccounts. */
  listAccounts(input?: ListAccountsInput): Promise<AdminAccountsPage>;
  /** CreateAccount. */
  createAccount(input: CreateAdminAccountInput): Promise<AdminAccount>;
  /** UpdateAccount. */
  updateAccount(input: UpdateAdminAccountInput): Promise<AdminAccount>;
  /** DeleteAccount. */
  deleteAccount(id: string): Promise<void>;
  /** ResetPassword. */
  resetPassword(input: ResetAdminPasswordInput): Promise<void>;
  /** GetSystemSettings. */
  getSystemSettings(): Promise<AdminPlatformSettings>;
  /** UpdateSystemSettings. */
  updateSystemSettings(
    settings: AdminPlatformSettings,
  ): Promise<AdminPlatformSettings>;
  /**
   * ListIdentityProviders + GetIdentityProvider: the slim public buttons list
   * hydrated into full records (see AdminIdentityProvider).
   */
  listIdentityProviders(): Promise<AdminIdentityProvider[]>;
  /** GetIdentityProvider. */
  getIdentityProvider(id: string): Promise<AdminIdentityProvider>;
  /** CreateIdentityProvider. */
  createIdentityProvider(
    input: CreateIdentityProviderInput,
  ): Promise<AdminIdentityProvider>;
  /** UpdateIdentityProvider. */
  updateIdentityProvider(
    input: UpdateIdentityProviderInput,
  ): Promise<AdminIdentityProvider>;
  /** DeleteIdentityProvider. */
  deleteIdentityProvider(id: string): Promise<void>;
}

function toAdminAccount(a: Account): AdminAccount {
  const j = toJson(AccountSchema, a) as AccountJson;
  return {
    id: j.id ?? "",
    email: j.email ?? "",
    emailVerified: j.emailVerified ?? false,
    nickname: j.nickname ?? "",
    isAdmin: j.isAdmin ?? false,
    createdAt: j.createdAt,
    updatedAt: j.updatedAt,
  };
}

function toAdminIdentityProvider(p: IdentityProvider): AdminIdentityProvider {
  const j = toJson(IdentityProviderSchema, p) as IdentityProviderJson;
  return {
    id: j.id ?? "",
    slug: j.slug ?? "",
    displayName: j.displayName ?? "",
    issuer: j.issuer ?? "",
    clientId: j.clientId ?? "",
    scopes: j.scopes ?? [],
    allowedDomains: j.allowedDomains ?? [],
    autoProvision: j.autoProvision ?? false,
    disabled: j.disabled ?? false,
    createdAt: j.createdAt,
    updatedAt: j.updatedAt,
  };
}

function toSettings(s: PlatformSettings | undefined): AdminPlatformSettings {
  const j = (s ? toJson(PlatformSettingsSchema, s) : {}) as PlatformSettingsJson;
  return {
    serverAddr: j.serverAddr ?? "",
    allowSelfRegistration: j.allowSelfRegistration ?? false,
    allowMemberTenantCreation: j.allowMemberTenantCreation ?? false,
  };
}

const realAdminProvider: AdminProvider = {
  async listAccounts(input) {
    const { accounts, nextPageToken } = await iamClient.listAccounts({
      pageSize: input?.pageSize,
      pageToken: input?.pageToken,
    });
    return { accounts: accounts.map(toAdminAccount), nextPageToken };
  },

  async createAccount(input) {
    const json: CreateAccountRequestJson = {
      email: input.email,
      nickname: input.nickname,
      isAdmin: input.isAdmin ?? false,
    };
    if (input.password) json.password = input.password;
    if (input.link) json.link = input.link;
    const { account } = await iamClient.createAccount(
      fromJson(CreateAccountRequestSchema, json as JsonValue),
    );
    if (!account) throw new Error("createAccount returned no account");
    return toAdminAccount(account);
  },

  async updateAccount(input) {
    const { account } = await iamClient.updateAccount({
      id: input.id,
      email: input.email,
      nickname: input.nickname,
    });
    if (!account) throw new Error("updateAccount returned no account");
    return toAdminAccount(account);
  },

  async deleteAccount(id) {
    await iamClient.deleteAccount({ id });
  },

  async resetPassword(input) {
    await iamClient.resetPassword({
      accountId: input.accountId,
      newPassword: input.newPassword,
    });
  },

  async getSystemSettings() {
    const { settings } = await systemSettingsClient.getSystemSettings({});
    return toSettings(settings);
  },

  async updateSystemSettings(settings) {
    const { settings: updated } = await systemSettingsClient.updateSystemSettings({
      settings: fromJson(PlatformSettingsSchema, settings),
    });
    return toSettings(updated);
  },

  async listIdentityProviders() {
    // The list RPC is the slim public buttons view (id/slug/displayName) — it
    // returns ENABLED providers only and omits admin config. Hydrate each into
    // a full record with GetIdentityProvider so the admin table has issuer,
    // client_id, domains, scopes, and the disabled flag.
    const { buttons } = await iamClient.listIdentityProviders({});
    const providers = await Promise.all(
      buttons.map((b) =>
        iamClient
          .getIdentityProvider({ id: b.id })
          .then((r) => r.provider)
          .catch(() => undefined),
      ),
    );
    return providers
      .filter((p): p is IdentityProvider => !!p)
      .map(toAdminIdentityProvider);
  },

  async getIdentityProvider(id) {
    const { provider } = await iamClient.getIdentityProvider({ id });
    if (!provider) throw new Error(`identity provider ${id} not found`);
    return toAdminIdentityProvider(provider);
  },

  async createIdentityProvider(input) {
    const json: CreateIdentityProviderRequestJson = {
      slug: input.slug,
      displayName: input.displayName,
      issuer: input.issuer,
      clientId: input.clientId,
      clientSecret: input.clientSecret,
      scopes: input.scopes,
      allowedDomains: input.allowedDomains,
      autoProvision: input.autoProvision,
    };
    const { provider } = await iamClient.createIdentityProvider(
      fromJson(CreateIdentityProviderRequestSchema, json),
    );
    if (!provider) throw new Error("createIdentityProvider returned no provider");
    return toAdminIdentityProvider(provider);
  },

  async updateIdentityProvider(input) {
    const json: UpdateIdentityProviderRequestJson = {
      id: input.id,
      scopes: input.scopes,
      allowedDomains: input.allowedDomains,
    };
    if (input.displayName !== undefined) json.displayName = input.displayName;
    if (input.issuer !== undefined) json.issuer = input.issuer;
    if (input.clientId !== undefined) json.clientId = input.clientId;
    // Empty/undefined client_secret leaves the stored secret unchanged.
    if (input.clientSecret) json.clientSecret = input.clientSecret;
    if (input.autoProvision !== undefined) json.autoProvision = input.autoProvision;
    if (input.disabled !== undefined) json.disabled = input.disabled;
    const { provider } = await iamClient.updateIdentityProvider(
      fromJson(UpdateIdentityProviderRequestSchema, json),
    );
    if (!provider) throw new Error("updateIdentityProvider returned no provider");
    return toAdminIdentityProvider(provider);
  },

  async deleteIdentityProvider(id) {
    await iamClient.deleteIdentityProvider({ id });
  },
};

let active: AdminProvider = realAdminProvider;

export function setAdminProvider(provider: AdminProvider): void {
  active = provider;
}

export function getAdminProvider(): AdminProvider {
  return active;
}
