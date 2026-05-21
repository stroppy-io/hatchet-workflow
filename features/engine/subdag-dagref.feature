# language: en
@engine @migration
Feature: Sub-Dag and Dag-ref node variants

  A node may be a task, an embedded sub_dag, or a dag_ref to a separate persisted
  Dag. A sub_dag executes in place and mirrors its terminal result into the owning
  node. A dag_ref executes or waits for another Dag through the runtime's
  DagRefRunner. Canon: runtime/primitive/dag.proto; internal/runtime.

  Scenario: A successful sub_dag completes the owning node
    Given a node whose sub_dag completes
    When the Dag runs
    Then the owning node is STATUS_COMPLETED

  Scenario: A multi-node sub_dag executes its embedded graph
    Given a sub_dag with several nodes and edges
    When it runs
    Then its nodes execute in dependency order before the owning node completes

  Scenario: A sub_dag failure propagates to the owning node
    Given a node whose sub_dag fails
    When the Dag runs
    Then the owning node is STATUS_FAILED with FailureCode SUB_DAG_FAILED

  Scenario: A cancelled sub_dag cancels the owning node
    Given a node whose sub_dag is cancelled
    Then the owning node is STATUS_CANCELLED

  Scenario: A dag_ref mirrors the child Dag terminal result
    Given a dag_ref node and a DagRefRunner whose child completes
    When the Dag runs
    Then the owning node is STATUS_COMPLETED

  Scenario: A dag_ref requires a registered DagRefRunner
    Given a dag_ref node and no DagRefRunner registered
    When the node runs
    Then it fails with DAG_REF_RUNNER_MISSING

  Scenario Outline: A dag_ref mirrors a child terminal status
    Given a dag_ref whose child reaches <child>
    Then the owning node is <node>

    Examples:
      | child            | node             |
      | STATUS_FAILED    | STATUS_FAILED    |
      | STATUS_CANCELLED | STATUS_CANCELLED |

  Scenario: A nil child Dag fails the dag_ref node
    Given a dag_ref whose runner returns a nil child
    Then the node fails with FailureCode DAG_REF_FAILED

  Scenario: A pending dag_ref child is polled until terminal
    Given a dag_ref whose child is still pending
    When the engine polls within the retry budget
    Then it waits, then mirrors the child's terminal result once reached

  Scenario: A dag_ref that stays pending exhausts attempts and fails
    Given a dag_ref child that never reaches a terminal status
    When the poll attempts are exhausted
    Then the node fails with DAG_REF_PENDING
