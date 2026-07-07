// Package workflows: this file implements RunRecipeWorkflow — the phase-1D
// wrapper workflow that turns one recipe run request (a bundle of recipe
// files + a run id) into compile -> provision -> execute -> teardown,
// emitting a workflowpb.RunState the same way domainTestWorkflow (test.go)
// does, but with 4 dynamic stage names of its own (compile/infra/execute/
// teardown) rather than test.go's 5 hardcoded ones.
//
// # Activity-call mechanism
//
// CompileRecipeActivity, ProvisionActivity and TeardownActivity have no
// generated proto activity service (mirroring ExecuteCompiledPlanWorkflow's
// own NomadSubmitJobActivityName precedent in dslrun.go): Task 4 registers
// their real implementations directly against the worker
// (registry.RegisterActivityWithOptions(impl.XxxActivity, activity.
// RegisterOptions{Name: XxxActivityName})), and this workflow calls them by
// NAME via workflow.ExecuteActivity(ctx, XxxActivityName, in).Get(ctx, &out)
// — never a typed generated wrapper — so this file compiles and is fully
// testable (via env.OnActivity(XxxActivityName, ...)) before Task 4 exists.
//
// # providerRef source
//
// RunRecipeInput carries only the recipe bundle, not a separate provider
// reference: dslpb.CompiledPlan already has a Provider *dslpb.ProviderRef
// field (cluster.yaml's `provider.use` + params, resolved by the compiler —
// see internal/dsl/lower/lower.go's lowerProvider), so CompileRecipeActivity
// resolves it as part of compilation and this workflow reads it off
// plan.GetProvider() once compile succeeds, threading that same ref into
// ProvisionActivity and (in the teardown defer) TeardownActivity. This
// avoids a second, potentially inconsistent provider reference living on
// RunRecipeInput.
//
// # Stage failure vs workflow error
//
// Unlike domainTestWorkflow (test.go), a CompileRecipeActivity/
// ProvisionActivity/child-workflow failure does NOT make RunRecipeWorkflow
// itself return a Go error: the corresponding RunState stage is marked
// FAILED (mirroring test.go's failStage/persist machinery) and
// RunRecipeOutput.Status reports "failed", but the workflow function
// returns (out, nil) — the same "activity failure surfaces as a status, not
// a workflow error" philosophy ExecuteCompiledPlanWorkflow (dslrun.go) uses
// for individual job failures. Only a genuine Temporal-level break (a
// persist-activity failure, teardown failing, or cancellation) produces a
// non-nil error, exactly as test.go's own persist/teardown error paths do.
//
// # Cancel / teardown
//
// Teardown always runs from a deferred func on a workflow.
// NewDisconnectedContext (mirroring domainTestWorkflow's teardown defer),
// so it still tears down infrastructure when the workflow context itself
// has already been canceled. TeardownActivity is skipped (a documented
// no-op) when providerRef is nil — i.e. compile never resolved a provider,
// so ProvisionActivity was never called and nothing was ever provisioned.
//
// # Bootstrap stage
//
// The brief's task-3-brief.md lists a 5th "bootstrap" stage (a
// WaitNomadReadyActivity polling the gateway's Nomad server) as a v1
// best-effort option ("or fold into execute"). This implementation folds it
// into infra/execute exactly as offered: the provider's Provision already
// bootstraps agents via cloud-init/docker (phase 1B), and
// ExecuteCompiledPlanWorkflow's service jobs submit to Nomad directly — a
// standalone WaitNomadReadyActivity is left for a later task rather than
// adding a 5th RunState stage with no activity behind it yet.
package workflows

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

// RunRecipeWorkflowName is the Temporal workflow type RunRecipeWorkflow
// registers under (see RegisterWorkflows in register.go). Like
// ExecuteCompiledPlanWorkflowName, this is hand-written rather than
// generated: RunRecipeWorkflow has no proto workflow service of its own.
const RunRecipeWorkflowName = "RunRecipeWorkflow"

// Activity names Task 4 registers the real implementations under. Declared
// here (not in an execution/activities package) so this workflow file is the
// single source of truth callers (Task 4's worker wiring) register against,
// mirroring dslrun.go's NomadSubmitJobActivityName precedent.
const (
	// CompileRecipeActivityName compiles a recipe bundle (internal/dsl.Compile)
	// into a dslpb.CompiledPlan.
	CompileRecipeActivityName = "CompileRecipeActivity"
	// ProvisionActivityName requests machines from the plan's provider for
	// every compiled machine group.
	ProvisionActivityName = "ProvisionActivity"
	// TeardownActivityName destroys every resource a provider previously
	// provisioned for a ProviderRef.
	TeardownActivityName = "TeardownActivity"
)

