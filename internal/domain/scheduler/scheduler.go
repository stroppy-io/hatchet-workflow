// Package scheduler owns the durable run lifecycle: enqueue → claim →
// running → terminal. It replaces the in-memory runCancels/runTenants/
// activeRuns counters that lived on Server with database-backed state so
// any restart can resume mid-flight runs and quotas survive process death.
//
// One scheduler runs per server process. Multiple servers can run side by
// side — claim uses FOR UPDATE SKIP LOCKED so they cooperate.
package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
)

// Runner is the interface scheduler uses to actually execute a run. The
// real impl is api.App; tests can swap a fake.
type Runner interface {
	Start(ctx context.Context, tenantID string, cfg types.RunConfig) error
	RecoverRun(ctx context.Context, tenantID string, snap *dag.Snapshot) error
	Storage() dag.Storage
}

// SettingsResolver returns the per-tenant settings (for quota limits) at
// the moment of admission. Decoupled so scheduler doesn't import api.
type SettingsResolver func(tenantID string) *types.ServerSettings

// RecoverChecker decides whether a snapshot is recoverable. Provider-aware:
// docker checks ContainerIDs alive, yandex pings agent /health.
type RecoverChecker func(state *dag.RunState) bool

// Config bundles scheduler tunables.
type Config struct {
	InstanceID        string // unique per server process
	Logger            *zap.Logger
	Pool              *pgxpool.Pool
	Runner            Runner
	SettingsResolver  SettingsResolver
	RecoverChecker    RecoverChecker
	ClaimInterval     time.Duration // how often to poll for claimable jobs
	HeartbeatInterval time.Duration // worker liveness ping
	StaleAfter        time.Duration // claim is considered dead if heartbeat older than this
	ReaperInterval    time.Duration // how often to revive stale claims
	InstanceHeartbeat time.Duration // server_instances heartbeat
	MarkOnNotRecover  string        // error message planted on non-recoverable claimed jobs
}

func Defaults() Config {
	return Config{
		ClaimInterval:     500 * time.Millisecond,
		HeartbeatInterval: 5 * time.Second,
		StaleAfter:        45 * time.Second,
		ReaperInterval:    10 * time.Second,
		InstanceHeartbeat: 5 * time.Second,
		MarkOnNotRecover:  "server restarted -- run could not be recovered",
	}
}

// Scheduler is the long-running service. Start it once on server boot,
// Stop on graceful shutdown.
type Scheduler struct {
	cfg    Config
	jobs   *postgres.JobStorage
	pool   *pgxpool.Pool
	logger *zap.Logger

	mu      sync.Mutex
	running map[string]context.CancelFunc // run_id → cancel for active workers
	wg      sync.WaitGroup

	stopCh chan struct{}
}

func New(cfg Config) *Scheduler {
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	d := Defaults()
	if cfg.ClaimInterval == 0 {
		cfg.ClaimInterval = d.ClaimInterval
	}
	if cfg.HeartbeatInterval == 0 {
		cfg.HeartbeatInterval = d.HeartbeatInterval
	}
	if cfg.StaleAfter == 0 {
		cfg.StaleAfter = d.StaleAfter
	}
	if cfg.ReaperInterval == 0 {
		cfg.ReaperInterval = d.ReaperInterval
	}
	if cfg.InstanceHeartbeat == 0 {
		cfg.InstanceHeartbeat = d.InstanceHeartbeat
	}
	if cfg.MarkOnNotRecover == "" {
		cfg.MarkOnNotRecover = d.MarkOnNotRecover
	}
	return &Scheduler{
		cfg:     cfg,
		jobs:    postgres.NewJobStorage(cfg.Pool),
		pool:    cfg.Pool,
		logger:  cfg.Logger,
		running: make(map[string]context.CancelFunc),
		stopCh:  make(chan struct{}),
	}
}

// Jobs exposes the storage so the API layer can enqueue / query.
func (s *Scheduler) Jobs() *postgres.JobStorage { return s.jobs }

// InstanceID returns this server process's UUID, used to tag claimed
// rows in job_runs / agent_commands so the reaper can detect dead servers.
func (s *Scheduler) InstanceID() string { return s.cfg.InstanceID }

// Start kicks off the scheduler goroutines: instance heartbeat, reaper,
// claim loop. Blocks only briefly to register the instance row.
func (s *Scheduler) Start(ctx context.Context) error {
	if err := s.registerInstance(ctx); err != nil {
		return fmt.Errorf("scheduler: register instance: %w", err)
	}
	if err := s.recoverInflight(ctx); err != nil {
		s.logger.Warn("scheduler: recovery pass had errors", zap.Error(err))
	}
	s.wg.Add(1)
	go s.instanceHeartbeatLoop()
	s.wg.Add(1)
	go s.reaperLoop()
	s.wg.Add(1)
	go s.claimLoop()
	return nil
}

