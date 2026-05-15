# Plan 01: Foundation + IAM Implementation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wipe legacy code, scaffold the new module layout, port reusable utilities from komeet, and ship a working IAM service (Auth, User, Tenant, Member, ApiToken) reachable via ConnectRPC. End state: `stroppy-cloud server` boots, applies migrations, accepts `stroppy-cloud login` from CLI, and persists/retrieves users + tenants.

**Architecture:** Single binary `stroppy-cloud` (Cobra root → `server` / `agent` / CLI subcommands). Service layer uses `repository.ProtoRepository` from ratel, transactions via avito-tech `go-transaction-manager` (komeet's `pgtx` wrapper). Connect handlers stay thin: validate → call service → wrap response. Errors flow through `*domainerr.Error` (proto-only `Code` enum) into Connect details. JWT auth middleware loads user from access token; refresh tokens persist in `refresh_tokens` with family-based rotation + reuse detection.

**Tech Stack:** Go 1.25, ConnectRPC, pgx/v5, ratel (`pkg/repository`, `pkg/exec`, `pkg/migrate`), avito-tech go-transaction-manager, OTel + otelzap, golang-jwt, bcrypt, Cobra, Viper (config), testcontainers-go.

---

## Files at end of plan

### Deleted
- `internal/domain/api/` (all)
- `internal/domain/agent/setup_*.go`, `client_*.go`, `agent_server.go`, `protocol.go`, `deployer.go`, `deploy_docker.go`, `cloudinit*.go`, `executor*.go`
- `internal/domain/run/builder.go`, `task_*.go`, `state.go`, `state_test.go`, `validate.go`, `effective_config.go`, `machines.go`, `placement.go`, `rendered_configs.go`, `cost.go`, `cost_test.go`, `pdisk_size.go`, `pdisk_size_test.go`, `builder_test.go`
- `internal/core/dag/`
- `internal/domain/scheduler/`
- `internal/domain/types/`
- `internal/domain/auth/` (existing token mw — replaced)
- `internal/domain/webhook/` (existing client — replaced)
- `internal/domain/metrics/` (existing — replaced)
- `internal/domain/dbconfig/`
- `internal/infrastructure/postgres/generated/`
- `internal/infrastructure/postgres/queries/`
- `internal/infrastructure/postgres/*_storage.go`
- `internal/infrastructure/postgres/db.go`
- `internal/infrastructure/postgres/db_test.go`
- `internal/infrastructure/postgres/migrate_iom3.go`
- `internal/infrastructure/postgres/job_cost_test.go`
- `internal/storage/`
- `cmd/cli/`

### Created
- `cmd/stroppy-cloud/main.go`
- `cmd/stroppy-cloud/cmd_server.go`
- `cmd/stroppy-cloud/cmd_agent.go` (stub — full impl in plan 05)
- `cmd/stroppy-cloud/cmd_cli/root.go`
- `cmd/stroppy-cloud/cmd_cli/login.go`
- `cmd/stroppy-cloud/cmd_cli/logout.go`
- `cmd/stroppy-cloud/cmd_cli/whoami.go`
- `cmd/stroppy-cloud/cmd_cli/tenant.go`
- `cmd/stroppy-cloud/cmd_cli/user.go`
- `cmd/stroppy-cloud/cmd_cli/context.go`
- `internal/core/configurator/config.go`
- `internal/core/configurator/load.go`
- `internal/core/configurator/load_test.go`
- `internal/core/domainerr/error.go`
- `internal/core/domainerr/codes.go`
- `internal/core/domainerr/error_test.go`
- `internal/core/eventing/bus.go`
- `internal/core/eventing/events.go`
- `internal/core/eventing/bus_test.go`
- `internal/core/ids/ulid.go`
- `internal/core/ids/ulid_test.go`
- `internal/core/tracing/entity.go`
- `internal/core/tracing/with.go`
- `internal/domain/services/iam/service.go`
- `internal/domain/services/iam/auth.go`
- `internal/domain/services/iam/users.go`
- `internal/domain/services/iam/tenants.go`
- `internal/domain/services/iam/members.go`
- `internal/domain/services/iam/api_tokens.go`
- `internal/domain/services/iam/passwords.go`
- `internal/domain/services/iam/jwt.go`
- `internal/domain/services/iam/integration_test.go`
- `internal/infrastructure/postgres/postgres.go` (replaces old `db.go`)
- `internal/infrastructure/postgres/pgtx/flow.go`
- `internal/infrastructure/postgres/pgtx/wrapper.go`
- `internal/sdk/client/client.go`
- `internal/sdk/client/credentials.go`
- `internal/testutil/pgcontainer/container.go`
- `internal/testutil/pgcontainer/schema.go`
- `internal/testutil/fixture/fixture.go`
- `internal/testutil/fixture/iam.go`
- `internal/transport/connect/iam.go`
- `internal/transport/connect/server.go`
- `internal/transport/middleware/recovery.go`
- `internal/transport/middleware/requestid.go`
- `internal/transport/middleware/logging.go`
- `internal/transport/middleware/auth.go`
- `internal/transport/middleware/tenant.go`
- `internal/transport/middleware/validate.go`
- `internal/transport/middleware/idempotency.go`
- `internal/transport/middleware/idempotency_registry.go`
- `internal/transport/middleware/errmap.go`
- `internal/transport/middleware/context.go`

### Modified
- `go.mod` (new deps), `go.sum`
- `Makefile` (add `make server-run` target)

---

## Task 1: Wipe legacy domain + storage code

**Files:**
- Delete: see "Deleted" list above

- [ ] **Step 1: Remove old domain/api**

Run:
```bash
rm -rf internal/domain/api
```

- [ ] **Step 2: Remove old agent runtime**

Run:
```bash
cd internal/domain/agent && \
  rm -f setup_*.go client_*.go agent_server.go protocol.go deployer.go deploy_docker.go cloudinit.go cloudinit_test.go executor.go executor_test.go
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud
ls internal/domain/agent/   # should be empty
rmdir internal/domain/agent
```

- [ ] **Step 3: Remove old run/scheduler/types/auth/webhook/metrics/dbconfig**

Run:
```bash
rm -rf internal/core/dag
rm -rf internal/domain/run
rm -rf internal/domain/scheduler
rm -rf internal/domain/types
rm -rf internal/domain/auth
rm -rf internal/domain/webhook
rm -rf internal/domain/metrics
rm -rf internal/domain/dbconfig
```

- [ ] **Step 4: Remove old DB layer (keep migrations + ratel postgres.go stub)**

Run:
```bash
cd internal/infrastructure/postgres && \
  rm -rf generated queries && \
  rm -f db.go db_test.go migrate_iom3.go job_storage.go job_storage_test.go job_cost_test.go \
        suite_storage.go run_storage.go run_storage_test.go run_preset_storage.go
cd /home/yaroher/devel/arenadata/stroppy-io/stroppy-cloud
rm -rf internal/storage
```

- [ ] **Step 5: Remove old cmd/cli**

Run:
```bash
rm -rf cmd/cli
```

- [ ] **Step 6: Verify build is broken (expected)**

Run:
```bash
go build ./... 2>&1 | head -5
```
Expected: errors referencing missing packages.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "chore: wipe legacy domain/api, storage, dag, scheduler — clear runway for big-bang refactor"
```

---

## Task 2: Scaffold new directory tree

**Files:** all new directories listed in §1 of the spec.

- [ ] **Step 1: Create core packages**

Run:
```bash
mkdir -p internal/core/{configurator,domainerr,eventing,ids,tracing}
mkdir -p internal/domain/services/{iam,catalog,testing,system,agent,ops,admin,stroppy}
mkdir -p internal/domain/workers/{nodeworker,scheduler,webhookworker,recovery}
mkdir -p internal/transport/{connect,stream,middleware}
mkdir -p internal/infrastructure/postgres/pgtx
mkdir -p internal/infrastructure/stroppybin
mkdir -p internal/sdk/client
mkdir -p internal/testutil/{pgcontainer,fixture}
mkdir -p cmd/stroppy-cloud/cmd_cli
mkdir -p tests/e2e
```

- [ ] **Step 2: Add per-package keep markers**

Run:
```bash
for dir in internal/core/{configurator,domainerr,eventing,ids,tracing} \
           internal/domain/services/{iam,catalog,testing,system,agent,ops,admin,stroppy} \
           internal/domain/workers/{nodeworker,scheduler,webhookworker,recovery} \
           internal/transport/{connect,stream,middleware} \
           internal/infrastructure/postgres/pgtx \
           internal/infrastructure/stroppybin \
           internal/sdk/client \
           internal/testutil/{pgcontainer,fixture}; do
    pkg=$(basename "$dir")
    echo "package $pkg" > "$dir/doc.go"
done
```

- [ ] **Step 3: Verify tree exists**

Run:
```bash
find internal cmd tests -type d -not -path "*/proto/*" | sort
```
Expected: every directory above present.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "chore: scaffold service/transport/worker/testutil packages with doc.go stubs"
```

---

## Task 3: Add Go module dependencies

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Pull deps**

Run:
```bash
go get \
  connectrpc.com/connect \
  connectrpc.com/otelconnect \
  github.com/avito-tech/go-transaction-manager/trm \
  github.com/avito-tech/go-transaction-manager/pgxv5 \
  github.com/golang-jwt/jwt/v5 \
  github.com/oklog/ulid/v2 \
  github.com/samber/lo \
  github.com/spf13/cobra \
  github.com/spf13/viper \
  github.com/testcontainers/testcontainers-go/modules/postgres \
  github.com/uptrace/opentelemetry-go-extra/otelzap \
  golang.org/x/crypto \
  go.opentelemetry.io/otel \
  go.opentelemetry.io/otel/trace \
  github.com/prometheus/client_golang
go mod tidy
```

- [ ] **Step 2: Verify**

Run:
```bash
grep -E "(connect|trm|jwt|ulid|cobra|viper|otel|testcontainers)" go.mod | sort -u
```
Expected: each library listed.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add ConnectRPC/JWT/Cobra/testcontainers/tx-manager deps"
```

---

## Task 4: Port pgtx from komeet

**Files:**
- Create: `internal/infrastructure/postgres/pgtx/flow.go`, `wrapper.go`

- [ ] **Step 1: Copy `flow.go`**

Create `internal/infrastructure/postgres/pgtx/flow.go`:

```go
package pgtx

import (
	"context"
	"fmt"

	"github.com/avito-tech/go-transaction-manager/pgxv5"
	trmpgx "github.com/avito-tech/go-transaction-manager/pgxv5"
	"github.com/avito-tech/go-transaction-manager/trm"
	"github.com/avito-tech/go-transaction-manager/trm/manager"
	"github.com/avito-tech/go-transaction-manager/trm/settings"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"
)

// TxManager re-exports the trm interface so callers depend on this package
// only.
type TxManager = trm.Manager

// NewTxFlow creates a TxExecutor + Manager bound to the given pool.
func NewTxFlow(pool *pgxpool.Pool, txSettings *trmpgx.Settings, options ...sqlexec.TxExecutorOption) (*sqlexec.TxExecutor, *manager.Manager, error) {
	if pool == nil {
		return nil, nil, fmt.Errorf("pgtx: pool is nil")
	}
	if txSettings == nil {
		txSettings = ReadCommittedSettings()
	}
	mgr, err := manager.New(trmpgx.NewDefaultFactory(pool), trm.WithSettings(*txSettings))
	if err != nil {
		return nil, nil, fmt.Errorf("pgtx: build manager: %w", err)
	}
	return sqlexec.NewTxExecutor(pool, options...), mgr, nil
}

// NewSettings builds tx settings with the given isolation level.
func NewSettings(level pgx.TxIsoLevel, opts ...settings.Opt) *trmpgx.Settings {
	s := pgxv5.MustSettings(settings.Must(opts...), pgxv5.WithTxOptions(pgx.TxOptions{IsoLevel: level}))
	return &s
}

// Convenience constructors.
func SerializableSettings(opts ...settings.Opt) *trmpgx.Settings {
	return NewSettings(pgx.Serializable, opts...)
}

func RepeatableReadSettings(opts ...settings.Opt) *trmpgx.Settings {
	return NewSettings(pgx.RepeatableRead, opts...)
}

func ReadCommittedSettings(opts ...settings.Opt) *trmpgx.Settings {
	return NewSettings(pgx.ReadCommitted, opts...)
}

// WithTransactionRet runs fn inside a transaction with the given isolation,
// returning the typed result.
func WithTransactionRet[T any](
	ctx context.Context,
	mgr TxManager,
	level pgx.TxIsoLevel,
	fn func(ctx context.Context) (T, error),
	opts ...settings.Opt,
) (ret T, err error) {
	err = mgr.DoWithSettings(ctx, NewSettings(level, opts...), func(ctx context.Context) error {
		ret, err = fn(ctx)
		return err
	})
	return ret, err
}
```

- [ ] **Step 2: Copy `wrapper.go`**

Create `internal/infrastructure/postgres/pgtx/wrapper.go`:

```go
package pgtx

import (
	"context"

	"github.com/avito-tech/go-transaction-manager/trm/settings"
	"github.com/jackc/pgx/v5"
)

// WithSerializable runs fn inside a SERIALIZABLE transaction.
func WithSerializable(ctx context.Context, mgr TxManager, fn func(ctx context.Context) error, opts ...settings.Opt) error {
	return mgr.DoWithSettings(ctx, NewSettings(pgx.Serializable, opts...), fn)
}

// WithSerializableRet returns a typed value from a SERIALIZABLE tx.
func WithSerializableRet[T any](ctx context.Context, mgr TxManager, fn func(ctx context.Context) (T, error), opts ...settings.Opt) (T, error) {
	return WithTransactionRet(ctx, mgr, pgx.Serializable, fn, opts...)
}

// WithRepeatableRead runs fn inside a REPEATABLE READ transaction.
func WithRepeatableRead(ctx context.Context, mgr TxManager, fn func(ctx context.Context) error, opts ...settings.Opt) error {
	return mgr.DoWithSettings(ctx, NewSettings(pgx.RepeatableRead, opts...), fn)
}

// WithReadCommitted runs fn inside a READ COMMITTED transaction.
func WithReadCommitted(ctx context.Context, mgr TxManager, fn func(ctx context.Context) error, opts ...settings.Opt) error {
	return mgr.DoWithSettings(ctx, NewSettings(pgx.ReadCommitted, opts...), fn)
}
```

- [ ] **Step 3: Build**

Run:
```bash
go build ./internal/infrastructure/postgres/pgtx/...
```
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/infrastructure/postgres/pgtx/
git commit -m "feat(pgtx): port avito-tech transaction manager wrapper from komeet"
```

---

## Task 5: Port tracing entity helpers from komeet

**Files:**
- Create: `internal/core/tracing/entity.go`, `with.go`

- [ ] **Step 1: Write `entity.go`**

Create `internal/core/tracing/entity.go`:

```go
package tracing

import (
	"context"

	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/stroppy-io/stroppy-cloud/internal/core/logger"
)

// Entity is an embeddable struct that gives a type its own tracer + otelzap
// logger. Services embed `*Entity` and call Trace / TraceRet to wrap span
// + structured logger context.
type Entity struct {
	tracer trace.Tracer
	logger *otelzap.Logger
}

// NewEntity builds an Entity with a tracer + logger scoped under name.
func NewEntity(name string) *Entity {
	return &Entity{
		tracer: otel.Tracer(name),
		logger: newLogger(name),
	}
}

func newLogger(name string) *otelzap.Logger {
	l := logger.Global().Named(name)
	return otelzap.New(l)
}

// Tracer returns the entity's tracer.
func (e *Entity) Tracer() trace.Tracer { return e.tracer }

// Logger returns the entity's otelzap-wrapped logger.
func (e *Entity) Logger() *otelzap.Logger { return e.logger }

// Trace wraps fn in a span; closes span on return; records error if non-nil.
func (e *Entity) Trace(ctx context.Context, name string, fn func(ctx context.Context, span trace.Span) error) error {
	return WithTraceErr(e.tracer, ctx, name, fn)
}
```

- [ ] **Step 2: Write `with.go`**

Create `internal/core/tracing/with.go`:

```go
package tracing

import (
	"context"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// WithTraceErr starts a span, runs fn, records error.
func WithTraceErr(tracer trace.Tracer, ctx context.Context, name string, fn func(ctx context.Context, span trace.Span) error) error {
	ctx, span := tracer.Start(ctx, name)
	defer span.End()
	err := fn(ctx, span)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

// WithTraceRet returns a typed value plus error.
func WithTraceRet[T any](tracer trace.Tracer, ctx context.Context, name string, fn func(ctx context.Context, span trace.Span) (T, error)) (T, error) {
	ctx, span := tracer.Start(ctx, name)
	defer span.End()
	v, err := fn(ctx, span)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}
```

- [ ] **Step 3: Verify `internal/core/logger` has Global()**

Run:
```bash
grep -n "func Global" internal/core/logger/*.go
```

If absent, add to `internal/core/logger/global.go`:

```go
package logger

import "go.uber.org/zap"

var global *zap.Logger = zap.NewNop()

func Global() *zap.Logger {
	if global == nil {
		return zap.NewNop()
	}
	return global
}

func SetGlobal(l *zap.Logger) { global = l }
```

- [ ] **Step 4: Build**

Run:
```bash
go build ./internal/core/tracing/...
```
Expected: success.

- [ ] **Step 5: Commit**

```bash
git add internal/core/tracing/ internal/core/logger/
git commit -m "feat(tracing): port Entity + WithTraceErr/WithTraceRet helpers"
```

---

## Task 6: Implement `domainerr` (proto-only Code)

**Files:**
- Create: `internal/core/domainerr/error.go`, `codes.go`, `error_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/core/domainerr/error_test.go`:

```go
package domainerr_test

import (
	"errors"
	"testing"

	errorspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
)

func TestErrorCarriesCode(t *testing.T) {
	e := domainerr.E(errorspb.Code_NOT_FOUND)
	if e.Code() != errorspb.Code_NOT_FOUND {
		t.Fatalf("expected NOT_FOUND, got %v", e.Code())
	}
	if e.Error() != "NOT_FOUND" {
		t.Fatalf("Error() should return enum name, got %q", e.Error())
	}
}

func TestErrorsIsMatchesCode(t *testing.T) {
	e := domainerr.E(errorspb.Code_PERMISSION_DENIED)
	if !errors.Is(e, domainerr.Codeful(errorspb.Code_PERMISSION_DENIED)) {
		t.Fatal("errors.Is should match same code")
	}
	if errors.Is(e, domainerr.Codeful(errorspb.Code_NOT_FOUND)) {
		t.Fatal("errors.Is should not match different code")
	}
}

func TestErrorDetailsAttached(t *testing.T) {
	d := &errorspb.Detail{Kind: &errorspb.Detail_ResourceInfo{
		ResourceInfo: &errorspb.ResourceInfo{ResourceType: "user", ResourceName: "u-1"},
	}}
	e := domainerr.E(errorspb.Code_NOT_FOUND, d)
	if len(e.Details()) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(e.Details()))
	}
}
```

- [ ] **Step 2: Run test, see it fail**

Run:
```bash
go test ./internal/core/domainerr/... 2>&1 | head -5
```
Expected: compile error (package missing).

- [ ] **Step 3: Implement `error.go`**

Create `internal/core/domainerr/error.go`:

```go
package domainerr

import (
	"errors"

	errorspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/errors"
)

// Error wraps a proto Code + structured details. Carries no human-readable
// message — all rendering happens client-side from the Code enum.
type Error struct {
	code    errorspb.Code
	details []*errorspb.Detail
	cause   error
}

// E builds an Error. Optional cause via WithCause.
func E(code errorspb.Code, details ...*errorspb.Detail) *Error {
	return &Error{code: code, details: details}
}

func (e *Error) Code() errorspb.Code         { return e.code }
func (e *Error) Details() []*errorspb.Detail { return e.details }
func (e *Error) Unwrap() error               { return e.cause }

// Error returns the enum name for diagnostic logging; never user-facing.
func (e *Error) Error() string { return e.code.String() }

// Is implements errors.Is matching by Code only.
func (e *Error) Is(target error) bool {
	var c *codefulErr
	if errors.As(target, &c) {
		return e.code == c.code
	}
	other, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.code == other.code
}

// WithCause attaches an underlying cause for unwrap chains.
func (e *Error) WithCause(cause error) *Error {
	e.cause = cause
	return e
}
```

- [ ] **Step 4: Implement `codes.go` (matcher helpers)**

Create `internal/core/domainerr/codes.go`:

```go
package domainerr

import (
	errorspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/errors"
)

// codefulErr is a tiny error matcher used by errors.Is to compare against
// a Code without constructing a full Error.
type codefulErr struct{ code errorspb.Code }

func (c *codefulErr) Error() string { return c.code.String() }

// Codeful returns an error sentinel for a specific Code, usable with errors.Is.
func Codeful(code errorspb.Code) error { return &codefulErr{code: code} }

// Common builders — strictly proto Code, never strings.

func NotFound(details ...*errorspb.Detail) *Error {
	return E(errorspb.Code_NOT_FOUND, details...)
}

func AlreadyExists(details ...*errorspb.Detail) *Error {
	return E(errorspb.Code_ALREADY_EXISTS, details...)
}

func PermissionDenied(details ...*errorspb.Detail) *Error {
	return E(errorspb.Code_PERMISSION_DENIED, details...)
}

func Unauthenticated(details ...*errorspb.Detail) *Error {
	return E(errorspb.Code_UNAUTHENTICATED, details...)
}

func InvalidArgument(details ...*errorspb.Detail) *Error {
	return E(errorspb.Code_INVALID_ARGUMENT, details...)
}

func FailedPrecondition(details ...*errorspb.Detail) *Error {
	return E(errorspb.Code_FAILED_PRECONDITION, details...)
}

func ResourceInfo(resourceType, name string) *errorspb.Detail {
	return &errorspb.Detail{Kind: &errorspb.Detail_ResourceInfo{
		ResourceInfo: &errorspb.ResourceInfo{ResourceType: resourceType, ResourceName: name},
	}}
}

func ErrorInfo(reason, domain string, metadata map[string]string) *errorspb.Detail {
	return &errorspb.Detail{Kind: &errorspb.Detail_ErrorInfo{
		ErrorInfo: &errorspb.ErrorInfo{Reason: reason, Domain: domain, Metadata: metadata},
	}}
}

func FieldViolation(field, reason string) *errorspb.Detail {
	return &errorspb.Detail{Kind: &errorspb.Detail_FieldViolation{
		FieldViolation: &errorspb.FieldViolation{Field: field, Description: reason},
	}}
}
```

- [ ] **Step 5: Run tests, expect PASS**

Run:
```bash
go test ./internal/core/domainerr/... -v
```
Expected: all three tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/core/domainerr/
git commit -m "feat(domainerr): code-only error wrapper with structured details"
```

---

## Task 7: Implement ULID factory

**Files:**
- Create: `internal/core/ids/ulid.go`, `ulid_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/core/ids/ulid_test.go`:

```go
package ids_test

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
)

