package agent

import (
	"bytes"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"text/template"
)

const (
	RemoteBinPath             = "/usr/local/bin/stroppy-agent"
	DefaultTemporalNamespace  = "default"
	DockerEnvFilePath         = "/etc/stroppy-agent.env"
	DockerAptProxyFilePath    = "/etc/apt/apt.conf.d/90stroppy-proxy"
	CloudInitEnvFilePath      = "/etc/stroppy/agent.env"
	DefaultAgentBinaryPathURL = "/agent/binary"
)

var blockedExtraEnv = map[string]struct{}{
	"APT_BACKEND":              {},
	"GRAFANA_BACKEND":          {},
	"HTTP_PROXY":               {},
	"HTTPS_PROXY":              {},
	"MONITORING_TOKEN":         {},
	"MONITORING_URL":           {},
	"NO_PROXY":                 {},
	EnvAgentToken:              {},
	"STROPPY_MONITORING_TOKEN": {},
	"TEMPORAL_ADDR":            {},
	"TEMPORAL_ADDRESS":         {},
	"TEMPORAL_HOSTPORT":        {},
	"TEMPORAL_URL":             {},
	"VICTORIA_LOGS_URL":        {},
	"VICTORIA_METRICS_URL":     {},
	"VICTORIA_URL":             {},
	"http_proxy":               {},
	"https_proxy":              {},
	"no_proxy":                 {},
}

type Bootstrap struct {
	ServerAddr        string
	BinaryURL         string
	TemporalNamespace string
	RunID             string
	ExtraEnv          map[string]string
	AgentToken        string
	AgentTaskQueue    string
}

type CloudInitOptions struct {
	SSHUser      string
	SSHPublicKey string
}

func Env(machineID string, bootstrap Bootstrap) (map[string]string, error) {
	if machineID == "" {
		return nil, fmt.Errorf("agent bootstrap: machine id is required")
	}
	if bootstrap.ServerAddr == "" {
		return nil, fmt.Errorf("agent bootstrap: server addr is required")
	}

	namespace := bootstrap.TemporalNamespace
	if namespace == "" {
		namespace = DefaultTemporalNamespace
	}
	binaryURL := bootstrap.BinaryURL
	if binaryURL == "" {
		binaryURL = strings.TrimRight(bootstrap.ServerAddr, "/") + DefaultAgentBinaryPathURL
	}

	env := copyExtraEnv(bootstrap.ExtraEnv)
	env["STROPPY_SERVER_ADDR"] = bootstrap.ServerAddr
	env["STROPPY_AGENT_BINARY_URL"] = binaryURL
	if bootstrap.AgentToken != "" {
		env[EnvAgentToken] = bootstrap.AgentToken
	}
	taskQueue := bootstrap.AgentTaskQueue
	if taskQueue == "" {
		if bootstrap.AgentToken != "" {
			return nil, fmt.Errorf("agent bootstrap: task queue is required when agent token is set")
		}
		taskQueue = TaskQueue(machineID)
	}
	env["STROPPY_MACHINE_ID"] = machineID
	if bootstrap.RunID != "" {
		env["STROPPY_RUN_ID"] = bootstrap.RunID
	}
	env["STROPPY_NODE_ID"] = machineID
	env["AGENT_MACHINE_ID"] = machineID
	env["AGENT_TASK_QUEUE"] = taskQueue
	env["TEMPORAL_NAMESPACE"] = namespace
	return env, nil
}

func EnvFile(machineID string, bootstrap Bootstrap) (string, error) {
	env, err := Env(machineID, bootstrap)
	if err != nil {
		return "", err
	}
	return EnvFileFromMap(env), nil
}

func EnvFileFromMap(env map[string]string) string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&b, "%s=%s\n", key, env[key])
	}
	return b.String()
}

