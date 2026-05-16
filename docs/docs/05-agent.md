# Agent Subsystem (Plan 05)

## Overview

Plan 05 delivered the agent subsystem — a remote-execution framework for DAG node handlers. The server-side `AgentService` and `AgentAdminService` manage agent lifecycle; the in-memory `Hub` bridges DAG node handlers to connected agents by enqueueing commands and blocking on per-command reply channels until reports arrive. Agent-side, the daemon binary registers via bootstrap token, enters a long-poll loop (default 2 seconds), and spawns goroutines to execute 5 verbs: `PutFile`, `RunShell`, `Systemctl`, `InstallPackage`, and `WaitFor`. All 5 verbs are idempotent via an LRU cache (256 entries) keyed on `command_id`, allowing safe replay on transport errors.

The architecture is polling-based: agents pull commands from the server, never push. This avoids firewall complexity and matches production requirements from Plan 04. Bootstrap tokens are JWT-based (HS256) with per-agent poll tokens minted on successful registration. Commands are persisted in `agent_commands` table with state machine: PENDING → DELIVERED → REPORTED (or TIMED_OUT). On server restart, `DispatchForNodeRun` checks if a node run already has a REPORTED command and short-circuits, replaying the cached result without re-issuing.

## Server-side

### Hub

The in-memory `Hub` struct (`internal/domain/services/agent/hub.go`) is the central dispatcher. It maintains per-agent command queues (buffered channels) and per-command reply waiters (channels for `CommandReport`). Node handlers enqueue work via `Dispatch` or `DispatchForNodeRun`; the latter checks the `agent_commands` table for an existing REPORTED row with the same `node_run_id` and short-circuits if found, replaying the result without enqueueing. Handlers then block on a per-command channel with a timeout (typically 5–30 minutes). Drain returns up to N pending commands from the queue and marks them DELIVERED in the database. Resolve unblocks the waiting handler by posting a `CommandReport` to its channel.

### Service

`AgentService` (`internal/domain/services/agent/service.go`) implements three RPC procedures:
- **Register**: Validates bootstrap token via `BootstrapTokenStore`, extracts tenant and dag_run IDs, inserts an `Agent` row, invalidates the bootstrap token (one-time use), and returns a poll_token (JWT).
- **Poll**: Long-polls up to 25 seconds for new commands using `Hub.WaitForCommand`, processes inbound `AgentReport` (heartbeat, completed command reports, log batches), marks commands REPORTED in the database, resolves pending waiters in the Hub, and publishes logs to the LogBus.
- **Deregister**: Marks the agent as TERMINATED and saves the termination timestamp.

All three procedures are in `AuthBypass` — Register requires the bootstrap token and validates it; Poll and Deregister require the poll_token (a signed JWT with agent_id + tenant_id) passed as `Authorization: Bearer <token>`.

### AgentAdminService

`AgentAdminService` (`internal/domain/services/agent/admin_service.go`) provides admin APIs (not yet wired to Connect handlers):
- **GetAgent**: Viewer role, returns a single agent by ID.
- **ListAgents**: Viewer role, filters by tenant and returns paginated agents.
- **TerminateAgent**: Admin role, marks the agent TERMINATED.

### Bootstrap Token Store

`BootstrapTokenStore` (`internal/domain/services/agent/bootstrap.go`) issues and validates one-time bootstrap tokens. Tokens are HS256 JWTs with claims: `tenant_id`, `dag_run_id`, `machine_id`, `role`, `exp`, and `jti` (JWT ID). The `jti` is stored in Valkey with TTL matching the token's lifetime. `Verify` checks both JWT validity and key existence in Valkey; `Invalidate` deletes the key. This ensures tokens are single-use and time-bounded.

### Connect Handler

`AgentHandler` and `AgentAdminHandler` (`internal/transport/connect/agent_handlers.go`) are thin pass-throughs to `Service` and `AdminService`. Both are mounted at their respective service paths in the ConnectRPC mux with `AuthBypass` middleware applied only to the agent service handler.

## Agent Daemon

### Daemon

The agent daemon (`internal/agent/daemon/daemon.go`) is the client-side binary. On startup, it loads or initializes state (agent_id, poll_token) from `<state_dir>/agent_state.json`. It enters a poll loop with exponential backoff reconnection; on each iteration, it drains queued reports and heartbeat from the dispatcher, sends a Poll RPC, and spawns a goroutine per new command.

