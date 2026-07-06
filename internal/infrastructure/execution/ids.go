package execution

// Deterministic Temporal workflow ids. These MUST match the id expressions baked
// into the generated workflowpb options so a launch, a query and a cancel all
// address the same execution:
//
//   - TestWorkflow:  "test-run/${! test_run.id }"   (test_temporal.pb.go)
//   - SuiteWorkflow: "suite-run/${! suite_run_id }" (test_temporal.pb.go)
//
// We build them by hand (rather than re-running the proto expression) so the
// Cancel/Query paths do not need a request message to derive the id from.

// testWorkflowID is the deterministic TestWorkflow id for a run id.
func testWorkflowID(runID string) string {
	return "test-run/" + runID
}

// suiteWorkflowID is the deterministic SuiteWorkflow id for a suite-run id.
func suiteWorkflowID(suiteRunID string) string {
	return "suite-run/" + suiteRunID
}

// runRecipeWorkflowID is the deterministic RunRecipeWorkflow id for a recipe
// run id — the parent workflow RecipeWorkflows.LaunchRecipeRun starts (see
// recipe_workflows.go). Unlike testWorkflowID/suiteWorkflowID, this is not
// (yet) baked into a generated client's query/cancel options: RunRecipeWorkflow
// has no proto workflow service of its own (internal/workflows/runrecipe.go
// registers it directly against the SDK), so there is no query/cancel path
// wired to this id today — overview's live query still only tries
// testWorkflowID(runID), degrading to the persisted RuntimeState fallback for
// a recipe run (see overview.go's package-level note). Extending the live
// query to try this id too is a documented follow-up.
func runRecipeWorkflowID(runID string) string {
	return "run-recipe/" + runID
}
