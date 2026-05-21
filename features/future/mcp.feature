# language: en
@future @mcp
Feature: MCP server — drive the benchmark cloud from AI assistants (roadmap)

  Forward-looking, NOT a migration recast. An MCP (Model Context Protocol) server
  exposes the benchmark cloud to AI assistants/agents as typed tools, backed by the
  same proto API (no separate business logic). Auth via an API token (H57): the
  token's tenant + capped role scope every tool call.

  Scenario: MCP tools mirror the read API
    Given an MCP client authenticated with a tenant API token
    Then it can call tools: list_presets, get_run_status, query_metrics,
      compare_runs, query_logs, build_log_link
    And each maps to the corresponding proto RPC, scoped to the token's tenant

  Scenario: Write tools are gated by the token role
    Given an API token with role VIEWER
    When the MCP client calls submit_run
    Then it is denied (write tools require role >= ADMIN, capped by the token)

  Scenario: Submitting a run via MCP
    Given a token with role ADMIN
    When the assistant calls submit_run with a TestPreset
    Then a run is created (RunService.SubmitTestRun) and the run id is returned

  @invariant
  Scenario: MCP adds no privilege or logic beyond the API
    Given any MCP tool call
    Then it goes through the same auth + tenant scoping + validation as the proto API
    And cannot do anything an equivalent API token could not
