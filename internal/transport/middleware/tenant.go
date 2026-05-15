package middleware

import (
	"context"
	"reflect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// TenantPort is implemented by iam.Service.
type TenantPort interface {
	HasTenantMember(ctx context.Context, userID *iampb.UserId, tenantID *iampb.TenantId) (bool, error)
}

// Tenant resolves tenant_id from request body or X-Tenant-Id header,
// verifies membership, injects into ctx. Bypass list = procedures that don't
// require tenant scope.
func Tenant(svc TenantPort, bypass map[string]bool) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if bypass[req.Spec().Procedure] {
				return next(ctx, req)
			}
			tenantID := tenantIDFromRequest(req)
			if tenantID == "" {
				return nil, toConnect(domainerr.PermissionDenied())
			}
			userID := UserFromCtx(ctx)
			if userID == "" {
				return nil, toConnect(domainerr.Unauthenticated())
			}
			ok, err := svc.HasTenantMember(ctx,
				&iampb.UserId{Value: userID},
				&iampb.TenantId{Value: tenantID},
			)
			if err != nil {
				return nil, toConnect(domainerr.PermissionDenied())
			}
			if !ok {
				return nil, toConnect(domainerr.PermissionDenied())
			}
			ctx = WithTenantID(ctx, tenantID)
			return next(ctx, req)
		}
	}
}

// tenantIDFromRequest reads "tenant_id" via reflection from any proto message
// or falls back to X-Tenant-Id header. Cheap one-time reflection per request.
func tenantIDFromRequest(req connect.AnyRequest) string {
	if v := req.Header().Get("X-Tenant-Id"); v != "" {
		return v
	}
	msg, ok := req.Any().(proto.Message)
	if !ok {
		return ""
	}
	rv := reflect.ValueOf(msg).Elem()
	field := rv.FieldByName("TenantId")
	if !field.IsValid() || field.IsNil() {
		return ""
	}
	// Field is *iampb.TenantId
	valField := field.Elem().FieldByName("Value")
	if !valField.IsValid() {
		return ""
	}
	return valField.String()
}
