-- Suite matrix model: a suite is now a cartesian product between a list of
-- database presets and a list of workload definitions. Earlier model bound
-- each item to one (db_preset, full_runconfig) pair; this one separates the
-- two axes so adding one workload to a suite hits N db targets at once.
--
-- suite_items keeps its identity column so comparison can still pair runs
-- across batches by suite_item_id; what changes is what an item *is* — now
-- a StroppyConfig (workload) blob rather than a full RunConfig snapshot.
-- Infrastructure (provider/network/platform/stroppy machine) lives on the
-- suite row instead, shared across every (preset × workload) launch.
--
-- No data migration: caller confirmed there are no live suites yet.

ALTER TABLE suites
    ADD COLUMN db_preset_ids   TEXT NOT NULL DEFAULT '[]',  -- JSON array of preset IDs
    ADD COLUMN provider        TEXT NOT NULL DEFAULT 'docker',
    ADD COLUMN platform_id     TEXT,
    ADD COLUMN network         TEXT NOT NULL DEFAULT '{"cidr":"10.10.0.0/24"}',
    ADD COLUMN stroppy_machine TEXT NOT NULL DEFAULT '{}';

ALTER TABLE suite_items
    DROP CONSTRAINT IF EXISTS suite_items_db_preset_id_fkey;
ALTER TABLE suite_items
    DROP COLUMN db_preset_id,
    DROP COLUMN run_config,
    ADD COLUMN workload TEXT NOT NULL DEFAULT '{}';

DROP INDEX IF EXISTS idx_suite_items_preset;

-- job_runs gains an explicit db_preset_id column so the comparison endpoint
-- can pair runs across batches by (db_preset_id, suite_item_id) — the matrix
-- cell coordinate. Without this, mixed-preset batches would have no stable
-- pairing key (positions reshuffle when items are added/removed/reordered).
ALTER TABLE job_runs
    ADD COLUMN db_preset_id TEXT;

CREATE INDEX idx_job_runs_matrix_cell
    ON job_runs(suite_id, db_preset_id, suite_item_id)
    WHERE suite_id IS NOT NULL;

-- Replace the old cron-idempotency index with one that includes db_preset_id —
-- a single cron tick now produces one row per (db_preset, item), not per item.
DROP INDEX IF EXISTS idx_job_runs_cron_idem;
CREATE UNIQUE INDEX idx_job_runs_cron_idem
    ON job_runs(suite_id, fire_at, db_preset_id, suite_item_id)
    WHERE fire_at IS NOT NULL;
