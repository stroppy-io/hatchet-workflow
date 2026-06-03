// TEMPORARY mock - MUST be deleted before real API wiring; do not build on this.

import type { PermissionJson } from "@/lib/proto/cloud/v1/iam/permission_pb";
import type {
  AccountApiToken,
  AccountExternalIdentity,
  AccountProfile,
  AccountProvider,
  ChangePasswordInput,
  CreateApiTokenInput,
  CreatedApiToken,
  UpdateAccountInput,
} from "@/services/account";

function daysAgo(d: number): string {
  return new Date(Date.now() - d * 86_400_000).toISOString();
}

function permission(
  resource: PermissionJson["resource"],
  action: PermissionJson["action"],
): PermissionJson {
  return { resource, action };
}

function clone<T>(value: T): T {
  return structuredClone(value);
}

function parseDurationSeconds(duration?: string): number | undefined {
  if (!duration) return undefined;
  const match = duration.match(/^(\d+)s$/);
  if (!match) throw new Error("ttl must be a protobuf JSON Duration like 2592000s");
  return Number(match[1]);
}

let account: AccountProfile = {
  id: "u-1",
  email: "ada@stroppy.cloud",
  emailVerified: true,
  nickname: "ada.lovelace",
  isAdmin: true,
  createdAt: daysAgo(720),
  updatedAt: daysAgo(1),
};

let identities: AccountExternalIdentity[] = [
  {
    id: "eid-google-1",
    providerId: "idp-google",
    subject: "google-oauth2|104882111111111111111",
    accountId: "u-1",
    email: "ada@stroppy.cloud",
    createdAt: daysAgo(300),
    updatedAt: daysAgo(30),
  },
  {
    id: "eid-keycloak-1",
    providerId: "idp-keycloak",
    subject: "stroppy-users:ada-lovelace",
    accountId: "u-1",
    email: "ada@stroppy.cloud",
    createdAt: daysAgo(210),
    updatedAt: daysAgo(14),
  },
];

let tokens: AccountApiToken[] = [
  {
    id: "tok-personal-1",
    accountId: "u-1",
    name: "laptop",
    type: "API_TOKEN_TYPE_PERSONAL",
    prefix: "stp_Ada7f3a",
    permissions: [],
    lastUsedAt: daysAgo(0.1),
    createdAt: daysAgo(120),
    updatedAt: daysAgo(2),
  },
  {
    id: "tok-service-1",
    accountId: "u-1",
    name: "ci-runner",
    type: "API_TOKEN_TYPE_SERVICE",
    prefix: "stp_CI91bc",
    permissions: [
      permission("RESOURCE_TEST_RUN", "ACTION_CREATE"),
      permission("RESOURCE_TEST_RUN", "ACTION_LIST"),
      permission("RESOURCE_PACKAGE", "ACTION_READ"),
    ],
    expiresAt: daysAgo(-90),
    lastUsedAt: daysAgo(1),
    createdAt: daysAgo(60),
    updatedAt: daysAgo(1),
  },
  {
    id: "tok-service-2",
    accountId: "u-1",
    name: "legacy-import",
    type: "API_TOKEN_TYPE_SERVICE",
    prefix: "stp_Leg04de",
    permissions: [permission("RESOURCE_PRESET", "ACTION_MANAGE")],
    expiresAt: daysAgo(10),
    createdAt: daysAgo(400),
    updatedAt: daysAgo(120),
  },
];

async function delay() {
  await new Promise((r) => setTimeout(r, 120));
}

export const mockAccountProvider: AccountProvider = {
  async getMyAccount(): Promise<AccountProfile> {
    await delay();
    return clone(account);
  },

  async updateAccount(input: UpdateAccountInput): Promise<AccountProfile> {
    await delay();
    if (input.id !== account.id) throw new Error(`mock: unknown account ${input.id}`);
    if (input.email !== undefined) {
      if (!input.email.includes("@")) throw new Error("email must be valid");
      account.email = input.email;
      account.emailVerified = false;
    }
    if (input.nickname !== undefined) {
      if (!/^[a-zA-Z0-9_.-]{3,64}$/.test(input.nickname)) {
        throw new Error("nickname must match ^[a-zA-Z0-9_.-]+$");
      }
      account.nickname = input.nickname;
    }
    account.updatedAt = new Date().toISOString();
    return clone(account);
  },

  async changePassword(input: ChangePasswordInput): Promise<void> {
    await delay();
    if (!input.oldPassword) throw new Error("old_password is required");
    if (input.newPassword.length < 8) {
      throw new Error("new_password must be at least 8 characters");
    }
  },

  async resendVerification(): Promise<void> {
    await delay();
  },

  async listExternalIdentities(
    accountId: string,
  ): Promise<AccountExternalIdentity[]> {
    await delay();
    return clone(identities.filter((identity) => identity.accountId === accountId));
  },

  async unlinkExternalIdentity(id: string): Promise<void> {
    await delay();
    identities = identities.filter((identity) => identity.id !== id);
  },

  async listApiTokens(accountId: string): Promise<AccountApiToken[]> {
    await delay();
    return clone(tokens.filter((token) => token.accountId === accountId));
  },

  async createApiToken(input: CreateApiTokenInput): Promise<CreatedApiToken> {
    await delay();
    if (input.accountId !== account.id) {
      throw new Error(`mock: unknown account ${input.accountId}`);
    }
    if (!input.name.trim()) throw new Error("name is required");
    if (
      input.type === "API_TOKEN_TYPE_PERSONAL" &&
      input.permissions.length > 0
    ) {
      throw new Error("personal tokens must not carry explicit permissions");
    }
    if (input.type === "API_TOKEN_TYPE_UNSPECIFIED") {
      throw new Error("type must be PERSONAL or SERVICE");
    }

    const ttlSeconds = parseDurationSeconds(input.ttl);
    const createdAt = new Date();
    const id = `tok-${Math.random().toString(16).slice(2, 10)}`;
    const prefix = `stp_${Math.random().toString(36).slice(2, 8)}`;
    const token: AccountApiToken = {
      id,
      accountId: input.accountId,
      name: input.name.trim(),
      type: input.type,
      prefix,
      permissions:
        input.type === "API_TOKEN_TYPE_SERVICE" ? input.permissions : [],
      createdAt: createdAt.toISOString(),
      updatedAt: createdAt.toISOString(),
      expiresAt:
        ttlSeconds === undefined
          ? undefined
          : new Date(createdAt.getTime() + ttlSeconds * 1000).toISOString(),
    };

    tokens = [token, ...tokens];
    return {
      token: clone(token),
      secret: `${prefix}.${Math.random().toString(36).slice(2)}${Math.random()
        .toString(36)
        .slice(2)}`,
    };
  },

  async revokeApiToken(id: string): Promise<void> {
    await delay();
    tokens = tokens.filter((token) => token.id !== id);
  },
};
