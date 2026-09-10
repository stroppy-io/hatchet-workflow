# pipelines — Graphene-пайплайны stroppy

Отдельный Go-модуль `github.com/stroppy-io/stroppy-cloud/pipelines`. Без
go.work: сервер подключает `schemas`/`spec` как обычную зависимость по версии
(тег `pipelines/vX.Y.Z`). Четыре бинарника, каждый — один `pipeline.Main`:

| Бинарник | id | Что делает |
|---|---|---|
| `cmd/run` | `stroppy-run` | RunSpec → машины (Crossplane yc/aws) → агенты → контейнеры → stroppy-сегменты → Result |
| `cmd/suite` | `stroppy-suite` | Suite → child-run `stroppy-run` на каждую ячейку (`RunAll`), сводка |
| `cmd/provider-verify` | `stroppy-provider-verify` | Проверка ключа/прав провайдера, read-only, ничего не создаёт |
| `cmd/quotas` | `stroppy-quotas` | Квоты и usage compute/vpc провайдера |

Все входы/выходы — `spec/` (Go-зеркало схем `schemas/spec/*`), протестировано
на совпадение через `Bake` (`spec/spec_test.go`).

## Раскладка

```
spec/                 Run, Suite, ProviderVerify, Quotas + Result'ы; Duration ("5m")
schemas/              все schemapb-схемы продукта (свой AGENTS.md)
internal/run          workflow stroppy-run: run.go (фазы), deploy.go, workload.go,
                      template.go (${ip:...}), phase.go (milestones+parallel), record.go
internal/suite        fan-out child-run'ов
internal/provision    Provider{Kind,Scheme,Provision,Record}: yandex.go, aws.go → Infra
internal/activities   тела activities (воркер: creds.go; агент: hostprep/files/docker/stroppy)
internal/cloud        SDK-клиенты yc/aws для verify/quotas
internal/probe        тела пайплайнов verify/quotas
internal/events       Emit(ctx, name, payload) — milestones для проекции сервера
internal/topo         toposort/GroupBy/Slug — чистые, тестируемые
```

## Правила workflow-кода (`internal/run`, `internal/suite`, `internal/probe`)

- **Recording pass.** `record.go` объявляет ВСЕ activities, kinds (оба
  провайдера), docker/agent/artifact/stand на нулевом проходе. Новая
  activity → `activities.Register` + константа `Name*`; новый k8s kind →
  `Provider.Record`. Проверка: `GRAPHENE_MANIFEST=1 ./stroppy-run`.
- Детерминизм: итерация только по `infra.Order`, `topo.SortedKeys`,
  `topo.GroupBy`; никаких `map` range в workflow.
- `parallel()` — workflow-горутины; внутри НЕ вызывать `ToStand`
  (`pipeline.Context{Context: gctx}` теряет pipelineId).
- Фазы: `provisioning → deploying → workload → collecting → teardown`,
  каждая через `phase[T]` (milestones `phase.started/finished/failed`, panic
  от `Ready` перехватывается и перекидывается).
- Сегменты workload — последовательно, `AtMostOnce`, таймаут
  `duration*1.5 + warmup + 30m`.
- `Keep` → `ToStand(infra.Root, KeepFor)` после результата; иначе cleanup
  interceptor Graphene сносит всё каскадом от root-сети.

## Правила activities (`internal/activities`)

- Тело = чистая функция `(ctx, Req) (Res, error)`; `Res` обязателен
  (`activity.Fn`). Ошибка = ретрай по политике, поэтому «плохой результат»
  (нет прав, сломан профиль) возвращается В результате с `nil` err.
- Агентские activities работают в run-workspace агента
  (`machine.Workspace()`); относительный `Dir` резолвится от него, путь
  одинаков на хосте, в контейнере агента и для docker daemon (bind-mount).
- `EnsureProviderConfig` — единственная, что ходит в k8s напрямую: пишет
  Secret + ProviderConfig `t-<tenant>` в `crossplane-system`. Не
  graphene-ресурс (общий на тенанта). Kubeconfig: in-cluster, иначе
  graphene-секрет `kubeconfig` в ns тенанта.

## Плейсхолдеры и env (контракт с сервером)

- `${ip:<machine>}`, `${ip:role:<role>}`, `${ips:role:<role>}`,
  `${public_ip:<machine>}` — в env/cmd/files контейнеров и env workload.
  Неизвестное имя/роль — ошибка до деплоя.
- Подключение к БД: `workload.url` RunSpec (с плейсхолдерами) + `driver_type`
  + `driver` (lowerCamel-ключи `drivers.0` stroppy-config.json) — пайплайн
  только пишет их в конфиг. Никакого env-моста: stroppy 6 типизирован.
- Имена облачных ресурсов: `stroppy-<tenant>-<run8>-...`; агенты
  `<run8>-<machine>`.

