# Plan 02: Catalog + Stroppy + Settings Implementation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the catalog tier (DatabasePreset, WorkloadPreset, Package, Settings) and the Stroppy helper service (binary version proxy, `stroppy probe` wrapper, `stroppy-config.json` renderer). After this plan, an authenticated user can: upload a package, define database/workload presets, save tenant settings (YC token/zones), pick a stroppy binary version, run a probe, and preview the final stroppy-config.

**Architecture:** Two new `services/` packages — `catalog/` and `stroppy/`. Catalog services share `pgtx` + `eventing` wiring established in plan 01 and use `repository.ProtoRepository` for every table. Stroppy service owns `internal/infrastructure/stroppybin/` (subprocess runner with valkey-backed JSON cache for `versions` proxy) and shells out to the chosen `stroppy` binary for `probe` and `preview` operations. Binary uploads land in S3 (existing `internal/infrastructure/s3/`) and are referenced by `Package` rows via `BinaryDownload.CachedRef`.

**Tech Stack:** Existing plan-01 stack + AWS SDK v2 (`s3`) presigned URLs, `os/exec` for stroppy subprocess, `valkey-go` Set/Get for version-cache TTL.

---

## Files at end of plan

### Created

- `internal/domain/services/catalog/service.go`
- `internal/domain/services/catalog/database_presets.go`
- `internal/domain/services/catalog/workload_presets.go`
- `internal/domain/services/catalog/packages.go`
- `internal/domain/services/catalog/settings.go`
- `internal/domain/services/catalog/integration_test.go`
- `internal/domain/services/stroppy/service.go`
- `internal/domain/services/stroppy/versions.go`
- `internal/domain/services/stroppy/probe.go`
- `internal/domain/services/stroppy/preview.go`
- `internal/domain/services/stroppy/integration_test.go`
- `internal/infrastructure/stroppybin/binary.go`
- `internal/infrastructure/stroppybin/probe.go`
- `internal/infrastructure/stroppybin/preview.go`
- `internal/infrastructure/s3/client.go` (refresh/adapt — replace any legacy interface)
- `internal/infrastructure/valkey/cache.go` (helper for typed JSON cache)
- `internal/transport/connect/catalog.go`
- `internal/transport/connect/stroppy.go`
- `internal/sdk/client/catalog.go`
- `internal/sdk/client/stroppy.go`
- `cmd/stroppy-cloud/cmd_cli/preset.go`
- `cmd/stroppy-cloud/cmd_cli/package.go`
- `cmd/stroppy-cloud/cmd_cli/settings.go`
- `cmd/stroppy-cloud/cmd_cli/probe.go`
- `cmd/stroppy-cloud/cmd_cli/version.go`

### Modified

- `internal/sdk/client/client.go` (add Catalog* + Stroppy clients)
- `internal/transport/connect/server.go` (mount catalog + stroppy handlers)
- `cmd/stroppy-cloud/cmd_server.go` (wire catalog + stroppy services)
- `cmd/stroppy-cloud/cmd_cli/root.go` (register new subcommands)
- `internal/testutil/fixture/iam.go` → split helpers; add `internal/testutil/fixture/catalog.go`, `stroppy.go`
- `Makefile` (add `make stroppy-bin-fetch` for tests)
- `deployments/local/server/config.yaml` (s3, stroppy sections)

---

## Task 1: Stroppy subprocess runner foundation

**Files:**
- Create: `internal/infrastructure/stroppybin/binary.go`, `probe.go`, `preview.go`

- [ ] **Step 1: `binary.go`**

```go
package stroppybin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Runner resolves and invokes the `stroppy` binary on the local host.
type Runner struct {
	defaultVersion string
	binariesDir    string

	mu       sync.Mutex
	resolved map[string]string // version → absolute binary path
}

// New builds a Runner.
func New(defaultVersion, binariesDir string) *Runner {
	return &Runner{
		defaultVersion: defaultVersion,
		binariesDir:    binariesDir,
		resolved:       make(map[string]string),
	}
}

// Resolve returns the absolute path to the binary for the requested version.
// Empty version → default. Lookup order:
//  1. <binariesDir>/stroppy-<version>
//  2. <binariesDir>/<version>/stroppy
//  3. error
func (r *Runner) Resolve(version string) (string, error) {
	if version == "" {
		version = r.defaultVersion
	}
	if override := strings.TrimSpace(os.Getenv("STROPPY_PROBE_BIN")); override != "" {
		return override, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if cached, ok := r.resolved[version]; ok {
		return cached, nil
	}
	candidates := []string{
		filepath.Join(r.binariesDir, "stroppy-"+version),
		filepath.Join(r.binariesDir, version, "stroppy"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			r.resolved[version] = c
			return c, nil
		}
	}
	return "", fmt.Errorf("stroppybin: binary not found for version %q (looked in %v)", version, candidates)
}

// ExecOptions configures a subprocess invocation.
type ExecOptions struct {
	Args  []string
	Stdin []byte
	Dir   string
	Env   []string
}

// Exec runs the binary with the given options, returns stdout, stderr, error.
func (r *Runner) Exec(ctx context.Context, version string, opts ExecOptions) ([]byte, []byte, error) {
	bin, err := r.Resolve(version)
	if err != nil {
		return nil, nil, err
	}
	cmd := exec.CommandContext(ctx, bin, opts.Args...)
	cmd.Dir = opts.Dir
	cmd.Env = append(os.Environ(), opts.Env...)
	if len(opts.Stdin) > 0 {
		cmd.Stdin = strings.NewReader(string(opts.Stdin))
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	return []byte(stdout.String()), []byte(stderr.String()), err
}
```

- [ ] **Step 2: `probe.go`**

```go
package stroppybin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ProbeInput is the wire-level configuration handed to `stroppy probe`.
type ProbeInput struct {
	Version      string
	Script       string
	SQL          string
	DriverType   string
	PoolSize     uint32
	ScaleFactor  uint32
	Env          map[string]string
	Files        []WorkloadFile
	IncludeHuman bool
}

// WorkloadFile is an auxiliary file (sql template, script) written next to
// the temporary stroppy-config.json.
type WorkloadFile struct {
	Name    string
	Content string
}

// ProbeResult is the parsed output of `stroppy probe -o json`.
type ProbeResult struct {
	StdoutJSON []byte
	Human      string
	HumanErr   string
}

// RunProbe builds a temp dir, writes config + workload files, executes
// `stroppy probe -f <cfg> -o json [-o human]`, returns parsed result.
func (r *Runner) RunProbe(ctx context.Context, in ProbeInput) (*ProbeResult, error) {
	tmp, err := os.MkdirTemp("", "stroppy-probe-*")
	if err != nil {
		return nil, fmt.Errorf("stroppybin: mkdir tmp: %w", err)
	}
	defer os.RemoveAll(tmp)

	cfg := buildRunConfig(in)
	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("stroppybin: marshal cfg: %w", err)
	}
	cfgPath := filepath.Join(tmp, "stroppy-config.json")
	if err := os.WriteFile(cfgPath, cfgBytes, 0o600); err != nil {
		return nil, fmt.Errorf("stroppybin: write cfg: %w", err)
	}
	for _, f := range in.Files {
		safe := filepath.Base(f.Name)
		if safe != f.Name || safe == "" || safe == "." || safe == ".." {
			return nil, fmt.Errorf("stroppybin: unsafe workload file name %q", f.Name)
		}
		if err := os.WriteFile(filepath.Join(tmp, safe), []byte(f.Content), 0o600); err != nil {
			return nil, fmt.Errorf("stroppybin: write workload file: %w", err)
		}
	}

	stdout, stderr, err := r.Exec(ctx, in.Version, ExecOptions{
		Args: []string{"probe", "-f", cfgPath, "-o", "json"},
		Dir:  tmp,
	})
	if err != nil {
		return nil, fmt.Errorf("stroppybin: probe failed: %w (stderr=%s)", err, string(stderr))
	}
	res := &ProbeResult{StdoutJSON: stdout}
	if in.IncludeHuman {
		hStdout, hStderr, hErr := r.Exec(ctx, in.Version, ExecOptions{
			Args: []string{"probe", "-f", cfgPath, "-o", "human"},
			Dir:  tmp,
		})
		if hErr != nil {
			res.HumanErr = string(hStderr)
		} else {
			res.Human = string(hStdout)
		}
	}
	return res, nil
}

// buildRunConfig assembles the minimal stroppy probe config the binary
// expects. Field names mirror stroppy's RunConfig proto-json shape.
func buildRunConfig(in ProbeInput) map[string]any {
	rc := map[string]any{
		"version": "1",
		"script":  in.Script,
	}
	if in.SQL != "" {
		rc["sql"] = in.SQL
	}
	if in.DriverType != "" {
		drv := map[string]any{"driver_type": in.DriverType, "url": defaultDriverURL(in.DriverType)}
		if in.PoolSize > 0 {
			drv["pool"] = map[string]any{"max_conns": in.PoolSize, "min_conns": in.PoolSize}
		}
		rc["drivers"] = map[string]any{"0": drv}
	}
	env := map[string]string{}
	for k, v := range in.Env {
		env[k] = v
	}
	if in.ScaleFactor > 0 {
		env["SCALE_FACTOR"] = fmt.Sprintf("%d", in.ScaleFactor)
	}
	if in.PoolSize > 0 {
		env["POOL_SIZE"] = fmt.Sprintf("%d", in.PoolSize)
	}
	if len(env) > 0 {
		rc["env"] = env
	}
	return rc
}

func defaultDriverURL(driver string) string {
	switch driver {
	case "postgres":
		return "postgres://stroppy:stroppy@127.0.0.1:5432/stroppy?sslmode=disable"
	case "mysql":
		return "mysql://stroppy:stroppy@127.0.0.1:3306/stroppy"
	case "picodata":
		return "picodata://admin:admin@127.0.0.1:3301/"
	default:
		return ""
	}
}
```

- [ ] **Step 3: `preview.go`**

