# Testing Patterns

**Analysis Date:** 2026-02-27

## Test Framework

**Runner:**
- Go standard `testing` package — no external test runner
- `github.com/stretchr/testify v1.11.1` — assertion library used in some tests

**Assertion Libraries:**
- `github.com/stretchr/testify/require` — used in `internal/core/` packages for fail-fast assertions
- Raw `testing.T` methods (`t.Fatalf`, `t.Errorf`, `t.Fatal`) — used in `internal/domain/` packages (no testify dependency)

**Run Commands:**
```bash
go test ./...                      # Run all tests
go test ./internal/...             # Run internal package tests
go test -run TestName ./pkg/...    # Run specific test
go test -bench=. ./internal/...    # Run benchmarks
go test -tags integration ./...    # Run integration tests (requires Docker daemon)
go test -v ./...                   # Verbose output
go test -cover ./...               # Coverage report
```

## Test File Organization

**Location:** Co-located with source — test files live in the same directory as the code they test

**Naming:**
- Unit tests: `<source_file>_test.go` — e.g., `ips_test.go` next to `ips.go`, `logger_test.go` next to `logger.go`
- Integration tests: `<source_file>_integration_test.go` with build tag — e.g., `docker_integration_test.go`
- Example tests: `topologies_examples_test.go` in `examples/database-topologies/`

**Package declaration:**
- Most tests use the same package as the source: `package provision`, `package ips`, `package shutdown`
- Some tests use white-box access with same package + nolint: `package logger //nolint:testpackage // test package`

**Structure:**
```
internal/core/logger/
├── logger.go
├── logger_test.go       # same package, tests unexported functions
├── ctx.go
└── ctx_test.go

internal/domain/provision/
├── provision.go
├── postgres-placement-builder.go
├── postgres_placement_builder_test.go
└── ip_count_test.go

internal/domain/edge/containers/
├── docker.go
└── docker_integration_test.go    # build tag: //go:build integration
```

## Test Structure

**Table-driven tests (primary pattern for domain logic):**
```go
func TestRequiredIPCount(t *testing.T) {
    tests := []struct {
        name string
        tmpl *database.Database_Template
        want int
    }{
        {
            name: "nil template",
            tmpl: nil,
            want: 0,
        },
        {
            name: "postgres instance",
            tmpl: &database.Database_Template{...},
            want: 1,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := RequiredIPCount(tt.tmpl); got != tt.want {
                t.Errorf("RequiredIPCount() = %d, want %d", got, tt.want)
            }
        })
    }
}
```

**Flat tests (for simple or state-dependent scenarios):**
```go
func TestBuildForPostgresInstance_SingleNode(t *testing.T) {
    network := networkWithIPs("10.0.0.1")
    b := newPostgresPlacementBuilder(network)

    intent, err := b.BuildForPostgresInstance(&database.Database_Template_PostgresInstance{...})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(intent.GetItems()) != 1 {
        t.Fatalf("expected 1 item, got %d", len(intent.GetItems()))
    }
}
```

**Patterns:**
- Setup data via helper constructor functions at file level (see Fixtures section)
- No `TestMain` — no global setup/teardown
- `t.Parallel()` used where tests are goroutine-safe; explicitly suppressed with `//nolint:paralleltest` for global state
- `t.Cleanup()` for resource teardown in integration tests

## Mocking

**Framework:** No mock generation framework (no mockery, gomock, etc.)

**Patterns:**
- Manual mock structs with boolean `called` fields for behavior verification:
```go
type mockStopFn struct {
    called bool
}

func (m *mockStopFn) Stop() {
    m.called = true
}
```
- Interface-based injection: production code depends on interfaces (`NetworkManager`, `DeploymentService`), allowing manual mock substitution in tests
- `zap.NewNop()` used for no-op logger in tests that need a logger argument

**Integration test conditional skip:**
```go
func dockerDaemonOrSkip(t *testing.T, ctx context.Context) *dockerClient.Client {
    t.Helper()
    cli, err := dockerClient.NewClientWithOpts(...)
    if err != nil {
        t.Skipf("docker client not available: %v", err)
    }
    if _, err := cli.Ping(ctx); err != nil {
        t.Skipf("docker daemon not available: %v", err)
    }
    return cli
}
```

