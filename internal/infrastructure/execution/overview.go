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

const executeDeploymentPlanNodeName = "execute_deployment_plan"

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

// OverviewReader implements test_run_overview.OverviewReader. It assembles the
// snapshot from (a) the persisted run record and its staged topology, (b) the
// live monitor.Overview projected from the Temporal TestWorkflow's RunState, and
// (c) the parent suite-run record when the run belongs to a suite.
type OverviewReader struct {
	tc    runStateQuerier
	store SnapshotRunReader
}

var _ test_run_overview.OverviewReader = (*OverviewReader)(nil)

// NewOverviewReader builds the test_run_overview.OverviewReader adapter over a
// Temporal client (the live RunState source) and a SnapshotRunReader (the
// persisted run/suite-run + topology source). store may be nil, in which case
// the snapshot carries only the live Overview projection.
func NewOverviewReader(c client.Client, store SnapshotRunReader) *OverviewReader {
	return &OverviewReader{tc: workflowpb.NewTestServiceClient(c), store: store}
}

// Get returns a one-shot snapshot for the run. The live Overview degrades to
// PENDING when the workflow has not started yet (the UI polls before it exists);
// only context cancellation is surfaced as an error.
func (r *OverviewReader) Get(ctx context.Context, runID string) (*api.TestRunOverviewSnapshot, error) {
	snap := &api.TestRunOverviewSnapshot{}
	var rec *models.TestRunRecord

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

	rs, err := r.tc.GetRunState(ctx, testWorkflowID(runID), "")
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		// The workflow may not be queryable yet or may already be closed. The
		// persisted record is the durable fallback; only a never-started record
		// degrades to PENDING.
		snap.Overview = overviewFromRecord(runID, rec)
		return snap, nil
	}
	snap.Overview = mergeOverviewWithRecord(projectOverview(runID, rs), rec)
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
func pendingOverview(runID string) *monitor.Overview {
	return &monitor.Overview{
		RunId:    runID,
		Status:   common.Status_STATUS_PENDING,
		Pipeline: &monitor.PipelineView{},
		Workers:  []*monitor.WorkerInfo{},
		Timeline: []*monitor.Event{},
	}
}

func overviewFromRecord(runID string, rec *models.TestRunRecord) *monitor.Overview {
	if rec == nil {
		return pendingOverview(runID)
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
		Workers:     workersFromRecord(rec),
		Timeline:    []*monitor.Event{},
	}
}

func mergeOverviewWithRecord(overview *monitor.Overview, rec *models.TestRunRecord) *monitor.Overview {
	if overview == nil {
		return overviewFromRecord("", rec)
	}
	if rec == nil {
		return overview
	}
	overview = enrichDeploymentPipeline(overview, rec)
	overview.Workers = workersFromRecord(rec)
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
	if rec == nil || rec.GetDeploymentPlan() == nil {
		return pipeline
	}
	root := deploymentPlanRoot(runID, rec.GetDeploymentPlan(), nil)
	pipeline.Roots = []*monitor.PipelineNode{root}
	return pipeline
}

func enrichDeploymentPipeline(overview *monitor.Overview, rec *models.TestRunRecord) *monitor.Overview {
	if overview == nil || rec == nil || rec.GetDeploymentPlan() == nil {
		return overview
	}
	if overview.Pipeline == nil {
		overview.Pipeline = &monitor.PipelineView{}
	}
	root := findPipelineNode(overview.Pipeline.GetRoots(), executeDeploymentPlanNodeName)
	if root == nil {
		root = deploymentPlanRoot(overview.GetRunId(), rec.GetDeploymentPlan(), nil)
		overview.Pipeline.Roots = append(overview.Pipeline.Roots, root)
		return overview
	}
	deploymentPlanRoot(overview.GetRunId(), rec.GetDeploymentPlan(), root)
	return overview
}

