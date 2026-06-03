// TEMPORARY mock - MUST be deleted before real API wiring; do not build on this.
//
// Stateful org-management payloads for /orgs and /orgs/:slug. Shapes mirror
// proto JSON fields from Tenant, Membership, Role, Account, and
// TenantSettingsRecord. API tokens are intentionally absent: they belong to the
// account mock.

import type { PermissionJson } from "@/lib/proto/cloud/v1/iam/permission_pb";
import type {
  CreateMembershipInput,
  CreateRoleInput,
  CreateTenantInput,
  OrgAccount,
  OrgDetailVM,
  OrgMemberVM,
  OrgMembership,
  OrgProvider,
  OrgRole,
  OrgSettings,
  OrgSummaryVM,
  OrgTenant,
  TransferTenantOwnershipInput,
  UpdateMembershipInput,
  UpdateRoleInput,
  UpdateTenantInput,
  UpdateTenantSettingsInput,
} from "@/services/org";

const CURRENT_ACCOUNT_ID = "u-1";

function daysAgo(d: number): string {
  return new Date(Date.now() - d * 86_400_000).toISOString();
}

function now(): string {
  return new Date().toISOString();
}

function clone<T>(value: T): T {
  return structuredClone(value);
}

function permission(
  resource: PermissionJson["resource"],
  action: PermissionJson["action"],
): PermissionJson {
  return { resource, action };
}

const ACCOUNTS: Record<string, OrgAccount> = {
  "u-1": {
    id: "u-1",
    email: "ada@stroppy.cloud",
    emailVerified: true,
    nickname: "ada.lovelace",
    isAdmin: true,
    createdAt: daysAgo(720),
    updatedAt: daysAgo(1),
  },
  "u-2": {
    id: "u-2",
    email: "grace@stroppy.cloud",
    emailVerified: true,
    nickname: "grace.hopper",
    isAdmin: false,
    createdAt: daysAgo(500),
    updatedAt: daysAgo(12),
  },
  "u-3": {
    id: "u-3",
    email: "alan@stroppy.cloud",
    emailVerified: true,
    nickname: "alan.turing",
    isAdmin: false,
    createdAt: daysAgo(430),
    updatedAt: daysAgo(30),
  },
  "u-4": {
    id: "u-4",
    email: "edsger@stroppy.cloud",
    emailVerified: false,
    nickname: "edsger.dijkstra",
    isAdmin: false,
    createdAt: daysAgo(220),
    updatedAt: daysAgo(91),
  },
  "u-5": {
    id: "u-5",
    email: "ci@globex.example",
    emailVerified: true,
    nickname: "globex.ci",
    isAdmin: false,
    createdAt: daysAgo(190),
    updatedAt: daysAgo(8),
  },
  "u-6": {
    id: "u-6",
    email: "kj@globex.example",
    emailVerified: true,
    nickname: "katherine.johnson",
    isAdmin: false,
    createdAt: daysAgo(80),
    updatedAt: daysAgo(2),
  },
  "u-7": {
    id: "u-7",
    email: "bill@initech.example",
    emailVerified: false,
    nickname: "bill.lumbergh",
    isAdmin: false,
    createdAt: daysAgo(63),
    updatedAt: daysAgo(63),
  },
};

const OWNER_PERMISSIONS: PermissionJson[] = [
  permission("RESOURCE_TENANT", "ACTION_MANAGE"),
  permission("RESOURCE_MEMBERSHIP", "ACTION_MANAGE"),
  permission("RESOURCE_ROLE", "ACTION_MANAGE"),
  permission("RESOURCE_SETTINGS", "ACTION_MANAGE"),
  permission("RESOURCE_TEST_RUN", "ACTION_MANAGE"),
  permission("RESOURCE_SUITE", "ACTION_MANAGE"),
  permission("RESOURCE_SUITE_RUN", "ACTION_MANAGE"),
  permission("RESOURCE_PRESET", "ACTION_MANAGE"),
  permission("RESOURCE_WIZARD", "ACTION_MANAGE"),
  permission("RESOURCE_SHARE", "ACTION_MANAGE"),
  permission("RESOURCE_PACKAGE", "ACTION_MANAGE"),
];

const OPERATOR_PERMISSIONS: PermissionJson[] = [
  permission("RESOURCE_TENANT", "ACTION_READ"),
  permission("RESOURCE_MEMBERSHIP", "ACTION_LIST"),
  permission("RESOURCE_ROLE", "ACTION_LIST"),
  permission("RESOURCE_SETTINGS", "ACTION_READ"),
  permission("RESOURCE_TEST_RUN", "ACTION_MANAGE"),
  permission("RESOURCE_SUITE_RUN", "ACTION_MANAGE"),
  permission("RESOURCE_PRESET", "ACTION_LIST"),
  permission("RESOURCE_PACKAGE", "ACTION_LIST"),
];

