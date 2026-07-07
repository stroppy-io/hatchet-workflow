# Phase 1F: Remodel Deleted/Broken Features Under Recipe Model

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Checkbox steps.

**Goal:** Remodel every feature the over-aggressive deletion broke or dropped, under the new recipe/DSL model — not restore-as-was. Root cause of most breakage: recipe runs persist a `TestRunRecord` with empty `Summary`/`Spec`/rating-flags, silently breaking rating/metrics/compare/share/dashboard. Plus quota enforcement, topology projection, overview live-id, compile preview, config view, stroppy validation, workload sizing, favorite-recipe.

**Architecture:** Fill run metadata from `CompiledPlan` at launch/execution; remodel quota reserve/commit/release from `machine_groups`; project topology from `CompiledPlan`+provisioned `Machines`; adapt overview to recipe workflow id; add a compile-preview RPC; surface the recipe bundle in run detail; validate stroppy at compile; add DSL sizing + favorite-recipe.

**Tech Stack:** Go (workflows/services/quotas/proto), TS/React (RecipeEditor/RunDetail/favorite).

## Global Constraints

- Module `github.com/stroppy-io/stroppy-cloud`; branch `feat/yaml-dsl-pivot`.
- Commits: Conventional Commits, imperative lowercase, NEVER Co-Authored-By/AI attribution.
- Principle (memory feedback_remodel_not_delete): REMODEL under recipe model, don't restore old InfrastructurePlan/QuotaRequestRef shapes. Quota requests derive from `dslpb.CompiledPlan.machine_groups`; topology from CompiledPlan+Machines.
- Gate: Go tasks `go build ./...`+`go test ./internal/...` green; FE tasks `cd web && npx tsc -b --noEmit` green. Live-stand verify deploy-gated (deferred).
- Owner decisions locked: full quota reserve/commit/release; networks = tf-module's job (stays gone); suite = matrix (stays gone); include ALL gaps (full remodel, not minimal).

---

### Task 1: Run Summary population (root — revives rating/metrics/compare/share/dashboard)

**Files:**
- Modify: `internal/services/recipe/recipe.go` (StartRun sets rating flags + seeds Summary), `internal/workflows/runrecipe.go` (after compile, persist Summary derived from CompiledPlan), `internal/infrastructure/execution/run_persistence.go` (applyRunSummary extend to carry the recipe-derived fields), possibly `protocols/cloud/v1/api/recipe.proto` (StartRunRequest gains rating opt-in) + regen
- Test: recipe_test.go, runrecipe_test.go

