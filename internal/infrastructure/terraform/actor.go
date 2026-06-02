package terraform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hashicorp/terraform-exec/tfexec"
)

const (
	DefaultWorkingDir     = "/tmp/stroppy-terraform"
	DefaultVarFileName    = "terraform.tfvars.json"
	DefaultConfigFileName = "custom.tfrc"
	defaultParallelism    = 10
)

const tfCliConfigFileEnvKey = "TF_CLI_CONFIG_FILE"

type TfFile interface {
	Content() []byte
	Name() string
}

type tfFile struct {
	content []byte
	name    string
}

func NewTfFile(content []byte, name string) TfFile {
	return &tfFile{content: content, name: name}
}

func (f *tfFile) Content() []byte { return f.content }
func (f *tfFile) Name() string    { return f.name }

type TfVarFile []byte
type TfEnv map[string]string
type TfOutput map[string][]byte
type WdId string

func NewWdId(value string) WdId { return WdId(value) }
func (w WdId) String() string   { return string(w) }

func NewTfVarFile[T any](value T) (TfVarFile, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func GetTfOutputVal[T any](output TfOutput, key string) (T, error) {
	val, ok := output[key]
	if !ok {
		var zero T
		return zero, fmt.Errorf("terraform output %q is missing", key)
	}
	var parsed T
	if err := json.Unmarshal(val, &parsed); err != nil {
		return parsed, fmt.Errorf("decode terraform output %q: %w", key, err)
	}
	return parsed, nil
}

type WorkdirWithParams struct {
	wd                    WdId
	workdirRoot           string
	varFileName           string
	terraformExecPath     string
	tfFiles               []TfFile
	varFile               TfVarFile
	env                   TfEnv
	parallelism           int
	preserveExistingState bool
	destroyOnApplyError   bool
}

type Option func(*WorkdirWithParams)

func NewWorkdirWithParams(wd WdId, opts ...Option) *WorkdirWithParams {
	w := &WorkdirWithParams{
		wd:                wd,
		workdirRoot:       DefaultWorkingDir,
		varFileName:       DefaultVarFileName,
		env:               make(TfEnv),
		parallelism:       defaultParallelism,
		terraformExecPath: defaultExecPath(),
	}
	for _, opt := range opts {
		opt(w)
	}
	if w.workdirRoot == "" {
		w.workdirRoot = DefaultWorkingDir
	}
	if w.varFileName == "" {
		w.varFileName = DefaultVarFileName
	}
	if w.parallelism <= 0 {
		w.parallelism = defaultParallelism
	}
	if w.terraformExecPath == "" {
		w.terraformExecPath = defaultExecPath()
	}
	return w
}

func WithTfFiles(files []TfFile) Option {
	return func(w *WorkdirWithParams) { w.tfFiles = append([]TfFile(nil), files...) }
}

func WithVarFile(file TfVarFile) Option {
	return func(w *WorkdirWithParams) { w.varFile = file }
}

func WithEnv(env TfEnv) Option {
	return func(w *WorkdirWithParams) {
		if env == nil {
			return
		}
		w.env = make(TfEnv, len(env))
		for k, v := range env {
			w.env[k] = v
		}
	}
}

func WithWorkdirRoot(root string) Option {
	return func(w *WorkdirWithParams) { w.workdirRoot = root }
}

func WithVarFileName(name string) Option {
	return func(w *WorkdirWithParams) { w.varFileName = name }
}

func WithTerraformExecPath(path string) Option {
	return func(w *WorkdirWithParams) { w.terraformExecPath = path }
}

func WithParallelism(parallelism int) Option {
	return func(w *WorkdirWithParams) { w.parallelism = parallelism }
}

func WithPreserveExistingState(preserve bool) Option {
	return func(w *WorkdirWithParams) { w.preserveExistingState = preserve }
}

func WithDestroyOnApplyError(destroy bool) Option {
	return func(w *WorkdirWithParams) { w.destroyOnApplyError = destroy }
}

func (w *WorkdirWithParams) WorkdirPath() string {
	return filepath.Join(w.workdirRoot, string(w.wd))
}

func (w *WorkdirWithParams) StateFilePresent() bool {
	_, err := os.Stat(filepath.Join(w.WorkdirPath(), "terraform.tfstate"))
	return err == nil
}

func (w *WorkdirWithParams) createDir() error {
	statePath := filepath.Join(w.WorkdirPath(), "terraform.tfstate")
	if w.preserveExistingState {
		if _, err := os.Stat(statePath); err == nil {
			return nil
		}
	}
	if err := os.RemoveAll(w.WorkdirPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clean terraform workdir: %w", err)
	}
	if err := os.MkdirAll(w.WorkdirPath(), 0o755); err != nil {
		return fmt.Errorf("create terraform workdir: %w", err)
	}
	return nil
}

func (w *WorkdirWithParams) writeFiles() error {
	for _, file := range w.tfFiles {
		target := filepath.Join(w.WorkdirPath(), file.Name())
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create terraform file dir %q: %w", file.Name(), err)
		}
		if err := os.WriteFile(target, file.Content(), 0o644); err != nil {
			return fmt.Errorf("write terraform file %q: %w", file.Name(), err)
		}
	}
	if w.varFile != nil {
		target := filepath.Join(w.WorkdirPath(), w.varFileName)
		if err := os.WriteFile(target, w.varFile, 0o600); err != nil {
			return fmt.Errorf("write terraform var file %q: %w", w.varFileName, err)
		}
	}
	return nil
}

type Actor struct {
	mu       sync.Mutex
	workdirs map[WdId]*WorkdirWithParams
}

