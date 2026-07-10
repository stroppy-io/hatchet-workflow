package recipe

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/stroppy-io/schemapb/schemapb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/schema"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	catalogsvc "github.com/stroppy-io/stroppy-cloud/internal/services/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// recipeKindStr is the KindStr recipe bundles use for
// catalogsvc.BundleIdentity / internal/gitrepo.EntryRepoName — distinct from
// catalog's own "provider"/"workflow" kind strings so a recipe's repo can
// never collide with (or be mistaken for) a catalog entry repo of the same
// slug, even though both can land in the same tenant Gitea org
// (gitrepo.TenantOrg). A recipe is ALWAYS tenant-owned (IsInstanceLevel is
// always false below) — there is no LEVEL_INSTANCE recipe, mirroring
// requireTenant's own invariant one layer up.
const recipeKindStr = "recipe"

// recipeBundleIdentity builds the catalogsvc.BundleIdentity a recipe's
// Write/Read/heal call is scoped to: one Gitea repo per (tenantID, name),
// versions are commits on it — exactly the product decision this task
// implements, generalized from catalog's own (level, tenant, kind, slug)
// scoping to recipe's (tenant, name) scoping (a recipe has no level/kind
// facet, and "name" plays the role "slug" plays for a catalog item).
func recipeBundleIdentity(tenantID, name string) catalogsvc.BundleIdentity {
	return catalogsvc.BundleIdentity{
		IsInstanceLevel: false,
		TenantID:        tenantID,
		KindStr:         recipeKindStr,
		Slug:            name,
	}
}

// resolveBundle returns rec's actual file bytes: reads through s.d.Bundles
// at sourceRef when the row has already been migrated (sourceRef != "" —
// note this is s.d.Bundles.Read, the CURRENTLY configured store, whatever
// shape its refs are: a git-commit ref once GITEA_TOKEN is provisioned, a
// bare content hash otherwise — see recipeKindStr's doc on how Bundles is
// selected), or heals a legacy row (sourceRef == "", bytes still embedded in
// rec.Bundle.Files from the old plain-blob storage model) into that same
// store first — see healRecipeSourceRef's doc for why this is lazy-on-read
// rather than a boot-time migration pass. This is the ONLY function in this
// package that produces bundle bytes for CheckRecipe/LaunchFormSchema/
// StartRun: the compiler never reads rec.Bundle.Files directly off a
// freshly-Get-ed row, only through here, so there is exactly one path from
// "a stored recipe" to "bytes the DSL compiler sees" — never two.
func (s *Service) resolveBundle(ctx context.Context, rec *models.RecipeRecord, sourceRef string) (map[string][]byte, error) {
	if sourceRef != "" {
		return s.d.Bundles.Read(ctx, sourceRef)
	}
	return s.healRecipeSourceRef(ctx, rec, sourceRef)
}

