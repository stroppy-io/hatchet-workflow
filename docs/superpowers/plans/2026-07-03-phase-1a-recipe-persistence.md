# Phase 1A: Recipe Persistence + RecipeService Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Persist DSL recipe bundles (cluster.yaml + workflow.yaml + components/** + providers/**) in postgres and expose a tenant-scoped `RecipeService` connect API (CRUD + Compile/Check delegating to the existing dsl compiler).

**Architecture:** komeet pattern — jsonb `data`-envelope table + hand-written SQL → `make db-gen` → `RecipeRepo` mirroring `SuiteRepo`. New proto `models.RecipeRecord` + `api.RecipeService`. Service reuses `internal/dsl.Compile` and `internal/services/dsl` logic over the stored bundle. Registered in `run.go` alongside `dslService`.

**Tech Stack:** Go 1.25, pgx+pgtx+bob (sqld codegen), connectrpc, easyp proto toolchain, existing internal/dsl compiler.

## Global Constraints

- Module `github.com/stroppy-io/stroppy-cloud`; branch `feat/yaml-dsl-pivot`.
- Commits: Conventional Commits, imperative lowercase ≤72 chars, NEVER `Co-Authored-By`/AI attribution.
- Spec: `docs/superpowers/specs/2026-07-03-flow-wiring-design.md` §4.1.
- Greenfield: no backward-compat with preset/suite storage.
- komeet DB pattern is mandatory — mirror `SuiteRepo` (`internal/infrastructure/postgres/records.go:69`), `suite_records` (`schema.sql:53`), `queries/records.sql` suite block, `models.SuiteRecord` (`protocols/cloud/v1/models/suite.proto`), `api.SuiteService` (`protocols/cloud/v1/api/suite.proto`).
- Tenant scoping: every repo method `(ctx, tenantID, id, ...)`; service reads `Authn.Caller(ctx)` + `req.tenant_id`; method-auth via `(cloud.v1.iam.auth)` proto option.
- Recipe bundle files = `map<string,bytes>` keyed by slash-path (same keys as `include.Sources`).
- No deploy. All tests local (pgtx tx-test harness + in-memory/fake). No bare `go mod tidy`.
- `make protocols` must commit ONLY new/intended files (revert unrelated TS regen drift — known easyp issue).

---

### Task 1: Proto — models.RecipeRecord + api.RecipeService + RESOURCE_RECIPE

**Files:**
- Create: `protocols/cloud/v1/models/recipe.proto`, `protocols/cloud/v1/api/recipe.proto`
- Modify: `protocols/cloud/v1/iam/permission.proto` (add `RESOURCE_RECIPE` enum value)
- Generated: `internal/proto/cloud/v1/{models,api,iam}/*` via `make protocols`

**Interfaces:**
- Produces (Go, `modelspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"`, `apipb ".../api"`):

```proto
// models/recipe.proto
message RecipeRecord {
    common.Entity entity = 1 [(validate.rules).message.required = true]; // id/tenant_id/name/description/timings
    RecipeBundle bundle = 2 [(validate.rules).message.required = true];
    uint32 version = 3;        // immutable version number; (tenant_id, name, version) unique
    Summary summary = 4;       // denormalized: provider name, machine-group count, service count
    message Summary {
        string provider = 1 [(validate.rules).string.max_len = 128];
        uint32 machine_group_count = 2;
        uint32 service_count = 3;
        bool compiles = 4;     // last known Check result (empty diagnostics)
    }
}
message RecipeBundle {
    // files maps slash-path -> file bytes; same keys internal/dsl include.Sources uses
    // (cluster.yaml, workflow.yaml, components/**, providers/**).
    map<string, bytes> files = 1 [(validate.rules).map = {min_pairs: 1, max_pairs: 512}];
}
```

