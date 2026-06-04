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

// SnapshotRunReader loads the persisted run record + parent suite run that the
// snapshot bundles alongside the live workflow projection. It is a read-only,
// tenant-agnostic lookup by id (the service has already authorised the caller).
// Implemented by the storage layer; injected so the overview reader stays an
// execution adapter without owning storage.
type SnapshotRunReader interface {
	// RunRecord returns the persisted TestRunRecord for the run id.
	RunRecord(ctx context.Context, runID string) (*models.TestRunRecord, error)
	// SuiteRun returns the parent SuiteRunRecord when the run belongs to a suite,
	// or (nil, nil) when it is a standalone run.
	SuiteRun(ctx context.Context, suiteRunID string) (*models.SuiteRunRecord, error)
}

// AgentPresenceReader returns the registry-backed liveness samples for run
// workers. Missing entries mean the registry has not seen that machine for the
// requested run.
type AgentPresenceReader interface {
	AgentPresence(ctx context.Context, runID string, machineIDs []string) (map[string]*monitor.WorkerInfo, error)
}

// OverviewReader implements test_run_overview.OverviewReader. It assembles the
// snapshot from (a) the persisted run record and its staged topology, (b) the
// live monitor.Overview projected from the Temporal TestWorkflow's RunState, and
// (c) the parent suite-run record when the run belongs to a suite.
type OverviewReader struct {
	tc       runStateQuerier
	store    SnapshotRunReader
	presence AgentPresenceReader
}

var _ test_run_overview.OverviewReader = (*OverviewReader)(nil)

// NewOverviewReader builds the test_run_overview.OverviewReader adapter over a
// Temporal client (the live RunState source) and a SnapshotRunReader (the
// persisted run/suite-run + topology source). store may be nil, in which case
// the snapshot carries only the live Overview projection.
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
	var rec *models.TestRunRecord
	observedAt := timestamppb.Now()
	degradedReasons := make([]string, 0, 2)

	if r.store != nil {
		var err error
		rec, err = r.store.RunRecord(ctx, runID)
		if err != nil {
			return nil, err
		}
		snap.Run = rec
		snap.Topology = topologyFromRecord(rec)
		if sid := rec.GetSuiteRunId(); sid != "" {
			suiteRun, err := r.store.SuiteRun(ctx, sid)
			if err != nil {
				return nil, err
			}
			snap.SuiteRun = suiteRun
		}
	}

	presence, err := r.agentPresence(ctx, runID, rec)
	if err != nil {
		degradedReasons = append(degradedReasons, "agent_registry_unavailable: "+err.Error())
	}

	rs, err := r.tc.GetRunState(ctx, testWorkflowID(runID), "")
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		// The workflow may not be queryable yet or may already be closed. The
		// persisted record is the durable fallback; only a never-started record
		// degrades to PENDING.
		degradedReasons = append(degradedReasons, "workflow_state_unavailable: "+err.Error())
		snap.Overview = overviewFromRecord(runID, rec, presence, observedAt)
		snap.Overview.DegradedReasons = append(snap.Overview.GetDegradedReasons(), degradedReasons...)
		return snap, nil
	}
	snap.Overview = mergeOverviewWithRecord(projectOverview(runID, rs, observedAt), rec, presence)
	if rec != nil {
		snap.Topology = topologyFromRecordWithRunState(rec, rs)
	}
	snap.Overview.DegradedReasons = append(snap.Overview.GetDegradedReasons(), degradedReasons...)
	overlayRunFromOverview(snap.Run, snap.Overview)
	return snap, nil
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

// topologyFromRecord assembles the staged topology envelope from a run record:
// the baked spec's topology spec + infrastructure plan, plus the live
// infrastructure state and deployment plan filled on the record as the run
// progresses.
func topologyFromRecord(rec *models.TestRunRecord) *topology.Topology {
	return topologyFromRecordWithRunState(rec, rec.GetRuntimeState())
}

