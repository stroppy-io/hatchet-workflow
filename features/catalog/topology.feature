# language: en
@catalog @migration
Feature: Topology as a provider-agnostic graph of components

  Topology is a single graph: machines carry components (Kind), components are
  linked by typed connections. The single/ha/replica/scale shapes are not an enum
  but emergent from the graph (replication mode + replicas + access proxy from
  Database.Options). Sizing lives on the Machine and is edited directly (there is
  no separate MachineOverride). Placement/zones live in deployment, not here. Canon:
  domain/topology.proto, deployment/*.proto.

  Scenario: single = one DATABASE node without replication
    Given Database.Options.postgres.replication.mode = SINGLE
    When the topology is built
    Then there is 1 machine with a component Kind=DATABASE
    And there is no connection with Kind=REPLICATION

  Scenario: postgres ha = DATABASE×N + COORDINATOR + PROXY with links
    Given Options.postgres.replication.mode = PATRONI, replicas = 2
    And Options.postgres.access.haproxy = true
    When the topology is built
    Then there are 3 components Kind=DATABASE
    And there is a component Kind=COORDINATOR (etcd)
    And there is a component Kind=PROXY (haproxy)
    And there is a connection Kind=REPLICATION between DATABASE
    And there is a connection Kind=COORDINATION to COORDINATOR
    And there is a connection Kind=PROXY from PROXY to DATABASE

  Scenario: mysql replica = primary + replica with REPLICATION
    Given Options.mysql.replication.mode = ASYNC, replicas = 1
    When the topology is built
    Then there are 2 components Kind=DATABASE
    And there is a connection Kind=REPLICATION primary→replica

  Scenario: A connection references only existing components
    Given a connection from "x" to "missing"
    When the topology is validated
    Then validation fails: target component not found

  @invariant
  Scenario: Every machine with commands has an AGENT component
    Given a machine with a component Kind=DATABASE
    When the topology is validated
    Then this machine has a component Kind=AGENT
    And commands are delivered to AGENT, targeting the neighboring component

  Scenario: One topology materializes across different providers
    Given a provider-agnostic topology
    When it is materialized under PROVIDER_DOCKER
    Then machines → docker containers
    When it is materialized under PROVIDER_YANDEX
    Then machines → yandex_compute_instance with zones

  Scenario: Zones are laid out round-robin at materialization (deployment)
    Given 3 machines and a round-robin strategy for Yandex
    When the topology is materialized into Yandex.Input
    Then the machine zones = ru-central1-a, -b, -d in a cycle

  Scenario: Sizing is edited on the Machine directly, the topology is the source of truth
    Given a DATABASE machine with cores=4
    When the wizard changes cores to 8
    Then Topology.Machine.cores = 8
    And there is no hidden override object
    And the preview shows 8 (effective)

  Scenario: A binding resolves from Deployment.Output
    Given render.Binding {role: DATABASE, attr: PRIVATE_IP}
    When terraform apply returned Yandex.VmOutput.internal_ip
    Then the resolver substitutes the internal_ip of the DATABASE component's machine

  @hole
  Scenario: ExternalDB (BYOD) — a DATABASE component without a machine
    Given Database.Target = external with endpoint "db.example.com:5432"
    When the topology is built
    Then there is a component Kind=DATABASE not attached to a Machine
    And there is a connection STROPPY→this component
    And the endpoint comes from Target.External (not a binding)
    # STRUCTURAL HOLE: proto holds Component only inside Machine.components[].
    # A machine-unattached component has nowhere to live — see the note in topology.proto.
