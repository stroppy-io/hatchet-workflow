package execution

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"

	deploymentbuilder "github.com/stroppy-io/stroppy-cloud/internal/domain/deployment"
	"go.temporal.io/sdk/client"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_run_overview"
)

// streamInterval is how often Stream re-polls GetRunState and emits a snapshot.
const streamInterval = time.Second

// overviewRunStateQueryTimeout bounds the one live Temporal query used by the
// detail overview. The Overview endpoint must degrade to the persisted runtime
// projection instead of hanging the UI when Temporal is slow or the workflow is
// already closed.
const overviewRunStateQueryTimeout = 4 * time.Second

const (
	stageInfrastructureNodeName       = "infrastructure"
	stageRenderDeploymentPlanNodeName = "render_deployment_plan"
	executeDeploymentPlanNodeName     = "execute_deployment_plan"
	stageWorkloadNodeName             = "workload"
	stageTeardownNodeName             = "teardown"
)

// runStateQuerier is the minimal slice of workflowpb.TestServiceClient the reader
// depends on for the live run state. Kept narrow so the reader is unit-testable
// without a live Temporal server.
type runStateQuerier interface {
	GetRunState(ctx context.Context, workflowID string, runID string) (*workflowpb.RunState, error)
}

// SnapshotRunReader loads the persisted run record that the snapshot bundles
// alongside the live workflow projection. It is a read-only, tenant-agnostic
// lookup by id (the service has already authorised the caller). Implemented by
// the storage layer; injected so the overview reader stays an execution
// adapter without owning storage.
type SnapshotRunReader interface {
	// RunRecord returns the persisted models.Run for the run id.
	RunRecord(ctx context.Context, runID string) (*models.Run, error)
}

// AgentPresenceReader returns the registry-backed liveness samples for run
// workers. Missing entries mean the registry has not seen that machine for the
// requested run.
type AgentPresenceReader interface {
	AgentPresence(ctx context.Context, runID string, machineIDs []string) (map[string]*monitor.WorkerInfo, error)
}

// OverviewReader implements test_run_overview.OverviewReader. It assembles the
// snapshot from (a) the persisted run record and its staged topology, and (b)
// the live monitor.Overview projected from the Temporal TestWorkflow's
// RunState.
type OverviewReader struct {
	tc                   runStateQuerier
	store                SnapshotRunReader
	presence             AgentPresenceReader
	runStateQueryTimeout time.Duration
}

var _ test_run_overview.OverviewReader = (*OverviewReader)(nil)

// NewOverviewReader builds the test_run_overview.OverviewReader adapter over a
// Temporal client (the live RunState source) and a SnapshotRunReader (the
// persisted run + topology source). store may be nil, in which case the
// snapshot carries only the live Overview projection.
func NewOverviewReader(c client.Client, store SnapshotRunReader, presence ...AgentPresenceReader) *OverviewReader {
	r := &OverviewReader{tc: workflowpb.NewTestServiceClient(c), store: store}
	if len(presence) > 0 {
		r.presence = presence[0]
	}
	return r
}

