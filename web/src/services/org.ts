// Organization (tenant) management surface for the rebuilt shell.
//
// Backs the Organizations area (/orgs and /orgs/:slug): Tenant, Membership,
// Role, and TenantSettingsRecord data. The shapes below are thin projections of
// generated proto JSON types, so the UI cannot grow fields that do not exist in
// the proto contract. API tokens are account-scoped in IAM and intentionally
// live in services/account.ts, not here.

import type { AccountJson } from "@/lib/proto/cloud/v1/iam/account_pb";
import type { MembershipJson } from "@/lib/proto/cloud/v1/iam/membership_pb";
import type {
  ActionJson,
  PermissionJson,
  ResourceJson,
} from "@/lib/proto/cloud/v1/iam/permission_pb";
import type { RoleJson } from "@/lib/proto/cloud/v1/iam/role_pb";
import type { TenantJson } from "@/lib/proto/cloud/v1/iam/tenant_pb";
import type { TenantSettingsRecordJson } from "@/lib/proto/cloud/v1/models/tenant_settings_pb";

export type OrgTenant = Required<
  Pick<TenantJson, "id" | "name" | "ownerAccountId" | "slug">
> &
  Pick<TenantJson, "tags" | "createdAt" | "updatedAt">;

export type OrgAccount = Required<
  Pick<AccountJson, "id" | "email" | "nickname">
> &
  Pick<
    AccountJson,
    "emailVerified" | "isAdmin" | "createdAt" | "updatedAt"
  >;

export type OrgMembership = Required<
  Pick<MembershipJson, "id" | "accountId" | "tenantId">
> & {
  roleIds: string[];
} & Pick<MembershipJson, "createdAt" | "updatedAt">;

export type OrgRole = Required<
  Pick<RoleJson, "id" | "name" | "scope" | "tenantId">
> & {
  permissions: PermissionJson[];
  isSystem: boolean;
} & Pick<RoleJson, "createdAt" | "updatedAt">;

export type OrgSettings = Required<Pick<TenantSettingsRecordJson, "entity">> &
  Pick<
    TenantSettingsRecordJson,
    | "defaultProvider"
    | "defaultInTenantRating"
    | "defaultInGlobalRating"
    | "defaultMaxParallel"
    | "runRetentionDays"
    | "yandexSettings"
  >;

/** Summary card for the org list (/orgs). */
export interface OrgSummaryVM {
  tenant: OrgTenant;
  currentRoles: OrgRole[];
}

/** A joined membership row for /orgs/:slug -> Members. */
export interface OrgMemberVM {
  membership: OrgMembership;
  account: OrgAccount;
  roles: OrgRole[];
}

/** The full single-org payload assembled for /orgs/:slug. */
export interface OrgDetailVM {
  tenant: OrgTenant;
  currentMembership?: OrgMembership;
  currentRoles: OrgRole[];
  memberships: OrgMemberVM[];
  roles: OrgRole[];
  settings: OrgSettings;
}

export interface CreateTenantInput {
  name: string;
  slug: string;
}

export interface UpdateTenantInput {
  id: string;
  name?: string;
  slug?: string;
}

export interface TransferTenantOwnershipInput {
  tenantId: string;
  newOwnerAccountId: string;
}

export interface CreateMembershipInput {
  accountId: string;
  tenantId: string;
  roleIds: string[];
}

export interface UpdateMembershipInput {
  id: string;
  roleIds: string[];
}

export interface CreateRoleInput {
  name: string;
  scope: OrgRole["scope"];
  tenantId: string;
  permissions: PermissionJson[];
}

export interface UpdateRoleInput {
  id: string;
  name?: string;
  permissions: PermissionJson[];
}

export interface UpdateTenantSettingsInput {
  tenantId: string;
  settings: OrgSettings;
}

export function roleListLabel(roles: Pick<OrgRole, "name">[]): string {
  if (roles.length === 0) return "No roles";
  if (roles.length === 1) return roles[0].name;
  return `${roles.length} roles`;
}

export function hasOrgPermission(
  roles: OrgRole[],
  resource: ResourceJson,
  action: ActionJson,
): boolean {
  return roles.some((role) =>
    role.permissions.some((permission) => {
      if (permission.resource !== resource) return false;
      return permission.action === action || permission.action === "ACTION_MANAGE";
    }),
  );
}

/**
 * OrgProvider abstracts the org reads so the Organizations pages never import
 * mock or backend specifics directly. The real backend provider replaces the
 * mock one; the pages only ever depend on this interface.
 */
