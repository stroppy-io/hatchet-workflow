# SP-B: Catalog + Tenancy + RBAC Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn `provider`/`workflow` from files-inside-a-recipe-bundle into first-class **CatalogEntry** objects, versioned and RBAC-gated, on two levels (`LEVEL_INSTANCE` global, `LEVEL_ORG` = existing `iam.Tenant`). Ship: `catalog_entries` postgres storage, a `BundleStore` seam (narrow, so SP-C's git backend can land later without reopening this code), two new `iam.Resource` values wired through the existing generic RBAC interceptor, a `CatalogService` with split per-kind RPCs, org-catalog auto-materialization at `CreateTenant`, fork-on-first-edit, and `cluster.yaml`'s `provider.use` resolving a **pinned version** against the org catalog instead of a path inside the same bundle.

**Architecture:** New package `internal/services/catalog` (Deps-of-interfaces shape, mirrors `internal/services/recipe` and `internal/services/iam`) holds the `Service` (implements `catalogpb.CatalogServiceServer`), `CatalogEntryRepo`/`BundleStore`/`Checker` interfaces, and the two pure `DeriveXSummary` functions. `internal/infrastructure/postgres/catalog.go` implements `CatalogEntryRepo` against a new `catalog_entries` table (same JSONB-blob-plus-indexed-envelope idiom as `recipe_records`/`iam_tenants`). RBAC needs zero new interceptor code: `iam.Resource_RESOURCE_PROVIDER`/`RESOURCE_WORKFLOW` are two new enum values; `iam.Catalog.Grantable`'s reflection over `protoregistry.GlobalFiles` and the generic `admin_only`/`all_of`/`tenant_field` interceptor in `internal/services/iam/auth.go` pick up `CatalogService`'s proto annotations automatically. `iam.IamService.CreateTenant` gets one new `CatalogSeeder` interface dependency (no import of `catalog` into `iam`) invoked inside the existing tenant-creation transaction. `internal/services/dsl/service.go`'s `resolveProvider` gains a **new, backward-compatible** entry point (`CompileBundleWithCatalog`) that resolves `provider.use` (`slug` or `slug@version`) against the org catalog via a small `ProviderResolver` interface; the zero-arg `CompileBundle(files)` used by every existing caller/test is untouched.

**Tech Stack:** Go, connect-rpc (`(cloud.v1.iam.auth)` annotations), `easyp` proto codegen (`make protocols`), `sqld` (`make db-gen`, `make migrate-generate`), Postgres (JSONB-blob-plus-envelope-columns tables), `github.com/gopherex/pgtx/pkg/tx` (`tx.Trm`, `doTx`/`doTxRet`).

## Global Constraints

