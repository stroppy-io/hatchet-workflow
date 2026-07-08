# SP-B: Catalog + tenancy + RBAC

Дата: 2026-07-08. Статус: дизайн утверждён (brainstorming). Ветка: `feat/yaml-dsl-pivot`.
Часть [product-vision](2026-07-08-product-vision.md), саб-проект B (даёт SP-C, SP-D).

---

## 1. Что и зачем

Сегодня provider — не сущность, а файлы **внутри** бандла рецепта: `providers/<name>/manifest.yaml` + `providers/<name>/module/*.tf`, лежащие рядом с `cluster.yaml`/`workflow.yaml` в одном `models.RecipeBundle` (`internal/proto/cloud/v1/models/recipe.proto`). `cluster.yaml`'s `provider.use` (`ast.ProviderUse.Use`, `internal/dsl/ast/cluster.go:31`) — голая строка, которую `internal/services/dsl/service.go`'s `resolveProvider`/`peekProviderUse` резолвит **только** внутри того же бандла (`providers/<name>/manifest.yaml`, см. `manifestPath(name)`). Реестра провайдеров нет, шаринга между рецептами/организациями нет — «кастомизация» = вклеить tf-модуль в свой рецепт заново.

Тенанси в системе уже есть (`internal/services/iam`), но она одноуровневая: `iam_tenants` (= организации) + `iam_memberships` + `iam_roles` (`Scope_SCOPE_TENANT`/`Scope_SCOPE_PLATFORM`) + `iam_accounts.is_admin` (платформенный суперюзер). `models.RecipeRecord` (`recipe_records` таблица, `internal/services/recipe`) — CRUD только на уровне tenant, никакого instance-уровня и никакого разделения provider/workflow.

**SP-B ремоделирует это**: provider и workflow становятся first-class объектами **каталога** — одной формы, на двух уровнях (instance/org), с явными метаданными в postgres и RBAC-гейтами поверх уже существующих `iam.Resource`/`iam.Action`/`iam.Role`/`iam.Membership` примитивов. Хранение файлового содержимого бандлов (git) — SP-C; SP-B описывает МЕТАДАННЫЕ каталога и координируется с SP-C через узкий интерфейс (`BundleStore`, §4).

---

## 2. Scope

**В scope:**
- Новая сущность **CatalogEntry** (общая форма для provider и workflow, `kind ∈ {PROVIDER, WORKFLOW}`), на двух уровнях: `LEVEL_INSTANCE` (глобально, без tenant_id) и `LEVEL_ORG` (= `tenant_id`, переиспользует существующий `iam.Tenant`).
- **Дефолт-ссылка instance → org**: при создании org (или лениво) каждый instance-объект появляется в org-каталоге как запись `origin = LINKED`; org может **форкнуть** свою копию (`origin = FORKED`) — редактирование не трогает ни instance-строку, ни копии других org.
- Локальный авторинг: org создаёт свои собственные объекты (`origin = NATIVE`), не производные от instance.
- **RBAC-гейты**: два новых `iam.Resource` (`RESOURCE_PROVIDER`, `RESOURCE_WORKFLOW`), проаннотированные на новом `CatalogService`'е через `(cloud.v1.iam.auth)` (как уже делают `IamAPI`/`RecipeService`) — авторинг (`ACTION_CREATE/UPDATE/DELETE`/`ACTION_MANAGE`) vs просмотр (`ACTION_READ/LIST`) на org-уровне; instance-уровень — `admin_only` (см. §3 почему).
- Разрыв provider↔workflow: `cluster.yaml`'s `provider.use` перестаёт резолвиться внутри одного бандла — резолвится как слаг **против org-каталога** (Provider-запись, отдельный от Workflow-записи бандл).
- Postgres-схема `catalog_entries` (метаданные) + repo-порты + сервис-порты (Go-интерфейсы), по образцу `internal/services/recipe` и `internal/services/iam`.