func TestNewULIDIs26Chars(t *testing.T) {
	id := ids.New()
	if len(id) != 26 {
		t.Fatalf("expected 26 chars, got %d (%q)", len(id), id)
	}
	if strings.ToUpper(id) != id {
		t.Fatalf("ULID must be uppercase Crockford base32, got %q", id)
	}
}

func TestULIDsMonotonic(t *testing.T) {
	prev := ids.New()
	for i := 0; i < 1000; i++ {
		next := ids.New()
		if next <= prev {
			t.Fatalf("non-monotonic at i=%d: prev=%s next=%s", i, prev, next)
		}
		prev = next
	}
}
```

- [ ] **Step 2: Run test, see it fail**

Run:
```bash
go test ./internal/core/ids/... 2>&1 | head -5
```
Expected: compile error.

- [ ] **Step 3: Implement**

Create `internal/core/ids/ulid.go`:

```go
package ids

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	mu      sync.Mutex
	entropy = ulid.Monotonic(rand.Reader, 0)
)

// New returns a fresh monotonic ULID (26 Crockford-base32 chars).
func New() string {
	mu.Lock()
	defer mu.Unlock()
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}
```

- [ ] **Step 4: Run tests**

Run:
```bash
go test ./internal/core/ids/... -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/core/ids/
git commit -m "feat(ids): monotonic ULID factory"
```

---

## Task 8: Implement in-memory event bus

**Files:**
- Create: `internal/core/eventing/bus.go`, `events.go`, `bus_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/core/eventing/bus_test.go`:

```go
package eventing_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
)

type pingEvent struct{ N int }

func (pingEvent) Topic() eventing.Topic { return "test.ping" }

func TestBusDelivers(t *testing.T) {
	bus := eventing.NewInMemoryBus()
	defer bus.Close()

	var count int32
	unsub := bus.Subscribe("test.ping", func(ctx context.Context, evt eventing.Event) {
		atomic.AddInt32(&count, 1)
	})
	defer unsub()

	bus.Publish(context.Background(), pingEvent{N: 1})
	bus.Publish(context.Background(), pingEvent{N: 2})

	deadline := time.Now().Add(time.Second)
	for atomic.LoadInt32(&count) < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("expected 2 deliveries, got %d", atomic.LoadInt32(&count))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	bus := eventing.NewInMemoryBus()
	defer bus.Close()

	var count int32
	unsub := bus.Subscribe("test.ping", func(ctx context.Context, evt eventing.Event) {
		atomic.AddInt32(&count, 1)
	})
	unsub()

	bus.Publish(context.Background(), pingEvent{N: 1})
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&count) != 0 {
		t.Fatalf("expected 0 deliveries after unsubscribe, got %d", atomic.LoadInt32(&count))
	}
}
```

- [ ] **Step 2: Run, expect fail**

```bash
go test ./internal/core/eventing/... 2>&1 | head -5
```
Expected: compile error.

- [ ] **Step 3: Implement bus**

Create `internal/core/eventing/bus.go`:

```go
package eventing

import (
	"context"
	"sync"
)

// Topic identifies a category of events.
type Topic string

// Event carries its own Topic.
type Event interface {
	Topic() Topic
}

// Handler reacts to an Event.
type Handler func(ctx context.Context, evt Event)

// Bus is the public event bus interface.
type Bus interface {
	Subscribe(topic Topic, h Handler) (unsubscribe func())
	Publish(ctx context.Context, evt Event)
	Close()
}

// NewInMemoryBus returns a non-blocking in-process pub/sub.
func NewInMemoryBus() Bus {
	return &inMemoryBus{subs: make(map[Topic][]*sub)}
}

type sub struct {
	id      uint64
	handler Handler
}

type inMemoryBus struct {
	mu     sync.RWMutex
	subs   map[Topic][]*sub
	nextID uint64
	closed bool
}

func (b *inMemoryBus) Subscribe(topic Topic, h Handler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return func() {}
	}
	b.nextID++
	s := &sub{id: b.nextID, handler: h}
	b.subs[topic] = append(b.subs[topic], s)
	id := s.id
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		list := b.subs[topic]
		for i, cur := range list {
			if cur.id == id {
				b.subs[topic] = append(list[:i], list[i+1:]...)
				return
			}
		}
	}
}

func (b *inMemoryBus) Publish(ctx context.Context, evt Event) {
	b.mu.RLock()
	subs := append([]*sub(nil), b.subs[evt.Topic()]...)
	b.mu.RUnlock()
	for _, s := range subs {
		go s.handler(ctx, evt)
	}
}

