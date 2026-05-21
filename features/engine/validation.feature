# language: en
@engine @migration
Feature: Dag validation and normalization

  ValidateDag rejects structurally invalid graphs before execution; normalizeDag
  assigns stable execution_ids and the sub_dag flag. execution_id is the immutable
  external identity (unique across the whole aggregate, including embedded sub-Dags);
  node id is the graph-local edge address. Canon: runtime/primitive/dag.proto;
  internal/runtime (ValidateDag, normalizeDag).

  Scenario Outline: ValidateDag rejects structurally invalid graphs
    Given a Dag that is <defect>
    When it is validated
    Then validation fails with FailureCode DAG_INVALID

    Examples:
      | defect                              |
      | nil                                 |
      | empty / has no nodes                |
      | has a duplicate node id             |
      | has an edge to an unknown node      |
      | contains a cycle                    |
      | has a node missing execution_id     |
      | has a duplicate execution_id        |

  @invariant
  Scenario: execution_id must be unique across embedded sub-Dags
    Given a top-level Dag and an embedded sub_dag that reuse the same execution_id
    When the aggregate is validated
    Then validation fails with DAG_INVALID (execution_id must be unique across the whole payload)

  Scenario: Normalization assigns stable execution_ids and the sub_dag flag
    Given a Dag whose nodes have no execution_id and an embedded sub_dag
    When it is normalized
    Then every node gets a stable execution_id
    And the embedded Dag is marked is_sub_dag

  Scenario: Normalization preserves an existing execution_id
    Given a node that already has an execution_id
    When the Dag is normalized
    Then that execution_id is left unchanged

  Scenario: A node is addressable by execution_id across the aggregate
    Given a node with execution_id "x" inside an embedded sub_dag
    When looked up by execution_id "x"
    Then the node is found (used by agent leases/reports, not a structural path)
