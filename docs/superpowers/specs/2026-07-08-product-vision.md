# Stroppy Cloud — Product Vision (North Star)

Дата: 2026-07-08. Статус: дизайн утверждён владельцем (brainstorming-сессия). Ветка: `feat/yaml-dsl-pivot`.

Это **целевое описание продукта** — то, к чему стримимся. Не план реализации. Разбито на саб-проекты (§7); каждый саб-проект получает свою `spec → plan → implementation` отдельно. Предыдущие спеки ([пивот](2026-07-03-yaml-dsl-pivot-design.md), [flow-wiring](2026-07-03-flow-wiring-design.md)) описывали ДВИЖОК; эта — описывает ПРОДУКТ поверх движка.

---

## 1. Проблема

Сегодня на ветке есть **движок** (компилятор YAML→`CompiledPlan` + Temporal-исполнитель + docker/nomad-провайдер, живой e2e на стенде) и **демо-UI** (recipe list/editor/runs, переиспользующий старую модель `TestRunRecord`). Но нет **продуктовой модели**: провайдеры, рецепты, прогоны, организации и квоты не складываются в одну историю.

Симптомы (заданы владельцем):
- **«Кастомные провайдеры — где всё это?»** Провайдер живёт файлами внутри бандла рецепта (`providers/<name>/manifest.yaml` + tf-модуль). Нет реестра, версионирования, шаринга, org-уровня — «кастомизация» = вклеить tf-модуль в свой рецепт.
- **«Почему таблица runs кардинально изменена?»** Её не переписали — переиспользовали старую `test_run_records` (`TestRunRecord` из до-пивотной демки). Recipe-раны оставляют половину полей пустыми/дерайвят из `CompiledPlan` → ощущение bolted-on.
- Итог: «переложенная демка», а не цельный продукт.

**Цель этой спеки** — зафиксировать цельную продуктовую модель, чтобы дальнейшая работа собирала продукт, а не докручивала демку.

---

## 2. Продукт (одним абзацем)

**Stroppy Cloud** — self-hostable мультитенант-платформа нагрузочного тестирования, где **организации авторят свои infra-провайдеры и benchmark-workflow как код** (в встроенной IDE поверх internal git), а конечные пользователи запускают прогоны через **форму, авто-генерируемую из workflow + provider**. Всё остальное (список прогонов, overview, метрики) — фиксированный UI продукта с полнотой данных не ниже ref-ветки.

Ключевой принцип: **вся кастомизация — на уровне администрирования (instance + org), из неё генерируется ровно одна вещь — форма запуска workflow.** Пользователь не авторит, он запускает.

---

## 3. Доменная модель

Четыре первоклассных сущности + тенанси/RBAC.

### 3.1 Provider

Параметризуемая infra-цель. Два вида:
- **builtin `docker`** — single-node docker + nomad-sidecar (уже живой), без tf-модуля.
- **tf-module** — HCL-модуль (напр. `yandex`), контракт `stroppy_nodes`/`stroppy_machines` (in/out), провижинится terraform-actor'ом.

Провайдер отдаёт **`schemapb.Schema`** — описание своих параметров, выведенное из `variables.tf` (+ `manifest.yaml` для UI-метадаты/capabilities). Эта схема:
- валидирует params при компиляции (сервер, Go);
- драйвит поля формы запуска (браузер, WASM) — та же схема, тот же движок, ноль дрейфа.

Провайдер — first-class объект каталога (§3.3), живёт на instance- и org-уровне.

### 3.2 Workflow (recipe)

Декларативная единица «что и как прогнать». Бандл файлов (internal git, §5):
- **`cluster.yaml`** — ЧТО: машины (группы), сервисы (топология БД), ссылка на `provider`.
- **`workflow.yaml`** — КАК: DAG джобов (deploy → bench → teardown) со `steps`/`needs`/`matrix`/`when`, + **`inputs:`** — типизированные (`schemapb`) параметры, которые заполняет end-user при запуске.
- `components/*` — переиспользуемые фрагменты (include).

**Контракт генерации формы:** workflow декларирует через `inputs`, что он умеет варьировать; из его ссылки на `cluster` + `provider` **дотягиваются доп. параметры** (params провайдера, размерности matrix). Итоговая форма = `workflow.inputs` ⊕ `provider.params-schema` (в рамках того, что org разрешила показать). См. §4.

Workflow — first-class объект каталога, instance- и org-уровень.

### 3.3 Catalog + тенанси

Иерархия: **instance → organizations (тенанты) → users**.

