# Сабпроект 1: Persistence + Flow Wiring (backend)

Дата: 2026-07-03. Статус: дизайн утверждён (автономный режим, greenfield). Продолжение [пивота](2026-07-03-yaml-dsl-pivot-design.md).

## 1. Что и зачем

Движок DSL готов (Tasks 1-18, ветка feat/yaml-dsl-pivot), но **dormant** — `ExecuteCompiledPlanWorkflow` не имеет ни одного production-caller'а, провайдер-раннер на fakes, рецепты негде хранить. Этот сабпроект превращает движок в **живой backend-flow**: рецепт хранится в БД → запускается → провижинит инфру реальными executor'ами → исполняет CompiledPlan → статус/логи/метрики текут в UI-читатели.

**Greenfield** (решение владельца): старый Go-путь сносим, не мигрируем. Инвариант — стек и подходы к коду не меняются.

**Ограничение deploy**: деплой на stage запрещён до отдельной команды. Поэтому цель сабпроекта — **весь код flow + локальные/юнит/интеграционные тесты** (testsuite Temporal, fake/local docker). Проверка «рецепт реально катится на stage» — отдельный шаг ПОСЛЕ разрешения на deploy, вне этого сабпроекта.

## 2. Целевой flow

```
UI/API → RecipeRunService.StartRun(recipeRef, tenant)
              │ persist run record (PENDING)
              ▼
        RunRecipeWorkflow (новый wrapper, task queue "stroppy-cloud")
          │ emit RunState stages (GetRunState query — UI-совместимость)
          ├─ stage "compile":   load recipe bundle → dsl.Compile → CompiledPlan
          ├─ stage "infra":     quotas/network (reuse) + provider.Provision → Machines (device paths!)
          ├─ stage "bootstrap": Nomad server on gateway node + clients on nodes (cloud-init)
          ├─ stage "execute":   ExecuteCompiledPlanWorkflow(plan, machines, bootstrap, gatewayNode)
          └─ defer teardown:    provider.Destroy + release quotas/network
              │ PersistRunState / AppendRunLogs / metrics (reuse existing activities)
              ▼
        run record → SUCCEEDED/FAILED; UI reads via overview/metrics/logs (generic projection)
```

## 3. Фазы

Сабпроект разбит на 5 фаз, каждая — отдельный план (спек→plan→SDD). Порядок по зависимостям:

