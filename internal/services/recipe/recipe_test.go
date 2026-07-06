package recipe

import (
	"context"
	"errors"
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
	===== StartRun =====
*/

func TestStartRunPersistsRunAndLaunchesWorkflow(t *testing.T) {
	repo := newFakeRecipeRepo()
	runs := newFakeRunRepo()
	workflows := newFakeRecipeWorkflows(nil)
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil), Runs: runs, Workflows: workflows})

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
	recipeID := created.GetRecipe().GetEntity().GetId()

	resp, err := svc.StartRun(context.Background(), &api.StartRunRequest{TenantId: "tenant-1", RecipeId: recipeID})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	run := resp.GetRun()
	if run.GetEntity().GetId() == "" {
		t.Fatal("run id was not stamped")
	}
	if run.GetEntity().GetTenantId() != "tenant-1" {
		t.Fatalf("tenant_id = %q, want tenant-1", run.GetEntity().GetTenantId())
	}
	if run.GetStatus() != common.Status_STATUS_PENDING {
		t.Fatalf("status = %s, want PENDING (workflow launched synchronously by the fake and did not fail)", run.GetStatus())
	}
	if len(runs.byID) != 1 {
		t.Fatalf("run repo has %d rows, want 1", len(runs.byID))
	}
	if len(workflows.launched) != 1 {
		t.Fatalf("workflows launched %d times, want 1", len(workflows.launched))
	}
	launched := workflows.launched[0]
	if launched.run.GetEntity().GetId() != run.GetEntity().GetId() {
		t.Fatal("workflow was launched for a different run id than the one returned")
	}
	if string(launched.bundle["cluster.yaml"]) != clusterYAML {
		t.Fatal("workflow was launched with a different bundle than the recipe's")
	}
}

func TestStartRunLaunchFailureMarksRunFailed(t *testing.T) {
	repo := newFakeRecipeRepo()
	runs := newFakeRunRepo()
	launchErr := errors.New("temporal unavailable")
	workflows := newFakeRecipeWorkflows(launchErr)
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil), Runs: runs, Workflows: workflows})

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
	recipeID := created.GetRecipe().GetEntity().GetId()

	_, err = svc.StartRun(context.Background(), &api.StartRunRequest{TenantId: "tenant-1", RecipeId: recipeID})
	if status.Code(err) != codes.Internal {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.Internal, err)
	}
	if len(runs.byID) != 1 {
		t.Fatalf("run repo has %d rows, want 1 (the record must still be visible)", len(runs.byID))
	}
	var rec *models.TestRunRecord
	for _, r := range runs.byID {
		rec = r
	}
	if rec.GetStatus() != common.Status_STATUS_FAILED {
		t.Fatalf("status = %s, want FAILED", rec.GetStatus())
	}
}

func TestStartRunMissingTenantIsInvalidArgument(t *testing.T) {
	repo := newFakeRecipeRepo()
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil), Runs: newFakeRunRepo(), Workflows: newFakeRecipeWorkflows(nil)})

	_, err := svc.StartRun(context.Background(), &api.StartRunRequest{RecipeId: "some-id"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.InvalidArgument, err)
	}
}

func TestStartRunMissingRecipeIDIsInvalidArgument(t *testing.T) {
	repo := newFakeRecipeRepo()
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil), Runs: newFakeRunRepo(), Workflows: newFakeRecipeWorkflows(nil)})

	_, err := svc.StartRun(context.Background(), &api.StartRunRequest{TenantId: "tenant-1"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.InvalidArgument, err)
	}
}

func TestStartRunUnknownRecipeIsNotFound(t *testing.T) {
	repo := newFakeRecipeRepo()
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil), Runs: newFakeRunRepo(), Workflows: newFakeRecipeWorkflows(nil)})

	_, err := svc.StartRun(context.Background(), &api.StartRunRequest{TenantId: "tenant-1", RecipeId: "missing"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.NotFound, err)
	}
}

func TestStartRunStampsRecipeIDLabel(t *testing.T) {
	repo := newFakeRecipeRepo()
	runs := newFakeRunRepo()
	workflows := newFakeRecipeWorkflows(nil)
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil), Runs: runs, Workflows: workflows})

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
	recipeID := created.GetRecipe().GetEntity().GetId()

	resp, err := svc.StartRun(context.Background(), &api.StartRunRequest{TenantId: "tenant-1", RecipeId: recipeID})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if resp.GetRun().GetRecipeId() != recipeID {
		t.Fatalf("run.recipe_id = %q, want %q", resp.GetRun().GetRecipeId(), recipeID)
	}
}

