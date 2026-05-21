# Stroppy Cloud — BDD / живая документация

Это корень поведенческой спецификации продукта. Сценарии (`*.feature`) описывают
**что** делает продукт на языке предметной области; код степов в `internal/bdd`
проверяет это против нового runtime/proto.

> **Канон глоссария = doc-comments в `.proto`.** Здесь — навигация и продуктовый
> обзор. Точные определения сущностей живут в комментариях proto-сообщений и
> являются единственным источником правды. Если термин уточняется — правим
> комментарий в proto, а не дублируем определение здесь.

## Цель продукта

**Stroppy Cloud — оркестратор бенчмаркинга БД.** Один бинарь разворачивает БД,
собирает HA-топологии, гоняет [Stroppy](https://github.com/stroppy-io/stroppy)-бенчмарки
и собирает метрики. Поддержка локального Docker и Yandex Cloud VMs.

### Архитектурные принципы (новый подход)

1. **Intent, не ops-план.** Пользователь описывает *желаемое* (движок, версия,
   workload, топология), а не шаги. Планировщик разворачивает интент в `runtime.ops`
   и узлы `Dag`. Источник: `domain/database.proto` («Database is an intent, not an ops plan»).
2. **Всё исполняемое = `Dag`.** Runtime не знает о домене (suite/test/terraform/agent).
   Домен компилирует сущности в `Dag`; runtime крутит узлы, рёбра, retry, cancel,
   propagation отказа и снапшоты. Источник: `runtime/primitive/dag.proto`.
3. **Агенты только poll.** Server не пушит команды — агент сам тянет их
   (`api/agent`: Register / Heartbeat / **Poll** / Report). Прод-требование.
4. **Мультиарендность.** Изоляция по `Tenant`.

## Хребет продукта

```
1. Define   юзер строит Database + Workload интенты → TestPreset → SuitePreset   [каталог]
2. Render   backend рендерит интент → RenderedConfig preview
3. Plan     планировщик: финальный интент + топология → runtime.ops + узлы Dag
            ───────────────────────────────────────────────────────────────────
            Define↔Render↔Plan — интерактивный цикл (WIZARD, human-in-the-loop):
            юзер вклинивается на любом шаге, шлёт Override-патчи, ре-рендерит,
            правит план. Wizard остаётся и расширяется.
            ───────────────────────────────────────────────────────────────────
4. Execute  DagProcessor крутит Dag; агенты poll'ят Command, исполняют, репортят
            (точка невозврата — дальше работают агенты)
5. Collect  метрики → VictoriaMetrics; сравнение запусков; стоимость
```

## Карта сущностей (→ канон в proto)

| Сущность | Роль | Канон |
|----------|------|-------|
| `Account` | пользователь | `models/account.proto` |
| `Tenant` / `TenantMember` | изолированное пространство + членство | `models/tenant.proto` |
| `Preset{DATABASE\|WORKLOAD\|TEST}` | переиспользуемое определение | `models/preset.proto` |
| `domain.Database` | интент БД (Kind, версия, конфиг) | `domain/database.proto` |
| `domain.Workload` | интент нагрузки (Protocol → stroppy) | `domain/workload.proto` |
| `domain.Topology` | компоненты + форма (single/ha/replica/scale) | `domain/topology.proto` |
| `TestPreset` | Database + Workload + Topology + DeploymentIntent | `domain/test.proto` |
| `SuitePreset` | Provider + Scheduling + [TestPreset…] | `domain/suite.proto` |
| `TestRun` | инстанс TestPreset, владеет `Dag` | `models/testing.proto` |
| `Suite` / `SuiteRun` | оркестрация набора TestRun | `models/testing.proto` |
| `Dag` | снимок исполнения (примитив runtime) | `runtime/primitive/dag.proto` |
| `Agent` | воркер на машине, poll-based | `models/agent.proto`, `api/agent/agent.proto` |
| `DeploymentIntent` / `Provider` | docker \| yandex | `deployment/*.proto` |

## Структура каталога

Фичи группируются **по продуктовой способности**, не по пакету/файлу:

```
features/
  engine/         новый runtime: scheduling, join-policy, retry, cancellation,
                  failure-propagation, sub-dag-ref, recovery
  agent/          lifecycle (poll-only), command-execution
  orchestration/  suite-run, test-run, run-state-machine
  catalog/        database-preset, workload-preset, topology, render-override
  provisioning/   terraform apply/destroy, cost
  tenancy/        auth, tenant-isolation
  results/        metrics-collect, compare
```

## Теги

| Тег | Смысл |
|-----|-------|
| `@engine` `@agent` `@orchestration` `@catalog` `@provisioning` `@tenancy` `@results` | область |
| `@migration` | поведение, восстановленное из старого кода → новый код обязан повторить (acceptance-гейт рефактора) |
| `@wip` | в работе, исключается из «зелёного» прогона |
| `@e2e` | требует реальной инфры (PG/MySQL/…), исключается из быстрого лейна |

## Уровни тестов (пирамида)

| Уровень | Покрывает | Форма |
|---------|-----------|-------|
| Acceptance / BDD | продуктовое поведение, cross-cutting | Gherkin → godog внутри `go test` |
| Spec-by-example (Go) | внутренности движка, билдеры | Go Given/When/Then + DSL |
| Unit (как есть) | retry-математика, cycle detect, validate, JWT | существующие table-тесты |

## Шаблон разбора механики

Для каждой механики старого кода фиксируем:

1. **Что делает** — поведение из старого кода и его тестов.
2. **Язык / сущности** — какие proto-сообщения (канон). Уточнения → в doc-comments proto.
3. **Как ложится на новый runtime** — Dag / Operation / agent / task.
4. **Примеры → сценарии** — Gherkin, декларативные, 1 поведение = 1 scenario.
5. **Теги** — `@migration`, если новый код обязан повторить старое.
6. **Решения / открытые вопросы.**

## Открытые дыры / решения (накапливаются по ходу)

Структурные находки и принятые архитектурные решения. Дыры закрываются в фазе
имплементации (правка proto + кодоген).

| # | Тип | Где | Суть |
|---|-----|-----|------|
| H1 | ✅ fixed | `database.proto` | `DatabasePreset.workload`→`database`. protoc OK |
| H2 | ✅ fixed | `database.proto` | docstring `RenderedDatabaseConfig`→`render.Config` |
| H3 | ✅ решено | `dbconfig`/render | `trust` ок для эфемерного изолированного стенда (DB без public IP; VPC+SG). Инвариант: public IP на DATABASE ⇒ парольная аутентификация, не trust (иначе валидация режет) |
| H4 | ✅ added | `render/config.proto` | `Config.Item.Binding` добавлен: token + repeated component_ids (string, runtime-pure) + attr (string, без enum). protoc OK |
| H5 | решение | backend | compat-матрицы (protocol×script) и install-рецепты пакетов = backend-данные + валидация, не proto |
| H6 | решение | `database.proto` | custom .deb = `Database.config` render `File.Ref`+`Cmd`+`Binding`; builtin = planner-данные |
| H7 | ✅ added | `topology.proto` | `Topology.external_components` (top-level, reference-only, без машины/агента) добавлен. protoc OK |
| H8 | решение | `topology.proto` | sizing на `Machine` напрямую, `MachineOverride` как сущность убрать |
| H9 | инвариант | весь render | preview == execution (кроме объявленных биндингов) |
| H10 | ✅ fixed | `testing.proto`,`api/ui/suite.proto` | `SuiteRuns`→`SuiteRun`; `Suite.List`=Suite под `suites`; `SuiteRun.List` под `suite_runs`. protoc OK |
| H11 | решение | `test.proto` | TestPreset = immutable snapshot (inline), композиция by-value, не by-reference |
| H12 | решение | `preset.proto` | `Preset.kind` + oneof оба, валидатор следит за совпадением |
| H13 | решение | `suite.proto` | SuitePreset.Scheduling → Dag.Scheduling (seq=1, parallel=N, on_node_failure passthrough) |
| H14 | решение | компилятор | on-host работа = per-command sub_dag (узел на WRITE_FILE/RUN_CMD/service) |
| H15 | решение | компилятор | нет общего State: данные через node output + render.Binding |
| H16 | решение | `dag.proto` | MustComplete → always_run teardown + рёбра от provision |
| H17 | UI-TODO | UI | cancel/teardown-поведение надо явно отрисовывать (отдельная задача) |
| H18 | решение | новый пакет `planner` | компилятор domain→primitive.Dag (размещение/имя позже) |
| H19 | решение | `api/agent` | очередь команд = узлы Dag; таблицу `agent_commands` + orphan-reaper убрать |
| H20 | решение | `runtime/agent` | Report{RUNNING} = прогресс + keepalive lease; терминальные статусы двигают узел |
| H21 | решение | `api/agent` | recovery: lease expiry → re-lease; новый boot_id → инвалидация in-flight лизов |
| H22 | решение | `status.proto` | run-состояние = Dag.status; JobState-таблицу убрать |
| H23 | решение | DagProcessor/Storage | multi-server: DB-lease на Dag (как agent-lease); reaper не нужен |
| H24 | решение | новый admission-контроллер | кросс-tenant квоты решаются перед стартом (PENDING→RUNNING), не в runtime |
| H25 | решение | recovery | always_run teardown структурно покрывает VM-leak; RecoverChecker не нужен в движке |
| H26 | ✅ added | `dag.proto` | `TaskState.ExecutionLocus` (SERVER/AGENT) добавлен; Poll выдаёт только AGENT. protoc OK |
| H27 | решение/keystone | `tf.proto` | биндинги резолвятся из `TfOperation.Output.outputs_json`/`Deployment.Output`; петля render замкнута |
| H28 | решение | provisioning | terraform/docker = server-locus, Poll их не выдаёт; bootstrap агента = cloud-init JWT |
| H29 | решение | `ops/operation` | агент = generic-исполнитель 6 ops; per-engine рецепты = planner-данные (install*/config* Go-функции удалить) |
| H30 | решение | `system/service` | systemd Service декомпозирует planner в WRITE_FILE+RUN_CMD; agent не трактует Service |
| H31 | решение | компилятор | host bootstrap = явный преамбула-узел (apt-lock+deps), retryable; ops идемпотентны |
| H32 | решение | `deployment.proto` | cloud-квота = провижининг-admission (после tenant-квоты D14); источник = облако, гибрид кэш+live |
| H33 | дыра/решение | сеть/inventory | subnet pre-allocate+lease (ips lib) до запуска, синк занятости из VPC; нужен cloud-inventory/reconcile слой |
| H34 | решение | `system/file.proto` | большие файлы (.deb/артефакты) = File.Ref(s3://); presigned URL, агент curl; не inline в Dag |
| H35 | решение(испр.) | `models/dag.proto` | лиз = **pg** (models.Dag.Processor + dags_claim_idx; models.Agent.lease_expires_at). Valkey = опц. advisory/кэш, не авторитет. (исправляет ранний «Valkey lease») |
| H39 | ✅ added | `deployment.proto`,`models/network.proto`,`api/ui/inventory.proto` | `QuotaResource`/`QuotaInventory`/`NetworkAllocation` + **`CloudInventoryService`** (ui, OWNER, tenant_id; FetchQuotas/ListNetworkAllocations/Reconcile). protoc OK. Осталось: `make protocols` |
| H51 | ✅ added | `models/{webhook,package}.proto`,`api/ui/{webhook,package}.proto`,`testing.proto` | `WebhookService`+`models.Webhook`; `PackageService`+`models.Package` (upload через presigned PUT, G2); `Suite.Cron`+`next_fire_at`. Бинари = HTTP-handler (не RPC). protoc OK |
| H52 | ✅ added | все `api/ui/*` | каждый tenant-scoped ui-запрос несёт inline `tenant_id` (валидация членство+роль, scope). Исключения: auth, ListMyTenants, публичный GetSharedRun. Bare-id RPC обёрнуты в Request{tenant_id,id} |
| H53 | ✅ added | `api/ui/{settings,inventory}.proto` | SettingsService + CloudInventoryService перенесены admin→**ui (OWNER)**; settings уже per-tenant (SettingsItem.tenant_id); разные tenant → разные провайдер-настройки; is_admin тоже проходит. admin-версии удалены |
| H54 | ✅ added | `runtime/logs/logs.proto`,`api/ui/run.proto` | унифицированная `LogLine` (command+journald+file обобщены) + `LogRef`/`LogCursor(observed_at+seq)`; RunService `QueryRunLogs/StreamTestRunLogs(unified)/BuildLogLink`; 2 оси (node_execution_id этап + component_id); логи только для run. protoc OK |
| H55 | дыра(impl) | Vector enrich / planner | дополнить метки логов `component_id` (+ `node_execution_id` для command-driven); сейчас только run_id/machine_id/role/unit |
| H56 | ✅ решено | `runtime/metrics/metrics.proto` | Grafana-дашборды ходят в VictoriaMetrics напрямую по HTTP (vmselect multitenant datasource) — **не трогаем, остаётся HTTP**; proto MetricsService = аддитивные summary для нашего UI (тот же VM). Лог/метрик-ссылки → свой вьювер, в Grafana пока не линкуем |
| H58 | depth | `engine/*.feature` | углублён движок (контрактные поведения, 6 файлов): edges-joins, retry-failure, subdag-dagref, recovery, validation, parallelism-cleanup. **Контракт `always_run` уточнён по тестам: НЕ «бежит всегда» — на success без ребра SKIPPED; force-run только на failure/cancel; teardown на success бежит через ребро** |
| H59 | depth | `orchestration/end-to-end.feature` | E2E-флоу одним сценарием (happy/failure/external) — стыкует B6→C12→D18→D16→D14→E/logs→teardown→cost |
| H63 | future | `features/future/*` | раздел «на будущее» (`@future`, НЕ `@migration`): resilience(degrade), quotas-retention, **mcp** (AI-ассистенты через API-токен), **agentic-analysis** (AI-разбор запуска + UI) |
| H65 | depth | `engine/*.feature` | precision-pass: пины `FailureCode` (DAG_INVALID/SUB_DAG_FAILED/DAG_REF_FAILED/DAG_REF_PENDING/DAG_REF_RUNNER_MISSING) + data-varying проза→Outline (retry attempts×fails→status, on_status). Не-движковые вердикты несут причину, кодов нет — оставлены. 16 Outline / 290 сценариев |
| H66 | дыра(design) | API/validation | нет типизированного каталога error-кодов для API/валидации (только рантайм-движковый `FailureCode`) — рассмотреть enum ошибок API позже |
| H64 | ✅ added | `features/tenancy/rbac.feature`, `api/ui/tenant.proto`, `api/admin/tenant.proto` | **полная RBAC-матрица** (concrete): min-роль на каждый RPC всех сервисов; read=VIEWER, operate=ADMIN, own-admin(settings-write/inventory-reconcile/tokens/members)=OWNER; settings/inventory read=ADMIN (creds masked); share=VIEWER, webhooks=ADMIN; is_admin кросс-тенант; api-token cap≤ADMIN; GetSharedRun public. **Member-management перенесён admin→ui TenantService (OWNER)** + role в AddMember. protoc OK |
| H62 | depth | `cli/headless` + 4 заглушки | CLI расписан под реализацию (auth/api-token, run/wait/validate/dry-run/compare/probe/logs/upload/packages, флаги, exit-коды, JSON-out, ошибки); edge для probe/artifacts(S3)/ydb-disk/share |
| H61 | depth | inline в 6 фич | error/edge кейсы (~22): render override/validation/unsupported; agent lease-гонка/dup-report/expired-foreign/boot_id; tf apply-fail+rollback/crash-recover; subnet-exhaustion/orphan-reconcile; token-invalid/expired/refresh-reuse/role-deny; generation-конфликт/soft-delete/FK-cascade |
| H60 | depth | `catalog/engine-matrix.feature` | матрица движок×топология (форма графа) + движок→workload-совместимость (protocol+scripts, B4); managed YDB / external спец-случаи |
| H57 | ✅ added | `models/apitoken.proto`,`api/ui/apitoken.proto` | API-токены (3-й auth-принципал, CI/SDK) — recast old `tenant_api_tokens`. `models.ApiToken` (Own, token_hash virtual+unique, hash-only, plaintext-once) + `ApiTokenService` (OWNER-only mint/revoke, tenant_id); роль cap ≤ ADMIN (validate not_in:[0,3]); auth: JWT→иначе hash-lookup. protoc OK. Замечание: старое держит refresh_tokens(pg) — новое refresh=Valkey (G3) |
| H41 | ✅ fixed | `runtime/system/net.proto`, `ip.proto` | убран неисп. `duration` (и `validate` из net.proto). protoc OK без warning |
| H42 | решение | `common.proto` | generic `CommonQuery.Filter` (string-values) не используем → per-model типизированные query; пересмотреть api-запросы, что встраивают CommonQuery (ListAgentsRequest) |
| H43 | решение | persistence | схема производна от proto (ratel diff); аггрегат JSONB + скаляр-mirror индексируемых полей; рукописный postgres-storage уходит |
| H44 | дыра(испр.) | `tenant.proto` | `TenantMember.table_name` был `"tenants"` (коллизия) → исправлено на `tenant_members` |
| H45 | решение | `account.proto`,`auth.proto` | добавлен `Account.is_admin` (platform-admin); RBAC=per-tenant TenantMember.Role; refresh/session в Valkey TTL; agent=машинный principal |
| H46 | решение | `runtime/metrics/metrics.proto` | свои summary-DTO (RunMetrics/…), НЕ OTLP; **в runtime/ (не models/) — не хранятся в БД; run id = string (runtime независим от models, как DagRef)**; MetricDef=backend; live+snapshot. Самоописываемые (name/unit/higher_is_better/description/group=данные; нет enum смыслов; фронт generic) |
| H47 | решение | E/D19 | **Cost — не отдельная сущность**: = `deployment.QuotaRequest` (Σ ресурсов). C8 схлопнут в D19. OTLP-dep тащить только под raw-ingest gateway (нет) |
| H48 | решение | `database.proto`,`topology.proto` | `auto_size_pdisks` УБРАН (не флаг состояния). Диск-ёмкость = size-intent на `Topology.Machine` (провайдер-агностично), дефолтит ВИЗАРД из workload scale; провайдер-тип/io-m3 = deployment. Принцип: дефолты = поведение визарда, не флаги в сущности |
| H49 | ✅ added | `topology.proto` | `Machine.disk_gb` + `data_disks_gb` (провайдер-агностичная ёмкость) добавлены; тип/округление в deployment. protoc OK |
| H50 | баг(испр.) | `ops/operation.proto` | `install*/config*` в комментарии содержал `*/` → закрывал коммент досрочно, ломал парс/кодоген всего дерева. Исправлено |
| H51 | дыра(PROPOSED) | `api/ui` | остаются сервисы: WebhookService, PackageService(+upload), cron-поля на `models.Suite`, HTTP-раздача agent/stroppy бинарей (G6) |
| H40 | находка | `models/dag.proto` | персистентность Dag = JSONB payload + скалярные mirror-колонки + ratel-таблица `dags` + Admission{kind,labels} (G1 паттерн) |
| H36 | решение | probe/preview | probe/preview = server-locus инструменты wizard (Render), не узлы run-Dag |
| H37 | решение | share | share = immutable snapshot запуска+метрик (как B6), токен, read-only без auth |
| H38 | решение | CLI | CLI = тонкий клиент proto-API; dry-run=compile(C12), validate=сборка(B6), без своей логики |

## Бэклог механик (из старого кода)

- [x] **Фундамент** — цели + сущности + хребет (этот файл)
- [x] A — Auth + Tenancy → `features/tenancy/{auth,isolation}.feature` + заметка `auth.proto`. Прото-правки: `Account.is_admin`, фикс `TenantMember.table_name`→`tenant_members`. Решения: access+refresh (refresh Valkey TTL); RBAC per-tenant `TenantMember.Role`; platform-admin=is_admin; изоляция=Own.tenant_id+членство; agent=машинный principal. protoc OK
- [x] B3 — DB-пресеты + `dbconfig` → `features/catalog/database-render.feature`; решение по render.Binding + инвариант preview==execution (заметки в `render/config.proto`); дыры именования (`database.proto`)
- [x] B4 — Workload-пресеты (workload-only: protocol/url, script, k6, params, files) → `features/catalog/workload-render.feature` + заметки `workload.proto`. Пакеты re-filed в Database/провижининг (заметка в `database.proto`); compat-матрицы = backend-данные + валидация
- [x] B5 — Топология → `features/catalog/topology.feature` + заметки `topology.proto`/`deployment.proto`. Решения: провайдер-агностичный граф; формы эмерджентны; sizing на Machine напрямую (нет MachineOverride); placement→deployment; AGENT-per-machine; биндинги резолвятся из Deployment.Output. ДЫРА: external DATABASE-компонент без машины (нет top-level component list)
- [x] B6 — Suite / Test пресеты → `features/catalog/test-suite-preset.feature` + заметки `test/suite/preset/testing.proto`. Решения: TestPreset = immutable snapshot (inline); валидация на сборке (script×proto×kind, stroppy.version-min, YDB-relational); kind+oneof оба+валидация; SuitePreset.Scheduling→Dag.Scheduling. **Каталог (B) закрыт.**
- [x] C7 — effective_config → впитано в B5 (sizing на Machine) + B3 (override)
- [ ] C8 — cost (оценка стоимости) — открыто
- [x] C9 — pdisk_size (YDB autosize) → `features/catalog/ydb-disk-sizing.feature` + заметка `topology.proto`. Решение: `auto_size_pdisks` УБРАН из `Database.Options.Ydb` (поле удалено); диск-ёмкость = провайдер-агностичный size-intent на `Topology.Machine`, дефолтит ВИЗАРД из workload scale (не флаг); провайдер-тип/io-m3 округление = deployment (D18); эвристика=backend. **C-серия закрыта.**
- [x] C10 — rendered_configs → впитано в B3/B4 (render)
- [x] C11 — validate → впитано в B6 (сборка TestPreset)
- [x] C12 — builder.Build → DAG → `features/orchestration/compile-to-dag.feature` + заметка `dag.proto`. Решения: форма эмерджентна; on-host = per-command sub_dag; Deps→edges, AlwaysRun→always_run; нет общего State (output+binding); MustComplete→always_run teardown+рёбра (UI TODO)
- [x] C13 — рецепты компонентов (task_* / setup_*) → покрыто D17 (рецепты = planner-данные → ops)
- [x] D14+D15 — state-машина + scheduler → `features/engine/lifecycle-scheduling.feature` + заметка `status.proto`. Решения: run-состояние = Dag.status (нет job-таблицы); multi-server = DB-lease на Dag; квоты = admission перед стартом (runtime domain-agnostic); recovery = always_run teardown структурно
- [x] D16 — жизненный цикл агента → `features/agent/lifecycle.feature` + заметки `api/agent` и `runtime/agent`. Решения: полный pull (очередь=узлы Dag, нет таблицы); lease via NodeAddress; Report{RUNNING} продляет lease; recovery = lease expiry + boot_id инвалидация; сервер не блокирует
- [x] D17 — команды + setup-рецепты → `features/agent/command-execution.feature` + заметки `ops/operation`, `system/service`. Решения: агент = generic-исполнитель 6 ops (нет per-engine кода); рецепты = planner-данные; Service декомпозирует planner; bootstrap = преамбула-узел; ops идемпотентны. **Область D закрыта.**
- [x] D18 — провижининг → `features/provisioning/provisioning.feature` + заметки `tf/deployment/dag/api-agent`. Решения: server-locus исполнение (enum-метка PROPOSED); биндинги резолвятся из TfOperation.Output (keystone, петля замкнута); crash-safe workdir_id+always_run DESTROY; bootstrap = cloud-init JWT
- [x] D19+D20 — cloud-квоты + сеть/subnet pre-allocation → `features/provisioning/cloud-quota-network.feature` + заметка `deployment.proto`. Решения: cloud-квота = провижининг-admission после tenant-квоты (D14); источник истины=облако (гибрид кэш+live reconcile); subnet pre-allocate+lease (ips lib), без hash-коллизии. **Область D закрыта.**
- [x] E19+E20+E22 — метрики/сравнение/cost → `features/results/metrics.feature` + новый `runtime/metrics/metrics.proto` (RunMetrics/MetricSummary/Comparison; **в runtime/, не models/ — не хранятся в БД; run id = string, runtime независим от models**; protoc OK). Решения: свои summary-DTO (не OTLP); MetricDef=backend; live+snapshot(share); **Cost = QuotaRequest (D19), отдельного типа нет — C8 схлопнут**. **Область E закрыта.**
- [x] E21 — обнаружение endpoint'ов/targets → впитано в D18 (Deployment.Output → render.Binding) + B5
- [x] C8 — cost → схлопнут в D19 (QuotaRequest = Σ ресурсов); отдельного Cost нет
- [x] F — Control-plane API → `features/api/control-plane.feature` + новые proto `api/ui/{run,suite}.proto`. **RunService** (lifecycle + метрики/сравнение/share свёрнуты сюда — всё про run; GetSharedRun = единственный публичный RPC) + **SuiteService**, protoc OK. Решения: запуск=submit snapshot→Dag (статус=Dag.status); live=connect-go server-stream LogLine; per-model query (H42). **Область F закрыта** (дыры: Webhook/Package сервисы, cron-поля Suite, раздача бинарей-HTTP). Пофикшен `*/`-баг в `operation.proto`

### Найдено gap-scan'ом (вне исходного бэклога)
- [x] G1 — Модель персистентности → `features/infra/persistence.feature` + заметка `common.proto`. Решения: схема производна от proto (ratel diff генерит миграции); аггрегат JSONB + скаляр-mirror индексируемых; per-model типизированные query (generic CommonQuery filter не используем); soft-delete deleted_at; рукописный postgres-слой уходит
- [x] G2 — Object storage (S3) → `features/infra/artifacts.feature` + заметка `system/file.proto`. File.Ref(s3://)→presigned URL, агент curl (D17)
- [x] G3 — Valkey lease → `features/infra/leasing.feature` + заметка `status.proto`. Гибрид: pg state + Valkey lease (уточняет D14/D16)
- [x] G4 — Probe / preview → `features/catalog/probe-preview.feature`. server-locus инструмент wizard до run-Dag
- [x] G5 — Share результатов → `features/results/share.feature`. immutable snapshot+метрики, токен, без auth
- [x] G6 — Раздача бинарей → `features/infra/artifacts.feature`. server отдаёт agent+stroppy бинари (D18/D17)
- [x] G7 — CLI / headless → `features/cli/headless.feature`. тонкий клиент proto-API, переиспользует домен
