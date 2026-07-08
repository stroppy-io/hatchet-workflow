# SP-E: Run-модель rewrite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `models.TestRunRecord` with a new first-class `models.Run` proto message whose fields map 1:1 to what `RunRecipeWorkflow` (the only live run path) actually produces — `Baked`/`CompiledPlan` identity (SP-A), `RunTopology`, `workflow.RunState`, `ObservabilityRefs`, denormalized `Summary` — and migrate every reader (26+ functions in `OverviewReader`, the runs-table repo, every `RunDetail` tab, Compare, Rating, tenant-dashboard) onto it without losing any field that reaches the screen today, per the spec's data-parity chart (§6).

**Architecture:** Additive-then-cutover. `models.Run` / `run_records` / `RunRepo` are built alongside the existing `TestRunRecord` / `test_run_records` / `TestRunRepo` (Task 1–2, zero behavior change to the live system). The write path (`RunRecipeWorkflow`, `StartRun`) is switched to mint and persist `Run` (Task 3) — this is the cutover point; after Task 3, `run_records` is what's live and `test_run_records` stops receiving new rows. Every reader is then migrated in dependency order: durable-projection helpers (Task 4: `OverviewReader` + `runtime_topology.go`), service/adapter layer + API proto renames (Task 5), web (Task 6). Task 7 deletes `TestRunRecord`/`test_run_records`/`RecipeTopologySnapshot` and the now-dead classic-`TestWorkflow` code paths they existed to serve. Task 8 is a cross-cutting data-parity guard suite keyed line-by-line to spec §6.

**Tech Stack:** Go, connect-rpc, Temporal (`go.temporal.io/sdk`), Postgres via the komeet `sqld` toolchain (`make db-gen`, jsonb-blob-per-row storage), proto via `easyp` (`make protocols`), React/TypeScript (`@bufbuild/protobuf` generated clients) for `web/`.

## Global Constraints

