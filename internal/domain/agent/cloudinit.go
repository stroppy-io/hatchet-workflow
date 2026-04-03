package agent

import (
	"bytes"
	"fmt"
	"text/template"
)

// CloudInitParams holds parameters for generating the cloud-init userdata script.
type CloudInitParams struct {
	// BinaryURL is the HTTP URL where the agent binary can be downloaded.
	// Typically the server's /agent/binary endpoint or an S3 presigned URL.
	BinaryURL string
	// ServerAddr is the callback address (http://host:port) the agent reports to.
	ServerAddr string
	// AgentPort is the port the agent will listen on.
	AgentPort int
	// MachineID is a unique identifier for this machine, used in agent registration.
	MachineID string
	// SSHUser is the login user created on the VM (default "stroppy").
	SSHUser string
	// SSHPublicKey is the SSH public key added to the user's authorized_keys.
	SSHPublicKey string
	// ExtraEnv is optional environment variables passed to the agent.
	ExtraEnv map[string]string
}

var cloudInitTmpl = template.Must(template.New("cloudinit").Parse(`#cloud-config
users:
  - name: {{.SSHUser}}
    groups: sudo
    shell: /bin/bash
    sudo: ALL=(ALL) NOPASSWD:ALL
{{- if .SSHPublicKey}}
    ssh_authorized_keys:
      - {{.SSHPublicKey}}
{{- end}}

write_files:
  - path: /etc/stroppy/agent.env
    content: |
      STROPPY_SERVER_ADDR={{.ServerAddr}}
      STROPPY_AGENT_PORT={{.AgentPort}}
      STROPPY_MACHINE_ID={{.MachineID}}
{{- range $k, $v := .ExtraEnv}}
      {{$k}}={{$v}}
{{- end}}

  - path: /etc/systemd/system/stroppy-agent.service
    content: |
      [Unit]
      Description=Stroppy Agent
      After=network-online.target
      Wants=network-online.target

      [Service]
      Type=simple
      EnvironmentFile=/etc/stroppy/agent.env
      ExecStart={{.BinPath}} agent
      Restart=on-failure
      RestartSec=5

      [Install]
      WantedBy=multi-user.target

runcmd:
  - mkdir -p /etc/stroppy
  - curl -fsSL -o {{.BinPath}} "{{.BinaryURL}}"
  - chmod +x {{.BinPath}}
  - systemctl daemon-reload
  - systemctl enable --now stroppy-agent
`))

// GenerateCloudInit renders the cloud-init YAML for bootstrapping an agent on a VM.
func GenerateCloudInit(params CloudInitParams) (string, error) {
	if params.AgentPort == 0 {
		params.AgentPort = DefaultAgentPort
	}
	if params.SSHUser == "" {
		params.SSHUser = "stroppy"
	}

	data := struct {
		CloudInitParams
		BinPath string
	}{
		CloudInitParams: params,
		BinPath:         RemoteBinPath,
	}

	var buf bytes.Buffer
	if err := cloudInitTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("agent: render cloud-init: %w", err)
	}
	return buf.String(), nil
}
