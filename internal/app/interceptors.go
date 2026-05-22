package app

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/grpc/metadata"

	"github.com/stroppy-io/stroppy-cloud/internal/api/caller"
	"github.com/stroppy-io/stroppy-cloud/internal/api/middleware"
)

// adminMethodPrefix scopes the admin guard to the platform-admin services.
const adminMethodPrefix = "/cloud.v1.api.admin."

// connectAuthInterceptor authenticates every connect/grpc/grpc-web request and
// stores the resolved Caller in context. It reuses the existing gRPC-metadata
// Authenticator by lifting the connect Authorization header into incoming gRPC
// metadata. Anonymous requests proceed (per-service guards decide); the admin API
// services additionally require the platform-admin claim.
type connectAuthInterceptor struct {
	authn middleware.Authenticator
}

func newConnectAuthInterceptor(authn middleware.Authenticator) *connectAuthInterceptor {
	return &connectAuthInterceptor{authn: authn}
}

func (i *connectAuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := i.auth(ctx, req.Header(), req.Spec().Procedure)
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i *connectAuthInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := i.auth(ctx, conn.RequestHeader(), conn.Spec().Procedure)
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

func (i *connectAuthInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *connectAuthInterceptor) auth(ctx context.Context, h http.Header, procedure string) (context.Context, error) {
	if tok := h.Get("Authorization"); tok != "" {
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", tok))
	}
	c, err := i.authn.Authenticate(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	if c != nil {
		ctx = caller.NewContext(ctx, c)
	}
	if strings.HasPrefix(procedure, adminMethodPrefix) {
		if c == nil || !c.IsAdmin {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("platform admin required"))
		}
	}
	return ctx, nil
}
