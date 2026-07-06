package agent

import (
	"strings"
	"testing"
)

func TestEnvBuildsProviderIndependentAgentContract(t *testing.T) {
	env, err := Env("postgres-master", Bootstrap{
		ServerAddr:        "http://server:8080",
		TemporalNamespace: "bench",
		AgentToken:        "agent-jwt",
		AgentTaskQueue:    "secret-queue-postgres-master",
		ExtraEnv: map[string]string{
			"STROPPY_SERVER_ADDR": "http://wrong",
			"CUSTOM_ENV":          "value",
		},
	})
	if err != nil {
		t.Fatalf("build env: %v", err)
	}

	checks := map[string]string{
		"STROPPY_SERVER_ADDR":      "http://server:8080",
		"STROPPY_AGENT_BINARY_URL": "http://server:8080/agent/binary",
		"STROPPY_MACHINE_ID":       "postgres-master",
		"STROPPY_NODE_ID":          "postgres-master",
		"AGENT_MACHINE_ID":         "postgres-master",
		"AGENT_TASK_QUEUE":         "secret-queue-postgres-master",
		"TEMPORAL_NAMESPACE":       "bench",
		"STROPPY_AGENT_TOKEN":      "agent-jwt",
		"CUSTOM_ENV":               "value",
	}
	for key, want := range checks {
		if got := env[key]; got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestEnvDoesNotLeakDirectBackendAddresses(t *testing.T) {
	env, err := Env("node-1", Bootstrap{
		ServerAddr: "https://control.stage",
		ExtraEnv: map[string]string{
			"APT_BACKEND":              "http://apt:3142",
			"GRAFANA_BACKEND":          "http://grafana:3000",
			"MONITORING_TOKEN":         "raw-token",
			"MONITORING_URL":           "http://vmauth:8427",
			"STROPPY_MONITORING_TOKEN": "agent-token",
			"TEMPORAL_HOSTPORT":        "temporal:7233",
			"TEMPORAL_URL":             "http://temporal:7233",
			"VICTORIA_LOGS_URL":        "http://victoria-logs:9428",
			"VICTORIA_METRICS_URL":     "http://victoria-metrics:8428",
			"VICTORIA_URL":             "http://victoria:8428",
			"WORKLOAD_CUSTOM_ENV":      "kept",
			"STROPPY_AGENT_BINARY_URL": "http://wrong/agent",
			"STROPPY_SERVER_ADDR":      "http://wrong",
			"TEMPORAL_NAMESPACE":       "wrong",
			"HTTP_PROXY":               "http://wrong-proxy",
			"HTTPS_PROXY":              "http://wrong-proxy",
			"NO_PROXY":                 "wrong",
			"http_proxy":               "http://wrong-proxy",
			"https_proxy":              "http://wrong-proxy",
			"no_proxy":                 "wrong",
		},
	})
	if err != nil {
		t.Fatalf("build env: %v", err)
	}

	for _, key := range []string{
		"APT_BACKEND",
		"GRAFANA_BACKEND",
		"MONITORING_TOKEN",
		"MONITORING_URL",
		"STROPPY_MONITORING_TOKEN",
		"TEMPORAL_HOSTPORT",
		"TEMPORAL_URL",
		"VICTORIA_LOGS_URL",
		"VICTORIA_METRICS_URL",
		"VICTORIA_URL",
		"HTTP_PROXY",
		"HTTPS_PROXY",
		"NO_PROXY",
		"http_proxy",
		"https_proxy",
		"no_proxy",
	} {
		if _, ok := env[key]; ok {
			t.Fatalf("%s leaked into agent env: %#v", key, env)
		}
	}

	checks := map[string]string{
		"STROPPY_SERVER_ADDR":      "https://control.stage",
		"STROPPY_AGENT_BINARY_URL": "https://control.stage/agent/binary",
		"TEMPORAL_NAMESPACE":       DefaultTemporalNamespace,
		"WORKLOAD_CUSTOM_ENV":      "kept",
	}
	for key, want := range checks {
		if got := env[key]; got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestCloudInitUsesSameEnvContract(t *testing.T) {
	cloudInit, err := CloudInit("node-1", Bootstrap{ServerAddr: "http://server:8080"}, CloudInitOptions{
		SSHUser:      "stroppy",
		SSHPublicKey: "ssh-rsa test",
	})
	if err != nil {
		t.Fatalf("cloud-init: %v", err)
	}

	for _, want := range []string{
		"#cloud-config",
		"STROPPY_SERVER_ADDR=http://server:8080",
		"STROPPY_AGENT_BINARY_URL=http://server:8080/agent/binary",
		"Acquire::http::Proxy \"http://server:8080\";",
		"Acquire::https::Proxy \"DIRECT\";",
		"STROPPY_MACHINE_ID=node-1",
		"AGENT_TASK_QUEUE=stroppy-agent-node-1",
		"EnvironmentFile=/etc/stroppy/agent.env",
		"ExecStartPre=/bin/sh -ec",
		"--retry 30 --retry-delay 5 --retry-connrefused",
		"ExecStart=/usr/local/bin/stroppy-agent agent",
		"systemctl enable --now stroppy-agent",
	} {
		if !strings.Contains(cloudInit, want) {
			t.Fatalf("cloud-init missing %q:\n%s", want, cloudInit)
		}
	}
	for _, unwanted := range []string{"HTTP_PROXY=", "HTTPS_PROXY=", "http_proxy=", "https_proxy=", "NO_PROXY=", "no_proxy="} {
		if strings.Contains(cloudInit, unwanted) {
			t.Fatalf("cloud-init leaked proxy env %q:\n%s", unwanted, cloudInit)
		}
	}
}

// cloudInitNonePathGolden is a byte-for-byte capture of CloudInit's output
// for a plain Bootstrap (no NomadRole) taken before Nomad support was added.
// It exists to guard the invariant that NomadRoleNone (the default, and the
// only path every current caller exercises) renders identical cloud-init
// after the Nomad template blocks are introduced.
const cloudInitNonePathGolden = `#cloud-config
users:
  - name: stroppy
    groups: sudo
    shell: /bin/bash
    sudo: ALL=(ALL) NOPASSWD:ALL
    ssh_authorized_keys:
      - ssh-rsa test

write_files:
  - path: /etc/stroppy/agent.env
    content: |
      AGENT_MACHINE_ID=node-1
      AGENT_TASK_QUEUE=stroppy-agent-node-1
      STROPPY_AGENT_BINARY_URL=http://server:8080/agent/binary
      STROPPY_MACHINE_ID=node-1
      STROPPY_NODE_ID=node-1
      STROPPY_SERVER_ADDR=http://server:8080
      TEMPORAL_NAMESPACE=default
  - path: /etc/apt/apt.conf.d/90stroppy-proxy
    content: |
      Acquire::http::Proxy "http://server:8080";
      Acquire::https::Proxy "DIRECT";
      Acquire::Retries "8";
      Acquire::http::Timeout "120";
      Acquire::https::Timeout "120";
      Acquire::Queue-Mode "access";
      DPkg::Lock::Timeout "600";
  - path: /etc/systemd/system/stroppy-agent.service
    content: |
      [Unit]
      Description=Stroppy Agent
      After=network-online.target
      Wants=network-online.target

      [Service]
      Type=simple
      EnvironmentFile=/etc/stroppy/agent.env
      ExecStartPre=/bin/sh -ec 'mkdir -p /usr/local/bin && curl -fsSL --retry 30 --retry-delay 5 --retry-connrefused -o /usr/local/bin/stroppy-agent.tmp "$STROPPY_AGENT_BINARY_URL" && chmod +x /usr/local/bin/stroppy-agent.tmp && mv /usr/local/bin/stroppy-agent.tmp /usr/local/bin/stroppy-agent'
      ExecStart=/usr/local/bin/stroppy-agent agent
      Restart=always
      RestartSec=2
      StartLimitIntervalSec=0

      [Install]
      WantedBy=multi-user.target

runcmd:
  - mkdir -p /etc/stroppy
  - mkdir -p /usr/local/bin
  - curl -fsSL --retry 30 --retry-delay 5 --retry-connrefused -o /usr/local/bin/stroppy-agent.tmp "http://server:8080/agent/binary" && chmod +x /usr/local/bin/stroppy-agent.tmp && mv /usr/local/bin/stroppy-agent.tmp /usr/local/bin/stroppy-agent
  - systemctl daemon-reload
  - systemctl enable --now stroppy-agent
`

func TestCloudInitNomadNoneIsByteIdenticalToPreNomadTemplate(t *testing.T) {
	cloudInit, err := CloudInit("node-1", Bootstrap{ServerAddr: "http://server:8080"}, CloudInitOptions{
		SSHUser:      "stroppy",
		SSHPublicKey: "ssh-rsa test",
	})
	if err != nil {
		t.Fatalf("cloud-init: %v", err)
	}
	if cloudInit != cloudInitNonePathGolden {
		t.Fatalf("cloud-init changed for NomadRoleNone (default path):\ngot:\n%s\nwant:\n%s", cloudInit, cloudInitNonePathGolden)
	}
}

func TestCloudInitNomadClientRendersClientHCLAndInstall(t *testing.T) {
	cloudInit, err := CloudInit("node-1", Bootstrap{
		ServerAddr:      "http://server:8080",
		NomadRole:       NomadRoleClient,
		NomadServerAddr: "10.0.0.1:4647",
	}, CloudInitOptions{SSHUser: "stroppy"})
	if err != nil {
		t.Fatalf("cloud-init: %v", err)
	}

	for _, want := range []string{
		"/etc/nomad.d/client.hcl",
		`datacenter = "dc1"`,
		"client {",
		`servers = ["10.0.0.1:4647"]`,
		"meta {",
		`stroppy_node_id = "node-1"`,
		"nomad.service",
		"ExecStart=/usr/local/bin/nomad agent -config=/etc/nomad.d",
		"https://releases.hashicorp.com/nomad/" + DefaultNomadVersion + "/nomad_" + DefaultNomadVersion + "_linux_amd64.zip",
		"apt-get install -y unzip",
		"systemctl enable --now nomad",
	} {
		if !strings.Contains(cloudInit, want) {
			t.Fatalf("cloud-init missing %q:\n%s", want, cloudInit)
		}
	}
	if strings.Contains(cloudInit, "server.hcl") {
		t.Fatalf("client cloud-init unexpectedly contains server.hcl:\n%s", cloudInit)
	}
	if strings.Contains(cloudInit, "bootstrap_expect") {
		t.Fatalf("client cloud-init unexpectedly contains bootstrap_expect:\n%s", cloudInit)
	}
}

func TestCloudInitNomadServerRendersServerHCLAndInstall(t *testing.T) {
	cloudInit, err := CloudInit("node-1", Bootstrap{
		ServerAddr: "http://server:8080",
		NomadRole:  NomadRoleServer,
	}, CloudInitOptions{SSHUser: "stroppy"})
	if err != nil {
		t.Fatalf("cloud-init: %v", err)
	}

	for _, want := range []string{
		"/etc/nomad.d/server.hcl",
		`datacenter = "dc1"`,
		"server {",
		"bootstrap_expect = 1",
		"client {",
		"enabled = true",
		"nomad.service",
		"apt-get install -y unzip",
		"systemctl enable --now nomad",
	} {
		if !strings.Contains(cloudInit, want) {
			t.Fatalf("cloud-init missing %q:\n%s", want, cloudInit)
		}
	}
	if strings.Contains(cloudInit, "client.hcl") {
		t.Fatalf("server cloud-init unexpectedly contains client.hcl:\n%s", cloudInit)
	}
}

// TestNomadServerHCLMatchesCloudInitServerRendering guards the invariant
// docker.go's nomadGatewayContainer relies on: the exported NomadServerHCL
// wrapper (used to deliver a config file into the docker Nomad gateway
// sidecar) must render byte-identical content to what CloudInit's
// NomadRoleServer path embeds into /etc/nomad.d/server.hcl for cloud-init
// providers, so the docker and cloud-init paths never drift on what a
// "gateway" Nomad config looks like.
func TestNomadServerHCLMatchesCloudInitServerRendering(t *testing.T) {
	got := NomadServerHCL()
	want := renderNomadHCL(NomadRoleServer, "", "")
	if got != want {
		t.Fatalf("NomadServerHCL diverged from renderNomadHCL(NomadRoleServer, \"\", \"\"):\ngot:\n%s\nwant:\n%s", got, want)
	}

	for _, wantSubstr := range []string{
		`datacenter = "dc1"`,
		"server {",
		"bootstrap_expect = 1",
		"client {",
		`plugin "docker"`,
		"allow_privileged = true",
	} {
		if !strings.Contains(got, wantSubstr) {
			t.Fatalf("NomadServerHCL missing %q:\n%s", wantSubstr, got)
		}
	}
	// NomadRoleServer stamps no meta.stroppy_node_id (only NomadRoleClient
	// does) — this is the documented single-node docker gap (see
	// nomadGatewayContainer's doc in internal/infrastructure/provider/docker.go):
	// nomad.BuildJob's per-node constraint can never match this node.
	if strings.Contains(got, "stroppy_node_id") {
		t.Fatalf("NomadServerHCL unexpectedly stamps stroppy_node_id meta:\n%s", got)
	}
}

// TestNomadConfigConstantsMatchUnexportedPaths guards NomadConfigDir/
// NomadServerHCLPath (the exported aliases internal/infrastructure/
// provider/docker.go uses) against drifting from the unexported cloud-init
// paths they mirror.
func TestNomadConfigConstantsMatchUnexportedPaths(t *testing.T) {
	if NomadConfigDir != nomadConfigDir {
		t.Fatalf("NomadConfigDir = %q, want %q", NomadConfigDir, nomadConfigDir)
	}
	if NomadServerHCLPath != nomadServerHCLPath {
		t.Fatalf("NomadServerHCLPath = %q, want %q", NomadServerHCLPath, nomadServerHCLPath)
	}
}

func TestAptProxyURLAddsExplicitDefaultProxyPort(t *testing.T) {
	proxyURL := AptProxyURL("http://caddy")
	if got, want := proxyURL, "http://caddy:80"; got != want {
		t.Fatalf("proxy URL = %q, want %q", got, want)
	}
}

func TestAptProxyURLUsesHTTPSOriginForPublicServer(t *testing.T) {
	proxyURL := AptProxyURL("https://stage.cloud.stroppy.io")
	if got, want := proxyURL, "https://stage.cloud.stroppy.io"; got != want {
		t.Fatalf("proxy URL = %q, want %q", got, want)
	}
}
