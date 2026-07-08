# SP-F: Execution shape-up

Дата: 2026-07-08. Статус: дизайн утверждён (brainstorming). Ветка: `feat/yaml-dsl-pivot`.
Часть [product-vision](2026-07-08-product-vision.md), саб-проект F (дошлифовка движка, зависит от SP-B и SP-E).

---

## 1. Что и зачем

Движок **живой**: docker recipe-run отработал end-to-end на dev-стенде (compile → provision → execute(nomad job postgres placed+running) → teardown → COMPLETED). SP-F не переделывает движок — **дошлифовывает** его под продуктовую модель (product-vision §3.4 Run, §7 SP-F) и закрывает deploy-gated хвосты, обнаруженные только на живом стенде:

- креды провайдера сейчас **process-wide** (один YC-токен на весь control-plane), а модель требует **per-org**;
- логи terraform/docker-провайдера пишутся в **общий stdout/stderr процесса**, а не per-run — их некуда положить в Run (SP-E);
- `executeServiceJob` ждёт healthy alloc, но **зашито внутри одной submit-активности с фиксированным 5-минутным таймаутом**, без отдельного наблюдаемого DAG-шага и без способа перепроверить/подождать дольше;
- terraform-провайдер (yandex) ни разу не гонялся live — есть структурные проблемы (общий workdir/state на все раны), которые всплывут только на реальном стенде;
- carryover'ы (docker `Destroy` через in-memory-состояние, single-node Nomad placement) — известны, но не входили в предыдущие итерации.

Без SP-F исполнитель работает только в демо-режиме (один процесс, один арендатор, один параллельный ран). Цель SP-F — снять эти ограничения, не меняя форму движка (компилятор/DAG-интерпретатор остаются).

---

## 2. Scope

**В scope:**
- Per-org провайдер-креды: откуда берутся, где хранятся, как попадают в `provider.Deps`/`terraform.Actor` без утечки в историю Temporal.
- Per-run захват логов провижининга (terraform apply/destroy, docker exec) — источник для читателей SP-E (Run.лог-рефы).
- `executeServiceJob`/`NomadSubmitJobActivity`: явный, наблюдаемый wait-for-healthy на уровне DAG (не только зашитый в активность 5-минутный поллинг).
- Terraform-провайдер на реальном стенде: per-run state/workdir изоляция (сейчас `moduleDir` константа → коллизия `WdId` между параллельными ранами), creds из п.1, verify destroy-on-error live.
- Carryover'ы (кратко, с решением на уровне «что делать», не полная реализация): docker `byNet` in-memory Destroy, nomad multi-node placement (`AssignNomadRoles` не вызывается).