The main loop calls `ensureRegistered` on startup: if state is empty, it issues a Register RPC with the bootstrap token, stores the returned agent_id and poll_token, and persists to disk. Then it enters `pollOnce`: it drains completed command reports from the dispatcher, builds an `AgentReport` carrying them, sends a Poll RPC with the report, and for each command in the batch, it either replays a cached report (from idempotency cache) or spawns a goroutine to run the command.

### State

Atomic state persistence (`internal/agent/daemon/state.go`) is a single JSON file at `<state_dir>/agent_state.json`. The Daemon reads it on startup and writes it on successful Register. If the server or agent crashes, the next poll loop resumes from the persisted state without re-registering.

### Idempotency Cache

The idempotency cache (`internal/agent/daemon/idempotency.go`) is an LRU with 256 entries, keyed on `command_id`. On duplicate command arrival (same command_id), it returns the cached `CommandReport` without re-running the verb. The cache is in-memory only; it survives agent restart via the command reports that are still queued in the `AgentReport` being sent to the server. This handles single-request transports (one-way loss or duplicate ACKs) but not across daemon restarts.

### Dispatcher

The dispatcher (`internal/agent/daemon/dispatcher.go`) is a registry of verb handlers keyed by `Action` protobuf type. `Dispatch` selects the handler based on `cmd.Action.Verb`, invokes it with the command, and returns a `CommandReport`. `DrainReports` collects queued reports from a buffered channel (256 capacity) and assembles an `AgentReport` with them, a heartbeat, and log batches. Verb handlers post reports and logs to channels; the dispatcher drains them periodically.

## Agent Daemon: Verbs

Each verb runs in its own goroutine spawned by the daemon's main poll loop. Verbs are invoked with a `Context`, command, and a log sink callback. They return a `CommandReport` with outcome (SUCCESS, FAILED, TIMEOUT) and verb-specific result.

### RunShell

Executes arbitrary shell commands. Takes `argv` (parsed), optional `shell=true` (join argv and run via `/bin/sh -c`), `stdin`, `cwd`, `env` dict, `timeout_seconds`, and `expected_exits` (list of allowed exit codes; default [0]). Spawns `exec.CommandContext` with the timeout, captures stdout/stderr, and pumps both streams to the log sink. If context deadline is exceeded, returns TIMEOUT outcome. Otherwise, compares exit code against expected_exits and returns SUCCESS or FAILED.

### PutFile

Writes a file to disk. Content is either inline (bytes) or fetched from HTTP(S) URL. For fetch, downloads to a temporary file while computing SHA256; if the hash mismatches the expected value, deletes the temp file and returns FAILED. For inline, writes directly. Both modes support atomic write via `.tmp` suffix and rename, `CreateParents` (mkdir -p directories), `Mode` (file permissions), and optional `Owner`/`Group` (uid/gid via name lookup).

### Systemctl

Runs `systemctl <op> <unit>` where op is one of: START, STOP, RESTART, RELOAD, ENABLE, DISABLE, IS_ACTIVE. Delegates to an injectable `Runner` interface (for testing) which executes the shell command. For IS_ACTIVE, captures and returns stdout in the result.

### InstallPackage

Resolves a package ID to an artifact (via an injectable `Resolve` callback) and installs it. The artifact specifies format (APT, DPKG, TARBALL, BINARY), URL, SHA256, and optional extraction path. Implementations:
- **APT**: Runs `apt-get install -y <package_name>` (respects `STROPPY_AGENT_SKIP_APT` env guard).
- **DPKG**: Downloads `.deb` to temp, runs `dpkg -i <file>`.
- **TARBALL**: Downloads tar.gz, verifies hash, extracts to `ExtractTo` path via `tar -xzf`.
- **BINARY**: Downloads executable, verifies hash, places at target with mode 0755.

### WaitFor

Polls a condition until it succeeds or timeout. Conditions are: PORT_OPEN (TCP dial), HTTP_OK (HTTP GET returns 200), FILE_EXISTS (os.Stat), PROCESS_RUNNING (ps grep). Polls every `interval_ms` (default 500) for up to `timeout_seconds`. Returns SUCCESS when condition passes, TIMEOUT on deadline, or FAILED if an unexpected error occurs (e.g., network error).

