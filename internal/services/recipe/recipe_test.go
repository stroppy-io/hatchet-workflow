package recipe

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

const clusterYAML = `version: 1
provider:
  use: yandex
machines:
  db:
    count: 3
    resources:
      cpu: 4
      ram: 8g
services:
  postgres:
    on: db
    image: postgres:16
`

func newBundle() *models.RecipeBundle {
	return &models.RecipeBundle{Files: map[string][]byte{"cluster.yaml": []byte(clusterYAML)}}
}

func TestCreateRecipePersistsFirstVersion(t *testing.T) {
	repo := newFakeRecipeRepo()
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil)})

	resp, err := svc.CreateRecipe(context.Background(), &api.CreateRecipeRequest{
		TenantId: "tenant-1",
		Recipe: &models.RecipeRecord{
			Entity: &common.Entity{Name: "pg-ha"},
			Bundle: newBundle(),
		},
	})
	if err != nil {
		t.Fatalf("create recipe: %v", err)
	}
	rec := resp.GetRecipe()
	if rec.GetEntity().GetId() == "" {
		t.Fatal("id was not stamped")
	}
	if rec.GetEntity().GetTenantId() != "tenant-1" {
		t.Fatalf("tenant_id = %q, want tenant-1", rec.GetEntity().GetTenantId())
	}
	if rec.GetVersion() != 1 {
		t.Fatalf("version = %d, want 1", rec.GetVersion())
	}
	if !rec.GetSummary().GetCompiles() {
		t.Fatal("summary.compiles = false, want true (stub checker returns no diagnostics)")
	}
	if len(repo.byTenantID["tenant-1"]) != 1 {
		t.Fatalf("repo has %d recipes, want 1", len(repo.byTenantID["tenant-1"]))
	}
}

func TestCreateRecipeSummaryReflectsChecker(t *testing.T) {
	repo := newFakeRecipeRepo()
	diags := []*dslpb.Diagnostic{{Severity: dslpb.Severity_SEVERITY_ERROR, Message: "boom"}}
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(diags)})

	resp, err := svc.CreateRecipe(context.Background(), &api.CreateRecipeRequest{
		TenantId: "tenant-1",
		Recipe: &models.RecipeRecord{
			Entity: &common.Entity{Name: "pg-ha"},
			Bundle: newBundle(),
		},
	})
	if err != nil {
		t.Fatalf("create recipe: %v", err)
	}
	if resp.GetRecipe().GetSummary().GetCompiles() {
		t.Fatal("summary.compiles = true, want false (stub checker returns a diagnostic)")
	}
}

func TestCreateRecipeSecondVersionIncrements(t *testing.T) {
	repo := newFakeRecipeRepo()
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil)})

	req := func() *api.CreateRecipeRequest {
		return &api.CreateRecipeRequest{
			TenantId: "tenant-1",
			Recipe: &models.RecipeRecord{
				Entity: &common.Entity{Name: "pg-ha"},
				Bundle: newBundle(),
			},
		}
	}

	first, err := svc.CreateRecipe(context.Background(), req())
	if err != nil {
		t.Fatalf("create recipe 1: %v", err)
	}
	if first.GetRecipe().GetVersion() != 1 {
		t.Fatalf("version = %d, want 1", first.GetRecipe().GetVersion())
	}

	second, err := svc.CreateRecipe(context.Background(), req())
	if err != nil {
		t.Fatalf("create recipe 2: %v", err)
	}
	if second.GetRecipe().GetVersion() != 2 {
		t.Fatalf("version = %d, want 2", second.GetRecipe().GetVersion())
	}
	if first.GetRecipe().GetEntity().GetId() == second.GetRecipe().GetEntity().GetId() {
		t.Fatal("second create reused the first id")
	}
}

func TestCreateRecipeMissingTenantIsInvalidArgument(t *testing.T) {
	repo := newFakeRecipeRepo()
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil)})

	_, err := svc.CreateRecipe(context.Background(), &api.CreateRecipeRequest{
		Recipe: &models.RecipeRecord{
			Entity: &common.Entity{Name: "pg-ha"},
			Bundle: newBundle(),
		},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.InvalidArgument, err)
	}
}

