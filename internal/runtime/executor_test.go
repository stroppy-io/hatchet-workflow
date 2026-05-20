package runtime

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestExecutorRunsNodesInDependencyOrder(t *testing.T) {
	var calls []string
	reg := NewTaskRegistry(
		NewTask("first", func(*emptypb.Empty) (*emptypb.Empty, error) {
			calls = append(calls, "first")
			return &emptypb.Empty{}, nil
		}),
		NewTask("second", func(*emptypb.Empty) (*emptypb.Empty, error) {
			calls = append(calls, "second")
			return &emptypb.Empty{}, nil
		}),
	)
	dag := testDag("dag-order",
		testNode(t, "a", "first", nil),
		testNode(t, "b", "second", nil),
	)
	dag.Edges = []*primitive.Dag_Edge{{
		Id:     "edge-a-b",
		Source: "a",
		Target: "b",
	}}

	err := NewExecutor(reg).Run(context.Background(), dag)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if dag.GetStatus() != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("status = %s, want completed", dag.GetStatus())
	}
	if !reflect.DeepEqual(calls, []string{"first", "second"}) {
		t.Fatalf("calls = %#v", calls)
	}
	if dag.GetNodes()[0].GetExecution().GetRetryState().GetAttempt() != 1 {
		t.Fatalf("first attempt not recorded")
	}
}

func TestExecutorRetryStateAndFailureHistory(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	attempts := 0
	reg := NewTaskRegistry(NewTask("flaky", func(*emptypb.Empty) (*emptypb.Empty, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("temporary failure")
		}
		return &emptypb.Empty{}, nil
	}))
	dag := testDag("dag-retry",
		testNode(t, "flaky", "flaky", &primitive.Retry_Policy{
			Attempts:  2,
			Delay:     durationpb.New(10 * time.Millisecond),
			MaxDelay:  durationpb.New(10 * time.Millisecond),
			DelayType: primitive.Retry_DELAY_TYPE_FIXED,
		}),
	)
	exec := NewExecutor(reg, WithClock(
		func() time.Time { return now },
		func(_ context.Context, d time.Duration) error {
			now = now.Add(d)
			return nil
		},
	))

	err := exec.Run(context.Background(), dag)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	node := dag.GetNodes()[0]
	if node.GetStatus() != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("node status = %s, want completed", node.GetStatus())
	}
	if got := node.GetExecution().GetRetryState().GetAttempt(); got != 2 {
		t.Fatalf("attempt = %d, want 2", got)
	}
	failures := node.GetExecution().GetFailures()
	if len(failures) != 1 {
		t.Fatalf("failures len = %d, want 1", len(failures))
	}
	if !failures[0].GetRetryable() {
		t.Fatalf("first failure should be retryable")
	}
	if failures[0].GetCode() != FailureCodeTaskFailed {
		t.Fatalf("failure code = %q", failures[0].GetCode())
	}
}

func TestExecutorStopPolicyMarksUnreachableNodesSkipped(t *testing.T) {
	reg := NewTaskRegistry(
		NewTask("fail", func(*emptypb.Empty) (*emptypb.Empty, error) {
			return nil, NewFailureError(errors.New("terraform quota exceeded"), &primitive.Dag_Failure{
				Code:   "TERRAFORM_APPLY_FAILED",
				Source: "terraform",
				Phase:  "terraform_apply",
			})
		}),
		NewTask("never", func(*emptypb.Empty) (*emptypb.Empty, error) {
			t.Fatal("dependent node should not run")
			return &emptypb.Empty{}, nil
		}),
	)
	dag := testDag("dag-stop",
		testNode(t, "machines", "fail", nil),
		testNode(t, "configure", "never", nil),
	)
	dag.Edges = []*primitive.Dag_Edge{{
		Id:     "edge-machines-configure",
		Source: "machines",
		Target: "configure",
	}}

	err := NewExecutor(reg).Run(context.Background(), dag)
	if err == nil {
		t.Fatal("Run() expected error")
	}
	if dag.GetStatus() != primitive.Status_STATUS_FAILED {
		t.Fatalf("dag status = %s, want failed", dag.GetStatus())
	}
	if dag.GetExecution().GetFailedNodeId() != "machines" {
		t.Fatalf("failed_node_id = %q", dag.GetExecution().GetFailedNodeId())
	}
	if got := dag.GetExecution().GetFailure().GetCode(); got != "TERRAFORM_APPLY_FAILED" {
		t.Fatalf("dag failure code = %q", got)
	}
	if got := dag.GetNodes()[1].GetStatus(); got != primitive.Status_STATUS_SKIPPED {
		t.Fatalf("dependent status = %s, want skipped", got)
	}
}