// RunRecipeInput is RunRecipeWorkflow's input: a run id, the owning tenant,
// the recipe bundle's raw files (cluster.yaml, workflow.yaml, components/**,
// files/** — see RecipeRecord.Bundle.Files), and the agent bootstrap
// (per-node Temporal task queues) ExecuteCompiledPlanWorkflow's steps/
// services dispatch activities through.
//
// There is no separate provider reference field — see the package doc's
// "providerRef source" note: CompileRecipeActivity resolves it from the
// bundle's cluster.yaml as part of compiling the CompiledPlan.
type RunRecipeInput struct {
	RunID     string
	TenantID  string
	Bundle    map[string][]byte
	Bootstrap *workflowpb.AgentBootstrap
}

// RunRecipeOutput is RunRecipeWorkflow's terminal result: Status is the
// lowercased common.Status the run finished in ("completed", "failed",
// "canceled" — see runRecipeStatusString), JobStatuses is
// ExecuteCompiledPlanOutput.JobStatuses forwarded verbatim (nil if execute
// never ran, e.g. compile or infra failed first).
type RunRecipeOutput struct {
	Status      string
	JobStatuses map[string]string
}

// CompileRecipeActivityInput is CompileRecipeActivity's input: the recipe
// bundle's raw files.
type CompileRecipeActivityInput struct {
	Bundle map[string][]byte
}

// CompileRecipeActivityOutput is CompileRecipeActivity's output: the
// compiled plan (nil when Diagnostics.HasErrors()) plus every diagnostic
// (errors AND warnings) the compiler produced, mirroring internal/dsl.
// Compile's own (plan, diag.List) return shape.
type CompileRecipeActivityOutput struct {
	Plan        *dslpb.CompiledPlan
	Diagnostics diag.List
}

// ProvisionActivityInput is ProvisionActivity's input: the compiled plan's
// machine groups and the provider reference to provision them against —
// mirrors internal/infrastructure/provider.Provider.Provision's own
// (ref, groups) argument pair.
type ProvisionActivityInput struct {
	Groups      []*dslpb.MachineGroup
	ProviderRef *dslpb.ProviderRef
}

// ProvisionActivityOutput is ProvisionActivity's output: the real machines
// the provider handed back, keyed by machine_groups entry name — the same
// shape ExecuteCompiledPlanInput.Machines expects.
type ProvisionActivityOutput struct {
	Machines map[string][]*deploymentpb.MachineState
}

// TeardownActivityInput is TeardownActivity's input: the provider reference
// whose previously-provisioned resources must be destroyed — mirrors
// internal/infrastructure/provider.Provider.Destroy's own argument.
type TeardownActivityInput struct {
	ProviderRef *dslpb.ProviderRef
}

// RunRecipeWorkflow orchestrates one recipe run: compile the bundle, provision
// the machines the compiled plan asks for, execute the plan's job DAG (as an
// ExecuteCompiledPlanWorkflow child), and always tear down whatever was
// provisioned — success, failure, or cancellation. See the package doc above
// for the stage-failure/workflow-error and cancel/teardown design.
func RunRecipeWorkflow(ctx workflow.Context, in *RunRecipeInput) (*RunRecipeOutput, error) {
	if in == nil {
		return nil, errors.New("run recipe workflow: input is required")
	}
	if in.RunID == "" {
		return nil, errors.New("run recipe workflow: run id is required")
	}

	w := newRunRecipeWorkflow(in)
	if err := workflow.SetQueryHandler(ctx, workflowpb.GetRunStateQueryName, w.GetRunState); err != nil {
		return nil, err
	}
	return w.run(ctx)
}

const (
	runRecipeStageCompile  = "compile"
	runRecipeStageInfra    = "infra"
	runRecipeStageExecute  = "execute"
	runRecipeStageTeardown = "teardown"
)

const (
	runRecipeStageCompileIndex = iota
	runRecipeStageInfraIndex
	runRecipeStageExecuteIndex
	runRecipeStageTeardownIndex
)

type runRecipeWorkflow struct {
	in    *RunRecipeInput
	state *workflowpb.RunState
}

func newRunRecipeWorkflow(in *RunRecipeInput) *runRecipeWorkflow {
	return &runRecipeWorkflow{
		in: in,
		state: &workflowpb.RunState{
			Status: common.Status_STATUS_PENDING,
			Stages: []*workflowpb.Stage{
				rootStage(runRecipeStageCompile, 1),
				rootStage(runRecipeStageInfra, 2),
				rootStage(runRecipeStageExecute, 3),
				rootStage(runRecipeStageTeardown, 4),
			},
		},
	}
}

