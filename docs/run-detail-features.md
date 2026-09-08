# Run Detail — полный инвентарь функциональности (спека для реимплементации)

Снято с кода `main @ cf16c9c0` (2026-09-02), страница `web/src/pages/RunDetail.tsx` и всё, что она рендерит и вызывает: сайдбар, 8 вкладок, кнопки шапки, публичная share-страница, compare, бэкенд за каждым RPC. Цель документа — при любой переделке UI/модели восстановить страницу «не беднее», зная **зачем** каждая фича и **как** она сделана сейчас (ссылки `file:line` на этот коммит).

Экран, с которого снят инвентарь: шапка (`← Runs`, имя + статус, id · PERSISTED · «as of»), кнопки Refresh / Rerun / New from / Save preset / Share / Compare / Shell / Delete (+ Cancel для живых), сайдбар RUN INFO (Identity / Database / Workload / Infrastructure / Runtime / Progress), вкладки Overview / Pipeline / Topology / Agents / Quotas / Logs / Metrics / Grafana, селектор «window» для Metrics/Grafana.

## Оглавление

1. Шапка, действия, вкладки Agents и Quotas, сквозные механики
2. Сайдбар RUN INFO, вкладка Overview, слой данных overview
3. Вкладки Pipeline и Topology
4. Вкладки Logs, Metrics, Grafana, селектор window
5. Сводка: что сломано или отсутствует сейчас

Каждая фича описана по схеме: **Зачем** → **Как сделано** (компонент/функция, `file:line`) → **API/proto** → **Сервер** → **Крайние случаи**.

Общий каркас: один RPC-снапшот `TestRunOverviewService.GetTestRunOverview` питает шапку, сайдбар, Pipeline, Topology, Agents и селектор window; пока ран живой — `StreamTestRunOverview` (server-stream, 1 кадр/с) с fallback на polling 5 с. Logs, Metrics, Grafana, Quotas ходят своими RPC. Активная вкладка и все фильтры логов живут в URL (`?view=…`).

---

# 1. Шапка, действия, Agents, Quotas, сквозные механики


Источник: `web/src/pages/RunDetail.tsx` @ main `cf16c9c0`. Все пути — относительно корня репо `stroppy-cloud/`.
Бэкенд: connectrpc-сервисы `internal/services/*`, порты/адаптеры `internal/infrastructure/*`, протоколы `protocols/cloud/v1/*`.

Общая механика страницы (нужна везде ниже):
- `tenantSlug = useTenantSlug()` из URL `/t/:slug/runs/:id` (`web/src/lib/router.tsx:70-73`). Все `Link`/`useNavigate` из `@/lib/router` автоматически префиксуют абсолютный путь `/runs` → `/t/<slug>/runs` (`router.tsx:53-60`, исключения `NON_TENANT_PREFIXES = /login,/select-tenant,/admin,/profile,/orgs,/share,/t/`).
- Все RPC-вызовы делают `resolveTenantId(tenantSlug)` (slug → tenant_id) и передают `tenantId` в запросе.
- Данные шапки берутся из `OverviewVM` (`services/run_overview.ts:148-170`), который строится из `TestRunOverviewService.GetTestRunOverview` (одноразово, `load()`), а пока ран не терминальный — из `StreamTestRunOverview` (server-stream, полный снапшот на каждый тик; при ошибке стрима — fallback на поллинг `getRunOverview` каждые 5 с) (`RunDetail.tsx:251-268`).

---

### 1. Header (шапка)

#### 1.1 Back link «← Runs»
- **Зачем**: возврат к таблице ранов.
- **Как сделано**: `RunDetail.tsx:424-426` — `<Link to="/runs">` (через tenant-префикс → `/t/<slug>/runs`). Иконка `ArrowLeft`, `text-xs text-muted-foreground`.
- **Крайние случаи**: чистый `Link`, состояние фильтров таблицы не сохраняется (Runs.tsx держит фильтры в собственном URL).

#### 1.2 Статус: точка + Badge
- **Зачем**: моментальное чтение состояния рана.
- **Как сделано**: `RunDetail.tsx:428-430`. Рендерится только если `status` есть (`overview?.status`).
  - `STATUS_DOT` (`:65-72`): `pending→bg-pending`, `running→bg-primary animate-pulse`, `cancelling→bg-warning animate-pulse`, `completed→bg-success`, `failed→bg-destructive`, `cancelled→bg-pending`.
  - `STATUS_VARIANT` (`:56-63`): `pending→"pending"`, `running→"default"`, `cancelling→"warning"`, `completed→"success"`, `failed→"destructive"`, `cancelled→"warning"`. Текст бейджа — сам строковый статус.
- **Тип `RunStatus`** (`services/dashboard.ts:15-21`): `"pending"|"running"|"cancelling"|"completed"|"failed"|"cancelled"`.
- **Маппинг из proto `cloud.v1.common.Status`** — `statusToVM` (`dashboard.ts:161-184`):
  - `STATUS_PENDING`, `STATUS_UNSPECIFIED`, любой неизвестный → `pending`
  - `STATUS_RUNNING`, `STATUS_ALLOCATED`, `STATUS_DEPLOYMENT`, `STATUS_RETRY_WAIT` → `running`
  - `STATUS_CANCELLING` → `cancelling`
  - `STATUS_COMPLETED`, `STATUS_DEPLOYED` → `completed`
  - `STATUS_FAILED` → `failed`
  - `STATUS_CANCELLED`, `STATUS_SKIPPED` → `cancelled`
- **Сервер (откуда статус в overview)**: `internal/infrastructure/execution/overview.go:98-160` `OverviewReader.Get`:
  1. читает `TestRunRecord` из БД (`store.RunRecord`), топологию, suite-run (если `suite_run_id`);
  2. `agentPresence` из реестра агентов;
  3. если запись терминальная (`persistedRecordIsTerminal`) — overview строится **только из записи** (`overviewFromRecord`, source=`PERSISTED_RECORD`);
  4. иначе — `tc.GetRunState` (Temporal query к workflow `testWorkflowID(runID)` с таймаутом `runStateQueryTimeout`); при ошибке — fallback на запись + `degradedReasons += "workflow_state_unavailable: …"`;
  5. `mergeOverviewWithRecord` (`overview.go:333+`): если в записи `CANCELLING` или терминальный статус — статус записи **перекрывает** живой (чтобы после `CancelTestRun` UI сразу показал `cancelling`).
- **Крайние случаи**: до появления workflow (`rec == nil`) — `pendingOverview` (source=`SYNTHETIC`, status=`PENDING`). Пока ран идёт, `Duration` пересчитывается от `observedAt`, а не берётся из записи (`overview.go:300-306`, «duration выглядел замороженным»).

#### 1.3 Имя рана + fallback
- `RunDetail.tsx:429`: `run?.name || \`Run ${id.slice(0, 8)}\``. `run` = `overview.run` = `testRunRecordToVM(snapshot.run)` (`services/runs.ts:372+`), имя = `entity.name`.
- **Сервер**: имя минтится при `StartTestRun` — `specName(spec)` (`internal/services/test_run/helpers.go:88-93`): `"stroppy " + workload.stroppy_version` либо `"test run"`. Визард может задать своё имя через draft.

#### 1.4 Строка id + chip источника + «as of»
- `RunDetail.tsx:432-440`, моно `text-[11px]`:
  - `id` (полный uuid из URL-параметра).
  - Chip источника показывается только если `overview.source && source !== "temporal"`. `SOURCE_LABEL` (`:96-102`): `temporal→"live"`, `persisted→"persisted"`, `plan→"plan"`, `registry→"registry"`, `synthetic→"synthetic"`. title=`"data source"`. Стиль: `border px-1 text-[10px] uppercase`.
  - `ObservationSource` маппинг (`run_overview.ts:260-267`): `OBSERVATION_SOURCE_TEMPORAL_RUN_STATE→temporal`, `_PERSISTED_RECORD→persisted`, `_DEPLOYMENT_PLAN→plan`, `_AGENT_REGISTRY→registry`, `_SYNTHETIC→synthetic`, иначе `""`.
  - `· as of {relTime(observedAt)}` с `title=observedAt` (ISO). `observedAt` — серверные часы момента сборки снапшота (`overview.go:101 timestamppb.Now()`).
- `relTime(iso)` (`:109-118`): `—` если пусто/NaN/год<2000; `<60s → "Ns ago"`, `<1h → "Nm ago"`, `<1d → "Nh ago"`, иначе `"Nd ago"`. Считается один раз при рендере (не тикает сам, обновляется с новым снапшотом).
- **Зачем chip**: сигнал, что данные не «живые» из Temporal (например, после завершения — persisted; до старта — synthetic).

#### 1.5 Баннеры error / notice / degraded
- `RunDetail.tsx:488-498`, все `shrink-0`, под шапкой:
  - `error` (красный `border-destructive/40 bg-destructive/10`): любая ошибка `load()` или действий (`setError(err.message)`).
  - `notice` (зелёный `border-success/40`): `flash(msg)` (`:292-296`) — ставит notice, сбрасывает error, авто-скрывает через **2500 мс** (`setTimeout`).
  - degraded (жёлтый `border-warning/40`, моно `text-[11px]`): `Degraded snapshot: {degradedReasons.join("; ")}` когда `overview.degradedReasons.length > 0`.
- **Сервер (degradedReasons)**: `overview.go:120-155`: `"agent_registry_unavailable: <err>"`, `"workflow_state_unavailable: temporal_client_not_configured"`, `"workflow_state_unavailable: <err>"`. Поле proto `monitor.Overview.degraded_reasons`.
- **Крайние случаи**: notice и error не взаимоисключающие визуально (оба могут висеть), но `flash` сбрасывает error.

#### 1.6 Breadcrumb label
- `RunDetail.tsx:194`: `useBreadcrumbLabel("id", overview?.run?.name || (id ? \`Run ${id.slice(0,8)}\` : undefined))`.
- Механика (`web/src/lib/breadcrumbs.tsx:187-205`): контекст `BreadcrumbContext`, реестр `CRUMB_REGISTRY` (`:74-97`) содержит `{pattern: "/t/:slug/runs/:id", label: overrides.id ?? params.id}`. Хук регистрирует override по имени параметра, снимает на unmount. Пока `undefined` — показывается сырой uuid.
- Трейл: `<org switcher> / Test Runs / <run name>`.

---

### 2. Кнопки действий и их гейтинг

Расположение: правая часть шапки, `flex flex-wrap gap-2` (`RunDetail.tsx:443-485`). Все `Button variant="outline" size="sm"`. Порядок: **Refresh, Rerun, New from, Save preset, Share, Compare, Shell, Cancel, Delete**.

#### 2.0 Матрица гейтинга — `actionsForStatus(status)` (`services/runs.ts:319-329`)
```
status      | view | share | clone | cancel | rerun | extract | delete
pending     |  ✓   |   ✓   |   ✓   |   ✓    |       |         |   ✓
running     |  ✓   |   ✓   |   ✓   |   ✓    |       |         |
cancelling  |  ✓   |   ✓   |   ✓   |        |       |         |
completed   |  ✓   |   ✓   |   ✓   |        |   ✓   |    ✓    |   ✓
failed      |  ✓   |   ✓   |   ✓   |        |   ✓   |         |   ✓
cancelled   |  ✓   |   ✓   |   ✓   |        |   ✓   |         |   ✓
```
Код: `canCancel = running|pending`; `inFlight = canCancel|cancelling`; `terminal = completed|failed|cancelled`; `cancel` при canCancel; `rerun` при terminal; `extract` только `completed`; `delete` при `!inFlight || pending` (т.е. всё кроме running/cancelling).
- В RunDetail: `allowed = status ? actionsForStatus(status) : new Set()` (`:236`) — **пока overview не загружен, гейтящиеся кнопки скрыты** (в отличие от таблицы Runs, где они disabled-with-reason). Негейтящиеся (Refresh, New from, Share, Compare, Shell) видны всегда.
- `busy` (общий флаг) дизейблит Rerun/Save preset/Share/Cancel/Delete во время любого действия. Refresh дизейблится по `loading`.
- Провайдер: `getRunsProvider()` (`runs.ts:670-678`) → по умолчанию `realRunsProvider` (`:582-664`), инжектится мок через `setRunsProvider`. Все методы → `testRunClient` (connect, `cloud.v1.api.TestRunService`).

#### 2.1 Rerun
- **UI** (`RunDetail.tsx:315-326`): `getRunsProvider().rerunRun(slug, id)` → `flash("Run re-launched")` → `navigate("/runs")` (уходит на таблицу; новый id не показывается). Без confirm.
- **RPC**: `TestRunService.StartTestRun { tenantId, source: {case:"testRunId", value: runId} }` (`runs.ts:617-623`). Auth: `RESOURCE_TEST_RUN/ACTION_CREATE`. Не идемпотентен.
- **Сервер** (`internal/services/test_run/test_run.go:29-107`):
  1. `Runs.Get(tenant, test_run_id)` — исходная запись (репо `Get` возвращает и soft-deleted); если `spec == nil` → `FailedPrecondition "source run has no spec to re-run"`.
  2. `cloneSpec` (proto.Clone) — **тот же baked spec** (database + workload + infrastructure_plan + render_overrides и т.д.), **без пере-bake**.
  3. `MaterializeTestRunPackages` — ре-резолв пакетов из каталога (`InvalidArgument` при ошибке).
  4. **Новый id** `uuid.NewString()` пишется и в `spec.Id`; `spec.ValidateAll()`.
  5. Новый `TestRunRecord`: `Name = specName(spec)` (НЕ имя исходного рана), `AuthorId = caller`, `Status = PENDING`, `Trigger = MANUAL` (для `run`-источника было бы `API`), `InTenantRating` default true, `InGlobalRating` default false, `Summary = Summarizer.Summarize(spec)`; `suite_run_id` пустой (suite-дети создаются только suite-сервисом).
  6. `Runs.Create` в serializable tx, затем **вне tx** `Workflows.LaunchTest(rec)`; при ошибке запуска запись помечается FAILED (`finishFailedRun`) и возвращается `Internal`.
- **Крайние случаи**: перезапуск удалённого (soft-deleted) рана технически возможен (Get не фильтрует). Прежний ран не трогается. Связи «родитель→ребёнок» нет.

#### 2.2 Save preset (extract)
- **UI** (`RunDetail.tsx:328-338`): `extractToPreset(slug, id)` → `flash("Saved as preset")`. Остаёмся на странице; ссылка на пресет не показывается.
- **RPC**: `TestRunService.ExtractToPreset { tenantId, id: runId, name: "" }` (`runs.ts:625-628`). Auth: `RESOURCE_PRESET/ACTION_CREATE`. Не идемпотентен — каждое нажатие создаёт новый пресет.
- **Сервер** (`test_run.go:288-338`):
  - Требует `spec.database` и `spec.workload` (иначе `FailedPrecondition "run has no database+workload to extract"`).
  - Имя: `req.name` либо `derivePresetName(rec)` (`helpers.go:96-101`): `"<run name> preset"` или `"preset from run <id>"`.
  - Создаётся `TestPresetRecord { Entity{id=uuid, tenant, name, author=caller}, Test{Database, Workload, Tags}, IsSystem=false }` через `Presets.CreateTestPreset`.
  - **Не** переносятся: provider, infrastructure_plan, machine overrides, render_overrides (test-preset = params-only, provider-agnostic — см. память `project_test_workflow_design`).
- **Где виден**: Library → `/t/<slug>/presets/test` (`TestPresets`), `presets/test/:id`.
- **Крайние случаи**: гейт только `completed` на UI; сервер разрешает любой статус со spec.

#### 2.3 Cancel
- **UI** (`RunDetail.tsx:340-358`): `useConfirm({ title:"Cancel run?", description:"The run will be stopped. Already-provisioned resources are torn down during teardown.", danger:true, confirmLabel:"Cancel run" })` → `cancelRun` → `flash("Cancel requested")` → `await load()` (перечитать overview; статус станет `cancelling`).
- **RPC**: `TestRunService.CancelTestRun { tenantId, id }` (`runs.ts:612-615`). Auth: `RESOURCE_TEST_RUN/ACTION_UPDATE`. Идемпотентен.
- **Сервер** (`test_run.go:157-213`):
  1. В tx: `Runs.Get`; если статус терминальный или уже `CANCELLING` — no-op, вернуть запись.
  2. Иначе `Status = CANCELLING`, `touchUpdated`, `Runs.Update`.
  3. Вне tx: `Workflows.CancelTest(runID)` (`internal/infrastructure/execution/test_workflows.go:108-130`): Temporal `CancelWorkflow(testWorkflowID(runID))`; затем `DescribeWorkflowExecution` — если не RUNNING → `ErrNotFound`.
  4. Если `ErrNotFound` (workflow нет/уже закрыт) — `finishCancelledRun`: сразу `markCancelled` (status=CANCELLED, StartedAt/FinishedAt = now если пусты, Duration).
- **Семантика teardown** (`internal/workflows/test.go:340-431`): отмена контекста Temporal → `defer` в workflow: если `teardownPlan != nil` (инфра была поднята) — teardown на **disconnected context** (доводится до конца несмотря на cancel), затем `releaseCommittedAllocations` (квоты/сети → RELEASED, стадия `teardown` action `release_quotas` order 90); если инфра не поднялась, но квоты зарезервированы и не закоммичены — `ReleaseQuotasActivity` на disconnected ctx (`:401-421`). После — `w.cancel(ctx)` ставит `STATUS_CANCELLED` всем незавершённым стадиям и persist.
- **Крайние случаи**: `cancelling` — кнопка скрыта (нельзя повторно). Ран `pending` без workflow → мгновенно `cancelled`. Пока идёт teardown, статус в UI — `cancelling` (overview берёт статус записи, `overview.go:339-343`).

#### 2.4 Delete
- **UI** (`RunDetail.tsx:360-376`): `confirm({ title:"Delete run?", description:"“<name|id>” will be permanently removed. This cannot be undone.", danger:true, confirmLabel:"Delete" })` → `deleteRun` → `navigate("/runs")`. При ошибке `busy=false` и error-баннер (успех не сбрасывает busy — страница уходит).
- **RPC**: `TestRunService.DeleteTestRun { tenantId, id }`. Auth: `RESOURCE_TEST_RUN/ACTION_DELETE`. Идемпотентен.
- **Сервер** (`test_run.go:231-286`) — **soft delete**:
  1. `Runs.Get`; NotFound → пустой успех.
  2. Если не терминальный — best-effort `Workflows.CancelTest`; `ErrNotFound` → `cancelMissing=true`; другая ошибка → `Internal`.
  3. В tx: перечитать; если уже `deleted_at` — no-op; если не терминальный: `cancelMissing ? markCancelled : Status=CANCELLING`; `markDeleted` → `entity.timings.deleted_at = now`; `Runs.Update`.
  - **Что удаляется**: только флаг `deleted_at` на `TestRunRecord`. Snapshot/spec/runtime_state остаются в строке. Метрики (VictoriaMetrics) и логи (VictoriaLogs) **не удаляются**. Share-снапшоты живут отдельно («share survives the run being deleted», `models/share.proto`). Квоты освобождаются workflow-ом при отмене.
  - После удаления `GetTestRun` возвращает NotFound (`test_run.go:112-115`), `ListTestRuns` скрывает без `filter.include_deleted`.
- **UI-текст «permanently removed» неточен** — это soft delete; таблица Runs умеет показать удалённые (`?del=1`).

#### 2.5 Гейтинг кнопок в UI vs сервер
- UI прячет; сервер — источник истины (идемпотентные no-op для Cancel/Delete, FailedPrecondition для Rerun/Extract без spec).

---

### 3. Refresh
- `RunDetail.tsx:444-446`: `onClick={() => void load()}`, `disabled={loading}`, иконка `RefreshCw` с `animate-spin` при loading.
- `load()` (`:196-210`): `setLoading(true); setError(null); overview = await getRunOverview(slug, id)`; **метрики не перечитывает** (их тянет отдельный эффект по окну сегмента: initial + каждые 12 с пока не терминальный, `:272-290`). Квоты тоже не трогает (у Quotas свой Refresh).
- `getRunOverview` (`run_overview.ts:540-547`): `TestRunOverviewService.GetTestRunOverview { tenantId, runId }` → `snapshotToVM(toJson(snapshot), snapshot.run, runId)`. Auth `RESOURCE_TEST_RUN/READ`; сервер `authorizeRun` (caller + tenant + `Runs.Exists`) (`internal/services/test_run_overview/service.go:157-172`).
- Крайние случаи: во время активного стрима Refresh просто делает лишний one-shot; live-стрим продолжает.

---

### 4. «New from» → `/runs/new?from=<id>`
- **UI**: `RunDetail.tsx:452-456`, `<Link to={\`/runs/new?from=${id}\`} title="Open the wizard pre-filled with this run's parameters">`, иконка `CopyPlus`. Не гейтится статусом.
- **NewRun.tsx**: `fromRunId = params.get("from")` (`:223-224`). Эффект (`:310-330`): если есть `slug && fromRunId && !draftId` и не запускалось (`cloneStartedRef` против StrictMode double-fire) → `getWizardProvider().start(slug, "Untitled run", { sourceRunId })` → `setDraft(d)`; URL заменяется на `?draft=<id>&step=database` (`replace:true`, чтобы reload/back не клонировал заново). Пока клонируется — экран `"Cloning run…"` (`:440-447`). Ошибка → error + сброс ref (можно повторить).
- **RPC**: `TestWizardService.StartTestWizard { tenantId, name:"Untitled run", testPresetId:"", sourceRunId }` (`services/wizard.ts:1085-1094`).
- **Сервер** (`internal/services/test_wizard/service.go:232-283`): `source_run_id` имеет приоритет над `test_preset_id`; `RunSpecs.Get(tenant, runId)`; `spec==nil` → `FailedPrecondition`; `Engine.InitialDraftFromRun(tenant, name, spec)`; далее сервис ставит Entity (id, author) и `Drafts.Create`.
- **Что переносится** (`internal/infrastructure/execution/test_wizard_engine.go:104-132`): `Database` (clone), `Workload` (clone), `RenderOverrides` (clone), `Provider = infrastructure_plan.provider`, `MachineOverrides = infrastructure_plan.machines[]` (per-node sizing). Затем `Compute` пересчитывает topology_spec / infrastructure_plan / render_preview / errors / ready. Имя рана, теги, рейтинг-флаги, trigger — не переносятся.
- **Отличие от Rerun**: Rerun = тот же baked spec без правок; New from = редактируемый draft, пересобираемый визардом (свежий bake).
- **История**: `5eac7221` feat(wizard): «New from run» — seed a draft from an existing run's spec (source_run_id); ранее `5ba92296` «rerun button: pre-fills NewRun with previous run's config».

---

### 5. Share

#### 5.1 Кнопка Share (создание)
- **UI** (`RunDetail.tsx:300-313`): `createShare(slug, id, { kind:"test_run" })` → если нет `share.token` — ошибка `"Share token was not returned"`; URL = `new URL(\`/shared/${encodeURIComponent(token)}\`, location.origin)` → `navigator.clipboard.writeText` → `flash("Public share link copied")`. Диалога/TTL-выбора нет. Не гейтится статусом. Каждое нажатие — **новый share** (новый токен).
- **Сервис** (`services/shares.ts:88-109`): `ShareService.CreateShare { tenantId, target:{kind: KIND_TEST_RUN|KIND_SUITE_RUN, id}, ttl?: Duration }` — RunDetail не передаёт ttl → серверный дефолт. Ответ `ShareRecord` → `ShareVM {id,name,token,targetKind,targetId,expiresAt,revoked,createdAt,capturedAt,snapshotName}`.
- **Сервер** (`internal/services/share/service.go:150-199`), auth `RESOURCE_SHARE/ACTION_CREATE`:
  - Валидация target (kind ≠ UNSPECIFIED, id ≠ "").
  - Токен: `RandomTokenMinter` (`internal/infrastructure/adapters/share.go:19-45`) — 32 случайных байта (256 бит; минимум 16), `base64.RawURLEncoding`.
  - `expiryFromTTL` (`:128-137`): `ttl == nil` → **7 дней** (`defaultShareTTL`); `ttl == 0` → никогда (nil); `ttl > 0` → now+ttl.
  - В serializable tx: `Snapshots.Build(tenant, target)` (верифицирует что ран существует **и принадлежит tenant**, иначе NotFound) → `ShareRecord{Entity{id,tenant,author}, Target, Token, ExpiresAt, Revoked:false, Snapshot}` → `Shares.Create` (уникальность токена — на стороне стора, `ErrConflict`).
- **Snapshot builder** (`adapters/share.go:86-140`, `RunSnapshotBuilder.Build/sharedTestRun`) — **замороженная allowlist-проекция**:
  - `SharedTestRun`: name, status, db_kind, db_name (=summary.db_preset_name), workload_name, stroppy_version, provider, topology_label, node_count, started_at, finished_at, duration, progress_pct.
  - `Metrics` = `RunMetrics` через `MetricsReader.Get(runId, nil)` — если доступно (ошибка → без метрик).
  - `WorkloadSegments[]` (`:257-286`): name, script, vus, duration, iterations, quiet, no_thresholds, pool_size, scale_factor, insert_method, bulk_size, steps, no_steps. **Не** читаются: `parameters.env`, `segment.sql`, files, `execution.extra_args`.
  - `Database` (`:291-355`): version + типизированные ключи по движку: postgres (`replicas, sync_replicas, haproxy, pgbouncer, patroni, etcd`), orioledb (`image, shared_buffers_mb, replicas, haproxy`), mysql/mariadb (`replicas, proxysql, group_replication, semi_sync`), picodata (`instances, replication_factor, shards, haproxy`), ydb (`storage_nodes, database_nodes, pdisks_per_storage_node, storage_groups, auto_size_pdisks, haproxy`), cockroach (`nodes`). Free-form `*_options` карты не читаются. Нулевые значения опускаются; если ни version, ни settings — `nil`.
  - `Machines[]` (`:145-196`): только для Yandex-машин из `spec.infrastructure_plan.machines[].yandex`: node_id, cores, memory_gb, boot_disk_gb, boot_disk_type, platform (`yandexPlatformName`, enum → `standard-v3`-подобные lowercase), zone (машины или дефолт из settings), secondary_disks[{size_gb,type}]. Docker-машины пропускаются. Provider settings (token/ssh key) не читаются, кроме enum platform/zone. Тест `share_projection_test.go` пиннит отсутствие утечек.
  - Suite-run: `SharedSuiteRun{name,status,provider,db_kinds,total,completed,failed,running,progress_pct,started_at,finished_at,duration}`.
- **ВАЖНО (крайний случай)**: proto-комментарий и код обещают «background job» обновления снапшота, но **фонового refresh-а в репо нет** (grep по `Snapshots.Build` — единственный вызов в `CreateShare`). Снапшот фиксируется в момент Share; для идущего рана публичные числа/статус не обновятся (кроме live Grafana, см. 5.3).

#### 5.2 Прочие RPC ShareService (есть в сервисе, **нет в UI**)
- `services/shares.ts`: `listShares(slug, targetId="")` → `ListShares{tenantId,targetId,filter,sort,page}`; `getShare`; `revokeShare(id)` → `RevokeShare` (auth `RESOURCE_SHARE/UPDATE`, идемпотентно, `revoked=true`); `deleteShare` → hard delete строки (идемпотентно); `setShareExpiry(id, ttlSec)` → `SetShareExpiry{ttl}` (та же семантика ttl: 0 = never).
- **Ни одна страница web/src не вызывает list/revoke/delete/setExpiry** (grep: только `share_pb.ts`). Для реимплементации: нужен UI управления шарами рана (список, revoke, expiry) — сейчас созданный share нельзя отозвать из интерфейса.

