ALTER TABLE job_runs DROP COLUMN IF EXISTS suite_policy;
ALTER TABLE suites   DROP COLUMN IF EXISTS execution_policy;
