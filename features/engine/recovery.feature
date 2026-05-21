# language: en
@engine @migration
Feature: Dag recovery and cancellation

  How the engine recovers a Dag re-read after a crash and how cancellation drives
  it to a terminal state while still running cleanup. RecoverDag normalizes node
  execution state on load so a fresh processor can resume. Canon:
  runtime/primitive/dag.proto; internal/runtime (RecoverDag).

  Scenario: An interrupted RUNNING node is recovered and re-run
    Given a Dag reloaded with a node left in STATUS_RUNNING (owner died mid-flight)
    When RecoverDag runs and the Dag is processed
    Then the node is re-driven to a terminal status

  Scenario: RecoverDag promotes a claimable PENDING Dag for execution
    Given a reloaded Dag with pending nodes
    When RecoverDag runs
    Then node execution state is restored and ready nodes resume

  @invariant
  Scenario: A recovered CANCELLING Dag still runs always_run cleanup
    Given a Dag reloaded in STATUS_CANCELLING
    When it is recovered
    Then always_run nodes remain runnable
    And teardown executes before the Dag reaches STATUS_CANCELLED

  Scenario: Context cancellation drives the Dag through CANCELLING to CANCELLED
    Given a RUNNING Dag
    When the run context is cancelled
    Then the Dag goes STATUS_CANCELLING, pending nodes are cancelled, always_run runs
    And the Dag ends STATUS_CANCELLED

  Scenario: Cancellation during a retry sleep aborts promptly
    Given a node waiting in STATUS_RETRY_WAIT (sleeping until next_run_at)
    When the context is cancelled during the sleep
    Then the wait aborts immediately and the Dag cancels
