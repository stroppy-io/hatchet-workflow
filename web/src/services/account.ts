// Account-scoped management surface.
//
// IAM ApiToken authenticates as an Account and is tenant-agnostic. Keep it out
// of /orgs so org management stays tenant-only.

import type { ApiTokenJson } from "@/lib/proto/cloud/v1/iam/apitoken_pb";
import type { PermissionJson } from "@/lib/proto/cloud/v1/iam/permission_pb";
import type { AccountJson } from "@/lib/proto/cloud/v1/iam/account_pb";
import type { ExternalIdentityJson } from "@/lib/proto/cloud/v1/iam/sso_pb";

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
  /** UnlinkExternalIdentity by id. */
  unlinkExternalIdentity(id: string): Promise<void>;
  /** List programmatic credentials for one account. */
  listApiTokens(accountId: string): Promise<AccountApiToken[]>;
  /** CreateApiToken returns metadata plus the one-time secret. */
  createApiToken(input: CreateApiTokenInput): Promise<CreatedApiToken>;
  /** RevokeApiToken by id. */
  revokeApiToken(id: string): Promise<void>;
}

const realAccountProvider: AccountProvider = {
  async getMyAccount() {
    throw new Error(
      "real AccountProvider not wired yet - run with VITE_MOCK=1 to preview account settings",
    );
  },
  async updateAccount() {
    throw new Error(
      "real AccountProvider not wired yet - run with VITE_MOCK=1 to preview account settings",
    );
  },
  async changePassword() {
    throw new Error(
      "real AccountProvider not wired yet - run with VITE_MOCK=1 to preview account settings",
    );
  },
  async resendVerification() {
    throw new Error(
      "real AccountProvider not wired yet - run with VITE_MOCK=1 to preview account settings",
    );
  },
  async listExternalIdentities() {
    throw new Error(
      "real AccountProvider not wired yet - run with VITE_MOCK=1 to preview account settings",
    );
  },
  async unlinkExternalIdentity() {
    throw new Error(
      "real AccountProvider not wired yet - run with VITE_MOCK=1 to preview account settings",
    );
  },
  async listApiTokens() {
    throw new Error(
      "real AccountProvider not wired yet - run with VITE_MOCK=1 to preview account tokens",
    );
  },
  async createApiToken() {
    throw new Error(
      "real AccountProvider not wired yet - run with VITE_MOCK=1 to preview account tokens",
    );
  },
  async revokeApiToken() {
    throw new Error(
      "real AccountProvider not wired yet - run with VITE_MOCK=1 to preview account tokens",
    );
  },
};

let active: AccountProvider = realAccountProvider;

export function setAccountProvider(provider: AccountProvider): void {
  active = provider;
}

export function getAccountProvider(): AccountProvider {
  return active;
}