- **1A — Recipe persistence + RecipeService**: таблица + repo + connect CRUD + Compile/Check делегация. Независима, первой.
- **1B — Provider adapters + device path (закрывает I2)**: thin-адаптеры к реальным terraform/docker executor'ам; провайдер возвращает disk device path в MachineState. Зависит только от существующего provider-пакета.
- **1C — Nomad bootstrap**: Bootstrap-расширение + cloud-init/docker Nomad install; gateway single-node server + clients. Зависит от agent-bootstrap.
- **1D — RunRecipeWorkflow + binding parity (закрывает I1) + launch wiring + RunState**: wrapper-workflow, единые CEL-биндинги compile↔runtime, RecipeRunService launch, GetRunState query, generic overview projection. Зависит от 1A/1B/1C.
- **1E — Снос старого пути**: test.go stage-machine, suite.go, test_workflows.go, test_wizard_engine + internal/domain/database/*, presets_seed, старые preset/suite/wizard сервисы и таблицы. ПОСЛЕ того как 1D зелёный (юнитами).

UI (dslClient + DslEditor + recipe-экраны + run-from-recipe) — **сабпроект 2**, не здесь.

## 4. Дизайн-решения

### 4.1 Recipe storage (1A)
komeet-паттерн, jsonb data-envelope (не blobstore — YAML-бандлы мелкие). Новая таблица `recipe_records`: `id/tenant_id/name/version/created_at/updated_at` + `data jsonb` (протоджсон `models.RecipeRecord`). Бандл = `map<string,bytes> files` внутри proto (те же ключи, что `include.Sources`: `cluster.yaml`, `workflow.yaml`, `components/**`, `providers/**`). Версии иммутабельны, `(tenant_id, name, version)` unique; отдельный «latest»-указатель — поле или max(version). Триада: `schema.sql` + `queries/recipe.sql` + `make db-gen` + `RecipeRepo` (мирроринг `SuiteRepo` в `records.go`).

`RecipeService` (connect, `internal/services/recipe/`): `Create/Get/List/Delete` (tenant-scoped, `Authn.Caller` + `req.tenant_id`, method-auth `any_authenticated`), плюс `Compile`/`CheckRecipe` — делегируют в существующий `dsl.Compile`/`dslService`-логику над сохранённым бандлом (переиспользовать код `internal/services/dsl`, не дублировать). Регистрация в `run.go` рядом с `dslService` (~609).

### 4.2 Provider device path — закрывает I2 (1B)
`Provider.Provision` возвращает `MachineState`, несущий disk device path, чтобы `dslrun.machineViewFor` заполнял `DiskView.Path` не пустым.
- terraform: путь из output модуля (`stroppy_machines[].disk_device`, доб. в контракт stroppy_machines) или конвенция `/dev/vdb`; кладём в endpoint label `disk_device`.
- docker: bind-mount host path; label `disk_device`.
Реальные адаптеры: `terraformExec` impl поверх `terraform.Executor.Execute(*Terraform_Input)` (строим Input как `renderTerraformInput`: embedded HCL + baked tfvars + env); `dockerExec` impl аккумулирует `ContainerSpec` в один `Docker_Input.Containers` и зовёт `Executor.Up`/`Down` (реальная batch-форма). Тесты: fake docker client / `renderDockerInput`-эквивалентность; device path проверяется дифф-тестом MachineState.

### 4.3 CEL binding parity — закрывает I1 (1D)
Runtime интерпретатор ДОЛЖЕН биндить тот же набор переменных, против которого компайл-тайм чекер (`graph.Validate`) типизировал. Решение: **пронести резолвленные inputs/target в CompiledJob** (proto `compiled.proto` += `map<string,string> resolved_inputs` + `string target_group` на `CompiledJob`; lower заполняет из `BoundComponent`), и `dslrun` строит `ComponentEnv`-совместимый набор `{inputs, target, machine, machines, matrix}` из этих полей + Machines. Единый билдер биндингов в общем пакете (напр. `internal/dsl/expr` или новый `bindctx`), вызываемый и чекером, и runtime — гарантия, что «что typecheck'нулось, то и биндится». Дифф-тест: рецепт с `${{ inputs.* }}` в cmd/when компилируется И исполняется (testsuite) без unbound-var.

### 4.4 Nomad bootstrap (1C)
`agentdomain.Bootstrap` += `NomadRole` (server|client|none) + `NomadServerAddr`. `CloudInit` template += блок: install `nomad` binary, write `/etc/nomad.d/{server|client}.hcl` (client: `meta { stroppy_node_id = "<machineID>" }`, `dc1`), `systemctl enable --now nomad`. Docker-путь: nomad как доп. `Docker_File` + entrypoint или sidecar (v1: single-node на gateway-контейнере, dev-mode). Gateway-нода рана = single-node Nomad server; остальные ноды = clients. Активности `NomadSubmitJobActivity` роутятся на gateway task queue (уже есть `GatewayNodeID` в `ExecuteCompiledPlanInput`; wrapper выбирает gateway-ноду = runner). Тесты: рендер cloud-init/Docker_File содержит корректный nomad-config (юнит по шаблону), meta stamping.

### 4.5 RunRecipeWorkflow + RunState + wiring (1D)
Новый wrapper-workflow (`internal/workflows/runrecipe.go`), task queue `"stroppy-cloud"`, workflow ID `run-recipe/<runID>`:
- Стадии эмитятся как `*workflowpb.RunState{Status, []*Stage}` — переиспользуем `PersistRunState`/`AppendRunLogs`/metrics активности и **экспонируем `GetRunState` query** (UI overview live-path работает без изменений).
- compile → infra (reuse `CalculateQuotasWorkflowChild`/`ProcessInfrastructureWorkflowChild` для quotas/network + `provider.Provision`) → bootstrap → `ExecuteCompiledPlanWorkflow` как child → defer teardown.
- Cancel-хендлинг (Temporal cancel → teardown через defer).
- Launch adapter: `RecipeRunService.StartRun` (новый или расширение test_run) persist run record + `client.ExecuteWorkflow(RunRecipeWorkflow)`. `internal/infrastructure/execution` — новый `RecipeWorkflows` порт вместо `test_workflows.go`.
- `overview.go`: persisted-record fallback (`pipelineFromRecord`, hardcoded 5 stages) → **generic projection из `RunState.Stages`** (live-path `projectOverviewWithSource` уже generic — переиспользовать для fallback).

### 4.6 Что НЕ трогаем
Переиспользуемые as-is (generic, не завязаны на старые стадии): `PersistRunState`/`PersistDeploymentPlan`/`AppendRunLogs`/`PersistSuiteRun` активности + `RuntimeActivities` порт (`runtime.go`); `MetricsReader`/`GetRunMetrics`; log query/stream; auth-слой (`authzGate`, method-auth, tenant-из-request); komeet DB-паттерн; connect/worker wiring-механика.

## 5. Снос (1E) — точный список
После зелёного 1D: `internal/workflows/test.go` (stage-machine), `suite.go` (TestWorkflowChildAsync coupling → на RunRecipe children), `internal/infrastructure/execution/test_workflows.go` + `ids.go:testWorkflowID`; `internal/infrastructure/execution/test_wizard_engine.go` + `suite_wizard_engine`; `internal/domain/database/*` (~5.7 KLOC per-DB renderers/topology); `internal/app/presets_seed.go` + `seedBuiltinCatalog` цепочка в `seed.go`; старые preset/suite/wizard connect-сервисы; таблицы `database_preset_records`/`workload_preset_records`/`test_preset_records`/`suite_records`/`suite_run_records`/`*_wizard_drafts` (миграция-дроп). `seedFirstBoot` admin/tenant/role bootstrap — ОСТАВИТЬ. Overview generic — уже в 1D.

Осторожно: preset/suite/wizard сервисы могут дёргаться фронтом — их экраны мертвеют вместе со сносом; фронт переезжает в сабпроекте 2. На время между 1E и сабпроектом 2 старые экраны сломаны — приемлемо (WIP-ветка, no deploy).

## 6. Тестирование (всё локально, без deploy)
- 1A: repo CRUD (pgtx tx-тест как существующие), RecipeService (in-memory/fake store), Compile-делегация над сохранённым бандлом.
- 1B: адаптеры против fake docker client + tfexec-мока; device-path дифф-тест MachineState; эквивалентность real-Input рендеру (renderDockerInput/renderTerraformInput).
- 1C: рендер-тесты cloud-init/Docker_File (nomad config, meta stamp).
- 1D: `testsuite.WorkflowTestSuite` — RunRecipeWorkflow полный DAG с моками (compile→infra→bootstrap→execute→teardown), RunState-эмиссия, GetRunState query, cancel→teardown; binding-parity дифф-тест (I1); launch adapter; overview generic projection unit.
- 1E: build зелёный после сноса, оставшиеся тесты проходят, `go vet`/lint clean.
- **Live stage e2e** (реальный ран рецепта на YC/docker) — ОТЛОЖЕН до разрешения на deploy; фиксируется как единственный не-локальный gate.

## 7. Открытые вопросы (решаются в планах фаз)
- Recipe версионирование: явное поле version + latest-pointer vs. max(version) — решить в 1A.
- Nomad на docker-провайдере: sidecar-контейнер vs. nomad-в-agent-контейнере — решить в 1C (v1: single-node dev на gateway).
- resolved_inputs в proto vs. пересборка BoundComponent в runtime — решить в 1D (склоняюсь к proto-полю: детерминизм + golden-совместимость).
- RecipeRunService: новый сервис vs. расширение TestRunService — решить в 1D (склоняюсь к новому, старый под снос).