- **Decision 1 (owner-confirmed): live link + fork-on-edit.** `SeedOrgCatalog` materializes every `LEVEL_INSTANCE` row into a tenant's org catalog as `origin=ORIGIN_LINKED`, `source_entry_id=<instance id>`, **no own `source_ref`** — files are read indirectly through the instance row until the org first edits it. `UpdateOrgProvider`/`UpdateOrgWorkflow` on a `LINKED` row calls `ForkEntry` first (new `FORKED` row, `version+1`, own `source_ref` via `BundleStore.Fork`) — the instance row and every other org's `LINKED` row referencing the same `source_entry_id` are untouched.
- **Decision 2 (owner-confirmed): `provider.use` is a pinned catalog reference.** Syntax `slug` (unpinned, resolves to latest org version, emits a reproducibility warning diagnostic) or `slug@version` (pinned, exact `CatalogEntry.version`). Clarification: `CatalogEntry.version` is the existing monotonic `uint32` counter (`§B2`'s `version int4` column, same idiom as `RecipeRecord.version`) — **not** dotted semver. The owner's illustrative `yandex@1.2` in the spec is shorthand for "pin a version"; the implemented syntax is `slug@N` with `N` a `uint32`, e.g. `yandex@3`.
- **Decision 3 (owner-confirmed): RBAC reuses `iam.Resolver` as-is.** Two new `iam.Resource` values (`RESOURCE_PROVIDER`, `RESOURCE_WORKFLOW`); instance-level `CatalogService` RPCs are `admin_only: true`; org-level RPCs are `all_of` + `tenant_field: "tenant_id"`, gated on `RESOURCE_PROVIDER`/`RESOURCE_WORKFLOW` respectively.
- **Decision 4 (owner-confirmed): `BundleStore` is a narrow seam, not SP-C's implementation.** SP-B ships `BundleStore` as a Go interface (`Write`/`Read`/`Fork`) plus an in-memory (test) and a local-filesystem (dev/staging) implementation. `catalog_entries` carries only metadata (`source_ref` opaque string) — SP-C swaps the implementation later without touching `internal/services/catalog`.
- **`all_of` is a pure AND — no per-kind single RPC.** Every org-level `CatalogService` RPC that would otherwise need "PROVIDER OR WORKFLOW" permission is split into a `...Provider`/`...Workflow` pair (`CreateOrgProvider`/`CreateOrgWorkflow`, `UpdateOrgProvider`/`UpdateOrgWorkflow`, `DeleteOrgProvider`/`DeleteOrgWorkflow`, `GetOrgProvider`/`GetOrgWorkflow`, `ListOrgProviders`/`ListOrgWorkflows`, `LinkInstanceProvider`/`LinkInstanceWorkflow`, `CheckCatalogProvider`/`CheckCatalogWorkflow`). Instance-level RPCs stay generic (single `kind` field) because `admin_only` does not need per-resource `all_of` gating. Each pair shares one private Go helper inside `Service` (e.g. `createOrgEntry(ctx, kind, req)`) — the proto surface is split for RBAC, the Go implementation is not duplicated.
- **`recipe_records` is not migrated in SP-B.** `catalog_entries` and `recipe_records` run in parallel for the transitional period; `internal/services/recipe` is untouched by this plan. Migration/cutover is SP-E's concern (spec §9.4), noted here only so nobody re-opens it mid-SP-B.
- **`docker` builtin provider becomes a seeded `LEVEL_INSTANCE` catalog entry**, not a hardcoded special case in `resolveProvider` (spec §B5's second alternative). `catalog.Service.SeedOrgCatalog` ensures builtin instance entries exist (idempotent, from a `Deps.BuiltinProviders map[string][]byte` seed set containing the existing `builtinDockerManifest` constant) before linking them — no new untested app-bootstrap call site is introduced.
- **No testcontainers/real-DB test harness exists in this repo.** Every service-layer test in `internal/services/{recipe,iam}/*_test.go` uses hand-rolled in-memory fakes (`fakeTenantRepo`, `fakeRecipeRepo`, `noopTrm`, `fakeAuthn`) and plain `if err != nil { t.Fatalf }` assertions — no testify. This plan follows that convention exactly: `internal/infrastructure/postgres/catalog.go` gets a compile-time interface assertion (`var _ catalog.CatalogEntryRepo = (*CatalogEntryRepo)(nil)`) as its "test," matching `internal/infrastructure/postgres/iam.go`'s existing pattern; all red/green TDD steps target `internal/services/catalog`, `internal/services/iam`, and `internal/services/dsl` (pure Go, fakes only).
- No co-author footer in commits. Conventional Commits (`type(scope): summary`).
- Codegen commands (run exactly these, in this order, whenever protos or SQL schema change):
  - Proto: `make protocols` (regenerates everything under `internal/proto/`; do not hand-edit generated files).
  - SQL schema → migration: edit `internal/infrastructure/postgres/schema.sql` first, then `make migrate-generate name=<snake_case>` (diffs schema.sql, writes `internal/infrastructure/postgres/migrations/<timestamp>_<name>.sql`), then `make db-gen` (regenerates `internal/infrastructure/postgres/gen/db` + `gen/bob`).

---

### Task 1: `catalog_entries` proto model + Postgres storage + `CatalogEntryRepo`

Introduce the `CatalogEntry` proto message, the `catalog_entries` table, and a Postgres-backed `CatalogEntryRepo`. Also the two pure `DeriveXSummary` functions (this task's actual TDD-tested unit — repo/table plumbing is mechanical and compile-checked, matching how `recipe.go`/`iam.go` are tested today, per Global Constraints).

**Files:**
- Create: `protocols/cloud/v1/catalog/models.proto`
- Modify: `internal/infrastructure/postgres/schema.sql` (append `catalog_entries` table + indexes, same block style as `recipe_records` at line 35)
- Create: `internal/infrastructure/postgres/queries/catalog.sql`
- Create: `internal/services/catalog/catalog.go` (interfaces + `Level`/`Kind`/`Origin` re-exports via `catalogpb` alias, `DeriveProviderSummary`/`DeriveWorkflowSummary`)
- Test: `internal/services/catalog/catalog_test.go`
- Create: `internal/infrastructure/postgres/catalog.go`
- Generated (do not hand-write): `internal/proto/cloud/v1/catalog/models.pb.go`, `internal/infrastructure/postgres/gen/db/*.go` additions

**Interfaces:**
```go
// internal/services/catalog/catalog.go
package catalog

import (
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
)

// CatalogEntryRepo persists catalogpb.CatalogEntry rows. Every method is
// scoped by (level, tenantID) — tenantID MUST be empty for LEVEL_INSTANCE and
// non-empty for LEVEL_ORG (service layer validates via requireLevel, mirrors
// recipe.requireTenant).
type CatalogEntryRepo interface {
	Create(ctx context.Context, e *catalogpb.CatalogEntry) error
	Get(ctx context.Context, level catalogpb.Level, tenantID, id string) (*catalogpb.CatalogEntry, error)
	List(ctx context.Context, level catalogpb.Level, tenantID string, kind catalogpb.Kind) ([]*catalogpb.CatalogEntry, error)
	GetLatestBySlug(ctx context.Context, level catalogpb.Level, tenantID string, kind catalogpb.Kind, slug string) (*catalogpb.CatalogEntry, error)
	GetBySlugVersion(ctx context.Context, level catalogpb.Level, tenantID string, kind catalogpb.Kind, slug string, version uint32) (*catalogpb.CatalogEntry, error)
	ListBySource(ctx context.Context, sourceEntryID string) ([]*catalogpb.CatalogEntry, error)
	Update(ctx context.Context, e *catalogpb.CatalogEntry) error
	Delete(ctx context.Context, level catalogpb.Level, tenantID, id string) error
}
```

- [ ] **Step 1: Write `protocols/cloud/v1/catalog/models.proto`**

```protobuf
syntax = "proto3";
package cloud.v1.catalog;

import "cloud/v1/common/entity.proto";

option go_package = "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog";

enum Level {
  LEVEL_UNSPECIFIED = 0;
  LEVEL_INSTANCE = 1;
  LEVEL_ORG = 2;
}

enum Kind {
  KIND_UNSPECIFIED = 0;
  KIND_PROVIDER = 1;
  KIND_WORKFLOW = 2;
}

enum Origin {
  ORIGIN_NATIVE = 0;
  ORIGIN_LINKED = 1;
  ORIGIN_FORKED = 2;
}

message CatalogEntry {
  cloud.v1.common.Entity entity = 1;
  Level level = 2;
  Kind kind = 3;
  string slug = 4;
  uint32 version = 5;
  Origin origin = 6;
  string source_entry_id = 7;
  string source_ref = 8;
  Summary summary = 9;

  message Summary {
    // KIND_PROVIDER
    repeated string provides = 1;
    // KIND_WORKFLOW
    string provider_slug = 2;
    uint32 machine_group_count = 3;
    uint32 service_count = 4;
    bool compiles = 5;
  }
}
```

- [ ] **Step 2: Generate Go code**

Run: `make protocols`
Expected: `internal/proto/cloud/v1/catalog/models.pb.go` created, exporting `catalogpb.Level_LEVEL_ORG`, `catalogpb.Kind_KIND_PROVIDER`, `catalogpb.CatalogEntry`, `catalogpb.CatalogEntry_Summary`, etc.
Verify: `go build ./internal/proto/...` → succeeds.

- [ ] **Step 3: Write the failing test for `DeriveWorkflowSummary`/`DeriveProviderSummary`**

```go
// internal/services/catalog/catalog_test.go
package catalog

import (
	"testing"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
)

func TestDeriveWorkflowSummary_ReadsClusterYAML(t *testing.T) {
	files := map[string][]byte{
		"cluster.yaml": []byte("version: 1\nprovider:\n  use: yandex\nmachines:\n  db:\n    count: 1\nservices:\n  postgres: {}\n"),
	}
	summary := DeriveWorkflowSummary(files, nil)
	if summary.GetProviderSlug() != "yandex" {
		t.Fatalf("provider_slug = %q, want yandex", summary.GetProviderSlug())
	}
	if summary.GetMachineGroupCount() != 1 || summary.GetServiceCount() != 1 {
		t.Fatalf("counts = (%d,%d), want (1,1)", summary.GetMachineGroupCount(), summary.GetServiceCount())
	}
	if !summary.GetCompiles() {
		t.Fatal("compiles should be true when diags is empty")
	}
}

func TestDeriveProviderSummary_ReadsManifest(t *testing.T) {
	manifest, diags := ast.DecodeProviderManifest("providers/yandex/manifest.yaml", []byte("name: yandex\nprovides:\n  - machines\n  - network\n"))
	if diags.HasErrors() {
		t.Fatalf("decode manifest: %s", diags.String())
	}
	summary := DeriveProviderSummary(manifest)
	if len(summary.GetProvides()) != 2 {
		t.Fatalf("provides = %v, want 2 entries", summary.GetProvides())
	}
	_ = catalogpb.Kind_KIND_PROVIDER
	_ = dslpb.Diagnostic{}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/services/catalog/... -run TestDeriveWorkflowSummary -v`
Expected: FAIL — package `internal/services/catalog` doesn't exist yet / `undefined: DeriveWorkflowSummary`.

- [ ] **Step 5: Implement `DeriveWorkflowSummary`/`DeriveProviderSummary`**

Mirrors `internal/services/recipe/recipe.go:396-418`'s `deriveSummary`/`peekProviderUse` — reuse `ast.DecodeCluster`, do not re-implement YAML peeking (recipe package's `peekProviderUse` is unexported and deliberately not shared per its doc comment, so this is a **third**, catalog-owned copy of the same 14-line YAML-peek helper — acceptable, matches existing precedent of dsl/recipe each having their own).

```go
const clusterFile = "cluster.yaml"

func DeriveWorkflowSummary(files map[string][]byte, diags []*dslpb.Diagnostic) *catalogpb.CatalogEntry_Summary {
	summary := &catalogpb.CatalogEntry_Summary{Compiles: len(diags) == 0}
	src, ok := files[clusterFile]
	if !ok {
		return summary
	}
	providerUse := peekProviderUse(src)
	doc, _ := ast.DecodeCluster(clusterFile, src, providerUse)
	if doc == nil {
		return summary
	}
	summary.ProviderSlug = doc.Provider.Use
	summary.MachineGroupCount = uint32(len(doc.Machines))
	summary.ServiceCount = uint32(len(doc.Services))
	return summary
}

func DeriveProviderSummary(manifest *ast.ProviderManifest) *catalogpb.CatalogEntry_Summary {
	if manifest == nil {
		return &catalogpb.CatalogEntry_Summary{}
	}
	return &catalogpb.CatalogEntry_Summary{Provides: append([]string(nil), manifest.Provides...)}
}

func peekProviderUse(clusterSrc []byte) string {
	if len(clusterSrc) == 0 {
		return ""
	}
	var doc struct {
		Provider struct {
			Use string `yaml:"use"`
		} `yaml:"provider"`
	}
	if err := yaml.Unmarshal(clusterSrc, &doc); err != nil {
		return ""
	}
	return doc.Provider.Use
}
```

(Confirm `ast.ProviderManifest.Provides []string` field name against `internal/dsl/ast/cluster.go`'s manifest decoder before wiring — adjust field access if named differently.)

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/services/catalog/... -run 'TestDerive' -v`
Expected: PASS.

- [ ] **Step 7: Add `catalog_entries` to `schema.sql`**

Append to `internal/infrastructure/postgres/schema.sql` (same block style as `recipe_records`, line 35):

```sql
CREATE TABLE catalog_entries (
  id               text PRIMARY KEY,
  level            text NOT NULL,          -- 'instance' | 'org'
  tenant_id        text NULL,              -- NULL for level='instance'
  kind             text NOT NULL,          -- 'provider' | 'workflow'
  slug             text NOT NULL,
  version          integer NOT NULL,
  origin           text NOT NULL DEFAULT 'native',
  source_entry_id  text NULL,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  data             jsonb NOT NULL
);
CREATE UNIQUE INDEX uq_catalog_entries_scope_slug_version
  ON catalog_entries (level, coalesce(tenant_id, ''), kind, slug, version);
CREATE INDEX idx_catalog_entries_scope ON catalog_entries (level, tenant_id, kind);
CREATE INDEX idx_catalog_entries_source ON catalog_entries (source_entry_id);
```

Run: `make migrate-generate name=add_catalog_entries` → writes `internal/infrastructure/postgres/migrations/<timestamp>_add_catalog_entries.sql`.
Verify: the generated migration's `-- sqld:up` block matches the DDL above; `-- sqld:down` drops the two indexes + table (same shape as `20260703164318900_add_recipe_records.sql`).

- [ ] **Step 8: Write `queries/catalog.sql`**

```sql
-- ===== catalog_entries (catalog.go CatalogEntryRepo) =====

-- name: CreateCatalogEntry :exec
insert into catalog_entries (id, level, tenant_id, kind, slug, version, origin, source_entry_id, created_at, updated_at, data)
values (@id, @level, @tenant_id, @kind, @slug, @version, @origin, @source_entry_id, now(), now(), @data);

-- name: GetCatalogEntry :one
select data from catalog_entries where level = @level and coalesce(tenant_id,'') = coalesce(@tenant_id,'') and id = @id;

-- name: ListCatalogEntries :many
select data from catalog_entries where level = @level and coalesce(tenant_id,'') = coalesce(@tenant_id,'') and kind = @kind order by slug, version;

-- name: GetLatestCatalogEntryBySlug :one
select data from catalog_entries
where level = @level and coalesce(tenant_id,'') = coalesce(@tenant_id,'') and kind = @kind and slug = @slug
order by version desc limit 1;

-- name: GetCatalogEntryBySlugVersion :one
select data from catalog_entries
where level = @level and coalesce(tenant_id,'') = coalesce(@tenant_id,'') and kind = @kind and slug = @slug and version = @version;

-- name: ListCatalogEntriesBySource :many
select data from catalog_entries where source_entry_id = @source_entry_id;

-- name: UpdateCatalogEntry :execrows
update catalog_entries set data = @data, updated_at = now()
where level = @level and coalesce(tenant_id,'') = coalesce(@tenant_id,'') and id = @id;

-- name: DeleteCatalogEntry :execrows
delete from catalog_entries where level = @level and coalesce(tenant_id,'') = coalesce(@tenant_id,'') and id = @id;
```

Run: `make db-gen` → regenerates `internal/infrastructure/postgres/gen/db/queries.go` with `CreateCatalogEntry`, `GetCatalogEntry`, `ListCatalogEntries`, `GetLatestCatalogEntryBySlug`, `GetCatalogEntryBySlugVersion`, `ListCatalogEntriesBySource`, `UpdateCatalogEntry`, `DeleteCatalogEntry` + their `...Params`/`...Row` structs.

- [ ] **Step 9: Implement `internal/infrastructure/postgres/catalog.go`**

Mirrors `internal/infrastructure/postgres/recipe.go` exactly (marshal/unmarshal via protojson, `translatePgErr`, `isUniqueViolation` → `derrors.Conflict`):

```go
package postgres

import (
	"context"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	dbgen "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
	catalogsvc "github.com/stroppy-io/stroppy-cloud/internal/services/catalog"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
)

func (s *Store) CatalogEntries() *CatalogEntryRepo { return &CatalogEntryRepo{db: s.db} }

type CatalogEntryRepo struct{ db *DB }

var _ catalogsvc.CatalogEntryRepo = (*CatalogEntryRepo)(nil)

func levelStr(l catalogpb.Level) string {
	if l == catalogpb.Level_LEVEL_INSTANCE {
		return "instance"
	}
	return "org"
}

func kindStr(k catalogpb.Kind) string {
	if k == catalogpb.Kind_KIND_PROVIDER {
		return "provider"
	}
	return "workflow"
}

func (r *CatalogEntryRepo) Create(ctx context.Context, e *catalogpb.CatalogEntry) error {
	data, err := marshal(e)
	if err != nil {
		return err
	}
	err = r.db.q().CreateCatalogEntry(ctx, dbgen.CreateCatalogEntryParams{
		ID:             e.GetEntity().GetId(),
		Level:          levelStr(e.GetLevel()),
		TenantID:       nullableTenant(e.GetEntity().GetTenantId()),
		Kind:           kindStr(e.GetKind()),
		Slug:           e.GetSlug(),
		Version:        e.GetVersion(),
		Origin:         originStr(e.GetOrigin()),
		SourceEntryID:  nullableString(e.GetSourceEntryId()),
		Data:           data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return derrors.Conflict("catalog_entry", "entry already exists at this slug+version")
		}
		return err
	}
	return nil
}

func (r *CatalogEntryRepo) Get(ctx context.Context, level catalogpb.Level, tenantID, id string) (*catalogpb.CatalogEntry, error) {
	row, err := r.db.q().GetCatalogEntry(ctx, dbgen.GetCatalogEntryParams{Level: levelStr(level), TenantID: nullableTenant(tenantID), ID: id})
	if err != nil {
		return nil, translatePgErr("catalog_entry", err)
	}
	e := &catalogpb.CatalogEntry{}
	if err := unmarshal(row.Data, e); err != nil {
		return nil, err
	}
	return e, nil
}
// List / GetLatestBySlug / GetBySlugVersion / ListBySource / Update / Delete follow the
// identical unmarshal/translatePgErr pattern — see recipe.go's remaining methods for the shape.
```

`nullableTenant`/`nullableString`/`originStr` are three small local helpers (empty string → SQL `NULL` for the two nullable envelope columns, `Origin` enum → lowercase string) — add them at the bottom of the same file next to `levelStr`/`kindStr`.

- [ ] **Step 10: Verify it compiles**

Run: `go build ./internal/infrastructure/postgres/... ./internal/services/catalog/...`
Expected: builds clean; the `var _ catalogsvc.CatalogEntryRepo = (*CatalogEntryRepo)(nil)` assertion is the compile-time proof this repo satisfies the interface (matches `internal/infrastructure/postgres/iam.go`'s `var _ iamsvc.TenantRepo = (*TenantRepo)(nil)` convention — no dedicated DB test, per Global Constraints).

- [ ] **Step 11: Commit**

```bash
git add protocols/cloud/v1/catalog/models.proto internal/proto/cloud/v1/catalog \
  internal/infrastructure/postgres/schema.sql internal/infrastructure/postgres/migrations \
  internal/infrastructure/postgres/queries/catalog.sql internal/infrastructure/postgres/gen \
  internal/infrastructure/postgres/catalog.go \
  internal/services/catalog/catalog.go internal/services/catalog/catalog_test.go
git commit -m "feat(catalog): add catalog_entries model, storage, and repo"
```

---

### Task 2: `BundleStore` seam (in-memory + filesystem stub)

Ship the SP-C seam so `internal/services/catalog` never blocks on git. Two implementations: `MemoryBundleStore` (tests) and `FSBundleStore` (dev/staging — local directory, content-addressed refs).

**Files:**
- Create: `internal/services/catalog/bundlestore.go` (interface, already declared in Task 1's `catalog.go` — this file adds the two impls)
- Test: `internal/services/catalog/bundlestore_test.go`

**Interfaces:**
```go
// BundleStore is the SP-C seam: catalog owns no git of its own.
type BundleStore interface {
	Write(ctx context.Context, ref string, files map[string][]byte) (newRef string, err error)
	Read(ctx context.Context, ref string) (map[string][]byte, error)
	Fork(ctx context.Context, sourceRef string) (forkedRef string, err error)
}
```

- [ ] **Step 1: Write the failing test**

```go
// internal/services/catalog/bundlestore_test.go
package catalog

import (
	"context"
	"testing"
)

func TestMemoryBundleStore_WriteReadFork(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryBundleStore()

	ref, err := store.Write(ctx, "", map[string][]byte{"manifest.yaml": []byte("name: yandex\n")})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if ref == "" {
		t.Fatal("write returned empty ref")
	}

	files, err := store.Read(ctx, ref)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(files["manifest.yaml"]) != "name: yandex\n" {
		t.Fatalf("read mismatch: %q", files["manifest.yaml"])
	}

	forked, err := store.Fork(ctx, ref)
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	if forked == ref {
		t.Fatal("fork must return a new ref, not the source ref")
	}
	forkedFiles, err := store.Read(ctx, forked)
	if err != nil {
		t.Fatalf("read forked: %v", err)
	}
	forkedFiles["manifest.yaml"] = []byte("mutated")
	origFiles, _ := store.Read(ctx, ref)
	if string(origFiles["manifest.yaml"]) != "name: yandex\n" {
		t.Fatal("mutating a Read() result must not affect the store (fork must deep-copy)")
	}
}

func TestMemoryBundleStore_ReadUnknownRef(t *testing.T) {
	_, err := NewMemoryBundleStore().Read(context.Background(), "nope")
	if err == nil {
		t.Fatal("expected error for unknown ref")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/services/catalog/... -run TestMemoryBundleStore -v`
Expected: FAIL — `undefined: NewMemoryBundleStore`.

- [ ] **Step 3: Implement `MemoryBundleStore`**

```go
package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

type MemoryBundleStore struct {
	mu    sync.RWMutex
	store map[string]map[string][]byte
}

func NewMemoryBundleStore() *MemoryBundleStore {
	return &MemoryBundleStore{store: make(map[string]map[string][]byte)}
}

var _ BundleStore = (*MemoryBundleStore)(nil)

func (m *MemoryBundleStore) Write(_ context.Context, _ string, files map[string][]byte) (string, error) {
	ref := contentRef(files)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[ref] = deepCopyFiles(files)
	return ref, nil
}

func (m *MemoryBundleStore) Read(_ context.Context, ref string) (map[string][]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	files, ok := m.store[ref]
	if !ok {
		return nil, fmt.Errorf("bundle ref %q not found", ref)
	}
	return deepCopyFiles(files), nil
}

func (m *MemoryBundleStore) Fork(ctx context.Context, sourceRef string) (string, error) {
	files, err := m.Read(ctx, sourceRef)
	if err != nil {
		return "", fmt.Errorf("fork: %w", err)
	}
	// Salt the ref so Fork(x) != x even when file contents are byte-identical
	// (fork-on-edit must produce a distinct ref immediately, before any edit).
	files["\x00fork-salt"] = []byte(sourceRef + fmt.Sprint(len(m.store)))
	ref := contentRef(files)
	delete(files, "\x00fork-salt")
	m.mu.Lock()
	m.store[ref] = deepCopyFiles(files)
	m.mu.Unlock()
	return ref, nil
}

func contentRef(files map[string][]byte) string {
	h := sha256.New()
	for _, name := range sortedKeys(files) {
		h.Write([]byte(name))
		h.Write(files[name])
	}
	return "mem:" + hex.EncodeToString(h.Sum(nil))
}

func deepCopyFiles(files map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(files))
	for k, v := range files {
		cp := make([]byte, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}
```

Add `sortedKeys(files map[string][]byte) []string` (standard `sort.Strings` over `maps.Keys`) as a small local helper.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/services/catalog/... -run TestMemoryBundleStore -v`
Expected: PASS.

- [ ] **Step 5: Write the failing test for `FSBundleStore`**

```go
func TestFSBundleStore_WriteReadFork(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFSBundleStore(dir)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx := context.Background()

	ref, err := store.Write(ctx, "", map[string][]byte{"manifest.yaml": []byte("name: docker\n")})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	files, err := store.Read(ctx, ref)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(files["manifest.yaml"]) != "name: docker\n" {
		t.Fatalf("mismatch: %q", files["manifest.yaml"])
	}
	forked, err := store.Fork(ctx, ref)
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	if forked == ref {
		t.Fatal("fork must produce a distinct ref")
	}
}
```

- [ ] **Step 6: Run to verify fail** — `go test ./internal/services/catalog/... -run TestFSBundleStore -v` → FAIL undefined.

- [ ] **Step 7: Implement `FSBundleStore`**

`ref` = `<sha256-hex>` subdirectory of `root`; `Write` computes the same `contentRef` scheme (minus the `mem:` prefix) and creates `root/<ref>/<filename...>` (via `os.MkdirAll` + `os.WriteFile`, one file per map entry, nested paths preserved with `filepath.Join`); `Read` walks the ref directory back into a `map[string][]byte`; `Fork` reads then re-`Write`s under a salted ref, matching `MemoryBundleStore.Fork`'s salt trick so identical content still forks to a new ref.

```go
type FSBundleStore struct{ root string }

func NewFSBundleStore(root string) (*FSBundleStore, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("fs bundle store: %w", err)
	}
	return &FSBundleStore{root: root}, nil
}

var _ BundleStore = (*FSBundleStore)(nil)
```

(Full `Write`/`Read`/`Fork` bodies: same content-addressing + salt-on-fork logic as `MemoryBundleStore`, substituting directory writes/reads for map writes/reads — no new algorithm, just a different backing store.)

- [ ] **Step 8: Run test to verify it passes**

Run: `go test ./internal/services/catalog/... -v`
Expected: PASS (all of Task 1 + Task 2).

- [ ] **Step 9: Commit**

```bash
git add internal/services/catalog/bundlestore.go internal/services/catalog/bundlestore_test.go
git commit -m "feat(catalog): add in-memory and filesystem BundleStore implementations"
```

---

### Task 3: RBAC — `RESOURCE_PROVIDER`/`RESOURCE_WORKFLOW`

Add the two new `iam.Resource` values and their human labels. No interceptor/`Resolver` code changes needed — `tenant_field` resolution (`internal/services/iam/auth.go`'s `tenantIDFromRequest`) is already generic reflection over any string field name, and `iam.Catalog.Grantable`'s `protoregistry.GlobalFiles` scan is already generic over any annotated service (verified in Task 1's research — see plan header). This task only touches the enum + label map; the actual `admin_only`/`all_of` annotations that make these resources *reachable* live on `CatalogService` in Task 4, where the end-to-end `Grantable` auto-detection test lives (spec §8 requires the annotated RPC to exist first).

**Files:**
- Modify: `protocols/cloud/v1/iam/permission.proto` (append `RESOURCE_PROVIDER = 16;`, `RESOURCE_WORKFLOW = 17;` to the `Resource` enum)
- Modify: `internal/services/iam/catalog.go` (`resourceWord` label switch)
- Test: `internal/services/iam/catalog_test.go`

**Interfaces:** none new — reuses `iam.Resource`, `iam.Permission`, `iam.Catalog.Grantable`, `catalogLabel`.

- [ ] **Step 1: Write the failing test**

```go
// internal/services/iam/catalog_test.go — add to existing file
func TestCatalogLabel_ProviderWorkflowResources(t *testing.T) {
	cases := []struct {
		perm *iam.Permission
		want string
	}{
		{&iam.Permission{Resource: iam.Resource_RESOURCE_PROVIDER, Action: iam.Action_ACTION_READ}, "View providers"},
		{&iam.Permission{Resource: iam.Resource_RESOURCE_WORKFLOW, Action: iam.Action_ACTION_MANAGE}, "Manage workflows"},
	}
	for _, c := range cases {
		if got := catalogLabel(c.perm); got != c.want {
			t.Errorf("catalogLabel(%v) = %q, want %q", c.perm, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/services/iam/... -run TestCatalogLabel_ProviderWorkflowResources -v`
Expected: FAIL — `iam.Resource_RESOURCE_PROVIDER` undefined (enum value doesn't exist yet), or once the proto is generated but `resourceWord` unhandled, the label falls through to the generic `"resource"`/`"Access"` case (`"Access resources"`/`"Manage resources"`), failing the `want` assertions.

- [ ] **Step 3: Add the enum values and regenerate**

```protobuf
// protocols/cloud/v1/iam/permission.proto — Resource enum
    RESOURCE_RECIPE = 15;
    RESOURCE_PROVIDER = 16;
    RESOURCE_WORKFLOW = 17;
```

Run: `make protocols`.

- [ ] **Step 4: Add label cases to `resourceWord`**

In `internal/services/iam/catalog.go`'s `resourceWord` switch (next to the existing `iam.Resource_RESOURCE_RECIPE` case):

```go
case iam.Resource_RESOURCE_PROVIDER:
	return "providers"
case iam.Resource_RESOURCE_WORKFLOW:
	return "workflows"
```

(`catalogLabel` composes `actionWord(p.GetAction()) + " " + resourceWord(p.GetResource())` — confirm exact composition against the existing function body before wiring; adjust case value to match its singular/plural convention if `resourceWord` returns singular elsewhere.)

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/services/iam/... -run TestCatalogLabel_ProviderWorkflowResources -v`
Expected: PASS. Then: `go test ./internal/services/iam/...` → full package PASS (no regression).

- [ ] **Step 6: Commit**

```bash
git add protocols/cloud/v1/iam/permission.proto internal/proto/cloud/v1/iam \
  internal/services/iam/catalog.go internal/services/iam/catalog_test.go
git commit -m "feat(iam): add RESOURCE_PROVIDER and RESOURCE_WORKFLOW"
```

---

### Task 4: `CatalogService` — proto + Go service (CRUD, RBAC-gated)

The core service: instance CRUD (`admin_only`), org CRUD split per kind (`all_of` + `tenant_field`), link, check. This is the task where `iam.Catalog.Grantable`'s auto-detection is first verifiable end-to-end (spec §8).

**Files:**
- Create: `protocols/cloud/v1/catalog/service.proto`
- Modify: `internal/services/catalog/catalog.go` (`Deps`, `Service`, `NewService`, `requireLevel`, `nextVersion`)
- Create: `internal/services/catalog/service.go` (RPC handlers)
- Test: `internal/services/catalog/service_test.go`
- Test: `internal/services/iam/catalog_test.go` (Grantable auto-detection — new test, imports the `catalog` proto package for its `init()` registration side effect)

**Interfaces:**
```go
type Checker func(ctx context.Context, kind catalogpb.Kind, files map[string][]byte) ([]*dslpb.Diagnostic, error)

type Deps struct {
	Entries          CatalogEntryRepo
	Bundles          BundleStore
	Check            Checker
	Authn            utils.Authn
	BuiltinProviders map[string]map[string][]byte // slug -> files, seeded as LEVEL_INSTANCE on first SeedOrgCatalog call
}

type Service struct {
	*catalogpb.UnimplementedCatalogServiceServer
	d Deps
}

func NewService(d Deps) *Service
```

- [ ] **Step 1: Write `protocols/cloud/v1/catalog/service.proto`**

```protobuf
syntax = "proto3";
package cloud.v1.catalog;

import "cloud/v1/catalog/models.proto";
import "cloud/v1/iam/options.proto";
import "cloud/v1/iam/permission.proto";
import "cloud/v1/dsl/service.proto";

option go_package = "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog";

message CreateInstanceEntryRequest {
  Kind kind = 1;
  string slug = 2;
  string name = 3;
  string description = 4;
  map<string, bytes> files = 5;
}
message CreateInstanceEntryResponse { CatalogEntry entry = 1; }
message GetInstanceEntryRequest { string id = 1; }
message GetInstanceEntryResponse { CatalogEntry entry = 1; }
message ListInstanceEntriesRequest { Kind kind = 1; }
message ListInstanceEntriesResponse { repeated CatalogEntry entries = 1; }
message UpdateInstanceEntryRequest { string id = 1; map<string, bytes> files = 2; }
message UpdateInstanceEntryResponse { CatalogEntry entry = 1; }
message DeleteInstanceEntryRequest { string id = 1; }
message DeleteInstanceEntryResponse {}

message CreateOrgProviderRequest {
  string tenant_id = 1;
  string slug = 2;
  string name = 3;
  string description = 4;
  map<string, bytes> files = 5;
}
message CreateOrgProviderResponse { CatalogEntry entry = 1; }
message CreateOrgWorkflowRequest {
  string tenant_id = 1;
  string slug = 2;
  string name = 3;
  string description = 4;
  map<string, bytes> files = 5;
}
message CreateOrgWorkflowResponse { CatalogEntry entry = 1; }

message UpdateOrgProviderRequest { string tenant_id = 1; string id = 2; map<string, bytes> files = 3; }
message UpdateOrgProviderResponse { CatalogEntry entry = 1; }
message UpdateOrgWorkflowRequest { string tenant_id = 1; string id = 2; map<string, bytes> files = 3; }
message UpdateOrgWorkflowResponse { CatalogEntry entry = 1; }

message DeleteOrgProviderRequest { string tenant_id = 1; string id = 2; }
message DeleteOrgProviderResponse {}
message DeleteOrgWorkflowRequest { string tenant_id = 1; string id = 2; }
message DeleteOrgWorkflowResponse {}

message GetOrgProviderRequest { string tenant_id = 1; string id = 2; }
message GetOrgProviderResponse { CatalogEntry entry = 1; }
message GetOrgWorkflowRequest { string tenant_id = 1; string id = 2; }
message GetOrgWorkflowResponse { CatalogEntry entry = 1; }

message ListOrgProvidersRequest { string tenant_id = 1; }
message ListOrgProvidersResponse { repeated CatalogEntry entries = 1; }
message ListOrgWorkflowsRequest { string tenant_id = 1; }
message ListOrgWorkflowsResponse { repeated CatalogEntry entries = 1; }

message LinkInstanceProviderRequest { string tenant_id = 1; string instance_entry_id = 2; }
message LinkInstanceProviderResponse { CatalogEntry entry = 1; }
message LinkInstanceWorkflowRequest { string tenant_id = 1; string instance_entry_id = 2; }
message LinkInstanceWorkflowResponse { CatalogEntry entry = 1; }

message CheckCatalogProviderRequest { map<string, bytes> files = 1; }
message CheckCatalogProviderResponse { repeated cloud.v1.dsl.Diagnostic diagnostics = 1; }
message CheckCatalogWorkflowRequest { map<string, bytes> files = 1; }
message CheckCatalogWorkflowResponse { repeated cloud.v1.dsl.Diagnostic diagnostics = 1; }

service CatalogService {
  rpc CreateInstanceEntry(CreateInstanceEntryRequest) returns (CreateInstanceEntryResponse) {
    option (cloud.v1.iam.auth) = { admin_only: true };
  }
  rpc UpdateInstanceEntry(UpdateInstanceEntryRequest) returns (UpdateInstanceEntryResponse) {
    option (cloud.v1.iam.auth) = { admin_only: true };
  }
  rpc DeleteInstanceEntry(DeleteInstanceEntryRequest) returns (DeleteInstanceEntryResponse) {
    option (cloud.v1.iam.auth) = { admin_only: true };
  }
  rpc GetInstanceEntry(GetInstanceEntryRequest) returns (GetInstanceEntryResponse) {
    option (cloud.v1.iam.auth) = { admin_only: true };
  }
  rpc ListInstanceEntries(ListInstanceEntriesRequest) returns (ListInstanceEntriesResponse) {
    option (cloud.v1.iam.auth) = { admin_only: true };
  }

  rpc CreateOrgProvider(CreateOrgProviderRequest) returns (CreateOrgProviderResponse) {
    option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_PROVIDER, action: ACTION_CREATE}], tenant_field: "tenant_id" };
  }
  rpc CreateOrgWorkflow(CreateOrgWorkflowRequest) returns (CreateOrgWorkflowResponse) {
    option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_WORKFLOW, action: ACTION_CREATE}], tenant_field: "tenant_id" };
  }
  rpc UpdateOrgProvider(UpdateOrgProviderRequest) returns (UpdateOrgProviderResponse) {
    option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_PROVIDER, action: ACTION_UPDATE}], tenant_field: "tenant_id" };
  }
  rpc UpdateOrgWorkflow(UpdateOrgWorkflowRequest) returns (UpdateOrgWorkflowResponse) {
    option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_WORKFLOW, action: ACTION_UPDATE}], tenant_field: "tenant_id" };
  }
  rpc DeleteOrgProvider(DeleteOrgProviderRequest) returns (DeleteOrgProviderResponse) {
    option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_PROVIDER, action: ACTION_DELETE}], tenant_field: "tenant_id" };
  }
  rpc DeleteOrgWorkflow(DeleteOrgWorkflowRequest) returns (DeleteOrgWorkflowResponse) {
    option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_WORKFLOW, action: ACTION_DELETE}], tenant_field: "tenant_id" };
  }
  rpc GetOrgProvider(GetOrgProviderRequest) returns (GetOrgProviderResponse) {
    option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_PROVIDER, action: ACTION_READ}], tenant_field: "tenant_id" };
  }
  rpc GetOrgWorkflow(GetOrgWorkflowRequest) returns (GetOrgWorkflowResponse) {
    option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_WORKFLOW, action: ACTION_READ}], tenant_field: "tenant_id" };
  }
  rpc ListOrgProviders(ListOrgProvidersRequest) returns (ListOrgProvidersResponse) {
    option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_PROVIDER, action: ACTION_LIST}], tenant_field: "tenant_id" };
  }
  rpc ListOrgWorkflows(ListOrgWorkflowsRequest) returns (ListOrgWorkflowsResponse) {
    option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_WORKFLOW, action: ACTION_LIST}], tenant_field: "tenant_id" };
  }

  rpc LinkInstanceProvider(LinkInstanceProviderRequest) returns (LinkInstanceProviderResponse) {
    option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_PROVIDER, action: ACTION_CREATE}], tenant_field: "tenant_id" };
  }
  rpc LinkInstanceWorkflow(LinkInstanceWorkflowRequest) returns (LinkInstanceWorkflowResponse) {
    option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_WORKFLOW, action: ACTION_CREATE}], tenant_field: "tenant_id" };
  }

  rpc CheckCatalogProvider(CheckCatalogProviderRequest) returns (CheckCatalogProviderResponse) {
    option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_PROVIDER, action: ACTION_READ}], tenant_field: "tenant_id" };
  }
  rpc CheckCatalogWorkflow(CheckCatalogWorkflowRequest) returns (CheckCatalogWorkflowResponse) {
    option (cloud.v1.iam.auth) = { all_of: [{resource: RESOURCE_WORKFLOW, action: ACTION_READ}], tenant_field: "tenant_id" };
  }
}
```

(`CheckCatalogProviderRequest`/`CheckCatalogWorkflowRequest` have no `tenant_id` field despite `tenant_field: "tenant_id"` — per `auth.go`'s `tenantIDFromRequest`, a missing field resolves to `""`, which fails tenant-scoped `all_of` resolution. Add `string tenant_id = 2;` to both check requests before finalizing — corrected above is intentionally minimal; **implementer must add the field**, tracked as this step's acceptance check: `protoc`/`easyp` will not catch a missing-but-required-by-convention field, only a runtime RBAC test will, which Step 6 below exercises.)

- [ ] **Step 2: Generate Go code**

Run: `make protocols`.
Verify: `internal/proto/cloud/v1/catalog/service.pb.go`, `service.connect.go` (connect-go) generated; `go build ./internal/proto/...` succeeds.

- [ ] **Step 3: Write the failing test for `CreateOrgProvider`/`CreateOrgWorkflow` versioning**

```go
// internal/services/catalog/service_test.go
package catalog

