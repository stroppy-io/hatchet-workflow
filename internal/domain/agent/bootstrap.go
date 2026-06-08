package agent

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"text/template"
)

const (
	RemoteBinPath             = "/usr/local/bin/stroppy-agent"
	DefaultTemporalNamespace  = "default"
	DockerEnvFilePath         = "/etc/stroppy-agent.env"
	CloudInitEnvFilePath      = "/etc/stroppy/agent.env"
	DefaultAgentBinaryPathURL = "/agent/binary"
)

var blockedExtraEnv = map[string]struct{}{
	"APT_BACKEND":              {},
	"GRAFANA_BACKEND":          {},
	"MONITORING_TOKEN":         {},
	"MONITORING_URL":           {},
	EnvAgentToken:              {},
	"STROPPY_MONITORING_TOKEN": {},
	"TEMPORAL_ADDR":            {},
	"TEMPORAL_ADDRESS":         {},
	"TEMPORAL_HOSTPORT":        {},
	"TEMPORAL_URL":             {},
	"VICTORIA_LOGS_URL":        {},
	"VICTORIA_METRICS_URL":     {},
	"VICTORIA_URL":             {},
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
	proxyURL, noProxyHosts := AgentProxyEnv(bootstrap.ServerAddr)
	if proxyURL != "" {
		env["HTTP_PROXY"] = proxyURL
		env["HTTPS_PROXY"] = proxyURL
		env["http_proxy"] = proxyURL
		env["https_proxy"] = proxyURL
		env["NO_PROXY"] = mergeNoProxy(env["NO_PROXY"], noProxyHosts)
		env["no_proxy"] = env["NO_PROXY"]
	}
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

	data := struct {
		SSHUser      string
		SSHPublicKey string
		EnvFile      string
		BinaryURL    string
		BinPath      string
		ProxyURL     string
	}{
		SSHUser:      sshUser,
		SSHPublicKey: options.SSHPublicKey,
		EnvFile:      indent(EnvFileFromMap(env), 6),
		BinaryURL:    env["STROPPY_AGENT_BINARY_URL"],
		BinPath:      RemoteBinPath,
		ProxyURL:     env["HTTP_PROXY"],
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
{{- if .ProxyURL}}
  - path: /etc/apt/apt.conf.d/90stroppy-proxy
    content: |
      Acquire::http::Proxy "{{.ProxyURL}}";
      Acquire::https::Proxy "{{.ProxyURL}}";
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

// AgentProxyEnv returns the HTTP proxy URL agents should use for outbound
// package/artifact traffic plus hosts that must bypass that proxy to avoid
// sending control-plane calls back through the apt forward-proxy path.
func AgentProxyEnv(serverAddr string) (string, []string) {
	u, err := url.Parse(serverAddr)
	if err != nil || u.Host == "" {
		return serverAddr, defaultNoProxyHosts("")
	}
	host := u.Hostname()
	proxyHost := u.Host
	if u.Scheme == "https" {
		proxyHost = host
		if port := u.Port(); port != "" && port != "443" {
			proxyHost = net.JoinHostPort(host, port)
		}
	}
	if u.Port() == "" && host != "" {
		proxyHost = net.JoinHostPort(host, "80")
	}
	if proxyHost == "" {
		return "", nil
	}
	return "http://" + proxyHost, defaultNoProxyHosts(host, u.Host)
}

func defaultNoProxyHosts(hosts ...string) []string {
	out := []string{"127.0.0.1", "localhost", "::1", "host.docker.internal", "169.254.169.254"}
	for _, host := range hosts {
		if host != "" {
			out = append(out, host)
		}
	}
	return out
}

func mergeNoProxy(existing string, hosts []string) string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(hosts)+4)
	add := func(values string) {
		for _, value := range strings.Split(values, ",") {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}
	add(existing)
	add(strings.Join(hosts, ","))
	return strings.Join(out, ",")
}
