-- Suite redesign: suites become reusable, cron-schedulable bundles of
-- (db_preset, run_config) items. The previous JSON `items` column is replaced
-- by a normalised `suite_items` table that pins each entry to a `presets` row
-- (the database topology) plus a frozen RunConfig snapshot. Cron, lease,
-- comparison and retention metadata move onto the suites row itself, so a
-- single suite can survive server restarts and fire on a schedule without
-- any external coordinator.
--
-- The audit-only `suite_runs` table is removed: its data is fully derivable
-- from job_runs (suite_id, batch_id, position) which already lives there.

ALTER TABLE suites
    ADD COLUMN cron_expr         TEXT,
    ADD COLUMN timezone          TEXT NOT NULL DEFAULT 'UTC',
    ADD COLUMN enabled           BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN next_fire_at      TIMESTAMPTZ,
    ADD COLUMN last_fire_at      TIMESTAMPTZ,
    ADD COLUMN last_batch_id     TEXT,
    ADD COLUMN concurrent_policy TEXT NOT NULL DEFAULT 'forbid'
        CHECK (concurrent_policy IN ('forbid','allow')),
    ADD COLUMN catchup_mode      TEXT NOT NULL DEFAULT 'skip'
        CHECK (catchup_mode IN ('skip','once')),
    ADD COLUMN lock_owner        TEXT,
    ADD COLUMN lock_expires_at   TIMESTAMPTZ,
    ADD COLUMN comparison_config TEXT NOT NULL DEFAULT '{}',
    ADD COLUMN retention_runs    INT  NOT NULL DEFAULT 30;

-- Cron picker hot-path: only enabled, scheduled rows whose next_fire_at is due.
CREATE INDEX idx_suites_cron_due ON suites(next_fire_at)
    WHERE enabled AND cron_expr IS NOT NULL;

-- Drop the obsolete inline items column. Existing rows lose their item list;
-- callers must repopulate via the new suite_items API. We do not attempt a
-- best-effort migration because the legacy schema (run_preset_id only) does
-- not carry an explicit db_preset_id, which is required by the new model.
ALTER TABLE suites DROP COLUMN items;

CREATE TABLE suite_items (
    id            TEXT PRIMARY KEY,
    suite_id      TEXT NOT NULL REFERENCES suites(id)  ON DELETE CASCADE,
    tenant_id     TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    position      INT  NOT NULL,
    name          TEXT NOT NULL,
    db_preset_id  TEXT NOT NULL REFERENCES presets(id) ON DELETE RESTRICT,
    run_config    TEXT NOT NULL,            -- frozen RunConfig JSON
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (suite_id, position)
);
CREATE INDEX idx_suite_items_suite  ON suite_items(suite_id);
CREATE INDEX idx_suite_items_preset ON suite_items(db_preset_id);
CREATE INDEX idx_suite_items_tenant ON suite_items(tenant_id);

-- job_runs gains explicit links back to the suite_item it was launched from
-- and to the cron firing instant. fire_at + suite_item_id together act as the
-- natural idempotency key: a cron tick that races with itself across two
-- servers can only enqueue each item once.
ALTER TABLE job_runs
    ADD COLUMN suite_item_id TEXT,
    ADD COLUMN trigger       TEXT,            -- 'manual' | 'cron' | 'api'
    ADD COLUMN fire_at       TIMESTAMPTZ;     -- scheduled tick instant; NULL for manual

-- Partial unique index: only enforced for cron-launched rows (fire_at NOT NULL).
-- Manual launches can re-fire the same suite_item freely.
CREATE UNIQUE INDEX idx_job_runs_cron_idem
    ON job_runs(suite_id, fire_at, suite_item_id)
    WHERE fire_at IS NOT NULL;

CREATE INDEX idx_job_runs_suite_item ON job_runs(suite_item_id)
    WHERE suite_item_id IS NOT NULL;

-- suite_runs is fully redundant with job_runs columns (suite_id, batch_id,
-- run_id, position). Drop it; callers join job_runs directly.
DROP TABLE IF EXISTS suite_runs;
