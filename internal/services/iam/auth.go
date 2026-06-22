package iam

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
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

type RestrictedTokenVerifier interface {
	VerifyWithRestrictions(ctx context.Context, token string) (*iam.AccessClaims, []*iam.Permission, bool, error)
}

// PermissionResolver resolves the caller's effective permissions in one tenant
// (the live union; an empty tenantID means platform-scoped only).
type PermissionResolver interface {
	EffectivePermissions(ctx context.Context, accountID, tenantID string) ([]*iam.Permission, error)
}

type verifiedCaller struct {
	claims      *iam.AccessClaims
	permissions []*iam.Permission
	restricted  bool
}

type AuthInterceptor struct {
	tokens TokenVerifier
	perms  PermissionResolver
}

func NewAuthInterceptor(tokens TokenVerifier, perms PermissionResolver) *AuthInterceptor {
	return &AuthInterceptor{tokens: tokens, perms: perms}
}

// Connect returns the Connect handler interceptor enforcing the auth annotation
// for both unary and streaming RPCs. Server-stream RPCs carry one request
// message; authorization is applied when that first message is received.
func (a *AuthInterceptor) Connect() connect.Interceptor {
	return connectAuthInterceptor{auth: a}
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

		verified, err := a.authenticate(ctx)
		if err != nil {
			return nil, err
		}
		ctx = contextWithVerifiedCaller(ctx, verified)

		if auth.GetAdminOnly() {
			if verified.restricted || !verified.claims.GetIsAdmin() {
				return nil, status.Error(codes.PermissionDenied, "admin only")
			}
			return handler(ctx, req)
		}

		if len(auth.GetAllOf()) > 0 {
			if err := a.authorize(ctx, verified, req, auth); err != nil {
				return nil, err
			}
		}

		// Empty annotation: any authenticated caller is sufficient.
		return handler(ctx, req)
	}
}

// ConnectUnary returns the equivalent Connect handler interceptor. Connect
// handlers are the browser/API path, so they must use the same proto auth
// annotations as native gRPC handlers.
func (a *AuthInterceptor) ConnectUnary() connect.UnaryInterceptorFunc {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			return a.connectUnary(ctx, req, next)
		}
	})
}

func (a *AuthInterceptor) connectUnary(ctx context.Context, req connect.AnyRequest, next connect.UnaryFunc) (connect.AnyResponse, error) {
	auth := methodAuth(req.Spec().Procedure)
	if auth == nil {
		auth = &iam.MethodAuth{AdminOnly: true}
	}
	if auth.GetPublic() {
		return next(ctx, req)
	}
	// Connect handlers run over plain HTTP, so the bearer credential arrives as
	// an HTTP header rather than gRPC incoming metadata. Bridge it so the shared
	// authenticate()/bearerToken() path (which reads grpc metadata) works for the
	// browser/Connect API exactly as it does for native gRPC.
	ctx = incomingMetadataFromHeader(ctx, req.Header())
	verified, err := a.authenticate(ctx)
	if err != nil {
		return nil, err
	}
	ctx = contextWithVerifiedCaller(ctx, verified)

	if auth.GetAdminOnly() {
		if verified.restricted || !verified.claims.GetIsAdmin() {
			return nil, status.Error(codes.PermissionDenied, "admin only")
		}
		return next(ctx, req)
	}
	if len(auth.GetAllOf()) > 0 {
		if err := a.authorize(ctx, verified, req.Any(), auth); err != nil {
			return nil, err
		}
	}
	return next(ctx, req)
}

// AuthorizeGraphQL enforces the proto auth annotation for a gRPC procedure on the
// in-process GraphQL path (the generated graphql-go resolvers delegate to the
// pb.*ServiceServer without going through the connect/gRPC interceptors, so they
// must call this to get identical per-method authn + authz). The bearer
// credential must already be present in ctx as gRPC incoming metadata — the
// /graphql HTTP middleware bridges the Authorization header before execution.
// procedure is the gRPC procedure name ("/cloud.v1.api.IamService/Login").
// Returns the context enriched with the verified caller.
func (a *AuthInterceptor) AuthorizeGraphQL(ctx context.Context, procedure string, msg any) (context.Context, error) {
	auth := methodAuth(procedure)
	if auth == nil {
		auth = &iam.MethodAuth{AdminOnly: true}
	}
	if auth.GetPublic() {
		return ctx, nil
	}
	verified, err := a.authenticate(ctx)
	if err != nil {
		return ctx, err
	}
	ctx = contextWithVerifiedCaller(ctx, verified)
	if auth.GetAdminOnly() {
		if verified.restricted || !verified.claims.GetIsAdmin() {
			return ctx, status.Error(codes.PermissionDenied, "admin only")
		}
		return ctx, nil
	}
	if len(auth.GetAllOf()) > 0 {
		if err := a.authorize(ctx, verified, msg, auth); err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}

type connectAuthInterceptor struct {
	auth *AuthInterceptor
}

func (i connectAuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		return i.auth.connectUnary(ctx, req, next)
	}
}

func (i connectAuthInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i connectAuthInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		auth := methodAuth(conn.Spec().Procedure)
		if auth == nil {
			auth = &iam.MethodAuth{AdminOnly: true}
		}
		if auth.GetPublic() {
			return next(ctx, conn)
		}
		ctx = incomingMetadataFromHeader(ctx, conn.RequestHeader())
		verified, err := i.auth.authenticate(ctx)
		if err != nil {
			return err
		}
		ctx = contextWithVerifiedCaller(ctx, verified)

		if auth.GetAdminOnly() {
			if verified.restricted || !verified.claims.GetIsAdmin() {
				return status.Error(codes.PermissionDenied, "admin only")
			}
			return next(ctx, conn)
		}
		if len(auth.GetAllOf()) == 0 {
			return next(ctx, conn)
		}
		return next(ctx, &authzConnectStream{
			StreamingHandlerConn: conn,
			ctx:                  ctx,
			auth:                 auth,
			verified:             verified,
			interceptor:          i.auth,
		})
	}
}

