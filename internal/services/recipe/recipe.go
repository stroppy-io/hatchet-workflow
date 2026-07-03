package recipe

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// clusterFile is the bundle-relative path of the DSL cluster document,
// matching internal/services/dsl's own constant (unexported there, so
// re-declared here rather than imported).
const clusterFile = "cluster.yaml"

/*
	===== Recipe records (CRUD + check) =====
*/

// CreateRecipe persists a new recipe bundle version owned by the caller. The
// server assigns entity.id / tenant_id / author / timings (any
// client-supplied values are overwritten) and computes version as one past
// the tenant's latest existing version for the same name (1 for a brand-new
// name). It runs Checker over the bundle and a light parse of cluster.yaml
// to fill the denormalized Summary before persisting.
func (s *Service) CreateRecipe(ctx context.Context, req *api.CreateRecipeRequest) (*api.CreateRecipeResponse, error) {
	if err := requireTenant(req.GetTenantId()); err != nil {
		return nil, err
	}
	rec := req.GetRecipe()
	if rec == nil || rec.GetBundle() == nil {
		return nil, status.Error(codes.InvalidArgument, "recipe bundle is required")
	}
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}

	name := rec.GetEntity().GetName()
	version, err := s.nextVersion(ctx, req.GetTenantId(), name)
	if err != nil {
		return nil, err
	}

	id := uuid.NewString()
	rec.Entity = &common.Entity{
		Id:          id,
		TenantId:    req.GetTenantId(),
		Name:        name,
		Description: rec.GetEntity().GetDescription(),
		AuthorId:    c.GetAccountId(),
		Timings: &common.Timings{
			CreatedAt: s.now(),
			UpdatedAt: s.now(),
		},
	}
	rec.Version = version

	diags, err := s.d.Checker(ctx, rec.GetBundle().GetFiles())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	rec.Summary = deriveSummary(rec.GetBundle(), diags)

	if err := s.d.Repo.Create(ctx, rec); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.CreateRecipeResponse{Recipe: rec}, nil
}

// GetRecipe reads one recipe record scoped to the tenant. NotFound covers
// both an absent row and a row owned by another tenant (the repo never
// returns a cross-tenant row).
func (s *Service) GetRecipe(ctx context.Context, req *api.GetRecipeRequest) (*api.GetRecipeResponse, error) {
	if err := requireTenant(req.GetTenantId()); err != nil {
		return nil, err
	}
	rec, err := s.d.Repo.Get(ctx, req.GetTenantId(), req.GetId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.GetRecipeResponse{Recipe: rec}, nil
}

// ListRecipes returns every recipe record for the tenant.
func (s *Service) ListRecipes(ctx context.Context, req *api.ListRecipesRequest) (*api.ListRecipesResponse, error) {
	if err := requireTenant(req.GetTenantId()); err != nil {
		return nil, err
	}
	recs, err := s.d.Repo.List(ctx, req.GetTenantId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListRecipesResponse{Recipes: recs}, nil
}

// DeleteRecipe removes a recipe record. Delegates idempotency to the repo:
// the postgres RecipeRepo (Task 3) hard-deletes by (tenantID, id) and
// returns derrors.ErrNotFound for an absent row, which this handler ignores
// so a repeated delete of an already-gone recipe is a no-op.
func (s *Service) DeleteRecipe(ctx context.Context, req *api.DeleteRecipeRequest) (*api.DeleteRecipeResponse, error) {
	if err := requireTenant(req.GetTenantId()); err != nil {
		return nil, err
	}
	if err := utils.MapErr(derrors.IgnoreNotFound(s.d.Repo.Delete(ctx, req.GetTenantId(), req.GetId()))); err != nil {
		return nil, err
	}
	return &api.DeleteRecipeResponse{}, nil
}

// CheckRecipe loads the stored bundle by id and re-runs Checker over it.
// Like internal/services/dsl.DslService.Check, it never returns an RPC error
// for a problem in the bundle itself — only for a transport/not-found
// failure (an absent record, or a genuine Checker failure).
func (s *Service) CheckRecipe(ctx context.Context, req *api.CheckRecipeRequest) (*api.CheckRecipeResponse, error) {
	if err := requireTenant(req.GetTenantId()); err != nil {
		return nil, err
	}
	rec, err := s.d.Repo.Get(ctx, req.GetTenantId(), req.GetId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	diags, err := s.d.Checker(ctx, rec.GetBundle().GetFiles())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.CheckRecipeResponse{Diagnostics: diags}, nil
}

/*
	===== helpers =====
*/

func (s *Service) caller(ctx context.Context) (*iam.AccessClaims, error) {
	c, err := s.d.Authn.Caller(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return c, nil
}

func (s *Service) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// requireTenant validates tenant_id is present. RBAC is enforced upstream by
// the auth interceptor, but every handler still needs a non-empty tenant_id
// to scope its repo call correctly (an empty tenant_id would silently query
// or stamp an unscoped/wrong partition instead of failing loudly).
func requireTenant(tenantID string) error {
	if tenantID == "" {
		return status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	return nil
}

// nextVersion computes the version CreateRecipe stamps on a new record: one
// past the tenant's latest existing version for name, or 1 when the tenant
// has no recipe of that name yet (GetLatestByName returns NotFound).
func (s *Service) nextVersion(ctx context.Context, tenantID, name string) (uint32, error) {
	latest, err := s.d.Repo.GetLatestByName(ctx, tenantID, name)
	if err != nil {
		if derrors.IgnoreNotFound(err) == nil {
			return 1, nil
		}
		return 0, utils.MapErr(err)
	}
	return latest.GetVersion() + 1, nil
}

// deriveSummary computes a RecipeRecord's denormalized Summary: compiles
// reflects whether the Checker run found any diagnostic, and
// provider/machine_group_count/service_count come from a light,
// error-tolerant parse of the bundle's cluster.yaml. A missing or
// unparseable cluster.yaml leaves the counts at their zero value rather than
// failing the create.
func deriveSummary(bundle *models.RecipeBundle, diags []*dslpb.Diagnostic) *models.RecipeRecord_Summary {
	summary := &models.RecipeRecord_Summary{Compiles: len(diags) == 0}

	src, ok := bundle.GetFiles()[clusterFile]
	if !ok {
		return summary
	}
	providerUse := peekProviderUse(src)
	doc, _ := ast.DecodeCluster(clusterFile, src, providerUse)
	if doc == nil {
		return summary
	}
	summary.Provider = doc.Provider.Use
	summary.MachineGroupCount = uint32(len(doc.Machines)) //nolint:gosec // bundle sizes are bounded (max 512 files); a machine-group count never approaches uint32's range.
	summary.ServiceCount = uint32(len(doc.Services))      //nolint:gosec // same bound as above.
	return summary
}

// peekProviderUse best-effort reads cluster.yaml's provider.use field
// without going through ast.DecodeCluster's strict decode (which itself
// needs the provider name up front — see its providerKey parameter). A
// missing/unparseable cluster.yaml, or one with no provider.use, yields ""
// and deriveSummary simply gets a doc with an empty Provider.Use; it never
// treats this as a Go error. Mirrors internal/services/dsl's unexported
// peekProviderUse (re-declared here rather than imported: that package
// exports no such helper today).
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