/*
	===== ListRuns =====
*/

func TestListRunsReturnsTenantRuns(t *testing.T) {
	repo := newFakeRecipeRepo()
	runs := newFakeRunRepo()
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil), Runs: runs, Workflows: newFakeRecipeWorkflows(nil)})

	for _, tenantID := range []string{"tenant-1", "tenant-1", "tenant-2"} {
		recipe, err := svc.CreateRecipe(context.Background(), &api.CreateRecipeRequest{
			TenantId: tenantID,
			Recipe:   &models.RecipeRecord{Entity: &common.Entity{Name: "pg-ha"}, Bundle: newBundle()},
		})
		if err != nil {
			t.Fatalf("create recipe: %v", err)
		}
		if _, err := svc.StartRun(context.Background(), &api.StartRunRequest{
			TenantId: tenantID,
			RecipeId: recipe.GetRecipe().GetEntity().GetId(),
		}); err != nil {
			t.Fatalf("start run: %v", err)
		}
	}

	resp, err := svc.ListRuns(context.Background(), &api.ListRunsRequest{TenantId: "tenant-1"})
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(resp.GetRuns()) != 2 {
		t.Fatalf("runs = %d, want 2 (only tenant-1's)", len(resp.GetRuns()))
	}
	for _, run := range resp.GetRuns() {
		if run.GetEntity().GetTenantId() != "tenant-1" {
			t.Fatalf("run tenant_id = %q, want tenant-1", run.GetEntity().GetTenantId())
		}
	}
}

func TestListRunsFiltersByRecipeID(t *testing.T) {
	repo := newFakeRecipeRepo()
	runs := newFakeRunRepo()
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil), Runs: runs, Workflows: newFakeRecipeWorkflows(nil)})

	var recipeIDs []string
	for _, name := range []string{"pg-ha", "ydb-mirror"} {
		recipe, err := svc.CreateRecipe(context.Background(), &api.CreateRecipeRequest{
			TenantId: "tenant-1",
			Recipe:   &models.RecipeRecord{Entity: &common.Entity{Name: name}, Bundle: newBundle()},
		})
		if err != nil {
			t.Fatalf("create recipe %s: %v", name, err)
		}
		recipeIDs = append(recipeIDs, recipe.GetRecipe().GetEntity().GetId())
		if _, err := svc.StartRun(context.Background(), &api.StartRunRequest{
			TenantId: "tenant-1",
			RecipeId: recipe.GetRecipe().GetEntity().GetId(),
		}); err != nil {
			t.Fatalf("start run for %s: %v", name, err)
		}
	}

	resp, err := svc.ListRuns(context.Background(), &api.ListRunsRequest{TenantId: "tenant-1", RecipeId: recipeIDs[0]})
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(resp.GetRuns()) != 1 {
		t.Fatalf("runs = %d, want 1", len(resp.GetRuns()))
	}
	if resp.GetRuns()[0].GetRecipeId() != recipeIDs[0] {
		t.Fatalf("run.recipe_id = %q, want %q", resp.GetRuns()[0].GetRecipeId(), recipeIDs[0])
	}
}

func TestListRunsMissingTenantIsInvalidArgument(t *testing.T) {
	svc := NewService(Deps{Repo: newFakeRecipeRepo(), Authn: fakeAuthn{}, Checker: stubChecker(nil), Runs: newFakeRunRepo(), Workflows: newFakeRecipeWorkflows(nil)})

	_, err := svc.ListRuns(context.Background(), &api.ListRunsRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.InvalidArgument, err)
	}
}

/*
	===== CancelRun =====
*/

func TestCancelRunCallsCancelRecipeRunForOwnedRun(t *testing.T) {
	repo := newFakeRecipeRepo()
	runs := newFakeRunRepo()
	workflows := newFakeRecipeWorkflows(nil)
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil), Runs: runs, Workflows: workflows})

	recipe, err := svc.CreateRecipe(context.Background(), &api.CreateRecipeRequest{
		TenantId: "tenant-1",
		Recipe:   &models.RecipeRecord{Entity: &common.Entity{Name: "pg-ha"}, Bundle: newBundle()},
	})
	if err != nil {
		t.Fatalf("create recipe: %v", err)
	}
	started, err := svc.StartRun(context.Background(), &api.StartRunRequest{TenantId: "tenant-1", RecipeId: recipe.GetRecipe().GetEntity().GetId()})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	runID := started.GetRun().GetEntity().GetId()

	if _, err := svc.CancelRun(context.Background(), &api.CancelRunRequest{TenantId: "tenant-1", RunId: runID}); err != nil {
		t.Fatalf("cancel run: %v", err)
	}
	if len(workflows.cancelled) != 1 || workflows.cancelled[0] != runID {
		t.Fatalf("cancelled = %v, want [%s]", workflows.cancelled, runID)
	}
}

