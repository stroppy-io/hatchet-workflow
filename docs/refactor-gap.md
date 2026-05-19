# Refactor Feature-Parity Gap (vs `main`)

**Status as of 2026-05-18.** Audit of `main` (chi-REST stack, 93 routes) against
`refactor` (ConnectRPC). Goal: every prod feature works 1:1 after `refactor`
merges. No silent removal.

## Legend

| Status | Meaning |
|---|---|
| ✅ | Real implementation, parity test or e2e demo |
| 🟡 | Skeleton in tree, behaviour incomplete or stubbed |
| 🔴 | Lost / regressed; needs implementation |
| ➕ | Net-new in refactor (out of parity scope) |

## A — Backend: stub → real

| # | Feature | `main` source | refactor target | Status |
|---|---|---|---|---|
| A1 | Cancel TestRun / DagRun | `/run/{id}/cancel` + run package | `testing.CancelTestRun` → `system.CancelDagRun` (cancel_requested col) | 🔴 column persisted, service still UNIMPLEMENTED |
| A2 | Watch run progress | snapshot endpoints + push | `testing.WatchTestRun` server-stream | 🔴 handler returns UNIMPLEMENTED |
| A3 | Stream run logs | `/ws/logs`, `/ws/logs/{runID}` + VictoriaLogs | `testing.StreamTestRunLogs` + `node_run_logs` ratel + LogBus | 🔴 handler UNIMPLEMENTED; agent collects but server-side LogBus missing |
| A4 | Run metrics | `/run/{id}/metrics` + VictoriaMetrics REST | `testing.GetTestRunMetrics` via MetricsPort | 🔴 handler UNIMPLEMENTED, `comparison.NewComparisonService(nil)` |
| A5 | Yandex Terraform provider | `internal/infrastructure/terraform/actor.go` driven by run package | `nodeworker/handlers/terraform.go` | 🟡 skeleton, no real Actor wiring |
| A6 | Docker provider | `internal/domain/agent/deploy_docker.go` | `nodeworker/handlers/docker.go` | 🟡 skeleton |
| A7 | Agent report flow | dedicated `/agent/report` + `/agent/logs-batch` | `agent.Poll` consumes `AgentReport{Heartbeat, Reports[], LogLines[]}` | 🟡 Reports[] flow OK, LogLines[] dropped on floor |
| A8 | Agent binary download | `/agent/binary` (public) | new HTTP route + binary_artifacts lookup | 🔴 missing |
| A9 | Cached binary proxy | `/api/binaries/{name}/{ver}/{file}` | new HTTP route via BinaryCacheService | 🔴 missing |
| A10 | Package binary up/download | `/packages/{id}/deb`, `/upload/deb`, `/upload/rpm`, `/packages/*` | mixed Connect svc + HTTP route (proto can't stream) | 🔴 UI calls REST still |
| A11 | `/metrics` Prometheus | `r.Get("/metrics", ...)` | promhttp on mux | 🔴 not mounted |
| A12 | Server health | `/health` JSON {version, uptime, ...} | currently `/healthz` returns empty 200 | 🔴 lost shape |
| A13 | Baselines CRUD | `/baseline/{name}`, `/baselines` | no proto / no service | 🔴 lost |
| A14 | Built-in packages seed | seeded on `createTenant` admin call | `iam.CreateTenant` doesn't seed | 🔴 lost |
| A15 | Suite items CRUD + reorder | `/suites/{id}/items[...]`, `/items/reorder` | Suite.Matrix shape, no item endpoints | 🔴 lost |
| A16 | Cross-compare batch | `/suites/{id}/batches/{b}/cross` | `comparison.CrossCompareBatch` declared | 🔴 handler UNIMPLEMENTED |
| A17 | Grafana settings | `/api/v1/grafana` returns config | settings keys exist but no endpoint shim | 🔴 missing; SPA still calls REST |
| A18 | Tenant `select-tenant` | `/auth/select-tenant` | client-side localStorage + X-Tenant-Id header | 🟡 functional change; documented |
| A18b | Login identifier = email OR nickname | n/a (main had email-only) | `auth.Login` branches on `@` to pick column | ✅ admin/admin works on dev |
| A19 | User self password change | `/api/v1/auth/password` | `UserService.UpdatePassword` declared | ✅ service + handler wired (`iam/update_password.go`) |
| A20 | Cancel suite batch | `/suites/{id}/batches/{b}/cancel` | `CancelTestSuiteRun` declared | 🔴 not wired |
| A21 | Run validate / dry-run | `/validate`, `/dry-run` | `TestRunService.DryRunTestRun` | ✅ proto+svc+handler; returns Dag preview |
| A22 | Rendered configs view | `/run/{id}/rendered-configs` | folded into `DryRunTestRun` Dag (ConfigApplyTask specs) | ✅ via A21 |
| A23 | Suite items | `/suites/{id}/items*` | `TestSuite.Matrix` (databases × workloads cartesian) | ✅ existing Matrix shape covers items |
| A24 | Stroppy probe | `/probe` REST | `StroppyService.ProbeStroppyConfig` ✅ | ✅ backend; SPA still calls REST in places |
| A25 | Stroppy versions/commits | `/stroppy-versions`, `/stroppy-commits` | `StroppyService.ListStroppy*` ✅ | ✅ backend; SPA mixed |
| A26 | Idempotency middleware | n/a | Valkey-backed Connect interceptor | ➕ new |
| A27 | Webhook outbox | n/a | WebhookDelivery + worker + HMAC | ➕ new |

## B — DAG builder real engine coverage

| # | Feature | `main` source | refactor target | Status |
|---|---|---|---|---|
| B1 | Full DAG per engine | `internal/domain/run/builder.go` + 29 typed tasks | `dagbuilder.FromTestRun` produces 4 generic nodes | 🟡 skeleton, no real install/config/init/monitor steps |
| B2 | Engine: Postgres + Patroni + pgbouncer + etcd + haproxy | `setup_*_postgres.go`, `setup_patroni.go`, `setup_pgbouncer.go`, `setup_etcd.go`, `setup_haproxy.go` | needs PackageInstall+ConfigApply+OneShot graph | 🔴 lost |
| B3 | Engine: MySQL + proxysql | `setup_mysql.go`, `setup_proxysql.go` | needs MariaDB variant too | 🔴 lost |
| B4 | Engine: Picodata | `setup_picodata.go` | new DAG branch | 🔴 lost |
| B5 | Engine: YDB (static cluster) | `setup_ydb.go` + blobstorage init | OneShot init steps | 🔴 lost |
| B6 | Engine: Cockroach | `setup_cockroach.go` + `cockroach init` | OneShot init | 🔴 lost |
| B7 | Managed YDB variant | separate TF module + post-create config mutation | needs DAG branch | 🔴 lost |
| B8 | External DB variant | skip infra, supply endpoint to stroppy | needs DAG short-circuit | 🔴 lost |
| B9 | Monitor stack on agents | vmagent + node_exporter + mysqld_exporter + postgres_exporter; log shipping | needs monitor-setup DAG step + ConfigApply templates | 🔴 lost |

## C — Frontend audit

| # | Page | Status |
|---|---|---|
| C1 | Login | ✅ on Connect (AuthContext) |
| C2 | SelectTenant | ✅ on Connect (ListMyTenants) |
| C3 | Runs / RunDetail / NewRun | 🟡 minimal Connect ports; legacy wizard pages lost (DAG viz, log stream, metrics panel, agents panel, dashboard selector, rerun/share dialogs) |
| C4 | Compare | 🟡 Connect comparison call, but MetricsPort nil on backend → empty diffs |
| C5 | SharedRun | 🟡 Connect getByToken; old `/api/share/:token` removed; UI URL still works (page renders by token) |
| C6 | Packages | 🟡 list on Connect; upload/download still REST |
| C7 | Presets / PresetDesigner | 🟡 model heavily reshaped; legacy form fields dropped |
| C8 | RunPresets | 🟡 maps onto TestRunTemplate but lost some form fields |
| C9 | Suites / SuiteBuilder / SuiteDetail | 🟡 items+reorder UI lost (backend gap A15) |
| C10 | Settings | 🟡 generic key/value; YC validation form lost |
| C11 | AdminTenants / AdminUsers | ✅ on Connect |
| C12 | TenantMembers / TenantTokens | ✅ on Connect |
| C13 | ServerHealth | 🔴 hits removed REST `/health` JSON shape |
| C14 | Dashboard | 🟡 placeholder |
| C15 | Grafana iframe | 🔴 calls removed `/api/v1/grafana` |

## D — CLI parity

| # | Command | `main` | refactor |
|---|---|---|---|
| D1 | `bench` (cloud, wait until terminal) | implemented | 🔴 no equivalent |
| D2 | `wait` (poll status) | implemented | 🔴 no equivalent |
| D3 | `validate` / `dry-run` | implemented | 🔴 lost |
| D4 | local run (full execution from config, no server) | implemented | 🔴 deferred — `cli run --watch` against `make server-run` covers dev/CI; full in-process engine = future work |
| D5 | `run/suite/preset/package` CRUD | implemented | ✅ ported |

## Execution order

Each `🔴` / `🟡` row above corresponds to a TaskCreate-tracked task in this
session (A11-A24 = tasks #11-#24, B = #25-#26, C = #27, D = #28). Order:

1. Foundation (A11, A12) — observability + healthz JSON, prerequisite for
   honest progress signal.
2. Async APIs (A2, A3, A4, A7) — Watch/Logs/Metrics + agent report flow;
   unblocks UI re-wire (C3, C4).
3. Engine reality (A5, A6, B1-B9) — DAG builder + real handlers; this is
   the biggest single chunk and the actual product value.
4. Misc backend gaps (A1, A8-A10, A13-A24).
5. Frontend (C) and CLI (D) after backend solid.

## Acceptance bar per row

Not green until ALL of:
- Integration test exists and passes (or e2e demo with steps documented).
- Touched routes return parity-correct shape to a `main`-style client (or
  matched proto consumer).
- No `connect.NewError(CodeUnimplemented, ...)` left in the code path.
- Doc row above flipped to ✅ with a link to the integration test.