#### 5.3 Публичная страница `/shared/:token` — `web/src/pages/SharedRun.tsx`
- Роут вне `RequireAuth` (`App.tsx:96`). Два независимых запроса при монтировании:
  1. `getSharedRun(token)` → `PublicShareService.GetSharedRun{token}` (без auth). Сервер (`internal/services/public_share/service.go:88-123`): `Shares.GetByToken`; unknown/revoked/expired/без снапшота → единый `NotFound "share not found, revoked, or expired"` (`gone()`); `isLive` = `!revoked && (expires_at == nil || now < expires_at)`.
  2. `startSharedSession(token)` → `GET /public/share/{token}/session` (gateway, `internal/gateway/publicmetrics.go:255-315`): резолвит токен (`ShareScopeResolver`, `adapters/share_scope.go:40-70`: только `KIND_TEST_RUN`), ставит **HttpOnly cookie `stroppy_share=<token>`** (`Path=/`, `MaxAge=12h`, `SameSite=Lax`, `Secure` при https), возвращает `{runId, from?, to?}` (unix millis из snapshot.started_at/finished_at; `to` отсутствует пока ран идёт). Ошибка → `session=null` → вкладка Grafana не показывается (деградация до замороженных чисел).
- **Layout «lab report»** (коммит `6bad5fc6`): full-bleed (`1dbd0c01`); header: имя (`"Unnamed run"` fallback), `Badge status`, `dbKind · workloadName`, справа `Snapshot taken <capturedAt>`; `FactStrip` (provider, topology, stroppy, started, elapsed, + progress если <100%); `Signature` — копируемая моно-строка `engine version · nodes=N · key=val… · <факты последнего сегмента>` (последний сегмент = измеряемый, bootstrap — setup).
- **Табы** (`d6e1badf`): `Overview` (только test_run) → `SharedConfig`: колонки Workload (сегменты как чипы `script vus=… dur=…/iters=… pool=… scale=… insert=… bulk=… quiet no-thresholds` + `steps: a → b`), Database (`version`, key/value или «Defaults, unchanged.»), Infrastructure (per-VM `N vCPU · M GB · 50GB network-ssd +100GB … · platform · zone`); `Metrics` → общий `MetricsPanel` c `run.metrics`; `Grafana` (test_run && session) → `SharedGrafana`: переключатель дашбордов `dashboardsForRun(dbKind)`, iframe `buildEmbedUrl(active,{runId,dbKind,startedAt,finishedAt})` высотой `calc(100vh-7rem)`.
- **Scoping Grafana** (`fe93de8a`, `publicmetrics.go:17-46`): Grafana-org `public` с единственным datasource → gateway `/public/metrics/*`; Grafana форвардит cookie (`keepCookies`); gateway: cookie→share→run; **удаляет** клиентские `extra_label`, `extra_filters[]`; форсит `extra_label=stroppy_run_id=<run>`; клампит `start/end/time` к окну рана; разрешает только VM read endpoints (`api/v1/query`, `query_range`, `series`, …). Правка `var-run_id` в iframe ничего не даёт.
- **Связанный fix** `cf16c9c0`: авторизованный RunDetail использует тот же путь — `GrafanaSession` RPC (`test_run_overview.proto:642`, `RESOURCE_TEST_RUN/READ`) выдаёт HMAC run-scope token, фронт кладёт его в тот же cookie `stroppy_share` (`ensureGrafanaSession`, `run_overview.ts:530-535`, `max-age=43200`, не HttpOnly) до монтирования iframe.
- **Крайние случаи**: suite-run share → только вкладка Metrics (пустые метрики) и header; старые снапшоты без `workloadSegments`/`machines` → секции скрыты; `!loading && !error && !run` → «This link no longer works…».

---

### 6. Compare → `/compare?runIds=<id>`
- **UI-кнопка** (`RunDetail.tsx:465-469`): `<Link to={\`/compare?runIds=${id}\`}>`, иконка `GitCompare`; не гейтится. Роут `/t/:slug/compare` (`App.tsx:145`).
- **Страница** `web/src/pages/Compare.tsx`:
  - Читает `?runIds` (fallback `?runs`), текстовое поле `run-a, run-b, run-c` (разделители `[\s,;]+`), submit → `setSearchParams({runIds: ids.join(",")})` + `load`. С одним id (как приходит из RunDetail) → **ошибка «At least two run IDs are required.»** — пользователь должен дописать id вручную; выбора из списка/пикера нет.
  - Baseline = первый id. Секции: плитки (BaselineTile + SummaryTile на каждый другой ран: badge `better/worse`, счётчики Better/Worse/Same/Missing/N/A); таблица **Run config** (Status, Database, DB preset, Workload, Stroppy, Provider, Topology, Nodes, Duration) — **без диффа spec**, только summary-поля; таблица **Metrics** с фильтром по name/key/unit/group/description, группировка `GROUP_ORDER = throughput, latency, errors, connections, replication, resources, system, stroppy, other`, sticky первая колонка; ячейка: `avg`, `max`, badge verdict (`better/worse/same`, `n/a` → «not comparable», `missing`) + `±X.X%`.
- **Сервис** `services/compare.ts`: `CompareService.CompareRuns{tenantId, runIds[]}` → `CompareVM{columns[], metrics[], summaries[]}`.
- **Сервер** (`internal/services/compare/compare.go`), auth `RESOURCE_TEST_RUN/READ`, NO_SIDE_EFFECTS: 2..16 id, непустые, уникальные; в tx на каждый — `Runs.Get(id)` + `assertTenant` (чужой tenant → NotFound, не PermissionDenied); `runColumn(rec)` = `RunColumn{run_id,name,status,db_kind,db_name(=db_preset_name),workload_name,stroppy_version,provider,topology_label,node_count,started_at,duration}` из summary; `Metrics.Compare(runIds)`.
- **MetricsComparator** (`internal/infrastructure/adapters/compare.go:64-167`): грузит `RunMetrics` каждого рана (nil-окно = весь ран), объединяет ключи (порядок baseline, потом новые first-seen), на каждую метрику `MetricRow{key,name,unit,higher_is_better,group,description, cells[]}`; `MetricCell{run_id,avg,max,diff_avg_pct,diff_max_pct,verdict,present,diff_*_defined}`; `relDiffPct = (v-b)/b*100` (b=0 → defined только если v=0); `verdict`: `|diff| <= 1.0%` (`sameThresholdPct`) → SAME, иначе по `higher_is_better` BETTER/WORSE; baseline-ячейка всегда SAME; отсутствие у baseline → `NotComparable++`, отсутствие у рана → `Missing++`. `Comparison{run_ids, range, metrics[≤512], summaries[]{run_id,better,worse,same,missing,not_comparable}}`. Ошибка NotFound если у рана нет метрик.
- **Крайние случаи**: нет сравнения конфигов/spec-диффа (только summary); нет выбора окна/сегмента; deleted-раны сравниваются (Get не фильтрует).

---

### 7. Shell → `/shell?runId=<id>` — **МЁРТВАЯ ССЫЛКА**
- **UI** (`RunDetail.tsx:470-474`): `<Link to={\`/shell?runId=${id}\`}>` + иконка `Terminal`, всегда видна. Через tenant-префикс → `/t/<slug>/shell?runId=…`.
- **Роута нет**: в `App.tsx:138-168` под `/t/:slug` нет `shell`; страницы `Shell*.tsx` нет ни в `pages/`, ни в `old/pages/`; `xterm` не в зависимостях; `agentShellClient` создан (`services/client.ts:153`), но нигде не используется. Попадание в `*` → `RootRedirect` → редирект на `/t/<first tenant>` (дашборд). Кнопка добавлена в `954e9934 "ui"` (2026-06-03) как задел.
- **Серверная часть существует и готова** — `internal/services/agent_shell/service.go`, `cloud.v1.api.AgentShellService.OpenShell` (bidi stream `ShellClientFrame ⇄ ShellServerFrame`), auth `RESOURCE_AGENT_SHELL/ACTION_CREATE`:
  - Первый кадр обязателен `ShellStart{tenant_id*, run_id, machine_id*, component_id, cols, rows, shell}` (`protocols/cloud/v1/api/agent_shell.proto:29-59`).
  - `Tenants.EnsureMember(caller, tenant)` (admin bypass), `Machines.Locate(tenant, machine, component)` (NotFound / FailedPrecondition если агент offline), `Hub.Open(machineID, agent.OpenShell{run_id,component_id,cols,rows,shell})` — PTY на агенте через control stream `agent/shell.proto Connect`, мультиплекс по session_id.
  - Audit: `ShellAuditEntry{id,session_id,account,tenant,run,machine,component,shell,opened_at}` в tx; в конце `Audit.End(session, exit_code, exit_err, at)`.
  - Bridge: клиент→агент `Stdin(bytes)`, `Resize{cols,rows}`, `Close`; агент→клиент `Stdout`, `Stderr`, `Exit{code,error}`. Второй `Start` — InvalidArgument.
- **Для реимплементации**: страница/дровер с выбором машины из `overview.workers` (machineId, presence=online), xterm.js + connect bidi stream, передача `runId` из query. Connect-web bidi поверх HTTP/1.1 не работает — нужен HTTP/2 или websocket-мост.

---

### 8. Вкладка Agents
- **UI** (`RunDetail.tsx:586-637`): grid 2 колонки (`lg:grid-cols-2`), `scrolls=true` (overflow-auto, p-3).
  - **Таблица workers**: колонки `Worker | Kind | Presence | Last seen | Version`. Worker: `w.id || "—"` моно + под ним `w.host || w.machineId`. Kind: `w.kind || "—"`. Presence: `<PresenceBadge presence online>` с `title=statusReason`. Last seen: `relTime(lastSeenAt)` с `title=lastSeenAt`. Version: `agentVersion || "—"`. Пусто → «No workers.».
  - `PRESENCE` (`:87-94`): `online→success "online"`, `stale→warning "stale"`, `offline→destructive "offline"`, `terminated→pending "terminated"`, `unknown/""→pending "unknown"`. `PresenceBadge` fallback: если presence не в карте — по `online` булю.
  - **Timeline**: `<ul>` `max-h-[60vh] overflow-y-auto`, события сортируются по `sequence` asc; строка: точка `SEVERITY_DOT` (`info→bg-primary, warning→bg-warning, error→bg-destructive, ""→bg-muted-foreground`), `fmtTime(at)` (`timeStyle:"medium"`, локаль), `e.kind` uppercase c цветом `SEVERITY_TEXT`, `e.message`, опц. `e.detail` моно 10px. Пусто → «No events.».
  - **Крайний случай**: `e.kind` — это сырое имя proto-enum (`KIND_STAGE_STARTED`, `KIND_RUN_STATUS` …) — маппинга в человекочитаемое нет (`run_overview.ts:501 kind: e.kind ?? ""`).
- **VM** (`run_overview.ts:104-146`): `WorkerVM{id,kind,machineId,host,online,status,currentNodeExecutionId,presence,statusReason,source,registeredAt,lastSeenAt,heartbeatIntervalSeconds,agentVersion}`; `TimelineEventVM{at,kind,message,nodeExecutionId,status,severity,source,sequence,detail}`. Маппинг из JSON снапшота `:483-509`; enum-маппинги `workerPresence` (`WORKER_PRESENCE_ONLINE|STALE|OFFLINE|TERMINATED|UNKNOWN`), `eventSeverity` (`EVENT_SEVERITY_INFO|WARNING|ERROR`).
- **Proto**: `monitor.WorkerInfo` (`protocols/cloud/v1/monitor/overview.proto`), `monitor.Event` (`:303-341`): `at, kind{STAGE_STARTED=1, STAGE_COMPLETED=2, STAGE_FAILED=3, STAGE_RETRYING=4, WORKER_ONLINE=5, WORKER_OFFLINE=6, RUN_STATUS=7}, message(≤4096), node_execution_id, status, source, severity, sequence, detail(≤4096)`.
- **Сервер — workers** (`internal/infrastructure/execution/overview.go:420-500`):
  - `workersFromRecord(rec, presence)`: список нод из записи (`workerMachineIDs`), для каждой: `Id="agent/"+nodeId`, `Kind=KIND_AGENT`, `Host=machineHost(infrastructure_state.machines[node])`, `CurrentNodeExecutionId` из deployment_plan, `Presence=recordWorkerPresence` (терминальный ран → `TERMINATED`, иначе `UNKNOWN`), `StatusReason` (`run_terminal_no_registry_sample` / `machine_not_materialized` / `no_registry_sample`), `Source=PERSISTED_RECORD`.
  - `mergeWorkerPresence(worker, sample)` — накладывает сэмпл реестра: host, registeredAt, lastSeenAt, heartbeatIntervalSeconds, agentVersion, runId; если запись уже `TERMINATED` — presence остаётся TERMINATED, `online=false`, reason `run_terminal_agent_terminated`; иначе presence/online/reason из сэмпла, `Source=AGENT_REGISTRY`.
  - **Presence по heartbeat** (`internal/infrastructure/execution/agent_registry.go:190-207`, `agentHeartbeatInterval = 15s`, интервал может прийти от агента): `lastSeenAt` пустой → `UNKNOWN "agent_registered_without_heartbeat"`; `age <= 2*interval` → `ONLINE "heartbeat_recent"`; `age <= 6*interval` → `STALE "heartbeat_stale_<age>"`; иначе `OFFLINE "heartbeat_missed_<age>"`. Реестр — in-memory карта на сервере (`s.mu`), `Heartbeat` RPC агента (`:106-134`).
  - Ошибка реестра → `degradedReasons += agent_registry_unavailable`, воркеры без сэмплов.
- **Сервер — timeline** (`overview.go:925-1090`, `projectOverview` из Temporal `RunState`): на каждую стадию: `STAGE_STARTED` (at=started_at, status=RUNNING, severity INFO) и, если finished_at — `stageEndKind(status)` (`COMPLETED→STAGE_COMPLETED, FAILED→STAGE_FAILED, RETRY_WAIT→STAGE_RETRYING`, иначе STAGE_COMPLETED) с `eventSeverity(status)` (`FAILED|CANCELLED→ERROR`, `RETRY_WAIT|CANCELLING→WARNING`, иначе INFO); плюс одно событие `RUN_STATUS` «run status: <STATUS>» на earliestStart (или now). Сортировка по `at`, `sequence = i+1`. `detail` **никогда не заполняется** сервером (grep `Detail:` — 0). `WORKER_ONLINE/OFFLINE` — не генерируются.
  - Для терминального рана (persisted fallback) `overviewFromRecord`: если есть `runtime_state` — timeline проецируется из него тем же кодом (source=PERSISTED_RECORD); если нет — `Timeline: []` (пусто).
- **Крайние случаи**: после завершения рана все воркеры → `terminated` даже если агент ещё шлёт heartbeat; `lastSeenAt` для persisted-only воркера отсутствует → «—».

---

### 9. Вкладка Quotas
- **UI** (`RunQuotaUsagePanel`, `RunDetail.tsx:646-738`): грузится лениво — `useEffect` при `view === "quotas"` (`:230-232`), состояние `quotaRows/quotaLoading/quotaError` отдельно от overview; кнопка Refresh → `loadQuotas()`.
  - Плитки `QuotaStat`: Rows (всего), Reserved, Allocated, Released (подсчёт по `row.status`; EXPIRED в плитках не считается).
  - Таблица (`min-w-[900px]`, sticky thead): `Node | Quota | Status | Amount | Created | Expires`. Node=`row.nodeId`; Quota: `info.name` моно + `resourceType · resourceId`; Status badge `quotaStatusVariant/Label` (`ALLOCATED→success "allocated"`, `RESERVED→default "reserved"`, `FAILED→destructive "failed"`, `EXPIRED→warning "expired"`, `RELEASED→pending "released"`, иначе `pending "unknown"`); Amount = `Intl.NumberFormat(Number(bigint))` + `info.units`; Created/Expires = `formatQuotaTimestamp` (`dateStyle:short, timeStyle:short`, `—` если пусто). Пусто → «No quota usage for this run.» / «Loading quota usage...». Внизу красный баннер `N quota reservation row(s) failed.` при failed>0.
  - Поля `service`, `runId`, `updatedAt` не отображаются.
- **Сервис** (`services/quotas.ts:44-48`): `getQuotaProvider().getRunQuotaUsage(slug, runId)` → `QuotaService.GetRunQuotaUsage{tenantId, runId}` → `reservations: QuotaReservationView[]` (сырые proto-сообщения, без VM).
- **Proto** (`protocols/cloud/v1/api/quota.proto:103-116`): `QuotaReservationView{id, run_id, node_id, info: deployment.Quota.Info{provider, name, units}, resource_type, resource_id, service, amount: uint64, status: deployment.Quota.ReservationStatus, created_at, updated_at, expires_at}`. Статусы (`deployment/quota.proto:23-30`): `RESERVED=1, ALLOCATED=2, RELEASED=3, EXPIRED=4, FAILED=5`.
- **Сервер** (`internal/services/quota/service.go:79-98`), auth `RESOURCE_TEST_RUN/READ`: `ValidateAll`, `authorizeTenant` (caller + tenant существует), `Runs.Get(tenant, runId)` (NotFound если чужой/нет), `Quotas.RunUsage` → `store.ListRunReservations` (`quotas/store.go:197+`, все статусы, `order by node_id, quota_name`) → `ReservationsToViews`.
- **Что за квоты**: это **собственный ledger** (`quota_reservations`) поверх **провайдерских квот** (снимки `quota_snapshots` от источников провайдера: Yandex Cloud — `compute.instances.count`, `compute.instanceCores.count`, `compute.instanceMemory.size`, `compute.hddDisks.size`, `compute.ssdDisks.size` и т.п. по `cloud_id` из tenant settings; Docker — scope `docker_host/local`). Не tenant-квоты. Scope = (tenant, provider, resource_type, resource_id) (`quotas/manager.go:283-315`). Units по имени если провайдер не дал: `.count→count`, `.size→GiB`, `.rate→rate`.
- **Жизненный цикл** (workflow `internal/workflows/test.go` + `deployment_activities.go:92-134` + `quotas/manager.go` + `store.go`):
  1. Стадия `infrastructure`, action 1 `calculate_quotas` → план + `QuotaRequestRef[] {node_id, request{info,request}}` по машинам.
  2. Action 2 `acquire_quotas` → `Manager.Reserve`: refresh снапшота если stale (`SnapshotTTL` 5 мин, env `QuotaSnapshotTTL`); в serializable tx с lock scope: сначала **`RESERVED` с истёкшим `expires_at` → `EXPIRED`** (`store.go:229-241`); проверка `provider_available - sum(чужие RESERVED живые + ALLOCATED новее observed_at) >= amount` иначе `QUOTA_INSUFFICIENT` (FailedPrecondition, стадия FAILED); insert/upsert строк `status=RESERVED, expires_at = now + ReservationTTL` (**30 мин**, env `QuotaReservationTTL`), `workflow_id`. Одна строка на (run, node_id, quota_name).
  3. После успешного provision — action 5 `commit_quotas` → `CommitRun`: `RESERVED → ALLOCATED` (без expires-семантики).
  4. Teardown (успех/ошибка/cancel) — `release_quotas` (order 90) → `ReleaseRun`: `RESERVED|ALLOCATED → RELEASED`. Если инфра не поднялась, а резерв был — release на disconnected ctx.
  - `FAILED` — **нигде сервером не выставляется** (нет non-pb сеттера); зарезервирован в enum. `EXPIRED` ставится лениво при следующем Reserve любого рана в том же scope (фонового экспайрера нет).
- **Крайние случаи**: docker-раны с `Quotas==nil` в активити — `echoQuotaAllocations` (эхо без ledger) → таблица пуста; таблица пуста и для ранов до стадии infrastructure; `expires_at` остаётся в строке после ALLOCATED (UI показывает «Expires» даже у allocated — семантически уже не действует).

---

### 10. Сквозные механики

#### 10.1 View-state в URL
- `type View = overview|pipeline|topology|logs|metrics|quotas|grafana|agents` (`RunDetail.tsx:120`); `VIEW_OPTIONS` (`:122-131`) с иконками lucide: Overview `SlidersHorizontal`, Pipeline `Workflow`, Topology `Network`, Agents `Activity`, Quotas `Gauge`, Logs `ScrollText`, Metrics `LineChart`, Grafana `BarChart3`. Порядок именно такой.
- `view = sp.get("view") ?? "overview"` (`:153`). `setView(v)` (`:154-167`): `next.set("view", v)`; **`next.delete("line")`** (скопированный якорь строки лога — транзиентный, сбрасывается при смене вкладки); `{ replace: true }` — смена вкладки **не** добавляет history-запись. Остальные параметры (`steps, comp, mach, unit, src, stream, phase, action, mention, q` — фильтры логов) сохраняются, т.е. ссылка `?view=logs&steps=…&q=error` шарибельна.
- `openLogsForNode(nodeExecutionId)` (`:170-192`): из Pipeline → `view=logs`, `steps=<id>`, удаляет все прочие лог-фильтры и `line`; `{ replace: false }` (история пишется — back вернёт в pipeline).
- `scrolls = overview|metrics|quotas|agents` (`:379`) — эти вкладки `overflow-auto p-3`; остальные (`pipeline, topology, logs, grafana`) сами управляют высотой (`overflow-hidden`).
- Селектор «window» (`:531-548`) показывается только на `grafana|metrics` при наличии сегментов; state `segmentSel` **не** в URL.

#### 10.2 Grafana lazy mount
- `grafanaSeen` (`:384-387`): ставится в true при первом `view==="grafana"`; `GrafanaPanel` монтируется один раз (`overview && grafanaSeen`) в `absolute inset-0` и далее **скрывается классом `hidden`**, а не размонтируется — повторное открытие мгновенное, iframes не перезагружаются. Не префетчится при загрузке (каждый iframe грузит полный Grafana). Остальные вкладки в соседнем div получают `hidden` при `view==="grafana"` (`:564`). Props: `tenantSlug, runId, dbKind=run?.dbKind, startedAt/finishedAt = окно сегмента либо рана, workers`.

#### 10.3 Ресайз сайдбара
- `SIDEBAR_KEY = "stroppy.runDetail.sidebarWidth"` (localStorage) (`:390`). Начальное: `parseInt(localStorage)`, валидно если `220..560`, иначе **300**. Каждый апдейт пишет обратно (`useEffect`, `:395-397`).
- Drag (`:398-417`): `onMouseDown` на разделителе (`w-2 cursor-col-resize`, визуальная линия `w-px bg-border`, hover → `primary/60`, active → `primary`) запоминает `{x: clientX, w}`; `mousemove` на window → `clamp(220, 560, w + dx)`; `mouseup` снимает слушатели. Только мышь (нет touch/keyboard).
- Сайдбар: `<div style={{width}} className="min-h-0 shrink-0 overflow-auto">` с `RunInfoSidebar overview run` или плейсхолдер `Loading…/No data`.

#### 10.4 useConfirm
- `web/src/components/ui/confirm-dialog.tsx`: `ConfirmProvider` монтирует один `Dialog` в корне; `confirm(opts): Promise<boolean>`. `ConfirmOptions{title, description?: ReactNode, confirmLabel?, cancelLabel?, danger?}`; defaults: confirmLabel = `"Delete"` если danger иначе `"Confirm"`, cancel = `"Cancel"`. danger → иконка `AlertTriangle` в красном круге и `variant="destructive"` кнопка. Закрытие по overlay/Esc → `false`. Resolve откладывается через `queueMicrotask` для анимации закрытия. Бросает, если вне провайдера.

#### 10.5 SegmentedControl
- `web/src/components/ui/segmented.tsx`: generic `<T extends string>`, `options[{value,label,icon?}]`, `value`, `onChange`, `variant: "boxed"|"tabs"`, `segmentClassName` (ширина сегмента; в RunDetail `w-[88px]`, variant `tabs`). Индикатор — абсолютный `span` с `width = (100% - 2*pad)/n`, `left = pad + idx*(…)`, transition `300ms cubic-bezier(0.22,1,0.36,1)`, `bg-primary/15 ring-primary/40`. Кнопки `font-mono text-[10px] uppercase tracking-wider`, активная `text-primary`. Контейнер в RunDetail: `h-12 shrink-0 p-2`.

#### 10.6 Live-обновление и терминальность
- `terminal = completed|failed|cancelled` (`:237`) — выключает стрим overview и интервал метрик; `cancelling` считается «в полёте» (стрим продолжается до реального CANCELLED).
- Ошибка стрима (например, connect без h2) → поллинг каждые 5 с (`:255-263`); AbortController на unmount/смену id.

---

### Сводка для реимплементации — что ломано/отсутствует сейчас
1. **Shell** — кнопка ведёт в никуда (нет роута/страницы); сервер `AgentShellService.OpenShell` готов.
2. **Compare** из RunDetail открывается с одним id → сразу ошибка; нет пикера второго рана.
3. **Share** — нет UI списка/revoke/expiry (RPC есть); снапшот не обновляется фоном (обещанный job не реализован); TTL всегда дефолтные 7 дней.
4. **Delete** — текст «permanently removed» при soft-delete.
5. **Agents timeline** — `kind` сырым enum-именем; `detail` сервером не заполняется; `WORKER_ONLINE/OFFLINE` не генерируются.
6. **Quotas** — `FAILED` статус не используется сервером; `EXPIRED` выставляется лениво; docker-раны без ledger дают пустую таблицу.
7. **Rerun** — не показывает/не открывает новый ран (уходит на список), имя нового рана = `stroppy <ver>`, не имя исходного.
8. Гейтинг кнопок — скрытие вместо disabled-with-reason (в Runs.tsx — наоборот).

---

# 2. Сайдбар RUN INFO + Overview + слой данных


Инвентаризация для переимплементации. Репо `stroppy-cloud` @ main (cf16c9c0).
Пути указаны от корня репо. Строки — на момент чтения.

---

### 0. Карта файлов

| Слой | Файл | Роль |
|---|---|---|
| UI | `web/src/pages/RunDetail.tsx` | страница: загрузка snapshot, стрим/поллинг, header, sidebar, переключатель view, select «window» |
| UI | `web/src/components/run/RunInfoSidebar.tsx` | левая колонка RUN INFO (Identity / Database / Workload / Infrastructure / Runtime / Progress) |
| UI | `web/src/components/run/RunConfigTab.tsx` | вкладка Overview (WORKLOAD card, DATABASE card, FULL SPEC (RAW)) |
| data | `web/src/services/run_overview.ts` | `OverviewVM`, `MetricVM`, `getRunOverview`, `streamRunOverview`, `getRunMetrics`, `workloadSegments`, маппинг proto→VM |
| data | `web/src/services/runs.ts` | `RunVM`, `WorkloadSegmentVM`, `testRunRecordToVM` (TestRunRecord → плоский VM + decoded `spec`) |
| data | `web/src/services/dashboard.ts:161` | `statusToVM` (common.Status → 6 UI-статусов) |
| data | `web/src/services/enums.ts` | `dbKindLabelFromJson`, `providerLabelFromJson`, `protocolLabelFromJson`, `triggerLabelFromJson` |
| data | `web/src/lib/author-display.ts` | `resolveAuthorDisplay` (IamService.GetAccount → nickname/email) |
| data | `web/src/services/client.ts:125-133` | connect-web transport (JSON, interceptors auth+refresh), `testRunOverviewClient`, `testRunClient` |
| proto | `protocols/cloud/v1/api/test_run_overview.proto` | `TestRunOverviewService`: GetTestRunOverview / StreamTestRunOverview / GetRunMetrics / … ; `TestRunOverviewSnapshot` |
| proto | `protocols/cloud/v1/monitor/overview.proto` | `monitor.Overview`, `PipelineView/PipelineNode`, `WorkerInfo`, `Event`, enum'ы `ObservationSource`, `WorkerPresence`, `EventSeverity` |
| proto | `protocols/cloud/v1/monitor/metrics.proto` | `RunMetrics`, `MetricSummary`, `TimeRange` |
| proto | `protocols/cloud/v1/models/test_run.proto` | `TestRunRecord` (entity, spec, status, trigger, suite_run_id, summary, infrastructure_state, deployment_plan, runtime_state) |
| proto | `protocols/cloud/v1/domain/test.proto`, `workload.proto`, `database.proto` | `domain.TestRun` (= `spec`), `Workload{segments[]}`, `Database{kind, source oneof}` |
| proto | `protocols/cloud/v1/workflow/test.proto` | `RunState{status, stages[]}`, `Stage` — Temporal query `GetRunState` |
| proto | `protocols/cloud/v1/common/status.proto` | `common.Status` (12 значений) |
| server | `internal/services/test_run_overview/service.go` | gRPC-сервис: `authorizeRun`, `GetTestRunOverview`, `StreamTestRunOverview`, `GetRunMetrics`, `streamOverview` |
| server | `internal/services/test_run_overview/connect.go` | Connect-хендлер (тот же svc; стрим через `connect.ServerStream.Send`) |
| server | `internal/infrastructure/execution/overview.go` | **`OverviewReader`** — сборка snapshot: Temporal query → fallback на persisted record → synthetic; проекция `RunState`→`monitor.Overview`; workers; timeline; progress |
| server | `internal/infrastructure/execution/agent_registry.go:137-207` | `AgentPresence` — registry heartbeat → `WorkerPresence` (online/stale/offline/unknown) |
| server | `internal/infrastructure/execution/run_persistence.go` | `PersistRunState` (runtime_state + summary.started/finished/duration/progress), `applyRunSummary` |
| server | `internal/infrastructure/execution/summarizer.go` | `RunSummarizer.Summarize` — статические поля `summary` при старте (db_kind, workload_name, stroppy_version, protocol, node_count, provider, topology_label, db_preset_id) |
| server | `internal/infrastructure/execution/monitoring.go:280-420` | `MetricsReader.Get` — окно + VictoriaMetrics |
| server | `internal/domain/metrics/collector.go` | `Collector.Collect` / `summarize` (p5–p95 trim, avg/min/max/last) |
| server | `internal/workflows/test.go`, `stages.go` | TestWorkflow: 5 root-стадий, стадии сегментов workload, throttle персиста runtime-проекции (20s) |
| wiring | `internal/app/run.go:167, 416` | `execution.NewOverviewReader(tc, snapReader, agentRegistry)`; `TestRunOverviewDeps{Runs: store.TestRuns(), Overview, Logs, Metrics, GrafanaScopeSigner}` |