- **No data migration.** `run_records` starts empty; `test_run_records` is dropped in Task 7 without a backfill (spec §1/§5, greenfield).
- **Additive until cutover.** Tasks 1–2 must NOT modify `TestRunRecord`, `RecipeTopologySnapshot`, `test_run_records`, or any existing reader — they only add new code. This keeps the live system green (`make protocols`/`make db-gen`/`go build ./...` pass) at every commit before Task 3 flips the write path.
- **Intentional temporary duplication.** Task 2's `run_list.go` is a near-duplicate of `internal/infrastructure/postgres/test_run_list.go` (same WHERE/ORDER-BY builder logic, different table + strip-columns). This is deliberate, not an oversight — Task 7 deletes `test_run_list.go` wholesale, leaving `run_list.go` as the only copy. Do not attempt to unify them mid-migration; that would touch code Task 7 is about to delete anyway.
- **Classic-`TestWorkflow` branches are dead code for `models.Run`.** `domain.TestRun`-only fields (`Spec`, `InfrastructureState`, `DeploymentPlan`) exist on `TestRunRecord` for a workflow (`domainTestWorkflow`) that is no longer registered (spec §1). `models.Run` never carries these fields. Where a migrated function's only remaining branch after dropping the classic path is the recipe/`RecipeTopology`-shaped one, DROP the classic branch — do not port dead code forward (see Task 4).
- **Resolves spec §9 open question 1:** `api/test_run.proto`'s `ListTestRunsRequest` remains the single facet/sort/paging source of truth (renamed `ListRunsRequest` in Task 5); `api/recipe.proto`'s `ListRunsRequest` stays a thin `recipe_id`→`workflow_id` wrapper over it, unchanged in shape. No proto message unification — see Task 5.
- **Resolves spec §9 open question 2:** `workflow_version` = `strconv.FormatUint(RecipeRecord.version, 10)` (same source `recipe.go`'s `StartRun` already formats today as `"v" + ...`), no SP-B dependency.
- **Resolves spec §9 open question 3:** `ObservabilityRefs` fields are populated at mint time in Task 3 (`= entity.id`, `= entity.id`, `= <relay's existing hardcoded uid>`) — cheap, documents the contract, matches spec's stated preference.
- **`reserved 14 to 20`** is carried on `models.Run` per spec §4 verbatim — do not consume this range for anything in this plan (spec §9 open question 4 reserves it for a future `batch_id`, out of scope here).
- No co-author footer in commits. Conventional Commits (`type(scope): summary`).
- Every task ends with the relevant `go build ./...` / `go test ./...` (or `cd web && npm run build`/`typecheck`) green before commit.
- Proto changes go through `make protocols` (regenerates `internal/proto/**` and `web/src/lib/proto/**` — both are committed generated code in this repo, do not hand-edit `.pb.go`/`_pb.ts`).
- Postgres changes go through `make db-gen` (regenerates `internal/infrastructure/postgres/gen/**`) and `make migrate-generate name=<x>` (schema-diff migration).

---

### Task 1: `models.Run` proto message + codegen

Adds `Run`, `Run.Summary`, `RunTopology`, `ObservabilityRefs` to `protocols/cloud/v1/models/test_run.proto`, alongside the untouched `TestRunRecord`/`RecipeTopologySnapshot`. Structurally identical `Summary` (spec §4: same 15 fields as `TestRunRecord.Summary`) so Task 2's query-builder porting is a mechanical table/type rename.

**Files:**
- Modify: `protocols/cloud/v1/models/test_run.proto` (append; `TestRunRecord`/`RecipeTopologySnapshot` untouched)
- Generated (via `make protocols`, committed): `internal/proto/cloud/v1/models/test_run.pb.go`, `.pb.jx.go`, `.pb.validate.go`; `web/src/lib/proto/cloud/v1/models/test_run_pb.ts`
- Test: `internal/proto/cloud/v1/models/run_test.go` (new)

**Interfaces:**
- Produces: `models.Run`, `models.Run_Summary` (Go: `*models.Run`, `*models.Run_Summary`), `models.RunTopology`, `models.RunTopology_MachineNode`, `models.RunTopology_MachineNode_ServiceNode`, `models.ObservabilityRefs`.

- [ ] **Step 1: Write the proto message**

Append to `protocols/cloud/v1/models/test_run.proto` (imports for `schemapb`/`dsl.CompiledPlan` need adding — check `protocols/cloud/v1/dsl/*.proto`'s go_package and `schemapb`'s buf dependency, both already used elsewhere in this repo per SP-A/SP-D recon):

```proto
import "cloud/v1/dsl/plan.proto";       // dsl.CompiledPlan (adjust path to the actual compiled-plan proto file — verify against internal/dsl's existing dslpb import path, e.g. protocols/cloud/v1/dsl/compiled_plan.proto)
import "schemapb/schema.proto";         // schemapb.Baked (verify against SP-A's actual buf module path for schemapb.Baked)

/*
    Run is a persisted recipe run (RunRecipeWorkflow's only live path — see
    package doc). Replaces TestRunRecord (above): every field here is
    something RunRecipeWorkflow actually produces (compile -> CompiledPlan +
    Baked identity; provision -> topology; execute -> per-job status via
    workflow.RunState; always -> observability refs), unlike TestRunRecord
    whose spec/infrastructure_state/deployment_plan fields a recipe run
    always leaves empty (see TestRunRecord's doc and StartRun's own comment).
 */
message Run {
    common.Entity entity = 1 [(validate.rules).message.required = true];
    common.Status status = 2;
    common.Trigger trigger = 3;

    // workflow_id is the originating models.RecipeRecord.entity.id (ex
    // recipe_id) that StartRun launched this run from. Stamped once, never
    // changed. Reserves the name for SP-B's catalog Workflow — until then,
    // its value is exactly what recipe_id carries on TestRunRecord today.
    string workflow_id = 4 [(validate.rules).string.max_len = 64];
    // workflow_version identifies the bundle version at launch time:
    // strconv.FormatUint(RecipeRecord.version, 10) (see recipe.go StartRun).
    string workflow_version = 5 [(validate.rules).string.max_len = 32];
    // baked is the sealed launch-form snapshot (SP-A's schemapb.Bake
    // output). nil until SP-D's generated-form launch path exists.
    schemapb.Baked baked = 6;
    // compiled_plan is the compiled DSL plan RunRecipeWorkflow executed.
    // NEW relative to TestRunRecord: today the plan lives only in workflow
    // memory and is never persisted; see persistRunCompiledPlan (Task 3).
    dsl.CompiledPlan compiled_plan = 7;

    // topology is the provisioned-machine snapshot (renamed
    // RecipeTopologySnapshot -> RunTopology, see below).
    RunTopology topology = 8;

    // runtime_state is the last workflow.RunState RunRecipeWorkflow
    // persisted (unchanged type/semantics from TestRunRecord.runtime_state).
    workflow.RunState runtime_state = 9;

    ObservabilityRefs observability = 10;

    bool in_tenant_rating = 11;
    bool in_global_rating = 12;

    // summary: same 15 fields as TestRunRecord.Summary (denormalized,
    // queryable facets for the runs table). See Run.Summary below.
    Summary summary = 13;

    reserved 14 to 20; // headroom, mirrors TestRunRecord's own reserved-field discipline.

    message Summary {
        domain.Database.Kind db_kind = 1;
        string db_preset_id = 2 [(validate.rules).string.max_len = 64];
        string db_preset_name = 3 [(validate.rules).string.max_len = 255];
        string workload_preset_id = 4 [(validate.rules).string.max_len = 64];
        string workload_name = 5 [(validate.rules).string.max_len = 255];
        string stroppy_version = 6 [(validate.rules).string.max_len = 64];
        domain.Workload.Protocol workload_protocol = 14;
        string test_preset_id = 15 [(validate.rules).string.max_len = 64];
        string test_preset_name = 16 [(validate.rules).string.max_len = 255];
        string topology_label = 7 [(validate.rules).string.max_len = 128];
        uint32 node_count = 8;
        deployment.Provider provider = 9;
        uint32 progress_pct = 10 [(validate.rules).uint32.lte = 100];
        google.protobuf.Timestamp started_at = 11;
        google.protobuf.Timestamp finished_at = 12;
        google.protobuf.Duration duration = 13;
    }
}

/*
    RunTopology replaces RecipeTopologySnapshot (see that message's doc,
    above) — same shape, generalized name: every live run is now a
    recipe/workflow run, so the "Recipe" prefix no longer distinguishes
    anything.
 */
message RunTopology {
    message ServiceNode {
        string name = 1;
        string image = 2;
    }
    message MachineNode {
        string node_id = 1;
        string group = 2;
        string ip = 3;
        common.Status status = 4;
        repeated ServiceNode services = 5;
        map<string, string> labels = 6;
    }
    string provider = 1;
    repeated MachineNode nodes = 2;
}

/*
    ObservabilityRefs makes the "runtime observations keyed by run id"
    convention (see logs.proto/metrics.proto doc comments) an explicit
    contract on Run instead of an implicit one on TestRunRecord. v1: every
    field is filled with entity.id (metrics/logs) or the relay's existing
    hardcoded uid (grafana) at mint time (Task 3) — SP-F gives these real
    per-provider variance.
 */
message ObservabilityRefs {
    string metrics_query_key = 1;
    string logs_query_key = 2;
    string grafana_dashboard_uid = 3;
}
```

> Before writing this, grep `protocols/cloud/v1/dsl/*.proto` for the actual file defining `CompiledPlan` (referenced as `dslpb.CompiledPlan` in Go — `internal/workflows/runrecipe_summary.go`'s `deriveRunSummary(plan *dslpb.CompiledPlan)` confirms the Go import path `github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl`; find the matching `.proto` source file and import path) and confirm `schemapb.Baked`'s buf module path against SP-A's actual implementation (SP-A plan Task 4/5 use `github.com/stroppy-io/schemapb v1.4.4` as a Go module — check whether SP-A vendors a `.proto` for it under `protocols/` or whether `schemapb.Baked` is Go-only with no proto wire type; if Go-only, `Run.baked` cannot be a proto field as written above and must instead be a `bytes baked_blob = 6` carrying `schemapb`'s own serialization, OR wait until SP-D lands before wiring the proto field — resolve this against the actual SP-A code before Step 1, not against this plan's assumption).

- [ ] **Step 2: Generate**

```bash
make protocols
```

Expected: `internal/proto/cloud/v1/models/test_run.pb.go` (+`.pb.jx.go`/`.pb.validate.go`) gains `Run`/`RunTopology`/`ObservabilityRefs` types; `web/src/lib/proto/cloud/v1/models/test_run_pb.ts` gains matching TS types. `TestRunRecord`/`RecipeTopologySnapshot` byte-for-byte unchanged (diff the generated files to confirm).

- [ ] **Step 3: Write the smoke test**

```go
package models_test

import (
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

func TestRun_ProtojsonRoundTrip(t *testing.T) {
	run := &models.Run{
		Entity:     &commonpb.Entity{Id: "run-1", TenantId: "tenant-1"},
		Status:     commonpb.Status_STATUS_RUNNING,
		WorkflowId: "recipe-1",
		Topology: &models.RunTopology{
			Provider: "docker",
			Nodes: []*models.RunTopology_MachineNode{
				{NodeId: "db-0", Group: "db", Status: commonpb.Status_STATUS_COMPLETED},
			},
		},
		Observability: &models.ObservabilityRefs{MetricsQueryKey: "run-1", LogsQueryKey: "run-1"},
		Summary:       &models.Run_Summary{DbKind: 0, TopologyLabel: "PG x1"},
	}

	data, err := protojson.Marshal(run)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	got := &models.Run{}
	if err := protojson.Unmarshal(data, got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.GetTopology().GetNodes()[0].GetNodeId() != "db-0" {
		t.Fatalf("round-trip lost topology.nodes[0].node_id")
	}
	if got.GetWorkflowId() != "recipe-1" {
		t.Fatalf("round-trip lost workflow_id")
	}
}
```

- [ ] **Step 4: Run to verify pass**

```
go test ./internal/proto/cloud/v1/models/... -run TestRun_ProtojsonRoundTrip -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add protocols/cloud/v1/models/test_run.proto internal/proto/cloud/v1/models/ web/src/lib/proto/cloud/v1/models/ internal/proto/cloud/v1/models/run_test.go
git commit -m "feat(models): add Run proto message alongside TestRunRecord"
```

---

### Task 2: Postgres `run_records` table + `RunRepo`

Mirrors the exact komeet jsonb-blob pattern `test_run_records`/`TestRunRepo` use (spec §3.2/§5). Additive: `test_run_records` and `TestRunRepo` are untouched.

**Files:**
- Modify: `internal/infrastructure/postgres/schema.sql` (append table)
- Modify: `internal/infrastructure/postgres/queries/records.sql` (append sqld query annotations)
- Generated (via `make db-gen`): `internal/infrastructure/postgres/gen/db/*`, `gen/bob/models/run_records.bob.go`
- New migration (via `make migrate-generate`): `internal/infrastructure/postgres/migrations/<n>_add_run_records.sql`
- Create: `internal/infrastructure/postgres/run_store.go` (Create/Get/Update/Delete/Exists — mirrors `store.go`'s `TestRunRepo`)
- Create: `internal/infrastructure/postgres/run_list.go` (List — near-duplicate of `test_run_list.go`, see Global Constraints)
- Test: `internal/infrastructure/postgres/run_list_test.go`

**Interfaces:**
- Produces:
  ```go
  type RunRepo struct{ db *DB }
  func (s *Store) Runs() *RunRepo
  func (r *RunRepo) Create(ctx context.Context, run *models.Run) error
  func (r *RunRepo) Get(ctx context.Context, tenantID, id string) (*models.Run, error)
  func (r *RunRepo) Update(ctx context.Context, run *models.Run) error
  func (r *RunRepo) Delete(ctx context.Context, tenantID, id string) error
  func (r *RunRepo) Exists(ctx context.Context, tenantID, runID string) error
  func (r *RunRepo) List(ctx context.Context, query *api.ListTestRunsRequest, callerAccountID string) ([]*models.Run, string, error)
  ```

- [ ] **Step 1: Schema + queries**

`schema.sql` (append after `test_run_records`'s block):

```sql
CREATE TABLE run_records (
  id         text PRIMARY KEY,
  tenant_id  text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  data       jsonb NOT NULL
);
CREATE INDEX idx_run_records_tenant ON run_records (tenant_id);
```

`queries/records.sql` (append, mirroring the `TestRunRecord` block at lines 3–21 exactly, renamed):

```sql
-- name: CreateRunRecord :exec
insert into run_records (id, tenant_id, created_at, updated_at, data)
values (@id, @tenant_id, now(), now(), @data);

-- name: GetRunRecord :one
select data from run_records where tenant_id = @tenant_id and id = @id;

-- name: ListRunRecords :many
select data from run_records where tenant_id = @tenant_id;

-- name: UpdateRunRecord :execrows
update run_records set data = @data, updated_at = now()
where tenant_id = @tenant_id and id = @id;

-- name: DeleteRunRecord :execrows
delete from run_records where tenant_id = @tenant_id and id = @id;

-- name: ExistsRunRecord :one
select 1 from run_records where tenant_id = @tenant_id and id = @id;
```

- [ ] **Step 2: Generate + migration**

```bash
make db-gen
make migrate-generate name=add_run_records
```

Expected: `gen/db` gains `CreateRunRecord`/`GetRunRecord`/... typed funcs + `CreateRunRecordParams` etc; a new migration file appears creating `run_records` + its index; `TestRunRecord`'s generated code unchanged.

- [ ] **Step 3: `RunRepo` CRUD (mirrors `store.go`'s `TestRunRepo`, lines 36–112)**

```go
package postgres

import (
	"context"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	dbgen "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

// Runs returns the RunRepo (the runs-table repo + overview run reader for
// models.Run — replaces TestRuns()/TestRunRepo for the live RunRecipeWorkflow
// path; see run_store.go's package doc).
func (s *Store) Runs() *RunRepo { return &RunRepo{db: s.db} }

type RunRepo struct{ db *DB }

func (r *RunRepo) Create(ctx context.Context, run *models.Run) error {
	data, err := marshal(run)
	if err != nil {
		return err
	}
	err = r.db.q().CreateRunRecord(ctx, dbgen.CreateRunRecordParams{
		ID:       run.GetEntity().GetId(),
		TenantID: run.GetEntity().GetTenantId(),
		Data:     data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("run", "run already exists")
		}
		return err
	}
	return nil
}

func (r *RunRepo) Get(ctx context.Context, tenantID, id string) (*models.Run, error) {
	row, err := r.db.q().GetRunRecord(ctx, dbgen.GetRunRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return nil, translatePgErr("run", err)
	}
	rec := &models.Run{}
	if err := unmarshal(row.Data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

func (r *RunRepo) Update(ctx context.Context, run *models.Run) error {
	data, err := marshal(run)
	if err != nil {
		return err
	}
	n, err := r.db.q().UpdateRunRecord(ctx, dbgen.UpdateRunRecordParams{
		Data:     data,
		TenantID: run.GetEntity().GetTenantId(),
		ID:       run.GetEntity().GetId(),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("run", "run not found")
	}
	return nil
}

func (r *RunRepo) Delete(ctx context.Context, tenantID, id string) error {
	n, err := r.db.q().DeleteRunRecord(ctx, dbgen.DeleteRunRecordParams{TenantID: tenantID, ID: id})
	if err != nil {
		return err
	}
	if n == 0 {
		return derrors.NotFound("run", "run not found")
	}
	return nil
}

func (r *RunRepo) Exists(ctx context.Context, tenantID, runID string) error {
	_, err := r.db.q().ExistsRunRecord(ctx, dbgen.ExistsRunRecordParams{TenantID: tenantID, ID: runID})
	if err != nil {
		return translatePgErr("run", err)
	}
	return nil
}
```

- [ ] **Step 4: `RunRepo.List` — port the query builder**

Copy `test_run_list.go` to `run_list.go` (package-private identifiers renamed to avoid collision: `argBuilder` is already shared/exported-in-package so it can stay; rename `jDBKind` etc. only if the const block would collide — since both files are in `package postgres`, EITHER share the `j*` consts (they're identical jsonb path expressions, both point at `data->'summary'->>'...'` which is the same shape on both `Run.summary` and `TestRunRecord.summary`) OR prefix new ones `jRun*`. Prefer reusing the existing `j*` consts from `test_run_list.go` unchanged — do not duplicate them — since the jsonb path shape is identical; only declare NEW consts for anything genuinely different).

Differences from `test_run_list.go`'s `List` (line 77 signature) to port into `run_list.go`:
1. Signature: `func (r *RunRepo) List(ctx context.Context, query *api.ListTestRunsRequest, callerAccountID string) ([]*models.Run, string, error)`.
2. Table name: `run_records` instead of `test_run_records` (the `FROM`/favorite-`EXISTS`-subquery/`FavoriteKind` target).
3. Strip-columns on the `SELECT` (line 239): `Run`'s heavy blobs are `runtimeState` and `compiledPlan` (both potentially large — `runtimeState` per the existing "~1.25MB" comment, `compiledPlan` is new and equally heavy since it embeds the full rendered plan) — strip both: `SELECT data - 'runtimeState' - 'compiledPlan' FROM run_records`. `Run` has no `deploymentPlan`/`infrastructureState` fields to strip (those don't exist on `Run`).
4. `jSuiteRunID`/`jSuiteCellID` consts and the `suite membership` WHERE-clause block (lines 173–186) are DROPPED — `Run` has no `suite_run_id`/`suite_cell_id` (spec §5/§6.A: dead field, dropped). `query.GetSuiteRunId()`/`GetSuiteCellIds()`/`GetStandalone()` become no-ops in `RunRepo.List` (silently ignored, since `ListTestRunsRequest` keeps the fields for `TestRunRepo.List`'s sake until Task 7 — do not remove them from the request proto in this task).
5. Row unmarshal target: `rec := &models.Run{}`.
6. `favoriteIDs` call: `commonpb.FavoriteKind_FAVORITE_KIND_TEST_RUN` — confirm whether a distinct `FAVORITE_KIND_RUN` should exist or whether `Run` favorites continue to share the `TEST_RUN` kind (favorites are keyed by `(kind, target_id)`; since a `Run.entity.id` and a `TestRunRecord.entity.id` are drawn from the same ID space and never coexist per row post-cutover, reusing `FAVORITE_KIND_TEST_RUN` is safe and avoids a proto enum change — use it as-is).

- [ ] **Step 5: Query-builder unit test (mirrors `test_run_list_test.go`)**

```go
package postgres

import (
	"strings"
	"testing"
)

func TestRunListStripsHeavyColumnsFromRunRecords(t *testing.T) {
	// Guards the SELECT projection: run_records (not test_run_records), and
	// strips runtimeState + compiledPlan (Run's heavy blobs), not
	// deploymentPlan/infrastructureState (TestRunRecord's, which Run doesn't have).
	sb := runListBaseSelect() // extract the literal SELECT string from run_list.go's List into a small helper for testability
	if !strings.Contains(sb, "FROM run_records") {
		t.Fatalf("select %q does not target run_records", sb)
	}
	if !strings.Contains(sb, "- 'runtimeState'") || !strings.Contains(sb, "- 'compiledPlan'") {
		t.Fatalf("select %q does not strip runtimeState/compiledPlan", sb)
	}
	if strings.Contains(sb, "deploymentPlan") || strings.Contains(sb, "infrastructureState") {
		t.Fatalf("select %q references TestRunRecord-only columns", sb)
	}
}

func TestRunListOrderByDropsSuiteAndKeepsSummaryFacets(t *testing.T) {
	// buildOrderBy is shared with test_run_list.go unchanged (Kind switch is
	// identical); this test only guards that RunRepo.List's WHERE-builder no
	// longer emits a jSuiteRunID/jSuiteCellID clause.
	b := &argBuilder{}
	where := runListWhereClauses(b, &api.ListTestRunsRequest{
		TenantId:    "t1",
		SuiteRunId:  "suite-1", // must be ignored
		Standalone:  proto.Bool(true), // must be ignored
	})
	for _, w := range where {
		if strings.Contains(w, "suiteRunId") || strings.Contains(w, "suiteCellId") {
			t.Fatalf("RunRepo.List WHERE clause %q still references dropped suite fields", w)
		}
	}
}
```

> Extract `runListBaseSelect()`/`runListWhereClauses(b, query)` as small internal helpers factored out of `RunRepo.List`'s body (same technique `test_run_list.go` already uses for `buildOrderBy`/`testRunSearchClause` as independently-testable pieces) — do not test through a live DB connection; this package has no DB-integration test harness today (confirmed: no `testcontainers`/`dockertest`/`TestMain` in `internal/infrastructure/postgres`), full CRUD correctness is exercised by the e2e docker recipe-run (Task 8's E2E note).

- [ ] **Step 6: Run to verify pass**

```
go test ./internal/infrastructure/postgres/... -run TestRunList -v
go build ./...
```

- [ ] **Step 7: Commit**

```bash
git add internal/infrastructure/postgres/schema.sql internal/infrastructure/postgres/queries/records.sql internal/infrastructure/postgres/gen/ internal/infrastructure/postgres/migrations/ internal/infrastructure/postgres/run_store.go internal/infrastructure/postgres/run_list.go internal/infrastructure/postgres/run_list_test.go
git commit -m "feat(postgres): add run_records table and RunRepo alongside TestRunRepo"
```

---

### Task 3: `RunRecipeWorkflow` write-path on `models.Run`

The cutover task: after this task, every NEW run is a `models.Run` row in `run_records`; `TestRunRepo`/`test_run_records` receive no further writes (though they remain readable until Task 7, since Tasks 4–6 haven't migrated yet — see the sequencing note below).

**Sequencing note:** Tasks 4–6 (readers) are NOT done yet when this task lands. `OverviewReader`/`compare`/`rating`/web still expect `*models.TestRunRecord`. This task must therefore ALSO stub a minimal adapter so the live system keeps building: `RunPersistenceStore`'s `RunRecord(ctx, runID) (*models.TestRunRecord, error)` (consumed by `OverviewReader`, `overview.go` line 57) needs a `*models.Run`→`*models.TestRunRecord` shim for the duration of Tasks 3–5 (a small lossy adapter in `internal/infrastructure/execution/run_shim.go` populating just `Entity`/`Status`/`Summary`/`RuntimeState` — enough that `overview.go`'s existing, unmigrated functions don't panic on a nil record — degrading but not crashing Topology/Agents tabs for the gap between Task 3 and Task 4 landing). If Tasks 3 and 4 are executed back-to-back in the same PR/branch (recommended), this shim can be skipped entirely — go straight from Task 3 to Task 4 without an intermediate deploy. Note this choice explicitly in the PR description if splitting the tasks across deploys.

**Files:**
- Modify: `internal/workflows/register.go` (`RuntimeActivities` interface, activity name consts, `RegisterActivities`)
- Modify: `internal/workflows/runtime.go` (`persist`, `persistRunSummary`, `persistRecipeTopology`, new `persistRunCompiledPlan`)
- Modify: `internal/workflows/runrecipe.go` (`RunRecipeWorkflow.run` — add `persistRunCompiledPlan` call after compile stage)
- Modify: `internal/workflows/runrecipe_summary.go` (`deriveRunSummary` return type)
- Modify: `internal/workflows/runrecipe_topology.go` (`deriveRecipeTopology` return type → `*models.RunTopology`)
- Modify: `internal/infrastructure/execution/run_persistence.go` (`RunPersistenceActivities`, `RunPersistenceStore` interface — `*models.Run` throughout, + `PersistRunCompiledPlan`)
- Modify: `internal/infrastructure/execution/recipe_workflows.go` (`LaunchRecipeRun(ctx, run *models.Run, ...)`)
- Modify: `internal/services/recipe/service.go` (`RunRepo`, `RecipeWorkflows` interfaces → `*models.Run`)
- Modify: `internal/services/recipe/recipe.go` (`StartRun` mints `models.Run`; `ListRuns`/`CancelRun`/`DeleteRun` type-only changes)
- Modify: `internal/app/glue.go` (wiring: `Store.Runs()` instead of `Store.TestRuns()` for recipe service deps)
- Test: `internal/workflows/runrecipe_test.go` (extend), `internal/workflows/runrecipe_summary_test.go`, `internal/workflows/runrecipe_topology_test.go` (type-adjust), `internal/infrastructure/execution/run_persistence_test.go`, `internal/services/recipe/recipe_test.go`

**Interfaces:**
- Consumes: Task 1 `models.Run`/`models.RunTopology`, Task 2 `postgres.RunRepo`.
- Produces:
  ```go
  // register.go
  const PersistRunCompiledPlanActivityName = "stroppy.runtime.PersistRunCompiledPlan"
  type RuntimeActivities interface {
      PersistRunState(context.Context, string, *workflowpb.RunState, *deploymentpb.InfrastructureState, *deploymentpb.DeploymentPlan) error
      PersistDeploymentPlan(context.Context, string, *deploymentpb.DeploymentPlan) error
      AppendRunLogs(context.Context, []*monitor.LogLine) error
      PersistRunSummary(context.Context, string, *models.Run_Summary) error
      PersistRecipeTopology(context.Context, string, *models.RunTopology) error
      PersistRunCompiledPlan(context.Context, string, *dslpb.CompiledPlan) error
  }

  // runtime.go
  func persistRunCompiledPlan(ctx workflow.Context, runID string, plan *dslpb.CompiledPlan) error

  // runrecipe_summary.go / runrecipe_topology.go
  func deriveRunSummary(plan *dslpb.CompiledPlan) *models.Run_Summary
  func deriveRecipeTopology(plan *dslpb.CompiledPlan, machines map[string][]*deploymentpb.MachineState) *models.RunTopology
  ```

- [ ] **Step 1: Write the failing tests**

Extend `internal/workflows/runrecipe_test.go`'s existing fake persistence store (backing `TestRunRecipeWorkflowPersistsSummaryFromCompiledPlan`/`TestRunRecipeWorkflowPersistsRecipeTopologyAfterProvision`, both already present) to assert against `*models.Run_Summary`/`*models.RunTopology` instead of `*models.TestRunRecord_Summary`/`*models.RecipeTopologySnapshot`, and add:

```go
func TestRunRecipeWorkflowPersistsCompiledPlanAfterCompile(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	fake := newFakeRuntimeActivities() // existing test helper, extend with a CompiledPlan capture field
	registerFakeActivities(env, fake)  // existing helper

	env.ExecuteWorkflow(RunRecipeWorkflowName, RunRecipeInput{
		RunID: "run-1", TenantID: "t1", Bundle: dockerBundleFiles(), // existing test bundle helper
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.NotNil(t, fake.persistedCompiledPlan, "persistRunCompiledPlan must be called after compile succeeds")
	require.NotEmpty(t, fake.persistedCompiledPlan.GetProvider(), "persisted plan is the real compiled plan, not empty")
	// ordering: compiled plan persists no later than the summary persist (same commit point, Task 3's design decision).
	require.LessOrEqual(t, fake.persistedCompiledPlanAt, fake.persistedSummaryAt)
}
```

Add to `internal/infrastructure/execution/run_persistence_test.go`:

```go
func TestPersistRunCompiledPlan_StoresOnRunRecord(t *testing.T) {
	store := newFakeRunStore(t) // extend existing fake to key by models.Run
	store.seed(&models.Run{Entity: &commonpb.Entity{Id: "run-1", TenantId: "t1"}})
	a := &RunPersistenceActivities{store: store}

	plan := &dslpb.CompiledPlan{Provider: &dslpb.ProviderRef{Name: "docker"}}
	require.NoError(t, a.PersistRunCompiledPlan(context.Background(), "run-1", plan))

	got, err := store.RunRecord(context.Background(), "run-1")
	require.NoError(t, err)
	require.Equal(t, "docker", got.GetCompiledPlan().GetProvider().GetName())
}
```

- [ ] **Step 2: Run to verify failure**

```
go test ./internal/workflows/... ./internal/infrastructure/execution/... -run "TestRunRecipeWorkflowPersistsCompiledPlanAfterCompile|TestPersistRunCompiledPlan_StoresOnRunRecord" -v
```

Expected: FAIL — `persistRunCompiledPlan`/`PersistRunCompiledPlan` undefined.

- [ ] **Step 3: Wire the new activity (`register.go`)**

```go
const PersistRunCompiledPlanActivityName = "stroppy.runtime.PersistRunCompiledPlan"

type RuntimeActivities interface {
	PersistRunState(context.Context, string, *workflowpb.RunState, *deploymentpb.InfrastructureState, *deploymentpb.DeploymentPlan) error
	PersistDeploymentPlan(context.Context, string, *deploymentpb.DeploymentPlan) error
	AppendRunLogs(context.Context, []*monitor.LogLine) error
	PersistRunSummary(context.Context, string, *models.Run_Summary) error
	PersistRecipeTopology(context.Context, string, *models.RunTopology) error
	PersistRunCompiledPlan(context.Context, string, *dslpb.CompiledPlan) error
}

func RegisterActivities(registry worker.ActivityRegistry, runtime RuntimeActivities) {
	if runtime != nil {
		registry.RegisterActivityWithOptions(runtime.PersistRunState, activity.RegisterOptions{Name: PersistRunStateActivityName})
		registry.RegisterActivityWithOptions(runtime.PersistDeploymentPlan, activity.RegisterOptions{Name: PersistDeploymentPlanActivityName})
		registry.RegisterActivityWithOptions(runtime.AppendRunLogs, activity.RegisterOptions{Name: AppendRunLogsActivityName})
		registry.RegisterActivityWithOptions(runtime.PersistRunSummary, activity.RegisterOptions{Name: PersistRunSummaryActivityName})
		registry.RegisterActivityWithOptions(runtime.PersistRecipeTopology, activity.RegisterOptions{Name: PersistRecipeTopologyActivityName})
		registry.RegisterActivityWithOptions(runtime.PersistRunCompiledPlan, activity.RegisterOptions{Name: PersistRunCompiledPlanActivityName})
	}
}
```

(`PersistRunState`'s signature is UNCHANGED — `workflow.RunState`/`InfrastructureState`/`DeploymentPlan` params stay as-is; `PersistRunState`'s underlying store write switches to `*models.Run` internally in Step 5, but `InfrastructureState`/`DeploymentPlan` params are simply ignored/unused by the `Run`-backed implementation since `Run` has no such fields — keep the params for interface stability with the (still-registered, until Task 7) `TestRunRecord`-backed activities elsewhere; the `Run`-backed activity impl accepts and no-ops on them, documented inline.)

- [ ] **Step 4: `runtime.go` — new persist function + type updates**

```go
func persistRunCompiledPlan(ctx workflow.Context, runID string, plan *dslpb.CompiledPlan) error {
	actx := runtimeActivityContext(ctx)
	return workflow.ExecuteActivity(actx, PersistRunCompiledPlanActivityName, runID, plan).Get(actx, nil)
}

func persistRunSummary(ctx workflow.Context, runID string, summary *models.Run_Summary) error {
	actx := runtimeActivityContext(ctx)
	return workflow.ExecuteActivity(actx, PersistRunSummaryActivityName, runID, summary).Get(actx, nil)
}

func persistRecipeTopology(ctx workflow.Context, runID string, snapshot *models.RunTopology) error {
	actx := runtimeActivityContext(ctx)
	return workflow.ExecuteActivity(actx, PersistRecipeTopologyActivityName, runID, snapshot).Get(actx, nil)
}
```

`persistRunState`/`persist` (lines 25–41, 628–630) — signatures unchanged (still take `*workflowpb.RunState`), no edit needed beyond what Step 5's store-layer change requires transitively.

- [ ] **Step 5: `runrecipe_summary.go` / `runrecipe_topology.go` — return-type swap**

`deriveRunSummary(plan *dslpb.CompiledPlan) *models.Run_Summary` — same field-by-field derivation logic (spec §3: "derivation logic `deriveRunSummary` не меняется"), only the constructed struct literal's type changes from `&models.TestRunRecord_Summary{...}` to `&models.Run_Summary{...}` (identical field names, confirmed in Task 1).

`deriveRecipeTopology(plan *dslpb.CompiledPlan, machines map[string][]*deploymentpb.MachineState) *models.RunTopology` — same, `&models.RecipeTopologySnapshot{...}` → `&models.RunTopology{...}` literal type swap only.

- [ ] **Step 6: `runrecipe.go` — call the new persist point**

At `runrecipe.go`'s compile-success block (currently line 378 `providerRef = plan.GetProvider()` through line 386's `persistRunSummary` call):

```go
providerRef = plan.GetProvider()
w.completeStage(ctx, runRecipeStageCompileIndex)

if perr := persistRunCompiledPlan(ctx, w.in.RunID, plan); perr != nil {
	return nil, perr
}
if perr := persistRunSummary(ctx, w.in.RunID, deriveRunSummary(plan)); perr != nil {
	return nil, perr
}
```

(Order: compiled-plan persist before summary persist — arbitrary but must match the test's `LessOrEqual` assertion from Step 1; either order is correct, pick one and keep the test in sync.)

- [ ] **Step 7: `run_persistence.go` — store layer on `*models.Run`**

```go
// RunPersistenceStore persists a Run (replaces the TestRunRecord-typed store
// this package used before SP-E's cutover — see run_persistence.go's history
// for the prior shape if a rollback reference is needed).
type RunPersistenceStore interface {
	RunRecord(ctx context.Context, runID string) (*models.Run, error)
	SaveRunRecord(ctx context.Context, run *models.Run) error
}

type RunPersistenceActivities struct{ store RunPersistenceStore }

func (a *RunPersistenceActivities) PersistRunCompiledPlan(ctx context.Context, runID string, plan *dslpb.CompiledPlan) error {
	rec, err := a.store.RunRecord(ctx, runID)
	if err != nil {
		return err
	}
	rec.CompiledPlan = proto.Clone(plan).(*dslpb.CompiledPlan)
	return a.store.SaveRunRecord(ctx, rec)
}

func (a *RunPersistenceActivities) PersistRunSummary(ctx context.Context, runID string, summary *models.Run_Summary) error {
	rec, err := a.store.RunRecord(ctx, runID)
	if err != nil {
		return err
	}
	rec.Summary = mergeRunSummary(rec.GetSummary(), summary) // mergeRunSummary: same additive-merge semantics as today's mergeRunSummary, retyped
	return a.store.SaveRunRecord(ctx, rec)
}

func (a *RunPersistenceActivities) PersistRecipeTopology(ctx context.Context, runID string, snapshot *models.RunTopology) error {
	rec, err := a.store.RunRecord(ctx, runID)
	if err != nil {
		return err
	}
	rec.Topology = proto.Clone(snapshot).(*models.RunTopology)
	return a.store.SaveRunRecord(ctx, rec)
}

func (a *RunPersistenceActivities) PersistRunState(ctx context.Context, runID string, state *workflowpb.RunState,
	_ *deploymentpb.InfrastructureState, _ *deploymentpb.DeploymentPlan) error {
	rec, err := a.store.RunRecord(ctx, runID)
	if err != nil {
		return err
	}
	rec.RuntimeState = proto.Clone(state).(*workflowpb.RunState)
	rec.Status = applyRunSummary(rec, state) // same status-derivation helper, retyped receiver
	return a.store.SaveRunRecord(ctx, rec)
}
```

(`mergeRunSummary`/`applyRunSummary` are the existing functions in this file, retyped from `*models.TestRunRecord`/`*models.TestRunRecord_Summary` receivers — logic unchanged, confirmed by spec §3 "derivation logic ... не меняется".)

Wire production `RunPersistenceStore` in `internal/app/glue.go` against `Store.Runs()` (Task 2's `postgres.RunRepo`) — mirrors glue.go's existing `RunRecord`/`SaveRunRecord` closures over `Store.TestRuns()` (lines 77–95), same pattern, new backing repo.

- [ ] **Step 8: `recipe_workflows.go` — `LaunchRecipeRun`/`CancelRecipeRun` on `*models.Run`**

```go
func (w *RecipeWorkflows) LaunchRecipeRun(ctx context.Context, run *models.Run, bundle map[string][]byte) error {
	// body unchanged except run.GetEntity()/run.GetTenantId() etc. accessors, which are identical field names on Run
}
```

- [ ] **Step 9: `services/recipe` — `RunRepo`/`RecipeWorkflows` interfaces + `StartRun`**

`service.go`:

```go
type RunRepo interface {
	Create(ctx context.Context, run *models.Run) error
	Update(ctx context.Context, run *models.Run) error
	Get(ctx context.Context, tenantID, id string) (*models.Run, error)
	List(ctx context.Context, query *api.ListTestRunsRequest, callerAccountID string) ([]*models.Run, string, error)
	Delete(ctx context.Context, tenantID, id string) error
}

type RecipeWorkflows interface {
	LaunchRecipeRun(ctx context.Context, run *models.Run, bundle map[string][]byte) error
	CancelRecipeRun(ctx context.Context, runID string) error
}
```

`recipe.go`'s `StartRun` (line 164) — the record-minting block (lines 182–207) changes type from `&models.TestRunRecord{...}` to `&models.Run{...}`, same field-by-field construction (`Entity`, `Status=PENDING`, `Trigger=API`, rating flags) PLUS new fields per spec §3 point 1: `WorkflowId: recipeRec.GetEntity().GetId()` (was `RecipeId`), `WorkflowVersion: strconv.FormatUint(recipeRec.GetVersion(), 10)` (was the ad-hoc `"v"+...` string — now stored, not just displayed), `Observability: &models.ObservabilityRefs{MetricsQueryKey: id, LogsQueryKey: id, GrafanaDashboardUid: <existing relay constant>}` filled at mint time (id = the run's own `entity.id`, generated before this literal). `Baked`/`CompiledPlan` stay nil (spec §3 point 1: not available at this step). `markRunFailed` (line 219 call site) — retyped receiver, same logic. `ListRuns`/`CancelRun`/`DeleteRun` — mechanical type-only changes (`*models.TestRunRecord` → `*models.Run` in signatures/locals), no logic change; `ListRuns`'s Go-side `recipe_id` facet filter (per `service.go`'s doc, since `ListTestRunsRequest` has no `recipe_id`/`workflow_id` facet) now filters on `run.GetWorkflowId()` instead of `run.GetRecipeId()`.

`glue.go` — wire `recipe.Deps.Runs`/`.Workflows` against `Store.Runs()`/the retyped `RecipeWorkflows` impl instead of `Store.TestRuns()`.

- [ ] **Step 10: Run to verify pass**

```
go test ./internal/workflows/... ./internal/infrastructure/execution/... ./internal/services/recipe/... -v
go build ./...
```

Expected: PASS. `go build ./...` will still show compile errors in `overview.go`/`compare`/`rating`/`tenant_dashboard`/adapters — those are Task 4/5's job; if this task is landed standalone (not back-to-back with Task 4), apply the sequencing-note shim from this task's header to keep `go build ./...` green, OR land Tasks 3+4 as one PR (recommended) and skip the shim.

- [ ] **Step 11: Commit**

```bash
git add internal/workflows/ internal/infrastructure/execution/run_persistence.go internal/infrastructure/execution/recipe_workflows.go internal/services/recipe/ internal/app/glue.go
git commit -m "feat(recipe): cut RunRecipeWorkflow write path over to models.Run"
```

---

### Task 4: `OverviewReader` + `runtime_topology.go` on `*models.Run`

Migrates the durable-projection read layer that backs every `RunDetail` tab except Config/Quotas/Logs/Metrics/Grafana. Per spec §6.C this covers Overview/Pipeline/Topology/Agents. Grouped into 4 sub-clusters per the file's own structure (`internal/infrastructure/execution/overview.go`, 1212 lines, 21 functions directly on `*models.TestRunRecord` + `internal/infrastructure/execution/runtime_topology.go`, ~1300 lines, ~20 more + `internal/infrastructure/execution/monitoring.go`'s `dbKindString`).

**Files:**
- Modify: `internal/infrastructure/execution/overview.go`
- Modify: `internal/infrastructure/execution/runtime_topology.go`
- Modify: `internal/infrastructure/execution/monitoring.go` (`RunRecord` port + `dbKindString`)
- Test: `internal/infrastructure/execution/overview_test.go` (extend), `internal/infrastructure/execution/runtime_topology_test.go` (extend if present, else create)

**Interfaces:**
- Consumes: Task 1 `models.Run`/`RunTopology`, Task 3's `RunPersistenceStore.RunRecord` now returning `*models.Run`.
- Produces: same public signatures as today, `*models.TestRunRecord` params replaced by `*models.Run` throughout; `SnapshotRunReader.RunRecord(ctx, runID) (*models.Run, error)` (was `*models.TestRunRecord`).

- [ ] **Step 0: Drop the dead classic-`TestWorkflow` branches (design decision, apply throughout this task)**

Per Global Constraints: `models.Run` has no `Spec`/`InfrastructureState`/`DeploymentPlan`. The research confirms these are read in `topologyFromRecordWithRunState`, `topologyState`, `pipelineFromRecord`, `workerMachineIDs`, `workersFromRecord`, `recordInfrastructureStatus`, `recordRenderPlanStatus`, `recordExecutePlanStatus` — but ALSO that `topologyState`'s own comment says recipe runs (the only live path) never populate `InfrastructureState`/`DeploymentPlan` either, branching instead on `RecipeTopology` (→ `Run.topology`). So for every one of these functions, the migration:
1. Deletes the `.Spec`/`.InfrastructureState`/`.DeploymentPlan`-keyed branch (dead for any `*models.Run`, which never has these fields).
2. Keeps/promotes the `.RecipeTopology`(→`.Topology`)-keyed branch as the ONLY branch.

This is a simplification, not a functional regression — the classic branch was already unreachable for the live `RunRecipeWorkflow` path (spec §1 confirms `domainTestWorkflow` is unregistered).

**Discovered gap to fix (beyond spec §6.C's "preserved" label):** `workerMachineIDs`/`workersFromRecord` currently read `.InfrastructureState.Machines`/`.DeploymentPlan.Components` — both of which `topologyState`'s own comment says recipe runs NEVER populate. This means the Agents tab's persisted-fallback path is structurally empty for every live run today (an undocumented gap, since spec's chart didn't distinguish "field renamed" from "field was already silently broken"). Task 4 Step 3 below FIXES this by sourcing `workerMachineIDs`/`workersFromRecord` from `Run.topology.nodes` (which carries exactly `node_id`/`group`/`status`/`services`/`labels` per machine) instead. Flag this explicitly in the PR description and in Task 8's parity table as a **fix**, alongside the already-documented Config-tab fix (spec §6.C).

- [ ] **Step 1: Write the failing tests (fixture-driven, mirrors `overview_test.go`'s existing pattern)**

```go
func TestOverviewFromRun_BuildsFromSummaryAndRuntimeState(t *testing.T) {
	run := &models.Run{
		Entity: &commonpb.Entity{Id: "run-1"},
		Status: commonpb.Status_STATUS_RUNNING,
		Summary: &models.Run_Summary{
			DbKind: domainpb.Database_KIND_POSTGRESQL, ProgressPct: 40,
		},
		RuntimeState: &workflowpb.RunState{},
	}
	got := overviewFromRun("run-1", run, nil, nil)
	require.Equal(t, commonpb.Status_STATUS_RUNNING, got.GetStatus())
	require.EqualValues(t, 40, got.GetProgressPct())
}

func TestTopologyFromRun_UsesTopologyFieldOnly(t *testing.T) {
	run := &models.Run{
		Topology: &models.RunTopology{Provider: "docker", Nodes: []*models.RunTopology_MachineNode{
			{NodeId: "db-0", Group: "db"},
		}},
	}
	got := topologyFromRunWithRunState(run, nil)
	require.Equal(t, topology.Topology_STATE_PROVISIONED, got.GetState()) // exact enum value per topologyState's recipe branch
	require.Len(t, got.GetNodes(), 1) // via runtimeTopologyFromRun, Step 3
}

func TestWorkerMachineIDs_SourcesFromRunTopologyNotDeploymentPlan(t *testing.T) {
	run := &models.Run{Topology: &models.RunTopology{Nodes: []*models.RunTopology_MachineNode{
		{NodeId: "db-0"}, {NodeId: "runner-0"},
	}}}
	ids := workerMachineIDsFromRun(run)
	require.ElementsMatch(t, []string{"db-0", "runner-0"}, ids, "must read Run.topology.nodes, not the always-nil InfrastructureState/DeploymentPlan")
}
```

- [ ] **Step 2: Run to verify failure**

```
go test ./internal/infrastructure/execution/... -run "TestOverviewFromRun|TestTopologyFromRun|TestWorkerMachineIDs" -v
```

Expected: FAIL — new function names undefined.

- [ ] **Step 3: Migrate cluster A — Overview construction**

Rename + retype: `overviewFromRecord`→`overviewFromRun`, `mergeOverviewWithRecord`→`mergeOverviewWithRun`, `persistedRecordIsTerminal`→`persistedRunIsTerminal`, `overlayRunFromOverview` (name already generic, retype param only). All read `.Status`/`.RuntimeState`/`.Summary` — identical field names on `Run`, mechanical `*models.TestRunRecord`→`*models.Run` swap in each signature and body.

- [ ] **Step 4: Migrate cluster B — Pipeline fallback**

Rename + retype: `pipelineFromRecord`→`pipelineFromRun`, `recordStageNode`→`runStageNode`, `recordInfrastructureStatus`→`runInfrastructureStatus`, `recordRenderPlanStatus`→`runRenderPlanStatus`, `recordExecutePlanStatus`→`runExecutePlanStatus`, `recordWorkloadStatus`→`runWorkloadStatus`, `recordTeardownStatus`→`runTeardownStatus`, `recordStageStatusReason`→`runStageStatusReason`. Per Step 0: `runInfrastructureStatus`/`runRenderPlanStatus`/`runExecutePlanStatus` drop their `.InfrastructureState`/`.DeploymentPlan` branches — derive stage status from `.Status` + `.Topology != nil` (provisioned) + `.RuntimeState` stage entries only (the only signals `Run` carries).

- [ ] **Step 5: Migrate cluster C — Topology projection (+ `runtime_topology.go`)**

`topologyFromRecord`→`topologyFromRun`, `topologyFromRecordWithRunState`→`topologyFromRunWithRunState`, `topologyState`→`runTopologyState` (per Step 0, this collapses to a single branch keyed on `run.GetTopology() != nil`). `runtimeTopologyFromRecord` (`runtime_topology.go` line 33) → `runtimeTopologyFromRun`, plus its ~15 downstream helpers (`recipeSnapshot`, `topologyRuntimeServerAddr`, `addControlPlane`, `machineAndAgent`, `controlPlaneRuntimeStatus`, `machineRuntimeStatus`, `machineStatusReason`, `componentRuntimeStatus`, `connectionStatus`, and the 3 more at lines 334/406/510 — confirm each's exact name via `grep -n "func.*models.TestRunRecord" internal/infrastructure/execution/runtime_topology.go` at Step 5 time) — every one takes `rec *models.TestRunRecord` purely to read `.RecipeTopology`/`.Status`/`.RuntimeState`; retype to `run *models.Run` reading `.Topology`/`.Status`/`.RuntimeState` (same field names except `RecipeTopology`→`Topology`). No logic changes beyond the Step 0 dead-branch removal in `recipeSnapshot`'s caller (`topologyFromRunWithRunState`/`runTopologyState`).

- [ ] **Step 6: Migrate cluster D — Worker/agent presence (+ the discovered fix)**

`agentPresence`→ retype param. `workerMachineIDs`→`workerMachineIDsFromRun`: per Step 0's discovered gap, source node IDs from `run.GetTopology().GetNodes()` (`[]*models.RunTopology_MachineNode`, each `.GetNodeId()`) instead of `.InfrastructureState.Machines`/`.DeploymentPlan.Components`. `workersFromRecord`→`workersFromRun`: build `monitor.WorkerInfo` from the same `Run.topology.nodes` entries (`.GetGroup()`, `.GetIp()`, `.GetStatus()`, `.GetServices()`) instead of `deploymentpb.MachineState`/`DeploymentPlan.Components` — `RunTopology.MachineNode` already carries everything `workersFromRecord` needs (confirmed Task 1's message shape mirrors `MachineState`'s useful subset). `recordWorkerPresence`→`runWorkerPresence`, `recordWorkerStatusReason`→`runWorkerStatusReason` — retype only, still read `.Status`.

- [ ] **Step 7: `monitoring.go` + `SnapshotRunReader` interface**

`monitoring.go` line 65: `RunRecord(ctx context.Context, runID string) (*models.TestRunRecord, error)` → `(*models.Run, error)`; `dbKindString(rec *models.TestRunRecord)` (line 391) → `dbKindString(run *models.Run)`, reads `.Summary.DbKind` unchanged. `overview.go` line 57's `SnapshotRunReader.RunRecord` — same retype. `OverviewReader.Get` (line 141) reads `.RecipeId`→`.WorkflowId` when picking the recipe-vs-classic workflow id (per Step 0, this becomes unconditional — every `Run` has a `WorkflowId`, no classic branch remains).

- [ ] **Step 8: Run to verify pass**

```
go test ./internal/infrastructure/execution/... -v
go build ./...
```

Expected: PASS for this package; `compare`/`rating`/`tenant_dashboard`/adapters/web still red until Task 5/6 (same sequencing note as Task 3 — recommend landing Task 4 immediately after Task 3, or apply an equivalent shim to those callers).

- [ ] **Step 9: Commit**

```bash
git add internal/infrastructure/execution/overview.go internal/infrastructure/execution/runtime_topology.go internal/infrastructure/execution/monitoring.go internal/infrastructure/execution/overview_test.go
git commit -m "feat(execution): migrate OverviewReader and runtime topology projection to models.Run"
```

---

### Task 5: Service/adapter layer + API proto rename

Migrates every remaining Go reader (spec §3.4's `compare`/`tenant_dashboard`/`quota`/adapters) and renames the API-facing proto messages that embed `TestRunRecord` (spec §4's "Зависимые API-сообщения").

**Files:**
- Modify: `protocols/cloud/v1/api/test_run.proto` (`ListTestRunsRequest`→`ListRunsRequest`, `ListTestRunsResponse`→`ListRunsResponse`, `.runs` field type)
- Modify: `protocols/cloud/v1/api/test_run_overview.proto` (`TestRunOverviewSnapshot.run` field type)
- Modify: `protocols/cloud/v1/api/recipe.proto` (`StartRunResponse.run`, `ListRunsResponse.runs` field types)
- Modify: `internal/services/compare/compare.go`, `service.go`
- Modify: `internal/services/quota/service.go`
- Modify: `internal/services/tenant_dashboard/service.go`
- Modify: `internal/infrastructure/adapters/compare.go`, `favorite.go`, `rating.go`, `share.go`, `tenant_dashboard.go`
- Modify: `internal/app/glue.go` (remaining wiring: `RatingRunsLister`, `DashboardRunsReader`, `ShareRunReader`, `FavoriteTargetRepos` entries for runs)
- Test: `internal/infrastructure/adapters/rating_test.go`, `tenant_dashboard_test.go` (extend)

**Interfaces:**
- Produces (proto rename, values unchanged): `api.ListRunsRequest`/`api.ListRunsResponse` (was `ListTestRunsRequest`/`ListTestRunsResponse`, same fields — `suite_run_id`/`suite_cell_id`/`standalone` fields STAY on the request proto per Task 2 Step 4's note, becoming no-ops; do not remove them from the request shape in this task, only from `Run`-side consumption), `api.TestRunOverviewSnapshot.run models.Run`, `api.StartRunResponse.run models.Run`, `api.recipe.ListRunsResponse.runs repeated models.Run`.

- [ ] **Step 1: Proto renames**

`test_run.proto`: rename message `ListTestRunsRequest`→`ListRunsRequest` (all fields unchanged, including the now-inert `suite_run_id`/`suite_cell_ids`/`standalone`), `ListTestRunsResponse`→`ListRunsResponse` with `repeated models.Run runs = 1` (was `repeated models.TestRunRecord runs = 1`). `ListTestRunFacetsRequest`/`Response` — unchanged names, no `TestRunRecord` reference (confirmed by earlier grep: only `ListTestRunsResponse.runs` embeds the type in this file).

> Naming collision: `api/recipe.proto` ALREADY defines its own `ListRunsRequest`/`ListRunsResponse` (spec §9 open question 1, resolved in Global Constraints: keep both, `test_run.proto`'s stays the facet/sort source of truth, `recipe.proto`'s stays a thin `workflow_id`-filter wrapper). Since both are named `ListRunsRequest`, verify they live in different proto packages/Go packages without a Go import collision — `test_run.proto` is `package cloud.v1.api` same as `recipe.proto` (check via `head -5` on both files before this step) — if they share a Go package, the generated Go type names collide (`api.ListRunsRequest` ambiguous). If so, keep `test_run.proto`'s original name `ListTestRunsRequest`/`ListTestRunsResponse` UNCHANGED (only retype the embedded `.runs` field to `models.Run`) rather than renaming — this still satisfies spec §4's requirement (which only asks for "an equivalent", not a specific new name) and avoids the collision entirely. Prefer this fallback unless the packages are confirmed distinct.

`test_run_overview.proto` line 131: `models.TestRunRecord run = 1` → `models.Run run = 1` on `TestRunOverviewSnapshot`.

`recipe.proto` line 178: `models.TestRunRecord run = 1` → `models.Run run = 1` on `StartRunResponse`. Line 209: `repeated models.TestRunRecord runs = 1` → `repeated models.Run runs = 1` on (recipe-scoped) `ListRunsResponse`.

`compare.proto`/`rating.proto` — NOT modified (spec §4: they use flat `RunColumn`/`RatingEntry` DTOs, never reference `TestRunRecord`/`Run` directly).

- [ ] **Step 2: Generate**

```bash
make protocols
```

- [ ] **Step 3: Write failing tests for each migrated service (representative — one per package)**

```go
// internal/services/compare/compare_test.go
func TestCompareRuns_ReadsRunSummaryNotTestRunRecord(t *testing.T) {
	svc := NewCompareService(Deps{Runs: fakeRunGetter{"run-1": &models.Run{
		Entity: &commonpb.Entity{Id: "run-1", TenantId: "t1"},
		Summary: &models.Run_Summary{DbKind: domainpb.Database_KIND_POSTGRESQL},
	}}, ...})
	resp, err := svc.CompareRuns(ctx, &api.CompareRunsRequest{TenantId: "t1", RunIds: []string{"run-1"}})
	require.NoError(t, err)
	require.Equal(t, domainpb.Database_KIND_POSTGRESQL, resp.GetColumns()[0].GetDbKind())
}
```

```go
// internal/infrastructure/adapters/rating_test.go — extend existing fixtures
func TestRankRuns_UsesRunTopLevelRatingFlags(t *testing.T) {
	runs := []*models.Run{{Entity: &commonpb.Entity{Id: "r1"}, InTenantRating: true, Summary: &models.Run_Summary{}}}
	ranked := rankRuns(ctx, fakeMetrics{}, runs, "throughput")
	require.Len(t, ranked, 1)
}
```

- [ ] **Step 4: Run to verify failure, then migrate each package**

For each of the 5 files/packages below, the change is a mechanical `*models.TestRunRecord`→`*models.Run` retype (spec §3.4 already confirms: "конвертеры... не меняют форму API-сообщения (`RunColumn`/`FavoriteItem`/`RatingEntry`/share-snapshot/dashboard-виджет)"):

1. `internal/services/compare/compare.go`: `CompareRuns` (line 22), `assertTenant(rec *models.TestRunRecord, ...)` (line 75) → `*models.Run`. `internal/infrastructure/adapters/compare.go`: `TestRunReader.Get` (line 22/42) → returns `*models.Run`; wire against `Store.Runs()` instead of `Store.TestRuns()` in `glue.go`.
2. `internal/infrastructure/adapters/rating.go`: `RatingRunsLister.ListRatingRuns` (line 38), `rankedRun.run` (line 243), `rankRuns` (line 252), `matchFacets` (line 299) → `*models.Run`; field reads (`.Summary.*`, `.InTenantRating`/`.InGlobalRating`, `.Entity.*`) unchanged names.
3. `internal/infrastructure/adapters/tenant_dashboard.go`: `DashboardRunsReader.ListTenantRuns` (line 39), `RecentRunsReader.RecentRuns` (line 120, incl. its `sortByCreatedDesc` closure at line 125) → `*models.Run`.
4. `internal/infrastructure/adapters/favorite.go`: `EntityGetterFromRecord[*models.TestRunRecord]` (line 117, a compile-time usage-example var) → `EntityGetterFromRecord[*models.Run]`; the generic itself (line 107) is untouched (works for any `GetEntity() *common.Entity` type).
5. `internal/infrastructure/adapters/share.go`: `ShareRunReader.GetTestRun` (line 51), `RunSnapshotBuilder.sharedTestRun(ctx, rec *models.TestRunRecord)` (line 99) → `*models.Run`; the public-snapshot field subset it reads (per spec §6.F: `entity`/`summary`, no auth-only fields) is unchanged.
6. `internal/services/quota/service.go`: the injected `Get(ctx, tenantID, id) (*models.TestRunRecord, error)` port (line 18) → `(*models.Run, error)` — spec §6.C confirms this is "mechanical правка типа" since quotas key off `tenant_id`+`run_id`, never a `Run`-specific field.
7. `internal/services/tenant_dashboard/service.go`: `RecentRuns(ctx, tenantID, limit) ([]*models.TestRunRecord, error)` port (line 61) → `[]*models.Run`.

- [ ] **Step 5: `glue.go` — final wiring sweep**

Every closure in `glue.go` that currently does `rec := &modelspb.TestRunRecord{}` / calls `Store.TestRuns()` for a Task-4/5-migrated consumer switches to `&modelspb.Run{}` / `Store.Runs()` (lines 36–95, 131–157, 244 per the earlier repo-wide grep). Cross-check against the grep output from this plan's research (`internal/app/glue.go` appeared with 12 `TestRunRecord` hits) — every one should now resolve to a Task-4/5-migrated consumer; if any remain pointing at `postgres.TestRunRepo`/`test_run_records` after this step, that consumer was missed and must be added to this task before proceeding.

- [ ] **Step 6: Run to verify pass**

```
go test ./internal/services/... ./internal/infrastructure/adapters/... -v
go build ./...
```

Expected: PASS, full `go build ./...` green (everything except web now compiles against `models.Run`).

- [ ] **Step 7: Commit**

```bash
git add protocols/cloud/v1/api/ internal/proto/cloud/v1/api/ web/src/lib/proto/cloud/v1/api/ internal/services/compare/ internal/services/quota/ internal/services/tenant_dashboard/ internal/infrastructure/adapters/ internal/app/glue.go
git commit -m "feat(services): migrate compare/rating/dashboard/quota/favorite/share adapters to models.Run"
```

---

### Task 6: Web readers on the new model

Migrates `web/src/services/{runs,run_overview,recipe,dashboard}.ts` and the pages/components that consume their VMs, per the research inventory (spec §3.4/§6). `Compare.tsx`/`services/compare.ts` are confirmed to need NO changes — `CompareColumnVM` is sourced entirely from `api.RunColumn`, a flat DTO the compare service already builds server-side (Task 5), never touching `TestRunRecord`/`Run` JSON shape directly in TS.

**Files:**
- Modify: `web/src/services/runs.ts` (`testRunRecordToVM`→`runToVM`, `RunVM`)
- Modify: `web/src/services/run_overview.ts` (`OverviewVM`, `snapshotToVM`)
- Modify: `web/src/services/recipe.ts` (`RunVM`, `runRecordToVM`→`runToVM`)
- Modify: `web/src/services/dashboard.ts` (`recentRunFromRecord`)
- Modify: `web/src/pages/RunDetail.tsx` (`recipeId`→`workflowId` rename site)
- Modify: `web/src/components/run/RunInfoSidebar.tsx` (drop the `suiteRunId` row)
- Modify: `web/src/components/run/RunConfigTab.tsx` (spec §6.C fix: read `Baked`/`CompiledPlan` instead of empty `spec`)
- Not modified: `web/src/pages/Compare.tsx`, `web/src/services/compare.ts`, `web/src/pages/RecipeRuns.tsx` (no direct field reference beyond `services/recipe.ts`'s VM, covered above)

**Interfaces:**
- Consumes: Task 5's `api.ListRunsResponse`/`ListTestRunsResponse` (whichever name Task 5 Step 1 settled on), `TestRunOverviewSnapshot.run models.Run`, `StartRunResponse.run models.Run`.
- Produces:
  ```ts
  export interface RunVM { /* same fields as today minus suiteRunId, recipeId renamed workflowId */ }
  export function runToVM(rec: Run): RunVM
  ```

- [ ] **Step 1: Write the failing tests**

```ts
// web/src/services/runs.test.ts (new, or extend existing test file if one exists — check web/src/services/*.test.ts convention first)
import { describe, it, expect } from "vitest";
import { create } from "@bufbuild/protobuf";
import { RunSchema } from "@/lib/proto/cloud/v1/models/test_run_pb";
import { runToVM } from "./runs";

describe("runToVM", () => {
  it("maps workflowId, drops suiteRunId", () => {
    const run = create(RunSchema, {
      entity: { id: "run-1" },
      workflowId: "recipe-1",
      summary: { dbKind: 0 },
    });
    const vm = runToVM(run);
    expect(vm.workflowId).toBe("recipe-1");
    expect((vm as any).suiteRunId).toBeUndefined();
    expect((vm as any).recipeId).toBeUndefined();
  });
});
```

- [ ] **Step 2: Run to verify failure**

```
cd web && npx vitest run src/services/runs.test.ts
```

Expected: FAIL — `runToVM` undefined (or `RunSchema` unresolved until `make protocols` regenerates `web/src/lib/proto`, already done in Task 5).

- [ ] **Step 3: `runs.ts` — rename mapper + VM**

`RunVM` interface (lines 76–148): remove `suiteRunId` (109–110) entirely; rename `recipeId` (111–118) to `workflowId`, doc comment updated to reference `Run.workflow_id`. `testRunRecordToVM` (237–343) → `runToVM(rec: Run): RunVM`, import switches from `TestRunRecordSchema, type TestRunRecord` to `RunSchema, type Run` (`@/lib/proto/cloud/v1/models/test_run_pb`). Local JSON-shape type annotation (238–293): drop `suiteRunId?: string`, rename `recipeId?: string` → `workflowId?: string`. Field mapping lines 315–343: drop line 332 (`suiteRunId: j.suiteRunId ?? ""`), rename line 333 (`recipeId: j.recipeId ?? ""` → `workflowId: j.workflowId ?? ""`). Every other field (315–331, unaffected per spec §6.A: `entity.*`, `status`, `trigger`, `summary.*`) unchanged.

Fix per spec §6.C (Config tab): add `baked?: unknown` / `compiledPlan?: unknown` (or their properly-typed protobuf-ES equivalents, `Baked`/`CompiledPlan` message types) to `RunVM`, sourced from `j.baked`/`j.compiledPlan` — these feed `RunConfigTab.tsx`'s fix in Step 6.

- [ ] **Step 4: `run_overview.ts`**

`OverviewVM.suiteRunId: string` (line 157) — DELETE the field. Line 506 (`suiteRunId: runRecord?.suiteRunId ?? ""`) — DELETE the assignment. `OverviewVM.run?: RunVM` (154) — unchanged wiring except its source: `runRecord: TestRunRecord | undefined` (line 468 param) → `runRecord: Run | undefined`, and `testRunRecordToVM(runRecord)` (505) → `runToVM(runRecord)`. Imports (line 7): `TestRunRecordSchema, type TestRunRecord` → `RunSchema, type Run`; `TestRunOverviewSnapshotSchema` (13, `api/test_run_overview_pb`) — regenerated by Task 5's proto rename, its `.run` field is now `Run`-typed, no import change needed beyond the schema/type swap already made.

- [ ] **Step 5: `recipe.ts` and `dashboard.ts`**

`recipe.ts`: `RunVM` interface (94–105) — rename `recipeId` (101–102) to `workflowId`; `runRecordToVM` (162–176) → `runToVM`, line 173 (`recipeId: j.recipeId ?? ""`) → `workflowId: j.workflowId ?? ""`; imports `TestRunRecordSchema, type TestRunRecord` (20–23) → `RunSchema, type Run`. **Do NOT touch** `startRun`/`listRuns`'s own `recipeId` PARAMETER (lines 286–323) — confirmed by research to be a different concept (which `RecipeRecord` bundle to act on / filter by), unrelated to the `Run.workflow_id` rename.

`dashboard.ts`: `recentRunFromRecord` (187–201) — retype param `rec: TestRunRecord` → `rec: Run`, import swap only; confirmed by research that this mapper touches none of the renamed/dropped fields (`entity.*`, `status`, `summary.*` only).

- [ ] **Step 6: `RunDetail.tsx`, `RunInfoSidebar.tsx`, `RunConfigTab.tsx`**

`RunDetail.tsx` line 325: `const recipeId = run?.recipeId;` → `const workflowId = run?.workflowId;`, propagated through the `onRerun`/Rerun-button call site (329, 337, 446) — `recipeStartRun(tenantSlug, workflowId)`. Comment block (318–323) updated to reference `Run.workflow_id`.

`RunInfoSidebar.tsx` line 189: delete the `{run?.suiteRunId && <Row ... />}` block entirely (spec §6.B: dropped field, no replacement row).

`RunConfigTab.tsx` (spec §6.C fix — the one genuine "fixed, not preserved" item): today (lines 186–217, 286–287) renders `RecipeBundleView` keyed off `run.recipeId` (→ `run.workflowId` per Step 3's rename) when the `domain.TestRun`-shaped `spec`/`workloadSegments` fields are empty (always true for a recipe run). Per spec §6.C, this task ADDS a real data source: render `Run.bakedFormValues`/`Run.compiledPlan`'s job/service/params (from Step 3's new `RunVM.baked`/`.compiledPlan` fields) as the primary config view, falling back to the existing `RecipeBundleView` (bundle source listing) as a secondary/raw-files section — do not remove `RecipeBundleView`, it still shows the underlying recipe files, which `Baked`/`CompiledPlan` don't replace. Exact UI treatment (e.g. a new "Resolved Parameters" panel above the existing bundle view) is a UI-design decision left to implementation; the requirement is that SOME rendering of `baked`/`compiledPlan` data appears where today nothing does.

- [ ] **Step 7: Run to verify pass**

```
cd web && npx vitest run src/services/
cd web && npm run typecheck
```

- [ ] **Step 8: Commit**

```bash
git add web/src/services/runs.ts web/src/services/run_overview.ts web/src/services/recipe.ts web/src/services/dashboard.ts web/src/pages/RunDetail.tsx web/src/components/run/RunInfoSidebar.tsx web/src/components/run/RunConfigTab.tsx web/src/services/runs.test.ts
git commit -m "feat(web): migrate run readers from TestRunRecord to Run"
```

---

### Task 7: Delete `TestRunRecord` / `test_run_records` / `RecipeTopologySnapshot`

Only task that removes code. Runs after Tasks 3–6 have moved every writer and reader onto `models.Run`. Confirms nothing still references the old type before deleting it (compile-time enforced).

**Files:**
- Modify: `protocols/cloud/v1/models/test_run.proto` (delete `TestRunRecord`, `RecipeTopologySnapshot` messages)
- Delete: `internal/infrastructure/postgres/test_run_list.go`, `test_run_list_test.go`
- Modify: `internal/infrastructure/postgres/store.go` (delete `TestRunRepo`, `TestRuns()`)
- Modify: `internal/infrastructure/postgres/schema.sql` (drop `test_run_records` table + index)
- Modify: `internal/infrastructure/postgres/queries/records.sql` (delete `TestRunRecord`-block queries)
- Modify: `internal/workflows/register.go`, `runtime.go` (delete any now-orphaned `TestRunRecord`-typed helper left from Task 3's transition, if the sequencing note's shim was used)
- New migration (via `make migrate-generate`): drops `test_run_records`

- [ ] **Step 1: Confirm zero remaining references**

```bash
grep -rln "models\.TestRunRecord\|TestRunRecord{" --include="*.go" internal | grep -v "_test.go\|/gen/\|/proto/cloud/v1/"
```

Expected: empty (proto-generated files and this plan's own working-tree diffs aside). If non-empty, resolve each hit before proceeding — do not delete the type out from under a live consumer.

```bash
grep -rln "TestRunRecord\|suiteRunId\|recipeId\b" web/src --include="*.ts" --include="*.tsx" | grep -v "/lib/proto/"
```

Expected: only `services/recipe.ts`'s intentionally-untouched `startRun`/`listRuns` `recipeId` PARAMETER (Task 6 Step 5's explicit exclusion) — confirm every other hit was migrated in Task 6.

- [ ] **Step 2: Delete the proto messages**

Remove `TestRunRecord` and `RecipeTopologySnapshot` from `protocols/cloud/v1/models/test_run.proto` entirely (both messages, in full — spec §5 confirms `RecipeTopologySnapshot`'s content already lives on `RunTopology`, nothing to preserve).

```bash
make protocols
```

Expected: `internal/proto/cloud/v1/models/test_run.pb.go` etc. lose the `TestRunRecord`/`RecipeTopologySnapshot` Go types; `web/src/lib/proto/.../test_run_pb.ts` likewise. `go build ./...` and `cd web && npm run typecheck` both fail loudly at ANY remaining reference — this is the safety net for Step 1's grep.

- [ ] **Step 3: Drop the postgres table + repo**

`store.go`: delete the `TestRunRepo` type and its 5 methods (lines 36–112), delete `Store.TestRuns()` (line 27). `test_run_list.go`/`test_run_list_test.go`: delete both files wholesale (per Global Constraints, this was always the plan — `run_list.go` is the sole survivor). `queries/records.sql`: delete the `CreateTestRunRecord`/`GetTestRunRecord`/`ListTestRunRecords`/`UpdateTestRunRecord`/`DeleteTestRunRecord`/`ExistsTestRunRecord` block (original lines 3–21). `schema.sql`: delete the `test_run_records` table + `idx_test_run_records_tenant` index.

```bash
make db-gen
make migrate-generate name=drop_test_run_records
```

- [ ] **Step 4: Run to verify pass**

```
go build ./...
go test ./... 
cd web && npm run typecheck && npx vitest run
```

Expected: fully green — this is the first point since Task 1 where the codebase has exactly ONE run-record type again.

- [ ] **Step 5: Commit**

```bash
git add protocols/cloud/v1/models/test_run.proto internal/proto/ web/src/lib/proto/ internal/infrastructure/postgres/ internal/workflows/
git commit -m "chore(models): remove TestRunRecord, RecipeTopologySnapshot, test_run_records"
```

---

### Task 8: Data-parity guard tests (spec §6 chart)

Cross-cutting suite that locks the invariant the whole SP exists to satisfy: every row of spec §6's A–F chart has a passing test asserting the mapped value is identical (or, for the 2 documented exceptions, correctly changed). This task can start as soon as Task 6 lands (needs the full read chain); its sub-steps mirror the spec's own lettered sections.

**Files:**
- Create: `internal/infrastructure/execution/overview_parity_test.go`
- Create: `internal/infrastructure/postgres/run_list_parity_test.go`
- Create: `internal/infrastructure/adapters/rating_parity_test.go` (extend `rating_test.go` if simpler)
- Create: `internal/services/compare/compare_parity_test.go`
- Create: `web/src/services/run_parity.test.ts`

- [ ] **Step 1: §6.A — runs-table fixture test**

```go
func TestRunListParity_AllSummaryFacetsAndSortKindsPreserved(t *testing.T) {
	// One Run fixture with every Summary facet set to a distinguishable value,
	// run through RunRepo.List's WHERE/ORDER builder for each
	// ListTestRunsRequest_Sort_Kind (KIND_STATUS..KIND_TRIGGER, spec §6.A's row
	// list) and each filter facet (author_ids, stroppy_versions,
	// db_preset_ids, workload_preset_ids, test_preset_ids) — asserting the
	// generated SQL references the expected jsonb path for every one, and
	// that suite_run_id/suite_cell_id/standalone are silently ignored (no
	// WHERE clause emitted), per Task 2 Step 4/5.
}
```

- [ ] **Step 2: §6.B/§6.C — RunDetail sidebar + tabs**

```go
func TestOverviewParity_SidebarAndTabFieldsFromRunFixture(t *testing.T) {
	run := runFixtureWithEveryField(t) // helper: populates entity/status/trigger/summary/topology/runtimeState/baked/compiledPlan/observability, one Run covering every spec §6.B/C row
	snap := overviewReaderGet(t, run) // via OverviewReader.Get against a fake SnapshotRunReader seeded with the fixture

	// §6.B sidebar fields
	require.Equal(t, run.GetEntity().GetAuthorId(), snap.GetRun().GetEntity().GetAuthorId())
	require.Equal(t, run.GetSummary().GetDbKind(), snap.GetRun().GetSummary().GetDbKind())
	// (suiteRunId intentionally absent — dropped, §6.B)

	// §6.C tabs
	require.NotNil(t, snap.GetOverview())     // Overview tab
	require.NotEmpty(t, snap.GetTopology().GetNodes()) // Topology tab, sourced from run.Topology
	require.NotEmpty(t, snap.GetWorkers())    // Agents tab — the discovered fix, sourced from run.Topology.Nodes not InfrastructureState
}
```

- [ ] **Step 3: §6.C Config-tab fix regression guard**

```ts
// web/src/services/run_parity.test.ts
it("RunConfigTab has non-empty baked/compiledPlan source for a recipe run (fixed, was always empty via spec)", () => {
  const run = create(RunSchema, { compiledPlan: { provider: { name: "docker" } } });
  const vm = runToVM(run);
  expect(vm.compiledPlan).toBeDefined(); // was `spec: undefined` always, pre-SP-E
});
```

- [ ] **Step 4: §6.D — Compare**

```go
func TestCompareParity_RunColumnFieldsFromRunFixture(t *testing.T) {
	run := runFixtureWithEveryField(t)
	col := runToRunColumn(run) // internal/services/compare's mapping, exercised directly
	require.Equal(t, run.GetEntity().GetId(), col.GetRunId())
	require.Equal(t, run.GetSummary().GetDbKind(), col.GetDbKind())
	require.Equal(t, run.GetSummary().GetTopologyLabel(), col.GetTopologyLabel())
	// ... one assertion per §6.D row
}
```

- [ ] **Step 5: §6.E — Rating**

```go
func TestRatingParity_InTenantAndGlobalRatingFlagsFromRun(t *testing.T) {
	run := &models.Run{InTenantRating: true, InGlobalRating: false, Summary: &models.Run_Summary{}}
	entries := rankRuns(context.Background(), fakeMetrics{}, []*models.Run{run}, "throughput")
	require.True(t, entries[0].run.GetInTenantRating())
}
```

- [ ] **Step 6: §6.F — Favorite/Share/tenant-dashboard**

```go
func TestFavoriteShareDashboardParity_ReadEntityAndSummaryFromRun(t *testing.T) {
	run := runFixtureWithEveryField(t)
	// Favorite: EntityGetterFromRecord[*models.Run] returns run.GetEntity() unchanged.
	require.Equal(t, run.GetEntity(), (EntityGetterFromRecord[*models.Run](fakeGetter(run)).GetEntity(context.Background(), "t1", "run-1")))
	// Share: sharedTestRun projects entity+summary only, no spec/suite fields.
	snap := (&RunSnapshotBuilder{}).sharedTestRunPublic(run)
	require.Empty(t, snap.GetSuiteRunId) // method must not even exist on the share snapshot type — compile-time guard, not runtime
	// Tenant-dashboard: RecentRuns/StatusCounts read summary+entity, verified via existing tenant_dashboard_test.go fixtures retyped to models.Run (Task 5).
}
```

- [ ] **Step 7: Full-suite + E2E run**

```
go test ./... -v
cd web && npx vitest run && npm run typecheck
```

Then the live e2e per spec §8: run a docker recipe-run on the dev stand end-to-end (compile→provision→execute(nomad)→teardown→COMPLETED) and manually confirm RunDetail renders all tabs including the now-populated Config tab — see `project_dsl_pivot`/`project_stage_invariant_matrix` memory for the existing stage-test harness this reuses.

- [ ] **Step 8: Commit**

```bash
git add internal/infrastructure/execution/overview_parity_test.go internal/infrastructure/postgres/run_list_parity_test.go internal/infrastructure/adapters/rating_parity_test.go internal/services/compare/compare_parity_test.go web/src/services/run_parity.test.ts
git commit -m "test(run): add data-parity guard suite for the TestRunRecord->Run migration"
```

---

## Self-Review

**Spec coverage — data-parity checklist §6 ↔ tasks (every row accounted for):**

| Spec §6 section | Rows | Migrated by | Guarded by |
|---|---|---|---|
| §6.A runs-table (id/name/author/dbKind/workload/.../recipeId/suiteRunId/sort-kinds/facets) | ~20 rows | Task 2 (`RunRepo.List`) | Task 8 Step 1 |
| §6.B RunDetail sidebar (status/owner/created/started/finished/trigger/suite/db-kind/.../workload) | ~10 rows | Task 4 (Overview cluster) + Task 6 (`runs.ts`/`RunInfoSidebar.tsx`) | Task 8 Step 2 |
| §6.C RunDetail tabs (Overview/Pipeline/Topology/Agents/Quotas/Logs/Metrics/Grafana/Config) | 9 rows | Task 4 (all durable-projection tabs) + Task 5 (quota, mechanical) + Task 6 (`RunConfigTab.tsx` fix) | Task 8 Step 2–3 |
| §6.D Compare (`RunColumn` 8 fields + metrics) | 8 rows | Task 5 (`compare.go`/`adapters/compare.go`) | Task 8 Step 4 |
| §6.E Rating (`RatingEntry`/`RatingFilter` ~10 fields) | 10 rows | Task 5 (`adapters/rating.go`) | Task 8 Step 5 |
| §6.F Favorite/Share/tenant-dashboard | 3 sub-items | Task 5 (`adapters/favorite.go`/`share.go`/`tenant_dashboard.go`) | Task 8 Step 6 |
| §6 "Итог" — 2 documented changes (suite dropped, Config fixed) | 2 | Task 2/Task 4/Task 6 (drop), Task 6 Step 6 (fix) | Task 8 Step 3 explicitly regression-guards the fix |

**Zero readers lost — cross-check against spec §3.4's explicit list:**
- `OverviewReader` (26 functions across `overview.go`+`runtime_topology.go`) → Task 4. ✓
- `RunRepo.List`/`Facets` (postgres) → Task 2 (structure) + Task 5 (proto rename) → 4 shared callers (`RecipeService.ListRuns`, `RatingService`, tenant-dashboard, rating glue) all covered transitively since they all go through the one `List` implementation. ✓
- `internal/services/compare` → Task 5 Step 4.1. ✓
- `internal/services/recipe` (`StartRun`/`ListRuns`/`CancelRun`/`DeleteRun`) → Task 3 Step 9. ✓
- `internal/services/tenant_dashboard` → Task 5 Step 4.3/4.7. ✓
- `internal/services/quota` → Task 5 Step 4.6. ✓
- `internal/infrastructure/adapters/{compare,favorite,rating,share,tenant_dashboard}.go` → Task 5 Step 4.1–4.5. ✓
- Web: `services/{runs,run_overview,recipe,dashboard}.ts`, `RunDetail.tsx`, `RecipeRuns.tsx` (unaffected, confirmed no direct field ref), `Compare.tsx` (unaffected, confirmed), `RunInfoSidebar.tsx`, `RunConfigTab.tsx` → Task 6. ✓
- `PipelineViewer.tsx`/`TopologyViewer.tsx` — spec §3.4 confirms these are already decoupled from `TestRunRecord` (work over `topology.Topology`/`monitor.PipelineView`); no task touches them, correctly out of scope. ✓

**Beyond-spec findings surfaced during planning (flagged inline, not silently absorbed):**
- Task 4 Step 0/6: `workerMachineIDs`/`workersFromRecord` read `.InfrastructureState.Machines`/`.DeploymentPlan.Components`, which `topologyState`'s own existing comment says recipe runs NEVER populate — meaning the Agents tab's persisted-fallback path is likely already silently empty for every live run today. Task 4 fixes this by sourcing from `Run.topology.nodes` instead of porting the (structurally-always-empty) old path forward. Flagged in the PR description and Task 8 Step 2's test as a discovered-and-fixed gap, distinct from spec's own documented Config-tab fix.
- Task 5 Step 1: `api/recipe.proto`'s pre-existing `ListRunsRequest`/`ListRunsResponse` name collision risk with `test_run.proto`'s proposed rename (spec §9 open question 1) — resolved with an explicit fallback (keep `test_run.proto`'s original name, retype only the embedded field) if both protos share a Go package.
- Task 1 Step 1: flags that `schemapb.Baked`'s exact wire representation (proto field vs. opaque bytes) must be confirmed against SP-A's actual landed code before the proto is written — this plan's snippet is written against the SP-A *plan's* stated shape, which may drift from what SP-A actually ships.

**Global Constraints resolve spec §9 open questions 1–4** (`ListTestRunsRequest` stays source of truth; `workflow_version` = `RecipeRecord.version`; `ObservabilityRefs` filled at mint time; `reserved 14-20` left untouched for the matrix `batch_id` question) — explicit design decisions, not deferred.

**Placeholder scan:** no bare TODO/TBD left unresolved as a plan gap — every "confirm against actual code" note (Task 1 Step 1's schemapb/dsl import paths, Task 5 Step 1's package-collision check, Task 4 Step 5's exact `runtime_topology.go` helper names) is an explicit, actionable verification step for the implementer, not an unspecified placeholder, and each is scoped to a single Step with a concrete fallback.

**Type/interface consistency:** `models.Run`/`models.RunTopology`/`models.ObservabilityRefs` (Task 1) flow unchanged in shape through Task 2's `RunRepo`, Task 3's `RunPersistenceStore`/`RuntimeActivities`, Task 4's `OverviewReader`, Task 5's service ports, and Task 6's `RunVM` — one proto message remains the source of truth for both write and read paths throughout (spec §9 open question 5's explicit non-decision, upheld).
