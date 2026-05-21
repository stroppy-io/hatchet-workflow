# language: en
@engine @migration
Feature: Dag edges, conditions and join policies

  Edge satisfaction and node readiness in the runtime engine (primitive.Dag).
  An edge with no condition is satisfied when its source reaches STATUS_COMPLETED;
  on_status satisfies on a chosen terminal status; predicate_name defers to a
  registered Go predicate. A node's join_policy merges its incoming edges.
  Canon: runtime/primitive/dag.proto; internal/runtime.

  Scenario: Nodes execute in dependency order
    Given nodes a and b with edge a -> b
    When the Dag runs
    Then a completes before b starts
    And the Dag status is STATUS_COMPLETED

  Scenario: An edge with no condition is satisfied on COMPLETED
    Given edge a -> b with no condition
    When a reaches STATUS_COMPLETED
    Then b becomes ready

  Scenario Outline: An on_status edge is satisfied only by the matching terminal status
    Given edge a -> b with on_status = <on_status>
    When a reaches <actual>
    Then b <runs>

    Examples:
      | on_status        | actual           | runs       |
      | STATUS_FAILED    | STATUS_FAILED    | runs       |
      | STATUS_COMPLETED | STATUS_COMPLETED | runs       |
      | STATUS_FAILED    | STATUS_COMPLETED | never runs |

  Scenario: A predicate edge allows or blocks the target
    Given edge a -> b with predicate_name "p"
    When a reaches a terminal status and predicate "p" returns true
    Then b runs
    When predicate "p" returns false
    Then b does not run

  @invariant
  Scenario: JOIN_POLICY_ALL requires every incoming edge and skips on a failed dependency
    Given node c with join_policy ALL and incoming edges from a and b
    When a completes but b fails
    Then c is marked STATUS_SKIPPED (a required dependency did not satisfy)

  Scenario: JOIN_POLICY_ANY needs only one satisfied incoming edge
    Given node c with join_policy ANY and incoming edges from a and b
    When a completes (b still failed)
    Then c becomes ready
