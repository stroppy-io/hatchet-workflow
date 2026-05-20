package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

type failingListStorage struct{}

func (failingListStorage) ListDagsByStatus(context.Context, []primitive.Status) ([]*primitive.Dag, error) {
	return nil, errors.New("list boom")
}

func (failingListStorage) SaveDag(context.Context, *primitive.Dag) error { return nil }

func TestProcessorStartReturnsStorageError(t *testing.T) {
	p := NewDagProcessor(failingListStorage{}, NewTaskRegistry())
	if err := p.Start(context.Background()); err == nil {
		t.Fatal("Start() must propagate ListDagsByStatus error")
	}
}

type memoryProcessorStorage struct {
	mu   sync.Mutex
	dags map[string]*primitive.Dag
}

func newMemoryProcessorStorage(dags ...*primitive.Dag) *memoryProcessorStorage {
	s := &memoryProcessorStorage{dags: make(map[string]*primitive.Dag)}
	for _, dag := range dags {
		s.dags[dag.GetId()] = proto.Clone(dag).(*primitive.Dag)
	}
	return s
}

func (s *memoryProcessorStorage) ListDagsByStatus(_ context.Context, statuses []primitive.Status) ([]*primitive.Dag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	allowed := make(map[primitive.Status]struct{}, len(statuses))
	for _, status := range statuses {
		allowed[status] = struct{}{}
	}
	var dags []*primitive.Dag
	for _, dag := range s.dags {
		if _, ok := allowed[dag.GetStatus()]; ok {
			dags = append(dags, proto.Clone(dag).(*primitive.Dag))
		}
	}
	return dags, nil
}

func (s *memoryProcessorStorage) SaveDag(_ context.Context, dag *primitive.Dag) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dags[dag.GetId()] = proto.Clone(dag).(*primitive.Dag)
	return nil
}

func TestDagProcessorStopReturnsWhileLifecycleIsInInitialProcessAll(t *testing.T) {
	reg := NewTaskRegistry(NewTask("slow", func(*emptypb.Empty) (*emptypb.Empty, error) {
		time.Sleep(25 * time.Millisecond)
		return &emptypb.Empty{}, nil
	}))
	dag := testDag("processor-stop",
		testNode(t, "slow", "slow", nil),
	)
	storage := newMemoryProcessorStorage(dag)
	processor := NewDagProcessor(storage, reg, WithProcessorInterval(time.Hour))

	if err := processor.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	done := make(chan struct{})
	go func() {
		processor.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop() hung")
	}
}

func TestDagProcessorRejectsAddDagForActiveID(t *testing.T) {
	reg := NewTaskRegistry(NewTask("slow", func(*emptypb.Empty) (*emptypb.Empty, error) {
		time.Sleep(50 * time.Millisecond)
		return &emptypb.Empty{}, nil
	}))
	dag := testDag("processor-active",
		testNode(t, "slow", "slow", nil),
	)
	storage := newMemoryProcessorStorage(dag)
	processor := NewDagProcessor(storage, reg, WithProcessorInterval(time.Hour))
	if err := processor.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer processor.Stop()

	deadline := time.Now().Add(time.Second)
	for {
		processor.mu.Lock()
		_, active := processor.active[dag.GetId()]
		processor.mu.Unlock()
		if active {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("dag did not become active")
		}
		time.Sleep(time.Millisecond)
	}

	if err := processor.AddDag(context.Background(), dag); err == nil {
		t.Fatal("AddDag() expected active-id error")
	}
}

func dagStatusInStorage(s *memoryProcessorStorage, id string) primitive.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.dags[id]; ok {
		return d.GetStatus()
	}
	return primitive.Status_STATUS_UNSPECIFIED
}