// GetRunState is the query handler registered under
// workflowpb.GetRunStateQueryName — the exact query name
// internal/infrastructure/execution/overview.go's runStateQuerier already
// targets for TestWorkflow (workflowpb.NewTestServiceClient(c).GetRunState),
// so a later wiring task can query RunRecipeWorkflow through that same
// client/reader with zero changes on the reader side.
func (w *runRecipeWorkflow) GetRunState() (*workflowpb.RunState, error) {
	return compactRunStateForRuntimeProjection(w.state), nil
}

// run drives the compile/infra/execute stages and, via its deferred
// teardown, guarantees teardown always runs (success, failure, or
// cancellation) exactly like domainTestWorkflow's own teardown defer.
func (w *runRecipeWorkflow) run(ctx workflow.Context) (result *RunRecipeOutput, err error) { //nolint:nonamedreturns // the deferred teardown/persist must observe and set both.
	w.state.Status = common.Status_STATUS_RUNNING

	var (
		providerRef *dslpb.ProviderRef
		jobStatuses map[string]string
		stageFailed bool
	)

	defer func() {
		tctx, _ := workflow.NewDisconnectedContext(ctx)
		canceled := temporal.IsCanceledError(err)

		w.startStage(tctx, runRecipeStageTeardownIndex)
		if perr := w.persist(tctx); perr != nil && err == nil {
			err = fmt.Errorf("persist teardown run state: %w", perr)
		}
		if terr := w.teardown(tctx, providerRef); terr != nil {
			w.failStage(tctx, runRecipeStageTeardownIndex, terr.Error())
			stageFailed = true
			if err != nil {
				err = fmt.Errorf("%w; teardown: %w", err, terr)
			} else {
				err = fmt.Errorf("teardown: %w", terr)
			}
		} else {
			w.completeStage(tctx, runRecipeStageTeardownIndex)
		}

		switch {
		case canceled:
			w.cancelStages(tctx)
		case stageFailed || err != nil:
			w.state.Status = common.Status_STATUS_FAILED
		default:
			w.state.Status = common.Status_STATUS_COMPLETED
		}
		if perr := w.persist(tctx); perr != nil && err == nil {
			err = fmt.Errorf("persist final run state: %w", perr)
		}

		result = &RunRecipeOutput{
			Status:      runRecipeStatusString(w.state.GetStatus()),
			JobStatuses: jobStatuses,
		}
	}()

	w.startStage(ctx, runRecipeStageCompileIndex)
	if perr := w.persist(ctx); perr != nil {
		return nil, perr
	}
	plan, cerr := w.compileRecipe(ctx)
	if cerr != nil {
		if temporal.IsCanceledError(cerr) {
			return nil, cerr //nolint:wrapcheck // propagated verbatim so the deferred teardown's temporal.IsCanceledError(err) check engages — see package doc's "Cancel / teardown" note.
		}
		w.failStage(ctx, runRecipeStageCompileIndex, cerr.Error())
		stageFailed = true
		if perr := w.persist(ctx); perr != nil {
			return nil, perr
		}
		return nil, nil //nolint:nilnil // stage failure is reported via RunState/output.Status, not a workflow error — see package doc.
	}
	providerRef = plan.GetProvider()
	w.completeStage(ctx, runRecipeStageCompileIndex)
	// Stamp the recipe-derived Summary facets (Provider/NodeCount/
	// TopologyLabel/DbKind/WorkloadName/StroppyVersion) onto the run record
	// as soon as the plan exists — this is the ROOT fix for rating/metrics/
	// compare/share/dashboard, all of which read Summary rather than the
	// (absent, for a recipe run) domain.TestRun spec. See runrecipe_summary.
	// go's deriveRunSummary for the derivation heuristics.
	if perr := persistRunSummary(ctx, w.in.RunID, deriveRunSummary(plan)); perr != nil {
		return nil, perr
	}
	if perr := w.persist(ctx); perr != nil {
		return nil, perr
	}

	w.startStage(ctx, runRecipeStageInfraIndex)
	if perr := w.persist(ctx); perr != nil {
		return nil, perr
	}
	machines, ierr := w.provision(ctx, plan, providerRef)
	if ierr != nil {
		if temporal.IsCanceledError(ierr) {
			return nil, ierr //nolint:wrapcheck // propagated verbatim — see compileRecipe's identical guard above.
		}
		w.failStage(ctx, runRecipeStageInfraIndex, ierr.Error())
		stageFailed = true
		if perr := w.persist(ctx); perr != nil {
			return nil, perr
		}
		return nil, nil //nolint:nilnil // see above.
	}
	w.completeStage(ctx, runRecipeStageInfraIndex)
	if perr := w.persist(ctx); perr != nil {
		return nil, perr
	}

	gatewayNodeID := pickGatewayNodeID(plan, machines)
	w.startStage(ctx, runRecipeStageExecuteIndex)
	if perr := w.persist(ctx); perr != nil {
		return nil, perr
	}
	execOut, eerr := w.executeCompiledPlan(ctx, plan, machines, gatewayNodeID)
	if eerr != nil {
		if temporal.IsCanceledError(eerr) {
			return nil, eerr //nolint:wrapcheck // propagated verbatim — see compileRecipe's identical guard above.
		}
		w.failStage(ctx, runRecipeStageExecuteIndex, eerr.Error())
		stageFailed = true
		if perr := w.persist(ctx); perr != nil {
			return nil, perr
		}
		return nil, nil //nolint:nilnil // see above.
	}
	jobStatuses = execOut.JobStatuses
	w.completeStage(ctx, runRecipeStageExecuteIndex)
	if perr := w.persist(ctx); perr != nil {
		return nil, perr
	}

	return nil, nil //nolint:nilnil // the deferred func fills result with the COMPLETED status + jobStatuses.
}