func TestExecutorSkipsAlwaysRunOnHappyPath(t *testing.T) {
	reg := NewTaskRegistry(
		NewTask("ok", func(*emptypb.Empty) (*emptypb.Empty, error) {
			return &emptypb.Empty{}, nil
		}),
		NewTask("cleanup", func(*emptypb.Empty) (*emptypb.Empty, error) {
			t.Fatal("always_run cleanup should not run on happy path")
			return &emptypb.Empty{}, nil
		}),
	)
	cleanup := testNode(t, "cleanup", "cleanup", nil)
	cleanup.Scheduling.AlwaysRun = true
	dag := testDag("dag-happy-cleanup",
		testNode(t, "main", "ok", nil),
		cleanup,
	)

	err := NewExecutor(reg).Run(context.Background(), dag)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if dag.GetStatus() != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("dag status = %s, want completed", dag.GetStatus())
	}
	if cleanup.GetStatus() != primitive.Status_STATUS_SKIPPED {
		t.Fatalf("cleanup status = %s, want skipped", cleanup.GetStatus())
	}
}

func TestExecutorPersistsFailedNodeAsRunningDagUntilCleanup(t *testing.T) {
	reg := NewTaskRegistry(
		NewTask("fail", func(*emptypb.Empty) (*emptypb.Empty, error) {
			return nil, errors.New("boom")
		}),
		NewTask("cleanup", func(*emptypb.Empty) (*emptypb.Empty, error) {
			return &emptypb.Empty{}, nil
		}),
	)
	cleanup := testNode(t, "cleanup", "cleanup", nil)
	cleanup.Scheduling.AlwaysRun = true
	dag := testDag("dag-cleanup-recovery",
		testNode(t, "main", "fail", nil),
		cleanup,
	)
	var sawRunningWithFailure bool
	exec := NewExecutor(reg, WithSaveHook(func(_ context.Context, snap *primitive.Dag) error {
		if snap.GetStatus() == primitive.Status_STATUS_RUNNING &&
			snap.GetExecution().GetFailedNodeId() == "main" &&
			findNode(snap, "main").GetStatus() == primitive.Status_STATUS_FAILED {
			sawRunningWithFailure = true
		}
		return nil
	}))

	err := exec.Run(context.Background(), dag)
	if err == nil {
		t.Fatal("Run() expected error")
	}
	if !sawRunningWithFailure {
		t.Fatal("expected persisted RUNNING dag with recorded failed node before cleanup/finalize")
	}
	if cleanup.GetStatus() != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("cleanup status = %s, want completed", cleanup.GetStatus())
	}
	if dag.GetStatus() != primitive.Status_STATUS_FAILED {
		t.Fatalf("dag status = %s, want final failed", dag.GetStatus())
	}
}

func TestNextDelayBackoffGrowsWithoutMaxDelay(t *testing.T) {
	policy := &primitive.Retry_Policy{
		Delay:     durationpb.New(100 * time.Millisecond),
		DelayType: primitive.Retry_DELAY_TYPE_BACKOFF,
	}
	first := nextDelay(policy, nil)
	second := nextDelay(policy, durationpb.New(first))
	third := nextDelay(policy, durationpb.New(second))

	if first != 100*time.Millisecond {
		t.Fatalf("first delay = %s", first)
	}
	if second != 200*time.Millisecond {
		t.Fatalf("second delay = %s", second)
	}
	if third != 400*time.Millisecond {
		t.Fatalf("third delay = %s", third)
	}
}

func TestExecutorNoReadyNonTerminalReturnsTerminalError(t *testing.T) {
	reg := NewTaskRegistry(
		NewTask("ok", func(*emptypb.Empty) (*emptypb.Empty, error) {
			return &emptypb.Empty{}, nil
		}),
	)
	dag := testDag("dag-no-ready",
		testNode(t, "a", "ok", nil),
		testNode(t, "b", "ok", nil),
	)
	dag.Edges = []*primitive.Dag_Edge{{
		Id:        "edge-a-b",
		Source:    "a",
		Target:    "b",
		Condition: &primitive.Dag_Edge_PredicateName{PredicateName: "missing"},
	}}

	err := NewExecutor(reg).Run(context.Background(), dag)
	if err == nil {
		t.Fatal("Run() expected terminal error")
	}
	if dag.GetStatus() != primitive.Status_STATUS_FAILED {
		t.Fatalf("dag status = %s, want failed", dag.GetStatus())
	}
}

