# language: en
@results @migration
Feature: Run logs — by DAG stage and by component, with links

  Logs are only for a run (suite-dag = system dispatch, visible from Dag). The unified
  model `runtime.logs.LogLine` generalizes both sources: agent command stdout/stderr
  And component/DB logs (Vector: journald + /var/log files). Two axes via labels:
  stage (node_execution_id) and component (component_id). Storage is VictoriaLogs
  (LogsQL, per-tenant AccountID). API on RunService (tenant_id in requests). Canon:
  runtime/logs/logs.proto, api/ui/run.proto.

  Scenario: Unifying sources into a single LogLine
    Given agent command stdout, journald of a systemd service and a line from a postgresql log file
    When they are indexed
    Then each → LogLine{observed_at, dag_id, node_execution_id, component_id, machine_id, source(COMMAND|JOURNALD|FILE), unit, stream, line, cursor}

  Scenario: DAG stage logs
    When RunService.QueryRunLogs with node_execution_id
    Then the lines of that stage are returned (the node's command ops)

  Scenario: Component logs (DB from file or stdout)
    When QueryRunLogs with component_id (e.g. DATABASE)
    Then the component logs are returned: journald + DB files (postgres/mysql) in a generic way
    And continuous DB logs are bound to component_id, not to a single node

  Scenario: Live stream of run logs
    When RunService.StreamTestRunLogs (optional filter by stage/component)
    Then the server streams unified runtime.logs.LogLine (connect-go)

  Scenario: Component log link
    When RunService.BuildLogLink with component_id
    Then a deep link (LogsQL+time) + LogRef{component_id} is returned

  @invariant
  Scenario: Link to a specific line
    Given a line with cursor{observed_at, seq}
    When BuildLogLink with this LogCursor
    Then the deep link opens the log viewer at this line
    And the anchor is stable (observed_at + seq dedups equal ts)

  @invariant
  Scenario: Isolation and tenant scope
    Given QueryRunLogs with tenant_id
    Then logs are filtered per-tenant (VictoriaLogs AccountID) and by run
    And a foreign tenant cannot see the logs

  Scenario: Vector enriches both axes with labels
    Given Vector on a machine sends component logs to VictoriaLogs
    Then enrich adds component_id (+ node_execution_id for command-driven)
    # the old enrich had run_id/machine_id/role/unit — add component_id/node_execution_id
