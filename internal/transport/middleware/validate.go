package middleware

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
)

type Validator interface {
	Validate() error
}

func ProtoValidate() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if msg, ok := req.Any().(proto.Message); ok {
				if v, ok := msg.(Validator); ok {
					if err := v.Validate(); err != nil {
						return nil, connect.NewError(connect.CodeInvalidArgument, err)
					}
				}
			}
			return next(ctx, req)
		}
	}
}