**Вне scope:**
- Изменение формы DAG-интерпретатора/компилятора (`internal/dsl/*`, `ExecuteCompiledPlanWorkflow`'ый wave-scheduler) — не трогаем, только точечные врезки (wait-шаг, per-run деривация env/writer).
- Реестр провайдеров как каталожная сущность (SP-B) — SP-F только потребляет creds, которые кладёт SP-B.
- Формат/хранение Run-сущности и UI логов (SP-E) — SP-F только производит поток логов/статусов, которые SP-E должен уметь принять.
- Полноценный multi-node Nomad оркестратор — SP-F описывает проблему и включает существующий, но неиспользуемый примитив (`AssignNomadRoles`), не проектирует новую архитектуру размещения.

---

## 3. Компоненты

### F1. Провайдер-креды на уровне org (не process-wide)

**Проблема.** `provider.Deps.Env` — единственный источник кредов для terraform (`internal/infrastructure/provider/factory.go:33-35`, комментарий: *«Env carries provider credentials... passed to every terraform apply/destroy for non-"docker" providers»*). Он строится **один раз** в `internal/app/run.go:426-443` из процессных переменных окружения (`os.Getenv("YC_TOKEN")` и т.д.) с явным комментарием-стопгэпом: *«Sourced straight from the process environment... no equivalent per-run credential path exists yet... a single control-plane-wide credential set is this wiring's v1»*. Дальше `Env` без изменений хранится в `terraformActorExec.env` (`internal/infrastructure/provider/terraform_adapter.go:25-29`) и применяется на **каждый** `Apply`/`Destroy` через `terraform.WithEnv(a.env)` (`terraform_adapter.go:42-51`). Итог: все org на всех ранах используют один и тот же YC-аккаунт — недопустимо для мультитенант-модели.

Существующий кандидат на хранилище (`identity_secrets`, `internal/infrastructure/identity/secrets.go:57-104`, namespace `provider_secret`) — **коллизия имени**: это OIDC client secrets для SSO (`ProviderSecrets`), не деплой-креды облака. Есть также `TenantSettingsRecord.yandex_settings` (`protocols/cloud/v1/deployment/yandex.proto:85-87`, `deployment.Yandex.Settings.token`) — per-**тенант** (не per-org), хранится как plain JSON в `tenant_settings_records`, **не проходит через AES-GCM seal**, и, что важнее, **новый DSL-путь `provider.Deps.Env` его вообще не читает** — используется только старым квота-резолвером (`internal/infrastructure/quotas/manager.go:219-264`, `internal/domain/settings/resolver.go:66-70`).

**Решение.** Ввести per-org credential resolver, встроенный в цепочку `RunRecipeWorkflow → ProvisionActivity/TeardownActivity`:
1. Хранилище секретов — новый namespace в `identity_secrets` (переиспользуем механизм AES-GCM seal из `SecretStore`, `internal/infrastructure/identity/secrets.go:15-24`), например `provider_deploy_cred`, ключ = `org_id + provider_ref`. Не `TenantSettingsRecord` (не seal-ится, не org-скоуп) и не текущий `provider_secret` (уже занят под OIDC). Финальное место хранения закрепляет SP-B (каталог провайдеров) — SP-F фиксирует контракт потребления.
2. **Без утечки в историю Temporal:** сегодня `providerEnv` строится в `internal/app/run.go` и передаётся в `provider.Deps` целиком на старте — это ещё безопасно (не проходит через workflow input). Но при переходе на per-run creds нельзя положить сырой секрет в `ProvisionActivityInput`/`TeardownActivityInput` (`internal/workflows/runrecipe.go:164-199`) — Temporal сериализует и пишет вход активности в историю. Вместо значения передаём **ссылку** (`org_id` + secret-key, уже есть в `ProvisionActivityInput` как часть `ProviderRef`/run-контекста), а сама активность (`internal/infrastructure/execution/recipe_activities.go:90-114, 173-200`, выполняется на control-plane worker, не в истории) резолвит секрет из `SecretStore` непосредственно перед вызовом `provider.NewProviderForRef`.
3. `provider.Deps.Env` перестаёт быть статическим полем структуры — становится функцией `EnvFn func(ctx context.Context, orgID string) (map[string]string, error)`, вызываемой внутри `ProvisionActivity`/`TeardownActivity` на каждый вызов (см. §4).

**Затрагиваемые файлы:** `internal/app/run.go:418-443` (убрать статичный `providerEnv`, завести `SecretStore`-клиент), `internal/infrastructure/provider/factory.go:33-35` (`Deps.Env` → `Deps.EnvFn`), `internal/infrastructure/provider/terraform_adapter.go:25-51` (env резолвится на вызов, не на конструирование), `internal/infrastructure/execution/recipe_activities.go:90-114,173-200` (прокинуть `orgID` в резолв), `internal/infrastructure/identity/secrets.go` (новый namespace), `internal/workflows/runrecipe.go:164-199` (`ProvisionActivityInput`/`TeardownActivityInput` несут `orgID`, не сырой секрет).

### F2. Per-run логи провижининга

**Проблема.** `terraform.Actor.stdout/stderr` задаются **один раз** при конструировании (`internal/infrastructure/terraform/actor.go:202-249`, опции `WithActorStdout`/`WithActorStderr`), а `internal/app/run.go:422` вызывает `terraform.NewActor()` **без опций** → дефолт `os.Stdout`/`os.Stderr`, то есть весь process-wide вывод. `newTerraform` (`actor.go:345-361`) делает `tf.SetStdout(a.stdout); tf.SetStderr(a.stderr)` (`actor.go:350-351`) на **каждый** apply/destroy — это один глобальный writer на весь процесс и все раны разом. Docker-адаптер логов вообще не капчурит (`internal/infrastructure/provider/docker_adapter.go` — только уровень container exec, без per-run стрима). В `internal/infrastructure/execution/recipe_activities.go` захвата логов нет вовсе — чистый passthrough к `provider.Provider`. Итог: нет источника «терраформ/докер-лог этого рана» для Run (SP-E).

**Решение.** `tf.SetStdout`/`SetStderr` уже вызываются **per-call** внутри `newTerraform` — значит, естественная точка врезки: пробросить per-run `io.Writer` через вызов, а не менять Actor-level конструктор.
1. Добавить в `terraform.WorkdirWithParams` (или отдельный `Option`) поле `stdout/stderr io.Writer`, которое `newTerraform` мёрджит с actor-default через `io.MultiWriter` (сохраняем сегодняшний глобальный stdout как fallback/дебаг-канал, добавляем per-run writer).
2. `provider.Deps` получает `LogSinkFn func(ctx context.Context, runID string) (stdout, stderr io.WriteCloser)` — `terraformProvider`/`dockerProvider` вызывают её на входе `Provision`/`Destroy`, передают writer'ы вниз в `terraform.Option`/docker exec.
3. `LogSinkFn` в проде отдаёт writer, стримящий в тот же log-backend, что и агентские логи (Vector/VictoriaMetrics Logs — см. product-vision §6 "Observability... переиспользуем"), с меткой `run_id` — тем самым Run (SP-E) получает единый лог-реф независимо от того, agent-степ это или provider-степ.
4. Docker-путь: `docker.Executor`/`dockerExecutorExec` получают тот же `io.Writer` для вывода `docker run`/`exec`, где сегодня либо нет вывода, либо он идёт в никуда.

**Затрагиваемые файлы:** `internal/infrastructure/terraform/actor.go:202-361` (per-call writer вместо/вместе с actor-level), `internal/infrastructure/provider/terraform_adapter.go`, `internal/infrastructure/provider/docker_adapter.go`, `internal/infrastructure/provider/factory.go` (`Deps.LogSinkFn`), `internal/infrastructure/execution/recipe_activities.go:90-114,173-200` (прокинуть `runID` в вызов).

### F3. `executeServiceJob` wait-for-healthy

**Уточнение по факту кода (важно для решения).** Изначальная гипотеза «submit и не ждёт» неточна: `NomadSubmitJobActivity` (`internal/agent/nomad_activities.go:72-147`) уже **поллит** alloc-статусы внутри активности (`for { ...; time.Sleep(pollInterval) }`, строки 111-146) до `WaitTimeout` (дефолт 5 мин, `defaultNomadWaitTimeout`, строка 21), с heartbeat каждый цикл (строка 116). `executeServiceJob` (`internal/workflows/dslrun.go:607-672`) блокируется на `.Get()` этой активности (строки 665-670) — то есть ран **не** завершается «completed», пока alloc не станет running/failed или не истечёт таймаут. Реальная проблема тоньше:
- health-wait **зашит внутри submit-активности** с фиксированным 5-минутным таймаутом — не настраивается per-job/per-step из `workflow.yaml`, не виден как отдельная стадия в DAG/Run per-job status (SP-E не сможет показать «ждём healthy» отдельно от «submitted»);
- `NomadJobStatusActivity` (`internal/agent/nomad_activities.go:162-189`) — one-shot фетч статуса, существует, но **не используется** отдельно от submit — нет способа перепроверить/подождать дольше 5 минут без ресабмита джобы (напр. для БД с долгой инициализацией);
- `dslpb.WaitStep{Http, Timeout}` (`protocols/cloud/v1/dsl/compiled.proto:100-104`, лоуринг в `internal/workflows/dslrun.go:568-600`) — это шаг **степс-джобы** (curl-поллинг агентом на ноде), концептуально не связан с ожиданием здоровья Nomad-сервисной джобы.

**Решение — два варианта:**

**Вариант A (минимальный, in-place).** Оставить поллинг внутри `NomadSubmitJobActivity`, но:
- сделать `WaitTimeout` параметром `NomadSubmitJobInput`, прокидываемым из `ServiceSpec`/`workflow.yaml` (а не глобальной константой);
- эмитить промежуточный `workflow.ExecuteActivity`-heartbeat/сигнал наружу (уже есть heartbeat к Temporal, строка 116 — добавить структурированный статус в heartbeat details), чтобы Run-слой (SP-E) мог показать «waiting for healthy» отдельной строкой per-job status, а не одним блокирующим шагом.

**Вариант B (декомпозиция, для длинных/переиспользуемых ожиданий).** Разделить submit и wait на два DAG-видимых шага:
- `NomadSubmitJobActivity` теряет внутренний поллинг (или получает `WaitTimeout: 0` = «не ждать»), возвращается сразу после регистрации job в Nomad;
- новый DAG job-level `WaitStep`-аналог (по образцу существующего `dslpb.WaitStep`, но для service-health, не HTTP) — из `executeServiceJob` или как отдельный `needs`-зависимый job, который в цикле дергает `NomadJobStatusActivity` (`workflow.Sleep` между попытками, таймаут — параметр степа) до healthy/failed/timeout.
- Плюс: явная видимость в DAG (Run per-job status показывает «submit» и «wait-healthy» как разные шаги), переиспользуемость (можно ждать джобу, засабмиченную раньше, без ресабмита), не блокирует Temporal activity heartbeat-таймаутами на произвольно долгий срок.
- Минус: больше связности с компилятором (нужен новый `ast`/`dslpb` step type) — выходит за пределы «точечной врезки» декларированного в §2 scope; если непринципиально — начать с варианта A, вариант B как последующая итерация.

**Рекомендация для плана SP-F:** начать с варианта A (не трогает компилятор/DAG-формат), зафиксировать вариант B как «известное développement» на случай, если live-тесты (§8) покажут, что 5 минут недостаточно для реальных БД-топологий.

**Затрагиваемые файлы:** `internal/agent/nomad_activities.go:21,72-147,162-189`, `internal/workflows/dslrun.go:607-672` (вариант A: пробросить таймаут; вариант B: новый шаг), `protocols/cloud/v1/dsl/compiled.proto` (только вариант B, новый step type), `internal/dsl/nomad/jobspec.go` (не меняется, только источник таймаута).

### F4. Terraform-провайдер на реальном стенде

**Проблема.** Docker живой, terraform (yandex) — нет. Три известных структурных риска, которые проявятся только на реальном стенде:
1. **Creds** — см. F1 (process-wide, единственный YC-аккаунт).
2. **Отсутствие per-run изоляции state/workdir.** `terraformProvider.moduleDir` фиксирован при конструировании (`internal/infrastructure/provider/terraform.go:39-41`) и становится `WdId` на каждый вызов (`terraform.NewWdId(dir)`, `terraform_adapter.go:56,69`). `deps.ModuleDir("yandex")` в `internal/app/run.go` всегда возвращает константную строку `"yandex"` (не зависящую от run/org) → каждый ран того же провайдера использует **тот же** terraform-workdir и **тот же** `terraform.tfstate` (`/tmp/stroppy-terraform/yandex` по умолчанию). `Actor.register` (`internal/infrastructure/terraform/actor.go:313-321`) возвращает `ErrWdAlreadyExists`, если тот же `WdId` уже активен — то есть **два параллельных terraform-рана сегодня физически конфликтуют**, а не просто «неаккуратно шарят state».
3. **Destroy-on-error** — частично уже есть: `Actor.ApplyTerraform` умеет `w.destroyOnApplyError` (auto-destroy при неудачном apply, `actor.go:279-284`), и `RunRecipeWorkflow` **всегда** вызывает `TeardownActivity` из deferred-функции (`internal/workflows/runrecipe.go:47-52,309+` — пропускается только если `ProvisionActivity` вообще не была вызвана). Это ещё не верифицировано **живьём** на terraform: docker-путь протестирован e2e, terraform — нет; неизвестно, ведёт ли себя `Actor.DestroyExisting`'s on-disk fallback (`actor.go:288-311`, статит `WorkdirPath()` при отсутствии in-memory записи) корректно после падения control-plane между provision и teardown.

**Решение.**
1. Кредсы — из F1.
2. `ModuleDir` перестаёт возвращать константу: `WdId` формируется из `run_id` (или `org_id + run_id`), workdir создаётся per-run (`terraform.NewWorkdirWithParams` уже принимает произвольный `WdId` — см. `terraform_adapter.go:56,69` — нужно только генерировать уникальный id вместо статичной строки `"yandex"`). Модуль (embed tf-файлы) остаётся общим — копируется/симлинкается в per-run workdir при `prepare`.
3. Провести реальный terraform apply/destroy на dev-стенде (yandex) как приёмочный тест SP-F (§8) — до этого момента terraform-путь product-vision-модели считается непроверенным, несмотря на то, что код компилируется и docker-путь работает.

**Затрагиваемые файлы:** `internal/app/run.go:426-443` (`ModuleDir` closure), `internal/infrastructure/provider/terraform.go:39-41`, `internal/infrastructure/provider/terraform_adapter.go:56,69`, `internal/infrastructure/terraform/actor.go` (без структурных изменений — генерация `WdId` происходит выше, в провайдере).

### F5. Прочие deploy-gated carryover'ы

**Nomad multi-node placement.** `internal/dsl/nomad/jobspec.go`'s `BuildJob` умеет пин task group на конкретный `stroppy_node_id` (`api.NewConstraint`, строки 45-48, 82-89, 113-120) — но docker-сайдкар поднимает **ровно один** комбинированный Nomad server+client (`internal/infrastructure/provider/docker.go:260-300`, явно документировано: *«multi-node service placement remains a terraform/cloud... concern»*), а для terraform-пути примитив назначения ролей **есть, но не вызывается**: `AssignNomadRoles` (`internal/domain/agent/nomad_roles.go:21-51`) детерминированно назначает `NomadRoleServer`/`NomadRoleClient` остальным машинам, его doc-комментарий описывает интеграцию в `RunRecipeWorkflow` (*«the caller... merges Role/ServerAddr into that machine's Bootstrap... before rendering cloud-init/docker»*), но **нигде не вызывается** — грep по `internal/workflows` даёт ноль вызовов. Решение (для отдельной итерации, не полностью в SP-F): подключить `AssignNomadRoles` в `RunRecipeWorkflow`-провижининге (после `pickGatewayNodeID`, `runrecipe.go:693-781`), смёржить `NomadRole`/`NomadServerAddr` в bootstrap каждой не-gateway машины. SP-F фиксирует, что примитив готов и не подключён — план SP-F может либо включить это как задачу, либо явно отложить.