func TestExecutorRecoversInterruptedRunningNode(t *testing.T) {
	calls := 0
	reg := NewTaskRegistry(NewTask("recoverable", func(*emptypb.Empty) (*emptypb.Empty, error) {
		calls++
		return &emptypb.Empty{}, nil
	}))
	dag := testDag("dag-recover",
		testNode(t, "apply", "recoverable", &primitive.Retry_Policy{
			Attempts:  1,
			Delay:     durationpb.New(0),
			DelayType: primitive.Retry_DELAY_TYPE_FIXED,
		}),
	)
	node := dag.GetNodes()[0]
	dag.Status = primitive.Status_STATUS_RUNNING
	node.Status = primitive.Status_STATUS_RUNNING
	node.Execution = &primitive.Dag_Node_Execution{
		Status: primitive.Status_STATUS_RUNNING,
		RetryState: &primitive.Retry_State{
			Attempt: 1,
		},
	}

	err := NewExecutor(reg).Run(context.Background(), dag)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want recovered rerun once", calls)
	}
	if dag.GetStatus() != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("dag status = %s, want completed", dag.GetStatus())
	}
	if got := node.GetExecution().GetRetryState().GetAttempt(); got != 2 {
		t.Fatalf("attempt = %d, want interrupted attempt plus rerun", got)
	}
	failures := node.GetExecution().GetFailures()
	if len(failures) != 1 {
		t.Fatalf("failures len = %d, want 1", len(failures))
	}
	if failures[0].GetCode() != FailureCodeNodeInterrupted {
		t.Fatalf("failure code = %q", failures[0].GetCode())
	}
}

func TestRecoverCancellingDagKeepsAlwaysRunRunnable(t *testing.T) {
	reg := NewTaskRegistry(
		NewTask("cleanup", func(*emptypb.Empty) (*emptypb.Empty, error) {
			return &emptypb.Empty{}, nil
		}),
	)
	main := testNode(t, "main", "cleanup", nil)
	cleanup := testNode(t, "cleanup", "cleanup", nil)
	cleanup.Scheduling.AlwaysRun = true
	dag := testDag("dag-cancelling", main, cleanup)
	dag.Status = primitive.Status_STATUS_CANCELLING
	main.Status = primitive.Status_STATUS_RUNNING

	err := NewExecutor(reg).Run(context.Background(), dag)
	if err == nil {
		t.Fatal("Run() expected cancellation terminal error")
	}
	if main.GetStatus() != primitive.Status_STATUS_CANCELLED {
		t.Fatalf("main status = %s, want cancelled", main.GetStatus())
	}
	if cleanup.GetStatus() != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("cleanup status = %s, want completed", cleanup.GetStatus())
	}
	if dag.GetStatus() != primitive.Status_STATUS_CANCELLED {
		t.Fatalf("dag status = %s, want cancelled", dag.GetStatus())
	}
}

func testDag(id string, nodes ...*primitive.Dag_Node) *primitive.Dag {
	return &primitive.Dag{
		Id:     id,
		Status: primitive.Status_STATUS_PENDING,
		Nodes:  nodes,
		Scheduling: &primitive.Dag_Scheduling{
			OnNodeFailure: primitive.Dag_Scheduling_ON_NODE_FAILURE_STOP,
		},
	}
}

func testNode(t *testing.T, id, handler string, retry *primitive.Retry_Policy) *primitive.Dag_Node {
	t.Helper()
	input, err := anypb.New(&emptypb.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if retry == nil {
		retry = &primitive.Retry_Policy{Attempts: 1}
	}
	return &primitive.Dag_Node{
		Id:     id,
		Status: primitive.Status_STATUS_PENDING,
		Scheduling: &primitive.Dag_Node_Scheduling{
			RetryPolicy: retry,
		},
		Variant: &primitive.Dag_Node_TaskState_{
			TaskState: &primitive.Dag_Node_TaskState{
				HandlerName: handler,
				Input:       input,
			},
		},
	}
}

// recorder collects task call names in a goroutine-safe way (runBatch runs nodes concurrently).
type recorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *recorder) add(name string) {
	r.mu.Lock()
	r.calls = append(r.calls, name)
	r.mu.Unlock()
}

func (r *recorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calls))
	copy(out, r.calls)
	return out
}

func okTask(rec *recorder, name string) Task[*anypb.Any, *anypb.Any] {
	return NewTask(name, func(*emptypb.Empty) (*emptypb.Empty, error) {
		if rec != nil {
			rec.add(name)
		}
		return &emptypb.Empty{}, nil
	})
}

