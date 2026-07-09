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
//
// # Global Constraints
//
// The DSL compiler runs ASYNCHRONOUSLY on a Temporal worker, inside
// CompileRecipeActivity — never inside the connect-rpc StartRun handler and
// never here in workflow code. Workflow code is deterministic and replayed;
// compilation is neither. RunRecipeInput.Baked (the sealed launch-form
// snapshot) is therefore threaded as pure data: set once at workflow-start,
// copied into CompileRecipeActivityInput, never inspected or mutated in
// workflow context. The provider params schema Baked's provider params are
// validated against is re-derived inside the activity from the bundle already
// present there, rather than threaded as a second value — keeping workflow
// history to just Baked itself.
package workflows

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/proto"

	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
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
	// ReserveQuotasActivityName reserves the quota demand the compiled
	// plan's machine groups compute (see internal/infrastructure/quotas/
	// reserve.go's Manager.Reserve) — called before ProvisionActivity so a
	// run that would exceed capacity never provisions unmetered
	// infrastructure (see reserveQuotas below).
	ReserveQuotasActivityName = "ReserveQuotasActivity"
	// CommitQuotasActivityName promotes a run's reservation to allocated
	// once ExecuteCompiledPlanWorkflow succeeds (see commitQuotas below).
	CommitQuotasActivityName = "CommitQuotasActivity"
	// ReleaseQuotasActivityName frees a run's quota reservation/allocation —
	// called unconditionally from the teardown defer (see releaseQuotas
	// below), regardless of how the run ended.
	ReleaseQuotasActivityName = "ReleaseQuotasActivity"
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
	// Baked is the deterministically-marshaled (proto.Marshal) sealed
	// launch-form snapshot (SP-D) this run was launched with — nil/empty for
	// a non-form launch. It is forwarded verbatim into
	// CompileRecipeActivityInput.Baked (see compileRecipe below); the
	// compiler runs on the Temporal WORKER inside CompileRecipeActivity, not
	// here in workflow code (see the package doc's Global Constraints), so
	// this workflow never itself touches Baked's contents — it is pure
	// data threaded through, exactly like Bootstrap above.
	//
	// Baked is carried as raw proto-marshaled bytes, NOT *schemapb.Baked,
	// deliberately: schemapb.Baked.Schema.Fields[].Kind is a protobuf oneof,
	// and RunRecipeInput is a plain Go struct — Temporal's default
	// DataConverter serializes struct fields with encoding/json (the
	// proto-aware converters in its chain only engage when the TOP-LEVEL
	// value passed to ExecuteWorkflow/ExecuteActivity is itself a proto.
	// Message), so a *schemapb.Baked struct field round-trips through
	// json.Marshal fine but fails json.Unmarshal back into the oneof's
	// interface-typed Kind field — this crashed RunRecipeWorkflow before a
	// single line of workflow code ran (SP-D live-stand bug 2). []byte is
	// plain JSON-serializable data (base64), sidestepping the issue entirely
	// without a custom DataConverter. proto.Marshal is deterministic for a
	// fixed message (field order is defined by the descriptor, not map
	// iteration — schemapb.Baked has no map fields), so this is safe to
	// depend on for Temporal replay: the exact same bytes are recorded in
	// the initial WorkflowExecutionStarted history event on every replay,
	// since Baked is set once, here, at workflow-start time and never
	// mutated afterward (never re-sent per attempt/heartbeat — see the
	// project's own history-bloat precedent, persistRunCompiledPlan's doc
	// comment, for why that distinction matters here).
	Baked []byte
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
	// Baked mirrors RunRecipeInput.Baked (see its doc comment): the sealed
	// launch-form snapshot as raw proto.Marshal bytes, not *schemapb.Baked.
	// CompileRecipeActivity (internal/infrastructure/execution/
	// recipe_activities.go) unmarshals it back into *schemapb.Baked (a real
	// proto.Message, decoded with proto.Unmarshal — no oneof/DataConverter
	// hazard, since this happens in plain activity code, not via Temporal's
	// converter) before passing it to dslservice.CompileBundle, which derives
	// the provider params schema it needs to validate Baked's provider
	// params directly from Bundle itself (see CompileBundle's own doc
	// comment). This input deliberately does NOT also carry a
	// separately-derived *schemapb.Schema: that would be a second, larger
	// value recorded in this activity's own history entry for no benefit,
	// since Bundle is already present to re-derive it from.
	Baked []byte
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
	// RunID, ServerAddr and GatewayGroup carry the infra-authored runtime
	// context the docker builtin provider needs but a recipe's cluster.yaml
	// provider.params cannot supply: the run's id (used to name the shared
	// docker network and containers), the agent-facing server address the
	// bootstrap dials home to, and which MachineGroup hosts the Nomad gateway
	// sidecar. ProvisionActivity injects them into a docker ProviderRef's
	// ParamsJson before calling Provision; non-docker providers ignore them.
	RunID        string
	ServerAddr   string
	GatewayGroup string
	// TenantID scopes the per-node agent tokens the docker builtin provider
	// issues while rendering each agent container (IssueAgentToken needs the
	// owning tenant). Non-docker providers ignore it.
	TenantID string
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
	// RunID and ServerAddr mirror ProvisionActivityInput's runtime triad:
	// the docker builtin's Destroy needs the run id (to compute the shared
	// network name it removes containers from), and decodeDockerParams —
	// shared with Provision — still validates server_addr, so it is injected
	// here too even though Destroy itself never reads it. Non-docker refs
	// ignore both. GatewayGroup is not carried: Destroy renders no sidecar,
	// so the group placement is irrelevant at teardown.
	RunID      string
	ServerAddr string
	// TenantID scopes F1's EnvFn credential resolution: Destroy must resolve
	// the same tenant's deploy creds Provision used, not a process-wide
	// default. Not a secret — an ordinary identifier, safe in Temporal
	// history like RunID/ServerAddr above.
	TenantID string
}

