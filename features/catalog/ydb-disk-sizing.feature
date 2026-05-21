# language: en
@catalog @migration
Feature: Disk sizing and auto-sizing as a wizard default (C9)

  Disk size is a provider-agnostic "capacity intent" on Topology.Machine (next to
  cores/memory). Auto-sizing is a SENSIBLE WIZARD DEFAULT derived from the workload
  scale_factor, NOT a persisted flag in the DB intent (the old auto_size_pdisks is
  removed). The user sees the default in preview and edits it. Provider specifics
  (disk type, io-m3/93GiB rounding) live in deployment (D18). The calculation
  heuristic is backend data (H5). Canon: domain/topology.proto, deployment (D18).

  Scenario Outline: the wizard defaults capacity from the workload scale
    Given script "<script>" and scale_factor <sf>
    When the wizard builds the topology (Render)
    Then Topology.Machine for YDB-storage gets a default capacity of <gb> GB
    And the value is shown in preview and is editable

    Examples:
      | script  | sf   | gb   |
      | tpcc/tx | 500  | 186  |
      | tpcc/tx | 5000 | 1116 |

  Scenario: Auto-sizing is not a state flag but wizard behavior
    Given a DB intent without any disk flag
    When the wizard renders the topology
    Then the disk capacity is computed as a default (backend heuristic)
    And there is no persisted auto_size flag in the entities

  Scenario: The user overrides the default capacity
    Given the wizard suggested a default capacity
    When the user changes it in preview
    Then Topology.Machine disk capacity = the user value (topology = source of truth, B5)

  Scenario: Provider specifics stay in deployment
    Given capacity on Topology.Machine (provider-agnostic)
    When it is materialized under Yandex
    Then the disk type and io-m3 rounding to 93 GiB are applied at the deployment layer (D18)
    And there are no provider parameters in Topology.Machine

  Scenario: Multi-disk YDB-storage
    Given a YDB-storage with several disks
    When the wizard defaults the capacity
    Then each disk gets an aligned capacity, the capacity is split across the pdisks

  # --- edge cases ---

  Scenario: The computed capacity is io-m3 rounded at materialization
    Given a wizard-defaulted capacity that is not a 93 GiB multiple
    When it materializes to a Yandex io-m3 disk
    Then the size is rounded up to a 93 GiB multiple (D18)
    And Topology.Machine keeps the provider-agnostic capacity

  Scenario: A user override below the estimated need is respected but warned
    Given the user overrides capacity below the estimated raw-data need
    Then the override is respected (topology = source of truth, B5)
    And preview surfaces a warning that capacity may be insufficient

  Scenario: A very large scale factor keeps chunk alignment
    Given script "tpcc/tx" and a very large scale_factor
    When the wizard defaults the capacity
    Then the result is a 93 GiB-aligned capacity (raw → chunk → x2 headroom)
