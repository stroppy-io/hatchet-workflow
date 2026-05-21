# language: en
@agent @migration
Feature: Agent lifecycle (poll-only)

  The agent only pulls work: Register → Heartbeat → Poll → Report. The server
  never pushes. The command queue = Dag nodes (no separate table). A command is
  handed out under a lease (NodeAddress + lease_expires_at); a long-running command holds the
  lease via periodic Report{RUNNING}. An expired lease returns the node to the pool
  (replacing the orphan-reaper). Canon: api/agent/agent.proto, runtime/agent/agent.proto.

  Scenario: Register is idempotent per agent
    Given an agent with target and boot_id "b1"
    When the agent calls Register twice
    Then a single Agent exists
    And the status is AGENT_STATUS_REGISTERED

  Scenario: Poll with no work — empty response (long-poll)
    Given there are no ready command nodes for the agent's machine
    When the agent calls Poll
    Then a PollResponse without a lease (on long-poll timeout)

  Scenario: Poll with a ready command hands out a lease
    Given the Dag has a ready node handler="agent.command" for the agent's machine
    When the agent calls Poll
    Then a CommandLease with NodeAddress{dag_id, node_execution_id} is returned
    And command.Operation is set
    And lease_expires_at is in the future

  Scenario: Report{COMPLETED} finishes the node
    Given the agent holds a lease on a node
    When the agent sends Report{COMPLETED} with Operation.Result
    Then the node transitions to STATUS_COMPLETED
    And the node output = Result
    And DagProcessor advances the graph

  Scenario: Report{FAILED} fails the node and triggers retry
    When the agent sends Report{FAILED} with error
    Then the node transitions to STATUS_FAILED
    And the node's retry policy is applied

  @invariant
  Scenario: A long-running command holds the lease via Report{RUNNING}
    Given an agent runs run_stroppy for several hours
    When the agent periodically sends Report{RUNNING}
    Then lease_expires_at is pushed forward by each Report{RUNNING}
    And the node stays STATUS_RUNNING
    And the lease does not expire while progress reports keep coming

  Scenario: An expired lease returns the node to the pool (replacing the reaper)
    Given an agent took a lease and went silent (no Report)
    When lease_expires_at has passed
    Then the node is available again for Poll by another/the same agent
    And no separate orphan-reaper is needed

  Scenario: A new boot_id invalidates the agent's in-flight leases
    Given the agent held leases with boot_id "b1"
    When the agent restarts and calls Register with boot_id "b2"
    Then in-flight leases for "b1" are invalidated immediately
    And their nodes are available for re-issue

  @invariant
  Scenario: The server does not block waiting for the agent
    Given a command node is handed out under a lease
    When the server processes the Dag
    Then the node stays STATUS_RUNNING without blocking wait
    And progress advances only via incoming Report

  Scenario: Addressing is strictly by NodeAddress
    When the agent leases and reports a command
    Then DagId + Node.execution_id is used
    And structural paths/machine-id parsing are not used

  Scenario: Heartbeat updates status and liveness
    When the agent sends Heartbeat with status AGENT_STATUS_READY
    Then Agent.status = READY
    And last_seen_at is updated

  # --- error / edge cases ---

  @invariant
  Scenario: Two agents racing for the same node — only one wins the lease
    Given a ready command node and two agents polling
    When both Poll at the same time
    Then exactly one receives the CommandLease
    And the other gets no lease for that node

  Scenario: A duplicate Report is idempotent
    Given an agent already reported COMPLETED for a command
    When it sends the same Report again
    Then the node stays STATUS_COMPLETED (no double-advance)

  Scenario: A Report on an expired or foreign lease is rejected
    Given a lease that already expired, or belongs to another agent
    When a Report arrives for it
    Then it is rejected and does not mutate the node

  Scenario: A new boot_id invalidates late Reports from the old boot
    Given an agent re-registered with a new boot_id
    When a late Report tagged with the old boot_id arrives
    Then it is ignored (the old lease was invalidated)
