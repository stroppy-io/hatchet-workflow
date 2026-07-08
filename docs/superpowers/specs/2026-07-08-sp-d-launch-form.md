# SP-D: Генерируемая форма запуска

Дата: 2026-07-08. Статус: дизайн утверждён (brainstorming). Ветка: `feat/yaml-dsl-pivot`.
Часть [product-vision](2026-07-08-product-vision.md), саб-проект D (product-vision §7: зависит от SP-A, SP-B; даёт запуск ранов конечным юзером). Опирается на контракт [SP-A](2026-07-08-sp-a-schema-layer.md).

---

## 1. Что и зачем

Product-vision §4 фиксирует: единственное, что генерится в продукте — форма запуска workflow (`workflow.inputs ⊕ provider.params`, с org-ограничениями сверху), а навигация/runs-таблица/overview остаются фикс-UI. SP-A даёт схем-слой (`schemapb.Schema`, `Filled`/`Baked`, функции `DeriveProviderParamsSchema`/`DeriveInputsSchema`/`ComposeFormSchema`/`BakeForm`), но **не доводит поток до браузера и до запуска рана** — это явно оставлено SP-D (SP-A §7, §9).

Сегодня разрыв в коде подтверждает масштаб задачи:
- `RecipeService.StartRun` (`internal/services/recipe/recipe.go:164-225`) принимает только `tenant_id`+`recipe_id`, берёт сырые файлы сохранённого бандла (`recipeRec.GetBundle().GetFiles()`) и запускает `s.d.Workflows.LaunchRecipeRun(ctx, run, files)` — **нет вообще никакого места для значений inputs**.
- `DslService.ComposedSchema` (`internal/services/dsl/service.go:118-148`) уже существует, но отдаёт **draft-2020-12 JSON Schema** (`schema_json string`, движок `santhosh-tekuri/jsonschema/v6` + `internal/dsl/schema/core.schema.json`) — это структурная схема бандла для линтера редактора (CodeMirror), не форма ввода и не `schemapb`. Фронтенд (`web/src/services/recipe.ts:234-238`) её вызывает (`composedSchema`), но ни одна страница сегодня эту функцию не использует — форма запуска greenfield.
- `CompiledJob.ResolvedInputs map[string]string` (`protocols/cloud/v1/dsl/compiled.proto:74-80`) — inputs всегда строки. Точка стрингификации — `jobInputFields` в `internal/dsl/lower/lower.go:280-309` (`resolvedInputs[inputName] = fmt.Sprint(val)`), при том что типизированное значение (`BoundComponent.Inputs map[string]any`, `internal/dsl/include/resolve.go:56-60`) в этот момент ещё живо. Потребитель — `baseJobVars` в `internal/workflows/dslrun.go:299-312`, с явным комментарием-лимитацией (`dslrun.go:289-298`): CEL `when: inputs.count > 0` не работает против строкового `"3"`. Это и есть I1, которую SP-A поручает снять SP-D на form-стороне.

**Цель SP-D:** конечный пользователь (не автор) выбирает workflow из org-каталога, получает форму, сгенерированную из его `inputs` + параметров провайдера, заполняет её с локальной WASM-валидацией идентичной серверной, и одним действием запускает ран — без ручного редактирования YAML/бандла.

---

## 2. Scope