**Docker `byNet` in-memory Destroy.** `dockerExecutorExec.byNet map[string][]string` (`internal/infrastructure/provider/docker_adapter.go:33-34`) заполняется только в рантайме (`track()` в `EnsureContainer`, строки 148-155) и теряется при рестарте control-plane-процесса. `RemoveContainers` (строки 93-127) после рестарта получает пустой список из `a.byNet[networkName]` (строка 95) → `docker.Executor.Down` (`internal/infrastructure/docker/executor.go:149-161`) не удаляет ни одного контейнера (цикл по пустому `input.GetContainers()`), ошибка `NetworkRemove` глушится (`_ = e.cli.NetworkRemove(...)`, строка 158) — **`TeardownActivity` после рестарта воркера молча оставляет утечку контейнеров, без ошибки**. Решение: `byNet` должен либо персиститься (напр. по docker label `stroppy.run_id=<id>` + `docker ps --filter` вместо in-memory карты — контейнеры и так тэгаются per-run), либо `RemoveContainers` должен фолбэчиться на label-based discovery, если `byNet[networkName]` пуст, аналогично тому, как `Actor.DestroyExisting` уже фолбэчится на on-disk state для terraform (`actor.go:296-304`).

**Затрагиваемые файлы:** `internal/domain/agent/nomad_roles.go`, `internal/workflows/runrecipe.go:693-781` (интеграция AssignNomadRoles), `internal/infrastructure/provider/docker_adapter.go:33-34,93-155`, `internal/infrastructure/docker/executor.go:149-161`.