```go
package stroppybin

import (
	"context"
	"encoding/json"
)

// PreviewInput is what the wizard provides.
type PreviewInput = ProbeInput

// RenderConfig builds the final stroppy-config.json the binary would consume,
// without spawning a probe. It mirrors buildRunConfig but returns pretty JSON
// for SPA display.
func RenderConfig(_ context.Context, in PreviewInput) ([]byte, error) {
	cfg := buildRunConfig(in)
	return json.MarshalIndent(cfg, "", "  ")
}
```

- [ ] **Step 4: Build**

```bash
go build ./internal/infrastructure/stroppybin/...
```
Expected: success.

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/stroppybin/
git commit -m "feat(stroppybin): subprocess runner + probe + preview renderer"
```

---

## Task 2: Valkey typed JSON cache helper

**Files:**
- Create: `internal/infrastructure/valkey/cache.go`

- [ ] **Step 1: Implement**

```go
package valkey

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// JSONCache is a thin typed wrapper for Get/Set with JSON marshaling + TTL.
type JSONCache[T any] struct {
	cli *Client
	ns  string
}

// NewJSONCache returns a typed cache scoped under namespace.
func NewJSONCache[T any](cli *Client, namespace string) *JSONCache[T] {
	return &JSONCache[T]{cli: cli, ns: namespace}
}

func (c *JSONCache[T]) key(k string) string { return c.ns + ":" + k }

// Get returns the cached value + true if hit.
func (c *JSONCache[T]) Get(ctx context.Context, k string) (T, bool, error) {
	var zero T
	raw, err := c.cli.Get(ctx, c.key(k))
	if err != nil {
		return zero, false, fmt.Errorf("valkey cache get: %w", err)
	}
	if raw == "" {
		return zero, false, nil
	}
	var v T
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return zero, false, fmt.Errorf("valkey cache decode: %w", err)
	}
	return v, true, nil
}

// Set stores v under k with TTL.
func (c *JSONCache[T]) Set(ctx context.Context, k string, v T, ttl time.Duration) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.cli.Set(ctx, c.key(k), string(raw), ttl)
}
```

- [ ] **Step 2: Build**

```bash
go build ./internal/infrastructure/valkey/...
```
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/infrastructure/valkey/cache.go
git commit -m "feat(valkey): typed JSON cache helper with TTL"
```

---

## Task 3: Catalog service skeleton + DatabasePreset CRUD

**Files:**
- Create: `internal/domain/services/catalog/service.go`, `database_presets.go`, `integration_test.go`
- Create: `internal/testutil/fixture/catalog.go`

- [ ] **Step 1: `service.go`**

```go
package catalog

import (
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"

	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
)

// Service owns the catalog tables: database_presets, workload_presets, packages, settings_items.
type Service struct {
	*tracing.Entity

	dbPresetRepo       *repository.ProtoRepository[catalogpb.DatabasePresetAlias, catalogpb.DatabasePresetColumnAlias, *catalogpb.DatabasePresetScanner, *catalogpb.DatabasePreset]
	workloadPresetRepo *repository.ProtoRepository[catalogpb.WorkloadPresetAlias, catalogpb.WorkloadPresetColumnAlias, *catalogpb.WorkloadPresetScanner, *catalogpb.WorkloadPreset]
	packageRepo        *repository.ProtoRepository[catalogpb.PackageAlias, catalogpb.PackageColumnAlias, *catalogpb.PackageScanner, *catalogpb.Package]
	settingsRepo       *repository.ProtoRepository[catalogpb.SettingsItemAlias, catalogpb.SettingsItemColumnAlias, *catalogpb.SettingsItemScanner, *catalogpb.SettingsItem]

	txMgr  pgtx.TxManager
	events eventing.Bus
}

// New wires the catalog service.
func New(executor exec.DB, txMgr pgtx.TxManager, events eventing.Bus) *Service {
	return &Service{
		Entity: tracing.NewEntity("catalog.Service"),
		dbPresetRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(catalogpb.DatabasePresets.Table, executor),
			catalogpb.DatabasePresetConverter,
		),
		workloadPresetRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(catalogpb.WorkloadPresets.Table, executor),
			catalogpb.WorkloadPresetConverter,
		),
		packageRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(catalogpb.Packages.Table, executor),
			catalogpb.PackageConverter,
		),
		settingsRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(catalogpb.SettingsItems.Table, executor),
			catalogpb.SettingsItemConverter,
		),
		txMgr:  txMgr,
		events: events,
	}
}
```

- [ ] **Step 2: `database_presets.go`**

```go
package catalog

import (
	"context"
	"errors"

	"github.com/yaroher/ratel/pkg/dml/set"
	"github.com/yaroher/ratel/pkg/repository"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func (s *Service) CreateDatabasePreset(ctx context.Context, tenantID *iampb.TenantId, createdBy *iampb.UserId, preset *catalogpb.DatabasePreset) (*catalogpb.DatabasePreset, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CreateDatabasePreset",
		func(ctx context.Context, _ any) (*catalogpb.DatabasePreset, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*catalogpb.DatabasePreset, error) {
					now := timestamppb.Now()
					preset.Id = &catalogpb.DatabasePresetId{Value: ids.New()}
					preset.TenantId = tenantID
					preset.CreatedBy = createdBy
					preset.Timestamps = &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now}
					if err := s.dbPresetRepo.Insert(ctx, preset); err != nil {
						return nil, err
					}
					return preset, nil
				})
		})
}

func (s *Service) GetDatabasePreset(ctx context.Context, id *catalogpb.DatabasePresetId) (*catalogpb.DatabasePreset, error) {
	p, err := s.dbPresetRepo.SelectOne(ctx, set.Eq(catalogpb.DatabasePresetColumnId, id.GetValue()))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("database_preset", id.GetValue()))
		}
		return nil, err
	}
	return p, nil
}

func (s *Service) ListDatabasePresets(ctx context.Context, tenantID *iampb.TenantId) ([]*catalogpb.DatabasePreset, error) {
	return s.dbPresetRepo.Select(ctx, set.Eq(catalogpb.DatabasePresetColumnTenantId, tenantID.GetValue()))
}

func (s *Service) DeleteDatabasePreset(ctx context.Context, id *catalogpb.DatabasePresetId) error {
	_, err := s.dbPresetRepo.Delete(ctx, set.Eq(catalogpb.DatabasePresetColumnId, id.GetValue()))
	if err != nil && errors.Is(err, repository.ErrNotFound) {
		return domainerr.NotFound(domainerr.ResourceInfo("database_preset", id.GetValue()))
	}
	return err
}

func (s *Service) CloneDatabasePreset(ctx context.Context, id *catalogpb.DatabasePresetId, callerID *iampb.UserId) (*catalogpb.DatabasePreset, error) {
	original, err := s.GetDatabasePreset(ctx, id)
	if err != nil {
		return nil, err
	}
	cloned := &catalogpb.DatabasePreset{
		TenantId: original.GetTenantId(),
		Identity: original.GetIdentity(),
		Database: original.GetDatabase(),
	}
	return s.CreateDatabasePreset(ctx, original.GetTenantId(), callerID, cloned)
}
```

- [ ] **Step 3: `fixture/catalog.go`**

```go
package fixture

import (
	"testing"

	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/catalog"
)

// CatalogFixture extends F with iam + catalog services.
type CatalogFixture struct {
	*IAMFixture
	Catalog *catalog.Service
}

// NewCatalog builds an IAM fixture and a catalog service.
func NewCatalog(t *testing.T) *CatalogFixture {
	t.Helper()
	iam := NewIAM(t)
	exec := iam.F.Executor.(*sqlexec.TxExecutor)
	cat := catalog.New(exec, iam.F.TxMgr, iam.F.Events)
	return &CatalogFixture{IAMFixture: iam, Catalog: cat}
}
```

- [ ] **Step 4: Write failing test**

Create `internal/domain/services/catalog/integration_test.go`:

```go
package catalog_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestCreateAndListDatabasePresets(t *testing.T) {
	f := fixture.NewCatalog(t)
	ctx := context.Background()

	u, _ := f.IAM.CreateUser(ctx, &iampb.User{Email: "c@e.com", Nickname: "c"}, "P@ss1234!")
	tn, _ := f.IAM.CreateTenant(ctx, &iampb.Tenant{Name: "T"}, u.GetId())

	preset := &catalogpb.DatabasePreset{
		Identity: &commonpb.Identity{Name: "pg-default"},
		Database: &catalogpb.Database{Kind: catalogpb.Database_DATABASE_KIND_POSTGRES},
	}
	created, err := f.Catalog.CreateDatabasePreset(ctx, tn.GetId(), u.GetId(), preset)
	require.NoError(t, err)
	require.NotEmpty(t, created.GetId().GetValue())

	list, err := f.Catalog.ListDatabasePresets(ctx, tn.GetId())
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "pg-default", list[0].GetIdentity().GetName())
}
```

- [ ] **Step 5: Run test**

```bash
go test ./internal/domain/services/catalog/... -v -run TestCreateAndListDatabasePresets
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/services/catalog/ internal/testutil/fixture/catalog.go
git commit -m "feat(catalog): service skeleton + DatabasePreset CRUD/clone"
```

---

## Task 4: WorkloadPreset CRUD

**Files:**
- Create: `internal/domain/services/catalog/workload_presets.go`

- [ ] **Step 1: Implement**

