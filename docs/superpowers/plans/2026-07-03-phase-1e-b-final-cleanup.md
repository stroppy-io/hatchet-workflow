# Phase 1E-B: Final Cleanup (proto + tables + dead code) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Checkbox steps.

**Goal:** Delete the old proto (api/{test,test_run,suite*,preset,test_wizard}, models/{preset,suite,*wizard}), surgically split workflow/{test,deployment}.proto (keep RunState/Stage/StageUpdate/AgentBootstrap), drop the 8 orphaned tables, and remove residual dead code — reaching a clean prod state. `go build`/`go test`/`tsc` green throughout.

**Architecture:** Ordered removal per recon (FE surgery → Go dead-code → suite-proto refactor → proto delete+split+regen+run.go → migration). Each step keeps build/tsc green.

**Tech Stack:** Go + easyp proto regen; React/TS (tsc); postgres migration + sqld regen.

## Global Constraints

- Module `github.com/stroppy-io/stroppy-cloud`; branch `feat/yaml-dsl-pivot`.
- Commits: Conventional Commits, imperative lowercase, NEVER Co-Authored-By/AI attribution.
- **Gate**: after each task, `go build ./...` + `go test ./internal/...` green (Go tasks); `cd web && npx tsc -b --noEmit` green (FE tasks).
- KEEP (never touch): models/test_run.proto (TestRunRecord), test_run_overview.proto + tenant_dashboard.proto (kept services — but SUITE fields inside them are refactored in Task 3), workflow RunState/Stage/StageUpdate/AgentBootstrap/GetRunState, recipe/dsl/quota(view)/compare/share/favorite/package/iam/settings, test_run_records+quota+recipe+iam tables.
- Proto regen reverts unrelated TS drift; stage only intended.
- No deploy.

---

### Task 1: Frontend surgery — remove testRunClient / extractToPreset / getRunRecord

**Files:**
- Modify: `web/src/services/run_overview.ts` (delete dead getRunRecord + testRunClient import), `web/src/pages/RunDetail.tsx` (remove "Save preset"/extract action + getRunsProvider import), `web/src/services/runs.ts` (prune dead RunsProvider/realRunsProvider/getRunsProvider/setRunsProvider/testRunClient import + dead consts/types; KEEP RunVM/WorkloadSegmentVM/testRunRecordToVM/actionsForStatus/RunAction), `web/src/services/client.ts` (remove testRunClient export)

**Interfaces:** After this, no kept web/src file imports TestRunService (api/test_run_pb.ts) → api/test_run.proto becomes deletable (Task 4). Per recon §6.

- [ ] **Step 1: run_overview.ts** — delete `getRunRecord` (dead, no caller) + drop testRunClient from imports. Keep testRunRecordToVM/RunVM.
- [ ] **Step 2: RunDetail.tsx** — remove the "Save preset" (extract) action: the Save-icon button (`allowed.has("extract")` branch), its handler (getRunsProvider().extractToPreset), the getRunsProvider import; remove `"extract"` from RunAction/actionsForStatus allow-table in runs.ts.
- [ ] **Step 3: runs.ts** — delete realRunsProvider (6 methods), RunsProvider interface, getRunsProvider/setRunsProvider, runSort, testRunClient/favoriteClient/ListTestRunsRequest_Sort_Kind imports, dead consts DB_KINDS/PROTOCOLS/TRIGGERS/RUN_STATUSES, dead types RunsQuery/RunsPage/RunFacets/SortField. KEEP RunVM/WorkloadSegmentVM/testRunRecordToVM/actionsForStatus/RunAction(minus extract) + local type aliases DbKind/Protocol/RunTrigger/DeployProvider (RunVM fields need them).
- [ ] **Step 4: client.ts** — remove testRunClient export + TestRunService import (now zero importers).
- [ ] **Step 5: tsc** — `cd web && npx tsc -b --noEmit` green.
- [ ] **Step 6: Commit** `chore(web): remove dead testRunClient and extract-to-preset`

---

### Task 2: Go dead-code prune (prober, quota reservation chain)

