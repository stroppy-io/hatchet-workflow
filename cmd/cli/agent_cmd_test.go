package main

import (
	"context"
	"testing"
)

func TestStaticHeadersProviderReturnsBearerHeaderCopy(t *testing.T) {
	provider := staticHeadersProvider{"authorization": "Bearer agent-token"}
	headers, err := provider.GetHeaders(context.Background())
	if err != nil {
		t.Fatalf("headers: %v", err)
	}
	if got, want := headers["authorization"], "Bearer agent-token"; got != want {
		t.Fatalf("authorization = %q, want %q", got, want)
	}

	headers["authorization"] = "Bearer changed"
	headers, err = provider.GetHeaders(context.Background())
	if err != nil {
		t.Fatalf("headers: %v", err)
	}
	if got, want := headers["authorization"], "Bearer agent-token"; got != want {
		t.Fatalf("authorization after mutation = %q, want %q", got, want)
	}
}

func TestAgentGRPCTargetUsesTLSPortForHTTPSServerAddr(t *testing.T) {
	target, tlsConfig := agentGRPCTarget("https://cloud.stroppy.io")
	if got, want := target, "cloud.stroppy.io:443"; got != want {
		t.Fatalf("target = %q, want %q", got, want)
	}
	if tlsConfig == nil {
		t.Fatal("tls config is nil")
	}
	if got, want := tlsConfig.ServerName, "cloud.stroppy.io"; got != want {
		t.Fatalf("server name = %q, want %q", got, want)
	}
}

func TestAgentGRPCTargetKeepsExplicitHTTPPortInsecure(t *testing.T) {
	target, tlsConfig := agentGRPCTarget("http://server:8080")
	if got, want := target, "server:8080"; got != want {
		t.Fatalf("target = %q, want %q", got, want)
	}
	if tlsConfig != nil {
		t.Fatal("tls config is not nil")
	}
}

func TestAgentGRPCTargetPreservesBareTarget(t *testing.T) {
	target, tlsConfig := agentGRPCTarget("temporal:7233")
	if got, want := target, "temporal:7233"; got != want {
		t.Fatalf("target = %q, want %q", got, want)
	}
	if tlsConfig != nil {
		t.Fatal("tls config is not nil")
	}
}
