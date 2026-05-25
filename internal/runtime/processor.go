package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"google.golang.org/protobuf/proto"
)

var StatusesToProcess = []primitive.Status{
	primitive.Status_STATUS_PENDING,
	primitive.Status_STATUS_RUNNING,
	primitive.Status_STATUS_RETRY_WAIT,
	primitive.Status_STATUS_CANCELLING,
}

type Storage interface {
	ListDagsByStatus(ctx context.Context, status []primitive.Status) ([]*primitive.Dag, error)
	GetDag(ctx context.Context, id string) (*primitive.Dag, error)
	SaveDag(ctx context.Context, dag *primitive.Dag) error
}

// MetaSuiteGated marks a per-test child dag that a suite's orchestration dag owns:
// the processor's main loop SKIPS gated dags (so they don't all run at once), and the
// orchestration's dag_ref node releases the gate (clears the key) when it schedules
// the child — which is how MaxParallelism is enforced (only as many children are
// released as the orchestration runs dag_ref nodes concurrently). suite.go sets it.
const MetaSuiteGated = "suite_gated"

type ProcessorOption func(*DagProcessor)

var ErrDagActive = errors.New("dag is already active")

type DagProcessor struct {
	storage    Storage
	tasks      TasksRegistry
	predicates PredicateRegistry
	interval   time.Duration
	stop       chan struct{}
	done       chan struct{}
	dags       cmap.ConcurrentMap[string, *primitive.Dag]
	mu         sync.Mutex
	started    bool
	active     map[string]struct{}
	generation map[string]uint64
	// cancelReq holds dag ids whose cancellation was requested via Cancel (the
	// runtime API). processOne forces a requested dag to CANCELLING before running
	// the executor, so the executor itself drives always_run teardown + settles
	// CANCELLED — no external status write races the executor's snapshots.
	cancelReq  map[string]struct{}
	cancel     context.CancelFunc
	stopOnce   sync.Once
	wg         sync.WaitGroup
	onTerminal func(context.Context, *primitive.Dag)
}

func NewDagProcessor(storage Storage, tasks TasksRegistry, opts ...ProcessorOption) *DagProcessor {
	p := &DagProcessor{
		storage:    storage,
		tasks:      tasks,
		interval:   time.Second,
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
		dags:       cmap.New[*primitive.Dag](),
		active:     make(map[string]struct{}),
		generation: make(map[string]uint64),
		cancelReq:  make(map[string]struct{}),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func WithProcessorPredicates(predicates PredicateRegistry) ProcessorOption {
	return func(p *DagProcessor) {
		p.predicates = predicates
	}
}

func WithProcessorInterval(interval time.Duration) ProcessorOption {
	return func(p *DagProcessor) {
		if interval > 0 {
			p.interval = interval
		}
	}
}

// WithTerminalHook registers a callback fired once when a dag reaches a terminal
// status (the app wires this to run/suite webhook delivery; runtime stays generic).
func WithTerminalHook(fn func(context.Context, *primitive.Dag)) ProcessorOption {
	return func(p *DagProcessor) { p.onTerminal = fn }
}

func (p *DagProcessor) lifecycle(ctx context.Context) {
	defer close(p.done)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.refreshFromStorage(ctx)
	p.processAll(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stop:
			return
		case <-ticker.C:
			p.refreshFromStorage(ctx)
			p.processAll(ctx)
		}
	}
}

func (p *DagProcessor) processAll(ctx context.Context) {
	for item := range p.dags.IterBuffered() {
		dag := item.Val
		if isDagTerminal(dag.GetStatus()) {
			p.dags.Remove(item.Key)
			continue
		}
		generation, ok := p.markActive(item.Key)
		if !ok {
			continue
		}
		p.wg.Add(1)
		go p.processOne(ctx, item.Key, generation, dag)
	}
}

func (p *DagProcessor) processOne(ctx context.Context, id string, generation uint64, dag *primitive.Dag) {
	defer p.wg.Done()
	defer p.markInactive(id)
	executor := NewExecutor(
		p.tasks,
		WithPredicates(p.predicates),
		WithSaveHook(p.storage.SaveDag),
		// dag_ref nodes (suite orchestration dags) drive their referenced per-test
		// dags through the processor; the runner releases the gate + waits for terminal.
		WithDagRefRunner(p),
	)
	runDag := proto.Clone(dag).(*primitive.Dag)
	// A cancellation was requested: drive the dag to CANCELLING so the executor's
	// cancel path runs always_run teardown (destroyDocker) under a live ctx and
	// settles CANCELLED. The flag (not a DB write) survives across ticks, so this
	// is race-free even if the dag was mid-flight when Cancel was called.
	if p.isCancelRequested(id) && !isDagTerminal(runDag.GetStatus()) {
		runDag.Status = primitive.Status_STATUS_CANCELLING
	}
	_ = executor.Run(ctx, runDag)
	_ = p.storage.SaveDag(context.Background(), runDag)
	if !p.isCurrentGeneration(id, generation) {
		return
	}
	if isDagTerminal(runDag.GetStatus()) {
		p.clearCancel(id)
		if p.onTerminal != nil {
			p.onTerminal(ctx, runDag)
		}
		p.dags.Remove(id)
		return
	}
	p.dags.Set(id, runDag)
}

func (p *DagProcessor) markActive(id string) (uint64, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.active[id]; ok {
		return 0, false
	}
	p.active[id] = struct{}{}
	return p.generation[id], true
}

func (p *DagProcessor) markInactive(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.active, id)
}