func TestGetRecipeRoundtrip(t *testing.T) {
	repo := newFakeRecipeRepo()
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil)})

	created, err := svc.CreateRecipe(context.Background(), &api.CreateRecipeRequest{
		TenantId: "tenant-1",
		Recipe: &models.RecipeRecord{
			Entity: &common.Entity{Name: "pg-ha"},
			Bundle: newBundle(),
		},
	})
	if err != nil {
		t.Fatalf("create recipe: %v", err)
	}

	resp, err := svc.GetRecipe(context.Background(), &api.GetRecipeRequest{
		TenantId: "tenant-1",
		Id:       created.GetRecipe().GetEntity().GetId(),
	})
	if err != nil {
		t.Fatalf("get recipe: %v", err)
	}
	if resp.GetRecipe().GetEntity().GetId() != created.GetRecipe().GetEntity().GetId() {
		t.Fatal("get recipe returned a different record")
	}
}

func TestGetRecipeMissingTenantIsInvalidArgument(t *testing.T) {
	repo := newFakeRecipeRepo()
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil)})

	_, err := svc.GetRecipe(context.Background(), &api.GetRecipeRequest{Id: "some-id"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.InvalidArgument, err)
	}
}

func TestListRecipes(t *testing.T) {
	repo := newFakeRecipeRepo()
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil)})

	for _, name := range []string{"pg-ha", "ydb-mirror"} {
		_, err := svc.CreateRecipe(context.Background(), &api.CreateRecipeRequest{
			TenantId: "tenant-1",
			Recipe: &models.RecipeRecord{
				Entity: &common.Entity{Name: name},
				Bundle: newBundle(),
			},
		})
		if err != nil {
			t.Fatalf("create recipe %s: %v", name, err)
		}
	}

	resp, err := svc.ListRecipes(context.Background(), &api.ListRecipesRequest{TenantId: "tenant-1"})
	if err != nil {
		t.Fatalf("list recipes: %v", err)
	}
	if len(resp.GetRecipes()) != 2 {
		t.Fatalf("recipes = %d, want 2", len(resp.GetRecipes()))
	}
}

func TestDeleteRecipeThenGetIsNotFound(t *testing.T) {
	repo := newFakeRecipeRepo()
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil)})

	created, err := svc.CreateRecipe(context.Background(), &api.CreateRecipeRequest{
		TenantId: "tenant-1",
		Recipe: &models.RecipeRecord{
			Entity: &common.Entity{Name: "pg-ha"},
			Bundle: newBundle(),
		},
	})
	if err != nil {
		t.Fatalf("create recipe: %v", err)
	}
	id := created.GetRecipe().GetEntity().GetId()

	if _, err := svc.DeleteRecipe(context.Background(), &api.DeleteRecipeRequest{TenantId: "tenant-1", Id: id}); err != nil {
		t.Fatalf("delete recipe: %v", err)
	}

	_, err = svc.GetRecipe(context.Background(), &api.GetRecipeRequest{TenantId: "tenant-1", Id: id})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.NotFound, err)
	}
}

func TestCheckRecipeReturnsCheckerDiagnostics(t *testing.T) {
	repo := newFakeRecipeRepo()
	diags := []*dslpb.Diagnostic{{Severity: dslpb.Severity_SEVERITY_WARNING, Message: "watch out"}}
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(diags)})

	created, err := svc.CreateRecipe(context.Background(), &api.CreateRecipeRequest{
		TenantId: "tenant-1",
		Recipe: &models.RecipeRecord{
			Entity: &common.Entity{Name: "pg-ha"},
			Bundle: newBundle(),
		},
	})
	if err != nil {
		t.Fatalf("create recipe: %v", err)
	}

	resp, err := svc.CheckRecipe(context.Background(), &api.CheckRecipeRequest{
		TenantId: "tenant-1",
		Id:       created.GetRecipe().GetEntity().GetId(),
	})
	if err != nil {
		t.Fatalf("check recipe: %v", err)
	}
	if len(resp.GetDiagnostics()) != 1 || resp.GetDiagnostics()[0].GetMessage() != "watch out" {
		t.Fatalf("diagnostics = %v, want the stub diagnostic", resp.GetDiagnostics())
	}
}

