# DSL Compiler (сабпроект 1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Чистый Go-пакет `internal/dsl`: parse → include-resolve → schema-validate → contract-check → graph-check → lower, компилирующий cluster.yaml + workflow.yaml в proto `CompiledPlan` (DAG jobs поверх существующих `deployment.AgentStep`).

**Architecture:** Компилятор — pure-функции без I/O (вход — `map[path]bytes`), диагностики — единая модель `diag.Diagnostic`, используемая и CLI-компиляцией и будущим IDE check-endpoint. CEL-выражения `${{ }}` типизируются на компиляции (native-types envs с правилом видимости), вычисляются в рантайме исполнителем. Динамическая JSON Schema композируется из core-схемы + фрагментов, деривленных из `variables.tf` провайдеров.

**Tech Stack:** Go 1.25, `gopkg.in/yaml.v3` (strict decode), `github.com/google/cel-go` (+ `ext.NativeTypes`), `github.com/santhosh-tekuri/jsonschema/v6`, `github.com/hashicorp/terraform-config-inspect`, easyp proto toolchain (`make protocols`).

## Global Constraints

- Module: `github.com/stroppy-io/stroppy-cloud`; ветка `feat/yaml-dsl-pivot`.
- Коммиты: Conventional Commits, БЕЗ `Co-Authored-By` футеров.
- Спек: `docs/superpowers/specs/2026-07-03-yaml-dsl-pivot-design.md` — терминология и примеры YAML берутся оттуда.
- Компилятор не делает I/O: вход `Sources{Files map[string][]byte}`, никаких `os.ReadFile` внутри `internal/dsl` (исключение: `schema/tfvars.go` читает каталог tf-модуля — единственная I/O-граница, изолирована в одном файле).
- Все ошибки пользовательского YAML — через `diag.Diagnostic` (не `error`); `error` только для программных багов.
- Существующие proto не менять; новый — только `protocols/cloud/v1/dsl/compiled.proto`.
- Линт: `make lint` (golangci) должен проходить после каждой задачи.

---

### Task 1: Пакет diag + зависимости

**Files:**
- Create: `internal/dsl/diag/diag.go`
- Test: `internal/dsl/diag/diag_test.go`
- Modify: `go.mod` (добавить deps)

**Interfaces:**
- Produces: `diag.Diagnostic{Severity, Path, Pos, Message, Module string}`, `diag.Severity` (`Error|Warning`), `diag.List` с методами `Add(d Diagnostic)`, `HasErrors() bool`, `Errorf(path string, pos Pos, format string, args ...any)`; `diag.Pos{Line, Col int}`. Все последующие пакеты возвращают `(T, diag.List)`.

- [ ] **Step 1: Добавить зависимости**

```bash
go get github.com/google/cel-go@latest github.com/santhosh-tekuri/jsonschema/v6@latest github.com/hashicorp/terraform-config-inspect@latest gopkg.in/yaml.v3
```

- [ ] **Step 2: Написать падающий тест**

```go
package diag_test

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
)

func TestListCollectsAndDetectsErrors(t *testing.T) {
	var l diag.List
	if l.HasErrors() {
		t.Fatal("empty list must not have errors")
	}
	l.Add(diag.Diagnostic{Severity: diag.Warning, Path: "cluster.yaml", Message: "w"})
	if l.HasErrors() {
		t.Fatal("warning is not an error")
	}
	l.Errorf("cluster.yaml", diag.Pos{Line: 3, Col: 5}, "bad field %q", "cpu")
	if !l.HasErrors() {
		t.Fatal("expected error after Errorf")
	}
	if got := l[1].Message; got != `bad field "cpu"` {
		t.Fatalf("unexpected message: %s", got)
	}
}
```

- [ ] **Step 3: Убедиться, что тест падает**

Run: `go test ./internal/dsl/diag/ -v`
Expected: FAIL (package does not exist)

- [ ] **Step 4: Минимальная реализация**

```go
// internal/dsl/diag/diag.go
package diag

import "fmt"

type Severity int

const (
	Error Severity = iota
	Warning
)

type Pos struct{ Line, Col int }

type Diagnostic struct {
	Severity Severity
	Path     string // файл внутри бандла рецепта
	Pos      Pos
	Message  string
	Module   string // модуль-автор правила (компонент/провайдер), для contract-check
}

type List []Diagnostic

func (l *List) Add(d Diagnostic) { *l = append(*l, d) }

func (l *List) Errorf(path string, pos Pos, format string, args ...any) {
	l.Add(Diagnostic{Severity: Error, Path: path, Pos: pos, Message: fmt.Sprintf(format, args...)})
}

func (l List) HasErrors() bool {
	for _, d := range l {
		if d.Severity == Error {
			return true
		}
	}
	return false
}
```

- [ ] **Step 5: Тест зелёный + коммит**

Run: `go test ./internal/dsl/diag/ -v && go mod tidy && make lint`
Expected: PASS

```bash
git add internal/dsl/diag go.mod go.sum
git commit -m "feat(dsl): add diagnostics model and compiler deps"
```

---

### Task 2: AST cluster.yaml (strict decode)

**Files:**
- Create: `internal/dsl/ast/cluster.go`
- Test: `internal/dsl/ast/cluster_test.go`

**Interfaces:**
- Consumes: `diag` из Task 1.
- Produces:

```go
type ClusterDoc struct {
	Version  int
	Provider ProviderUse            // {Use string; Params map[string]any}
	Machines map[string]MachineGroup
	Services map[string]Service
}
type MachineGroup struct {
	Count     int
	Resources Resources // {CPU int; RAM ByteSize; Disk *Disk}
	Ext       map[string]any // содержимое блока с ключом == Provider.Use
}
type Disk struct{ Size ByteSize; Type string }
type Service struct {
	On      string
	Image   string
	Network string
	Volumes []string
	Env     map[string]string
	Configs []ConfigFile // {Template, Dest string}
	Health  *Health      // {HTTP string; Timeout Duration}
}
func DecodeCluster(path string, src []byte, providerKey string) (*ClusterDoc, diag.List)
```

