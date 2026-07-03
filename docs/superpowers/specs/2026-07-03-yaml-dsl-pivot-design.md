# Пивот: YAML DSL + внешние провайдеры + Nomad

Дата: 2026-07-03. Статус: дизайн утверждён, сабпроект 1 в работе.

## 1. Мотивация

Сегодня вся логика деплоя захардкожена в Go:

- **Провайдеры** — жёсткий enum из двух (`PROVIDER_DOCKER`, `PROVIDER_YANDEX`),
  switch в `internal/workflows/deployment.go` + рендер параметров в
  `provider_render.go` (~535 LOC). Добавить провайдера снаружи невозможно.
- **Рецепты БД** — ~5.7 KLOC в `internal/domain/database/*`: 11 kinds, на
  каждую БД topology builder + deployment renderer, диспатч — switch по kind.
  Владелец организации не может кастомизировать ни топологию, ни шаги.
- **Пресеты** — ещё ~1 KLOC Go-сидов (`internal/app/presets_seed.go`).

При этом под Go-рецептами уже лежит чистый data-driven слой: proto
`AgentStep` (CreateDir/WriteFile/FetchFile/CallCmd) + `EngineComponent`
assembler. Go-код по сути генерирует данные — значит его можно заменить
декларативным форматом без переделки исполнения.

## 2. Целевая архитектура

```
владелец org → internal git (yaml-репо) → browser IDE (валидация DSL)
                        │ push/tag
                        ▼
            компилятор YAML → workflow-граф (DAG)
                        │
              Temporal (движок исполнения, как сейчас)
               │                    │
               ▼                    ▼
        агент (node-prep:        Nomad API (сервисы: БД,
        mkfs, sysctl, apt,       exporters, stroppy —
        нестандартные cmd)       docker jobs поверх VM-флота)
                        ▲
             terraform-провайдеры (плагинные) → VM + nomad clients
```

### Принятые решения

| Вопрос | Решение |
|---|---|
| Scope Nomad | Параллельно с агентом: Nomad забирает запуск/супервизию сервисов, агент остаётся каналом «сырая команда на ноде» (mkfs, sysctl, apt). Systemd-путь в рецептах умирает. |
| Nomad driver | **docker везде** (dev и stage). Один формат `service:` — image + ports + volumes + env. |
| Топология Nomad | **Per-run кластер**: server на gateway-ноде рана (`bootstrap_expect=1`), clients на DB-нодах через cloud-init. Control plane НЕ дозванивается до Nomad API — gateway-агент (Temporal worker) получает активности `NomadSubmitJob`/`NomadJobStatus`/`NomadAllocLogs`, дёргающие `localhost:4646`. Ноль новых inbound-портов. Teardown = terraform destroy, GC не нужен. |
| Пресеты | Умирают как класс. Замена — переиспользуемые параметризованные YAML-фрагменты (`include:` с `inputs:`, аналог GitLab CI components). Wizard в UI генерирует YAML. |
| Workflow | Произвольный DAG: `needs:`, `when:`, `matrix:`. Исполнитель — generic Temporal-интерпретатор графа (durability/retry/observability сохраняются). |
| Кастомизатор | Владелец организации в продукте. Идеал — internal git + browser IDE с валидацией DSL (сабпроект 4). |
| Perf fidelity | Осознанно принят docker-оверхед (host network + volume mounts минимизируют). OrioleDB уже так живёт, stage e2e зелёный. |

### Декомпозиция на сабпроекты

1. **DSL + компилятор** (ядро, делается первым) — схема cluster.yaml +
   workflow.yaml, include-механизм, слой контрактов, компиляция в
   Temporal-исполняемый план.
2. **Провайдер-интерфейс** — tf-модуль как внешний контракт, docker builtin.
   Почти независим от 1, можно параллельно.
3. **Nomad execution backend** — bootstrap per-run кластера, шаг `service:` →
   Nomad job. Зависит от 1.
4. **Git + browser IDE** — internal git + Monaco + yaml-language-server +
   composed JSON Schema + check-endpoint. Зависит от 1 (схема — источник
   валидации). Выбор основы (Forgejo / go-git / BYO) отложен.

Порядок: 1 → (2 ∥ 3) → 4.

## 3. Контракт провайдера

Провайдер = каталог (builtin или в git организации), пишет админ:

```
providers/yandex/
  manifest.yaml      # capabilities, lowering, метаданные
  module/            # обычный terraform-модуль
    variables.tf     # ← источник схемы параметров (terraform-config-inspect)
    main.tf
    outputs.tf       # ← обязан отдать stroppy_machines
```

Конвенции модуля (вместо отдельного lowering-кода):

- **Схема `provider.params` не пишется руками** — деривится из `variables.tf`
  (имя, тип, default, description → фрагмент JSON Schema).
- **Стандартный вход**: variable `stroppy_nodes` — список запрошенных машин
  `[{id, cpu, ram_gb, disk: {size_gb, type}, ext: {...}}]` (уже после
  lowering, см. §5).
- **Стандартный выход**:

