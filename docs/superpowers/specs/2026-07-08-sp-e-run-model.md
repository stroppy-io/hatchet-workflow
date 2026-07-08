# SP-E: Run-модель rewrite

Дата: 2026-07-08. Статус: дизайн утверждён (brainstorming). Ветка: `feat/yaml-dsl-pivot`.
Часть [product-vision](2026-07-08-product-vision.md), саб-проект E (переиспользует `schemapb.Baked` из SP-A, §3.4/§7).

---

## 1. Что и зачем

Сегодня прогон (recipe run) хранится в `models.TestRunRecord` (`protocols/cloud/v1/models/test_run.proto`) — модели, унаследованной от до-пивотного DAG-исполнителя (`domain.TestRun`/классический `TestWorkflow`). Этот классический воркфлоу уже **выпилен**: `internal/workflows/register.go` регистрирует только `ExecuteCompiledPlanWorkflow` и `RunRecipeWorkflow` — упоминания `domainTestWorkflow` в комментариях `runrecipe.go`/`runtime_projection.go` чисто исторические ("mirrors domainTestWorkflow's own teardown defer"), самого воркфлоу в коде нет.

Но `RunRecipeWorkflow` (единственный живой путь запуска) **переиспользует** `TestRunRecord`, для которой большинство полей структурно не подходят. Это задокументировано прямо в коде:

- `protocols/cloud/v1/api/recipe.proto`, `StartRunResponse`: *"the run reuses models.TestRunRecord so overview/metrics/logs work unchanged for a recipe run; **its spec/topology fields are left empty**"*.
- `internal/services/recipe/recipe.go`, `StartRun`: *"The minted TestRunRecord deliberately **carries no Spec/Topology**: those fields describe a `domain.TestRun` (the classic TestWorkflow input), which a recipe run has none of"*.

Симптом из product-vision §1: «таблица runs кардинально изменена» и «recipe-раны оставляют половину полей пустыми» — это ровно `TestRunRecord.Spec`, `TestRunRecord.InfrastructureState`, `TestRunRecord.DeploymentPlan`, которые `RunRecipeWorkflow` никогда не заполняет, восполняя своими собственными боковыми полями (`Summary`, `RecipeTopology`, `RuntimeState`) через отдельные функции `persistRunSummary`/`persistRecipeTopology`/`persistRunState`.

**SP-E** заменяет `TestRunRecord` на новую first-class сущность `Run`, чьи поля 1:1 соответствуют тому, что `RunRecipeWorkflow` реально производит (compile → `CompiledPlan` + `Baked`-идентичность; infra → topology; execute → per-job status; всегда — рефы на observability), и мигрирует **всех читателей** прогона (runs-таблица, overview, pipeline/topology/agents/quotas/logs/metrics/grafana вкладки RunDetail, compare, rating) на неё, не теряя ни одного поля, которое сегодня доходит до экрана.

---

## 2. Scope

**В scope:**
- Новое proto-сообщение `models.Run` (замена `models.TestRunRecord`), с `Baked`/`CompiledPlan`-снапшотами, topology, runtime-состоянием, observability-рефами, rating-флагами, denormalized `Summary`.
- Новая postgres-таблица `run_records` (komeet jsonb-паттерн, как `test_run_records` сегодня) + `RunRepo`.
- Переписка write-пути `RunRecipeWorkflow` (`internal/workflows/runrecipe.go`, `runrecipe_summary.go`, `runtime.go`) на запись `Run` вместо `TestRunRecord`.
- Миграция ВСЕХ читателей (см. §3.4) на `Run`: `OverviewReader` (`internal/infrastructure/execution/overview.go`), `TestRunRepo.List`→`RunRepo.List` (`internal/infrastructure/postgres/test_run_list.go`), `internal/services/compare`, `internal/services/recipe`, `internal/services/tenant_dashboard`, `internal/services/quota`, `internal/infrastructure/adapters/{compare,favorite,rating,share,tenant_dashboard}.go`, web (`RunDetail.tsx`, `RecipeRuns.tsx`, `Compare.tsx`, `RunInfoSidebar.tsx`, `RunConfigTab.tsx`, `services/{runs,run_overview,recipe,dashboard}.ts`).
- proto-переименования зависимых сообщений: `api/test_run.proto` (`ListTestRunsRequest/Response`), `api/test_run_overview.proto` (`TestRunOverviewSnapshot.run`), `api/compare.proto`/`api/rating.proto` (читают через репозиторий, не сам тип — см. §4), `api/recipe.proto` (`StartRunResponse.run`, `ListRunsResponse.runs`).
- **Инвариант data-parity** (§6 — обязательный чеклист).

