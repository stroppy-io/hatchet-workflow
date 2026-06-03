package gateway

import (
	agentdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"google.golang.org/grpc/metadata"
)

type AgentTokenVerifier interface {
	VerifyAgentToken(token string) (*agentdomain.TokenClaims, error)
}

func agentClaimsFromBearer(header string, verifier AgentTokenVerifier) (*agentdomain.TokenClaims, bool) {
	if verifier == nil {
		return nil, false
	}
	token := agentdomain.BearerToken(header)
	if token == "" {
		return nil, false
	}
	claims, err := verifier.VerifyAgentToken(token)
	if err != nil {
		return nil, false
	}
	return claims, true
}

func validAgentBearer(header string, verifier AgentTokenVerifier) bool {
	_, ok := agentClaimsFromBearer(header, verifier)
	return ok
}

func validAgentGRPCBearer(md metadata.MD, verifier AgentTokenVerifier) bool {
	if verifier == nil {
		return false
	}
	for _, header := range md.Get("authorization") {
		if validAgentBearer(header, verifier) {
			return true
		}
	}
	return false
}
