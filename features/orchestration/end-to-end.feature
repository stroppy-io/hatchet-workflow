# language: en
@orchestration @e2e @migration
Feature: End-to-end run lifecycle (the product spine stitched together)

  One integration view of Define → Render → Plan → Execute → Collect, exercising
  every area: catalog snapshot (B6), compile to Dag (C12), provisioning + binding
  resolution (D18), agent pull execution (D16), scheduling/status (D14), metrics
  and logs (E, logs), teardown (always_run), and cost (= QuotaRequest, D19). Canon:
  the per-area features; this ties them into a single flow.

  @invariant
  Scenario: Happy-path run end to end (postgres single)
    Given a TestPreset snapshot for postgres single (B6)
    When RunService.SubmitTestRun is called
    Then the planner compiles a test Dag: render_config -> terraform_apply -> install_and_run -> collect_results -> terraform_destroy (C12)
    And terraform_apply is a server-locus node; install_and_run is an agent command sub_dag (D17/D18)
    When terraform_apply finishes
    Then render.Binding values resolve from Deployment.Output (real IPs) (D18)
    And the agent polls, leases, executes the command sub_dag and reports (D16)
    And run status follows Dag.status to STATUS_COMPLETED (D14)
    And metrics are collectible (E) and logs queryable by stage/component (logs)
    And terraform_destroy runs via its edge after collect_results
    And the run cost is its QuotaRequest set (D19)

  Scenario: A mid-run failure still tears down (no VM leak)
    Given a run whose install_and_run fails after terraform_apply
    When the Dag reaches terminal
    Then ordinary downstream nodes do not run
    But terraform_destroy (always_run) is force-run (engine: always_run on failure)
    And run status is STATUS_FAILED
    And provisioned cloud resources are released

  Scenario: External (BYOD) run skips provisioning
    Given a TestPreset whose Database.Target = external endpoint
    When the run is compiled
    Then there are no provision/install DATABASE nodes (B5/C12)
    And only the stroppy runner is provisioned and pointed at the endpoint
    And metrics/logs are still collected and teardown is minimal
