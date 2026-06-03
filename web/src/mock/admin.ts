// TEMPORARY mock - MUST be deleted before real API wiring; do not build on this.

import type {
  AdminAccount,
  AdminPlatformSettings,
  AdminProvider,
  CreateAdminAccountInput,
  ListAccountsInput,
  ResetAdminPasswordInput,
  UpdateAdminAccountInput,
} from "@/services/admin";

function now(): string {
  return new Date().toISOString();
}

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

let accounts: AdminAccount[] = [
  {
    id: "u-1",
    email: "ada@stroppy.cloud",
    emailVerified: true,
    nickname: "ada.lovelace",
    isAdmin: true,
    createdAt: "2026-01-08T09:20:00.000Z",
    updatedAt: "2026-05-19T11:45:00.000Z",
  },
  {
    id: "u-2",
    email: "grace@stroppy.cloud",
    emailVerified: true,
    nickname: "grace.hopper",
    isAdmin: false,
    createdAt: "2026-01-12T14:05:00.000Z",
    updatedAt: "2026-05-20T10:10:00.000Z",
  },
  {
    id: "u-3",
    email: "linus@stroppy.cloud",
    emailVerified: false,
    nickname: "linus.torvalds",
    isAdmin: false,
    createdAt: "2026-02-03T16:40:00.000Z",
    updatedAt: "2026-05-21T08:30:00.000Z",
  },
  {
    id: "u-4",
    email: "barbara@stroppy.cloud",
    emailVerified: true,
    nickname: "barbara.liskov",
    isAdmin: false,
    createdAt: "2026-02-19T13:00:00.000Z",
    updatedAt: "2026-05-22T12:00:00.000Z",
  },
];

let settings: AdminPlatformSettings = {
  serverAddr: "https://cloud.stroppy.dev",
  allowSelfRegistration: false,
  allowMemberTenantCreation: true,
};

function validateAccountIdentity(email: string, nickname: string, id?: string) {
  const normalizedEmail = email.trim().toLowerCase();
  const normalizedNickname = nickname.trim();
  if (!normalizedEmail || !normalizedNickname) {
    throw new Error("email and nickname are required");
  }
  if (
    accounts.some(
      (account) => account.id !== id && account.email.toLowerCase() === normalizedEmail,
    )
  ) {
    throw new Error("email already exists");
  }
  if (
    accounts.some(
      (account) => account.id !== id && account.nickname === normalizedNickname,
    )
  ) {
    throw new Error("nickname already exists");
  }
}

export const mockAdminProvider: AdminProvider = {
  async listAccounts(input?: ListAccountsInput) {
    const pageSize = input?.pageSize && input.pageSize > 0 ? input.pageSize : 50;
    const offset = input?.pageToken ? Number(input.pageToken) : 0;
    const sorted = [...accounts].sort((a, b) =>
      a.nickname.localeCompare(b.nickname),
    );
    const page = sorted.slice(offset, offset + pageSize);
    const next = offset + pageSize < sorted.length ? String(offset + pageSize) : "";
    return { accounts: clone(page), nextPageToken: next };
  },

  async createAccount(input: CreateAdminAccountInput) {
    const email = input.email.trim().toLowerCase();
    const nickname = input.nickname.trim();
    validateAccountIdentity(email, nickname);
    if (!input.password && !input.link) {
      throw new Error("password or link is required");
    }
    if (input.link) {
      if (!input.link.providerId || !input.link.subject || !input.link.email) {
        throw new Error("link.provider_id, link.subject and link.email are required");
      }
    }
    const stamp = now();
    const account: AdminAccount = {
      id: `u-${accounts.length + 1}`,
      email,
      emailVerified: !!input.link,
      nickname,
      isAdmin: !!input.isAdmin,
      createdAt: stamp,
      updatedAt: stamp,
    };
    accounts = [...accounts, account];
    return clone(account);
  },

  async updateAccount(input: UpdateAdminAccountInput) {
    const account = accounts.find((item) => item.id === input.id);
    if (!account) throw new Error("account not found");
    const email = input.email?.trim().toLowerCase() ?? account.email;
    const nickname = input.nickname?.trim() ?? account.nickname;
    validateAccountIdentity(email, nickname, account.id);
    Object.assign(account, {
      email,
      nickname,
      updatedAt: now(),
    });
    return clone(account);
  },

  async deleteAccount(id: string) {
    const account = accounts.find((item) => item.id === id);
    if (!account) return;
    if (account.id === "u-1") {
      throw new Error("cannot delete the active mock admin account");
    }
    accounts = accounts.filter((item) => item.id !== id);
  },

  async resetPassword(input: ResetAdminPasswordInput) {
    const account = accounts.find((item) => item.id === input.accountId);
    if (!account) throw new Error("account not found");
    if (!input.newPassword) throw new Error("new_password is required");
    account.updatedAt = now();
  },

  async getSystemSettings() {
    return clone(settings);
  },

  async updateSystemSettings(next: AdminPlatformSettings) {
    settings = {
      serverAddr: next.serverAddr.trim(),
      allowSelfRegistration: !!next.allowSelfRegistration,
      allowMemberTenantCreation: !!next.allowMemberTenantCreation,
    };
    return clone(settings);
  },
};
