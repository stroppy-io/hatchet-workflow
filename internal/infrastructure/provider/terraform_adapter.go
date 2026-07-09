package provider

import (
	"context"
	"io"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
)

// tfActor is the minimal terraform.Actor surface terraformActorExec needs.
// *terraform.Actor satisfies it; tests inject a fake so WorkdirWithParams
// construction can be observed without shelling out to a real terraform
// binary.
type tfActor interface {
	ApplyTerraform(ctx context.Context, w *terraform.WorkdirWithParams) (terraform.TfOutput, error)
	DestroyExisting(ctx context.Context, wd terraform.WdId, opts ...terraform.Option) error
}

// terraformActorExec adapts terraform.Actor (via the tfActor surface) to the
// terraformExec interface terraformProvider depends on. tfFiles is the
// module's embedded HCL (constant per module), env carries provider
// credentials (F1, resolved once per NewProviderForRef call), and
// stdout/stderr are this run's per-run log sink (F2, nil when no LogSinkFn is
// configured — every apply/destroy then falls back to the Actor's own
// default writer, unchanged pre-F2 behavior). All are supplied once at
// construction and reused for every Apply/Destroy call, matching how
// renderTerraformInput (internal/workflows/provider_render.go) builds one
// Terraform_Input per module with a fixed file set and env.
type terraformActorExec struct {
	actor          tfActor
	tfFiles        []terraform.TfFile
	env            map[string]string
	stdout, stderr io.Writer
}

// NewTerraformActorExec builds a terraformExec backed by actor, applying
// tfFiles, env and the per-run stdout/stderr log sink to every workdir it
// constructs. stdout/stderr may be nil (no per-run sink; Apply/Destroy then
// use the terraform.Actor's own default writer).
func NewTerraformActorExec(actor tfActor, tfFiles []terraform.TfFile, env map[string]string, stdout, stderr io.Writer) *terraformActorExec {
	return &terraformActorExec{actor: actor, tfFiles: tfFiles, env: env, stdout: stdout, stderr: stderr}
}

// options returns the file/var-file-name/env/parallelism/state-preservation/
// log-sink option set shared by Apply and Destroy, mirroring the values
// renderTerraformInput uses for a real deploy (DefaultVarFileName,
// parallelism 10, preserve existing state) plus dir/varsJSON as the only
// per-call knobs.
func (a *terraformActorExec) options(varsJSON []byte) []terraform.Option {
	return []terraform.Option{
		terraform.WithTfFiles(a.tfFiles),
		terraform.WithVarFile(terraform.TfVarFile(varsJSON)),
		terraform.WithVarFileName(terraform.DefaultVarFileName),
		terraform.WithEnv(a.env),
		terraform.WithParallelism(10),
		terraform.WithPreserveExistingState(true),
		terraform.WithWorkdirStdout(a.stdout),
		terraform.WithWorkdirStderr(a.stderr),
	}
}

// Apply runs `terraform apply` for dir using varsJSON as the tfvars file
// content, returning the module's raw outputs.
func (a *terraformActorExec) Apply(ctx context.Context, dir string, varsJSON []byte) (map[string][]byte, error) {
	w := terraform.NewWorkdirWithParams(terraform.NewWdId(dir), a.options(varsJSON)...)
	out, err := a.actor.ApplyTerraform(ctx, w)
	if err != nil {
		return nil, err
	}
	return map[string][]byte(out), nil
}

// Destroy runs `terraform destroy` for dir using varsJSON as the tfvars file
// content, with the same file/env/parallelism options Apply uses so a
// workdir that was never applied in this process (e.g. after a restart) is
// still reconstructed identically before tearing it down.
func (a *terraformActorExec) Destroy(ctx context.Context, dir string, varsJSON []byte) error {
	return a.actor.DestroyExisting(ctx, terraform.NewWdId(dir), a.options(varsJSON)...)
}

var _ terraformExec = (*terraformActorExec)(nil)
