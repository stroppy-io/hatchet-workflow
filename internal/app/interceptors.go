package app

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/api/caller"
	"github.com/stroppy-io/stroppy-cloud/internal/api/middleware"
)

// authStreamInterceptor authenticates a server-stream and wraps the stream so its
// downstream context carries the resolved Caller (the unary interceptor handles the
// rest; streams need their own because grpc keeps the two chains separate).
func authStreamInterceptor(authn middleware.Authenticator) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		c, err := authn.Authenticate(ss.Context())
		if err != nil {
			return status.Error(codes.Unauthenticated, err.Error())
		}
		ctx := ss.Context()
		if c != nil {
			ctx = caller.NewContext(ctx, c)
		}
		return handler(srv, &ctxStream{ServerStream: ss, ctx: ctx})
	}
}

type ctxStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *ctxStream) Context() context.Context { return s.ctx }

// adminMethodPrefix scopes the admin guard to the platform-admin services.
const adminMethodPrefix = "/cloud.v1.api.admin."

// adminGuardInterceptor enforces the platform-admin claim, but only for the admin
// API services — every other service does its own per-tenant RBAC at the service
// layer, so a single global chain stays correct.
func adminGuardInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if strings.HasPrefix(info.FullMethod, adminMethodPrefix) {
			c, ok := caller.FromContext(ctx)
			if !ok || c == nil || !c.IsAdmin {
				return nil, status.Error(codes.PermissionDenied, "platform admin required")
			}
		}
		return handler(ctx, req)
	}
}
