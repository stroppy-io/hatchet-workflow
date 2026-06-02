# Механика №1 — типизированный gRPC-proto для схем/топологии/параметров всех БД

Статус: **Фаза 2 (брейншторм) — жду согласования surface. Кода пока нет.**
Скоуп: ВСЕ БД (postgres/mysql/mariadb/ydb/ydb-managed/cockroach/picodata) + провайдеры (docker/yandex) + топология, одной шириной разом. Заменяет `schemapb.Baked` на типизированный proto + реестр `validate`. Из gRPC генерим GraphQL+OpenAPI; фронт рендерит из сгенерённого клиента.

---

## 0. Ключевой вывод (переосмысляет всю механику)

В дереве сейчас **два параллельных мира**:

1. **Живой рантайм `ref`** = ручные Go-структуры (`internal/domain/types/run.go`: `DatabaseConfig` + 7 per-DB `*Topology` структур + мешки `map[string]string` + мапы `db_defaults.go`), хранятся как **непрозрачные JSON-блобы** (`runs.snapshot`, `presets.topology`, `run_presets.config`, `suite_items.workload`). Фронт повторно объявляет каждый тип руками в `web/src/api/types.ts` и валидирует/ветвится/считает топологию на клиенте.

2. **Дизайн `origin/temporal`** = целевая форма **уже построена**: provider-agnostic `topology.Topology`/`Component`/`Connection`, `domain.Database`(oneof source)/`Workload`/`Test`/`TestRun`, params-only `models.*PresetRecord` (одно сообщение ≈ одна таблица), планировщик `expand` (реестр) → `buildTopology` → `recipe.Wire` → per-provider деплойер `dockerprov`, и 24 `api/*.proto` сервиса с `validate.rules` + кастомной method-опцией `(cloud.v1.iam.auth)`. **Сгенеренный Go для `domain`/`topology`/`models` физически уже лежит на `ref`, но это мёртвый код.**

Загвоздка: temporal смоделировал параметры БД / настройки провайдера / provider_parms как **`schemapb.Baked` (схема-как-данные)** — непрозрачный `google.protobuf.Struct` + встроенная схема. Это ровно то, что механика должна убить (норт-стар D1; твоя строка задачи «вместо schemapb.Baked»).

**Поэтому Механика №1 = взять скелет сообщений/сервисов temporal, заменить каждое поле `schemapb.Baked` типизированным сообщением, инварианты которого живут в `validate`/CEL.** Переиспользуем структуру (проверенную, сгенеренную, планировщик готов) — меняем только модель данных. Риска сильно меньше, чем гринфилд.

---

## 1. Q1 — модель схемы БД в proto

**Решение: одно типизированное сообщение на движок, выбор через `oneof engine` в `DatabaseSpec`. БЕЗ `schemapb.Baked`. БЕЗ `map<string,string>` верхнего уровня.** Малый escape-хатч `map<string,string> extra_config` на движок остаётся для хвоста конфиг-ключей (текущие `RenderedConfigOverrides`/`*Options`), но всё, что показывает визард — типизированные поля.

Обоснование:
- У движков реально разная форма (PG: patroni/etcd/pgbouncer/sync_replicas; MySQL: group-repl⊕semisync; Picodata: shards/tiers; YDB: fault_tolerance/storage_groups; managed: resource_preset/autoscale). Общий `DatabaseSpec` + map выбрасывает типизацию — то, что убираем. Per-engine сообщения держат каждый кноб типизированным и дают `validate` выражать инварианты по движку.
- `oneof engine` авторитетен; вид БД выводится из того, какая ветка задана (денормализуем `Kind` в `*Record.Summary` для дешёвых list/filter, не в сам `DatabaseSpec`).
- **Версия типизирована по движку** где набор закрыт (enum `PostgresVersion {PG_15,PG_16,PG_17}`), или строка + `validate (in: [...])` где набор плывёт. Допустимые версии отдаём фронту через `SchemaService.ListDatabases` — `DB_VERSIONS` уезжает с клиента.
- **Tuning — типизированное под-сообщение** (`PostgresTuning{shared_buffers, max_connections, work_mem, ...}`), строковое где нужен синтаксис процентов («25%»), числовое где счётчик. Хвост → `extra_config`.
- **Флаг-суп HA/репликации схлопывается в enum/oneof** — главный выигрыш типизации, удаляет фронт-валидаторы:
  - PG `Patroni bool`+`Etcd bool`+`PgBouncer bool` → `HaMode ha` enum (`NONE`/`PATRONI`; etcd подразумевается patroni, отдельного тогла больше нет → убивает клиентский валидатор «Patroni требует Etcd») + `bool pgbouncer`.
  - MySQL `GroupRepl bool`+`SemiSync bool` (взаимоисключающие) → `oneof replication { Async | GroupRepl | SemiSync }` → убивает валидатор «GR⊕semisync» структурно.