```go
package catalog

import (
	"context"
	"errors"

	"github.com/yaroher/ratel/pkg/dml/set"
	"github.com/yaroher/ratel/pkg/repository"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func (s *Service) CreateWorkloadPreset(ctx context.Context, tenantID *iampb.TenantId, createdBy *iampb.UserId, preset *catalogpb.WorkloadPreset) (*catalogpb.WorkloadPreset, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CreateWorkloadPreset",
		func(ctx context.Context, _ any) (*catalogpb.WorkloadPreset, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr, func(ctx context.Context) (*catalogpb.WorkloadPreset, error) {
				now := timestamppb.Now()
				preset.Id = &catalogpb.WorkloadPresetId{Value: ids.New()}
				preset.TenantId = tenantID
				preset.CreatedBy = createdBy
				preset.Timestamps = &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now}
				if err := s.workloadPresetRepo.Insert(ctx, preset); err != nil {
					return nil, err
				}
				return preset, nil
			})
		})
}

func (s *Service) GetWorkloadPreset(ctx context.Context, id *catalogpb.WorkloadPresetId) (*catalogpb.WorkloadPreset, error) {
	p, err := s.workloadPresetRepo.SelectOne(ctx, set.Eq(catalogpb.WorkloadPresetColumnId, id.GetValue()))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("workload_preset", id.GetValue()))
		}
		return nil, err
	}
	return p, nil
}

func (s *Service) ListWorkloadPresets(ctx context.Context, tenantID *iampb.TenantId) ([]*catalogpb.WorkloadPreset, error) {
	return s.workloadPresetRepo.Select(ctx, set.Eq(catalogpb.WorkloadPresetColumnTenantId, tenantID.GetValue()))
}

func (s *Service) DeleteWorkloadPreset(ctx context.Context, id *catalogpb.WorkloadPresetId) error {
	_, err := s.workloadPresetRepo.Delete(ctx, set.Eq(catalogpb.WorkloadPresetColumnId, id.GetValue()))
	if err != nil && errors.Is(err, repository.ErrNotFound) {
		return domainerr.NotFound(domainerr.ResourceInfo("workload_preset", id.GetValue()))
	}
	return err
}

func (s *Service) CloneWorkloadPreset(ctx context.Context, id *catalogpb.WorkloadPresetId, callerID *iampb.UserId) (*catalogpb.WorkloadPreset, error) {
	original, err := s.GetWorkloadPreset(ctx, id)
	if err != nil {
		return nil, err
	}
	cloned := &catalogpb.WorkloadPreset{
		TenantId: original.GetTenantId(),
		Identity: original.GetIdentity(),
		Workload: original.GetWorkload(),
	}
	return s.CreateWorkloadPreset(ctx, original.GetTenantId(), callerID, cloned)
}
```

- [ ] **Step 2: Append test**

Append to `integration_test.go`:

```go
func TestCreateAndListWorkloadPresets(t *testing.T) {
	f := fixture.NewCatalog(t)
	ctx := context.Background()

	u, _ := f.IAM.CreateUser(ctx, &iampb.User{Email: "w@e.com", Nickname: "w"}, "P@ss1234!")
	tn, _ := f.IAM.CreateTenant(ctx, &iampb.Tenant{Name: "T"}, u.GetId())

	preset := &catalogpb.WorkloadPreset{
		Identity: &commonpb.Identity{Name: "tpcc-default"},
		Workload: &catalogpb.Workload{},
	}
	created, err := f.Catalog.CreateWorkloadPreset(ctx, tn.GetId(), u.GetId(), preset)
	require.NoError(t, err)
	require.NotEmpty(t, created.GetId().GetValue())

	list, err := f.Catalog.ListWorkloadPresets(ctx, tn.GetId())
	require.NoError(t, err)
	require.Len(t, list, 1)
}
```

- [ ] **Step 3: Run + commit**

```bash
go test ./internal/domain/services/catalog/... -v -run TestCreateAndListWorkloadPresets
```
Expected: PASS.

```bash
git add internal/domain/services/catalog/
git commit -m "feat(catalog): WorkloadPreset CRUD + clone"
```

---

## Task 5: Package CRUD + binary upload via S3

**Files:**
- Create: `internal/domain/services/catalog/packages.go`
- Adapt: `internal/infrastructure/s3/client.go`

- [ ] **Step 1: Inspect existing s3 package**

```bash
ls internal/infrastructure/s3/
```

If pre-existing files lean on old domain types, replace `internal/infrastructure/s3/client.go` with a clean implementation:

```go
package s3

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	awsv2cfg "github.com/aws/aws-sdk-go-v2/config"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/stroppy-io/stroppy-cloud/internal/core/configurator"
)

type Client struct {
	api    *awss3.Client
	bucket string

	presign *awss3.PresignClient
}

// New builds an S3 client from configurator entry.
func New(ctx context.Context, cfg configurator.S3Config) (*Client, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3: bucket required")
	}
	loadOpts := []func(*awsv2cfg.LoadOptions) error{
		awsv2cfg.WithRegion(cfg.Region),
		awsv2cfg.WithCredentialsProvider(
			awscreds.NewStaticCredentialsProvider(
				envOrEmpty(cfg.AccessKeyEnv),
				envOrEmpty(cfg.SecretKeyEnv),
				"",
			),
		),
	}
	awsCfg, err := awsv2cfg.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("s3: load aws config: %w", err)
	}
	opts := []func(*awss3.Options){}
	if cfg.Endpoint != "" {
		opts = append(opts, func(o *awss3.Options) {
			o.BaseEndpoint = &cfg.Endpoint
			o.UsePathStyle = cfg.UsePathStyle
		})
	}
	api := awss3.NewFromConfig(awsCfg, opts...)
	return &Client{api: api, bucket: cfg.Bucket, presign: awss3.NewPresignClient(api)}, nil
}

func envOrEmpty(env string) string {
	if env == "" {
		return ""
	}
	v, _ := lookupEnv(env)
	return v
}

// PutObject stores a blob. Returns ETag.
func (c *Client) PutObject(ctx context.Context, key string, body io.Reader, contentType string) (string, error) {
	out, err := c.api.PutObject(ctx, &awss3.PutObjectInput{
		Bucket:      &c.bucket,
		Key:         &key,
		Body:        body,
		ContentType: &contentType,
	})
	if err != nil {
		return "", err
	}
	if out.ETag == nil {
		return "", nil
	}
	return *out.ETag, nil
}

// PresignGet returns a short-lived URL for downloading the key.
func (c *Client) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := c.presign.PresignGetObject(ctx, &awss3.GetObjectInput{Bucket: &c.bucket, Key: &key},
		awss3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// Delete removes a key.
func (c *Client) Delete(ctx context.Context, key string) error {
	_, err := c.api.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: &c.bucket, Key: &key})
	if err != nil {
		var nsk *s3types.NoSuchKey
		if asErr(err, &nsk) {
			return nil
		}
		return err
	}
	return nil
}

func asErr(err error, target any) bool {
	type aser interface{ As(any) bool }
	if x, ok := err.(aser); ok {
		return x.As(target)
	}
	return false
}

var lookupEnv = func(k string) (string, bool) {
	return defaultLookup(k)
}

func defaultLookup(k string) (string, bool) {
	return getenv(k)
}

func getenv(k string) (string, bool) {
	v := osGetenv(k)
	return v, v != ""
}

var osGetenv = func(k string) string {
	return goosGetenv(k)
}

func goosGetenv(k string) string { return httpStdEnv(k) }

// httpStdEnv is replaced by os.Getenv at compile via init below.
var httpStdEnv = func(k string) string { return "" }

func init() {
	httpStdEnv = func(k string) string { return httpStdLookup(k) }
}

func httpStdLookup(k string) string { return httpStdProxy(k) }

// httpStdProxy is overridden by Go stdlib at link-time? — replace this chain
// with a single os.Getenv call. Trim the indirection:
```

Replace the trailing indirection chain. Final clean s3 module — keep
the public API but drop the `lookupEnv` chain. Final `client.go` (canonical):

```go
package s3

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	awsv2cfg "github.com/aws/aws-sdk-go-v2/config"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/stroppy-io/stroppy-cloud/internal/core/configurator"
)

type Client struct {
	api     *awss3.Client
	presign *awss3.PresignClient
	bucket  string
}

func New(ctx context.Context, cfg configurator.S3Config) (*Client, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3: bucket required")
	}
	awsCfg, err := awsv2cfg.LoadDefaultConfig(ctx,
		awsv2cfg.WithRegion(cfg.Region),
		awsv2cfg.WithCredentialsProvider(awscreds.NewStaticCredentialsProvider(os.Getenv(cfg.AccessKeyEnv), os.Getenv(cfg.SecretKeyEnv), "")),
	)
	if err != nil {
		return nil, fmt.Errorf("s3: load aws config: %w", err)
	}
	opts := []func(*awss3.Options){}
	if cfg.Endpoint != "" {
		opts = append(opts, func(o *awss3.Options) {
			o.BaseEndpoint = &cfg.Endpoint
			o.UsePathStyle = cfg.UsePathStyle
		})
	}
	api := awss3.NewFromConfig(awsCfg, opts...)
	return &Client{api: api, bucket: cfg.Bucket, presign: awss3.NewPresignClient(api)}, nil
}

func (c *Client) PutObject(ctx context.Context, key string, body io.Reader, contentType string) (string, error) {
	out, err := c.api.PutObject(ctx, &awss3.PutObjectInput{Bucket: &c.bucket, Key: &key, Body: body, ContentType: &contentType})
	if err != nil {
		return "", err
	}
	if out.ETag == nil {
		return "", nil
	}
	return *out.ETag, nil
}

func (c *Client) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := c.presign.PresignGetObject(ctx, &awss3.GetObjectInput{Bucket: &c.bucket, Key: &key},
		awss3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	_, err := c.api.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: &c.bucket, Key: &key})
	if err != nil {
		var nsk *s3types.NoSuchKey
		// Fast type assertion (avoids errors.As import).
		if _, ok := any(err).(*s3types.NoSuchKey); ok {
			_ = nsk
			return nil
		}
	}
	return err
}
```

- [ ] **Step 2: `packages.go`**