// Get returns a one-shot snapshot for the run. The live Overview degrades to
// PENDING when the workflow has not started yet (the UI polls before it exists);
// only context cancellation is surfaced as an error.
func (r *OverviewReader) Get(ctx context.Context, runID string) (*api.TestRunOverviewSnapshot, error) {
	snap := &api.TestRunOverviewSnapshot{}
	var run *models.Run
	observedAt := timestamppb.Now()
	degradedReasons := make([]string, 0, 2)

	if r.store != nil {
		var err error
		run, err = r.store.RunRecord(ctx, runID)
		if err != nil {
			return nil, err
		}
		snap.Topology = topologyFromRun(run)
	}

	presence, err := r.agentPresence(ctx, runID, run)
	if err != nil {
		degradedReasons = append(degradedReasons, "agent_registry_unavailable: "+err.Error())
	}

	// finish converts the accumulated models.Run into the api.
	// TestRunOverviewSnapshot.Run shape and returns the snapshot. Deferred to
	// the end of every return path (rather than assigned once up front) so
	// overlayRunFromOverview's status/summary mutation below is reflected in
	// the converted value too.
	finish := func() (*api.TestRunOverviewSnapshot, error) {
		snap.Run = run
		return snap, nil
	}

	if persistedRunIsTerminal(run) {
		snap.Overview = overviewFromRun(runID, run, presence, observedAt)
		snap.Overview.DegradedReasons = append(snap.Overview.GetDegradedReasons(), degradedReasons...)
		return finish()
	}

	if r.tc == nil {
		degradedReasons = append(degradedReasons, "workflow_state_unavailable: temporal_client_not_configured")
		snap.Overview = overviewFromRun(runID, run, presence, observedAt)
		snap.Overview.DegradedReasons = append(snap.Overview.GetDegradedReasons(), degradedReasons...)
		return finish()
	}

	// Every models.Run executes under RunRecipeWorkflow, addressed by
	// runRecipeWorkflowID(runID) — see ids.go. Unlike the retired classic
	// TestWorkflow (unregistered, spec §1), there is no other workflow kind
	// to branch on: a Run always carries a workflow_id, so this is now
	// unconditional. If the query misses (e.g. workflow not yet started or
	// already closed), the existing fallback below to overviewFromRun
	// (projecting run.GetRuntimeState(), the last RunState the workflow
	// itself persisted) is unchanged and still correct.
	workflowID := runRecipeWorkflowID(runID)
	queryCtx, cancel := context.WithTimeout(ctx, r.effectiveRunStateQueryTimeout())
	defer cancel()
	rs, err := r.tc.GetRunState(queryCtx, workflowID, "")
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		// The workflow may not be queryable yet or may already be closed. The
		// persisted record is the durable fallback; only a never-started record
		// degrades to PENDING.
		degradedReasons = append(degradedReasons, "workflow_state_unavailable: "+err.Error())
		snap.Overview = overviewFromRun(runID, run, presence, observedAt)
		snap.Overview.DegradedReasons = append(snap.Overview.GetDegradedReasons(), degradedReasons...)
		return finish()
	}
	snap.Overview = mergeOverviewWithRun(projectOverview(runID, rs, observedAt), run, presence)
	if run != nil {
		snap.Topology = topologyFromRunWithRunState(run, rs)
	}
	snap.Overview.DegradedReasons = append(snap.Overview.GetDegradedReasons(), degradedReasons...)
	overlayRunFromOverview(run, snap.Overview)
	return finish()
}

func (r *OverviewReader) effectiveRunStateQueryTimeout() time.Duration {
	if r != nil && r.runStateQueryTimeout > 0 {
		return r.runStateQueryTimeout
	}
	return overviewRunStateQueryTimeout
}

func persistedRunIsTerminal(run *models.Run) bool {
	if run == nil {
		return false
	}
	return isTerminalStatus(run.GetStatus()) || isTerminalStatus(run.GetRuntimeState().GetStatus())
}

// Stream pushes a fresh full snapshot every tick until ctx is cancelled or the
// run reaches a terminal state, then closes the channel.
func (r *OverviewReader) Stream(ctx context.Context, runID string) (<-chan *api.TestRunOverviewSnapshot, error) {
	out := make(chan *api.TestRunOverviewSnapshot)
	go func() {
		defer close(out)

		send := func(s *api.TestRunOverviewSnapshot) bool {
			select {
			case <-ctx.Done():
				return false
			case out <- s:
				return true
			}
		}

		emit := func() (terminal bool) {
			snap, err := r.Get(ctx, runID)
			if err != nil {
				return true // ctx cancelled or hard store error: stop
			}
			if !send(snap) {
				return true
			}
			return isTerminalStatus(snap.GetOverview().GetStatus())
		}

		if emit() {
			return
		}

		ticker := time.NewTicker(streamInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if emit() {
					return
				}
			}
		}
	}()
	return out, nil
}

// topologyFromRun assembles the staged topology envelope from a run's
// current runtime_state.
func topologyFromRun(run *models.Run) *topology.Topology {
	return topologyFromRunWithRunState(run, run.GetRuntimeState())
}

func topologyFromRunWithRunState(run *models.Run, rs *workflowpb.RunState) *topology.Topology {
	if run == nil {
		return nil
	}
	// Spec/InfrastructurePlan/InfrastructureState/DeploymentPlan/Tags stay
	// unset: models.Run never carries a baked domain.TestRun spec or a
	// classic deployment.InfrastructureState/DeploymentPlan (see Run's own
	// doc comment) — those fields existed only for the retired classic
	// TestWorkflow (spec §1: domainTestWorkflow is unregistered).
	t := &topology.Topology{}
	t.State = runTopologyState(run)
	t.RuntimeNodes, t.RuntimeConnections = runtimeTopologyFromRun(run, rs)
	return t
}

// runTopologyState classifies how far the topology has materialized from the
// run's filled artifacts. A recipe run never produces a deployment.
// InfrastructureState/DeploymentPlan (models.Run has no such fields at all)
// — its topology snapshot is filled once, right after ProvisionActivity,
// which is the closest equivalent to STATE_INFRASTRUCTURE_DEPLOYED.
func runTopologyState(run *models.Run) topology.Topology_State {
	if run.GetTopology() != nil {
		return topology.Topology_STATE_INFRASTRUCTURE_DEPLOYED
	}
	return topology.Topology_STATE_UNSPECIFIED
}