- **`validate`-правила = реестр параметров.** Полевые правила (`uint32 {lte:16}`) + CEL уровня сообщения (`sync_replicas <= replica_count`; `instances >= replication_factor*shards`) кодируют каждый инвариант, который фронт сейчас переписывает в `PresetDesigner.tsx`. cel-go гоняет их на сервере (авторитетно); те же правила питают render-хинты `SchemaService` + (опц.) cel-es для живого UX.

Драфт (postgres = пример HA-супа; mysql = пример oneof-репликации; ydb-managed = пример полностью типизированного; остальные по тому же паттерну в Фазе 3):

```proto
// cloud/v1/domain/database.proto
message DatabaseSpec {
  oneof engine {
    PostgresSpec   postgres    = 1;
    MysqlSpec      mysql       = 2;
    MariadbSpec    mariadb     = 3;
    PicodataSpec   picodata    = 4;
    YdbSpec        ydb         = 5;
    YdbManagedSpec ydb_managed = 6;
    CockroachSpec  cockroach   = 7;
    ExternalDatabase external  = 8;   // BYO dsn (заменяет RunConfig.ExternalDB)
  }
}

enum PostgresVersion { POSTGRES_VERSION_UNSPECIFIED=0; PG_15=1; PG_16=2; PG_17=3; }
enum HaMode          { HA_MODE_UNSPECIFIED=0; HA_NONE=1; HA_PATRONI=2; }

message PostgresSpec {
  option (buf.validate.message).cel = {
    id: "pg.sync_le_replicas",
    expression: "this.sync_replicas <= this.replica_count",
    message: "sync_replicas must not exceed replica_count"
  };
  PostgresVersion version       = 1 [(buf.validate.field).enum = {defined_only:true, not_in:[0]}];
  uint32          replica_count = 2 [(buf.validate.field).uint32 = {lte:16}];
  HaMode          ha            = 3;
  bool            pgbouncer     = 4;
  uint32          sync_replicas = 5;
  PostgresTuning  tuning        = 6;
  MachineShape    db_machine    = 7;   // provider-agnostic cores/mem/disk
  MachineShape    proxy_machine = 8;   // только при ha==HA_PATRONI
  map<string,string> extra_config = 15; // escape (был RenderedConfigOverrides + *Options)
}
message PostgresTuning {
  string shared_buffers       = 1;  // "25%" | "2GB"
  uint32 max_connections      = 2;
  string work_mem             = 3;
  string effective_cache_size = 4;
  string max_wal_size         = 5;
}

message MysqlSpec {
  MysqlVersion version = 1;
  oneof replication {                 // GR ⊕ semisync становится структурным
    AsyncRepl    async_repl  = 2;
    GroupRepl    group_repl  = 3;     // validate: 3..9 нод
    SemiSyncRepl semi_sync   = 4;     // validate: >=1 реплика
  }
  MysqlTuning  tuning        = 5;
  MachineShape db_machine    = 6;
  MachineShape proxy_machine = 7;     // proxysql
  map<string,string> extra_config = 15;
}

message YdbManagedSpec {              // на ref уже полностью типизирован — портируем как есть
  YdbManagedKind type            = 1; // SERVERLESS | DEDICATED
  YdbManagedCompute compute_type = 2; // OLTP | OLAP (dedicated)
  string resource_preset_id      = 3; // dedicated
  uint32 node_count              = 4; // dedicated
  YdbManagedAutoScale auto_scale = 5; // dedicated
  string storage_type            = 6;
  uint32 storage_groups          = 7;
  uint32 throttling_rcus         = 8; // serverless
}
```

