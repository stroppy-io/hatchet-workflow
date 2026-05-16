# Repository Guidelines

stroppy-cloud is a Go control plane for stroppy benchmark fleets — ConnectRPC services, ratel-generated proto repositories, in-memory DAG engine + worker pool, an agent daemon talking to remote VMs, and a React SPA. Single binary (`stroppy-cloud`) ships server, agent, and CLI subcommands.

## Project Structure & Module Organization

- `protocols/cloud/v1/` — proto sources, one package per domain (`iam`, `catalog`, `system`, `testing`, `agent`, `ops`, `admin`, `stroppy`, `errors`, `common`, `tasks`). Single source of truth for API + data schema.
- `internal/proto/cloud/v1/` — generated `.pb.go` + `_plain.pb.go` + `_ratel.pb.go` + `_connect.go`. Never hand-edit; regenerate via `make proto-gen`.
- `internal/core/` — building blocks: `configurator`, `domainerr`, `eventing`, `ids` (ULID), `tracing`, `logger`, `shutdown`.
- `internal/domain/services/<pkg>/` — business logic per domain. Stateless struct holding `ProtoRepository` instances + `pgtx.TxManager` + `eventing.Bus`.
- `internal/domain/workers/` — long-running goroutines: `nodeworker` (DAG executor pool, `FOR UPDATE SKIP LOCKED`), `scheduler` (cron + lease), `recovery` (startup sweep), `webhookworker` (outbox drain).
- `internal/transport/connect/` — ConnectRPC handlers. Thin: validate → resolve user/tenant from ctx → call service → wrap response.
- `internal/transport/middleware/` — interceptors: recovery, requestID, auth (JWT), tenant, protovalidate, idempotency (valkey-backed), error mapping, RequirePlatformAdmin.
- `internal/infrastructure/` — adapters: `postgres` (pool + ratel migrate + `pgtx` from avito-tech), `valkey`, `s3` (aws-sdk-go-v2), `stroppybin` (subprocess runner), `terraform`, `victoria`.
- `internal/sdk/client/` — typed wrapper around generated Connect clients. Used by CLI + tests + external automation.
- `internal/testutil/` — `pgcontainer` (testcontainers Postgres + schema-per-test isolation), `fixture` (entity factories), `testserver` (in-process HTTP server for handler tests).
- `internal/agent/daemon/` — agent-side: register/poll loop + dispatcher + 5 verbs (`PutFile`, `RunShell`, `Systemctl`, `InstallPackage`, `WaitFor`).
- `cmd/stroppy-cloud/` — single binary entry: `main.go`, `cmd_server.go`, `cmd_agent.go`, `cmd_cli/`.
- `tests/e2e/` — `//go:build e2e` Go tests against in-process server + Playwright TS specs.
- `web/` — Vite + React 19 + TypeScript + Tailwind SPA (in-flight migration to ConnectRPC).
- `deployments/local/server/config.yaml` — example local server config; `deployments/docker/` — Dockerfile.
- `docs/superpowers/plans/` — implementation plans by phase (02–09); `docs/docs/` — delivered-feature docs.

## Build, Test, and Development Commands

- `make proto-gen` — regenerate Go + TS proto artifacts via easyp. Run after any `.proto` edit.
- `make migrate-clear` — regenerate migrations from proto. Pre-v1 only; wipes existing `internal/infrastructure/postgres/migrations/*.sql`.
- `make build` — build the `stroppy-cloud` binary with embedded SPA.
- `make build-all` — multi-platform release builds.
- `make test-unit` — pure unit tests (no DB).
- `make test-db` — DB-backed integration (auto-starts a postgres testcontainer).
- `make test-full` — `test-unit` + `test-db`.
- `make test-integration` — full integration suite (requires Docker).
- `make smoke` — bring up stack + login + tiny postgres run end-to-end.
- `make server-run` — run the server locally against `deployments/local/server/config.yaml`.
- `make docker-up` / `make docker-down` — Postgres + Valkey + VictoriaMetrics stack.
- `make web-dev` — Vite SPA dev server. `make web-build` — production SPA bundle.

## Coding Style & Naming Conventions