// compileRecipe calls CompileRecipeActivity and turns an error-severity
// diagnostic list into a Go error the caller reports as the compile stage's
// failure (see run's stageFailed handling) — a plan is only ever returned
// once Diagnostics.HasErrors() is false.
func (w *runRecipeWorkflow) compileRecipe(ctx workflow.Context) (*dslpb.CompiledPlan, error) {
	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
	})
	var out CompileRecipeActivityOutput
	if err := workflow.ExecuteActivity(actx, CompileRecipeActivityName, &CompileRecipeActivityInput{
		Bundle: w.in.Bundle,
	}).Get(actx, &out); err != nil {
		return nil, err
	}
	if out.Diagnostics.HasErrors() {
		return nil, fmt.Errorf("recipe bundle failed to compile: %s", diagErrorSummary(out.Diagnostics))
	}
	if out.Plan == nil {
		return nil, errors.New("compile recipe activity: no error diagnostics but plan is nil")
	}
	return out.Plan, nil
}

// provision calls ProvisionActivity for plan's machine groups against ref.
// Heartbeat/timeout are generous (terraform apply / docker up can run for
// several minutes per node), mirroring executeComponentDeployment's own
// 60-minute/1-minute convention in deployment.go.
func (w *runRecipeWorkflow) provision(ctx workflow.Context, plan *dslpb.CompiledPlan, ref *dslpb.ProviderRef) (map[string][]*deploymentpb.MachineState, error) {
	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Minute,
		HeartbeatTimeout:    time.Minute,
	})
	var out ProvisionActivityOutput
	if err := workflow.ExecuteActivity(actx, ProvisionActivityName, &ProvisionActivityInput{
		Groups:      plan.GetMachineGroups(),
		ProviderRef: ref,
	}).Get(actx, &out); err != nil {
		return nil, err
	}
	return out.Machines, nil
}

// executeCompiledPlan runs the compiled plan's job DAG as an
// ExecuteCompiledPlanWorkflow child (Task 12/16's generic interpreter, see
// dslrun.go). Bound to a stable child workflow id (run id + "/execute") so a
// replay/retry of this parent does not attempt to start a second child with
// a colliding random id.
func (w *runRecipeWorkflow) executeCompiledPlan(
	ctx workflow.Context,
	plan *dslpb.CompiledPlan,
	machines map[string][]*deploymentpb.MachineState,
	gatewayNodeID string,
) (*ExecuteCompiledPlanOutput, error) {
	cctx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: w.in.RunID + "/execute",
	})
	var out ExecuteCompiledPlanOutput
	if err := workflow.ExecuteChildWorkflow(cctx, ExecuteCompiledPlanWorkflowName, &ExecuteCompiledPlanInput{
		Plan:          plan,
		Machines:      machines,
		Bootstrap:     w.in.Bootstrap,
		GatewayNodeID: gatewayNodeID,
	}).Get(cctx, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// teardown calls TeardownActivity for ref, unless ref is nil — i.e. compile
// never resolved a provider (it failed before doing so), so nothing was ever
// provisioned and there is nothing to destroy. Documented no-op per the
// package doc's "cancel / teardown" note.
func (w *runRecipeWorkflow) teardown(ctx workflow.Context, ref *dslpb.ProviderRef) error {
	if ref == nil {
		return nil
	}
	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Minute,
		HeartbeatTimeout:    time.Minute,
	})
	return workflow.ExecuteActivity(actx, TeardownActivityName, &TeardownActivityInput{
		ProviderRef: ref,
	}).Get(actx, nil)
}

