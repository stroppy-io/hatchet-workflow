# language: en
@engine @migration
Feature: Node retry and failure propagation

  Per-node retry policy and how failures propagate to the Dag. Retry progress lives
  in Node.Execution.retry_state; structured failures accumulate in Node.Execution.
  Dag.Scheduling.on_node_failure controls whether ordinary pending nodes keep being
  admitted after a failure. Pure backoff math (overflow guard, jitter bounds) stays
  in unit tests. Canon: runtime/primitive/dag.proto, retry.proto; internal/runtime.

  Scenario: Retry records attempts and failure history
    Given a node with retry_policy attempts = 2 that fails once then succeeds
    When the Dag runs
    Then attempt 1 is recorded with its failure
    And the node ends STATUS_COMPLETED on attempt 2

  Scenario Outline: Retry outcome by attempt budget and failures
    Given a node with retry_policy attempts <attempts> that fails <fails> time(s)
    When the Dag runs
    Then the node ends <node_status> on attempt <ran>
    And the Dag is <dag_status>

    Examples:
      | attempts | fails | node_status      | ran | dag_status       |
      | 2        | 1     | STATUS_COMPLETED | 2   | STATUS_COMPLETED |
      | 3        | 2     | STATUS_COMPLETED | 3   | STATUS_COMPLETED |
      | 2        | 5     | STATUS_FAILED    | 2   | STATUS_FAILED    |

  Scenario: A retrying node passes through RETRY_WAIT
    Given a node that fails once then succeeds
    When the Dag runs
    Then it moves PENDING -> RUNNING -> RETRY_WAIT -> RUNNING -> COMPLETED

  Scenario: A non-retryable failure stops retries immediately
    Given a node that returns a FailureError with retryable = false
    When it fails the first time
    Then no further attempts are made
    And the node is STATUS_FAILED

  Scenario: A retryable failure is retried
    Given a node that returns a plain error (assumed retryable) with attempts left
    When it fails
    Then the node enters STATUS_RETRY_WAIT and retries

  Scenario: A node keeps the current failure plus structured history
    Given a node that fails several attempts
    Then Node.Execution.failure holds the latest failure
    And Node.Execution.failures keeps the per-attempt history

  Scenario Outline: Retry delay follows the policy type
    Given retry_policy delay_type <type>
    When the node waits between attempts
    Then the next delay follows <type> (fixed | exponential growth | capped by max_delay)

    Examples:
      | type                    |
      | DELAY_TYPE_FIXED        |
      | DELAY_TYPE_BACKOFF      |

  @invariant
  Scenario: on_node_failure STOP marks unreachable pending nodes SKIPPED
    Given Dag.Scheduling.on_node_failure = ON_NODE_FAILURE_STOP
    When a node fails
    Then ordinary pending nodes that can no longer run are marked STATUS_SKIPPED
    And only always_run nodes may still execute

  Scenario: on_node_failure CONTINUE keeps independent nodes running
    Given Dag.Scheduling.on_node_failure = ON_NODE_FAILURE_CONTINUE
    When one node fails
    Then independent ready nodes continue to be admitted
