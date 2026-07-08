# SP-A: Schema-слой на schemapb (form-facing)

Дата: 2026-07-08. Статус: дизайн утверждён (brainstorming). Ветка: `feat/yaml-dsl-pivot`.
Часть [product-vision](2026-07-08-product-vision.md), саб-проект A (фундамент для SP-D форма и SP-E run).

---

## 1. Что и зачем

Форма запуска (product-vision §4) должна строиться из `workflow.inputs ⊕ provider.params` и валидироваться **идентично в браузере и на сервере**. Сегодня схема — JSON Schema (`santhosh-tekuri/jsonschema/v6`): `internal/dsl/schema/tfvars.go` (`variables.tf`→JSON Schema), `core.schema.json`, `compose.go`, `ComposedSchema` RPC (`SchemaJson string`). JSON Schema не даёт WASM-parity-движка, `Filled`/`Baked` идентичности и CEL-правил из коробки.

`schemapb` (наш, v1.4.4) даёт всё это: builder (`NewSchema/Object/Enum/List/Ref/Rule`), `Schema.Bake(values)→Baked` (хешируемый снапшот), `Filled.Bake()`, `Compute` (CEL/computed/when), Go-валидатор + WASM-энтрипоинт (`wasm/main.go`).

**SP-A переводит form-facing схему на schemapb.** Валидация СТРУКТУРЫ бандла (корректен ли `cluster.yaml`/`workflow.yaml` синтаксически) остаётся на `jsonschema/v6` — это внутренний compile-time концерн, работает, не касается формы (решение владельца: «только form-facing»).

---

## 2. Scope

**В scope:**
- `provider.params` + `provider.ext`: `variables.tf`(+`manifest.yaml`) → `schemapb.Schema` (богато: типы + validation-блоки + описания + default + sensitive→secret).
- `workflow.inputs`: `ast.InputSpec` → `schemapb.Schema` (типизированные inputs, не string-скаляры).
- **Form-schema композиция**: `form = compose(workflow.inputs, provider.params)` → `schemapb.Schema`.
- **Filled/Baked контракт**: форма отдаёт `schemapb.Filled` → сервер `Bake`-ит → `schemapb.Baked` (хешируемый) — идентичность прогона.
- `ComposedSchema` RPC: отдаёт `schemapb.Schema` (не JSON-строку).

**Вне scope (не трогаем):**
- Структурная валидация бандла (`core.schema.json`, `compose.go` bundle-часть, `validate.go`) — остаётся jsonschema/v6.
- WASM-хостинг в web (SP-D), хранение `Baked` в Run (SP-E), IDE-диагностика (SP-C).
- Подстановка Baked-значений в бандл → `CompiledPlan` (граничит с компилятором; SP-A даёт значения, интеграцию добивает SP-D — см. §7).

---

## 3. Компоненты

### A1. `variables.tf` → `schemapb.Schema` (provider params)
Заменяет `schema.DeriveParamsSchema(moduleDir) (params, ext map[string]any)` на schemapb-выход.

Маппинг (богатый):
- Terraform type-constraint → schemapb kind: `string`→String, `number`→Int64/Double, `bool`→Bool, `list(T)`→List, `object({...})`→Object, `optional(T, default)`→необязательное поле + default. Неизвестное → пермиссивный fallback (сохранить контракт tfvars.go «best-effort, не падать на одной переменной»).
- HCL `validation { condition, error_message }` → `schemapb.Rule(expr, msg)` (условие — CEL-переложение; где не переложить автоматически, пропустить с диагностикой, не падать).
- `description` → schemapb title/description (UI).
- `default` → schemapb default.
- `sensitive = true` → schemapb secret-флаг.
- `machineExtVarName` (well-known) → отдельная `ext`-схема (как сейчас), но schemapb.

Пакет: `internal/dsl/schema` расширяется, функция `DeriveProviderParamsSchemapb(moduleDir, providerName string) (params, ext *schemapb.Schema, diags diag.List)`. (`providerName` — для namespace схемы `stroppy.provider.<name>`.)

### A2. `workflow.inputs` → `schemapb.Schema`
`ast.InputSpec{Type, Default, ...}` → schemapb field defs. Типы inputs: enum/number/version/bool/string/list/matrix-размерность. Заменяет «scalar inputs = strings» (снимает I1-ограничение для формы: типы сохраняются до Bake).

**Предусловие (обнаружено при разведке SP-D):** сегодня топ-левел `inputs:` есть только у `ast.ComponentDoc`, но НЕ у `ast.WorkflowDoc` — а форма запускается по workflow. Нужно добавить `Inputs map[string]InputSpec` в `ast.WorkflowDoc` + декодер `inputs:` в `workflow.yaml` (первый шаг Task 2 плана). Поэтому сигнатура берёт готовую map, а не сам `wf`, чтобы не завязываться на структуру:

Функция `DeriveInputsSchema(namespace string, inputs map[string]ast.InputSpec) (*schemapb.Schema, diag.List)`.

### A3. Form-schema композиция
`form = ComposeFormSchema(inputs, params *schemapb.Schema) *schemapb.Schema` — через `schemapb` compose/merge (см. `schemapb/compose.go`), либо сборкой единой `NewSchema().Fields(inputsFields..., Object("provider", paramsFields...))`. Точный вызов добивает план SP-A.

