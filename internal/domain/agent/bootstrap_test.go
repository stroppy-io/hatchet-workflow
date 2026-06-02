package agent

import (
	"strings"
	"testing"
)

func TestEnvBuildsProviderIndependentAgentContract(t *testing.T) {
	env, err := Env("postgres-master", Bootstrap{
		ServerAddr:        "http://server:8080",
		TemporalNamespace: "bench",
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
		"AGENT_TASK_QUEUE":         "stroppy-agent-postgres-master",
		"TEMPORAL_NAMESPACE":       "bench",
		"CUSTOM_ENV":               "value",
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
		"ExecStart=/usr/local/bin/stroppy-agent agent",
		"systemctl enable --now stroppy-agent",
	} {
		if !strings.Contains(cloudInit, want) {
			t.Fatalf("cloud-init missing %q:\n%s", want, cloudInit)
		}
	}
}
