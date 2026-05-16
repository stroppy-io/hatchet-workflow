# System Engine + Workers (Plan 03)

## Overview

Plan 03 delivered the core DAG execution engine and three worker goroutines that drive task execution in a DB-native way. The system is a pure state machine living in Postgres: DAGs are template rows, DagRuns are execution instances, NodeRuns are individual task invocations, and state entries store scratch data (e.g., terraform working directory IDs) across restarts. This design makes the engine reusable for any workflow (testing, deployment, ETL) and survives process crashes—killed mid-flight, the server resumes on next start by reading the database instead of rebuilding from in-memory pointers.

Workers operate as three independent goroutines inside `cmd_server`. NodeWorker claims ready tasks via `SELECT ... FOR UPDATE SKIP LOCKED`, executes them outside the database transaction, and marks completion. Scheduler fires cron schedules by advancing `next_fire_at` with lease-based exactly-once semantics across a cluster. Recovery sweeps at startup to reset tasks interrupted by crashes and finalize DAG runs to terminal states. All operations are idempotent and safe for restarts.

## Service: system.Service

Located in `internal/domain/services/system/`, the service owns five proto repositories (dags, dag_runs, node_runs, dag_run_state_entries, schedules) and exposes methods consumed by the three workers and the ScheduleService handler.

### DAG Management

- **SaveDag(ctx, dag) → dag** — Persists a Dag template row with a fresh ID and timestamps. Each save is a new row (graph_hash deduplication is future work).
- **GetDag(ctx, id) → dag** — Fetches a Dag template by ID. Returns domainerr.NotFound if missing or soft-deleted.

### DagRun Lifecycle

- **StartDagRun(ctx, input) → dag_run** — Creates a new DagRun row and spawns N NodeRun rows from the Dag.Graph. Nodes with no dependencies start as READY; nodes with deps start as PENDING_DEPS. Returns the DagRun (auto-assigned ID, timestamps, status = RUNNING).
- **GetDagRun(ctx, id) → dag_run** — Fetches a DagRun by ID with its current status (RUNNING, SUCCEEDED, FAILED, CANCELED).
- **CancelDagRun(ctx, id) → error** — Currently a stub returning "cancel not implemented"; needs proto column `dag_runs.cancel_requested` to signal workers to stop child nodes.

### NodeRun State Machine

- **MarkNodeRunSucceeded(ctx, node_id) → error** — Transitions a NodeRun to DONE, then recomputes downstream nodes with `depends_on` dependency on this node, advancing any PENDING_DEPS → READY. Finalizes the parent DagRun to SUCCEEDED if all nodes are DONE or SKIPPED.
- **MarkNodeRunFailed(ctx, node_id, error) → error** — Transitions a NodeRun to FAILED, stores the error message, and finalizes the parent DagRun to FAILED (no downstream unblocking on failure).

### State Store (Scratch Data)

- **PutState(ctx, dag_run_id, key, value) → entry** — Upserts a DagRunStateEntry in `dag_run_state_entries` with optimistic versioning. Used by handlers to store ephemeral state (e.g., terraform working directory ID, agent command output) that survives restarts.
- **GetState(ctx, dag_run_id, key) → value, exists, error** — Retrieves a state entry by key. Returns false for exists if the key is not found.

### Schedule Management

- **CreateSchedule(ctx, sched) → sched** — Inserts a new Schedule row with ID, timestamps, cron expression, enabled flag, and optional payload. No tenant_id column exists.
- **GetSchedule(ctx, id) → sched** — Fetches a Schedule by ID.
- **ListSchedules(ctx) → []*sched** — Returns all non-deleted schedules (no tenant filter applied).
- **DeleteSchedule(ctx, id) → error** — Soft-deletes a Schedule by setting deleted_at = now().
- **UpdateSchedule(ctx, sched) → sched** — Updates a Schedule's cron_expr, enabled, hook_name, payload, and metadata fields within a Serializable transaction.
- **EnableSchedule(ctx, id) → error** — Sets enabled = true.
- **DisableSchedule(ctx, id) → error** — Sets enabled = false.
- **TriggerNow(ctx, id) → sched** — Immediately fires a schedule by resetting next_fire_at = now(), lease_owner = '', and lease_expires_at = NULL. Returns the updated schedule.

