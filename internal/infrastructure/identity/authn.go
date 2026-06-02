package identity

import (
	"context"
	"strings"

	"google.golang.org/grpc/metadata"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	iamsvc "github.com/stroppy-io/stroppy-cloud/internal/services/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// JWTAuthn implements utils.Authn: it extracts the bearer token from the
// incoming request metadata in ctx, verifies it, and returns the caller's
// AccessClaims. It first honors claims an upstream interceptor may have stashed
// (iamsvc.ContextWithClaims) so a single verification per request is reused.
type JWTAuthn struct {
	verifier iamsvc.TokenVerifier
}

var _ utils.Authn = (*JWTAuthn)(nil)

// NewJWTAuthn builds the authn adapter from a configured signing secret by
// constructing a verifier over it. Refresh-session features are unused here, so
// a no-op session store backs the verifier (Verify never touches it).
func NewJWTAuthn(cfg Config) (*JWTAuthn, error) {
	svc, err := NewJWTTokenService(cfg, noopSessionStore{})
	if err != nil {
		return nil, err
	}
	return &JWTAuthn{verifier: svc}, nil
}

// NewJWTAuthnWithVerifier builds the authn adapter over an existing verifier
// (e.g. the same JWTTokenService used to mint tokens).
func NewJWTAuthnWithVerifier(verifier iamsvc.TokenVerifier) *JWTAuthn {
	return &JWTAuthn{verifier: verifier}
}

// Caller returns the authenticated principal for the request, or a typed
// Unauthenticated error when no valid bearer token is present.
func (a *JWTAuthn) Caller(ctx context.Context) (*iam.AccessClaims, error) {
	if c, ok := iamsvc.ClaimsFromContext(ctx); ok {
		return c, nil
	}
	token, err := bearerToken(ctx)
	if err != nil {
		return nil, err
	}
	claims, err := a.verifier.Verify(ctx, token)
	if err != nil {
		return nil, derrors.Unauthenticated("invalid access token")
	}
	return claims, nil
}

// bearerToken reads the "authorization: Bearer <token>" header from the incoming
// gRPC/connect metadata in ctx.
func bearerToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", derrors.Unauthenticated("missing authorization metadata")
	}
	for _, v := range md.Get("authorization") {
		if len(v) > 7 && strings.EqualFold(v[:7], "bearer ") {
			if t := strings.TrimSpace(v[7:]); t != "" {
				return t, nil
			}
		}
	}
	return "", derrors.Unauthenticated("missing bearer token")
}

// noopSessionStore satisfies RefreshSessionStore for the verify-only path. Its
// mutating methods are never invoked by Verify; they fail safe if called.
type noopSessionStore struct{}

func (noopSessionStore) Create(context.Context, RefreshSession) error { return nil }

func (noopSessionStore) Get(context.Context, string) (RefreshSession, error) {
	return RefreshSession{}, derrors.NotFound("refresh_session", "refresh session not found")
}

func (noopSessionStore) Delete(context.Context, string) error { return nil }