// pendingOverview is the snapshot returned before the workflow exists: just the
// run id, a PENDING status and empty collections.
func pendingOverview(runID string, observedAt *timestamppb.Timestamp) *monitor.Overview {
	return &monitor.Overview{
		RunId:      runID,
		Status:     common.Status_STATUS_PENDING,
		Pipeline:   &monitor.PipelineView{},
		Workers:    []*monitor.WorkerInfo{},
		Timeline:   []*monitor.Event{},
		ObservedAt: observedAt,
		Source:     monitor.ObservationSource_OBSERVATION_SOURCE_SYNTHETIC,
	}
}

func overviewFromRun(runID string, run *models.Run, presence map[string]*monitor.WorkerInfo, observedAt *timestamppb.Timestamp) *monitor.Overview {
	if run == nil {
		return pendingOverview(runID, observedAt)
	}
	if run.GetRuntimeState() != nil {
		overview := projectOverviewWithSource(runID, run.GetRuntimeState(), observedAt, monitor.ObservationSource_OBSERVATION_SOURCE_PERSISTED_RECORD)
		overview.Workers = workersFromRun(run, presence)
		sum := run.GetSummary()
		runtimeTerminal := isTerminalStatus(run.GetRuntimeState().GetStatus())
		stored := run.GetStatus()
		if stored != common.Status_STATUS_UNSPECIFIED && (!runtimeTerminal || stored == common.Status_STATUS_CANCELLING || isTerminalStatus(stored)) {
			overview.Status = run.GetStatus()
		}
		if sum.GetStartedAt() != nil {
			overview.StartedAt = sum.GetStartedAt()
		}
		if sum.GetFinishedAt() != nil {
			overview.FinishedAt = sum.GetFinishedAt()
		}
		// Only trust the stored duration once the run is terminal. While running
		// it is a frozen snapshot from the last persist, which made the UI
		// duration look stuck; recompute it live against the observation time.
		if runtimeTerminal && sum.GetDuration() != nil {
			overview.Duration = sum.GetDuration()
		} else if overview.GetStartedAt() != nil {
			overview.Duration = spanDuration(overview.GetStartedAt(), overview.GetFinishedAt(), observedAt.AsTime())
		}
		if sum.GetProgressPct() > overview.GetProgressPct() {
			overview.ProgressPct = sum.GetProgressPct()
		}
		return overview
	}
	status := run.GetStatus()
	if status == common.Status_STATUS_UNSPECIFIED {
		status = common.Status_STATUS_PENDING
	}
	sum := run.GetSummary()
	return &monitor.Overview{
		RunId:       runID,
		Status:      status,
		StartedAt:   sum.GetStartedAt(),
		FinishedAt:  sum.GetFinishedAt(),
		Duration:    sum.GetDuration(),
		ProgressPct: sum.GetProgressPct(),
		Pipeline:    pipelineFromRun(runID, run),
		Workers:     workersFromRun(run, presence),
		Timeline:    []*monitor.Event{},
		ObservedAt:  observedAt,
		Source:      monitor.ObservationSource_OBSERVATION_SOURCE_PERSISTED_RECORD,
	}
}

func mergeOverviewWithRun(overview *monitor.Overview, run *models.Run, presence map[string]*monitor.WorkerInfo) *monitor.Overview {
	if overview == nil {
		return overviewFromRun("", run, presence, timestamppb.Now())
	}
	if run == nil {
		return overview
	}
	overview.Workers = workersFromRun(run, presence)
	stored := run.GetStatus()
	if stored == common.Status_STATUS_CANCELLING || isTerminalStatus(stored) {
		sum := run.GetSummary()
		overview.Status = stored
		if sum.GetStartedAt() != nil {
			overview.StartedAt = sum.GetStartedAt()
		}
		if sum.GetFinishedAt() != nil {
			overview.FinishedAt = sum.GetFinishedAt()
		}
		if sum.GetDuration() != nil {
			overview.Duration = sum.GetDuration()
		}
		if sum.GetProgressPct() > overview.GetProgressPct() {
			overview.ProgressPct = sum.GetProgressPct()
		}
	}
	return overview
}