---

## 2. Q2 — топология

**Решение: берём temporal provider-agnostic `Topology`/`Instance`/`Component`/`Connection` + планировщик-реестр `expand`. Топология = ВЫХОД (материализует бэк), не вход. `Instance.provider_parms : schemapb.Baked` заменяем типизированным `oneof provider_parms { YandexInstanceParams | DockerInstanceParams }`.**

Обоснование (аудит B обосновал):
- Per-DB структуры заставляют дублировать два гигантских `switch`а (`FillMachinesFromTopology` + `BakeMachineOverrideIntoTopology`) и **не имеют модели связей/рёбер**. У proto `Topology` есть типизированный граф `Connection` (proxy→db, replica→primary, monitor→all, workload→proxy) и «добавить БД = зарегистрировать `Expander`, без изменения proto».
- `MachineShape` (cores/mem/disk) держим provider-agnostic; специфику провайдера (zone, platform_id, boot_disk_type, placement) уводим в типизированный `provider_parms` — убивает провайдерские кнобы, сейчас зашитые в `MachineSpec` ref (`DiskType`, `Placement`, `SecondaryDisks`).
- Цепочка материализации: юзер правит `DatabaseSpec` → `expand.Expand(kind, spec)` → `[]VM` → `buildTopology` → `Topology{Instances,Components,Connections}` → per-provider деплойер читает типизированный `provider_parms` → строит tfvars `deployment.Yandex_Input` (ref `task_infra.go` уже делает из машин) / docker-спеку (temporal `dockerprov`). Managed-сервисы → `Component KIND_EXTERNAL` + `external_components` (чисто поглощает спецслучай `YDBManagedTopology`).
- Связь с существующими `deployment.Yandex`/`Docker`: они остаются стоком tfvars/спеки провайдера; `YandexInstanceParams` — per-instance подмножество, которое планировщик раскладывает в мапу `deployment.Yandex_Input` по machine id (маппинг `task_infra.go:477` уже делает, теперь типизированно).

```proto
// cloud/v1/topology/topology.proto
message Topology {
  repeated Instance   instances           = 1 [(buf.validate.field).repeated.min_items=1];
  repeated Connection connections         = 2;
  repeated Component  external_components = 3; // managed (KIND_EXTERNAL)
}
message Instance {
  string       id      = 1;
  MachineShape machine = 2;                    // provider-agnostic
  oneof provider_parms {                       // ТИПИЗИРОВАНО (был schemapb.Baked)
    YandexInstanceParams yandex = 3;
    DockerInstanceParams docker = 4;
  }
  repeated Component components = 5;
  common.Status status         = 6;
}
message Component { string id=1; ComponentKind kind=2; common.Status status=3; }
message Connection {
  string from=1; string to=2;
  ConnectionKind kind=3; ConnectionProtocol protocol=4;
  uint32 port=5; bool inner=6;
}
message YandexInstanceParams { string zone=1; string platform_id=2; string boot_disk_type=3; }
message DockerInstanceParams { string network=1; }
```

---

## 3. Q3 — граница визарда (какие RPC; что фронт перестаёт знать)

Ключ: **при типизированных параметрах сгенеренная GraphQL/OpenAPI-схема И ЕСТЬ дескриптор формы.** Фронт рендерит поля прямо из типов сгенерённого клиента — `schemapb` дескриптор формы не нужен. Добавляем тонкий слой **render-хинтов + авторитета** для того, что типы proto не несут (наборы допустимых значений, диапазоны-по-контексту, условная видимость, sizing-матан, превью топологии).