func CloudInit(machineID string, bootstrap Bootstrap, options CloudInitOptions) (string, error) {
	env, err := Env(machineID, bootstrap)
	if err != nil {
		return "", err
	}
	sshUser := options.SSHUser
	if sshUser == "" {
		sshUser = "stroppy"
	}

	aptProxyConfig := AptProxyConfig(bootstrap.ServerAddr)
	if aptProxyConfig != "" {
		aptProxyConfig = indent(aptProxyConfig, 6)
	}

	data := struct {
		SSHUser        string
		SSHPublicKey   string
		EnvFile        string
		BinaryURL      string
		BinPath        string
		AptProxyConfig string
	}{
		SSHUser:        sshUser,
		SSHPublicKey:   options.SSHPublicKey,
		EnvFile:        indent(EnvFileFromMap(env), 6),
		BinaryURL:      env["STROPPY_AGENT_BINARY_URL"],
		BinPath:        RemoteBinPath,
		AptProxyConfig: aptProxyConfig,
	}

	var buf bytes.Buffer
	if err := cloudInitTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("agent bootstrap: render cloud-init: %w", err)
	}
	return buf.String(), nil
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
{{.EnvFile}}
{{- if .AptProxyConfig}}
  - path: /etc/apt/apt.conf.d/90stroppy-proxy
    content: |
{{.AptProxyConfig}}
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
      ExecStartPre=/bin/sh -ec 'mkdir -p /usr/local/bin && curl -fsSL --retry 30 --retry-delay 5 --retry-connrefused -o {{.BinPath}}.tmp "$STROPPY_AGENT_BINARY_URL" && chmod +x {{.BinPath}}.tmp && mv {{.BinPath}}.tmp {{.BinPath}}'
      ExecStart={{.BinPath}} agent
      Restart=always
      RestartSec=2
      StartLimitIntervalSec=0

      [Install]
      WantedBy=multi-user.target

runcmd:
  - mkdir -p /etc/stroppy
  - mkdir -p /usr/local/bin
  - curl -fsSL --retry 30 --retry-delay 5 --retry-connrefused -o {{.BinPath}}.tmp "{{.BinaryURL}}" && chmod +x {{.BinPath}}.tmp && mv {{.BinPath}}.tmp {{.BinPath}}
  - systemctl daemon-reload
  - systemctl enable --now stroppy-agent
`))

func indent(s string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func copyExtraEnv(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		if _, ok := blockedExtraEnv[key]; ok {
			continue
		}
		out[key] = value
	}
	return out
}

// AptProxyURL returns the forward-proxy URL apt should use for package traffic.
// It is intentionally not exported in the agent environment: arbitrary curl/wget
// commands must use the machine's normal network path, while apt alone goes
// through the gateway cache.
func AptProxyURL(serverAddr string) string {
	u, err := url.Parse(serverAddr)
	if err != nil || u.Host == "" {
		return serverAddr
	}
	host := u.Hostname()
	proxyHost := u.Host
	scheme := u.Scheme
	if scheme == "" {
		scheme = "http"
	}
	if scheme == "http" && u.Port() == "" && host != "" {
		proxyHost = host + ":80"
	}
	if proxyHost == "" {
		return ""
	}
	return scheme + "://" + proxyHost
}

func AptProxyConfig(serverAddr string) string {
	proxyURL := AptProxyURL(serverAddr)
	if proxyURL == "" {
		return ""
	}
	// Retries + connection timeouts are essential: several cluster nodes run
	// apt-get update concurrently through the single gateway proxy, which under
	// load can drop or stall a connection. Without a timeout apt waits forever
	// on a silent socket (cluster deploys hung at apt-get update for 15m+); with
	// these, a stalled fetch aborts and is retried instead of blocking deploy.
	return fmt.Sprintf(
		"Acquire::http::Proxy %q;\n"+
			"Acquire::https::Proxy \"DIRECT\";\n"+
			"Acquire::Retries \"8\";\n"+
			"Acquire::http::Timeout \"120\";\n"+
			"Acquire::https::Timeout \"120\";\n"+
			// Serialize downloads to one connection per host. apt opens ~10
			// parallel connections per repo by default; across a multi-node
			// cluster that floods the single gateway apt proxy and it starts
			// dropping connections ("empty reply"). One-at-a-time keeps the
			// proxy load low enough that retries reliably succeed.
			"Acquire::Queue-Mode \"access\";\n",
		proxyURL,
	)
}
