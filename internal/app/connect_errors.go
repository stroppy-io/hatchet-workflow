package app

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// grpcStatusToConnect is a server-side Connect interceptor that translates the
// gRPC status errors returned by the services (utils.MapErr / status.Error) into
// Connect errors. Without it every error reaches Connect clients as CodeUnknown
// (Connect does not understand gRPC status), so clients/SPA cannot distinguish
// NotFound / InvalidArgument / PermissionDenied / Unauthenticated. gRPC and
// Connect share the canonical code numbering, so codes map 1:1.
//
// It is wired OUTERMOST in the handler interceptor chain so it also translates
// errors produced by the auth interceptor.
type grpcStatusToConnect struct{}

func (grpcStatusToConnect) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		resp, err := next(ctx, req)
		return resp, translateGRPCError(err)
	}
}

func (grpcStatusToConnect) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (grpcStatusToConnect) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		return translateGRPCError(next(ctx, conn))
	}
}

func translateGRPCError(err error) error {
	if err == nil {
		return nil
	}
	// Leave errors that are already Connect errors untouched.
	var ce *connect.Error
	if errors.As(err, &ce) {
		return err
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() == codes.OK {
		return err
	}
	return connect.NewError(connect.Code(st.Code()), errors.New(st.Message()))
}