- Instance-admin и org **оперируют одинаковыми сущностями** (provider, workflow).
- Что создал instance-admin — **доступно всем org** как дефолт-ссылка. У каждой org по умолчанию есть **ссылка/локальная копия** instance-каталога, которую org может кастомизировать под себя (не затрагивая других).
- Org авторит и свои локальные сущности.
- **Форма запуска генерится из ОРГ-каталога** — пользователь видит ровно то, что его org включила/сконфигурировала.

Оба уровня авторят одинаково (§5); разница — область видимости (instance-глобально vs org-локально) и RBAC.

### 3.4 Run

Первоклассная сущность (не переиспользование `TestRunRecord`). Несёт:
- **`Baked`-снапшот** (`schemapb.Baked`: схема + разрешённые значения inputs, хешируемый) — идентичность и воспроизводимость прогона;
- **`CompiledPlan`-снапшот** (результат компиляции бандла с подставленными inputs);
- **topology** (проекция машин/сервисов);
- **per-job status** (DAG-прогресс из Temporal);
- **рефы на метрики/логи** (VictoriaMetrics/Logs, Grafana);
- **rating / compare** метаданные.

**Инвариант data-parity:** всё, что рисуется по прогону (runs-таблица, overview, metrics, logs, grafana, compare, rating), **не уступает ref-ветке**. Модель можно менять — полнота отображения не должна проседать. Runs-таблица и overview — **фиксированный UI продукта** (не генерируются).

### 3.5 RBAC

Полноценный RBAC (как в ref-ветке). Роли гейтят:
- **авторинг** (IDE/каталог: create/edit provider+workflow) — instance-admin (глобально), org-admin (в своей org);
- **запуск** (форма) + **просмотр** (runs) — org-member/end-user;
- операции (квоты, креды провайдера) — org-admin; развёртывание/апдейты — instance-admin.

---

## 4. Генерация формы запуска (единственное, что генерится)

Поток:

1. Пользователь выбирает workflow из org-каталога.
2. Сервер собирает **`schemapb.Schema` формы** = `workflow.inputs` ⊕ параметры, дотянутые из `cluster`+`provider` (params провайдера, размерности `matrix`), с учётом org-ограничений (что видно/дефолты/лимиты).
3. Схема отдаётся в браузер; **schemapb, скомпилированный в WASM, валидирует ввод локально идентично Go-серверу** (CEL-правила, условные поля `when`, динамические enum-опции, computed-поля) — ноль дрейфа.
4. Пользователь заполняет → `schemapb.Filled`.
5. Сервер запечатывает → `schemapb.Baked` (хешируемый снапшот), подставляет значения в бандл, компилирует `CompiledPlan`, запускает `RunRecipeWorkflow`.
6. `Baked` сохраняется в Run (§3.4) — идентичность/воспроизводимость.

**Фикс vs генерируемое:** генерируется ТОЛЬКО форма запуска. Навигация, runs-таблица, overview, метрики — фиксированный UI.

---

## 5. Авторинг: internal git + встроенная IDE

- **Internal git** — сервер держит git-репозитории: один на instance (глобальный каталог) и один на каждую org (её каталог, инициализируется как копия/ссылка на instance). Provider- и workflow-бандлы — файлы в этих репо. История, ветки, ревью — родные git-примитивы.
- **Встроенная IDE** — реальный редактор (напр. **code-server / VSCode в браузере**) + **плагин `stroppy-yaml`**:
  - LSP поверх **schemapb + DSL-компилятора**: автодополнение по схеме, live-диагностика (те же диагностики, что `DslService.Check`), hover/goto по include/provider-ref, preview скомпилированного плана.
  - Работает с internal git репо org/instance (по RBAC).
- Org кастомизирует свою копию в той же IDE; изменения — коммиты в её git.

Это заменяет текущий одно-бандловый CodeMirror-редактор. CodeMirror-редактор может остаться как «lite»-путь, но целевой авторинг — полноценная IDE.

---

## 6. Что переиспользуем / выкидываем

**Переиспользуем (движок готов, e2e-живой):**
- Компилятор `internal/dsl/*` (YAML→`CompiledPlan`, schemapb-schema derivation, CEL, include, contract).
- Исполнитель `internal/workflows` (`RunRecipeWorkflow`, `ExecuteCompiledPlanWorkflow`, DAG-интерпретатор), провайдеры `internal/infrastructure/provider` (docker живой, terraform), агент+nomad.
- Observability (VictoriaMetrics/Logs, Grafana relay), gateway, connect/ogen/graphql-слой, React web, komeet-postgres, schemapb (v1.4.4, уже зависимость).

