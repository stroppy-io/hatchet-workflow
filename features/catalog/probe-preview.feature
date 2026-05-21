# language: en
@catalog @migration
Feature: Probe and workload preview (G4) — wizard tools before a run

  In the Define/Render phase (human-in-the-loop) the user can check a workload
  BEFORE committing a run: preview renders stroppy-config.json (B4), probe actually
  runs `stroppy probe` against the script and returns env/steps/sql/drivers. These
  are server-locus operations (like terraform — on the control-plane), NOT part of
  the run-Dag. Canon: domain/workload.proto; old probe_handler/stroppy_preview_handler.

  Scenario: Preview renders a stroppy-config without a run
    Given a Workload intent
    When the user requests a preview
    Then the rendered stroppy-config.json is returned (the same renderer, B4)
    And no run is created

  Scenario: Probe validates the script against the engine
    Given Workload.script and Database.Kind/Protocol
    When the user runs probe
    Then the server builds a minimal stroppy-config and runs `stroppy probe -o json`
    And returns env/steps/sql/driver setups
    And it is a server-locus operation, not a run-Dag node

  Scenario: Probe catches an incompatible script before a run
    Given a script outside ScriptCompat for (kind, protocol)
    When probe/validation runs
    Then the error is shown in the wizard before the run is committed

  # --- error / edge cases ---

  Scenario: A missing or unparseable script fails probe
    Given a Workload.script that does not resolve or parses with errors
    When probe runs
    Then probe returns the parse error and no preview is produced

  Scenario: Probe respects a time budget
    Given a probe that exceeds its timeout
    Then it is terminated and reports a timeout (server-locus op)

  Scenario: Preview re-renders with an override applied
    Given a previous preview and a render Override
    When the preview is requested again
    Then the override is applied and the item is marked overridden (B3)