---

## 4. Интерфейсы (изменения)

```go
// internal/infrastructure/provider/factory.go — было: Env map[string]string (статично).
type Deps struct {
    DockerExec  dockerExec
    Actor       terraformExec
    EnvFn       func(ctx context.Context, orgID string) (map[string]string, error) // F1: было Env map[string]string
    LogSinkFn   func(ctx context.Context, runID string) (stdout, stderr io.WriteCloser)  // F2: новое
    AgentTokens AgentTokenSource
    ModuleDir   func(orgID, runID, name string) (string, []terraform.TfFile, bool) // F4: было func(name string) ...
}

// internal/infrastructure/execution/recipe_activities.go — прокинуть orgID/runID в резолв.
type ProvisionActivityInput struct {
    // ...existing...
    OrgID string // F1: ссылка на секрет, не сырое значение
    RunID string // F2/F4: для log sink и per-run workdir
}

// internal/agent/nomad_activities.go — F3 вариант A.
type NomadSubmitJobInput struct {
    JobJSON     string
    WaitTimeout time.Duration // было: захардкожено defaultNomadWaitTimeout
}
```

`RunRecipeWorkflow`/`ProvisionActivityInput`/`TeardownActivityInput` (`internal/workflows/runrecipe.go:164-199`) уже несут контекст рана — добавление `OrgID` туда безопасно (не секрет, обычный identifier, идёт в историю Temporal как есть, как и остальные поля).