func failTask(rec *recorder, name string) Task[*anypb.Any, *anypb.Any] {
	return NewTask(name, func(*emptypb.Empty) (*emptypb.Empty, error) {
		if rec != nil {
			rec.add(name)
		}
		return nil, errors.New("boom")
	})
}

func alwaysRunNode(t *testing.T, id, handler string) *primitive.Dag_Node {
	n := testNode(t, id, handler, nil)
	n.Scheduling.AlwaysRun = true
	return n
}

// --- always_run semantics ----------------------------------------------------

// On full success, always_run cleanup nodes are reserved for teardown only and
// must be SKIPPED, not block the dag (regression for the "no ready nodes" hang).
func TestAlwaysRunSkippedOnSuccess(t *testing.T) {
	rec := &recorder{}
	reg := NewTaskRegistry(okTask(rec, "work"), okTask(rec, "cleanup"))
	dag := testDag("dag-arsuccess",
		testNode(t, "work", "work", nil),
		alwaysRunNode(t, "cleanup", "cleanup"),
	)

	if err := NewExecutor(reg).Run(context.Background(), dag); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if dag.GetStatus() != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("dag status = %s, want completed", dag.GetStatus())
	}
	if got := nodeByID(dag, "cleanup").GetStatus(); got != primitive.Status_STATUS_SKIPPED {
		t.Fatalf("cleanup status = %s, want skipped", got)
	}
	for _, c := range rec.snapshot() {
		if c == "cleanup" {
			t.Fatal("cleanup task ran on success path, want skipped")
		}
	}
}

// Under STOP policy, a failed node triggers always_run teardown.
func TestAlwaysRunExecutedOnFailureStop(t *testing.T) {
	rec := &recorder{}
	reg := NewTaskRegistry(failTask(rec, "work"), okTask(rec, "cleanup"))
	dag := testDag("dag-arstop",
		testNode(t, "work", "work", nil),
		alwaysRunNode(t, "cleanup", "cleanup"),
	)

	err := NewExecutor(reg).Run(context.Background(), dag)
	if err == nil {
		t.Fatal("Run() expected failure")
	}
	if dag.GetStatus() != primitive.Status_STATUS_FAILED {
		t.Fatalf("dag status = %s, want failed", dag.GetStatus())
	}
	if got := nodeByID(dag, "cleanup").GetStatus(); got != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("cleanup status = %s, want completed (teardown must run)", got)
	}
}

// Characterizes CURRENT behavior: under CONTINUE policy a failed node does NOT
// trigger always_run teardown; cleanup is skipped instead. Per dag.proto
// (always_run runs "after failure or cancellation") this is arguably a bug.
func TestAlwaysRunOnFailureContinue_CurrentBehavior(t *testing.T) {
	rec := &recorder{}
	reg := NewTaskRegistry(failTask(rec, "work"), okTask(rec, "cleanup"))
	dag := testDag("dag-arcontinue",
		testNode(t, "work", "work", nil),
		alwaysRunNode(t, "cleanup", "cleanup"),
	)
	dag.Scheduling.OnNodeFailure = primitive.Dag_Scheduling_ON_NODE_FAILURE_CONTINUE

	err := NewExecutor(reg).Run(context.Background(), dag)
	if err == nil {
		t.Fatal("Run() expected failure")
	}
	if dag.GetStatus() != primitive.Status_STATUS_FAILED {
		t.Fatalf("dag status = %s, want failed", dag.GetStatus())
	}
	// BUG MARKER: cleanup is skipped under CONTINUE even though a node failed.
	if got := nodeByID(dag, "cleanup").GetStatus(); got != primitive.Status_STATUS_SKIPPED {
		t.Fatalf("cleanup status = %s; current behavior is skipped", got)
	}
}

// --- join policies -----------------------------------------------------------

// JOIN_POLICY_ANY: target runs once any single incoming edge is satisfied.
func TestJoinPolicyAny(t *testing.T) {
	rec := &recorder{}
	reg := NewTaskRegistry(okTask(rec, "a"), failTask(rec, "b"), okTask(rec, "join"))
	a := testNode(t, "a", "a", nil)
	b := testNode(t, "b", "b", nil)
	join := testNode(t, "join", "join", nil)
	join.Scheduling.JoinPolicy = primitive.Dag_Node_Scheduling_JOIN_POLICY_ANY
	dag := testDag("dag-any", a, b, join)
	dag.Scheduling.OnNodeFailure = primitive.Dag_Scheduling_ON_NODE_FAILURE_CONTINUE
	dag.Edges = []*primitive.Dag_Edge{
		{Id: "a-join", Source: "a", Target: "join"},
		{Id: "b-join", Source: "b", Target: "join"},
	}

	_ = NewExecutor(reg).Run(context.Background(), dag)
	if got := nodeByID(dag, "join").GetStatus(); got != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("join status = %s, want completed (ANY satisfied by a)", got)
	}
	if dag.GetStatus() != primitive.Status_STATUS_FAILED {
		t.Fatalf("dag status = %s, want failed (b failed under continue)", dag.GetStatus())
	}
}