---

### 1. Транспорт и API

#### 1.1 RPC
Сервис `cloud.v1.api.TestRunOverviewService` (`test_run_overview.proto:428`). Все методы гейтятся `RESOURCE_TEST_RUN / ACTION_READ` (аннотация `(cloud.v1.iam.auth)`, auth-интерцептор; сервис сам permission не перепроверяет).

| RPC | Тип | Request | Response | Использование в зоне |
|---|---|---|---|---|
| `GetTestRunOverview` | unary, `NO_SIDE_EFFECTS` | `{tenant_id, run_id}` | `{snapshot: TestRunOverviewSnapshot}` | первичная загрузка, Refresh, poll-fallback |
| `StreamTestRunOverview` | server-stream | `{tenant_id, run_id}` | `stream TestRunOverviewSnapshot` | live-обновление пока run не terminal |
| `GetRunMetrics` | unary | `{tenant_id, run_id, window?: monitor.TimeRange}` | `{metrics: monitor.RunMetrics}` | вкладка Metrics + select «window» (данные, не рисуются в sidebar/Overview, но зависят от pipeline overview) |
| `GrafanaSession` | unary | `{tenant_id, run_id}` | `{run_id, scope_token}` | `ensureGrafanaSession` — не в этой зоне |

Дублируется в REST через ogen (`internal/proto/cloud/v1/api/admin.ogenadapter.go:1248, 2244, 983`): `GET /api/v1/test-run-overview/get-test-run-overview`, `POST …/stream-test-run-overview`, `GET …/get-run-metrics`. Фронт REST не использует.

#### 1.2 Транспорт фронта
`web/src/services/client.ts:125` — `createConnectTransport({ baseUrl, interceptors: [refreshRetryInterceptor, authInterceptor] })` из `@connectrpc/connect-web`. Значит: Connect-протокол поверх fetch, **JSON**-кодирование (useBinaryFormat не задан), unary = POST `/cloud.v1.api.TestRunOverviewService/GetTestRunOverview`, server-stream = POST `…/StreamTestRunOverview` с enveloped-фреймами в теле ответа (не SSE, не WebSocket). `Authorization: Bearer <accessToken>`; 401 → single-flight Refresh и повтор.

#### 1.3 Серверный гейт: `authorizeRun` (`service.go:154-169`)
1. `Authn.Caller(ctx)` → иначе `Unauthenticated`.
2. `tenant_id == ""` → `InvalidArgument "tenant_id is required"`; `run_id == ""` → `InvalidArgument "run_id is required"`.
3. `Runs.Exists(ctx, tenantID, runID)` (`store.TestRuns()`) — run другого тенанта или несуществующий → `derrors.ErrNotFound` → `NotFound` (существование не утекает).

#### 1.4 Стрим на сервере
- `service.go:190` `StreamTestRunOverview` → `streamOverview(ctx, tenant, run, stream.Send)`; `connect.go:31` — то же с `connect.ServerStream.Send`.
- `streamOverview` (`service.go:294`): `authorizeRun` → `Overview.Stream(ctx, runID)` → цикл `select {ctx.Done → status.FromContextError; snap,ok := <-ch; !ok → return nil (нормальное закрытие); nil-snap пропускается; send}`.
- `OverviewReader.Stream` (`overview.go:179`): горутина; **первый snapshot сразу**, далее `time.NewTicker(streamInterval = 1s)`; каждый тик = полный `Get(ctx, runID)`; остановка (закрытие канала) при: ctx done, ошибке `Get` (hard store error), или `isTerminalStatus(snap.overview.status)` (COMPLETED/FAILED/CANCELLED/SKIPPED). Каждый фрейм — **полный** snapshot, клиент просто заменяет state (без diff-merge; см. комментарий `test_run_overview.proto:173-177`).

#### 1.5 Клиентская стратегия обновления (`RunDetail.tsx:196-290`)
- `load()` (`:196`) — `getRunOverview` → `setOverview`; `loading` флаг; ошибка → `error` баннер. Вызывается при mount и по кнопке Refresh (`:444`) и после Cancel (`:352`).
- Live-эффект (`:251-268`): условие `tenantSlug && id && status && !terminal` (status берётся из уже загруженного overview → до первого snapshot стрима нет). Создаёт `AbortController`, вызывает `streamRunOverview(tenantSlug, id, setOverview, signal)`. Если промис отклонился **и signal не aborted** → `startPoll()`: `setInterval(getRunOverview → setOverview, 5000)` с проглатыванием ошибок. Cleanup: abort + clearInterval. Deps `[tenantSlug, id, status, terminal]` → **при каждой смене status (pending→running и т.п.) стрим переоткрывается**. При terminal — эффект не запускается (стрим закрыт сервером, последний фрейм terminal).
- `streamRunOverview` (`run_overview.ts:863`): `for await (const snap of testRunOverviewClient.streamTestRunOverview({tenantId, runId}, {signal}))` → `toJson(TestRunOverviewSnapshotSchema, snap)` → `snapshotToVM(j, snap.run, runId)` → `onFrame`. AbortError / `signal.aborted` — проглатываются (это штатная остановка), иные ошибки пробрасываются (→ poll-fallback).
- `getRunOverview` (`:537`): `resolveTenantId(slug)` → `getTestRunOverview({tenantId, runId})` → `toJson(GetTestRunOverviewResponseSchema)` → `snapshotToVM(j.snapshot ?? {}, resp.snapshot?.run, runId)`. Note: header-record `run` передаётся **как raw proto** (не JSON), потому что `testRunRecordToVM` сам делает `toJson`.
- Метрики (`:272-290`): fetch при mount и при смене `selSeg.startedAt/finishedAt`; плюс `setInterval(12000)` пока `!terminal`. Стрим overview метрик не несёт.

---

### 2. Proto-структуры, попадающие в UI

#### 2.1 `api.TestRunOverviewSnapshot` (`test_run_overview.proto:127`)
| поле | тип | откуда на сервере | куда во фронте |
|---|---|---|---|
| `run` | `models.TestRunRecord` (required) | `store.RunRecord(runID)` + `overlayRunFromOverview` (при live-пути) | `OverviewVM.run` = `testRunRecordToVM(run)` → sidebar группы Identity/Database/Workload/Infrastructure, вкладка Overview |
| `topology` | `topology.Topology` | `topologyFromRecord[WithRunState]` | `OverviewVM.topology` (Topology view, не эта зона) |
| `overview` | `monitor.Overview` | `OverviewReader.Get` (см. §3) | статус/время/progress/pipeline/workers/timeline/source/observedAt/degradedReasons |
| `suite_run` | `models.SuiteRunRecord` | `store.SuiteRun(run.suite_run_id)` если задан | `OverviewVM.suiteRunId = snap.suiteRun.entity.id` (sidebar использует `run.suiteRunId`, а не это) |

#### 2.2 `monitor.Overview` (`overview.proto:726`)
`run_id`, `status: common.Status`, `started_at`, `finished_at`, `duration: Duration`, `progress_pct: uint32 ≤100`, `pipeline: PipelineView{roots[]}`, `workers: WorkerInfo[]`, `timeline: Event[]`, `observed_at`, `degraded_reasons: string[] ≤32`, `source: ObservationSource`.

`ObservationSource` (`:678`): `UNSPECIFIED=0, TEMPORAL_RUN_STATE=1, PERSISTED_RECORD=2, DEPLOYMENT_PLAN=3, AGENT_REGISTRY=4, SYNTHETIC=5` → фронт `obsSource()` (`run_overview.ts:260`) → `"temporal"|"persisted"|"plan"|"registry"|"synthetic"|""`; UI-лейбл `SOURCE_LABEL` (`RunInfoSidebar.tsx:51`, `RunDetail.tsx:96`): temporal→**"live"**, остальные как есть.

`WorkerPresence` (`:688`): `UNKNOWN=1, ONLINE=2, STALE=3, OFFLINE=4, TERMINATED=5` → `workerPresence()` (`run_overview.ts:269`).

`EventSeverity` (`:698`): `INFO/WARNING/ERROR` → `eventSeverity()` (`:278`).

`PipelineNode` (`:768`) поля, используемые в зоне: `node_execution_id`, `name`, `status`, `started_at`, `finished_at`, `duration`, `children[]`, `phase`, `source`, `order`, `parent_node_execution_id`. (Остальные — Pipeline view.)

`WorkerInfo` (`:914`): `id`, `kind`, `machine_id`, `host`, `online`, `status`, `current_node_execution_id`, `presence`, `registered_at`, `last_seen_at`, `heartbeat_interval_seconds`, `agent_version`, `status_reason`, `source`. Sidebar использует только `presence` + `online`.

`Event` (`:948`): `at`, `kind` (STAGE_STARTED/COMPLETED/FAILED/RETRYING, WORKER_ONLINE/OFFLINE, RUN_STATUS), `message`, `node_execution_id`, `status`, `source`, `severity`, `sequence`, `detail`. Sidebar использует `length` и `severity`.

#### 2.3 `models.TestRunRecord` (`models/test_run.proto:35`)
| поле | в RunVM (`runs.ts:444`) | показ |
|---|---|---|
| `entity.id` | `id` | (header `id`; sidebar берёт `overview.runId`) |
| `entity.name` | `name` | header h1 (`RunDetail.tsx:429`), breadcrumb |
| `entity.author_id` | `authorId` | sidebar **owner** |
| `entity.timings.created_at` | `createdAt` | sidebar **created** |
| `entity.timings.deleted_at` | `deleted` | — |
| `entity.is_favorite` | `favorite` | — |
| `status` | `status` (statusToVM) | не в sidebar (там `overview.status`) |
| `trigger` (`common.Trigger` MANUAL/CRON/API) | `trigger` | sidebar **trigger** |
| `suite_run_id` | `suiteRunId` | sidebar **suite** |
| `summary.db_kind` (`domain.Database.Kind`) | `dbKind` | sidebar Database **kind**, Overview Database **Kind** |
| `summary.workload_name` | `workload` | sidebar Workload **name**, Overview **Name** |
| `summary.stroppy_version` | `stroppyVersion` | **stroppy** / **Stroppy** |
| `summary.workload_protocol` | `protocol` | **protocol** / **Protocol** |
| `summary.topology_label` | `topologyLabel` | **topology** / **Topology** |
| `summary.node_count` | `nodeCount` | Infrastructure **nodes** / Overview **Nodes** |
| `summary.provider` (`deployment.Provider` DOCKER/YANDEX) | `provider` | Infrastructure **provider** / Overview **Provider** |
| `summary.progress_pct` | `progressPct` | не используется (sidebar берёт `overview.progressPct`) |
| `summary.started_at/finished_at/duration` | `startedAt/finishedAt/durationSec` | не используются в sidebar (берётся overview) |
| `summary.db_preset_id` | `dbPresetId` | Database **preset** (8 символов) / Overview **Preset** |
| `summary.workload_preset_id` | `workloadPresetId` | Workload **preset** / Overview **Preset** |
| `summary.test_preset_id` | `testPresetId` | Database **test** |
| `spec` (`domain.TestRun`) | `spec` (весь JSON) + `workloadSegments[]` | Overview tab целиком, sidebar сегменты |
| `runtime_state` (`workflow.RunState`) | — | сервер: fallback-проекция |
| `infrastructure_state`, `deployment_plan` | — | сервер: workers, topology, pipelineFromRecord |

`summary` заполняется: статика — `RunSummarizer.Summarize` при Start (`summarizer.go:23`): `DbKind=spec.database.kind`, `WorkloadName = "stroppy " + stroppy_version` (иначе ""), `StroppyVersion`, `WorkloadProtocol`, `NodeCount = len(spec.topology_spec.nodes)`, `Provider = spec.infrastructure_plan.provider`, `TopologyLabel = spec.topology_spec.labels["label"]`, `DbPresetId` только если `database.source == database_preset_id`. Динамика — `applyRunSummary` (`run_persistence.go:250`): `StartedAt = min(stage.started_at)`, `FinishedAt = max(stage.finished_at)` **только если status terminal**, `Duration = spanDuration(started, finished, now)`, `ProgressPct = round(terminalStages/totalStages·100)`.

#### 2.4 `domain.TestRun` = `spec` (`domain/test.proto:42`)
`id`, `suite_id`, `database: Database`, `workload: Workload`, `topology_spec`, `infrastructure_plan`, `render_overrides`, `tags`. Всё это попадает в FULL SPEC (RAW) как camelCase JSON (`toJson`).

`Workload` (`workload.proto`): `stroppy_version`, `protocol`, `segments[]{name, script, sql, execution{vus?, duration|iterations (oneof limit), quiet?, no_thresholds, extra_args[]}, parameters{pool_size, scale_factor, default_insert_method, env{}, steps[], no_steps[], bulk_size}, files[]{name, kind, content}}`, `tags`.

`Database` (`database.proto`): `kind`, oneof `source`: `params: DatabaseParams{version, package: Package{id,name,db_kind,db_version,is_builtin,apt_packages,pre_install,custom_repo,custom_repo_key,deb_filename,package_record_id}, oneof engine: postgres|mysql|mariadb|picodata|ydb|ydb_managed|cockroach|orioledb|noop|pg_noop}` | `external{dsn, tags}` | `database_preset_id{id}`; `package_id`; `tags`. Engine-params содержат `*_options` map<string,string> (postgresql.conf, haproxy, pgbouncer, patroni, etcd, my.cnf, proxysql, picodata instance, ydb storage/database, orioledb postgres, cockroach options, pg_noop options).

#### 2.5 `common.Status` → UI (`dashboard.ts:161 statusToVM`)
| proto | UI |
|---|---|
| `STATUS_PENDING`, `STATUS_UNSPECIFIED`, default | `pending` |
| `STATUS_RUNNING`, `STATUS_ALLOCATED`, `STATUS_DEPLOYMENT`, `STATUS_RETRY_WAIT` | `running` |
| `STATUS_CANCELLING` | `cancelling` |
| `STATUS_COMPLETED`, `STATUS_DEPLOYED` | `completed` |
| `STATUS_FAILED` | `failed` |
| `STATUS_CANCELLED`, `STATUS_SKIPPED` | `cancelled` |

#### 2.6 `monitor.RunMetrics` (`metrics.proto:71`)
`run_id`, `range: TimeRange{start,end}`, `metrics: MetricSummary[]{key, name, unit, avg, min, max, last, higher_is_better, description, group}`. Фронт `MetricVM` (`run_overview.ts:173`) 1:1, `NaN/Infinity` → 0 (`num()` `:250`), `name ?? key`.

---

### 3. Сервер: сборка `monitor.Overview` (`OverviewReader.Get`, `overview.go:98-161`)

#### 3.1 Алгоритм
```
observedAt := now()                                   // Overview.observed_at
rec := store.RunRecord(runID)  (ошибка → return err → NotFound/Internal)
snap.Run = rec; snap.Topology = topologyFromRecord(rec)
if rec.suite_run_id != "" → snap.SuiteRun = store.SuiteRun(id)
presence, err := agentPresence(runID, machineIDs(rec))   // registry; err → degraded "agent_registry_unavailable: <err>"

if persistedRecordIsTerminal(rec)                      // rec.status terminal ИЛИ rec.runtime_state.status terminal
    → overview = overviewFromRecord(...)  (source PERSISTED_RECORD)   // Temporal НЕ опрашивается
elif tc == nil
    → degraded "workflow_state_unavailable: temporal_client_not_configured"; overviewFromRecord
else
    rs, err := tc.GetRunState(ctx(timeout 4s), "test-run/"+runID, "")   // Temporal query к TestWorkflow
    err → (если ctx отменён — вернуть ошибку) иначе degraded "workflow_state_unavailable: <err>"; overviewFromRecord
    ok  → overview = mergeOverviewWithRecord(projectOverview(runID, rs, observedAt) /*source TEMPORAL*/, rec, presence)
          snap.Topology = topologyFromRecordWithRunState(rec, rs)
          overlayRunFromOverview(snap.Run, overview)     // мутирует record: status (если не cancelling/terminal), summary.started/finished/duration, progress=max
overview.DegradedReasons += degradedReasons
```
Таймаут 4s (`overviewRunStateQueryTimeout`, `:33`) — поднят с меньшего в коммите `5b116d14` («keep run status/duration live in overview»): UI выглядел зависшим, потому что Temporal-query таймаутил и отдавалась протухшая persisted-проекция.

Workflow id: `testWorkflowID` (`ids.go:14`) = `"test-run/" + runID` (совпадает с bloblang-выражением в `workflow/test.proto` `id: 'test-run/${! testRun.id }'`).

