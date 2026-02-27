# Codebase Structure

**Analysis Date:** 2026-02-27

## Directory Layout

```
stroppy-cloud/                         # Repo root (module: github.com/stroppy-io/hatchet-workflow)
├── cmd/                               # Go binary entry points
│   ├── edge-worker/                   # Edge worker binary (runs on provisioned VMs)
│   │   └── edge.go
│   ├── master-worker/                 # Master orchestrator binary
│   │   └── master.go
│   └── run/                           # One-shot CLI test runner
│       └── main.go
├── deployments/
│   └── docker/                        # Dockerfiles for each binary
│       ├── edge-worker.Dockerfile
│       └── master-worker.Dockerfile
├── examples/
│   ├── database-topologies/           # YAML test configs for various Postgres topologies
│   └── nightly/                       # Example nightly workflow config
├── internal/                          # All application code (not importable externally)
│   ├── core/                          # Cross-cutting utilities
│   │   ├── build/                     # Version + service name linker variables
│   │   ├── consts/                    # Typed constant helpers (EnvKey, ConstValue, etc.)
│   │   ├── defaults/                  # Default value helpers
│   │   ├── envs/                      # Env var slice utilities
│   │   ├── hatchet-ext/               # Generic WTask/PTask wrappers for Hatchet
│   │   ├── ids/                       # ULID generation, RunId type
│   │   ├── ips/                       # Subnet/IP allocation helpers
│   │   ├── logger/                    # Zerolog/Zap wrapper + env-based configuration
│   │   ├── protoyaml/                 # YAML ↔ protobuf marshaling
│   │   ├── shutdown/                  # Graceful shutdown signal handler
│   │   ├── types/                     # Shared type aliases
│   │   ├── uow/                       # Unit-of-work LIFO rollback
│   │   └── utils/                     # General Go utilities (embed filename listing)
│   ├── domain/                        # Business logic
│   │   ├── database/                  # Database domain helpers
│   │   ├── deployment/                # Deployment service interface + backends
│   │   │   ├── docker/                # Docker engine backend
│   │   │   ├── scripting/             # Cloud-init / shell script generation
│   │   │   ├── yandex/                # Terraform + Yandex Cloud backend (embeds .tf files)
│   │   │   └── deployment.go          # Registry (strategy pattern over target enum)
│   │   ├── edge/                      # Edge worker helpers
│   │   │   ├── containers/            # Docker container lifecycle on edge VMs
│   │   │   ├── postgres/              # Embedded Postgres config/SQL
│   │   │   └── edge.go                # Task identifier encoding/decoding, worker naming
│   │   ├── managers/                  # Managers wrapping infrastructure with domain logic
│   │   │   └── network.go             # NetworkManager: Valkey-backed CIDR reservation
│   │   ├── provision/                 # Placement planning and deployment orchestration
│   │   │   ├── provision.go           # ProvisionerService: AcquireNetwork → BuildPlacement → DeployPlan
│   │   │   └── postgres-placement-builder.go  # Postgres-specific VM placement logic
│   │   └── workflows/                 # Hatchet DAG definitions
│   │       ├── edge/                  # Edge-side task functions (containers, stroppy)
│   │       │   ├── containers.go      # SetupContainersTask
│   │       │   └── stroppy.go         # InstallStroppy, RunStroppyTask
│   │       └── test/                  # Master-side workflow definitions
│   │           ├── test-run.go        # TestRunWorkflow (full test lifecycle DAG)
│   │           ├── test-suit.go       # TestSuiteWorkflow (parallel multi-test runner)
│   │           ├── deps.go            # Deps struct: Valkey + ProvisionerService wiring
│   │           └── utils.go           # waitMultipleWorkersUp helper
│   ├── infrastructure/                # External system adapters
│   │   ├── s3/                        # AWS S3 client wrapper
│   │   ├── terraform/                 # Terraform executor (init/apply/destroy/output)
│   │   │   └── actor.go               # Actor struct with concurrent workdir map
│   │   └── valkey/                    # Valkey client factory + distributed locker
│   └── proto/                         # Generated protobuf Go code (DO NOT EDIT)
│       ├── database/                  # Database template types (Postgres cluster/instance)
│       ├── deployment/                # Deployment, Network, VM types
│       ├── edge/                      # Edge worker task identifier types
│       ├── provision/                 # Placement, PlacementIntent, DeployedPlacement types
│       ├── settings/                  # Settings, HatchetConnection, DockerSettings, YandexCloudSettings
│       ├── stroppy/                   # Test, TestResult, StroppyCli types
│       ├── validate/                  # Protoc-gen-validate generated code
│       └── workflows/                 # Workflow input/output types (Tasks, Workflows messages)
├── tools/
│   └── proto/                         # Proto source definitions + easyp config
│       ├── database/                  # database.proto, postgres.proto, common.proto
│       ├── deployment/                # deployment.proto
│       ├── edge/                      # edge.proto
│       ├── provision/                 # provision.proto
│       ├── settings/                  # settings.proto
│       ├── stroppy/                   # test.proto
│       ├── workflows/                 # workflows.proto, tasks.proto
│       └── easyp.yaml                 # Proto generation config (easyp/buf)
├── web/                               # React + TypeScript frontend
│   ├── public/                        # Static assets
│   └── src/
│       ├── assets/                    # Images, icons
│       ├── components/                # React feature components
│       │   ├── ui/                    # Primitive UI components (Input, Select, Stepper, etc.)
│       │   ├── SettingsEditor.tsx
│       │   ├── TestWizard.tsx
│       │   └── TopologyCanvas.tsx
│       ├── proto/                     # Generated TypeScript protobuf bindings (DO NOT EDIT)
│       │   ├── database/
│       │   ├── deployment/
│       │   ├── edge/
│       │   ├── provision/
│       │   ├── settings/
│       │   └── stroppy/
│       ├── App.tsx                    # Root app component + state
│       ├── index.css                  # Global CSS (Tailwind)
│       └── main.tsx                   # React entry point
├── .planning/                         # GSD planning documents
│   └── codebase/
├── docker-compose.infra.yaml          # Hatchet engine + Valkey + Postgres + RabbitMQ
├── docker-compose.dev.yaml            # master-worker + edge-worker containers for local dev
├── go.mod                             # Go module: github.com/stroppy-io/hatchet-workflow
├── go.sum
├── Makefile                           # Build, run, release targets
└── README.md
```