func TestCheckRecipeMissingTenantIsInvalidArgument(t *testing.T) {
	repo := newFakeRecipeRepo()
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil)})

	_, err := svc.CheckRecipe(context.Background(), &api.CheckRecipeRequest{Id: "some-id"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.InvalidArgument, err)
	}
}

func TestDeleteRecipeMissingTenantIsInvalidArgument(t *testing.T) {
	repo := newFakeRecipeRepo()
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil)})

	_, err := svc.DeleteRecipe(context.Background(), &api.DeleteRecipeRequest{Id: "some-id"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.InvalidArgument, err)
	}
}

func TestListRecipesMissingTenantIsInvalidArgument(t *testing.T) {
	repo := newFakeRecipeRepo()
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil)})

	_, err := svc.ListRecipes(context.Background(), &api.ListRecipesRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.InvalidArgument, err)
	}
}

/*
	===== test doubles =====
*/

func stubChecker(diags []*dslpb.Diagnostic) func(context.Context, map[string][]byte) ([]*dslpb.Diagnostic, error) {
	return func(context.Context, map[string][]byte) ([]*dslpb.Diagnostic, error) {
		return diags, nil
	}
}

type fakeAuthn struct{}

func (fakeAuthn) Caller(context.Context) (*iam.AccessClaims, error) {
	return &iam.AccessClaims{AccountId: "account-1"}, nil
}

// fakeRecipeRepo is an in-memory RecipeRepo keyed by (tenantID, id), with a
// name index for GetLatestByName.
type fakeRecipeRepo struct {
	byTenantID map[string]map[string]*models.RecipeRecord
}

func newFakeRecipeRepo() *fakeRecipeRepo {
	return &fakeRecipeRepo{byTenantID: map[string]map[string]*models.RecipeRecord{}}
}

func (r *fakeRecipeRepo) Create(_ context.Context, rec *models.RecipeRecord) error {
	tenantID := rec.GetEntity().GetTenantId()
	if r.byTenantID[tenantID] == nil {
		r.byTenantID[tenantID] = map[string]*models.RecipeRecord{}
	}
	r.byTenantID[tenantID][rec.GetEntity().GetId()] = rec
	return nil
}

func (r *fakeRecipeRepo) Get(_ context.Context, tenantID, id string) (*models.RecipeRecord, error) {
	rec, ok := r.byTenantID[tenantID][id]
	if !ok {
		return nil, derrors.NotFound("recipe", "recipe not found")
	}
	return rec, nil
}

func (r *fakeRecipeRepo) List(_ context.Context, tenantID string) ([]*models.RecipeRecord, error) {
	out := make([]*models.RecipeRecord, 0, len(r.byTenantID[tenantID]))
	for _, rec := range r.byTenantID[tenantID] {
		out = append(out, rec)
	}
	return out, nil
}

func (r *fakeRecipeRepo) GetLatestByName(_ context.Context, tenantID, name string) (*models.RecipeRecord, error) {
	var latest *models.RecipeRecord
	for _, rec := range r.byTenantID[tenantID] {
		if rec.GetEntity().GetName() != name {
			continue
		}
		if latest == nil || rec.GetVersion() > latest.GetVersion() {
			latest = rec
		}
	}
	if latest == nil {
		return nil, derrors.NotFound("recipe", "recipe not found")
	}
	return latest, nil
}

func (r *fakeRecipeRepo) Delete(_ context.Context, tenantID, id string) error {
	if _, ok := r.byTenantID[tenantID][id]; !ok {
		return derrors.NotFound("recipe", "recipe not found")
	}
	delete(r.byTenantID[tenantID], id)
	return nil
}
