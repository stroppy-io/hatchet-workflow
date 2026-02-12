package scripting

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
)

//go:embed install-edge-worker.sh
var InstallEdgeWorker []byte

// InstallEdgeWorkerScript returns the raw bash script.
// When embedding in Cloud-Init YAML under 'write_files', use a block scalar (|).
// No extra escaping is needed for the script content itself, provided the indentation is correct.
func InstallEdgeWorkerScript() string {
	return string(InstallEdgeWorker)
}

const InstallEdgeWorkerCloudInitFileContent = "/tmp/install-edge-worker.sh"

func shellEscape(value string) string {
	// Use single-quote escaping for safe shell injection.
	// Example: abc'def -> 'abc'"'"'def'
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func InstallEdgeWorkerCloudInitCmd(env map[string]string) []string {
	scriptB64 := base64.StdEncoding.EncodeToString(InstallEdgeWorker)
	cmdStr := fmt.Sprintf(
		"echo %s | base64 -d > %s && chmod +x %s && %s",
		shellEscape(scriptB64),
		shellEscape(InstallEdgeWorkerCloudInitFileContent),
		shellEscape(InstallEdgeWorkerCloudInitFileContent),
		shellEscape(InstallEdgeWorkerCloudInitFileContent),
	)
	for k, v := range env {
		cmdStr += fmt.Sprintf(" %s=%s", k, shellEscape(v))
	}
	return []string{"bash", "-c", cmdStr}
}

type installWorkerOptions struct {
	User *deployment.VmUser
	Env  map[string]string
}

type Options func(*installWorkerOptions)

func WithUser(user *deployment.VmUser) Options {
	return func(o *installWorkerOptions) {
		o.User = user
	}
}

func WithEnv(env map[string]string) Options {
	return func(o *installWorkerOptions) {
		o.Env = env
	}
}

func WithAddEnv(env map[string]string) Options {
	return func(o *installWorkerOptions) {
		for k, v := range env {
			o.Env[k] = v
		}
	}
}

func InstallEdgeWorkerCloudInit(
	opts ...Options,
) (*deployment.CloudInit, error) {
	o := &installWorkerOptions{}
	for _, opt := range opts {
		opt(o)
	}
	config := &CloudConfig{
		Users: []CloudConfigUser{
			{
				Name:              o.User.GetName(),
				Groups:            strings.Join(o.User.GetGroups(), ","),
				SshAuthorizedKeys: o.User.GetSshAuthorizedKeys(),
				Sudo:              o.User.GetSudoRules(),
				Shell:             o.User.GetShell(),
			},
		},
		RunCmd: [][]string{
			InstallEdgeWorkerCloudInitCmd(o.Env),
		},
	}

	machineScriptBytes, err := config.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to generate cloud-init: %w", err)
	}
	return &deployment.CloudInit{
		Content: string(machineScriptBytes),
		Env:     o.Env,
	}, nil
}
