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

	// NomadDatacenter is the only Nomad datacenter stroppy clusters use. It
	// MUST match nomad's defaultDatacenter (internal/dsl/nomad/jobspec.go):
	// BuildJob's job carries Datacenters: []string{"dc1"}, and a Nomad client
	// only ever picks up work from a server whose datacenter it shares.
	NomadDatacenter = "dc1"

	// nomadDataDir is where the Nomad agent stores its local state.
	nomadDataDir = "/opt/nomad/data"

	// nomadConfigDir/nomadBinPath/nomadServiceUnit describe where the
	// installed binary, config, and systemd unit live on the node.
	nomadConfigDir     = "/etc/nomad.d"
	nomadBinPath       = "/usr/local/bin/nomad"
	nomadServiceUnit   = "/etc/systemd/system/nomad.service"
	nomadServerHCLPath = nomadConfigDir + "/server.hcl"
	nomadClientHCLPath = nomadConfigDir + "/client.hcl"

	// DefaultNomadVersion pins the Nomad release installed by cloud-init when
	// no explicit Bootstrap.NomadVersion is set. Bump deliberately (and retest
	// against the target Nomad server version) — it is deploy-gated (see
	// renderNomadInstall doc).
	DefaultNomadVersion = "1.9.3"

	// NomadConfigDir and NomadServerHCLPath re-export the unexported
	// nomadConfigDir/nomadServerHCLPath paths for callers outside this package
	// that deliver the Nomad config through a different mechanism than
	// cloud-init's write_files — e.g. the docker provider's Nomad gateway
	// sidecar (internal/infrastructure/provider/docker.go), which copies
	// NomadServerHCL() into the container at this path (via
	// deployment.Docker_File / Executor.copyFileToContainer, run before
	// ContainerStart) instead of a cloud-init write_files entry, then points
	// the container's command at -config=NomadConfigDir.
	NomadConfigDir     = nomadConfigDir
	NomadServerHCLPath = nomadServerHCLPath
)

// NomadRole selects which Nomad agent cloud-init installs and configures on
// the node, if any.
type NomadRole string

const (
	// NomadRoleNone means cloud-init installs no Nomad agent at all (today's
	// default and only behavior for every existing caller).
	NomadRoleNone NomadRole = ""
	// NomadRoleServer configures a single-node dev-style Nomad server that
	// also runs the client subsystem colocated (server+client enabled,
	// bootstrap_expect=1) — this is the gateway/control node.
	NomadRoleServer NomadRole = "server"
	// NomadRoleClient configures a Nomad client that joins NomadServerAddr
	// and stamps meta.stroppy_node_id so nomad's BuildJob per-node
	// constraint (${meta.stroppy_node_id} == NodeID) can place work on it.
	NomadRoleClient NomadRole = "client"
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

	// NomadRole, when set, makes CloudInit additionally install and
	// configure a Nomad agent on the node (see NomadRole for the
	// server/client distinction). Zero value (NomadRoleNone) installs no
	// Nomad agent — CloudInit's output for that case is unchanged from
	// before Nomad support existed.
	NomadRole NomadRole
	// NomadServerAddr is the Nomad server's advertise address
	// ("<gateway-private-ip>:4647") that a NomadRoleClient node joins. Unused
	// for NomadRoleServer (the server is its own coordinator).
	NomadServerAddr string
	// NomadVersion pins the Nomad release cloud-init installs. Empty selects
	// DefaultNomadVersion.
	NomadVersion string
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

	nomadEnabled := bootstrap.NomadRole != NomadRoleNone
	var nomadHCLPath, nomadHCLContent, nomadVersion, nomadZipURL string
	if nomadEnabled {
		nomadVersion = bootstrap.NomadVersion
		if nomadVersion == "" {
			nomadVersion = DefaultNomadVersion
		}
		nomadZipURL = fmt.Sprintf(
			"https://releases.hashicorp.com/nomad/%s/nomad_%s_linux_amd64.zip",
			nomadVersion, nomadVersion,
		)
		switch bootstrap.NomadRole {
		case NomadRoleNone:
			// unreachable: nomadEnabled is only true when NomadRole != NomadRoleNone.
		case NomadRoleServer:
			nomadHCLPath = nomadServerHCLPath
		case NomadRoleClient:
			nomadHCLPath = nomadClientHCLPath
		}
		nomadHCLContent = indent(renderNomadHCL(bootstrap.NomadRole, bootstrap.NomadServerAddr, machineID), 6)
	}

	data := struct {
		SSHUser          string
		SSHPublicKey     string
		EnvFile          string
		BinaryURL        string
		BinPath          string
		AptProxyConfig   string
		NomadEnabled     bool
		NomadHCLPath     string
		NomadHCLContent  string
		NomadZipURL      string
		NomadBinPath     string
		NomadConfigDir   string
		NomadServiceUnit string
	}{
		SSHUser:          sshUser,
		SSHPublicKey:     options.SSHPublicKey,
		EnvFile:          indent(EnvFileFromMap(env), 6),
		BinaryURL:        env["STROPPY_AGENT_BINARY_URL"],
		BinPath:          RemoteBinPath,
		AptProxyConfig:   aptProxyConfig,
		NomadEnabled:     nomadEnabled,
		NomadHCLPath:     nomadHCLPath,
		NomadHCLContent:  nomadHCLContent,
		NomadZipURL:      nomadZipURL,
		NomadBinPath:     nomadBinPath,
		NomadConfigDir:   nomadConfigDir,
		NomadServiceUnit: nomadServiceUnit,
	}

	var buf bytes.Buffer
	if err := cloudInitTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("agent bootstrap: render cloud-init: %w", err)
	}
	return buf.String(), nil
}

