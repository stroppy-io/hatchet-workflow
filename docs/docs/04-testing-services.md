# Testing Services (Plan 04)

## Overview

Plan 04 delivers the complete testing tier of the platform. Seven service implementations provide the full domain for creating test templates, launching parameterized test runs and suites, monitoring progress in real-time, sharing results publicly, and comparing metrics between baselines. At the core, a `DagBuilder` bridges the user-facing `TestRun` abstraction into the system-level DAG engine from Plan 03, translating test specifications into executable task graphs with staging pipelines, workload execution, and optional cluster teardown. Event-driven progress streaming ties all services together via the in-memory bus: as the engine executes nodes, subscribers receive live updates without polling.

The testing tier is production-ready for single-node, two-node, and three-node database presets with Stroppy workload drivers. Streaming RPC endpoints (Watch/StreamLogs) are stubbed to return Unimplemented pending log infrastructure in Plan 05. The comparison service is wired to accept metrics queries but delegates to VictoriaMetrics adapter, which is provisioned on-demand during server startup.

## Services

**TemplateService** (`internal/domain/services/testing/template_service.go`) — manages reusable test templates. Key methods: `CreateTestRunTemplate`, `GetTestRunTemplate`, `UpdateTestRunTemplate`, `DeleteTestRunTemplate`, `ListTestRunTemplates`. Each operation enforces tenant-scoped IAM roles: editor for create/update/delete, viewer for read.

**TestRunService** (`internal/domain/services/testing/test_run_service.go`) — core service for launching individual test executions. CRUD: `CreateTestRun`, `GetTestRun`, `ListTestRuns`, `UpdateTestRun`, `DeleteTestRun`. Launch flow: `LaunchTestRun` calls DagBuilder to materialize the DAG, invokes `SystemEnginePort.LaunchDag`, persists the returned `DagRunId`, transitions status to RUNNING, and publishes a `TestRunLaunched` event. `InstantiateTestRun` creates a copy from a template with PENDING status. Monitoring: `WatchTestRun` subscribes to engine progress events and maps node updates to test step status; `StreamTestRunLogs` opens a live log stream for a specific step; `GetTestRunMetrics` queries the metrics adapter. `CancelTestRun` propagates the cancel signal through the engine.

**TestSuiteService** (`internal/domain/services/testing/test_suite_service.go`) — manages collections of test templates to execute together. Methods: `CreateTestSuite`, `GetTestSuite`, `UpdateTestSuite`, `DeleteTestSuite`, `ListTestSuites`, `CloneTestSuite`. A suite stores an ordered list of template IDs and optional sequential/parallel execution policy. `CloneTestSuite` deep-copies the template list with fresh ULIDs.

**TestSuiteRunService** (`internal/domain/services/testing/test_suite_run_service.go`) — executes an entire suite as a coordinated set of test runs. `LaunchTestSuite` materializes one TestRun per suite template (status=PENDING, spec copied), builds a meta-DAG via SuiteDagBuilder with `test_run_ref` task nodes, launches the meta-DAG, and returns a TestSuiteRun with a child-ID mapping. Parallel execution by default; sequential if suite policy dictates. `CancelTestSuiteRun` cancels the meta-DAG; the engine's test-run-ref handler cascades the signal to child runs. `GetTestSuiteRun`, `ListTestSuiteRunsByTenant`, `ListTestSuiteRunsBySuite`, `WatchTestSuiteRun`, `StreamTestSuiteRunLogs`, `GetTestSuiteRunMetrics` mirror the TestRun equivalents.

**SharedTestRunService** (`internal/domain/services/testing/shared_test_run_service.go`) — generates time-limited, revocable public share links for completed test runs. `CreateSharedTestRun` requires editor role on the owning tenant, generates a random 32-byte Base64 token, stores an expiration, and persists a snapshot pointer. `GetByToken` requires no authentication and returns the test run snapshot; returns NOT_FOUND for expired or revoked tokens to avoid leaking existence. `RevokeSharedTestRun` marks the share as revoked. Tokens are added to the auth middleware bypass list.

