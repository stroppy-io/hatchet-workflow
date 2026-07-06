# Phase 1E-A: Delete Old Backend Path Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Checkbox steps.

**Goal:** Delete the old test-orchestration Go backend (TestWorkflow stage-machine, per-DB renderers, preset/suite/wizard services, old provisioning) now that the recipe/DSL path exists — WITHOUT touching proto files or frontend (deferred to 1E-B after the new UI lands, to avoid cascade build breaks). Build + remaining tests stay green after every task.

**Architecture:** Surgical removal following the recon deletion map. Shared seams (agentTaskQueue/executeAgentStep, runtime.go persist activities, domain/deployment, settings, overview/monitoring, quotas for QuotaService, terraform/docker exec, models.TestRunRecord + test_run_records) STAY. Old-only code goes. run.go/register.go/seed.go/postgres repos are rewired, not just deleted.

**Tech Stack:** Go 1.25. No proto regen (proto deletion is 1E-B). No frontend changes (1E-B).

## Global Constraints

- Module `github.com/stroppy-io/stroppy-cloud`; branch `feat/yaml-dsl-pivot`.
- Commits: Conventional Commits, imperative lowercase, NEVER Co-Authored-By/AI attribution.
- Spec §5 of `docs/superpowers/specs/2026-07-03-flow-wiring-design.md`.
- **Build-green gate**: after EVERY task, `go build ./...` green + `go test ./internal/...` green (minus deleted tests). A task that leaves the build broken is not done.
- Do NOT touch: any `protocols/**` or `internal/proto/**` (proto stays until 1E-B — the generated Go types for old services remain, just unused/unregistered), any `web/**` (frontend stays until 1E-B — old screens become runtime-404, tsc still compiles since proto TS remains).
- SHARED-KEEP (never delete, recon-confirmed): `internal/workflows/runtime.go`; `deployment.go`'s agentTaskQueue/executeActivityNoResult/executeAgentStep/expectedExitCode; `internal/domain/deployment/*`; `internal/domain/settings/*`; `internal/infrastructure/execution/{overview,monitoring,run_persistence,agent_registry,agent_logs,recipe_activities,recipe_workflows}.go`; `internal/infrastructure/quotas/*` (QuotaService); `internal/infrastructure/{terraform,docker}`; `internal/services/{test_run_overview,quota,compare,share,favorite,packages,iam,system_settings,tenant_settings,tenant_dashboard,stroppy,rating,public_rating,public_share,agent_shell,dsl,recipe}`; `models.TestRunRecord` + `test_run_records` table; `workflow.RunState/Stage/StageUpdate`.
- golangci: no NEW issues in touched files.

---

### Task 1: Extract shared agent-exec helpers out of deployment.go

**Files:**
- Create: `internal/workflows/agent_exec.go`
- Modify: `internal/workflows/deployment.go` (remove the extracted funcs)

**Interfaces:**
- Move (verbatim, no behavior change) from deployment.go to agent_exec.go: `agentTaskQueue` (~line 532), `executeActivityNoResult` (~630), `executeAgentStep` (~607), `expectedExitCode` (~634) — these are the ONLY deployment.go symbols dslrun.go/runrecipe.go still use. Leave the rest of deployment.go in place for now (Task 2 deletes it).