func topologyFromRecordWithRunState(rec *models.TestRunRecord, rs *workflowpb.RunState) *topology.Topology {
	if rec == nil {
		return nil
	}
	spec := rec.GetSpec()
	t := &topology.Topology{
		Spec:                spec.GetTopologySpec(),
		InfrastructurePlan:  spec.GetInfrastructurePlan(),
		InfrastructureState: rec.GetInfrastructureState(),
		DeploymentPlan:      rec.GetDeploymentPlan(),
		Tags:                spec.GetTags(),
	}
	t.State = topologyState(rec)
	t.RuntimeNodes, t.RuntimeConnections = runtimeTopologyFromRecord(rec, rs)
	return t
}

// topologyState classifies how far the topology has materialized from the
// record's filled artifacts.
func topologyState(rec *models.TestRunRecord) topology.Topology_State {
	switch {
	case rec.GetDeploymentPlan() != nil:
		return topology.Topology_STATE_DEPLOYED
	case rec.GetInfrastructureState() != nil:
		return topology.Topology_STATE_INFRASTRUCTURE_DEPLOYED
	case rec.GetSpec().GetInfrastructurePlan() != nil:
		return topology.Topology_STATE_INFRASTRUCTURE_PLANNED
	case rec.GetSpec().GetTopologySpec() != nil:
		return topology.Topology_STATE_SPEC
	default:
		return topology.Topology_STATE_UNSPECIFIED
	}
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

func overviewFromRecord(runID string, rec *models.TestRunRecord, presence map[string]*monitor.WorkerInfo, observedAt *timestamppb.Timestamp) *monitor.Overview {
	if rec == nil {
		return pendingOverview(runID, observedAt)
	}
	if rec.GetRuntimeState() != nil {
		overview := projectOverviewWithSource(runID, rec.GetRuntimeState(), observedAt, monitor.ObservationSource_OBSERVATION_SOURCE_PERSISTED_RECORD)
		overview.Workers = workersFromRecord(rec, presence)
		sum := rec.GetSummary()
		if rec.GetStatus() != common.Status_STATUS_UNSPECIFIED {
			overview.Status = rec.GetStatus()
		}
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
		return overview
	}
	status := rec.GetStatus()
	if status == common.Status_STATUS_UNSPECIFIED {
		status = common.Status_STATUS_PENDING
	}
	sum := rec.GetSummary()
	return &monitor.Overview{
		RunId:       runID,
		Status:      status,
		StartedAt:   sum.GetStartedAt(),
		FinishedAt:  sum.GetFinishedAt(),
		Duration:    sum.GetDuration(),
		ProgressPct: sum.GetProgressPct(),
		Pipeline:    pipelineFromRecord(runID, rec),
		Workers:     workersFromRecord(rec, presence),
		Timeline:    []*monitor.Event{},
		ObservedAt:  observedAt,
		Source:      monitor.ObservationSource_OBSERVATION_SOURCE_PERSISTED_RECORD,
	}
}

func mergeOverviewWithRecord(overview *monitor.Overview, rec *models.TestRunRecord, presence map[string]*monitor.WorkerInfo) *monitor.Overview {
	if overview == nil {
		return overviewFromRecord("", rec, presence, timestamppb.Now())
	}
	if rec == nil {
		return overview
	}
	overview.Workers = workersFromRecord(rec, presence)
	stored := rec.GetStatus()
	if stored == common.Status_STATUS_CANCELLING || isTerminalStatus(stored) {
		sum := rec.GetSummary()
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

func pipelineFromRecord(runID string, rec *models.TestRunRecord) *monitor.PipelineView {
	pipeline := &monitor.PipelineView{}
	if rec == nil {
		return pipeline
	}
	pipeline.Roots = []*monitor.PipelineNode{
		recordStageNode(runID, rec, stageInfrastructureNodeName, 1, recordInfrastructureStatus(rec)),
		recordStageNode(runID, rec, stageRenderDeploymentPlanNodeName, 2, recordRenderPlanStatus(rec)),
		recordStageNode(runID, rec, executeDeploymentPlanNodeName, 3, recordExecutePlanStatus(rec)),
		recordStageNode(runID, rec, stageWorkloadNodeName, 4, recordWorkloadStatus(rec)),
		recordStageNode(runID, rec, stageTeardownNodeName, 5, recordTeardownStatus(rec)),
	}
	if rec.GetDeploymentPlan() != nil {
		pipeline.Roots[1].Outputs = deploymentbuilder.DeploymentPlanOutputs(rec.GetDeploymentPlan())
		deploymentPlanRoot(runID, rec.GetDeploymentPlan(), pipeline.Roots[2])
	}
	return pipeline
}

func (r *OverviewReader) agentPresence(ctx context.Context, runID string, rec *models.TestRunRecord) (map[string]*monitor.WorkerInfo, error) {
	if r == nil || r.presence == nil || rec == nil {
		return nil, nil
	}
	machineIDs := workerMachineIDs(rec)
	if len(machineIDs) == 0 {
		return nil, nil
	}
	return r.presence.AgentPresence(ctx, runID, machineIDs)
}

func workerMachineIDs(rec *models.TestRunRecord) []string {
	if rec == nil {
		return nil
	}
	nodeIDs := make(map[string]struct{})
	for _, machine := range rec.GetInfrastructureState().GetMachines() {
		if machine == nil || machine.GetNodeId() == "" {
			continue
		}
		nodeIDs[machine.GetNodeId()] = struct{}{}
	}
	for _, component := range rec.GetDeploymentPlan().GetComponents() {
		if component.GetNodeId() != "" {
			nodeIDs[component.GetNodeId()] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(nodeIDs))
	for nodeID := range nodeIDs {
		ordered = append(ordered, nodeID)
	}
	sort.Strings(ordered)
	return ordered
}

func workersFromRecord(rec *models.TestRunRecord, presence map[string]*monitor.WorkerInfo) []*monitor.WorkerInfo {
	if rec == nil {
		return nil
	}

	machines := make(map[string]*deploymentpb.MachineState)
	for _, machine := range rec.GetInfrastructureState().GetMachines() {
		if machine == nil || machine.GetNodeId() == "" {
			continue
		}
		machines[machine.GetNodeId()] = machine
	}

	current := currentExecutionByNode(rec.GetDeploymentPlan())
	ordered := workerMachineIDs(rec)

	workers := make([]*monitor.WorkerInfo, 0, len(ordered))
	for _, nodeID := range ordered {
		machine := machines[nodeID]
		status := workerStatus(machine, current[nodeID])
		worker := &monitor.WorkerInfo{
			Id:                     "agent/" + nodeID,
			Kind:                   domainpb.Worker_KIND_AGENT,
			MachineId:              nodeID,
			Host:                   machineHost(machine),
			CurrentNodeExecutionId: current[nodeID],
			Status:                 status,
			Presence:               recordWorkerPresence(rec, machine),
			Source:                 monitor.ObservationSource_OBSERVATION_SOURCE_PERSISTED_RECORD,
			StatusReason:           recordWorkerStatusReason(rec, machine),
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
	worker.Online = sample.GetPresence() == monitor.WorkerPresence_WORKER_PRESENCE_ONLINE
	worker.Presence = sample.GetPresence()
	worker.RegisteredAt = sample.GetRegisteredAt()
	worker.LastSeenAt = sample.GetLastSeenAt()
	worker.HeartbeatIntervalSeconds = sample.GetHeartbeatIntervalSeconds()
	worker.AgentVersion = sample.GetAgentVersion()
	worker.RunId = sample.GetRunId()
	worker.StatusReason = sample.GetStatusReason()
	worker.Source = monitor.ObservationSource_OBSERVATION_SOURCE_AGENT_REGISTRY
}

func recordWorkerPresence(rec *models.TestRunRecord, machine *deploymentpb.MachineState) monitor.WorkerPresence {
	if rec != nil && isTerminalStatus(rec.GetStatus()) {
		return monitor.WorkerPresence_WORKER_PRESENCE_TERMINATED
	}
	if workerOnline(machine) {
		return monitor.WorkerPresence_WORKER_PRESENCE_UNKNOWN
	}
	return monitor.WorkerPresence_WORKER_PRESENCE_UNKNOWN
}

func recordWorkerStatusReason(rec *models.TestRunRecord, machine *deploymentpb.MachineState) string {
	if rec != nil && isTerminalStatus(rec.GetStatus()) {
		return "run_terminal_no_registry_sample"
	}
	if machine == nil {
		return "machine_not_materialized"
	}
	return "no_registry_sample"
}

func currentExecutionByNode(plan *deploymentpb.DeploymentPlan) map[string]string {
	current := make(map[string]string)
	if plan == nil {
		return current
	}
	for _, component := range plan.GetComponents() {
		if component == nil || component.GetNodeId() == "" {
			continue
		}
		for _, step := range component.GetSteps() {
			if step == nil || !activeStatus(step.GetStatus()) {
				continue
			}
			current[component.GetNodeId()] = labelOr(step.GetLabels(), deploymentbuilder.LabelNodeExecutionID, deploymentbuilder.StepExecutionID(component.GetComponentId(), step.GetId()))
			break
		}
		if current[component.GetNodeId()] == "" && activeStatus(component.GetStatus()) {
			current[component.GetNodeId()] = labelOr(component.GetLabels(), deploymentbuilder.LabelNodeExecutionID, deploymentbuilder.ComponentExecutionID(component.GetComponentId()))
		}
	}
	return current
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

func workerOnline(machine *deploymentpb.MachineState) bool {
	if machine == nil {
		return false
	}
	switch pendingIfUnspecified(machine.GetStatus()) {
	case common.Status_STATUS_DEPLOYED,
		common.Status_STATUS_COMPLETED,
		common.Status_STATUS_RUNNING,
		common.Status_STATUS_DEPLOYMENT:
		return true
	default:
		return false
	}
}

func activeStatus(status common.Status) bool {
	switch pendingIfUnspecified(status) {
	case common.Status_STATUS_ALLOCATED,
		common.Status_STATUS_DEPLOYMENT,
		common.Status_STATUS_RUNNING,
		common.Status_STATUS_RETRY_WAIT,
		common.Status_STATUS_CANCELLING:
		return true
	default:
		return false
	}
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

func recordStageNode(runID string, rec *models.TestRunRecord, name string, order uint32, status common.Status) *monitor.PipelineNode {
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
		StatusReason:    recordStageStatusReason(rec, name, status),
	}
}

func recordInfrastructureStatus(rec *models.TestRunRecord) common.Status {
	state := rec.GetInfrastructureState()
	if state == nil || len(state.GetMachines()) == 0 {
		if isTerminalStatus(rec.GetStatus()) && rec.GetStatus() != common.Status_STATUS_COMPLETED {
			return rec.GetStatus()
		}
		return common.Status_STATUS_PENDING
	}
	var completed, total int
	for _, machine := range state.GetMachines() {
		if machine == nil {
			continue
		}
		total++
		switch pendingIfUnspecified(machine.GetStatus()) {
		case common.Status_STATUS_FAILED, common.Status_STATUS_CANCELLED:
			return machine.GetStatus()
		case common.Status_STATUS_ALLOCATED, common.Status_STATUS_DEPLOYMENT, common.Status_STATUS_RUNNING:
			return common.Status_STATUS_RUNNING
		case common.Status_STATUS_DEPLOYED, common.Status_STATUS_COMPLETED:
			completed++
		}
	}
	if total > 0 && completed == total {
		return common.Status_STATUS_COMPLETED
	}
	return common.Status_STATUS_PENDING
}

func recordRenderPlanStatus(rec *models.TestRunRecord) common.Status {
	if rec.GetDeploymentPlan() != nil {
		return common.Status_STATUS_COMPLETED
	}
	if isTerminalStatus(rec.GetStatus()) && recordInfrastructureStatus(rec) == common.Status_STATUS_COMPLETED && rec.GetStatus() != common.Status_STATUS_COMPLETED {
		return rec.GetStatus()
	}
	if recordInfrastructureStatus(rec) == common.Status_STATUS_COMPLETED && rec.GetStatus() == common.Status_STATUS_RUNNING {
		return common.Status_STATUS_RUNNING
	}
	return common.Status_STATUS_PENDING
}

func recordExecutePlanStatus(rec *models.TestRunRecord) common.Status {
	status := deploymentPlanStatus(rec.GetDeploymentPlan())
	if status != common.Status_STATUS_PENDING {
		return status
	}
	if rec.GetStatus() == common.Status_STATUS_COMPLETED && rec.GetDeploymentPlan() != nil {
		return common.Status_STATUS_COMPLETED
	}
	if isTerminalStatus(rec.GetStatus()) && rec.GetDeploymentPlan() == nil && recordRenderPlanStatus(rec) == common.Status_STATUS_COMPLETED {
		return rec.GetStatus()
	}
	return status
}

func recordWorkloadStatus(rec *models.TestRunRecord) common.Status {
	switch rec.GetStatus() {
	case common.Status_STATUS_COMPLETED:
		return common.Status_STATUS_COMPLETED
	case common.Status_STATUS_FAILED, common.Status_STATUS_CANCELLED:
		if recordExecutePlanStatus(rec) == common.Status_STATUS_COMPLETED {
			return rec.GetStatus()
		}
		return common.Status_STATUS_PENDING
	case common.Status_STATUS_RUNNING, common.Status_STATUS_CANCELLING:
		if recordExecutePlanStatus(rec) == common.Status_STATUS_COMPLETED {
			return rec.GetStatus()
		}
	}
	return common.Status_STATUS_PENDING
}

func recordTeardownStatus(rec *models.TestRunRecord) common.Status {
	switch rec.GetStatus() {
	case common.Status_STATUS_COMPLETED:
		return common.Status_STATUS_COMPLETED
	case common.Status_STATUS_FAILED:
		if recordWorkloadStatus(rec) == common.Status_STATUS_COMPLETED {
			return common.Status_STATUS_FAILED
		}
	case common.Status_STATUS_CANCELLED:
		if recordInfrastructureStatus(rec) == common.Status_STATUS_COMPLETED {
			return common.Status_STATUS_CANCELLED
		}
	case common.Status_STATUS_RUNNING, common.Status_STATUS_CANCELLING:
		if recordWorkloadStatus(rec) == common.Status_STATUS_COMPLETED {
			return common.Status_STATUS_RUNNING
		}
	}
	return common.Status_STATUS_PENDING
}

func recordStageStatusReason(rec *models.TestRunRecord, name string, status common.Status) string {
	if rec != nil && isTerminalStatus(rec.GetStatus()) {
		return "persisted_record_terminal_" + strings.ToLower(rec.GetStatus().String())
	}
	if status == common.Status_STATUS_PENDING {
		return "persisted_record_missing_live_stage"
	}
	return "persisted_record_" + strings.ToLower(status.String())
}

func deploymentStatusReason(status common.Status) string {
	status = pendingIfUnspecified(status)
	if status == common.Status_STATUS_PENDING {
		return "deployment_plan_pending_or_not_started"
	}
	return "deployment_plan_" + strings.ToLower(status.String())
}

func deploymentPlanRoot(runID string, plan *deploymentpb.DeploymentPlan, root *monitor.PipelineNode) *monitor.PipelineNode {
	if root == nil {
		id := deploymentbuilder.StageExecutionID(executeDeploymentPlanNodeName)
		root = &monitor.PipelineNode{
			NodeExecutionId: id,
			Name:            executeDeploymentPlanNodeName,
			Status:          deploymentPlanStatus(plan),
			LogRef:          logRef(runID, id, ""),
			Attempt:         1,
			Source:          monitor.ObservationSource_OBSERVATION_SOURCE_DEPLOYMENT_PLAN,
			Order:           3,
			Phase:           executeDeploymentPlanNodeName,
		}
	}
	if root.GetNodeExecutionId() == "" {
		root.NodeExecutionId = deploymentbuilder.StageExecutionID(executeDeploymentPlanNodeName)
	}
	if root.GetSource() == monitor.ObservationSource_OBSERVATION_SOURCE_UNSPECIFIED ||
		root.GetSource() == monitor.ObservationSource_OBSERVATION_SOURCE_PERSISTED_RECORD {
		root.Source = monitor.ObservationSource_OBSERVATION_SOURCE_DEPLOYMENT_PLAN
	}
	root.Phase = executeDeploymentPlanNodeName
	if root.GetOrder() == 0 {
		root.Order = 3
	}
	if root.LogRef == nil {
		root.LogRef = logRef(runID, root.GetNodeExecutionId(), "")
	}
	root.Children = deploymentPlanChildren(runID, plan, root.GetStatus())
	return root
}

func deploymentPlanChildren(runID string, plan *deploymentpb.DeploymentPlan, parentStatus common.Status) []*monitor.PipelineNode {
	components := append([]*deploymentpb.ComponentDeployment(nil), plan.GetComponents()...)
	sort.SliceStable(components, func(i, j int) bool {
		left := components[i]
		right := components[j]
		if left.GetGlobalPriority() != right.GetGlobalPriority() {
			return left.GetGlobalPriority() < right.GetGlobalPriority()
		}
		if left.GetNodeId() != right.GetNodeId() {
			return left.GetNodeId() < right.GetNodeId()
		}
		if left.GetNodePriority() != right.GetNodePriority() {
			return left.GetNodePriority() < right.GetNodePriority()
		}
		return left.GetComponentId() < right.GetComponentId()
	})
	nodes := make([]*monitor.PipelineNode, 0, len(components))
	parentID := deploymentbuilder.StageExecutionID(executeDeploymentPlanNodeName)
	for idx, component := range components {
		if component == nil {
			continue
		}
		componentID := component.GetComponentId()
		nodeExecutionID := labelOr(component.GetLabels(), deploymentbuilder.LabelNodeExecutionID, deploymentbuilder.ComponentExecutionID(componentID))
		componentStatus := inheritedDeploymentStatus(component.GetStatus(), parentStatus)
		nodes = append(nodes, &monitor.PipelineNode{
			NodeExecutionId: nodeExecutionID,
			Name:            componentID,
			Status:          componentStatus,
			Worker: &domainpb.Worker{
				Id:   "agent/" + component.GetNodeId(),
				Kind: domainpb.Worker_KIND_AGENT,
			},
			LogRef:                logRef(runID, nodeExecutionID, componentID),
			Children:              agentStepNodes(runID, component, nodeExecutionID, componentStatus),
			Attempt:               1,
			Source:                monitor.ObservationSource_OBSERVATION_SOURCE_DEPLOYMENT_PLAN,
			Order:                 uint32(idx + 1),
			ParentNodeExecutionId: parentID,
			Phase:                 executeDeploymentPlanNodeName,
			ComponentId:           componentID,
			MachineId:             component.GetNodeId(),
			StatusReason:          deploymentStatusReason(componentStatus),
		})
	}
	return nodes
}

func agentStepNodes(runID string, component *deploymentpb.ComponentDeployment, parentNodeExecutionID string, parentStatus common.Status) []*monitor.PipelineNode {
	steps := append([]*deploymentpb.AgentStep(nil), component.GetSteps()...)
	sort.SliceStable(steps, func(i, j int) bool {
		if steps[i].GetOrder() != steps[j].GetOrder() {
			return steps[i].GetOrder() < steps[j].GetOrder()
		}
		return steps[i].GetId() < steps[j].GetId()
	})
	nodes := make([]*monitor.PipelineNode, 0, len(steps))
	for idx, step := range steps {
		if step == nil {
			continue
		}
		componentID := component.GetComponentId()
		nodeExecutionID := labelOr(step.GetLabels(), deploymentbuilder.LabelNodeExecutionID, deploymentbuilder.StepExecutionID(componentID, step.GetId()))
		operation := deploymentbuilder.AgentStepOperation(step)
		name := deploymentbuilder.AgentStepStageName(step)
		stepStatus := inheritedDeploymentStatus(step.GetStatus(), parentStatus)
		nodes = append(nodes, &monitor.PipelineNode{
			NodeExecutionId: nodeExecutionID,
			Name:            name,
			Status:          stepStatus,
			Worker: &domainpb.Worker{
				Id:   "agent/" + component.GetNodeId(),
				Kind: domainpb.Worker_KIND_AGENT,
			},
			LogRef:                logRef(runID, nodeExecutionID, componentID),
			Attempt:               1,
			Source:                monitor.ObservationSource_OBSERVATION_SOURCE_DEPLOYMENT_PLAN,
			Order:                 uint32(idx + 1),
			ParentNodeExecutionId: parentNodeExecutionID,
			Phase:                 executeDeploymentPlanNodeName,
			ComponentId:           componentID,
			MachineId:             component.GetNodeId(),
			StatusReason:          deploymentStatusReason(stepStatus),
			Operation:             operation,
			Outputs:               deploymentbuilder.AgentStepResultOutputs(component, step),
		})
	}
	return nodes
}

func inheritedDeploymentStatus(status, parentStatus common.Status) common.Status {
	resolved := pendingIfUnspecified(status)
	if resolved != common.Status_STATUS_PENDING {
		return resolved
	}
	switch parentStatus {
	case common.Status_STATUS_COMPLETED, common.Status_STATUS_DEPLOYED:
		return common.Status_STATUS_COMPLETED
	default:
		return resolved
	}
}

func deploymentPlanStatus(plan *deploymentpb.DeploymentPlan) common.Status {
	if plan == nil || len(plan.GetComponents()) == 0 {
		return common.Status_STATUS_PENDING
	}
	var completed int
	for _, component := range plan.GetComponents() {
		switch pendingIfUnspecified(component.GetStatus()) {
		case common.Status_STATUS_FAILED, common.Status_STATUS_CANCELLED, common.Status_STATUS_SKIPPED:
			return component.GetStatus()
		case common.Status_STATUS_DEPLOYMENT, common.Status_STATUS_RUNNING, common.Status_STATUS_ALLOCATED:
			return common.Status_STATUS_RUNNING
		case common.Status_STATUS_DEPLOYED, common.Status_STATUS_COMPLETED:
			completed++
		}
	}
	if completed == len(plan.GetComponents()) {
		return common.Status_STATUS_COMPLETED
	}
	return common.Status_STATUS_PENDING
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

func overlayRunFromOverview(rec *models.TestRunRecord, overview *monitor.Overview) {
	if rec == nil || overview == nil {
		return
	}
	if rec.GetStatus() != common.Status_STATUS_CANCELLING && !isTerminalStatus(rec.GetStatus()) {
		rec.Status = overview.GetStatus()
	}
	if rec.Summary == nil {
		rec.Summary = &models.TestRunRecord_Summary{}
	}
	if overview.GetStartedAt() != nil {
		rec.Summary.StartedAt = overview.GetStartedAt()
	}
	if overview.GetFinishedAt() != nil {
		rec.Summary.FinishedAt = overview.GetFinishedAt()
	}
	if overview.GetDuration() != nil {
		rec.Summary.Duration = overview.GetDuration()
	}
	if overview.GetProgressPct() > rec.Summary.GetProgressPct() {
		rec.Summary.ProgressPct = overview.GetProgressPct()
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