func (b *inMemoryBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	b.subs = nil
}
```

- [ ] **Step 4: Implement domain events skeleton**

Create `internal/core/eventing/events.go`:

```go
package eventing

const (
	TopicUserCreated     Topic = "iam.user.created"
	TopicTenantCreated   Topic = "iam.tenant.created"
	TopicMemberAdded     Topic = "iam.member.added"
	TopicTestRunLaunched Topic = "testing.test_run.launched"
	TopicTestRunDone     Topic = "testing.test_run.done"
	TopicNodeRunDone     Topic = "system.node_run.done"
	TopicDagRunDone      Topic = "system.dag_run.done"
	TopicWebhookEmit     Topic = "ops.webhook.emit"
)

// Concrete events implement the Event interface. Each new event = new type +
// Topic() method. Keep payloads small; consumers query DB for detail.

type UserCreated struct{ UserID string }

func (UserCreated) Topic() Topic { return TopicUserCreated }

type TenantCreated struct{ TenantID string }

func (TenantCreated) Topic() Topic { return TopicTenantCreated }

type MemberAdded struct{ UserID, TenantID, Role string }

func (MemberAdded) Topic() Topic { return TopicMemberAdded }

type TestRunLaunched struct{ TenantID, TestRunID, DagRunID string }

func (TestRunLaunched) Topic() Topic { return TopicTestRunLaunched }

type TestRunDone struct {
	TenantID, TestRunID, DagRunID string
	Success                       bool
}

func (TestRunDone) Topic() Topic { return TopicTestRunDone }

type NodeRunDone struct {
	NodeRunID, DagRunID string
	Status              string
}

func (NodeRunDone) Topic() Topic { return TopicNodeRunDone }

type DagRunDone struct {
	DagRunID string
	Success  bool
}

func (DagRunDone) Topic() Topic { return TopicDagRunDone }
```

- [ ] **Step 5: Run tests**

Run:
```bash
go test ./internal/core/eventing/... -v
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/core/eventing/
git commit -m "feat(eventing): in-memory pub/sub bus + typed domain events"
```

---

## Task 9: Configurator (YAML + env)

**Files:**
- Create: `internal/core/configurator/config.go`, `load.go`, `load_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/core/configurator/load_test.go`:

```go
package configurator_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/core/configurator"
)

func TestLoadYAMLWithEnvSubstitution(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	body := `
server:
  http_addr: ":8080"
postgres:
  dsn: "postgres://stroppy:${PG_PASSWORD}@127.0.0.1:5432/stroppy?sslmode=disable"
  max_conns: 25
auth:
  jwt_secret_env: "JWT_SECRET"
  access_ttl: "15m"
  refresh_ttl: "720h"
workers:
  node_workers: 4
  scheduler_tick: "5s"
  webhook_workers: 2
  recovery_on_start: true
log:
  level: "info"
  format: "json"
`
	if err := os.WriteFile(cfgPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PG_PASSWORD", "secret123")

	cfg, err := configurator.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.HTTPAddr != ":8080" {
		t.Errorf("http_addr: %q", cfg.Server.HTTPAddr)
	}
	if cfg.Postgres.DSN != "postgres://stroppy:secret123@127.0.0.1:5432/stroppy?sslmode=disable" {
		t.Errorf("dsn substitution: %q", cfg.Postgres.DSN)
	}
	if cfg.Auth.AccessTTL != 15*time.Minute {
		t.Errorf("access_ttl: %v", cfg.Auth.AccessTTL)
	}
	if !cfg.Workers.RecoveryOnStart {
		t.Errorf("recovery_on_start should be true")
	}
}
```

- [ ] **Step 2: Run, expect fail**

```bash
go test ./internal/core/configurator/... 2>&1 | head -5
```
Expected: compile error.

- [ ] **Step 3: Implement `config.go`**

Create `internal/core/configurator/config.go`:

```go
package configurator

import "time"

type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Postgres    PostgresConfig    `mapstructure:"postgres"`
	Valkey      ValkeyConfig      `mapstructure:"valkey"`
	S3          S3Config          `mapstructure:"s3"`
	Victoria    VictoriaConfig    `mapstructure:"victoria"`
	Terraform   TerraformConfig   `mapstructure:"terraform"`
	Stroppy     StroppyConfig     `mapstructure:"stroppy"`
	Auth        AuthConfig        `mapstructure:"auth"`
	Idempotency IdempotencyConfig `mapstructure:"idempotency"`
	Workers     WorkersConfig     `mapstructure:"workers"`
	Features    FeaturesConfig    `mapstructure:"features"`
	Log         LogConfig         `mapstructure:"log"`
}

type ServerConfig struct {
	HTTPAddr string `mapstructure:"http_addr"`
}

type PostgresConfig struct {
	DSN      string `mapstructure:"dsn"`
	MaxConns int    `mapstructure:"max_conns"`
}

type ValkeyConfig struct {
	Addr string `mapstructure:"addr"`
	DB   int    `mapstructure:"db"`
}

type S3Config struct {
	Endpoint     string `mapstructure:"endpoint"`
	Bucket       string `mapstructure:"bucket"`
	Region       string `mapstructure:"region"`
	AccessKeyEnv string `mapstructure:"access_key_env"`
	SecretKeyEnv string `mapstructure:"secret_key_env"`
	UsePathStyle bool   `mapstructure:"use_path_style"`
}

type VictoriaConfig struct {
	PushURL  string `mapstructure:"push_url"`
	QueryURL string `mapstructure:"query_url"`
}

type TerraformConfig struct {
	BinaryPath  string `mapstructure:"binary_path"`
	WorkdirRoot string `mapstructure:"workdir_root"`
}

type StroppyConfig struct {
	DefaultVersion string `mapstructure:"default_version"`
	BinariesDir    string `mapstructure:"binaries_dir"`
}

type AuthConfig struct {
	JWTSecretEnv string        `mapstructure:"jwt_secret_env"`
	AccessTTL    time.Duration `mapstructure:"access_ttl"`
	RefreshTTL   time.Duration `mapstructure:"refresh_ttl"`
}

type IdempotencyConfig struct {
	Enabled bool          `mapstructure:"enabled"`
	TTL     time.Duration `mapstructure:"ttl"`
}

type WorkersConfig struct {
	NodeWorkers     int           `mapstructure:"node_workers"`
	SchedulerTick   time.Duration `mapstructure:"scheduler_tick"`
	WebhookWorkers  int           `mapstructure:"webhook_workers"`
	RecoveryOnStart bool          `mapstructure:"recovery_on_start"`
}

type FeaturesConfig struct {
	InitialAdminEmail       string `mapstructure:"initial_admin_email"`
	InitialAdminPasswordEnv string `mapstructure:"initial_admin_password_env"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}
```

- [ ] **Step 4: Implement `load.go`**

Create `internal/core/configurator/load.go`:

```go
package configurator

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Load reads YAML from path, expands ${ENV_VAR} references against os.Environ.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("configurator: read %s: %w", path, err)
	}
	expanded := os.ExpandEnv(string(raw))

	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytesReader(expanded)); err != nil {
		return nil, fmt.Errorf("configurator: parse: %w", err)
	}
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("configurator: unmarshal: %w", err)
	}
	return cfg, nil
}

func bytesReader(s string) *stringsReader { return &stringsReader{s: s} }

type stringsReader struct {
	s   string
	off int
}

func (r *stringsReader) Read(p []byte) (int, error) {
	if r.off >= len(r.s) {
		return 0, fmtErrEOF()
	}
	n := copy(p, r.s[r.off:])
	r.off += n
	return n, nil
}

func fmtErrEOF() error { return fmtErr("EOF") }

func fmtErr(s string) error { return &simpleErr{s} }

type simpleErr struct{ s string }

func (e *simpleErr) Error() string { return e.s }
```

Note: viper's `ReadConfig` requires `io.Reader`; std `strings.NewReader` works — use it instead:

Replace the helper block at the bottom of `load.go` with:

```go
import "strings"

func bytesReader(s string) *strings.Reader { return strings.NewReader(s) }
```

…and delete the `stringsReader`, `fmtErrEOF`, `fmtErr`, `simpleErr` definitions. Final `load.go`:

```go
package configurator

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("configurator: read %s: %w", path, err)
	}
	expanded := os.ExpandEnv(string(raw))

	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader(expanded)); err != nil {
		return nil, fmt.Errorf("configurator: parse: %w", err)
	}
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("configurator: unmarshal: %w", err)
	}
	return cfg, nil
}
```

- [ ] **Step 5: Run tests**

Run:
```bash
go test ./internal/core/configurator/... -v
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/core/configurator/
git commit -m "feat(configurator): YAML loader with env-var expansion"
```

---

## Task 10: Postgres pool factory + ratel-migrate runner

**Files:**
- Create: `internal/infrastructure/postgres/postgres.go`

- [ ] **Step 1: Implement**

Create `internal/infrastructure/postgres/postgres.go`:

```go
package postgres

import (
	"context"
	"fmt"
	"hash/fnv"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yaroher/ratel/pkg/migrate"

	"github.com/stroppy-io/stroppy-cloud/internal/core/configurator"
	"github.com/stroppy-io/stroppy-cloud/internal/core/logger"
)

const migrationSchema = "public"

// MigrationContent matches komeet's alias for embedded migration FSs.
type MigrationContent = fs.FS

// New builds a pgxpool from the configurator entry.
func New(cfg configurator.PostgresConfig) (*pgxpool.Pool, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("postgres: DSN required")
	}
	pcfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse DSN: %w", err)
	}
	if cfg.MaxConns > 0 {
		pcfg.MaxConns = int32(cfg.MaxConns)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), pcfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: new pool: %w", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return pool, nil
}

// MigrateAtlas runs ratel-managed migrations.
func MigrateAtlas(pool *pgxpool.Pool, migrations ...MigrationContent) error {
	return migrate.Migrate(pool, logger.Global().Named("migrate"), migrationSchema, migrations...)
}

// MigrateWithLock takes a pg_advisory_lock to prevent concurrent
// migrations across replicas, then applies migrations.
func MigrateWithLock(pool *pgxpool.Pool, lockName string, migrations ...MigrationContent) error {
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("postgres: acquire conn for lock: %w", err)
	}
	defer conn.Release()

	lockKey := int64(hash64(lockName))
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockKey); err != nil {
		return fmt.Errorf("postgres: advisory_lock: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockKey)
	}()
	return MigrateAtlas(pool, migrations...)
}

func hash64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}
```

- [ ] **Step 2: Build**

Run:
```bash
go build ./internal/infrastructure/postgres/...
```
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/infrastructure/postgres/postgres.go
git commit -m "feat(postgres): pool factory + ratel-migrate runner with advisory-lock variant"
```

---

## Task 11: testcontainer Postgres helper

**Files:**
- Create: `internal/testutil/pgcontainer/container.go`, `schema.go`

- [ ] **Step 1: Implement `container.go`**

Create `internal/testutil/pgcontainer/container.go`:

```go
package pgcontainer

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/migrations"
	pgxinfra "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
)

var (
	once    sync.Once
	rootDSN string
	rootErr error
	rootCnt testcontainers.Container
)

// Shared starts (once per test process) a Postgres container, returns its DSN.
func Shared(t *testing.T) string {
	t.Helper()
	once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		c, err := postgres.Run(ctx,
			"postgres:18-alpine",
			postgres.WithDatabase("stroppy"),
			postgres.WithUsername("stroppy"),
			postgres.WithPassword("stroppy"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).WithStartupTimeout(2*time.Minute),
			),
		)
		if err != nil {
			rootErr = err
			return
		}
		rootCnt = c
		dsn, err := c.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			rootErr = err
			return
		}
		rootDSN = dsn
	})
	if rootErr != nil {
		t.Fatalf("pgcontainer: shared container start failed: %v", rootErr)
	}
	return rootDSN
}

// Bootstrap returns a pool connected to the shared container with migrations
// applied. Caller is responsible for calling t.Cleanup to close the pool.
func Bootstrap(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := Shared(t)
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgcontainer: pool: %v", err)
	}
	if err := pgxinfra.MigrateAtlas(pool, migrations.Content); err != nil {
		pool.Close()
		t.Fatalf("pgcontainer: migrate: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}
```

- [ ] **Step 2: Implement `schema.go` (per-test isolation)**

Create `internal/testutil/pgcontainer/schema.go`:

```go
package pgcontainer

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
)

// IsolatedPool returns a pool restricted to a fresh per-test schema.
// All tables from migrations are recreated inside that schema; the schema
// is dropped on test cleanup. Multiple tests can run in parallel.
func IsolatedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := Shared(t)
	schema := "t_" + strings.ToLower(ids.New())

	// Connect to default DB first to create schema.
	root, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgcontainer: root pool: %v", err)
	}
	defer root.Close()

	if _, err := root.Exec(context.Background(), fmt.Sprintf(`CREATE SCHEMA %q`, schema)); err != nil {
		t.Fatalf("pgcontainer: create schema: %v", err)
	}

	// Open a new pool with search_path pinned.
	cfg, _ := pgxpool.ParseConfig(dsn)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("pgcontainer: scoped pool: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		cleanup, _ := pgxpool.New(context.Background(), dsn)
		defer cleanup.Close()
		_, _ = cleanup.Exec(context.Background(), fmt.Sprintf(`DROP SCHEMA %q CASCADE`, schema))
	})

	// Apply migrations into this schema. Ratel's migrate respects search_path.
	return pool
}
```

Note: Because ratel migrations include literal `"public"."tablename"` qualified names, per-test schema isolation needs migrations rewritten or a different strategy. For Phase 0-2 we use `Bootstrap` (shared schema, tests must clean up rows) and switch to `IsolatedPool` once ratel supports a schema-prefix. Keep both helpers but use `Bootstrap` from fixtures for now.

- [ ] **Step 3: Build**

Run:
```bash
go build ./internal/testutil/pgcontainer/...
```
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/testutil/pgcontainer/
git commit -m "feat(testutil): pgcontainer with Bootstrap + IsolatedPool helpers"
```