**Выкидываем / переписываем:**
- `TestRunRecord` как модель прогона → новая Run-сущность (§3.4).
- Flat `RecipeBundle` в `recipe_records` как единственное хранилище → internal git + catalog-модель (provider/workflow как объекты каталога, instance/org-уровни).
- Одно-бандловый редактор как целевой авторинг → встроенная IDE.
- Отсутствие provider-реестра → provider как first-class объект каталога.

---

## 7. Декомпозиция на саб-проекты

Порядок отражает зависимости. Каждый SP — отдельная `spec → plan`.

### SP-A. Schema-слой (schemapb)
Провайдер-params `schemapb.Schema` из `variables.tf`/manifest; workflow `inputs` как `schemapb`; контракт `Filled`/`Baked`; derivation-правила `workflow.inputs ⊕ provider.params → form schema`. Фундамент для формы (SP-D) и Run-идентичности (SP-E).
**Зависит от:** —. **Даёт:** SP-D, SP-E.

### SP-B. Catalog + tenancy + RBAC
Provider/workflow как объекты каталога; instance- и org-уровни; дефолт-ссылка instance→org + локальная копия/кастомизация; RBAC-гейты авторинга/запуска/просмотра. Модель хранения (git-бэкенд, см. SP-C) + метаданные каталога в postgres.
**Зависит от:** — (координируется с SP-C по стораджу). **Даёт:** SP-C, SP-D.

### SP-C. Internal git + встроенная IDE
Git-репо на instance/org; сервинг code-server (VSCode); плагин `stroppy-yaml` (LSP на schemapb + компилятор); интеграция с каталогом (SP-B) и RBAC.
**Зависит от:** SP-B. **Даёт:** авторинг-поверхность.

### SP-D. Генерируемая форма запуска
Сборка form-schema (SP-A) из выбранного workflow+provider org-каталога (SP-B); schemapb→WASM в web; `Filled→Baked→CompiledPlan→run`.
**Зависит от:** SP-A, SP-B. **Даёт:** запуск ранов конечным юзером.

### SP-E. Run-модель rewrite
Новая Run-сущность (`Baked` + `CompiledPlan` + topology + per-job status + metrics/logs refs + rating/compare); миграция читателей overview/table/metrics/logs/grafana/compare на неё; **инвариант data-parity ≥ ref**. Выпил `TestRunRecord`.
**Зависит от:** SP-A. **Даёт:** фикс-UI прогона.

### SP-F. Execution shape-up
Дошлифовка живого движка под новую модель: провайдер-креды на уровне org (не process-wide), per-run логи, terraform-провайдер на реальном стенде, execute-service-job wait-for-healthy (сейчас fire-and-forget). Закрытие deploy-gated carryover'ов (per-run log capture, per-tenant provider creds, nomad multi-node placement).
**Зависит от:** SP-B (org-креды), SP-E (run-логи). **Даёт:** прод-готовый исполнитель.

Грубый порядок: **SP-A → (SP-B ∥ SP-E) → (SP-C ∥ SP-D) → SP-F**.

---

## 8. Non-goals (v1)

- Marketplace/публичный шаринг рецептов между org (каталог — внутри instance).
- Не-benchmark юзкейсы (продукт заточен под нагрузочное тестирование; расширяемость провайдерами — да, но north-star — benchmarking).
- Мульти-облачный оркестратор помимо того, что даёт tf-провайдер.
- Автоматический workload-sizing по CEL (явный YAML достаточен для v1 — как в текущем движке).

---

## 9. Открытые вопросы (уточняются при планировании SP)

**РЕШЕНО (2026-07-08):**
1. ~~git-бэкенд~~ → **Gitea sidecar** (SP-C).
2. ~~IDE-хостинг~~ → **shared per-org code-server** (SP-C).
3. ~~дефолт-ссылка~~ → **живая ссылка + fork-on-edit** (LINKED→FORKED, SP-B).
6. **provider.use** → **пиннинг версии** (`name@ver`, SP-B/SP-E) — воспроизводимость Run.
7. **variables.tf → schemapb** → **богато** (типы+validation→CEL-rules+описания+secret+default, SP-A).

**Ещё открыто:**
5. **Провайдер-креды per-org:** где хранятся секреты (новый namespace в identity_secrets) и как прокидываются в terraform без утечки в историю Temporal (резолв внутри активности) — детали в SP-F.

---

## 10. Связанное

- Движок YAML-DSL: [пивот](2026-07-03-yaml-dsl-pivot-design.md), [flow-wiring](2026-07-03-flow-wiring-design.md).
- Текущее состояние движка: живой e2e docker recipe-run на dev-стенде — GREEN (compile→provision→execute(nomad)→teardown→COMPLETED).
- schemapb: `github.com/stroppy-io/schemapb` (v1.4.4, уже зависимость; наш, расширяемый).
