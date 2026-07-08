# SP-A: Schema Layer (form-facing schemapb) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the form-facing schema library on `schemapb`: derive provider-params and workflow-inputs `schemapb.Schema`, compose them into a launch-form schema, and bake filled values into a hashable `Baked` snapshot.

**Architecture:** New pure functions in `internal/dsl/schema` produce `*schemapb.Schema` (reusing the existing HCL type-constraint scanners in `tfvars.go`). A compose function nests provider params under a `provider` field via `schemapb.ObjectOf`. Bake goes through `schemapb`'s validator. `DslService.ComposedSchema` returns the form schema as `schemapb.Schema` protojson (replacing jsonschema). Bundle-structure validation stays on `jsonschema/v6`, untouched.

**Tech Stack:** Go, `github.com/stroppy-io/schemapb` v1.4.4 (fluent builder + `Bake`), `github.com/hashicorp/terraform-config-inspect/tfconfig`, connect-rpc, `internal/dsl` (pure, no I/O except tfconfig).

## Global Constraints

- schemapb version floor: `github.com/stroppy-io/schemapb v1.4.4` (already in go.mod) — do NOT bump.
- `internal/dsl/*` is pure except `DeriveParamsSchema`/`DeriveProviderParamsSchemapb` (tfconfig I/O). Keep new derivation the only I/O; everything else pure.
- Diagnostics use `internal/dsl/diag.List` (`diags.Errorf(path, pos, ...)`); never panic, never fail the whole module over one bad variable (existing tfvars.go contract: permissive fallback).
- No co-author footer in commits. Conventional Commits (`type(scope): summary`).
- Bundle-structure validation (`core.schema.json`, `compose.go`, `validate.go`, jsonschema/v6) is OUT of scope — do not modify.
- schemapb builder terminals: field builders implement `FieldDef` (consumed by `.Fields(...)`); `SchemaB.Build() (*Schema, error)` / `MustBuild()`.

---

### Task 1: Provider params → schemapb (`DeriveProviderParamsSchemapb`)

Port `tfvars.go`'s JSON-Schema derivation to schemapb, reusing the HCL scanners. Rich mapping: types, `validation` blocks → rules, `description` → title/descr, `default`, `sensitive` → secret, `stroppy_machine_ext` → separate ext schema.

**Files:**
- Create: `internal/dsl/schema/tfvars_schemapb.go`
- Test: `internal/dsl/schema/tfvars_schemapb_test.go`
- Reference (reuse, do not modify): `internal/dsl/schema/tfvars.go` (`parseCall`, `splitTopLevel`, `parseObjectBody`, `machineExtVarName`, `parseLiteral`)

**Interfaces:**
- Consumes: `tfconfig.LoadModule`, schemapb builders (`schemapb.NewSchema`, `String`, `Int64`, `Double`, `Bool`, `List`, `Object`, `ObjectOf`, field modifiers `.Default/.Descr/.Secret/.Title/.Required`), existing `internal/dsl/schema` scanners.
- Produces:
  ```go
  // params: object schema with one field per non-ext variable.
  // ext: schema of stroppy_machine_ext alone (nil if absent).
  func DeriveProviderParamsSchemapb(moduleDir, providerName string) (params, ext *schemapb.Schema, diags diag.List)
  // fieldForTFType maps a parsed TF type-constraint + variable meta to a schemapb FieldDef.
  func fieldForTFType(name, tfType, description string, def any, sensitive bool) schemapb.FieldDef
  ```

- [ ] **Step 1: Write the failing test**

```go
package schema

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeriveProviderParamsSchemapb_ScalarsAndExt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "variables.tf", `
variable "image" {
  type        = string
  description = "container image"
  default     = "postgres:16"
}
variable "replicas" {
  type    = number
  default = 3
}
variable "token" {
  type      = string
  sensitive = true
}
variable "stroppy_machine_ext" {
  type = object({ zone = string, disk_gb = optional(number, 100) })
}
`)

	params, ext, diags := DeriveProviderParamsSchemapb(dir, "yandex")
	require.False(t, diags.HasErrors(), diags.String())
	require.NotNil(t, params)
	require.NotNil(t, ext)

	byName := fieldsByName(params) // helper: map[string]*schemapb.Schema_Filed
	require.Contains(t, byName, "image")
	require.Contains(t, byName, "replicas")
	require.Contains(t, byName, "token")
	require.NotContains(t, byName, "stroppy_machine_ext", "ext var must be split out")

	require.Equal(t, "container image", byName["image"].GetDescription())
	require.True(t, byName["token"].GetSecret(), "sensitive var → secret")

	extFields := fieldsByName(ext)
	require.Contains(t, extFields, "zone")
	require.Contains(t, extFields, "disk_gb")
}
```