// NomadServerHCL renders the single-node Nomad server+client config
// (bootstrap_expect=1, docker driver with allow_privileged) used by both
// cloud-init's NomadRoleServer path and the docker provider's Nomad gateway
// sidecar (internal/infrastructure/provider/docker.go), so the two callers
// never drift on what a "gateway" Nomad config looks like. It carries no
// serverAddr/machineID (NomadRoleServer ignores both, see renderNomadHCL) and
// therefore, unlike NomadRoleClient, stamps no meta.stroppy_node_id — see
// docker.go's nomadGatewayContainer doc for why that is a known gap for
// nomad's BuildJob per-node constraint on the docker single-node path.
func NomadServerHCL() string {
	return renderNomadHCL(NomadRoleServer, "", "")
}

// renderNomadHCL renders the /etc/nomad.d/{server,client}.hcl content for
// role. serverAddr is the Nomad server's advertise address a client joins
// (ignored for NomadRoleServer); machineID is stamped into a client's
// meta.stroppy_node_id so nomad.BuildJob's per-node placement constraint
// (${meta.stroppy_node_id} == NodeID, see internal/dsl/nomad/jobspec.go) can
// match this node. Both roles share NomadDatacenter ("dc1") — jobspec.go's
// BuildJob only ever targets that one datacenter, and a Nomad client that
// advertised a different one would never be offered stroppy's jobs.
func renderNomadHCL(role NomadRole, serverAddr, machineID string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "datacenter = %q\n", NomadDatacenter)
	fmt.Fprintf(&b, "data_dir   = %q\n\n", nomadDataDir)

	switch role {
	case NomadRoleNone:
		// renderNomadHCL is only called when role != NomadRoleNone; nothing
		// to add for this case.
	case NomadRoleServer:
		b.WriteString("server {\n")
		b.WriteString("  enabled          = true\n")
		b.WriteString("  bootstrap_expect = 1\n")
		b.WriteString("}\n\n")
		b.WriteString("client {\n")
		b.WriteString("  enabled = true\n")
		b.WriteString("}\n\n")
	case NomadRoleClient:
		b.WriteString("client {\n")
		b.WriteString("  enabled = true\n")
		fmt.Fprintf(&b, "  servers = [%q]\n\n", serverAddr)
		b.WriteString("  meta {\n")
		fmt.Fprintf(&b, "    stroppy_node_id = %q\n", machineID)
		b.WriteString("  }\n")
		b.WriteString("}\n\n")
	}

	b.WriteString("plugin \"docker\" {\n")
	b.WriteString("  config {\n")
	b.WriteString("    allow_privileged = true\n")
	b.WriteString("  }\n")
	b.WriteString("}\n")
	return b.String()
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
{{- if .NomadEnabled}}
  - path: {{.NomadHCLPath}}
    content: |
{{.NomadHCLContent}}
  - path: {{.NomadServiceUnit}}
    content: |
      [Unit]
      Description=Nomad
      After=network-online.target
      Wants=network-online.target

      [Service]
      Type=simple
      ExecStart={{.NomadBinPath}} agent -config={{.NomadConfigDir}}
      Restart=always
      RestartSec=2
      StartLimitIntervalSec=0
      LimitNOFILE=65536

      [Install]
      WantedBy=multi-user.target
{{- end}}

runcmd:
  - mkdir -p /etc/stroppy
  - mkdir -p /usr/local/bin
  - curl -fsSL --retry 30 --retry-delay 5 --retry-connrefused -o {{.BinPath}}.tmp "{{.BinaryURL}}" && chmod +x {{.BinPath}}.tmp && mv {{.BinPath}}.tmp {{.BinPath}}
  - systemctl daemon-reload
  - systemctl enable --now stroppy-agent
{{- if .NomadEnabled}}
  - mkdir -p {{.NomadConfigDir}}
  - apt-get update && apt-get install -y unzip
  - curl -fsSL --retry 30 --retry-delay 5 --retry-connrefused -o /tmp/nomad.zip "{{.NomadZipURL}}" && unzip -o /tmp/nomad.zip -d /usr/local/bin && chmod +x {{.NomadBinPath}} && rm -f /tmp/nomad.zip
  - systemctl daemon-reload
  - systemctl enable --now nomad
{{- end}}
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
			"Acquire::Queue-Mode \"access\";\n"+
			// Freshly-booted Ubuntu cloud images run unattended-upgrades, which
			// holds /var/lib/dpkg/lock-frontend; our install then fails fast with
			// "Could not get lock ... exit 100". Wait for the lock instead of
			// aborting so the early-boot upgrade can finish first.
			"DPkg::Lock::Timeout \"600\";\n",
		proxyURL,
	)
}
