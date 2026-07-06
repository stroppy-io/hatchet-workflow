# Subproject 2: Recipe/DSL UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Checkbox steps.

**Goal:** A production recipe/DSL UI replacing the deleted wizard/preset/suite screens: browse/edit recipe bundles in a CodeMirror editor with live DSL diagnostics, launch runs, and view run status/logs/metrics (reusing the kept overview backend). Plus the small backend RPCs the UI needs (ListRuns, CancelRun, DeleteRun).

**Architecture:** Frontend on the existing stack (React 19 + Vite + connect-web + Tailwind 4 + shadcn primitives + CodeMirror 6). `recipeClient`/`dslClient` added to client.ts; a `recipe.ts` service module (proto↔VM, resolveTenantId); a `DslEditor` mirroring `json-editor.tsx`'s `linter()` mechanism, mapping `DslService.check` diagnostics to CodeMirror lint. Backend gaps closed first: `RecipeService.ListRuns/CancelRun/DeleteRun` (reuse TestRunRepo + Temporal CancelWorkflow). Old dead screens removed last.

**Tech Stack:** Go (proto+service for backend RPCs), TypeScript/React/CodeMirror/Tailwind (frontend).

## Global Constraints

- Module `github.com/stroppy-io/stroppy-cloud`; branch `feat/yaml-dsl-pivot`.
- Commits: Conventional Commits, imperative lowercase, NEVER Co-Authored-By/AI attribution.
- Spec: subproject 2 of `docs/superpowers/specs/2026-07-03-flow-wiring-design.md` (UI).
- **Gate**: backend tasks → `go build ./...` + `go test ./internal/...` green; frontend tasks → `cd web && npx tsc -b --noEmit` green (the typecheck gate; `npm run build` = tsc+vite). Visual browser verification is deploy-gated (deferred).
- Reuse existing UI: shadcn primitives (`web/src/components/ui/*`: Card/Button/Table/Dialog/Tabs/Badge/Input/confirm-dialog), Tailwind dark theme (`web/src/index.css` @theme), tenant router (`@/lib/router`: Link/NavLink/useNavigate/useTenantSlug), `resolveTenantId` (`@/services/tenant`), `json-editor.tsx` linter pattern. Match existing visual style — do NOT introduce a new design language.
- Proto regen (`make protocols`) reverts unrelated TS drift (known easyp issue); stage only intended.
- No deploy. golangci/tsc clean for touched code.

---

### Task 1: Backend RPCs — RecipeService.ListRuns / CancelRun / DeleteRun

**Files:**
- Modify: `protocols/cloud/v1/api/recipe.proto` (3 RPCs) + regen
- Modify: `internal/services/recipe/recipe.go` (handlers) + `service.go` (ports)
- Modify: `internal/infrastructure/execution/recipe_workflows.go` (CancelRecipeRun via Temporal client CancelWorkflow "run-recipe/<id>")
- Test: `internal/services/recipe/recipe_test.go`