Add test helpers `writeFile`, `fieldsByName` (map field name → `*schemapb.Schema_Filed`, reading `params.GetFields()`) in the test file.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/dsl/schema/ -run TestDeriveProviderParamsSchemapb_ScalarsAndExt -v`
Expected: FAIL — `undefined: DeriveProviderParamsSchemapb`.

- [ ] **Step 3: Write minimal implementation**

```go
package schema

import (
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
)

// DeriveProviderParamsSchemapb reads a provider module's variables.tf and
// derives two schemapb schemas: params (one field per non-ext variable) and
// ext (the stroppy_machine_ext object, nil if absent). Best-effort: an
// unparseable variable degrades to a permissive field, never fails the module.
func DeriveProviderParamsSchemapb(moduleDir, providerName string) (params, ext *schemapb.Schema, diags diag.List) {
	mod, loadDiags := tfconfig.LoadModule(moduleDir)
	if err := loadDiags.Err(); err != nil {
		diags.Errorf(moduleDir, diag.Pos{}, "load terraform module: %v", err)
		return nil, nil, diags
	}

	var paramFields []schemapb.FieldDef
	var extSchema *schemapb.Schema
	for name, v := range mod.Variables {
		f := fieldForTFType(name, v.Type, v.Description, v.Default, v.Sensitive)
		if name == machineExtVarName {
			extSchema = schemapb.NewSchema("stroppy.provider."+providerName, "machine_ext", "1").
				Fields(objectFieldsOf(f)...).MustBuild()
			continue
		}
		paramFields = append(paramFields, f)
	}

	built, err := schemapb.NewSchema("stroppy.provider."+providerName, "params", "1").
		Fields(paramFields...).Build()
	if err != nil {
		diags.Errorf(moduleDir, diag.Pos{}, "build params schema: %v", err)
		return nil, nil, diags
	}
	return built, extSchema, diags
}

// fieldForTFType maps a TF type-constraint to a schemapb FieldDef, reusing
// tfvars.go's balanced-paren scanners (parseCall/splitTopLevel/parseObjectBody).
func fieldForTFType(name, tfType, description string, def any, sensitive bool) schemapb.FieldDef {
	f := builderForTFType(name, tfType) // switch on parseType-style classification, see below
	if description != "" {
		f = f.Descr(description) // fieldBase.Descr
	}
	if sensitive {
		f = f.Secret()
	}
	// default applied per-kind inside builderForTFType when def != nil
	return f
}
```

Implement `builderForTFType(name, raw string) schemapb.FieldDef` mirroring `parseType`'s switch but emitting builders:
- `"string"` → `schemapb.String(name)`
- `"number"` → `schemapb.Double(name)` (numbers may be fractional; use Double for TF `number`)
- `"bool"` → `schemapb.Bool(name)`
- `"any"`/`""` → `schemapb.String(name)` permissive fallback (documented)
- `list(T)`/`set(T)` → `schemapb.List(name, builderForTFType("item", T))`
- `object({...})` → `schemapb.Object(name, <fields from parseObjectBody>...)`
- `map(T)` → permissive object fallback for v1 (note in comment)

Add `objectFieldsOf(f schemapb.FieldDef) []schemapb.FieldDef` helper: if `f` is an object field, return its inner fields (for ext schema flattening); else wrap.

> NOTE: `builderForTFType` reuses the existing `parseCall`/`parseObjectBody` from tfvars.go — call them, do not re-implement the scanning.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/dsl/schema/ -run TestDeriveProviderParamsSchemapb_ScalarsAndExt -v`
Expected: PASS.

- [ ] **Step 5: Add validation-block → rule mapping test + impl**

```go
func TestDeriveProviderParamsSchemapb_ValidationToRule(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "variables.tf", `
variable "replicas" {
  type = number
  validation {
    condition     = var.replicas >= 1
    error_message = "at least 1 replica"
  }
}
`)
	params, _, diags := DeriveProviderParamsSchemapb(dir, "yandex")
	require.False(t, diags.HasErrors())
	require.NotEmpty(t, params.GetRules(), "tf validation block → schemapb rule")
	require.Contains(t, params.GetRules()[0].GetMessage(), "at least 1 replica")
}
```

