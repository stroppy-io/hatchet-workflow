# pipelines — Graphene-пайплайны stroppy

Отдельный Go-модуль `github.com/stroppy-io/stroppy-cloud/pipelines` (go.work в
корне). Четыре бинарника, каждый — один `pipeline.Main`:

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
- Workload env, задаваемый пайплайном: `STROPPY_URL`, `STROPPY_DRIVER_TYPE`,
  `STROPPY_INSERT_METHOD`, `STROPPY_OTLP_HEADERS`.
- Имена облачных ресурсов: `stroppy-<tenant>-<run8>-...`; агенты
  `<run8>-<machine>`.

## Зависимости (не трогать без причины)

- provider-yc v0.14.0 (импорт только versioned `apis/cluster/*/v1alpha1`,
  корень `apis` тянет pre-release `common/v2`), provider-aws/v2 v2.6.0
  (типы с суффиксом `_2`), crossplane-runtime/v2 **v2.2.0** (≥2.3.3 без
  `apis/common/v1`), k8s 0.36.3, controller-runtime 0.24.0, docker
  v28.5.2+incompatible (v29 переехал в moby/moby/api).

## Проверка

```
cd pipelines
go test ./... -count=1               # 83 теста, golden: -update
golangci-lint run --config ../.golangci.yaml ./...
make -C .. build-pipelines           # bin/stroppy-*
GRAPHENE_MANIFEST=1 ../bin/stroppy-run | jq '.activities|length'   # 17
```
