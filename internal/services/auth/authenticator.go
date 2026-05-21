package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/api/caller"
	domainauth "github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

const authorizationHeader = "authorization"

var errInvalidCredentials = status.Error(codes.Unauthenticated, "invalid credentials")

// Authenticate resolves the request principal from the Authorization Bearer
// header: a JWT (account or agent) first, then an opaque API token by sha256
// hash. No credentials -> anonymous (nil, nil); guards/services enforce who may
// proceed. Implements middleware.Authenticator.
func (s *AuthService) Authenticate(ctx context.Context) (*caller.Caller, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, nil
	}
	raw := bearerToken(md)
	if raw == "" {
		return nil, nil
	}

	if claims, err := s.signer.Parse(raw); err == nil {
		switch claims.Kind {
		case domainauth.KindAccount:
			return &caller.Caller{
				Kind:      caller.PrincipalAccount,
				AccountID: &models.AccountId{Value: claims.Subject},
				IsAdmin:   claims.IsAdmin,
			}, nil
		case domainauth.KindAgent:
			return &caller.Caller{
				Kind:     caller.PrincipalAgent,
				AgentID:  &models.AgentId{Value: claims.AgentID},
				TenantID: &models.TenantId{Value: claims.TenantID},
			}, nil
		default:
			return nil, errInvalidCredentials
		}
	}

	tok, err := s.getApiTokenByHash(ctx, domainauth.HashToken(raw))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "authenticate: %v", err)
	}
	if tok == nil {
		return nil, errInvalidCredentials
	}
	if exp := tok.GetExpiresAt(); exp != nil && exp.AsTime().Before(time.Now()) {
		return nil, errInvalidCredentials
	}
	return &caller.Caller{
		Kind:     caller.PrincipalApiToken,
		TenantID: tok.GetOwned().GetTenantId(),
		Role:     tok.GetRole(),
	}, nil
}

func bearerToken(md metadata.MD) string {
	vals := md.Get(authorizationHeader)
	if len(vals) == 0 {
		return ""
	}
	const prefix = "Bearer "
	v := vals[0]
	if len(v) >= len(prefix) && strings.EqualFold(v[:len(prefix)], prefix) {
		return v[len(prefix):]
	}
	return v
}

func (s *AuthService) getApiTokenByHash(ctx context.Context, hash string) (*models.ApiToken, error) {
	tok, err := s.apiTokenRepo.QueryRow(ctx,
		models.ApiTokens.SelectAll().Where(
			models.ApiTokens.TokenHash.Eq(hash),
			models.ApiTokens.DeletedAt.IsNull(),
		))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return tok, nil
}
