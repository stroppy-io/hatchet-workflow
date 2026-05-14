# Protocol Redesign — Summary

Comparison of current proto schema vs the Go implementation it replaces.
Each section: **what changed → where → effect on the system**.

---

## 1. Engine vs Domain separation

**Was:** Go code mixed DAG mechanics with the test domain.

- `internal/core/dag/` — `dag.Graph`, `dag.Node`, `dag.Task interface` (generic engine).
- `internal/domain/run/builder.go` — domain (TestRun) knew about DAG internals.
- `internal/domain/run/state.go` — `State` held `terraformWdId` **plus `*terraform.Actor`** (in-memory pointer, not serialised, lost on restart).

**Now:** `system/` proto package — pure engine.

- `system/dag.proto`: `Dag`, `DagRun`, `NodeRun`, `DagRunStateEntry` + statuses.
- `system/schedule.proto`: generic `Schedule` + cron + hook name.
- No imports from `domain/`.
- `Node.spec = google.protobuf.Any` — engine treats payload as opaque.

**Effect:**

- Engine reusable for non-test workflows (deploy, ETL).
- Engine tests independent of stroppy.
- Domain evolves without touching engine.

---

## 2. Tasks: 29 typed → 8 generic

**Was (Go):**

```
networkTask / machinesTask / teardownTask
pgInstall / pgConfig
mysqlInstall / mysqlConfig
ydbInstall / ydbConfig / ydbInit / ydbStartDB
cockroachInstall / cockroachConfig / cockroachInit
... (29 typed task structs)
```

**Now (proto):**

```
tasks/
  terraform.proto         — all YC infra (network + machines + teardown)
  docker.proto            — all docker infra (same)
  package_install.proto   — all 11 installs, dispatched by EngineKind + Role
  config_apply.proto      — all 11 configs
  one_shot.proto          — cockroach/ydb init
  stroppy_run.proto       — stroppy execution
  test_run_ref.proto      — meta-DAG SUITE → TEST_RUN
```

**Effect:**

- 29 hard-coded Go structs → 8 typed payloads.
- New engine = new `Package` recipe + handler, no proto changes.
- `TerraformTask { op: APPLY | DESTROY }` removes the `terraformWdId` problem from `state.go` → stored as a row in `dag_run_state_entries[key=yc.wd]`, survives restart without `*Actor` pointer.

---

## 3. Agent protocol: 27 actions → 5 primitives

**Was (`internal/domain/agent/protocol.go`):**

```go
type Action string  // 27 values: ActionInstallPostgres, ActionConfigYDB, ...
type Command { ID; Action; Config any }   // untyped JSON
type PostgresClusterConfig, YDBStaticConfig, ... (15+ structs)
```

Cluster bootstrap (Patroni leader election, YDB blobstorage placeholders) lived in `setup_*.go` on the agent.

**Now (`agent/protocol.proto` + `agent/service.proto`):**

```
Action.oneof:
    PutFile | RunShell | SystemctlOp | InstallPackage | WaitForCondition
```

Server-side rendering: controller renders final bytes (peer hostnames substituted), agent stays a dumb shell-executor.

**Effect:**

- Agent surface area: 27 typed actions → 5 verbs.
- Cluster topology logic concentrated server-side, not scattered across `setup_*.go`.
- Mock-agent tests cover 5 verbs, not 27.
- New engine = new `Package` + server renderer. Agent unchanged.

---

## 4. Auth & tenancy: single → multi-tenant

**Was:**

```
Account { id; tenant_id; email; nickname; role }   // one account per tenant
```

A legacy `tenant_members(user_id, tenant_id, role)` table also existed — schema conflict.

**Now:**

```
iam/user.proto:    User         { id; email; nickname }    — global
iam/member.proto:  TenantMember { user_id; tenant_id; role } — N:M
```

Roles: `VIEWER / MEMBER / ADMIN / OWNER` (expanded from 3 to 4).

**Effect:**