// pipelineFromRun renders the classic 5-stage pipeline skeleton from a run's
// own status when no live/persisted RunState exists yet (the very first
// snapshot after a run is created, before RunRecipeWorkflow reports its
// first stage). Per-component detail (deployment.DeploymentPlan's
// component/step tree) has no equivalent on models.Run — RunRecipeWorkflow's
// own dynamic stage names take over via projectOverviewWithSource /
// overviewFromRun's runtime_state branch as soon as the first stage lands.
func pipelineFromRun(runID string, run *models.Run) *monitor.PipelineView {
	pipeline := &monitor.PipelineView{}
	if run == nil {
		return pipeline
	}
	pipeline.Roots = []*monitor.PipelineNode{
		runStageNode(runID, run, stageInfrastructureNodeName, 1, runInfrastructureStatus(run)),
		runStageNode(runID, run, stageRenderDeploymentPlanNodeName, 2, runRenderPlanStatus(run)),
		runStageNode(runID, run, executeDeploymentPlanNodeName, 3, runExecutePlanStatus(run)),
		runStageNode(runID, run, stageWorkloadNodeName, 4, runWorkloadStatus(run)),
		runStageNode(runID, run, stageTeardownNodeName, 5, runTeardownStatus(run)),
	}
	return pipeline
}

func (r *OverviewReader) agentPresence(ctx context.Context, runID string, run *models.Run) (map[string]*monitor.WorkerInfo, error) {
	if r == nil || r.presence == nil || run == nil {
		return nil, nil
	}
	machineIDs := workerMachineIDsFromRun(run)
	if len(machineIDs) == 0 {
		return nil, nil
	}
	return r.presence.AgentPresence(ctx, runID, machineIDs)
}

// workerMachineIDsFromRun sources node ids from run.topology.nodes — the
// only per-machine artifact models.Run carries (see Task 4's discovered-gap
// fix note: the classic InfrastructureState/DeploymentPlan machine lookup
// this replaced was always empty for a recipe run, since Run never has
// those fields).
func workerMachineIDsFromRun(run *models.Run) []string {
	if run == nil {
		return nil
	}
	nodeIDs := make(map[string]struct{})
	for _, node := range run.GetTopology().GetNodes() {
		if node == nil || node.GetNodeId() == "" {
			continue
		}
		nodeIDs[node.GetNodeId()] = struct{}{}
	}
	ordered := make([]string, 0, len(nodeIDs))
	for nodeID := range nodeIDs {
		ordered = append(ordered, nodeID)
	}
	sort.Strings(ordered)
	return ordered
}

// workersFromRun builds the Agents tab's persisted-fallback worker list from
// run.topology.nodes (models.RunTopology_MachineNode), which already carries
// everything a worker row needs (node_id/group/ip/status/services) —
// replacing the old deploymentpb.MachineState/DeploymentPlan lookup, which
// was structurally empty for every live recipe run (see Task 4's discovered
// gap). current_node_execution_id has no equivalent on the topology
// snapshot (a point-in-time fact, not live-updated — see RunTopology's own
// doc) and stays unset, same as it already effectively was under the old,
// always-empty DeploymentPlan lookup.
func workersFromRun(run *models.Run, presence map[string]*monitor.WorkerInfo) []*monitor.WorkerInfo {
	if run == nil {
		return nil
	}

	nodesByID := make(map[string]*models.RunTopology_MachineNode)
	for _, node := range run.GetTopology().GetNodes() {
		if node == nil || node.GetNodeId() == "" {
			continue
		}
		nodesByID[node.GetNodeId()] = node
	}
	ordered := workerMachineIDsFromRun(run)

	workers := make([]*monitor.WorkerInfo, 0, len(ordered))
	for _, nodeID := range ordered {
		node := nodesByID[nodeID]
		status := pendingIfUnspecified(node.GetStatus())
		worker := &monitor.WorkerInfo{
			Id:           "agent/" + nodeID,
			Kind:         domainpb.Worker_KIND_AGENT,
			MachineId:    nodeID,
			Host:         node.GetIp(),
			Status:       status,
			Presence:     runWorkerPresence(run, node),
			Source:       monitor.ObservationSource_OBSERVATION_SOURCE_PERSISTED_RECORD,
			StatusReason: runWorkerStatusReason(run, node),
		}
		worker.Online = worker.GetPresence() == monitor.WorkerPresence_WORKER_PRESENCE_ONLINE
		mergeWorkerPresence(worker, presence[nodeID])
		workers = append(workers, worker)
	}
	return workers
}

