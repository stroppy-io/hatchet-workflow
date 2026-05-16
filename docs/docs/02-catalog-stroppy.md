# Plan 02: Catalog + Stroppy Services

## Overview

Plan 02 delivers the catalog tier and stroppy helper service, enabling users to manage reusable infrastructure templates and invoke stroppy workload analysis. Four domain services handle database/workload presets, packages, and tenant settings. The stroppy service acts as a GitHub release proxy with valkey-backed caching and wraps the stroppy binary for probe and configuration preview operations. S3 integration enables reproducible package distribution. Together, these components establish a foundation for the test run engine in Plan 03.

**Commits:** `4f9cabd` → `7f5f1e3` on branch `refactor`.

## Services Added

### DatabasePresetService
CRUD operations for database topology templates. Supports cloning. Stores shape (node count, replica flags), tuning parameters, and topology variant (postgres, mysql, picodata, ydb, external).

- **Entry point:** `internal/domain/services/catalog/database_presets.go` — `CreateDatabasePreset`, `GetDatabasePreset`, `ListDatabasePresets`, `UpdateDatabasePreset`, `DeleteDatabasePreset`, `CloneDatabasePreset`.
- **Proto:** `cloud.v1.catalog.DatabasePreset` — tied to `catalog/database.proto`.

### WorkloadPresetService
CRUD for workload definition templates. Includes script, SQL, driver config, and scale parameters.

- **Entry point:** `internal/domain/services/catalog/workload_presets.go` — `CreateWorkloadPreset`, `GetWorkloadPreset`, `ListWorkloadPresets`, `UpdateWorkloadPreset`, `DeleteWorkloadPreset`, `CloneWorkloadPreset`.
- **Proto:** `cloud.v1.catalog.WorkloadPreset`.

### PackageService
CRUD for software packages (database binaries, workload tools). Binary payload cached in S3 via `BinaryDownload.CachedRef`; upload destination provided by `UploadPackageBinary()`.

- **Entry point:** `internal/domain/services/catalog/packages.go` — `CreatePackage`, `GetPackage`, `ListPackages`, `UpdatePackage`, `DeletePackage`, `ClonePackage`, `UploadPackageBinary` (via storage interface).
- **Proto:** `cloud.v1.catalog.Package`.
- **S3 integration:** `internal/infrastructure/s3/client.go` — `PutObject`, `PresignGet`, `Delete` (aws-sdk-go-v2).

### SettingsService
Key-value store for tenant configuration. Typed parts and keys (e.g., `PART_YANDEX_CLOUD / KEY_YANDEX_CLOUD_TOKEN`). Upsert semantics (insert or replace).

- **Entry point:** `internal/domain/services/catalog/settings.go` — `SetSetting`, `GetSetting`, `ListSettings`, `SetSettingMany`.
- **Proto:** `cloud.v1.catalog.SettingsItem` — typed `Value` oneof (string, int64, list).

### StroppyService
Binary version proxy and workload analysis helper. Caches GitHub releases and commits for 5 minutes via valkey. Wraps stroppy binary for probe (subprocess) and preview (JSON render).

- **Entry point:** `internal/domain/services/stroppy/service.go` — `ListStroppyVersions`, `ListStroppyCommits`, `ProbeStroppyConfig`, `PreviewStroppyConfig`.
- **Versions cache:** `versions.go` — queries GitHub API, marshals to proto, stores in valkey with TTL.
- **Probe runner:** `probe.go` — invokes `stroppy probe` subprocess, collects JSON output and optional human-readable report.
- **Config preview:** `preview.go` — renders stroppy-config.json without spawning subprocess.
- **Proto:** `cloud.v1.stroppy.StroppyVersionList`, `StroppyCommitList`, `ProbeStroppyConfigRequest/Response`, `PreviewStroppyConfigRequest/Response`.

## Infrastructure

### Stroppy Binary Runner
`internal/infrastructure/stroppybin/`:
- **`binary.go`:** `Runner` struct — resolves binary path by version, caches lookup, executes via `os/exec.Command`.
- **`probe.go`:** `RunProbe()` — builds temp dir, writes `stroppy-config.json` + workload files, invokes `stroppy probe -f <cfg> -o json`, returns stdout and optional human output.
- **`preview.go`:** `RenderConfig()` — renders JSON without subprocess.