```proto
// cloud/v1/api/schema.proto  (read-only каталог + render-хинты)
service SchemaService {
  rpc ListDatabases(ListDatabasesRequest) returns (ListDatabasesResponse);     // kinds + labels + versions[] + protocols[]
  rpc GetDatabaseSchema(GetDatabaseSchemaRequest) returns (DatabaseUiSchema);   // хинты полей: диапазоны, enum-опции, CEL видимости, дефолты
  rpc ListProviders(ListProvidersRequest) returns (ListProvidersResponse);     // платформы, зоны, типы дисков, лимиты
  rpc GetScriptCompat(GetScriptCompatRequest) returns (GetScriptCompatResponse);// скрипты для (kind,protocol)
}
// cloud/v1/api/wizard.proto  (авторитет: validate + materialize)
service WizardService {
  rpc ValidateDatabase(ValidateDatabaseRequest) returns (ValidationResult);     // DatabaseSpec -> FieldError[]
  rpc ValidateWorkload(ValidateWorkloadRequest) returns (ValidationResult);     // WorkloadSpec + kind -> FieldError[]
  rpc Materialize(MaterializeRequest) returns (MaterializeResponse);            // Test + ProviderSettings -> превью Topology + node count + sizing
}
```
(Плюс stateful `TestWizardService`/`SuiteWizardService` draft-модель уже спроектирована в temporal — берём как есть, заменив её форму `schemapb.Filled` на патчи типизированных `DatabaseSpec`/`WorkloadSpec`.)

**Фронт перестаёт знать (уезжает за эти RPC + сгенеренный клиент):** все валидаторы `PresetDesigner.tsx`; `TopologyDiagram.tsx` `getRolesFromTopology`/`getRolesFromPresetName`/sizing; `NewRun.tsx` `suggestDiskGb`/`extractDbDiskGb`/ветвление protocol+version+package; `WorkloadForm.tsx` `suggestStroppyMachine`/`probeDriverType`; `SuiteBuilder.tsx` `defaultDBVersionTS`/`cronShapeOK`; и каждый зашитый список в `api/types.ts` (`ALL_DB_KINDS`, `DB_VERSIONS`, `KIND_PROTOCOLS`, `SCRIPT_COMPAT`, `YDB_MANAGED_RESOURCE_PRESETS`, `YC_PLATFORMS`, disk-perf таблицы). Фронт держит: рендер, локальный UX (оптимистичные хинты из `DatabaseUiSchema`), submit.

---

## 4. Q4 — schemapb → typed: что теряем; миграция данных

**Теряем:** (а) рантайм-динамику — добавить кноб теперь = изменение proto + регенерация + редеплой (норт-стар D1 этого и хочет: «добавить БД = proto + регенерация, не непрозрачные данные»); (б) самоописывающийся запечатанный `Baked`-снимок (хранимый run нёс свою схему). (б) митигируем хранением типизированных спек + штампа `proto_schema_version` на записях, чтобы исторические runs десериализовались против известной версии сообщения.

**Получаем:** типобезопасность; сгенеренные клиенты знают формы (graphql/ogen НЕ генерят из непрозрачного `Struct`); validate-как-реестр; бесплатное похудение фронта.

**Миграция (существующие данные в БД):**
- `presets.topology` JSON (старые `*Topology` Go-структуры ref) → типизированный `DatabasePresetRecord.database` + `WorkloadPresetRecord.workload`. Маппинг механический (поля старой структуры ≈ новые типизированные; HA-bool → `HaMode`; мапы опций → типизированный tuning + остаток в `extra_config`). Разовый Go-джоб по таблице `presets`.
- `run_presets.config` (полный RunConfig JSON) → разбить на Database/Workload preset-записи (+ провайдер уходит на suite/run, не в preset).
- `runs.snapshot` (исторические RunConfig-блобы): **forward-only.** Старую таблицу `runs` держим read-only за legacy read path (или best-effort конвертер для отображения). Новые runs пишут типизированный `TestRunRecord`. Логика: runs — исторические артефакты; конвертить каждый блоб = высокий риск, низкая ценность против конверта переиспользуемых presets.
- блобы `suites`/`suite_items` → типизированные `SuiteRecord`/`SuiteRunRecord` по модели temporal.

Открыто: нужен ли конвертер старых runs (display-only) или жёсткий cutover (старые runs только в legacy-view)? **Рек: жёсткий cutover + legacy read path.**

---

## 5. Q5 — codegen инжектит grpc-impl; где живёт app-wiring