```go
package catalog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/yaroher/ratel/pkg/dml/set"
	"github.com/yaroher/ratel/pkg/repository"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/s3"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// PackageStorage is a thin port over S3 to keep the service unit-testable.
type PackageStorage interface {
	Put(ctx context.Context, key string, body io.Reader, contentType string) (etag string, err error)
	Presign(ctx context.Context, key string, ttl time.Duration) (url string, err error)
	Delete(ctx context.Context, key string) error
}

// NewS3PackageStorage adapts s3.Client.
func NewS3PackageStorage(cli *s3.Client) PackageStorage { return &s3PackageStorage{cli: cli} }

type s3PackageStorage struct{ cli *s3.Client }

func (s *s3PackageStorage) Put(ctx context.Context, key string, body io.Reader, contentType string) (string, error) {
	return s.cli.PutObject(ctx, key, body, contentType)
}

func (s *s3PackageStorage) Presign(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return s.cli.PresignGet(ctx, key, ttl)
}

func (s *s3PackageStorage) Delete(ctx context.Context, key string) error { return s.cli.Delete(ctx, key) }

// SetPackageStorage wires the storage adapter into the service.
func (s *Service) SetPackageStorage(p PackageStorage) { s.pkgStorage = p }

// CreatePackage persists a Package row. Binary upload happens via UploadPackageBinary.
func (s *Service) CreatePackage(ctx context.Context, tenantID *iampb.TenantId, callerID *iampb.UserId, pkg *catalogpb.Package) (*catalogpb.Package, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CreatePackage",
		func(ctx context.Context, _ any) (*catalogpb.Package, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr, func(ctx context.Context) (*catalogpb.Package, error) {
				now := timestamppb.Now()
				pkg.Id = &catalogpb.PackageId{Value: ids.New()}
				if !pkg.GetIsBuiltin() {
					pkg.TenantId = tenantID
				}
				pkg.CreatedBy = callerID
				pkg.Timestamps = &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now}
				if err := s.packageRepo.Insert(ctx, pkg); err != nil {
					return nil, err
				}
				return pkg, nil
			})
		})
}

func (s *Service) GetPackage(ctx context.Context, id *catalogpb.PackageId) (*catalogpb.Package, error) {
	p, err := s.packageRepo.SelectOne(ctx, set.Eq(catalogpb.PackageColumnId, id.GetValue()))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("package", id.GetValue()))
		}
		return nil, err
	}
	return p, nil
}

func (s *Service) ListPackages(ctx context.Context, tenantID *iampb.TenantId, dbKind *catalogpb.Database_Kind) ([]*catalogpb.Package, error) {
	// Filter = (tenant_id IS NULL OR tenant_id = $1) [AND db_kind = $2]
	filters := []set.Filter{set.Or(
		set.IsNull(catalogpb.PackageColumnTenantId),
		set.Eq(catalogpb.PackageColumnTenantId, tenantID.GetValue()),
	)}
	if dbKind != nil {
		filters = append(filters, set.Eq(catalogpb.PackageColumnDbKind, dbKind.String()))
	}
	return s.packageRepo.Select(ctx, set.And(filters...))
}

func (s *Service) DeletePackage(ctx context.Context, id *catalogpb.PackageId) error {
	_, err := s.packageRepo.Delete(ctx, set.Eq(catalogpb.PackageColumnId, id.GetValue()))
	if err != nil && errors.Is(err, repository.ErrNotFound) {
		return domainerr.NotFound(domainerr.ResourceInfo("package", id.GetValue()))
	}
	return err
}

func (s *Service) ClonePackage(ctx context.Context, id *catalogpb.PackageId, tenantID *iampb.TenantId, callerID *iampb.UserId) (*catalogpb.Package, error) {
	orig, err := s.GetPackage(ctx, id)
	if err != nil {
		return nil, err
	}
	cloned := &catalogpb.Package{
		IsBuiltin: false,
		Identity:  orig.GetIdentity(),
		DbKind:    orig.GetDbKind(),
		DbVersion: orig.GetDbVersion(),
		Source:    orig.GetSource(),
	}
	return s.CreatePackage(ctx, tenantID, callerID, cloned)
}

// UploadPackageBinary stores a binary blob in S3 and returns a stable key
// the Package row can reference.
func (s *Service) UploadPackageBinary(ctx context.Context, tenantID *iampb.TenantId, filename string, body io.Reader, contentType string) (string, error) {
	if s.pkgStorage == nil {
		return "", domainerr.E(0 /* INTERNAL */).WithCause(fmt.Errorf("package storage not configured"))
	}
	key := fmt.Sprintf("packages/%s/%s/%s", tenantID.GetValue(), ids.New(), filename)
	if _, err := s.pkgStorage.Put(ctx, key, body, contentType); err != nil {
		return "", err
	}
	return key, nil
}

// PresignPackageDownload returns a short-lived URL to fetch the binary.
func (s *Service) PresignPackageDownload(ctx context.Context, key string) (string, error) {
	return s.pkgStorage.Presign(ctx, key, 10*time.Minute)
}
```

- [ ] **Step 3: Add storage field to Service**

Append to `service.go` struct definition (insert before `txMgr`):

```go
pkgStorage PackageStorage
```

…and ensure the field is zero-valued by default in `New`.

- [ ] **Step 4: Append test**

Append to `integration_test.go`:

```go
func TestPackageCRUDWithoutBinary(t *testing.T) {
	f := fixture.NewCatalog(t)
	ctx := context.Background()

	u, _ := f.IAM.CreateUser(ctx, &iampb.User{Email: "p@e.com", Nickname: "p"}, "P@ss1234!")
	tn, _ := f.IAM.CreateTenant(ctx, &iampb.Tenant{Name: "T"}, u.GetId())

	pkg := &catalogpb.Package{
		Identity:  &commonpb.Identity{Name: "postgres-15"},
		DbKind:    catalogpb.Database_DATABASE_KIND_POSTGRES,
		DbVersion: "15",
		Source: &catalogpb.Package_PackageSource{
			Source: &catalogpb.Package_PackageSource_Apt{
				Apt: &catalogpb.Package_AptSource{
					AptPackages: []string{"postgresql-15"},
				},
			},
		},
	}
	created, err := f.Catalog.CreatePackage(ctx, tn.GetId(), u.GetId(), pkg)
	require.NoError(t, err)
	require.NotEmpty(t, created.GetId().GetValue())

	list, err := f.Catalog.ListPackages(ctx, tn.GetId(), nil)
	require.NoError(t, err)
	require.NotEmpty(t, list)
}
```

- [ ] **Step 5: Build + test**

```bash
go build ./internal/infrastructure/s3/... ./internal/domain/services/catalog/...
go test ./internal/domain/services/catalog/... -v -run TestPackageCRUDWithoutBinary
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/infrastructure/s3/ internal/domain/services/catalog/
git commit -m "feat(catalog,s3): Package CRUD + S3-backed binary upload port"
```

---

## Task 6: Settings CRUD

**Files:**
- Create: `internal/domain/services/catalog/settings.go`

- [ ] **Step 1: Implement**

```go
package catalog

import (
	"context"
	"errors"

	"github.com/yaroher/ratel/pkg/dml/set"
	"github.com/yaroher/ratel/pkg/repository"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// ListSettings returns all SettingsItems for a tenant.
func (s *Service) ListSettings(ctx context.Context, tenantID *iampb.TenantId) ([]*catalogpb.SettingsItem, error) {
	return s.settingsRepo.Select(ctx, set.Eq(catalogpb.SettingsItemColumnTenantId, tenantID.GetValue()))
}

// SetSetting upserts a single (tenant, part, key) entry with a typed value.
func (s *Service) SetSetting(ctx context.Context, tenantID *iampb.TenantId, part catalogpb.SettingsItem_Part, key catalogpb.SettingsItem_Key, value *catalogpb.SettingsItem_Value) (*catalogpb.SettingsItem, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "SetSetting",
		func(ctx context.Context, _ any) (*catalogpb.SettingsItem, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr, func(ctx context.Context) (*catalogpb.SettingsItem, error) {
				existing, err := s.settingsRepo.SelectOne(ctx,
					set.And(
						set.Eq(catalogpb.SettingsItemColumnTenantId, tenantID.GetValue()),
						set.Eq(catalogpb.SettingsItemColumnPart, part.String()),
						set.Eq(catalogpb.SettingsItemColumnKey, key.String()),
					))
				now := timestamppb.Now()
				if err != nil {
					if !errors.Is(err, repository.ErrNotFound) {
						return nil, err
					}
					item := &catalogpb.SettingsItem{
						Id:         &catalogpb.SettingsItemId{Value: ids.New()},
						TenantId:   tenantID,
						Part:       part,
						Key:        key,
						Value:      value,
						Timestamps: &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now},
					}
					if err := s.settingsRepo.Insert(ctx, item); err != nil {
						return nil, err
					}
					return item, nil
				}
				existing.Value = value
				existing.GetTimestamps().UpdatedAt = now
				_, err = s.settingsRepo.Update(ctx,
					set.Update(catalogpb.SettingsItemColumnValue, value),
					set.Eq(catalogpb.SettingsItemColumnId, existing.GetId().GetValue()),
				)
				if err != nil {
					return nil, err
				}
				return existing, nil
			})
		})
}

// GetSetting fetches one entry.
func (s *Service) GetSetting(ctx context.Context, tenantID *iampb.TenantId, part catalogpb.SettingsItem_Part, key catalogpb.SettingsItem_Key) (*catalogpb.SettingsItem, error) {
	row, err := s.settingsRepo.SelectOne(ctx,
		set.And(
			set.Eq(catalogpb.SettingsItemColumnTenantId, tenantID.GetValue()),
			set.Eq(catalogpb.SettingsItemColumnPart, part.String()),
			set.Eq(catalogpb.SettingsItemColumnKey, key.String()),
		))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("settings_item", part.String()+"/"+key.String()))
		}
		return nil, err
	}
	return row, nil
}
```

- [ ] **Step 2: Append test**

```go
func TestSettingsSetAndGet(t *testing.T) {
	f := fixture.NewCatalog(t)
	ctx := context.Background()

	u, _ := f.IAM.CreateUser(ctx, &iampb.User{Email: "s@e.com", Nickname: "s"}, "P@ss1234!")
	tn, _ := f.IAM.CreateTenant(ctx, &iampb.Tenant{Name: "T"}, u.GetId())

	val := &catalogpb.SettingsItem_Value{Value: &catalogpb.SettingsItem_Value_StringValue{StringValue: "yc-token-123"}}
	row, err := f.Catalog.SetSetting(ctx, tn.GetId(),
		catalogpb.SettingsItem_PART_YANDEX_CLOUD,
		catalogpb.SettingsItem_KEY_YANDEX_CLOUD_TOKEN,
		val,
	)
	require.NoError(t, err)
	require.NotEmpty(t, row.GetId().GetValue())

	got, err := f.Catalog.GetSetting(ctx, tn.GetId(),
		catalogpb.SettingsItem_PART_YANDEX_CLOUD,
		catalogpb.SettingsItem_KEY_YANDEX_CLOUD_TOKEN,
	)
	require.NoError(t, err)
	require.Equal(t, "yc-token-123", got.GetValue().GetStringValue())
}
```