- Go: standard `gofmt` + `goimports`; run `make fmt` + `make lint` (`golangci-lint`).
- One package per directory; `internal/domain/services/<pkg>/service.go` holds the constructor.
- Service methods accept `context.Context` + proto messages, return proto + `*domainerr.Error`. No naked strings for errors at the domain boundary — use `domainerr.E(code)` + `Detail`.
- Wrap mutations in `tracing.WithTraceRet` + `pgtx.WithSerializableRet` sandwich (see `internal/domain/services/catalog/database_presets.go` for the canonical pattern).
- ratel real API: `repo.Execute(ctx, Table.Insert().From(scanner.AllSetters()...))`, `repo.QueryRow(ctx, Table.SelectAll().Where(...))`, `Update().Set(...).Where(...)`. **Not** the speculative `Insert/SelectOne/FindByID/Delete` shorthand from older plan text.
- TypeScript (web/): functional components, PascalCase exports, kebab-case file names, `@/*` alias for `web/src` imports. Prettier + ESLint via `lint-staged`.
- Proto: snake_case fields, PascalCase messages, screaming-snake-case enum values prefixed with type. ULID IDs via `*Id { string value = 1 [(validate.rules).string.len = 26] }`. Embed `cloud.v1.common.Timestamps` for soft fields.

## Testing Guidelines

| Category | Where | Speed |
|----------|-------|-------|
| Unit | `internal/core/...` | <30 s |
| Integration | `internal/domain/services/<pkg>/*_test.go` | 3–5 min |
| Transport | `internal/transport/connect/*_test.go` | <2 min |
| E2E | `tests/e2e/` (`-tags=e2e`) | 5–15 min |

- One container per package run, isolated schemas per test allow `t.Parallel()`.
- No DB mocks. No pool mocks. Real ratel + real SQL exercised.
- Acceptance: `go test ./internal/... -count=1 -timeout 600s` must be green before merging.
- E2E: `go test -tags=e2e ./tests/e2e/... -count=1 -timeout 300s`.
- Probe tests requiring a real `stroppy` binary use the `STROPPY_PROBE_BIN` env override; otherwise skipped.

## Commit & Pull Request Guidelines

- Refactor branch (`refactor`) uses linear commits, short imperative subjects, conventional-commits-style prefix: `feat(<area>):`, `fix(<area>):`, `chore:`, `test(<area>):`, `docs:`, `refactor:`. No `Co-Authored-By` or generator trailers.
- Commit message body explains the WHY, not the WHAT (the diff already shows what).
- Pull requests: link the docs/superpowers plan task being implemented, list verification commands run (`go test ./... && make test-integration`), screenshots for SPA changes.
- Never push to `main` without explicit approval. Pre-v1: rebase + force-push to `refactor` is acceptable; everywhere else, follow standard `--ff-only`.

## Repo-Specific Knowledge (Accumulated)

### Proto / ratel

- `make migrate-clear` is destructive (pre-v1 only). After any proto schema change run it to regenerate the single rolled-up migration; do **not** hand-edit SQL files in `internal/infrastructure/postgres/migrations/`.
- Generated table pluralization is naive: `DagRunStateEntry` → `DagRunStateEntrys` (not `…Entries`). Trust the symbol that `_ratel.pb.go` actually emits.
- Generated enum String() round-trips to DB text (e.g. `"NODE_RUN_STATUS_READY"`); claim SQL and recovery SQL use those literal strings.
- `oneof` accessor names follow the inner field, not "Inline"/"PresetId": for `DatabaseOrPreset` the variants are `Variant: &DatabaseOrPreset_Database{...}` / `DatabaseOrPreset_DatabasePresetId{...}` (getters `GetDatabase()` / `GetDatabasePresetId()`).
- Some ratel column types resolve to `*string` rather than `string` (nullable text FK to oneof preset IDs). pgx can't scan `NULL` into a non-pointer `*string` directly. Workaround: keep a `safeSelectCols` list in the service that omits those columns from `SelectAll(...)`; the empty default ("") in the scanner is fine and `IntoPb` ignores empty preset IDs.
- JSONB columns marshalled via `protojson` for proto sub-messages (Graph, Metadata, etc.) sometimes serialize as `[]byte{}` on empty input — Postgres rejects empty as invalid JSON. Always coerce to `[]byte("{}")` or `nil` before insert. See `IntoPlain` callsites.
- Some scanner fields back NOT NULL columns with zero values (e.g. `Label []string`). Add `if scanner.Label == nil { scanner.Label = []string{} }` before insert.
- `anypb.Any` JSON marshal fails for unregistered TypeUrls (e.g. test stubs like `"type.googleapis.com/Mock"`). Don't include `Spec: &anypb.Any{TypeUrl: "Mock"}` in test Dag templates — leave Spec nil and let the MockHandler match by `""` kind.
- The handler `Registry.Resolve(typeURL)` matches by case-insensitive suffix. `MockHandler.Kind() == ""` makes it a catch-all — fine for tests, not production.
- DagRun cancel propagation: `cancel_requested` column lives on `dag_runs` (added in tech-debt cleanup). NodeWorker checks it before executing each claim; if set, marks NodeRun failed with reason "cancelled".

