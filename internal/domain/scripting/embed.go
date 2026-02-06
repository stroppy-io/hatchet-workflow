package scripting

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
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

func InstallEdgeWorkerCloudInitFile() *crossplane.WriteFile {
	return &crossplane.WriteFile{
		Path:        InstallEdgeWorkerCloudInitFileContent,
		Content:     string(InstallEdgeWorker),
		Permissions: "0755",
	}
}

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
	Users  []*crossplane.User
	Groups []string
	Ssh    *crossplane.SSHConfig
	Env    map[string]string
}

type Options func(*installWorkerOptions)

func WithUsers(users []*crossplane.User) Options {
	return func(o *installWorkerOptions) {
		o.Users = users
	}
}

func WithGroups(groups []string) Options {
	return func(o *installWorkerOptions) {
		o.Groups = groups
	}
}

func WithSsh(ssh *crossplane.SSHConfig) Options {
	return func(o *installWorkerOptions) {
		o.Ssh = ssh
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
) *crossplane.CloudInit {
	o := &installWorkerOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return &crossplane.CloudInit{
		Users:      o.Users,
		Groups:     o.Groups,
		Ssh:        o.Ssh,
		WriteFiles: []*crossplane.WriteFile{},
		Runcmd: []*crossplane.StringList{
			{
				Values: InstallEdgeWorkerCloudInitCmd(o.Env),
			},
		},
	}
}