---

## 5. Что переиспользуем

- Компилятор и DAG-интерпретатор `ExecuteCompiledPlanWorkflow` (`internal/workflows/dslrun.go:74-197`) — без изменений формы (вариант A из F3 не трогает step-типы).
- `terraform.Actor`'s `destroyOnApplyError` (`actor.go:279-284`) и on-disk fallback в `DestroyExisting` (`actor.go:288-311`) — уже дают часть F4 «destroy-on-error» бесплатно, нужно только верифицировать live.
- `SecretStore`/AES-GCM seal-механизм (`internal/infrastructure/identity/secrets.go:15-24`) для F1 — переиспользуем инфраструктуру, заводим новый namespace, не пишем seal с нуля.
- `NomadJobStatusActivity` (`nomad_activities.go:162-189`) — уже существующий one-shot примитив, на нём строится вариант B из F3 без нового кода в агенте.
- `AssignNomadRoles` (`internal/domain/agent/nomad_roles.go:21-51`) — готовый примитив для F5, просто не подключён.
- Observability-пайплайн (VictoriaMetrics/Logs, Vector, Grafana relay — product-vision §6) как приёмник для F2 per-run логов, вместо нового лог-стораджа.

---

## 6. Поток данных

```
КРЕДЫ (F1):
  org secret store (identity_secrets, new namespace)
    ──(ProvisionActivity/TeardownActivity резолвит по orgID, вне Temporal-истории)──>
  provider.Deps.EnvFn(ctx, orgID) ──> terraform.WithEnv(...) на каждый Apply/Destroy

ЛОГИ (F2):
  ProvisionActivity/TeardownActivity ──(runID)──> provider.Deps.LogSinkFn(ctx, runID)
    ──> per-run io.Writer ──> terraform.Option{stdout,stderr} / docker exec writer
    ──> log backend (Vector/VictoriaMetrics Logs, run_id-меченный) ──> [Run: SP-E лог-реф]

HEALTH-WAIT (F3, вариант A):
  executeServiceJob ──> NomadSubmitJobActivity(WaitTimeout из workflow.yaml)
    ──> heartbeat(status) ──> [Run per-job status: SP-E]

TERRAFORM STATE (F4):
  provider.Deps.ModuleDir(orgID, runID, name) ──> per-run WdId ──> per-run workdir/tfstate
    (было: константный WdId "yandex", общий на все раны)
```