**В scope:**
- Сборка `schemapb` в WASM (`github.com/stroppy-io/schemapb@v1.4.4`, уже Go-зависимость) и хостинг артефакта в `web/`.
- Новый RPC получения form-schema для конкретного recipe/workflow (композиция через SP-A `ComposeFormSchema` + org-overlay поверх, SP-B определяет механику overlay).
- React form-renderer — компонент(ы), рендерящие произвольный `schemapb.Schema` (все kind'ы полей: String/Int64/Double/Bool/Enum/List/Object/Ref; `when`-условность, динамические enum-опции, computed-поля, list-count).
- Расширение `StartRun`: принимает `schemapb.Filled`; сервер `BakeForm`→`Baked`, подстановка значений в бандл, компиляция `CompiledPlan`, запуск `RunRecipeWorkflow` — по существующему `internal/dsl/include`+`lower` пайплайну.
- Точка снятия I1 для form-driven запуска: типизированные значения из `Baked` доходят до `CompiledJob`/CEL-рантайма (а не `fmt.Sprint`-строка).
- UX: live-валидация в браузере через WASM, идентичная серверной (CEL/when/computed/enum-options), ошибки полей.

**Вне scope:**
- Навигация/runs-таблица/overview/metrics — фикс-UI продукта (product-vision §4), не трогаем.
- Каталог/тенанси/RBAC и сама механика org-overlay — SP-B. SP-D только **применяет** overlay поверх базовой form-схемы.
- Internal git + встроенная IDE авторинга workflow/provider — SP-C.
- Полная Run-сущность и хранение `Baked` в ней — SP-E. SP-D **производит** `Baked` и передаёт дальше, не хранит.
- Сами derivation-функции (`DeriveProviderParamsSchema`/`DeriveInputsSchema`/`ComposeFormSchema`/`BakeForm`) — определены в SP-A; SP-D их вызывает, не переопределяет.
- Снятие string-only ограничения для НЕ-form путей запуска (если такие появятся) — только form-driven launch попадает в scope.

---

## 3. Компоненты

### D1. WASM-сборка schemapb + хостинг в web

`schemapb` уже несёт `wasm/main.go` (build tag `js && wasm`) и Makefile-таргет в своём модуле:
```makefile
wasm:
	GOOS=js GOARCH=wasm go build -o packages/schemapb/schemapb.wasm ./wasm
	cp -f "$(go env GOROOT)/lib/wasm/wasm_exec.js" packages/schemapb/wasm_exec.js
```
JS-контракт (`js.Global().Set`, JSON-in/JSON-out, `wasm/main.go:34-41`): `schemapbValidate`, `schemapbCompute`, `schemapbBake`, `schemapbMerge`, `schemapbLink`, `schemapbFieldActive`, `schemapbEnumOptions`, `schemapbListCount`. На некорректный вход каждая функция возвращает `{"error":"..."}`, не бросает исключение.

Модуль уже публикует npm-обвязку внутри себя: `packages/schemapb/` (`schemapb.ts` — загрузчик `wasm_exec.js`+`.wasm`, кэширующий движок) и `packages/schemapb-react/` (`useSchemaForm.ts` — headless React-хук: `values`/`computed`/`errors`, `register(path)` в духе react-hook-form, `fieldActive`/`enumOptions`/`listCount`, `handleSubmit` вызывает `engine.bake(...)`). Решение SP-D: **подключить эти npm-пакеты как версионированные зависимости** (`@stroppy-io/schemapb`, `@stroppy-io/schemapb-react`), не переизобретать JS-обвязку вручную — согласуется с общим правилом «инструменты ставятся из реестра, не пересобираются локально».

Что нужно добавить в `stroppy-cloud`:
- Build/publish-пайплайн `schemapb.wasm`+`wasm_exec.js` как статических ассетов в `web/` (сегодня в `web/vite.config.ts` **нет** wasm-плагина — ни `vite-plugin-wasm`, ни `vite-plugin-top-level-await`; нужен один из них либо статическая раздача `?url`-импортом).
- `wasm_exec.js` обязан соответствовать версии Go, которой собран `.wasm` (копируется из `go env GOROOT` при сборке) — версию нужно закрепить/сверять в CI, т.к. Go периодически меняет `wasm_exec.js` API между релизами.
- Размер `.wasm` не документирован модулем — нужно измерить и решить: eager-load при старте web-приложения vs lazy-load только при открытии страницы формы запуска (см. §9).

### D2. React form-renderer из schemapb.Schema

Новый компонент(ы) под `web/src/components/launch-form/` (по аналогии с существующим `web/src/components/run/`), потребляющие `useSchemaForm` из D1 (или собственный тонкий слой над сырыми WASM-глобалами, если `schemapb-react`-хук не подойдёт по API).

Контекст фронтенда (`web/package.json`): React 19.1.0, `@connectrpc/connect-web@2.1.1`, Vite 6.3.1, TypeScript 5.8.3, Radix UI-примитивы, **нет `react-hook-form`/`zod`**. Решение: не добавлять отдельную форм-библиотеку/схема-валидатор — `schemapb`-движок (через WASM) сам является единственным источником истины по валидации (CEL-правила, `when`, computed, enum-options); дублировать его через zod/RHF-resolver означало бы два источника истины и дрейф, чего SP-D должен избежать по построению.

Рендерер должен покрывать все `schemapb` kind'ы полей (String/Int64/Double/Bool/Enum/List/Object/Ref), условную видимость через `fieldActive` (`when`), динамические опции через `enumOptions`, повторяемые группы через `listCount`, и live-пересчёт `computed`-полей через `compute` — на каждое изменение поля, без похода на сервер.

### D3. StartRun с Filled

Расширение `web/src/services/recipe.ts` (сегодня `startRun(tenantSlug, recipeId)` → `recipeClient.startRun({ tenantId, recipeId })`, `recipe.ts:286-291`) — добавляется параметр `filled` (JSON `schemapb.Filled`, собранный D2), передаётся в `recipeClient.startRun({ tenantId, recipeId, filled })`.

Новый вызов до этого — получение form-schema: `RecipeService.LaunchFormSchema(tenantId, recipeId)`, вызывается со страницы, где сегодня стоит кнопка запуска (`RecipeEditor.tsx:263-274`, `onRun`, гейт `canRun` на `RecipeEditor.tsx:212`) — либо на отдельной странице «Запуск», см. §9.

### D4. Сервер: Bake + подстановка + launch

`RecipeService.StartRun` (`internal/services/recipe/recipe.go:164-225`) расширяется:
1. Если запрос несёт `filled`, собрать form-schema тем же путём, что и D3-предзапрос (детерминированно, без гонки схем) → SP-A `BakeForm(form, filled) → (*Baked, []*FieldError, error)`.
2. `FieldError` непусто → `StartRunResponse{field_errors}` + `codes.InvalidArgument`, ран не создаётся (или создаётся и сразу помечается failed — решить при планировании, см. §9).
3. `Baked` валиден → подстановка типизированных значений в бандл **до** компиляции: новая функция `ApplyBakedInputs(resolved *include.Resolved, baked *schemapb.Baked) error`, оверлеящая листовые значения `Baked` в `BoundComponent.Inputs map[string]any` (`internal/dsl/include/resolve.go:56-60`) — это точка, где значения **ещё типизированы**, до `lower.go`-стрингификации.
4. Дальше — существующий путь: `lower.go` компилирует `CompiledJob`, `LaunchRecipeRun`/`RunRecipeWorkflow` запускается как сегодня.
5. `Baked` передаётся дальше для сохранения в Run (SP-E — хранение; SP-D — только производит и прокидывает).

**Снятие I1 требует решения по типизированной передаче inputs в CEL-рантайм**, а не только в `BoundComponent.Inputs`: `jobInputFields` (`lower.go:280-309`) всё ещё делает `fmt.Sprint(val)` в `CompiledJob.ResolvedInputs map[string]string`, и `baseJobVars` (`dslrun.go:299-312`) биндит эти строки в CEL `inputs.*` как есть. Чтобы `when: inputs.count > 0` действительно заработало для `count int64` из формы, нужно либо (a) сделать `resolved_inputs` типизированным на уровне `compiled.proto` (например `map<string, schemapb.Value>` или аналог), либо (b) пронести рядом карту типов (`input_kinds map<string,string>`), которую `baseJobVars` использует для перебиндинга строки в нужный Go/CEL-тип перед `inputs.*`. Точный выбор — предмет плана SP-D (см. §9), но сама точка изменения зафиксирована здесь: `protocols/cloud/v1/dsl/compiled.proto:74-80`, `internal/dsl/lower/lower.go:280-309`, `internal/workflows/dslrun.go:289-312`.

---

## 4. Публичные интерфейсы

### RPC (proto, `protocols/cloud/v1/api/recipe.proto`)

```protobuf
// Новый: отдаёт form-schema для запуска (workflow.inputs ⊕ provider.params ⊕ org-overlay)
rpc LaunchFormSchema (LaunchFormSchemaRequest) returns (LaunchFormSchemaResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
}
message LaunchFormSchemaRequest {
    string tenant_id = 1 [(validate.rules).string = {min_len: 1, max_len: 64}];
    string recipe_id = 2 [(validate.rules).string = {min_len: 1, max_len: 64}];
}
message LaunchFormSchemaResponse {
    schemapb.Schema schema = 1; // базовая ⊕ org-overlay уже применён
}

// Расширение существующего StartRun
message StartRunRequest {
    string tenant_id = 1 [(validate.rules).string = {min_len: 1, max_len: 64}];
    string recipe_id = 2 [(validate.rules).string = {min_len: 1, max_len: 64}];
    schemapb.Filled filled = 3; // опционально: заполненная форма запуска
}
message StartRunResponse {
    models.TestRunRecord run = 1;
    repeated schemapb.FieldError field_errors = 2; // непусто ⇒ Bake упал, run не запущен
}
```

`LaunchFormSchema` намеренно **не** переиспользует `DslService.ComposedSchema` — тот остаётся структурной JSON Schema для линтера бандла (SP-A §5, не трогаем), а `LaunchFormSchema` — новая `schemapb.Schema`-выдача, завязанная на `recipe_id`→каталожную запись→org-контекст (тенанси нужна для overlay, SP-B), поэтому логичнее живёт на `RecipeService`, а не на `DslService`.

### Go (сервер)

```go
// внутри internal/services/recipe (или internal/dsl, по месту, решить в плане)
func ApplyBakedInputs(resolved *include.Resolved, baked *schemapb.Baked) error
```

### TS (web)

```ts
// web/src/services/schemapbEngine.ts (новый)
function loadSchemapbEngine(): Promise<SchemapbEngine> // обёртка над @stroppy-io/schemapb

// web/src/services/recipe.ts (расширение)
function fetchLaunchFormSchema(tenantSlug: string, recipeId: string): Promise<Schema>
function startRun(tenantSlug: string, recipeId: string, filled?: FilledJson): Promise<string>
```

---

## 5. Что переиспользуем

- `schemapb` как Go-зависимость (`v1.4.4`, уже в `go.mod`) — WASM-сборка и npm-обвязка (`packages/schemapb`, `packages/schemapb-react`) уже внутри модуля, не пишем с нуля.
- SP-A: `DeriveProviderParamsSchema`, `DeriveInputsSchema`, `ComposeFormSchema`, `BakeForm` — SP-D только вызывает.
- `internal/dsl/include/resolve.go` — механизм `Resolve`/`BoundComponent` уже даёт типизированную точку (`Inputs map[string]any`) для подстановки `Baked`-значений; не переписываем include-движок.
- connect-web транспорт (`web/src/services/client.ts:130-131`, `recipeClient`/`dslClient`, auth-interceptor с refresh) — новый RPC и расширенный `StartRun` идут через тот же клиент.
- UX-паттерн live-валидации из `RecipeEditor.tsx` (debounced `DslService.checkBundle`, `previewBundle`) — как прецедент для дебаунса/индикации ошибок в форме, хотя сам механизм валидации там другой (jsonschema/v6 на сервере, а не WASM в браузере).
- `RecipeService.StartRun`/`LaunchRecipeRun` рантайм-путь (`recipe.go:164-225` → `s.d.Workflows.LaunchRecipeRun`) — расширяем, не заменяем.

---

## 6. Поток данных

```
АВТОРИНГ (SP-C, вне этого SP):
  workflow.yaml (inputs) + cluster.yaml (provider ref) ──git──> org-каталог (SP-B)

ЗАПУСК (SP-D, launch-time):
  1. UI: пользователь выбирает recipe/workflow из org-каталога (RecipeService, SP-B)
  2. UI → RecipeService.LaunchFormSchema(tenantId, recipeId)
       сервер: workflow.inputs ─A2 DeriveInputsSchema─┐
                                                       ├─A3 ComposeFormSchema─> base form Schema
               provider.params  ─A1 DeriveProviderParamsSchema─┘
               base form Schema ──org-overlay (SP-B, видимость/дефолты/лимиты)──> effective Schema
     ← schemapb.Schema (protojson)
  3. UI: React form-renderer (D2) + schemapb.wasm (D1) —
       schemapbValidate/Compute/FieldActive/EnumOptions/ListCount — live-валидация
       идентичная серверной (CEL/when/computed/enum-options), без похода на сервер
  4. UI: пользователь заполняет → schemapb.Filled (JSON)
  5. UI → RecipeService.StartRun(tenantId, recipeId, filled)   (D3)
  6. сервер: BakeForm(form, filled) ─A4─> Baked | []FieldError               (D4)
       ok:  ApplyBakedInputs(Resolved, Baked) → типизированные BoundComponent.Inputs
              → lower.go compile → CompiledPlan → LaunchRecipeRun/RunRecipeWorkflow
       err: ← StartRunResponse{field_errors}, codes.InvalidArgument
  7. Baked-снапшот прокидывается дальше — сохранение в Run это SP-E
```

---

## 7. Зависимости

- **Upstream:**
  - **SP-A** — `schemapb.Schema`/`Filled`/`Baked` контракт и derivation-функции (D1-D4 их напрямую вызывают). Дополнительно: SP-A предполагает `DeriveInputsSchema(wf *ast.WorkflowDoc)`, но сегодня `ast.WorkflowDoc` (`internal/dsl/ast/workflow.go:10-12`) **не имеет** топ-левел `inputs:` — `InputSpec`/`Inputs` существуют только на `ast.ComponentDoc` (`internal/dsl/ast/component.go:11-19`), привязываются через `include:`. Это реальный пререквизит (не просто функция-обёртка), который должен закрыться до того, как SP-D сможет собрать форму из чего-то кроме `provider.params` — координировать с SP-A при планировании.
  - **SP-B** — каталог (разрешение `recipe_id`→workflow/provider-ссылки в контексте org) и org-overlay механика (что видно/дефолты/лимиты). SP-D **потребляет** overlay, не проектирует его.
- **Downstream:**
  - **SP-E** — получает `Baked` для сохранения в Run как идентичность/воспроизводимость прогона.

---

## 8. Тестирование

- **Parity Go vs WASM (end-to-end)**: та же `schemapb.Schema` + те же значения → одинаковый вердикт серверного `BakeForm`/Go-валидатора и браузерного `schemapbBake`/`schemapbValidate`. Технически — прогон `.wasm`-артефакта в headless-движке (Node + `wasm_exec.js`, либо Playwright/vitest в реальном Chrome) над теми же golden-фикстурами, что SP-A уже использует для A4.
- **D1**: сборка `.wasm` воспроизводима из зафиксированной версии `schemapb`; `wasm_exec.js` соответствует версии Go, которой собран артефакт (regression-тест на несовпадение версий).
- **D2**: снапшот/рендер-тесты React form-renderer на каждый `schemapb` kind поля (String/Int64/Double/Bool/Enum/List/Object/Ref), включая `when`-условность и computed-пересчёт.
- **D3/D4**: `ApplyBakedInputs` — типизированные значения из `Baked` действительно долетают до `BoundComponent.Inputs` и (после решения по §3 D4) до CEL `inputs.*` с правильным типом, не строкой; невалидный `Filled` → `StartRunResponse.field_errors`, ран не создаётся/помечается failed (решить при планировании).
- **Интеграционный**: полный `StartRun` с заполненной формой на docker-рецепте на dev-стенде (по аналогии с уже живым e2e docker recipe-run, project_dsl_pivot) — форма реально управляет параметрами прогона, не только валидируется.
- **RBAC**: `LaunchFormSchema`/`StartRun` с `filled` уважают org-overlay (скрытые/дефолтные/лимитированные поля org-admin'ом реально применяются, а не только визуально в форме — сервер должен перепроверять, не доверять клиенту).

---

## 9. Открытые вопросы

1. **Форм-рендерер: vendor vs hand-roll.** Принять `@stroppy-io/schemapb-react` (`useSchemaForm`) как npm-зависимость (готовый headless-хук с `register`/`fieldActive`/`enumOptions`/`listCount`/`handleSubmit`→`bake`) или писать тонкий слой самим против сырых WASM-глобалов? Первое быстрее и убирает дублирование логики, но добавляет внешнюю npm-зависимость с своим циклом версионирования, привязанным к Go-модулю `schemapb` — нужно решить в плане, синхронизирован ли npm-пакет с версией Go-модуля, который уже закреплён (`v1.4.4`).
2. **WASM-доставка**: eager-load при старте `web/` vs lazy-load только на странице формы запуска; какой vite-плагин (`vite-plugin-wasm` или статический `?url`-импорт + ручной `wasm_exec.js`); версия `wasm_exec.js` должна отслеживать версию Go, которой собирается `.wasm`, — где и как это проверяется в CI.
3. **Типизированный inputs-рантайм (I1)**: точный способ пронести типы из `Baked` через `CompiledJob` до CEL (`compiled.proto` типизированный `resolved_inputs` vs параллельная карта `input_kinds`) — зафиксировать конкретный proto-diff и изменения в `lower.go`/`dslrun.go` в плане SP-D.
4. **`ast.WorkflowDoc.inputs`**: сегодня отсутствует на уровне AST (только на `ast.ComponentDoc`). Кто это добавляет — SP-A (расширение AST) или SP-D как пререквизитный подшаг? Блокирует `DeriveInputsSchema` содержательным входом за пределами `provider.params`.
5. **Где живёт `LaunchFormSchema`**: `RecipeService` (выбран здесь, т.к. нужен `recipe_id`→каталог+org-контекст) vs расширение `DslService` — подтвердить при ревью SP-B, поскольку SP-B владеет разрешением каталожных ссылок.
6. **Org-overlay представление**: SP-B ещё не зафиксировала структуру (allow/deny-список полей? override-дефолтов? лимиты диапазонов?) — SP-D не может закрыть §3 D4 шаг «effective Schema» без этого контракта.
7. **i18n ошибок формы**: `FieldError` (SP-A A4) несёт машинные коды — где живёт таблица переводов в `web/`, кто её ведёт.
8. **`matrix`-размерности провайдера в форме**: product-vision §4 упоминает, что в форму дотягиваются «размерности `matrix`» — как они мапятся на `schemapb` List/`listCount`-семантику; нужен конкретный пример в плане SP-D.
