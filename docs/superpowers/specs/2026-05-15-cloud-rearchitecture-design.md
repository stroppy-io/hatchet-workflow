# stroppy-cloud rearchitecture — design spec

**Date:** 2026-05-15
**Status:** approved (brainstorming → ready for implementation plan)
**Scope:** Full big-bang refactor of `stroppy-cloud` Go code to align with the
redesigned protobuf API (`protocols/cloud/v1/*`) and ratel-driven DB layer.

---

## 0. Goals & non-goals

### Goals

- Replace the legacy chi-REST surface with ConnectRPC services derived
  one-to-one from the proto packages.
- Replace sqlc-based DB access with ratel-generated `repository.ProtoRepository`
  (single source of truth = proto).
- Reshape domain code (29 typed tasks → 8 generic specs, 27 agent actions →
  5 verbs, single-tenant Account → multi-tenant User/TenantMember, in-memory
  scheduler → DB-driven DAG engine).
- Keep product semantics unchanged. No feature additions, no feature drops.
- Single binary (`stroppy-cloud`) with three subcommand modes: `server`,
  `agent`, CLI client.
- Full restart-survival: any unfinished work is recoverable from DB state on
  next process start.
- Frontend (already on ConnectRPC) is migrated in lockstep, no dual-stack.

### Non-goals

- No data migration from legacy schema. DB is wiped and re-bootstrapped from
  the new migrations. The product is pre-v1, no production data is preserved.
- No backward-compatible REST shim for the old SPA.
- No support for mixed-version controller/agent fleets (single binary release
  forces lockstep upgrade).
- No NATS, no Redis-as-broker, no external message bus.
- No audit-log subsystem in this scope.

### Hard constraints (from product owner)

1. Strategy = big-bang. No staged migration, no parallel old-vs-new.
2. Identical product semantics — refactor, not reinvent.
3. One linear branch with sequential commits. No PRs, no merge windows, no
   co-author signatures.
4. ratel / protoc-gen-go-plain / protoc-gen-ratel bugs = P0. Block, repro,
   report, bump, regen, verify, resume.
5. All error definitions live in `cloud.v1.errors.Code` enum + `Detail`
   variants. **No human-readable error strings in Go.**
6. Audit-log is explicitly out of scope.

---

## 1. Target tree

```
stroppy-cloud/
├── cmd/stroppy-cloud/          # single binary, cobra root
│   ├── main.go
│   ├── cmd_server.go           # `stroppy-cloud server`
│   ├── cmd_agent.go            # `stroppy-cloud agent`
│   └── cmd_cli/                # `stroppy-cloud <verb>` — full client
├── protocols/cloud/v1/         # proto sources (done)
├── internal/
│   ├── proto/cloud/v1/         # generated pb/grpc/connect/validate/plain/ratel
│   ├── core/
│   │   ├── build/              # version meta
│   │   ├── configurator/       # YAML + env loader (copied/adapted from komeet)
│   │   ├── domainerr/          # Code-only error wrapper (copy adapted from komeet)
│   │   ├── eventing/           # in-memory bus + Group()/Publish primitives
│   │   ├── ids/                # ULID factory
│   │   ├── logger/             # zap (existing)
│   │   ├── shutdown/           # graceful shutdown (existing)
│   │   └── tracing/            # OTel decorators (copy from komeet)
│   ├── domain/
│   │   ├── services/
│   │   │   ├── iam/            # Auth/User/Tenant/Member/ApiToken/RefreshToken
│   │   │   ├── catalog/        # Package/DatabasePreset/WorkloadPreset/Settings
│   │   │   ├── testing/        # TestRun/TestSuite/TestSuiteRun/Shared*/Comparison
│   │   │   ├── system/         # DagService (internal), ScheduleService
│   │   │   ├── agent/          # registry + Hub + protocol
│   │   │   ├── ops/            # Webhook/Quota/BinaryArtifact
│   │   │   ├── admin/          # cross-tenant ops + BinaryCacheAdmin
│   │   │   └── stroppy/        # probe/preview/versions
│   │   └── workers/
│   │       ├── nodeworker/     # DAG node executor pool (SKIP LOCKED claim)
│   │       ├── scheduler/      # cron tick + lease-based fire
│   │       ├── webhookworker/  # outbox drain + retry
│   │       └── recovery/       # startup sweep
│   ├── transport/
│   │   ├── connect/            # ConnectRPC handlers (thin)
│   │   ├── stream/             # Watch* / Stream* server-streaming bridges
│   │   └── middleware/         # recovery, requestID, auth, tenant, validate, idempotency, errmap
│   ├── infrastructure/
│   │   ├── postgres/
│   │   │   ├── postgres.go     # pool factory + ratel migrate runner
│   │   │   ├── pgtx/           # avito-tech tx manager (copy from komeet)
│   │   │   └── migrations/     # ratel-generated SQL + embed.go
│   │   ├── s3/                 # existing, adapted
│   │   ├── terraform/          # existing, adapted to TerraformTask
│   │   ├── valkey/             # existing, used for idempotency store
│   │   ├── victoria/           # existing, adapted
│   │   └── stroppybin/         # subprocess runner for `stroppy probe/preview`
│   ├── sdk/client/             # unified Go client over generated Connect stubs
│   └── testutil/
│       ├── pgcontainer/        # testcontainers Postgres factory + schema-per-test
│       └── fixture/            # entity factories
├── tests/e2e/                  # black-box scenarios against running server
└── web/                        # frontend (out of this spec's scope except interface)
```