## Зависимости (не трогать без причины)

- provider-yc v0.14.0 (импорт только versioned `apis/cluster/*/v1alpha1`,
  корень `apis` тянет pre-release `common/v2`), provider-aws/v2 v2.6.0
  (типы с суффиксом `_2`), crossplane-runtime/v2 **v2.2.0** (≥2.3.3 без
  `apis/common/v1`), k8s 0.36.3, controller-runtime 0.24.0, docker
  v28.5.2+incompatible (v29 переехал в moby/moby/api).

## Симуляция прогона (pipelinetest)

`internal/run/sim_test.go` гоняет ВЕСЬ workflow `stroppy-run` на
`pipeline/pkg/pipelinetest` (Temporal testsuite + модель ресурсов/агентов/
cleanup/stand) с адаптерами `library/k8s/k8stest` (Crossplane-объекты
становятся Ready фикстурами в виртуальном времени) и
`library/docker/dockertest`. Activities машин — моки `OnAgentActivity` по
агенту; run-queue контракты (`ensure-config`, события) — `Handle1`. Ни
облака, ни docker, ни сервера: `go test ./internal/run -run TestSimulated`.
Покрыто (`sim_test.go`, `sim_more_test.go`, 16 сценариев): happy path
postgres/yandex и noop/aws; неизвестный провайдер и отказ `ensure-config`
(ничего не создано); отказ VM (квота) и агент без коннекта (таймаут) —
чистый каскад; healthcheck-fail останавливает deploy; слои `depends_on` на
двух машинах, `${ip:<m>}`/`${ips:role}`; baseline-fail и артефакт-fail
нефатальны; два сегмента, `result_expectations` → degraded; потеря
activity сегмента (AtMostOnce, без повтора); cancel в provisioning и в
workload; `keep` → stand + TTL и `keep` игнорируется после провала.
`internal/suite/sim_test.go` — fan-out через серверный контракт
start/await-child; `internal/probe/sim_test.go` — verify/quotas. Правило:
любой новый шаг workflow — сначала сюда, потом на кластер.

Ловушка Temporal: контексты выводятся по конкретному типу — в
`workflow.WithCancel/Go/NewWaitGroup/AwaitWithTimeout` отдавать
`ctx.Context` (встроенный `workflow.Context`), не `pipeline.Context`,
иначе `panic: cancelCtx not found` на первом же параллельном ожидании.

## Проверка

```
cd pipelines
go test ./... -count=1               # 113 тестов, golden: -update
golangci-lint run --config ../.golangci.yaml ./...
make -C .. build-pipelines           # bin/stroppy-*
GRAPHENE_MANIFEST=1 ../bin/stroppy-run | jq '.activities|length'   # 18
```

## stroppy 6 (без фолбеков на v5)

Пакет `stroppycfg/` — чистая половина запуска stroppy: рендер
`stroppy-config.json` и командной строки, разбор вывода. Источник правды —
чекаут `~/devel/github/stroppy-io/stroppy` (`stroppy probe -o json`,
`stroppy run <workload> --help`, `stroppy help config-file|drivers|steps`).

- Образ: `ghcr.io/stroppy-io/stroppy:v6.0.0.62` (тег = релиз + номер сборки),
  entrypoint `stroppy`, WORKDIR `/workspace`; контейнер на host-сети,
  директория сегмента примонтирована как `/workspace`.
- Команда: `run -f /workspace/stroppy-config.json --log-mode production
  --log-level <lvl> [--<extra>=<v>…]`. Конфиг: `script`, `global{runId,
  metadata, logger, exporter.otlpExport}`, `drivers.0`, `run{executor, vus,
  duration|iterations, queryTimeout}`, `params{<lowerCamel>}`, `steps/noSteps`.
  Snake_case ключи схемы → lowerCamel (`scale_factor` → `scaleFactor`);
  `sql_file`/`schema_file` с именем файла из `files` → `/workspace/<name>`.
- Итоги: JSON-отчёта у `stroppy run` НЕТ. Пайплайн парсит stderr-блоки
  `=== bench summary ===` (counters + histograms `count/avg/p50/p90/p95/p99`,
  мс) и `=== bench completed with errors ===`, плюс stdout-строку
  `{"compliance": …}` (TPC-C). Nonfatal-ошибки = exit 0 (учтены в `errors`),
  130/143 — cancel, 1 — ошибка. Пороги (`thresholds`) применяет пайплайн.
- Baseline: `stroppy baseline --json --no-save --download always [...]` до
  сегментов на runner-машине; отчёт schema 1 → `result.baseline`; fail
  вердикта не валит прогон.
- Сборки с коммита: версия `nightly-<sha>` (так печатает `stroppy version`),
  образ задаёт каталог. Параметры новых сборок, не описанные схемой, идут через
  `extra_params` как типизированные флаги.
