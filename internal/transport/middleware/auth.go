package middleware

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// AuthPort is implemented by iam.Service.
type AuthPort interface {
	VerifyAccessToken(token string) (userID string, jti string, platformRole string, err error)
	VerifyApiToken(ctx context.Context, plain string) (*iampb.ApiToken, error)
}

// Auth returns a unary interceptor that validates the Authorization header.
// Bypass: methods in the bypassSet (full procedure path).
func Auth(svc AuthPort, bypass map[string]bool) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if bypass[req.Spec().Procedure] {
				return next(ctx, req)
			}
			header := req.Header().Get("Authorization")
			if header == "" {
				return nil, toConnect(domainerr.Unauthenticated())
			}
			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 {
				return nil, toConnect(domainerr.Unauthenticated())
			}
			scheme := strings.ToLower(parts[0])
			token := parts[1]
			switch scheme {
			case "bearer":
				userID, _, platformRole, err := svc.VerifyAccessToken(token)
				if err != nil {
					return nil, toConnect(domainerr.Unauthenticated())
				}
				ctx = WithUserID(ctx, userID)
				ctx = WithPlatformRole(ctx, platformRole)
			case "token":
				row, err := svc.VerifyApiToken(ctx, token)
				if err != nil {
					return nil, toConnect(domainerr.Unauthenticated())
				}
				ctx = WithUserID(ctx, row.GetCreatedBy().GetValue())
				ctx = WithTenantID(ctx, row.GetTenantId().GetValue())
			default:
				return nil, toConnect(domainerr.Unauthenticated())
			}
			return next(ctx, req)
		}
	}
}

// MustAuth fails if Auth bypass list missing — used for safety on dev.
var ErrAuthMisconfigured = errors.New("auth middleware: bypass list empty")