**What to Mock:**
- External I/O (Docker daemon, cloud APIs, Terraform)
- Global state that cannot be shared across parallel tests (global logger, global shutdown stopper)

**What NOT to Mock:**
- Pure in-memory logic (IP calculations, placement builders, ID generation)
- Proto message construction (use real proto structs directly)

## Fixtures and Factories

**Test data constructed via package-level helper functions:**
```go
// Network factory
func networkWithIPs(ips ...string) *deployment.Network {
    out := make([]*deployment.Ip, len(ips))
    for i, v := range ips {
        out[i] = ip(v)
    }
    return &deployment.Network{
        Identifier: &deployment.Identifier{Id: "net-1", Name: "test-net"},
        Cidr:       &deployment.Cidr{Value: "10.0.0.0/24"},
        Ips:        out,
    }
}

// Hardware factory
func hw(cores, mem, disk uint32) *deployment.Hardware {
    return &deployment.Hardware{Cores: cores, Memory: mem, Disk: disk}
}

// Settings factory
func pgSettings() *database.Postgres_Settings {
    return &database.Postgres_Settings{
        Version:       database.Postgres_Settings_VERSION_17,
        StorageEngine: database.Postgres_Settings_STORAGE_ENGINE_HEAP,
    }
}
```

**Helper search functions for asserting results:**
```go
func findItem(items []*provision.PlacementIntent_Item, name string) *provision.PlacementIntent_Item
func findContainer(items []*provision.PlacementIntent_Item, itemName, containerID string) *provision.Container
```

**Pointer helpers:**
```go
func uint32Ptr(v uint32) *uint32 { return &v }
```

**Location:** Fixtures are defined at the top of each `_test.go` file in the same package; no shared fixture files.

## Coverage

**Requirements:** No enforced coverage threshold

**View Coverage:**
```bash
go test -cover ./...
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

## Test Types

**Unit Tests:**
- Cover pure functions and in-memory logic exhaustively
- Test happy paths, nil inputs, boundary conditions, and error paths
- Examples: `internal/core/ips/ips_test.go` (11 test functions + 3 benchmarks), `internal/domain/provision/postgres_placement_builder_test.go` (16 test functions)
- Scope: single function or small group of related functions per test file

**Integration Tests:**
- Gated by `//go:build integration` build tag at the top of the file
- Require live infrastructure (Docker daemon)
- Use `t.Skipf` to self-skip when infrastructure is unavailable
- Use `t.Cleanup` for teardown regardless of test result
- Example: `internal/domain/edge/containers/docker_integration_test.go`

**Example/Smoke Tests:**
- Filesystem-driven: `examples/database-topologies/topologies_examples_test.go` reads all `.yaml` files in a directory and validates each one as a sub-test
- Pattern: `t.Run(filepath.Base(file), func(t *testing.T) {...})`

**Benchmarks:**
- Co-located with unit tests, using `func BenchmarkXxx(b *testing.B)` signature
- Example in `internal/core/ips/ips_test.go`: `BenchmarkFirstFreeIP`, `BenchmarkRandomIP`, `BenchmarkRandomIP_LargeNetwork`

## Common Patterns

**Async Testing (integration tests):**
```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
defer cancel()
// ... operations with ctx
```

**Error Testing:**
```go
// Expect success:
if err != nil {
    t.Fatalf("unexpected error: %v", err)
}

// Expect failure:
if err == nil {
    t.Fatal("expected error for nil template")
}

// testify style (in core packages):
require.NoError(t, err)
require.NotNil(t, result)
require.True(t, mock.called)
require.Len(t, stopper.stops, 1)
```

**Sub-test naming convention:**
- Descriptive snake_case or human-readable names in table tests: `"nil template"`, `"postgres instance"`, `"cluster with 2 replicas"`
- Function-name suffix for flat tests: `TestBuildForPostgresInstance_SingleNode`, `TestBuildForPostgresInstance_NilTemplate`, `TestBuildForPostgresInstance_WithSidecars`

**Parallel test declaration:**
```go
func TestStopper_Register(t *testing.T) {
    t.Parallel()
    // ... test body
}
```

---

*Testing analysis: 2026-02-27*
