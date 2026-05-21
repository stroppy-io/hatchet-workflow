# language: en
@results @migration
Feature: Sharing run results (G5)

  A share link freezes the run result (snapshot + collected metrics) into a
  read-only record, addressable by token, accessible without auth. The snapshot is
  immutable (like TestPreset, B6): later changes to the run/metrics do not touch it.
  Canon: models/testing.proto (snapshot); old share_handlers.

  Scenario: Creating a share link freezes the snapshot + metrics
    Given a completed run with metrics
    When the user creates a share link
    Then the run snapshot and metrics are copied into the share record (immutable)
    And a token is generated, the URL /share/<token> is returned

  Scenario: Viewing a share requires no auth
    Given a share token
    When someone opens /share/<token>
    Then the frozen snapshot + metrics are returned
    And no authorization is needed

  @invariant
  Scenario: Changes after sharing do not change the share
    Given a share has been created
    When the original run/metrics later change
    Then the share contents do not change (immutable snapshot, like B6)

  # --- error / edge cases ---

  Scenario: An unknown or revoked share token returns not-found
    Given a token that does not exist or was revoked
    When GetSharedRun is called
    Then it returns not-found and leaks no data

  Scenario: Sharing a run snapshots its state at creation time
    Given a run in any state (running or terminal)
    When a share link is created
    Then the snapshot captures the run state + metrics at creation (immutable thereafter)