#### 3.2 `projectOverviewWithSource(runID, rs, observedAt, source)` (`:917-1090`) — RunState → Overview
- Для каждого `Stage` → `PipelineNode` (копия полей; `order` = stage.order или idx+1; `phase` = stage.phase или name для корней; `status_reason` = stage.status_reason или `"<temporal|persisted>_run_state_<status>"`; `Duration = spanDuration(started, finished, now)`; `Worker` = stage.worker или синтетический `agent/<machine_id>`; `LogRef{run_id, node_execution_id, component_id}`).
- Дерево: сортировка по `(parent_node_execution_id, order, index)`, затем узел с известным родителем → `parent.Children`, иначе → `roots`.
- Timeline: на каждую стадию с `started_at` → `Event{KIND_STAGE_STARTED, "stage started: <name>", status RUNNING, severity INFO}`; с `finished_at` → `Event{stageEndKind(status) (COMPLETED→STAGE_COMPLETED, FAILED→STAGE_FAILED, RETRY_WAIT→STAGE_RETRYING, иначе STAGE_COMPLETED), "stage finished: <name>", severity eventSeverity(status)}`; плюс один `Event{KIND_RUN_STATUS, "run status: <STATUS_X>", at = earliestStart|now}`. Сортировка по `at`, `sequence = 1..n`.
- `eventSeverity` (`:1138`): FAILED/CANCELLED → ERROR; RETRY_WAIT/CANCELLING → WARNING; иначе INFO.
- `StartedAt = min(stage.started_at)`; `FinishedAt = max(stage.finished_at)` **только если rs.status terminal**; `Duration = spanDuration(started, finished, now)`.
- `ProgressPct = progressPct(completed, len(stages))` = `round(completed/total·100)` clamp [0,100], 0 при total=0; `completed` — стадии со статусом `isPipelineTerminalStatus` (COMPLETED/FAILED/CANCELLED/SKIPPED **+ DEPLOYED**). Считаются **все** стадии, включая вложенные (agent-step'ы) — поэтому процент растёт по мере выполнения шагов деплоя.
- `Workers = []` (заполняются далее из record).

#### 3.3 `overviewFromRecord(runID, rec, presence, observedAt)` (`:280-330`) — fallback
- `rec == nil` → `pendingOverview`: `status PENDING`, пустые pipeline/workers/timeline, `source SYNTHETIC`.
- `rec.runtime_state != nil` → `projectOverviewWithSource(..., PERSISTED_RECORD)` + `Workers = workersFromRecord`; затем поправки из record:
  - `Status = rec.status` если он задан и (`runtime_state.status` не terminal, ИЛИ rec.status == CANCELLING, ИЛИ rec.status terminal);
  - `StartedAt/FinishedAt` из `summary` если заданы;
  - `Duration`: если runtime terminal и summary.duration есть → summary.duration; **иначе пересчёт live** `spanDuration(started, finished, observedAt)` (коммит `5b116d14`: раньше копировали замороженный summary.duration → таймер в UI «залипал»);
  - `ProgressPct = max(summary.progress_pct, projected)`.
- `runtime_state == nil` (запись до первого персиста) → плоский Overview из `summary` (started/finished/duration/progress) + `Pipeline = pipelineFromRecord(rec)` + `Workers = workersFromRecord`, `Timeline = []`, `source PERSISTED_RECORD`.

`pipelineFromRecord` (`:360`): 5 синтетических корней `infrastructure(1) / render_deployment_plan(2) / execute_deployment_plan(3) / workload(4) / teardown(5)` (`source PERSISTED_RECORD`, `status_reason` `persisted_record_*`), статусы выводятся из `infrastructure_state.machines[].status`, наличия `deployment_plan`, статусов компонентов плана и `rec.status` (`recordInfrastructureStatus … recordTeardownStatus`, `:597-688`). Дети `execute_deployment_plan` — компоненты и agent-step'ы из `deployment_plan` (`source DEPLOYMENT_PLAN`). Outputs для infra/render из `deploymentbuilder.*Outputs`.

#### 3.4 `mergeOverviewWithRecord` (`:332`) — live-путь
`Workers = workersFromRecord(rec, presence)`; если `rec.status ∈ {CANCELLING, terminal}` → `Status/StartedAt/FinishedAt/Duration` из record/summary, `ProgressPct = max`. Т.е. persisted terminal/cancelling статус имеет приоритет над Temporal.

#### 3.5 Workers (`workersFromRecord`, `:420-456`)
- Множество машин = `infrastructure_state.machines[].node_id ∪ deployment_plan.components[].node_id`, отсортировано.
- `Id = "agent/<nodeId>"`, `Kind = AGENT`, `MachineId`, `Host` = endpoint `private` → `public` → любой с адресом.
- `CurrentNodeExecutionId` = первый активный step (ALLOCATED/DEPLOYMENT/RUNNING/RETRY_WAIT/CANCELLING) компонента на этой ноде, иначе активный компонент.
- `Status` = RUNNING если есть current, иначе `machine.status` (PENDING если машины нет).
- `Presence` (record-уровень): `TERMINATED` если rec.status terminal, иначе `UNKNOWN`; `StatusReason`: `run_terminal_no_registry_sample` / `machine_not_materialized` / `no_registry_sample`; `Source PERSISTED_RECORD`; `Online = presence==ONLINE` (т.е. false).
- `mergeWorkerPresence(worker, registrySample)` (`:458`): переносит `Host` (если есть), `RegisteredAt`, `LastSeenAt`, `HeartbeatIntervalSeconds`, `AgentVersion`, `RunId`; если worker уже `TERMINATED` → `Online=false`, reason `run_terminal_agent_terminated` (registry не перебивает); иначе `Presence/Online/StatusReason` из сэмпла, `Source AGENT_REGISTRY`.
- Registry (`agent_registry.go:137-207`): in-memory карта `machineID → {runID, registeredAt, lastSeenAt, heartbeatInterval, host, agentVersion}` из `Register/Heartbeat` агентов; отдаются только записи с совпадающим `runID`. `classifyAgentPresence`: нет heartbeat → `UNKNOWN "agent_registered_without_heartbeat"`; `age ≤ 2·interval` → `ONLINE "heartbeat_recent"`; `≤ 6·interval` → `STALE "heartbeat_stale_<age>"`; иначе `OFFLINE "heartbeat_missed_<age>"`.

#### 3.6 Persisted `runtime_state`
`internal/workflows/test.go:996 persistRuntimeProjection(ctx, force)`: активити `PersistRunState(runID, compactRunState, nil, nil)` не чаще `runtimeProjectionPersistMinInterval = 20s` (`:55`), force при стадии FAILED/CANCELLED (`isProjectionForceStage`, `:1011`). Компактизация (`compactRunStateForRuntimeProjection`): усечение error_message/операций/outputs (лимиты 80 outputs, 512 байт текста, 16 элементов списков). История: в `5b116d14` throttle снижали до 2s и форсили на каждом completion; в `188eb2e1` («Temporal history bloat») вернули 20s, план шлётся в history один раз. Следствие: **persisted-fallback может отставать до 20s** от Temporal — это и есть причина существования live-query + `degraded_reasons`.
`PersistRunState` (`run_persistence.go:41`): `rec.Status = nextRunStatus(old, state.status)` (terminal и CANCELLING не откатываются), `applyRunSummary`, `rec.RuntimeState = clone(state)`, опционально infra state / deployment plan, каскад в `PersistSuiteRun`.

#### 3.7 Stages, которые видит эта зона
`newDomainTestWorkflow` (`workflows/test.go:319`): `RunState.stages = rootStage("infrastructure",1), ("render_deployment_plan",2), ("execute_deployment_plan",3), ("workload",4), ("teardown",5)` (`stages.go:18`: `NodeExecutionId = StageExecutionID(name)`, `Phase = name`, `Worker = master`). Дочерние: компоненты (`componentStage`, `Phase = execute_deployment_plan`), agent-step'ы; для workload — `workloadAgentStepStage` (`test.go:305`): `Phase = "workload"`, `Parent = StageExecutionID("workload")`, по одному call_cmd step на сегмент (`stepID = "<900+idx*10>_run_stroppy_<slug(segment.name)>"`, labels `phase=workload`, `segment=<segment.name>`; `test.go:254-262`). Имя стадии = `deploymentbuilder.AgentStepStageName(step)` (`domain/deployment/operation.go:24`) = сокращённое `operation.summary` (для call_cmd — заголовок команды), **не** `segment.name`.

---

### 4. Метрики (`GetRunMetrics`) — данные для select «window»

`MetricsReader.Get` (`monitoring.go:322`):
1. `base == ""` (мониторинг не сконфигурирован) → пустой `RunMetrics{run_id}`.
2. `resolveKindAndWindow`: record → `dbKind` (`summary.db_kind` → fallback `spec.database.kind`; MYSQL/MARIADB→"mysql", YDB/YDB_MANAGED→"ydb", др. lowercase; default "postgres"), окно `[summary.started_at − 30s, (summary.finished_at | now) + 30s]`; нет record/started → `[now−1h, now]`.
3. `overrideWindow(req.window)`: если `start` и `end` заданы и `end > start` → окно из запроса (без паддинга).
4. `victoria.NewClient(base + metricsTenantPath, token)` → `metrics.NewCollectorForDB(client, dbKind).Collect(ctx, runID, tr)` (`collector.go:41`): по каждому `MetricDef` из `MetricsForDB(kind)` → `RenderQuery(def, runID)` → `query_range(start, end, step)`; `step` по ширине окна (≤15m→5s, ≤1h→15s, ≤6h→1m, иначе 5m); `summarize`: все семплы всех серий → `last` = последний сырой; сортировка, обрезка p5–p95 (`lo = n·5/100, hi = n−lo`), `min/max/avg` по обрезанному. Ошибка любого запроса → ошибка всего вызова.

Фронт `getRunMetrics(tenantSlug, runId, window?)` (`run_overview.ts:553`): `window` отправляется **только когда заданы оба** `startedAt` и `finishedAt` → для ещё идущего сегмента (нет `finishedAt`) запрос идёт без окна = весь run.

---

### 5. Фронтовый маппинг `snapshotToVM` (`run_overview.ts:473-517`)

`OverviewVM`:
- `runId = ov.runId ?? runId(url)`; `status = statusToVM(ov.status)`; `startedAt/finishedAt` ISO-строки как есть; `durationSec = durSec(ov.duration)` (`:253`: строка `"12.5s"` от toJson → `parseFloat`, объект `{seconds,nanos}` → сумма; undefined → undefined); `progressPct = ov.progressPct ?? 0`.
- `pipeline = roots.map(nodeToVM)` (рекурсивно, `:403`); `workers` (`:483`), `timeline` (`:499`, `sequence` Number(string)).
- `run = testRunRecordToVM(runRecord)` если snapshot.run есть; `suiteRunId = snap.suiteRun?.entity?.id ?? ""`; `topology`; `observedAt = ov.observedAt`; `degradedReasons = ov.degradedReasons ?? []`; `source = obsSource(ov.source)`.

`testRunRecordToVM` (`runs.ts:444`): `toJson(TestRunRecordSchema)` → плоский `RunVM` (см. §2.3), `workloadSegments` из `spec.workload.segments[]` (`:502-520`):
`name, script, vus=execution.vus, duration=execution.duration, iterations=execution.iterations, poolSize=parameters.poolSize, scaleFactor=parameters.scaleFactor, insertMethod=parameters.defaultInsertMethod, bulkSize=parameters.bulkSize, env=parameters.env, steps=parameters.steps, noSteps=parameters.noSteps, quiet=execution.quiet, noThresholds=execution.noThresholds, extraArgs=execution.extraArgs, sql, files`. `spec = j.spec` целиком. Нюанс toJson: proto3 scalar-нули/пустые строки/пустые списки **не сериализуются** → в VM `undefined` (это и даёт «(default)»-логику).

Типовые несоответствия VM↔proto (влияют на рендер, см. §7.3): `WorkloadSegmentVM.noSteps: boolean` при proto `no_steps: repeated string`; `files?: string[]` при proto `files: WorkloadFile{name,kind,content}[]`.

---

### 6. Страница `RunDetail.tsx` — элементы, относящиеся к зоне

#### 6.1 Header (`:421-486`)
- Ссылка «← Runs».
- Точка статуса `STATUS_DOT[status]` (running/cancelling — `animate-pulse`), `h1 = run.name || "Run <id[0:8]>"`, `Badge STATUS_VARIANT[status]` (pending→pending, running→default, cancelling→warning, completed→success, failed→destructive, cancelled→warning).
- Моно-строка: полный `id`; **бейдж источника** `SOURCE_LABEL[source]` только если `source && source !== "temporal"` (`:434`) — т.е. «persisted/plan/registry/synthetic» подсвечиваются, live не помечается; `· as of <relTime(observedAt)>` (`relTime` `:109`: «Ns ago / Nm ago / Nh ago / Nd ago», «—» при пустом/невалидном/год<2000).
- Кнопки: Refresh (`load`, spinner при loading), Rerun (если `allowed.has("rerun")`), New from (`/runs/new?from=id`), Save preset (extract), Share, Compare, Shell, Cancel, Delete — гейт `actionsForStatus(overview.status)` (`runs.ts:396`).
- Баннеры: `error` (destructive), `notice` (success, 2.5s), **`Degraded snapshot: <degradedReasons.join("; ")>`** (warning, моно 11px) при `degradedReasons.length > 0` (`:494`).

#### 6.2 Layout sidebar (`:389-417, :502-520`)
- Ширина в state, персист `localStorage["stroppy.runDetail.sidebarWidth"]`, диапазон 220..560, default 300; драг-divider (`onDividerDown`, mousemove/mouseup на window).
- Контент: `overview ? <RunInfoSidebar overview run={overview.run}/> : ("Loading…" | "No data")` (`:504-510`). При ошибке загрузки overview остаётся null → «No data» + баннер ошибки.

#### 6.3 Переключатель view + select «window» (`:523-549`)
- `SegmentedControl` tabs `overview|pipeline|topology|agents|quotas|logs|metrics|grafana`, state в URL `?view=` (`setView` `:154`, replace, сбрасывает `line`). Default `overview`.
- **select «window»** показывается только при `view ∈ {grafana, metrics}` и `segments.length > 0`; `<option value=-1>Whole run</option>` + по одному на сегмент с `s.name`; state `segmentSel` (не в URL, сбрасывается при remount); `selSeg = segments[segmentSel]`; `winStart = selSeg?.startedAt ?? overview.startedAt`, `winEnd = selSeg?.finishedAt ?? overview.finishedAt` → пропсы GrafanaPanel и окно `getRunMetrics`. title: «Scope Grafana and metric averages to a workload segment».
- `segments = useMemo(workloadSegments(overview))` (`:242`).
- `view === "overview" && run` → `<RunConfigTab run={run}/>` (`:565`); при отсутствии `run` (snapshot без record) вкладка пустая. Скролл-режим: overview входит в `scrolls` → контейнер `overflow-auto p-3`.

#### 6.4 `workloadSegments(overview)` (`run_overview.ts:610-622`)
Рекурсивный обход `overview.pipeline`; узел попадает в список если `phase === "workload" && children.length === 0 && startedAt` → `{name, startedAt, finishedAt}` в порядке обхода (= порядок выполнения: сортировка на сервере по parent/order). Зачем (коммит `9cecd39b`): bootstrap-сегмент (create_schema+load_data) искажал средние измеренной нагрузки; выбор сегмента ограничивает Grafana и `GetRunMetrics.window` его временным окном. Крайние случаи:
- до старта стадии workload — пусто → select скрыт;
- корневой узел `workload` без детей (persisted-fallback без runtime_state, или до появления шагов), но со `startedAt` — сам становится «сегментом» с именем `workload`;
- имя опции — имя стадии (сокращённое summary команды), не `segment.name`;
- идущий сегмент без `finishedAt` → окно не передаётся (весь run), Grafana получает `winEnd = overview.finishedAt` (undefined → «до сейчас»).

---

### 7. Компонент `RunInfoSidebar` (`RunInfoSidebar.tsx`)

Общие примитивы:
- `Group{label}` (`:93`): секция с uppercase моно-заголовком 10px, `border-b`.
- `Row{icon,label,value}` (`:104`): **скрывается полностью**, если `value` ∈ {undefined, null, "", "—"} (`:113`). Лейбл фикс. ширины `w-16`, значение `break-all` моно 11px.
- `fmtTs(iso)` (`:30`): "" для пустого/NaN/год<2000; иначе `toLocaleString("en-GB", {month:"short", day, HH:mm:ss, hour12:false})` (напр. `12 Mar, 14:03:07`).
- `fmtDur(sec)` (`:44`): undefined/не-finite → "—"; `<60` → `Ns`; `<3600` → `Nm Ss`; иначе `Nh Mm`.
- `Elapsed{startedAt}` (`:81`): live-таймер, `setInterval(1000)` форсит ререндер, показывает `fmtDur(now − startedAt)` (clamp ≥0).
- Пропсы: `overview: OverviewVM`, `run?: RunVM` (= `overview.run`).
- Вычисления при рендере: `isRunning = status ∈ {running, cancelling}`; `pipeline = countPipeline(overview.pipeline)`; `workers` агрегат; `errorEvents/warningEvents` по `timeline.severity`.

#### 7.1 Группа **Identity** (всегда)
| строка | значение | источник | крайние случаи |
|---|---|---|---|
| status | `overview.status` (текст: pending/running/cancelling/completed/failed/cancelled) | `monitor.Overview.status` → statusToVM | всегда есть (default pending) |
| id | `runId.slice(0,18) + "…"`, `title` = полный | `Overview.run_id` (fallback id из URL) | — |
| owner | `<Avatar name/> + label`, `title` = `ownerDisplay.title` | `run.authorId` (entity.author_id); `useEffect` (`:141`): сначала `fallbackAuthorDisplay(id)` (label=id), затем `resolveAuthorDisplay(id)` → `IamService.GetAccount` → `nickname || email || id`, title `"<label> · <email> · <id>"`; при ошибке — id | строка отсутствует без `run`/`authorId`; смена run → повторный resolve; защита от гонки `cancelled` |
| created | `fmtTs(run.createdAt)` | `entity.timings.created_at` | скрыта без record |
| started | `fmtTs(overview.startedAt) \|\| "—"` | `Overview.started_at` (= min stage.started_at; в fallback — summary.started_at) | **скрыта** пока не стартовал (значение "—" фильтруется Row) |
| finished | `fmtTs(overview.finishedAt)` | `Overview.finished_at` (только когда terminal) | скрыта пока идёт |
| duration | running/cancelling и есть startedAt → `<Elapsed/>` (тикает); иначе `fmtDur(overview.durationSec)` | `Overview.duration` (сервер: spanDuration; для terminal — summary.duration) | pending без started → "—" → скрыта; failed/completed — статичная |
| trigger | `run.trigger` (manual/cron/api) | `TestRunRecord.trigger` | скрыта при UNSPECIFIED |
| suite | `suiteRunId.slice(0,12) + "…"`, title полный | `TestRunRecord.suite_run_id` | скрыта для standalone |

#### 7.2 Группа **Database** — рендерится если `run && (dbKind || topologyLabel)`
| строка | значение | источник |
|---|---|---|
| kind | `run.dbKind \|\| "—"` (postgres/mysql/mariadb/ydb/ydb_managed/cockroach/picodata/orioledb/external/noop/pgnoop) | `summary.db_kind` через `dbKindLabelFromJson` |
| topology | `run.topologyLabel` (напр. «PG HA x3») | `summary.topology_label` = `spec.topology_spec.labels["label"]` |
| preset | `run.dbPresetId.slice(0,8)` | `summary.db_preset_id` (только если БД из пресета) |
| test | `run.testPresetId.slice(0,8)` | `summary.test_preset_id` |

#### 7.3 Группа **Workload** — если `run && (workload || stroppyVersion || segments.length > 0)`
| строка | значение | источник |
|---|---|---|
| name | `run.workload \|\| "—"` | `summary.workload_name` (= `"stroppy <ver>"`, см. summarizer) |
| protocol | `run.protocol` (pg/mysql/picodata/ydb_grpc/ydb_grpcs/cockroach/noop) | `summary.workload_protocol` |
| stroppy | `run.stroppyVersion` | `summary.stroppy_version` |
| preset | `run.workloadPresetId.slice(0,8)` | `summary.workload_preset_id` |
| `<seg.name \| "seg N">` (по одной на сегмент, иконка Gauge) | `[script, "vus=N", duration ? "dur=<d>" : iterations!==undefined && "iters=N", "pool=N", "scale=N"].filter(Boolean).join(" · ")` | `spec.workload.segments[i]` (`execution.vus/duration/iterations`, `parameters.pool_size/scale_factor`) |

Зачем (коммит `15f7481d`): «Overview показывал только имя workload, не то, как запущен stroppy» — сегменты в sidebar делают run воспроизводимым «с одного взгляда». Сегменты есть только когда record несёт `spec` (snapshot / GetTestRun), из list-summary их нет.

#### 7.4 Группа **Infrastructure** — если `run && (provider || nodeCount > 0)`
| строка | значение | источник |
|---|---|---|
| provider | `run.provider \|\| "—"` (docker/yandex) | `summary.provider` = `spec.infrastructure_plan.provider` |
| nodes | `String(nodeCount)` если > 0 | `summary.node_count` = `len(spec.topology_spec.nodes)` |

#### 7.5 Группа **Runtime** (всегда)
| строка | вычисление (`:126-139`) | формат | скрыта когда |
|---|---|---|---|
| agents | по `overview.workers`: `online` если `presence==="online" \|\| worker.online`; `stale`; `offline` если presence ∈ {offline, terminated}; иначе `unknown` | `compactCounts([["online",n],["stale",n],["offline",n],["unknown",n]])` → `"online:2 stale:1"` (нулевые пропущены); fallback `String(total)` | `workers.length === 0` (напр. до infra state / без record) |
| stages | `countPipeline` (`:59`) — **рекурсивно по всем узлам, включая детей**: running (`running`/`cancelling`), completed, failed, cancelled, pending (все остальные) | `"run:1 ok:12 fail:0→скрыт wait:30 cancel:0"` через `compactCounts([["run"],["ok"],["fail"],["wait"],["cancel"]])`; fallback total | `pipeline` пуст (synthetic pending) |
| events | `timeline.length` | число | 0 (fallback без runtime_state → timeline пуст) |
| issues | `errorEvents = count(severity==="error")`, `warningEvents = count("warning")` | `"errors:N warn:M"` | оба 0 |
| source | `SOURCE_LABEL[overview.source] ?? source` | `live` / `persisted` / `plan` / `registry` / `synthetic` | source "" (UNSPECIFIED) |
| observed | `fmtTs(overview.observedAt)` | серверное время сборки snapshot | — |
| degraded | `String(degradedReasons.length)` | число (тексты причин — в header-баннере) | 0 |

Семантика `source` для этой зоны: `live` = Temporal query успешен (run идёт); `persisted` = terminal run или Temporal недоступен/таймаут (см. degraded); `synthetic` = record не найден/store нет (только pending-заглушка); `plan`/`registry` на верхнем уровне Overview не выставляются (только у узлов/воркеров).

#### 7.6 Группа **Progress** (всегда)
Полоса `h-1 rounded-full bg-muted` + заливка `width = clamp(progressPct, 0, 100)%`, `transition-all duration-700`; цвет: `failed` → `bg-destructive`, `completed` → `bg-success`, `cancelled`/`cancelling` → `bg-pending`, иначе `bg-primary`. Справа `Math.round(progressPct)%` tabular-nums. Источник: `Overview.progress_pct` (§3.2: доля terminal-стадий среди всех стадий RunState; при fallback `max(summary.progress_pct, projected)`). Замечание: completed run показывает 100% только если все стадии terminal; при failed/cancelled процент = доля завершившихся (не 100).

---

### 8. Вкладка Overview — `RunConfigTab` (`RunConfigTab.tsx`)

Назначение (шапка файла + коммит `18835c48`): полный read-only «эффективный spec запуска», чтобы run был воспроизводим/аудируем без «New from run»; workload-часть курируется, database + всё прочее рендерится **генерически** из decoded spec, чтобы новые движки/поля появлялись без правки UI.

#### 8.1 Guard
`!run.spec || Object.keys(run.spec).length === 0` → `<div class="p-6 text-sm text-muted-foreground">No spec available for this run (list summary only — open the run detail to load the full spec).</div>` (`:176`). В реальности на странице деталей spec всегда есть (snapshot.run required); сообщение для случая RunVM из списка.

#### 8.2 Примитивы
- `humanizeKey` (`:14`): camelCase/snake/kebab → «Title Case» (`masterOptions` → «Master Options», `isBuiltin` → «Is Builtin»).
- `isPlainObject`, `isFlatMap` (все значения примитивы), `isEmptyValue` (null/undefined/""/[]/{}).
- `Prim` — моно 11px `String(value)`; `KV{label}` — строка: лейбл `w-40 truncate` (title=label) + значение.
- `ConfigValue` (`:56`) — рекурсивный рендер: пустое → null; массив примитивов → `join(", ")`; массив объектов → стопка карточек `border p-2` с рекурсией; объект → `KV(humanizeKey(k))` для непустых полей, вложенные объекты/массивы рекурсивно; примитив → `Prim`. Булевы → «true»/«false»; числа как есть.
- `Card{icon,title}` — `rounded-lg border bg-card/40 p-4`, заголовок uppercase 11px.
- `OptionsTable{opts}` (`:106`) — «как конфиг-файл»: `bg-black/20`, строки `key = value` (ключ `text-primary/80`), пустые значения отфильтрованы.

#### 8.3 Сетка: `grid lg:grid-cols-2` → карточка WORKLOAD, карточка DATABASE; ниже — FULL SPEC (RAW).

#### 8.4 Карточка **WORKLOAD** (`:202-215`)
Шапка (KV, каждая только если значение непустое): `Name = run.workload`, `Protocol = run.protocol`, `Stroppy = run.stroppyVersion`, `Preset = run.workloadPresetId` (полный id, не обрезан).
Далее список `SegmentCard` по `run.workloadSegments`; при пустом — текст «No segments.».

**`SegmentCard{seg, index}`** (`:122-156`), рамка `border p-3`, заголовок `Gauge` + `seg.name || "segment N"` (bold mono xs). Строки (KV, порядок фиксирован):
| лейбл | условие показа | значение | proto |
|---|---|---|---|
| Script | `seg.script` | текст | `segment.script` |
| Execution | `exec.length > 0` | `["vus=N", duration ? "duration=<d>" : iterations!==undefined && "iterations=N", quiet && "quiet", noThresholds && "no-thresholds"].filter(Boolean).join(" · ")` | `execution.vus / duration\|iterations / quiet / no_thresholds` |
| Insert method | всегда | `seg.insertMethod \|\| "native"` | `parameters.default_insert_method` (пусто = native) |
| Bulk size | `insertMethod ∈ {plain_bulk, columnar} \|\| bulkSize > 0` | `seg.bulkSize ?? "(default)"` (0 не сериализуется → «(default)» = 2500 по докстрингу proto) | `parameters.bulk_size`; `columnar` добавлен коммитом `68926a36` |
| Pool size | `poolSize !== undefined` | число | `parameters.pool_size` |
| Scale factor | `scaleFactor !== undefined` | число (может быть дробным, напр. 0.01) | `parameters.scale_factor` |
| Steps | `seg.noSteps` truthy | **«(all)»** | `parameters.no_steps` |
| Steps | `steps.length > 0` | `steps.join(", ")` | `parameters.steps` |
| Extra args | `extraArgs.length > 0` | `join(" ")` | `execution.extra_args` |
| Env | `env` непустой | `OptionsTable` (`KEY = value`) | `parameters.env` (map) |
| Files | `files.length > 0` | `files.join(", ")` | `segment.files[]` |
| SQL | `seg.sql` | `<pre max-h-40 overflow-auto bg-black/30>` | `segment.sql` |

Флаги «quiet · no-thresholds»: в proto `quiet` — `optional bool` (absent = production-default `-q`, явный `false` = убрать `-q`); UI показывает слово `quiet` **только при явном true**, absent и false неразличимы. `no_thresholds` (`--no-thresholds`) показывается при true.

Известные дефекты рендера (важно для переимплементации):
- **Steps «(all)»**: proto `no_steps` — `repeated string` (блоклист фаз), VM типизирует как boolean; непустой массив truthy → рендерится «Steps: (all)» вместо списка исключённых шагов. Если заданы и `steps`, и `no_steps` (proto запрещает), будут две строки «Steps».
- **Files**: `files` — массив объектов `{name, kind, content}`; `join(", ")` даёт `[object Object], …`. Ожидалось — имена файлов.
- **SQL**: `segment.sql` по proto — «второй позиционный аргумент stroppy (напр. имя SQL-файла)», max 512 символов, а не inline-SQL; UI показывает как `<pre>` с SQL.

#### 8.5 Карточка **DATABASE** (`:218-247`)
Шапка (KV): `Kind = run.dbKind || dbParams?.engine || "—"`; `Topology = run.topologyLabel`; `Preset = run.dbPresetId`; `Provider = run.provider`; `Nodes = run.nodeCount` (если > 0).

Разбор spec (`:184-196`):
```
db = spec.database (объект)
dbParams = extractDbParams(db):
    paramsHolder = db.source?.params  ??  db.params      // protobuf-es toJson даёт db.params (oneof плоско); ветка db.source — ogen-форма, во фронте не встречается
    вернуть ПЕРВУЮ пару [engine, v] из Object.entries(paramsHolder), где v — plain object
optionMaps  = поля dbParams.params: plain object && isFlatMap && /options$/i.test(key)
scalarParams = остальные непустые поля
```
Рендер: `scalarParams` → `<ConfigValue>` (KV с humanize-ключами; вложенные объекты — рекурсивно); `optionMaps` → для каждого: подзаголовок `Settings2` + `humanizeKey(k)` (напр. «Master Options», «Haproxy Options») и `OptionsTable`. Если `dbParams === null` → «No detailed database params in spec.» (external / preset-id source / отсутствие params).

**Почему на практике карточка показывает «Id builtin/…, Name, Db Kind, Db Version, Is Builtin»**: `DatabaseParams` в JSON имеет поля в порядке номеров: `version`(1, строка — пропускается), **`package`(2, объект)**, затем engine (`postgres`=10 …). `extractDbParams` берёт первый объект → `engine = "package"`, `params = Package{id: "builtin/…", name, dbKind: "KIND_POSTGRES", dbVersion, isBuiltin, aptPackages[], preInstall[], customRepo, …}`. Поэтому в scalarParams попадают поля пакета (`Id`, `Name`, `Db Kind`, `Db Version`, `Is Builtin`, при наличии `Apt Packages` (join ", "), `Pre Install`), а **реальные engine-параметры (replicas/patroni/haproxy/master_options/…) не рендерятся вообще**, когда в spec есть `package` (т.е. для любого self-deploy run после появления `Package` в `DatabaseParams`). `version` (engine version label) тоже не показывается. Kind в шапке при этом корректен (из summary). При переимплементации: брать `db.params.version`, `db.params.package` отдельно, а engine — по имени oneof-ветки (`postgres|mysql|mariadb|picodata|ydb|ydbManaged|cockroach|orioledb|noop|pgNoop`), сопоставляя с `db.kind`.

Что должно отображаться при корректном разборе (по proto `database.proto`): Postgres — `replicas, haproxy, pgbouncer, patroni, etcd, sync_replicas` + `master_options/replica_options/haproxy_options/pgbouncer_options/patroni_options/etcd_options`; MySQL/MariaDB — `replicas, proxysql, group_replication, semi_sync` + `primary_options/replica_options/proxysql_options`; Picodata — `instances, haproxy, replication_factor, shards, tiers[]{name, replication_factor, can_vote, count}` + `instance_options/haproxy_options`; YDB — `storage_nodes, database_nodes, haproxy, pdisks_per_storage_node, fault_tolerance, failure_domain_type, default_disk_type, storage_groups, auto_size_pdisks, database_path` + `storage_options/database_options/haproxy_options`; YDB managed — `type, compute_type, resource_preset_id, node_count, auto_scale{min,max,cpu}, storage_groups, storage_type, throttling_rcus`; OrioleDB — `image, initdb_locale, shared_buffers_mb, replicas, haproxy` + `postgres_options/replica_options/haproxy_options`; Cockroach — `nodes` + `options`; PgNoop — `workers` + `options`; Noop — пусто. Плюс `DatabaseParams.version`, `Package{…}`, `Database.package_id`, `Database.tags`; External — `dsn`, `tags`.

#### 8.6 **FULL SPEC (RAW)** (`:251-266`)
Кнопка-строка `ChevronRight`(поворот 90° при open) + `Code2` + «Full spec (raw)», state `rawOpen` (default закрыт, не персистится). При open — `<pre max-h-[32rem] overflow-auto border-t bg-black/30 p-3 mono 11px>{JSON.stringify(run.spec, null, 2)}</pre>`. Содержимое = `toJson(TestRunRecord).spec` = `domain.TestRun` в camelCase JSON: `id, suiteId, database{kind, params{version, package, <engine>}|external|databasePresetId, packageId, tags}, workload{stroppyVersion, protocol, segments[], tags}, topologySpec, infrastructurePlan, renderOverrides, tags`. Enum'ы — строками (`KIND_POSTGRES`, `PROTOCOL_PG`), Duration — строкой (`"600s"`), нули/пустые опущены. Это «всё, что известно» fallback — ничего не скрывается.

---

### 9. Состояния зоны (сводно)

| Состояние | Сервер | Sidebar | Header | Overview tab |
|---|---|---|---|---|
| Первичная загрузка | — | «Loading…» | h1 «Run <id8>», без статуса | не рендерится (run undefined) |
| Ошибка GetTestRunOverview (NotFound/Unauth/…) | `authorizeRun`/store error | «No data» | баннер `error` | — |
| Record есть, workflow ещё не стартовал (PENDING, нет runtime_state, Temporal query падает) | `overviewFromRecord` без runtime_state: status из rec (PENDING), pipeline = 5 синтетических корней (pending), timeline пуст, workers по infra (обычно пусто), `source PERSISTED`, degraded `workflow_state_unavailable: …` | status pending; started/finished/duration скрыты; agents/events/issues скрыты; stages `wait:5`; source `persisted`; degraded `1`; progress 0% | бейдж «PERSISTED», «as of Ns ago», жёлтый баннер Degraded | полный spec |
| Record не найден store'ом, но Exists прошёл (гипотетически) / store nil | `pendingOverview` `source SYNTHETIC` | минимальные строки; source `synthetic` | бейдж «SYNTHETIC» | пусто (нет run) |
| Идёт (Temporal ok) | `projectOverview` TEMPORAL + merge с record; стрим 1s | duration тикает (`Elapsed`), stages `run:1 ok:n wait:m`, agents `online:k`, events N, source `live`, observed обновляется каждую секунду | бейдж источника **не показан**, «as of 0s ago» | — |
| Идёт, Temporal таймаут (>4s)/недоступен | fallback на `runtime_state` (до 20s отставания), `source PERSISTED`, degraded `workflow_state_unavailable: <err>`; duration пересчитывается live от observedAt | source `persisted`, degraded `1`, duration всё равно тикает (клиент считает от startedAt) | бейдж «PERSISTED» + Degraded баннер | — |
| Registry недоступен | degraded `agent_registry_unavailable: …`; workers c presence UNKNOWN/`no_registry_sample` | agents `unknown:N` | Degraded баннер | — |
| Cancelling | статус из record (CANCELLING приоритетнее Temporal) | status `cancelling`, duration тикает, progress `bg-pending` | badge warning, точка pulse | — |
| Terminal (completed/failed/cancelled) | persistedRecordIsTerminal → Temporal **не опрашивается**; всё из record; стрим закрывается после terminal-фрейма; клиент стрим/поллинг не поднимает | duration статичная = summary.duration; finished показан; agents `offline:N` (TERMINATED); source `persisted` | бейдж «PERSISTED» (всегда для terminal!) | — |
| Terminal без runtime_state (старые/аварийные записи) | плоский Overview из summary + `pipelineFromRecord` | stages по статусам синтетических корней и deployment_plan; events скрыты | — | — |
| Suite-child | `snap.suite_run` заполнен | строка `suite <id12>…` | — | — |
| External DB / preset-id source | — | Database: kind `external`, без topology если нет label | — | «No detailed database params in spec.» |

---

### 10. Чек-лист для переимплементации (что обязательно сохранить)

1. Один snapshot-контракт для unary и stream; клиент заменяет state целиком.
2. Стрим только пока не terminal; при ошибке стрима — poll 5s; Refresh — unary.
3. Тройной источник overview с явной пометкой `source` + `observed_at` + `degraded_reasons` — UI обязан показывать не-live источник (бейдж) и причины (баннер), sidebar — счётчик.
4. Terminal run никогда не ходит в Temporal.
5. Duration: сервер отдаёт span; клиент для running тикает сам от `startedAt`.
6. progress = доля terminal-стадий (включая вложенные) — не «этап из 5».
7. Sidebar-строки скрываются при пустом значении (нет «—»-заглушек, кроме kind/name/provider где `|| "—"` тоже в итоге скрывается через Row).
8. Overview tab: WORKLOAD курируемый (сегменты — таблица §8.4), DATABASE генерический (scalars через ConfigValue, `*_options` — как конфиг-файл), RAW JSON — полный `spec`.
9. Исправить при переносе: разбор `DatabaseParams` (package vs engine oneof), `no_steps` как список, `files[].name`, семантику `sql`.
10. `workloadSegments` — листья `phase="workload"` со `startedAt`; окно метрик отправлять только при полном `[start,end]`.

---

# 3. Pipeline + Topology


Репо `stroppy-cloud` @ main cf16c9c0. Все пути абсолютные от корня репо.

Ключевые файлы:
- UI: `web/src/pages/RunDetail.tsx`, `web/src/components/run/PipelineViewer.tsx`, `web/src/components/run/TopologyViewer.tsx`
- Данные (клиент): `web/src/services/run_overview.ts`, `web/src/services/dashboard.ts` (`statusToVM`)
- Proto: `protocols/cloud/v1/api/test_run_overview.proto`, `protocols/cloud/v1/monitor/overview.proto`, `protocols/cloud/v1/monitor/logs.proto` (LogRef), `protocols/cloud/v1/topology/topology.proto`, `protocols/cloud/v1/topology/connection.proto`, `protocols/cloud/v1/topology/component.proto`, `protocols/cloud/v1/workflow/test.proto` (RunState/Stage), `protocols/cloud/v1/common/status.proto`
- Сервер: `internal/services/test_run_overview/service.go` + `connect.go` (handler), `internal/infrastructure/execution/overview.go` (OverviewReader — сборка снапшота), `internal/infrastructure/execution/runtime_topology.go` (runtime-граф), `internal/infrastructure/execution/run_persistence.go` (PersistRunState), `internal/workflows/test.go` + `internal/workflows/stages.go` + `internal/workflows/deployment.go` (Temporal — источник Stage), `internal/domain/deployment/execution_ids.go` + `operation.go` + `infrastructure_outputs.go` (ID, операции, outputs)

---

### 0. Общая картина потока данных

```
Temporal TestWorkflow (RunState{status, stages[]})
   │  query GetRunState (compactRunStateForRuntimeProjection)      ┐ live
   │  activity PersistRunState → TestRunRecord.runtime_state       ┘ persisted fallback
   ▼
OverviewReader.Get (overview.go:98)
   ├─ snap.Run       = TestRunRecord (persisted)
   ├─ snap.Topology  = topologyFromRecordWithRunState(rec, rs)   → Topology{spec, infra_plan, infra_state, deployment_plan, runtime_nodes[], runtime_connections[]}
   ├─ snap.Overview  = projectOverview(rs) | overviewFromRecord(rec)  → monitor.Overview{pipeline.roots[], workers, timeline, ...}
   └─ snap.SuiteRun
   ▼
TestRunOverviewService.GetTestRunOverview / StreamTestRunOverview (1 снапшот/сек)
   ▼
run_overview.ts: toJson(...) → snapshotToVM → OverviewVM{pipeline: PipelineNodeVM[], topology: TopologyJson, ...}
   ▼
RunDetail.tsx: view=pipeline → <PipelineViewer pipeline onOpenLogs>; view=topology → <TopologyViewer topology>
```

Одна проекция для всего (нет отдельного «статического blueprint»): для незапущенного рана сервер отдаёт то же дерево с PENDING (overview.proto:113-117).

---

### 1. Контейнер: RunDetail.tsx — переключение вкладок и связка с логами

#### Фича 1.1. Вкладки Pipeline / Topology в едином view-switcher
- **Зачем**: одно место для всех представлений рана; состояние в URL → любой вид — shareable link.
- **Как**: `RunDetail.tsx:120-131` — тип `View = "overview"|"pipeline"|"topology"|"logs"|"metrics"|"quotas"|"grafana"|"agents"`, `VIEW_OPTIONS` (иконки lucide: `Workflow` для pipeline, `Network` для topology). `SegmentedControl variant="tabs"` (`RunDetail.tsx:524-530`, `segmentClassName="w-[88px]"`).
- Активный view читается из `?view=` (`RunDetail.tsx:152-153`, default `"overview"`); `setView` (`:154-167`) пишет `view` через `setSp(..., {replace:true})` и **удаляет `line`** (transient-якорь строки лога).
- Рендер: `RunDetail.tsx:566` `{view === "pipeline" && <PipelineViewer pipeline={overview?.pipeline ?? []} onOpenLogs={openLogsForNode} />}`; `:567` `{view === "topology" && <TopologyViewer topology={overview?.topology} />}`.
- Контейнер: `RunDetail.tsx:564` — для pipeline/topology `scrolls=false` (`:379`, только overview/metrics/quotas/agents скроллят страницу) → обёртка `h-full overflow-hidden`, компонент сам управляет высотой (у PipelineViewer внутренний `overflow-y-auto`, у TopologyViewer — canvas xyflow).
- **Крайние случаи**: `overview` ещё null → Pipeline получает `[]` (empty state «Waiting for pipeline steps…»), Topology получает `undefined` («Topology not available for this run»).

#### Фича 1.2. Jump «View in Logs» из pipeline-узла
- **Зачем**: контракт proto — каждый PipelineNode несёт LogRef, UI должен прыгать в логи стадии (overview.proto:24-26, 145-150).
- **Как**: `RunDetail.openLogsForNode(nodeExecutionId)` (`RunDetail.tsx:170-192`): `setSp(prev → next)` с `view=logs`, `steps=<nodeExecutionId>`, и **удалением** всех остальных фильтров логов: `comp, mach, unit, src, stream, phase, action, mention, q, line`. `{replace:false}` — создаётся запись истории (кнопка «назад» вернёт на pipeline).
- Приёмник: `LogsPanel.tsx:177` `stepsParam = sp.get("steps")` → `steps = Set(csv(...))` (`:194`) → `serverFilter().nodeExecutionIds = stepIds` (`:299`) → `api.LogFilter.node_execution_ids` (см. `toLogFilter` в `run_overview.ts:626-647`). Multi-select «Step» (`LogsPanel.tsx:684`) заполняется из того же `pipeline` (`flattenNodes` `:59-65`: id=nodeExecutionId, label=humanize(name) || id.slice(0,8)).
- **Сервер**: `monitor.LogRef{run_id, node_execution_id?, component_id?, start?, end?, cursor?}` (logs.proto:113-125). В overview.go каждый узел получает `LogRef: logRef(runID, nodeExecutionID, componentID)` (`overview.go:878-884`, `:969-973`) — но фронт **не читает `log_ref`** (nodeToVM его игнорирует), а строит фильтр напрямую из `nodeExecutionId`. `ResolveLogRef` RPC есть (`run_overview.ts:840-854`), но PipelineViewer им не пользуется.
- **Крайние случаи**: у узла без `nodeExecutionId` кнопка не рендерится (`PipelineViewer.tsx:252`, `:345`). У band-заголовка кнопка — `<span role="button" tabIndex=0>` внутри `<button>` (вложенные кнопки запрещены HTML), `e.stopPropagation()` чтобы не сворачивать band.

#### Фича 1.3. Live-обновление (общая для обеих вкладок)
- **Зачем**: pipeline/topology должны меняться по ходу рана без F5.
- **Как**: `RunDetail.tsx:251-268` — пока `status` не terminal (`completed|failed|cancelled`, `:237`), открывается `streamRunOverview(tenantSlug, id, setOverview, signal)`; при ошибке стрима (не abort) — fallback polling `getRunOverview` каждые 5000 мс. Каждый фрейм — **полный** снапшот, `setOverview` заменяет состояние целиком (нет diff-merge; `StreamTestRunOverviewRequest` comment: «client just replaces its state»).
- Первичная загрузка `load()` (`:196-214`) — `getRunOverview`.
- Кнопка Refresh (`:444-446`) — ручной `load()`.
- **Сервер**: `OverviewReader.Stream` (`overview.go:179-222`) — `Get` сразу, затем ticker `streamInterval = 1s` (`:27`), закрывает канал при terminal статусе `isTerminalStatus(snap.Overview.Status)` (COMPLETED/FAILED/CANCELLED/SKIPPED, `:1181-1191`) или ctx.Done. Handler: `service.go:190-192` → `streamOverview`; connect-обёртка `connect.go:31-33`.
- **Крайние случаи**: `useMemo` в PipelineViewer/TopologyViewer пересчитывают по ссылке `pipeline`/`topology` → каждый фрейм = полный пересчёт layout (у Topology — union-find + раскладка; ок для сотен узлов). При смене `status` эффект переподписывается (dependency `[tenantSlug,id,status,terminal]`).
- Header показывает `overview.source` если не `temporal` (бейдж `persisted|plan|registry|synthetic`, `RunDetail.tsx:434-438`, `SOURCE_LABEL :96-102`) и `as of <relTime(observedAt)>`; `degradedReasons` — жёлтая полоса (`:494-498`).

---

### 2. Слой данных (клиент): run_overview.ts

#### Фича 2.1. Получение снапшота
- `getRunOverview(tenantSlug, runId)` (`run_overview.ts:537-545`): `resolveTenantId` → `testRunOverviewClient.getTestRunOverview({tenantId, runId})` → `toJson(GetTestRunOverviewResponseSchema)` → `snapshotToVM(j.snapshot, resp.snapshot?.run, runId)`. Run record передаётся **как proto** (не JSON) для `testRunRecordToVM`.
- `streamRunOverview` (`:863-884`): `for await` по `streamTestRunOverview`, `toJson(TestRunOverviewSnapshotSchema, snap)`, AbortError глотается.

#### Фича 2.2. Маппинг Pipeline: proto → PipelineNodeVM
- `SnapshotJson.overview.pipeline.roots` (`:440`) → `nodeToVM` рекурсивно (`:403-424`).
- `PipelineNodeVM` (`:84-113`): `nodeExecutionId, name, status: RunStatus, startedAt?, finishedAt?, durationSec?, attempt, children[], source: ObservationSource, order, parentNodeExecutionId, phase, componentId, machineId, statusReason, errorMessage, operation?: OperationVM, outputs: PipelineOutputVM[]`.
- **Не маппятся** из proto: `worker` (domain.Worker), `params` (schemapb.Baked), `log_ref`. Для реимплементации — данные есть на проводе, UI их не показывает.
- Статус: `statusToVM` (`dashboard.ts:161-184`): PENDING/UNSPECIFIED→`pending`; RUNNING/ALLOCATED/DEPLOYMENT/RETRY_WAIT→`running`; CANCELLING→`cancelling`; COMPLETED/DEPLOYED→`completed`; FAILED→`failed`; CANCELLED/SKIPPED→`cancelled`.
- Duration: `durSec` (`:253-258`) — JSON Duration либо строка `"12.5s"` (parseFloat), либо `{seconds,nanos}`.
- `source`: `obsSource` (`:260-267`) OBSERVATION_SOURCE_TEMPORAL_RUN_STATE→`temporal`, PERSISTED_RECORD→`persisted`, DEPLOYMENT_PLAN→`plan`, AGENT_REGISTRY→`registry`, SYNTHETIC→`synthetic`.
- `OperationVM` (`:43-63`) ← `monitor.PipelineOperation`: `kind` (`operationKind` `:285-291`: create_dir|write_file|fetch_file|call_cmd), `stepId, stepOrder, target, summary, mentions[], labels, commandText, argv[], filePath, fileSizeBytes (uint64→Number), contentPreview, resultAvailable, exitCode, timedOut, elapsedSec, stdoutPreview, stderrPreview, resultSummary`. Сырые `command/file/dir` payload'ы не маппятся (и на проводе они уже обнулены компакцией, см. §4.6).
- `PipelineOutputVM` (`:66-81`) ← `monitor.PipelineOutput`: `kind` (`outputKind` `:293-301`: summary|render_artifact|deployment_plan|command_result|file|directory), `id, name, summary, componentId, machineId, stepId, action, target, commandText, contentPreview, count, sizeBytes, labels`.

#### Фича 2.3. Маппинг Topology
- `OverviewVM.topology?: TopologyJson` (`:163`) — **сырой JSON** proto `topology.Topology` (без VM-слоя); `snapshotToVM` просто прокидывает `snap.topology` (`:512`). TopologyViewer работает с `TopologyJson`, `RuntimeNodeJson`, `RuntimeConnectionJson` из `@/lib/proto/cloud/v1/topology/topology_pb`.
- Enum'ы в JSON — строки (`"KIND_MACHINE"`, `"STATUS_RUNNING"`, `"PROTOCOL_HTTP"`, `"MODE_STREAM"`).

#### Фича 2.4. workloadSegments (потребитель pipeline вне вкладки)
- `run_overview.ts:610-622`: обход дерева, берёт **листья с `phase === "workload"` и `startedAt`** → `{name, startedAt, finishedAt}`. Используется в RunDetail для селектора «window» (Grafana/Metrics, `RunDetail.tsx:242-246, 531-548`). Важно для реимплементации: семантика `phase` на leaf-узлах должна сохраниться.

---

### 3. Proto-контракт

#### 3.1 api.TestRunOverviewSnapshot (test_run_overview.proto:127-148)
`run: models.TestRunRecord (required)`, `topology: topology.Topology (required)`, `overview: monitor.Overview (required)`, `suite_run: models.SuiteRunRecord?`.
RPC: `GetTestRunOverview(tenant_id, run_id) → {snapshot}`, `StreamTestRunOverview → stream TestRunOverviewSnapshot`. Авторизация: `authorizeRun` (`service.go:155-169`): caller → tenant_id → run_id → `Runs.Exists(tenant, run)` (cross-tenant = NotFound).

#### 3.2 monitor.Overview (overview.proto:82-111)
`run_id, status: common.Status, started_at, finished_at, duration, progress_pct (0..100), pipeline: PipelineView{roots: PipelineNode[]}, workers: WorkerInfo[], timeline: Event[], observed_at, degraded_reasons[] (≤32), source: ObservationSource`.

#### 3.3 monitor.PipelineNode (overview.proto:124-183) — поля с номерами
`node_execution_id=10 (≤128, = identity И ключ логов)`, `name=1`, `status=2`, `started_at=3`, `finished_at=4`, `duration=5`, `worker=6 (domain.Worker{id, kind MASTER|AGENT})`, `params=11 (schemapb.Baked)`, `log_ref=7`, `children=8 (рекурсивно)`, `attempt=9 (>1 = retry)`, `source=12`, `order=13 (1-based sibling order)`, `parent_node_execution_id=14`, `phase=15`, `component_id=16`, `machine_id=17`, `status_reason=18 (≤256)`, `error_message=19 (≤4096)`, `operation=20 (PipelineOperation)`, `outputs=21 (≤4096)`.

#### 3.4 monitor.PipelineOperation (overview.proto:218-267)
`kind: OperationKind (CREATE_DIR|WRITE_FILE|FETCH_FILE|CALL_CMD)`, `step_id`, `step_order`, `target`, `summary`, `mentions[] (≤64)`, `labels (map)`, `command: common.Cmd`, `file: common.File`, `dir: common.Dir`, `command_text (≤1 MiB)`, `argv[] (≤512)`, `file_path`, `file_size_bytes`, `content_preview (≤4096)`, `result_available`, `exit_code`, `timed_out`, `elapsed`, `stdout_preview`, `stderr_preview`, `result_summary`.

#### 3.5 monitor.PipelineOutput (overview.proto:186-215)
`kind: OutputKind (SUMMARY|RENDER_ARTIFACT|DEPLOYMENT_PLAN|COMMAND_RESULT|FILE|DIRECTORY)`, `id, name, summary, component_id, machine_id, step_id, action, target, command_text (≤1 MiB), content_preview (≤4096), count, size_bytes, labels (≤64)`.

#### 3.6 topology.Topology (topology.proto:246-299)
`state: State (SPEC|INFRASTRUCTURE_PLANNED|INFRASTRUCTURE_DEPLOYED|DEPLOYMENT_PLANNED|DEPLOYED|UNDEPLOYED|ARCHIVE)`, `spec: TopologySpec{nodes[] (Node{id, component_ids[], labels, tags}), components[], connections[], external_components[], labels, tags}`, `infrastructure_plan`, `infrastructure_state`, `deployment_plan`, `tags`, `runtime_nodes: RuntimeNode[] (≤4096)`, `runtime_connections: RuntimeConnection[] (≤8192)`.

#### 3.7 topology.RuntimeNode (topology.proto:83-166)
`id (например control-plane, machine/node-1, agent/node-1, component/postgres-master, monitor/node-1/vmagent)`, `kind: Kind (CONTROL_PLANE=1, MACHINE=2, AGENT=3, COMPONENT=4, MONITOR=5, EXPORTER=6, WORKLOAD=7, EXTERNAL=8)`, `label, engine, role, component_id, machine_id, node_execution_id, status: common.Status, status_reason, address, port?, started_at, finished_at, labels, tags`.

#### 3.8 topology.RuntimeConnection (topology.proto:175-241)
`id, from_node_id, to_node_id, kind: Connection.Kind (FLOW=1, PROXY=2, REPLICATION=3, COORDINATION=4, OBSERVATION=5, SUPPORT=6)`, `protocol: Connection.Protocol (TCP=1, GRPC=2, HTTP=3, REPLICATION=4, POOL=5, CONTROL=6, OTLP=7, PROMETHEUS_REMOTE_WRITE=8, PROMETHEUS_PULL=9)`, `mode: Connection.Mode (REQUEST=1, STREAM=2, SYNC=3, HEARTBEAT=4, BROADCAST=5)`, `endpoint_name, port?, phase, node_execution_id, status, status_reason, started_at, finished_at, labels (в т.ч. `relation`, `source`), tags`.

#### 3.9 common.Status (status.proto)
`UNSPECIFIED=0, PENDING=1, RUNNING=2, COMPLETED=3, FAILED=4, RETRY_WAIT=5, SKIPPED=6, CANCELLING=7, CANCELLED=8, ALLOCATED=9, DEPLOYMENT=10, DEPLOYED=11`.

#### 3.10 workflow.RunState / Stage (test.proto:57-140) — источник pipeline
`RunState{status, stages: Stage[]}` — **плоский список**; `Stage{node_execution_id, name, status, started_at, finished_at, attempt, order, parent_node_execution_id, phase, component_id, machine_id, worker, status_reason, error_message, operation: monitor.PipelineOperation, outputs: monitor.PipelineOutput[]}`. `StageUpdate{stage}` — сигнал Temporal от child-workflow.

---

### 4. Сервер: как рождаются pipeline-узлы

#### 4.1 Temporal TestWorkflow — 5 корневых фаз
- `internal/workflows/test.go:23-27`: константы фаз `infrastructure`, `render_deployment_plan`, `execute_deployment_plan`, `workload`, `teardown`; индексы `:47-51`.
- Инициализация состояния `newDomainTestWorkflow` (`test.go:319-334`): `RunState{Status: PENDING, Stages: [rootStage(name, order 1..5)]}`. `rootStage` (`stages.go:18-29`): `NodeExecutionId = deploymentbuilder.StageExecutionID(name)` = `"stage/<name>"`, `Phase = name`, `Attempt = 1`, `Worker = master`, `StatusReason = "workflow_stage_pending"`.
- Жизненный цикл корня: `startRootStage/completeRootStage/failRootStage/cancelRootStage` (`stages.go:159-183`) ставят status + `StartedAt/FinishedAt = workflow.Now`; `w.startStage(idx)/completeStage/failStage` (`test.go:930-945`). Порядок в `Execute` (`test.go:337-670`): infra → persist → render (child `RenderDeploymentPlanWorkflowChild`) → `registerDeploymentPlanStages` → execute (child `ExecuteDeploymentPlanWorkflowChild`) → `drainStageUpdates` → `registerDeploymentPlanStages` (перерегистрация с финальными статусами) → workload (child `RunWorkloadWorkflowChild`) → drain → complete. Teardown — в `defer` на **любом** исходе (`test.go:353-395`, disconnected ctx), при `err==nil` ставит `state.Status = COMPLETED`.
- `fail()` (`test.go:947-958`) / `cancel()` (`:960-971`): помечают **первую** PENDING/RUNNING стадию FAILED/CANCELLED и выходят.

#### 4.2 Дочерние узлы фазы `infrastructure` и `teardown` — «action stages» мастера
- `temporalActionStage(phase, parentID, order, status, started, finished, err, path...)` (`stages.go:97-116`): `NodeExecutionId = StageExecutionID(phase + "/" + path...)` → `"stage/infrastructure/acquire_quotas"`, `Name = last(path)`, `Worker = master`, без operation.
- Имена действий (`test.go:29-43`): infrastructure: `calculate_quotas`(1), `acquire_quotas`(2), `acquire_network`(3), `process_infrastructure`(4)/`render_docker_input`/`docker_pull`/`docker_up`/`render_terraform_variables`/`terraform_apply`, `commit_quotas`(5), `commit_network`(6); teardown: `render_docker_input`(1)+`docker_down`(2) или `render_terraform_variables`(1)+`terraform_destroy`(2), `release_quotas`(90), `release_network`(91).
- Создание: `w.startActionStage(ctx, phase, parentID, order, path...)` (`test.go:893-898`) → `applyStageUpdate` + persist (throttled); завершение `completeActionStage / completeActionStageWithOutputs / failActionStage` (`:900-925`) — `FinishedAt`, `ErrorMessage`, опц. `Outputs`.
- Outputs корня infrastructure: `InfrastructurePlanOutputs(plan)` + `InfrastructureStateOutputs(state)` (`test.go:585-589`; `infrastructure_outputs.go`) — SUMMARY-outputs с id `infrastructure/plan`, `infrastructure/provider_settings`, `infrastructure/machine/<node>`, `quota/requests`, `quota/request/<node>/<name>`, `quota/allocations`, `quota/allocation/<node>/<name>`, `infrastructure/state`, `infrastructure/state/machine/<node>`, `.../quota/<name>`.

#### 4.3 Дочерние узлы фазы `execute_deployment_plan` — компоненты и agent-steps
- `registerDeploymentPlanStages(runID, plan)` (`test.go:841-867`): `StampDeploymentPlanExecutionContext` (лейблы `stroppy.io/node-execution-id` и т.д., `execution_ids.go:69-138`), `sortComponentExecution`, затем для каждого компонента `componentStage(...)` (`stages.go:31-54`): `NodeExecutionId = labels[LabelNodeExecutionID] || ComponentExecutionID(id)` = `"component/<id>"`, `Name = componentID`, `ParentNodeExecutionId = "stage/execute_deployment_plan"`, `Phase = execute_deployment_plan`, `ComponentId`, `MachineId = nodeId`, `Worker = agent/<nodeId>`; и для каждого шага `agentStepStage` (`stages.go:56-95`): `NodeExecutionId = "component/<id>/step/<stepId>"`, `Name = AgentStepStageName(step)` (summary операции, ≤96 симв., `operation.go:24-34, 399-411`), `Parent = component/<id>`, `Operation = AgentStepOperation(step)`, `Outputs = AgentStepResultOutputs(component, step)` (COMMAND_RESULT после выполнения).
- Наследование статуса при регистрации: компонент UNSPECIFIED при родителе COMPLETED → DEPLOYED; шаг UNSPECIFIED при компоненте DEPLOYED → DEPLOYED (`test.go:851-864`). `deploymentRuntimeStatus` (`stages.go:144-157`): DEPLOYMENT/ALLOCATED/RUNNING→RUNNING, DEPLOYED→COMPLETED.
- Live-статусы во время execute: child workflow `internal/workflows/deployment.go` эмитит `emitStageUpdate(componentStage(... RUNNING/FAILED/COMPLETED ...))` (`:286-298`) и `agentStepStage` (`:545-560`) через сигнал `UpdateStageExternal` в родительский/корневой workflow (`stages.go:122-142`). Корень слушает `listenStageUpdates` (`test.go:807-822`) → `applyStageUpdate` (upsert по `node_execution_id`, `mergeStage` `test.go:1089-1119`: статус всегда перезаписывается, остальные поля — если не пусты, `Outputs` — если непустые) → `sortRunStages` (parent, order, id).
- ID ограничены 128 символами: `boundedExecutionID` (`execution_ids.go:154-165`) → при переполнении `"<prefix>/<sha256[:24]>"`.

#### 4.4 Дочерние узлы фазы `workload` — сегменты
- `RunWorkloadWorkflow` (`test.go:144-172`): для каждого `Workload.Segment` — отдельный agent step `workloadRunStep` (`:227`, stepId `"<order>_run_stroppy_<slug>"`), `stampWorkloadStepExecutionContext` (`:288-303`: `LabelPhase = workload`, parent = `stage/workload`), эмит `workloadAgentStepStage` (`:306-308` → `agentStepStageForPhase(..., phase="workload", parent="stage/workload")`) со статусами RUNNING → COMPLETED/FAILED и своим `StartedAt/FinishedAt`. Именно эти листья `phase=="workload"` собирает `workloadSegments` на фронте. Ошибка сегмента прерывает остальные.

#### 4.5 Проекция RunState → monitor.Overview (сервер)
- `projectOverviewWithSource(runID, rs, observedAt, source)` (`overview.go:917-1090`): по каждому Stage создаёт `PipelineNode` (копия полей; `order==0 → idx+1`; `phase==""&&parent=="" → phase=name`; `statusReason==""` → `sourceStatusReason` = `"temporal_run_state_<status>"`/`"persisted_run_state_<status>"`; `Worker` синтезируется `agent/<machineId>` если пуст; `Duration = spanDuration(started, finished, now)` — **живая** для незавершённых; `LogRef{runID, nodeExecId, componentId}`), сортирует (parent, order, index), **собирает дерево по `parent_node_execution_id`** через `nodesByID` — узел без найденного родителя становится корнем. Прогресс: `progressPct = round(completed/total*100)` где completed = стадии с `isPipelineTerminalStatus` (terminal + DEPLOYED). `StartedAt = min(started)`, `FinishedAt = max(finished)` только если run terminal. Timeline: STAGE_STARTED/STAGE_COMPLETED|FAILED|RETRYING per stage + RUN_STATUS.
- Источник `Get` (`overview.go:98-161`): порядок приоритетов
  1. `persistedRecordIsTerminal(rec)` (`:170-175`, status или runtime_state terminal) → `overviewFromRecord` (source PERSISTED_RECORD), Temporal **не** опрашивается.
  2. нет Temporal-клиента → `overviewFromRecord` + degraded `workflow_state_unavailable: temporal_client_not_configured`.
  3. `tc.GetRunState(testWorkflowID(runID))` с таймаутом `overviewRunStateQueryTimeout = 4s` (`:33`); ошибка → `overviewFromRecord` + degraded `workflow_state_unavailable: <err>`.
  4. успех → `mergeOverviewWithRecord(projectOverview(rs), rec, presence)` (`:332-358`: workers из записи; если stored status CANCELLING/terminal — перекрывает статус/время из `rec.Summary`), `snap.Topology = topologyFromRecordWithRunState(rec, rs)`, `overlayRunFromOverview` (`:886-908`, синхронизирует `rec.Status/Summary` с overview для sidebar).
- `overviewFromRecord` (`:280-330`): если есть `rec.RuntimeState` (персистированный RunState) → та же `projectOverviewWithSource(..., PERSISTED_RECORD)` + правки статуса/времени из `rec.Summary`; **duration пересчитывается живьём** пока не terminal (commit 5b116d14: замороженный таймер). Если `RuntimeState` нет → синтетический `pipelineFromRecord` (`:360-383`): 5 корней `recordStageNode` (`:581-595`, source PERSISTED_RECORD, `statusReason = persisted_record_*`) со статусами, выведенными из артефактов записи (`recordInfrastructureStatus` по machines, `recordRenderPlanStatus` по наличию deployment_plan, `recordExecutePlanStatus` = `deploymentPlanStatus(plan)`, `recordWorkloadStatus`, `recordTeardownStatus`, `:597-688`), + outputs infra/plan, + дети execute из `deploymentPlanRoot → deploymentPlanChildren → agentStepNodes` (`:708-828`, source DEPLOYMENT_PLAN, id из лейблов, порядок: global_priority, node_id, node_priority, component_id; шаги по order,id; статус `inheritedDeploymentStatus`).
- `pendingOverview` (`:266-278`): нет записи → PENDING, пустой pipeline, source SYNTHETIC.

#### 4.6 Персистенция RunState и компакция
- `persistRunState` активити → `RunPersistenceActivities.PersistRunState` (`run_persistence.go:41-80`): `rec.RuntimeState = clone(state)`, `rec.Status = nextRunStatus`, `applyRunSummary`; infra/plan только если не nil (дедуп больших payload'ов, `test.go:973-996`).
- Throttle: `persistRuntimeProjection` (`test.go:998-1012`) — не чаще `runtimeProjectionPersistMinInterval = 20s` (`:55`), кроме `isProjectionForceStage` (FAILED/CANCELLED, `:1014-1025`). Примечание: commit 5b116d14 говорил о 2s/force-on-every-completion; текущий код: 20s и force только FAILED/CANCELLED — т.е. при fallback на persisted UI может видеть до 20s отставания по шагам.
- Компакция `compactRunStateForRuntimeProjection` (`test.go:1027-1070`) применяется **и к query GetRunState, и к persist**: `op.Command/File/Dir = nil`, текстовые поля ≤ `maxRuntimeProjectionTextBytes = 512`, списки ≤ 16 (`maxRuntimeProjectionListItems`), outputs ≤ 80 (`maxRuntimeProjectionOutputs`). Значит UI никогда не получает больше 512 байт command_text/stdout на узел из RunState (для persisted-plan пути `AgentStepOperation` даёт до 2048).

#### 4.7 Runtime topology (сервер) — как строится граф для вкладки Topology
- Вход: `topologyFromRecordWithRunState(rec, rs)` (`overview.go:232-247`) — envelope из записи + `t.State = topologyState(rec)` (`:251-264`: DEPLOYED если есть deployment_plan, INFRASTRUCTURE_DEPLOYED если infra_state, INFRASTRUCTURE_PLANNED если infra_plan, SPEC если spec) + `runtimeTopologyFromRecord(rec, rs)` (`runtime_topology.go:33-64`).
- Порядок сборки `runtimeTopologyBuilder` (map по id, повтор = merge `mergeRuntimeNode/mergeRuntimeEdge` `:848-942`, статус по рангу `runtimeStatusRank` `:953-970`: FAILED/CANCELLED 100 > CANCELLING/RETRY_WAIT 90 > RUNNING/DEPLOYMENT/ALLOCATED 80 > COMPLETED/DEPLOYED 70 > SKIPPED 60 > PENDING 10):
  1. `addControlPlane` (`:80-99`): id `control-plane`, KIND_CONTROL_PLANE, label «stroppy server», engine stroppy, role control-plane, `Address = serverAddr` (лейбл `stroppy.io/server-addr` из spec/plan/infra labels, `:66-78`), статус `controlPlaneRuntimeStatus` (`:1064-1076`).
  2. `addMachineAndAgent` для `runtimeMachineIDs` (объединение spec.nodes, infra_state.machines, plan.components.node_id, stages.machine_id; `:1025-1048`): узел `machine/<id>` (KIND_MACHINE, engine = provider_resource_id, `Address = machineHost` — endpoint private→public→любой, `overview.go:562-579`), узел `agent/<id>` (KIND_AGENT, `NodeExecutionId = currentExecutionID` из `currentExecutionByNode(plan)`), рёбра `agent→control-plane` `agent_control` (SUPPORT/CONTROL/HEARTBEAT, relation `agent_control`) и `/api/binaries` (SUPPORT/HTTP/REQUEST, relation `binary_cache`).
  3. `addComponents` (`:179-249`): узел `component/<id>` (kind по `runtimeComponentKind` `:1174-1187`: AGENT→AGENT, MONITOR→MONITOR, WORKLOAD→WORKLOAD, EXTERNAL→EXTERNAL, иначе COMPONENT; label `"<id> (<role>)"`; статус `componentRuntimeStatus` `:1117-1125`), рёбра `machine→component` `placement` и `agent→component` `agent_execution` (оба SUPPORT/CONTROL).
  4. `addLogicalConnections` (`:251-308`): из `spec.connections` → рёбра `component→component` с дефолтами kind FLOW / protocol TCP / mode REQUEST, `endpoint_name = "<kind>/<protocol>"`, статус `logicalConnectionStatus` (`:1135-1157`), синтетический EXTERNAL-узел для неизвестного id (`addSyntheticExternalComponent` `:765-778`).
  5. `addComponentDependencies` (`:325-353`): `depends_on` (SUPPORT/CONTROL), направление component→dependency.
  6. `addMonitoringRuntime` (`:355-386`, только при непустом serverAddr): на каждую машину с компонентами — узлы `monitor/<m>/node_exporter` (EXPORTER, :9100), `monitor/<m>/vmagent` (MONITOR, engine victoriametrics), `monitor/<m>/vector` (MONITOR); рёбра vmagent→node_exporter (`OBSERVATION/PROMETHEUS_PULL`, endpoint `node`, port 9100), vmagent→control-plane (`PROMETHEUS_REMOTE_WRITE/STREAM`, endpoint `/insert/0/prometheus/api/v1/write`), vector→control-plane (`HTTP/STREAM`, `/insert/jsonline`), vector→component (`logs/journald_and_files`, STREAM); БД-экспортеры по `databaseMetricKind(engine, role)` (`:1200-1223`): postgres master/replica → `postgres_exporter` (:9187) + ребро exporter→component `postgres_local` :5432; mysql primary/replica → `mysqld_exporter` :9104/:3306; picodata instance → прямой скрейп :8081; ydb storage :8765 / database :8766.
  7. `addStages(rs)` (`:388-423`): для каждой Temporal-стадии с machine_id — upsert machine+agent (`NodeExecutionId = stage id`); для стадии с component_id **без** operation — upsert component с `Status/StartedAt/FinishedAt` стадии (source `temporal_run_state`); `addStageAction` (`:425-499`) для стадий с operation: ребро `agent→(component|machine)` `agent_action` (SUPPORT/CONTROL, labels из `operationLabels`), плюс по «фактам» из текста операции (`operationRuntimeFacts` `:1225-1257`: подстроки node_exporter, postgres_exporter, mysqld_exporter, vmagent|/etc/vmagent|prometheus/api/v1/write, vector|/etc/vector|/insert/jsonline, /api/binaries/) — upsert соответствующих монитор-узлов со статусом/временем стадии и рёбер `agent_action` → monitor.
  8. `sortedNodes` (kind, machine_id, id), `sortedEdges` (from, to, id).
- ID рёбер: `runtimeEdgeID(parts...)` = join "/" (>240 симв. → обрезка + fnv32) (`:1360-1368`).

---

### 5. Вкладка PIPELINE — PipelineViewer.tsx

Компонент: `PipelineViewer({pipeline: PipelineNodeVM[], onOpenLogs?})` (`PipelineViewer.tsx:146-314`). Заголовочный комментарий (`:1-6`) устарел частично: «Unknown phases fall into an Other band» — **в коде нет band «Other» и нет отбрасывания фаз**: ровно один band на каждый корень `pipeline.roots`; фиксированного списка фаз нет (см. §8).

#### Фича 5.1. Плоский список + прогресс-бар
- **Зачем**: мгновенно видеть done/total/err по всему рану.
- **Как**: `flatten(pipeline)` (`:60-67`) — DFS, siblings сортируются по `order` (стабильно), каждому узлу добавляется `depth`. `total = flat.length`, `done = count(bucket==="done")`, `failed = count("failed")` (`:193-195`).
- UI (`:215-230`): полоса `h-1` шириной `done/total*100%`, цвет: `failed>0 → bg-destructive`, `done===total → bg-success`, иначе `bg-primary`; справа `done/total` и `N err` (красным). Переход `transition-all duration-700`.
- **Крайние случаи**: `total===0` → empty state (`:204-211`): спиннер + «Waiting for pipeline steps…» (на всю высоту, центр). Это же состояние, пока `overview` ещё null.

#### Фича 5.2. Bucket статусов и точки (StepDot)
- `bucket(status)` (`:87-93`): completed→done, running→running, failed→failed, cancelled|cancelling→cancelled, иначе pending. (Т.е. RunStatus `"cancelling"` в pipeline рисуется как cancelled.)
- `StepDot` (`:103-134`): круг 14px: done = зелёный с `Check`; failed = красный с `X`; cancelled = `bg-pending` с `X`; running = primary с `Loader2 animate-spin`; pending = серый с точкой.
- `BAND_STYLE` (`:95-101`): border/bg/text для done/running/failed/cancelled/pending.

#### Фича 5.3. Band на каждую корневую фазу
- **Зачем**: группировка по фазам (Infrastructure / Render Deployment Plan / Execute Deployment Plan / Workload / Teardown), как в legacy DAG-view.
- **Как**: `bands = pipeline.map(root => ({id: root.nodeExecutionId || root.name, label: humanize(root.phase || root.name) || "—", icon: iconFor(root.phase || root.name), root, steps: flatten(root.children)}))` (`:157-167`). `humanize` (`:76-78`): `_`/`-` → пробел, Capitalize Each Word (`execute_deployment_plan` → «Execute Deployment Plan»).
- Иконка по ключевым словам `iconFor` (`:34-44`, regex по lowercase имени): infra|network|machine|provision → `Server`; teardown|cleanup|destroy|undeploy → `Trash2`; monitor|metric|observ → `BarChart3`; workload|stroppy|benchmark|runner|run\b → `Play`; postgres|mysql|ydb|picodata|etcd|database|\bdb\b|master|replica|proxy|pgbouncer → `Database`; deploy|render|execute|plan → `Zap`; иначе `Circle`. Порядок проверок важен (например «render_deployment_plan» — сначала не совпадает с infra…, попадает в `Zap`).
- Статус band: `groupBucket(subtree)` (`:136-144`) над `[root, ...steps]`: any failed → failed; any cancelled → cancelled; all done → done; any running|done → running; иначе pending. (Заметь: band с частью done и остальным pending считается «running».)
- Заголовок band (`:243-289`): `<button>` на всю ширину; содержимое: [hover-кнопка логов] `StepDot(root.status)` + иконка (цвет `style.text`) + label (`font-mono text-xs font-semibold truncate`) + `fmtDur(root.durationSec)` + (если есть шаги) `groupDone/subtree.length` + `ChevronDown` (rotate-180 при open).
- `hasSteps = steps.length>0 || root.outputs.length>0` (`:240`) — без шагов и outputs band не кликабелен (`cursor-default`).
- `fmtDur(sec)` (`:80-84`): пусто если ≤0/NaN; `<60 → "Ns"`, иначе `"Mm Ss"`.

#### Фича 5.4. Авто-раскрытие активных band + ручной override
- **Зачем**: на живом ране автоматически показывать то, что сейчас выполняется/упало, без потери ручного выбора пользователя.
- **Как**: `autoOpen` (`:171-180`) — Set id band'ов, где в `[root,...steps]` есть узел running или failed; пересчитывается при каждом новом `pipeline`. `overrides: Map<string, boolean>` (`:181`) — ручные клики; `isOpenBand(id) = overrides.get(id) ?? autoOpen.has(id)` (`:182`). `toggle(id)` (`:197-202`) пишет в override инверсию текущего эффективного состояния.
- **Крайние случаи**: override «закрыть» переживает live-обновления (auto больше не откроет). Ключ band — `nodeExecutionId || name`; при смене source (temporal↔persisted) id корней одинаковые (`stage/<name>`), состояние сохраняется. Ничего не персистится в localStorage.

#### Фича 5.5. Содержимое раскрытого band
- `:290-307`: если `root.outputs.length>0` — сверху `OutputsDetail(root.outputs)` (отделено `border-b`), затем `StepRow` на каждый flatten-узел (`key = nodeExecutionId || name`).

#### Фича 5.6. Строка шага (StepRow) — отступ по глубине, метки, раскрытие
- `StepRow({node: FlatNode, expanded, onToggle, onOpenLogs})` (`:318-407`).
- Отступ: `paddingLeft = 2 + depth*14` px (`:336`) — depth относительно детей корня (компонент=0, agent step=1, и т.д.).
- Label: `op?.summary || humanize(node.name) || "—"` (`:331`), `title = op?.target || label`; красный текст при failed. Слева: [hover-кнопка «View in Logs», только если `onOpenLogs && nodeExecutionId`] + `StepDot` + иконка операции `opIcon(kind)` (`:47-55`: create_dir→FolderPlus, write_file→FileText, fetch_file→Download, call_cmd→Terminal, иначе Circle; только если есть `operation`).
- Справа: до 3 `op.mentions` (бейджи, `hidden md:inline`), `machineId` (`hidden lg:inline`, если длиннее 14 симв. — `…<последние 14>`, title `machine: <id>`), `×N` жёлтым если `attempt>1`, `fmtDur(durationSec)`, `ChevronDown` если `hasDetail`.
- `hasDetail = !!op || !!errorMessage || outputs.length>0` (`:330`) — только тогда строка кликабельна (`cursor-pointer`) и открывает панель деталей. Фон `bg-primary/[0.05]` у running-строки.
- Состояние раскрытия: `openSteps: Set<string>` в родителе (`:185-191`), ключ `nodeExecutionId || name`, не сбрасывается live-обновлениями.
- **Крайние случаи**: узел без `operation`, без ошибки и без outputs (например action stage `acquire_quotas`, компонент `postgres-master`) — не раскрывается, показывает только статус/длительность. `statusReason` показывается только внутри раскрытой панели, т.е. для нераскрываемых узлов не виден вообще.

#### Фича 5.7. Панель деталей шага
- `:388-404`, вложена под строкой (`ml-5 border-l pl-3`):
  1. `statusReason` — серым моно (`:390-392`).
  2. `errorMessage` — `<pre>` красная рамка, `whitespace-pre-wrap break-all`, + `CopyBtn("copy error")` (`:393-400`).
  3. `OperationDetail(op)` (`:409-478`):
     - строка `step: <stepId>` `#<stepOrder>` (если есть);
     - CALL_CMD: `<pre max-h-48>` с `commandText || argv.join(" ")` + `CopyBtn("copy command")`;
     - если `resultAvailable`: `exit <code>` (зелёный если `exitCode===0 && !timedOut`, иначе красный), `timed out` (жёлтый), `fmtDur(elapsedSec)`, `resultSummary`; затем `<pre>` со `stdoutPreview` + `"stderr:\n"+stderrPreview`;
     - WRITE_FILE/FETCH_FILE: `filePath` + `· fmtBytes(fileSizeBytes)` (`fmtBytes` `:69-74`: B/KiB/MiB с 1 знаком), `<pre>` с `contentPreview`;
     - CREATE_DIR: `target`;
     - все `mentions` бейджами.
  4. `OutputsDetail(outputs)` (`:480-508`): карточки (макс **120**, далее «+N more outputs»): строка `kind` | `name||id||target||"output"` | `componentId` | `action` | `count N` | `fmtBytes(sizeBytes)`; затем `summary`, `target` (break-all), `<pre max-h-32>` с `commandText || contentPreview`. Ключ карточки `id || kind-name-target`.
- `CopyBtn` (`:510-528`): `navigator.clipboard.writeText`, 1.2s показывает «copied» с галочкой, `stopPropagation` (не сворачивает шаг).
- Нет: hover-tooltip'ов кроме `title`, клавиатурной навигации (только нативные button'ы), легенды статусов, выбора узла (selection) — единственное «выделение» = раскрытие.

#### Фича 5.8. Outputs на уровне корня (Infrastructure / Render Deployment Plan)
- Корни `infrastructure` и `render_deployment_plan` имеют `outputs` (§4.2, §4.3: `DeploymentPlanOutputs` — DEPLOYMENT_PLAN «rendered N components, M steps (...)», по компоненту SUMMARY «rendered N steps on <node>», по шагу RENDER_ARTIFACT с `commandText/contentPreview`; `operation.go:108-193`, лимит `maxPipelineOutputs = 256`, после компакции ≤80). Показываются в раскрытом band над шагами (Фича 5.5); band раскрывается даже без шагов благодаря `hasSteps` включающему outputs.

---

### 6. Вкладка TOPOLOGY — TopologyViewer.tsx

Компонент `TopologyViewer({topology?: TopologyJson})` (`TopologyViewer.tsx:733-767`). Библиотека `@xyflow/react` (+ `dist/style.css`). `build(topo)` (`:728-731`): если `runtimeNodes.length>0` → `buildRuntime`, иначе `buildSpec` (fallback на логический spec для ранних снапшотов).

#### Фича 6.1. Canvas и управление
- `<ReactFlow nodes edges nodeTypes fitView fitViewOptions={{padding:0.16}} nodesDraggable={false} nodesConnectable={false} elementsSelectable={false} panOnDrag zoomOnScroll minZoom=0.2 maxZoom=2 proOptions={{hideAttribution:true}} colorMode="dark">` + `<Background Dots gap=20 size=1 color=#1a1a1a>` + `<Controls showInteractive={false}>` (zoom in/out/fit; классы `!border-border !bg-card`) (`:746-764`).
- **Нет**: выделения/клика по узлу, сайдбара деталей, тогглов слоёв, мини-карты, легенды. Единственная «деталь» — нативный `title` на карточке (hint).
- Empty state (`:736-742`): `nodes.length===0` → «Topology not available for this run» (моно, центр).
- Пересборка `useMemo(() => build(topology), [topology])` — каждый стрим-фрейм; `fitView` применяется xyflow при первом монтировании (последующие обновления позиций не «прыгают» камерой, т.к. layout детерминирован).

#### Фича 6.2. Типы узлов (custom nodeTypes `{card, group, server}`, `:299`)
- `CardNode` (`:221-233`): flex-row: точка статуса (`data.status` цвет) + заголовок (`data.title`, цвет `data.accent`, mono 11px semibold) + подзаголовок (`data.subtitle`, 9px zinc-500); `title={data.hint}`. Содержит `SideHandles` (`:210-219`): 4 невидимых handle'а (`portCls` opacity-0): `tl` target-left 34%, `sl` source-left 66%, `tr` target-right 34%, `sr` source-right 66%.
- `GroupNode` (`:235-246`): контейнер машины; в углу точка статуса + `title` (uppercase 10px) + `host` (9px zinc-600).
- `ServerNode` (`:276-297`): как Card, но handle-«банки»: на каждую сторону `l|r` по `max(slots,1)` пар `<side>t<i>`/`<side>s<i>`, распределённых `hemiSlot` (`:256-271`) по полуокружности: до `floor(n/4)` (max 4) сверху, столько же снизу, остальные по боковой грани; top/bottom слоты в диапазоне `edgeBase + k*36%` где `edgeBase = 8% (l) | 56% (r)`.
- Геометрия (`:83-95`): `CHILD_W=248, CHILD_H=52, CHILD_GAP=14, GROUP_PAD_X=16, GROUP_PAD_TOP=38, GROUP_PAD_BOTTOM=16, RAIL_W=150, RAIL_TRACK=16, GROUP_W=CHILD_W+GROUP_PAD_X+RAIL_W=414, MACHINE_GAP=52, COL_GAP=230, CONTROL_W=264, CONTROL_H=64`.

#### Фича 6.3. Цвета
- `RUNTIME_KIND_COLOR` (`:32-42`): CONTROL_PLANE #f97316 (orange), MACHINE #64748b, AGENT #a1a1aa, COMPONENT #3b82f6 (blue), MONITOR #a78bfa, EXPORTER #c084fc, WORKLOAD #22c55e (green), EXTERNAL #94a3b8, UNSPECIFIED #525252.
- `SPEC_KIND_COLOR` (`:44-55`, только fallback): DATABASE #3b82f6, REPLICA #60a5fa, PROXY #eab308, MONITOR #a78bfa, AGENT #71717a, WORKLOAD #22c55e, COORDINATOR #7c6cc8, ADDON #94a3b8, EXTERNAL #64748b.
- `CONN_COLOR` (`:57-65`): FLOW #3b82f6, PROXY #eab308, REPLICATION #60a5fa, COORDINATION #7c6cc8, OBSERVATION #a78bfa, SUPPORT #737373.
- `STATUS_COLOR` (`:67-80`): RUNNING/DEPLOYMENT/ALLOCATED/RETRY_WAIT #f59e0b (amber); COMPLETED/DEPLOYED #22c55e; FAILED/CANCELLED/CANCELLING #ef4444; SKIPPED/PENDING #71717a; UNSPECIFIED #525252.
- Карточка (`cardNode` `:607-638`): `background = accent+"12"` (alpha), `border 1px accent+"55"`, `borderLeft 3px <status color>`, radius 4. Group: `background #0d0d0d`, `border 1px #2a2a2a`, radius 6.
- Ребро `edgeColor` (`:130-136`): FAILED/CANCELLED/CANCELLING → красный статуса; «quiet support» → #4d4d4d; RUNNING/DEPLOYMENT/ALLOCATED → amber; иначе `CONN_COLOR[kind]`.

#### Фича 6.4. Тексты узла/ребра
- `nodeTitle(n)` = `label || role || engine || id || "node"` (`:181-183`); `nodeSubtitle(n)` = `[engine, role].join(" / ")` + `address[:port]`, либо `machineId`, либо short kind (`:184-189`). Hint (title) = `statusReason || nodeExecutionId` (`:627`).
- Group: `title = machineId || label || id || "machine"`, `host = address[:port] || machineId` (`:445-452`).
- Метка ребра `runtimeConnLabel(c)` (`:152-158`): `"<protocol lower> <compactEndpoint> <port>"`, где `compactEndpoint` (`:142-150`): `/insert/0/prometheus/api/v1/write`→«metrics», `/insert/jsonline`→«logs», `/api/binaries`→«binaries», `write_file:*`→«write file», `fetch_file:*`→«fetch file».
- `visibleRuntimeConnLabel` (`:160-172`): скрыта для quiet-support и для endpoint `call_cmd|create_dir|agent_execution|placement|agent_control`. Полная метка всегда идёт в `edge.data.title` (не показывается — xyflow smoothstep по умолчанию title не рендерит; задел).
- Дедуп меток: одна метка на пару `(source|target|label)` (`seenLabel`, `:586-590`) — параллельные одинаковые рёбра не дублируют подпись.

#### Фича 6.5. «Quiet support» — приглушение служебных рёбер
- **Зачем**: agent_control/placement/depends_on/agent_action/agent_execution — сантехника, шумит; показывать пунктиром и не учитывать в layout.
- `quietSupport(edge)` (`:107-128`): `kind===KIND_SUPPORT && labels.relation ∈ {agent_control, agent_execution, placement, depends_on, agent_action}`; ИЛИ `protocol===PROTOCOL_CONTROL && endpointName` начинается с `write_file`/`fetch_file` или равен `create_dir`/`call_cmd`/`agent_action` (защита от иначе размеченного бэкенда).
- Quiet-ребро: `strokeWidth 0.9, opacity 0.3, dasharray "4 4", zIndex 0, animated=false`, без метки (`makeEdge` `:313-347`). Не участвует в union-find группировке машин и в подсчёте слотов сервера.
- Заметь: `/api/binaries` (relation `binary_cache`, HTTP) — **не** quiet, показывается как «http binaries».

#### Фича 6.6. Layout runtime-графа (`buildRuntime`, `:349-605`)
Детерминированный, без force-layout:
1. Классификация: `machines = kind KIND_MACHINE`, `controls = KIND_CONTROL_PLANE`. Остальные узлы прикрепляются к машине по `machineId` (`machineByMid`), иначе — `orphanNodes` (`:358-376`).
2. Порядок детей в машине (`:365-378`): AGENT(0) → WORKLOAD/COMPONENT(1) → EXPORTER(2) → MONITOR(3) → прочее(4), затем по id. Мотив: источники метрик выше стоков (exporter над vmagent, vector последним) → внутренние рёбра идут вниз, короткие.
3. Группировка машин union-find по **не-quiet** рёбрам между компонентами разных машин (`:385-409`) — кластер (etcd+patroni+replica) держится на одной стороне.
4. Балансировка сторон L/R (`:412-426`): группы сортируются по убыванию размера, каждая идёт на менее нагруженную сторону (`loadL <= loadR → L`); машины группы contiguous.
5. Колонки (`:433-436`): `xLeft=0`, `xServer=GROUP_W+COL_GAP=644`, `xRight=xServer+CONTROL_W+COL_GAP=1138`.
6. `place(side, x, mids)` (`:438-466`): машины стеком по y с `MACHINE_GAP`; высота `machineH(n)=GROUP_PAD_TOP + max(n,1)*CHILD_H + max(n-1,0)*CHILD_GAP + GROUP_PAD_BOTTOM`; дети `parentId=machine`, `extent="parent"`, x = `RAIL_W` (левая колонка: карточки прижаты к серверу, рейл слева) или `GROUP_PAD_X` (правая колонка), y = `GROUP_PAD_TOP + i*(CHILD_H+CHILD_GAP)`.
7. Сервер (`:474-499`): считаются server-bound не-quiet рёбра по сторонам `nL/nR`; `serverH = max(CONTROL_H, min(max(nL,nR)*24+20, max(totalH,160)))`; y центрируется по `totalH/2`; `type="server"`, `data.slotsL/slotsR`. Несколько control-plane — стеком с шагом `serverH+24` (на практике один).
8. Orphans (узлы без машины, не machine/control) — под правой колонкой (`:502-507`), стек карточек.
9. Рёбра (`:510-602`): пропуск если нет source/target, self-loop или неизвестный узел. Три класса:
   - **internal** (одна машина): handles `s<r>/t<r>` где `r = railSuffix(source)` (`R`-сторона → `r`, иначе `l`), рейл-дорожки: quiet → `offset 10`; иначе `offset = 22 + track*RAIL_TRACK`, track инкрементируется на сторону (веер, не слипаются).
   - **crossSameSide** (разные машины, одна колонка): те же рейл-handles, `offset = RAIL_W+56` (шина за пределами боксов), `behind=true` (zIndex 0).
   - иначе (cross-column / к серверу): `endHandle` (`:524-533`): для сервера — свежий слот `<side><s|t><i mod count>` на стороне партнёра; для карточки — `s|t` + `facing` (`:513-518`: L-колонка → `r`, R-колонка → `l`); `offset 26`.
   - `makeEdge`: `type "smoothstep"`, `pathOptions {borderRadius 10, offset}`, `markerEnd ArrowClosed 11×11 цвет ребра`, `animated = !quiet && (activeStatus(status) || mode==="MODE_STREAM")` (`:599`; `activeStatus` `:138-140`: RUNNING/DEPLOYMENT/ALLOCATED), stroke 1.3/opacity .92, label style mono 9px #c8c8c8 на фоне #0a0a0a, zIndex 6 (не-quiet).
   - id ребра: `c.id ?? "<src>-><tgt>-<n>"`.

#### Фича 6.7. Fallback на логический spec (`buildSpec`, `:642-713`)
- Когда `runtime_nodes` пуст (сервер не смог собрать — напр. `rec==nil`; на практике `runtimeTopologyFromRecord` всегда даёт хотя бы control-plane при наличии записи).
- Машины из `spec.nodes` (id `m:<id>`, group, status серый #71717a, одна левая колонка), компоненты по `node.component_ids` (карточка `specCard`: title `role || kind`, subtitle `engine || kind`, цвет `SPEC_KIND_COLOR`), orphan-компоненты и `external_components` в правой колонке (x = `GROUP_W+COL_GAP`), начиная с середины высоты.
- Рёбра из `spec.connections` (`from_component_id → to_component_id`), `pickHandles` (`:307-311`: internal → `sl/tl`; иначе по x), цвет `CONN_COLOR[kind]`, метка `specConnLabel` = `"<protocol>:<port>"` или kind, `animated` для FLOW/REPLICATION. Статусов нет.

---

### 7. Sidebar-связка (для полноты)
`RunInfoSidebar.tsx:126` `countPipeline(overview.pipeline)` → счётчики run/ok/fail/wait/cancel по всем узлам + `overview.progressPct` (`:262-269, 304-308`). Т.е. те же `PipelineNodeVM` питают и сводку.

---

### 8. Фазы: что группируется и что «выпадает»
- Фиксированный набор корней сейчас задаёт **сервер/Temporal**: `infrastructure`(1), `render_deployment_plan`(2), `execute_deployment_plan`(3), `workload`(4), `teardown`(5). UI ничего не фильтрует и не переименовывает кроме `humanize`.
- Legacy (commit 74191880, 2026-05-31): был `stagesToNodes/canonicalPhase` — маппинг лейблов Temporal-стадий на «канонические phase id SPA» с агрегацией per-target стадий в один узел, и панель `effective_configs`. В редизайне 960925ce (2026-06-04) PipelineViewer переписан как «data-only port of the legacy DAG view» — без effective-config панели (снапшот её не несёт; комментарий `PipelineViewer.tsx:4-6`), с деревом bands из `pipeline.roots`. Упоминание «Other band» в комментарии — рудимент; реального «Other» нет: любая новая корневая фаза автоматически станет band'ом (label = humanize(phase||name), иконка по regex).
- Что *визуально* отсутствует: узлы без `nodeExecutionId` не имеют кнопки логов; узлы без operation/error/outputs не раскрываются (statusReason скрыт); `worker`, `params`, `log_ref`, `source` узла не отображаются (только на уровне run в шапке).
- Фаза `bootstrap`/`install` как отдельных корней нет: bootstrap агентов — часть `execute_deployment_plan` (agent steps), workload-bootstrap — сегмент внутри `workload`.

---

### 9. Крайние случаи / ловушки (сводно)
1. **Отставание persisted-пути**: если Temporal-запрос упал/таймаут 4s или ран terminal — данные из `rec.runtime_state`, который пишется не чаще 20s (force только FAILED/CANCELLED). Шапка покажет бейдж `persisted` и degraded-строку.
2. **Компакция**: RunState на проводе обрезан (512 байт текстов, 16 argv/mentions, 80 outputs, без raw Cmd/File/Dir). Полные тексты команды/файлов есть только в `rec.deployment_plan` (persisted-plan путь `pipelineFromRecord`, лимиты 2048), но он используется лишь когда `runtime_state` отсутствует.
3. **Дерево по parent id**: узел с `parent_node_execution_id`, не найденным в списке, всплывает корнем (`overview.go:1038-1042`) → лишний band.
4. **Идентичность ключей**: React-key узлов = `nodeExecutionId || name`; при дублях имён без id — коллизии ключей.
5. **groupBucket «running» при частичном done**: band с done+pending без running показывается как running (синий).
6. **Cancelling** на уровне узла рисуется как cancelled (серый X).
7. **Topology**: узлы без `id` фильтруются (`:350`); рёбра к неизвестным узлам отбрасываются; несколько control-plane сдвигаются вниз; если серверный `serverAddr` пуст — нет мониторинговых узлов/рёбер (только stage-facts могут их добавить).
8. **Topology перерисовывается каждую секунду** при стриме (полный rebuild узлов; xyflow держит камеру).
9. **URL-фильтры логов** полностью сбрасываются при jump из pipeline (в т.ч. `q`, временное окно `from_ts/to_ts` НЕ удаляется — их нет в списке `openLogsForNode`; они остаются).
10. **hemiSlot**: при `count<4` все слоты на боковой грани; при большом числе рёбер `i % count` циклит слоты.
11. `fmtDur` скрывает 0/отрицательные; `spanDuration` на сервере клампит отрицательные в 0.
12. **statusToVM**: RETRY_WAIT → «running» (UI не различает retry), `attempt` всегда 1 у Temporal-стадий (`stages.go` ставит `Attempt: 1`; ×N бейдж практически не появляется).

---

### 10. История/мотивы (git)
- `74191880` 2026-05-31 — миграция на Temporal; появились `monitor/overview.proto`, `topology.proto`; overview-grouping через canonicalPhase.
- `960925ce` 2026-06-04 — редизайн Run Detail: sidebar + единый view-switcher со стейтом в URL, PipelineViewer (bands + jump-to-logs), TopologyViewer на xyflow, `run_overview` отдаёт topology.
- `5e36b806` 2026-06-04 «Improve test run observability» — `runtime_topology.go`, расширение PipelineNode (operation/outputs/phase/order/parent/source), RuntimeNode/RuntimeConnection в topology.proto.
- `fbfe3bca` 2026-06-04 — правки observability + запуск stroppy workload (per-segment stages).
- `5b116d14` 2026-06-10 — «UI showed frozen statuses»: overviewFromRecord пересчитывает duration живьём; taймаут Temporal-запроса 4s; force-persist/throttle (в текущем коде: 20s, force FAILED/CANCELLED).

---

# 4. Logs + Metrics + Grafana + window


Инвентаризация фич для переписывания. Репо `stroppy-cloud` @ `cf16c9c0` (main).
Все пути от корня репо. Формат каждой фичи: **Зачем → Как сделано → Данные/API → Сервер → Крайние случаи**.

Ключевые файлы:

| Слой | Файл |
|---|---|
| Страница | `web/src/pages/RunDetail.tsx` (VIEW_OPTIONS 122-131, view/URL 152-190, window-селектор 239-247 + 531-548, metrics fetch 275-297, grafanaSeen 384-388, рендер вкладок 550-580) |
| Logs UI | `web/src/components/run/LogsPanel.tsx` (882 строк) |
| Metrics UI | `web/src/components/run/MetricsPanel.tsx` (242) |
| Grafana UI | `web/src/components/run/GrafanaPanel.tsx` (187) + `web/src/services/grafana.ts` (UID-карта, `buildEmbedUrl`) |
| Data layer | `web/src/services/run_overview.ts`: `getRunMetrics` 553-597, `workloadSegments` 610-624, `toLogFilter` 626-648, `getLogFacets` 662-684, `queryLogs` 686-712, `logLineToVM` 742-761, `logCursorFromKey` 768-779, `streamLogs` 787-838, `resolveLogRef` 840-855, `ensureGrafanaSession` 530-535 |
| UI-примитивы | `web/src/components/ui/multi-filter.tsx`, `web/src/components/ui/datetime-picker.tsx` (props 148-165), `web/src/components/ui/segmented.tsx` |
| Proto | `protocols/cloud/v1/api/test_run_overview.proto` (LogFilter 45-124, QueryLogs 210-256, StreamLogs 263-282, ResolveLogRef 289-316, GetRunMetrics 322-345, GetLogFacets ~350-390, GrafanaSession ~392-420); `protocols/cloud/v1/monitor/logs.proto` (Source/Stream/LogCursor/LogLine/LogRef); `protocols/cloud/v1/monitor/metrics.proto` (TimeRange/MetricSummary/RunMetrics) |
| gRPC-сервис | `internal/services/test_run_overview/service.go` (порты `LogReader`/`MetricsReader`, `authorizeRun`, хендлеры QueryLogs/StreamLogs/GetLogFacets/ResolveLogRef/GetRunMetrics/GrafanaSession) |
| Адаптеры | `internal/infrastructure/execution/monitoring.go` (LogReader против VictoriaLogs, MetricsReader против VictoriaMetrics, LogsQL builder 400-500, decodeLogLines 577-625) |
| HTTP-клиенты | `internal/infrastructure/victoria/logs_client.go` (`/select/logsql/query`, `/select/logsql/field_values`, `/insert/jsonline`), `internal/infrastructure/victoria/client.go` (`/api/v1/query_range`) |
| Каталог метрик | `internal/domain/metrics/queries.go` (MetricDef per DB), `internal/domain/metrics/collector.go` (агрегация p5–p95) |
| Ингест логов | `internal/infrastructure/execution/agent_logs.go` (ShipLogs), `internal/agent/log_sink.go` (агент: stdout/stderr команд), `internal/workflows/server_logs.go` (SOURCE_SERVER), `internal/domain/deployment/monitor.go:452-560` (vector.yaml: journald + файлы), `internal/domain/deployment/operation.go:357` (`extractMentions`) |
| Gateway | `internal/gateway/gateway.go` (маршруты), `internal/gateway/publicmetrics.go` (scoped-прокси `/public/metrics/*`), `internal/gateway/runscope.go` (HMAC run-scope cookie), `internal/gateway/monitorproxy.go` (`/grafana/*` reverse proxy) |
| Wiring | `internal/app/run.go:168-170` (readers), `:423-425` (GrafanaScopeSigner = `gateway.SignRunScope(cfg.JWTSecret, runID)`), `:779-785` (gateway cfg), `internal/app/config.go:59-74` |
| Grafana | `deployments/grafana/dashboards/*.json`, `deployments/grafana/provisioning/{datasources,dashboards}`, `deployments/grafana/init.sh`, `docker-compose.yaml:73-126`, `deployments/caddy/Caddyfile:40-43`, `web/vite.config.ts` (dev-proxy `/grafana`) |

---

### 0. Общий каркас страницы (то, от чего зависят все три вкладки)

#### 0.1 Вкладки и состояние в URL
- **Зачем.** Любое состояние вкладки (view + фильтры логов + якорь строки) — shareable ссылка; «View in logs» из Pipeline и кнопка «назад» работают через один и тот же URL.
- **Как.** `RunDetail.tsx:152-168`: `view = sp.get("view") ?? "overview"`, `setView` пишет `?view=` с `{replace:true}` и **удаляет `line`** (якорь строки транзитен — при смене вкладки сбрасывается). Порядок вкладок `VIEW_OPTIONS` (122-131): Overview, Pipeline, Topology, Agents, Quotas, **Logs**, **Metrics**, **Grafana** (иконки lucide: ScrollText / LineChart / BarChart3). Переключатель — `SegmentedControl variant="tabs"`, `segmentClassName="w-[88px]"`.
- **Высота.** `scrolls = view ∈ {overview, metrics, quotas, agents}` (379): Metrics — скроллящийся контейнер `overflow-auto p-3`; Logs и Grafana — «fill» (`overflow-hidden`, компонент сам управляет высотой).
- **Jump из Pipeline.** `openLogsForNode(nodeExecutionId)` (170-190): ставит `view=logs`, `steps=<id>`, **удаляет** comp/mach/unit/src/stream/phase/action/mention/q/line (заменяет все лог-фильтры), push (не replace) — чтобы «назад» вернул в pipeline.

#### 0.2 Данные overview и live-обновление
- `load()` (200-210) → `getRunOverview` один раз. Пока run не terminal (`completed|failed|cancelled`): `streamRunOverview` (server-stream полных снапшотов); при падении стрима — polling `getRunOverview` каждые 5 с (256-274). Из overview вкладки берут: `pipeline` (Logs: список Step-опций и сегменты), `startedAt/finishedAt` (Logs: границы календаря; Grafana/Metrics: окно), `workers` (Grafana: пилюли машин), `run.dbKind` (Grafana: выбор дашборда).
- Баннер `Degraded snapshot: …` при `overview.degradedReasons.length>0` (497-501).

#### 0.3 Селектор «window» (сегменты workload) — общий для Metrics и Grafana
- **Зачем** (коммит `9cecd39b` 2026-06-17): усреднение метрик и окно Grafana по всему run включает bootstrap/load; выбор одного workload-сегмента исключает их.
- **Как.** `workloadSegments(overview)` (`run_overview.ts:610-624`): рекурсивный обход `pipeline`, берутся **листовые** узлы (`children.length===0`) с `phase==="workload"` и непустым `startedAt` → `{name, startedAt, finishedAt}` в порядке обхода. Пусто до старта workload-стадии. Имя фазы `"workload"` — константа `stageWorkloadNodeName` (`internal/infrastructure/execution/overview.go:39`).
- Состояние `segmentSel` (`useState(-1)`, -1 = «Whole run»), **не в URL**, не персистится. `selSeg = segments[segmentSel]`; `winStart = selSeg?.startedAt ?? overview.startedAt`, `winEnd = selSeg?.finishedAt ?? overview.finishedAt` (239-247).
- UI (531-548): показывается только когда `view ∈ {grafana, metrics}` **и** `segments.length>0`; справа в тулбаре (`ml-auto`), label «window» (font-mono uppercase, zinc-600), нативный `<select>` (h-7, border zinc-800, bg #0a0a0a, mono 11px), `<option value=-1>Whole run</option>` + по опции на сегмент (`s.name`), title «Scope Grafana and metric averages to a workload segment».
- Потребители: `GrafanaPanel startedAt={winStart} finishedAt={winEnd}`; `getRunMetrics(tenant, id, selSeg ? {startedAt, finishedAt} : undefined)`.
- **Крайние случаи.** Выбранный сегмент ещё идёт (`finishedAt` undefined) → для Metrics `win` не строится (оба поля обязательны в `getRunMetrics`) → серверное окно по всему run; для Grafana `to=now&refresh=5s`. Индекс сегмента может «поехать», если pipeline перестроится (сегменты добавляются в конец — практически стабильно). Grafana ремонтирует все iframe при смене окна (см. 3.6).

---

### 1. Вкладка LOGS (`LogsPanel.tsx`)

Props: `tenantSlug, runId, pipeline: PipelineNodeVM[], runStart?, runEnd?` (ISO).

#### 1.1 Модель фильтров в URL (единственный источник правды)
- **Зачем.** Любой отфильтрованный вид — ссылка; pipeline-jump пишет те же параметры.
- **Параметры** (177-206): `steps` (node_execution_id), `comp` (component_id), `mach` (machine_id), `unit`, `src` (токены `command|journald|file|server`), `stream` (`stdout|stderr`), `phase`, `action`, `mention`, `q` (поиск), `from_ts`/`to_ts` (ISO), `line` (якорь = cursorKey). Многозначные — CSV (`csv()` 80). Все `setSp(..., {replace:true})` (`setParam` 208-221).
- `clearFilters` (223-232) чистит все 12 ключей (`line` не трогает).
- `totalActive` (620) = сумма размеров всех Set + q + from + to → показывает кнопку «Clear» и префикс `N/` в счётчике строк.
- Маппинг в серверный `LogFilter` — `serverFilter()` (296-312): `search`, `nodeExecutionIds`, `componentIds`, `machineIds`, `units`, `sources` (enum), `streams` (enum), `phases`, `actions`, `mentions`, `start`, `end`. `filterKey` (314-331) — JSON тех же полей, ключ для эффектов «перезапросить».
- Не используются UI, но есть в proto: `query` (raw LogsQL), `nodeIds` (legacy = machine), `unit` (single, legacy), `parentNodeExecutionIds`, `stageNames`, `stepIds`.

#### 1.2 Дропдауны-фильтры (`MultiFilter`) с кросс-счётчиками
- **Зачем.** Legacy LogStream-поведение: Step/Component/Machine/Unit с count-ами и цветами машин; после `fc497684` (2026-07-01) значения берутся с сервера по всему run, а не из загруженной страницы.
- **Ряд 1 тулбара** (681-716), порядок: Step (Zap), Component (FileText), Machine (Server), Unit (Cpu), Source (Terminal), Stream (Radio), Phase (Radio), Action (FileText), Mention (Tags), затем «Clear» (X). Step/Component/Machine/Unit/Phase/Action/Mention рендерятся **только если есть опции**; Source и Stream — всегда.
- `MultiFilter` (`ui/multi-filter.tsx`): Popover, summary «All» (пусто = все) / label одной / «N selected», выделение рамкой primary при активном; toggle по значению; «select all» = пустой Set; опции `{value,label,count?,color?}`.
- **Источник опций** (561-618):
  - Серверные facets `facets[field]` для `node_execution_id, component_id, machine_id, unit, phase, action` (+ `step_id, stage_name` приходят, но UI не использует). Fallback — счётчики по загруженному буферу `lines`, если facet-поле отсутствует (нет мониторинга / ещё не загрузились).
  - Всегда добавляются **выбранные** значения с count 0 (иначе активный фильтр сам себя убирает из списка).
  - Step: пересечение `flattenNodes(pipeline)` (55-65: `{id: nodeExecutionId, label: humanize(name) || id.slice(0,8)}`) с facet-ключами (или выбранными) → label человеческий, не id.
  - Machine: цвет по `MACHINE_COLORS` (34-37, 8 tailwind-классов, назначаются в порядке первого появления через `colorMap` ref, не сбрасывается).
  - Unit: label без суффикса `.service`.
  - Phase/Action: `humanize()` (снейк/кебаб → Title Case).
  - Source/Stream: фиксированные списки `SOURCE_FILTERS`/`STREAM_FILTERS` (39-49) с counts **из буфера** (не facet).
  - Mention: только из буфера (на сервере хранится comma-joined, field_values даёт мусор) ∪ выбранные.
  - Сортировка: value-строки `sort()`; Step — в порядке pipeline.
- Facets перезапрашиваются на каждый `filterKey` (426-440) через `getLogFacets(serverFilter())` — кросс-сужение как у запроса; ошибка → `{}`.

#### 1.3 Поиск по тексту
- Ряд 2 (719-738): input `Search logs…` (w-40 → w-56 на фокус), применяется **по Enter** → `q`; крестик очищает input и `q`. `searchInput` синхронизируется с URL (`useEffect` 283) — при jump/back поле обновляется.
- Сервер: `_msg:"<q>"` — подстрочный матч VictoriaLogs (`monitoring.go` buildLogsFilter). Клиент: `<Highlight>` (67-78) подсвечивает **первое** вхождение (case-insensitive) `<mark bg-warning/30>`.

#### 1.4 Временное окно From–To
- **Зачем** (`ea643001`, `cf989bfa`, `353b2829`, `3309382b`, `d7335add`): читать историю в диапазоне; нативный datetime-local был уродлив/AM-PM → свой shadcn-пикер 24h с аналоговыми часами.
- `DateTimePicker` ×2 (742-758): value из `from_ts/to_ts`, onChange → ISO в URL (одно событие на «отпустить» — не по каждому тику драга). `minDate/maxDate` = окно run (`calMin/calMax`, 193-196): `validRunTs` отбрасывает `0001-01-01T00:00:00Z` и годы <2000; если run ещё идёт — maxDate = сегодня. `onOpenChange={setPicking}` — пока попап открыт, live-tail **приостановлен** (иначе 150-мс flush перерисовывает весь неверсионный лог и дёргает анимацию стрелки).
- `hasRange` = есть from или to → **Live принудительно выключен** (кнопка disabled, title «Live tail disabled while a time range is set»), `jumpBottom` не включает live.
- Сервер: `filter.start/end` → `start/end` LogsQL-запроса (RFC3339Nano); курсор дополнительно сужает соответствующую границу.

#### 1.5 Загрузка страниц и курсоры (`queryLogs`)
- API `QueryLogs{tenant_id, run_id, filter, from: LogCursor, direction: OLDER|NEWER, limit≤5000}` → `{lines[], older: LogCursor, newer: LogCursor}`. Клиент дефолт limit 200, панель шлёт **300**.
- **Первичная загрузка** `load()` (333-372): `direction: "older"` без курсора = **хвост** (newest 300, в хронологическом порядке). Если есть `?line=` — `from = logCursorFromKey(anchor)` (страница, заканчивающаяся якорем) + best-effort `newer` от якоря limit 150 (чтобы сама строка и контекст после неё попали в буфер), `appendUnique`. `anchorRef` потребляется один раз; последующие изменения фильтров грузят хвост.
- **loadOlder** (374-394): scroll-up (`scrollTop<80`) → `direction older, from: olderCursor` → `prependUnique`, сохранение позиции через `prependAnchor` (`useLayoutEffect` 512-517 корректирует `scrollTop` на разницу `scrollHeight`). Индикатор «Loading older…» сверху (807-809).
- **loadNewer** (399-416): scroll-down к низу (`<80px`) **только когда live выключен** (`87ffc10d`) → `direction newer, from: newerCursor` → `appendUnique`.
- Курсоры хранятся в ref как сырые proto (`resp.older/newer`), `older===undefined` = начало логов (стоп-пейджинг).
- Дедуп: `lineKey = cursorKey || observedAt|lineNo|machineId|line` (117-119).
- Сервер (`monitoring.go` Query 97-147): LogsQL = `buildLogsFilter` + `| sort by (_time) desc` для OLDER (потом массив разворачивается `normalizeLogPageOrder`) / `| sort by (_time)` для NEWER; окно: `filter.start/end`, затем курсор: OLDER → `end=cursor.observed_at`, NEWER → `start=cursor.observed_at`. `older = lines[0].cursor`, `newer = lines[last].cursor`. limit 0 → 200.
- **Крайние случаи.** Граница по времени **включительная** и без `seq` → повтор строк с тем же `_time` на стыке страниц (снимается клиентским дедупом); при >limit строк в одну и ту же наносекунду — бесконечное самоповторение страницы. `cursor.seq` = `line_no` из хранилища или порядковый номер в ответе (`decodeLogLines`: `lineNo==0 → seq++`), т.е. **не глобально стабилен**, если `line_no` не записан (сейчас ни агент, ни vector его не пишут → `seq` = индекс внутри страницы, зависит от страницы!). `cursorKey = "<observedAt>@<seq>"`.

#### 1.6 Live tail (стриминг)
- Кнопка «Live» (781-789): variant default+пульс иконки когда `live && !hasRange`; toggle. Старт `live = !anchorKey` (при заходе по ссылке на строку — выключен, чтобы не прыгать вниз).
- Эффект (446-488): условия `live && !hasRange && !picking && loadedFilterKey===filterKey` (стрим стартует только после того, как история под текущим фильтром загружена). `streamLogs(serverFilter(), from: newerCursor)`; каждая строка → `pendingRef`, обновляет `newerCursor`; flush таймером **каждые 150 мс** (`FLUSH_MS`) одним `setLines`, буфер обрезается до **`MAX_LIVE_LINES=5000`** (иначе O(n²) appendUnique и заморозка на 500+ строк/с). AbortController на выключение/смену фильтра/размонтирование; AbortError глотается.
- Автоскролл: следует вниз, если `live && atBottom` (порог 40 px от низа) и не идёт prepend (496-500); включение live принудительно скроллит вниз (502-509).
- Сервер `LogReader.Stream` (150-203): **polling** — каждую 1 с `Query(NEWER, limit 500, from: cursor)`, курсор двигается на `newer`; начальный курсор: `from` → как есть; иначе, если задан `filter.start` → nil (с начала окна); иначе `now`. Ошибка Query → канал закрывается (клиент увидит завершение стрима без ошибки — tail тихо умирает, Live остаётся «включён»). Отсутствие бэкенда → сразу закрытый канал.
- **Крайние случаи.** Каждый тик может вернуть дубли последней строки (включительный `start`) — дедуп на клиенте. Смена фильтра при live: стрим перезапускается после `load()`.

#### 1.7 Навигация: Home/End/PgUp/PgDn
- Кнопки ряда 2 (767-779) + клавиатура на фокусе лог-контейнера (`tabIndex=0`, `onKeyDown` 669-677).
- `jumpTop` (627-640): `live=false`, запрос `direction newer` без курсора limit 300 (самое **старое** из всех логов под фильтром), `olderCursor=undefined`, замена буфера, `jumpTargetRef="top"` → `scrollTop=0` после рендера.
- `jumpBottom` (641-654): `direction older` без курсора (самое новое), замена буфера, `scrollTop=max`, `live=true` если нет диапазона.
- `pageBy(±1)` (658-665): скролл на `500 × высота первой строки` (fallback 18 px); дальнейшую подгрузку делает `onScroll`.

#### 1.8 Рендер строки
- Контейнер (800-806): `overflow-auto border bg-black/40 font-mono text-[11px] leading-relaxed`. **Нет виртуализации** (rows.map) — отсюда cap 5000 и все «pause» костыли.
- Строка (822-874): `data-ck={cursorKey}`, `whitespace-pre-wrap` (Wrap on, дефолт) / `whitespace-pre`; hover `bg-foreground/[0.03]`; якорная строка `bg-warning/10 ring-1 ring-warning/40`.
  - Гаттер (834-855): **монотонный индекс буфера `i+1`** (серверный `lineNo` — per-node счётчик, выглядел мусором при перемешивании узлов, `cf989bfa`); на hover превращается в иконку Link2 = «Copy link to this line»; после копирования — Check зелёный 1.5 с. Кнопка disabled без `cursorKey`.
  - Время `logClock` (147-152): `toLocaleTimeString(hour12:false)`, title = полный ISO.
  - `[machineId]` цветом машины (`max-w-[10rem] truncate`).
  - `componentId` показывается, только если непуст и ≠ machineId (`showComponent`).
  - Текст: `text-destructive` для stderr; `[overflow-wrap:anywhere]` при wrap; `<Highlight>`; title = tooltip (154-171) со всеми полями: time, machine, component, source, stream, unit, action, step, phase, stage, node, mentions.
- Счётчик «`{rows}/{lines}` lines» (762-765) — при активных фильтрах префикс (сейчас rows===lines, префикс декоративный).
- Пустые/ошибки: «Loading logs…» / «No log lines.» (810-811); ошибка — красная плашка над контейнером (795-797).
- Wrap-кнопка (790-792), состояние локальное, не персистится.

#### 1.9 Deep-link на строку (`?line=<cursorKey>`)
- `copyLineLink` (268-280): берёт текущие sp + `line=cursorKey`, копирует `origin+pathname?…` в clipboard, **не** меняет текущий URL.
- Открытие: `anchorKey` → live off, `load()` грузит страницу до якоря + 150 после; `useEffect` (548-555) один раз `scrollIntoView({block:"center"})` по `[data-ck]`; подсветка держится, пока `line` в URL (сбрасывается сменой вкладки).
- `ResolveLogRef` RPC (proto + `LogReader.Resolve` — чистая проекция LogRef→LogFilter+cursor без бэкенда) — сервис `resolveLogRef` в data-layer есть, **UI не вызывает**.

#### 1.10 Экспорт / группировка
- **Нет** экспорта (download/copy all) и **нет** группировки строк (по step/machine) — только фильтры и плоский список. Единственная «группировка» — Step-опции по pipeline.

#### 1.11 Серверная сторона логов (для переписывания)
- **RPC** (`service.go`): `authorizeRun` = Authn.Caller + tenant_id/run_id непустые + `Runs.Exists(tenant, run)` (чужой run → NotFound). Permission `RESOURCE_TEST_RUN/READ` — в auth-интерцепторе.
- **Хранилище**: VictoriaLogs, AccountID **0** (общий, изоляция по полю `run_id`; см. блок ACCOUNT-ID RULE в `monitoring.go`). Endpoint `<MONITORING_URL>/select/logsql/query` через vmauth с `MonitoringToken`.
- **LogsQL-builder** `buildLogsFilter` (`monitoring.go`): `run_id:"<id>"` + для каждого списка `field:"v"` или `(field:"a" OR field:"b")` для полей node_execution_id, component_id, machine_id(=node_ids∪machine_ids), phase, parent_node_execution_id, stage_name, step_id, action, mentions, source, stream (enum→строка), unit (units ∪ unit) + `_msg:"search"` + raw `query`. Термины соединены пробелом (AND).
- **Поля записи** (JSONL, `logRow`): `_time, _msg, run_id, line_no, node_execution_id, parent_node_execution_id, phase, stage_name, component_id, machine_id, step_id, action, mentions (CSV), source, unit, stream`. `tenant_id` дописывает gateway (`monitorlabels.go rewriteJSONLineLabels`) — но фильтр по нему не используется.
- **Facets** `LogReader.Facets`: для полей `node_execution_id, component_id, machine_id, unit, phase, action, step_id, stage_name` — по запросу на каждое `/select/logsql/field_values?field=…&query=<filter>&limit=1000` (+start/end). Пустые значения выбрасываются; поля без значений опускаются. Порядок значений — как отдаёт VL (по hits desc).
- **Ингест**:
  1. Команды агента (`internal/agent/log_sink.go`): построчно stdout/stderr → `LogLine{source COMMAND, stream, run/node/parent/phase/stage/component/machine/step/action/mentions/unit}` из env шага (`EnvRunID`, `EnvNodeExecutionID`, … `EnvOperationMentions`); ship через connect `AgentLogService.ShipLogs` с agent JWT; сервер (`agent_logs.go`) сверяет run_id/machine_id с claims и пишет `LogsClient.Write` → `/insert/jsonline` (AccountID 0). `line_no` **не проставляется** (пишется только если >0).
  2. journald + файлы БД (vector на каждой машине, `monitor.go:452-560`): источники `journald` (current_boot_only) + `file` для postgres `/var/log/postgresql/*.log` и mysql `/var/log/mysql/*.log`; `reduce` multiline_join (starts_when regex даты/`:LEVEL `); remap: `run_id, machine_id, role, source=journald|file, stream=stdout, unit=_SYSTEMD_UNIT|file|source_type`; заголовки `VL-Msg-Field: message`, `VL-Time-Field: timestamp`, `VL-Stream-Fields: run_id,machine_id,role,unit,source`. **Нет** component_id/phase/node_execution_id → такие строки не попадают в фильтр Step/Component, только Machine/Unit/Source.
  3. Серверные (`workflows/server_logs.go`): terraform/docker-стадии → `SOURCE_SERVER`, `node_execution_id = actionStageExecutionID(phase, path…)`, `stage_name/action/unit = последний элемент path`, `mentions = serverStageMentions(path)` (токены по `_ - /`).
  - `mentions` для агентских шагов — `extractMentions` (`operation.go:357`): regex `[A-Za-z][A-Za-z0-9_.-]{1,63}` по тексту команды/файлов, lowercase, ≥3 симв., без shell-шума, ≤64.
- **Degrade**: `MONITORING_URL` пуст → Query возвращает пустую страницу и nil-курсоры, Stream — закрытый канал, Facets — nil (UI: «No log lines.», дропдауны из буфера = пустые).

---

### 2. Вкладка METRICS (`MetricsPanel.tsx`)

#### 2.1 Получение данных (RunDetail)
- `getRunMetrics(tenant, id, window?)` (`run_overview.ts:553-597`): `GetRunMetricsRequest{tenant_id, run_id, window?: TimeRange{start,end}}`; окно только если **оба** `startedAt&&finishedAt`. Ответ `metrics.metrics[]` → `MetricVM{key, name(=name||key), unit, avg, min, max, last, higherIsBetter, group, description}`; `num()` превращает `"NaN"/"Infinity"` JSON-строки в числа.
- Эффекты (`RunDetail.tsx:275-297`): при монтировании и смене `selSeg.startedAt/finishedAt`; пока run не terminal — **каждые 12 с** с тем же окном (overview-стрим метрики не несёт). Ошибки глотаются (`catch(() => {})`), старое значение остаётся.

#### 2.2 Таблица
- Пусто: «No metrics yet.» (87-93, рамка, центр).
- Шапка (97-113): «Metrics», `«{visible} of {total}»`, поле «Filter metrics» (локальный state, без URL) — фильтр подстрокой по `name|key|unit|group|description` (66-72). Нет совпадений → «No metrics match the filter.».
- Группировка (74-85): `categoryOf` = `group.trim().toLowerCase()` если есть, иначе по префиксу ключа (`db_`→Database, `cpu|memory|mem|disk|net_`→System, `stroppy_|k6_`→Stroppy, иначе Other). Порядок групп `GROUP_ORDER` = throughput, latency, errors, connections, replication, resources, system, stroppy, other; неизвестные — в конец по алфавиту. Заголовок группы: `groupLabel` (Title Case для известных) + `(N)`.
- Колонки: **Metric** (sticky left, 340px; `MetricIdentity`: name с title=description, badge key (mono, truncate 190px), badge unit, `DirectionBadge` «higher/lower is better» с ArrowUp зелёной / ArrowDown primary), **Trend** (`Trend`: `last` vs `avg` — TrendingUp/Down, зелёный если направление совпадает с `higherIsBetter`, иначе warning; Minus если last или avg = 0/не число), **Last** (жирнее), **Avg**, **Min**, **Max**. Таблица `min-w-[860px]` в `overflow-x-auto`.
- Форматирование `fmt` (37-52): `0`/`NaN`/`±Inf` → **«—»** (нулевые серии выглядят как отсутствующие — намеренно, т.к. без данных сервер отдаёт нули); `bytes` → `formatBytes` (B/KB/MB/GB/TB, ×1e3); `bytes/s|B/s` → `…/s`; `%` → 2 знака + `%`; `s|ms` → 2 знака + unit; `ops/s|txn/s|q/s|req/s|iter/s|err/s` → 1 знак; иначе 2 знака с суффиксами K/M (≥1e3/1e6) + unit. `trimFixed` убирает хвостовые нули.
- **Нет**: графиков, сортировки колонок, экспорта, ссылок в Grafana (proto явно: «deep-linking into Grafana is not done for now»).

#### 2.3 Сервер (`MetricsReader.Get` + `metrics.Collector`)
- `authorizeRun` → `Metrics.Get(runID, window)`.
- `resolveKindAndWindow` (`monitoring.go:355-395`): из `RunRecord` — `dbKind` (`dbKindString`: mysql|mariadb→"mysql", ydb|ydb_managed→"ydb", cockroach, picodata, postgres, orioledb; unspecified → `spec.database.kind`; fallback "postgres"); окно = `[started_at − 30s, (finished_at ?? now) + 30s]`; без записи/store — трейлинг 1 ч до now. `overrideWindow(window)` заменяет окно, если `start<end` (паддинга **нет** для сегмента).
- Клиент `victoria.NewClient(base + "/select/0/prometheus", token)` → `GET /api/v1/query_range?query&start&end&step`. `step = inferStep`: ≤15м→5s, ≤1h→15s, ≤6h→1m, иначе 5m.
- Каталог `MetricsForDB(kind)` (`queries.go`): DB-набор + `systemMetrics()` (всегда). Плейсхолдеры: `%s` → `stroppy_run_id="<run>"`, `%p` → `stroppy_<run с _ вместо ->`.
  - postgres/orioledb: `db_tps` (sum rate xact_commit+rollback 5m, txn/s, ↑, throughput), `db_qps` (tup_fetched, rows/s, ↑), `db_connections` (pg_stat_activity_count, connections), `db_repl_lag` (max pg_replication_lag_seconds, s, replication).
  - mysql/mariadb: `db_tps` (commands commit|rollback), `db_qps` (queries), `db_connections` (threads_connected), `db_repl_lag` (seconds_behind_master).
  - ydb: `db_qps` (grpc request_count, req/s), `db_errors` (response_count status!=SUCCESS, err/s), `db_sessions` (kqp_SessionActors_Active, ↑), `db_latency_p99` (histogram_quantile ydb_table_query_execution_latency ms), `db_cpu` (%), `db_storage` (bytes).
  - picodata: `db_qps` (pico_sql_query_total), `db_errors`, `db_raft_commit` (max index), `db_tables` (count tnt_space_len).
  - cockroach: `db_qps` (sql_query_count), `db_tps` (commit+abort), `db_errors` (sql_failure_count), `db_connections` (sql_conns), `db_latency_p99` (sql_service_latency_bucket /1e6, ms), `db_storage` (livebytes).
  - system (group "system"): `cpu_usage` (100−idle, %), `memory_usage` (%), `disk_read`/`disk_write`/`net_rx`/`net_tx` (bytes/s).
  - stroppy (group "stroppy", по `%p`): `stroppy_vus` (↑), `stroppy_ops` (rate iterations 30s, iter/s, ↑), `stroppy_iter_p99` (ms), `stroppy_query_rate` (q/s, ↑), `stroppy_latency_p99` (ms), `stroppy_errors` (sum run_query_error_rate).
  - `description` нигде не заполняется (всегда «»).
- Агрегация `summarize` (`collector.go:63-107`): все значения всех серий в один массив (NaN/Inf отбрасываются); `last` = последний сэмпл **до сортировки** (последняя серия, не обязательно самая поздняя по времени при нескольких сериях); сортировка, обрезка **p5–p95** (lo=n·5/100, hi=n−lo; если hi≤lo — без обрезки) → `min/max/avg` по обрезанному. Пустой результат → все нули (UI покажет «—» и Minus в тренде).
- Ошибка любого одного запроса → вся RPC падает (`fmt.Errorf("metrics: query %q…")`), UI молча остаётся с прошлыми данными. Без `MONITORING_URL` → `RunMetrics{run_id}` без метрик → «No metrics yet.».
- Запросы выполняются **последовательно** (~16-18 query_range на вызов, ×каждые 12 с на живом run).

---

### 3. Вкладка GRAFANA (`GrafanaPanel.tsx` + `services/grafana.ts`)

#### 3.1 Ленивый монтаж и кэш
- `RunDetail.tsx:384-388`: `grafanaSeen` становится true при первом `view==="grafana"`; панель монтируется в абсолютный слой (`absolute inset-0`) и далее **скрывается CSS** (`hidden`), не размонтируется → повторное открытие мгновенно; никогда не префетчится на загрузке run (каждый iframe грузит полное Grafana-приложение). Комментарий в панели: пре-варм всего произведения дашборды×машины давал сотни запросов — «never do that».

#### 3.2 Список дашбордов по dbKind
- `DASHBOARD_UID` (`grafana.ts:18-33`): `workload→stroppy-metrics-v1`, `system→stroppy-system`, `postgres→stroppy-postgres`, `mysql→stroppy-mysql`, `mariadb→stroppy-mysql` (`8e88804f`), `cockroach→stroppy-cockroach` (`ee743150`), `ydb→stroppy-ydb`, `picodata→stroppy-picodata`, `orioledb→stroppy-postgres` (`1737a409`). Labels `DASHBOARD_LABEL` (Workload, System, PostgreSQL, MySQL, MariaDB, CockroachDB, YDB, Picodata, OrioleDB).
- `dashboardsForRun(dbKind)` = `["workload","system"] + dbKind` (если есть UID). Значение `run.dbKind` — строка из `dbKindLabelFromJson`-маппинга RunVM (нижний регистр kind). Неизвестный kind (напр. `ydb_managed`?) → только Workload+System.
- Переключатель `SegmentedControl variant="tabs"` (98-105), `selected` default = первый (workload); state локальный.
- Кнопка «Open» (107-115): `<a target=_blank>` на тот же embed URL (kiosk).
- Не встроены, но провижинятся: `stroppy-compare` (Compare page), `stroppy-server` (сервер).

#### 3.3 Пилюли машин (только System)
- `machineActive = selected==="system"`; `targets = workers.map({id: machineId||host||id, role: kind}).filter(id)` (52-55). UI (119-162): «Machines» + «all» (title «All machines (auto)») + по кнопке на worker: бейдж role (uppercase 9px) + id (`…` + последние 24 символа, если длиннее 24; title `role: id`). Выбор — `machine` state ("" = All), не в URL.
- Ключ iframe `keyFor(name, m)` = `name|` + (`m` только для system) → у не-system дашбордов один iframe независимо от машины.

#### 3.4 Построение URL (`buildEmbedUrl`, `grafana.ts:70-108`)
- База `GRAFANA_BASE` = `VITE_GRAFANA_URL` без хвостового `/` или **`/grafana`** (same-origin через gateway). URL = `${base}/d/${uid}?${pairs}`; пары собираются **вручную** (не URLSearchParams), чтобы `kiosk` был **голым флагом** (`kiosk=` со значением Grafana не считает kiosk-режимом).
- Переменные: `workload` → `var-prefix=stroppy_<runId с _>_` (`runPrefix`, с завершающим `_`; `0d9cf009` «Fix Grafana workload metric prefix»); остальные → `var-run_id=<runId>`; `system` дополнительно `var-node=<machine || "All">`.
- Время: если `startedAt` валиден (`validTs`: не zero-sentinel, год ≥2000): `from = started − 30 000 мс`; если `finishedAt` валиден → `to = finished + 60 000`; иначе `to=now&refresh=5s`. Без `startedAt` — параметров времени нет (дашборд по своему дефолту `now-…`).
- Всегда `theme=dark`, `kiosk`.
- В URL **нет** `orgId` (аноним попадает в org `public` автоматически) и нет `var-job`/`var-Instance`/`var-Interval` (дефолты дашборда).

#### 3.5 Scope-cookie (`cf16c9c0`, 2026-07-28)
- **Зачем.** После hardening (`fe93de8a`) аноним Grafana живёт в org `public`, единственный datasource которой — gateway `/public/metrics` (share-scoped). Iframe run-detail анонимен для Grafana (без bearer) → попадал туда без cookie → 404, все панели «No data». Решение — выдать залогиненному тот же scoped-путь, авторизованный per-run.
- Клиент: `ensureGrafanaSession(tenant, run)` (`run_overview.ts:530-535`): RPC `GrafanaSession{tenant_id, run_id}` → `scope_token` → `document.cookie = "stroppy_share=<token>; path=/; max-age=43200; samesite=lax"` (не HttpOnly — ставится клиентом; даёт только чтение уже доступного run). В `GrafanaPanel` (40-50): `sessionReady` false → iframes **не монтируются**, пока RPC не завершится (ошибка глотается — панели покажут ошибку Grafana).
- Сервер (`service.go` GrafanaSession): `authorizeRun` → `GrafanaScopeSigner(runID)` (= `gateway.SignRunScope(cfg.JWTSecret, runID)`; nil → Unimplemented «grafana metrics scoping is not configured», пусто → Internal). Токен `run.<runID>.<base64url(HMAC-SHA256(secret, runID))>` (`runscope.go`); runID с `.` не подписывается.
- Gateway `/public/metrics/*` (`publicmetrics.go`): cookie `stroppy_share` = либо run-scope токен (проверка HMAC, **без** ограничения окна — пользователь свободно двигает time range), либо share-токен (`ShareResolver`, окно клампится). Нет/невалидно → 404 (неотличимо). Далее: allowlist путей (`api/v1/query`, `query_range`, `series`, `labels`, `label/<n>/values`, `status/buildinfo`); удаление клиентских `extra_label`/`extra_filters[]` + принудительный `extra_label=stroppy_run_id=<run>` в query и в form-body POST (VictoriaMetrics OR-ит `extra_filters[]` — поэтому строго стрип); `Cookie`/`Authorization` не утекают, ставится backend bearer; upstream `STROPPY_METRICS_QUERY_BACKEND` (`http://vmselect:8481/select/multitenant/prometheus`).
- Grafana (`docker-compose.yaml:73-107`): порт не публикуется; `GF_AUTH_ANONYMOUS_ENABLED=true`, `ORG_NAME=public`, `ORG_ROLE=Viewer`, Explore off, внешние снапшоты off, `ALLOW_EMBEDDING=true`, `COOKIE_SAMESITE=lax`, `GF_SERVER_ROOT_URL=…/grafana/`, `SERVE_FROM_SUB_PATH=true`. `grafana-init` (`init.sh`) создаёт org `public`, datasource `VictoriaMetrics` (uid `victoriametrics-public`, url `PUBLIC_METRICS_URL`, `jsonData.keepCookies: ["stroppy_share"]`), папку и заливает те же JSON-дашборды (по имени datasource, имена per-org). Main Org (orgId 1) — файловый provisioning, datasource `victoriametrics` → vmselect напрямую (все тенанты; только для авторизованных админов).
- Gateway `/grafana` и `/grafana/*` → `newMonitorProxy(GrafanaBackend)` (reverse proxy без bearer, `gateway.go:216-222`). Caddy намеренно **не** проксирует `/grafana` напрямую (`Caddyfile:40-43`) — единственная точка входа gateway. Dev: `vite.config.ts` проксирует `/grafana` на `:8080`.

#### 3.6 Жизненный цикл iframe
- `srcsRef: Map<key, url>` (59); при смене `activeKey` и `sessionReady` — если ключа нет, строится URL и добавляется (71-79): монтируется **только просмотренное**, просмотренные остаются (hidden) → переключение = CSS toggle, без перезагрузки Grafana (`ebce8c8a` «fix iframe reload always»).
- При смене `startedAt/finishedAt` (окно run или выбранный сегмент) — **все** URL пересобираются, `loaded` сбрасывается → все iframe перезагружаются (83-93).
- Рендер (166-183): контейнер `relative flex-1 minHeight: calc(100vh − 16rem)`; каждый iframe `absolute inset-0`, `block|hidden` по activeKey, `sandbox="allow-scripts allow-same-origin allow-popups allow-forms"`, `onLoad` → в Set `loaded`. Пока активный не загрузился — оверлей «Loading dashboard…» со спиннером (`bg-background/70`).
- **Крайние случаи.** Run завершается во время просмотра → все дашборды перезагрузятся с locked окном (задумано: «locked from..to is part of the URL»). Пилюля машины при переключении на другую машину создаёт новый iframe (ещё одно приложение Grafana). Смена вкладки Grafana→другая не выгружает iframes (продолжают `refresh=5s` в фоне на живом run). Cookie living 12 ч; при истечении в открытой вкладке панели начнут 404-ить — повторный RPC только при remount (смена tenant/run).

#### 3.7 Дашборды (переменные, на которые опирается embed)
| UID | title | templating (hide=2 = скрыт) | Дефолт time/refresh |
|---|---|---|---|
| `stroppy-metrics-v1` (workload) | Stroppy — DB Stress Testing | `datasource` (type datasource, prometheus), **`prefix`** textbox (default `stroppy_k6_tpcc_`), `job` = `label_values({__name__=~"${prefix}.*"}, stroppy_machine_id)` all/multi, `scenario`, `step`, `tx_name`, `tx_action`, `tx_isolation`, `query_name`, `query_type`, `table_name` (all/multi) | now-30m, 10s |
| `stroppy-system` | Node Exporter Full (rfmoz 1860) | **`run_id`** textbox hide=2, `job` = label_values(node_uname_info, job), `nodename`, **`node`** = `label_values(node_uname_info{job="$job",stroppy_run_id="$run_id"}, stroppy_machine_id)` includeAll/multi, allValue `.+` | now-24h, 1m |
| `stroppy-postgres` | PostgreSQL (stroppy) | `run_id` hide=2, `Instance` = label_values(pg_up{run}, stroppy_machine_id) all/multi, `Database` (regex исключает template*/postgres), `Interval` (default 10m) | now-24h |
| `stroppy-mysql` | Stroppy / MySQL | `run_id`, `Instance` (mysql_up), `Interval` 10m | now-1h, 10s |
| `stroppy-cockroach` | Stroppy / CockroachDB | `run_id`, `Instance` (sql_query_count), `Interval` 1m | now-3h |
| `stroppy-ydb` | Stroppy / YDB | `run_id`, `Instance`, … | — |
| `stroppy-picodata` | Stroppy / Picodata | `run_id`, `Instance` (pico_instance_state), `Interval` | now-15m, 10s |

- Все панельные выражения фильтруют `stroppy_run_id="$run_id"` и `stroppy_machine_id=~"$node|$Instance"`; workload — по имени метрики `${prefix}tx_count` и т.п. (пример: `sum(increase(${prefix}tx_count{scenario=~"$scenario",…}[$__range])) / $__range_s`). Метки `stroppy_tenant_id/stroppy_run_id/stroppy_machine_id` навешивает gateway на ingest (`monitorlabels.go`), а gateway на чтение дополнительно навязывает `extra_label=stroppy_run_id`.
- Панели postgres/mysql/cockroach — legacy `graph`/`singlestat` (Grafana 12 auто-мигрирует). Панели workload/system/picodata — `timeseries/stat/bargauge/table`.
- Крайний случай: `datasource`-переменная workload-дашборда имеет `current.value="victoriametrics"` (uid Main Org); в org `public` такого uid нет (`victoriametrics-public`) — Grafana подставляет первый prometheus datasource, работает, но при появлении второго datasource в org сломается.

---

### 4. Сводка API (для нового контракта)

| RPC (`TestRunOverviewService`) | Request | Response | UI-вызов |
|---|---|---|---|
| `QueryLogs` | tenant_id, run_id, `LogFilter`, `from: LogCursor?`, `direction OLDER/NEWER`, `limit ≤5000` | `lines[] LogLine`, `older`, `newer` | load (older, 300 [+newer 150 при якоре]), loadOlder, loadNewer, jumpTop (newer), jumpBottom (older) |
| `StreamLogs` (server-stream) | tenant_id, run_id, `LogFilter`, `from?` | поток `LogLine` | live tail, `from=newerCursor` |
| `GetLogFacets` | tenant_id, run_id, `LogFilter` | `fields[]{field, values[]{value,count}}` | на каждый filterKey |
| `ResolveLogRef` | tenant_id, `LogRef{run_id, node_execution_id?, component_id?, start?, end?, cursor?}` | run_id, `LogFilter`, `LogCursor` | не используется UI |
| `GetRunMetrics` | tenant_id, run_id, `window: TimeRange?` | `RunMetrics{run_id, range, metrics[] MetricSummary}` | initial + сегмент + 12 с polling |
| `GrafanaSession` | tenant_id, run_id | `run_id, scope_token` | перед первым iframe |

`LogLine` поля: observed_at, run_id, line_no, node_execution_id, parent_node_execution_id, phase, stage_name, component_id, machine_id, step_id, action, mentions[], unit, source (enum), stream (enum), line, cursor{observed_at, seq}.
`MetricSummary`: key, name, unit, avg, min, max, last, higher_is_better, description, group.

---

### 5. Известные слабости текущей реализации (учесть при переписывании)
1. Логи без виртуализации → cap 5000 строк, пауза стрима при открытом пикере, flush 150 мс — всё это костыли под отсутствие виртуализации.
2. Курсор пейджинга — только `_time` (включительно) без `seq` в запросе → дубли на стыках и потенциальный цикл при >limit строк в один timestamp; `line_no` не пишется ни одним продюсером, `seq` = индекс внутри страницы → `cursorKey` не стабилен между разными страницами/клиентами, хотя задуман как стабильный.
3. Live tail = серверный polling 1 с; тихая смерть стрима при ошибке (UI не узнаёт).
4. Facets — 8 последовательных HTTP-запросов к VL на каждое изменение фильтра; `mentions` не фасетится (CSV-хранение).
5. Метрики: ~17 последовательных query_range каждые 12 с на живом run; ошибка одной метрики валит все; `last` — последний сэмпл последней серии; нули маскируются «—»; `description` пустой.
6. Grafana: каждый (dashboard×machine) — отдельный полноценный Grafana-app в iframe; при смене окна перегружаются все; cookie 12 ч без обновления; `segmentSel`/`selected`/`machine`/`wrap` не в URL (не шарятся), в отличие от лог-фильтров.
7. Journald/file-логи не несут component_id/phase/node_execution_id → фильтр Step/Component их отсекает; Source/Stream count-ы считаются по буферу, а не по всему run.

---

# 5. Сводка: что сломано или отсутствует (по всем зонам)

Консолидировано из разделов 1.«Сводка», 2.«Чек-лист», 3.«Крайние случаи», 4.«Слабости». Это то, что при реимплементации либо надо чинить, либо осознанно повторять.

## 5.1 Мёртвое / неподключённое (сервер есть, UI нет)
- **Shell** — кнопка `/shell?runId=` ведёт в `RootRedirect`, роута и страницы нет; `AgentShellService.OpenShell` (bidi-stream, audit, `RESOURCE_AGENT_SHELL/CREATE`) полностью реализован на сервере.
- **Share management** — `ListShares` / `RevokeShare` / `SetShareExpiry` есть в `services/shares.ts` и на сервере, ни одна страница их не использует; TTL всегда дефолт 7 дней; обещанный фоновый refresh снапшота не существует.
- **Compare** из RunDetail открывается с одним id → мгновенная ошибка «At least two run IDs are required», пикера второго рана нет. Сервер сравнивает только summary + N-way metric diff с dead-band 1 %; spec-diff нет.
- **`LogRef`** в `PipelineNode` приходит по проводу, UI его не читает — связка pipeline↔logs только через `nodeExecutionId`.

## 5.2 Баги отображения
- **DATABASE card** (`RunConfigTab.tsx:161 extractDbParams`) берёт первое object-поле `DatabaseParams` → всегда `package`, реальные engine-параметры (replicas/patroni/`master_options`) никогда не рендерятся; `version` теряется.
- **Segment card**: `no_steps` (repeated string) типизирован как boolean → любой блоклист = «Steps: (all)»; `files[]` — объекты `{name,kind,content}` → `[object Object]`; `sql` — позиционный аргумент (имя файла), не inline SQL.
- **Agents timeline**: `kind` — сырое имя enum; `detail` сервером не заполняется; `WORKER_ONLINE/OFFLINE` не генерируются (только `STAGE_*` + один `RUN_STATUS`).
- **Delete** говорит «permanently removed», а делает soft-delete (`deleted_at`); метрики/логи/шары не удаляются.
- **Rerun** уводит на `/runs`, не показывая новый id; имя нового рана `stroppy <ver>`, не имя исходного.
- **Terminal run** всегда с бейджем `PERSISTED` — это норма (сервер не ходит в Temporal для завершённых), не деградация; UI это не объясняет.
- Гейтинг кнопок в RunDetail — скрытие, в `Runs.tsx` — disabled-with-reason; несогласованно.

## 5.3 Производительность / надёжность
- Логи без виртуализации → cap 5000 строк, пауза стрима при открытом date-picker, flush 150 мс.
- Курсор пейджинга логов — только `_time` (включительно), без `seq` в запросе → дубли на стыках и возможный цикл при >limit строк в одну метку; `line_no` не пишет ни один продюсер → `?line=` deep-link нестабилен между страницами.
- Live tail = серверный polling VictoriaLogs 1 с; при ошибке стрим умирает молча.
- Facets — 8 последовательных запросов `field_values` на каждое изменение фильтра; `mentions` не фасетится (CSV).
- Метрики — ~17 последовательных `query_range` каждые 12 с на живом ране; ошибка одной валит все; `last` = последний сэмпл последней серии; нули маскируются «—».
- Grafana — каждый (dashboard × machine) = отдельный полный Grafana-app в iframe; при смене окна перегружаются все; scope-cookie 12 ч без продления.
- Persisted fallback overview отстаёт до 20 с (`runtimeProjectionPersistMinInterval`; история: `5b116d14` 2 с → `188eb2e1` 20 с из-за bloat истории Temporal) — компенсируется live-запросом + `degraded_reasons`.
- Journald/file-логи не несут `component_id/phase/node_execution_id` → фильтры Step/Component их отсекают.

## 5.4 Состояние не в URL (не шарится ссылкой)
`segmentSel` (window), выбранный дашборд/машина/wrap в Grafana, выбранный узел в Pipeline, ширина сайдбара (localStorage). В отличие от `?view` и всех фильтров логов.

## 5.5 Что обязательно сохранить (must-keep)
1. Один snapshot-контракт для unary и stream; клиент заменяет state целиком; стрим только до terminal, fallback poll 5 с; Refresh = unary.
2. Тройной источник overview (`temporal` → persisted `runtime_state` → `synthetic`) с явными `source`, `observed_at`, `degraded_reasons`; UI показывает не-live источник бейджем и причины баннером.
3. Terminal run никогда не ходит в Temporal.
4. `progress` = доля terminal-стадий включая вложенные; duration — сервер отдаёт span, клиент тикает для running.
5. Presence агентов: `≤2×heartbeat` online, `≤6×` stale, иначе offline; terminal → `terminated`.
6. Pipeline: band на каждый `pipeline.roots[]` (5 фаз из `internal/workflows/test.go:23-27`), `nodeExecutionId` формата `stage/<n>`, `component/<id>`, `component/<id>/step/<stepId>`; jump «View in Logs» сбрасывает все прочие фильтры.
7. Logs: все 12 URL-параметров фильтров как единственный источник правды; LogsQL `run_id:"…"` + OR-группы + `_msg:"q"`; три пути ингеста (agent stdout/stderr, vector journald/file, server terraform/docker).
8. Metrics/Grafana window: `workloadSegments` = листья `phase="workload"`; окно отправляется только при полном `[start,end]`; Grafana `from=started−30s`, `to=finished+60s | now&refresh=5s`; per-dbKind UID map (mariadb→mysql, orioledb→postgres); scope-cookie `stroppy_share` через `GrafanaSession` + gateway `/public/metrics` с принудительным `extra_label=stroppy_run_id`.
9. Share: 256-bit токен, allowlist-проекция снапшота (summary + metrics + segments + typed DB settings + Yandex VM sizing), публичная страница = lab-report с Overview/Metrics/Grafana.
10. Quotas: ledger RESERVED (TTL 30 мин) → ALLOCATED (после provision) → RELEASED (teardown); EXPIRED лениво.
11. Cancel: запись → CANCELLING + Temporal `CancelWorkflow`; teardown через `defer` на отвязанном контексте, квоты/сети освобождаются; если workflow уже нет → сразу CANCELLED.
12. Sidebar-строки скрываются при пустом значении; RAW JSON = полный `spec`; ширина сайдбара 220–560, дефолт 300, в localStorage.