- One person = one `User`, can be member of multiple tenants.
- UI tenant-switcher is pure frontend (passes `tenant_id` in each RPC).
- Email/nickname unique globally, not per-tenant.

---

## 5. Database: 7 topology structs → one oneof + External

**Was (Go):**

```go
DatabaseConfig {
    Kind; Postgres *PostgresTopology; MySQL *MySQLTopology;
    Picodata *PicodataTopology; YDB *YDBTopology; ...
}
```

Sizing mixed into topology (`Master MachineSpec`, `Replicas []MachineSpec`).

**Now (`catalog/database.proto`):**

```
Database {
    Kind kind
    oneof variant {
        Postgres { Shape shape; Tuning tuning }
        Mysql { ... }
        ...
        External { endpoint; database; credentials }   // BYOD
    }
}
```

`Shape` = topology only (counts, flags). Sizing pulled out (future `MachineProfile`). `Tuning` = engine config options.

**Effect:**

- BYOD use case (`Database.External`) — previously `ExternalDBConfig` lived on the side. Now a first-class variant.
- Shape reusable across sizing variants.
- MariaDB as `oneof.mariadb = Mysql` — shares MySQL shape.

---

## 6. Quotas: hard-coded → dynamic `resource_id`

**Was:**

```go
TenantQuotas {
    MaxConcurrentCPUs int; MaxConcurrentMemoryMB int;
    MaxConcurrentDiskGB int; MaxConcurrentVMs int;
    MaxConcurrentRuns int; MaxQueueDepth int
}
```

Fixed fields. New limits required recompilation.

**Now (`ops/quota.proto`):**

```
Quota {
    string resource_id;    // "yc.cpus", "yc.memory_gb", "runs.concurrent", ...
    QuotaSource source;    // PLATFORM | TENANT | PROVIDER
    int64 limit; int64 used; int64 available
}
```

Plus `QuotaService.RefreshQuotas` queries the YC API.

**Effect:**

- New limit = INSERT, not a proto change.
- Live quotas from YC (`source = PROVIDER`).
- UI renders by `display_name`, no enum switch.

---

## 7. Binary cache: per-agent fetch → server cache

**Was:** every agent boot pulled binaries from GitHub/vendor. *N* agents × *M* runs = *N×M* external requests.

**Now (`ops/binary_artifact.proto` + `api/binary_cache.proto`):**

```
BinaryArtifact { name; version; filename; arch; os; sha256; storage_uri }
BinaryCacheService.Resolve(name, version, filename) → presigned URL
```

Server fetches upstream on cache miss → persists → re-serves. Subsequent agents hit cache.

`Package.BinaryDownload.origin` is now `oneof { DirectUrl | CachedRef }`. Builtin packages migrate to `CachedRef`.

**Effect:**

- One upstream fetch per `(name, version)` across the cluster.
- Faster agent boot (server LAN vs GitHub round-trip).
- Reproducibility: sha256 pinned server-side.

---

## 8. Errors: ~330 scattered strings → ~80 typed codes

**Was:** `errors.New(...)` / `fmt.Errorf(...)` produced ~330 unique strings. One sentinel `ErrWdAlreadyExists`. HTTP status mapping inline in handlers.

**Now (`errors/errors.proto`):**

```
Code enum (~80 values, 100-blocks per domain)
Error { Code code; HttpStatus http_status; message; details; request_id; fields }
```

**Effect:**

- i18n viable (UI renders by code, not string).
- Metrics aggregate by error code.
- Stable API contract — adding a code is an additive enum change.

---

## 9. Test entities: one file → six

**Was (`models/test.proto`):** four entities in a single file + `SharedRun` separate.

**Now:**

```
testing/
  test_run.proto
  test_run_template.proto
  test_suite.proto
  test_suite_run.proto
  shared_test_run.proto     ← split out
  shared_suite_run.proto    ← split out
```

**Effect:**

- One file = one entity, easier navigation.
- `SharedRun` split into `SharedTestRun` (single run) and `SharedSuiteRun` (batch). Previously one type couldn't model a suite share.