func mergeWorkerPresence(worker *monitor.WorkerInfo, sample *monitor.WorkerInfo) {
	if worker == nil || sample == nil {
		return
	}
	if sample.GetHost() != "" {
		worker.Host = sample.GetHost()
	}
	worker.RegisteredAt = sample.GetRegisteredAt()
	worker.LastSeenAt = sample.GetLastSeenAt()
	worker.HeartbeatIntervalSeconds = sample.GetHeartbeatIntervalSeconds()
	worker.AgentVersion = sample.GetAgentVersion()
	worker.RunId = sample.GetRunId()
	if worker.GetPresence() == monitor.WorkerPresence_WORKER_PRESENCE_TERMINATED {
		worker.Online = false
		worker.StatusReason = "run_terminal_agent_terminated"
		return
	}
	worker.Online = sample.GetPresence() == monitor.WorkerPresence_WORKER_PRESENCE_ONLINE
	worker.Presence = sample.GetPresence()
	worker.StatusReason = sample.GetStatusReason()
	worker.Source = monitor.ObservationSource_OBSERVATION_SOURCE_AGENT_REGISTRY
}

// runWorkerPresence reports TERMINATED for a terminal run; otherwise UNKNOWN
// pending a real agent-registry sample (mergeWorkerPresence upgrades this
// once one arrives). Collapses the old machine-liveness branch: it always
// returned UNKNOWN either way (workerOnline(machine) was never able to
// distinguish "online" from "unknown" here — a registry sample was the only
// source of truth for that), so dropping it is not a behavior change.
func runWorkerPresence(run *models.Run, node *models.RunTopology_MachineNode) monitor.WorkerPresence {
	if run != nil && isTerminalStatus(run.GetStatus()) {
		return monitor.WorkerPresence_WORKER_PRESENCE_TERMINATED
	}
	return monitor.WorkerPresence_WORKER_PRESENCE_UNKNOWN
}

func runWorkerStatusReason(run *models.Run, node *models.RunTopology_MachineNode) string {
	if run != nil && isTerminalStatus(run.GetStatus()) {
		return "run_terminal_no_registry_sample"
	}
	if node == nil {
		return "machine_not_materialized"
	}
	return "no_registry_sample"
}

func workerStatus(machine *deploymentpb.MachineState, currentNodeExecutionID string) common.Status {
	if currentNodeExecutionID != "" {
		return common.Status_STATUS_RUNNING
	}
	if machine == nil {
		return common.Status_STATUS_PENDING
	}
	return pendingIfUnspecified(machine.GetStatus())
}

func machineHost(machine *deploymentpb.MachineState) string {
	if machine == nil {
		return ""
	}
	for _, name := range []string{"private", "public"} {
		for _, endpoint := range machine.GetEndpoints() {
			if endpoint.GetName() == name && endpoint.GetAddress() != "" {
				return endpoint.GetAddress()
			}
		}
	}
	for _, endpoint := range machine.GetEndpoints() {
		if endpoint.GetAddress() != "" {
			return endpoint.GetAddress()
		}
	}
	return ""
}

func runStageNode(runID string, run *models.Run, name string, order uint32, status common.Status) *monitor.PipelineNode {
	id := deploymentbuilder.StageExecutionID(name)
	status = pendingIfUnspecified(status)
	return &monitor.PipelineNode{
		NodeExecutionId: id,
		Name:            name,
		Status:          status,
		LogRef:          logRef(runID, id, ""),
		Attempt:         1,
		Source:          monitor.ObservationSource_OBSERVATION_SOURCE_PERSISTED_RECORD,
		Order:           order,
		Phase:           name,
		StatusReason:    runStageStatusReason(run, name, status),
	}
}

// runInfrastructureStatus derives the pre-runtime_state "infrastructure"
// stage status from run.topology alone (models.Run has no
// InfrastructureState machine list to inspect per-machine) — provisioned
// (topology != nil) reads COMPLETED, else PENDING/terminal-passthrough.
func runInfrastructureStatus(run *models.Run) common.Status {
	if run.GetTopology() != nil {
		return common.Status_STATUS_COMPLETED
	}
	if isTerminalStatus(run.GetStatus()) && run.GetStatus() != common.Status_STATUS_COMPLETED {
		return run.GetStatus()
	}
	return common.Status_STATUS_PENDING
}

// runRenderPlanStatus's classic DeploymentPlan-presence check has no
// equivalent on models.Run; compiled_plan (the compiled DSL plan
// RunRecipeWorkflow executed) is the closest analog for "the plan is
// rendered".
func runRenderPlanStatus(run *models.Run) common.Status {
	if run.GetCompiledPlan() != nil {
		return common.Status_STATUS_COMPLETED
	}
	if isTerminalStatus(run.GetStatus()) && runInfrastructureStatus(run) == common.Status_STATUS_COMPLETED && run.GetStatus() != common.Status_STATUS_COMPLETED {
		return run.GetStatus()
	}
	if runInfrastructureStatus(run) == common.Status_STATUS_COMPLETED && run.GetStatus() == common.Status_STATUS_RUNNING {
		return common.Status_STATUS_RUNNING
	}
	return common.Status_STATUS_PENDING
}

