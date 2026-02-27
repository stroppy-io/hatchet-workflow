# Coding Conventions

**Analysis Date:** 2026-02-27

## Languages & Scope

This codebase has two distinct language environments with separate conventions:
- **Go** — backend services in `cmd/`, `internal/`
- **TypeScript/React** — frontend in `web/src/`

---

## Go Conventions

### Naming Patterns

**Files:**
- `snake_case.go` for all Go source files
- `snake_case_test.go` for unit tests co-located with source
- `snake_case_integration_test.go` for integration tests with build tag `//go:build integration`
- Hyphens are used in multi-word filenames when the file is not a package entry point: `postgres-placement-builder.go`

**Functions:**
- Exported: `PascalCase` — e.g., `NewProvisionerService`, `SearchDatabasePlacementItem`, `FirstFreeIP`
- Unexported: `camelCase` — e.g., `newDefault`, `newZapCfg`, `resolveNetworkSettings`, `stroppyMetadata`
- Constructor pattern: `New<TypeName>` — e.g., `New()`, `NewFromConfig()`, `NewFromEnv()`, `NewWorkerName()`

**Variables and Constants:**
- Exported constants: `PascalCase` — e.g., `DevelopmentMod`, `ProductionMod`, `WorkerNamePrefix`
- Unexported constants: `camelCase` — e.g., `defaultDockerNetworkCidr`, `metadataRoleKey`, `ctxLoggerKey`
- Constant type aliases used as domain primitives: `consts.DefaultValue`, `consts.EnvKey`, `consts.ConstValue`, `consts.Str` (all `= string` aliases for semantic clarity)

**Types:**
- Structs: `PascalCase` — e.g., `ProvisionerService`, `Config`, `Stopper`
- Interfaces: `PascalCase`, often with a verb or role suffix — e.g., `NetworkManager`, `DeploymentService`, `TerraformActor`, `StopInterface`
- Type aliases for strong typing: `type LogMod string`, `type RunId Ulid`, `type StopFn func()`

**Packages:**
- All lowercase, single word — `logger`, `shutdown`, `provision`, `ids`, `ips`, `consts`, `defaults`

### Code Style

**Formatting:**
- Standard `gofmt` formatting enforced
- Imports grouped: stdlib, then third-party, then internal (separated by blank lines)

**Linting:**
- `gochecknoglobals` linter is active; global vars are explicitly suppressed with `//nolint:gochecknoglobals // reason` when needed
- `paralleltest` linter is active; tests suppressed with `//nolint:paralleltest // reason` when global state prevents parallelism
- Nolint comments always include a reason after `//`

### Import Organization

**Order (three groups, blank-line separated):**
1. Standard library — e.g., `"context"`, `"fmt"`, `"os"`, `"net"`
2. Third-party packages — e.g., `"github.com/samber/lo"`, `"go.uber.org/zap"`
3. Internal packages — `"github.com/stroppy-io/hatchet-workflow/internal/..."`

**Aliasing:**
- Use aliases only to resolve conflicts: `edgeDomain "github.com/stroppy-io/hatchet-workflow/internal/domain/edge"`, `databasepb "github.com/stroppy-io/hatchet-workflow/internal/proto/database"`

### Error Handling

**Patterns:**
- Return `error` as the last return value; callers always check it immediately
- Errors are propagated up with `return nil, err` (no wrapping unless context is needed)
- Fatal configuration errors in initialization use `panic(err)` — e.g., in `NewFromConfig` when log level parsing fails
- Sentinel errors declared as package-level vars: `var ErrUnsupportedDeploymentTarget = fmt.Errorf("unsupported deployment target")`
- Plain `fmt.Errorf("descriptive message: %v", err)` used for wrapping; no `%w` wrapping observed in production code

**Error Testing (Go):**
```go
// Standard pattern for testing error conditions:
if err != nil {
    t.Fatalf("unexpected error: %v", err)
}
// Testing expected errors:
if err == nil {
    t.Fatal("expected error for nil template")
}
```

