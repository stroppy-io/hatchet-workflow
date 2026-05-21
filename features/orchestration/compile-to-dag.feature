# language: en
@orchestration @migration
Feature: Compiling a preset into an executable Dag (Plan phase)

  The scheduler takes a snapshot of TestPreset (Database.Options + merged Topology +
  Workload) and compiles it into a primitive.Dag. The graph shape is emergent from
  Database.Options (etcd/patroni/proxy appear as needed). The agent's on-host work
  is a per-command sub_dag (each WRITE_FILE/RUN_CMD is a node). Data flows between
  nodes via node output + render.Binding; there is no shared mutable State.
  Canon: runtime/primitive/dag.proto. Replaces the old run.builder.Build.

  Scenario: postgres single compiles into a linear Dag
    Given TestPreset: postgres single
    When the preset is compiled
    Then the Dag contains nodes in order: render_config → terraform_apply → install_and_run → collect_results → terraform_destroy
    And terraform_destroy is marked always_run
    And install_and_run is a sub_dag

  Scenario: On-host work — a per-command sub_dag
    Given the install_and_run node (sub_dag)
    When we look at its nodes
    Then each step is a separate node: WRITE_FILE sources.list, RUN_CMD apt-get, WRITE_FILE postgresql.conf, start service
    And each node has its own retry and execution_id
    And the agent leases and reports a command by DagId + execution_id

  Scenario: postgres ha — the shape is emergent from Options
    Given Database.Options.postgres.replication.mode = PATRONI, access.haproxy = true
    When the preset is compiled
    Then etcd, patroni, proxy nodes appear in the Dag
    And they are absent for a single configuration

  Scenario: Patroni supersedes configure_db
    Given Options.postgres.replication.mode = PATRONI
    When the preset is compiled
    Then the configure_db node is not emitted
    And DB configuration goes through the configure_patroni node

  Scenario: ExternalDB compiles into a minimal Dag
    Given Database.Target = external
    When the preset is compiled
    Then there are no install/configure DATABASE nodes
    And there is only provisioning runner + install_and_run(stroppy) + collect + destroy

  Scenario: Deps of the old graph become Dag edges
    Given phase B depended on phase A
    When the preset is compiled
    Then there is an edge source=A target=B
    And an empty edge condition = the source reached STATUS_COMPLETED

  Scenario: A cycle in the compiled Dag is rejected
    Given compilation produced a cycle of edges
    When the Dag is validated
    Then ValidateDag fails with DAG_INVALID

  @invariant
  Scenario: No shared State — data flows via output and binding
    Given terraform_apply returns machine IPs in the node output
    When configure nodes need the DB endpoint
    Then they get it via render.Binding, resolvable from the output
    And there is no hidden mutable State between nodes

  @cleanup @ui-note
  Scenario: Teardown runs even on failure and on cancellation
    Given terraform_apply finished but install_and_run failed
    When the Dag reaches a terminal state
    Then terraform_destroy (always_run) still executes
    # The old MustComplete is replaced by always_run + edges from provision.
    # TODO UI: cancel/teardown behavior must be explicitly rendered (separate task).