**Вне scope:**
- Сама генерация `Baked` (Filled→Baked, WASM-валидация, форма) — это SP-D. SP-E только **хранит** `Baked` на `Run` и определяет, что поле существует; заполнять его будет SP-D, когда появится generated-form launch-путь (см. §7 — до SP-D поле остаётся пустым, `RunRecipeWorkflow` продолжает запускаться как сегодня, через `RecipeService.StartRun(recipe_id)`).
- Catalog/tenancy-модель провайдера и workflow (SP-B) — `Run.workflow_id` резервирует место под будущую ссылку на catalog `Workflow`, но до SP-B её значением остаётся текущий `RecipeRecord.entity.id` (см. §7, §9).
- Provider-креды per-org, per-run log capture, terraform на реальном стенде — SP-F.
- Suite/`SuiteRunRecord` — уже выпилены системой ДО SP-E (см. `test_run_overview.proto`: *"4 was suite_run (models.SuiteRunRecord): suites/suite runs were removed with the old test-orchestration backend"*). SP-E лишь подчищает мёртвые `suite_run_id`/`suite_cell_id` поля вслед за этим (см. §5, §6).

---

## 3. Компоненты

### E1. `models.Run` proto-сообщение

Заменяет `models.TestRunRecord` (`protocols/cloud/v1/models/test_run.proto`). Живёт в том же файле/пакете (`cloud.v1.models`), схема — см. §4.

### E2. Postgres `run_records`

Заменяет `test_run_records` (`internal/infrastructure/postgres/schema.sql:9-16`). Тот же komeet jsonb-конверт:

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

`RunRepo` (заменяет `TestRunRepo`, `internal/infrastructure/postgres/test_run_list.go`) переносит динамический `List`-запрос: те же jsonb path-выражения (`jDBKind`, `jProvider`, `jProgressPct`, ...), только на `data->'summary'->>...` из **нового** `Run.summary` (структурно идентичного `TestRunRecord.Summary` — см. §4) и с `run_records` вместо `test_run_records`. Greenfield: `test_run_records` **дропается**, данных не переносим (§5).

### E3. Запись из `RunRecipeWorkflow`

`internal/workflows/runrecipe.go` сегодня пишет прогон в 4 точки: `persist` (RunState, при каждой смене стадии), `persistRunSummary` (сразу после компиляции — `runrecipe_summary.go`), `persistRecipeTopology` (сразу после provision), плюс неявное создание записи в `internal/services/recipe/recipe.go`'s `StartRun`. SP-E меняет форму записи, не меняет точки/порядок:

1. `StartRun` (`recipe.go`) минтит `Run` вместо `TestRunRecord`: `Entity` + `Status=PENDING` + `Trigger=API` + `WorkflowId=recipe_id` + rating-флаги — как сегодня, без Spec-эквивалента (`Baked`/`CompiledPlan` ещё не существуют на этом шаге).
2. Компиляция успешна → **новая** запись `persistRunCompiledPlan(ctx, runID, plan)`: сохраняет `dslpb.CompiledPlan` целиком на `Run.compiled_plan` (сегодня `plan` живёт только в памяти воркфлоу и передаётся в `ExecuteCompiledPlanWorkflow` — нигде не персистится). Плюс существующий `persistRunSummary` (derivation logic `deriveRunSummary` не меняется — тот же вход `plan`, тот же выход `Summary`-shape, теперь на `Run.summary`).
3. Provision успешен → `persistRecipeTopology` записывает на `Run.topology` (переименованный тип `RunTopology`, бывший `RecipeTopologySnapshot`, см. §4) — derivation `deriveRecipeTopology` не меняется.
4. Каждая смена стадии → `persist` записывает `workflowpb.RunState` на `Run.runtime_state` (тип не меняется — переиспользуем `workflow.RunState`, см. §5 «переиспользуем»).

