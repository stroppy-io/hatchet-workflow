# SP-D: Launch Form (schemapb-driven) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let an end user pick a workflow from the org catalog, get a `schemapb.Schema`-driven launch form (`workflow.inputs ⊕ provider.params`), fill it with live WASM validation identical to the server, and launch a run — without touching YAML. Closes the loop SP-A's schema layer opens: `ComposeFormSchema` → browser WASM → `Filled` → server `BakeForm` → `Baked` → typed substitution into the compiled bundle → `RunRecipeWorkflow`.

**Architecture:** `schemapb`'s own npm packages (`@stroppy-io/schemapb`, `@stroppy-io/schemapb-react`) are vendored as GitHub-Packages dependencies into `web/` — no new WASM loader or form-renderer is hand-rolled. A new `RecipeService.LaunchFormSchema` RPC composes the form schema server-side (SP-A functions) and a React page renders it via `useSchemaForm`. `StartRun` gains an optional `filled` field; the server bakes it synchronously (fast field-error feedback, no bundle compile needed) and, only on success, threads the resulting `Baked` snapshot through the **existing async path** — `RunRecipeWorkflow` → `CompileRecipeActivity` → `dsl.Compile` — because that is where the DSL compiler (`include.Resolve` → `lower.Lower`) actually runs (a Temporal activity, not the connect-rpc handler). Provider-param values overlay `ast.ClusterDoc.Provider.Params` before lowering; workflow-level input values are threaded into a new `include.Resolve` parameter so `${{ inputs.x }}` substitution sees them, and the existing `fmt.Sprint`-based `resolved_inputs` stringification is replaced with a typed `google.protobuf.Value`, closing I1 for CEL `when:`/interpolation.

**Tech Stack:** Go 1.25, `github.com/stroppy-io/schemapb` v1.4.4 (Go side: `internal/dsl/schema` from SP-A); TypeScript/React 19, Vite 6, `@stroppy-io/schemapb`/`@stroppy-io/schemapb-react` (npm, GitHub Packages, `0.0.1`); connect-rpc; Temporal Go SDK; Vitest + Testing Library (new to `web/`, bootstrapped in Task 1).

## Global Constraints