const VIEWER_PERMISSIONS: PermissionJson[] = [
  permission("RESOURCE_TENANT", "ACTION_READ"),
  permission("RESOURCE_MEMBERSHIP", "ACTION_LIST"),
  permission("RESOURCE_ROLE", "ACTION_LIST"),
  permission("RESOURCE_SETTINGS", "ACTION_READ"),
  permission("RESOURCE_TEST_RUN", "ACTION_LIST"),
  permission("RESOURCE_SUITE", "ACTION_LIST"),
  permission("RESOURCE_SUITE_RUN", "ACTION_LIST"),
  permission("RESOURCE_PRESET", "ACTION_LIST"),
];

function role(
  tenantId: string,
  suffix: string,
  name: string,
  permissions: PermissionJson[],
  isSystem = true,
  createdDaysAgo = 420,
): OrgRole {
  return {
    id: `r-${tenantId.slice(2)}-${suffix}`,
    name,
    scope: "SCOPE_TENANT",
    tenantId,
    permissions,
    isSystem,
    createdAt: daysAgo(createdDaysAgo),
    updatedAt: daysAgo(Math.max(1, Math.round(createdDaysAgo / 14))),
  };
}

function membership(
  tenantId: string,
  accountId: string,
  roleIds: string[],
  createdDaysAgo: number,
): OrgMembership {
  return {
    id: `m-${tenantId.slice(2)}-${accountId.slice(2)}`,
    accountId,
    tenantId,
    roleIds,
    createdAt: daysAgo(createdDaysAgo),
    updatedAt: daysAgo(Math.max(1, Math.round(createdDaysAgo / 3))),
  };
}

function tenant(
  id: string,
  slug: string,
  name: string,
  ownerAccountId: string,
  createdDaysAgo: number,
  tags: string[],
  labels: Record<string, string>,
): OrgTenant {
  return {
    id,
    name,
    ownerAccountId,
    slug,
    tags: { tags, labels },
    createdAt: daysAgo(createdDaysAgo),
    updatedAt: daysAgo(Math.max(1, Math.round(createdDaysAgo / 5))),
  };
}

function settings(
  tenantId: string,
  provider: OrgSettings["defaultProvider"],
  yandex?: OrgSettings["yandexSettings"],
): OrgSettings {
  return {
    entity: {
      id: `settings-${tenantId}`,
      tenantId,
      name: "Tenant settings",
      description: "Per-tenant deployment defaults.",
      timings: {
        createdAt: daysAgo(300),
        updatedAt: daysAgo(3),
      },
      authorId: CURRENT_ACCOUNT_ID,
      isFavorite: false,
    },
    defaultProvider: provider,
    defaultInTenantRating: true,
    defaultInGlobalRating: false,
    defaultMaxParallel: provider === "PROVIDER_YANDEX" ? 4 : 0,
    runRetentionDays: provider === "PROVIDER_YANDEX" ? 45 : 0,
    yandexSettings: yandex,
  };
}

function joinMembers(
  memberships: OrgMembership[],
  roles: OrgRole[],
): OrgMemberVM[] {
  return memberships.map((m) => ({
    membership: m,
    account: ACCOUNTS[m.accountId],
    roles: roles.filter((r) => m.roleIds.includes(r.id)),
  }));
}

function syncDetail(detail: OrgDetailVM): OrgDetailVM {
  detail.memberships = joinMembers(
    detail.memberships.map((m) => m.membership),
    detail.roles,
  );
  const current = detail.memberships.find(
    (m) => m.membership.accountId === CURRENT_ACCOUNT_ID,
  );
  detail.currentMembership = current?.membership;
  detail.currentRoles = current?.roles ?? [];
  return detail;
}

function acmeRoles() {
  return [
    role("t-acme", "owner", "owner", OWNER_PERMISSIONS),
    role("t-acme", "operator", "operator", OPERATOR_PERMISSIONS),
    role("t-acme", "viewer", "viewer", VIEWER_PERMISSIONS),
    role(
      "t-acme",
      "ci",
      "ci-runner",
      [
        permission("RESOURCE_TEST_RUN", "ACTION_CREATE"),
        permission("RESOURCE_TEST_RUN", "ACTION_LIST"),
        permission("RESOURCE_PACKAGE", "ACTION_READ"),
      ],
      false,
    ),
  ];
}