`Baked` заполняется НЕ здесь: до SP-D `RunRecipeInput` не несёт `Baked` (запуск идёт по текущему `recipe_id`-пути), поле остаётся `nil`. SP-D добавит: `StartRun`-эквивалент бэйкает форму → кладёт `Baked` в `RunRecipeInput` → `RunRecipeWorkflow` персистит его рядом с `compiled_plan` на шаге 2 (тот же commit-point, тривиальное расширение — задел оставляем в интерфейсе `RunRecipeInput`, реализацию не трогаем).

### E4. Читатели

Полный список (см. §6 за построчным mapping'ом):

- **`OverviewReader`** (`internal/infrastructure/execution/overview.go`) — 26 функций, все принимающие `*models.TestRunRecord` (`topologyFromRecordWithRunState`, `overviewFromRecord`, `mergeOverviewWithRecord`, `pipelineFromRecord`, `workersFromRecord`, `recordWorkerPresence`, `recordStageNode`, `recordInfrastructureStatus`, ...) — сигнатуры меняются на `*models.Run`, тела перечитывают те же под-поля (`.Status`→`.Status`, `.RuntimeState`→`.RuntimeState` (не переименовано), `.RecipeTopology`→`.Topology`, `.Summary`→`.Summary`).
- **`RunRepo.List`/`Facets`** (постгрес, §3.2) — обслуживает `RecipeService.ListRuns`, `RatingService`, `internal/app`'s tenant-dashboard/rating glue (см. `api/test_run.proto`'s doc-комментарий — этот один query shape реально шарится четырьмя вызывающими сторонами).
- **`internal/services/compare`** (`compare.go`, `service.go`) — `CompareService.CompareRuns` собирает `RunColumn` из `Run.entity/status/summary`.
- **`internal/services/recipe`** (`recipe.go`, `service.go`) — `StartRun`/`ListRuns`/`CancelRun`/`DeleteRun`.
- **`internal/services/tenant_dashboard`** — "Top benchmarks"/recent-runs виджеты, читает `Summary` + rating-флаги.
- **`internal/services/quota`** — читает `Run` только для tenant/entity-скоупа (квоты сами keyed по `tenant_id`+`run_id`, не по полям `Run`) — механическая правка типа.
- **`internal/infrastructure/adapters/{compare,favorite,rating,share,tenant_dashboard}.go`** — конвертеры между `Run` и соответствующими API-сообщениями (`RunColumn`, `FavoriteItem`, `RatingEntry`, share-snapshot, dashboard-виджет).
- **Web**: `web/src/services/runs.ts` (`testRunRecordToVM`→`runToVM`, `RunVM`), `web/src/services/run_overview.ts` (`OverviewVM`), `web/src/services/recipe.ts` (`listRuns`, минимальный `RunVM` для `RecipeRuns.tsx`), `web/src/services/dashboard.ts`; страницы `RunDetail.tsx`, `RecipeRuns.tsx`, `Compare.tsx`; компоненты `RunInfoSidebar.tsx`, `RunConfigTab.tsx`, `PipelineViewer.tsx`, `TopologyViewer.tsx` (не меняются — работают над `topology.Topology`/`monitor.PipelineView`, уже развязаны от `TestRunRecord`).

---

## 4. Публичные интерфейсы (proto)