## Workers

Three independent goroutines run under `workerCtx` (cancelled on server shutdown).

### NodeWorker

Located in `internal/domain/workers/nodeworker/worker.go`.

Runs N worker goroutines (default 4, configurable) polling every 500 ms. Each loop iteration claims one READY NodeRun via `claimReadyNode()` using `SELECT ... FOR UPDATE SKIP LOCKED`, moving its status to RUNNING in the same transaction. Once claimed, execution happens outside the transaction: the handler is resolved from the registry by matching the `node_run.spec.type_url` suffix (case-insensitive), invoked with the NodeRun and its parent Dag's node spec, and allowed to run long (HTTP requests, shell commands, etc.). After execution, `MarkNodeRunSucceeded` or `MarkNodeRunFailed` is called to update the database, trigger downstream unblocking, and potentially finalize the DagRun.

**Claim loop logic:**
1. Query for one row in `node_runs` where `status = 'NODE_RUN_STATUS_READY'` and `dag_run_id = ...` ordered by `created_at asc`.
2. Lock with `FOR UPDATE SKIP LOCKED` (skips locked rows, avoids blocking other workers).
3. Update status to RUNNING, increment attempt, set started_at = now().
4. Commit transaction.
5. Execute handler outside transaction.
6. Mark succeed or fail, which triggers downstream unblocking and DAG finalization.

### Scheduler

Located in `internal/domain/workers/scheduler/scheduler.go`.

Polls every 5 seconds (configurable). Each tick claims a batch of up to 10 (configurable) Schedule rows where:
- `enabled = true`, `next_fire_at <= now()`, `deleted_at IS NULL`
- Lock with `FOR UPDATE SKIP LOCKED`
- Atomically update `lease_owner = hostname:pid`, `lease_expires_at = now() + 30s` (configurable)

For each leased schedule:
1. Parse `cron_expr` using `robfig/cron/v3`.
2. Execute `svc.StartDagRun(ctx, StartDagRunInput{Dag: ...})` to create a DagRun and N NodeRuns from the template.
3. Advance `next_fire_at` to the next cron tick via `cron.Next(now)`.
4. Update `last_fired_at = now()`, reset `consecutive_failures = 0`.
5. Release lease by setting `lease_owner = ''`, `lease_expires_at = NULL`.

Lease-based semantics ensure that even if two cluster instances poll at the same time, only one will claim and fire each schedule in a given tick window. Lost leases (instance crashes mid-fire) are automatically released after 30 seconds, allowing another instance to retry.

### Recovery Sweep

Located in `internal/domain/workers/recovery/sweep.go`.

Runs once on server startup (if enabled in config). Executes four idempotent SQL statements:

1. **Reset stale in-flight tasks:** NodeRuns in RUNNING or CANCELING (from before restart) → READY, increment attempt, set error = 'process_restart'.
2. **Requeue webhook deliveries:** WebhookDeliveries in IN_FLIGHT → PENDING, set last_error = 'restart_requeue'.
3. **Clear stale agent states:** Agents with status BUSY and last_seen_at > 60s ago → UNKNOWN.
4. **Finalize DAG runs:** Aggregate NodeRun statuses and update parent DagRuns to SUCCEEDED (all children DONE or SKIPPED) or FAILED (any child FAILED), but leave RUNNING runs untouched (may still have pending nodes).

Tolerates missing tables (e.g., `webhook_deliveries`, `agents`) and logs warnings instead of failing. This allows the recovery sweep to run safely across different project configurations.

## Handler Registry

Located in `internal/domain/workers/nodeworker/registry.go`.

**Handler interface:**
```go
type Handler interface {
    Kind() string
    Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state StateStore) (*anypb.Any, error)
}
```

**Registry:**
```go
type Registry struct{ m map[string]Handler }

func (r *Registry) Resolve(typeURL string) Handler
```