function makeDetails(): Record<string, OrgDetailVM> {
  const acme = acmeRoles();
  const globex = [
    role("t-globex", "owner", "owner", OWNER_PERMISSIONS),
    role("t-globex", "operator", "operator", OPERATOR_PERMISSIONS),
    role("t-globex", "viewer", "viewer", VIEWER_PERMISSIONS),
  ];
  const initech = [
    role("t-initech", "owner", "owner", OWNER_PERMISSIONS),
    role("t-initech", "viewer", "viewer", VIEWER_PERMISSIONS),
  ];

  const result: Record<string, OrgDetailVM> = {
    acme: {
      tenant: tenant(
        "t-acme",
        "acme",
        "Acme Corporation",
        CURRENT_ACCOUNT_ID,
        420,
        ["benchmarking", "production"],
        { cost_center: "platform", region: "ru-central" },
      ),
      currentRoles: [],
      memberships: joinMembers(
        [
          membership("t-acme", CURRENT_ACCOUNT_ID, [acme[0].id], 420),
          membership("t-acme", "u-2", [acme[1].id], 300),
          membership("t-acme", "u-3", [acme[1].id, acme[3].id], 210),
          membership("t-acme", "u-4", [acme[2].id], 90),
        ],
        acme,
      ),
      roles: acme,
      settings: settings("t-acme", "PROVIDER_YANDEX", {
        token: "mock-yc-token-acme",
        cloudId: "b1g-acme-cloud",
        folderId: "b1g-acme-folder",
        zone: "ZONE_RU_CENTRAL1_A",
        networkId: "enp-acme-network",
        networkName: "stroppy-acme",
        subnetCidr: "10.80.0.0/16",
        platformId: "PLATFORM_ID_STANDARD_V3",
        imageId: "fd8-acme-image",
        assignPublicIp: false,
        softwareAcceleratedNetwork: true,
        sshUser: "ubuntu",
        sshPublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIacme",
      }),
    },
    globex: {
      tenant: tenant(
        "t-globex",
        "globex",
        "Globex Industries",
        "u-5",
        180,
        ["ci", "shared"],
        { cost_center: "qa" },
      ),
      currentRoles: [],
      memberships: joinMembers(
        [
          membership("t-globex", CURRENT_ACCOUNT_ID, [globex[1].id], 150),
          membership("t-globex", "u-5", [globex[0].id], 180),
          membership("t-globex", "u-6", [globex[2].id], 40),
        ],
        globex,
      ),
      roles: globex,
      settings: settings("t-globex", "PROVIDER_YANDEX", {
        token: "mock-yc-token-globex",
        cloudId: "b1g-globex-cloud",
        folderId: "b1g-globex-folder",
        zone: "ZONE_RU_CENTRAL1_B",
        networkId: "enp-globex-network",
        networkName: "stroppy-globex",
        subnetCidr: "10.92.0.0/16",
        platformId: "PLATFORM_ID_STANDARD_V2",
        imageId: "fd8-globex-image",
        assignPublicIp: true,
        softwareAcceleratedNetwork: false,
        sshUser: "ubuntu",
        sshPublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIglobex",
      }),
    },
    initech: {
      tenant: tenant(
        "t-initech",
        "initech",
        "Initech LLC",
        "u-7",
        63,
        ["trial"],
        { plan: "evaluation" },
      ),
      currentRoles: [],
      memberships: joinMembers(
        [
          membership("t-initech", CURRENT_ACCOUNT_ID, [initech[1].id], 63),
          membership("t-initech", "u-7", [initech[0].id], 63),
        ],
        initech,
      ),
      roles: initech,
      settings: settings("t-initech", "PROVIDER_DOCKER"),
    },
  };

  Object.values(result).forEach(syncDetail);
  return result;
}

let DETAILS = makeDetails();

async function delay() {
  await new Promise((r) => setTimeout(r, 120));
}

function detailBySlug(slug: string): OrgDetailVM {
  const detail = DETAILS[slug];
  if (!detail) throw new Error(`mock: unknown org "${slug}"`);
  return syncDetail(detail);
}

function detailByTenantId(tenantId: string): OrgDetailVM {
  const detail = Object.values(DETAILS).find((d) => d.tenant.id === tenantId);
  if (!detail) throw new Error(`mock: unknown tenant "${tenantId}"`);
  return syncDetail(detail);
}

function slugForTenantId(tenantId: string): string {
  const entry = Object.entries(DETAILS).find(([, d]) => d.tenant.id === tenantId);
  if (!entry) throw new Error(`mock: unknown tenant "${tenantId}"`);
  return entry[0];
}

