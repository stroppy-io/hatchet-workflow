# language: en
@provisioning @migration
Feature: Infrastructure provisioning and closing the binding loop

  Topology is materialized into a DeploymentIntent for a provider. terraform/docker
  are executed by the SERVER (YC creds / docker daemon on the control-plane) — these
  are server-locus nodes, not handed to agents via Poll. After apply, the real
  IPs/endpoints from the output resolve render.Binding (the keystone of the entire render chain
  B3→B4→B5→C12→D16→D18). Canon: ops/tf.proto, deployment/*.proto.

  Scenario: Topology materializes into a DeploymentIntent for a provider
    Given a provider-agnostic Topology
    When it materializes for PROVIDER_YANDEX
    Then DeploymentIntent.specs = yandex_vm (+ managed_ydb when managed)
    When it materializes for PROVIDER_DOCKER
    Then DeploymentIntent.specs = docker_container

  @invariant
  Scenario: Provisioning nodes are marked server-locus and are not given to agents
    Given a TfOperation(APPLY) node
    When the agent calls Poll
    Then this node is not offered in a CommandLease
    And it is executed by the server-side handler (control-plane)

  Scenario: TfOperation APPLY records workdir_id BEFORE apply (crash-safe)
    When TfOperation(APPLY) executes
    Then workdir_id is persisted in the Dag node before apply starts
    And on a server crash, teardown can run destroy by this workdir_id

  @invariant
  Scenario: The binding is resolved from TfOperation.Output (keystone)
    Given render.Binding {role: DATABASE, attr: PRIVATE_IP}
    When TfOperation(APPLY) finished with outputs_json (the machine's internal_ip)
    Then the resolver substitutes the real internal_ip into the on-host WRITE_FILE
    And the agent writes the config with the real address

  Scenario: terraform_destroy — always_run, runs on failure and cancellation
    Given apply succeeded but a subsequent node failed
    When the Dag heads to terminal
    Then TfOperation(DESTROY) (always_run) executes
    And it frees the cloud resources (no VM leak)

  Scenario: Crash-safe recovery restores the workdir
    Given the server crashed during apply
    When another server picks up the Dag
    Then preserve_existing_state keeps terraform.tfstate
    And destroy reconstructs the workdir by workdir_id

  Scenario: Managed YDB returns endpoint/db_path into the binding
    Given a DeploymentIntent with managed_ydb
    When apply finished
    Then the ydb endpoint and database_path from the output resolve stroppy bindings (B4)

  Scenario: Docker — internal_ip into the binding, teardown removes resources
    Given PROVIDER_DOCKER
    When the containers are up
    Then ContainerOutput.internal_ip resolves the bindings
    And teardown removes the containers and the network

  Scenario: Agent bootstrap via cloud-init JWT
    Given Yandex.Vm.user_data with a per-machine JWT
    When the VM comes up
    Then the agent is downloaded, starts, and calls Register with the embedded JWT
    And then proceeds to Poll (bridge to D16)

  Scenario: Provider sizing materialization rules
    Given a Machine with an io-m3 disk and memory
    When it materializes into a Yandex.Vm
    Then the io-m3 disk size is rounded up to a multiple of 93 GiB
    And memory is a multiple of the number of cores

  # --- error / edge cases ---

  Scenario: terraform apply failure triggers rollback
    Given TfOperation(APPLY) with destroy_on_apply_error = true
    When apply or output fails after init
    Then a rollback destroy is attempted (Output.rollback.attempted = true)
    And the node fails with the terraform error

  Scenario: A crash mid-apply still lets teardown destroy
    Given workdir_id was persisted before apply and the server crashed mid-apply
    When teardown runs on a recovered server
    Then DESTROY reconstructs the workdir and releases partially-created resources