### Removed in Phase 0

- `internal/domain/api/` (chi stack, 19 files, ~6700 LOC).
- `internal/domain/agent/{setup_*.go,client_*.go,agent_server.go,protocol.go,deployer.go,deploy_docker.go,cloudinit*}`.
- `internal/domain/run/{builder.go,task_*.go,state.go,validate.go}`.
- `internal/core/dag/` (entire engine).
- `internal/domain/scheduler/`.
- `internal/domain/types/` (replaced by proto types).
- `internal/infrastructure/postgres/{generated/,queries/,*_storage.go,migrate_iom3.go}`.
- `internal/storage/`.
- `cmd/cli/`.

---

## 2. Layer responsibilities

| Layer | Responsibility |
|-------|----------------|
| `protocols/cloud/v1/` | Single source of truth for API + data schema. Adding a field starts here. |
| `internal/proto/cloud/v1/` | Generated artefacts. Never hand-edited. Regenerated via `make proto-gen`. |
| `internal/domain/services/<pkg>/` | Business logic. Stateless structs holding repositories + tx manager + event publisher + cross-domain ports. Methods accept `context.Context` and proto messages, return proto messages + `*domainerr.Error`. |
| `internal/domain/workers/` | Long-running goroutines that operate on DB state (claim → execute → update). No HTTP exposure. |
| `internal/transport/connect/` | Thin Connect handlers: validate → resolve user/tenant from ctx → call service method → wrap response. No business logic. |
| `internal/transport/middleware/` | Cross-cutting: auth, tenant, validate, idempotency, error mapping, request id, tracing, logging. |
| `internal/infrastructure/` | Adapters to external systems: Postgres, Valkey, S3, Terraform binary, stroppy binary, VictoriaMetrics. |
| `internal/sdk/client/` | Typed wrapper around all generated Connect client stubs. Used by CLI subcommands, integration tests, and external automation. |

---

## 3. DB access pattern (komeet-derived)

Every table gets a `ProtoRepository`:

```go
testRunRepo *repository.ProtoRepository[
    testingpb.TestRunAlias,
    testingpb.TestRunColumnAlias,
    *testingpb.TestRunScanner,
    *testingpb.TestRun,
]
```

Service constructor wires repos against a shared `exec.DB` that auto-resolves
the current `pgx.Tx` from `context.Context`:

```go
func New(executor exec.DB, txManager pgtx.TxManager, events eventing.Publisher) *TestRunService {
    return &TestRunService{
        testRunRepo: repository.NewProtoRepository(
            repository.NewScannerRepository(testingpb.TestRuns.Table, executor),
            testingpb.TestRunConverter,
        ),
        txManager: txManager,
        events:    events,
    }
}
```

Method body pattern:

```go
func (s *TestRunService) LaunchTestRun(ctx context.Context, ...) (*testingpb.TestRun, error) {
    return tracing.WithTraceRet(s.Tracer(), ctx, "LaunchTestRun",
        func(ctx context.Context, span trace.Span) (*testingpb.TestRun, error) {
            return pgtx.WithSerializableRet(ctx, s.txManager,
                func(ctx context.Context) (*testingpb.TestRun, error) {
                    // tx-scoped repo calls, business logic, event publish
                })
        })
}
```

Cross-domain dependencies are injected as **ports** (interfaces) defined inside
the consuming package:

```go
// services/testing/service.go
type IamPort interface {
    HasTenantRole(ctx context.Context, u *iampb.UserId, t *iampb.TenantId, min iampb.TenantRole) (bool, error)
}
```

`iam.AuthService` implements the port and is passed to
`testing.New(...)`. No import cycles.

### Transaction isolation choices

- `pgtx.WithSerializable` for: launch flows, refresh-token rotation, scheduler
  claim, anything that mutates multiple coupled rows.
- `pgtx.WithRepeatableRead` for: report aggregation, comparison snapshots.
- `pgtx.WithReadCommitted` (default) for: simple CRUD reads.

---

## 4. Transport layer

### Middleware chain (Connect Interceptors, request order)

1. `recovery` — panic → `Internal` + traceID
2. `requestID` — X-Request-Id propagation
3. `otelconnect` — tracing span per RPC
4. `logging` — zap with `rpc.method`/`rpc.tenant`/`rpc.user` fields
5. `metrics` — prom counters
6. `auth` — JWT validate; bypass list:
   `AuthService.Login`/`Refresh`/`Logout`, `AgentService.Register`
7. `tenant` — read `tenant_id` from request body or `X-Tenant-Id`; verify
   `TenantMember` exists; bypass list: `AdminService.*`, `AuthService.*`
8. `protovalidate` — proto-validate CEL rules
9. `idempotency` — see §6
10. `errorMapper` — `*domainerr.Error` → `*connect.Error` with details

### JWT scheme

- Access token: 15 min, JWT payload `{user_id, jti}`.
- Refresh token: 30 d, opaque ULID, persisted in `refresh_tokens` table.
- Rotation on refresh:
  - Validate incoming refresh, check `revoked_at IS NULL`.
  - Issue new pair, mark old `revoked_at=now()`, `replaced_by=new_id`.
  - **Reuse detection:** if incoming already has `revoked_at` set, revoke the
    entire `family_id` chain and emit security event.

### Tenant resolution

