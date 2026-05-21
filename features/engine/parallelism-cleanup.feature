# language: en
@engine @migration
Feature: Parallelism, priority and always_run cleanup

  Admission of ready nodes within a Dag and cleanup semantics. Dag.Scheduling.
  max_parallelism caps concurrent ready nodes; Node.Scheduling.priority orders the
  remaining slots; Node.Scheduling.always_run is reserved for cleanup/teardown that
  may run after failure or cancellation. Canon: runtime/primitive/dag.proto;
  internal/runtime.

  Scenario: max_parallelism caps concurrency and priority breaks ties
    Given several ready nodes and Dag.Scheduling.max_parallelism = 1
    When the engine admits ready nodes
    Then it runs them one at a time
    And higher Node.Scheduling.priority runs earlier

  @invariant
  Scenario: always_run means "also run on failure/cancel", not "run unconditionally"
    Given an always_run cleanup node with NO satisfied incoming edge and an all-success Dag
    When the Dag completes
    Then the cleanup node is STATUS_SKIPPED (always_run did not force it on the happy path)

  Scenario: A teardown with an edge runs on success via that edge
    Given a teardown node marked always_run with edge final -> teardown
    When final completes successfully
    Then teardown runs via the satisfied edge (normal readiness)

  Scenario: always_run executes on failure with on_node_failure STOP
    Given a failing node, on_node_failure = ON_NODE_FAILURE_STOP, and an always_run teardown
    When the Dag fails
    Then ordinary pending nodes are skipped
    But the always_run teardown is force-run to STATUS_COMPLETED

  Scenario: always_run teardown also force-runs on failure with on_node_failure CONTINUE
    Given a failing node, on_node_failure = ON_NODE_FAILURE_CONTINUE, and an always_run teardown
    When the Dag reaches terminal
    Then the always_run teardown is force-run to STATUS_COMPLETED

  Scenario: always_run teardown respects max_parallelism
    Given multiple always_run cleanup nodes and max_parallelism = 1
    When cleanup runs
    Then teardown nodes also run within the parallelism cap

  Scenario: always_run waits for a running dependency before starting
    Given an always_run node depending on a still-running node
    When readiness is evaluated
    Then the always_run node waits until the dependency finishes

  Scenario: No ready non-terminal nodes returns a terminal error (deadlock guard)
    Given a Dag where no node is ready and the Dag is not yet terminal
    When the engine evaluates readiness
    Then it returns a terminal error instead of hanging
