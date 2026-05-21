# language: en
@agent @migration
Feature: Agent command execution and setup recipes

  The agent is a generic executor of six ops (WRITE_FILE/APPEND_FILE/MAKE_DIR/
  MAKE_TEMP_DIR/READ_OS_INFO/RUN_CMD). No per-engine knowledge: all component
  install/configure logic = scheduler data, expanded into a
  sequence of ops inside a command sub_dag. A systemd Service is decomposed by the
  scheduler. Canon: ops/operation.proto, system/cmd.proto, system/service.proto.

  @invariant
  Scenario: The agent only executes ops, with no knowledge of engines
    Given a command node with Operation
    When the agent executes the command
    Then it applies one of the 6 ops and reports a Result
    And it contains no installPostgres/configMySQL functions etc.

  Scenario: Package install expands into sub_dag ops
    Given a builtin recipe for (postgres, 16)
    When the scheduler compiles the install
    Then sub_dag: WRITE_FILE(sources.list) → RUN_CMD(apt update) → RUN_CMD(apt install)
    And the recipe came from scheduler data, not from the agent

  Scenario: A custom .deb is installed via File.Ref + RUN_CMD + deb_token binding
    Given Database.config specifies a custom .deb
    When the scheduler compiles the install
    Then there is an Item File.Ref(uri) for the .deb and RUN_CMD(apt install)
    And deb_token is substituted by render.Binding (a runtime secret)

  Scenario: Configuration is written by ops with binding resolution
    Given a rendered postgresql.conf with a PRIVATE_IP binding
    When the scheduler compiles the configure step
    Then WRITE_FILE(postgresql.conf) with the already-resolved address
    And then Service (postgresql) start is applied

  Scenario: A systemd Service is decomposed by the scheduler into ops
    Given a render-item Service(systemd, start=true, daemon_reload=true)
    When the scheduler compiles
    Then nodes: WRITE_FILE(unit_file) → RUN_CMD(systemctl daemon-reload) → RUN_CMD(systemctl enable/start)
    And Operation has no separate service branch (the agent is dumb)

  Scenario: RUN_CMD is idempotent via expected_exit_codes
    Given RUN_CMD apt install with expected_exit_codes [0]
    When the package is already installed and the command is repeated (node retry)
    Then the outcome is interpreted by exit_code and the node does not fail falsely

  Scenario: A long-running command streams logs and holds the lease
    Given RUN_CMD run_stroppy (foreground, hours)
    When the command executes
    Then stdout/stderr are streamed as LogLine (SendLogs)
    And a periodic Report{RUNNING} holds the lease (D16)

  Scenario: Host bootstrap — an explicit preamble node
    Given a fresh machine with an agent
    When the plan for the machine is compiled
    Then the first command node = bootstrap (RUN_CMD: release the apt-lock, base deps)
    And it is visible in the graph and retryable
    And subsequent install nodes depend on it

  @invariant
  Scenario: Component recipes = the same ops, different scheduler data
    Given components Patroni, etcd, HAProxy, ydb
    When the scheduler compiles their setup
    Then each = a sequence of the same 6 ops
    And the differences are only in data (files/commands), not in agent code
