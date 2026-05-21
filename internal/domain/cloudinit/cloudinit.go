// Package cloudinit renders the per-VM cloud-init user-data that bootstraps a
// stroppy agent (D18): it installs the agent binary, writes an env file with the
// per-machine agent JWT + server address, and starts the agent as a systemd
// service. Recast of internal/old/domain/agent/cloudinit.go.
package cloudinit

import (
	"bytes"
	"fmt"
	"text/template"
)

const (
	// RemoteBinPath is where the agent binary is installed on the VM.
	RemoteBinPath = "/usr/local/bin/stroppy-agent"
	// DefaultAgentPort is the agent's listen port.
	DefaultAgentPort = 9090
)

// Params drives cloud-init rendering for one VM.
type Params struct {
	// BinaryURL is where the agent binary is downloaded from (server endpoint / presigned S3).
	BinaryURL string
	// ServerAddr is the control-plane address the agent polls.
	ServerAddr string
	// AgentPort is the agent listen port (0 -> DefaultAgentPort).
	AgentPort int
	// MachineID is the topology machine id (STROPPY_MACHINE_ID).
	MachineID string
	// AgentToken is the per-machine agent JWT (machine principal, D18).
	AgentToken string
	// SSHUser is the login user ("" -> "stroppy").
	SSHUser string
	// SSHPublicKey is added to the user's authorized_keys when set.
	SSHPublicKey string
	// ExtraEnv adds further agent env vars.
	ExtraEnv map[string]string
}

var tmpl = template.Must(template.New("cloudinit").Parse(`#cloud-config
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
{{- if .AgentToken}}
      STROPPY_AGENT_TOKEN={{.AgentToken}}
{{- end}}
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
      Restart=always
      RestartSec=2
      StartLimitIntervalSec=0

      [Install]
      WantedBy=multi-user.target

runcmd:
  - mkdir -p /etc/stroppy
  - curl -fsSL -o {{.BinPath}} "{{.BinaryURL}}"
  - chmod +x {{.BinPath}}
  - systemctl daemon-reload
  - systemctl enable --now stroppy-agent
`))

// Render produces the cloud-init YAML for one VM.
func Render(p Params) (string, error) {
	if p.AgentPort == 0 {
		p.AgentPort = DefaultAgentPort
	}
	if p.SSHUser == "" {
		p.SSHUser = "stroppy"
	}
	data := struct {
		Params
		BinPath string
	}{Params: p, BinPath: RemoteBinPath}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("cloudinit: render: %w", err)
	}
	return buf.String(), nil
}
