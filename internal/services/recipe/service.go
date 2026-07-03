// Package recipe implements the RecipeService connect handler: tenant-scoped
// CRUD over persisted DSL recipe bundles (models.RecipeRecord), plus
// CheckRecipe, which re-runs the DSL compiler in check-mode over an
// already-stored bundle. It owns no storage or compilation of its own —
// persistence goes through RecipeRepo and compilation goes through the
// injected Checker (in production, internal/services/dsl.CheckBundle),
// mirroring internal/services/suite's Deps/port shape.
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
