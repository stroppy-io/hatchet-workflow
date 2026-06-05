package adapters

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/stroppy-io/schemapb/schemapb"
	"google.golang.org/protobuf/proto"

	databasecockroach "github.com/stroppy-io/stroppy-cloud/internal/domain/database/cockroach"
	databasemysql "github.com/stroppy-io/stroppy-cloud/internal/domain/database/mysql"
	databasepicodata "github.com/stroppy-io/stroppy-cloud/internal/domain/database/picodata"
	databasepostgres "github.com/stroppy-io/stroppy-cloud/internal/domain/database/postgres"
	databaseydb "github.com/stroppy-io/stroppy-cloud/internal/domain/database/ydb"
	databaseydbmanaged "github.com/stroppy-io/stroppy-cloud/internal/domain/database/ydbmanaged"
	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	infrastructurebuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/infrastructure"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/packages"
	runbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	workloadbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/workload"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/suite_wizard"
)

// SuiteCellResolver is the consumer interface the engine resolves a cell's source
// (preset pair / test preset / inline test) into a concrete database+workload
// through. The gormstore preset repos satisfy these once injected:
//
//	DatabasePreset -> *DatabasePresetRepo.Get(ctx, tenant, id, "") -> GetDatabase()
//	WorkloadPreset -> *WorkloadPresetRepo.Get(ctx, tenant, id, "") -> GetWorkload()
//	TestPreset     -> *TestPresetRepo.Get(ctx, tenant, id, "")     -> GetTest()
//
// Each returns derrors.ErrNotFound when the (tenant, id) pair is unknown.
type SuiteCellResolver interface {
	DatabasePreset(ctx context.Context, tenantID, presetID string) (*domain.Database, error)
	WorkloadPreset(ctx context.Context, tenantID, presetID string) (*domain.Workload, error)
	TestPreset(ctx context.Context, tenantID, presetID string) (*domain.Test, error)
}

// SuiteBaker is the consumer interface the engine persists/prepares a baked
// suite through. It crosses the storage + execution boundary (handled by other
// workers), so the engine assembles the records and delegates the write/start:
//   - SaveSuite persists the reusable SuiteRecord (returns the stored record with
//     its server-owned identity filled in). replaceSuiteID is set when the
//     wizard was seeded from an existing suite and should update that definition
//     instead of creating a copy.
//   - StartSuiteRun persists a SuiteRunRecord whose children are the supplied
//     baked child runs, returning the record and a post-commit starter. Called
//     only when the wizard requested start=true.
type SuiteBaker interface {
	SaveSuite(ctx context.Context, suite *models.SuiteRecord, replaceSuiteID string) (*models.SuiteRecord, error)
	StartSuiteRun(ctx context.Context, suite *models.SuiteRecord, children []*BakedSuiteChild, trigger common.Trigger, maxParallel uint32) (*models.SuiteRunRecord, func(context.Context) error, error)
}

// BakedSuiteChild is one fully-baked suite cell ready to become a child TestRun.
type BakedSuiteChild struct {
	// CellID is the originating domain.SuiteCell id.
	CellID string
	// Name is the child display label (cell name, falling back to a derived one).
	Name string
	// Run is the materialized, validated domain.TestRun for the cell.
	Run *domain.TestRun
	// InTenantRating / InGlobalRating carry the resolved rating membership.
	InTenantRating bool
	InGlobalRating bool
}

// SuiteWizardEngine implements suite_wizard.WizardEngine. It expands a provider +
// db x workload cell matrix, resolves and validates each cell via the real domain
// builders (the same set the workflow worker registers, mirroring
// execution.TestWizardEngine), and bakes a ready draft into the SuiteRecord /
// SuiteRunRecord through the injected SuiteBaker.
type SuiteWizardEngine struct {
	cells           SuiteCellResolver
	baker           SuiteBaker
	packageResolver packages.Resolver
	renderers       deploymentbuilder.Registry
}

var _ suite_wizard.WizardEngine = (*SuiteWizardEngine)(nil)