- [ ] **Step 3: Run + commit**

```bash
go test ./internal/domain/services/catalog/... -v -run TestSettingsSetAndGet
git add internal/domain/services/catalog/
git commit -m "feat(catalog): SettingsItem upsert + get"
```

---

## Task 7: Stroppy service skeleton + versions proxy (with valkey cache)

**Files:**
- Create: `internal/domain/services/stroppy/service.go`, `versions.go`
- Create: `internal/testutil/fixture/stroppy.go`

- [ ] **Step 1: `service.go`**

```go
package stroppy

import (
	"net/http"

	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/stroppybin"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/valkey"
	stroppypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/stroppy"
)

// Service serves binary version listings + probe/preview wizard helpers.
type Service struct {
	*tracing.Entity

	runner     *stroppybin.Runner
	httpClient *http.Client

	versionCache *valkey.JSONCache[stroppypb.StroppyVersionList]
	commitCache  *valkey.JSONCache[stroppypb.StroppyCommitList]

	releasesURL string // GitHub-style API URL (configurable)
	commitsURL  string
}

func New(runner *stroppybin.Runner, vk *valkey.Client, releasesURL, commitsURL string) *Service {
	return &Service{
		Entity:       tracing.NewEntity("stroppy.Service"),
		runner:       runner,
		httpClient:   &http.Client{Timeout: 0}, // configured per-call
		versionCache: valkey.NewJSONCache[stroppypb.StroppyVersionList](vk, "stroppy:versions"),
		commitCache:  valkey.NewJSONCache[stroppypb.StroppyCommitList](vk, "stroppy:commits"),
		releasesURL:  releasesURL,
		commitsURL:   commitsURL,
	}
}
```

- [ ] **Step 2: `versions.go`**

```go
package stroppy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	stroppypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/stroppy"
)

const versionCacheTTL = 5 * time.Minute

// ListStroppyVersions returns cached or freshly-fetched GitHub releases.
func (s *Service) ListStroppyVersions(ctx context.Context) (*stroppypb.StroppyVersionList, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "ListStroppyVersions",
		func(ctx context.Context, _ any) (*stroppypb.StroppyVersionList, error) {
			if cached, ok, err := s.versionCache.Get(ctx, "all"); err == nil && ok {
				return &cached, nil
			}
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.releasesURL, nil)
			req.Header.Set("Accept", "application/vnd.github+json")
			res, err := s.httpClient.Do(req)
			if err != nil {
				return nil, fmt.Errorf("stroppy: github releases: %w", err)
			}
			defer res.Body.Close()
			if res.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("stroppy: github releases status %d", res.StatusCode)
			}
			var ghReleases []struct {
				TagName     string    `json:"tag_name"`
				Prerelease  bool      `json:"prerelease"`
				PublishedAt time.Time `json:"published_at"`
				HTMLURL     string    `json:"html_url"`
				Assets      []struct {
					BrowserDownloadURL string `json:"browser_download_url"`
				} `json:"assets"`
			}
			if err := json.NewDecoder(res.Body).Decode(&ghReleases); err != nil {
				return nil, fmt.Errorf("stroppy: decode releases: %w", err)
			}
			out := &stroppypb.StroppyVersionList{}
			for _, r := range ghReleases {
				v := &stroppypb.StroppyVersion{
					Tag:        strings.TrimPrefix(r.TagName, "refs/tags/"),
					Prerelease: r.Prerelease,
					ReleaseUrl: r.HTMLURL,
				}
				if !r.PublishedAt.IsZero() {
					v.PublishedAt = timestamppb.New(r.PublishedAt)
				}
				for _, a := range r.Assets {
					v.AssetUrls = append(v.AssetUrls, a.BrowserDownloadURL)
				}
				out.Versions = append(out.Versions, v)
			}
			_ = s.versionCache.Set(ctx, "all", *out, versionCacheTTL)
			return out, nil
		})
}

// ListStroppyCommits is analogous for the commits feed. Same caching window.
func (s *Service) ListStroppyCommits(ctx context.Context) (*stroppypb.StroppyCommitList, error) {
	if cached, ok, err := s.commitCache.Get(ctx, "all"); err == nil && ok {
		return &cached, nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.commitsURL, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stroppy: github commits: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stroppy: github commits status %d", res.StatusCode)
	}
	var ghCommits []struct {
		Sha    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Name string    `json:"name"`
				Date time.Time `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ghCommits); err != nil {
		return nil, fmt.Errorf("stroppy: decode commits: %w", err)
	}
	out := &stroppypb.StroppyCommitList{}
	for _, c := range ghCommits {
		row := &stroppypb.StroppyCommit{
			Sha:      c.Sha,
			ShortSha: c.Sha[:8],
			Message:  c.Commit.Message,
			Author:   c.Commit.Author.Name,
		}
		if !c.Commit.Author.Date.IsZero() {
			row.CommittedAt = timestamppb.New(c.Commit.Author.Date)
		}
		out.Commits = append(out.Commits, row)
	}
	_ = s.commitCache.Set(ctx, "all", *out, versionCacheTTL)
	return out, nil
}
```

- [ ] **Step 3: `fixture/stroppy.go`**

```go
package fixture

import (
	"net/http/httptest"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/stroppy"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/stroppybin"
)

// StroppyFixture wires a Stroppy service against a stub GitHub server + the
// real binary runner.
type StroppyFixture struct {
	*IAMFixture
	Stroppy *stroppy.Service
	GH      *httptest.Server
}

func NewStroppy(t *testing.T, releases, commits string) *StroppyFixture {
	t.Helper()
	iam := NewIAM(t)
	gh := httptest.NewServer(stubGitHubHandler(releases, commits))
	t.Cleanup(gh.Close)

	runner := stroppybin.New("dev", "/tmp/stroppy-test-binaries")
	svc := stroppy.New(runner, fakeValkey(t), gh.URL+"/releases", gh.URL+"/commits")
	return &StroppyFixture{IAMFixture: iam, Stroppy: svc, GH: gh}
}
```

You need helpers `stubGitHubHandler`, `fakeValkey`. Append to `fixture/stroppy.go`:

```go
import (
	"net/http"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/valkey"
)

func stubGitHubHandler(releasesJSON, commitsJSON string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/releases", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(releasesJSON))
	})
	mux.HandleFunc("/commits", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(commitsJSON))
	})
	return mux
}

// fakeValkey returns an in-memory valkey.Client implementation for tests.
// Backed by miniredis or a custom in-mem impl — pick whatever is in tree.
func fakeValkey(t *testing.T) *valkey.Client {
	t.Helper()
	cli, err := valkey.NewInMemory()
	if err != nil {
		t.Fatalf("fake valkey: %v", err)
	}
	t.Cleanup(cli.Close)
	return cli
}
```

If `valkey.NewInMemory()` doesn't yet exist in your `internal/infrastructure/valkey/` package, add it now:

```go
// internal/infrastructure/valkey/inmemory.go
package valkey

import (
	"context"
	"sync"
	"time"
)

type entry struct {
	val     string
	expires time.Time
}

type memClient struct {
	mu   sync.Mutex
	data map[string]entry
}

// NewInMemory returns a Client backed by a process-local map. Tests only.
func NewInMemory() (*Client, error) {
	return &Client{impl: &memClient{data: make(map[string]entry)}}, nil
}

// adapt the Client surface — assuming Client wraps an interface impl with:
//   Get, Set, SetNX, Del, Close
```

If the existing `valkey.Client` struct doesn't allow an injected impl, refactor
to thin out: `Client` becomes an interface struct holding a concrete `impl`
that the in-memory and real implementations satisfy. Keep this refactor
self-contained to `internal/infrastructure/valkey/`.

- [ ] **Step 4: Write versions test**

Create `internal/domain/services/stroppy/integration_test.go`:

```go
package stroppy_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestListStroppyVersionsCached(t *testing.T) {
	releases := `[
		{"tag_name":"v4.1.0","prerelease":false,"published_at":"2026-04-01T10:00:00Z","html_url":"https://example.com/rel/v4.1.0","assets":[{"browser_download_url":"https://example.com/a"}]},
		{"tag_name":"v4.0.0","prerelease":false,"published_at":"2026-03-01T10:00:00Z","html_url":"https://example.com/rel/v4.0.0","assets":[]}
	]`
	f := fixture.NewStroppy(t, releases, "[]")

	list, err := f.Stroppy.ListStroppyVersions(context.Background())
	require.NoError(t, err)
	require.Len(t, list.GetVersions(), 2)
	require.Equal(t, "v4.1.0", list.GetVersions()[0].GetTag())

	// Second call must hit cache — close upstream to prove
	f.GH.Close()
	list2, err := f.Stroppy.ListStroppyVersions(context.Background())
	require.NoError(t, err)
	require.Len(t, list2.GetVersions(), 2)
}
```

- [ ] **Step 5: Build + run**

```bash
go build ./internal/domain/services/stroppy/... ./internal/infrastructure/valkey/...
go test ./internal/domain/services/stroppy/... -v -run TestListStroppyVersions
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/services/stroppy/ internal/infrastructure/valkey/ internal/testutil/fixture/stroppy.go
git commit -m "feat(stroppy): service + GitHub-proxy versions/commits with valkey JSON cache"
```

---

## Task 8: Stroppy probe + preview methods

**Files:**
- Create: `internal/domain/services/stroppy/probe.go`, `preview.go`

- [ ] **Step 1: `probe.go`**

```go
package stroppy

import (
	"context"
	"encoding/json"

	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/stroppybin"
	stroppypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/stroppy"
)

