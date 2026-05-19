// Raw SQL is required here for SKIP LOCKED, which ratel does not (yet) support.
// All other database access in this binary goes through ratel — see CLAUDE.md.
package scheduler

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// Hook is dispatched on every fire of a Schedule whose hook_name matches the
// registry key. payload is the schedule's Any blob, typed by the hook.
type Hook func(ctx context.Context, payload *anypb.Any) error

// HookRegistry maps hook_name → Hook. Concurrency-safe.
type HookRegistry struct {
	mu    sync.RWMutex
	hooks map[string]Hook
}

func NewHookRegistry() *HookRegistry {
	return &HookRegistry{hooks: map[string]Hook{}}
}

// Register adds (or replaces) a hook implementation.
func (r *HookRegistry) Register(name string, h Hook) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooks[name] = h
}

// Lookup returns the registered hook for name, or false if absent.
func (r *HookRegistry) Lookup(name string) (Hook, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.hooks[name]
	return h, ok
}

// Config holds scheduler tuning parameters.
type Config struct {
	// Tick is how often the scheduler polls for due schedules.
	Tick time.Duration
	// LeaseDuration is how long a lease is held before another instance can steal it.
	LeaseDuration time.Duration
	// Batch is the maximum number of schedules claimed per tick.
	Batch int
}

func (c *Config) withDefaults() {
	if c.Tick == 0 {
		c.Tick = 5 * time.Second
	}
	if c.LeaseDuration == 0 {
		c.LeaseDuration = 30 * time.Second
	}
	if c.Batch == 0 {
		c.Batch = 10
	}
}

// Scheduler claims and fires due cron schedules with lease-based exactly-once
// semantics across a cluster.
type Scheduler struct {
	pool       *pgxpool.Pool
	sys        *system.Service
	hooks      *HookRegistry
	cfg        Config
	log        *zap.Logger
	instanceID string
}

// WithHooks installs the hook registry used by fire(). When unset (nil) the
// scheduler logs and skips dispatch for any due schedule.
func (s *Scheduler) WithHooks(reg *HookRegistry) *Scheduler {
	s.hooks = reg
	return s
}

// New creates a Scheduler. instanceID is derived from hostname + PID so that
// multiple processes in the same cluster can be distinguished.
func New(pool *pgxpool.Pool, sys *system.Service, cfg Config, log *zap.Logger) *Scheduler {
	cfg.withDefaults()
	hostname, _ := os.Hostname()
	instanceID := fmt.Sprintf("sched-%s-%d", hostname, os.Getpid())
	return &Scheduler{
		pool:       pool,
		sys:        sys,
		cfg:        cfg,
		log:        log,
		instanceID: instanceID,
	}
}

// Run starts the tick loop and blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	tick := time.NewTicker(s.cfg.Tick)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.tickOnce(ctx)
		}
	}
}

type leasedRow struct {
	id         string
	cronExpr   string
	nextFireAt time.Time
}

// tickOnce claims all due schedules and advances their next_fire_at.
func (s *Scheduler) tickOnce(ctx context.Context) {
	leaseSecs := int(s.cfg.LeaseDuration.Seconds())
	const q = `
WITH eligible AS (
    SELECT id FROM schedules
    WHERE next_fire_at <= now()
      AND (lease_owner = '' OR lease_expires_at IS NULL OR lease_expires_at < now())
      AND deleted_at IS NULL
      AND enabled = true
    ORDER BY next_fire_at
    FOR UPDATE SKIP LOCKED
    LIMIT $3
)
UPDATE schedules s
SET lease_owner      = $1,
    lease_expires_at = now() + ($2 || ' seconds')::interval,
    updated_at       = now()
FROM eligible
WHERE s.id = eligible.id
RETURNING s.id, s.cron_expr, s.next_fire_at;
`
	rows, err := s.pool.Query(ctx, q, s.instanceID, fmt.Sprintf("%d", leaseSecs), s.cfg.Batch)
	if err != nil {
		s.log.Error("scheduler: lease query failed", zap.Error(err))
		return
	}
	defer rows.Close()

	var leased []leasedRow
	for rows.Next() {
		var r leasedRow
		if err := rows.Scan(&r.id, &r.cronExpr, &r.nextFireAt); err != nil {
			s.log.Error("scheduler: scan lease row", zap.Error(err))
			return
		}
		leased = append(leased, r)
	}
	if err := rows.Err(); err != nil {
		s.log.Error("scheduler: rows error", zap.Error(err))
		return
	}
	rows.Close()

	for _, r := range leased {
		s.processRow(ctx, r)
	}
}

