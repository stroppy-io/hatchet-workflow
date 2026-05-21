# language: en
@future @resilience
Feature: Resilience and graceful degradation (roadmap)

  Forward-looking, NOT a migration recast. How the system degrades when a backing
  store is unavailable. Principle: the run (a Dag) is durable in Postgres; the
  metrics/logs stores are observability sidecars — their downtime must not fail a
  run. Canon TBD.

  Scenario: VictoriaMetrics down does not fail a run
    Given a run executing while VictoriaMetrics is unreachable
    Then the run continues (metrics ingest path is independent of the engine)
    And the UI shows "metrics temporarily unavailable", retrying

  Scenario: VictoriaLogs down does not fail a run
    Given VictoriaLogs is unreachable
    Then commands still execute and report; log shipping retries/buffers
    And QueryRunLogs surfaces a degraded state, not an error page

  Scenario: Postgres (state) outage pauses, does not lose
    Given Postgres is briefly unavailable
    Then the DagProcessor cannot claim/save and pauses
    And on recovery it resumes from the persisted Dag (no data loss)

  Scenario: Valkey (lease/cache) outage degrades to pg
    Given Valkey is unavailable
    Then leasing falls back to the pg claim (models.Dag.Processor) and refresh/cache degrade

  Scenario: An unreachable agent is handled by lease expiry, not a hang
    Given an agent stops responding mid-command
    When its lease expires
    Then the node is re-leasable and the run does not hang (D16)
