package recipe

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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

// TestStartRunSetsRatingFlags asserts a recipe run defaults to the tenant
// leaderboard (in_tenant_rating) but NOT the cross-tenant/public global
// leaderboard (in_global_rating), matching TestRunRecord.Summary's own
// documented platform defaults ("tenant true, global false" — see
// tenant_settings.proto's default_in_tenant_rating/default_in_global_rating
// doc comments). Before this, StartRun left both flags at their proto zero
// value (false), so recipe runs never appeared on the tenant dashboard's
// "Top benchmarks" board — see internal/app/glue.go's ratingRunsLister,
// which reads exactly these two flags.
func TestStartRunSetsRatingFlags(t *testing.T) {
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
	if !run.GetInTenantRating() {
		t.Error("in_tenant_rating = false, want true (recipe runs must appear on the tenant leaderboard by default)")
	}
	if run.GetInGlobalRating() {
		t.Error("in_global_rating = true, want false (global/public leaderboard membership stays opt-in)")
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

// TestListRunsPageSizeIsRespectedAndNextPageTokenSurfaced is the regression
// test for the truncation bug this change fixes: ListRuns used to call
// Runs.List with a bare tenant-only ListTestRunsRequest (no page at all),
// so a tenant with more runs than TestRunRepo.List's default page size (50)
// could never see its older runs, and ListRunsResponse had no field to
// expose that more rows existed even if it had asked. This asserts (a) the
// handler passes ListRunsRequest.page through to Runs.List rather than
// dropping it, and (b) the previously-discarded next_page_token now reaches
// the response — a client can tell there is more to fetch and paginate to
// it, instead of silently only ever seeing the first page.
func TestListRunsPageSizeIsRespectedAndNextPageTokenSurfaced(t *testing.T) {
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
	recipeID := recipe.GetRecipe().GetEntity().GetId()

	const totalRuns = 3
	for i := 0; i < totalRuns; i++ {
		if _, err := svc.StartRun(context.Background(), &api.StartRunRequest{TenantId: "tenant-1", RecipeId: recipeID}); err != nil {
			t.Fatalf("start run %d: %v", i, err)
		}
	}

	// First page: page_size=2 must be threaded to Runs.List, so only 2 of
	// the 3 runs come back and next_page_token must be non-empty (proof the
	// handler no longer discards it via `runs, _, err := ...`).
	first, err := svc.ListRuns(context.Background(), &api.ListRunsRequest{
		TenantId: "tenant-1",
		Page:     &common.Page{Size: 2},
	})
	if err != nil {
		t.Fatalf("list runs page 1: %v", err)
	}
	if len(first.GetRuns()) != 2 {
		t.Fatalf("page 1 runs = %d, want 2 (page_size was not passed through to Runs.List)", len(first.GetRuns()))
	}
	if first.GetNextPageToken() == "" {
		t.Fatal("page 1 next_page_token = \"\", want non-empty (a third run still exists beyond this page)")
	}

	// Second page: passing the returned token back must reach the storage
	// layer and yield the remaining run, with an empty token signalling the
	// end — proof next_page_token round-trips rather than being a dead field.
	second, err := svc.ListRuns(context.Background(), &api.ListRunsRequest{
		TenantId: "tenant-1",
		Page:     &common.Page{Size: 2, Token: first.GetNextPageToken()},
	})
	if err != nil {
		t.Fatalf("list runs page 2: %v", err)
	}
	if len(second.GetRuns()) != 1 {
		t.Fatalf("page 2 runs = %d, want 1 (the third, previously-truncated run)", len(second.GetRuns()))
	}
	if second.GetNextPageToken() != "" {
		t.Fatalf("page 2 next_page_token = %q, want empty (no further pages)", second.GetNextPageToken())
	}

	seen := map[string]bool{}
	for _, run := range append(first.GetRuns(), second.GetRuns()...) {
		seen[run.GetEntity().GetId()] = true
	}
	if len(seen) != totalRuns {
		t.Fatalf("distinct runs across both pages = %d, want %d (no run silently dropped or duplicated)", len(seen), totalRuns)
	}
}

// TestListRunsRecipeIDFilterAppliesPerPage documents (and locks in) the v1
// tradeoff called out on ListRunsResponse.next_page_token: recipe_id
// filtering happens in Go over an already-paginated storage page, so a page
// can legitimately return zero matching runs while next_page_token is still
// non-empty. This is acceptable now that a client can tell there is more to
// fetch (before this fix there was no token at all, so a recipe_id filter
// landing beyond the first 50 tenant-runs silently returned empty with no
// error and no way to know more existed).
func TestListRunsRecipeIDFilterAppliesPerPage(t *testing.T) {
	repo := newFakeRecipeRepo()
	runs := newFakeRunRepo()
	svc := NewService(Deps{Repo: repo, Authn: fakeAuthn{}, Checker: stubChecker(nil), Runs: runs, Workflows: newFakeRecipeWorkflows(nil)})

	noise, err := svc.CreateRecipe(context.Background(), &api.CreateRecipeRequest{
		TenantId: "tenant-1",
		Recipe:   &models.RecipeRecord{Entity: &common.Entity{Name: "noise"}, Bundle: newBundle()},
	})
	if err != nil {
		t.Fatalf("create noise recipe: %v", err)
	}
	// Two runs of a recipe we do NOT filter on occupy the first page.
	for i := 0; i < 2; i++ {
		if _, err := svc.StartRun(context.Background(), &api.StartRunRequest{TenantId: "tenant-1", RecipeId: noise.GetRecipe().GetEntity().GetId()}); err != nil {
			t.Fatalf("start noise run %d: %v", i, err)
		}
	}

	target, err := svc.CreateRecipe(context.Background(), &api.CreateRecipeRequest{
		TenantId: "tenant-1",
		Recipe:   &models.RecipeRecord{Entity: &common.Entity{Name: "target"}, Bundle: newBundle()},
	})
	if err != nil {
		t.Fatalf("create target recipe: %v", err)
	}
	targetID := target.GetRecipe().GetEntity().GetId()
	// The run we actually want lands on the second storage page (page_size=2).
	if _, err := svc.StartRun(context.Background(), &api.StartRunRequest{TenantId: "tenant-1", RecipeId: targetID}); err != nil {
		t.Fatalf("start target run: %v", err)
	}

	firstPage, err := svc.ListRuns(context.Background(), &api.ListRunsRequest{
		TenantId: "tenant-1",
		RecipeId: targetID,
		Page:     &common.Page{Size: 2},
	})
	if err != nil {
		t.Fatalf("list runs page 1: %v", err)
	}
	if len(firstPage.GetRuns()) != 0 {
		t.Fatalf("page 1 filtered runs = %d, want 0 (target run is on page 2)", len(firstPage.GetRuns()))
	}
	if firstPage.GetNextPageToken() == "" {
		t.Fatal("page 1 next_page_token = \"\", want non-empty (the target run still exists on the next page)")
	}

	secondPage, err := svc.ListRuns(context.Background(), &api.ListRunsRequest{
		TenantId: "tenant-1",
		RecipeId: targetID,
		Page:     &common.Page{Size: 2, Token: firstPage.GetNextPageToken()},
	})
	if err != nil {
		t.Fatalf("list runs page 2: %v", err)
	}
	if len(secondPage.GetRuns()) != 1 || secondPage.GetRuns()[0].GetRecipeId() != targetID {
		t.Fatalf("page 2 filtered runs = %v, want exactly the target run", secondPage.GetRuns())
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

// fakeRunRepo is an in-memory RunRepo keyed by (tenantID, id). order tracks
// insertion order so List can honor page.size/page.token deterministically,
// mirroring postgres.TestRunRepo.List's offset-cursor pagination (see
// test_run_list.go's decodeOffsetToken/LIMIT size+1 dance): the fake encodes
// its cursor as a plain decimal offset rather than the real repo's opaque
// token, which is fine since nothing outside this fake ever inspects it.
type fakeRunRepo struct {
	byID  map[string]*models.TestRunRecord
	order []string
}

func newFakeRunRepo() *fakeRunRepo {
	return &fakeRunRepo{byID: map[string]*models.TestRunRecord{}}
}

func (r *fakeRunRepo) Create(_ context.Context, run *models.TestRunRecord) error {
	id := run.GetEntity().GetId()
	if _, exists := r.byID[id]; !exists {
		r.order = append(r.order, id)
	}
	r.byID[id] = run
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

// List honors tenant_id plus query.GetPage() (size + offset-token), ignoring
// every other ListTestRunsRequest facet: the recipe service applies its own
// recipe_id filter in Go over the returned page, matching how ListRuns calls
// this in production. When no page is set (page == nil or size == 0) it
// returns every matching row with an empty next token — the same shape the
// real repo falls back to with its own default page size, just without a
// truncation cap, since these fakes exist to exercise the handler rather
// than reproduce the storage layer's default LIMIT.
func (r *fakeRunRepo) List(_ context.Context, query *api.ListTestRunsRequest, _ string) ([]*models.TestRunRecord, string, error) {
	matched := make([]*models.TestRunRecord, 0, len(r.order))
	for _, id := range r.order {
		run := r.byID[id]
		if run.GetEntity().GetTenantId() == query.GetTenantId() {
			matched = append(matched, run)
		}
	}

	page := query.GetPage()
	if page == nil || page.GetSize() == 0 {
		return matched, "", nil
	}

	offset := 0
	if tok := page.GetToken(); tok != "" {
		o, err := strconv.Atoi(tok)
		if err != nil {
			return nil, "", fmt.Errorf("invalid fake page token %q: %w", tok, err)
		}
		offset = o
	}
	if offset >= len(matched) {
		return []*models.TestRunRecord{}, "", nil
	}

	end := offset + int(page.GetSize())
	nextToken := ""
	if end < len(matched) {
		nextToken = strconv.Itoa(end)
	} else {
		end = len(matched)
	}
	return matched[offset:end], nextToken, nil
}

// Delete returns derrors.ErrNotFound for an absent id or one owned by
// another tenant, mirroring postgres.TestRunRepo.Delete's contract.
func (r *fakeRunRepo) Delete(_ context.Context, tenantID, id string) error {
	run, ok := r.byID[id]
	if !ok || run.GetEntity().GetTenantId() != tenantID {
		return derrors.NotFound("test_run", "run not found")
	}
	delete(r.byID, id)
	for i, existing := range r.order {
		if existing == id {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
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