function detailByMembershipId(id: string): OrgDetailVM {
  const detail = Object.values(DETAILS).find((d) =>
    d.memberships.some((m) => m.membership.id === id),
  );
  if (!detail) throw new Error(`mock: unknown membership "${id}"`);
  return syncDetail(detail);
}

function detailByRoleId(id: string): OrgDetailVM {
  const detail = Object.values(DETAILS).find((d) =>
    d.roles.some((role) => role.id === id),
  );
  if (!detail) throw new Error(`mock: unknown role "${id}"`);
  return syncDetail(detail);
}

function ensureRoleIds(detail: OrgDetailVM, roleIds: string[]) {
  if (roleIds.length === 0) throw new Error("role_ids must not be empty");
  const known = new Set(detail.roles.map((role) => role.id));
  const missing = roleIds.filter((id) => !known.has(id));
  if (missing.length > 0) throw new Error(`unknown role_ids: ${missing.join(", ")}`);
}

function visibleSummaries(): OrgSummaryVM[] {
  return Object.values(DETAILS)
    .map(syncDetail)
    .filter((detail) => !!detail.currentMembership)
    .map((detail) => ({
      tenant: detail.tenant,
      currentRoles: detail.currentRoles,
    }));
}

export const mockOrgProvider: OrgProvider = {
  async listOrgs(): Promise<OrgSummaryVM[]> {
    await delay();
    return clone(visibleSummaries());
  },

  async getOrg(slug: string): Promise<OrgDetailVM> {
    await delay();
    return clone(detailBySlug(slug));
  },

  async createTenant(input: CreateTenantInput): Promise<OrgDetailVM> {
    await delay();
    if (!input.name.trim()) throw new Error("name is required");
    if (!/^[a-z0-9_-]{3,64}$/.test(input.slug)) {
      throw new Error("slug must match ^[a-z0-9_-]+$");
    }
    if (DETAILS[input.slug]) throw new Error(`slug "${input.slug}" already exists`);

    const tenantId = `t-${input.slug}`;
    const roles = [
      role(tenantId, "owner", "owner", OWNER_PERMISSIONS, true, 0),
      role(tenantId, "operator", "operator", OPERATOR_PERMISSIONS, true, 0),
      role(tenantId, "viewer", "viewer", VIEWER_PERMISSIONS, true, 0),
    ];
    const detail: OrgDetailVM = {
      tenant: {
        id: tenantId,
        name: input.name.trim(),
        ownerAccountId: CURRENT_ACCOUNT_ID,
        slug: input.slug,
        tags: { tags: [], labels: {} },
        createdAt: now(),
        updatedAt: now(),
      },
      currentRoles: [],
      memberships: joinMembers(
        [membership(tenantId, CURRENT_ACCOUNT_ID, [roles[0].id], 0)],
        roles,
      ),
      roles,
      settings: settings(tenantId, "PROVIDER_DOCKER"),
    };
    DETAILS[input.slug] = syncDetail(detail);
    return clone(DETAILS[input.slug]);
  },

  async updateTenant(input: UpdateTenantInput): Promise<OrgDetailVM> {
    await delay();
    const oldSlug = slugForTenantId(input.id);
    const detail = DETAILS[oldSlug];
    if (input.name !== undefined) detail.tenant.name = input.name.trim();
    if (input.slug !== undefined && input.slug !== oldSlug) {
      if (!/^[a-z0-9_-]{3,64}$/.test(input.slug)) {
        throw new Error("slug must match ^[a-z0-9_-]+$");
      }
      if (DETAILS[input.slug]) throw new Error(`slug "${input.slug}" already exists`);
      detail.tenant.slug = input.slug;
      delete DETAILS[oldSlug];
      DETAILS[input.slug] = detail;
    }
    detail.tenant.updatedAt = now();
    return clone(syncDetail(detail));
  },

  async deleteTenant(id: string): Promise<void> {
    await delay();
    const slug = slugForTenantId(id);
    delete DETAILS[slug];
  },

  async transferTenantOwnership(
    input: TransferTenantOwnershipInput,
  ): Promise<OrgDetailVM> {
    await delay();
    const detail = detailByTenantId(input.tenantId);
    const isMember = detail.memberships.some(
      (m) => m.membership.accountId === input.newOwnerAccountId,
    );
    if (!isMember) throw new Error("new_owner_account_id must already be a member");
    detail.tenant.ownerAccountId = input.newOwnerAccountId;
    detail.tenant.updatedAt = now();
    return clone(syncDetail(detail));
  },

  async leaveTenant(tenantId: string): Promise<void> {
    await delay();
    const detail = detailByTenantId(tenantId);
    if (detail.tenant.ownerAccountId === CURRENT_ACCOUNT_ID) {
      throw new Error("owner must transfer ownership before leaving");
    }
    detail.memberships = detail.memberships.filter(
      (m) => m.membership.accountId !== CURRENT_ACCOUNT_ID,
    );
    syncDetail(detail);
  },

  async createMembership(input: CreateMembershipInput): Promise<OrgDetailVM> {
    await delay();
    const detail = detailByTenantId(input.tenantId);
    if (!ACCOUNTS[input.accountId]) {
      throw new Error(`mock: unknown account "${input.accountId}"`);
    }
    if (
      detail.memberships.some((m) => m.membership.accountId === input.accountId)
    ) {
      throw new Error("account is already a member");
    }
    ensureRoleIds(detail, input.roleIds);
    detail.memberships.push({
      membership: {
        id: `m-${detail.tenant.id.slice(2)}-${input.accountId.slice(2)}`,
        accountId: input.accountId,
        tenantId: input.tenantId,
        roleIds: input.roleIds,
        createdAt: now(),
        updatedAt: now(),
      },
      account: ACCOUNTS[input.accountId],
      roles: [],
    });
    return clone(syncDetail(detail));
  },

  async updateMembership(input: UpdateMembershipInput): Promise<OrgDetailVM> {
    await delay();
    const detail = detailByMembershipId(input.id);
    ensureRoleIds(detail, input.roleIds);
    const member = detail.memberships.find((m) => m.membership.id === input.id);
    if (!member) throw new Error(`mock: unknown membership "${input.id}"`);
    member.membership.roleIds = input.roleIds;
    member.membership.updatedAt = now();
    return clone(syncDetail(detail));
  },

  async deleteMembership(id: string): Promise<OrgDetailVM> {
    await delay();
    const detail = detailByMembershipId(id);
    const member = detail.memberships.find((m) => m.membership.id === id);
    if (!member) throw new Error(`mock: unknown membership "${id}"`);
    if (member.membership.accountId === detail.tenant.ownerAccountId) {
      throw new Error("owner membership cannot be removed");
    }
    detail.memberships = detail.memberships.filter((m) => m.membership.id !== id);
    return clone(syncDetail(detail));
  },

  async createRole(input: CreateRoleInput): Promise<OrgDetailVM> {
    await delay();
    const detail = detailByTenantId(input.tenantId);
    if (!input.name.trim()) throw new Error("name is required");
    const id = `r-${input.tenantId.slice(2)}-${Math.random()
      .toString(36)
      .slice(2, 8)}`;
    detail.roles.push({
      id,
      name: input.name.trim(),
      scope: input.scope,
      tenantId: input.scope === "SCOPE_TENANT" ? input.tenantId : "",
      permissions: input.permissions,
      isSystem: false,
      createdAt: now(),
      updatedAt: now(),
    });
    return clone(syncDetail(detail));
  },

  async updateRole(input: UpdateRoleInput): Promise<OrgDetailVM> {
    await delay();
    const detail = detailByRoleId(input.id);
    const role = detail.roles.find((item) => item.id === input.id);
    if (!role) throw new Error(`mock: unknown role "${input.id}"`);
    if (role.isSystem) throw new Error("system roles cannot be updated");
    if (input.name !== undefined) role.name = input.name.trim();
    role.permissions = input.permissions;
    role.updatedAt = now();
    return clone(syncDetail(detail));
  },

  async deleteRole(id: string): Promise<OrgDetailVM> {
    await delay();
    const detail = detailByRoleId(id);
    const role = detail.roles.find((item) => item.id === id);
    if (!role) throw new Error(`mock: unknown role "${id}"`);
    if (role.isSystem) throw new Error("system roles cannot be deleted");
    const inUse = detail.memberships.some((m) => m.membership.roleIds.includes(id));
    if (inUse) throw new Error("role is still assigned to a membership");
    detail.roles = detail.roles.filter((item) => item.id !== id);
    return clone(syncDetail(detail));
  },

  async updateTenantSettings(
    input: UpdateTenantSettingsInput,
  ): Promise<OrgDetailVM> {
    await delay();
    const detail = detailByTenantId(input.tenantId);
    const nextSettings = clone(input.settings);
    nextSettings.entity.tenantId = input.tenantId;
    nextSettings.entity.timings = {
      ...nextSettings.entity.timings,
      updatedAt: now(),
    };
    detail.settings = nextSettings;
    return clone(syncDetail(detail));
  },
};