---

## Task 12: Fixture factory base

**Files:**
- Create: `internal/testutil/fixture/fixture.go`, `iam.go`

- [ ] **Step 1: Implement `fixture.go`**

Create `internal/testutil/fixture/fixture.go`:

```go
package fixture

import (
	"context"
	"testing"

	"github.com/avito-tech/go-transaction-manager/trm/manager"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/pgcontainer"
)

// F holds wired infrastructure + service instances for a single test.
type F struct {
	T        *testing.T
	Ctx      context.Context
	Pool     *pgxpool.Pool
	Executor any // sqlexec.TxExecutor — set in concrete fixture builders
	TxMgr    *manager.Manager
	Events   eventing.Bus
}

// New returns a base fixture. Test-specific factories (e.g. NewIAM) wire
// services on top.
func New(t *testing.T) *F {
	t.Helper()
	pool := pgcontainer.Bootstrap(t)
	exec, mgr, err := pgtx.NewTxFlow(pool, pgtx.ReadCommittedSettings())
	if err != nil {
		t.Fatalf("fixture: tx flow: %v", err)
	}
	bus := eventing.NewInMemoryBus()
	t.Cleanup(bus.Close)
	return &F{
		T:        t,
		Ctx:      context.Background(),
		Pool:     pool,
		Executor: exec,
		TxMgr:    mgr,
		Events:   bus,
	}
}
```

- [ ] **Step 2: Stub IAM fixture builder**

Create `internal/testutil/fixture/iam.go`:

```go
package fixture

// IAM-specific helpers are populated in plan task 14 (after iam service exists).
// This file reserves the import so service tests can reference
// fixture.NewIAM(t) without further plumbing once the service lands.
```

- [ ] **Step 3: Build**

Run:
```bash
go build ./internal/testutil/...
```
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/testutil/fixture/
git commit -m "feat(testutil): fixture base with pool/tx/events wiring"
```

---

## Task 13: IAM service skeleton + repos

**Files:**
- Create: `internal/domain/services/iam/service.go`, `passwords.go`, `jwt.go`

- [ ] **Step 1: Implement `service.go`**

Create `internal/domain/services/iam/service.go`:

```go
package iam

import (
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"

	"github.com/stroppy-io/stroppy-cloud/internal/core/configurator"
	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// Service owns all IAM tables (users, tenants, tenant_members, refresh_tokens,
// api_tokens) and exposes business methods consumed by the Connect handler.
type Service struct {
	*tracing.Entity

	userRepo    *repository.ProtoRepository[iampb.UserAlias, iampb.UserColumnAlias, *iampb.UserScanner, *iampb.User]
	tenantRepo  *repository.ProtoRepository[iampb.TenantAlias, iampb.TenantColumnAlias, *iampb.TenantScanner, *iampb.Tenant]
	memberRepo  *repository.ProtoRepository[iampb.TenantMemberAlias, iampb.TenantMemberColumnAlias, *iampb.TenantMemberScanner, *iampb.TenantMember]
	refreshRepo *repository.ProtoRepository[iampb.RefreshTokenAlias, iampb.RefreshTokenColumnAlias, *iampb.RefreshTokenScanner, *iampb.RefreshToken]
	tokenRepo   *repository.ProtoRepository[iampb.ApiTokenAlias, iampb.ApiTokenColumnAlias, *iampb.ApiTokenScanner, *iampb.ApiToken]

	txManager pgtx.TxManager
	events    eventing.Bus
	cfg       configurator.AuthConfig
	jwtSecret []byte
}

// New wires the service.
func New(executor exec.DB, txManager pgtx.TxManager, events eventing.Bus, cfg configurator.AuthConfig, jwtSecret []byte) *Service {
	return &Service{
		Entity: tracing.NewEntity("iam.Service"),
		userRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(iampb.Users.Table, executor),
			iampb.UserConverter,
		),
		tenantRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(iampb.Tenants.Table, executor),
			iampb.TenantConverter,
		),
		memberRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(iampb.TenantMembers.Table, executor),
			iampb.TenantMemberConverter,
		),
		refreshRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(iampb.RefreshTokens.Table, executor),
			iampb.RefreshTokenConverter,
		),
		tokenRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(iampb.ApiTokens.Table, executor),
			iampb.ApiTokenConverter,
		),
		txManager: txManager,
		events:    events,
		cfg:       cfg,
		jwtSecret: jwtSecret,
	}
}
```

- [ ] **Step 2: Implement `passwords.go`**

Create `internal/domain/services/iam/passwords.go`:

```go
package iam

import "golang.org/x/crypto/bcrypt"

func hashPassword(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

func verifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
```

- [ ] **Step 3: Implement `jwt.go`**

Create `internal/domain/services/iam/jwt.go`:

```go
package iam

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type accessClaims struct {
	UserID string `json:"sub"`
	JTI    string `json:"jti"`
	jwt.RegisteredClaims
}

func (s *Service) signAccessToken(userID, jti string) (string, error) {
	now := time.Now()
	claims := accessClaims{
		UserID: userID,
		JTI:    jti,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.AccessTTL)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(s.jwtSecret)
}

// VerifyAccessToken parses a JWT, returns (user_id, jti).
func (s *Service) VerifyAccessToken(token string) (string, string, error) {
	parsed, err := jwt.ParseWithClaims(token, &accessClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected sign method: %v", t.Method)
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return "", "", err
	}
	claims, ok := parsed.Claims.(*accessClaims)
	if !ok || !parsed.Valid {
		return "", "", fmt.Errorf("invalid token")
	}
	return claims.UserID, claims.JTI, nil
}
```

- [ ] **Step 4: Build**

Run:
```bash
go build ./internal/domain/services/iam/...
```
Expected: success.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/services/iam/
git commit -m "feat(iam): service skeleton with repos + password/JWT helpers"
```

---

## Task 14: IAM users CRUD

**Files:**
- Create: `internal/domain/services/iam/users.go`
- Append to: `internal/testutil/fixture/iam.go`

- [ ] **Step 1: Write failing test**

Create `internal/domain/services/iam/integration_test.go`:

```go
package iam_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestCreateUser(t *testing.T) {
	f := fixture.NewIAM(t)

	user := &iampb.User{
		Email:    "alice@example.com",
		Nickname: "alice",
	}
	created, err := f.IAM.CreateUser(context.Background(), user, "P@ssw0rd!")
	require.NoError(t, err)
	require.NotEmpty(t, created.GetId().GetValue())

	got, err := f.IAM.GetUserByID(context.Background(), created.GetId())
	require.NoError(t, err)
	require.Equal(t, "alice@example.com", got.GetEmail())
}

func TestCreateUserDuplicateEmail(t *testing.T) {
	f := fixture.NewIAM(t)
	_, err := f.IAM.CreateUser(context.Background(), &iampb.User{Email: "dup@e.com", Nickname: "dup1"}, "P@ssw0rd!")
	require.NoError(t, err)
	_, err = f.IAM.CreateUser(context.Background(), &iampb.User{Email: "dup@e.com", Nickname: "dup2"}, "P@ssw0rd!")
	require.Error(t, err)
}
```

- [ ] **Step 2: Implement `users.go`**

Create `internal/domain/services/iam/users.go`:

```go
package iam

import (
	"context"
	"errors"

	"github.com/yaroher/ratel/pkg/dml/set"
	"github.com/yaroher/ratel/pkg/repository"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	errorspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/errors"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CreateUser persists a new user with a hashed password.
func (s *Service) CreateUser(ctx context.Context, user *iampb.User, password string) (*iampb.User, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CreateUser",
		func(ctx context.Context, _ any) (*iampb.User, error) {
			return pgtx.WithSerializableRet(ctx, s.txManager,
				func(ctx context.Context) (*iampb.User, error) {
					hash, err := hashPassword(password)
					if err != nil {
						return nil, domainerr.E(errorspb.Code_INTERNAL).WithCause(err)
					}
					now := timestamppb.Now()
					user.Id = &iampb.UserId{Value: ids.New()}
					user.PasswordHash = hash
					user.Timestamps = &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now}

					if err := s.userRepo.Insert(ctx, user); err != nil {
						if isUniqueViolation(err) {
							return nil, domainerr.AlreadyExists(domainerr.ResourceInfo("user", user.GetEmail()))
						}
						return nil, err
					}
					s.events.Publish(ctx, eventingUserCreated(user.GetId().GetValue()))
					return user, nil
				})
		})
}

// GetUserByID retrieves a user.
func (s *Service) GetUserByID(ctx context.Context, id *iampb.UserId) (*iampb.User, error) {
	u, err := s.userRepo.SelectOne(ctx, set.Eq(iampb.UserColumnId, id.GetValue()))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("user", id.GetValue()))
		}
		return nil, err
	}
	return u, nil
}

// GetUserByEmail looks up by email.
func (s *Service) GetUserByEmail(ctx context.Context, email string) (*iampb.User, error) {
	u, err := s.userRepo.SelectOne(ctx, set.Eq(iampb.UserColumnEmail, email))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("user", email))
		}
		return nil, err
	}
	return u, nil
}

// ListUsers returns all users (admin only).
func (s *Service) ListUsers(ctx context.Context) ([]*iampb.User, error) {
	return s.userRepo.Select(ctx)
}
```

- [ ] **Step 3: Implement `isUniqueViolation` helper + eventing alias**

Append to `internal/domain/services/iam/users.go` (top, after imports):

```go
import "github.com/jackc/pgx/v5/pgconn"

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
```

Append to top of `users.go` (helper for events):

```go
import "github.com/stroppy-io/stroppy-cloud/internal/core/eventing"

func eventingUserCreated(id string) eventing.Event {
	return eventing.UserCreated{UserID: id}
}
```

Final `users.go` is the concatenation of the above; remove duplicate import lines.

- [ ] **Step 4: Add `fixture.NewIAM`**

Replace `internal/testutil/fixture/iam.go` with:

```go
package fixture

import (
	"testing"
	"time"

	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"

	"github.com/stroppy-io/stroppy-cloud/internal/core/configurator"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/iam"
)

// IAMFixture extends the base fixture with a wired iam.Service.
type IAMFixture struct {
	*F
	IAM *iam.Service
}

// NewIAM builds an IAMFixture against the shared Postgres container.
func NewIAM(t *testing.T) *IAMFixture {
	t.Helper()
	base := New(t)
	exec := base.Executor.(*sqlexec.TxExecutor)
	svc := iam.New(
		exec,
		base.TxMgr,
		base.Events,
		configurator.AuthConfig{
			AccessTTL:  15 * time.Minute,
			RefreshTTL: 720 * time.Hour,
		},
		[]byte("test-jwt-secret-32-bytes-long!!"),
	)
	return &IAMFixture{F: base, IAM: svc}
}
```

- [ ] **Step 5: Run tests**

Run:
```bash
go test ./internal/domain/services/iam/... -v -run TestCreateUser
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/services/iam/users.go internal/domain/services/iam/integration_test.go internal/testutil/fixture/iam.go
git commit -m "feat(iam): create/get/list users + duplicate email handling"
```

---

## Task 15: IAM tenants + members CRUD

**Files:**
- Create: `internal/domain/services/iam/tenants.go`, `members.go`
- Append tests to: `internal/domain/services/iam/integration_test.go`

- [ ] **Step 1: Append tests**

Append to `internal/domain/services/iam/integration_test.go`:

```go
func TestCreateTenantAndMember(t *testing.T) {
	f := fixture.NewIAM(t)
	ctx := context.Background()

	user, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "owner@e.com", Nickname: "owner"}, "P@ssw0rd!")
	require.NoError(t, err)

	tenant, err := f.IAM.CreateTenant(ctx, &iampb.Tenant{Name: "Acme"}, user.GetId())
	require.NoError(t, err)
	require.NotEmpty(t, tenant.GetId().GetValue())

	ok, err := f.IAM.HasTenantRole(ctx, user.GetId(), tenant.GetId(), iampb.TenantRole_TENANT_ROLE_OWNER)
	require.NoError(t, err)
	require.True(t, ok)
}
```

- [ ] **Step 2: Implement `tenants.go`**

Create `internal/domain/services/iam/tenants.go`:

```go
package iam

import (
	"context"
	"errors"

	"github.com/yaroher/ratel/pkg/dml/set"
	"github.com/yaroher/ratel/pkg/repository"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	errorspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/errors"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CreateTenant creates a tenant and adds the owner as TENANT_ROLE_OWNER in one tx.
func (s *Service) CreateTenant(ctx context.Context, tenant *iampb.Tenant, ownerID *iampb.UserId) (*iampb.Tenant, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CreateTenant",
		func(ctx context.Context, _ any) (*iampb.Tenant, error) {
			return pgtx.WithSerializableRet(ctx, s.txManager,
				func(ctx context.Context) (*iampb.Tenant, error) {
					now := timestamppb.Now()
					tenant.Id = &iampb.TenantId{Value: ids.New()}
					tenant.Timestamps = &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now}

					if err := s.tenantRepo.Insert(ctx, tenant); err != nil {
						return nil, err
					}
					member := &iampb.TenantMember{
						Id:         &iampb.TenantMemberId{Value: ids.New()},
						UserId:     ownerID,
						TenantId:   tenant.GetId(),
						Role:       iampb.TenantRole_TENANT_ROLE_OWNER,
						Timestamps: &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now},
					}
					if err := s.memberRepo.Insert(ctx, member); err != nil {
						return nil, err
					}
					s.events.Publish(ctx, eventing.TenantCreated{TenantID: tenant.GetId().GetValue()})
					s.events.Publish(ctx, eventing.MemberAdded{
						UserID:   ownerID.GetValue(),
						TenantID: tenant.GetId().GetValue(),
						Role:     iampb.TenantRole_TENANT_ROLE_OWNER.String(),
					})
					return tenant, nil
				})
		})
}

func (s *Service) GetTenantByID(ctx context.Context, id *iampb.TenantId) (*iampb.Tenant, error) {
	t, err := s.tenantRepo.SelectOne(ctx, set.Eq(iampb.TenantColumnId, id.GetValue()))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("tenant", id.GetValue()))
		}
		return nil, err
	}
	return t, nil
}

func (s *Service) ListTenantsForUser(ctx context.Context, userID *iampb.UserId) ([]*iampb.Tenant, error) {
	members, err := s.memberRepo.Select(ctx, set.Eq(iampb.TenantMemberColumnUserId, userID.GetValue()))
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, nil
	}
	tenantIDs := make([]any, 0, len(members))
	for _, m := range members {
		tenantIDs = append(tenantIDs, m.GetTenantId().GetValue())
	}
	return s.tenantRepo.Select(ctx, set.In(iampb.TenantColumnId, tenantIDs...))
}
```

- [ ] **Step 3: Implement `members.go`**

Create `internal/domain/services/iam/members.go`:

```go
package iam

import (
	"context"
	"errors"

	"github.com/yaroher/ratel/pkg/dml/set"
	"github.com/yaroher/ratel/pkg/repository"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AddMember adds a user to a tenant with a role.
func (s *Service) AddMember(ctx context.Context, userID *iampb.UserId, tenantID *iampb.TenantId, role iampb.TenantRole) (*iampb.TenantMember, error) {
	now := timestamppb.Now()
	m := &iampb.TenantMember{
		Id:         &iampb.TenantMemberId{Value: ids.New()},
		UserId:     userID,
		TenantId:   tenantID,
		Role:       role,
		Timestamps: &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	if err := s.memberRepo.Insert(ctx, m); err != nil {
		return nil, err
	}
	s.events.Publish(ctx, eventing.MemberAdded{UserID: userID.GetValue(), TenantID: tenantID.GetValue(), Role: role.String()})
	return m, nil
}

// HasTenantRole returns true if the user is a member of the tenant at least at the minimum role.
func (s *Service) HasTenantRole(ctx context.Context, userID *iampb.UserId, tenantID *iampb.TenantId, min iampb.TenantRole) (bool, error) {
	m, err := s.memberRepo.SelectOne(ctx,
		set.And(
			set.Eq(iampb.TenantMemberColumnUserId, userID.GetValue()),
			set.Eq(iampb.TenantMemberColumnTenantId, tenantID.GetValue()),
		))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return m.GetRole() >= min, nil
}

// HasTenantMember is a convenience for "any role".
func (s *Service) HasTenantMember(ctx context.Context, userID *iampb.UserId, tenantID *iampb.TenantId) (bool, error) {
	return s.HasTenantRole(ctx, userID, tenantID, iampb.TenantRole_TENANT_ROLE_VIEWER)
}

func (s *Service) ListMembers(ctx context.Context, tenantID *iampb.TenantId) ([]*iampb.TenantMember, error) {
	return s.memberRepo.Select(ctx, set.Eq(iampb.TenantMemberColumnTenantId, tenantID.GetValue()))
}

func (s *Service) RemoveMember(ctx context.Context, id *iampb.TenantMemberId) error {
	_, err := s.memberRepo.Delete(ctx, set.Eq(iampb.TenantMemberColumnId, id.GetValue()))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domainerr.NotFound(domainerr.ResourceInfo("tenant_member", id.GetValue()))
		}
		return err
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

Run:
```bash
go test ./internal/domain/services/iam/... -v -run TestCreateTenant
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/services/iam/{tenants,members}.go internal/domain/services/iam/integration_test.go
git commit -m "feat(iam): tenant create + member CRUD + HasTenantRole"
```

---

## Task 16: IAM auth (Login + Logout + Refresh rotation)

**Files:**
- Create: `internal/domain/services/iam/auth.go`
- Append tests to: `internal/domain/services/iam/integration_test.go`

- [ ] **Step 1: Append tests**

Append to `internal/domain/services/iam/integration_test.go`:

```go
func TestLoginAndRefreshRotation(t *testing.T) {
	f := fixture.NewIAM(t)
	ctx := context.Background()

	_, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "user@e.com", Nickname: "user1"}, "P@ssw0rd!")
	require.NoError(t, err)

	pair1, err := f.IAM.Login(ctx, "user@e.com", "P@ssw0rd!")
	require.NoError(t, err)
	require.NotEmpty(t, pair1.GetAccessToken())
	require.NotEmpty(t, pair1.GetRefreshToken())

	pair2, err := f.IAM.RefreshTokens(ctx, pair1.GetRefreshToken())
	require.NoError(t, err)
	require.NotEqual(t, pair1.GetAccessToken(), pair2.GetAccessToken())
	require.NotEqual(t, pair1.GetRefreshToken(), pair2.GetRefreshToken())

	// Reuse old token → triggers family revoke
	_, err = f.IAM.RefreshTokens(ctx, pair1.GetRefreshToken())
	require.Error(t, err)

	// New token also invalid (family revoked)
	_, err = f.IAM.RefreshTokens(ctx, pair2.GetRefreshToken())
	require.Error(t, err)
}

