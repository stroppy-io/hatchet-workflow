# language: en
@tenancy @migration
Feature: Full RBAC matrix (A)

  The complete role x action contract. Per-tenant authority is TenantMember.Role
  with VIEWER < ADMIN < OWNER, evaluated against the request's tenant_id. The
  platform admin (Account.is_admin) is cross-tenant and bypasses tenant roles for
  AccountAdminService / TenantAdminService. An API token is capped <= ADMIN.
  GetSharedRun is public (no auth). Canon: models/tenant.proto, the api/ui and
  api/admin services.

  Rule: a caller is allowed iff their effective role in the target tenant is >=
  the action's minimum role (and is_admin passes platform actions).

  Scenario Outline: minimum role required per action
    Given the action <action>
    Then its minimum tenant role is <min_role>

    Examples: read (VIEWER)
      | action                                     | min_role |
      | RunService.GetTestRun                      | VIEWER   |
      | RunService.ListTestRuns                    | VIEWER   |
      | RunService.GetRunMetrics                   | VIEWER   |
      | RunService.CompareRuns                     | VIEWER   |
      | RunService.QueryRunLogs                    | VIEWER   |
      | RunService.StreamTestRunLogs               | VIEWER   |
      | RunService.BuildLogLink                    | VIEWER   |
      | RunService.CreateShareLink                 | VIEWER   |
      | SuiteService.GetSuite                      | VIEWER   |
      | SuiteService.ListSuiteRuns                 | VIEWER   |
      | PresetService.ListPresets                  | VIEWER   |
      | PackageService.ListPackages                | VIEWER   |
      | SettingsService.ListSettingsItems          | ADMIN    |
      | SettingsService.GetSettingsItem            | ADMIN    |
      | CloudInventoryService.FetchQuotas          | ADMIN    |
      | CloudInventoryService.ListNetworkAllocations | ADMIN  |

    Examples: operate (ADMIN)
      | action                                     | min_role |
      | RunService.SubmitTestRun                   | ADMIN    |
      | RunService.CancelTestRun                   | ADMIN    |
      | SuiteService.CreateSuite                   | ADMIN    |
      | SuiteService.LaunchSuiteRun                | ADMIN    |
      | SuiteService.CancelSuiteRun                | ADMIN    |
      | PresetService.CreatePreset                 | ADMIN    |
      | PresetService.UpdatePreset                 | ADMIN    |
      | PresetService.DeletePreset                 | ADMIN    |
      | PresetService.ClonePreset                  | ADMIN    |
      | PackageService.RequestPackageUpload        | ADMIN    |
      | PackageService.DeletePackage               | ADMIN    |
      | WebhookService.CreateWebhook               | ADMIN    |
      | WebhookService.UpdateWebhook               | ADMIN    |
      | WebhookService.DeleteWebhook               | ADMIN    |

    Examples: own-tenant administration (OWNER)
      | action                                     | min_role |
      | SettingsService.SetSettingsItem            | OWNER    |
      | SettingsService.DeleteSettingsItem         | OWNER    |
      | CloudInventoryService.Reconcile            | OWNER    |
      | ApiTokenService.CreateApiToken             | OWNER    |
      | ApiTokenService.RevokeApiToken             | OWNER    |
      | TenantService.AddMemberToTenant            | OWNER    |
      | TenantService.RemoveMemberFromTenant       | OWNER    |

  Scenario: Authenticated, role-free actions
    Given any authenticated account (any tenant role)
    Then AuthService.Me and TenantService.ListMyTenants are allowed (self-scope)
    And AuthService.Login/RefreshTokens/Logout are public auth endpoints

  Scenario: Settings/inventory reads are ADMIN but secrets are masked
    Given an ADMIN reading SettingsService.GetSettingsItem
    Then non-secret values are visible
    But credential values (e.g. Yandex token) are masked in the response

  Scenario: Platform admin bypasses tenant roles cross-tenant
    Given Account.is_admin = true
    Then AccountAdminService.* and TenantAdminService.{Create,Update,Delete}Tenant are allowed across tenants
    And these are NOT available to a non-admin OWNER

  Scenario: GetSharedRun is public
    Given no credentials
    When RunService.GetSharedRun is called with a valid token
    Then it succeeds (the only unauthenticated RPC)

  Scenario: An API token cannot exceed its capped role
    Given an API token with role ADMIN
    Then it can perform VIEWER and ADMIN actions
    But it is denied OWNER actions (settings write, members, tokens) even within its tenant

  @invariant
  Scenario: A request to a tenant the caller is not a member of is denied
    Given an account with no membership in the target tenant_id (and not is_admin)
    When any tenant-scoped action is called
    Then it is denied regardless of the role they hold elsewhere
