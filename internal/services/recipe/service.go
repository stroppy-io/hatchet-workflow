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

// RunRepo persists the models.TestRunRecord StartRun mints for a recipe run.
// This deliberately reuses the same storage row shape (and, transitively,
// the same postgres.TestRunRepo the test_run service already writes through)
// so the run immediately participates in overview/metrics/logs without any
// new plumbing — see StartRun's doc comment for what a recipe run's record
// does and does not populate.
//
// Get/List/Delete mirror postgres.TestRunRepo's existing methods exactly
// (same signatures) so that repo satisfies this port without any new
// storage-layer code: List(ctx, query, callerAccountID) is the same
// full-filter ListTestRunsRequest query the (removed) test_run service used,
// reused here rather than adding a second, narrower list method — ListRuns
// passes a tenant-only query and filters the recipe_id facet in Go (see
// recipe.go's ListRuns), since ListTestRunsRequest has no recipe_id facet of
// its own.
type RunRepo interface {
	Create(ctx context.Context, run *models.TestRunRecord) error
	Update(ctx context.Context, run *models.TestRunRecord) error
	// Get returns the tenant-scoped run record, or derrors.ErrNotFound for an
	// absent row or one owned by another tenant.
	Get(ctx context.Context, tenantID, id string) (*models.TestRunRecord, error)
	// List returns the page of runs matching query (tenant-scoped) plus the
	// next-page token; callerAccountID fills each row's per-caller
	// is_favorite (see postgres.TestRunRepo.List).
	List(ctx context.Context, query *api.ListTestRunsRequest, callerAccountID string) ([]*models.TestRunRecord, string, error)
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
	LaunchRecipeRun(ctx context.Context, run *models.TestRunRecord, bundle map[string][]byte) error
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
	// Runs persists the TestRunRecord StartRun mints. Required for StartRun;
	// every other handler works without it.
	Runs RunRepo
	// Workflows launches RunRecipeWorkflow for a StartRun call. Required for
	// StartRun; every other handler works without it.
	Workflows RecipeWorkflows
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
