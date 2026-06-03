package execution

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	infrastructurebuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/infrastructure"
	runbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deployment "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"github.com/stroppy-io/stroppy-cloud/internal/services/suite"
)

// CellResolver resolves a suite cell's source (test preset, db+workload preset
// pair, or inline test) into a concrete db+workload domain.Test. Preset loading
// is storage-scoped, so it is injected: the launcher itself owns no storage. It
// returns an error (NotFound / FailedPrecondition friendly) when a referenced
// preset is missing or incompatible.
type CellResolver interface {
	ResolveCell(ctx context.Context, tenantID string, cell *domain.SuiteCell) (*domain.Test, error)
}

// ChildRunPersister persists a child TestRunRecord expanded from a suite cell. It
// runs inside the ambient ctx transaction (the suite service opens one), so the
// child rows commit atomically with the SuiteRunRecord.
type ChildRunPersister interface {
	CreateChildRun(ctx context.Context, run *models.TestRunRecord) error
	UpdateChildRun(ctx context.Context, run *models.TestRunRecord) error
}

// SuiteRunPersister persists the parent SuiteRunRecord after child expansion and
// before the SuiteWorkflow is started.
type SuiteRunPersister interface {
	SaveSuiteRun(ctx context.Context, run *models.SuiteRunRecord) error
}

// SuiteRunLauncher implements suite.SuiteRunLauncher. It expands a baked
// domain.Suite into one child TestRunRecord per compatible+enabled cell,
// persists them, fills the SuiteRunRecord children + summary, and builds a
// post-commit SuiteWorkflow starter with one RunConfig per child.
type SuiteRunLauncher struct {
	tc       workflowpb.SuiteWorkflowServiceClient
	resolver CellResolver
	children ChildRunPersister
	suites   SuiteRunPersister
	settings runbuilder.SettingsSource
}

var _ suite.SuiteRunLauncher = (*SuiteRunLauncher)(nil)

// NewSuiteRunLauncher builds the suite.SuiteRunLauncher adapter.
//
//   - c        Temporal client used to start SuiteWorkflow.
//   - resolver resolves each cell's preset/inline source into a db+workload Test.
//   - children persists the expanded child TestRunRecords.
//   - suites persists the parent SuiteRunRecord after children are attached.
//   - settings supplies per-provider settings + agent bootstrap for the RunConfigs
//     (may be nil to build RunConfigs without provider settings / bootstrap).
func NewSuiteRunLauncher(c client.Client, resolver CellResolver, children ChildRunPersister, suites SuiteRunPersister, settings runbuilder.SettingsSource) *SuiteRunLauncher {
	return &SuiteRunLauncher{
		tc:       workflowpb.NewSuiteWorkflowServiceClient(c),
		resolver: resolver,
		children: children,
		suites:   suites,
		settings: settings,
	}
}

// Validate checks the suite expands into >=1 runnable cell and that every
// referenced cell resolves + bakes. It performs no writes.
func (l *SuiteRunLauncher) Validate(ctx context.Context, tenantID string, spec *domain.Suite) error {
	if spec == nil {
		return errors.New("suite spec is required")
	}
	runnable := 0
	for _, cell := range spec.GetCells() {
		if !cell.GetEnabled() {
			continue
		}
		if _, err := l.bakeCell(ctx, tenantID, spec, cell); err != nil {
			return fmt.Errorf("cell %q: %w", cellLabel(cell), err)
		}
		runnable++
	}
	if runnable == 0 {
		return errors.New("suite has no enabled, runnable cells")
	}
	return nil
}