- [ ] **Step 1: Move the 4 funcs** to agent_exec.go (same package `workflows`, keep imports). deployment.go loses them.
- [ ] **Step 2: Build** — `go build ./internal/workflows/...` green (dslrun.go/runrecipe.go now reference agent_exec.go's copies).
- [ ] **Step 3: Test** — `go test ./internal/workflows/... ` green (dslrun/runrecipe tests unaffected).
- [ ] **Step 4: Commit** `refactor(workflows): extract agent exec helpers from deployment`

---

### Task 2: Delete old workflow files + deployment activities

**Files:**
- Delete: `internal/workflows/test.go` (+ test), `internal/workflows/suite.go` (+ test), `internal/workflows/provider_render.go` (+ test), `internal/workflows/deployment.go` (+ test — now that agent_exec.go has the shared bits), `internal/workflows/deployment_activities.go` (+ test), `internal/workflows/stages.go` IF old-only (grep: does runrecipe.go use stages.go helpers? recon said runrecipe emits dynamic stages — check; if runrecipe uses stages.go's startRootStage/etc., KEEP stages.go)
- Modify: `internal/workflows/register.go` — strip `Options`/`DefaultOptions`/`normalizeOptions`, the old `RegisterDeploymentServiceWorkflows`/`RegisterTestServiceWorkflows`/`RegisterSuiteWorkflowServiceWorkflows` calls + `NewDeploymentWorkflows`/`NewTestWorkflows`/`NewSuiteWorkflows`, `RegisterDeploymentServiceActivities`/`NewDeploymentActivities` call + `ActivityOptions.Quotas/Networks/Logs` fields. KEEP: ExecuteCompiledPlanWorkflow + RunRecipeWorkflow registration, RuntimeActivities interface + 4 runtime.* registrations, RecipeActivityImpl + RegisterRecipeActivities.

**Interfaces:**
- Consumes: agent_exec.go (Task 1). register.go's RegisterWorkflows/RegisterActivities signatures may change (drop options param) — update the run.go call site accordingly (or keep signatures, pass empty). Prefer simplifying: RegisterWorkflows(registry) + RegisterActivities(registry, runtime).

- [ ] **Step 1: Delete the files + strip register.go.** Fix register.go's RegisterWorkflows/RegisterActivities to drop old registrations + the Options/ActivityOptions old fields.
- [ ] **Step 2: Build** — `go build ./internal/workflows/...` will fail at run.go later; first make the workflows package itself compile (register.go references only kept symbols). Then `go build ./...` will surface run.go breakage — that's Task 6. For THIS task, get `go vet ./internal/workflows/` clean and note run.go/register.go callers to fix in Task 6. If register.go signature changed, the ONLY external caller is run.go (Task 6) — leave a compile break there to fix in Task 6, OR keep signatures stable to build incrementally. DECISION: keep RegisterWorkflows(registry, options) signature but make Options optional/ignored, so run.go still builds until Task 6 cleans it. Simplest: RegisterWorkflows(registry) new signature, and DO Task 6's run.go edit in the same task if the break blocks build. Pragmatic: merge register.go + run.go worker-section edits so build stays green — if that means touching run.go here, do it (small, targeted: the worker registration lines).
- [ ] **Step 3: Build green** — `go build ./...` (touching run.go worker section minimally if needed to keep it compiling — full run.go service cleanup is Task 6).
- [ ] **Step 4: Test** — remaining workflows tests green.
- [ ] **Step 5: Commit** `chore(workflows): delete old test/suite/deployment workflows`

---

### Task 3: Delete old domain packages

**Files:**
- Delete: `internal/domain/database/*` (all DB subdirs + builder.go), `internal/domain/run/*`, `internal/domain/infrastructure/*`, `internal/domain/workload/*`, `internal/infrastructure/networks/*`
- Prune (NOT delete): `internal/domain/deployment/builder.go` — if it imports domain/packages/topology only for the old Registry/renderer machinery, prune those symbols; KEEP the package (dslrun uses ShellQuote, overview/log_sink/runtime_topology import it). Grep what dslrun/overview actually use from domain/deployment and keep exactly that.

**Interfaces:**
- After deletion, the importers recon flagged (all old-only: presets_seed.go, suite_wizard_engine.go, test_wizard_engine.go, test_workflows.go, suite_launcher.go, domain/run/builder.go) are ALSO being deleted (Task 4/5) — so deletion order matters. This task deletes the LEAF domain packages; if a still-present file imports them, that file is old-only and deleted in Task 4/5 — but build won't be green until those are gone too. So: do Tasks 3+4+5 as a COORDINATED delete (they're interdependent), committing when `go build ./...` is green after all three. RESTRUCTURE: merge Tasks 3-5 into one "delete old domain + execution + services" task with a single build-green gate, OR delete in dependency order (services → execution → domain) so each step builds. PREFER dependency order: delete top-of-stack first.

- [ ] **Step 1: Reorder** — delete in reverse-dependency order: first the services (Task 5 content), then execution adapters (Task 5), then domain leaves (this task). Follow recon's removal order 5→4. This task = the domain-leaf deletion, done AFTER Task 5. (Renumber mentally: execute Task 5 before Task 3 if build requires. The controller sequences to keep build green.)
- [ ] **Step 2: Delete domain leaves** once no importer remains.
- [ ] **Step 3: Build green** `go build ./...`
- [ ] **Step 4: Commit** `chore(domain): delete old database/run/infrastructure/workload packages`

---

### Task 4: Delete old execution adapters + services

**Files:**
- Delete: `internal/infrastructure/execution/{test_workflows,suite_launcher,suite_canceller,test_wizard_engine}.go` (+ tests), `internal/infrastructure/adapters/suite_wizard_engine.go` (+ test), `internal/services/{test_run,test_wizard,suite,suite_run,suite_wizard,preset}/` (whole dirs + tests)
- KEEP: `internal/services/test_run_overview` (shared — recipe runs use it), compare, quota, all the SHARED-KEEP services.

- [ ] **Step 1: Delete** the old-only execution adapters + service dirs. This must precede domain deletion (Task 3) since these import domain/*.
- [ ] **Step 2: Fix run.go** — remove construction + registration of the deleted services (databasePreset/workloadPreset/testPreset/testRun/testWizard/suite/suiteRun/suiteWizard services): their `register(mux,...)` entries, gqlapi.Server fields, ogenAdapter args. This is the bulk of run.go cleanup — coordinate with Task 6. (Controller may merge run.go edits into one pass.)
- [ ] **Step 3: Build green** `go build ./...`
- [ ] **Step 4: Test** remaining green.
- [ ] **Step 5: Commit** `chore(services): delete old test-run/suite/preset/wizard services`

---

### Task 5: Rewrite postgres repos + seed (drop old repos, keep shared)

**Files:**
- Modify: `internal/infrastructure/postgres/records.go` (delete SuiteRepo/SuiteRunRepo/SuiteDraftRepo/DatabasePresetRepo/WorkloadPresetRepo/TestPresetRepo; keep ShareRepo/FavoriteRepo/PackageRepo/TenantSettingsRepo), `facets.go` (delete SuiteRepo.ListFacets, keep TestRunRepo.ListFacets + shared helpers), `list_helpers.go` (delete suite/preset filters+sorts+summary helpers incl the workloadbuilder import; keep generic + share/package/draft filters), `store.go` (drop Drafts()/Presets()/suite/preset accessors, keep TestRuns()/generic)
- Modify: `internal/app/seed.go` (remove seedBuiltinCatalog call + chain), Delete: `internal/app/presets_seed.go` (+ test)

- [ ] **Step 1: Prune postgres repos** — remove old repo types/accessors, keep TestRunRepo + Share/Favorite/Package/TenantSettings. Remove the workloadbuilder import in list_helpers.go by deleting the preset-summary helpers.
- [ ] **Step 2: Prune seed** — delete presets_seed.go, remove seedBuiltinCatalog from seed.go (keep seedFirstBoot tenant/account/settings bootstrap).
- [ ] **Step 3: Build green** `go build ./...`
- [ ] **Step 4: Test** postgres + app tests green (minus deleted).
- [ ] **Step 5: Commit** `chore(postgres): drop old preset/suite repos and seed`

---

### Task 6: Final run.go + glue.go cleanup + verify

**Files:**
- Modify: `internal/app/run.go` (remove any remaining old-service construction/registration/wiring not yet cleaned; drop networkStore/networkManager, cells/suiteLauncher/suiteCanceller, ActivityOptions Quotas/Networks), `internal/app/glue.go` (remove old wizard/suite bindings)

- [ ] **Step 1: Final sweep** — grep run.go + glue.go for any reference to deleted packages/services; remove. Simplify RegisterActivities/RegisterWorkflows call to the new signatures. Keep quotaService (unrelated feature), all SHARED-KEEP services, recipe/dsl services, recipe worker registration.
- [ ] **Step 2: Full build + test** — `go build ./...` green; `go test ./internal/...` green; `go vet ./...` (only pre-existing issues); `golangci-lint run ./internal/app/...` no new issues.
- [ ] **Step 3: Grep sanity** — `grep -rl "domain/database\|domain/run\b\|services/preset\|services/suite\b\|presets_seed\|test_wizard" internal/ --include=*.go` returns only expected residue (or nothing). Confirm no dangling imports.
- [ ] **Step 4: Commit** `chore(app): finish old-path wiring cleanup`

---

## Self-Review
- Spec §5 backend deletion covered; proto + frontend deletion explicitly deferred to 1E-B (avoids cascade breaks — old proto Go types stay unused, old web screens become runtime-404 with tsc still green).
- SHARED-KEEP list enforced per recon; models.TestRunRecord + test_run_records + overview/monitoring/quota kept.
- Build-green gate after every task; deletion in dependency order (services→execution→domain, then postgres/seed, then final run.go). Controller sequences Tasks 4/5 before 3 where build requires.
- Deferred to 1E-B: old .proto deletion + workflow/test.proto surgical split (keep RunState/Stage/StageUpdate) + old table drop migration + all frontend removal.
- Dead-code follow-up (non-blocking): quotas Reserve/Commit/Release become unused after deployment_activities deletion — prune in 1E-B or leave.
