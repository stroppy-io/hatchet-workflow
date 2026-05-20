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
	SaveDag(ctx context.Context, dag *primitive.Dag) error
}

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
	cancel     context.CancelFunc
	stopOnce   sync.Once
	wg         sync.WaitGroup
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

func (p *DagProcessor) lifecycle(ctx context.Context) {
	defer close(p.done)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.processAll(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stop:
			return
		case <-ticker.C:
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
	)
	runDag := proto.Clone(dag).(*primitive.Dag)
	_ = executor.Run(ctx, runDag)
	_ = p.storage.SaveDag(context.Background(), runDag)
	if !p.isCurrentGeneration(id, generation) {
		return
	}
	if isDagTerminal(runDag.GetStatus()) {
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