```proto
// api/recipe.proto — RecipeService: CRUD + Compile/Check over a stored bundle.
// Mirror api/suite.proto structure (tenant_id on every request, ogen file option,
// (cloud.v1.iam.auth) per method: CRUD gated RESOURCE_RECIPE, Compile/Check ACTION_READ).
service RecipeService {
    rpc CreateRecipe(CreateRecipeRequest) returns (CreateRecipeResponse);   // auth: {all_of:[{resource:RESOURCE_RECIPE, action:ACTION_CREATE}]}
    rpc GetRecipe(GetRecipeRequest) returns (GetRecipeResponse);            // ACTION_READ
    rpc ListRecipes(ListRecipesRequest) returns (ListRecipesResponse);     // ACTION_LIST
    rpc DeleteRecipe(DeleteRecipeRequest) returns (DeleteRecipeResponse);  // ACTION_DELETE
    // CheckRecipe compiles the stored bundle in check-mode; diagnostics, never RPC error for user input.
    rpc CheckRecipe(CheckRecipeRequest) returns (CheckRecipeResponse);     // ACTION_READ, returns repeated dsl.Diagnostic
}
// Requests carry tenant_id + (id | RecipeRecord). Reuse cloud.v1.dsl.Diagnostic in CheckRecipeResponse
// (import cloud/v1/dsl/service.proto).
```

- RESOURCE_RECIPE: add to the resource enum in `iam/permission.proto` next to RESOURCE_SUITE (pick the next free enum number — read the file, do not reuse an existing number).

- [ ] **Step 1: Study patterns** — read `protocols/cloud/v1/models/suite.proto`, `protocols/cloud/v1/api/suite.proto` (Create/Get/List/Delete request+response shapes, ogen option, auth options), `protocols/cloud/v1/iam/permission.proto` (resource enum), `protocols/cloud/v1/dsl/service.proto` (Diagnostic message to reuse).

