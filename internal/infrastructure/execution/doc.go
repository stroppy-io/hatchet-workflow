// Package execution implements the non-storage "execution" port interfaces of
// the connect services, wired to this repo's real execution layer: the Temporal
// workflows (internal/workflows + the generated workflowpb clients) and the
// typed domain builders (internal/domain/{run,deployment,topology,settings}).
//
// Each adapter lives in its own file and is constructed with the runtime
// collaborators it needs (a Temporal client.Client, an optional *slog.Logger, an
// optional monitoring base URL, and — where an adapter must bridge into storage
// it does not own — narrow injected ports). The integration layer wires these
// adapters into the service Deps structs.
//
// Adapters and the ports they satisfy:
//
//   - TestWorkflows           -> test_run.Workflows
//   - RunSummarizer           -> test_run.Summarizer
//   - OverviewReader          -> test_run_overview.OverviewReader
//   - LogReader               -> test_run_overview.LogReader
//   - MetricsReader           -> test_run_overview.MetricsReader
//   - TestRunStarter          -> test_wizard.TestRunStarter
//   - TestWizardEngine        -> test_wizard.WizardEngine
//   - SuiteRunLauncher        -> suite.SuiteRunLauncher
//   - SuiteRunCanceller       -> suite_run.SuiteRunCanceller
//
// The monitoring readers degrade gracefully: when no monitoring backend URL is
// configured at construction time they return empty results (this is intentional
// degradation, not a placeholder — there is simply nothing to read).
package execution