// NewSuiteWizardEngine builds the engine wired to the default per-database
// builders + workload renderer, so the wizard preview matches the workflow
// render.
func NewSuiteWizardEngine(cells SuiteCellResolver, baker SuiteBaker) *SuiteWizardEngine {
	return &SuiteWizardEngine{
		cells: cells,
		baker: baker,
		packageResolver: packages.NewRegistry(
			databasepostgres.PackageResolver{},
			databasepicodata.PackageResolver{},
			databaseydb.PackageResolver{},
			databaseydbmanaged.PackageResolver{},
			databasecockroach.PackageResolver{},
			databasemysql.PackageResolver{},
		),
		renderers: deploymentbuilder.NewRegistry(
			databasepostgres.DeploymentRenderer{},
			databasepicodata.DeploymentRenderer{},
			databaseydb.DeploymentRenderer{},
			databaseydbmanaged.DeploymentRenderer{},
			databasecockroach.DeploymentRenderer{},
			databasemysql.DeploymentRenderer{},
			workloadbuilder.DeploymentRenderer{},
		),
	}
}

// Initial builds the starting provider + cell list for a fresh wizard, optionally
// seeded from an existing suite. Cells carry only their spec; previews/readiness
// are filled by the subsequent Recompute.
func (e *SuiteWizardEngine) Initial(_ context.Context, _, _ string, seed *domain.Suite) (deployment.Provider, []*models.SuiteWizardDraftRecord_Cell, error) {
	provider := deployment.Provider_PROVIDER_UNSPECIFIED
	var cells []*models.SuiteWizardDraftRecord_Cell
	if seed != nil {
		provider = seed.GetProvider()
		for _, sc := range seed.GetCells() {
			spec := proto.Clone(sc).(*domain.SuiteCell)
			if spec.GetId() == "" {
				spec.Id = uuid.NewString()
			}
			cells = append(cells, &models.SuiteWizardDraftRecord_Cell{Spec: spec})
		}
	}
	return provider, cells, nil
}

// Recompute resolves each cell from its current spec (db/workload payloads,
// topology, infrastructure plan, render preview, compatibility), tallies per-cell
// and draft-level errors, and reports readiness. It is pure: same draft in => same
// result out (safe in a retried transaction).
func (e *SuiteWizardEngine) Recompute(ctx context.Context, tenantID string, draft *models.SuiteWizardDraftRecord) ([]*models.SuiteWizardDraftRecord_Cell, []*schemapb.FieldError, bool, error) {
	provider := draft.GetProvider()
	var draftErrs []*schemapb.FieldError
	if provider == deployment.Provider_PROVIDER_UNSPECIFIED {
		draftErrs = append(draftErrs, &schemapb.FieldError{Field: "provider", Message: "provider is required"})
	}

	out := make([]*models.SuiteWizardDraftRecord_Cell, 0, len(draft.GetCells()))
	anyEnabledReady := false

	for _, cell := range draft.GetCells() {
		resolved := e.recomputeCell(ctx, tenantID, provider, cell)
		out = append(out, resolved)
		if resolved.GetSpec().GetEnabled() && resolved.GetReady() {
			anyEnabledReady = true
		}
	}

	if len(draft.GetCells()) == 0 {
		draftErrs = append(draftErrs, &schemapb.FieldError{Field: "cells", Message: "at least one cell is required"})
	} else if !anyEnabledReady {
		draftErrs = append(draftErrs, &schemapb.FieldError{Field: "cells", Message: "at least one enabled cell must be ready"})
	}

	ready := len(draftErrs) == 0
	return out, draftErrs, ready, nil
}