// ProbeStroppyConfig invokes `stroppy probe -o json` and transforms the
// stdout JSON into the SPA-facing proto response.
func (s *Service) ProbeStroppyConfig(ctx context.Context, req *stroppypb.ProbeStroppyConfigRequest) (*stroppypb.ProbeStroppyConfigResponse, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "ProbeStroppyConfig",
		func(ctx context.Context, _ any) (*stroppypb.ProbeStroppyConfigResponse, error) {
			files := make([]stroppybin.WorkloadFile, 0, len(req.GetFiles()))
			for _, f := range req.GetFiles() {
				files = append(files, stroppybin.WorkloadFile{Name: f.GetName(), Content: f.GetContent()})
			}
			result, err := s.runner.RunProbe(ctx, stroppybin.ProbeInput{
				Version:      req.GetStroppyVersion(),
				Script:       req.GetScript(),
				SQL:          req.GetSql(),
				DriverType:   req.GetDriverType(),
				PoolSize:     req.GetPoolSize(),
				ScaleFactor:  req.GetScaleFactor(),
				Env:          req.GetEnv(),
				Files:        files,
				IncludeHuman: req.GetIncludeHuman(),
			})
			if err != nil {
				return nil, err
			}
			resp := &stroppypb.ProbeStroppyConfigResponse{RawJson: string(result.StdoutJSON)}
			if req.GetIncludeHuman() {
				resp.Human = result.Human
			}
			// Best-effort enrichment — parse the probe JSON for env_declarations / steps / sql_sections / driver_setups
			var parsed struct {
				EnvDecls []struct {
					Name, Type, Default, Description string
					Required                         bool
				} `json:"env_declarations"`
				Steps []struct{ Name, Kind, Description string } `json:"steps"`
				SQL   []struct{ Name, Sql string }               `json:"sql_sections"`
				Drv   []struct {
					DriverType string            `json:"driver_type"`
					Settings   map[string]string `json:"settings"`
				} `json:"driver_setups"`
			}
			if err := json.Unmarshal(result.StdoutJSON, &parsed); err == nil {
				for _, e := range parsed.EnvDecls {
					resp.EnvDeclarations = append(resp.EnvDeclarations, &stroppypb.ProbeEnvDecl{
						Name: e.Name, Type: e.Type, DefaultValue: e.Default,
						Description: e.Description, Required: e.Required,
					})
				}
				for _, st := range parsed.Steps {
					resp.Steps = append(resp.Steps, &stroppypb.ProbeStep{Name: st.Name, Kind: st.Kind, Description: st.Description})
				}
				for _, ss := range parsed.SQL {
					resp.SqlSections = append(resp.SqlSections, &stroppypb.ProbeSqlSection{Name: ss.Name, Sql: ss.Sql})
				}
				for _, d := range parsed.Drv {
					resp.DriverSetups = append(resp.DriverSetups, &stroppypb.ProbeDriverSetup{DriverType: d.DriverType, Settings: d.Settings})
				}
			}
			return resp, nil
		})
}
```

- [ ] **Step 2: `preview.go`**

```go
package stroppy

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/stroppybin"
	stroppypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/stroppy"
)

func (s *Service) PreviewStroppyConfig(ctx context.Context, req *stroppypb.PreviewStroppyConfigRequest) (*stroppypb.PreviewStroppyConfigResponse, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "PreviewStroppyConfig",
		func(ctx context.Context, _ any) (*stroppypb.PreviewStroppyConfigResponse, error) {
			if override := req.GetConfigOverrideJson(); override != "" {
				return &stroppypb.PreviewStroppyConfigResponse{StroppyConfigJson: override}, nil
			}
			files := make([]stroppybin.WorkloadFile, 0, len(req.GetFiles()))
			for _, f := range req.GetFiles() {
				files = append(files, stroppybin.WorkloadFile{Name: f.GetName(), Content: f.GetContent()})
			}
			data, err := stroppybin.RenderConfig(ctx, stroppybin.PreviewInput{
				Version: req.GetStroppyVersion(), Script: req.GetScript(), SQL: req.GetSql(),
				DriverType: req.GetDriverType(), PoolSize: req.GetPoolSize(), ScaleFactor: req.GetScaleFactor(),
				Env: req.GetEnv(), Files: files,
			})
			if err != nil {
				return nil, err
			}
			return &stroppypb.PreviewStroppyConfigResponse{StroppyConfigJson: string(data)}, nil
		})
}
```

- [ ] **Step 3: Append preview test**

Append to `integration_test.go`:

```go
import (
	stroppypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/stroppy"
)

func TestPreviewWithOverridePassthrough(t *testing.T) {
	f := fixture.NewStroppy(t, "[]", "[]")
	req := &stroppypb.PreviewStroppyConfigRequest{
		ConfigOverrideJson: `{"hand":"crafted"}`,
	}
	out, err := f.Stroppy.PreviewStroppyConfig(context.Background(), req)
	require.NoError(t, err)
	require.JSONEq(t, `{"hand":"crafted"}`, out.GetStroppyConfigJson())
}

func TestPreviewRenders(t *testing.T) {
	f := fixture.NewStroppy(t, "[]", "[]")
	req := &stroppypb.PreviewStroppyConfigRequest{
		Script:     "tpcc/procs",
		DriverType: "postgres",
		PoolSize:   16,
	}
	out, err := f.Stroppy.PreviewStroppyConfig(context.Background(), req)
	require.NoError(t, err)
	require.Contains(t, out.GetStroppyConfigJson(), "tpcc/procs")
	require.Contains(t, out.GetStroppyConfigJson(), "postgres")
}
```

- [ ] **Step 4: Build + run**

```bash
go test ./internal/domain/services/stroppy/... -v -run TestPreview
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/services/stroppy/
git commit -m "feat(stroppy): probe (subprocess) + preview (render-only) methods"
```

---

## Task 9: Connect handlers for catalog + stroppy

**Files:**
- Create: `internal/transport/connect/catalog.go`, `stroppy.go`
- Modify: `internal/transport/connect/server.go`

- [ ] **Step 1: `catalog.go`**

```go
package connectrpc

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

type CatalogHandler struct{ svc *catalog.Service }

func NewCatalogHandler(svc *catalog.Service) *CatalogHandler { return &CatalogHandler{svc: svc} }

// --- DatabasePreset ---

func (h *CatalogHandler) CreateDatabasePreset(ctx context.Context, req *connect.Request[catalogpb.CreateDatabasePresetRequest]) (*connect.Response[catalogpb.DatabasePreset], error) {
	tenantID := &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	callerID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	p, err := h.svc.CreateDatabasePreset(ctx, tenantID, callerID, req.Msg.GetPreset())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(p), nil
}