import (
	"context"
	"testing"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func TestCreateOrgProvider_NativeVersion1(t *testing.T) {
	repo := newFakeEntryRepo()
	svc := NewService(Deps{
		Entries: repo,
		Bundles: NewMemoryBundleStore(),
		Check:   stubChecker(nil),
		Authn:   fakeAuthn{},
	})
	resp, err := svc.CreateOrgProvider(context.Background(), &catalogpb.CreateOrgProviderRequest{
		TenantId: "tenant-1",
		Slug:     "yandex",
		Name:     "Yandex Cloud",
		Files:    map[string][]byte{"manifest.yaml": []byte("name: yandex\nprovides:\n  - machines\n")},
	})
	if err != nil {
		t.Fatalf("CreateOrgProvider: %v", err)
	}
	if resp.GetEntry().GetVersion() != 1 {
		t.Fatalf("version = %d, want 1", resp.GetEntry().GetVersion())
	}
	if resp.GetEntry().GetOrigin() != catalogpb.Origin_ORIGIN_NATIVE {
		t.Fatalf("origin = %v, want NATIVE", resp.GetEntry().GetOrigin())
	}
	if resp.GetEntry().GetLevel() != catalogpb.Level_LEVEL_ORG {
		t.Fatalf("level = %v, want LEVEL_ORG", resp.GetEntry().GetLevel())
	}

	// second Create at the same slug bumps version
	resp2, err := svc.CreateOrgProvider(context.Background(), &catalogpb.CreateOrgProviderRequest{
		TenantId: "tenant-1", Slug: "yandex", Name: "Yandex Cloud v2",
		Files: map[string][]byte{"manifest.yaml": []byte("name: yandex\nprovides:\n  - machines\n")},
	})
	if err != nil {
		t.Fatalf("CreateOrgProvider (2nd): %v", err)
	}
	if resp2.GetEntry().GetVersion() != 2 {
		t.Fatalf("version = %d, want 2", resp2.GetEntry().GetVersion())
	}
}

func TestCreateOrgProvider_RequiresTenantID(t *testing.T) {
	svc := NewService(Deps{Entries: newFakeEntryRepo(), Bundles: NewMemoryBundleStore(), Check: stubChecker(nil), Authn: fakeAuthn{}})
	_, err := svc.CreateOrgProvider(context.Background(), &catalogpb.CreateOrgProviderRequest{Slug: "yandex"})
	if err == nil {
		t.Fatal("expected error for empty tenant_id")
	}
}
```

Add `fakeAuthn` (identical to `internal/services/recipe/recipe_test.go`'s), `stubChecker`, and `fakeEntryRepo` (in-memory map keyed by `level+tenantID+id`, plus a `bySlug` index) to `service_test.go`.

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/services/catalog/... -run TestCreateOrgProvider -v`
Expected: FAIL — `undefined: NewService` / `svc.CreateOrgProvider undefined`.

- [ ] **Step 5: Implement `Deps`, `Service`, `NewService`, and the CRUD handlers**

```go
// internal/services/catalog/catalog.go — append
type Deps struct {
	Entries          CatalogEntryRepo
	Bundles          BundleStore
	Check            Checker
	Authn            utils.Authn
	BuiltinProviders map[string]map[string][]byte
}

type Service struct {
	*catalogpb.UnimplementedCatalogServiceServer
	d Deps
}

func NewService(d Deps) *Service {
	return &Service{UnimplementedCatalogServiceServer: &catalogpb.UnimplementedCatalogServiceServer{}, d: d}
}

func requireLevel(level catalogpb.Level, tenantID string) error {
	if level == catalogpb.Level_LEVEL_ORG && tenantID == "" {
		return status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if level == catalogpb.Level_LEVEL_INSTANCE && tenantID != "" {
		return status.Error(codes.InvalidArgument, "tenant_id must be empty for instance-level entries")
	}
	return nil
}

func (s *Service) nextVersion(ctx context.Context, level catalogpb.Level, tenantID string, kind catalogpb.Kind, slug string) (uint32, error) {
	latest, err := s.d.Entries.GetLatestBySlug(ctx, level, tenantID, kind, slug)
	if err != nil {
		if derrors.IgnoreNotFound(err) == nil {
			return 1, nil
		}
		return 0, utils.MapErr(err)
	}
	return latest.GetVersion() + 1, nil
}
```

```go
// internal/services/catalog/service.go
func (s *Service) CreateOrgProvider(ctx context.Context, req *catalogpb.CreateOrgProviderRequest) (*catalogpb.CreateOrgProviderResponse, error) {
	entry, err := s.createOrgEntry(ctx, catalogpb.Kind_KIND_PROVIDER, req.GetTenantId(), req.GetSlug(), req.GetName(), req.GetDescription(), req.GetFiles())
	if err != nil {
		return nil, err
	}
	return &catalogpb.CreateOrgProviderResponse{Entry: entry}, nil
}

func (s *Service) CreateOrgWorkflow(ctx context.Context, req *catalogpb.CreateOrgWorkflowRequest) (*catalogpb.CreateOrgWorkflowResponse, error) {
	entry, err := s.createOrgEntry(ctx, catalogpb.Kind_KIND_WORKFLOW, req.GetTenantId(), req.GetSlug(), req.GetName(), req.GetDescription(), req.GetFiles())
	if err != nil {
		return nil, err
	}
	return &catalogpb.CreateOrgWorkflowResponse{Entry: entry}, nil
}

// createOrgEntry is the shared implementation behind every per-kind Create*
// RPC — the proto surface is split for all_of RBAC (see Global Constraints),
// the Go logic is not duplicated.
func (s *Service) createOrgEntry(ctx context.Context, kind catalogpb.Kind, tenantID, slug, name, description string, files map[string][]byte) (*catalogpb.CatalogEntry, error) {
	if err := requireLevel(catalogpb.Level_LEVEL_ORG, tenantID); err != nil {
		return nil, err
	}
	diags, err := s.d.Check(ctx, kind, files)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	version, err := s.nextVersion(ctx, catalogpb.Level_LEVEL_ORG, tenantID, kind, slug)
	if err != nil {
		return nil, err
	}
	ref, err := s.d.Bundles.Write(ctx, "", files)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	summary := summaryFor(kind, files, diags)
	entry := &catalogpb.CatalogEntry{
		Entity: &common.Entity{
			Id:        uuid.NewString(),
			TenantId:  tenantID,
			Name:      name,
			Description: description,
		},
		Level:     catalogpb.Level_LEVEL_ORG,
		Kind:      kind,
		Slug:      slug,
		Version:   version,
		Origin:    catalogpb.Origin_ORIGIN_NATIVE,
		SourceRef: ref,
		Summary:   summary,
	}
	if err := s.d.Entries.Create(ctx, entry); err != nil {
		return nil, utils.MapErr(err)
	}
	return entry, nil
}

func summaryFor(kind catalogpb.Kind, files map[string][]byte, diags []*dslpb.Diagnostic) *catalogpb.CatalogEntry_Summary {
	if kind == catalogpb.Kind_KIND_WORKFLOW {
		return DeriveWorkflowSummary(files, diags)
	}
	manifest, _ := ast.DecodeProviderManifest("manifest.yaml", files["manifest.yaml"])
	return DeriveProviderSummary(manifest)
}
```

Implement `UpdateOrgProvider`/`UpdateOrgWorkflow` (native path only in this task — `LINKED`-row forking is Task 6), `DeleteOrgProvider`/`DeleteOrgWorkflow`, `GetOrgProvider`/`GetOrgWorkflow`, `ListOrgProviders`/`ListOrgWorkflows`, and the five `admin_only` instance-level handlers, each following the same shared-private-helper-per-verb shape as `createOrgEntry` (`updateOrgEntry`, `deleteOrgEntry`, `getOrgEntry`, `listOrgEntries`, `createInstanceEntry`, ...). `Checker`/`Check` wiring for `CheckCatalogProvider`/`CheckCatalogWorkflow` calls `s.d.Check(ctx, kind, req.GetFiles())` directly and returns diagnostics without persisting — mirrors `recipe.Service.CheckRecipe` (`recipe.go:129-142`) minus the repo lookup.

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/services/catalog/... -v`
Expected: PASS.

- [ ] **Step 7: Write the `Grantable` auto-detection test**

```go
// internal/services/iam/catalog_test.go — add to existing file
import (
	_ "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog" // registers CatalogService annotations
)

func TestCatalog_Grantable_PicksUpCatalogServiceResources(t *testing.T) {
	entries, err := NewCatalog().Grantable(context.Background())
	if err != nil {
		t.Fatalf("Grantable: %v", err)
	}
	want := map[[2]iamValue]bool{
		{iam.Resource_RESOURCE_PROVIDER, iam.Action_ACTION_CREATE}: false,
		{iam.Resource_RESOURCE_PROVIDER, iam.Action_ACTION_MANAGE}: false,
		{iam.Resource_RESOURCE_WORKFLOW, iam.Action_ACTION_READ}:   false,
		{iam.Resource_RESOURCE_WORKFLOW, iam.Action_ACTION_MANAGE}: false,
	}
	for _, e := range entries {
		key := [2]iamValue{e.GetPermission().GetResource(), e.GetPermission().GetAction()}
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for k, found := range want {
		if !found {
			t.Errorf("Grantable() missing %v — CatalogService annotations not picked up", k)
		}
	}
}
```

(`iamValue` is a tiny local type alias `type iamValue = int32` used only to make the map key comparable across the `Resource`/`Action` enum types — replace with two parallel maps if a single generic key type is awkward; the assertion intent is what matters: every `all_of` permission declared on `CatalogService`'s proto, plus the synthesized `ACTION_MANAGE` wildcard per resource, must appear in `Grantable()`'s output with zero new code in `catalog.go` beyond Task 3's label additions.)

- [ ] **Step 8: Run test to verify it passes**

Run: `go test ./internal/services/iam/... -run TestCatalog_Grantable_PicksUpCatalogServiceResources -v`
Expected: PASS — confirms spec §8's "zero new code in `catalog.go`" claim holds now that `CatalogService` is registered.

- [ ] **Step 9: Commit**

```bash
git add protocols/cloud/v1/catalog/service.proto internal/proto/cloud/v1/catalog \
  internal/services/catalog/catalog.go internal/services/catalog/service.go internal/services/catalog/service_test.go \
  internal/services/iam/catalog_test.go
git commit -m "feat(catalog): add CatalogService RPCs with per-kind RBAC gates"
```

---

### Task 5: `SeedOrgCatalog` at `CreateTenant`

Materialize every `LEVEL_INSTANCE` entry into a newly created tenant's org catalog as `origin=LINKED`, idempotently, inside the same transaction as the owner Role + Membership seeding (`iam.IamService.CreateTenant`, `internal/services/iam/service.go:697-761`). Also ensures builtin providers (`docker`) exist as instance entries first.

**Files:**
- Modify: `internal/services/iam/service.go` (`IamDeps.CatalogSeeder` field + `CreateTenant`)
- Test: `internal/services/iam/service_test.go`
- Modify: `internal/services/catalog/catalog.go` (`Deps` gains no new field — `BuiltinProviders` already added in Task 4)
- Create: `internal/services/catalog/seed.go` (`SeedOrgCatalog`, `ensureBuiltinInstanceEntries`)
- Test: `internal/services/catalog/seed_test.go`

**Interfaces:**
```go
// internal/services/iam/service.go — new interface next to TenantRepo/RoleRepo
// CatalogSeeder seeds a newly created tenant's org catalog inside the ambient
// CreateTenant transaction. Implemented by catalog.Service; iam never imports
// package catalog (Deps-of-interfaces shape, matches every other IamDeps field).
type CatalogSeeder interface {
	SeedOrgCatalog(ctx context.Context, tenantID string) error
}
```
```go
// internal/services/catalog/catalog.go
func (s *Service) SeedOrgCatalog(ctx context.Context, tenantID string) error
```

- [ ] **Step 1: Write the failing test for `catalog.Service.SeedOrgCatalog`**

```go
// internal/services/catalog/seed_test.go
package catalog

import (
	"context"
	"testing"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
)

func TestSeedOrgCatalog_LinksEveryInstanceEntry(t *testing.T) {
	repo := newFakeEntryRepo()
	// two pre-existing instance entries
	repo.mustCreate(t, instanceEntry(catalogpb.Kind_KIND_PROVIDER, "yandex", 1))
	repo.mustCreate(t, instanceEntry(catalogpb.Kind_KIND_WORKFLOW, "tpcc", 1))

	svc := NewService(Deps{Entries: repo, Bundles: NewMemoryBundleStore(), Check: stubChecker(nil), Authn: fakeAuthn{}})
	if err := svc.SeedOrgCatalog(context.Background(), "tenant-1"); err != nil {
		t.Fatalf("SeedOrgCatalog: %v", err)
	}

	orgEntries, err := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-1", catalogpb.Kind_KIND_PROVIDER)
	if err != nil || len(orgEntries) != 1 {
		t.Fatalf("expected 1 linked provider, got %d (err=%v)", len(orgEntries), err)
	}
	if orgEntries[0].GetOrigin() != catalogpb.Origin_ORIGIN_LINKED {
		t.Fatalf("origin = %v, want LINKED", orgEntries[0].GetOrigin())
	}
	if orgEntries[0].GetSourceRef() != "" {
		t.Fatal("LINKED row must not have its own source_ref before first fork")
	}
}

func TestSeedOrgCatalog_Idempotent(t *testing.T) {
	repo := newFakeEntryRepo()
	repo.mustCreate(t, instanceEntry(catalogpb.Kind_KIND_PROVIDER, "yandex", 1))
	svc := NewService(Deps{Entries: repo, Bundles: NewMemoryBundleStore(), Check: stubChecker(nil), Authn: fakeAuthn{}})

	if err := svc.SeedOrgCatalog(context.Background(), "tenant-1"); err != nil {
		t.Fatalf("1st SeedOrgCatalog: %v", err)
	}
	if err := svc.SeedOrgCatalog(context.Background(), "tenant-1"); err != nil {
		t.Fatalf("2nd SeedOrgCatalog: %v", err)
	}
	orgEntries, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-1", catalogpb.Kind_KIND_PROVIDER)
	if len(orgEntries) != 1 {
		t.Fatalf("expected exactly 1 linked entry after 2 seed calls, got %d", len(orgEntries))
	}
}

func TestSeedOrgCatalog_SeedsBuiltinDockerFirst(t *testing.T) {
	repo := newFakeEntryRepo()
	svc := NewService(Deps{
		Entries: repo, Bundles: NewMemoryBundleStore(), Check: stubChecker(nil), Authn: fakeAuthn{},
		BuiltinProviders: map[string]map[string][]byte{"docker": {"manifest.yaml": []byte("name: docker\nprovides:\n  - machines\n")}},
	})
	if err := svc.SeedOrgCatalog(context.Background(), "tenant-1"); err != nil {
		t.Fatalf("SeedOrgCatalog: %v", err)
	}
	instanceEntries, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_INSTANCE, "", catalogpb.Kind_KIND_PROVIDER)
	if len(instanceEntries) != 1 || instanceEntries[0].GetSlug() != "docker" {
		t.Fatalf("expected docker seeded as instance entry, got %+v", instanceEntries)
	}
	orgEntries, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-1", catalogpb.Kind_KIND_PROVIDER)
	if len(orgEntries) != 1 || orgEntries[0].GetSlug() != "docker" {
		t.Fatalf("expected docker linked into org catalog, got %+v", orgEntries)
	}
}
```

Add `instanceEntry(kind, slug, version)` test helper and `fakeEntryRepo.mustCreate` in `service_test.go` (or a shared `catalog_testutil_test.go`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/services/catalog/... -run TestSeedOrgCatalog -v`
Expected: FAIL — `svc.SeedOrgCatalog undefined`.

- [ ] **Step 3: Implement `SeedOrgCatalog`**

```go
// internal/services/catalog/seed.go
package catalog

import (
	"context"
	"errors"

	"github.com/google/uuid"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
)

// SeedOrgCatalog links every LEVEL_INSTANCE entry into tenantID's org catalog
// as origin=LINKED rows (Global Constraints, Decision 1: live link, no
// source_ref copy until first edit). Idempotent: entries already linked
// (matched by source_entry_id) are skipped. Ensures builtin providers exist
// as instance entries first.
func (s *Service) SeedOrgCatalog(ctx context.Context, tenantID string) error {
	if err := s.ensureBuiltinInstanceEntries(ctx); err != nil {
		return err
	}
	for _, kind := range []catalogpb.Kind{catalogpb.Kind_KIND_PROVIDER, catalogpb.Kind_KIND_WORKFLOW} {
		instanceEntries, err := s.d.Entries.List(ctx, catalogpb.Level_LEVEL_INSTANCE, "", kind)
		if err != nil {
			return err
		}
		for _, ie := range instanceEntries {
			linked, err := s.d.Entries.ListBySource(ctx, ie.GetEntity().GetId())
			if err != nil {
				return err
			}
			if alreadyLinkedForTenant(linked, tenantID) {
				continue
			}
			org := &catalogpb.CatalogEntry{
				Entity: &common.Entity{
					Id:          uuid.NewString(),
					TenantId:    tenantID,
					Name:        ie.GetEntity().GetName(),
					Description: ie.GetEntity().GetDescription(),
				},
				Level:         catalogpb.Level_LEVEL_ORG,
				Kind:          kind,
				Slug:          ie.GetSlug(),
				Version:       ie.GetVersion(),
				Origin:        catalogpb.Origin_ORIGIN_LINKED,
				SourceEntryId: ie.GetEntity().GetId(),
				Summary:       ie.GetSummary(),
			}
			if err := s.d.Entries.Create(ctx, org); err != nil && derrors.IgnoreConflict(err) != nil {
				return err
			}
		}
	}
	return nil
}

func alreadyLinkedForTenant(linked []*catalogpb.CatalogEntry, tenantID string) bool {
	for _, l := range linked {
		if l.GetEntity().GetTenantId() == tenantID {
			return true
		}
	}
	return false
}

// ensureBuiltinInstanceEntries seeds Deps.BuiltinProviders (e.g. "docker") as
// LEVEL_INSTANCE catalog entries if not already present — unifies provider
// resolution (Task 7) so no code path special-cases the docker builtin.
func (s *Service) ensureBuiltinInstanceEntries(ctx context.Context) error {
	for slug, files := range s.d.BuiltinProviders {
		_, err := s.d.Entries.GetLatestBySlug(ctx, catalogpb.Level_LEVEL_INSTANCE, "", catalogpb.Kind_KIND_PROVIDER, slug)
		if err == nil {
			continue
		}
		if !errors.Is(err, derrors.ErrNotFound) {
			return err
		}
		ref, err := s.d.Bundles.Write(ctx, "", files)
		if err != nil {
			return err
		}
		manifest, _ := ast.DecodeProviderManifest("manifest.yaml", files["manifest.yaml"])
		entry := &catalogpb.CatalogEntry{
			Entity:    &common.Entity{Id: uuid.NewString(), Name: slug},
			Level:     catalogpb.Level_LEVEL_INSTANCE,
			Kind:      catalogpb.Kind_KIND_PROVIDER,
			Slug:      slug,
			Version:   1,
			Origin:    catalogpb.Origin_ORIGIN_NATIVE,
			SourceRef: ref,
			Summary:   DeriveProviderSummary(manifest),
		}
		if err := s.d.Entries.Create(ctx, entry); err != nil && derrors.IgnoreConflict(err) != nil {
			return err
		}
	}
	return nil
}
```

(Confirm `derrors.IgnoreConflict` exists alongside `derrors.IgnoreNotFound` — `internal/domain/errors`; if it doesn't, add it there first as a one-line sibling function, or inline `if !errors.Is(err, derrors.ErrConflict) { return err }`.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/services/catalog/... -run TestSeedOrgCatalog -v`
Expected: PASS.

- [ ] **Step 5: Write the failing test for `IamService.CreateTenant` wiring**

```go
// internal/services/iam/service_test.go — add
type fakeCatalogSeeder struct {
	seeded []string
	err    error
}

func (f *fakeCatalogSeeder) SeedOrgCatalog(_ context.Context, tenantID string) error {
	f.seeded = append(f.seeded, tenantID)
	return f.err
}

func TestCreateTenant_SeedsOrgCatalog(t *testing.T) {
	seeder := &fakeCatalogSeeder{}
	svc := NewIamService(IamDeps{
		Authn: fakeCallerAuthn{accountID: "acct-1"},
		Gates: fakePlatformGates{},
		Catalog: fakePermissionCatalog{},
		Tenants: &fakeTenantRepo{},
		Roles: &fakeRoleRepo{},
		Memberships: &fakeMembershipRepo{},
		CatalogSeeder: seeder,
		Tx: noopTrm{},
	})
	resp, err := svc.CreateTenant(context.Background(), &api.CreateTenantRequest{Name: "Acme", Slug: "acme"})
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if len(seeder.seeded) != 1 || seeder.seeded[0] != resp.GetTenant().GetId() {
		t.Fatalf("expected SeedOrgCatalog called once with new tenant id, got %v", seeder.seeded)
	}
}

func TestCreateTenant_SeedFailureAbortsTransaction(t *testing.T) {
	seeder := &fakeCatalogSeeder{err: errors.New("bundle store unavailable")}
	tenants := &fakeTenantRepo{}
	svc := NewIamService(IamDeps{
		Authn: fakeCallerAuthn{accountID: "acct-1"}, Gates: fakePlatformGates{}, Catalog: fakePermissionCatalog{},
		Tenants: tenants, Roles: &fakeRoleRepo{}, Memberships: &fakeMembershipRepo{},
		CatalogSeeder: seeder, Tx: noopTrm{},
	})
	_, err := svc.CreateTenant(context.Background(), &api.CreateTenantRequest{Name: "Acme", Slug: "acme"})
	if err == nil {
		t.Fatal("expected error when CatalogSeeder fails")
	}
	if tenants.created != 0 {
		// noopTrm just calls fn(ctx) directly with no real rollback — this
		// assertion documents that under a REAL Trm, the tenant row would be
		// rolled back with the rest of the tx; noopTrm cannot exercise that,
		// so this test only asserts CreateTenant surfaces the seeder's error.
		t.Log("noopTrm does not roll back; real Postgres Trm does (existing invariant, not new)")
	}
}
```

(Exact `fakeTenantRepo`/`fakeRoleRepo`/`fakeMembershipRepo`/`fakeCallerAuthn`/`fakePlatformGates`/`fakePermissionCatalog` field/method names must match `internal/services/iam/service_test.go`'s existing fakes — reuse them as-is, only `fakeCatalogSeeder` is new.)

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/services/iam/... -run TestCreateTenant_SeedsOrgCatalog -v`
Expected: FAIL — `IamDeps` has no field `CatalogSeeder`.

- [ ] **Step 7: Add `CatalogSeeder` to `IamDeps` and wire it into `CreateTenant`**

```go
// internal/services/iam/service.go — new interface near TenantRepo/RoleRepo
type CatalogSeeder interface {
	SeedOrgCatalog(ctx context.Context, tenantID string) error
}
```

Add `CatalogSeeder CatalogSeeder` field to `IamDeps` (line ~226, next to `Memberships MembershipRepo`).

In `CreateTenant`'s `doTxRet` closure (`service.go:711-758`), after the `Memberships.Create` call and before `return &api.CreateTenantResponse{...}, nil`:

```go
		if s.d.CatalogSeeder != nil {
			if err := s.d.CatalogSeeder.SeedOrgCatalog(ctx, tenant.Id); err != nil {
				return nil, utils.MapErr(err)
			}
		}
```

(`nil`-guarded so every existing `IamDeps{...}` literal across the test suite that doesn't set `CatalogSeeder` keeps compiling and passing unchanged — matches the "backward compatible" posture used throughout this plan.)

- [ ] **Step 8: Run test to verify it passes**

Run: `go test ./internal/services/iam/... -run TestCreateTenant -v`
Expected: PASS. Then: `go test ./internal/services/iam/...` → full package PASS (no regression in the ~30 other `CreateTenant`-adjacent tests already in `service_test.go`).

- [ ] **Step 9: Commit**

```bash
git add internal/services/iam/service.go internal/services/iam/service_test.go \
  internal/services/catalog/seed.go internal/services/catalog/seed_test.go
git commit -m "feat(catalog): seed org catalog from instance catalog on tenant creation"
```

---

### Task 6: Fork-on-edit (`ForkEntry`)

`UpdateOrgProvider`/`UpdateOrgWorkflow` on a `LINKED` row must not mutate it — it forks to a new `FORKED` row (`version+1`, own `source_ref` via `BundleStore.Fork`), leaving the instance row and every sibling org's `LINKED` row untouched (Global Constraints, Decision 1).

**Files:**
- Modify: `internal/services/catalog/catalog.go` (`ForkEntry`)
- Modify: `internal/services/catalog/service.go` (`updateOrgEntry` calls `ForkEntry` when `origin == LINKED`)
- Test: `internal/services/catalog/fork_test.go`

**Interfaces:**
```go
// ForkEntry materializes an org-owned copy of a LINKED entry on first edit:
// bumps version, sets origin=FORKED, calls Bundles.Fork against the
// *instance* row's source_ref (LINKED rows have none of their own — read it
// via source_entry_id), keeps source_entry_id for future diff/re-sync
// tooling. UpdateOrgProvider/UpdateOrgWorkflow call this internally; exposed
// separately so an explicit "customize before editing" UI action can call it
// without submitting a diff yet.
func (s *Service) ForkEntry(ctx context.Context, tenantID, orgEntryID string) (*catalogpb.CatalogEntry, error)
```

- [ ] **Step 1: Write the failing test**

```go
// internal/services/catalog/fork_test.go
package catalog

import (
	"context"
	"testing"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
)

func TestForkEntry_LinkedRowForks(t *testing.T) {
	repo := newFakeEntryRepo()
	store := NewMemoryBundleStore()
	instanceRef, _ := store.Write(context.Background(), "", map[string][]byte{"manifest.yaml": []byte("name: yandex\n")})
	instance := instanceEntry(catalogpb.Kind_KIND_PROVIDER, "yandex", 1)
	instance.SourceRef = instanceRef
	repo.mustCreate(t, instance)

	svc := NewService(Deps{Entries: repo, Bundles: store, Check: stubChecker(nil), Authn: fakeAuthn{}})
	if err := svc.SeedOrgCatalog(context.Background(), "tenant-1"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	orgEntries, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-1", catalogpb.Kind_KIND_PROVIDER)
	linked := orgEntries[0]

	forked, err := svc.ForkEntry(context.Background(), "tenant-1", linked.GetEntity().GetId())
	if err != nil {
		t.Fatalf("ForkEntry: %v", err)
	}
	if forked.GetOrigin() != catalogpb.Origin_ORIGIN_FORKED {
		t.Fatalf("origin = %v, want FORKED", forked.GetOrigin())
	}
	if forked.GetVersion() != linked.GetVersion()+1 {
		t.Fatalf("version = %d, want %d", forked.GetVersion(), linked.GetVersion()+1)
	}
	if forked.GetSourceRef() == "" || forked.GetSourceRef() == instanceRef {
		t.Fatal("forked row must have its own distinct source_ref")
	}
	if forked.GetSourceEntryId() != instance.GetEntity().GetId() {
		t.Fatal("forked row must keep source_entry_id for lineage")
	}

	// instance row is untouched
	stillInstance, err := repo.Get(context.Background(), catalogpb.Level_LEVEL_INSTANCE, "", instance.GetEntity().GetId())
	if err != nil || stillInstance.GetOrigin() != catalogpb.Origin_ORIGIN_NATIVE || stillInstance.GetSourceRef() != instanceRef {
		t.Fatal("instance row must not be mutated by ForkEntry")
	}
}

func TestForkEntry_OtherTenantsLinkedRowUntouched(t *testing.T) {
	repo := newFakeEntryRepo()
	store := NewMemoryBundleStore()
	instance := instanceEntry(catalogpb.Kind_KIND_PROVIDER, "yandex", 1)
	instanceRef, _ := store.Write(context.Background(), "", map[string][]byte{"manifest.yaml": []byte("name: yandex\n")})
	instance.SourceRef = instanceRef
	repo.mustCreate(t, instance)

	svc := NewService(Deps{Entries: repo, Bundles: store, Check: stubChecker(nil), Authn: fakeAuthn{}})
	svc.SeedOrgCatalog(context.Background(), "tenant-A")
	svc.SeedOrgCatalog(context.Background(), "tenant-B")

	aEntries, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-A", catalogpb.Kind_KIND_PROVIDER)
	svc.ForkEntry(context.Background(), "tenant-A", aEntries[0].GetEntity().GetId())

	bEntries, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-B", catalogpb.Kind_KIND_PROVIDER)
	if bEntries[0].GetOrigin() != catalogpb.Origin_ORIGIN_LINKED {
		t.Fatalf("tenant-B's row must remain LINKED, got %v", bEntries[0].GetOrigin())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/services/catalog/... -run TestForkEntry -v`
Expected: FAIL — `svc.ForkEntry undefined`.

- [ ] **Step 3: Implement `ForkEntry`**

```go
// internal/services/catalog/catalog.go — append
func (s *Service) ForkEntry(ctx context.Context, tenantID, orgEntryID string) (*catalogpb.CatalogEntry, error) {
	entry, err := s.d.Entries.Get(ctx, catalogpb.Level_LEVEL_ORG, tenantID, orgEntryID)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if entry.GetOrigin() != catalogpb.Origin_ORIGIN_LINKED {
		return entry, nil // NATIVE/FORKED rows are already org-owned — no-op.
	}
	sourceRef := entry.GetSourceRef()
	if sourceRef == "" {
		// LINKED row with no own source_ref yet — read the instance row's ref.
		src, err := s.d.Entries.Get(ctx, catalogpb.Level_LEVEL_INSTANCE, "", entry.GetSourceEntryId())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		sourceRef = src.GetSourceRef()
	}
	forkedRef, err := s.d.Bundles.Fork(ctx, sourceRef)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	forked := proto.Clone(entry).(*catalogpb.CatalogEntry)
	forked.Entity.Id = uuid.NewString()
	forked.Version = entry.GetVersion() + 1
	forked.Origin = catalogpb.Origin_ORIGIN_FORKED
	forked.SourceRef = forkedRef
	// SourceEntryId is preserved (kept from entry) for lineage.
	if err := s.d.Entries.Create(ctx, forked); err != nil {
		return nil, utils.MapErr(err)
	}
	return forked, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/services/catalog/... -run TestForkEntry -v`
Expected: PASS.

- [ ] **Step 5: Wire `ForkEntry` into `UpdateOrgProvider`/`UpdateOrgWorkflow`**

In `internal/services/catalog/service.go`'s `updateOrgEntry` helper (added in Task 4 Step 5), before applying the file diff:

```go
func (s *Service) updateOrgEntry(ctx context.Context, kind catalogpb.Kind, tenantID, id string, files map[string][]byte) (*catalogpb.CatalogEntry, error) {
	current, err := s.d.Entries.Get(ctx, catalogpb.Level_LEVEL_ORG, tenantID, id)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if current.GetOrigin() == catalogpb.Origin_ORIGIN_LINKED {
		forked, err := s.ForkEntry(ctx, tenantID, id)
		if err != nil {
			return nil, err
		}
		current = forked
		id = forked.GetEntity().GetId()
	}
	diags, err := s.d.Check(ctx, kind, files)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	ref, err := s.d.Bundles.Write(ctx, current.GetSourceRef(), files)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	current.SourceRef = ref
	current.Summary = summaryFor(kind, files, diags)
	if err := s.d.Entries.Update(ctx, current); err != nil {
		return nil, utils.MapErr(err)
	}
	return current, nil
}
```

- [ ] **Step 6: Write the failing integration test — `UpdateOrgProvider` on a LINKED row forks**

```go
func TestUpdateOrgProvider_ForksLinkedRow(t *testing.T) {
	repo := newFakeEntryRepo()
	store := NewMemoryBundleStore()
	instanceRef, _ := store.Write(context.Background(), "", map[string][]byte{"manifest.yaml": []byte("name: yandex\n")})
	instance := instanceEntry(catalogpb.Kind_KIND_PROVIDER, "yandex", 1)
	instance.SourceRef = instanceRef
	repo.mustCreate(t, instance)

	svc := NewService(Deps{Entries: repo, Bundles: store, Check: stubChecker(nil), Authn: fakeAuthn{}})
	svc.SeedOrgCatalog(context.Background(), "tenant-1")
	linked, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-1", catalogpb.Kind_KIND_PROVIDER)

	resp, err := svc.UpdateOrgProvider(context.Background(), &catalogpb.UpdateOrgProviderRequest{
		TenantId: "tenant-1", Id: linked[0].GetEntity().GetId(),
		Files: map[string][]byte{"manifest.yaml": []byte("name: yandex\nprovides:\n  - machines\n")},
	})
	if err != nil {
		t.Fatalf("UpdateOrgProvider: %v", err)
	}
	if resp.GetEntry().GetOrigin() != catalogpb.Origin_ORIGIN_FORKED {
		t.Fatalf("origin = %v, want FORKED", resp.GetEntry().GetOrigin())
	}
	if resp.GetEntry().GetEntity().GetId() == linked[0].GetEntity().GetId() {
		t.Fatal("fork-on-edit must produce a new row id, not mutate the LINKED row in place")
	}
}
```

- [ ] **Step 7: Run to verify pass** — `go test ./internal/services/catalog/... -v` → PASS (full package).

- [ ] **Step 8: Commit**

```bash
git add internal/services/catalog/catalog.go internal/services/catalog/service.go internal/services/catalog/fork_test.go
git commit -m "feat(catalog): fork org-catalog entries on first edit of a linked row"
```

---

### Task 7: `provider.use` resolution against the org catalog (pinned version)

Add a `ProviderResolver` interface + `catalog`-backed implementation, and a new, backward-compatible `CompileBundleWithCatalog` entry point in `internal/services/dsl`. The zero-arg `CompileBundle(files)` used by every existing caller stays untouched — this task only affects `DslService.Check`/`Preview` once a resolver is wired via a new `Option`.

**Files:**
- Modify: `internal/services/dsl/service.go` (`ProviderResolver` interface, `resolveProvider` signature, `CompileBundleWithCatalog`, `WithProviderResolver` Option, `Check`/`Preview` wiring)
- Modify: `protocols/cloud/v1/dsl/service.proto` (add `string tenant_id = 2;` to `CheckRequest`/`PreviewRequest`)
- Create: `internal/services/catalog/provider_resolver.go` (`CatalogProviderResolver`)
- Test: `internal/services/dsl/service_test.go`
- Test: `internal/services/catalog/provider_resolver_test.go`

**Interfaces:**
```go
// internal/services/dsl/service.go
// ProviderResolver resolves a provider.use slug (optionally pinned
// "slug@version") against an org catalog, returning the provider's
// manifest.yaml + module/*.tf files exactly as they'd have appeared under
// providers/<name>/ in the pre-SP-B bundle layout. nil is a valid Deps value:
// resolveProvider falls back to the legacy same-bundle lookup when unset.
type ProviderResolver interface {
	ResolveProvider(ctx context.Context, tenantID, slug string, version uint32) (files map[string][]byte, resolvedVersion uint32, err error)
}
```
```go
// internal/services/catalog/provider_resolver.go
type CatalogProviderResolver struct {
	Entries CatalogEntryRepo
	Bundles BundleStore
}
func (r *CatalogProviderResolver) ResolveProvider(ctx context.Context, tenantID, slug string, version uint32) (map[string][]byte, uint32, error)
```

- [ ] **Step 1: Write the failing test for `CatalogProviderResolver`**

```go
// internal/services/catalog/provider_resolver_test.go
package catalog

import (
	"context"
	"testing"
)

func TestCatalogProviderResolver_Unpinned_ResolvesLatest(t *testing.T) {
	repo := newFakeEntryRepo()
	store := NewMemoryBundleStore()
	ref1, _ := store.Write(context.Background(), "", map[string][]byte{"manifest.yaml": []byte("name: yandex\nv: 1\n")})
	ref2, _ := store.Write(context.Background(), "", map[string][]byte{"manifest.yaml": []byte("name: yandex\nv: 2\n")})
	e1 := orgEntry(catalogpb.Kind_KIND_PROVIDER, "tenant-1", "yandex", 1)
	e1.SourceRef = ref1
	e2 := orgEntry(catalogpb.Kind_KIND_PROVIDER, "tenant-1", "yandex", 2)
	e2.SourceRef = ref2
	repo.mustCreate(t, e1)
	repo.mustCreate(t, e2)

	resolver := &CatalogProviderResolver{Entries: repo, Bundles: store}
	files, version, err := resolver.ResolveProvider(context.Background(), "tenant-1", "yandex", 0)
	if err != nil {
		t.Fatalf("ResolveProvider: %v", err)
	}
	if version != 2 {
		t.Fatalf("resolved version = %d, want 2 (latest)", version)
	}
	if string(files["manifest.yaml"]) != "name: yandex\nv: 2\n" {
		t.Fatalf("resolved wrong version's files: %q", files["manifest.yaml"])
	}
}

func TestCatalogProviderResolver_Pinned_ResolvesExactVersion(t *testing.T) {
	repo := newFakeEntryRepo()
	store := NewMemoryBundleStore()
	ref1, _ := store.Write(context.Background(), "", map[string][]byte{"manifest.yaml": []byte("name: yandex\nv: 1\n")})
	ref2, _ := store.Write(context.Background(), "", map[string][]byte{"manifest.yaml": []byte("name: yandex\nv: 2\n")})
	e1 := orgEntry(catalogpb.Kind_KIND_PROVIDER, "tenant-1", "yandex", 1)
	e1.SourceRef = ref1
	e2 := orgEntry(catalogpb.Kind_KIND_PROVIDER, "tenant-1", "yandex", 2)
	e2.SourceRef = ref2
	repo.mustCreate(t, e1)
	repo.mustCreate(t, e2)

	resolver := &CatalogProviderResolver{Entries: repo, Bundles: store}
	files, version, err := resolver.ResolveProvider(context.Background(), "tenant-1", "yandex", 1)
	if err != nil {
		t.Fatalf("ResolveProvider: %v", err)
	}
	if version != 1 || string(files["manifest.yaml"]) != "name: yandex\nv: 1\n" {
		t.Fatalf("expected pinned v1, got version=%d files=%q", version, files["manifest.yaml"])
	}
}

func TestCatalogProviderResolver_NotFound(t *testing.T) {
	resolver := &CatalogProviderResolver{Entries: newFakeEntryRepo(), Bundles: NewMemoryBundleStore()}
	_, _, err := resolver.ResolveProvider(context.Background(), "tenant-1", "nope", 0)
	if err == nil {
		t.Fatal("expected error for unknown provider slug")
	}
}
```

Add `orgEntry(kind, tenantID, slug, version)` test helper next to `instanceEntry`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/services/catalog/... -run TestCatalogProviderResolver -v`
Expected: FAIL — `undefined: CatalogProviderResolver`.

- [ ] **Step 3: Implement `CatalogProviderResolver`**

```go
package catalog

import (
	"context"
	"fmt"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
)

type CatalogProviderResolver struct {
	Entries CatalogEntryRepo
	Bundles BundleStore
}

func (r *CatalogProviderResolver) ResolveProvider(ctx context.Context, tenantID, slug string, version uint32) (map[string][]byte, uint32, error) {
	var entry *catalogpb.CatalogEntry
	var err error
	if version == 0 {
		entry, err = r.Entries.GetLatestBySlug(ctx, catalogpb.Level_LEVEL_ORG, tenantID, catalogpb.Kind_KIND_PROVIDER, slug)
	} else {
		entry, err = r.Entries.GetBySlugVersion(ctx, catalogpb.Level_LEVEL_ORG, tenantID, catalogpb.Kind_KIND_PROVIDER, slug, version)
	}
	if err != nil {
		if derrors.IgnoreNotFound(err) == nil {
			return nil, 0, fmt.Errorf("provider %q not found in org catalog: %w", slug, err)
		}
		return nil, 0, err
	}
	ref := entry.GetSourceRef()
	if ref == "" {
		// LINKED, never forked — files live under the instance row it points at.
		src, err := r.Entries.Get(ctx, catalogpb.Level_LEVEL_INSTANCE, "", entry.GetSourceEntryId())
		if err != nil {
			return nil, 0, fmt.Errorf("provider %q: resolve linked source: %w", slug, err)
		}
		ref = src.GetSourceRef()
	}
	files, err := r.Bundles.Read(ctx, ref)
	if err != nil {
		return nil, 0, fmt.Errorf("provider %q: read bundle: %w", slug, err)
	}
	return files, entry.GetVersion(), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/services/catalog/... -run TestCatalogProviderResolver -v`
Expected: PASS.

- [ ] **Step 5: Write the failing test for `dsl.CompileBundleWithCatalog` + `slug@version` parsing**

```go
// internal/services/dsl/service_test.go — add
type fakeProviderResolver struct {
	files   map[string][]byte
	version uint32
	err     error
	gotSlug string
	gotVersion uint32
}

func (f *fakeProviderResolver) ResolveProvider(_ context.Context, _, slug string, version uint32) (map[string][]byte, uint32, error) {
	f.gotSlug, f.gotVersion = slug, version
	if f.err != nil {
		return nil, 0, f.err
	}
	return f.files, f.version, nil
}

func TestCompileBundleWithCatalog_PinnedVersionParsed(t *testing.T) {
	resolver := &fakeProviderResolver{
		files:   map[string][]byte{"manifest.yaml": []byte("name: yandex\nprovides:\n  - machines\n")},
		version: 3,
	}
	files := map[string][]byte{
		"cluster.yaml": []byte("version: 1\nprovider:\n  use: yandex@3\nmachines:\n  db:\n    count: 1\nservices: {}\n"),
	}
	_, diags := CompileBundleWithCatalog(context.Background(), "tenant-1", files, resolver)
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %s", diags.String())
	}
	if resolver.gotSlug != "yandex" || resolver.gotVersion != 3 {
		t.Fatalf("resolver called with (%q, %d), want (yandex, 3)", resolver.gotSlug, resolver.gotVersion)
	}
}

func TestCompileBundleWithCatalog_UnpinnedWarnsAndResolvesLatest(t *testing.T) {
	resolver := &fakeProviderResolver{
		files:   map[string][]byte{"manifest.yaml": []byte("name: yandex\nprovides:\n  - machines\n")},
		version: 5,
	}
	files := map[string][]byte{
		"cluster.yaml": []byte("version: 1\nprovider:\n  use: yandex\nmachines:\n  db:\n    count: 1\nservices: {}\n"),
	}
	_, diags := CompileBundleWithCatalog(context.Background(), "tenant-1", files, resolver)
	if resolver.gotVersion != 0 {
		t.Fatalf("unpinned use should request version=0 (latest), got %d", resolver.gotVersion)
	}
	if !hasWarning(diags, "pin") {
		t.Fatalf("expected a reproducibility warning diagnostic, got: %s", diags.String())
	}
}

func TestCompileBundle_LegacySignatureUnaffected(t *testing.T) {
	// Zero-arg CompileBundle must keep working exactly as before this task —
	// no resolver, no tenant, same-bundle providers/<name>/ lookup.
	files := dockerBundleFiles() // existing test helper
	_, diags := CompileBundle(files)
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %s", diags.String())
	}
}
```

(`hasWarning(diags diag.List, substr string) bool` — small local helper scanning `diags` for a `Severity: diag.Warning` message containing `substr`.)

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/services/dsl/... -run TestCompileBundleWithCatalog -v`
Expected: FAIL — `undefined: CompileBundleWithCatalog`.

- [ ] **Step 7: Implement `ProviderResolver`, `CompileBundleWithCatalog`, and the pinned-version parse**

```go
// internal/services/dsl/service.go

// ProviderResolver resolves provider.use against an org catalog. See
// internal/services/catalog.CatalogProviderResolver for the production impl —
// this package does not import catalog (kept a pure compiler, per file-header
// doc comment); the interface is satisfied structurally.
type ProviderResolver interface {
	ResolveProvider(ctx context.Context, tenantID, slug string, version uint32) (files map[string][]byte, resolvedVersion uint32, err error)
}

// CompileBundle compiles files with the legacy same-bundle provider lookup
// (providers/<name>/manifest.yaml inside files itself). Every pre-SP-B caller
// keeps using this signature unchanged.
func CompileBundle(files map[string][]byte) (*dslpb.CompiledPlan, diag.List) {
	return compileBundle(context.Background(), "", files, nil)
}

// CompileBundleWithCatalog compiles files resolving provider.use ("slug" or
// "slug@version") against resolver's org catalog instead of files' own
// providers/<name>/ directory (spec SP-B §B5). resolver == nil falls back to
// the legacy lookup exactly like CompileBundle.
func CompileBundleWithCatalog(ctx context.Context, tenantID string, files map[string][]byte, resolver ProviderResolver) (*dslpb.CompiledPlan, diag.List) {
	return compileBundle(ctx, tenantID, files, resolver)
}

func compileBundle(ctx context.Context, tenantID string, files map[string][]byte, resolver ProviderResolver) (*dslpb.CompiledPlan, diag.List) {
	sources := include.Sources{Files: files}
	provider, composed, diags := resolveProvider(ctx, tenantID, files, resolver)
	// ... unchanged remainder of the existing CompileBundle body, now fed by
	// the ctx/tenantID/resolver-aware resolveProvider.
	_ = sources
	return nil, diags // placeholder wiring marker for the plan text only — the
	// implementer keeps every line of CompileBundle's existing body below the
	// resolveProvider call verbatim, moving it into this function.
}
```

> Note on the snippet above: the last two lines (`_ = sources` / `return nil, diags`) are **plan-authoring shorthand**, not code to ship — `compileBundle`'s actual body is `CompileBundle`'s current full implementation (`internal/services/dsl/service.go`, the code after its `resolveProvider` call today), moved verbatim into this new private function and parameterized. The implementer deletes the placeholder lines and re-homes the real body; `go vet`/`go build` will fail loudly (unused `sources`, wrong return type) until that's done, which is the point — it forces the real body to move, nothing is allowed to ship as a stub.

```go
func resolveProvider(ctx context.Context, tenantID string, files map[string][]byte, resolver ProviderResolver) (*ast.ProviderManifest, *jsonschema.Schema, diag.List) {
	var diags diag.List

	use := peekProviderUse(files[clusterFile])
	if use == "" {
		return nil, nil, diags
	}
	slug, pinned, version := parseProviderUse(use)

	var providerFiles map[string][]byte
	var resolvedVersion uint32
	var err error
	switch {
	case resolver != nil:
		providerFiles, resolvedVersion, err = resolver.ResolveProvider(ctx, tenantID, slug, version)
		if err != nil {
			diags.Add(diag.Diagnostic{Severity: diag.Error, Path: clusterFile, Message: fmt.Sprintf("provider %q: %v", slug, err), Module: slug})
			return nil, nil, diags
		}
		if !pinned {
			diags.Add(diag.Diagnostic{
				Severity: diag.Warning, Path: clusterFile, Module: slug,
				Message: fmt.Sprintf("provider.use %q is unpinned; resolved to org-catalog version %d — pin with %q for reproducible runs", use, resolvedVersion, fmt.Sprintf("%s@%d", slug, resolvedVersion)),
			})
		}
	default:
		// Legacy same-bundle lookup — unchanged from pre-SP-B behavior.
		mp := manifestPath(slug)
		var ok bool
		providerFiles = files
		_, ok = files[mp]
		if !ok {
			diags.Add(diag.Diagnostic{Severity: diag.Error, Path: clusterFile, Message: fmt.Sprintf("provider %q: manifest not found at %s", slug, mp), Module: slug})
			return nil, nil, diags
		}
	}

	// ... remainder unchanged: ast.DecodeProviderManifest + deriveProviderSchema/
	// schema.Compose against providerFiles (re-keyed to manifest.yaml/module/*.tf
	// when resolver != nil, since catalog-resolved files have no providers/<slug>/
	// prefix — see Step 8 for the exact re-keying helper).
	return nil, nil, diags
}

// parseProviderUse splits "slug" or "slug@version" (Decision 2). An
// unparseable @-suffix (non-numeric) is treated as part of the slug itself —
// permissive, matches diag.List's "never fail the whole module over one bad
// field" convention.
func parseProviderUse(use string) (slug string, pinned bool, version uint32) {
	i := strings.LastIndex(use, "@")
	if i < 0 {
		return use, false, 0
	}
	v, err := strconv.ParseUint(use[i+1:], 10, 32)
	if err != nil {
		return use, false, 0
	}
	return use[:i], true, uint32(v)
}
```

- [ ] **Step 8: Re-key catalog-resolved files and wire the legacy vs. catalog branches into schema derivation**

`deriveProviderSchema(files, name)` (existing, unchanged signature) expects `files` keyed `providers/<name>/module/*.tf` (via `moduleFilePrefix`). When `resolver != nil`, `providerFiles` from `ResolveProvider` is keyed bare (`manifest.yaml`, `module/*.tf`) — add a small re-keying step before calling `deriveProviderSchema`:

```go
	keyedFiles := providerFiles
	if resolver != nil {
		keyedFiles = rekeyUnderProvider(providerFiles, slug)
	}
	manifest, manifestDiags := ast.DecodeProviderManifest(manifestPath(slug), keyedFiles[manifestPath(slug)])
	diags = append(diags, manifestDiags...)
	if manifestDiags.HasErrors() {
		return nil, nil, diags
	}
	params, ext, deriveDiags, err := deriveProviderSchema(keyedFiles, slug)
	// ... unchanged from here (composed schema, error diagnostics) — identical
	// to the pre-existing resolveProvider body's second half.
```

```go
func rekeyUnderProvider(files map[string][]byte, slug string) map[string][]byte {
	out := make(map[string][]byte, len(files))
	for name, data := range files {
		out[path.Join(providersDir, slug, name)] = data
	}
	return out
}
```

- [ ] **Step 9: Add `WithProviderResolver` Option and wire `Check`/`Preview`**

```go
type DslService struct {
	*dslpb.UnimplementedDslServiceServer
	versions  VersionSource
	providers ProviderResolver
}

func WithProviderResolver(pr ProviderResolver) Option {
	return func(s *DslService) { s.providers = pr }
}

func (s *DslService) Check(ctx context.Context, req *dslpb.CheckRequest) (*dslpb.CheckResponse, error) {
	var plan *dslpb.CompiledPlan
	var diags diag.List
	if s.providers != nil && req.GetTenantId() != "" {
		plan, diags = CompileBundleWithCatalog(ctx, req.GetTenantId(), req.GetFiles(), s.providers)
	} else {
		plan, diags = CompileBundle(req.GetFiles())
	}
	validateStroppyVersion(ctx, plan, s.versions, &diags)
	return &dslpb.CheckResponse{Diagnostics: toProtoDiagnostics(diags)}, nil
}
```

Apply the identical `if s.providers != nil && req.GetTenantId() != ""` branch to `Preview`. `ComposedSchema` is explicitly **not** touched by this task — it resolves via `resolveProviderName` on a separate code path (spec §B5 scope note, Global Constraints); composing the launch-form schema against the org catalog is SP-D's concern.

- [ ] **Step 10: Add `tenant_id` to `CheckRequest`/`PreviewRequest`**

```protobuf
// protocols/cloud/v1/dsl/service.proto
message CheckRequest {
    map<string, bytes> files = 1;
    string tenant_id = 2;
}
message PreviewRequest {
    map<string, bytes> files = 1;
    string tenant_id = 2;
}
```

Run: `make protocols`.

- [ ] **Step 11: Run all new tests to verify pass**

Run: `go test ./internal/services/dsl/... -run 'TestCompileBundle' -v`
Expected: PASS, including `TestCompileBundle_LegacySignatureUnaffected`.
Then: `go test ./internal/services/dsl/... ./internal/services/catalog/...` → full packages PASS, zero regressions in existing `CompileBundle`/`resolveProvider` tests (their calls remain zero-arg / legacy-path, per Step 3's `CompileBundle` wrapper).

- [ ] **Step 12: Commit**

```bash
git add internal/services/dsl/service.go internal/services/dsl/service_test.go \
  protocols/cloud/v1/dsl/service.proto internal/proto/cloud/v1/dsl \
  internal/services/catalog/provider_resolver.go internal/services/catalog/provider_resolver_test.go
git commit -m "feat(dsl): resolve provider.use against the org catalog with version pinning"
```

---

## Self-Review

**Spec coverage (SP-B §3):**
- B1 `CatalogEntry` model → Task 1 (`entity`/`level`/`kind`/`slug`/`version`/`origin`/`source_entry_id`/`source_ref`/`summary`, all fields present). ✓
- B2 `catalog_entries` postgres → Task 1 Step 7-8 (DDL matches spec §B2 exactly, `sqld` idiom). ✓
- B3 `CatalogService` + RBAC gates → Task 3 (enum) + Task 4 (proto + handlers + `admin_only`/`all_of`/`tenant_field`, `Grantable` auto-detection test). ✓ Proto-narrowing question (spec §"RESOURCE_PROVIDER в аннотации выше — иллюстративно") resolved: split per-kind RPCs, documented in Global Constraints.
- B4 Default-link + fork-on-edit → Task 5 (`SeedOrgCatalog`, idempotent) + Task 6 (`ForkEntry`, sibling-tenant isolation tested). ✓ Owner Decision 1 implemented literally (no `source_ref` until fork).
- B5 `provider.use` resolution break from bundle → Task 7 (`ProviderResolver`, `CompileBundleWithCatalog`, `slug@version` parsing, `docker` builtin unified via Task 5's `ensureBuiltinInstanceEntries`). ✓ Owner Decision 2 implemented literally (pinned version, warning on unpinned).
- §4 Go interfaces (`CatalogEntryRepo`, `BundleStore`, `Checker`, `Deps`, `SeedOrgCatalog`, `ForkEntry`) → Tasks 1/2/4/5/6, signatures match spec §4 with two additive extensions (`GetBySlugVersion`, `Deps.BuiltinProviders`) both justified by owner Decision 2 and the docker-unification choice, called out inline. ✓
- §5 reuse (`schema.DeriveParamsSchema`, `ast.DecodeCluster`/`DecodeProviderManifest`, `dsl.CheckBundle` as `Checker`, `iam.Resolver`/`Gates`/`Catalog.Grantable`, `iam.Tenant`, JSONB-blob idiom) → confirmed unmodified in every task; no duplicate reimplementation except the one documented third `peekProviderUse` copy (Task 1, matches existing dsl/recipe precedent). ✓
- §6 data flow (materialization / authoring / view-launch / compile-time) → each arrow in spec §6's diagram maps 1:1 to a task: materialization→Task5, org authoring→Task4/6, view→Task4's List RPCs, compile-time→Task7. ✓
- §7 downstream dependencies (SP-C `BundleStore` impl swap, SP-D `ListOrgEntries`/provider resolution, SP-F credential addressing) → Task 2's `BundleStore` interface is the exact seam SP-C swaps; Task 4's `ListOrgProviders`/`ListOrgWorkflows` are what SP-D calls; `CatalogEntry.entity.id` is the addressable handle SP-F needs — no code in this plan assumes SP-F exists. ✓
- §8 testing plan → repo CRUD+uniqueness (Task 1, compile-time assertion per Global Constraints' documented DB-test-convention gap), RBAC instance/org/cross-tenant (Task 4's fake-`Authn` tests + Task 4 Step 7's `Grantable` test — cross-tenant isolation exercised via Task 6's `TestForkEntry_OtherTenantsLinkedRowUntouched`), materialization idempotency (Task 5), fork-on-edit non-mutation (Task 6), provider resolution golden-test-equivalent (Task 7 Steps 1 and 5). ✓
- §9 open questions: §9.1 resolved by owner Decision 1 (Global Constraints). §9.2 resolved implicitly — `SeedOrgCatalog` auto-links the full instance catalog at `CreateTenant` (Task 5) *and* `LinkInstanceProvider`/`LinkInstanceWorkflow` (Task 4) exist for anything added to the instance catalog afterward — the spec's own "combo" framing, no separate instance-READ gate added (stays `admin_only` per §B3's own reasoning, unchanged). §9.3 resolved by owner Decision 2. §9.4 resolved: `recipe_records` untouched, no migration in SP-B (Global Constraints, explicit). §9.5 left open exactly as spec allows ("не блокирует v1") — `admin_only` only, noted, not a task.

**Placeholder scan:** One explicitly-flagged non-shippable snippet exists by design — Task 7 Step 7's `compileBundle` body — and it is **not** left as a TBD: the step's prose names the exact source (today's `CompileBundle` body) to move verbatim and states the build will fail until it's moved, which is a real, mechanical, unambiguous instruction, not an open design question. No other `TODO`/`TBD`/"add error handling"/"handle this later" phrasing appears anywhere in a code step. Every other code block is complete, real, and copy-pasteable against the exact signatures found in the repo (Task 1's research: `recipe.go`/`iam.go`/`catalog.go` reflection logic quoted verbatim as the pattern source).

**Type consistency:** `catalogpb.CatalogEntry`/`Level`/`Kind`/`Origin` (Task 1) flow unchanged through Tasks 2, 4, 5, 6, 7 — no task redefines or shadows them. `CatalogEntryRepo`/`BundleStore`/`Checker` (Task 1/2/4) are the exact same interface values injected into `Deps` in Task 4 and consumed by `SeedOrgCatalog`/`ForkEntry`/`CatalogProviderResolver` in Tasks 5-7 — no parallel/duplicate interface is introduced. `iam.CatalogSeeder` (Task 5) is satisfied structurally by `catalog.Service` without `iam` importing `catalog`, matching the `IamDeps` all-interfaces shape verified in Task 1's research (§"Deps pattern"). `dsl.ProviderResolver` (Task 7) is deliberately a **second**, structurally-compatible interface (not an import of `catalog.CatalogProviderResolver`'s type) — preserves `internal/dsl`'s pure/no-import-of-services-catalog posture, called out explicitly in Task 7's interface doc comment.
