package app

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type validateAll interface {
	ValidateAll() error
}

type validate interface {
	Validate() error
}

// requestValidationInterceptor enforces generated PGV validation for every
// inbound Connect request before auth or handler code reads request fields.
type requestValidationInterceptor struct{}

func (requestValidationInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if err := validateMessage(req.Any()); err != nil {
			return nil, invalidArgument(err)
		}
		return next(ctx, req)
	}
}

func (requestValidationInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (requestValidationInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		return next(ctx, validatingConnectStream{StreamingHandlerConn: conn})
	}
}

type validatingConnectStream struct {
	connect.StreamingHandlerConn
}

func (s validatingConnectStream) Receive(msg any) error {
	if err := s.StreamingHandlerConn.Receive(msg); err != nil {
		return err
	}
	if err := validateMessage(msg); err != nil {
		return invalidArgument(err)
	}
	return nil
}

func grpcValidationUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := validateMessage(req); err != nil {
			return nil, invalidArgument(err)
		}
		return handler(ctx, req)
	}
}

func grpcValidationStream() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, validatingServerStream{ServerStream: stream})
	}
}

type validatingServerStream struct {
	grpc.ServerStream
}

func (s validatingServerStream) RecvMsg(msg any) error {
	if err := s.ServerStream.RecvMsg(msg); err != nil {
		return err
	}
	if err := validateMessage(msg); err != nil {
		return invalidArgument(err)
	}
	return nil
}

func validateMessage(msg any) error {
	switch v := msg.(type) {
	case validateAll:
		return v.ValidateAll()
	case validate:
		return v.Validate()
	default:
		return nil
	}
}

func invalidArgument(err error) error {
	if err == nil {
		return nil
	}
	return status.Error(codes.InvalidArgument, err.Error())
}
