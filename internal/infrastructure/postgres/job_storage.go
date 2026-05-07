package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// JobState mirrors the CHECK constraint on job_runs.state. Lifecycle:
//
//	queued → claimed → running → finished
//	                          ↘ failed
//	                          ↘ cancelled
//
// `claimed` is a transient pre-start state — the scheduler writes the
// heartbeat row + reserves quota before spawning the worker goroutine.
// `running` is set by the worker once `app.Start` is in flight.
type JobState string

const (
	JobStateQueued    JobState = "queued"
	JobStateClaimed   JobState = "claimed"
	JobStateRunning   JobState = "running"
	JobStateFinished  JobState = "finished"
	JobStateFailed    JobState = "failed"
	JobStateCancelled JobState = "cancelled"
)

// JobCost is the in-flight resource footprint a single run holds while
// running. Units are deliberately additive (sum across all running jobs of
// a tenant gives the tenant's current `used`). Mirrors the JSON shape
// stored in job_runs.cost and quota_accounts.used.
type JobCost struct {
	CPUs        int `json:"cpus"`
	MemoryMB    int `json:"memory_mb"`
	DiskGB      int `json:"disk_gb"`
	VMCount     int `json:"vm_count"`
	RunsRunning int `json:"runs_running"`
}

// Add returns a + b. Used to compute "used after admit" for quota checks.
func (c JobCost) Add(o JobCost) JobCost {
	return JobCost{
		CPUs: c.CPUs + o.CPUs, MemoryMB: c.MemoryMB + o.MemoryMB,
		DiskGB: c.DiskGB + o.DiskGB, VMCount: c.VMCount + o.VMCount,
		RunsRunning: c.RunsRunning + o.RunsRunning,
	}
}

func (c JobCost) Sub(o JobCost) JobCost {
	r := JobCost{
		CPUs: c.CPUs - o.CPUs, MemoryMB: c.MemoryMB - o.MemoryMB,
		DiskGB: c.DiskGB - o.DiskGB, VMCount: c.VMCount - o.VMCount,
		RunsRunning: c.RunsRunning - o.RunsRunning,
	}
	// Used is a counter, never negative — guard against drift caused by
	// earlier double-frees so the tenant doesn't get stuck below zero.
	if r.CPUs < 0 {
		r.CPUs = 0
	}
	if r.MemoryMB < 0 {
		r.MemoryMB = 0
	}
	if r.DiskGB < 0 {
		r.DiskGB = 0
	}
	if r.VMCount < 0 {
		r.VMCount = 0
	}
	if r.RunsRunning < 0 {
		r.RunsRunning = 0
	}
	return r
}

