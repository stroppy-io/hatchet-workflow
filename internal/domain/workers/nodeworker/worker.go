package nodeworker

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

type Config struct {
	Workers int
	Tick    time.Duration
}

type Worker struct {
	pool     *pgxpool.Pool
	system   *system.Service
	registry *Registry
	cfg      Config
	log      *zap.Logger
}

func New(pool *pgxpool.Pool, sys *system.Service, reg *Registry, cfg Config, log *zap.Logger) *Worker {
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.Tick == 0 {
		cfg.Tick = 500 * time.Millisecond
	}
	return &Worker{pool: pool, system: sys, registry: reg, cfg: cfg, log: log}
}

func (w *Worker) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < w.cfg.Workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tick := time.NewTicker(w.cfg.Tick)
			defer tick.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-tick.C:
					w.tryOnce(ctx, id)
				}
			}
		}(i)
	}
	wg.Wait()
}

func (w *Worker) tryOnce(ctx context.Context, id int) {
	node, err := claimReadyNode(ctx, w.pool,
		systempb.NodeRunStatus_NODE_RUN_STATUS_READY.String(),
		systempb.NodeRunStatus_NODE_RUN_STATUS_RUNNING.String())
	if err != nil {
		w.log.Warn("claim error", zap.Int("worker", id), zap.Error(err))
		return
	}
	if node == nil {
		return
	}
	w.log.Debug("claimed node", zap.Int("worker", id), zap.String("node_run_id", node.ID))
	execute(ctx, w, node)
}
