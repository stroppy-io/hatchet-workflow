package gateway

import (
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestValidAgentGRPCBearer(t *testing.T) {
	token, verifier := testAgentToken(t)
	md := metadata.Pairs("authorization", "Bearer "+token)
	if !validAgentGRPCBearer(md, verifier) {
		t.Fatal("valid bearer was rejected")
	}
}

func TestValidAgentGRPCBearerRejectsMissingOrInvalidToken(t *testing.T) {
	_, verifier := testAgentToken(t)
	for name, md := range map[string]metadata.MD{
		"missing": metadata.Pairs(),
		"wrong":   metadata.Pairs("authorization", "Bearer wrong"),
		"basic":   metadata.Pairs("authorization", "Basic wrong"),
	} {
		if validAgentGRPCBearer(md, verifier) {
			t.Fatalf("%s bearer was accepted", name)
		}
	}
}

func TestValidAgentGRPCBearerRejectsMissingVerifier(t *testing.T) {
	token, _ := testAgentToken(t)
	if validAgentGRPCBearer(metadata.Pairs("authorization", "Bearer "+token), nil) {
		t.Fatal("missing verifier should reject token")
	}
}
