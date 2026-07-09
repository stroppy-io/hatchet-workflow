package execution

// runRecipeWorkflowID is the deterministic RunRecipeWorkflow id for a recipe
// run id — the parent workflow RecipeWorkflows.LaunchRecipeRun starts (see
// recipe_workflows.go). Unlike testWorkflowID/suiteWorkflowID, this is not
// baked into a generated client's query/cancel options: RunRecipeWorkflow has
// no proto workflow service of its own (internal/workflows/runrecipe.go
// registers it directly against the SDK). OverviewReader.Get still queries it
// directly by this id for recipe runs (rec.GetRecipeId() != "") — see
// overview.go — using the same GetRunState query name RunRecipeWorkflow
// registers (subproject 1D-T3), so the live query works without a generated
// client wired to this id.
func runRecipeWorkflowID(runID string) string {
	return "run-recipe/" + runID
}
