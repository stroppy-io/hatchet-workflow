-- run_presets: saved templates of run parameters (workload, infra, stroppy
-- knobs). Distinct from `presets` (which is purely DB topology). Stores the
-- whole RunConfig JSON minus identity fields; the API strips ID/Name/Desc on
-- save and re-injects them on load.
CREATE TABLE run_presets (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    db_kind     TEXT NOT NULL,
    config      TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, name)
);

CREATE INDEX idx_run_presets_tenant ON run_presets(tenant_id);
CREATE INDEX idx_run_presets_kind   ON run_presets(tenant_id, db_kind);

-- suites: ordered group of run-presets executed sequentially as one batch.
-- The runs themselves still live in `runs`; the suite stores the order +
-- per-run parameter overrides applied on (re-)launch.
CREATE TABLE suites (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    -- items is a JSON array of {run_preset_id, overrides} entries; overrides
    -- is a partial RunConfig patch applied at launch time.
    items       TEXT NOT NULL DEFAULT '[]',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, name)
);

CREATE INDEX idx_suites_tenant ON suites(tenant_id);

-- suite_runs maps suite executions to the actual runs they produced. One
-- suite execution = one batch_id; each run in the batch gets a row.
CREATE TABLE suite_runs (
    suite_id   TEXT NOT NULL REFERENCES suites(id) ON DELETE CASCADE,
    batch_id   TEXT NOT NULL,
    run_id     TEXT NOT NULL,
    tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    position   INT  NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (suite_id, batch_id, run_id)
);

CREATE INDEX idx_suite_runs_suite ON suite_runs(suite_id, batch_id);
CREATE INDEX idx_suite_runs_run   ON suite_runs(tenant_id, run_id);