func TestLoginWrongPassword(t *testing.T) {
	f := fixture.NewIAM(t)
	ctx := context.Background()

	_, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "u@e.com", Nickname: "u"}, "Correct123!")
	require.NoError(t, err)

	_, err = f.IAM.Login(ctx, "u@e.com", "wrong")
	require.Error(t, err)
}
```

- [ ] **Step 2: Implement `auth.go`**

Create `internal/domain/services/iam/auth.go`:

```go
package iam

import (
	"context"
	"errors"
	"time"

	"github.com/yaroher/ratel/pkg/dml/set"
	"github.com/yaroher/ratel/pkg/repository"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	errorspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/errors"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// Login validates credentials and returns a fresh TokenPair. Refresh token
// starts a new rotation family.
func (s *Service) Login(ctx context.Context, email, password string) (*iampb.TokenPair, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "Login",
		func(ctx context.Context, _ any) (*iampb.TokenPair, error) {
			return pgtx.WithSerializableRet(ctx, s.txManager,
				func(ctx context.Context) (*iampb.TokenPair, error) {
					user, err := s.GetUserByEmail(ctx, email)
					if err != nil {
						if errors.Is(err, domainerr.Codeful(errorspb.Code_NOT_FOUND)) {
							return nil, domainerr.Unauthenticated()
						}
						return nil, err
					}
					if !verifyPassword(user.GetPasswordHash(), password) {
						return nil, domainerr.Unauthenticated()
					}
					return s.issuePair(ctx, user.GetId(), ids.New() /* new family */)
				})
		})
}

// RefreshTokens rotates a refresh token. Reuse of a revoked token (post-rotation)
// revokes the entire family — incident-response signal.
func (s *Service) RefreshTokens(ctx context.Context, refreshTokenValue string) (*iampb.TokenPair, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "RefreshTokens",
		func(ctx context.Context, _ any) (*iampb.TokenPair, error) {
			return pgtx.WithSerializableRet(ctx, s.txManager,
				func(ctx context.Context) (*iampb.TokenPair, error) {
					tok, err := s.refreshRepo.SelectOne(ctx, set.Eq(iampb.RefreshTokenColumnToken, refreshTokenValue))
					if err != nil {
						if errors.Is(err, repository.ErrNotFound) {
							return nil, domainerr.Unauthenticated()
						}
						return nil, err
					}
					if tok.GetRevokedAt() != nil {
						// Reuse detection: revoke whole family.
						if err := s.revokeFamily(ctx, tok.GetFamilyId()); err != nil {
							return nil, err
						}
						return nil, domainerr.Unauthenticated()
					}
					if tok.GetExpiresAt().AsTime().Before(time.Now()) {
						return nil, domainerr.Unauthenticated()
					}

					// Revoke this token, issue successor in same family.
					_, err = s.refreshRepo.Update(ctx,
						set.Update(iampb.RefreshTokenColumnRevokedAt, timestamppb.Now()),
						set.Eq(iampb.RefreshTokenColumnId, tok.GetId().GetValue()),
					)
					if err != nil {
						return nil, err
					}
					return s.issuePair(ctx, tok.GetUserId(), tok.GetFamilyId())
				})
		})
}

// Logout revokes the specific refresh token.
func (s *Service) Logout(ctx context.Context, refreshTokenValue string) error {
	_, err := s.refreshRepo.Update(ctx,
		set.Update(iampb.RefreshTokenColumnRevokedAt, timestamppb.Now()),
		set.Eq(iampb.RefreshTokenColumnToken, refreshTokenValue),
	)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	return nil
}

func (s *Service) issuePair(ctx context.Context, userID *iampb.UserId, familyID string) (*iampb.TokenPair, error) {
	now := time.Now()
	jti := ids.New()
	access, err := s.signAccessToken(userID.GetValue(), jti)
	if err != nil {
		return nil, err
	}
	refreshValue := ids.New() + ids.New() // 52 chars opaque
	row := &iampb.RefreshToken{
		Id:         &iampb.RefreshTokenId{Value: ids.New()},
		UserId:     userID,
		FamilyId:   familyID,
		Jti:        jti,
		Token:      refreshValue,
		ExpiresAt:  timestamppb.New(now.Add(s.cfg.RefreshTTL)),
		Timestamps: &commonpb.Timestamps{CreatedAt: timestamppb.New(now), UpdatedAt: timestamppb.New(now)},
	}
	if err := s.refreshRepo.Insert(ctx, row); err != nil {
		return nil, err
	}
	return &iampb.TokenPair{
		AccessToken:           access,
		RefreshToken:          refreshValue,
		AccessTokenExpiresIn:  durationToProto(s.cfg.AccessTTL),
		RefreshTokenExpiresIn: durationToProto(s.cfg.RefreshTTL),
	}, nil
}

func (s *Service) revokeFamily(ctx context.Context, familyID string) error {
	_, err := s.refreshRepo.Update(ctx,
		set.Update(iampb.RefreshTokenColumnRevokedAt, timestamppb.Now()),
		set.And(
			set.Eq(iampb.RefreshTokenColumnFamilyId, familyID),
			set.IsNull(iampb.RefreshTokenColumnRevokedAt),
		),
	)
	return err
}

func durationToProto(d time.Duration) *durationproto.Duration {
	return &durationproto.Duration{Seconds: int64(d.Seconds())}
}
```

Add to top imports:
```go
import durationproto "google.golang.org/protobuf/types/known/durationpb"
```

- [ ] **Step 3: Run tests**

Run:
```bash
go test ./internal/domain/services/iam/... -v -run "TestLogin|TestRefresh"
```
Expected: PASS (including reuse-detection).

- [ ] **Step 4: Commit**

```bash
git add internal/domain/services/iam/auth.go internal/domain/services/iam/integration_test.go
git commit -m "feat(iam): Login + RefreshTokens rotation + family reuse-detection + Logout"
```

---

## Task 17: API tokens CRUD

**Files:**
- Create: `internal/domain/services/iam/api_tokens.go`
- Append test.

- [ ] **Step 1: Append test**

Append to `integration_test.go`:

```go
func TestApiTokenCreateAndVerify(t *testing.T) {
	f := fixture.NewIAM(t)
	ctx := context.Background()

	u, _ := f.IAM.CreateUser(ctx, &iampb.User{Email: "t@e.com", Nickname: "t"}, "P@ss123!")
	tn, _ := f.IAM.CreateTenant(ctx, &iampb.Tenant{Name: "T"}, u.GetId())

	token, plain, err := f.IAM.CreateApiToken(ctx, tn.GetId(), u.GetId(), "ci-pipeline")
	require.NoError(t, err)
	require.NotEmpty(t, plain)
	require.NotEqual(t, plain, token.GetHash())

	resolved, err := f.IAM.VerifyApiToken(ctx, plain)
	require.NoError(t, err)
	require.Equal(t, tn.GetId().GetValue(), resolved.GetTenantId().GetValue())
}
```

- [ ] **Step 2: Implement**

Create `internal/domain/services/iam/api_tokens.go`:

```go
package iam

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/yaroher/ratel/pkg/dml/set"
	"github.com/yaroher/ratel/pkg/repository"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// CreateApiToken creates a token tied to (tenant, user). Returns the proto
// row + the cleartext token (last chance to read it).
func (s *Service) CreateApiToken(ctx context.Context, tenantID *iampb.TenantId, userID *iampb.UserId, name string) (*iampb.ApiToken, string, error) {
	now := timestamppb.Now()
	plain := "sct_" + ids.New() + ids.New() // 56 chars
	hash := sha256Hex(plain)
	row := &iampb.ApiToken{
		Id:         &iampb.ApiTokenId{Value: ids.New()},
		TenantId:   tenantID,
		UserId:     userID,
		Name:       name,
		Hash:       hash,
		Timestamps: &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	if err := s.tokenRepo.Insert(ctx, row); err != nil {
		return nil, "", err
	}
	return row, plain, nil
}

// VerifyApiToken looks up a cleartext token, returns the persisted row.
func (s *Service) VerifyApiToken(ctx context.Context, plain string) (*iampb.ApiToken, error) {
	hash := sha256Hex(plain)
	row, err := s.tokenRepo.SelectOne(ctx, set.Eq(iampb.ApiTokenColumnHash, hash))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, domainerr.Unauthenticated()
		}
		return nil, err
	}
	if row.GetRevokedAt() != nil {
		return nil, domainerr.Unauthenticated()
	}
	return row, nil
}