// runExecutePlanStatus has no per-component DeploymentPlan detail to derive
// from on models.Run; this only fires before the first runtime_state stage
// lands (overviewFromRun's runtime_state branch takes over immediately
// after), so a coarse status/topology-driven signal is all that's needed.
func runExecutePlanStatus(run *models.Run) common.Status {
	if run.GetStatus() == common.Status_STATUS_COMPLETED && run.GetTopology() != nil {
		return common.Status_STATUS_COMPLETED
	}
	if isTerminalStatus(run.GetStatus()) && runRenderPlanStatus(run) == common.Status_STATUS_COMPLETED {
		return run.GetStatus()
	}
	return common.Status_STATUS_PENDING
}

func runWorkloadStatus(run *models.Run) common.Status {
	switch run.GetStatus() {
	case common.Status_STATUS_COMPLETED:
		return common.Status_STATUS_COMPLETED
	case common.Status_STATUS_FAILED, common.Status_STATUS_CANCELLED:
		if runExecutePlanStatus(run) == common.Status_STATUS_COMPLETED {
			return run.GetStatus()
		}
		return common.Status_STATUS_PENDING
	case common.Status_STATUS_RUNNING, common.Status_STATUS_CANCELLING:
		if runExecutePlanStatus(run) == common.Status_STATUS_COMPLETED {
			return run.GetStatus()
		}
	}
	return common.Status_STATUS_PENDING
}

func runTeardownStatus(run *models.Run) common.Status {
	switch run.GetStatus() {
	case common.Status_STATUS_COMPLETED:
		return common.Status_STATUS_COMPLETED
	case common.Status_STATUS_FAILED:
		if runWorkloadStatus(run) == common.Status_STATUS_COMPLETED {
			return common.Status_STATUS_FAILED
		}
	case common.Status_STATUS_CANCELLED:
		if runInfrastructureStatus(run) == common.Status_STATUS_COMPLETED {
			return common.Status_STATUS_CANCELLED
		}
	case common.Status_STATUS_RUNNING, common.Status_STATUS_CANCELLING:
		if runWorkloadStatus(run) == common.Status_STATUS_COMPLETED {
			return common.Status_STATUS_RUNNING
		}
	}
	return common.Status_STATUS_PENDING
}

func runStageStatusReason(run *models.Run, name string, status common.Status) string {
	if run != nil && isTerminalStatus(run.GetStatus()) {
		return "persisted_record_terminal_" + strings.ToLower(run.GetStatus().String())
	}
	if status == common.Status_STATUS_PENDING {
		return "persisted_record_missing_live_stage"
	}
	return "persisted_record_" + strings.ToLower(status.String())
}

func pendingIfUnspecified(status common.Status) common.Status {
	if status == common.Status_STATUS_UNSPECIFIED {
		return common.Status_STATUS_PENDING
	}
	return status
}

func labelOr(labels map[string]string, key, fallback string) string {
	if value := labels[key]; value != "" {
		return value
	}
	return fallback
}

func logRef(runID, nodeExecutionID, componentID string) *monitor.LogRef {
	return &monitor.LogRef{
		RunId:           runID,
		NodeExecutionId: refString(nodeExecutionID),
		ComponentId:     refString(componentID),
	}
}

func overlayRunFromOverview(run *models.Run, overview *monitor.Overview) {
	if run == nil || overview == nil {
		return
	}
	if run.GetStatus() != common.Status_STATUS_CANCELLING && !isTerminalStatus(run.GetStatus()) {
		run.Status = overview.GetStatus()
	}
	if run.Summary == nil {
		run.Summary = &models.Run_Summary{}
	}
	if overview.GetStartedAt() != nil {
		run.Summary.StartedAt = overview.GetStartedAt()
	}
	if overview.GetFinishedAt() != nil {
		run.Summary.FinishedAt = overview.GetFinishedAt()
	}
	if overview.GetDuration() != nil {
		run.Summary.Duration = overview.GetDuration()
	}
	if overview.GetProgressPct() > run.Summary.GetProgressPct() {
		run.Summary.ProgressPct = overview.GetProgressPct()
	}
}

func projectOverview(runID string, rs *workflowpb.RunState, observedAt *timestamppb.Timestamp) *monitor.Overview {
	return projectOverviewWithSource(runID, rs, observedAt, monitor.ObservationSource_OBSERVATION_SOURCE_TEMPORAL_RUN_STATE)
}

