package middleware

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/api/caller"
)

// NewRequireAdminInterceptor builds a unary interceptor that enforces
// Account.is_admin. Mount it on the platform-level admin services
// (AccountAdminService / TenantAdminService) at registration. It assumes the
// auth interceptor ran first to populate the Caller.
func NewRequireAdminInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		c, ok := caller.FromContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "authentication required")
		}
		if !c.IsAdmin {
			return nil, status.Error(codes.PermissionDenied, "platform admin required")
		}
		return handler(ctx, req)
	}
}
