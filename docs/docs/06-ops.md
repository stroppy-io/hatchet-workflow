# Ops Subsystem (Plan 06)

## Overview

The Ops subsystem provides operational infrastructure for webhook delivery, quota management, and binary artifact resolution. It includes three core services: WebhookService for managing and testing webhooks synchronously, QuotaService for stubbed quota operations, and BinaryCacheService for resolving and downloading binary artifacts from storage. The subsystem is mounted into the ConnectRPC server via dedicated handlers and CLI commands.

Key design decisions: webhook delivery is synchronous (no async outbox pattern implemented), quota enforcement is not integrated into the test execution pipeline, and the binary cache service provides read-only artifact access via storage URIs. Future plans defer outbox/dispatcher/drain implementation pending webhook_deliveries table addition to the proto schema.

## Services

### WebhookService
Location: `internal/domain/services/ops/webhook_service.go`

Implements CRUD operations on the webhooks table and TestWebhook procedure. TestWebhook performs a synchronous HTTP POST to the webhook URL without outbox buffering, delivery tracking, or retry logic. No webhook_deliveries table is currently defined in the proto schema, preventing implementation of async delivery, HMAC signing, and retry mechanisms.

### QuotaService
A pure stub that returns an empty QuotaList for all queries. The quota_counters table is not defined in the proto schema, preventing quota tracking and enforcement. QuotaService methods are wired but do not integrate with test execution or resource limits.

### BinaryCacheService
Implements Resolve(name, version, filename) which reads a binary_artifacts row and returns ResolveArtifactResponse with DownloadUrl set to the artifact's storage_uri. Supports read-only access to preloaded binary artifacts for test execution.

## Transport

The ops.go handler implements three ConnectRPC interfaces:
- `ops.WebhookService` — webhook CRUD and test procedures
- `ops.QuotaService` — quota list and query procedures
- `agentconnect.BinaryCache` — artifact resolution

All handlers are mounted in server.go alongside other service handlers.

## Wiring

`cmd_server` constructs instances of WebhookService, QuotaService, and BinaryCacheService, passing dependencies (db, logger) to each. The ops handler receives all three services and exposes them via ConnectRPC. This wiring pattern follows the established convention: service → handler → transport layer.

## CLI

Subcommands organized under `webhook` and `quota` namespaces:
- `webhook list` — list all webhooks
- `webhook get <id>` — retrieve webhook by ID
- `webhook create <url>` — create new webhook
- `webhook delete <id>` — delete webhook
- `webhook test <id>` — synchronously test webhook delivery
- `quota list` — list quotas (stub, returns empty)

## Known Limitations

- No webhook_deliveries table defined in proto → no async outbox, no delivery retry, no HMAC signing or request idempotency
- No quota_counters table → QuotaService is a pure stub
- Quota enforcement is not integrated into TestRunService; test execution does not check quotas before running
- Headers column in webhooks table stores text+JSON, not jsonb; special codec in pgcontainer handles serialization
- BinaryCacheService.Resolve only returns storage URIs; actual S3 artifact retrieval occurs client-side
- No webhook delivery filtering by event type or tenant routing

## Next Plan

Plan 07 — Admin Subsystem: cross-tenant operations (ListAllTenants, CreateTenant, DeleteTenantHard) and binary cache administration (Prewarm, ListArtifacts, EvictArtifact).