func (p *DagProcessor) isActive(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.active[id]
	return ok
}

// Cancel requests cancellation of a dag (the runtime API the run/suite services
// call). It records intent in-process; the next process pass forces the dag to
// CANCELLING and the executor runs always_run teardown + settles CANCELLED. The
// runtime owns the status transition — callers never write the dag status directly.
func (p *DagProcessor) Cancel(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cancelReq[id] = struct{}{}
}

func (p *DagProcessor) isCancelRequested(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.cancelReq[id]
	return ok
}

func (p *DagProcessor) clearCancel(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.cancelReq, id)
}

// RunDagRef drives a suite orchestration dag's dag_ref node: it releases the
// referenced per-test child dag's gate (so the processor's main loop starts running
// it — agent polling and all) and then blocks until that child reaches a terminal
// status, returning it. Because the orchestration executor only runs MaxParallelism
// dag_ref nodes concurrently (runBatch + the batch-at-a-time loop), only that many
// children are ever released + running at once — that is how suite parallelism is
// bounded. A cancelled context (processor stop / suite cancel) unblocks the poll.
func (p *DagProcessor) RunDagRef(ctx context.Context, ref *primitive.Dag_Node_DagRef) (*primitive.Dag, error) {
	id := ref.GetDagId()
	child, err := p.storage.GetDag(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load dag ref %q: %w", id, err)
	}
	if child == nil {
		return nil, fmt.Errorf("dag ref %q not found", id)
	}
	// Release the gate once so the main loop picks the child up.
	if child.GetMetadata()[MetaSuiteGated] != "" {
		delete(child.Metadata, MetaSuiteGated)
		if err := p.storage.SaveDag(ctx, child); err != nil {
			return nil, fmt.Errorf("release dag ref %q: %w", id, err)
		}
	}
	for {
		if isDagTerminal(child.GetStatus()) {
			return child, nil
		}
		select {
		case <-ctx.Done():
			return child, ctx.Err()
		case <-time.After(p.interval):
		}
		child, err = p.storage.GetDag(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("poll dag ref %q: %w", id, err)
		}
		if child == nil {
			return nil, fmt.Errorf("dag ref %q vanished", id)
		}
	}
}

// refreshFromStorage pulls newly-submitted (or recovered) processable dags into the
// in-memory working set each tick, so runs persisted by the API after boot get
// picked up. Already-tracked or in-flight dags are left untouched.
func (p *DagProcessor) refreshFromStorage(ctx context.Context) {
	dags, err := p.storage.ListDagsByStatus(ctx, StatusesToProcess)
	if err != nil {
		return
	}
	for _, dag := range dags {
		id := dag.GetId()
		// A suite-gated per-test dag is owned by its orchestration dag's dag_ref node —
		// the main loop must NOT run it until that node releases the gate (RunDagRef),
		// otherwise every test would start at once and MaxParallelism would be ignored.
		if dag.GetMetadata()[MetaSuiteGated] != "" {
			continue
		}
		// Skip a dag that is mid-flight (a processOne goroutine owns it) to avoid
		// clobbering its in-progress snapshot. Otherwise (re)load the storage copy
		// every tick: it is the source of truth and carries agent Reports applied
		// out-of-band by the CommandQueue, which the cached in-memory copy lacks.
		if p.isActive(id) {
			continue
		}
		p.dags.Set(id, dag)
	}
}

func (p *DagProcessor) isCurrentGeneration(id string, generation uint64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.generation[id] == generation
}

func (p *DagProcessor) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return nil
	}
	p.started = true
	runCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.mu.Unlock()

	dags, err := p.storage.ListDagsByStatus(runCtx, StatusesToProcess)
	if err != nil {
		cancel()
		return err
	}
	for _, d := range dags {
		p.dags.Set(d.GetId(), d)
	}
	go p.lifecycle(runCtx)
	return nil
}

func (p *DagProcessor) AddDag(ctx context.Context, dag *primitive.Dag) error {
	if dag == nil {
		return fmt.Errorf("dag is nil")
	}
	normalizeDag(dag)
	if err := ValidateDag(dag); err != nil {
		return err
	}
	id := dag.GetId()
	p.mu.Lock()
	if _, ok := p.active[id]; ok {
		p.mu.Unlock()
		return ErrDagActive
	}
	p.generation[id]++
	p.mu.Unlock()

	if err := p.storage.SaveDag(ctx, dag); err != nil {
		return err
	}
	p.dags.Set(id, dag)
	return nil
}

func (p *DagProcessor) Stop() {
	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return
	}
	p.started = false
	cancel := p.cancel
	p.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	p.stopOnce.Do(func() {
		close(p.stop)
	})
	<-p.done
	p.wg.Wait()
}

func isDagTerminal(status primitive.Status) bool {
	switch status {
	case primitive.Status_STATUS_COMPLETED, primitive.Status_STATUS_FAILED, primitive.Status_STATUS_CANCELLED, primitive.Status_STATUS_SKIPPED:
		return true
	default:
		return false
	}
}