```hcl
output "stroppy_machines" {
  value = [for m in yandex_compute_instance.node : {
    id         = m.labels.stroppy_node_id
    private_ip = m.network_interface[0].ip_address
    public_ip  = try(m.network_interface[0].nat_ip_address, null)
  }]
}
```

Runtime подаёт inputs, забирает `stroppy_machines`, лоуверит в `MachineState`.
Docker-провайдер — builtin-реализация того же интерфейса без terraform.

## 4. DSL

Два вида документов + переиспользуемые фрагменты. Выражения — `${{ CEL }}`.

### cluster.yaml (compose-подобный: «что стоит»)

```yaml
version: 1
provider:
  use: yandex          # ссылка на провайдер
  params:              # ← валидируется динамической схемой из variables.tf
    zone: ru-central1-a

machines:
  db:
    count: 3
    resources: { cpu: 8, ram: 32g, disk: { size: 100g, type: ssd } }  # домен
    yandex:                                  # ← ext-блок (см. §5)
      platform_id: standard-v3
      disk_type: network-ssd-nonreplicated
  runner:
    count: 1
    resources: { cpu: 4, ram: 8g }

services:                # docker jobs, исполняются Nomad'ом
  postgres:
    on: db               # группа машин → job per machine
    image: postgres:17
    network: host
    volumes: [ "/data/pg:/var/lib/postgresql/data" ]
    env:
      PATRONI_ETCD_HOSTS: ${{ machines.db | map(m, m.ip + ":2379") | join(",") }}
    configs:
      - template: files/patroni.yaml.tpl
        dest: /etc/patroni/patroni.yaml
    health: { http: ":8008/health", timeout: 120s }
```

### workflow.yaml (CI-подобный: «в каком порядке», произвольный DAG)

```yaml
jobs:
  prep-disks:
    on: db
    steps:
      - cmd: mkfs.xfs ${{ machine.disks[0].device }} && mount ...
  etcd:
    needs: [prep-disks]
    service: etcd              # up сервиса из cluster.yaml (Nomad job)
  patroni:
    needs: [etcd]
    service: postgres
  wait-primary:
    needs: [patroni]
    on: runner
    steps:
      - wait: { http: "http://${{ machines.db[0].ip }}:8008/primary", timeout: 300s }
  bench:
    needs: [wait-primary]
    matrix: { workload: [insert, select] }
    service: stroppy
    with: { args: "--workload ${{ matrix.workload }}" }
```

Шаги-примитивы = существующие `AgentStep` (cmd / write_file / fetch / dir) +
новые `service:` (Nomad job) и `wait:` (healthcheck). Переиспользование —
`include:` с `inputs:`: параметризованные фрагменты заменяют пресеты.

## 5. Доменная модель, ext-слой и lowering

**Доменная модель — единственный общий язык модулей.** Типизированные
сущности: `MachineGroup{count, cpu, ram, disks[]}`, `Disk{size, type}`,
`Workload{iterations, row_bytes, …}`, `Network`. Модули говорят контрактами о
ней, а не друг о друге (иначе N×M связей).

Провайдер-специфичные атрибуты машины отвязаны от домена тремя механизмами:

### 5.1 Ext-блок

Блок с ключом `provider.use` внутри machine group (`yandex:` в примере выше).
Форму декларирует сам провайдер — variable `stroppy_machine_ext`
(object type) в `variables.tf`; из неё деривится фрагмент JSON Schema →
попадает в composed-схему организации → IDE автокомплитит и валидирует
ext-поля наравне с доменными. Редактируемость полная, схема — провайдера.

### 5.2 Lowering-таблица

Мост доменных enum'ов в термины провайдера, декларируется в манифесте:

```yaml
# providers/yandex/manifest.yaml
lowering:
  disk.type: { ssd: network-ssd, hdd: network-hdd, local: local-ssd }
```

Прецеденс при компиляции: `доменное значение → lowering → ext-override`
(ext-поле победило — компилятор лишь сверяет его с capabilities). В tf-модуль
уезжает финальное значение.

### 5.3 Правило видимости (собственно отвязка)

| кто пишет CEL | видит |
|---|---|
| компонент / workload (`requires:`) | **только домен**: `inputs.nodes.count`, `m.ram_gb`, `m.disk_gb` |
| провайдер (`capabilities.constraints:`) | домен **+ свой ext**: `machine.ext.core_fraction == 100 ? … : true` |

Компонент физически не может завязаться на `platform_id` — поля нет в его
типовом окружении, выражение не компилируется. Рецепты переносимы между
провайдерами as-is; при смене `provider.use` компилятор подсвечивает
осиротевшие ext-блоки (unknown key в новой composed-схеме).

## 6. Слой контрактов (requires / provides / capabilities)

Универсальный механизм кросс-валидации вместо прямого кода. Три стороны:

### Провайдер: что он может дать домену

```yaml
# providers/yandex/manifest.yaml
provides: [machines, disk.network-ssd, disk.local-ssd, network.private]
capabilities:
  machine:
    cpu:  { enum: [2, 4, 8, 16, 32, 64] }
    ram:  { expr: "ram_gb in [cpu, cpu*2, cpu*4, cpu*8]" }
  constraints:
    - expr: "machine.disk.type == 'local-ssd' ? machine.cpu >= 8 : true"
      message: "local-ssd только от 8 vCPU"
```

