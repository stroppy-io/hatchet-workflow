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
