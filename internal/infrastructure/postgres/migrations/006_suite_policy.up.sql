-- Suite execution policy: controls how the scheduler admits step jobs in
-- a launched batch. JSON-encoded so we can extend without ALTER TABLE.
-- {
--   "mode": "sequential" | "parallel",
--   "max_parallel": 1,                  -- only used when mode=parallel
--   "on_step_fail": "continue" | "stop",
--   "step_timeout_min": 0               -- 0 = no timeout
-- }
ALTER TABLE suites
  ADD COLUMN execution_policy TEXT NOT NULL DEFAULT
    '{"mode":"sequential","max_parallel":1,"on_step_fail":"continue","step_timeout_min":0}';

-- Snapshot of the suite's policy at launch time, copied into every step's
-- job_runs row. Lets the scheduler's claim path decide gating (sequential
-- blockers vs parallel max_parallel) without joining suites every tick,
-- and means policy edits made AFTER launch don't retroactively change a
-- running batch.
ALTER TABLE job_runs
  ADD COLUMN suite_policy TEXT NOT NULL DEFAULT '{}';