## Directory Purposes

**`cmd/`:**
- Purpose: Binary entry points only; minimal logic — read env vars, wire dependencies, start Hatchet workers
- Contains: One `package main` per binary
- Key files: `cmd/master-worker/master.go`, `cmd/edge-worker/edge.go`, `cmd/run/main.go`

**`internal/core/`:**
- Purpose: Reusable utilities with no domain knowledge; safe to import anywhere inside `internal/`
- Contains: Logger, ULID IDs, IP math, protobuf-YAML, shutdown hooks, generic Hatchet task adapters
- Key files: `internal/core/hatchet-ext/task.go` (WTask/PTask generics), `internal/core/uow/uow.go` (rollback)

**`internal/domain/`:**
- Purpose: All business logic; organised by subdomain
- Contains: Workflow DAG definitions, provisioner service, deployment backends, network manager, edge helpers
- Key files: `internal/domain/provision/provision.go`, `internal/domain/deployment/deployment.go`, `internal/domain/workflows/test/test-run.go`

**`internal/infrastructure/`:**
- Purpose: Thin adapters to external services; no business logic
- Contains: Terraform runner, Valkey locker factory, S3 client
- Key files: `internal/infrastructure/terraform/actor.go`

**`internal/proto/`:**
- Purpose: Generated code only — never edit directly; source of truth is `tools/proto/*.proto`
- Contains: `.pb.go` (struct types), `.pb.validate.go` (validation), `.pb.json.go` (JSON marshaling) for all domains
- Generated: Yes (via `easyp generate` configured in `tools/proto/easyp.yaml`)
- Committed: Yes

