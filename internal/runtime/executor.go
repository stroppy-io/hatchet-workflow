package runtime

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PredicateFunc func(ctx DagContext, edge *primitive.Dag_Edge) bool

type PredicateRegistry interface {
	GetPredicate(name string) (PredicateFunc, bool)
}

type PredicateRegistryMap map[string]PredicateFunc

func (r PredicateRegistryMap) GetPredicate(name string) (PredicateFunc, bool) {
	fn, ok := r[name]
	return fn, ok
}

type DagRefRunner interface {
	RunDagRef(ctx context.Context, ref *primitive.Dag_Node_DagRef) (*primitive.Dag, error)
}

type ExecutorOption func(*Executor)

type Executor struct {
	tasks      TasksRegistry
	predicates PredicateRegistry
	dagRefs    DagRefRunner
	save       func(context.Context, *primitive.Dag) error
	now        func() time.Time
	sleep      func(context.Context, time.Duration) error
}

func NewExecutor(tasks TasksRegistry, opts ...ExecutorOption) *Executor {
	e := &Executor{
		tasks: tasks,
		now:   time.Now,
		sleep: func(ctx context.Context, d time.Duration) error {
			if d <= 0 {
				return nil
			}
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func WithPredicates(predicates PredicateRegistry) ExecutorOption {
	return func(e *Executor) {
		e.predicates = predicates
	}
}

func WithSaveHook(save func(context.Context, *primitive.Dag) error) ExecutorOption {
	return func(e *Executor) {
		e.save = save
	}
}

func WithDagRefRunner(runner DagRefRunner) ExecutorOption {
	return func(e *Executor) {
		e.dagRefs = runner
	}
}

func WithClock(now func() time.Time, sleep func(context.Context, time.Duration) error) ExecutorOption {
	return func(e *Executor) {
		if now != nil {
			e.now = now
		}
		if sleep != nil {
			e.sleep = sleep
		}
	}
}

func (e *Executor) Run(ctx context.Context, dag *primitive.Dag) error {
	if dag == nil {
		return fmt.Errorf("dag is nil")
	}
	normalizeDag(dag)
	if err := ValidateDag(dag); err != nil {
		failure := FailureFromError(err, FailureSourceRuntime, FailurePhaseDag, FailureCodeDagInvalid, 0, false, nil)
		setDagFailed(dag, "", failure, e.now())
		_ = e.saveDag(ctx, dag)
		return err
	}

	RecoverDag(dag, e.now())
	if isDagTerminal(dag.GetStatus()) {
		_ = e.saveDag(ctx, dag)
		return terminalError(dag)
	}
	if dag.GetStatus() != primitive.Status_STATUS_CANCELLING {
		setDagStatus(dag, primitive.Status_STATUS_RUNNING, e.now())
	}
	if err := e.saveDag(ctx, dag); err != nil {
		return err
	}

	for {
		if dag.GetStatus() == primitive.Status_STATUS_CANCELLING {
			if err := e.runAlwaysRun(ctx, dag); err != nil {
				return err
			}
			finalizeDag(dag, e.now())
			if err := e.saveDag(ctx, dag); err != nil {
				return err
			}
			return terminalError(dag)
		}

		if ctx.Err() != nil {
			e.cancelPending(dag, ctx.Err())
			_ = e.runAlwaysRun(ctx, dag)
			finalizeDag(dag, e.now())
			_ = e.saveDag(context.Background(), dag)
			return ctx.Err()
		}

		// A conditional branch the run did NOT take never becomes ready; skip such
		// dead nodes (terminal source + unsatisfiable join) so the dag can finish.
		if e.skipDeadNodes(dag, e.now()) {
			if err := e.saveDag(ctx, dag); err != nil {
				return err
			}
		}

		if shouldStopOrdinary(dag) {
			if err := e.runAlwaysRun(ctx, dag); err != nil {
				return err
			}
			finalizeDag(dag, e.now())
			if err := e.saveDag(ctx, dag); err != nil {
				return err
			}
			return terminalError(dag)
		}

		ready := e.readyNodes(dag, false)
		if len(ready) == 0 {
			if anyNodeRunning(dag) {
				// Only externally-driven work remains in flight (agent-locus nodes
				// leased out, or sub-dags awaiting agents). Yield: the processor
				// re-runs this Dag after a Report advances a node. The server never
				// blocks waiting for an agent.
				if err := e.saveDag(ctx, dag); err != nil {
					return err
				}
				return nil
			}
			if ordinaryNodesTerminal(dag) {
				// always_run teardown is deferred to the end and runs REGARDLESS of
				// success or failure/cancellation (dag.proto: "reserved for
				// cleanup/teardown and may still run after failure or cancellation").
				if err := e.runAlwaysRun(ctx, dag); err != nil {
					return err
				}
				skipPendingAlwaysRun(dag, e.now())
				finalizeDag(dag, e.now())
				if err := e.saveDag(ctx, dag); err != nil {
					return err
				}
				return terminalError(dag)
			}
			if allTerminal(dag) {
				finalizeDag(dag, e.now())
				if err := e.saveDag(ctx, dag); err != nil {
					return err
				}
				return terminalError(dag)
			}
			next := nextRetryAt(dag)
			if next.IsZero() {
				failure := FailureFromError(fmt.Errorf("dag has no ready nodes and is not terminal"), FailureSourceRuntime, FailurePhaseDag, FailureCodeDagInvalid, 0, false, nil)
				setDagFailed(dag, "", failure, e.now())
				finalizeDag(dag, e.now())
				if err := e.saveDag(ctx, dag); err != nil {
					return err
				}
				return terminalError(dag)
			}
			if err := e.sleep(ctx, next.Sub(e.now())); err != nil {
				continue
			}
			continue
		}

		if max := dag.GetScheduling().GetMaxParallelism(); max > 0 && len(ready) > int(max) {
			ready = ready[:max]
		}
		if err := e.runBatch(ctx, dag, ready); err != nil {
			return err
		}
	}
}

func RecoverDag(dag *primitive.Dag, now time.Time) {
	if dag == nil {
		return
	}
	normalizeDag(dag)
	recoverDagExecution(dag, now)
	for _, node := range dag.GetNodes() {
		if sub := node.GetSubDag(); sub != nil && !isDagTerminal(sub.GetStatus()) {
			RecoverDag(sub, now)
		}
		recoverNodeExecution(dag, node, now)
	}
	if dag.GetStatus() == primitive.Status_STATUS_CANCELLING {
		recoverCancellingDag(dag, now)
		return
	}
	if allTerminal(dag) {
		finalizeDag(dag, now)
	}
}

func recoverDagExecution(dag *primitive.Dag, now time.Time) {
	if dag.Execution == nil {
		dag.Execution = &primitive.Dag_Execution{}
	}
	if dag.Execution.Status == primitive.Status_STATUS_RUNNING && dag.Status == primitive.Status_STATUS_PENDING {
		dag.Status = primitive.Status_STATUS_RUNNING
	}
	if dag.Execution.StartedAt == nil && (dag.Status == primitive.Status_STATUS_RUNNING || dag.Status == primitive.Status_STATUS_CANCELLING) {
		dag.Execution.StartedAt = timestamppb.New(now)
	}
}

func recoverNodeExecution(dag *primitive.Dag, node *primitive.Dag_Node, now time.Time) {
	if node.GetStatus() != primitive.Status_STATUS_RUNNING {
		return
	}
	if nodeIsAgentLocus(node) {
		// Agent-locus liveness is governed by the lease (CommandQueue), not by
		// crash recovery — a validly-leased command keeps running across restarts.
		// TODO(runtime): a sub-dag node merely AWAITING agents is still reset here
		// (it self-heals by re-running + re-parking, with churn). Distinguish it.
		return
	}
	exec := ensureNodeExecution(node)
	state := ensureRetryState(exec)
	attempt := state.GetAttempt()
	if attempt == 0 {
		attempt = 1
		state.Attempt = attempt
	}
	failure := &primitive.Dag_Failure{
		Message:    fmt.Sprintf("node %q was interrupted while running", node.GetId()),
		Code:       FailureCodeNodeInterrupted,
		Source:     FailureSourceRuntime,
		Phase:      FailurePhaseRecovery,
		Attempt:    attempt,
		Retryable:  true,
		OccurredAt: timestamppb.New(now),
		Metadata: map[string]string{
			"node_id":           node.GetId(),
			"node_execution_id": node.GetExecutionId(),
		},
	}
	if handler := node.GetTaskState().GetHandlerName(); handler != "" {
		failure.Metadata["handler_name"] = handler
	}
	recordNodeFailure(node, failure)

	delay := nextDelay(node.GetScheduling().GetRetryPolicy(), state.GetCurrentDelay())
	node.Status = primitive.Status_STATUS_RETRY_WAIT
	exec.Status = primitive.Status_STATUS_RETRY_WAIT
	state.CurrentDelay = durationpb.New(delay)
	state.NextRunAt = timestamppb.New(now.Add(delay))
	state.LastError = failure.GetMessage()
	state.LastAttemptAt = timestamppb.New(now)

	if dag.GetExecution().GetFailure() == nil {
		dag.Execution.Failure = proto.Clone(failure).(*primitive.Dag_Failure)
	}
}

func recoverCancellingDag(dag *primitive.Dag, now time.Time) {
	failure := FailureFromError(context.Canceled, FailureSourceRuntime, FailurePhaseRecovery, FailureCodeDagCancelled, 0, false, nil)
	for _, node := range dag.GetNodes() {
		if node.GetScheduling().GetAlwaysRun() {
			if node.GetStatus() == primitive.Status_STATUS_RUNNING {
				recoverNodeExecution(dag, node, now)
			}
			continue
		}
		if !isTerminalStatus(node.GetStatus()) {
			markNodeCancelled(node, failure, now)
		}
	}
	if len(collectRunnableAlwaysRun(dag)) == 0 {
		dag.Status = primitive.Status_STATUS_CANCELLED
		dag.Execution.Status = primitive.Status_STATUS_CANCELLED
		if dag.Execution.FinishedAt == nil {
			dag.Execution.FinishedAt = timestamppb.New(now)
		}
	}
}

func (e *Executor) runAlwaysRun(ctx context.Context, dag *primitive.Dag) error {
	for {
		ready := e.readyNodes(dag, true)
		if len(ready) == 0 {
			return nil
		}
		if max := dag.GetScheduling().GetMaxParallelism(); max > 0 && len(ready) > int(max) {
			ready = ready[:max]
		}
		if err := e.runBatch(ctx, dag, ready); err != nil {
			return err
		}
	}
}

func (e *Executor) runBatch(ctx context.Context, dag *primitive.Dag, nodes []*primitive.Dag_Node) error {
	now := e.now()
	for _, node := range nodes {
		markNodeRunning(node, now)
	}
	if err := e.saveDag(ctx, dag); err != nil {
		return err
	}

	results := make(chan nodeResult, len(nodes))
	g, runCtx := errgroup.WithContext(ctx)
	for _, node := range nodes {
		n := node
		g.Go(func() error {
			results <- e.executeNode(runCtx, dag, n)
			return nil
		})
	}
	_ = g.Wait()
	close(results)

	for result := range results {
		e.applyNodeResult(dag, result)
	}
	return e.saveDag(ctx, dag)
}

type nodeResult struct {
	nodeID    string
	taskState *primitive.Dag_Node_TaskState
	subDag    *primitive.Dag
	cancelled bool
	// parked marks externally-driven work that has not finished in-process: an
	// agent-locus node (leased out via Poll, advanced by Report) or a sub-dag that
	// yielded while waiting on agents. The node stays RUNNING and the executor
	// yields; the processor re-runs it after a Report advances a node.
	parked bool
	err    error
}

// dagContext is the DagContext passed to a server-locus task: it exposes the dag
// (its input + every node's output) so a task is a pure proto->proto transform that
// reads upstream outputs via GetOutput. Outputs are read off the live dag, which the
// executor mutates as nodes complete.
type dagContext struct {
	context.Context
	dag *primitive.Dag
}

func (d *dagContext) Dag() *primitive.Dag { return d.dag }

func (d *dagContext) GetOutput(id string) (*anypb.Any, error) {
	for _, n := range d.dag.GetNodes() {
		if n.GetId() == id {
			if out := n.GetTaskState().GetOutput(); out != nil {
				return out, nil
			}
			return nil, fmt.Errorf("node %q has no output yet", id)
		}
	}
	return nil, fmt.Errorf("node %q not found", id)
}

func (e *Executor) executeNode(ctx context.Context, dag *primitive.Dag, node *primitive.Dag_Node) nodeResult {
	switch v := node.GetVariant().(type) {
	case *primitive.Dag_Node_TaskState_:
		if nodeIsAgentLocus(node) {
			// Agent-locus: never executed by the server. The agent leases it via
			// Poll and Reports terminal status. Park it (stays RUNNING) and yield.
			return nodeResult{nodeID: node.GetId(), parked: true}
		}
		state, err := RunTask(&dagContext{Context: ctx, dag: dag}, v.TaskState, e.tasks)
		return nodeResult{nodeID: node.GetId(), taskState: state, err: err}
	case *primitive.Dag_Node_SubDag:
		sub := proto.Clone(v.SubDag).(*primitive.Dag)
		runErr := e.Run(ctx, sub)
		res := nodeResult{nodeID: node.GetId(), subDag: sub}
		switch sub.GetStatus() {
		case primitive.Status_STATUS_COMPLETED:
			// success; terminal result mirrored into owning node
		case primitive.Status_STATUS_CANCELLED:
			res.cancelled = true
		case primitive.Status_STATUS_RUNNING, primitive.Status_STATUS_PENDING:
			// sub-dag yielded while waiting on agent commands; re-runs next tick.
			res.parked = true
		default:
			if runErr == nil {
				runErr = fmt.Errorf("sub-dag %q ended in status %s", sub.GetId(), sub.GetStatus())
			}
			res.err = NewFailureError(runErr, &primitive.Dag_Failure{
				Code:      FailureCodeSubDagFailed,
				Source:    FailureSourceRuntime,
				Phase:     FailurePhaseSubDag,
				Retryable: true,
			})
		}
		return res
	case *primitive.Dag_Node_DagRef_:
		return e.executeDagRef(ctx, node, v.DagRef)
	default:
		err := fmt.Errorf("node %q has no executable variant", node.GetId())
		return nodeResult{nodeID: node.GetId(), err: NewFailureError(err, &primitive.Dag_Failure{
			Code:   FailureCodeDagInvalid,
			Source: FailureSourceRuntime,
			Phase:  FailurePhaseDag,
		})}
	}
}

func (e *Executor) executeDagRef(ctx context.Context, node *primitive.Dag_Node, ref *primitive.Dag_Node_DagRef) nodeResult {
	id := node.GetId()
	meta := map[string]string{"dag_id": ref.GetDagId()}
	if e.dagRefs == nil {
		err := fmt.Errorf("node %q references dag %q but no dag ref runner is configured", id, ref.GetDagId())
		return nodeResult{nodeID: id, err: NewFailureError(err, &primitive.Dag_Failure{
			Code:     FailureCodeDagRefRunnerMissing,
			Source:   FailureSourceRuntime,
			Phase:    FailurePhaseDagRef,
			Metadata: meta,
		})}
	}
	child, err := e.dagRefs.RunDagRef(ctx, ref)
	if err != nil {
		return nodeResult{nodeID: id, err: NewFailureError(err, &primitive.Dag_Failure{
			Code:      FailureCodeDagRefFailed,
			Source:    FailureSourceRuntime,
			Phase:     FailurePhaseDagRef,
			Retryable: true,
			Metadata:  meta,
		})}
	}
	if child == nil {
		err := fmt.Errorf("dag ref %q runner returned no dag", ref.GetDagId())
		return nodeResult{nodeID: id, err: NewFailureError(err, &primitive.Dag_Failure{
			Code:     FailureCodeDagRefFailed,
			Source:   FailureSourceRuntime,
			Phase:    FailurePhaseDagRef,
			Metadata: meta,
		})}
	}
	switch child.GetStatus() {
	case primitive.Status_STATUS_COMPLETED:
		return nodeResult{nodeID: id}
	case primitive.Status_STATUS_CANCELLED:
		return nodeResult{nodeID: id, cancelled: true}
	case primitive.Status_STATUS_FAILED:
		return nodeResult{nodeID: id, err: NewFailureError(terminalError(child), &primitive.Dag_Failure{
			Code:      FailureCodeDagRefFailed,
			Source:    FailureSourceRuntime,
			Phase:     FailurePhaseDagRef,
			Retryable: true,
			Metadata:  meta,
		})}
	default:
		// Child not terminal yet: wait by re-polling under the node retry policy.
		err := fmt.Errorf("dag ref %q not terminal: status %s", ref.GetDagId(), child.GetStatus())
		return nodeResult{nodeID: id, err: NewFailureError(err, &primitive.Dag_Failure{
			Code:      FailureCodeDagRefPending,
			Source:    FailureSourceRuntime,
			Phase:     FailurePhaseDagRef,
			Retryable: true,
			Metadata:  meta,
		})}
	}
}

func (e *Executor) applyNodeResult(dag *primitive.Dag, result nodeResult) {
	node := findNode(dag, result.nodeID)
	if node == nil {
		return
	}
	if result.parked {
		// Externally-driven work: leave the node RUNNING (set by runBatch). For a
		// parked sub-dag, mirror the latest embedded snapshot so progress persists.
		if result.subDag != nil {
			node.Variant = &primitive.Dag_Node_SubDag{SubDag: result.subDag}
		}
		return
	}
	if result.subDag != nil {
		node.Variant = &primitive.Dag_Node_SubDag{SubDag: result.subDag}
	}
	if result.cancelled {
		failure := FailureFromError(
			fmt.Errorf("node %q cancelled by terminal child", node.GetId()),
			FailureSourceRuntime, phaseForNode(node), FailureCodeDagCancelled,
			node.GetExecution().GetRetryState().GetAttempt(), false, nodeFailureMetadata(node),
		)
		recordNodeFailure(node, failure)
		markNodeCancelled(node, failure, e.now())
		if dag.GetExecution().GetFailedNodeId() == "" {
			recordDagFailure(dag, node.GetId(), failure, e.now())
		}
		return
	}
	if result.err == nil {
		if result.taskState != nil {
			node.Variant = &primitive.Dag_Node_TaskState_{TaskState: result.taskState}
		}
		markNodeCompleted(node, e.now())
		return
	}

	exec := ensureNodeExecution(node)
	state := ensureRetryState(exec)
	attempt := state.GetAttempt()
	// opinion is the executor/task verdict on retryability, recorded on the
	// failure before retry policy budget is applied (proto Failure.retryable).
	opinion := errorRetryableOpinion(result.err)
	failure := FailureFromError(result.err, FailureSourceTask, FailurePhaseTaskCall, FailureCodeTaskFailed, attempt, opinion, nodeFailureMetadata(node))
	recordNodeFailure(node, failure)

	if opinion && canRetry(node.GetScheduling().GetRetryPolicy(), attempt) {
		delay := nextDelay(node.GetScheduling().GetRetryPolicy(), state.GetCurrentDelay())
		nextRun := e.now().Add(delay)
		node.Status = primitive.Status_STATUS_RETRY_WAIT
		exec.Status = primitive.Status_STATUS_RETRY_WAIT
		state.CurrentDelay = durationpb.New(delay)
		state.NextRunAt = timestamppb.New(nextRun)
		state.LastError = failure.GetMessage()
		state.LastAttemptAt = timestamppb.New(e.now())
		return
	}

	markNodeFailed(node, failure, e.now())
	if dag.GetExecution().GetFailedNodeId() == "" {
		recordDagFailure(dag, node.GetId(), failure, e.now())
	}
}

func nodeFailureMetadata(node *primitive.Dag_Node) map[string]string {
	meta := map[string]string{
		"node_id":           node.GetId(),
		"node_execution_id": node.GetExecutionId(),
	}
	if handlerName := node.GetTaskState().GetHandlerName(); handlerName != "" {
		meta["handler_name"] = handlerName
	}
	if dagID := node.GetDagRef().GetDagId(); dagID != "" {
		meta["dag_id"] = dagID
	}
	return meta
}

// nodeIsAgentLocus reports whether the node is an agent-locus task — leased out
// to an agent via Poll and never executed by the server.
func nodeIsAgentLocus(node *primitive.Dag_Node) bool {
	return node.GetTaskState().GetLocus() == primitive.Dag_Node_TaskState_EXECUTION_LOCUS_AGENT
}

// anyNodeRunning reports whether any direct node is RUNNING. Server task nodes
// finish in-process within a batch, so a RUNNING node between iterations means
// externally-driven (agent / awaiting-agent sub-dag) work is in flight.
func anyNodeRunning(dag *primitive.Dag) bool {
	for _, node := range dag.GetNodes() {
		if node.GetStatus() == primitive.Status_STATUS_RUNNING {
			return true
		}
	}
	return false
}

func phaseForNode(node *primitive.Dag_Node) string {
	switch {
	case node.GetSubDag() != nil:
		return FailurePhaseSubDag
	case node.GetDagRef() != nil:
		return FailurePhaseDagRef
	default:
		return FailurePhaseDag
	}
}

func (e *Executor) readyNodes(dag *primitive.Dag, alwaysRunOnly bool) []*primitive.Dag_Node {
	now := e.now()
	var ready []*primitive.Dag_Node
	for _, node := range dag.GetNodes() {
		if alwaysRunOnly && !node.GetScheduling().GetAlwaysRun() {
			continue
		}
		if !alwaysRunOnly && node.GetScheduling().GetAlwaysRun() {
			continue
		}
		if !isRunnableStatus(node.GetStatus()) {
			continue
		}
		if node.GetStatus() == primitive.Status_STATUS_RETRY_WAIT && retryWaitUntil(node).After(now) {
			continue
		}
		if alwaysRunOnly {
			if noRunningDependencies(dag, node) {
				ready = append(ready, node)
			}
			continue
		}
		if edgesSatisfied(dag, node, e.predicates) {
			ready = append(ready, node)
		}
	}
	sort.SliceStable(ready, func(i, j int) bool {
		return ready[i].GetScheduling().GetPriority() > ready[j].GetScheduling().GetPriority()
	})
	return ready
}

func (e *Executor) cancelPending(dag *primitive.Dag, err error) {
	failure := FailureFromError(err, FailureSourceRuntime, FailurePhaseDag, FailureCodeDagCancelled, 0, false, nil)
	setDagStatus(dag, primitive.Status_STATUS_CANCELLING, e.now())
	for _, node := range dag.GetNodes() {
		if node.GetScheduling().GetAlwaysRun() {
			continue
		}
		if isRunnableStatus(node.GetStatus()) {
			markNodeCancelled(node, failure, e.now())
		}
	}
	setDagFailed(dag, "", failure, e.now())
	dag.Status = primitive.Status_STATUS_CANCELLED
	dag.Execution.Status = primitive.Status_STATUS_CANCELLED
}

func (e *Executor) saveDag(ctx context.Context, dag *primitive.Dag) error {
	if e.save == nil {
		return nil
	}
	return e.save(ctx, proto.Clone(dag).(*primitive.Dag))
}

func ValidateDag(dag *primitive.Dag) error {
	return validateDag(dag, make(map[string]string))
}

func validateDag(dag *primitive.Dag, executionIDs map[string]string) error {
	if dag == nil {
		return fmt.Errorf("dag is nil")
	}
	if dag.GetId() == "" {
		return fmt.Errorf("dag id is required")
	}
	if len(dag.GetNodes()) == 0 {
		return fmt.Errorf("dag %q has no nodes", dag.GetId())
	}
	nodes := make(map[string]*primitive.Dag_Node, len(dag.GetNodes()))
	for _, node := range dag.GetNodes() {
		if node.GetId() == "" {
			return fmt.Errorf("dag %q contains node without id", dag.GetId())
		}
		if _, exists := nodes[node.GetId()]; exists {
			return fmt.Errorf("dag %q contains duplicate node %q", dag.GetId(), node.GetId())
		}
		if node.GetExecutionId() == "" {
			return fmt.Errorf("dag %q node %q execution_id is required", dag.GetId(), node.GetId())
		}
		if owner, exists := executionIDs[node.GetExecutionId()]; exists {
			return fmt.Errorf("dag %q node %q has duplicate execution_id %q already used by %s", dag.GetId(), node.GetId(), node.GetExecutionId(), owner)
		}
		executionIDs[node.GetExecutionId()] = fmt.Sprintf("dag %q node %q", dag.GetId(), node.GetId())
		if node.GetScheduling() == nil {
			return fmt.Errorf("node %q scheduling is required", node.GetId())
		}
		if node.GetVariant() == nil {
			return fmt.Errorf("node %q variant is required", node.GetId())
		}
		if sub := node.GetSubDag(); sub != nil {
			if err := validateDag(sub, executionIDs); err != nil {
				return err
			}
		}
		nodes[node.GetId()] = node
	}
	for _, edge := range dag.GetEdges() {
		if nodes[edge.GetSource()] == nil {
			return fmt.Errorf("edge %q references unknown source %q", edge.GetId(), edge.GetSource())
		}
		if nodes[edge.GetTarget()] == nil {
			return fmt.Errorf("edge %q references unknown target %q", edge.GetId(), edge.GetTarget())
		}
	}
	return detectCycle(dag, nodes)
}

func detectCycle(dag *primitive.Dag, nodes map[string]*primitive.Dag_Node) error {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(nodes))
	out := make(map[string][]string)
	for _, edge := range dag.GetEdges() {
		out[edge.GetSource()] = append(out[edge.GetSource()], edge.GetTarget())
	}
	var visit func(string) error
	visit = func(id string) error {
		color[id] = gray
		for _, to := range out[id] {
			switch color[to] {
			case gray:
				return fmt.Errorf("dag %q cycle detected at node %q", dag.GetId(), to)
			case white:
				if err := visit(to); err != nil {
					return err
				}
			}
		}
		color[id] = black
		return nil
	}
	var errs []error
	for id := range nodes {
		if color[id] == white {
			errs = append(errs, visit(id))
		}
	}
	return errors.Join(errs...)
}

func normalizeDag(dag *primitive.Dag) {
	normalizeDagWithPrefix(dag, dag.GetId())
}

func normalizeDagWithPrefix(dag *primitive.Dag, prefix string) {
	if dag.Scheduling == nil {
		dag.Scheduling = &primitive.Dag_Scheduling{
			OnNodeFailure: primitive.Dag_Scheduling_ON_NODE_FAILURE_STOP,
		}
	}
	if dag.Scheduling.OnNodeFailure == primitive.Dag_Scheduling_ON_NODE_FAILURE_UNSPECIFIED {
		dag.Scheduling.OnNodeFailure = primitive.Dag_Scheduling_ON_NODE_FAILURE_STOP
	}
	if dag.Status == primitive.Status_STATUS_UNSPECIFIED {
		dag.Status = primitive.Status_STATUS_PENDING
	}
	if dag.Execution == nil {
		dag.Execution = &primitive.Dag_Execution{}
	}
	if dag.Execution.Status == primitive.Status_STATUS_UNSPECIFIED {
		dag.Execution.Status = dag.Status
	}
	for _, node := range dag.GetNodes() {
		if node.ExecutionId == "" {
			node.ExecutionId = defaultNodeExecutionID(prefix, node.GetId())
		}
		if node.Scheduling == nil {
			node.Scheduling = &primitive.Dag_Node_Scheduling{}
		}
		if node.Scheduling.RetryPolicy == nil {
			node.Scheduling.RetryPolicy = &primitive.Retry_Policy{Attempts: 1}
		}
		if node.Status == primitive.Status_STATUS_UNSPECIFIED {
			node.Status = primitive.Status_STATUS_PENDING
		}
		exec := ensureNodeExecution(node)
		if exec.Status == primitive.Status_STATUS_UNSPECIFIED {
			exec.Status = node.Status
		}
		ensureRetryState(exec)
		if sub := node.GetSubDag(); sub != nil {
			normalizeDagWithPrefix(sub, node.GetExecutionId())
			sub.Scheduling.IsSubDag = true
		}
	}
}

func defaultNodeExecutionID(prefix, nodeID string) string {
	if prefix == "" {
		return nodeID
	}
	if nodeID == "" {
		return prefix
	}
	return prefix + "." + nodeID
}

func setDagStatus(dag *primitive.Dag, status primitive.Status, now time.Time) {
	dag.Status = status
	if dag.Execution == nil {
		dag.Execution = &primitive.Dag_Execution{}
	}
	dag.Execution.Status = status
	if dag.Execution.StartedAt == nil && status == primitive.Status_STATUS_RUNNING {
		dag.Execution.StartedAt = timestamppb.New(now)
	}
}

func setDagFailed(dag *primitive.Dag, nodeID string, failure *primitive.Dag_Failure, now time.Time) {
	recordDagFailure(dag, nodeID, failure, now)
	dag.Status = primitive.Status_STATUS_FAILED
	dag.Execution.Status = primitive.Status_STATUS_FAILED
}

func recordDagFailure(dag *primitive.Dag, nodeID string, failure *primitive.Dag_Failure, now time.Time) {
	if dag.Execution == nil {
		dag.Execution = &primitive.Dag_Execution{}
	}
	if nodeID != "" && dag.Execution.FailedNodeId == "" {
		dag.Execution.FailedNodeId = nodeID
	}
	if failure != nil {
		dag.Execution.Failure = proto.Clone(failure).(*primitive.Dag_Failure)
		dag.Execution.Failures = append(dag.Execution.Failures, proto.Clone(failure).(*primitive.Dag_Failure))
	}
	if dag.Execution.StartedAt == nil {
		dag.Execution.StartedAt = timestamppb.New(now)
	}
}

func finalizeDag(dag *primitive.Dag, now time.Time) {
	if dag.Execution == nil {
		dag.Execution = &primitive.Dag_Execution{}
	}
	if dag.Execution.FinishedAt == nil {
		dag.Execution.FinishedAt = timestamppb.New(now)
	}
	if dag.Status == primitive.Status_STATUS_FAILED || dag.Status == primitive.Status_STATUS_CANCELLED {
		markUnreachableSkipped(dag, now)
		dag.Execution.Status = dag.Status
		return
	}
	for _, node := range dag.GetNodes() {
		switch node.GetStatus() {
		case primitive.Status_STATUS_FAILED:
			dag.Status = primitive.Status_STATUS_FAILED
			dag.Execution.Status = primitive.Status_STATUS_FAILED
			markUnreachableSkipped(dag, now)
			if dag.Execution.FailedNodeId == "" {
				dag.Execution.FailedNodeId = node.GetId()
			}
			if dag.Execution.Failure == nil {
				if failure := node.GetExecution().GetFailure(); failure != nil {
					dag.Execution.Failure = proto.Clone(failure).(*primitive.Dag_Failure)
				}
			}
			return
		case primitive.Status_STATUS_CANCELLED, primitive.Status_STATUS_CANCELLING:
			dag.Status = primitive.Status_STATUS_CANCELLED
			dag.Execution.Status = primitive.Status_STATUS_CANCELLED
			return
		case primitive.Status_STATUS_PENDING, primitive.Status_STATUS_RUNNING, primitive.Status_STATUS_RETRY_WAIT:
			if shouldStopOrdinary(dag) && !node.GetScheduling().GetAlwaysRun() {
				markNodeSkipped(node, now)
				continue
			}
			return
		}
	}
	dag.Status = primitive.Status_STATUS_COMPLETED
	dag.Execution.Status = primitive.Status_STATUS_COMPLETED
}

// skipDeadNodes marks PENDING ordinary nodes whose join can never be satisfied as
// SKIPPED: an incoming edge is "dead" when its source is terminal but its condition
// is unsatisfied. JOIN_ALL dies if any incoming edge is dead; JOIN_ANY dies only
// when every incoming edge is dead. This is how the conditional branch the run did
// not take (e.g. the unselected provider) terminates so the dag can complete.
func (e *Executor) skipDeadNodes(dag *primitive.Dag, now time.Time) bool {
	changed := false
	for _, node := range dag.GetNodes() {
		if node.GetScheduling().GetAlwaysRun() || node.GetStatus() != primitive.Status_STATUS_PENDING {
			continue
		}
		incoming := incomingEdges(dag, node.GetId())
		if len(incoming) == 0 {
			continue
		}
		dead := 0
		for _, edge := range incoming {
			src := findNode(dag, edge.GetSource())
			if src != nil && isTerminalStatus(src.GetStatus()) && !edgeSatisfied(dag, edge, e.predicates) {
				dead++
			}
		}
		policy := node.GetScheduling().GetJoinPolicy()
		if policy == primitive.Dag_Node_Scheduling_JOIN_POLICY_UNSPECIFIED {
			policy = primitive.Dag_Node_Scheduling_JOIN_POLICY_ALL
		}
		isDead := dead > 0
		if policy == primitive.Dag_Node_Scheduling_JOIN_POLICY_ANY {
			isDead = dead == len(incoming)
		}
		if isDead {
			markNodeSkipped(node, now)
			changed = true
		}
	}
	return changed
}

func markUnreachableSkipped(dag *primitive.Dag, now time.Time) {
	for _, node := range dag.GetNodes() {
		if node.GetScheduling().GetAlwaysRun() {
			continue
		}
		if isRunnableStatus(node.GetStatus()) {
			markNodeSkipped(node, now)
		}
	}
}

func markNodeRunning(node *primitive.Dag_Node, now time.Time) {
	exec := ensureNodeExecution(node)
	state := ensureRetryState(exec)
	state.Attempt++
	if state.FirstAttemptAt == nil {
		state.FirstAttemptAt = timestamppb.New(now)
	}
	state.LastAttemptAt = timestamppb.New(now)
	state.NextRunAt = nil
	node.Status = primitive.Status_STATUS_RUNNING
	exec.Status = primitive.Status_STATUS_RUNNING
	if exec.StartedAt == nil {
		exec.StartedAt = timestamppb.New(now)
	}
}

func markNodeCompleted(node *primitive.Dag_Node, now time.Time) {
	exec := ensureNodeExecution(node)
	node.Status = primitive.Status_STATUS_COMPLETED
	exec.Status = primitive.Status_STATUS_COMPLETED
	exec.FinishedAt = timestamppb.New(now)
	exec.Failure = nil
}

func markNodeFailed(node *primitive.Dag_Node, failure *primitive.Dag_Failure, now time.Time) {
	exec := ensureNodeExecution(node)
	node.Status = primitive.Status_STATUS_FAILED
	exec.Status = primitive.Status_STATUS_FAILED
	exec.Failure = proto.Clone(failure).(*primitive.Dag_Failure)
	exec.FinishedAt = timestamppb.New(now)
}

func markNodeCancelled(node *primitive.Dag_Node, failure *primitive.Dag_Failure, now time.Time) {
	exec := ensureNodeExecution(node)
	node.Status = primitive.Status_STATUS_CANCELLED
	exec.Status = primitive.Status_STATUS_CANCELLED
	exec.Failure = proto.Clone(failure).(*primitive.Dag_Failure)
	exec.FinishedAt = timestamppb.New(now)
}

func markNodeSkipped(node *primitive.Dag_Node, now time.Time) {
	exec := ensureNodeExecution(node)
	node.Status = primitive.Status_STATUS_SKIPPED
	exec.Status = primitive.Status_STATUS_SKIPPED
	exec.FinishedAt = timestamppb.New(now)
}

func recordNodeFailure(node *primitive.Dag_Node, failure *primitive.Dag_Failure) {
	exec := ensureNodeExecution(node)
	exec.Failure = proto.Clone(failure).(*primitive.Dag_Failure)
	if node.GetScheduling().GetRetryPolicy().GetLastErrorOnly() {
		// Keep only the most recent attempt failure.
		exec.Failures = []*primitive.Dag_Failure{proto.Clone(failure).(*primitive.Dag_Failure)}
		return
	}
	exec.Failures = append(exec.Failures, proto.Clone(failure).(*primitive.Dag_Failure))
}

func ensureNodeExecution(node *primitive.Dag_Node) *primitive.Dag_Node_Execution {
	if node.Execution == nil {
		node.Execution = &primitive.Dag_Node_Execution{}
	}
	return node.Execution
}

func ensureRetryState(exec *primitive.Dag_Node_Execution) *primitive.Retry_State {
	if exec.RetryState == nil {
		exec.RetryState = &primitive.Retry_State{}
	}
	return exec.RetryState
}

func canRetry(policy *primitive.Retry_Policy, attempt uint32) bool {
	if policy == nil {
		return false
	}
	if policy.GetUntilSucceeded() && policy.GetAttempts() == 0 {
		return true
	}
	attempts := policy.GetAttempts()
	if attempts == 0 {
		attempts = 1
	}
	return attempt < attempts
}

// maxBackoffDuration is the largest delay that can still be doubled without
// overflowing time.Duration (int64 nanoseconds).
const maxBackoffDuration = time.Duration(1) << 62

func nextDelay(policy *primitive.Retry_Policy, current *durationpb.Duration) time.Duration {
	if policy == nil {
		return 0
	}
	base := pbDuration(policy.GetDelay())
	maxDelay := pbDuration(policy.GetMaxDelay())
	var delay time.Duration
	switch policy.GetDelayType() {
	case primitive.Retry_DELAY_TYPE_BACKOFF, primitive.Retry_DELAY_TYPE_BACKOFF_JITTER:
		prev := pbDuration(current)
		switch {
		case prev <= 0:
			delay = base
		case prev > maxBackoffDuration:
			// Doubling would overflow time.Duration; hold at the previous delay.
			delay = prev
		default:
			delay = prev * 2
		}
	case primitive.Retry_DELAY_TYPE_RANDOM:
		// retry-go RandomDelay draws uniformly from [0, max_jitter).
		delay = randomDuration(0, pbDuration(policy.GetMaxJitter()))
	default:
		delay = base
	}
	if delay < 0 {
		delay = 0
	}
	if policy.GetDelayType() == primitive.Retry_DELAY_TYPE_BACKOFF_JITTER {
		jitter := pbDuration(policy.GetMaxJitter())
		if jitter > 0 {
			delay += randomDuration(0, jitter)
		}
	}
	if maxDelay > 0 && delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

func pbDuration(d *durationpb.Duration) time.Duration {
	if d == nil {
		return 0
	}
	return d.AsDuration()
}

func randomDuration(min, max time.Duration) time.Duration {
	if max <= min {
		return min
	}
	return min + time.Duration(rand.Int64N(int64(max-min)))
}

func findNode(dag *primitive.Dag, id string) *primitive.Dag_Node {
	for _, node := range dag.GetNodes() {
		if node.GetId() == id {
			return node
		}
	}
	return nil
}

// FindNodeByExecutionID returns a node from the persisted Dag aggregate by its
// immutable external identity. It traverses embedded sub-Dags as part of the
// same aggregate; dag_ref children are separate persisted aggregates and are
// intentionally not followed here.
func FindNodeByExecutionID(dag *primitive.Dag, executionID string) *primitive.Dag_Node {
	if dag == nil || executionID == "" {
		return nil
	}
	for _, node := range dag.GetNodes() {
		if node.GetExecutionId() == executionID {
			return node
		}
		if sub := node.GetSubDag(); sub != nil {
			if found := FindNodeByExecutionID(sub, executionID); found != nil {
				return found
			}
		}
	}
	return nil
}

func isRunnableStatus(status primitive.Status) bool {
	return status == primitive.Status_STATUS_PENDING || status == primitive.Status_STATUS_RETRY_WAIT
}

func isTerminalStatus(status primitive.Status) bool {
	switch status {
	case primitive.Status_STATUS_COMPLETED, primitive.Status_STATUS_FAILED, primitive.Status_STATUS_SKIPPED, primitive.Status_STATUS_CANCELLED:
		return true
	default:
		return false
	}
}

func allTerminal(dag *primitive.Dag) bool {
	for _, node := range dag.GetNodes() {
		if !isTerminalStatus(node.GetStatus()) {
			return false
		}
	}
	return true
}

func ordinaryNodesTerminal(dag *primitive.Dag) bool {
	for _, node := range dag.GetNodes() {
		if node.GetScheduling().GetAlwaysRun() {
			continue
		}
		if !isTerminalStatus(node.GetStatus()) {
			return false
		}
	}
	return true
}

func skipPendingAlwaysRun(dag *primitive.Dag, now time.Time) {
	for _, node := range dag.GetNodes() {
		if node.GetScheduling().GetAlwaysRun() && isRunnableStatus(node.GetStatus()) {
			markNodeSkipped(node, now)
		}
	}
}

func shouldStopOrdinary(dag *primitive.Dag) bool {
	if dag.GetScheduling().GetOnNodeFailure() != primitive.Dag_Scheduling_ON_NODE_FAILURE_STOP {
		return false
	}
	for _, node := range dag.GetNodes() {
		if node.GetStatus() == primitive.Status_STATUS_FAILED || node.GetStatus() == primitive.Status_STATUS_CANCELLED {
			return true
		}
	}
	return false
}

func retryWaitUntil(node *primitive.Dag_Node) time.Time {
	if ts := node.GetExecution().GetRetryState().GetNextRunAt(); ts != nil {
		return ts.AsTime()
	}
	return time.Time{}
}

func nextRetryAt(dag *primitive.Dag) time.Time {
	var next time.Time
	for _, node := range dag.GetNodes() {
		if node.GetStatus() != primitive.Status_STATUS_RETRY_WAIT {
			continue
		}
		at := retryWaitUntil(node)
		if at.IsZero() {
			return time.Time{}
		}
		if next.IsZero() || at.Before(next) {
			next = at
		}
	}
	return next
}

func edgesSatisfied(dag *primitive.Dag, target *primitive.Dag_Node, predicates PredicateRegistry) bool {
	incoming := incomingEdges(dag, target.GetId())
	if len(incoming) == 0 {
		return true
	}
	policy := target.GetScheduling().GetJoinPolicy()
	if policy == primitive.Dag_Node_Scheduling_JOIN_POLICY_UNSPECIFIED {
		policy = primitive.Dag_Node_Scheduling_JOIN_POLICY_ALL
	}
	satisfied := 0
	for _, edge := range incoming {
		if edgeSatisfied(dag, edge, predicates) {
			satisfied++
		}
	}
	if policy == primitive.Dag_Node_Scheduling_JOIN_POLICY_ANY {
		return satisfied > 0
	}
	return satisfied == len(incoming)
}

func edgeSatisfied(dag *primitive.Dag, edge *primitive.Dag_Edge, predicates PredicateRegistry) bool {
	source := findNode(dag, edge.GetSource())
	if source == nil {
		return false
	}
	if edge.GetPredicateName() != "" {
		if !isTerminalStatus(source.GetStatus()) {
			return false
		}
		if predicates == nil {
			return false
		}
		fn, ok := predicates.GetPredicate(edge.GetPredicateName())
		return ok && fn(&dagContext{Context: context.Background(), dag: dag}, edge)
	}
	if status := edge.GetOnStatus(); status != primitive.Status_STATUS_UNSPECIFIED {
		return source.GetStatus() == status
	}
	return source.GetStatus() == primitive.Status_STATUS_COMPLETED
}

func incomingEdges(dag *primitive.Dag, targetID string) []*primitive.Dag_Edge {
	var edges []*primitive.Dag_Edge
	for _, edge := range dag.GetEdges() {
		if edge.GetTarget() == targetID {
			edges = append(edges, edge)
		}
	}
	return edges
}

func noRunningDependencies(dag *primitive.Dag, target *primitive.Dag_Node) bool {
	for _, edge := range incomingEdges(dag, target.GetId()) {
		source := findNode(dag, edge.GetSource())
		if source != nil && source.GetStatus() == primitive.Status_STATUS_RUNNING {
			return false
		}
	}
	return true
}

func collectRunnableAlwaysRun(dag *primitive.Dag) []*primitive.Dag_Node {
	var nodes []*primitive.Dag_Node
	for _, node := range dag.GetNodes() {
		if node.GetScheduling().GetAlwaysRun() && isRunnableStatus(node.GetStatus()) {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func terminalError(dag *primitive.Dag) error {
	switch dag.GetStatus() {
	case primitive.Status_STATUS_FAILED:
		msg := dag.GetExecution().GetFailure().GetMessage()
		if msg == "" {
			msg = "unknown failure"
		}
		if nodeID := dag.GetExecution().GetFailedNodeId(); nodeID != "" {
			return fmt.Errorf("dag %q failed at node %q: %s", dag.GetId(), nodeID, msg)
		}
		return fmt.Errorf("dag %q failed: %s", dag.GetId(), msg)
	case primitive.Status_STATUS_CANCELLED:
		return fmt.Errorf("dag %q cancelled", dag.GetId())
	default:
		return nil
	}
}