`Resolve(typeURL)` finds the first registered handler whose `Kind()` suffix matches the end of the typeURL (case-insensitive). This allows typeURLs like `type.googleapis.com/cloud.v1.tasks.TerraformTask` to match a handler registered with `Kind() = "terraform"`.

**MockHandler** (in `handlers/mock.go`): Implements Kind() = "" to match any spec (wildcard), sleeps for a configurable duration, and returns success. Used in tests and cmd_server's initial development loop. Real handlers (terraform, docker, agent commands) register in plans 04/05.

## Schema Realities

NodeRun stores only `node_id` (string reference to `Dag.Graph.Nodes[*].Id`) and `dag_run_id`. The node's specification (Spec, MaxAttempts, TimeoutSeconds, Dependencies) lives in the parent Dag and is fetched at execution time via `GetDag()`. This avoids denormalization and keeps NodeRun rows minimal. The `depends_on` array is reconstructed by scanning the Dag.Graph at StartDagRun time and stored as a comma-separated list in the `depends_on` column for efficient querying.

**Limitation:** The `cancel_requested` column does not yet exist on dag_runs. `CancelDagRun()` is a stub pending a proto change to add this column. Once added, the recovery sweep and NodeWorker will check this flag and abort child nodes.

## Transport

`internal/transport/connect/system.go` implements `ScheduleServiceHandler` wrapping the system.Service. The handler provides eight RPC methods (CreateSchedule, UpdateSchedule, DeleteSchedule, GetSchedule, ListSchedules, EnableSchedule, DisableSchedule, TriggerNow), all mounted in `server.go` under the gRPC mux. ListSchedules ignores the tenant_id payload since schedules have no tenant column (future work).

## SDK + CLI

`internal/sdk/client/system.go` exports a `client.Schedule` struct with methods for CRUD operations. The CLI in `cmd/stroppy-cloud/cmd_cli/schedule.go` provides subcommands:
```
schedule {list,get,delete,enable,disable,trigger}
```

Example:
```bash
stroppy-cloud schedule list
stroppy-cloud schedule get --id <schedule_id>
stroppy-cloud schedule trigger --id <schedule_id>
```

## Wiring

In `cmd/stroppy-cloud/cmd_server.go`:
1. System service is instantiated with dags, dag_runs, node_runs, state, schedules repos.
2. Recovery sweep runs at startup (if `Workers.EnableRecoverySweep = true`).
3. NodeWorker pool, Scheduler, and (future) WebhookWorker are spawned as goroutines under `workerCtx`.
4. On server shutdown, `workerCtx` is cancelled, all workers stop gracefully.
5. ScheduleHandler wraps the system service and is mounted in the ConnectRPC mux in `transport/connect/server.go`.

## Tests

- **nodeworker_test.go:** `TestWorkerCompletesTwoNodeDag()` builds a two-node Dag (A → B), starts a DagRun, runs the worker pool with MockHandler, and verifies the DagRun reaches SUCCEEDED status within 15 seconds.
- **scheduler_test.go:** Verifies that next_fire_at advances on cron tick and that lease-based claiming prevents duplicate fires.
- **recovery_test.go:** Confirms that RUNNING → READY transition occurs after startup sweep and that DAG run aggregation finalizes correctly.

## Known Limitations

- **CancelDagRun unimplemented:** Needs `dag_runs.cancel_requested` proto column and recovery sweep logic to cascade cancellation to child NodeRuns.
- **Real node handlers in plans 04/05:** Currently only MockHandler executes; terraform, docker, and agent command handlers are not yet wired.
- **Schedules lack tenant_id column:** ListSchedules ignores the tenant parameter; schedule isolation per tenant is not yet modeled.
- **Workers run mock handler in cmd_server only:** Real handler registration occurs in future plans; cmd_server's worker loop is safe for development but not production use without custom handlers.

## Next Plan

Plan 04 introduces Testing services (TestRunService, TestSuiteService, ComparisonService) and a DagRun-builder that translates a TestRun template into a Dag with task nodes representing terraform, docker, and agent setup/teardown. This plan also registers real handlers for common packages (PostgreSQL, MySQL, YDB, Cockroach) and their configuration tasks.
