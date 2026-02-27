# External Integrations

**Analysis Date:** 2026-02-27

## APIs & External Services

**Workflow Orchestration:**
- Hatchet - Distributed task/workflow engine; all workers connect to a Hatchet engine via gRPC
  - SDK/Client: `github.com/hatchet-dev/hatchet` v0.77.37
  - Auth: `HATCHET_CLIENT_TOKEN` (env var)
  - Connection: `HATCHET_CLIENT_HOST_PORT` or `HATCHET_CLIENT_SERVER_URL`
  - TLS: `HATCHET_CLIENT_TLS_STRATEGY` (`none` in dev, cert-based in prod)
  - Workers register tasks; master-worker dispatches suites; edge-worker executes per-task workloads

**Container Runtime:**
- Docker Engine (local socket) - Used by edge-worker to pull images, create networks, and run containers
  - Client: `github.com/docker/docker` v28.5.2 via `dockerClient.FromEnv` (reads `DOCKER_HOST`/socket)
  - Registry auth: reads `~/.docker/config.json` or `$DOCKER_CONFIG/config.json` at pull time
  - Relevant files: `internal/domain/edge/containers/docker.go`

**Cloud Provider (Infrastructure Provisioning):**
- Yandex Cloud - VMs provisioned via Terraform using the `yandex-cloud/yandex` provider
  - Client: Terraform CLI (hashicorp-releases.yandexcloud.net mirror) via `github.com/hashicorp/terraform-exec`
  - Auth env vars: `YC_TOKEN`, `YC_CLOUD_ID`, `YC_FOLDER_ID`, `YC_ZONE`
  - Terraform files: `internal/domain/deployment/yandex/*.tf`
  - Terraform mirror: `https://terraform-mirror.yandexcloud.net/` (configured in `tfrcTemplate` in `internal/infrastructure/terraform/actor.go`)
  - Working directory: `/tmp/stroppy-terraform/{deployment-id}/`

**GitHub Releases (Edge Worker Distribution):**
- GitHub API - Used by `install-edge-worker.sh` to discover and download the latest edge-worker binary
  - Endpoint: `https://api.github.com/repos/stroppy-io/hatchet-workflow/releases`
  - Download: `https://github.com/stroppy-io/hatchet-workflow/releases/download/{tag}/edge-worker`
  - Used during VM bootstrap via cloud-init user-data

## Data Storage