func TestCancelRunForeignRunIsNotFound(t *testing.T) {
	repo := newFakeRecipeRepo()
	runs := newFakeRunRepo()
	workflows := newFakeRecipeWorkflows(nil)
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil), Runs: runs, Workflows: workflows})

	recipe, err := svc.CreateRecipe(context.Background(), &api.CreateRecipeRequest{
		TenantId: "tenant-1",
		Recipe:   &models.RecipeRecord{Entity: &common.Entity{Name: "pg-ha"}, Bundle: newBundle()},
	})
	if err != nil {
		t.Fatalf("create recipe: %v", err)
	}
	started, err := svc.StartRun(context.Background(), &api.StartRunRequest{TenantId: "tenant-1", RecipeId: recipe.GetRecipe().GetEntity().GetId()})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	runID := started.GetRun().GetEntity().GetId()

	_, err = svc.CancelRun(context.Background(), &api.CancelRunRequest{TenantId: "tenant-2", RunId: runID})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.NotFound, err)
	}
	if len(workflows.cancelled) != 0 {
		t.Fatalf("cancelled = %v, want none (foreign tenant must not trigger cancel)", workflows.cancelled)
	}
}

func TestCancelRunAbsentRunIsNotFound(t *testing.T) {
	svc := NewService(Deps{Repo: newFakeRecipeRepo(), Authn: fakeAuthn{}, Checker: stubChecker(nil), Runs: newFakeRunRepo(), Workflows: newFakeRecipeWorkflows(nil)})

	_, err := svc.CancelRun(context.Background(), &api.CancelRunRequest{TenantId: "tenant-1", RunId: "missing"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.NotFound, err)
	}
}

func TestCancelRunMissingTenantIsInvalidArgument(t *testing.T) {
	svc := NewService(Deps{Repo: newFakeRecipeRepo(), Authn: fakeAuthn{}, Checker: stubChecker(nil), Runs: newFakeRunRepo(), Workflows: newFakeRecipeWorkflows(nil)})

	_, err := svc.CancelRun(context.Background(), &api.CancelRunRequest{RunId: "some-id"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.InvalidArgument, err)
	}
}

/*
	===== DeleteRun =====
*/

func TestDeleteRunRemovesOwnedRun(t *testing.T) {
	repo := newFakeRecipeRepo()
	runs := newFakeRunRepo()
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil), Runs: runs, Workflows: newFakeRecipeWorkflows(nil)})

	recipe, err := svc.CreateRecipe(context.Background(), &api.CreateRecipeRequest{
		TenantId: "tenant-1",
		Recipe:   &models.RecipeRecord{Entity: &common.Entity{Name: "pg-ha"}, Bundle: newBundle()},
	})
	if err != nil {
		t.Fatalf("create recipe: %v", err)
	}
	started, err := svc.StartRun(context.Background(), &api.StartRunRequest{TenantId: "tenant-1", RecipeId: recipe.GetRecipe().GetEntity().GetId()})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	runID := started.GetRun().GetEntity().GetId()

	if _, err := svc.DeleteRun(context.Background(), &api.DeleteRunRequest{TenantId: "tenant-1", RunId: runID}); err != nil {
		t.Fatalf("delete run: %v", err)
	}
	if len(runs.byID) != 0 {
		t.Fatalf("run repo has %d rows, want 0", len(runs.byID))
	}
}

func TestDeleteRunForeignRunIsNotFound(t *testing.T) {
	repo := newFakeRecipeRepo()
	runs := newFakeRunRepo()
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil), Runs: runs, Workflows: newFakeRecipeWorkflows(nil)})

	recipe, err := svc.CreateRecipe(context.Background(), &api.CreateRecipeRequest{
		TenantId: "tenant-1",
		Recipe:   &models.RecipeRecord{Entity: &common.Entity{Name: "pg-ha"}, Bundle: newBundle()},
	})
	if err != nil {
		t.Fatalf("create recipe: %v", err)
	}
	started, err := svc.StartRun(context.Background(), &api.StartRunRequest{TenantId: "tenant-1", RecipeId: recipe.GetRecipe().GetEntity().GetId()})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	runID := started.GetRun().GetEntity().GetId()

	_, err = svc.DeleteRun(context.Background(), &api.DeleteRunRequest{TenantId: "tenant-2", RunId: runID})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.NotFound, err)
	}
	if len(runs.byID) != 1 {
		t.Fatalf("run repo has %d rows, want 1 (foreign-tenant delete must not remove it)", len(runs.byID))
	}
}