// recomputeCell resolves and validates one cell, returning a fresh Cell with the
// server-derived sections filled. Build failures become per-cell field errors
// (not hard errors) so the wizard can surface them.
func (e *SuiteWizardEngine) recomputeCell(ctx context.Context, tenantID string, provider deployment.Provider, in *models.SuiteWizardDraftRecord_Cell) *models.SuiteWizardDraftRecord_Cell {
	cell := &models.SuiteWizardDraftRecord_Cell{Spec: in.GetSpec()}
	var errs []*schemapb.FieldError
	addErr := func(field, message string) {
		errs = append(errs, &schemapb.FieldError{Field: field, Message: message})
	}

	db, wl, resolveErr := e.resolveSource(ctx, tenantID, in.GetSpec())
	if resolveErr != nil {
		addErr("source", resolveErr.Error())
	}
	cell.Database = db
	cell.Workload = wl

	if db == nil {
		addErr("database", "database could not be resolved from cell source")
	}
	if wl == nil {
		addErr("workload", "workload could not be resolved from cell source")
	}
	if provider == deployment.Provider_PROVIDER_UNSPECIFIED {
		addErr("provider", "provider is required")
	}

	if len(errs) == 0 {
		run, err := runbuilder.BuildTestRun(runbuilder.BuildOptions{
			ID:              previewCellRunID(in.GetSpec()),
			Database:        db,
			Workload:        wl,
			Provider:        provider,
			Infrastructure:  infrastructurebuilder.BuildOptionsFromMachineOverrides(provider, in.GetSpec().GetMachineOverrides()),
			RenderOverrides: in.GetSpec().GetRenderOverrides(),
		})
		if err != nil {
			addErr("topology", err.Error())
		} else {
			cell.TopologySpec = run.GetTopologySpec()
			cell.InfrastructurePlan = run.GetInfrastructurePlan()
			cell.Compatible = true
			if missing := missingMachineOverrideNodeIDs(provider, run.GetInfrastructurePlan(), in.GetSpec().GetMachineOverrides()); len(missing) > 0 {
				addErr("machine_overrides", fmt.Sprintf("confirm machine settings for every node: %s", strings.Join(missing, ", ")))
			}

			preview, perr := deploymentbuilder.BuildPreview(run.GetTopologySpec(), deploymentbuilder.PreviewOptions{
				Database:        db,
				Workload:        wl,
				PackageResolver: e.packageResolver,
				Renderers:       e.renderers,
				RenderOverrides: in.GetSpec().GetRenderOverrides(),
			})
			if perr != nil {
				addErr("render_preview", perr.Error())
			} else {
				cell.RenderPreview = preview
			}
		}
	}

	cell.Errors = errs
	cell.Ready = len(errs) == 0
	return cell
}

// resolveSource turns a cell's source oneof into a concrete database + workload.
func (e *SuiteWizardEngine) resolveSource(ctx context.Context, tenantID string, spec *domain.SuiteCell) (*domain.Database, *domain.Workload, error) {
	switch src := spec.GetSource().(type) {
	case *domain.SuiteCell_PresetPair_:
		db, err := e.cells.DatabasePreset(ctx, tenantID, src.PresetPair.GetDbPresetId())
		if err != nil {
			return nil, nil, err
		}
		wl, err := e.cells.WorkloadPreset(ctx, tenantID, src.PresetPair.GetWorkloadPresetId())
		if err != nil {
			return nil, nil, err
		}
		return db, wl, nil
	case *domain.SuiteCell_TestPresetId:
		test, err := e.cells.TestPreset(ctx, tenantID, src.TestPresetId)
		if err != nil {
			return nil, nil, err
		}
		return test.GetDatabase(), test.GetWorkload(), nil
	case *domain.SuiteCell_InlineTest:
		return src.InlineTest.GetDatabase(), src.InlineTest.GetWorkload(), nil
	default:
		return nil, nil, errors.New("cell has no source")
	}
}