**Databases:**
- PostgreSQL 15.6 - Used exclusively by Hatchet engine (not by application code directly)
  - Connection: `DATABASE_URL` (set in `docker-compose.infra.yaml`)
  - Client: `github.com/jackc/pgx/v5` (indirect, Hatchet's internal driver)
  - Note: Application code does not query Postgres directly; Hatchet owns this database

- PostgreSQL (managed by stroppy on edge VMs) - The **target** database being benchmarked; deployed as containers on edge worker hosts
  - Deployment modes: standalone (`postgres:17`), Patroni HA cluster (3 nodes), PgBouncer pooler
  - Compose spec: `internal/domain/edge/postgres/embed/postgres-docker-compose.yaml`
  - Supports etcd DCS (1, 3, or 5 node clusters) via `quay.io/coreos/etcd:v3.5.17`

**Key-Value / Cache:**
- Valkey (Redis-compatible) - Used for distributed network CIDR allocation and distributed locking
  - Connection: `VALKEY_URL` env var (format: `redis://user:pass@host:port`)
  - Client: `github.com/valkey-io/valkey-go` v1.0.71 with OpenTelemetry instrumentation (`valkeyotel`)
  - Distributed locking: `valkeylock` (prefix `valkeylock:`, 5s key validity)
  - Data model: IP subnet reservations stored as Redis Sets keyed by `network:{target}:{name}`
  - Relevant files: `internal/infrastructure/valkey/`, `internal/domain/managers/network.go`

**File Storage:**
- S3-compatible object storage - Used for storing test artifacts/results
  - Client: `github.com/aws/aws-sdk-go-v2/service/s3` v1.96.0
  - Connection: `S3_URL` (custom endpoint, e.g. MinIO or Yandex Object Storage), `S3_REGION`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`
  - Configured with `UsePathStyle: true` for non-AWS endpoints
  - OpenTelemetry instrumented via `otelaws` middleware
  - Relevant files: `internal/infrastructure/s3/s3.go`, `internal/infrastructure/s3/config.go`
  - Note: MinIO is commented out in `docker-compose.infra.yaml`; production uses external S3

## Authentication & Identity

**Auth Provider:**
- Hatchet token auth - Simple bearer token (`HATCHET_CLIENT_TOKEN`) for all gRPC connections to Hatchet engine
  - Implementation: passed to `v0Client.WithToken(token)` when constructing the Hatchet client
- Docker registry auth - Local `config.json` file parsed at image pull time; supports base64-encoded `auth`, `username`/`password`, and `identityToken` formats
  - Implementation: `registryAuthForImage()` in `internal/domain/edge/containers/docker.go`
- Yandex Cloud auth - IAM token (`YC_TOKEN`) passed as Terraform environment variable; cloud and folder scoped via `YC_CLOUD_ID` / `YC_FOLDER_ID`

## Monitoring & Observability

**Tracing:**
- OpenTelemetry - Tracing instrumentation on S3 and Valkey clients
  - S3: `go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws`
  - Valkey: `github.com/valkey-io/valkey-go/valkeyotel`
  - HTTP: `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`
  - Exporter: `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` (OTLP HTTP)
  - Metrics SDK: `go.opentelemetry.io/otel/sdk/metric`

**Error Tracking:**
- Sentry (`github.com/getsentry/sentry-go` v0.42.0) - Present as an indirect dependency (via Hatchet); not directly configured in application code

**Logs:**
- `go.uber.org/zap` v1.27.0 - Structured JSON logs in production mode, human-readable in development
- `github.com/rs/zerolog` v1.34.0 - Used as adapter for Hatchet's logger interface
- Log config: `LOG_MOD` (production|development), `LOG_LEVEL` (debug|info|warn|error), `LOG_MAPPING` (per-logger level overrides), `LOG_SKIP_CALLER`

**Metrics (on edge VMs):**
- prometheus-postgres-exporter (`prometheuscommunity/postgres-exporter`) - PostgreSQL metrics
- prometheus-pgbouncer-exporter - PgBouncer metrics
- node-exporter (`prom/node-exporter`) - Host system metrics
- All defined in `internal/domain/edge/postgres/embed/postgres-docker-compose.yaml` as optional profiles

## CI/CD & Deployment

**Hosting:**
- Docker Compose - All services run in containers for both local dev and production
- Yandex Cloud Compute - Remote edge VMs provisioned via Terraform

**CI Pipeline:**
- Not detected (no CI config files found)

**Release Process:**
- GitHub Releases (`gh release create`) used for edge-worker binary distribution
- `make release-dev-edge` builds binary with version stamp and uploads as GitHub pre-release
- Edge workers installed on VMs via `install-edge-worker.sh` which fetches from GitHub Releases API

## Environment Configuration

**Required env vars (master-worker):**
- `HATCHET_CLIENT_TOKEN` - Hatchet connection token
- `HATCHET_CLIENT_HOST_PORT` - Hatchet engine gRPC address
- `VALKEY_URL` - Valkey connection string
- `HATCHET_EDGE_WORKER_SSH_KEY` - SSH key for deploying edge workers
- `HATCHET_EDGE_WORKER_USER_NAME` - SSH username for edge VM access
- `DOCKER_NETWORK_NAME` - Docker network for container isolation (default `stroppy-net`)
- `EDGE_WORKER_DOCKER_IMAGE` - Docker image for local edge workers (default `stroppy-edge-worker:latest`)

**Required env vars (edge-worker):**
- `HATCHET_CLIENT_TOKEN` - Hatchet connection token
- `HATCHET_CLIENT_HOST_PORT` or `HATCHET_CLIENT_SERVER_URL` - Hatchet engine address
- `HATCHET_EDGE_WORKER_NAME` - Unique worker name
- `HATCHET_EDGE_ACCEPTABLE_TASKS` - Comma-separated list of task IDs this worker should handle
- `S3_URL`, `S3_REGION`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY` - Object storage

**Required env vars (Yandex Cloud deployments):**
- `YC_TOKEN`, `YC_CLOUD_ID`, `YC_FOLDER_ID`, `YC_ZONE` - Injected per-deployment operation

**Required env vars (infra stack):**
- `POSTGRES_PASSWORD` - Hatchet's PostgreSQL password
- `VALKEY_PASSWORD` - Valkey auth password

**Secrets location:**
- `.env` file in repo root (gitignored); loaded by `Makefile` via `include .env`
- Edge worker env file: `/etc/hatchet/edge-worker.env` on remote VMs

## Webhooks & Callbacks

**Incoming:**
- Not detected

**Outgoing:**
- Hatchet task status polling - master-worker polls `c.Runs().Get(ctx, runID)` with exponential backoff (500ms initial, 5s max) to detect workflow completion; implemented in `internal/domain/workflows/test/test-suit.go`

## Message Queue

**RabbitMQ 3:**
- Used by Hatchet engine internally for task queue messaging
- Connection: `SERVER_MSGQUEUE_RABBITMQ_URL` (`amqp://...`)
- Not directly accessed by application code; owned by Hatchet infrastructure

---

*Integration audit: 2026-02-27*