- `ByteSize` — uint64 байт, парсит `32g/100m/512`; `Duration` — обёртка `time.Duration`, парсит `120s`.
- Decode strict: неизвестный ключ = диагностика (не паника). Ext-блок: ключ верхнего уровня группы, равный `providerKey`, уходит в `Ext`; другие неизвестные ключи — ошибка.

- [ ] **Step 1: Падающий тест**

```go
package ast_test

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
)

const clusterYAML = `
version: 1
provider:
  use: yandex
  params: { zone: ru-central1-a }
machines:
  db:
    count: 3
    resources: { cpu: 8, ram: 32g, disk: { size: 100g, type: ssd } }
    yandex: { platform_id: standard-v3 }
services:
  postgres:
    on: db
    image: postgres:17
    network: host
    env: { A: "${{ machines.db[0].ip }}" }
    health: { http: ":8008/health", timeout: 120s }
`

func TestDecodeCluster(t *testing.T) {
	doc, diags := ast.DecodeCluster("cluster.yaml", []byte(clusterYAML), "yandex")
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	db := doc.Machines["db"]
	if db.Count != 3 || db.Resources.CPU != 8 || db.Resources.RAM != 32<<30 {
		t.Fatalf("bad machines.db: %+v", db)
	}
	if db.Resources.Disk.Size != 100<<30 || db.Resources.Disk.Type != "ssd" {
		t.Fatalf("bad disk: %+v", db.Resources.Disk)
	}
	if db.Ext["platform_id"] != "standard-v3" {
		t.Fatalf("ext not captured: %+v", db.Ext)
	}
	if doc.Services["postgres"].On != "db" {
		t.Fatal("service.on lost")
	}
}

func TestDecodeClusterUnknownKey(t *testing.T) {
	_, diags := ast.DecodeCluster("cluster.yaml", []byte("version: 1\nmachines:\n  db: { count: 1, resurces: {} }"), "yandex")
	if !diags.HasErrors() {
		t.Fatal("typo key must produce error diagnostic")
	}
}
```

- [ ] **Step 2: Run — FAIL** (`go test ./internal/dsl/ast/ -v`)

- [ ] **Step 3: Реализация**

Декодировать через `yaml.Node` (не прямой Unmarshal) — нужны позиции для diag и перехват ext-ключа. Паттерн:

```go
// internal/dsl/ast/cluster.go (каркас; полные типы — в Interfaces выше)
package ast

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
)

type ByteSize uint64

func ParseByteSize(s string) (ByteSize, error) {
	mult := uint64(1)
	suffix := strings.ToLower(s[len(s)-1:])
	switch suffix {
	case "g":
		mult, s = 1<<30, s[:len(s)-1]
	case "m":
		mult, s = 1<<20, s[:len(s)-1]
	case "k":
		mult, s = 1<<10, s[:len(s)-1]
	}
	n, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bad size %q", s)
	}
	return ByteSize(n * mult), nil
}

type Duration time.Duration

func DecodeCluster(path string, src []byte, providerKey string) (*ClusterDoc, diag.List) {
	var diags diag.List
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		diags.Errorf(path, diag.Pos{}, "yaml parse: %v", err)
		return nil, diags
	}
	// walk mapping nodes: known keys → поля, providerKey на уровне machine group → Ext,
	// прочие неизвестные → diags.Errorf с node.Line/node.Column.
	...
	return doc, diags
}
```

Хелпер `decodeMapping(node *yaml.Node, handlers map[string]func(*yaml.Node) error, onUnknown func(key string, n *yaml.Node))` — переиспользуется в Task 3/4 для workflow/component. Вынести в `internal/dsl/ast/yamlwalk.go`.

- [ ] **Step 4: Run — PASS** (`go test ./internal/dsl/ast/ -v`)

- [ ] **Step 5: Commit**

```bash
git add internal/dsl/ast
git commit -m "feat(dsl): cluster.yaml AST with strict positional decode"
```

---

### Task 3: AST workflow.yaml

**Files:**
- Create: `internal/dsl/ast/workflow.go`
- Test: `internal/dsl/ast/workflow_test.go`

**Interfaces:**
- Consumes: `decodeMapping`, `Duration` из Task 2.
- Produces:

```go
type WorkflowDoc struct{ Jobs map[string]Job }
type Job struct {
	Needs   []string
	On      string            // machine group; пусто для service-jobs
	Service string            // ссылка на services из cluster.yaml
	With    map[string]string
	Matrix  map[string][]string
	When    string            // CEL
	Steps   []Step
}
type Step struct { // ровно один из
	Cmd       string
	WriteFile *WriteFile // {Dest, Content, Template string}
	Fetch     *Fetch     // {URL, Dest string}
	Dir       string
	Wait      *Wait      // {HTTP string; Timeout Duration}
}
func DecodeWorkflow(path string, src []byte) (*WorkflowDoc, diag.List)
```

- [ ] **Step 1: Падающий тест**

```go
func TestDecodeWorkflow(t *testing.T) {
	src := []byte(`
jobs:
  prep:
    on: db
    steps:
      - cmd: mkfs.xfs /dev/vdb
  etcd:
    needs: [prep]
    service: etcd
  bench:
    needs: [etcd]
    matrix: { workload: [insert, select] }
    service: stroppy
    with: { args: "--workload ${{ matrix.workload }}" }
`)
	doc, diags := ast.DecodeWorkflow("workflow.yaml", src)
	if diags.HasErrors() {
		t.Fatalf("diags: %+v", diags)
	}
	if doc.Jobs["bench"].Matrix["workload"][1] != "select" {
		t.Fatal("matrix lost")
	}
	if doc.Jobs["prep"].Steps[0].Cmd == "" {
		t.Fatal("cmd step lost")
	}
}

func TestDecodeWorkflowStepMustHaveOneAction(t *testing.T) {
	src := []byte("jobs:\n  a:\n    steps:\n      - { cmd: x, dir: /y }")
	_, diags := ast.DecodeWorkflow("workflow.yaml", src)
	if !diags.HasErrors() {
		t.Fatal("two actions in one step must error")
	}
}
```

- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Реализация** — тем же `decodeMapping`; правило «ровно одно действие в Step» проверяется в декодере.
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(dsl): workflow.yaml AST`

---

### Task 4: AST component.yaml + provider manifest.yaml

**Files:**
- Create: `internal/dsl/ast/component.go`, `internal/dsl/ast/provider.go`
- Test: `internal/dsl/ast/component_test.go`

**Interfaces:**
- Produces:

```go
type ComponentDoc struct {
	Inputs   map[string]InputSpec // {Type string; Default any}
	Requires []Requirement        // {Expr, Capability, Message string} — Expr XOR Capability
	Provides []string
	Cluster  *ClusterFragment     // опц.: services/machines для мержа (v1: services only)
	Jobs     map[string]Job       // фрагмент workflow
}
type ProviderManifest struct {
	Name         string
	Provides     []string
	Capabilities Capabilities // {Machine map[string]CapRule; Constraints []Requirement}
	Lowering     map[string]map[string]string // "disk.type" → {ssd: network-ssd}
}
type CapRule struct{ Enum []any; Expr string }
func DecodeComponent(path string, src []byte) (*ComponentDoc, diag.List)
func DecodeProviderManifest(path string, src []byte) (*ProviderManifest, diag.List)
```

- [ ] **Step 1: Падающий тест** — decode etcd-компонента из спеки §6 (requires expr + provides) и yandex-манифеста из спеки §6 (enum cpu, constraint local-ssd, lowering disk.type); проверить `Requires[0].Expr != ""`, `Lowering["disk.type"]["ssd"] == "network-ssd"`, а также ошибку на requirement с одновременно `expr:` и `capability:`.
- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Реализация** (тот же decodeMapping-паттерн)
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(dsl): component and provider manifest AST`

---

### Task 5: Core JSON Schema + валидатор

**Files:**
- Create: `internal/dsl/schema/core.schema.json`, `internal/dsl/schema/validate.go`
- Test: `internal/dsl/schema/validate_test.go`

**Interfaces:**
- Produces: `schema.Validate(docKind Kind, src []byte, composed *jsonschema.Schema) diag.List` (Kind: `Cluster|Workflow|Component`); `schema.MustCore() *jsonschema.Schema` — компилирует embed-схему.
- core.schema.json: draft 2020-12; `$defs` для machines/services/jobs/steps по типам из Task 2–4; `provider.params` и ext-блоки — `"$dynamicRef": "#providerParams"` / незаданный `additionalProperties` — их закрывает композер (Task 6). YAML → JSON: `yaml.Unmarshal` в `any` + нормализация ключей map[any]any → map[string]any.

- [ ] **Step 1: Падающий тест** — валидный cluster.yaml из Task 2 проходит; `count: -1` и `health: {timeout: 999}` (не строка-duration) дают diag с path.
- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Написать core.schema.json + validate.go** (santhosh v6: `jsonschema.NewCompiler()`, `AddResource`, ошибки → `diag.Diagnostic{Path: <yaml file>, Message: <InstanceLocation + причина>}`)
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(dsl): core JSON Schema and validator`

---

### Task 6: Динамическая композиция схемы + деривация из variables.tf

**Files:**
- Create: `internal/dsl/schema/tfvars.go`, `internal/dsl/schema/compose.go`
- Test: `internal/dsl/schema/compose_test.go`, testdata: `internal/dsl/schema/testdata/yandex-module/variables.tf`

**Interfaces:**
- Consumes: `MustCore()` из Task 5, `ast.ProviderManifest` из Task 4.
- Produces:

```go
// DeriveParamsSchema: variables.tf → JSON Schema fragment (map[string]any).
// Правила: string→type:string, number→number, bool→boolean, list(T)/set(T)→array,
// map(T)/object({...})→object(+properties), default→default, description→description.
// Variable "stroppy_machine_ext" (object) → отдельный фрагмент ExtSchema.
func DeriveParamsSchema(moduleDir string) (params, ext map[string]any, err error)

// Compose: core + $defs.providerParams + $defs.machineExt + $defs.includeInputs.
// Возвращает скомпилированную схему и её JSON (для отдачи IDE endpoint'ом).
func Compose(providers map[string]ProviderSchemas, fragments map[string]map[string]any) (*jsonschema.Schema, []byte, error)
type ProviderSchemas struct{ Params, Ext map[string]any }
```

- [ ] **Step 1: testdata/variables.tf**

```hcl
variable "zone" {
  type        = string
  default     = "ru-central1-a"
  description = "YC zone"
}
variable "stroppy_machine_ext" {
  type = object({
    platform_id   = optional(string, "standard-v3")
    core_fraction = optional(number, 100)
    preemptible   = optional(bool, false)
    disk_type     = optional(string)
  })
}
```

- [ ] **Step 2: Падающий тест** — `DeriveParamsSchema(testdata)` даёт `params.properties.zone.type == "string"` с default; `ext.properties.platform_id` присутствует. `Compose` с этим ext: cluster.yaml с `yandex: {platform_id: 5}` (число вместо строки) → diag error; с валидным ext → пусто; `yandex: {unknown_field: 1}` → error (additionalProperties: false в ext-фрагменте).
- [ ] **Step 3: Run — FAIL**
- [ ] **Step 4: Реализация** — `tfconfig.LoadModule(moduleDir)` (terraform-config-inspect); типы приходят строками (`"object({...})"`) — парсить примитивы/list/map/object конечным автоматом по скобкам (без полного HCL-eval; optional(T, default) → T + default). Compose — склейка map'ов + `compiler.AddResource("composed.json", ...)`.
- [ ] **Step 5: Run — PASS**
- [ ] **Step 6: Commit** `feat(dsl): dynamic schema composition from provider tf modules`

---

### Task 7: CEL-окружения с правилом видимости

**Files:**
- Create: `internal/dsl/expr/env.go`, `internal/dsl/expr/expr.go`
- Test: `internal/dsl/expr/expr_test.go`

**Interfaces:**
- Consumes: `diag`.
- Produces:

```go
// Extract возвращает CEL-выражения из строки с ${{ ... }} вставками.
func Extract(s string) []string