// healRecipeSourceRef repairs a recipe_records row that predates the
// repo-per-recipe git backing: it writes the row's own already-loaded
// rec.Bundle.Files (the pre-migration storage model — the full bundle was
// simply embedded in the row's protojson blob) into a brand-new git repo via
// s.d.Bundles.Write, persists the resulting ref onto the row via
// s.d.Repo.UpdateSourceRef (which also strips Bundle.Files from the
// persisted blob, so from this point on the row's protojson stops
// duplicating what git now owns), and returns the files it just wrote.
//
// Deliberately lazy-on-read, exactly like catalog.healSourceRef: no separate
// migration command to remember to run, only rows a caller actually still
// reads are ever touched, and it is safe under a concurrent double-heal race
// — s.d.Bundles.Write's EnsureOrg/EnsureRepo are idempotent (see
// GitEntryBundleStore's own doc) and a second heal of the same row simply
// adds a second (empty-diff) commit and a second (also correct)
// UpdateSourceRef rather than corrupting anything.
//
// A sourceRef that is non-empty but NOT git-backed (should not happen in
// practice — recipe_records has never had any OTHER post-blob pre-git
// shape) is treated the same as "" : the row's own Bundle.Files, not the
// unrecognized ref, is trusted as the legacy source.
func (s *Service) healRecipeSourceRef(ctx context.Context, rec *models.RecipeRecord, _ string) (map[string][]byte, error) {
	files := rec.GetBundle().GetFiles()
	if len(files) == 0 {
		return nil, status.Error(codes.NotFound, "recipe bundle has no files to migrate")
	}
	id := recipeBundleIdentity(rec.GetEntity().GetTenantId(), rec.GetEntity().GetName())
	newRef, err := s.d.Bundles.Write(ctx, id, "", files)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "heal recipe bundle: migrate into git store: %v", err)
	}
	healed, ok := proto.Clone(rec).(*models.RecipeRecord)
	if !ok {
		return nil, status.Error(codes.Internal, "heal recipe bundle: cloned record has unexpected type")
	}
	healed.Bundle = &models.RecipeBundle{}
	if err := s.d.Repo.UpdateSourceRef(ctx, healed, newRef); err != nil {
		return nil, status.Errorf(codes.Internal, "heal recipe bundle: persist migrated ref: %v", err)
	}
	return files, nil
}

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

	// The bundle's bytes are written to git FIRST — the single canonical
	// store from here on (see resolveBundle's doc) — and only the resulting
	// ref, never the bytes themselves, is persisted onto the row: stored is
	// a clone with Bundle.Files stripped, so a fresh Create never
	// duplicates what git now owns (unlike a pre-migration legacy row,
	// which this Create path can no longer produce).
	bundleID := recipeBundleIdentity(req.GetTenantId(), name)
	sourceRef, err := s.d.Bundles.Write(ctx, bundleID, "", rec.GetBundle().GetFiles())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "store recipe bundle: %v", err)
	}
	stored, ok := proto.Clone(rec).(*models.RecipeRecord)
	if !ok {
		return nil, status.Error(codes.Internal, "create recipe: cloned record has unexpected type")
	}
	stored.Bundle = &models.RecipeBundle{}
	if err := s.d.Repo.Create(ctx, stored, sourceRef); err != nil {
		return nil, utils.MapErr(err)
	}
	// rec (NOT stored) is returned to the caller — it still carries the
	// bundle it was just created with, exactly the same response shape as
	// before this task, even though nothing byte-for-byte-identical is kept
	// in the row's own protojson anymore.
	return &api.CreateRecipeResponse{Recipe: rec}, nil
}

// GetRecipe reads one recipe record scoped to the tenant. NotFound covers
// both an absent row and a row owned by another tenant (the repo never
// returns a cross-tenant row).
func (s *Service) GetRecipe(ctx context.Context, req *api.GetRecipeRequest) (*api.GetRecipeResponse, error) {
	if err := requireTenant(req.GetTenantId()); err != nil {
		return nil, err
	}
	rec, sourceRef, err := s.d.Repo.Get(ctx, req.GetTenantId(), req.GetId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	files, err := s.resolveBundle(ctx, rec, sourceRef)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve recipe bundle: %v", err)
	}
	rec.Bundle = &models.RecipeBundle{Files: files}
	return &api.GetRecipeResponse{Recipe: rec}, nil
}

