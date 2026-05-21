# language: en
@future @policy
Feature: API rate limits and data retention (roadmap)

  Forward-looking, NOT a migration recast — policy contracts to finalize. Per-tenant
  API rate limiting and TTL-based retention of runs/logs/metrics. Distinct from the
  resource quota (D19) and the tenant concurrency admission (D14).

  Scenario: Per-tenant API rate limit
    Given a tenant exceeding its API request rate
    When further requests arrive
    Then they are rejected with a rate-limit error (HTTP 429) until the window resets

  Scenario: Run retention purges old runs
    Given runs older than the tenant's retention window
    When retention runs
    Then those run records (and their dags) are purged per policy

  Scenario: Log and metric retention
    Given VictoriaLogs/VictoriaMetrics retention windows
    Then logs and metrics older than the window are dropped by the store
    And share snapshots (G5) keep their frozen copy regardless

  Scenario: Retention respects active and shared runs
    Given an active run or a run referenced by a live share link
    Then it is not purged while active/shared
