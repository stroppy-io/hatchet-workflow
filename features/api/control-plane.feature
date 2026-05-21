# language: en
@api @migration
Feature: Control-plane API (F)

  RPC services on top of proto (connect-go). Identity/catalog/agent already existed
  (Auth/Account/Tenant/Settings/Preset/Agent); operational ones were added:
  RunService (+ metrics/comparison/share folded in here — all about run) and
  SuiteService. A run = submit a snapshot preset → a Dag is created (D14), status =
  Dag.status. Live logs are server-streaming (F2). List — typed per-model
  query (H42), tenant-scope (A). Canon: api/ui/{run,suite}.proto.

  Scenario: Running a test creates a Dag
    Given a TestPreset (snapshot, B6)
    When RunService.SubmitTestRun
    Then a models.Dag is created (D14), TestRun{dag} is returned
    And the run status = Dag.status

  Scenario: Run status and cancellation
    When RunService.GetTestRun
    Then a TestRun is returned with the status from Dag.status
    When RunService.CancelTestRun
    Then Dag → CANCELLING → always_run teardown (D14)

  Scenario: Live logs via server-streaming
    Given an active run
    When RunService.StreamTestRunLogs
    Then the server streams runtime.agent.LogLine (connect-go server-stream)
    And the UI renders them as they arrive

  Scenario: Running a suite
    Given a SuitePreset
    When SuiteService.LaunchSuiteRun
    Then a suite Dag with dag_ref to the test Dags; scheduling from SuitePreset (B6)

  Scenario: Metrics and comparison (on RunService)
    When RunService.GetRunMetrics
    Then RunMetrics is returned (E, self-describing)
    When RunService.CompareRuns(a,b,threshold)
    Then a Comparison with verdicts is returned

  Scenario: Share without auth (on RunService)
    When RunService.CreateShareLink
    Then a token + url is returned
    When RunService.GetSharedRun(token) without authorization
    Then a frozen snapshot + metrics is returned (G5)
    And this is the only public RPC of the service

  @invariant
  Scenario: All RPCs (except share/agent) are authorized by tenant
    Given the caller is a tenant member
    When they call RunService/PresetService/...
    Then access by TenantMember.Role (A); another tenant — denied
    And ListTestRuns/ListPresets return only the rows of their tenant

  Scenario: List — a typed per-model query
    When ListTestRuns with typed fields (status, page)
    Then a generic CommonQuery with string filters is not used (H42)
