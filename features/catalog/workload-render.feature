# language: en
@catalog @migration
Feature: Rendering a workload into a stroppy config

  A Workload is ONLY the load: script, protocol, k6 execution profile, parameters
  and run-scoped files. Installing the DB engine and packages does NOT belong here
  (that is the Database intent and provisioning). The backend renders the Workload
  into stroppy.RunConfig protojson. driverType and URL come from the protocol
  registry; the URL host/port is a runtime binding (known after provision). Canon:
  domain/workload.proto.

  Background:
    Given a test with a DB engine "postgres" version "16"
    And a Workload intent with script "tpcc/tx"

  Scenario: Empty Protocol → engine default
    Given Workload.protocol = UNSPECIFIED
    When the workload is rendered
    Then protocol PG is selected
    And driverType = "postgres"

  Scenario: An unsupported protocol for the engine is rejected
    Given Workload.protocol = MYSQL
    When a TestPreset is assembled (postgres + workload)
    Then validation fails: the engine does not speak this protocol

  Scenario Outline: The script is checked against the ScriptCompat matrix
    Given engine "<kind>", protocol <protocol>, script "<script>"
    When a TestPreset is assembled
    Then the validation result = <valid>

    Examples:
      | kind     | protocol   | script              | valid    |
      | postgres | PG         | tpcc/tx             | accepted |
      | postgres | PG         | tpcc/tx-ydb-pgwire  | rejected |
      | ydb      | YDB_PGWIRE | tpcc/tx-ydb-pgwire  | accepted |
      | picodata | PICODATA   | tpcc/procs          | rejected |

  @invariant
  Scenario: stroppy-config.json — preview is byte-for-byte equal to the executable, except the URL binding
    When the workload is rendered in preview
    And the same workload is planned into a file for the stroppy component
    Then the stroppy-config.json content matches byte-for-byte
    Except for the render.Binding for host/port in the Url field

  Scenario Outline: The URL shape depends on the driver (single source, no divergence)
    Given protocol <protocol>
    When the connection string is built for host "H" port "P"
    Then Url = "<url>"

    Examples:
      | protocol  | url                                        |
      | PG        | postgresql://H:P/postgres?sslmode=disable  |
      | MYSQL     | root@tcp(H:P)/                             |
      | YDB_GRPC  | grpc://H:P/Root/testdb                     |
      | COCKROACH | postgresql://H:P/defaultdb?sslmode=disable |

  Scenario: Managed YDB uses grpcs and a dynamic db-path
    Given a DB engine "ydb" with Options.ydb.managed
    When the workload is rendered
    Then protocol = YDB_GRPCS, port = 2135
    And the database path is substituted by a binding (known after terraform apply)

  Scenario: Execution profile — duration and iterations are mutually exclusive
    When Execution contains both duration and iterations
    Then validation fails (oneof limit)

  Scenario: steps and no_steps are mutually exclusive
    Given Parameters.steps = ["load_data"] and Parameters.no_steps = ["workload"]
    When the workload is validated
    Then validation fails: steps ⊕ no_steps

  Scenario: A fractional scale_factor is valid for smoke
    Given Parameters.scale_factor = 0.01
    When the workload is validated
    Then validation passes

  Scenario: A WorkloadFile is staged next to stroppy-config.json
    Given Workload.files contains a file "probe.sql" of kind "sql"
    When the workload is planned
    Then "probe.sql" lands in the run directory next to stroppy-config.json