// JOIN_POLICY_ALL (default): one failed dependency leaves the target unrunnable,
// it ends up SKIPPED under STOP policy.
func TestJoinPolicyAllSkipsOnFailedDependency(t *testing.T) {
	reg := NewTaskRegistry(
		okTask(nil, "a"),
		failTask(nil, "b"),
		NewTask("join", func(*emptypb.Empty) (*emptypb.Empty, error) {
			t.Fatal("join must not run when an ALL dependency failed")
			return &emptypb.Empty{}, nil
		}),
	)
	join := testNode(t, "join", "join", nil)
	dag := testDag("dag-all", testNode(t, "a", "a", nil), testNode(t, "b", "b", nil), join)
	dag.Edges = []*primitive.Dag_Edge{
		{Id: "a-join", Source: "a", Target: "join"},
		{Id: "b-join", Source: "b", Target: "join"},
	}

	_ = NewExecutor(reg).Run(context.Background(), dag)
	if got := nodeByID(dag, "join").GetStatus(); got != primitive.Status_STATUS_SKIPPED {
		t.Fatalf("join status = %s, want skipped", got)
	}
}

// --- predicate edges ---------------------------------------------------------

func TestPredicateEdgeAllowsAndBlocks(t *testing.T) {
	run := func(t *testing.T, pred bool) *primitive.Dag {
		rec := &recorder{}
		reg := NewTaskRegistry(okTask(rec, "a"), okTask(rec, "b"))
		a := testNode(t, "a", "a", nil)
		b := testNode(t, "b", "b", nil)
		dag := testDag("dag-pred", a, b)
		dag.Edges = []*primitive.Dag_Edge{{
			Id:        "a-b",
			Source:    "a",
			Target:    "b",
			Condition: &primitive.Dag_Edge_PredicateName{PredicateName: "p"},
		}}
		preds := PredicateRegistryMap{"p": func(*primitive.Dag, *primitive.Dag_Edge) bool { return pred }}
		_ = NewExecutor(reg, WithPredicates(preds)).Run(context.Background(), dag)
		return dag
	}

	t.Run("true_runs_target", func(t *testing.T) {
		dag := run(t, true)
		if got := nodeByID(dag, "b").GetStatus(); got != primitive.Status_STATUS_COMPLETED {
			t.Fatalf("b status = %s, want completed", got)
		}
		if dag.GetStatus() != primitive.Status_STATUS_COMPLETED {
			t.Fatalf("dag status = %s, want completed", dag.GetStatus())
		}
	})

	// Characterizes CURRENT behavior: a target whose only edge predicate is false
	// can never become ready; the dag fails with "no ready nodes" instead of
	// skipping the unreachable node.
	t.Run("false_fails_dag_current_behavior", func(t *testing.T) {
		dag := run(t, false)
		if dag.GetStatus() != primitive.Status_STATUS_FAILED {
			t.Fatalf("dag status = %s; current behavior is failed", dag.GetStatus())
		}
	})
}

// --- priority + parallelism --------------------------------------------------

func TestMaxParallelismRunsByPriority(t *testing.T) {
	rec := &recorder{}
	reg := NewTaskRegistry(okTask(rec, "low"), okTask(rec, "high"), okTask(rec, "mid"))
	low := testNode(t, "low", "low", nil)
	low.Scheduling.Priority = 1
	high := testNode(t, "high", "high", nil)
	high.Scheduling.Priority = 5
	mid := testNode(t, "mid", "mid", nil)
	mid.Scheduling.Priority = 3
	dag := testDag("dag-prio", low, high, mid)
	dag.Scheduling.MaxParallelism = 1

	if err := NewExecutor(reg).Run(context.Background(), dag); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got := rec.snapshot()
	want := []string{"high", "mid", "low"}
	if len(got) != len(want) {
		t.Fatalf("calls = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("calls = %#v, want %#v", got, want)
		}
	}
}

// --- retry exhaustion --------------------------------------------------------