**Interfaces:**
- `TestRunRecord.Summary` fields (grep models/test_run.proto Summary: DbKind, DbPresetName, WorkloadName, StroppyVersion, Provider, TopologyLabel, NodeCount, timings) must be populated for recipe runs. Derive from CompiledPlan:
  - `Provider` = plan.GetProvider().GetName().
  - `NodeCount` = sum of machine_groups[].count.
  - `TopologyLabel` = a short string e.g. `"<provider>/<N> nodes"` or derived from group names.
  - `DbKind` = the database engine — heuristic: find the service that is the DB (a service NOT named runner/stroppy; or a convention label on the ServiceSpec; pick the primary DB service's image/name → map to a kind string). Document the heuristic; if ambiguous, leave "" (metrics then defaults postgres — acceptable v1, but prefer a real value: e.g. parse from the primary non-stroppy service name or a `db_kind` label the recipe author sets on the service). SIMPLEST robust: add an optional `db_kind` field a recipe can set on a service (or on cluster), else infer from service name.
  - `WorkloadName` = the stroppy service's workload (from matrix workload values or the stroppy job args).
  - `StroppyVersion` = the stroppy service image tag.
- `InTenantRating`/`InGlobalRating` flags: StartRun sets them (default true, or from a new StartRunRequest field). So recipe runs appear on rating boards.
- Persist: RunRecipeWorkflow's compile stage has the CompiledPlan → call a persist that writes Summary onto the record (extend the PersistRunState activity path or add a small SummaryActivity). run_persistence.applyRunSummary currently only sets timings — extend to accept + write the derived Summary fields.

- [ ] **Step 1: Read models/test_run.proto Summary + monitoring.go dbKindString + rating/compare/share readers** to know exactly which Summary fields each kept feature reads (audit Part B named them).
- [ ] **Step 2: Failing tests** — StartRun sets InTenantRating/InGlobalRating; after RunRecipeWorkflow compile, the record's Summary has Provider/NodeCount/DbKind/WorkloadName/StroppyVersion populated (testsuite: mock persist captures the Summary).
- [ ] **Step 3: Implement** — StartRun rating flags; runrecipe compile-stage Summary derivation from CompiledPlan; run_persistence write. Document the DbKind heuristic (+ optional recipe-author db_kind override).
- [ ] **Step 4: Run — PASS** (recipe + workflows tests). Verify monitoring.dbKindString now returns a real kind for recipe runs (metrics query set correct).
- [ ] **Step 5: Commit** `feat(recipe): populate run summary and rating flags from compiled plan`

---

### Task 2: Quota reserve/commit/release remodel (revives quota page + enforces limits)

**Files:**
- Modify: `internal/infrastructure/quotas/{manager.go,store.go}` (re-add Reserve/Commit/Release, remodeled to take machine_groups), `internal/infrastructure/execution/recipe_activities.go` (ReserveQuotasActivity/CommitQuotasActivity/ReleaseQuotasActivity), `internal/workflows/runrecipe.go` (reserve before Provision → commit after execute → release in teardown), register.go
- Test: quotas + runrecipe tests

**Interfaces:**
- Recover the deleted Store.Reserve/CommitRun/ReleaseRun via `git show efd9bffc^:internal/infrastructure/quotas/store.go` for the reservation row shape + SQL, but REMODEL the input: instead of `[]*QuotaRequestRef` (proto deleted), compute quota requests from `[]*dslpb.MachineGroup`:
  - per group: cpu = group.cpu * group.count; ram = group.ram_mb * group.count; disk = sum(disks.size_gb) * group.count. Aggregate per resource kind (cpu/ram/disk) for the provider.
  - `Manager.Reserve(ctx, tenantID, runID, workflowID string, provider string, groups []*dslpb.MachineGroup) error` — writes quota_reservations rows keyed (tenant, run, resource-kind, amount); fails if a kind exceeds AvailableForRuns.
  - `Manager.Commit(ctx, tenantID, runID)` / `Release(ctx, tenantID, runID)` restored (they were run-id keyed, mostly reusable).
- RunRecipeWorkflow: new infra sub-step — ReserveQuotasActivity(tenant, runID, wfID, provider, plan.machine_groups) BEFORE ProvisionActivity (reserve fails → fail infra stage, don't provision); CommitQuotasActivity after execute succeeds; ReleaseQuotasActivity in the teardown defer (always, so a failed/cancelled run frees its reservation).
- This makes quota_reservations write-live → the quota page's Reserved/Run-free columns + run-usage ledger populate.

- [ ] **Step 1: Recover deleted Reserve/CommitRun/ReleaseRun SQL + row shape** (git show efd9bffc^). Read the current quota table schema + ListQuotaViews join to know the reservation row columns.
- [ ] **Step 2: Failing tests** — Manager.Reserve from machine_groups writes reservation rows + computes per-kind amounts; over-limit → error; Commit/Release; RunRecipeWorkflow reserves before provision, commits after execute, releases on teardown (testsuite mock activities assert order).
- [ ] **Step 3: Implement** — re-add Store methods (remodeled), Manager methods (machine_groups input), activities, workflow hooks.
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(quota): remodel reserve/commit/release from compiled machine groups`

---

### Task 3: Topology projection for recipe runs

**Files:**
- Modify: `internal/workflows/runrecipe.go` (build a topology/machines snapshot from CompiledPlan+provisioned Machines, persist it), `internal/infrastructure/execution/{overview.go,runtime_topology.go}` (project topology from the recipe snapshot when Spec is nil), possibly `protocols/cloud/v1/models/test_run.proto` (a recipe_topology / machines snapshot field) + regen
- Test: overview/runtime_topology tests

**Interfaces:**
- RunRecipeWorkflow after ProvisionActivity has `Machines map[group][]MachineState` + CompiledPlan.machine_groups + services. Build a topology projection: nodes = each machine (id, group, ip, role from services on that group), connections = service dependencies (from workflow needs / service on-group). Persist onto the record (add a field, e.g. `TestRunRecord.recipe_topology` = a monitor.Topology-shaped message, OR reuse/extend RunState to carry machine identity so runtime_topology can build from stages).
- overview.go topologyFromRecord: when rec.Spec == nil but rec has recipe_topology (recipe run), build the overview topology from it (mirror how it built from TopologySpec). runtime_topology.runtimeMachineIDs: source machine ids from the recipe snapshot for recipe runs.
- Decide the carrier (new proto field vs RunState machine stamping). PREFER a dedicated persisted field (deterministic, simplest for overview to read) — add `TestRunRecord.machines` (repeated MachineState) or a compact topology snapshot filled at provision; document.

- [ ] **Step 1: Read overview.topologyFromRecord + runtime_topology model** (RuntimeNode/Connection shape). Decide the carrier field.
- [ ] **Step 2: Failing test** — a recipe run record with machine snapshot → overview topology has the machine nodes (not just control-plane); runtime_topology projects machines/services.
- [ ] **Step 3: Implement** — proto carrier field + regen; runrecipe persists the snapshot after provision; overview/runtime_topology read it for recipe runs.
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(recipe): project run topology from compiled plan and machines`

---

### Task 4: Overview live-query id for recipe runs

**Files:**
- Modify: `internal/infrastructure/execution/overview.go` (try runRecipeWorkflowID for recipe runs)
- Test: overview test

**Interfaces:**
- overview.go:129-147: detect a recipe run (rec.GetRecipeId() != "") → query `runRecipeWorkflowID(runID)` instead of `testWorkflowID(runID)`; keep testWorkflowID for legacy. So a running recipe run gets true live Temporal freshness, not just persisted fallback.
- ids.go: runRecipeWorkflowID helper exists (grep). The GetRunState querier works for RunRecipeWorkflow (it registered GetRunState query — subproject 1D-T3).

- [ ] **Step 1: Failing test** — OverviewReader for a recipe-run record queries the run-recipe workflow id (mock querier asserts the id).
- [ ] **Step 2: Implement** — id selection by recipe_id.
- [ ] **Step 3: Run — PASS**
- [ ] **Step 4: Commit** `fix(overview): query recipe workflow id for recipe runs`

---

### Task 5: Compile/plan preview (RPC + RecipeEditor panel)

**Files:**
- Modify: `protocols/cloud/v1/dsl/service.proto` (Compile/Preview RPC returning CompiledPlan summary) + regen, `internal/services/dsl/service.go` (handler over dsl.CompileBundle), `web/src/services/recipe.ts` (previewBundle), `web/src/pages/RecipeEditor.tsx` (preview panel)
- Test: dsl service test + tsc

**Interfaces:**
- `DslService.Preview(PreviewRequest{files}) → PreviewResponse{compiled: <plan summary>, diagnostics}` — runs dsl.CompileBundle, returns a UI-friendly summary (machine groups w/ counts+specs, services, jobs DAG) + diagnostics. Reuse the traversal-safe CompileBundle.
- RecipeEditor: a "Preview" tab/panel showing the resolved plan (N nodes, services, provider) before Run.

- [ ] **Step 1: proto + regen; failing dsl service test** — Preview with postgres-ha bundle returns a plan summary (machine groups, services); broken bundle returns diagnostics.
- [ ] **Step 2: Implement** handler + recipe.ts previewBundle + RecipeEditor panel.
- [ ] **Step 3: build+test+tsc green.**
- [ ] **Step 4: Commit** `feat(dsl): compile preview rpc and recipe editor plan panel`

---

### Task 6: RunConfigTab — show recipe bundle/version

**Files:**
- Modify: `web/src/components/run/RunConfigTab.tsx` (for recipe runs, show the recipe id/version + a read-only bundle view instead of "No spec available"), `web/src/services/recipe.ts`/run_overview to fetch the run's recipe
- Test: tsc

**Interfaces:**
- RunConfigTab: when run.spec empty but run has recipeId (RunVM.recipeId), fetch the recipe (getRecipe) + render its bundle files read-only (DslEditor readOnly) + version. Link back to the recipe.

- [ ] **Step 1: Implement** — recipeId-driven config view (read-only bundle) replacing the empty-state for recipe runs.
- [ ] **Step 2: tsc green.**
- [ ] **Step 3: Commit** `feat(web): show recipe bundle in run config tab`

---

### Task 7: Stroppy version validation at compile

**Files:**
- Modify: `internal/dsl/...` (compile-time check that the stroppy service's image/version is resolvable) OR `internal/services/dsl` check path
- Test: dsl test

**Interfaces:**
- During compile/check, validate the stroppy service (the workload runner) references a resolvable stroppy version/image — a warning/error diagnostic if the pinned version doesn't exist (reuse stroppy.ListStroppyVersions to validate the tag). Surfaces in the editor diagnostics (fail fast, not at container-pull time).
- Keep minimal: a diagnostic (warning) if the stroppy service image tag isn't in the known versions list. Document it's advisory.

- [ ] **Step 1: Failing test** — a bundle with a bogus stroppy version → a diagnostic; valid → none.
- [ ] **Step 2: Implement** the validation in the check path.
- [ ] **Step 3: Run — PASS.**
- [ ] **Step 4: Commit** `feat(dsl): validate stroppy version at compile time`

---

### Task 8: Workload sizing (DSL) + favorite-recipe + clone-link cleanup

**Files:**
- Modify: DSL (sizing expression support — allow `resources: {cpu: "${{ ... }}"}` CEL in machine group resources, or a `sizing:` helper), `protocols/.../common/favorite.proto` (FAVORITE_KIND_RECIPE) + regen + run.go FavoriteTargetRepos wire recipe repo + web favorite recipe, `web/src/services/runs.ts` (remove dead "clone" action or wire to /recipes/new?from)
- Test: dsl sizing + favorite + tsc

**Interfaces:**
- Sizing: allow CEL in machine-group resources so a recipe can derive runner size from workload params (e.g. `cpu: "${{ 2 + inputs.iterations / 500000 }}"`) — the compiler already evaluates CEL in strings; extend resources parsing to accept CEL-yielding numbers. Document. (If too broad, defer sizing to a follow-up + note — but favorite/clone are cheap, do them.)
- Favorite recipe: add FAVORITE_KIND_RECIPE, wire Recipes repo into FavoriteTargetRepos, add a favorite toggle on the recipe list.
- Clone: wire the dead "clone"/"New from" to `/recipes/new?from=:recipeId` (seed editor from a past run's recipe) OR remove from actionsForStatus.

- [ ] **Step 1: Favorite recipe** — proto kind + repo wire + FE toggle. Test + tsc.
- [ ] **Step 2: Clone link** — wire or remove. tsc.
- [ ] **Step 3: Sizing** — CEL in resources (or defer w/ documented note). Test.
- [ ] **Step 4: build+test+tsc green.**
- [ ] **Step 5: Commit** `feat: workload sizing, favorite recipe, clone link`

---

## Self-Review
- Root (Summary) first → revives rating/metrics/compare/share/dashboard (audit Part B 1-3). Then quotas (Q1/Q2), topology (Q3), overview (Q4), preview/config/validation (Part A 7 + Part B 5), then product gaps (sizing/favorite/clone).
- Remodel not restore: quotas from machine_groups (not QuotaRequestRef), topology from CompiledPlan (not TopologySpec).
- Guided wizard (Part A 2): the RecipeEditor + Preview panel (Task 5) + Config view (Task 6) substantially close the UX gap (see the plan before run, edit with live diagnostics); a full form-driven wizard is deferred as an explicit product decision (documented) — the YAML editor + preview is the v1 authoring UX.
- Deploy-gated: live-stand run verification deferred.
