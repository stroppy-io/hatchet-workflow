package execution

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/stroppy-io/schemapb/schemapb"
	"google.golang.org/protobuf/proto"

	databasecockroach "github.com/stroppy-io/stroppy-cloud/internal/domain/database/cockroach"
	databasemysql "github.com/stroppy-io/stroppy-cloud/internal/domain/database/mysql"
	databaseorioledb "github.com/stroppy-io/stroppy-cloud/internal/domain/database/orioledb"
	databasepgnoop "github.com/stroppy-io/stroppy-cloud/internal/domain/database/pgnoop"
	databasepicodata "github.com/stroppy-io/stroppy-cloud/internal/domain/database/picodata"
	databasepostgres "github.com/stroppy-io/stroppy-cloud/internal/domain/database/postgres"
	databaseydb "github.com/stroppy-io/stroppy-cloud/internal/domain/database/ydb"
	databaseydbmanaged "github.com/stroppy-io/stroppy-cloud/internal/domain/database/ydbmanaged"
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
			databaseydbmanaged.PackageResolver{},
			databasecockroach.PackageResolver{},
			databasemysql.PackageResolver{},
			databaseorioledb.PackageResolver{},
			databasepgnoop.PackageResolver{},
		),
		renderers: deploymentbuilder.NewRegistry(
			databasepostgres.DeploymentRenderer{},
			databasepicodata.DeploymentRenderer{},
			databaseydb.DeploymentRenderer{},
			databaseydbmanaged.DeploymentRenderer{},
			databasecockroach.DeploymentRenderer{},
			databasemysql.DeploymentRenderer{},
			databaseorioledb.DeploymentRenderer{},
			databasepgnoop.DeploymentRenderer{},
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

// InitialDraftFromRun builds the starting draft for a "New from run" clone: it
// seeds the editable values from an existing run's immutable baked spec so the
// user lands in the wizard with that run's exact parameters, free to edit before
// launching. Unlike a re-run (which relaunches the frozen spec verbatim), this
// produces a fresh, fully editable draft. Seeded: provider, database, workload,
// render overrides AND the per-node machine settings (provider VM/container
// sizing) — the baked infrastructure_plan.machines are MachinePlans keyed by
// topology node_id, the exact shape draft.machine_overrides expects, so the clone
// reproduces the same infrastructure. Compute regenerates topology identically
// (same db+workload+provider => same node_ids) and re-applies these overrides.
func (e *TestWizardEngine) InitialDraftFromRun(ctx context.Context, tenantID, name string, spec *domain.TestRun) (*models.TestWizardDraftRecord, error) {
	draft := &models.TestWizardDraftRecord{
		Provider: deployment.Provider_PROVIDER_UNSPECIFIED,
	}
	if spec != nil {
		if db := spec.GetDatabase(); db != nil {
			draft.Database = proto.Clone(db).(*domain.Database)
		}
		if wl := spec.GetWorkload(); wl != nil {
			draft.Workload = proto.Clone(wl).(*domain.Workload)
		}
		if ov := spec.GetRenderOverrides(); ov != nil {
			draft.RenderOverrides = proto.Clone(ov).(*deployment.RenderOverrideSet)
		}
		// Provider + per-node machine settings live on the baked
		// infrastructure_plan; seed both so the clone targets the same backend and
		// the same VM/container sizing by default.
		if plan := spec.GetInfrastructurePlan(); plan != nil {
			draft.Provider = plan.GetProvider()
			for _, m := range plan.GetMachines() {
				draft.MachineOverrides = append(draft.MachineOverrides, proto.Clone(m).(*deployment.MachinePlan))
			}
		}
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
	machineOverrides := infrastructurebuilder.BuildOptionsFromMachineOverrides(draft.GetProvider(), draft.GetMachineOverrides())

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
			if missing := missingMachineOverrideNodeIDs(draft.GetProvider(), run.GetInfrastructurePlan(), draft.GetMachineOverrides()); len(missing) > 0 {
				addErr("machine_overrides", fmt.Sprintf("confirm machine settings for every node: %s", strings.Join(missing, ", ")))
			}

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
		Infrastructure:  infrastructurebuilder.BuildOptionsFromMachineOverrides(draft.GetProvider(), draft.GetMachineOverrides()),
		RenderOverrides: draft.GetRenderOverrides(),
	})
}

func missingMachineOverrideNodeIDs(provider deployment.Provider, plan *deployment.InfrastructurePlan, overrides []*deployment.MachinePlan) []string {
	if plan == nil {
		return nil
	}
	confirmed := make(map[string]struct{}, len(overrides))
	for _, machine := range overrides {
		if machineOverrideMatchesProvider(provider, machine) {
			confirmed[machine.GetNodeId()] = struct{}{}
		}
	}
	var missing []string
	for _, machine := range plan.GetMachines() {
		if machine.GetNodeId() == "" {
			continue
		}
		if _, ok := confirmed[machine.GetNodeId()]; !ok {
			missing = append(missing, machine.GetNodeId())
		}
	}
	return missing
}

func machineOverrideMatchesProvider(provider deployment.Provider, machine *deployment.MachinePlan) bool {
	if machine.GetNodeId() == "" {
		return false
	}
	switch provider {
	case deployment.Provider_PROVIDER_DOCKER:
		return machine.GetDocker() != nil
	case deployment.Provider_PROVIDER_YANDEX:
		return machine.GetYandex() != nil
	default:
		return false
	}
}

func previewRunID(draft *models.TestWizardDraftRecord) string {
	if id := draft.GetEntity().GetId(); id != "" {
		return id
	}
	return "preview"
}
