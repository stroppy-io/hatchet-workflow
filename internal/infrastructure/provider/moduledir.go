package provider

import "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"

// YandexModuleDirResolver builds a Deps.ModuleDir resolver for the builtin
// "yandex" terraform module. tfFiles is the module's embedded HCL, loaded
// once by the caller (yandextf.EmbeddedTfFiles() in internal/app/run.go) —
// this function does no I/O, so it is directly unit-testable with a fake
// file set.
//
// The returned dir doubles as the terraform.WdId (see terraform_adapter.go's
// Apply/Destroy: terraform.NewWdId(dir)) — F4: before this, every call
// returned the constant "yandex", so two concurrent runs against the yandex
// provider collided on the same terraform.Actor workdir/state
// (terraform.Actor.register's ErrWdAlreadyExists). Deriving the id from
// runID ("yandex-<runID>") gives each run its own workdir/tfstate; the
// module's HCL content (tfFiles) is unaffected — it is copied into whichever
// per-run workdir terraform.Actor.prepare creates.
//
// An empty runID (ad-hoc callers: tests, or any future direct construction
// that has no run context) falls back to the pre-F4 constant "yandex" so
// those callers are unaffected.
func YandexModuleDirResolver(tfFiles []terraform.TfFile) func(tenantID, runID, name string) (string, []terraform.TfFile, bool) {
	return func(_, runID, name string) (string, []terraform.TfFile, bool) {
		if name != "yandex" {
			return "", nil, false
		}
		wdID := "yandex"
		if runID != "" {
			wdID = "yandex-" + runID
		}
		return wdID, tfFiles, true
	}
}
