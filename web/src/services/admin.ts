// Platform-admin API surface for the rebuilt shell.
//
// This wraps only admin-capable proto surfaces that exist today:
// - IamService account administration
// - SystemSettingsService platform settings
// There is intentionally no "server health" or list-all-tenants contract here
// until those RPCs exist in proto.

import type { PlatformSettingsJson } from "@/lib/proto/cloud/v1/api/admin_pb";
import type {
  CreateAccountRequestJson,
  ListAccountsRequestJson,
  ResetPasswordRequestJson,
  UpdateAccountRequestJson,
} from "@/lib/proto/cloud/v1/api/iam_pb";
import type { AccountJson } from "@/lib/proto/cloud/v1/iam/account_pb";

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
}

const realAdminProvider: AdminProvider = {
  async listAccounts() {
    throw new Error(
      "real AdminProvider not wired yet - run with VITE_MOCK=1 to preview platform admin",
    );
  },
  async createAccount() {
    throw new Error(
      "real AdminProvider not wired yet - run with VITE_MOCK=1 to preview platform admin",
    );
  },
  async updateAccount() {
    throw new Error(
      "real AdminProvider not wired yet - run with VITE_MOCK=1 to preview platform admin",
    );
  },
  async deleteAccount() {
    throw new Error(
      "real AdminProvider not wired yet - run with VITE_MOCK=1 to preview platform admin",
    );
  },
  async resetPassword() {
    throw new Error(
      "real AdminProvider not wired yet - run with VITE_MOCK=1 to preview platform admin",
    );
  },
  async getSystemSettings() {
    throw new Error(
      "real AdminProvider not wired yet - run with VITE_MOCK=1 to preview platform admin",
    );
  },
  async updateSystemSettings() {
    throw new Error(
      "real AdminProvider not wired yet - run with VITE_MOCK=1 to preview platform admin",
    );
  },
};

let active: AdminProvider = realAdminProvider;

export function setAdminProvider(provider: AdminProvider): void {
  active = provider;
}

export function getAdminProvider(): AdminProvider {
  return active;
}