var ErrWdAlreadyExists = errors.New("terraform workdir is already active")

func NewActor() (*Actor, error) {
	if defaultExecPath() == "" {
		return nil, errors.New("terraform exec path is empty")
	}
	if err := os.MkdirAll(DefaultWorkingDir, 0o755); err != nil {
		return nil, fmt.Errorf("create terraform root: %w", err)
	}
	if err := writeTfCLIConfig(DefaultWorkingDir); err != nil {
		return nil, err
	}
	return &Actor{workdirs: make(map[WdId]*WorkdirWithParams)}, nil
}

func (a *Actor) PlanTerraform(ctx context.Context, w *WorkdirWithParams) (bool, error) {
	if err := a.register(w); err != nil {
		return false, err
	}
	tf, err := a.prepare(ctx, w)
	if err != nil {
		return false, err
	}
	opts := []tfexec.PlanOption{tfexec.Parallelism(w.parallelism)}
	if w.varFile != nil {
		opts = append(opts, tfexec.VarFile(w.varFileName))
	}
	return tf.Plan(ctx, opts...)
}

func (a *Actor) ApplyTerraform(ctx context.Context, w *WorkdirWithParams) (TfOutput, error) {
	if err := a.register(w); err != nil {
		return nil, err
	}
	tf, err := a.prepare(ctx, w)
	if err != nil {
		return nil, err
	}
	opts := []tfexec.ApplyOption{tfexec.Parallelism(w.parallelism)}
	if w.varFile != nil {
		opts = append(opts, tfexec.VarFile(w.varFileName))
	}
	if err := tf.Apply(ctx, opts...); err != nil {
		if w.destroyOnApplyError {
			_ = tf.Destroy(ctx, tfexec.Parallelism(w.parallelism))
		}
		return nil, fmt.Errorf("terraform apply: %w", err)
	}
	return readOutput(ctx, tf)
}

func (a *Actor) DestroyExisting(ctx context.Context, wd WdId, opts ...Option) error {
	a.mu.Lock()
	w, ok := a.workdirs[wd]
	a.mu.Unlock()
	if ok {
		for _, opt := range opts {
			opt(w)
		}
	} else {
		w = NewWorkdirWithParams(wd, opts...)
		if _, err := os.Stat(w.WorkdirPath()); err != nil {
			return fmt.Errorf("terraform workdir %q is missing at %s: %w", wd, w.WorkdirPath(), err)
		}
		if err := a.register(w); err != nil {
			return err
		}
	}
	tf, err := a.newTerraform(ctx, w)
	if err != nil {
		return err
	}
	defer a.unregister(w.wd)
	return tf.Destroy(ctx, tfexec.Parallelism(w.parallelism))
}

func (a *Actor) register(w *WorkdirWithParams) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.workdirs[w.wd]; ok {
		return ErrWdAlreadyExists
	}
	a.workdirs[w.wd] = w
	return nil
}

func (a *Actor) unregister(wd WdId) {
	a.mu.Lock()
	delete(a.workdirs, wd)
	a.mu.Unlock()
}

func (a *Actor) prepare(ctx context.Context, w *WorkdirWithParams) (*tfexec.Terraform, error) {
	if err := os.MkdirAll(w.workdirRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create terraform root %q: %w", w.workdirRoot, err)
	}
	if err := writeTfCLIConfig(w.workdirRoot); err != nil {
		return nil, err
	}
	if err := w.createDir(); err != nil {
		return nil, err
	}
	if err := w.writeFiles(); err != nil {
		return nil, err
	}
	return a.newTerraform(ctx, w)
}

func (a *Actor) newTerraform(ctx context.Context, w *WorkdirWithParams) (*tfexec.Terraform, error) {
	tf, err := tfexec.NewTerraform(w.WorkdirPath(), w.terraformExecPath)
	if err != nil {
		return nil, fmt.Errorf("create terraform: %w", err)
	}
	tf.SetStdout(os.Stdout)
	tf.SetStderr(os.Stderr)
	if err := tf.SetEnv(mergeEnv(os.Environ(), w.env, map[string]string{
		tfCliConfigFileEnvKey: filepath.Join(w.workdirRoot, DefaultConfigFileName),
	})); err != nil {
		return nil, fmt.Errorf("set terraform env: %w", err)
	}
	if err := tf.Init(ctx); err != nil {
		return nil, fmt.Errorf("terraform init: %w", err)
	}
	return tf, nil
}

func readOutput(ctx context.Context, tf *tfexec.Terraform) (TfOutput, error) {
	out, err := tf.Output(ctx)
	if err != nil {
		return nil, fmt.Errorf("terraform output: %w", err)
	}
	result := make(TfOutput, len(out))
	for key, value := range out {
		result[key] = value.Value
	}
	return result, nil
}

func defaultExecPath() string {
	if path := os.Getenv("TERRAFORM_EXEC_PATH"); path != "" {
		return path
	}
	return "/usr/local/bin/terraform"
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

func writeTfCLIConfig(root string) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create terraform root %q: %w", root, err)
	}
	if err := os.WriteFile(filepath.Join(root, DefaultConfigFileName), []byte(tfrcTemplate), 0o644); err != nil {
		return fmt.Errorf("write terraform CLI config: %w", err)
	}
	return nil
}

func mergeEnv(environ []string, maps ...map[string]string) map[string]string {
	out := make(map[string]string, len(environ))
	for _, kv := range environ {
		key, value, ok := strings.Cut(kv, "=")
		if ok {
			out[key] = value
		}
	}
	for _, values := range maps {
		for key, value := range values {
			out[key] = value
		}
	}
	return out
}