// View-структуры — CEL native types (ext.NativeTypes):
type MachineView struct {
	IP     string
	RAMGb  int64  `cel:"ram_gb"`
	CPU    int64
	DiskGb int64  `cel:"disk_gb"`
	Disks  []DiskView
}
type MachineGroupView struct {
	Count    int64
	Machines []MachineView
}
type ProviderMachineView struct { // домен + ext, только для провайдерских constraint'ов
	MachineView
	Ext map[string]any
}

// Envs (правило видимости из спеки §5.3):
func ComponentEnv() (*cel.Env, error) // vars: inputs (map), target (MachineGroupView), machines (map[string]MachineGroupView), machine (MachineView), matrix (map[string]string)
func ProviderEnv() (*cel.Env, error)  // vars: machine (ProviderMachineView)

// Check типизирует выражение в env; ошибка → diag (path, pos — позиция строки-хозяина).
func Check(env *cel.Env, expr, path string, pos diag.Pos) diag.List
// Eval — для contract-check на компиляции и исполнителя в рантайме.
func Eval(env *cel.Env, expr string, vars map[string]any) (any, error)
```

- [ ] **Step 1: Падающий тест**

```go
func TestVisibilityRule(t *testing.T) {
	compEnv, _ := expr.ComponentEnv()
	provEnv, _ := expr.ProviderEnv()
	// компонент видит домен
	if d := expr.Check(compEnv, "inputs.nodes.count >= 3", "c.yaml", diag.Pos{}); d.HasErrors() {
		t.Fatalf("domain expr must typecheck: %+v", d)
	}
	// компонент НЕ видит ext — ключевой негативный тест спеки §10
	if d := expr.Check(compEnv, "machine.ext.platform_id == 'x'", "c.yaml", diag.Pos{}); !d.HasErrors() {
		t.Fatal("component must not see ext")
	}
	// провайдер видит ext
	if d := expr.Check(provEnv, "machine.ext.core_fraction == 100 ? machine.cpu >= 8 : true", "m.yaml", diag.Pos{}); d.HasErrors() {
		t.Fatalf("provider must see ext: %+v", d)
	}
}

func TestExtractAndEval(t *testing.T) {
	exprs := expr.Extract(`http://${{ machines.db.machines[0].ip }}:8008/${{ matrix.workload }}`)
	if len(exprs) != 2 {
		t.Fatalf("extract: %v", exprs)
	}
	env, _ := expr.ComponentEnv()
	out, err := expr.Eval(env, "machines.db.count * 2", map[string]any{
		"machines": map[string]any{"db": expr.MachineGroupView{Count: 3}},
	})
	if err != nil || out != int64(6) {
		t.Fatalf("eval: %v %v", out, err)
	}
}
```

- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Реализация** — `cel.NewEnv(ext.NativeTypes(reflect.TypeOf(MachineView{}), ...), cel.Variable("inputs", cel.MapType(cel.StringType, cel.DynType)), ...)`. `inputs`/`matrix` — dyn-map (типы инпутов проверяет include-resolver, Task 8). Видимость обеспечивается составом env: в ComponentEnv регистрируется `MachineView` (без Ext), в ProviderEnv — `ProviderMachineView`. Extract — regexp `\$\{\{(.*?)\}\}`.
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(dsl): CEL envs with ext visibility rule`

---

### Task 8: Include-резолвер (фрагменты с inputs)

**Files:**
- Create: `internal/dsl/include/resolve.go`
- Test: `internal/dsl/include/resolve_test.go`

**Interfaces:**
- Consumes: `ast.ComponentDoc`, `ast.WorkflowDoc`, `diag`.
- Produces:

```go
type Sources struct{ Files map[string][]byte } // весь бандл рецепта

// Include-синтаксис в workflow.yaml (расширение Task 3 Job):
//   jobs:
//     ha:
//       include: components/patroni     # каталог с component.yaml
//       inputs: { nodes: db }
// Resolve разворачивает include-джобы: инстанцирует Jobs фрагмента с
// префиксом "ha/", биндит inputs (машинная группа по имени / скаляры),
// проверяет типы inputs по InputSpec, детектит циклы include.
func Resolve(cluster *ast.ClusterDoc, wf *ast.WorkflowDoc, src Sources) (*Resolved, diag.List)
type Resolved struct {
	Cluster    *ast.ClusterDoc
	Jobs       map[string]ast.Job            // плоский DAG после разворота
	Components []BoundComponent              // для contract-check
}
type BoundComponent struct {
	Name     string
	Doc      *ast.ComponentDoc
	Inputs   map[string]any // {"nodes": "db"} → группа резолвится в contract-check
}
```