func (s *Scheduler) processRow(ctx context.Context, r leasedRow) {
	// fire is a stub for plan 03; dag-template bridge is plan 04+ scope.
	s.fire(ctx, r.id)

	next, err := computeNext(r.cronExpr, r.nextFireAt)
	if err != nil {
		s.log.Error("scheduler: computeNext failed",
			zap.String("schedule_id", r.id),
			zap.String("cron_expr", r.cronExpr),
			zap.Error(err),
		)
		// Release lease without advancing to avoid stale lock.
		s.releaseLease(ctx, r.id)
		return
	}

	const updateQ = `
UPDATE schedules
SET next_fire_at    = $1,
    last_fired_at   = now(),
    lease_owner     = '',
    lease_expires_at = NULL,
    updated_at      = now()
WHERE id = $2
`
	if _, err := s.pool.Exec(ctx, updateQ, next, r.id); err != nil {
		s.log.Error("scheduler: advance next_fire_at failed",
			zap.String("schedule_id", r.id),
			zap.Error(err),
		)
	}
}

// fire resolves the schedule, dispatches to the registered hook, and records
// failure state. Hook errors increment consecutive_failures + store last
// error; on success the row is left unchanged so the caller can advance
// last_fired_at / next_fire_at.
func (s *Scheduler) fire(ctx context.Context, scheduleID string) {
	sched, err := s.sys.GetSchedule(ctx, &systempb.ScheduleId{Value: scheduleID})
	if err != nil {
		s.log.Error("scheduler: GetSchedule before fire failed",
			zap.String("schedule_id", scheduleID), zap.Error(err))
		return
	}
	if s.hooks == nil {
		s.log.Warn("scheduler: no hook registry installed — skipping dispatch",
			zap.String("schedule_id", scheduleID),
			zap.String("hook_name", sched.GetHookName()))
		return
	}
	hook, ok := s.hooks.Lookup(sched.GetHookName())
	if !ok {
		s.recordFailure(ctx, scheduleID, fmt.Errorf("no hook registered for %q", sched.GetHookName()))
		return
	}
	if hookErr := hook(ctx, sched.GetPayload()); hookErr != nil {
		s.recordFailure(ctx, scheduleID, hookErr)
		s.log.Error("scheduler: hook failed",
			zap.String("schedule_id", scheduleID),
			zap.String("hook_name", sched.GetHookName()),
			zap.Error(hookErr))
		return
	}
	s.clearFailures(ctx, scheduleID)
	s.log.Info("scheduler fired",
		zap.String("schedule_id", scheduleID),
		zap.String("hook_name", sched.GetHookName()))
}

// recordFailure bumps consecutive_failures and stores last_failure_error.
func (s *Scheduler) recordFailure(ctx context.Context, scheduleID string, err error) {
	const q = `UPDATE schedules
SET consecutive_failures = consecutive_failures + 1,
    last_failure_error   = $1,
    updated_at           = now()
WHERE id = $2`
	if _, dbErr := s.pool.Exec(ctx, q, err.Error(), scheduleID); dbErr != nil {
		s.log.Error("scheduler: recordFailure failed",
			zap.String("schedule_id", scheduleID), zap.Error(dbErr))
	}
}

// clearFailures resets the consecutive_failures counter after a successful fire.
func (s *Scheduler) clearFailures(ctx context.Context, scheduleID string) {
	const q = `UPDATE schedules
SET consecutive_failures = 0,
    last_failure_error   = '',
    updated_at           = now()
WHERE id = $1 AND consecutive_failures > 0`
	if _, err := s.pool.Exec(ctx, q, scheduleID); err != nil {
		s.log.Warn("scheduler: clearFailures failed",
			zap.String("schedule_id", scheduleID), zap.Error(err))
	}
}

func (s *Scheduler) releaseLease(ctx context.Context, scheduleID string) {
	const q = `UPDATE schedules SET lease_owner = '', lease_expires_at = NULL, updated_at = now() WHERE id = $1`
	if _, err := s.pool.Exec(ctx, q, scheduleID); err != nil {
		s.log.Error("scheduler: release lease failed",
			zap.String("schedule_id", scheduleID),
			zap.Error(err),
		)
	}
}

// computeNext returns the next fire time after `after` for the given cron expression.
// Accepts standard 5-field or optional 6-field (with seconds) expressions.
func computeNext(expr string, after time.Time) (time.Time, error) {
	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	sched, err := parser.Parse(expr)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(after.UTC()), nil
}
