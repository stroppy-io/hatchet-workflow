# Dev context — stroppy-cloud BDD spec → incremental development

Read this first in a new session, then `features/README.md`. Goal: start
implementing the system against the BDD spec built in the prior session, area by area.

## What this is
**stroppy-cloud** — database benchmarking orchestrator (Go, one binary = server + agent).
Branch `refactorv2` is a mid-rewrite: NEW `protocols/cloud/v1` (proto canon) + `internal/runtime`
(the engine); OLD logic lives in `internal/domain/*` and is being recast.

A prior session produced a full **BDD living spec** (the contract) + real proto changes.
Now: implement it incrementally.

## Where things are
- `features/README.md` — root of the spec: product goals, spine, entity map, the **backlog
  + decisions/holes tracker H1–H66** (this IS the implementation backlog).
- `features/**/*.feature` — ~37 English Gherkin specs grouped by capability. Tags:
  `@migration` = recast of old behavior the new code MUST satisfy; `@future` = roadmap (not now);
  `@invariant` = core contract.
- `protocols/cloud/v1/**.proto` — proto canon (single source of truth). Architectural decisions
  recorded inline as `BDD decision (...)` doc-comments.
- `internal/runtime/*` — the new Dag engine (executor.go, processor.go) + extensive Go tests
  (the test names are the behavior catalog the engine features mirror).
- `internal/domain/*`, `internal/infrastructure/*` — OLD code = the behavior to recast.
- Auto-loaded memory `project_bdd_organization.md` — the session summary.

## Architecture (the model)
- Spine: **Define → Render → Plan → Execute → Collect**. Define↔Render↔Plan is the wizard
  (human-in-the-loop) — user overrides at any step before Execute.
- **Everything executable = `primitive.Dag`.** Runtime is domain-agnostic; the domain compiles
  entities → Dag (the "planner", C12). Agents only poll, never pushed to.
- Layers: `domain/*` = intent DTOs; `models/*` = persisted ratel tables; `runtime/*` = execution
  primitives + observations (NOT persisted); `api/{ui,admin,agent}/*` = RPC services (connect-go).
- Cross-cutting decisions (all in spec/proto):
  - `render.Binding` (typed late-binding: token + component_ids + string attr) resolved at
    plan→execute from `Deployment.Output`/`TfOperation.Output`. Replaces string placeholders.
  - Invariant **preview == execution** (byte-for-byte except declared bindings).
  - Agent = dumb generic executor of 6 ops; per-engine recipes = planner DATA; on-host work =
    per-command sub_dag; provisioning (terraform/docker) = server-locus task nodes (`ExecutionLocus`).
  - No shared mutable State → node outputs + bindings. Run state = `Dag.status`.
  - Lease = **pg** authoritative (`models.Dag.Processor` + dags_claim_idx; `models.Agent`); Valkey
    = optional fast advisory only.
  - Quotas = admission controller before start (runtime domain-agnostic). **Cost is not a type** =
    `deployment.QuotaRequest`. Cloud = source of truth for quota/network (reconcile).
  - Auth: 3 principals — Account JWT (access+refresh; refresh in Valkey TTL), agent machine JWT,
    API token (capped ≤ ADMIN). RBAC per-tenant `TenantMember.Role` (VIEWER<ADMIN<OWNER) +
    `Account.is_admin`. EVERY ui request carries `tenant_id`. Full matrix: `tenancy/rbac.feature`.
  - Metrics/logs = runtime observations queried from VictoriaMetrics/Logs; embedded Grafana stays
    direct HTTP; our proto = additive summaries; **self-describing** (no metric-name enums).

## Rules / working mode (follow these)
- **Discuss-then-write.** Research the mechanic → present 2-3 options/forks → ASK clarifying
  questions → get the answer → THEN write. Do NOT batch-dispatch agents before decisions are agreed.
- Drive **area by area**; ask on real forks, pick obvious defaults otherwise and say so.
- **Reuse canon/existing types** — grep `models/*` and `runtime/system/*` before inventing
  (e.g. `system.Cidr`, Id wrappers, `models.Dag.Processor`). Verify against canon first.
- Defaults are **wizard behavior + sensible values**, not boolean flags in entity state.
- Prefer **self-describing data** over hardcoded enums for open sets; **backend data**
  (compat matrices, metric defs, install recipes) over proto for things that change often.
- **No git commit/merge/push** without explicit permission; changes stay unstaged.
- **No `yc` CLI** (terraform + YC_TOKEN env only). Agents poll server, never push.
- Proto gotcha: `*/` inside a `/* */` doc-comment closes it early (once broke codegen).
- Validate proto: `cd protocols && protoc -I . -I easyp_vendor --descriptor_set_out=/dev/null <files>`.
  Regen: `make protocols` (needs dep `connectrpc.com/connect`).

## State
- Spec: ~37 `.feature` / ~290 scenarios, English, areas A–G + `future`. Proto: 51 files, protoc-OK;
  `make protocols` regenerates clean Go+TS.
- **Pre-existing build break (NOT from spec work):** `internal/domain/run/task_infra.go` imports a
  missing `deployments/terraform/yandex_managed_ydb` (only `.../yandex` exists) — old refactorv2 WIP.
- Pending: wire **godog** (specs don't execute yet); materialize the H-tracker decisions into Go;
  the `@future` items (resilience, rate/retention, MCP, agentic analysis).

## How to start development (incremental)
1. Open `features/README.md` → the **H1–H66 tracker** is the backlog (each = a decision/hole).
2. Pick one area/hole. The matching `.feature` = the contract; the proto = the types; the old
   `internal/domain/*` = the behavior to recast.
3. Good first targets:
   - **Wire godog** on `engine/*` (the runtime already has tests + helpers) → make the spec
     executable, get a regression gate for the core engine.
   - **The planner** (domain → `primitive.Dag`, C12) — the keystone bridge; everything depends on it.
   - **Auth + tenant scoping** (A) — needed by every ui RPC.
4. For each: discuss the approach with me, confirm forks, then implement against the feature + proto.