func TestRetryExhaustionFailsDag(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	reg := NewTaskRegistry(failTask(nil, "work"))
	dag := testDag("dag-exhaust", testNode(t, "work", "work", &primitive.Retry_Policy{
		Attempts:  2,
		Delay:     durationpb.New(10 * time.Millisecond),
		DelayType: primitive.Retry_DELAY_TYPE_FIXED,
	}))
	exec := NewExecutor(reg, WithClock(
		func() time.Time { return now },
		func(_ context.Context, d time.Duration) error { now = now.Add(d); return nil },
	))

	if err := exec.Run(context.Background(), dag); err == nil {
		t.Fatal("Run() expected failure after exhausting retries")
	}
	node := nodeByID(dag, "work")
	if node.GetStatus() != primitive.Status_STATUS_FAILED {
		t.Fatalf("node status = %s, want failed", node.GetStatus())
	}
	if got := node.GetExecution().GetRetryState().GetAttempt(); got != 2 {
		t.Fatalf("attempt = %d, want 2", got)
	}
	failures := node.GetExecution().GetFailures()
	if len(failures) != 2 {
		t.Fatalf("failures = %d, want 2", len(failures))
	}
	if failures[1].GetRetryable() {
		t.Fatal("last failure should be non-retryable")
	}
	if dag.GetExecution().GetFailedNodeId() != "work" {
		t.Fatalf("failed_node_id = %q", dag.GetExecution().GetFailedNodeId())
	}
}

// --- sub-dag -----------------------------------------------------------------

func subDagNode(id string, sub *primitive.Dag) *primitive.Dag_Node {
	return &primitive.Dag_Node{
		Id:         id,
		Status:     primitive.Status_STATUS_PENDING,
		Scheduling: &primitive.Dag_Node_Scheduling{RetryPolicy: &primitive.Retry_Policy{Attempts: 1}},
		Variant:    &primitive.Dag_Node_SubDag{SubDag: sub},
	}
}

func TestSubDagSuccess(t *testing.T) {
	rec := &recorder{}
	reg := NewTaskRegistry(okTask(rec, "inner"))
	inner := testDag("inner", testNode(t, "in", "inner", nil))
	outer := testDag("outer", subDagNode("sub", inner))

	if err := NewExecutor(reg).Run(context.Background(), outer); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if outer.GetStatus() != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("outer status = %s, want completed", outer.GetStatus())
	}
	subNode := nodeByID(outer, "sub")
	if subNode.GetStatus() != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("sub node status = %s, want completed", subNode.GetStatus())
	}
	if got := subNode.GetSubDag().GetStatus(); got != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("inner dag status = %s, want completed (state must be written back)", got)
	}
}

func TestSubDagFailurePropagates(t *testing.T) {
	reg := NewTaskRegistry(failTask(nil, "inner"))
	inner := testDag("inner", testNode(t, "in", "inner", nil))
	outer := testDag("outer", subDagNode("sub", inner))

	err := NewExecutor(reg).Run(context.Background(), outer)
	if err == nil {
		t.Fatal("Run() expected sub-dag failure")
	}
	if outer.GetStatus() != primitive.Status_STATUS_FAILED {
		t.Fatalf("outer status = %s, want failed", outer.GetStatus())
	}
	subNode := nodeByID(outer, "sub")
	if subNode.GetStatus() != primitive.Status_STATUS_FAILED {
		t.Fatalf("sub node status = %s, want failed", subNode.GetStatus())
	}
	if got := subNode.GetExecution().GetFailure().GetCode(); got != FailureCodeSubDagFailed {
		t.Fatalf("failure code = %q, want %q", got, FailureCodeSubDagFailed)
	}
	// #11 regression: the failed inner dag state is written back onto the node.
	if got := subNode.GetSubDag().GetStatus(); got != primitive.Status_STATUS_FAILED {
		t.Fatalf("inner dag status = %s, want failed", got)
	}
}

// --- dag-ref -----------------------------------------------------------------

type dagRefRunnerFunc func(context.Context, *primitive.Dag_Node_DagRef) (*primitive.Dag, error)

func (f dagRefRunnerFunc) RunDagRef(ctx context.Context, ref *primitive.Dag_Node_DagRef) (*primitive.Dag, error) {
	return f(ctx, ref)
}

func dagRefNode(id, dagID string) *primitive.Dag_Node {
	return &primitive.Dag_Node{
		Id:         id,
		Status:     primitive.Status_STATUS_PENDING,
		Scheduling: &primitive.Dag_Node_Scheduling{RetryPolicy: &primitive.Retry_Policy{Attempts: 1}},
		Variant: &primitive.Dag_Node_DagRef_{
			DagRef: &primitive.Dag_Node_DagRef{DagId: dagID},
		},
	}
}