**SharedSuiteRunService** (`internal/domain/services/testing/shared_suite_run_service.go`) — mirrors SharedTestRunService against suite runs. Same token-based public access, same IAM enforcement for creators, same NOT_FOUND behavior for invalid tokens.

**ComparisonService** (`internal/domain/services/testing/comparison_service.go`) — diffs metrics between test runs. `CompareRuns` accepts baseline and candidate test run IDs, requires viewer role on both, queries the MetricsPort for average-over-time series for each metric, and returns per-metric percent change: `(candidate - baseline) / baseline * 100`. `CrossCompareBatch` applies the same logic to pairs. Results drive A/B testing decisions and performance regression detection.

## DagBuilder

Located in `internal/domain/services/testing/dagbuilder/`, this package converts test specifications into executable DAGs without importing the parent testing package (avoiding circular imports). The builder accepts a `CatalogPort` interface to fetch database and workload presets.

`Builder.FromTestRun(ctx, tr)` materializes a 4- or 5-node DAG: **provision** (TerraformTask to apply the database module with vars), **install** (InstallPackageTask to place the database binary on all hosts), **init** (OneShotTask running the database init script on the primary), **workload** (StroppyRunTask with workload and database preset IDs, Stroppy version), and optionally **teardown** (TerraformTask destroy). Dependencies chain linearly. If `tr.Spec.KeepClusterAfter` is true, teardown is skipped. All nodes are packed with proto-encoded task specs into `DagNode.Spec` using `anypb.New()`.

`Builder.FromTestSuite(ctx, suite, childTestRunIDs)` builds a meta-DAG of `test_run_ref` task nodes, one per child. If `suite.Spec.Sequential` is false (default), all nodes have empty `DependsOn`, allowing parallel execution. If sequential, each node depends on the previous, enforcing serialization. This allows suites to explore both wide and narrow execution patterns.

## Event-driven progress

The `system.Service.SubscribeProgress` method (added in Plan 04) consumes `NodeRunDone` and `DagRunDone` events from the in-memory bus. As NodeWorker executes tasks and publishes events, `MarkNodeRunSucceeded`, `MarkNodeRunFailed`, and `recomputeAfterNode` now emit `NodeRunDone` with updated node status. When all nodes complete, `DagRunDone` fires. Test: `internal/domain/services/system/subscribe_test.go` verifies channel delivery with multiple subscribers.

## Transport

All seven testing services are exposed via ConnectRPC handlers in `internal/transport/connect/testing_handlers.go`. Each handler struct holds a reference to its service and implements the corresponding Connect interface: `TestRunTemplateServiceHandler`, `TestRunServiceHandler`, `TestSuiteServiceHandler`, `TestSuiteRunServiceHandler`, `SharedTestRunServiceHandler`, `SharedSuiteRunServiceHandler`, `ComparisonServiceHandler`.

Unary methods (CreateTestRun, LaunchTestRun, etc.) are implemented as simple passthrough: extract request, call service, wrap response in `connect.NewResponse()`. Streaming methods (`WatchTestRun`, `StreamTestRunLogs`, `WatchTestSuiteRun`, `StreamTestSuiteRunLogs`) accept a `connect.ServerStream` and loop over the service's returned channel, forwarding each item via `stream.Send()` until the channel closes or context cancels. `CancelTestRun` and `GetTestRunMetrics` return Unimplemented for now (engine cancel mechanism and metrics hydration pending); callers see a gRPC error code directing them to future releases.

## SDK + CLI

The SDK client (`internal/sdk/client/testing.go`) bundles all seven Connect clients under `TestingClients` and is accessible from the main SDK root. CLI subcommands in `cmd/stroppy-cloud/cmd_cli/` provide human-friendly interfaces:

- **run**: `start` (instantiate + launch + optionally watch), `list`, `get`, `cancel`, `logs` (with optional `--step` filter and `--follow`), `metrics` (accepts `--query <promql>`), `share` (generates token, prints TTL).
- **suite**: `list`, `create` (with `--templates` comma-list), `launch`, `cancel`, `get`.
- **template**: `list`, `create` (accepts `--spec` YAML file path), `delete`, `instantiate` (prints resulting TestRun ID).
- **compare**: `runs <baseline-id> <candidate-id> --metric <name>,...` renders a table of percent changes.

Watch mode uses a live progress loop or text-only table; logs with `--follow` streams until Ctrl-C; share tokens print with TTL and are usable immediately with `curl https://api/testing.v1/SharedTestRun.GetByToken --header "Authorization: Bearer <token>"`.

## Wiring

`cmd/stroppy-cloud/cmd_server.go` constructs all testing services in dependency order: ports first (CatalogPort, SystemEnginePort, MetricsPort), then services (TemplateService, TestRunService, TestSuiteService, TestSuiteRunService, SharedTestRunService, SharedSuiteRunService, ComparisonService). DagBuilder is instantiated once and shared. Scheduler is wired to TestRunService for template-based triggering. All seven handlers are registered with the ConnectRPC mux under their respective service paths. The auth middleware's bypass list is extended to include `/testing.v1.SharedTestRunService/GetByToken` and `/testing.v1.SharedSuiteRunService/GetByToken` so anonymous users can redeem share tokens.

## Tests

Integration tests in `internal/domain/services/testing/` exercise all services with real Postgres testcontainers and mock system engine. Test coverage includes:

- **TemplateService**: create/get/update/delete/list with role boundary checks.
- **TestRunService**: create/launch/cancel, instantiate from template, status transitions, GetTestRun with step hydration, List scoped to tenant, Update (metadata only), Delete (terminal status only).
- **TestSuiteService**: create/get/update/delete/list/clone, preserving template list.
- **TestSuiteRunService**: launch (materializes children, persists meta-DAG), cancel propagation, get, list variants, watch.
- **SharedTestRunService**: create with TTL, get by token, revoke, expiration + revocation rejection.
- **SharedSuiteRunService**: mirror of SharedTestRunService.
- **ComparisonService**: compare two runs, percent-change calculation against mock metrics.
- **DagBuilder unit tests**: `FromTestRun` produces 4-node or 5-node DAG depending on KeepClusterAfter; `FromTestSuite` produces N-node meta-DAG with correct dependencies for sequential vs. parallel; nil workload presets handled gracefully.
- **System subscribe test**: verifies event-channel delivery to multiple subscribers.

Run `go test ./internal/domain/services/testing/... -count=1` for full integration suite (~3 minutes with Docker provisioning).

## Known limitations

- **Task 13 (scheduler `fire()` integration)** deferred pending Schedule proto augmentation. The Schedule message must include a `template_id` field to link schedules to test templates; once added, the scheduler can call `TestRunService.InstantiateAndLaunch` directly.
- **Streaming RPC responses** (`WatchTestRun`, `StreamTestRunLogs`, `WatchTestSuiteRun`, `StreamTestSuiteRunLogs`) currently return Unimplemented. Event marshaling and log tail logic will be finalized in Plan 05 alongside Agent integration.
- **ComparisonService.MetricsPort** is wired but points to a nil adapter. VictoriaMetrics integration is provisioned on-demand in cmd_server and surfaces through infrastructure/victoria; the adapter is pending availability of the Victoria client SDK.
- **CancelTestRun** is stubbed to return Unimplemented. System.CancelDagRun requires a schema migration to add a `cancel_requested` column to the dag_runs table; once in place, the TestRunService can forward the cancel and mark status as CANCELING.

## Next plan

Plan 05 — Agent + 5-verb command protocol + AgentService + command journal for idempotent execution. Real node handlers (terraform, install, stroppy_run) will be dispatched to remote agents via the Hub; event streaming and log aggregation will be completed.