// Stop signals all loops to exit and waits for active workers to finish or
// for ctx to expire. Returns once all background goroutines have returned.
func (s *Scheduler) Stop(ctx context.Context) {
	close(s.stopCh)
	// Mark our instance as stopping so other servers' reapers know they
	// can take over claimed rows belonging to us.
	_, _ = s.pool.Exec(ctx, `UPDATE server_instances SET stopping=TRUE WHERE id=$1`, s.cfg.InstanceID)
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
	}
	// Best-effort: remove our row so the next reaper iteration doesn't keep
	// us alive forever after process death.
	_, _ = s.pool.Exec(context.Background(), `DELETE FROM server_instances WHERE id=$1`, s.cfg.InstanceID)
}

// CancelRun stops a running worker if this server owns it; otherwise
// transitions a queued row to cancelled. Returns true when something was
// done.
func (s *Scheduler) CancelRun(ctx context.Context, tenantID, runID string) (bool, error) {
	s.mu.Lock()
	cancel, ok := s.running[runID]
	s.mu.Unlock()
	if ok {
		cancel()
		return true, nil
	}
	cancelled, err := s.jobs.CancelQueued(ctx, tenantID, runID)
	if err != nil {
		return false, err
	}
	return cancelled, nil
}

// --- internals ---

func (s *Scheduler) registerInstance(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO server_instances (id, listen_addr, version)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE
		   SET heartbeat_at=NOW(), stopping=FALSE`,
		s.cfg.InstanceID, "", "")
	return err
}

func (s *Scheduler) instanceHeartbeatLoop() {
	defer s.wg.Done()
	t := time.NewTicker(s.cfg.InstanceHeartbeat)
	defer t.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err := s.pool.Exec(ctx, `UPDATE server_instances SET heartbeat_at=NOW() WHERE id=$1`, s.cfg.InstanceID)
			cancel()
			if err != nil {
				s.logger.Warn("scheduler: instance heartbeat failed", zap.Error(err))
			}
		}
	}
}

func (s *Scheduler) reaperLoop() {
	defer s.wg.Done()
	t := time.NewTicker(s.cfg.ReaperInterval)
	defer t.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if n, err := s.jobs.ReviveStale(ctx, s.cfg.StaleAfter); err != nil {
				s.logger.Warn("scheduler: reaper failed", zap.Error(err))
			} else if n > 0 {
				s.logger.Info("scheduler: revived stale claims", zap.Int("count", n))
			}
			// Cancel agent_commands belonging to dead server instances —
			// keeps the durable command queue from accumulating zombies
			// after a restart. Identifies dead servers via heartbeat
			// freshness against the same StaleAfter threshold.
			if n, err := s.cancelOrphanCommands(ctx); err != nil {
				s.logger.Debug("scheduler: cancelOrphanCommands failed", zap.Error(err))
			} else if n > 0 {
				s.logger.Info("scheduler: cancelled orphan agent commands", zap.Int("count", n))
			}
			cancel()
		}
	}
}

// cancelOrphanCommands flags any pending/claimed agent_commands rows whose
// claimed_by points at a server_instances row that's missed its heartbeat
// for longer than StaleAfter. Returns the number of rows transitioned.
func (s *Scheduler) cancelOrphanCommands(ctx context.Context) (int, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE agent_commands ac
		SET state='cancelled', completed_at=NOW(),
		    error=COALESCE(NULLIF(ac.error,''), 'orphaned: server instance dead')
		WHERE ac.state IN ('pending','claimed')
		  AND ac.claimed_by IS NOT NULL
		  AND NOT EXISTS (
		    SELECT 1 FROM server_instances si
		    WHERE si.id = ac.claimed_by
		      AND si.heartbeat_at >= NOW() - ($1 * INTERVAL '1 second')
		  )`, s.cfg.StaleAfter.Seconds())
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func (s *Scheduler) claimLoop() {
	defer s.wg.Done()
	t := time.NewTicker(s.cfg.ClaimInterval)
	defer t.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-t.C:
			s.tryClaim()
		}
	}
}

// tryClaim attempts at most one claim per tick. Multiple ticks can run
// claims in quick succession when the queue is backed up.
func (s *Scheduler) tryClaim() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	limits := s.resolveLimits(ctx)
	job, err := s.jobs.ClaimNext(ctx, s.cfg.InstanceID, limits)
	if err != nil {
		s.logger.Warn("scheduler: claim failed", zap.Error(err))
		return
	}
	if job == nil {
		return
	}
	s.spawnWorker(*job)
}