**Files:**
- Delete: `internal/infrastructure/execution/prober.go`
- Modify: `internal/infrastructure/quotas/{manager.go,store.go}` (delete Reserve/Commit/Release + their private helpers), `internal/domain/deployment/infrastructure_outputs.go` (delete QuotaRequestRefOutputs/QuotaAllocationRefOutputs)

**Interfaces:** Removing the quota reservation chain frees QuotaRequestRef/QuotaAllocationRef proto messages (dropped in Task 4). QuotaService's ListQuotas/RefreshQuotas/RunUsage + StartRefresher stay. Per recon §5.

- [ ] **Step 1: Delete prober.go** (zero importers, recon-confirmed).
- [ ] **Step 2: quotas** — delete Manager.Reserve/Commit/Release (manager.go:117,173,181) + quotaNames/servicesFromRequests/aggregateRequests + Store.Reserve/CommitRun/ReleaseRun/ReservationsToAllocationRefs (store.go). Keep ListQuotas/RefreshQuotas/RunUsage/StartRefresher.
- [ ] **Step 3: infrastructure_outputs.go** — delete QuotaRequestRefOutputs/QuotaAllocationRefOutputs (zero callers).
- [ ] **Step 4: build+test** — `go build ./...` + `go test ./internal/...` green.
- [ ] **Step 5: Commit** `chore: remove dead prober and quota reservation code`

---

### Task 3: Suite-proto refactor (free models/suite.proto)

**Files:**
- Modify: `protocols/cloud/v1/api/test_run_overview.proto` (drop TestRunOverviewSnapshot.suite_run field), `protocols/cloud/v1/api/tenant_dashboard.proto` (drop TenantDashboard.recent_suite_runs field), regen
- Modify: `internal/infrastructure/execution/overview.go` (drop SnapshotRunReader.SuiteRun/SaveSuiteRun/SuiteRecord/SaveSuiteRecord from the interface + the suite-run branch lines ~112-117), `internal/infrastructure/adapters/{share.go,tenant_dashboard.go}` (drop GetSuiteRun / ListTenantSuiteRuns; ListScheduledSuites → return a small non-proto struct {ID,Name,Cron,NextRunAt} instead of []*models.SuiteRecord), `internal/app/glue.go` (drop the now-unused suite stubs), `web/src/services/run_overview.ts` (drop the snap.suiteRun mapping line ~429; keep suiteRunId string from record.suite_run_id)

**Interfaces:** After this, models/suite.proto has zero kept references → deletable (Task 4). The still-live "Upcoming suites" dashboard feature keeps working via UpcomingSuite (self-contained) + the new non-proto ListScheduledSuites struct. Per recon §1 (the models/suite.proto trap).