**Вне scope (не трогаем/не решаем здесь):**
- Git-бэкенд, IDE (code-server, LSP) — SP-C. SP-B определяет узкий `BundleStore`-интерфейс (§4), достаточный чтобы SP-B тестировался с фейком/стабом, не дожидаясь SP-C.
- Композиция `workflow.inputs ⊕ provider.params` и Filled/Baked-контракт — SP-A. SP-B только гарантирует, ЧТО именно (какая версия provider/workflow) читает SP-D для этой композиции.
- Форма запуска, WASM-валидация — SP-D.
- Модель Run/миграция `TestRunRecord` — SP-E.
- Провайдер-креды per-org (где хранить секреты для terraform) — SP-F; SP-B только фиксирует, что креды навешиваются на Provider catalog entry, не на процесс.
- Миграция существующих строк `recipe_records` в `catalog_entries` — переносится в план SP-B, не решается в спеке (см. §9.4).
- Marketplace/шаринг между instance (vision §8 non-goal).

---

## 3. Компоненты

### B1. `CatalogEntry` — общая модель provider/workflow

Одна форма для обоих `kind`, на двух `level`:
- `id`, `level` (`LEVEL_INSTANCE` | `LEVEL_ORG`), `tenant_id` (пусто для instance), `kind` (`KIND_PROVIDER` | `KIND_WORKFLOW`), `slug`, `version` (иммутабельно, как `RecipeRecord.version`), `entity` (`common.Entity` — id/name/description/author/timings, как у `RecipeRecord`/`RunRecord`).
- `origin` (`ORIGIN_NATIVE` | `ORIGIN_LINKED` | `ORIGIN_FORKED`) + `source_entry_id` (instance-entry, из которого org-запись произошла; пусто для `NATIVE`).
- `source_ref` — непрозрачный указатель на файлы бандла в git (SP-C владеет форматом; SP-B хранит и передаёт его как строку).
- `summary` — денормализованная проекция для списков (для provider: `provides`/`capabilities` кратко; для workflow: `provider_slug`/`machine_group_count`/`service_count`/`compiles`, как сегодняшний `RecipeRecord.Summary`, `internal/services/recipe/recipe.go:396` `deriveSummary`).

### B2. `catalog_entries` — postgres-метаданные

По образцу `iam_tenants`/`recipe_records` (`internal/infrastructure/postgres/queries/iam.sql`, `20260703164318900_add_recipe_records.sql`): индексированная обёртка вокруг JSONB-блоба.

```sql
CREATE TABLE "public"."catalog_entries" (
  "id" text NOT NULL,
  "level" text NOT NULL,          -- 'instance' | 'org'
  "tenant_id" text NULL,          -- NULL for level='instance'
  "kind" text NOT NULL,           -- 'provider' | 'workflow'
  "slug" text NOT NULL,
  "version" int4 NOT NULL,
  "origin" text NOT NULL DEFAULT 'native',   -- 'native' | 'linked' | 'forked'
  "source_entry_id" text NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  "data" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX "uq_catalog_entries_scope_slug_version"
  ON "public"."catalog_entries" ("level", coalesce("tenant_id",''), "kind", "slug", "version");
CREATE INDEX "idx_catalog_entries_scope" ON "public"."catalog_entries" ("level", "tenant_id", "kind");
CREATE INDEX "idx_catalog_entries_source" ON "public"."catalog_entries" ("source_entry_id");
```

`(level, tenant_id, kind, slug, version)` уникально — тот же паттерн, что `uq_recipe_records_tenant_name_version`, расширенный `level`+`kind`.

### B3. `CatalogService` (proto/connect) + RBAC-гейты

Новый сервис `cloud.v1.catalog.CatalogService` (`protocols/cloud/v1/catalog/*.proto`), аннотированный `(cloud.v1.iam.auth)` (`options.proto`) — механизм уже несёт интерцептор на каждый метод, ничего нового изобретать не нужно:

- **Instance-CRUD** (`CreateInstanceEntry`/`UpdateInstanceEntry`/`DeleteInstanceEntry`/`GetInstanceEntry`/`ListInstanceEntries`) — `admin_only = true`. **Почему не `all_of` через `RESOURCE_PROVIDER`/`RESOURCE_WORKFLOW`**: `MethodAuth.all_of` резолвится в конкретном tenant (`tenant_field`, дефолт `tenant_id`, `options.proto:75`), а instance-объекты вне tenant — резолвить permission не в каком tenant. `admin_only` — существующий путь именно для «платформенно-зарезервированных» операций без tenant-контекста (`CreateAccount` уже так гейтится). Открытый вопрос: нужен ли делегируемый (не-`is_admin`) instance-catalog-admin — §9.5.
- **Org-CRUD** (`CreateOrgEntry`/`UpdateOrgEntry`/`DeleteOrgEntry`/`DeleteOrgEntry`) — `all_of: [{resource: RESOURCE_PROVIDER|RESOURCE_WORKFLOW, action: ACTION_CREATE|UPDATE|DELETE}]`, `tenant_field: "tenant_id"` — как у `RecipeService` (`RESOURCE_RECIPE`). Org-admin получает это через свою `owner`/кастомную роль (те же `iam.Role`/`iam.Membership`, ничего нового).
- **Org-Read** (`GetOrgEntry`/`ListOrgEntries`) — `all_of: [{resource: ..., action: ACTION_READ|ACTION_LIST}]` — org-member видит каталог своей org без права его редактировать.
- `LinkInstanceEntry(tenant_id, instance_entry_id) -> CatalogEntry` — материализует один instance-объект в org-каталог как `origin=LINKED` (org-admin, `ACTION_CREATE`).
- `CheckCatalogEntry(kind, files)` — переиспользует `internal/services/dsl.CheckBundle`/`schema.DeriveParamsSchema` за интерфейсом (см. §5): для `KIND_WORKFLOW` — полный `dsl.Compile`-check (как сегодня `RecipeService.CheckRecipe`), для `KIND_PROVIDER` — только derive tfvars-схемы + манифест.