- [ ] **Step 1: Падающий тест** — бандл: `components/etcd/component.yaml` (inputs.nodes: machine_group, jobs: install{on: ${{ inputs.nodes }}}), workflow с `include: components/etcd, inputs: {nodes: db}`; проверить: в `Resolved.Jobs` есть `etcd/install`, `needs` внешнего джоба на `etcd` переписан на `etcd/install`; отсутствующий input → error; `inputs: {nodes: nosuch}` → error (нет такой группы); цикл include A→B→A → error, не зависание.
- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Реализация** — рекурсивный разворот с posted-set для циклов; job-имена фрагмента префиксуются `<includeName>/`; ссылки `needs` внутри фрагмента — тоже.
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(dsl): include resolver with typed inputs`

---

### Task 9: Доменный граф + lowering-прецеденс

**Files:**
- Create: `internal/dsl/graph/domain.go`
- Test: `internal/dsl/graph/domain_test.go`

**Interfaces:**
- Consumes: `ast.ClusterDoc`, `ast.ProviderManifest`, `expr` views.
- Produces:

```go
type Domain struct {
	Groups   map[string]GroupState // name → {Spec ast.MachineGroup; View expr.MachineGroupView; LoweredDiskType string; Ext map[string]any}
	Provider *ast.ProviderManifest
}
// Build применяет lowering-прецеденс (спека §5.2):
//   домен → manifest.Lowering["disk.type"][spec] → ext override (ext["disk_type"])
// и заполняет views для CEL (IP на компиляции пустые — известны в рантайме).
func Build(cluster *ast.ClusterDoc, provider *ast.ProviderManifest) (*Domain, diag.List)
```

- [ ] **Step 1: Падающий тест** — группа `disk.type: ssd` + lowering `{ssd: network-ssd}` → `LoweredDiskType == "network-ssd"`; та же группа с ext `disk_type: network-ssd-nonreplicated` → override побеждает; `disk.type: nvme` без записи в lowering → diag error «provider yandex does not lower disk.type=nvme».
- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Реализация**
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(dsl): domain graph with lowering precedence`

---

### Task 10: Contract-checker (capabilities + CEL-предикаты)

**Files:**
- Create: `internal/dsl/contract/check.go`
- Test: `internal/dsl/contract/check_test.go`

**Interfaces:**
- Consumes: `graph.Domain`, `include.BoundComponent`, `expr.{ComponentEnv,ProviderEnv,Eval}`.
- Produces:

```go
// Check — алгоритм спеки §6:
// 1) собрать множество provides: провайдер + все компоненты;
// 2) каждый requires:capability закрыт, иначе diag(Module=имя компонента);
// 3) все requires:expr компонентов — Eval в ComponentEnv с биндингом inputs
//    (machine_group-инпуты → GroupView из Domain);
// 4) capabilities.machine (enum/expr) и capabilities.constraints провайдера —
//    Eval в ProviderEnv для каждой группы (machine = ProviderMachineView с Ext).
// false → diag с message автора правила.
func Check(dom *graph.Domain, comps []include.BoundComponent) diag.List
```

- [ ] **Step 1: Падающий тест** (кейсы из спеки §6):

```go
// 1. etcd count=2 → error "etcd: нечётный кворум ≥ 3" (Module == "etcd")
// 2. patroni requires capability kv.etcd, etcd в бандле → нет ошибки;
//    без etcd → error unresolved capability
// 3. провайдер: cpu enum [2,4,8], группа cpu=6 → error
// 4. constraint local-ssd → группа {disk lowered local-ssd, cpu 4} → error
//    с message "local-ssd только от 8 vCPU"
// 5. workload-формула: iterations=10000, row_bytes=512, disk 1g → error;
//    disk 100g → ок
```

- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Реализация**
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(dsl): contract checker (capabilities + CEL requires)`

---

### Task 11: Workflow-граф: ацикличность, ссылки, matrix-разворот, typecheck выражений

**Files:**
- Create: `internal/dsl/graph/workflow.go`
- Test: `internal/dsl/graph/workflow_test.go`

**Interfaces:**
- Consumes: `include.Resolved`, `expr.{ComponentEnv,Check,Extract}`.
- Produces:

```go
// Validate: needs-ссылки существуют; DAG ацикличен (Kahn, цикл в diag с
// перечислением участников); on: указывает на существующую группу;
// service: указывает на существующий cluster.Services; каждый ${{ }} в
// cmd/env/with/when — expr.Check в ComponentEnv.
// Expand: разворачивает matrix в инстансы "bench[workload=insert]"
// (needs внешних джобов на "bench" → на все инстансы).
func Validate(r *include.Resolved) diag.List
func Expand(r *include.Resolved) map[string]ast.Job
```

- [ ] **Step 1: Падающий тест** — цикл a→b→a детектится; `needs: [nosuch]` → error; `service: nosuch` → error; невалидный CEL в cmd (`${{ machines.db.cpu +^ }}`) → error с позицией; matrix 2×2 → 4 инстанса с проставленными `matrix` values и корректной перепиской needs.
- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Реализация**
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(dsl): workflow DAG validation and matrix expansion`

---

### Task 12: Proto CompiledPlan

**Files:**
- Create: `protocols/cloud/v1/dsl/compiled.proto`
- Generated: `internal/proto/cloud/v1/dsl/*` (через `make protocols`)

**Interfaces:**
- Produces (Go: `dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"`):

```proto
syntax = "proto3";

package cloud.v1.dsl;

import "cloud/v1/deployment/plan.proto";

// CompiledPlan — результат компиляции DSL-бандла: запрос машин провайдеру,
// сервисы (Nomad jobs) и DAG джобов. Строковые поля могут содержать
// невычисленные ${{ CEL }} — их вычисляет исполнитель по рантайм-данным.
message CompiledPlan {
    ProviderRef provider = 1;
    repeated MachineGroup machine_groups = 2;
    repeated ServiceSpec services = 3;
    repeated CompiledJob jobs = 4;
}

message ProviderRef {
    string name = 1;
    // params_json — provider.params как JSON (схема динамическая, в proto не типизируется).
    string params_json = 2;
}

message MachineGroup {
    string name = 1;
    uint32 count = 2;
    uint32 cpu = 3;
    uint64 ram_mb = 4;
    repeated DiskSpec disks = 5;
    // ext_json — провайдерский ext-блок после lowering-прецеденса, как JSON.
    string ext_json = 6;
}

message DiskSpec {
    uint64 size_gb = 1;
    // type — уже lowered в термины провайдера.
    string type = 2;
}

message ServiceSpec {
    string name = 1;
    string on_group = 2;
    string image = 3;
    string network = 4;
    repeated string volumes = 5;
    map<string, string> env = 6;
    repeated ConfigFile configs = 7;
    HealthCheck health = 8;
}

message ConfigFile {
    string template_path = 1;
    string dest = 2;
}

message HealthCheck {
    string http = 1;
    string timeout = 2;
}

message CompiledJob {
    // id включает matrix-инстанс: "bench[workload=insert]".
    string id = 1;
    repeated string needs = 2;
    string on_group = 3;
    map<string, string> matrix = 4;
    string when = 5; // CEL, пусто = всегда
    oneof action {
        StepList steps = 10;
        // service — имя ServiceSpec: запуск Nomad job + ожидание health.
        string service = 11;
    }
    map<string, string> with = 6;
}

message StepList {
    repeated DslStep steps = 1;
}

message DslStep {
    oneof step {
        cloud.v1.deployment.AgentStep agent = 1;
        WaitStep wait = 2;
    }
}

message WaitStep {
    string http = 1;
    string timeout = 2;
}
```