// ReserveQuotasActivityInput is ReserveQuotasActivity's input: the compiled
// plan's provider name (plan.GetProvider().GetName() — "docker"/"yandex",
// see quotas.providerFromDslName) and machine groups, plus the run's
// identity — mirrors quotas.Manager.Reserve's own (tenantID, runID,
// workflowID, providerName, groups) argument shape.
type ReserveQuotasActivityInput struct {
	TenantID   string
	RunID      string
	WorkflowID string
	Provider   string
	Groups     []*dslpb.MachineGroup
}

// CommitQuotasActivityInput is CommitQuotasActivity's input — mirrors
// quotas.Manager.Commit's (tenantID, runID) argument shape.
type CommitQuotasActivityInput struct {
	TenantID string
	RunID    string
}

// ReleaseQuotasActivityInput is ReleaseQuotasActivity's input — mirrors
// quotas.Manager.Release's (tenantID, runID) argument shape.
type ReleaseQuotasActivityInput struct {
	TenantID string
	RunID    string
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
		// teardown and releaseQuotas both run unconditionally — a
		// TeardownActivity failure must never skip freeing the run's quota
		// reservation (and vice versa), since either alone would strand a
		// resource (infrastructure or quota capacity) that this run no
		// longer needs. See releaseQuotas' own doc comment for why this is
		// always safe to call, even when Reserve was never called for this
		// run (e.g. the compile stage failed before an infra stage ever
		// ran).
		terr := w.teardown(tctx, providerRef)
		rerr := w.releaseQuotas(tctx)
		switch {
		case terr != nil && rerr != nil:
			w.failStage(tctx, runRecipeStageTeardownIndex, fmt.Sprintf("teardown: %s; release quotas: %s", terr.Error(), rerr.Error()))
			stageFailed = true
			err = teardownErr(err, fmt.Errorf("teardown: %w; release quotas: %w", terr, rerr))
		case terr != nil:
			w.failStage(tctx, runRecipeStageTeardownIndex, terr.Error())
			stageFailed = true
			err = teardownErr(err, fmt.Errorf("teardown: %w", terr))
		case rerr != nil:
			w.failStage(tctx, runRecipeStageTeardownIndex, rerr.Error())
			stageFailed = true
			err = teardownErr(err, fmt.Errorf("release quotas: %w", rerr))
		default:
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
	// Persist the compiled plan itself onto the run record — SP-E Task 3:
	// before this, the plan lived only in this workflow's memory and was
	// never durably stored (RunDetail/rerun/audit had no way to see exactly
	// what a recipe run executed). Called exactly once, right here, never
	// again for this run (no re-compile mid-run) — deliberately NOT wired
	// into persist()/every heartbeat, mirroring the project's own prior
	// history-bloat lesson (a deploy plan re-sent on every persist bloated
	// Temporal history badly enough to need a fix — see runtime.go's
	// persistRunCompiledPlan doc). Ordered before persistRunSummary below:
	// both derive from the same just-compiled plan at the same commit
	// point, so their relative order is arbitrary but fixed here.
	if perr := persistRunCompiledPlan(ctx, w.in.RunID, plan); perr != nil {
		return nil, perr
	}
	// Stamp the recipe-derived Summary facets (Provider/NodeCount/
	// TopologyLabel/DbKind/WorkloadName/StroppyVersion) onto the run record
	// as soon as the plan exists — this is the ROOT fix for rating/metrics/
	// compare/share/dashboard, all of which read Summary rather than a
	// baked domain.TestRun spec (Run has none). See runrecipe_summary.go's
	// deriveRunSummary for the derivation heuristics.
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
	if rerr := w.reserveQuotas(ctx, plan); rerr != nil {
		if temporal.IsCanceledError(rerr) {
			return nil, rerr //nolint:wrapcheck // propagated verbatim — see compileRecipe's identical guard above.
		}
		w.failStage(ctx, runRecipeStageInfraIndex, rerr.Error())
		stageFailed = true
		if perr := w.persist(ctx); perr != nil {
			return nil, perr
		}
		return nil, nil //nolint:nilnil // see above.
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
	// Stamp the recipe topology snapshot (machines + their group/service
	// placement) onto the run record now that ProvisionActivity has
	// returned real machines — this is the ROOT fix for recipe-run topology
	// (overview.go/runtime_topology.go project it for a recipe run in place
	// of the TopologySpec/InfrastructureState/DeploymentPlan trio a classic
	// run has). See runrecipe_topology.go's deriveRecipeTopology.
	if perr := persistRecipeTopology(ctx, w.in.RunID, deriveRecipeTopology(plan, machines)); perr != nil {
		return nil, perr
	}
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
	if cerr := w.commitQuotas(ctx); cerr != nil {
		if temporal.IsCanceledError(cerr) {
			return nil, cerr //nolint:wrapcheck // propagated verbatim — see compileRecipe's identical guard above.
		}
		w.failStage(ctx, runRecipeStageExecuteIndex, cerr.Error())
		stageFailed = true
		if perr := w.persist(ctx); perr != nil {
			return nil, perr
		}
		return nil, nil //nolint:nilnil // see above.
	}
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
		Baked:  w.in.Baked,
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
		Groups:       plan.GetMachineGroups(),
		ProviderRef:  ref,
		RunID:        w.in.RunID,
		ServerAddr:   w.in.Bootstrap.GetServerAddr(),
		GatewayGroup: gatewayGroupName(plan),
		TenantID:     w.in.TenantID,
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
		Bootstrap:     bootstrapWithAgentQueues(w.in.Bootstrap, machines),
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
		RunID:       w.in.RunID,
		ServerAddr:  w.in.Bootstrap.GetServerAddr(),
		TenantID:    w.in.TenantID,
	}).Get(actx, nil)
}

