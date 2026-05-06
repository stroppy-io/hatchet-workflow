package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// JobStorage tests rely on a real Postgres because the load-bearing logic
// is the SQL itself: FOR UPDATE SKIP LOCKED, transactional quota math,
// suite-policy gating predicates. Each test seeds a unique synthetic
// tenant + run prefix to stay isolated from other suites running against
// the same DB.

func newTestTenant(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	id := "tenant-" + uuid.New().String()[:8]
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO tenants(id, name) VALUES ($1, $2)`, id, id); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM job_runs WHERE tenant_id=$1`, id)
		_, _ = pool.Exec(context.Background(), `DELETE FROM quota_accounts WHERE tenant_id=$1`, id)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1`, id)
	})
	return id
}

func sampleJob(tenantID, runID string, cost JobCost) JobRun {
	cfg, _ := json.Marshal(map[string]any{"id": runID, "provider": "docker"})
	return JobRun{
		RunID: runID, TenantID: tenantID,
		Config: cfg, Cost: cost,
	}
}

func TestJobStorage_EnqueueClaimFinishLifecycle(t *testing.T) {
	pool := testPool(t)
	st := NewJobStorage(pool)
	tenantID := newTestTenant(t, pool)
	runID := "run-" + uuid.New().String()[:8]
	ctx := context.Background()

	cost := JobCost{CPUs: 4, MemoryMB: 1024, DiskGB: 10, VMCount: 1}
	if err := st.Enqueue(ctx, sampleJob(tenantID, runID, cost)); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	got, err := st.Get(ctx, tenantID, runID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || got.State != JobStateQueued {
		t.Fatalf("expected queued, got %+v", got)
	}

	// No limits → unlimited.
	limits := map[string]JobCost{tenantID: {}}
	claimed, err := st.ClaimNext(ctx, "test-instance", limits)
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if claimed == nil || claimed.RunID != runID || claimed.State != JobStateClaimed {
		t.Fatalf("ClaimNext returned %+v", claimed)
	}
	if claimed.ClaimedBy != "test-instance" {
		t.Errorf("claimed_by = %q, want test-instance", claimed.ClaimedBy)
	}

	// quota_accounts.used should now reflect the claimed cost.
	used, err := st.GetUsed(ctx, tenantID)
	if err != nil {
		t.Fatalf("GetUsed: %v", err)
	}
	if used.CPUs != 4 || used.RunsRunning != 1 {
		t.Errorf("used after claim = %+v, want CPUs=4 runs=1", used)
	}

	if err := st.MarkRunning(ctx, tenantID, runID); err != nil {
		t.Fatalf("MarkRunning: %v", err)
	}

	if err := st.Finish(ctx, tenantID, runID, JobStateFinished, ""); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	used, _ = st.GetUsed(ctx, tenantID)
	if used.CPUs != 0 || used.RunsRunning != 0 {
		t.Errorf("used after finish = %+v, want zeroed", used)
	}

	got, _ = st.Get(ctx, tenantID, runID)
	if got.State != JobStateFinished {
		t.Errorf("state after finish = %s, want finished", got.State)
	}
}

func TestJobStorage_QuotaBlocksAdmission(t *testing.T) {
	pool := testPool(t)
	st := NewJobStorage(pool)
	tenantID := newTestTenant(t, pool)
	ctx := context.Background()

	// Two queued jobs, each costs 4 CPUs. Tenant cap is 4 CPUs total —
	// only one should claim.
	cost := JobCost{CPUs: 4, RunsRunning: 1}
	for i := 0; i < 2; i++ {
		runID := fmt.Sprintf("run-quota-%d-%s", i, uuid.New().String()[:6])
		if err := st.Enqueue(ctx, sampleJob(tenantID, runID, cost)); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
	}

	limits := map[string]JobCost{tenantID: {CPUs: 4, RunsRunning: 2}}
	first, err := st.ClaimNext(ctx, "i1", limits)
	if err != nil || first == nil {
		t.Fatalf("first claim: %v %+v", err, first)
	}
	second, err := st.ClaimNext(ctx, "i1", limits)
	if err != nil {
		t.Fatalf("second claim err: %v", err)
	}
	if second != nil {
		t.Fatalf("second claim should be blocked by CPUs quota, got %+v", second)
	}

	// Free the first; second should now admit.
	if err := st.Finish(ctx, tenantID, first.RunID, JobStateFinished, ""); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	second, err = st.ClaimNext(ctx, "i1", limits)
	if err != nil || second == nil {
		t.Fatalf("after finish, second claim should succeed: %v %+v", err, second)
	}
}

func TestJobStorage_SequentialSuiteGating(t *testing.T) {
	pool := testPool(t)
	st := NewJobStorage(pool)
	tenantID := newTestTenant(t, pool)
	ctx := context.Background()

	batchID := uuid.New().String()
	for i := 0; i < 3; i++ {
		runID := fmt.Sprintf("run-seq-%d-%s", i, uuid.New().String()[:6])
		j := sampleJob(tenantID, runID, JobCost{CPUs: 1, RunsRunning: 1})
		j.BatchID = batchID
		j.SuiteID = "suite-x"
		j.Position = i
		j.SuitePolicy = json.RawMessage(`{"mode":"sequential","max_parallel":1,"on_step_fail":"continue"}`)
		if err := st.Enqueue(ctx, j); err != nil {
			t.Fatalf("Enqueue %d: %v", i, err)
		}
	}

	// Position 0 first.
	j0, _ := st.ClaimNext(ctx, "i", map[string]JobCost{tenantID: {}})
	if j0 == nil || j0.Position != 0 {
		t.Fatalf("first claim position = %v, want 0", j0)
	}
	// Positions 1 + 2 must NOT be claimable while 0 is in flight.
	j1, _ := st.ClaimNext(ctx, "i", map[string]JobCost{tenantID: {}})
	if j1 != nil {
		t.Fatalf("position 1 claimed while 0 in flight: %+v", j1)
	}
	// Finish 0; 1 should now claim.
	_ = st.Finish(ctx, tenantID, j0.RunID, JobStateFinished, "")
	j1, _ = st.ClaimNext(ctx, "i", map[string]JobCost{tenantID: {}})
	if j1 == nil || j1.Position != 1 {
		t.Fatalf("position 1 should claim after 0 done: %+v", j1)
	}
}

func TestJobStorage_ParallelSuiteRespectsMaxParallel(t *testing.T) {
	pool := testPool(t)
	st := NewJobStorage(pool)
	tenantID := newTestTenant(t, pool)
	ctx := context.Background()

	batchID := uuid.New().String()
	for i := 0; i < 4; i++ {
		runID := fmt.Sprintf("run-par-%d-%s", i, uuid.New().String()[:6])
		j := sampleJob(tenantID, runID, JobCost{CPUs: 1, RunsRunning: 1})
		j.BatchID = batchID
		j.SuiteID = "suite-p"
		j.Position = i
		j.SuitePolicy = json.RawMessage(`{"mode":"parallel","max_parallel":2,"on_step_fail":"continue"}`)
		if err := st.Enqueue(ctx, j); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
	}

	limits := map[string]JobCost{tenantID: {}}
	a, _ := st.ClaimNext(ctx, "i", limits)
	b, _ := st.ClaimNext(ctx, "i", limits)
	c, _ := st.ClaimNext(ctx, "i", limits)
	if a == nil || b == nil {
		t.Fatalf("expected at least 2 parallel claims, got a=%v b=%v", a, b)
	}
	if c != nil {
		t.Fatalf("third claim should be blocked by max_parallel=2: %+v", c)
	}
}

func TestJobStorage_OnStepFailStopCancelsRemaining(t *testing.T) {
	pool := testPool(t)
	st := NewJobStorage(pool)
	tenantID := newTestTenant(t, pool)
	ctx := context.Background()

	batchID := uuid.New().String()
	for i := 0; i < 3; i++ {
		runID := fmt.Sprintf("run-stop-%d-%s", i, uuid.New().String()[:6])
		j := sampleJob(tenantID, runID, JobCost{CPUs: 1, RunsRunning: 1})
		j.BatchID = batchID
		j.SuiteID = "suite-stop"
		j.Position = i
		j.SuitePolicy = json.RawMessage(`{"mode":"sequential","max_parallel":1,"on_step_fail":"stop"}`)
		if err := st.Enqueue(ctx, j); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
	}

	// Claim & fail position 0.
	first, _ := st.ClaimNext(ctx, "i", map[string]JobCost{tenantID: {}})
	if err := st.Finish(ctx, tenantID, first.RunID, JobStateFailed, "boom"); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	// Next ClaimNext should mark positions 1 and 2 cancelled, not claim them.
	for i := 0; i < 3; i++ {
		next, _ := st.ClaimNext(ctx, "i", map[string]JobCost{tenantID: {}})
		if next != nil {
			t.Fatalf("on_step_fail=stop: claimed %+v, expected nil", next)
		}
	}
	rows, err := st.ListByBatch(ctx, tenantID, batchID)
	if err != nil {
		t.Fatalf("ListByBatch: %v", err)
	}
	for _, r := range rows {
		if r.Position == 0 {
			if r.State != JobStateFailed {
				t.Errorf("position 0 = %s, want failed", r.State)
			}
		} else {
			if r.State != JobStateCancelled {
				t.Errorf("position %d = %s, want cancelled (on_step_fail=stop)", r.Position, r.State)
			}
		}
	}
}

func TestJobStorage_ConcurrentClaimNoDouble(t *testing.T) {
	pool := testPool(t)
	st := NewJobStorage(pool)
	tenantID := newTestTenant(t, pool)
	ctx := context.Background()

	// 5 jobs, 8 simulated schedulers — every job gets exactly one claimer.
	n := 5
	for i := 0; i < n; i++ {
		runID := fmt.Sprintf("run-c-%d-%s", i, uuid.New().String()[:6])
		if err := st.Enqueue(ctx, sampleJob(tenantID, runID, JobCost{CPUs: 1, RunsRunning: 1})); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
	}
	limits := map[string]JobCost{tenantID: {}}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		claimed = make(map[string]int)
	)
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(wid int) {
			defer wg.Done()
			for {
				j, err := st.ClaimNext(ctx, fmt.Sprintf("worker-%d", wid), limits)
				if err != nil {
					return
				}
				if j == nil {
					return
				}
				mu.Lock()
				claimed[j.RunID]++
				mu.Unlock()
			}
		}(w)
	}
	wg.Wait()
	if len(claimed) != n {
		t.Errorf("claimed runs = %d, want %d", len(claimed), n)
	}
	for runID, cnt := range claimed {
		if cnt != 1 {
			t.Errorf("run %s claimed %d times, want 1", runID, cnt)
		}
	}
}

func TestJobStorage_ReviveStaleResetsAndFreesQuota(t *testing.T) {
	pool := testPool(t)
	st := NewJobStorage(pool)
	tenantID := newTestTenant(t, pool)
	ctx := context.Background()

	runID := fmt.Sprintf("run-stale-%s", uuid.New().String()[:6])
	cost := JobCost{CPUs: 4, RunsRunning: 1}
	if err := st.Enqueue(ctx, sampleJob(tenantID, runID, cost)); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	limits := map[string]JobCost{tenantID: {}}
	j, err := st.ClaimNext(ctx, "dead-instance", limits)
	if err != nil || j == nil {
		t.Fatalf("ClaimNext: %v %+v", err, j)
	}

	// Force the heartbeat into the past so ReviveStale picks it up.
	if _, err := pool.Exec(ctx, `
		UPDATE job_runs SET heartbeat_at = NOW() - INTERVAL '1 minute'
		WHERE run_id=$1 AND tenant_id=$2`, runID, tenantID); err != nil {
		t.Fatalf("force heartbeat past: %v", err)
	}

	n, err := st.ReviveStale(ctx, 30*time.Second)
	if err != nil {
		t.Fatalf("ReviveStale: %v", err)
	}
	if n != 1 {
		t.Errorf("ReviveStale = %d, want 1", n)
	}

	got, _ := st.Get(ctx, tenantID, runID)
	if got.State != JobStateQueued {
		t.Errorf("state after revive = %s, want queued", got.State)
	}
	used, _ := st.GetUsed(ctx, tenantID)
	if used.CPUs != 0 || used.RunsRunning != 0 {
		t.Errorf("used after revive = %+v, want zeroed", used)
	}
}

func TestJobStorage_CancelQueued(t *testing.T) {
	pool := testPool(t)
	st := NewJobStorage(pool)
	tenantID := newTestTenant(t, pool)
	ctx := context.Background()

	runID := fmt.Sprintf("run-cancel-%s", uuid.New().String()[:6])
	if err := st.Enqueue(ctx, sampleJob(tenantID, runID, JobCost{CPUs: 1, RunsRunning: 1})); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	ok, err := st.CancelQueued(ctx, tenantID, runID)
	if err != nil || !ok {
		t.Fatalf("CancelQueued: %v ok=%v", err, ok)
	}
	got, _ := st.Get(ctx, tenantID, runID)
	if got.State != JobStateCancelled {
		t.Errorf("state = %s, want cancelled", got.State)
	}

	// Cancelling again is a no-op (already terminal).
	ok, _ = st.CancelQueued(ctx, tenantID, runID)
	if ok {
		t.Errorf("second CancelQueued should report no-op")
	}
}