- [ ] **Step 1: Написать proto** (контент выше), запустить `make protocols`
Expected: генерируется `internal/proto/cloud/v1/dsl/compiled.pb.go` без ошибок плагинов; `go build ./...` зелёный.
- [ ] **Step 2: Commit** `feat(proto): add cloud.v1.dsl CompiledPlan`

---

### Task 13: Lowering + фасад компилятора

**Files:**
- Create: `internal/dsl/lower/lower.go`, `internal/dsl/compiler.go`
- Test: `internal/dsl/compiler_test.go`

**Interfaces:**
- Consumes: всё выше.
- Produces:

```go
// internal/dsl/compiler.go
type Input struct {
	Sources  include.Sources               // бандл рецепта (cluster.yaml, workflow.yaml, components/**, files/**)
	Provider *ast.ProviderManifest         // выбранный провайдер
	Composed *jsonschema.Schema            // из schema.Compose (nil → только core)
}
// Compile: parse → schema.Validate → include.Resolve → graph.Build →
// contract.Check → graph.Validate/Expand → lower.
// Диагностики аккумулируются по фазам; фаза не запускается, если предыдущая
// дала errors (warnings не блокируют).
func Compile(in Input) (*dslpb.CompiledPlan, diag.List)
```

Lowering-маппинг (lower.go): `ast.Step{Cmd}` → `AgentStep{call_cmd: common.Cmd{...}}` (script-режим, как текущий `CallCmdActivity`); `WriteFile` → `write_file`; `Fetch` → `fetch_file`; `Dir` → `create_dir`; `Wait` → `DslStep{wait}`; `Job.Service` → `CompiledJob{service}`; порядок `AgentStep.Order` — индекс в списке, `Id` — `<jobID>/<idx>`. `MachineGroup.ext_json`/`ProviderRef.params_json` — `json.Marshal` соответствующих map.

- [ ] **Step 1: Падающий тест** — минимальный бандл (1 группа, 1 сервис, 3 джоба: cmd → service → wait) компилируется без diags; в плане: `jobs[0].action.steps.steps[0].agent.call_cmd` заполнен, `jobs[1].service == "postgres"`, `machine_groups[0].ext_json` содержит platform_id, needs сохранены. Второй тест: бандл с нарушением контракта (etcd count=2) → `Compile` возвращает nil-план + diag c Module="etcd" (проверка сквозной интеграции фаз).
- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Реализация**
- [ ] **Step 4: Run — PASS** (`go test ./internal/dsl/... -v && make lint`)
- [ ] **Step 5: Commit** `feat(dsl): lowering to CompiledPlan and compiler facade`

---

### Task 14: Golden-тест: реальный рецепт postgres-ha

**Files:**
- Create: `examples/dsl/postgres-ha/cluster.yaml`, `examples/dsl/postgres-ha/workflow.yaml`, `examples/dsl/postgres-ha/components/etcd/component.yaml`, `examples/dsl/postgres-ha/components/patroni/component.yaml`, `examples/dsl/postgres-ha/providers/yandex/manifest.yaml`, `examples/dsl/postgres-ha/providers/yandex/module/variables.tf`, `examples/dsl/postgres-ha/files/patroni.yaml.tpl`
- Test: `internal/dsl/golden_test.go`, golden: `internal/dsl/testdata/postgres-ha.golden.json`

**Interfaces:**
- Consumes: `Compile`, `schema.{DeriveParamsSchema,Compose}`.
- Produces: эталонный рецепт = живой пример DSL для документации и следующих планов (интерпретатор, миграция БД §9 спеки).

- [ ] **Step 1: Написать рецепт** — перенос смысла существующего Go-рендера postgres HA (`internal/domain/database/postgres/`): группы `db×3` (etcd+patroni) и `runner×1`; сервисы etcd/patroni-postgres/haproxy/stroppy как docker jobs; workflow: prep-disks → etcd → patroni → wait-primary (`:8008/primary`) → bench (matrix по workload). Компоненты etcd/patroni с requires из спеки §6. Точное содержимое конфигов НЕ копировать из Go-рендера — цель зафиксировать структуру плана, не побайтовую эквивалентность (полный дифф-тест — миграционный план, вне этого).
- [ ] **Step 2: Golden-тест**

```go
func TestPostgresHAGolden(t *testing.T) {
	src := loadDir(t, "../../examples/dsl/postgres-ha") // map path→bytes
	params, ext, err := schema.DeriveParamsSchema("../../examples/dsl/postgres-ha/providers/yandex/module")
	if err != nil {
		t.Fatal(err)
	}
	composed, _, err := schema.Compose(map[string]schema.ProviderSchemas{"yandex": {Params: params, Ext: ext}}, nil)
	...
	plan, diags := dsl.Compile(dsl.Input{Sources: src, Provider: manifest, Composed: composed})
	if diags.HasErrors() {
		t.Fatalf("diags: %+v", diags)
	}
	got, _ := protojson.MarshalOptions{Multiline: true}.Marshal(plan)
	compareOrUpdateGolden(t, "testdata/postgres-ha.golden.json", got) // -update flag
}
```