### Patterns to mirror

- New service: copy `internal/domain/services/catalog/database_presets.go` shape. Constructor takes `(exec.DB, pgtx.TxManager, eventing.Bus)`. Methods wrap in `tracing.WithTraceRet` + `pgtx.WithSerializableRet`. Soft-delete via `deleted_at` UPDATE returning the pre-delete row.
- New ConnectRPC handler: copy `internal/transport/connect/catalog.go`. Tenant from `middleware.TenantFromCtx(ctx)`, caller from `middleware.UserFromCtx(ctx)`. Delegate to service; never read request fields not present in the proto.
- New CLI subcommand: copy `cmd/stroppy-cloud/cmd_cli/preset.go`. Use `authedCLI()` helper + `printJSON()` from `cmd_cli/`.
- Event publication: `s.events.Publish(ctx, eventing.Event{Topic: eventing.TopicX, Payload: typedStruct})`. Subscribers receive `eventing.Event{}` and type-assert the payload.

### Workflow

- After proto changes: `make proto-gen-go && make migrate-clear && go build ./...` in that order.
- Plans live under `docs/superpowers/plans/2026-05-15-NN-*.md`. Per-feature delivery docs go to `docs/docs/NN-*.md`. Use the haiku model for doc-writing tasks.
- Integration tests rely on testcontainers + pgcontainer; cold start ~5 s, warm ~1 s. Don't add `t.Parallel()` to tests that mutate the shared GitHub stub on the stroppy fixture.

## Behavioral Guidelines

### 1. Think Before Coding

Don't assume. Don't hide confusion. Surface tradeoffs.

Before implementing:

- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them — don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

### 2. Simplicity First

Minimum code that solves the problem. Nothing speculative.

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If 200 lines could be 50, rewrite it.

The test: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

### 3. Surgical Changes

Touch only what you must. Clean up only your own mess.

When editing existing code:

- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If unrelated dead code surfaces, mention it — don't delete it.

When your changes create orphans:

- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

Every changed line should trace directly to the user's request.

### 4. Goal-Driven Execution

Define success criteria. Loop until verified.

Transform tasks into verifiable goals:

- "Add validation" → "Write tests for invalid inputs, then make them pass."
- "Fix the bug" → "Write a test that reproduces it, then make it pass."
- "Refactor X" → "Ensure tests pass before and after."

For multi-step tasks, state a brief plan:

```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria require constant clarification.

### 5. Agentic Behavior — Critical

Always propose before acting. Before writing code, editing files, running commands, or installing packages:

1. State what you plan to do and why, in one or two sentences.
2. Wait for acknowledgement when the change touches shared infrastructure (proto, migrations, config.yaml, Dockerfile, CI).
3. For local edits with reversible blast radius, proceed and report results.
4. Never push, force-push, or run destructive `git`/`make` targets (`migrate-clear`, `docker-down`, `smoke-clean`) without explicit instruction.

## Security & Configuration

- Never commit secrets or local `.env` values. JWT secret + S3 credentials + INITIAL_ADMIN_PASSWORD come from env vars referenced in `config.yaml`, never literal in the YAML.
- Review `protocols/`, `deployments/`, `internal/infrastructure/postgres/migrations/` changes carefully — they affect wire schema, runtime infra, and DB layout for every tenant.
- Admin endpoints (`adminconnect.*` + `BinaryCacheAdminService`) require `platform_role=PLATFORM_ROLE_ADMIN` via the `RequirePlatformAdmin` middleware. Don't bypass.
- Agent bootstrap tokens are HS256 JWTs signed by the server with a 10 min TTL — never log them, never share between tenants.
