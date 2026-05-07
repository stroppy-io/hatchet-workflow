-- Rollback suite redesign. Re-creates suite_runs (empty), restores suites.items
-- as an empty JSON array, and removes the new columns / table.

CREATE TABLE IF NOT EXISTS suite_runs (
    suite_id   TEXT NOT NULL REFERENCES suites(id) ON DELETE CASCADE,
    batch_id   TEXT NOT NULL,
    run_id     TEXT NOT NULL,
    tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    position   INT  NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (suite_id, batch_id, run_id)
);
CREATE INDEX IF NOT EXISTS idx_suite_runs_suite ON suite_runs(suite_id, batch_id);
CREATE INDEX IF NOT EXISTS idx_suite_runs_run   ON suite_runs(tenant_id, run_id);

DROP INDEX IF EXISTS idx_job_runs_cron_idem;
DROP INDEX IF EXISTS idx_job_runs_suite_item;
ALTER TABLE job_runs
    DROP COLUMN IF EXISTS suite_item_id,
    DROP COLUMN IF EXISTS trigger,
    DROP COLUMN IF EXISTS fire_at;

DROP TABLE IF EXISTS suite_items;

ALTER TABLE suites
    ADD COLUMN IF NOT EXISTS items TEXT NOT NULL DEFAULT '[]';

DROP INDEX IF EXISTS idx_suites_cron_due;
ALTER TABLE suites
    DROP COLUMN IF EXISTS cron_expr,
    DROP COLUMN IF EXISTS timezone,
    DROP COLUMN IF EXISTS enabled,
    DROP COLUMN IF EXISTS next_fire_at,
    DROP COLUMN IF EXISTS last_fire_at,
    DROP COLUMN IF EXISTS last_batch_id,
    DROP COLUMN IF EXISTS concurrent_policy,
    DROP COLUMN IF EXISTS catchup_mode,
    DROP COLUMN IF EXISTS lock_owner,
    DROP COLUMN IF EXISTS lock_expires_at,
    DROP COLUMN IF EXISTS comparison_config,
    DROP COLUMN IF EXISTS retention_runs;