### S3 Client
`internal/infrastructure/s3/client.go`:
- AWS SDK v2 wrapper. Reads credentials from env vars (`S3_ACCESS_KEY_ENV`, `S3_SECRET_KEY_ENV`). Presigned URLs for downloads (15min default TTL).
- Methods: `PutObject(key, body, contentType)`, `PresignGet(key, ttl)`, `Delete(key)`.

### Valkey JSON Cache
`internal/infrastructure/valkey/cache.go`:
- Typed generic `JSONCache[T]` — Get/Set with JSON marshaling + TTL.
- Used by stroppy service for versions and commits (5min TTL).
- Allows in-memory impl for tests via `NewInMemory()`.

## Transport

### ConnectRPC Handlers
Mounted in `internal/transport/connect/server.go`:
- **`catalog.go`:** `CatalogHandler` — dispatches to all four catalog services. Tenant and caller resolved from context middleware. Bypass list extended to include catalog endpoints.
- **`stroppy.go`:** `StroppyHandler` — dispatches to stroppy service. Versions/commits endpoints added to bypass list (no tenant check).

## CLI

New subcommands registered in `cmd/stroppy-cloud/cmd_cli/root.go`:

```
preset database {list,get,delete,clone}
preset workload {list,get,delete,clone}
package {list,get,delete,clone}
settings {list,set}
probe
version {list,commits}
```

Implementations:
- `preset.go` — database/workload list/get/delete/clone.
- `package.go` — package list/get/delete/clone.
- `settings.go` — settings list/set (read-only on this plan; upload deferred).
- `probe.go` — executes `stroppy probe` via HTTP RPC to server (requires binary deployment on server).
- `version.go` — list available stroppy versions and recent commits (cached on server).

Note: Package binary upload/download subcommands deferred to Plan 03 (no PackageService upload RPC yet).

## Configuration

Reference `deployments/local/server/config.yaml`:

```yaml
s3:
  endpoint: "http://127.0.0.1:9000"       # MinIO for local dev
  bucket: "stroppy"
  region: "us-east-1"
  access_key_env: "S3_ACCESS_KEY"         # Env var name for credentials
  secret_key_env: "S3_SECRET_KEY"

stroppy:
  default_version: "v4.1.0"
  binaries_dir: "/var/lib/stroppy/bin"    # Where stroppy binary versions live
  releases_url: "https://api.github.com/repos/stroppy-io/stroppy/releases"
  commits_url: "https://api.github.com/repos/stroppy-io/stroppy/commits"
```

Environment variables required at runtime:
- `S3_ACCESS_KEY` — MinIO/AWS access key.
- `S3_SECRET_KEY` — MinIO/AWS secret key.

## Testing

### Integration Tests
File: `internal/domain/services/catalog/integration_test.go` (6 tests)

```bash
go test ./internal/domain/services/catalog/... -v
```

Tests cover:
- DatabasePreset CRUD and clone.
- WorkloadPreset CRUD and clone.
- Package CRUD without binary (APT source).
- SettingsItem upsert and retrieval.

Stroppy integration: `internal/domain/services/stroppy/integration_test.go` — versions/commits cache TTL, preview rendering. Probe subprocess tests deferred (require external stroppy binary).

### Probe Tests
Execution deferred. Requires stroppy binary at `/tmp/stroppy-test-binaries/stroppy-dev`. Placeholder target added:

```bash
make stroppy-bin-fetch  # Downloads/caches stroppy binary for tests
```

## Known Limitations

- **Binary upload/download:** CLI subcommands and PackageService RPC for upload deferred to Plan 03. Current code allows S3 presigned URLs but no end-to-end flow.
- **Probe binary dependency:** Probe test execution requires external stroppy binary at configured binaries_dir. Cannot run in isolation.
- **Proto field documentation:** Individual proto message fields lack domain documentation. Defer to proto source files for full schema.
- **GitHub API rate limits:** Version/commit listing hits GitHub API; cached for 5 minutes. Requests without auth are subject to public rate limits (60/hour).

## Next Plan

**Plan 03 — System Engine + Workers:** DAG execution, node worker scheduler, task recovery after restart, webhook delivery.
