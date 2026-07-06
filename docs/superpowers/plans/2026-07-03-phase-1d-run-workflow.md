# Phase 1D: RunRecipeWorkflow + Binding Parity (I1) + Launch Wiring + RunState Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Checkbox steps.

**Goal:** Wire the dormant DSL engine into a live run: a `RunRecipeWorkflow` that compiles a stored recipe, provisions infra via the provider factory, bootstraps Nomad, runs `ExecuteCompiledPlanWorkflow`, and tears down — emitting `RunState` so the existing status/logs/metrics/overview readers work, launched from a `RecipeRunService`. Also close review finding I1 (compile-time typecheck env ⊃ runtime bindings) by threading resolved inputs/target into `CompiledJob`.

**Architecture:** New wrapper workflow reuses the generic persistence activities (`PersistRunState`/`AppendRunLogs`) + exposes a `GetRunState` query (overview compatibility). Provisioning reuses quota/network children where useful + the provider factory (phase 1B) + Nomad role assignment (phase 1C). I1: `CompiledJob` gains `resolved_inputs`/`input_groups`/`target_group`; lower fills them from `BoundComponent`; the interpreter binds the same `{inputs, target, machine, machines, matrix}` the checker validated. Real infra runs are deploy-gated; this phase is unit/testsuite-tested with mocked activities.

**Tech Stack:** Go 1.25, Temporal (go.temporal.io/sdk + testsuite), internal/dsl compiler+lower, internal/workflows, internal/infrastructure/{provider,execution}, internal/services.

## Global Constraints