---

## 10. Queue / Scheduler: separate tables → DAG state

**Was:**

- `jobs` postgres table with `JobState` enum (`queued / claimed / running / done / failed`).
- Scheduler Go code mutates rows directly.
- `internal/domain/scheduler/scheduler.go` owns the queue.

**Now:**

- Queue = view over `node_runs.status` (`PENDING_DEPS`, `READY`, `RUNNING`).
- Scheduler = state-machine transitioner `READY → RUNNING` (admission under `TenantQuotas`).
- Cron = `Schedule.next_fire_at` ticker → `INSERT DagRun`.
- `DagService.ListTenantNodeRuns(statuses=[READY,...])` exposes queue introspection.

**Effect:**

- Three concepts (DAG / queue / scheduler) collapse to one (DAG).
- Recovery after restart = read `node_runs.status`, no separate job table.
- Cancel / retry / pause = uniform operation on `NodeRun`.

---

## 11. Directory layout

**Was (Go):** flat `internal/domain/types/` + `internal/domain/run/` + ...

**Now (proto):**

```
common/    primitives (timestamps, config_file, identity)
iam/       identity & access
catalog/   reusable templates (database, workload, package, deployment, settings)
testing/   workflows (test, suite, runs, shares)
ops/       integrations (webhook, quota, binary_artifact)
agent/     agent registry + wire protocol + service
tasks/     DAG task payloads
system/    engine kernel (dag, schedule) — zero deps
errors/    Code enum + Error envelope
api/       1 service per file (~25 services)
```

**Effect:**

- 60 files grouped by concern.
- Engine isolated — `system/` imports only `common/`.
- 1:1 service-to-file in `api/`.

---

## 12. Conventions

| Convention | Value |
|---|---|
| ID type | `<Entity>Id { string value = 1 [string.len = 26] }` — ULID |
| Field numbering | `id=1, tenant_id=2, ts=3, identity=4, fk=5–8, created_by=9, payload=10+` |
| Audit | `created_by` (`SET_NULL` on user delete) on 10 entities |
| Soft fields | `cloud.common.Timestamps` embed (`created_at / updated_at / deleted_at`) |
| Composite indexes | unique only where semantically required (email/nickname/part+key) |
| Validate | `defined_only` + `not_in:[0]` for required pickers |
| Cross-tenant ref | `metadata` jsonb on engine entities, FK in domain |

---

## 13. What stayed in Go (not yet migrated)

- `internal/core/dag/` Go interface (engine runtime).
- `internal/domain/run/task_*.go` (legacy bridge until full migration).
- `internal/domain/agent/setup_*.go` (transition to 5-verb agent).
- `internal/infrastructure/postgres/` storage — ratel schema must be regenerated.
- Build pipeline (`buf.yaml` / `easyp`) — paths need updating.

---

## Breaking changes (migration)

1. **Storage:** ratel generates new tables (`users`, `tenant_members`, `dags`, `dag_runs`, `node_runs`, `dag_run_state_entries`, `binary_artifacts`, ...). Legacy `jobs`, `accounts` need migration.
2. **API endpoints:** HTTP routes reassemble through a gRPC gateway from the 25 services. Old URLs require a compat shim or are breaking.
3. **Agent:** wire protocol incompatible — dual-stack required during transition.
4. **Configs:** stored `Preset` / `RunConfig` JSON blobs no longer parse with the new types. Migration script required.

---

## Net effect

- **Fewer types:** 29 → 8 tasks, 27 → 5 agent verbs, 330 → 80 error codes.
- **Cleaner dependencies:** engine ignorant of domain.
- **Extensible:** new engine / quota / task type rarely requires proto changes.
- **Persisted state:** `terraformWdId` is now a row in `dag_run_state_entries`, surviving restart without an `*Actor` pointer in memory.
- **Production-ready integrations:** webhooks, API tokens, refresh tokens, shared links, binary cache — standard SaaS surface.
- **Multi-tenant ready:** N:M user ↔ tenant correctly modelled.