// ListRecipes returns every recipe record for the tenant. The table view
// this feeds renders off Summary (denormalized at Create time), not the raw
// bundle, so — unlike GetRecipe — this does not pay a git Read per row for
// an already-migrated recipe; it only heals a legacy row still carrying its
// bundle inline (a heal here is a Write of bytes already in hand, no extra
// Read needed either — see healRecipeSourceRef). An already-migrated row's
// Bundle comes back empty, exactly like an already-migrated CatalogEntry's
// bundle is never inlined into a List response.
func (s *Service) ListRecipes(ctx context.Context, req *api.ListRecipesRequest) (*api.ListRecipesResponse, error) {
	if err := requireTenant(req.GetTenantId()); err != nil {
		return nil, err
	}
	recs, refs, err := s.d.Repo.List(ctx, req.GetTenantId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	for i, rec := range recs {
		if refs[i] != "" {
			rec.Bundle = &models.RecipeBundle{}
			continue
		}
		if _, err := s.healRecipeSourceRef(ctx, rec, refs[i]); err != nil {
			return nil, status.Errorf(codes.Internal, "heal recipe bundle %q: %v", rec.GetEntity().GetId(), err)
		}
		rec.Bundle = &models.RecipeBundle{}
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
	rec, sourceRef, err := s.d.Repo.Get(ctx, req.GetTenantId(), req.GetId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	files, err := s.resolveBundle(ctx, rec, sourceRef)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve recipe bundle: %v", err)
	}
	diags, err := s.d.Checker(ctx, files)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.CheckRecipeResponse{Diagnostics: diags}, nil
}

// LaunchFormSchema composes and returns the launch-form schemapb.Schema for
// the tenant's stored recipe bundle. Read-only. Unlike CheckRecipe/Preview,
// a schema-composition failure (unresolvable provider, unparseable
// workflow.yaml) IS an RPC error (InvalidArgument): LaunchFormSchemaResponse
// carries no diagnostics channel, mirroring DslService.ComposedSchema's own
// contract for the same reason.
func (s *Service) LaunchFormSchema(ctx context.Context, req *api.LaunchFormSchemaRequest) (*api.LaunchFormSchemaResponse, error) {
	if err := requireTenant(req.GetTenantId()); err != nil {
		return nil, err
	}
	rec, sourceRef, err := s.d.Repo.Get(ctx, req.GetTenantId(), req.GetRecipeId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	files, err := s.resolveBundle(ctx, rec, sourceRef)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve recipe bundle: %v", err)
	}
	form, diags, err := s.d.FormSchema(ctx, files)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "compose launch form schema: %v", err)
	}
	if diags.HasErrors() {
		return nil, status.Errorf(codes.InvalidArgument, "compose launch form schema: %s", diags.String())
	}
	return &api.LaunchFormSchemaResponse{Schema: form}, nil
}

