package recipe

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
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

// defaultGrafanaDashboardUID is the Grafana dashboard StartRun stamps onto a
// freshly-minted Run's ObservabilityRefs.grafana_dashboard_uid (see that
// field's own doc comment: "v1: ... filled with ... the relay's existing
// hardcoded uid (grafana) at mint time"). There is no backend-side per-run
// Grafana settings RPC (the gateway reverse-proxies the embedded Grafana at
// a fixed /grafana sub-path — see internal/gateway/gateway.go's
// GrafanaBackend doc); the frontend's own dashboard-kind -> uid table (web/
// src/services/grafana.ts's DASHBOARD_UID) is the closest thing to an
// existing convention, and "workload" is its always-shown, run-scoped
// default dashboard. SP-F gives ObservabilityRefs real per-provider
// variance; this is the documented v1 placeholder.
const defaultGrafanaDashboardUID = "stroppy-metrics-v1"

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
StartRun launches a new run of an already-stored recipe bundle: it persists a
run record (models.Run — see the package doc's RunRepo/RecipeWorkflows
comments; SP-E Task 3 cut this over from models.TestRunRecord) and starts
RunRecipeWorkflow for it via s.d.Workflows.LaunchRecipeRun.

The minted Run deliberately carries no Baked/CompiledPlan at mint time:
Baked is nil until SP-D's generated-form launch path exists, and CompiledPlan
is filled later, durably, by RunRecipeWorkflow itself right after it compiles
the bundle (see internal/workflows/runtime.go's persistRunCompiledPlan) — not
here, since StartRun never compiles the bundle itself. Name/Description
carry the recipe's identity, WorkflowId is stamped with the recipe record's
id (see Run.workflow_id's own doc: "reserves the name for SP-B's catalog
Workflow") so ListRuns can filter by recipe and RunDetail/rerun can trace the
run back to the bundle that produced it, and WorkflowVersion mirrors it
durably as a stamped string (previously only interpolated into
Description's free text).

Not idempotent: each call mints a new run, exactly like
test_run.StartTestRun.
*/
func (s *Service) StartRun(ctx context.Context, req *api.StartRunRequest) (*api.StartRunResponse, error) {
	if err := requireTenant(req.GetTenantId()); err != nil {
		return nil, err
	}
	if req.GetRecipeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "recipe_id is required")
	}
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}

	recipeRec, err := s.d.Repo.Get(ctx, req.GetTenantId(), req.GetRecipeId())
	if err != nil {
		return nil, utils.MapErr(err)
	}

	runID := uuid.NewString()
	workflowVersion := strconv.FormatUint(uint64(recipeRec.GetVersion()), 10)
	run := &models.Run{
		Entity: &common.Entity{
			Id:          runID,
			TenantId:    req.GetTenantId(),
			Name:        recipeRec.GetEntity().GetName(),
			Description: "recipe run of " + recipeRec.GetEntity().GetId() + " v" + workflowVersion,
			AuthorId:    c.GetAccountId(),
			Timings: &common.Timings{
				CreatedAt: s.now(),
				UpdatedAt: s.now(),
			},
		},
		Status:          common.Status_STATUS_PENDING,
		Trigger:         common.Trigger_TRIGGER_API,
		WorkflowId:      recipeRec.GetEntity().GetId(),
		WorkflowVersion: workflowVersion,
		// Observability is filled at mint time (v1 — see ObservabilityRefs'
		// own doc comment): metrics/logs are keyed by the run's own entity
		// id (the runtime observations convention every logs.proto/
		// metrics.proto consumer already follows), grafana is the relay's
		// existing hardcoded dashboard uid (see defaultGrafanaDashboardUID).
		Observability: &models.ObservabilityRefs{
			MetricsQueryKey:     runID,
			LogsQueryKey:        runID,
			GrafanaDashboardUid: defaultGrafanaDashboardUID,
		},
		// Rating flags mirror Run.Summary's documented platform defaults
		// (tenant_settings.proto: default_in_tenant_rating/
		// default_in_global_rating doc comments) — "tenant true, global
		// false". Every recipe run counts toward its own tenant's
		// leaderboard/dashboard ("Top benchmarks") by default;
		// cross-tenant/public global-leaderboard membership stays an
		// explicit opt-in (there is no tenant-settings override wired here
		// yet — see task-1-report.md).
		InTenantRating: true,
		InGlobalRating: false,
	}

	if err := s.d.Runs.Create(ctx, run); err != nil {
		return nil, utils.MapErr(err)
	}

	// Launch is external IO: it MUST happen after the record commits. If
	// launch fails, close the already-visible record as FAILED so
	// list/overview do not expose an unrecoverable PENDING run forever —
	// mirrors test_run.StartTestRun's finishFailedRun.
	if err := s.d.Workflows.LaunchRecipeRun(ctx, run, recipeRec.GetBundle().GetFiles()); err != nil {
		markRunFailed(run, s.now())
		if uerr := s.d.Runs.Update(ctx, run); uerr != nil {
			return nil, utils.MapErr(uerr)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	// api.StartRunResponse.run is still models.TestRunRecord-typed —
	// protocols/cloud/v1/api/recipe.proto is not retyped to models.Run until
	// SP-E Task 5 (see that task's "Modify: protocols/cloud/v1/api/
	// recipe.proto (StartRunResponse.run, ListRunsResponse.runs field
	// types)" step). runToAPIResponse bridges the gap for the duration of
	// Tasks 3-4, mirroring execution.RunToTestRunRecord's identical
	// adapter (duplicated rather than imported: execution already imports
	// this package for RecipeWorkflows, so the reverse import would cycle).
	return &api.StartRunResponse{Run: runToAPIResponse(run)}, nil
}

// ListRuns returns one page of runs for the tenant, optionally narrowed to a
// single recipe. It reuses the same s.d.Runs.List that backs the postgres
// RunRepo, passing req.GetPage() straight through so the storage layer's
// own LIMIT/OFFSET pagination applies (rather than silently relying on
// RunRepo.List's default page size, which would otherwise cap every call at
// the first 50 tenant runs); the recipe_id filter (a facet
// ListTestRunsRequest has no field for) is then applied in Go against
// Run.workflow_id (renamed from TestRunRecord.recipe_id — see Run.workflow_id's
// own doc comment: same value, "reserves the name for SP-B's catalog
// Workflow").
//
// IMPORTANT recipe_id caveat: because the recipe_id filter runs in-process
// over the already-paginated storage page rather than in the storage query
// itself, it is a per-page filter, not a global one. A page can come back
// with zero matching runs while ListRunsResponse.next_page_token is still
// non-empty (the matching runs may live on a later tenant page). Callers
// that set recipe_id MUST keep calling ListRuns with the returned
// next_page_token until it is empty rather than stopping on the first empty
// runs page — otherwise they will silently miss older runs of that recipe,
// which is exactly the truncation bug this pagination fixes. Recipe run
// sets are expected to stay small enough per tenant that this remains a
// handful of pages in practice; a dedicated storage-side recipe_id facet
// (filtering inside the SQL query, not after it) is the proper follow-up if
// that stops being true.
func (s *Service) ListRuns(ctx context.Context, req *api.ListRunsRequest) (*api.ListRunsResponse, error) {
	if err := requireTenant(req.GetTenantId()); err != nil {
		return nil, err
	}
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}

	runs, nextPageToken, err := s.d.Runs.List(ctx, &api.ListTestRunsRequest{
		TenantId: req.GetTenantId(),
		Page:     req.GetPage(),
	}, c.GetAccountId())
	if err != nil {
		return nil, utils.MapErr(err)
	}

	if recipeID := req.GetRecipeId(); recipeID != "" {
		filtered := make([]*models.Run, 0, len(runs))
		for _, run := range runs {
			if run.GetWorkflowId() == recipeID {
				filtered = append(filtered, run)
			}
		}
		runs = filtered
	}

	// api.ListRunsResponse.runs is still models.TestRunRecord-typed until
	// SP-E Task 5 — see StartRun's identical runToAPIResponse note above.
	wireRuns := make([]*models.TestRunRecord, 0, len(runs))
	for _, run := range runs {
		wireRuns = append(wireRuns, runToAPIResponse(run))
	}

	return &api.ListRunsResponse{Runs: wireRuns, NextPageToken: nextPageToken}, nil
}

// CancelRun requests cancellation of an in-flight recipe run's
// RunRecipeWorkflow. It first confirms the run is owned by the caller's
// tenant (Runs.Get returns derrors.ErrNotFound for an absent or
// cross-tenant row, mapped to codes.NotFound) so a caller can never
// request cancellation of another tenant's run by guessing its id.
// Cancellation itself is asynchronous — this handler only triggers it; the
// workflow's own cancel path persists the resulting run status.
func (s *Service) CancelRun(ctx context.Context, req *api.CancelRunRequest) (*api.CancelRunResponse, error) {
	if err := requireTenant(req.GetTenantId()); err != nil {
		return nil, err
	}
	if req.GetRunId() == "" {
		return nil, status.Error(codes.InvalidArgument, "run_id is required")
	}

	if _, err := s.d.Runs.Get(ctx, req.GetTenantId(), req.GetRunId()); err != nil {
		return nil, utils.MapErr(err)
	}

	if err := s.d.Workflows.CancelRecipeRun(ctx, req.GetRunId()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &api.CancelRunResponse{}, nil
}

// DeleteRun removes a run record. Like CancelRun, it first confirms tenant
// ownership via Runs.Get before deleting, so a caller can never delete
// another tenant's run by guessing its id (Runs.Delete alone is already
// tenant-scoped, but a bare Delete would map an absent row and a
// cross-tenant row to the same NotFound either way — the explicit Get
// keeps the ownership check visible and consistent with CancelRun).
func (s *Service) DeleteRun(ctx context.Context, req *api.DeleteRunRequest) (*api.DeleteRunResponse, error) {
	if err := requireTenant(req.GetTenantId()); err != nil {
		return nil, err
	}
	if req.GetRunId() == "" {
		return nil, status.Error(codes.InvalidArgument, "run_id is required")
	}

	if _, err := s.d.Runs.Get(ctx, req.GetTenantId(), req.GetRunId()); err != nil {
		return nil, utils.MapErr(err)
	}
	if err := s.d.Runs.Delete(ctx, req.GetTenantId(), req.GetRunId()); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.DeleteRunResponse{}, nil
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

// markRunFailed closes a just-minted run record as FAILED in place, mirroring
// internal/services/test_run's own unexported markFailed (re-declared here
// rather than imported: that package exports no such helper, and StartRun
// already holds the record in memory so — unlike test_run.finishFailedRun —
// there is no need to re-fetch it from storage first). Retyped for SP-E
// Task 3's models.Run cutover (was *models.TestRunRecord).
func markRunFailed(rec *models.Run, now *timestamppb.Timestamp) {
	rec.Status = common.Status_STATUS_FAILED
	if rec.Summary == nil {
		rec.Summary = &models.Run_Summary{}
	}
	if rec.Summary.StartedAt == nil {
		rec.Summary.StartedAt = now
	}
	if rec.Summary.FinishedAt == nil {
		rec.Summary.FinishedAt = now
	}
	if start := rec.Summary.GetStartedAt(); start != nil && now != nil {
		d := now.AsTime().Sub(start.AsTime())
		if d < 0 {
			d = 0
		}
		rec.Summary.Duration = durationpb.New(d)
	}
	if rec.Entity.Timings == nil {
		rec.Entity.Timings = &common.Timings{CreatedAt: now}
	}
	rec.Entity.Timings.UpdatedAt = now
}

// runToAPIResponse adapts a persisted models.Run onto the models.TestRunRecord
// shape api.StartRunResponse.run / api.ListRunsResponse.runs still expect —
// protocols/cloud/v1/api/recipe.proto is not retyped to models.Run until
// SP-E Task 5. Deliberately lossy (mirrors
// internal/infrastructure/execution/run_shim.go's RunToTestRunRecord, which
// exists for the exact same reason on the OverviewReader side; duplicated
// here rather than imported since execution already imports this package for
// RecipeWorkflows — the reverse import would cycle). Delete both adapters
// once Task 5 lands and the wire types carry models.Run directly.
func runToAPIResponse(run *models.Run) *models.TestRunRecord {
	if run == nil {
		return nil
	}
	return &models.TestRunRecord{
		Entity:         run.GetEntity(),
		Status:         run.GetStatus(),
		Trigger:        run.GetTrigger(),
		InTenantRating: run.GetInTenantRating(),
		InGlobalRating: run.GetInGlobalRating(),
		Summary:        runSummaryToAPIResponse(run.GetSummary()),
		RuntimeState:   run.GetRuntimeState(),
		RecipeId:       run.GetWorkflowId(),
	}
}

// runSummaryToAPIResponse field-copies a models.Run_Summary onto a
// models.TestRunRecord_Summary — the two messages share an identical field
// set (see models/test_run.proto's Run.Summary doc: "same 15 fields as
// TestRunRecord.Summary"), so this is a straight, lossless copy.
func runSummaryToAPIResponse(s *models.Run_Summary) *models.TestRunRecord_Summary {
	if s == nil {
		return nil
	}
	return &models.TestRunRecord_Summary{
		DbKind:           s.GetDbKind(),
		DbPresetId:       s.GetDbPresetId(),
		DbPresetName:     s.GetDbPresetName(),
		WorkloadPresetId: s.GetWorkloadPresetId(),
		WorkloadName:     s.GetWorkloadName(),
		StroppyVersion:   s.GetStroppyVersion(),
		WorkloadProtocol: s.GetWorkloadProtocol(),
		TestPresetId:     s.GetTestPresetId(),
		TestPresetName:   s.GetTestPresetName(),
		TopologyLabel:    s.GetTopologyLabel(),
		NodeCount:        s.GetNodeCount(),
		Provider:         s.GetProvider(),
		ProgressPct:      s.GetProgressPct(),
		StartedAt:        s.GetStartedAt(),
		FinishedAt:       s.GetFinishedAt(),
		Duration:         s.GetDuration(),
	}
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