// RevokeApiToken marks a token revoked.
func (s *Service) RevokeApiToken(ctx context.Context, id *iampb.ApiTokenId) error {
	_, err := s.tokenRepo.Update(ctx,
		set.Update(iampb.ApiTokenColumnRevokedAt, timestamppb.Now()),
		set.Eq(iampb.ApiTokenColumnId, id.GetValue()),
	)
	return err
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 3: Run tests**

Run:
```bash
go test ./internal/domain/services/iam/... -v -run TestApiToken
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/domain/services/iam/api_tokens.go internal/domain/services/iam/integration_test.go
git commit -m "feat(iam): API tokens with sha256 storage + verify/revoke"
```

---

## Task 18: Middleware base (recovery, requestID, logging, context)

**Files:**
- Create: `internal/transport/middleware/recovery.go`, `requestid.go`, `logging.go`, `context.go`

- [ ] **Step 1: `context.go`**

```go
package middleware

import "context"

type ctxKey int

const (
	keyUserID ctxKey = iota
	keyTenantID
	keyRequestID
)

func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyUserID, id)
}

func UserFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(keyUserID).(string)
	return v
}

func WithTenantID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyTenantID, id)
}

func TenantFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(keyTenantID).(string)
	return v
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyRequestID, id)
}

func RequestIDFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(keyRequestID).(string)
	return v
}
```

- [ ] **Step 2: `recovery.go`**

```go
package middleware

import (
	"context"
	"errors"
	"runtime/debug"

	"connectrpc.com/connect"
	"go.uber.org/zap"
)

func Recovery(log *zap.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (resp connect.AnyResponse, err error) {
			defer func() {
				if r := recover(); r != nil {
					log.Error("panic in handler",
						zap.Any("panic", r),
						zap.String("rpc", req.Spec().Procedure),
						zap.ByteString("stack", debug.Stack()),
					)
					err = connect.NewError(connect.CodeInternal, errors.New("internal error"))
				}
			}()
			return next(ctx, req)
		}
	}
}
```

- [ ] **Step 3: `requestid.go`**

```go
package middleware

import (
	"context"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
)

const HeaderRequestID = "X-Request-Id"

func RequestID() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			id := req.Header().Get(HeaderRequestID)
			if id == "" {
				id = ids.New()
			}
			ctx = WithRequestID(ctx, id)
			resp, err := next(ctx, req)
			if resp != nil {
				resp.Header().Set(HeaderRequestID, id)
			}
			return resp, err
		}
	}
}
```

- [ ] **Step 4: `logging.go`**

```go
package middleware

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"go.uber.org/zap"
)

func Logging(log *zap.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			fields := []zap.Field{
				zap.String("rpc", req.Spec().Procedure),
				zap.Duration("duration", time.Since(start)),
				zap.String("request_id", RequestIDFromCtx(ctx)),
			}
			if uid := UserFromCtx(ctx); uid != "" {
				fields = append(fields, zap.String("user_id", uid))
			}
			if tid := TenantFromCtx(ctx); tid != "" {
				fields = append(fields, zap.String("tenant_id", tid))
			}
			if err != nil {
				fields = append(fields, zap.Error(err))
				log.Warn("rpc.error", fields...)
			} else {
				log.Info("rpc.ok", fields...)
			}
			return resp, err
		}
	}
}
```

- [ ] **Step 5: Build**

```bash
go build ./internal/transport/middleware/...
```
Expected: success.

- [ ] **Step 6: Commit**

```bash
git add internal/transport/middleware/{context,recovery,requestid,logging}.go
git commit -m "feat(middleware): recovery + requestID + logging + context helpers"
```

---

## Task 19: Error-mapping middleware

**Files:**
- Create: `internal/transport/middleware/errmap.go`

- [ ] **Step 1: Implement**

```go
package middleware

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	errorspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/errors"
)

func ErrorMapper() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			resp, err := next(ctx, req)
			if err == nil {
				return resp, nil
			}
			return resp, toConnect(err)
		}
	}
}

func toConnect(err error) error {
	var de *domainerr.Error
	if !errors.As(err, &de) {
		// already a connect.Error or unknown — let it pass through
		var ce *connect.Error
		if errors.As(err, &ce) {
			return ce
		}
		return connect.NewError(connect.CodeInternal, errors.New(errorspb.Code_INTERNAL.String()))
	}
	cerr := connect.NewError(mapCode(de.Code()), errors.New(de.Code().String()))
	for _, d := range de.Details() {
		detail, derr := connect.NewErrorDetail(d)
		if derr == nil {
			cerr.AddDetail(detail)
		}
	}
	return cerr
}

func mapCode(c errorspb.Code) connect.Code {
	switch c {
	case errorspb.Code_OK:
		return connect.Code(0)
	case errorspb.Code_CANCELLED:
		return connect.CodeCanceled
	case errorspb.Code_INVALID_ARGUMENT:
		return connect.CodeInvalidArgument
	case errorspb.Code_DEADLINE_EXCEEDED:
		return connect.CodeDeadlineExceeded
	case errorspb.Code_NOT_FOUND:
		return connect.CodeNotFound
	case errorspb.Code_ALREADY_EXISTS:
		return connect.CodeAlreadyExists
	case errorspb.Code_PERMISSION_DENIED:
		return connect.CodePermissionDenied
	case errorspb.Code_RESOURCE_EXHAUSTED:
		return connect.CodeResourceExhausted
	case errorspb.Code_FAILED_PRECONDITION:
		return connect.CodeFailedPrecondition
	case errorspb.Code_ABORTED:
		return connect.CodeAborted
	case errorspb.Code_OUT_OF_RANGE:
		return connect.CodeOutOfRange
	case errorspb.Code_UNIMPLEMENTED:
		return connect.CodeUnimplemented
	case errorspb.Code_UNAVAILABLE:
		return connect.CodeUnavailable
	case errorspb.Code_DATA_LOSS:
		return connect.CodeDataLoss
	case errorspb.Code_UNAUTHENTICATED:
		return connect.CodeUnauthenticated
	default:
		return connect.CodeInternal
	}
}
```

- [ ] **Step 2: Build**

```bash
go build ./internal/transport/middleware/...
```
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/transport/middleware/errmap.go
git commit -m "feat(middleware): domainerr.Error → connect.Error mapping with details"
```

---

## Task 20: Auth + Tenant middleware

**Files:**
- Create: `internal/transport/middleware/auth.go`, `tenant.go`

- [ ] **Step 1: Define ports**

Create `internal/transport/middleware/auth.go`:

```go
package middleware

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// AuthPort is implemented by iam.Service.
type AuthPort interface {
	VerifyAccessToken(token string) (userID string, jti string, err error)
	VerifyApiToken(ctx context.Context, plain string) (*iampb.ApiToken, error)
}

// Auth returns a unary interceptor that validates the Authorization header.
// Bypass: methods in the bypassSet (full procedure path).
func Auth(svc AuthPort, bypass map[string]bool) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if bypass[req.Spec().Procedure] {
				return next(ctx, req)
			}
			header := req.Header().Get("Authorization")
			if header == "" {
				return nil, toConnect(domainerr.Unauthenticated())
			}
			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 {
				return nil, toConnect(domainerr.Unauthenticated())
			}
			scheme := strings.ToLower(parts[0])
			token := parts[1]
			switch scheme {
			case "bearer":
				userID, _, err := svc.VerifyAccessToken(token)
				if err != nil {
					return nil, toConnect(domainerr.Unauthenticated())
				}
				ctx = WithUserID(ctx, userID)
			case "token":
				row, err := svc.VerifyApiToken(ctx, token)
				if err != nil {
					return nil, toConnect(domainerr.Unauthenticated())
				}
				ctx = WithUserID(ctx, row.GetUserId().GetValue())
				ctx = WithTenantID(ctx, row.GetTenantId().GetValue())
			default:
				return nil, toConnect(domainerr.Unauthenticated())
			}
			return next(ctx, req)
		}
	}
}

// MustAuth fails if Auth bypass list missing — used for safety on dev.
var ErrAuthMisconfigured = errors.New("auth middleware: bypass list empty")
```

- [ ] **Step 2: `tenant.go`**

```go
package middleware

import (
	"context"
	"reflect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// TenantPort is implemented by iam.Service.
type TenantPort interface {
	HasTenantMember(ctx context.Context, userID *iampb.UserId, tenantID *iampb.TenantId) (bool, error)
}

// Tenant resolves tenant_id from request body or X-Tenant-Id header,
// verifies membership, injects into ctx. Bypass list = procedures that don't
// require tenant scope.
func Tenant(svc TenantPort, bypass map[string]bool) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if bypass[req.Spec().Procedure] {
				return next(ctx, req)
			}
			tenantID := tenantIDFromRequest(req)
			if tenantID == "" {
				return nil, toConnect(domainerr.PermissionDenied())
			}
			userID := UserFromCtx(ctx)
			if userID == "" {
				return nil, toConnect(domainerr.Unauthenticated())
			}
			ok, err := svc.HasTenantMember(ctx,
				&iampb.UserId{Value: userID},
				&iampb.TenantId{Value: tenantID},
			)
			if err != nil {
				return nil, toConnect(domainerr.PermissionDenied())
			}
			if !ok {
				return nil, toConnect(domainerr.PermissionDenied())
			}
			ctx = WithTenantID(ctx, tenantID)
			return next(ctx, req)
		}
	}
}

// tenantIDFromRequest reads "tenant_id" via reflection from any proto message
// or falls back to X-Tenant-Id header. Cheap one-time reflection per request.
func tenantIDFromRequest(req connect.AnyRequest) string {
	if v := req.Header().Get("X-Tenant-Id"); v != "" {
		return v
	}
	msg, ok := req.Any().(proto.Message)
	if !ok {
		return ""
	}
	rv := reflect.ValueOf(msg).Elem()
	field := rv.FieldByName("TenantId")
	if !field.IsValid() || field.IsNil() {
		return ""
	}
	// Field is *iampb.TenantId
	valField := field.Elem().FieldByName("Value")
	if !valField.IsValid() {
		return ""
	}
	return valField.String()
}
```

- [ ] **Step 3: Build**

```bash
go build ./internal/transport/middleware/...
```
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/transport/middleware/{auth,tenant}.go
git commit -m "feat(middleware): JWT/API-token auth + tenant resolver"
```

---

## Task 21: Validate + Idempotency middleware

**Files:**
- Create: `internal/transport/middleware/validate.go`, `idempotency.go`, `idempotency_registry.go`

- [ ] **Step 1: `validate.go`**

```go
package middleware

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
)

type Validator interface {
	Validate() error
}

func ProtoValidate() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if msg, ok := req.Any().(proto.Message); ok {
				if v, ok := msg.(Validator); ok {
					if err := v.Validate(); err != nil {
						return nil, connect.NewError(connect.CodeInvalidArgument, err)
					}
				}
			}
			return next(ctx, req)
		}
	}
}
```

(Generated `*.pb.validate.go` from protoc-gen-validate adds `Validate()` to each
message — this interceptor invokes it.)

- [ ] **Step 2: `idempotency_registry.go`**

```go
package middleware

import (
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// BuildIdempotencyRegistry scans all registered proto files and returns a set
// of fully-qualified procedure paths whose method has idempotency_level=IDEMPOTENT.
func BuildIdempotencyRegistry() map[string]bool {
	reg := map[string]bool{}
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			svc := services.Get(i)
			methods := svc.Methods()
			for j := 0; j < methods.Len(); j++ {
				m := methods.Get(j)
				opts, ok := m.Options().(*descriptorpbMethodOptions)
				if !ok {
					continue
				}
				if opts.GetIdempotencyLevel() == descriptorpbIdempotent {
					proc := "/" + string(svc.FullName()) + "/" + string(m.Name())
					reg[proc] = true
				}
			}
		}
		return true
	})
	return reg
}
```

Replace the two `descriptorpb*` placeholder names with real imports:

```go
import (
	descpb "google.golang.org/protobuf/types/descriptorpb"
)

// in code:
opts, ok := m.Options().(*descpb.MethodOptions)
if !ok { continue }
if opts.GetIdempotencyLevel() == descpb.MethodOptions_IDEMPOTENT {
```

Final `idempotency_registry.go`:

```go
package middleware

import (
	descpb "google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func BuildIdempotencyRegistry() map[string]bool {
	reg := map[string]bool{}
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			svc := services.Get(i)
			methods := svc.Methods()
			for j := 0; j < methods.Len(); j++ {
				m := methods.Get(j)
				opts, ok := m.Options().(*descpb.MethodOptions)
				if !ok {
					continue
				}
				if opts.GetIdempotencyLevel() == descpb.MethodOptions_IDEMPOTENT {
					proc := "/" + string(svc.FullName()) + "/" + string(m.Name())
					reg[proc] = true
				}
			}
		}
		return true
	})
	return reg
}
```

- [ ] **Step 3: `idempotency.go`**