Impl: for each `v.Validation`-equivalent (tfconfig exposes validation via the module's variable — confirm field; if tfconfig lacks it, parse the `validation {}` block from the raw HCL file with the existing scanners and translate simple `var.X <op> N` conditions to CEL `X <op> N`). Attach via `schemapb.Rule(celExpr, msg)`; conditions not translatable → skip with a `diags.Warnf` (do not fail).

Run: `go test ./internal/dsl/schema/ -run TestDeriveProviderParamsSchemapb -v` → PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/dsl/schema/tfvars_schemapb.go internal/dsl/schema/tfvars_schemapb_test.go
git commit -m "feat(schema): derive provider params as schemapb from variables.tf"
```

---

### Task 2: Workflow inputs → schemapb (`DeriveInputsSchema`)

Map `ast` workflow inputs to a schemapb schema (typed, not string-scalars).

**Files:**
- Create: `internal/dsl/schema/inputs_schemapb.go`
- Test: `internal/dsl/schema/inputs_schemapb_test.go`
- Reference: `internal/dsl/ast/component.go` (`InputSpec{Type string; Default any}`), `internal/dsl/ast/workflow.go` (`WorkflowDoc`).

**Interfaces:**
- Consumes: `ast.WorkflowDoc` (its declared inputs), schemapb builders.
- Produces:
  ```go
  func DeriveInputsSchema(namespace string, inputs map[string]ast.InputSpec) (*schemapb.Schema, diag.List)
  ```

- [ ] **Step 1: Write the failing test**

```go
func TestDeriveInputsSchema_Types(t *testing.T) {
	inputs := map[string]ast.InputSpec{
		"db_version": {Type: "string", Default: "16"},
		"threads":    {Type: "number", Default: 64},
		"ssl":        {Type: "bool", Default: false},
	}
	s, diags := DeriveInputsSchema("stroppy.workflow.tpcc", inputs)
	require.False(t, diags.HasErrors(), diags.String())
	byName := fieldsByName(s)
	require.Contains(t, byName, "db_version")
	require.Contains(t, byName, "threads")
	require.Contains(t, byName, "ssl")
}
```

- [ ] **Step 2: Run to verify fail** — `go test ./internal/dsl/schema/ -run TestDeriveInputsSchema_Types -v` → FAIL undefined.

- [ ] **Step 3: Implement**

```go
func DeriveInputsSchema(namespace string, inputs map[string]ast.InputSpec) (*schemapb.Schema, diag.List) {
	var diags diag.List
	var fields []schemapb.FieldDef
	for name, spec := range inputs {
		switch spec.Type {
		case "string", "version", "":
			b := schemapb.String(name)
			if s, ok := spec.Default.(string); ok {
				b = b.Default(s)
			}
			fields = append(fields, b)
		case "number", "int":
			b := schemapb.Int64(name)
			if d, ok := toInt64(spec.Default); ok {
				b = b.Default(d)
			}
			fields = append(fields, b)
		case "bool":
			b := schemapb.Bool(name)
			if v, ok := spec.Default.(bool); ok {
				b = b.Default(v)
			}
			fields = append(fields, b)
		default:
			diags.Warnf("", diag.Pos{}, "input %q: unknown type %q, treating as string", name, spec.Type)
			fields = append(fields, schemapb.String(name))
		}
	}
	s, err := schemapb.NewSchema(namespace, "inputs", "1").Fields(fields...).Build()
	if err != nil {
		diags.Errorf("", diag.Pos{}, "build inputs schema: %v", err)
	}
	return s, diags
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	}
	return 0, false
}
```

- [ ] **Step 4: Run to verify pass** — PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/dsl/schema/inputs_schemapb.go internal/dsl/schema/inputs_schemapb_test.go
git commit -m "feat(schema): derive workflow inputs as schemapb"
```

---

### Task 3: Compose form schema (`ComposeFormSchema`)

Merge inputs + provider params into one form schema; params nested under a `provider` object field.

**Files:**
- Create: `internal/dsl/schema/form.go`
- Test: `internal/dsl/schema/form_test.go`

**Interfaces:**
- Consumes: Task 1 `params *schemapb.Schema`, Task 2 inputs `*schemapb.Schema`, `schemapb.ObjectOf`.
- Produces:
  ```go
  func ComposeFormSchema(namespace string, inputs, params *schemapb.Schema) (*schemapb.Schema, error)
  ```

- [ ] **Step 1: Write the failing test**

```go
func TestComposeFormSchema_NestsProvider(t *testing.T) {
	inputs := schemapb.NewSchema("ns", "inputs", "1").
		Fields(schemapb.String("db_version").Default("16")).MustBuild()
	params := schemapb.NewSchema("ns", "params", "1").
		Fields(schemapb.Double("replicas").Default(3)).MustBuild()

	form, err := ComposeFormSchema("stroppy.form.tpcc", inputs, params)
	require.NoError(t, err)
	byName := fieldsByName(form)
	require.Contains(t, byName, "db_version", "inputs hoisted to top level")
	require.Contains(t, byName, "provider", "params nested under provider")
}
```

