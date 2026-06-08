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
}