- Module `github.com/stroppy-io/stroppy-cloud`; branch `feat/yaml-dsl-pivot`.
- Commits: Conventional Commits, imperative lowercase, NEVER Co-Authored-By/AI attribution.
- Spec §4.3/§4.5 of `docs/superpowers/specs/2026-07-03-flow-wiring-design.md`.
- No deploy: RunRecipeWorkflow tested via `testsuite.WorkflowTestSuite` with mocked activities/child-workflows; no live Temporal server, provider, or Nomad. Live run is a deploy-gated follow-up.
- Reuse (don't rewrite): `PersistRunState`/`AppendRunLogs`/`PersistDeploymentPlan` activities + `RuntimeActivities` port (`internal/workflows/runtime.go`), `MetricsReader`/log readers, `projectOverviewWithSource` generic projection.
- Temporal determinism: no map-iteration-driven control flow, no time.Now/rand/native goroutines in workflow code (mirror ExecuteCompiledPlanWorkflow's discipline).
- Old test.go stage-machine stays until phase 1E — do NOT delete here; add the new path alongside.
- golangci clean. No bare `go mod tidy`.

---

### Task 1: I1 — resolved inputs/target in CompiledJob (proto + lower)

**Files:**
- Modify: `protocols/cloud/v1/dsl/compiled.proto` (CompiledJob fields)
- Regenerate: `internal/proto/cloud/v1/dsl/*` via `make protocols` (revert unrelated TS drift)
- Modify: `internal/dsl/lower/lower.go` (fill new fields from BoundComponent/resolved job)
- Modify: `internal/dsl/testdata/postgres-ha.golden.json` (regen with -update)
- Test: `internal/dsl/lower/lower_test.go`, `internal/dsl/golden_test.go`

**Interfaces:**
- Produces (proto CompiledJob additions):

```proto
message CompiledJob {
    // ... existing id/needs/on_group/matrix/when/action/with ...
    // resolved_inputs are the component's scalar inputs (int/string/bool) resolved
    // at compile time, bound as CEL `inputs.<name>` at runtime. Values are the
    // stringified scalar; the runtime rebinds them typed via the component's InputSpec if needed (v1: string-typed dyn).
    map<string, string> resolved_inputs = 7;
    // input_groups maps a machine_group-typed input name → the cluster machine
    // group name it was bound to, so the runtime can bind `inputs.<name>` to that
    // group's MachineGroupView.
    map<string, string> input_groups = 8;
    // target_group is the single machine_group input's group name (the CEL `target`
    // binding), empty when the component has zero or multiple machine_group inputs.
    string target_group = 9;
}
```

- lower fills these from the `include.BoundComponent` that produced each job (the job carries which component instantiated it via its prefix; lower has access to the resolved components — thread `[]BoundComponent` into Lower, or attach per-job during expansion). READ how lower currently maps jobs; the BoundComponent.Inputs (map[string]any, machine_group values are group-name strings) is the source. Scalars → resolved_inputs; machine_group inputs → input_groups; the sole machine_group input → target_group (empty if 0/many).

- [ ] **Step 1: proto edit + `make protocols`** — CompiledJob gets fields 7/8/9. Regen clean, revert unrelated TS drift. `go build ./...` green.

- [ ] **Step 2: Failing lower test** — a bundle with a component whose job cmd uses `${{ inputs.nodes.count }}` (nodes = machine_group input bound to group "db") + a scalar input → after Compile, the CompiledJob for that job has `input_groups["nodes"]=="db"`, `target_group=="db"`, `resolved_inputs` carrying the scalar. RED (lower doesn't set them yet).

- [ ] **Step 3: Run — FAIL**
- [ ] **Step 4: Implement** lower filling — thread BoundComponent data to the per-job lowering. Update golden test with `-update`, verify stable (run twice).
- [ ] **Step 5: Run — PASS** (`go test ./internal/dsl/... -v`)
- [ ] **Step 6: Commit** `feat(dsl): thread resolved inputs and target into compiled jobs (I1 groundwork)`

---

### Task 2: I1 — interpreter binds inputs/target matching the checker

**Files:**
- Modify: `internal/workflows/dslrun.go` (`baseJobVars` / binding builders bind inputs/target from the new CompiledJob fields)
- Test: `internal/workflows/dslrun_test.go`

**Interfaces:**
- Consumes: CompiledJob.resolved_inputs/input_groups/target_group (Task 1), `expr.MachineGroupView`, Machines map.
- Produces: `baseJobVars` now binds `inputs` (a map: scalar names → string values, group-input names → the group's MachineGroupView built from Machines), `target` (the target_group's MachineGroupView, unset when empty), in addition to the existing `machines`/`matrix`. So `${{ inputs.nodes.count }}`, `${{ inputs.iterations }}`, `${{ target.count }}` all resolve at runtime — matching what `graph.Validate` typechecked against ComponentEnv (closing I1).

- [ ] **Step 1: Failing test** — ExecuteCompiledPlanInput with a plan whose steps-job cmd is `echo ${{ inputs.nodes.machines[0].ip }}` (inputs.nodes → input_group "db", Machines["db"] has an IP) + a `when: inputs.count > 0` with resolved_inputs.count="3"; mocked CallCmd captures the resolved script; assert it contains the real IP and the when-guarded job ran. RED (baseJobVars doesn't bind inputs/target).
- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Implement** — extend baseJobVars/withMachineBinding to add inputs+target bindings from the CompiledJob fields (build group views via the existing machineViewFor/group-view helper). Update the dslrun package doc (it currently says "only machines and matrix bound" — correct it to include inputs/target).
- [ ] **Step 4: Run — PASS**. Add a negative-ish test: a plan referencing `${{ inputs.missing }}` where resolved_inputs lacks it → the eval errors gracefully (job failed, not workflow panic) — confirm the interpreter's existing when/eval error path handles it.
- [ ] **Step 5: Commit** `feat(workflows): bind inputs and target in interpreter (closes I1)`

---

### Task 3: RunRecipeWorkflow — provision → bootstrap → execute → teardown + RunState

**Files:**
- Create: `internal/workflows/runrecipe.go`
- Modify: `internal/workflows/register.go` (register RunRecipeWorkflow)
- Test: `internal/workflows/runrecipe_test.go`

**Interfaces:**
- Consumes: `dsl.Compile` (compile the bundle — but compile is pure Go, run it in an activity to keep the workflow deterministic/small OR pass the already-compiled plan in — DECIDE: compile in a `CompileRecipeActivity` so the workflow input is the recipe bundle + provider, not a pre-compiled plan; document), provider factory (phase 1B) via a `ProvisionActivity`, Nomad role assignment (phase 1C), `ExecuteCompiledPlanWorkflow` (child), quota/network children (reuse if useful), persistence activities.
- Produces:

```go
type RunRecipeInput struct {
	RunID      string
	TenantID   string
	Bundle     map[string][]byte   // recipe files (from RecipeRecord)
	ProviderManifest ...            // or resolved inside via bundle providers/**
	Bootstrap  *workflowpb.AgentBootstrap
}
func RunRecipeWorkflow(ctx workflow.Context, in *RunRecipeInput) (*RunRecipeOutput, error)
```

- Stages (each emits a `*workflowpb.RunState` Stage via the persist activity, mirroring how domainTestWorkflow.persist works — READ test.go persist + RunState/Stage shape):
  1. `compile` — CompileRecipeActivity(bundle) → CompiledPlan (+ diagnostics → fail stage on errors).
  2. `infra` — ProvisionActivity(plan.machine_groups, providerRef) → Machines map (+ quotas/network if the provider needs them — reuse CalculateQuotasWorkflowChild only for yandex; docker skips). Pick gateway node (runner group's node), AssignNomadRoles, attach tokens/queues (reuse attachAgentTokens).
  3. `bootstrap` — the provider's Provision already bootstrapped agents via cloud-init/docker; Nomad comes up via the bootstrap. A `WaitNomadReadyActivity` (or fold into execute) may poll the gateway nomad — v1: best-effort, document.
  4. `execute` — ExecuteCompiledPlanWorkflow child with (plan, Machines, Bootstrap, gatewayNodeID).
  5. defer `teardown` — provider Destroy + release quotas/network (reuse teardown activities).
- Expose `GetRunState` query returning the accumulated `*workflowpb.RunState` (so overview.go's runStateQuerier works — use the SAME query name the overview reader expects; read overview.go for GetRunStateQueryName). Cancel handling: on ctx cancel, run teardown via defer.

- [ ] **Step 1: Failing test** — `testsuite.WorkflowTestSuite`, mock CompileRecipeActivity→a small CompiledPlan, ProvisionActivity→Machines, mock ExecuteCompiledPlanWorkflow child→success, mock persist/teardown activities. Assert: stages run in order (compile→infra→execute), teardown runs, RunState query returns stages with statuses, ExecuteCompiledPlanWorkflow child invoked with the provisioned Machines. Second test: ProvisionActivity fails → execute NOT run, teardown runs, RunState shows infra failed. RED (workflow doesn't exist).
- [ ] **Step 2: Run — FAIL** (`go test ./internal/workflows/ -run RunRecipe -v`)
- [ ] **Step 3: Implement** RunRecipeWorkflow + register. Study domainTestWorkflow for the RunState/persist/query pattern; mirror it (do not couple to the 5 fixed stage names — emit dynamic stages named compile/infra/bootstrap/execute/teardown).
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(workflows): RunRecipeWorkflow orchestrating compile/provision/execute/teardown`

---

### Task 4: Provision + compile activities

**Files:**
- Create: `internal/infrastructure/execution/recipe_activities.go` (CompileRecipeActivity, ProvisionActivity, teardown)
- Modify: `internal/workflows/register.go` (register the new activities)
- Test: `internal/infrastructure/execution/recipe_activities_test.go`

**Interfaces:**
- Consumes: `dsl.Compile`, provider factory `NewProviderForRef` + real executor Deps (constructed in app, injected), `deploymentpb.MachineState`.
- Produces:

```go
type RecipeActivities struct { /* provider Deps, blobstore, etc. */ }
func (a *RecipeActivities) CompileRecipeActivity(ctx, in *CompileRecipeInput) (*CompileRecipeOutput, error) // bundle → CompiledPlan + diagnostics
func (a *RecipeActivities) ProvisionActivity(ctx, in *ProvisionInput) (*ProvisionOutput, error)             // plan groups + providerRef → Machines map
func (a *RecipeActivities) TeardownActivity(ctx, in *TeardownInput) error                                    // provider.Destroy(ref)
```

- CompileRecipeActivity wraps `dsl.Compile(Input{Sources, Provider, Composed})` — resolve provider manifest from the bundle's `providers/**` (reuse the dsl service's provider resolution — the traversal-safe path). Diagnostics with errors → return them so the workflow fails the compile stage.
- ProvisionActivity: build the Provider via `NewProviderForRef` + real Deps, call Provision, marshal Machines.

- [ ] **Step 1: Failing tests** — CompileRecipeActivity with the postgres-ha bundle → non-nil CompiledPlan, no error-diagnostics; with a broken bundle → diagnostics returned. ProvisionActivity with a fake provider (inject a fake Provider or fake exec Deps) → Machines map returned. RED.
- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Implement** (inject provider Deps so tests use fakes, real executors wired in app Task 6).
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(execution): recipe compile and provision activities`

---

### Task 5: RecipeRunService launch + RecipeWorkflows port + overview generic

**Files:**
- Create: `internal/services/recipe_run/` (service) OR extend recipe service with a StartRun RPC — DECIDE (new proto RunRecipeRequest or reuse). Simplest: add `StartRun` to RecipeService (proto) taking recipe id + tenant → creates a run record + launches. Read the existing api/recipe.proto; add the RPC + regen.
- Create: `internal/infrastructure/execution/recipe_workflows.go` (RecipeWorkflows port: LaunchRecipeRun)
- Modify: `internal/infrastructure/execution/overview.go` (pipelineFromRecord → generic projection from RunState.Stages)
- Test: service + overview tests

**Interfaces:**
- Produces:
  - `RecipeService.StartRun(ctx, {tenant_id, recipe_id}) → {run: TestRunRecord}` — persist a run record (reuse TestRunRecord or a RecipeRunRecord — reuse TestRunRecord's status/RunState machinery so overview/metrics/logs work unchanged; the run record just needs id/tenant/status/RunState), then `RecipeWorkflows.LaunchRecipeRun(ctx, runRec, bundle, bootstrap)` which does `client.ExecuteWorkflow(RunRecipeWorkflow, ...)` with workflow id `run-recipe/<runID>`.
  - overview.go: replace the hardcoded 5-stage `pipelineFromRecord` fallback with a generic projection from the persisted `RunState.Stages` (the live path `projectOverviewWithSource` is already generic — reuse it for the record fallback so a DAG-shaped RunState renders correctly).

- [ ] **Step 1: proto StartRun + regen** (if adding RPC). Failing service test: StartRun persists a run record (fake repo) + calls a fake RecipeWorkflows.LaunchRecipeRun. RED.
- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Implement** service handler + RecipeWorkflows port impl (Temporal client ExecuteWorkflow, mockable via an interface) + overview generic projection. overview test: a RunState with dynamic stages (compile/infra/execute) → overview projects them generically (not forced into 5 fixed nodes).
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(services): recipe run launch and generic overview projection`

---

### Task 6: App wiring — real executors + worker registration

**Files:**
- Modify: `internal/app/run.go` (construct RecipeActivities with real terraform.Actor + docker.Executor via the phase-1B adapters + factory Deps; wire RecipeWorkflows into RecipeService; register RunRecipeWorkflow + recipe activities on the worker)

**Interfaces:**
- Consumes: everything above + real `terraform.NewActor`, `docker.NewExecutor`, `provider.NewTerraformActorExec`/`NewDockerExecutorExec`/`NewProviderForRef`, `yandextf.EmbeddedTfFiles`.

- [ ] **Step 1: Wire** — build the real executors (grep how the OLD path built docker/terraform executors in run.go — reuse that construction), assemble `provider.Deps{DockerExec: NewDockerExecutorExec(dockerExec), Actor: tfActor, Env: providerEnv, ModuleDir: yandexResolver}`, construct `RecipeActivities`, register `RunRecipeWorkflow` + activities in the worker section (near `workflows.RegisterWorkflows`), wire `RecipeWorkflows` into the recipe service. The ModuleDir resolver: builtin "yandex" → EmbeddedTfFiles; document recipe-supplied modules as future.
- [ ] **Step 2: Build + tests** — `go build ./...` green; `go test ./internal/...` green. No live run (deploy-gated).
- [ ] **Step 3: Commit** `feat(app): wire recipe run workflow with real executors`

---

## Self-Review
- Spec §4.3 (I1) → Tasks 1-2; §4.5 (RunRecipeWorkflow + RunState + launch + overview) → Tasks 3-6.
- I1 closed: proto carries resolved inputs/target (T1), interpreter binds them matching the checker (T2), locked by a runtime test resolving `${{ inputs.* }}`/`${{ target.* }}`.
- Deploy-gated: real provision/nomad/execute run only on a real stand — all tasks tested via mocks/testsuite/fakes.
- Carryover gaps addressed or re-flagged: docker meta-constraint (1C gap) — RunRecipeWorkflow picks gateway NodeRefs; if docker single-node scheduling still needs a jobspec relaxation, flag in T3/T4 and handle or defer to a follow-up (document, don't silently ship non-scheduling docker).
- Old path untouched (1E deletes it). No placeholders: proto fields, activity signatures, workflow stages concrete. Types consistent across tasks.