func TestDagRefSuccess(t *testing.T) {
	outer := testDag("outer-ref", dagRefNode("child", "child-dag"))
	exec := NewExecutor(NewTaskRegistry(), WithDagRefRunner(dagRefRunnerFunc(func(_ context.Context, ref *primitive.Dag_Node_DagRef) (*primitive.Dag, error) {
		if ref.GetDagId() != "child-dag" {
			t.Fatalf("dag ref id = %q, want child-dag", ref.GetDagId())
		}
		return &primitive.Dag{Id: ref.GetDagId(), Status: primitive.Status_STATUS_COMPLETED}, nil
	})))

	if err := exec.Run(context.Background(), outer); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := nodeByID(outer, "child").GetStatus(); got != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("dag_ref node status = %s, want completed", got)
	}
}

func TestDagRefRequiresRunner(t *testing.T) {
	outer := testDag("outer-ref-missing-runner", dagRefNode("child", "child-dag"))
	err := NewExecutor(NewTaskRegistry()).Run(context.Background(), outer)
	if err == nil {
		t.Fatal("Run() expected dag_ref runner error")
	}
	node := nodeByID(outer, "child")
	if got := node.GetExecution().GetFailure().GetCode(); got != FailureCodeDagRefRunnerMissing {
		t.Fatalf("failure code = %q, want %q", got, FailureCodeDagRefRunnerMissing)
	}
}

// --- cancellation ------------------------------------------------------------

func TestRunCancelsOnContextCancel(t *testing.T) {
	reg := NewTaskRegistry(NewTask("work", func(*emptypb.Empty) (*emptypb.Empty, error) {
		t.Fatal("task must not run when context already cancelled")
		return &emptypb.Empty{}, nil
	}))
	dag := testDag("dag-cancel", testNode(t, "work", "work", nil))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := NewExecutor(reg).Run(ctx, dag)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	if dag.GetStatus() != primitive.Status_STATUS_CANCELLED {
		t.Fatalf("dag status = %s, want cancelled", dag.GetStatus())
	}
	if got := nodeByID(dag, "work").GetStatus(); got != primitive.Status_STATUS_CANCELLED {
		t.Fatalf("node status = %s, want cancelled", got)
	}
}

// --- nextDelay unit ----------------------------------------------------------

func TestNextDelayBackoffGrows(t *testing.T) {
	policy := &primitive.Retry_Policy{
		Delay:     durationpb.New(100 * time.Millisecond),
		DelayType: primitive.Retry_DELAY_TYPE_BACKOFF,
	}
	d0 := nextDelay(policy, nil)
	d1 := nextDelay(policy, durationpb.New(d0))
	d2 := nextDelay(policy, durationpb.New(d1))
	if d0 != 100*time.Millisecond || d1 != 200*time.Millisecond || d2 != 400*time.Millisecond {
		t.Fatalf("backoff = %v,%v,%v want 100ms,200ms,400ms", d0, d1, d2)
	}
}

func TestNextDelayBackoffCappedByMaxDelay(t *testing.T) {
	policy := &primitive.Retry_Policy{
		Delay:     durationpb.New(100 * time.Millisecond),
		MaxDelay:  durationpb.New(150 * time.Millisecond),
		DelayType: primitive.Retry_DELAY_TYPE_BACKOFF,
	}
	if got := nextDelay(policy, durationpb.New(100*time.Millisecond)); got != 150*time.Millisecond {
		t.Fatalf("capped delay = %v, want 150ms", got)
	}
}

func TestNextDelayFixed(t *testing.T) {
	policy := &primitive.Retry_Policy{
		Delay:     durationpb.New(100 * time.Millisecond),
		DelayType: primitive.Retry_DELAY_TYPE_FIXED,
	}
	if got := nextDelay(policy, durationpb.New(5*time.Second)); got != 100*time.Millisecond {
		t.Fatalf("fixed delay = %v, want 100ms", got)
	}
}

func TestNextDelayRandomWithinBounds(t *testing.T) {
	policy := &primitive.Retry_Policy{
		Delay:     durationpb.New(100 * time.Millisecond),
		MaxDelay:  durationpb.New(200 * time.Millisecond),
		DelayType: primitive.Retry_DELAY_TYPE_RANDOM,
	}
	for range 50 {
		got := nextDelay(policy, nil)
		if got < 100*time.Millisecond || got > 200*time.Millisecond {
			t.Fatalf("random delay = %v, want within [100ms,200ms]", got)
		}
	}
}