**Interfaces:**
```proto
rpc ListRuns(ListRunsRequest) returns (ListRunsResponse);   // {tenant_id, recipe_id? optional filter} → repeated models.TestRunRecord. auth RESOURCE_TEST_RUN/ACTION_LIST
rpc CancelRun(CancelRunRequest) returns (CancelRunResponse); // {tenant_id, run_id} → cancel the running RunRecipeWorkflow. auth RESOURCE_TEST_RUN/ACTION_UPDATE
rpc DeleteRun(DeleteRunRequest) returns (DeleteRunResponse); // {tenant_id, run_id} → delete run record. auth RESOURCE_TEST_RUN/ACTION_DELETE
```
- ListRuns: reuse the kept `TestRunRepo` (store.TestRuns()) — add a List(ctx, tenantID) if not present (grep; TestRunRepo likely has ListTestRuns). Recipe runs are TestRunRecords (StartRun persists them). Optional recipe_id filter: recipe runs don't currently store their recipe id — either add a label on the record at StartRun (recipe_id in TestRunRecord labels/tags) or list all tenant runs (v1: list all tenant runs, document; add recipe_id filter later). DECIDE: v1 list all tenant runs, ignore recipe_id filter or filter by a label if StartRun stamps one — prefer stamping recipe_id into the record's labels at StartRun (Task also touches StartRun) so ListRuns can filter.
- CancelRun: RecipeWorkflows port gains CancelRecipeRun(ctx, runID) → Temporal client.CancelWorkflow(ctx, "run-recipe/"+runID, ""). Mark record CANCELLED? (the workflow's cancel path persists status; handler just triggers cancel).
- DeleteRun: TestRunRepo.Delete(ctx, tenant, id) (grep it exists).

- [ ] **Step 1: proto + regen** — 3 RPCs with auth options, revert TS drift, go build green.
- [ ] **Step 2: Failing tests** — fake repo+workflows: ListRuns returns tenant runs; CancelRun calls CancelRecipeRun; DeleteRun deletes record; missing tenant→InvalidArgument; StartRun stamps recipe_id label (if chosen).
- [ ] **Step 3: Run — FAIL**
- [ ] **Step 4: Implement** handlers + CancelRecipeRun in recipe_workflows.go + ports.
- [ ] **Step 5: Run — PASS** (`go test ./internal/services/recipe/... ./internal/infrastructure/execution/...`) + go build + wire any new handler (already registered service, new RPCs auto-included).
- [ ] **Step 6: Commit** `feat(recipe): add ListRuns/CancelRun/DeleteRun RPCs`

---

### Task 2: Frontend clients + recipe service module

**Files:**
- Modify: `web/src/services/client.ts` (add recipeClient + dslClient)
- Create: `web/src/services/recipe.ts`

**Interfaces:**
- recipeClient = createClient(RecipeService, transport); dslClient = createClient(DslService, transport) (imports from `@/lib/proto/cloud/v1/api/recipe_pb` + `dsl/service_pb`).
- recipe.ts exports (mirror `runs.ts`/`preset.ts` shape: resolveTenantId + proto↔VM):
  - `listRecipes(slug) → RecipeVM[]`, `getRecipe(slug, id) → RecipeVM` (with bundle files decoded Uint8Array→string), `createRecipe(slug, {name, files}) `, `deleteRecipe(slug, id)`, `checkStoredRecipe(slug, id) → Diagnostic[]`.
  - `checkBundle(files: Record<string,string>) → Diagnostic[]` (dslClient.check, files as Uint8Array via TextEncoder) + `composedSchema(files) → schemaJson`.
  - `startRun(slug, recipeId) → runId`, `listRuns(slug, recipeId?) → RunVM[]`, `cancelRun(slug, runId)`, `deleteRun(slug, runId)`.
  - VM types: RecipeVM{id,name,version,provider,machineGroupCount,serviceCount,compiles,files: Record<string,string>}; DiagnosticVM{severity,path,line,col,message,module}.

- [ ] **Step 1: Add clients + write recipe.ts** with the VM mappers (Uint8Array↔string via TextEncoder/TextDecoder for bundle files).
- [ ] **Step 2: tsc** — `cd web && npx tsc -b --noEmit` green.
- [ ] **Step 3: Commit** `feat(web): recipe/dsl clients and service module`

---

### Task 3: DslEditor component (CodeMirror + Check linter)

**Files:**
- Create: `web/src/components/ui/dsl-editor.tsx`
- Test/verify: tsc

**Interfaces:**
- `<DslEditor path value onChange files />` — CodeMirror (yaml() extension for .yaml/.yml, plain otherwise) + `linter()` source that debounce-calls `checkBundle(files)` and maps server DiagnosticVM (1-based line/col, filtered to `path`) → CodeMirror Diagnostic (`doc.line(line).from + (col-1)`, severity ERROR→"error"/WARNING→"warning"), + lintGutter(). Mirror `json-editor.tsx` exactly for the CodeMirror setup + fontTheme.
- Debounce checkBundle (e.g. 400ms) so keystrokes don't spam the server; only apply diagnostics for the currently-open `path`.

- [ ] **Step 1: Write DslEditor** mirroring json-editor.tsx; linter maps Check diagnostics.
- [ ] **Step 2: tsc green.**
- [ ] **Step 3: Commit** `feat(web): DslEditor with live DSL diagnostics`

---

### Task 4: Recipes list page

**Files:**
- Create: `web/src/pages/Recipes.tsx`

**Interfaces:**
- Table (shadcn table.tsx) of recipes: name, version, provider, compiles badge, machine/service counts, actions (Open, Delete via confirm-dialog). "New Recipe" button → navigate to recipes/new. Uses recipe.ts listRecipes/deleteRecipe. useTenantSlug. Match Suites.tsx/preset list visual patterns (Card wrapper, table, Button variants).

- [ ] **Step 1: Write Recipes.tsx** (list + create nav + delete confirm). tsc green.
- [ ] **Step 2: Commit** `feat(web): recipes list page`

---

### Task 5: RecipeEditor page (bundle editor + check + save + run)

**Files:**
- Create: `web/src/pages/RecipeEditor.tsx`

**Interfaces:**
- New (recipes/new) or edit (recipes/:id) a recipe bundle. Left: a file tree/tabs (Tabs component) of bundle files (cluster.yaml, workflow.yaml, components/**, providers/**) + add/remove file. Right: DslEditor for the active file, live diagnostics across the bundle. A diagnostics panel (all files' errors from checkBundle). Header: recipe name input, Save (createRecipe — new version), Run (startRun → navigate to run detail). Uses recipe.ts. Seed a new recipe with a minimal cluster.yaml/workflow.yaml template. Match existing form pages (DatabasePresetForm.tsx) visual style.

- [ ] **Step 1: Write RecipeEditor.tsx** (multi-file bundle editing, DslEditor, checkBundle diagnostics panel, Save/Run). tsc green.
- [ ] **Step 2: Commit** `feat(web): recipe editor with bundle files and run launch`

---

### Task 6: Recipe runs — list + RunDetail reuse (rewire actions)

**Files:**
- Create: `web/src/pages/RecipeRuns.tsx` (runs list for a recipe / tenant, via recipe.ts listRuns)
- Modify: `web/src/pages/RunDetail.tsx` (re-point the action buttons rerun/cancel/delete from the dead runs.ts provider to recipe.ts: startRun(rerun via recipe)/cancelRun/deleteRun; keep the overview/metrics/logs read path unchanged — it uses the KEPT testRunOverviewClient)
- Modify: `web/src/services/run_overview.ts` if the action wiring lives there — re-point to recipe.ts.

**Interfaces:**
- RecipeRuns: table of runs (id, status, started, actions cancel/delete) from listRuns; row → RunDetail.
- RunDetail: overview/metrics/logs UNCHANGED (testRunOverviewClient kept). Actions: cancel→recipe.cancelRun, delete→recipe.deleteRun, rerun→recipe.startRun(recipeId) (recipeId from the run's label if ListRuns stamped it, else hide rerun). Remove the dead runs.ts provider calls.

- [ ] **Step 1: Write RecipeRuns + rewire RunDetail actions to recipe.ts.** tsc green (RunDetail no longer imports dead runs provider actions).
- [ ] **Step 2: Commit** `feat(web): recipe runs list and rewired run detail actions`

---

### Task 7: Routing + nav

**Files:**
- Modify: `web/src/App.tsx` (add recipes routes, remove dead routes: runs/new, suites*, presets/*), `web/src/components/TenantSidebar.tsx` (add Recipes nav, remove dead nav entries: New Run/Suites/Presets)

**Interfaces:**
- Routes: `recipes` (Recipes), `recipes/new` + `recipes/:id` (RecipeEditor), `recipes/runs` (RecipeRuns), `runs/:id` (RunDetail — kept). Remove routes to NewRun/Suites/SuiteDetail/SuiteWizard/library presets. Nav: Recipes (minLevel 1), New Recipe (minLevel 2), Runs; remove Suites/Presets/New Run nav items.

- [ ] **Step 1: Wire routes + nav.** tsc green (removed routes' page imports removed).
- [ ] **Step 2: Commit** `feat(web): recipe routes and navigation`

---

### Task 8: Remove dead old screens + service modules

**Files:**
- Delete: `web/src/pages/{NewRun,Suites,SuiteDetail,SuiteWizard,Runs}.tsx`, `web/src/pages/library/*Preset*.tsx`, `web/src/services/{wizard,preset,suites,suiteWizard,suiteRuns,runs}.ts` (and their now-unused client.ts exports for deleted services)
- Modify: `web/src/services/client.ts` (remove dead client exports: testRunClient/testWizardClient/suiteClient/suiteRunClient/suiteWizardClient/databasePresetClient/workloadPresetClient/testPresetClient)

**Interfaces:**
- After removal, grep web/src for any remaining import of the deleted modules/clients; fix. tsc green. NOTE: the generated proto TS for those services (test_run_pb.ts etc.) can stay (harmless, removed in 1E-B with backend proto) OR delete now — leave them (1E-B).

- [ ] **Step 1: Delete dead screens+services+client exports, fix dangling imports.** tsc green.
- [ ] **Step 2: Commit** `chore(web): remove old wizard/suite/preset screens`

---

## Self-Review
- Backend gaps closed (T1: ListRuns/CancelRun/DeleteRun) before frontend needs them.
- Reuses kept overview/metrics/logs backend for run detail (T6); does not rebuild them.
- Visual consistency: reuses shadcn primitives + dark theme + tenant router + json-editor linter pattern (no new design language).
- Gate: backend go build/test; frontend tsc. Visual browser check deploy-gated (deferred — noted).
- Deferred to 1E-B: delete old proto (backend+TS) + drop old tables. The old proto TS staying is why tsc passes through this subproject.
- Open product gaps flagged: recipe_id on run records (stamped at StartRun for ListRuns filter + rerun); if not stamped, rerun/recipe-scoped run list degrade to tenant-wide (documented).