func (h *CatalogHandler) GetDatabasePreset(ctx context.Context, req *connect.Request[catalogpb.DatabasePresetId]) (*connect.Response[catalogpb.DatabasePreset], error) {
	p, err := h.svc.GetDatabasePreset(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(p), nil
}

func (h *CatalogHandler) ListDatabasePresets(ctx context.Context, req *connect.Request[iampb.TenantId]) (*connect.Response[catalogpb.DatabasePreset_List], error) {
	list, err := h.svc.ListDatabasePresets(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&catalogpb.DatabasePreset_List{DatabasePresets: list}), nil
}

func (h *CatalogHandler) DeleteDatabasePreset(ctx context.Context, req *connect.Request[catalogpb.DatabasePresetId]) (*connect.Response[emptypb.Empty], error) {
	if err := h.svc.DeleteDatabasePreset(ctx, req.Msg); err != nil {
		return nil, err
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *CatalogHandler) CloneDatabasePreset(ctx context.Context, req *connect.Request[catalogpb.DatabasePresetId]) (*connect.Response[catalogpb.DatabasePreset], error) {
	callerID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	p, err := h.svc.CloneDatabasePreset(ctx, req.Msg, callerID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(p), nil
}

// --- WorkloadPreset (symmetric — same shape, different proto types) ---
//
// (Implement CreateWorkloadPreset, GetWorkloadPreset, ListWorkloadPresets,
// DeleteWorkloadPreset, CloneWorkloadPreset identically to DatabasePreset
// counterparts but against workloadpb types.)

// --- Package ---

func (h *CatalogHandler) CreatePackage(ctx context.Context, req *connect.Request[catalogpb.CreatePackageRequest]) (*connect.Response[catalogpb.Package], error) {
	tenantID := &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	callerID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	p, err := h.svc.CreatePackage(ctx, tenantID, callerID, req.Msg.GetPackage())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(p), nil
}

func (h *CatalogHandler) GetPackage(ctx context.Context, req *connect.Request[catalogpb.PackageId]) (*connect.Response[catalogpb.Package], error) {
	p, err := h.svc.GetPackage(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(p), nil
}

func (h *CatalogHandler) ListPackages(ctx context.Context, req *connect.Request[catalogpb.ListPackagesRequest]) (*connect.Response[catalogpb.Package_List], error) {
	var dbKind *catalogpb.Database_Kind
	if req.Msg.GetDbKind() != catalogpb.Database_DATABASE_KIND_UNSPECIFIED {
		k := req.Msg.GetDbKind()
		dbKind = &k
	}
	tenantID := req.Msg.GetTenantId()
	if tenantID == nil {
		tenantID = &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	}
	list, err := h.svc.ListPackages(ctx, tenantID, dbKind)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&catalogpb.Package_List{Packages: list}), nil
}

func (h *CatalogHandler) DeletePackage(ctx context.Context, req *connect.Request[catalogpb.PackageId]) (*connect.Response[emptypb.Empty], error) {
	if err := h.svc.DeletePackage(ctx, req.Msg); err != nil {
		return nil, err
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *CatalogHandler) ClonePackage(ctx context.Context, req *connect.Request[catalogpb.PackageId]) (*connect.Response[catalogpb.Package], error) {
	tenantID := &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	callerID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	p, err := h.svc.ClonePackage(ctx, req.Msg, tenantID, callerID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(p), nil
}

// --- Settings ---

func (h *CatalogHandler) ListSettings(ctx context.Context, req *connect.Request[catalogpb.ListSettingsRequest]) (*connect.Response[catalogpb.SettingsItem_List], error) {
	list, err := h.svc.ListSettings(ctx, req.Msg.GetTenantId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&catalogpb.SettingsItem_List{SettingsItems: list}), nil
}

func (h *CatalogHandler) SetSetting(ctx context.Context, req *connect.Request[catalogpb.SetSettingRequest]) (*connect.Response[catalogpb.SettingsItem], error) {
	tenantID := &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	row, err := h.svc.SetSetting(ctx, tenantID, /* TODO: derive Part+Key from req */ catalogpb.SettingsItem_PART_UNSPECIFIED, catalogpb.SettingsItem_KEY_UNSPECIFIED, req.Msg.GetValue())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(row), nil
}
```

Note: The `SetSettingRequest` proto in `cloud/v1/catalog/settings.proto`
carries an `id` (`SettingsItemId`) — handler must look up the existing item
to discover its Part+Key. Adjust the handler call:

```go
existing, err := h.svc.GetSettingByID(ctx, req.Msg.GetId())
if err != nil { return nil, err }
row, err := h.svc.SetSetting(ctx, existing.GetTenantId(), existing.GetPart(), existing.GetKey(), req.Msg.GetValue())
```

…and add `GetSettingByID` to `settings.go`:

```go
func (s *Service) GetSettingByID(ctx context.Context, id *catalogpb.SettingsItemId) (*catalogpb.SettingsItem, error) {
	row, err := s.settingsRepo.SelectOne(ctx, set.Eq(catalogpb.SettingsItemColumnId, id.GetValue()))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("settings_item", id.GetValue()))
		}
		return nil, err
	}
	return row, nil
}
```

- [ ] **Step 2: `stroppy.go` handler**

```go
package connectrpc

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/stroppy"
	stroppypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/stroppy"
)

type StroppyHandler struct{ svc *stroppy.Service }

func NewStroppyHandler(svc *stroppy.Service) *StroppyHandler { return &StroppyHandler{svc: svc} }

func (h *StroppyHandler) ListStroppyVersions(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[stroppypb.StroppyVersionList], error) {
	list, err := h.svc.ListStroppyVersions(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(list), nil
}

func (h *StroppyHandler) ListStroppyCommits(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[stroppypb.StroppyCommitList], error) {
	list, err := h.svc.ListStroppyCommits(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(list), nil
}

func (h *StroppyHandler) ProbeStroppyConfig(ctx context.Context, req *connect.Request[stroppypb.ProbeStroppyConfigRequest]) (*connect.Response[stroppypb.ProbeStroppyConfigResponse], error) {
	resp, err := h.svc.ProbeStroppyConfig(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *StroppyHandler) PreviewStroppyConfig(ctx context.Context, req *connect.Request[stroppypb.PreviewStroppyConfigRequest]) (*connect.Response[stroppypb.PreviewStroppyConfigResponse], error) {
	resp, err := h.svc.PreviewStroppyConfig(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
```

- [ ] **Step 3: Update `server.go`**

Append to `Deps`:

```go
CatalogHandler *CatalogHandler
StroppyHandler *StroppyHandler
```

Update `Mount`:

```go
catalogconnectPath, catalogconnectHandler := catalogconnect.NewDatabasePresetServiceHandler(d.CatalogHandler, d.Interceptors)
mux.Handle(catalogconnectPath, catalogconnectHandler)
// repeat for WorkloadPresetService, PackageService, SettingsService

stroppyPath, stroppyHandler := stroppyconnect.NewStroppyServiceHandler(d.StroppyHandler, d.Interceptors)
mux.Handle(stroppyPath, stroppyHandler)
```

Adjust `TenantBypass`: stroppy `ListVersions`/`ListCommits` don't need tenant, add them to bypass.

- [ ] **Step 4: Build**

```bash
go build ./internal/transport/...
```
Expected: success.

- [ ] **Step 5: Commit**

```bash
git add internal/transport/connect/
git commit -m "feat(connect): catalog + stroppy handlers + bypass updates"
```

---

## Task 10: Wire catalog + stroppy into cmd_server

**Files:**
- Modify: `cmd/stroppy-cloud/cmd_server.go`

- [ ] **Step 1: Add infrastructure init**

Insert after `valkeyCli := valkey.New(cfg.Valkey)`:

```go
s3Client, err := s3.New(ctx, cfg.S3)
if err != nil {
    return fmt.Errorf("s3: %w", err)
}

stroppyRunner := stroppybin.New(cfg.Stroppy.DefaultVersion, cfg.Stroppy.BinariesDir)
```

- [ ] **Step 2: Build services**

Insert after `iamSvc := iam.New(...)`:

```go
catalogSvc := catalog.New(exec, txMgr, bus)
catalogSvc.SetPackageStorage(catalog.NewS3PackageStorage(s3Client))

stroppySvc := stroppy.New(stroppyRunner, valkeyCli, cfg.Stroppy.ReleasesURL, cfg.Stroppy.CommitsURL)
```

Add `ReleasesURL`, `CommitsURL` fields to `configurator.StroppyConfig`:

```go
ReleasesURL string `mapstructure:"releases_url"`
CommitsURL  string `mapstructure:"commits_url"`
```

And update `deployments/local/server/config.yaml` `stroppy` section:

```yaml
stroppy:
  default_version: "v4.1.0"
  binaries_dir: "/var/lib/stroppy/bin"
  releases_url: "https://api.github.com/repos/stroppy-io/stroppy/releases"
  commits_url:  "https://api.github.com/repos/stroppy-io/stroppy/commits"
```

- [ ] **Step 3: Mount handlers**

Update `connectrpc.Mount` call:

```go
mux.Handle("/", connectrpc.Mount(connectrpc.Deps{
    IAMHandler:     connectrpc.NewIAMHandler(iamSvc),
    CatalogHandler: connectrpc.NewCatalogHandler(catalogSvc),
    StroppyHandler: connectrpc.NewStroppyHandler(stroppySvc),
    Interceptors:   interceptors,
}))
```

Imports to add:
```go
"github.com/stroppy-io/stroppy-cloud/internal/domain/services/catalog"
"github.com/stroppy-io/stroppy-cloud/internal/domain/services/stroppy"
"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/s3"
"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/stroppybin"
```

- [ ] **Step 4: Build**

```bash
go build ./cmd/stroppy-cloud/...
```
Expected: success.

- [ ] **Step 5: Commit**

```bash
git add cmd/stroppy-cloud/cmd_server.go internal/core/configurator/config.go deployments/local/server/config.yaml
git commit -m "feat(server): wire catalog + stroppy services + s3 client into runtime"
```

---

## Task 11: CLI subcommands — preset, package, settings, probe, version

**Files:**
- Create: `cmd/stroppy-cloud/cmd_cli/preset.go`, `package.go`, `settings.go`, `probe.go`, `version.go`
- Modify: `cmd/stroppy-cloud/cmd_cli/root.go`
- Modify: `internal/sdk/client/client.go`

- [ ] **Step 1: Extend SDK client**

Append to `internal/sdk/client/client.go` struct fields:

```go
DatabasePreset catalogconnect.DatabasePresetServiceClient
WorkloadPreset catalogconnect.WorkloadPresetServiceClient
Package        catalogconnect.PackageServiceClient
Settings       catalogconnect.SettingsServiceClient
Stroppy        stroppyconnect.StroppyServiceClient
```

In `New`, wire them:

```go
c.DatabasePreset = catalogconnect.NewDatabasePresetServiceClient(c.httpClient, serverURL, connOpts...)
c.WorkloadPreset = catalogconnect.NewWorkloadPresetServiceClient(c.httpClient, serverURL, connOpts...)
c.Package = catalogconnect.NewPackageServiceClient(c.httpClient, serverURL, connOpts...)
c.Settings = catalogconnect.NewSettingsServiceClient(c.httpClient, serverURL, connOpts...)
c.Stroppy = stroppyconnect.NewStroppyServiceClient(c.httpClient, serverURL, connOpts...)
```

Add imports.

- [ ] **Step 2: `preset.go`**

```go
package cmd_cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/sdk/client"
)

func presetCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "preset", Short: "Manage database/workload presets"}
	cmd.AddCommand(presetDatabaseCmd(), presetWorkloadCmd())
	return cmd
}

func presetDatabaseCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "database", Short: "Database preset commands"}
	cmd.AddCommand(presetDatabaseListCmd(), presetDatabaseGetCmd(), presetDatabaseDeleteCmd())
	return cmd
}

func presetWorkloadCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "workload", Short: "Workload preset commands"}
	cmd.AddCommand(presetWorkloadListCmd())
	return cmd
}

func presetDatabaseListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List database presets in the current tenant",
		RunE: func(c *cobra.Command, _ []string) error {
			cli, tenant, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.DatabasePreset.ListDatabasePresets(context.Background(), connect.NewRequest(&iampb.TenantId{Value: tenant}))
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(resp.Msg.GetDatabasePresets())
		},
	}
}

func presetDatabaseGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Args:  cobra.ExactArgs(1),
		Short: "Get a database preset",
		RunE: func(c *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.DatabasePreset.GetDatabasePreset(context.Background(), connect.NewRequest(&catalogpb.DatabasePresetId{Value: args[0]}))
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(resp.Msg)
		},
	}
}

func presetDatabaseDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Args:  cobra.ExactArgs(1),
		Short: "Delete a database preset",
		RunE: func(c *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			_, err = cli.DatabasePreset.DeleteDatabasePreset(context.Background(), connect.NewRequest(&catalogpb.DatabasePresetId{Value: args[0]}))
			return err
		},
	}
}

func presetWorkloadListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List workload presets in the current tenant",
		RunE: func(c *cobra.Command, _ []string) error {
			cli, tenant, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.WorkloadPreset.ListWorkloadPresets(context.Background(), connect.NewRequest(&iampb.TenantId{Value: tenant}))
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(resp.Msg.GetWorkloadPresets())
		},
	}
}

