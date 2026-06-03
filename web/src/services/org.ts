// Organization (tenant) management surface for the rebuilt shell.
//
// Backs the Organizations area (/orgs and /orgs/:slug): Tenant, Membership,
// Role, and TenantSettingsRecord data. The shapes below are thin projections of
// generated proto JSON types, so the UI cannot grow fields that do not exist in
// the proto contract. API tokens are account-scoped in IAM and intentionally
// live in services/account.ts, not here.

import { toJson, fromJson } from "@bufbuild/protobuf";
import {
  AccountSchema,
  type Account,
  type AccountJson,
} from "@/lib/proto/cloud/v1/iam/account_pb";
import {
  MembershipSchema,
  type Membership,
  type MembershipJson,
} from "@/lib/proto/cloud/v1/iam/membership_pb";
import {
  PermissionSchema,
  type ActionJson,
  type PermissionJson,
  type ResourceJson,
} from "@/lib/proto/cloud/v1/iam/permission_pb";
import {
  RoleSchema,
  type Role,
  type RoleJson,
} from "@/lib/proto/cloud/v1/iam/role_pb";
import {
  TenantSchema,
  type Tenant,
  type TenantJson,
} from "@/lib/proto/cloud/v1/iam/tenant_pb";
import {
  TenantSettingsRecordSchema,
  type TenantSettingsRecord,
  type TenantSettingsRecordJson,
} from "@/lib/proto/cloud/v1/models/tenant_settings_pb";
import { CreateRoleRequestSchema } from "@/lib/proto/cloud/v1/api/iam_pb";
import {
  ProviderSettingsSchema,
  type ProviderSettingsJson,
} from "@/lib/proto/cloud/v1/deployment/provider_pb";
import { iamClient, tenantSettingsClient } from "@/services/client";

/** One grantable permission with its UI label, flattened from CatalogEntry. */
export interface OrgPermissionCatalogEntry {
  resource: ResourceJson;
  action: ActionJson;
  label: string;
}

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

/**
 * SetProviderSettingsInput targets ONE provider's config. The oneof is the
 * deployment.ProviderSettings JSON shape: exactly one of docker / yandex set.
 * Docker.Settings is an empty message (no fields); Yandex.Settings carries the
 * YC credentials/placement values.
 */
export interface SetProviderSettingsInput {
  tenantSlug: string;
  tenantId: string;
  settings: ProviderSettingsJson;
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
  /** SetTenantProviderSettings: replace ONE provider's config. */
  setProviderSettings(input: SetProviderSettingsInput): Promise<OrgDetailVM>;
  /** ListPermissions: the grantable-permission catalog for the role editor. */
  listPermissionCatalog(): Promise<OrgPermissionCatalogEntry[]>;
  /** GetMembership: fetch one membership row by id. */
  getMembership(id: string): Promise<OrgMembership>;
  /** GetRole: fetch one role by id. */
  getRole(id: string): Promise<OrgRole>;
}

// --- proto -> VM mappers -----------------------------------------------------

function toOrgTenant(t: Tenant): OrgTenant {
  const j = toJson(TenantSchema, t) as TenantJson;
  return {
    id: j.id ?? "",
    name: j.name ?? "",
    ownerAccountId: j.ownerAccountId ?? "",
    slug: j.slug ?? "",
    tags: j.tags,
    createdAt: j.createdAt,
    updatedAt: j.updatedAt,
  };
}