func TestDeleteRunAbsentRunIsNotFound(t *testing.T) {
	svc := NewService(Deps{Repo: newFakeRecipeRepo(), Authn: fakeAuthn{}, Checker: stubChecker(nil), Runs: newFakeRunRepo(), Workflows: newFakeRecipeWorkflows(nil)})

	_, err := svc.DeleteRun(context.Background(), &api.DeleteRunRequest{TenantId: "tenant-1", RunId: "missing"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.NotFound, err)
	}
}

func TestDeleteRunMissingTenantIsInvalidArgument(t *testing.T) {
	svc := NewService(Deps{Repo: newFakeRecipeRepo(), Authn: fakeAuthn{}, Checker: stubChecker(nil), Runs: newFakeRunRepo(), Workflows: newFakeRecipeWorkflows(nil)})

	_, err := svc.DeleteRun(context.Background(), &api.DeleteRunRequest{RunId: "some-id"})
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

// fakeRunRepo is an in-memory RunRepo keyed by (tenantID, id).
type fakeRunRepo struct {
	byID map[string]*models.TestRunRecord
}

func newFakeRunRepo() *fakeRunRepo {
	return &fakeRunRepo{byID: map[string]*models.TestRunRecord{}}
}

func (r *fakeRunRepo) Create(_ context.Context, run *models.TestRunRecord) error {
	r.byID[run.GetEntity().GetId()] = run
	return nil
}

func (r *fakeRunRepo) Update(_ context.Context, run *models.TestRunRecord) error {
	if _, ok := r.byID[run.GetEntity().GetId()]; !ok {
		return derrors.NotFound("test_run", "run not found")
	}
	r.byID[run.GetEntity().GetId()] = run
	return nil
}

// Get returns derrors.ErrNotFound for an absent id or one owned by another
// tenant, mirroring postgres.TestRunRepo.Get's tenant-scoping contract.
func (r *fakeRunRepo) Get(_ context.Context, tenantID, id string) (*models.TestRunRecord, error) {
	run, ok := r.byID[id]
	if !ok || run.GetEntity().GetTenantId() != tenantID {
		return nil, derrors.NotFound("test_run", "run not found")
	}
	return run, nil
}

// List ignores every ListTestRunsRequest facet except tenant_id: the recipe
// service applies its own recipe_id filter in Go over the tenant's full run
// set, matching how ListRuns calls this in production.
func (r *fakeRunRepo) List(_ context.Context, query *api.ListTestRunsRequest, _ string) ([]*models.TestRunRecord, string, error) {
	out := make([]*models.TestRunRecord, 0, len(r.byID))
	for _, run := range r.byID {
		if run.GetEntity().GetTenantId() == query.GetTenantId() {
			out = append(out, run)
		}
	}
	return out, "", nil
}

// Delete returns derrors.ErrNotFound for an absent id or one owned by
// another tenant, mirroring postgres.TestRunRepo.Delete's contract.
func (r *fakeRunRepo) Delete(_ context.Context, tenantID, id string) error {
	run, ok := r.byID[id]
	if !ok || run.GetEntity().GetTenantId() != tenantID {
		return derrors.NotFound("test_run", "run not found")
	}
	delete(r.byID, id)
	return nil
}

// launchedRecipeRun captures one LaunchRecipeRun call for assertions.
type launchedRecipeRun struct {
	run    *models.TestRunRecord
	bundle map[string][]byte
}

// fakeRecipeWorkflows is a RecipeWorkflows double that records every launch
// and cancel call, returning launchErr (nil for success) from
// LaunchRecipeRun and cancelErr (nil for success) from CancelRecipeRun.
type fakeRecipeWorkflows struct {
	launchErr error
	cancelErr error
	launched  []launchedRecipeRun
	cancelled []string
}

func newFakeRecipeWorkflows(launchErr error) *fakeRecipeWorkflows {
	return &fakeRecipeWorkflows{launchErr: launchErr}
}

func (w *fakeRecipeWorkflows) LaunchRecipeRun(_ context.Context, run *models.TestRunRecord, bundle map[string][]byte) error {
	w.launched = append(w.launched, launchedRecipeRun{run: run, bundle: bundle})
	return w.launchErr
}

func (w *fakeRecipeWorkflows) CancelRecipeRun(_ context.Context, runID string) error {
	w.cancelled = append(w.cancelled, runID)
	return w.cancelErr
}