- [ ] **Step 1: proto** — drop suite_run field from TestRunOverviewSnapshot, recent_suite_runs from TenantDashboard. Regen. (suiteRunId string stays — it's plain, no suite type.)
- [ ] **Step 2: Go** — prune the SnapshotRunReader/DashboardRunsReader/ShareRunReader suite methods + overview.go suite branch + glue.go stubs; ListScheduledSuites returns a non-proto struct (define it in the reader's package). Keep UpcomingSuite dashboard path.
- [ ] **Step 3: FE** — run_overview.ts drop snap.suiteRun mapping (keep suiteRunId from record.suite_run_id).
- [ ] **Step 4: build+test+tsc** — `go build ./...` + `go test ./internal/...` + `cd web && npx tsc -b --noEmit` green.
- [ ] **Step 5: Commit** `refactor: remove suite record refs from kept overview/dashboard`

---

### Task 4: Proto deletion + split + regen + run.go

**Files:**
- Delete: `protocols/cloud/v1/api/{test,test_run,test_wizard,suite,suite_run,suite_wizard,preset}.proto`, `protocols/cloud/v1/models/{preset,suite,test_wizard,suite_wizard}.proto`
- Split: `protocols/cloud/v1/workflow/test.proto` (keep RunState/Stage/StageUpdate + a GetRunState/UpdateStage-only service; cut TestWorkflow*/InstallStroppy*/InstallDatabase*/RunWorkload* messages+rpcs + entire SuiteWorkflowService), `protocols/cloud/v1/workflow/deployment.proto` (move AgentBootstrap into a new `workflow/agent.proto`; delete DeploymentService + all quota/network/docker/terraform activity+workflow messages + QuotaRequestRef/QuotaAllocationRef)
- Regen: `make protocols` → deletes generated Go/TS for removed services; NewOgenAdapter arity changes
- Modify: `internal/app/run.go` (drop the 8 nil positional args from api.NewOgenAdapter per recon §3: databasePreset/workloadPreset/testPreset/suite/suiteRun/suiteWizard/testRun/testWizard slots)

**Interfaces:** AgentBootstrap stays package cloud.v1.workflow (workflowpb) — moving its .proto file needs NO Go change (dslrun/agent_exec use workflowpb.AgentBootstrap regardless of file). run.go NewOgenAdapter is the ONE mandatory Go edit (positional arity). gqlapi.Server{} literal uses named fields → no edit. Per recon §2/§3.

- [ ] **Step 1: Delete the old .proto files** (api + models, per list). test_run.proto now safe (Task 1 freed FE).
- [ ] **Step 2: Split workflow/test.proto + deployment.proto** — surgically keep RunState/Stage/StageUpdate + GetRunState service in test.proto; move AgentBootstrap to workflow/agent.proto; delete the rest. Keep the go_package/package cloud.v1.workflow.
- [ ] **Step 3: make protocols** — regen. Revert unrelated TS drift. Expect NewOgenAdapter signature to shrink + generated service files to vanish.
- [ ] **Step 4: Fix run.go** — drop the 8 nil args from api.NewOgenAdapter (compute the new arg order from the regenerated signature — recon §3 lists the surviving 15: compare/favorite/iam/package/rating/publicRating/publicShare/quota/recipe/share/stroppy/systemSettings/tenantDashboard/tenantSettings/testRunOverview). Verify gqlapi.Server literal still compiles (named fields).
- [ ] **Step 5: build+test+tsc** — `go build ./...` + `go test ./internal/...` + `go vet ./...` (pre-existing only) + `cd web && npx tsc -b --noEmit` green. Grep no dangling refs to deleted proto types.
- [ ] **Step 6: Commit** `chore(proto): delete old service protos and split workflow protos`

---

### Task 5: Drop old tables migration

**Files:**
- Create: `internal/infrastructure/postgres/migrations/<timestamp>_drop_old_tables.sql` (via make migrate-generate or hand-write, mirroring existing migration format)
- Regen: `make db-gen` (sqld codegen drops the now-orphaned table constants from gen/db/gen/bob)

**Interfaces:** DROP the 8 orphaned tables (recon §4): database_preset_records, workload_preset_records, test_preset_records, suite_records, suite_run_records, suite_wizard_drafts, test_wizard_drafts, network_reservations. KEEP test_run_records/quota/recipe/iam/share/favorite/package/tenant_settings/platform_settings.

- [ ] **Step 1: Edit schema.sql** — remove the 8 CREATE TABLE blocks (so make db-gen stops generating their code).
- [ ] **Step 2: migrate-generate** (or hand-write) a DROP TABLE migration for the 8; verify it embeds.
- [ ] **Step 3: make db-gen** — regenerate gen/db/gen/bob (orphaned constants vanish).
- [ ] **Step 4: build+test** — `go build ./...` + `go test ./internal/...` green.
- [ ] **Step 5: Commit** `chore(postgres): drop old preset/suite/network tables`

---

## Self-Review
- Order keeps build/tsc green: FE surgery (T1, frees test_run.proto) → Go dead-code (T2, frees quota refs) → suite refactor (T3, frees suite.proto) → proto delete+split+regen+run.go (T4) → migration (T5).
- KEEP invariants: RunState/Stage/StageUpdate/AgentBootstrap/GetRunState, models/test_run.proto, test_run_overview/tenant_dashboard services (minus suite fields), recipe/dsl/quota-view, test_run_records+recipe+iam+quota tables.
- run.go NewOgenAdapter arity is the one mandatory Go edit after regen (recon §3).
- models/suite.proto only deletable after T3 refactor (the trap); T4 does the delete.
- After 1E-B: no old proto, no old tables, no residual dead code. Prod-clean.
