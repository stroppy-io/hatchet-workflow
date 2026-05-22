# language: en
@catalog @ui @migration
Feature: TestPreset authoring wizard

  The wizard is the human-in-the-loop surface for Define -> Render -> Plan. It
  starts from reusable DatabasePreset and WorkloadPreset catalog entries, copies
  them by value into a draft, lets the user override any layer, and ends with two
  immutable previews: the exact TestPreset snapshot and the compiled Dag
  blueprint. The blueprint is not a run: no TestRun is created, no state is
  persisted as execution state, and no task is started.
  Canon: api/ui/authoring.proto, models/preset.proto, domain/test.proto,
  runtime/primitive/dag.proto.

  # --- routing / tenant context ---

  @invariant
  Scenario: Every wizard route is tenant-scoped
    Given the active tenant_id is "tenant-a"
    When the user opens any wizard, preset, run, or settings page
    Then the URL starts with "/t/tenant-a/"
    And every tenant-scoped RPC carries TenantId = "tenant-a"

  # --- step 1: choose reusable building blocks ---

  Scenario: The wizard starts from DatabasePreset and WorkloadPreset
    Given a tenant has a Preset.kind = KIND_DATABASE with oneof database_preset
    And the tenant has a Preset.kind = KIND_WORKLOAD with oneof workload_preset
    When the user selects both presets and a Provider
    Then AuthoringService.AssembleFromPresets is called
    And the response contains a TestPresetAssembly
    And the selected presets are copied by value into the draft

  @invariant
  Scenario: Later catalog edits do not mutate the wizard draft
    Given the user assembled a draft from DatabasePreset "pg-ha"
    When another user edits "pg-ha" in the catalog
    Then the already-assembled wizard draft does not change
    And submitting the draft uses the copied TestPreset values, not preset ids

  Scenario: Incompatible presets are accepted into the draft with diagnostics
    Given a DatabasePreset for postgres/PG
    And a WorkloadPreset whose script is unsupported for postgres/PG
    When the wizard assembles from presets
    Then the draft TestPreset is returned when it can be structurally assembled
    And diagnostics include SCRIPT_UNSUPPORTED
    And submit is blocked until validation has no ERROR diagnostics

  # --- steps 2-3: structured intent editing ---

  Scenario: Database edits reassemble the TestPreset
    Given an assembled draft
    When the user changes Database.Options or Database.Target in the structured editor
    Then AuthoringService.AssembleTestPreset is called with the by-value draft
    And AuthoringService.ValidateTestPreset reports cross-entity diagnostics
    And AuthoringService.PreviewDatabaseRender returns the database render preview

  Scenario: Workload edits reassemble and preview the stroppy config
    Given an assembled draft
    When the user changes Workload.script, Protocol, Execution, Parameters, or Files
    Then AuthoringService.AssembleTestPreset is called with the by-value draft
    And AuthoringService.ProbeWorkload can validate the workload before submit
    And AuthoringService.PreviewWorkloadConfig returns the generated stroppy config

  # --- step 4: topology control ---

  Scenario: Topology is merged from preset fragments and user patches
    Given DatabasePreset has a database topology fragment
    And WorkloadPreset has a workload topology fragment
    When the wizard opens the topology step
    Then AuthoringService.MergeTopology merges both fragments and the user patch
    And the result has provenance for database preset, workload preset, generated, and user fields

  Scenario: Machine sizing is edited directly on Topology
    Given a generated Topology.Machine with cores = 4
    When the user changes cores to 8
    Then the draft Topology.Machine.cores = 8
    And there is no hidden machine_override object
    And later assembly keeps the user-edited field pinned

  # --- step 5: deployment control ---

  Scenario: DeploymentIntent is materialized by default
    Given an assembled TestPreset with a provider-agnostic Topology
    When the wizard opens the deployment step for PROVIDER_YANDEX
    Then AuthoringService.MaterializeDeploymentIntent returns a DeploymentIntent
    And the UI shows the Yandex VMs, disks, network, placement, and managed services that will be created
    And the user is not required to edit provider-specific fields

  Scenario: Advanced deployment edits pin user fields
    Given a materialized DeploymentIntent for Yandex Cloud
    When the user changes a VM platform or disk type in advanced mode
    Then the edited field is marked as user provenance
    And future re-materialization keeps the edited field unless the user resets it
    And AuthoringService.CheckDeployment reports quota feasibility

  # --- step 6: review and launch ---

  @invariant
  Scenario: Review shows both the TestPreset snapshot and the Dag blueprint
    Given an assembled and valid TestPreset
    When the user opens the review step
    Then the UI shows the exact TestPreset that will be submitted
    And AuthoringService.CompileTestPresetPreview returns a primitive.Dag blueprint
    And the UI shows nodes, dependencies, always_run teardown, retry, scheduling, and execution locus
    And no TestRun is created
    And no Dag execution state is started

  Scenario: Submit uses the reviewed TestPreset
    Given the review step has no ERROR diagnostics
    When the user clicks Submit
    Then RunService.SubmitTestRun is called with tenant_id, name, description, and the reviewed TestPreset
    And the returned TestRun belongs to the same tenant

  Scenario: Save actions persist the chosen snapshot scope
    Given the user reviewed a draft
    When the user saves as WorkloadPreset
    Then only the WorkloadPreset part is stored in Preset.kind = KIND_WORKLOAD
    When the user saves as DatabasePreset
    Then the DatabasePreset part is stored in Preset.kind = KIND_DATABASE
    When the user saves as TestPreset
    Then the full TestPreset snapshot is stored in Preset.kind = KIND_TEST
