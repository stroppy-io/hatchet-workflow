# language: en
@infra @migration
Feature: Persistence model (G1) — schema derived from proto

  The DB schema is declared in proto via ratel annotations (ratel.table/ratel.column);
  migrations are generated (ratel diff from a proto diff), not written by hand; storage
  is generated. Aggregates are stored as a JSONB payload, indexable/queryable
  fields as scalar mirror columns (the models.Dag pattern). Common blocks: Ulid Id
  (type_alias→scalar), Timestamps, Entity, Own. Canon: models/*.proto, common.proto.

  Scenario: A table exists if and only if it is declared in proto
    Given a message with (ratel.table)
    When migrate-gen runs (ratel diff)
    Then a migration is generated from the proto↔schema difference
    And a hand-written CREATE TABLE is not needed

  @invariant
  Scenario: Aggregate is JSONB, indexable is a scalar mirror
    Given models.Dag
    Then the payload (nodes/edges/execution) is stored as JSONB
    And status, lease_expires_at are extracted into scalar columns for indexes
    And save paths derive the scalars from the payload

  Scenario: FK cascade by ownership (Own)
    Given a tenant with dags/presets/runs
    When the tenant is deleted
    Then dependent rows are cascade-deleted (Own.tenant_id on_delete CASCADE)

  Scenario: Soft-delete via deleted_at
    Given a row with deleted_at
    When an ordinary list/get runs
    Then rows with deleted_at IS NOT NULL are filtered out

  Scenario: Typed Ulid Id — a scalar PK/FK
    Given DagId/TenantId (type_alias)
    Then the column is a scalar ULID(26)
    And serves as the primary/foreign key

  Scenario: Queries are typed per-model, not generic CommonQuery
    Given a model needs listing/filtering
    When a query is defined
    Then the model's own typed query struct is used
    And a generic CommonQuery with string filters is NOT used (type safety + injection protection)

  Scenario: Migrations are generated and pre-v1 regenerable
    Given a proto schema change
    When ratel diff runs
    Then a new migration is added (atlas.sum)
    And before v1 migrate-clear can regenerate from scratch

  # --- error / edge cases ---

  @invariant
  Scenario: A stale processor write loses on a generation conflict
    Given a Dag claimed at generation N, then re-claimed at generation N+1 by another server
    When the stale owner (generation N) tries to save
    Then the write is rejected by the generation guard
    And the newer snapshot wins

  Scenario: Soft-deleted rows are excluded from normal queries
    Given a row with deleted_at set
    When a normal list/get runs
    Then the row is not returned

  Scenario: Deleting a tenant cascades to its owned rows
    Given a tenant with dags/presets/runs/api_tokens
    When the tenant is deleted
    Then the dependent rows are removed (Own.tenant_id ON DELETE CASCADE)
