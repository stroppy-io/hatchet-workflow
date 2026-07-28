package gateway

import "testing"

func TestRunScopeRoundTrip(t *testing.T) {
	const secret = "shared-signing-secret"
	tok := SignRunScope(secret, "run-123")
	if tok == "" {
		t.Fatal("SignRunScope returned empty")
	}
	got, ok := verifyRunScope(secret, tok)
	if !ok || got != "run-123" {
		t.Fatalf("verifyRunScope = %q,%v; want run-123,true", got, ok)
	}
}

func TestRunScopeRejectsTampering(t *testing.T) {
	const secret = "shared-signing-secret"
	tok := SignRunScope(secret, "run-mine")

	// Swapping the run id without re-signing must fail — otherwise a viewer could
	// point the scope at any run.
	forged := "run.run-other." + tok[len("run.run-mine."):]
	if _, ok := verifyRunScope(secret, forged); ok {
		t.Fatal("forged run id accepted")
	}
	// Wrong secret must fail.
	if _, ok := verifyRunScope("other-secret", tok); ok {
		t.Fatal("token verified under the wrong secret")
	}
	// A public share token (no run. prefix) is not a run-scope token.
	if _, ok := verifyRunScope(secret, "VXYE8BiiX4dRshareToken"); ok {
		t.Fatal("non-run-scope value accepted")
	}
	// Empty secret disables signing/verifying entirely.
	if SignRunScope("", "run-1") != "" {
		t.Fatal("empty secret should not sign")
	}
	if _, ok := verifyRunScope("", tok); ok {
		t.Fatal("empty secret should not verify")
	}
}