function toOrgAccount(a: Account): OrgAccount {
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

function toOrgMembership(m: Membership): OrgMembership {
  const j = toJson(MembershipSchema, m) as MembershipJson;
  return {
    id: j.id ?? "",
    accountId: j.accountId ?? "",
    tenantId: j.tenantId ?? "",
    roleIds: j.roleIds ?? [],
    createdAt: j.createdAt,
    updatedAt: j.updatedAt,
  };
}

function toOrgRole(r: Role): OrgRole {
  const j = toJson(RoleSchema, r) as RoleJson;
  return {
    id: j.id ?? "",
    name: j.name ?? "",
    scope: j.scope ?? "SCOPE_UNSPECIFIED",
    tenantId: j.tenantId ?? "",
    permissions: j.permissions ?? [],
    isSystem: j.isSystem ?? false,
    createdAt: j.createdAt,
    updatedAt: j.updatedAt,
  };
}

function toOrgSettings(s: TenantSettingsRecord | undefined): OrgSettings {
  const j = (s ? toJson(TenantSettingsRecordSchema, s) : {}) as TenantSettingsRecordJson;
  return {
    entity: j.entity ?? {},
    defaultProvider: j.defaultProvider,
    defaultInTenantRating: j.defaultInTenantRating,
    defaultInGlobalRating: j.defaultInGlobalRating,
    defaultMaxParallel: j.defaultMaxParallel,
    runRetentionDays: j.runRetentionDays,
    yandexSettings: j.yandexSettings,
  };
}

// Mutations identify roles/memberships by id alone but must return the full org
// detail. We remember the owning tenant of every row loaded via getOrg so an
// id-only mutate can rebuild the detail it came from.
const roleTenant = new Map<string, string>();
const membershipTenant = new Map<string, string>();

function tenantOf(map: Map<string, string>, id: string, kind: string): string {
  const tenantId = map.get(id);
  if (!tenantId) {
    throw new Error(`unknown ${kind} ${id}: load the organization before mutating it`);
  }
  return tenantId;
}

async function fetchTenantById(tenantId: string): Promise<Tenant> {
  const { tenant } = await iamClient.getTenant({
    ref: { case: "id", value: tenantId },
  });
  if (!tenant) throw new Error(`tenant ${tenantId} not found`);
  return tenant;
}

// Assemble the full /orgs/:slug payload from an already-fetched Tenant: roles,
// memberships joined to their accounts, the caller's own membership, settings.
async function buildDetail(tenant: Tenant): Promise<OrgDetailVM> {
  const tenantId = tenant.id;
  const [{ memberships }, { roles }, { account: me }, settingsResp] =
    await Promise.all([
      iamClient.listMemberships({ tenantId }),
      iamClient.listRoles({ tenantId }),
      iamClient.getMyAccount({}),
      tenantSettingsClient.getTenantSettings({ tenantId }).catch(() => ({
        settings: undefined,
      })),
    ]);

  const orgRoles = roles.map(toOrgRole);
  const rolesById = new Map(orgRoles.map((r) => [r.id, r]));
  orgRoles.forEach((r) => roleTenant.set(r.id, tenantId));

  // Join each membership to its account (one read per distinct account).
  const accountIds = [...new Set(memberships.map((m) => m.accountId))];
  const accounts = await Promise.all(
    accountIds.map((id) =>
      iamClient.getAccount({ id }).then((r) => r.account),
    ),
  );
  const accountById = new Map(
    accounts.filter((a): a is Account => !!a).map((a) => [a.id, toOrgAccount(a)]),
  );

  const memberVMs: OrgMemberVM[] = memberships.map((m) => {
    membershipTenant.set(m.id, tenantId);
    const membership = toOrgMembership(m);
    return {
      membership,
      account:
        accountById.get(membership.accountId) ??
        ({ id: membership.accountId, email: "", nickname: "" } as OrgAccount),
      roles: membership.roleIds
        .map((id) => rolesById.get(id))
        .filter((r): r is OrgRole => !!r),
    };
  });

  const myMembership = memberVMs.find(
    (mv) => me && mv.membership.accountId === me.id,
  );

  return {
    tenant: toOrgTenant(tenant),
    currentMembership: myMembership?.membership,
    currentRoles: myMembership?.roles ?? [],
    memberships: memberVMs,
    roles: orgRoles,
    settings: toOrgSettings(settingsResp.settings),
  };
}

const realOrgProvider: OrgProvider = {
  async listOrgs() {
    const { tenants } = await iamClient.listMyTenants({});
    const { account: me } = await iamClient.getMyAccount({});
    return Promise.all(
      tenants.map(async (tenant) => {
        const { roles } = await iamClient.listRoles({ tenantId: tenant.id });
        const orgRoles = roles.map(toOrgRole);
        const rolesById = new Map(orgRoles.map((r) => [r.id, r]));
        let currentRoles: OrgRole[] = [];
        try {
          const { memberships } = await iamClient.listMemberships({
            tenantId: tenant.id,
          });
          const mine = memberships.find((m) => me && m.accountId === me.id);
          currentRoles =
            mine?.roleIds
              .map((id) => rolesById.get(id))
              .filter((r): r is OrgRole => !!r) ?? [];
        } catch {
          /* membership listing may be denied for platform admins not in tenant */
        }
        return { tenant: toOrgTenant(tenant), currentRoles };
      }),
    );
  },

  async getOrg(slug) {
    const { tenant } = await iamClient.getTenant({
      ref: { case: "slug", value: slug },
    });
    if (!tenant) throw new Error(`organization ${slug} not found`);
    return buildDetail(tenant);
  },

  async createTenant(input) {
    const { tenant } = await iamClient.createTenant({
      name: input.name,
      slug: input.slug,
    });
    if (!tenant) throw new Error("createTenant returned no tenant");
    return buildDetail(tenant);
  },

  async updateTenant(input) {
    const { tenant } = await iamClient.updateTenant({
      id: input.id,
      name: input.name,
      slug: input.slug,
    });
    if (!tenant) throw new Error("updateTenant returned no tenant");
    return buildDetail(tenant);
  },

  async deleteTenant(id) {
    await iamClient.deleteTenant({ id });
  },

  async transferTenantOwnership(input) {
    await iamClient.transferTenantOwnership({
      tenantId: input.tenantId,
      newOwnerAccountId: input.newOwnerAccountId,
    });
    return buildDetail(await fetchTenantById(input.tenantId));
  },

  async leaveTenant(tenantId) {
    await iamClient.leaveTenant({ tenantId });
  },

  async createMembership(input) {
    await iamClient.createMembership({
      accountId: input.accountId,
      tenantId: input.tenantId,
      roleIds: input.roleIds,
    });
    return buildDetail(await fetchTenantById(input.tenantId));
  },

  async updateMembership(input) {
    const tenantId = tenantOf(membershipTenant, input.id, "membership");
    await iamClient.updateMembership({ id: input.id, roleIds: input.roleIds });
    return buildDetail(await fetchTenantById(tenantId));
  },

  async deleteMembership(id) {
    const tenantId = tenantOf(membershipTenant, id, "membership");
    await iamClient.deleteMembership({ id });
    return buildDetail(await fetchTenantById(tenantId));
  },

  async createRole(input) {
    await iamClient.createRole(
      fromJson(CreateRoleRequestSchema, {
        name: input.name,
        scope: input.scope,
        tenantId: input.tenantId,
        permissions: input.permissions,
      }),
    );
    return buildDetail(await fetchTenantById(input.tenantId));
  },

  async updateRole(input) {
    const tenantId = tenantOf(roleTenant, input.id, "role");
    await iamClient.updateRole({
      id: input.id,
      name: input.name,
      permissions: input.permissions.map((p) => fromJson(PermissionSchema, p)),
    });
    return buildDetail(await fetchTenantById(tenantId));
  },

  async deleteRole(id) {
    const tenantId = tenantOf(roleTenant, id, "role");
    await iamClient.deleteRole({ id });
    return buildDetail(await fetchTenantById(tenantId));
  },

  async updateTenantSettings(input) {
    await tenantSettingsClient.updateTenantSettings({
      tenantId: input.tenantId,
      settings: fromJson(TenantSettingsRecordSchema, input.settings),
    });
    return buildDetail(await fetchTenantById(input.tenantId));
  },

  async setProviderSettings(input) {
    await tenantSettingsClient.setTenantProviderSettings({
      tenantId: input.tenantId,
      settings: fromJson(ProviderSettingsSchema, input.settings),
    });
    return buildDetail(await fetchTenantById(input.tenantId));
  },

  async listPermissionCatalog() {
    const { entries } = await iamClient.listPermissions({});
    const catalog: OrgPermissionCatalogEntry[] = [];
    for (const e of entries) {
      if (!e.permission) continue;
      // Convert the numeric-enum Permission message to its JSON-string form so
      // the UI and hasOrgPermission speak the same union as PermissionJson.
      const j = toJson(PermissionSchema, e.permission) as PermissionJson;
      const resource = j.resource ?? "RESOURCE_UNSPECIFIED";
      const action = j.action ?? "ACTION_UNSPECIFIED";
      if (resource === "RESOURCE_UNSPECIFIED" || action === "ACTION_UNSPECIFIED") {
        continue;
      }
      catalog.push({ resource, action, label: e.label });
    }
    return catalog;
  },

  async getMembership(id) {
    const { membership } = await iamClient.getMembership({ id });
    if (!membership) throw new Error(`membership ${id} not found`);
    return toOrgMembership(membership);
  },

  async getRole(id) {
    const { role } = await iamClient.getRole({ id });
    if (!role) throw new Error(`role ${id} not found`);
    return toOrgRole(role);
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