export interface OrgProvider {
  /** List the organizations the signed-in user belongs to. */
  listOrgs(): Promise<OrgSummaryVM[]>;
  /** Fetch the detail payload for a single org by slug. */
  getOrg(slug: string): Promise<OrgDetailVM>;
  /** CreateTenant. */
  createTenant(input: CreateTenantInput): Promise<OrgDetailVM>;
  /** UpdateTenant. */
  updateTenant(input: UpdateTenantInput): Promise<OrgDetailVM>;
  /** DeleteTenant. */
  deleteTenant(id: string): Promise<void>;
  /** TransferTenantOwnership. */
  transferTenantOwnership(
    input: TransferTenantOwnershipInput,
  ): Promise<OrgDetailVM>;
  /** LeaveTenant for the caller. */
  leaveTenant(tenantId: string): Promise<void>;
  /** CreateMembership. */
  createMembership(input: CreateMembershipInput): Promise<OrgDetailVM>;
  /** UpdateMembership. */
  updateMembership(input: UpdateMembershipInput): Promise<OrgDetailVM>;
  /** DeleteMembership. */
  deleteMembership(id: string): Promise<OrgDetailVM>;
  /** CreateRole. */
  createRole(input: CreateRoleInput): Promise<OrgDetailVM>;
  /** UpdateRole. */
  updateRole(input: UpdateRoleInput): Promise<OrgDetailVM>;
  /** DeleteRole. */
  deleteRole(id: string): Promise<OrgDetailVM>;
  /** UpdateTenantSettings. */
  updateTenantSettings(input: UpdateTenantSettingsInput): Promise<OrgDetailVM>;
}

/**
 * Real backend provider. TODO(real-api): wire the read paths through the
 * connect IAM client (ListMyTenants/GetTenant/ListMemberships/ListRoles plus
 * joined Account reads) and the tenant-settings client. Throws until
 * implemented so a missing-backend misconfig is loud, not silent.
 */
const realOrgProvider: OrgProvider = {
  async listOrgs() {
    throw new Error(
      "real OrgProvider not wired yet - run with VITE_MOCK=1 to preview the org pages",
    );
  },
  async getOrg() {
    throw new Error(
      "real OrgProvider not wired yet - run with VITE_MOCK=1 to preview the org pages",
    );
  },
  async createTenant() {
    throw new Error(
      "real OrgProvider not wired yet - run with VITE_MOCK=1 to preview the org pages",
    );
  },
  async updateTenant() {
    throw new Error(
      "real OrgProvider not wired yet - run with VITE_MOCK=1 to preview the org pages",
    );
  },
  async deleteTenant() {
    throw new Error(
      "real OrgProvider not wired yet - run with VITE_MOCK=1 to preview the org pages",
    );
  },
  async transferTenantOwnership() {
    throw new Error(
      "real OrgProvider not wired yet - run with VITE_MOCK=1 to preview the org pages",
    );
  },
  async leaveTenant() {
    throw new Error(
      "real OrgProvider not wired yet - run with VITE_MOCK=1 to preview the org pages",
    );
  },
  async createMembership() {
    throw new Error(
      "real OrgProvider not wired yet - run with VITE_MOCK=1 to preview the org pages",
    );
  },
  async updateMembership() {
    throw new Error(
      "real OrgProvider not wired yet - run with VITE_MOCK=1 to preview the org pages",
    );
  },
  async deleteMembership() {
    throw new Error(
      "real OrgProvider not wired yet - run with VITE_MOCK=1 to preview the org pages",
    );
  },
  async createRole() {
    throw new Error(
      "real OrgProvider not wired yet - run with VITE_MOCK=1 to preview the org pages",
    );
  },
  async updateRole() {
    throw new Error(
      "real OrgProvider not wired yet - run with VITE_MOCK=1 to preview the org pages",
    );
  },
  async deleteRole() {
    throw new Error(
      "real OrgProvider not wired yet - run with VITE_MOCK=1 to preview the org pages",
    );
  },
  async updateTenantSettings() {
    throw new Error(
      "real OrgProvider not wired yet - run with VITE_MOCK=1 to preview the org pages",
    );
  },
};

// --- Provider injection -----------------------------------------------------
//
// Defaults to the real backend. The mock (and ONLY the mock) overrides it via
// setOrgProvider() from the single gate in main.tsx. To remove the mock:
// delete src/mock/ and the gated call in main.tsx.

let active: OrgProvider = realOrgProvider;

export function setOrgProvider(provider: OrgProvider): void {
  active = provider;
}

export function getOrgProvider(): OrgProvider {
  return active;
}