// Launch expands + persists the SuiteRunRecord's children and returns a
// post-commit SuiteWorkflow starter. The handler has already filled the run's
// identity/tenant/trigger/timing; this fills children + summary and carries the
// suite's resolved rating defaults onto each child.
func (l *SuiteRunLauncher) Launch(ctx context.Context, run *models.SuiteRunRecord, spec *domain.Suite) (func(context.Context) error, error) {
	if spec == nil {
		return nil, errors.New("suite spec is required")
	}

	now := timestamppb.New(time.Now())
	inTenant := spec.GetDefaultInTenantRating()
	inGlobal := spec.GetDefaultInGlobalRating()

	var (
		children     []*models.SuiteRunRecord_ChildRun
		childRecords []*models.TestRunRecord
		configs      []*workflowpb.RunConfig
		dbKinds      []domain.Database_Kind
	)

	for _, cell := range spec.GetCells() {
		if !cell.GetEnabled() {
			continue
		}
		testRun, err := l.bakeCell(ctx, runTenantID(run), spec, cell)
		if err != nil {
			return nil, fmt.Errorf("cell %q: %w", cellLabel(cell), err)
		}

		childID := testRun.GetId()
		childRec := &models.TestRunRecord{
			Entity: &commonpb.Entity{
				Id:       childID,
				TenantId: runTenantID(run),
				Name:     cellLabel(cell),
				AuthorId: run.GetEntity().GetAuthorId(),
				Timings:  &commonpb.Timings{CreatedAt: now, UpdatedAt: now},
			},
			Spec:           testRun,
			Status:         commonpb.Status_STATUS_PENDING,
			SuiteRunId:     run.GetEntity().GetId(),
			SuiteCellId:    cell.GetId(),
			Trigger:        commonpb.Trigger_TRIGGER_API,
			InTenantRating: inTenant,
			InGlobalRating: inGlobal,
			Summary:        RunSummarizer{}.Summarize(testRun),
		}
		if err := l.children.CreateChildRun(ctx, childRec); err != nil {
			return nil, err
		}
		childRecords = append(childRecords, childRec)

		children = append(children, &models.SuiteRunRecord_ChildRun{
			SuiteCellId: cell.GetId(),
			TestRunId:   childID,
			Name:        cellLabel(cell),
			Status:      commonpb.Status_STATUS_PENDING,
		})
		dbKinds = append(dbKinds, testRun.GetDatabase().GetKind())

		cfg, err := l.runConfig(ctx, runTenantID(run), spec.GetProvider(), testRun)
		if err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}

	run.Children = children
	run.Summary = &models.SuiteRunRecord_Summary{
		SuiteName: spec.GetId(),
		Provider:  spec.GetProvider(),
		DbKinds:   dedupeKinds(dbKinds),
		Total:     uint32(len(children)),
		Pending:   uint32(len(children)),
		StartedAt: now,
	}
	if l.suites == nil {
		return nil, errors.New("suite run persister is required")
	}
	if err := l.suites.SaveSuiteRun(ctx, run); err != nil {
		return nil, err
	}

	req := &workflowpb.SuiteWorkflowRequest{
		SuiteRunId:  run.GetEntity().GetId(),
		Runs:        configs,
		MaxParallel: run.GetMaxParallel(),
	}
	start := func(ctx context.Context) error {
		if _, err := l.tc.SuiteWorkflowAsync(ctx, req); err != nil {
			if ferr := l.failPreparedSuite(ctx, run, childRecords); ferr != nil {
				return ferr
			}
			return err
		}
		return nil
	}
	return start, nil
}

func (l *SuiteRunLauncher) failPreparedSuite(ctx context.Context, run *models.SuiteRunRecord, children []*models.TestRunRecord) error {
	now := timestamppb.New(time.Now())
	run.Status = commonpb.Status_STATUS_FAILED
	if run.Summary == nil {
		run.Summary = &models.SuiteRunRecord_Summary{}
	}
	var failed uint32
	for _, child := range run.GetChildren() {
		if isTerminalStatus(child.GetStatus()) {
			continue
		}
		child.Status = commonpb.Status_STATUS_FAILED
		failed++
	}
	run.Summary.Total = uint32(len(run.GetChildren()))
	run.Summary.Failed = failed
	run.Summary.Pending = 0
	run.Summary.Running = 0
	run.Summary.ProgressPct = progressPct(int(failed+run.Summary.GetCompleted()), len(run.GetChildren()))
	if run.Summary.StartedAt == nil {
		run.Summary.StartedAt = now
	}
	if run.Summary.FinishedAt == nil {
		run.Summary.FinishedAt = now
	}
	run.Summary.Duration = suiteDuration(run.Summary.GetStartedAt(), run.Summary.GetFinishedAt(), time.Now())
	touchRecordUpdated(run.GetEntity(), time.Now())
	if err := l.suites.SaveSuiteRun(ctx, run); err != nil {
		return err
	}

	for _, child := range children {
		if isTerminalStatus(child.GetStatus()) {
			continue
		}
		child.Status = commonpb.Status_STATUS_FAILED
		if child.Summary == nil {
			child.Summary = &models.TestRunRecord_Summary{}
		}
		if child.Summary.StartedAt == nil {
			child.Summary.StartedAt = now
		}
		if child.Summary.FinishedAt == nil {
			child.Summary.FinishedAt = now
		}
		child.Summary.Duration = spanDuration(child.Summary.GetStartedAt(), child.Summary.GetFinishedAt(), time.Now())
		child.Summary.ProgressPct = 100
		touchRecordUpdated(child.GetEntity(), time.Now())
		if err := l.children.UpdateChildRun(ctx, child); err != nil {
			return err
		}
	}
	return nil
}