- [ ] **Step 3: Run — FAIL, затем зафиксировать golden** (`go test ./internal/dsl/ -run Golden -update`)
- [ ] **Step 4: Run — PASS без -update**; негативный smoke: сломать `count: 2` у db-группы в копии — etcd-контракт должен дать error (тест `TestPostgresHAContractViolation` с in-memory правкой байтов cluster.yaml).
- [ ] **Step 5: Commit** `feat(dsl): postgres-ha example recipe with golden test`

---

### Task 15: Nomad job-spec маппинг + активности gateway-агента

**Files:**
- Create: `internal/dsl/nomad/jobspec.go`, `internal/agent/nomad_activities.go`
- Test: `internal/dsl/nomad/jobspec_test.go`, `internal/agent/nomad_activities_test.go`
- Modify: `go.mod` (добавить `github.com/hashicorp/nomad/api`)

**Interfaces:**
- Consumes: `dslpb.ServiceSpec`, `dslpb.MachineGroup` (Task 12); паттерн существующих активностей `internal/agent/activities.go` (heartbeat-тикер, структура Input/Output, регистрация в `cmd/cli/agent_cmd.go`).
- Produces:

```go
// internal/dsl/nomad/jobspec.go
// BuildJob: ServiceSpec + целевые ноды → Nomad job (docker driver, host network,
// constraint по node_id через node meta stroppy_node_id, один task group на машину
// группы). Health из ServiceSpec → nomad service check (http, timeout).
func BuildJob(svc *dslpb.ServiceSpec, nodes []NodeRef) (*api.Job, error)
type NodeRef struct{ NodeID, PrivateIP string }

// internal/agent/nomad_activities.go — активности gateway-агента, дёргают
// localhost:4646 (api.NewClient c address из env STROPPY_NOMAD_ADDR, default localhost).
// Регистрируются ТОЛЬКО на gateway-ноде (роль в bootstrap).
func (a *Activities) NomadSubmitJobActivity(ctx context.Context, in *NomadSubmitJobInput) (*NomadSubmitJobOutput, error)   // register + poll до running/failed, heartbeat
func (a *Activities) NomadJobStatusActivity(ctx context.Context, in *NomadJobStatusInput) (*NomadJobStatusOutput, error)
func (a *Activities) NomadStopJobActivity(ctx context.Context, in *NomadStopJobInput) (*NomadStopJobOutput, error)
func (a *Activities) NomadAllocLogsActivity(ctx context.Context, in *NomadAllocLogsInput) (*NomadAllocLogsOutput, error)
```

- [ ] **Step 1: Падающий тест jobspec** — `BuildJob` для postgres-ServiceSpec (image, host network, volumes, env с `${{ }}`-строкой уже вычисленной, health) на 3 нодах → job c 3 task groups, у каждой constraint `${meta.stroppy_node_id} == <id>`, docker config с image/network_mode=host/volumes, check http с timeout.
- [ ] **Step 2: Run — FAIL** (`go test ./internal/dsl/nomad/ -v`)
- [ ] **Step 3: Реализация jobspec + активностей** — активности тестируются httptest-сервером, имитирующим Nomad API (register 200 + evaluations поллинг); heartbeat-паттерн скопировать из `CallCmdActivity`.
- [ ] **Step 4: Run — PASS** (`go test ./internal/dsl/nomad/ ./internal/agent/ -v`)
- [ ] **Step 5: Commit** `feat(dsl): nomad jobspec mapping and gateway agent activities`

---

### Task 16: Temporal-интерпретатор CompiledPlan

**Files:**
- Create: `internal/workflows/dslrun.go`
- Test: `internal/workflows/dslrun_test.go`
- Modify: `internal/workflows/register.go` (регистрация workflow)

**Interfaces:**
- Consumes: `dslpb.CompiledPlan` (Task 12), `expr.Eval` (Task 7), Nomad-активности (Task 15); существующие паттерны: `executeAgentStep`/`executeActivityNoResult` (`internal/workflows/deployment.go:607-632`), per-node task queue (`agentTaskQueue`, deployment.go:493), child-workflow структура `internal/workflows/test.go`.
- Produces:

```go
// ExecuteCompiledPlanWorkflow — generic DAG-исполнитель:
//  - вход: план + map group→[]MachineState (от провайдера) + bootstrap;
//  - готовность джоба = все needs в terminal-success; параллелизм через workflow.Go
//    (паттерн волн из executeDeploymentPlanWorkflow);
//  - when: CEL eval (детерминированно: только по входным данным);
//  - steps-джоб: DslStep.agent → существующие агент-активности на task queue каждой
//    машины on_group; DslStep.wait → активность-поллер (SideEffect-free, retry policy);
//  - service-джоб: expr.Eval всех ${{ }} в env/args по рантайм-биндингам
//    (machines → views с IP из MachineState) → nomad.BuildJob → NomadSubmitJobActivity
//    на gateway task queue;
//  - фейл джоба → фейл зависимых, независимые ветки дорабатывают; итог — статус по джобам.
type ExecuteCompiledPlanInput struct {
	Plan     *dslpb.CompiledPlan
	Machines map[string][]*deploymentpb.MachineState // group → machines
	Bootstrap *workflowpb.AgentBootstrap             // тип как в существующих workflow
}
func ExecuteCompiledPlanWorkflow(ctx workflow.Context, in *ExecuteCompiledPlanInput) (*ExecuteCompiledPlanOutput, error)
```