---

## 7. Зависимости

- **SP-B (Catalog + tenancy + RBAC)** — SP-F зависит: org-каталог провайдеров и их creds-хранилище (§3 F1 «где хранятся секреты (identity_secrets?)» — открытый вопрос product-vision §9.5) должен закрепить SP-B; SP-F фиксирует контракт потребления (`EnvFn(ctx, orgID)`), не владеет схемой хранения.
- **SP-E (Run-модель rewrite)** — SP-F зависит и отдаёт: Run нуждается в per-job health-статусе (F3) и лог-рефах (F2) — SP-F производит эти сигналы, SP-E — потребитель/хранитель (`Run.рефы на метрики/логи`, product-vision §3.4).
- **SP-A (Schema-слой)** — косвенно: если org-креды провайдера когда-нибудь параметризуются через `provider.params`-схему (а не отдельный secret store), это пересекается с A1 (`variables.tf`→`schemapb.Schema`, `sensitive`→secret-флаг). SP-F не блокируется на этом — секреты хранятся отдельно от form-schema.

---

## 8. Тестирование

- F1: unit — `EnvFn` резолвит верный секрет по `orgID`, разные org получают разные креды, отсутствие секрета для org → явная ошибка (не молчаливый fallback на процессные env-переменные); секрет не попадает в сериализованный вход активности (grep Temporal history payload в интеграционном тесте).
- F2: unit — per-run writer получает вывод terraform apply/destroy и docker exec, два параллельных рана не смешивают вывод (два `io.Writer` с разными `run_id`-метками); live — реальный лог виден в Vector/VictoriaMetrics Logs с правильным `run_id`.
- F3: unit — вариант A: `WaitTimeout` из `workflow.yaml` долетает до `NomadSubmitJobInput`; таймаут → структурированная ошибка, не голое `context deadline exceeded`. Live — БД с медленным стартом (>5 мин init) должна либо успешно дождаться (увеличенный таймаут), либо явно провалиться с диагностируемой причиной, не «false COMPLETED».
- F4: **live на dev-стенде** — полный terraform-цикл (apply → job execute → destroy) на yandex, включая: два параллельных рана не конфликтуют по `WdId`/state; принудительный обрыв control-plane между provision и teardown → повторный teardown (или ручной) корректно доводит destroy до конца (верификация `DestroyExisting`'s on-disk fallback, `actor.go:288-311`, живьём, не только в unit).
- F5: unit/regression — docker teardown после рестарта воркера действительно очищает контейнеры (текущий баг воспроизводится тестом на `byNet`-миссе); `AssignNomadRoles`, если подключается в рамках SP-F, — e2e на терраформ-пути с >1 non-gateway node, DB-джоба реально размещается не только на gateway.
- Общий acceptance: повторить исходный e2e-сценарий (docker recipe-run, product-vision §10 «живой e2e... GREEN») **после** всех правок F1-F3 — регресс не допускается; затем повторить тот же сценарий на terraform/yandex (новый acceptance-кейс, ранее не существовавший).

---

## 9. Открытые вопросы

1. **Где именно хранить org-креды провайдера** (product-vision §9.5, ещё не закрыт SP-B): новый namespace в `identity_secrets`, или отдельная таблица `provider_credentials` со своим seal-механизмом? Влияет на точную сигнатуру `EnvFn`.
2. **F3 вариант A vs B** — начинать с минимального (таймаут-параметр) или сразу проектировать decoupled wait-step? Зависит от того, насколько реальные БД-топологии (postgres/ydb HA) укладываются в разумный (пусть и настраиваемый) таймаут активности, или системно нужен произвольно долгий, наблюдаемый wait — выяснится по факту live-тестов §8.
3. **Per-run terraform workdir cleanup** (F4): после успешного `Destroy` кто и когда удаляет per-run workdir с диска (сейчас единственный `"yandex"`-workdir жил вечно, вопрос ретеншна не стоял)? Нужна политика (удалять сразу после успешного destroy vs хранить N дней для дебага).
4. **AssignNomadRoles подключение** (F5) — входит ли реально в объём плана SP-F, или переносится в отдельный tracked carryover (multi-node placement — не deploy-gated блокер для текущего single-node e2e, но блокер для реалистичных multi-node HA-топологий).
5. **LogSinkFn backend** (F2) — переиспользуем существующий Vector/VictoriaMetrics Logs пайплайн агентских логов «как есть», или provider-логи (control-plane-side, не agent-side) требуют отдельного пути передачи (control-plane обычно не имеет прямого Vector-сайдкара, в отличие от agent-нод)?

---

## 10. Связанное

- [Product vision](2026-07-08-product-vision.md) §7 SP-F, §9.5.
- [SP-A: Schema-слой](2026-07-08-sp-a-schema-layer.md) — формат-эталон для этой спеки.
- Текущее состояние движка: живой e2e docker recipe-run на dev-стенде — GREEN (compile→provision→execute(nomad)→teardown→COMPLETED); terraform-путь — не верифицирован live (см. §3 F4).
