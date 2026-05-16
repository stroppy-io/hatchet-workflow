package middleware

import (
	"context"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// RequirePlatformAdmin is a ConnectRPC unary interceptor that rejects requests
// from non-admin callers with PermissionDenied. It reads the platform_role from
// context (set by the Auth interceptor from the JWT claim).
func RequirePlatformAdmin() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			role := PlatformRoleFromCtx(ctx)
			if role != iampb.PlatformRole_PLATFORM_ROLE_ADMIN.String() {
				return nil, toConnect(domainerr.PermissionDenied())
			}
			return next(ctx, req)
		}
	}
}
