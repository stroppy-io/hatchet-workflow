# Web table pages and API scope

This file fixes the frontend page inventory before implementation. The web UI
must not invent client-side API contracts: every table page below maps to a
generated proto RPC. If a required list/query RPC is missing or too weak for a
shadcn/TanStack data table, the proto must be changed and regenerated before the
page is implemented.

All tenant-scoped web routes use `/t/<tenant_id>/...`, and every tenant-scoped
RPC carries the same `tenant_id`.

## Required pages

Public and shell:

| Route | Page | Table | API |
| --- | --- | --- | --- |
| `/login` | Login | no | `AuthService.Login`, `RefreshTokens`, `Me`, `Logout` |
| `/t/<tenant_id>` | Tenant entry redirect | no | `TenantService.ListMyTenants` |
| `/t/<tenant_id>/wizard` | Authoring wizard | no | `AuthoringService.*`, `PresetService.ListPresets`, `RunService.SubmitTestRun` |

Viewer pages:

| Route | Page | Table | Current API |
| --- | --- | --- | --- |
| `/t/<tenant_id>/runs` | Test runs | yes | `RunService.ListTestRuns` |
| `/t/<tenant_id>/runs/:run_id` | Run details | no | `RunService.GetTestRun`, logs, metrics, share |
| `/t/<tenant_id>/presets` | Presets catalog | yes | `PresetService.ListPresets` |
| `/t/<tenant_id>/suites` | Suites | yes | gap: only `SuiteService.ListSuiteRuns` exists |
| `/t/<tenant_id>/suite-runs/:suite_run_id` | Suite run details | no | `SuiteService.GetSuiteRun` |

Admin pages:

| Route | Page | Table | Current API |
| --- | --- | --- | --- |
| `/t/<tenant_id>/packages` | Package catalog | yes | `PackageService.ListPackages` |
| `/t/<tenant_id>/webhooks` | Webhooks | yes | `WebhookService.ListWebhooks` |
| `/t/<tenant_id>/settings` | Tenant settings | yes | `SettingsService.ListSettingsItems` |
| `/t/<tenant_id>/inventory` | Cloud inventory | yes | `CloudInventoryService.FetchQuotas`, `ListNetworkAllocations` |

Owner pages:

| Route | Page | Table | Current API |
| --- | --- | --- | --- |
| `/t/<tenant_id>/tokens` | API tokens | yes | `ApiTokenService.ListApiTokens` |
| `/t/<tenant_id>/members` | Tenant members | yes | partial: `TenantService.ListMyTenants` returns embedded members for current account's tenants |

Platform admin pages:

| Route | Page | Table | Current API |
| --- | --- | --- | --- |
| `/t/<tenant_id>/platform/accounts` | Accounts | yes | gap: `AccountAdminService` has create/update/delete only |
| `/t/<tenant_id>/platform/tenants` | Tenants | yes | gap: `TenantAdminService` has create/update/delete only |
| `/t/<tenant_id>/platform/settings` | Platform settings | no | `PlatformAdminService.GetPlatformSettings`, `SetPlatformSettings` |

## Table feature contract

All table pages should use one reusable shadcn/TanStack wrapper, not page-local
table renderers. The wrapper must support:

- column definitions supplied by the page;
- sorting;
- text filtering/search;
- faceted/status filtering where the API exposes typed filters;
- column visibility;
- row selection;
- row actions;
- pagination;
- loading, empty, and error states;
- stable action links under `/t/<tenant_id>/...`.

The protocol must expose enough query shape for server-backed versions of those
features. Client-only filtering/sorting is acceptable only for small embedded
lists, not for authoritative entity catalogs.

## API fit by table page

| Page | Current fit | Required before frontend implementation |
| --- | --- | --- |
| Runs | partial | `ListTestRunsRequest` has `status`, `page_size`, `page_token`; response lacks `PageInfo`, total, typed sort, and text/name filters. |
| Presets | partial | `ListPresetRequest` has required `kind`; page can make three calls for Workload/Database/Test, but a single catalog table needs optional/repeated kind, pagination, search, tags, and sort. |
| Suites | gap | Add `ListSuites` for suite definitions. `ListSuiteRuns` is not a replacement for the `/suites` catalog. |
| Suite runs | partial | `ListSuiteRunsRequest` mirrors runs and has the same pagination/sort/filter gaps. |
| Packages | partial | `ListPackagesRequest` has only `tenant_id`; needs pagination, search, db kind/version filters, builtin/custom filter, and sort. |
| Webhooks | partial | `ListWebhooksRequest` has only `tenant_id`; needs pagination, enabled/event filters, search by URL, and sort. |
| Settings | partial | `ListSettingsItemsRequest` has only `tenant_id`; needs at least part/key filtering and clear masked-secret response semantics for list rows. |
| Inventory allocations | partial | `ListNetworkAllocationsRequest` has provider + page fields; response lacks `PageInfo`, typed sort, CIDR/zone/lease filters. |
| Inventory quotas | partial | `FetchQuotas` is snapshot-style, not a pageable table; ok for a small quota table if quota list is bounded. |
| API tokens | partial | `ListApiTokensRequest` has only `tenant_id`; needs pagination, role/expiry filters, and sort. |
| Members | gap | Add tenant-scoped `ListTenantMembers` or `GetTenant` by id. `ListMyTenants` is an identity bootstrap API, not a member-management table contract. |
| Platform accounts | gap | Add `AccountAdminService.ListAccounts` with platform-admin query contract. |
| Platform tenants | gap | Add `TenantAdminService.ListTenants` with platform-admin query contract. |

## Proto direction to discuss before changing

Recommended shape for table-grade list RPCs:

- keep per-model typed request messages;
- reuse only `CommonQuery.Page` / `PageInfo` if we keep them;
- do not use stringly `CommonQuery.Filter` for UI entity lists;
- add explicit optional typed filters per model;
- add explicit sortable fields as enums per model, not arbitrary strings;
- every list response should include repeated rows plus `PageInfo`.

No frontend table page should be implemented against a guessed contract. The
next step is to agree which proto gaps to close first, regenerate, and only then
wire the shadcn data-table wrapper to the generated clients.