// projectOverviewWithSource maps a RunState onto a full Overview snapshot. The
// RunState already contains the runtime tree facts; this function only copies
// stage fields into monitor nodes and links children by parent_node_execution_id.
func projectOverviewWithSource(runID string, rs *workflowpb.RunState, observedAt *timestamppb.Timestamp, source monitor.ObservationSource) *monitor.Overview {
	if rs == nil {
		return pendingOverview(runID, observedAt)
	}
	now := time.Now()
	terminal := isTerminalStatus(rs.GetStatus())

	type stageEntry struct {
		index int
		node  *monitor.PipelineNode
	}
	entries := make([]stageEntry, 0, len(rs.GetStages()))
	nodesByID := make(map[string]*monitor.PipelineNode, len(rs.GetStages()))
	timeline := make([]*monitor.Event, 0, len(rs.GetStages())+1)

	var (
		earliestStart *timestamppb.Timestamp
		latestFinish  *timestamppb.Timestamp
		completed     int
	)

	for idx, st := range rs.GetStages() {
		started := st.GetStartedAt()
		finished := st.GetFinishedAt()
		order := st.GetOrder()
		if order == 0 {
			order = uint32(idx + 1)
		}
		phase := st.GetPhase()
		if phase == "" && st.GetParentNodeExecutionId() == "" {
			phase = st.GetName()
		}
		statusReason := st.GetStatusReason()
		if statusReason == "" {
			statusReason = sourceStatusReason(source, st.GetStatus())
		}
		worker := st.GetWorker()
		if worker == nil && st.GetMachineId() != "" {
			worker = &domainpb.Worker{
				Id:   "agent/" + st.GetMachineId(),
				Kind: domainpb.Worker_KIND_AGENT,
			}
		}

		node := &monitor.PipelineNode{
			NodeExecutionId: st.GetNodeExecutionId(),
			Name:            st.GetName(),
			Status:          pendingIfUnspecified(st.GetStatus()),
			StartedAt:       started,
			FinishedAt:      finished,
			Duration:        spanDuration(started, finished, now),
			Worker:          worker,
			LogRef: &monitor.LogRef{
				RunId:           runID,
				NodeExecutionId: refString(st.GetNodeExecutionId()),
				ComponentId:     refString(st.GetComponentId()),
			},
			Attempt:               st.GetAttempt(),
			Source:                source,
			Order:                 order,
			ParentNodeExecutionId: st.GetParentNodeExecutionId(),
			Phase:                 phase,
			ComponentId:           st.GetComponentId(),
			MachineId:             st.GetMachineId(),
			StatusReason:          statusReason,
			ErrorMessage:          st.GetErrorMessage(),
			Operation:             st.GetOperation(),
			Outputs:               st.GetOutputs(),
		}
		entries = append(entries, stageEntry{index: idx, node: node})
		if node.GetNodeExecutionId() != "" {
			nodesByID[node.GetNodeExecutionId()] = node
		}

		if started != nil && (earliestStart == nil || started.AsTime().Before(earliestStart.AsTime())) {
			earliestStart = started
		}
		if finished != nil && (latestFinish == nil || finished.AsTime().After(latestFinish.AsTime())) {
			latestFinish = finished
		}
		if isPipelineTerminalStatus(st.GetStatus()) {
			completed++
		}

		if started != nil {
			timeline = append(timeline, &monitor.Event{
				At:              started,
				Kind:            monitor.Event_KIND_STAGE_STARTED,
				Message:         "stage started: " + st.GetName(),
				NodeExecutionId: st.GetNodeExecutionId(),
				Status:          common.Status_STATUS_RUNNING,
				Source:          source,
				Severity:        monitor.EventSeverity_EVENT_SEVERITY_INFO,
			})
		}
		if finished != nil {
			timeline = append(timeline, &monitor.Event{
				At:              finished,
				Kind:            stageEndKind(st.GetStatus()),
				Message:         "stage finished: " + st.GetName(),
				NodeExecutionId: st.GetNodeExecutionId(),
				Status:          st.GetStatus(),
				Source:          source,
				Severity:        eventSeverity(st.GetStatus()),
			})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		left := entries[i].node
		right := entries[j].node
		if left.GetParentNodeExecutionId() != right.GetParentNodeExecutionId() {
			return left.GetParentNodeExecutionId() < right.GetParentNodeExecutionId()
		}
		if left.GetOrder() != right.GetOrder() {
			return left.GetOrder() < right.GetOrder()
		}
		return entries[i].index < entries[j].index
	})
	roots := make([]*monitor.PipelineNode, 0, len(entries))
	for _, entry := range entries {
		node := entry.node
		if parent := nodesByID[node.GetParentNodeExecutionId()]; parent != nil {
			parent.Children = append(parent.Children, node)
			continue
		}
		roots = append(roots, node)
	}

	runAt := earliestStart
	if runAt == nil {
		runAt = timestamppb.New(now)
	}
	timeline = append(timeline, &monitor.Event{
		At:       runAt,
		Kind:     monitor.Event_KIND_RUN_STATUS,
		Message:  "run status: " + rs.GetStatus().String(),
		Status:   rs.GetStatus(),
		Source:   source,
		Severity: eventSeverity(rs.GetStatus()),
	})
	sort.SliceStable(timeline, func(i, j int) bool {
		left := timeline[i].GetAt()
		right := timeline[j].GetAt()
		if left == nil {
			return false
		}
		if right == nil {
			return true
		}
		return left.AsTime().Before(right.AsTime())
	})
	for i, event := range timeline {
		event.Sequence = uint64(i + 1)
	}

	var finishedAt *timestamppb.Timestamp
	if terminal {
		finishedAt = latestFinish
	}

	return &monitor.Overview{
		RunId:       runID,
		Status:      rs.GetStatus(),
		StartedAt:   earliestStart,
		FinishedAt:  finishedAt,
		Duration:    spanDuration(earliestStart, finishedAt, now),
		ProgressPct: progressPct(completed, len(rs.GetStages())),
		Pipeline:    &monitor.PipelineView{Roots: roots},
		Workers:     []*monitor.WorkerInfo{},
		Timeline:    timeline,
		ObservedAt:  observedAt,
		Source:      source,
	}
}