type authzConnectStream struct {
	connect.StreamingHandlerConn
	ctx         context.Context
	auth        *iam.MethodAuth
	verified    verifiedCaller
	interceptor *AuthInterceptor
	authorized  bool
}

func (s *authzConnectStream) Receive(msg any) error {
	if err := s.StreamingHandlerConn.Receive(msg); err != nil {
		return err
	}
	if !s.authorized {
		if err := s.interceptor.authorize(s.ctx, s.verified, msg, s.auth); err != nil {
			return err
		}
		s.authorized = true
	}
	return nil
}

// Stream returns the native gRPC streaming interceptor. It authenticates before
// the handler runs, then authorizes on the first received request message so
// tenant-scoped stream RPCs can still use MethodAuth. Server-streaming handlers
// receive exactly one request message; bidi/client streams are authorized on
// their first inbound message.
func (a *AuthInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		auth := methodAuth(info.FullMethod)
		if auth == nil {
			auth = &iam.MethodAuth{AdminOnly: true}
		}
		if auth.GetPublic() {
			return handler(srv, stream)
		}
		verified, err := a.authenticate(stream.Context())
		if err != nil {
			return err
		}
		ctx := contextWithVerifiedCaller(stream.Context(), verified)
		wrapped := &contextServerStream{ServerStream: stream, ctx: ctx}

		if auth.GetAdminOnly() {
			if verified.restricted || !verified.claims.GetIsAdmin() {
				return status.Error(codes.PermissionDenied, "admin only")
			}
			return handler(srv, wrapped)
		}
		if len(auth.GetAllOf()) == 0 {
			return handler(srv, wrapped)
		}
		return handler(srv, &authzServerStream{
			contextServerStream: wrapped,
			auth:                auth,
			verified:            verified,
			interceptor:         a,
		})
	}
}

func (a *AuthInterceptor) authenticate(ctx context.Context) (verifiedCaller, error) {
	token, err := bearerToken(ctx)
	if err != nil {
		return verifiedCaller{}, err
	}
	if v, ok := a.tokens.(RestrictedTokenVerifier); ok {
		claims, permissions, restricted, err := v.VerifyWithRestrictions(ctx, token)
		if err != nil {
			return verifiedCaller{}, status.Error(codes.Unauthenticated, "invalid access token")
		}
		return verifiedCaller{claims: claims, permissions: permissions, restricted: restricted}, nil
	}
	claims, err := a.tokens.Verify(ctx, token)
	if err != nil {
		return verifiedCaller{}, status.Error(codes.Unauthenticated, "invalid access token")
	}
	return verifiedCaller{claims: claims}, nil
}

func (a *AuthInterceptor) authorize(ctx context.Context, verified verifiedCaller, req any, auth *iam.MethodAuth) error {
	if verified.claims.GetIsAdmin() && !verified.restricted {
		return nil // platform super-user short-circuits to allow.
	}
	switch req.(type) {
	case *api.CreateTenantRequest:
		// Tenant creation is governed by PlatformSettings.allow_member_tenant_creation
		// in the handler; a tenant-scoped RBAC check cannot apply before the tenant
		// exists.
		return nil
	case *api.GetTenantRequest:
		// GetTenant supports slug lookup. The handler resolves slug->id and
		// enforces admin/member access there.
		return nil
	case *api.GetAccountRequest, *api.UpdateAccountRequest:
		// Account self-service and tenant-member account joins need request-aware
		// checks (self/admin/co-tenant), not platform-wide RESOURCE_ACCOUNT grants.
		// The handlers enforce the narrower policy after loading the target.
		return nil
	}
	tenantID := tenantIDFromRequest(req, auth.GetTenantField())
	granted, err := a.perms.EffectivePermissions(ctx, verified.claims.GetAccountId(), tenantID)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if !hasAll(granted, auth.GetAllOf()) {
		return status.Error(codes.PermissionDenied, "missing required permission")
	}
	if verified.restricted && !hasAll(verified.permissions, auth.GetAllOf()) {
		return status.Error(codes.PermissionDenied, "token missing required permission")
	}
	return nil
}

func contextWithVerifiedCaller(ctx context.Context, verified verifiedCaller) context.Context {
	return ContextWithClaims(ctx, verified.claims)
}

type contextServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *contextServerStream) Context() context.Context { return s.ctx }

type authzServerStream struct {
	*contextServerStream
	auth        *iam.MethodAuth
	verified    verifiedCaller
	interceptor *AuthInterceptor
	authorized  bool
}

func (s *authzServerStream) RecvMsg(m any) error {
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return err
	}
	if !s.authorized {
		if err := s.interceptor.authorize(s.ctx, s.verified, m, s.auth); err != nil {
			return err
		}
		s.authorized = true
	}
	return nil
}

// incomingMetadataFromHeader makes an HTTP Authorization header visible to the
// gRPC-metadata based bearerToken() path. It is a no-op when grpc incoming
// metadata already carries authorization (native gRPC) or no header is present.
func incomingMetadataFromHeader(ctx context.Context, h http.Header) context.Context {
	if md, ok := metadata.FromIncomingContext(ctx); ok && len(md.Get("authorization")) > 0 {
		return ctx
	}
	authz := h.Get("Authorization")
	if authz == "" {
		return ctx
	}
	return metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", authz))
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