```go
package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/valkey"
)

// IdempotencyConfig knobs.
type IdempotencyConfig struct {
	Enabled bool
	TTL     time.Duration
}

// CachedResp is what we store.
type cachedResp struct {
	Code    int32
	Message string
	Body    []byte
}

const (
	headerKey = "X-Idempotency-Key"
	prefix    = "idemp:"
)

func Idempotency(store *valkey.Client, registry map[string]bool, cfg IdempotencyConfig) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if !cfg.Enabled || !registry[req.Spec().Procedure] {
				return next(ctx, req)
			}
			key := req.Header().Get(headerKey)
			if key == "" {
				return next(ctx, req)
			}
			tenantID := TenantFromCtx(ctx)
			if tenantID == "" {
				return next(ctx, req)
			}
			storeKey := prefix + tenantID + ":" + req.Spec().Procedure + ":" + key

			acquired, err := store.SetNX(ctx, storeKey, "pending", cfg.TTL)
			if err != nil || !acquired {
				// Look up existing state
				raw, err := store.Get(ctx, storeKey)
				if err == nil && raw != "" && raw != "pending" {
					var c cachedResp
					if err := json.Unmarshal([]byte(raw), &c); err == nil {
						return replay(req, c)
					}
				}
				return nil, connect.NewError(connect.CodeAborted, errors.New("duplicate request in flight"))
			}

			resp, herr := next(ctx, req)
			cache(ctx, store, storeKey, cfg.TTL, resp, herr)
			return resp, herr
		}
	}
}

func cache(ctx context.Context, store *valkey.Client, key string, ttl time.Duration, resp connect.AnyResponse, herr error) {
	c := cachedResp{}
	if herr != nil {
		var ce *connect.Error
		if errors.As(herr, &ce) {
			c.Code = int32(ce.Code())
			c.Message = ce.Message()
		}
		// Don't cache transient codes
		switch connect.Code(c.Code) {
		case connect.CodeUnavailable, connect.CodeDeadlineExceeded, connect.CodeResourceExhausted:
			_ = store.Del(ctx, key)
			return
		}
	}
	if resp != nil {
		if m, ok := resp.Any().(proto.Message); ok {
			body, _ := proto.Marshal(m)
			c.Body = body
		}
	}
	data, _ := json.Marshal(c)
	_ = store.Set(ctx, key, string(data), ttl)
}

func replay(_ connect.AnyRequest, c cachedResp) (connect.AnyResponse, error) {
	if c.Code != 0 {
		return nil, connect.NewError(connect.Code(c.Code), errors.New(c.Message))
	}
	// Returning a raw bytes envelope is non-trivial without the response type.
	// For Phase 02 we accept that cached responses cannot replay the body —
	// idempotent ops re-run on cache miss; the value of the cache is the
	// duplicate-in-flight protection. Refine post Phase 02 when the response
	// type can be resolved from the procedure registry.
	return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("idempotent replay placeholder"))
}
```

Note: `internal/infrastructure/valkey/` already exists; ensure it exposes `Set`, `SetNX`, `Get`, `Del`. If it doesn't, add these methods (one-line wrappers over the existing client).

- [ ] **Step 4: Build**

```bash
go build ./internal/transport/middleware/...
```
Expected: success.

- [ ] **Step 5: Commit**

```bash
git add internal/transport/middleware/{validate,idempotency,idempotency_registry}.go
git commit -m "feat(middleware): protovalidate + idempotency interceptor + registry"
```

---

## Task 22: Connect handler for IAM

**Files:**
- Create: `internal/transport/connect/iam.go`, `server.go`

- [ ] **Step 1: `iam.go` handler**

```go
package connectrpc

import (
	"context"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

type IAMHandler struct{ svc *iam.Service }

func NewIAMHandler(svc *iam.Service) *IAMHandler { return &IAMHandler{svc: svc} }

// AuthService
func (h *IAMHandler) Login(ctx context.Context, req *connect.Request[iampb.LoginRequest]) (*connect.Response[iampb.LoginResponse], error) {
	pair, err := h.svc.Login(ctx, req.Msg.GetEmail(), req.Msg.GetPassword())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&iampb.LoginResponse{Tokens: pair}), nil
}

func (h *IAMHandler) RefreshTokens(ctx context.Context, req *connect.Request[iampb.RefreshTokenRequest]) (*connect.Response[iampb.RefreshTokenResponse], error) {
	pair, err := h.svc.RefreshTokens(ctx, req.Msg.GetRefreshToken())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&iampb.RefreshTokenResponse{Tokens: pair}), nil
}

func (h *IAMHandler) Logout(ctx context.Context, req *connect.Request[iampb.LogoutRequest]) (*connect.Response[emptypb.Empty], error) {
	if err := h.svc.Logout(ctx, req.Msg.GetRefreshToken()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

// UserService
func (h *IAMHandler) CreateUser(ctx context.Context, req *connect.Request[iampb.CreateUserRequest]) (*connect.Response[iampb.User], error) {
	u, err := h.svc.CreateUser(ctx, req.Msg.GetUser(), req.Msg.GetPassword())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(u), nil
}

func (h *IAMHandler) GetUser(ctx context.Context, req *connect.Request[iampb.UserId]) (*connect.Response[iampb.User], error) {
	u, err := h.svc.GetUserByID(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(u), nil
}

func (h *IAMHandler) ListUsers(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[iampb.User_List], error) {
	users, err := h.svc.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&iampb.User_List{Users: users}), nil
}

// TenantService
func (h *IAMHandler) CreateTenant(ctx context.Context, req *connect.Request[iampb.CreateTenantRequest]) (*connect.Response[iampb.Tenant], error) {
	userID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	t, err := h.svc.CreateTenant(ctx, req.Msg.GetTenant(), userID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(t), nil
}

func (h *IAMHandler) ListTenants(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[iampb.Tenant_List], error) {
	userID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	tenants, err := h.svc.ListTenantsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&iampb.Tenant_List{Tenants: tenants}), nil
}
```

Add import: `"google.golang.org/protobuf/types/known/emptypb"`.

Note: Real generated `iampb.User_List` / `iampb.Tenant_List` types come from
`message List { repeated X xs = 1 }` declared in each proto. If actual type
names differ after generation, adjust accordingly.

- [ ] **Step 2: `server.go` (router wiring)**

```go
package connectrpc

import (
	"net/http"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam/iamconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
)

// Mount returns a chi/mux-compatible http.Handler aggregating all Connect handlers.
type Deps struct {
	IAMHandler *IAMHandler

	Interceptors connect.Option
}

func Mount(d Deps) http.Handler {
	mux := http.NewServeMux()

	// Auth service
	authPath, authHandler := iamconnect.NewAuthServiceHandler(d.IAMHandler, d.Interceptors)
	mux.Handle(authPath, authHandler)

	// User service
	userPath, userHandler := iamconnect.NewUserServiceHandler(d.IAMHandler, d.Interceptors)
	mux.Handle(userPath, userHandler)

	// Tenant service
	tenantPath, tenantHandler := iamconnect.NewTenantServiceHandler(d.IAMHandler, d.Interceptors)
	mux.Handle(tenantPath, tenantHandler)

	return mux
}

// AuthBypass returns the set of fully-qualified procedure paths that skip
// JWT enforcement.
func AuthBypass() map[string]bool {
	return map[string]bool{
		iamconnect.AuthServiceLoginProcedure:         true,
		iamconnect.AuthServiceRefreshTokensProcedure: true,
		iamconnect.AuthServiceLogoutProcedure:        true,
	}
}

// TenantBypass returns procedures that skip tenant resolution.
func TenantBypass() map[string]bool {
	out := AuthBypass()
	out[iamconnect.UserServiceCreateUserProcedure] = true
	out[iamconnect.UserServiceGetUserProcedure] = true
	out[iamconnect.UserServiceListUsersProcedure] = true
	out[iamconnect.TenantServiceCreateTenantProcedure] = true
	out[iamconnect.TenantServiceListTenantsProcedure] = true
	return out
}

func WithMiddleware(i ...connect.Interceptor) connect.HandlerOption {
	return connect.WithInterceptors(i...)
}
```

- [ ] **Step 3: Build**

```bash
go build ./internal/transport/...
```
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/transport/connect/
git commit -m "feat(connect): IAM handler + Mount + auth/tenant bypass sets"
```

---

## Task 23: cmd/stroppy-cloud root + server subcommand

**Files:**
- Create: `cmd/stroppy-cloud/main.go`, `cmd_server.go`, `cmd_agent.go` (stub)

- [ ] **Step 1: `main.go`**

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/stroppy-io/stroppy-cloud/cmd/stroppy-cloud/cmd_cli"
)

func main() {
	root := &cobra.Command{
		Use:   "stroppy-cloud",
		Short: "Stroppy Cloud control plane",
	}
	root.AddCommand(serverCmd())
	root.AddCommand(agentCmd())
	root.AddCommand(cmd_cli.Root())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: `cmd_server.go`**

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"connectrpc.com/connect"
	otelconnect "connectrpc.com/otelconnect"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/stroppy-io/stroppy-cloud/internal/core/configurator"
	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/logger"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/migrations"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/valkey"
	connectrpc "github.com/stroppy-io/stroppy-cloud/internal/transport/connect"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
)

func serverCmd() *cobra.Command {
	var cfgPath string
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run the control-plane server",
		RunE: func(c *cobra.Command, _ []string) error {
			return runServer(c.Context(), cfgPath)
		},
	}
	cmd.Flags().StringVar(&cfgPath, "config", os.Getenv("CONFIG_PATH"), "path to config.yaml")
	return cmd
}

func runServer(ctx context.Context, cfgPath string) error {
	if cfgPath == "" {
		return fmt.Errorf("--config or CONFIG_PATH required")
	}
	cfg, err := configurator.Load(cfgPath)
	if err != nil {
		return err
	}

	zlog, err := logger.New(cfg.Log.Level, cfg.Log.Format)
	if err != nil {
		return err
	}
	logger.SetGlobal(zlog)
	defer zlog.Sync()

	pool, err := postgres.New(cfg.Postgres)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := postgres.MigrateWithLock(pool, "stroppy-cloud", migrations.Content); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	exec, txMgr, err := pgtx.NewTxFlow(pool, pgtx.ReadCommittedSettings())
	if err != nil {
		return err
	}

	valkeyCli := valkey.New(cfg.Valkey)
	defer valkeyCli.Close()

	bus := eventing.NewInMemoryBus()
	defer bus.Close()

	jwtSecret := []byte(os.Getenv(cfg.Auth.JWTSecretEnv))
	if len(jwtSecret) < 32 {
		return fmt.Errorf("JWT secret env %s must hold ≥32 bytes", cfg.Auth.JWTSecretEnv)
	}

	iamSvc := iam.New(exec, txMgr, bus, cfg.Auth, jwtSecret)

	// Bootstrap initial admin (idempotent)
	if cfg.Features.InitialAdminEmail != "" {
		pwd := os.Getenv(cfg.Features.InitialAdminPasswordEnv)
		if pwd != "" {
			if err := bootstrapAdmin(ctx, iamSvc, cfg.Features.InitialAdminEmail, pwd, zlog); err != nil {
				return fmt.Errorf("bootstrap admin: %w", err)
			}
		}
	}

	otelInterceptor, err := otelconnect.NewInterceptor()
	if err != nil {
		return err
	}

	interceptors := connect.WithInterceptors(
		middleware.Recovery(zlog),
		middleware.RequestID(),
		otelInterceptor,
		middleware.Logging(zlog),
		middleware.Auth(iamSvc, connectrpc.AuthBypass()),
		middleware.Tenant(iamSvc, connectrpc.TenantBypass()),
		middleware.ProtoValidate(),
		middleware.Idempotency(valkeyCli, middleware.BuildIdempotencyRegistry(), middleware.IdempotencyConfig{
			Enabled: cfg.Idempotency.Enabled,
			TTL:     cfg.Idempotency.TTL,
		}),
		middleware.ErrorMapper(),
	)

	mux := http.NewServeMux()
	mux.Handle("/", connectrpc.Mount(connectrpc.Deps{IAMHandler: connectrpc.NewIAMHandler(iamSvc), Interceptors: interceptors}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	srv := &http.Server{
		Addr:    cfg.Server.HTTPAddr,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	zlog.Info("serving", zap.String("addr", cfg.Server.HTTPAddr))

	stopCtx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	go func() {
		<-stopCtx.Done()
		zlog.Info("shutdown signal received")
		shutdownCtx, c := context.WithTimeout(context.Background(), 30_000_000_000)
		defer c()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func bootstrapAdmin(ctx context.Context, iamSvc *iam.Service, email, password string, log *zap.Logger) error {
	existing, err := iamSvc.ListUsers(ctx)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}
	user, err := iamSvc.CreateUser(ctx, &iampbUserShim(email), password)
	if err != nil {
		return err
	}
	log.Info("bootstrapped initial admin", zap.String("user_id", user.GetId().GetValue()))
	return nil
}
```

Add helper at file bottom:

```go
func iampbUserShim(email string) iampb.User {
	return iampb.User{Email: email, Nickname: "admin"}
}
```

and import `iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"`.

Replace `30_000_000_000` with `30 * time.Second` and `import "time"` at top.

- [ ] **Step 3: `cmd_agent.go` stub**

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func agentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "agent",
		Short: "Run the remote-host agent (placeholder — implemented in plan 05)",
		RunE: func(c *cobra.Command, _ []string) error {
			return fmt.Errorf("agent mode not implemented yet — see plan 05")
		},
	}
}
```

- [ ] **Step 4: Build**

```bash
go build ./cmd/stroppy-cloud/...
```
Expected: success.

- [ ] **Step 5: Commit**

```bash
git add cmd/stroppy-cloud/
git commit -m "feat(cmd): root + server subcommand wiring IAM into ConnectRPC mux"
```

---

## Task 24: CLI client SDK + login/logout/whoami

**Files:**
- Create: `internal/sdk/client/client.go`, `credentials.go`
- Create: `cmd/stroppy-cloud/cmd_cli/root.go`, `login.go`, `logout.go`, `whoami.go`, `context.go`

- [ ] **Step 1: `client.go`**

```go
package client

import (
	"net/http"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam/iamconnect"
)

// Client aggregates typed Connect stubs for every public service.
type Client struct {
	server string

	httpClient *http.Client
	headers    http.Header

	Auth   iamconnect.AuthServiceClient
	User   iamconnect.UserServiceClient
	Tenant iamconnect.TenantServiceClient
}

// Option configures Client.
type Option func(c *Client)

