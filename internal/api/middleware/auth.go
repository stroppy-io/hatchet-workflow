// Package middleware holds gRPC server interceptors shared by the api services:
// authentication (resolving the request principal) and RBAC guards. Transport
// adaptation (gRPC/connect) is wired at the application level.
package middleware

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/api/caller"
)

// Authenticator resolves the request principal from incoming gRPC metadata —
// account JWT, agent machine JWT, or API token (A, features/tenancy/auth.feature).
// It returns (nil, nil) for an anonymous request (no credentials presented), a
// populated Caller on success, or an error when presented credentials are
// invalid. Implemented by internal/services later.
type Authenticator interface {
	Authenticate(ctx context.Context) (*caller.Caller, error)
}

// NewAuthInterceptor builds a unary interceptor that authenticates every request
// and stores the resolved Caller in context. It does not itself reject anonymous
// requests — per-service guards (e.g. RequireAdmin) decide who may proceed, so
// public RPCs keep working. Invalid credentials map to codes.Unauthenticated.
func NewAuthInterceptor(auth Authenticator) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		c, err := auth.Authenticate(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		if c != nil {
			ctx = caller.NewContext(ctx, c)
		}
		return handler(ctx, req)
	}
}