func workersFromRecord(rec *models.TestRunRecord) []*monitor.WorkerInfo {
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
	nodeIDs := make(map[string]struct{}, len(machines)+len(current))
	for nodeID := range machines {
		nodeIDs[nodeID] = struct{}{}
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

	workers := make([]*monitor.WorkerInfo, 0, len(ordered))
	for _, nodeID := range ordered {
		machine := machines[nodeID]
		status := workerStatus(machine, current[nodeID])
		workers = append(workers, &monitor.WorkerInfo{
			Id:                     "agent/" + nodeID,
			Kind:                   domainpb.Worker_KIND_AGENT,
			MachineId:              nodeID,
			Host:                   machineHost(machine),
			Online:                 workerOnline(machine),
			CurrentNodeExecutionId: current[nodeID],
			Status:                 status,
		})
	}
	return workers
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

func deploymentPlanRoot(runID string, plan *deploymentpb.DeploymentPlan, root *monitor.PipelineNode) *monitor.PipelineNode {
	if root == nil {
		id := deploymentbuilder.StageExecutionID(executeDeploymentPlanNodeName)
		root = &monitor.PipelineNode{
			NodeExecutionId: id,
			Name:            executeDeploymentPlanNodeName,
			Status:          deploymentPlanStatus(plan),
			LogRef:          logRef(runID, id, ""),
			Attempt:         1,
		}
	}
	if root.GetNodeExecutionId() == "" {
		root.NodeExecutionId = deploymentbuilder.StageExecutionID(executeDeploymentPlanNodeName)
	}
	if root.LogRef == nil {
		root.LogRef = logRef(runID, root.GetNodeExecutionId(), "")
	}
	root.Children = deploymentPlanChildren(runID, plan)
	return root
}

func deploymentPlanChildren(runID string, plan *deploymentpb.DeploymentPlan) []*monitor.PipelineNode {
	components := append([]*deploymentpb.ComponentDeployment(nil), plan.GetComponents()...)
	sort.SliceStable(components, func(i, j int) bool {
		if components[i].GetGlobalPriority() != components[j].GetGlobalPriority() {
			return components[i].GetGlobalPriority() < components[j].GetGlobalPriority()
		}
		if components[i].GetNodePriority() != components[j].GetNodePriority() {
			return components[i].GetNodePriority() < components[j].GetNodePriority()
		}
		return components[i].GetComponentId() < components[j].GetComponentId()
	})
	nodes := make([]*monitor.PipelineNode, 0, len(components))
	for _, component := range components {
		if component == nil {
			continue
		}
		componentID := component.GetComponentId()
		nodeExecutionID := labelOr(component.GetLabels(), deploymentbuilder.LabelNodeExecutionID, deploymentbuilder.ComponentExecutionID(componentID))
		nodes = append(nodes, &monitor.PipelineNode{
			NodeExecutionId: nodeExecutionID,
			Name:            componentID,
			Status:          pendingIfUnspecified(component.GetStatus()),
			Worker: &domainpb.Worker{
				Id:   "agent/" + component.GetNodeId(),
				Kind: domainpb.Worker_KIND_AGENT,
			},
			LogRef:   logRef(runID, nodeExecutionID, componentID),
			Children: agentStepNodes(runID, component),
			Attempt:  1,
		})
	}
	return nodes
}

func agentStepNodes(runID string, component *deploymentpb.ComponentDeployment) []*monitor.PipelineNode {
	steps := append([]*deploymentpb.AgentStep(nil), component.GetSteps()...)
	sort.SliceStable(steps, func(i, j int) bool {
		if steps[i].GetOrder() != steps[j].GetOrder() {
			return steps[i].GetOrder() < steps[j].GetOrder()
		}
		return steps[i].GetId() < steps[j].GetId()
	})
	nodes := make([]*monitor.PipelineNode, 0, len(steps))
	for _, step := range steps {
		if step == nil {
			continue
		}
		componentID := component.GetComponentId()
		nodeExecutionID := labelOr(step.GetLabels(), deploymentbuilder.LabelNodeExecutionID, deploymentbuilder.StepExecutionID(componentID, step.GetId()))
		nodes = append(nodes, &monitor.PipelineNode{
			NodeExecutionId: nodeExecutionID,
			Name:            agentStepName(step),
			Status:          pendingIfUnspecified(step.GetStatus()),
			Worker: &domainpb.Worker{
				Id:   "agent/" + component.GetNodeId(),
				Kind: domainpb.Worker_KIND_AGENT,
			},
			LogRef:  logRef(runID, nodeExecutionID, componentID),
			Attempt: 1,
		})
	}
	return nodes
}

func agentStepName(step *deploymentpb.AgentStep) string {
	action := deploymentbuilder.AgentStepActionKind(step)
	switch typed := step.GetAction().(type) {
	case *deploymentpb.AgentStep_CreateDir:
		return shortNodeName(action, typed.CreateDir.GetInfo().GetPath())
	case *deploymentpb.AgentStep_WriteFile:
		return shortNodeName(action, typed.WriteFile.GetInfo().GetPath())
	case *deploymentpb.AgentStep_FetchFile:
		return shortNodeName(action, typed.FetchFile.GetInfo().GetPath())
	case *deploymentpb.AgentStep_CallCmd:
		return shortNodeName(action, commandSummary(typed.CallCmd))
	default:
		return step.GetId()
	}
}

func commandSummary(cmd *common.Cmd) string {
	if cmd == nil || cmd.GetSpec() == nil {
		return ""
	}
	switch typed := cmd.GetSpec().GetCommand().(type) {
	case *common.Cmd_Spec_Argv:
		return strings.Join(typed.Argv.GetArgs(), " ")
	case *common.Cmd_Spec_Script:
		text := strings.TrimSpace(typed.Script.GetText())
		if i := strings.IndexByte(text, '\n'); i >= 0 {
			return text[:i]
		}
		return text
	default:
		return ""
	}
}

func shortNodeName(action, detail string) string {
	const maxDetail = 96
	if detail == "" {
		return action
	}
	if len(detail) > maxDetail {
		detail = detail[:maxDetail-1] + "..."
	}
	return action + ": " + detail
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

func findPipelineNode(nodes []*monitor.PipelineNode, name string) *monitor.PipelineNode {
	for _, node := range nodes {
		if node.GetName() == name {
			return node
		}
	}
	return nil
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

// projectOverview maps a live RunState onto a full Overview snapshot: one
// pipeline node + timeline events per stage, overall progress and timing.
func projectOverview(runID string, rs *workflowpb.RunState) *monitor.Overview {
	now := time.Now()
	terminal := isTerminalStatus(rs.GetStatus())

	roots := make([]*monitor.PipelineNode, 0, len(rs.GetStages()))
	timeline := make([]*monitor.Event, 0, len(rs.GetStages())+1)

	var (
		earliestStart *timestamppb.Timestamp
		latestFinish  *timestamppb.Timestamp
		completed     int
	)

	for _, st := range rs.GetStages() {
		started := st.GetStartedAt()
		finished := st.GetFinishedAt()

		roots = append(roots, &monitor.PipelineNode{
			NodeExecutionId: st.GetNodeExecutionId(),
			Name:            st.GetName(),
			Status:          st.GetStatus(),
			StartedAt:       started,
			FinishedAt:      finished,
			Duration:        spanDuration(started, finished, now),
			LogRef: &monitor.LogRef{
				RunId:           runID,
				NodeExecutionId: refString(st.GetNodeExecutionId()),
			},
			Attempt: st.GetAttempt(),
		})

		if started != nil && (earliestStart == nil || started.AsTime().Before(earliestStart.AsTime())) {
			earliestStart = started
		}
		if finished != nil && (latestFinish == nil || finished.AsTime().After(latestFinish.AsTime())) {
			latestFinish = finished
		}
		if isTerminalStatus(st.GetStatus()) {
			completed++
		}

		if started != nil {
			timeline = append(timeline, &monitor.Event{
				At:              started,
				Kind:            monitor.Event_KIND_STAGE_STARTED,
				Message:         "stage started: " + st.GetName(),
				NodeExecutionId: st.GetNodeExecutionId(),
				Status:          common.Status_STATUS_RUNNING,
			})
		}
		if finished != nil {
			timeline = append(timeline, &monitor.Event{
				At:              finished,
				Kind:            stageEndKind(st.GetStatus()),
				Message:         "stage finished: " + st.GetName(),
				NodeExecutionId: st.GetNodeExecutionId(),
				Status:          st.GetStatus(),
			})
		}
	}

	runAt := earliestStart
	if runAt == nil {
		runAt = timestamppb.New(now)
	}
	timeline = append(timeline, &monitor.Event{
		At:      runAt,
		Kind:    monitor.Event_KIND_RUN_STATUS,
		Message: "run status: " + rs.GetStatus().String(),
		Status:  rs.GetStatus(),
	})

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
