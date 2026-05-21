DROP INDEX IF EXISTS idx_job_runs_matrix_cell;
DROP INDEX IF EXISTS idx_job_runs_cron_idem;
ALTER TABLE job_runs DROP COLUMN IF EXISTS db_preset_id;

CREATE UNIQUE INDEX idx_job_runs_cron_idem
    ON job_runs(suite_id, fire_at, suite_item_id)
    WHERE fire_at IS NOT NULL;

ALTER TABLE suite_items
    DROP COLUMN IF EXISTS workload,
    ADD COLUMN db_preset_id TEXT REFERENCES presets(id) ON DELETE RESTRICT,
    ADD COLUMN run_config TEXT NOT NULL DEFAULT '{}';
CREATE INDEX IF NOT EXISTS idx_suite_items_preset ON suite_items(db_preset_id);

ALTER TABLE suites
    DROP COLUMN IF EXISTS db_preset_ids,
    DROP COLUMN IF EXISTS provider,
    DROP COLUMN IF EXISTS platform_id,
    DROP COLUMN IF EXISTS network,
    DROP COLUMN IF EXISTS stroppy_machine;
