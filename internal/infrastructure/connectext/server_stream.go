// Package connectext bridges connect-go and gRPC server streaming so the existing
// gRPC-style service handlers (which write to a grpc.ServerStreamingServer) can be
// mounted as connect handlers unchanged. The connect handler receives a
// *connect.ServerStream and wraps it via NewConnectStreamWrapper, which satisfies
// the grpc.ServerStreamingServer[T] interface the service expects.
package connectext

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func errUnimplemented(method string) error {
	return status.Error(codes.Unimplemented, "method: "+method+" not implemented")
}

// ConnectStreamWrapper adapts a connect server stream to grpc.ServerStreamingServer[T].
type ConnectStreamWrapper[T any] struct {
	ctx context.Context
	*connect.ServerStream[T]
}

func NewConnectStreamWrapper[T any](ctx context.Context, stream *connect.ServerStream[T]) *ConnectStreamWrapper[T] {
	return &ConnectStreamWrapper[T]{ctx: ctx, ServerStream: stream}
}

func (c *ConnectStreamWrapper[T]) Send(event *T) error      { return c.ServerStream.Send(event) }
func (c *ConnectStreamWrapper[T]) Context() context.Context { return c.ctx }

func (c *ConnectStreamWrapper[T]) SetHeader(metadata.MD) error { panic(errUnimplemented("SetHeader")) }
func (c *ConnectStreamWrapper[T]) SendHeader(metadata.MD) error {
	panic(errUnimplemented("SendHeader"))
}
func (c *ConnectStreamWrapper[T]) SetTrailer(metadata.MD) { panic(errUnimplemented("SetTrailer")) }
func (c *ConnectStreamWrapper[T]) SendMsg(any) error      { panic(errUnimplemented("SendMsg")) }
func (c *ConnectStreamWrapper[T]) RecvMsg(any) error      { panic(errUnimplemented("RecvMsg")) }
