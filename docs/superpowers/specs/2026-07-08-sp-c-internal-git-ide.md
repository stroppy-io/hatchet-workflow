# SP-C: Internal git + встроенная IDE

Дата: 2026-07-08. Статус: дизайн утверждён (brainstorming). Ветка: `feat/yaml-dsl-pivot`.
Часть [product-vision](2026-07-08-product-vision.md), саб-проект C (авторинг-поверхность; зависит от SP-B каталога/RBAC).

---

## 1. Что и зачем

Vision §5 фиксирует: авторинг provider/workflow-бандлов — это **internal git + встроенная IDE**, не форма и не одно-бандловый CodeMirror-редактор. Сегодня `web/src/pages/RecipeEditor.tsx` — single-bundle редактор поверх CodeMirror 6 (`web/src/components/ui/dsl-editor.tsx`): один рецепт = один "документ" (`Record<string, string>` файлов), без версионирования кроме `RecipeService.CreateRecipe`'а неявного `nextVersion` (см. `RecipeEditor.tsx` шапка-комментарий — "Save всегда создаёт НОВЫЙ id"), без веток, без ревью, без работы с несколькими провайдерами/рецептами разом.

Продукту нужно два новых слоя:
1. **Хранилище с историей** — git вместо flat `models.RecipeBundle{Files map[string][]byte}` в постгресе. Ветки/коммиты/диффы/ревью — бесплатно от git, а не переизобретаются в схеме БД.
2. **Реальная IDE** вместо textarea+CodeMirror-на-один-файл — с автодополнением по схеме, live-диагностикой компилятора, hover/goto по include/provider-ref, деревом файлов всего каталога org (не одного бандла).

SP-C даёт эту авторинг-поверхность. Она встраивается в SP-B каталог (provider/workflow как git-файлы под git-путём, метаданные каталога — отдельно в postgres, см. SP-B) и работает под RBAC SP-B (кто может коммитить в instance-репо vs org-репо).

---

## 2. Scope

**В scope:**
- Internal git-хранилище: один репозиторий на instance (глобальный каталог) + один на каждый tenant (org), инициализируемый как копия/ссылка на instance-репо.
- Git-доступ поверх RBAC: кто может читать/писать/коммитить в какой репозиторий (instance-admin → instance-репо; org-admin → своя org-репо; см. §7).
- Хостинг встроенной IDE (code-server / VSCode-в-браузере) и её проксирование через `internal/gateway` на тот же публичный порт, что и остальной продукт.
- Расширение `stroppy-yaml`: LSP-сервер поверх `DslService.Check`/`ComposedSchema` (SP-A schemapb-derivation) — автодополнение, live-диагностика, hover/goto, preview скомпилированного плана.
- Интеграция открытия IDE из каталога (SP-B): "редактировать provider/workflow X" открывает IDE на нужном git-репо/пути.

**Вне scope (не трогаем здесь):**
- Модель каталога и метаданные provider/workflow (кто на что ссылается, instance→org дефолт-ссылка/копия) — SP-B.
- RBAC-роли и их семантика (кто есть instance-admin/org-admin) — SP-B; SP-C только **потребляет** эти роли как git-ACL.
- schemapb-derivation сама (`DeriveProviderParamsSchemapb`/`DeriveInputsSchema`) и WASM-валидатор — SP-A/SP-D; SP-C только **переиспользует** уже готовый `DslService.Check`/`ComposedSchema` как источник диагностик/схемы для LSP.
- Одно-бандловый CodeMirror-редактор — не удаляется, остаётся как lite-путь (vision §5); SP-C не трогает `RecipeEditor.tsx`/`dsl-editor.tsx` кроме, возможно, ссылки "Open in IDE" (не обязательно к этому SP).

---

## 3. Компоненты

### C1. Git-стораж

Два уровня репозиториев:
- **Instance-репо** — один на весь instance, глобальный каталог: `providers/<name>/...`, `workflows/<name>/...` (структура файлов — та же, что сегодня описывает бандл: `cluster.yaml`/`workflow.yaml`/`providers/<name>/manifest.yaml`+`module/*.tf`/`components/*`, см. `internal/services/dsl/service.go` константы `clusterFile`/`providersDir`/`manifestFile`/`moduleDirName`).
- **Org-репо** — один на каждый tenant. Инициализируется на создание org как копия (или git-remote-ссылка, см. открытый вопрос §9.3 vision) instance-репо; дальше org коммитит в свой репозиторий независимо, не трогая instance.