Permission-каталог (`iam/catalog.go`'s `Catalog.Grantable`, reflection по `(cloud.v1.iam.auth)` annotations всех сервисов) **автоматически подхватывает** `RESOURCE_PROVIDER`/`RESOURCE_WORKFLOW` из аннотаций `CatalogService`, плюс синтезирует `ACTION_MANAGE`-wildcard для каждого — без единой строчки нового кода в `catalog.go` (см. §8 тест на это).

### B4. Дефолт-ссылка + форк-на-редактирование

- **Материализация**: при `iam.IamService.CreateTenant` (`internal/services/iam/service.go:697`, тот же шов, где сегодня сеется `owner`-роль + `Membership`) для каждой instance-записи каталога создаётся org-строка `origin=LINKED`, `source_entry_id = instance.id`, без своего `source_ref` (пока не форкнута — файлы читаются через `source_entry_id` косвенно, SP-C решает механику "живого" чтения).
- **Форк-на-редактирование**: `UpdateOrgEntry` на строке с `origin=LINKED` не мутирует её на месте — создаёт новую org-версию (`version+1`, как `nextVersion` в `internal/services/recipe/recipe.go:385`) с `origin=FORKED`, вызывает `BundleStore.Fork` (копия файлов на новый `source_ref`), сохраняет `source_entry_id` (для будущей сверки/diff). Instance-строка и другие org-копии не затрагиваются — соответствует зашитому решению 2.
- **Живая ссылка vs форк по умолчанию** (когда именно копируются файлы: сразу при линковке или лениво при первом редактировании) — открытый вопрос §9.1, решение SP-B не меняет.

### B5. Резолюция provider на этапе компиляции — разрыв с бандлом

Сегодня (`internal/services/dsl/service.go:267` `resolveProvider`): `name := peekProviderUse(files[clusterFile])` → `files[manifestPath(name)]` — оба читаются из ОДНОГО map файлов запроса/бандла.

После SP-B: `provider.use` — слаг **org-каталога** (не путь внутри бандла). Резолюция становится двухшаговой:
1. `CatalogEntryRepo.GetLatestBySlug(ctx, LEVEL_ORG, tenantID, KIND_PROVIDER, slug)` → `CatalogEntry` (или ошибка "provider not found in org catalog").
2. `BundleStore.Read(ctx, entry.SourceRef)` → `map[string][]byte` (provider'а собственный `manifest.yaml` + `module/*.tf`) — тот же формат, что раньше лежал в `providers/<name>/` бандла рецепта, теперь в своём собственном каталожном бандле.

`schema.DeriveParamsSchema`/`ast.DecodeProviderManifest` не меняются — меняется только ОТКУДА берутся байты. `builtinDockerProviderName` (`docker`, `service.go:47`) остаётся спецкейсом без каталожного бандла (как сегодня — `builtinDockerManifest` инлайн-константа), либо (альтернатива, решить в плане) — сеется как instance-каталожная запись `KIND_PROVIDER, slug="docker"` при бутстрапе, чтобы не иметь двух разных путей резолюции.

Аналогично `workflow.yaml`+`cluster.yaml`+`components/*` (сегодня `RecipeBundle.files` целиком) — становятся файлами `KIND_WORKFLOW`-каталожной записи; `provider.use` внутри её `cluster.yaml` ссылается на `KIND_PROVIDER`-запись ТОЙ ЖЕ org по слагу (никогда не поперёк org — соответствует "форма генерится из ОРГ-каталога", vision §3.3).

---

## 4. Публичные интерфейсы

### Go (новый пакет `internal/services/catalog`, по форме `internal/services/recipe`/`internal/services/iam`)

```go
// ===== модель =====

type Level int32
const (
    Level_LEVEL_UNSPECIFIED Level = iota
    Level_LEVEL_INSTANCE
    Level_LEVEL_ORG
)

type Kind int32
const (
    Kind_KIND_UNSPECIFIED Kind = iota
    Kind_KIND_PROVIDER
    Kind_KIND_WORKFLOW
)

type Origin int32
const (
    Origin_ORIGIN_NATIVE Origin = iota
    Origin_ORIGIN_LINKED
    Origin_ORIGIN_FORKED
)

// ===== зависимости (constructor-injected, как recipe.Deps/iam.IamDeps) =====

// CatalogEntryRepo persists catalog.Entry rows. Every method is scoped by
// (level, tenantID) — tenantID MUST be empty for LEVEL_INSTANCE and non-empty
// for LEVEL_ORG (service layer validates, mirrors requireTenant in
// internal/services/recipe/recipe.go:375).
type CatalogEntryRepo interface {
    Create(ctx context.Context, e *catalogpb.CatalogEntry) error
    Get(ctx context.Context, level catalogpb.Level, tenantID, id string) (*catalogpb.CatalogEntry, error)
    List(ctx context.Context, level catalogpb.Level, tenantID string, kind catalogpb.Kind) ([]*catalogpb.CatalogEntry, error)
    // GetLatestBySlug returns the highest-version row for (level, tenantID,
    // kind, slug) — mirrors recipe.RecipeRepo.GetLatestByName; used both by
    // CreateXEntry's version stamping and by provider resolution (§3 B5).
    GetLatestBySlug(ctx context.Context, level catalogpb.Level, tenantID string, kind catalogpb.Kind, slug string) (*catalogpb.CatalogEntry, error)
    // ListBySource returns every org row linked/forked from a given instance
    // entry id — used by CreateTenant seeding and future re-sync tooling.
    ListBySource(ctx context.Context, sourceEntryID string) ([]*catalogpb.CatalogEntry, error)
    Update(ctx context.Context, e *catalogpb.CatalogEntry) error
    Delete(ctx context.Context, level catalogpb.Level, tenantID, id string) error
}

// BundleStore is the SP-C seam: catalog owns no git of its own. Write persists
// a new/updated file set and returns the opaque ref catalog.Entry.source_ref
// stores; Read fetches a ref's files (used by provider resolution, §3 B5, and
// by CheckCatalogEntry); Fork produces a NEW ref pointing at a copy of
// sourceRef's files (used by fork-on-edit, §3 B4) without mutating sourceRef.
type BundleStore interface {
    Write(ctx context.Context, ref string, files map[string][]byte) (newRef string, err error)
    Read(ctx context.Context, ref string) (map[string][]byte, error)
    Fork(ctx context.Context, sourceRef string) (forkedRef string, err error)
}

// Checker validates a catalog entry's files at create/update/check time.
// kind=PROVIDER derives tfvars schema only (schema.DeriveParamsSchema);
// kind=WORKFLOW runs the full dsl.Compile check-mode pipeline
// (internal/services/dsl.CheckBundle) — same function signature that already
// backs recipe.Deps.Checker, reused rather than duplicated.
type Checker func(ctx context.Context, kind catalogpb.Kind, files map[string][]byte) ([]*dslpb.Diagnostic, error)

type Deps struct {
    Entries CatalogEntryRepo
    Bundles BundleStore
    Check   Checker
    Authn   utils.Authn
}

// Service implements catalogpb.CatalogServiceServer.
type Service struct {
    *catalogpb.UnimplementedCatalogServiceServer
    d Deps
}

func NewService(d Deps) *Service

// ===== org-catalog materialization (CreateTenant seam, §3 B4) =====

// SeedOrgCatalog links every LEVEL_INSTANCE entry into tenantID's org catalog
// as origin=LINKED rows. Called from iam.IamService.CreateTenant's tx
// (internal/services/iam/service.go:711), the same place the owner Role +
// Membership are seeded — wired via a narrow interface iam.CreateTenant's
// Deps takes, not an import of this package (keeps iam dependency-light,
// matching its existing Deps-of-interfaces shape).
func (s *Service) SeedOrgCatalog(ctx context.Context, tenantID string) error

// ForkEntry materializes an org-owned copy of a LINKED entry on first edit
// (§3 B4): bumps version, sets origin=FORKED, calls Bundles.Fork, keeps
// source_entry_id. UpdateOrgEntry calls this internally when the target row's
// origin is LINKED; exposed separately so an explicit "customize before
// editing" UI action can call it without submitting a diff yet.
func (s *Service) ForkEntry(ctx context.Context, tenantID, orgEntryID string) (*catalogpb.CatalogEntry, error)
```

### Proto (`protocols/cloud/v1/catalog/*.proto`, `package cloud.v1.catalog`)

```proto
enum Level { LEVEL_UNSPECIFIED = 0; LEVEL_INSTANCE = 1; LEVEL_ORG = 2; }
enum Kind  { KIND_UNSPECIFIED = 0; KIND_PROVIDER = 1; KIND_WORKFLOW = 2; }
enum Origin { ORIGIN_NATIVE = 0; ORIGIN_LINKED = 1; ORIGIN_FORKED = 2; }

message CatalogEntry {
  common.Entity entity = 1;         // id / tenant_id / name / description / author / timings
  Level level = 2;
  Kind kind = 3;
  string slug = 4;
  uint32 version = 5;
  Origin origin = 6;
  string source_entry_id = 7;
  string source_ref = 8;            // opaque, SP-C-owned
  Summary summary = 9;

  message Summary {
    // KIND_PROVIDER
    repeated string provides = 1;
    // KIND_WORKFLOW
    string provider_slug = 2;
    uint32 machine_group_count = 3;
    uint32 service_count = 4;
    bool compiles = 5;
  }
}

service CatalogService {
  // instance level — admin_only (§3 B3)
  rpc CreateInstanceEntry(CreateInstanceEntryRequest) returns (CreateInstanceEntryResponse) {
    option (cloud.v1.iam.auth) = { admin_only: true };
  }
  rpc UpdateInstanceEntry(...) returns (...) { option (cloud.v1.iam.auth) = { admin_only: true }; }
  rpc DeleteInstanceEntry(...) returns (...) { option (cloud.v1.iam.auth) = { admin_only: true }; }
  rpc GetInstanceEntry(...)    returns (...) { option (cloud.v1.iam.auth) = { admin_only: true }; }
  rpc ListInstanceEntries(...) returns (...) { option (cloud.v1.iam.auth) = { admin_only: true }; }

  // org level — RESOURCE_PROVIDER / RESOURCE_WORKFLOW, tenant_field defaults to "tenant_id"
  rpc CreateOrgEntry(...) returns (...) {
    option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_PROVIDER, action: ACTION_CREATE}] }; // per-kind at runtime
  }
  rpc UpdateOrgEntry(...) returns (...) { option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_PROVIDER, action: ACTION_UPDATE}] }; }
  rpc DeleteOrgEntry(...) returns (...) { option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_PROVIDER, action: ACTION_DELETE}] }; }
  rpc GetOrgEntry(...)    returns (...) { option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_PROVIDER, action: ACTION_READ}] }; }
  rpc ListOrgEntries(...) returns (...) { option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_PROVIDER, action: ACTION_LIST}] }; }

  rpc LinkInstanceEntry(LinkInstanceEntryRequest) returns (LinkInstanceEntryResponse) {
    option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_PROVIDER, action: ACTION_CREATE}] };
  }
  rpc CheckCatalogEntry(CheckCatalogEntryRequest) returns (CheckCatalogEntryResponse) {
    option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_PROVIDER, action: ACTION_READ}] };
  }
}
```

`RESOURCE_PROVIDER` в аннотации выше — иллюстративно; т.к. `Create/Update/Delete/Get/ListOrgEntries` работают с ОБОИМИ `kind`, точную схему (одна аннотация `all_of` с обоими ресурсами через OR — которого `all_of` не поддерживает, см. `options.proto`'s "there is intentionally no any_of" — или раздельные RPC `CreateOrgProvider`/`CreateOrgWorkflow`) фиксирует план SP-B, не эта спека: `all_of` — чистый AND, значит "работает с provider ИЛИ workflow" требует **раздельных RPC на kind**, а не общего `CreateOrgEntry(kind=...)`. Проектная модель (B1-B4) от этого не меняется — меняется только proto-нарезка (`CreateOrgProvider`/`CreateOrgWorkflow` вместо одного `CreateOrgEntry`), решить в плане.

### Postgres (queries, по образцу `iam.sql`)

```sql
-- name: CreateCatalogEntry :exec
insert into catalog_entries (id, level, tenant_id, kind, slug, version, origin, source_entry_id, created_at, updated_at, data)
values (@id, @level, @tenant_id, @kind, @slug, @version, @origin, @source_entry_id, now(), now(), @data);

-- name: GetCatalogEntry :one
select data from catalog_entries where level = @level and coalesce(tenant_id,'') = coalesce(@tenant_id,'') and id = @id;

-- name: ListCatalogEntries :many
select data from catalog_entries where level = @level and coalesce(tenant_id,'') = coalesce(@tenant_id,'') and kind = @kind;

-- name: GetLatestCatalogEntryBySlug :one
select data from catalog_entries
where level = @level and coalesce(tenant_id,'') = coalesce(@tenant_id,'') and kind = @kind and slug = @slug
order by version desc limit 1;

-- name: ListCatalogEntriesBySource :many
select data from catalog_entries where source_entry_id = @source_entry_id;
```

---

## 5. Что переиспользуем / остаётся

- **`schema.DeriveParamsSchema`** (tfvars → JSON Schema, `internal/dsl/schema`) — сигнатура не меняется; меняется источник байт `variables.tf` (из `BundleStore.Read` каталожной Provider-записи, а не из `providers/<name>/module/` внутри общего бандла рецепта).
- **`ast.DecodeCluster`/`ast.ProviderUse`/`ast.DecodeProviderManifest`** (`internal/dsl/ast`) — не меняются; `provider.use` остаётся строкой, просто резолвится как каталожный слаг, а не путь в файловом дереве бандла.
- **`internal/services/dsl.CheckBundle`/`DslService.Check`** (полный `dsl.Compile` check-режим) — переиспользуется как `Checker` для `KIND_WORKFLOW` через тот же контракт, что уже прокинут в `recipe.Deps.Checker`.
- **`iam.Resource`/`iam.Action`/`iam.Scope`/`iam.Permission`, `iam.Role`/`iam.Membership`, `Resolver.EffectivePermissions`, `Gates`, `Catalog.Grantable`** (`internal/services/iam`) — ничего не переписывается; добавляются два значения `Resource` (`RESOURCE_PROVIDER`, `RESOURCE_WORKFLOW`) и аннотации на новом `CatalogService`, всё остальное (permission-catalog reflection, gate-интерцептор, seed owner-роли при `CreateTenant`) работает без изменений.
- **`iam.Tenant`** как модель организации — переиспользуется как есть (никакой отдельной "Org"-сущности не заводится; `level=LEVEL_ORG`+`tenant_id` = существующий `iam.Tenant.Id`).
- **JSONB-блоб-с-индексированными-колонками** идиома storage-слоя (`internal/infrastructure/postgres/iam.go`, `recipe.go`) — переиспользуется 1:1 для `catalog_entries`.
- **`RecipeRecord`/`RecipeBundle`/`recipe_records`** — не удаляются в рамках SP-B (миграция существующих строк в `catalog_entries` — отдельный шаг плана, §9.4); `internal/services/recipe` продолжает работать по-старому до момента переключения.

---

## 6. Поток данных

```
АВТОРИНГ INSTANCE (admin_only):
  instance-admin ──IDE(SP-C)/CatalogService.CreateInstanceEntry──> BundleStore.Write ──> catalog_entries(level=instance)

МАТЕРИАЛИЗАЦИЯ ORG (CreateTenant seam):
  iam.CreateTenant ──(owner Role + Membership, как сегодня)──┐
                                                              ├──> catalog.SeedOrgCatalog(tenantID)
  catalog_entries(level=instance, *) ──каждая запись──> catalog_entries(level=org, tenant_id, origin=LINKED, source_entry_id)

АВТОРИНГ ORG (org-admin, RESOURCE_PROVIDER|WORKFLOW):
  UpdateOrgEntry(linked row) ──ForkEntry──> BundleStore.Fork ──> catalog_entries(level=org, origin=FORKED, version+1)
  CreateOrgEntry (native)    ──────────────> catalog_entries(level=org, origin=NATIVE)

ПРОСМОТР/ЗАПУСК (org-member, READ|LIST) — потребляется SP-D:
  ListOrgEntries(tenant_id, kind=WORKFLOW) ──> [workflow picker]
  ListOrgEntries(tenant_id, kind=PROVIDER) ──> [provider picker / auto-resolve по cluster.yaml provider.use]

КОМПИЛЯЦИЯ (launch-time, §3 B5):
  workflow entry ──BundleStore.Read──> cluster.yaml (provider.use = slug)
  slug ──CatalogEntryRepo.GetLatestBySlug(LEVEL_ORG, tenantID, KIND_PROVIDER, slug)──> provider entry
  provider entry ──BundleStore.Read──> manifest.yaml + module/*.tf ──schema.DeriveParamsSchema──> [SP-A/SP-D]
```

---

## 7. Зависимости

- **Upstream:** нет (не блокируется SP-A/SP-C — тестируется с фейковым `BundleStore`/`Checker`). Координация с SP-C только по форме `source_ref`/`BundleStore` (интерфейс, не реализация).
- **Downstream:**
  - **SP-C** — реализует `BundleStore` (git), встраивает `CatalogService` как storage/RBAC-слой под IDE (создание/редактирование файлов идёт через `Write`/`Fork`, не мимо).
  - **SP-D** — `ListOrgEntries` даёт список workflow+provider для пикера запуска; резолюция `provider.use` (§3 B5) — прямая зависимость компиляции формы от каталога.
  - **SP-F** — провайдер-креды (vision §9.3.5) вешаются на `CatalogEntry` (`kind=PROVIDER`) уровня org, а не на процесс — SP-B закладывает адресацию (`entry.id`), сам механизм хранения секретов — SP-F.

---

## 8. Тестирование

- **Repo/уникальность:** `catalog_entries` — CRUD + `(level, tenant_id, kind, slug, version)`-уникальность (конфликт → `derrors.ErrConflict`, как `recipe_records`); `GetLatestBySlug` возвращает максимальную версию.
- **RBAC:**
  - Instance-CRUD отклоняет не-`is_admin` (`admin_only` гейт), не резолвится через `all_of`/tenant.
  - Org-CRUD: org-member без `RESOURCE_PROVIDER/WORKFLOW` `CREATE/UPDATE/DELETE` — `PermissionDenied`; с `READ/LIST` — доступ к `Get/ListOrgEntries` есть, к мутациям — нет.
  - Кросс-tenant изоляция: org A не видит/не мутирует `catalog_entries` org B (как `RecipeRepo`/`TestRunRepo` уже гарантируют для своих таблиц).
  - `iam.Catalog.Grantable` (`internal/services/iam/catalog_test.go`-стиль) подхватывает `RESOURCE_PROVIDER`/`RESOURCE_WORKFLOW` из аннотаций `CatalogService` автоматически, включая синтезированный `ACTION_MANAGE`.
- **Материализация:** `SeedOrgCatalog` при `CreateTenant` — org получает `origin=LINKED`-копию каждой instance-записи; повторный вызов идемпотентен (не дублирует строки).
- **Форк-на-редактирование:** `UpdateOrgEntry` на `LINKED`-строке создаёт новую `FORKED`-версию, не мутирует instance-строку и не мутирует `LINKED`-строки других org с тем же `source_entry_id`.
- **Резолюция provider (B5):** golden-тест — `resolveProvider`-эквивалент с фейковым `CatalogEntryRepo`+`BundleStore` вместо чтения `providers/<name>/` из бандла; те же diagnostics-кейсы, что сегодня покрывает `internal/services/dsl` (нет provider.use, provider не найден в org-каталоге, невалидный manifest).
- **`CheckCatalogEntry`:** `KIND_WORKFLOW` даёт те же diagnostics, что `RecipeService.CheckRecipe` сегодня; `KIND_PROVIDER` — derive-only diagnostics (path-traversal reject, malformed tf, и т.д. — переиспользует существующие `schema.DeriveParamsSchema`-тесты).

---

## 9. Открытые вопросы

1. **Живая ссылка vs форк instance→org** (vision §9.3): при линковке (`SeedOrgCatalog`/`LinkInstanceEntry`) org-строка ссылается на instance-объект (`source_entry_id`, без своего `source_ref`) до первого редактирования — **живая ссылка** до форка. Альтернатива — копировать файлы (`BundleStore.Fork`) сразу при линковке, форк-на-редактирование становится «просто новая версия», не первое отделение.
   - *Живая*: instance-admin может докатить фикс/апдейт во все org разом (нужен `SyncOrgLinkedEntries` или аналог); риск — org незаметно получает изменившееся поведение под уже запланированными/повторяемыми прогонами; сложнее история "что именно исполнилось" для `Baked`-идентичности прогона (SP-E) — один и тот же catalog id может означать разный файловый набор в разное время, если не запинить версию.
   - *Форк сразу*: org с первого дня полностью владеет копией, прогон детерминирован по `(catalog id, version)` без дополнительной семантики; минус — апдейты/патчи instance-admin'а не долетают без явного действия org (само-сервисный "подтянуть последнюю instance-версию" — нетривиальный diff/merge UX).
   - Схема §3/§4 (`origin`+`source_entry_id`+версии) поддерживает оба варианта без переделки — решить в плане SP-B.
2. **Доступ org-admin к сырому instance-каталогу**: нужен ли `ListInstanceEntries`, читаемый НЕ только `is_admin` (чтобы org-admin выбирал, что линковать), или в v1 достаточно авто-линковки всего instance-каталога при `CreateTenant` (а `LinkInstanceEntry` — для того, что появилось в instance ПОСЛЕ создания org)? Влияет, нужен ли отдельный READ-гейт для instance-уровня помимо `admin_only`.
3. **Формат `provider.use`**: голый слаг → всегда последняя org-версия провайдера (как `GetLatestByName` для `RecipeRecord` сегодня), или нужен явный пин версии (`slug@version`) ради воспроизводимости прогона (актуальнее станет с `Baked`-снапшотом в SP-E)?
4. **Судьба `recipe_records`**: SP-B вводит `catalog_entries`, но не переносит существующие `RecipeRecord`-строки (разбиение одного бандла на Provider+Workflow-записи — не автоматическая one-to-one миграция, т.к. сегодняшний provider внутри бандла не версionирован отдельно). Разово мигрировать при переключении или крутить `recipe_records` и `catalog_entries` параллельно на переходный период — решить в плане SP-B, координируется с SP-E (когда `RecipeService`/`TestRunRecord` в принципе выпиливаются).
5. **Кто такой "instance-admin" для каталога**: сегодня единственный платформенный суперюзер — буквальный `Account.is_admin` (полный байпас всех проверок). Vision различает "instance-admin (глобальный каталог+эксплуатация)" как операционную роль — но `MethodAuth` не умеет резолвить `all_of`-permission без tenant-контекста (см. §3 B3), поэтому v1 гейтит instance-CRUD только `admin_only`. Нужна ли отдельная делегируемая (не полный `is_admin`) роль именно для каталога — и если да, то как расширять `MethodAuth`/интерцептор под tenant-less permission-резолюцию — открыто для плана SP-B (потенциально отдельный tech-debt тикет, не блокирует v1).