// WithBearer sets the access-token header on every request.
func WithBearer(token string) Option {
	return func(c *Client) {
		if c.headers == nil {
			c.headers = http.Header{}
		}
		c.headers.Set("Authorization", "Bearer "+token)
	}
}

// New builds a Client targeting serverURL.
func New(serverURL string, opts ...Option) *Client {
	c := &Client{server: serverURL, httpClient: http.DefaultClient, headers: http.Header{}}
	for _, opt := range opts {
		opt(c)
	}
	connOpts := []connect.ClientOption{connect.WithInterceptors(headerInterceptor(c.headers))}
	c.Auth = iamconnect.NewAuthServiceClient(c.httpClient, serverURL, connOpts...)
	c.User = iamconnect.NewUserServiceClient(c.httpClient, serverURL, connOpts...)
	c.Tenant = iamconnect.NewTenantServiceClient(c.httpClient, serverURL, connOpts...)
	return c
}

func headerInterceptor(h http.Header) connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx connect.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			for k, vs := range h {
				for _, v := range vs {
					req.Header().Add(k, v)
				}
			}
			return next(ctx, req)
		}
	})
}
```

If `connect.Context` doesn't exist (it's `context.Context`), adjust: replace `connect.Context` with `context.Context`, add `import "context"`.

- [ ] **Step 2: `credentials.go`**

```go
package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Credentials struct {
	Server       string    `json:"server"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type Context struct {
	CurrentServer string `json:"current_server"`
	CurrentTenant string `json:"current_tenant_id"`
}

func configDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	out := filepath.Join(dir, "stroppy-cloud")
	if err := os.MkdirAll(out, 0o700); err != nil {
		return "", err
	}
	return out, nil
}

func LoadCredentials(server string) (*Credentials, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "credentials.json"))
	if err != nil {
		return nil, err
	}
	var all map[string]*Credentials
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, err
	}
	return all[server], nil
}

func SaveCredentials(c *Credentials) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "credentials.json")

	all := map[string]*Credentials{}
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &all)
	}
	all[c.Server] = c
	out, _ := json.MarshalIndent(all, "", "  ")
	return os.WriteFile(path, out, 0o600)
}

func LoadContext() (*Context, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "context.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return &Context{}, nil
		}
		return nil, err
	}
	c := &Context{}
	return c, json.Unmarshal(raw, c)
}

func SaveContext(c *Context) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	out, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(filepath.Join(dir, "context.json"), out, 0o600)
}
```

- [ ] **Step 3: `cmd_cli/root.go`**

```go
package cmd_cli

import "github.com/spf13/cobra"

// Root returns the parent command for all client subcommands.
func Root() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cli",
		Short: "Client subcommands (login, run, suite, etc.)",
	}
	cmd.AddCommand(loginCmd())
	cmd.AddCommand(logoutCmd())
	cmd.AddCommand(whoamiCmd())
	cmd.AddCommand(contextCmd())
	return cmd
}
```

- [ ] **Step 4: `cmd_cli/login.go`**

```go
package cmd_cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/stroppy-io/stroppy-cloud/internal/sdk/client"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func loginCmd() *cobra.Command {
	var server string
	cmd := &cobra.Command{
		Use:   "login --server <url>",
		Short: "Authenticate and persist tokens",
		RunE: func(c *cobra.Command, _ []string) error {
			email, pwd, err := readCreds()
			if err != nil {
				return err
			}
			cli := client.New(server)
			resp, err := cli.Auth.Login(context.Background(), connect.NewRequest(&iampb.LoginRequest{Email: email, Password: pwd}))
			if err != nil {
				return err
			}
			pair := resp.Msg.GetTokens()
			cred := &client.Credentials{
				Server:       server,
				AccessToken:  pair.GetAccessToken(),
				RefreshToken: pair.GetRefreshToken(),
				ExpiresAt:    time.Now().Add(pair.GetAccessTokenExpiresIn().AsDuration()),
			}
			if err := client.SaveCredentials(cred); err != nil {
				return err
			}
			ctxObj, _ := client.LoadContext()
			ctxObj.CurrentServer = server
			_ = client.SaveContext(ctxObj)
			fmt.Println("ok")
			return nil
		},
	}
	cmd.Flags().StringVar(&server, "server", "http://localhost:8080", "server URL")
	return cmd
}

func readCreds() (string, string, error) {
	r := bufio.NewReader(os.Stdin)
	fmt.Print("email: ")
	email, err := r.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	fmt.Print("password: ")
	pwdBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", "", err
	}
	email = trim(email)
	return email, string(pwdBytes), nil
}

func trim(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}
```

- [ ] **Step 5: `cmd_cli/logout.go`, `whoami.go`, `context.go`**

`logout.go`:

```go
package cmd_cli

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	"github.com/stroppy-io/stroppy-cloud/internal/sdk/client"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func logoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Revoke current refresh token",
		RunE: func(c *cobra.Command, _ []string) error {
			ctxObj, _ := client.LoadContext()
			if ctxObj == nil || ctxObj.CurrentServer == "" {
				return fmt.Errorf("no active session")
			}
			cred, err := client.LoadCredentials(ctxObj.CurrentServer)
			if err != nil || cred == nil {
				return fmt.Errorf("no saved credentials")
			}
			cli := client.New(ctxObj.CurrentServer, client.WithBearer(cred.AccessToken))
			_, err = cli.Auth.Logout(context.Background(), connect.NewRequest(&iampb.LogoutRequest{RefreshToken: cred.RefreshToken}))
			if err != nil {
				return err
			}
			cred.AccessToken = ""
			cred.RefreshToken = ""
			_ = client.SaveCredentials(cred)
			fmt.Println("ok")
			return nil
		},
	}
}
```

`whoami.go`:

```go
package cmd_cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stroppy-io/stroppy-cloud/internal/sdk/client"
)

func whoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Print current user + tenants",
		RunE: func(c *cobra.Command, _ []string) error {
			ctxObj, _ := client.LoadContext()
			cred, _ := client.LoadCredentials(ctxObj.CurrentServer)
			if cred == nil {
				return fmt.Errorf("not logged in")
			}
			cli := client.New(ctxObj.CurrentServer, client.WithBearer(cred.AccessToken))
			tenants, err := cli.Tenant.ListTenants(context.Background(), connect.NewRequest(&emptypb.Empty{}))
			if err != nil {
				return err
			}
			out := map[string]any{
				"server":  ctxObj.CurrentServer,
				"tenants": tenants.Msg.GetTenants(),
				"current_tenant": ctxObj.CurrentTenant,
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}
}
```

`context.go`:

```go
package cmd_cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/stroppy-io/stroppy-cloud/internal/sdk/client"
)

func contextCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "context", Short: "Manage local context"}
	use := &cobra.Command{
		Use:   "use --tenant <id>",
		Short: "Switch active tenant",
		RunE: func(c *cobra.Command, _ []string) error {
			tenant, _ := c.Flags().GetString("tenant")
			if tenant == "" {
				return fmt.Errorf("--tenant required")
			}
			ctxObj, _ := client.LoadContext()
			ctxObj.CurrentTenant = tenant
			return client.SaveContext(ctxObj)
		},
	}
	use.Flags().String("tenant", "", "tenant id")
	cmd.AddCommand(use)
	return cmd
}
```

- [ ] **Step 6: Add `golang.org/x/term` dep**

```bash
go get golang.org/x/term
go mod tidy
```

- [ ] **Step 7: Build**

```bash
go build ./...
```
Expected: success.

- [ ] **Step 8: Commit**

```bash
git add internal/sdk/client/ cmd/stroppy-cloud/cmd_cli/ go.mod go.sum
git commit -m "feat(cli,sdk): login/logout/whoami/context subcommands + typed client wrapper"
```

---

## Task 25: End-to-end smoke (server + cli)

**Files:**
- Create: `tests/e2e/iam_smoke_test.go`

- [ ] **Step 1: Write e2e test**

```go
//go:build e2e

package e2e_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/sdk/client"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestE2E_LoginCreateTenant(t *testing.T) {
	f := fixture.NewIAM(t)
	ctx := context.Background()

	// Bootstrap admin directly
	admin, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "root@e.com", Nickname: "root"}, "RootP@ss!")
	require.NoError(t, err)

	// Stand up an HTTP test server with handler.
	handler := newTestServer(f)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	cli := client.New(srv.URL)
	resp, err := cli.Auth.Login(ctx, connect.NewRequest(&iampb.LoginRequest{Email: "root@e.com", Password: "RootP@ss!"}))
	require.NoError(t, err)
	access := resp.Msg.GetTokens().GetAccessToken()
	require.NotEmpty(t, access)

	authed := client.New(srv.URL, client.WithBearer(access))
	tenantResp, err := authed.Tenant.CreateTenant(ctx, connect.NewRequest(&iampb.CreateTenantRequest{Tenant: &iampb.Tenant{Name: "Acme"}}))
	require.NoError(t, err)
	require.NotEmpty(t, tenantResp.Msg.GetId().GetValue())

	_ = admin
}

func newTestServer(f *fixture.IAMFixture) http.Handler {
	// Build connect mux against f.IAM
	// (mirrors cmd_server.go wiring; reuse a helper in real impl)
	// Implementation omitted here — bring in connectrpc.Mount with no middleware
	// to focus on protocol shake-down.
	panic("wire up Mount(Deps{IAMHandler: NewIAMHandler(f.IAM)}) — implement during task")
}
```

Replace `newTestServer` body with the real wire-up. Extract a helper to
`internal/testutil/testserver/` if needed:

Create `internal/testutil/testserver/testserver.go`:

```go
package testserver

import (
	"net/http"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/iam"
	connectrpc "github.com/stroppy-io/stroppy-cloud/internal/transport/connect"
)

func ForIAM(svc *iam.Service) http.Handler {
	return connectrpc.Mount(connectrpc.Deps{IAMHandler: connectrpc.NewIAMHandler(svc)})
}
```

…and update the test to use `testserver.ForIAM(f.IAM)`.

- [ ] **Step 2: Run e2e**

```bash
go test -tags=e2e -run TestE2E_LoginCreateTenant ./tests/e2e/...
```
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add tests/e2e/ internal/testutil/testserver/
git commit -m "test(e2e): login → create tenant smoke against in-process server"
```

---

## Task 26: Makefile target for server-run

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Append target**

Append to `Makefile`:

```makefile
.PHONY: server-run
server-run: ## Run the new server locally (requires CONFIG_PATH + JWT_SECRET)
	go run ./cmd/stroppy-cloud server --config $${CONFIG_PATH:-./deployments/local/server/config.yaml}
```

- [ ] **Step 2: Create example config**

Create `deployments/local/server/config.yaml`:

```yaml
server:
  http_addr: ":8080"
postgres:
  dsn: "postgres://stroppy:stroppy@127.0.0.1:5436/stroppy?sslmode=disable"
  max_conns: 25
valkey:
  addr: "127.0.0.1:6379"
  db: 0
auth:
  jwt_secret_env: "JWT_SECRET"
  access_ttl: "15m"
  refresh_ttl: "720h"
idempotency:
  enabled: true
  ttl: "24h"
workers:
  node_workers: 4
  scheduler_tick: "5s"
  webhook_workers: 2
  recovery_on_start: true
features:
  initial_admin_email: "admin@local"
  initial_admin_password_env: "INITIAL_ADMIN_PASSWORD"
log:
  level: "info"
  format: "json"
```

- [ ] **Step 3: Commit**

```bash
git add Makefile deployments/local/server/config.yaml
git commit -m "chore: add make server-run + example local server config"
```

---

## Task 27: Final phase verification

- [ ] **Step 1: Full build**

```bash
go build ./...
```
Expected: success.

- [ ] **Step 2: Full test sweep (integration only — no e2e)**

```bash
go test ./internal/... -count=1 -p 4
```
Expected: all green.

- [ ] **Step 3: E2E**

```bash
go test -tags=e2e -count=1 ./tests/e2e/...
```
Expected: green.

- [ ] **Step 4: Manual smoke**

Run server in one terminal:
```bash
export JWT_SECRET=$(head -c 32 /dev/urandom | base64)
export INITIAL_ADMIN_PASSWORD="AdminP@ss123!"
docker compose up -d postgres valkey
make server-run
```

In another terminal:
```bash
go run ./cmd/stroppy-cloud cli login --server http://localhost:8080
# enter admin@local + the password above
go run ./cmd/stroppy-cloud cli whoami
```
Expected: tenants list (initially empty).

- [ ] **Step 5: Commit final state**

```bash
git status
# (nothing to commit if previous tasks closed cleanly)
```

If lingering scratch files exist, remove or stage them.

---

## Acceptance for plan 01

- ✅ Legacy `internal/domain/api/`, `internal/domain/run/`, `internal/core/dag/`, `internal/domain/scheduler/`, `internal/storage/`, `internal/infrastructure/postgres/generated/` removed.
- ✅ `cmd/stroppy-cloud` binary builds; `stroppy-cloud server` boots; `stroppy-cloud cli login` works.
- ✅ Migrations apply under advisory-lock on startup.
- ✅ Initial admin bootstraps idempotently.
- ✅ `make server-run` runs against local postgres+valkey.
- ✅ Integration tests for users / tenants / members / auth / api_tokens green.
- ✅ E2E smoke `login → create tenant` green.
- ✅ All middleware in place (recovery, requestID, logging, otel, auth, tenant, validate, idempotency, errmap).
- ✅ Errors flow through `*domainerr.Error` → connect.Error with `cloud.v1.errors.Code` only.

## Out of scope for plan 01 (next plan)

- Catalog / Stroppy / Settings services (plan 02).
- DAG engine + workers (plan 03).
- Testing services (plan 04).
- Agent (plan 05).
- Ops / webhook outbox (plan 06).
- Admin RPCs (plan 07).
- Frontend integration (plan 08).
- Full e2e sweep + perf smoke (plan 09).