Оба репозитория живут **на сервере** (не на клиенте) — сервер держит bare-репо на диске (или в объектном сторадже — см. §9). IDE (C2) работает с ними как с обычными git-репо (checkout/commit/push против localhost) — code-server примонтирован на тот же файловый путь, что видит bare-репо через worktree, либо ходит по git-протоколу на localhost.

История/ветки/ревью — родные git-примитивы: `git log`, `git diff`, `git branch`, стандартный PR-флоу (если понадобится — см. §9.2) поверх этого хранилища, а не самодельная версионность в постгресе (в отличие от сегодняшнего `RecipeService.CreateRecipe`'а `nextVersion`).

### C2. Встроенная IDE (code-server) + проксирование

**code-server** (или другой VSCode-in-browser) — процесс(ы) на сервере, обслуживающие редактирование git-репо изнутри браузера. Прокидывается через `internal/gateway` тем же паттерном, что `GrafanaBackend`/`RegistryBackend` сегодня (`internal/gateway/gateway.go`):

```go
// Config (аналогично существующим *Backend полям)
IdeBackend string // internal code-server base URL, reverse-proxied на /ide/*
```

`gateway.serveHTTP` получает новую ветку `case strings.HasPrefix(r.URL.Path, "/ide/"):` рядом с существующими `/grafana/*`, `/v2/*` (см. `gateway.go:162-176`) — тот же `newMonitorProxy`-конструктор (single-host reverse proxy) переиспользуется для code-server backend, как уже переиспользуется для Grafana и registry. Т.е. C2 добавляет ОДНУ новую ветку в уже существующий диспетчер, не новый сервер.

Хостинг-модель (per-user контейнер vs shared instance с воркспейсами) — открытый вопрос, см. §9.2.

### C3. Расширение `stroppy-yaml` (LSP)

VSCode-расширение (устанавливается в code-server) реализует Language Server Protocol для `.yaml`-файлов бандла (`cluster.yaml`/`workflow.yaml`/`components/*.yaml`). LSP-сервер — **тонкий клиент к уже существующему `DslService`** (не переизобретает компилятор):

- **Диагностика** (`textDocument/publishDiagnostics`): на каждое изменение документа (debounced, как сегодняшний CodeMirror-линтер в `dsl-editor.tsx` — "every ~400ms of editing quiet") дёргает `DslService.Check(files)` с полным деревом файлов текущего git-worktree, транслирует `[]*dslpb.Diagnostic` → LSP `Diagnostic[]` (та же схема line/col/severity, что уже конвертирует `toCmDiagnostic` в `dsl-editor.tsx`).
- **Автодополнение** (`textDocument/completion`): по `schemapb.Schema`, отдаваемой `DslService.ComposedSchema` (после SP-A — типизированной, не JSON Schema строкой) — поля объекта, enum-значения, obligatory-маркеры.
- **Hover/goto** (`textDocument/hover`, `textDocument/definition`): по `include:`-ссылкам (`internal/dsl/include`) и `provider.use`/`provider-ref` — прыжок в файл, на который ссылается путь, ресолвится тем же path-resolution, что `internal/dsl/include`/`internal/services/dsl/service.go resolveProvider`.
- **Preview** (custom LSP command, не стандартный метод): дёргает `DslService.Preview` и рендерит `CompiledPlan` в side-panel (webview) — то же, что сегодня делает `RecipeEditor.tsx`'s "Preview"-кнопка (`previewBundle`), но живёт в IDE, а не в отдельной странице.

LSP-сервер работает над **множеством файлов git-worktree** (не одним бандлом) — на каждый LSP-запрос он сначала определяет "к какому бандлу принадлежит открытый файл" (родительская директория с `cluster.yaml`+`workflow.yaml`), затем собирает `files map[string][]byte` из этого поддерева и зовёт `DslService`, ровно как `DslEditor`'s линтер сегодня получает пропом весь `files: Record<string, string>` бандла (`dsl-editor.tsx` `DslEditorProps.files`).

### C4. Интеграция с каталогом и RBAC (SP-B)

- Открытие "редактировать provider X" / "редактировать workflow Y" из каталога (SP-B UI) переходит в IDE (C2), открытую на git-пути, соответствующем этой каталожной записи, внутри репозитория (instance или org), которым владеет запись.
- RBAC-проверка — **на границе гейтвея/git-стораджа**, не внутри code-server: прежде чем проксировать `/ide/*` (или прежде чем открыть git-worktree), сервер проверяет, что текущий пользователь имеет право авторинга (SP-B роль instance-admin/org-admin) на целевой репозиторий; иначе 403, до того как code-server вообще увидит запрос.
- Коммиты в git несут автора (git `user.name`/`user.email` = аутентифицированный пользователь) — естественный audit trail поверх RBAC, без отдельной таблицы аудита для авторинг-действий.

---

## 4. Интерфейсы

### Git-API сервера

Сервер не обязан выставлять "свой" REST/git-API наружу напрямую — граница взаимодействия с git идёт через C2 (IDE открывает git-репо изнутри инстанса, где сервер уже держит bare-репо на диске/в объектном сторадже). Тем не менее для программного доступа (CI, `stroppy` CLI, будущий "clone локально") нужен стандартный git smart-HTTP или SSH endpoint:

```
GET/POST /git/{scope}/{name}.git/info/refs
POST     /git/{scope}/{name}.git/git-upload-pack
POST     /git/{scope}/{name}.git/git-receive-pack
```
где `scope` — `instance` или `org/<tenant-slug>`. Это стандартный git smart-HTTP протокол (что `go-git`, `gitea`, и голый `git http-backend` уже реализуют) — SP-C не проектирует новый протокол, только решает, откуда он берётся (§9.1: embed vs sidecar).

### LSP-контракт (расширение ↔ сервер)

Расширение `stroppy-yaml` говорит стандартным LSP (JSON-RPC over stdio/socket — LSP-сервер работает как sidecar-процесс рядом с code-server, не как отдельный сетевой сервис) и внутри **дёргает существующий connect/gRPC `DslService`** как обычный клиент:

```go
// DslService — уже существующий контракт (internal/services/dsl/service.go),
// LSP не добавляет новых RPC, только новых КЛИЕНТОВ этих двух:
func (s *DslService) Check(ctx, *CheckRequest) (*CheckResponse, error)
func (s *DslService) ComposedSchema(ctx, *ComposedSchemaRequest) (*ComposedSchemaResponse, error)
func (s *DslService) Preview(ctx, *PreviewRequest) (*PreviewResponse, error)
```

`CheckRequest.Files`/`ComposedSchemaRequest.Files`/`PreviewRequest.Files` — `map[string][]byte`, тот же bundle-files контракт, что сегодня шлёт `checkBundle`/`previewBundle` из `web/src/services/recipe.ts`. LSP-сервер собирает этот `files`-снапшот из текущего git-worktree на диске (не по сети — worktree уже локален относительно сервера, т.к. C2/C3 сайдкары живут рядом с git-стораджем), а не из открытых в редакторе буферов — так диагностика видит консистентное дерево, а не только несохранённый документ.

### Как extension дёргает DslService

Два варианта транспорта LSP-сервер→`DslService`, оба валидны и не блокируют друг друга:
- **In-process**: LSP-сервер — Go-процесс (переиспользует `internal/services/dsl` пакет напрямую, `CompileBundle`/`resolveProvider` и т.д., без сети) — самый дешёвый путь, раз LSP-сайдкар живёт на том же сервере.
- **connect-client**: LSP-сервер — обычный connect/gRPC клиент `DslService` по localhost (как `web`), если LSP-процесс написан не на Go или должен разделять код с браузерным клиентом.

Выбор — деталь плана SP-C (не фиксируется здесь); оба сохраняют инвариант "LSP не переизобретает диагностику, только форматирует существующую".

---

## 5. Что переиспользуем

- `internal/services/dsl.DslService.Check/ComposedSchema/Preview` — **вся** диагностика/схема/preview-логика; LSP не добавляет новых источников истины, только новый транспорт/форматирование поверх тех же трёх RPC, что уже используют `web/src/services/recipe.ts checkBundle/previewBundle` и `web/src/components/ui/dsl-editor.tsx`.
- `internal/dsl/*` (компилятор, `include`, `contract`, `diag`) — не трогаем; SP-C — чистый потребитель через `DslService`.
- `internal/gateway` — паттерн single-host reverse-proxy (`newMonitorProxy`) для `GrafanaBackend`/`RegistryBackend` — C2 добавляет `IdeBackend` тем же паттерном, ноль новой инфраструктуры проксирования.
- `web/src/components/ui/dsl-editor.tsx`'s diagnostic-конвертация (`toCmDiagnostic`, line/col-клэмпинг) — концептуальный эталон для LSP-стороны (`Diagnostic` → LSP `Diagnostic`), хотя сам код не шарится (CodeMirror vs LSP — разные протоколы).
- schemapb (SP-A `DeriveInputsSchema`/`DeriveProviderParamsSchemapb`, `ComposedSchema` перешедший на `schemapb.Schema`) — источник автодополнения; SP-C не переоткрывает эту деривацию.

---

## 6. Поток данных

```
АВТОРИНГ:
  пользователь открывает IDE (C2, через gateway /ide/*, RBAC-гейт C4)
    └─ code-server примонтирован на git-worktree (instance- или org-репо, C1)
         └─ расширение stroppy-yaml (C3) на каждое изменение файла:
              ├─ собирает files map[string][]byte из worktree
              ├─ DslService.Check(files)      ──> LSP-диагностика (squiggles)
              ├─ DslService.ComposedSchema(...) ──> автодополнение
              └─ DslService.Preview(files) [on-demand] ──> side-panel CompiledPlan

  пользователь коммитит (git commit, встроенный в code-server UI/терминал)
    └─ коммит попадает в org- или instance-репо (C1), автор = аутентифицированный пользователь

КАТАЛОГ (SP-B, вне SP-C):
  provider/workflow-запись каталога ссылается на git-путь (репо+ref+path)
    └─ запуск (SP-D) читает файлы ИЗ этого git-пути на момент компиляции —
       не из отдельного "хранилища бандлов"; git — единственный источник истины
       для содержимого provider/workflow-файлов.
```

Важно: SP-C не вводит отдельный шаг "публикации" из git в какое-то другое хранилище — `CompileBundle`/`DslService` уже принимают `files map[string][]byte`, git-репо на любой коммит/ветку — это ровно такой снапшот файлов, читаемый напрямую (`git show <ref>:<path>` / worktree read), без промежуточной копии.

---

## 7. Зависимости

- **Upstream: SP-B** (Catalog + tenancy + RBAC). SP-C не может определить "какой репозиторий чей" и "кто имеет право коммитить" без каталожной модели tenant→org-репо и ролей instance-admin/org-admin. Vision §7 явно фиксирует: `SP-C зависит от SP-B`.
- **Upstream (мягко): SP-A** (schemapb-слой) — LSP-автодополнение (C3) полноценно работает только когда `ComposedSchema` отдаёт `schemapb.Schema` вместо `SchemaJson string`; до SP-A автодополнение может временно работать по JSON Schema (деградированный режим), диагностика (`Check`/`Preview`) не зависит от SP-A вовсе.
- **Даёт:** авторинг-поверхность продукта (vision §7) — ничего дальше по цепочке не блокирует напрямую (SP-D/SP-E не читают IDE), но заменяет собой одно-бандловый редактор как целевой UX авторинга (vision §5).

---

## 8. Тестирование

- C1: git-стораж — создание instance-репо, инициализация org-репо (копия/ссылка), коммит/чтение файла по пути и ревизии, конкурентные коммиты в разные org-репо не блокируют друг друга.
- C2: gateway-проксирование `/ide/*` — маршрутизация как у существующих `monitorproxy_test.go`/аналогов для Grafana/registry (та же тестовая форма: httptest-backend + gateway routing assertions).
- C3 LSP: golden-тесты "документ + курсор → ожидаемые completion items" и "документ с ошибкой → ожидаемый Diagnostic[]" — переиспользуя фикстуры `internal/services/dsl/service_test.go` (те же бандлы, что уже гоняются через `Check`/`ComposedSchema`) как источник ожидаемых диагностик, только на LSP-транспорте.
- C4 RBAC: org-member без авторинг-роли не может открыть `/ide/*` на чужом/instance-репо (403 до code-server); org-admin может писать только в свой org-репо, не в instance или чужую org.
- E2E (после SP-B готова): открыть IDE → отредактировать `workflow.yaml` → увидеть live-диагностику → закоммитить → каталог видит новую версию на HEAD org-репо.

---

## 9. Открытые вопросы

> **РЕШЕНО владельцем (2026-07-08):**
> - **Git-бэкенд = Gitea sidecar** (§9.1 → gitea). RBAC SP-B — источник правды; gateway проверяет права ДО проксирования в gitea (не полагаемся на gitea-ACL).
> - **IDE-хостинг = shared per-org code-server** (§9.2 → per-org): один code-server на org, воркспейс = git-репо org.
> - Инициализация org-репо согласована с SP-B «живая ссылка + fork-on-edit» (org-репо стартует как ссылка на instance, форк-на-правку).

### 9.1 Git-бэкенд: встроенный vs внешний

- **Встроенный** (`go-git` + bare-репо на диске сервера, git smart-HTTP руками поверх `internal/gateway`): ноль новых процессов/сервисов в деплое, полный контроль над ACL-проверкой на границе (переиспользует существующий auth-стек), но сервер сам реализует git-протокол (smart-HTTP upload-pack/receive-pack) и вопросы вроде GC/repack, что `gitea` даёт из коробки.
- **Внешний** (gitea как sidecar-контейнер рядом с сервером, как уже есть nomad/vector/grafana sidecar-паттерн в стенде): git-протокол, веб-UI для diff/PR, ревью-флоу, GC — всё бесплатно и проверено; но новый компонент в деплое (ещё один контейнер, ещё один backend в `gateway.Config`, аналогично `GrafanaBackend`), и ACL нужно синхронизировать между RBAC SP-B и gitea-своим ACL (двойной источник правды на права), либо полностью делегировать авторизацию в gitea (org=gitea-org).

**Рекомендация:** начинать с **gitea sidecar** — самый быстрый путь к работающему git-хранилищу с ревью/diff-UI бесплатно (vision явно называет `code-server`/встроенную IDE приоритетом, не "написать свой git-сервер"); прокидывается через gateway тем же `newMonitorProxy`-паттерном, что уже используется для трёх других sidecar-бэкендов. ACL синхронизируется односторонне: RBAC SP-B — источник правды, гейтвей проверяет права ДО проксирования в gitea (как §3 C4 уже описывает), а не полагается на gitea-собственный ACL. Встроенный `go-git`-бэкенд остаётся опцией для self-host-минимализма (меньше контейнеров) — не исключается дизайном, просто не первый выбор.

### 9.2 IDE-хостинг: per-user контейнер vs shared instance с воркспейсами

- **Per-user контейнер**: сильная изоляция (падение/зависание одного пользователя не аффектит других, ресурсные лимиты per-container), но дороже (N пользователей = N процессов code-server, holding памяти на каждый даже при простое) и сложнее в оркестрации (spin up/down по активности, вопрос "куда монтировать git-worktree" per-контейнер).
- **Shared instance с воркспейсами**: один (или несколько per-tenant) code-server процесс(а), воркспейсы = поддиректории с git-worktree'ами; дешевле в ресурсах, но требует своей изоляции воркспейсов внутри процесса (файловые права, вопрос "может ли пользователь A code-server'а увидеть файлы пользователя B через тот же процесс") и общий blast radius при сбое code-server-процесса.

**Рекомендация:** начинать с **shared instance per-tenant** (один code-server-процесс/под на org, не на пользователя) — компромисс: изоляция на границе, которая УЖЕ существует в модели (org — единица RBAC и git-репозитория, §3 C1), без изоляции на пользователя внутри org (та же org уже доверяет друг другу через общий git-репо). Per-user контейнеры — следующий шаг, если конкурентное редактирование одного org несколькими пользователями одновременно окажется узким местом (конфликт открытых портов/процессов внутри shared code-server), не блокер для v1.

### 9.3 Наследование от vision §9.3 (дефолт-ссылка instance→org)

Живая ссылка vs форк org-репо от instance-репо — решается в SP-B, но напрямую определяет C1: живая ссылка технически — либо git submodule/subtree, либо read-through fallback (org-репо не содержит файл → сервер читает из instance-репо), форк — обычный `git clone`. SP-C-план должен дождаться решения SP-B здесь, прежде чем фиксировать точный механизм инициализации org-репо.