**`tools/proto/`:**
- Purpose: Proto source definitions and generation tooling
- Contains: `*.proto` files, `easyp.yaml` (generates Go to `internal/proto/`, TypeScript to `web/src/proto/`)
- Committed: Yes

**`web/src/proto/`:**
- Purpose: Generated TypeScript protobuf bindings for frontend use
- Contains: `*_pb.ts` files generated by `protoc-gen-es`
- Generated: Yes
- Committed: Yes

**`examples/`:**
- Purpose: Sample YAML configurations for test runs; used by `cmd/run --file`
- Contains: Pre-built topology configs for Postgres clusters of varying sizes/configurations; nightly workflow example
- Key files: `examples/database-topologies/instance-basic.yaml`, `examples/nightly/workflow.yaml`

**`deployments/docker/`:**
- Purpose: Multi-stage Dockerfiles for each binary
- Contains: `edge-worker.Dockerfile` (Ubuntu 22.04 runtime), `master-worker.Dockerfile` (distroless + Terraform binary)

## Key File Locations

**Entry Points:**
- `cmd/master-worker/master.go`: Registers test workflows; long-running daemon
- `cmd/edge-worker/edge.go`: Registers edge tasks from env vars; ephemeral per-VM
- `cmd/run/main.go`: One-shot test runner CLI

**Core Workflow Logic:**
- `internal/domain/workflows/test/test-run.go`: Full test lifecycle DAG — 11 tasks
- `internal/domain/workflows/test/test-suit.go`: Parallel test suite runner with `RunMany`
- `internal/domain/workflows/edge/stroppy.go`: `InstallStroppy` and `RunStroppyTask`
- `internal/domain/workflows/edge/containers.go`: `SetupContainersTask`

**Service Layer:**
- `internal/domain/provision/provision.go`: `ProvisionerService` — central orchestration
- `internal/domain/provision/postgres-placement-builder.go`: Postgres-specific VM/container layout
- `internal/domain/deployment/deployment.go`: `Registry` (deployment strategy map)
- `internal/domain/managers/network.go`: Valkey-backed CIDR allocation

**Deployment Backends:**
- `internal/domain/deployment/yandex/yandex.go`: Terraform/Yandex backend
- `internal/domain/deployment/docker/docker.go`: Local Docker backend
- `internal/domain/deployment/scripting/cloud_init.go`: Cloud-init YAML generation

**Infrastructure:**
- `internal/infrastructure/terraform/actor.go`: Terraform lifecycle management
- `internal/infrastructure/valkey/locker.go`: Distributed lock factory

**Proto Sources:**
- `tools/proto/settings/settings.proto`: `Settings`, `HatchetConnection`, `DockerSettings`, `YandexCloudSettings`
- `tools/proto/provision/provision.proto`: `Placement`, `PlacementIntent`, `DeployedPlacement`
- `tools/proto/workflows/workflows.proto`: Workflow input/output types
- `tools/proto/stroppy/test.proto`: `Test`, `TestResult`, `StroppyCli`

**Configuration:**
- `docker-compose.infra.yaml`: Hatchet engine stack (Postgres, RabbitMQ, Valkey, hatchet-engine, hatchet-dashboard)
- `docker-compose.dev.yaml`: Dev environment (master-worker + edge-worker containers)
- `Makefile`: `up-infra`, `up-dev`, `build`, `run-test`, `release-dev-edge` targets
- `tools/proto/easyp.yaml`: Proto code generation config

