package api

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/api/v1/apiv1connect"
)

type contextKey string

const usernameKey contextKey = "username"

func UsernameFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(usernameKey).(string); ok {
		return v
	}
	return ""
}

// authInterceptor implements connect.Interceptor for both unary and streaming RPCs.
type authInterceptor struct {
	jwtMgr             *JWTManager
	publicProcedures   map[string]bool
}

func NewAuthInterceptor(jwtMgr *JWTManager) connect.Interceptor {
	return &authInterceptor{
		jwtMgr: jwtMgr,
		publicProcedures: map[string]bool{
			apiv1connect.AuthAPILoginProcedure:        true,
			apiv1connect.AuthAPIRefreshTokenProcedure: true,
		},
	}
}

func (i *authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if i.publicProcedures[req.Spec().Procedure] {
			return next(ctx, req)
		}
		ctx, err := i.authenticate(ctx, req.Header().Get("Authorization"))
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i *authInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *authInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := i.authenticate(ctx, conn.RequestHeader().Get("Authorization"))
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

func (i *authInterceptor) authenticate(ctx context.Context, authHeader string) (context.Context, error) {
	if authHeader == "" {
		return ctx, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		return ctx, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	claims, err := i.jwtMgr.ValidateAccessToken(token)
	if err != nil {
		return ctx, connect.NewError(connect.CodeUnauthenticated, err)
	}
	return context.WithValue(ctx, usernameKey, claims.Username), nil
}
