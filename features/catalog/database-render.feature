# language: en
@catalog @migration
Feature: Rendering DB configuration from intent

  The same renderer serves both preview (the Review step in the wizard) and
  execution on the agent. The renderer is pure: a Database intent + topology →
  a set of render.Config.Item (system.File / system.Service). Runtime values (IP,
  host, endpoints) are expressed by typed render.Binding and resolved in a single
  place (plan→execute). Canon: domain/database.proto, runtime/render/config.proto.

  Background:
    Given a Database intent for engine "postgres" version "16"
    And target = self_hosted

  Scenario: A user parameter wins over the default
    Given Options.postgres.parameters sets "shared_buffers" = "512MB"
    When the intent is rendered
    Then render-item "postgresql.conf" contains "shared_buffers = 512MB"
    And the user value overrides the engine default

  Scenario Outline: Memory percentages resolve to megabytes
    Given a parameter with value "<percent>%" and a memory budget of <budget_mb> MB
    When the intent is rendered
    Then the parameter value equals "<result>"

    Examples:
      | percent | budget_mb | result |
      | 25      | 4096      | 1024MB |
      | 25      | 64        | 32MB   |
      | 90      | 8192      | 2048MB |

  Scenario: PATRONI replication mode emits patroni.yml and does not duplicate knobs
    Given Options.postgres.replication.mode = PATRONI
    When the intent is rendered
    Then there is a render-item "patroni.yml"
    And render-item "postgresql.conf" does not contain "wal_level"
    And streaming replication is managed by Patroni, not postgresql.conf

  Scenario: HAProxy with three replicas renders a backend for each node
    Given Options.postgres.access.haproxy = true
    And the topology contains 1 master and 2 replica DB components
    When the intent is rendered
    Then render-item "haproxy.cfg" contains 3 backend servers

  @invariant
  Scenario: Preview is byte-for-byte equal to the executable artifact, except bindings
    When the intent is rendered in preview
    And the same intent is planned into a WRITE_FILE operation for the agent
    Then the render-item content and the WRITE_FILE content match byte-for-byte
    Except for the set of declared render.Binding
    And there are no other differences

  Scenario: Override text_patch marks the item and changes its content
    Given a rendered item "postgresql.conf" with id "pg-conf"
    When the user sends Override.text_patch for item_id "pg-conf"
    And the intent is re-rendered
    Then item "pg-conf" has overridden = true
    And its content equals the patch

  Scenario: External (BYOD) target does not render DB config
    Given target = external with endpoint "db.example.com:5432"
    When the intent is rendered
    Then DB config files are not rendered
    And only connection-metadata for stroppy is rendered

  # --- Runtime bindings and anti-leak ---

  Scenario: A runtime binding resolves from a topology coordinate
    Given render-item "patroni.yml" with Binding token "etcd_hosts"
    And the binding Source = {role: COORDINATOR, attr: ENDPOINT}
    When the plan is executed and the addresses of coordinator components are known
    Then the resolver substitutes the real endpoints into the content
    And the agent receives already-resolved bytes

  @anti-leak
  Scenario: A WRITE_FILE with an unresolved binding is invalid (no leak downward)
    Given a WRITE_FILE operation whose content contains an unresolved Binding
    When the operation is validated before being sent to the agent
    Then validation fails with a contract error
    And the agent never receives such an operation

  Note: the rule "domain does not reference render.Binding" (no leak upward)
    is checked by an architectural import test, not a BDD scenario.

  # --- error / edge cases ---

  Scenario: An override referencing an unknown item_id is rejected
    Given a render Config that has no item with id "ghost"
    When an Override with item_id "ghost" is applied
    Then the re-render fails: the override target does not exist

  Scenario: Conflicting overrides on the same item are rejected
    Given two overrides for the same item_id "pg-conf"
    When the render is applied
    Then it is rejected as a conflicting override

  Scenario: An unsupported engine version is rejected
    Given Database engine "postgres" version "13" outside the supported set
    When the intent is validated
    Then it is rejected with an unsupported-version error

  Scenario: A malformed engine parameter is rejected
    Given Options.postgres.parameters with a malformed value
    When the intent is validated
    Then validation fails on that parameter