- [ ] **Step 2: Write the three proto edits** exactly per Interfaces. RecipeService: skip graphql+ogen like DslService did? NO — recipe CRUD SHOULD have REST/GraphQL like suite (it's a first-class resource). Mirror suite.proto's generator surface (ogen file option present, no graphqlopt.skip). If a generator errors, wire config the way suite is wired.

- [ ] **Step 3: `make protocols`** — Expected: `internal/proto/cloud/v1/models/recipe.pb.go` + `api/recipe*.pb.go` (+ connect/jx/validate/ogen/graphql per suite's surface) generate clean; `go build ./...` green.

- [ ] **Step 4: Revert unrelated TS drift** — `git status`: stage ONLY new recipe proto + generated recipe artifacts + permission.proto changes + intended TS. Revert unrelated `web/src/lib/proto/**` regen drift.

- [ ] **Step 5: Commit**

```bash
git add protocols/cloud/v1/models/recipe.proto protocols/cloud/v1/api/recipe.proto protocols/cloud/v1/iam/permission.proto internal/proto/cloud/v1/ web/src/lib/proto/cloud/v1/models/recipe_pb.ts web/src/lib/proto/cloud/v1/api/recipe_pb.ts docs/
git commit -m "feat(proto): add RecipeRecord model and RecipeService api"
```

---

### Task 2: DB schema + queries + codegen

**Files:**
- Modify: `internal/infrastructure/postgres/schema.sql` (add `recipe_records` table)
- Create: `internal/infrastructure/postgres/queries/recipe.sql`
- Generated: `internal/infrastructure/postgres/gen/{db,bob}/*` via `make db-gen`
- Migration: `internal/infrastructure/postgres/migrations/*_add_recipe_records.sql` via `make migrate-generate`

**Interfaces:**
- Produces: `dbgen.{CreateRecipeRecord,GetRecipeRecord,ListRecipeRecords,GetLatestRecipeRecordByName,DeleteRecipeRecord}` query funcs + params structs.

- [ ] **Step 1: Add table to schema.sql** (mirror `suite_records` at schema.sql:53, add name/version columns for the unique constraint + latest lookup):

```sql
CREATE TABLE recipe_records (
  id         text PRIMARY KEY,
  tenant_id  text NOT NULL,
  name       text NOT NULL,
  version    integer NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE INDEX idx_recipe_records_tenant ON recipe_records (tenant_id);
CREATE UNIQUE INDEX uq_recipe_records_tenant_name_version ON recipe_records (tenant_id, name, version);
```

- [ ] **Step 2: Write queries/recipe.sql** (mirror the suite block in queries/records.sql; add latest-by-name):

```sql
-- name: CreateRecipeRecord :exec
insert into recipe_records (id, tenant_id, name, version, created_at, updated_at, data)
values (@id, @tenant_id, @name, @version, now(), now(), @data);

-- name: GetRecipeRecord :one
select data from recipe_records where tenant_id = @tenant_id and id = @id;

-- name: ListRecipeRecords :many
select data from recipe_records where tenant_id = @tenant_id order by name, version;

-- name: GetLatestRecipeRecordByName :one
select data from recipe_records where tenant_id = @tenant_id and name = @name
order by version desc limit 1;

-- name: DeleteRecipeRecord :execrows
delete from recipe_records where tenant_id = @tenant_id and id = @id;
```

- [ ] **Step 3: `make db-gen`** — Expected: `gen/db` + `gen/bob` regenerate with the new query funcs; `go build ./internal/infrastructure/postgres/...` green.

- [ ] **Step 4: `make migrate-generate name=add_recipe_records`** — Expected: new incremental migration file with the CREATE TABLE. Verify it embeds (migrations.go `//go:embed *.sql`).

- [ ] **Step 5: Commit** `feat(postgres): add recipe_records table and queries`

---

### Task 3: RecipeRepo + port interface

**Files:**
- Create: `internal/infrastructure/postgres/recipe.go` (RecipeRepo)
- Modify: `internal/infrastructure/postgres/store.go` (wire `Recipes()` accessor)
- Test: `internal/infrastructure/postgres/recipe_test.go`
- The port interface lives in the service package (Task 4) — this task's repo must satisfy it.

**Interfaces:**
- Consumes: `dbgen` funcs (Task 2), `models.RecipeRecord` (Task 1).
- Produces:

```go
type RecipeRepo struct{ db *DB }
func (r *RecipeRepo) Create(ctx context.Context, rec *models.RecipeRecord) error   // derrors.Conflict on dup
func (r *RecipeRepo) Get(ctx context.Context, tenantID, id string) (*models.RecipeRecord, error) // derrors.NotFound
func (r *RecipeRepo) List(ctx context.Context, tenantID string) ([]*models.RecipeRecord, error)
func (r *RecipeRepo) GetLatestByName(ctx context.Context, tenantID, name string) (*models.RecipeRecord, error)
func (r *RecipeRepo) Delete(ctx context.Context, tenantID, id string) error        // derrors.NotFound
```

- [ ] **Step 1: Write the failing test** (mirror an existing repo test — find one under `internal/infrastructure/postgres/*_test.go`; use the same tx/pool test harness). Test Create→Get roundtrip, List, GetLatestByName picks max version, Delete→NotFound, Create dup→Conflict, tenant isolation (other tenant can't Get).

```go
func TestRecipeRepoRoundtrip(t *testing.T) {
	// harness: acquire test DB per existing pattern
	rec := &models.RecipeRecord{Entity: &common.Entity{Id: "r1", TenantId: "t1", Name: "pg-ha"}, Version: 1, Bundle: &models.RecipeBundle{Files: map[string][]byte{"cluster.yaml": []byte("version: 1")}}}
	// Create, Get, assert bundle files roundtrip; List len 1; GetLatestByName("t1","pg-ha").Version==1
	// insert version 2, GetLatestByName → version 2; Get with tenant "t2" → NotFound
}
```

- [ ] **Step 2: Run — FAIL** (`go test ./internal/infrastructure/postgres/ -run Recipe -v`)
- [ ] **Step 3: Implement RecipeRepo** (mirror SuiteRepo exactly — marshal/unmarshal helpers, translatePgErr, isUniqueViolation). Wire `(s *Store).Recipes() *RecipeRepo` in store.go.
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(postgres): add RecipeRepo`

---

### Task 4: RecipeService (CRUD + Check delegation)

**Files:**
- Create: `internal/services/recipe/service.go` (Deps + ports), `internal/services/recipe/recipe.go` (handlers)
- Test: `internal/services/recipe/recipe_test.go`

**Interfaces:**
- Consumes: `apipb.RecipeService*` request/response, `RecipeRepo` (via port), existing `internal/services/dsl` check logic, `internal/dsl.Compile`.
- Produces:

```go
type RecipeRepo interface { // the port RecipeRepo (Task 3) satisfies
	Create(ctx context.Context, rec *models.RecipeRecord) error
	Get(ctx context.Context, tenantID, id string) (*models.RecipeRecord, error)
	List(ctx context.Context, tenantID string) ([]*models.RecipeRecord, error)
	Delete(ctx context.Context, tenantID, id string) error
}
type Deps struct {
	Repo  RecipeRepo
	Authn utils.Authn
	// Checker runs dsl check over a bundle → diagnostics (reuse internal/services/dsl logic; inject as a small func/iface).
	Checker func(ctx context.Context, files map[string][]byte) ([]*dslpb.Diagnostic, error)
}
type Service struct{ d Deps }
func NewService(d Deps) *Service
// handlers: CreateRecipe (mint id, stamp tenant, version=next, compute Summary via Checker+quick parse, persist),
// GetRecipe, ListRecipes, DeleteRecipe, CheckRecipe (load bundle by id or use inline, run Checker).
```

- Summary computation: on Create, run Checker; set `Summary.compiles = len(diags)==0`; derive provider/counts by a light parse of cluster.yaml (reuse ast.DecodeCluster — tolerate errors, leave zero).
- Auth: handlers resolve `s.d.Authn.Caller(ctx)`, use `req.GetTenantId()`; RBAC enforced by interceptor (don't re-check), but validate tenant present + ownership stamping like suite handlers.

- [ ] **Step 1: Write failing tests** — fake RecipeRepo (in-memory map) + stub Checker. Test: CreateRecipe persists with version 1 + stamped tenant/id + Summary.compiles reflects stub; GetRecipe returns it; ListRecipes; DeleteRecipe→then Get NotFound; CheckRecipe returns stub diagnostics; missing tenant_id → InvalidArgument.
- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Implement handlers** (mirror `internal/services/suite/suite.go` handler shape + `utils.MapErr` for error codes).
- [ ] **Step 4: Run — PASS** (`go test ./internal/services/recipe/ -v`)
- [ ] **Step 5: Commit** `feat(services): add RecipeService CRUD and check`

---

### Task 5: App wiring + registration

**Files:**
- Modify: `internal/app/run.go` (construct RecipeService in section 6, register in section 7 near dslService ~line 609)
- Test: extend `internal/services/recipe/recipe_test.go` or add a light wiring smoke if an app-level test harness exists (else skip — Task 4 covers logic).

**Interfaces:**
- Consumes: `recipe.NewService`, `store.Recipes()`, the dsl check func, `authzGate`.

- [ ] **Step 1: Construct + register** — in run.go section 6 build the Checker closure over the existing dsl service/`internal/dsl` logic (reuse `internal/services/dsl`'s check entrypoint — call its exported check func, or factor a shared `dslcheck.Check(files)` if not exported; do NOT duplicate the traversal-safe derivation). Build `recipeService := recipe.NewService(recipe.Deps{Repo: store.Recipes(), Authn: iam.ClaimsAuthn{}, Checker: dslCheck})`. Add a `register(mux, ...)` entry `apiconnect.NewRecipeServiceHandler(recipeService, handlerOpts...)` near line 609. If GraphQL/REST surfaces were generated (Task 1), add to `gqlapi.Server` + `ogenAdapter` like suite.
- [ ] **Step 2: Build + existing tests** — `go build ./...` green; `go test ./internal/... ` green (focus services/recipe, app).
- [ ] **Step 3: Commit** `feat(app): wire and register RecipeService`

---

## Self-Review
- Spec §4.1 coverage: table+repo (T2/T3), proto record+service (T1), CRUD+Check delegation (T4), registration (T5). Versioning = explicit `version` field + `GetLatestByName` (open question resolved: explicit field).
- Deferred to later phases: run-from-recipe launch (1D), recipe→CompiledPlan at run time (1D uses Compile), UI (subproject 2).
- No placeholders: SQL/proto/Go shapes are concrete; tests name concrete assertions.
- Types consistent: `RecipeRecord`/`RecipeBundle`/`RecipeRepo` names identical across tasks; repo satisfies the service port.
