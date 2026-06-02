package iam

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

/*
	auth.go is the single enforcement point for the (cloud.v1.iam.auth) method
	annotation. It reads MethodAuth off each RPC's descriptor and applies it
	BEFORE the handler runs, so no handler can forget its gate (see
	iam/options.proto). It also stashes the verified AccessClaims in the context,
	which is how handlers recover the caller (see ClaimsAuthn).

	Enforcement order mirrors MethodAuth: public -> admin_only -> all_of -> any
	authenticated. An ABSENT annotation is treated as the most restrictive
	default (admin_only) — a method must opt in explicitly or it is admin-fenced.
*/

type claimsCtxKey struct{}

// ContextWithClaims returns a child context carrying the caller's claims.
func ContextWithClaims(ctx context.Context, claims *iam.AccessClaims) context.Context {
	return context.WithValue(ctx, claimsCtxKey{}, claims)
}

// ClaimsFromContext recovers the claims placed by the interceptor, if any.
func ClaimsFromContext(ctx context.Context) (*iam.AccessClaims, bool) {
	c, ok := ctx.Value(claimsCtxKey{}).(*iam.AccessClaims)
	return c, ok
}

// TokenVerifier validates a bearer access token and returns its claims.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (*iam.AccessClaims, error)
}

// PermissionResolver resolves the caller's effective permissions in one tenant
// (the live union; an empty tenantID means platform-scoped only).
type PermissionResolver interface {
	EffectivePermissions(ctx context.Context, accountID, tenantID string) ([]*iam.Permission, error)
}

type AuthInterceptor struct {
	tokens TokenVerifier
	perms  PermissionResolver
}

func NewAuthInterceptor(tokens TokenVerifier, perms PermissionResolver) *AuthInterceptor {
	return &AuthInterceptor{tokens: tokens, perms: perms}
}

// Unary returns the gRPC interceptor enforcing the auth annotation.
func (a *AuthInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		auth := methodAuth(info.FullMethod)

		// Absent annotation -> most restrictive default (admin_only).
		if auth == nil {
			auth = &iam.MethodAuth{AdminOnly: true}
		}
		if auth.GetPublic() {
			return handler(ctx, req)
		}

		claims, err := a.authenticate(ctx)
		if err != nil {
			return nil, err
		}
		ctx = ContextWithClaims(ctx, claims)

		if auth.GetAdminOnly() {
			if !claims.GetIsAdmin() {
				return nil, status.Error(codes.PermissionDenied, "admin only")
			}
			return handler(ctx, req)
		}

		if len(auth.GetAllOf()) > 0 {
			if err := a.authorize(ctx, claims, req, auth); err != nil {
				return nil, err
			}
		}

		// Empty annotation: any authenticated caller is sufficient.
		return handler(ctx, req)
	}
}

func (a *AuthInterceptor) authenticate(ctx context.Context) (*iam.AccessClaims, error) {
	token, err := bearerToken(ctx)
	if err != nil {
		return nil, err
	}
	claims, err := a.tokens.Verify(ctx, token)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid access token")
	}
	return claims, nil
}

func (a *AuthInterceptor) authorize(ctx context.Context, claims *iam.AccessClaims, req any, auth *iam.MethodAuth) error {
	if claims.GetIsAdmin() {
		return nil // platform super-user short-circuits to allow.
	}
	tenantID := tenantIDFromRequest(req, auth.GetTenantField())
	granted, err := a.perms.EffectivePermissions(ctx, claims.GetAccountId(), tenantID)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if !hasAll(granted, auth.GetAllOf()) {
		return status.Error(codes.PermissionDenied, "missing required permission")
	}
	return nil
}

func bearerToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing authorization metadata")
	}
	for _, v := range md.Get("authorization") {
		if len(v) > 7 && strings.EqualFold(v[:7], "bearer ") {
			if t := strings.TrimSpace(v[7:]); t != "" {
				return t, nil
			}
		}
	}
	return "", status.Error(codes.Unauthenticated, "missing bearer token")
}

// methodAuth resolves the (cloud.v1.iam.auth) option for a "/pkg.Service/Method"
// path via the global proto registry. Returns nil when the method or annotation
// is not found.
func methodAuth(fullMethod string) *iam.MethodAuth {
	svc, method, ok := splitFullMethod(fullMethod)
	if !ok {
		return nil
	}
	desc, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(svc))
	if err != nil {
		return nil
	}
	sd, ok := desc.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil
	}
	md := sd.Methods().ByName(protoreflect.Name(method))
	if md == nil {
		return nil
	}
	opts := md.Options()
	if opts == nil {
		return nil
	}
	ext := proto.GetExtension(opts, iam.E_Auth)
	auth, _ := ext.(*iam.MethodAuth)
	return auth
}

func splitFullMethod(fullMethod string) (service, method string, ok bool) {
	// "/cloud.v1.api.IamAPI/CreateAccount"
	s := strings.TrimPrefix(fullMethod, "/")
	i := strings.LastIndex(s, "/")
	if i < 0 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

// tenantIDFromRequest reads the request's tenant id from the named string field
// (MethodAuth.tenant_field), defaulting to "tenant_id". Returns "" when absent.
// The per-request slug -> id gate lives in the HTTP layer; here we only honor an
// explicit id field.
func tenantIDFromRequest(req any, field string) string {
	if field == "" {
		field = "tenant_id"
	}
	m, ok := req.(proto.Message)
	if !ok {
		return ""
	}
	fd := m.ProtoReflect().Descriptor().Fields().ByName(protoreflect.Name(field))
	if fd == nil || fd.Kind() != protoreflect.StringKind {
		return ""
	}
	return m.ProtoReflect().Get(fd).String()
}

// ClaimsAuthn is a ready Authn implementation that recovers the caller from the
// context the interceptor populated. Wire it into the IAM service's Authn dep.
type ClaimsAuthn struct{}

var errNoClaims = errors.New("no caller in context")

func (ClaimsAuthn) Caller(ctx context.Context) (*iam.AccessClaims, error) {
	if c, ok := ClaimsFromContext(ctx); ok {
		return c, nil
	}
	return nil, errNoClaims
}
