# language: en
@engine @migration
Feature: Run lifecycle and scheduling (DagProcessor)

  Run state = primitive.Status of its Dag (no separate job table).
  DagProcessor pulls a Dag by status and runs it through the Executor. Multi-server:
  the Dag is leased in the DB (like an agent command in D16) — owner + lease_expires;
  an expired lease is picked up by another server. Cross-tenant quotas are decided by the
  admission controller before start (PENDING→RUNNING); the runtime is domain-agnostic.
  Canon: runtime/primitive/status.proto, dag.proto; internal/runtime.

  Scenario Outline: run state maps to Dag.status
    Given a run in state "<old>"
    Then Dag.status = <new>

    Examples:
      | old       | new               |
      | queued    | STATUS_PENDING    |
      | running   | STATUS_RUNNING    |
      | finished  | STATUS_COMPLETED  |
      | failed    | STATUS_FAILED     |
      | cancelled | STATUS_CANCELLED  |

  # --- Admission / quotas ---

  Scenario: A PENDING Dag starts only when the tenant has quota
    Given a tenant's PENDING Dag within quotas (CPU/mem/VM/concurrent runs)
    When the admission controller checks
    Then the Dag is admitted and transitions to STATUS_RUNNING

  Scenario: An exhausted quota keeps the Dag in PENDING
    Given the tenant is already at the max of concurrent runs
    When another PENDING Dag arrives
    Then it stays STATUS_PENDING until quota is freed
    And the runtime engine knows nothing about quotas (admission decides)

  # --- Multi-server coordination ---

  Scenario: A Dag is leased by a single processor
    Given two servers with DagProcessor
    When both see the same PENDING Dag
    Then exactly one leases it (DB-claim), the other skips
    And generation dedup prevents double processing

  Scenario: An expired Dag lease is picked up by another server
    Given a server took a lease on a Dag and died
    When lease_expires_at has passed
    Then another server re-claims the Dag and continues
    And no separate reaper is needed

  # --- Cancel / recovery ---

  Scenario: Cancellation goes through CANCELLING to teardown
    Given a RUNNING Dag
    When cancellation is requested
    Then Dag → STATUS_CANCELLING
    And pending nodes are cancelled
    And always_run teardown still executes
    And Dag → STATUS_CANCELLED

  @invariant
  Scenario: Recovery on restart guarantees teardown (no VM leak)
    Given an unfinished Dag after a server crash
    When the server starts and re-reads the Dag
    Then RecoverDag restores node state
    And always_run teardown runs structurally
    And no separate provider RecoverChecker is needed (reachability is a domain predicate)

  Scenario: RETRY_WAIT retries a node after next_run_at
    Given a node failed with a retryable error and a policy of attempts>1
    When retry.next_run_at arrives
    Then the node is retried
    And attempt is incremented

  Scenario: A terminal Dag leaves the active set
    When a Dag reaches COMPLETED/FAILED/CANCELLED/SKIPPED
    Then DagProcessor removes it from processing
