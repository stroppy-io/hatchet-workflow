package cloudinit_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/cloudinit"
)

func TestRenderContainsAgentEnvAndUnit(t *testing.T) {
	out, err := cloudinit.Render(cloudinit.Params{
		BinaryURL:    "https://server.example/agent/download",
		ServerAddr:   "control.example:8080",
		AgentPort:    9091,
		MachineID:    "machine-42",
		AgentToken:   "jwt-token-xyz",
		SSHUser:      "ubuntu",
		SSHPublicKey: "ssh-ed25519 AAAAKEY user@host",
		ExtraEnv: map[string]string{
			"STROPPY_LOG_LEVEL": "debug",
		},
	})
	require.NoError(t, err)

	// Header.
	require.True(t, strings.HasPrefix(out, "#cloud-config\n"), "output must start with #cloud-config")

	// agent.env file with the per-machine env vars from params.
	require.Contains(t, out, "path: /etc/stroppy/agent.env")
	require.Contains(t, out, "STROPPY_SERVER_ADDR=control.example:8080")
	require.Contains(t, out, "STROPPY_AGENT_PORT=9091")
	require.Contains(t, out, "STROPPY_MACHINE_ID=machine-42")
	require.Contains(t, out, "STROPPY_AGENT_TOKEN=jwt-token-xyz")

	// ExtraEnv lines rendered.
	require.Contains(t, out, "STROPPY_LOG_LEVEL=debug")

	// systemd unit.
	require.Contains(t, out, "path: /etc/systemd/system/stroppy-agent.service")
	require.Contains(t, out, "Description=Stroppy Agent")
	require.Contains(t, out, "EnvironmentFile=/etc/stroppy/agent.env")
	require.Contains(t, out, "ExecStart="+cloudinit.RemoteBinPath+" agent")

	// runcmd that curls the binary down and enables the service.
	require.Contains(t, out, "runcmd:")
	require.Contains(t, out, "curl -fsSL -o "+cloudinit.RemoteBinPath+" \"https://server.example/agent/download\"")
	require.Contains(t, out, "chmod +x "+cloudinit.RemoteBinPath)
	require.Contains(t, out, "systemctl daemon-reload")
	require.Contains(t, out, "systemctl enable --now stroppy-agent")

	// SSH user and key block.
	require.Contains(t, out, "name: ubuntu")
	require.Contains(t, out, "ssh_authorized_keys:")
	require.Contains(t, out, "- ssh-ed25519 AAAAKEY user@host")
}

func TestRenderDefaults(t *testing.T) {
	// AgentPort 0 and SSHUser "" must fall back to defaults.
	out, err := cloudinit.Render(cloudinit.Params{
		BinaryURL:  "https://server.example/agent",
		ServerAddr: "ctl:8080",
		MachineID:  "m1",
		AgentToken: "tok",
	})
	require.NoError(t, err)

	require.Contains(t, out, "STROPPY_AGENT_PORT=9090")
	require.Equal(t, 9090, cloudinit.DefaultAgentPort)
	require.Contains(t, out, "name: stroppy")
}

func TestRenderSSHKeyBlockOmittedWhenUnset(t *testing.T) {
	out, err := cloudinit.Render(cloudinit.Params{
		BinaryURL:  "https://server.example/agent",
		ServerAddr: "ctl:8080",
		MachineID:  "m1",
		AgentToken: "tok",
		// no SSHPublicKey
	})
	require.NoError(t, err)

	require.NotContains(t, out, "ssh_authorized_keys:")
}

func TestRenderAgentTokenOmittedWhenUnset(t *testing.T) {
	out, err := cloudinit.Render(cloudinit.Params{
		BinaryURL:  "https://server.example/agent",
		ServerAddr: "ctl:8080",
		MachineID:  "m1",
		// no AgentToken
	})
	require.NoError(t, err)

	require.NotContains(t, out, "STROPPY_AGENT_TOKEN=")
}

func TestRenderMultipleExtraEnv(t *testing.T) {
	out, err := cloudinit.Render(cloudinit.Params{
		BinaryURL:  "https://server.example/agent",
		ServerAddr: "ctl:8080",
		MachineID:  "m1",
		AgentToken: "tok",
		ExtraEnv: map[string]string{
			"FOO": "1",
			"BAR": "two",
		},
	})
	require.NoError(t, err)

	require.Contains(t, out, "FOO=1")
	require.Contains(t, out, "BAR=two")
}