// reserveQuotas calls ReserveQuotasActivity for plan's provider and machine
// groups, run in the infra stage BEFORE provision (see run's infra-stage
// block) — a non-nil error here fails the infra stage and skips
// ProvisionActivity entirely, exactly like a ProvisionActivity failure
// itself. workflow.GetInfo(ctx).WorkflowExecution.ID is threaded through as
// the reservation's WorkflowID (mirrors the pre-DSL-pivot AcquireQuotasActivity
// caller's own convention of stamping the acquiring workflow's id onto each
// reservation row).
func (w *runRecipeWorkflow) reserveQuotas(ctx workflow.Context, plan *dslpb.CompiledPlan) error {
	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
	})
	return workflow.ExecuteActivity(actx, ReserveQuotasActivityName, &ReserveQuotasActivityInput{
		TenantID:   w.in.TenantID,
		RunID:      w.in.RunID,
		WorkflowID: workflow.GetInfo(ctx).WorkflowExecution.ID,
		Provider:   plan.GetProvider().GetName(),
		Groups:     plan.GetMachineGroups(),
	}).Get(actx, nil)
}

// commitQuotas calls CommitQuotasActivity once ExecuteCompiledPlanWorkflow
// has succeeded (see run's execute-stage block) — promotes the run's
// reservation to allocated.
func (w *runRecipeWorkflow) commitQuotas(ctx workflow.Context) error {
	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
	})
	return workflow.ExecuteActivity(actx, CommitQuotasActivityName, &CommitQuotasActivityInput{
		TenantID: w.in.TenantID,
		RunID:    w.in.RunID,
	}).Get(actx, nil)
}

