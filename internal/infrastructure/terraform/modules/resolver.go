package modules

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
)

// Resolver implements handlers.ModuleResolver against the embedded module
// bundle. It maps each TerraformTask_Module enum to its sub-directory in
// the embedded FS and returns the .tf files plus a JSON-encoded tfvars
// file (terraform accepts terraform.tfvars.json natively).
type Resolver struct{}

// NewResolver returns a Resolver instance. Stateless; safe to share.
func NewResolver() *Resolver { return &Resolver{} }

// moduleDir returns the directory name inside the embedded FS for a given
// TerraformTask_Module. Unknown modules return ("", false).
func moduleDir(m taskspb.TerraformTask_Module) (string, bool) {
	switch m {
	case taskspb.TerraformTask_MODULE_YANDEX:
		return "yandex_cloud", true
	case taskspb.TerraformTask_MODULE_YANDEX_MANAGED_YDB:
		return "yandex_managed_ydb", true
	}
	return "", false
}

// Resolve loads the .tf files for `module` from the embedded FS and
// serialises `vars` as a JSON tfvars file. The handler writes the tfvars
// blob as VarFileName (terraform.tfvars.json) into the workdir.
//
// `vars` is the unstructured tfvars map the builder produced. We do NOT
// re-encode in HCL — terraform accepts terraform.tfvars.json directly,
// which keeps the round-trip lossless (strings, numbers, bools, nested
// objects all survive). Documented limitation: the JSON encoding cannot
// emit HCL heredoc syntax, but no module in this codebase relies on it.
func (r *Resolver) Resolve(module taskspb.TerraformTask_Module, vars map[string]any) ([]terraform.TfFile, terraform.TfVarFile, error) {
	dir, ok := moduleDir(module)
	if !ok {
		return nil, nil, fmt.Errorf("modules.Resolve: unknown module %s", module)
	}

	subFS, err := fs.Sub(FS, dir)
	if err != nil {
		return nil, nil, fmt.Errorf("modules.Resolve: sub %s: %w", dir, err)
	}

	var files []terraform.TfFile
	walkErr := fs.WalkDir(subFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || path.Ext(p) != ".tf" {
			return nil
		}
		content, err := fs.ReadFile(subFS, p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		files = append(files, terraform.NewTfFile(content, p))
		return nil
	})
	if walkErr != nil {
		return nil, nil, fmt.Errorf("modules.Resolve: walk %s: %w", dir, walkErr)
	}
	if len(files) == 0 {
		return nil, nil, fmt.Errorf("modules.Resolve: module %s has no .tf files", dir)
	}

	if vars == nil {
		vars = map[string]any{}
	}
	raw, err := json.Marshal(vars)
	if err != nil {
		return nil, nil, fmt.Errorf("modules.Resolve: marshal tfvars: %w", err)
	}
	return files, terraform.TfVarFile(raw), nil
}