Every non-bypass RPC carries either `tenant_id` in the request body (default —
the proto messages already have it) or an `X-Tenant-Id` header (for endpoints
where it's awkward to embed). Middleware:

1. Extract `tenant_id`.
2. Lookup `(user_id, tenant_id)` in `TenantMember` (cached via Valkey, 5 min
   TTL).
3. If absent → `PermissionDenied`.
4. Inject into ctx; handlers read via `middleware.TenantFromCtx(ctx)`.

`AgentService` is special: agents authenticate via `agent_token`, the tenant is
loaded from the `agents.tenant_id` row.

---

## 5. Errors

All error identity lives in `cloud.v1.errors.Code` enum (canonical 16-value gRPC
codes + optional ErrorInfo with structured `reason`). **Zero hardcoded error
strings in Go.**

```go
type Error struct {
    code    errorspb.Code
    details []*errorspb.Detail
}

func E(code errorspb.Code, details ...*errorspb.Detail) *Error { ... }
func (e *Error) Code() errorspb.Code { return e.code }
func (e *Error) Error() string       { return e.code.String() }   // diagnostic only
```

Usage:

```go
return nil, domainerr.E(errorspb.Code_NOT_FOUND,
    &errorspb.Detail{Kind: &errorspb.Detail_ResourceInfo{
        ResourceInfo: &errorspb.ResourceInfo{ResourceType: "user", ResourceName: id.Value},
    }})
```

Mapping interceptor produces:

```go
cerr := connect.NewError(mapCodeToConnect(dErr.Code()), errors.New(dErr.Code().String()))
for _, d := range dErr.Details() {
    cerr.AddDetail(connect.NewErrorDetail(d))
}
```

Frontend renders human-readable text from a `Code` → i18n string map, using
`details` for parameterisation.

---

## 6. Idempotency

Pattern lifted from komeet (`internal/infrastructure/grpcext/idempotency/`),
adapted from gRPC unary interceptor to Connect `UnaryInterceptorFunc`.

- Header: `X-Idempotency-Key`.
- Scope: per-tenant (key namespaced by `tenant_id`).
- Registry: built from generated handler spec — RPCs with
  `option idempotency_level = IDEMPOTENT` are eligible.
- Store: Valkey (existing `internal/infrastructure/valkey/`) with TTL.
- State machine: `Pending` → `Cached` (or skipped on transient codes).
- Concurrent duplicate with same key while `Pending` → `Aborted`.
- `Cached` hit replays the cached response bytes verbatim.
- Transient codes (`Unavailable`, `DeadlineExceeded`, `ResourceExhausted`) are
  not cached.

---

## 7. Eventing

Single-binary in-memory pub/sub. No NATS, no Kafka.

```go
type Bus interface {
    Subscribe(topic Topic, handler func(ctx context.Context, evt Event)) (unsubscribe func())
    Publish(ctx context.Context, evt Event)
}
```

Typed event structs (no `any`):

```go
type TestRunLaunched struct {
    TenantID  *iampb.TenantId
    TestRunID *testingpb.TestRunId
    DagRunID  *systempb.DagRunId
}
type NodeRunCompleted struct {
    NodeRunID *systempb.NodeRunId
    Status    systempb.NodeRunStatus
}
// ... per category
```

### Webhook delivery (only flow requiring durability)

A new table `webhook_deliveries` (added to `cloud.v1.ops.webhook`):

```
id, webhook_id (FK), payload jsonb, state enum (PENDING/IN_FLIGHT/DELIVERED/DEAD),
attempts int, next_attempt_at, last_error, created_at
```

Flow:

1. Service publishes event in-process (`Bus.Publish`).
2. `webhookworker` subscribes; on event it pattern-matches subscribed webhooks
   for the tenant and `INSERT webhook_deliveries`.
3. Worker drain loop claims `state=PENDING AND next_attempt_at<=now()`, POSTs,
   on 2xx → `DELIVERED`, on failure → exponential backoff
   (`attempts++`, schedule next), after `max_attempts` → `DEAD`.

Live UI streams (`WatchTestRun`, `StreamTestRunLogs`) use the in-memory bus
directly with a `since` cursor for backlog replay from DB tables. Stream
durability is not a goal — clients reconnect with the last cursor and replay.

---

## 8. DAG runtime

### Engine model

Engine is fully DB-driven. There is no in-memory graph state. Workers compete
via `SELECT ... FOR UPDATE SKIP LOCKED`.

### NodeWorker pool

`internal/domain/workers/nodeworker/`. Configurable parallelism
(`workers.node_workers`).

Claim query:

```sql
WITH ready AS (
  SELECT id FROM node_runs
  WHERE status = 'NODE_RUN_STATUS_READY'
  ORDER BY started_at NULLS FIRST, id
  FOR UPDATE SKIP LOCKED LIMIT 1
)
UPDATE node_runs n
SET status='NODE_RUN_STATUS_RUNNING', started_at=now()
FROM ready WHERE n.id = ready.id
RETURNING *;
```

The claim transaction closes immediately after the UPDATE. The handler then
runs outside the tx (jobs can be long — terraform apply 30+ min).

Lifecycle:

1. Resolve `NodeHandler` from `node.spec.type_url` (Any → kind).
2. Execute handler; handlers for agent-side work enqueue via `agent.Hub`.
3. On success → `UPDATE node_runs SET status=DONE, finished_at, output`;
   recompute `status=READY` for downstream nodes whose deps are all DONE.
4. On failure → `status=FAILED, error`; if `attempt < retry_max`, spawn a
   retry `node_run`.
5. When all leaves DONE → `dag_runs.status = SUCCEEDED`; publish
   `NodeRunCompleted` / `DagRunCompleted`.
6. Cancel: poll-detect via `dag_runs.cancel_requested` between handler
   yield points; propagate via ctx.Done.

### NodeHandler set

One per task kind:

- `terraform.go` — wraps `infrastructure/terraform`; op=APPLY/DESTROY.
- `docker.go` — local docker run/rm for agent containers.
- `package_install.go` — enqueues 5-verb action via `agent.Hub`.
- `config_apply.go` — same pattern.
- `one_shot.go` — same pattern (init scripts: cockroach init, ydb init).
- `stroppy_run.go` — enqueues stroppy execution to agent.
- `test_run_ref.go` — spawns child DagRun (meta-DAG for suite).

### Scheduler

`internal/domain/workers/scheduler/`. Lease-based, multi-replica-safe even
though we run a single binary (defensive).

Tick loop (every `workers.scheduler_tick`, e.g. 5 s):

```sql
UPDATE schedules SET
  lease_owner = $instance,
  lease_expires_at = now() + interval '30 seconds'
WHERE id IN (
  SELECT id FROM schedules
  WHERE next_fire_at <= now()
    AND (lease_expires_at IS NULL OR lease_expires_at < now())
  ORDER BY next_fire_at
  FOR UPDATE SKIP LOCKED LIMIT 10
)
RETURNING *;
```

For each claimed schedule:
- Apply `MisfirePolicy` (SKIP / FIRE_ONCE / FIRE_ALL).
- Apply `CatchupMode` (catch missed slots or jump to now).
- `INSERT DagRun + INSERT node_runs` from `Schedule.template`.
- `UPDATE schedules SET next_fire_at = cron.Next(next_fire_at)`.

### Restart recovery (`internal/domain/workers/recovery/`)

Runs once at server start, before NodeWorker / Scheduler start.

Since the binary is single-replica per deployment, every `RUNNING` row on
startup is guaranteed to be from a dead process — blanket reset is safe.

```sql
UPDATE node_runs
SET status = CASE WHEN attempt < retry_max
                  THEN 'NODE_RUN_STATUS_READY'
                  ELSE 'NODE_RUN_STATUS_FAILED' END,
    attempt = attempt + 1,
    error = COALESCE(error, 'process_restart')
WHERE status IN ('NODE_RUN_STATUS_RUNNING', 'NODE_RUN_STATUS_CANCELING');

UPDATE webhook_deliveries
SET state='PENDING', last_error='restart_requeue'
WHERE state='IN_FLIGHT';

UPDATE agents
SET status='AGENT_STATUS_UNKNOWN'
WHERE status='AGENT_STATUS_BUSY' AND last_seen_at < now() - interval '60 seconds';

-- Recompute dag_runs.status from aggregate of node_runs.status
WITH agg AS (
  SELECT dag_run_id,
         bool_and(status IN ('DONE','SKIPPED')) AS all_done,
         bool_or(status='FAILED') AS any_failed
  FROM node_runs GROUP BY dag_run_id
)
UPDATE dag_runs d
SET status = CASE WHEN agg.any_failed THEN 'DAG_RUN_STATUS_FAILED'
                  WHEN agg.all_done   THEN 'DAG_RUN_STATUS_SUCCEEDED'
                  ELSE 'DAG_RUN_STATUS_RUNNING' END
FROM agg WHERE d.id = agg.dag_run_id
  AND d.status = 'DAG_RUN_STATUS_RUNNING';
```

No `worker_owner` or `last_heartbeat_at` columns. They'd be needed only for
multi-replica; YAGNI now, easy to add later if scale demands.

---

## 9. Agent transport

### Wire

ConnectRPC, server-streaming for commands + unary for everything else
(survives reverse proxies; long-poll-like semantics).

```
rpc Register(RegisterRequest)     returns (RegisterResponse)    // bootstrap, gets agent_id
rpc PollCommands(PollRequest)     returns (stream Command)      // long-lived
rpc Report(ReportRequest)         returns (ReportResponse)      // unary, per-command
rpc Heartbeat(HeartbeatRequest)   returns (HeartbeatResponse)   // unary, every 15s
rpc LogBatch(LogBatchRequest)     returns (LogBatchResponse)    // unary, stdout/stderr push
```

### Hub

`internal/domain/services/agent/Hub` — in-memory bridge between node-handlers
(on the controller) and connected agents:

```go
type Hub struct {
    streams map[string]chan *agentpb.Command   // agent_id → outgoing
    pending map[string]*pendingCmd             // command_id → reply waiter
}

func (h *Hub) AttachStream(agentID string, send func(*agentpb.Command) error) (release func())
func (h *Hub) Dispatch(ctx context.Context, agentID string, cmd *agentpb.Command, timeout time.Duration) (*agentpb.Report, error)
func (h *Hub) Resolve(cmdID string, report *agentpb.Report)
```

### Persist-and-replay for restart safety

A new `agent_commands` table (added to `cloud.v1.agent`):

```
id ulid PK
agent_id FK
node_run_id FK             -- back-link
command_payload jsonb      -- serialized Command
state enum                 -- PENDING | DELIVERED | REPORTED | TIMED_OUT
issued_at timestamptz
delivered_at timestamptz?
reported_at timestamptz?
result_payload jsonb?      -- serialized Report
attempt int
```

Flow with persistence:

1. NodeHandler: `INSERT agent_commands(state=PENDING)`.
2. `Hub.Dispatch` sends to stream, marks `state=DELIVERED`, waits for Report.
3. Agent processes Command, calls `Report` unary; server handler:
   `UPDATE state=REPORTED, result_payload` + `Hub.Resolve` to wake the waiter.
4. After server restart: `PENDING` and `DELIVERED` rows remain. NodeHandler on
   retry checks `agent_commands` for a `REPORTED` row for the same
   `node_run_id` before re-issuing — idempotent recovery.

### Agent-side idempotency

Agent caches the last N processed `command_id`s on disk. If it sees the same
`command_id` twice (server retransmit after reconnect), it replies with the
cached `Report` instead of re-executing.

### Logs

`LogBatch(node_run_id, lines[], cursor)` arrives unary, server:

1. Inserts into `node_run_logs` (insert-only, date-partitioned).
2. Publishes to in-memory `LogBus` for live subscribers.

`StreamTestRunLogs` consumer flow:
- If `since` provided, server back-fills from `node_run_logs` first.
- Then subscribes to `LogBus` for `(test_run_id, optional step_id)` and streams
  forward.

---

## 10. Single binary modes

`cmd/stroppy-cloud/main.go` is a Cobra root with three top-level subcommands:

```
stroppy-cloud server     # long-running controller
stroppy-cloud agent      # long-running remote-host worker
stroppy-cloud <verb>     # CLI client (login, run, suite, preset, package, ...)
```

### server

- Loads config (`internal/core/configurator/`, YAML + env).
- Builds pool, applies migrations under pg_advisory_lock.
- Wires all `services/*`, `workers/*`, `transport/connect/*`.
- Bootstraps initial admin user/tenant if DB is empty.
- Serves HTTP/2 (h2c) + embedded SPA fallback.
- Graceful shutdown drains workers (30 s cap) and closes pool.

### agent

- Loads agent config (server URL + agent token from cloud-init or local file).
- Reads/creates persistent `agent_id` from `/var/lib/stroppy-agent/id`.
- Opens `PollCommands` stream with reconnect + exponential backoff.
- Dispatches received commands to 5 verb handlers:
  - `PutFile` — write bytes to disk
  - `RunShell` — exec.CommandContext with kill_grace_seconds
  - `SystemctlOp` — systemctl start/stop/restart/reload
  - `InstallPackage` — apt-get / dpkg / tarball fetch via
    `BinaryCacheService.Resolve`
  - `WaitForCondition` — poll-loop predicate (port open, http 200, etc.)
- Heartbeat every 15 s, log batches every 1 s (or 64 KiB), reports per command
  completion.

### CLI client

`cmd/stroppy-cloud/cmd_cli/` — one Cobra subcommand per domain, mapped to
public ConnectRPC services:

```
stroppy-cloud login --server <url>
stroppy-cloud logout
stroppy-cloud context use --tenant <id>
stroppy-cloud tenant {list,create,...}
stroppy-cloud user {list,create,reset-password,...}
stroppy-cloud preset database {list,create,clone,...}
stroppy-cloud preset workload {...}
stroppy-cloud package {list,upload,...}
stroppy-cloud template {list,create,...}
stroppy-cloud run {start,list,get,cancel,logs,metrics,share}
stroppy-cloud suite {list,create,launch,cancel}
stroppy-cloud schedule {list,create,delete}
stroppy-cloud probe --script <id> --driver <type>
stroppy-cloud admin {tenant-list,user-create,...}
```

All CLI commands route through `internal/sdk/client/`, which is the same
typed wrapper used by integration tests.

Credentials cache at `~/.config/stroppy-cloud/credentials.json`; active context
at `~/.config/stroppy-cloud/context.json`. CLI auto-refreshes access tokens
when expired.

---

## 11. Configuration

YAML files under `deployments/local/{server,agent}/`; loader handles
`${ENV_VAR}` substitution and validates required keys.

Minimum server config:

```yaml
server:        { http_addr: ":8080" }
postgres:      { dsn: "...", max_conns: 25 }
valkey:        { addr: "...", db: 0 }
s3:            { endpoint: "...", bucket: "...", access_key_env: "...", secret_key_env: "..." }
victoria:      { push_url: "...", query_url: "..." }
terraform:     { binary_path: "...", workdir_root: "..." }
stroppy:       { default_version: "v4.1.0", binaries_dir: "..." }
auth:          { jwt_secret_env: "JWT_SECRET", access_ttl: "15m", refresh_ttl: "720h" }
idempotency:   { enabled: true, ttl: "24h" }
workers:       { node_workers: 8, scheduler_tick: "5s", webhook_workers: 4, recovery_on_start: true }
features:      { initial_admin_email: "...", initial_admin_password_env: "INITIAL_ADMIN_PASSWORD" }
log:           { level: "info", format: "json" }
```

Agent config is small: `server_url`, `agent_token`, `state_dir`.

---

## 12. Testing strategy

### Categories

| Category | Where | Speed | Coverage |
|----------|-------|-------|----------|
| Unit | `internal/core/...` and pure-function helpers | <30 s total | Algorithms, validators, ID factories |
| Integration | `internal/domain/services/<pkg>/integration_test.go` | 3-5 min total | Service methods against real Postgres + ratel + tx mgr |
| E2E | `tests/e2e/` (`-tags=e2e`) | 5-15 min | Full server + agent + CLI scenarios |
| Transport | `internal/transport/connect/*_test.go` | <2 min | Connect handlers via in-process Connect client (middleware chain exercised) |

### Helpers

- `internal/testutil/pgcontainer/`: testcontainers Postgres + schema-per-test.
  One container per `go test ./internal/...` package run, isolated schemas per
  test allow `t.Parallel()`.
- `internal/testutil/fixture/`: entity factories
  (`f.Tenant()`, `f.User()`, `f.Member(...)`, `f.TestRun(...)`, `f.Agent(...)`).
- No DB mocks. No pool mocks. ratel converters and SQL are exercised in real.

### CI matrix

```
unit:        go test ./internal/core/... -race -count=1
integration: go test $(go list ./internal/domain/... ./internal/transport/...) -race -p 4
e2e:         go test -tags=e2e ./tests/e2e/... -timeout 30m
```

---

## 13. Phasing

Linear, single branch, no parallel work.

| # | Phase | Acceptance criteria |
|---|-------|---------------------|
| 0 | Cleanup | Tree per §1 in place; old code deleted; empty service stubs present; `go build` fails on purpose |
| 1 | Foundation: pgtx, tracing, domainerr, ids, eventing, middleware, testutil | `go test ./internal/core/... ./internal/testutil/...` green |
| 2 | IAM service + auth middleware | integration_test covers login/refresh/rotation/CRUD; e2e `login → tenant create` |
| 3 | Catalog + Stroppy + Settings | integration CRUD; e2e `package create → stroppy probe` |
| 4 | System engine + workers + recovery | integration_test: 2-node DagRun executes to DONE; restart-test: kill mid-flight → resume on next start |
| 5 | Testing services + DagRun-builder | e2e `template create → test_run launch → wait done` with mock node-handlers |
| 6 | Agent service + 5-verb agent + persist commands | e2e real agent subprocess registers, executes RunShell, server records Report |
| 7 | Ops: webhook outbox + quota + binary cache | integration_test: TestRun complete → webhook fires → mock endpoint receives |
| 8 | Admin + binary_cache admin | integration_test cross-tenant ops |
| 9 | Frontend on ConnectRPC | SPA wizard works end-to-end against new API |
| 10 | E2E suite + perf smoke + docs | All acceptance from §0 met |

### Rules

- One linear branch, sequential commits.
- No PRs, no merge windows, no co-author signatures.
- A phase doesn't start until the previous phase's acceptance criteria are
  green.

### P0 protocol for ratel / protoc-gen-go-plain / protoc-gen-ratel bugs

When a codegen bug is hit:

1. Stop current implementation work.
2. Produce a minimal proto-fragment repro.
3. Write a bug report (Component / Severity / Repro / Expected / Suspected
   location / Minimal test) — same format used earlier for
   `references_schema:"public"` and oneof-embed naming.
4. Hand off to upstream maintainer, wait for fix + version bump.
5. Update `easyp.go.yaml` deps + `go.mod`, run `make proto-gen` and
   `make migrate-clear`, verify the bug-repro proto now generates correctly.
6. Resume the interrupted phase.

---

## 14. Risks + mitigations

| Risk | Mitigation |
|------|------------|
| ratel converter / oneof embed / topo-sort edge cases | Phase 2 (IAM) is the canary — if it ships clean, the rest follows the pattern. Bug-report flow per §13. |
| Long-lived branch with broken build | Branch is single-developer, single-narrative. No CI gating until phase ends. Acceptance criteria gate progression. |
| Infrastructure adapters (terraform/s3/victoria) drift from new task specs | Phase 4 wires real adapters into node-handlers; integration tests use real Postgres + mock external system, plus one e2e per adapter against a real backend (S3 → minio, victoria → vm-single). |
| Wizard `stroppy probe` JSON shape change | Phase 3 includes a snapshot test against a real `stroppy probe` invocation; if upstream stroppy emits new fields, `ProbeStroppyConfigResponse` is widened (proto change) before the snapshot is updated. |
| Agent reconnect / command dedup | `agent_commands.state` machine + agent-side `last_processed_command_id` cache; integration_test kills agent mid-Report and verifies idempotent resume. |
| Frontend lags backend on a proto change | Frontend lives on the same branch in `web/`. Proto regen produces both Go and TS in one `make proto-gen`. Frontend phase (9) is held until backend phase (8) is green. |
| Restart-recovery undercounts edge cases | Phase 4 ships with explicit `recovery_test.go` covering: RUNNING → READY, CANCELING → CANCELED, IN_FLIGHT webhook requeue, stale agent, dag_runs status recompute. |

---

## 15. Out-of-scope deliverables (deferred)

- Audit log subsystem (`audit_logs` table + per-RPC decorator). Add post-v1 if
  product needs it.
- Multi-replica HA: requires `node_runs.worker_owner` + heartbeat + scheduler
  leader election. Not needed for current scale.
- Per-tenant rate limiting at gateway level.
- Cross-region replication.
- GUI for admin (admin RPCs are CLI-only for now).
- Real-time metrics aggregation in-process (still delegated to VictoriaMetrics).

---

## 16. Acceptance (overall)

The refactor is complete when:

- `go build ./cmd/stroppy-cloud` produces a single binary.
- `stroppy-cloud server` boots, applies migrations, serves all Connect
  services declared in `protocols/cloud/v1/*/`.
- `stroppy-cloud agent` registers, executes 5-verb commands, reports back.
- `stroppy-cloud <verb>` CLI exposes a command for every public RPC.
- Every `services/*` package has a green `integration_test.go`.
- E2E suite covers: login → tenant → preset → template → run → completion →
  metrics → webhook delivery → share link.
- Frontend wizard runs end-to-end on the new API.
- Kill -9 of `stroppy-cloud server` mid-run resumes on next start (verified by
  `recovery_test.go` and an e2e scenario).
- `X-Idempotency-Key` deduplicates `LaunchTestRun` and other IDEMPOTENT RPCs
  via Valkey.
- All errors surface as `cloud.v1.errors.Code` + structured details — no
  hard-coded strings in Go remain.
