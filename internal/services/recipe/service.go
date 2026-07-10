// Package recipe implements the RecipeService connect handler: tenant-scoped
// CRUD over persisted DSL recipe bundles (models.RecipeRecord), plus
// CheckRecipe, which re-runs the DSL compiler in check-mode over an
// already-stored bundle. It owns no storage or compilation of its own —
// persistence goes through RecipeRepo and compilation goes through the
// injected Checker (in production, internal/services/dsl.CheckBundle),
// following the same Deps/port shape as the other tenant-scoped CRUD
// services in this directory.
package recipe

import (
	"context"

	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	catalogsvc "github.com/stroppy-io/stroppy-cloud/internal/services/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	===== Dependency interfaces (constructor-injected) =====

	AUTHZ NOTE: RBAC (RESOURCE_RECIPE) is enforced by the auth interceptor
	reading the proto annotations — handlers do NOT re-check the annotated
	permission. Handlers DO: resolve caller identity (for CreateRecipe's
	author stamp), validate the request's tenant_id is present, and return
	correct gRPC codes via utils.MapErr.
*/

// RecipeRepo persists models.RecipeRecord rows. Every method is
// tenant-scoped: a row is addressed by (tenantID, id) so a caller can never
// read or mutate another tenant's recipe. Get/Delete return
// derrors.ErrNotFound when the (tenantID,id) pair has no row; Create returns
// derrors.ErrConflict on a duplicate (tenantID, name, version). This is the
// port the postgres RecipeRepo (Task 3) satisfies.
//
// A row's file bytes live in git, not in this repo (see recipe.go's
// resolveBundle / recipeBundleIdentity doc): every read method returns a
// source_ref string alongside the record — "" for a pre-git-migration
// legacy row, non-"" once healed (see healRecipeSourceRef) — and the
// service layer, never this port, resolves that ref into actual file bytes
// via a BundleStore. This mirrors internal/services/catalog's
// CatalogEntryRepo/BundleStore split exactly, generalized to recipe's own
// (tenantID, name, version) scoping instead of catalog's (level, tenant,
// kind, slug, version).
type RecipeRepo interface {
	// Create persists rec (with rec.Bundle.Files left however the caller set
	// it — the postgres impl never re-derives sourceRef from it) plus
	// sourceRef, the ref its bundle bytes were already written under via
	// BundleStore.Write.
	Create(ctx context.Context, rec *models.RecipeRecord, sourceRef string) error
	Get(ctx context.Context, tenantID, id string) (*models.RecipeRecord, string, error)
	List(ctx context.Context, tenantID string) ([]*models.RecipeRecord, []string, error)
	// GetLatestByName returns the highest-version row for (tenantID, name),
	// or derrors.ErrNotFound when the tenant has no recipe of that name yet
	// — CreateRecipe uses this to compute the next version (latest+1, or 1
	// on NotFound).
	GetLatestByName(ctx context.Context, tenantID, name string) (*models.RecipeRecord, string, error)
	Delete(ctx context.Context, tenantID, id string) error
	// UpdateSourceRef persists a healed sourceRef (and rec's refreshed
	// envelope, Bundle.Files already stripped by the caller) onto an
	// existing row — the lazy-migration write path, never called on a
	// freshly Created row.
	UpdateSourceRef(ctx context.Context, rec *models.RecipeRecord, sourceRef string) error
}

// RunRepo persists the models.Run StartRun mints for a recipe run — SP-E
// Task 3's cutover: this used to reuse models.TestRunRecord/TestRunRepo, but
// RunRecipeWorkflow's write path now targets models.Run/run_records (see
// postgres.RunRepo, backing this port via Store.Runs()).
//
// Get/List/Delete mirror postgres.RunRepo's existing methods exactly (same
// signatures) so that repo satisfies this port without any new
// storage-layer code: List(ctx, query, callerAccountID) is the same
// full-filter ListTestRunsRequest query TestRunRepo.List used, reused here
// rather than adding a second, narrower list method — ListRuns passes a
// tenant-only query and filters the workflow_id facet in Go (see recipe.go's
// ListRuns), since ListTestRunsRequest has no workflow_id/recipe_id facet of
// its own.
type RunRepo interface {
	Create(ctx context.Context, run *models.Run) error
	Update(ctx context.Context, run *models.Run) error
	// Get returns the tenant-scoped run record, or derrors.ErrNotFound for an
	// absent row or one owned by another tenant.
	Get(ctx context.Context, tenantID, id string) (*models.Run, error)
	// List returns the page of runs matching query (tenant-scoped) plus the
	// next-page token; callerAccountID fills each row's per-caller
	// is_favorite (see postgres.RunRepo.List).
	List(ctx context.Context, query *api.ListTestRunsRequest, callerAccountID string) ([]*models.Run, string, error)
	// Delete removes the tenant-scoped run record, or derrors.ErrNotFound for
	// an absent row or one owned by another tenant.
	Delete(ctx context.Context, tenantID, id string) error
}

// RecipeWorkflows launches and cancels the per-run RunRecipeWorkflow
// (internal/workflows/runrecipe.go) for a persisted recipe run. Launch is
// started AFTER the run record is committed — it is external IO and must
// not run inside an ambient transaction — mirroring test_run.Workflows.
// LaunchTest's own contract. bundle is the recipe's raw files
// (models.RecipeBundle.Files); the implementation resolves everything else
// (agent bootstrap, task queue, workflow id) itself, the same way
// infrastructure/execution.TestWorkflows resolves bootstrap internally
// rather than taking it as a parameter.
type RecipeWorkflows interface {
	// baked is the sealed launch-form snapshot StartRun's synchronous
	// BakeForm step produced (nil for a launch with no Filled — today's
	// no-inputs behavior). It is threaded through so RunRecipeWorkflow can
	// eventually apply it before lowering (see internal/dsl/schema.
	// ApplyBakedInputs) — wiring baked into RunRecipeInput/
	// CompileRecipeActivity itself is a follow-up task; this interface only
	// carries it through the launch call.
	LaunchRecipeRun(ctx context.Context, run *models.Run, bundle map[string][]byte, baked *schemapb.Baked) error
	// CancelRecipeRun requests cancellation of the RunRecipeWorkflow bound to
	// runID (the same deterministic workflow id LaunchRecipeRun started).
	// Cancellation is asynchronous: the workflow's own cancel path persists
	// the resulting run status, this call only requests it.
	CancelRecipeRun(ctx context.Context, runID string) error
}

// Deps bundles every dependency for the constructor.
type Deps struct {
	Repo  RecipeRepo
	Authn utils.Authn
	// Bundles is the SP-C seam's BundleStore (see catalogsvc.BundleStore's
	// doc) recipe bundle bytes are written to and read from — the SAME
	// wiring seam catalogsvc.Deps.Bundles uses (FSBundleStore by default,
	// GitEntryBundleStore once GITEA_TOKEN is provisioned — see
	// internal/app/run.go). Recipes reuse this store rather than inventing a
	// second one: one repo per (tenant, recipe name), addressed via
	// catalogsvc.BundleIdentity{KindStr: recipeKindStr, ...} exactly like a
	// catalog item, just never IsInstanceLevel (a recipe is always
	// tenant-owned — there is no LEVEL_INSTANCE recipe). Required for every
	// handler that touches a bundle (Create/Get/List/Check/LaunchFormSchema/
	// StartRun).
	Bundles catalogsvc.BundleStore
	// Unlike catalog_entries (which had two superseded pre-current storage
	// shapes: a bare content hash, then a retired single-monorepo git
	// store — see catalogsvc.Deps.LegacyBundles's doc), a recipe_records
	// row has only ever had ONE pre-migration shape: its bundle bytes
	// embedded directly in data.bundle.files, decoded for free by
	// RecipeRepo.Get/List/GetLatestByName's normal protojson unmarshal. So
	// there is no separate LegacyBundles seam here — healRecipeSourceRef
	// heals straight from the already-loaded rec.Bundle.Files.
	// Checker runs the DSL check-mode compile pipeline over a bundle's files
	// and returns wire-shaped diagnostics. In production this is
	// internal/services/dsl.CheckBundle, injected so this package reuses the
	// path-traversal-safe provider derivation living there instead of
	// duplicating it. It never returns a Go error for a problem in the
	// bundle itself (see CheckBundle's doc comment) — a non-nil error here
	// means a genuine transport/compute failure and is mapped via
	// utils.MapErr.
	Checker func(ctx context.Context, files map[string][]byte) ([]*dslpb.Diagnostic, error)
	// Runs persists the models.Run StartRun mints. Required for StartRun;
	// every other handler works without it.
	Runs RunRepo
	// Workflows launches RunRecipeWorkflow for a StartRun call. Required for
	// StartRun; every other handler works without it.
	Workflows RecipeWorkflows
	// FormSchema composes the launch-form schemapb.Schema for a bundle's
	// files (workflow.inputs ⊕ provider.params). In production this is
	// internal/services/dsl.ComposeLaunchFormSchema, injected for the same
	// reason Checker is: reuse the path-traversal-safe provider-module
	// derivation living in internal/services/dsl instead of duplicating it.
	// Required for LaunchFormSchema; every other handler works without it.
	FormSchema func(ctx context.Context, files map[string][]byte) (*schemapb.Schema, diag.List, error)
}

// Service implements api.RecipeServiceServer.
type Service struct {
	*api.UnimplementedRecipeServiceServer
	d Deps
}

var _ api.RecipeServiceServer = (*Service)(nil)

// NewService constructs the RecipeService connect handler. A nil d.Bundles
// defaults to a fresh catalogsvc.MemoryBundleStore — every existing unit
// test in this package constructs Deps without a Bundles value (they only
// ever cared about RecipeRepo/RunRepo/RecipeWorkflows), and requiring every
// one of them to wire a store just to keep compiling would test nothing
// beyond "a store was provided". Production wiring (internal/app/run.go)
// always sets Deps.Bundles explicitly to the shared catalogBundles instance,
// so this default is never reached outside tests.
func NewService(d Deps) *Service {
	if d.Bundles == nil {
		d.Bundles = catalogsvc.NewMemoryBundleStore()
	}
	return &Service{UnimplementedRecipeServiceServer: &api.UnimplementedRecipeServiceServer{}, d: d}
}
