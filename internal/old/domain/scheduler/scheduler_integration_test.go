package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	"github.com/stroppy-io/stroppy-cloud/internal/old_core/dag"
)

// fakeRunner records every Start/RecoverRun call and lets each test inject
// success/failure + delays. Replaces api.App in tests so we can exercise
// the scheduler without standing up the full HTTP server.
type fakeRunner struct {
	mu        sync.Mutex
	starts    []string
	recovers  []string
	startErr  error
	startWait time.Duration
	storage   *fakeStorage
}

func (f *fakeRunner) Start(ctx context.Context, tenantID string, cfg types.RunConfig) error {
	f.mu.Lock()
	f.starts = append(f.starts, cfg.ID)
	wait := f.startWait
	err := f.startErr
	f.mu.Unlock()
	if wait > 0 {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}
func (f *fakeRunner) RecoverRun(ctx context.Context, tenantID string, snap *dag.Snapshot) error {
	f.mu.Lock()
	f.recovers = append(f.recovers, "recover")
	f.mu.Unlock()
	return nil
}
func (f *fakeRunner) Storage() dag.Storage { return f.storage }

// fakeStorage implements dag.Storage with in-memory snapshots so the
// scheduler's recovery path can locate runs.
type fakeStorage struct {
	mu   sync.Mutex
	snap map[string]*dag.Snapshot
}

func (f *fakeStorage) Save(ctx context.Context, tenantID, id string, snap *dag.Snapshot) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.snap == nil {
		f.snap = make(map[string]*dag.Snapshot)
	}
	f.snap[id] = snap
	return nil
}
func (f *fakeStorage) Load(ctx context.Context, tenantID, id string) (*dag.Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.snap[id], nil
}
func (f *fakeStorage) List(ctx context.Context, tenantID string) ([]dag.RunSummary, error) {
	return nil, nil
}
func (f *fakeStorage) Delete(ctx context.Context, tenantID, id string) error { return nil }
func (f *fakeStorage) SetBaseline(ctx context.Context, tenantID, name, runID string) error {
	return nil
}
func (f *fakeStorage) GetBaseline(ctx context.Context, tenantID, name string) (string, error) {
	return "", nil
}
func (f *fakeStorage) ListBaselines(ctx context.Context, tenantID string) (map[string]string, error) {
	return nil, nil
}

func mustPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping scheduler integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func mustTenant(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	id := "tenant-sch-" + uuid.New().String()[:8]
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO tenants(id, name) VALUES ($1, $2)`, id, id); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM job_runs WHERE tenant_id=$1`, id)
		_, _ = pool.Exec(context.Background(), `DELETE FROM quota_accounts WHERE tenant_id=$1`, id)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1`, id)
	})
	return id
}

// TestScheduler_AdmitsAndCompletesQueuedJob walks the full happy path:
// enqueue → scheduler claims via its own loop → fakeRunner.Start succeeds
// → row transitions to finished, quota released, in-memory map cleared.
func TestScheduler_AdmitsAndCompletesQueuedJob(t *testing.T) {
	pool := mustPool(t)
	tenantID := mustTenant(t, pool)
	runner := &fakeRunner{storage: &fakeStorage{}}

	sch := New(Config{
		InstanceID:        uuid.New().String(),
		Logger:            zap.NewNop(),
		Pool:              pool,
		Runner:            runner,
		SettingsResolver:  func(_ string) *types.ServerSettings { return &types.ServerSettings{} },
		RecoverChecker:    func(_ *dag.RunState) bool { return true },
		ClaimInterval:     50 * time.Millisecond,
		HeartbeatInterval: 200 * time.Millisecond,
		StaleAfter:        2 * time.Second,
		ReaperInterval:    500 * time.Millisecond,
		InstanceHeartbeat: 200 * time.Millisecond,
	})

	if err := sch.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sch.Stop(context.Background())

	runID := "run-sch-" + uuid.New().String()[:6]
	cfg, _ := json.Marshal(map[string]any{"id": runID, "provider": "docker"})
	err := sch.Jobs().Enqueue(context.Background(), postgres.JobRun{
		RunID: runID, TenantID: tenantID, Config: cfg,
		Cost: postgres.JobCost{CPUs: 1, RunsRunning: 1},
	})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// Wait up to 5s for the scheduler to pick + finish.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		j, _ := sch.Jobs().Get(context.Background(), tenantID, runID)
		if j != nil && j.State == postgres.JobStateFinished {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	got, _ := sch.Jobs().Get(context.Background(), tenantID, runID)
	if got == nil || got.State != postgres.JobStateFinished {
		t.Fatalf("final state = %v, want finished", got)
	}

	runner.mu.Lock()
	defer runner.mu.Unlock()
	if len(runner.starts) != 1 || runner.starts[0] != runID {
		t.Errorf("starts = %v, want [%s]", runner.starts, runID)
	}

	used, _ := sch.Jobs().GetUsed(context.Background(), tenantID)
	if used.RunsRunning != 0 || used.CPUs != 0 {
		t.Errorf("quota leak after finish: %+v", used)
	}
}

// TestScheduler_FailedJobMarksFailedAndFreesQuota: when fakeRunner returns
// an error, scheduler must transition the row to failed (not finished) and
// still release the reserved quota.
func TestScheduler_FailedJobMarksFailedAndFreesQuota(t *testing.T) {
	pool := mustPool(t)
	tenantID := mustTenant(t, pool)
	runner := &fakeRunner{storage: &fakeStorage{}, startErr: errors.New("synthetic failure")}

	sch := New(Config{
		InstanceID:        uuid.New().String(),
		Logger:            zap.NewNop(),
		Pool:              pool,
		Runner:            runner,
		SettingsResolver:  func(_ string) *types.ServerSettings { return &types.ServerSettings{} },
		RecoverChecker:    func(_ *dag.RunState) bool { return false },
		ClaimInterval:     50 * time.Millisecond,
		HeartbeatInterval: 200 * time.Millisecond,
		StaleAfter:        2 * time.Second,
		ReaperInterval:    500 * time.Millisecond,
		InstanceHeartbeat: 200 * time.Millisecond,
	})
	if err := sch.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sch.Stop(context.Background())

	cfg, _ := json.Marshal(map[string]any{"id": "run-x"})
	runID := "run-fail-" + uuid.New().String()[:6]
	if err := sch.Jobs().Enqueue(context.Background(), postgres.JobRun{
		RunID: runID, TenantID: tenantID, Config: cfg,
		Cost: postgres.JobCost{CPUs: 2, RunsRunning: 1},
	}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		j, _ := sch.Jobs().Get(context.Background(), tenantID, runID)
		if j != nil && (j.State == postgres.JobStateFailed || j.State == postgres.JobStateFinished) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	got, _ := sch.Jobs().Get(context.Background(), tenantID, runID)
	if got == nil || got.State != postgres.JobStateFailed {
		t.Fatalf("final state = %v, want failed", got)
	}
	if got.Error == "" {
		t.Errorf("expected error message in failed row")
	}
	used, _ := sch.Jobs().GetUsed(context.Background(), tenantID)
	if used.CPUs != 0 {
		t.Errorf("quota leak after failure: %+v", used)
	}
}

// TestScheduler_RecoversInflightOnStartup: an "old" server process leaves
// a row in state=running. Booting a new scheduler with RecoverChecker→true
// and a snapshot in storage must drive that row to terminal via
// fakeRunner.RecoverRun (not Start).
func TestScheduler_RecoversInflightOnStartup(t *testing.T) {
	pool := mustPool(t)
	tenantID := mustTenant(t, pool)
	storage := &fakeStorage{}
	runID := "run-recov-" + uuid.New().String()[:6]
	// Pre-seed snapshot so RecoverRun has something to load.
	_ = storage.Save(context.Background(), tenantID, runID, &dag.Snapshot{
		State: &dag.RunState{Provider: "docker", RunConfig: json.RawMessage(`{"id":"x"}`)},
		Nodes: []dag.NodeStatus{{ID: "network", Status: dag.StatusDone}},
	})
	runner := &fakeRunner{storage: storage}

	// Pre-seed an inflight job_run as if a previous server claimed it.
	cfg, _ := json.Marshal(map[string]any{"id": runID})
	costJSON, _ := json.Marshal(postgres.JobCost{CPUs: 1, RunsRunning: 1})
	_, err := pool.Exec(context.Background(), `
		INSERT INTO job_runs (run_id, tenant_id, state, config, cost, claimed_by, claimed_at, heartbeat_at, started_at)
		VALUES ($1,$2,'running',$3,$4,'old-instance', NOW(), NOW(), NOW())`,
		runID, tenantID, string(cfg), string(costJSON))
	if err != nil {
		t.Fatalf("seed running: %v", err)
	}

	sch := New(Config{
		InstanceID:        uuid.New().String(),
		Logger:            zap.NewNop(),
		Pool:              pool,
		Runner:            runner,
		SettingsResolver:  func(_ string) *types.ServerSettings { return &types.ServerSettings{} },
		RecoverChecker:    func(_ *dag.RunState) bool { return true },
		ClaimInterval:     50 * time.Millisecond,
		HeartbeatInterval: 200 * time.Millisecond,
		StaleAfter:        2 * time.Second,
		ReaperInterval:    500 * time.Millisecond,
		InstanceHeartbeat: 200 * time.Millisecond,
	})
	if err := sch.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sch.Stop(context.Background())

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		j, _ := sch.Jobs().Get(context.Background(), tenantID, runID)
		if j != nil && j.State == postgres.JobStateFinished {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	got, _ := sch.Jobs().Get(context.Background(), tenantID, runID)
	if got == nil || got.State != postgres.JobStateFinished {
		t.Fatalf("after recovery: state = %+v, want finished", got)
	}
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if len(runner.recovers) == 0 {
		t.Errorf("RecoverRun not called; runner=%+v", runner)
	}
}

// TestScheduler_QuotaQueuesSecondJob: with a tight CPU limit, a second
// concurrent enqueue must wait until the first finishes before claiming.
func TestScheduler_QuotaQueuesSecondJob(t *testing.T) {
	pool := mustPool(t)
	tenantID := mustTenant(t, pool)
	runner := &fakeRunner{storage: &fakeStorage{}, startWait: 700 * time.Millisecond}

	sch := New(Config{
		InstanceID: uuid.New().String(),
		Logger:     zap.NewNop(),
		Pool:       pool,
		Runner:     runner,
		SettingsResolver: func(_ string) *types.ServerSettings {
			return &types.ServerSettings{Quotas: types.TenantQuotas{
				MaxConcurrentCPUs: 4, MaxConcurrentRuns: 5,
			}}
		},
		RecoverChecker:    func(_ *dag.RunState) bool { return true },
		ClaimInterval:     50 * time.Millisecond,
		HeartbeatInterval: 200 * time.Millisecond,
		StaleAfter:        2 * time.Second,
		ReaperInterval:    500 * time.Millisecond,
		InstanceHeartbeat: 200 * time.Millisecond,
	})
	if err := sch.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sch.Stop(context.Background())

	cfg, _ := json.Marshal(map[string]any{"id": "x"})
	r1 := "run-q1-" + uuid.New().String()[:6]
	r2 := "run-q2-" + uuid.New().String()[:6]
	for _, id := range []string{r1, r2} {
		if err := sch.Jobs().Enqueue(context.Background(), postgres.JobRun{
			RunID: id, TenantID: tenantID, Config: cfg,
			Cost: postgres.JobCost{CPUs: 4, RunsRunning: 1},
		}); err != nil {
			t.Fatalf("enqueue %s: %v", id, err)
		}
	}

	// Wait for first to finish.
	deadline := time.Now().Add(5 * time.Second)
	var observedSecondQueued atomic.Bool
	for time.Now().Before(deadline) {
		j1, _ := sch.Jobs().Get(context.Background(), tenantID, r1)
		j2, _ := sch.Jobs().Get(context.Background(), tenantID, r2)
		// While first is running, second must remain queued.
		if j1 != nil && (j1.State == postgres.JobStateRunning || j1.State == postgres.JobStateClaimed) {
			if j2 == nil || j2.State == postgres.JobStateQueued {
				observedSecondQueued.Store(true)
			}
		}
		if j1 != nil && j1.State == postgres.JobStateFinished &&
			j2 != nil && j2.State == postgres.JobStateFinished {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !observedSecondQueued.Load() {
		t.Errorf("second run was never observed in queued state — quota gate not enforcing")
	}
}
