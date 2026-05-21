package terraform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/hashicorp/terraform-exec/tfexec"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/samber/lo"
	"github.com/stroppy-io/stroppy-cloud/internal/old_core/defaults"
	"github.com/stroppy-io/stroppy-cloud/internal/old_core/logger"
	"github.com/stroppy-io/stroppy-cloud/internal/old_core/uow"
)

const (
	Version        = "1.14.5"
	WorkingDir     = "/tmp/stroppy-terraform"
	VarFileName    = "terraform.tfvars.json"
	ConfigFileName = "custom.tfrc"
)

const (
	TfCliConfigFileEnvKey = "TF_CLI_CONFIG_FILE"
)

type TfFile interface {
	Content() []byte
	Name() string
}
type tfFile struct {
	content []byte
	name    string
}

func (f *tfFile) Content() []byte {
	return f.content
}
func (f *tfFile) Name() string {
	return f.name
}
func NewTfFile(content []byte, name string) TfFile {
	return &tfFile{
		content: content,
		name:    name,
	}
}

type TfVarFile json.RawMessage

func NewTfVarFile[T any](val T) (TfVarFile, error) {
	raw, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

type TfEnv map[string]string

type TfOutput map[string][]byte

func GetTfOutputVal[T any](output TfOutput, key string) (T, error) {
	val, ok := output[key]
	if !ok {
		var zero T
		return zero, fmt.Errorf("key %s not found in output", key)
	}
	var out T
	err := json.Unmarshal(val, &out)
	if err != nil {
		return out, fmt.Errorf("error unmarshaling output value: %s", err)
	}
	return out, nil
}

type WdId string

func (w WdId) String() string {
	return string(w)
}

func NewWdId(str string) WdId {
	return WdId(str)
}

const tfrcTemplate = `
provider_installation {
    network_mirror {
        url = "https://terraform-mirror.yandexcloud.net/"
        include = ["registry.terraform.io/*/*"]
    }
    direct {
        exclude = ["registry.terraform.io/*/*"]
    }
}`

type WorkdirWithParams struct {
	wd          WdId
	tfFiles     []TfFile
	varFile     TfVarFile
	env         TfEnv
	workdirPath workdirPath
}

func (w *WorkdirWithParams) String() string {
	return fmt.Sprintf("WdId: %s, WorkdirPath: %s", w.wd, w.workdirPath)
}
func NewWorkdirWithParams(wd WdId, opts ...Options) *WorkdirWithParams {
	w := &WorkdirWithParams{
		wd:          wd,
		env:         make(TfEnv),
		workdirPath: workdirPath(path.Join(WorkingDir, string(wd))),
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

type Options func(*WorkdirWithParams)

func WithTfFiles(files []TfFile) Options {
	return func(w *WorkdirWithParams) {
		w.tfFiles = files
	}
}

func WithVarFile(file TfVarFile) Options {
	return func(w *WorkdirWithParams) {
		w.varFile = file
	}
}

func WithEnv(env TfEnv) Options {
	return func(w *WorkdirWithParams) {
		if env == nil {
			return
		}
		w.env = env
	}
}

// CreateDir prepares the workdir for terraform. CRITICAL: it must NOT
// remove an existing terraform.tfstate — that file is the only record of
// resources already created in the cloud, and wiping it on a recovered
// run causes terraform to re-apply against an empty state, producing a
// second set of VMs (the original ones are orphaned and unreachable
// because no state knows about them).
//
// Strategy:
//   - If workdir exists AND has terraform.tfstate, keep it intact and
//     just rewrite the .tf source files + var file (handled by WriteFiles
//     downstream). Subsequent `terraform apply` becomes a no-op refresh.
//   - If workdir exists but state file is absent (interrupted before
//     first apply), wipe and recreate — the alternative is an orphaned
//     `.terraform/` cache pointing at a stale provider lockfile.
//   - If workdir is missing, create it fresh.
func (w *WorkdirWithParams) CreateDir() error {
	statePath := path.Join(string(w.workdirPath), "terraform.tfstate")
	if _, err := os.Stat(statePath); err == nil {
		// State exists — preserve it. Ensure the directory itself exists
		// (it does, since stat succeeded) and bail out without touching
		// disk further.
		return nil
	}
	if err := os.RemoveAll(string(w.workdirPath)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error cleaning up working directory: %s", err)
	}
	if err := os.MkdirAll(string(w.workdirPath), os.ModePerm); err != nil {
		return fmt.Errorf("error creating working directory: %s", err)
	}
	return nil
}

func (w *WorkdirWithParams) WriteFiles() error {
	for _, file := range w.tfFiles {
		dst := path.Join(string(w.workdirPath), file.Name())
		// File name may contain subpath (e.g. "modules/network/main.tf") when
		// the embed bundles child modules — ensure the parent dir exists.
		if dir := path.Dir(dst); dir != string(w.workdirPath) {
			if err := os.MkdirAll(dir, os.ModePerm); err != nil {
				return fmt.Errorf("error creating tf file dir: %s", err)
			}
		}
		err := os.WriteFile(dst, file.Content(), os.ModePerm)
		if err != nil {
			return fmt.Errorf("error writing tf file: %s", err)
		}
	}
	err := os.WriteFile(path.Join(string(w.workdirPath), VarFileName), w.varFile, os.ModePerm)
	if err != nil {
		return fmt.Errorf("error writing var file: %s", err)
	}
	return nil
}

type Actor struct {
	workdirs cmap.ConcurrentMap[WdId, *WorkdirWithParams]
}

var (
	installedTerraformExecPath = defaults.StringOrDefault(os.Getenv("TERRAFORM_EXEC_PATH"), "/usr/local/bin/terraform")
)

func NewActor() (*Actor, error) {
	if installedTerraformExecPath == "" {
		return nil, fmt.Errorf("TERRAFORM_EXEC_PATH is not set")
	}
	err := os.MkdirAll(WorkingDir, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("error creating working directory: %s", err)
	}
	err = os.WriteFile(
		path.Join(WorkingDir, ConfigFileName),
		[]byte(tfrcTemplate),
		os.ModePerm,
	)
	if err != nil {
		return nil, fmt.Errorf("error writing config file: %s", err)
	}
	return &Actor{
		workdirs: cmap.NewStringer[WdId, *WorkdirWithParams](),
	}, nil
}

var ErrWdAlreadyExists = errors.New("working directory already exists")

func envSliceToMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i > 0 {
			m[kv[:i]] = kv[i+1:]
		}
	}
	return m
}

type workdirPath string

func (a *Actor) newTerraform(ctx context.Context, params *WorkdirWithParams) (*tfexec.Terraform, error) {
	tf, err := tfexec.NewTerraform(string(params.workdirPath), installedTerraformExecPath)
	if err != nil {
		return nil, fmt.Errorf("error running NewTerraform: %s", err)
	}
	tf.SetStdout(os.Stdout)
	tf.SetStderr(os.Stderr)
	err = tf.SetEnv(lo.Assign(
		envSliceToMap(os.Environ()),
		params.env,
		map[string]string{
			TfCliConfigFileEnvKey: path.Join(WorkingDir, ConfigFileName),
		},
	))
	if err != nil {
		return nil, fmt.Errorf("error setting env: %s", err)
	}
	tf.SetLogger(logger.StdLog())
	err = tf.SetLogProvider("TRACE")
	if err != nil {
		return nil, fmt.Errorf("error setting log provider: %s", err)
	}
	err = tf.SetLogCore("TRACE")
	if err != nil {
		return nil, fmt.Errorf("error setting log core: %s", err)
	}
	err = tf.SetLog("TRACE")
	if err != nil {
		return nil, fmt.Errorf("error setting log: %s", err)
	}
	err = tf.Init(ctx)
	if err != nil {
		return nil, fmt.Errorf("error running init: %s", err)
	}
	return tf, nil
}

func (a *Actor) ApplyTerraform(
	ctx context.Context,
	w *WorkdirWithParams,
) (TfOutput, error) {
	unitWork := uow.UnitOfWork()
	_, ok := a.workdirs.Get(w.wd)
	if ok {
		return nil, ErrWdAlreadyExists
	}
	a.workdirs.Set(w.wd, w)
	err := w.CreateDir()
	if err != nil {
		return nil, fmt.Errorf("error creating working directory: %s", err)
	}
	err = w.WriteFiles()
	if err != nil {
		return nil, fmt.Errorf("error writing files: %s", err)
	}
	tf, err := a.newTerraform(ctx, w)
	if err != nil {
		return nil, fmt.Errorf("error creating terraform instance: %s", err)
	}
	unitWork.Add("terraform destroy", func() error {
		defer a.workdirs.Remove(w.wd)
		return tf.Destroy(ctx, tfexec.Parallelism(10))
	})
	err = tf.Apply(
		ctx,
		tfexec.Parallelism(10),
		tfexec.VarFile(VarFileName),
	)
	if err != nil {
		return nil, unitWork.Rollback(fmt.Errorf("error running apply: %s", err))
	}
	out, err := tf.Output(ctx)
	if err != nil {
		return nil, unitWork.Rollback(fmt.Errorf("error running output: %s", err))
	}
	output := make(TfOutput)
	for k, v := range out {
		output[k] = v.Value
	}
	unitWork.Commit()
	return output, nil
}

func (a *Actor) DestroyTerraform(ctx context.Context, wd WdId) error {
	w, ok := a.workdirs.Get(wd)
	if !ok {
		return fmt.Errorf("working directory %s not found. Available directories: %+v", wd, a.workdirs.Items())
	}
	tf, err := a.newTerraform(ctx, w)
	if err != nil {
		return fmt.Errorf("error running NewTerraform: %s", err)
	}
	defer a.workdirs.Remove(wd)
	return tf.Destroy(ctx, tfexec.Parallelism(10))
}

// DestroyExisting runs `terraform destroy` against an on-disk workdir even
// when the in-memory workdirs map is empty — needed after server restart,
// where the Apply ran in a previous process so the actor that did it is
// long gone, but `/tmp/stroppy-terraform/<wd>/terraform.tfstate` is still
// on the persistent volume. Caller supplies env (YC_TOKEN etc) via opts so
// the provider can authenticate.
//
// Errors out if the on-disk directory is missing — there's nothing to
// destroy if the tfstate isn't there.
func (a *Actor) DestroyExisting(ctx context.Context, wd WdId, opts ...Options) error {
	if w, ok := a.workdirs.Get(wd); ok {
		// Already registered — fall through to the standard path. Apply
		// env overrides on top so the caller can refresh YC_TOKEN if it
		// rotated between apply and destroy.
		for _, opt := range opts {
			opt(w)
		}
		tf, err := a.newTerraform(ctx, w)
		if err != nil {
			return fmt.Errorf("error running NewTerraform: %s", err)
		}
		defer a.workdirs.Remove(wd)
		return tf.Destroy(ctx, tfexec.Parallelism(10))
	}
	// Cold path: rebuild the workdir record from the on-disk path.
	w := NewWorkdirWithParams(wd, opts...)
	if _, err := os.Stat(string(w.workdirPath)); err != nil {
		return fmt.Errorf("on-disk workdir for %s not found at %s: %w", wd, w.workdirPath, err)
	}
	a.workdirs.Set(wd, w)
	tf, err := a.newTerraform(ctx, w)
	if err != nil {
		return fmt.Errorf("error running NewTerraform: %s", err)
	}
	defer a.workdirs.Remove(wd)
	return tf.Destroy(ctx, tfexec.Parallelism(10))
}