// authedCLI loads creds + tenant context, returns a client.
func authedCLI() (*client.Client, string, error) {
	ctxObj, _ := client.LoadContext()
	if ctxObj == nil || ctxObj.CurrentServer == "" {
		return nil, "", fmt.Errorf("not logged in")
	}
	cred, _ := client.LoadCredentials(ctxObj.CurrentServer)
	if cred == nil || cred.AccessToken == "" {
		return nil, "", fmt.Errorf("missing access token")
	}
	if ctxObj.CurrentTenant == "" {
		return nil, "", fmt.Errorf("no active tenant; run `stroppy-cloud cli context use --tenant <id>`")
	}
	return client.New(ctxObj.CurrentServer, client.WithBearer(cred.AccessToken), client.WithTenant(ctxObj.CurrentTenant)), ctxObj.CurrentTenant, nil
}
```

`client.WithTenant` needs to exist — add it to `internal/sdk/client/client.go`:

```go
func WithTenant(tenantID string) Option {
	return func(c *Client) {
		if c.headers == nil {
			c.headers = http.Header{}
		}
		c.headers.Set("X-Tenant-Id", tenantID)
	}
}
```

- [ ] **Step 3: `package.go` — list + upload + create**

```go
package cmd_cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
)

func packageCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "package", Short: "Manage packages"}
	cmd.AddCommand(packageListCmd(), packageGetCmd(), packageDeleteCmd())
	return cmd
}

func packageListCmd() *cobra.Command {
	var kind string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List packages visible in the current tenant",
		RunE: func(c *cobra.Command, _ []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			req := &catalogpb.ListPackagesRequest{}
			if kind != "" {
				k, ok := catalogpb.Database_Kind_value[kind]
				if !ok {
					return fmt.Errorf("unknown db kind %q", kind)
				}
				dk := catalogpb.Database_Kind(k)
				req.DbKind = dk
			}
			resp, err := cli.Package.ListPackages(context.Background(), connect.NewRequest(req))
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(resp.Msg.GetPackages())
		},
	}
	cmd.Flags().StringVar(&kind, "db-kind", "", "filter by Database_Kind enum value (e.g. DATABASE_KIND_POSTGRES)")
	return cmd
}

func packageGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Args:  cobra.ExactArgs(1),
		Short: "Get a package",
		RunE: func(c *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Package.GetPackage(context.Background(), connect.NewRequest(&catalogpb.PackageId{Value: args[0]}))
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(resp.Msg)
		},
	}
}

func packageDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Args:  cobra.ExactArgs(1),
		Short: "Delete a package",
		RunE: func(c *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			_, err = cli.Package.DeletePackage(context.Background(), connect.NewRequest(&catalogpb.PackageId{Value: args[0]}))
			return err
		},
	}
}
```

- [ ] **Step 4: `settings.go`**

```go
package cmd_cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func settingsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "settings", Short: "Manage tenant settings"}
	cmd.AddCommand(settingsListCmd(), settingsSetStringCmd())
	return cmd
}

func settingsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List settings for the current tenant",
		RunE: func(c *cobra.Command, _ []string) error {
			cli, tenant, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Settings.ListSettings(context.Background(), connect.NewRequest(&catalogpb.ListSettingsRequest{TenantId: &iampb.TenantId{Value: tenant}}))
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(resp.Msg.GetSettingsItems())
		},
	}
}

func settingsSetStringCmd() *cobra.Command {
	var id, value string
	cmd := &cobra.Command{
		Use:   "set-string --id <id> --value <v>",
		Short: "Update an existing settings item with a string value",
		RunE: func(c *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("--id required")
			}
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			req := &catalogpb.SetSettingRequest{
				Id:    &catalogpb.SettingsItemId{Value: id},
				Value: &catalogpb.SettingsItem_Value{Value: &catalogpb.SettingsItem_Value_StringValue{StringValue: value}},
			}
			_, err = cli.Settings.SetSetting(context.Background(), connect.NewRequest(req))
			return err
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "settings item id")
	cmd.Flags().StringVar(&value, "value", "", "string value")
	return cmd
}
```

- [ ] **Step 5: `probe.go`**

```go
package cmd_cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	stroppypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/stroppy"
)

func probeCmd() *cobra.Command {
	var (
		script, sql, driver, version string
		poolSize, scaleFactor        uint32
	)
	cmd := &cobra.Command{
		Use:   "probe --script <id> [--driver <type>] [--pool-size N] [--scale-factor N]",
		Short: "Server-side `stroppy probe` invocation",
		RunE: func(c *cobra.Command, _ []string) error {
			if script == "" {
				return fmt.Errorf("--script required")
			}
			cli, tenant, err := authedCLI()
			if err != nil {
				return err
			}
			req := &stroppypb.ProbeStroppyConfigRequest{
				TenantId:       &iampb.TenantId{Value: tenant},
				StroppyVersion: version,
				Script:         script,
				Sql:            sql,
				DriverType:     driver,
				PoolSize:       poolSize,
				ScaleFactor:    scaleFactor,
				IncludeHuman:   true,
			}
			resp, err := cli.Stroppy.ProbeStroppyConfig(context.Background(), connect.NewRequest(req))
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&script, "script", "", "workload script id (e.g. tpcc/procs)")
	cmd.Flags().StringVar(&sql, "sql", "", "optional second SQL arg")
	cmd.Flags().StringVar(&driver, "driver", "postgres", "driver type")
	cmd.Flags().StringVar(&version, "version", "", "stroppy binary version (empty = default)")
	cmd.Flags().Uint32Var(&poolSize, "pool-size", 0, "driver pool size")
	cmd.Flags().Uint32Var(&scaleFactor, "scale-factor", 0, "workload scale factor")
	return cmd
}
```

- [ ] **Step 6: `version.go`**

```go
package cmd_cli

import (
	"context"
	"encoding/json"
	"os"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"
)

func versionCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "version", Short: "Stroppy binary version catalog"}
	cmd.AddCommand(versionListCmd(), commitsListCmd())
	return cmd
}

func versionListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List stroppy releases",
		RunE: func(c *cobra.Command, _ []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Stroppy.ListStroppyVersions(context.Background(), connect.NewRequest(&emptypb.Empty{}))
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(resp.Msg.GetVersions())
		},
	}
}

func commitsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "commits",
		Short: "List recent stroppy commits",
		RunE: func(c *cobra.Command, _ []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Stroppy.ListStroppyCommits(context.Background(), connect.NewRequest(&emptypb.Empty{}))
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(resp.Msg.GetCommits())
		},
	}
}
```

- [ ] **Step 7: Register in `root.go`**

```go
cmd.AddCommand(presetCmd())
cmd.AddCommand(packageCmd())
cmd.AddCommand(settingsCmd())
cmd.AddCommand(probeCmd())
cmd.AddCommand(versionCmd())
```

- [ ] **Step 8: Build**

```bash
go build ./...
```
Expected: success.

- [ ] **Step 9: Commit**

```bash
git add cmd/stroppy-cloud/cmd_cli/ internal/sdk/client/client.go
git commit -m "feat(cli,sdk): preset/package/settings/probe/version subcommands + client wrappers"
```

---

## Task 12: Stroppy binary fetch helper for tests

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add target**

Append to `Makefile`:

```makefile
.PHONY: stroppy-bin-fetch
stroppy-bin-fetch: ## Download a stroppy binary into /tmp/stroppy-test-binaries for integration tests
	@mkdir -p /tmp/stroppy-test-binaries
	@if [ ! -x /tmp/stroppy-test-binaries/stroppy-dev ]; then \
	  echo "Provide a real stroppy binary at /tmp/stroppy-test-binaries/stroppy-dev to exercise probe tests."; \
	  echo "Skipping for now."; \
	fi
```

Note: full automation of the probe integration test depends on a real
stroppy binary being available. For CI we shall later mock the runner via
the `STROPPY_PROBE_BIN` env override.

- [ ] **Step 2: Commit**

```bash
git add Makefile
git commit -m "chore: add stroppy-bin-fetch make target placeholder for probe tests"
```

---

## Task 13: Verification + E2E smoke

- [ ] **Step 1: Full build**

```bash
go build ./...
```
Expected: success.

- [ ] **Step 2: Run integration tests**

```bash
go test ./internal/domain/services/iam/... ./internal/domain/services/catalog/... ./internal/domain/services/stroppy/... -count=1
```
Expected: green (stroppy probe tests may be skipped without a real binary — but versions/preview should pass).

- [ ] **Step 3: Manual smoke**

```bash
# Terminal 1
docker compose up -d postgres valkey minio
export JWT_SECRET=$(head -c 32 /dev/urandom | base64)
export INITIAL_ADMIN_PASSWORD="AdminP@ss123!"
export S3_ACCESS_KEY=minioadmin
export S3_SECRET_KEY=minioadmin
make server-run

# Terminal 2
go run ./cmd/stroppy-cloud cli login --server http://localhost:8080
# login as admin@local
go run ./cmd/stroppy-cloud cli tenant create --name MyTenant   # (assuming command exists; defer to plan 04 if not)
go run ./cmd/stroppy-cloud cli context use --tenant <printed id>
go run ./cmd/stroppy-cloud cli preset database list
go run ./cmd/stroppy-cloud cli version list
```

Expected: empty preset list + a populated stroppy version list (if GitHub
reachable).

- [ ] **Step 4: Commit (if anything lingering)**

```bash
git status
```

---

## Acceptance for plan 02

- ✅ Catalog service ships CRUD + clone for DatabasePreset, WorkloadPreset, Package, Settings.
- ✅ Stroppy service serves `ListStroppyVersions`, `ListStroppyCommits` (with valkey JSON cache), `ProbeStroppyConfig`, `PreviewStroppyConfig`.
- ✅ S3 adapter handles PutObject + presigned GET.
- ✅ CLI exposes `preset {database,workload}`, `package`, `settings`, `probe`, `version` subcommands.
- ✅ Integration tests for catalog (presets + packages + settings) and stroppy (versions + preview) green.
- ✅ `cmd_server` wires new services + handlers; example `config.yaml` extended.

## Out of scope for plan 02 (next plan)

- DAG engine, NodeWorker pool, scheduler, recovery sweep (plan 03).
- Test/Suite services (plan 04).
- Agent (plan 05).
- Webhook outbox (plan 06).
- Admin (plan 07).
- Frontend (plan 08).
- Full E2E + perf (plan 09).