func TestFailureErrorWrapsAndUnwraps(t *testing.T) {
	base := errors.New("underlying")
	fe := NewFailureError(base, &primitive.Dag_Failure{Code: "MY_CODE"})
	if fe.Error() != "underlying" {
		t.Fatalf("Error() = %q, want %q", fe.Error(), "underlying")
	}
	if !errors.Is(fe, base) {
		t.Fatal("errors.Is should unwrap to the base error")
	}
	var target *FailureError
	if !errors.As(fe, &target) {
		t.Fatal("errors.As should match *FailureError")
	}
	if got := fe.Failure().GetCode(); got != "MY_CODE" {
		t.Fatalf("Failure().Code = %q, want MY_CODE", got)
	}

	var nilFE *FailureError
	if nilFE.Error() != "" || nilFE.Unwrap() != nil || nilFE.Failure() != nil {
		t.Fatal("nil *FailureError methods must be safe and zero-valued")
	}
}

func TestTaskFnCallAdapter(t *testing.T) {
	var fn TaskFn[*emptypb.Empty, *emptypb.Empty] = func(in *emptypb.Empty) (*emptypb.Empty, error) {
		return in, nil
	}
	if _, err := fn.Call(&emptypb.Empty{}); err != nil {
		t.Fatalf("TaskFn.Call() error = %v", err)
	}
}

func nodeByID(dag *primitive.Dag, id string) *primitive.Dag_Node {
	return findNode(dag, id)
}

func TestDagRefRunnerErrorFails(t *testing.T) {
	outer := testDag("outer-ref-err", dagRefNode("child", "child-dag"))
	exec := NewExecutor(NewTaskRegistry(), WithDagRefRunner(dagRefRunnerFunc(func(context.Context, *primitive.Dag_Node_DagRef) (*primitive.Dag, error) {
		return nil, errors.New("backend down")
	})))

	if err := exec.Run(context.Background(), outer); err == nil {
		t.Fatal("Run() expected failure when runner errors")
	}
	if got := nodeByID(outer, "child").GetExecution().GetFailure().GetCode(); got != FailureCodeDagRefFailed {
		t.Fatalf("failure code = %q, want %q", got, FailureCodeDagRefFailed)
	}
}

func TestDagRefChildFailedFails(t *testing.T) {
	outer := testDag("outer-ref-childfail", dagRefNode("child", "child-dag"))
	exec := NewExecutor(NewTaskRegistry(), WithDagRefRunner(dagRefRunnerFunc(func(_ context.Context, ref *primitive.Dag_Node_DagRef) (*primitive.Dag, error) {
		return &primitive.Dag{
			Id:        ref.GetDagId(),
			Status:    primitive.Status_STATUS_FAILED,
			Execution: &primitive.Dag_Execution{Status: primitive.Status_STATUS_FAILED, Failure: &primitive.Dag_Failure{Message: "child boom"}},
		}, nil
	})))

	if err := exec.Run(context.Background(), outer); err == nil {
		t.Fatal("Run() expected failure when child dag failed")
	}
	node := nodeByID(outer, "child")
	if node.GetStatus() != primitive.Status_STATUS_FAILED {
		t.Fatalf("dag_ref node status = %s, want failed", node.GetStatus())
	}
	if got := node.GetExecution().GetFailure().GetCode(); got != FailureCodeDagRefFailed {
		t.Fatalf("failure code = %q, want %q", got, FailureCodeDagRefFailed)
	}
}

// canRetry: until_succeeded with attempts==0 retries indefinitely until success.
func TestRetryUntilSucceeded(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	attempts := 0
	reg := NewTaskRegistry(NewTask("flaky", func(*emptypb.Empty) (*emptypb.Empty, error) {
		attempts++
		if attempts < 3 {
			return nil, errors.New("not yet")
		}
		return &emptypb.Empty{}, nil
	}))
	dag := testDag("dag-until", testNode(t, "flaky", "flaky", &primitive.Retry_Policy{
		Attempts:       0,
		UntilSucceeded: true,
		Delay:          durationpb.New(time.Millisecond),
		DelayType:      primitive.Retry_DELAY_TYPE_FIXED,
	}))
	exec := NewExecutor(reg, WithClock(
		func() time.Time { return now },
		func(_ context.Context, d time.Duration) error { now = now.Add(d); return nil },
	))

	if err := exec.Run(context.Background(), dag); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if dag.GetStatus() != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("dag status = %s, want completed", dag.GetStatus())
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}