// Bake turns a ready draft into the persisted SuiteRecord and, when start=true, a
// prepared SuiteRunRecord. Every compatible+ready ENABLED cell becomes a baked
// child TestRun. It rejects a draft that is not ready.
func (e *SuiteWizardEngine) Bake(ctx context.Context, draft *models.SuiteWizardDraftRecord, req *api.FinishSuiteWizardRequest) (*models.SuiteRecord, *models.SuiteRunRecord, func(context.Context) error, error) {
	if !draft.GetReady() {
		return nil, nil, nil, derrors.FailedPrecondition("draft_not_ready", "draft is not ready")
	}

	suiteCells := make([]*domain.SuiteCell, 0, len(draft.GetCells()))
	children := make([]*BakedSuiteChild, 0, len(draft.GetCells()))
	dbKinds := make([]domain.Database_Kind, 0, len(draft.GetCells()))
	seenKind := make(map[domain.Database_Kind]struct{})

	inTenant := draft.GetDefaultInTenantRating()
	if req.InTenantRating != nil {
		inTenant = req.GetInTenantRating()
	}
	inGlobal := draft.GetDefaultInGlobalRating()
	if req.InGlobalRating != nil {
		inGlobal = req.GetInGlobalRating()
	}

	for _, cell := range draft.GetCells() {
		if !cell.GetSpec().GetEnabled() {
			continue
		}
		suiteCells = append(suiteCells, cell.GetSpec())
		if !cell.GetReady() || !cell.GetCompatible() {
			continue
		}
		run, err := runbuilder.BuildTestRun(runbuilder.BuildOptions{
			ID:              uuid.NewString(),
			Database:        cell.GetDatabase(),
			Workload:        cell.GetWorkload(),
			Provider:        draft.GetProvider(),
			Infrastructure:  infrastructurebuilder.BuildOptionsFromMachineOverrides(draft.GetProvider(), cell.GetSpec().GetMachineOverrides()),
			RenderOverrides: cell.GetSpec().GetRenderOverrides(),
		})
		if err != nil {
			return nil, nil, nil, derrors.Invalid("cells", err.Error()).Wrap(err)
		}
		children = append(children, &BakedSuiteChild{
			CellID:         cell.GetSpec().GetId(),
			Name:           cell.GetSpec().GetName(),
			Run:            run,
			InTenantRating: inTenant,
			InGlobalRating: inGlobal,
		})
		if k := cell.GetDatabase().GetKind(); k != domain.Database_KIND_UNSPECIFIED {
			if _, ok := seenKind[k]; !ok {
				seenKind[k] = struct{}{}
				dbKinds = append(dbKinds, k)
			}
		}
	}

	suiteName := req.GetSuiteName()
	if suiteName == "" {
		suiteName = draft.GetEntity().GetName()
	}

	suiteID := uuid.NewString()
	if draft.GetSuiteId() != "" {
		suiteID = draft.GetSuiteId()
	}

	spec := &domain.Suite{
		Cells:                 suiteCells,
		Provider:              draft.GetProvider(),
		Schedule:              draft.GetSchedule(),
		DefaultMaxParallel:    draft.GetMaxParallel(),
		DefaultInTenantRating: boolPtr(inTenant),
		DefaultInGlobalRating: boolPtr(inGlobal),
	}
	now := nowTimestamp()
	suite := &models.SuiteRecord{
		Entity: &common.Entity{
			Id:       suiteID,
			TenantId: draft.GetEntity().GetTenantId(),
			Name:     suiteName,
			AuthorId: draft.GetEntity().GetAuthorId(),
			Timings:  &common.Timings{CreatedAt: now, UpdatedAt: now},
		},
		Spec: spec,
		Summary: &models.SuiteRecord_Summary{
			ScheduleEnabled: draft.GetSchedule().GetEnabled(),
			Cron:            draft.GetSchedule().GetCron(),
			CellCount:       uint32(len(suiteCells)), //nolint:gosec // bounded by cells.
		},
	}
	spec.Id = suiteID

	savedSuite, err := e.baker.SaveSuite(ctx, suite, draft.GetSuiteId())
	if err != nil {
		return nil, nil, nil, err
	}

	if !req.GetStart() {
		return savedSuite, nil, nil, nil
	}

	suiteRun, starter, err := e.baker.StartSuiteRun(ctx, savedSuite, children, common.Trigger_TRIGGER_MANUAL, draft.GetMaxParallel())
	if err != nil {
		return nil, nil, nil, err
	}
	return savedSuite, suiteRun, starter, nil
}

// boolPtr returns a pointer to b for the optional rating defaults.
func boolPtr(b bool) *bool { return &b }

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

func previewCellRunID(cell *domain.SuiteCell) string {
	if id := cell.GetId(); id != "" {
		return id
	}
	return "preview"
}