## CLI

The `stroppy-cloud agent` subcommand (`cmd/stroppy-cloud/cmd_agent.go`) loads agent config (server URL, bootstrap token, machine ID, role, state dir) from a YAML file (default `/etc/stroppy-cloud/agent.yaml` or via `--config` flag). Environment variable overrides are supported. It constructs a `Daemon` with the config and calls `Run(ctx)`, where `ctx` is signal-aware (SIGTERM, SIGINT trigger graceful shutdown and Deregister).

## Wiring

In `cmd/stroppy-cloud/cmd_server.go`, the server constructor assembles:
1. `CommandsRepo` — repository wrapper for `agent_commands` table
2. `Hub` — in-memory command queues and waiters
3. `BootstrapTokenStore` — token issuer/validator
4. `AgentService` — Register/Poll/Deregister business logic
5. `AgentAdminService` — GetAgent/ListAgents/TerminateAgent
6. Connect handlers for both services
7. Auth middleware extension to handle poll_token (Bearer token)

Node handlers in `internal/domain/services/system/handlers/` are wired with the Hub and call `hub.DispatchForNodeRun` instead of local mock execution. The Hub is also registered in the system's `NodeWorker` to enable DAG execution to proceed.

## Tests

All tests are green:
- `internal/domain/services/agent/hub_test.go`: Dispatch/Resolve round trip, timeout transitions, short-circuit for already-reported commands.
- `internal/domain/services/agent/service_test.go`: Register (happy path, invalid token), Poll (long-poll returns commands, routes reports), Deregister.
- `internal/domain/services/agent/bootstrap_test.go`: Issue/Verify/Invalidate JWTs, expiration.
- `internal/agent/daemon/daemon_test.go`: Register/Poll loop with mock server, idempotency replay.
- `internal/agent/daemon/dispatcher_test.go`: Dispatcher registration and verb dispatch.
- `internal/agent/daemon/idempotency_test.go`: LRU cache hit/miss and persistence.
- `internal/agent/daemon/state_test.go`: JSON persistence across restarts.
- `internal/agent/daemon/verbs/{run_shell,put_file,systemctl,install_package,wait_for}_test.go`: Per-verb success, failure, timeout, integrity checks.

Run all: `go test ./internal/agent/... ./internal/domain/services/agent/...` (green, ~2-3 minutes with testcontainer Postgres).

## Known Limitations

- **agent_commands persistence**: Proto codegen for the `agent_commands` table was not implemented in Plan 05. The Hub is entirely in-memory; if the server restarts, pending commands are lost. The short-circuit logic in `DispatchForNodeRun` relies on checking the database for already-reported commands, but once PENDING commands are lost, handlers will re-issue them on restart (safe, but inefficient). This will be addressed in Plan 06 with proper DB layer generation.

- **BootstrapTokenStore in-memory**: In production, the token store should use signed JWT validation without a separate Valkey check (simpler, stateless). The current implementation stores each `jti` in Valkey to enforce single-use, which is good for security but requires external state. For a fully stateless design, consider a signed token with server-side revocation lists or simpler TTL-only validation.

- **AgentAdminService not wired**: Get/ListAgents/Terminate RPC procedures are implemented but not exposed to Connect handlers in this plan. They will be wired in Plan 07 (admin).

- **Node handlers still use Hub, not persisted**: Real node handlers (terraform, docker, agent verbs) now dispatch via the Hub, but the system still uses the Hub for in-process command batching. Actual DAG execution (terraform apply, docker run, etc.) is not yet replaced — system continues with stubs from Plan 03. This is intentional: Plan 05 focuses on the agent framework; Plan 06+ will replace node handler implementations.

- **Logs are collected but not streamed**: The daemon collects log lines in the dispatcher's log sink and batches them in the AgentReport. The server receives them via the LogBus, but server-side log streaming (e.g., `/subscribe/logs/<node_run_id>`) is not yet built. This will be addressed in a future plan.

## Next Plan

Plan 06 — Ops (webhook outbox + quota enforcement + binary cache). The agent already calls `BinaryCache.Resolve` from `InstallPackage`, but the server-side implementation is a stub. Plan 06 will build the real BinaryCache service, operator-facing APIs for binary uploads, and quota enforcement gates for agent resource consumption.
