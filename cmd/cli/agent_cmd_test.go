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
