-- Durable scheduler tables. Replaces the in-memory queue / runtime state so
-- runs survive any server restart and quota accounting becomes
-- transactional. `runs` (snapshots) stays as-is — that's DAG execution
-- state. `job_runs` adds the queue + lifecycle layer; the two join on
-- (run_id, tenant_id).

CREATE TABLE job_runs (
    run_id        TEXT NOT NULL,
    tenant_id     TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    batch_id      TEXT,                      -- NULL for ad-hoc runs; set for suite-launched ones
    suite_id      TEXT,                      -- NULL outside suites
    run_preset_id TEXT,
    position      INT  NOT NULL DEFAULT 0,   -- ordering within a batch
    state         TEXT NOT NULL CHECK (state IN ('queued','claimed','running','finished','failed','cancelled')),
    config        TEXT NOT NULL,             -- merged RunConfig as JSON
    cost          TEXT NOT NULL DEFAULT '{}',-- {cpus,memory_mb,disk_gb,vm_count} JSON
    priority      INT  NOT NULL DEFAULT 0,
    not_before    TIMESTAMPTZ,
    claimed_by    TEXT,                      -- server_instances.id that owns this row
    claimed_at    TIMESTAMPTZ,
    heartbeat_at  TIMESTAMPTZ,
    started_at    TIMESTAMPTZ,
    finished_at   TIMESTAMPTZ,
    error         TEXT,
    quota_freed   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (run_id, tenant_id)
);
CREATE INDEX idx_job_runs_state         ON job_runs(state, priority DESC, created_at);
CREATE INDEX idx_job_runs_tenant        ON job_runs(tenant_id, state);
CREATE INDEX idx_job_runs_batch         ON job_runs(batch_id, position);
CREATE INDEX idx_job_runs_heartbeat     ON job_runs(heartbeat_at)
    WHERE state IN ('claimed','running');
CREATE INDEX idx_job_runs_suite_pending ON job_runs(suite_id, state)
    WHERE state IN ('queued','claimed','running');

-- Server instances table: each running server process registers itself
-- and writes a heartbeat. Workers' rows in job_runs reference claimed_by;
-- a stale heartbeat means the worker died and the row is orphaned, so the
-- reaper revives it (state→queued).
CREATE TABLE server_instances (
    id           TEXT PRIMARY KEY,
    started_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    heartbeat_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    listen_addr  TEXT,
    version      TEXT,
    stopping     BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX idx_server_instances_heartbeat ON server_instances(heartbeat_at);

-- Per-tenant in-flight resource accounting. Updated transactionally on
-- claim (used += cost) / release (used -= cost) so two competing schedulers
-- can't double-spend the same quota slot. `used` is JSON shaped
-- {cpus, memory_mb, disk_gb, vm_count, runs_running}.
CREATE TABLE quota_accounts (
    tenant_id  TEXT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    used       TEXT NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Durable agent command queue. Replaces the in-memory PollClient queue so
-- a server restart between dispatch and report doesn't lose the command.
-- agentPoll claims one row with FOR UPDATE SKIP LOCKED, agentReport finalises.
CREATE TABLE agent_commands (
    id           BIGSERIAL PRIMARY KEY,
    run_id       TEXT NOT NULL,
    machine_id   TEXT NOT NULL,
    payload      TEXT NOT NULL,
    state        TEXT NOT NULL CHECK (state IN ('pending','claimed','done','failed','cancelled')),
    claimed_by   TEXT,                     -- server_instances.id
    claimed_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    result       TEXT,
    error        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_agent_commands_dispatch ON agent_commands(machine_id, state, created_at)
    WHERE state IN ('pending','claimed');
CREATE INDEX idx_agent_commands_run      ON agent_commands(run_id);

-- Seed an empty quota row for every existing tenant so the scheduler's
-- UPSERT path doesn't need to special-case "first job ever" for that tenant.
INSERT INTO quota_accounts (tenant_id, used)
SELECT id, '{}' FROM tenants
ON CONFLICT (tenant_id) DO NOTHING;