### Logging

**Framework:** `go.uber.org/zap` as primary logger; `go.uber.org/zap/exp/zapslog` bridges to `log/slog`; `github.com/rs/zerolog` used for Docker/Hatchet SDK compatibility

**Global logger:** `logger.Global()` returns the application-wide `*zap.Logger`; `logger.Named(name)` returns a named sub-logger with configurable level

**Context-aware logging:**
- `logger.WrapInCtx(ctx, logger)` stores logger in context
- `logger.NewFromCtx(ctx)` retrieves logger from context (falls back to global)
- `logger.CtxWithAttrs(ctx, fields...)` attaches fields to context for later retrieval with `logger.GetCtxFields(ctx)`

**Usage pattern:**
```go
log := logger.Named("provisioner")
log.Info("message", zap.String("field", value))
```

### Function Design

**Constructors:** Always return a pointer to the struct; accept dependencies as parameters (dependency injection, not global lookup)
```go
func NewProvisionerService(
    networkManager NetworkManager,
    deploymentService DeploymentService,
) *ProvisionerService {
    return &ProvisionerService{
        networkManager:    networkManager,
        deploymentService: deploymentService,
    }
}
```

**Interfaces:** Defined at the point of use (in the consumer package), not the implementation. Interfaces are small and focused (1-4 methods).

**Method receivers:** Value receivers for read-only methods on small structs (`ProvisionerService`), pointer receivers where mutation or large copy is a concern.

### Comments

**When to Comment:**
- Exported symbols get godoc comments: `// NewFromConfig creates new logger from config.`
- Inline comments explain non-obvious logic (padding constants, nolint rationale)
- No JSDoc-style parameter documentation; plain English sentences

**Commented-Out Code:**
- Kept in source with surrounding context when deletion is deferred (e.g., `ids.go` HatchetRunId block)

### Module Design

**Package organization:**
- `internal/core/` — cross-cutting utilities (logger, ids, ips, defaults, consts, shutdown)
- `internal/domain/` — business logic grouped by bounded context
- `internal/infrastructure/` — external system adapters
- `internal/proto/` — generated protobuf code (do not hand-edit)

**Constants via type aliases in `consts` package:**
```go
// Use typed aliases to communicate intent:
const HatchetClientHostPortKey consts.EnvKey = "HATCHET_CLIENT_HOST_PORT"
const DefaultUserName consts.DefaultValue = "stroppy"
```

---

## TypeScript/React Conventions (web/)

### Naming Patterns

**Files:** `PascalCase.tsx` for components — e.g., `TestWizard.tsx`, `SettingsEditor.tsx`, `TopologyCanvas.tsx`
**Components:** Named exports, PascalCase — `export const TestWizard = ...`
**Props interfaces:** `<ComponentName>Props` — e.g., `interface TestWizardProps`

### Code Style

**Formatting/Linting:**
- ESLint with `@eslint/js`, `typescript-eslint`, `eslint-plugin-react-hooks`, `eslint-plugin-react-refresh`
- TypeScript strict mode via `tsconfig.app.json`
- Path alias `@/*` maps to `./src/*`

**Import pattern (TSX):**
1. React and library imports
2. Proto-generated types (from `../proto/...`)
3. UI component imports
4. Utility imports

### Component Pattern

```tsx
// Props interface first
interface TestWizardProps {
    test: Test
    onChange: (updates: Partial<Test>) => void
}

// Named arrow function export
export const TestWizard = ({ test, onChange }: TestWizardProps) => {
    const [activeTab, setActiveTab] = useState('database')
    // ...
}
```

**Styling:** Tailwind CSS utility classes; `cn()` helper from `../App` for conditional class merging

**Protobuf integration:** `@bufbuild/protobuf` `create()` function used to construct proto messages; generated types imported from `../proto/<service>/<name>_pb`

---

*Convention analysis: 2026-02-27*
