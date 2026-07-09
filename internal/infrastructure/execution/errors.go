package execution

import "errors"

// errRecipeRunMissingID is returned by RecipeWorkflows.LaunchRecipeRun when
// the run record it was asked to launch has no id — the deterministic
// workflow id (runRecipeWorkflowID) has nothing to key off.
var errRecipeRunMissingID = errors.New("recipe run record has no id to launch")
