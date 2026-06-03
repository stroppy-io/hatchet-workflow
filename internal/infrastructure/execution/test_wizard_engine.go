package execution

import (
	"context"

	"github.com/google/uuid"
	"github.com/stroppy-io/schemapb/schemapb"
	"google.golang.org/protobuf/proto"

	databasecockroach "github.com/stroppy-io/stroppy-cloud/internal/domain/database/cockroach"
	databasemysql "github.com/stroppy-io/stroppy-cloud/internal/domain/database/mysql"
	databasepicodata "github.com/stroppy-io/stroppy-cloud/internal/domain/database/picodata"
	databasepostgres "github.com/stroppy-io/stroppy-cloud/internal/domain/database/postgres"
	databaseydb "github.com/stroppy-io/stroppy-cloud/internal/domain/database/ydb"
	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	infrastructurebuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/infrastructure"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/packages"
	runbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	workloadbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/workload"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_wizard"
)

// TestWizardEngine implements test_wizard.WizardEngine. It is the typed brain of
// the test wizard: it turns the draft's editable values (provider, database,
// workload, render_overrides) into the server-derived sections (topology_spec,
// infrastructure_plan, render_preview, errors, ready) via the real domain
// builders, and bakes a ready draft into a domain.TestRun. Pure: no IO, no
// storage — same editable values in => same derived state out (safe in a retried
// transaction).
type TestWizardEngine struct {
	packageResolver packages.Resolver
	renderers       deploymentbuilder.Registry
}

var _ test_wizard.WizardEngine = (*TestWizardEngine)(nil)

// NewTestWizardEngine builds the test_wizard.WizardEngine adapter wired to the
// default per-database builders + workload renderer (the same set the workflow
// worker registers), so the wizard preview matches what the workflow will render.
func NewTestWizardEngine() *TestWizardEngine {
	return &TestWizardEngine{
		packageResolver: packages.NewRegistry(
			databasepostgres.PackageResolver{},
			databasepicodata.PackageResolver{},
			databaseydb.PackageResolver{},
			databasecockroach.PackageResolver{},
			databasemysql.PackageResolver{},
		),
		renderers: deploymentbuilder.NewRegistry(
			databasepostgres.DeploymentRenderer{},
			databasepicodata.DeploymentRenderer{},
			databaseydb.DeploymentRenderer{},
			databasecockroach.DeploymentRenderer{},
			databasemysql.DeploymentRenderer{},
			workloadbuilder.DeploymentRenderer{},
		),
	}
}

// InitialDraft builds the starting draft for a fresh wizard, optionally seeded
// from an existing test preset (db+workload). The returned draft has its
// server-derived sections already computed for the seeded values.
func (e *TestWizardEngine) InitialDraft(ctx context.Context, tenantID, name string, seed *models.TestPresetRecord) (*models.TestWizardDraftRecord, error) {
	draft := &models.TestWizardDraftRecord{
		Provider: deployment.Provider_PROVIDER_UNSPECIFIED,
	}
	if test := seed.GetTest(); test != nil {
		draft.Database = proto.Clone(test.GetDatabase()).(*domain.Database)
		draft.Workload = proto.Clone(test.GetWorkload()).(*domain.Workload)
		draft.TestPresetId = seed.GetEntity().GetId()
	}
	if err := e.Compute(ctx, tenantID, draft); err != nil {
		return nil, err
	}
	return draft, nil
}

// Compute recomputes the server-derived sections IN PLACE from the draft's
// editable values. It regenerates topology_spec + infrastructure_plan (via the
// run builder), the render_preview (via the deployment preview builder), the
// authoritative errors and the ready flag. A build failure is reported as a
// field error rather than a hard error so the wizard can surface it to the user.
func (e *TestWizardEngine) Compute(_ context.Context, _ string, draft *models.TestWizardDraftRecord) error {
	machineOverrides := infrastructurebuilder.BuildOptionsFromPlanOverrides(draft.GetInfrastructurePlan())

	// Reset derived sections; they are wholly recomputed below.
	draft.TopologySpec = nil
	draft.InfrastructurePlan = nil
	draft.RenderPreview = nil
	draft.Errors = nil
	draft.Ready = false

	var errs []*schemapb.FieldError
	addErr := func(field, message string) {
		errs = append(errs, &schemapb.FieldError{Field: field, Message: message})
	}

	if draft.GetDatabase() == nil {
		addErr("database", "database is required")
	}
	if draft.GetWorkload() == nil {
		addErr("workload", "workload is required")
	}
	if draft.GetProvider() == deployment.Provider_PROVIDER_UNSPECIFIED {
		addErr("provider", "provider is required")
	}

	if len(errs) == 0 {
		run, err := runbuilder.BuildTestRun(runbuilder.BuildOptions{
			ID:              previewRunID(draft),
			Database:        draft.GetDatabase(),
			Workload:        draft.GetWorkload(),
			Provider:        draft.GetProvider(),
			Infrastructure:  machineOverrides,
			RenderOverrides: draft.GetRenderOverrides(),
		})
		if err != nil {
			addErr("topology", err.Error())
		} else {
			draft.TopologySpec = run.GetTopologySpec()
			draft.InfrastructurePlan = run.GetInfrastructurePlan()

			preview, err := deploymentbuilder.BuildPreview(run.GetTopologySpec(), deploymentbuilder.PreviewOptions{
				Database:        draft.GetDatabase(),
				Workload:        draft.GetWorkload(),
				PackageResolver: e.packageResolver,
				Renderers:       e.renderers,
				RenderOverrides: draft.GetRenderOverrides(),
			})
			if err != nil {
				addErr("render_preview", err.Error())
			} else {
				draft.RenderPreview = preview
			}
		}
	}

	draft.Errors = errs
	draft.Ready = len(errs) == 0
	return nil
}

// Bake turns a ready draft into the materialized domain.TestRun (database +
// workload + generated topology + infrastructure plan). It rejects a draft that
// is not ready. Start paths mint a fresh record id and overwrite this spec id;
// non-start callers still receive a valid baked TestRun.
func (e *TestWizardEngine) Bake(_ context.Context, draft *models.TestWizardDraftRecord) (*domain.TestRun, error) {
	if !draft.GetReady() {
		return nil, errDraftNotReady
	}
	return runbuilder.BuildTestRun(runbuilder.BuildOptions{
		ID:              uuid.NewString(),
		Database:        draft.GetDatabase(),
		Workload:        draft.GetWorkload(),
		Provider:        draft.GetProvider(),
		Infrastructure:  infrastructurebuilder.BuildOptionsFromPlanOverrides(draft.GetInfrastructurePlan()),
		RenderOverrides: draft.GetRenderOverrides(),
	})
}

func previewRunID(draft *models.TestWizardDraftRecord) string {
	if id := draft.GetEntity().GetId(); id != "" {
		return id
	}
	return "preview"
}