/*
StartRun launches a new run of an already-stored recipe bundle: it persists a
run record (models.Run — see the package doc's RunRepo/RecipeWorkflows
comments; SP-E Task 3 cut this over from models.TestRunRecord) and starts
RunRecipeWorkflow for it via s.d.Workflows.LaunchRecipeRun.

If the request carries a Filled payload (a submitted launch form — see
LaunchFormSchema), StartRun re-composes that same form schema server-side
and synchronously re-Bakes the submitted values (schema.BakeForm) before
minting anything: this is the trust boundary — the browser-side WASM Bake
only seals the payload for a smoother UX, it is never trusted on its own.
Because the composed form schema is STRICT (see dsl.ComposeLaunchFormSchema/
ComposeFormSchema), an undeclared key in Filled is rejected right here. A
blocking field error short-circuits before any run record is created:
StartRunResponse.field_errors comes back non-empty and Run unset, with no
side effects (no minted run, no launch). A clean bake seals a
*schemapb.Baked, threaded into s.d.Workflows.LaunchRecipeRun for
RunRecipeWorkflow to apply once it compiles the bundle (see
internal/dsl/schema.ApplyBakedInputs) — that consumption itself is a
follow-up task; StartRun's job ends at producing and passing the Baked.
Baked stays nil (today's behavior) when the request carries no Filled.

Name/Description carry the recipe's identity, WorkflowId is stamped with the
recipe record's id (see Run.workflow_id's own doc: "reserves the name for
SP-B's catalog Workflow") so ListRuns can filter by recipe and RunDetail/
rerun can trace the run back to the bundle that produced it, and
WorkflowVersion mirrors it durably as a stamped string (previously only
interpolated into Description's free text).

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

	recipeRec, sourceRef, err := s.d.Repo.Get(ctx, req.GetTenantId(), req.GetRecipeId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	// resolveBundle is the ONLY path from a stored recipe row to compiler-
	// facing bytes (see its own doc) — StartRun, like CheckRecipe/
	// LaunchFormSchema above, never reads recipeRec.GetBundle().GetFiles()
	// directly off the just-Get-ed row, so the launch it mints and the
	// FormSchema/Bake step immediately below it are guaranteed to compile
	// the exact same bytes, resolved through the exact same single storage
	// model, every time.
	bundleFiles, err := s.resolveBundle(ctx, recipeRec, sourceRef)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve recipe bundle: %v", err)
	}

	var baked *schemapb.Baked
	if filled := req.GetFilled(); filled != nil {
		form, diags, ferr := s.d.FormSchema(ctx, bundleFiles)
		if ferr != nil {
			return nil, status.Errorf(codes.InvalidArgument, "compose launch form schema: %v", ferr)
		}
		if diags.HasErrors() {
			return nil, status.Errorf(codes.InvalidArgument, "compose launch form schema: %s", diags.String())
		}
		var fieldErrors []*schemapb.FieldError
		var berr error
		baked, fieldErrors, berr = schema.BakeForm(form, filled.GetValues().AsMap())
		if berr != nil {
			return nil, status.Errorf(codes.InvalidArgument, "bake launch form: %v", berr)
		}
		if len(fieldErrors) > 0 && baked == nil {
			// A blocking field error means Bake returned no Baked — no run is
			// minted (see StartRunResponse.field_errors doc: "non-empty exactly
			// when filled failed BakeForm — the run is NOT created in that
			// case"). This is returned as a normal (err == nil) response, NOT
			// a bare RPC error: a bare status.Error would only give the
			// launch form a single opaque message, but LaunchFormRenderer
			// needs the field errors themselves — each with its own field
			// path — to surface a per-field message (its onInvalid prop).
			// fieldErrors alone can also be non-blocking warnings BakeForm
			// still sealed past — only treat this as fatal (no run minted)
			// when baked is nil.
			return &api.StartRunResponse{FieldErrors: fieldErrors}, nil
		}
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
		// Baked is the sealed launch-form snapshot this run was launched
		// with (nil for a non-form launch — the same condition that leaves
		// the local `baked` variable nil above). Stamped onto the persisted
		// run record itself (NOT just threaded into the Temporal workflow
		// input via LaunchRecipeRun below) so GetTestRunOverview's
		// snapshot.run carries it durably — this is what the rerun-prefill
		// UI (commit 82d6a8b0, /recipes/:id/launch?from=<runId>) reads to
		// prefill a form from a previous run. Before this fix, `baked` was
		// only ever passed as a side parameter to LaunchRecipeRun and never
		// written back onto `run`, so run_records.data never had a "baked"
		// key for any run, form-launched or not.
		Baked: baked,
	}

	if err := s.d.Runs.Create(ctx, run); err != nil {
		return nil, utils.MapErr(err)
	}

	// Launch is external IO: it MUST happen after the record commits. If
	// launch fails, close the already-visible record as FAILED so
	// list/overview do not expose an unrecoverable PENDING run forever —
	// mirrors test_run.StartTestRun's finishFailedRun.
	if err := s.d.Workflows.LaunchRecipeRun(ctx, run, bundleFiles, baked); err != nil {
		markRunFailed(run, s.now())
		if uerr := s.d.Runs.Update(ctx, run); uerr != nil {
			return nil, utils.MapErr(uerr)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &api.StartRunResponse{Run: run}, nil
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

	return &api.ListRunsResponse{Runs: runs, NextPageToken: nextPageToken}, nil
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
	latest, _, err := s.d.Repo.GetLatestByName(ctx, tenantID, name)
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
