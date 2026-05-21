# language: en
@results @migration
Feature: Metrics collection, summary and comparison (E)

  stroppy sends OTLP to VictoriaMetrics (prefix = run id, per-tenant accountID).
  The control-plane queries VM via PromQL (per-dbKind metric definitions are
  backend data, not proto) and returns our summary DTOs (RunMetrics/MetricSummary/
  Comparison — not raw OTLP). Live for active runs, snapshot in share (G5) for
  the archive. Cost is a QuotaRequest (D19), there is no separate entity. Types in
  runtime/metrics (not models — they are not stored in the DB; run id = string). Canon:
  runtime/metrics/metrics.proto.

  Scenario: Collecting run metrics from VictoriaMetrics
    Given a completed/active test run and a MetricDef for its dbKind
    When the collector queries VM via PromQL for the run id
    Then RunMetrics{range, [MetricSummary{key,name,unit,avg,min,max,last}]} is returned

  @invariant
  Scenario: Metric isolation by tenant and run
    Given two runs of different tenants
    When metrics are written to VM
    Then series are namespaced by prefix=run id and per-tenant accountID
    And one tenant cannot see another's metrics

  Scenario: Comparing two runs
    Given RunMetrics of two test runs and a threshold
    When the comparison is performed
    Then Comparison{[MetricDiff{avg_a,avg_b,diff_avg_pct,verdict}], summary{better,worse,same}}
    And the per-metric verdict = BETTER/WORSE/SAME by threshold

  Scenario: Run cost is a QuotaRequest, not a separate type
    Given the run's DeploymentIntent
    When a "cost" is needed
    Then it is a set of deployment.QuotaRequest (Σ cores/memory/ssd/instances, D19)
    And there is no separate Cost message

  Scenario: Live for active, snapshot for share
    Given an active run
    When metrics are requested
    Then they are computed live from VM (the source of truth)
    When a share is created (G5)
    Then RunMetrics is frozen into the share record (immutable)

  Scenario: Metric definitions are per-dbKind backend data
    Given dbKind postgres/mysql/ydb
    Then MetricDef/PromQL templates come from backend data (like ScriptCompat, H5)
    And are not modeled in proto

  @invariant
  Scenario: Metrics are self-describing — the frontend renders them generically
    Given a MetricSummary with name, unit, higher_is_better, description, group
    When the frontend renders a metric
    Then it uses only these fields, without knowing what the metric is
    And there are NO enum metric names/meanings (no stroppy_latency etc.)
    And adding a new metric = backend data only, the frontend is untouched
