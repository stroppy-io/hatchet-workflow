# language: en
@infra @migration
Feature: Leases and coordination (G3) — refines D14/D16 per the models.Dag canon

  The authoritative lease lives in Postgres (canon: models.Dag.Processor{claimed_by,
  lease_expires_at, generation} + index dags_claim_idx for SKIP LOCKED;
  models.Agent.lease_expires_at for the agent). The Dag's durable state = JSONB
  payload in the same row. Valkey is an optional fast advisory-lock/cache, NOT
  the source of truth. Canon: models/dag.proto, models/agent.proto.

  Scenario: Claim a Dag via a pg lease (SKIP LOCKED)
    Given a claimable Dag (status, not_before, lease_expires_at in dags_claim_idx)
    When the DagProcessor takes it
    Then claimed_by/claimed_at/lease_expires_at are written to models.Dag.Processor
    And generation is incremented (protection against stale overwrite)

  Scenario: An expired pg lease hands the Dag to another server
    Given a server took the lease and died
    When lease_expires_at has passed
    Then another DagProcessor reclaims it via dags_claim_idx (D14)
    And generation grows, the stale owner cannot overwrite the payload

  Scenario: Agent command lease is also pg
    Given a command issued to the agent (D16)
    Then lease_expires_at lives on models.Agent
    And Report{RUNNING} extends it

  Scenario: Valkey is an optional fast path, not the authority
    Given high-frequency advisory locks (e.g. cron-tick dedup)
    When Valkey is used
    Then it is acceleration/cache
    And the source of truth stays pg (the model does not depend on Valkey)

  @invariant
  Scenario: Protection against double execution
    Given two servers contend for one Dag
    When both try to claim
    Then pg SKIP LOCKED + generation hand it to exactly one
    And the second sees that the Dag is already taken
