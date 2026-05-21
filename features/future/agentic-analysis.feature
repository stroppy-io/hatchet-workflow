# language: en
@future @ai
Feature: Agentic run analysis with UI (roadmap)

  Forward-looking, NOT a migration recast. An AI agent analyzes a finished run —
  its metric summaries (RunMetrics, E), a comparison vs a baseline (Comparison),
  and logs (logs feature) — and produces a natural-language diagnosis with
  recommendations, surfaced in the UI as a "Run Analysis" panel. Server-side LLM;
  tenant-scoped; reuses existing run data. Canon TBD.

  Scenario: On-demand analysis of a finished run
    Given a completed run with metrics and logs
    When the user requests "Analyze run" in the UI
    Then an agent reads RunMetrics + logs (+ optional baseline Comparison)
    And returns a NL summary: throughput/latency verdict, anomalies, likely causes, next steps

  Scenario: The analysis cites concrete evidence
    Given an analysis result
    Then it references specific metrics (key/value) and links to relevant log lines via LogRef (deep-link)
    And does not invent metrics not present in the run

  Scenario: Comparison-driven analysis vs a baseline
    Given a run and a chosen baseline run
    When analysis runs
    Then it explains the per-metric BETTER/WORSE/SAME verdicts (Comparison) in plain language

  Scenario: Analysis is tenant-scoped and stored with the run
    Given a tenant member requests analysis
    Then the agent only sees that tenant's run data (A)
    And the result is cached/stored against the run for re-view

  Scenario: Optional auto-analysis on completion
    Given a tenant policy enabling auto-analysis
    When a run reaches a terminal status
    Then analysis is produced automatically and shown in the UI
