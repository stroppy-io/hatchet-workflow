# language: en
@catalog @migration
Feature: Assembling Test/Suite presets and cross-entity validation

  TestPreset is the assembly point: Database + Workload + merged Topology +
  materialized DeploymentIntent. It is stored as an IMMUTABLE snapshot (inline
  copies) so that a run is reproducible even after the catalog presets are edited.
  Cross-entity validation (the old ValidateConfig) also lives here. SuitePreset.
  Scheduling maps directly to Dag.Scheduling. Canon: domain/test.proto,
  domain/suite.proto, models/preset.proto, models/testing.proto.

  # --- Cross-entity validation at assembly ---

  Scenario Outline: script × (kind, protocol) is checked when assembling a TestPreset
    Given Database engine "<kind>", Workload protocol <protocol>, script "<script>"
    When a TestPreset is assembled
    Then the result = <result>

    Examples:
      | kind     | protocol | script    | result   |
      | postgres | PG       | tpcc/tx   | accepted |
      | postgres | PG       | foobar/xx | rejected |
      | picodata | PICODATA | tpcc/procs| rejected |

  Scenario: A stroppy.version below the minimum is rejected
    Given Workload.stroppy_version = "5.0.0" and the minimum is 5.1.1
    When a TestPreset is assembled
    Then validation fails: version below the minimum

  Scenario: A commit-pinned stroppy.version bypasses semver
    Given Workload.stroppy_version = "commit:a1b2c3d"
    When a TestPreset is assembled
    Then validation passes (valid hex 7–40)

  Scenario: YDB mirror-3-dc with a disk failure-domain requires a quorum
    Given Database.Options.ydb.self_hosted fault_tolerance = MIRROR_3_DC, failure_domain = DISK
    And storage_nodes = 2
    When a TestPreset is assembled
    Then validation fails: need ≥3 storage and ≥3 secondary disks per node

  Scenario: A topology↔kind mismatch is structurally impossible
    When Database.Options is set
    Then the oneof allows exactly one engine
    And the old check "kind=postgres but a mysql topology is set" is no longer needed

  # --- Snapshot semantics ---

  @invariant
  Scenario: TestPreset is an immutable snapshot
    Given a TestPreset assembled from DATABASE preset "pg16" and WORKLOAD preset "tpcc"
    When the catalog preset "pg16" is later changed
    Then the already-assembled TestPreset does not change
    And a repeat run is reproducible

  Scenario: The merged Topology and materialized DeploymentIntent are stored, not recomputed
    When a TestPreset is assembled
    Then Topology and DeploymentIntent are stored as a snapshot inside the TestPreset

  Scenario: Preset.kind must match the oneof branch
    Given Preset.kind = KIND_DATABASE but the oneof test_preset is set
    When the Preset is validated
    Then validation fails: kind does not match the preset branch

  # --- Bridge SuitePreset → Dag.Scheduling ---

  @orchestration
  Scenario: SuitePreset sequential → suite Dag with max_parallelism = 1
    Given SuitePreset.scheduling.sequential
    When the suite is compiled into a Dag
    Then Dag.Scheduling.max_parallelism = 1

  @orchestration
  Scenario: SuitePreset parallel{N} → max_parallelism = N
    Given SuitePreset.scheduling.parallel.max_parallel = 4
    When the suite is compiled into a Dag
    Then Dag.Scheduling.max_parallelism = 4

  @orchestration
  Scenario: SuitePreset.OnNodeFailure is propagated into the Dag
    Given SuitePreset.scheduling.on_node_failure = STOP
    When the suite is compiled into a Dag
    Then Dag.Scheduling.on_node_failure = ON_NODE_FAILURE_STOP