- [ ] **Step 2: Run to verify fail** → FAIL undefined.

- [ ] **Step 3: Implement**

```go
func ComposeFormSchema(namespace string, inputs, params *schemapb.Schema) (*schemapb.Schema, error) {
	b := schemapb.NewSchema(namespace, "form", "1")
	// top-level input fields
	for _, f := range inputs.GetFields() {
		b = b.Fields(rawField(f)) // wrap an already-built *Schema_Filed as a FieldDef (see rawField)
	}
	if params != nil && len(params.GetFields()) > 0 {
		b = b.Fields(schemapb.ObjectOf("provider", params))
	}
	return b.Build()
}
```

Implement `rawField(f *schemapb.Schema_Filed) schemapb.FieldDef` — a tiny adapter whose `Done()` returns `f` (so pre-built fields can be re-added). If schemapb already exposes such an adapter, use it instead; otherwise this 5-line shim lives in `form.go`.

- [ ] **Step 4: Run to verify pass** → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/dsl/schema/form.go internal/dsl/schema/form_test.go
git commit -m "feat(schema): compose launch-form schema (inputs + provider params)"
```

---

### Task 4: Bake filled form (`BakeForm`)

Validate + seal filled values into a hashable `Baked` (run identity).

**Files:**
- Modify: `internal/dsl/schema/form.go`
- Test: `internal/dsl/schema/form_test.go`

**Interfaces:**
- Consumes: `*schemapb.Schema` (Task 3), `schemapb.Schema.Bake`.
- Produces:
  ```go
  func BakeForm(form *schemapb.Schema, values map[string]any) (*schemapb.Baked, []*schemapb.FieldError, error)
  ```

- [ ] **Step 1: Write the failing test**

```go
func TestBakeForm_ValidAndInvalid(t *testing.T) {
	form := schemapb.NewSchema("ns", "form", "1").
		Fields(schemapb.Int64("replicas").Gte(1).Default(3)).MustBuild()

	baked, ferrs, err := BakeForm(form, map[string]any{"replicas": int64(5)})
	require.NoError(t, err)
	require.Empty(t, ferrs)
	require.NotNil(t, baked)

	_, ferrs2, err2 := BakeForm(form, map[string]any{"replicas": int64(0)})
	require.NoError(t, err2)
	require.NotEmpty(t, ferrs2, "value below Gte(1) → FieldError")
}
```

- [ ] **Step 2: Run to verify fail** → FAIL undefined.

- [ ] **Step 3: Implement**

```go
func BakeForm(form *schemapb.Schema, values map[string]any) (*schemapb.Baked, []*schemapb.FieldError, error) {
	if form == nil {
		return nil, nil, fmt.Errorf("bake form: nil schema")
	}
	baked, ferrs := form.Bake(values)
	return baked, ferrs, nil
}
```

- [ ] **Step 4: Run to verify pass** → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/dsl/schema/form.go internal/dsl/schema/form_test.go
git commit -m "feat(schema): bake filled form values into hashable Baked"
```

---

### Task 5: Wire `DslService.ComposedSchema` to schemapb

Return the composed form schema as `schemapb.Schema` protojson instead of jsonschema.

**Files:**
- Modify: `internal/services/dsl/service.go` (`ComposedSchema`, ~line 118; `deriveProviderSchema` caller path)
- Test: `internal/services/dsl/service_test.go`
- Reference: `internal/proto/cloud/v1/dsl` `ComposedSchemaResponse{SchemaJson string}` (keep the string field; it now carries schemapb protojson).

**Interfaces:**
- Consumes: Task 1 `DeriveProviderParamsSchemapb`, Task 2 `DeriveInputsSchema`, Task 3 `ComposeFormSchema`; `protojson.Marshal`.
- Produces: `ComposedSchema` returns `ComposedSchemaResponse{SchemaJson: <schemapb.Schema protojson>}`.

- [ ] **Step 1: Write the failing test**

```go
func TestComposedSchema_ReturnsSchemapbProtojson(t *testing.T) {
	svc := NewDslService(/* existing test deps */)
	resp, err := svc.ComposedSchema(context.Background(), &dslpb.ComposedSchemaRequest{
		Files: dockerBundleFiles(), // existing test bundle helper
	})
	require.NoError(t, err)
	var s schemapb.Schema
	require.NoError(t, protojson.Unmarshal([]byte(resp.GetSchemaJson()), &s),
		"response must be schemapb.Schema protojson")
	require.NotEmpty(t, s.GetName())
}
```