// resolveLimits walks every tenant with a queued job and resolves their
// settings into JobCost limits the storage layer can compare against.
// We don't preload everything globally to keep settings cache fresh.
func (s *Scheduler) resolveLimits(ctx context.Context) map[string]postgres.JobCost {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT tenant_id FROM job_runs WHERE state='queued'
		  AND (not_before IS NULL OR not_before <= NOW())`)
	if err != nil {
		s.logger.Warn("scheduler: list queued tenants failed", zap.Error(err))
		return nil
	}
	defer rows.Close()
	out := make(map[string]postgres.JobCost)
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			continue
		}
		if s.cfg.SettingsResolver == nil {
			out[t] = postgres.JobCost{}
			continue
		}
		set := s.cfg.SettingsResolver(t)
		if set == nil {
			out[t] = postgres.JobCost{}
			continue
		}
		q := set.Quotas
		out[t] = postgres.JobCost{
			CPUs:        q.MaxConcurrentCPUs,
			MemoryMB:    q.MaxConcurrentMemoryMB,
			DiskGB:      q.MaxConcurrentDiskGB,
			VMCount:     q.MaxConcurrentVMs,
			RunsRunning: q.MaxConcurrentRuns,
		}
	}
	return out
}

// spawnWorker runs the actual job in a goroutine. The worker handles the
// running→terminal transition + heartbeats + final quota release. Caller
// has already moved the row to claimed.
func (s *Scheduler) spawnWorker(job postgres.JobRun) {
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.running[job.RunID] = cancel
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			s.mu.Lock()
			delete(s.running, job.RunID)
			s.mu.Unlock()
			cancel()
		}()

		var cfg types.RunConfig
		if err := json.Unmarshal(job.Config, &cfg); err != nil {
			s.finishWith(job, postgres.JobStateFailed, fmt.Sprintf("unmarshal config: %v", err))
			return
		}

		// Heartbeat goroutine — keeps job_runs.heartbeat_at fresh so the
		// reaper doesn't revive us.
		hbCtx, hbCancel := context.WithCancel(ctx)
		go s.heartbeatLoop(hbCtx, job.TenantID, job.RunID)
		defer hbCancel()

		if err := s.jobs.MarkRunning(context.Background(), job.TenantID, job.RunID); err != nil {
			s.logger.Warn("scheduler: mark running failed", zap.String("run", job.RunID), zap.Error(err))
		}

		// Apply per-step timeout from the suite policy snapshot, if any.
		// Timeout fires by cancelling the worker context — the DAG's
		// AlwaysRun teardown phase still gets a chance to clean up.
		runCtx := ctx
		if mins := stepTimeoutMinutes(job.SuitePolicy); mins > 0 {
			var tcancel context.CancelFunc
			runCtx, tcancel = context.WithTimeout(ctx, time.Duration(mins)*time.Minute)
			defer tcancel()
		}

		err := s.cfg.Runner.Start(runCtx, job.TenantID, cfg)
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				s.finishWith(job, postgres.JobStateCancelled, err.Error())
				return
			}
			s.finishWith(job, postgres.JobStateFailed, err.Error())
			return
		}
		s.finishWith(job, postgres.JobStateFinished, "")
	}()
}

func (s *Scheduler) heartbeatLoop(ctx context.Context, tenantID, runID string) {
	t := time.NewTicker(s.cfg.HeartbeatInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_ = s.jobs.Heartbeat(c, tenantID, runID)
			cancel()
		}
	}
}

func (s *Scheduler) finishWith(job postgres.JobRun, state postgres.JobState, errMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.jobs.Finish(ctx, job.TenantID, job.RunID, state, errMsg); err != nil {
		s.logger.Warn("scheduler: finish failed", zap.String("run", job.RunID), zap.Error(err))
	}
}

// recoverInflight runs once at startup. For every claimed/running row in
// the DB (left over by a previous server process) we decide:
//
//   - snapshot present and machines reachable → spawn RecoverRun (resume)
//   - snapshot present but machines unreachable on a cloud provider → still
//     spawn RecoverRun: the DAG's teardown phase is AlwaysRun, so even when
//     the agent-driven phases fail to talk to dead agents, terraform
//     destroy fires and releases cloud resources. Without this we'd leak
//     paid Yandex VMs every server restart that catches a run mid-flight.
//   - snapshot present but everything's gone (docker, no recoverable
//     resources) → mark failed and free quota; nothing to clean up.
//   - no snapshot at all → mark failed and free quota.
func (s *Scheduler) recoverInflight(ctx context.Context) error {
	jobs, err := s.jobs.ListInflight(ctx)
	if err != nil {
		return err
	}
	for _, j := range jobs {
		s.logger.Info("scheduler: inflight on startup", zap.String("run", j.RunID), zap.String("state", string(j.State)))
		snap, err := s.cfg.Runner.Storage().Load(ctx, j.TenantID, j.RunID)
		if err != nil || snap == nil || snap.State == nil {
			s.markFailedAndFree(ctx, j, "snapshot missing or unreadable on recovery")
			continue
		}
		recoverable := false
		if s.cfg.RecoverChecker != nil {
			recoverable = s.cfg.RecoverChecker(snap.State)
		}
		if !recoverable {
			// For yandex, push through to teardown to avoid leaking VMs.
			// For docker, skip — containers are already gone.
			if snap.State.Provider == "yandex" {
				s.logger.Warn("scheduler: yandex run unreachable, running teardown to free cloud resources",
					zap.String("run", j.RunID))
				// Pre-mark non-teardown pending nodes as failed in the
				// snapshot so RecoverRun doesn't waste time waiting on dead
				// agents — only teardown (AlwaysRun) will fire.
				preFailNonTeardown(snap, s.cfg.MarkOnNotRecover)
				_ = s.cfg.Runner.Storage().Save(ctx, j.TenantID, j.RunID, snap)
				s.spawnRecover(j, snap)
				continue
			}
			s.markFailedAndFree(ctx, j, s.cfg.MarkOnNotRecover)
			continue
		}
		// Resume: leave the row in its current state, take over heartbeat
		// + spawn a worker that calls RecoverRun (not Start).
		s.spawnRecover(j, snap)
	}
	return nil
}

// stepTimeoutMinutes pulls step_timeout_min out of the suite policy
// snapshot. Returns 0 (no timeout) for ad-hoc runs and suites that don't
// configure one.
func stepTimeoutMinutes(rawPolicy []byte) int {
	if len(rawPolicy) == 0 {
		return 0
	}
	var p struct {
		StepTimeoutMin int `json:"step_timeout_min"`
	}
	_ = json.Unmarshal(rawPolicy, &p)
	if p.StepTimeoutMin < 0 {
		return 0
	}
	return p.StepTimeoutMin
}

// preFailNonTeardown converts every pending/running node in the snapshot to
// failed EXCEPT teardown phases — leaves teardown pending so DAG's
// AlwaysRun semantics fire it on the recovery RecoverRun path.
func preFailNonTeardown(snap *dag.Snapshot, msg string) {
	for i := range snap.Nodes {
		if snap.Nodes[i].ID == "teardown" {
			continue
		}
		if snap.Nodes[i].Status == dag.StatusPending || snap.Nodes[i].Status == dag.StatusRunning {
			snap.Nodes[i].Status = dag.StatusFailed
			if snap.Nodes[i].Error == "" {
				snap.Nodes[i].Error = msg
			}
		}
	}
}

func (s *Scheduler) markFailedAndFree(ctx context.Context, j postgres.JobRun, msg string) {
	if err := s.jobs.Finish(ctx, j.TenantID, j.RunID, postgres.JobStateFailed, msg); err != nil {
		s.logger.Warn("scheduler: finish on recovery failed", zap.String("run", j.RunID), zap.Error(err))
	}
	// Also write the failure into the snapshot so the UI sees the message.
	if snap, err := s.cfg.Runner.Storage().Load(ctx, j.TenantID, j.RunID); err == nil && snap != nil {
		changed := false
		for i := range snap.Nodes {
			if snap.Nodes[i].Status == dag.StatusPending || snap.Nodes[i].Status == dag.StatusRunning {
				snap.Nodes[i].Status = dag.StatusFailed
				snap.Nodes[i].Error = msg
				changed = true
			}
		}
		if changed {
			_ = s.cfg.Runner.Storage().Save(ctx, j.TenantID, j.RunID, snap)
		}
	}
}

func (s *Scheduler) spawnRecover(job postgres.JobRun, snap *dag.Snapshot) {
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.running[job.RunID] = cancel
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			s.mu.Lock()
			delete(s.running, job.RunID)
			s.mu.Unlock()
			cancel()
		}()

		// Take over claimed_by + heartbeat so other reapers don't revive.
		_, _ = s.pool.Exec(ctx, `
			UPDATE job_runs SET claimed_by=$3, heartbeat_at=NOW(), updated_at=NOW()
			WHERE run_id=$1 AND tenant_id=$2`, job.RunID, job.TenantID, s.cfg.InstanceID)

		hbCtx, hbCancel := context.WithCancel(ctx)
		go s.heartbeatLoop(hbCtx, job.TenantID, job.RunID)
		defer hbCancel()

		err := s.cfg.Runner.RecoverRun(ctx, job.TenantID, snap)
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				s.finishWith(job, postgres.JobStateCancelled, err.Error())
				return
			}
			s.finishWith(job, postgres.JobStateFailed, err.Error())
			return
		}
		s.finishWith(job, postgres.JobStateFinished, "")
	}()
}
