// Account-scoped management surface.
//
// IAM ApiToken authenticates as an Account and is tenant-agnostic. Keep it out
// of /orgs so org management stays tenant-only.

import { toJson, fromJson } from "@bufbuild/protobuf";
import {
  ApiTokenSchema,
  type ApiToken,
  type ApiTokenJson,
} from "@/lib/proto/cloud/v1/iam/apitoken_pb";
import type { PermissionJson } from "@/lib/proto/cloud/v1/iam/permission_pb";
import {
  AccountSchema,
  type Account,
  type AccountJson,
} from "@/lib/proto/cloud/v1/iam/account_pb";
import {
  ExternalIdentitySchema,
  type ExternalIdentity,
  type ExternalIdentityJson,
} from "@/lib/proto/cloud/v1/iam/sso_pb";
import { CreateApiTokenRequestSchema } from "@/lib/proto/cloud/v1/api/iam_pb";
import { iamClient } from "@/services/client";

export type AccountProfile = Required<
  Pick<AccountJson, "id" | "email" | "nickname">
> &
  Pick<
    AccountJson,
    "emailVerified" | "isAdmin" | "createdAt" | "updatedAt"
  >;

export type AccountExternalIdentity = Required<
  Pick<
    ExternalIdentityJson,
    "id" | "providerId" | "subject" | "accountId" | "email"
  >
> &
  Pick<ExternalIdentityJson, "createdAt" | "updatedAt">;

export type AccountApiToken = Required<
  Pick<ApiTokenJson, "id" | "accountId" | "name" | "type" | "prefix">
> & {
  permissions: PermissionJson[];
} & Pick<ApiTokenJson, "expiresAt" | "lastUsedAt" | "createdAt" | "updatedAt">;

export interface UpdateAccountInput {
  id: string;
  email?: string;
  nickname?: string;
}

export interface ChangePasswordInput {
  oldPassword: string;
  newPassword: string;
}

export interface CreateApiTokenInput {
  accountId: string;
  name: string;
  type: AccountApiToken["type"];
  permissions: PermissionJson[];
  /** Protobuf JSON Duration, for example "2592000s". */
  ttl?: string;
}

export interface CreatedApiToken {
  token: AccountApiToken;
  secret: string;
}

export interface LinkExternalIdentityInput {
  accountId: string;
  providerId: string;
  subject: string;
  email: string;
}

export interface AccountProvider {
  /** GetMyAccount: returns the caller's own account. */
  getMyAccount(): Promise<AccountProfile>;
  /** UpdateAccount: mutates email and/or nickname. */
  updateAccount(input: UpdateAccountInput): Promise<AccountProfile>;
  /** ChangePassword: self-service password rotation. */
  changePassword(input: ChangePasswordInput): Promise<void>;
  /** ResendVerification: dispatches a new verification token to current email. */
  resendVerification(): Promise<void>;
  /** ListExternalIdentities for the account. */
  listExternalIdentities(accountId: string): Promise<AccountExternalIdentity[]>;
  /**
   * LinkExternalIdentity: bind an account to an admin-asserted (provider,
   * subject) IdP identity. Returns the resulting link.
   */
  linkExternalIdentity(
    input: LinkExternalIdentityInput,
  ): Promise<AccountExternalIdentity>;
  /** UnlinkExternalIdentity by id. */
  unlinkExternalIdentity(id: string): Promise<void>;
  /** List programmatic credentials for one account. */
  listApiTokens(accountId: string): Promise<AccountApiToken[]>;
  /** CreateApiToken returns metadata plus the one-time secret. */
  createApiToken(input: CreateApiTokenInput): Promise<CreatedApiToken>;
  /** RevokeApiToken by id. */
  revokeApiToken(id: string): Promise<void>;
}

// --- proto -> VM mappers -----------------------------------------------------
// VMs are JSON-shaped (timestamps/durations as strings); toJson does the
// proto3 conversion, then we coerce the always-present identity fields.

function toProfile(a: Account): AccountProfile {
  const j = toJson(AccountSchema, a) as AccountJson;
  return {
    id: j.id ?? "",
    email: j.email ?? "",
    nickname: j.nickname ?? "",
    emailVerified: j.emailVerified,
    isAdmin: j.isAdmin,
    createdAt: j.createdAt,
    updatedAt: j.updatedAt,
  };
}

function toIdentity(e: ExternalIdentity): AccountExternalIdentity {
  const j = toJson(ExternalIdentitySchema, e) as ExternalIdentityJson;
  return {
    id: j.id ?? "",
    providerId: j.providerId ?? "",
    subject: j.subject ?? "",
    accountId: j.accountId ?? "",
    email: j.email ?? "",
    createdAt: j.createdAt,
    updatedAt: j.updatedAt,
  };
}

function toToken(t: ApiToken): AccountApiToken {
  const j = toJson(ApiTokenSchema, t) as ApiTokenJson;
  return {
    id: j.id ?? "",
    accountId: j.accountId ?? "",
    name: j.name ?? "",
    type: j.type ?? "API_TOKEN_TYPE_UNSPECIFIED",
    prefix: j.prefix ?? "",
    permissions: j.permissions ?? [],
    expiresAt: j.expiresAt,
    lastUsedAt: j.lastUsedAt,
    createdAt: j.createdAt,
    updatedAt: j.updatedAt,
  };
}

const realAccountProvider: AccountProvider = {
  async getMyAccount() {
    const { account } = await iamClient.getMyAccount({});
    if (!account) throw new Error("getMyAccount returned no account");
    return toProfile(account);
  },

  async updateAccount(input) {
    const { account } = await iamClient.updateAccount({
      id: input.id,
      email: input.email,
      nickname: input.nickname,
    });
    if (!account) throw new Error("updateAccount returned no account");
    return toProfile(account);
  },

  async changePassword(input) {
    await iamClient.changePassword({
      oldPassword: input.oldPassword,
      newPassword: input.newPassword,
    });
  },

  async resendVerification() {
    await iamClient.resendVerification({});
  },

  async listExternalIdentities(accountId) {
    const { identities } = await iamClient.listExternalIdentities({ accountId });
    return identities.map(toIdentity);
  },

  async linkExternalIdentity(input) {
    const { identity } = await iamClient.linkExternalIdentity({
      accountId: input.accountId,
      link: {
        providerId: input.providerId,
        subject: input.subject,
        email: input.email,
      },
    });
    if (!identity) throw new Error("linkExternalIdentity returned no identity");
    return toIdentity(identity);
  },

  async unlinkExternalIdentity(id) {
    await iamClient.unlinkExternalIdentity({ id });
  },

  async listApiTokens(accountId) {
    const { tokens } = await iamClient.listApiTokens({ accountId });
    return tokens.map(toToken);
  },

  async createApiToken(input) {
    const req = fromJson(CreateApiTokenRequestSchema, {
      accountId: input.accountId,
      name: input.name,
      type: input.type,
      permissions: input.permissions,
      ...(input.ttl ? { ttl: input.ttl } : {}),
    });
    const { token, secret } = await iamClient.createApiToken(req);
    if (!token) throw new Error("createApiToken returned no token");
    return { token: toToken(token), secret };
  },

  async revokeApiToken(id) {
    await iamClient.revokeApiToken({ id });
  },
};

let active: AccountProvider = realAccountProvider;

export function setAccountProvider(provider: AccountProvider): void {
  active = provider;
}

export function getAccountProvider(): AccountProvider {
  return active;
}