- [ ] **Step 2: Run to verify fail** — `go test ./internal/services/dsl/ -run TestComposedSchema_ReturnsSchemapbProtojson -v` → FAIL (still jsonschema).

- [ ] **Step 3: Implement**

Replace `ComposedSchema`'s jsonschema derivation+compose with: parse the bundle's workflow inputs (`ast`) → `DeriveInputsSchema`; resolve provider module dir → `DeriveProviderParamsSchemapb`; `ComposeFormSchema`; `protojson.Marshal` into `SchemaJson`. Keep the "no provider → permissive placeholder" and diagnostics-as-empty behavior. Leave `deriveProviderSchema` (jsonschema, used by `resolveProvider` for bundle-structure) intact — this task only rewires the form-facing `ComposedSchema` output.

- [ ] **Step 4: Run to verify pass** → PASS. Then full package: `go test ./internal/services/dsl/... ./internal/dsl/schema/...` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/services/dsl/service.go internal/services/dsl/service_test.go
git commit -m "feat(dsl): ComposedSchema returns schemapb form schema (protojson)"
```

---

### Task 6: Parity guard test (Go bake == schema contract)

Lock the invariant that the composed form schema bakes filled values deterministically (foundation for SP-D WASM parity).

**Files:**
- Test: `internal/dsl/schema/parity_test.go`

**Interfaces:**
- Consumes: Tasks 1–4 functions.

- [ ] **Step 1: Write the test**

```go
func TestFormBake_StableHash(t *testing.T) {
	inputs := schemapb.NewSchema("ns", "inputs", "1").
		Fields(schemapb.String("db_version").Default("16")).MustBuild()
	params := schemapb.NewSchema("ns", "params", "1").
		Fields(schemapb.Double("replicas").Default(3)).MustBuild()
	form, err := ComposeFormSchema("ns.form", inputs, params)
	require.NoError(t, err)

	vals := map[string]any{"db_version": "17", "provider": map[string]any{"replicas": float64(5)}}
	b1, e1, err1 := BakeForm(form, vals)
	b2, e2, err2 := BakeForm(form, vals)
	require.NoError(t, err1)
	require.NoError(t, err2)
	require.Empty(t, e1)
	require.Empty(t, e2)
	require.Equal(t, hashBaked(t, b1), hashBaked(t, b2), "same schema+values → same Baked hash")
}
```

Add `hashBaked` helper using schemapb's hashpb sum (see `schemapb` `Baked` hash API — `*Baked` is hashable).

- [ ] **Step 2: Run to verify pass** — `go test ./internal/dsl/schema/ -run TestFormBake_StableHash -v` → PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/dsl/schema/parity_test.go
git commit -m "test(schema): stable Baked hash for identical form+values"
```

---

## Self-Review

**Spec coverage (SP-A §3):**
- A1 provider.params (rich) → Task 1 (types + validation→rule + secret + default + ext). ✓
- A2 workflow.inputs → Task 2. ✓
- A3 compose form → Task 3. ✓
- A4 Filled/Baked → Task 4 (+ Task 6 hash stability). ✓
- ComposedSchema RPC (§4) → Task 5. ✓
- jsonschema untouched (§5) → Task 5 leaves `deriveProviderSchema`/bundle-structure intact. ✓
- Compiler boundary (§7, Baked→CompiledPlan substitution) → deferred to SP-D, noted in spec; NOT a task here. ✓

**Open items surfaced for the implementer:**
- Task 1 Step 5: confirm whether `tfconfig` exposes variable `validation` blocks; if not, parse from raw HCL using existing scanners. (Spec §10.1.)
- Task 3: exact `ObjectOf`/`rawField` adapter — verify against `schemapb/compose.go` (`ObjectOf(name, *Schema) *ObjectB`) before implementing. (Spec §10.2.)
- Task 5: `ComposedSchemaResponse.SchemaJson` reused as protojson carrier avoids a proto message change and the ogen/GQL converter rabbit hole. (Spec §10.3.)
- Form-schema identity/versioning (namespace = `stroppy.form.<workflow-id>`) — Task 3 uses a stable namespace; refine in SP-D if run identity needs the bundle version. (Spec §10.4.)

**Placeholder scan:** no TBD/TODO; representative test code + real impl in every code step. HCL-scanner reuse is an explicit call to existing named functions, not a placeholder.

**Type consistency:** `*schemapb.Schema` produced by Tasks 1–3 flows into Task 4 `BakeForm` and Task 5 RPC; `fieldsByName`/`writeFile` helpers shared across schema tests.