- [ ] **Step 1: Падающий тест** — `testsuite.WorkflowTestSuite` с замоканными активностями: план из 4 джобов (a→b, a→c, [b,c]→d; b = service, остальные steps): проверить порядок (a раньше b/c; d последним), b вызвал NomadSubmitJobActivity, matrix-джоб исполнился per-инстанс; фейл c (мок возвращает error) → d не исполнен, b исполнен, workflow вернул статусы `{a: ok, b: ok, c: failed, d: skipped}`.
- [ ] **Step 2: Run — FAIL** (`go test ./internal/workflows/ -run CompiledPlan -v`)
- [ ] **Step 3: Реализация**
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(workflows): generic CompiledPlan DAG interpreter`

---

### Task 17: Провайдер-раннер (сабпроект 2): tf-модуль как внешний контракт

**Files:**
- Create: `internal/infrastructure/provider/provider.go`, `internal/infrastructure/provider/terraform.go`, `internal/infrastructure/provider/docker.go`
- Test: `internal/infrastructure/provider/terraform_test.go`, `internal/infrastructure/provider/docker_test.go`

**Interfaces:**
- Consumes: `dslpb.MachineGroup`/`dslpb.ProviderRef` (Task 12); существующий terraform executor (`internal/infrastructure/terraform/executor.go` — `Apply/Destroy/Output`), docker executor (`internal/infrastructure/docker/executor.go`), `deploymentpb.MachineState`.
- Produces:

```go
// Интерфейс провайдера (замена enum-switch — спека §3):
type Provider interface {
	// Provision: машины по группам. nodes-вход tf-модуля:
	//   stroppy_nodes = [{id, group, cpu, ram_gb, disk: {size_gb, type}, ext: {...}}]
	// (id генерится здесь: "<group>-<idx>"); выход: output stroppy_machines
	//   [{id, private_ip, public_ip}] → []MachineState (id → node_id, группа в labels).
	Provision(ctx context.Context, ref *dslpb.ProviderRef, groups []*dslpb.MachineGroup) (map[string][]*deploymentpb.MachineState, error)
	Destroy(ctx context.Context, ref *dslpb.ProviderRef) error
}
func NewTerraform(moduleDir string, exec terraformExec) Provider // params_json+groups → tfvars JSON
func NewDocker(exec dockerExec) Provider                         // builtin: группа → контейнеры stroppy-agent (паттерн текущего docker-провайдера)
```

- [ ] **Step 1: Падающий тест terraform-провайдера** — фейковый `terraformExec` (интерфейс с Apply/Output), проверить: tfvars содержит `stroppy_nodes` с 3 нодами группы db (id "db-0..2", lowered disk type, ext развёрнут из ext_json) + params из params_json; `stroppy_machines`-output парсится в MachineState c верными node_id/private_ip; отсутствующий output → ошибка «module must export stroppy_machines».
- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Реализация** (docker-builtin — по образу текущего `renderDockerInput`: контейнер на машину, agent-bootstrap env)
- [ ] **Step 4: Run — PASS** (`go test ./internal/infrastructure/provider/ -v`)
- [ ] **Step 5: Commit** `feat(provider): pluggable provider interface with tf-module contract`

---

### Task 18: Connect-сервис: schema-endpoint + check-endpoint

**Files:**
- Create: `protocols/cloud/v1/dsl/service.proto`, `internal/services/dsl_service.go`
- Test: `internal/services/dsl_service_test.go`
- Modify: `internal/app/*` (wiring по образцу остальных 20 connect-сервисов — найти место регистрации сервисов в app и добавить туда)

**Interfaces:**
- Consumes: `schema.Compose`/`DeriveParamsSchema` (Task 6), `dsl.Compile` (Task 13), `diag.Diagnostic`.
- Produces:

```proto
service DslService {
    // ComposedSchema — динамическая JSON Schema организации (для Monaco/yaml-language-server).
    rpc ComposedSchema(ComposedSchemaRequest) returns (ComposedSchemaResponse); // {schema_json string}
    // Check — компиляция бандла в check-режиме, возвращает диагностики (IDE-линтер).
    rpc Check(CheckRequest) returns (CheckResponse); // files map<string,bytes> → repeated Diagnostic{severity,path,line,col,message,module}
}
```

Хранилище провайдеров для v1: каталог `providers/` внутри переданного в Check бандла + builtin docker (открытый вопрос спеки §11 про постоянное хранилище решается в сабпроекте 4, здесь — stateless).

- [ ] **Step 1: proto + `make protocols`** — генерация зелёная.
- [ ] **Step 2: Падающий тест** — `Check` с валидным postgres-ha бандлом (из Task 14) → 0 diagnostics; с etcd count=2 → diagnostic {severity: ERROR, module: "etcd"}; `ComposedSchema` для бандла с yandex-модулем → schema_json содержит `platform_id`.
- [ ] **Step 3: Run — FAIL**
- [ ] **Step 4: Реализация + wiring в app**
- [ ] **Step 5: Run — PASS** (`go test ./internal/services/ -run Dsl -v && go build ./...`)
- [ ] **Step 6: Commit** `feat(services): dsl schema and check endpoints`

---

## Вне этого плана

- Миграционные дифф-тесты YAML↔Go-рендер по БД (§9 спеки), перевод остальных 10 БД.
- Bootstrap per-run Nomad-кластера в cloud-init/terraform-модуле (сабпроект 3, инфра-часть).
- Git + browser IDE (сабпроект 4).

## Self-Review

- Spec coverage: §4 DSL (Tasks 2,3,8), §5 ext/lowering/видимость (6,7,9), §6 контракты (4,10), §7 компилятор/lowering (12,13), §8 composed-схема (5,6; endpoint — вне плана, отмечено), §10 негативный тест видимости ext (Task 7 Step 1). Интерпретатор и endpoint — сознательно вынесены, зафиксировано в «Вне этого плана».
- Placeholders: код каркасный только там, где полный листинг дублировал бы Interfaces-блок; каждый тест — конкретный. `...` в Step 3 Task 2 и golden-тесте — свёртка механики, описанной текстом рядом.
- Types: `diag.List` возвращаемый тип везде; `ast.*` имена согласованы между задачами; `dslpb` путь совпадает с easyp-конвенцией генерации (`internal/proto/cloud/v1/dsl`).
