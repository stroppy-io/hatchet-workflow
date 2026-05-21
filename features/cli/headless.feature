# language: en
@cli @migration
Feature: CLI / headless client (G7)

  The product works without a UI: the CLI is a thin connect-go client of the same
  proto API. No separate business logic — it reuses the domain (compile, validate,
  submit) server-side. Built for CI/automation: auth via an API token, JSON output,
  meaningful exit codes. A config file is a TestPreset intent (Database + Workload).
  Canon: cmd/cloud_*; api/ui services; same proto messages as the UI.

  # --- global concerns ---

  Scenario: Global flags select server, tenant, output, and credentials
    Given the CLI is invoked
    Then `--server` sets the API endpoint
    And `--tenant` sets the tenant_id sent on every request (A)
    And `-o json|table` selects output format (json for CI parsing)
    And the API token comes from `--api-token` or env STROPPY_API_TOKEN

  Scenario: Headless auth uses an API token (no interactive login)
    Given env STROPPY_API_TOKEN set to a valid tenant API token (H57)
    When any command runs
    Then it authenticates as that token's tenant + capped role
    And no browser/interactive login is needed

  Scenario: Interactive login stores credentials
    When `cloud auth login --email a@b.c`
    Then it calls AuthService.Login and stores the TokenPair in a credentials file
    When `cloud auth whoami`
    Then it prints the caller's Account (Me)

  # --- core commands ---

  Scenario: validate checks the assembled TestPreset, no run
    When `cloud validate -c config.json`
    Then cross-entity validation runs (B6: script×protocol×kind, stroppy.version)
    And errors print; exit code is non-zero on invalid; nothing is submitted

  Scenario: dry-run compiles and prints the Dag without running
    When `cloud dry-run -c config.json`
    Then the planner compiles the Dag (C12) and prints it
    And nothing is started

  Scenario: run submits a test run
    When `cloud run -c config.json`
    Then SubmitTestRun is called with the assembled TestPreset
    And the new run id is printed

  Scenario: run --wait blocks until terminal and sets the exit code
    When `cloud run -c config.json --wait`
    Then the CLI polls GetTestRun until Dag.status is terminal (D14)
    And exit code 0 on STATUS_COMPLETED, non-zero on FAILED/CANCELLED

  Scenario: wait attaches to an existing run
    When `cloud wait <run-id>`
    Then it polls until terminal and exits with the run's result code

  Scenario: compare two runs
    When `cloud compare <a> <b> --threshold 5`
    Then CompareRuns returns a Comparison (E20) printed as table or json

  Scenario: probe a workload before committing
    When `cloud probe -c config.json`
    Then the server runs stroppy probe (G4) and prints env/steps/sql/drivers

  Scenario: logs query and follow
    When `cloud logs <run-id> --component <id>` or `--stage <execution_id>`
    Then QueryRunLogs returns matching lines (logs feature)
    When `--follow`
    Then it streams via StreamTestRunLogs

  Scenario: upload a custom .deb package
    When `cloud upload my.deb --kind postgres --version 16`
    Then RequestPackageUpload returns a presigned PUT URL
    And the CLI uploads the file to S3 directly (G2)

  Scenario: packages lists the catalog
    When `cloud packages`
    Then ListPackages returns builtin + custom packages (B4)

  # --- error / edge cases ---

  Scenario: A missing or unreadable config file fails fast
    When `cloud run -c missing.json`
    Then it errors before any API call, non-zero exit

  Scenario: A server/auth error surfaces clearly with a non-zero exit
    Given an invalid API token or unreachable server
    When any command runs
    Then a clear error is printed and the exit code is non-zero

  Scenario: JSON output is machine-parseable for CI
    Given `-o json`
    When a command produces output
    Then it emits a single well-formed JSON document (no prose/log noise on stdout)
