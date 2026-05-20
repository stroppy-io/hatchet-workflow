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
func TestAlwaysRunOnFailureContinue_RunsTeardown(t *testing.T) {
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
	// always_run teardown must run after failure regardless of CONTINUE policy.
	if got := nodeByID(dag, "cleanup").GetStatus(); got != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("cleanup status = %s, want completed (teardown must run)", got)
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
	// retryable is the opinion (a plain error is transient), recorded before
	// policy limits. The node still FAILED because the attempt budget ran out.
	if !failures[1].GetRetryable() {
		t.Fatal("plain-error failure opinion should stay retryable even on the last attempt")
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

func TestRunAssignsStableExecutionIDsAcrossEmbeddedDag(t *testing.T) {
	rec := &recorder{}
	reg := NewTaskRegistry(okTask(rec, "inner"))
	inner := testDag("inner", testNode(t, "in", "inner", nil))
	outer := testDag("outer", subDagNode("sub", inner))

	if err := NewExecutor(reg).Run(context.Background(), outer); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	sub := nodeByID(outer, "sub")
	if got := sub.GetExecutionId(); got != "outer.sub" {
		t.Fatalf("sub execution_id = %q, want outer.sub", got)
	}
	innerNode := nodeByID(sub.GetSubDag(), "in")
	if got := innerNode.GetExecutionId(); got != "outer.sub.in" {
		t.Fatalf("inner execution_id = %q, want outer.sub.in", got)
	}
	if got := FindNodeByExecutionID(outer, "outer.sub.in"); got != innerNode {
		t.Fatalf("FindNodeByExecutionID returned %p, want %p", got, innerNode)
	}
}

func TestRunPreservesExistingExecutionID(t *testing.T) {
	rec := &recorder{}
	reg := NewTaskRegistry(okTask(rec, "work"))
	node := testNode(t, "work", "work", nil)
	node.ExecutionId = "command-01"
	dag := testDag("dag", node)

	if err := NewExecutor(reg).Run(context.Background(), dag); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := node.GetExecutionId(); got != "command-01" {
		t.Fatalf("execution_id = %q, want command-01", got)
	}
}

func TestValidateDagRejectsDuplicateExecutionIDAcrossEmbeddedDag(t *testing.T) {
	inner := testDag("inner", testNode(t, "in", "noop", nil))
	inner.GetNodes()[0].ExecutionId = "dup"
	outer := testDag("outer", subDagNode("sub", inner), testNode(t, "other", "noop", nil))
	outer.GetNodes()[0].ExecutionId = "sub"
	outer.GetNodes()[1].ExecutionId = "dup"

	if err := ValidateDag(outer); err == nil {
		t.Fatal("ValidateDag() expected duplicate execution_id error")
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
	// retry-go RandomDelay draws uniformly from [0, max_jitter).
	policy := &primitive.Retry_Policy{
		Delay:     durationpb.New(100 * time.Millisecond),
		MaxJitter: durationpb.New(200 * time.Millisecond),
		DelayType: primitive.Retry_DELAY_TYPE_RANDOM,
	}
	sawNonZero := false
	for range 200 {
		got := nextDelay(policy, nil)
		if got < 0 || got >= 200*time.Millisecond {
			t.Fatalf("random delay = %v, want within [0,200ms)", got)
		}
		if got > 0 {
			sawNonZero = true
		}
	}
	if !sawNonZero {
		t.Fatal("expected at least one non-zero random delay")
	}
}

func TestNextDelayRandomZeroJitter(t *testing.T) {
	policy := &primitive.Retry_Policy{
		Delay:     durationpb.New(100 * time.Millisecond),
		DelayType: primitive.Retry_DELAY_TYPE_RANDOM,
	}
	if got := nextDelay(policy, nil); got != 0 {
		t.Fatalf("random delay without max_jitter = %v, want 0", got)
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

// --- retryable opinion -------------------------------------------------------

func TestNonRetryableOpinionStopsRetries(t *testing.T) {
	calls := 0
	reg := NewTaskRegistry(NewTask("auth", func(*emptypb.Empty) (*emptypb.Empty, error) {
		calls++
		return nil, NewFailureError(errors.New("unauthorized"), &primitive.Dag_Failure{
			Code:      "AUTH_FAILED",
			Retryable: false,
		})
	}))
	dag := testDag("dag-nonretry", testNode(t, "auth", "auth", &primitive.Retry_Policy{
		Attempts:  5,
		Delay:     durationpb.New(time.Millisecond),
		DelayType: primitive.Retry_DELAY_TYPE_FIXED,
	}))

	if err := NewExecutor(reg).Run(context.Background(), dag); err == nil {
		t.Fatal("Run() expected failure")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (non-retryable opinion must not retry)", calls)
	}
	node := nodeByID(dag, "auth")
	if node.GetStatus() != primitive.Status_STATUS_FAILED {
		t.Fatalf("node status = %s, want failed", node.GetStatus())
	}
	if node.GetExecution().GetFailure().GetRetryable() {
		t.Fatal("failure.retryable should be false (task opinion)")
	}
}

func TestRetryableOpinionFailureErrorRetries(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	calls := 0
	reg := NewTaskRegistry(NewTask("flaky", func(*emptypb.Empty) (*emptypb.Empty, error) {
		calls++
		if calls == 1 {
			return nil, NewFailureError(errors.New("transient"), &primitive.Dag_Failure{
				Code:      "TRANSIENT",
				Retryable: true,
			})
		}
		return &emptypb.Empty{}, nil
	}))
	dag := testDag("dag-retryop", testNode(t, "flaky", "flaky", &primitive.Retry_Policy{
		Attempts:  2,
		Delay:     durationpb.New(time.Millisecond),
		DelayType: primitive.Retry_DELAY_TYPE_FIXED,
	}))
	exec := NewExecutor(reg, WithClock(
		func() time.Time { return now },
		func(_ context.Context, d time.Duration) error { now = now.Add(d); return nil },
	))

	if err := exec.Run(context.Background(), dag); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if got := nodeByID(dag, "flaky").GetStatus(); got != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("node status = %s, want completed", got)
	}
}

// --- last_error_only ---------------------------------------------------------

func TestLastErrorOnlyKeepsSingleFailure(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	reg := NewTaskRegistry(failTask(nil, "work"))
	dag := testDag("dag-lasterr", testNode(t, "work", "work", &primitive.Retry_Policy{
		Attempts:      3,
		Delay:         durationpb.New(time.Millisecond),
		DelayType:     primitive.Retry_DELAY_TYPE_FIXED,
		LastErrorOnly: true,
	}))
	exec := NewExecutor(reg, WithClock(
		func() time.Time { return now },
		func(_ context.Context, d time.Duration) error { now = now.Add(d); return nil },
	))

	if err := exec.Run(context.Background(), dag); err == nil {
		t.Fatal("Run() expected failure")
	}
	node := nodeByID(dag, "work")
	if got := node.GetExecution().GetRetryState().GetAttempt(); got != 3 {
		t.Fatalf("attempt = %d, want 3", got)
	}
	if got := len(node.GetExecution().GetFailures()); got != 1 {
		t.Fatalf("failures = %d, want 1 (last_error_only)", got)
	}
}

// --- backoff overflow guard --------------------------------------------------

func TestNextDelayBackoffOverflowGuard(t *testing.T) {
	policy := &primitive.Retry_Policy{
		Delay:     durationpb.New(time.Second),
		DelayType: primitive.Retry_DELAY_TYPE_BACKOFF,
	}
	prev := maxBackoffDuration + time.Hour
	if got := nextDelay(policy, durationpb.New(prev)); got != prev {
		t.Fatalf("delay = %v, want held at previous %v (no overflow)", got, prev)
	}
}

// --- dag_ref terminal mirroring ----------------------------------------------

func TestDagRefCancelledChildCancelsNode(t *testing.T) {
	runner := dagRefRunnerFunc(func(_ context.Context, ref *primitive.Dag_Node_DagRef) (*primitive.Dag, error) {
		return &primitive.Dag{Id: ref.GetDagId(), Status: primitive.Status_STATUS_CANCELLED}, nil
	})
	dag := testDag("dag-ref-cancel", dagRefNode("child", "child-dag"))

	err := NewExecutor(NewTaskRegistry(), WithDagRefRunner(runner)).Run(context.Background(), dag)
	if err == nil {
		t.Fatal("Run() expected cancellation terminal error")
	}
	if got := nodeByID(dag, "child").GetStatus(); got != primitive.Status_STATUS_CANCELLED {
		t.Fatalf("node status = %s, want cancelled (mirror child)", got)
	}
	if dag.GetStatus() != primitive.Status_STATUS_CANCELLED {
		t.Fatalf("dag status = %s, want cancelled", dag.GetStatus())
	}
}

func TestDagRefNilChildFails(t *testing.T) {
	runner := dagRefRunnerFunc(func(context.Context, *primitive.Dag_Node_DagRef) (*primitive.Dag, error) {
		return nil, nil
	})
	dag := testDag("dag-ref-nil", dagRefNode("child", "child-dag"))

	err := NewExecutor(NewTaskRegistry(), WithDagRefRunner(runner)).Run(context.Background(), dag)
	if err == nil {
		t.Fatal("Run() expected failure on nil child")
	}
	failure := nodeByID(dag, "child").GetExecution().GetFailure()
	if failure.GetCode() != FailureCodeDagRefFailed {
		t.Fatalf("failure code = %q, want %q", failure.GetCode(), FailureCodeDagRefFailed)
	}
	if failure.GetRetryable() {
		t.Fatal("nil child failure should be non-retryable")
	}
}

func TestDagRefPendingChildPollsUntilTerminal(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	polls := 0
	runner := dagRefRunnerFunc(func(_ context.Context, ref *primitive.Dag_Node_DagRef) (*primitive.Dag, error) {
		polls++
		if polls < 3 {
			return &primitive.Dag{Id: ref.GetDagId(), Status: primitive.Status_STATUS_RUNNING}, nil
		}
		return &primitive.Dag{Id: ref.GetDagId(), Status: primitive.Status_STATUS_COMPLETED}, nil
	})
	node := dagRefNode("child", "child-dag")
	node.Scheduling.RetryPolicy = &primitive.Retry_Policy{
		Attempts:       0,
		UntilSucceeded: true,
		Delay:          durationpb.New(time.Millisecond),
		DelayType:      primitive.Retry_DELAY_TYPE_FIXED,
	}
	dag := testDag("dag-ref-pending", node)
	exec := NewExecutor(NewTaskRegistry(), WithDagRefRunner(runner), WithClock(
		func() time.Time { return now },
		func(_ context.Context, d time.Duration) error { now = now.Add(d); return nil },
	))

	if err := exec.Run(context.Background(), dag); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if polls != 3 {
		t.Fatalf("polls = %d, want 3 (poll until terminal)", polls)
	}
	if got := nodeByID(dag, "child").GetStatus(); got != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("node status = %s, want completed", got)
	}
}

func TestDagRefPendingExhaustsAttempts(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	runner := dagRefRunnerFunc(func(_ context.Context, ref *primitive.Dag_Node_DagRef) (*primitive.Dag, error) {
		return &primitive.Dag{Id: ref.GetDagId(), Status: primitive.Status_STATUS_RUNNING}, nil
	})
	node := dagRefNode("child", "child-dag")
	node.Scheduling.RetryPolicy = &primitive.Retry_Policy{
		Attempts:  2,
		Delay:     durationpb.New(time.Millisecond),
		DelayType: primitive.Retry_DELAY_TYPE_FIXED,
	}
	dag := testDag("dag-ref-pending-exhaust", node)
	exec := NewExecutor(NewTaskRegistry(), WithDagRefRunner(runner), WithClock(
		func() time.Time { return now },
		func(_ context.Context, d time.Duration) error { now = now.Add(d); return nil },
	))

	if err := exec.Run(context.Background(), dag); err == nil {
		t.Fatal("Run() expected failure after pending attempts exhausted")
	}
	if got := nodeByID(dag, "child").GetExecution().GetFailure().GetCode(); got != FailureCodeDagRefPending {
		t.Fatalf("failure code = %q, want %q", got, FailureCodeDagRefPending)
	}
}

// --- sub_dag cancelled mirroring ---------------------------------------------

func TestSubDagCancelledCancelsNode(t *testing.T) {
	reg := NewTaskRegistry(okTask(nil, "inner"))
	inner := testDag("inner", testNode(t, "in", "inner", nil))
	inner.Status = primitive.Status_STATUS_CANCELLING
	outer := testDag("outer-subcancel", subDagNode("sub", inner))

	err := NewExecutor(reg).Run(context.Background(), outer)
	if err == nil {
		t.Fatal("Run() expected cancellation terminal error")
	}
	if got := nodeByID(outer, "sub").GetStatus(); got != primitive.Status_STATUS_CANCELLED {
		t.Fatalf("sub node status = %s, want cancelled", got)
	}
	if outer.GetStatus() != primitive.Status_STATUS_CANCELLED {
		t.Fatalf("outer status = %s, want cancelled", outer.GetStatus())
	}
}

// --- execution_id + is_sub_dag normalization ---------------------------------

func TestNormalizeAssignsExecutionIDsAndSubDagFlag(t *testing.T) {
	inner := testDag("inner", testNode(t, "in", "x", nil))
	outer := testDag("outer-exec", subDagNode("sub", inner), testNode(t, "task", "x", nil))

	normalizeDag(outer)

	sub := outer.GetNodes()[0]
	if sub.GetExecutionId() != "outer-exec.sub" {
		t.Fatalf("sub execution_id = %q, want outer-exec.sub", sub.GetExecutionId())
	}
	if outer.GetNodes()[1].GetExecutionId() != "outer-exec.task" {
		t.Fatalf("task execution_id = %q, want outer-exec.task", outer.GetNodes()[1].GetExecutionId())
	}
	embedded := sub.GetSubDag()
	if embedded.GetNodes()[0].GetExecutionId() != "outer-exec.sub.in" {
		t.Fatalf("inner node execution_id = %q, want outer-exec.sub.in", embedded.GetNodes()[0].GetExecutionId())
	}
	if !embedded.GetScheduling().GetIsSubDag() {
		t.Fatal("embedded sub dag should be marked is_sub_dag")
	}
	if outer.GetScheduling().GetIsSubDag() {
		t.Fatal("top-level dag must not be marked is_sub_dag")
	}
}

func TestValidateRejectsDuplicateExecutionID(t *testing.T) {
	a := testNode(t, "a", "x", nil)
	b := testNode(t, "b", "x", nil)
	a.ExecutionId = "dup"
	b.ExecutionId = "dup"
	dag := testDag("dag-dupexec", a, b)

	if err := ValidateDag(dag); err == nil {
		t.Fatal("ValidateDag expected duplicate execution_id error")
	}
}

func TestFindNodeByExecutionID(t *testing.T) {
	inner := testDag("inner", testNode(t, "in", "x", nil))
	outer := testDag("outer-find", subDagNode("sub", inner))
	normalizeDag(outer)

	found := FindNodeByExecutionID(outer, "outer-find.sub.in")
	if found == nil || found.GetId() != "in" {
		t.Fatalf("FindNodeByExecutionID = %v, want inner node 'in'", found)
	}
	if FindNodeByExecutionID(outer, "missing") != nil {
		t.Fatal("expected nil for missing execution_id")
	}
}

// --- validation edge cases ---------------------------------------------------

func TestValidateDagRejectsMissingExecutionID(t *testing.T) {
	// testNode leaves execution_id empty; ValidateDag (without normalize) rejects.
	dag := testDag("vexec", testNode(t, "a", "x", nil))
	if err := ValidateDag(dag); err == nil {
		t.Fatal("expected execution_id required error")
	}
}

func TestValidateDagRejectsNilEmptyAndNoNodes(t *testing.T) {
	if err := ValidateDag(nil); err == nil {
		t.Fatal("nil dag must error")
	}
	if err := ValidateDag(&primitive.Dag{}); err == nil {
		t.Fatal("empty dag id must error")
	}
	if err := ValidateDag(&primitive.Dag{Id: "x"}); err == nil {
		t.Fatal("dag without nodes must error")
	}
}

func TestValidateDagRejectsDuplicateNodeID(t *testing.T) {
	a := testNode(t, "dup", "x", nil)
	a.ExecutionId = "e1"
	b := testNode(t, "dup", "x", nil)
	b.ExecutionId = "e2"
	dag := testDag("vdup", a, b)
	if err := ValidateDag(dag); err == nil {
		t.Fatal("duplicate node id must error")
	}
}

func TestValidateDagRejectsUnknownEdgeAndCycle(t *testing.T) {
	a := testNode(t, "a", "x", nil)
	a.ExecutionId = "ea"
	bad := testDag("vedge", a)
	bad.Edges = []*primitive.Dag_Edge{{Id: "e", Source: "a", Target: "ghost"}}
	if err := ValidateDag(bad); err == nil {
		t.Fatal("unknown edge target must error")
	}
	bad.Edges = []*primitive.Dag_Edge{{Id: "e", Source: "ghost", Target: "a"}}
	if err := ValidateDag(bad); err == nil {
		t.Fatal("unknown edge source must error")
	}

	a2 := testNode(t, "a", "x", nil)
	a2.ExecutionId = "ca"
	b2 := testNode(t, "b", "x", nil)
	b2.ExecutionId = "cb"
	cyc := testDag("vcycle", a2, b2)
	cyc.Edges = []*primitive.Dag_Edge{
		{Id: "ab", Source: "a", Target: "b"},
		{Id: "ba", Source: "b", Target: "a"},
	}
	if err := ValidateDag(cyc); err == nil {
		t.Fatal("cycle must error")
	}
}

// --- small unit gaps ---------------------------------------------------------

func TestNextDelayBackoffJitter(t *testing.T) {
	policy := &primitive.Retry_Policy{
		Delay:     durationpb.New(100 * time.Millisecond),
		MaxJitter: durationpb.New(50 * time.Millisecond),
		DelayType: primitive.Retry_DELAY_TYPE_BACKOFF_JITTER,
	}
	for range 100 {
		got := nextDelay(policy, nil)
		if got < 100*time.Millisecond || got >= 150*time.Millisecond {
			t.Fatalf("backoff+jitter = %v, want [100ms,150ms)", got)
		}
	}
}

func TestCanRetrySemantics(t *testing.T) {
	if canRetry(nil, 0) {
		t.Fatal("nil policy must not retry")
	}
	bounded := &primitive.Retry_Policy{UntilSucceeded: true, Attempts: 2}
	if !canRetry(bounded, 1) {
		t.Fatal("attempt 1 < 2 should retry")
	}
	if canRetry(bounded, 2) {
		t.Fatal("until_succeeded with attempts>0 stays bounded")
	}
	infinite := &primitive.Retry_Policy{UntilSucceeded: true, Attempts: 0}
	if !canRetry(infinite, 1000) {
		t.Fatal("until_succeeded with attempts=0 retries indefinitely")
	}
}

func TestDefaultNodeExecutionID(t *testing.T) {
	if got := defaultNodeExecutionID("", "n"); got != "n" {
		t.Fatalf("empty prefix = %q, want n", got)
	}
	if got := defaultNodeExecutionID("p", ""); got != "p" {
		t.Fatalf("empty node = %q, want p", got)
	}
	if got := defaultNodeExecutionID("p", "n"); got != "p.n" {
		t.Fatalf("got %q, want p.n", got)
	}
}

// --- edge on_status ----------------------------------------------------------

func TestEdgeOnStatusFailedRunsHandler(t *testing.T) {
	rec := &recorder{}
	reg := NewTaskRegistry(failTask(rec, "work"), okTask(rec, "onfail"))
	dag := testDag("onstatus-fail",
		testNode(t, "work", "work", nil),
		testNode(t, "onfail", "onfail", nil),
	)
	dag.Scheduling.OnNodeFailure = primitive.Dag_Scheduling_ON_NODE_FAILURE_CONTINUE
	dag.Edges = []*primitive.Dag_Edge{{
		Id:        "w-h",
		Source:    "work",
		Target:    "onfail",
		Condition: &primitive.Dag_Edge_OnStatus{OnStatus: primitive.Status_STATUS_FAILED},
	}}

	_ = NewExecutor(reg).Run(context.Background(), dag)
	if got := nodeByID(dag, "onfail").GetStatus(); got != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("handler status = %s, want completed (on_status FAILED matched)", got)
	}
	if dag.GetStatus() != primitive.Status_STATUS_FAILED {
		t.Fatalf("dag status = %s, want failed", dag.GetStatus())
	}
}

func TestEdgeOnStatusCompletedMatches(t *testing.T) {
	reg := NewTaskRegistry(okTask(nil, "a"), okTask(nil, "b"))
	dag := testDag("onstatus-ok", testNode(t, "a", "a", nil), testNode(t, "b", "b", nil))
	dag.Edges = []*primitive.Dag_Edge{{
		Id:        "a-b",
		Source:    "a",
		Target:    "b",
		Condition: &primitive.Dag_Edge_OnStatus{OnStatus: primitive.Status_STATUS_COMPLETED},
	}}

	if err := NewExecutor(reg).Run(context.Background(), dag); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if nodeByID(dag, "b").GetStatus() != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("b status = %s, want completed", nodeByID(dag, "b").GetStatus())
	}
}

func TestEdgeOnStatusUnmatchedNeverRunsTarget(t *testing.T) {
	reg := NewTaskRegistry(
		okTask(nil, "a"),
		NewTask("b", func(*emptypb.Empty) (*emptypb.Empty, error) {
			t.Fatal("b must not run: on_status FAILED never matches a COMPLETED source")
			return &emptypb.Empty{}, nil
		}),
	)
	dag := testDag("onstatus-unmatched", testNode(t, "a", "a", nil), testNode(t, "b", "b", nil))
	dag.Edges = []*primitive.Dag_Edge{{
		Id:        "a-b",
		Source:    "a",
		Target:    "b",
		Condition: &primitive.Dag_Edge_OnStatus{OnStatus: primitive.Status_STATUS_FAILED},
	}}

	if err := NewExecutor(reg).Run(context.Background(), dag); err == nil {
		t.Fatal("Run() expected failure (target unsatisfiable)")
	}
	if dag.GetStatus() != primitive.Status_STATUS_FAILED {
		t.Fatalf("dag status = %s, want failed", dag.GetStatus())
	}
}

// --- always_run waits for a running dependency -------------------------------

func TestReadyNodesAlwaysRunWaitsForRunningDependency(t *testing.T) {
	b := alwaysRunNode(t, "b", "x")
	b.Status = primitive.Status_STATUS_RUNNING
	c := alwaysRunNode(t, "c", "x")
	dag := testDag("ar-wait", b, c)
	dag.Edges = []*primitive.Dag_Edge{{Id: "bc", Source: "b", Target: "c"}}
	normalizeDag(dag)

	ready := NewExecutor(NewTaskRegistry()).readyNodes(dag, true)
	for _, n := range ready {
		if n.GetId() == "c" {
			t.Fatal("c must wait while always_run dependency b is RUNNING")
		}
	}
}

// --- Run entry guards --------------------------------------------------------

func TestRunNilDag(t *testing.T) {
	if err := NewExecutor(NewTaskRegistry()).Run(context.Background(), nil); err == nil {
		t.Fatal("Run(nil) must error")
	}
}

func TestRunInvalidDagMarksFailed(t *testing.T) {
	dag := testDag("inv", testNode(t, "a", "x", nil))
	dag.Edges = []*primitive.Dag_Edge{{Id: "e", Source: "a", Target: "ghost"}}

	err := NewExecutor(NewTaskRegistry()).Run(context.Background(), dag)
	if err == nil {
		t.Fatal("Run() with invalid dag must error")
	}
	if dag.GetStatus() != primitive.Status_STATUS_FAILED {
		t.Fatalf("dag status = %s, want failed", dag.GetStatus())
	}
}

// --- save hook errors --------------------------------------------------------

func TestRunReturnsSaveErrorOnStart(t *testing.T) {
	errSave := errors.New("save boom")
	reg := NewTaskRegistry(okTask(nil, "a"))
	dag := testDag("save-start", testNode(t, "a", "a", nil))
	exec := NewExecutor(reg, WithSaveHook(func(context.Context, *primitive.Dag) error { return errSave }))

	if err := exec.Run(context.Background(), dag); !errors.Is(err, errSave) {
		t.Fatalf("Run() error = %v, want save error", err)
	}
}

func TestRunReturnsSaveErrorDuringBatch(t *testing.T) {
	errSave := errors.New("save boom")
	reg := NewTaskRegistry(okTask(nil, "a"))
	dag := testDag("save-batch", testNode(t, "a", "a", nil))
	calls := 0
	exec := NewExecutor(reg, WithSaveHook(func(context.Context, *primitive.Dag) error {
		calls++
		if calls >= 2 {
			return errSave
		}
		return nil
	}))

	if err := exec.Run(context.Background(), dag); !errors.Is(err, errSave) {
		t.Fatalf("Run() error = %v, want save error during batch", err)
	}
}

// --- recovery ----------------------------------------------------------------

func TestRecoverDagPromotesPendingToRunning(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	dag := testDag("rec-promote", testNode(t, "a", "x", nil))
	normalizeDag(dag)
	dag.Status = primitive.Status_STATUS_PENDING
	dag.Execution.Status = primitive.Status_STATUS_RUNNING

	RecoverDag(dag, now)

	if dag.GetStatus() != primitive.Status_STATUS_RUNNING {
		t.Fatalf("dag status = %s, want promoted to running", dag.GetStatus())
	}
	if dag.GetExecution().GetStartedAt() == nil {
		t.Fatal("started_at must be set on recovery")
	}
}

func TestRecoverCancellingKeepsRunningAlwaysRun(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	cleanup := alwaysRunNode(t, "cleanup", "x")
	cleanup.Status = primitive.Status_STATUS_RUNNING
	main := testNode(t, "main", "x", nil)
	dag := testDag("rec-cancel", main, cleanup)
	dag.Status = primitive.Status_STATUS_CANCELLING

	RecoverDag(dag, now)

	if cleanup.GetStatus() != primitive.Status_STATUS_RETRY_WAIT {
		t.Fatalf("running always_run status = %s, want retry_wait", cleanup.GetStatus())
	}
	if main.GetStatus() != primitive.Status_STATUS_CANCELLED {
		t.Fatalf("main status = %s, want cancelled", main.GetStatus())
	}
	if dag.GetStatus() != primitive.Status_STATUS_CANCELLING {
		t.Fatalf("dag status = %s, want still cancelling (runnable always_run left)", dag.GetStatus())
	}
}

// --- run on already-terminal dag ---------------------------------------------

func TestRunAlreadyCompletedIsNoop(t *testing.T) {
	reg := NewTaskRegistry(NewTask("never", func(*emptypb.Empty) (*emptypb.Empty, error) {
		t.Fatal("task must not run on an already-completed dag")
		return &emptypb.Empty{}, nil
	}))
	dag := testDag("already-done", testNode(t, "a", "never", nil))
	normalizeDag(dag)
	dag.Status = primitive.Status_STATUS_COMPLETED
	node := dag.GetNodes()[0]
	node.Status = primitive.Status_STATUS_COMPLETED
	node.Execution.Status = primitive.Status_STATUS_COMPLETED

	if err := NewExecutor(reg).Run(context.Background(), dag); err != nil {
		t.Fatalf("Run() error = %v, want nil for completed dag", err)
	}
}

func TestRunAlreadyFailedReturnsTerminalError(t *testing.T) {
	reg := NewTaskRegistry(okTask(nil, "a"))
	dag := testDag("already-failed", testNode(t, "a", "a", nil))
	normalizeDag(dag)
	dag.Status = primitive.Status_STATUS_FAILED
	dag.Execution.Status = primitive.Status_STATUS_FAILED
	node := dag.GetNodes()[0]
	node.Status = primitive.Status_STATUS_FAILED
	node.Execution.Status = primitive.Status_STATUS_FAILED

	if err := NewExecutor(reg).Run(context.Background(), dag); err == nil {
		t.Fatal("Run() on a failed dag must return terminal error")
	}
}

// --- defensive units ---------------------------------------------------------

func TestExecuteNodeNoVariant(t *testing.T) {
	node := &primitive.Dag_Node{
		Id:         "x",
		Scheduling: &primitive.Dag_Node_Scheduling{RetryPolicy: &primitive.Retry_Policy{Attempts: 1}},
	}
	res := NewExecutor(NewTaskRegistry()).executeNode(context.Background(), node)
	if res.err == nil {
		t.Fatal("node without variant must error")
	}
	var fe *FailureError
	if !errors.As(res.err, &fe) || fe.Failure().GetCode() != FailureCodeDagInvalid {
		t.Fatalf("error = %v, want %q", res.err, FailureCodeDagInvalid)
	}
}

func TestNextDelayNilPolicyAndNegative(t *testing.T) {
	if got := nextDelay(nil, nil); got != 0 {
		t.Fatalf("nil policy delay = %v, want 0", got)
	}
	neg := &primitive.Retry_Policy{
		Delay:     durationpb.New(-100 * time.Millisecond),
		DelayType: primitive.Retry_DELAY_TYPE_FIXED,
	}
	if got := nextDelay(neg, nil); got != 0 {
		t.Fatalf("negative delay = %v, want clamped to 0", got)
	}
}

func TestCanRetryDefaultsAttemptsToOne(t *testing.T) {
	policy := &primitive.Retry_Policy{Attempts: 0}
	if !canRetry(policy, 0) {
		t.Fatal("attempt 0 of default-1 budget should retry")
	}
	if canRetry(policy, 1) {
		t.Fatal("attempt 1 exhausts default-1 budget")
	}
}

func TestNextRetryAtZeroWithoutNextRun(t *testing.T) {
	node := testNode(t, "a", "x", nil)
	node.Status = primitive.Status_STATUS_RETRY_WAIT
	node.Execution = &primitive.Dag_Node_Execution{RetryState: &primitive.Retry_State{Attempt: 1}}
	dag := testDag("nr", node)

	if !nextRetryAt(dag).IsZero() {
		t.Fatal("nextRetryAt must be zero when a retry-wait node has no next_run_at")
	}
}

func TestTerminalErrorUnknownFailure(t *testing.T) {
	dag := &primitive.Dag{Id: "x", Status: primitive.Status_STATUS_FAILED}
	err := terminalError(dag)
	if err == nil || err.Error() != `dag "x" failed: unknown failure` {
		t.Fatalf("terminalError = %v, want unknown failure message", err)
	}
}

// --- multi-node sub_dag ------------------------------------------------------

func TestSubDagMultiNode(t *testing.T) {
	rec := &recorder{}
	reg := NewTaskRegistry(okTask(rec, "x"))
	inner := testDag("inner", testNode(t, "a", "x", nil), testNode(t, "b", "x", nil))
	inner.Edges = []*primitive.Dag_Edge{{Id: "a-b", Source: "a", Target: "b"}}
	outer := testDag("outer-multi", subDagNode("sub", inner))

	if err := NewExecutor(reg).Run(context.Background(), outer); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if outer.GetStatus() != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("outer status = %s, want completed", outer.GetStatus())
	}
	if got := len(rec.snapshot()); got != 2 {
		t.Fatalf("inner task calls = %d, want 2", got)
	}
}

