package middleware

import (
	"context"
	"errors"
	"runtime/debug"

	"connectrpc.com/connect"
	"go.uber.org/zap"
)

func Recovery(log *zap.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (resp connect.AnyResponse, err error) {
			defer func() {
				if r := recover(); r != nil {
					log.Error("panic in handler",
						zap.Any("panic", r),
						zap.String("rpc", req.Spec().Procedure),
						zap.ByteString("stack", debug.Stack()),
					)
					err = connect.NewError(connect.CodeInternal, errors.New("internal error"))
				}
			}()
			return next(ctx, req)
		}
	}
}
