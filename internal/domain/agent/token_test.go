package agent

import "testing"

func TestTokenServiceIssuesAndVerifiesAgentClaims(t *testing.T) {
	svc, err := NewTokenService("secret")
	if err != nil {
		t.Fatalf("token service: %v", err)
	}
	queue := TaskQueueWithNonce("node-1", "nonce")
	token, err := svc.IssueAgentToken("tenant-1", "run-1", "node-1", queue)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	claims, err := svc.VerifyAgentToken(token)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if claims.TenantID != "tenant-1" || claims.RunID != "run-1" || claims.MachineID != "node-1" || claims.TaskQueue != queue {
		t.Fatalf("claims = %#v, want tenant/run/node/queue", claims)
	}
}

func TestTokenServiceRejectsWrongSecret(t *testing.T) {
	issuer, err := NewTokenService("secret-1")
	if err != nil {
		t.Fatalf("issuer: %v", err)
	}
	verifier, err := NewTokenService("secret-2")
	if err != nil {
		t.Fatalf("verifier: %v", err)
	}
	token, err := issuer.IssueAgentToken("tenant-1", "run-1", "node-1", TaskQueueWithNonce("node-1", "nonce"))
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	if _, err := verifier.VerifyAgentToken(token); err == nil {
		t.Fatal("token signed with a different secret was accepted")
	}
}

func TestEnvRejectsAgentTokenWithoutTaskQueue(t *testing.T) {
	if _, err := Env("node-1", Bootstrap{ServerAddr: "http://server", AgentToken: "jwt"}); err == nil {
		t.Fatal("agent token without task queue was accepted")
	}
}