Пайплайн (аудит E, конкретно):
1. **Скомпилить API-сервисы в gRPC** — раскомментить `- directory: cloud/v1/api` в `protocols/easyp.api.go.yaml`, чтобы `go-grpc` выдал интерфейсы `XxxServiceServer` в `internal/proto/cloud/v1/api`. (`go`+`validate` уже покрывают структуры сообщений для всего `cloud/**`.)
2. **Добавить два gopherex-плагина** (порядок в `easyp.api.go.yaml`: `go-grpc` → `connect-go`(оставить) → `go-graphql` → `ogen` → `doc`):
   - `protoc-gen-go-graphql` (gqlgen, proto = SoT через autobind). Две фазы: protoc выдаёт `gqlapi/` + `gqlgen.yml` + делегирующий `Resolver`; затем **Фаза B `go generate`** гоняет gqlgen → `exec/exec.go`. Классификация по `idempotency_level`: `NO_SIDE_EFFECTS`→Query, `IDEMPOTENT`/без → Mutation, server-stream→Subscription.
   - `protoc-gen-go-ogen` (REST/OpenAPI, только unary). Нужны **новые аннотации `(ogen.file)` + `(ogen.method)`** (http_method/path/params/responses) на api-proto; `validate.rules` авто-мапятся в OpenAPI-ограничения.
3. **Makefile**: добавить `go generate ./internal/proto/cloud/v1/api/...` после шага api `easyp generate` (Фаза B gqlgen). Блок `tool (...)` в go.mod (Go 1.25.5) пинит оба плагина + `gqlgenrun`.
4. **Инжект grpc-impl / app-wiring:**
   - Руками пишем gRPC-impl (`api.IamServiceServer`, `PresetService`, `WizardService`, `SchemaService`, …), переиспользуя логику `App`/store — **это единый источник.**
   - GraphQL: `exec.NewExecutableSchema(exec.Config{Resolvers:&gqlapi.Resolver{WizardService:wizImpl, ...}})` → `handler.New(schema)` на `/graphql`.
   - OpenAPI: `adapter := apipb.NewOgenAdapter(wizImpl, ...); srv,_ := ogenapi.NewServer(adapter)` под `/api/v1`.
   - Оба монтируются на существующий chi `Router()` (`internal/domain/api/server.go:140`), уже передан как `gateway.Config.HTTPFallback` — **gateway не трогаем.** Опция `(cloud.v1.iam.auth)` переезжает из chi-middleware `RequireRole` в gRPC-interceptor на impl.

Дом app-wiring: новый `internal/api/grpcimpl/` (типизированные impl сервисов) + `internal/api/serve.go` (wiring resolver/adapter), монтаж из `cmd/cli/main.go`.

---

## 6. Q6 — сгенеренный фронт-клиент + удаления

**Решение: фронт потребляет сгенеренный GraphQL-клиент; OpenAPI существует для внешних/CLI потребителей + агента.** Логика: GraphQL даёт типизированный композируемый слой данных с Subscriptions — ложится на будущий шаг норт-стара read-model/live-run-WS; один query-слой для SPA. (REST остаётся для не-SPA клиентов через ogen.) Тулинг: `@graphql-codegen/typescript` + `typescript-operations` (SPA уже на Vite; вывод protobuf-es оставляем только если нужен какой-то не-API proto-тип).

**Удаления на фронте (web/src):**
- `api/types.ts`: `ALL_DB_KINDS`, `DB_META`, `DB_VERSIONS`, `KIND_PROTOCOLS`, `SCRIPT_COMPAT`, `DEFAULT_INSERT_METHODS`, `YDB_MANAGED_RESOURCE_PRESETS`, и все per-DB topology-интерфейсы → заменяются сгенеренными типами + ответами `SchemaService`.
- `PresetDesigner.tsx`: все `validate*` функции (Postgres/MySQL/Picodata/YDB/machine) → `WizardService.ValidateDatabase`.
- `TopologyDiagram.tsx`: `getRolesFromTopology`/`getRolesFromPresetName`/`formatSpec` матан счёта → рендер `Topology` из `WizardService.Materialize`.
- `NewRun.tsx`: `suggestDiskGb`, `extractDbDiskGb`, `PHASE_GROUPS`, ветвление version/protocol/package → на сервер.
- `WorkloadForm.tsx`: `suggestStroppyMachine`, `probeDriverType` (явно «mirrors backend»).
- `SuiteBuilder.tsx`: `defaultDBVersionTS`, `cronShapeOK`.
- `sliders.tsx`: `YC_PLATFORMS`, disk-perf/IOPS таблицы, константа io-m3=93 → `SchemaService.ListProviders`.