**Frontend:**
- `web/src/App.tsx`: Root state management and navigation
- `web/src/components/TopologyCanvas.tsx`: Visual cluster topology editor
- `web/src/components/TestWizard.tsx`: Step-by-step test configuration
- `web/src/components/SettingsEditor.tsx`: Hatchet/cloud settings form

## Naming Conventions

**Go Files:**
- Lowercase, hyphen-separated: `test-run.go`, `test-suit.go`, `cloud_init.go`
- Infrastructure and domain files use snake_case or single-word names: `provision.go`, `network.go`
- Test files end in `_test.go`: `postgres_placement_builder_test.go`, `docker_integration_test.go`
- Proto generated files follow pattern `{name}.pb.go`, `{name}.pb.validate.go`, `{name}.pb.json.go`

**Go Packages:**
- `cmd/*`: All `package main`
- Domain packages: short noun names matching directory (`package provision`, `package deployment`, `package edge`, `package managers`)
- `internal/core/hatchet-ext`: `package hatchet_ext` (hyphen converted to underscore)

**Proto Source Files:**
- Lowercase noun names: `settings.proto`, `provision.proto`, `deployment.proto`
- Subdirectory per domain: `tools/proto/{domain}/{name}.proto`

**TypeScript/React Files:**
- PascalCase for components: `TestWizard.tsx`, `TopologyCanvas.tsx`, `SettingsEditor.tsx`
- UI primitives in `components/ui/`: `Input.tsx`, `Select.tsx`, `Stepper.tsx`
- Generated proto bindings end in `_pb.ts`

**Constants:**
- Typed constant wrappers in `internal/core/consts/`: `consts.EnvKey`, `consts.ConstValue`, `consts.DefaultValue`
- Usage: `const ValkeyUrl consts.EnvKey = "VALKEY_URL"`

## Where to Add New Code

**New Hatchet workflow (master-side):**
- Task functions: `internal/domain/workflows/test/`
- Register in worker: `cmd/master-worker/master.go` with `hatchetLib.WithWorkflows(...)`

**New Hatchet task (edge-side):**
- Task function: `internal/domain/workflows/edge/`
- Add new `edge.Task_Kind` variant: `tools/proto/edge/edge.proto`, then regenerate `internal/proto/edge/`
- Register in edge worker switch: `cmd/edge-worker/edge.go`

**New deployment target:**
- Implement `deployment.Service` interface: new subdirectory under `internal/domain/deployment/`
- Register target in `internal/domain/deployment/deployment.go` `NewRegistry`

**New domain service or manager:**
- Service logic: `internal/domain/{subdomain}/`
- Wire in `internal/domain/workflows/test/deps.go`

**New infrastructure adapter:**
- Add under `internal/infrastructure/{name}/`

**New shared proto message:**
- Add to relevant `.proto` in `tools/proto/{domain}/`
- Run `easyp generate` from `tools/proto/` to regenerate `internal/proto/` and `web/src/proto/`

**New frontend component:**
- Feature-level: `web/src/components/{ComponentName}.tsx`
- Primitive UI: `web/src/components/ui/{ComponentName}.tsx`

**New utility (no domain dependency):**
- Add to appropriate subdirectory of `internal/core/`

**New example test configuration:**
- Add YAML to `examples/database-topologies/` following the naming pattern `{description}-{replicas}.yaml`

## Special Directories

**`internal/proto/`:**
- Purpose: Generated Go protobuf code
- Generated: Yes — by `easyp generate` from `tools/proto/`
- Committed: Yes
- Do not edit directly

**`web/src/proto/`:**
- Purpose: Generated TypeScript protobuf bindings
- Generated: Yes — by `easyp generate` (uses `protoc-gen-es`)
- Committed: Yes
- Do not edit directly

**`.planning/codebase/`:**
- Purpose: GSD planning documents produced by `/gsd:map-codebase`
- Generated: By AI tooling
- Committed: Yes

---

*Structure analysis: 2026-02-27*
