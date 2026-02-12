package terraform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/samber/lo"
	"github.com/stroppy-io/hatchet-workflow/internal/core/consts"
	"github.com/stroppy-io/hatchet-workflow/internal/core/logger"
)

const (
	Version        consts.ConstValue = "1.0.6"
	WorkingDir     consts.ConstValue = "/tmp/stroppy-terraform"
	VarFileName    consts.ConstValue = "terraform.tfvars.json"
	ConfigFileName consts.ConstValue = "custom.tfrc"
)

const (
	TfCliConfigFileEnvKey consts.EnvKey = "TF_CLI_CONFIG_FILE"
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

type Actor struct {
	execPath string
	workdirs map[WdId]string
}

func NewActor() (*Actor, error) {
	installer := &releases.ExactVersion{
		Product: product.Terraform,
		Version: version.Must(version.NewVersion(Version)),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	execPath, err := installer.Install(ctx)
	if err != nil {
		return nil, fmt.Errorf("error installing Terraform: %s", err)
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
		execPath: execPath,
	}, nil
}

var ErrWdAlreadyExists = errors.New("working directory already exists")

func (a *Actor) ApplyTerraform(
	ctx context.Context,
	wd WdId,
	tfFiles []TfFile,
	varFile TfVarFile,
	env TfEnv,
) (TfOutput, error) {
	if env == nil {
		env = make(TfEnv)
	}
	_, ok := a.workdirs[wd]
	if ok {
		return nil, ErrWdAlreadyExists
	}
	newWd := path.Join(WorkingDir, string(wd))
	err := os.RemoveAll(newWd)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("error cleaning up working directory: %s", err)
	}
	err = os.MkdirAll(newWd, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("error creating working directory: %s", err)
	}
	a.workdirs[wd] = newWd
	for _, file := range tfFiles {
		err = os.WriteFile(path.Join(newWd, file.Name()), file.Content(), os.ModePerm)
		if err != nil {
			return nil, fmt.Errorf("error writing tf file: %s", err)
		}
	}
	err = os.WriteFile(path.Join(newWd, VarFileName), varFile, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("error writing var file: %s", err)
	}
	tf, err := tfexec.NewTerraform(newWd, a.execPath)
	if err != nil {
		return nil, fmt.Errorf("error running NewTerraform: %s", err)
	}
	tf.SetStdout(os.Stdout)
	tf.SetStderr(os.Stderr)
	err = tf.SetEnv(lo.Assign(env, map[string]string{
		TfCliConfigFileEnvKey: path.Join(WorkingDir, ConfigFileName),
	}))
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
	err = tf.Apply(
		ctx,
		tfexec.Parallelism(10),
		tfexec.VarFile(VarFileName),
	)
	if err != nil {
		return nil, fmt.Errorf("error running apply: %s", err)
	}
	out, err := tf.Output(ctx)
	if err != nil {
		return nil, fmt.Errorf("error running output: %s", err)
	}
	output := make(TfOutput)
	for k, v := range out {
		output[k] = v.Value
	}
	return output, nil
}

func (a *Actor) DestroyTerraform(ctx context.Context, wd WdId) error {
	newWd := path.Join(WorkingDir, string(wd))
	tf, err := tfexec.NewTerraform(newWd, a.execPath)
	if err != nil {
		return fmt.Errorf("error running NewTerraform: %s", err)
	}
	return tf.Destroy(ctx, tfexec.Parallelism(10))
}