Фронт держит: лейаут/рендер шагов, локальный UX-хинт из `DatabaseUiSchema`, submit/poll.

---

## 7. Таблица маппинга старое → новое

| Старое (ref) | Новое (типизированный proto) |
|---|---|
| `types.DatabaseConfig` + 7 `*Topology` структур (run.go) | `domain.DatabaseSpec{oneof engine}` + per-engine `*Spec` |
| `db_defaults.go` `map[string]string` | типизированные `*Tuning` под-сообщения + остаток `extra_config` |
| PG `Patroni`/`Etcd`/`PgBouncer` bool | `HaMode ha` enum + `bool pgbouncer` |
| MySQL `GroupRepl`/`SemiSync` bool | `oneof replication {Async\|GroupRepl\|SemiSync}` |
| `types.Protocols`/`KindProtocols`/`ScriptCompat` (Go-мапы) | `Protocol` enum + `SchemaService.GetScriptCompat` + `validate` CEL |
| `MachineSpec`{cpus,mem,disk,DiskType,Placement,Secondary} | `MachineShape`{cores,mem,disk} + типизированный `provider_parms` |
| switch'и `FillMachinesFromTopology`/`BakeMachineOverrideIntoTopology` | реестр `expand` → `topology.Topology` |
| (нет модели рёбер) | граф `topology.Connection` |
| спецслучай `YDBManagedTopology` | `Component KIND_EXTERNAL` + `Topology.external_components` |
| `StroppyConfig` (рыхлые int/string) | `domain.WorkloadSpec` (enum protocol/insert, oneof termination) |
| `RunConfig` (всё-в-одном) | `domain.Test` (абстракт) + `domain.TestRun` (запечён: provider+topology+db+workload) |
| `RunConfig.ExternalDB`/`PresetID` | `DatabaseSpec.external` / ссылка на preset в `Test` |
| `Preset` (только топология) + `run_presets` (всё) | `models.{Database,Workload,Test}PresetRecord` (params-only, по таблицам) |
| `deployment.ProviderSettings : schemapb.Baked` | `ProviderSettings{oneof provider {Yandex\|Docker}}` типизированный |
| блобы `runs.snapshot`/`presets.topology` JSON | типизированные записи; gorm/CQRS позже (шаг 3 норт-стара) |
| chi-хендлеры (`server.go`) | gRPC-impl → сгенеренные GraphQL+OpenAPI |
| зашитые списки/валидаторы/топология-матан фронта | сгенеренный клиент + `SchemaService`/`WizardService` |

---

## 8. Открытые развилки — нужно твоё решение (до того как разверну полный 7-DB .proto)

1. **Переиспользовать vs переписать:** перенести proto temporal (`domain`/`topology`/`models`/`api`) + Go (`expand`/`recipe`/`dockerprov`) в `ref`, конвертя `Baked`→typed (рек) — vs писать типизированный proto на `ref` заново, игнорируя temporal. Переиспользование сохраняет планировщик + surface сервисов + docker-деплойер; цена — примирение рантайма temporal-эпохи с ref.
2. **Гранулярность tuning:** типизированные well-known кнобы + escape-мапа `extra_config` (рек) — vs полностью типизировано (без мап, каждый кноб — поле) — vs минимум-типизации + бо́льшая мапа.
3. **Миграция старых runs:** жёсткий cutover + legacy read path (рек) — vs строить конвертер blob→typed для исторических runs.
4. **Фронт-клиент:** GraphQL primary для SPA + OpenAPI для внешних (рек) — vs OpenAPI/openapi-ts на оба.
5. **Типизация provider_parms:** `oneof на провайдера` (рек) — vs одно типизированное `ProviderInstanceParams` с полем провайдера + опц. под-полями.

Дефолты выше — мои рекомендации; скажи какие перевернуть, и разверну полный per-DB `.proto` surface (все 7 движков + оба провайдера + все сервисы) на финальный аппрув, дальше Фаза 3.