func waitForDagStatus(t *testing.T, s *memoryProcessorStorage, id string, want primitive.Status, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if dagStatusInStorage(s, id) == want {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("dag %q status = %s, want %s within %s", id, dagStatusInStorage(s, id), want, timeout)
}

// AddDag schedules a dag that the polling lifecycle drives to completion.
func TestProcessorRunsAddedDagToCompletion(t *testing.T) {
	storage := newMemoryProcessorStorage()
	reg := NewTaskRegistry(okTask(nil, "work"))
	p := NewDagProcessor(storage, reg, WithProcessorInterval(5*time.Millisecond))

	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer p.Stop()

	dag := testDag("proc-complete", testNode(t, "work", "work", nil))
	if err := p.AddDag(context.Background(), dag); err != nil {
		t.Fatalf("AddDag() error = %v", err)
	}
	waitForDagStatus(t, storage, "proc-complete", primitive.Status_STATUS_COMPLETED, 2*time.Second)
}

// Dags already persisted are picked up on Start.
func TestProcessorProcessesDagFromStorageOnStart(t *testing.T) {
	pending := testDag("proc-existing", testNode(t, "work", "work", nil))
	storage := newMemoryProcessorStorage(pending)
	reg := NewTaskRegistry(okTask(nil, "work"))
	p := NewDagProcessor(storage, reg, WithProcessorInterval(5*time.Millisecond))

	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer p.Stop()
	waitForDagStatus(t, storage, "proc-existing", primitive.Status_STATUS_COMPLETED, 2*time.Second)
}

// A failing dag reaches FAILED terminal state via the processor.
func TestProcessorRunsFailingDagToFailed(t *testing.T) {
	storage := newMemoryProcessorStorage()
	reg := NewTaskRegistry(failTask(nil, "work"))
	p := NewDagProcessor(storage, reg, WithProcessorInterval(5*time.Millisecond))

	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer p.Stop()

	dag := testDag("proc-fail", testNode(t, "work", "work", nil))
	if err := p.AddDag(context.Background(), dag); err != nil {
		t.Fatalf("AddDag() error = %v", err)
	}
	waitForDagStatus(t, storage, "proc-fail", primitive.Status_STATUS_FAILED, 2*time.Second)
}

// Once a dag finishes and is no longer active, the same id may be re-added.
func TestProcessorAllowsReAddAfterCompletion(t *testing.T) {
	storage := newMemoryProcessorStorage()
	reg := NewTaskRegistry(okTask(nil, "work"))
	p := NewDagProcessor(storage, reg, WithProcessorInterval(5*time.Millisecond))

	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer p.Stop()

	dag := testDag("proc-readd", testNode(t, "work", "work", nil))
	if err := p.AddDag(context.Background(), dag); err != nil {
		t.Fatalf("first AddDag() error = %v", err)
	}
	waitForDagStatus(t, storage, "proc-readd", primitive.Status_STATUS_COMPLETED, 2*time.Second)

	// Reset to pending and re-add; the id is inactive again so it is accepted.
	fresh := proto.Clone(dag).(*primitive.Dag)
	fresh.Status = primitive.Status_STATUS_PENDING
	fresh.Execution = nil
	for _, n := range fresh.GetNodes() {
		n.Status = primitive.Status_STATUS_PENDING
		n.Execution = nil
	}
	if err := p.AddDag(context.Background(), fresh); err != nil {
		t.Fatalf("re-add AddDag() error = %v", err)
	}
	waitForDagStatus(t, storage, "proc-readd", primitive.Status_STATUS_COMPLETED, 2*time.Second)
}

// Predicate registry is wired through the processor into the executor.
func TestProcessorUsesPredicates(t *testing.T) {
	storage := newMemoryProcessorStorage()
	reg := NewTaskRegistry(okTask(nil, "a"), okTask(nil, "b"))
	preds := PredicateRegistryMap{"go": func(*primitive.Dag, *primitive.Dag_Edge) bool { return true }}
	p := NewDagProcessor(storage, reg,
		WithProcessorInterval(5*time.Millisecond),
		WithProcessorPredicates(preds),
	)
	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer p.Stop()

	a := testNode(t, "a", "a", nil)
	b := testNode(t, "b", "b", nil)
	dag := testDag("proc-pred", a, b)
	dag.Edges = []*primitive.Dag_Edge{{
		Id:        "a-b",
		Source:    "a",
		Target:    "b",
		Condition: &primitive.Dag_Edge_PredicateName{PredicateName: "go"},
	}}
	if err := p.AddDag(context.Background(), dag); err != nil {
		t.Fatalf("AddDag() error = %v", err)
	}
	waitForDagStatus(t, storage, "proc-pred", primitive.Status_STATUS_COMPLETED, 2*time.Second)
}

func TestProcessorStopBeforeStartIsNoop(t *testing.T) {
	p := NewDagProcessor(newMemoryProcessorStorage(), NewTaskRegistry())
	done := make(chan struct{})
	go func() { p.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop() before Start() should return immediately")
	}
}

func TestProcessorStartIdempotent(t *testing.T) {
	p := NewDagProcessor(newMemoryProcessorStorage(), NewTaskRegistry(), WithProcessorInterval(5*time.Millisecond))
	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("first Start() error = %v", err)
	}
	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("second Start() error = %v", err)
	}
	p.Stop()
}

func TestProcessorAddDagRejectsNilAndInvalid(t *testing.T) {
	p := NewDagProcessor(newMemoryProcessorStorage(), NewTaskRegistry())

	if err := p.AddDag(context.Background(), nil); err == nil {
		t.Fatal("AddDag(nil) must error")
	}
	bad := testDag("bad", testNode(t, "a", "x", nil))
	bad.Edges = []*primitive.Dag_Edge{{Id: "e", Source: "a", Target: "ghost"}}
	if err := p.AddDag(context.Background(), bad); err == nil {
		t.Fatal("AddDag with invalid dag must error")
	}
}

func TestProcessorProcessAllRemovesTerminalDag(t *testing.T) {
	p := NewDagProcessor(newMemoryProcessorStorage(), NewTaskRegistry())
	done := testDag("term", testNode(t, "a", "x", nil))
	done.Status = primitive.Status_STATUS_COMPLETED
	p.dags.Set("term", done)

	p.processAll(context.Background())

	if _, ok := p.dags.Get("term"); ok {
		t.Fatal("terminal dag must be dropped from the active set")
	}
}
