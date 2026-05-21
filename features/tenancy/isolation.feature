# language: en
@tenancy @migration
Feature: RBAC and tenant isolation (A)

  Authorization is per-tenant via TenantMember.Role (VIEWER<ADMIN<OWNER) in the
  target tenant. Platform-level power (AdminServices) is the Account.is_admin flag,
  outside tenant roles. Isolation: every Own entity is scoped by tenant_id; a request
  to another tenant without membership is denied. Canon: models/tenant.proto, common.proto (Own).

  Scenario Outline: access by role in the target tenant
    Given a tenant member with role <role>
    When they perform <action>
    Then the result = <result>

    Examples:
      | role   | action          | result  |
      | VIEWER | view runs       | allowed |
      | VIEWER | start a test    | denied  |
      | ADMIN  | start a test    | allowed |
      | OWNER  | change members  | allowed |
      | ADMIN  | change members  | denied  |

  Scenario: Access to another tenant is forbidden
    Given an account that is a member of tenant A, not a member of tenant B
    When they request a resource of tenant B
    Then denied (no membership)
    And Own.tenant_id filters the output to their tenants

  Scenario: A platform admin bypasses tenant roles
    Given Account.is_admin = true
    When they call AccountAdminService/TenantAdminService
    Then allowed regardless of TenantMember.Role

  Scenario: A regular account has no admin access
    Given Account.is_admin = false
    When they call AdminService
    Then denied

  @invariant
  Scenario: All Own entities are isolated by tenant
    Given presets/runs/dags of different tenants
    When a member of one tenant lists them
    Then they see only the rows of their tenant (Own.tenant_id + membership)

  @invariant
  Scenario: Every ui request carries a tenant_id and is validated
    Given an account that is a member of multiple tenants
    When they send a ui request with an explicit tenant_id
    Then the server checks membership + role for exactly that tenant_id
    And filters entities by it
    Exceptions: auth RPCs, ListMyTenants, the public GetSharedRun

  Scenario: OWNER manages their tenant's settings (not only server-admin)
    Given a tenant OWNER
    When they call SettingsService.SetSettingsItem (e.g. Yandex creds) for their tenant_id
    Then allowed, settings are saved per-tenant
    And different tenants have different provider settings
    And VIEWER/ADMIN cannot do this (only OWNER or is_admin)

  Scenario: OWNER views quotas/networks and runs reconcile for their tenant
    Given a tenant OWNER
    When they call CloudInventoryService.FetchQuotas/ListNetworkAllocations/Reconcile for their tenant_id
    Then allowed (per their provider settings), no server-admin required