// releaseQuotas calls ReleaseQuotasActivity — run unconditionally from the
// teardown defer (see run's deferred func), regardless of whether
// reserveQuotas ever ran or succeeded for this run: freeing a reservation
// that was never made is a documented no-op (see quotas.Store.ReleaseRun).
func (w *runRecipeWorkflow) releaseQuotas(ctx workflow.Context) error {
	actx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
	})
	return workflow.ExecuteActivity(actx, ReleaseQuotasActivityName, &ReleaseQuotasActivityInput{
		TenantID: w.in.TenantID,
		RunID:    w.in.RunID,
	}).Get(actx, nil)
}

// teardownErr folds next onto existing (nil-safe) — used by the teardown
// defer to combine a pre-existing workflow error (e.g. a persist failure
// observed earlier in the same defer) with a teardown/release-quotas
// failure discovered afterward, without ever losing either message.
func teardownErr(existing, next error) error {
	if existing != nil {
		return fmt.Errorf("%w; %w", existing, next)
	}
	return next
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
// gatewayGroupName selects, by MachineGroup name alone, which group hosts the
// Nomad gateway sidecar the docker builtin provider renders. It mirrors
// pickGatewayNodeID's group-selection convention (prefer a group literally
// named "runner" then "gateway", else the first declared group) but runs
// before any machine is provisioned — Provision needs the group name to decide
// where to place the sidecar, whereas pickGatewayNodeID needs the resulting
// node id after provisioning. Returns "" when the plan has no machine groups,
// in which case no sidecar is rendered.
// bootstrapWithAgentQueues returns a clone of bootstrap whose AgentTaskQueues
// map is populated with an entry per provisioned machine: node id ->
// agentdomain.TaskQueue(nodeID). The execute child's service-job steps route
// activities to bootstrap.AgentTaskQueues[nodeID] (see agent_exec.go's
// agentTaskQueue lookup), so without this every service job fails with "agent
// task queue for node ... is missing".
//
// The queue name is the DETERMINISTIC TaskQueue(nodeID) — not the nonce-bearing
// NewTaskQueue — because the docker builtin provider renders each agent
// container's AGENT_TASK_QUEUE from the same deterministic TaskQueue(nodeID)
// (internal/domain/agent.Env, called with an empty Bootstrap.AgentTaskQueue),
// so both sides must agree on the exact string. TaskQueue is a pure function
// of nodeID, so calling it here is workflow-deterministic. Returns bootstrap
// unchanged when it is nil (compile failed before a bootstrap existed) or when
// there are no machines.
func bootstrapWithAgentQueues(bootstrap *workflowpb.AgentBootstrap, machines map[string][]*deploymentpb.MachineState) *workflowpb.AgentBootstrap {
	if bootstrap == nil || len(machines) == 0 {
		return bootstrap
	}
	out := proto.Clone(bootstrap).(*workflowpb.AgentBootstrap)
	if out.AgentTaskQueues == nil {
		out.AgentTaskQueues = map[string]string{}
	}
	for _, group := range machines {
		for _, m := range group {
			nodeID := m.GetNodeId()
			if nodeID == "" {
				continue
			}
			out.AgentTaskQueues[nodeID] = agentdomain.TaskQueue(nodeID)
		}
	}
	return out
}

func gatewayGroupName(plan *dslpb.CompiledPlan) string {
	groups := plan.GetMachineGroups()
	for _, name := range []string{"runner", "gateway"} {
		for _, mg := range groups {
			if mg.GetName() == name {
				return name
			}
		}
	}
	if len(groups) == 0 {
		return ""
	}
	return groups[0].GetName()
}

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
