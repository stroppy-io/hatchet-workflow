package execution

import "errors"

// errDraftNotReady is returned by a wizard engine's Bake when asked to bake a
// draft that has not passed its readiness gate.
var errDraftNotReady = errors.New("draft is not ready: resolve validation errors and provider settings first")