```proto
// cloud.v1.models, заменяет TestRunRecord (protocols/cloud/v1/models/test_run.proto)
message Run {
  common.Entity entity = 1 [(validate.rules).message.required = true];
  common.Status status = 2;
  common.Trigger trigger = 3;

  // ===== identity / воспроизводимость (Baked = SP-A) =====
  string workflow_id = 4 [(validate.rules).string.max_len = 64];      // ex recipe_id — см. §7 про переход на catalog Workflow (SP-B)
  string workflow_version = 5 [(validate.rules).string.max_len = 32]; // бандла, на момент запуска
  schemapb.Baked baked = 6;         // launch-форма, запечатанная (SP-A Bake); nil до SP-D
  dsl.CompiledPlan compiled_plan = 7; // скомпилированный план + подставленные Baked-значения — НОВОЕ: сегодня план нигде не персистится

  // ===== topology =====
  RunTopology topology = 8;         // переименованный RecipeTopologySnapshot (§5) — provider + машины/сервисы

  // ===== per-job статус (DAG-прогресс из Temporal) =====
  workflow.RunState runtime_state = 9; // тип не меняется — durable-проекция для закрытого Temporal workflow

  // ===== observability рефы =====
  ObservabilityRefs observability = 10;

  // ===== rating/compare =====
  bool in_tenant_rating = 11;
  bool in_global_rating = 12;

  // ===== denormalized queryable facets (runs-таблица) — структура НЕ меняется =====
  Summary summary = 13; // те же 13 полей, что TestRunRecord.Summary сегодня (см. §6)

  reserved 14 to 20; // headroom, мирроринг TestRunRecord'овской дисциплины (там поле 5 было retired)
}

// Переименован из RecipeTopologySnapshot — то же содержимое (provider + MachineNode[]),
// имя обобщено: ВСЕ раны теперь recipe/workflow-раны, префикс "Recipe" больше не различает кейсы.
message RunTopology {
  string provider = 1;
  repeated MachineNode nodes = 2; // MachineNode/ServiceNode без изменений
}

// НОВОЕ: явные рефы вместо неявной конвенции "keyed by run id" (logs.proto/metrics.proto
// доки: "Runtime observations (logs/metrics) are keyed by the run id directly"). Для v1 поля
// заполняются run.entity.id (конвенция сохраняется функционально); сообщение существует, чтобы
// зафиксировать контракт и дать SP-F точку расширения (per-provider Grafana uid и т.п. — см. §9).
message ObservabilityRefs {
  string metrics_query_key = 1; // VictoriaMetrics-скоуп; сегодня == entity.id
  string logs_query_key = 2;    // VictoriaLogs-скоуп; сегодня == entity.id
  string grafana_dashboard_uid = 3; // сегодня хардкод в relay (GrafanaPanel), не per-run
}
```

`Summary` (nested под `Run`, было nested под `TestRunRecord`) переносится **без изменений полей**: `db_kind`, `db_preset_id`, `db_preset_name`, `workload_preset_id`, `workload_name`, `stroppy_version`, `workload_protocol`, `test_preset_id`, `test_preset_name`, `topology_label`, `node_count`, `provider`, `progress_pct`, `started_at`, `finished_at`, `duration`.

### Зависимые API-сообщения (переименование поля/типа, без изменения формы запроса)

