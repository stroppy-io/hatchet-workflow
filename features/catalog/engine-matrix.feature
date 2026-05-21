# language: en
@catalog @migration
Feature: Engine x topology x workload compatibility matrix

  Consolidated cross-engine view: the topology shape each (engine, topology) produces
  (emergent from Database.Options, B5) and each engine's compatibility with the stroppy
  workload (default protocol + supported scripts, backend ScriptCompat, B4). Canon:
  domain/database.proto, domain/topology.proto, domain/workload.proto.

  Scenario Outline: Topology shape per engine and topology
    Given Database engine "<engine>" with topology "<topology>"
    When the topology is built
    Then it has <db_nodes> DATABASE component(s)
    And extra components: <extras>
    And replication: <replication>

    Examples:
      | engine    | topology   | db_nodes        | extras                  | replication        |
      | postgres  | single     | 1               | none                    | none               |
      | postgres  | ha         | N (1+replicas)  | COORDINATOR, PROXY      | PATRONI            |
      | postgres  | scale      | N               | PROXY                   | STREAMING          |
      | mysql     | single     | 1               | none                    | none               |
      | mysql     | replica    | 1 + replicas    | PROXY (proxysql) opt    | ASYNC/SEMI_SYNC    |
      | mysql     | group      | N               | PROXY (proxysql) opt    | GROUP_REPLICATION  |
      | mariadb   | replica    | 1 + replicas    | PROXY (proxysql) opt    | ASYNC/SEMI_SYNC    |
      | picodata  | single     | 1               | none                    | none               |
      | picodata  | cluster    | N (tiers)       | PROXY (haproxy) opt     | sharded            |
      | picodata  | scale      | N (tiers)       | PROXY (haproxy) opt     | sharded            |
      | ydb       | selfhosted | storage+compute | COORDINATOR opt         | distributed        |
      | cockroach | cluster    | N               | none                    | distributed        |

  Scenario Outline: Workload compatibility per engine (default protocol + scripts)
    Given Database engine "<engine>"
    When a Workload with UNSPECIFIED protocol is assembled into a TestPreset
    Then the default protocol is <protocol>
    And script "<supported>" is accepted
    And script "<unsupported>" is rejected (not in ScriptCompat for that kind+protocol)

    Examples:
      | engine    | protocol   | supported          | unsupported        |
      | postgres  | PG         | tpcc/procs         | tpcc/tx-ydb-pgwire |
      | mysql     | MYSQL      | tpcc/tx            | tpcc/tx-ydb-pgwire |
      | mariadb   | MYSQL      | tpcc/procs         | tpcc/tx-ydb-pgwire |
      | picodata  | PICODATA   | tpcc/tx            | tpcc/procs         |
      | ydb       | YDB_GRPC   | tpcc/tx            | tpcc/procs         |
      | cockroach | COCKROACH  | tpcc/tx            | tpcc/procs         |

  Scenario: YDB pgwire uses engine-specific script variants
    Given Database engine "ydb" with Workload protocol YDB_PGWIRE
    Then script "tpcc/tx-ydb-pgwire" is accepted
    And the plain "tpcc/tx" / "tpcc/procs" are not used on that surface

  Scenario: Managed YDB has no DB-side machines
    Given Database.Options.ydb.managed
    When the topology is built
    Then there are no self-hosted DATABASE machines
    And only the stroppy client machine is provisioned
    And the protocol is YDB_GRPCS (port 2135), database path resolved by binding (B4)

  Scenario: External (BYOD) engine has no provisioned DATABASE machine
    Given Database.Target = external for any engine
    When the topology is built
    Then the DATABASE component is machine-less (Topology.external_components, H7)
    And only the stroppy runner is provisioned