// spanDuration computes a span's elapsed time: finished-started when closed, else
// now-started while running. Returns nil with no start.
func spanDuration(start, finish *timestamppb.Timestamp, now time.Time) *durationpb.Duration {
	if start == nil {
		return nil
	}
	end := now
	if finish != nil {
		end = finish.AsTime()
	}
	d := end.Sub(start.AsTime())
	if d < 0 {
		d = 0
	}
	return durationpb.New(d)
}

// progressPct is round(completed/total*100), clamped to [0,100], 0 with no stages.
func progressPct(completed, total int) uint32 {
	if total <= 0 {
		return 0
	}
	pct := math.Round(float64(completed) / float64(total) * 100)
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return uint32(pct)
}

// stageEndKind maps a stage's terminal status to its completion event kind.
func stageEndKind(s common.Status) monitor.Event_Kind {
	switch s {
	case common.Status_STATUS_COMPLETED:
		return monitor.Event_KIND_STAGE_COMPLETED
	case common.Status_STATUS_FAILED:
		return monitor.Event_KIND_STAGE_FAILED
	case common.Status_STATUS_RETRY_WAIT:
		return monitor.Event_KIND_STAGE_RETRYING
	default:
		return monitor.Event_KIND_STAGE_COMPLETED
	}
}

func eventSeverity(s common.Status) monitor.EventSeverity {
	switch s {
	case common.Status_STATUS_FAILED, common.Status_STATUS_CANCELLED:
		return monitor.EventSeverity_EVENT_SEVERITY_ERROR
	case common.Status_STATUS_RETRY_WAIT, common.Status_STATUS_CANCELLING:
		return monitor.EventSeverity_EVENT_SEVERITY_WARNING
	default:
		return monitor.EventSeverity_EVENT_SEVERITY_INFO
	}
}

func sourceStatusReason(source monitor.ObservationSource, status common.Status) string {
	status = pendingIfUnspecified(status)
	switch source {
	case monitor.ObservationSource_OBSERVATION_SOURCE_PERSISTED_RECORD:
		return "persisted_run_state_" + strings.ToLower(status.String())
	case monitor.ObservationSource_OBSERVATION_SOURCE_TEMPORAL_RUN_STATE:
		return "temporal_run_state_" + strings.ToLower(status.String())
	default:
		return strings.ToLower(status.String())
	}
}

func isPipelineTerminalStatus(s common.Status) bool {
	switch s {
	case common.Status_STATUS_DEPLOYED:
		return true
	default:
		return isTerminalStatus(s)
	}
}

// isTerminalStatus reports whether a run/stage status is final.
func isTerminalStatus(s common.Status) bool {
	switch s {
	case common.Status_STATUS_COMPLETED,
		common.Status_STATUS_FAILED,
		common.Status_STATUS_CANCELLED,
		common.Status_STATUS_SKIPPED:
		return true
	default:
		return false
	}
}

// refString returns a pointer to s, or nil for the empty string (leaving an unset
// node id absent on the LogRef oneof).
func refString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