- `api/test_run.proto`: `ListTestRunsRequest`→`ListRunsRequest`-эквивалент (уже почти дублирует `api/recipe.proto`'s `ListRunsRequest` — план SP-E должен решить, схлопывать ли их в одно сообщение или оставить `test_run.proto`'s facet/sort машинерию источником истины, а `recipe.proto`'s `ListRunsRequest` — тонкой обёрткой с `recipe_id`-фильтром, как сегодня). `ListTestRunsResponse.runs` → `repeated models.Run`.
- `api/test_run_overview.proto`: `TestRunOverviewSnapshot.run` → `models.Run run = 1`.
- `api/recipe.proto`: `StartRunResponse.run` → `models.Run`, `ListRunsResponse.runs` → `repeated models.Run`.
- `api/compare.proto`, `api/rating.proto`: не ссылаются на `TestRunRecord`/`Run` напрямую (свои `RunColumn`/`RatingEntry` — плоские DTO) — их сборка (`internal/services/compare`, `internal/infrastructure/adapters/rating.go`) переключается с чтения `TestRunRecord` на `Run`, сама proto-схема этих сообщений не меняется (см. §6.D/§6.E).

---

## 5. Что выпиливаем / что переиспользуем

**Выпиливаем:**
- `models.TestRunRecord` целиком (proto-сообщение + `.pb.go`/`.pb.jx.go`/`.pb.validate.go`) — заменён `models.Run`.
- `models.RecipeTopologySnapshot` как отдельное имя — содержимое переезжает в `models.RunTopology` (§4), сам смысл не меняется.
- Postgres-таблица `test_run_records` — заменена `run_records` (§3.2), **без миграции данных** (greenfield).
- `TestRunRecord.suite_run_id`/`suite_cell_id` (поля 4, 12) и `RunVM.suiteRunId` (`web/src/services/runs.ts`) — `SuiteRunRecord` уже удалена системно (см. §1), эти поля сегодня всегда пустые для живого пути (`RunRecipeWorkflow`/`recipe.go`'s `StartRun` их не заполняет). `Run` их не несёт (см. §9 про matrix-групповой ключ вместо suite).
- `TestRunRecord.spec` (`domain.TestRun`) как поле прогона — классический `TestWorkflow`, который его заполнял, уже не зарегистрирован (§1); `RunConfigTab.tsx`'s рендер `workloadSegments`/`spec` для recipe-рана всегда был пустым (recipe.go's `StartRun` документирует это явно) — SP-E не «ломает» тут ничего, а **чинит** дырку, подставляя `Baked`+`CompiledPlan` (см. §6.C).

**Переиспользуем без изменений:**
- `common.Entity`/`common.Status`/`common.Trigger` — тот же envelope.
- `workflow.RunState` (тип) и весь Temporal live-query механизм (`workflowpb.GetRunStateQueryName`, `GetRunState` query handler) — persist/read логика стадий не меняется, меняется только контейнер (`Run.runtime_state` вместо `TestRunRecord.runtime_state`).
- komeet jsonb-паттерн постгреса (envelope-колонки + `data jsonb`, динамический `->`/`->>` query-билдер) — переносится 1:1 на новую таблицу.
- Observability relay целиком: VictoriaMetrics/VictoriaLogs keyed-by-run-id конвенция, Grafana same-origin relay (project_run_detail_redesign: "Grafana=same-origin /grafana relay, uids hardcoded") — `ObservabilityRefs` документирует контракт, не меняет реализацию.
- `quotas.Manager` (резервирование/коммит/релиз квот, keyed по `tenant_id`+`run_id`) — не завязан на форму `Run`.
- Agent registry / presence (`internal/infrastructure/execution/overview.go`'s `agentPresence`) — не меняется, только сигнатура принимающей функции.
- gateway, connect/ogen/graphql-слой, React web-каркас, `schemapb` (v1.4.4) — как в product-vision §6.

---

## 6. Data-parity чеклист (ОБЯЗАТЕЛЬНО)

Каждая строка = поле/панель, которое сегодня реально доходит до экрана (или до API-ответа), и куда оно переезжает на `Run`. «Preserved» = то же значение, другой путь чтения. «Fixed» = поле СЕГОДНЯ пустое для recipe-рана (задокументированный пробел, §1) и SP-E его заполняет.

### A. Runs-таблица / фасеты (`web/src/services/runs.ts`'s `RunVM`, `api/test_run.proto`'s `ListTestRunsRequest.Sort.Kind` + `ListTestRunFacetsResponse`)

| Поле UI/API | Источник сегодня (`TestRunRecord`) | Источник в `Run` | Статус |
|---|---|---|---|
| id, name, authorId, createdAt, favorite, deleted | `entity.*` | `entity.*` | preserved |
| status | `status` | `status` | preserved |
| trigger | `trigger` | `trigger` | preserved |
| dbKind | `summary.db_kind` | `summary.db_kind` | preserved |
| workload / workloadName | `summary.workload_name` | `summary.workload_name` | preserved |
| stroppyVersion | `summary.stroppy_version` | `summary.stroppy_version` | preserved |
| protocol | `summary.workload_protocol` | `summary.workload_protocol` | preserved |
| topologyLabel | `summary.topology_label` | `summary.topology_label` | preserved |
| nodeCount | `summary.node_count` | `summary.node_count` | preserved |
| provider | `summary.provider` | `summary.provider` | preserved |
| progressPct | `summary.progress_pct` | `summary.progress_pct` | preserved |
| startedAt / finishedAt / durationSec | `summary.{started_at,finished_at,duration}` | `summary.{started_at,finished_at,duration}` | preserved |
| dbPresetId/Name, workloadPresetId, testPresetId/Name | `summary.*` | `summary.*` | preserved |
| recipeId | `recipe_id` | `workflow_id` | renamed (значение — то же `RecipeRecord.entity.id` до SP-B, см. §7/§9) |
| suiteRunId | `suite_run_id` | — | **dropped** (мёртвое поле, §5) |
| Sort: KIND_STATUS/DB_KIND/WORKLOAD/PROVIDER/PROGRESS/DURATION/STARTED_AT/FINISHED_AT/NODE_COUNT/PROTOCOL/TEST_PRESET/TRIGGER | jsonb path в `summary`/top-level | те же path в `Run.summary`/top-level | preserved (`RunRepo.List` копирует `jDBKind`/`jProvider`/... 1:1 на новый префикс) |
| Facets: author_ids, stroppy_versions, db_preset_ids, workload_preset_ids, test_preset_ids | `entity.author_id`, `summary.*` | то же | preserved |

### B. RunDetail — сайдбар (`RunInfoSidebar.tsx`)

| Поле | Источник сегодня | Источник в `Run` | Статус |
|---|---|---|---|
| status, id | `overview.status`/`overview.runId` (Temporal-derived) | не меняется (`OverviewReader` строит `monitor.Overview` из `Run`, не из `TestRunRecord` напрямую в этих полях) | preserved |
| owner/author | `run.authorId` (RunVM) | `run.entity.author_id` | preserved |
| created | `run.createdAt` | `run.entity.timings.created_at` | preserved |
| started / finished | `overview.{startedAt,finishedAt}` | те же (Overview строится из `Run.summary`/`runtime_state`) | preserved |
| trigger | `run.trigger` | `run.trigger` | preserved |
| suite | `run.suiteRunId` | — | **dropped** (§5) |
| db kind, topology label, db preset, test preset | `run.{dbKind,topologyLabel,dbPresetId,testPresetId}` | `run.summary.*` | preserved |
| workload name, protocol, stroppy version, workload preset | `run.{workload,protocol,stroppyVersion,workloadPresetId}` | `run.summary.*` | preserved |

### C. RunDetail — вкладки (`VIEW_OPTIONS` в `RunDetail.tsx`)

| Вкладка | Payload | Источник сегодня | Источник в `Run` | Статус |
|---|---|---|---|---|
| Overview | `monitor.Overview` (live/persisted) | `OverviewReader.Get`/`overviewFromRecord`/`mergeOverviewWithRecord` над `TestRunRecord` | те же функции над `Run` (`.status`/`.runtime_state`/`.summary`) | preserved (сигнатуры меняются, семантика — нет) |
| Pipeline | `monitor.PipelineView` | live Temporal query, fallback `pipelineFromRecord(TestRunRecord.runtime_state)` | live Temporal query (не меняется), fallback из `Run.runtime_state` | preserved |
| Topology | `topology.Topology` | `topologyFromRecordWithRunState(TestRunRecord.recipe_topology, runtime_state)` | `topologyFromRecordWithRunState(Run.topology, Run.runtime_state)` | preserved (тип `RunTopology`, §4) |
| Agents | `monitor.WorkerInfo[]` | `workersFromRecord(TestRunRecord)` + agent registry presence | `workersFromRecord(Run)` + agent registry (не меняется) | preserved |
| Quotas | `QuotaReservationView[]` | `quotas.Manager`, keyed `tenant_id`+`run_id` (не читает `TestRunRecord`-поля) | не меняется | preserved (тривиально) |
| Logs | `monitor.LogLine[]` | VictoriaLogs, keyed `run_id` (=`entity.id`) | `ObservabilityRefs.logs_query_key` (=`entity.id`, §4) | preserved (тривиально) |
| Metrics | `monitor.RunMetrics` | VictoriaMetrics, keyed `run_id` | `ObservabilityRefs.metrics_query_key` | preserved (тривиально) |
| Grafana | relay iframe | same-origin `/grafana` relay, hardcoded uid + `run_id` var | `ObservabilityRefs.grafana_dashboard_uid` (пока = тот же хардкод, §4) | preserved (тривиально) |
| Config (`RunConfigTab.tsx`) | workload segments / full spec | `domain.TestRun spec.workload.segments` — **для recipe-рана ВСЕГДА пусто** (задокументированный пробел, §1) | `Run.baked` (значения формы, когда есть — SP-D) + `Run.compiled_plan` (job/service/params компилятора) | **fixed** — новый источник данных, реальный контент вместо пустой вкладки |

### D. Compare (`Compare.tsx`, `api/compare.proto`)

| Поле `RunColumn` | Источник сегодня | Источник в `Run` | Статус |
|---|---|---|---|
| run_id, name, status | `entity.id`/`entity.name`/`status` | то же | preserved |
| db_kind, db_name (=dbPresetName) | `summary.db_kind`/`summary.db_preset_name` | `summary.*` | preserved |
| workload_name, stroppy_version, provider | `summary.*` | `summary.*` | preserved |
| topology_label, node_count | `summary.*` | `summary.*` | preserved |
| started_at, duration | `summary.*` | `summary.*` | preserved |
| `monitor.Comparison` (метрики) | VictoriaMetrics, keyed `run_id` | не меняется | preserved (тривиально) |

### E. Rating (`api/rating.proto`)

| Поле `RatingEntry` / `RatingFilter` | Источник сегодня | Источник в `Run` | Статус |
|---|---|---|---|
| metric_value/unit | VictoriaMetrics-агрегация | не меняется | preserved |
| db_kind, workload_name, stroppy_version, provider, topology_label, node_count | `summary.*` | `summary.*` | preserved |
| run_at, run_id, author_name, tenant_name | `summary.started_at`/`entity.{id,author_id,tenant_id}` | то же | preserved |
| ранжирование по `in_tenant_rating`/`in_global_rating` | top-level флаги `TestRunRecord` | top-level флаги `Run` | preserved |
| `RatingFilter.{db_kinds,stroppy_versions,providers,started_after/before}` | `summary.*` фильтры | `summary.*` | preserved |

### F. Favorite / Share / Tenant-dashboard

- `internal/infrastructure/adapters/favorite.go` — favorite keyed `entity.id`, читает `entity.name`/`status` для отображения строки — preserved, механическая правка типа.
- `internal/infrastructure/adapters/share.go` — share-снепшот прогона; читает подмножество полей для публичной страницы (без auth-полей типа `author_name`/`tenant_name`, аналогично `public_rating.proto`) — preserved, поля берутся из `Run.entity`/`Run.summary`, не из выпиленных `spec`/`suite_*`.
- `internal/services/tenant_dashboard` — "recent runs"/"top benchmarks" виджеты читают `summary.*` + rating-флаги — preserved.

**Итог:** единственные два реальных изменения относительно сегодняшнего дисплея — (1) `suite_run_id`/`suite_cell_id` дропаются (уже мёртвые поля, ничего не отображается по ним для живого пути); (2) `RunConfigTab`'s config-вкладка получает реальные данные вместо пустой (fix, не regression). Всё остальное — 1:1 перенос значения на новый путь чтения. Это и есть требуемый data-parity ≥ ref.

---

## 7. Зависимости

- **Upstream:** SP-A (`schemapb.Baked` — тип и `Bake`-контракт для поля `Run.baked`; SP-E определяет слот, не производит значение).
- **Параллельно (без жёсткой зависимости, per product-vision §7 `SP-A → (SP-B ∥ SP-E)`):** SP-B (catalog `Workflow`) — `Run.workflow_id` заведён как имя вперёд на будущее, но до SP-B его значением остаётся текущий `RecipeRecord.entity.id` (тот же `RecipeService.StartRun(recipe_id)`-путь). Когда SP-B ставит настоящий catalog `Workflow`, меняется ТОЛЬКО то, что именно пишется в `workflow_id` — форма `Run` не меняется.
- **Downstream:** SP-D (заполняет `Run.baked` через launch-форму — до этого поле `nil`, RunConfigTab использует только `compiled_plan`, §6.C). SP-F (execution shape-up — читает `Run` для per-run логов/провайдер-кредов, не меняет модель).

---

## 8. Тестирование

- **E1/E2** (proto + postgres): golden-тест `deriveRunSummary`/`deriveRecipeTopology` → тот же `Summary`/`RunTopology` shape, что сегодня даёт `TestRunRecord.Summary`/`RecipeTopologySnapshot` для тех же входов (regression-гарантия при переносе). `RunRepo.List` — тот же тестовый набор фильтров/сортировок/фасетов, что `test_run_list_test.go` сегодня гоняет против `TestRunRepo`, продублированный на новую таблицу.
- **E3** (write-path): unit-тест `RunRecipeWorkflow` через `testsuite` (Temporal test env) — проверить, что `persistRunCompiledPlan` пишет `Run.compiled_plan` сразу после compile-стадии (новая точка), остальные persist-точки не регрессируют (существующие тесты `runrecipe_test.go`-типа должны продолжать проходить после смены типа записи).
- **E4** (читатели): для каждой строки чеклиста §6 — снапшот-тест: один и тот же `Run` fixture → `OverviewReader.Get`/`compare`/`rating`/`RunVM`-mapper дают то же (или явно улучшенное, §6.C) значение, что дал бы эквивалентный `TestRunRecord` fixture сегодня. Web: `runs.ts`/`run_overview.ts` mapper unit-тесты на fixture JSON (protojson) нового `Run`.
- **E2E**: живой docker recipe-run на dev-стенде (см. project_dsl_pivot memory) — после миграции прогон должен так же зелено пройти compile→provision→execute(nomad)→teardown→COMPLETED, и RunDetail должен отрисовать ВСЕ вкладки (включая теперь непустой Config) без деградации.

---

## 9. Открытые вопросы (для плана SP-E)

1. **`ListTestRunsRequest` vs `recipe.proto`'s `ListRunsRequest`:** сегодня это два похожих, но не идентичных сообщения (`test_run.proto`'s — полный facet/sort/paging контракт, используемый 4 внутренними вызывающими сторонами; `recipe.proto`'s — тонкая обёртка с `recipe_id`). Схлопнуть в одно при рефакторинге под `Run` или оставить дублирование как есть? Затрагивает ogen/REST-поверхность.
2. **`workflow_version`:** чем стабильно идентифицировать версию бандла на момент запуска — `RecipeRecord.version` (uint32, уже есть, видно в `recipe.go`'s `StartRun` construction `"v" + strconv.FormatUint(...)`)? Или ждать SP-B's catalog-версионирование (git-коммит/тег)? Для v1 достаточно текущего `RecipeRecord.version`.
3. **`ObservabilityRefs` — заполнять сразу или оставить документирующей заглушкой?** Функционально v1 не нуждается в per-run значениях (все бэкенды уже keyed by `entity.id`, Grafana uid — relay-константа). Заполнить полями-константами (`= entity.id`) в SP-E, или оставить `nil` до SP-F, когда uid/query-key реально станут per-provider/per-run? Предлагается: заполнять в SP-E (дёшево, не ломает контракт), реальную вариативность добавляет SP-F.
4. **Matrix-группировка (workflow.yaml `matrix:`) вместо suite:** `SuiteRunRecord` выпилена, но product-vision §3.2 вводит `matrix` в `workflow.yaml` — множественные `Run` из одного launch (напр. по db-параметру). Нужен ли `Run` группирующий ключ (`batch_id`) для UI-фильтрации "показать все раны этого запуска формы", или каждый `Run` из matrix — независимая строка без группировки в v1? Не специфицировано в product-vision — решить при планировании SP-D (matrix — часть launch-потока), SP-E оставляет место (см. `reserved 14 to 20` в §4) на случай, если поле понадобится.
5. **Совместное proto-сообщение `Run` vs раздельные read-model/write-model:** `RunRecipeWorkflow` пишет через Temporal-детерминированные activity-обёртки (`persist*`), читатели (`OverviewReader`) читают напрямую из постгреса. Одно proto-сообщение `Run` (как сегодня `TestRunRecord`) остаётся источником истины для обоих путей — альтернатива (раздельные write-DTO/read-projection) не рассматривается: усложнение без обоснованной выгоды при текущем масштабе.
