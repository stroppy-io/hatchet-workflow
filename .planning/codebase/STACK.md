# Technology Stack

**Analysis Date:** 2026-02-27

## Languages

**Primary:**
- Go 1.25.5 - Backend services (master-worker, edge-worker, run CLI)
- TypeScript ~5.9.3 - Frontend web UI (`web/`)

**Secondary:**
- HCL (Terraform) - Infrastructure-as-code for Yandex Cloud (`internal/domain/deployment/yandex/*.tf`)
- Protocol Buffers - Schema definitions for all domain types (`tools/proto/**/*.proto`)
- Bash - Edge worker bootstrap and deployment scripts (`internal/domain/deployment/scripting/install-edge-worker.sh`)

## Runtime

**Environment:**
- Go: compiled to static Linux/amd64 binaries (CGO_ENABLED=0)
- Node.js: frontend dev/build only; final artifact is static files bundled by Vite

**Package Manager:**
- Go: `go mod` - lockfile: `go.sum` (present)
- Node: `yarn` - lockfile: `web/yarn.lock` (present)

## Frameworks

**Core (Backend):**
- Hatchet v0.77.37 (`github.com/hatchet-dev/hatchet`) - Distributed workflow orchestration engine; all worker logic is defined as Hatchet tasks and workflows
- Echo v4.15.0 (`github.com/labstack/echo/v4`) - HTTP framework (indirect, pulled in by Hatchet)

**Core (Frontend):**
- React 19.2.0 - UI component framework
- Vite 7.3.1 - Build tool and dev server (`web/vite.config.ts`)
- Tailwind CSS 4.1.18 - Utility-first CSS (integrated as Vite plugin)

**Serialization:**
- Protocol Buffers (google.golang.org/protobuf v1.36.11) - wire format for all domain messages
- protoc-gen-validate (envoyproxy) v1.3.0 - proto field validation
- `@bufbuild/protobuf` ^2.11.0 - Frontend protobuf support

**Testing:**
- `github.com/stretchr/testify` v1.11.1 - Go test assertions

**Build/Dev:**
- `github.com/hashicorp/hc-install` v0.9.2 + `github.com/hashicorp/terraform-exec` v0.24.0 - Programmatic Terraform execution
- `github.com/hashicorp/go-version` v1.8.0 - Semantic versioning helpers

## Key Dependencies

**Critical:**
- `github.com/hatchet-dev/hatchet` v0.77.37 - The entire distributed workflow system is built on Hatchet; workers register tasks with it and all orchestration goes through it
- `github.com/docker/docker` v28.5.2 - Docker Engine SDK used directly to spawn/manage containers on edge worker hosts
- `github.com/hashicorp/terraform-exec` v0.24.0 - Used to programmatically run `terraform apply`/`destroy` for Yandex Cloud VM provisioning
- `github.com/valkey-io/valkey-go` v1.0.71 - Valkey (Redis-compatible) client used for distributed network CIDR reservation and locking

**Infrastructure:**
- `github.com/aws/aws-sdk-go-v2` v1.41.1 + `service/s3` v1.96.0 - S3-compatible object storage client (used with custom endpoints, e.g. MinIO/Yandex S3)
- `github.com/jackc/pgx/v5` v5.8.0 - PostgreSQL driver (indirect via Hatchet)
- `github.com/rs/zerolog` v1.34.0 - Zerolog logger (adapter for Hatchet's logger interface)
- `go.uber.org/zap` v1.27.0 - Primary application logger
- `github.com/sourcegraph/conc` v0.3.1 - Structured concurrency pools for parallel workflow fan-out
- `github.com/cenkalti/backoff/v4` v4.3.0 - Exponential backoff for retries (Docker image pulls, workflow polling)
- `github.com/samber/lo` v1.52.0 - Generic Go utility helpers
- `github.com/oklog/ulid/v2` v2.1.1 - Monotonic sortable unique IDs for run tracking
- `github.com/google/uuid` v1.6.0 - UUIDs for instance identity

**Frontend:**
- `@xyflow/react` ^12.10.0 - React Flow library for topology graph visualization
- `framer-motion` ^12.34.0 - Animation library
- `lucide-react` ^0.563.0 - Icon set
- `js-yaml` ^4.1.1 - YAML serialization for manifest output
- `clsx` + `tailwind-merge` - Conditional className utilities (shadcn/ui pattern)

## Configuration

**Environment:**
- All runtime configuration read from environment variables at startup; no config files (except optional `.env` loaded by Makefile)
- Key backend vars:
  - `HATCHET_CLIENT_TOKEN` - Required for all workers to connect to Hatchet
  - `HATCHET_CLIENT_HOST_PORT` or `HATCHET_CLIENT_SERVER_URL` - Hatchet engine address
  - `HATCHET_CLIENT_TLS_STRATEGY` - TLS mode (`none` in dev)
  - `VALKEY_URL` - Valkey connection string (`redis://...`)
  - `S3_URL`, `S3_REGION`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY` - S3 config
  - `YC_TOKEN`, `YC_CLOUD_ID`, `YC_FOLDER_ID`, `YC_ZONE` - Yandex Cloud credentials (injected per-deploy)
  - `TERRAFORM_EXEC_PATH` - Path to terraform binary (default `/usr/local/bin/terraform`)
  - `LOG_MOD`, `LOG_LEVEL`, `LOG_MAPPING`, `LOG_SKIP_CALLER` - Logging config
  - `HATCHET_EDGE_WORKER_NAME`, `HATCHET_EDGE_ACCEPTABLE_TASKS` - Edge worker identity
  - `DOCKER_NETWORK_NAME`, `EDGE_WORKER_DOCKER_IMAGE` - Container deployment settings

**Build:**
- `deployments/docker/master-worker.Dockerfile` - Multi-stage build; embeds Terraform 1.14.5 binary; base `distroless/static-debian11`
- `deployments/docker/edge-worker.Dockerfile` - Multi-stage build; base `ubuntu:22.04` (needs apt/curl/bash for bootstrap)
- Build flags inject `Version` and `ServiceName` at link time into `internal/core/build/build.go`
- Terraform downloaded from `hashicorp-releases.yandexcloud.net` (Yandex mirror) in Dockerfile

## Platform Requirements

**Development:**
- Go 1.25.5+
- Node.js + Yarn (for `web/`)
- Docker + Docker Compose (for `make up-infra` / `make up-dev`)
- Running Hatchet stack (`docker-compose.infra.yaml`)
- Running Valkey (`docker-compose.infra.yaml`)
- `zap-pretty` optional (for `make run-master-worker` log prettifier)

**Production:**
- master-worker: Docker container or binary; needs Docker socket mounted (`/var/run/docker.sock`) and network host mode
- edge-worker: Deployed to remote VMs (Yandex Cloud) as a systemd service via `install-edge-worker.sh`; distributed as GitHub Release binary
- Hatchet engine + PostgreSQL 15.6 + RabbitMQ 3 + Valkey (all via Docker Compose)

---

*Stack analysis: 2026-02-27*
