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
type RecipeRepo interface {
	Create(ctx context.Context, rec *models.RecipeRecord) error
	Get(ctx context.Context, tenantID, id string) (*models.RecipeRecord, error)
	List(ctx context.Context, tenantID string) ([]*models.RecipeRecord, error)
	// GetLatestByName returns the highest-version row for (tenantID, name),
	// or derrors.ErrNotFound when the tenant has no recipe of that name yet
	// — CreateRecipe uses this to compute the next version (latest+1, or 1
	// on NotFound).
	GetLatestByName(ctx context.Context, tenantID, name string) (*models.RecipeRecord, error)
	Delete(ctx context.Context, tenantID, id string) error
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

// NewService constructs the RecipeService connect handler.
func NewService(d Deps) *Service {
	return &Service{UnimplementedRecipeServiceServer: &api.UnimplementedRecipeServiceServer{}, d: d}
}