// persist mirrors domainTestWorkflow.persist, minus the infrastructure-state/
// deployment-plan de-duplication logic: RunRecipeWorkflow produces neither a
// deploymentpb.InfrastructureState nor a deploymentpb.DeploymentPlan (its
// provisioning result is a plain map[string][]*deploymentpb.MachineState —
// dslpb.CompiledPlan's own shape, not the old deployment-plan domain model),
// so both are always persisted as nil (persistRunState/PersistRunState
// treats nil as "keep the stored value" — see runtime.go).
func (w *runRecipeWorkflow) persist(ctx workflow.Context) error {
	return persistRunState(ctx, w.in.RunID, w.state, nil, nil)
}

func (w *runRecipeWorkflow) startStage(ctx workflow.Context, index int) {
	startRootStage(ctx, w.state.Stages[index])
}

func (w *runRecipeWorkflow) completeStage(ctx workflow.Context, index int) {
	completeRootStage(ctx, w.state.Stages[index])
}

func (w *runRecipeWorkflow) failStage(ctx workflow.Context, index int, errText string) {
	failRootStage(ctx, w.state.Stages[index])
	w.state.Stages[index].ErrorMessage = errText
}

// cancelStages marks the run and every not-yet-terminal stage CANCELED,
// mirroring domainTestWorkflow.cancel (test.go) — except it cancels every
// pending/running stage rather than just the first, since RunRecipeWorkflow
// has no ordering dependency between its own root stages once one is
// interrupted mid-flight.
func (w *runRecipeWorkflow) cancelStages(ctx workflow.Context) {
	w.state.Status = common.Status_STATUS_CANCELLED //nolint:misspell // generated proto enum identifier, not a spelling choice
	for _, stage := range w.state.GetStages() {
		if stage.GetStatus() == common.Status_STATUS_RUNNING || stage.GetStatus() == common.Status_STATUS_PENDING {
			cancelRootStage(ctx, stage)
		}
	}
}

// runRecipeStatusString lowercases a common.Status for RunRecipeOutput.Status
// (e.g. STATUS_COMPLETED -> "completed"), matching the convention
// sourceStatusReason (overview.go) and stageStatusReason (stages.go) already
// use for status-derived strings elsewhere in this codebase.
func runRecipeStatusString(status common.Status) string {
	return strings.TrimPrefix(strings.ToLower(status.String()), "status_")
}

// diagErrorSummary joins up to the first 5 error-severity diagnostics
// (path: message) into one string for compileRecipe's returned error —
// enough to identify the problem without unbounded output for a bundle with
// many errors.
func diagErrorSummary(diags diag.List) string {
	const maxDiagnostics = 5
	var b strings.Builder
	n := 0
	for _, d := range diags {
		if d.Severity != diag.Error {
			continue
		}
		if n > 0 {
			b.WriteString("; ")
		}
		b.WriteString(d.Path)
		b.WriteString(": ")
		b.WriteString(d.Message)
		n++
		if n >= maxDiagnostics {
			break
		}
	}
	return b.String()
}

// pickGatewayNodeID selects the node whose task queue
// ExecuteCompiledPlanWorkflow's service jobs submit Nomad activities to (see
// executeServiceJob in dslrun.go). CompiledPlan carries no explicit "this is
// the gateway" marker today, so the convention is: a machine_groups entry
// literally named "runner" or "gateway" (checked in that order) is
// preferred; otherwise the first machine of the first machine_groups entry,
// in the plan's own declaration order, is used. Returns "" if the plan has
// no machine groups or the selected group's provisioned machines are empty —
// a recipe with a service job but no eligible group is a bundle authoring
// problem executeServiceJob already reports clearly ("on_group ... has no
// machines" / "gateway task queue: ...").
func pickGatewayNodeID(plan *dslpb.CompiledPlan, machines map[string][]*deploymentpb.MachineState) string {
	groups := plan.GetMachineGroups()
	for _, name := range []string{"runner", "gateway"} {
		for _, mg := range groups {
			if mg.GetName() != name {
				continue
			}
			if ms := machines[name]; len(ms) > 0 {
				return ms[0].GetNodeId()
			}
		}
	}
	if len(groups) == 0 {
		return ""
	}
	if ms := machines[groups[0].GetName()]; len(ms) > 0 {
		return ms[0].GetNodeId()
	}
	return ""
}