// --- default (real) clock ----------------------------------------------------

func TestExecutorRealClockRetrySleep(t *testing.T) {
	calls := 0
	reg := NewTaskRegistry(NewTask("flaky", func(*emptypb.Empty) (*emptypb.Empty, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("transient")
		}
		return &emptypb.Empty{}, nil
	}))
	dag := testDag("real-sleep", testNode(t, "flaky", "flaky", &primitive.Retry_Policy{
		Attempts:  2,
		Delay:     durationpb.New(2 * time.Millisecond),
		DelayType: primitive.Retry_DELAY_TYPE_FIXED,
	}))

	// No WithClock: exercises the real timer-based sleep.
	if err := NewExecutor(reg).Run(context.Background(), dag); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if dag.GetStatus() != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("dag status = %s, want completed", dag.GetStatus())
	}
}

func TestExecutorRealClockCancelDuringSleep(t *testing.T) {
	reg := NewTaskRegistry(NewTask("flaky", func(*emptypb.Empty) (*emptypb.Empty, error) {
		return nil, errors.New("always fails")
	}))
	dag := testDag("real-cancel", testNode(t, "flaky", "flaky", &primitive.Retry_Policy{
		Attempts:       0,
		UntilSucceeded: true,
		Delay:          durationpb.New(time.Hour), // long; cancel interrupts the sleep
		DelayType:      primitive.Retry_DELAY_TYPE_FIXED,
	}))
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := NewExecutor(reg).Run(ctx, dag)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	if dag.GetStatus() != primitive.Status_STATUS_CANCELLED {
		t.Fatalf("dag status = %s, want cancelled", dag.GetStatus())
	}
}

