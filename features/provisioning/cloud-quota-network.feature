# language: en
@provisioning @migration
Feature: Cloud quotas and network pre-allocation (provisioning preconditions)

  Before a run, we need to know the cloud's real limits and free subnets. The source
  of truth = the cloud (internal accounting drifts). Hybrid: a cached inventory for
  a fast reject + a live provider query for final confirmation before
  apply. Cloud quota is a separate admission gate AFTER the tenant quota (D14). The subnet
  is pre-allocated and leased before the run (collision-safe). Canon:
  deployment/deployment.proto (QuotaRequest/Quota), internal/core/ips.

  # --- D19: cloud quotas ---

  Scenario: QuotaRequest is computed from the sum of VM resources
    Given a DeploymentIntent for N VMs with cores/mem/disk
    When the QuotaRequests are computed
    Then there are requests per quota_name (cores, memory, ssd, instances) with requested = Σ

  Scenario: Hybrid cloud quota check
    Given the QuotaRequests for a run
    When admission checks the quota
    Then the cached inventory rejects clearly-unfittable ones quickly
    And before apply a live provider query is made to confirm available

  Scenario: An exhausted cloud quota does not materialize the run
    Given Quota.available is less than requested
    When the run goes through provisioning admission
    Then it is not materialized (the Dag waits/is rejected)
    And this is a separate gate from the tenant quota (D14)

  Scenario: Deployment records the requested quotas (audit)
    When the DeploymentIntent materializes
    Then Deployment.quota_requests contains what was requested

  # --- D20: network / subnet ---

  Scenario: A subnet is pre-allocated and leased before the run
    Given a parent CIDR and a list of occupied subnets from the real VPC
    When the plan prepares the network
    Then the ips lib picks a free CIDR, avoiding the occupied ones
    And the allocation is reserved (lease) until apply

  Scenario: Concurrent runs get different subnets
    Given two runs start at the same time
    When both pre-allocate a subnet
    Then each gets its own CIDR (the lease excludes the race)
    And the old hash(runID)→CIDR collision is eliminated

  @invariant
  Scenario: The source of truth is the real cloud (reconcile)
    Given a previous run left an unswept subnet/VM
    When reconcile syncs state from the provider
    Then what is occupied in the cloud is counted as occupied
    And quotas/subnets are computed from the real state, not from the internal ledger

  # --- error / edge cases ---

  Scenario: No free subnet in the parent CIDR fails pre-allocation
    Given the parent CIDR has no free block of the requested size
    When the plan tries to pre-allocate a subnet
    Then pre-allocation fails (no available CIDR) and the run does not provision

  Scenario: Reconcile releases an orphaned allocation
    Given a NetworkAllocation whose run is gone but the lease lingers
    When reconcile runs against the real VPC
    Then the orphan allocation is released (orphans_released > 0)