// bakeCell resolves a cell into a db+workload Test and bakes it into a fully
// materialized domain.TestRun (topology + infrastructure plan) with a minted id.
func (l *SuiteRunLauncher) bakeCell(ctx context.Context, tenantID string, spec *domain.Suite, cell *domain.SuiteCell) (*domain.TestRun, error) {
	test, err := l.resolver.ResolveCell(ctx, tenantID, cell)
	if err != nil {
		return nil, err
	}
	if test.GetDatabase() == nil || test.GetWorkload() == nil {
		return nil, errors.New("resolved cell has no database+workload")
	}

	infraOpts := infrastructurebuilder.BuildOptions{}
	overrides := cell.GetRenderOverrides()

	testRun, err := runbuilder.BuildTestRun(runbuilder.BuildOptions{
		ID:              uuid.NewString(),
		SuiteID:         spec.GetId(),
		Database:        test.GetDatabase(),
		Workload:        test.GetWorkload(),
		Provider:        spec.GetProvider(),
		Infrastructure:  infraOpts,
		RenderOverrides: overrides,
		Tags:            test.GetTags(),
	})
	if err != nil {
		return nil, err
	}
	return testRun, nil
}

// runConfig builds the workflow RunConfig for an already-baked child test run,
// resolving provider settings + agent bootstrap through the settings source.
func (l *SuiteRunLauncher) runConfig(ctx context.Context, tenantID string, provider deployment.Provider, testRun *domain.TestRun) (*workflowpb.RunConfig, error) {
	cfg := &workflowpb.RunConfig{
		TenantId:           tenantID,
		Id:                 testRun.GetId(),
		Database:           testRun.GetDatabase(),
		Workload:           testRun.GetWorkload(),
		TopologySpec:       testRun.GetTopologySpec(),
		InfrastructurePlan: testRun.GetInfrastructurePlan(),
		RenderOverrides:    testRun.GetRenderOverrides(),
	}
	if l.settings != nil {
		providerSettings, err := l.settings.ProviderSettings(ctx, tenantID, provider)
		if err != nil {
			return nil, err
		}
		if cfg.InfrastructurePlan != nil {
			cfg.InfrastructurePlan = proto.Clone(cfg.InfrastructurePlan).(*deployment.InfrastructurePlan)
			cfg.InfrastructurePlan.Settings = providerSettings
		}
		bootstrap, err := l.settings.AgentBootstrap(ctx)
		if err != nil {
			return nil, err
		}
		cfg.AgentBootstrap = bootstrap
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// tenantID reads the suite run's tenant from its entity.
func runTenantID(run *models.SuiteRunRecord) string {
	return run.GetEntity().GetTenantId()
}

// cellLabel returns a human label for a cell (its name, else its id).
func cellLabel(cell *domain.SuiteCell) string {
	if n := cell.GetName(); n != "" {
		return n
	}
	return cell.GetId()
}

// dedupeKinds returns the distinct db kinds in first-seen order.
func dedupeKinds(kinds []domain.Database_Kind) []domain.Database_Kind {
	seen := make(map[domain.Database_Kind]struct{}, len(kinds))
	out := make([]domain.Database_Kind, 0, len(kinds))
	for _, k := range kinds {
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}