- **Dependency on SP-A**: this plan assumes SP-A has landed `internal/dsl/schema.{DeriveProviderParamsSchemapb, DeriveInputsSchema, ComposeFormSchema, BakeForm}` and `ast.WorkflowDoc.Inputs map[string]ast.InputSpec` (SP-A Task 5's Step 0 prerequisite). Every Go snippet below imports these as already-existing.
- **schemapb version floor**: Go module `github.com/stroppy-io/schemapb v1.4.4` (already in `go.mod` and `protocols/easyp.go.yaml`'s `deps:` — `schemapb/schema.proto` is already importable from `.proto` files, precedent: `protocols/cloud/v1/common/file.proto`'s `schemapb.Baked` field). npm packages pinned at `0.0.1` (their current published version — NOT yet version-synced with the Go module; flagged as a real gap, not silently "fixed" — see Task 1).
- **Do not hand-roll a form renderer or a WASM loader.** `@stroppy-io/schemapb` (`schemapb()`, `Schemapb.validate/compute/bake/fieldActive/enumOptions/listCount`) and `@stroppy-io/schemapb-react` (`useSchemaForm`) are vendored, not reimplemented.
- **Do not modify** `internal/dsl/schema/tfvars.go` core scanners, `core.schema.json`, `internal/services/dsl` bundle-structure (jsonschema/v6) validation, or the runs-table/overview UI (product-vision §4: only the launch form is generated).
- **The DSL compiler runs asynchronously**, inside `CompileRecipeActivity` on a Temporal worker (`internal/infrastructure/execution/recipe_activities.go`), NOT inside the connect-rpc `StartRun` handler (`internal/services/recipe/recipe.go`). Any typed value that must reach `include.Resolve`/`lower.Lower` has to be threaded through `RunRecipeInput` → `CompileRecipeActivityInput` → `dsl.Input`, never mutated in-process at `StartRun` time. This is the single most important correction this plan makes to the spec's §3 D4 sketch — see Task 8 and the Self-Review.
- No co-author footer in commits. Conventional Commits (`type(scope): summary`).
- Go: never panic on user input; diagnostics via `diag.List`. TS: no new global form/validation library (no react-hook-form, no zod) — `schemapb` is the single source of validation truth, per spec §3 D2.

---

### Task 1: Web test harness + `@stroppy-io/schemapb` WASM engine wiring (D1)

`web/` has no test runner today (`web/package.json` scripts are `dev`/`build`/`preview`/`proto:coverage` only — no `test`). This task bootstraps Vitest, vendors the schemapb npm packages from GitHub Packages, and wires a thin loader so later tasks don't touch `globalThis.schemapbValidate`/etc. directly.

**Files:**
- Modify: `web/package.json` (deps + devDeps + `test` script)
- Create: `web/.npmrc`
- Modify: `web/vite.config.ts` (add `test` block for Vitest)
- Create: `web/src/services/schemapbEngine.ts`
- Test: `web/src/services/schemapbEngine.test.ts`
- Modify: `.github/workflows/ci.yml` (new `web` job — none exists today)

**Interfaces:**
- Consumes: `@stroppy-io/schemapb`'s `schemapb(): Promise<Schemapb>` (`packages/schemapb/schemapb.ts`).
- Produces:
  ```ts
  // web/src/services/schemapbEngine.ts
  export function loadSchemapbEngine(): Promise<Schemapb>
  ```

- [ ] **Step 1: Write the failing test**

`web/src/services/schemapbEngine.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { create } from "@bufbuild/protobuf";
import { SchemaSchema } from "@stroppy-io/schemapb";
import { loadSchemapbEngine } from "./schemapbEngine";

describe("loadSchemapbEngine", () => {
  it("loads the shared WASM engine and bakes a trivial schema", async () => {
    const engine = await loadSchemapbEngine();
    const schema = create(SchemaSchema, {
      id: { namespace: "stroppy.test", name: "smoke", version: "v1" },
      fields: [
        { name: "replicas", required: true, kind: { case: "int64", value: { gte: 1n } } },
      ],
    });

    const bad = engine.bake(schema, { replicas: 0 });
    expect(bad.baked).toBeUndefined();
    expect(bad.errors.length).toBeGreaterThan(0);

    const ok = engine.bake(schema, { replicas: 3 });
    expect(ok.baked).toBeDefined();
    expect(ok.errors).toHaveLength(0);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && npx vitest run src/services/schemapbEngine.test.ts`
Expected: FAIL — no `test` runner configured yet, `@stroppy-io/schemapb` not installed, `schemapbEngine.ts` does not exist.

- [ ] **Step 3: Write minimal implementation**

`web/.npmrc` (new — GitHub Packages scoped registry, per this org's "install from GitHub @version, never rebuild locally" convention):
```
@stroppy-io:registry=https://npm.pkg.github.com
//npm.pkg.github.com/:_authToken=${GITHUB_TOKEN}
```

`web/package.json` — add to `"dependencies"`:
```json
    "@stroppy-io/schemapb": "0.0.1",
    "@stroppy-io/schemapb-react": "0.0.1",
```
add to `"devDependencies"`:
```json
    "@testing-library/jest-dom": "^6.6.0",
    "@testing-library/react": "^16.0.0",
    "jsdom": "^25.0.0",
    "vitest": "^2.1.0",
```
add to `"scripts"`:
```json
    "test": "vitest run",
```

`web/vite.config.ts` — extend the existing `defineConfig`:
```ts
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
  },
  server: {
    // ... unchanged
  },
});
```

`web/src/test/setup.ts` (new):
```ts
import "@testing-library/jest-dom/vitest";
```

`web/src/services/schemapbEngine.ts` (new):
```ts
// Thin, memoized loader over @stroppy-io/schemapb so the rest of web/ never
// touches the WASM engine's globalThis functions directly (D1 — see
// docs/superpowers/specs/2026-07-08-sp-d-launch-form.md §3 D1). The vendor
// package (packages/schemapb inside github.com/stroppy-io/schemapb) already
// owns the WASM instantiation, caching, and JSON<->protojson bridging.
import { schemapb, type Schemapb } from "@stroppy-io/schemapb";

export type { Schemapb } from "@stroppy-io/schemapb";

/**
 * Returns the shared, lazily-loaded schemapb WASM engine. Safe to call from
 * multiple components: @stroppy-io/schemapb's own `schemapb()` helper
 * memoizes the load, this wrapper exists purely so callers depend on
 * services/schemapbEngine (this app's own module boundary) rather than the
 * vendor package directly.
 */
export function loadSchemapbEngine(): Promise<Schemapb> {
  return schemapb();
}
```

No vite-plugin-wasm/vite-plugin-top-level-await needed (resolves spec §9 open question 2): `packages/schemapb/schemapb.ts`'s `loadDefaultBytes()` fetches `schemapb.wasm` via `new URL("./schemapb.wasm", import.meta.url)` and `wasm_exec.js` via a plain dynamic `import()` of a `.js` URL — both are Vite's standard asset-URL and dynamic-import idioms, requiring no special loader plugin. Confirm this holds by checking the built asset is emitted as a separate file, not inlined as base64 (Vite's default `assetsInlineLimit` is 4KB; `schemapb.wasm` is expected to be well over that — verify with `du -h web/dist/assets/*.wasm` after Step 4's build and set `build.assetsInlineLimit` explicitly if it is ever inlined).

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
cd web
npm install
npx vitest run src/services/schemapbEngine.test.ts
npm run build   # confirms schemapb.wasm ships as a real asset, not inlined
```
Expected: PASS; `du -h web/dist/assets/*.wasm` shows a real file (not absent, meaning it wasn't tree-shaken/inlined away).

Add a `web` job to `.github/workflows/ci.yml` (no web job exists today — `test`/`lint`/`integration` are Go-only):
```yaml
  web:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: 20 }
      - run: cd web && npm install
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
      - run: cd web && npm run test
      - run: cd web && npm run build
```

- [ ] **Step 5: Commit**

```bash
git add web/package.json web/.npmrc web/vite.config.ts web/src/test/setup.ts \
  web/src/services/schemapbEngine.ts web/src/services/schemapbEngine.test.ts \
  .github/workflows/ci.yml web/package-lock.json web/yarn.lock
git commit -m "feat(web): vendor schemapb WASM engine, bootstrap vitest"
```

---

### Task 2: React launch-form field renderer (D2)

Render every `schemapb` field kind the spec names (String/Int64/Double/Bool/Enum/List/Object/Ref), `when`-conditional visibility, dynamic enum options, and live `Computed` recompute — using `useSchemaForm`, not a hand-rolled controller.

**Files:**
- Create: `web/src/components/launch-form/LaunchFormRenderer.tsx`
- Create: `web/src/components/launch-form/fields.tsx` (per-kind field components)
- Test: `web/src/components/launch-form/LaunchFormRenderer.test.tsx`

**Interfaces:**
- Consumes: `useSchemaForm` (`@stroppy-io/schemapb-react`), `web/src/components/ui/{input,select,switch,label,num-field}.tsx`.
- Produces:
  ```tsx
  export function LaunchFormRenderer(props: {
    schema: Schema;
    onSubmit: (baked: BakedJson) => void;
    submitLabel?: string;
  }): JSX.Element
  ```

- [ ] **Step 1: Write the failing test**

```tsx
// web/src/components/launch-form/LaunchFormRenderer.test.tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { SchemaSchema } from "@stroppy-io/schemapb";
import { LaunchFormRenderer } from "./LaunchFormRenderer";

function fieldCoverageSchema() {
  return create(SchemaSchema, {
    id: { namespace: "stroppy.test", name: "launch", version: "v1" },
    fields: [
      { name: "db_version", kind: { case: "string", value: { default: "16" } } },
      { name: "threads", kind: { case: "int64", value: { default: 4n, gte: 1n } } },
      { name: "ratio", kind: { case: "double", value: { default: 0.5 } } },
      { name: "ssl", kind: { case: "bool", value: { default: false } } },
      {
        name: "engine",
        kind: { case: "enum", value: { values: { 1: "postgres", 2: "mysql" }, definedOnly: true } },
      },
      {
        name: "tls_key",
        when: "this.ssl == true",
        kind: { case: "string", value: {} },
      },
      {
        name: "nodes",
        kind: { case: "list", value: { item: { kind: { case: "string", value: {} } } } },
      },
      {
        name: "provider",
        kind: { case: "object", value: { schema: create(SchemaSchema, {
          id: { namespace: "stroppy.test", name: "provider", version: "v1" },
          fields: [{ name: "zone", kind: { case: "string", value: {} } }],
        }) } },
      },
    ],
  });
}

describe("LaunchFormRenderer", () => {
  it("renders every declared field kind and hides a when-gated field until its condition is met", async () => {
    const onSubmit = vi.fn();
    render(<LaunchFormRenderer schema={fieldCoverageSchema()} onSubmit={onSubmit} />);

    await waitFor(() => expect(screen.getByLabelText("db_version")).toBeInTheDocument());
    expect(screen.getByLabelText("threads")).toBeInTheDocument();
    expect(screen.getByLabelText("ratio")).toBeInTheDocument();
    expect(screen.getByLabelText("ssl")).toBeInTheDocument();
    expect(screen.getByLabelText("engine")).toBeInTheDocument();
    expect(screen.getByLabelText("nodes")).toBeInTheDocument();
    expect(screen.getByLabelText("provider.zone")).toBeInTheDocument();

    // tls_key is when-gated on ssl == true — hidden until toggled.
    expect(screen.queryByLabelText("tls_key")).not.toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("ssl"));
    await waitFor(() => expect(screen.getByLabelText("tls_key")).toBeInTheDocument());
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && npx vitest run src/components/launch-form/LaunchFormRenderer.test.tsx`
Expected: FAIL — `LaunchFormRenderer` module does not exist.

- [ ] **Step 3: Write minimal implementation**

`web/src/components/launch-form/fields.tsx` (new — one component per spec-named kind):
```tsx
import { Input } from "@/components/ui/input";
import { NumField } from "@/components/ui/num-field";
import { Switch } from "@/components/ui/switch";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Label } from "@/components/ui/label";
import type { FieldProps } from "@stroppy-io/schemapb-react";
import type { Schema_Filed } from "@stroppy-io/schemapb";

// One component per schemapb field kind the launch form covers (String/
// Int64/Double/Bool/Enum/List/Object/Ref — spec §3 D2). Each takes the
// useSchemaForm `register(path)` FieldProps plus the field's own descriptor
// (for enum labels, list item schema, nested object schema).
export function StringField({ label, field }: { label: string; field: FieldProps }) {
  return (
    <div className="flex flex-col gap-1">
      <Label htmlFor={field.name}>{label}</Label>
      <Input id={field.name} aria-label={label} value={(field.value as string) ?? ""} onChange={field.onChange} />
    </div>
  );
}

export function Int64Field({ label, field }: { label: string; field: FieldProps }) {
  return (
    <div className="flex flex-col gap-1">
      <Label htmlFor={field.name}>{label}</Label>
      <NumField id={field.name} aria-label={label} value={Number(field.value ?? 0)}
        onChange={(v) => field.onChange(BigInt(v))} />
    </div>
  );
}

export function DoubleField({ label, field }: { label: string; field: FieldProps }) {
  return (
    <div className="flex flex-col gap-1">
      <Label htmlFor={field.name}>{label}</Label>
      <NumField id={field.name} aria-label={label} step={0.01} value={Number(field.value ?? 0)}
        onChange={(v) => field.onChange(v)} />
    </div>
  );
}

export function BoolField({ label, field }: { label: string; field: FieldProps }) {
  return (
    <div className="flex items-center gap-2">
      <Switch id={field.name} aria-label={label} checked={!!field.value}
        onCheckedChange={(v) => field.onChange(v)} />
      <Label htmlFor={field.name}>{label}</Label>
    </div>
  );
}

export function EnumField({
  label, field, values,
}: { label: string; field: FieldProps; values: Record<number, string> }) {
  return (
    <div className="flex flex-col gap-1">
      <Label htmlFor={field.name}>{label}</Label>
      <Select value={String(field.value ?? "")} onValueChange={(v) => field.onChange(Number(v))}>
        <SelectTrigger id={field.name} aria-label={label}><SelectValue /></SelectTrigger>
        <SelectContent>
          {Object.entries(values).map(([n, name]) => (
            <SelectItem key={n} value={n}>{name}</SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
```

`web/src/components/launch-form/LaunchFormRenderer.tsx` (new):
```tsx
import { useSchemaForm } from "@stroppy-io/schemapb-react";
import type { Schema, BakedJson } from "@stroppy-io/schemapb";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { StringField, Int64Field, DoubleField, BoolField, EnumField } from "./fields";

export interface LaunchFormRendererProps {
  schema: Schema;
  onSubmit: (baked: BakedJson) => void;
  onInvalid?: (errors: Record<string, unknown>) => void;
  submitLabel?: string;
}

// Renders one schemapb.Schema as a controlled form via useSchemaForm — the
// launch form's only rendering surface (D2). Kinds not in the spec's list
// (float/int32/uint32/uint64/duration/timestamp/oneOf) fall back to a plain
// text field rather than being dropped silently, so an author sees SOME
// input rather than a missing field.
export function LaunchFormRenderer({ schema, onSubmit, onInvalid, submitLabel = "Launch" }: LaunchFormRendererProps) {
  const form = useSchemaForm({ schema });

  const renderField = (f: NonNullable<Schema["fields"]>[number], path: string) => {
    if (!form.fieldActive(path)) return null;
    const label = path;
    const field = form.register(path);
    switch (f.kind.case) {
      case "string":
        return <StringField key={path} label={label} field={field} />;
      case "int64":
        return <Int64Field key={path} label={label} field={field} />;
      case "double":
        return <DoubleField key={path} label={label} field={field} />;
      case "bool":
        return <BoolField key={path} label={label} field={field} />;
      case "enum":
        return <EnumField key={path} label={label} field={field} values={f.kind.value.values} />;
      case "list": {
        const count = form.listCount(path);
        return (
          <div key={path} className="flex flex-col gap-1">
            <Label>{label}</Label>
            {Array.from({ length: count || 1 }, (_, i) => renderField(f.kind.value.item!, `${path}.${i}`))}
          </div>
        );
      }
      case "object": {
        const nested = f.kind.value.schema;
        return (
          <fieldset key={path} className="flex flex-col gap-2 border border-zinc-800 p-3">
            <legend className="text-xs uppercase text-zinc-500">{label}</legend>
            {nested?.fields.map((nf) => renderField(nf, `${path}.${nf.name}`))}
          </fieldset>
        );
      }
      case "ref":
        // Refs are resolved server-side before the schema reaches the
        // browser (schemapb.Schema.Link — see D3's LaunchFormSchema
        // handler), so a Ref surviving to the renderer means an
        // unresolved identity-ref: render nothing rather than guess.
        return null;
      default:
        return <StringField key={path} label={label} field={field} />;
    }
  };

  return (
    <form
      onSubmit={form.handleSubmit(onSubmit, onInvalid)}
      className="flex flex-col gap-4"
    >
      {schema.fields.map((f) => renderField(f, f.name))}
      <Button type="submit" disabled={!form.ready}>{submitLabel}</Button>
    </form>
  );
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd web && npx vitest run src/components/launch-form/LaunchFormRenderer.test.tsx`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/launch-form/
git commit -m "feat(web): render a launch form from any schemapb.Schema"
```

---

### Task 3: Server form-schema composition + `LaunchFormSchema` RPC (D3 backend)

Add a `RecipeService.LaunchFormSchema` RPC that composes `workflow.inputs ⊕ provider.params` into one `schemapb.Schema`, reusing SP-A's derivation functions and the existing path-traversal-safe provider-module materialization (`internal/services/dsl`'s `deriveProviderSchema`/`resolveProviderName`) rather than duplicating it.

**Files:**
- Modify: `protocols/cloud/v1/api/recipe.proto` (new RPC + messages)
- Modify: `internal/services/dsl/service.go` (new exported `ComposeLaunchFormSchema`)
- Test: `internal/services/dsl/service_test.go`
- Modify: `internal/services/recipe/service.go` (`Deps.FormSchema`)
- Modify: `internal/services/recipe/recipe.go` (`LaunchFormSchema` handler)
- Test: `internal/services/recipe/recipe_test.go`
- Modify: `internal/app/run.go` (wire `Deps.FormSchema`)

**Interfaces:**
- Consumes: `schema.DeriveInputsSchema`, `schema.DeriveProviderParamsSchemapb`, `schema.ComposeFormSchema` (SP-A), `ast.DecodeWorkflow`.
- Produces:
  ```go
  // internal/services/dsl
  func ComposeLaunchFormSchema(files map[string][]byte) (*schemapb.Schema, diag.List, error)
  // internal/services/recipe
  Deps.FormSchema func(ctx context.Context, files map[string][]byte) (*schemapb.Schema, diag.List, error)
  ```

- [ ] **Step 1: Write the failing test (dsl side)**

`internal/services/dsl/service_test.go`:
```go
func TestComposeLaunchFormSchema_InputsAndProviderParams(t *testing.T) {
	files := dockerBundleFilesWithWorkflowInputs(t) // helper added below: workflow.yaml carries
	                                                 // `inputs: { db_version: {type: string, default: "16"} }`
	s, diags, err := ComposeLaunchFormSchema(files)
	require.NoError(t, err)
	require.False(t, diags.HasErrors(), diags.String())
	require.NotNil(t, s)

	byName := map[string]*schemapb.Schema_Filed{}
	for _, f := range s.GetFields() {
		byName[f.GetName()] = f
	}
	require.Contains(t, byName, "db_version", "workflow.inputs hoisted to top level")
	require.Contains(t, byName, "provider", "provider.params nested under provider")
}

// dockerBundleFilesWithWorkflowInputs extends the package's existing
// dockerBundleFiles() test fixture (used by TestComposedSchema_*) with a
// top-level `inputs:` block in workflow.yaml.
func dockerBundleFilesWithWorkflowInputs(t *testing.T) map[string][]byte {
	t.Helper()
	files := dockerBundleFiles()
	wf := files["workflow.yaml"]
	files["workflow.yaml"] = append([]byte("inputs:\n  db_version: { type: string, default: \"16\" }\n"), wf...)
	return files
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/services/dsl/ -run TestComposeLaunchFormSchema_InputsAndProviderParams -v`
Expected: FAIL — `undefined: ComposeLaunchFormSchema`.

- [ ] **Step 3: Write minimal implementation**

`internal/services/dsl/service.go` — add alongside `ComposedSchema`:
```go
// ComposeLaunchFormSchema derives and composes the launch-form schemapb.Schema
// for a bundle: workflow.yaml's top-level `inputs:` (SP-A DeriveInputsSchema)
// hoisted to the top level, plus the resolved provider's params.tf-derived
// schema (SP-A DeriveProviderParamsSchemapb) nested under "provider" (SP-A
// ComposeFormSchema). It reuses resolveProviderName/deriveProviderSchema's
// exact path-traversal-safe module materialization (see deriveProviderSchema's
// doc comment) rather than a second implementation — RecipeService.
// LaunchFormSchema (internal/services/recipe) calls this via Deps.FormSchema
// rather than importing this package directly, matching Deps.Checker's
// existing injection precedent.
//
// A bundle with no resolvable provider composes inputs-only (params nil);
// this mirrors ComposedSchema's own "no provider -> permissive" contract. A
// bundle whose workflow.yaml fails to decode, or whose provider.use is
// ambiguous, returns an error (unlike ComposedSchema's diagnostics-only
// contract) because a launch form has no partial-success rendering: either
// there is a usable Schema or there is not.
func ComposeLaunchFormSchema(files map[string][]byte) (*schemapb.Schema, diag.List, error) {
	wf, wfDiags := ast.DecodeWorkflow("workflow.yaml", files["workflow.yaml"])
	if wfDiags.HasErrors() {
		return nil, wfDiags, fmt.Errorf("decode workflow.yaml: %s", joinDiagMessages(wfDiags))
	}

	inputsSchema, inputDiags := schema.DeriveInputsSchema("stroppy.workflow.inputs", wf.Inputs)
	diags := append(diag.List{}, wfDiags...)
	diags = append(diags, inputDiags...)

	name, err := resolveProviderName(files)
	if err != nil {
		return nil, diags, fmt.Errorf("resolve provider: %w", err)
	}

	var paramsSchema *schemapb.Schema
	if name != "" {
		paramsSchema, _, paramDiags, derr := deriveProviderParamsSchemapb(files, name)
		diags = append(diags, paramDiags...)
		if derr != nil {
			return nil, diags, fmt.Errorf("derive provider %q params: %w", name, derr)
		}
		_ = paramsSchema // assigned below; kept in a nested scope to avoid shadow-lint noise
	}

	form, err := schema.ComposeFormSchema("stroppy.form."+name, inputsSchema, paramsSchema)
	if err != nil {
		return nil, diags, fmt.Errorf("compose form schema: %w", err)
	}
	return form, diags, nil
}

// deriveProviderParamsSchemapb bridges the bundle's raw files to
// schema.DeriveProviderParamsSchemapb the same way deriveProviderSchema
// bridges to the jsonschema-era schema.DeriveParamsSchema: materialize
// providers/<name>/module/**'s bytes into a private temp dir (reusing the
// exact path-traversal guard deriveProviderSchema already implements), then
// call the schemapb deriver against that directory.
func deriveProviderParamsSchemapb(files map[string][]byte, name string) (params, ext *schemapb.Schema, diags diag.List, err error) {
	tmp, cleanup, err := materializeProviderModule(files, name) // extracted from deriveProviderSchema's body (Step 3b)
	if err != nil {
		return nil, nil, diags, err
	}
	defer cleanup()
	params, ext, diags = schema.DeriveProviderParamsSchemapb(tmp, name)
	return params, ext, diags, nil
}
```

Step 3b (refactor, no behavior change): extract `deriveProviderSchema`'s temp-dir-materialize-and-clean loop (`internal/services/dsl/service.go:451-495`, the `os.MkdirTemp`/path-traversal-guard/`os.WriteFile` block) into a shared helper `materializeProviderModule(files map[string][]byte, name string) (dir string, cleanup func(), err error)`, and make `deriveProviderSchema` call it too. This is a pure refactor — `deriveProviderSchema`'s existing tests must still pass unchanged.

Run `go test ./internal/services/dsl/... -run TestComposeLaunchFormSchema_InputsAndProviderParams -v` should now PASS. Then run the full package (`go test ./internal/services/dsl/...`) to confirm the `deriveProviderSchema` refactor didn't regress `TestComposedSchema_*`.

- [ ] **Step 4: Proto — new RPC**

`protocols/cloud/v1/api/recipe.proto` — add import and message/rpc:
```protobuf
import "schemapb/schema.proto";
```
```protobuf
message LaunchFormSchemaRequest {
    string tenant_id = 1 [(validate.rules).string = {min_len: 1, max_len: 64}];
    string recipe_id = 2 [(validate.rules).string = {min_len: 1, max_len: 64}];
}
message LaunchFormSchemaResponse {
    schemapb.Schema schema = 1;
}
```
inside `service RecipeService { ... }`:
```protobuf
    rpc LaunchFormSchema (LaunchFormSchemaRequest) returns (LaunchFormSchemaResponse) {
        option (ogen.method) = {
            http_method: HTTP_METHOD_GET
            path: "/launch-form-schema"
            operation_id: "launchFormSchema"
            request_body: { required: true }
            responses: { status: 200 description: "OK" }
            responses: { description: "Unexpected error." schema: { ref: "#/components/schemas/Error" } }
        };
        option idempotency_level = NO_SIDE_EFFECTS;
        option (cloud.v1.iam.auth) = {all_of: [{resource: RESOURCE_RECIPE, action: ACTION_READ}]};
    }
```

Regenerate: `make proto-tools protocols` (per root `Makefile`'s `protocols` target — regenerates `internal/proto/**` and `web/src/lib/proto/**`).

- [ ] **Step 5: Write failing test (recipe side) + implement handler**

`internal/services/recipe/recipe_test.go`:
```go
func TestLaunchFormSchema_ComposesFormSchema(t *testing.T) {
	repo := newFakeRecipeRepo()
	repo.put(&models.RecipeRecord{
		Entity: &common.Entity{Id: "r1", TenantId: "t1"},
		Bundle: &models.RecipeBundle{Files: dockerBundleFilesWithInputs()},
	})
	svc := NewService(Deps{
		Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil),
		Runs: newFakeRunRepo(), Workflows: newFakeRecipeWorkflows(nil),
		FormSchema: func(_ context.Context, files map[string][]byte) (*schemapb.Schema, diag.List, error) {
			return dslservice.ComposeLaunchFormSchema(files)
		},
	})

	resp, err := svc.LaunchFormSchema(context.Background(), &api.LaunchFormSchemaRequest{TenantId: "t1", RecipeId: "r1"})
	require.NoError(t, err)
	require.NotNil(t, resp.GetSchema())
	require.NotEmpty(t, resp.GetSchema().GetFields())
}
```

`internal/services/recipe/service.go` — add to `Deps`:
```go
	// FormSchema composes the launch-form schemapb.Schema for a bundle's
	// files (workflow.inputs ⊕ provider.params). In production this is
	// internal/services/dsl.ComposeLaunchFormSchema, injected for the same
	// reason Checker is: reuse the path-traversal-safe provider-module
	// derivation living in internal/services/dsl instead of duplicating it.
	// Required for LaunchFormSchema; every other handler works without it.
	FormSchema func(ctx context.Context, files map[string][]byte) (*schemapb.Schema, diag.List, error)
```

`internal/services/recipe/recipe.go` — add handler:
```go
// LaunchFormSchema composes and returns the launch-form schemapb.Schema for
// the tenant's stored recipe bundle. Read-only. Unlike CheckRecipe/Preview,
// a schema-composition failure (unresolvable provider, unparseable
// workflow.yaml) IS an RPC error (InvalidArgument): LaunchFormSchemaResponse
// carries no diagnostics channel, mirroring DslService.ComposedSchema's own
// contract for the same reason.
func (s *Service) LaunchFormSchema(ctx context.Context, req *api.LaunchFormSchemaRequest) (*api.LaunchFormSchemaResponse, error) {
	if err := requireTenant(req.GetTenantId()); err != nil {
		return nil, err
	}
	rec, err := s.d.Repo.Get(ctx, req.GetTenantId(), req.GetId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	form, diags, err := s.d.FormSchema(ctx, rec.GetBundle().GetFiles())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "compose launch form schema: %v", err)
	}
	if diags.HasErrors() {
		return nil, status.Errorf(codes.InvalidArgument, "compose launch form schema: %s", diags.String())
	}
	return &api.LaunchFormSchemaResponse{Schema: form}, nil
}
```

`internal/app/run.go` — wire the new dep alongside the existing `Checker: dslService.CheckBundle` (line 470):
```go
		Checker:    dslService.CheckBundle,
		FormSchema: dslservice.ComposeLaunchFormSchema,
```

- [ ] **Step 6: Run tests to verify pass**

Run: `go test ./internal/services/dsl/... ./internal/services/recipe/... ./internal/app/... -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add protocols/cloud/v1/api/recipe.proto internal/proto web/src/lib/proto \
  internal/services/dsl/service.go internal/services/dsl/service_test.go \
  internal/services/recipe/service.go internal/services/recipe/recipe.go internal/services/recipe/recipe_test.go \
  internal/app/run.go
git commit -m "feat(recipe): add LaunchFormSchema RPC composing workflow.inputs+provider.params"
```

---

### Task 4: Web `fetchLaunchFormSchema` + `LaunchForm` page (D3 frontend, D2 assembly)

Wire Task 3's RPC and Task 2's renderer into a real page, reached from `RecipeEditor`'s existing Run button.

**Files:**
- Modify: `web/src/services/recipe.ts` (`fetchLaunchFormSchema`)
- Test: `web/src/services/recipe.test.ts`
- Create: `web/src/pages/LaunchForm.tsx`
- Modify: `web/src/App.tsx` (route)
- Modify: `web/src/pages/RecipeEditor.tsx` (`onRun` navigates to the new page instead of calling `startRun` directly)

**Interfaces:**
- Consumes: `recipeClient.launchFormSchema` (generated from Task 3's proto), `LaunchFormRenderer` (Task 2).
- Produces:
  ```ts
  export async function fetchLaunchFormSchema(tenantSlug: string, recipeId: string): Promise<Schema>
  ```

- [ ] **Step 1: Write the failing test**

`web/src/services/recipe.test.ts`:
```ts
import { describe, it, expect, vi } from "vitest";
import { create } from "@bufbuild/protobuf";
import { SchemaSchema } from "@stroppy-io/schemapb";

vi.mock("@/services/client", () => ({
  recipeClient: {
    launchFormSchema: vi.fn().mockResolvedValue({
      schema: create(SchemaSchema, { id: { namespace: "ns", name: "form", version: "v1" }, fields: [] }),
    }),
  },
  dslClient: {},
}));
vi.mock("@/services/tenant", () => ({ resolveTenantId: vi.fn().mockResolvedValue("t1") }));

import { fetchLaunchFormSchema } from "./recipe";

describe("fetchLaunchFormSchema", () => {
  it("resolves the tenant and returns the composed schema", async () => {
    const schema = await fetchLaunchFormSchema("acme", "r1");
    expect(schema.id?.name).toBe("form");
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && npx vitest run src/services/recipe.test.ts`
Expected: FAIL — `fetchLaunchFormSchema` is not exported.

- [ ] **Step 3: Write minimal implementation**

`web/src/services/recipe.ts` — add near `composedSchema`:
```ts
import type { Schema } from "@stroppy-io/schemapb";

/** RecipeService.LaunchFormSchema — the schemapb form schema for a stored recipe's launch form. */
export async function fetchLaunchFormSchema(tenantSlug: string, recipeId: string): Promise<Schema> {
  const tenantId = await resolveTenantId(tenantSlug);
  const { schema } = await recipeClient.launchFormSchema({ tenantId, recipeId });
  if (!schema) throw new Error("launchFormSchema returned no schema");
  return schema;
}
```

`web/src/pages/LaunchForm.tsx` (new — mirrors `RecipeEditor.tsx`'s loading/error chrome):
```tsx
// LaunchForm — /recipes/:id/launch. Fetches the composed schemapb form
// schema (D3) and renders it (D2); on submit, calls startRun with the
// baked Filled values (D3 §StartRun) and navigates to the new run, exactly
// like RecipeEditor's onRun does for the no-inputs path.
import { useEffect, useState, useCallback } from "react";
import { AlertCircle, ArrowLeft, Loader2 } from "lucide-react";
import { useNavigate, useParams, useTenantSlug } from "@/lib/router";
import { LaunchFormRenderer } from "@/components/launch-form/LaunchFormRenderer";
import { fetchLaunchFormSchema, startRun } from "@/services/recipe";
import type { Schema, BakedJson } from "@stroppy-io/schemapb";

export function LaunchForm() {
  const { id } = useParams<{ id: string }>();
  const slug = useTenantSlug();
  const navigate = useNavigate();

  const [schema, setSchema] = useState<Schema | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [launching, setLaunching] = useState(false);
  const [launchError, setLaunchError] = useState<string | null>(null);

  useEffect(() => {
    if (!slug || !id) return;
    setLoading(true);
    fetchLaunchFormSchema(slug, id)
      .then(setSchema)
      .catch((e) => setLoadError(e instanceof Error ? e.message : String(e)))
      .finally(() => setLoading(false));
  }, [slug, id]);

  const onSubmit = useCallback(
    async (baked: BakedJson) => {
      if (!slug || !id) return;
      setLaunching(true);
      setLaunchError(null);
      try {
        const runId = await startRun(slug, id, baked);
        navigate(`/runs/${runId}`);
      } catch (e) {
        setLaunchError(e instanceof Error ? e.message : String(e));
        setLaunching(false);
      }
    },
    [slug, id, navigate],
  );

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-zinc-500">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" /> Loading launch form…
      </div>
    );
  }
  if (loadError || !schema) {
    return (
      <div className="p-5">
        <div className="flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
          <AlertCircle className="h-4 w-4" /> {loadError ?? "no form schema"}
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center gap-3 border-b border-zinc-800/80 px-5 py-3">
        <button
          type="button"
          onClick={() => navigate(`/recipes/${id}`)}
          className="flex h-7 w-7 items-center justify-center border border-zinc-800 text-zinc-500 hover:border-zinc-700 hover:text-zinc-300"
          aria-label="Back"
        >
          <ArrowLeft className="h-4 w-4" />
        </button>
        <div className="text-sm font-semibold">Launch</div>
      </div>
      <div className="max-w-xl p-5">
        {launchError && (
          <div className="mb-4 flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
            <AlertCircle className="h-4 w-4" /> {launchError}
          </div>
        )}
        <LaunchFormRenderer schema={schema} onSubmit={onSubmit} submitLabel={launching ? "Launching…" : "Launch"} />
      </div>
    </div>
  );
}
```

`web/src/App.tsx` — add the route next to `recipes/:id`:
```tsx
              <Route path="recipes/:id/launch" element={<LaunchForm />} />
```
(with `import { LaunchForm } from "@/pages/LaunchForm";` alongside the existing `RecipeEditor` import.)

`web/src/pages/RecipeEditor.tsx` — `onRun` navigates to the launch form instead of calling `startRun(slug, id)` directly (bundle inputs now require a form pass, even when the form composes to zero required fields — the form still needs to run `BakeForm` to seal a `Baked` snapshot):
```tsx
  const onRun = useCallback(() => {
    if (!slug || !id) return;
    navigate(`/recipes/${id}/launch`);
  }, [slug, id, navigate]);
```
(Drop the now-unused `startRun` import from this file — Task 5's `startRun(slug, id, baked)` is called from `LaunchForm.tsx` instead.)

- [ ] **Step 4: Run test to verify it passes**

Run: `cd web && npx vitest run src/services/recipe.test.ts && npm run build`
Expected: PASS; `tsc -b` (part of `npm run build`) catches any stale `startRun` import left in `RecipeEditor.tsx`.

- [ ] **Step 5: Commit**

```bash
git add web/src/services/recipe.ts web/src/services/recipe.test.ts \
  web/src/pages/LaunchForm.tsx web/src/App.tsx web/src/pages/RecipeEditor.tsx
git commit -m "feat(web): launch-form page wired from recipe editor's Run button"
```

---

### Task 5: Typed `resolved_inputs` runtime (I1 groundwork)

`dslrun.go`'s own doc comment (`internal/workflows/dslrun.go:289-298`) names this exact limitation: `CompiledJob.ResolvedInputs` is `map<string,string>`, stringified via `fmt.Sprint` in `lower.go`'s `jobInputFields`, so `when: inputs.count > 0` against an int-typed input errors at CEL eval time. This task switches the wire type to `google.protobuf.Value` (which every already-typed Go value — from `BoundComponent.Inputs map[string]any`, itself populated by `include.checkInputType`'s typed binding — converts to directly via `structpb.NewValue`).

**Files:**
- Modify: `protocols/cloud/v1/dsl/compiled.proto`
- Modify: `internal/dsl/lower/lower.go`
- Test: `internal/dsl/lower_test.go`
- Modify: `internal/workflows/dslrun.go`
- Test: `internal/workflows/dslrun_test.go` (or equivalent existing test file)

**Interfaces:**
- Consumes: `google.golang.org/protobuf/types/known/structpb`.
- Produces: `CompiledJob.ResolvedInputs map[string]*structpb.Value` (was `map[string]string`).

- [ ] **Step 1: Write the failing test**

`internal/dsl/lower_test.go` (extend the existing component-inputs test, or add):
```go
func TestJobInputFields_TypedScalarsNotStringified(t *testing.T) {
	// A component input declared as `{type: int}` bound to 3 (a Go int, per
	// include.checkInputType's "int" case) must lower to a structpb NumberValue,
	// not the string "3" (see dslrun.go's documented I1 limitation).
	components := []include.BoundComponent{{
		Name: "ha",
		Doc:  &ast.ComponentDoc{Inputs: map[string]ast.InputSpec{"count": {Type: "int"}}},
		Inputs: map[string]any{"count": 3},
	}}
	resolvedInputs, _, _ := jobInputFields("ha/install", components) // jobInputFields stays unexported; test lives in package lower

	v, ok := resolvedInputs["count"]
	require.True(t, ok)
	require.Equal(t, float64(3), v.GetNumberValue(), "typed int must round-trip as a structpb NumberValue, not a stringified scalar")
}
```
(This test lives in `package lower` — an internal test, matching `lower.go`'s own package; if `lower_test.go` today is `package lower_test`, add a same-package `lower_internal_test.go` instead, since `jobInputFields` is unexported.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/dsl/lower/... -run TestJobInputFields_TypedScalarsNotStringified -v`
Expected: FAIL — `resolvedInputs["count"]` is a `string`, `.GetNumberValue()` does not compile against `map[string]string`.

- [ ] **Step 3: Write minimal implementation**

`protocols/cloud/v1/dsl/compiled.proto`:
```protobuf
import "google/protobuf/struct.proto";
```
```protobuf
    // resolved_inputs are the component's scalar inputs (int/string/bool),
    // typed, resolved at compile time, bound as CEL `inputs.<name>` at
    // runtime via google.protobuf.Value.AsInterface(). Empty for a job that
    // did not originate from an include component.
    map<string, google.protobuf.Value> resolved_inputs = 7;
```
Regenerate: `make protocols`.

`internal/dsl/lower/lower.go` — `jobInputFields` (lines 269-309):
```go
func jobInputFields(jobID string, components []include.BoundComponent) (resolvedInputs map[string]*structpb.Value, inputGroups map[string]string, targetGroup string) {
	bc := componentForJob(baseJobName(jobID), components)
	if bc == nil {
		return nil, nil, ""
	}

	resolvedInputs = map[string]*structpb.Value{}
	inputGroups = map[string]string{}
	for inputName, spec := range bc.Doc.Inputs {
		val, has := bc.Inputs[inputName]
		if !has {
			continue
		}
		if spec.Type == "machine_group" {
			if group, ok := val.(string); ok {
				inputGroups[inputName] = group
			}
			continue
		}
		v, err := structpb.NewValue(val)
		if err != nil {
			// Unrepresentable scalar (should not happen for the int/string/
			// bool types checkInputType binds) — skip rather than fail the
			// whole job, matching this function's existing permissive contract.
			continue
		}
		resolvedInputs[inputName] = v
	}

	if len(inputGroups) == 1 {
		for _, group := range inputGroups {
			targetGroup = group
		}
	}

	return resolvedInputs, inputGroups, targetGroup
}
```
Add `"google.golang.org/protobuf/types/known/structpb"` to `lower.go`'s imports; update the `cj := &dslpb.CompiledJob{... ResolvedInputs: resolvedInputs, ...}` assembly (line ~251) — no change needed there, the field type now matches directly.

`internal/workflows/dslrun.go` — `baseJobVars` (lines 299-322):
```go
	inputs := make(map[string]any, len(job.GetResolvedInputs())+len(job.GetInputGroups()))
	for name, val := range job.GetResolvedInputs() {
		inputs[name] = val.AsInterface() // typed: float64/string/bool/nil/[]any/map[string]any
	}
```
Update the doc comment above `baseJobVars` (lines 289-298) to remove the "KNOWN LIMITATION" paragraph — it is now typed. Update `dslrun.go`'s package-doc bullet on `ResolvedInputs` (lines 274-275) accordingly.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/dsl/... ./internal/workflows/... -v`
Expected: PASS. Also re-run the full suite (`go test ./... -count=1`) since `compiled.proto`'s field type change is a breaking wire change touching every `CompiledJob` construction site in tests.

- [ ] **Step 5: Commit**

```bash
git add protocols/cloud/v1/dsl/compiled.proto internal/proto \
  internal/dsl/lower/lower.go internal/dsl/lower_test.go \
  internal/workflows/dslrun.go internal/workflows/dslrun_test.go
git commit -m "fix(dsl): resolved_inputs carries typed google.protobuf.Value, not stringified scalars"
```

---

### Task 6: `include.Resolve` workflow-input threading + `ApplyBakedInputs` (D4 groundwork)

Two gaps found during planning (not in the spec's sketch, discovered by tracing `include.Resolve`):
1. `Resolve`'s top-level call (`internal/dsl/compiler.go:83`) passes `boundInputs: nil` for `workflow.yaml`'s own jobs — so a top-level `include: ..., inputs: {x: "${{ inputs.y }}"}` referencing a workflow-level input can never resolve today, even statically. `Resolve` already has the exact machinery for this (`forwardInputs`/`substituteOn` against `fragmentCtx.boundInputs`) for *nested* includes; it just isn't wired at the top level.
2. Provider-param values (`cluster.Provider.Params map[string]any`, consumed by `lower.go`'s `lowerProvider`) are a completely different field from `BoundComponent.Inputs` — the spec's `ApplyBakedInputs(resolved *include.Resolved, baked *schemapb.Baked) error` signature is kept, but its body targets `resolved.Cluster.Provider.Params`, not `BoundComponent.Inputs` (see Self-Review).

**Files:**
- Modify: `internal/dsl/include/resolve.go` (`Resolve` gains a `workflowInputs map[string]any` parameter)
- Test: `internal/dsl/include/resolve_test.go`
- Create: `internal/dsl/schema/baked_apply.go`
- Test: `internal/dsl/schema/baked_apply_test.go`
- Modify: `internal/dsl/compiler.go` (`Compile` threads `in.Baked` through both)

**Interfaces:**
- Produces:
  ```go
  // internal/dsl/include
  func Resolve(cluster *ast.ClusterDoc, wf *ast.WorkflowDoc, src Sources, workflowInputs map[string]any) (*Resolved, diag.List)
  // internal/dsl/schema
  func SplitBakedValues(baked *schemapb.Baked) (workflowInputs, providerParams map[string]any)
  func ApplyBakedInputs(resolved *include.Resolved, baked *schemapb.Baked) error
  ```

- [ ] **Step 1: Write the failing test (Resolve threading)**

`internal/dsl/include/resolve_test.go` — extend with a workflow-level forwarding case:
```go
func TestResolve_TopLevelWorkflowInputForwarding(t *testing.T) {
	cluster := testCluster(t)
	wf := testWorkflow(t, `
jobs:
  ha:
    include: components/etcd
    inputs: { nodes: "${{ inputs.target_group }}" }
`)
	resolved, diags := Resolve(cluster, wf, Sources{Files: map[string][]byte{
		"components/etcd/component.yaml": []byte(etcdComponentYAML),
	}}, map[string]any{"target_group": "db"})

	require.False(t, diags.HasErrors(), diags.String())
	require.Len(t, resolved.Components, 1)
	require.Equal(t, "db", resolved.Components[0].Inputs["nodes"], "workflow-level input forwarded into the top-level include job")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/dsl/include/... -run TestResolve_TopLevelWorkflowInputForwarding -v`
Expected: FAIL — `Resolve` takes 3 args, not 4; once the signature below is added mechanically without wiring `boundInputs`, the test fails with `"include (job \"ha\"): input \"nodes\": unknown outer input \"target_group\""` (the exact `forwardInputs` error path).

- [ ] **Step 3: Write minimal implementation**

`internal/dsl/include/resolve.go` — `Resolve`'s signature and its one internal call:
```go
func Resolve(cluster *ast.ClusterDoc, wf *ast.WorkflowDoc, src Sources, workflowInputs map[string]any) (*Resolved, diag.List) {
	r := &resolver{
		src:        src,
		cluster:    cluster,
		diags:      &diag.List{},
		out:        map[string]ast.Job{},
		jobOrigins: map[string]jobOrigin{},
	}
	r.expandFragment(wf.Jobs, "", nil, map[string]bool{}, fragmentCtx{
		path:        "workflow.yaml",
		boundInputs: workflowInputs,
	})

	return &Resolved{
		Cluster:    cluster,
		Jobs:       r.out,
		Components: r.comps,
	}, *r.diags
}
```
(Only the `fragmentCtx{...}` literal changes — `boundInputs: workflowInputs` instead of the implicit zero value. `forwardInputs`/`substituteOn` need no changes: they already read `outerCtx.boundInputs` generically, whichever level it comes from.)

Update every other call site in `internal/dsl/include/resolve_test.go` to pass a trailing `nil` (reproduces today's exact behavior for every existing test that doesn't test workflow-level forwarding — a mechanical find/replace of `Resolve(cluster, wf, Sources{...})` to `Resolve(cluster, wf, Sources{...}, nil)`).

`internal/dsl/compiler.go` — `Compile`'s one call site (line 83):
```go
	workflowInputs, _ := schema.SplitBakedValues(in.Baked) // nil-safe: SplitBakedValues(nil) returns nil, nil
	resolved, resolveDiags := include.Resolve(cluster, wf, in.Sources, workflowInputs)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/dsl/include/... ./internal/dsl/... -v`
Expected: PASS (including every pre-existing `Resolve(...)` call site updated with a trailing `nil`).

- [ ] **Step 5: Write the failing test (ApplyBakedInputs)**

`internal/dsl/schema/baked_apply_test.go` (new):
```go
package schema_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	spb "github.com/stroppy-io/schemapb/schemapb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/include"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/schema"
)

func TestApplyBakedInputs_OverlaysProviderParams(t *testing.T) {
	cluster := &ast.ClusterDoc{Provider: ast.ProviderUse{Use: "yandex", Params: map[string]any{"zone": "ru-central1-a"}}}
	resolved := &include.Resolved{Cluster: cluster}

	values, err := structpb.NewStruct(map[string]any{
		"db_version": "17",
		"provider":   map[string]any{"replicas": float64(5)},
	})
	require.NoError(t, err)
	baked := &spb.Baked{Values: values}

	require.NoError(t, schema.ApplyBakedInputs(resolved, baked))
	require.Equal(t, "ru-central1-a", cluster.Provider.Params["zone"], "existing static param untouched")
	require.Equal(t, float64(5), cluster.Provider.Params["replicas"], "baked provider value overlaid")
}

func TestSplitBakedValues_NilSafe(t *testing.T) {
	wfInputs, providerParams := schema.SplitBakedValues(nil)
	require.Nil(t, wfInputs)
	require.Nil(t, providerParams)
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/dsl/schema/... -run 'TestApplyBakedInputs_OverlaysProviderParams|TestSplitBakedValues_NilSafe' -v`
Expected: FAIL — `undefined: ApplyBakedInputs`, `undefined: SplitBakedValues`.

- [ ] **Step 7: Write minimal implementation**

`internal/dsl/schema/baked_apply.go` (new):
```go
// Package schema (this file): the launch-time bridge from a sealed
// schemapb.Baked launch form to the two places its typed values actually
// flow before/around compilation — see docs/superpowers/plans/
// 2026-07-08-sp-d-launch-form.md Task 6/8 for why this is NOT a
// BoundComponent.Inputs overlay (the spec's original sketch): provider
// params live on ast.ClusterDoc.Provider.Params (consumed by lower.go's
// lowerProvider), a completely different field from an include's bound
// component inputs (which are already typed and forwarded during
// include.Resolve itself, via the workflowInputs parameter Resolve now
// takes — see internal/dsl/include/resolve.go).
package schema

import (
	"fmt"

	spb "github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/include"
)

// SplitBakedValues splits a Baked launch form's values into the two
// destinations ComposeFormSchema's own layout implies (SP-A: workflow
// inputs hoisted to the top level, provider params nested under
// "provider" — see form.go's ComposeFormSchema): workflowInputs is
// everything except the "provider" key (to feed include.Resolve's
// workflowInputs parameter), providerParams is the "provider" object's
// contents (to overlay onto ast.ClusterDoc.Provider.Params). A nil baked
// (no form submitted — e.g. a non-form launch path) returns (nil, nil).
func SplitBakedValues(baked *spb.Baked) (workflowInputs, providerParams map[string]any) {
	if baked.GetValues() == nil {
		return nil, nil
	}
	all := baked.GetValues().AsMap()
	if p, ok := all["provider"].(map[string]any); ok {
		providerParams = p
	}
	workflowInputs = make(map[string]any, len(all))
	for k, v := range all {
		if k == "provider" {
			continue
		}
		workflowInputs[k] = v
	}
	return workflowInputs, providerParams
}

// ApplyBakedInputs overlays baked's provider-param values onto
// resolved.Cluster.Provider.Params (baked values win over cluster.yaml's
// static provider.params — the launch form is the source of truth for a
// form-driven launch). It is a no-op (nil error) when baked is nil, so
// callers can invoke it unconditionally on the non-form launch path.
//
// Workflow-level input values are NOT applied here: they must be known
// BEFORE/DURING include.Resolve (its `${{ inputs.x }}` substitution runs
// inline during Resolve, not as a post-process — see resolve.go's
// forwardInputs/substituteOn), so callers thread them via
// SplitBakedValues + include.Resolve's workflowInputs parameter instead
// (see internal/dsl/compiler.go's Compile).
func ApplyBakedInputs(resolved *include.Resolved, baked *spb.Baked) error {
	if baked == nil {
		return nil
	}
	if resolved == nil || resolved.Cluster == nil {
		return fmt.Errorf("apply baked inputs: resolved cluster is required")
	}
	_, providerParams := SplitBakedValues(baked)
	if len(providerParams) == 0 {
		return nil
	}
	if resolved.Cluster.Provider.Params == nil {
		resolved.Cluster.Provider.Params = make(map[string]any, len(providerParams))
	}
	for k, v := range providerParams {
		resolved.Cluster.Provider.Params[k] = v
	}
	return nil
}
```

- [ ] **Step 8: Run tests to verify pass**

Run: `go test ./internal/dsl/schema/... ./internal/dsl/include/... ./internal/dsl/... -v`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/dsl/include/resolve.go internal/dsl/include/resolve_test.go \
  internal/dsl/compiler.go internal/dsl/schema/baked_apply.go internal/dsl/schema/baked_apply_test.go
git commit -m "feat(dsl): thread baked launch-form values into Resolve + provider params"
```

---

### Task 7: `StartRun` accepts `Filled`, synchronous `BakeForm` validation (D4 part 1)

The field-error feedback loop is pure — `BakeForm` only needs the form `schemapb.Schema` (Task 3's composition, no bundle compile required) — so it runs synchronously in the connect-rpc handler, exactly like the spec describes. Only the *compiled-plan substitution* (Task 8) has to move to the async activity.

**Files:**
- Modify: `protocols/cloud/v1/api/recipe.proto` (`StartRunRequest.filled`, `StartRunResponse.field_errors`)
- Modify: `internal/services/recipe/service.go` (`RecipeWorkflows.LaunchRecipeRun` gains a `baked` param)
- Modify: `internal/services/recipe/recipe.go` (`StartRun`)
- Test: `internal/services/recipe/recipe_test.go`
- Modify: `web/src/services/recipe.ts` (`startRun(tenantSlug, recipeId, filled?)`)

**Interfaces:**
- Produces:
  ```go
  type RecipeWorkflows interface {
      LaunchRecipeRun(ctx context.Context, run *models.TestRunRecord, bundle map[string][]byte, baked *schemapb.Baked) error
      CancelRecipeRun(ctx context.Context, runID string) error
  }
  ```
  ```ts
  export async function startRun(tenantSlug: string, recipeId: string, filled?: BakedJson): Promise<string>
  ```

- [ ] **Step 1: Write the failing test**

`internal/services/recipe/recipe_test.go`:
```go
func TestStartRun_FilledBakeFailure_NoRunMinted(t *testing.T) {
	repo := newFakeRecipeRepo()
	repo.put(&models.RecipeRecord{Entity: &common.Entity{Id: "r1", TenantId: "t1"}, Bundle: &models.RecipeBundle{Files: dockerBundleFilesWithInputs()}})
	runs := newFakeRunRepo()
	svc := NewService(Deps{
		Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil),
		Runs: runs, Workflows: newFakeRecipeWorkflows(nil),
		FormSchema: func(_ context.Context, files map[string][]byte) (*schemapb.Schema, diag.List, error) {
			return dslservice.ComposeLaunchFormSchema(files)
		},
	})

	badValues, err := structpb.NewStruct(map[string]any{"threads": float64(-1)}) // violates a Gte(1) rule
	require.NoError(t, err)

	resp, err := svc.StartRun(context.Background(), &api.StartRunRequest{
		TenantId: "t1", RecipeId: "r1",
		Filled: &schemapb.Filled{Values: badValues},
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Nil(t, resp)
	require.Empty(t, runs.all(), "no run record minted on a field-error bake")
}

func TestStartRun_FilledBakeSuccess_LaunchesWithBaked(t *testing.T) {
	repo := newFakeRecipeRepo()
	repo.put(&models.RecipeRecord{Entity: &common.Entity{Id: "r1", TenantId: "t1"}, Bundle: &models.RecipeBundle{Files: dockerBundleFilesWithInputs()}})
	workflows := newFakeRecipeWorkflows(nil)
	svc := NewService(Deps{
		Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil),
		Runs: newFakeRunRepo(), Workflows: workflows,
		FormSchema: func(_ context.Context, files map[string][]byte) (*schemapb.Schema, diag.List, error) {
			return dslservice.ComposeLaunchFormSchema(files)
		},
	})

	values, err := structpb.NewStruct(map[string]any{"threads": float64(4)})
	require.NoError(t, err)

	resp, err := svc.StartRun(context.Background(), &api.StartRunRequest{
		TenantId: "t1", RecipeId: "r1",
		Filled: &schemapb.Filled{Values: values},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.GetRun())
	require.Empty(t, resp.GetFieldErrors())
	require.NotNil(t, workflows.lastBaked, "Baked snapshot threaded into LaunchRecipeRun")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/services/recipe/... -run 'TestStartRun_Filled' -v`
Expected: FAIL — `StartRunRequest` has no `Filled` field, `newFakeRecipeWorkflows`'s `LaunchRecipeRun` fake has the old 3-arg signature.

- [ ] **Step 3: Proto**

`protocols/cloud/v1/api/recipe.proto`:
```protobuf
message StartRunRequest {
    string tenant_id = 1 [(validate.rules).string = {min_len: 1, max_len: 64}];
    string recipe_id = 2 [(validate.rules).string = {min_len: 1, max_len: 64}];
    // filled is the launch form's submitted values (schemapb.Filled), sealed
    // browser-side by the WASM engine's Bake and re-baked server-side
    // (BakeForm) before launch. Optional: absent means "launch with the
    // bundle's static provider.params/no workflow inputs" (today's behavior).
    schemapb.Filled filled = 3;
}
message StartRunResponse {
    models.TestRunRecord run = 1;
    // field_errors is non-empty exactly when filled failed BakeForm — the
    // run is NOT created in that case; run is unset.
    repeated schemapb.FieldError field_errors = 2;
}
```
Regenerate: `make protocols`.

- [ ] **Step 4: Write minimal implementation**

`internal/services/recipe/service.go` — `RecipeWorkflows`:
```go
type RecipeWorkflows interface {
	// baked is the sealed launch-form snapshot (nil for a non-form launch,
	// e.g. RecipeEditor's legacy no-inputs path if one still exists) —
	// threaded through to RunRecipeWorkflow so CompileRecipeActivity can
	// apply it before lowering (see internal/dsl/schema.ApplyBakedInputs).
	LaunchRecipeRun(ctx context.Context, run *models.TestRunRecord, bundle map[string][]byte, baked *schemapb.Baked) error
	CancelRecipeRun(ctx context.Context, runID string) error
}
```

`internal/services/recipe/recipe.go` — `StartRun`, insert the bake step before `s.d.Runs.Create`:
```go
func (s *Service) StartRun(ctx context.Context, req *api.StartRunRequest) (*api.StartRunResponse, error) {
	if err := requireTenant(req.GetTenantId()); err != nil {
		return nil, err
	}
	if req.GetRecipeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "recipe_id is required")
	}
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}

	recipeRec, err := s.d.Repo.Get(ctx, req.GetTenantId(), req.GetRecipeId())
	if err != nil {
		return nil, utils.MapErr(err)
	}

	var baked *schemapb.Baked
	if filled := req.GetFilled(); filled != nil {
		form, diags, ferr := s.d.FormSchema(ctx, recipeRec.GetBundle().GetFiles())
		if ferr != nil {
			return nil, status.Errorf(codes.InvalidArgument, "compose launch form schema: %v", ferr)
		}
		if diags.HasErrors() {
			return nil, status.Errorf(codes.InvalidArgument, "compose launch form schema: %s", diags.String())
		}
		var fieldErrors []*schemapb.FieldError
		var berr error
		baked, fieldErrors, berr = schema.BakeForm(form, filled.GetValues().AsMap())
		if berr != nil {
			return nil, status.Errorf(codes.InvalidArgument, "bake launch form: %v", berr)
		}
		if len(fieldErrors) > 0 {
			// A blocking field error means Bake returned no Baked — no run
			// is minted (spec §4 StartRunResponse doc: "непусто ⇒ Bake
			// упал, run не запущен"). fieldErrors alone can also be
			// non-blocking warnings BakeForm still sealed past — only treat
			// this as fatal when baked is nil.
			if baked == nil {
				return &api.StartRunResponse{FieldErrors: fieldErrors}, status.Error(codes.InvalidArgument, "launch form failed validation")
			}
		}
	}

	runID := uuid.NewString()
	run := &models.TestRunRecord{ /* ... unchanged ... */ }

	if err := s.d.Runs.Create(ctx, run); err != nil {
		return nil, utils.MapErr(err)
	}

	if err := s.d.Workflows.LaunchRecipeRun(ctx, run, recipeRec.GetBundle().GetFiles(), baked); err != nil {
		markRunFailed(run, s.now())
		if uerr := s.d.Runs.Update(ctx, run); uerr != nil {
			return nil, utils.MapErr(uerr)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &api.StartRunResponse{Run: run}, nil
}
```
Add `"github.com/stroppy-io/stroppy-cloud/internal/dsl/schema"` and the schemapb Go import (`spb "github.com/stroppy-io/schemapb/schemapb"`, referenced above as `schemapb.Baked`/`schemapb.FieldError`) to `recipe.go`'s imports.

`web/src/services/recipe.ts` — `startRun`:
```ts
import type { BakedJson } from "@stroppy-io/schemapb";
import { fromJson } from "@bufbuild/protobuf";
import { FilledSchema } from "@stroppy-io/schemapb"; // re-exported proto message schema

export async function startRun(tenantSlug: string, recipeId: string, filled?: BakedJson): Promise<string> {
  const tenantId = await resolveTenantId(tenantSlug);
  const { run, fieldErrors } = await recipeClient.startRun({
    tenantId,
    recipeId,
    filled: filled ? fromJson(FilledSchema, { values: filled.values }) : undefined,
  });
  if (fieldErrors.length > 0) {
    throw new Error(fieldErrors.map((e) => `${e.field}: ${e.message}`).join("; "));
  }
  if (!run?.entity?.id) throw new Error("startRun returned no run");
  return run.entity.id;
}
```
(`BakedJson`'s `values` field is exactly the `Filled.values` shape — both are `structpb.Struct`-shaped JSON; `LaunchForm.tsx`'s `onSubmit(baked: BakedJson)` from Task 4 passes it straight through.)

- [ ] **Step 5: Run tests to verify pass**

Run: `go test ./internal/services/recipe/... -v` and `cd web && npx vitest run && npm run build`
Expected: PASS. Update every other `LaunchRecipeRun(ctx, run, bundle)` call site (the `internal/infrastructure/execution.RecipeWorkflows` real implementation, and every `newFakeRecipeWorkflows`/mock in tests) to the new 4-arg signature — mechanical, add `baked` param, real implementation threads it into `RunRecipeInput` (Task 8), fakes just record it (`lastBaked`, used by Step 1's test).

- [ ] **Step 6: Commit**

```bash
git add protocols/cloud/v1/api/recipe.proto internal/proto web/src/lib/proto \
  internal/services/recipe/service.go internal/services/recipe/recipe.go internal/services/recipe/recipe_test.go \
  web/src/services/recipe.ts web/src/services/recipe.test.ts
git commit -m "feat(recipe): StartRun bakes a submitted launch form before launching"
```

---

### Task 8: Thread `Baked` through `RunRecipeWorkflow` → `CompileRecipeActivity` → `dsl.Compile` (D4 part 2)

The part Task 7 deferred: the real `RecipeWorkflows.LaunchRecipeRun` implementation (`internal/infrastructure/execution/recipe_workflows.go`) must carry `baked` into the Temporal workflow input, since that is genuinely where `include.Resolve`/`lower.Lower` run (see Global Constraints).

**Files:**
- Modify: `internal/workflows/runrecipe.go` (`RunRecipeInput.Baked`, `CompileRecipeActivityInput.Baked`)
- Modify: `internal/infrastructure/execution/recipe_workflows.go` (`LaunchRecipeRun` threads `baked`)
- Modify: `internal/infrastructure/execution/recipe_activities.go` (`CompileRecipeActivity` threads `in.Baked`)
- Modify: `internal/services/dsl/service.go` (`CompileBundle` gains a `baked` param)
- Test: `internal/services/dsl/service_test.go`, `internal/infrastructure/execution/recipe_activities_test.go`, `internal/workflows/runrecipe_test.go`
- Modify: `internal/dsl/compiler.go` (`Input.Baked`, `Compile` calls `schema.ApplyBakedInputs`)

**Interfaces:**
```go
type RunRecipeInput struct {
    RunID     string
    TenantID  string
    Bundle    map[string][]byte
    Bootstrap *workflowpb.AgentBootstrap
    Baked     *schemapb.Baked // new
}
type CompileRecipeActivityInput struct {
    Bundle map[string][]byte
    Baked  *schemapb.Baked // new
}
func CompileBundle(files map[string][]byte, baked *schemapb.Baked) (*dslpb.CompiledPlan, diag.List)
```

- [ ] **Step 1: Write the failing test (dsl.Compile applies Baked)**

`internal/services/dsl/service_test.go`:
```go
func TestCompileBundle_AppliesBakedProviderParams(t *testing.T) {
	files := dockerBundleFiles()
	values, err := structpb.NewStruct(map[string]any{"provider": map[string]any{"image": "postgres:17"}})
	require.NoError(t, err)
	baked := &schemapb.Baked{Values: values}

	plan, diags := CompileBundle(files, baked)
	require.False(t, diags.HasErrors(), diags.String())
	require.Contains(t, plan.GetProvider().GetParamsJson(), `"image":"postgres:17"`, "baked provider param reached the lowered ProviderRef")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/services/dsl/... -run TestCompileBundle_AppliesBakedProviderParams -v`
Expected: FAIL — `CompileBundle(files)` takes one arg, not two.

- [ ] **Step 3: Write minimal implementation**

`internal/dsl/compiler.go` — `Input` and `Compile`:
```go
type Input struct {
	Sources  include.Sources
	Provider *ast.ProviderManifest
	Composed *jsonschema.Schema
	// Baked is the sealed launch-form snapshot from RecipeService.StartRun
	// (nil for a non-form launch). See internal/dsl/schema.ApplyBakedInputs
	// and SplitBakedValues for exactly what it overlays and when.
	Baked *schemapb.Baked
}

func Compile(in Input) (*dslpb.CompiledPlan, diag.List) {
	cluster, wf, diags := parse(in)
	if diags.HasErrors() {
		return nil, diags
	}

	workflowInputs, _ := schema.SplitBakedValues(in.Baked)
	resolved, resolveDiags := include.Resolve(cluster, wf, in.Sources, workflowInputs)
	diags = append(diags, resolveDiags...)
	if diags.HasErrors() {
		return nil, diags
	}

	if in.Baked != nil {
		if err := schema.ApplyBakedInputs(resolved, in.Baked); err != nil {
			diags.Errorf("", diag.Pos{}, "apply baked launch form: %v", err)
			return nil, diags
		}
	}

	dom, buildDiags := graph.Build(cluster, in.Provider)
	// ... unchanged from here (contract.Check, graph.Validate, graph.Expand, lower.Lower)
}
```
Add `spb "github.com/stroppy-io/schemapb/schemapb"` import to `compiler.go` (aliased `schemapb` is already used as a package-local name inside `internal/dsl/schema`, so `compiler.go` — which is `package dsl`, not `package schema` — imports it fresh; use whichever alias avoids collision with the local `schema` package import already present).

`internal/services/dsl/service.go` — `CompileBundle` and its four in-package call sites:
```go
func CompileBundle(files map[string][]byte, baked *schemapb.Baked) (*dslpb.CompiledPlan, diag.List) {
	sources := include.Sources{Files: files}
	provider, composed, diags := resolveProvider(files)
	plan, compileDiags := dsl.Compile(dsl.Input{Sources: sources, Provider: provider, Composed: composed, Baked: baked})
	diags = append(diags, compileDiags...)
	return plan, diags
}
```
Update `Check`/`Preview`/`CheckBundle` (lines 167, 182, 214, 228) to pass `nil` — none of them launch a run, so there is no `Baked` to apply; this exactly preserves today's behavior for every existing test.

`internal/workflows/runrecipe.go` — add the field and thread it into the activity call:
```go
type RunRecipeInput struct {
	RunID     string
	TenantID  string
	Bundle    map[string][]byte
	Bootstrap *workflowpb.AgentBootstrap
	Baked     *schemapb.Baked
}

type CompileRecipeActivityInput struct {
	Bundle map[string][]byte
	Baked  *schemapb.Baked
}
```
```go
func (w *runRecipeWorkflow) compileRecipe(ctx workflow.Context) (*dslpb.CompiledPlan, error) {
	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 5 * time.Minute})
	var out CompileRecipeActivityOutput
	if err := workflow.ExecuteActivity(actx, CompileRecipeActivityName, &CompileRecipeActivityInput{
		Bundle: w.in.Bundle,
		Baked:  w.in.Baked,
	}).Get(actx, &out); err != nil {
		return nil, err
	}
	// ... unchanged
}
```
(This mirrors `Bootstrap *workflowpb.AgentBootstrap`'s existing precedent of a proto-message field threaded straight through Temporal's default data converter — no new (de)serialization plumbing needed.)

`internal/infrastructure/execution/recipe_activities.go` — `CompileRecipeActivity`:
```go
	plan, diags := dslservice.CompileBundle(in.Bundle, in.Baked)
```

`internal/infrastructure/execution/recipe_workflows.go` — `LaunchRecipeRun`/`runRecipeInput`:
```go
func (w *RecipeWorkflows) LaunchRecipeRun(ctx context.Context, run *models.TestRunRecord, bundle map[string][]byte, baked *schemapb.Baked) error {
	in, err := w.runRecipeInput(ctx, run, bundle, baked)
	// ... unchanged
}

func (w *RecipeWorkflows) runRecipeInput(ctx context.Context, run *models.TestRunRecord, bundle map[string][]byte, baked *schemapb.Baked) (*workflows.RunRecipeInput, error) {
	runID := run.GetEntity().GetId()
	if runID == "" {
		return nil, errRecipeRunMissingID
	}
	in := &workflows.RunRecipeInput{
		RunID:    runID,
		TenantID: run.GetEntity().GetTenantId(),
		Bundle:   bundle,
		Baked:    baked,
	}
	// ... unchanged (Bootstrap)
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/dsl/... ./internal/services/dsl/... ./internal/services/recipe/... ./internal/workflows/... ./internal/infrastructure/execution/... -v -count=1`
Expected: PASS. This is the widest blast-radius task in the plan — every `CompileBundle`/`Compile`/`LaunchRecipeRun`/`RunRecipeInput` call site across the whole compiler+workflow stack now has one more parameter; run the FULL suite (`go test ./... -count=1`), not just the touched packages, before moving on.

- [ ] **Step 5: Integration check (manual, per spec §8)**

Against a live dev stand (docker recipe, per `project_dsl_pivot`/`project_orioledb` memory precedent): save a recipe with a workflow-level `inputs:` block and a provider param exposed on the launch form, submit the launch form with a non-default value for both, and confirm (a) the run's compiled plan actually reflects the submitted provider param (inspect `CompiledPlan.Provider.ParamsJson` via a debug log or the existing Preview panel logic), and (b) a component `when:` clause referencing a workflow-level int input evaluates correctly (not a CEL type error). This is the spec's §8 "форма реально управляет параметрами прогона" check — no automated test replaces actually watching one run.

- [ ] **Step 6: Commit**

```bash
git add internal/dsl/compiler.go internal/services/dsl/service.go internal/services/dsl/service_test.go \
  internal/workflows/runrecipe.go internal/workflows/runrecipe_test.go \
  internal/infrastructure/execution/recipe_workflows.go internal/infrastructure/execution/recipe_activities.go \
  internal/infrastructure/execution/recipe_activities_test.go
git commit -m "feat(recipe): thread baked launch-form values through RunRecipeWorkflow into the compiler"
```

---

### Task 9: Parity test — Go `BakeForm` vs WASM `schemapbBake`

Lock the invariant the whole design depends on: the browser's WASM validation and the server's Go validation must agree, over the same schema+values, for every launch. This is the SP-D-side half of SP-A Task 6's `TestFormBake_StableHash` (Go-only) — this test additionally drives the real `.wasm` artifact.

**Files:**
- Create: `web/src/components/launch-form/parity.test.ts` (drives the WASM side against a fixture)
- Create: `internal/dsl/schema/parity_wasm_fixture_test.go` (writes the SAME fixture's schema+values as JSON, for the TS test to import)
- Create: `internal/dsl/schema/testdata/parity_form.json` (shared fixture: `{schema, valid_values, invalid_values}`)

**Interfaces:**
- No new production code — this task is pure test infrastructure locking an existing contract (`schemapb.Schema.Bake` / WASM `schemapbBake` already exist from SP-A/Task 1).

- [ ] **Step 1: Write the failing test**

`internal/dsl/schema/parity_wasm_fixture_test.go` (new — generates the shared fixture from the same builders SP-A's `TestFormBake_StableHash` uses, so both sides of the parity check are provably the same schema):
```go
package schema_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	spb "github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/schema"
)

// TestWriteParityFixture regenerates testdata/parity_form.json from the same
// ComposeFormSchema call SP-A's TestFormBake_StableHash exercises, so the Go
// test and the TS parity test (web/src/components/launch-form/parity.test.ts)
// share one source of truth for "what does a real launch form look like".
// Run with -run TestWriteParityFixture to regenerate after a schema shape
// change; every other run just re-validates the checked-in fixture is fresh
// (byte-identical) rather than silently drifting.
func TestWriteParityFixture(t *testing.T) {
	inputs := spb.NewSchema("stroppy.test", "inputs", "1").
		Fields(spb.Str("db_version").Default("16"), spb.Int64("threads").Gte(1).Default(4)).MustBuild()
	params := spb.NewSchema("stroppy.test", "params", "1").
		Fields(spb.Double("replicas").Default(3)).MustBuild()
	form, err := schema.ComposeFormSchema("stroppy.test.form", inputs, params)
	require.NoError(t, err)

	schemaJSON, err := protojson.Marshal(form)
	require.NoError(t, err)

	fixture := struct {
		Schema        json.RawMessage `json:"schema"`
		ValidValues   map[string]any  `json:"valid_values"`
		InvalidValues map[string]any  `json:"invalid_values"`
	}{
		Schema:        schemaJSON,
		ValidValues:   map[string]any{"db_version": "17", "threads": float64(8), "provider": map[string]any{"replicas": float64(5)}},
		InvalidValues: map[string]any{"db_version": "17", "threads": float64(0), "provider": map[string]any{"replicas": float64(5)}}, // threads < Gte(1)
	}
	out, err := json.MarshalIndent(fixture, "", "  ")
	require.NoError(t, err)

	existing, readErr := os.ReadFile("testdata/parity_form.json")
	if readErr == nil {
		require.JSONEq(t, string(existing), string(out), "testdata/parity_form.json is stale — regenerate with -run TestWriteParityFixture")
		return
	}
	require.NoError(t, os.WriteFile("testdata/parity_form.json", out, 0o644))
}

func TestGoBakeForm_MatchesFixtureExpectations(t *testing.T) {
	raw, err := os.ReadFile("testdata/parity_form.json")
	require.NoError(t, err)
	var fixture struct {
		Schema        json.RawMessage `json:"schema"`
		ValidValues   map[string]any  `json:"valid_values"`
		InvalidValues map[string]any  `json:"invalid_values"`
	}
	require.NoError(t, json.Unmarshal(raw, &fixture))

	var form spb.Schema
	require.NoError(t, protojson.Unmarshal(fixture.Schema, &form))

	_, ferrs, err := schema.BakeForm(&form, fixture.ValidValues)
	require.NoError(t, err)
	require.Empty(t, ferrs, "Go: valid_values must bake clean")

	_, ferrs2, err2 := schema.BakeForm(&form, fixture.InvalidValues)
	require.NoError(t, err2)
	require.NotEmpty(t, ferrs2, "Go: invalid_values must fail Gte(1) on threads")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/dsl/schema/... -run 'TestWriteParityFixture|TestGoBakeForm_MatchesFixtureExpectations' -v`
Expected: FAIL — `testdata/parity_form.json` does not exist yet (first run of `TestWriteParityFixture` creates it and passes; delete it once to observe the intended "file missing" first-fail, matching the task's spirit, then let Step 2's actual verification be `TestGoBakeForm_MatchesFixtureExpectations` failing with `no such file or directory` before the fixture is generated).

- [ ] **Step 3: Generate fixture + write the TS side**

Run: `go test ./internal/dsl/schema/... -run TestWriteParityFixture -v` — creates `internal/dsl/schema/testdata/parity_form.json`.

`web/src/components/launch-form/parity.test.ts` (new):
```ts
import { describe, it, expect } from "vitest";
import { fromJson } from "@bufbuild/protobuf";
import { SchemaSchema } from "@stroppy-io/schemapb";
import { loadSchemapbEngine } from "@/services/schemapbEngine";
import fixture from "../../../../internal/dsl/schema/testdata/parity_form.json";

describe("WASM/Go BakeForm parity", () => {
  it("agrees with the Go server on the shared fixture", async () => {
    const engine = await loadSchemapbEngine();
    const schema = fromJson(SchemaSchema, JSON.parse(JSON.stringify(fixture.schema)));

    const valid = engine.bake(schema, fixture.valid_values);
    expect(valid.errors).toHaveLength(0);
    expect(valid.baked).toBeDefined();

    const invalid = engine.bake(schema, fixture.invalid_values);
    expect(invalid.errors.length).toBeGreaterThan(0);
    expect(invalid.baked).toBeUndefined();
  });
});
```
(`web/vite.config.ts`'s `resolve.alias` already maps `@` to `web/src`; the fixture is imported by a relative path crossing out of `web/` into the Go module root — confirm Vite's default `fs.allow` doesn't block reading outside `web/` during `vitest run`; if it does, add `test: { deps: { ... } }` or copy the fixture into `web/src/components/launch-form/testdata/` via a `pretest` script instead of a raw relative import — pick whichever `npx vitest run` proves works when Step 4 is executed.)

- [ ] **Step 4: Run both sides to verify pass**

Run: `go test ./internal/dsl/schema/... -v` and `cd web && npx vitest run src/components/launch-form/parity.test.ts`
Expected: PASS on both. If the cross-directory import fails Vite's fs allowlist, apply the fixture-copy fallback noted in Step 3 and re-run.

- [ ] **Step 5: Commit**

```bash
git add internal/dsl/schema/parity_wasm_fixture_test.go internal/dsl/schema/testdata/parity_form.json \
  web/src/components/launch-form/parity.test.ts
git commit -m "test(launch-form): lock Go/WASM BakeForm parity on a shared fixture"
```

---

## Self-Review

**Spec coverage (SP-D §3):**
- D1 (WASM build + web hosting) → Task 1. Vendored, not hand-rolled; resolves spec §9 open question 2 concretely (no wasm vite plugin needed — `new URL(..., import.meta.url)` + dynamic `import()` are stock Vite asset/module handling). ✓
- D2 (React form-renderer, all kinds + when + computed + enum-options + list-count) → Task 2, via `useSchemaForm`. ✓
- D3 (`LaunchFormSchema` RPC + `StartRun` extension + web wiring) → Tasks 3, 4, 7. ✓
- D4 (Bake + substitution + launch, I1 removal) → Tasks 5, 6, 7, 8 — split into four because the naive single-task sketch in spec §3 D4 does not match the actual runtime topology (see below). ✓
- §8 testing: parity (Task 9), D1 wasm/Go-version regression (Task 1's build-asset check), D2 per-kind render tests (Task 2), D3/D4 typed-value + field-error tests (Tasks 5-8), integration check (Task 8 Step 5). RBAC re-check of org-overlay is explicitly **not** covered — SP-B's overlay contract does not exist yet (spec §9 open question 6); flagged, not silently skipped.

**Corrections this plan makes to the spec's §3 D4 sketch (found during research, not assumed):**
1. **Compilation is asynchronous.** `RecipeService.StartRun` never calls `include.Resolve`/`dsl.Compile` — `internal/services/recipe/recipe.go:217` hands raw bundle bytes to `s.d.Workflows.LaunchRecipeRun`, which fires a Temporal workflow (`RunRecipeWorkflow`, `internal/workflows/runrecipe.go:238`). The actual compile happens inside `CompileRecipeActivity` (`internal/infrastructure/execution/recipe_activities.go:71`), on a worker, after `StartRun` has already returned. The spec's `ApplyBakedInputs(resolved *include.Resolved, baked)` sketch implicitly assumed a synchronous, in-process `*include.Resolved` to mutate — no such object exists at `StartRun` time. This plan threads `Baked` through `RunRecipeInput` → `CompileRecipeActivityInput` → `dsl.Input` instead (Task 8), and keeps only the pure, schema-only `BakeForm` validation (no bundle compile needed) synchronous in `StartRun` (Task 7) for fast field-error feedback.
2. **`ApplyBakedInputs`'s actual target is not `BoundComponent.Inputs`.** Provider params live on `ast.ClusterDoc.Provider.Params` (`lower.go`'s `lowerProvider` reads it directly), a different field from an include's bound component inputs. `BoundComponent.Inputs` is already typed and forwarded correctly *during* `include.Resolve` (via `forwardInputs`/`bindInputs`, which already call `checkInputType`) — the only string-only leak downstream of that is `lower.go`'s `fmt.Sprint` stringification (Task 5), not a missing overlay. The function name/signature from spec §4 is kept for contract continuity; its body was redesigned to target what is actually mutable at that pipeline stage (Task 6).
3. **Workflow-level `${{ inputs.x }}` forwarding into a top-level `include:` job does not work today**, independent of this plan — `include.Resolve`'s top-level call passes `boundInputs: nil` (spec never noticed this because `ast.WorkflowDoc.Inputs` didn't exist before SP-A). Task 6 wires it, reusing `Resolve`'s existing nested-include machinery rather than adding a parallel mechanism.

**Scoped out, flagged explicitly (not silently dropped):**
- `Resolved.Inputs`/`CompiledJob.ResolvedInputs` for jobs with **no** originating `BoundComponent` (pure workflow-level jobs referencing `inputs.*` directly in their own `when:`) stays empty, matching `lower.go`'s current documented contract (`jobInputFields`'s "no BoundComponent → zero values"). Workflow-level values that flow *into* a component (the common case — most recipes launch via `include:`) get the full typed fix (Tasks 5+6 together). Extending typed `inputs.*` to bare workflow-level jobs is a natural, small follow-up but was not required by any test in `docs/superpowers/specs/2026-07-08-sp-d-launch-form.md` and would expand `CompiledJob` assembly beyond what Task 5/6 already touch.
- Org-overlay (spec §9 open questions 5-6) — SP-B dependency, not yet designed; `LaunchFormSchema` composes the base form only.
- i18n of `FieldError` codes (spec §9 open question 7) — no i18n table exists in `web/` yet; out of scope.
- `matrix` dimensions in the form (spec §9 open question 8) — no concrete example surfaced during this bundle's research; deferred until a workflow actually needs matrix-driven form fields.
- Baked persistence into `TestRunRecord` — explicitly SP-E's job (spec §3 D4 point 5); Task 8 threads `Baked` only as far as the compile activity, then lets it fall out of scope.

**Placeholder scan:** no TBD/TODO in any code block; every step has real, compilable-shaped Go/TS reusing actual signatures found in the codebase (`internal/dsl/schema`, `internal/dsl/include/resolve.go`, `internal/workflows/runrecipe.go`, `packages/schemapb{,-react}`). Test helpers referenced but not fully inlined (`dockerBundleFiles`, `newFakeRecipeRepo`, `newFakeRunRepo`, `newFakeRecipeWorkflows`, `stubChecker`) are named, pre-existing fixtures in `internal/services/{dsl,recipe}/*_test.go` per that package's own established convention (mirrored from SP-A's plan, which does the same for `fieldsByName`/`writeFile`).

**Type/signature consistency across tasks:** `schemapb.Baked` flows Task 7 (`BakeForm` produces it) → Task 8 (`RunRecipeInput.Baked` carries it) → Task 6 (`ApplyBakedInputs`/`SplitBakedValues` consume it) unchanged; `include.Resolve`'s new `workflowInputs map[string]any` param (Task 6) is fed by the same `SplitBakedValues` call `dsl.Compile` (Task 8) makes; `CompiledJob.ResolvedInputs`'s new `map[string]*structpb.Value` type (Task 5) is independent of the `Baked` threading and lands first so Task 6/8's typed values have a non-stringifying sink waiting for them.
