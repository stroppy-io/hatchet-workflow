// Package terraform runs Terraform for the control-plane terraform_apply /
// terraform_destroy task handlers. It consumes the canonical ops.TfOperation:
// writes the embedded .tf files + terraform.tfvars.json into
// /tmp/stroppy-terraform/<workdir_id>, runs init/apply (or destroy), and exposes
// outputs as raw JSON (Output.outputs_json — the keystone for render.Binding,
// H27). Recast of internal/old/infrastructure/terraform.
package terraform

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gopherex/xlog"
	"github.com/hashicorp/terraform-exec/tfexec"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/system"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

const (
	defaultRoot        = "/tmp/stroppy-terraform"
	varFileName        = "terraform.tfvars.json"
	defaultExecPath    = "/usr/local/bin/terraform"
	defaultParallelism = 10
)

// Runner executes Terraform operations against on-disk workdirs.
type Runner struct {
	*tracing.Entity
	root     string
	execPath string
}

// New builds a Runner. TERRAFORM_EXEC_PATH overrides the terraform binary path.
func New(logger *xlog.Logger) *Runner {
	execPath := os.Getenv("TERRAFORM_EXEC_PATH")
	if execPath == "" {
		execPath = defaultExecPath
	}
	return &Runner{
		Entity:   tracing.NewEntity(logger.AppendName("Terraform")),
		root:     defaultRoot,
		execPath: execPath,
	}
}

// Run dispatches a TfOperation by action and returns its Output.
func (r *Runner) Run(ctx context.Context, op *ops.TfOperation) (*ops.TfOperation_Output, error) {
	in := op.GetInput()
	if in == nil {
		return nil, fmt.Errorf("terraform: operation has no input")
	}
	switch in.GetAction() {
	case ops.TfOperation_ACTION_APPLY:
		return r.apply(ctx, in)
	case ops.TfOperation_ACTION_DESTROY:
		return r.destroy(ctx, in)
	default:
		return nil, fmt.Errorf("terraform: unsupported action %s", in.GetAction())
	}
}

func (r *Runner) apply(ctx context.Context, in *ops.TfOperation_Input) (*ops.TfOperation_Output, error) {
	wd := r.workdir(in)
	if err := r.prepareWorkdir(wd, in); err != nil {
		return failed(in), err
	}
	tf, err := r.newTerraform(ctx, wd, in)
	if err != nil {
		return failed(in), err
	}
	if err := tf.Apply(ctx, tfexec.Parallelism(parallelism(in)), tfexec.VarFile(varFileName)); err != nil {
		if in.GetDestroyOnApplyError() {
			_ = tf.Destroy(ctx, tfexec.Parallelism(parallelism(in)))
		}
		return failed(in), fmt.Errorf("terraform apply: %w", err)
	}
	outs, err := tf.Output(ctx)
	if err != nil {
		return failed(in), fmt.Errorf("terraform output: %w", err)
	}
	outputs := make(map[string][]byte, len(outs))
	for k, v := range outs {
		outputs[k] = []byte(v.Value)
	}
	return &ops.TfOperation_Output{
		Status:           primitive.Status_STATUS_COMPLETED,
		WorkdirId:        in.GetWorkdirId(),
		Workdir:          dirAt(wd),
		StateFilePresent: true,
		OutputsJson:      outputs,
	}, nil
}

func (r *Runner) destroy(ctx context.Context, in *ops.TfOperation_Input) (*ops.TfOperation_Output, error) {
	wd := r.workdir(in)
	// On a recovered destroy the module files may need re-writing next to the
	// persisted terraform.tfstate.
	if len(in.GetFiles()) > 0 {
		if err := r.prepareWorkdir(wd, in); err != nil {
			return failed(in), err
		}
	}
	tf, err := r.newTerraform(ctx, wd, in)
	if err != nil {
		return failed(in), err
	}
	if err := tf.Destroy(ctx, tfexec.Parallelism(parallelism(in))); err != nil {
		return failed(in), fmt.Errorf("terraform destroy: %w", err)
	}
	return &ops.TfOperation_Output{
		Status:    primitive.Status_STATUS_COMPLETED,
		WorkdirId: in.GetWorkdirId(),
		Workdir:   dirAt(wd),
	}, nil
}

// prepareWorkdir creates the dir and writes the module files + tfvars.
func (r *Runner) prepareWorkdir(wd string, in *ops.TfOperation_Input) error {
	if err := os.MkdirAll(wd, 0o755); err != nil {
		return fmt.Errorf("terraform: mkdir %s: %w", wd, err)
	}
	for _, f := range in.GetFiles() {
		if err := writeFile(wd, f.GetInfo().GetPath(), fileContent(f)); err != nil {
			return err
		}
	}
	if vf := in.GetVarFile(); vf != nil {
		if err := writeFile(wd, varFileName, fileContent(vf)); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) newTerraform(ctx context.Context, wd string, in *ops.TfOperation_Input) (*tfexec.Terraform, error) {
	execPath := r.execPath
	if p := in.GetTerraformExecPath(); p != "" {
		execPath = p
	}
	tf, err := tfexec.NewTerraform(wd, execPath)
	if err != nil {
		return nil, fmt.Errorf("terraform: init exec: %w", err)
	}
	env := make(map[string]string)
	for _, kv := range os.Environ() {
		if i := indexByte(kv, '='); i > 0 {
			env[kv[:i]] = kv[i+1:]
		}
	}
	for k, v := range in.GetEnv() {
		env[k] = v
	}
	if err := tf.SetEnv(env); err != nil {
		return nil, fmt.Errorf("terraform: set env: %w", err)
	}
	if err := tf.Init(ctx); err != nil {
		return nil, fmt.Errorf("terraform init: %w", err)
	}
	return tf, nil
}

func (r *Runner) workdir(in *ops.TfOperation_Input) string {
	if p := in.GetWorkdir().GetInfo().GetPath(); p != "" {
		return p
	}
	return filepath.Join(r.root, in.GetWorkdirId())
}

func parallelism(in *ops.TfOperation_Input) int {
	if p := in.GetParallelism(); p > 0 {
		return int(p)
	}
	return defaultParallelism
}

func failed(in *ops.TfOperation_Input) *ops.TfOperation_Output {
	return &ops.TfOperation_Output{Status: primitive.Status_STATUS_FAILED, WorkdirId: in.GetWorkdirId()}
}

func dirAt(path string) *system.Dir {
	return &system.Dir{Info: &system.Dir_Info{Path: path}}
}

func writeFile(wd, rel string, content []byte) error {
	full := filepath.Join(wd, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("terraform: mkdir for %s: %w", rel, err)
	}
	if err := os.WriteFile(full, content, 0o600); err != nil {
		return fmt.Errorf("terraform: write %s: %w", rel, err)
	}
	return nil
}

// fileContent extracts inline text content.
//
// TODO(terraform): only inline text File_Content is handled; File_Ref (s3://)
// fetching is not wired. Reported.
func fileContent(f *system.File) []byte {
	return []byte(f.GetContent().GetText())
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