Учёт org-ограничений (что видно/дефолты/лимиты) — это SP-B/SP-D применяет ПОВЕРХ базовой form-схемы (overlay). SP-A даёт базовую схему из workflow+provider; overlay-механику определяет SP-B.

### A4. Filled/Baked контракт
- Форма (SP-D, браузер) собирает `schemapb.Filled` (schema-ref + значения).
- Сервер: `Filled.Bake() → (*Baked, []*FieldError)` — валидирует (те же CEL/when/constraints, что WASM в браузере) и запечатывает.
- `Baked` — хешируемый (`hashpb`), это **идентичность/воспроизводимость прогона** → сохраняется в Run (SP-E), подставляется в бандл для компиляции (§7).
- Ошибки — `[]*FieldError` (стабильные машинные коды для i18n).

Функция `BakeForm(form *schemapb.Schema, values map[string]any) (*schemapb.Baked, []*schemapb.FieldError, error)` (сервер бейкает значения из `Filled`, собранного браузером; см. план Task 4).

---

## 4. Публичные интерфейсы (Go)

```go
// A1
func DeriveProviderParamsSchemapb(moduleDir, providerName string) (params, ext *schemapb.Schema, diags diag.List)
// A2  (требует нового ast.WorkflowDoc.Inputs — см. §A2 предусловие)
func DeriveInputsSchema(namespace string, inputs map[string]ast.InputSpec) (*schemapb.Schema, diag.List)
// A3
func ComposeFormSchema(namespace string, inputs, params *schemapb.Schema) (*schemapb.Schema, error)
// A4
func BakeForm(form *schemapb.Schema, values map[string]any) (*schemapb.Baked, []*schemapb.FieldError, error)
```

RPC: `DslService.ComposedSchema` меняет ответ с `SchemaJson string` на `schemapb.Schema` (proto-месседж). Новый (или расширенный) путь для Bake — вероятнее в StartRun-потоке (SP-D), но контракт `BakeForm` живёт здесь.

---

## 5. Что остаётся на jsonschema/v6

Структурная валидация бандла: `core.schema.json` (форма cluster/workflow), `schema.Compose`/`CompileComposed`/`validate.go` в bundle-режиме, диагностики компилятора. Не трогаем — работает, внутренний концерн, не form-facing. (Полная миграция core-схемы — потенциальный поздний SP, помечено в vision §7.)

---

## 6. Поток данных

```
АВТОРИНГ (compile-time):
  bundle files ──jsonschema/v6──> структурная валидация (без изменений)

ФОРМА (launch-time):
  workflow.inputs ─A2─┐
                      ├─A3 ComposeFormSchema──> form Schema ──(SP-D)──> WASM в браузере
  provider.params ─A1─┘                                              │ Filled
                                                                     ▼
  сервер: Filled.Bake ─A4──> Baked (хеш) ──> [Run: SP-E] + [подстановка в бандл → CompiledPlan → run]
```

---

## 7. Граница с компилятором (важно)

Сегодня `CompiledJob.ResolvedInputs map[string]string` — inputs подставляются как строки. С типизированным `Baked` нужно, чтобы компилятор принимал типизированные значения inputs при подстановке (`CompiledPlan` из бандла + Baked-значений). SP-A **определяет контракт** (Baked как источник значений); саму подстановку Baked→CompiledPlan добивает SP-D (launch-поток), опираясь на существующий include/resolve компилятора. Здесь фиксируем: значения берутся из `Baked`, типы сохраняются, string-only ограничение (I1) снимается на form-стороне.

---

## 8. Тестирование

- A1: golden-тесты `variables.tf` фикстур → ожидаемая `schemapb.Schema` (типы, validation→rules, secret, default, ext).
- A2: `ast.InputSpec` фикстуры → schemapb field defs (все виды типов).
- A3: compose — поля inputs+params не конфликтуют, namespace/идентичность корректны.
- A4: Bake — валидные значения → Baked (стабильный хеш); невалидные → `FieldError` с кодами; CEL-rule из tf-validation срабатывает.
- Parity-тест: та же схема + значения → одинаковый вердикт Go-валидатор vs (позже, SP-D) WASM.

---

## 9. Зависимости

- **Upstream:** нет.
- **Downstream:** SP-D (WASM-форма + Filled→Baked→run), SP-E (Baked в Run), SP-C (schemapb-диагностика в IDE-LSP переиспользует A1/A2).

---

## 10. Открытые вопросы (для плана SP-A)

1. **HCL `validation.condition` → CEL:** HCL-выражения ≠ CEL. Насколько автоматически переложимо (простые сравнения — да; сложные — пропуск с диагностикой)? Определить границу в плане.
2. **Точный compose-вызов schemapb:** `compose.go` API (`Compose`/`Merge`) vs ручная сборка `NewSchema().Fields(...)` с вложенным `Object("provider", ...)`. Выбрать при планировании (прочитать schemapb/compose.go).
3. **`ComposedSchema` proto:** заменить `SchemaJson string` на `schemapb.Schema` — как проходит через ogen/graphql-слои (schemapb уже в proto-графе, но проверить REST/GQL-конверторы).
4. **Версионирование form-схемы:** schemapb.Schema несёт `namespace/name/version` — как назначаем (workflow-id + provider-id + версия бандла?) для стабильной идентичности Baked.
