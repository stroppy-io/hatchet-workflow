package middleware

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

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
			// Platform admins have cross-tenant access — skip the per-tenant
			// membership check. UI relies on this for root users who switch
			// between tenants via the TenantSwitcher without explicit membership.
			if PlatformRoleFromCtx(ctx) == iampb.PlatformRole_PLATFORM_ROLE_ADMIN.String() {
				ctx = WithTenantID(ctx, tenantID)
				return next(ctx, req)
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

// tenantIDFromRequest reads the tenant identifier from a request. Order:
//  1. X-Tenant-Id header (canonical — set by the SPA on every call).
//  2. proto field `tenant_id` if it carries a *TenantId message — walked via
//     protoreflect so any service that embeds a tenant id is detected without
//     hard-coding field offsets per request type.
//  3. proto fields of TenantId type with name suffix "_id" / containing
//     "tenant" — same protoreflect walk handles AddMemberRequest, settings
//     requests, and any future shape that follows the convention.
func tenantIDFromRequest(req connect.AnyRequest) string {
	if v := req.Header().Get("X-Tenant-Id"); v != "" {
		return v
	}
	msg, ok := req.Any().(proto.Message)
	if !ok {
		return ""
	}
	return walkProtoForTenantID(msg.ProtoReflect())
}

// walkProtoForTenantID iterates populated message-typed fields looking for
// the embedded TenantId wrapper.
func walkProtoForTenantID(m protoreflect.Message) string {
	var found string
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.Kind() != protoreflect.MessageKind {
			return true
		}
		if fd.IsList() || fd.IsMap() {
			return true
		}
		sub := v.Message()
		// Match by proto message full name to handle every TenantId wrapper.
		if sub.Descriptor().FullName() == "cloud.v1.iam.TenantId" {
			vf := sub.Descriptor().Fields().ByName("value")
			if vf != nil {
				found = sub.Get(vf).String()
				return false
			}
		}
		return true
	})
	return found
}