// --- registry + FailureError -------------------------------------------------

func TestTaskRegistryZeroValueAndNilReceiver(t *testing.T) {
	var r TaskRegistry // nil map
	r.Register(okTask(nil, "a"))
	if _, ok := r.GetTaskHandler("a"); !ok {
		t.Fatal("Register must lazily init the map on a zero-value registry")
	}
	if _, ok := r.GetTaskHandler("missing"); ok {
		t.Fatal("missing handler must return false")
	}
	var nilReg *TaskRegistry
	if _, ok := nilReg.GetTaskHandler("a"); ok {
		t.Fatal("nil *TaskRegistry must return false")
	}
}

func TestNewFailureErrorDefaults(t *testing.T) {
	fe := NewFailureError(nil, nil)
	if fe.Error() == "" {
		t.Fatal("nil error must get a default message")
	}
	if fe.Failure() == nil || fe.Failure().GetMessage() == "" {
		t.Fatal("failure message must default to the error text")
	}
	fe2 := NewFailureError(errors.New("x"), &primitive.Dag_Failure{Message: "preset"})
	if fe2.Failure().GetMessage() != "preset" {
		t.Fatal("preset failure message must be preserved")
	}
}

// --- normalize defaults ------------------------------------------------------

func TestNormalizeFillsDefaults(t *testing.T) {
	input, err := anypb.New(&emptypb.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	dag := &primitive.Dag{
		Id: "d",
		Nodes: []*primitive.Dag_Node{{
			Id:      "n",
			Variant: &primitive.Dag_Node_TaskState_{TaskState: &primitive.Dag_Node_TaskState{HandlerName: "h", Input: input}},
		}},
	}

	normalizeDag(dag)

	if dag.GetScheduling().GetOnNodeFailure() != primitive.Dag_Scheduling_ON_NODE_FAILURE_STOP {
		t.Fatal("nil scheduling must default on_node_failure to STOP")
	}
	if dag.GetStatus() != primitive.Status_STATUS_PENDING {
		t.Fatalf("default dag status = %s, want pending", dag.GetStatus())
	}
	n := dag.GetNodes()[0]
	if n.GetScheduling().GetRetryPolicy().GetAttempts() != 1 {
		t.Fatal("default retry attempts must be 1")
	}
	if n.GetStatus() != primitive.Status_STATUS_PENDING {
		t.Fatalf("default node status = %s, want pending", n.GetStatus())
	}
	if n.GetExecution() == nil || n.GetExecutionId() != "d.n" {
		t.Fatalf("node execution/id not initialized: exec=%v id=%q", n.GetExecution(), n.GetExecutionId())
	}

	dag2 := &primitive.Dag{
		Id:         "d2",
		Scheduling: &primitive.Dag_Scheduling{}, // present but UNSPECIFIED on_node_failure
		Nodes: []*primitive.Dag_Node{{
			Id:      "n",
			Variant: &primitive.Dag_Node_TaskState_{TaskState: &primitive.Dag_Node_TaskState{HandlerName: "h", Input: input}},
		}},
	}
	normalizeDag(dag2)
	if dag2.GetScheduling().GetOnNodeFailure() != primitive.Dag_Scheduling_ON_NODE_FAILURE_STOP {
		t.Fatal("unspecified on_node_failure must default to STOP")
	}
}

func TestAlwaysRunTeardownRespectsMaxParallelism(t *testing.T) {
	reg := NewTaskRegistry(failTask(nil, "work"), okTask(nil, "x"))
	work := testNode(t, "work", "work", nil)
	c1 := alwaysRunNode(t, "c1", "x")
	c2 := alwaysRunNode(t, "c2", "x")
	dag := testDag("ar-maxpar", work, c1, c2)
	dag.Scheduling.MaxParallelism = 1

	if err := NewExecutor(reg).Run(context.Background(), dag); err == nil {
		t.Fatal("Run() expected failure")
	}
	if c1.GetStatus() != primitive.Status_STATUS_COMPLETED || c2.GetStatus() != primitive.Status_STATUS_COMPLETED {
		t.Fatalf("both cleanups should run under max_parallelism=1: c1=%s c2=%s", c1.GetStatus(), c2.GetStatus())
	}
}

// --- task handler errors -----------------------------------------------------

func TestTaskHandlerNotFound(t *testing.T) {
	reg := NewTaskRegistry() // empty registry
	dag := testDag("no-handler", testNode(t, "a", "ghost", nil))

	if err := NewExecutor(reg).Run(context.Background(), dag); err == nil {
		t.Fatal("Run() expected failure for missing handler")
	}
	failure := nodeByID(dag, "a").GetExecution().GetFailure()
	if failure.GetCode() != FailureCodeTaskHandlerNotFound {
		t.Fatalf("failure code = %q, want %q", failure.GetCode(), FailureCodeTaskHandlerNotFound)
	}
}

func TestTaskInputTypeMismatch(t *testing.T) {
	// Handler expects *durationpb.Duration but the node input is *emptypb.Empty.
	reg := NewTaskRegistry(NewTask("typed", func(*durationpb.Duration) (*emptypb.Empty, error) {
		return &emptypb.Empty{}, nil
	}))
	dag := testDag("type-mismatch", testNode(t, "a", "typed", nil))

	if err := NewExecutor(reg).Run(context.Background(), dag); err == nil {
		t.Fatal("Run() expected failure on input type mismatch")
	}
	if got := nodeByID(dag, "a").GetStatus(); got != primitive.Status_STATUS_FAILED {
		t.Fatalf("node status = %s, want failed", got)
	}
}
