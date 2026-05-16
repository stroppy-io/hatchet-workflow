# Admin Subsystem (Plan 07)

## Overview

The Admin subsystem provides cross-tenant operational capabilities for administrators, including tenant and user management, and binary artifact cache administration. It extends iam.Service with cross-tenant methods and introduces BinaryCacheAdminService for artifact lifecycle management (Prewarm, List, Evict). All admin procedures bypass tenant isolation checks via TenantBypass, but rely on regular JWT authentication only—no PlatformAdmin JWT claim middleware is implemented due to missing PlatformRole in the proto IAM schema.

Key design: AdminService delegates cross-tenant operations to extended iam.Service, while BinaryCacheAdminService handles artifact prewarming (skeleton row insertion only, not S3 copy). Admin endpoints are discoverable via ConnectRPC handlers and CLI, but lack role-based access control pending PlatformRole addition to the IAM schema.

## Services

### AdminService
Location: `internal/domain/services/admin/admin_service.go`

Provides cross-tenant administrative operations:
- `ListAllTenants()` — returns all tenants across organization
- `CreateTenant(name, config)` — creates new tenant
- `DeleteTenantHard(id)` — hard-deletes tenant and all associated data
- `ListAllUsers()` — returns all users across all tenants
- `CreateUser(tenant_id, email, password)` — creates user in specified tenant
- `DeleteUser(id)` — deletes user by ID
- `ResetUserPassword(id, new_password)` — resets user password

All operations delegate to extended iam.Service methods and are included in TenantBypass list to override tenant isolation.

### BinaryCacheAdminService
Manages binary artifact cache lifecycle:
- `Prewarm(items []ResolveArtifactRequest)` — inserts skeleton rows into binary_artifacts for each item (name, version, filename, placeholder storage_uri); does not perform S3 copy or validate sources
- `GetArtifact(name, version, filename)` — retrieves single artifact metadata
- `ListArtifacts(filter)` — lists artifacts with optional name/version/filename filtering
- `EvictArtifact(id)` — soft-deletes artifact by setting deleted_at timestamp

## Modified iam.Service

iam.Service is extended with cross-tenant methods:
- `ListAllTenants(ctx) []Tenant` — returns all tenants regardless of context tenant
- `DeleteTenantHard(ctx, id)` — cascades delete to all users and related data
- `DeleteUser(ctx, id)` — deletes user by ID across tenant boundary
- `ResetUserPassword(ctx, id, password)` — updates user password without tenant check

Existing tenant-scoped methods remain unchanged.

## Transport

The admin.go handler implements two ConnectRPC interfaces:
- `adminconnect.AdminServiceHandler` — all admin procedures
- `agentconnect.BinaryCacheAdminServiceHandler` — artifact admin procedures

Both handlers are mounted in server.go. All admin procedure names are added to TenantBypass list in IAM config to allow cross-tenant execution without tenant context.

## SDK + CLI

SDK exposes `Client.Admin` and `Client.BinaryCacheAdmin` interfaces with full CRUD coverage.

CLI subcommands:
- `admin tenant-list` — list all tenants
- `admin tenant-create <name>` — create tenant
- `admin tenant-delete <id>` — hard-delete tenant
- `admin user-list` — list all users
- `admin user-create <tenant-id> <email> <password>` — create user
- `admin user-delete <id>` — delete user
- `admin user-reset-password <id> <password>` — reset user password
- `admin artifact-list [filter]` — list artifacts with optional filtering
- `admin artifact-prewarm <name> <version> <filename> <url>` — insert skeleton row
- `admin artifact-evict <id>` — soft-delete artifact
- `admin artifact-get <name> <version> <filename>` — retrieve artifact metadata

## Known Limitations

- No PlatformAdmin role gating — any authenticated user can call admin endpoints; this is a security debt pending PlatformRole addition to IAM proto
- Prewarm does not perform S3 copy or download; it only inserts skeleton rows with placeholder storage_uri
- No admin integration tests; cross-tenant iam.Service methods are tested via iam test suite only
- No audit logging for admin operations; tenant create/delete/user mutations are not tracked for compliance
- Admin endpoints do not validate organization membership; cross-tenant access control relies solely on TenantBypass configuration

## Next Plan

Plan 08 — Frontend SPA migration to ConnectRPC: TypeScript/React single-page application using generated ConnectRPC client libraries, separate build stack and deployment pipeline.
