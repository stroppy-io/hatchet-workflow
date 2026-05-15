package middleware

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"go.uber.org/zap"
)

func Logging(log *zap.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			fields := []zap.Field{
				zap.String("rpc", req.Spec().Procedure),
				zap.Duration("duration", time.Since(start)),
				zap.String("request_id", RequestIDFromCtx(ctx)),
			}
			if uid := UserFromCtx(ctx); uid != "" {
				fields = append(fields, zap.String("user_id", uid))
			}
			if tid := TenantFromCtx(ctx); tid != "" {
				fields = append(fields, zap.String("tenant_id", tid))
			}
			if err != nil {
				fields = append(fields, zap.Error(err))
				log.Warn("rpc.error", fields...)
			} else {
				log.Info("rpc.ok", fields...)
			}
			return resp, err
		}
	}
}