// JobRun is one row of job_runs.
type JobRun struct {
	RunID       string
	TenantID    string
	BatchID     string
	SuiteID     string
	RunPresetID string
	Position    int
	State       JobState
	Config      json.RawMessage
	Cost        JobCost
	Priority    int
	NotBefore   *time.Time
	ClaimedBy   string
	ClaimedAt   *time.Time
	HeartbeatAt *time.Time
	StartedAt   *time.Time
	FinishedAt  *time.Time
	Error       string
	QuotaFreed  bool
	// SuitePolicy snapshots the parent suite's execution_policy at launch
	// time so the scheduler's claim path can gate sequential vs parallel
	// without joining suites every tick. Empty for ad-hoc (non-suite) jobs.
	SuitePolicy json.RawMessage
	// SuiteItemID points back to the suite_items row this job was launched
	// from. Stable across runs of the same item, so comparison can pair
	// runs across batches by item identity. Empty for ad-hoc runs.
	SuiteItemID string
	// Trigger records what enqueued this job: "manual" | "cron" | "api".
	// Empty for legacy rows.
	Trigger string
	// FireAt is the scheduled cron tick instant for cron-launched jobs.
	// Acts (with suite_id + db_preset_id + suite_item_id) as an idempotency
	// key against races where two servers fire the same tick concurrently.
	FireAt *time.Time
	// DBPresetID identifies the database axis cell of the matrix this job
	// was launched for. Empty for ad-hoc runs and pre-matrix suites.
	DBPresetID string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type JobStorage struct {
	pool *pgxpool.Pool
}

func NewJobStorage(pool *pgxpool.Pool) *JobStorage {
	return &JobStorage{pool: pool}
}

// Enqueue creates a new queued job row in its own transaction, also
// upserting the tenant's quota_accounts row so subsequent UPDATE …
// transactions never miss it.
func (s *JobStorage) Enqueue(ctx context.Context, j JobRun) error {
	cfgStr := string(j.Config)
	if cfgStr == "" {
		return errors.New("job storage: empty config")
	}
	costJSON, err := json.Marshal(j.Cost)
	if err != nil {
		return fmt.Errorf("job storage: marshal cost: %w", err)
	}
	policyStr := string(j.SuitePolicy)
	if policyStr == "" {
		policyStr = "{}"
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Idempotent quota row creation — covers tenants created before the
	// migration's seed ran and any race with tenant creation.
	_, err = tx.Exec(ctx, `
		INSERT INTO quota_accounts (tenant_id, used) VALUES ($1, '{}')
		ON CONFLICT (tenant_id) DO NOTHING`, j.TenantID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO job_runs (
			run_id, tenant_id, batch_id, suite_id, run_preset_id, position,
			state, config, cost, priority, not_before, suite_policy,
			suite_item_id, trigger, fire_at, db_preset_id,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,'queued',$7,$8,$9,$10,$11, $12,$13,$14,$15, NOW(), NOW())
		ON CONFLICT DO NOTHING`,
		j.RunID, j.TenantID, nullStr(j.BatchID), nullStr(j.SuiteID), nullStr(j.RunPresetID), j.Position,
		cfgStr, string(costJSON), j.Priority, j.NotBefore, policyStr,
		nullStr(j.SuiteItemID), nullStr(j.Trigger), j.FireAt, nullStr(j.DBPresetID),
	)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ListBySuite returns all jobs ever produced by the given suite, newest
// first. Replaces the legacy suite_runs join — every column needed lives
// on job_runs already.
func (s *JobStorage) ListBySuite(ctx context.Context, tenantID, suiteID string) ([]JobRun, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT run_id, tenant_id, COALESCE(batch_id,''), COALESCE(suite_id,''), COALESCE(run_preset_id,''),
		       position, state, config, cost, priority, not_before, COALESCE(suite_policy, '{}'),
		       COALESCE(claimed_by,''), claimed_at, heartbeat_at, started_at, finished_at,
		       COALESCE(error,''), quota_freed,
		       COALESCE(suite_item_id,''), COALESCE(trigger,''), fire_at,
		       COALESCE(db_preset_id,''),
		       created_at, updated_at
		FROM job_runs WHERE tenant_id=$1 AND suite_id=$2
		ORDER BY created_at DESC, position ASC`, tenantID, suiteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJobs(rows)
}

// Get returns a single job row by (tenant_id, run_id), or nil if missing.
func (s *JobStorage) Get(ctx context.Context, tenantID, runID string) (*JobRun, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT run_id, tenant_id, COALESCE(batch_id,''), COALESCE(suite_id,''), COALESCE(run_preset_id,''),
		       position, state, config, cost, priority, not_before, COALESCE(suite_policy, '{}'),
		       COALESCE(claimed_by,''), claimed_at, heartbeat_at, started_at, finished_at,
		       COALESCE(error,''), quota_freed,
		       COALESCE(suite_item_id,''), COALESCE(trigger,''), fire_at,
		       COALESCE(db_preset_id,''),
		       created_at, updated_at
		FROM job_runs WHERE run_id=$1 AND tenant_id=$2`, runID, tenantID)
	return scanJob(row)
}

// ListByBatch returns all jobs in a suite-launched batch ordered by position.
func (s *JobStorage) ListByBatch(ctx context.Context, tenantID, batchID string) ([]JobRun, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT run_id, tenant_id, COALESCE(batch_id,''), COALESCE(suite_id,''), COALESCE(run_preset_id,''),
		       position, state, config, cost, priority, not_before, COALESCE(suite_policy, '{}'),
		       COALESCE(claimed_by,''), claimed_at, heartbeat_at, started_at, finished_at,
		       COALESCE(error,''), quota_freed,
		       COALESCE(suite_item_id,''), COALESCE(trigger,''), fire_at,
		       COALESCE(db_preset_id,''),
		       created_at, updated_at
		FROM job_runs WHERE tenant_id=$1 AND batch_id=$2 ORDER BY position`, tenantID, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJobs(rows)
}

// ClaimNext atomically picks one queued job that fits the supplied per-tenant
// quota and transitions it to `claimed`, updating quota_accounts.used. Skips
// rows other servers have already locked. Returns nil, nil when nothing is
// claimable right now (queue empty or quota exhausted).
//
// The quota check is done in-transaction by computing used + cost vs limits
// passed in `limits`; this lets the caller resolve per-tenant limits from
// settings without leaking that lookup into storage.
func (s *JobStorage) ClaimNext(ctx context.Context, instanceID string, limits map[string]JobCost) (*JobRun, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// FOR UPDATE SKIP LOCKED ensures multiple schedulers don't fight over
	// the same row. ORDER BY enforces fairness within a tenant: highest
	// priority first, oldest first within priority.
	rows, err := tx.Query(ctx, `
		SELECT run_id, tenant_id, COALESCE(batch_id,''), COALESCE(suite_id,''), COALESCE(run_preset_id,''),
		       position, state, config, cost, priority, not_before, COALESCE(suite_policy, '{}'),
		       COALESCE(claimed_by,''), claimed_at, heartbeat_at, started_at, finished_at,
		       COALESCE(error,''), quota_freed,
		       COALESCE(suite_item_id,''), COALESCE(trigger,''), fire_at,
		       COALESCE(db_preset_id,''),
		       created_at, updated_at
		FROM job_runs
		WHERE state='queued' AND (not_before IS NULL OR not_before <= NOW())
		ORDER BY priority DESC, created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 50`)
	if err != nil {
		return nil, err
	}
	jobs, err := scanJobs(rows)
	rows.Close()
	if err != nil {
		return nil, err
	}

	for _, j := range jobs {
		// Skip rows for tenants the caller didn't supply limits for. The
		// scheduler builds `limits` from the set of tenants with queued
		// jobs, so a missing entry means "this tenant isn't ours" — relevant
		// when running multiple test suites against the same DB and useful
		// safety in multi-replica deployments where each scheduler scopes
		// to its tenant set.
		if _, ok := limits[j.TenantID]; !ok {
			continue
		}
		// Suite gating: read snapshotted policy and decide.
		// - sequential: position N requires every position<N to be terminal
		// - parallel:   admit up to MaxParallel concurrent jobs in batch
		// - on_step_fail=stop: any earlier step in failed/cancelled state
		//   short-circuits — mark this row cancelled instead of claiming.
		if j.BatchID != "" {
			policy := struct {
				Mode        string `json:"mode"`
				MaxParallel int    `json:"max_parallel"`
				OnStepFail  string `json:"on_step_fail"`
			}{Mode: "sequential", MaxParallel: 1, OnStepFail: "continue"}
			if len(j.SuitePolicy) > 0 {
				_ = json.Unmarshal(j.SuitePolicy, &policy)
			}

			if policy.OnStepFail == "stop" {
				var failedEarlier int
				if err := tx.QueryRow(ctx, `
					SELECT COUNT(*) FROM job_runs
					WHERE tenant_id=$1 AND batch_id=$2 AND position<$3
					  AND state IN ('failed','cancelled')`,
					j.TenantID, j.BatchID, j.Position).Scan(&failedEarlier); err != nil {
					return nil, err
				}
				if failedEarlier > 0 {
					if _, err := tx.Exec(ctx, `
						UPDATE job_runs SET state='cancelled', error='earlier step failed (on_step_fail=stop)',
						    finished_at=NOW(), updated_at=NOW(), quota_freed=TRUE
						WHERE run_id=$1 AND tenant_id=$2 AND state='queued'`,
						j.RunID, j.TenantID); err != nil {
						return nil, err
					}
					// Commit the cancellation immediately — otherwise the
					// outer tx.Rollback (which fires once the iteration
					// finds nothing claimable) would discard it. After
					// commit we re-open a fresh tx and continue scanning,
					// so other queued rows in the same batch can also get
					// cancelled in this pass.
					if err := tx.Commit(ctx); err != nil {
						return nil, err
					}
					tx, err = s.pool.Begin(ctx)
					if err != nil {
						return nil, err
					}
					// rebind defer so the new tx rolls back if the rest
					// of the loop bails. Old defer is moot — old tx is
					// already committed and Rollback is a no-op on it.
					defer tx.Rollback(ctx)
					continue
				}
			}

			if policy.Mode == "parallel" {
				var inflight int
				if err := tx.QueryRow(ctx, `
					SELECT COUNT(*) FROM job_runs
					WHERE tenant_id=$1 AND batch_id=$2
					  AND state IN ('claimed','running')`,
					j.TenantID, j.BatchID).Scan(&inflight); err != nil {
					return nil, err
				}
				if policy.MaxParallel > 0 && inflight >= policy.MaxParallel {
					continue
				}
			} else if j.Position > 0 {
				// Sequential default — block until all earlier terminal.
				var blockers int
				if err := tx.QueryRow(ctx, `
					SELECT COUNT(*) FROM job_runs
					WHERE tenant_id=$1 AND batch_id=$2 AND position<$3
					  AND state IN ('queued','claimed','running')`,
					j.TenantID, j.BatchID, j.Position).Scan(&blockers); err != nil {
					return nil, err
				}
				if blockers > 0 {
					continue
				}
			}
		}

		// Quota check: load tenant's used, compute used+cost, compare to
		// the limits the caller resolved.
		used, err := getUsedTx(ctx, tx, j.TenantID)
		if err != nil {
			return nil, err
		}
		next := used.Add(j.Cost)
		next.RunsRunning = used.RunsRunning + 1
		lim := limits[j.TenantID]
		if !fits(next, lim) {
			continue
		}

		// Atomic claim + quota write.
		nextJSON, _ := json.Marshal(next)
		if _, err := tx.Exec(ctx, `
			UPDATE quota_accounts SET used=$2, updated_at=NOW() WHERE tenant_id=$1`,
			j.TenantID, string(nextJSON)); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE job_runs
			SET state='claimed', claimed_by=$3, claimed_at=NOW(), heartbeat_at=NOW(), updated_at=NOW()
			WHERE run_id=$1 AND tenant_id=$2 AND state='queued'`,
			j.RunID, j.TenantID, instanceID); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		now := time.Now()
		j.State = JobStateClaimed
		j.ClaimedBy = instanceID
		j.ClaimedAt = &now
		j.HeartbeatAt = &now
		return &j, nil
	}
	return nil, nil
}

// MarkRunning flips claimed → running. Called by the worker right before
// it dispatches `app.Start`.
func (s *JobStorage) MarkRunning(ctx context.Context, tenantID, runID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE job_runs SET state='running', started_at=NOW(), heartbeat_at=NOW(), updated_at=NOW()
		WHERE run_id=$1 AND tenant_id=$2 AND state='claimed'`, runID, tenantID)
	return err
}

// Heartbeat updates the worker's liveness timestamp. The reaper revives any
// claimed/running row whose heartbeat is older than its threshold.
func (s *JobStorage) Heartbeat(ctx context.Context, tenantID, runID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE job_runs SET heartbeat_at=NOW() WHERE run_id=$1 AND tenant_id=$2`,
		runID, tenantID)
	return err
}

// Finish transitions to a terminal state and frees the tenant's quota.
// Idempotent on quota_freed so double-frees from concurrent paths don't
// underflow the counter.
func (s *JobStorage) Finish(ctx context.Context, tenantID, runID string, state JobState, errMsg string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var (
		costStr     string
		alreadyFree bool
	)
	if err := tx.QueryRow(ctx, `
		SELECT cost, quota_freed FROM job_runs
		WHERE run_id=$1 AND tenant_id=$2 FOR UPDATE`, runID, tenantID,
	).Scan(&costStr, &alreadyFree); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if !alreadyFree {
		var cost JobCost
		_ = json.Unmarshal([]byte(costStr), &cost)
		used, err := getUsedTx(ctx, tx, tenantID)
		if err != nil {
			return err
		}
		freed := used.Sub(cost)
		if freed.RunsRunning >= 1 {
			freed.RunsRunning = used.RunsRunning - 1
			if freed.RunsRunning < 0 {
				freed.RunsRunning = 0
			}
		}
		freedJSON, _ := json.Marshal(freed)
		if _, err := tx.Exec(ctx, `
			UPDATE quota_accounts SET used=$2, updated_at=NOW() WHERE tenant_id=$1`,
			tenantID, string(freedJSON)); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE job_runs SET state=$3, error=$4, finished_at=NOW(), updated_at=NOW(), quota_freed=TRUE
		WHERE run_id=$1 AND tenant_id=$2`, runID, tenantID, string(state), errMsg); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CancelQueued marks a queued job cancelled without spinning anything up.
// Returns true if a row was actually updated. Worker-running jobs need
// runtime cancel via the scheduler — this helper is for queued-only.
func (s *JobStorage) CancelQueued(ctx context.Context, tenantID, runID string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE job_runs SET state='cancelled', finished_at=NOW(), updated_at=NOW()
		WHERE run_id=$1 AND tenant_id=$2 AND state='queued'`, runID, tenantID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ReviveStale flips claimed/running rows whose worker died back to queued
// and frees their reserved quota so a fresh claim can take them. The
// "dead" condition is two-pronged: the row's heartbeat is older than
// staleAfter AND the owning server_instance is itself missing or stale.
//
// The server-instance check is what protects against the
// duplicate-machines bug — without it, a momentary heartbeat lag (the
// worker is busy doing a long terraform apply, didn't get a chance to
// hit the heartbeat goroutine for staleAfter seconds) was enough for the
// reaper to revive the row, the scheduler to pick it up, and a SECOND
// worker to start applying terraform on top. Both workers then created
// duplicate cloud resources.
func (s *JobStorage) ReviveStale(ctx context.Context, staleAfter time.Duration) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT j.run_id, j.tenant_id, j.cost, j.quota_freed FROM job_runs j
		LEFT JOIN server_instances si ON si.id = j.claimed_by
		WHERE j.state IN ('claimed','running')
		  AND j.heartbeat_at < NOW() - ($1 * INTERVAL '1 second')
		  AND (
		    -- claimed_by is unset (legacy rows / claim race) → revive.
		    j.claimed_by IS NULL
		    -- owner server row was deleted (clean shutdown) → revive.
		    OR si.id IS NULL
		    -- owner server explicitly stopping → revive.
		    OR si.stopping = TRUE
		    -- owner server itself missed its instance heartbeat → revive.
		    OR si.heartbeat_at < NOW() - ($1 * INTERVAL '1 second')
		  )
		FOR UPDATE SKIP LOCKED`, staleAfter.Seconds())
	if err != nil {
		return 0, err
	}
	type stale struct {
		runID, tenantID, costStr string
		freed                    bool
	}
	var collected []stale
	for rows.Next() {
		var s stale
		if err := rows.Scan(&s.runID, &s.tenantID, &s.costStr, &s.freed); err != nil {
			rows.Close()
			return 0, err
		}
		collected = append(collected, s)
	}
	rows.Close()

	for _, s := range collected {
		if !s.freed {
			var cost JobCost
			_ = json.Unmarshal([]byte(s.costStr), &cost)
			used, err := getUsedTx(ctx, tx, s.tenantID)
			if err != nil {
				return 0, err
			}
			freed := used.Sub(cost)
			if used.RunsRunning > 0 {
				freed.RunsRunning = used.RunsRunning - 1
			}
			freedJSON, _ := json.Marshal(freed)
			if _, err := tx.Exec(ctx, `
				UPDATE quota_accounts SET used=$2, updated_at=NOW() WHERE tenant_id=$1`,
				s.tenantID, string(freedJSON)); err != nil {
				return 0, err
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE job_runs
			SET state='queued', claimed_by=NULL, claimed_at=NULL, heartbeat_at=NULL,
			    quota_freed=FALSE, updated_at=NOW()
			WHERE run_id=$1 AND tenant_id=$2`, s.runID, s.tenantID); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(collected), nil
}

// ListInflight returns every claimed/running job — used by RecoverOnStartup
// to decide what to do with each (resume / mark failed / requeue).
func (s *JobStorage) ListInflight(ctx context.Context) ([]JobRun, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT run_id, tenant_id, COALESCE(batch_id,''), COALESCE(suite_id,''), COALESCE(run_preset_id,''),
		       position, state, config, cost, priority, not_before, COALESCE(suite_policy, '{}'),
		       COALESCE(claimed_by,''), claimed_at, heartbeat_at, started_at, finished_at,
		       COALESCE(error,''), quota_freed,
		       COALESCE(suite_item_id,''), COALESCE(trigger,''), fire_at,
		       COALESCE(db_preset_id,''),
		       created_at, updated_at
		FROM job_runs WHERE state IN ('claimed','running')
		`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJobs(rows)
}

// CountQueued reports how many queued jobs exist for a tenant. Used by
// the API to surface queue depth to clients.
func (s *JobStorage) CountQueued(ctx context.Context, tenantID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM job_runs WHERE tenant_id=$1 AND state='queued'`, tenantID).Scan(&n)
	return n, err
}

// GetUsed returns current quota usage for a tenant (used for `/quotas`
// surface).
func (s *JobStorage) GetUsed(ctx context.Context, tenantID string) (JobCost, error) {
	var s2 string
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(used, '{}') FROM quota_accounts WHERE tenant_id=$1`, tenantID).Scan(&s2)
	if errors.Is(err, pgx.ErrNoRows) {
		return JobCost{}, nil
	}
	if err != nil {
		return JobCost{}, err
	}
	var c JobCost
	_ = json.Unmarshal([]byte(s2), &c)
	return c, nil
}

// --- helpers ---

func getUsedTx(ctx context.Context, tx pgx.Tx, tenantID string) (JobCost, error) {
	var s string
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(used,'{}') FROM quota_accounts WHERE tenant_id=$1 FOR UPDATE`, tenantID,
	).Scan(&s)
	if errors.Is(err, pgx.ErrNoRows) {
		// Race with a parallel tenant create — insert empty row and retry once.
		if _, err := tx.Exec(ctx, `
			INSERT INTO quota_accounts (tenant_id, used) VALUES ($1, '{}')
			ON CONFLICT (tenant_id) DO NOTHING`, tenantID); err != nil {
			return JobCost{}, err
		}
		return JobCost{}, nil
	}
	if err != nil {
		return JobCost{}, err
	}
	var c JobCost
	_ = json.Unmarshal([]byte(s), &c)
	return c, nil
}

func fits(used, lim JobCost) bool {
	if lim.CPUs > 0 && used.CPUs > lim.CPUs {
		return false
	}
	if lim.MemoryMB > 0 && used.MemoryMB > lim.MemoryMB {
		return false
	}
	if lim.DiskGB > 0 && used.DiskGB > lim.DiskGB {
		return false
	}
	if lim.VMCount > 0 && used.VMCount > lim.VMCount {
		return false
	}
	if lim.RunsRunning > 0 && used.RunsRunning > lim.RunsRunning {
		return false
	}
	return true
}

func scanJob(row pgx.Row) (*JobRun, error) {
	var (
		j         JobRun
		costStr   string
		policyStr string
		notBef    *time.Time
		claimAt   *time.Time
		hbAt      *time.Time
		startAt   *time.Time
		finishAt  *time.Time
		fireAt    *time.Time
		state     string
	)
	err := row.Scan(
		&j.RunID, &j.TenantID, &j.BatchID, &j.SuiteID, &j.RunPresetID,
		&j.Position, &state, &j.Config, &costStr, &j.Priority, &notBef, &policyStr,
		&j.ClaimedBy, &claimAt, &hbAt, &startAt, &finishAt,
		&j.Error, &j.QuotaFreed,
		&j.SuiteItemID, &j.Trigger, &fireAt,
		&j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	j.State = JobState(state)
	j.NotBefore = notBef
	j.ClaimedAt = claimAt
	j.HeartbeatAt = hbAt
	j.StartedAt = startAt
	j.FinishedAt = finishAt
	j.FireAt = fireAt
	_ = json.Unmarshal([]byte(costStr), &j.Cost)
	if policyStr != "" {
		j.SuitePolicy = json.RawMessage(policyStr)
	}
	return &j, nil
}

func scanJobs(rows pgx.Rows) ([]JobRun, error) {
	var out []JobRun
	for rows.Next() {
		var (
			j         JobRun
			costStr   string
			policyStr string
			notBef    *time.Time
			claimAt   *time.Time
			hbAt      *time.Time
			startAt   *time.Time
			finishAt  *time.Time
			fireAt    *time.Time
			state     string
		)
		if err := rows.Scan(
			&j.RunID, &j.TenantID, &j.BatchID, &j.SuiteID, &j.RunPresetID,
			&j.Position, &state, &j.Config, &costStr, &j.Priority, &notBef, &policyStr,
			&j.ClaimedBy, &claimAt, &hbAt, &startAt, &finishAt,
			&j.Error, &j.QuotaFreed,
			&j.SuiteItemID, &j.Trigger, &fireAt,
			&j.DBPresetID,
			&j.CreatedAt, &j.UpdatedAt,
		); err != nil {
			return nil, err
		}
		j.State = JobState(state)
		j.NotBefore = notBef
		j.ClaimedAt = claimAt
		j.HeartbeatAt = hbAt
		j.StartedAt = startAt
		j.FinishedAt = finishAt
		j.FireAt = fireAt
		_ = json.Unmarshal([]byte(costStr), &j.Cost)
		if policyStr != "" {
			j.SuitePolicy = json.RawMessage(policyStr)
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
