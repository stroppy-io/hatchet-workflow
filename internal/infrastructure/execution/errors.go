package execution

import "errors"

// errDraftNotReady is returned by a wizard engine's Bake when asked to bake a
// draft that has not passed its readiness gate.
var errDraftNotReady = errors.New("draft is not ready: resolve validation errors and provider settings first")

// errRecipeRunMissingID is returned by RecipeWorkflows.LaunchRecipeRun when
// the run record it was asked to launch has no id — the deterministic
// workflow id (runRecipeWorkflowID) has nothing to key off.
var errRecipeRunMissingID = errors.New("recipe run record has no id to launch")