### Компонент: что он требует от домена

```yaml
# components/etcd/component.yaml
inputs:
  nodes: { type: machine_group }
requires:
  - expr: "inputs.nodes.count >= 3 && inputs.nodes.count % 2 == 1"
    message: "etcd: нечётный кворум ≥ 3"
provides: [kv.etcd]
```

```yaml
# components/patroni/component.yaml
requires:
  - capability: kv.etcd        # матчинг provides/requires, не прямая ссылка
  - expr: "inputs.nodes.every(m, m.ram_gb >= 4)"
```

### Workload: требования с арифметикой

```yaml
# workloads/insert/component.yaml
inputs: { iterations: int, row_bytes: { type: int, default: 512 } }
requires:
  - expr: "target.disk_gb * 1e9 >= inputs.iterations * inputs.row_bytes * 2.5"
    message: "диск мал для заданного числа итераций"
```

### Алгоритм проверки (компилятор, не знает про конкретные модули)

1. Собрать доменный граф из cluster.yaml (машины, диски, привязки компонентов).
2. **Capability-матчинг**: каждый `requires: capability:` закрыт чьим-то
   `provides:` в графе; незакрыт — ошибка со ссылкой на модуль.
3. **CEL-предикаты**: все `requires.expr` и `capabilities.constraints`
   выполняются над типизированным графом; fail → диагностика с message
   автора модуля.

Это **checker, не solver**: ловит нарушение, не подбирает значения
(авто-подбор монотонных формул — возможное развитие, вне v1). Тот же движок
отдаёт диагностики в IDE через check-endpoint — нарушения видны в редакторе
до запуска.

Разделение труда: **админ** пишет tf-модуль + capabilities манифеста;
**автор компонента** (мы или организация) пишет requires/provides;
**владелец org** собирает cluster/workflow и получает машинную проверку на
компиляции.

## 7. Компилятор и исполнение

Компилятор — чистый Go-пакет без I/O:

```
parse → include-resolve → schema-validate → contract-check
      → graph-check (ацикличность, ссылки on:/needs:/service:) → lower
```

Target — существующий proto-слой: `InfrastructurePlan` +
`DeploymentPlan`/`AgentStep` + новый `NomadJobStep`. Temporal получает один
generic-workflow «интерпретатор DAG» вместо захардкоженной stage-машины
`test.go`. Retry, heartbeat, observability, logs, UI — нетронуты. Nomad API
дёргается активностями gateway-агента (`localhost:4646`).

## 8. Динамическая валидация и IDE

Схема composed, не статическая (провайдеры кастомные → их параметры и
ext-блоки известны только в рантайме организации):

```
core-schema (статика: machines/services/jobs/steps)
   + $defs.providerParams ← из variables.tf установленных провайдеров org
   + $defs.machineExt     ← из stroppy_machine_ext провайдеров org
   + $defs.includes       ← inputs-схемы фрагментов org
        ↓
GET /api/orgs/{org}/dsl/schema.json   (кеш по ревизии провайдеров)
```

- **Один источник правды**: composed-схему используют и компилятор при
  запуске, и IDE (Monaco + yaml-language-server по schema-URL).
- Что схемой не выразить (ацикличность DAG, ссылки, contract-check,
  CEL-типы) — компилятор в check-режиме отдаёт diagnostics по API; IDE
  показывает как линтер (позже — полноценный LSP-endpoint).
- Провайдер обновился → ревизия схемы сдвинулась → IDE и компилятор
  синхронно видят новые параметры.

## 9. Миграция

- Старый Go-путь живёт рядом за флагом до конца миграции.
- 11 БД переводятся в YAML по одной; критерий эквивалентности — дифф-тест:
  YAML-рецепт компилируется в план, эквивалентный Go-рендеру (по
  нормализованному `DeploymentPlan`).
- Пресеты/wizard переезжают на YAML-генерацию в конце (после сабпроекта 4
  или временно поверх файлового хранилища рецептов).

## 10. Тестирование

- **Компилятор**: golden-тесты (YAML → нормализованный план), негативные
  кейсы на каждый класс диагностик (schema, contract, graph).
- **Дифф-тесты миграции**: per-DB сравнение с Go-рендером (§9).
- **Контракты**: unit на capability-матчинг и CEL-окружения (видимость
  ext — отдельный негативный тест: компонент, ссылающийся на ext, обязан
  падать на компиляции).
- **e2e**: существующий smoke/stage-контур; первый YAML-рецепт — postgres
  (самый большой Go-рендер, 942 LOC — максимум сигнала).

## 11. Открытые вопросы (вне сабпроекта 1)

- Основа git+IDE (Forgejo / go-git / BYO) — решить перед сабпроектом 4.
- Формат хранения рецептов до появления git (файлы в репо продукта /
  таблица в postgres) — решить в начале имплементации.
- Авто-solver размеров (инверсия монотонных формул requires) — кандидат
  после v1.
- Точный контракт docker-builtin-провайдера (какие ext-поля, какой аналог
  `stroppy_nodes`).
